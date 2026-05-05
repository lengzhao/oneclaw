package workflow

// WorkflowNodeResult is the standard node output contract in workflow v2.
type WorkflowNodeResult struct {
	Text  string         `json:"text,omitempty" yaml:"text,omitempty"`
	Data  map[string]any `json:"data,omitempty" yaml:"data,omitempty"`
	Error string         `json:"error,omitempty" yaml:"error,omitempty"`
}
