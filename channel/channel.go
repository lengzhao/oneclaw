package channel

import (
	"context"
	"fmt"
	"sync"

	"github.com/lengzhao/oneclaw/routing"
)

// Registry holds connector Specs in registration order.
type Registry struct {
	mu    sync.Mutex
	specs []Spec
	keys  map[string]struct{}
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{keys: make(map[string]struct{})}
}

// RegisterSpec adds a spec. Key must be unique; panics on duplicate Key.
func (r *Registry) RegisterSpec(s Spec) {
	if s.Key == "" {
		panic("channel: empty Spec.Key")
	}
	if s.New == nil {
		panic("channel: nil Spec.New for " + s.Key)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.keys[s.Key]; dup {
		panic("channel: duplicate register " + s.Key)
	}
	r.keys[s.Key] = struct{}{}
	r.specs = append(r.specs, s)
}

type startInstance struct {
	spec   Spec
	id     string
	params map[string]any
}

func cloneParams(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func instancesToStart(specs []Spec, boot Bootstrap) ([]startInstance, error) {
	if boot.Config != nil {
		chs := boot.Config.Channels()
		if len(chs) > 0 {
			var out []startInstance
			for _, ch := range chs {
				if ch.ID == "" || ch.Type == "" {
					return nil, fmt.Errorf("channel: channels entry needs non-empty id and type")
				}
				var found *Spec
				for i := range specs {
					if specs[i].Key == ch.Type {
						found = &specs[i]
						break
					}
				}
				if found == nil {
					return nil, fmt.Errorf("channel: unknown channel type %q (no Spec.Key)", ch.Type)
				}
				out = append(out, startInstance{spec: *found, id: ch.ID, params: cloneParams(ch.Params)})
			}
			return out, nil
		}
	}
	out := make([]startInstance, 0, len(specs))
	for _, s := range specs {
		out = append(out, startInstance{spec: s, id: s.Key, params: nil})
	}
	return out, nil
}

// StartAll builds IO chans, registers routing.Sink per instance id, runs submitLoop + Connector.Run.
func (r *Registry) StartAll(ctx context.Context, boot Bootstrap) ([]Connector, error) {
	if boot.Engine == nil {
		return nil, fmt.Errorf("channel: Bootstrap.Engine is nil")
	}
	r.mu.Lock()
	specs := append([]Spec(nil), r.specs...)
	r.mu.Unlock()

	instances, err := instancesToStart(specs, boot)
	if err != nil {
		return nil, err
	}

	eng := boot.Engine
	var ran []Connector
	for _, inst := range instances {
		s := inst.spec
		inCh := make(chan InboundTurn, 8)
		outCh := make(chan routing.Record, 64)
		routing.RegisterDefaultSink(inst.id, newChanSink(outCh))

		runCtx, cancel := context.WithCancel(ctx)
		routerDone := make(chan struct{})
		go func() {
			defer close(routerDone)
			submitLoop(runCtx, inCh, eng, inst.id)
		}()

		cn, err := s.New(runCtx, connectorConfig(boot, inst.id, inst.params))
		if err != nil {
			cancel()
			<-routerDone
			close(inCh)
			close(outCh)
			return nil, fmt.Errorf("channel %q (%q): construct: %w", s.Key, inst.id, err)
		}

		io := IO{InboundChan: inCh, OutboundChan: outCh}

		if err := cn.Run(runCtx, io); err != nil {
			cancel()
			<-routerDone
			close(inCh)
			close(outCh)
			return nil, fmt.Errorf("channel %q (%q): run: %w", s.Key, inst.id, err)
		}
		cancel()
		<-routerDone
		close(inCh)
		close(outCh)
		ran = append(ran, cn)
	}
	return ran, nil
}
