//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	"fmt"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

// PrepareCohortRetained requires the actual already-admitted V3 retained owner.
// Exact original graph references remain references; no raw graph is rewritten
// or unioned, and no source-text CALLS relation is inferred.
func PrepareCohortRetained(ctx context.Context, source Config, c CohortConfig, owner pilot.RetainedPageCheckpointOwnerPrivateV3) ([]CohortItem, error) {
	if ValidateConfig(source) != nil || ValidateCohortConfig(c) != nil || c.SourceConfigSHA256 != ConfigDigest(source) || c.ScopeSHA256 != source.ScopeSHA256 || c.Requested != source.Requested {
		return nil, ErrInvalid
	}
	manifest, _, _, _, e := owner.HostFrontierPrivateV3()
	if e != nil {
		return nil, e
	}
	if manifest.Application != source.Application || manifest.ScopeSHA256 != source.ScopeSHA256 || manifest.Revision.Policy.ConfigSHA256 != source.ProducerConfigSHA256 || int(manifest.Targets) != source.Requested || manifest.NextBatch != source.StartBatch {
		return nil, ErrInvalid
	}
	var items []CohortItem
	for n := manifest.NextBatch; n < int(manifest.Revision.BatchDenominator); n++ {
		batch, e := owner.SelectedBatch(ctx, n, source.Exchange)
		if e != nil {
			return items, e
		}
		view, e := batch.Read(ctx, source.Exchange)
		if e != nil {
			return items, e
		}
		p, e := derived.ProjectRetainedCheckpointPrivateV3(ctx, batch, source.Exchange)
		if e != nil {
			return items, e
		}
		a, e := derived.ComputeInstabilityRetainedCheckpointPrivateV3(ctx, p, source.Seeds, source.Exchange)
		if e != nil {
			return items, e
		}
		reader, e := prep.NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, a, source.Preparation, source.Exchange)
		if e != nil {
			return items, e
		}
		_, prepared, e := reader.Advance(ctx, 1, source.Exchange)
		if e != nil {
			return items, e
		}
		raw, e := prepared.Read(ctx, source.Exchange)
		if e != nil {
			return items, e
		}
		var r prep.OwnASTReportRetainedCheckpointPrivateV3
		if json.Unmarshal(raw, &r) != nil || r.PreparedCount != len(r.Bodies) || r.Accepted || r.Authority != 0 || r.RequestType != prep.RequestTypeRetainedCheckpointPrivateV3 {
			return items, ErrInvalid
		}
		pub, e := prepared.Publication(ctx, source.Exchange)
		if e != nil {
			return items, e
		}
		refs, e := pub.Advance(ctx, 1, source.Exchange)
		if e != nil {
			return items, e
		}
		cost := pilot.PageCostPrivateV3{LogicalBytes: uint64(len(raw))*16 + 65536, ReadHashBytes: uint64(len(raw))*32 + 65536, WriteBytes: uint64(len(raw))*4 + 65536, Work: uint64(len(raw)) + 1024}
		if e = owner.ReserveHostWorkPrivateV3(ctx, cost, source.Exchange); e != nil {
			return items, e
		}
		if e = owner.RecordPublishedReferences(ctx, batch, [3]string{refs.CanonicalSHA256, refs.AssessmentSHA256, refs.PreparedSHA256}, source.Exchange); e != nil {
			return items, e
		}
		if _, e = owner.CheckpointCompletedBatch(ctx, batch, hash([]byte(fmt.Sprintf("%s/cohort-next/%d", source.Application, n))), source.Exchange); e != nil {
			return items, e
		}
		for _, b := range r.Bodies {
			input := r.SourceInputID + "/" + b.Target
			items = append(items, CohortItem{ItemID: "cohort-item-" + hash([]byte(input))[7:23], InputID: input, Target: b.Target, Attempt: fmt.Sprintf("%s/batch-%d/item-%d", c.Grant, n, len(items)), Grant: c.Grant, ConfigSHA256: ConfigDigest(source), ScopeSHA256: source.ScopeSHA256, PreparedSHA256: refs.PreparedSHA256, CheckpointSHA256: r.Source.CheckpointSHA256, OriginalRawReferences: view.ObjectSHA256, Body: b.Declaration.Body, OwnBodySHA256: b.OwnBodySHA256, Revision: 1})
		}
	}
	if len(items) != source.Requested {
		return items, ErrInvalid
	}
	return items, nil
}
func RunCohortRetained(ctx context.Context, source Config, c CohortConfig, owner pilot.RetainedPageCheckpointOwnerPrivateV3, b CohortBackend) (CohortReport, error) {
	items, e := PrepareCohortRetained(ctx, source, c, owner)
	if e != nil {
		return CohortReport{}, e
	}
	return RunCohort(ctx, c, items, cohortReservedBackend{owner: owner, source: source, config: c, backend: b})
}
