package adr0011methodresult

import (
	"context"
	"strings"
	"testing"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/sessionruntime"
)

const candidateLocation = `{"uri":"file:///w/a.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`

func candidateQuery(id, method string) CandidateQuery {
	return CandidateQuery{OccurrenceID: id, SessionID: "exact-fixture", Generation: 7, Method: method, ParamsSHA256: "sha256:" + strings.Repeat("a", 64)}
}

func candidateEnvelope(t *testing.T, method, raw string, maxElements int) EnvelopeObservation {
	t.Helper()
	f := &fixtureRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}, response: fixtureResponse(raw)}
	wire := transport.New(f).Execute(context.Background(), fixtureRequest(method, false))
	if wire.Outcome() != transport.OutcomeTransportSuccess {
		t.Fatalf("fixture transport outcome: %s", wire.Outcome())
	}
	return ObserveEnvelope(wire, maxElements)
}

func TestCandidateLedgerBalancesDistinctQueryAndElementUnits(t *testing.T) {
	def := candidateQuery("definition-query", transport.MethodDefinition)
	refs := candidateQuery("references-query", transport.MethodReferences)
	valid := CandidateElement{Ordinal: 0, Began: true, Terminal: CandidateValidElement}
	second := CandidateElement{Ordinal: 1, Began: true, Terminal: CandidateValidElement}
	for _, tc := range []struct {
		name   string
		ledger CandidateLedger
	}{
		{"definition-null", CandidateLedger{Queries: []CandidateQuery{def}, Members: []CandidateQueryMember{{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: candidateEnvelope(t, def.Method, `null`, 3)}}}},
		{"references-empty", CandidateLedger{Queries: []CandidateQuery{refs}, Members: []CandidateQueryMember{{OccurrenceID: refs.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: candidateEnvelope(t, refs.Method, `[]`, 3)}}}},
		{"definition-repeated", CandidateLedger{Queries: []CandidateQuery{def}, Members: []CandidateQueryMember{{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: candidateEnvelope(t, def.Method, `[`+candidateLocation+`,`+candidateLocation+`]`, 3), Elements: []CandidateElement{valid, second}}}}},
		{"references-repeated", CandidateLedger{Queries: []CandidateQuery{refs}, Members: []CandidateQueryMember{{OccurrenceID: refs.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: candidateEnvelope(t, refs.Method, `[`+candidateLocation+`,`+candidateLocation+`]`, 3), Elements: []CandidateElement{valid, second}}}}},
		{"malformed-second-of-three", CandidateLedger{Queries: []CandidateQuery{refs}, Members: []CandidateQueryMember{{OccurrenceID: refs.OccurrenceID, Began: true, Terminal: CandidateMalformed, Envelope: candidateEnvelope(t, refs.Method, `[`+candidateLocation+`,{"uri":"file:///w/b.go","uri":"file:///w/c.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}},`+candidateLocation+`]`, 3), Elements: []CandidateElement{valid, {Ordinal: 1, Began: true, Terminal: CandidateMalformedElement}}}}}},
		{"preflight-no-dispatch", CandidateLedger{Queries: []CandidateQuery{def}}},
		{"timeout-before-output", CandidateLedger{Queries: []CandidateQuery{refs}}},
		{"over-element-limit", CandidateLedger{Queries: []CandidateQuery{def}, Members: []CandidateQueryMember{{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateLimited, Envelope: candidateEnvelope(t, def.Method, `[`+candidateLocation+`,`+candidateLocation+`]`, 1)}}}},
		{"declared-two-begun-one", CandidateLedger{Queries: []CandidateQuery{def, refs}, Members: []CandidateQueryMember{{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: candidateEnvelope(t, def.Method, `null`, 3)}}}},
		{"pending-element-not-terminal-query", CandidateLedger{Queries: []CandidateQuery{def}, Members: []CandidateQueryMember{{OccurrenceID: def.OccurrenceID, Began: true, Envelope: candidateEnvelope(t, def.Method, `[`+candidateLocation+`,`+candidateLocation+`]`, 3), Elements: []CandidateElement{valid, {Ordinal: 1, Began: true}}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckCandidateLedger(tc.ledger); err != nil {
				t.Fatalf("ASSERT_ADR0011_CANDIDATE_LEDGER_VALID: %v", err)
			}
		})
	}
}

func TestCandidateLedgerRejectsFabricatedTerminalsAndOrdinals(t *testing.T) {
	def := candidateQuery("definition-query", transport.MethodDefinition)
	refs := candidateQuery("references-query", transport.MethodReferences)
	empty := candidateEnvelope(t, def.Method, `null`, 3)
	two := candidateEnvelope(t, def.Method, `[`+candidateLocation+`,`+candidateLocation+`]`, 3)
	over := candidateEnvelope(t, def.Method, `[`+candidateLocation+`,`+candidateLocation+`]`, 1)
	good := CandidateElement{Ordinal: 0, Began: true, Terminal: CandidateValidElement}
	base := func(member CandidateQueryMember) CandidateLedger {
		return CandidateLedger{Queries: []CandidateQuery{def}, Members: []CandidateQueryMember{member}}
	}
	for _, tc := range []struct {
		name   string
		ledger CandidateLedger
	}{
		{"no-declaration", CandidateLedger{}},
		{"duplicate-declaration", CandidateLedger{Queries: []CandidateQuery{def, def}}},
		{"too-many-declarations", func() CandidateLedger {
			queries := make([]CandidateQuery, 17)
			for i := range queries {
				queries[i] = candidateQuery(string(rune('a'+i)), transport.MethodDefinition)
			}
			return CandidateLedger{Queries: queries}
		}()},
		{"invalid-query-digest", CandidateLedger{Queries: []CandidateQuery{{OccurrenceID: def.OccurrenceID, SessionID: def.SessionID, Generation: def.Generation, Method: def.Method, ParamsSHA256: "sha256:forged"}}}},
		{"undeclared-query", base(CandidateQueryMember{OccurrenceID: "other", Began: true, Terminal: CandidateEmpty, Envelope: empty})},
		{"timeout-presented-as-begun", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: EnvelopeObservation{Reason: EnvelopeNonSuccess}})},
		{"missing-raw-presented-as-begun", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: EnvelopeObservation{Reason: EnvelopeUnavailable}})},
		{"duplicate-terminal", CandidateLedger{Queries: []CandidateQuery{def}, Members: []CandidateQueryMember{{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: empty}, {OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: empty}}}},
		{"terminal-without-begin", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Terminal: CandidateEmpty, Envelope: empty})},
		{"empty-from-malformed", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: EnvelopeObservation{Reason: EnvelopeMalformed}})},
		{"empty-from-overlimit", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: over})},
		{"empty-with-returned-elements", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateEmpty, Envelope: two})},
		{"items-with-missing-element", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: two, Elements: []CandidateElement{good}})},
		{"items-with-pending-element", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: two, Elements: []CandidateElement{good, {Ordinal: 1, Began: true}}})},
		{"duplicate-element-ordinal", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: two, Elements: []CandidateElement{good, good}})},
		{"out-of-bounds-element", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: two, Elements: []CandidateElement{good, {Ordinal: 2, Began: true, Terminal: CandidateValidElement}}})},
		{"element-terminal-without-begin", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: two, Elements: []CandidateElement{good, {Ordinal: 1, Terminal: CandidateValidElement}}})},
		{"malformed-then-later-begun", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateMalformed, Envelope: EnvelopeObservation{Known: true, Form: EnvelopeArray, Count: 3}, Elements: []CandidateElement{good, {Ordinal: 1, Began: true, Terminal: CandidateMalformedElement}, {Ordinal: 2, Began: true, Terminal: CandidateValidElement}}})},
		{"malformed-without-failing-element", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateMalformed, Envelope: two, Elements: []CandidateElement{good}})},
		{"limit-from-known-empty", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateLimited, Envelope: empty})},
		{"items-from-unknown-count", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: over})},
		{"nonterminal-with-terminal-element-beyond-count", base(CandidateQueryMember{OccurrenceID: def.OccurrenceID, Began: true, Envelope: empty, Elements: []CandidateElement{good}})},
		{"references-scalar-form", CandidateLedger{Queries: []CandidateQuery{refs}, Members: []CandidateQueryMember{{OccurrenceID: refs.OccurrenceID, Began: true, Terminal: CandidateItems, Envelope: EnvelopeObservation{Known: true, Form: EnvelopeDefinitionScalar, Count: 1}, Elements: []CandidateElement{good}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckCandidateLedger(tc.ledger); err == nil {
				t.Fatal("ASSERT_ADR0011_CANDIDATE_LEDGER_REJECTS_FABRICATION: accepted")
			}
		})
	}
}

