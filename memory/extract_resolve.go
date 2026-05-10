package memory

import (
	"strings"

	lzmodel "github.com/lengzhao/memory/model"
)

func resolveExtractLLM(explicit *lzmodel.LLMConfig, maxOut int64, postTurn bool) *lzmodel.LLMConfig {
	if explicit == nil || strings.TrimSpace(explicit.APIKey) == "" || strings.TrimSpace(explicit.Model) == "" {
		return nil
	}
	outTok := maintenanceEffectiveMaxTokens(maxOut, postTurn)
	maxTok := 4096
	if outTok > 0 && int(outTok) < maxTok {
		maxTok = int(outTok)
	}
	if maxTok < 512 {
		maxTok = 512
	}
	var timeoutSec int
	if postTurn {
		timeoutSec = int(postTurnMaintainTimeout().Seconds())
		if timeoutSec <= 0 {
			timeoutSec = 120
		}
	} else {
		timeoutSec = int(scheduledMaintainTimeout().Seconds())
		if timeoutSec <= 0 {
			timeoutSec = 1800
		}
	}
	c := *explicit
	if c.MaxTokens <= 0 {
		c.MaxTokens = maxTok
	}
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = timeoutSec
	}
	if c.Temperature == 0 {
		c.Temperature = 0.2
	}
	return &c
}
