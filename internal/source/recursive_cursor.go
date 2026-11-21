package source

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// RecursiveCursor owns directory offsets for a single traversal. Each Next
// performs at most one positive-count directory read. It is not resumable.
type RecursiveCursor struct {
	root   *os.Root
	paths  []string
	prunes []string
	file   *os.File
	path   string
	limits Limits
	work   int
	seen   map[string]bool
}

func OpenRecursiveCursor(base string, paths, prunes []string, limits Limits) (*RecursiveCursor, error) {
	if len(paths) == 0 || limits.MaxEntries < 1 || limits.MaxWork < 1 || limits.MaxPathBytes < 1 || limits.MaxDepth < 1 {
		return nil, ErrDiscoveryLimit
	}
	for _, p := range paths {
		if !filepath.IsLocal(p) || filepath.Clean(p) != p {
			return nil, ErrUnsafeSource
		}
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	return &RecursiveCursor{root: root, paths: append([]string(nil), paths...), prunes: append([]string(nil), prunes...), limits: limits, seen: make(map[string]bool)}, nil
}

func (c *RecursiveCursor) Close() error {
	if c.root == nil {
		return nil
	}
	var err error
	if c.file != nil {
		err = c.file.Close()
		c.file = nil
	}
	err = errors.Join(err, c.root.Close())
	c.root = nil
	return err
}

func (c *RecursiveCursor) Next(ctx context.Context, count int) (records []Record, done bool, err error) {
	defer func() {
		if done || err != nil {
			err = errors.Join(err, c.Close())
		}
	}()
	if c.root == nil || ctx == nil || count < 1 {
		return nil, false, ErrUnsafeSource
	}
	add := func(path string, r Record) error {
		if c.seen[path] {
			return nil
		}
		if len(c.seen) >= c.limits.MaxEntries {
			return ErrDiscoveryLimit
		}
		c.seen[path] = true
		records = append(records, r)
		return nil
	}
	check := func(path string) error {
		if !utf8.ValidString(path) {
			return ErrUnsafeSource
		}
		c.work++
		if c.work > c.limits.MaxWork || len(path) > c.limits.MaxPathBytes || pathDepth(path) > c.limits.MaxDepth {
			return ErrDiscoveryLimit
		}
		return ctx.Err()
	}
	for c.file == nil && len(c.paths) > 0 {
		path := c.paths[0]
		c.paths = c.paths[1:]
		if err := check(path); err != nil {
			return nil, false, err
		}
		if prunedDirectory(c.prunes, path) || c.seen[path] {
			continue
		}
		info, e := c.root.Lstat(path)
		if e != nil {
			if e = add(path, Record{Name: filepath.ToSlash(path), Kind: SourceOther, Inclusion: Unavailable, Reason: unavailableReason(e)}); e != nil {
				return nil, false, e
			}
			return records, len(c.paths) == 0, nil
		}
		if !info.IsDir() {
			if e = add(path, classify(filepath.ToSlash(path), info.Mode())); e != nil {
				return nil, false, e
			}
			return records, len(c.paths) == 0, nil
		}
		// Open a directory, not an arbitrary file that a racing rename could
		// replace with a blocking device or FIFO.
		directory, e := c.root.OpenRoot(path)
		if e != nil {
			return nil, false, e
		}
		f, openErr := directory.Open(".")
		if e := errors.Join(openErr, directory.Close()); e != nil {
			if f != nil {
				_ = f.Close()
			}
			return nil, false, e
		}
		opened, e := f.Stat()
		if e != nil || !os.SameFile(info, opened) {
			_ = f.Close()
			return nil, false, ErrUnsafeSource
		}
		c.file, c.path = f, path
		if e := add(path, classify(filepath.ToSlash(path), info.Mode())); e != nil {
			return nil, false, e
		}
	}
	if c.file == nil {
		return records, true, nil
	}
	if e := ctx.Err(); e != nil {
		return nil, false, e
	}
	count = min(count, c.limits.MaxWork-c.work)
	if count <= 0 {
		return nil, false, ErrDiscoveryLimit
	}
	entries, e := c.file.ReadDir(count)
	if e != nil && !errors.Is(e, io.EOF) {
		return nil, false, e
	}
	for _, entry := range entries {
		path := filepath.Join(c.path, entry.Name())
		if e := check(path); e != nil {
			return nil, false, e
		}
		if entry.IsDir() {
			if !prunedDirectory(c.prunes, path) && !c.seen[path] {
				c.paths = append(c.paths, path)
			}
			continue
		}
		if e := add(path, classify(filepath.ToSlash(path), entry.Type())); e != nil {
			return nil, false, e
		}
	}
	if len(entries) < count {
		e = c.file.Close()
		c.file = nil
		if e != nil {
			return nil, false, e
		}
	}
	return records, c.file == nil && len(c.paths) == 0, nil
}
