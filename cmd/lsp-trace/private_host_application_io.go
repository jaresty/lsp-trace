//go:build darwin || linux

package main

import (
	"io"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"syscall"
)

func applicationReadBounded(path string, limit int) ([]byte, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() > int64(limit) || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 {
		return nil, product.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(raw) > limit {
		return nil, product.ErrInvalid
	}
	return raw, e
}
