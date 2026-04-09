package session

import (
	"testing"

	"github.com/lengzhao/oneclaw/tools"
)

func TestSessionShardIndex_deterministic(t *testing.T) {
	n := 4
	a := sessionShardIndex("feishu\x00thr1", n)
	b := sessionShardIndex("feishu\x00thr1", n)
	if a != b {
		t.Fatal("shard unstable")
	}
	if a < 0 || a >= n {
		t.Fatalf("out of range: %d", a)
	}
}

func TestWorkerPool_newClose(t *testing.T) {
	t.Parallel()
	wp, err := NewWorkerPool(3, func(SessionHandle) (*Engine, error) {
		return NewEngine(t.TempDir(), tools.NewRegistry()), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wp.Close()
}

func TestNewWorkerPool_nilFactory(t *testing.T) {
	_, err := NewWorkerPool(2, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
