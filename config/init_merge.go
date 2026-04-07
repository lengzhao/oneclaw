package config

import (
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

// mergeInitYAML parses template and existing config YAML, copies existing as the base,
// then fills in any keys present in template but missing in existing (recursive for maps).
// Slices and scalars already set by the user are left unchanged.
// Returns encoded YAML and whether any key was added.
func mergeInitYAML(templateYAML, existingYAML []byte) ([]byte, bool, error) {
	tmpl, err := parseYAMLRootMap(templateYAML)
	if err != nil {
		return nil, false, fmt.Errorf("config.init: parse embedded template: %w", err)
	}
	exist, err := parseYAMLRootMap(existingYAML)
	if err != nil {
		return nil, false, fmt.Errorf("config.init: parse existing config: %w", err)
	}
	dest := deepCopyYAMLMap(exist)
	changed := mergeMissingFromTemplate(tmpl, dest)
	if !changed {
		return nil, false, nil // no write; merged bytes intentionally nil
	}
	out, err := yaml.Marshal(dest)
	if err != nil {
		return nil, false, fmt.Errorf("config.init: marshal merged config: %w", err)
	}
	return out, true, nil
}

func parseYAMLRootMap(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root == nil {
		return map[string]any{}, nil
	}
	m, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("root must be a mapping, got %T", root)
	}
	return m, nil
}

func mergeMissingFromTemplate(template, dest map[string]any) bool {
	var changed bool
	for k, tv := range template {
		dv, ok := dest[k]
		if !ok {
			dest[k] = deepCopyYAMLValue(tv)
			changed = true
			continue
		}
		tmplMap, tIsMap := tv.(map[string]any)
		if !tIsMap {
			continue
		}
		destMap, dIsMap, converted := ensureYAMLStringMap(dv)
		if !dIsMap {
			continue
		}
		if converted {
			dest[k] = destMap
		}
		if mergeMissingFromTemplate(tmplMap, destMap) {
			changed = true
		}
	}
	return changed
}

// ensureYAMLStringMap returns a string-keyed map for in-place merge; if v was map[any]any,
// converted is true and the caller must store the returned map back into dest (parent did so).
func ensureYAMLStringMap(v any) (m map[string]any, ok bool, converted bool) {
	if sm, ok := v.(map[string]any); ok {
		return sm, true, false
	}
	am, ok := v.(map[any]any)
	if !ok {
		return nil, false, false
	}
	out := make(map[string]any, len(am))
	for k, val := range am {
		ks, ok := k.(string)
		if !ok {
			return nil, false, false
		}
		out[ks] = val
	}
	return out, true, true
}

func deepCopyYAMLMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyYAMLValue(v)
	}
	return out
}

func deepCopyYAMLValue(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case map[string]any:
		return deepCopyYAMLMap(x)
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, val := range x {
			ks, ok := k.(string)
			if !ok {
				return v
			}
			m[ks] = deepCopyYAMLValue(val)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i := range x {
			s[i] = deepCopyYAMLValue(x[i])
		}
		return s
	default:
		if reflect.ValueOf(v).Kind() == reflect.Slice {
			rv := reflect.ValueOf(v)
			n := rv.Len()
			out := reflect.MakeSlice(rv.Type(), n, n)
			for i := 0; i < n; i++ {
				out.Index(i).Set(rv.Index(i))
			}
			return out.Interface()
		}
		return v
	}
}
