package config

import (
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

// RuntimeView is the process-local snapshot after PushRuntime (reference-architecture §2.1).
type RuntimeView struct {
	Config *File
}

var runtimeSnap atomic.Pointer[RuntimeView]

// PushRuntime stores an immutable snapshot (YAML-cloned) for readers via Runtime().
func PushRuntime(f *File) {
	if f == nil {
		runtimeSnap.Store(nil)
		return
	}
	cp := cloneFile(f)
	runtimeSnap.Store(&RuntimeView{Config: cp})
}

// Runtime returns the last PushRuntime snapshot or nil.
func Runtime() *RuntimeView {
	return runtimeSnap.Load()
}

func cloneFile(f *File) *File {
	b, err := yaml.Marshal(f)
	if err != nil {
		cp := *f
		return &cp
	}
	var out File
	if err := yaml.Unmarshal(b, &out); err != nil {
		cp := *f
		return &cp
	}
	return &out
}
