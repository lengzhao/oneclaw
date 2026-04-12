//go:build e2e

package e2e_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-70 PublishOutbound：一轮 SubmitUser 至少发出一条助手文本（无流式 Done 事件）
func TestE2E_70_PublishOutboundAssistantText(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "sink_e2e_70_reply"))
	e2eEnvMinimal(t, stub)

	var mu sync.Mutex
	var texts []string
	e := newStubEngine(t, stub, t.TempDir())
	e.PublishOutbound = func(_ context.Context, msg *bus.OutboundMessage) error {
		mu.Lock()
		defer mu.Unlock()
		if msg != nil && strings.TrimSpace(msg.Text) != "" {
			texts = append(texts, msg.Text)
		}
		return nil
	}

	if err := e.SubmitUser(context.Background(), bus.InboundMessage{ClientID: "cli", SessionID: "C1", Content: "hello sink"}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(texts) < 1 {
		t.Fatalf("expected at least one outbound text, got %v", texts)
	}
	var saw bool
	for _, s := range texts {
		if strings.Contains(s, "sink_e2e_70_reply") {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatalf("expected reply in outbound texts: %v", texts)
	}
}
