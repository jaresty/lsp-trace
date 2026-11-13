//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	ai "lsp-trace/internal/applicationinventory"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplicationInventoryV3ControllerCrashRecovery(t *testing.T) {
	t.Run("phase32-publication-binding", inventoryPhase32PublicationBinding)
	t.Run("fail-entry-ignores-changed-ready-cost-only", func(t *testing.T) {
		s := inventoryTestSetup(t)
		j, tr := inventoryTestOpenTransport(t, s)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- tr.Exchange("Describe")(ctx, inventoryTestDescribe(), &bytes.Buffer{}) }()
		inventoryTestAwaitDescribe(t, s)
		if n, _, e := inventoryTestEntry([]string{"respond", s.path, s.runID, "interpreter", "1"}, []byte("{bad")); n != 0 {
			t.Fatal(e)
		}
		if <-done == nil {
			t.Fatal("malformed accepted")
		}
		before := j.Head()
		tr.Close()
		j.Close()
		// External READY replacement is not new semantic admission on fail/recovery.
		ready := product.Ready{Protocol: product.InventoryProtocolV3, Role: "interpreter", LaunchID: "changed-ready", PromptSHA256: s.q.Configuration.RolePromptSHA256[0]}
		inventoryTestWrite(t, filepath.Join(product.InventoryMailboxPathV3(s.q.Configuration.Store, s.runID), "ready-interpreter.json"), ready)
		n, raw, e := inventoryTestEntry([]string{"fail", s.path, s.runID}, nil)
		if n != 1 || !bytes.Contains(raw, []byte("COST_ONLY_NO_REPLAY")) {
			t.Fatal("ASSERT_V3_FAIL_ENTRY_COST_ONLY", n, string(raw), e)
		}
		reopened, er := product.OpenApplication(s.q.Configuration, time.Now())
		if er != nil {
			t.Fatal(er)
		}
		defer reopened.Close()
		after := reopened.Head()
		if before.Reserved.Calls != after.Reserved.Calls || before.Reserved.RequestBytes != after.Reserved.RequestBytes || before.Reserved.ResponseBytes != after.Reserved.ResponseBytes || before.Observed.ResponseBytes != after.Observed.ResponseBytes || after.Observed.ResponseBytes != 4 || after.LastCompletePublicationSHA256 != "" {
			t.Fatal("ASSERT_V3_RECOVERY_COST_NO_REPLAY_OR_AUTHORITY")
		}
		if _, er := os.Stat(filepath.Join(product.InventoryMailboxPathV3(s.q.Configuration.Store, s.runID), "request-00000002.json")); !os.IsNotExist(er) {
			t.Fatal("ASSERT_V3_RECOVERY_NO_RESEND")
		}
	})
	t.Run("bad-v3-config-still-charges-finalization", func(t *testing.T) {
		s := inventoryTestSetup(t)
		q := s.q
		q.Inventory.Pins.GrammarSHA256 = "bad"
		inventoryTestSeal(&q)
		inventoryTestWrite(t, s.path, q)
		n, _, e := inventoryTestEntry([]string{"finalize", s.path, s.runID}, []byte("[]"))
		if n == 0 || !strings.Contains(e, "CHARGED") {
			t.Fatal("ASSERT_V3_BAD_CONFIG_FINALIZE_REFUSED", e)
		}
		if _, er := os.Stat(filepath.Join(s.q.Configuration.Store, "host-finalize-attempt-00.json")); er != nil {
			t.Fatal("ASSERT_V3_BAD_CONFIG_FINALIZE_CHARGED")
		}
		for attempt := 1; attempt < 16; attempt++ {
			n, _, e := inventoryTestEntry([]string{"finalize", s.path, s.runID}, []byte("[]"))
			if n == 0 || !strings.Contains(e, "CHARGED") {
				t.Fatal("ASSERT_V3_INVALID_FINALIZATION_NEVER_REFUNDED", attempt, e)
			}
		}
		n, _, e = inventoryTestEntry([]string{"finalize", s.path, s.runID}, []byte("[]"))
		if n == 0 || !strings.Contains(e, "ATTEMPTS_EXHAUSTED") {
			t.Fatal("ASSERT_V3_FINALIZE_ORIGINAL_SIXTEEN_POOL", e)
		}
	})
	for _, kind := range []string{"corrupt-report", "partial-manifest", "malformed-observations"} {
		t.Run(kind, func(t *testing.T) {
			s := inventoryTestSetup(t)
			terminal := inventoryTestRun(t, s, nil)
			returns := inventoryTestReturns(s, terminal)
			destination := filepath.Join(s.q.Configuration.Store, "inventory-result-"+s.runID[7:])
			switch kind {
			case "corrupt-report":
				if e := os.WriteFile(filepath.Join(destination, "report.json"), []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
			case "partial-manifest":
				if e := os.Remove(filepath.Join(destination, "manifest.json")); e != nil {
					t.Fatal(e)
				}
			case "malformed-observations":
				returns = []byte("{bad")
			}
			n, _, e := inventoryTestEntry([]string{"finalize", s.path, s.runID}, returns)
			if n == 0 {
				t.Fatal("ASSERT_V3_FINALIZE_CORRUPTION_REFUSED", e)
			}
			if _, er := os.Stat(filepath.Join(s.q.Configuration.Store, "host-finalize-attempt-00.json")); er != nil {
				t.Fatal("ASSERT_V3_CORRUPT_FINALIZATION_CHARGED")
			}
			if _, er := os.Stat(filepath.Join(s.q.Configuration.Store, "host-observation-"+s.runID[7:]+".json")); !os.IsNotExist(er) {
				t.Fatal("ASSERT_V3_NO_PARTIAL_FINALIZE_SUCCESS")
			}
		})
	}
}
func TestApplicationInventoryV3ControllerLegacyControls(t *testing.T) {
	path, q := inventoryTestConfig(t)
	c := q.Configuration
	c.Schema = "private-host-application-config.v1"
	c.Profile = product.ApplicationProfile
	c.RolePromptSHA256 = [2]string{pilot.Digest([]byte(product.CohortRolePrompt("interpreter"))), pilot.Digest([]byte(product.CohortRolePrompt("reviewer")))}
	source := hostProductRequestV3{RevisionAssertion: c.RevisionAssertion, ServerSHA256: c.ProducerExecutableSHA256, OperationTimeoutMS: 10000, ExchangeTimeoutMS: 1000}
	source.CensusArguments = []string{"--workspace", c.SourceRoot, "--server", filepath.Join(filepath.Dir(c.SourceRoot), "never-executed-synthetic-server"), "--publication-root", filepath.Join(filepath.Dir(c.SourceRoot), "unused-publication"), "--source", ".", "--include", "*.go", "--machine", "--private-enumeration-projection"}
	source.Policy.TotalBytes = 4096
	source.Policy.FileBytes = 4096
	source.Policy.MaxPathDepth = 32
	r := hostApplicationRequest{Schema: "private-host-application-request.v1", Configuration: c, Template: source, MaxUnitsPerRun: 1, UnitResponseBytes: 1024, UnitTimeoutMS: 1000}
	policy := hostApplicationUnitPolicyDigest(r)
	r.Configuration.UnitPolicySHA256 = policy
	r.Configuration.ProducerConfigSHA256 = policy
	r.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(r.Configuration)
	r.ApprovalSHA256 = hostApplicationDigest(r)
	raw, _ := json.Marshal(r)
	if isApplicationInventoryV3(raw) {
		t.Fatal("ASSERT_LEGACY_NOT_RETARGETED")
	}
	inventoryTestWrite(t, path, r)
	// Empty source guarantees no original/native acquisition branch is reachable.
	for _, op := range []string{"setup", "discover"} {
		n, out, e := inventoryTestEntry([]string{op, path}, nil)
		if n != 0 {
			t.Fatal("ASSERT_LEGACY_WHOLE_CONTROLLER_POSITIVE", op, e, string(out))
		}
		if bytes.Contains(out, []byte("inventory-terminal.v3")) {
			t.Fatal("ASSERT_LEGACY_BYTES_NOT_V3")
		}
	}
	n, out, e := inventoryTestEntry([]string{"prompts"}, nil)
	if n != 0 || len(out) == 0 {
		t.Fatal("ASSERT_LEGACY_PROMPTS", e)
	}
	t.Log("ASSERT_LEGACY_WHOLE_CONTROLLER_POSITIVE: empty discover, no native process")
}

