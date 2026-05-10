package memory

import (
	"context"
	"strings"

	lzmem "github.com/lengzhao/memory"
	lzmodel "github.com/lengzhao/memory/model"
	lzservice "github.com/lengzhao/memory/service"
)

// SeedAgentMemoryItem inserts one row into agent_memory.sqlite (for integration / e2e tests).
func SeedAgentMemoryItem(layout Layout, isolateSessionID string, ns lzmodel.NamespaceType, title, content string) error {
	layout.EnsureDirs()
	db, err := getAgentMemoryGorm(layout)
	if err != nil {
		return err
	}
	sid := strings.TrimSpace(isolateSessionID)
	if sid == "" {
		sid = "default"
	}
	ctx := lzservice.WithIsolation(context.Background(), layoutStableTenantID(layout), "default", sid, DefaultRootAgentMemoryAgentID)
	svc := lzmem.NewMemoryService(db)
	_, err = svc.Remember(ctx, lzservice.RememberRequest{
		NamespaceType: ns,
		Title:         title,
		Content:       content,
		SourceType:    lzmodel.SourceTypeUser,
		Confidence:    0.95,
		Importance:    50,
	})
	return err
}
