package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

func retainedScopeFixture(t *testing.T) (*Manager, StartResult, *roundTripChild, []RetainedScopeMemberPrivate) {
	t.Helper()
	m, s, c, first := ownedDocumentFixture(t, true)
	t.Cleanup(func() { c.Teardown(context.Background()); c.Close() })
	members := make([]RetainedScopeMemberPrivate, 6)
	for i := range members {
		uri := first.URI
		content := []byte("package fixture\nfunc Query() {}\n")
		if i > 0 {
			path := filepath.Join(m.sessions[s.SessionID].record.Profile.Workspace().String(), fmt.Sprintf("scope%d.go", i))
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			uri = (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
			result := m.PrepareDocument(context.Background(), DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
			if result.Failure != "" || result.Supply == nil {
				t.Fatalf("prepare: %+v", result)
			}
		}
		role := RetainedScopeNominationPrivate
		if i >= 4 {
			role = RetainedScopeContextOnlyPrivate
		}
		members[i] = RetainedScopeMemberPrivate{URI: uri, LanguageID: "go", Expected: content, Role: role}
	}
	return m, s, c, members
}
func retainedScopeRequest(s StartResult, g *RetainedScopeGuardPrivate, method, uri string) RoundTripRequest {
	key := "textDocument"
	if method == "callHierarchy/incomingCalls" || method == "callHierarchy/outgoingCalls" {
		key = "item"
	}
	p, _ := json.Marshal(map[string]any{key: map[string]string{"uri": uri}})
	return RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, RetainedScopePrivate: g, Method: method, Params: p, MaxMessages: 1, MaxBytes: 2 << 20, Deadline: time.Now().Add(time.Second)}
}
func TestRetainedScopePrivateEqualAndCachedSupply(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	cached := m.PrepareDocument(context.Background(), DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: members[0].URI, LanguageID: "go", CaptureSupply: true})
	if cached.Supply == nil {
		t.Fatal("cached capture unavailable")
	}
	cached.Supply.Content[0] = 'X'
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" || g == nil {
		t.Fatal(f)
	}
	members[0].Expected[0] = 'X'
	copyOut := g.MembersPrivate()
	copyOut[0].Expected[0] = 'Y'
	copyOut[0].URI = "bad"
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		req := retainedScopeRequest(s, g, method, g.MembersPrivate()[0].URI)
		if got := m.RoundTrip(context.Background(), req); got.Failure != "" || got.Key.ID == 0 {
			t.Fatalf("%s: %+v", method, got)
		}
	}
	if _, f = m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, g.MembersPrivate()); f == "" {
		t.Fatal("rebound active guard")
	}
}
func TestRetainedScopePrivateBindRefusals(t *testing.T) {
	for _, kind := range []string{"nil-supply", "bytes", "language", "role", "duplicate", "alias", "count", "large", "total", "generation", "cancelled", "preparing", "owned", "closed", "poisoned", "supply-session", "supply-generation", "supply-uri", "supply-version", "supply-method", "supply-class", "supply-params", "language-mismatch", "unknown-role"} {
		t.Run(kind, func(t *testing.T) {
			m, s, _, members := retainedScopeFixture(t)
			ctx := context.Background()
			r := m.sessions[s.SessionID]
			switch kind {
			case "nil-supply":
				d := r.documents[members[5].URI]
				d.supply = nil
				r.documents[members[5].URI] = d
			case "bytes":
				members[5].Expected = []byte("different")
			case "language":
				members[5].LanguageID = ""
			case "role":
				members[5].Role = RetainedScopeNominationPrivate
			case "duplicate":
				members[5].URI = members[0].URI
			case "alias":
				members[5].URI += "?alias"
			case "count":
				members = members[:5]
			case "large":
				members[0].Expected = make([]byte, (2<<20)+1)
			case "total":
				for i := range members {
					members[i].Expected = make([]byte, 2<<20)
				}
			case "generation":
				s.Generation++
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "preparing":
				r.retainedScopePreparing = 1
			case "owned":
				r.protocolOwned = true
			case "closed":
				m.closed = true
			case "poisoned":
				r.record.State = session.Poisoned
			case "supply-session":
				r.documents[members[5].URI].supply.SessionID = "foreign"
			case "supply-generation":
				r.documents[members[5].URI].supply.Generation++
			case "supply-uri":
				r.documents[members[5].URI].supply.URI = members[0].URI
			case "supply-version":
				r.documents[members[5].URI].supply.DocumentVersion++
			case "supply-method":
				r.documents[members[5].URI].supply.Method = "textDocument/definition"
			case "supply-class":
				r.documents[members[5].URI].supply.Classification = "CALLER"
			case "supply-params":
				r.documents[members[5].URI].supply.Params = json.RawMessage(`{}`)
			case "language-mismatch":
				members[5].LanguageID = "csharp"
			case "unknown-role":
				members[5].Role = "OTHER"
			}
			g, f := m.BindRetainedScopePrivate(ctx, s.SessionID, s.Generation, members)
			if f == "" || g != nil || r.retainedScopePrivate != nil || len(r.requests) != 0 {
				t.Fatalf("accepted %s", kind)
			}
		})
	}
}
func TestRetainedScopePrivateRequestRefusals(t *testing.T) {
	for _, kind := range []string{"absent", "zero", "copied", "foreign-manager", "foreign-session", "stale-generation", "version", "language", "supply", "digest", "content", "external", "duplicate", "nested-duplicate", "wrong-container", "unsupported", "cancelled", "lease-retired", "poisoned"} {
		t.Run(kind, func(t *testing.T) {
			m, s, c, members := retainedScopeFixture(t)
			g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
			if f != "" {
				t.Fatal(f)
			}
			req := retainedScopeRequest(s, g, "callHierarchy/incomingCalls", members[0].URI)
			ctx := context.Background()
			r := m.sessions[s.SessionID]
			d := r.documents[members[5].URI]
			switch kind {
			case "absent":
				req.RetainedScopePrivate = nil
			case "zero":
				req.RetainedScopePrivate = &RetainedScopeGuardPrivate{}
			case "copied":
				cp := *g
				req.RetainedScopePrivate = &cp
			case "foreign-manager":
				other, otherSession, _, otherMembers := retainedScopeFixture(t)
				if _, failure := other.BindRetainedScopePrivate(ctx, otherSession.SessionID, otherSession.Generation, otherMembers); failure != "" {
					t.Fatal(failure)
				}
				m = other
				req.SessionID, req.Generation = otherSession.SessionID, otherSession.Generation
			case "foreign-session":
				req.SessionID = "missing"
			case "stale-generation":
				req.Generation++
			case "version":
				// A coherent new full-text supply with identical bytes still retires
				// the bound version; digest equality alone must not admit it.
				d.version++
				d.supply.DocumentVersion = d.version
				d.supply.Method = "textDocument/didChange"
				d.supply.Params, _ = json.Marshal(map[string]any{"textDocument": map[string]any{"uri": members[5].URI, "version": d.version}, "contentChanges": []map[string]string{{"text": string(d.supply.Content)}}})
				if !validRetainedScopeDocumentPrivate(d, members[5], s.SessionID, s.Generation) {
					t.Fatal("incoherent same-byte version fixture")
				}
				r.documents[members[5].URI] = d
			case "language":
				d.languageID = "other"
				r.documents[members[5].URI] = d
			case "supply":
				d.supply = nil
				r.documents[members[5].URI] = d
			case "digest":
				d.digest[0] ^= 1
				r.documents[members[5].URI] = d
			case "content":
				d.supply.Content[0] ^= 1
			case "external":
				req = retainedScopeRequest(s, g, req.Method, "file:///external.go")
			case "duplicate":
				req.Params = json.RawMessage(`{"item":{"uri":"bad","uri":"bad"}}`)
			case "nested-duplicate":
				req.Params = json.RawMessage(fmt.Sprintf(`{"item":{"uri":%q,"data":{"a":1,"\u0061":2}}}`, members[0].URI))
			case "wrong-container":
				req.Params = json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, members[0].URI))
			case "unsupported":
				req.Method = "textDocument/definition"
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "lease-retired":
				r.leaseNonce++
			case "poisoned":
				r.record.State = session.Poisoned
			}
			before, _ := c.snapshot()
			got := m.RoundTrip(ctx, req)
			after, _ := c.snapshot()
			if got.Failure == "" || got.Key.ID != 0 || len(r.requests) != 0 || len(after) != len(before) {
				t.Fatalf("not pre-effect: %+v", got)
			}
		})
	}
}
func TestRetainedScopePrivatePreparationFrozen(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	// A missing checkout must not change the refusal classification: no source read.
	u, _ := url.Parse(members[0].URI)
	if err := os.Remove(u.Path); err != nil {
		t.Fatal(err)
	}
	for _, refresh := range []bool{false, true} {
		req := DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: members[0].URI, LanguageID: "go", CaptureSupply: true}
		var result DocumentResult
		if refresh {
			result = m.RefreshDocumentSupply(context.Background(), req)
		} else {
			result = m.PrepareDocument(context.Background(), req)
		}
		if result.Failure != DocumentSupplyUnavailable || result.DiagnosticOperation != (DiagnosticOperationHandle{}) {
			t.Fatalf("read checkout: %+v", result)
		}
	}
	if !bytes.Equal(g.MembersPrivate()[0].Expected, members[0].Expected) {
		t.Fatal("binding changed")
	}
}

