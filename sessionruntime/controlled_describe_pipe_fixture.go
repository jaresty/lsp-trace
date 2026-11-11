//go:build lsp_trace_controlled_describe_fixture

package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
)

// Explicit build-selected test support, never native backend coverage. The tag
// is not a security boundary. No caller-supplied protocol, source or Starter is
// accepted and no source evidence/completion authority is constructed here.
const controlledDescribeDocument = "package fixture\nfunc Query() {}\nvar use = Query\n"
const controlledDescribeProcess = "controlled-Describe-io.Pipe"

type ControlledDescribePipeFixture struct {
	mu       sync.Mutex
	manager  *Manager
	child    *controlledDescribeChild
	root     *os.Root
	literal  os.FileInfo
	uri      string
	start    StartRequest
	closed   bool
	closeErr error
}

func NewControlledDescribePipeFixture(root string) (*ControlledDescribePipeFixture, error) {
	return newControlledDescribePipeFixture(root, false)
}

// NewControlledDescribeFinishFailurePipeFixture simulates a failed resource-close
// report AFTER actual pipe close and server join. It is not an OS-close fault.
// Native failed custody remains failed on every subsequent Close report.
func NewControlledDescribeFinishFailurePipeFixture(root string) (*ControlledDescribePipeFixture, error) {
	return newControlledDescribePipeFixture(root, true)
}

func newControlledDescribePipeFixture(root string, failCloseReport bool) (*ControlledDescribePipeFixture, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, errors.New("canonical temporary root required")
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || canonical != root {
		return nil, errors.Join(errors.New("canonical temporary root required"), err)
	}
	temp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(temp, root)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return nil, errors.New("private temporary child required")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, errors.Join(errors.New("private directory required"), err)
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*ControlledDescribePipeFixture, error) {
		return nil, errors.Join(cause, handle.Close())
	}
	pinned, err := handle.Stat(".")
	if err != nil || !os.SameFile(info, pinned) {
		return fail(errors.Join(errors.New("root identity changed"), err))
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		return fail(errors.Join(errors.New("empty fixture directory required"), err))
	}
	file, err := handle.OpenFile("query.go", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fail(err)
	}
	_, writeErr := io.WriteString(file, controlledDescribeDocument)
	literal, statErr := file.Stat()
	closeErr := file.Close()
	if err = errors.Join(writeErr, statErr, closeErr); err != nil {
		return fail(errors.Join(err, handle.Remove("query.go")))
	}
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		return fail(errors.Join(err, handle.Remove("query.go")))
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "query.go")}).String()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	child := &controlledDescribeChild{input: inW, serverInput: inR, output: outR, serverOutput: outW, done: make(chan struct{}), closing: make(chan struct{}), uri: uri, failCloseReport: failCloseReport}
	go child.serve()
	starter := &controlledDescribeStarter{child: child, root: root}
	manager, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 16, MaxChildren: 16, MaxCancels: 8, MaxTombstones: 32, MaxObservations: 256, MaxOperations: 8}, Starter: starter})
	if err != nil {
		child.stop()
		<-child.done
		return fail(errors.Join(err, child.stopErr, child.serveErr, handle.Remove("query.go")))
	}
	return &ControlledDescribePipeFixture{manager: manager, child: child, root: handle, literal: literal, uri: uri, start: StartRequest{Profile: runtimeprofile.Resolve(selector), Process: managedprocess.Spec{Path: controlledDescribeProcess, Dir: root}, LanguageID: "go"}}, nil
}

// ControlledDescribeCloseSnapshot is detached test observation, not custody or
// authority. Physical pipe join is distinct from Manager/native cleanup success.
type ControlledDescribeCloseSnapshot struct {
	CloseReports, ReferenceReplies uint64
	PipeJoined, PhysicalCloseOK    bool
}

func (f *ControlledDescribePipeFixture) CloseSnapshot() ControlledDescribeCloseSnapshot {
	c := f.child
	s := ControlledDescribeCloseSnapshot{CloseReports: c.closeReports.Load(), ReferenceReplies: c.referenceReplies.Load()}
	select {
	case <-c.done:
		s.PipeJoined = true
	default:
	}
	if s.CloseReports != 0 {
		s.PhysicalCloseOK = c.stopErr == nil && c.serveErr == nil
	}
	return s
}

func (f *ControlledDescribePipeFixture) Manager() *Manager { return f.manager }
func (f *ControlledDescribePipeFixture) URI() string       { return f.uri }
func (f *ControlledDescribePipeFixture) StartRequest(deadline time.Time) StartRequest {
	q := f.start
	q.Deadline = deadline
	return q
}

