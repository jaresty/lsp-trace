package sessionclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisition/sessionclient"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type cancellationChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	stdout *io.PipeReader
	output *io.PipeWriter
	once   sync.Once
	closed chan struct{}
}

func newCancellationChild() *cancellationChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	return &cancellationChild{input: input, stdin: stdin, stdout: stdout, output: output, closed: make(chan struct{})}
}
func (c *cancellationChild) Stdin() io.WriteCloser { return c.stdin }
func (c *cancellationChild) Stdout() io.ReadCloser { return c.stdout }
func (c *cancellationChild) Close() managedprocess.ResourceObservation {
	c.once.Do(func() { c.stdin.Close(); c.input.Close(); c.stdout.Close(); c.output.Close(); close(c.closed) })
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *cancellationChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}

type cancellationStarter struct{ child *cancellationChild }

func (s cancellationStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

// This is the independent review's real-Manager stalled-pipe reproduction,
// inverted into a correctness guard. The watchdog reports an assertion failure,
// closes the fixture, and joins Acquire even on the original broken runtime.
func TestManagedSupplyDeadline(t *testing.T) {
	for _, prior := range []bool{false, true} {
		t.Run(map[bool]string{false: "stalled-open", true: "retained-earlier-evidence"}[prior], func(t *testing.T) {
			dir := t.TempDir()
			uri := func(name string) string {
				p := filepath.Join(dir, name)
				if err := os.WriteFile(p, []byte("package p\nfunc a() {}\n"), 0600); err != nil {
					t.Fatal(err)
				}
				return (&url.URL{Scheme: "file", Path: p}).String()
			}
			rootURI, nextURI := uri("a.go"), uri("b.go")
			child := newCancellationChild()
			defer child.Close()
			m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: cancellationStarter{child}})
			if err != nil {
				t.Fatal(err)
			}
			v, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: dir, Profile: "go", EnvironmentReference: "local"})
			if err != nil {
				t.Fatal(err)
			}
			s := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(v)})
			if x := m.ObserveInitialization(s.SessionID, s.Generation, true); x.State != session.Ready {
				t.Fatal(x)
			}
			serverDone := make(chan struct{})
			if prior {
				go func() {
					defer close(serverDone)
					reader := lspwire.NewReader(child.input, lspwire.DefaultLimits())
					writer := lspwire.NewWriter(child.output, lspwire.DefaultLimits())
					// Confirm root didOpen and its prepare response, then stop reading before b's didOpen.
					if _, err := reader.Read(); err != nil {
						return
					}
					msg, err := reader.Read()
					if err != nil {
						return
					}
					item := lsp.CallHierarchyItem{Name: "a", Kind: 12, URI: rootURI, Range: lsp.Range{End: lsp.Position{Character: 10}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}
					raw, _ := json.Marshal([]lsp.CallHierarchyItem{item})
					_ = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: raw})
				}()
			} else {
				close(serverDone)
			}
			r := req()
			r.Context.SessionID = s.SessionID
			r.Context.Generation = s.Generation
			r.Root.Locator.URI = rootURI
			r.Limits.Timeout = 10 * time.Millisecond
			r.Limits.RequestTimeout = 5 * time.Millisecond
			if prior {
				target := r.Root
				target.ID = "next"
				target.Locator.URI = nextURI
				r.RequiredTargets = []acquisition.Target{target}
			}
			type outcome struct {
				result acquisition.Result
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				result, err := acquisition.Acquire(context.Background(), sessionclient.New(m), r)
				done <- outcome{result, err}
			}()
			var got outcome
			select {
			case got = <-done:
				t.Log("ASSERT_SUPPLY_DEADLINE: PASS")
			case <-time.After(500 * time.Millisecond):
				t.Error("ASSERT_SUPPLY_DEADLINE: Acquire exceeded 500ms with global=10ms request=5ms")
				child.Close()
				select {
				case got = <-done:
				case <-time.After(time.Second):
					t.Fatal("fixture cleanup did not release Acquire")
				}
			}
			<-serverDone
			if got.err != nil {
				t.Fatal(got.err)
			}
			if c := m.Census(); c.Workers != 0 || c.Requests != 0 {
				t.Fatalf("ASSERT_SUPPLY_RELEASE: %+v", c)
			}
			select {
			case <-child.closed:
			default:
				t.Error("ASSERT_SUPPLY_RETIRE: stalled generation transport remains open")
			}
			if prior {
				if len(got.result.Supplies) != 1 || got.result.Targets[0].Resolution.Status != acquisition.Resolved {
					t.Fatalf("ASSERT_SUPPLY_PRIOR: supplies=%d root=%+v", len(got.result.Supplies), got.result.Targets[0].Resolution)
				}
			} else if len(got.result.Supplies) != 0 {
				t.Fatal("ASSERT_SUPPLY_NO_FABRICATION: failed write supplied evidence")
			}
		})
	}
}
