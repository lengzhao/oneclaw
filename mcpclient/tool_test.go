package mcpclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type stubCaller struct {
	out *mcp.CallToolResult
	err error
}

func (s stubCaller) CallTool(ctx context.Context, serverName, toolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	return s.out, s.err
}

func TestTool_Name_stablePrefix(t *testing.T) {
	mt := &mcp.Tool{Name: "read_stuff"}
	tool := NewTool(stubCaller{}, "myserver", mt, 0)
	if got := tool.Name(); !strings.HasPrefix(got, "mcp_") {
		t.Fatalf("name=%q", got)
	}
}

func TestTruncateUTF8StringByBytes(t *testing.T) {
	s := "hello" + string([]byte{0xe2}) // incomplete rune at end if cut wrong
	full := s + "世"
	if got := truncateUTF8StringByBytes(full, 8); utf8.ValidString(got) != true {
		t.Fatalf("invalid utf8: %q", got)
	}
}

func TestTool_Execute_text(t *testing.T) {
	c := stubCaller{
		out: &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "  hello  "}},
		},
	}
	mt := &mcp.Tool{Name: "t1"}
	tool := NewTool(c, "srv", mt, 0)
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"a":1}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("got %q", out)
	}
}
