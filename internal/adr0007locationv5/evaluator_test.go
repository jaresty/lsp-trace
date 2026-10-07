package adr0007locationv5

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

type cond struct {
	Cancel   bool `json:"cancel"`
	Deadline bool `json:"deadlineExpired"`
}

func corpusRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "docs", "pilot", "adr0007", "experiment", "location-intersection-prospective-v5", "inputs")
}
func loadCase(t *testing.T, name string) ([]byte, []byte, cond) {
	t.Helper()
	p := filepath.Join(corpusRoot(), name)
	r, e := os.ReadFile(filepath.Join(p, "REQUEST.raw.json"))
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(p, "BINDING.json"))
	if os.IsNotExist(e) {
		b = nil
	} else if e != nil {
		t.Fatal(e)
	}
	var c cond
	z, e := os.ReadFile(filepath.Join(p, "CONDITION.json"))
	if e != nil {
		t.Fatal(e)
	}
	if json.Unmarshal(z, &c) != nil {
		t.Fatal("condition")
	}
	return r, b, c
}
func TestApproved26DeterministicAndImmutable(t *testing.T) {
	dirs, e := os.ReadDir(corpusRoot())
	if e != nil {
		t.Fatal(e)
	}
	var names []string
	for _, d := range dirs {
		if d.IsDir() {
			names = append(names, d.Name())
		}
	}
	sort.Strings(names)
	if len(names) != 26 {
		t.Fatalf("ASSERT corpus-count FAIL got=%d", len(names))
	}
	for _, n := range names {
		t.Run(n, func(t *testing.T) {
			r, b, c := loadCase(t, n)
			rh, bh := SHA256(r), SHA256(b)
			a, e := Evaluate(r, b, StaticControl{Cancel: c.Cancel, Deadline: c.Deadline}, PublishedLimits())
			if e != nil {
				t.Fatal(e)
			}
			x, _ := Canonical(a)
			d, e := Evaluate(append([]byte(nil), r...), append([]byte(nil), b...), StaticControl{Cancel: c.Cancel, Deadline: c.Deadline}, PublishedLimits())
			if e != nil {
				t.Fatal(e)
			}
			y, _ := Canonical(d)
			if !bytes.Equal(x, y) {
				t.Fatal("ASSERT deterministic-output FAIL")
			}
			if x[len(x)-1] != '\n' || a.Schema != ResultSchema {
				t.Fatal("ASSERT canonical-result FAIL")
			}
			if SHA256(r) != rh || SHA256(b) != bh {
				t.Fatal("ASSERT input-immutable FAIL")
			}
		})
	}
}
func TestCanonicalMeasurements(t *testing.T) {
	tests := []struct {
		name      string
		wantBytes uint64
		wantWork  uint64
	}{
		{"01-exact-intersects", 914, 30864},
		{"04-adjacent-half-open", 420, 15808},
		{"18-member-unavailable", 430, 15803},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, b, c := loadCase(t, tt.name)
			x, e := Evaluate(r, b, StaticControl{Cancel: c.Cancel, Deadline: c.Deadline}, PublishedLimits())
			if e != nil {
				t.Fatal(e)
			}
			if x.Counters.OutputBytes != tt.wantBytes || x.Counters.Work != tt.wantWork {
				t.Fatalf("ASSERT canonical-measurements FAIL got B=%d/W=%d want B=%d/W=%d", x.Counters.OutputBytes, x.Counters.Work, tt.wantBytes, tt.wantWork)
			}
			measurement := x
			measurement.Counters.Work = 0
			measurement.Counters.OutputBytes = 0
			image, _ := Canonical(measurement)
			if uint64(len(image)) != x.Counters.OutputBytes {
				t.Fatalf("ASSERT canonical-measurement-image FAIL len=%d counter=%d", len(image), x.Counters.OutputBytes)
			}
		})
	}
}

