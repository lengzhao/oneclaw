package routing

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Attachment is wire-neutral inbound payload: either a project-relative Path (under .oneclaw/media/inbound/<date>/)
// after persistence, or inline Text before the engine persists it.
type Attachment struct {
	Name string
	MIME string
	// Path is relative to the session cwd (slash-separated); when set, Text is not sent to the model.
	Path string
	Text string
}

const (
	maxAttachmentRunes = 200_000
	maxTotalAttRunes   = 400_000
)

// NormalizeAttachments returns a shallow copy with per-item and total UTF-8 rune caps; adds a truncation notice when trimmed.
func NormalizeAttachments(in []Attachment) []Attachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]Attachment, 0, len(in))
	total := 0
	for _, a := range in {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			name = "attachment"
		}
		mime := strings.TrimSpace(a.MIME)
		if mime == "" {
			mime = "text/plain"
		}
		if p := strings.TrimSpace(a.Path); p != "" {
			out = append(out, Attachment{Name: name, MIME: mime, Path: p})
			continue
		}
		body := a.Text
		if strings.TrimSpace(body) == "" {
			continue
		}
		if n := utf8.RuneCountInString(body); n > maxAttachmentRunes {
			body = truncateRunes(body, maxAttachmentRunes)
			body += "\n\n[truncated: attachment exceeded " + strconv.Itoa(maxAttachmentRunes) + " runes]"
		}
		n := utf8.RuneCountInString(body)
		if total+n > maxTotalAttRunes {
			room := maxTotalAttRunes - total
			if room <= 0 {
				break
			}
			body = truncateRunes(body, room)
			body += "\n\n[truncated: total attachments budget exceeded]"
			n = utf8.RuneCountInString(body)
		}
		total += n
		out = append(out, Attachment{Name: name, MIME: mime, Text: body})
	}
	return out
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
