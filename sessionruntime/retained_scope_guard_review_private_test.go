package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/session"
)

func TestRetainedScopePrivateReviewAliases(t *testing.T) {
	m, s, c, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	uri := members[0].URI
	// Show the decoder mismatch as a local Go observation, not a server claim.
	witness := fmt.Sprintf(`{"item":{"uri":%q,"URI":"file:///external.go"}}`, uri)
	var receiver struct {
		Item struct {
			URI string `json:"uri"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(witness), &receiver); err != nil || receiver.Item.URI != "file:///external.go" {
		t.Fatal("case-insensitive decoder witness failed", err)
	}
	baseline := m.RoundTrip(context.Background(), retainedScopeRequest(s, g, "textDocument/documentSymbol", uri))
	if baseline.Failure != "" {
		t.Fatal(baseline.Failure)
	}
	before, _ := c.snapshot()
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		key, alias := "textDocument", "TextDocument"
		if strings.HasPrefix(method, "callHierarchy/") {
			key, alias = "item", "Item"
		}
		for _, raw := range []string{
			fmt.Sprintf(`{%q:{"uri":%q,"URI":"file:///external.go"}}`, key, uri),
			fmt.Sprintf(`{%q:{"URI":"file:///external.go","uri":%q}}`, key, uri),
			fmt.Sprintf(`{%q:{"uri":%q,"\u0055RI":%q}}`, key, uri, uri),
			fmt.Sprintf(`{%q:{"URI":%q}}`, key, uri),
			fmt.Sprintf(`{%q:{"uri":%q},%q:{"uri":"file:///external.go"}}`, key, uri, alias),
			fmt.Sprintf(`{%q:{"uri":"file:///external.go"},%q:{"uri":%q}}`, alias, key, uri),
			fmt.Sprintf(`{%q:{"uri":%q}}`, alias, uri),
			fmt.Sprintf(`{%q:{"uri":%q,"uri":%q}}`, key, uri, uri),
			fmt.Sprintf(`{%q:{"uri":%q},%q:{"uri":%q}}`, key, uri, key, uri),
		} {
			req := retainedScopeRequest(s, g, method, uri)
			req.Params = json.RawMessage(raw)
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
				t.Errorf("accepted %s %s: failure=%s key=%+v", method, raw, got.Failure, got.Key)
			}
		}
	}
	after, _ := c.snapshot()
	if len(before) != len(after) || len(m.sessions[s.SessionID].requests) != 0 {
		t.Error("alias refusal wrote or retained pending state")
	}
	got := m.RoundTrip(context.Background(), retainedScopeRequest(s, g, "textDocument/documentSymbol", uri))
	if got.Failure != "" || got.Key.ID != baseline.Key.ID+1 {
		t.Error("alias refusals allocated pending IDs", got.Key)
	}
}

func TestRetainedScopePrivateReviewDTOKeys(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	uri := members[0].URI
	cases := []struct{ method, raw string }{
		{"textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q},"Item":{"uri":"file:///external.go"}}`, uri)},
		{"textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q},"WorkDoneToken":1}`, uri)},
		{"textDocument/prepareCallHierarchy", fmt.Sprintf(`{"textDocument":{"uri":%q},"Position":{"line":0,"character":0}}`, uri)},
		{"textDocument/prepareCallHierarchy", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"Line":1,"character":0}}`, uri)},
		{"callHierarchy/incomingCalls", fmt.Sprintf(`{"item":{"uri":%q},"TextDocument":{"uri":"file:///external.go"}}`, uri)},
		{"callHierarchy/incomingCalls", fmt.Sprintf(`{"item":{"uri":%q,"Name":"Target"}}`, uri)},
		{"callHierarchy/outgoingCalls", fmt.Sprintf(`{"item":{"uri":%q,"Data":{"uri":"file:///external.go"}}}`, uri)},
		{"callHierarchy/outgoingCalls", fmt.Sprintf(`{"item":{"uri":%q,"range":{"Start":{"line":0,"character":0},"end":{"line":1,"character":0}}}}`, uri)},
		{"callHierarchy/outgoingCalls", fmt.Sprintf(`{"item":{"uri":%q,"selectionRange":{"start":{"line":0,"character":0,"Character":1},"end":{"line":1,"character":0}}}}`, uri)},
		{"callHierarchy/outgoingCalls", fmt.Sprintf(`{"item":{"uri":%q,"data":{"x":1,"\u0078":2}}}`, uri)},
	}
	for _, tc := range cases {
		req := retainedScopeRequest(s, g, tc.method, uri)
		req.Params = json.RawMessage(tc.raw)
		if got := m.RoundTrip(context.Background(), req); got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
			t.Errorf("DTO alias accepted %s: %s", tc.raw, got.Failure)
		}
	}
	if got := m.RoundTrip(context.Background(), retainedScopeRequest(s, g, "textDocument/documentSymbol", uri)); got.Failure != "" || got.Key.ID != 1 {
		t.Fatal("DTO refusal allocated pending", got.Failure, got.Key)
	}
}

func TestRetainedScopePrivateReviewSupplyAliases(t *testing.T) {
	for _, method := range []string{"textDocument/didOpen", "textDocument/didChange"} {
		for _, where := range []string{"root", "uri", "version", "language-or-text", "change"} {
			t.Run(method+"/"+where, func(t *testing.T) {
				m, s, _, members := retainedScopeFixture(t)
				member := members[5]
				if method == "textDocument/didChange" {
					got := m.RefreshDocumentSupply(context.Background(), DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: member.URI, LanguageID: "go", CaptureSupply: true})
					if got.Failure != "" {
						t.Fatal(got.Failure)
					}
				}
				d := m.sessions[s.SessionID].documents[member.URI]
				var root map[string]json.RawMessage
				if err := json.Unmarshal(d.supply.Params, &root); err != nil {
					t.Fatal(err)
				}
				var doc map[string]json.RawMessage
				if err := json.Unmarshal(root["textDocument"], &doc); err != nil {
					t.Fatal(err)
				}
				switch where {
				case "root":
					root["TextDocument"] = root["textDocument"]
				case "uri":
					doc["URI"] = doc["uri"]
				case "version":
					doc["Version"] = doc["version"]
				case "language-or-text":
					if method == "textDocument/didOpen" {
						doc["LanguageId"] = doc["languageId"]
					} else {
						root["ContentChanges"] = root["contentChanges"]
					}
				case "change":
					if method == "textDocument/didOpen" {
						doc["Text"] = doc["text"]
					} else {
						var changes []map[string]json.RawMessage
						_ = json.Unmarshal(root["contentChanges"], &changes)
						changes[0]["Text"] = changes[0]["text"]
						root["contentChanges"], _ = json.Marshal(changes)
					}
				}
				root["textDocument"], _ = json.Marshal(doc)
				d.supply.Params, _ = json.Marshal(root)
				if guard, failure := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members); failure != DocumentSupplyUnavailable || guard != nil {
					t.Fatalf("supply alias admitted: %s", d.supply.Params)
				}
			})
		}
	}
}

func TestRetainedScopePrivateReviewItemPayloadAndMethods(t *testing.T) {
	m, s, c, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	uri := members[0].URI
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		req := retainedScopeRequest(s, g, method, uri)
		if strings.HasPrefix(method, "callHierarchy/") {
			req.Params = json.RawMessage(fmt.Sprintf(`{"item":{"name":"Target","kind":12,"uri":%q,"range":{"start":{"line":0,"character":0},"end":{"line":1,"character":0}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}},"detail":"retained","tags":[1],"data":{"uri":"opaque","URI":"also opaque","item":1,"Item":2,"nested":{"textDocument":1,"TextDocument":2}}},"workDoneToken":"work","partialResultToken":3}`, uri))
		} else if method == "textDocument/prepareCallHierarchy" {
			req.Params = json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"character":0},"workDoneToken":"work"}`, uri))
		}
		got := m.RoundTrip(context.Background(), req)
		if got.Failure != "" {
			t.Fatalf("valid %s: %s", method, got.Failure)
		}
		messages, _ := c.snapshot()
		sent := messages[len(messages)-1]
		if sent.Method != method || !bytes.Equal(sent.Params, req.Params) {
			t.Fatalf("payload changed: %s", sent.Params)
		}
	}
}

