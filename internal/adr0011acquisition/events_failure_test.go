package adr0011acquisition

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/publication"
)

func TestPrivateEventsCommitStages(t *testing.T) {
	for _, stage := range []string{"precommit", "postcommit"} {
		t.Run(stage, func(t *testing.T) {
			root, x := responseReadFixture(t, "[]")
			response, e := publishResponseRead(root, x, nil)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := publishRawResult(root, response, x.Payload, x, nil)
			if e != nil {
				t.Fatal(e)
			}
			scanner, e := observeRawScanner(root, x.Payload, nil)
			if e != nil {
				t.Fatal(e)
			}
			expected, ok := responseReadExpected(root, x, publication.ReadVerifiedBoundFile)
			if !ok {
				t.Fatal("response")
			}
			identity := adr0011methodresult.PrivateTransitionIdentity{Transaction: expected.TransactionID, RequestKey: expected.RequestKey, Invocation: expected.InvocationID, ResponseRead: response.selector, RawSelector: raw.selector, RawDigest: privateDigest([]byte("[]"))}
			evaluation, journal, e := adr0011methodresult.RunPrivateAttachedReferences(root, x.Pair.Key, json.RawMessage("[]"), identity)
			if e != nil {
				t.Fatal(e)
			}
			record, ok := eventsExpected(root, response, raw, scanner, x, expected.TransactionID, []byte("[]"), journal, nil)
			if !ok {
				t.Fatal("events expected")
			}
			encoded, e := canonicalEvents(record)
			if e != nil {
				t.Fatal(e)
			}
			selector := eventsSelector(eventsDigest(encoded))
			if stage == "precommit" {
				changed := scanner
				changed.knownE++
				state, err := publishEvents(root, response, raw, changed, evaluation, x, expected.TransactionID, []byte("[]"), journal, nil, nil)
				if err == nil || state.stage != "ABSENT" {
					t.Fatalf("ASSERT_EVENTS_PRECOMMIT: %+v %v", state, err)
				}
				return
			}
			fired := false
			trace := func(event publication.BoundFileTraceEvent) {
				if fired {
					return
				}
				if stage == "postcommit" && event.Stage == "HARDLINK" && event.Result == "INSTALLED" {
					fired = true
					_ = os.Remove(filepath.Join(root.Path(), selector))
				}
			}
			state, e := publishEvents(root, response, raw, scanner, evaluation, x, expected.TransactionID, []byte("[]"), journal, nil, trace)
			if !fired || e == nil {
				t.Fatalf("ASSERT_EVENTS_STAGE_INJECTED: %+v %v", state, e)
			}
			want := "COMMITTED_UNVERIFIED"
			if state.stage != want {
				t.Fatalf("ASSERT_EVENTS_STAGE_%s: %+v", want, state)
			}
		})
	}
}
