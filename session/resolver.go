package session

import (
	"context"
	"fmt"
	"sync"

	"github.com/lengzhao/clawbridge/bus"
)

// SessionHandle identifies one logical chat (same IM source + session key).
type SessionHandle struct {
	Source     string
	SessionKey string
}

func (h SessionHandle) key() string {
	src := h.Source
	if h.SessionKey == "" {
		return src + "\x00__default__"
	}
	return src + "\x00" + h.SessionKey
}

// SessionResolver maps SessionHandle to an Engine (lazy) and serializes turns per handle.
type SessionResolver struct {
	mu       sync.Mutex
	slots    map[string]*sessionSlot
	factory  func(SessionHandle) (*Engine, error)
	onCreate func(SessionHandle, *Engine) // optional hook for tests / metrics
}

type sessionSlot struct {
	mu  sync.Mutex
	eng *Engine
}

// NewSessionResolver returns a resolver. factory must return a new *Engine for each distinct handle.
// IM integrations usually set Engine.SessionID from the handle (e.g. stable hash of instance id + SessionKey).
func NewSessionResolver(factory func(SessionHandle) (*Engine, error)) *SessionResolver {
	return &SessionResolver{
		slots:   make(map[string]*sessionSlot),
		factory: factory,
	}
}

// SetOnEngineCreated registers a callback invoked once when a slot creates an engine.
func (r *SessionResolver) SetOnEngineCreated(fn func(SessionHandle, *Engine)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onCreate = fn
}

func (r *SessionResolver) slotFor(h SessionHandle) (*sessionSlot, error) {
	k := h.key()
	r.mu.Lock()
	slot, ok := r.slots[k]
	if !ok {
		eng, err := r.factory(h)
		if err != nil {
			r.mu.Unlock()
			return nil, fmt.Errorf("session resolver: create engine: %w", err)
		}
		slot = &sessionSlot{eng: eng}
		r.slots[k] = slot
		if r.onCreate != nil {
			cb := r.onCreate
			r.mu.Unlock()
			cb(h, eng)
			return slot, nil
		}
	}
	r.mu.Unlock()
	return slot, nil
}

// EngineFor returns the engine for h, creating it on first use.
func (r *SessionResolver) EngineFor(h SessionHandle) (*Engine, error) {
	slot, err := r.slotFor(h)
	if err != nil {
		return nil, err
	}
	return slot.eng, nil
}

// SubmitUser resolves the engine for in.Channel + session key (Peer.ID) and runs one user turn.
// Same handle is processed strictly one turn at a time (FIFO).
func (r *SessionResolver) SubmitUser(ctx context.Context, in bus.InboundMessage) error {
	h := SessionHandle{Source: in.Channel, SessionKey: InboundSessionKey(in)}
	slot, err := r.slotFor(h)
	if err != nil {
		return err
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.eng.SubmitUser(ctx, in)
}