func TestRetainedScopePrivateRoutesRefuseBeforePending(t *testing.T) {
	m, s, c, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	before, _ := c.snapshot()
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		req := retainedScopeRequest(s, nil, method, members[0].URI)
		for _, directional := range []bool{false, true} {
			var got RoundTripResult
			if directional {
				req.directionalV5 = &privateDirectionalCallV5{}
				got = m.roundTripWithPrivateTransaction(context.Background(), req, nil, &privateB4Transaction{directionalV5: req.directionalV5})
			} else {
				got = m.RoundTrip(context.Background(), req)
			}
			if got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
				t.Fatalf("route %s/%v: %+v", method, directional, got)
			}
		}
	}
	after, _ := c.snapshot()
	if len(before) != len(after) {
		t.Fatal("unguarded write")
	}
	got := m.RoundTrip(context.Background(), retainedScopeRequest(s, g, "textDocument/documentSymbol", members[0].URI))
	if got.Failure != "" || got.Key.ID != 1 {
		t.Fatalf("refusals allocated pending IDs: %+v", got)
	}
}

func TestRetainedScopePrivatePreparingExcludesBinding(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	m.documentFinalHook = func() { close(entered); <-release }
	done := make(chan DocumentResult, 1)
	go func() {
		done <- m.PrepareDocument(context.Background(), DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: members[0].URI, LanguageID: "go", CaptureSupply: true})
	}()
	<-entered
	_, failure := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	close(release)
	if result := <-done; result.Failure != "" {
		t.Fatal(result.Failure)
	}
	m.documentFinalHook = nil
	if failure != DocumentSupplyUnavailable {
		t.Fatal("binding raced preparation", failure)
	}
	if _, failure = m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members); failure != "" {
		t.Fatal(failure)
	}
	m.documentFinalHook = func() { t.Error("post-bind preparation reached filesystem-final hook") }
	for _, capture := range []bool{false, true} {
		if got := m.PrepareDocument(context.Background(), DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: members[0].URI, LanguageID: "go", CaptureSupply: capture}); got.Failure != DocumentSupplyUnavailable {
			t.Fatal(got.Failure)
		}
	}
}

