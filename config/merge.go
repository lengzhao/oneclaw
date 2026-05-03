package config

func mergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, v := range src {
		if v == nil {
			dst[k] = nil
			continue
		}
		vmap, ok := v.(map[string]any)
		if !ok {
			dst[k] = v
			continue
		}
		existing, ok := dst[k].(map[string]any)
		if !ok || existing == nil {
			dst[k] = mergeMaps(nil, vmap)
			continue
		}
		dst[k] = mergeMaps(existing, vmap)
	}
	return dst
}
