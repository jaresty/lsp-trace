package censuslineagepilot

import (
	"os"
	"path/filepath"
	"strings"
)

// ValidateDurableRoot validates an existing caller-owned private root without mutating it.
func ValidateDurableRoot(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrInvalid
	}
	privateTmp := string(filepath.Separator) + filepath.Join("private", "tmp")
	if path == privateTmp || strings.HasPrefix(path, privateTmp+string(filepath.Separator)) {
		return ErrInvalid
	}
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(path, current), string(filepath.Separator))
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalid
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return ErrInvalid
	}
	return nil
}
