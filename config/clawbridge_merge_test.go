package config

import (
	"testing"

	cbconfig "github.com/lengzhao/clawbridge/config"
)

func TestUpsertClawbridge_appendsAndReplacesByID(t *testing.T) {
	f := &File{
		Clawbridge: cbconfig.Config{
			Clients: []cbconfig.ClientConfig{
				{ID: "a", Driver: "noop", Enabled: false},
			},
		},
	}
	UpsertClawbridge(f, cbconfig.Config{
		Clients: []cbconfig.ClientConfig{
			{ID: "b", Driver: "webchat", Enabled: true},
		},
	})
	if len(f.Clawbridge.Clients) != 2 {
		t.Fatalf("clients len %d", len(f.Clawbridge.Clients))
	}
	UpsertClawbridge(f, cbconfig.Config{
		Clients: []cbconfig.ClientConfig{
			{ID: "a", Driver: "weixin", Enabled: true},
		},
	})
	if len(f.Clawbridge.Clients) != 2 {
		t.Fatalf("after replace len %d", len(f.Clawbridge.Clients))
	}
	var sawWeixin bool
	for _, c := range f.Clawbridge.Clients {
		if c.ID == "a" && c.Driver == "weixin" && c.Enabled {
			sawWeixin = true
		}
	}
	if !sawWeixin {
		t.Fatalf("got %+v", f.Clawbridge.Clients)
	}
}

func TestUpsertClawbridge_mediaRoot(t *testing.T) {
	f := &File{}
	UpsertClawbridge(f, cbconfig.Config{
		Media: cbconfig.MediaConfig{Root: "/tmp/m"},
	})
	if f.Clawbridge.Media.Root != "/tmp/m" {
		t.Fatalf("root %q", f.Clawbridge.Media.Root)
	}
	UpsertClawbridge(f, cbconfig.Config{
		Media: cbconfig.MediaConfig{Root: "/tmp/n"},
	})
	if f.Clawbridge.Media.Root != "/tmp/n" {
		t.Fatalf("root %q", f.Clawbridge.Media.Root)
	}
}
