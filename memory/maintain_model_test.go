package memory

import (
	"testing"
)

func TestResolveMaintenanceModel_postTurn(t *testing.T) {
	t.Setenv("ONCLAW_MAINTENANCE_MODEL", "mini-model")
	t.Setenv("ONCLAW_MAINTENANCE_SCHEDULED_MODEL", "cron-model")
	m, o := ResolveMaintenanceModel("main", false)
	if m != "mini-model" || !o {
		t.Fatalf("got %q override=%v", m, o)
	}
}

func TestResolveMaintenanceModel_scheduledPrefersScheduledEnv(t *testing.T) {
	t.Setenv("ONCLAW_MAINTENANCE_MODEL", "mini-model")
	t.Setenv("ONCLAW_MAINTENANCE_SCHEDULED_MODEL", "cron-model")
	m, o := ResolveMaintenanceModel("main", true)
	if m != "cron-model" || !o {
		t.Fatalf("got %q override=%v", m, o)
	}
}

func TestResolveMaintenanceModel_fallbackMain(t *testing.T) {
	t.Setenv("ONCLAW_MAINTENANCE_MODEL", "")
	t.Setenv("ONCLAW_MAINTENANCE_SCHEDULED_MODEL", "")
	m, o := ResolveMaintenanceModel("main-chat", true)
	if m != "main-chat" || o {
		t.Fatalf("got %q override=%v", m, o)
	}
}
