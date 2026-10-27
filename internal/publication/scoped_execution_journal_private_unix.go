//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// Per-invocation seams, not process-global hooks. Tests may inject original
// causes without changing legacy publication helpers or another journal.
type scopedExecutionOpsPrivate struct {
	read      contentReadOps
	reread    func(*os.File, int64) ([]byte, error)
	syncFile  func(*os.File) error
	closeFile func(*os.File) error
	syncDir   func(int) error
	closeFD   func(int) error
	install   func(int, int, string) error
}

func defaultScopedExecutionOpsPrivate() scopedExecutionOpsPrivate {
	return scopedExecutionOpsPrivate{
		read:      contentReadOps{lstat: statObservedContent, open: openObservedContent},
		reread:    func(f *os.File, limit int64) ([]byte, error) { return io.ReadAll(io.LimitReader(f, limit)) },
		syncFile:  func(f *os.File) error { return f.Sync() },
		closeFile: func(f *os.File) error { return f.Close() },
		syncDir:   unix.Fsync, closeFD: unix.Close, install: installScopedExecutionPrivate,
	}
}
func scopedExecutionSupportedPrivate() bool { return true }

type scopedExecutionCloseObserverPrivate struct {
	observedFile
	failed *bool
}

func (f scopedExecutionCloseObserverPrivate) Close() error {
	err := f.observedFile.Close()
	if err != nil {
		*f.failed = true
	}
	return err
}

func loadScopedExecutionPrivate(ctx context.Context, j *ScopedExecutionJournalPrivate, ops scopedExecutionOpsPrivate) ([]byte, error) {
	if err := j.validate(ctx); err != nil {
		return nil, err
	}
	readOps := ops.read
	firstAbsent := false
	stats := 0
	readOps.lstat = func(r *Root, name string) (os.FileInfo, error) {
		stats++
		info, err := ops.read.lstat(r, name)
		if stats == 1 {
			firstAbsent = scopedExecutionOnlyAbsentPrivate(err)
		}
		return info, err
	}
	raw, err := j.root.observed.root.readObservedFile(ctx, j.selector, scopedExecutionHeadLimitPrivate, readOps, false)
	err = errors.Join(err, ctx.Err())
	if firstAbsent && scopedExecutionOnlyAbsentPrivate(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Preserve present zero bytes even if the reader implementation returns nil.
	if raw == nil {
		raw = make([]byte, 0)
	}
	return raw, nil
}

func compareScopedExecutionPrivate(ctx context.Context, j *ScopedExecutionJournalPrivate, raw []byte, ops scopedExecutionOpsPrivate) (receipt *CompareAndReplaceReceipt, err error) {
	if len(raw) == 0 || len(raw) > scopedExecutionHeadLimitPrivate {
		return nil, ErrContentObservation
	}
	if err = j.validate(ctx); err != nil {
		return nil, err
	}
	expected := bytes.Clone(raw)
	root := j.root.observed.root
	fd, err := unix.Dup(int(root.file.Fd()))
	if err != nil {
		return nil, err
	}
	var source *os.File
	// All acquired resources close once, including on earlier failures. A stale
	// result is emitted only after clean cleanup of an affirmative EEXIST.
	present := false
	finalCloseFailed := false
	defer func() {
		var closeErr error
		if source != nil {
			closeErr = errors.Join(closeErr, ops.closeFile(source))
		}
		closeErr = errors.Join(closeErr, ops.closeFD(fd))
		err = errors.Join(err, closeErr, ctx.Err())
		if receipt != nil {
			receipt.CloseStatus = postcommitCloseStatus(true, closeErr)
			if finalCloseFailed {
				receipt.CloseStatus = CloseFailed
			}
		} else if present {
			if err == nil {
				receipt = &CompareAndReplaceReceipt{}
				err = ErrStalePredecessor
			} else {
				err = errors.Join(unix.EEXIST, err)
			}
		}
	}()
	source, err = newScopedExecutionSourcePrivate(fd)
	if err != nil {
		return nil, err
	}
	if err = source.Chmod(0600); err != nil {
		return nil, err
	}
	if err = writeAll(source, expected); err != nil {
		return nil, err
	}
	if err = ops.syncFile(source); err != nil {
		return nil, err
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	got, readErr := ops.reread(source, int64(len(expected))+1)
	if readErr != nil {
		return nil, readErr
	}
	if !bytes.Equal(got, expected) {
		return nil, ErrContentObservation
	}
	if err = j.validate(ctx); err != nil {
		return nil, err
	}
	// Atomic compare-absent linearization point: clone/link of the exact verified
	// source FD. The kernel refuses an existing destination; no check-then-rename,
	// pathname source, legacy CAS lock, retry, overwrite or success recovery.
	if err = ops.install(int(source.Fd()), fd, j.selector); err != nil {
		if err == unix.EEXIST {
			present = true
			return nil, nil
		}
		return nil, err
	}
	receipt = &CompareAndReplaceReceipt{Committed: true, Outcome: CASOutcomeCommitted, BoundFileReceipt: &BoundFileReceipt{
		FinalSelector: j.selector, Digest: DigestForTest(expected), ByteLength: uint64(len(expected)),
		Mechanism: BoundFileMechanism, NamespaceAtomic: true, CloseStatus: CloseNotAttempted,
	}}
	syncErr := ops.syncDir(fd)
	receipt.DirectorySyncStatus = postcommitDirectorySyncStatus(true, syncErr)
	receipt.CrashDurability = receipt.DirectorySyncStatus
	// This is verification only, never retry/recovery. Preserve both sync and read
	// causes; even exact bytes cannot promote either error into success.
	verifyOps := ops
	verifyOps.read.open = func(r *Root, name string) (observedFile, error) {
		f, err := ops.read.open(r, name)
		if f != nil {
			return scopedExecutionCloseObserverPrivate{f, &finalCloseFailed}, err
		}
		return nil, err
	}
	observed, verifyErr := loadScopedExecutionPrivate(ctx, j, verifyOps)
	if verifyErr == nil && !bytes.Equal(observed, expected) {
		verifyErr = ErrContentObservation
	}
	receipt.VerificationStatus = "VERIFIED"
	if verifyErr != nil {
		receipt.VerificationStatus = CASOutcomeCommittedVerificationFailed
		receipt.Outcome = CASOutcomeCommittedVerificationFailed
	}
	return receipt, errors.Join(syncErr, verifyErr)
}
