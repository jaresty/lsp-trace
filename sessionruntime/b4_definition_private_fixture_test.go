package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

const b4ID1Root = "../.pi/evidence/adr0011-composed-b4-manager-id1-held-v1"
const b4ID1Manifest = "e2d1880bd3fc09ca6c2e1f6f186d4d387d745cc10e72522597101d1d661ceb4b"

type b4ID1Pin struct {
	Path   string `json:"path"`
	Length int    `json:"length"`
	SHA    string `json:"sha256"`
}
type b4ID1Fixture struct{ pins map[string]b4ID1Pin }

func b4ID1Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func b4ID1Read(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatalf("fixture precondition read %s: %v", p, e)
	}
	return b
}
func b4ID1JSON(t *testing.T, b []byte, v any) {
	t.Helper()
	if e := json.Unmarshal(b, v); e != nil {
		t.Fatalf("fixture precondition JSON: %v", e)
	}
}
func b4ID1FixtureLoad(t *testing.T) b4ID1Fixture {
	t.Helper()
	b := b4ID1Read(t, filepath.Join(b4ID1Root, "manifest.json"))
	if b4ID1Hash(b) != b4ID1Manifest {
		t.Fatal("fixture precondition manifest pin")
	}
	var m struct {
		Assets []b4ID1Pin `json:"assets"`
	}
	b4ID1JSON(t, b, &m)
	if len(m.Assets) != 57 {
		t.Fatal("fixture precondition asset count")
	}
	f := b4ID1Fixture{pins: map[string]b4ID1Pin{}}
	for _, p := range m.Assets {
		if _, ok := f.pins[p.Path]; ok {
			t.Fatal("fixture precondition duplicate pin")
		}
		f.pins[p.Path] = p
		f.get(t, p.Path)
	}
	return f
}
func (f b4ID1Fixture) get(t *testing.T, path string) []byte {
	t.Helper()
	p, ok := f.pins[path]
	if !ok {
		t.Fatalf("fixture precondition missing pin %s", path)
	}
	b := b4ID1Read(t, filepath.Join(b4ID1Root, path))
	if len(b) != p.Length || b4ID1Hash(b) != p.SHA {
		t.Fatalf("fixture precondition changed %s", path)
	}
	return b
}

type b4ID1Child struct {
	input    *io.PipeReader
	stdin    *io.PipeWriter
	output   *io.PipeWriter
	stdout   *io.PipeReader
	response []byte
}

func newB4ID1Child(response []byte) *b4ID1Child {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &b4ID1Child{input: input, stdin: stdin, output: output, stdout: stdout, response: append([]byte(nil), response...)}
	go func() {
		r := lspwire.NewReader(input, lspwire.DefaultLimits())
		for {
			msg, e := r.Read()
			if e != nil {
				return
			}
			if msg.Method == "textDocument/definition" && bytes.Equal(msg.ID, []byte("1")) {
				_, _ = output.Write(c.response)
			}
		}
	}()
	return c
}
func (c *b4ID1Child) Stdin() io.WriteCloser { return c.stdin }
func (c *b4ID1Child) Stdout() io.ReadCloser { return c.stdout }
func (c *b4ID1Child) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *b4ID1Child) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func b4ID1Manager(t *testing.T, f b4ID1Fixture, c string) (*Manager, StartResult, RoundTripRequest, B4DefinitionOwner) {
	t.Helper()
	var d struct {
		Session, Method, Transaction, CompletedOwnerKey string
		Generation                                      uint64
		ProfileSelector                                 struct {
			TrustDomain, Workspace, Profile, EnvironmentReference string `json:"-"`
		}
	}
	// Decode selector separately: its snake_case keys must not be guessed from a response.
	var raw map[string]json.RawMessage
	b4ID1JSON(t, f.get(t, c+"/DECLARATION.json"), &raw)
	b4ID1JSON(t, f.get(t, c+"/DECLARATION.json"), &d)
	var selector struct {
		TrustDomain          string `json:"trust_domain"`
		Workspace            string
		Profile              string
		EnvironmentReference string `json:"environment_reference"`
	}
	b4ID1JSON(t, raw["profile_selector"], &selector)
	validated, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: selector.TrustDomain, Workspace: selector.Workspace, Profile: selector.Profile, EnvironmentReference: selector.EnvironmentReference})
	if e != nil {
		t.Fatalf("fixture precondition profile: %v", e)
	}
	child := newB4ID1Child(f.get(t, c+"/response.frame"))
	m, e := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if e != nil {
		t.Fatalf("fixture precondition manager: %v", e)
	}
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	s := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if s.SessionID != d.Session || s.Generation != d.Generation || s.Generation != 1 {
		t.Fatalf("fixture precondition profile/session: got %q/%d want %q/%d", s.SessionID, s.Generation, d.Session, d.Generation)
	}
	if ready := m.ObserveInitialization(s.SessionID, s.Generation, true); ready.State != session.Ready {
		t.Fatalf("fixture precondition readiness: %+v", ready)
	}
	req := RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: d.Method, Params: json.RawMessage(f.get(t, c+"/request.params")), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureDefinitionResponseFrameMaxBytes: int64(len(f.get(t, c+"/response.frame"))), CaptureMethodRequestFrameMaxBytes: int64(len(f.get(t, c+"/request.frame")))}
	return m, s, req, B4DefinitionOwner{Transaction: d.Transaction, CompletedOwnerKey: d.CompletedOwnerKey}
}
