package turnhub

import (
	"context"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	clawbridge "github.com/lengzhao/clawbridge"

	"github.com/lengzhao/oneclaw/runner"
)

// Processor runs one inbound turn; errors are ignored by the hub (log inside the processor).
type Processor func(ctx context.Context, msg clawbridge.InboundMessage) error

type inboundJob struct {
	msg clawbridge.InboundMessage
	pol Policy
}

// Hub multiplexes inbound turns per SessionHandle with serial/insert/preempt queue policies.
type Hub struct {
	root context.Context
	proc Processor

	mu          sync.Mutex
	workers     map[string]chan inboundJob
	maxBuf      int
	turnTimeout time.Duration
	onDropped   func(context.Context, clawbridge.InboundMessage) error

	waitMu   sync.Mutex
	inFlight int
}

// NewHub constructs a hub; parent ctx cancels all session workers.
func NewHub(parent context.Context, proc Processor, opts ...HubOption) *Hub {
	if proc == nil {
		proc = func(context.Context, clawbridge.InboundMessage) error { return nil }
	}
	h := &Hub{
		root:    parent,
		proc:    proc,
		workers: make(map[string]chan inboundJob),
		maxBuf:  256,
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Enqueue schedules msg for handle using pol. Schedule-driven turns should use PolicySerial (reference-architecture section 2.6).
func (h *Hub) Enqueue(handle SessionHandle, pol Policy, msg clawbridge.InboundMessage) error {
	select {
	case <-h.root.Done():
		return h.root.Err()
	default:
	}

	ch := h.workerChan(handle)
	job := inboundJob{msg: msg, pol: pol}
	for {
		select {
		case <-h.root.Done():
			return h.root.Err()
		case ch <- job:
			return nil
		default:
			// Buffer was full; prefer recv (drop oldest) or send if another goroutine drained ch.
			select {
			case <-h.root.Done():
				return h.root.Err()
			case ch <- job:
				return nil
			case dropped := <-ch:
				h.invokeDropped(dropped)
			default:
				// Rare race: fullness flipped between outer default and here; yield and retry.
				runtime.Gosched()
			}
		}
	}
}

func (h *Hub) workerChan(handle SessionHandle) chan inboundJob {
	key := handle.String()
	h.mu.Lock()
	defer h.mu.Unlock()
	ch, ok := h.workers[key]
	if !ok {
		ch = make(chan inboundJob, h.maxBuf)
		h.workers[key] = ch
		go h.runSession(ch)
	}
	return ch
}

func (h *Hub) invokeDropped(job inboundJob) {
	m := job.msg
	if h.onDropped == nil {
		slog.Warn("turnhub dropped queued inbound (mailbox full)",
			"client_id", m.ClientID,
			"session_id", m.SessionID,
			"message_id", m.MessageID,
		)
		return
	}
	if err := h.onDropped(h.root, m); err != nil {
		slog.Warn("turnhub onDropped",
			"err", err,
			"client_id", m.ClientID,
			"session_id", m.SessionID,
			"message_id", m.MessageID,
		)
	}
}

func (h *Hub) runSession(ch chan inboundJob) {
	var pending []clawbridge.InboundMessage
	for {
		if len(pending) == 0 {
			select {
			case <-h.root.Done():
				return
			case job := <-ch:
				pending = []clawbridge.InboundMessage{job.msg}
			}
		}

		msg := pending[0]
		pending = pending[1:]

		procDone := make(chan struct{})
		go func(m clawbridge.InboundMessage) {
			defer close(procDone)
			h.beginTurn()
			defer h.endTurn()
			turnCtx := h.root
			if h.turnTimeout > 0 {
				var cancel context.CancelFunc
				turnCtx, cancel = context.WithTimeout(h.root, h.turnTimeout)
				defer cancel()
			}
			if err := h.proc(turnCtx, m); err != nil {
				args := []any{
					"err", err,
					"client_id", m.ClientID,
					"session_id", m.SessionID,
					"message_id", m.MessageID,
				}
				if m.Metadata != nil {
					if c := strings.TrimSpace(m.Metadata[runner.InboundMetaCorrelation]); c != "" {
						args = append(args, "correlation_id", c)
					}
					if j := strings.TrimSpace(m.Metadata[runner.InboundMetaScheduleJob]); j != "" {
						args = append(args, "schedule_job_id", j)
					}
				}
				slog.Error("turnhub turn failed", args...)
			}
		}(msg)

	drain:
		for {
			select {
			case <-h.root.Done():
				return
			case <-procDone:
				break drain
			case job := <-ch:
				pending = mergeWhileRunning(pending, job)
			}
		}
	}
}

func mergeWhileRunning(pending []clawbridge.InboundMessage, job inboundJob) []clawbridge.InboundMessage {
	switch job.pol {
	case PolicySerial:
		return append(pending, job.msg)
	case PolicyInsert, PolicyPreempt:
		return []clawbridge.InboundMessage{job.msg}
	default:
		return append(pending, job.msg)
	}
}

func (h *Hub) beginTurn() {
	h.waitMu.Lock()
	h.inFlight++
	h.waitMu.Unlock()
}

func (h *Hub) endTurn() {
	h.waitMu.Lock()
	h.inFlight--
	h.waitMu.Unlock()
}

// WaitIdle blocks until no turn is in flight (for tests / shutdown sequencing).
func (h *Hub) WaitIdle(ctx context.Context) error {
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			h.waitMu.Lock()
			n := h.inFlight
			h.waitMu.Unlock()
			if n == 0 {
				return nil
			}
		}
	}
}
