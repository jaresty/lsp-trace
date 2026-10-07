package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

const b4ID1FixtureDir = "adr0011-manager-id1-v1"
const b4ID1Manifest = "69653d8dff7133cc1829a8e183181b2149795117a0674a308985eb3d78d73a25"

type b4ID1Pin struct {
	Path   string `json:"path"`
	Length int    `json:"length"`
	SHA    string `json:"sha256"`
}
type b4ID1Fixture struct{ pins map[string]b4ID1Pin }

func b4ID1Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func b4ID1FixtureRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("fixture source location unavailable")
	}
	return filepath.Join(filepath.Dir(file), "testdata", b4ID1FixtureDir), nil
}

func b4ID1ReadFile(root, rel string) ([]byte, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("fixture root must be absolute")
	}
	if rel == "" || rel == "." || filepath.IsAbs(rel) || filepath.Clean(rel) != rel || strings.Contains(rel, `\`) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("unconfined fixture path %q", rel)
	}
	name := filepath.Join(root, rel)
	within, err := filepath.Rel(root, name)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
		return nil, fmt.Errorf("fixture path escapes root %q", rel)
	}
	current := filepath.Clean(root)
	parts := []string{current}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		parts = append(parts, current)
	}
	for _, part := range parts {
		info, err := os.Lstat(part)
		if err != nil {
			return nil, fmt.Errorf("fixture component %q: %w", part, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("fixture symlink component %q", part)
		}
	}
	info, err := os.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("fixture stat %q: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("fixture is not regular %q", rel)
	}
	return os.ReadFile(name)
}

func b4ID1Read(t *testing.T, rel string) []byte {
	t.Helper()
	root, err := b4ID1FixtureRoot()
	if err != nil {
		t.Fatalf("fixture precondition root: %v", err)
	}
	b, err := b4ID1ReadFile(root, rel)
	if err != nil {
		t.Fatalf("fixture precondition read %s: %v", rel, err)
	}
	return b
}
func b4ID1JSON(t *testing.T, b []byte, v any) {
	t.Helper()
	if e := json.Unmarshal(b, v); e != nil {
		t.Fatalf("fixture precondition JSON: %v", e)
	}
}
func b4ID1FixtureLoadAt(root, manifestSHA string) (b4ID1Fixture, error) {
	b, err := b4ID1ReadFile(root, "manifest.json")
	if err != nil {
		return b4ID1Fixture{}, err
	}
	if b4ID1Hash(b) != manifestSHA {
		return b4ID1Fixture{}, fmt.Errorf("fixture manifest pin")
	}
	var m struct {
		Assets []b4ID1Pin `json:"assets"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return b4ID1Fixture{}, fmt.Errorf("fixture manifest JSON: %w", err)
	}
	if len(m.Assets) != 57 {
		return b4ID1Fixture{}, fmt.Errorf("fixture asset count")
	}
	f := b4ID1Fixture{pins: map[string]b4ID1Pin{}}
	for _, p := range m.Assets {
		if _, ok := f.pins[p.Path]; ok {
			return b4ID1Fixture{}, fmt.Errorf("fixture duplicate pin %q", p.Path)
		}
		asset, err := b4ID1ReadFile(root, p.Path)
		if err != nil {
			return b4ID1Fixture{}, err
		}
		if len(asset) != p.Length || b4ID1Hash(asset) != p.SHA {
			return b4ID1Fixture{}, fmt.Errorf("fixture changed %q", p.Path)
		}
		f.pins[p.Path] = p
	}
	return f, nil
}

func b4ID1FixtureLoad(t *testing.T) b4ID1Fixture {
	t.Helper()
	root, err := b4ID1FixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	f, err := b4ID1FixtureLoadAt(root, b4ID1Manifest)
	if err != nil {
		t.Fatalf("fixture precondition: %v", err)
	}
	return f
}
func (f b4ID1Fixture) get(t *testing.T, path string) []byte {
	t.Helper()
	p, ok := f.pins[path]
	if !ok {
		t.Fatalf("fixture precondition missing pin %s", path)
	}
	b := b4ID1Read(t, path)
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
