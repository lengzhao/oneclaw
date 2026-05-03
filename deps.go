// Package oneclaw retains go.mod requires until feature packages import these paths directly.
package oneclaw

import (
	_ "github.com/cloudwego/eino/adk"
	_ "github.com/cloudwego/eino-ext/components/model/openai"
	_ "github.com/eino-contrib/jsonschema"
	_ "github.com/lengzhao/clawbridge/drivers"
	_ "github.com/modelcontextprotocol/go-sdk/mcp"
	_ "github.com/openai/openai-go"
	_ "github.com/robfig/cron/v3"
	_ "golang.org/x/term"
	_ "gopkg.in/yaml.v3"
)
