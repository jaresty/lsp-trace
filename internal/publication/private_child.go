package publication

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// OpenPrivateChild opens one canonical child through the pinned capability.
// fresh creates exclusively, never adopts, chmods, or falls back to another root.
// A failed fresh admission removes only an empty child, never retained contents.
func (r *Root) OpenPrivateChild(name string, fresh bool) (out *Root, err error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return nil, ErrBoundFilePolicy
	}
	if err = r.ValidatePrivate(); err != nil {
		return nil, err
	}
	if fresh {
		if err = r.handle.Mkdir(name, 0700); err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				err = errors.Join(err, removeEmptyChildDirectory(r.file, name))
			}
		}()
	}
	before, err := r.handle.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, ErrBoundFilePolicy
	}
	f, err := openObservedDirectory(r.file, name)
	if err != nil {
		return nil, err
	}
	child, err := r.handle.OpenRoot(name)
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	out = &Root{path: filepath.Join(r.path, name), file: f, handle: child, info: before}
	info, statErr := child.Stat(".")
	if statErr != nil || !os.SameFile(before, info) {
		err = errors.Join(ErrBoundFilePolicy, statErr)
	} else {
		err = out.ValidatePrivate()
	}
	if err == nil && fresh {
		var synced bool
		synced, err = r.SyncDirectory()
		if !synced {
			err = errors.Join(err, errors.New("child namespace sync unavailable"))
		}
	}
	if err != nil {
		err = errors.Join(err, out.Close())
		return nil, err
	}
	return out, nil
}

// RemoveEmptyPrivateChild is pre-header cleanup only. Remove never recurses.
func (r *Root) RemoveEmptyPrivateChild(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return ErrBoundFilePolicy
	}
	if err := r.ValidatePrivate(); err != nil {
		return err
	}
	if err := removeEmptyChildDirectory(r.file, name); err != nil {
		return err
	}
	ok, err := r.SyncDirectory()
	if !ok {
		return errors.Join(err, errors.New("child removal sync unavailable"))
	}
	return err
}
