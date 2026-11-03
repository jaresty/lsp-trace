//go:build darwin || linux

package main

import (
	"encoding/json"
	"io"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf8"
)

func applicationValidTerminal(raw []byte) bool {
	var terminal applicationTerminal
	return utf8.Valid(raw) && json.Unmarshal(raw, &terminal) == nil && terminal.Schema == "private-host-application-terminal.v1" && terminal.Status != ""
}

func applicationReadBounded(path string, limit int) ([]byte, error) {
	return applicationReadFileBounded(path, limit, true)
}

// Only the caller-owned request locator uses this native bootstrap contract.
// Its filename is unrestricted; it is not a published terminal/ready/source
// artifact and confers no admission before the existing canonical config gates.
func applicationReadLocatorBounded(path string, limit int) ([]byte, error) {
	return applicationReadFileBounded(path, limit, false)
}
func applicationReadFileBounded(path string, limit int, published bool) ([]byte, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	// Publication is a finite, prepaid nonblocking observation. A locked inode
	// is in progress, distinct from durable-invalid bytes checked below.
	if e = applicationPublicationLock(f); e != nil {
		return nil, e
	}
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() <= 0 || st.Size() > int64(limit) || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 {
		return nil, product.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(raw) > limit || int64(len(raw)) != st.Size() {
		return nil, product.ErrInvalid
	}
	last, se := f.Stat()
	if se != nil || applicationFileIdentity(last) != applicationFileIdentity(st) {
		return nil, product.ErrInvalid
	}
	if e != nil {
		return nil, e
	}
	if !published {
		return raw, f.Close()
	}
	root, e := os.OpenRoot(filepath.Dir(path))
	if e != nil {
		return nil, e
	}
	defer root.Close()
	if e = applicationPublicationAdmitted(root, filepath.Base(path), f, raw); e != nil {
		return nil, e
	}
	return raw, f.Close()
}
