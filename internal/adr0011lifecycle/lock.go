// Package adr0011lifecycle contains private, synthetic-only lifecycle primitives.
// No production entry point constructs this capability.
package adr0011lifecycle

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/publication"
)

const lockDeadline = 5 * time.Second

type pinnedDirectory struct {
	path string
	info os.FileInfo
}

type syntheticRoot struct {
	root, anchor                   *publication.Root
	lock                           *os.File
	lockInfo, rootInfo, anchorInfo os.FileInfo
	ancestry                       []pinnedDirectory
	unlockForTest                  func(*os.File) error // synthetic test injection only; nil uses OS unlock
}

// exactDirectorySpelling rejects case-folding aliases on case-insensitive hosts.
// The bounded scan fails closed if an ancestor has too many entries.
func exactDirectorySpelling(path string) error {
	if path == string(filepath.Separator) {
		return nil
	}
	parent := filepath.Dir(path)
	f, err := os.Open(parent)
	if err != nil {
		return err
	}
	defer f.Close()
	const maxNames = 262144
	for scanned := 0; scanned < maxNames; {
		count := 256
		if maxNames-scanned < count {
			count = maxNames - scanned
		}
		names, e := f.Readdirnames(count)
		scanned += len(names)
		for _, name := range names {
			if name == filepath.Base(path) {
				return nil
			}
		}
		if e == io.EOF {
			return errors.New("noncanonical directory spelling")
		}
		if e != nil {
			return e
		}
		if len(names) == 0 {
			return errors.New("directory spelling cannot be verified")
		}
	}
	return errors.New("directory spelling scan limit exceeded")
}

// canonicalAncestry refuses aliases rather than rewriting the caller's path.
// Each component is pinned for repeat checks; this does not make pathname I/O
// atomic against a hostile same-UID actor swapping components between checks.
func canonicalAncestry(path string) ([]pinnedDirectory, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("clean absolute canonical path required")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if resolved != path {
		return nil, errors.New("noncanonical or symlinked path ancestry")
	}
	current := string(filepath.Separator)
	parts := []pinnedDirectory{}
	for _, component := range append([]string{""}, strings.Split(strings.TrimPrefix(path, current), current)...) {
		if component != "" {
			current = filepath.Join(current, component)
		}
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("symlinked or non-directory path ancestry")
		}
		if err := exactDirectorySpelling(current); err != nil {
			return nil, err
		}
		parts = append(parts, pinnedDirectory{path: current, info: info})
	}
	return parts, nil
}
func (l *syntheticRoot) validateAncestry() error {
	for _, p := range l.ancestry {
		current, err := os.Lstat(p.path)
		if err != nil {
			return err
		}
		if !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(current, p.info) {
			return errors.New("directory ancestry replaced")
		}
		if err := exactDirectorySpelling(p.path); err != nil {
			return err
		}
	}
	for _, path := range []string{l.root.Path(), l.anchor.Path()} {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		if resolved != path {
			return errors.New("noncanonical or symlinked path ancestry")
		}
	}
	return nil
}

// openSynthetic is deliberately package-private: neither provisioned authority nor
// durable fence/replay is established by this checkpoint.
func openSynthetic(rootPath, anchorPath string) (*syntheticRoot, error) {
	if !supportedLock {
		return nil, errors.New("unsupported OS advisory lock platform")
	}
	if filepath.Dir(rootPath) != filepath.Dir(anchorPath) || rootPath == anchorPath {
		return nil, errors.New("anchor must be a distinct sibling directory")
	}
	rootChain, err := canonicalAncestry(rootPath)
	if err != nil {
		return nil, err
	}
	anchorChain, err := canonicalAncestry(anchorPath)
	if err != nil {
		return nil, err
	}
	rootInfo, err := os.Lstat(rootPath)
	if err != nil {
		return nil, err
	}
	anchorInfo, err := os.Lstat(anchorPath)
	if err != nil {
		return nil, err
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = root.Close()
		}
	}()
	anchor, e := publication.OpenRoot(anchorPath)
	if e != nil {
		err = e
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = anchor.Close()
		}
	}()
	if err = root.ValidatePrivate(); err != nil {
		return nil, err
	}
	if err = anchor.ValidatePrivate(); err != nil {
		return nil, err
	}
	name := filepath.Join(rootPath, "adr0011-root.lock")
	// Exclusive creation never truncates an existing lock. Existing files are
	// opened no-follow and checked against their pathname and link count.
	f, e := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		f, e = openLockNoFollow(name)
	}
	if e != nil {
		err = e
		return nil, err
	}
	info, e := f.Stat()
	if e != nil {
		f.Close()
		err = e
		return nil, err
	}
	l := &syntheticRoot{root: root, anchor: anchor, lock: f, lockInfo: info, rootInfo: rootInfo, anchorInfo: anchorInfo, ancestry: append(rootChain, anchorChain...)}
	if e = l.validate(); e != nil {
		f.Close()
		err = e
		return nil, err
	}
	return l, nil
}
func (l *syntheticRoot) close() error {
	if l == nil {
		return nil
	}
	var e error
	if l.lock != nil {
		e = l.lock.Close()
	}
	if l.anchor != nil {
		_ = l.anchor.Close()
	}
	if l.root != nil {
		_ = l.root.Close()
	}
	return e
}
func (l *syntheticRoot) validate() error {
	if l == nil || l.lock == nil {
		return errors.New("closed lifecycle lock")
	}
	if err := l.validateAncestry(); err != nil {
		return err
	}
	for _, pair := range []struct {
		r      *publication.Root
		pinned os.FileInfo
	}{{l.root, l.rootInfo}, {l.anchor, l.anchorInfo}} {
		if err := pair.r.ValidatePrivate(); err != nil {
			return err
		}
		current, err := os.Lstat(pair.r.Path())
		if err != nil {
			return err
		}
		if !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(current, pair.pinned) {
			return errors.New("directory path replaced")
		}
	}
	info, err := l.lock.Stat()
	if err != nil {
		return err
	}
	path, err := os.Lstat(filepath.Join(l.root.Path(), "adr0011-root.lock"))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || !os.SameFile(info, l.lockInfo) || !os.SameFile(path, info) || path.Mode()&os.ModeSymlink != 0 || !singleLink(info) {
		return errors.New("lock pathname, inode, mode or link count changed")
	}
	return nil
}
func (l *syntheticRoot) withLock(fn func() error) (result error) {
	if err := l.validate(); err != nil {
		return err
	}
	deadline := time.Now().Add(lockDeadline)
	for {
		err := tryExclusive(l.lock)
		if err == nil {
			break
		}
		if !lockBusy(err) {
			return fmt.Errorf("OS advisory lock denied: %w", err)
		}
		if !time.Now().Before(deadline) {
			return errors.New("lock deadline exceeded")
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer func() {
		unlock := unlockExclusive
		if l.unlockForTest != nil {
			unlock = l.unlockForTest
		}
		if err := unlock(l.lock); err != nil {
			result = errors.Join(result, fmt.Errorf("OS advisory unlock failed: %w", err))
		}
	}()
	if err := l.validate(); err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	return l.validate()
}
