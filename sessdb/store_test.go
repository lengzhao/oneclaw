package sessdb

import (
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/session"
)

func TestRecallRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.sqlite")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	h := session.SessionHandle{Source: "feishu", SessionKey: "t1"}
	sid := session.StableSessionID(h)
	b := NewRecallBridge(st, h)

	_, err = b.LoadRecall(sid)
	if err != nil {
		t.Fatal(err)
	}

	if err := b.SaveRecall(sid, memory.RecallState{
		SurfacedPaths: map[string]struct{}{"/a/b.md": {}},
		SurfacedBytes: 42,
	}); err != nil {
		t.Fatal(err)
	}
	st2, err := b.LoadRecall(sid)
	if err != nil {
		t.Fatal(err)
	}
	if st2.SurfacedBytes != 42 {
		t.Fatalf("surfaced bytes: %d", st2.SurfacedBytes)
	}
	if _, ok := st2.SurfacedPaths["/a/b.md"]; !ok {
		t.Fatal("missing path")
	}
}
