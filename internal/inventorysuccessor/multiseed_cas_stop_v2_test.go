package inventorysuccessor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

type observedMultiSeedCASV2 struct {
	censussourcecontinuation.HeadStore
	calls     int
	uncertain bool
}

func (h *observedMultiSeedCASV2) CompareAndSwap(ctx context.Context, key string, revision censussourcecontinuation.HeadRevision, raw []byte) censussourcecontinuation.CASResult {
	h.calls++
	r := h.HeadStore.CompareAndSwap(ctx, key, revision, raw)
	if h.uncertain && r.Outcome == censussourcecontinuation.CASCommitted {
		r.Outcome = censussourcecontinuation.CASCommitUncertain
	}
	return r
}
func TestMultiSeedV2CASStopsAreSticky(t *testing.T) {
	t.Log("procedure: real private CAS not-committed/commit-uncertain/cancel -> typed stop; repeat same operation -> no second CAS")
	for _, name := range []string{"conflict", "uncertain", "cancel"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			directory, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Chmod(directory, 0700); err != nil {
				t.Fatal(err)
			}
			root, err := publication.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			memory := &memoryHeads{}
			request := validRequest(memory)
			if _, err = coordinateTest(ctx, request); err != nil {
				t.Fatal(err)
			}
			heads, err := NewMultiSeedSuccessorHeadsV2(root)
			if err != nil {
				t.Fatal(err)
			}
			if r := heads.CompareAndSwap(ctx, request.HeadKey, censussourcecontinuation.HeadRevision{}, memory.raw); r.Outcome != censussourcecontinuation.CASCommitted {
				t.Fatal(r)
			}
			revision, err := ParseHeadRevision(memory.raw)
			if err != nil {
				t.Fatal(err)
			}
			proxy := &observedMultiSeedCASV2{HeadStore: heads, uncertain: name == "uncertain"}
			operation := &MultiSeedCheckpointOperationV2{headKey: request.HeadKey}
			want := ErrMultiSeedIndeterminateV2
			expectedCalls := 1
			switch name {
			case "conflict":
				revision.Generation++
				want = ErrMultiSeedConflictV2
			case "cancel":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
				want = context.Canceled
				expectedCalls = 0
			}
			err = operation.cas(ctx, proxy, revision, memory.raw)
			if !errors.Is(err, want) {
				t.Fatalf("STICKY_TYPED_CAS_STOP: wanted=%v got=%v", want, err)
			}
			err = operation.cas(context.Background(), proxy, revision, memory.raw)
			if !errors.Is(err, want) || proxy.calls != expectedCalls || operation.ObservationsV2().CASAttempts != uint64(expectedCalls) {
				t.Fatalf("NO_RETRY_AFTER_CAS_STOP: calls=%d want=%d err=%v", proxy.calls, expectedCalls, err)
			}
		})
	}
}
