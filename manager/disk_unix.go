//go:build !windows

package manager

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
)

// DiskStats holds usage information for a filesystem.
type DiskStats struct {
	Path    string
	Total   uint64
	Used    uint64
	Free    uint64
	Percent float64
}

// recordingDir extracts the base directory from a filename pattern like
// "videos/{{.Username}}_{{.Year}}-..." → "videos".
func recordingDir(pattern string) string {
	idx := strings.Index(pattern, "{{")
	if idx == -1 {
		// No template variables, use the directory of the pattern itself
		dir := filepath.Dir(pattern)
		if dir == "" || dir == "." {
			return "."
		}
		return filepath.Clean(dir)
	}
	dir := filepath.Dir(pattern[:idx])
	if dir == "" || dir == "." {
		return "."
	}
	return filepath.Clean(dir)
}

func getDiskStats(path string) (DiskStats, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return DiskStats{}, fmt.Errorf("statfs %s: %w", path, err)
	}
	
	// Bavail = blocks available to non-privileged users (actual usable space)
	// This is what 'df -h' shows as "Available"
	available := stat.Bavail * uint64(stat.Bsize)
	
	// For "Total", show available + used (not the entire filesystem)
	// This gives a realistic view of usable space
	totalFree := stat.Bfree * uint64(stat.Bsize)
	totalBlocks := stat.Blocks * uint64(stat.Bsize)
	used := totalBlocks - totalFree
	
	// Total usable = available + used
	totalUsable := available + used
	
	var pct float64
	if totalUsable > 0 {
		pct = float64(used) / float64(totalUsable) * 100
	}
	
	return DiskStats{
		Path:    path,
		Total:   totalUsable,
		Used:    used,
		Free:    available,
		Percent: pct,
	}, nil
}
