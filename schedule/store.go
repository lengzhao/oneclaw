package schedule

import (
	"encoding/json"
	"errors"
	"os"
)

const currentVersion = 1

// Load reads path into a File; missing file yields empty jobs list.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{Version: currentVersion, Jobs: nil}, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if f.Version == 0 {
		f.Version = currentVersion
	}
	for i := range f.Jobs {
		f.Jobs[i].Normalize()
	}
	return &f, nil
}

// Save atomically writes f to path (0664).
func Save(path string, f *File) error {
	if f == nil {
		f = &File{}
	}
	if f.Version == 0 {
		f.Version = currentVersion
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
