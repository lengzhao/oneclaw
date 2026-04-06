package session

import (
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/mediastore"
	"github.com/lengzhao/oneclaw/routing"
)

// maxPersistAttachmentBytes matches statichttp upload cap (raw bytes before rune normalization on legacy text-only paths).
const maxPersistAttachmentBytes = 4 << 20

// ValidateInboundMediaPaths rejects attachment Path values that are not under .oneclaw/media/inbound (any date subfolder).
func ValidateInboundMediaPaths(cwd string, atts []routing.Attachment) error {
	for _, a := range atts {
		if p := strings.TrimSpace(a.Path); p != "" {
			if err := mediastore.ValidateRelPath(cwd, p); err != nil {
				return fmt.Errorf("inbound attachment %q: %w", a.Name, err)
			}
		}
	}
	return nil
}

// PersistInlineAttachmentFiles writes Text to the media store and sets Path; clears Text.
func PersistInlineAttachmentFiles(cwd string, atts *[]routing.Attachment) error {
	for i := range *atts {
		a := &(*atts)[i]
		if strings.TrimSpace(a.Path) != "" {
			continue
		}
		if strings.TrimSpace(a.Text) == "" {
			continue
		}
		rel, err := mediastore.StoreBytes(cwd, a.Name, []byte(a.Text), maxPersistAttachmentBytes)
		if err != nil {
			return fmt.Errorf("persist attachment %q: %w", a.Name, err)
		}
		a.Path = rel
		a.Text = ""
	}
	return nil
}
