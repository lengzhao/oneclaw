package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/mediastore"
	"github.com/lengzhao/oneclaw/tools/pathutil"
)

// effectiveMediaRoot returns the clawbridge media backend root used to validate inbound locators.
func (e *Engine) effectiveMediaRoot() string {
	if e == nil {
		return ""
	}
	if s := strings.TrimSpace(e.MediaRoot); s != "" {
		return filepath.Clean(s)
	}
	if s := strings.TrimSpace(e.UserDataRoot); s != "" {
		return filepath.Join(s, "media")
	}
	return ""
}

func (e *Engine) resolveInboundMediaLocator(p, mediaRoot string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return "", fmt.Errorf("http(s) locator not supported for workspace ingest")
	}
	if strings.HasPrefix(strings.ToLower(p), "s3://") {
		return "", fmt.Errorf("s3 locator not supported for workspace ingest")
	}
	if filepath.IsAbs(p) {
		abs, err := filepath.Abs(filepath.Clean(p))
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(abs); err != nil {
			return "", err
		}
		return abs, nil
	}
	for _, base := range []string{e.CWD, mediaRoot} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		cand := filepath.Join(base, p)
		abs, err := filepath.Abs(filepath.Clean(cand))
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}
	return "", fmt.Errorf("cannot resolve media locator")
}

// rehomeInboundAttachments copies attachment bytes from the global MediaStore (or other paths outside cwd)
// into <cwd>/media/inbound/... using mediastore.StoreBytes, which prefixes a random id so the same
// original filename from different sessions never collides in the workspace store.
func (e *Engine) rehomeInboundAttachments(atts *[]Attachment) error {
	if e == nil || atts == nil {
		return nil
	}
	mediaRoot := e.effectiveMediaRoot()
	for i := range *atts {
		a := &(*atts)[i]
		p := strings.TrimSpace(a.Path)
		if p == "" {
			continue
		}
		if err := mediastore.ValidateRelPath(e.CWD, p); err == nil {
			continue
		}
		if mediaRoot == "" {
			return fmt.Errorf("inbound attachment %q: path outside workspace (media root unset)", a.Name)
		}
		absSrc, err := e.resolveInboundMediaLocator(p, mediaRoot)
		if err != nil {
			return fmt.Errorf("inbound attachment %q: %w", a.Name, err)
		}
		if !pathutil.IsUnderRoot(mediaRoot, absSrc) {
			return fmt.Errorf("inbound attachment %q: path outside media root", a.Name)
		}
		data, err := os.ReadFile(absSrc)
		if err != nil {
			return fmt.Errorf("inbound attachment %q: %w", a.Name, err)
		}
		rel, err := mediastore.StoreBytes(e.CWD, a.Name, data, maxPersistAttachmentBytes)
		if err != nil {
			return fmt.Errorf("inbound attachment %q: ingest to workspace: %w", a.Name, err)
		}
		a.Path = rel
	}
	return nil
}
