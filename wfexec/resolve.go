package wfexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/catalog"
)

// ResolveWorkflowPath picks workflows/<agent>.yaml|.yml then manifest default_turn (docs/workflows-spec §3).
func ResolveWorkflowPath(catalogRoot, agentID string, mf *catalog.Manifest) (string, error) {
	root, err := filepath.Abs(catalogRoot)
	if err != nil {
		return "", err
	}
	try := func(base string) (string, error) {
		for _, ext := range []string{".yaml", ".yml"} {
			p := base + ext
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return ensureUnderRoot(root, p)
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
	dt := "default.turn"
	if mf != nil {
		dt = mf.ResolvedDefaultTurn()
	}
	p, err := try(filepath.Join(root, "workflows", strings.TrimSpace(dt)))
	if err == nil {
		return p, nil
	}
	return "", fmt.Errorf("wfexec: no workflow for agent %q and default_turn %q under %s", agentID, dt, filepath.Join(root, "workflows"))
}

func ensureUnderRoot(rootAbs, filePath string) (string, error) {
	target, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("wfexec: workflow path %q escapes catalog root", filePath)
	}
	return target, nil
}
