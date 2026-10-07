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
