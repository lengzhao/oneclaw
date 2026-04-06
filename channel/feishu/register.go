package feishu

import (
	"context"

	chann "github.com/lengzhao/oneclaw/channel"
)

func init() {
	chann.RegisterDefault(chann.Spec{
		Key: RegistryName,
		New: func(ctx context.Context, cfg chann.ConnectorConfig) (chann.Connector, error) {
			return New(cfg)
		},
	})
}
