//go:build darwin

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

// This is an explicit synthetic test grant, not the runtime application cap.
func pipelineGrantV3() pilot.PageCostPrivateV3 {
	return pilot.PageCostPrivateV3{LogicalBytes: 1 << 40, ReadHashBytes: 1 << 40, WriteBytes: 1 << 34, Work: 1 << 32}
}
func TestProducerPagesPrivateV3PipelineTwenty(t *testing.T) {
	ctx := context.Background()
	grant := pipelineGrantV3()
	owner, revision := pilot.ControlledTwentyPagesPrivateV3(t, grant)
	refs := map[string]bool{}
	inputs := map[string]bool{}
	prepared := 0
	var firstAssessment derived.AssessmentPrivateV3
	var firstPrepared prep.PreparedPrivateV3
	var firstBytes []byte
	var firstSourceURI string
	for i := 0; i < 20; i++ {
		reader, e := owner.SelectedBatchReader(ctx, revision, i, grant)
		if e != nil {
			t.Fatal("ASSERT_V3_ADMISSION", i, e)
		}
		_, batch, e := reader.Advance(ctx, 1, grant)
		if e != nil {
			t.Fatal("ASSERT_V3_ADMISSION", i, e)
		}
		p, e := derived.ProjectPrivateV3(ctx, batch, grant)
		if e != nil {
			t.Fatal("ASSERT_V3_PROJECT", i, e)
		}
		evidence, e := p.Read(ctx, grant)
		if e != nil {
			t.Fatal(e)
		}
		expectedSites := 0
		for _, edge := range evidence.Selected.Source.Evidence.Acquisition.Graph.Edges {
			expectedSites += len(edge.CallSites)
		}
		if !bytes.Equal(evidence.OriginalNativeV3, evidence.Selected.Source.Evidence.GraphBytes) || bytes.Equal(evidence.OriginalNativeV3, evidence.DerivedCanonicalV5) || len(evidence.Math.Occurrences) != expectedSites {
			t.Fatal("ASSERT_V3_EXACT_NATIVE_AND_DUPLICATE_CALLS", i, len(evidence.Math.Occurrences), expectedSites)
		}
		if !bytes.Contains(evidence.OriginalNativeV3, []byte("1.000e+2")) {
			t.Fatal("ASSERT_V3_NUMERIC_LEXEME", i)
		}
		a, e := derived.ComputeInstabilityPrivateV3(ctx, p, []uint64{3, 7}, grant)
		if e != nil {
			t.Fatal("ASSERT_V3_ASSESS", i, e)
		}
		assessmentReport, _, err := derived.ValidateInstabilityPrivateV3(ctx, p, a, grant)
		if err != nil || !assessmentReport.Source.Partial || assessmentReport.Math.IndependentSources != 1 || len(assessmentReport.Math.Runs) != 2 {
			t.Fatal("ASSERT_V3_ASSESSMENT_PROFILE", i, err)
		}
		for _, run := range assessmentReport.Math.Runs {
			if run.Partition.Algorithm != "TRIVIAL_SINGLE_NODE_PRIVATE_V3" || len(run.Partition.Communities) != 1 || len(run.Partition.Communities[0].Members) != 1 || run.Partition.Communities[0].Members[0] != evidence.Math.NodeIdentities[0] {
				t.Fatal("ASSERT_V3_TRIVIAL_PARTITION_NOT_LEIDEN", i)
			}
		}
		limits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
		var own prep.PreparedPrivateV3
		if i == 0 {
			firstAssessment = a
			cursor, err := prep.NewOwnASTReaderPrivateV3(ctx, p, a, limits, grant)
			if err != nil {
				t.Fatal(err)
			}
			progress, empty, err := cursor.Advance(ctx, 1, pilot.PageCostPrivateV3{})
			if !errors.Is(err, pilot.ErrPagesPausedPrivateV3) || progress.Prepared != 0 {
				t.Fatal("ASSERT_V3_PREP_BUDGET_PAUSE", progress, err)
			}
			if _, err = empty.Read(ctx, grant); err == nil {
				t.Fatal("ASSERT_V3_PAUSE_HAS_NO_BODY")
			}
			if _, _, err = cursor.Advance(ctx, 1, grant); err == nil {
				t.Fatal("ASSERT_V3_PREP_STALE_GENERATION")
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			progress, _, err = cursor.Advance(canceled, 2, grant)
			if !errors.Is(err, context.Canceled) || progress.Status != "CANCELLED" {
				t.Fatal("ASSERT_V3_PREP_CANCEL", progress, err)
			}
			// Detached stale kind/range and foreign source bytes cannot steer this owner.
			for j := range evidence.Selected.Source.Evidence.Acquisition.Targets {
				n := evidence.Selected.Source.Evidence.Acquisition.Targets[j].Resolution.Identity
				if n != nil {
					n.Kind = 1
					n.Range.End.Line = 999
					n.SelectionRange.Start.Line = 998
				}
			}
			if _, err = p.OwnSource(ctx, "file:///foreign-source.go", grant); err == nil {
				t.Fatal("ASSERT_V3_NO_FOREIGN_SOURCE_FALLBACK")
			}
			_, own, e = cursor.Advance(ctx, 3, grant)
			if _, _, err = cursor.Advance(ctx, 4, grant); err == nil {
				t.Fatal("ASSERT_V3_PREP_TERMINAL_REPLAY")
			}
		} else {
			if i == 1 {
				if _, err := prep.NewOwnASTReaderPrivateV3(ctx, p, firstAssessment, limits, grant); err == nil {
					t.Fatal("ASSERT_V3_FOREIGN_ASSESSMENT")
				}
			}
			own, e = prep.PrivateOwnASTPrepareV3(ctx, p, a, limits, grant)
		}
		if e != nil {
			t.Fatal("ASSERT_V3_PIPELINE_TWENTY_OWN_BODIES", i, e)
		}
		raw, e := own.Read(ctx, grant)
		if e != nil {
			t.Fatal(e)
		}
		var report prep.OwnASTReportPrivateV3
		if json.Unmarshal(raw, &report) != nil {
			t.Fatal("ASSERT_V3_REPORT")
		}
		if report.PreparedCount != 1 || len(report.Bodies) != 1 || report.Bodies[0].Declaration.Body != fmt.Sprintf("func Owned%02d() {}", i) || report.Bodies[0].OwnBodySHA256 != pilot.Digest([]byte(fmt.Sprintf("func Owned%02d() {}", i))) || report.FullTargetCount != 20 || report.FullBatchCount != 20 || report.Source.BatchOrdinal != i || report.Authority != 0 || report.Accepted || report.FeatureIdentity != "UNRESOLVED" || report.SourceCompleteness != "UNKNOWN" || report.AbsenceInference != "PROHIBITED" {
			t.Fatal("ASSERT_V3_PIPELINE_TWENTY_OWN_BODIES", i, string(raw))
		}
		if i == 0 {
			firstPrepared = own
			firstBytes = bytes.Clone(raw)
			firstSourceURI = report.Bodies[0].SourceURI
		}
		if i == 19 {
			prior, err := firstPrepared.Read(ctx, grant)
			if err != nil || !bytes.Equal(prior, firstBytes) {
				t.Fatal("ASSERT_V3_PRIOR_PREPARED_FACTS_IMMUTABLE", err)
			}
		}
		pub, err := own.Publication(ctx, grant)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			status, err := pub.Advance(ctx, 1, pilot.PageCostPrivateV3{})
			if !errors.Is(err, pilot.ErrPagesPausedPrivateV3) || status.VerifiedReferences != 0 {
				t.Fatal("ASSERT_V3_PUBLICATION_PAUSE", status, err)
			}
			if _, err = pub.Advance(ctx, 1, grant); err == nil {
				t.Fatal("ASSERT_V3_PUBLICATION_STALE_GENERATION")
			}
		}
		generation := uint64(1)
		if i == 0 {
			generation = 2
		}
		publication, err := pub.Advance(ctx, generation, grant)
		id := publication.PreparedSHA256
		if err != nil || publication.VerifiedReferences != 3 || id != pilot.Digest(raw) || publication.CanonicalSHA256 != report.Source.DerivedCanonicalV5SHA256 || publication.AssessmentSHA256 != report.AssessmentSHA256 || refs[id] || inputs[report.SourceInputID] {
			t.Fatal("ASSERT_V3_IMMUTABLE_PUBLICATION", i, publication, err)
		}
		if !bytes.Equal(pilot.ReadControlledPagePairPrivateV3(t, owner, id, 1<<20), raw) || !bytes.Equal(pilot.ReadControlledPagePairPrivateV3(t, owner, publication.CanonicalSHA256, 1<<20), evidence.DerivedCanonicalV5) || !bytes.Equal(pilot.ReadControlledPagePairPrivateV3(t, owner, publication.AssessmentSHA256, 1<<20), report.Assessment) {
			t.Fatal("ASSERT_V3_VERIFIED_REFERENCES")
		}
		if _, err = pub.Advance(ctx, generation+1, grant); err == nil {
			t.Fatal("ASSERT_V3_PUBLICATION_TERMINAL_REPLAY")
		}
		refs[id] = true
		inputs[report.SourceInputID] = true
		prepared++
	}
	if prepared != 20 || len(refs) != 20 || len(inputs) != 20 {
		t.Fatal("ASSERT_V3_PIPELINE_TWENTY_OWN_BODIES", prepared)
	}
	var forged prep.PreparedPrivateV3
	if json.Unmarshal(firstBytes, &forged) != nil {
		t.Fatal("ASSERT_V3_DIAGNOSTIC_DECODE")
	}
	if _, err := forged.Publish(ctx, grant); err == nil {
		t.Fatal("ASSERT_V3_DTO_CANNOT_MINT_PREPARED")
	}
	u, err := url.Parse(firstSourceURI)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(u.Path, []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := owner.BudgetSpent()
	if _, err = firstPrepared.Read(ctx, grant); err == nil || before == owner.BudgetSpent() {
		t.Fatal("ASSERT_V3_PREPARED_CHANGED_SOURCE_CHARGED", err)
	}
	if _, err = firstPrepared.Publish(ctx, grant); err == nil {
		t.Fatal("ASSERT_V3_PUBLICATION_CHANGED_SOURCE")
	}
	// Earlier paired retained facts are not deleted by the later refusal.
	if !bytes.Equal(pilot.ReadControlledPagePairPrivateV3(t, owner, pilot.Digest(firstBytes), 1<<20), firstBytes) {
		t.Fatal("ASSERT_V3_PRIOR_PUBLICATION_SURVIVES")
	}
	t.Logf("CONTROLLED_STAGE_RESERVATIONS_NOT_TELEMETRY: grant=%+v cumulative=%+v", grant, owner.BudgetSpent())
	t.Log("ASSERT_V3_PIPELINE_TWENTY_OWN_BODIES: PASS; controlled, not live; 20 distinct batch inputs, one original source origin")
}
