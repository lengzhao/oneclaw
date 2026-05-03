package tools

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool/utils"
)

func TestRegistryFilter(t *testing.T) {
	r := NewRegistry(t.TempDir())
	echoT, err := utils.InferTool("echo", "e", func(ctx context.Context, in registryEchoIn) (string, error) {
		return in.Message, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Register(echoT); err != nil {
		t.Fatal(err)
	}
	out, err := r.FilterByNames([]string{"echo"})
	if err != nil || len(out) != 1 {
		t.Fatalf("out=%v err=%v", out, err)
	}
	_, err = r.FilterByNames([]string{"missing"})
	if err == nil {
		t.Fatal("want error")
	}
}

type registryEchoIn struct {
	Message string `json:"message" jsonschema:"description=test"`
}