func TestDirectEvaluatorCase01ExactCanonicalBoundaries(t *testing.T) {
	r, b, c := loadCase(t, "01-exact-intersects")
	base := PublishedLimits()
	tests := []struct {
		name   string
		limits Limits
		out    string
		detail string
		work   uint64
		bytes  uint64
	}{
		{"case01-W-30864-exact", base, "COMPLETE", "NONE", 30864, 914},
		{"case01-W-30863-minus-one", func() Limits { l := base; l.MaxWork = 30863; return l }(), "RESOURCE_LIMIT", "WORK", 0, 0},
		{"case01-B-914-exact", func() Limits { l := base; l.MaxOutputBytes = 914; return l }(), "COMPLETE", "NONE", 30864, 914},
		{"case01-B-913-minus-one", func() Limits { l := base; l.MaxOutputBytes = 913; return l }(), "RESOURCE_LIMIT", "OUTPUT_BYTES", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, err := Evaluate(r, b, StaticControl{Cancel: c.Cancel, Deadline: c.Deadline}, tt.limits)
			if err != nil {
				t.Fatal(err)
			}
			if x.Outcome != tt.out || x.Detail != tt.detail {
				t.Fatalf("ASSERT case01-boundary-outcome FAIL got=%s/%s want=%s/%s", x.Outcome, x.Detail, tt.out, tt.detail)
			}
			if tt.out == "COMPLETE" && (x.Counters.Work != tt.work || x.Counters.OutputBytes != tt.bytes) {
				t.Fatalf("ASSERT case01-boundary-counters FAIL got W=%d/B=%d want W=%d/B=%d", x.Counters.Work, x.Counters.OutputBytes, tt.work, tt.bytes)
			}
			canonical, err := Canonical(x)
			if err != nil {
				t.Fatal(err)
			}
			if canonical[len(canonical)-1] != '\n' {
				t.Fatal("ASSERT case01-boundary-canonical-newline FAIL")
			}
		})
	}
}

func TestDirectEvaluatorAdversarialJSONFraming(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{"duplicate-top-level-key", []byte(`{"schema":"lsp-trace.adr0007.location-intersection.request.private.v5","id":"a","id":"b"}`), "DUPLICATE_FIELD"},
		{"trailing-json-object", []byte(`{"schema":"lsp-trace.adr0007.location-intersection.request.private.v5","id":"a"}{}`), "TRAILING_DATA"},
		{"trailing-json-scalar", []byte(`{"schema":"lsp-trace.adr0007.location-intersection.request.private.v5","id":"a"}0`), "TRAILING_DATA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, err := Evaluate(tt.raw, nil, StaticControl{}, PublishedLimits())
			if err != nil {
				t.Fatal(err)
			}
			if x.Outcome != "INVALID_REQUEST" || x.Detail != tt.want {
				t.Fatalf("ASSERT adversarial-json-framing FAIL got=%s/%s want INVALID_REQUEST/%s", x.Outcome, x.Detail, tt.want)
			}
		})
	}
}

func TestDirectEvaluatorAdversarialEnvelopeShape(t *testing.T) {
	r, b, _ := loadCase(t, "01-exact-intersects")
	var env map[string]any
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		mut  func(map[string]any)
		want string
	}{
		{"envelope-forbidden-field", func(m map[string]any) { m["extra"] = 0 }, "BINDING_SCHEMA"},
		{"envelope-missing-binding", func(m map[string]any) { delete(m, "binding") }, "BINDING_SCHEMA"},
		{"binding-forbidden-field", func(m map[string]any) { m["binding"].(map[string]any)["extra"] = 0 }, "BINDING_SCHEMA"},
		{"binding-missing-sources", func(m map[string]any) { delete(m["binding"].(map[string]any), "sources") }, "BINDING_SCHEMA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m map[string]any
			bb, _ := json.Marshal(env)
			if err := json.Unmarshal(bb, &m); err != nil {
				t.Fatal(err)
			}
			tt.mut(m)
			mut, _ := json.Marshal(m)
			x, err := Evaluate(r, mut, StaticControl{}, PublishedLimits())
			if err != nil {
				t.Fatal(err)
			}
			if x.Outcome != "SOURCE_ADMISSION_MISMATCH" || x.Detail != tt.want {
				t.Fatalf("ASSERT adversarial-envelope-shape FAIL got=%s/%s want SOURCE_ADMISSION_MISMATCH/%s", x.Outcome, x.Detail, tt.want)
			}
		})
	}
}

