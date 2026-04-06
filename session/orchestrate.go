package session

import (
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/routing"
)

// InboundMetaForModel builds a user-shaped routing context block. Omits CorrelationID and RawRef by design
// (see docs/inbound-routing-design.md).
func InboundMetaForModel(in routing.Inbound) string {
	var lines []string
	if s := strings.TrimSpace(in.Source); s != "" {
		lines = append(lines, "source: "+s)
	}
	if s := strings.TrimSpace(in.SessionKey); s != "" {
		lines = append(lines, "session_key: "+s)
	}
	if s := strings.TrimSpace(in.Locale); s != "" {
		lines = append(lines, "locale: "+s)
	}
	if s := strings.TrimSpace(in.UserID); s != "" {
		lines = append(lines, "user_id: "+s)
	}
	if s := strings.TrimSpace(in.TenantID); s != "" {
		lines = append(lines, "tenant_id: "+s)
	}
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<inbound-context>\n")
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	b.WriteString("</inbound-context>")
	return b.String()
}

// FormatInboundAttachmentMessages turns normalized attachments into user message bodies.
func FormatInboundAttachmentMessages(atts []routing.Attachment) []string {
	if len(atts) == 0 {
		return nil
	}
	out := make([]string, 0, len(atts))
	for _, a := range atts {
		if p := strings.TrimSpace(a.Path); p != "" {
			out = append(out, fmt.Sprintf(
				"[Attachment: %s (%s)]\nFile saved in the project media store. Read with read_file using this path (relative to cwd):\n%s",
				a.Name, a.MIME, p,
			))
			continue
		}
		if strings.TrimSpace(a.Text) != "" {
			out = append(out, fmt.Sprintf("[Attachment: %s (%s)]\n%s", a.Name, a.MIME, a.Text))
		}
	}
	return out
}

// ModelUserLine is the primary user message line sent to the model (placeholder when only attachments).
func ModelUserLine(text string, hasAttachments bool) string {
	t := strings.TrimSpace(text)
	if t != "" {
		return t
	}
	if hasAttachments {
		return "（本轮无正文，仅附件。）"
	}
	return ""
}

// SlimTranscriptUserLine is persisted in the slim transcript (memory prefixes omitted).
func SlimTranscriptUserLine(in routing.Inbound) string {
	t := strings.TrimSpace(in.Text)
	if len(in.Attachments) == 0 {
		return t
	}
	var sb strings.Builder
	sb.WriteString(t)
	if t != "" {
		sb.WriteString("\n\n")
	}
	sb.WriteString("[attachments:")
	for i, a := range in.Attachments {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(" ")
		if strings.TrimSpace(a.Path) != "" {
			sb.WriteString(a.Name)
			sb.WriteString("@")
			sb.WriteString(a.Path)
		} else {
			sb.WriteString(a.Name)
		}
	}
	sb.WriteString(" ]")
	return sb.String()
}

func combinedInboundPreview(in routing.Inbound) string {
	t := strings.TrimSpace(in.Text)
	if len(in.Attachments) == 0 {
		return t
	}
	var sb strings.Builder
	sb.WriteString(t)
	for _, a := range in.Attachments {
		if sb.Len() > 0 {
			sb.WriteString(" · ")
		}
		if strings.TrimSpace(a.Path) != "" {
			sb.WriteString(a.Name)
			sb.WriteString("→")
			sb.WriteString(a.Path)
		} else {
			sb.WriteString(a.Name)
		}
	}
	return sb.String()
}
