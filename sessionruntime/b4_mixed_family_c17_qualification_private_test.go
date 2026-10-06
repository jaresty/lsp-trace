package sessionruntime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

func fillPrivateB4C17Capability(t *testing.T, profile *privateB4EventAccountC17, count int) {
	t.Helper()
	if count == 0 {
		return
	}
	if failure, poison := profile.withCapabilityBatchAdmission(count, func() (session.Failure, bool) { return "", false }); failure != "" || poison {
		t.Fatalf("ASSERT_C17_MIXED_FILL failure=%s poison=%t count=%d", failure, poison, count)
	}
}

func TestPrivateB4C17MixedEachFamilyFinalSuccessAndRefusal(t *testing.T) {
	t.Run("acquisition", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			fill int
			want session.Failure
		}{
			{name: "final success", fill: privateB4EventLimit - 4},
			{name: "final refusal", fill: privateB4EventLimit - 3, want: session.ResourceExhausted},
		} {
			t.Run(tc.name, func(t *testing.T) {
				manager, req, lease := privateB4PreparedSource(t, []byte("package mixedacquisition\n"))
				profile, failure := newPrivateB4EventAccountC17()
				if failure != "" {
					t.Fatal(failure)
				}
				fillPrivateB4C17Capability(t, profile, tc.fill)
				before := profile.eventSnapshot()
				reservation := &privateB4Reservation{sourceLeases: []B4DefinitionSourceLease{lease}}
				manager.mu.Lock()
				got := manager.admitPrivateB4SourcesC17Locked(privateB4C17AcquisitionRequest(req), reservation, profile)
				manager.mu.Unlock()
				if got != tc.want {
					t.Fatalf("ASSERT_C17_MIXED_ACQUISITION_FINAL failure=%s want=%s", got, tc.want)
				}
				after := profile.eventSnapshot()
				if tc.want == "" {
					if after.Admitted != privateB4EventLimit || after.TerminalRequested || lease.state.state.Load() != privateB4SourceTransferred {
						t.Fatalf("ASSERT_C17_MIXED_ACQUISITION_8192 before=%+v after=%+v state=%d", before, after, lease.state.state.Load())
					}
				} else if after.Admitted != before.Admitted || !after.TerminalRequested || lease.state.state.Load() != privateB4SourceHeld {
					t.Fatalf("ASSERT_C17_MIXED_ACQUISITION_8193_ATOMIC before=%+v after=%+v state=%d", before, after, lease.state.state.Load())
				}
			})
		}
	})

	t.Run("capability", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			fill int
			want session.Failure
		}{
			{name: "final success", fill: privateB4EventLimit - 1},
			{name: "final refusal", fill: privateB4EventLimit, want: session.ResourceExhausted},
		} {
			t.Run(tc.name, func(t *testing.T) {
				profile, failure := newPrivateB4EventAccountC17()
				if failure != "" {
					t.Fatal(failure)
				}
				fillPrivateB4C17Capability(t, profile, tc.fill)
				before := profile.eventSnapshot()
				var responderCalls atomic.Int32
				got, poison := writePrivateB4CapabilityResponseC17(profile, "client/registerCapability", []byte(`{"registrations":[{"id":"final","method":"m"}]}`), func() (session.Failure, bool) {
					responderCalls.Add(1)
					return "", false
				})
				if got != tc.want || poison {
					t.Fatalf("ASSERT_C17_MIXED_CAPABILITY_FINAL failure=%s want=%s poison=%t", got, tc.want, poison)
				}
				after := profile.eventSnapshot()
				if tc.want == "" {
					if after.Admitted != privateB4EventLimit || responderCalls.Load() != 1 {
						t.Fatalf("ASSERT_C17_MIXED_CAPABILITY_8192 after=%+v responder=%d", after, responderCalls.Load())
					}
				} else if after.Admitted != before.Admitted || responderCalls.Load() != 0 || !after.TerminalRequested {
					t.Fatalf("ASSERT_C17_MIXED_CAPABILITY_8193_ATOMIC before=%+v after=%+v responder=%d", before, after, responderCalls.Load())
				}
			})
		}
	})

	t.Run("target", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			fill int
			want error
		}{
			{name: "final success", fill: privateB4EventLimit - 2},
			{name: "final refusal", fill: privateB4EventLimit - 1, want: errPrivateB4EventAdmission},
		} {
			t.Run(tc.name, func(t *testing.T) {
				profile, failure := newPrivateB4EventAccountC17()
				if failure != "" {
					t.Fatal(failure)
				}
				fillPrivateB4C17Capability(t, profile, tc.fill)
				before := profile.eventSnapshot()
				var appendCalls atomic.Int32
				err := (&PrivateB4DefinitionBorrow{c17: profile}).WithTargetAppendAdmission(42, func() error {
					appendCalls.Add(1)
					return nil
				})
				if !errors.Is(err, tc.want) {
					t.Fatalf("ASSERT_C17_MIXED_TARGET_FINAL err=%v want=%v", err, tc.want)
				}
				after := profile.eventSnapshot()
				if tc.want == nil {
					if after.Admitted != privateB4EventLimit || appendCalls.Load() != 1 {
						t.Fatalf("ASSERT_C17_MIXED_TARGET_8192 after=%+v append=%d", after, appendCalls.Load())
					}
				} else if after.Admitted != before.Admitted || appendCalls.Load() != 0 || !after.TerminalRequested {
					t.Fatalf("ASSERT_C17_MIXED_TARGET_8193_ATOMIC before=%+v after=%+v append=%d", before, after, appendCalls.Load())
				}
			})
		}
	})
}