func TestDirectEvaluatorInvalidTuplePrecedesSourceCount(t *testing.T) {
	r, b, _ := loadCase(t, "01-exact-intersects")
	var env map[string]any
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	sources := env["binding"].(map[string]any)["sources"].([]any)
	sources[0].(map[string]any)["bytes"] = "not-base64"
	mut, _ := json.Marshal(env)
	limits := PublishedLimits()
	limits.MaxSources = 0
	x, err := Evaluate(r, mut, StaticControl{}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if x.Outcome != "SOURCE_ADMISSION_MISMATCH" || x.Detail != "BINDING_INVALID_SOURCE" {
		t.Fatalf("ASSERT invalid-tuple-before-source-count FAIL got=%s/%s", x.Outcome, x.Detail)
	}
}

func TestDirectEvaluatorLowWorkIntermediateCheckpoints(t *testing.T) {
	r, b, _ := loadCase(t, "01-exact-intersects")
	base := PublishedLimits()
	tests := []struct {
		name string
		max  uint64
	}{
		{"P-request-size-accounting", 0},
		{"R-request-parse-accounting", 2116},
		{"M-member-validation-accounting", 2129},
		{"S-source-tuple-accounting", 2136},
		{"Q-source-bytes-accounting", 2147},
		{"X-intersection-accounting", 2173},
		{"C-canonical-output-accounting", 29949},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := base
			limits.MaxWork = tt.max
			x, err := Evaluate(r, b, StaticControl{}, limits)
			if err != nil {
				t.Fatal(err)
			}
			if x.Outcome != "RESOURCE_LIMIT" || x.Detail != "WORK" {
				t.Fatalf("ASSERT low-work-%s FAIL got=%s/%s", tt.name, x.Outcome, x.Detail)
			}
		})
	}
}

type checkpointControl struct {
	at       int
	deadline bool
	polls    int
}

func (c *checkpointControl) Cancelled() bool {
	c.polls++
	return !c.deadline && c.polls == c.at
}
func (c *checkpointControl) DeadlineExceeded() bool {
	return c.deadline && c.polls == c.at
}

func TestDirectEvaluatorCancellationAndDeadlineCheckpoints(t *testing.T) {
	r, b, _ := loadCase(t, "01-exact-intersects")
	tests := []struct {
		name     string
		at       int
		deadline bool
		out      string
		detail   string
	}{
		{"cancel-initial", 1, false, "CANCELLED", "CANCEL_SIGNAL"},
		{"cancel-after-request", 2, false, "CANCELLED", "CANCEL_SIGNAL"},
		{"cancel-before-envelope", 4, false, "CANCELLED", "CANCEL_SIGNAL"},
		{"cancel-during-intersections", 7, false, "CANCELLED", "CANCEL_SIGNAL"},
		{"deadline-initial", 1, true, "TIMEOUT", "DEADLINE"},
		{"deadline-after-request", 2, true, "TIMEOUT", "DEADLINE"},
		{"deadline-before-envelope", 4, true, "TIMEOUT", "DEADLINE"},
		{"deadline-during-intersections", 7, true, "TIMEOUT", "DEADLINE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			control := &checkpointControl{at: tt.at, deadline: tt.deadline}
			x, err := Evaluate(r, b, control, PublishedLimits())
			if err != nil {
				t.Fatal(err)
			}
			if x.Outcome != tt.out || x.Detail != tt.detail {
				t.Fatalf("ASSERT control-checkpoint FAIL got=%s/%s want=%s/%s polls=%d", x.Outcome, x.Detail, tt.out, tt.detail, control.polls)
			}
		})
	}
}

