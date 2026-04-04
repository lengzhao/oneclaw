package builtin

import "github.com/openai/openai-go"

func objectSchema(properties map[string]any, required []string) openai.FunctionParameters {
	return openai.FunctionParameters{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}
