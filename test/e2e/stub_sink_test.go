package e2e_test

import (
	"context"
	"sync"
	"testing"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-70 SinkRegistry：CLI 源注册 Sink 后，一轮 SubmitUser 收到 text 与 done
func TestE2E_70_SinkRegistryTextAndDone(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "sink_e2e_70_reply"))
	e2eEnvMinimal(t, stub)

	var mu sync.Mutex
	var recs []routing.Record
	reg := routing.NewMapRegistry()
	reg.Register("cli", captureSink{recs: &recs, mu: &mu})

	e := newStubEngine(t, t.TempDir())
	e.SinkRegistry = reg

	if err := e.SubmitUser(context.Background(), routing.Inbound{Source: "cli", Text: "hello sink"}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	var sawText, sawDone bool
	for _, r := range recs {
		switch r.Kind {
		case routing.KindText:
			if c, _ := r.Data["content"].(string); c != "" {
				sawText = true
			}
		case routing.KindDone:
			if ok, _ := r.Data["ok"].(bool); ok {
				sawDone = true
			}
		}
	}
	if !sawText || !sawDone {
		t.Fatalf("expected KindText and KindDone(ok); got %v", recs)
	}
}

type captureSink struct {
	recs *[]routing.Record
	mu   *sync.Mutex
}

func (c captureSink) Emit(_ context.Context, r routing.Record) error {
	c.mu.Lock()
	*c.recs = append(*c.recs, r)
	c.mu.Unlock()
	return nil
}
