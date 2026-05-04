package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/test/e2e/dotenv"
)

func TestMain(m *testing.M) {
	tryLoadE2EDotenv()
	os.Exit(m.Run())
}

func tryLoadE2EDotenv() {
	candidates := []string{
		strings.TrimSpace(os.Getenv("ONECLAW_E2E_DOTENV")),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "test", "e2e", ".env"),
			filepath.Join(wd, ".env"),
		)
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		if err := dotenv.Load(p); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "oneclaw e2e: dotenv %s: %v\n", p, err)
			os.Exit(1)
		}
		return
	}
}
