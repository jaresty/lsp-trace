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
var testHookBoundFileDirectorySync func() error
var testHookBoundFileSourceClose func() error
var testHookBoundFileFinalClose func() error
var testHookBoundFileRootClose func() error
var testForceUnsupportedPrimitive bool

const (
	DirectorySyncNotAttemptedPostCommit = "NOT_ATTEMPTED_POST_COMMIT"
	DirectorySyncComplete               = "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE"
	DirectorySyncFailed                 = "COMMITTED_DIRECTORY_SYNC_FAILED"
	CloseNotAttempted                   = "NOT_ATTEMPTED"
	CloseComplete                       = "COMMITTED_CLOSE_COMPLETE"
	CloseFailed                         = "COMMITTED_CLOSE_FAILED"
)

func postcommitDirectorySyncStatus(attempted bool, err error) string {
	if !attempted {
		return DirectorySyncNotAttemptedPostCommit
	}
	if err != nil {
		return DirectorySyncFailed
	}
	return DirectorySyncComplete
}

func postcommitCloseStatus(attempted bool, err error) string {
	if !attempted {
		return CloseNotAttempted
	}
	if err != nil {
		return CloseFailed
	}
	return CloseComplete
}

type BoundFileReceipt struct {
	FinalSelector       string
	Digest              string
	ByteLength          uint64
	Mechanism           string
	NamespaceAtomic     bool
	CrashDurability     string
	DirectorySyncStatus string
	CloseStatus         string
	VerificationStatus  string
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
	committed := false
	closeTarget := func() error {
		if t == nil || !t.owned || t.parent == nil {
			return nil
		}
		err := t.parent.Close()
		t.parent = nil
		if testHookBoundFileRootClose != nil {
			err = errors.Join(err, testHookBoundFileRootClose())
		}
		return err
	}
	defer func() {
		if !committed {
			_ = closeTarget()
		}
	}()
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
	published, directoryStatus, closeStatus, err := publishExactFD(root, selector, raw, verify)
	if err != nil {
		return nil, errors.Join(err, closeTarget())
	}
	committed = published
	if err := closeTarget(); err != nil {
		closeStatus = CloseFailed
	}
	sum := sha256.Sum256(raw)
	status := "VERIFIED"
	if testHookBoundFileAfterPublish != nil {
		testHookBoundFileAfterPublish()
	}
	verifyErr, verifyCloseFailed := verifyPublishedExact(root, selector, raw, verify)
	if verifyErr != nil {
		status = "COMMITTED_VERIFICATION_FAILED"
	}
	if verifyCloseFailed {
		closeStatus = CloseFailed
	}
	return &BoundFileReceipt{
		FinalSelector: selector, Digest: "sha256:" + hex.EncodeToString(sum[:]),
		ByteLength: uint64(len(raw)), Mechanism: BoundFileMechanism,
		NamespaceAtomic: published, CrashDurability: directoryStatus,
		DirectorySyncStatus: directoryStatus, CloseStatus: closeStatus,
		VerificationStatus: status,
	}, nil
}

func verifyPublishedExact(root *Root, selector string, raw []byte, verify func([]byte) error) (verifyErr error, closeFailed bool) {
	t, err := existingTarget(root, selector)
	if err != nil {
		return err, false
	}
	defer func() {
		if !t.owned {
			return
		}
		if err := t.parent.Close(); err != nil {
			closeFailed = true
		}
		t.parent = nil
		if testHookBoundFileRootClose != nil && testHookBoundFileRootClose() != nil {
			closeFailed = true
		}
	}()
	before, err := t.parent.Lstat(t.name)
	if err != nil || !before.Mode().IsRegular() || before.Mode().Perm() != 0o600 || before.Size() != int64(len(raw)) || !validPublishedMetadata(root.info, before) {
		return errors.New("published bundle metadata mismatch"), false
	}
	f, err := t.parent.Open(t.name)
	if err != nil {
		return err, false
	}
	defer func() {
		if err := f.Close(); err != nil {
			closeFailed = true
		}
		if testHookBoundFileFinalClose != nil && testHookBoundFileFinalClose() != nil {
			closeFailed = true
		}
	}()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return errors.New("published bundle identity changed"), false
	}
	got, readErr := io.ReadAll(io.LimitReader(f, int64(len(raw))+1))
	if readErr != nil {
		return readErr, false
	}
	if len(got) != len(raw) || !equalBytes(got, raw) {
		return errors.New("published exact bytes mismatch"), false
	}
	return verify(got), false
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
