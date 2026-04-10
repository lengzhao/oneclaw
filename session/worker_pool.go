package session

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"

	"github.com/lengzhao/clawbridge/bus"
)

const defaultWorkerCount = 8

// WorkerPool runs session turns on a fixed number of worker goroutines.
// Each inbound message is routed to worker hash(session)%N so the same SessionHandle
// is always processed on the same worker (FIFO per shard). A fresh Engine is built
// per job via factory and discarded after SubmitUser — no unbounded Engine map.
type WorkerPool struct {
	chans   []chan submitWork
	factory func(SessionHandle) (*Engine, error)
	wg      sync.WaitGroup
}

type submitWork struct {
	ctx context.Context
	in  bus.InboundMessage
	h   SessionHandle
	err chan error
}

// NewWorkerPool starts n worker goroutines. If n < 1, defaultWorkerCount is used.
func NewWorkerPool(n int, factory func(SessionHandle) (*Engine, error)) (*WorkerPool, error) {
	if factory == nil {
		return nil, fmt.Errorf("session: worker pool: nil factory")
	}
	if n < 1 {
		n = defaultWorkerCount
	}
	wp := &WorkerPool{
		chans:   make([]chan submitWork, n),
		factory: factory,
	}
	for i := 0; i < n; i++ {
		wp.chans[i] = make(chan submitWork, 256)
		wp.wg.Add(1)
		go wp.runWorker(i, wp.chans[i])
	}
	return wp, nil
}

func (wp *WorkerPool) runWorker(workerID int, ch <-chan submitWork) {
	defer wp.wg.Done()
	for w := range ch {
		sid := StableSessionID(w.h)
		slog.Info("session.worker.job_start",
			"worker_id", workerID,
			"session_id", sid,
			"channel", w.h.Source,
			"session_key", w.h.SessionKey,
		)
		eng, factoryErr := wp.factory(w.h)
		if factoryErr != nil {
			slog.Info("session.worker.job_end",
				"worker_id", workerID,
				"session_id", sid,
				"channel", w.h.Source,
				"session_key", w.h.SessionKey,
				"err", factoryErr,
			)
			w.err <- factoryErr
			continue
		}
		submitErr := eng.SubmitUser(w.ctx, w.in)
		endArgs := []any{
			"worker_id", workerID,
			"session_id", sid,
			"channel", w.h.Source,
			"session_key", w.h.SessionKey,
		}
		if submitErr != nil {
			endArgs = append(endArgs, "err", submitErr)
		}
		slog.Info("session.worker.job_end", endArgs...)
		w.err <- submitErr
	}
}

// sessionShardIndex maps a stable session key string to [0, n).
func sessionShardIndex(key string, n int) int {
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % uint32(n))
}

// SubmitUser enqueues one turn on the shard for this session and waits for completion.
func (wp *WorkerPool) SubmitUser(ctx context.Context, in bus.InboundMessage) error {
	h := SessionHandle{Source: in.Channel, SessionKey: InboundSessionKey(in)}
	idx := sessionShardIndex(h.key(), len(wp.chans))
	errCh := make(chan error, 1)
	w := submitWork{ctx: ctx, in: in, h: h, err: errCh}
	select {
	case wp.chans[idx] <- w:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops workers after draining each shard channel. Do not call SubmitUser after Close.
func (wp *WorkerPool) Close() {
	for _, ch := range wp.chans {
		close(ch)
	}
	wp.wg.Wait()
}
