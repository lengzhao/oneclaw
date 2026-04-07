package usageledger

import (
	"strings"

	"github.com/lengzhao/oneclaw/rtopts"
)

// priceEntry is USD per 1M tokens.
type priceEntry struct {
	prefix   string
	input1M  float64
	output1M float64
}

// Built-in list uses public ballpark rates; gateways may differ. Unknown models use usage.default_*_per_mtok from config.
var builtinPricing = []priceEntry{
	{prefix: "gpt-4o-mini", input1M: 0.15, output1M: 0.60},
	{prefix: "gpt-4o", input1M: 2.50, output1M: 10.00},
	{prefix: "gpt-4-turbo", input1M: 10.00, output1M: 30.00},
	{prefix: "gpt-4", input1M: 30.00, output1M: 60.00},
	{prefix: "gpt-3.5-turbo", input1M: 0.50, output1M: 1.50},
	{prefix: "o1-mini", input1M: 1.10, output1M: 4.40},
	{prefix: "o1", input1M: 15.00, output1M: 60.00},
	{prefix: "claude-3-5-sonnet", input1M: 3.00, output1M: 15.00},
	{prefix: "claude-3-5-haiku", input1M: 0.80, output1M: 4.00},
	{prefix: "claude-3-opus", input1M: 15.00, output1M: 75.00},
	{prefix: "claude", input1M: 3.00, output1M: 15.00},
}

// PricePerMillion returns input and output USD price per 1M tokens for model (longest matching prefix).
func PricePerMillion(model string) (inputPerM, outputPerM float64) {
	m := strings.TrimSpace(strings.ToLower(model))
	for _, e := range builtinPricing {
		if strings.HasPrefix(m, e.prefix) {
			return e.input1M, e.output1M
		}
	}
	return defaultPricePerMillion()
}

func defaultPricePerMillion() (float64, float64) {
	rt := rtopts.Current()
	in, out := rt.UsageInputPerMtok, rt.UsageOutputPerMtok
	if in <= 0 {
		in = 5
	}
	if out <= 0 {
		out = 15
	}
	return in, out
}

// EstimateCostUSDFromTokens approximates spend from token counts (only used when features.usage_estimate_cost is set).
func EstimateCostUSDFromTokens(model string, promptTokens, completionTokens int64) float64 {
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	pi, po := PricePerMillion(model)
	return float64(promptTokens)*pi/1e6 + float64(completionTokens)*po/1e6
}
