package turnhub

import (
	"context"
	"sync"
	"testing"
	"time"

	clawbridge "github.com/lengzhao/clawbridge"
)

func TestHub_serialPreservesOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var mu sync.Mutex
	var got []string
	h := NewHub(ctx, func(_ context.Context, msg clawbridge.InboundMessage) error {
		mu.Lock()
		got = append(got, msg.Content)
		mu.Unlock()
		return nil
	})
	handle := SessionHandle{Channel: "c", Session: "s"}
	if err := h.Enqueue(handle, PolicySerial, clawbridge.InboundMessage{ClientID: "c", SessionID: "s", Content: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := h.Enqueue(handle, PolicySerial, clawbridge.InboundMessage{ClientID: "c", SessionID: "s", Content: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := h.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
}

func TestHandleFromInbound_sanitizesSession(t *testing.T) {
	in := clawbridge.InboundMessage{ClientID: "cli", SessionID: "../evil"}
	h := HandleFromInbound(&in)
	if h.Session == "" || h.Session == "../evil" {
		t.Fatalf("unexpected session %q", h.Session)
	}
}
