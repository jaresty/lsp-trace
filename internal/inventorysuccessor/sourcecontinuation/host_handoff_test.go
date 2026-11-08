//go:build darwin || linux

package sourcecontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	cc "lsp-trace/internal/censuscontinuation"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/describeworker"
	host "lsp-trace/internal/privatedescribeprep"
)

type sourceHostEmbeddedStore struct{ *RootlessSourceHostStore }

func TestSourceHostStoreClosed(t *testing.T) {
	store, err := NewRootlessSourceHostStore(8)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int64{0, -1, cc.MaxCaptureBytes + 1} {
		if _, err := NewRootlessSourceHostStore(size); err == nil {
			t.Fatal("ASSERT_HOST_MEMORY_FACTORY_BOUND")
		}
	}
	ctx := context.Background()
	input := []byte("original")
	id, err := store.Put(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = 'X'
	got, err := store.Get(ctx, id)
	if err != nil || !bytes.Equal(got, []byte("original")) {
		t.Fatal("ASSERT_HOST_MEMORY_INPUT_CLONE", err)
	}
	got[0] = 'X'
	got, err = store.Get(ctx, id)
	if err != nil || !bytes.Equal(got, []byte("original")) {
		t.Fatal("ASSERT_HOST_MEMORY_OUTPUT_CLONE", err)
	}
	if _, err := store.Put(ctx, []byte("oversized")); err == nil {
		t.Fatal("ASSERT_HOST_MEMORY_OBJECT_BOUND")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	gets, puts := store.OperationCounts()
	if _, err := store.Get(cancelled, id); err == nil {
		t.Fatal("ASSERT_HOST_MEMORY_CANCEL")
	}
	if _, err := store.Put(cancelled, []byte("x")); err == nil {
		t.Fatal("ASSERT_HOST_MEMORY_CANCEL")
	}
	g, p := store.OperationCounts()
	if gets != g || puts != p {
		t.Fatal("ASSERT_HOST_MEMORY_CANCEL_NO_IO")
	}
	var nilStore *RootlessSourceHostStore
	for _, foreign := range []cc.Store{nilStore, &RootlessSourceHostStore{}, cc.NewMemoryStore(), &sourceHostEmbeddedStore{store}} {
		_, _, err := sourceHostPreflight(ctx, host.SourceHostConfig{}, SourceAcquisitionRequest{Resume: ResumeRequest{Store: foreign}}, describeworker.SuppliedResponseLimits{})
		if err == nil || err.Error() != "source host requires exact rootless process-local memory store; filesystem stores unsupported" {
			t.Fatal("ASSERT_HOST_MEMORY_EXACT_TYPE", err)
		}
	}
	t.Log("ASSERT_HOST_MEMORY_FACTORY_BOUND: PASS; ASSERT_HOST_MEMORY_INPUT_CLONE: PASS; ASSERT_HOST_MEMORY_OUTPUT_CLONE: PASS; ASSERT_HOST_MEMORY_OBJECT_BOUND: PASS; ASSERT_HOST_MEMORY_CANCEL: PASS; ASSERT_HOST_MEMORY_CANCEL_NO_IO: PASS; ASSERT_HOST_MEMORY_EXACT_TYPE: PASS")
}

// Explicit caller deadline with delayed Err observation; no mutable clock.
type sourceHostDeadlineContext struct {
	context.Context
	deadline time.Time
}

func (c sourceHostDeadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }
func TestSourceHostEligibilityDeadline(t *testing.T) {
	now := time.Now()
	past, future := now.Add(-time.Second), now.Add(time.Second)
	ctx := context.Background()
	if !sourceHostPublicationLive(ctx, future) || sourceHostEligibilityLive(ctx, future, past) {
		t.Fatal("ASSERT_SOURCE_HOST_MAILBOX_EARLIER_DEADLINE")
	}
	if sourceHostEligibilityLive(ctx, past, future) || !sourceHostEligibilityLive(ctx, future, future) {
		t.Fatal("ASSERT_SOURCE_HOST_ELIGIBLE_MINIMUM")
	}
	if sourceHostEligibilityLive(sourceHostDeadlineContext{ctx, past}, future, future) {
		t.Fatal("ASSERT_SOURCE_HOST_ELIGIBLE_CALLER_DEADLINE")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if sourceHostEligibilityLive(cancelled, future, future) || sourceHostEligibilityLive(nil, future, future) || sourceHostEligibilityLive(ctx, future, time.Time{}) {
		t.Fatal("ASSERT_SOURCE_HOST_ELIGIBLE_CONTEXT")
	}
	t.Log("ASSERT_SOURCE_HOST_MAILBOX_EARLIER_DEADLINE: PASS; ASSERT_SOURCE_HOST_ELIGIBLE_MINIMUM: PASS; ASSERT_SOURCE_HOST_ELIGIBLE_CALLER_DEADLINE: PASS; ASSERT_SOURCE_HOST_ELIGIBLE_CONTEXT: PASS")
	_, testFile, _, _ := runtime.Caller(0)
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, strings.TrimSuffix(testFile, "_test.go")+".go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	text := func(node ast.Node) string {
		var b bytes.Buffer
		if node != nil {
			_ = format.Node(&b, set, node)
		}
		return b.String()
	}
	bound := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "PrepareSourceHost" || len(fn.Body.List) < 3 {
			continue
		}
		body := fn.Body.List
		guard, ok := body[len(body)-2].(*ast.IfStmt)
		if !ok {
			continue
		}
		bound = text(guard.Cond) == "!sourceHostEligibilityLive(ctx, s.request.Deadline, mailbox.Deadline())" && len(guard.Body.List) == 2 && text(guard.Body.List[0]) == "mailbox.Close()" && strings.HasPrefix(text(guard.Body.List[1]), "return SourceHostHandoff{},") && text(body[len(body)-3]) == "q.Resume.PrivateSources = &resume" && strings.HasPrefix(text(body[len(body)-1]), "return SourceHostHandoff{&sourceHostState{") && strings.Count(text(fn.Body), "host.NewSourceHostMailbox(") == 1
	}
	if !bound {
		t.Fatal("ASSERT_SOURCE_HOST_MAILBOX_EARLIER_DEADLINE_BOUND")
	}
	t.Log("ASSERT_SOURCE_HOST_MAILBOX_EARLIER_DEADLINE_BOUND: PASS")
}

func TestSourceHostUpperOpaque(t *testing.T) {
	var h SourceHostHandoff
	if e := json.Unmarshal([]byte(`{"state":{"used":false},"Eligible":true}`), &h); e != nil {
		t.Fatal(e)
	}
	if h.Eligible() {
		t.Fatal("ASSERT_SOURCE_HOST_NO_PUBLIC_OWNER")
	}
	if r, e := CompleteSourceHost(context.Background(), h); e == nil || r.Supplied().ID() != "" || r.Delivery().Delivered() {
		t.Fatal("ASSERT_SOURCE_HOST_NO_PUBLIC_OWNER", e)
	}
	if _, e := PrepareSourceHost(context.Background(), host.SourceHostConfig{}, SourceAcquisitionRequest{}, describeworker.SuppliedResponseLimits{}); e == nil {
		t.Fatal("ASSERT_SOURCE_HOST_EXPECTED_REQUIRED")
	}
	t.Log("ASSERT_SOURCE_HOST_NO_PUBLIC_OWNER: PASS; ASSERT_SOURCE_HOST_EXPECTED_REQUIRED: PASS")
}
func TestSourceHostUpperDeadline(t *testing.T) {
	if sourceHostPublicationLive(context.Background(), time.Now().Add(-time.Second)) {
		t.Fatal("ASSERT_SOURCE_HOST_ORIGINAL_DEADLINE")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sourceHostPublicationLive(ctx, time.Now().Add(time.Second)) {
		t.Fatal("ASSERT_SOURCE_HOST_CANCEL")
	}
	if !sourceHostPublicationLive(context.Background(), time.Now().Add(time.Second)) {
		t.Fatal("ASSERT_SOURCE_HOST_LIVE_CONTROL")
	}
	t.Log("ASSERT_SOURCE_HOST_ORIGINAL_DEADLINE: PASS; ASSERT_SOURCE_HOST_CANCEL: PASS; ASSERT_SOURCE_HOST_LIVE_CONTROL: PASS")
}
