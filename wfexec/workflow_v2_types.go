package wfexec

import (
	"github.com/lengzhao/oneclaw/engine"
)

// TurnWorkflowInput is the workflow v2 public input contract.
type TurnWorkflowInput struct {
	UserPrompt string
	AgentID    string
	Runtime    *engine.RuntimeContext
}

// TurnWorkflowResult is the workflow v2 public output contract.
type TurnWorkflowResult struct {
	Assistant string
	Runtime   *engine.RuntimeContext
}
