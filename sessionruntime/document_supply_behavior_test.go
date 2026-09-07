package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

type supplyWriter struct {
	bytes.Buffer
	fail       bool
	afterWrite func()
}

func (w *supplyWriter) Write(p []byte) (int, error) {
	if w.fail {
		return 0, errors.New("injected failed notification write")
	}
	n, err := w.Buffer.Write(p)
	if bytes.Contains(p, []byte(`"textDocument"`)) && w.afterWrite != nil {
		w.afterWrite()
	}
	return n, err
}
func (*supplyWriter) Close() error { return nil }

type supplyChild struct{ writer *supplyWriter }

func (c supplyChild) Stdin() io.WriteCloser { return c.writer }
func (supplyChild) Stdout() io.ReadCloser   { return io.NopCloser(bytes.NewReader(nil)) }
func (supplyChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (supplyChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func supplyFixture(t *testing.T, content []byte) (*Manager, DocumentRequest, string, *supplyWriter) {
	t.Helper()
	root := t.TempDir()
	file := filepath.Join(root, "seed.go")
	if err := os.WriteFile(file, content, 0600); err != nil {
		t.Fatal(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	writer := &supplyWriter{}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 16}, Starter: oneChildStarter{supplyChild{writer}}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(selected)})
	if started.Failure != "" {
		t.Fatal(started)
	}
	if result := m.ObserveInitialization(started.SessionID, started.Generation, true); result.Failure != "" {
		t.Fatal(result)
	}
	req := DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String(), LanguageID: "go", CaptureSupply: true}
	return m, req, file, writer
}

