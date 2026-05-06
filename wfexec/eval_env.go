package wfexec

import (
	"fmt"
)

// EvalEnv binds workflow template rendering to the current compile/render graph snapshot.
type EvalEnv struct {
	State      *compileState
	GraphInput map[string]any
}

// Render expands $start / $nodes.* / $runtime.* refs in src against GraphInput.
func (e *EvalEnv) Render(src string) (string, error) {
	if e == nil || e.State == nil {
		return "", fmt.Errorf("wfexec: template eval: missing state")
	}
	return renderNodeTemplate(e.State, e.GraphInput, src)
}
