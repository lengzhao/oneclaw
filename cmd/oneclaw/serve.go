// Serve loads merged YAML config (including clawbridge), starts the Bridge, TurnHub, inbound consumer, and optional schedule poller.
// Turn path is identical for live drivers and schedule: clawbridge InboundMessage → [turnhub.Hub.Enqueue] → [runner.ExecuteTurn] → [clawbridge.Bridge.Reply].
// Scheduled jobs are created only via the agent cron tool (no separate cron admin API on serve); delivery uses next-run timer sleep + wake on job file changes (same idea as main branch host poller).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"maps"
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

const (
	turnhubTurnTimeout           = 30 * time.Minute
	turnhubDiscardReplyTimeout   = 15 * time.Second
	turnhubQueueDiscardUserText = "您的消息因处理队列已满已被丢弃，请稍后重试。"
)

func cmdServe(ctx context.Context, g globalOpts, args []string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	buf := &strings.Builder{}
	fs.SetOutput(buf)
	mockLLM := fs.Bool("mock-llm", false, "use stub ChatModel for every turn")
	noSchedule := fs.Bool("no-schedule", false, "disable scheduled_jobs.json delivery loop")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("serve: %w\n%s", err, buf.String())
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
		if info.Err == nil {
			return
		}
		cid := ""
		if info.Message != nil {
			cid = info.Message.ClientID
		}
		slog.Warn("clawbridge outbound", "client_id", cid, "err", info.Err)
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

	hub := turnhub.NewHub(ctx, newTurnProcessor(b, root, ocfg, cat, mf, mockLLM),
		turnhub.WithTurnTimeout(turnhubTurnTimeout),
		turnhub.WithOnDropped(func(_ context.Context, dropped clawbridge.InboundMessage) error {
			replyCtx, cancel := context.WithTimeout(context.Background(), turnhubDiscardReplyTimeout)
			defer cancel()
			_, err := b.Reply(replyCtx, &dropped, turnhubQueueDiscardUserText, "")
			return err
		}),
	)
	go runConsumeInbound(ctx, b, hub)

	schedPath := paths.ScheduledJobsPath(root)
	var schedCancel context.CancelFunc
	if !*noSchedule {
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
		go schedule.RunPollerLoop(schedCtx, p)
	}
	if schedCancel != nil {
		defer schedCancel()
	}

	if err := b.Start(ctx); err != nil {
		return fmt.Errorf("clawbridge start: %w", err)
	}

	schedMode := "disabled"
	if !*noSchedule {
		schedMode = "wake_timer"
	}
	slog.Info("oneclaw serve",
		"user_data_root", root,
		"schedule", schedPath,
		"schedule_delivery", schedMode,
		"schedule_min_sleep", schedule.MinTimerSleep(),
		"schedule_idle_sleep", schedule.IdleTimerSleep(),
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
		sess := strings.TrimSpace(msgCopy.SessionID)
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
		var inboundMeta map[string]string
		if msgCopy.Metadata != nil {
			inboundMeta = maps.Clone(msgCopy.Metadata)
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
			InboundMeta:     inboundMeta,
			RequiredOutboundMetadataKeysForSend: func(clientID string) []string {
				keys, ok := b.RequiredOutboundMetadataKeysForSend(clientID)
				if !ok {
					return nil
				}
				return keys
			},
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
