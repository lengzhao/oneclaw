package workflow

// Workflow is workflow_spec_version: 2 document.
type Workflow struct {
	SpecVersion int               `yaml:"workflow_spec_version"`
	ID          string            `yaml:"id"`
	Description string            `yaml:"description,omitempty"`
	Defaults    map[string]any    `yaml:"defaults,omitempty"`
	Meta        map[string]any    `yaml:"meta,omitempty"`
	Nodes       map[string]Node   `yaml:"nodes"`
	End         string            `yaml:"end,omitempty"`
}

// Node is one workflow node in v2.
type Node struct {
	Use       string         `yaml:"use"`
	AgentType string         `yaml:"agent_type,omitempty"`
	Input     string         `yaml:"input,omitempty"`
	Prompt    string         `yaml:"prompt,omitempty"`
	DependsOn []string       `yaml:"depends_on,omitempty"`
	Async     bool           `yaml:"async,omitempty"`
	Params    map[string]any `yaml:"params,omitempty"`
}

type stepSugar struct {
	ID        string         `yaml:"id,omitempty"`
	Use       string         `yaml:"use"`
	AgentType string         `yaml:"agent_type,omitempty"`
	Input     string         `yaml:"input,omitempty"`
	Prompt    string         `yaml:"prompt,omitempty"`
	DependsOn []string       `yaml:"depends_on,omitempty"`
	Async     bool           `yaml:"async,omitempty"`
	HostTurn  bool           `yaml:"host_turn,omitempty"`
	Params    map[string]any `yaml:"params,omitempty"`
}

type rawDoc struct {
	SpecVersion int               `yaml:"workflow_spec_version"`
	ID          string            `yaml:"id"`
	Description string            `yaml:"description,omitempty"`
	Defaults    map[string]any    `yaml:"defaults,omitempty"`
	Meta        map[string]any    `yaml:"meta,omitempty"`
	Nodes       map[string]Node   `yaml:"nodes,omitempty"`
	End         string            `yaml:"end,omitempty"`
	Steps       []stepSugar       `yaml:"steps,omitempty"`
}