func TestRetainedScopePrivateReviewPreCopyBounds(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	oversized := bytes.Repeat([]byte(" "), (2<<20)+1)
	for _, guard := range []*RetainedScopeGuardPrivate{g, {}} {
		for _, tc := range []struct {
			method string
			params json.RawMessage
		}{
			{"textDocument/documentSymbol", oversized},
			{"textDocument/definition", oversized},
			{"textDocument/definition", oversized[:2<<20]},
			{"textDocument/documentSymbol", nil},
			{"textDocument/definition", nil},
		} {
			req := retainedScopeRequest(s, guard, tc.method, members[0].URI)
			req.Params = tc.params
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			var got RoundTripResult
			for i := 0; i < 4; i++ {
				got = m.RoundTrip(context.Background(), req)
			}
			runtime.ReadMemStats(&after)
			if got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
				t.Fatal("oversize did not refuse before pending", got.Failure)
			}
			// Four illegal Params copies cost >8 MiB; a 1 MiB total allowance is
			// deliberately generous for fixed runtime overhead, including race builds.
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
				t.Errorf("pre-copy guard allocated %d bytes for %s/%d bytes", allocated, tc.method, len(tc.params))
			}
		}
	}
	for _, params := range []json.RawMessage{nil, {}, oversized} {
		req := retainedScopeRequest(s, g, "textDocument/documentSymbol", members[0].URI)
		req.Params = params
		if got := m.RoundTrip(context.Background(), req); got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
			t.Fatal("invalid params accepted", got.Failure)
		}
	}
	// The cap is inclusive. Whitespace is legal JSON and keeps source identity.
	req := retainedScopeRequest(s, g, "textDocument/documentSymbol", members[0].URI)
	req.Params = append(req.Params, bytes.Repeat([]byte(" "), (2<<20)-len(req.Params))...)
	if got := m.RoundTrip(context.Background(), req); got.Failure != "" || got.Key.ID != 1 {
		t.Fatal("exact cap refused or prior refusals allocated pending", got.Failure, got.Key)
	}
}

func TestRetainedScopePrivateReviewNilBindContext(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("nil context panicked: %v", p)
		}
	}()
	g, f := m.BindRetainedScopePrivate(nil, s.SessionID, s.Generation, members)
	if g != nil || f != session.LifecycleConflict || m.sessions[s.SessionID].retainedScopePrivate != nil {
		t.Fatalf("nil context result guard=%p failure=%s", g, f)
	}
}
