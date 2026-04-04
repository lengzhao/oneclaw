package e2e_test

import (
	"context"
	"testing"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-60 transcript 序列化再加载
func TestE2E_60_TranscriptRoundTrip(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "once"))
	e2eEnvMinimal(t, stub)
	e := newStubEngine(t, t.TempDir())
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	nBefore := len(e.Messages)
	data, err := e.MarshalTranscript()
	if err != nil {
		t.Fatal(err)
	}
	e2 := newStubEngine(t, e.CWD)
	if err := e2.LoadTranscript(data); err != nil {
		t.Fatal(err)
	}
	if len(e2.Messages) != nBefore {
		t.Fatalf("want %d msgs, got %d", nBefore, len(e2.Messages))
	}
}

// E2E-61 transcript 损坏 JSON 报错
func TestE2E_61_TranscriptInvalidJSON(t *testing.T) {
	e := newStubEngine(t, t.TempDir())
	err := e.LoadTranscript([]byte(`{"not":"valid structure"`))
	if err == nil {
		t.Fatal("expected error")
	}
}
