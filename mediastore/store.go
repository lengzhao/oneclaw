// Package mediastore writes inbound attachment bytes under <cwd>/.oneclaw/media/inbound/<YYYY-MM-DD>/
// (UTC date) so the model can read them via read_file, and old days can be deleted as a whole directory.
package mediastore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/tools/pathutil"
)

// InboundDir returns the absolute root for inbound media (all date buckets live under it).
func InboundDir(cwd string) string {
	return filepath.Join(cwd, memory.DotDir, "media", "inbound")
}

// inboundDayDir returns the absolute directory for today's UTC date bucket, creating it if needed.
func inboundDayDir(cwd string) (string, error) {
	day := time.Now().UTC().Format("2006-01-02")
	dir := filepath.Join(InboundDir(cwd), day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mediastore: mkdir day bucket: %w", err)
	}
	return dir, nil
}

// StoreBytes writes body to a new file under .oneclaw/media/inbound/<UTC-date>/ and returns a path relative to cwd
// (slash-separated). logicalName is used for a safe suffix; maxBytes caps stored size.
func StoreBytes(cwd, logicalName string, body []byte, maxBytes int) (relToCwd string, err error) {
	if cwd == "" {
		return "", fmt.Errorf("mediastore: empty cwd")
	}
	if len(body) > maxBytes {
		return "", fmt.Errorf("mediastore: payload exceeds %d bytes", maxBytes)
	}
	if len(body) == 0 {
		return "", fmt.Errorf("mediastore: empty payload")
	}
	dir, err := inboundDayDir(cwd)
	if err != nil {
		return "", err
	}
	id := randomID()
	safe := sanitizeFileSuffix(logicalName)
	finalName := id + "_" + safe
	abs := filepath.Join(dir, finalName)
	if err := os.WriteFile(abs, body, 0o644); err != nil {
		return "", fmt.Errorf("mediastore: write: %w", err)
	}
	rel, err := filepath.Rel(filepath.Clean(cwd), abs)
	if err != nil {
		_ = os.Remove(abs)
		return "", fmt.Errorf("mediastore: rel path: %w", err)
	}
	return filepath.ToSlash(rel), nil
}

// StoreReader streams up to maxBytes+1 bytes; if more than maxBytes, removes file and returns error.
func StoreReader(cwd, logicalName string, r io.Reader, maxBytes int) (relToCwd string, err error) {
	if cwd == "" {
		return "", fmt.Errorf("mediastore: empty cwd")
	}
	dir, err := inboundDayDir(cwd)
	if err != nil {
		return "", err
	}
	id := randomID()
	safe := sanitizeFileSuffix(logicalName)
	finalName := id + "_" + safe
	abs := filepath.Join(dir, finalName)
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("mediastore: create: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = f.Close()
			_ = os.Remove(abs)
		}
	}()
	n, err := io.Copy(f, io.LimitReader(r, int64(maxBytes)+1))
	if err != nil {
		return "", fmt.Errorf("mediastore: copy: %w", err)
	}
	if int(n) > maxBytes {
		return "", fmt.Errorf("mediastore: payload exceeds %d bytes", maxBytes)
	}
	if n == 0 {
		return "", fmt.Errorf("mediastore: empty payload")
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(abs)
		return "", err
	}
	ok = true
	rel, err := filepath.Rel(filepath.Clean(cwd), abs)
	if err != nil {
		_ = os.Remove(abs)
		return "", fmt.Errorf("mediastore: rel path: %w", err)
	}
	return filepath.ToSlash(rel), nil
}

// ValidateRelPath ensures rel resolves under cwd and under .oneclaw/media/inbound (including date subfolders).
func ValidateRelPath(cwd, rel string) error {
	if strings.TrimSpace(rel) == "" {
		return fmt.Errorf("empty path")
	}
	abs, err := pathutil.ResolveUnderRoot(cwd, rel)
	if err != nil {
		return err
	}
	inboundRoot, err := filepath.Abs(InboundDir(cwd))
	if err != nil {
		return err
	}
	relSub, err := filepath.Rel(inboundRoot, abs)
	if err != nil {
		return fmt.Errorf("path outside media inbound")
	}
	if relSub == ".." || strings.HasPrefix(relSub, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path outside media inbound")
	}
	return nil
}

func randomID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func sanitizeFileSuffix(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == ".." {
		return "upload"
	}
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "upload"
	}
	if len(out) > 180 {
		ext := filepath.Ext(out)
		basePart := strings.TrimSuffix(out, ext)
		if len(basePart) > 160 {
			basePart = basePart[:160]
		}
		out = basePart + ext
	}
	return out
}
