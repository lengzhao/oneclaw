package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// loopMessagesGenModelInput builds model messages like loop.BuildRequestMessages(system, history)
// mapped to Eino schema: system instruction plus full OpenAI-shaped transcript.
// Runner.Run is called with nil messages; the payload comes entirely from cfg.Messages here.
func loopMessagesGenModelInput(cfg *loop.Config) adk.GenModelInput {
	return func(ctx context.Context, instruction string, input *adk.AgentInput) ([]adk.Message, error) {
		return buildADKMessagesFromLoop(ctx, instruction, cfg.Messages)
	}
}

func buildADKMessagesFromLoop(ctx context.Context, instruction string, msgs *[]openai.ChatCompletionMessageParamUnion) ([]adk.Message, error) {
	out := make([]adk.Message, 0)
	if s := strings.TrimSpace(instruction); s != "" {
		sp := schema.SystemMessage(s)
		vs := adk.GetSessionValues(ctx)
		if len(vs) > 0 {
			ct := prompt.FromMessages(schema.FString, sp)
			ms, err := ct.Format(ctx, vs)
			if err != nil {
				return nil, fmt.Errorf("session: eino instruction format: %w", err)
			}
			if len(ms) == 0 {
				return nil, fmt.Errorf("session: eino instruction format: empty result")
			}
			sp = ms[0]
		}
		out = append(out, sp)
	}
	if msgs == nil {
		return out, nil
	}
	rest, err := openAIParamsToSchemaMessages(*msgs)
	if err != nil {
		return nil, err
	}
	out = append(out, rest...)
	return out, nil
}

func openAIParamsToSchemaMessages(msgs []openai.ChatCompletionMessageParamUnion) ([]*schema.Message, error) {
	out := make([]*schema.Message, 0, len(msgs))
	for i, u := range msgs {
		m, err := openAIParamUnionToSchema(u)
		if err != nil {
			return nil, fmt.Errorf("session: eino message %d: %w", i, err)
		}
		out = append(out, m)
	}
	return out, nil
}

func openAIParamUnionToSchema(u openai.ChatCompletionMessageParamUnion) (*schema.Message, error) {
	switch {
	case u.OfDeveloper != nil:
		s, err := stringFromDeveloperContent(u.OfDeveloper.Content)
		if err != nil {
			return nil, err
		}
		return schema.SystemMessage(s), nil
	case u.OfSystem != nil:
		s, err := stringFromSystemContent(u.OfSystem.Content)
		if err != nil {
			return nil, err
		}
		return schema.SystemMessage(s), nil
	case u.OfUser != nil:
		return userParamToSchema(u.OfUser)
	case u.OfAssistant != nil:
		return assistantParamToSchema(u.OfAssistant)
	case u.OfTool != nil:
		return toolParamToSchema(u.OfTool)
	case u.OfFunction != nil:
		content := ""
		if !param.IsOmitted(u.OfFunction.Content) {
			content = u.OfFunction.Content.Value
		}
		return schema.UserMessage(content), nil
	default:
		return nil, fmt.Errorf("unsupported or empty message union")
	}
}

func stringFromSystemContent(c openai.ChatCompletionSystemMessageParamContentUnion) (string, error) {
	if !param.IsOmitted(c.OfString) {
		return c.OfString.Value, nil
	}
	if !param.IsOmitted(c.OfArrayOfContentParts) {
		var b strings.Builder
		for _, p := range c.OfArrayOfContentParts {
			b.WriteString(p.Text)
		}
		return b.String(), nil
	}
	return "", nil
}

func stringFromDeveloperContent(c openai.ChatCompletionDeveloperMessageParamContentUnion) (string, error) {
	if !param.IsOmitted(c.OfString) {
		return c.OfString.Value, nil
	}
	if !param.IsOmitted(c.OfArrayOfContentParts) {
		var b strings.Builder
		for _, p := range c.OfArrayOfContentParts {
			b.WriteString(p.Text)
		}
		return b.String(), nil
	}
	return "", nil
}

func userParamToSchema(u *openai.ChatCompletionUserMessageParam) (*schema.Message, error) {
	c := u.Content
	if !param.IsOmitted(c.OfString) {
		return schema.UserMessage(c.OfString.Value), nil
	}
	if !param.IsOmitted(c.OfArrayOfContentParts) {
		parts, err := userContentPartsToInputParts(c.OfArrayOfContentParts)
		if err != nil {
			return nil, err
		}
		return &schema.Message{
			Role:                  schema.User,
			UserInputMultiContent: parts,
		}, nil
	}
	return schema.UserMessage(""), nil
}

