package main

import (
	"strings"
	"testing"
)

func TestStrictConditionRejectsTextualAndBadJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"copied-result-prohibition-condition-string", []byte(`"CANCEL"`)},
		{"unknown-condition-field", []byte(`{"cancel":false,"oracle":"forbidden"}`)},
		{"duplicate-condition-field", []byte(`{"cancel":false,"cancel":true}`)},
		{"trailing-condition-json", []byte(`{"cancel":false}{}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c conditionFile
			if err := strictJSON("CONDITION.json", tc.raw, &c); err == nil {
				t.Fatalf("expected strict parse failure")
			}
		})
	}
}

func TestStrictConditionParsesStructurally(t *testing.T) {
	var c conditionFile
	if err := strictJSON("CONDITION.json", []byte(`{"cancel":true,"deadlineExpired":false}`), &c); err != nil {
		t.Fatal(err)
	}
	if !c.Cancel || c.DeadlineExpired {
		t.Fatalf("bad structural parse: %+v", c)
	}
}

func TestSourceDigestsIncludeCommandAndEvaluator(t *testing.T) {
	d := sourceDigest()
	var hasCommand, hasEvaluator bool
	for p := range d {
		if strings.HasSuffix(p, "cmd/locationv5execute/main.go") {
			hasCommand = true
		}
		if strings.HasSuffix(p, "internal/adr0007locationv5/evaluator.go") {
			hasEvaluator = true
		}
	}
	if !hasCommand || !hasEvaluator {
		t.Fatalf("missing source digests command=%t evaluator=%t in %#v", hasCommand, hasEvaluator, d)
	}
}
