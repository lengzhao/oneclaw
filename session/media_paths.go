package session

import (
	"path/filepath"
	"strings"
)

// AttachmentsFromMediaPaths builds attachments from bus media path locators (basename as name).
func AttachmentsFromMediaPaths(paths []string) []Attachment {
	if len(paths) == 0 {
		return nil
	}
	out := make([]Attachment, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		base := filepath.Base(strings.ReplaceAll(p, "\\", "/"))
		if base == "" || base == "." {
			base = "attachment"
		}
		out = append(out, Attachment{Name: base, MIME: "application/octet-stream", Path: p})
	}
	return out
}
