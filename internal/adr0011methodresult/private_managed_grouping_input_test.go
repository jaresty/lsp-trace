package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// The existing composedChild/composedStarter are io.Pipe simulated peers. No
// executable is started and these exchanges are not provider qualification.
type groupingPeer struct {
	mu              sync.Mutex
	methods         []string
	results         map[string][]byte
	publicationPath string
	dropMethod      string
	entered         chan struct{}
}

func groupingFixture(t *testing.T, refs, defs bool, languages ...string) (*sessionruntime.Manager, PrivateManagedGroupingRequest, *groupingPeer) {
	t.Helper()
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	language := "go"
	if len(languages) > 0 {
		language = languages[0]
	}
	path := filepath.Join(workspace, "query."+language)
	source := []byte("package fixture\nfunc Query() {}\n")
	if language == "cue" {
		source = []byte("package fixture\nlet  Query = string\n")
	}
	if err := os.WriteFile(path, source, 0644); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	symbol := []byte(`[{"name":"Query","kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":15}},"selectionRange":{"start":{"line":1,"character":5},"end":{"line":1,"character":10}}}]`)
	location := fmt.Sprintf(`{"uri":%q,"range":{"start":{"line":1,"character":5},"end":{"line":1,"character":10}}}`, uri)
	p := &groupingPeer{results: map[string][]byte{"textDocument/documentSymbol": symbol, "textDocument/references": []byte("[" + location + "," + location + "]"), "textDocument/definition": []byte("[" + location + "," + location + "]")}}
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &composedChild{input: input, stdin: stdin, stdout: stdout, output: output, observed: make(chan error, 1)}
	go func() {
		r := lspwire.NewReader(input, lspwire.DefaultLimits())
		for {
			msg, err := r.Read()
			if err != nil {
				return
			}
			p.mu.Lock()
			p.methods = append(p.methods, msg.Method)
			result := append([]byte(nil), p.results[msg.Method]...)
			drop, entered := p.dropMethod == msg.Method, p.entered
			p.mu.Unlock()
			if drop {
				if entered != nil {
					select {
					case entered <- struct{}{}:
					default:
					}
				}
				continue
			}
			if len(msg.ID) == 0 {
				continue
			}
			if msg.Method == "initialize" {
				result = []byte(fmt.Sprintf(`{"serverInfo":{"name":"controlled-io-pipe","version":"fixture-v1"},"capabilities":{"positionEncoding":"utf-16","documentSymbolProvider":true,"referencesProvider":%t,"definitionProvider":%t}}`, refs, defs))
			}
			if len(result) == 0 {
				result = []byte("null")
			}
			body := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, result))
			if _, err = fmt.Fprintf(output, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
				return
			}
		}
	}()
	m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 4, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Starter: composedStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Teardown(context.Background()); child.Close(); m.Shutdown(context.Background()) })
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: language, EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), LanguageID: language})
	if s.Failure != "" {
		t.Fatal(s.Failure)
	}
	composedRequireReadiness(t, m, s)
	rootPath := filepath.Join(t.TempDir(), "published")
	if err = os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	p.publicationPath = rootPath
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	req := PrivateManagedGroupingRequest{Session: sessionruntime.SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: workspace}, URI: uri, OccurrenceID: "controlled-query", Revision: "controlled-revision", Privacy: "controlled-private", OccurrenceAdmissionPolicyID: "controlled-ordinal-policy", CompositionPolicyID: "controlled-identity-policy", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Root: root, Deadline: time.Now().Add(10 * time.Second)}
	return m, req, p
}

func TestPrivateManagedGroupingControlledBaseline(t *testing.T) {
	m, q, _ := groupingFixture(t, true, true)
	err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
		d := m.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: q.Session.SessionID, Generation: q.Session.Generation, URI: q.URI, LanguageID: "go", CaptureSupply: true})
		if d.Failure != "" || d.Supply == nil {
			t.Fatalf("BASELINE_SUPPLY: %+v", d)
		}
		binding := sessionruntime.OwnedDocumentBinding{URI: q.URI, Version: d.Version, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(d.Supply.Content))}
		params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": q.URI}})
		wire := transport.New(m).Execute(ctx, transport.Request{SessionID: q.Session.SessionID, Generation: q.Session.Generation, Method: "textDocument/documentSymbol", Params: params, Deadline: q.Deadline, MaxMessages: 4, MaxBytes: 65536, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding})
		pair, ok := wire.OwnedPair()
		if !ok || wire.Outcome() != transport.OutcomeTransportSuccess {
			t.Fatalf("BASELINE_MANAGED_PAIR: %s %s", wire.Outcome(), wire.FailureText())
		}
		candidate, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(adr0011querytarget.Query{OccurrenceID: q.OccurrenceID, URI: q.URI, Encoding: "utf-16", DocumentVersion: fmt.Sprint(d.Version), SourceDigest: binding.SHA256, SessionID: q.Session.SessionID, Generation: q.Session.Generation, Line: q.Line, Character: q.Character}, pair.Result)
		if err != nil || candidate.SymbolName != "Query" {
			t.Fatalf("BASELINE_CANONICAL_TARGET: %+v %v", candidate, err)
		}
		for _, method := range []string{"textDocument/references", "textDocument/definition"} {
			position := map[string]uint32{"line": q.Line, "character": q.Character}
			p := map[string]any{"textDocument": map[string]string{"uri": q.URI}, "position": position}
			if method == "textDocument/references" {
				p["context"] = map[string]bool{"includeDeclaration": false}
			}
			raw, _ := json.Marshal(p)
			w := transport.New(m).Execute(ctx, transport.Request{SessionID: q.Session.SessionID, Generation: q.Session.Generation, Method: method, Params: raw, Deadline: q.Deadline, MaxMessages: 4, MaxBytes: 65536, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding})
			native, ok := w.OwnedPair()
			if !ok || !bytes.Equal(native.Params, raw) || w.Outcome() != transport.OutcomeTransportSuccess {
				t.Fatalf("BASELINE_NATIVE_METHOD: %s %s", method, w.FailureText())
			}
			parsed, failure := Parse(w, 8)
			if failure != nil || len(parsed.Items) != 2 || parsed.Items[1].Ordinal != 1 {
				t.Fatalf("BASELINE_NATIVE_ORDINAL: %+v %v", parsed, failure)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("BASELINE_ACTUAL_CONTROLLED_MANAGED_D_R: PASS")
}

func TestPrivateManagedGroupingReceiptToTypedInput(t *testing.T) {
	m, q, _ := groupingFixture(t, true, true)
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	if err != nil || len(got.Input.Contributions()) != 4 {
		t.Fatalf("ASSERT_PRIVATE_OWNED_RECEIPT_TO_TYPED_INPUT: err=%v contributions=%d", err, len(got.Input.Contributions()))
	}
	t.Log("ASSERT_PRIVATE_OWNED_RECEIPT_TO_TYPED_INPUT: PASS")
}
