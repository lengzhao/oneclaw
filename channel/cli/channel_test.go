package cli

import (
	"testing"

	"github.com/lengzhao/oneclaw/channel"
)

func TestNew(t *testing.T) {
	c, err := New(channel.ConnectorConfig{})
	if err != nil || c == nil {
		t.Fatalf("New: %v %v", c, err)
	}
	if c.Name() != RegistryName {
		t.Fatalf("name=%q", c.Name())
	}
}
