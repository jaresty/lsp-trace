//go:build darwin || linux

package applicationinventory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/describerequest"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	tp "lsp-trace/internal/targetpacket"
)

// Pure wire data exercises only the production data reader. This is not an
// accepted head, opaque native admission, or evidence of producer correctness.
func TestDescriptionPureDataBoundary(t *testing.T) {
	for _, kind := range []string{"valid", "corrupt", "unreferenced", "intrinsic"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			if e := os.Chmod(dir, 0700); e != nil {
				t.Fatal(e)
			}
			root, e := publication.OpenRoot(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer root.Close()
			s := DurableStore{Root: root, Directory: dir}
			u := tp.PrivateSourceUnit{Version: tp.PrivateSourceVersion, ClaimCeiling: tp.PrivateSourceCeiling, ID: "pure-data", Body: "fixture", SourceDigest: cat.NativeDigestV3([]byte("fixture"))}
			request, e := describerequest.FormatPrivateSourceRequestCandidate(u, 1000, 8<<20)
			if e != nil {
				t.Fatal(e)
			}
			d := cat.SelectedEvidenceV3{ID: "pure-data", HandoffID: "data", CheckpointID: "data", Unit: u, Request: request}
			ref := ownerStore(t, s, "inventory-description-", d)
			if kind == "intrinsic" {
				d.Request.RecordID = "invalid"
				ref = ownerStore(t, s, "inventory-description-", d)
			}
			if kind == "unreferenced" {
				d.Interpreter = "unreferenced"
				ownerStore(t, s, "inventory-description-", d)
			}
			if kind == "corrupt" {
				if e = os.WriteFile(filepath.Join(dir, "inventory-description-"+ref.ID+".json"), []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			_, e = sc.ReopenInventoryDescription(context.Background(), root, ref, ownerBudget(t, 64<<20))
			if kind == "corrupt" || kind == "intrinsic" {
				if !errors.Is(e, sc.ErrDurableDescribeV1) {
					t.Fatalf("reader boundary: %v", e)
				}
			} else if e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestDescriptionStructuralReservation(t *testing.T) {
	v := durableInventoryState{RunID: strings.Repeat("a", 64), Cursor: 3, Prepared: make([]publication.StoredObject, 3)}
	for _, id := range []string{"a", "b", "c"} {
		v.Report.Census = append(v.Report.Census, Lead{ID: id})
		v.Report.Dispositions = append(v.Report.Dispositions, Disposition{LeadID: id, Status: "UNAVAILABLE"})
	}
	// Nil Root is deliberate: these structural-only cases must not read objects.
	for _, limit := range []uint64{7167, 7168} {
		b := ownerBudget(t, limit)
		ds, e := (DurableStore{}).descriptions(context.Background(), v, b)
		if limit == 7167 {
			if e != ErrDurableInventory || !b.Exhausted() {
				t.Fatalf("reservation must refuse before work: %v", e)
			}
		} else {
			used, _ := b.Usage()
			if e != nil || len(ds) != 0 || used != 7168 {
				t.Fatalf("exact structure charge: %d %v", used, e)
			}
		}
	}
	v.Report.Census[1].ID = "a"
	v.Report.Dispositions[1].LeadID = "a"
	if _, e := (DurableStore{}).descriptions(context.Background(), v, ownerBudget(t, 7168)); e != ErrDurableInventory {
		t.Fatalf("duplicate index: %v", e)
	}
}
