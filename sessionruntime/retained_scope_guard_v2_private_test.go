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
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/session"
)

// Build independent members using the lower owned-document fake child. The
// extra baseline document is intentionally outside the bound inventory.
func retainedScopeV2Fixture(t *testing.T, n, c int) (*Manager, StartResult, *roundTripChild, []RetainedScopeMemberPrivate) {
	t.Helper()
	m, s, child, _ := ownedDocumentFixture(t, true)
	t.Cleanup(func() { child.Teardown(context.Background()); child.Close() })
	members := make([]RetainedScopeMemberPrivate, n+c)
	for i := range members {
		content := []byte(fmt.Sprintf("package fixture\nfunc Member%d() {}\n", i))
		path := filepath.Join(m.sessions[s.SessionID].record.Profile.Workspace().String(), fmt.Sprintf("v2-member-%d.go", i))
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
		uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
		got := m.PrepareDocument(context.Background(), DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
		if got.Failure != "" || got.Supply == nil {
			t.Fatalf("prepare: %+v", got)
		}
		role := RetainedScopeNominationPrivate
		if i >= n {
			role = RetainedScopeContextOnlyPrivate
		}
		members[i] = RetainedScopeMemberPrivate{URI: uri, LanguageID: "go", Expected: content, Role: role}
	}
	return m, s, child, members
}
func retainedScopeV2WireRequests(child *roundTripChild) int {
	messages, _ := child.snapshot()
	n := 0
	for _, message := range messages {
		if len(message.ID) > 0 {
			n++
		}
	}
	return n
}

func retainedScopeV2Request(s StartResult, g *RetainedScopeGuardPrivateV2, method, uri string) RoundTripRequest {
	req := retainedScopeRequest(s, nil, method, uri)
	req.RetainedScopePrivateV2 = g
	return req
}
func bindScopeV2(t *testing.T, m *Manager, s StartResult, members []RetainedScopeMemberPrivate) *RetainedScopeGuardPrivateV2 {
	t.Helper()
	g, f := m.BindRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, members)
	if f != "" || g == nil {
		t.Fatalf("bind V2: %s", f)
	}
	return g
}
func TestRetainedScopeV2Variety(t *testing.T) {
	for _, counts := range [][2]int{{1, 0}, {2, 1}, {5, 2}, {8, 8}} {
		t.Run(fmt.Sprint(counts), func(t *testing.T) {
			m, s, _, members := retainedScopeV2Fixture(t, counts[0], counts[1])
			g := bindScopeV2(t, m, s, members)
			members[0].Expected[0] = 'X'
			detached := g.MembersPrivate()
			detached[0].Expected[0] = 'Y'
			detached[0].URI = "bad"
			if bytes.Equal(g.MembersPrivate()[0].Expected, detached[0].Expected) {
				t.Fatal("expectations not detached")
			}
			for _, method := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
				for _, member := range g.MembersPrivate() {
					req := retainedScopeV2Request(s, g, method, member.URI)
					if got := m.RoundTrip(context.Background(), req); got.Failure != "" || got.Key.ID == 0 {
						t.Fatalf("%s: %+v", method, got)
					}
				}
			}
			if f := m.CheckRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, g); f != "" {
				t.Fatal(f)
			}
			events, _ := m.RetainedScopeDiagnosticsPrivateV2(g)
			if len(events) == 0 {
				t.Fatal("no diagnostics")
			}
			events[0].Reason = "mutated"
			again, _ := m.RetainedScopeDiagnosticsPrivateV2(g)
			if again[0].Reason == "mutated" {
				t.Fatal("diagnostics not detached")
			}
		})
	}
}
func TestRetainedScopeV2BindRefusals(t *testing.T) {
	for _, kind := range []string{"zero", "nom0", "nom9", "context9", "total17", "role", "duplicate", "alias", "path-alias", "large", "total", "uri", "language-large", "language-invalid", "text-invalid", "unsigned", "changed", "version", "language", "supply-session", "supply-generation", "supply-alias", "supply-duplicate", "preparing", "owned", "closed", "nil-context", "cancelled", "generation", "session"} {
		t.Run(kind, func(t *testing.T) {
			n, c := 2, 1
			switch kind {
			case "nom0":
				n, c = 0, 1
			case "nom9":
				n, c = 9, 0
			case "context9":
				n, c = 1, 9
			case "total17":
				n, c = 8, 9
			}
			m, s, _, members := retainedScopeV2Fixture(t, n, c)
			r := m.sessions[s.SessionID]
			ctx := context.Background()
			last := len(members) - 1
			switch kind {
			case "zero":
				members = nil
			case "role":
				members[last].Role = "UNKNOWN"
			case "duplicate":
				members[last].URI = members[0].URI
			case "alias":
				members[last].URI += "?alias"
			case "path-alias":
				members[last].URI = strings.Replace(members[last].URI, "/v2-member-", "/./v2-member-", 1)
			case "large":
				members[0].Expected = make([]byte, (2<<20)+1)
			case "total":
				members = append(members, members[0], members[0])
				for i := 0; i < 4; i++ {
					members[i].Expected = make([]byte, 2<<20)
				}
				members[4].Expected = []byte("x")
			case "uri":
				members[0].URI = strings.Repeat("x", 8193)
			case "language-large":
				members[0].LanguageID = strings.Repeat("g", 257)
			case "language-invalid":
				members[0].LanguageID = string([]byte{255})
			case "text-invalid":
				members[0].Expected = []byte{255}
			case "unsigned":
				d := r.documents[members[last].URI]
				d.supply = nil
				r.documents[members[last].URI] = d
			case "changed":
				members[last].Expected = []byte("changed")
			case "version":
				d := r.documents[members[last].URI]
				d.version++
				r.documents[members[last].URI] = d
			case "language":
				members[last].LanguageID = "csharp"
			case "supply-session":
				r.documents[members[last].URI].supply.SessionID = "foreign"
			case "supply-generation":
				r.documents[members[last].URI].supply.Generation++
			case "supply-alias":
				r.documents[members[last].URI].supply.Params = json.RawMessage(`{"TextDocument":{}}`)
			case "supply-duplicate":
				r.documents[members[last].URI].supply.Params = json.RawMessage(`{"textDocument":{},"textDocument":{}}`)
			case "preparing":
				r.retainedScopePreparing = 1
			case "owned":
				r.protocolOwned = true
			case "closed":
				m.closed = true
			case "nil-context":
				ctx = nil
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "generation":
				s.Generation++
			case "session":
				s.SessionID = "missing"
			}
			g, f := m.BindRetainedScopePrivateV2(ctx, s.SessionID, s.Generation, members)
			if g != nil || f == "" || r.retainedScopePrivate != nil || len(r.requests) != 0 {
				t.Fatalf("accepted %s: %s", kind, f)
			}
		})
	}
}
func TestRetainedScopeV2RequestRefusals(t *testing.T) {
	for _, kind := range []string{"missing", "zero", "copied", "wrong-profile", "wrong-owner", "base-v1", "mixed", "manager", "session", "generation", "lease", "external", "dto-alias", "dto-duplicate", "unsupported", "oversize", "version", "language", "supply", "supply-session", "supply-generation", "content", "params"} {
		t.Run(kind, func(t *testing.T) {
			m, s, child, members := retainedScopeV2Fixture(t, 5, 2)
			g := bindScopeV2(t, m, s, members)
			req := retainedScopeV2Request(s, g, "textDocument/documentSymbol", members[0].URI)
			r := m.sessions[s.SessionID]
			last := members[len(members)-1]
			d := r.documents[last.URI]
			switch kind {
			case "missing":
				req.RetainedScopePrivateV2 = nil
			case "zero":
				req.RetainedScopePrivateV2 = &RetainedScopeGuardPrivateV2{}
			case "copied":
				cp := *g
				req.RetainedScopePrivateV2 = &cp
			case "wrong-profile":
				req.RetainedScopePrivateV2 = &RetainedScopeGuardPrivateV2{base: &RetainedScopeGuardPrivate{}}
			case "wrong-owner":
				cp := *g.base
				req.RetainedScopePrivateV2 = &RetainedScopeGuardPrivateV2{base: &cp}
			case "base-v1":
				req.RetainedScopePrivateV2 = nil
				req.RetainedScopePrivate = g.base
			case "mixed":
				req.RetainedScopePrivate = &RetainedScopeGuardPrivate{}
			case "manager":
				other, os, _, om := retainedScopeV2Fixture(t, 1, 0)
				bindScopeV2(t, other, os, om)
				m = other
				req.SessionID, req.Generation = os.SessionID, os.Generation
			case "session":
				req.SessionID = "missing"
			case "generation":
				req.Generation++
			case "lease":
				r.leaseNonce++
			case "external":
				req = retainedScopeV2Request(s, g, req.Method, "file:///external.go")
			case "dto-alias":
				req.Params = json.RawMessage(fmt.Sprintf(`{"TextDocument":{"uri":%q}}`, members[0].URI))
			case "dto-duplicate":
				req.Params = json.RawMessage(`{"textDocument":{},"textDocument":{}}`)
			case "unsupported":
				req.Method = "textDocument/definition"
			case "oversize":
				req.Params = make([]byte, maxRetainedScopeParamsPrivate+1)
			case "version":
				d.version++
				r.documents[last.URI] = d
			case "language":
				d.languageID = "csharp"
				r.documents[last.URI] = d
			case "supply":
				d.supply = nil
				r.documents[last.URI] = d
			case "supply-session":
				d.supply.SessionID = "foreign"
			case "supply-generation":
				d.supply.Generation++
			case "content":
				d.supply.Content[0] = 'X'
			case "params":
				d.supply.Params = json.RawMessage(`{}`)
			}
			before := retainedScopeV2WireRequests(child)
			if got := m.RoundTrip(context.Background(), req); got.Failure == "" || got.Key.ID != 0 {
				t.Fatalf("accepted %s: %+v", kind, got)
			}
			if retainedScopeV2WireRequests(child) != before || len(r.requests) != 0 {
				t.Fatal("refusal reached pending/wire")
			}
			if strings.HasPrefix(kind, "supply") || kind == "version" || kind == "language" || kind == "content" || kind == "params" {
				if f := m.CheckRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, g); f == "" {
					t.Fatal("Check missed last context drift")
				}
			}
		})
	}
}
func TestRetainedScopeV2VersionIsolation(t *testing.T) {
	for _, v2first := range []bool{false, true} {
		t.Run(fmt.Sprint(v2first), func(t *testing.T) {
			m, s, _, members := retainedScopeV2Fixture(t, 4, 2)
			if v2first {
				g := bindScopeV2(t, m, s, members)
				if f := m.CheckRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, g.base); f == "" {
					t.Fatal("V1 accepted V2 base")
				}
				if _, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members); f == "" {
					t.Fatal("V1 rebound V2")
				}
			} else {
				if _, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members); f != "" {
					t.Fatal(f)
				}
				if _, f := m.BindRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, members); f == "" {
					t.Fatal("V2 rebound V1")
				}
			}
		})
	}
	// Both attachments must fail even with an unbound session.
	m, s, child, members := retainedScopeV2Fixture(t, 1, 0)
	req := retainedScopeV2Request(s, &RetainedScopeGuardPrivateV2{}, "textDocument/documentSymbol", members[0].URI)
	req.RetainedScopePrivate = &RetainedScopeGuardPrivate{}
	before := retainedScopeV2WireRequests(child)
	if got := m.RoundTrip(context.Background(), req); got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
		t.Fatal(got)
	}
	if retainedScopeV2WireRequests(child) != before {
		t.Fatal("mixed attachments wrote")
	}
}
func TestRetainedScopeV2PreparationAndRetirement(t *testing.T) {
	m, s, _, members := retainedScopeV2Fixture(t, 2, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	m.documentFinalHook = func() { close(entered); <-release }
	done := make(chan DocumentResult, 1)
	go func() {
		done <- m.PrepareDocument(context.Background(), DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: members[0].URI, LanguageID: "go", CaptureSupply: true})
	}()
	<-entered
	_, f := m.BindRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, members)
	close(release)
	if got := <-done; got.Failure != "" {
		t.Fatal(got.Failure)
	}
	m.documentFinalHook = nil
	if f != DocumentSupplyUnavailable {
		t.Fatal("bind raced preparation", f)
	}
	g := bindScopeV2(t, m, s, members)
	m.documentFinalHook = func() { t.Error("frozen preparation reached source hook") }
	for _, capture := range []bool{false, true} {
		req := DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: members[0].URI, LanguageID: "go", CaptureSupply: capture}
		if got := m.PrepareDocument(context.Background(), req); got.Failure != DocumentSupplyUnavailable {
			t.Fatal(got)
		}
		if got := m.RefreshDocumentSupply(context.Background(), req); got.Failure != DocumentSupplyUnavailable {
			t.Fatal(got)
		}
	}
	m.documentFinalHook = nil
	m.sessions[s.SessionID].process = referenceChild{}
	m.starter = &referenceStarter{}
	accepted := m.Restart(context.Background(), s.SessionID, "v2-explicit-restart")
	if accepted.Failure != "" || accepted.IntentID == "" {
		t.Fatal(accepted)
	}
	waitOperation(t, m, accepted.IntentID, OperationComplete)
	r := m.sessions[s.SessionID]
	if r.record.Generation == s.Generation || r.retainedScopePrivate != nil || len(r.documents) != 0 {
		t.Fatal("retirement retained scope")
	}
	s.Generation = r.record.Generation
	if got := m.ObserveInitialization(s.SessionID, s.Generation, true); got.Failure != "" {
		t.Fatal(got)
	}
	if got := m.RoundTrip(context.Background(), retainedScopeV2Request(s, g, "textDocument/documentSymbol", members[0].URI)); got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
		t.Fatal(got)
	}
	if events, _ := m.RetainedScopeDiagnosticsPrivateV2(g); events != nil {
		t.Fatal("retired diagnostics available")
	}
}
func TestCheckRetainedScopeV2LeaseAndIdentity(t *testing.T) {
	m, s, child, members := retainedScopeV2Fixture(t, 2, 1)
	var g *RetainedScopeGuardPrivateV2
	err := m.WithReadySessionLease(context.Background(), SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: m.sessions[s.SessionID].record.Profile.Workspace().String()}, func(ctx context.Context) error {
		var f session.Failure
		g, f = m.BindRetainedScopePrivateV2(ctx, s.SessionID, s.Generation, members)
		if f != "" {
			t.Fatal(f)
		}
		before := retainedScopeV2WireRequests(child)
		if f = m.CheckRetainedScopePrivateV2(ctx, s.SessionID, s.Generation, g); f != "" {
			t.Fatal(f)
		}
		cp := *g
		if f = m.CheckRetainedScopePrivateV2(ctx, s.SessionID, s.Generation, &cp); f == "" {
			t.Fatal("copied Check accepted")
		}
		if events, _ := m.RetainedScopeDiagnosticsPrivateV2(&cp); events != nil {
			t.Fatal("copied diagnostics accepted")
		}
		if retainedScopeV2WireRequests(child) != before {
			t.Fatal("Check wrote wire")
		}
		if got := m.RoundTrip(ctx, retainedScopeV2Request(s, g, "textDocument/documentSymbol", members[0].URI)); got.Failure != "" {
			t.Fatal(got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if f := m.CheckRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, g); f == "" {
		t.Fatal("retired lease accepted")
	}
}
func TestCheckRetainedScopeV2EveryMember(t *testing.T) {
	m, s, child, members := retainedScopeV2Fixture(t, 8, 8)
	g := bindScopeV2(t, m, s, members)
	for _, member := range members {
		m.mu.Lock()
		original := m.sessions[s.SessionID].documents[member.URI]
		changed := original
		changed.version++
		m.sessions[s.SessionID].documents[member.URI] = changed
		m.mu.Unlock()
		before := retainedScopeV2WireRequests(child)
		if f := m.CheckRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, g); f == "" {
			t.Fatal("Check missed member drift", member.URI)
		}
		if got := m.RoundTrip(context.Background(), retainedScopeV2Request(s, g, "textDocument/documentSymbol", members[0].URI)); got.Failure != DocumentSupplyUnavailable {
			t.Fatal("request missed member drift", got)
		}
		if retainedScopeV2WireRequests(child) != before {
			t.Fatal("drift reached wire")
		}
		m.mu.Lock()
		m.sessions[s.SessionID].documents[member.URI] = original
		m.mu.Unlock()
		if f := m.CheckRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, g); f != "" {
			t.Fatal("restored member refused", f)
		}
	}
}

