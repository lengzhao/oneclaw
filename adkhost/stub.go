package adkhost

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// StubChatModel is a minimal ToolCallingChatModel for tests and --mock-llm (FR-EINO-04).
type StubChatModel struct {
	Reply string
}

// NewStubChatModel returns a model that always replies with Assistant text (no Stream).
func NewStubChatModel(reply string) *StubChatModel {
	if reply == "" {
		reply = "(stub) ok"
	}
	return &StubChatModel{Reply: reply}
}

func (s *StubChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(s.Reply, nil), nil
}

func (s *StubChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (
	*schema.StreamReader[*schema.Message], error,
) {
	return nil, fmt.Errorf("adkhost: stub Stream not implemented")
}

func (s *StubChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return s, nil
}
