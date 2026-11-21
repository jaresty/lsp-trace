//go:build lsp_trace_controlled_describe_fixture

package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
)

// ControlledInventoryPipeFixture is fixed, build-selected test substrate, not
// funding, source admission, or production-provider coverage. The constructor
// owns exactly 17 literal Go documents. Its two one-shot Starter factories are
// invoked by the discovery runner, never eagerly by Construction. No OS process
// is started; Process.Path is matched only by controlledDescribeStarter.
//
// The source contains both literal crossbatch references: Query01 is used in
// document 17 and Query17 in document 1. Responses are fixed from these literals,
// not supplied scripts. Definition and CALLS are unrequested, not observed empty.
// This does not implement the inventory-parent CAS funding bridge.
type ControlledInventoryPipeFixture struct {
	mu               sync.Mutex
	root             *os.Root
	path             string
	profile          runtimeprofile.Profile
	documents        [17]controlledInventoryDocument
	children         [2]*controlledDescribeChild
	started          [2]bool
	stopping, closed bool
	closeErr         error
}

type controlledInventoryDocument struct {
	name, uri, text, symbol, reference string
	literal                            os.FileInfo
}

type controlledInventoryProtocol struct {
	documents [17]controlledInventoryDocument
	opened    map[string]bool // private to the sole protocol goroutine
}

// NewControlledInventoryPipeFixture accepts only an empty, canonical, private
// temporary child directory. No caller-provided document or protocol is accepted.
func NewControlledInventoryPipeFixture(path string) (*ControlledInventoryPipeFixture, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("canonical temporary root required")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || canonical != path {
		return nil, errors.New("canonical temporary root required")
	}
	temp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(temp, path)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return nil, errors.New("private temporary child required")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, errors.New("private directory required")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	f := &ControlledInventoryPipeFixture{root: root, path: path}
	fail := func(cause error) (*ControlledInventoryPipeFixture, error) {
		for _, d := range f.documents {
			if d.literal != nil {
				cause = errors.Join(cause, root.Remove(d.name))
			}
		}
		return nil, errors.Join(cause, root.Close())
	}
	pinned, err := root.Stat(".")
	if err != nil || !os.SameFile(info, pinned) {
		return fail(errors.New("root identity changed"))
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		return fail(errors.New("empty fixture directory required"))
	}
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: path, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		return fail(err)
	}
	f.profile = runtimeprofile.Resolve(selector)
	for i := range f.documents {
		name := fmt.Sprintf("query-%02d.go", i+1)
		symbol := fmt.Sprintf("Query%02d", i+1)
		text := "package fixture\nfunc " + symbol + "() {}\n"
		if i == 0 {
			text += "var use17 = Query17\n"
		}
		if i == 16 {
			text += "var use01 = Query01\n"
		}
		file, e := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return fail(e)
		}
		_, writeErr := io.WriteString(file, text)
		literal, statErr := file.Stat()
		closeErr := file.Close()
		f.documents[i] = controlledInventoryDocument{name: name, uri: (&url.URL{Scheme: "file", Path: filepath.Join(path, name)}).String(), text: text, symbol: symbol, literal: literal}
		if e = errors.Join(writeErr, statErr, closeErr); e != nil {
			return fail(e)
		}
	}
	f.documents[0].reference = f.documents[16].uri
	f.documents[16].reference = f.documents[0].uri
	return f, nil
}

