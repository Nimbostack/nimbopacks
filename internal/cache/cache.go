// Package cache manages the nimbopacks build cache at ~/.nimbopacks/cache.
package cache

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// baseDirOverride redirects the cache base directory in tests.
// Set via SetBaseDirForTesting; reset to "" to restore the default.
var baseDirOverride string

// SetBaseDirForTesting overrides the cache base directory.
// Call with "" to restore the default (~/.nimbopacks).
func SetBaseDirForTesting(dir string) {
	baseDirOverride = dir
}

// CacheDir returns the nimbopacks cache directory (~/.nimbopacks/cache).
func CacheDir() string {
	base := baseDirOverride
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".nimbopacks")
	}
	return filepath.Join(base, "cache")
}

// BuildsDir returns the build workspaces directory.
func BuildsDir() string {
	return filepath.Join(CacheDir(), "builds")
}

// NewBuildWorkDir creates a new timestamped build workspace under BuildsDir.
// The directory is created before returning.
func NewBuildWorkDir() (string, error) {
	if err := os.MkdirAll(BuildsDir(), 0o755); err != nil {
		return "", fmt.Errorf("creating builds dir: %w", err)
	}
	dir, err := os.MkdirTemp(BuildsDir(), time.Now().Format("20060102-150405-"))
	if err != nil {
		return "", fmt.Errorf("creating build workspace: %w", err)
	}
	return dir, nil
}

// CacheEntry describes one cache location with its current disk usage.
type CacheEntry struct {
	Name  string
	Path  string
	Size  int64 // total recursive size in bytes
	Count int   // number of direct children (e.g. build workspace directories)
}

// List returns all known nimbopacks cache locations with current sizes.
func List() ([]CacheEntry, error) {
	entries := []CacheEntry{
		{Name: "build workspaces", Path: BuildsDir()},
	}
	for i := range entries {
		size, count, err := dirStats(entries[i].Path)
		if err != nil {
			return nil, err
		}
		entries[i].Size = size
		entries[i].Count = count
	}
	return entries, nil
}

// Clean removes the contents of each entry's path and recreates the empty dir.
// On partial failure it continues cleaning remaining entries and returns a
// combined error.
func Clean(entries []CacheEntry) error {
	var errs []error
	for _, e := range entries {
		if err := os.RemoveAll(e.Path); err != nil {
			errs = append(errs, fmt.Errorf("removing %s: %w", e.Path, err))
			continue
		}
		if err := os.MkdirAll(e.Path, 0o755); err != nil {
			errs = append(errs, fmt.Errorf("recreating %s: %w", e.Path, err))
		}
	}
	return errors.Join(errs...)
}

// dirStats returns total recursive size in bytes and the count of direct
// children of dir. Returns 0, 0, nil when dir does not exist.
func dirStats(dir string) (int64, int, error) {
	children, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	count := len(children)
	var size int64
	err = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return size, count, nil
}
