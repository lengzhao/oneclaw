package notify

import (
	"context"
	"errors"
	"testing"
)

func TestMultiEmit(t *testing.T) {
	var n int
	s1 := FuncSink(func(ctx context.Context, ev Event) error {
		n++
		return nil
	})
	s2 := FuncSink(func(ctx context.Context, ev Event) error {
		n++
		return errors.New("x")
	})
	s3 := FuncSink(func(ctx context.Context, ev Event) error {
		n++
		return nil
	})
	m := Multi{s1, s2, s3}
	err := m.Emit(context.Background(), NewEvent(EventTurnComplete, ""))
	if err == nil || err.Error() != "x" {
		t.Fatalf("err=%v", err)
	}
	if n != 3 {
		t.Fatalf("calls=%d", n)
	}
}

func TestEmitSafeNil(t *testing.T) {
	EmitSafe(nil, context.Background(), NewEvent(EventTurnComplete, ""))
}

func TestMultiRegister(t *testing.T) {
	var m Multi
	var n int
	m.Register(nil, FuncSink(func(ctx context.Context, ev Event) error {
		n++
		return nil
	}))
	if len(m) != 1 {
		t.Fatalf("len=%d", len(m))
	}
	_ = m.Emit(context.Background(), NewEvent(EventTurnComplete, ""))
	if n != 1 {
		t.Fatalf("calls=%d", n)
	}
}

func TestEmitSafePanic(t *testing.T) {
	s := FuncSink(func(ctx context.Context, ev Event) error {
		panic("boom")
	})
	EmitSafe(s, context.Background(), NewEvent(EventTurnComplete, ""))
}
