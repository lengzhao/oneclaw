package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type sendMessageAttachmentIn struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Path string `json:"path"`
	Text string `json:"text"`
}

type sendMessageToolInput struct {
	Text          string                    `json:"text"`
	Source        string                    `json:"source"`
	SessionKey    string                    `json:"session_key"`
	UserID        string                    `json:"user_id"`
	TenantID      string                    `json:"tenant_id"`
	CorrelationID string                    `json:"correlation_id"`
	RawRef        json.RawMessage           `json:"raw_ref"`
	Attachments   []sendMessageAttachmentIn `json:"attachments"`
}

// SendMessageTool pushes text and/or media via clawbridge without ending the model turn (proactive notify).
type SendMessageTool struct{}

func (SendMessageTool) Name() string          { return "send_message" }
func (SendMessageTool) ConcurrencySafe() bool { return false }

func (SendMessageTool) Description() string {
	return "Proactively notify the user or another configured channel/thread: send text and optional attachments (project-relative media paths under .oneclaw/media/inbound/…, or small inline text bodies). " +
		"Use when the user should get an immediate ping (e.g. long doc finished, timer reminder) or when delivering files outside the normal assistant reply. " +
		"Defaults: source/session_key/user_id/tenant_id come from the current turn when omitted."
}

func (SendMessageTool) Parameters() openai.FunctionParameters {
	attItem := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Display name"},
			"mime": map[string]any{"type": "string", "description": "MIME type (default text/plain)"},
			"path": map[string]any{"type": "string", "description": "Project-relative path after persistence, usually under .oneclaw/media/inbound/"},
			"text": map[string]any{"type": "string", "description": "Inline body; host may persist to media store"},
		},
	}
	return objectSchema(map[string]any{
		"text": map[string]any{
			"type":        "string",
			"description": "Message body (required unless attachments only)",
		},
		"source": map[string]any{
			"type":        "string",
			"description": "Channel instance id (config channels[].id); default: current turn's client id",
		},
		"session_key": map[string]any{
			"type":        "string",
			"description": "Optional: target thread/session (Peer.ID)",
		},
		"user_id": map[string]any{
			"type":        "string",
			"description": "Optional: sender/recipient user id (Sender)",
		},
		"tenant_id": map[string]any{
			"type":        "string",
			"description": "Optional: tenant hint (Sender.Platform)",
		},
		"correlation_id": map[string]any{
			"type":        "string",
			"description": "Optional: MessageID / correlation for logging",
		},
		"raw_ref": map[string]any{
			"type":        "object",
			"description": "Deprecated: ignored in clawbridge host",
		},
		"attachments": map[string]any{
			"type":        "array",
			"description": "Optional files or inline snippets",
			"items":       attItem,
		},
	}, []string{})
}

func routingTargetOverridesTurn(in sendMessageToolInput, tctx *toolctx.Context) bool {
	tin := tctx.TurnInbound
	if strings.TrimSpace(in.Source) != "" && strings.TrimSpace(in.Source) != strings.TrimSpace(tin.Channel) {
		return true
	}
	if strings.TrimSpace(in.SessionKey) != "" && strings.TrimSpace(in.SessionKey) != strings.TrimSpace(tin.Peer.ID) {
		return true
	}
	uid := session.InboundUserID(tin)
	if strings.TrimSpace(in.UserID) != "" && strings.TrimSpace(in.UserID) != uid {
		return true
	}
	th := session.InboundTenantHint(tin)
	if strings.TrimSpace(in.TenantID) != "" && strings.TrimSpace(in.TenantID) != th {
		return true
	}
	return false
}

func (SendMessageTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	if tctx == nil {
		return "", fmt.Errorf("send_message: missing tool context")
	}
	var in sendMessageToolInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	src := strings.TrimSpace(in.Source)
	if src == "" {
		src = strings.TrimSpace(tctx.TurnInbound.Channel)
	}
	if src == "" {
		return "", fmt.Errorf("send_message: need source (or a turn with channel context)")
	}

	out := tctx.TurnInbound
	out.Channel = src
	out.Content = strings.TrimSpace(in.Text)
	out.MediaPaths = nil

	if sk := strings.TrimSpace(in.SessionKey); sk != "" {
		out.Peer.ID = sk
	}
	if u := strings.TrimSpace(in.UserID); u != "" {
		out.Sender.PlatformID = u
		out.Sender.CanonicalID = u
	}
	if ten := strings.TrimSpace(in.TenantID); ten != "" {
		out.Sender.Platform = ten
	}
	if corr := strings.TrimSpace(in.CorrelationID); corr != "" {
		out.MessageID = corr
	} else if !routingTargetOverridesTurn(in, tctx) {
		out.MessageID = strings.TrimSpace(tctx.TurnInbound.MessageID)
	}

	var atts []session.Attachment
	for _, a := range in.Attachments {
		atts = append(atts, session.Attachment{
			Name: strings.TrimSpace(a.Name),
			MIME: strings.TrimSpace(a.MIME),
			Path: strings.TrimSpace(a.Path),
			Text: a.Text,
		})
	}
	atts = session.NormalizeAttachments(atts)
	if err := session.PersistInlineAttachmentFiles(tctx.CWD, &atts); err != nil {
		return "", err
	}
	for _, a := range atts {
		if p := strings.TrimSpace(a.Path); p != "" {
			out.MediaPaths = append(out.MediaPaths, p)
		}
	}

	if strings.TrimSpace(out.Content) == "" && len(out.MediaPaths) == 0 {
		return "", fmt.Errorf("send_message: need text and/or attachments")
	}
	if tctx.SendMessage == nil {
		return "", fmt.Errorf("send_message: not available in this runtime (no outbound host)")
	}
	if err := tctx.SendMessage(ctx, out); err != nil {
		return "", err
	}
	return "sent", nil
}
