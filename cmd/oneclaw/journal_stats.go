package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/lengzhao/oneclaw/journaltools"
)

func cmdJournalStats(ctx context.Context, _ globalOpts, args []string) error {
	_ = ctx
	fs := flag.NewFlagSet("journal-stats", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jp := fs.String("journal", "", "path to turn run journal JSONL")
	udr := fs.String("user-data-root", "", "UserDataRoot (for resolving absolute skills paths)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*jp) == "" {
		return fmt.Errorf("journal-stats: --journal required")
	}
	m, err := journaltools.ScanJournalFile(*jp, *udr, nil)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "%s\n", string(b))
	return err
}
