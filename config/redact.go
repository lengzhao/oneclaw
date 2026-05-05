package config

import (
	"strings"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"gopkg.in/yaml.v3"
)

// RedactSecretsDeepCopy returns a YAML round-tripped File with model API keys and
// clawbridge option fields that look sensitive replaced by "***" (for config show).
func RedactSecretsDeepCopy(f *File) (*File, error) {
	if f == nil {
		return nil, nil
	}
	b, err := yaml.Marshal(f)
	if err != nil {
		return nil, err
	}
	var out File
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	for i := range out.Models {
		if strings.TrimSpace(out.Models[i].APIKey) != "" {
			out.Models[i].APIKey = "***"
		}
	}
	redactClawbridgeOptions(&out.Clawbridge)
	return &out, nil
}

func redactClawbridgeOptions(c *cbconfig.Config) {
	for ci := range c.Clients {
		opts := c.Clients[ci].Options
		if opts == nil {
			continue
		}
		for k := range opts {
			if sensitiveOptionKey(k) {
				opts[k] = "***"
			}
		}
	}
}

func sensitiveOptionKey(k string) bool {
	lk := strings.ToLower(strings.TrimSpace(k))
	for _, s := range []string{"secret", "token", "password", "credential", "api_key", "apikey", "private"} {
		if strings.Contains(lk, s) {
			return true
		}
	}
	return false
}
