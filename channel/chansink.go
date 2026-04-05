package channel

import (
	"context"

	"github.com/lengzhao/oneclaw/routing"
)

type chanSink struct {
	ch chan<- routing.Record
}

func newChanSink(ch chan<- routing.Record) routing.Sink {
	return &chanSink{ch: ch}
}

func (s *chanSink) Emit(_ context.Context, r routing.Record) error {
	s.ch <- r
	return nil
}
