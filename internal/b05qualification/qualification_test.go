package b05qualification

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/incomingops"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
	"lsp-trace/sliceops"
)

type expectation struct {
	Kind           string   `json:"kind"`
	Terminal       string   `json:"terminal,omitempty"`
	Observations   []string `json:"observations"`
	Relations      []string `json:"relations"`
	Prohibited     []string `json:"prohibited"`
	Completeness   string   `json:"completeness"`
	RevisionPolicy string   `json:"revision_policy"`
}
type ledger struct {
	SchemaVersion string                 `json:"schema_version"`
	Families      map[string]expectation `json:"families"`
}

var requiredFamilies = []string{"advertised_no_prepare", "bounds", "callback_passage", "ember_glimmer_chain", "identity", "mixed_revision", "plain_js_calls", "red_validation", "unsupported_template"}
var requiredRed = []string{"callback_passage_as_invocation", "false_completeness", "mislabeled_calls", "omitted_revision", "runtime_claim", "state_update_as_render"}

func loadLedger(t *testing.T) ledger {
	t.Helper()
	raw, err := os.ReadFile("testdata/fixture-ledger.json")
	if err != nil {
		t.Fatal(err)
	}
	var got ledger
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func validateLedger(got ledger) error {
	if got.SchemaVersion != "lsp-trace.b05-qualification.v1" {
		return assertionError("ASSERT_B05_LEDGER_SCHEMA")
	}
	names := make([]string, 0, len(got.Families))
	for name := range got.Families {
		names = append(names, name)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, requiredFamilies) {
		return assertionError("ASSERT_B05_EXACT_NINE_FAMILIES")
	}
	for name, family := range got.Families {
		if family.Kind == "" || family.Completeness == "" || family.RevisionPolicy == "" {
			return assertionError("ASSERT_B05_EXACT_EXPECTATION_" + name)
		}
	}
	red := got.Families["red_validation"]
	for _, prohibited := range requiredRed {
		if !contains(red.Prohibited, prohibited) {
			return assertionError("ASSERT_B05_REJECT_" + prohibited)
		}
	}
	callback := got.Families["callback_passage"]
	if !contains(callback.Relations, "PASSES_CALLBACK") || contains(callback.Relations, "INVOKES_TASK") {
		return assertionError("ASSERT_B05_CALLBACK_NOT_INVOCATION")
	}
	noPrepare := got.Families["advertised_no_prepare"]
	if noPrepare.Terminal != "PREPARE_RETURNED_NO_ITEM" || noPrepare.Completeness != "UNKNOWN" || contains(noPrepare.Relations, "CALLS") {
		return assertionError("ASSERT_B05_NO_PREPARE_UNKNOWN_ZERO_CALLS")
	}
	unsupported := got.Families["unsupported_template"]
	if unsupported.Terminal != "ADAPTER_NOT_AVAILABLE" {
		return assertionError("ASSERT_B05_UNSUPPORTED_NOT_EMPTY")
	}
	mixed := got.Families["mixed_revision"]
	if mixed.Terminal != "MIXED_REVISION" || mixed.RevisionPolicy != "REJECT" {
		return assertionError("ASSERT_B05_MIXED_REVISION_FAIL_CLOSED")
	}
	identity := got.Families["identity"]
	if !contains(identity.Observations, "reordered_traversal_same_ids_and_digest") {
		return assertionError("ASSERT_B05_IDENTITY_REORDER_STABLE")
	}
	return nil
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestExactFixtureLedger(t *testing.T) {
	if err := validateLedger(loadLedger(t)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ASSERT_B05_LEDGER_SCHEMA", "ASSERT_B05_EXACT_NINE_FAMILIES", "ASSERT_B05_CALLBACK_NOT_INVOCATION", "ASSERT_B05_NO_PREPARE_UNKNOWN_ZERO_CALLS", "ASSERT_B05_UNSUPPORTED_NOT_EMPTY", "ASSERT_B05_MIXED_REVISION_FAIL_CLOSED", "ASSERT_B05_IDENTITY_REORDER_STABLE"} {
		t.Log("PASS " + id)
	}
}

func TestRedFixturesAreAssertionSpecific(t *testing.T) {
	base := loadLedger(t)
	cases := []struct {
		name, want string
		mutate     func(*ledger)
	}{
		{"mislabeled_calls", "ASSERT_B05_CALLBACK_NOT_INVOCATION", func(l *ledger) {
			f := l.Families["callback_passage"]
			f.Relations = []string{"CALLS", "INVOKES_TASK"}
			l.Families["callback_passage"] = f
		}},
		{"false_completeness", "ASSERT_B05_NO_PREPARE_UNKNOWN_ZERO_CALLS", func(l *ledger) {
			f := l.Families["advertised_no_prepare"]
			f.Completeness = "COMPLETE_WITHIN_BOUNDS"
			l.Families["advertised_no_prepare"] = f
		}},
		{"omitted_revision", "ASSERT_B05_EXACT_EXPECTATION_mixed_revision", func(l *ledger) {
			f := l.Families["mixed_revision"]
			f.RevisionPolicy = ""
			l.Families["mixed_revision"] = f
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clone := base
			clone.Families = map[string]expectation{}
			for k, v := range base.Families {
				clone.Families[k] = v
			}
			tc.mutate(&clone)
			err := validateLedger(clone)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("%s: got %v", tc.want, err)
			}
			t.Log("FAIL " + tc.want + " / PASS restored")
		})
	}
}

type managedRuntime struct{ calls []string }

func (*managedRuntime) Records() []sessionruntime.Record { return nil }
func (*managedRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return sessionruntime.SessionMetadata{PositionEncoding: "utf-16", CallHierarchySupport: true}, ""
}
func (r *managedRuntime) RoundTrip(_ context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	r.calls = append(r.calls, req.Method)
	item := `{"name":"root","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}}}`
	switch req.Method {
	case "textDocument/prepareCallHierarchy":
		return sessionruntime.RoundTripResult{Result: json.RawMessage(`[` + item + `]`)}
	case "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
		return sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}
	}
	return sessionruntime.RoundTripResult{Failure: session.RequestTimeout}
}

