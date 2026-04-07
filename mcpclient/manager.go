// Package mcpclient connects to external MCP servers (stdio / streamable HTTP) and exposes CallTool.
package mcpclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func loadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open env file: %w", err)
	}
	defer file.Close()

	envVars := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: %s", lineNum, line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("invalid format at line %d: empty key", lineNum)
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		envVars[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read env file: %w", err)
	}
	return envVars, nil
}

// ServerConfig describes one MCP server (mirrors config.MCPServerFile).
type ServerConfig struct {
	Enabled  bool
	Command  string
	Args     []string
	Env      map[string]string
	EnvFile  string
	Type     string
	URL      string
	Headers  map[string]string
}

// ServerConnection is one live MCP session.
type ServerConnection struct {
	Name    string
	Client  *mcp.Client
	Session *mcp.ClientSession
	Tools   []*mcp.Tool
}

// Manager holds MCP server sessions for the process.
type Manager struct {
	servers map[string]*ServerConnection
	mu      sync.RWMutex
	closed  atomic.Bool
	wg      sync.WaitGroup
}

// NewManager creates an empty manager.
func NewManager() *Manager {
	return &Manager{servers: make(map[string]*ServerConnection)}
}

// Load connects to all enabled servers in parallel. If every enabled server fails, returns an error.
func (m *Manager) Load(ctx context.Context, workspace string, servers map[string]ServerConfig) error {
	if len(servers) == 0 {
		slog.Info("mcp.load", "msg", "no MCP servers configured")
		return nil
	}

	var enabledNames []string
	for name, sc := range servers {
		if sc.Enabled {
			enabledNames = append(enabledNames, name)
		}
	}
	if len(enabledNames) == 0 {
		slog.Info("mcp.load", "msg", "no enabled MCP servers")
		return nil
	}

	slog.Info("mcp.load", "servers", len(enabledNames))

	var wg sync.WaitGroup
	errs := make(chan error, len(enabledNames))

	for _, name := range enabledNames {
		sc := servers[name]
		wg.Add(1)
		go func(name string, cfg ServerConfig) {
			defer wg.Done()
			cfgCopy := cfg
			if cfgCopy.EnvFile != "" && !filepath.IsAbs(cfgCopy.EnvFile) {
				if workspace == "" {
					errs <- fmt.Errorf("mcp server %q: relative env_file %q needs non-empty workspace", name, cfgCopy.EnvFile)
					return
				}
				cfgCopy.EnvFile = filepath.Join(workspace, cfgCopy.EnvFile)
			}
			if err := m.ConnectServer(ctx, name, cfgCopy); err != nil {
				slog.Error("mcp.connect", "server", name, "err", err)
				errs <- fmt.Errorf("server %s: %w", name, err)
			}
		}(name, sc)
	}

	wg.Wait()
	close(errs)
	var all []error
	for err := range errs {
		all = append(all, err)
	}

	connected := len(m.GetServers())
	if len(enabledNames) > 0 && connected == 0 {
		return fmt.Errorf("mcp: all %d enabled servers failed: %w", len(enabledNames), errors.Join(all...))
	}
	if len(all) > 0 {
		slog.Warn("mcp.load", "connected", connected, "failed", len(all))
	} else {
		slog.Info("mcp.load", "connected", connected)
	}
	return nil
}

// ConnectServer connects a single server.
func (m *Manager) ConnectServer(ctx context.Context, name string, cfg ServerConfig) error {
	client := mcp.NewClient(&mcp.Implementation{Name: "oneclaw", Version: "1.0.0"}, nil)

	transportType := cfg.Type
	if transportType == "" {
		if cfg.URL != "" {
			transportType = "sse"
		} else if cfg.Command != "" {
			transportType = "stdio"
		} else {
			return fmt.Errorf("either url or command is required")
		}
	}

	var transport mcp.Transport
	switch transportType {
	case "sse", "http":
		if cfg.URL == "" {
			return fmt.Errorf("url required for %s transport", transportType)
		}
		disableStandaloneSSE := cfg.Type == "http"
		sseTransport := &mcp.StreamableClientTransport{
			Endpoint:             cfg.URL,
			DisableStandaloneSSE: disableStandaloneSSE,
		}
		if len(cfg.Headers) > 0 {
			sseTransport.HTTPClient = &http.Client{
				Transport: &headerTransport{base: http.DefaultTransport, headers: cfg.Headers},
			}
		}
		transport = sseTransport
	case "stdio":
		if cfg.Command == "" {
			return fmt.Errorf("command required for stdio transport")
		}
		cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
		envMap := make(map[string]string)
		for _, e := range cmd.Environ() {
			if idx := strings.Index(e, "="); idx > 0 {
				envMap[e[:idx]] = e[idx+1:]
			}
		}
		if cfg.EnvFile != "" {
			fromFile, err := loadEnvFile(cfg.EnvFile)
			if err != nil {
				return fmt.Errorf("env file %s: %w", cfg.EnvFile, err)
			}
			for k, v := range fromFile {
				envMap[k] = v
			}
		}
		for k, v := range cfg.Env {
			envMap[k] = v
		}
		env := make([]string, 0, len(envMap))
		for k, v := range envMap {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
		transport = &mcp.CommandTransport{Command: cmd}
	default:
		return fmt.Errorf("unsupported transport %q (use stdio, sse, http)", transportType)
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	initResult := session.InitializeResult()
	srvName, srvVer := "", ""
	if initResult != nil && initResult.ServerInfo != nil {
		srvName = initResult.ServerInfo.Name
		srvVer = initResult.ServerInfo.Version
	}
	slog.Info("mcp.connected", "id", name, "server_name", srvName, "server_version", srvVer)

	var toolList []*mcp.Tool
	if initResult != nil && initResult.Capabilities != nil && initResult.Capabilities.Tools != nil {
		for tool, err := range session.Tools(ctx, nil) {
			if err != nil {
				slog.Warn("mcp.list_tools", "server", name, "err", err)
				continue
			}
			toolList = append(toolList, tool)
		}
		slog.Info("mcp.tools", "server", name, "count", len(toolList))
	}

	m.mu.Lock()
	m.servers[name] = &ServerConnection{
		Name:    name,
		Client:  client,
		Session: session,
		Tools:   toolList,
	}
	m.mu.Unlock()
	return nil
}

// GetServers returns a shallow copy of connections.
func (m *Manager) GetServers() map[string]*ServerConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*ServerConnection, len(m.servers))
	for k, v := range m.servers {
		out[k] = v
	}
	return out
}

// CallTool invokes a tool on a server.
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	if m.closed.Load() {
		return nil, fmt.Errorf("mcp manager closed")
	}
	m.mu.RLock()
	if m.closed.Load() {
		m.mu.RUnlock()
		return nil, fmt.Errorf("mcp manager closed")
	}
	conn, ok := m.servers[serverName]
	if ok {
		m.wg.Add(1)
	}
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("mcp server %q not found", serverName)
	}
	defer m.wg.Done()

	return conn.Session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: arguments})
}

// Close shuts down all sessions.
func (m *Manager) Close() error {
	if m.closed.Swap(true) {
		return nil
	}
	m.wg.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	slog.Info("mcp.close", "count", len(m.servers))
	var errs []error
	for name, conn := range m.servers {
		if err := conn.Session.Close(); err != nil {
			slog.Error("mcp.session_close", "server", name, "err", err)
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	m.servers = make(map[string]*ServerConnection)
	if len(errs) > 0 {
		return fmt.Errorf("mcp close: %w", errors.Join(errs...))
	}
	return nil
}
