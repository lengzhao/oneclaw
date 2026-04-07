package mcpclient

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/tools"
)

// RegisterIfEnabled loads MCP servers from merged config, registers tools on reg, and returns
// a Manager to Close on shutdown plus a short system-prompt note. When MCP is off or misconfigured, mgr is nil.
func RegisterIfEnabled(ctx context.Context, r *config.Resolved, reg *tools.Registry, cwd string) (mgr *Manager, systemNote string, err error) {
	if r == nil || reg == nil || !r.MCPEnabled() {
		return nil, "", nil
	}
	mcpBlock := r.MCP()
	if len(mcpBlock.Servers) == 0 {
		slog.Warn("mcp.register", "msg", "mcp.enabled true but no servers")
		return nil, "", nil
	}
	hasEnabled := false
	for _, s := range mcpBlock.Servers {
		if s.Enabled {
			hasEnabled = true
			break
		}
	}
	if !hasEnabled {
		slog.Warn("mcp.register", "msg", "no server has enabled: true")
		return nil, "", nil
	}

	servers := make(map[string]ServerConfig)
	for name, sf := range mcpBlock.Servers {
		servers[name] = ServerConfig{
			Enabled:  sf.Enabled,
			Command:  sf.Command,
			Args:     sf.Args,
			Env:      sf.Env,
			EnvFile:  sf.EnvFile,
			Type:     sf.Type,
			URL:      sf.URL,
			Headers:  sf.Headers,
		}
	}

	mgr = NewManager()
	if loadErr := mgr.Load(ctx, cwd, servers); loadErr != nil {
		_ = mgr.Close()
		return nil, "", loadErr
	}

	maxInline := mcpBlock.MaxInlineTextRunes
	var sb strings.Builder
	var regErrs []error
	for sname, conn := range mgr.GetServers() {
		if len(conn.Tools) > 0 {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			fmt.Fprintf(&sb, "%s (%d tools)", sname, len(conn.Tools))
		}
		for _, mt := range conn.Tools {
			t := NewTool(mgr, sname, mt, maxInline)
			if e := reg.Register(t); e != nil {
				slog.Warn("mcp.register_tool", "name", t.Name(), "err", e)
				regErrs = append(regErrs, e)
				continue
			}
		}
	}

	note := strings.TrimSpace(sb.String())
	if note != "" {
		note = "External MCP servers (prefix `mcp_*` tools): " + note + "."
	}
	if len(regErrs) > 0 {
		slog.Warn("mcp.register", "skipped_duplicates", len(regErrs))
	}
	return mgr, note, nil
}
