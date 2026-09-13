package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const BoundFileMechanism = "capability_bound_file_with_atomic_activation"

// BoundFileHooks are package-private deterministic fault/mutation hooks.
var (
	testHookBoundFileAfterVerify    func(parent *os.Root, name string)
	testHookBoundFileBeforeActivate func(parent *os.Root, name string)
	boundFileWriteAll               = writeAll
	boundFileSync                   = func(f *os.File) error { return f.Sync() }
	boundFileClose                  = func(f *os.File) error { return f.Close() }
	boundFileRemove                 = func(parent *os.Root, name string) error { return parent.Remove(name) }
)

type BoundFileReceipt struct {
	FinalSelector   string
	Digest          string
	ByteLength      uint64
	Mechanism       string
	NamespaceAtomic bool
	CrashDurability string
}

// PublishBoundFile reserves the final bundle inode no-replace, writes, syncs,
// rereads and verifies it through one handle, then atomically creates a small
// activation entry. The inactive reserved name is deliberately not resolvable
// through ReadBoundFile. Activation binds the exact inode and byte digest.
func PublishBoundFile(root *Root, selector string, raw []byte, verify func([]byte) error) (*BoundFileReceipt, error) {
	if root == nil || raw == nil || verify == nil {
		return nil, errors.New("invalid bound file request")
	}
	if err := root.ValidatePrivate(); err != nil {
		return nil, err
	}
	t, err := capabilityTarget(root, selector)
	if err != nil {
		return nil, err
	}
	defer t.close()
	lock := targetLock(t.key)
	lock.Lock()
	defer lock.Unlock()
	if _, err := t.parent.Lstat(t.name); err == nil {
		return nil, os.ErrExist
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	activation := t.name + ".active"
	if _, err := t.parent.Lstat(activation); err == nil {
		return nil, os.ErrExist
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	f, err := t.parent.OpenFile(t.name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			removeSameFile(t.parent, t.name, f)
		}
		_ = boundFileClose(f)
	}()
	if err = boundFileWriteAll(f, raw); err != nil {
		return nil, err
	}
	if err = boundFileSync(f); err != nil {
		return nil, err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	got, err := io.ReadAll(io.LimitReader(f, int64(len(raw))+1))
	if err != nil || len(got) != len(raw) {
		return nil, errors.Join(err, errors.New("bound file reread mismatch"))
	}
	if err = verify(got); err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("bound regular file identity unavailable")
	}
	sum := sha256.Sum256(got)
	identity, err := stableFileIdentity(info)
	if err != nil {
		return nil, err
	}
	binding := fmt.Sprintf("v1:%s:sha256:%s:%d", identity, hex.EncodeToString(sum[:]), len(got))
	if testHookBoundFileAfterVerify != nil {
		testHookBoundFileAfterVerify(t.parent, t.name)
	}
	if testHookBoundFileBeforeActivate != nil {
		testHookBoundFileBeforeActivate(t.parent, t.name)
	}
	named, statErr := t.parent.Lstat(t.name)
	if statErr != nil || !os.SameFile(info, named) {
		return nil, errors.New("reserved bundle identity changed")
	}
	if err = t.parent.Symlink(binding, activation); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, os.ErrExist
		}
		return nil, err
	}
	committed = true
	durability := "NOT_CHECKED_POST_COMMIT"
	if d, openErr := t.parent.Open("."); openErr == nil {
		if d.Sync() == nil {
			durability = "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE"
		}
		_ = d.Close()
	}
	return &BoundFileReceipt{FinalSelector: selector, Digest: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: uint64(len(got)), Mechanism: BoundFileMechanism, NamespaceAtomic: true, CrashDurability: durability}, nil
}

func removeSameFile(parent *os.Root, name string, f *os.File) {
	want, err := f.Stat()
	if err != nil {
		return
	}
	got, err := parent.Lstat(name)
	if err == nil && os.SameFile(want, got) {
		_ = boundFileRemove(parent, name)
	}
}

func ReadBoundFile(root *Root, selector string, limit int64) ([]byte, error) {
	if limit < 1 {
		return nil, errors.New("positive bound file limit required")
	}
	t, err := existingTarget(root, selector)
	if err != nil {
		return nil, err
	}
	defer t.close()
	activation := t.name + ".active"
	binding, err := t.parent.Readlink(activation)
	if err != nil {
		return nil, errors.New("bound file is not active")
	}
	before, err := t.parent.Lstat(t.name)
	if err != nil || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, errors.New("invalid active bound file")
	}
	f, err := t.parent.Open(t.name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, errors.New("active bound file identity changed")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, errors.New("active bound file limit exceeded")
	}
	sum := sha256.Sum256(raw)
	identity, err := stableFileIdentity(after)
	if err != nil {
		return nil, err
	}
	want := fmt.Sprintf("v1:%s:sha256:%s:%d", identity, hex.EncodeToString(sum[:]), len(raw))
	if binding != want {
		return nil, errors.New("active bound file binding mismatch")
	}
	return raw, nil
}
