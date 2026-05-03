package meta

import "strings"

// FilterReplyMetaByKeys keeps only non-empty values for the given metadata key names (typically from
// clawbridge [RequiredOutboundMetadataKeysForSend] for the active client driver).
func FilterReplyMetaByKeys(m map[string]string, keys []string) map[string]string {
	if len(m) == 0 || len(keys) == 0 {
		return nil
	}
	out := make(map[string]string)
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if v := strings.TrimSpace(m[k]); v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
