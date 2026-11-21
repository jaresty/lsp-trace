//go:build lsp_trace_controlled_describe_fixture && (darwin || linux)

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	ai "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	product "lsp-trace/internal/privatehostproductv3"
	pa "lsp-trace/internal/programcadmission"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This composes the existing controller with the existing tagged io.Pipe native
// fixture. Its authorization and semantic responses are synthetic test data,
// not live grants, provider/model calls or evidence of context independence.
func TestCensusReviewModeNonemptyControllerV3(t *testing.T) {
	publication, request := censusNonemptyIdentityControllerV3(t, "", false)
	// Use the actual controller publication above, not a canned publication.
	// This witnesses readback rejection, not atomic filesystem publication or
	// normal-folder continuation (which has its own still-failing guard).
	path := filepath.Join(publication.Directory, "report.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("ASSERT_PUBLICATION_READBACK: setup", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal("ASSERT_PUBLICATION_READBACK: perturbation", err)
	}
	// Restore even if the rejection assertion fails. Never leave the fixture's
	// publication corrupted for its other cleanup/finalization assertions.
	t.Cleanup(func() {
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Error("ASSERT_PUBLICATION_READBACK: cleanup restore", err)
		}
	})
	if err := product.VerifyInventoryPublicationV3(publication, request); err == nil {
		t.Fatal("ASSERT_PUBLICATION_READBACK: FAIL accepted corrupted report")
	}
	t.Log("ASSERT_PUBLICATION_READBACK: rejected corrupted report")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal("ASSERT_PUBLICATION_READBACK: restore", err)
	}
	if err := product.VerifyInventoryPublicationV3(publication, request); err != nil {
		t.Fatal("ASSERT_PUBLICATION_READBACK: FAIL rejected restored report", err)
	}
	t.Log("ASSERT_PUBLICATION_READBACK: PASS exact restored publication")
}
func TestCensusFreshIdentityCaptureV3(t *testing.T) {
	for _, kind := range []string{"CLEAN_GIT", "DIRTY_GIT", "NON_GIT"} {
		t.Run(kind, func(t *testing.T) { censusNonemptyIdentityControllerV3(t, kind, false) })
	}
	t.Run("stale-captured-digest", func(t *testing.T) { censusNonemptyIdentityControllerV3(t, "DIRTY_GIT", true) })
}
func censusNonemptyIdentityControllerV3(t *testing.T, sourceKind string, stale bool) (product.InventoryPublicationV3, product.InventoryRequestV3) {
	t.Helper()
	assertion := "ASSERT_REVIEW_NONEMPTY_CONTROLLER"
	if sourceKind != "" {
		assertion = "ASSERT_FRESH_IDENTITY_CAPTURE"
	}
	descriptions, calls := 1, 3
	if stale {
		descriptions, calls = 0, 2
	}
	c, root, owner, fixture := prepareCLIFixture(t)
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	path, legacy := inventoryTestConfig(t)
	raw, _ := json.Marshal(legacy)
	var q product.InventoryRequestV3
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal("SETUP_REQUEST", err)
	}
	q.Configuration.SourceRoot = c.Workspace
	if sourceKind != "" {
		document, err := os.ReadFile(strings.TrimPrefix(fixture.URI(), "file://"))
		if err != nil {
			t.Fatal("SETUP_DOCUMENT_IDENTITY", err)
		}
		expected := cat.NativeDigestV3(document)
		if stale {
			expected = cat.NativeDigestV3([]byte("stale approved bytes"))
		}
		identity := product.InventorySourceIdentityV3{Schema: "inventory-source-identity.v1", SourceRoot: c.Workspace, Kind: sourceKind, DeclaredRevision: "controlled working document", Claim: "CALLER_ASSERTED_NOT_AUTHENTICATED", Completeness: "CALLER_DECLARED_COMPLETE_NOT_FILESYSTEM_VERIFIED", Documents: []product.InventorySourceDocumentV3{{URI: fixture.URI(), SourceDigest: expected}}}
		if sourceKind != "NON_GIT" {
			identity.GitHead = strings.Repeat("a", 40)
		}
		q.Inventory.SourceIdentity = &identity
		q.Inventory.SourceRevision = product.InventorySourceIdentityRevisionV3(identity)
		q.Configuration.RevisionAssertion = q.Inventory.SourceRevision
	}
	q.Inventory.ReviewMode = cat.ReviewModeSameThreadV3
	lead := ai.Lead{ID: "pipe-query", Provenance: "controlled pipe reference; not repository enumeration"}
	q.Inventory.Manifest = ai.Manifest{Scope: "same-thread-one-pipe-reference", Provenance: "controlled one-lead manifest", Leads: []ai.Lead{lead}, UntouchedGaps: []string{"all other application source remains unexamined"}}
	q.Inventory.NativeRetainedUpper = 8 << 20
	pin := prepareCLIHash("nonempty controlled contract")
	input := cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "controlled-source", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, err := cc.NewContinuationContract(input)
	if err != nil {
		t.Fatal("SETUP_CONTRACT", err)
	}
	native := product.InventoryNativeInputV3{Lead: lead, Owner: owner,
		Capture:       inv.PrivateMixedInputRequestV2{Manager: fixture.Manager(), Root: root, Start: fixture.StartRequest(c.Persist.Deadline), URI: fixture.URI(), OccurrenceID: "controlled-query", Revision: q.Inventory.SourceRevision, Privacy: "private-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 5, Kinds: []pa.RelationKind{pa.TypedReferencesSymbol}},
		Configuration: inv.PrivateSelectedConfigurationV2{RunID: "nonempty-fixture", SeedNamespace: "nonempty-fixture-seed", Lineage: "nonempty-fixture-lineage", Mode: "R", Seed: 7, Attempts: [3]string{prepareCLIHash("nonempty-M"), prepareCLIHash("nonempty-P"), prepareCLIHash("nonempty-S")}, Contract: contract, ContractInput: input, SourceLimits: targetpacket.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}},
		Role:          sc.PrivateSelectedRoleV3{Kind: "REFERENCES_SYMBOL", Role: "REFERENCED_SYMBOL"},
	}
	q.Inventory.Targets = []product.InventoryTargetV3{product.InventoryNativeTargetV3(native)}
	// Bind the exact resolved request before setup; no later approval retrofit.
	policy, _ := json.Marshal(q.Inventory)
	manifest, _ := json.Marshal(q.Inventory.Manifest)
	q.Configuration.UnitPolicySHA256 = cat.NativeDigestV3(policy)
	q.Configuration.ProducerConfigSHA256 = q.Configuration.UnitPolicySHA256
	q.Configuration.SelectionRuleSHA256 = cat.NativeDigestV3(manifest)
	q.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(q.Configuration)
	q.ApprovalSHA256 = product.InventoryRequestDigestV3(q)
	if err := product.ValidateInventoryRequestV3(q); err != nil {
		t.Fatal("SETUP_BOUND_REQUEST", err)
	}
	inventoryTestWrite(t, path, q)
	code, setupBytes, diagnostic := inventoryTestEntry([]string{"setup", path}, nil)
	var setup struct{ RunID string }
	if code != 0 || json.Unmarshal(setupBytes, &setup) != nil || !validApplicationDigest(setup.RunID) {
		t.Fatal("SETUP_CONTROLLER", code, diagnostic)
	}
	const actor = "synthetic-one-context"
	for i, role := range []string{"interpreter", "reviewer"} {
		code, _, diagnostic = inventoryTestEntry([]string{"register", path, setup.RunID, role, actor, q.Configuration.RolePromptSHA256[i]}, nil)
		if code != 0 {
			t.Fatal("SETUP_REGISTRATION", role, diagnostic)
		}
	}
	raw, _ = json.Marshal(q)
	done := make(chan inventoryTestResult, 1)
	go func() {
		var out, diag bytes.Buffer
		code := runApplicationInventoryControllerV3([]string{"run", path, setup.RunID}, raw, bytes.NewReader(nil), &out, &diag, product.InventoryNativeInputsV3{lead.ID: native}, nil)
		done <- inventoryTestResult{code, out.Bytes(), diag.String()}
	}()
	// Join on every test exit. The original finite native deadline is not renewed.
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("controlled controller did not join")
		}
	})
	var proposedRevision string
	cursor := map[string]uint64{}
	operations := []string{"Describe", "GroupDecision", "Reviewer"}
	if stale {
		operations = operations[1:]
	}
	for _, operation := range operations {
		role := "interpreter"
		if operation == "Reviewer" {
			role = "reviewer"
		}
		code, eventBytes, diagnostic := inventoryTestEntry([]string{"wait", path, setup.RunID, role, fmt.Sprint(cursor[role]), "2000"}, nil)
		var event applicationHostEvent
		if code != 0 || json.Unmarshal(eventBytes, &event) != nil || event.Event != "NEEDS_HOST" || event.Bundle == nil {
			t.Fatal(assertion, "FAIL exchange", operation, code, diagnostic, string(eventBytes))
		}
		frame, err := product.ReadInventoryBundleV3(*event.Bundle)
		if err != nil || frame.Operation != operation || frame.Role != role || frame.LaunchID != actor || frame.SourceRevision != q.Inventory.SourceRevision {
			t.Fatal(assertion, "FAIL operation binding", operation, frame.Operation, frame.Role, frame.LaunchID, err, string(frame.Payload))
		}
		cursor[role] = event.GlobalSequence
		var payload []byte
		if operation == "Describe" {
			// Reaching Describe requires genuine selected native preparation and cleanup.
			if fixture.Manager().Census().Workers != 0 || !fixture.CloseSnapshot().PipeJoined {
				t.Fatal(assertion, "FAIL native cleanup before Describe")
			}
			payload = dw.EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"holds a fixture reference","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"references Query","consumer_relative":false},"boundary_contribution":"fixture-local reference","limitations":["controlled fixture; no application completeness"]}`)
		} else {
			var catalog cat.NativeCatalogWireV3
			if err := cat.DecodeNativeV3(frame.Payload, &catalog); err != nil || len(catalog.Evidence) != descriptions || catalog.ReviewMode != cat.ReviewModeSameThreadV3 {
				t.Fatal(assertion, "FAIL bound description", operation, err)
			}
			if stale {
				if fixture.Manager().Census().Workers != 0 || !fixture.CloseSnapshot().PipeJoined {
					t.Fatal(assertion, "FAIL native preparation/cleanup not observed")
				}
				payload, _ = json.Marshal(cat.NativeDecisionV3{Predecessor: catalog.Revision, Actor: actor, Reason: "unresolved stale source expectation", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
			} else {
				group := cat.NativeGroupV3{ID: "fixture-reference", Label: "fixture reference", Reason: "controlled proposal", Members: []string{catalog.Evidence[0].ID}}
				affected := []string{}
				if operation == "Reviewer" {
					if len(catalog.Groups) != 1 || catalog.Predecessor != proposedRevision || catalog.Actor != actor {
						t.Fatal(assertion, "FAIL reviewer did not receive proposal")
					}
					group = catalog.Groups[0]
					group.Label = "sequentially reviewed fixture reference"
					group.Reason = "separate review exchange; not independent context"
					affected = []string{group.ID}
				} else {
					proposedRevision = catalog.Revision
				}
				payload, _ = json.Marshal(cat.NativeDecisionV3{Predecessor: catalog.Revision, Actor: actor, Reason: "scripted sequential review; not authenticated", Affected: affected, Groups: []cat.NativeGroupV3{group}})
			}
		}
		code, _, diagnostic = inventoryTestEntry([]string{"respond", path, setup.RunID, role, fmt.Sprint(event.GlobalSequence)}, payload)
		if code != 0 {
			t.Fatal(assertion, "FAIL response", operation, diagnostic)
		}
	}
	result := <-done
	done <- result
	var terminal applicationInventoryTerminalRecordV3
	expectedCode, expectedStatus := 0, "PROVISIONAL"
	if stale {
		expectedCode, expectedStatus = 1, "PROVISIONAL_WITH_PREPARATION_REFUSALS"
	}
	if result.code != expectedCode || json.Unmarshal(result.raw, &terminal) != nil || terminal.Status != expectedStatus || terminal.InventoryHead == "" || terminal.Calls != uint64(calls) || terminal.Authority != 0 || terminal.Accepted || terminal.Completeness != "UNKNOWN" {
		t.Fatal(assertion, "FAIL terminal", result.code, result.err, string(result.raw))
	}
	var publication product.InventoryPublicationV3
	if err := product.ReadApplicationRecord(q.Configuration.Store, terminal.InventoryHead, &publication); err != nil || product.VerifyInventoryPublicationV3(publication, q) != nil || publication.Leads != 1 || publication.Dispositions != 1 || publication.SuccessfulDescriptions != descriptions || publication.UntouchedGaps != 1 {
		t.Fatal(assertion, "FAIL publication reopen", err)
	}
	read := func(name string, value any) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(publication.Directory, name))
		if err != nil || cat.DecodeNativeV3(b, value) != nil {
			t.Fatal(assertion, "FAIL artifact reopen", name, err)
		}
	}
	var report ai.Report
	read("report.json", &report)
	var corrected cat.NativeCatalogWireV3
	read("catalog-corrected.json", &corrected)
	if !report.Published || report.ReviewMode != cat.ReviewModeSameThreadV3 || report.SuccessfulDescriptions != descriptions || report.WithMembership != descriptions || report.WithoutMembership != 0 || len(report.Census) != 1 || len(report.Dispositions) != 1 || report.Dispositions[0].LeadID != lead.ID || report.Stages["review"].Successful != 1 || corrected.ReviewMode != cat.ReviewModeSameThreadV3 || corrected.ReviewIdentity != "NOT_AUTHENTICATED" || len(corrected.Evidence) != descriptions || len(corrected.Groups) != descriptions || corrected.Authority != 0 || corrected.Accepted || corrected.Completeness != "UNKNOWN" {
		t.Fatal(assertion, "FAIL conservation or review", report, corrected)
	}
	if stale {
		if report.Dispositions[0].Status != "PREPARATION_FAILED" {
			t.Fatal(assertion, "FAIL stale source not unresolved", report.Dispositions)
		}
	} else if len(corrected.Groups[0].Members) != 1 || corrected.Groups[0].Members[0] != corrected.Evidence[0].ID || corrected.Groups[0].Label != "sequentially reviewed fixture reference" {
		t.Fatal(assertion, "FAIL reviewed membership")
	}
	// The return helper needs only these legacy configuration/mode fields; the
	// actual finalizer independently decodes the full target-bound request on disk.
	legacy.Configuration, legacy.Inventory.ReviewMode = q.Configuration, q.Inventory.ReviewMode
	session := inventoryTestSession{path: path, q: legacy, runID: setup.RunID}
	code, finalBytes, diagnostic := inventoryTestEntry([]string{"finalize", path, setup.RunID}, inventoryTestReturns(session, terminal))
	var final struct {
		ReviewMode           cat.ReviewModeV3
		Status, Completeness string
		Authority            int
		Accepted             bool
	}
	if code != 0 || json.Unmarshal(finalBytes, &final) != nil || final.ReviewMode != cat.ReviewModeSameThreadV3 || final.Status != "SCRIPTED_CONTRACT_RETURNS_NOT_AGENTS" || final.Completeness != "UNKNOWN" || final.Authority != 0 || final.Accepted {
		t.Fatal(assertion, "FAIL finalization", code, diagnostic, string(finalBytes))
	}
	if product.VerifyInventoryPublicationV3(publication, q) != nil {
		t.Fatal(assertion, "FAIL post-finalization reopen")
	}
	t.Log(assertion + ": PASS")
	return publication, q
}
