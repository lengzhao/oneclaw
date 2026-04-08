package session

import (
	"errors"
	"testing"

	"github.com/lengzhao/oneclaw/tools"
)

var errTestResolver = errors.New("test resolver factory error")

func TestSessionResolverDistinctEngines(t *testing.T) {
	r := NewSessionResolver(func(SessionHandle) (*Engine, error) {
		return NewEngine(t.TempDir(), tools.NewRegistry()), nil
	})

	e1, err := r.EngineFor(SessionHandle{Source: "feishu", SessionKey: "a"})
	if err != nil {
		t.Fatal(err)
	}
	e2, err := r.EngineFor(SessionHandle{Source: "feishu", SessionKey: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if e1 == e2 {
		t.Fatal("expected different engines")
	}
}

func TestSessionResolverSameHandleReusesEngine(t *testing.T) {
	r := NewSessionResolver(func(SessionHandle) (*Engine, error) {
		return NewEngine(t.TempDir(), tools.NewRegistry()), nil
	})
	e1, err := r.EngineFor(SessionHandle{Source: "slack", SessionKey: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	e2, err := r.EngineFor(SessionHandle{Source: "slack", SessionKey: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if e1 != e2 {
		t.Fatal("expected same engine instance")
	}
}

func TestSessionResolverFactoryError(t *testing.T) {
	r := NewSessionResolver(func(SessionHandle) (*Engine, error) {
		return nil, errTestResolver
	})
	_, err := r.EngineFor(SessionHandle{Source: "http", SessionKey: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
