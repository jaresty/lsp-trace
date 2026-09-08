package qualificationpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/qualificationmatrix"
)

func programBFixture(t *testing.T) (ProgramAAdmission, qualificationmatrix.Profile, qualificationmatrix.AdmissionRequest, ProgramBAdmissionBinding) {
	t.Helper()
	ctx := testAssessmentContext("assessment-b", "RANKING", "NORMATIVE_PROGRAM_B_ONLY")
	a, err := AdmitVerifiedProgramA(admittedProgramASubstrateWithSigner(t, newTestReceiptSigner(t), ctx))
	if err != nil {
		t.Fatal(err)
	}
	axes := []qualificationmatrix.Axis{}
	for _, x := range []struct{ n, v string }{{"relation_family", "CALLS"}, {"custody_adapter", "GIT"}, {"state", "COMPLETE"}, {"projection_class", "DIRECTED"}, {"transport", "CLI"}, {"publication_mode", "INLINE"}, {"provider_class", "TYPESCRIPT_LANGUAGE_SERVER"}, {"provider_version", "5.7.3"}, {"language", "TYPESCRIPT"}, {"framework", "NONE"}} {
		axes = append(axes, qualificationmatrix.Axis{Name: x.n, Members: []string{x.v}})
	}
	p := qualificationmatrix.Profile{SchemaVersion: qualificationmatrix.SchemaVersionV2, ProfileID: "analytics-profile", Version: "2", Authority: "release-council", CustodyReceiptID: "receipt", CustodyAuthenticationState: "AUTHENTICATED", Axes: axes, Products: []qualificationmatrix.Product{{ID: "analytics-provider", Version: "2", Axes: []string{"relation_family", "custody_adapter", "state", "projection_class", "transport", "publication_mode", "provider_class", "provider_version", "language", "framework"}}}}
	cells, err := qualificationmatrix.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	p.FoundationalCellIDs = []string{cells[0].ID}
	result := qualificationmatrix.Result{CellID: cells[0].ID, Status: qualificationmatrix.StatusPass, RealServerEvidence: true, EvidenceProviderClass: "TYPESCRIPT_LANGUAGE_SERVER", EvidenceProviderVersion: "5.7.3"}
	matrix, err := qualificationmatrix.CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	binding := ProgramBAdmissionBinding{BuildRevision: a.Revision, Operation: "RANKING", Scope: "NORMATIVE_PROGRAM_B_ONLY", SubstrateID: a.SubstrateID, MatrixDigest: digestBytes(matrix), EvidenceSetDigest: programAEvidenceSetDigest(a.receiptDigests), DecisionPolicy: ProgramBDecisionPolicy}
	return a, p, qualificationmatrix.AdmissionRequest{Results: []qualificationmatrix.Result{result}, RequestedOperations: []string{"RANKING"}}, binding
}

func TestProgramBAdmissionRejectsReceiptDigestSwapAndSubstitution(t *testing.T) {
	a, p, req, binding := programBFixture(t)
	if _, err := VerifyProgramBAdmission(a, p, req, binding, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ASSERT_PROGRAM_B_EXACT_EVIDENCE_ACCEPTED: %v", err)
	}
	t.Run("swap", func(t *testing.T) {
		swapped := a
		swapped.receiptDigests[0], swapped.receiptDigests[1] = swapped.receiptDigests[1], swapped.receiptDigests[0]
		if _, err := VerifyProgramBAdmission(swapped, p, req, binding, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)); err == nil {
			t.Fatal("ASSERT_PROGRAM_B_RECEIPT_DIGEST_SWAP_REJECTED")
		}
	})
	t.Run("substitution", func(t *testing.T) {
		substituted := a
		sum := sha256.Sum256([]byte("substituted"))
		substituted.receiptDigests[2] = "sha256:" + hex.EncodeToString(sum[:])
		if _, err := VerifyProgramBAdmission(substituted, p, req, binding, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)); err == nil {
			t.Fatal("ASSERT_PROGRAM_B_RECEIPT_SUBSTITUTION_REJECTED")
		}
	})
}

