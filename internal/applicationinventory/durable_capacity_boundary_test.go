//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"strings"
	"testing"
	"time"
)

func TestCompletionClosedStateGrowth(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 100} {
		v := durableInventoryState{Schema: "lsp-trace.inventory-state.v1", Report: initialInventoryReport(), Prepared: make([]publication.StoredObject, n), Descriptions: []publication.StoredObject{}}
		ref := publication.StoredObject{ID: strings.Repeat("f", 64), Bytes: 8 << 20}
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("%03d", i) + strings.Repeat("<", 253)
			v.Report.Census = append(v.Report.Census, Lead{id, strings.Repeat("<", 4096)})
			v.Report.Dispositions = append(v.Report.Dispositions, Disposition{id, "PREPARED", "DESCRIBE_PENDING"})
			v.Prepared[i] = ref
		}
		before, _ := json.Marshal(v)
		v.Cursor = n
		v.Base = ref
		v.Proposed = ref
		v.Corrected = ref
		for i := 0; i < n; i++ {
			v.Descriptions = append(v.Descriptions, ref)
			v.Report.Dispositions[i].Status = strings.Repeat("x", 32)
			v.Report.Dispositions[i].Reason = strings.Repeat("x", 64)
		}
		v.Report.CallerBytes = 64 << 20
		v.Report.PublicationRunReservedBytes = 64 << 20
		v.Report.PublicationRunWork = 1 << 32
		v.Report.PublicationRunPackets = 64
		v.Report.SemanticReservedNS = int64(120 * time.Second)
		v.Report.CallerAccounting = "PRE_PUBLICATION_SNAPSHOT_FINAL_IN_MANIFEST"
		v.Report.CatalogRevision = "sha256:" + strings.Repeat("f", 64)
		v.Report.Published = true
		v.Report.SuccessfulDescriptions = n
		v.Report.WithMembership = n
		v.Report.WithoutMembership = n
		for key := range v.Report.Stages {
			v.Report.Stages[key] = Stage{Requested: 10000, Successful: 10000, Failed: 10000}
		}
		after, _ := json.Marshal(v)
		if len(after) > len(before)+n*256+4096 {
			t.Fatalf("state underquote N=%d before=%d after=%d", n, len(before), len(after))
		}
	}
}