// Close interrupts both directions and joins the actual pipe server. Success
// additionally requires Manager cleanup/zero workers. It removes only its own
// literal and filesystem capability even when native custody remains sticky;
// that failure is returned, never relabeled as successful completion. If the
// server join itself times out, a later bounded Close can retry that join.
func (f *ControlledDescribePipeFixture) Close(ctx context.Context) error {
	if f == nil || ctx == nil {
		return errors.New("fixture cleanup context required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return f.closeErr
	}
	f.child.stop()
	select {
	case <-f.child.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	err := errors.Join(f.child.stopErr, f.child.serveErr, f.manager.Shutdown(ctx), ctx.Err())
	census := f.manager.Census()
	if census.Workers != 0 || census.Sessions != 0 {
		err = errors.Join(err, errors.New("fixture lifecycle not joined"))
	}
	// Native failed-window custody is deliberately not refunded or rewritten.
	// Even when that ownership remains sticky, release our separate filesystem
	// capability and literal after the actual pipe server has joined, and report
	// the native cleanup failure rather than leaking our descriptor or succeeding.
	current, removeErr := f.root.Lstat("query.go")
	if removeErr == nil && !os.SameFile(f.literal, current) {
		removeErr = errors.New("fixture literal replaced; refusing removal")
	}
	if removeErr == nil {
		removeErr = f.root.Remove("query.go")
	}
	f.closeErr = errors.Join(err, removeErr, f.root.Close())
	f.closed = true
	return f.closeErr
}

type controlledDescribeStarter struct {
	mu      sync.Mutex
	child   *controlledDescribeChild
	root    string
	started bool
}

func (*controlledDescribeStarter) privateProviderWindowControlledBackend() {}
func (s *controlledDescribeStarter) Start(ctx context.Context, spec managedprocess.Spec) (Child, managedprocess.StartObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil || s.started || spec.Path != controlledDescribeProcess || spec.Dir != s.root || len(spec.Args) != 0 || len(spec.Env) != 0 {
		return nil, managedprocess.StartObservation{}
	}
	s.started = true
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

type controlledDescribeChild struct {
	input             *io.PipeWriter
	serverInput       *io.PipeReader
	output            *io.PipeReader
	serverOutput      *io.PipeWriter
	done              chan struct{}
	closing           chan struct{}
	uri               string
	stopOnce          sync.Once
	stopErr, serveErr error
	failCloseReport   bool
	closeReports      atomic.Uint64
	referenceReplies  atomic.Uint64
}

func (c *controlledDescribeChild) Stdin() io.WriteCloser { return c.input }
func (c *controlledDescribeChild) Stdout() io.ReadCloser { return c.output }
func (c *controlledDescribeChild) stop() {
	c.stopOnce.Do(func() {
		close(c.closing)
		c.stopErr = errors.Join(c.input.Close(), c.output.Close(), c.serverInput.Close(), c.serverOutput.Close())
	})
}
func (c *controlledDescribeChild) Close() managedprocess.ResourceObservation {
	c.stop()
	<-c.done
	c.closeReports.Add(1)
	if c.failCloseReport || errors.Join(c.stopErr, c.serveErr) != nil {
		return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesCloseFailed}
	}
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *controlledDescribeChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *controlledDescribeChild) serve() {
	defer close(c.done)
	defer func() { c.serveErr = errors.Join(c.serveErr, c.serverOutput.Close(), c.serverInput.Close()) }()
	limits := lspwire.DefaultLimits()
	limits.MaxHeaderBytes = 4096
	limits.MaxBodyBytes = 32 << 10
	reader := lspwire.NewReader(c.serverInput, limits)
	writer := lspwire.NewWriter(c.serverOutput, limits)
	for frames := 0; frames < 32; frames++ {
		msg, err := reader.Read()
		if err != nil {
			select {
			case <-c.closing:
				return
			default:
			}
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
				c.serveErr = err
			}
			return
		}
		if msg.Method == "exit" {
			return
		}
		if len(msg.ID) == 0 {
			switch msg.Method {
			case "initialized", "$/cancelRequest":
			case "textDocument/didOpen":
				var p struct{ TextDocument struct{ URI, Text string } }
				if json.Unmarshal(msg.Params, &p) != nil || p.TextDocument.URI != c.uri || p.TextDocument.Text != controlledDescribeDocument {
					c.serveErr = errors.New("fixed document mismatch")
					return
				}
			default:
				c.serveErr = errors.New("unknown fixture notification")
				return
			}
			continue
		}
		result := "[]"
		switch msg.Method {
		case "initialize":
			result = `{"serverInfo":{"name":"controlled-Describe-pipe","version":"fixture-1"},"capabilities":{"positionEncoding":"utf-16","documentSymbolProvider":true,"referencesProvider":true,"definitionProvider":true}}`
		case "textDocument/documentSymbol":
			result = `[{"name":"Query","kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":15}},"selectionRange":{"start":{"line":1,"character":5},"end":{"line":1,"character":10}}}]`
		case "textDocument/references":
			result = `[{"uri":` + strconv.Quote(c.uri) + `,"range":{"start":{"line":2,"character":10},"end":{"line":2,"character":15}}}]`
		case "textDocument/definition":
		case "shutdown":
			result = "null"
		default:
			c.serveErr = errors.New("unknown fixture request")
			return
		}
		if strings.HasPrefix(msg.Method, "textDocument/") {
			var p struct{ TextDocument struct{ URI string } }
			if json.Unmarshal(msg.Params, &p) != nil || p.TextDocument.URI != c.uri {
				c.serveErr = errors.New("fixed query URI mismatch")
				return
			}
		}
		if err = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(result)}); err != nil {
			select {
			case <-c.closing:
				return
			default:
			}
			if !errors.Is(err, io.ErrClosedPipe) {
				c.serveErr = err
			}
			return
		}
		if msg.Method == "textDocument/references" {
			c.referenceReplies.Add(1)
		}
	}
	c.serveErr = errors.New("fixture frame cap exceeded")
}
