package loop

import (
	"encoding/json"

	"github.com/openai/openai-go"
)

// assistantResponseJSONKeysHandledByToParam are top-level keys the OpenAI SDK maps in
// ChatCompletionMessage.ToAssistantMessageParam; anything else from the wire JSON is treated
// as vendor extension and merged via SetExtraFields so round-trips keep provider-specific fields.
var assistantResponseJSONKeysHandledByToParam = map[string]struct{}{
	"role":          {},
	"content":       {},
	"refusal":       {},
	"tool_calls":    {},
	"function_call": {},
	"audio":         {},
}

func mergeAssistantExtra(asst *openai.ChatCompletionAssistantMessageParam, add map[string]any) {
	prev := asst.ExtraFields()
	merged := make(map[string]any, len(prev)+len(add))
	for k, v := range prev {
		merged[k] = v
	}
	for k, v := range add {
		merged[k] = v
	}
	asst.SetExtraFields(merged)
}

// assistantParamFromCompletion builds the assistant message param for history: starts from
// ToParam(), then merges any extra top-level JSON keys from the completion RawJSON (e.g.
// reasoning_content, or other vendor fields openai-go does not model). Assistant messages with
// tool_calls get reasoning_content defaulted to "" when still absent, matching strict gateways.
func assistantParamFromCompletion(msg openai.ChatCompletionMessage) openai.ChatCompletionMessageParamUnion {
	u := msg.ToParam()
	if u.OfAssistant == nil {
		return u
	}
	extras := extensionFieldsFromAssistantRawJSON(msg.RawJSON())
	if len(msg.ToolCalls) > 0 {
		if extras == nil {
			extras = make(map[string]any)
		}
		if v, ok := extras["reasoning_content"]; !ok || v == nil {
			extras["reasoning_content"] = ""
		}
	}
	if len(extras) > 0 {
		mergeAssistantExtra(u.OfAssistant, extras)
	}
	return u
}

func extensionFieldsFromAssistantRawJSON(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var aux map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &aux); err != nil {
		return nil
	}
	out := make(map[string]any)
	for k, rb := range aux {
		if _, skip := assistantResponseJSONKeysHandledByToParam[k]; skip {
			continue
		}
		var val any
		if err := json.Unmarshal(rb, &val); err != nil {
			continue
		}
		out[k] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ApplyOutboundAssistantExtensionFields ensures assistant messages with tool_calls include
// reasoning_content when missing (empty string). Other extension keys are expected to have been
// stored when the message was appended from assistantParamFromCompletion; this pass fixes older
// transcripts and any path that built assistant params without extras.
func ApplyOutboundAssistantExtensionFields(msgs []openai.ChatCompletionMessageParamUnion) {
	for i := range msgs {
		if len(msgs[i].GetToolCalls()) == 0 {
			continue
		}
		asst := msgs[i].OfAssistant
		if asst == nil {
			continue
		}
		if ex := asst.ExtraFields(); ex != nil {
			if v, ok := ex["reasoning_content"]; ok && v != nil {
				continue
			}
		}
		mergeAssistantExtra(asst, map[string]any{"reasoning_content": ""})
	}
}
