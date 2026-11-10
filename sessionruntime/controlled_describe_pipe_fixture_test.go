//go:build lsp_trace_controlled_describe_fixture

package sessionruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
)

func controlledDescribeFixtureTest(t *testing.T) *ControlledDescribePipeFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := NewControlledDescribePipeFixture(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Bodies that explicitly inspect a failed Close have already released
		// fixture-owned resources; never turn that inspected failure into success.
		if f.closed {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := f.Close(ctx); err != nil {
			t.Errorf("ASSERT_CONTROLLED_PIPE_CLOSE: %v census=%+v", err, f.manager.Census())
		}
	})
	return f
}

type controlledDescribeUnmarked struct {
	child  Child
	starts int
}

func (s *controlledDescribeUnmarked) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	s.starts++
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func TestControlledDescribePipeRefusals(t *testing.T) {
	for _, fault := range []string{"unmarked", "startup-cancel", "incomplete", "failed-consume", "cancel-after-consume", "cleanup-cancel"} {
		t.Run(fault, func(t *testing.T) {
			f := controlledDescribeFixtureTest(t)
			m := f.Manager()
			q := f.StartRequest(time.Now().Add(5 * time.Second))
			if fault == "unmarked" {
				starter := &controlledDescribeUnmarked{child: f.child}
				other, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 16, MaxChildren: 16, MaxCancels: 8, MaxTombstones: 32, MaxObservations: 256, MaxOperations: 8}, Starter: starter})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					if err := other.Shutdown(ctx); err != nil {
						t.Error(err)
					}
				})
				_, err = other.PrivateProviderWindowEnvelope(q, PrivateProviderWindowPlan{References: true})
				if !errors.Is(err, ErrPrivateProviderWindowUnavailable) || starter.starts != 0 {
					t.Fatal("ASSERT_PIPE_UNMARKED_REFUSED", err)
				}
				t.Log("ASSERT_PIPE_UNMARKED_REFUSED: PASS")
				return
			}
			if fault == "startup-cancel" {
				canceled, cancel := context.WithCancel(context.Background())
				cancel()
				if got := m.Start(canceled, q); got.Failure == "" {
					t.Fatal("ASSERT_PIPE_START_CANCEL_REFUSED")
				}
				t.Log("ASSERT_PIPE_START_CANCEL_REFUSED: PASS")
				return
			}
			if fault == "cleanup-cancel" {
				canceled, cancel := context.WithCancel(context.Background())
				cancel()
				if err := f.Close(canceled); !errors.Is(err, context.Canceled) {
					t.Fatal("ASSERT_PIPE_CLEANUP_NO_FALSE_SUCCESS", err)
				}
				select {
				case <-f.child.done:
				case <-time.After(time.Second):
					t.Fatal("cleanup did not join server")
				}
				t.Log("ASSERT_PIPE_CLEANUP_NO_FALSE_SUCCESS: PASS")
				return
			}
			envelope, err := m.PrivateProviderWindowEnvelope(q, PrivateProviderWindowPlan{References: true})
			if err != nil {
				t.Fatal(err)
			}
			ctx, err := m.PrivateProviderWindowContext(context.Background(), envelope, q.Deadline)
			if err != nil {
				t.Fatal(err)
			}
			started := m.Start(ctx, q)
			if started.Failure != "" {
				t.Fatal(started.Failure)
			}
			pending := m.BeginReadiness(ctx, started.SessionID, started.Generation, q.Deadline)
			ready, ok := m.WaitReadiness(ctx, pending.ID)
			if !ok || ready.State != ReadinessReady {
				t.Fatal("readiness", ready)
			}
			if fault == "incomplete" {
				done, err := m.FinishPrivateProviderWindow(ctx)
				if err == nil || done.Completed {
					t.Fatal("ASSERT_PIPE_INCOMPLETE_NO_COMPLETION")
				}
				cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				err = f.Close(cleanup)
				cancel()
				if err == nil || m.Census().Workers != 1 || m.Census().Sessions != 1 {
					t.Fatal("ASSERT_PIPE_UNJOINED_NOT_SUCCESS", err, m.Census())
				}
				select {
				case <-f.child.done:
				default:
					t.Fatal("pipe not joined")
				}
				if _, err := os.Stat(filepath.Join(q.Profile.Workspace().String(), "query.go")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("owned literal leaked", err)
				}
				t.Log("ASSERT_PIPE_INCOMPLETE_NO_COMPLETION: PASS; ASSERT_PIPE_UNJOINED_NOT_SUCCESS: PASS; sticky native owner retained, actual pipe joined")
				return
			}
			lease := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: q.Profile.Workspace().String()}
			err = m.WithReadySessionLease(ctx, lease, func(leased context.Context) error {
				h, err := m.CapturePrivateGroupingOwned(leased, PrivateGroupingOwnedRequest{Session: lease, URI: f.URI(), OccurrenceID: "fixed-query", Revision: "fixed-revision", Privacy: "private", Line: 1, Character: 5, References: true, Deadline: q.Deadline})
				if err != nil {
					return err
				}
				if fault == "failed-consume" {
					refusal := errors.New("controlled consumer refusal")
					if err = m.ConsumePrivateGroupingOwned(leased, h, func(PrivateGroupingOwnedView) error { return refusal }); !errors.Is(err, refusal) {
						t.Fatal("consumer not exercised", err)
					}
					if evidence, err := m.TakePrivateGroupingSourceEvidence(leased, h); err == nil || evidence.SourceByteLength() != 0 {
						t.Fatal("ASSERT_PIPE_FAILED_CONSUME_NO_TAKE")
					}
					t.Log("ASSERT_PIPE_FAILED_CONSUME_NO_TAKE: PASS")
				} else {
					if err = m.ConsumePrivateGroupingOwned(leased, h, func(PrivateGroupingOwnedView) error { return nil }); err != nil {
						return err
					}
					canceled, cancel := context.WithCancel(leased)
					cancel()
					if evidence, err := m.TakePrivateGroupingSourceEvidence(canceled, h); err == nil || evidence.SourceByteLength() != 0 {
						t.Fatal("ASSERT_PIPE_CANCELED_NO_TAKE")
					}
					t.Log("ASSERT_PIPE_CANCELED_NO_TAKE: PASS")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			done, err := m.FinishPrivateProviderWindow(ctx)
			if err != nil || !done.Completed || m.Census().Workers != 0 {
				t.Fatal("negative cleanup", done, err, m.Census())
			}
		})
	}
}

