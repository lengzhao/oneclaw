package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/routing"
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
	Text           string                    `json:"text"`
	Source         string                    `json:"source"`
	SessionKey     string                    `json:"session_key"`
	UserID         string                    `json:"user_id"`
	TenantID       string                    `json:"tenant_id"`
	CorrelationID  string                    `json:"correlation_id"`
	RawRef         json.RawMessage           `json:"raw_ref"`
	Attachments    []sendMessageAttachmentIn `json:"attachments"`
}

// SendMessageTool pushes text and/or media to a channel Sink without ending the model turn (proactive notify).
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
			"description": "Channel instance id (config channels[].id); default: current turn's source",
		},
		"session_key": map[string]any{
			"type":        "string",
			"description": "Optional: target thread/session for SinkFactory",
		},
		"user_id": map[string]any{
			"type":        "string",
			"description": "Optional: recipient user id for routing",
		},
		"tenant_id": map[string]any{
			"type":        "string",
			"description": "Optional: tenant id for routing",
		},
		"correlation_id": map[string]any{
			"type":        "string",
			"description": "Optional: id on outbound Record.job_id",
		},
		"raw_ref": map[string]any{
			"type":        "object",
			"description": "Optional: opaque JSON for channel-specific SinkFactory (advanced)",
		},
		"attachments": map[string]any{
			"type":        "array",
			"description": "Optional files or inline snippets",
			"items":       attItem,
		},
	}, []string{})
}

func routingTargetOverridesTurn(in sendMessageToolInput, tctx *toolctx.Context) bool {
	if strings.TrimSpace(in.Source) != "" && strings.TrimSpace(in.Source) != strings.TrimSpace(tctx.TurnInbound.Source) {
		return true
	}
	if strings.TrimSpace(in.SessionKey) != "" && strings.TrimSpace(in.SessionKey) != strings.TrimSpace(tctx.TurnInbound.SessionKey) {
		return true
	}
	if strings.TrimSpace(in.UserID) != "" && strings.TrimSpace(in.UserID) != strings.TrimSpace(tctx.TurnInbound.UserID) {
		return true
	}
	if strings.TrimSpace(in.TenantID) != "" && strings.TrimSpace(in.TenantID) != strings.TrimSpace(tctx.TurnInbound.TenantID) {
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
		src = strings.TrimSpace(tctx.TurnInbound.Source)
	}
	if src == "" {
		return "", fmt.Errorf("send_message: need source (or a turn with channel source context)")
	}
	out := routing.Inbound{
		Source:        src,
		Text:          strings.TrimSpace(in.Text),
		SessionKey:    strings.TrimSpace(in.SessionKey),
		UserID:        strings.TrimSpace(in.UserID),
		TenantID:      strings.TrimSpace(in.TenantID),
		CorrelationID: strings.TrimSpace(in.CorrelationID),
	}
	if out.SessionKey == "" {
		out.SessionKey = strings.TrimSpace(tctx.TurnInbound.SessionKey)
	}
	if out.UserID == "" {
		out.UserID = strings.TrimSpace(tctx.TurnInbound.UserID)
	}
	if out.TenantID == "" {
		out.TenantID = strings.TrimSpace(tctx.TurnInbound.TenantID)
	}
	if out.CorrelationID == "" {
		out.CorrelationID = strings.TrimSpace(tctx.TurnInbound.CorrelationID)
	}
	if len(in.RawRef) > 0 && string(in.RawRef) != "null" {
		var ref any
		if err := json.Unmarshal(in.RawRef, &ref); err != nil {
			return "", fmt.Errorf("send_message: raw_ref: %w", err)
		}
		out.RawRef = ref
	} else if !routingTargetOverridesTurn(in, tctx) && tctx.TurnInbound.RawRef != nil {
		out.RawRef = tctx.TurnInbound.RawRef
	}
	for _, a := range in.Attachments {
		out.Attachments = append(out.Attachments, routing.Attachment{
			Name: strings.TrimSpace(a.Name),
			MIME: strings.TrimSpace(a.MIME),
			Path: strings.TrimSpace(a.Path),
			Text: a.Text,
		})
	}
	if out.Text == "" && len(out.Attachments) == 0 {
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