// Execute actual Run and its original publication without emitting a terminal.
// This lets the controlled substitution reach readback, without overwriting or
// weakening the separate durable terminal-publication protocol.
func inventoryPhase32UnterminatedRun(t *testing.T, s inventoryTestSession) (applicationInventoryTerminalRecordV3, ai.Report) {
	t.Helper()
	q := inventoryTestTyped(t, s.q)
	j, e := product.OpenApplication(q.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	if _, e = j.ReserveOperation(pilot.Digest([]byte(s.runID+"/inventory-single-run")), "inventory_single_run.v3", product.ApplicationBounds{}, time.Now()); e != nil {
		t.Fatal(e)
	}
	tr, e := product.NewInventoryTransportV3(j, q, s.runID, product.InventoryMailboxPathV3(q.Configuration.Store, s.runID))
	if e != nil {
		t.Fatal(e)
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type result struct {
		report ai.Report
		err    error
	}
	done := make(chan result, 1)
	destination := filepath.Join(q.Configuration.Store, "inventory-result-"+s.runID[7:])
	go func() {
		r, e := ai.Run(ctx, q.Inventory.RunRequest(ctx, product.InventoryNativeInputsV3{}, q.Configuration.SourceRoot, destination, tr.Binding.Ready[0].LaunchID, tr.Binding.Ready[1].LaunchID, tr))
		done <- result{r, e}
	}()
	for _, role := range []string{"interpreter", "reviewer"} {
		n, b, errout := inventoryTestEntry([]string{"wait", s.path, s.runID, role, "0", "2000"}, nil)
		var event applicationHostEvent
		if n != 0 || json.Unmarshal(b, &event) != nil || event.Bundle == nil {
			t.Fatal("scripted Run wait", errout, string(b))
		}
		f, e := product.ReadInventoryBundleV3(*event.Bundle)
		if e != nil {
			t.Fatal(e)
		}
		var c cat.NativeCatalogWireV3
		if json.Unmarshal(f.Payload, &c) != nil {
			t.Fatal("catalog")
		}
		b, _ = json.Marshal(cat.NativeDecisionV3{Predecessor: c.Revision, Actor: f.LaunchID, Reason: "scripted actual Run, no native owner", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
		n, _, errout = inventoryTestEntry([]string{"respond", s.path, s.runID, role, fmt.Sprint(event.GlobalSequence)}, b)
		if n != 0 {
			t.Fatal(errout)
		}
	}
	resultRun := <-done
	if resultRun.err != nil || !resultRun.report.Published || tr.ReconcilePhysicalReturnsV3() != nil {
		t.Fatal("actual Run", resultRun.err)
	}
	if _, e = j.PublishInventoryRunV3(q, s.runID, destination, resultRun.report); e != nil {
		t.Fatal("original publication", e)
	}
	h := j.Head()
	r := resultRun.report
	counts := map[string]int{}
	for _, d := range r.Dispositions {
		counts[d.Status]++
	}
	status := "PROVISIONAL"
	if r.Stages["native_preparation"].Failed > 0 {
		status = "PROVISIONAL_WITH_PREPARATION_REFUSALS"
	}
	return applicationInventoryTerminalRecordV3{Schema: "private-application-inventory-terminal.v3", Status: status, ApplicationSHA256: h.ApplicationSHA256, RunID: s.runID, SourceRevision: q.Inventory.SourceRevision, InventoryHead: h.LastCompletePublicationSHA256, SourceLeads: len(r.Census), Dispositions: len(r.Dispositions), UntouchedGaps: len(r.UntouchedGaps), SuccessfulDescriptions: r.SuccessfulDescriptions, DispositionCounts: counts, Calls: h.Observed.Calls, RequestBytes: h.Observed.RequestBytes, ResponseBytes: h.Observed.ResponseBytes, LastSequences: tr.LastSequences(), Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", Accounting: "SCRIPTED_CONTRACT_NO_NATIVE_AUTHORITY", OriginalCleanup: "UNKNOWN_NO_NEW_NATIVE_QUALIFICATION"}, r
}
func inventoryPhase32Verify(t *testing.T, p product.InventoryPublicationV3, s inventoryTestSession) error {
	t.Helper()
	return product.VerifyInventoryPublicationV3(p, inventoryTestTyped(t, s.q))
}
func inventoryPhase32Object(store, kind, id string) string {
	return filepath.Join(store, "objects", id[7:9], kind+"-"+id+".json")
}
func inventoryPhase32PublicationBinding(t *testing.T) {
	for _, kind := range []string{"positive", "positive-empty-census", "positive-nil-census", "positive-empty-gaps", "positive-nil-gaps", "empty-gaps-array", "scope", "provenance", "foreign-gap", "gap-order", "gap-nil", "lead-id", "lead-provenance", "lead-order", "census-nil", "empty-census-array", "foreign-disposition", "duplicate-disposition", "unknown-status", "membership", "report-revision", "report-version", "report-status", "catalog-predecessor", "catalog-evidence", "catalog-actor", "catalog-affected", "qconfig", "request-approval", "run-substitution", "ledger-approval", "missing-run-claim"} {
		t.Run(kind, func(t *testing.T) {
			s := inventoryTestSetupWith(t, func(q *inventoryTestRequest) {
				if kind != "empty-census-array" && kind != "positive-empty-census" && kind != "positive-nil-census" {
					q.Inventory.Manifest.Leads = []ai.Lead{{ID: "a", Provenance: "approved-a"}, {ID: "b", Provenance: "approved-b"}}
				}
				q.Inventory.Manifest.UntouchedGaps = []string{"approved-gap-a", "approved-gap-b"}
				if kind == "positive-nil-census" {
					q.Inventory.Manifest.Leads = nil
				}
				if kind == "positive-empty-gaps" {
					q.Inventory.Manifest.UntouchedGaps = []string{}
				}
				if kind == "positive-nil-gaps" || kind == "empty-gaps-array" {
					q.Inventory.Manifest.UntouchedGaps = nil
				}
			})
			terminal, originalReport := inventoryPhase32UnterminatedRun(t, s)
			var pub product.InventoryPublicationV3
			if e := product.ReadApplicationRecord(s.q.Configuration.Store, terminal.InventoryHead, &pub); e != nil {
				t.Fatal(e)
			}
			if e := inventoryPhase32Verify(t, pub, s); e != nil {
				t.Fatal("ASSERT_PHASE32_F2_ORIGINAL_PUBLICATION_POSITIVE", e)
			}
			var report ai.Report
			b, e := os.ReadFile(filepath.Join(pub.Directory, "report.json"))
			if e != nil || json.Unmarshal(b, &report) != nil {
				t.Fatal("read report", e)
			}
			q := s.q
			artifact := "report.json"
			switch kind {
			case "scope":
				report.Scope = "foreign-scope"
			case "provenance":
				report.Provenance = "foreign-provenance"
			case "foreign-gap":
				report.UntouchedGaps[0] = "FOREIGN GAP NOT IN THE APPROVED CENSUS"
			case "gap-order":
				report.UntouchedGaps[0], report.UntouchedGaps[1] = report.UntouchedGaps[1], report.UntouchedGaps[0]
			case "gap-nil":
				report.UntouchedGaps = nil
				pub.UntouchedGaps = 0
			case "lead-id":
				report.Census[0].ID = "foreign"
				report.Dispositions[0].LeadID = "foreign"
			case "lead-provenance":
				report.Census[0].Provenance = "foreign"
			case "lead-order":
				report.Census[0], report.Census[1] = report.Census[1], report.Census[0]
				report.Dispositions[0], report.Dispositions[1] = report.Dispositions[1], report.Dispositions[0]
			case "census-nil":
				report.Census = nil
				report.Dispositions = nil
				pub.Leads = 0
				pub.Dispositions = 0
			case "empty-census-array":
				report.Census = []ai.Lead{}
			case "empty-gaps-array":
				report.UntouchedGaps = []string{}
			case "foreign-disposition":
				report.Dispositions[0].LeadID = "foreign"
			case "duplicate-disposition":
				report.Dispositions[1] = report.Dispositions[0]
			case "unknown-status":
				report.Dispositions[0].Status = "INVENTED"
			case "membership":
				report.WithMembership++
			case "report-revision":
				report.CatalogRevision = pilot.Digest([]byte("foreign-revision"))
			case "report-version":
				report.Version = "foreign-version"
			case "report-status":
				report.Status = "ACCEPTED"
			case "catalog-predecessor", "catalog-evidence", "catalog-actor", "catalog-affected":
				artifact = "catalog-corrected.json"
			case "qconfig":
				q.Configuration.StorageIdentitySHA256 = pilot.Digest([]byte("foreign-storage"))
				inventoryTestSeal(&q)
			case "request-approval":
				q.Inventory.HostWindowMS++
				inventoryTestSeal(&q)
			case "run-substitution":
				pub.RunID = pilot.Digest([]byte("foreign-run"))
				destination := filepath.Join(q.Configuration.Store, "inventory-result-"+pub.RunID[7:])
				if e = os.Rename(pub.Directory, destination); e != nil {
					t.Fatal(e)
				}
				pub.Directory = destination
			case "ledger-approval":
				ledgerID := pilot.Digest([]byte(pub.ApplicationSHA256 + "/inventory-v3/" + s.runID))
				path := inventoryPhase32Object(q.Configuration.Store, "inventory-ledger", ledgerID)
				var ledger map[string]json.RawMessage
				b, e = os.ReadFile(path)
				if e != nil || json.Unmarshal(b, &ledger) != nil {
					t.Fatal("ledger", e)
				}
				var binding map[string]any
				if json.Unmarshal(ledger["Binding"], &binding) != nil {
					t.Fatal("binding")
				}
				binding["ConfigSHA256"] = pilot.Digest([]byte("foreign-approval"))
				ledger["Binding"], _ = json.Marshal(binding)
				inventoryTestWrite(t, path, ledger)
			case "missing-run-claim":
				id := pilot.Digest([]byte(s.runID + "/inventory-single-run"))
				if e = os.Remove(inventoryPhase32Object(q.Configuration.Store, "claim", id)); e != nil {
					t.Fatal(e)
				}
			}
			if !strings.HasPrefix(kind, "positive") && kind != "qconfig" && kind != "request-approval" && kind != "run-substitution" && kind != "ledger-approval" && kind != "missing-run-claim" {
				if artifact == "report.json" {
					b, _ = json.Marshal(report)
				} else {
					var c cat.NativeCatalogWireV3
					b, e = os.ReadFile(filepath.Join(pub.Directory, artifact))
					if e != nil || json.Unmarshal(b, &c) != nil {
						t.Fatal("catalog", e)
					}
					switch kind {
					case "catalog-predecessor":
						c.Predecessor = pilot.Digest([]byte("foreign-predecessor"))
					case "catalog-evidence":
						c.Evidence = append(c.Evidence, cat.SelectedEvidenceV3{ID: "foreign-evidence"})
					case "catalog-actor":
						c.Actor = "foreign-actor"
					case "catalog-affected":
						c.Affected = []string{"foreign-group"}
					}
					c.Revision = ""
					b, _ = json.Marshal(c)
					c.Revision = pilot.Digest(b)
					report.CatalogRevision = c.Revision
					reportBytes, _ := json.Marshal(report)
					if e = os.WriteFile(filepath.Join(pub.Directory, "report.json"), reportBytes, 0600); e != nil {
						t.Fatal(e)
					}
					for i := range pub.Manifest.Artifacts {
						if pub.Manifest.Artifacts[i].Name == "report.json" {
							pub.Manifest.Artifacts[i].Bytes = len(reportBytes)
							pub.Manifest.Artifacts[i].SHA256 = pilot.Digest(reportBytes)
						}
					}
					b, _ = json.Marshal(c)
				}
				if e = os.WriteFile(filepath.Join(pub.Directory, artifact), b, 0600); e != nil {
					t.Fatal(e)
				}
				for i := range pub.Manifest.Artifacts {
					if pub.Manifest.Artifacts[i].Name == artifact {
						pub.Manifest.Artifacts[i].Bytes = len(b)
						pub.Manifest.Artifacts[i].SHA256 = pilot.Digest(b)
					}
				}
				inventoryTestWrite(t, filepath.Join(pub.Directory, "manifest.json"), pub.Manifest)
				b, _ = json.Marshal(pub.Manifest)
				pub.ManifestSHA256 = pilot.Digest(b)
			}
			if strings.HasPrefix(kind, "positive") {
				b, _ = json.Marshal(terminal)
				if e = applicationWriteOnce(applicationTerminalPath(s.q.Configuration.Store, s.runID), b); e != nil {
					t.Fatal(e)
				}
				n, _, errout := inventoryTestEntry([]string{"finalize", s.path, s.runID}, inventoryTestReturns(s, terminal))
				if n != 0 {
					t.Fatal("ASSERT_PHASE32_F2_ORIGINAL_PUBLICATION_POSITIVE", errout)
				}
				t.Log("ASSERT_PHASE32_F2_ORIGINAL_PUBLICATION_POSITIVE")
				return
			}
			candidate := s
			candidate.q = q
			verifyErr := inventoryPhase32Verify(t, pub, candidate)
			j, e := product.OpenApplication(s.q.Configuration, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			_, publishErr := j.PublishInventoryRunV3(inventoryTestTyped(t, q), pub.RunID, pub.Directory, originalReport)
			j.Close()
			// Keep running all three checks after a failure so the RED records their independent behavior.
			if verifyErr == nil {
				t.Error("ASSERT_PHASE32_F2_VERIFY_REFUSES", kind)
			}
			if publishErr == nil {
				t.Error("ASSERT_PHASE32_F2_PUBLISH_REFUSES", kind)
			}
			if kind == "qconfig" || kind == "request-approval" {
				inventoryTestWrite(t, s.path, q)
			}
			b, _ = json.Marshal(pub)
			id := pilot.Digest(b)
			path := inventoryPhase32Object(s.q.Configuration.Store, "data", id)
			if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
			terminal.InventoryHead = id
			b, _ = json.Marshal(terminal)
			if !applicationInventoryValidTerminalV3(b) {
				t.Fatal("invalid synthetic terminal")
			}
			if e = applicationWriteOnce(applicationTerminalPath(s.q.Configuration.Store, s.runID), b); e != nil {
				t.Fatal(e)
			}
			n, _, errout := inventoryTestEntry([]string{"finalize", s.path, s.runID}, inventoryTestReturns(s, terminal))
			t.Logf("phase32 %s verify=%v publish=%v finalize=%d stderr=%s", kind, verifyErr, publishErr, n, errout)
			if n == 0 {
				t.Error("ASSERT_PHASE32_F2_FINALIZE_REFUSES", kind, errout)
			}
			if _, e = os.Stat(filepath.Join(s.q.Configuration.Store, "host-finalize-attempt-00.json")); e != nil {
				t.Fatal("ASSERT_PHASE32_F2_FINALIZE_CHARGED", e)
			}
			if !t.Failed() {
				t.Log("ASSERT_PHASE32_F2_VERIFY_REFUSES; ASSERT_PHASE32_F2_PUBLISH_REFUSES; ASSERT_PHASE32_F2_FINALIZE_REFUSES; ASSERT_PHASE32_F2_FINALIZE_CHARGED", kind)
			}
		})
	}
}