func TestProgramBAdmissionRejectsContextMatrixAndEvidenceMutations(t *testing.T) {
	a, p, req, binding := programBFixture(t)
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	for name, mutate := range map[string]func(*ProgramBAdmissionBinding){
		"revision":  func(b *ProgramBAdmissionBinding) { b.BuildRevision = "other" },
		"operation": func(b *ProgramBAdmissionBinding) { b.Operation = "ANALYSIS" },
		"scope":     func(b *ProgramBAdmissionBinding) { b.Scope = "OTHER" },
		"substrate": func(b *ProgramBAdmissionBinding) { b.SubstrateID = "other" },
		"matrix":    func(b *ProgramBAdmissionBinding) { b.MatrixDigest = digestBytes([]byte("other")) },
		"evidence":  func(b *ProgramBAdmissionBinding) { b.EvidenceSetDigest = digestBytes([]byte("other")) },
		"policy":    func(b *ProgramBAdmissionBinding) { b.DecisionPolicy = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := binding
			mutate(&changed)
			if _, err := VerifyProgramBAdmission(a, p, req, changed, now); err == nil {
				t.Fatalf("ASSERT_PROGRAM_B_%s_BINDING_REJECTED", strings.ToUpper(name))
			}
		})
	}
	v1 := p
	v1.SchemaVersion = qualificationmatrix.SchemaVersion
	if _, err := VerifyProgramBAdmission(a, v1, req, binding, now); err == nil {
		t.Fatal("ASSERT_PROGRAM_B_V1_MATRIX_REJECTED")
	}
	duplicate := a
	duplicate.receiptDigests[1] = duplicate.receiptDigests[0]
	binding.EvidenceSetDigest = programAEvidenceSetDigest(duplicate.receiptDigests)
	if _, err := VerifyProgramBAdmission(duplicate, p, req, binding, now); err == nil {
		t.Fatal("ASSERT_PROGRAM_B_DUPLICATE_RECEIPT_REJECTED")
	}
	omitted := a
	omitted.receiptDigests[5] = ""
	binding.EvidenceSetDigest = programAEvidenceSetDigest(omitted.receiptDigests)
	if _, err := VerifyProgramBAdmission(omitted, p, req, binding, now); err == nil {
		t.Fatal("ASSERT_PROGRAM_B_OMITTED_RECEIPT_REJECTED")
	}
}

func TestProgramBExecutionChecksEveryExpectedBinding(t *testing.T) {
	a, p, req, binding := programBFixture(t)
	admission, err := VerifyProgramBAdmission(a, p, req, binding, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	expected := ProgramBExecutionExpectation{BuildRevision: binding.BuildRevision, Operation: binding.Operation, Scope: binding.Scope, SubstrateID: binding.SubstrateID, MatrixDigest: binding.MatrixDigest, EvidenceSetDigest: binding.EvidenceSetDigest}
	if err := admission.VerifyExecution(expected); err != nil {
		t.Fatalf("ASSERT_PROGRAM_B_EXECUTION_EXACT_BINDING_ACCEPTED: %v", err)
	}
	for name, mutate := range map[string]func(*ProgramBExecutionExpectation){"revision": func(e *ProgramBExecutionExpectation) { e.BuildRevision = "other" }, "operation": func(e *ProgramBExecutionExpectation) { e.Operation = "other" }, "scope": func(e *ProgramBExecutionExpectation) { e.Scope = "other" }, "substrate": func(e *ProgramBExecutionExpectation) { e.SubstrateID = "other" }, "matrix": func(e *ProgramBExecutionExpectation) { e.MatrixDigest = digestBytes([]byte("other")) }, "evidence": func(e *ProgramBExecutionExpectation) { e.EvidenceSetDigest = digestBytes([]byte("other")) }} {
		t.Run(name, func(t *testing.T) {
			changed := expected
			mutate(&changed)
			if err := admission.VerifyExecution(changed); err == nil {
				t.Fatalf("ASSERT_PROGRAM_B_EXECUTION_%s_REJECTED", strings.ToUpper(name))
			}
		})
	}
}

func TestQualificationPolicyExportsNoProgramAReceiptInspectionOrMinting(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.IsExported() && strings.Contains(fn.Name.Name, "ProgramABinding") {
				t.Fatalf("ASSERT_EXTERNAL_CALLER_CANNOT_INSPECT_PROGRAM_A: exported %s", fn.Name.Name)
			}
			if ts, ok := d.(*ast.GenDecl); ok {
				for _, s := range ts.Specs {
					if typ, ok := s.(*ast.TypeSpec); ok && typ.Name.Name == "ProgramABinding" {
						t.Fatal("ASSERT_EXTERNAL_CALLER_CANNOT_INSPECT_PROGRAM_A: ProgramABinding exported")
					}
				}
			}
		}
	}
}
