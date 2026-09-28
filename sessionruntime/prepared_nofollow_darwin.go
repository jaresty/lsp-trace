//go:build darwin

package sessionruntime

import (
	"errors"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// readPreparedNoFollow walks every component from / using directory descriptors.
// The bytes are read from the same verified regular leaf descriptor.
func readPreparedNoFollow(workspace, document string) ([]byte, error) {
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace || !filepath.IsAbs(document) || filepath.Clean(document) != document || workspace == "/" {
		return nil, errors.New("noncanonical prepared root or document")
	}
	rel, err := filepath.Rel(workspace, document)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, errors.New("outside prepared root")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unix.Close(fd) }()
	components := strings.Split(strings.TrimPrefix(workspace, "/"), "/")
	components = append(components, strings.Split(rel, string(filepath.Separator))...)
	for i, component := range components {
		if component == "" || component == "." || component == ".." {
			return nil, errors.New("invalid prepared component")
		}
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
		if i < len(components)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, e := unix.Openat(fd, component, flags, 0)
		if e != nil {
			return nil, e
		}
		_ = unix.Close(fd)
		fd = next
	}
	file := os.NewFile(uintptr(fd), "prepared-leaf")
	// file owns fd from this point; suppress the deferred close of the same descriptor.
	fd = -1
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("prepared leaf not regular")
	}
	text, err := io.ReadAll(io.LimitReader(file, MaxDocumentSupplyBytes+1))
	if err != nil || len(text) == 0 || len(text) > MaxDocumentSupplyBytes {
		return nil, errors.New("prepared leaf empty or unbounded")
	}
	return text, nil
}
