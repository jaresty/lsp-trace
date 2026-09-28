package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/publication"
)

func proposalContextFixture(t *testing.T) (*publication.Root, proposalContextInputs) {
	t.Helper()
	root, r := responseReadFixture(t, "[]")
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
	identity := adr0011methodresult.PrivateTransitionIdentity{Transaction: expected.TransactionID, RequestKey: expected.RequestKey, Invocation: expected.InvocationID, ResponseRead: response.selector, RawSelector: raw.selector, RawDigest: privateDigest([]byte("[]"))}
	evaluation, journal, err := adr0011methodresult.RunPrivateAttachedReferences(root, r.Pair.Key, []byte("[]"), identity)
	if err != nil {
		t.Fatal(err)
	}
	events, err := publishEvents(root, response, raw, observed, evaluation, r, expected.TransactionID, []byte("[]"), journal, nil, nil)
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
	return root, proposalContextInputs{
		TargetInputs: r.TargetInputs, ResponseInputs: r, MethodInputs: mi, Method: method, Target: r.Target, ResponseRead: response,
		RawResult: raw, Scanner: scanner, Events: events, Policies: policies, Payload: []byte("[]"), Journal: journal,
		ScannerObservation: observed, Evaluation: evaluation, Git: r.TargetInputs.Git, SourceBytes: r.TargetInputs.Original,
		SchemaDigest: reviewedSuccessorSchemaDigest, ImplementationDigest: reviewedSyntheticSourceDigest, OccurrenceID: r.Query.OccurrenceID,
	}
}