func TestCancellationPrecedence(t *testing.T) {
	r, b, _ := loadCase(t, "01-exact-intersects")
	x, _ := Evaluate(r, b, StaticControl{Cancel: true, Deadline: true}, PublishedLimits())
	if x.Outcome != "CANCELLED" || x.Detail != "CANCEL_SIGNAL" {
		t.Fatalf("ASSERT cancellation-precedence FAIL %+v", x)
	}
	t.Log("ASSERT cancellation-precedence PASS")
}
func TestStrictUnknownAndDuplicate(t *testing.T) {
	r, b, _ := loadCase(t, "07-schema-unknown-field")
	x, _ := Evaluate(r, b, StaticControl{}, PublishedLimits())
	if x.Detail != "UNKNOWN_FIELD" {
		t.Fatalf("ASSERT unknown-field FAIL got=%s", x.Detail)
	}
	bad := []byte(`{"schema":"lsp-trace.adr0007.location-intersection.request.private.v5","id":"a","id":"b"}`)
	x, _ = Evaluate(bad, nil, StaticControl{}, PublishedLimits())
	if x.Detail != "DUPLICATE_FIELD" {
		t.Fatalf("ASSERT duplicate-field FAIL got=%s", x.Detail)
	}
	t.Log("ASSERT strict-diagnostics PASS")
}
func TestTypedDuplicateBinding(t *testing.T) {
	r, b, _ := loadCase(t, "13-typed-duplicate-source")
	x, _ := Evaluate(r, b, StaticControl{}, PublishedLimits())
	if x.Detail != "BINDING_DUPLICATE_SOURCE" {
		t.Fatalf("ASSERT typed-duplicate FAIL got=%s/%s", x.Outcome, x.Detail)
	}
	t.Log("ASSERT typed-duplicate PASS")
}
func TestCanonicalEmptyArrays(t *testing.T) {
	r, b, _ := loadCase(t, "04-adjacent-half-open")
	x, _ := Evaluate(r, b, StaticControl{}, PublishedLimits())
	out, _ := Canonical(x)
	if bytes.Contains(out, []byte(`"ranked":null`)) || bytes.Contains(out, []byte(`"witnesses":null`)) {
		t.Fatalf("ASSERT canonical-empty-arrays FAIL %s", out)
	}
	t.Log("ASSERT canonical-empty-arrays PASS")
}

func TestDefensiveResultCopy(t *testing.T) {
	r, b, _ := loadCase(t, "01-exact-intersects")
	x, _ := Evaluate(r, b, StaticControl{}, PublishedLimits())
	y := cloneResult(x)
	y.Members[0].Witnesses[0].Path = "mutated"
	if x.Members[0].Witnesses[0].Path == "mutated" {
		t.Fatal("ASSERT defensive-copy FAIL")
	}
	t.Log("ASSERT defensive-copy PASS")
}
func TestMutationMatrix(t *testing.T) {
	tests := []struct {
		name string
		mut  func([]byte) []byte
		want string
	}{{"unknown-field", func(b []byte) []byte { return bytes.Replace(b, []byte(`"topK":10`), []byte(`"extra":0,"topK":10`), 1) }, "UNKNOWN_FIELD"}, {"policy-digest", func(b []byte) []byte {
		return bytes.Replace(b, []byte(PolicyDigest), []byte("sha256:0000000000000000000000000000000000000000000000000000000000000000"), 1)
	}, "POLICY_DIGEST"}, {"mid-surrogate", func(b []byte) []byte {
		var q map[string]any
		if json.Unmarshal(b, &q) != nil {
			panic("fixture")
		}
		members := q["members"].([]any)
		ranges := members[0].(map[string]any)["ranges"].([]any)
		ranges[0].(map[string]any)["end"].(map[string]any)["character"] = float64(2)
		out, _ := json.Marshal(q)
		return append(out, '\n')
	}, "INVALID_LOCATION"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, b, _ := loadCase(t, "01-exact-intersects")
			x, _ := Evaluate(tt.mut(r), b, StaticControl{}, PublishedLimits())
			got := x.Detail
			if tt.want == "INVALID_LOCATION" && len(x.Members) > 0 {
				got = x.Members[0].Outcome
			}
			if got != tt.want {
				t.Fatalf("ASSERT mutation-%s FAIL got=%s", tt.name, got)
			}
			t.Logf("ASSERT mutation-%s PASS", tt.name)
		})
	}
}
