package adr0011methodresult

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
)

func remintCandidate(t *testing.T, raw []byte, key uint64, result string) []byte {
	t.Helper()
	c, err := VerifyCanonicalCandidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	c.ID = ""
	c.KeyID = key
	c.ResultBase64 = base64.StdEncoding.EncodeToString([]byte(result))
	c.ResultSHA256 = rawSHA([]byte(result))
	c.Null = result == "null"
	c.ItemCount = 0
	pre, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	c.ID = queryResultCandidateDigest(pre)
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if _, err := VerifyCanonicalCandidate(encoded); err != nil {
		t.Fatalf("remint fixture: %v", err)
	}
	return encoded
}

func TestCompleteCandidateTransactionDenominator(t *testing.T) {
	raw, expected := retentionFixture(t)
	array := remintCandidate(t, raw, 3, "[]")
	sameKey := remintCandidate(t, raw, 2, "[]")
	q := func(id string) CandidateQuery {
		return CandidateQuery{OccurrenceID: id, SessionID: expected.SessionID, Generation: expected.Generation, Method: expected.Method, ParamsSHA256: expected.ParamsSHA256}
	}
	m := func(id string, form EnvelopeForm) CandidateQueryMember {
		return CandidateQueryMember{OccurrenceID: id, Began: true, Terminal: CandidateEmpty, Envelope: EnvelopeObservation{Known: true, Form: form}}
	}
	good := CandidateLedger{Queries: []CandidateQuery{q("one"), q("two")}, Members: []CandidateQueryMember{m("one", EnvelopeNull), m("two", EnvelopeArray)}}
	bindings := []CandidateBinding{{"one", raw}, {"two", array}}
	if err := CheckCompleteCandidateTransactionLedger(bindings, good); err != nil {
		t.Fatalf("ASSERT_COMPLETE_DENOMINATOR_PASS: %v", err)
	}
	for _, tc := range []struct {
		name     string
		bindings []CandidateBinding
		ledger   CandidateLedger
	}{
		{"missing-candidate", bindings[:1], good},
		{"missing-member", bindings, CandidateLedger{Queries: good.Queries, Members: good.Members[:1]}},
		{"duplicate-occurrence", []CandidateBinding{{"one", raw}, {"one", array}}, good},
		{"undeclared-occurrence", []CandidateBinding{{"one", raw}, {"other", array}}, good},
		{"replayed-candidate", []CandidateBinding{{"one", raw}, {"two", raw}}, good},
		{"duplicate-request-key", []CandidateBinding{{"one", raw}, {"two", sameKey}}, good},
		{"wrong-query-digest", bindings, CandidateLedger{Queries: []CandidateQuery{q("one"), {OccurrenceID: "two", SessionID: expected.SessionID, Generation: expected.Generation, Method: expected.Method, ParamsSHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}}, Members: good.Members}},
		{"null-vs-array", bindings, CandidateLedger{Queries: good.Queries, Members: []CandidateQueryMember{m("one", EnvelopeArray), good.Members[1]}}},
		{"pending-member", bindings, CandidateLedger{Queries: good.Queries, Members: []CandidateQueryMember{good.Members[0], {OccurrenceID: "two", Began: true, Envelope: EnvelopeObservation{Known: true, Form: EnvelopeArray}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckCompleteCandidateTransactionLedger(tc.bindings, tc.ledger); !errors.Is(err, ErrCandidateJoin) {
				t.Fatalf("ASSERT_COMPLETE_DENOMINATOR_REJECT: %v", err)
			}
		})
	}
}
