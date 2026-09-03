package mcpcontract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"lsp-trace/internal/session"
)

const lifecycleVocabularyParityPath = "testdata/lifecycle-vocabulary-parity.v1.json"

type lifecycleVocabularyParity struct {
	Version      string   `json:"version"`
	States       []string `json:"states"`
	Events       []string `json:"events"`
	Guards       []string `json:"guards"`
	Availability []string `json:"availability"`
	Failures     []string `json:"failures"`
	Restart      []string `json:"restart"`
}

func TestLifecycleVocabularyParity(t *testing.T) {
	raw, err := os.ReadFile(lifecycleVocabularyParityPath)
	if err != nil {
		t.Fatalf("ASSERT_LIFECYCLE_VOCABULARY_VECTOR_PRESENT: %v", err)
	}
	var contract lifecycleVocabularyParity
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&contract); err != nil {
		t.Fatalf("ASSERT_LIFECYCLE_VOCABULARY_VECTOR_CLOSED: %v", err)
	}
	if contract.Version != "1" {
		t.Fatalf("ASSERT_LIFECYCLE_VOCABULARY_VERSION: got %q want %q", contract.Version, "1")
	}

	assertVocabularyEqual(t, "states", contract.States, stringsOf(session.PublicStates()))
	assertVocabularyEqual(t, "events", contract.Events, stringsOf(session.PublicEvents()))
	assertVocabularyEqual(t, "guards", contract.Guards, stringsOf(session.PublicGuards()))
	assertVocabularyEqual(t, "availability", contract.Availability, stringsOf(session.AvailabilityValues()))
	assertVocabularyEqual(t, "failures", contract.Failures, stringsOf(session.FailureValues()))
	assertVocabularyEqual(t, "restart", contract.Restart, stringsOf(session.RestartValues()))
	assertLifecycleFailureEnvelopeParity(t, contract.Failures)
}

func assertLifecycleFailureEnvelopeParity(t *testing.T, failures []string) {
	t.Helper()
	paths, err := filepath.Glob("testdata/schemas/envelope-session-*.v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	owned := map[string]bool{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties struct {
				Code struct {
					Const string   `json:"const"`
					Enum  []string `json:"enum"`
				} `json:"code"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		if schema.Properties.Code.Const != "" {
			owned[schema.Properties.Code.Const] = true
		}
		for _, code := range schema.Properties.Code.Enum {
			owned[code] = true
		}
	}
	for _, failure := range failures {
		// SESSION_STOPPING is a queued-request teardown result, not a lifecycle-tool envelope code.
		if failure == "SESSION_STOPPING" {
			continue
		}
		if !owned[failure] {
			t.Errorf("ASSERT_LIFECYCLE_FAILURE_ENVELOPE_PARITY: ADR-owned manager failure %q has no candidate envelope schema", failure)
		}
	}
}

func assertVocabularyEqual(t *testing.T, dimension string, contract, algebra []string) {
	t.Helper()
	want := append([]string(nil), contract...)
	got := append([]string(nil), algebra...)
	sort.Strings(want)
	sort.Strings(got)
	if hasDuplicate(want) || hasDuplicate(got) || !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_LIFECYCLE_VOCABULARY_PARITY[%s]: algebra=%v contract=%v", dimension, got, want)
	}
	t.Logf("ASSERT_LIFECYCLE_VOCABULARY_PARITY[%s]: pass", dimension)
}

func hasDuplicate(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] {
			return true
		}
	}
	return false
}

type stringValue interface{ ~string }

func stringsOf[T stringValue](values []T) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}
