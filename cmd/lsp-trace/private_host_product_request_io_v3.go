//go:build darwin || linux

package main

import (
	"io"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"path/filepath"
	"syscall"
)

func readHostProductRequestRawV3(path string) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, product.ErrInvalid
	}
	real, e := filepath.EvalSymlinks(path)
	if e != nil || real != path {
		return nil, product.ErrInvalid
	}
	root, e := os.OpenRoot(filepath.Dir(path))
	if e != nil {
		return nil, product.ErrInvalid
	}
	defer root.Close()
	f, e := root.OpenFile(filepath.Base(path), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, product.ErrInvalid
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 || st.Size() > 64<<10 {
		return nil, product.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, 64<<10+1))
	if e != nil || len(raw) > 64<<10 {
		return nil, product.ErrInvalid
	}
	return raw, nil
}
