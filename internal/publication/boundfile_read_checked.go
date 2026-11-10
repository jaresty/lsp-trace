package publication

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ReadVerifiedBoundFileChecked has the custody checks of ReadVerifiedBoundFile,
// but a successful return also requires successful release of every descriptor
// opened by this read. It borrows root, never reopens its pathname, and does not
// close root's file or directory handle. Digest/expected-byte checks remain the
// caller's responsibility: selector bytes alone are not trusted hash authority.
func ReadVerifiedBoundFileChecked(root *Root, selector string, limit int64) (raw []byte, err error) {
	if root == nil || root.handle == nil || limit < 1 || limit == 1<<63-1 {
		return nil, errors.Join(ErrBoundFilePolicy, errors.New("private root and finite positive bound required"))
	}
	if err = root.ValidatePrivate(); err != nil {
		return nil, errors.Join(ErrBoundFilePolicy, err)
	}
	if selector == "" || strings.IndexByte(selector, 0) >= 0 || filepath.IsAbs(selector) || !filepath.IsLocal(selector) {
		return nil, ErrBoundFilePolicy
	}
	parts, err := selectorPathParts(runtime.GOOS, selector)
	if err != nil {
		return nil, errors.Join(ErrBoundFilePolicy, err)
	}
	pinned, err := root.handle.Stat(".")
	if err != nil || !os.SameFile(root.info, pinned) {
		return nil, errors.Join(ErrBoundFilePolicy, err)
	}

	// Unlike existingTarget, this walk retains every opened parent until checked
	// release. Each child enters ownership immediately, including on failed Stat.
	// The borrowed canonical root is deliberately absent from parents.
	handles := &boundFileReadHandles{}
	defer func() {
		if handles != nil {
			handles.finish(&raw, &err)
		}
	}()
	current := root.file
	for _, part := range parts[:len(parts)-1] {
		// This existing primitive is O_DIRECTORY|O_NOFOLLOW on supported
		// systems. It does not acquire or reopen the borrowed root pathname.
		child, e := openObservedDirectory(current, part)
		if e != nil {
			return nil, errors.Join(ErrBoundFilePolicy, e)
		}
		handles.parents = append(handles.parents, child)
		info, e := child.Stat()
		if e != nil || !info.IsDir() {
			return nil, errors.Join(ErrBoundFilePolicy, e)
		}
		current = child
	}
	// A descriptor-only private view supplies the existing no-follow/nonblocking
	// data opener. It is not an OpenRoot result and is never closed as a Root.
	file, err := openObservedContent(&Root{file: current}, parts[len(parts)-1])
	if err != nil {
		return nil, err
	}
	handles.data = file
	before, err := file.Stat()
	if err != nil {
		return nil, errors.Join(ErrBoundFilePolicy, err)
	}
	owned := handles
	handles = nil // transfer all newly owned handles, not the borrowed root
	return owned.read(root.info, before, limit)
}

// Only this private IO boundary is substitutable by in-package fault tests.
// Production supplies only the actual descriptor values opened above;
// there is no global hook, exported cleanup assertion, or caller callback.
type boundFileReadData interface {
	io.ReadCloser
	Stat() (os.FileInfo, error)
}
type boundFileReadHandles struct {
	data    boundFileReadData
	parents []io.Closer // acquisition order; root itself is never owned here
}

func checkedBoundFileMetadata(rootInfo, fileInfo os.FileInfo) bool {
	return fileInfo != nil && fileInfo.Mode().IsRegular() && fileInfo.Mode().Perm() == 0o600 && validPublishedMetadata(rootInfo, fileInfo)
}

func (h *boundFileReadHandles) finish(raw *[]byte, err *error) {
	if h.data != nil {
		*err = errors.Join(*err, h.data.Close())
		h.data = nil
	}
	for i := len(h.parents) - 1; i >= 0; i-- {
		*err = errors.Join(*err, h.parents[i].Close())
	}
	h.parents = nil
	if *err != nil {
		*raw = nil
	}
}

func (h *boundFileReadHandles) read(rootInfo, before os.FileInfo, limit int64) (raw []byte, err error) {
	defer h.finish(&raw, &err)
	if h.data == nil || limit < 1 || limit == 1<<63-1 || !checkedBoundFileMetadata(rootInfo, before) {
		return nil, ErrBoundFilePolicy
	}
	if before.Size() > limit {
		return nil, ErrBoundFileLimit
	}
	opened, err := h.data.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.Join(ErrBoundFilePolicy, err)
	}
	raw, err = io.ReadAll(io.LimitReader(h.data, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, ErrBoundFileLimit
	}
	after, err := h.data.Stat()
	if err != nil || !os.SameFile(opened, after) || after.Size() != int64(len(raw)) || !checkedBoundFileMetadata(rootInfo, after) {
		return nil, errors.Join(ErrBoundFilePolicy, err)
	}
	return raw, nil
}
