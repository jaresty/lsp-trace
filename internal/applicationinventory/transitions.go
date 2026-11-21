package applicationinventory

import (
	"context"
	"encoding/json"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
)

func initialInventoryReport() Report {
	out := Report{Version: "lsp-trace.application-inventory.v3", Status: "PROVISIONAL", Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", CallbackContainment: "COOPERATIVE_NOT_OS_PHYSICAL", Stages: map[string]Stage{}}
	for _, name := range []string{"census", "native_preparation", "describe", "group_decision", "review", "catalog", "publication"} {
		out.Stages[name] = Stage{}
	}

	return out
}
func recordInventoryStage(out *Report, name string, ok bool) {
	s := out.Stages[name]
	s.Requested++
	if ok {
		s.Successful++
	} else {
		s.Failed++
	}
	out.Stages[name] = s
}
func recordInventoryDescription(out *Report, i int) {
	recordInventoryStage(out, "describe", true)
	out.SuccessfulDescriptions++
	out.WithoutMembership++
	out.Dispositions[i] = Disposition{out.Census[i].ID, "DESCRIBED_UNMATCHED", "NO_SEMANTIC_MEMBERSHIP"}
}
func admitInventoryManifest(ctx context.Context, q Request, budget *sc.CallerBudgetV3, out Report) (Report, uint64, error) {
	if len(q.Manifest.Leads) > 10000 || len(q.Manifest.UntouchedGaps) > 10000 || q.MaxNativeAttempts < 0 || q.MaxNativeAttempts > 100 || len(q.Manifest.Scope) == 0 || len(q.Manifest.Scope) > 4096 || len(q.Manifest.Provenance) == 0 || len(q.Manifest.Provenance) > 4096 {
		return out, 0, ErrInventoryV3
	}

	// Reserve all admitted lead storage, validation and eventual report copies before
	// snapshot; counts/lengths are bounded before loops. Duplicates reject admission.
	var n uint64 = 32768 + uint64(len(q.Manifest.Leads))*1024 + uint64(len(q.Manifest.UntouchedGaps))*128
	for _, l := range q.Manifest.Leads {
		if len(l.ID) == 0 || len(l.ID) > 256 || len(l.Provenance) > 4096 {
			return out, 0, ErrInventoryV3
		}
		n += uint64(len(l.ID)+len(l.Provenance)) * 12
	}
	for _, g := range q.Manifest.UntouchedGaps {
		if len(g) > 4096 {
			return out, 0, ErrInventoryV3
		}
		n += uint64(len(g)) * 12
	}
	if budget.Reserve(ctx, n) != nil {
		return out, 0, ErrInventoryV3
	}
	manifest := q.Manifest
	manifest.Leads = append([]Lead(nil), manifest.Leads...)
	manifest.UntouchedGaps = append([]string(nil), manifest.UntouchedGaps...)
	seen := map[string]bool{}
	for _, l := range manifest.Leads {
		if seen[l.ID] {
			return out, 0, ErrInventoryV3
		}
		seen[l.ID] = true
	}
	out.Scope = manifest.Scope
	out.Provenance = manifest.Provenance
	out.UntouchedGaps = manifest.UntouchedGaps
	out.Census = manifest.Leads // existing pre-reserved immutable admission snapshot
	for _, l := range manifest.Leads {
		out.Dispositions = append(out.Dispositions, Disposition{l.ID, "UNTOUCHED", "NOT_ATTEMPTED"})
	}
	stage := func(name string, ok bool) { recordInventoryStage(&out, name, ok) }
	for range manifest.Leads {
		stage("census", true)
	}

	return out, n, nil
}
func checkInventoryDecisionActor(ctx context.Context, budget *sc.CallerBudgetV3, raw []byte, review bool, actor string) error {
	if review {
		if budget.Reserve(ctx, uint64(len(raw))*8+4096) != nil {
			return ErrInventoryV3
		}
		var d cat.NativeDecisionV3
		if cat.DecodeNativeV3(raw, &d) != nil || d.Actor != actor {
			return ErrInventoryV3
		}
	}
	return nil
}
func applyInventoryDecision(ctx context.Context, budget *sc.CallerBudgetV3, c cat.NativeCatalogV3, raw []byte, review bool, actor string) (cat.NativeCatalogV3, error) {
	if e := checkInventoryDecisionActor(ctx, budget, raw, review, actor); e != nil {
		return cat.NativeCatalogV3{}, e
	}
	return cat.ApplyNativeDecisionV3(ctx, c, raw, review)
}
func reconcileInventory(ctx context.Context, budget *sc.CallerBudgetV3, out Report, descriptions []sc.BoundSelectedDescriptionV3, corrected cat.NativeCatalogV3) (Report, []byte, error) {
	// Stage a charged reconciliation; no corrected report state escapes until
	// every fallible read and per-lead/count check below has succeeded.
	withMembership, withoutMembership := corrected.MembershipCounts()
	revision := corrected.Revision()
	if budget.Reserve(ctx, uint64(len(out.Dispositions))*192+uint64(len(descriptions))*1024+8192) != nil {
		return out, nil, ErrInventoryV3
	}
	reconciled := append([]Disposition(nil), out.Dispositions...)
	finalRaw, e := corrected.Bytes(ctx)
	if e != nil {
		return out, nil, e
	}
	var final cat.NativeCatalogWireV3
	if budget.Reserve(ctx, uint64(len(finalRaw))*8+4096) != nil || cat.DecodeNativeV3(finalRaw, &final) != nil {
		return out, nil, ErrInventoryV3
	}
	membership := map[string]bool{}
	for _, g := range final.Groups {
		for _, id := range g.Members {
			membership[id] = true
		}
	}
	// Description order follows processing, evidence order is deterministic by ID.
	j := 0
	for i := range reconciled {
		if reconciled[i].Status == "DESCRIBED_UNMATCHED" {
			if j >= len(descriptions) {
				return out, nil, ErrInventoryV3
			}
			ev, e := descriptions[j].Evidence(ctx)
			if e != nil {
				return out, nil, e
			}
			j++
			if membership[ev.ID] {
				reconciled[i].Status = "DESCRIBED_WITH_MEMBERSHIP"
				reconciled[i].Reason = "SUPPLIED_SEMANTIC_PROPOSAL_REVIEWED_UNAUTHENTICATED"
			}
		}
	}
	if len(reconciled) != len(out.Census) || j != len(descriptions) {
		return out, nil, ErrInventoryV3
	}
	matched, unmatched := 0, 0
	for i, d := range reconciled {
		if d.LeadID != out.Census[i].ID {
			return out, nil, ErrInventoryV3
		}
		switch d.Status {
		case "DESCRIBED_WITH_MEMBERSHIP":
			matched++
		case "DESCRIBED_UNMATCHED":
			unmatched++
		}
	}
	if matched != withMembership || unmatched != withoutMembership || matched+unmatched != out.SuccessfulDescriptions {
		return out, nil, ErrInventoryV3
	}
	// One report-state commit; there is no intervening fallible operation.
	out.WithMembership, out.WithoutMembership, out.CatalogRevision, out.Dispositions = withMembership, withoutMembership, revision, reconciled

	return out, finalRaw, nil
}
func publishInventory(ctx context.Context, q Request, budget *sc.CallerBudgetV3, out Report, n uint64, baseRaw, proposedRaw, finalRaw []byte) (Report, error) {
	stage := func(name string, ok bool) { recordInventoryStage(&out, name, ok) }
	// Reserve reporting, all publication bytes, write and readback before effects.
	if budget.Reserve(ctx, n+uint64(len(finalRaw))*2+32768) != nil {
		return out, ErrInventoryV3
	}
	out.CallerAccounting = "PRE_PUBLICATION_SNAPSHOT_FINAL_IN_MANIFEST"
	out.Published = true
	stage("publication", true)
	callerBytes, semanticReserved := budget.Usage()
	out.CallerBytes, out.SemanticReservedNS = callerBytes, int64(semanticReserved)
	report, e := json.Marshal(out)
	if e != nil {
		out.Published = false
		out.Stages["publication"] = Stage{Requested: 1, Failed: 1}
		return out, e
	}
	_, e = PublishV3(ctx, q.Destination, []ArtifactV3{{"catalog-base.json", baseRaw}, {"catalog-proposed.json", proposedRaw}, {"catalog-corrected.json", finalRaw}, {"report.json", report}}, budget)
	if e != nil {
		out.Published = false
		out.Stages["publication"] = Stage{Requested: 1, Failed: 1}
		return out, e
	}
	out.Published = true
	return out, nil
}
