package model

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/openai/openai-go"
)

func TestChatRetryBackoff(t *testing.T) {
	if got := chatRetryBackoff(0); got != 0 {
		t.Fatalf("0 -> %v", got)
	}
	if got := chatRetryBackoff(1); got != 400*time.Millisecond {
		t.Fatalf("1 -> %v", got)
	}
	if got := chatRetryBackoff(2); got != 800*time.Millisecond {
		t.Fatalf("2 -> %v", got)
	}
	if got := chatRetryBackoff(10); got != 2*time.Second {
		t.Fatalf("cap -> %v", got)
	}
}

func TestIsRetriableChatError(t *testing.T) {
	if isRetriableChatError(nil) {
		t.Fatal("nil")
	}
	if isRetriableChatError(context.Canceled) {
		t.Fatal("canceled")
	}
	if !isRetriableChatError(context.DeadlineExceeded) {
		t.Fatal("deadline")
	}
	if !isRetriableChatError(fmt.Errorf("wrap: %w", context.DeadlineExceeded)) {
		t.Fatal("wrap deadline")
	}

	api503 := &openai.Error{StatusCode: http.StatusServiceUnavailable}
	if !isRetriableChatError(api503) {
		t.Fatal("503")
	}
	api429 := &openai.Error{StatusCode: http.StatusTooManyRequests}
	if !isRetriableChatError(api429) {
		t.Fatal("429")
	}
	api400 := &openai.Error{StatusCode: http.StatusBadRequest}
	if isRetriableChatError(api400) {
		t.Fatal("400 should not retry")
	}

	var netTO net.Error = timeoutNetErr{}
	if !isRetriableChatError(netTO) {
		t.Fatal("net timeout")
	}
}

type timeoutNetErr struct{}

func (timeoutNetErr) Error() string   { return "timeout" }
func (timeoutNetErr) Timeout() bool   { return true }
func (timeoutNetErr) Temporary() bool { return false }
