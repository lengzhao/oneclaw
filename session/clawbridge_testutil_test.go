package session

import (
	"context"
	"testing"
	"time"

	"github.com/lengzhao/clawbridge"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/clawbridge/client"
	_ "github.com/lengzhao/clawbridge/drivers"
)

// testStartNoopBridge starts a noop-driver bridge for clientIDs and returns it for assigning to [Engine.Bridge].
// Optional captureMsg is invoked from OutboundSendNotify after each outbound send (for assertions).
// Call cleanup in defer; do not use t.Parallel in tests that rely on this helper.
func testStartNoopBridge(t *testing.T, clientIDs []string, captureMsg func(*bus.OutboundMessage)) (bridge *clawbridge.Bridge, cleanup func()) {
	t.Helper()
	cfgs := make([]clawbridge.ClientConfig, len(clientIDs))
	for i, id := range clientIDs {
		cfgs[i] = clawbridge.ClientConfig{ID: id, Driver: "noop", Enabled: true}
	}
	var opts []clawbridge.Option
	if captureMsg != nil {
		opts = append(opts, clawbridge.WithOutboundSendNotify(func(ctx context.Context, info client.OutboundSendNotifyInfo) {
			if info.Message != nil {
				captureMsg(info.Message)
			}
		}))
	}
	b, err := clawbridge.New(clawbridge.Config{Clients: cfgs}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return b, func() {
		cancel()
		stopCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = b.Stop(stopCtx)
	}
}

// waitForOutboundDispatch waits until PublishOutbound has been picked up by the bridge outbound
// loop and the noop driver's Send completed (OutboundSendNotify runs after Send).
func waitForOutboundDispatch(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("timeout waiting for outbound dispatch / notify")
}
