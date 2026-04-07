package memory

import (
	"testing"

	"github.com/lengzhao/oneclaw/rtopts"
)

func TestResolveMaintenanceModel_postTurn(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	rtopts.Set(&rtopts.Snapshot{
		MaintenanceModel:          "mini-model",
		MaintenanceScheduledModel: "cron-model",
	})
	m, o := ResolveMaintenanceModel("main", false)
	if m != "mini-model" || !o {
		t.Fatalf("got %q override=%v", m, o)
	}
}

func TestResolveMaintenanceModel_scheduledPrefersScheduledEnv(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	rtopts.Set(&rtopts.Snapshot{
		MaintenanceModel:          "mini-model",
		MaintenanceScheduledModel: "cron-model",
	})
	m, o := ResolveMaintenanceModel("main", true)
	if m != "cron-model" || !o {
		t.Fatalf("got %q override=%v", m, o)
	}
}

func TestResolveMaintenanceModel_fallbackMain(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	rtopts.Set(&rtopts.Snapshot{})
	m, o := ResolveMaintenanceModel("main-chat", true)
	if m != "main-chat" || o {
		t.Fatalf("got %q override=%v", m, o)
	}
}
