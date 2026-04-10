package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go"
)

const defaultMaxInlineRunes = 16 * 1024

// maxMCPArtifactFileBytes caps bytes written to .oneclaw/artifacts/mcp/*.txt (DOM dumps, etc.).
const maxMCPArtifactFileBytes = 256 * 1024

const mcpArtifactTruncNote = "\n\n[truncated: MCP artifact file byte cap]\n"

func truncateUTF8StringByBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	if maxBytes <= 0 {
		return ""
	}
	s = s[:maxBytes]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// ToolCaller is implemented by *Manager.
type ToolCaller interface {
	CallTool(ctx context.Context, serverName, toolName string, arguments map[string]any) (*mcp.CallToolResult, error)
}

// Tool wraps one MCP tool for the OpenAI function-calling loop.
type Tool struct {
	caller     ToolCaller
	serverName string
	mcpTool    *mcp.Tool
	maxInline  int
}

// NewTool builds a registry tool. maxInlineRunes <= 0 uses default.
func NewTool(caller ToolCaller, serverName string, t *mcp.Tool, maxInlineRunes int) *Tool {
	return &Tool{
		caller:     caller,
		serverName: serverName,
		mcpTool:    t,
		maxInline:  maxInlineRunes,
	}
}

func sanitizeIdentifierComponent(s string) string {
	const maxLen = 64
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	prevUnderscore := false
	for _, r := range s {
		allowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if !allowed {
			if !prevUnderscore {
				b.WriteRune('_')
				prevUnderscore = true
			}
			continue
		}
		if r == '_' {
			if prevUnderscore {
				continue
			}
			prevUnderscore = true
		} else {
			prevUnderscore = false
		}
		b.WriteRune(r)
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		result = "unnamed"
	}
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return result
}

// Name matches OpenAI function name limits; prefixed by server to avoid collisions.
func (t *Tool) Name() string {
	sanitizedServer := sanitizeIdentifierComponent(t.serverName)
	sanitizedTool := sanitizeIdentifierComponent(t.mcpTool.Name)
	full := fmt.Sprintf("mcp_%s_%s", sanitizedServer, sanitizedTool)
	lossless := strings.ToLower(t.serverName) == sanitizedServer &&
		strings.ToLower(t.mcpTool.Name) == sanitizedTool
	const maxTotal = 64
	if lossless && len(full) <= maxTotal {
		return full
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(t.serverName + "\x00" + t.mcpTool.Name))
	suffix := fmt.Sprintf("%08x", h.Sum32())
	base := full
	if len(base) > maxTotal-9 {
		base = strings.TrimRight(full[:maxTotal-9], "_")
	}
	return base + "_" + suffix
}

func (t *Tool) Description() string {
	desc := t.mcpTool.Description
	if desc == "" {
		desc = fmt.Sprintf("MCP tool from server %q", t.serverName)
	}
	return fmt.Sprintf("[MCP:%s] %s", t.serverName, desc)
}

func (t *Tool) ConcurrencySafe() bool { return false }

func (t *Tool) Parameters() openai.FunctionParameters {
	schema := inputSchemaToMap(t.mcpTool.InputSchema)
	b, err := json.Marshal(schema)
	if err != nil {
		return openai.FunctionParameters{"type": "object", "properties": map[string]any{}}
	}
	var fp openai.FunctionParameters
	if err := json.Unmarshal(b, &fp); err != nil {
		return openai.FunctionParameters{"type": "object", "properties": map[string]any{}}
	}
	return fp
}

func inputSchemaToMap(schema any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}}
	}
	if m, ok := schema.(map[string]any); ok {
		return m
	}
	var raw []byte
	switch v := schema.(type) {
	case json.RawMessage:
		raw = v
	case []byte:
		raw = v
	}
	if raw != nil {
		var out map[string]any
		if json.Unmarshal(raw, &out) == nil {
			return out
		}
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}}
	}
	var out map[string]any
	if json.Unmarshal(b, &out) != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}}
	}
	return out
}

