//go:build darwin || linux

package inventorysuccessor_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	app "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
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

type configurationCaseV3 struct {
	ID, Mode, Kind, Role string
	Fixture              configurationFixtureV3
}
type configurationRecordV3 struct {
	CaseID                string            `json:"case_id"`
	Configuration         string            `json:"configuration"`
	Requested             []pa.RelationKind `json:"requested_relation_families"`
	Direction             string            `json:"direction"`
	Role                  string            `json:"endpoint_role"`
	Multiplicity          string            `json:"multiplicity"`
	ExpectedFamilyState   map[string]string `json:"expected_per_family_state"`
	ObservedFamilyState   map[string]string `json:"observed_per_family_state"`
	Layer                 string            `json:"evidence_layer"`
	Obligations           []string          `json:"applicable_obligations"`
	Expected              string            `json:"expected_outcome"`
	Actual                string            `json:"actual_outcome"`
	Disposition           string            `json:"disposition"`
	Operation             string            `json:"operation_identity"`
	Authority             string            `json:"native_authority_identity"`
	OriginalInputID       string            `json:"original_input_id,omitempty"`
	AdmissionID           string            `json:"admission_id,omitempty"`
	RequestID             string            `json:"request_id,omitempty"`
	Gap                   string            `json:"gap_reason"`
	NativeReturnWorkers   int               `json:"native_return_workers"`
	FixtureCleanupWorkers int               `json:"fixture_cleanup_workers"`
	NativeDiagnostic      any               `json:"native_diagnostic,omitempty"`
}

const configurationFullLayerV3 = "CONTROLLED_NATIVE.V3_FULL"

