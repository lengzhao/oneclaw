package wfexec

import (
	"fmt"

	"github.com/lengzhao/oneclaw/engine"
)

func coalesceRTX(in map[string]any) (*engine.RuntimeContext, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("wfexec: empty merge map")
	}
	for _, v := range in {
		rtx, ok := v.(*engine.RuntimeContext)
		if ok && rtx != nil {
			return rtx, nil
		}
	}
	return nil, fmt.Errorf("wfexec: merge map missing *engine.RuntimeContext")
}
