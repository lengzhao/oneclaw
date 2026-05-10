package memory

import (
	"context"
	"strings"
	"testing"

	lzmem "github.com/lengzhao/memory"
	lzmodel "github.com/lengzhao/memory/model"
	lzservice "github.com/lengzhao/memory/service"
)

func TestTruncateRecallDisplay(t *testing.T) {
	if got := truncateRecallDisplay("hello", 10); got != "hello" {
		t.Fatalf("short string: %q", got)
	}
	long := strings.Repeat("あ", 20)
	got := truncateRecallDisplay(long, 5)
	if len([]rune(got)) != 6 { // 5 runes + ellipsis
		t.Fatalf("want 6 runes, got %q len runes %d", got, len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix: %q", got)
	}
}

func TestSelectRecall_findsRememberedItem(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	lay.EnsureDirs()
	sess := "session-recall-test"
	ctx := lzservice.WithIsolation(context.Background(), layoutStableTenantID(lay), "default", sess, DefaultRootAgentMemoryAgentID)
	db, err := getAgentMemoryGorm(lay)
	if err != nil {
		t.Fatal(err)
	}
	svc := lzmem.NewMemoryService(db)
	marker := "zebrarecall_selectrecall_unique_token_42"
	if _, err := svc.Remember(ctx, lzservice.RememberRequest{
		NamespaceType: lzmodel.NamespaceTypeKnowledge,
		Title:         "recall test",
		Content:       "body text " + marker + " tail",
		SourceType:    lzmodel.SourceTypeUser,
		Confidence:    0.95,
		Importance:    50,
	}); err != nil {
		t.Fatal(err)
	}
	body, st := SelectRecall(lay, sess, marker, nil, 12_000)
	if !strings.Contains(body, marker) {
		t.Fatalf("expected marker in recall:\n%s", body)
	}
	if !strings.Contains(body, "Attachment: relevant_memories") {
		t.Fatalf("expected attachment header:\n%s", body)
	}
	if st == nil || len(st.SurfacedPaths) == 0 {
		t.Fatal("expected UpdatedRecall to record surfaced memory ids")
	}
}

func TestSelectRecall_wrongSessionEmpty_forTransient(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	lay.EnsureDirs()
	ctx := lzservice.WithIsolation(context.Background(), layoutStableTenantID(lay), "default", "session-a", DefaultRootAgentMemoryAgentID)
	db, err := getAgentMemoryGorm(lay)
	if err != nil {
		t.Fatal(err)
	}
	svc := lzmem.NewMemoryService(db)
	marker := "session_isolation_marker_qwerty"
	// Transient namespace includes session in the DB key; knowledge/profile/action are tenant+user scoped only.
	if _, err := svc.Remember(ctx, lzservice.RememberRequest{
		NamespaceType: lzmodel.NamespaceTypeTransient,
		Title:         "iso",
		Content:       marker,
		SourceType:    lzmodel.SourceTypeUser,
		Confidence:    0.95,
		Importance:    50,
	}); err != nil {
		t.Fatal(err)
	}
	body, _ := SelectRecall(lay, "session-b", marker, nil, 12_000)
	if body != "" {
		t.Fatalf("expected empty recall for wrong session, got:\n%s", body)
	}
}

func TestSelectRecall_secondCallDedupesSurfacedIDs(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	lay.EnsureDirs()
	sess := "session-dedupe"
	ctx := lzservice.WithIsolation(context.Background(), layoutStableTenantID(lay), "default", sess, DefaultRootAgentMemoryAgentID)
	db, err := getAgentMemoryGorm(lay)
	if err != nil {
		t.Fatal(err)
	}
	svc := lzmem.NewMemoryService(db)
	shared := "overlap_shared_keyword_xyzzy"
	if _, err := svc.Remember(ctx, lzservice.RememberRequest{
		NamespaceType: lzmodel.NamespaceTypeKnowledge,
		Title:         "first",
		Content:       "aaa " + shared + " one",
		SourceType:    lzmodel.SourceTypeUser,
		Confidence:    0.95,
		Importance:    50,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Remember(ctx, lzservice.RememberRequest{
		NamespaceType: lzmodel.NamespaceTypeKnowledge,
		Title:         "second",
		Content:       "bbb " + shared + " two",
		SourceType:    lzmodel.SourceTypeUser,
		Confidence:    0.95,
		Importance:    50,
	}); err != nil {
		t.Fatal(err)
	}
	body1, st1 := SelectRecall(lay, sess, shared, nil, 12_000)
	if !strings.Contains(body1, shared) {
		t.Fatalf("first recall missing keyword:\n%s", body1)
	}
	body2, _ := SelectRecall(lay, sess, shared, st1, 12_000)
	if body2 != "" {
		t.Fatalf("expected empty second recall after dedupe, got:\n%s", body2)
	}
}

func TestSelectRecall_mergesScheduledMaintainIsolation(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	lay.EnsureDirs()
	ctxSched := lzservice.WithIsolation(context.Background(), layoutStableTenantID(lay), "default", ScheduledMaintainIsolationSessionID, DefaultRootAgentMemoryAgentID)
	db, err := getAgentMemoryGorm(lay)
	if err != nil {
		t.Fatal(err)
	}
	svc := lzmem.NewMemoryService(db)
	marker := "scheduled_scope_merge_marker_unique_zx"
	if _, err := svc.Remember(ctxSched, lzservice.RememberRequest{
		NamespaceType: lzmodel.NamespaceTypeTransient,
		Title:         "sched",
		Content:       marker,
		SourceType:    lzmodel.SourceTypeUser,
		Confidence:    0.95,
		Importance:    50,
	}); err != nil {
		t.Fatal(err)
	}
	body, _ := SelectRecall(lay, "user-session-normal", marker, nil, 12_000)
	if !strings.Contains(body, marker) {
		t.Fatalf("expected merged recall from scheduled isolation, got:\n%s", body)
	}
}