func configurationLayerAllowedV3(layer string) bool {
	switch layer {
	case "CONTROLLED_NATIVE.V3_FULL", "CONTROLLED_NATIVE.PROVIDER", "CONTROLLED_NATIVE.CAPTURE_REFUSAL", "CONTROLLED_NATIVE.SOURCE_PREPARATION_REFUSAL", "CONTRACT.CALLER_GUARD", "CONTRACT.COVERAGE":
		return true
	}
	return false
}
func configurationEmitV3(t *testing.T, r *configurationRecordV3) {
	t.Helper()
	if !configurationLayerAllowedV3(r.Layer) {
		t.Error("ASSERT_CONFIGURATION_V3_EVIDENCE_LAYER", r.Layer)
	}
	r.Operation = t.Name()
	if r.Disposition == "" {
		r.Disposition = "PASS_BOUNDED"
	}
	if t.Failed() {
		r.Disposition = "FAIL"
	}
	if t.Skipped() {
		r.Disposition = "NOT_RUN"
		r.Gap = "Darwin controlled SELF only; Linux compile-only"
	}
	raw, _ := json.Marshal(r)
	t.Log("CONFIGURATION_MATRIX", string(raw))
}
func configurationExpectedV3(c configurationCaseV3) map[string]string {
	result := map[string]string{}
	for _, pair := range []struct{ letter, kind, state string }{{"C", "CALLS", c.Fixture.Outgoing}, {"R", "REFERENCES_SYMBOL", c.Fixture.References}, {"D", "RESOLVES_TO_DEFINITION", c.Fixture.Definitions}} {
		state := "NOT_REQUESTED"
		if strings.Contains(c.Mode, pair.letter) {
			state = "OBSERVED_NONEMPTY"
			if pair.state == "empty" {
				state = "OBSERVED_EMPTY"
			}
			if pair.state == "unavailable" {
				state = "UNAVAILABLE"
			}
			if pair.letter == "C" && c.Fixture.Incoming != "empty" && c.Fixture.Incoming != "unavailable" {
				state = "OBSERVED_NONEMPTY"
			}
		}
		result[pair.kind] = state
	}
	return result
}
func configurationPositiveV3(t *testing.T, c configurationCaseV3) {
	t.Helper()
	record := configurationRecordV3{CaseID: c.ID, Configuration: c.Mode, Requested: configurationKindsV3(c.Mode), Direction: "outgoing-only", Role: c.Role, Multiplicity: "one per requested family; R/D share span", ExpectedFamilyState: configurationExpectedV3(c), Layer: configurationFullLayerV3, Obligations: []string{"configuration", "exact-role", "selected-native-joins", "driver-publication"}, Expected: "PUBLISHED_PROVISIONAL", Actual: "NOT_COMPLETED", Authority: "fresh isolated controlled owner; no live authority"}
	defer configurationEmitV3(t, &record)
	q := repairRequestV3(t, sc.PrivateSelectedResultV2{}) // caller-tail defaults only; the opaque placeholder is never consumed
	q.Manifest.Provenance = "controlled case " + c.ID
	if !strings.Contains(c.Mode, "C") {
		record.Direction = "NOT_REQUESTED"
	} else if c.Fixture.Outgoing == "empty" {
		record.Direction = "incoming-only"
	}
	nativeCalls, describes, groups, reviews := 0, 0, 0, 0
	describe, group, review := q.DescribeHost, q.GroupDecision, q.Reviewer
	q.DescribeHost = func(ctx context.Context, b []byte, w io.Writer) error { describes++; return describe(ctx, b, w) }
	q.GroupDecision = func(ctx context.Context, b []byte, w io.Writer) error { groups++; return group(ctx, b, w) }
	q.Reviewer = func(ctx context.Context, b []byte, w io.Writer) error { reviews++; return review(ctx, b, w) }
	q.Preparer = func(ctx context.Context, _ app.Lead) (app.NativeResult, error) {
		nativeCalls++
		owner, capture, config, _ := configurationInputsV3(t, c)
		selected, e := so.PrepareSelectedRoleV3(ctx, owner, capture, config, sc.PrivateSelectedRoleV3{Kind: c.Kind, Role: c.Role})
		if e != nil {
			record.Actual = e.Error()
			return app.NativeResult{}, e
		}
		row, unit, request, ok := selected.Selected()
		if !ok || row.Slot.Mode != c.Mode || row.Intent.Mode != c.Mode || row.Intent.Endpoint.Kind != c.Kind || row.Intent.Endpoint.Role != c.Role || row.Intent.InputID == "" || unit.InputID != "" || unit.ID != request.Envelope.AdmissionID || request.Lineage.PacketID != unit.ID || capture.Manager.Census().Workers != 0 || !time.Now().Before(capture.Start.Deadline) {
			t.Fatal("ASSERT_CONFIGURATION_V3_EXACT_NATIVE_IDENTITY", c.ID, row, unit, request)
		}
		expectedBody := "func Other"
		if c.Role == "QUERY_OCCURRENCE" || c.Role == "REFERENCED_SYMBOL" || c.Role == "CALLER_OCCURRENCE" {
			expectedBody = "func Root"
		}
		if c.Fixture.Incoming != "empty" && c.Fixture.Outgoing == "empty" {
			if c.Role == "CALLER_OCCURRENCE" {
				expectedBody = "func Other"
			}
			if c.Role == "CALLEE_TARGET" {
				expectedBody = "func Root"
			}
		}
		if !strings.Contains(unit.Body, expectedBody) {
			t.Fatal("ASSERT_CONFIGURATION_V3_DISTINCT_ENDPOINT_BODY", c.ID, expectedBody, unit.Body)
		}
		record.Authority = "CONTROLLED_RESOURCE_HEAD:" + owner.PreparationHeadV1()
		record.OriginalInputID = row.Intent.InputID
		record.AdmissionID = unit.ID
		record.RequestID = request.RecordID
		record.ObservedFamilyState = map[string]string{c.Kind: "SOURCE_ADMITTED_NONEMPTY"}
		record.Gap = "Other requested-family provider dispositions are separately exercised by provider cases; no application denominator"
		return app.NativeResult{Selected: selected, Status: "SELECTED"}, nil
	}
	out, e := app.Run(context.Background(), q)
	if e != nil || !out.Published || nativeCalls != 1 || describes != 1 || groups != 1 || reviews != 1 || out.CallerBytes > 64<<20 || out.SemanticReservedNS != int64(3*time.Second) || out.WithMembership != 1 || out.WithoutMembership != 0 || len(out.Dispositions) != 2 || out.Dispositions[0].Status != "DESCRIBED_WITH_MEMBERSHIP" || out.Dispositions[1].Status != "UNTOUCHED" {
		t.Fatal("ASSERT_CONFIGURATION_V3_NATIVE_POSITIVE", c.ID, e, record.Actual, out)
	}
	raw, e := os.ReadFile(filepath.Join(q.Destination, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	var report app.Report
	if json.Unmarshal(raw, &report) != nil || len(report.Census) != 2 || report.Census[0].Provenance != q.Manifest.Leads[0].Provenance || report.Census[1].Provenance != q.Manifest.Leads[1].Provenance || report.SemanticReservedNS != int64(3*time.Second) {
		t.Fatal("ASSERT_CONFIGURATION_V3_REPORT", c.ID, report)
	}
	raw, e = os.ReadFile(filepath.Join(q.Destination, "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var manifest app.PublicationManifestV3
	if json.Unmarshal(raw, &manifest) != nil || manifest.SemanticReservedNS != out.SemanticReservedNS || manifest.CallerBytes != out.CallerBytes || len(manifest.Artifacts) != 4 {
		t.Fatal("ASSERT_CONFIGURATION_V3_MANIFEST", c.ID)
	}
	for _, a := range manifest.Artifacts {
		data, e := os.ReadFile(filepath.Join(q.Destination, a.Name))
		if e != nil || cat.NativeDigestV3(data) != a.SHA256 {
			t.Fatal("ASSERT_CONFIGURATION_V3_READBACK", c.ID, e)
		}
	}
	base, _ := os.ReadFile(filepath.Join(q.Destination, "catalog-proposed.json"))
	corrected, _ := os.ReadFile(filepath.Join(q.Destination, "catalog-corrected.json"))
	var catalog cat.NativeCatalogWireV3
	if string(base) == string(corrected) || cat.DecodeNativeV3(corrected, &catalog) != nil || catalog.Authority != 0 || catalog.Accepted || catalog.Feature != "UNRESOLVED" || catalog.Consumer != "UNRESOLVED" || catalog.Completeness != "UNKNOWN" || catalog.Absence != "PROHIBITED" {
		t.Fatal("ASSERT_CONFIGURATION_V3_CONTROLLED_CEILING", c.ID)
	}
	var proposed cat.NativeCatalogWireV3
	if cat.DecodeNativeV3(base, &proposed) != nil || catalog.Predecessor != proposed.Revision || catalog.Revision == proposed.Revision || catalog.Actor != q.ReviewerIdentity || catalog.Actor == proposed.Actor || catalog.ReviewIdentity != "NOT_AUTHENTICATED" || len(catalog.Evidence) != 1 || len(catalog.Groups) != 1 || len(catalog.Groups[0].Members) != 1 || catalog.Groups[0].Members[0] != catalog.Evidence[0].ID {
		t.Fatal("ASSERT_CONFIGURATION_V3_CORRECTION_BINDING", c.ID)
	}
	ev := catalog.Evidence[0]
	beforeEvidence, _ := json.Marshal(proposed.Evidence)
	afterEvidence, _ := json.Marshal(catalog.Evidence)
	if string(beforeEvidence) != string(afterEvidence) || ev.OriginalInputID != record.OriginalInputID || ev.Selected.Mode != c.Mode || ev.Selected.Intent.Endpoint.Kind != c.Kind || ev.Selected.Intent.Endpoint.Role != c.Role || ev.Unit.InputID != "" || ev.Unit.ID != record.AdmissionID || ev.Request.RecordID != record.RequestID || ev.Request.Envelope.AdmissionID != ev.Unit.ID || ev.Request.Lineage.PacketID != ev.Unit.ID || len(ev.Record) == 0 || ev.RawAdapterDigest == "" {
		t.Fatal("ASSERT_CONFIGURATION_V3_PERSISTED_NATIVE_BINDING", c.ID)
	}
	for name, s := range out.Stages {
		if s.Requested != s.Successful+s.Failed {
			t.Fatal("ASSERT_CONFIGURATION_V3_STAGES", c.ID, name, s)
		}
	}
	record.Actual = "PUBLISHED_PROVISIONAL"
	t.Log("ASSERT_CONFIGURATION_V3_NATIVE_POSITIVE: PASS", c.ID)
}
func configurationModesV3() []string { return []string{"C", "R", "D", "CR", "CD", "RD", "CRD"} }
func TestConfigurationV3LedgerContract(t *testing.T) {
	modes := configurationModesV3()
	if strings.Join(modes, ",") != "C,R,D,CR,CD,RD,CRD" {
		t.Fatal("ASSERT_CONFIGURATION_V3_SEVEN_CONFIGURATION_LEDGER", modes)
	}
	if configurationFullLayerV3 != "CONTROLLED_NATIVE.V3_FULL" || configurationLayerAllowedV3("REAL_APPLICATION") || configurationLayerAllowedV3("REAL_APPLICATION.V3_FULL") {
		t.Fatal("ASSERT_CONFIGURATION_V3_EVIDENCE_LAYER", configurationFullLayerV3)
	}
	alias, _ := json.Marshal(configurationKindsV3("RCD"))
	canonical, _ := json.Marshal(configurationKindsV3("CRD"))
	if string(alias) != string(canonical) {
		t.Fatal("ASSERT_CONFIGURATION_V3_ALIAS_NOT_EIGHTH")
	}
	for _, mode := range modes {
		rec := configurationRecordV3{CaseID: "contract-configuration-" + mode, Configuration: mode, Requested: configurationKindsV3(mode), Direction: "CONTRACT_ONLY", Role: "no native evidence", Multiplicity: "canonical requested kind set", Layer: "CONTRACT.COVERAGE", Obligations: []string{"seven-configuration-ledger", "RCD-alias", "no-live-promotion"}, Expected: "EXACT_CLOSED_CONFIGURATION", Actual: "EXACT_CLOSED_CONFIGURATION; CONTRACT ONLY"}
		configurationEmitV3(t, &rec)
	}
	t.Log("ASSERT_CONFIGURATION_V3_SEVEN_CONFIGURATION_LEDGER: PASS")
}
func TestConfigurationV3Campaign(t *testing.T) {
	for _, mode := range configurationModesV3() {
		t.Run(mode, func(t *testing.T) {
			kind, role := "RESOLVES_TO_DEFINITION", "DEFINITION_TARGET"
			if !strings.Contains(mode, "D") {
				kind, role = "REFERENCES_SYMBOL", "REFERENCING_OCCURRENCE"
			}
			if mode == "C" {
				kind, role = "CALLS", "CALLEE_TARGET"
			}
			configurationPositiveV3(t, configurationCaseV3{ID: "positive-" + mode, Mode: mode, Kind: kind, Role: role, Fixture: configurationFixtureV3{Incoming: "empty", Outgoing: "one", References: "one", Definitions: "one"}})
		})
	}
}
func configurationKindsV3(mode string) []pa.RelationKind {
	if mode == "RCD" {
		mode = "CRD"
	} // spelling alias only, never an eighth native configuration
	var kinds []pa.RelationKind
	for _, ch := range mode {
		switch ch {
		case 'C':
			kinds = append(kinds, pa.TypedCalls)
		case 'R':
			kinds = append(kinds, pa.TypedReferencesSymbol)
		case 'D':
			kinds = append(kinds, pa.TypedResolvesToDefinition)
		}
	}
	return kinds
}
func configurationCountV3(t *testing.T, id string) {
	t.Helper()
	path := os.Getenv("CONFIGURATION_V3_COUNT_FILE")
	if path != "" {
		old, e := os.ReadFile(path)
		if e != nil && !os.IsNotExist(e) {
			t.Fatal(e)
		}
		// Reserve16 of128 for the unchanged historical fixture tests in writer gates.
		if strings.Count(string(old), "\n") >= 112 {
			t.Fatal("controlled preparation ceiling; reserved old-fixture allowance exhausted")
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := json.Marshal(map[string]string{"run": os.Getenv("CONFIGURATION_V3_RUN"), "case": id, "event": "CONTROLLED_PREPARATION_ATTEMPT"})
		_, e = f.Write(append(raw, '\n'))
		ce := f.Close()
		if e != nil || ce != nil {
			t.Fatal(e, ce)
		}
	}
	t.Log("CONFIGURATION_NATIVE_PREPARATION", id)
}
func configurationInputsV3(t *testing.T, c configurationCaseV3) (*inv.PrivateManagedInputOwnerV2, inv.PrivateMixedInputRequestV2, inv.PrivateSelectedConfigurationV2, string) {
	t.Helper()
	manager, start, uri := mixedStartFixtureV2(t)
	configurationCountV3(t, c.ID)
	owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
	if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\nfunc Third(){ Root() }\n"), 0644); e != nil {
		t.Fatal(e)
	}
	events := filepath.Join(t.TempDir(), "provider-events.jsonl")
	knobs, _ := json.Marshal(c.Fixture)
	start.Process.Args = []string{"-test.run=^TestConfigurationV3SELFResponder$", "--", "configuration-self-v3", start.Profile.Workspace().String(), string(knobs), events}
	q := inv.PrivateMixedInputRequestV2{Manager: manager, Root: root, Start: start, URI: uri, OccurrenceID: "configuration-v3", Revision: "controlled-configuration-v3", Privacy: "configuration-fixture", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: configurationKindsV3(c.Mode)}
	d := "sha256:" + strings.Repeat("1", 64)
	input := cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "controlled-source", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, e := cc.NewContinuationContract(input)
	if e != nil {
		t.Fatal(e)
	}
	hash := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
	config := inv.PrivateSelectedConfigurationV2{RunID: "configuration-" + c.ID, SeedNamespace: "configuration-v3", Lineage: "controlled-" + c.ID, Mode: c.Mode, Seed: 7, Attempts: [3]string{hash(c.ID + "-M"), hash(c.ID + "-P"), hash(c.ID + "-S")}, Contract: contract, ContractInput: input, SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}}
	t.Cleanup(func() {
		ctx, cancel := context.WithDeadline(context.Background(), q.Start.Deadline)
		defer cancel()
		manager.Shutdown(ctx)
		if manager.Census().Workers != 0 {
			t.Error("ASSERT_CONFIGURATION_V3_NATIVE_CLEANUP")
		}
	})
	return owner, q, config, events
}

// The exact existing public fixed M/P/S sequence; Begin is never inspected or
// replayed. This probe establishes whether nil selection can express the request.
func configurationGenericV3(ctx context.Context, owner *inv.PrivateManagedInputOwnerV2, q inv.PrivateMixedInputRequestV2, c inv.PrivateSelectedConfigurationV2) (sc.PrivateSelectedResultV2, error) {
	ctx, cancel := context.WithDeadline(ctx, q.Start.Deadline)
	defer cancel()
	plan, e := inv.NewPrivateSelectedPreparationPlanV2(owner, q, c)
	if e != nil {
		return sc.PrivateSelectedResultV2{}, e
	}
	m, e := owner.ReserveSelectedMixedInputV2(ctx, c.Attempts[0], q, plan)
	if e != nil {
		return sc.PrivateSelectedResultV2{}, e
	}
	captured, e := inv.ExecuteSelectedMixedInputV2(ctx, m, q, plan)
	if e != nil {
		return sc.PrivateSelectedResultV2{}, e
	}
	p, e := owner.ReserveSelectedPreparationV2(ctx, captured)
	if e != nil {
		return sc.PrivateSelectedResultV2{}, e
	}
	prepared, e := inv.ExecuteSelectedPreparationV2(ctx, p)
	if e != nil {
		return sc.PrivateSelectedResultV2{}, e
	}
	s, e := owner.ReserveSelectedSourceV2(ctx, prepared)
	if e != nil {
		return sc.PrivateSelectedResultV2{}, e
	}
	return sc.PrepareSelectedV2(ctx, s, nil)
}
func TestConfigurationV3Dimensions(t *testing.T) {
	baseline := configurationFixtureV3{Incoming: "empty", Outgoing: "one", References: "one", Definitions: "one"}
	cases := []configurationCaseV3{
		{ID: "role-C-caller-out", Mode: "C", Kind: "CALLS", Role: "CALLER_OCCURRENCE", Fixture: baseline},
		{ID: "role-R-referenced", Mode: "R", Kind: "REFERENCES_SYMBOL", Role: "REFERENCED_SYMBOL", Fixture: baseline},
		{ID: "role-D-query", Mode: "D", Kind: "RESOLVES_TO_DEFINITION", Role: "QUERY_OCCURRENCE", Fixture: baseline},
		{ID: "shared-span-RD-reference", Mode: "RD", Kind: "REFERENCES_SYMBOL", Role: "REFERENCING_OCCURRENCE", Fixture: baseline},
	}
	incoming := baseline
	incoming.Incoming = "one"
	incoming.Outgoing = "empty"
	cases = append(cases, configurationCaseV3{ID: "direction-C-in-caller", Mode: "C", Kind: "CALLS", Role: "CALLER_OCCURRENCE", Fixture: incoming}, configurationCaseV3{ID: "direction-C-in-callee", Mode: "C", Kind: "CALLS", Role: "CALLEE_TARGET", Fixture: incoming})
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) { configurationPositiveV3(t, c) })
	}
}
func TestConfigurationV3Ambiguity(t *testing.T) {
	cases := []configurationCaseV3{
		{ID: "C-composed-ambiguous", Mode: "C", Kind: "CALLS", Role: "CALLEE_TARGET", Fixture: configurationFixtureV3{Incoming: "one", Outgoing: "one", References: "one", Definitions: "one"}},
		{ID: "R-multiple-ambiguous", Mode: "R", Kind: "REFERENCES_SYMBOL", Role: "REFERENCING_OCCURRENCE", Fixture: configurationFixtureV3{Incoming: "empty", Outgoing: "one", References: "multiple", Definitions: "one"}},
		{ID: "R-duplicates-ambiguous", Mode: "R", Kind: "REFERENCES_SYMBOL", Role: "REFERENCING_OCCURRENCE", Fixture: configurationFixtureV3{Incoming: "empty", Outgoing: "one", References: "duplicates", Definitions: "one"}},
	}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			rec := configurationRecordV3{CaseID: c.ID, Configuration: c.Mode, Requested: configurationKindsV3(c.Mode), Direction: "composed if C; otherwise NOT_REQUESTED", Role: c.Role, Multiplicity: "multiple/duplicate eligible identities", ExpectedFamilyState: configurationExpectedV3(c), Layer: "CONTROLLED_NATIVE.SOURCE_PREPARATION_REFUSAL", Obligations: []string{"no-first-row-fallback", "multiple-match-refusal"}, Expected: "SELECTED_ROW_REFUSED", Actual: "NOT_COMPLETED", Authority: "fresh isolated controlled owner"}
			defer configurationEmitV3(t, &rec)
			owner, q, config, _ := configurationInputsV3(t, c)
			selected, e := so.PrepareSelectedRoleV3(context.Background(), owner, q, config, sc.PrivateSelectedRoleV3{Kind: c.Kind, Role: c.Role})
			if !errors.Is(e, sc.ErrPrivateSelectedRowV2) || selected.Available() || q.Manager.Census().Workers != 0 {
				t.Fatal("ASSERT_CONFIGURATION_V3_AMBIGUITY_REFUSED", c.ID, e, selected.Available())
			}
			rec.Actual = "SELECTED_ROW_REFUSED; no opaque result, HOST, membership or publication"
			t.Log("ASSERT_CONFIGURATION_V3_AMBIGUITY_REFUSED: PASS", c.ID)
		})
	}
}
func configurationProviderCasesV3() []configurationCaseV3 {
	base := configurationFixtureV3{Incoming: "empty", Outgoing: "one", References: "one", Definitions: "one"}
	var cases []configurationCaseV3
	for _, mode := range []string{"C", "R", "D", "CR", "CD", "RD", "CRD"} {
		cases = append(cases, configurationCaseV3{ID: "provider-" + mode, Mode: mode, Fixture: base})
	}
	for _, mode := range []string{"C", "R", "D"} {
		for _, state := range []string{"empty", "unavailable"} {
			f := base
			switch mode {
			case "C":
				f.Incoming = state
				f.Outgoing = state
			case "R":
				f.References = state
			case "D":
				f.Definitions = state
			}
			cases = append(cases, configurationCaseV3{ID: "provider-" + mode + "-" + state, Mode: mode, Fixture: f})
		}
	}
	for _, name := range []string{"RD-R-empty", "RD-R-unavailable", "RD-D-empty", "CR-C-empty", "CR-C-unavailable"} {
		f := base
		mode := "RD"
		switch name {
		case "RD-R-empty":
			f.References = "empty"
		case "RD-R-unavailable":
			f.References = "unavailable"
		case "RD-D-empty":
			f.Definitions = "empty"
		case "CR-C-empty":
			mode = "CR"
			f.Outgoing = "empty"
		case "CR-C-unavailable":
			mode = "CR"
			f.Incoming = "unavailable"
			f.Outgoing = "unavailable"
		}
		cases = append(cases, configurationCaseV3{ID: "provider-mixed-" + name, Mode: mode, Fixture: f})
	}
	for _, state := range []string{"multiple", "duplicates"} {
		f := base
		f.References = state
		cases = append(cases, configurationCaseV3{ID: "provider-R-" + state, Mode: "R", Fixture: f})
	}
	f := base
	f.Incoming = "duplicates"
	f.Outgoing = "duplicates"
	cases = append(cases, configurationCaseV3{ID: "provider-C-composed-duplicates", Mode: "C", Fixture: f})
	return cases
}
func TestConfigurationV3ProviderStates(t *testing.T) {
	for _, c := range configurationProviderCasesV3() {
		t.Run(c.ID, func(t *testing.T) {
			rec := configurationRecordV3{CaseID: c.ID, Configuration: c.Mode, Requested: configurationKindsV3(c.Mode), Direction: "outgoing-only", Role: "native occurrence ledger, not selected source role", Multiplicity: "zero/one/multiple/duplicates per literal fixture", ExpectedFamilyState: configurationExpectedV3(c), ObservedFamilyState: map[string]string{}, Layer: "CONTROLLED_NATIVE.PROVIDER", Obligations: []string{"native-provider-state", "not-requested", "multiplicity"}, Expected: "EXACT_NATIVE_STATE_OR_TYPED_REFUSAL", Actual: "NOT_COMPLETED", Authority: "fresh isolated controlled owner"}
			defer configurationEmitV3(t, &rec)
			if !strings.Contains(c.Mode, "C") {
				rec.Direction = "NOT_REQUESTED"
			}
			if c.Fixture.Incoming != "empty" && c.Fixture.Incoming != "unavailable" {
				rec.Direction = "composed incoming+outgoing"
			}
			owner, q, _, events := configurationInputsV3(t, c)
			ctx, cancel := context.WithDeadline(context.Background(), q.Start.Deadline)
			defer cancel()
			op, e := owner.ReserveMixedInputV2(ctx, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(c.ID))), q)
			if e != nil {
				t.Fatal("provider reserve prerequisite", e)
			}
			input, e := inv.ExecuteMixedInputV2(ctx, op, q)
			rec.NativeReturnWorkers = q.Manager.Census().Workers
			if e == nil && rec.NativeReturnWorkers != 0 {
				t.Fatal("ASSERT_CONFIGURATION_V3_PROVIDER_CLEANUP", c.ID, e)
			}
			if e != nil {
				codes := inv.ProjectPrivatePrepareFailureV1(e)
				rec.NativeDiagnostic = codes
				if rec.NativeReturnWorkers != 0 {
					q.Manager.Shutdown(ctx)
				}
				rec.FixtureCleanupWorkers = q.Manager.Census().Workers
				if rec.FixtureCleanupWorkers != 0 {
					t.Fatal("ASSERT_CONFIGURATION_V3_PROVIDER_CLEANUP", c.ID, e)
				}
				rec.Layer = "CONTROLLED_NATIVE.CAPTURE_REFUSAL"
				rec.Actual = fmt.Sprintf("%s/%s; native workers=%d; separate fixture cleanup=%d", codes.Stage.String(), codes.Reason.String(), rec.NativeReturnWorkers, rec.FixtureCleanupWorkers)
				if codes.Stage.String() != "CAPTURE" && codes.Stage.String() != "READINESS" {
					t.Fatal("ASSERT_CONFIGURATION_V3_NATIVE_STATE", c.ID, e, codes)
				}
				if _, originalErr := input.SourceOriginalsV1(); originalErr == nil {
					t.Fatal("ASSERT_CONFIGURATION_V3_NO_SYNTHETIC_ADMISSION", c.ID)
				}
				known := false
				if rec.NativeReturnWorkers == 0 && len(q.Kinds) == 1 && errors.Is(e, inv.ErrPrivateMixedInputEmptyV2) {
					rec.ObservedFamilyState[string(q.Kinds[0])] = "OBSERVED_EMPTY"
					known = true
				}
				if rec.NativeReturnWorkers == 0 && len(q.Kinds) == 1 && errors.Is(e, inv.ErrPrivateMixedInputUnavailableV2) {
					rec.ObservedFamilyState[string(q.Kinds[0])] = "UNAVAILABLE"
					known = true
				}
				if known {
					for kind, actual := range rec.ObservedFamilyState {
						if actual != rec.ExpectedFamilyState[kind] {
							t.Fatal("ASSERT_CONFIGURATION_V3_DISTINCT_NATIVE_STATES", c.ID, kind, actual, rec.ExpectedFamilyState[kind])
						}
					}
				}
				if !known {
					rec.Disposition = "BLOCKED"
					rec.Gap = "Native refusal has no publicly retained per-family state; no fixture-emission/driver-status substitution. No source admission/HOST/publication."
				}
				t.Log("ASSERT_CONFIGURATION_V3_NATIVE_STATE: PASS", c.ID, rec.Actual, "known per-family", known)
				return
			}
			originals, e := input.SourceOriginalsV1()
			if e != nil || originals.InputID() == "" {
				t.Fatal("native confirmed source-original prerequisite", e)
			}
			rec.OriginalInputID = originals.InputID()
			rec.Authority = "CONTROLLED_RESOURCE_HEAD:" + owner.PreparationHeadV1()
			counts := map[string]int{}
			ids := map[string]bool{}
			for _, row := range input.TypedInput.Occurrences {
				if ids[row.Identity] {
					t.Fatal("ASSERT_CONFIGURATION_V3_MULTIPLICITY_IDENTITIES", c.ID)
				}
				ids[row.Identity] = true
				counts[string(row.Kind)]++
			}
			for kind, want := range rec.ExpectedFamilyState {
				actual := "NOT_REQUESTED"
				if want != "NOT_REQUESTED" {
					method := kind
					if kind == "REFERENCES_SYMBOL" {
						method = "textDocument/references"
					}
					if kind == "RESOLVES_TO_DEFINITION" {
						method = "textDocument/definition"
					}
					actual = input.Dispositions[method]
					if kind == "CALLS" {
						actual = "OBSERVED_EMPTY"
						if counts[kind] > 0 {
							actual = "OBSERVED_NONEMPTY"
						}
					}
				}
				rec.ObservedFamilyState[kind] = actual
				if actual != want {
					t.Fatal("ASSERT_CONFIGURATION_V3_NATIVE_STATE", c.ID, kind, actual, want, input.Dispositions, counts)
				}
			}
			if c.Fixture.References == "multiple" || c.Fixture.References == "duplicates" {
				if counts["REFERENCES_SYMBOL"] != 2 {
					t.Fatal("ASSERT_CONFIGURATION_V3_MULTIPLICITY_IDENTITIES", c.ID, counts)
				}
			}
			if c.ID == "provider-C-composed-duplicates" {
				if err := configurationDuplicateCanonicalV3(input.TypedInput); err != nil {
					t.Fatal("ASSERT_CONFIGURATION_V3_MULTIPLICITY_IDENTITIES", c.ID, err)
				}
				if len(input.Ledger) < 2 {
					t.Fatal("composed edge prerequisite")
				}
			}
			if !strings.Contains(c.Mode, "C") {
				wire, _ := os.ReadFile(events)
				if strings.Contains(string(wire), "callHierarchy/") || strings.Contains(string(wire), "prepareCallHierarchy") {
					t.Fatal("ASSERT_CONFIGURATION_V3_NOT_REQUESTED", c.ID)
				}
			}
			rec.Actual = "NATIVE_PROVIDER_STATES_VERIFIED; no source-selection/Describe claim"
			t.Log("ASSERT_CONFIGURATION_V3_NATIVE_STATE: PASS", c.ID)
			t.Log("CONFIGURATION_NATIVE_LEDGER", c.ID, counts)
		})
	}
}

