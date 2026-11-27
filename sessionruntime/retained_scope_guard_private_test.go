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
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func retainedScopeFixture(t *testing.T) (*Manager, StartResult, *roundTripChild, []RetainedScopeMemberPrivate) {
	t.Helper()
	return retainedScopeObservedFixture(t, nil, true)
}

func retainedScopeObservedFixture(t *testing.T, beforeAppend func(int, lspwire.Message), acknowledge bool) (*Manager, StartResult, *roundTripChild, []RetainedScopeMemberPrivate) {
	t.Helper()
	m, s, c, first := ownedDocumentFixture(t, true)
	c.observeWrites(beforeAppend)
	t.Cleanup(func() {
		c.Teardown(context.Background())
		c.Close()
		<-c.observation.done
	})
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
	if acknowledge {
		acknowledgeRetainedScopeSetup(t, c, members)
	}
	return m, s, c, members
}

func acknowledgeRetainedScopeSetup(t *testing.T, c *roundTripChild, members []RetainedScopeMemberPrivate) {
	t.Helper()
	c.awaitRecorded(len(members))
	messages, _ := c.snapshot()
	if len(messages) != len(members) {
		t.Fatalf("setup acknowledgment: got %d messages, want %d", len(messages), len(members))
	}
	for i, msg := range messages {
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(msg.Params, &params) != nil || msg.Method != "textDocument/didOpen" || len(msg.ID) != 0 || params.TextDocument.URI != members[i].URI {
			t.Fatalf("setup acknowledgment: message %d = %+v", i, msg)
		}
	}
}

type retainedScopeTraffic struct {
	messages        []lspwire.Message
	attempts, bytes int
}

func retainedScopeTrafficSnapshot(c *roundTripChild) retainedScopeTraffic {
	messages, _ := c.snapshot()
	attempts, bytes := c.observedWrites()
	return retainedScopeTraffic{messages, attempts, bytes}
}

func (before retainedScopeTraffic) unchanged(after retainedScopeTraffic) bool {
	return reflect.DeepEqual(before.messages, after.messages) && before.attempts == after.attempts && before.bytes == after.bytes
}

// The schedule is deliberately synthetic: it witnesses the observation gap,
// not the identity of a notification in a historical CI execution.
func TestRetainedScopeRefusalObservationBoundary(t *testing.T) {
	for _, kind := range []string{"version", "digest"} {
		t.Run(kind, func(t *testing.T) {
			held, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			m, s, c, members := retainedScopeObservedFixture(t, func(n int, msg lspwire.Message) {
				if n == 6 {
					close(held)
					<-release
				}
			}, false)
			<-held // complete frame consumed; its append is still held
			g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
			if f != "" {
				t.Fatal(f)
			}
			r := m.sessions[s.SessionID]
			d := r.documents[members[5].URI]
			if kind == "version" {
				d.version++
				d.supply.DocumentVersion = d.version
				d.supply.Method = "textDocument/didChange"
				d.supply.Params, _ = json.Marshal(map[string]any{"textDocument": map[string]any{"uri": members[5].URI, "version": d.version}, "contentChanges": []map[string]string{{"text": string(d.supply.Content)}}})
				if !validRetainedScopeDocumentPrivate(d, members[5], s.SessionID, s.Generation) {
					t.Fatal("incoherent version witness")
				}
			} else {
				d.digest[0] ^= 1
			}
			r.documents[members[5].URI] = d
			req := retainedScopeRequest(s, g, "callHierarchy/incomingCalls", members[0].URI)
			old := retainedScopeTrafficSnapshot(c)
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 || len(r.requests) != 0 || got.RequestMessages != 0 || got.RequestBytes != 0 {
				t.Fatalf("witness refusal not pre-effect: %+v pending=%d", got, len(r.requests))
			}
			unblock()
			acknowledgeRetainedScopeSetup(t, c, members)
			before := retainedScopeTrafficSnapshot(c)
			methods := func(ms []lspwire.Message) []string {
				var out []string
				for _, msg := range ms {
					out = append(out, msg.Method)
				}
				return out
			}
			t.Logf("old snapshot false failure: before=%d %v after=%d %v pending=%d failure=%s id=%d attempts=%d/%d bytes=%d/%d", len(old.messages), methods(old.messages), len(before.messages), methods(before.messages), len(r.requests), got.Failure, got.Key.ID, old.attempts, before.attempts, old.bytes, before.bytes)
			if len(old.messages) != 5 || len(before.messages) != 6 || old.attempts != before.attempts || old.bytes != before.bytes {
				t.Fatal("setup scheduling witness did not isolate receiver append")
			}
			got = m.RoundTrip(context.Background(), req)
			after := retainedScopeTrafficSnapshot(c)
			if !before.unchanged(after) || got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 || len(r.requests) != 0 {
				t.Fatalf("acknowledged refusal not pre-effect: %+v pending=%d before=%+v after=%+v", got, len(r.requests), before, after)
			}
		})
	}
}

