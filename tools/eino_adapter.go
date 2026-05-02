package tools

import (
	"context"
	"encoding/json"

	"github.com/lengzhao/oneclaw/toolctx"
)

// EinoBinding is a provider-neutral bridge object shaped for Eino runtime wiring.
// It intentionally avoids importing Eino packages in this step, so migration can
// proceed incrementally while preserving compile/test stability.
type EinoBinding struct {
	Name            string
	Description     string
	ParametersJSON  json.RawMessage
	ConcurrencySafe bool
	Execute         func(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error)
}

// EinoBindings returns stable-name-ordered tool bindings for Eino runtime adapters.
func (r *Registry) EinoBindings() []EinoBinding {
	descs := r.Descriptors()
	out := make([]EinoBinding, 0, len(descs))
	for _, d := range descs {
		out = append(out, EinoBinding{
			Name:            d.Name,
			Description:     d.Description,
			ParametersJSON:  marshalParameters(d.Parameters),
			ConcurrencySafe: d.ConcurrencySafe,
			Execute:         d.Execute,
		})
	}
	return out
}

func marshalParameters(p any) json.RawMessage {
	if p == nil {
		return json.RawMessage("{}")
	}
	b, err := json.Marshal(p)
	if err != nil || len(b) == 0 {
		return json.RawMessage("{}")
	}
	return b
}

