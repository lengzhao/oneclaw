package config

import (
	"fmt"
	"sort"
	"strings"
)

// ResolveModelProfile returns the profile for id, or the highest-priority profile when id is empty (Priority asc, then ID).
func ResolveModelProfile(f *File, id string) (*ModelProfile, error) {
	if f == nil || len(f.Models) == 0 {
		return nil, fmt.Errorf("config: no model profiles")
	}
	want := strings.TrimSpace(id)
	if want == "" {
		idx := sortedFailoverIndices(f.Models)
		return &f.Models[idx[0]], nil
	}
	for i := range f.Models {
		if f.Models[i].ID == want {
			return &f.Models[i], nil
		}
	}
	return nil, fmt.Errorf("config: unknown model profile %q", want)
}

// OrderedModelProfiles returns profiles in failover order (Priority asc, then ID asc).
// Useful for trying backups after errors without embedding policy in config loader.
func OrderedModelProfiles(f *File) []*ModelProfile {
	if f == nil || len(f.Models) == 0 {
		return nil
	}
	idx := sortedFailoverIndices(f.Models)
	out := make([]*ModelProfile, len(idx))
	for i, j := range idx {
		out[i] = &f.Models[j]
	}
	return out
}

func sortedFailoverIndices(profiles []ModelProfile) []int {
	idx := make([]int, len(profiles))
	for i := range profiles {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := profiles[idx[a]].Priority, profiles[idx[b]].Priority
		if pa != pb {
			return pa < pb
		}
		return profiles[idx[a]].ID < profiles[idx[b]].ID
	})
	return idx
}