func TestRetainedScopeObserverDetectsHeldTraffic(t *testing.T) {
	held, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	_, _, c, _ := retainedScopeObservedFixture(t, func(n int, msg lspwire.Message) {
		if n == 7 {
			close(held)
			<-release
		}
	}, true)
	before := retainedScopeTrafficSnapshot(c)
	written := make(chan error, 1)
	go func() {
		written <- lspwire.NewWriter(c.Stdin(), lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "textDocument/didChange", Params: json.RawMessage(`{}`)})
	}()
	if err := awaitRetainedScopeHeldWrite(c, held, written); err != nil {
		t.Fatal(err)
	}
	after := retainedScopeTrafficSnapshot(c)
	if !reflect.DeepEqual(before.messages, after.messages) {
		t.Fatal("receiver was not held")
	}
	if before.unchanged(after) {
		t.Fatalf("prohibited held traffic escaped observer: before=%+v after=%+v", before, after)
	}
	t.Logf("held traffic detected: receiver=%d/%d attempts=%d/%d bytes=%d/%d", len(before.messages), len(after.messages), before.attempts, after.attempts, before.bytes, after.bytes)
	unblock()
	c.awaitRecorded(7)
	if before.unchanged(retainedScopeTrafficSnapshot(c)) {
		t.Fatal("recorded traffic escaped observer")
	}
}

// A successful pipe write can finish before beforeAppend runs. Wait for both
// observations, but never strand the caller on held after a terminal failure.
func awaitRetainedScopeHeldWrite(c *roundTripChild, held <-chan struct{}, written <-chan error) error {
	defer func() {
		if written != nil {
			// Receiver exit must also join a writer whose result is not yet read.
			_ = c.stdin.Close()
			<-written
		}
	}()
	for held != nil || written != nil {
		select {
		case <-held:
			held = nil
		case err := <-written:
			written = nil
			if err != nil {
				return fmt.Errorf("held write failed: %w", err)
			}
		case <-c.observation.done:
			return fmt.Errorf("receiver exited before held write completed")
		}
	}
	return nil
}

