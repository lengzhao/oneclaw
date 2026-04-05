package routing

import (
	"context"
	"errors"
	"testing"
)

type stubSink struct{ name string }

func (s stubSink) Emit(context.Context, Record) error { return nil }

type testFactory struct {
	sink Sink
	err  error
}

func (f testFactory) NewSink(context.Context, Inbound) (Sink, error) {
	return f.sink, f.err
}

func TestResolveTurnSinkFactoryWins(t *testing.T) {
	reg := NewMapRegistry()
	reg.Register("cli", stubSink{"reg"})
	f := testFactory{sink: stubSink{"fact"}, err: nil}
	s, err := ResolveTurnSink(context.Background(), reg, f, Inbound{Source: "cli", Text: "hi"})
	if err != nil || s == nil {
		t.Fatalf("ResolveTurnSink: %v %v", s, err)
	}
	if s.(stubSink).name != "fact" {
		t.Fatalf("expected factory sink, got %#v", s)
	}
}

func TestResolveTurnSinkFallbackToRegistry(t *testing.T) {
	reg := NewMapRegistry()
	reg.Register("cli", stubSink{"reg"})
	f := testFactory{err: ErrUseRegistrySink}
	s, err := ResolveTurnSink(context.Background(), reg, f, Inbound{Source: "cli", Text: "hi"})
	if err != nil || s == nil {
		t.Fatalf("ResolveTurnSink: %v %v", s, err)
	}
	if s.(stubSink).name != "reg" {
		t.Fatalf("expected registry sink, got %#v", s)
	}
}

func TestResolveTurnSinkFactoryError(t *testing.T) {
	reg := NewMapRegistry()
	reg.Register("cli", stubSink{"reg"})
	f := testFactory{err: errors.New("boom")}
	_, err := ResolveTurnSink(context.Background(), reg, f, Inbound{Source: "cli", Text: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveTurnSinkRegistryOnly(t *testing.T) {
	reg := NewMapRegistry()
	reg.Register("cli", stubSink{"reg"})
	s, err := ResolveTurnSink(context.Background(), reg, nil, Inbound{Source: "cli", Text: "hi"})
	if err != nil || s == nil {
		t.Fatalf("ResolveTurnSink: %v %v", s, err)
	}
}

func TestResolveTurnSinkNilRegistry(t *testing.T) {
	s, err := ResolveTurnSink(context.Background(), nil, nil, Inbound{Source: "cli", Text: "hi"})
	if err != nil || s != nil {
		t.Fatalf("expected nil sink, got %v err=%v", s, err)
	}
}
