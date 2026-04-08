//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-50 默认在 auto memory daily log 追加一行
func TestE2E_50_DailyLogAppendDefault(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "assistant line for log"))
	e2eEnvWithMemory(t, stub)
	s := rtopts.Current()
	s.DisableMemoryExtract = false
	rtopts.Set(&s)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "user line for log"}); err != nil {
		t.Fatal(err)
	}
	lay := memory.DefaultLayout(cwd, home)
	logPath := memory.DailyLogPath(lay.Auto, time.Now().Format("2006-01-02"))
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("daily log: %v", err)
	}
	if !strings.Contains(string(b), "user:") || !strings.Contains(string(b), "assistant:") {
		t.Fatalf("log content: %s", b)
	}
}

// E2E-51 features.disable_memory_extract 不写 daily log
func TestE2E_51_DailyLogDisabledByEnv(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "x"))
	e2eEnvWithMemory(t, stub)
	s := rtopts.Current()
	s.DisableMemoryExtract = true
	rtopts.Set(&s)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "y"}); err != nil {
		t.Fatal(err)
	}
	lay := memory.DefaultLayout(cwd, home)
	logPath := memory.DailyLogPath(lay.Auto, time.Now().Format("2006-01-02"))
	if _, err := os.Stat(logPath); err == nil {
		t.Fatalf("log should not exist: %s", logPath)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
