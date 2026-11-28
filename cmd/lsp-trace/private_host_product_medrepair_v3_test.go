//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	product "lsp-trace/internal/privatehostproductv3"
)

// None_Model; direct retained-core pause, actual CLI resume admission, new allowance.
func TestHostProductCLIResumeAdmissionFailureQualified(t *testing.T) {
	for _, mode := range []string{"source", "corrupt_ledger", "stale_tip", "bootstrap", "transfer", "runtime_grant"} {
		t.Run(mode, func(t *testing.T) { hostProductResumeRefusalFixture(t, mode) })
	}
}

func TestHostProductCLIPriorPublicationAdmission(t *testing.T) {
	for _, mode := range []string{"partial", "earlypartial", "machine_mismatch", "human_mismatch", "outside_paths", "counts", "candidate", "ledger", "profile", "lifecycle", "noncanonical", "complete", "checkpoint", "reservation", "config", "status", "gap", "ledger_tip", "citation_overflow"} {
		t.Run(mode, func(t *testing.T) { hostProductResumeRefusalFixture(t, "prior_"+mode) })
	}
}

func hostProductResumeRefusalFixture(t *testing.T, mode string) {
	r, path, base := hostProductCLIRequestFixture(t)
	r.Configuration.MaxCalls = 2
	r.Configuration.ApprovalSHA256 = product.ConfigDigest(r.Configuration)
	r.RuntimeGrantSHA256 = hostProductRequestDigestV3(r)
	raw, _ := json.Marshal(r)
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	tool := filepath.Join(base, "lsp-trace")
	if raw, e := exec.Command("go", "build", "-o", tool, ".").CombinedOutput(); e != nil {
		t.Fatalf("build: %s %v", raw, e)
	}
	first, e := runHostProductFixtureCommand(t, tool, "retained-core", path, r)
	if e == nil || first.Status != "PAUSED_BUDGET" || first.Completed != 1 {
		t.Fatalf("pause prerequisite: %+v %v", first, e)
	}
	expectedPrior := first
	prior := map[string][]byte{}
	for _, paths := range [][2]string{first.MachinePaths, first.HumanPaths} {
		for _, p := range paths {
			b, e := os.ReadFile(filepath.Join(r.Configuration.Store, p))
			if e != nil {
				t.Fatal(e)
			}
			prior[p] = b
		}
	}
	claimPath := filepath.Join(r.Configuration.Store, "claim/claim-00000000.json")
	claim, e := os.ReadFile(claimPath)
	if e != nil {
		t.Fatal(e)
	}
	old := r.Configuration
	r.Configuration.StartBatch = first.NextBatch
	r.Configuration.AllowanceRevision++
	r.Configuration.PreviousConfigSHA256 = product.ConfigDigest(old)
	r.Configuration.MaxCalls = 40
	r.Configuration.ApprovalSHA256 = product.ConfigDigest(r.Configuration)
	if mode == "bootstrap" {
		r.BootstrapAllowance.LogicalBytes = 1
		r.BootstrapAllowance.ReadHashBytes = 1
		r.BootstrapAllowance.WriteBytes = 1
		r.BootstrapAllowance.Work = 1
	}
	r.RuntimeGrantSHA256 = hostProductRequestDigestV3(r)
	if mode == "runtime_grant" {
		r.RuntimeGrantSHA256 = "refused"
	}
	path = filepath.Join(base, "resume.json")
	raw, _ = json.Marshal(r)
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	sourcePath := filepath.Join(base, "source", "owned.go")
	source, e := os.ReadFile(sourcePath)
	if e != nil {
		t.Fatal(e)
	}
	if len(mode) > 6 && mode[:6] == "prior_" {
		fake := first
		fake.ReportRevision = 2
		fake.MachinePaths = [2]string{"result/report-00000003.json", "result/report-00000004.json"}
		fake.HumanPaths = [2]string{"result/inventory-00000003.md", "result/inventory-00000004.md"}
		switch mode {
		case "prior_citation_overflow":
			fake.Candidates = append([]product.Candidate(nil), first.Candidates...)
			fake.Candidates[0].QuoteStart = int(^uint(0) >> 1)
			fake.Candidates[0].QuoteEnd = -fake.Candidates[0].QuoteStart - 1
			fake.Candidates[0].Quote = "x"
			// Add a matching canonical progress successor, retaining the genuine
			// predecessor. Admission must reject before physical response lookup.
			pages, err := filepath.Glob(filepath.Join(r.Configuration.Store, "pages/state-*.json"))
			if err != nil || len(pages) == 0 {
				t.Fatal("ASSERT_OVERFLOW_PROGRESS_PREREQUISITE", err)
			}
			body, err := os.ReadFile(pages[len(pages)-1])
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(first.Candidates)
			after, _ := json.Marshal(fake.Candidates)
			if !bytes.Contains(body, before) {
				t.Fatal("ASSERT_OVERFLOW_PROGRESS_CANDIDATE")
			}
			body = bytes.Replace(body, before, after, 1)
			if err = os.WriteFile(filepath.Join(r.Configuration.Store, fmt.Sprintf("pages/state-%08d.json", len(pages)+1)), body, 0600); err != nil {
				t.Fatal(err)
			}
		case "prior_outside_paths":
			fake.MachinePaths[1] = "/outside/report.json"
		case "prior_counts":
			fake.Completed++
		case "prior_candidate":
			fake.Candidates = append([]product.Candidate(nil), first.Candidates...)
			fake.Candidates[0].WorkingLabel = "fabricated owner"
		case "prior_ledger":
			fake.LedgerGeneration = 0
		case "prior_profile":
			fake.Profile = "FOREIGN"
		case "prior_lifecycle":
			fake.Lifecycle.OriginalStopCalls = -1
		case "prior_checkpoint":
			fake.CheckpointSHA256 = "sha256:" + string(bytes.Repeat([]byte("0"), 64))
		case "prior_reservation":
			fake.Spend.PolicyReservation = old.Exchange
			fake.Spend.PolicyReservation.Work = 0
		case "prior_config":
			fake.ConfigSHA256 = "sha256:" + string(bytes.Repeat([]byte("0"), 64))
		case "prior_status":
			fake.Status = "PROVISIONAL"
		case "prior_gap":
			fake.Gaps = 1
			fake.Pending--
			fake.TargetGaps = []product.TargetGap{{Target: "fabricated", Phase: "HOST_SEMANTIC_EXCHANGE", Class: "FAILED"}}
		case "prior_ledger_tip":
			fake.LedgerTip = "invalid"
		}
		machine, _ := json.Marshal(fake)
		if mode == "prior_noncanonical" {
			machine = append([]byte(" "), machine...)
		}
		humanBytes := prior[first.HumanPaths[0]]
		if mode == "prior_counts" {
			humanBytes = bytes.ReplaceAll(humanBytes, []byte("20 requested; 1 candidates;"), []byte("20 requested; 2 candidates;"))
		}
		if mode == "prior_candidate" {
			humanBytes = bytes.ReplaceAll(humanBytes, []byte(first.Candidates[0].WorkingLabel), []byte(fake.Candidates[0].WorkingLabel))
		}
		if mode == "prior_checkpoint" {
			humanBytes = bytes.ReplaceAll(humanBytes, []byte(first.CheckpointSHA256), []byte(fake.CheckpointSHA256))
		}
		if mode == "prior_status" {
			humanBytes = bytes.ReplaceAll(humanBytes, []byte("inventory — PAUSED_BUDGET"), []byte("inventory — PROVISIONAL"))
		}
		if mode == "prior_gap" {
			humanBytes = bytes.ReplaceAll(humanBytes, []byte("1 candidates; 0 gaps; 19 pending"), []byte("1 candidates; 1 gaps; 18 pending"))
		}
		if mode == "prior_complete" {
			expectedPrior = fake
		}
		// Human views do not advertise their own paths; unchanged normal-report
		// fields regenerate identical bytes even in the higher namespace.
		for i, p := range []string{"result/report-00000003.json", "result/report-00000004.json", "result/inventory-00000003.md", "result/inventory-00000004.md"} {
			if (mode == "prior_partial" || mode == "prior_earlypartial") && i > 0 {
				break
			}
			b := machine
			if i > 1 {
				b = humanBytes
			}
			if mode == "prior_machine_mismatch" && i == 1 {
				b = []byte("{}")
			}
			if mode == "prior_human_mismatch" && i == 3 {
				b = []byte("different human")
			}
			if e = os.WriteFile(filepath.Join(r.Configuration.Store, p), b, 0600); e != nil {
				t.Fatal(e)
			}
			prior[p] = append([]byte(nil), b...)
		}
		if e = os.WriteFile(sourcePath, append(source, []byte("// refused current source\n")...), 0600); e != nil {
			t.Fatal(e)
		}
	}
	switch mode {
	case "source":
		if e = os.WriteFile(sourcePath, append(source, []byte("// changed pinned source\n")...), 0600); e != nil {
			t.Fatal(e)
		}
	case "corrupt_ledger":
		if e = os.WriteFile(filepath.Join(r.AuxiliaryRoot, "ledger", fmt.Sprintf("reservation-%020d.json", first.LedgerGeneration)), []byte("{}"), 0600); e != nil {
			t.Fatal(e)
		}
	case "stale_tip":
		if e = os.WriteFile(filepath.Join(r.AuxiliaryRoot, "ledger", fmt.Sprintf("reservation-%020d.json", first.LedgerGeneration+1)), []byte("{}"), 0600); e != nil {
			t.Fatal(e)
		}
	case "transfer":
		if e = os.Remove(filepath.Join(r.AuxiliaryRoot, "transfer.json")); e != nil {
			t.Fatal(e)
		}
	}
	var final product.Report
	if mode == "prior_earlypartial" {
		final, e = product.DeliverEarlyFailure(r.Configuration, "CONTROLLED_EARLY_FAILURE", "RETAINED_ADMISSION_REFUSED")
	} else {
		final, e = runHostProductFixtureCommand(t, tool, "resume", path, r)
	}
	if (e == nil && mode != "prior_earlypartial") || (e != nil && mode == "prior_earlypartial") || final.Profile != product.Profile || final.Status != "FAILED" || final.Failure == nil || final.Completed != first.Completed || final.Pending != first.Pending || final.Gaps != first.Gaps || !reflect.DeepEqual(final.Candidates, first.Candidates) {
		t.Fatalf("ASSERT_RESUME_ADMISSION_QUALIFIED_RETAINED_FAILURE: profile=%q status=%q completed=%d pending=%d failure=%+v err=%v", final.Profile, final.Status, final.Completed, final.Pending, final.Failure, e)
	}
	if final.Accepted || final.Authority != 0 || final.SourceCompleteness != "UNKNOWN" || final.FeatureIdentity != "UNRESOLVED" || final.AbsenceInference != "PROHIBITED" {
		t.Fatal("ASSERT_RESUME_NO_AUTHORITY_PROMOTION")
	}
	for p, before := range prior {
		after, e := os.ReadFile(filepath.Join(r.Configuration.Store, p))
		if e != nil || !bytes.Equal(before, after) {
			t.Fatal("ASSERT_RESUME_PRIOR_VIEWS_IMMUTABLE")
		}
	}
	after, e := os.ReadFile(claimPath)
	if e != nil || !bytes.Equal(claim, after) {
		t.Fatal("ASSERT_RESUME_NO_ORIGINAL_REPLAY")
	}
	for _, paths := range [][2]string{final.MachinePaths, final.HumanPaths} {
		if paths[0] == "" || paths[0] == first.MachinePaths[0] || paths[0] == first.HumanPaths[0] {
			t.Fatal("ASSERT_RESUME_NEW_IMMUTABLE_PAIR")
		}
		a, e := os.ReadFile(filepath.Join(r.Configuration.Store, paths[0]))
		if e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile(filepath.Join(r.Configuration.Store, paths[1]))
		if e != nil || !bytes.Equal(a, b) {
			t.Fatal("ASSERT_RESUME_NEW_IMMUTABLE_PAIR")
		}
	}
	if final.LedgerTip != first.LedgerTip || final.LedgerGeneration != first.LedgerGeneration || final.Spend.Calls != first.Spend.Calls {
		t.Fatal("ASSERT_RESUME_PRIOR_LEDGER_OBSERVATION_NOT_RECOMPUTED")
	}
	// Inspect additive qualification via JSON so this regression compiles before repair.
	raw, _ = json.Marshal(final)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	var admission struct {
		CurrentValidation, LedgerValidation, SourceValidation, Coverage string
		PriorMachinePaths, PriorHumanPaths                              [2]string
	}
	if json.Unmarshal(fields["Admission"], &admission) != nil || admission.CurrentValidation != "FAILED" || admission.LedgerValidation != "UNKNOWN" || admission.SourceValidation != "UNKNOWN" || admission.Coverage != "PREVIOUS_REPORT_OBSERVATION" || admission.PriorMachinePaths != expectedPrior.MachinePaths || admission.PriorHumanPaths != expectedPrior.HumanPaths {
		t.Fatal("ASSERT_RESUME_PREVIOUS_NOT_CURRENT_VALIDATION")
	}
	if len(mode) > 6 && mode[:6] == "prior_" {
		var qualification struct{ PriorPublicationValidation string }
		if json.Unmarshal(fields["Admission"], &qualification) != nil || qualification.PriorPublicationValidation != map[bool]string{true: "LOCAL_CONSISTENCY_ONLY", false: "UNKNOWN"}[mode == "prior_complete"] {
			t.Fatal("ASSERT_INCOMPLETE_LATER_PUBLICATION_EXPLICIT_UNKNOWN")
		}
		for _, paths := range [][2]string{admission.PriorMachinePaths, admission.PriorHumanPaths} {
			a, err := os.ReadFile(filepath.Join(r.Configuration.Store, paths[0]))
			b, err2 := os.ReadFile(filepath.Join(r.Configuration.Store, paths[1]))
			if err != nil || err2 != nil || !bytes.Equal(a, b) {
				t.Fatal("ASSERT_ADMITTED_PRIOR_PATHS_EXIST_EXACT")
			}
		}
	}
	t.Log("ASSERT_RESUME_ADMISSION_QUALIFIED_RETAINED_FAILURE: PASS; controlled None_Model, no fresh source ownership")
}