func TestPrivateB4C17MixedInterleavingReplayAndCumulativeNonRollback(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	fillPrivateB4C17Capability(t, profile, 2)
	if err := profile.withTargetAppendAdmission(7, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	token, err := profile.beginAcquisitionAdmission(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := token.commit(); err != nil {
		t.Fatal(err)
	}
	want := []privateB4EventIdentityC17{
		{Family: "CAPABILITY", Kind: "CAPABILITY_ENTRY", Ordinal: 0},
		{Family: "CAPABILITY", Kind: "CAPABILITY_ENTRY", Ordinal: 1},
		{Family: "TARGET", Kind: "TARGET_BEGIN", Ordinal: 7},
		{Family: "TARGET", Kind: "TARGET_TERMINAL", Ordinal: 7},
	}
	want = append(want, privateB4C17WantAcquisition(1)...)
	got := privateB4C17CommittedIdentities(t, profile)
	if len(got) != len(want) {
		t.Fatalf("ASSERT_C17_MIXED_INTERLEAVED_COUNT got=%+v want=%+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ASSERT_C17_MIXED_INTERLEAVED_ORDER index=%d got=%+v want=%+v", i, got[i], want[i])
		}
	}
	beforeReplay := profile.eventSnapshot()
	called := false
	if err := profile.withTargetAppendAdmission(7, func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || called {
		t.Fatalf("ASSERT_C17_MIXED_TARGET_REPLAY err=%v called=%t", err, called)
	}
	if replay, err := profile.beginAcquisitionAdmission(1); !errors.Is(err, errPrivateB4EventAdmission) || replay != nil {
		t.Fatalf("ASSERT_C17_MIXED_ACQUISITION_REPLAY token=%v err=%v", replay, err)
	}
	if afterReplay := profile.eventSnapshot(); afterReplay != beforeReplay {
		t.Fatalf("ASSERT_C17_MIXED_REPLAY_NO_RECHARGE before=%+v after=%+v", beforeReplay, afterReplay)
	}
	fillPrivateB4C17Capability(t, profile, privateB4EventLimit-int(beforeReplay.Admitted))
	atLimit := profile.eventSnapshot()
	if err := profile.withTargetAppendAdmission(8, func() error { called = true; return nil }); !errors.Is(err, errPrivateB4EventAdmission) || called {
		t.Fatalf("ASSERT_C17_MIXED_LATER_REFUSAL err=%v called=%t", err, called)
	}
	if after := profile.eventSnapshot(); after.Admitted != atLimit.Admitted || after.Admitted != privateB4EventLimit {
		t.Fatalf("ASSERT_C17_MIXED_CUMULATIVE_NON_ROLLBACK before=%+v after=%+v", atLimit, after)
	}
}

func TestPrivateB4C17ConcurrentMixedFamilyAdmission(t *testing.T) {
	profile, failure := newPrivateB4EventAccountC17()
	if failure != "" {
		t.Fatal(failure)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	wg.Add(3)
	go func() {
		defer wg.Done()
		<-start
		errs <- profile.withAcquisitionAdmission(1, func() error { return nil })
	}()
	go func() {
		defer wg.Done()
		<-start
		failure, poison := profile.withCapabilityBatchAdmission(3, func() (session.Failure, bool) { return "", false })
		if failure != "" || poison {
			errs <- errors.New(string(failure))
			return
		}
		errs <- nil
	}()
	go func() {
		defer wg.Done()
		<-start
		errs <- profile.withTargetAppendAdmission(99, func() error { return nil })
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ASSERT_C17_MIXED_CONCURRENT_ADMISSION err=%v", err)
		}
	}
	if snapshot := profile.eventSnapshot(); snapshot.Admitted != 9 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
		t.Fatalf("ASSERT_C17_MIXED_CONCURRENT_SHARED_TOTAL snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C17MixedBatchOwnershipAndCheckedSettlement(t *testing.T) {
	t.Run("equal numeric batches remain family local", func(t *testing.T) {
		profile, failure := newPrivateB4EventAccountC17()
		if failure != "" {
			t.Fatal(failure)
		}
		acquisition, err := profile.beginAcquisitionAdmission(1)
		if err != nil {
			t.Fatal(err)
		}
		entered := make(chan struct{})
		release := make(chan struct{})
		capabilityDone := make(chan session.Failure, 1)
		go func() {
			failure, _ := profile.withCapabilityBatchAdmission(2, func() (session.Failure, bool) {
				close(entered)
				<-release
				return "", false
			})
			capabilityDone <- failure
		}()
		<-entered
		profile.mu.Lock()
		acquisitionBatch, capabilityBatch := uint64(0), uint64(0)
		for _, entry := range profile.entries {
			switch entry.batchFamily {
			case privateB4EventBatchAcquisitionC17:
				acquisitionBatch = entry.batch
			case privateB4EventBatchCapabilityC17:
				capabilityBatch = entry.batch
			}
		}
		profile.mu.Unlock()
		if acquisitionBatch == 0 || acquisitionBatch != capabilityBatch {
			t.Fatalf("ASSERT_C17_EQUAL_NUMERIC_FAMILY_COLLISION_FIXTURE acquisition=%d capability=%d", acquisitionBatch, capabilityBatch)
		}
		if err := acquisition.commit(); err != nil {
			t.Fatalf("ASSERT_C17_MIXED_BATCH_ACQUISITION_COMMIT err=%v", err)
		}
		if snapshot := profile.eventSnapshot(); snapshot.Admitted != 4 || snapshot.InFlight != 2 || snapshot.ActiveCallbacks != 1 {
			t.Fatalf("ASSERT_C17_MIXED_BATCH_TOKEN_LOCAL snapshot=%+v", snapshot)
		}
		close(release)
		if failure := <-capabilityDone; failure != "" {
			t.Fatalf("ASSERT_C17_MIXED_BATCH_CAPABILITY_COMMIT failure=%s", failure)
		}
		if snapshot := profile.eventSnapshot(); snapshot.Admitted != 6 || snapshot.InFlight != 0 || snapshot.ActiveCallbacks != 0 {
			t.Fatalf("ASSERT_C17_MIXED_BATCH_SETTLED snapshot=%+v", snapshot)
		}
	})

	t.Run("acquisition incomplete ownership rejects before subtraction", func(t *testing.T) {
		profile, failure := newPrivateB4EventAccountC17()
		if failure != "" {
			t.Fatal(failure)
		}
		token, err := profile.beginAcquisitionAdmission(0)
		if err != nil {
			t.Fatal(err)
		}
		profile.mu.Lock()
		for i := range profile.entries {
			if profile.entries[i].batchFamily == privateB4EventBatchAcquisitionC17 && profile.entries[i].batch == token.batch {
				profile.entries[i] = privateB4EventEntryC17{}
				break
			}
		}
		profile.mu.Unlock()
		before := profile.eventSnapshot()
		if err := token.commit(); !errors.Is(err, errPrivateB4EventAdmission) {
			t.Fatalf("ASSERT_C17_ACQUISITION_COUNT_MISMATCH_REJECTED err=%v", err)
		}
		if after := profile.eventSnapshot(); after != before || after.InFlight != 2 || after.Admitted != 0 {
			t.Fatalf("ASSERT_C17_ACQUISITION_COUNT_MISMATCH_NO_SUBTRACTION before=%+v after=%+v", before, after)
		}
	})

	t.Run("capability incomplete ownership rejects before subtraction", func(t *testing.T) {
		profile, failure := newPrivateB4EventAccountC17()
		if failure != "" {
			t.Fatal(failure)
		}
		entered := make(chan struct{})
		release := make(chan struct{})
		done := make(chan session.Failure, 1)
		go func() {
			failure, _ := profile.withCapabilityBatchAdmission(2, func() (session.Failure, bool) {
				close(entered)
				<-release
				return "", false
			})
			done <- failure
		}()
		<-entered
		profile.mu.Lock()
		for i := range profile.entries {
			if profile.entries[i].batchFamily == privateB4EventBatchCapabilityC17 && profile.entries[i].state == 1 {
				profile.entries[i] = privateB4EventEntryC17{}
				break
			}
		}
		profile.mu.Unlock()
		close(release)
		if failure := <-done; failure != session.ResourceExhausted {
			t.Fatalf("ASSERT_C17_CAPABILITY_COUNT_MISMATCH_REJECTED failure=%s", failure)
		}
		if snapshot := profile.eventSnapshot(); snapshot.Admitted != 0 || snapshot.InFlight != 2 || snapshot.ActiveCallbacks != 0 || !snapshot.TerminalRequested {
			t.Fatalf("ASSERT_C17_CAPABILITY_COUNT_MISMATCH_NO_SUBTRACTION snapshot=%+v", snapshot)
		}
	})
}

func TestPrivateB4C17MixedStopRestartJoinAllFamilies(t *testing.T) {
	for _, operation := range []string{"STOP", "RESTART"} {
		t.Run(operation, func(t *testing.T) {
			profile, failure := newPrivateB4EventAccountC17()
			if failure != "" {
				t.Fatal(failure)
			}
			entered := make(chan struct{}, 3)
			release := make(chan struct{})
			done := make(chan error, 3)
			go func() {
				done <- profile.withAcquisitionAdmission(1, func() error { entered <- struct{}{}; <-release; return nil })
			}()
			go func() {
				failure, poison := profile.withCapabilityBatchAdmission(2, func() (session.Failure, bool) { entered <- struct{}{}; <-release; return "", false })
				if failure != "" || poison {
					done <- errors.New(string(failure))
					return
				}
				done <- nil
			}()
			go func() {
				done <- profile.withTargetAppendAdmission(11, func() error { entered <- struct{}{}; <-release; return nil })
			}()
			for i := 0; i < 3; i++ {
				<-entered
			}
			joined := make(chan session.Failure, 1)
			go func() { joined <- profile.requestTerminalReleaseC17(context.Background()) }()
			select {
			case failure := <-joined:
				t.Fatalf("ASSERT_C17_MIXED_%s_JOIN_EARLY failure=%s", operation, failure)
			case <-time.After(20 * time.Millisecond):
			}
			close(release)
			for i := 0; i < 3; i++ {
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			}
			select {
			case failure := <-joined:
				if failure != "" {
					t.Fatal(failure)
				}
			case <-time.After(time.Second):
				t.Fatalf("ASSERT_C17_MIXED_%s_JOIN_TIMEOUT", operation)
			}
			if snapshot := profile.eventSnapshot(); snapshot.Admitted != 8 || !snapshot.BackingReleased || snapshot.ActiveCallbacks != 0 {
				t.Fatalf("ASSERT_C17_MIXED_%s_JOIN_SETTLED snapshot=%+v", operation, snapshot)
			}
		})
	}
}

func TestPrivateB4C17DiagnosticsExcludedAndDefaultOffPreserved(t *testing.T) {
	bytesAccount := mustPrivateB4ByteAccountV2(t)
	profile, failure := newPrivateB4EventAccountC17WithBytes(bytesAccount)
	if failure != "" {
		t.Fatal(failure)
	}
	fillPrivateB4C17Capability(t, profile, 1)
	before := profile.eventSnapshot()
	manager := newC15DiagnosticManager(t)
	generation := manager.newDiagnosticGeneration("attempt", "session", 1)
	handle, collector := manager.newPrivateB4DiagnosticOperation(generation, managedprocess.Identity{}, bytesAccount)
	manager.describeDiagnosticOperation(handle, "method", "file:///diagnostic.go", SessionMetadata{})
	manager.completeDiagnosticOperation(handle, collector, diagnosticEventTerminalResponse)
	if after := profile.eventSnapshot(); after != before {
		t.Fatalf("ASSERT_C17_DIAGNOSTICS_EXCLUDED before=%+v after=%+v", before, after)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if failure := profile.requestTerminalReleaseC17(context.Background()); failure != "" {
		t.Fatal(failure)
	}

	called := false
	if failure, poison := writePrivateB4CapabilityResponseC17(nil, "client/registerCapability", []byte(`{"registrations":[{"id":"a","method":"m"}]}`), func() (session.Failure, bool) {
		called = true
		return "", false
	}); failure != "" || poison || !called {
		t.Fatalf("ASSERT_C17_MIXED_DEFAULT_OFF_CAPABILITY failure=%s poison=%t called=%t", failure, poison, called)
	}
	called = false
	if err := (PrivateB4DefinitionBorrow{}).WithTargetAppendAdmission(1, func() error { called = true; return nil }); !errors.Is(err, ErrPrivateB4C17NotEnabled) || called {
		t.Fatalf("ASSERT_C17_MIXED_DEFAULT_OFF_TARGET err=%v called=%t", err, called)
	}
}
