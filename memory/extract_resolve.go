package memory

import (
	"strings"

	lzmodel "github.com/lengzhao/memory/model"
)

// modelRequiresExtractTemperatureOne matches OpenAI-compatible models that reject sampling temperature != 1.
func modelRequiresExtractTemperatureOne(model string) bool {
	m := strings.TrimSpace(model)
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	m = strings.ToLower(m)
	switch {
	case strings.HasPrefix(m, "o1"),
		strings.HasPrefix(m, "o3"),
		strings.HasPrefix(m, "o4"),
		strings.HasPrefix(m, "gpt-5"),
		strings.HasPrefix(m, "kimi"): // Moonshot Kimi (e.g. kimi-k2.5): API allows only temperature 1
		return true
	default:
		return false
	}
}

// normalizeExtractLLMTemperature sets Temperature for extraction requests (may run again after the model string changes).
func normalizeExtractLLMTemperature(cfg *lzmodel.LLMConfig) {
	if cfg == nil {
		return
	}
	if modelRequiresExtractTemperatureOne(cfg.Model) {
		cfg.Temperature = 1
	} else if cfg.Temperature == 0 {
		cfg.Temperature = 0.2
	}
}

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
	normalizeExtractLLMTemperature(&c)
	return &c
}
