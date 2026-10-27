package publication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrObservedRootAdmission = errors.New("observed root admission rejected")
var ErrObservedRootUnsupported = errors.New("no-follow observed root acquisition unsupported")

// ObservedRoot is a read-only capability, not a publication capability. Its two
// underlying capabilities refer to the exact root admitted by no-follow traversal. No path
// is reopened during content observation. Do not use or close it concurrently.
type ObservedRoot struct {
	root        *Root
	closeFile   func(*os.File) error
	closeHandle func(*os.Root) error
}

// OpenObservedRoot admits a clean absolute existing private current-user root,
// excluding /private/tmp and its descendants. On supported platforms every path
// component is opened relative to its pinned parent with O_DIRECTORY|O_NOFOLLOW.
// A secondary os.Root is acquired via the path ONLY with exact descriptor
// identity equality checked before return; a substituted path cannot authorize a
// different root. This is not a before/after pathname validation scheme.
// All acquired handles are closed on failure and close errors are joined.
// Cancellation checks occur at safe boundaries, not inside arbitrary kernel I/O.
func OpenObservedRoot(ctx context.Context, path string) (*ObservedRoot, error) {
	return openObservedRoot(ctx, path, defaultObservedRootOps())
}

type observedRootOps struct {
	openDir     func(*os.File, string) (*os.File, error)
	statFile    func(*os.File) (os.FileInfo, error)
	openHandle  func(string) (*os.Root, error)
	statHandle  func(*os.Root) (os.FileInfo, error)
	closeFile   func(*os.File) error
	closeHandle func(*os.Root) error
}

func defaultObservedRootOps() observedRootOps {
	return observedRootOps{
		openDir:     openObservedDirectory,
		statFile:    func(f *os.File) (os.FileInfo, error) { return f.Stat() },
		openHandle:  os.OpenRoot,
		statHandle:  func(h *os.Root) (os.FileInfo, error) { return h.Stat(".") },
		closeFile:   func(f *os.File) error { return f.Close() },
		closeHandle: func(h *os.Root) error { return h.Close() },
	}
}
func openObservedRoot(ctx context.Context, path string, ops observedRootOps) (out *ObservedRoot, err error) {
	if ctx == nil {
		return nil, ErrObservedRootAdmission
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/private/tmp" || strings.HasPrefix(path, "/private/tmp/") {
		return nil, ErrObservedRootAdmission
	}
	var current *os.File
	var handle *os.Root
	// Ownership is transferred only if the entire acquisition, including the last
	// cancellation check, succeeds. Even handles returned alongside errors close.
	defer func() {
		err = errors.Join(err, ctx.Err())
		if err != nil {
			if handle != nil {
				err = errors.Join(err, ops.closeHandle(handle))
			}
			if current != nil {
				err = errors.Join(err, ops.closeFile(current))
			}
			err = errors.Join(ErrObservedRootAdmission, err)
			out = nil
		}
	}()
	current, err = ops.openDir(nil, "/")
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrObservedRootAdmission
	}
	// Canonical absolute Unix paths have no empty, dot or parent components.
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		child, openErr := ops.openDir(current, part)
		closeErr := ops.closeFile(current)
		current = child
		if err = errors.Join(openErr, closeErr); err != nil {
			return nil, err
		}
		if current == nil {
			return nil, ErrObservedRootAdmission
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	info, err := ops.statFile(current)
	if err != nil {
		return nil, err
	}
	if info == nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, ErrObservedRootAdmission
	}
	if err = validateRootOwner(info); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	handle, err = ops.openHandle(path)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, ErrObservedRootAdmission
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	handleInfo, err := ops.statHandle(handle)
	if err != nil {
		return nil, err
	}
	if handleInfo == nil || !os.SameFile(info, handleInfo) {
		return nil, ErrObservedRootAdmission
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	root := &Root{path: path, file: current, handle: handle, info: info}
	if err = root.ValidatePrivate(); err != nil {
		return nil, err
	}
	return &ObservedRoot{root: root, closeFile: ops.closeFile, closeHandle: ops.closeHandle}, nil
}
func (r *ObservedRoot) ValidatePrivate() error {
	if r == nil || r.root == nil {
		return ErrObservedRootAdmission
	}
	if err := r.root.ValidatePrivate(); err != nil {
		return err
	}
	if r.root.handle == nil {
		return ErrObservedRootAdmission
	}
	info, err := r.root.handle.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(r.root.info, info) {
		return ErrObservedRootAdmission
	}
	return nil
}
func (r *ObservedRoot) ReadContentObserved(ctx context.Context, id string, limit int64) ([]byte, error) {
	if r == nil || r.root == nil {
		return nil, ErrObservedRootAdmission
	}
	return r.root.ReadContentObserved(ctx, id, limit)
}

// Close attempts both capability closes and preserves both errors. It is
// idempotent; errors never authorize use of the consumed capability afterward.
func (r *ObservedRoot) Close() error {
	if r == nil || r.root == nil {
		return nil
	}
	root := r.root
	r.root = nil
	var err error
	if root.handle != nil {
		err = errors.Join(err, r.closeHandle(root.handle))
		root.handle = nil
	}
	if root.file != nil {
		err = errors.Join(err, r.closeFile(root.file))
		root.file = nil
	}
	return err
}
