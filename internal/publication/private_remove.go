package publication

import (
	"errors"
	"os"
)

// PrivateRemoval reports namespace removal separately from directory durability.
// Neither field proves secure erasure, absence of other links/copies, or a
// 24-hour retention policy. The operation does not issue an evidence receipt.
type PrivateRemoval struct {
	Removed         bool
	DirectorySynced bool
}

// RemovePrivateFile removes one existing private regular file relative to the
// pinned root capability. It refuses symlinks, directories, non-private files,
// unsafe selectors, and closed/non-private roots. It cannot enumerate or remove
// sibling artifacts, backups, or publication temporaries; callers must account
// for those independently before asserting a retention deadline.
func (r *Root) RemovePrivateFile(selector string) (PrivateRemoval, error) {
	if err := r.ValidatePrivate(); err != nil {
		return PrivateRemoval{}, err
	}
	t, err := existingTarget(r, selector)
	if err != nil {
		return PrivateRemoval{}, err
	}
	defer t.close()
	parentInfo, err := t.parent.Stat(".")
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode().Perm()&0o077 != 0 {
		return PrivateRemoval{}, errors.New("private selector parent required")
	}
	// Match PublishBoundFile's pinned-parent/final-component lock identity.
	// Unlike capabilityTarget, existingTarget never creates missing directories.
	lock := targetLock(targetLockKey(parentInfo, t.name))
	lock.Lock()
	defer lock.Unlock()
	before, err := t.parent.Lstat(t.name)
	if err != nil {
		return PrivateRemoval{}, err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0o077 != 0 {
		return PrivateRemoval{}, errors.New("private regular selector required")
	}
	file, err := t.parent.Open(t.name)
	if err != nil {
		return PrivateRemoval{}, err
	}
	after, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil || closeErr != nil || !os.SameFile(before, after) {
		return PrivateRemoval{}, errors.New("private selector identity changed")
	}
	beforeRemove, err := t.parent.Lstat(t.name)
	if err != nil || !os.SameFile(before, beforeRemove) {
		return PrivateRemoval{}, errors.New("private selector identity changed")
	}
	if err := t.parent.Remove(t.name); err != nil {
		return PrivateRemoval{}, err
	}
	result := PrivateRemoval{Removed: true}
	parent, err := t.parent.Open(".")
	if err != nil {
		return result, err
	}
	syncErr := parent.Sync()
	closeErr = parent.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return result, err
	}
	result.DirectorySynced = true
	return result, nil
}
