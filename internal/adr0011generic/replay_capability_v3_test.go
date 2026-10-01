package adr0011generic

import (
	"strings"
	"testing"
)

func b4Event(ord uint64, raw string) capabilityOriginalV3 {
	return capabilityOriginalV3{Session: "s", Generation: 1, Transaction: "t", Ordinal: ord, Raw: []byte(raw)}
}
func b4Input(cap string) capabilityReplayInputV3 {
	raw := []byte(`{"capabilities":` + cap + `}`)
	original := capabilityClientSelectorOriginalV3{Session: "s", Generation: 1, Transaction: "t", Raw: raw}
	return capabilityReplayInputV3{HeldInitialize: original, ClaimantInitialize: original, Query: capabilityQueryV3{Session: "s", Generation: 1, Transaction: "t", WriteOrdinal: 10, Method: "textDocument/references", URI: "file:///a", Language: b4String("go")}}
}
func b4String(s string) *string { return &s }
func b4Register(selector string) string {
	return `{"registrations":[{"id":"r","method":"textDocument/references","registerOptions":{"documentSelector":` + selector + `}}]}`
}
func TestB4InitializeCrossTransaction(t *testing.T) {
	in := b4Input(`{"referencesProvider":true}`)
	in.HeldInitialize.Transaction = "other"
	if r := replayCapabilityV3(in); r.Outcome != capabilityInvalidChronologyV3 || r.capabilityGuardPassed() || r.OwnError == "" {
		t.Fatalf("cross-transaction initialize: %+v", r)
	}
}
func TestB4DefinitionParity(t *testing.T) {
	for _, tc := range []struct {
		name, cap, event string
		want             capabilityOutcomeV3
	}{
		{"static", `{"definitionProvider":true}`, "", capabilitySupportedV3},
		{"dynamic", `{}`, `{"registrations":[{"id":"d","method":"textDocument/definition","registerOptions":{"documentSelector":[{"language":"go"}]}}]}`, capabilitySupportedV3},
		{"otherMethod", `{}`, b4Register(`[{"language":"go"}]`), capabilityUnsupportedV3},
		{"staticDynamic", `{"definitionProvider":true}`, `{"registrations":[{"id":"d","method":"textDocument/definition","registerOptions":{"documentSelector":[{"language":"go"}]}}]}`, capabilityInvalidChronologyV3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4Input(tc.cap)
			in.Query.Method = "textDocument/definition"
			if tc.event != "" {
				in.Held = []capabilityOriginalV3{b4Event(1, tc.event)}
				in.Claimant = in.Held
			}
			r := replayCapabilityV3(in)
			if r.Outcome != tc.want || r.capabilityGuardPassed() != (tc.want == capabilitySupportedV3) {
				t.Fatalf("definition %s: %+v", tc.name, r)
			}
		})
	}
}
func TestB4InitializeMustBeFullResult(t *testing.T) {
	in := b4Input(`{"referencesProvider":true}`)
	in.HeldInitialize.Raw = []byte(`{"referencesProvider":true}`)
	in.ClaimantInitialize.Raw = in.HeldInitialize.Raw
	if r := replayCapabilityV3(in); r.Outcome != capabilityMalformedV3 || r.capabilityGuardPassed() {
		t.Fatalf("bare capabilities admitted as initialize result: %+v", r)
	}
}
func TestB4StaticAndCustody(t *testing.T) {
	for _, tc := range []struct {
		name, cap string
		want      capabilityOutcomeV3
	}{
		{"absent", `{}`, capabilityUnsupportedV3}, {"null", `{"referencesProvider":null}`, capabilityUnsupportedV3}, {"nullWhitespace", `{"referencesProvider": null }`, capabilityUnsupportedV3}, {"false", `{"referencesProvider":false}`, capabilityUnsupportedV3}, {"falseWhitespace", `{"referencesProvider": false }`, capabilityUnsupportedV3}, {"true", `{"referencesProvider":true}`, capabilitySupportedV3}, {"options", `{"referencesProvider":{}}`, capabilitySupportedV3}, {"badType", `{"referencesProvider":4}`, capabilityMalformedV3}, {"badKnown", `{"referencesProvider":{"workDoneProgress":1}}`, capabilityMalformedV3}, {"duplicate", `{"referencesProvider":{"workDoneProgress":true,"workDoneProgress":false}}`, capabilityMalformedV3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4Input(tc.cap)
			if got := replayCapabilityV3(in).Outcome; got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
	in := b4Input(`{"referencesProvider":true}`)
	in.ClaimantInitialize.Raw = []byte(`{"capabilities":{"referencesProvider":false}}`)
	if r := replayCapabilityV3(in); r.Outcome != capabilitySupportedV3 || r.OriginalsEqual || r.capabilityGuardPassed() {
		t.Fatal("claimant mismatch passed exact capability guard", r)
	}
}
func TestB4DynamicSelectors(t *testing.T) {
	for _, tc := range []struct {
		name, options string
		want          capabilityOutcomeV3
	}{
		{"missingOptions", ``, capabilityUnknownV3}, {"badOptions", `,"registerOptions":false`, capabilityMalformedV3}, {"missingSelector", `,"registerOptions":{}`, capabilityUnknownV3}, {"empty", `,"registerOptions":{"documentSelector":[]}`, capabilityMalformedV3}, {"language", `,"registerOptions":{"documentSelector":[{"language":"go"}]}`, capabilitySupportedV3}, {"case", `,"registerOptions":{"documentSelector":[{"language":"Go"}]}`, capabilityUnsupportedV3}, {"unknownOnly", `,"registerOptions":{"documentSelector":[{"future":1}]}`, capabilityUnknownV3}, {"extension", `,"registerOptions":{"documentSelector":[{"language":"go","vendor":1}]}`, capabilitySupportedV3}, {"wrongKnown", `,"registerOptions":{"documentSelector":[{"language":4}]}`, capabilityMalformedV3}, {"pattern", `,"registerOptions":{"documentSelector":[{"language":"go","pattern":"**/*.go"}]}`, capabilityUnknownV3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4Input(`{}`)
			in.Held = []capabilityOriginalV3{b4Event(1, `{"registrations":[{"id":"r","method":"textDocument/references"`+tc.options+`}]}`)}
			in.Claimant = in.Held
			if got := replayCapabilityV3(in).Outcome; got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, selector string
		held           string
		want           capabilityOutcomeV3
	}{
		{"schemeCase", `[{"scheme":"file"}]`, `FILE:///x`, capabilitySupportedV3}, {"invalidScheme", `[{"scheme":"file"}]`, `1file:///x`, capabilityUnknownV3}, {"notebookStar", `[{"notebook":"*","language":"*"}]`, `file:///cell`, capabilitySupportedV3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4Input(`{}`)
			in.Query.URI = tc.held
			if tc.name == "notebookStar" {
				in.Query.Cell = true
				in.Query.NotebookURI = b4String("file:///book")
			}
			in.Held = []capabilityOriginalV3{b4Event(1, b4Register(tc.selector))}
			in.Claimant = in.Held
			if got := replayCapabilityV3(in).Outcome; got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
}
func TestB4HeldSelectorAndFilterLogic(t *testing.T) {
	for _, tc := range []struct {
		name, selector string
		want           capabilityOutcomeV3
	}{
		{"andFalsePattern", `[{"language":"Go","pattern":"*.go"}]`, capabilityUnsupportedV3},
		{"orTrueUnknown", `[{"language":"go"},{"pattern":"*.go"}]`, capabilitySupportedV3},
		{"malformedOtherElement", `[{"language":"go"},{"language":4}]`, capabilityMalformedV3},
		{"schemeASCII", `[{"scheme":"git+ssh"}]`, capabilitySupportedV3},
		{"notebookMissingParent", `[{"notebook":"*"}]`, capabilityUnknownV3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4Input(`{}`)
			in.Query.URI = "Git+SSH:///a"
			in.Query.Cell = true
			in.Held = []capabilityOriginalV3{b4Event(1, b4Register(tc.selector))}
			in.Claimant = in.Held
			if got := replayCapabilityV3(in).Outcome; got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
	for _, held := range []string{`[{"language":"go"}]`, `null`} {
		in := b4Input(`{}`)
		in.HeldClientSelector = &capabilityClientSelectorOriginalV3{Session: "s", Generation: 1, Transaction: "t", Raw: []byte(held)}
		in.ClaimantClientSelector = in.HeldClientSelector
		in.Held = []capabilityOriginalV3{b4Event(1, b4Register(`null`))}
		in.Claimant = in.Held
		want := capabilitySupportedV3
		if held == "null" {
			want = capabilityUnknownV3
		}
		if got := replayCapabilityV3(in).Outcome; got != want {
			t.Fatalf("held selector %s: %s", held, got)
		}
	}
	in := b4Input(`{}`)
	in.Held = []capabilityOriginalV3{b4Event(1, `{"registrations":[{"id":"other","method":"textDocument/definition","registerOptions":{"documentSelector":[{"language":"go"}]}}]}`)}
	in.Claimant = in.Held
	if got := replayCapabilityV3(in).Outcome; got != capabilityUnsupportedV3 {
		t.Fatalf("cross-method: %s", got)
	}
}
func TestB4PostWriteCannotInvalidatePriorSupport(t *testing.T) {
	in := b4Input(`{}`)
	in.Held = []capabilityOriginalV3{b4Event(1, b4Register(`[{"language":"go"}]`)), {Session: "other", Generation: 1, Transaction: "other", Ordinal: 11, Raw: []byte(`not json`)}}
	in.Claimant = in.Held
	if r := replayCapabilityV3(in); r.Outcome != capabilitySupportedV3 || !r.capabilityGuardPassed() {
		t.Fatalf("post-WRITE original changed earlier capability: %+v", r)
	}
}
func TestB4InvalidSelectorSchemeMalformed(t *testing.T) {
	in := b4Input(`{}`)
	in.Held = []capabilityOriginalV3{b4Event(1, b4Register(`[{"scheme":"file!"}]`))}
	in.Claimant = in.Held
	if r := replayCapabilityV3(in); r.Outcome != capabilityMalformedV3 || r.capabilityGuardPassed() {
		t.Fatalf("invalid known scheme: %+v", r)
	}
}
func TestB4NullRequiresHeldClientOriginal(t *testing.T) {
	in := b4Input(`{}`)
	in.Held = []capabilityOriginalV3{b4Event(1, b4Register(`null`))}
	in.Claimant = in.Held
	in.ClaimantClientSelector = &capabilityClientSelectorOriginalV3{Session: "s", Generation: 1, Transaction: "t", Raw: []byte(`[{"language":"go"}]`)}
	if r := replayCapabilityV3(in); r.Outcome != capabilityUnknownV3 || r.capabilityGuardPassed() {
		t.Fatalf("claimant-only client selector passed: %+v", r)
	}
	in.HeldClientSelector = &capabilityClientSelectorOriginalV3{Session: "s", Generation: 1, Transaction: "t", Raw: []byte(`[{"language":"go"}]`)}
	if r := replayCapabilityV3(in); !r.capabilityGuardPassed() {
		t.Fatalf("held matching client selector rejected: %+v", r)
	}
	in.HeldClientSelector.Raw = []byte(`[{"language":"Go"}]`)
	if r := replayCapabilityV3(in); r.capabilityGuardPassed() || r.OriginalsEqual {
		t.Fatalf("altered held client selector accepted: %+v", r)
	}
	in.HeldClientSelector.Raw = []byte(`[{"language":"go"}]`)
	in.HeldClientSelector.Transaction = "other"
	if r := replayCapabilityV3(in); r.capabilityGuardPassed() {
		t.Fatalf("cross-transaction client selector accepted: %+v", r)
	}
}
func TestB4ChronologyAndExactOriginals(t *testing.T) {
	reg := b4Event(1, b4Register(`[{"language":"go"}]`))
	unreg := b4Event(2, `{"unregistrations":[{"id":"r","method":"textDocument/references"}]}`)
	for _, tc := range []struct {
		name   string
		cap    string
		events []capabilityOriginalV3
		want   capabilityOutcomeV3
	}{
		{"active", `{}`, []capabilityOriginalV3{reg}, capabilitySupportedV3}, {"staticCollision", `{"referencesProvider":true}`, []capabilityOriginalV3{reg}, capabilityInvalidChronologyV3}, {"staticLaterUnregistered", `{"referencesProvider":true}`, []capabilityOriginalV3{reg, unreg}, capabilityInvalidChronologyV3}, {"exactUnregister", `{}`, []capabilityOriginalV3{reg, unreg}, capabilityUnsupportedV3}, {"wrongID", `{}`, []capabilityOriginalV3{reg, b4Event(2, strings.ReplaceAll(string(unreg.Raw), `"r"`, `"wrong"`))}, capabilityInvalidChronologyV3}, {"wrongMethod", `{}`, []capabilityOriginalV3{reg, b4Event(2, strings.ReplaceAll(string(unreg.Raw), `textDocument/references`, `textDocument/definition`))}, capabilityInvalidChronologyV3}, {"late", `{}`, []capabilityOriginalV3{b4Event(11, string(reg.Raw))}, capabilityUnsupportedV3}, {"duplicateEventKey", `{}`, []capabilityOriginalV3{b4Event(1, `{"registrations":[],"registrations":[]}`)}, capabilityMalformedV3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4Input(tc.cap)
			in.Held = tc.events
			in.Claimant = tc.events
			if got := replayCapabilityV3(in).Outcome; got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
	in := b4Input(`{}`)
	in.Held = []capabilityOriginalV3{reg, unreg}
	in.Claimant = []capabilityOriginalV3{unreg, reg}
	if replayCapabilityV3(in).OriginalsEqual {
		t.Fatal("reorder accepted")
	}
	in.Claimant = in.Held[:1]
	if replayCapabilityV3(in).OriginalsEqual {
		t.Fatal("missing accepted")
	}
	in.Claimant = append(append([]capabilityOriginalV3{}, in.Held...), reg)
	if replayCapabilityV3(in).OriginalsEqual {
		t.Fatal("extra accepted")
	}
	in.Claimant = append([]capabilityOriginalV3{}, in.Held...)
	in.Held[0].Raw = []byte(b4Register(`[{"language":"Go"}]`))
	if replayCapabilityV3(in).OriginalsEqual {
		t.Fatal("altered held accepted")
	}
}