func TestConfigurationV3ExistingResponderEmptyComparison(t *testing.T) {
	c := configurationCaseV3{ID: "existing-responder-R-empty", Mode: "R", Fixture: configurationFixtureV3{References: "empty"}}
	owner, q, _, _ := configurationInputsV3(t, c)
	q.Start.Process.Args = []string{"-test.run=^TestPrivateInventoryMixedLeaseReplayV2SELFResponder$", "--", "mixed-lease-self-v2", q.Start.Profile.Workspace().String(), "empty"}
	ctx, cancel := context.WithDeadline(context.Background(), q.Start.Deadline)
	defer cancel()
	op, e := owner.ReserveMixedInputV2(ctx, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(c.ID))), q)
	if e != nil {
		t.Fatal(e)
	}
	input, e := inv.ExecuteMixedInputV2(ctx, op, q)
	codes := inv.ProjectPrivatePrepareFailureV1(e)
	before := q.Manager.Census()
	cancel()
	cleanup, stop := context.WithDeadline(context.Background(), q.Start.Deadline)
	defer stop()
	q.Manager.Shutdown(cleanup)
	_, originalErr := input.SourceOriginalsV1()
	t.Logf("EXISTING_RESPONDER_EMPTY_DIAGNOSTIC error=%#v codes=%+v native=%+v after_cancel=%+v source_original_err=%v", e, codes, before, q.Manager.Census(), originalErr)
	if q.Manager.Census().Workers != 0 {
		t.Fatal("ASSERT_CONFIGURATION_V3_EXISTING_EMPTY_JOIN", codes)
	}
}
func TestConfigurationV3CallerRefusals(t *testing.T) {
	for _, id := range []string{"foreign-kind", "foreign-role", "changed-configuration"} {
		t.Run(id, func(t *testing.T) {
			c := configurationCaseV3{ID: "caller-" + id, Mode: "R", Kind: "REFERENCES_SYMBOL", Role: "REFERENCING_OCCURRENCE", Fixture: configurationFixtureV3{Incoming: "empty", Outgoing: "one", References: "one", Definitions: "one"}}
			rec := configurationRecordV3{CaseID: c.ID, Configuration: c.Mode, Requested: configurationKindsV3(c.Mode), Direction: "NOT_REQUESTED", Role: c.Role, Multiplicity: "not acquired", Layer: "CONTRACT.CALLER_GUARD", Obligations: []string{"foreign-kind-role-configuration", "no-effects-before-M"}, Expected: "CALLER_INPUT_REFUSED_NO_NATIVE_EFFECT", Actual: "NOT_COMPLETED", Authority: "fresh owner but no native authority consumed"}
			defer configurationEmitV3(t, &rec)
			owner, q, config, events := configurationInputsV3(t, c)
			role := sc.PrivateSelectedRoleV3{Kind: c.Kind, Role: c.Role}
			switch id {
			case "foreign-kind":
				role = sc.PrivateSelectedRoleV3{Kind: "CALLS", Role: "CALLEE_TARGET"}
			case "foreign-role":
				role.Role = "DEFINITION_TARGET"
			case "changed-configuration":
				config.Mode = "C"
			}
			head := owner.PreparationHeadV1()
			result, e := so.PrepareSelectedRoleV3(context.Background(), owner, q, config, role)
			if e == nil || result.Available() || head != owner.PreparationHeadV1() || q.Manager.Census().Workers != 0 {
				t.Fatal("ASSERT_CONFIGURATION_V3_CALLER_REFUSAL", id, e)
			}
			if _, e := os.Stat(events); !os.IsNotExist(e) {
				t.Fatal("ASSERT_CONFIGURATION_V3_CALLER_NO_NATIVE_EFFECT", id, e)
			}
			rec.Actual = "CALLER_INPUT_REFUSED_NO_NATIVE_EFFECT"
			t.Log("ASSERT_CONFIGURATION_V3_CALLER_REFUSAL: PASS", id)
		})
	}
}
func TestConfigurationV3LegacyNilRefuses(t *testing.T) {
	rec := configurationRecordV3{CaseID: "legacy-nil-RD-ambiguity", Configuration: "RD", Requested: configurationKindsV3("RD"), Direction: "NOT_REQUESTED", Role: "nil legacy criterion", Multiplicity: "shared-span R/D identities", Layer: "CONTROLLED_NATIVE.SOURCE_PREPARATION_REFUSAL", Obligations: []string{"legacy-nil-no-first-fallback"}, Expected: "SELECTED_ROW_REFUSED", Actual: "NOT_COMPLETED", Authority: "fresh controlled owner"}
	defer configurationEmitV3(t, &rec)
	c := configurationCaseV3{ID: "generic-RD", Mode: "RD", Kind: "RESOLVES_TO_DEFINITION", Role: "DEFINITION_TARGET", Fixture: configurationFixtureV3{Incoming: "empty", Outgoing: "one", References: "one", Definitions: "one"}}
	owner, q, config, events := configurationInputsV3(t, c)
	result, e := configurationGenericV3(context.Background(), owner, q, config)
	if e != nil {
		raw, _ := os.ReadFile(events)
		t.Logf("GENERIC_SELECTION_OBSERVATION error=%#v events=%s census=%+v", e, raw, q.Manager.Census())
	}
	if !errors.Is(e, sc.ErrPrivateSelectedRowV2) || result.Available() {
		t.Fatal("ASSERT_CONFIGURATION_V3_LEGACY_NIL_AMBIGUITY", e)
	}
	rec.Actual = "SELECTED_ROW_REFUSED"
	t.Log("ASSERT_CONFIGURATION_V3_LEGACY_NIL_AMBIGUITY: PASS")
}
