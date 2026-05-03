package workflow

// Workflow is a normalized document (graph filled; defaults merged into nodes).
type Workflow struct {
	SpecVersion int               `yaml:"workflow_spec_version"`
	ID          string            `yaml:"id"`
	Description string            `yaml:"description,omitempty"`
	Defaults    map[string]any    `yaml:"defaults,omitempty"`
	Meta        map[string]any    `yaml:"meta,omitempty"`
	Graph Graph `yaml:"graph"`
}

// Graph is the DAG model (§4).
type Graph struct {
	Entry string          `yaml:"entry"`
	Nodes map[string]Node `yaml:"nodes"`
	Edges []Edge          `yaml:"edges"`
}

// Node is one workflow vertex (§5).
type Node struct {
	Use    string         `yaml:"use"`
	Params map[string]any `yaml:"params,omitempty"`
	Async  bool           `yaml:"async,omitempty"`
}

// Edge is a directed edge (§4.2).
type Edge struct {
	From   string `yaml:"from"`
	To     string `yaml:"to"`
	Branch *bool  `yaml:"branch,omitempty"`
}

type stepSugar struct {
	Use    string         `yaml:"use"`
	ID     string         `yaml:"id,omitempty"`
	Params map[string]any `yaml:"params,omitempty"`
	Async  bool           `yaml:"async,omitempty"`
}

type rawDoc struct {
	SpecVersion int               `yaml:"workflow_spec_version"`
	ID          string            `yaml:"id"`
	Description string            `yaml:"description,omitempty"`
	Defaults    map[string]any    `yaml:"defaults,omitempty"`
	Meta        map[string]any    `yaml:"meta,omitempty"`
	Graph       *Graph            `yaml:"graph,omitempty"`
	Steps       []stepSugar       `yaml:"steps,omitempty"`
}
