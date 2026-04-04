package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/session"
)

// REPLConfig drives the terminal message loop.
type REPLConfig struct {
	Engine *session.Engine
	// TranscriptPath is written on exit and on /save (optional).
	TranscriptPath string
	// In, Out, Err default to os.Stdin, os.Stdout, os.Stderr.
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// RunREPL reads lines from In, submits user turns to Engine, handles /exit and /save.
func RunREPL(cfg REPLConfig) error {
	if cfg.Engine == nil {
		return fmt.Errorf("cli: Engine is nil")
	}
	in := cfg.In
	if in == nil {
		in = os.Stdin
	}
	out := cfg.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := cfg.Err
	if errOut == nil {
		errOut = os.Stderr
	}

	fmt.Fprintln(errOut, "oneclaw — type a message, /exit to quit, /save <path>, empty line + Ctrl-D to exit")
	sc := bufio.NewScanner(in)
	var turnSeq int
	for {
		fmt.Fprint(out, "> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
			continue
		case line == "/exit":
			saveTranscript(cfg.Engine, cfg.TranscriptPath)
			return nil
		case strings.HasPrefix(line, "/save "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "/save "))
			if path == "" {
				fmt.Fprintln(errOut, "usage: /save <path>")
				continue
			}
			saveTranscript(cfg.Engine, path)
			continue
		}

		turnSeq++
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		in := routing.Inbound{
			Source:        routing.SourceCLI,
			Text:          line,
			SessionKey:    cfg.Engine.SessionID,
			CorrelationID: fmt.Sprintf("cli-%d", turnSeq),
		}
		err := cfg.Engine.SubmitUser(ctx, in)
		aborted := ctx.Err() != nil
		stop()
		if err != nil {
			if aborted {
				continue
			}
			slog.Error("cli.turn.failed", "component", "cli", "err", err)
			continue
		}
	}
	if err := sc.Err(); err != nil {
		slog.Error("stdin", "err", err)
	}
	saveTranscript(cfg.Engine, cfg.TranscriptPath)
	return nil
}

func saveTranscript(eng *session.Engine, path string) {
	if path == "" {
		return
	}
	b, err := eng.MarshalTranscript()
	if err != nil {
		slog.Error("marshal transcript", "err", err)
		return
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		slog.Error("write transcript", "err", err)
		return
	}
	slog.Info("saved transcript", "path", path)
}
