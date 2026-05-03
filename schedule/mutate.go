package schedule

import (
	"log/slog"
	"path/filepath"
	"sync"
)

type pathMuRegistry struct {
	mu sync.Mutex
	m  map[string]*sync.Mutex
}

func (r *pathMuRegistry) lock(absPath string) (unlock func()) {
	key := filepath.Clean(absPath)
	r.mu.Lock()
	if r.m == nil {
		r.m = make(map[string]*sync.Mutex)
	}
	m, ok := r.m[key]
	if !ok {
		m = &sync.Mutex{}
		r.m[key] = m
	}
	r.mu.Unlock()
	m.Lock()
	return m.Unlock
}

var schedulePathLocks pathMuRegistry

func compactStaleJobs(f *File) {
	if f == nil {
		return
	}
	out := f.Jobs[:0]
	for _, j := range f.Jobs {
		j.Normalize()
		if !j.Enabled && j.NextRunUnix <= 0 {
			continue
		}
		out = append(out, j)
	}
	f.Jobs = out
}

// mutateJobsFile loads path, compacts stale rows, runs fn, saves when fn reports a mutation.
func mutateJobsFile(path string, fn func(*File) (bool, error)) error {
	unlock := schedulePathLocks.lock(path)
	defer unlock()

	f, err := Load(path)
	if err != nil {
		slog.Error("schedule.mutate", "phase", "load", "path", path, "err", err)
		return err
	}
	compactStaleJobs(f)
	changed, err := fn(f)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	compactStaleJobs(f)
	if err := Save(path, f); err != nil {
		slog.Error("schedule.mutate", "phase", "save", "path", path, "err", err)
		return err
	}
	slog.Debug("schedule.mutate", "phase", "save_ok", "path", path)
	return nil
}
