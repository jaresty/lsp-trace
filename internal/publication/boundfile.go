package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

const BoundFileMechanism = "exact_fd_atomic_no_replace"

// Verified bound-file readers expose narrow typed boundaries so terminal
// resolvers can classify custody and size failures without parsing messages.
// ReadBoundFile intentionally retains its existing behavior.
var (
	ErrBoundFilePolicy = errors.New("bound file policy violation")
	ErrBoundFileLimit  = errors.New("bound file limit exceeded")
)

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

type BoundFileTraceEvent struct {
	Stage  string
	Result string
	OK     bool
}

type BoundFileTrace func(BoundFileTraceEvent)

type BoundFileTraceSink interface {
	WriteBoundFileTrace(BoundFileTraceEvent) error
}

type BoundFileTraceSinkFailure struct {
	Stage  string
	Reason string
	cause  error
}

func (f *BoundFileTraceSinkFailure) Error() string { return "bound-file trace sink failed" }
func (f *BoundFileTraceSinkFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.cause
}

type BoundFilePublicationResult struct {
	Receipt      *BoundFileReceipt
	Failure      *Failure
	TraceFailure *BoundFileTraceSinkFailure
}

type boundFileTraceSinkFunc func(BoundFileTraceEvent) error

func (f boundFileTraceSinkFunc) WriteBoundFileTrace(event BoundFileTraceEvent) error { return f(event) }

func emitBoundFileTrace(trace BoundFileTrace, stage, result string, ok bool) {
	if trace != nil {
		trace(BoundFileTraceEvent{Stage: stage, Result: result, OK: ok})
	}
}

var testHookBoundFileAfterTempBeforeInstall func() error
var errInjectedBeforeInstall = errors.New("injected before install")

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
	return PublishBoundFileWithTrace(root, selector, raw, verify, nil)
}

func PublishBoundFileWithTrace(root *Root, selector string, raw []byte, verify func([]byte) error, trace BoundFileTrace) (receipt *BoundFileReceipt, returnErr error) {
	if root == nil || raw == nil || verify == nil {
		emitBoundFileTrace(trace, "OPEN_VALIDATE", "INVALID_REQUEST", false)
		return nil, errors.New("invalid bound file request")
	}
	if err := root.ValidatePrivate(); err != nil {
		emitBoundFileTrace(trace, "OPEN_VALIDATE", "PRIVATE_INVALID", false)
		return nil, err
	}
	emitBoundFileTrace(trace, "OPEN_VALIDATE", "PRIVATE_VALID", true)
	t, err := capabilityTarget(root, selector)
	if err != nil {
		emitBoundFileTrace(trace, "CANDIDATE", "CANONICALIZATION_FAILED", false)
		return nil, err
	}
	emitBoundFileTrace(trace, "CANDIDATE", "CANONICAL", true)
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
			cleanupErr := closeTarget()
			returnErr = errors.Join(returnErr, cleanupErr)
			emitBoundFileTrace(trace, "CLEANUP", map[bool]string{true: "COMPLETE", false: "FAILED"}[cleanupErr == nil], cleanupErr == nil)
		}
	}()
	lock := targetLock(t.key)
	lock.Lock()
	defer lock.Unlock()
	if _, err := t.parent.Lstat(t.name); err == nil {
		emitBoundFileTrace(trace, "TARGET", "EXISTS", false)
		return nil, os.ErrExist
	} else if !os.IsNotExist(err) {
		emitBoundFileTrace(trace, "TARGET", "STAT_FAILED", false)
		return nil, err
	}
	emitBoundFileTrace(trace, "TARGET", "ABSENT", true)
	if testForceUnsupportedPrimitive {
		emitBoundFileTrace(trace, "TEMP", "UNSUPPORTED", false)
		return nil, errExactFDUnsupported
	}
	published, directoryStatus, closeStatus, err := publishExactFD(root, selector, raw, verify, trace)
	if err != nil {
		return nil, err
	}
	committed = published
	if err := closeTarget(); err != nil {
		closeStatus = CloseFailed
		emitBoundFileTrace(trace, "CLEANUP", "FAILED", false)
	} else {
		emitBoundFileTrace(trace, "CLEANUP", "COMPLETE", true)
	}
	sum := sha256.Sum256(raw)
	status := "VERIFIED"
	if testHookBoundFileAfterPublish != nil {
		testHookBoundFileAfterPublish()
	}
	verifyErr, verifyCloseFailed := verifyPublishedExact(root, selector, raw, verify)
	if verifyErr != nil {
		status = "COMMITTED_VERIFICATION_FAILED"
		emitBoundFileTrace(trace, "TARGET_EQUAL", "NOT_EQUAL", false)
	} else {
		emitBoundFileTrace(trace, "TARGET_EQUAL", "EQUAL", true)
	}
	if verifyCloseFailed {
		closeStatus = CloseFailed
	}
	emitBoundFileTrace(trace, "RECEIPT", "CREATED", true)
	return &BoundFileReceipt{
		FinalSelector: selector, Digest: "sha256:" + hex.EncodeToString(sum[:]),
		ByteLength: uint64(len(raw)), Mechanism: BoundFileMechanism,
		NamespaceAtomic: published, CrashDurability: directoryStatus,
		DirectorySyncStatus: directoryStatus, CloseStatus: closeStatus,
		VerificationStatus: status,
	}, nil
}