func TestProposalContextSynthetic(t *testing.T) {
	root, x := proposalContextFixture(t)
	got, err := canonicalProposalContext(root, x, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(got) || !json.Valid(got) {
		t.Fatal("ASSERT_CONTEXT_UTF8_JSON")
	}
	contract, err := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if json.Unmarshal(contract, &schema) != nil {
		t.Fatal("schema")
	}
	id := schema.(map[string]any)["$id"].(string)
	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource(id, schema); err != nil {
		t.Fatal(err)
	}
	shape, err := compiler.Compile(id + "#/$defs/context")
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if json.Unmarshal(got, &value) != nil {
		t.Fatal("decode")
	}
	if err = shape.Validate(value); err != nil {
		t.Fatalf("ASSERT_CONTEXT_SCHEMA: %v", err)
	}
	fields := value.(map[string]any)
	roles := fields["schema_digests"].([]any)
	if len(roles) != len(proposalContextSchemaRoles) {
		t.Fatal("ASSERT_CONTEXT_ROLES")
	}
	for i, entry := range roles {
		item := entry.(map[string]any)
		if item["role"] != proposalContextSchemaRoles[i] || item["digest"] != proposalContextSchemaDigest {
			t.Fatal("ASSERT_CONTEXT_ALIASES_DIGEST")
		}
	}
	if fields["method_ref"].(map[string]any)["schema_version"] != methodRecordVersion || fields["target_ref"].(map[string]any)["schema_version"] != targetRecordVersion {
		t.Fatal("ASSERT_CONTEXT_VERSION_REFS")
	}
	if fields["implementation_digest"] != x.ImplementationDigest || fields["query_occurrence_id"] != x.OccurrenceID {
		t.Fatal("ASSERT_CONTEXT_OWNER_PINS")
	}
	if !bytes.Equal(targetIdentityStringJSON("é☃"), []byte(`"é☃"`)) || !validTargetIdentityString("é☃") || validTargetIdentityString("e\u0301") {
		t.Fatal("ASSERT_CONTEXT_JCS_UTF8_NFC")
	}
	// A camel-case $defs identifier cannot be substituted for an indexing alias.
	camel := bytes.Replace(got, []byte(`"role":"host-git"`), []byte(`"role":"hostGit"`), 1)
	var invalid any
	if bytes.Equal(camel, got) || json.Unmarshal(camel, &invalid) != nil || shape.Validate(invalid) == nil {
		t.Fatal("ASSERT_CONTEXT_REJECT_CAMEL_ROLE")
	}
	// No claimant context is an argument; mutations affect only owner expectations
	// or VERIFIED predecessor state, never a claim accepted by this constructor.
	tests := map[string]func(*proposalContextInputs){
		"implementation-empty":    func(y *proposalContextInputs) { y.ImplementationDigest = "" },
		"implementation-unpinned": func(y *proposalContextInputs) { y.ImplementationDigest = "claimant" },
		"schema":                  func(y *proposalContextInputs) { y.SchemaDigest = "other" },
		"occurrence":              func(y *proposalContextInputs) { y.OccurrenceID = "sha256:" + strings.Repeat("b", 64) },
		"source":                  func(y *proposalContextInputs) { y.SourceBytes = []byte("different") },
		"target-absent":           func(y *proposalContextInputs) { y.Target.stage = "ABSENT" },
		"method-absent":           func(y *proposalContextInputs) { y.Method.stage = "ABSENT" },
		"response-absent":         func(y *proposalContextInputs) { y.ResponseRead.stage = "ABSENT" },
		"raw-absent":              func(y *proposalContextInputs) { y.RawResult.stage = "ABSENT" },
		"scanner-absent":          func(y *proposalContextInputs) { y.Scanner.stage = "ABSENT" },
		"events-absent":           func(y *proposalContextInputs) { y.Events.stage = "ABSENT" },
		"policy-absent":           func(y *proposalContextInputs) { y.Policies[2].Record.stage = "ABSENT" },
		"policy-bytes-absent":     func(y *proposalContextInputs) { y.Policies[3].Bytes.stage = "ABSENT" },
		"prepared-absent":         func(y *proposalContextInputs) { y.TargetInputs.Prepared.stage = "ABSENT" },
		"source-identity-absent":  func(y *proposalContextInputs) { y.TargetInputs.Source.stage = "ABSENT" },
		"revision-absent":         func(y *proposalContextInputs) { y.TargetInputs.Revision.stage = "ABSENT" },
		"before-absent":           func(y *proposalContextInputs) { y.TargetInputs.Before.stage = "ABSENT" },
		"after-absent":            func(y *proposalContextInputs) { y.TargetInputs.After.stage = "ABSENT" },
		"owner-read-absent":       func(y *proposalContextInputs) { y.ResponseInputs.OwnerRead.stage = "ABSENT" },
		"target-result-absent":    func(y *proposalContextInputs) { y.TargetInputs.Result.stage = "ABSENT" },
		"payload-absent":          func(y *proposalContextInputs) { y.ResponseInputs.Payload.stage = "ABSENT" },
		"key":                     func(y *proposalContextInputs) { y.ResponseInputs.Pair.Key.ID++ },
		"transaction": func(y *proposalContextInputs) {
			y.Journal = bytes.Replace(y.Journal, []byte(fields["transaction_id"].(string)), []byte("sha256:"+strings.Repeat("c", 64)), 1)
		},
		"evaluation":          func(y *proposalContextInputs) { y.Evaluation = nil },
		"payload":             func(y *proposalContextInputs) { y.Payload = []byte("null") },
		"scanner-observation": func(y *proposalContextInputs) { y.ScannerObservation.knownE++ },
		"target-ref":          func(y *proposalContextInputs) { y.Target.digest = "sha256:" + strings.Repeat("d", 64) },
		"method-ref":          func(y *proposalContextInputs) { y.Method.digest = "sha256:" + strings.Repeat("e", 64) },
		"policy-ref":          func(y *proposalContextInputs) { y.Policies[0].Record.digest = "sha256:" + strings.Repeat("f", 64) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			y := x
			mutate(&y)
			if _, err := canonicalProposalContext(root, y, nil); err == nil {
				t.Fatal("ASSERT_CONTEXT_REJECT_SUBSTITUTION")
			}
		})
	}
}
