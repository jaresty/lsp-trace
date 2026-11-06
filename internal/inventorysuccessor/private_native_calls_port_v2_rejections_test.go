package inventorysuccessor_test

import (
	"context"
	"errors"
	"testing"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programccompose"
)

func TestPrivateNativeConstructorDiscriminatorWholeRejections(t *testing.T) {
	for _, mode := range []string{"missing-source-file", "unequal-actual-RPC-edge"} {
		t.Run(mode, func(t *testing.T) {
			m, q, _ := nativeConstructorFixtureV2(t, false, false, mode == "unequal-actual-RPC-edge", mode == "missing-source-file")
			err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
				_, e := inventorysuccessor.ConstructNativeCallsPrivateV2(ctx, m, q)
				return e
			})
			if mode == "missing-source-file" {
				if !errors.Is(err, graphprovenance.ErrPrivateNativeOrdinaryCaptureV2) {
					t.Fatalf("ASSERT_WHOLE_MISSING_SOURCE: %v", err)
				}
				t.Log("ASSERT_WHOLE_MISSING_SOURCE: PASS; private native constructor ordinary source coverage refused")
			} else {
				if programccompose.ErrorBranch(err) != programccompose.BranchEdgeConflict {
					t.Fatalf("ASSERT_WHOLE_UNEQUAL_EDGE: branch=%s err=%v", programccompose.ErrorBranch(err), err)
				}
				t.Log("ASSERT_WHOLE_UNEQUAL_EDGE: PASS; EDGE_CONFLICT")
			}
		})
	}
}
