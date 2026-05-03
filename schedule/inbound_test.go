package schedule

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/meta"
)

func TestInboundFromJob_peerMatchesDirectChannelShape(t *testing.T) {
	in := InboundFromJob(Job{ID: "sj_a", SessionSegment: "sess-1", Prompt: "x"})
	if in.Peer.Kind != "direct" || in.Peer.ID != "sess-1" {
		t.Fatalf("peer: %+v", in.Peer)
	}
}

func TestInboundFromJob_contentExplainsScheduleFire(t *testing.T) {
	in := InboundFromJob(Job{ID: "sj_z", Name: "nm", SessionSegment: "s", Prompt: "你好", NextRunUnix: 99})
	if !strings.Contains(in.Content, "sj_z") || !strings.Contains(in.Content, "你好") {
		t.Fatalf("content: %q", in.Content)
	}
	if !strings.Contains(in.Content, "Scheduler") || !strings.Contains(in.Content, "非用户实时输入") {
		t.Fatalf("expected scheduler markers in content: %q", in.Content)
	}
	if strings.TrimSpace(in.Content) == "你好" {
		t.Fatal("content must not be raw prompt only")
	}
}

func TestInboundFromJob_mergesReplyMeta(t *testing.T) {
	j := Job{
		ID:             "sj_x",
		SessionSegment: "u1@im.wechat",
		ClientID:       "weixin-1",
		Prompt:         "ping",
		ReplyMeta: map[string]string{
			"context_token": "tok-9",
			"from_user_id":  "u1",
		},
	}
	j.Normalize()
	in := InboundFromJob(j)
	if in.SessionID != "u1@im.wechat" || in.Peer.ID != "u1@im.wechat" {
		t.Fatalf("session/peer: session_id=%q peer.id=%q", in.SessionID, in.Peer.ID)
	}
	if in.Metadata["context_token"] != "tok-9" {
		t.Fatalf("context_token: %q", in.Metadata["context_token"])
	}
	if in.Metadata["from_user_id"] != "u1" {
		t.Fatalf("from_user_id: %q", in.Metadata["from_user_id"])
	}
	if in.Metadata[meta.SourceKey] != meta.SourceSchedule {
		t.Fatalf("source: %q", in.Metadata[meta.SourceKey])
	}
}

func TestInboundFromJob_replyMetaDoesNotOverrideScheduleKeys(t *testing.T) {
	j := Job{
		ID:             "sj_x",
		SessionSegment: "u1@im.wechat",
		ClientID:       "weixin-1",
		Prompt:         "ping",
		AgentID:        "default",
		ReplyMeta: map[string]string{
			meta.InboundAgent: "evil",
		},
	}
	j.Normalize()
	in := InboundFromJob(j)
	if in.Metadata[meta.InboundAgent] != "default" {
		t.Fatalf("agent should stay from job field, got %q", in.Metadata[meta.InboundAgent])
	}
}

func TestInboundFromJob_ignoresEmptyReplyMeta(t *testing.T) {
	j := Job{
		ID:             "sj_x",
		SessionSegment: "s",
		Prompt:         "p",
		ReplyMeta: map[string]string{
			"context_token": "  ",
		},
	}
	j.Normalize()
	in := InboundFromJob(j)
	if strings.TrimSpace(in.Metadata["context_token"]) != "" {
		t.Fatalf("empty token should be skipped")
	}
}
