// Package toolhost defines narrow interfaces for tool registries so engine/wfexec
// do not depend on concrete registry implementations (phase 4 hardening).
package toolhost

import "github.com/cloudwego/eino/components/tool"

// Registry is the parent view needed to derive child tool subsets (FR-AGT-03).
type Registry interface {
	Names() []string
	WorkspaceRoot() string
	FilterByNames(allow []string) ([]tool.BaseTool, error)
}
