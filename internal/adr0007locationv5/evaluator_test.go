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
