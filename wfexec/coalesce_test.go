package wfexec

import (
	"testing"

	"github.com/lengzhao/oneclaw/engine"
)

func TestCoalesceRTX(t *testing.T) {
	rtx := &engine.RuntimeContext{UserPrompt: "z"}
	got, err := coalesceRTX(map[string]any{"b": rtx, "c": (*engine.RuntimeContext)(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if got != rtx {
		t.Fatal("coalesce picked wrong value")
	}
}

func TestCoalesceRTX_errors(t *testing.T) {
	if _, err := coalesceRTX(nil); err == nil {
		t.Fatal("expected error")
	}
	if _, err := coalesceRTX(map[string]any{"x": "string"}); err == nil {
		t.Fatal("expected error")
	}
}