func TestRetainedScopePrivateLeaseLifetime(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	var guard *RetainedScopeGuardPrivate
	err := m.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: m.sessions[s.SessionID].record.Profile.Workspace().String()}, func(ctx context.Context) error {
		var failure session.Failure
		guard, failure = m.BindRetainedScopePrivate(ctx, s.SessionID, s.Generation, members)
		if failure != "" {
			t.Fatal(failure)
		}
		if got := m.RoundTrip(context.Background(), retainedScopeRequest(s, guard, "textDocument/documentSymbol", members[0].URI)); got.Failure != session.LifecycleConflict {
			t.Fatal(got.Failure)
		}
		if got := m.RoundTrip(ctx, retainedScopeRequest(s, guard, "textDocument/documentSymbol", members[0].URI)); got.Failure != "" {
			t.Fatal(got.Failure)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.RoundTrip(context.Background(), retainedScopeRequest(s, guard, "textDocument/documentSymbol", members[0].URI)); got.Failure != DocumentSupplyUnavailable {
		t.Fatal("retired lease accepted", got.Failure)
	}
}

func TestRetainedScopePrivateRefreshBeforeBindAndWireIsolation(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	doc := m.RefreshDocumentSupply(context.Background(), DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: members[5].URI, LanguageID: "go", CaptureSupply: true})
	if doc.Failure != "" || doc.Supply == nil || doc.Supply.Method != "textDocument/didChange" {
		t.Fatal(doc)
	}
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	req := retainedScopeRequest(s, g, "textDocument/documentSymbol", members[5].URI)
	field, ok := reflect.TypeOf(req).FieldByName("RetainedScopePrivate")
	if !ok || field.Tag.Get("json") != "-" {
		t.Fatal("guard must not serialize")
	}
	if got := m.RoundTrip(context.Background(), req); got.Failure != "" {
		t.Fatal(got.Failure)
	}
}

