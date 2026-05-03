package engine

import (
	"context"
	"testing"
)

func TestReadSnapshot_roundTripContext(t *testing.T) {
	ctx := context.Background()
	s := ReadSnapshot{UserPrompt: "x", SessionRoot: "/s"}
	ctx = WithReadSnapshot(ctx, s)
	got, ok := ReadSnapshotFromContext(ctx)
	if !ok || got.UserPrompt != "x" || got.SessionRoot != "/s" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestEffectiveUserPrompt_fromGoCtxSnapshot(t *testing.T) {
	rtx := &RuntimeContext{UserPrompt: "live"}
	ctx := WithReadSnapshot(context.Background(), ReadSnapshot{UserPrompt: "frozen"})
	rtx.GoCtx = ctx
	if rtx.EffectiveUserPrompt() != "frozen" {
		t.Fatalf("got %q", rtx.EffectiveUserPrompt())
	}
	rtx.GoCtx = context.Background()
	if rtx.EffectiveUserPrompt() != "live" {
		t.Fatalf("got %q", rtx.EffectiveUserPrompt())
	}
}