// Construction returns detached metadata and a one-shot factory. These ceilings
// retain the existing discovery enclosure; they are declarations, NOT authority
// or a complete end-to-end resource profile. A funded owner must reserve first.
func (f *ControlledInventoryPipeFixture) Construction(ordinal int, deadline time.Time) (PrivateDiscoveryConstruction, error) {
	if f == nil || ordinal < 0 || ordinal > 1 || !time.Now().Before(deadline) || time.Until(deadline) > 15*time.Second {
		return PrivateDiscoveryConstruction{}, ErrPrivateDiscovery
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopping || f.started[ordinal] {
		return PrivateDiscoveryConstruction{}, ErrPrivateDiscovery
	}
	first, last := 0, 16
	if ordinal == 1 {
		first, last = 16, 17
	}
	members := make([]PrivateDiscoveryMember, 0, last-first)
	for _, d := range f.documents[first:last] {
		members = append(members, PrivateDiscoveryMember{Path: filepath.Join(f.path, d.name), URI: d.uri, LanguageID: "go"})
	}
	limits := PrivateDiscoveryLimits{SourceBytes: 16 * (1<<20 + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}
	start := StartRequest{Profile: f.profile, Process: managedprocess.Spec{Path: controlledDescribeProcess, Dir: f.path}, LanguageID: "go", Deadline: deadline}
	factory := func() (Starter, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.stopping || f.started[ordinal] || !time.Now().Before(deadline) {
			return nil, ErrPrivateDiscovery
		}
		f.started[ordinal] = true
		inR, inW := io.Pipe()
		outR, outW := io.Pipe()
		child := &controlledDescribeChild{input: inW, serverInput: inR, output: outR, serverOutput: outW, done: make(chan struct{}), closing: make(chan struct{}), inventory: &controlledInventoryProtocol{documents: f.documents, opened: make(map[string]bool, 16)}}
		f.children[ordinal] = child
		go child.serve()
		return &controlledDescribeStarter{child: child, root: f.path}, nil
	}
	return PrivateDiscoveryConstruction{CaptureOwnedMethods: true, NewStarter: factory, Start: start, Limits: limits, Deadline: deadline, Members: members}, nil
}

func (p *controlledInventoryProtocol) supply(uri, text string) bool {
	for _, d := range p.documents {
		if d.uri == uri && d.text == text && !p.opened[uri] {
			p.opened[uri] = true
			return true
		}
	}
	return false
}
func (p *controlledInventoryProtocol) response(msg lspwire.Message) (string, error) {
	var q struct {
		TextDocument struct{ URI string }
		Position     struct{ Line, Character uint32 }
	}
	if json.Unmarshal(msg.Params, &q) != nil || !p.opened[q.TextDocument.URI] {
		return "", errors.New("fixed supplied query required")
	}
	for _, d := range p.documents {
		if d.uri != q.TextDocument.URI {
			continue
		}
		switch msg.Method {
		case "textDocument/documentSymbol":
			return `[{"name":` + strconv.Quote(d.symbol) + `,"kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":17}},"selectionRange":{"start":{"line":1,"character":5},"end":{"line":1,"character":12}}}]`, nil
		case "textDocument/references":
			if q.Position.Line != 1 || q.Position.Character != 5 {
				return "", errors.New("fixed declaration query required")
			}
			if d.reference == "" {
				return "[]", nil
			}
			return `[{"uri":` + strconv.Quote(d.reference) + `,"range":{"start":{"line":2,"character":12},"end":{"line":2,"character":19}}}]`, nil
		default:
			return "", errors.New("R-only fixture; method unrequested")
		}
	}
	return "", errors.New("fixed query URI mismatch")
}

// Close interrupts and joins both actually-created pipe children, then removes
// only constructor-owned literals whose inode identity still matches. It never
// rewrites native or parent completion. A canceled join may be retried solely
// for cleanup; stopping permanently prevents new Starter creation.
func (f *ControlledInventoryPipeFixture) Close(ctx context.Context) error {
	if f == nil || ctx == nil {
		return errors.New("fixture cleanup context required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return f.closeErr
	}
	f.stopping = true
	for _, c := range f.children {
		if c != nil {
			c.stop()
		}
	}
	var err error
	for _, c := range f.children {
		if c == nil {
			continue
		}
		select {
		case <-c.done:
		case <-ctx.Done():
			return ctx.Err()
		}
		err = errors.Join(err, c.stopErr, c.serveErr)
	}
	for _, d := range f.documents {
		current, e := f.root.Lstat(d.name)
		if e == nil && !os.SameFile(d.literal, current) {
			e = errors.New("fixture literal replaced; refusing removal")
		}
		if e == nil {
			e = f.root.Remove(d.name)
		}
		err = errors.Join(err, e)
	}
	f.closeErr = errors.Join(err, f.root.Close(), ctx.Err())
	f.closed = true
	return f.closeErr
}
