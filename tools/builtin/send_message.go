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

// sendMessageToolInput aligns with clawbridge bus addressing (see bus.InboundMessage / Recipient).
// Legacy JSON keys remain accepted: source, session_key, user_id, correlation_id.
type sendMessageToolInput struct {
	Text        string                    `json:"text"`
	ClientID    string                    `json:"client_id"`
	SessionID   string                    `json:"session_id"`
	PeerKind    string                    `json:"peer_kind"`
	PeerID      string                    `json:"peer_id"`
	ToUserID    string                    `json:"to_user_id"`
	TenantID    string                    `json:"tenant_id"`
	ReplyToID   string                    `json:"reply_to_id"`
	LegacySrc   string                    `json:"source"`
	LegacySess  string                    `json:"session_key"`
	LegacyUser  string                    `json:"user_id"`
	LegacyReply string                    `json:"correlation_id"`
	Attachments []sendMessageAttachmentIn `json:"attachments"`
}

func (in *sendMessageToolInput) mergeLegacy() {
	if strings.TrimSpace(in.ClientID) == "" {
		in.ClientID = in.LegacySrc
	}
	if strings.TrimSpace(in.SessionID) == "" {
		in.SessionID = in.LegacySess
	}
	if strings.TrimSpace(in.ToUserID) == "" {
		in.ToUserID = in.LegacyUser
	}
	if strings.TrimSpace(in.ReplyToID) == "" {
		in.ReplyToID = in.LegacyReply
	}
}

func (in *sendMessageToolInput) routingArgs() session.SendMessageRoutingArgs {
	in.mergeLegacy()
	return session.SendMessageRoutingArgs{
		ClientID:  strings.TrimSpace(in.ClientID),
		SessionID: strings.TrimSpace(in.SessionID),
		PeerKind:  strings.TrimSpace(in.PeerKind),
		PeerID:    strings.TrimSpace(in.PeerID),
		ToUserID:  strings.TrimSpace(in.ToUserID),
		TenantID:  strings.TrimSpace(in.TenantID),
	}
}

// SendMessageTool pushes text and/or media via clawbridge without ending the model turn (proactive notify).
type SendMessageTool struct{}

func (SendMessageTool) Name() string          { return "send_message" }
func (SendMessageTool) ConcurrencySafe() bool { return false }

func (SendMessageTool) Description() string {
	return "Proactive outbound via clawbridge: deliver text and/or attachments to a client + session (bus OutboundMessage). " +
		"Defaults match the current turn. " +
		"IMPORTANT: session_id must be the driver routing key (same as inbound <inbound-context> session_key, e.g. webchat wc-…). " +
		"Do NOT use workspace_session_id from inbound context nor the hex id from /status “工作区会话 ID”—those are transcript/CWD hashes, not clawbridge subscribers. " +
		"To reach another tab/thread set session_id to that tab's session_key; another IM client set client_id. " +
		"Legacy keys still work: source, session_key, user_id, correlation_id."
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
		"client_id": map[string]any{
			"type":        "string",
			"description": "clawbridge client id (config clients[].id); default: current turn ClientID",
		},
		"session_id": map[string]any{
			"type":        "string",
			"description": "Clawbridge OutboundMessage.To.SessionID: same value as inbound session_key (e.g. wc-… for webchat). Not the workspace_session_id / /status 工作区 hex id.",
		},
		"peer_kind": map[string]any{
			"type":        "string",
			"description": "Optional: bus.Peer.Kind / Recipient.Kind (e.g. direct, channel)",
		},
		"peer_id": map[string]any{
			"type":        "string",
			"description": "Optional: bus.Peer.ID when the platform threads by peer id (distinct from session_id)",
		},
		"to_user_id": map[string]any{
			"type":        "string",
			"description": "Optional: bus.Recipient.UserID for DM-style delivery when the driver uses it",
		},
		"tenant_id": map[string]any{
			"type":        "string",
			"description": "Optional: Sender.Platform workspace hint (rare)",
		},
		"reply_to_id": map[string]any{
			"type":        "string",
			"description": "Optional: OutboundMessage.reply_to_id (platform message to thread under)",
		},
		"attachments": map[string]any{
			"type":        "array",
			"description": "Optional files or inline snippets",
			"items":       attItem,
		},
	}, []string{})
}

func routingTargetOverridesTurn(in sendMessageToolInput, tctx *toolctx.Context) bool {
	return session.SendMessageTargetOverridesTurn(tctx.TurnInbound, in.routingArgs())
}

func (SendMessageTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	if tctx == nil {
		return "", fmt.Errorf("send_message: missing tool context")
	}
	var in sendMessageToolInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	in.mergeLegacy()
	src := strings.TrimSpace(in.ClientID)
	if src == "" {
		src = strings.TrimSpace(tctx.TurnInbound.ClientID)
	}
	if src == "" {
		return "", fmt.Errorf("send_message: need client_id (or a turn with ClientID context)")
	}

	out := tctx.TurnInbound
	out.ClientID = src
	out.Content = strings.TrimSpace(in.Text)
	out.MediaPaths = nil

	if sid := strings.TrimSpace(in.SessionID); sid != "" {
		out.SessionID = sid
	}
	if pk := strings.TrimSpace(in.PeerKind); pk != "" {
		out.Peer.Kind = pk
	}
	if pid := strings.TrimSpace(in.PeerID); pid != "" {
		out.Peer.ID = pid
	}
	if ten := strings.TrimSpace(in.TenantID); ten != "" {
		out.Sender.Platform = ten
	}
	if u := strings.TrimSpace(in.ToUserID); u != "" {
		if out.Metadata == nil {
			out.Metadata = make(map[string]string)
		}
		out.Metadata[session.MetadataKeyOutboundRecipientUserID] = u
	}
	if rid := strings.TrimSpace(in.ReplyToID); rid != "" {
		out.MessageID = rid
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