func TestRealManagedIncomingAndSliceAcceptance(t *testing.T) {
	r := &managedRuntime{}
	incoming, failure := incomingops.NewExecutor(r).Execute(context.Background(), operation.Request{Name: incomingops.OperationIncoming, Input: json.RawMessage(`{"session_id":"s","generation":1,"uri":"file:///w/a.go","line":0,"character":0}`)})
	if failure != nil || !strings.Contains(string(incoming.Artifact), `"schema_version":"lsp-trace.graph.v3"`) {
		t.Fatalf("ASSERT_B05_MANAGED_INCOMING: %v %s", failure, incoming.Artifact)
	}
	slice, failure := sliceops.NewExecutor(r).Execute(context.Background(), operation.Request{Name: sliceops.OperationSlice, Input: json.RawMessage(`{"session_id":"s","generation":1,"start_mode":"at","uri":"file:///w/a.go","line":0,"character":0,"down_depth":1,"up_depth":1}`)})
	if failure != nil || !strings.Contains(string(slice.Artifact), `"schema_version":"lsp-trace.graph.v3"`) {
		t.Fatalf("ASSERT_B05_MANAGED_SLICE: %v %s", failure, slice.Artifact)
	}
	if strings.Contains(string(incoming.Artifact), "PASSES_CALLBACK") || strings.Contains(string(slice.Artifact), "PASSES_CALLBACK") {
		t.Fatal("ASSERT_B05_CALLS_ONLY_COMPATIBILITY")
	}
	t.Log("PASS ASSERT_B05_MANAGED_INCOMING")
	t.Log("PASS ASSERT_B05_MANAGED_SLICE")
	t.Log("PASS ASSERT_B05_CALLS_ONLY_COMPATIBILITY")
}