func TestRetainedScopeV2BindNoSourceIOAndOpaqueData(t *testing.T) {
	m, s, child, members := retainedScopeV2Fixture(t, 2, 1)
	for _, member := range members {
		u, _ := url.Parse(member.URI)
		if err := os.Remove(u.Path); err != nil {
			t.Fatal(err)
		}
	}
	m.documentFinalHook = func() { t.Error("bind or Check reached source hook") }
	before := retainedScopeV2WireRequests(child)
	g := bindScopeV2(t, m, s, members)
	if f := m.CheckRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, g); f != "" {
		t.Fatal(f)
	}
	if retainedScopeV2WireRequests(child) != before {
		t.Fatal("bind or Check wrote")
	}
	for _, method := range []string{"callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		req := retainedScopeV2Request(s, g, method, members[0].URI)
		req.Params = json.RawMessage(fmt.Sprintf(`{"item":{"uri":%q,"data":{"TextDocument":{"URI":"opaque"},"nested":[1,2]}}}`, members[0].URI))
		if got := m.RoundTrip(context.Background(), req); got.Failure != "" {
			t.Fatal("opaque item data refused", got)
		}
	}
	other, _, _, _ := retainedScopeV2Fixture(t, 1, 0)
	if events, _ := other.RetainedScopeDiagnosticsPrivateV2(g); events != nil {
		t.Fatal("foreign Manager diagnostics accepted")
	}
	for i := 0; i < 100; i++ {
		req := retainedScopeV2Request(s, g, "textDocument/documentSymbol", "file:///external.go")
		if got := m.RoundTrip(context.Background(), req); got.Failure != DocumentSupplyUnavailable {
			t.Fatal(got)
		}
	}
	events, overflow := m.RetainedScopeDiagnosticsPrivateV2(g)
	raw, _ := json.Marshal(events)
	if !overflow || len(events) > 64 || len(raw) > 8000 {
		t.Fatal("diagnostic bounds", len(events), len(raw), overflow)
	}
}

