package sinks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lengzhao/oneclaw/memory"
)

var pathLocks sync.Map // path string -> *sync.Mutex

func lockPath(path string) *sync.Mutex {
	v, _ := pathLocks.LoadOrStore(path, new(sync.Mutex))
	return v.(*sync.Mutex)
}

func appendJSONLRecord(cwd, auditSessionID, agentSegment, subdir string, when time.Time, rec any) error {
	if cwd == "" {
		cwd = "."
	}
	y := when.UTC().Format("2006")
	mo := when.UTC().Format("01")
	day := when.UTC().Format("2006-01-02")
	var path string
	if sid := strings.TrimSpace(auditSessionID); sid != "" {
		safe := SanitizeAgentSegment(sid)
		path = filepath.Join(cwd, memory.DotDir, "sessions", safe, "audit", agentSegment, subdir, y, mo, day+".jsonl")
	} else {
		path = filepath.Join(cwd, memory.DotDir, "audit", agentSegment, subdir, y, mo, day+".jsonl")
	}
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
