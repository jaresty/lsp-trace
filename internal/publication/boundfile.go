package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

const BoundFileMechanism = "exact_fd_atomic_no_replace"

var testHookBoundFileAfterVerify func()
var testHookBoundFileBeforePublish func()
var testHookBoundFileAfterPublish func()
var testForceUnsupportedPrimitive bool

type BoundFileReceipt struct {
	FinalSelector      string
	Digest             string
	ByteLength         uint64
	Mechanism          string
	NamespaceAtomic    bool
	CrashDurability    string
	VerificationStatus string
}

// PublishBoundFile verifies one canonical source handle and asks the platform
// helper to publish that exact handle directly at selector, no-replace. Once
// the kernel operation succeeds the result is committed success; any late
// verification problem is represented in VerificationStatus, never as failure.
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
	if testForceUnsupportedPrimitive {
		return nil, errExactFDUnsupported
	}
	published, durability, err := publishExactFD(root, selector, raw, verify)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	status := "VERIFIED"
	if testHookBoundFileAfterPublish != nil {
		testHookBoundFileAfterPublish()
	}
	if verifyPublishedExact(root, selector, raw, verify) != nil {
		status = "COMMITTED_VERIFICATION_FAILED"
	}
	return &BoundFileReceipt{
		FinalSelector: selector, Digest: "sha256:" + hex.EncodeToString(sum[:]),
		ByteLength: uint64(len(raw)), Mechanism: BoundFileMechanism,
		NamespaceAtomic: published, CrashDurability: durability,
		VerificationStatus: status,
	}, nil
}

func verifyPublishedExact(root *Root, selector string, raw []byte, verify func([]byte) error) error {
	t, err := existingTarget(root, selector)
	if err != nil {
		return err
	}
	defer t.close()
	before, err := t.parent.Lstat(t.name)
	if err != nil || !before.Mode().IsRegular() || before.Mode().Perm() != 0o600 || before.Size() != int64(len(raw)) || !validPublishedMetadata(root.info, before) {
		return errors.New("published bundle metadata mismatch")
	}
	f, err := t.parent.Open(t.name)
	if err != nil {
		return err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return errors.New("published bundle identity changed")
	}
	got, readErr := io.ReadAll(io.LimitReader(f, int64(len(raw))+1))
	if readErr != nil {
		return readErr
	}
	if len(got) != len(raw) || !equalBytes(got, raw) {
		return errors.New("published exact bytes mismatch")
	}
	return verify(got)
}

func ReadBoundFile(root *Root, selector string, limit int64) ([]byte, error) {
	return root.ReadSelector(selector, limit)
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var d byte
	for i := range a {
		d |= a[i] ^ b[i]
	}
	return d == 0
}
