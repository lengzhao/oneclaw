package statichttp

import "testing"

func TestResolveListenAddr(t *testing.T) {
	if got := resolveListenAddr(nil); got != defaultListenAddr {
		t.Fatalf("default: got %q", got)
	}
	if got := resolveListenAddr(map[string]any{"listen_addr": " :9999 "}); got != ":9999" {
		t.Fatalf("trim: got %q", got)
	}
}

func TestResolveStaticDir(t *testing.T) {
	if got := resolveStaticDir(map[string]any{"static_dir": " /tmp/x "}); got != "/tmp/x" {
		t.Fatalf("got %q", got)
	}
}
