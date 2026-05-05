package config

import (
	"testing"

	cbconfig "github.com/lengzhao/clawbridge/config"
)

func TestRedactSecretsDeepCopy_modelsAndBridge(t *testing.T) {
	f := &File{
		Models: []ModelProfile{{ID: "d", APIKey: "secret-key"}},
		Clawbridge: cbconfig.Config{
			Clients: []cbconfig.ClientConfig{{
				ID:      "w",
				Driver:  "weixin",
				Enabled: true,
				Options: map[string]any{"context_token": "tok", "listen": "127.0.0.1:1"},
			}},
		},
	}
	out, err := RedactSecretsDeepCopy(f)
	if err != nil {
		t.Fatal(err)
	}
	if out.Models[0].APIKey != "***" {
		t.Fatalf("model key %q", out.Models[0].APIKey)
	}
	if f.Models[0].APIKey != "secret-key" {
		t.Fatal("mutated original model")
	}
	if out.Clawbridge.Clients[0].Options["context_token"] != "***" {
		t.Fatalf("token opt %+v", out.Clawbridge.Clients[0].Options)
	}
	if out.Clawbridge.Clients[0].Options["listen"] != "127.0.0.1:1" {
		t.Fatal("listen should not redact")
	}
}
