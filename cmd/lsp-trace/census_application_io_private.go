//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// A held directory descriptor plus repeated namespace identity checks, not a
// same-user mutation lease. These adapters verify bytes; they never import owners.
type censusApplicationStorePrivate struct {
	ctx       context.Context
	path      string
	root      *os.Root
	identity  os.FileInfo
	remaining int64
}

func openCensusApplicationStorePrivate(ctx context.Context, path string, budget int) (*censusApplicationStorePrivate, error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || budget < 1 {
		return nil, errors.New("application store admission")
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 || before.Mode().Perm()&0077 != 0 {
		return nil, errors.New("application store must be a private real directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	s := &censusApplicationStorePrivate{ctx: ctx, path: path, root: root, identity: before, remaining: int64(budget)}
	if err = s.check(); err != nil {
		_ = root.Close()
		return nil, err
	}
	return s, nil
}
func (s *censusApplicationStorePrivate) check() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	opened, err := s.root.Stat(".")
	if err != nil {
		return err
	}
	current, err := os.Lstat(s.path)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, s.identity) || !os.SameFile(current, s.identity) {
		return errors.New("application store namespace changed")
	}
	return nil
}
func (s *censusApplicationStorePrivate) close() error { return s.root.Close() }
func (s *censusApplicationStorePrivate) charge(n int64) error {
	if n < 0 || n > s.remaining {
		return errors.New("application cumulative read budget")
	}
	s.remaining -= n
	return nil
}
func (s *censusApplicationStorePrivate) read(name string, limit int) (raw []byte, err error) {
	if filepath.Base(name) != name || name == "." || limit < 1 {
		return nil, errors.New("application leaf selector")
	}
	if err = s.check(); err != nil {
		return nil, err
	}
	f, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("application artifact must be a private regular file")
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || st.Uid != uint32(os.Getuid()) {
		return nil, errors.New("application artifact owner")
	}
	if info.Size() > int64(limit) {
		return nil, errors.New("application artifact byte bound")
	}
	if err = s.charge(info.Size()); err != nil {
		return nil, err
	}
	raw, err = io.ReadAll(io.LimitReader(censusInventoryConfigReaderPrivate{s.ctx, f}, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > limit || int64(len(raw)) != info.Size() {
		return nil, errors.New("application artifact changed or exceeded bound")
	}
	return raw, s.check()
}
func (s *censusApplicationStorePrivate) write(name string, raw []byte, limit int) (err error) {
	if filepath.Base(name) != name || len(raw) > limit {
		return errors.New("application write bound")
	}
	if err = s.check(); err != nil {
		return err
	}
	f, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	_, e := f.Write(raw)
	err = errors.Join(e, f.Sync(), f.Close(), s.check())
	return
}
func (s *censusApplicationStorePrivate) names(limit int) ([]string, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	f, err := s.root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, e := f.ReadDir(limit + 1)
	err = errors.Join(e, f.Close())
	if err != nil || len(entries) > limit {
		return nil, errors.Join(err, errors.New("application directory entry bound"))
	}
	names := make([]string, len(entries))
	for i, v := range entries {
		names[i] = v.Name()
	}
	return names, s.check()
}
func decodeCensusApplicationPrivate(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	canonical, err := json.Marshal(v)
	if err != nil || !bytes.Equal(raw, canonical) {
		return errors.New("application noncanonical artifact")
	}
	return nil
}

// All external artifact paths are scoped under their exact private parent. No
// following leaf links, FIFO waits, unbounded ReadFile, or abandoned goroutines.
func (s *censusApplicationStorePrivate) external(path string, limit int) ([]byte, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	other, err := openCensusApplicationStorePrivate(s.ctx, filepath.Dir(path), int(s.remaining))
	if err != nil {
		return nil, err
	}
	raw, e := other.read(filepath.Base(path), limit)
	ce := other.close()
	s.remaining = other.remaining
	return raw, errors.Join(e, ce, s.check())
}
