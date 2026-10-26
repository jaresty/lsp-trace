package inventoryworkflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgramCQualificationSubjectPrivateCompositionSurface(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if os.IsNotExist(err) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	workflow := read("internal/inventoryworkflow/qualification_subject.go")
	if !strings.Contains(workflow, "BuildProgramCRepresentativeQualificationSubject") ||
		!strings.Contains(workflow, "subject = programcrepresentative.ProgramCRepresentativeQualificationSubject{}") {
		t.Fatal("ASSERT_PROGRAM_C_QUALIFICATION_SUBJECT_PRIVATE_COMPOSITION_SURFACE")
	}
	predecessor := read("internal/inventorysuccessor/coordinator.go")
	if !strings.Contains(predecessor, "VerifyWaitingEvidenceHead") {
		t.Fatal("ASSERT_PREDECESSOR_WAITING_EVIDENCE_READ_ONLY_VERIFIER_SURFACE")
	}
	mapper := read("internal/censuscontinuation/programc_qualification_subject.go")
	if !strings.Contains(mapper, "verifyProgramCQualificationAuthority") || strings.Contains(mapper, "func preflightProgramCRepresentativeCorpus") {
		t.Fatal("ASSERT_COMPLETE_CORPUS_PREFLIGHT_DURING_RECONSTRUCTION_SURFACE")
	}
	if strings.Contains(mapper, "ProgramCRepresentativeWorkflowAuthority") || strings.Contains(mapper, "NewProgramCRepresentativeQualificationSubject") || !strings.Contains(mapper, "BuildProgramCRepresentativeQualificationEvidence") {
		t.Fatal("ASSERT_CENSUS_QUALIFICATION_EVIDENCE_ONLY_SURFACE")
	}
	ordered := []string{"VerifyWaitingEvidenceHead", "BuildProgramCRepresentativeQualificationEvidence", "qualificationWorkflowIdentity", "programcrepresentative.NewProgramCRepresentativeQualificationSubject"}
	previous := -1
	for _, symbol := range ordered {
		current := strings.Index(workflow, symbol)
		if current <= previous {
			t.Fatalf("ASSERT_PROGRAM_C_QUALIFICATION_COMPOSITION_ORDER symbol=%q", symbol)
		}
		previous = current
	}
	pure := read("internal/programcrepresentative/qualification_subject.go")
	if !strings.Contains(pure, "BuildAdapterContract") || !strings.Contains(pure, "BuildStage6Implementation") {
		t.Fatal("ASSERT_PURE_SUBJECT_BUILDERS_SURFACE")
	}
	for _, forbidden := range []string{"denominator", "lineage", "successor v2"} {
		if strings.Contains(mapper, forbidden) || strings.Contains(workflow, forbidden) {
			t.Fatalf("ASSERT_NO_PUBLIC_OR_LATER_STAGE_SUBJECT_SURFACE forbidden=%q", forbidden)
		}
	}
	for _, forbidden := range []string{"\"lsp-trace/internal/mcp", "\"lsp-trace/internal/inventorysuccessor\"", "\"lsp-trace/internal/censuscontinuation\""} {
		if strings.Contains(pure, forbidden) {
			t.Fatalf("ASSERT_NO_PUBLIC_OR_LATER_STAGE_SUBJECT_SURFACE forbidden=%q", forbidden)
		}
	}
}
