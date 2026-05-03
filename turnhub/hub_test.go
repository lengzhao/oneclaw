package turnhub

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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

func TestHub_turnTimeout(t *testing.T) {
	ctx := context.Background()
	h := NewHub(ctx, func(c context.Context, _ clawbridge.InboundMessage) error {
		select {
		case <-time.After(10 * time.Second):
			return nil
		case <-c.Done():
			return c.Err()
		}
	}, WithTurnTimeout(40*time.Millisecond))

	handle := SessionHandle{Channel: "c", Session: "s"}
	if err := h.Enqueue(handle, PolicySerial, clawbridge.InboundMessage{ClientID: "c", SessionID: "s"}); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.WaitIdle(waitCtx); err != nil {
		t.Fatal(err)
	}
}

func TestHub_dropOldestWhenChannelSaturated(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var drops atomic.Int32
	h := NewHub(ctx, func(c context.Context, _ clawbridge.InboundMessage) error {
		select {
		case <-time.After(80 * time.Millisecond):
			return nil
		case <-c.Done():
			return c.Err()
		}
	}, WithMaxBuf(2), WithOnDropped(func(_ context.Context, _ clawbridge.InboundMessage) error {
		drops.Add(1)
		return nil
	}))

	handle := SessionHandle{Channel: "c", Session: "s"}
	var wg sync.WaitGroup
	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = h.Enqueue(handle, PolicySerial, clawbridge.InboundMessage{
				ClientID: "c", SessionID: "s",
				Content: fmt.Sprintf("m%d", i),
			})
		}(i)
	}
	wg.Wait()
	if err := h.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if drops.Load() == 0 {
		t.Fatal("expected at least one dropped inbound when mailbox buffer saturates")
	}
}
