package sinks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var pathLocks sync.Map // path string -> *sync.Mutex

func lockPath(path string) *sync.Mutex {
	v, _ := pathLocks.LoadOrStore(path, new(sync.Mutex))
	return v.(*sync.Mutex)
}

func appendJSONLRecord(cwd, segment, subdir string, when time.Time, rec any) error {
	if cwd == "" {
		cwd = "."
	}
	y := when.UTC().Format("2006")
	mo := when.UTC().Format("01")
	day := when.UTC().Format("2006-01-02")
	path := filepath.Join(cwd, ".oneclaw", "audit", segment, subdir, y, mo, day+".jsonl")
	mu := lockPath(path)
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

func wallTimeFromEventTS(ts int64) time.Time {
	if ts <= 0 {
		return time.Now().UTC()
	}
	return time.UnixMilli(ts).UTC()
}
