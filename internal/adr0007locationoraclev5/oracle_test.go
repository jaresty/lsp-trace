package adr0007locationoraclev5

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func corpusRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "docs", "pilot", "adr0007", "experiment", "location-intersection-prospective-v5")
}
func evalCase(t *testing.T, id string, limits Limits) Result {
	t.Helper()
	d := filepath.Join(corpusRoot(t), "inputs", id)
	raw, err := os.ReadFile(filepath.Join(d, "REQUEST.raw.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c Condition
	b, err := os.ReadFile(filepath.Join(d, "CONDITION.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	binding, err := os.ReadFile(filepath.Join(d, "BINDING.json"))
	present := err == nil
	if !present {
		binding = nil
	}
	return Evaluate(raw, binding, present, c, limits)
}
func TestCorpusNormalAndDeterministic(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(corpusRoot(t), "inputs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 26 {
		t.Fatalf("ASSERT corpus-count FAIL got=%d", len(entries))
	}
	for _, e := range entries {
		id := e.Name()
		t.Run(id, func(t *testing.T) {
			a := evalCase(t, id, PublishedLimits())
			b := evalCase(t, id, PublishedLimits())
			if !reflect.DeepEqual(a, b) || string(Canonical(a)) != string(Canonical(b)) {
				t.Fatalf("ASSERT deterministic FAIL case=%s", id)
			}
			if a.Schema != ResultSchema {
				t.Fatalf("ASSERT result-schema FAIL case=%s", id)
			}
			if a.Outcome == "COMPLETE" {
				sum := a.Counters.Eligible + a.Counters.Ineligible + a.Counters.UnavailableLocation + a.Counters.InvalidLocation + a.Counters.DuplicateMember + a.Counters.FilteredByPolicy
				if sum != a.Counters.Input {
					t.Fatalf("ASSERT six-way FAIL case=%s", id)
				}
			} else if len(a.Members) != 0 || len(a.Ranked) != 0 || a.Counters != (Counters{}) {
				t.Fatalf("ASSERT total-failure FAIL case=%s", id)
			}
		})
	}
}
func TestCancellationPrecedesDeadline(t *testing.T) {
	r := evalCase(t, "24-cancel-deadline", PublishedLimits())
	if r.Outcome != "CANCELLED" || r.Detail != "CANCEL_SIGNAL" {
		t.Fatalf("ASSERT cancel-precedence FAIL got=%s/%s", r.Outcome, r.Detail)
	}
}
func TestHalfOpenAdjacency(t *testing.T) {
	r := evalCase(t, "04-adjacent-half-open", PublishedLimits())
	want := "INELIGIBLE"
	if os.Getenv("ADR0007_ORACLE_MUTATION") == "half-open-inclusive" {
		want = "ELIGIBLE"
	}
	if r.Outcome != "COMPLETE" || len(r.Members) != 1 || r.Members[0].Outcome != want {
		t.Fatalf("ASSERT half-open FAIL want=%s got=%s", want, Canonical(r))
	}
}
func TestCorrectedCaseSemantics(t *testing.T) {
	t.Run("case05-repeated-union-reached", func(t *testing.T) {
		r := evalCase(t, "05-union-repeat", PublishedLimits())
		if r.Outcome != "COMPLETE" || len(r.Members) != 1 || r.Members[0].Outcome != "ELIGIBLE" || len(r.Members[0].Witnesses) != 2 || r.Counters.Witnesses != 2 || r.Counters.Eligible != 1 {
			t.Fatalf("ASSERT case05 repeated-union-reached FAIL got=%s", Canonical(r))
		}
	})
	t.Run("case16-eligible", func(t *testing.T) {
		r := evalCase(t, "16-member-eligible", PublishedLimits())
		if r.Outcome != "COMPLETE" || len(r.Members) != 1 || r.Members[0].Outcome != "ELIGIBLE" || len(r.Members[0].Witnesses) != 1 || r.Counters.Eligible != 1 {
			t.Fatalf("ASSERT case16 eligible FAIL got=%s", Canonical(r))
		}
	})
	t.Run("case17-ineligible", func(t *testing.T) {
		r := evalCase(t, "17-member-ineligible", PublishedLimits())
		if r.Outcome != "COMPLETE" || len(r.Members) != 1 || r.Members[0].Outcome != "INELIGIBLE" || len(r.Members[0].Witnesses) != 0 || r.Counters.Ineligible != 1 {
			t.Fatalf("ASSERT case17 ineligible FAIL got=%s", Canonical(r))
		}
	})
}
func TestStrictRequestParserAdversarialRejects(t *testing.T) {
	d := filepath.Join(corpusRoot(t), "inputs", "01-exact-intersects")
	raw, _ := os.ReadFile(filepath.Join(d, "REQUEST.raw.json"))
	binding, _ := os.ReadFile(filepath.Join(d, "BINDING.json"))
	for _, tc := range []struct{ name, body, wantID, detail string }{
		{"duplicate-id-recovers-first-id", strings.Replace(string(raw), `"id":"c01"`, `"id":"c01","id":"evil"`, 1), "c01", "JSON_SYNTAX"},
		{"trailing-token-recovers-id", strings.TrimSpace(string(raw)) + ` {}`, "c01", "JSON_SYNTAX"},
		{"missing-member-priority", strings.Replace(string(raw), `,"members":[`, `,"x_unknown":true,"members":[`, 1), "c01", "UNKNOWN_FIELD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Evaluate([]byte(tc.body), binding, true, Condition{}, PublishedLimits())
			if r.RequestID != tc.wantID || r.Outcome != "INVALID_REQUEST" || r.Detail != tc.detail {
				t.Fatalf("ASSERT strict-request-parser FAIL got id=%q %s/%s", r.RequestID, r.Outcome, r.Detail)
			}
		})
	}
}

func TestEnvelopeProjectionAdversarialRejects(t *testing.T) {
	d := filepath.Join(corpusRoot(t), "inputs", "01-exact-intersects")
	raw, _ := os.ReadFile(filepath.Join(d, "REQUEST.raw.json"))
	binding, _ := os.ReadFile(filepath.Join(d, "BINDING.json"))
	for _, tc := range []struct{ name, body string }{
		{"complete-with-input", strings.Replace(string(binding), `}}`, `},"input":[]}`, 1)},
		{"duplicate-envelope-key", strings.Replace(string(binding), `"outcome":"COMPLETE"`, `"outcome":"COMPLETE","outcome":"COMPLETE"`, 1)},
		{"tuple-missing-bytes", strings.Replace(string(binding), `,"bytes":"YfCfmIBiDQp4eQo="`, ``, 1)},
		{"tuple-wrong-path-type", strings.Replace(string(binding), `"path":"src/a"`, `"path":7`, 1)},
		{"typed-with-binding", strings.Replace(string(binding), `"outcome":"COMPLETE"`, `"outcome":"INVALID_SOURCE","detail":"FILE_DIGEST"`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Evaluate(raw, []byte(tc.body), true, Condition{}, PublishedLimits())
			if r.Outcome != "SOURCE_ADMISSION_MISMATCH" || r.Detail != "BINDING_SCHEMA" && r.Detail != "BINDING_INVALID_SOURCE" {
				t.Fatalf("ASSERT envelope-projection FAIL got=%s/%s", r.Outcome, r.Detail)
			}
		})
	}
	t.Run("invalid-tuple-before-max-sources-zero", func(t *testing.T) {
		l := PublishedLimits()
		l.MaxSources = 0
		body := strings.Replace(string(binding), `"path":"src/a"`, `"path":""`, 1)
		r := Evaluate(raw, []byte(body), true, Condition{}, l)
		if r.Outcome != "SOURCE_ADMISSION_MISMATCH" || r.Detail != "BINDING_INVALID_SOURCE" {
			t.Fatalf("ASSERT invalid-tuple-before-max-sources-zero FAIL got=%s/%s", r.Outcome, r.Detail)
		}
	})
}

func TestEnvelopeNonCompleteExactTopLevelFieldSets(t *testing.T) {
	d := filepath.Join(corpusRoot(t), "inputs", "01-exact-intersects")
	raw, _ := os.ReadFile(filepath.Join(d, "REQUEST.raw.json"))
	const source = `{"path":"src/a","revision":"rev-v5","fileDigest":"sha256:a0bf9128df23e89ede49c09a4d61193dbc5e32b6791e7ab336381d617e01cb34","objectDigest":"sha256:a0bf9128df23e89ede49c09a4d61193dbc5e32b6791e7ab336381d617e01cb34","bytes":"YfCfmIBiDQp4eQo="}`
	for _, tc := range []struct {
		name, valid, wantOutcome, wantDetail, missing, forbidden string
	}{
		{"invalid-request", `{"schema":"` + EnvelopeSchema + `","outcome":"INVALID_REQUEST","detail":"REQUEST_FIELD"}`, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_REQUEST", `"detail":"REQUEST_FIELD"`, `,"input":[]`},
		{"invalid-source", `{"schema":"` + EnvelopeSchema + `","outcome":"INVALID_SOURCE","detail":"FILE_DIGEST","input":[` + source + `]}`, "SOURCE_ADMISSION_MISMATCH", "BINDING_INVALID_SOURCE", `,"input":[` + source + `]`, `,"duplicatePath":"src/a"`},
		{"duplicate-source", `{"schema":"` + EnvelopeSchema + `","outcome":"DUPLICATE_SOURCE","detail":"DUPLICATE_PATH","duplicatePath":"src/a","input":[` + source + `]}`, "SOURCE_ADMISSION_MISMATCH", "BINDING_DUPLICATE_SOURCE", `,"duplicatePath":"src/a"`, `,"binding":{}`},
		{"resource-limit", `{"schema":"` + EnvelopeSchema + `","outcome":"RESOURCE_LIMIT","detail":"SOURCES","input":[` + source + `]}`, "RESOURCE_LIMIT", "SOURCES", `,"input":[` + source + `]`, `,"duplicatePath":"src/a"`},
	} {
		t.Run(tc.name+"-valid-field-set", func(t *testing.T) {
			r := Evaluate(raw, []byte(tc.valid), true, Condition{}, PublishedLimits())
			if r.Outcome != tc.wantOutcome || r.Detail != tc.wantDetail {
				t.Fatalf("ASSERT envelope-%s-valid-field-set FAIL got=%s/%s", tc.name, r.Outcome, r.Detail)
			}
		})
		t.Run(tc.name+"-missing-required", func(t *testing.T) {
			body := strings.Replace(tc.valid, tc.missing, ``, 1)
			r := Evaluate(raw, []byte(body), true, Condition{}, PublishedLimits())
			if r.Outcome != "SOURCE_ADMISSION_MISMATCH" || r.Detail != "BINDING_SCHEMA" {
				t.Fatalf("ASSERT envelope-%s-missing-required FAIL got=%s/%s body=%s", tc.name, r.Outcome, r.Detail, body)
			}
		})
		t.Run(tc.name+"-forbidden-known-key", func(t *testing.T) {
			body := strings.TrimSuffix(tc.valid, "}") + tc.forbidden + `}`
			r := Evaluate(raw, []byte(body), true, Condition{}, PublishedLimits())
			if r.Outcome != "SOURCE_ADMISSION_MISMATCH" || r.Detail != "BINDING_SCHEMA" {
				t.Fatalf("ASSERT envelope-%s-forbidden-known-key FAIL got=%s/%s body=%s", tc.name, r.Outcome, r.Detail, body)
			}
		})
		t.Run(tc.name+"-additional-key", func(t *testing.T) {
			body := strings.TrimSuffix(tc.valid, "}") + `,"extra":true}`
			r := Evaluate(raw, []byte(body), true, Condition{}, PublishedLimits())
			if r.Outcome != "SOURCE_ADMISSION_MISMATCH" || r.Detail != "BINDING_SCHEMA" {
				t.Fatalf("ASSERT envelope-%s-additional-key FAIL got=%s/%s body=%s", tc.name, r.Outcome, r.Detail, body)
			}
		})
	}
}

type workEvent struct {
	Stage, Event                 string
	Before, Charge, After, Limit uint64
}

func captureWorkEvents(fn func() Result) (Result, []workEvent) {
	var events []workEvent
	old := observeWorkStage
	observeWorkStage = func(stage, event string, before, charge, after, limit uint64) {
		if stage == "P" || stage == "R" || stage == "M" || stage == "S" || stage == "Q" || stage == "X" || stage == "C" || stage == "B" {
			events = append(events, workEvent{stage, event, before, charge, after, limit})
		}
	}
	defer func() { observeWorkStage = old }()
	return fn(), events
}

func firstAfter(events []workEvent, stage string) (workEvent, bool) {
	for _, e := range events {
		if e.Stage == stage && e.Event == "after" && e.Charge > 0 {
			return e, true
		}
	}
	return workEvent{}, false
}

func TestCheckpointStageIsolatedProof(t *testing.T) {
	for _, tc := range []struct{ stage, caseID string }{
		{"M", "01-exact-intersects"}, {"P", "01-exact-intersects"}, {"S", "01-exact-intersects"}, {"R", "01-exact-intersects"},
		{"Q", "01-exact-intersects"}, {"X", "01-exact-intersects"}, {"C", "05-union-repeat"}, {"B", "01-exact-intersects"},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			base, baseEvents := captureWorkEvents(func() Result { return evalCase(t, tc.caseID, PublishedLimits()) })
			if base.Outcome != "COMPLETE" {
				t.Fatalf("ASSERT stage-base-%s FAIL got=%s/%s", tc.stage, base.Outcome, base.Detail)
			}
			target, ok := firstAfter(baseEvents, tc.stage)
			if !ok {
				t.Fatalf("ASSERT stage-after-%s-observed FAIL", tc.stage)
			}

			l := PublishedLimits()
			l.MaxWork = target.After - 1
			minus, minusEvents := captureWorkEvents(func() Result { return evalCase(t, tc.caseID, l) })
			if minus.Outcome != "RESOURCE_LIMIT" || minus.Detail != "WORK" {
				t.Fatalf("ASSERT stage-%s-after-minus-one FAIL got=%s/%s", tc.stage, minus.Outcome, minus.Detail)
			}
			var failed *workEvent
			for i := range minusEvents {
				if minusEvents[i].Event == "fail" {
					failed = &minusEvents[i]
					break
				}
			}
			if failed == nil || failed.Stage != tc.stage || failed.Before != target.Before || failed.Charge != target.Charge || failed.Limit != target.After-1 {
				t.Fatalf("ASSERT stage-%s-exact-charge FAIL target=%+v failed=%+v", tc.stage, target, failed)
			}

			l.MaxWork = target.After
			cross, crossEvents := captureWorkEvents(func() Result { return evalCase(t, tc.caseID, l) })
			idx := -1
			for i, e := range crossEvents {
				if e.Stage == tc.stage && e.Event == "after" && e.Before == target.Before && e.Charge == target.Charge && e.After == target.After {
					idx = i
					break
				}
			}
			if idx < 0 {
				t.Fatalf("ASSERT stage-%s-crosses-charge FAIL got=%s/%s events=%+v", tc.stage, cross.Outcome, cross.Detail, crossEvents)
			}
			if cross.Outcome != "COMPLETE" && idx == len(crossEvents)-1 {
				t.Fatalf("ASSERT stage-%s-next-checkpoint FAIL got=%s/%s", tc.stage, cross.Outcome, cross.Detail)
			}
		})
	}
}

func TestWorkStageCoefficientSensitivity(t *testing.T) {
	_, events := captureWorkEvents(func() Result { return evalCase(t, "05-union-repeat", PublishedLimits()) })
	coeff := map[string]uint64{"P": 7, "R": 11, "M": 13, "S": 1, "Q": 19, "X": 23, "C": 29, "B": 31}
	seen := map[string]bool{}
	for _, e := range events {
		if e.Event != "before" || e.Charge == 0 {
			continue
		}
		if e.Charge%coeff[e.Stage] != 0 {
			t.Fatalf("ASSERT coefficient-%s-placement FAIL event=%+v", e.Stage, e)
		}
		seen[e.Stage] = true
	}
	for stage := range coeff {
		if !seen[stage] {
			t.Fatalf("ASSERT coefficient-%s-observed FAIL", stage)
		}
	}
}

func TestMutationWitnesses(t *testing.T) {
	t.Run("policy-digest", func(t *testing.T) {
		d := filepath.Join(corpusRoot(t), "inputs", "01-exact-intersects")
		raw, _ := os.ReadFile(filepath.Join(d, "REQUEST.raw.json"))
		raw = []byte(strings.Replace(string(raw), PolicyDigest, "sha256:"+strings.Repeat("0", 64), 1))
		binding, _ := os.ReadFile(filepath.Join(d, "BINDING.json"))
		r := Evaluate(raw, binding, true, Condition{}, PublishedLimits())
		if r.Outcome != "POLICY_MISMATCH" || r.Detail != "POLICY_DIGEST" {
			t.Fatalf("ASSERT policy-digest FAIL got=%s/%s", r.Outcome, r.Detail)
		}
	})
	t.Run("work-boundary", func(t *testing.T) {
		base := evalCase(t, "01-exact-intersects", PublishedLimits())
		if base.Outcome != "COMPLETE" {
			t.Fatalf("ASSERT work-base FAIL %s/%s", base.Outcome, base.Detail)
		}
		l := PublishedLimits()
		l.MaxWork = base.Counters.Work - 1
		r := evalCase(t, "01-exact-intersects", l)
		if r.Outcome != "RESOURCE_LIMIT" || r.Detail != "WORK" {
			t.Fatalf("ASSERT work-minus-one FAIL got=%s/%s", r.Outcome, r.Detail)
		}
	})
	t.Run("output-boundary", func(t *testing.T) {
		base := evalCase(t, "01-exact-intersects", PublishedLimits())
		l := PublishedLimits()
		l.MaxOutputBytes = base.Counters.OutputBytes - 1
		r := evalCase(t, "01-exact-intersects", l)
		if r.Outcome != "RESOURCE_LIMIT" || r.Detail != "OUTPUT_BYTES" {
			t.Fatalf("ASSERT output-minus-one FAIL got=%s/%s", r.Outcome, r.Detail)
		}
	})
}
