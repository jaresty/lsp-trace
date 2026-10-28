//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/publication"
)

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
	h, m, cfg, f := pilot.NewScopedPartialExact25V4RehearsalFixturePrivate(t)
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
	after, prepAfter := f.Traffic()
	if before != after || prepares != prepAfter {
		t.Fatal("ASSERT_OFFLINE_ZERO_TRAFFIC")
	}
	t.Log("ASSERT_PAIRED_PRIVATE_INSPECTION: PASS; ASSERT_NATIVE_CEILING: PASS; ASSERT_OFFLINE_ZERO_TRAFFIC: PASS")
}
