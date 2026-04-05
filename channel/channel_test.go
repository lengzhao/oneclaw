package channel

import (
	"context"
	"testing"

	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

type stubConn struct{}

func (stubConn) Name() string { return "stub" }

func (stubConn) Run(context.Context, IO) error { return nil }

func TestStartAllNilEngine(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterSpec(Spec{
		Key: "a",
		New: func(context.Context, ConnectorConfig) (Connector, error) {
			return stubConn{}, nil
		},
	})
	_, err := reg.StartAll(context.Background(), Bootstrap{})
	if err == nil {
		t.Fatal("expected error for nil Engine")
	}
}

func TestRegistryStartAllStubConnectors(t *testing.T) {
	eng := session.NewEngine(t.TempDir(), builtin.DefaultRegistry())
	reg := NewRegistry()
	reg.RegisterSpec(Spec{
		Key: "a",
		New: func(context.Context, ConnectorConfig) (Connector, error) {
			return stubConn{}, nil
		},
	})
	reg.RegisterSpec(Spec{
		Key: "b",
		New: func(context.Context, ConnectorConfig) (Connector, error) {
			return stubConn{}, nil
		},
	})
	_, err := reg.StartAll(context.Background(), Bootstrap{Engine: eng})
	if err != nil {
		t.Fatal(err)
	}
}
