package channel

import (
	"context"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/session"
)

// Bootstrap is passed to StartAll only (Engine + config). Connectors receive ConnectorConfig.
type Bootstrap struct {
	Engine *session.Engine
	Config *config.Resolved
}

// ConnectorConfig is what Spec.New may use; no session/routing types required.
// ChannelID is the instance id for this run (Inbound.Source / sink registry key).
// Params holds type-specific YAML fields from the channels list (excluding id/type).
type ConnectorConfig struct {
	Config    *config.Resolved
	Engine    *session.Engine
	ChannelID string
	Params    map[string]any
}

func connectorConfig(b Bootstrap, channelID string, params map[string]any) ConnectorConfig {
	return ConnectorConfig{Config: b.Config, Engine: b.Engine, ChannelID: channelID, Params: params}
}

// Spec registers one connector implementation. Key is the channel type name (YAML channels[].type).
// StartAll registers a chanSink on routing.DefaultRegistry for each started instance id.
type Spec struct {
	Key string
	New func(ctx context.Context, cfg ConnectorConfig) (Connector, error)
}