func userContentPartsToInputParts(parts []openai.ChatCompletionContentPartUnionParam) ([]schema.MessageInputPart, error) {
	out := make([]schema.MessageInputPart, 0, len(parts))
	for i, p := range parts {
		part, err := userContentPartToInputPart(p)
		if err != nil {
			return nil, fmt.Errorf("part %d: %w", i, err)
		}
		out = append(out, part)
	}
	return out, nil
}

func userContentPartToInputPart(p openai.ChatCompletionContentPartUnionParam) (schema.MessageInputPart, error) {
	if p.OfText != nil {
		return schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: p.OfText.Text}, nil
	}
	if p.OfImageURL != nil {
		url := p.OfImageURL.ImageURL.URL
		detail := schema.ImageURLDetailAuto
		switch strings.TrimSpace(p.OfImageURL.ImageURL.Detail) {
		case "low":
			detail = schema.ImageURLDetailLow
		case "high":
			detail = schema.ImageURLDetailHigh
		case "auto", "":
			detail = schema.ImageURLDetailAuto
		}
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{URL: &url},
				Detail:            detail,
			},
		}, nil
	}
	if p.OfInputAudio != nil {
		b64 := p.OfInputAudio.InputAudio.Data
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeAudioURL,
			Audio: &schema.MessageInputAudio{
				MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64},
			},
		}, nil
	}
	if p.OfFile != nil {
		f := p.OfFile.File
		var mi schema.MessageInputFile
		if !param.IsOmitted(f.FileData) {
			d := f.FileData.Value
			mi.MessagePartCommon.Base64Data = &d
		}
		if !param.IsOmitted(f.Filename) {
			mi.Name = f.Filename.Value
		}
		if !param.IsOmitted(f.FileID) {
			id := f.FileID.Value
			mi.MessagePartCommon.Extra = map[string]any{"openai_file_id": id}
		}
		if mi.MessagePartCommon.Base64Data == nil && len(mi.MessagePartCommon.Extra) == 0 {
			return schema.MessageInputPart{}, fmt.Errorf("unsupported file part (need file_data or file_id)")
		}
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeFileURL,
			File: &mi,
		}, nil
	}
	return schema.MessageInputPart{}, fmt.Errorf("unsupported user content part")
}

func assistantParamToSchema(a *openai.ChatCompletionAssistantMessageParam) (*schema.Message, error) {
	text := assistantTextContent(a)
	var tcalls []schema.ToolCall
	if len(a.ToolCalls) > 0 {
		tcalls = make([]schema.ToolCall, 0, len(a.ToolCalls))
		for _, tc := range a.ToolCalls {
			tcalls = append(tcalls, schema.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	} else if strings.TrimSpace(a.FunctionCall.Name) != "" {
		tcalls = []schema.ToolCall{{
			ID:   "",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      a.FunctionCall.Name,
				Arguments: a.FunctionCall.Arguments,
			},
		}}
	}
	return schema.AssistantMessage(text, tcalls), nil
}

func assistantTextContent(a *openai.ChatCompletionAssistantMessageParam) string {
	c := a.Content
	if !param.IsOmitted(c.OfString) {
		return c.OfString.Value
	}
	if !param.IsOmitted(c.OfArrayOfContentParts) {
		var b strings.Builder
		for _, part := range c.OfArrayOfContentParts {
			if part.OfText != nil {
				b.WriteString(part.OfText.Text)
			}
			if part.OfRefusal != nil {
				b.WriteString(part.OfRefusal.Refusal)
			}
		}
		return b.String()
	}
	return ""
}

func toolParamToSchema(t *openai.ChatCompletionToolMessageParam) (*schema.Message, error) {
	body, err := stringFromToolContent(t.Content)
	if err != nil {
		return nil, err
	}
	return schema.ToolMessage(body, t.ToolCallID), nil
}

func stringFromToolContent(c openai.ChatCompletionToolMessageParamContentUnion) (string, error) {
	if !param.IsOmitted(c.OfString) {
		return c.OfString.Value, nil
	}
	if !param.IsOmitted(c.OfArrayOfContentParts) {
		var b strings.Builder
		for _, p := range c.OfArrayOfContentParts {
			b.WriteString(p.Text)
		}
		return b.String(), nil
	}
	return "", nil
}