func TestRetainedScopePrivateExplicitGenerationRetirement(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	// Exercise only the pre-existing explicit lifecycle path with non-wire fakes.
	// The guard itself must never request a restart or recover an invocation.
	m.sessions[s.SessionID].process = referenceChild{}
	m.starter = &referenceStarter{}
	accepted := m.Restart(context.Background(), s.SessionID, "scope-test-explicit-restart")
	if accepted.Failure != "" || accepted.IntentID == "" {
		t.Fatal(accepted)
	}
	waitOperation(t, m, accepted.IntentID, OperationComplete)
	r := m.sessions[s.SessionID]
	if r.record.Generation == s.Generation || r.retainedScopePrivate != nil || len(r.documents) != 0 {
		t.Fatal("old generation scope survived explicit retirement")
	}
	s.Generation = r.record.Generation
	if got := m.ObserveInitialization(s.SessionID, s.Generation, true); got.Failure != "" {
		t.Fatal(got.Failure)
	}
	if got := m.RoundTrip(context.Background(), retainedScopeRequest(s, g, "textDocument/documentSymbol", members[0].URI)); got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
		t.Fatal("old guard reached successor", got.Failure)
	}
}

func TestRetainedScopePrivateTransportCancellationPreservesPoison(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	c := newRoundTripChild("hang")
	c.requestSeen = make(chan struct{}, 1)
	t.Cleanup(func() { c.Teardown(context.Background()); c.Close() })
	m.sessions[s.SessionID].process = c
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan RoundTripResult, 1)
	go func() {
		done <- m.RoundTrip(ctx, retainedScopeRequest(s, g, "textDocument/documentSymbol", members[0].URI))
	}()
	select {
	case <-c.requestSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach transport")
	}
	cancel()
	select {
	case got := <-done:
		if got.Failure != session.RequestCancelled {
			t.Fatal(got.Failure)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not join")
	}
	if m.sessions[s.SessionID].record.State != session.Poisoned {
		t.Fatal("lost terminal poison")
	}
	if _, failure := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members); failure != session.LifecycleConflict {
		t.Fatal("poison recovered", failure)
	}
	before, _ := c.snapshot()
	if got := m.RoundTrip(context.Background(), retainedScopeRequest(s, g, "textDocument/documentSymbol", members[0].URI)); got.Failure != session.LifecycleConflict {
		t.Fatal(got.Failure)
	}
	after, _ := c.snapshot()
	if len(before) != len(after) {
		t.Fatal("poison caused new write")
	}
}

func TestRetainedScopePrivateURIAndParams(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	for _, uri := range []string{members[0].URI + "#fragment", members[0].URI + "?", strings.Replace(members[0].URI, "file:///", "file:/", 1), "file://localhost/a.go", "file:///a/../b.go", "file:///a%2Fb.go", "file:///a%00.go"} {
		if canonicalRetainedScopeURIPrivate(uri) {
			t.Fatalf("canonical alias: %s", uri)
		}
	}
	for _, raw := range []string{`null`, `[]`, `{}`, `{"item":null}`, `{"item":{"URI":"bad"}}`, `{"item":{"uri":123}}`, `{"item":{"uri":"file:///a"},"item":{}}`, `{"item":{}} {}`, `{"item":`} {
		req := retainedScopeRequest(s, g, "callHierarchy/outgoingCalls", members[0].URI)
		req.Params = json.RawMessage(raw)
		if got := m.RoundTrip(context.Background(), req); got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
			t.Fatalf("params %s: %+v", raw, got)
		}
	}
}
