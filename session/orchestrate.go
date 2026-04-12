package session

import (
	"fmt"
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

// InboundMetaForModel builds a user-shaped routing context block. Omits MessageID (correlation) by design
// (see docs/inbound-routing-design.md).
func InboundMetaForModel(in bus.InboundMessage) string {
	var lines []string
	if s := strings.TrimSpace(in.ClientID); s != "" {
		lines = append(lines, "source: "+s)
	}
	// Driver delivery key for send_message / OutboundMessage.To.SessionID (e.g. webchat wc-…).
	// Do not confuse with workspace_session_id (StableSessionID / transcript folder hash).
	if sk := InboundSessionKey(in); sk != "" {
		lines = append(lines, "session_key: "+sk)
	}
	if src := strings.TrimSpace(in.ClientID); src != "" {
		h := SessionHandle{Source: src, SessionKey: InboundSessionKey(in)}
		lines = append(lines, "workspace_session_id: "+StableSessionID(h))
	}
	if s := InboundUserID(in); s != "" {
		lines = append(lines, "user_id: "+s)
	}
	if s := InboundTenantHint(in); s != "" {
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

// formatInboundAttachmentUserText is one user-shaped block for a single attachment (read_file hint or inline text).
func formatInboundAttachmentUserText(a Attachment) string {
	if p := strings.TrimSpace(a.Path); p != "" {
		return fmt.Sprintf(
			"[Attachment: %s (%s)]\nFile saved in the project media store. Read with read_file using this path (relative to cwd):\n%s",
			a.Name, a.MIME, p,
		)
	}
	if strings.TrimSpace(a.Text) != "" {
		return fmt.Sprintf("[Attachment: %s (%s)]\n%s", a.Name, a.MIME, a.Text)
	}
	return ""
}

// ModelUserLine is the primary user message line sent to the model (placeholder when only attachments).
func ModelUserLine(text string, hasAttachments bool) string {
	t := strings.TrimSpace(text)
	if t != "" {
		return t
	}
	if hasAttachments {
		return ""
	}
	return ""
}

// SlimTranscriptUserLine is persisted in the slim transcript (memory prefixes omitted).
func SlimTranscriptUserLine(text string, atts []Attachment) string {
	t := strings.TrimSpace(text)
	if len(atts) == 0 {
		return t
	}
	var sb strings.Builder
	sb.WriteString(t)
	if t != "" {
		sb.WriteString("\n\n")
	}
	sb.WriteString("[attachments:")
	for i, a := range atts {
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

func combinedInboundPreview(text string, atts []Attachment) string {
	t := strings.TrimSpace(text)
	if len(atts) == 0 {
		return t
	}
	var sb strings.Builder
	sb.WriteString(t)
	for _, a := range atts {
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
