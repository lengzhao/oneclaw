// Serve loads merged YAML config (including clawbridge), starts the Bridge, TurnHub, inbound consumer, and optional schedule poller.
// Turn path is identical for live drivers and schedule: clawbridge InboundMessage → [turnhub.Hub.Enqueue] → [runner.ExecuteTurn] → [clawbridge.Bridge.Reply].
// Scheduled jobs are created only via the agent cron tool (no separate cron admin API on serve); the poller only delivers due rows from scheduled_jobs.json.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	clawbridge "github.com/lengzhao/clawbridge"
	"github.com/lengzhao/clawbridge/bus"
	cbconfig "github.com/lengzhao/clawbridge/config"
	_ "github.com/lengzhao/clawbridge/drivers" // register webchat and other drivers

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/runner"
	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/turnhub"
)

func cmdServe(ctx context.Context, g globalOpts, args []string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	buf := &strings.Builder{}
	fs.SetOutput(buf)
	mockLLM := fs.Bool("mock-llm", false, "use stub ChatModel for every turn")
	scheduleTick := fs.Duration("schedule-interval", time.Minute, "poll scheduled_jobs.json at this interval (minimum 1m; 0 disables)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("serve: %w\n%s", err, buf.String())
	}
	if *scheduleTick > 0 && *scheduleTick < time.Minute {
		return fmt.Errorf("serve: --schedule-interval must be at least 1m when enabled (got %s)", *scheduleTick)
	}

	cfgPaths := []string{}
	if cp := strings.TrimSpace(g.ConfigPath); cp != "" {
		cfgPaths = append(cfgPaths, cp)
	} else {
		rootGuess, err := paths.ResolveUserDataRoot(nil)
		if err != nil {
			return fmt.Errorf("resolve default user data root: %w", err)
		}
		candidate := filepath.Join(rootGuess, "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			cfgPaths = append(cfgPaths, candidate)
		}
	}
	ocfg, err := config.LoadMerged(cfgPaths)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config.ApplyEnvSecrets(ocfg)
	config.PushRuntime(ocfg)

	root, err := paths.ResolveUserDataRoot(ocfg)
	if err != nil {
		return err
	}
	catRoot := paths.CatalogRoot(root)
	mf, err := catalog.LoadManifest(catRoot)
	if err != nil {
		return err
	}
	cat, err := catalog.Load(filepath.Join(catRoot, "agents"))
	if err != nil {
		return err
	}

	cbCfg := ocfg.Clawbridge
	if countEnabledClients(cbCfg.Clients) == 0 {
		return fmt.Errorf("clawbridge: no enabled clients in config — add a `clawbridge:` section with at least one enabled driver (e.g. webchat); see setup/templates/config.yaml")
	}

	b, err := clawbridge.New(cbCfg, clawbridge.WithOutboundSendNotify(func(_ context.Context, info clawbridge.OutboundSendNotifyInfo) {
		if info.Err != nil {
			slog.Warn("clawbridge outbound", "client_id", info.Message.ClientID, "err", info.Err)
		}
	}))
	if err != nil {
		return fmt.Errorf("clawbridge new: %w", err)
	}

	shutdownBridge := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := b.Stop(shutdownCtx); err != nil {
			slog.Warn("clawbridge stop", "err", err)
		}
	}
	defer shutdownBridge()

	hub := turnhub.NewHub(ctx, newTurnProcessor(b, root, ocfg, cat, mf, mockLLM))
	go runConsumeInbound(ctx, b, hub)

	schedPath := paths.ScheduledJobsPath(root)
	var schedCancel context.CancelFunc
	if *scheduleTick > 0 {
		var schedCtx context.Context
		schedCtx, schedCancel = context.WithCancel(ctx)
		p := schedule.NewPoller(schedPath, func(c context.Context, j schedule.Job) error {
			in := schedule.InboundFromJob(j)
			if in.Metadata == nil {
				in.Metadata = map[string]string{}
			}
			in.Metadata[runner.InboundMetaCorrelation] = subagent.NewCorrelationID()
			if err := hub.Enqueue(turnhub.HandleFromInbound(&in), turnhub.PolicySerial, in); err != nil {
				slog.Error("turnhub Enqueue (schedule)", "err", err,
					"schedule_job_id", j.ID,
					"session_id", in.SessionID,
					"correlation_id", in.Metadata[runner.InboundMetaCorrelation],
				)
				return err
			}
			return nil
		})
		go func() {
			t := time.NewTicker(*scheduleTick)
			defer t.Stop()
			for {
				select {
				case <-schedCtx.Done():
					return
				case <-t.C:
					if err := p.Tick(schedCtx); err != nil {
						slog.Error("schedule tick", "err", err)
					}
				}
			}
		}()
	}
	if schedCancel != nil {
		defer schedCancel()
	}

	if err := b.Start(ctx); err != nil {
		return fmt.Errorf("clawbridge start: %w", err)
	}

	schedIV := "disabled"
	if scheduleTick != nil && *scheduleTick > 0 {
		schedIV = (*scheduleTick).String()
	}
	slog.Info("oneclaw serve",
		"user_data_root", root,
		"schedule", schedPath,
		"schedule_interval", schedIV,
	)
	<-ctx.Done()
	return nil
}

