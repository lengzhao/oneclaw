package config

import (
	"fmt"
	"strings"
)

// SplitProviderModel splits "profile_id_or_provider/model_name" on the first '/'.
// Model names that contain '/' keep the remainder (e.g. "openrouter/org/model").
func SplitProviderModel(s string) (credentialKey, modelName string, ok bool) {
	s = strings.TrimSpace(s)
	i := strings.IndexByte(s, '/')
	if i <= 0 || i >= len(s)-1 {
		return "", "", false
	}
	credentialKey = strings.TrimSpace(s[:i])
	modelName = strings.TrimSpace(s[i+1:])
	if credentialKey == "" || modelName == "" {
		return "", "", false
	}
	return credentialKey, modelName, true
}

// EffectiveModelSelector returns inbound/CLI profile when set, otherwise agent frontmatter model.
func EffectiveModelSelector(inboundOrCLIPprofile, agentModel string) string {
	if s := strings.TrimSpace(inboundOrCLIPprofile); s != "" {
		return s
	}
	return strings.TrimSpace(agentModel)
}

// ResolveProfilesForCredentialKey returns profiles for the left segment of a credential/model selector,
// in failover order (Priority asc, then ID asc).
//
// If key equals some models[].id, only that profile is returned (pinned route).
// Otherwise key is matched case-insensitively against models[].provider; all matches are returned in failover order.
func ResolveProfilesForCredentialKey(f *File, key string) ([]ModelProfile, error) {
	if f == nil || len(f.Models) == 0 {
		return nil, fmt.Errorf("config: no model profiles")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("config: empty credential key")
	}
	for i := range f.Models {
		if f.Models[i].ID == key {
			return []ModelProfile{f.Models[i]}, nil
		}
	}
	var out []ModelProfile
	for _, j := range sortedFailoverIndices(f.Models) {
		p := f.Models[j]
		if strings.EqualFold(strings.TrimSpace(p.Provider), key) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("config: unknown model profile id or provider %q", key)
	}
	return out, nil
}

func profilesWithAPICopyModel(f *File, key, modelName string) ([]ModelProfile, error) {
	bases, err := ResolveProfilesForCredentialKey(f, key)
	if err != nil {
		return nil, err
	}
	out := make([]ModelProfile, len(bases))
	for i := range bases {
		out[i] = bases[i]
		out[i].DefaultModel = modelName
	}
	return out, nil
}

// ResolveModelProfilesForTurn resolves ordered credential profiles with API model id set for each copy.
// selector must be empty or "profile_id_or_provider/api_model_id" (split on first '/' only).
// Empty selector uses File.DefaultModel.
func ResolveModelProfilesForTurn(f *File, selector string) ([]ModelProfile, error) {
	if f == nil || len(f.Models) == 0 {
		return nil, fmt.Errorf("config: no model profiles")
	}
	sel := strings.TrimSpace(selector)
	src := sel
	if src == "" {
		src = strings.TrimSpace(f.DefaultModel)
	}
	key, model, ok := SplitProviderModel(src)
	if !ok {
		if sel != "" {
			return nil, fmt.Errorf("config: model selector must be profile_id_or_provider/api_model_id (got %q)", sel)
		}
		return nil, fmt.Errorf("config: default_model must be profile_id_or_provider/api_model_id (got %q)", strings.TrimSpace(f.DefaultModel))
	}
	return profilesWithAPICopyModel(f, key, model)
}

// ResolveModelForTurn returns the first resolved profile (same as ResolveModelProfilesForTurn(f, selector)[0]).
func ResolveModelForTurn(f *File, selector string) (*ModelProfile, error) {
	all, err := ResolveModelProfilesForTurn(f, selector)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("config: no model profiles matched selector")
	}
	p := all[0]
	return &p, nil
}
