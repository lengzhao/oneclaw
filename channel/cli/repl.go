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

	"github.com/lengzhao/oneclaw/channel"
)

func printOutbound(ctx context.Context, io channel.IO) {
	out := os.Stdout
	errOut := os.Stderr
	for {
		select {
		case <-ctx.Done():
			return
		case r, ok := <-io.OutboundChan:
			if !ok {
				return
			}
			printRecord(out, errOut, r)
		}
	}
}

func printRecord(out, errOut io.Writer, r channel.Record) {
	switch r.Kind {
	case channel.KindText:
		if c, _ := r.Data["content"].(string); c != "" {
			fmt.Fprintln(out, c)
		}
	case channel.KindTool:
	case channel.KindDone:
		ok, _ := r.Data["ok"].(bool)
		if ok {
			fmt.Fprintln(errOut, "(turn done)")
		} else {
			msg, _ := r.Data["error"].(string)
			if msg != "" {
				fmt.Fprintln(errOut, msg)
			}
		}
	}
}

func stdinLoop(ctx context.Context, io channel.IO) error {
	in := os.Stdin
	out := os.Stdout
	errOut := os.Stderr

	fmt.Fprintln(errOut, "oneclaw — type a message, /exit to quit, empty line + Ctrl-D to exit")
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
			slog.Info("cli.exit.requested")
			os.Exit(0)
			return nil
		}

		turnSeq++
		turnCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		done := make(chan error, 1)
		io.InboundChan <- channel.InboundTurn{
			Ctx:           turnCtx,
			Text:          line,
			CorrelationID: fmt.Sprintf("cli-%d", turnSeq),
			Done:          done,
		}
		var err error
		select {
		case err = <-done:
		case <-ctx.Done():
			err = ctx.Err()
		}
		stop()
		aborted := turnCtx.Err() != nil
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
	return nil
}