func TestControlledDescribePipeConstructorRefusals(t *testing.T) {
	for _, fault := range []string{"nonempty", "nonprivate", "symlink"} {
		t.Run(fault, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "nonempty":
				if err = os.WriteFile(filepath.Join(root, "caller"), []byte("caller-owned"), 0600); err != nil {
					t.Fatal(err)
				}
			case "nonprivate":
				if err = os.Chmod(root, 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				link := root + "-link"
				if err = os.Symlink(root, link); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Remove(link); err != nil {
						t.Error(err)
					}
				})
				root = link
			}
			f, err := NewControlledDescribePipeFixture(root)
			if err == nil || f != nil {
				t.Fatal("ASSERT_PIPE_CONSTRUCTOR_REFUSED", fault, err)
			}
			if fault == "nonempty" {
				raw, err := os.ReadFile(filepath.Join(root, "caller"))
				if err != nil || string(raw) != "caller-owned" {
					t.Fatal("caller file touched", err)
				}
			}
			t.Log("ASSERT_PIPE_CONSTRUCTOR_REFUSED: PASS", fault)
		})
	}
}

func TestControlledDescribePipeAccepted(t *testing.T) {
	f := controlledDescribeFixtureTest(t)
	q := f.StartRequest(time.Now().Add(5 * time.Second))
	env, err := f.Manager().PrivateProviderWindowEnvelope(q, PrivateProviderWindowPlan{References: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := f.Manager().PrivateProviderWindowContext(context.Background(), env, q.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	started := f.Manager().Start(ctx, q)
	if started.Failure != "" {
		t.Fatal(started.Failure)
	}
	pending := f.Manager().BeginReadiness(ctx, started.SessionID, started.Generation, q.Deadline)
	ready, ok := f.Manager().WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != ReadinessReady {
		t.Fatal("readiness", ready)
	}
	lease := SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: q.Profile.Workspace().String()}
	err = f.Manager().WithReadySessionLease(ctx, lease, func(leased context.Context) error {
		h, e := f.Manager().CapturePrivateGroupingOwned(leased, PrivateGroupingOwnedRequest{Session: lease, URI: f.URI(), OccurrenceID: "fixed-query", Revision: "fixed-revision", Privacy: "private", Line: 1, Character: 5, References: true, Deadline: q.Deadline})
		if e != nil {
			return e
		}
		if _, e = f.Manager().TakePrivateGroupingSourceEvidence(leased, h); e == nil {
			t.Fatal("ASSERT_PIPE_PRECONSUME_REFUSED")
		}
		if e = f.Manager().ConsumePrivateGroupingOwned(leased, h, func(v PrivateGroupingOwnedView) error {
			if string(v.Source) != controlledDescribeDocument {
				return errors.New("source mismatch")
			}
			return nil
		}); e != nil {
			return e
		}
		evidence, e := f.Manager().TakePrivateGroupingSourceEvidence(leased, h)
		if e != nil || evidence.SourceByteLength() != len(controlledDescribeDocument) {
			t.Fatal("ASSERT_PIPE_ACTUAL_TAKE", e)
		}
		if _, e = f.Manager().TakePrivateGroupingSourceEvidence(leased, h); e == nil {
			t.Fatal("ASSERT_PIPE_DUPLICATE_TAKE_REFUSED")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := f.Manager().FinishPrivateProviderWindow(ctx)
	if err != nil || !completed.Completed || f.Manager().Census().Workers != 0 {
		t.Fatal("ASSERT_PIPE_WINDOW_COMPLETE", err, completed, f.Manager().Census())
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = f.Close(closeCtx); err != nil {
		t.Fatal("ASSERT_CONTROLLED_PIPE_CLOSE", err)
	}
	select {
	case <-f.child.done:
	default:
		t.Fatal("server not joined")
	}
	if _, err = os.Stat(filepath.Join(q.Profile.Workspace().String(), "query.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("literal not removed", err)
	}
	if err = os.WriteFile(filepath.Join(q.Profile.Workspace().String(), "root-live"), []byte("usable"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_PIPE_ACTUAL_TAKE: PASS; ASSERT_PIPE_DUPLICATE_TAKE_REFUSED: PASS; ASSERT_PIPE_WINDOW_COMPLETE: PASS; ASSERT_CONTROLLED_PIPE_CLOSE: PASS; ASSERT_PIPE_ROOT_USABLE: PASS")
}