func (t *Tool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var args map[string]any
	if len(input) > 0 && string(input) != "null" {
		if err := json.Unmarshal(input, &args); err != nil {
			return "", err
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	execCtx := ctx
	if tctx != nil && tctx.Abort != nil {
		execCtx = tctx.Abort
	}
	res, err := t.caller.CallTool(execCtx, t.serverName, t.mcpTool.Name, args)
	if err != nil {
		return "", fmt.Errorf("mcp call: %w", err)
	}
	if res == nil {
		return "", fmt.Errorf("mcp: nil result")
	}
	if res.IsError {
		return "", fmt.Errorf("mcp tool error: %s", contentSummary(res.Content))
	}
	return t.formatResult(res.Content, tctx)
}

func contentSummary(content []mcp.Content) string {
	var parts []string
	for _, c := range content {
		switch v := c.(type) {
		case *mcp.TextContent:
			parts = append(parts, strings.TrimSpace(v.Text))
		case *mcp.ImageContent:
			parts = append(parts, fmt.Sprintf("[image %s]", v.MIMEType))
		default:
			parts = append(parts, fmt.Sprintf("[%T]", c))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func (t *Tool) formatResult(content []mcp.Content, tctx *toolctx.Context) (string, error) {
	var textParts []string
	for _, c := range content {
		switch v := c.(type) {
		case *mcp.TextContent:
			s := strings.TrimSpace(v.Text)
			if s != "" {
				textParts = append(textParts, s)
			}
		case *mcp.ImageContent:
			textParts = append(textParts, fmt.Sprintf("[MCP image %s, %d bytes]", v.MIMEType, len(v.Data)))
		case *mcp.AudioContent:
			textParts = append(textParts, fmt.Sprintf("[MCP audio %s, %d bytes]", v.MIMEType, len(v.Data)))
		case *mcp.ResourceLink:
			textParts = append(textParts, fmt.Sprintf("[resource %s %s]", v.Name, v.URI))
		case *mcp.EmbeddedResource:
			if v.Resource != nil {
				if len(v.Resource.Blob) > 0 {
					textParts = append(textParts, fmt.Sprintf("[embedded blob %s, %d bytes]", v.Resource.MIMEType, len(v.Resource.Blob)))
				} else if txt := strings.TrimSpace(v.Resource.Text); txt != "" {
					textParts = append(textParts, txt)
				}
			}
		default:
			textParts = append(textParts, fmt.Sprintf("[unsupported MCP content %T]", c))
		}
	}
	out := strings.Join(textParts, "\n")
	limit := t.maxInline
	if limit <= 0 {
		limit = defaultMaxInlineRunes
	}
	if utf8.RuneCountInString(out) <= limit {
		return out, nil
	}
	cwd := ""
	if tctx != nil {
		cwd = tctx.CWD
	}
	if cwd == "" {
		slog.Warn("mcp.large_result", "tool", t.mcpTool.Name, "msg", "truncating (no cwd for artifact)")
		runes := []rune(out)
		if len(runes) > limit {
			out = string(runes[:limit]) + "\n\n[truncated: MCP result exceeded inline limit]"
		}
		return out, nil
	}
	dir := filepath.Join(cwd, ".oneclaw", "artifacts", "mcp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mcp artifact dir: %w", err)
	}
	pattern := fmt.Sprintf("%s_%s_*.txt", sanitizeIdentifierComponent(t.serverName), sanitizeIdentifierComponent(t.mcpTool.Name))
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("mcp artifact file: %w", err)
	}
	path := f.Name()
	artifactOut := out
	if len(artifactOut) > maxMCPArtifactFileBytes {
		budget := maxMCPArtifactFileBytes - len(mcpArtifactTruncNote)
		if budget < 0 {
			budget = 0
		}
		artifactOut = truncateUTF8StringByBytes(artifactOut, budget) + mcpArtifactTruncNote
		slog.Warn("mcp.artifact_truncated", "tool", t.mcpTool.Name, "orig_bytes", len(out), "cap", maxMCPArtifactFileBytes)
	}
	if _, err := f.WriteString(artifactOut); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	n := utf8.RuneCountInString(out)
	if len(out) > maxMCPArtifactFileBytes {
		return fmt.Sprintf("[MCP returned large text (%d runes; on-disk artifact truncated to %d bytes); saved to %s]", n, len(artifactOut), path), nil
	}
	return fmt.Sprintf("[MCP returned large text (%d runes); saved to %s]", n, path), nil
}
