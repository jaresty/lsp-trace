//go:build darwin || linux

package inventorysuccessor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	app "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	so "lsp-trace/internal/inventorysuccessor/sourceoperator"
	pa "lsp-trace/internal/programcadmission"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

func selectedFixtureV3(t *testing.T) sc.PrivateSelectedResultV2 {
	t.Helper()
	owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
	manager, start, uri := mixedStartFixtureV2(t)
	if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n"), 0644); e != nil {
		t.Fatal(e)
	}
	start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), "positive"}
	q := inv.PrivateMixedInputRequestV2{Manager: manager, Root: root, Start: start, URI: uri, OccurrenceID: "v3-controlled", Revision: "v3-controlled-revision", Privacy: "v3-controlled-private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []pa.RelationKind{pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}}
	d := "sha256:" + strings.Repeat("1", 64)
	input := cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "controlled-source", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, e := cc.NewContinuationContract(input)
	if e != nil {
		t.Fatal(e)
	}
	hash := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
	config := inv.PrivateSelectedConfigurationV2{RunID: "v3-fixture-run", SeedNamespace: "v3-fixture-namespace", Lineage: "v3-fixture-lineage", Mode: "RD", Seed: 7, Attempts: [3]string{hash("v3-M"), hash("v3-P"), hash("v3-S")}, Contract: contract, ContractInput: input, SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}}
	result, e := so.PrepareRDTargetV2(context.Background(), owner, q, config)
	if e != nil || !result.Available() {
		t.Fatal("ASSERT_V3_GENUINE_SELECTED_PREPARATION", e)
	}
	if manager.Census().Workers != 0 {
		t.Fatal("ASSERT_V3_NATIVE_CLEANUP_BEFORE_EXPOSURE")
	}
	row, unit, record, ok := result.Selected()
	if !ok || row.Intent.InputID == "" || unit.InputID != "" || row.Intent.Endpoint.Kind != "RESOLVES_TO_DEFINITION" || !strings.Contains(unit.Body, "func ") || record.Envelope.AdmissionID != unit.ID {
		t.Fatal("ASSERT_V3_NATIVE_LINEAGE")
	}
	before, _ := json.Marshal(manager.Census())
	t.Cleanup(func() {
		after, _ := json.Marshal(manager.Census())
		if !bytes.Equal(before, after) {
			t.Error("ASSERT_V3_ZERO_EXTRA_ACQUISITION", string(before), string(after))
		} else {
			t.Log("ASSERT_V3_ZERO_EXTRA_ACQUISITION: PASS")
		}
	})
	t.Log("ASSERT_V3_GENUINE_SELECTED_PREPARATION: PASS; ASSERT_V3_NATIVE_CLEANUP_BEFORE_EXPOSURE: PASS; ASSERT_V3_NATIVE_LINEAGE: PASS")
	return result
}
func semanticAdapterV3() []byte {
	return dw.EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"coordinates the fixture call","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"invokes the other fixture function","consumer_relative":false},"boundary_contribution":"fixture-local orchestration","limitations":["fixture only; no application completeness"]}`)
}
func bindingV3(t *testing.T, r sc.PrivateSelectedResultV2, b *sc.CallerBudgetV3) sc.CallerDescribeBindingV3 {
	t.Helper()
	_, _, q, _ := r.Selected()
	pin := "sha256:" + strings.Repeat("2", 64)
	return sc.CallerDescribeBindingV3{AttemptID: "fixture-description", Interpreter: "fixture-interpreter", Pins: dw.HostPinsV2{WorkerSHA256: pin, ModelSHA256: pin, GrammarSHA256: pin, PromptSHA256: "sha256:" + q.Envelope.InputSHA256}, Deadline: time.Now().Add(10 * time.Second), Limits: dw.SuppliedResponseLimits{MaxAdapterBytes: 64 << 10, MaxSemanticBytes: 16 << 10}, Budget: b}
}
func TestSelectedConsumerV3Genuine(t *testing.T) {
	r := selectedFixtureV3(t)
	ctx := context.Background()
	b, e := sc.NewCallerBudgetV3(time.Now().Add(120*time.Second), 64<<20)
	if e != nil {
		t.Fatal(e)
	}
	q := bindingV3(t, r, b)
	a, e := sc.PrepareSelectedDescribeV3(ctx, r, q)
	if e != nil {
		t.Fatal("ASSERT_V3_CONSUMER_PREPARES_POST_CLEANUP", e)
	}
	bound, e := sc.CompleteSelectedDescribeV3(ctx, a, semanticAdapterV3())
	if e != nil || !bound.Available() {
		t.Fatal("ASSERT_V3_BOUND_SEMANTIC_V2", e)
	}
	if _, e = sc.CompleteSelectedDescribeV3(ctx, a, semanticAdapterV3()); e == nil {
		t.Fatal("ASSERT_V3_COMPLETION_ONE_SHOT")
	}
	evidence, e := bound.Evidence(ctx)
	if e != nil {
		t.Fatal(e)
	}
	record, e := dw.ParseResponseRecordV2(evidence.Record)
	if e != nil || record.Host().RequestRecordID != evidence.Request.RecordID || record.Host().Provenance.PacketID != evidence.Unit.ID || record.Consumer().Resolution != "UNRESOLVED" {
		t.Fatal("ASSERT_V3_EXACT_HOST_NATIVE_BINDING", e)
	}
	for name, change := range map[string]func(*sc.CallerDescribeBindingV3){"pins": func(v *sc.CallerDescribeBindingV3) { v.Pins.PromptSHA256 = "foreign" },
		"worker-pin": func(v *sc.CallerDescribeBindingV3) { v.Pins.WorkerSHA256 = "foreign" }, "expiry": func(v *sc.CallerDescribeBindingV3) { v.Deadline = time.Now().Add(-time.Second) }} {
		v := q
		change(&v)
		if _, e = sc.PrepareSelectedDescribeV3(ctx, r, v); e == nil {
			t.Fatal("ASSERT_V3_PREPARE_REFUSAL", name)
		}
	}
	malformed, e := sc.PrepareSelectedDescribeV3(ctx, r, q)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sc.CompleteSelectedDescribeV3(ctx, malformed, []byte("{}\n")); e == nil {
		t.Fatal("ASSERT_V3_MALFORMED_REFUSED")
	}
	if _, e = sc.CompleteSelectedDescribeV3(ctx, malformed, semanticAdapterV3()); e == nil {
		t.Fatal("ASSERT_V3_REFUSAL_CONSUMES")
	}
	oversized, e := sc.PrepareSelectedDescribeV3(ctx, r, q)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sc.CompleteSelectedDescribeV3(ctx, oversized, make([]byte, q.Limits.MaxAdapterBytes+1)); e == nil {
		t.Fatal("ASSERT_V3_ADAPTER_OVERRUN_REFUSED")
	}
	canceled, e := sc.PrepareSelectedDescribeV3(ctx, r, q)
	if e != nil {
		t.Fatal(e)
	}
	cx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = sc.CompleteSelectedDescribeV3(cx, canceled, semanticAdapterV3()); e == nil {
		t.Fatal("ASSERT_V3_CANCEL_COMPLETION_REFUSED")
	}
	if _, e = sc.CompleteSelectedDescribeV3(ctx, canceled, semanticAdapterV3()); e == nil {
		t.Fatal("ASSERT_V3_CANCEL_CONSUMES")
	}
	t.Log("ASSERT_V3_ADAPTER_OVERRUN_REFUSED: PASS; ASSERT_V3_CANCEL_COMPLETION_REFUSED: PASS; ASSERT_V3_CANCEL_CONSUMES: PASS")
	t.Log("ASSERT_V3_BOUND_SEMANTIC_V2: PASS; ASSERT_V3_COMPLETION_ONE_SHOT: PASS; ASSERT_V3_EXACT_HOST_NATIVE_BINDING: PASS; ASSERT_V3_MALFORMED_REFUSED: PASS; ASSERT_V3_REFUSAL_CONSUMES: PASS")
}
func TestNativeCatalogV3GenuineMembers(t *testing.T) {
	r := selectedFixtureV3(t)
	ctx := context.Background()
	b, _ := sc.NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
	a, e := sc.PrepareSelectedDescribeV3(ctx, r, bindingV3(t, r, b))
	if e != nil {
		t.Fatal(e)
	}
	bound, e := sc.CompleteSelectedDescribeV3(ctx, a, semanticAdapterV3())
	if e != nil {
		t.Fatal(e)
	}
	base, e := cat.BuildNativeV3(ctx, b, "fixture-interpreter", []cat.NativeBoundDescriptionV3{bound})
	if e != nil {
		t.Fatal(e)
	}
	before, _ := base.Bytes(ctx)
	ev, _ := bound.Evidence(ctx)
	d := cat.NativeDecisionV3{Predecessor: base.Revision(), Actor: "fixture-interpreter", Reason: "fixture semantic adjudication", Affected: []string{}, Groups: []cat.NativeGroupV3{{ID: "g", Label: "fixture behavior", Reason: "source-bound decision", Members: []string{ev.ID, ev.ID}}}}
	raw, _ := json.Marshal(d)
	if _, e = cat.ApplyNativeDecisionV3(ctx, base, raw, false); e == nil {
		t.Fatal("ASSERT_V3_DUPLICATE_MEMBER_REFUSED")
	}
	d.Groups[0].Members = []string{ev.ID}
	raw, _ = json.Marshal(d)
	proposed, e := cat.ApplyNativeDecisionV3(ctx, base, raw, false)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := base.Bytes(ctx)
	if !bytes.Equal(before, after) {
		t.Fatal("ASSERT_V3_PREDECESSOR_BYTES_IMMUTABLE")
	}
	groups, e := proposed.SearchGroups(ctx, "FIXTURE", 1)
	if e != nil || len(groups) != 1 || groups[0].Members[0] != ev.ID {
		t.Fatal("ASSERT_V3_DESCRIPTIVE_SEARCH", e)
	}
	groups[0].Members[0] = "mutated"
	again, _ := proposed.SearchGroups(ctx, "fixture", 1)
	if again[0].Members[0] != ev.ID {
		t.Fatal("ASSERT_V3_SEARCH_SNAPSHOT")
	}
	d.Predecessor = proposed.Revision()
	d.Actor = "independent-fixture-reviewer"
	d.Affected = []string{"foreign"}
	raw, _ = json.Marshal(d)
	if _, e = cat.ApplyNativeDecisionV3(ctx, proposed, raw, true); e == nil {
		t.Fatal("ASSERT_V3_FOREIGN_AFFECTED_REFUSED")
	}
	t.Log("ASSERT_V3_DUPLICATE_MEMBER_REFUSED: PASS; ASSERT_V3_PREDECESSOR_BYTES_IMMUTABLE: PASS; ASSERT_V3_DESCRIPTIVE_SEARCH: PASS; ASSERT_V3_SEARCH_SNAPSHOT: PASS; ASSERT_V3_FOREIGN_AFFECTED_REFUSED: PASS")
}
func TestSelectedConsumerV3ConcurrentCompletion(t *testing.T) {
	r := selectedFixtureV3(t)
	ctx := context.Background()
	b, _ := sc.NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
	a, e := sc.PrepareSelectedDescribeV3(ctx, r, bindingV3(t, r, b))
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := sc.CompleteSelectedDescribeV3(ctx, a, semanticAdapterV3())
			results <- e == nil && v.Available()
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for v := range results {
		if v {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("ASSERT_V3_CONCURRENT_ONE_SHOT", wins)
	}
	t.Log("ASSERT_V3_CONCURRENT_ONE_SHOT: PASS")
}

// repairRequestV3 reuses one genuine, already-cleaned controlled result for
// bounded caller-tail trials. It does not replay native preparation or renew grants.
func repairRequestV3(t *testing.T, selected sc.PrivateSelectedResultV2) app.Request {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	pin := "sha256:" + strings.Repeat("2", 64)
	group := func(_ context.Context, raw []byte, w io.Writer) error {
		var c cat.NativeCatalogWireV3
		if e := cat.DecodeNativeV3(raw, &c); e != nil {
			return e
		}
		if len(c.Evidence) != 1 {
			return fmt.Errorf("fixture requires one bound description")
		}
		return json.NewEncoder(w).Encode(cat.NativeDecisionV3{Predecessor: c.Revision, Actor: "fixture-interpreter", Reason: "bound fixture semantic proposal", Affected: []string{}, Groups: []cat.NativeGroupV3{{ID: "g", Label: "fixture coordination", Reason: "source-bound fixture decision", Members: []string{c.Evidence[0].ID}}}})
	}
	review := func(_ context.Context, raw []byte, w io.Writer) error {
		var c cat.NativeCatalogWireV3
		if e := cat.DecodeNativeV3(raw, &c); e != nil {
			return e
		}
		g := c.Groups[0]
		g.Label = "reviewed fixture coordination"
		g.Reason = "distinct fixture reviewer correction"
		return json.NewEncoder(w).Encode(cat.NativeDecisionV3{Predecessor: c.Revision, Actor: "fixture-reviewer", Reason: "fixture correction; not authenticated", Affected: []string{g.ID}, Groups: []cat.NativeGroupV3{g}})
	}
	return app.Request{Manifest: app.Manifest{Scope: "supplied two-lead fixture, not repository enumeration", Provenance: "controlled manifest", Leads: []app.Lead{{ID: "selected", Provenance: "selected-source-v1"}, {ID: "untouched", Provenance: "untouched-source-v1"}}}, Preparer: func(context.Context, app.Lead) (app.NativeResult, error) {
		return app.NativeResult{Selected: selected, Status: "SELECTED"}, nil
	}, DescribeHost: func(_ context.Context, _ []byte, w io.Writer) error { _, e := w.Write(semanticAdapterV3()); return e }, GroupDecision: group, Reviewer: review, Interpreter: "fixture-interpreter", ReviewerIdentity: "fixture-reviewer", Pins: dw.HostPinsV2{WorkerSHA256: pin, ModelSHA256: pin, GrammarSHA256: pin}, Destination: filepath.Join(root, "publication"), MaxNativeAttempts: 1, NativeRetainedUpper: 8 << 20, HostWindow: time.Second}
}
func TestApplicationInventoryV3RepairCensusProvenance(t *testing.T) {
	q := repairRequestV3(t, selectedFixtureV3(t))
	want := append([]app.Lead(nil), q.Manifest.Leads...)
	prepare := q.Preparer
	q.Preparer = func(ctx context.Context, l app.Lead) (app.NativeResult, error) {
		for i := range q.Manifest.Leads {
			q.Manifest.Leads[i].Provenance = "mutated-during-callback"
		}
		return prepare(ctx, l)
	}
	out, e := app.Run(context.Background(), q)
	if e != nil || !out.Published {
		t.Fatal("fixture delivery prerequisite", e)
	}
	check := func(where string, raw []byte, expected []app.Lead) {
		t.Helper()
		var wire struct{ Census []app.Lead }
		e := json.Unmarshal(raw, &wire)
		got, _ := json.Marshal(wire.Census)
		want, _ := json.Marshal(expected)
		if e != nil || !bytes.Equal(got, want) {
			t.Fatal("ASSERT_V3_REPAIR_CENSUS_PROVENANCE", where, string(got), string(want), e)
		}
	}
	q.Manifest.Leads[0].Provenance = "mutated-after-return"
	returned, _ := json.Marshal(out)
	check("returned immutable admitted snapshot", returned, want)
	persisted, e := os.ReadFile(filepath.Join(q.Destination, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	check("persisted selected and untouched", persisted, want)
	q.Preparer = prepare
	q.Destination = filepath.Join(filepath.Dir(q.Destination), "changed")
	q.Manifest.Leads = []app.Lead{{ID: "selected", Provenance: "selected-source-v2"}, {ID: "untouched", Provenance: "untouched-source-v2"}}
	changed, e := app.Run(context.Background(), q)
	if e != nil || !changed.Published {
		t.Fatal("changed provenance delivery prerequisite", e)
	}
	raw, e := os.ReadFile(filepath.Join(q.Destination, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	check("changed corresponding delivered content", raw, q.Manifest.Leads)
	returned, _ = json.Marshal(changed)
	check("changed return", returned, q.Manifest.Leads)
	t.Log("ASSERT_V3_REPAIR_CENSUS_PROVENANCE: PASS")
}

func TestApplicationInventoryV3RepairSemanticAccounting(t *testing.T) {
	q := repairRequestV3(t, selectedFixtureV3(t))
	out, e := app.Run(context.Background(), q)
	if e != nil || !out.Published {
		t.Fatal("fixture delivery prerequisite", e)
	}
	raw, e := os.ReadFile(filepath.Join(q.Destination, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	var persisted app.Report
	if e = json.Unmarshal(raw, &persisted); e != nil {
		t.Fatal(e)
	}
	raw, e = os.ReadFile(filepath.Join(q.Destination, "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var manifest app.PublicationManifestV3
	if e = json.Unmarshal(raw, &manifest); e != nil {
		t.Fatal(e)
	}
	if persisted.SemanticReservedNS != int64(3*time.Second) || persisted.SemanticReservedNS != manifest.SemanticReservedNS || persisted.SemanticReservedNS != out.SemanticReservedNS {
		t.Fatal("ASSERT_V3_REPAIR_PERSISTED_SEMANTIC_RESERVATIONS", persisted.SemanticReservedNS, manifest.SemanticReservedNS, out.SemanticReservedNS)
	}
	if persisted.CallerAccounting != "PRE_PUBLICATION_SNAPSHOT_FINAL_IN_MANIFEST" || persisted.CallerBytes >= manifest.CallerBytes || manifest.CallerBytes != out.CallerBytes {
		t.Fatal("pre-publication/final byte accounting distinction", persisted.CallerBytes, manifest.CallerBytes, out.CallerBytes)
	}
	t.Log("ASSERT_V3_REPAIR_PERSISTED_SEMANTIC_RESERVATIONS: PASS")
}

func TestApplicationInventoryV3RepairAtomicReconciliation(t *testing.T) {
	// One controlled native acquisition, then at most 62 bounded caller-tail
	// invocations: control +26 binary trials +boundary +32 post-seal samples.
	q := repairRequestV3(t, selectedFixtureV3(t))
	trial := 0
	root := filepath.Dir(q.Destination)
	run := func(limit uint64) (app.Report, error) {
		trial++
		request := q
		request.BudgetBytes = limit
		request.Destination = filepath.Join(root, fmt.Sprintf("trial-%d", trial))
		return app.Run(context.Background(), request)
	}
	control, e := run(64 << 20)
	if e != nil || !control.Published || control.Stages["catalog"].Successful != 3 {
		t.Fatal("complete corrected catalog control prerequisite", e, control)
	}
	lo, hi := uint64(1), uint64(64<<20)
	for lo < hi {
		mid := lo + (hi-lo)/2
		o, _ := run(mid)
		if o.Stages["catalog"].Successful == 3 {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	boundary, refusal := run(lo)
	if refusal == nil || boundary.Published || boundary.Stages["catalog"].Successful != 3 || boundary.Stages["review"].Successful != 1 || boundary.SuccessfulDescriptions != 1 {
		t.Fatal("sealed corrected catalog / budget refusal prerequisite", lo, refusal, boundary)
	}
	t.Logf("REPAIR_ATOMIC_BOUNDARY: budget=%d catalog.successful=%d review.successful=%d describe.successful=%d error=%v", lo, boundary.Stages["catalog"].Successful, boundary.Stages["review"].Successful, boundary.SuccessfulDescriptions, refusal)
	check := func(o app.Report, minimum bool) {
		t.Helper()
		with, without := 0, 0
		for _, d := range o.Dispositions {
			switch d.Status {
			case "DESCRIBED_WITH_MEMBERSHIP":
				with++
			case "DESCRIBED_UNMATCHED":
				without++
			}
		}
		if with != o.WithMembership || without != o.WithoutMembership || with+without != o.SuccessfulDescriptions || (with > 0 && o.CatalogRevision == "") || (minimum && (with != 0 || without != 1 || o.CatalogRevision != "")) {
			t.Fatal("ASSERT_V3_REPAIR_ATOMIC_RECONCILIATION", minimum, with, without, o)
		}
	}
	check(boundary, true)
	if control.CallerBytes <= lo {
		t.Fatal("post-seal sampling interval prerequisite", lo, control.CallerBytes)
	}
	for i := uint64(1); i <= 32; i++ {
		o, _ := run(lo + (control.CallerBytes-lo)*i/32)
		if o.Stages["catalog"].Successful != 3 {
			t.Fatal("post-seal sample prerequisite", i, o)
		}
		check(o, false)
	}
	check(control, false)
	if trial > 62 {
		t.Fatal("bounded fixture calibration", trial)
	}
	t.Logf("ASSERT_V3_REPAIR_ATOMIC_RECONCILIATION: PASS; trials=%d; all native preparations=1", trial)
}

func TestApplicationInventoryV3NativeDelivery(t *testing.T) {
	nativeCalls := 0
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	destination := filepath.Join(root, "publication")
	pin := "sha256:" + strings.Repeat("2", 64)
	group := func(ctx context.Context, raw []byte, w io.Writer) error {
		var c cat.NativeCatalogWireV3
		if e := cat.DecodeNativeV3(raw, &c); e != nil {
			return e
		}
		d := cat.NativeDecisionV3{Predecessor: c.Revision, Actor: "fixture-interpreter", Reason: "actual controlled fixture semantic decision", Affected: []string{}, Groups: []cat.NativeGroupV3{{ID: "fixture-group", Label: "fixture coordination", Reason: "bound description describes fixture coordination", Members: []string{c.Evidence[0].ID}}}}
		return json.NewEncoder(w).Encode(d)
	}
	review := func(ctx context.Context, raw []byte, w io.Writer) error {
		var c cat.NativeCatalogWireV3
		if e := cat.DecodeNativeV3(raw, &c); e != nil {
			return e
		}
		g := c.Groups[0]
		g.Label = "reviewed fixture coordination"
		g.Reason = "distinct fixture reviewer renamed proposal"
		return json.NewEncoder(w).Encode(cat.NativeDecisionV3{Predecessor: c.Revision, Actor: "fixture-reviewer", Reason: "fixture-only correction, not authenticated human review", Affected: []string{g.ID}, Groups: []cat.NativeGroupV3{g}})
	}
	q := app.Request{Manifest: app.Manifest{Scope: "controlled two-lead fixture; not whole repository", Provenance: "supplied fixture manifest", Leads: []app.Lead{{ID: "selected", Provenance: "genuine native fixture"}, {ID: "untouched", Provenance: "budget gap"}}, UntouchedGaps: []string{"repository enumeration not exercised"}}, Preparer: func(context.Context, app.Lead) (app.NativeResult, error) {
		nativeCalls++
		return app.NativeResult{Selected: selectedFixtureV3(t), Status: "SELECTED"}, nil
	}, DescribeHost: func(_ context.Context, _ []byte, w io.Writer) error { _, e := w.Write(semanticAdapterV3()); return e }, GroupDecision: group, Reviewer: review, Interpreter: "fixture-interpreter", ReviewerIdentity: "fixture-reviewer", Pins: dw.HostPinsV2{WorkerSHA256: pin, ModelSHA256: pin, GrammarSHA256: pin}, Destination: destination, MaxNativeAttempts: 1, NativeRetainedUpper: 8 << 20, HostWindow: time.Second}
	out, e := app.Run(context.Background(), q)
	if e != nil || !out.Published || nativeCalls != 1 {
		t.Fatal("ASSERT_V3_NATIVE_END_TO_END", e, out)
	}
	if len(out.Dispositions) != 2 || out.Dispositions[1].Status != "UNTOUCHED" || out.SuccessfulDescriptions != out.WithMembership+out.WithoutMembership || out.WithMembership != 1 {
		t.Fatal("ASSERT_V3_E2E_CONSERVATION", out)
	}
	for name, s := range out.Stages {
		if s.Requested != s.Successful+s.Failed {
			t.Fatal("ASSERT_V3_NAMED_STAGE_CONSERVATION", name, s)
		}
	}
	manifest, e := os.ReadFile(filepath.Join(destination, "manifest.json"))
	var published app.PublicationManifestV3
	if e != nil || json.Unmarshal(manifest, &published) != nil || len(published.Artifacts) != 4 {
		t.Fatal("ASSERT_V3_MANIFEST_READBACK", e)
	}
	for _, a := range published.Artifacts {
		raw, e := os.ReadFile(filepath.Join(destination, a.Name))
		if e != nil || cat.NativeDigestV3(raw) != a.SHA256 {
			t.Fatal("ASSERT_V3_ARTIFACT_READBACK", e)
		}
	}
	base, _ := os.ReadFile(filepath.Join(destination, "catalog-proposed.json"))
	corrected, _ := os.ReadFile(filepath.Join(destination, "catalog-corrected.json"))
	if bytes.Equal(base, corrected) {
		t.Fatal("ASSERT_V3_IMMUTABLE_CORRECTION_SUCCESSOR")
	}
	var c cat.NativeCatalogWireV3
	if cat.DecodeNativeV3(corrected, &c) != nil || c.Authority != 0 || c.Accepted || c.Feature != "UNRESOLVED" || c.Consumer != "UNRESOLVED" || c.Absence != "PROHIBITED" {
		t.Fatal("ASSERT_V3_PROVISIONAL_CEILING")
	}
	q.ReviewerIdentity = q.Interpreter
	q.Destination = filepath.Join(root, "refused-publication")
	refused, refusal := app.Run(context.Background(), q)
	if refusal == nil || refused.Published || refused.SuccessfulDescriptions != 1 || refused.WithMembership+refused.WithoutMembership != 1 || refused.WithoutMembership != 1 {
		t.Fatal("ASSERT_V3_LATE_FAILURE_DESCRIPTION_CONSERVATION", refusal, refused)
	}
	for name, s := range refused.Stages {
		if s.Requested != s.Successful+s.Failed {
			t.Fatal("ASSERT_V3_LATE_FAILURE_STAGE_CONSERVATION", name, s)
		}
	}
	if _, e = os.Stat(q.Destination); !os.IsNotExist(e) {
		t.Fatal("ASSERT_V3_REFUSED_REVIEW_NO_PUBLICATION", e)
	}
	t.Log("ASSERT_V3_LATE_FAILURE_DESCRIPTION_CONSERVATION: PASS; ASSERT_V3_LATE_FAILURE_STAGE_CONSERVATION: PASS; ASSERT_V3_REFUSED_REVIEW_NO_PUBLICATION: PASS")
	t.Log("ASSERT_V3_NATIVE_END_TO_END: PASS; ASSERT_V3_E2E_CONSERVATION: PASS; ASSERT_V3_NAMED_STAGE_CONSERVATION: PASS; ASSERT_V3_MANIFEST_READBACK: PASS; ASSERT_V3_ARTIFACT_READBACK: PASS; ASSERT_V3_IMMUTABLE_CORRECTION_SUCCESSOR: PASS; ASSERT_V3_PROVISIONAL_CEILING: PASS")
}
