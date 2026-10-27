//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type scopeBinding struct {
	parent, file *os.File
	name         string
	info         os.FileInfo
	directory    bool
	changed      unix.Timespec
}

func scopeIdentity(info os.FileInfo) scopeFileIdentity {
	s := info.Sys().(*syscall.Stat_t)
	return scopeFileIdentity{uint64(s.Dev), uint64(s.Ino)}
}
func scopeSame(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// readFeatureScope owns every descriptor until validation and closes before any
// output escapes. Source roots may be temporary; this is NOT durable retention.
// Pinned directories survive post-admission rename. Named descendants must still
// bind to their observed identities. Checks are safe-boundary cancellation, not
// arbitrary-I/O interruption, actor monitoring, or an atomic checkout snapshot.
func readFeatureScope(ctx context.Context, root string, ops scopeReadOps) (out []scopeObservedFile, identity scopeFileIdentity, err error) {
	var owned []*os.File
	closeFile := ops.close
	if closeFile == nil {
		closeFile = func(f *os.File) error { return f.Close() }
	}
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			err = errors.Join(err, closeFile(owned[i]))
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			out = nil
			identity = scopeFileIdentity{}
		}
	}()
	hook := func(stage, path string) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if ops.hook != nil {
			if e := ops.hook(stage, path); e != nil {
				return e
			}
		}
		return ctx.Err()
	}
	open := func(parent *os.File, name string, dir bool) (*os.File, error) {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
		if dir {
			flags |= unix.O_DIRECTORY
		}
		var fd int
		var e error
		if parent == nil {
			fd, e = unix.Open(name, flags, 0)
		} else {
			fd, e = unix.Openat(int(parent.Fd()), name, flags, 0)
		}
		if e != nil {
			return nil, e
		}
		f := os.NewFile(uintptr(fd), name)
		owned = append(owned, f)
		return f, nil
	}
	bind := func(parent *os.File, name string, dir bool) (scopeBinding, error) {
		f, e := open(parent, name, dir)
		if e != nil {
			return scopeBinding{}, e
		}
		info, e := f.Stat()
		if e != nil {
			return scopeBinding{}, e
		}
		if (dir && !info.IsDir()) || (!dir && !info.Mode().IsRegular()) {
			return scopeBinding{}, ErrInvalid
		}
		var st unix.Stat_t
		if e = unix.Fstat(int(f.Fd()), &st); e != nil {
			return scopeBinding{}, e
		}
		return scopeBinding{parent: parent, file: f, name: name, info: info, directory: dir, changed: st.Ctim}, nil
	}
	// Reopen only descriptor-relative to the retained parent. No ambient path is
	// used to prove identity. Directory mtime may change; identity/type may not.
	validate := func(b scopeBinding) error {
		f, e := open(b.parent, b.name, b.directory)
		if e != nil {
			return e
		}
		now, e := f.Stat()
		if e != nil {
			return e
		}
		held, e := b.file.Stat()
		if e != nil {
			return e
		}
		if b.directory {
			if !now.IsDir() || !held.IsDir() || !os.SameFile(b.info, now) || !os.SameFile(b.info, held) {
				return ErrInvalid
			}
		} else {
			if !scopeSame(b.info, now) || !scopeSame(b.info, held) {
				return ErrInvalid
			}
			var namedStat, heldStat unix.Stat_t
			if e = unix.Fstat(int(f.Fd()), &namedStat); e != nil {
				return e
			}
			if e = unix.Fstat(int(b.file.Fd()), &heldStat); e != nil {
				return e
			}
			if b.changed != namedStat.Ctim || b.changed != heldStat.Ctim {
				return ErrInvalid
			}
		}
		return nil
	}
	base, e := bind(nil, "/", true)
	if e != nil {
		return nil, identity, e
	}
	current := base.file
	ancestors := []scopeBinding{base}
	for _, part := range strings.Split(strings.TrimPrefix(root, "/"), "/") {
		if part == "" {
			continue
		}
		b, e := bind(current, part, true)
		if e != nil {
			return nil, identity, e
		}
		ancestors = append(ancestors, b)
		current = b.file
	}
	if e = hook("root-opened", root); e != nil {
		return nil, identity, e
	}
	for _, b := range ancestors {
		if e = validate(b); e != nil {
			return nil, identity, e
		}
	}
	info, e := current.Stat()
	if e != nil {
		return nil, identity, e
	}
	identity = scopeIdentity(info)
	var bindings []scopeBinding
	total := 0
	for i, path := range featureScopePaths() {
		parent := current
		parts := strings.Split(path, "/")
		for _, part := range parts[:len(parts)-1] {
			b, e := bind(parent, part, true)
			if e != nil {
				return nil, identity, e
			}
			bindings = append(bindings, b)
			parent = b.file
		}
		first, e := bind(parent, parts[len(parts)-1], false)
		if e != nil {
			return nil, identity, e
		}
		bindings = append(bindings, first)
		readLimit := int64(featureScopeFileLimit)
		if remaining := int64(featureScopeTotalLimit - total); remaining < readLimit {
			readLimit = remaining
		}
		if first.info.Size() < 1 || first.info.Size() > readLimit {
			return nil, identity, ErrInvalid
		}
		if e = hook("file-opened", path); e != nil {
			return nil, identity, e
		}
		if e = validate(first); e != nil {
			return nil, identity, e
		}
		raw, e := io.ReadAll(io.LimitReader(preflightContextReader{ctx, first.file}, readLimit+1))
		if e != nil {
			return nil, identity, e
		}
		if e = hook("file-read", path); e != nil {
			return nil, identity, e
		}
		if e = validate(first); e != nil {
			return nil, identity, e
		}
		total += len(raw)
		if len(raw) == 0 || len(raw) > featureScopeFileLimit || int64(len(raw)) != first.info.Size() || total > featureScopeTotalLimit {
			return nil, identity, ErrInvalid
		}
		role := "NOMINATION"
		if i >= 4 {
			role = "CONTEXT_ONLY"
		}
		out = append(out, scopeObservedFile{scopeManifestFile: scopeManifestFile{Path: path, Role: role, SHA256: Digest(raw), ByteLength: uint64(len(raw)), Identity: scopeIdentity(first.info), Mode: uint32(first.info.Mode()), ModifiedNS: first.info.ModTime().UnixNano(), ChangedSec: int64(first.changed.Sec), ChangedNsec: int64(first.changed.Nsec)}, Bytes: bytes.Clone(raw)})
	}
	if e = hook("before-final", root); e != nil {
		return nil, identity, e
	}
	for _, b := range bindings {
		if e = validate(b); e != nil {
			return nil, identity, e
		}
	}
	return out, identity, nil
}