// A finite allocated-byte discriminator, not a precise heap-cost contract.
// Setup and warmup are outside measurement; the 64 KiB allowance tolerates
// incidental runtime work but cannot hide a 2 MiB Params copy on each call.
func TestRetainedScopeV2MixedAttachmentRefusesBeforeParamsCopy(t *testing.T) {
	m, s, child, _ := ownedDocumentFixture(t, true)
	t.Cleanup(func() { child.Teardown(context.Background()); child.Close() })
	params := make([]byte, maxRetainedScopeParamsPrivate)
	for i := range params {
		params[i] = ' '
	}
	copy(params, `{"textDocument":{"uri":"file:///mixed.go"}}`)
	if !json.Valid(params) {
		t.Fatal("invalid padded DTO fixture")
	}
	req := retainedScopeRequest(s, &RetainedScopeGuardPrivate{}, "textDocument/documentSymbol", "file:///mixed.go")
	req.RetainedScopePrivateV2 = &RetainedScopeGuardPrivateV2{}
	req.Params = params
	ctx := context.Background()
	if got := m.RoundTrip(ctx, req); got.Failure == "" || got.Key.ID != 0 {
		t.Fatal("warmup accepted mixed attachments")
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	const attempts = 16
	refused := true
	for i := 0; i < attempts; i++ {
		got := m.RoundTrip(ctx, req)
		refused = refused && got.Failure != "" && got.Key.ID == 0
	}
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(params)
	if !refused {
		t.Fatal("mixed attachments accepted")
	}
	bytesPerCall := (after.TotalAlloc - before.TotalAlloc) / attempts
	t.Logf("mixed attachment refusal allocated %d bytes/call over %d calls", bytesPerCall, attempts)
	if bytesPerCall >= 64<<10 {
		t.Fatalf("mixed attachment copied Params before refusal: %d bytes/call >= 65536", bytesPerCall)
	}
}

func TestRetainedScopeV2JSONValues(t *testing.T) {
	m, s, _, members := retainedScopeV2Fixture(t, 1, 0)
	g := bindScopeV2(t, m, s, members)
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"minted-value", *g}, {"zero-value", RetainedScopeGuardPrivateV2{}},
		{"minted-pointer", g}, {"zero-pointer", &RetainedScopeGuardPrivateV2{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if raw, err := json.Marshal(tc.value); err == nil {
				t.Fatalf("serialized V2 token: %s", raw)
			}
		})
	}
	var nilToken *RetainedScopeGuardPrivateV2
	if raw, err := json.Marshal(nilToken); err != nil || string(raw) != "null" {
		t.Fatalf("nil pointer JSON semantics changed: %s %v", raw, err)
	}
}