func TestCandidateLedgerKeepsRawElementsSeparateFromParserItems(t *testing.T) {
	const malformedSecond = `{"uri":"file:///w/b.go","uri":"file:///w/c.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`
	refs := candidateQuery("references-query", transport.MethodReferences)
	f := &fixtureRuntime{metadata: sessionruntime.SessionMetadata{ReferencesSupport: true}, response: fixtureResponse(`[` + candidateLocation + `,` + malformedSecond + `,` + candidateLocation + `]`)}
	wire := transport.New(f).Execute(context.Background(), fixtureRequest(refs.Method, false))
	parsed, failure := Parse(wire, 3)
	e := ObserveEnvelope(wire, 3)
	if failure == nil || failure.Code != FailureMalformed || failure.Ordinal != 1 || len(parsed.Items) != 0 || !e.Known || e.Count != 3 {
		t.Fatalf("ASSERT_ADR0011_LEDGER_E_NOT_P: result=%+v failure=%+v envelope=%+v", parsed, failure, e)
	}
	member := CandidateQueryMember{OccurrenceID: refs.OccurrenceID, Began: true, Terminal: CandidateMalformed, Envelope: e,
		Elements: []CandidateElement{{Ordinal: 0, Began: true, Terminal: CandidateValidElement}, {Ordinal: 1, Began: true, Terminal: CandidateMalformedElement}}}
	ledger := CandidateLedger{Queries: []CandidateQuery{refs}, Members: []CandidateQueryMember{member}}
	if err := CheckCandidateLedger(ledger); err != nil {
		t.Fatalf("ASSERT_ADR0011_LEDGER_MALFORMED_PREFIX: %v", err)
	}
	ledger.Members[0].Elements = append(ledger.Members[0].Elements, CandidateElement{Ordinal: 2, Began: true, Terminal: CandidateValidElement})
	if err := CheckCandidateLedger(ledger); err == nil {
		t.Fatal("ASSERT_ADR0011_LEDGER_UNBEGUN_THIRD_REJECTED: accepted")
	}
}
