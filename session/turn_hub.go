package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/clawbridge/bus"
)

const turnHubMailCap = 256

// TurnHub routes inbound messages to one actor (mailbox + loop) per SessionHandle.
// It replaces WorkerPool for cmd/oneclaw when using per-session serial/insert/preempt policies.
type TurnHub struct {
	parentCtx context.Context
	policy    TurnPolicy
	factory   func(SessionHandle) (*Engine, error)

	mu      sync.Mutex
	coords  map[string]*turnCoordinator
	closed  atomic.Bool
	closeMu sync.Mutex
}

// NewTurnHub builds a hub. parentCtx is typically the process root context; each turn uses
// context.WithCancel(parentCtx) so [TurnHub.CancelInflightTurn] (/stop) can hard-abort the active turn via ctx.
// Preempt policy still serializes through the coordinator mailbox (see [TurnPolicyPreempt]); /stop hard-cancels ctx.
func NewTurnHub(parentCtx context.Context, policy TurnPolicy, factory func(SessionHandle) (*Engine, error)) (*TurnHub, error) {
	if factory == nil {
		return nil, fmt.Errorf("session: TurnHub: nil factory")
	}
	return &TurnHub{
		parentCtx: parentCtx,
		policy:    policy,
		factory:   factory,
		coords:    make(map[string]*turnCoordinator),
	}, nil
}

// Submit enqueues one inbound for the session handle derived from in and blocks until
// Engine.SubmitUser completes for that work item (same contract as WorkerPool.SubmitUser).
func (h *TurnHub) Submit(parentCtx context.Context, in bus.InboundMessage) error {
	if h.closed.Load() {
		return fmt.Errorf("session: turn hub closed")
	}
	handle := SessionHandle{Source: in.ClientID, SessionKey: InboundSessionKey(in)}
	c := h.coordFor(handle)

	if h.policy == TurnPolicyInsert {
		ok, err := c.tryInject(parentCtx, in)
		if ok {
			return err
		}
	}

	done := make(chan error, 1)
	w := &inboundWork{ctx: parentCtx, in: in, done: done}
	c.shutdownMu.Lock()
	stopped := c.mailClosed
	c.shutdownMu.Unlock()
	if stopped {
		return fmt.Errorf("session: turn coordinator stopped")
	}
	select {
	case c.mail <- w:
	case <-parentCtx.Done():
		return parentCtx.Err()
	case <-h.parentCtx.Done():
		return h.parentCtx.Err()
	}
	select {
	case err := <-done:
		return err
	case <-parentCtx.Done():
		return parentCtx.Err()
	case <-h.parentCtx.Done():
		return h.parentCtx.Err()
	}
}

// CancelInflightTurn hard-aborts the active turn via context cancel (e.g. /stop). Preempt policy alone does not call this.
func (h *TurnHub) CancelInflightTurn(handle SessionHandle) {
	if h.closed.Load() {
		return
	}
	h.mu.Lock()
	c := h.coords[handle.key()]
	h.mu.Unlock()
	if c != nil {
		c.cancelRunning()
	}
}

// Close shuts down all session actors; do not call Submit after Close.
func (h *TurnHub) Close() {
	h.closeMu.Lock()
	defer h.closeMu.Unlock()
	if !h.closed.CompareAndSwap(false, true) {
		return
	}
	h.mu.Lock()
	list := make([]*turnCoordinator, 0, len(h.coords))
	for _, c := range h.coords {
		list = append(list, c)
	}
	h.mu.Unlock()
	for _, c := range list {
		c.shutdownMail()
	}
	for _, c := range list {
		c.wg.Wait()
	}
}

func (h *TurnHub) coordFor(handle SessionHandle) *turnCoordinator {
	key := handle.key()
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.coords[key]; ok {
		return c
	}
	c := newTurnCoordinator(handle, h.factory, h.policy)
	h.coords[key] = c
	return c
}

type inboundWork struct {
	ctx  context.Context
	in   bus.InboundMessage
	done chan error
}

type turnCoordinator struct {
	handle  SessionHandle
	factory func(SessionHandle) (*Engine, error)
	policy  TurnPolicy

	mail       chan *inboundWork
	shutdownMu sync.Mutex
	mailClosed bool

	wg sync.WaitGroup

	mu         sync.Mutex
	turnCancel context.CancelFunc
	injectBuf  []string
}

func newTurnCoordinator(handle SessionHandle, factory func(SessionHandle) (*Engine, error), policy TurnPolicy) *turnCoordinator {
	c := &turnCoordinator{
		handle:  handle,
		factory: factory,
		policy:  policy,
		mail:    make(chan *inboundWork, turnHubMailCap),
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.loop()
	}()
	return c
}

func (c *turnCoordinator) shutdownMail() {
	c.shutdownMu.Lock()
	defer c.shutdownMu.Unlock()
	if c.mailClosed {
		return
	}
	c.mailClosed = true
	close(c.mail)
}

func (c *turnCoordinator) loop() {
	for w := range c.mail {
		if w == nil {
			continue
		}
		c.runOne(w)
	}
}

func (c *turnCoordinator) cancelRunning() {
	c.mu.Lock()
	fn := c.turnCancel
	c.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (c *turnCoordinator) tryInject(parentCtx context.Context, in bus.InboundMessage) (handled bool, err error) {
	if !injectableInboundText(in) {
		return false, nil
	}
	text := strings.TrimSpace(in.Content)
	c.mu.Lock()
	if c.turnCancel == nil {
		c.mu.Unlock()
		return false, nil
	}
	c.injectBuf = append(c.injectBuf, text)
	c.mu.Unlock()

	eng, ferr := c.factory(c.handle)
	if ferr != nil {
		return true, ferr
	}
	eng.ApplyInboundMessageStatus(parentCtx, in, bus.StatusProcessing)
	eng.ApplyInboundMessageStatus(context.WithoutCancel(parentCtx), in, bus.StatusCompleted)
	slog.Debug("session.turn_hub.inject", "session", StableSessionID(c.handle), "chars", len(text))
	return true, nil
}

func (c *turnCoordinator) drainInjectToMessages(msgs *[]*schema.Message) error {
	c.mu.Lock()
	lines := c.injectBuf
	c.injectBuf = nil
	c.mu.Unlock()
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		*msgs = append(*msgs, schema.UserMessage(line))
	}
	return nil
}

func (c *turnCoordinator) runOne(w *inboundWork) {
	turnCtx, cancel := context.WithCancel(w.ctx)
	c.mu.Lock()
	c.turnCancel = cancel
	c.mu.Unlock()

	defer func() {
		cancel()
		c.mu.Lock()
		c.turnCancel = nil
		c.mu.Unlock()
	}()

	eng, err := c.factory(c.handle)
	if err != nil {
		w.done <- err
		return
	}
	if c.policy == TurnPolicyInsert {
		eng.BeforeChatModel = func(ctx context.Context, step int, msgs *[]*schema.Message) error {
			return c.drainInjectToMessages(msgs)
		}
		defer func() { eng.BeforeChatModel = nil }()
	}

	err = eng.SubmitUser(turnCtx, w.in)
	select {
	case w.done <- err:
	default:
	}
}