func TestDocumentSupplySameReadSurvivesLaterMutation(t *testing.T) {
	a := []byte("package fixture\n// A: café 😀\r\n")
	b := []byte("package fixture\n// B\n")
	m, req, file, writer := supplyFixture(t, a)
	writer.afterWrite = func() {
		if err := os.WriteFile(file, b, 0600); err != nil {
			t.Error(err)
		}
	}
	result := m.PrepareDocument(context.Background(), req)
	if result.Failure != "" || result.Supply == nil {
		t.Fatalf("ASSERT_SUPPLY_SUCCESS: %+v", result)
	}
	s := result.Supply
	if s.Classification != "LSP_SUPPLIED" || s.SessionID != req.SessionID || s.Generation != req.Generation || s.URI != req.URI || s.DocumentVersion != 1 || s.Method != "textDocument/didOpen" || !bytes.Equal(s.Content, a) {
		t.Fatalf("ASSERT_EXACT_SAME_READ_SUPPLY: %+v", s)
	}
	msg, err := lspwire.NewReader(bytes.NewReader(writer.Bytes()), lspwire.DefaultLimits()).Read()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Method != s.Method || !bytes.Equal(msg.Params, s.Params) {
		t.Fatal("ASSERT_EXACT_NOTIFICATION_PARAMS")
	}
	var params struct {
		TextDocument struct {
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(s.Params, &params); err != nil || params.TextDocument.Text != string(a) {
		t.Fatalf("ASSERT_WIRE_TEXT_EXACT: %v", err)
	}
	later, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(later, b) {
		t.Fatalf("ASSERT_ACTUAL_A_TO_B_MUTATION: %q %v", later, err)
	}
	writer.afterWrite = nil
	second := m.PrepareDocument(context.Background(), req)
	if second.Failure != "" || second.Supply == nil || second.Supply.DocumentVersion != 2 || second.Supply.Method != "textDocument/didChange" || !bytes.Equal(second.Supply.Content, b) || !bytes.Equal(s.Content, a) {
		t.Fatalf("ASSERT_COMPETING_SUPPLIES_REMAIN_SEPARATE: %+v", second)
	}
	var change struct {
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(second.Supply.Params, &change); err != nil || len(change.ContentChanges) != 1 || change.ContentChanges[0].Text != string(b) {
		t.Fatal("ASSERT_FULL_TEXT_CHANGE")
	}
	// Mutating returned bytes cannot affect a cached digest or another result.
	s.Content[0] = 'X'
	second.Supply.Params[0] = 'X'
	before := writer.Len()
	cached := m.PrepareDocument(context.Background(), req)
	if cached.Failure != "" || cached.Version != 2 || cached.Supply != nil || writer.Len() != before {
		t.Fatalf("ASSERT_CACHED_IS_NOT_NEW_SUPPLY: %+v", cached)
	}
	req.Generation++
	stale := m.PrepareDocument(context.Background(), req)
	if stale.Failure != session.StaleGeneration || stale.Supply != nil || writer.Len() != before {
		t.Fatalf("ASSERT_STALE_HAS_NO_SUPPLY: %+v", stale)
	}
}

func TestDocumentSupplyRemainsHistoricalAfterFailedRestart(t *testing.T) {
	m, req, _, _ := supplyFixture(t, []byte("package fixture\n"))
	first := m.PrepareDocument(context.Background(), req)
	if first.Failure != "" || first.Supply == nil {
		t.Fatalf("ASSERT_INITIAL_SUPPLY: %+v", first)
	}
	// This fake child intentionally cannot complete graceful shutdown.
	// Failed lifecycle must not fabricate a new supply observation.
	accepted := m.Restart(context.Background(), req.SessionID, "supply-test")
	if accepted.Failure != "" {
		t.Fatal(accepted)
	}
	waitOperation(t, m, accepted.IntentID, OperationFailed)
	stale := m.PrepareDocument(context.Background(), req)
	if stale.Failure != session.LifecycleConflict || stale.Supply != nil {
		t.Fatalf("ASSERT_FAILED_LIFECYCLE_REJECTS_SUPPLY: %+v", stale)
	}
	if first.Supply.Generation != req.Generation || first.Supply.Classification != "LSP_SUPPLIED" {
		t.Fatal("ASSERT_RESTART_DOES_NOT_REWRITE_HISTORICAL_SUPPLY")
	}
}

func TestDocumentSupplyOmittedAndLateOptIn(t *testing.T) {
	m, req, _, writer := supplyFixture(t, []byte("package fixture\n"))
	req.CaptureSupply = false
	first := m.PrepareDocument(context.Background(), req)
	if first.Failure != "" || first.Supply != nil {
		t.Fatalf("ASSERT_OMITTED_NO_BYTES: %+v", first)
	}
	before := writer.Len()
	req.CaptureSupply = true
	late := m.PrepareDocument(context.Background(), req)
	if late.Failure != "" || late.Supply != nil || writer.Len() != before {
		t.Fatalf("ASSERT_NO_RETROSPECTIVE_SUPPLY: %+v", late)
	}
}

func TestDocumentSupplyFailureAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name      string
		content   []byte
		failure   session.Failure
		failWrite bool
	}{
		{"invalid-utf8", []byte{'p', 0xff}, DocumentSupplyUnavailable, false},
		{"too-large", bytes.Repeat([]byte{'x'}, MaxDocumentSupplyBytes+1), DocumentSupplyUnavailable, false},
		{"failed-write", []byte("package fixture\n"), session.SessionPoisoned, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, req, _, writer := supplyFixture(t, tc.content)
			writer.fail = tc.failWrite
			got := m.PrepareDocument(context.Background(), req)
			if got.Failure != tc.failure || got.Supply != nil || writer.Len() != 0 {
				t.Fatalf("ASSERT_NO_FALSE_SUPPLY: %+v bytes=%d", got, writer.Len())
			}
		})
	}
}

func TestDocumentSupplyScopeAndCanonicalURI(t *testing.T) {
	for _, kind := range []string{"query", "fragment", "dot-path", "escape-link", "directory", "missing"} {
		t.Run(kind, func(t *testing.T) {
			m, req, file, writer := supplyFixture(t, []byte("package fixture\n"))
			switch kind {
			case "query":
				req.URI += "?alias=1"
			case "fragment":
				req.URI += "#alias"
			case "dot-path":
				req.URI = (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Dir(file)) + "/./seed.go"}).String()
			case "escape-link":
				outside := filepath.Join(t.TempDir(), "outside.go")
				if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, file); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(file, 0700); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			got := m.PrepareDocument(context.Background(), req)
			if got.Failure == "" || got.Supply != nil || writer.Len() != 0 {
				t.Fatalf("ASSERT_SCOPE_REJECT_WITHOUT_SUPPLY: %+v", got)
			}
		})
	}
}
