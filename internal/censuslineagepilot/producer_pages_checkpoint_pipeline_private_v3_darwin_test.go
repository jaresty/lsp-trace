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
	"os/exec"
	"path/filepath"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

func TestRetainedCheckpointPrivateV3PipelineTwoProcesses(t *testing.T) {
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	transfer := filepath.Join(dir, "transfer.json")
	for _, role := range []string{"A", "B"} {
		cmd := exec.Command(exe, "-test.run=^TestRetainedCheckpointPrivateV3PipelineChild$", "-test.v")
		cmd.Env = []string{"LSP_RETAINED_PIPELINE_PROCESS=" + role, "LSP_RETAINED_PIPELINE_TRANSFER=" + transfer, "TMPDIR=" + dir, "HOME=" + dir}
		raw, e := cmd.CombinedOutput()
		t.Logf("CONTROLLED_PIPELINE_PROCESS_%s:\n%s", role, raw)
		if e != nil {
			t.Fatalf("ASSERT_RETAINED_TWO_PROCESS_COMPUTED_AST_PUBLICATION: %s %v", role, e)
		}
	}
	t.Log("ASSERT_RETAINED_TWO_PROCESS_COMPUTED_AST_PUBLICATION: PASS")
}
func TestRetainedCheckpointPrivateV3PipelineChild(t *testing.T) {
	role := os.Getenv("LSP_RETAINED_PIPELINE_PROCESS")
	if role == "" {
		t.Skip("controlled child only")
	}
	path := os.Getenv("LSP_RETAINED_PIPELINE_TRANSFER")
	if role == "A" {
		pilot.ControlledRetainedCheckpointExportPrivateV3(t, path)
		fmt.Println("ASSERT_RETAINED_ORIGINAL_PROCESS_EXIT: PASS")
		os.Exit(0)
	}
	if role != "B" {
		t.Fatal("unknown controlled role")
	}
	ctx := context.Background()
	grant := pipelineGrantV3()
	owner, expected := pilot.ControlledRetainedCheckpointOpenPrivateV3(t, path)
	var firstBytes []byte
	var firstRef string
	var firstAssessment derived.AssessmentRetainedCheckpointPrivateV3
	var lastPrepared prep.PreparedRetainedCheckpointPrivateV3
	var sourceURI string
	beforeResume := pilot.PageCostPrivateV3{}
	for i := 0; i < 20; i++ {
		var b pilot.RetainedSelectedBatchPrivateV3
		var e error
		if i == 0 {
			reader, err := owner.SelectedBatchReader(ctx, 0, grant)
			if err != nil {
				t.Fatal(err)
			}
			progress, empty, err := reader.Advance(ctx, 1, 1, grant)
			if err != nil || progress.VerifiedPages != 2 || progress.Status != "PAUSED_PAGE_BOUND" {
				t.Fatal("ASSERT_RETAINED_CROSS_PROCESS_SELECTED_FORWARD_PROGRESS", progress, err)
			}
			if _, err = empty.Read(ctx, grant); err == nil {
				t.Fatal("ASSERT_RETAINED_PAUSED_SELECTION_NOT_COMPLETE")
			}
			_, b, e = reader.Advance(ctx, 2, ^uint64(0), grant)
		} else {
			b, e = owner.SelectedBatch(ctx, i, grant)
		}
		if e != nil {
			t.Fatal("ASSERT_RETAINED_PIPELINE_SELECTED", i, e)
		}
		view, e := b.Read(ctx, grant)
		if e != nil || view.ObjectSHA256 != expected[i] || view.Profile != pilot.RetainedCheckpointProfilePrivateV3 || view.FullTargetCount != 20 {
			t.Fatal("ASSERT_RETAINED_PIPELINE_PROFILE", i, view.FullTargetCount, e)
		}
		p, e := derived.ProjectRetainedCheckpointPrivateV3(ctx, b, grant)
		if e != nil {
			t.Fatal("ASSERT_RETAINED_PIPELINE_PROJECT", i, e)
		}
		v, e := p.Read(ctx, grant)
		if e != nil || v.Source.InputFamily != pilot.RetainedCheckpointProfilePrivateV3 || v.Source.Admission != pilot.RetainedCheckpointProfilePrivateV3 || !bytes.Equal(v.OriginalNativeV3, view.Source.Evidence.GraphBytes) || !bytes.Contains(v.OriginalNativeV3, []byte("1.000e+2")) {
			t.Fatal("ASSERT_RETAINED_DISTINCT_SOURCE_BINDING", i, e)
		}
		sites := 0
		for _, edge := range view.Source.Evidence.Acquisition.Graph.Edges {
			sites += len(edge.CallSites)
		}
		if len(v.Math.Occurrences) != sites {
			t.Fatal("ASSERT_RETAINED_DUPLICATE_CALL_OCCURRENCES", i)
		}
		a, e := derived.ComputeInstabilityRetainedCheckpointPrivateV3(ctx, p, []uint64{3, 7}, grant)
		if e != nil {
			t.Fatal("ASSERT_RETAINED_PIPELINE_MATH", i, e)
		}
		ar, _, e := derived.ValidateInstabilityRetainedCheckpointPrivateV3(ctx, p, a, grant)
		if e != nil || ar.Math.IndependentSources != 1 || ar.Source.Admission != pilot.RetainedCheckpointProfilePrivateV3 {
			t.Fatal("ASSERT_RETAINED_PIPELINE_ASSESSMENT", i, e)
		}
		for _, r := range ar.Math.Runs {
			if r.Partition.Algorithm != "TRIVIAL_SINGLE_NODE_PRIVATE_V3" {
				t.Fatal("ASSERT_RETAINED_TRIVIAL_NOT_LEIDEN")
			}
		}
		limits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
		if i == 0 {
			small := limits
			small.MaxPartBytes = 2048
			if _, e = prep.NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, a, small, grant); !errors.Is(e, pilot.ErrPagesUnitPrivateV3) {
				t.Fatal("ASSERT_RETAINED_OWNED_HEADER_CAP_REFUSAL", e)
			}
			t.Log("ASSERT_RETAINED_OWNED_HEADER_CAP_REFUSAL: PASS")
		}
		if i == 1 {
			if _, e = prep.NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, firstAssessment, limits, grant); e == nil {
				t.Fatal("ASSERT_RETAINED_FOREIGN_ASSESSMENT")
			}
		}
		reader, e := prep.NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, a, limits, grant)
		if e != nil {
			t.Fatal("ASSERT_RETAINED_PIPELINE_AST_CONSTRUCTOR", i, e)
		}
		gen := uint64(1)
		if i == 0 {
			firstAssessment = a
			progress, empty, e := reader.Advance(ctx, gen, pilot.PageCostPrivateV3{})
			if !errors.Is(e, pilot.ErrPagesPausedPrivateV3) || progress.Prepared != 0 {
				t.Fatal("ASSERT_RETAINED_AST_PAUSE", e)
			}
			if _, e = empty.Read(ctx, grant); e == nil {
				t.Fatal("ASSERT_RETAINED_PARTIAL_HAS_NO_BODY")
			}
			gen++
			dead, cancel := context.WithCancel(ctx)
			cancel()
			if _, _, e = reader.Advance(dead, gen, grant); !errors.Is(e, context.Canceled) {
				t.Fatal("ASSERT_RETAINED_AST_CANCEL", e)
			}
			gen++
		}
		_, own, e := reader.Advance(ctx, gen, grant)
		if e != nil {
			t.Fatal("ASSERT_RETAINED_TWENTY_COMPUTED_OWN_BODIES", i, e)
		}
		lastPrepared = own
		raw, e := own.Read(ctx, grant)
		if e != nil {
			t.Fatal(e)
		}
		var report prep.OwnASTReportRetainedCheckpointPrivateV3
		if json.Unmarshal(raw, &report) != nil || report.RequestType != prep.RequestTypeRetainedCheckpointPrivateV3 || report.PreparedCount != 1 || len(report.Bodies) != 1 || report.Bodies[0].Declaration.Body != fmt.Sprintf("func Owned%02d() {}", i) || report.FullTargetCount != 20 || report.FullBatchCount != 20 || report.Source.Admission != pilot.RetainedCheckpointProfilePrivateV3 || report.Source.CheckpointSHA256 != view.CheckpointSHA256 || report.Authority != 0 || report.Accepted || report.SourceCompleteness != "UNKNOWN" || report.FeatureIdentity != "UNRESOLVED" || report.AbsenceInference != "PROHIBITED" {
			t.Fatal("ASSERT_RETAINED_TWENTY_COMPUTED_OWN_BODIES", i, string(raw))
		}
		sourceURI = report.Bodies[0].SourceURI
		pub, e := own.Publication(ctx, grant)
		if e != nil {
			t.Fatal(e)
		}
		pubgen := uint64(1)
		if i == 0 {
			pr, e := pub.Advance(ctx, 1, pilot.PageCostPrivateV3{})
			if !errors.Is(e, pilot.ErrPagesPausedPrivateV3) || pr.VerifiedReferences != 0 {
				t.Fatal("ASSERT_RETAINED_PUBLICATION_PAUSE", e)
			}
			pubgen++
		}
		refs, e := pub.Advance(ctx, pubgen, grant)
		if e != nil || refs.VerifiedReferences != 3 || refs.PreparedSHA256 != pilot.Digest(raw) || refs.CanonicalSHA256 != v.Source.DerivedCanonicalV5SHA256 || refs.AssessmentSHA256 != report.AssessmentSHA256 {
			t.Fatal("ASSERT_RETAINED_PAIRED_PUBLICATION", i, e)
		}
		for j, id := range []string{refs.CanonicalSHA256, refs.AssessmentSHA256, refs.PreparedSHA256} {
			got, e := owner.ReadOutput(ctx, id, 1<<20, grant)
			want := [][]byte{v.DerivedCanonicalV5, report.Assessment, raw}[j]
			if e != nil || !bytes.Equal(got, want) {
				t.Fatal("ASSERT_RETAINED_EXACT_PUBLICATION_REFERENCES", i, j, e)
			}
		}
		if i == 0 {
			firstBytes = bytes.Clone(raw)
			firstRef = refs.PreparedSHA256
			var old prep.PreparedPrivateV3
			if json.Unmarshal(raw, &old) != nil {
				t.Fatal("DTO decode")
			}
			if _, e = old.Read(ctx, grant); e == nil {
				t.Fatal("ASSERT_RETAINED_CANNOT_MINT_ORIGINAL_PREPARED")
			}
		}
		if e = owner.RecordPublishedReferences(ctx, b, [3]string{refs.CanonicalSHA256, refs.AssessmentSHA256, refs.PreparedSHA256}, grant); e != nil {
			t.Fatal(e)
		}
		next, e := owner.CheckpointCompletedBatch(ctx, b, pilot.Digest([]byte(fmt.Sprintf("pipeline-next-%d", i))), grant)
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			beforeResume = owner.BudgetSpent()
			pilot.ControlledReplaceRetainedCheckpointManifestPrivateV3(t, path, next, owner)
			owner, _ = pilot.ControlledRetainedCheckpointOpenPrivateV3(t, path)
			after := owner.BudgetSpent()
			if after.LogicalBytes < beforeResume.LogicalBytes || after.ReadHashBytes < beforeResume.ReadHashBytes || after.WriteBytes < beforeResume.WriteBytes || after.Work < beforeResume.Work {
				t.Fatal("ASSERT_RETAINED_SEMANTIC_REOPEN_NO_REFUND")
			}
		}
	}
	got, e := owner.ReadOutput(ctx, firstRef, 1<<20, grant)
	if e != nil || !bytes.Equal(got, firstBytes) {
		t.Fatal("ASSERT_RETAINED_PRIOR_PREPARED_REFERENCE_IMMUTABLE", e)
	}
	u, e := url.Parse(sourceURI)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(u.Path, []byte("package changed\n"), 0600); e != nil {
		t.Fatal(e)
	}
	beforeFailure := owner.BudgetSpent()
	if _, e = lastPrepared.Read(ctx, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_LATER_SOURCE_CHANGE_REFUSED")
	}
	afterFailure := owner.BudgetSpent()
	if beforeFailure == afterFailure || afterFailure.LogicalBytes < beforeFailure.LogicalBytes || afterFailure.ReadHashBytes < beforeFailure.ReadHashBytes {
		t.Fatal("ASSERT_RETAINED_LATER_SOURCE_FAILURE_CHARGED")
	}
	if !bytes.Equal(pilot.ControlledReadRetainedReferenceAfterFailurePrivateV3(t, owner, firstRef, grant), firstBytes) {
		t.Fatal("ASSERT_RETAINED_PRIOR_PAIRED_BYTES_SURVIVE_LATER_FAILURE")
	}
	t.Logf("ASSERT_RETAINED_TWENTY_COMPUTED_OWN_BODIES: PASS; no original capture handles in B; grant=%+v cumulative=%+v", grant, owner.BudgetSpent())
}
