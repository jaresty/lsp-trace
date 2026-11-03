//go:build darwin || linux

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	product "lsp-trace/internal/privatehostproductv3"
)

// The root is pinned for the entire finalize invocation. No caller operand can
// select a publication path; all names are fixed or built from checked digests.
func applicationFinalizeOpenRoot(path string) (*os.Root, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, product.ErrInvalid
	}
	st, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !st.IsDir() || st.Mode().Perm() != 0700 {
		return nil, product.ErrInvalid
	}
	root, e := os.OpenRoot(path)
	if e != nil {
		return nil, e
	}
	pinned, e := root.Stat(".")
	if e != nil || !os.SameFile(st, pinned) {
		root.Close()
		return nil, product.ErrInvalid
	}
	return root, nil
}
func applicationFinalizeOpen(root *os.Root, name string, flags int, lanes ...*applicationFinalizeLane) (*os.File, error) {
	if e := applicationFinalizeCheck(lanes...); e != nil {
		return nil, e
	}
	// Root confinement alone permits symlinks within the root. Refuse every
	// component, including intermediate object directories, as well as the leaf.
	parts := stringsForFinalizePath(name)
	if len(parts) == 0 {
		return nil, product.ErrInvalid
	}
	for i := 1; i < len(parts); i++ {
		if e := applicationFinalizeCheck(lanes...); e != nil {
			return nil, e
		}
		st, e := root.Lstat(filepath.Join(parts[:i]...))
		if ce := applicationFinalizeCheck(lanes...); ce != nil {
			return nil, ce
		}
		if e != nil {
			return nil, e
		}
		if !st.IsDir() || st.Mode().Perm() != 0700 {
			return nil, product.ErrInvalid
		}
	}
	if e := applicationFinalizeCheck(lanes...); e != nil {
		return nil, e
	}
	f, e := root.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if ce := applicationFinalizeCheck(lanes...); ce != nil {
		if f != nil {
			f.Close()
		}
		return nil, ce
	}
	return f, e
}
func stringsForFinalizePath(name string) []string {
	if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || name == "." || name == ".." {
		return nil
	}
	var parts []string
	for name != "." {
		dir, base := filepath.Split(name)
		if base == ".." || base == "" {
			return nil
		}
		parts = append([]string{base}, parts...)
		name = filepath.Clean(dir)
	}
	return parts
}
func applicationFinalizeRead(root *os.Root, name string, limit int, lanes ...*applicationFinalizeLane) ([]byte, error) {
	f, e := applicationFinalizeOpen(root, name, os.O_RDONLY, lanes...)
	if e != nil {
		return nil, e
	}
	b, e := applicationFinalizeReadFile(f, limit, lanes...)
	if e == nil {
		e = applicationPublicationAdmitted(root, name, f, b, lanes...)
	}
	ce := f.Close()
	return b, errors.Join(e, ce, applicationFinalizeCheck(lanes...))
}
func applicationFinalizeReadFile(f *os.File, limit int, lanes ...*applicationFinalizeLane) ([]byte, error) {
	if e := applicationFinalizeCheck(lanes...); e != nil {
		return nil, e
	}
	if e := applicationPublicationLock(f, lanes...); e != nil {
		return nil, e
	}
	st, e := f.Stat()
	if ce := applicationFinalizeCheck(lanes...); ce != nil {
		return nil, ce
	}
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 || st.Size() > int64(limit) {
		return nil, product.ErrInvalid
	}
	var input io.Reader = f
	for _, l := range lanes {
		input = applicationFinalizeReader{input, l}
	}
	b, e := io.ReadAll(io.LimitReader(input, int64(limit)+1))
	if e != nil || len(b) > limit || int64(len(b)) != st.Size() {
		return nil, product.ErrInvalid
	}
	last, e := f.Stat()
	if ce := applicationFinalizeCheck(lanes...); ce != nil {
		return nil, ce
	}
	if e != nil || applicationFileIdentity(last) != applicationFileIdentity(st) {
		return nil, product.ErrInvalid
	}
	return b, nil
}

// Equal readable bytes are NOT proof of durable publication. Both the create
// and equal-existing paths must complete file Sync/Close and directory
// Sync/Close. Failed or partial records are never removed, rewritten or repaired.
func applicationFinalizePublish(root *os.Root, name string, raw []byte, equalOK bool, lanes ...*applicationFinalizeLane) error {
	if e := applicationFinalizeCheck(lanes...); e != nil {
		return e
	}
	if len(raw) > 64<<10 {
		return product.ErrInvalid
	}
	f, e := applicationFinalizeOpen(root, name, os.O_RDWR|os.O_CREATE|os.O_EXCL, lanes...)
	existing := os.IsExist(e)
	if existing && equalOK {
		f, e = applicationFinalizeOpen(root, name, os.O_RDWR, lanes...)
	}
	if e != nil {
		return e
	}
	return applicationFinalizeComplete(root, f, raw, existing, lanes...)
}

// Charging an exclusive name is admission, including for an already-expired
// caller. Only that bounded create uses the admission slice. All subsequent IO
// uses the effective application deadline; an empty/torn name stays spent.
func applicationFinalizeCharge(root *os.Root, name string, admission, lane *applicationFinalizeLane) error {
	f, e := applicationFinalizeOpen(root, name, os.O_RDWR|os.O_CREATE|os.O_EXCL, admission)
	if e != nil {
		return e
	}
	return applicationFinalizeComplete(root, f, []byte("1"), false, lane)
}

func applicationFinalizeComplete(root *os.Root, f *os.File, raw []byte, existing bool, lanes ...*applicationFinalizeLane) error {
	if ce := applicationFinalizeCheck(lanes...); ce != nil {
		return errors.Join(ce, f.Close())
	}
	if existing {
		prior, re := applicationFinalizeReadFile(f, len(raw), lanes...)
		if ce := applicationFinalizeCheck(lanes...); ce != nil {
			return errors.Join(ce, f.Close())
		}
		if re != nil || !bytes.Equal(prior, raw) {
			return errors.Join(product.ErrInvalid, re, f.Close())
		}
	} else {
		if ce := applicationFinalizeCheck(lanes...); ce != nil {
			return errors.Join(ce, f.Close())
		}
		n, we := f.Write(raw)
		if ce := applicationFinalizeCheck(lanes...); ce != nil {
			return errors.Join(ce, f.Close())
		}
		if we != nil || n != len(raw) {
			return errors.Join(io.ErrShortWrite, we, f.Close())
		}
	}
	if ce := applicationFinalizeCheck(lanes...); ce != nil {
		return errors.Join(ce, f.Close())
	}
	e := f.Sync()
	ce := f.Close() // Cleanup is permitted even when Sync returns late.
	if e != nil || ce != nil {
		return errors.Join(e, ce)
	}
	if ce := applicationFinalizeCheck(lanes...); ce != nil {
		return ce
	}
	// Names used for publication are root-level, so sync the pinned root rather
	// than reopening a caller-controlled absolute pathname.
	dir, e := root.Open(".")
	if e != nil {
		return e
	}
	if ce := applicationFinalizeCheck(lanes...); ce != nil {
		return errors.Join(ce, dir.Close())
	}
	e = dir.Sync()
	ce = dir.Close()
	return errors.Join(e, ce, applicationFinalizeCheck(lanes...))
}