func countEnabledClients(clients []cbconfig.ClientConfig) int {
	n := 0
	for _, c := range clients {
		if c.Enabled {
			n++
		}
	}
	return n
}

func runConsumeInbound(ctx context.Context, b *clawbridge.Bridge, hub *turnhub.Hub) {
	for {
		in, err := b.Bus().ConsumeInbound(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, bus.ErrClosed) {
				return
			}
			slog.Error("clawbridge ConsumeInbound", "err", err)
			return
		}
		if err := hub.Enqueue(turnhub.HandleFromInbound(&in), turnhub.PolicySerial, in); err != nil {
			args := []any{"err", err, "client_id", in.ClientID, "session_id", in.SessionID, "message_id", in.MessageID}
			if in.Metadata != nil {
				if c := strings.TrimSpace(in.Metadata[runner.InboundMetaCorrelation]); c != "" {
					args = append(args, "correlation_id", c)
				}
				if j := strings.TrimSpace(in.Metadata[runner.InboundMetaScheduleJob]); j != "" {
					args = append(args, "schedule_job_id", j)
				}
			}
			slog.Error("turnhub Enqueue", args...)
		}
	}
}

func newTurnProcessor(b *clawbridge.Bridge, root string, ocfg *config.File, cat *catalog.Catalog, mf *catalog.Manifest, globalMock *bool) turnhub.Processor {
	return func(c context.Context, msg clawbridge.InboundMessage) error {
		msgCopy := msg
		sess := turnhub.HandleFromInbound(&msgCopy).Session
		agent := strings.TrimSpace(msgCopy.Metadata[runner.InboundMetaAgent])
		prof := strings.TrimSpace(msgCopy.Metadata[runner.InboundMetaProfile])
		mock := *globalMock
		if v := strings.TrimSpace(msgCopy.Metadata[runner.InboundMetaMockLLM]); v == "1" || strings.EqualFold(v, "true") {
			mock = true
		}
		corr := strings.TrimSpace(msgCopy.Metadata[runner.InboundMetaCorrelation])
		if corr == "" {
			corr = subagent.NewCorrelationID()
		}
		return runner.ExecuteTurn(runner.Params{
			Ctx:             c,
			UserDataRoot:    root,
			Config:          ocfg,
			Catalog:         cat,
			Manifest:        mf,
			AgentID:         agent,
			ProfileID:       prof,
			SessionSegment:  sess,
			UserPrompt:      strings.TrimSpace(msgCopy.Content),
			UseMock:         mock,
			Stdout:          os.Stdout,
			CorrelationID:   corr,
			InboundClientID: strings.TrimSpace(msgCopy.ClientID),
			PostAssistantRespond: func(ctx context.Context, assistant string) error {
				_, err := b.Reply(ctx, &msgCopy, assistant, "")
				if err != nil {
					args := []any{"err", err, "client_id", msgCopy.ClientID, "session_id", sess, "correlation_id", corr}
					if msgCopy.Metadata != nil {
						if j := strings.TrimSpace(msgCopy.Metadata[runner.InboundMetaScheduleJob]); j != "" {
							args = append(args, "schedule_job_id", j)
						}
					}
					slog.Error("clawbridge Reply", args...)
				}
				return err
			},
		})
	}
}
