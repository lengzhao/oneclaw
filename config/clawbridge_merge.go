package config

import (
	"strings"

	cbconfig "github.com/lengzhao/clawbridge/config"
)

// UpsertClawbridge merges patch into dst.Clawbridge: clients upserted by ID; media root set if patch provides a non-empty root.
func UpsertClawbridge(dst *File, patch cbconfig.Config) {
	if dst == nil {
		return
	}
	if strings.TrimSpace(patch.Media.Root) != "" {
		dst.Clawbridge.Media.Root = strings.TrimSpace(patch.Media.Root)
	}
	byID := make(map[string]int, len(dst.Clawbridge.Clients))
	for i, c := range dst.Clawbridge.Clients {
		byID[c.ID] = i
	}
	for _, c := range patch.Clients {
		if idx, ok := byID[c.ID]; ok {
			dst.Clawbridge.Clients[idx] = c
			continue
		}
		dst.Clawbridge.Clients = append(dst.Clawbridge.Clients, c)
		byID[c.ID] = len(dst.Clawbridge.Clients) - 1
	}
}
