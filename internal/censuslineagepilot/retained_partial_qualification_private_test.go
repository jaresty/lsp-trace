//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/schema"
)

func zeroDerivedPrivate[T any](_ T) T { var z T; return z }

func TestRetainedPartialVerifierSourcePinPrivate(t *testing.T) {
	raw, e := os.ReadFile("retained_partial_qualification_private.go")
	if e != nil {
		t.Fatal(e)
	}
	if pilot.Digest(raw) != pilot.RetainedPartialQualificationVerifierPinPrivate() {
		t.Fatal("ASSERT_VERIFIER_SOURCE_PIN_STALE")
	}
}

func TestRetainedPartialQualificationPrivate(t *testing.T) {
	testRetainedPartialQualificationPrivate(t, false)
}
func TestRetainedPartialQualificationNonDefaultContextPrivate(t *testing.T) {
	testRetainedPartialQualificationPrivate(t, true)
}
func testRetainedPartialQualificationPrivate(t *testing.T, nondefault bool) {
	h, m, cfg, f := pilot.NewScopedPartialExact25V4RehearsalFixturePrivate(t)
	if nondefault {
		h.SetNonDefaultRetainedContextPrivate()
	}
	report, e := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if len(report.Records) != 7 || report.OutcomeReceipt == nil || !report.OutcomeReceipt.Verified {
		t.Fatal("owned producer did not retain eight objects")
	}
	var inputs [3]*publication.ContentRetentionRoot
	var outputs [2]*publication.ContentRetentionRoot
	for i, path := range cfg.InputRoots {
		inputs[i], e = publication.OpenContentRetentionRoot(context.Background(), path)
		if e != nil {
			t.Fatal(e)
		}
		defer inputs[i].Close()
	}
	for i, path := range cfg.OutputRoots[:2] {
		outputs[i], e = publication.OpenContentRetentionRoot(context.Background(), path)
		if e != nil {
			t.Fatal(e)
		}
		defer outputs[i].Close()
	}
	pins := pilot.RetainedPartialQualificationPinsPrivate{Schema: "lsp-trace.retained-partial-qualification-input.private.v1", CodeHEAD: cfg.CodeHEAD, VerifierSourceSHA256: pilot.RetainedPartialQualificationVerifierPinPrivate(), PolicySHA256: pilot.RetainedPartialQualificationPolicyDigestPrivate(), MaxReadBytes: 128 << 20, MaxWorkBytes: 2 << 30}
	records := append(append([]pilot.ScopedRehearsalRecordPrivate{}, report.Records...), *report.OutcomeReceipt)
	var original [8][]byte
	for i, r := range records {
		pins.Objects[i] = pilot.RetainedPartialObjectPinPrivate{Name: r.Name, ID: r.ID, Bytes: uint64(r.Bytes)}
		original[i], e = outputs[0].ReadPartialInspectionV2Private(context.Background(), r.ID, int64(r.Bytes))
		if e != nil {
			t.Fatal(e)
		}
	}
	before, prepares := f.Traffic()
	run := func(p pilot.RetainedPartialQualificationPinsPrivate) ([]byte, error) {
		b, _ := json.Marshal(p)
		in, e := pilot.NewExactRetainedPartialQualificationInput(b, inputs, outputs)
		if e != nil {
			return nil, e
		}
		q, e := pilot.QualifyRetainedPartialNativePrivate(context.Background(), in)
		return q.ReportBytes(), e
	}
	good, e := run(pins)
	if e != nil {
		t.Fatal("ASSERT_PAIRED_PRIVATE_INSPECTION", e)
	}
	var observed struct {
		Status            string   `json:"status"`
		Targets           int      `json:"targets"`
		Authority         int      `json:"authority"`
		Accepted          bool     `json:"accepted"`
		Partial           bool     `json:"partial"`
		CanonicalComplete bool     `json:"canonical_complete"`
		Missing           []string `json:"missing_native_obligations"`
	}
	if json.Unmarshal(good, &observed) != nil || observed.Status != "INSPECTION_QUALIFIED_NATIVE_BLOCKED_NATIVE_CUSTODY" || observed.Targets != 25 || observed.Authority != 0 || observed.Accepted || !observed.Partial || observed.CanonicalComplete || len(observed.Missing) != 4 {
		t.Fatalf("ASSERT_NATIVE_CEILING %s", good)
	}
	again, e := run(pins)
	if e != nil || !bytes.Equal(good, again) {
		t.Fatal("ASSERT_DETERMINISTIC_READONLY", e)
	}
	// Returning bytes is detached, and copying an inspection result grants nothing.
	rawPins, _ := json.Marshal(pins)
	in, e := pilot.NewExactRetainedPartialQualificationInput(rawPins, inputs, outputs)
	if e != nil {
		t.Fatal(e)
	}
	q, e := pilot.QualifyRetainedPartialNativePrivate(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	t.Run("derived-private-native-input", func(t *testing.T) {
		budget := pilot.RetainedPartialDerivationBudgetPrivate{MaxInputBytes: 8 << 20, MaxOutputBytes: 8 << 20, MaxWorkBytes: 2 << 30}
		derived, err := pilot.DeriveRetainedPartialNativePrivate(context.Background(), q, pins, budget)
		if err != nil {
			t.Fatal("ASSERT_ACTUAL_DERIVED_NATIVE_INPUT", err)
		}
		parsed, err := pilot.ParseDerivedRetainedPartialNativePrivate(derived.Bytes())
		if err != nil {
			t.Fatal("ASSERT_PRIVATE_MANIFEST_VALIDATES", err)
		}
		manifest := parsed.Manifest
		if manifest.Profile != pilot.DerivedRetainedPartialPrivateV1 || manifest.Status != "VALIDATED_PRIVATE_INPUT_MANIFEST" || manifest.ProgramCAdmission != "NOT_PROGRAM_C_ADMITTED" || manifest.ReplayAdmitted || manifest.OriginalNativeV5IdentityClaimed || manifest.Accepted || manifest.Authority != 0 || manifest.CanonicalComplete || !manifest.Partial || manifest.SourceCompleteness != "UNKNOWN" || manifest.FeatureIdentity != "UNRESOLVED" || manifest.Targets != 25 || manifest.SourcePolicySHA256 == "" || manifest.CanonicalSeedsSHA256 == "" || len(manifest.SourcePolicy.Members) != 6 || manifest.OriginalAcquisitionGraphSHA256 == manifest.CanonicalNativeGraphSHA256 || pilot.Digest(parsed.NativeGraph) != manifest.CanonicalNativeGraphSHA256 {
			t.Fatal("ASSERT_DERIVED_LINEAGE_AND_CEILINGS")
		}
		policyBytes, _ := json.Marshal(manifest.SourcePolicy)
		if manifest.Origin != "DERIVED_RETAINED_INSPECTION" || pilot.Digest(policyBytes) != manifest.SourcePolicySHA256 || pilot.Digest(manifest.CanonicalSeeds) != manifest.CanonicalSeedsSHA256 || manifest.SourceCarrierSHA256 != pilot.Digest(original[5]) || manifest.CallerAssertedCodeHEAD != cfg.CodeHEAD || manifest.VerifierSourcePin != pins.VerifierSourceSHA256 || manifest.ProviderCommand != cfg.Server || manifest.SessionID != cfg.Session.SessionID || manifest.Generation != cfg.Session.Generation {
			t.Fatal("ASSERT_EXACT_PRIVATE_INPUT_BINDINGS")
		}
		carrier, err := graphprovenance.ParseSourceWithholdingPrivateV1(parsed.Records[5])
		if err != nil || carrier.ReplayAdmitted || manifest.WithheldCount != carrier.WithheldCount || !bytes.Equal(parsed.Records[5], original[5]) || !bytes.Equal(parsed.Records[4], original[4]) || !bytes.Equal(parsed.Records[6], original[6]) {
			t.Fatal("ASSERT_SOURCE_RAW_BINDINGS_PRESERVED", err)
		}
		// Compare original full native metadata, not Result's narrower decode model.
		type nativePreservedFields struct {
			ProcessContext    graph.ProcessContext   `json:"process_context"`
			ExecutionBundleID string                 `json:"execution_bundle_id"`
			Nodes             []graph.Node           `json:"nodes"`
			Edges             []graph.Edge           `json:"edges"`
			Targets           []string               `json:"targets"`
			Frontier          []graph.Boundary       `json:"frontier"`
			Terminals         []graph.Boundary       `json:"terminals"`
			Seeds             []graph.SeedResult     `json:"seeds"`
			Memberships       []graph.SeedMembership `json:"seed_memberships"`
			Slice             *graph.SliceEvidence   `json:"slice"`
			EvidenceReceipt   *graph.EvidenceReceipt `json:"evidence_receipt"`
		}
		var sourceFields, derivedFields nativePreservedFields
		if json.Unmarshal(carrier.Evidence.GraphBytes, &sourceFields) != nil || json.Unmarshal(parsed.NativeGraph, &derivedFields) != nil {
			t.Fatal("full native projection decode")
		}
		if nondefault && (sourceFields.ProcessContext.EffectiveEnvironmentVariableCount != 2 || len(sourceFields.ProcessContext.Environment) != 1 || sourceFields.ProcessContext.Environment[0].Name != "FIXTURE_TOKEN") {
			t.Fatal("ASSERT_ACTUAL_CAPTURE_NONDEFAULT_CONTEXT")
		}
		if !reflect.DeepEqual(sourceFields.ProcessContext, derivedFields.ProcessContext) {
			t.Fatal("ASSERT_QUALIFIED_RETAINED_PROCESS_CONTEXT_EXACT")
		}
		// Workspace/provider enrichment changes only execution-bundle references;
		// CALLS and membership IDs and all occurrence/order/endpoint fields stay exact.
		for i := range sourceFields.Edges {
			sourceFields.Edges[i].ExecutionBundleID = derivedFields.ExecutionBundleID
		}
		for i := range sourceFields.Memberships {
			sourceFields.Memberships[i].ExecutionBundleID = derivedFields.ExecutionBundleID
		}
		sourceFields.ExecutionBundleID = derivedFields.ExecutionBundleID
		if !reflect.DeepEqual(sourceFields, derivedFields) {
			t.Fatal("ASSERT_QUALIFIED_NATIVE_FULL_FIELDS_PRESERVED")
		}
		if detected, err := schema.ValidateFor(parsed.NativeGraph, schema.FamilyGraph, "v5"); err != nil || detected != graph.SchemaVersionV5 {
			t.Fatal("ASSERT_DERIVED_GRAPH_V5_SCHEMA", err)
		}
		native, err := graph.DecodeNativeV3(parsed.NativeGraph)
		if err != nil || native.SchemaVersion != graph.SchemaVersionV5 || native.Summary.Complete || native.Invocation.Expansion.TopmostSiblings || native.Invocation.Expansion.TopmostSiblingOutcome != graph.TopmostSiblingNotRequested || len(native.SiblingCandidates) != 0 || !reflect.DeepEqual(native.Invocation.Seeds, carrier.Evidence.Acquisition.Graph.Invocation.Seeds) || !reflect.DeepEqual(native.Targets, carrier.Evidence.Acquisition.Graph.Targets) || len(native.Nodes) != manifest.Nodes || len(native.Edges) != manifest.Calls {
			t.Fatal("ASSERT_NATIVE_TYPED_PROJECTION_PRESERVES_TARGETS", err)
		}
		nativeSites := 0
		for _, edge := range native.Edges {
			nativeSites += len(edge.CallSites)
		}
		if nativeSites != manifest.CallSites {
			t.Fatal("ASSERT_NATIVE_OCCURRENCES_PRESERVED")
		}
		if _, err = graphprovenance.ValidateFor(derived.Bytes(), graphprovenance.Family, "v5"); err == nil {
			t.Fatal("ASSERT_ORDINARY_V5_STAYS_CLOSED")
		}
		if _, err = graphprovenance.CaptureV5(derived.NativeGraphBytes(), cfg.Session.SessionID, cfg.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &carrier.Evidence); err == nil || err.Error() != "V5 source snapshot must be admitted provenance v2" {
			t.Fatal("ASSERT_OLD_SOURCE_GATE_STAYS_CLOSED")
		}
		second, err := pilot.DeriveRetainedPartialNativePrivate(context.Background(), q, pins, budget)
		if err != nil || !bytes.Equal(derived.Bytes(), second.Bytes()) {
			t.Fatal("ASSERT_DERIVATION_DETERMINISTIC", err)
		}
		mutated := derived.Bytes()
		mutated[0] = 'x'
		mutated = derived.NativeGraphBytes()
		mutated[0] = 'x'
		mutated = derived.ManifestBytes()
		mutated[0] = 'x'
		parsed.Manifest.SourcePolicy.Members[0].URI = "changed"
		parsed.Manifest.CanonicalSeeds[0] = 'x'
		parsed.Records[5][0] = 'x'
		if !bytes.Equal(derived.Bytes(), second.Bytes()) {
			t.Fatal("ASSERT_DERIVED_ACCESSORS_DETACHED")
		}
		zero := zeroDerivedPrivate(q)
		if _, err = pilot.DeriveRetainedPartialNativePrivate(context.Background(), zero, pins, budget); err == nil {
			t.Fatal("ASSERT_ZERO_QUALIFIER_REFUSED")
		}
		if err = json.Unmarshal(q.ReportBytes(), &zero); err != nil {
			t.Fatal(err)
		}
		if _, err = pilot.DeriveRetainedPartialNativePrivate(context.Background(), zero, pins, budget); err == nil {
			t.Fatal("ASSERT_DECODED_QUALIFIER_REFUSED")
		}
		wrongPins := pins
		wrongPins.CodeHEAD = strings.Repeat("b", 40)
		if _, err = pilot.DeriveRetainedPartialNativePrivate(context.Background(), q, wrongPins, budget); err == nil {
			t.Fatal("ASSERT_MIXED_PINS_REFUSED")
		}
		for _, field := range []string{"input", "output", "work"} {
			b := budget
			switch field {
			case "input":
				b.MaxInputBytes = 1
			case "output":
				b.MaxOutputBytes = 1
			case "work":
				b.MaxWorkBytes = 1
			}
			if _, err = pilot.DeriveRetainedPartialNativePrivate(context.Background(), q, pins, b); err == nil {
				t.Fatal("ASSERT_DERIVATION_BUDGET", field)
			}
		}
		for _, field := range []string{"scope", "policy", "seed", "graph", "lineage", "provider", "code", "bundle-proof", "record"} {
			t.Run("substitution-"+field, func(t *testing.T) {
				p, err := pilot.ParseDerivedRetainedPartialNativePrivate(second.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				switch field {
				case "scope":
					p.Manifest.SourcePolicy.Members[0].URI += "x"
				case "policy":
					p.Manifest.SourcePolicySHA256 = pilot.Digest([]byte("wrong"))
				case "seed":
					p.Manifest.CanonicalSeeds[0] = 'x'
				case "graph":
					p.NativeGraph[0] = 'x'
				case "lineage":
					p.Manifest.OriginalAcquisitionGraphSHA256 = p.Manifest.CanonicalNativeGraphSHA256
				case "provider":
					p.Manifest.ProviderCommand += "x"
				case "code":
					p.Manifest.CallerAssertedCodeHEAD = strings.Repeat("b", 40)
				case "bundle-proof":
					p.QualificationReport[0] = 'x'
				case "record":
					p.Records[2][0] = 'x'
				}
				raw, _ := json.Marshal(p)
				if _, err = pilot.ParseDerivedRetainedPartialNativePrivate(raw); err == nil {
					t.Fatal("ASSERT_PRIVATE_INPUT_SUBSTITUTION_REFUSED", field)
				}
			})
		}
		for _, kind := range []string{"unknown", "duplicate", "case", "missing", "oversize"} {
			raw := second.Bytes()
			switch kind {
			case "unknown":
				raw = append([]byte(`{"extra":0,`), raw[1:]...)
			case "duplicate":
				raw = append([]byte(`{"schema":"x",`), raw[1:]...)
			case "case":
				raw = bytes.Replace(raw, []byte(`"schema"`), []byte(`"Schema"`), 1)
			case "missing":
				raw = bytes.Replace(raw, []byte(`"authority":0,`), nil, 1)
			case "oversize":
				raw = make([]byte, pilot.MaxDerivedRetainedPartialPrivateBytes+1)
			}
			if _, err = pilot.ParseDerivedRetainedPartialNativePrivate(raw); err == nil {
				t.Fatal("ASSERT_STRICT_PRIVATE_ENVELOPE", kind)
			}
		}
		t.Logf("ASSERT_ACTUAL_DERIVED_NATIVE_INPUT bytes=%d graph=%d targets=%d nodes=%d calls=%d sites=%d withheld=%d requests=%d", len(second.Bytes()), len(second.NativeGraphBytes()), manifest.Targets, manifest.Nodes, manifest.Calls, manifest.CallSites, manifest.WithheldCount, manifest.Requests)
	})
	if _, ok := any(q).(acquisition.Client); ok {
		t.Fatal("ASSERT_NO_NATIVE_CLIENT")
	}
	if _, ok := any(q).(censusacquisition.Acquirer); ok {
		t.Fatal("ASSERT_NO_NATIVE_ACQUIRER")
	}
	copyQ := q
	mut := q.ReportBytes()
	mut[0] = 'x'
	if !bytes.Equal(copyQ.ReportBytes(), good) {
		t.Fatal("ASSERT_DETACHED_REPORT")
	}
	var decoded struct{}
	if json.Unmarshal([]byte(`{}`), &decoded) != nil {
		t.Fatal("decode")
	}
	if raw, _ := json.Marshal(q); string(raw) != "{}" {
		t.Fatal("ASSERT_NO_SERIALIZABLE_CAPABILITY")
	}
	if zero, e := pilot.QualifyRetainedPartialNativePrivate(context.Background(), pilot.ExactRetainedPartialQualificationInput{}); e == nil || len(zero.ReportBytes()) != 0 {
		t.Fatal("ASSERT_ZERO_INPUT_REFUSED")
	}
	// Every retained object is read in BOTH copies, not merely named by hashes.
	for i, pin := range pins.Objects {
		t.Run("backup-"+pin.Name, func(t *testing.T) {
			path := filepath.Join(cfg.OutputRoots[1], pin.ID)
			if e := os.WriteFile(path, []byte("corrupt"), 0600); e != nil {
				t.Fatal(e)
			}
			_, err := run(pins)
			if e := os.WriteFile(path, original[i], 0600); e != nil {
				t.Fatal(e)
			}
			if err == nil {
				t.Fatal("ASSERT_EACH_BACKUP_READ")
			}
		})
	}
	for _, kind := range []string{"code", "verifier", "policy", "read-budget", "work-budget", "hash", "seed-label"} {
		t.Run(kind, func(t *testing.T) {
			p := pins
			switch kind {
			case "code":
				p.CodeHEAD = strings.Repeat("b", 40)
			case "verifier":
				p.VerifierSourceSHA256 = "sha256:" + strings.Repeat("0", 64)
			case "policy":
				p.PolicySHA256 = "sha256:" + strings.Repeat("0", 64)
			case "read-budget":
				p.MaxReadBytes = 1
			case "work-budget":
				p.MaxWorkBytes = 1
			case "hash":
				p.Objects[5].ID = "sha256:" + strings.Repeat("0", 64)
			case "seed-label":
				p.Objects[4].Name = "seeds"
			}
			if _, err := run(p); err == nil {
				t.Fatal("ASSERT_PIN_OR_BUDGET_REFUSAL", kind)
			}
		})
	}
	for _, kind := range []string{"unknown", "duplicate", "case", "version", "missing"} {
		t.Run("strict-"+kind, func(t *testing.T) {
			b := bytes.Clone(rawPins)
			switch kind {
			case "unknown":
				b = append([]byte(`{"extra":0,`), b[1:]...)
			case "duplicate":
				b = append([]byte(`{"schema":"lsp-trace.retained-partial-qualification-input.private.v1",`), b[1:]...)
			case "case":
				b = bytes.Replace(b, []byte(`"schema"`), []byte(`"Schema"`), 1)
			case "version":
				b = bytes.Replace(b, []byte("input.private.v1"), []byte("input.private.v2"), 1)
			case "missing":
				b = bytes.Replace(b, []byte(`"max_read_bytes":134217728,`), nil, 1)
			}
			if _, e := pilot.NewExactRetainedPartialQualificationInput(b, inputs, outputs); e == nil {
				t.Fatal("ASSERT_STRICT_CANONICAL_INPUT", kind)
			}
		})
	}
	// Semantic mutations receive fresh genuine content-retention receipts and a
	// rewritten outcome reference chain, so rejection cannot rely on stale hashes.
	semantic := []struct {
		name     string
		index    int
		from, to string
	}{
		{"config", 0, cfg.CodeHEAD, strings.Repeat("c", 40)},
		{"config-profile-empty", 0, `"profile":"go"`, `"profile":""`},
		{"config-trust-empty", 0, `"trust_domain":"test"`, `"trust_domain":""`},
		{"config-envref-invalid", 0, `"environment_reference":"local"`, `"environment_reference":"/invalid"`},
		{"config-environment-unsorted", 0, `"environment":["PATH=/fixture/no-exec"]`, `"environment":["Z=1","A=2"]`},
		{"config-environment-duplicates", 0, `"environment":["PATH=/fixture/no-exec"]`, `"environment":["A=1","A=2"]`},
		{"config-environment-invalid-name", 0, `"environment":["PATH=/fixture/no-exec"]`, `"environment":["1INVALID=1"]`},
		{"config-args-nul", 0, `"args":null`, `"args":["\u0000"]`},
		{"config-args-over-count", 0, `"args":null`, `"args":[` + strings.TrimSuffix(strings.Repeat(`"x",`, 17), ",") + `]`},
		{"config-server-relative", 0, `"server":"/fixture/server"`, `"server":"relative-server"`},
		{"input-observation", 1, `"head":"`, `"head":"A`},
		{"claim", 2, `"commit_reported":true`, `"commit_reported":false`},
		{"discovery", 3, `"root"`, `"forged-root"`},
		{"seed-path", 4, "cmd/lsp-trace/filter_command.go", "cmd/lsp-trace/main.go"},
		{"views", 6, `"root"`, `"forged-root"`},
		{"private-acceptance", 5, `"accepted":false`, `"accepted":true`},
		{"query-membership", 5, "cmd/lsp-trace/filter_command.go", "cmd/lsp-trace/main.go"},
		{"source-hash", 5, `"sha256":"sha256:`, `"sha256":"sha256:0`},
		{"private-replay", 5, `"replay_admitted":false`, `"replay_admitted":true`},
		{"source-content", 5, `"content":"`, `"content":"A`},
		{"source-nomination", 5, `"role":"NOMINATION"`, `"role":"CONTEXT_ONLY"`},
		{"source-withheld-receipt", 5, `"status":"POLICY_WITHHELD"`, `"status":"READABLE"`},
		{"artifact-label", 5, `"target-000009"`, `"forged-target"`},
		{"withheld-count", 5, `"withheld_count":1`, `"withheld_count":2`},
	}
	for _, tc := range semantic {
		t.Run("semantic-"+tc.name, func(t *testing.T) {
			changed := bytes.Replace(original[tc.index], []byte(tc.from), []byte(tc.to), 1)
			if bytes.Equal(changed, original[tc.index]) {
				t.Fatal("mutation did not change bytes", tc.name)
			}
			if strings.HasPrefix(tc.name, "config-") {
				var invalid pilot.ScopedRehearsalConfigPrivate
				if e := json.Unmarshal(changed, &invalid); e != nil {
					t.Fatal(e)
				}
				if e := pilot.ValidateScopedRehearsalConfigFixturePrivate(invalid); e == nil {
					t.Fatal("mutation not producer-invalid", tc.name)
				}
			}
			p := pins
			var outer pilot.ScopedPartialOutcomeObservationPrivate
			if e := json.Unmarshal(original[7], &outer); e != nil {
				t.Fatal(e)
			}
			rec := records[tc.index]
			rec.ID = pilot.Digest(changed)
			rec.Bytes = len(changed)
			rec.Primary = "NAMESPACE_COMMITTED"
			rec.Backup = "NAMESPACE_COMMITTED"
			rec.PrimaryReceipt, e = outputs[0].PublishPartialInspectionV2Private(context.Background(), rec.ID, changed)
			if e != nil {
				t.Fatal(e)
			}
			rec.BackupReceipt, e = outputs[1].PublishPartialInspectionV2Private(context.Background(), rec.ID, changed)
			if e != nil {
				t.Fatal(e)
			}
			outer.Records[tc.index] = rec
			outcome, e := json.Marshal(outer)
			if e != nil {
				t.Fatal(e)
			}
			oid := pilot.Digest(outcome)
			for _, root := range outputs {
				if _, e := root.PublishPartialInspectionV2Private(context.Background(), oid, outcome); e != nil {
					t.Fatal(e)
				}
			}
			p.Objects[tc.index].ID = rec.ID
			p.Objects[tc.index].Bytes = uint64(rec.Bytes)
			p.Objects[7].ID = oid
			p.Objects[7].Bytes = uint64(len(outcome))
			if _, e := run(p); e == nil {
				t.Fatal("ASSERT_SEMANTIC_CHAIN_REFUSAL", tc.name)
			}
		})
	}
	// Ordinary public provenance cannot admit this private profile even after a
	// successful inspection qualification. This is an independent public gate.
	carrier, e := graphprovenance.ParseSourceWithholdingPrivateV1(original[5])
	if e != nil {
		t.Fatal(e)
	}
	if _, e := graphprovenance.CaptureV5(carrier.Evidence.GraphBytes, cfg.Session.SessionID, cfg.Session.Generation, manageddiagnostic.QueryResult{}, &carrier.Evidence); e == nil {
		t.Fatal("ASSERT_ORDINARY_NATIVE_REFUSAL")
	}
	for _, kind := range []string{"outcome-authority", "outcome-budget", "outcome-reference", "outcome-classification", "outcome-server-command"} {
		t.Run(kind, func(t *testing.T) {
			var outer pilot.ScopedPartialOutcomeObservationPrivate
			if e := json.Unmarshal(original[7], &outer); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "outcome-server-command":
				outer.Runtime.ServerCommand = "/unrelated/server"
			case "outcome-authority":
				outer.Authority = 1
			case "outcome-budget":
				outer.EffectiveBudget.Requests++
			case "outcome-reference":
				outer.Records[0].ID = pins.Objects[1].ID
			case "outcome-classification":
				outer.SourceDispositionCounts["READABLE"]++
			}
			raw, _ := json.Marshal(outer)
			id := pilot.Digest(raw)
			for _, r := range outputs {
				if _, e := r.PublishPartialInspectionV2Private(context.Background(), id, raw); e != nil {
					t.Fatal(e)
				}
			}
			p := pins
			p.Objects[7].ID = id
			p.Objects[7].Bytes = uint64(len(raw))
			if _, e := run(p); e == nil {
				t.Fatal("ASSERT_OUTCOME_SEMANTIC_REFUSAL", kind)
			}
		})
	}
	for _, pair := range [][2]*publication.ContentRetentionRoot{{outputs[1], outputs[0]}, {outputs[0], outputs[0]}, {inputs[0], outputs[1]}} {
		forged, e := pilot.NewExactRetainedPartialQualificationInput(rawPins, inputs, pair)
		if e == nil {
			bad, e := pilot.QualifyRetainedPartialNativePrivate(context.Background(), forged)
			if e == nil || len(bad.ReportBytes()) != 0 {
				t.Fatal("ASSERT_MIXED_ROOTS_REFUSED")
			}
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if bad, e := pilot.QualifyRetainedPartialNativePrivate(cancelled, in); e == nil || len(bad.ReportBytes()) != 0 {
		t.Fatal("ASSERT_CANCELLATION_NO_QUALIFICATION")
	}
	// A sealed snapshot owns bytes, not readers: even closed borrowed roots cannot
	// cause derivation to re-read source, recover a claim, or reacquire anything.
	for _, root := range inputs {
		if err := root.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, root := range outputs {
		if err := root.Close(); err != nil {
			t.Fatal(err)
		}
	}
	closedDerived, err := pilot.DeriveRetainedPartialNativePrivate(context.Background(), q, pins, pilot.RetainedPartialDerivationBudgetPrivate{MaxInputBytes: 8 << 20, MaxOutputBytes: 8 << 20, MaxWorkBytes: 2 << 30})
	if err != nil || len(closedDerived.NativeGraphBytes()) == 0 {
		t.Fatal("ASSERT_DERIVATION_HAS_NO_BORROWED_READER_IO", err)
	}
	after, prepAfter := f.Traffic()
	if before != after || prepares != prepAfter {
		t.Fatal("ASSERT_OFFLINE_ZERO_TRAFFIC")
	}
	t.Log("ASSERT_PAIRED_PRIVATE_INSPECTION: PASS; ASSERT_NATIVE_CEILING: PASS; ASSERT_OFFLINE_ZERO_TRAFFIC: PASS")
}
