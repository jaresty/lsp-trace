package applicationinventory

import (
	"context"
	"encoding/json"
	"io"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"path/filepath"
	"testing"
	"time"
)

func TestConnectedReviewSinglePrevalidationCharge(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	exchange := func(actor string) ByteExchange {
		return func(_ context.Context, raw []byte, out io.Writer) error {
			var c cat.NativeCatalogWireV3
			if err := cat.DecodeNativeV3(raw, &c); err != nil {
				return err
			}
			return json.NewEncoder(out).Encode(cat.NativeDecisionV3{Predecessor: c.Revision, Actor: actor, Reason: "bounded review", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
		}
	}
	q := Request{Manifest: Manifest{Scope: "fixture", Provenance: "empty bounded fixture"}, Preparer: func(context.Context, Lead) (NativeResult, error) { return NativeResult{}, nil }, MaxNativeAttempts: 1, NativeRetainedUpper: 4096, Interpreter: "interpreter", ReviewerIdentity: "reviewer", HostWindow: time.Second, GroupDecision: exchange("interpreter"), Reviewer: exchange("reviewer"), Destination: filepath.Join(root, "out"), BudgetBytes: 2419000}
	r, err := Run(context.Background(), q)
	if err != nil || !r.Published {
		t.Fatalf("single review charge must fit tight budget: used=%d stages=%v err=%v", r.CallerBytes, r.Stages, err)
	}
	t.Logf("single review charge: used=%d", r.CallerBytes)
	q.Destination = filepath.Join(root, "wrong")
	q.Reviewer = exchange("wrong-actor")
	r, err = Run(context.Background(), q)
	if err == nil || r.Published || r.Stages["review"].Failed != 1 {
		t.Fatalf("wrong actor refused with failed review stage: %+v %v", r, err)
	}
}
