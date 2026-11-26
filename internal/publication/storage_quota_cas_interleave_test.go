//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func assertCASRepairLocksReleased(t *testing.T, r *Root) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	q, e := ReserveStorage(ctx, r, 0)
	if e != nil {
		t.Error("STORAGE_LOCK_LEAK", e)
	} else if e = q.Complete(true); e != nil {
		t.Error(e)
	}
	// Probe independently even if storage acquisition/classification failed.
	casCtx, casCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer casCancel()
	unlock, e := r.casLock(casCtx, "head")
	if e != nil {
		t.Error("CAS_LOCK_LEAK", e)
	} else {
		unlock()
		t.Log("CAS_AND_STORAGE_RELEASE_PROBES_COMPLETED")
	}
}

// Preserve the review's separate product-gap observation; safety repair must
// not silently reset/reclaim conservative retirement or change status reads.
func TestStorageQuotaCASNoReclamation(t *testing.T) {
	r := quotaRoot(t, 128<<10)
	raw := bytes.Repeat([]byte("x"), 4096)
	if _, e := PublishBoundFile(r, "head", raw, func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	n := 0
	for ; n < 32; n++ {
		_, e := CompareAndReplaceBoundFile(context.Background(), r, "head", BoundFilePredecessor{Selector: "head", Digest: DigestForTest(raw), ByteLength: uint64(len(raw))}, raw, func([]byte) error { return nil })
		if errors.Is(e, ErrStorageCapacity) {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	a, b := quotaStatus(t, r), quotaStatus(t, r)
	if *a != *b || n != 12 || a.FileBytes != 8192 || a.RetirementReservedBytes != 53248 || a.UncertainReservedBytes != 0 || a.AvailableBytes != 4096 {
		t.Fatal("NO_RECLAMATION_CONTRACT_CHANGED", n, a, b)
	}
	t.Logf("NO_RECLAMATION_PRESERVED successful_replacements=%d retired=%d named=%d", n, a.RetirementReservedBytes, a.FileBytes)
}

func TestStorageQuotaCASFinalPredecessorCases(t *testing.T) {
	for _, kind := range []string{"changed-same-length", "appeared", "disappeared", "same-content-new-inode"} {
		t.Run(kind, func(t *testing.T) {
			r := quotaRoot(t, 2<<20)
			other, e := OpenRoot(r.Path())
			if e != nil {
				t.Fatal(e)
			}
			defer other.Close()
			old := bytes.Repeat([]byte("a"), 4096)
			next := []byte("next")
			pred := BoundFilePredecessor{Selector: "head", Digest: DigestForTest(old), ByteLength: uint64(len(old))}
			var readers []*os.File
			defer func() {
				for _, f := range readers {
					f.Close()
				}
			}()
			if kind == "appeared" {
				pred = BoundFilePredecessor{Absent: true}
			} else {
				if _, e = PublishBoundFile(r, "head", old, func([]byte) error { return nil }); e != nil {
					t.Fatal(e)
				}
				f, e := os.Open(filepath.Join(r.Path(), "head"))
				if e != nil {
					t.Fatal(e)
				}
				readers = append(readers, f)
			}
			observed := 0
			callbacks := 0
			observe := func(name string, limit int64) ([]byte, error) {
				observed++
				got, readErr := r.ReadSelectorWithAbsence(name, limit)
				if kind == "appeared" {
					if !os.IsNotExist(readErr) {
						t.Fatal("real absence required", readErr)
					}
				} else {
					if readErr != nil {
						t.Fatal(readErr)
					}
					if _, e := other.RemovePrivateFile(name); e != nil {
						t.Fatal(e)
					}
				}
				if kind != "disappeared" {
					value := old
					if kind == "changed-same-length" || kind == "appeared" {
						value = bytes.Repeat([]byte("b"), 4096)
					}
					if _, e := PublishBoundFile(other, name, value, func([]byte) error { return nil }); e != nil {
						t.Fatal(e)
					}
					f, e := os.Open(filepath.Join(r.Path(), name))
					if e != nil {
						t.Fatal(e)
					}
					readers = append(readers, f)
					if kind == "same-content-new-inode" {
						a, e := readers[0].Stat()
						if e != nil {
							t.Fatal(e)
						}
						b, e := f.Stat()
						if e != nil || os.SameFile(a, b) {
							t.Fatal("distinct inode required", e)
						}
					}
				}
				return got, readErr
			}
			receipt, e := compareAndReplaceBoundFile(context.Background(), r, "head", pred, next, func([]byte) error { callbacks++; return nil }, observe)
			if observed != 1 {
				t.Fatal("initial observation replayed", observed)
			}
			if kind == "same-content-new-inode" {
				if e != nil || receipt == nil || !receipt.Committed || callbacks != 3 {
					t.Fatal("CONTENT_CAS_COMPATIBILITY", receipt, e, callbacks)
				}
				v := quotaStatus(t, r)
				if v.RetirementReservedBytes != 8192 {
					t.Fatal("EQUAL_CONTENT_INODE_RETIREMENT", v)
				}
				for _, f := range readers {
					got := make([]byte, len(old))
					if _, e = f.ReadAt(got, 0); e != nil || !bytes.Equal(got, old) {
						t.Fatal("old inode unreadable", e)
					}
				}
				if v.FileBytes+8192 > v.Limit {
					t.Fatal("live content exceeded", v)
				}
			} else {
				if e != ErrStalePredecessor || receipt == nil || receipt.Committed || receipt.Outcome != CASOutcomeNotCommitted || callbacks != 2 {
					t.Fatal("EXACT_FINAL_STALE", receipt, e, callbacks)
				}
				current, ce := r.ReadSelectorWithAbsence("head", 4096)
				if kind == "disappeared" {
					if !os.IsNotExist(ce) {
						t.Fatal("disappearance replaced", ce)
					}
				} else if ce != nil || !bytes.Equal(current, bytes.Repeat([]byte("b"), 4096)) {
					t.Fatal("interposed content overwritten", ce)
				}
			}
			assertCASRepairLocksReleased(t, r)
		})
	}
}

func TestStorageQuotaCASFinalLockCancellationAndCleanup(t *testing.T) {
	for _, kind := range []string{"success", "final-callback-failure", "cancel-before-final-lock"} {
		t.Run(kind, func(t *testing.T) {
			r := quotaRoot(t, 2<<20)
			other, e := OpenRoot(r.Path())
			if e != nil {
				t.Fatal(e)
			}
			defer other.Close()
			old := []byte("old")
			next := []byte("new")
			if _, e = PublishBoundFile(r, "head", old, func([]byte) error { return nil }); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("controlled final callback failure")
			calls := 0
			var concurrent, pendingCancelled chan error
			writerCtx, writerCancel := context.WithCancel(context.Background())
			defer writerCancel()
			verify := func(got []byte) error {
				calls++
				if !bytes.Equal(got, next) {
					t.Error("callback bytes")
					return errors.New("callback bytes")
				}
				if kind == "cancel-before-final-lock" && calls == 1 {
					cancel()
				}
				if calls == 3 {
					// Observe an actual failed flock before explicit cancellation.
					// No elapsed-time assumption determines the expected error.
					base, stop := context.WithCancel(context.Background())
					wait := &quotaLockObservedContext{Context: base, blocked: make(chan struct{})}
					cancelled := make(chan error, 1)
					go func() {
						q, e := ReserveStorage(wait, other, 0)
						if q != nil {
							e = errors.Join(e, q.Complete(true))
						}
						cancelled <- e
					}()
					awaitQuotaBlocked(t, wait.blocked)
					stop()
					ce, settled := awaitQuotaResult(t, cancelled)
					if !settled {
						pendingCancelled = cancelled
					}
					if settled && (!errors.Is(ce, context.Canceled) || errors.Is(ce, context.DeadlineExceeded)) {
						t.Error("FINAL_LOCK_CANCEL_CLASSIFICATION", ce)
					}
					// This second real reservation queues the writer while CAS holds
					// the final flock. Always exercise it, even if classification failed.
					queued := &quotaLockObservedContext{Context: writerCtx, blocked: make(chan struct{})}
					concurrent = make(chan error, 1)
					go func() {
						q, e := ReserveStorage(queued, other, 0)
						if q != nil {
							e = errors.Join(e, q.Complete(true))
						}
						if e == nil {
							_, e = other.RemovePrivateFile("head")
						}
						if e == nil {
							_, e = PublishBoundFile(other, "head", []byte("after"), func([]byte) error { return nil })
						}
						concurrent <- e
					}()
					awaitQuotaBlocked(t, queued.blocked)
					if kind == "final-callback-failure" {
						return sentinel
					}
				}
				return nil
			}
			receipt, e := CompareAndReplaceBoundFile(ctx, r, "head", BoundFilePredecessor{Selector: "head", Digest: DigestForTest(old), ByteLength: uint64(len(old))}, next, verify)
			// Settle queued work and probe both locks before outcome assertions.
			if pendingCancelled != nil {
				awaitQuotaResult(t, pendingCancelled)
			}
			if concurrent != nil {
				we, finished := awaitQuotaResult(t, concurrent)
				writerCancel()
				if !finished {
					we, finished = awaitQuotaResult(t, concurrent)
				}
				if finished && we != nil {
					t.Error("COOPERATING_WRITER_LOCK_ORDER", we)
				}
				if finished && we == nil {
					got, re := r.ReadSelector("head", 5)
					if re != nil || string(got) != "after" {
						t.Error("QUEUED_WRITER_CONTENT", re)
					}
					t.Log("QUEUED_WRITER_COMPLETED_AFTER_CAS_RELEASE")
				}
			}
			assertCASRepairLocksReleased(t, r)
			if kind == "cancel-before-final-lock" {
				if !errors.Is(e, context.Canceled) || receipt == nil || receipt.Committed || calls != 2 {
					t.Fatal("CANCEL_COMMIT_OR_CALLBACK", receipt, e, calls)
				}
				got, e := r.ReadSelector("head", 3)
				if e != nil || !bytes.Equal(got, old) {
					t.Fatal("cancel changed predecessor", e)
				}
			} else {
				if receipt == nil || !receipt.Committed || calls != 3 {
					t.Fatal("callback contract", receipt, e, calls)
				}
				if kind == "success" && e != nil {
					t.Fatal(e)
				}
				if kind == "final-callback-failure" && (!errors.Is(e, sentinel) || receipt.Outcome != CASOutcomeCommittedVerificationFailed) {
					t.Fatal("postcommit failure erased", receipt, e)
				}
			}
		})
	}
}

// Derived from the independent review's real-API counterexample. The seam only
// schedules interposition AFTER a real bounded read and returns those exact
// bytes. It cannot fabricate the predecessor used by final revalidation.
func TestStorageQuotaCASMixedWriterLiveReaders(t *testing.T) {
	r := quotaRoot(t, 128<<10)
	old := []byte("a")
	large := bytes.Repeat([]byte("b"), 30000)
	other, e := OpenRoot(r.Path())
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	verifyCalls := 0
	verify := func([]byte) error { verifyCalls++; return nil }
	if _, e = PublishBoundFile(r, "head", old, func([]byte) error { return nil }); e != nil {
		t.Fatal(e)
	}
	var readers []*os.File
	defer func() {
		for _, f := range readers {
			f.Close()
		}
	}()
	observations := 0
	readExisting := func(name string, limit int64) ([]byte, error) {
		observations++
		got, e := r.ReadSelectorWithAbsence(name, limit)
		if e != nil {
			return got, e
		}
		if _, e = other.RemovePrivateFile(name); e != nil {
			t.Fatal(e)
		}
		if _, e = PublishBoundFile(other, name, large, func([]byte) error { return nil }); e != nil {
			t.Fatal(e)
		}
		reader, e := os.Open(filepath.Join(r.Path(), name))
		if e != nil {
			t.Fatal(e)
		}
		readers = append(readers, reader)
		return got, nil
	}
	for i := 0; i < 4; i++ {
		next := []byte{byte('c' + i)}
		receipt, e := compareAndReplaceBoundFile(context.Background(), r, "head", BoundFilePredecessor{Selector: "head", Digest: DigestForTest(old), ByteLength: 1}, next, verify, readExisting)
		if e != nil {
			if e != ErrStalePredecessor || receipt == nil || receipt.Committed || receipt.Outcome != CASOutcomeNotCommitted {
				t.Fatal("FINAL_PREDECESSOR_NOT_EXACT_STALE", receipt, e)
			}
			if i != 0 || observations != 1 || verifyCalls != 2 {
				t.Fatal("SEAM_OR_CALLBACK_REPLAY", i, observations, verifyCalls)
			}
			current, e := r.ReadSelector("head", int64(len(large)))
			if e != nil || !bytes.Equal(current, large) {
				t.Fatal("INTERPOSED_CURRENT_CHANGED", e)
			}
			got := make([]byte, len(large))
			if _, e = readers[0].ReadAt(got, 0); e != nil || !bytes.Equal(got, large) {
				t.Fatal("live reader changed", e)
			}
			currentInfo, e := os.Stat(filepath.Join(r.Path(), "head"))
			if e != nil {
				t.Fatal(e)
			}
			readerInfo, e := readers[0].Stat()
			if e != nil || !os.SameFile(currentInfo, readerInfo) {
				t.Fatal("interposed inode retired despite stale refusal", e)
			}
			v := quotaStatus(t, r)
			if v.RetirementReservedBytes != 2 || v.UncertainReservedBytes != 0 || v.FileBytes != 4096+30000+1 {
				t.Fatal("STALE_REFUSAL_REFUNDED_OR_LOST_TEMP", v)
			}
			t.Logf("MIXED_WRITER_STALE_BEFORE_RENAME named=%d live_unlinked=0 retirement=%d temporary_retained=1 exact_error_identity=true", v.FileBytes, v.RetirementReservedBytes)
			return
		}
		if receipt == nil || !receipt.Committed {
			t.Fatal("missing receipt")
		}
		old = next
	}
	if _, e = PublishBoundFile(r, "additional", bytes.Repeat([]byte("d"), 8192), func([]byte) error { return nil }); e != nil {
		t.Fatal("unexpected capacity escape instead of stale refusal", e)
	}
	unlinked := uint64(0)
	for _, reader := range readers {
		got := make([]byte, len(large))
		if _, e = reader.ReadAt(got, 0); e != nil || !bytes.Equal(got, large) {
			t.Fatal("reader lost", e)
		}
		unlinked += uint64(len(got))
	}
	v := quotaStatus(t, r)
	t.Logf("INTERLEAVE live_unlinked_readers=%d retired=%d observed=%d actual_live_content=%d limit=%d available=%d", unlinked, v.RetirementReservedBytes, v.FileBytes, v.FileBytes+unlinked, v.Limit, v.AvailableBytes)
	if v.FileBytes+unlinked > v.Limit {
		t.Fatalf("COOPERATING_CONTENT_QUOTA_EXCEEDED: live=%d limit=%d", v.FileBytes+unlinked, v.Limit)
	}
	t.Fatal("CHANGED_PREDECESSOR_WAS_NOT_REFUSED")
}
