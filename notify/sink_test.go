package notify

import (
	"context"
	"errors"
	"testing"
)

// testFuncSink is a minimal Sink for this package's tests (replaces removed FuncSink).
type testFuncSink struct {
	fn func(context.Context, Event) error
}

func (s testFuncSink) Emit(ctx context.Context, ev Event) error {
	if s.fn == nil {
		return nil
	}
	return s.fn(ctx, ev)
}

func TestMultiEmit(t *testing.T) {
	var n int
	s1 := testFuncSink{fn: func(ctx context.Context, ev Event) error {
		n++
		return nil
	}}
	s2 := testFuncSink{fn: func(ctx context.Context, ev Event) error {
		n++
		return errors.New("x")
	}}
	s3 := testFuncSink{fn: func(ctx context.Context, ev Event) error {
		n++
		return nil
	}}
	m := Multi{s1, s2, s3}
	err := m.Emit(context.Background(), NewEvent(EventTurnEnd, ""))
	if err == nil || err.Error() != "x" {
		t.Fatalf("err=%v", err)
	}
	if n != 3 {
		t.Fatalf("calls=%d", n)
	}
}

func TestEmitSafeNil(t *testing.T) {
	EmitSafe(nil, context.Background(), NewEvent(EventTurnEnd, ""))
}

func TestMultiRegister(t *testing.T) {
	var m Multi
	var n int
	m.Register(nil, testFuncSink{fn: func(ctx context.Context, ev Event) error {
		n++
		return nil
	}})
	if len(m) != 1 {
		t.Fatalf("len=%d", len(m))
	}
	_ = m.Emit(context.Background(), NewEvent(EventTurnEnd, ""))
	if n != 1 {
		t.Fatalf("calls=%d", n)
	}
}

func TestEmitSafePanic(t *testing.T) {
	s := testFuncSink{fn: func(ctx context.Context, ev Event) error {
		panic("boom")
	}}
	EmitSafe(s, context.Background(), NewEvent(EventTurnEnd, ""))
}