func TestRetainedScopeV2JSON(t *testing.T) {
	m, s, _, members := retainedScopeV2Fixture(t, 1, 0)
	g := bindScopeV2(t, m, s, members)
	for _, token := range []*RetainedScopeGuardPrivateV2{g, {}} {
		if _, err := json.Marshal(token); err == nil {
			t.Fatal("serialized V2")
		}
	}
	req := retainedScopeV2Request(s, g, "textDocument/documentSymbol", members[0].URI)
	req.RetainedScopePrivate = &RetainedScopeGuardPrivate{}
	for _, name := range []string{"RetainedScopePrivate", "RetainedScopePrivateV2"} {
		field, ok := reflect.TypeOf(req).FieldByName(name)
		if !ok || field.Tag.Get("json") != "-" {
			t.Fatal("attachment JSON tag missing", name)
		}
	}
	raw, err := json.Marshal(struct {
		V1 *RetainedScopeGuardPrivate   `json:"-"`
		V2 *RetainedScopeGuardPrivateV2 `json:"-"`
	}{req.RetainedScopePrivate, req.RetainedScopePrivateV2})
	if err != nil || string(raw) != "{}" {
		t.Fatalf("attachment leaked: %s %v", raw, err)
	}
	var zero RetainedScopeGuardPrivateV2
	if err = json.Unmarshal([]byte(`{}`), &zero); err != nil {
		t.Fatal(err)
	}
	if f := m.CheckRetainedScopePrivateV2(context.Background(), s.SessionID, s.Generation, &zero); f == "" {
		t.Fatal("JSON minted authority")
	}
}
