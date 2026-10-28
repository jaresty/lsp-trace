//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"testing"

	"lsp-trace/internal/graphprovenance"
)

// The predicate is deliberately private. Its production call site is strictly
// after ParseSourceWithholdingPrivateV1, never an alternative receipt validator.
func TestScopedPartialSourceDispositionTablePrivate(t *testing.T) {
	const workspace = "file:///owned/workspace"
	rows := []struct{ uri, motivated string }{
		{workspace + "/a.go", ""},
		{"file:///outside/a.go", "OUTSIDE_SCOPE"},
		{"untitled:buffer", "VIRTUAL_URI"},
		{"relative.go", "INVALID_URI"},
	}
	statuses := []string{"READABLE", graphprovenance.PolicyWithheldPrivateV1, "OUTSIDE_SCOPE", "VIRTUAL_URI", "INVALID_URI", "UNREADABLE", "MISSING", "NONREGULAR", "BYTE_LIMIT_EXCEEDED", "CANCELLED", "BUDGET_EXCEEDED", "FORGED"}
	for _, row := range rows {
		for _, status := range statuses {
			t.Run(row.motivated+"/"+status, func(t *testing.T) {
				want := status == row.motivated
				if row.motivated == "" {
					want = status == "READABLE" || status == graphprovenance.PolicyWithheldPrivateV1
				}
				got := scopedPartialSourceAcceptedPrivate(workspace, graphprovenance.Receipt{URI: row.uri, Status: status})
				if got != want {
					t.Fatalf("ASSERT_MOTIVATED_SOURCE_DISPOSITION: uriClass=%s status=%s got=%v want=%v", row.motivated, status, got, want)
				}
			})
		}
	}
}

func TestScopedPartialSourceClassificationRequiresClosedCarrierPrivate(t *testing.T) {
	WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *ScopedBatchFixturePrivate) {
		f.Fault("empty-directions")
		selection, authority, binding, req := closedInputsPrivate(t, ctx, f)
		raw, _, err := AcquireScopedPartialInspectionPrivate(ctx, f.Manager, selection, authority, binding, req)
		if err != nil {
			t.Fatal(err)
		}
		if err = validateScopedPartialSourcesPrivate(raw); err != nil {
			t.Fatal(err)
		}
		carrier, err := graphprovenance.ParseSourceWithholdingPrivateV1(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, status := range []string{"OUTSIDE_SCOPE", "VIRTUAL_URI", "INVALID_URI", "FORGED"} {
			copy := carrier
			copy.Evidence.Captures = append([]graphprovenance.Receipt(nil), carrier.Evidence.Captures...)
			copy.Evidence.Captures[0].Status = status
			changed, err := json.Marshal(copy)
			if err != nil {
				t.Fatal(err)
			}
			if err = validateScopedPartialSourcesPrivate(changed); err == nil {
				t.Fatal("ASSERT_UNVALIDATED_STATUS_REFUSED", status)
			}
		}
	})
}
