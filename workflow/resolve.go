package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/catalog"
)

// ResolveWorkflowPath picks workflows/<agent>.yaml|.yml then manifest default_turn (docs/workflows-spec.md §8).
func ResolveWorkflowPath(catalogRoot, agentID string, mf *catalog.Manifest) (string, error) {
	root, err := filepath.Abs(catalogRoot)
	if err != nil {
		return "", err
	}
	try := func(base string) (string, error) {
		for _, ext := range []string{".yaml", ".yml"} {
			p := base + ext
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return ensureUnderWorkflowRoot(root, p)
			}
		}
		return "", os.ErrNotExist
	}
	ap := strings.TrimSpace(agentID)
	if ap != "" {
		p, err := try(filepath.Join(root, "workflows", ap))
		if err == nil {
			return p, nil
		}
	}
	dt := mf.ResolvedDefaultTurn()
	dt = strings.TrimSpace(dt)
	if dt == "" {
		dt = "default.turn"
	}
	p, err := try(filepath.Join(root, "workflows", dt))
	if err == nil {
		return p, nil
	}
	return "", fmt.Errorf("workflow: no workflow for agent %q and default_turn %q under %s", agentID, dt, filepath.Join(root, "workflows"))
}

func ensureUnderWorkflowRoot(rootAbs, filePath string) (string, error) {
	target, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("workflow: workflow path %q escapes catalog root", filePath)
	}
	return target, nil
}