func PublishBoundFileWithTraceSink(root *Root, selector string, raw []byte, verify func([]byte) error, sink BoundFileTraceSink) BoundFilePublicationResult {
	var traceFailure *BoundFileTraceSinkFailure
	var last, primaryFailure, cleanupFailure BoundFileTraceEvent
	trace := func(event BoundFileTraceEvent) {
		last = event
		if !event.OK {
			if event.Stage == "CLEANUP" {
				cleanupFailure = event
			} else if primaryFailure.Stage == "" {
				primaryFailure = event
			}
		}
		if sink != nil && traceFailure == nil {
			if err := sink.WriteBoundFileTrace(event); err != nil {
				traceFailure = &BoundFileTraceSinkFailure{Stage: "TRACE_WRITE", Reason: "SINK_FAILED", cause: err}
			}
		}
	}
	receipt, err := PublishBoundFileWithTrace(root, selector, raw, verify, trace)
	result := BoundFilePublicationResult{Receipt: receipt, TraceFailure: traceFailure}
	if err != nil {
		failureEvent := primaryFailure
		if failureEvent.Stage == "" {
			failureEvent = cleanupFailure
		}
		if failureEvent.Stage == "" {
			failureEvent = last
		}
		stage, code := failureEvent.Stage, failureEvent.Result
		if errors.Is(err, errInjectedBeforeInstall) {
			stage, code = "NO_REPLACE", "INJECTED_BEFORE_INSTALL"
		}
		result.Failure = &Failure{Stage: stage, Code: code, Cleanup: receipt == nil, AtomicRename: false, cause: err}
	}
	return result
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

// ReadVerifiedBoundFile reads an object previously published by PublishBoundFile
// while revalidating its private, single-link, owner-only custody metadata.
func ReadVerifiedBoundFile(root *Root, selector string, limit int64) ([]byte, error) {
	if root == nil || limit < 1 {
		return nil, errors.Join(ErrBoundFilePolicy, errors.New("private root and positive bound required"))
	}
	if err := root.ValidatePrivate(); err != nil {
		return nil, errors.Join(ErrBoundFilePolicy, err)
	}
	t, err := existingTarget(root, selector)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, errors.Join(ErrBoundFilePolicy, err)
	}
	defer t.close()
	before, err := t.parent.Lstat(t.name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm() != 0o600 || !validPublishedMetadata(root.info, before) {
		return nil, errors.Join(ErrBoundFilePolicy, errors.New("bound file custody metadata invalid"))
	}
	if before.Size() > limit {
		return nil, errors.Join(ErrBoundFileLimit, errors.New("bound file exceeds configured byte limit"))
	}
	f, err := t.parent.Open(t.name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.Join(ErrBoundFilePolicy, errors.New("bound file identity changed"))
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.Join(ErrBoundFileLimit, errors.New("bound file byte limit invalid"))
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(opened, after) || after.Size() != int64(len(raw)) || !validPublishedMetadata(root.info, after) {
		return nil, errors.Join(ErrBoundFilePolicy, errors.New("bound file identity or custody changed"))
	}
	return raw, nil
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
