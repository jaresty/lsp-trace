package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/publication"
)

func proposalRecordFixture(t *testing.T, token string) (*publication.Root, proposalContextInputs) {
	t.Helper()
	root, r := responseReadFixture(t, token)
	response, err := publishResponseRead(root, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := publishRawResult(root, response, r.Payload, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := publishScannerRecord(root, response, raw, r.Payload, r, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := observeRawScanner(root, r.Payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, ok := responseReadExpected(root, r, publication.ReadVerifiedBoundFile)
	if !ok {
		t.Fatal("response")
	}
	identity := adr0011methodresult.PrivateTransitionIdentity{Transaction: expected.TransactionID, RequestKey: expected.RequestKey, Invocation: expected.InvocationID, ResponseRead: response.selector, RawSelector: raw.selector, RawDigest: privateDigest([]byte(token))}
	evaluation, journal, err := adr0011methodresult.RunPrivateAttachedReferences(root, r.Pair.Key, []byte(token), identity)
	if err != nil {
		t.Fatal(err)
	}
	events, err := publishEvents(root, response, raw, observed, evaluation, r, expected.TransactionID, []byte(token), journal, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	mi := methodRecordInputs{r, response, raw}
	method, err := publishMethodRecord(root, mi, nil)
	if err != nil {
		t.Fatal(err)
	}
	var policies [4]policyPublication
	for i, selected := range policySelections {
		policies[i], err = publishPolicy(root, selected, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return root, proposalContextInputs{TargetInputs: r.TargetInputs, ResponseInputs: r, MethodInputs: mi, Method: method, Target: r.Target, ResponseRead: response, RawResult: raw, Scanner: scanner, Events: events, Policies: policies, Payload: []byte(token), Journal: journal, ScannerObservation: observed, Evaluation: evaluation, Git: r.TargetInputs.Git, SourceBytes: r.TargetInputs.Original, SchemaDigest: reviewedSuccessorSchemaDigest, ImplementationDigest: reviewedSyntheticSourceDigest, OccurrenceID: r.Query.OccurrenceID}
}

func TestPrivateProposalSynthetic(t *testing.T) {
	contract, err := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if err := json.Unmarshal(contract, &schema); err != nil {
		t.Fatal(err)
	}
	id := schema.(map[string]any)["$id"].(string)
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(id, schema); err != nil {
		t.Fatal(err)
	}
	shape, err := compiler.Compile(id + "#/$defs/proposal")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"null", "[]", "[" + eventsLocation + "," + eventsLocation + "]"} {
		t.Run(token, func(t *testing.T) {
			root, x := proposalRecordFixture(t, token)
			b, err := canonicalProposalRecord(root, x, nil)
			if err != nil {
				t.Fatal(err)
			}
			var value any
			if json.Unmarshal(b, &value) != nil || shape.Validate(value) != nil {
				t.Fatalf("ASSERT_PROPOSAL_SCHEMA: %s %v", b, shape.Validate(value))
			}
			fields := value.(map[string]any)
			if fields["raw_result_digest"] != privateDigest([]byte(token)) || int(fields["raw_result_byte_length"].(float64)) != len(token) || fields["raw_result_presence"] != "PRESENT" || fields["top_level_form"] != scanRawTopLevel([]byte(token)).form {
				t.Fatal("ASSERT_PROPOSAL_RAW_SCANNER")
			}
			if fields["context"].(map[string]any)["implementation_digest"] != x.ImplementationDigest || fields["observed_b"] != float64(1) || fields["whole_result_p"] != float64(x.ScannerObservation.knownE) {
				t.Fatal("ASSERT_PROPOSAL_CONTEXT_EVENTS")
			}
			state, err := publishProposalRecord(root, x, nil, nil)
			if err != nil || state.stage != "VERIFIED" || !replayProposalRecord(root, state, x, nil) {
				t.Fatalf("ASSERT_PROPOSAL_VERIFIED: %+v %v", state, err)
			}
			got, err := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
			if err != nil || !bytes.Equal(got, b) || state.digest != proposalRecordDigest(b) || state.selector != proposalRecordSelector(state.digest) {
				t.Fatal("ASSERT_PROPOSAL_EXACT")
			}
			changed := x
			changed.ImplementationDigest = "sha256:" + strings.Repeat("b", 64)
			if replayProposalRecord(root, state, changed, nil) {
				t.Fatal("ASSERT_PROPOSAL_CONTEXT_SWAP")
			}
			changed = x
			changed.Events.digest = "sha256:" + strings.Repeat("c", 64)
			if replayProposalRecord(root, state, changed, nil) {
				t.Fatal("ASSERT_PROPOSAL_EVENTS_SWAP")
			}
			changed = x
			changed.Payload = []byte("different")
			if replayProposalRecord(root, state, changed, nil) {
				t.Fatal("ASSERT_PROPOSAL_RAW_SWAP")
			}
			again, err := publishProposalRecord(root, x, nil, nil)
			if err == nil || again.stage != "COMMITTED_UNVERIFIED" {
				t.Fatalf("ASSERT_PROPOSAL_NO_REPLACE: %+v %v", again, err)
			}
		})
	}
}

func TestPrivateProposalCommitStages(t *testing.T) {
	root, x := proposalRecordFixture(t, "[]")
	changed := x
	changed.ScannerObservation.knownE++
	state, err := publishProposalRecord(root, changed, nil, nil)
	if err == nil || state.stage != "ABSENT" {
		t.Fatalf("ASSERT_PROPOSAL_PRECOMMIT: %+v %v", state, err)
	}
	b, err := canonicalProposalRecord(root, x, nil)
	if err != nil {
		t.Fatal(err)
	}
	selector := proposalRecordSelector(proposalRecordDigest(b))
	fired := false
	trace := func(event publication.BoundFileTraceEvent) {
		if !fired && event.Stage == "HARDLINK" && event.Result == "INSTALLED" {
			fired = true
			_ = os.Remove(filepath.Join(root.Path(), selector))
		}
	}
	state, err = publishProposalRecord(root, x, nil, trace)
	if !fired || err == nil || state.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_PROPOSAL_POSTCOMMIT: %+v %v", state, err)
	}
}
