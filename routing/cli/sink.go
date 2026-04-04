// Package cli is the terminal channel: registers routing.SourceCLI on import and provides RunREPL for stdin input.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/lengzhao/oneclaw/routing"
)

// Sink writes assistant output to a terminal: text → stdout; optional JSONL per Record → stderr.
type Sink struct {
	OutStdout  io.Writer
	OutStderr  io.Writer
	JSONEvents bool
}

func init() {
	routing.RegisterDefaultSink(routing.SourceCLI, NewSink(os.Stdout, os.Stderr, false))
}

// NewSink builds a terminal sink. When jsonEvents is true, each Record is also JSON-encoded to stderr (one line per event).
func NewSink(outStdout, outStderr io.Writer, jsonEvents bool) *Sink {
	return &Sink{OutStdout: outStdout, OutStderr: outStderr, JSONEvents: jsonEvents}
}

func (s *Sink) Emit(_ context.Context, r routing.Record) error {
	if s.JSONEvents {
		enc := json.NewEncoder(s.OutStderr)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	switch r.Kind {
	case routing.KindText:
		if c, _ := r.Data["content"].(string); c != "" {
			fmt.Fprintln(s.OutStdout, c)
		}
	case routing.KindTool:
		// Human-readable stream stays quiet when JSONEvents is false.
	case routing.KindDone:
		ok, _ := r.Data["ok"].(bool)
		if ok {
			if !s.JSONEvents {
				fmt.Fprintln(s.OutStderr, "(turn done)")
			}
		} else {
			msg, _ := r.Data["error"].(string)
			if msg != "" {
				fmt.Fprintln(s.OutStderr, msg)
			}
		}
	}
	return nil
}
