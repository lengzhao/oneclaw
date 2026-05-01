package session

import (
	"context"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/tools"
)

func TestNewTurnHub_nilFactory(t *testing.T) {
	_, err := NewTurnHub(context.Background(), TurnPolicySerial, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTurnHub_Submit_factoryError(t *testing.T) {
	hub, err := NewTurnHub(context.Background(), TurnPolicySerial, func(SessionHandle) (*Engine, error) {
		return nil, errBoom{}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	subErr := hub.Submit(context.Background(), bus.InboundMessage{ClientID: "c", SessionID: "s", Content: "hi"})
	if subErr == nil {
		t.Fatal("expected error")
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

func TestTurnHub_Close_idempotent(t *testing.T) {
	hub, err := NewTurnHub(context.Background(), TurnPolicySerial, func(SessionHandle) (*Engine, error) {
		return NewEngine(t.TempDir(), tools.NewRegistry()), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	hub.Close()
	hub.Close()
}

func TestParseTurnPolicy(t *testing.T) {
	if ParseTurnPolicy("") != TurnPolicySerial || ParseTurnPolicy("serial") != TurnPolicySerial {
		t.Fatal("serial default")
	}
	if ParseTurnPolicy("INSERT") != TurnPolicyInsert {
		t.Fatal("insert")
	}
	if ParseTurnPolicy("preempt") != TurnPolicyPreempt {
		t.Fatal("preempt")
	}
	if ParseTurnPolicy("unknown") != TurnPolicySerial {
		t.Fatal("unknown -> serial")
	}
}

func TestInjectableInboundText(t *testing.T) {
	if !injectableInboundText(bus.InboundMessage{Content: "hello"}) {
		t.Fatal("plain text")
	}
	if injectableInboundText(bus.InboundMessage{Content: "/help"}) {
		t.Fatal("slash")
	}
	if injectableInboundText(bus.InboundMessage{Content: "x", MediaPaths: []string{"/a"}}) {
		t.Fatal("media")
	}
}