func TestRetainedScopeHeldWriteTermination(t *testing.T) {
	for _, kind := range []string{"closed-writer", "receiver-exit"} {
		t.Run(kind, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newRoundTripChild("references-empty")
				receiverEntered, receiverRelease := make(chan struct{}), make(chan struct{})
				var hook func(int, lspwire.Message)
				if kind == "closed-writer" {
					hook = func(int, lspwire.Message) { close(receiverEntered); <-receiverRelease }
				}
				c.observeWrites(hook)
				defer func() {
					close(receiverRelease)
					c.Teardown(context.Background())
					c.Close()
					<-c.observation.done
				}()
				held := make(chan struct{})
				written := make(chan error, 1)
				if kind == "closed-writer" {
					// Keep the receiver alive on an earlier frame, so receiver exit
					// cannot mask a missing early-write-error select case.
					if err := lspwire.NewWriter(c.Stdin(), lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "textDocument/didChange", Params: json.RawMessage(`{}`)}); err != nil {
						t.Fatal(err)
					}
					<-receiverEntered
					_ = c.stdin.Close()
				}
				go func() {
					written <- lspwire.NewWriter(c.Stdin(), lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "textDocument/didChange", Params: json.RawMessage(`{}`)})
				}()
				// Obtain the actual pipe result before testing the wait. In the
				// receiver-exit case it is nil, but no held callback ever occurred.
				writeErr := <-written
				if (writeErr != nil) != (kind == "closed-writer") {
					t.Fatalf("unexpected real-pipe result: %v", writeErr)
				}
				written <- writeErr
				if kind == "receiver-exit" {
					_ = c.input.Close()
					<-c.observation.done
				}
				returned := make(chan error, 1)
				go func() { returned <- awaitRetainedScopeHeldWrite(c, held, written) }()
				synctest.Wait()
				select {
				case err := <-returned:
					if err == nil {
						t.Fatal("terminal wait accepted absent hold")
					}
					if len(written) != 0 {
						t.Fatal("terminal wait did not join writer result")
					}
					t.Logf("terminal wait returned: case=%s write=%v wait=%v writer joined; receiver release/join deferred", kind, writeErr, err)
				default:
					// Recovery is after the observation: make the old unconditional
					// held wait joinable, rather than hanging the negative control.
					close(held)
					<-returned
					t.Fatal("terminal wait stranded before held acknowledgment")
				}
			})
		})
	}
}

func TestRetainedScopeHeldWriteNeedsAcknowledgment(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered, allowHold := make(chan struct{}), make(chan struct{})
		held, release := make(chan struct{}), make(chan struct{})
		var allowOnce, releaseOnce sync.Once
		allow := func() { allowOnce.Do(func() { close(allowHold) }) }
		unblock := func() { releaseOnce.Do(func() { close(release) }) }
		c := newRoundTripChild("references-empty")
		c.observeWrites(func(int, lspwire.Message) { close(entered); <-allowHold; close(held); <-release })
		defer func() { allow(); unblock(); c.Teardown(context.Background()); c.Close(); <-c.observation.done }()
		written := make(chan error, 1)
		go func() {
			written <- lspwire.NewWriter(c.Stdin(), lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "textDocument/didChange", Params: json.RawMessage(`{}`)})
		}()
		<-entered
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		written <- nil
		returned := make(chan error, 1)
		go func() { returned <- awaitRetainedScopeHeldWrite(c, held, written) }()
		synctest.Wait()
		select {
		case err := <-returned:
			t.Fatalf("nil write completion substituted for acknowledgment: %v", err)
		default:
		}
		allow()
		if err := <-returned; err != nil {
			t.Fatal(err)
		}
		unblock()
		c.awaitRecorded(1)
		t.Log("nil write completion waited for real hold acknowledgment; writer joined")
	})
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
				other, otherSession, otherChild, otherMembers := retainedScopeFixture(t)
				if _, failure := other.BindRetainedScopePrivate(ctx, otherSession.SessionID, otherSession.Generation, otherMembers); failure != "" {
					t.Fatal(failure)
				}
				m, c = other, otherChild
				r = other.sessions[otherSession.SessionID]
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
			before := retainedScopeTrafficSnapshot(c)
			got := m.RoundTrip(ctx, req)
			after := retainedScopeTrafficSnapshot(c)
			if got.Failure == "" || got.Key.ID != 0 || len(r.requests) != 0 || !before.unchanged(after) {
				t.Fatalf("not pre-effect: %+v pending=%d before=%+v after=%+v", got, len(r.requests), before, after)
			}
			if (kind == "version" || kind == "digest") && got.Failure != DocumentSupplyUnavailable {
				t.Fatalf("wrong refusal classification: %+v", got)
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
