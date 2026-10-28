//go:build darwin || linux

package graphprovenance

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strings"

	"golang.org/x/sys/unix"
)

// Source-only policy: no owner, mode-0600, publication name, or content-ID rule.
// Hooks are per-instance, package-local fault instrumentation, never public read
// authority. Root and descendant descriptors are owned, not concurrently shared.
type sourceReadOpsPrivateV1 struct {
	hook  func(stage, name string, info os.FileInfo) error
	read  func(*os.File, []byte) (int, error)
	close func(*os.File) error
}
type sourceLinkPrivateV1 struct {
	parent, file *os.File
	name         string
	stat         unix.Stat_t
}
type sourceRootPrivateV1 struct {
	file   *os.File
	links  []sourceLinkPrivateV1
	ops    sourceReadOpsPrivateV1
	closed bool
}

func openSourceRootPrivateV1(ctx context.Context, workspace string) (*sourceRootPrivateV1, error) {
	return openSourceRootWithOpsPrivateV1(ctx, workspace, sourceReadOpsPrivateV1{})
}
func sourceOpenPrivateV1(ctx context.Context, parent *os.File, name string, directory bool) (*os.File, error) {
	if ctx == nil {
		return nil, errors.New("private source context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	if directory {
		flags |= unix.O_DIRECTORY
	}
	var fd int
	var err error
	if parent == nil {
		if name != "/" {
			return nil, errors.New("private source absolute open refused")
		}
		fd, err = unix.Open("/", flags, 0)
	} else {
		fd, err = unix.Openat(int(parent.Fd()), name, flags, 0)
	}
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
func openSourceRootWithOpsPrivateV1(ctx context.Context, workspace string, ops sourceReadOpsPrivateV1) (root *sourceRootPrivateV1, err error) {
	if ctx == nil || !path.IsAbs(workspace) || path.Clean(workspace) != workspace || len(workspace) > 8192 || strings.ContainsAny(workspace, "\\\x00:") {
		return nil, errors.New("private source root path")
	}
	r := &sourceRootPrivateV1{ops: ops}
	defer func() {
		if err != nil {
			err = errors.Join(err, r.Close())
			root = nil
		}
	}()
	parts := strings.Split(strings.TrimPrefix(workspace, "/"), "/")
	if workspace == "/" {
		parts = nil
	}
	if len(parts) > MaxDepthV2 {
		return nil, errors.New("private source root depth")
	}
	parts = append([]string{"/"}, parts...)
	var parent *os.File
	for _, name := range parts {
		if err = r.event(ctx, "before-root-component", name, nil); err != nil {
			return nil, err
		}
		var f *os.File
		f, err = sourceOpenPrivateV1(ctx, parent, name, true)
		if err != nil {
			return nil, err
		}
		link := sourceLinkPrivateV1{parent: parent, file: f, name: name}
		r.links = append(r.links, link)
		if err = unix.Fstat(int(f.Fd()), &r.links[len(r.links)-1].stat); err != nil {
			return nil, err
		}
		if r.links[len(r.links)-1].stat.Mode&unix.S_IFMT != unix.S_IFDIR {
			return nil, errors.New("private source root not directory")
		}
		parent = f
	}
	r.file = parent
	if err = r.event(ctx, "root-opened", workspace, nil); err != nil {
		return nil, err
	}
	if err = r.validate(ctx, r.links); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *sourceRootPrivateV1) closeFile(f *os.File) error {
	if r.ops.close != nil {
		return r.ops.close(f)
	}
	return f.Close()
}
func (r *sourceRootPrivateV1) Close() (err error) {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true
	for i := len(r.links) - 1; i >= 0; i-- {
		err = errors.Join(err, r.closeFile(r.links[i].file))
	}
	return err
}
func (r *sourceRootPrivateV1) event(ctx context.Context, stage, name string, info os.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.ops.hook != nil {
		if err := r.ops.hook(stage, name, info); err != nil {
			return err
		}
	}
	return ctx.Err()
}
func sourceSameDirectoryPrivateV1(a, b unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode&unix.S_IFMT == unix.S_IFDIR && b.Mode&unix.S_IFMT == unix.S_IFDIR
}
func sourceSameFilePrivateV1(a, b unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Mode&unix.S_IFMT == unix.S_IFREG && a.Size == b.Size && a.Mtim == b.Mtim && a.Ctim == b.Ctim && a.Nlink == b.Nlink && a.Uid == b.Uid && a.Gid == b.Gid
}
func sourceNamedStatPrivateV1(link sourceLinkPrivateV1) (s unix.Stat_t, err error) {
	fd := unix.AT_FDCWD
	if link.parent != nil {
		fd = int(link.parent.Fd())
	}
	err = unix.Fstatat(fd, link.name, &s, unix.AT_SYMLINK_NOFOLLOW)
	return
}
func (r *sourceRootPrivateV1) validate(ctx context.Context, links []sourceLinkPrivateV1) error {
	if r == nil || r.closed || r.file == nil {
		return errors.New("private source root retired")
	}
	for _, link := range links {
		if err := ctx.Err(); err != nil {
			return err
		}
		var held unix.Stat_t
		if err := unix.Fstat(int(link.file.Fd()), &held); err != nil {
			return err
		}
		named, err := sourceNamedStatPrivateV1(link)
		if err != nil {
			return err
		}
		if !sourceSameDirectoryPrivateV1(link.stat, held) || !sourceSameDirectoryPrivateV1(link.stat, named) {
			return errors.New("private source directory identity changed")
		}
	}
	return ctx.Err()
}
func readSourcePrivateV1(ctx context.Context, root *sourceRootPrivateV1, name string, limit int64) ([]byte, error) {
	return root.read(ctx, name, limit)
}
func (r *sourceRootPrivateV1) read(ctx context.Context, name string, limit int64) (raw []byte, err error) {
	if ctx == nil || limit < 1 || limit > MaxFileBytes || name == "" || name == "." || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || name == ".." || strings.ContainsAny(name, "\\\x00:") || len(name) > 8192 {
		return nil, errors.New("private source input bounds/path")
	}
	if r == nil {
		return nil, errors.New("private source root retired")
	}
	if err = r.validate(ctx, r.links); err != nil {
		return nil, err
	}
	var owned []*os.File
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			err = errors.Join(err, r.closeFile(owned[i]))
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			raw = nil
		}
	}()
	parts := strings.Split(name, "/")
	if len(parts) > MaxDepthV2 {
		return nil, errors.New("private source path depth")
	}
	parent := r.file
	links := append([]sourceLinkPrivateV1(nil), r.links...)
	for i, part := range parts[:len(parts)-1] {
		if err = r.event(ctx, "before-directory-open", strings.Join(parts[:i+1], "/"), nil); err != nil {
			return nil, err
		}
		var f *os.File
		f, err = sourceOpenPrivateV1(ctx, parent, part, true)
		if err != nil {
			return nil, err
		}
		owned = append(owned, f)
		link := sourceLinkPrivateV1{parent: parent, file: f, name: part}
		if err = unix.Fstat(int(f.Fd()), &link.stat); err != nil {
			return nil, err
		}
		if link.stat.Mode&unix.S_IFMT != unix.S_IFDIR {
			return nil, errors.New("private source component not directory")
		}
		links = append(links, link)
		parent = f
		info, e := f.Stat()
		if e != nil {
			return nil, e
		}
		if err = r.event(ctx, "directory-opened", strings.Join(parts[:i+1], "/"), info); err != nil {
			return nil, err
		}
	}
	if err = r.validate(ctx, links); err != nil {
		return nil, err
	}
	leaf := sourceLinkPrivateV1{parent: parent, name: parts[len(parts)-1]}
	before, e := sourceNamedStatPrivateV1(leaf)
	if e != nil {
		return nil, e
	}
	if before.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, errors.New("input must be a regular file")
	}
	if before.Size < 0 || before.Size > limit {
		return nil, errors.New("input exceeds byte limit")
	}
	if err = r.event(ctx, "before-file-open", name, nil); err != nil {
		return nil, err
	}
	f, e := sourceOpenPrivateV1(ctx, parent, leaf.name, false)
	if e != nil {
		return nil, e
	}
	owned = append(owned, f)
	var opened unix.Stat_t
	if err = unix.Fstat(int(f.Fd()), &opened); err != nil {
		return nil, err
	}
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	// Observe the actual opened FD before any identity rejection, so a test spy
	// cannot accidentally hide a wrong-target open behind a later mismatch.
	if err = r.event(ctx, "file-opened", name, info); err != nil {
		return nil, err
	}
	if !sourceSameFilePrivateV1(before, opened) {
		return nil, errors.New("private source file changed at open")
	}
	validateFile := func() error {
		if e := r.validate(ctx, links); e != nil {
			return e
		}
		var held unix.Stat_t
		if e := unix.Fstat(int(f.Fd()), &held); e != nil {
			return e
		}
		named, e := sourceNamedStatPrivateV1(leaf)
		if e != nil {
			return e
		}
		if !sourceSameFilePrivateV1(opened, held) || !sourceSameFilePrivateV1(opened, named) {
			return errors.New("private source file identity/metadata changed")
		}
		return ctx.Err()
	}
	if err = validateFile(); err != nil {
		return nil, err
	}
	if err = r.event(ctx, "before-read", name, info); err != nil {
		return nil, err
	}
	// Allocation is bounded by the validated opened FD's size, plus one byte to
	// distinguish EOF from growth. No path is reopened to supply these bytes.
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	buf := make([]byte, int(opened.Size)+1)
	reader := sourceContextReaderPrivateV1{ctx: ctx, file: f, read: r.ops.read}
	n, readErr := io.ReadFull(reader, buf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return nil, readErr
	}
	if int64(n) != opened.Size {
		return nil, errors.New("private source read length changed")
	}
	if err = r.event(ctx, "file-read", name, info); err != nil {
		return nil, err
	}
	if err = validateFile(); err != nil {
		return nil, err
	}
	return buf[:n:n], nil
}

type sourceContextReaderPrivateV1 struct {
	ctx  context.Context
	file *os.File
	read func(*os.File, []byte) (int, error)
}

func (r sourceContextReaderPrivateV1) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	var n int
	var err error
	if r.read == nil {
		n, err = r.file.Read(p)
	} else {
		n, err = r.read(r.file, p)
	}
	if canceled := r.ctx.Err(); canceled != nil {
		return n, errors.Join(err, canceled)
	}
	return n, err
}
