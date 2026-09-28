package adr0011acquisition

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"testing"

	"lsp-trace/internal/publication"
)

func TestPrivatePolicyReceipts(t *testing.T) {
	for _, s := range policySelections {
		t.Run(s.name, func(t *testing.T) {
			root := privatePublicationRoot(t)
			b, err := selectedPolicyBytes(s, nil)
			if err != nil {
				t.Fatal(err)
			}
			state, err := publishPolicy(root, s, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !replayPolicy(root, s, state, nil, nil) {
				t.Fatal("replay")
			}
			if state.Record.digest != sourceRoleDigest(s.role, []byte(`{"policy_bytes_ref":{"digest":"`+privateDigest(b)+`","selector":"`+policyBytesSelector(privateDigest(b))+`"},"schema_version":"`+s.role+`"}`)) {
				t.Fatal("role digest")
			}
			if state.Bytes.selector == state.Record.selector || state.Bytes.stage != "VERIFIED" || state.Record.stage != "VERIFIED" {
				t.Fatal("separate verified receipts")
			}
			other := policySelections[(indexPolicy(s)+1)%4]
			if replayPolicy(root, other, state, nil, nil) {
				t.Fatal("role swap accepted")
			}
			duplicate := append([]byte(nil), b...)
			duplicate = append(duplicate[:len(duplicate)-1], []byte(`,"schema_version":"`+s.role+`"}`)...)
			for _, bad := range [][]byte{nil, append([]byte(nil), b[:len(b)-1]...), append(append([]byte(nil), b...), '\n'), duplicate, append(append([]byte(nil), b[:len(b)-1]...), []byte(`,"extra":null}`)...)} {
				source := func(string) ([]byte, error) { return bad, nil }
				if replayPolicy(root, s, state, source, nil) {
					t.Fatal("changed policy replayed")
				}
				got, e := publishPolicy(root, s, source, nil)
				if e == nil || got.Bytes.stage != "ABSENT" || got.Record.stage != "ABSENT" {
					t.Fatal("changed source published", got, e)
				}
			}
			missing := func(string) ([]byte, error) { return nil, os.ErrNotExist }
			if replayPolicy(root, s, state, missing, nil) {
				t.Fatal("missing source")
			}
			// A failed readback cannot promote a committed receipt.
			calls := 0
			failedRead := func(r *publication.Root, selector string, max int64) ([]byte, error) {
				calls++
				return nil, errors.New("readback")
			}
			_ = calls
			another := privatePublicationRoot(t)
			got, e := publishPolicy(another, s, nil, failedRead)
			if e == nil || got.Bytes.stage != "COMMITTED_UNVERIFIED" || got.Record.stage != "ABSENT" {
				t.Fatal("readback promoted", got, e)
			}
			if _, e = publishPolicy(root, s, nil, nil); e == nil {
				t.Fatal("no-replace collision accepted")
			}
		})
	}
}
func TestPolicyRecordCollisionAndSourceSwap(t *testing.T) {
	s := policySelections[0]
	original, e := selectedPolicyBytes(s, nil)
	if e != nil {
		t.Fatal(e)
	}
	root := privatePublicationRoot(t)
	canonical, _ := canonicalPolicyRecord(s, original)
	digest := sourceRoleDigest(s.role, canonical)
	selector := policyRecordSelector(s, digest)
	// An existing selector is never adopted even if its spelling appears correct.
	_, e = publication.PublishBoundFile(root, selector, []byte(`{"unexpected":null}`), func([]byte) error { return nil })
	if e != nil {
		t.Fatal(e)
	}
	state, e := publishPolicy(root, s, nil, nil)
	if e == nil || state.Record.stage != "COMMITTED_UNVERIFIED" {
		t.Fatal("record collision", state, e)
	}
	root = privatePublicationRoot(t)
	count := 0
	changing := func(string) ([]byte, error) {
		count++
		if count > 1 {
			return append(append([]byte(nil), original...), ' '), nil
		}
		return original, nil
	}
	state, e = publishPolicy(root, s, changing, nil)
	if e == nil || state.Bytes.stage != "VERIFIED" || state.Record.stage != "ABSENT" {
		t.Fatal("source swap", state, e)
	}
	root = privatePublicationRoot(t)
	state, e = publishPolicy(root, s, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{[]byte(`{"policy_bytes_ref":null,"schema_version":"` + s.role + `"}`), append(append([]byte(nil), canonical[:len(canonical)-1]...), []byte(`,"extra":0}`)...), append(append([]byte(nil), canonical[:len(canonical)-1]...), []byte(`,"schema_version":"`+s.role+`"}`)...)} {
		read := func(r *publication.Root, selector string, max int64) ([]byte, error) {
			if selector == state.Record.selector {
				return bad, nil
			}
			return publication.ReadVerifiedBoundFile(r, selector, max)
		}
		if replayPolicy(root, s, state, nil, read) {
			t.Fatal("nonclosed record replayed", string(bad))
		}
	}
}
func TestPolicyRecordSuccessorSchema(t *testing.T) {
	contract, e := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if e != nil {
		t.Fatal(e)
	}
	var document map[string]any
	if e = json.Unmarshal(contract, &document); e != nil {
		t.Fatal(e)
	}
	id := document["$id"].(string)
	compiler := jsonschema.NewCompiler()
	if e = compiler.AddResource(id, document); e != nil {
		t.Fatal(e)
	}
	schema, e := compiler.Compile(id + "#/$defs/policy")
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range policySelections {
		b, e := selectedPolicyBytes(s, nil)
		if e != nil {
			t.Fatal(e)
		}
		record, e := canonicalPolicyRecord(s, b)
		if e != nil {
			t.Fatal(e)
		}
		var v any
		if e = json.Unmarshal(record, &v); e != nil {
			t.Fatal(e)
		}
		if e = schema.Validate(v); e != nil {
			t.Fatalf("%s: %v", s.name, e)
		}
		for _, bad := range [][]byte{[]byte(`{"schema_version":"` + s.role + `","policy_bytes_ref":null}`), []byte(`{"schema_version":"` + s.role + `","policy_bytes_ref":{"selector":"x","digest":"` + privateDigest(b) + `"}}`), []byte(`{"schema_version":"` + s.role + `","policy_bytes_ref":{"selector":"` + policyBytesSelector(privateDigest(b)) + `","digest":"` + privateDigest(b) + `","extra":null}}`)} {
			if json.Unmarshal(bad, &v) != nil {
				t.Fatal("bad fixture")
			}
			if schema.Validate(v) == nil {
				t.Fatalf("%s schema accepted %s", s.name, bad)
			}
		}
	}
}
func indexPolicy(s policySelection) int {
	for i, p := range policySelections {
		if p == s {
			return i
		}
	}
	return -1
}
func TestPolicySourcePinsAndCanonicalClosure(t *testing.T) {
	for _, s := range policySelections {
		b, e := diskPolicySource(policySourcePath(s))
		if e != nil {
			t.Fatal(e)
		}
		if !selectedPolicyOK(s, b) {
			t.Fatalf("%s source pin or canonical", s.name)
		}
	}
}
func selectedPolicyOK(s policySelection, b []byte) bool {
	_, e := selectedPolicyBytes(s, func(string) ([]byte, error) { return b, nil })
	return e == nil
}
