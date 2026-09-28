package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/publication"
)

func TestPrivateCandidateSynthetic(t *testing.T) {
	contract, err := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if json.Unmarshal(contract, &schema) != nil {
		t.Fatal("schema decode")
	}
	id := schema.(map[string]any)["$id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err = compiler.AddResource(id, schema); err != nil {
		t.Fatal(err)
	}
	shape, err := compiler.Compile(id + "#/$defs/candidate")
	if err != nil {
		t.Fatal(err)
	}
	var empty []byte
	for _, token := range []string{"null", "[]", "[" + eventsLocation + "," + eventsLocation + "]"} {
		t.Run(token, func(t *testing.T) {
			root, x := proposalRecordFixture(t, token)
			proposal, err := publishProposalRecord(root, x, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			b, err := canonicalCandidateRecord(root, proposal, x, nil)
			if err != nil {
				t.Fatal(err)
			}
			var value any
			if json.Unmarshal(b, &value) != nil || shape.Validate(value) != nil {
				t.Fatalf("ASSERT_CANDIDATE_SCHEMA: %s %v", b, shape.Validate(value))
			}
			fields := value.(map[string]any)
			items := fields["occurrences"].([]any)
			if len(items) != x.ScannerObservation.knownE || int(fields["proposed_p"].(float64)) != len(items) {
				t.Fatal("ASSERT_CANDIDATE_COUNT")
			}
			if len(items) == 2 {
				a, c := items[0].(map[string]any), items[1].(map[string]any)
				if a["ordinal"] != float64(0) || c["ordinal"] != float64(1) || a["returned_uri"] != c["returned_uri"] || a["occurrence_id"] == c["occurrence_id"] {
					t.Fatal("ASSERT_CANDIDATE_DISTINCT_ORDINAL")
				}
				ctx := fields["context"].(map[string]any)
				canonical := func(v any) []byte {
					b, e := json.Marshal(v)
					if e != nil {
						t.Fatal(e)
					}
					return b
				}
				independent := func(ordinal int, uri string, rng any) string {
					parts := [][]byte{[]byte("REFERENCES_OCCURRENCE_V1"), []byte(ctx["transaction_id"].(string)), []byte(ctx["query_occurrence_id"].(string)), canonical(ctx["method_ref"]), canonical(ctx["target_ref"]), []byte(strconv.Itoa(ordinal)), canonical(uri), canonical(rng)}
					h := sha256.New()
					for _, part := range parts {
						var n [8]byte
						binary.BigEndian.PutUint64(n[:], uint64(len(part)))
						h.Write(n[:])
						h.Write(part)
					}
					return fmt.Sprintf("sha256:%x", h.Sum(nil))
				}
				if a["occurrence_id"] != independent(0, a["returned_uri"].(string), a["returned_range"]) || c["occurrence_id"] != independent(1, c["returned_uri"].(string), c["returned_range"]) {
					t.Fatal("ASSERT_CANDIDATE_INDEPENDENT_LP")
				}
				if a["occurrence_id"] == independent(1, a["returned_uri"].(string), a["returned_range"]) || a["occurrence_id"] == independent(0, "file:///swapped", a["returned_range"]) {
					t.Fatal("ASSERT_CANDIDATE_SWAP_ID")
				}
			}
			if token == "null" {
				empty = b
			}
			if token == "[]" && bytes.Equal(empty, b) {
				t.Fatal("ASSERT_CANDIDATE_EMPTY_IDENTITIES")
			}
			state, err := publishCandidateRecord(root, proposal, x, nil, nil)
			if err != nil || state.stage != "VERIFIED" || !replayCandidateRecord(root, state, proposal, x, nil) {
				t.Fatalf("ASSERT_CANDIDATE_VERIFIED: %+v %v", state, err)
			}
			changed := x
			changed.Payload = []byte("[]")
			if token != "[]" && replayCandidateRecord(root, state, proposal, changed, nil) {
				t.Fatal("ASSERT_CANDIDATE_RAW_SWAP")
			}
			changed = x
			changed.ImplementationDigest = "sha256:" + strings.Repeat("b", 64)
			if replayCandidateRecord(root, state, proposal, changed, nil) {
				t.Fatal("ASSERT_CANDIDATE_CONTEXT_SWAP")
			}
			again, err := publishCandidateRecord(root, proposal, x, nil, nil)
			if err == nil || again.stage != "COMMITTED_UNVERIFIED" {
				t.Fatalf("ASSERT_CANDIDATE_COLLISION: %+v %v", again, err)
			}
		})
	}
}

func TestPrivateCandidateRawParserRejectsMalformedAndLimit(t *testing.T) {
	for _, raw := range []string{`[{"uri":"file:///a","uri":"file:///b","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}}]`, `[{"uri":"file:///a","range":{"start":{"line":0,"line":1,"character":0},"end":{"line":0,"character":0}}}]`, `[{"targetUri":"file:///a"}]`} {
		if _, failure := adr0011methodresult.ParseRawReferences([]byte(raw), 1000); failure == nil {
			t.Fatalf("ASSERT_CANDIDATE_RAW_MALFORMED: %s", raw)
		}
	}
	if _, failure := adr0011methodresult.ParseRawReferences([]byte("["+eventsLocation+","+eventsLocation+"]"), 1); failure == nil || failure.Code != adr0011methodresult.FailureResource {
		t.Fatal("ASSERT_CANDIDATE_RAW_LIMIT")
	}
}

func TestPrivateCandidateCommitStages(t *testing.T) {
	root, x := proposalRecordFixture(t, "[]")
	absent, err := publishCandidateRecord(root, privateBodyPublication{stage: "ABSENT"}, x, nil, nil)
	if err == nil || absent.stage != "ABSENT" {
		t.Fatal("ASSERT_CANDIDATE_MISSING_PREDECESSOR")
	}
	proposal, err := publishProposalRecord(root, x, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	changed := x
	changed.ScannerObservation.knownE++
	absent, err = publishCandidateRecord(root, proposal, changed, nil, nil)
	if err == nil || absent.stage != "ABSENT" {
		t.Fatal("ASSERT_CANDIDATE_PRECOMMIT")
	}
	b, err := canonicalCandidateRecord(root, proposal, x, nil)
	if err != nil {
		t.Fatal(err)
	}
	selector := candidateRecordSelector(candidateRecordDigest(b))
	fired := false
	trace := func(event publication.BoundFileTraceEvent) {
		if !fired && event.Stage == "HARDLINK" && event.Result == "INSTALLED" {
			fired = true
			_ = os.Remove(filepath.Join(root.Path(), selector))
		}
	}
	state, err := publishCandidateRecord(root, proposal, x, nil, trace)
	if !fired || err == nil || state.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_CANDIDATE_POSTCOMMIT: %+v %v", state, err)
	}
}
