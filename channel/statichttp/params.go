package statichttp

import (
	"strings"
)

const (
	defaultListenAddr = "127.0.0.1:8765"
)

func paramString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func resolveListenAddr(m map[string]any) string {
	if s := paramString(m, "listen_addr"); s != "" {
		return s
	}
	return defaultListenAddr
}

func resolveStaticDir(m map[string]any) string {
	return paramString(m, "static_dir")
}
