package config

import "strings"

// UpsertModelProfile replaces or appends a profile by ID (empty id treated as "default").
func UpsertModelProfile(dst *File, patch ModelProfile) {
	if dst == nil {
		return
	}
	id := strings.TrimSpace(patch.ID)
	if id == "" {
		id = "default"
		patch.ID = id
	}
	for i := range dst.Models {
		if dst.Models[i].ID == id {
			dst.Models[i] = patch
			return
		}
	}
	dst.Models = append(dst.Models, patch)
}