func TestCompletionClosedCatalogEscaping(t *testing.T) {
	for p := 2; p <= 5; p++ {
		w := cat.NativeCatalogWireV3{Version: "lsp-trace.native-catalog.v3", Revision: "sha256:" + strings.Repeat("f", 64), Status: "PROVISIONAL", Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", Origin: "SUPPLIED_UNAUTHENTICATED", Actor: strings.Repeat("<", 256), ReviewIdentity: "NOT_AUTHENTICATED", Evidence: []cat.SelectedEvidenceV3{}, Groups: []cat.NativeGroupV3{}, Affected: []string{}}
		sum := 0
		for i := 0; i < p; i++ {
			e := cat.SelectedEvidenceV3{ID: fmt.Sprintf("sha256:%064d", i), Record: json.RawMessage(`{}`)}
			e.Unit.Body = strings.Repeat("<", 32769)
			raw, _ := json.Marshal(e)
			sum += len(raw)
			w.Evidence = append(w.Evidence, e)
		}
		base, _ := json.Marshal(w)
		if len(base) > sum+4096 {
			t.Fatal("catalog base underquote")
		}
		w.Predecessor = "sha256:" + strings.Repeat("f", 64)
		w.Reason = strings.Repeat("<", 4096)
		for i, e := range w.Evidence {
			id := fmt.Sprint(i) + strings.Repeat("<", 255)
			w.Groups = append(w.Groups, cat.NativeGroupV3{ID: id, Label: strings.Repeat("<", 4096), Reason: strings.Repeat("<", 4096), Members: []string{e.ID}})
			w.Affected = append(w.Affected, id)
		}
		raw, _ := json.Marshal(w)
		bound := sum + 4096 + 6*(p*8448+4352+p*256) + p*256 + 1024
		if len(raw) > bound {
			t.Fatalf("catalog escaped underquote P=%d actual=%d bound=%d", p, len(raw), bound)
		}
	}
}

// DecodeNativeV3 accepts JSON's replacement of invalid UTF-8. The 64KiB raw
// cap therefore does not by itself cap decoded group string bytes at64KiB.
// This is a quote/parser boundary test, not a native eight-item admission.
func TestCompletionDecodedGroupStringReservation(t *testing.T) {
	const p = 8
	d := cat.NativeDecisionV3{Predecessor: "sha256:" + strings.Repeat("f", 64), Actor: "a", Reason: "x", Affected: []string{}, Groups: []cat.NativeGroupV3{}}
	refs := make([]publication.StoredObject, p)
	payloads := make([]uint64, p)
	for i := 0; i < p; i++ {
		d.Groups = append(d.Groups, cat.NativeGroupV3{ID: fmt.Sprint(i) + strings.Repeat("\ufffd", 85), Label: strings.Repeat("\ufffd", 1365) + "x", Reason: strings.Repeat("\ufffd", 1365) + "x", Members: []string{fmt.Sprintf("sha256:%064d", i)}})
		refs[i] = publication.StoredObject{ID: strings.Repeat("f", 64), Bytes: 4096}
		payloads[i] = 4096
	}
	raw, _ := json.Marshal(d)
	raw = bytes.ReplaceAll(raw, []byte("\ufffd"), []byte{0xff})
	if len(raw) > 65536 {
		t.Fatal("specimen exceeds raw cap")
	}
	var decoded cat.NativeDecisionV3
	if cat.DecodeNativeV3(raw, &decoded) != nil {
		t.Fatal("valid parser specimen refused")
	}
	stringsBytes := uint64(0)
	for _, g := range decoded.Groups {
		if len(g.ID) > 256 || len(g.Label) > 4096 || len(g.Reason) > 4096 {
			t.Fatal("specimen exceeds field limits")
		}
		stringsBytes += uint64(len(g.ID) + len(g.Label) + len(g.Reason))
	}
	limits := dw.SuppliedResponseLimits{MaxAdapterBytes: 1024, MaxSemanticBytes: 512}
	q, e := quoteDurableCompletion(refs, 4096, 32768, limits, payloads)
	if e != nil {
		t.Fatal(e)
	}
	base := uint64(p*(4096+1024+2048) + 4096)
	grown := base + 6*65536 + p*256 + 1024
	seal := uint64(24*p*4096 + p*32768 + 16384)
	other := 16*q.StateUpper + 65536 + 8*base + 16384 + 16*65536 + 16384 + seal + p*1024 + grown + 1
	if q.Stages[p] < other+12*stringsBytes {
		t.Fatalf("decoded group seal underquote: raw=%d decoded=%d quoted=%d required=%d", len(raw), stringsBytes, q.Stages[p], other+12*stringsBytes)
	}
}

func TestCompletionStageCannotBorrowGlobalCapacity(t *testing.T) {
	// A real state-read reservation must refuse before touching even the root when
	// its local slice cannot fund it, despite ample unused global capacity.
	b, e := cat.NewReservedCallerBudgetV3(time.Now().Add(time.Minute), 1024, 1<<20, 0)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := b.Usage()
	_, e = (DurableStore{}).state(context.Background(), durableInventoryHead{State: publication.StoredObject{ID: strings.Repeat("f", 64), Bytes: 4096}}, b)
	if e == nil {
		t.Fatal("underfunded stage borrowed global capacity")
	}
	after, _ := b.Usage()
	if after != before {
		t.Fatal("failed stage refunded or partially spent", before, after)
	}
}

func TestCompletionLargeInputsRefuseBeforeEffects(t *testing.T) {
	limits := dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}
	for _, size := range []int{32767, 32768, 32769, 8 << 20, (8 << 20) + 1} {
		refs := []publication.StoredObject{{ID: strings.Repeat("f", 64), Bytes: size}}
		q, e := quoteDurableCompletion(refs, 4096, 32768, limits, []uint64{uint64(size) + 65536})
		if size >= 8<<20 && e == nil {
			t.Fatal("oversized completion admitted")
		}
		if e == nil {
			if total, ok := capacitySum(q.Stages...); !ok || total > 64<<20 {
				t.Fatal("unfunded plan")
			}
		}
	}
	if _, e := quoteDurableCompletion([]publication.StoredObject{{ID: strings.Repeat("f", 64), Bytes: 4096}}, 4096, 1, limits, []uint64{^uint64(0)}); e == nil {
		t.Fatal("payload overflow admitted")
	}
}
