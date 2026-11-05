package inventorysuccessor_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	methodresult "lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// The same io.Pipe Child/Starter contract used by the existing managed fixtures;
// no OS process, real provider, qualification or session acquisition is used.
type managedOperationPipe struct {
	in     *io.PipeReader
	stdin  *io.PipeWriter
	out    *io.PipeWriter
	stdout *io.PipeReader
}

func (c *managedOperationPipe) Stdin() io.WriteCloser { return c.stdin }
func (c *managedOperationPipe) Stdout() io.ReadCloser { return c.stdout }
func (c *managedOperationPipe) Teardown(context.Context) managedprocess.TeardownObservation {
	c.stdin.Close()
	c.in.Close()
	c.out.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *managedOperationPipe) Close() managedprocess.ResourceObservation {
	c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type managedOperationStarter struct{ child *managedOperationPipe }

func (s managedOperationStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}
func managedOperationFixture(t *testing.T, rawAttack ...string) (*sessionruntime.Manager, methodresult.PrivateManagedGroupingRequest, func() int, string) {
	t.Helper()
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(workspace, "query.go")
	if err = os.WriteFile(source, []byte("package fixture\nfunc Query() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: source}).String()
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	child := &managedOperationPipe{in, stdin, out, stdout}
	var mu sync.Mutex
	effects := 0
	go func() {
		reader := lspwire.NewReader(in, lspwire.DefaultLimits())
		for {
			msg, err := reader.Read()
			if err != nil {
				return
			}
			if msg.Method != "initialize" && msg.Method != "initialized" {
				mu.Lock()
				effects++
				mu.Unlock()
			}
			if len(msg.ID) == 0 {
				continue
			}
			result := `null`
			switch msg.Method {
			case "initialize":
				result = `{"serverInfo":{"name":"controlled-io-pipe","version":"fixture-v1"},"capabilities":{"positionEncoding":"utf-16","documentSymbolProvider":true,"referencesProvider":true,"definitionProvider":true}}`
			case "textDocument/documentSymbol":
				if len(rawAttack) != 0 {
					_, _ = io.WriteString(out, rawAttack[0])
					return
				}
				result = `[{"name":"Query","kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":15}},"selectionRange":{"start":{"line":1,"character":5},"end":{"line":1,"character":10}}}]`
			case "textDocument/references", "textDocument/definition":
				result = fmt.Sprintf(`[{"uri":%q,"range":{"start":{"line":1,"character":5},"end":{"line":1,"character":10}}}]`, uri)
			}
			body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, result)
			if _, err = fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
				return
			}
		}
	}()
	m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 4, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Starter: managedOperationStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Teardown(context.Background()); child.Close(); m.Shutdown(context.Background()) })
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile), LanguageID: "go"})
	if s.Failure != "" {
		t.Fatal(s.Failure)
	}
	pending := m.BeginReadiness(context.Background(), s.SessionID, s.Generation, time.Now().Add(time.Second))
	ready, found := m.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
		t.Fatal("controlled readiness", ready)
	}
	pubpath := t.TempDir()
	if err = os.Chmod(pubpath, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(pubpath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	q := methodresult.PrivateManagedGroupingRequest{Session: sessionruntime.SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: workspace}, URI: uri, OccurrenceID: "controlled", Revision: "declared", Privacy: "private", OccurrenceAdmissionPolicyID: "ordinal", CompositionPolicyID: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Root: root, Deadline: time.Now().Add(10 * time.Second)}
	return m, q, func() int { mu.Lock(); defer mu.Unlock(); return effects }, pubpath
}

func TestPrivateManagedInputResourceRawFrameV2(t *testing.T) {
	for _, attack := range []string{string(bytes.Repeat([]byte("x"), 8192)), "Content-Length: nope\r\n\r\n", "Content-Length: 4611686018427387904\r\n\r\n"} {
		m, q, effects, _ := managedOperationFixture(t, attack)
		owner, _ := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
		binding, err := methodresult.PrivateManagedGroupingOperationBinding(m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
		if err != nil {
			t.Fatal(err)
		}
		q.Operation, err = owner.Reserve(context.Background(), fmt.Sprintf("sha256:%064x", len(attack)), binding)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = methodresult.AcquirePrivateManagedGroupingInput(context.Background(), m, q); err == nil || effects() != 2 {
			t.Fatalf("ASSERT_RESOURCE_MANAGER_RAW_REJECT: err=%v effects=%d", err, effects())
		}
	}
}

func TestPrivateManagedInputResourceSourceBoundaryV2(t *testing.T) {
	for _, size := range []int{16 << 10, (16 << 10) + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m, q, effects, _ := managedOperationFixture(t)
			u, err := url.Parse(q.URI)
			if err != nil {
				t.Fatal(err)
			}
			text := make([]byte, size)
			copy(text, "package fixture\nfunc Query() {}\n")
			for i := len("package fixture\nfunc Query() {}\n"); i < len(text); i++ {
				text[i] = ' '
			}
			if err := os.WriteFile(u.Path, text, 0644); err != nil {
				t.Fatal(err)
			}
			owner, _ := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
			binding, err := methodresult.PrivateManagedGroupingOperationBinding(m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
			if err != nil {
				t.Fatal(err)
			}
			q.Operation, err = owner.Reserve(context.Background(), fmt.Sprintf("sha256:%064x", size), binding)
			if err != nil {
				t.Fatal(err)
			}
			_, err = methodresult.AcquirePrivateManagedGroupingInput(context.Background(), m, q)
			if size > 16<<10 {
				if err == nil || effects() != 0 {
					t.Fatalf("ASSERT_RESOURCE_SOURCE_BEFORE_SEND: err=%v effects=%d", err, effects())
				}
			} else if err != nil || effects() == 0 {
				t.Fatalf("ASSERT_RESOURCE_SOURCE_AT_CAP: err=%v effects=%d", err, effects())
			}
		})
	}
}

// These attacks intentionally use only the pre-repair public API. They must
// remain compiling guards when those manual entry points become refusals.
func TestPrivateManagedInputManualCompletionCannotObserveV2(t *testing.T) {
	for _, mode := range []string{"mismatch", "cancelled-consume", "cancel-after-consume", "manual-success", "copied-operation"} {
		t.Run(mode, func(t *testing.T) {
			m, q, effects, pubpath := managedOperationFixture(t)
			var old sessionruntime.PrivateGroupingOwnedHandoff
			if err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
				var err error
				old, err = m.CapturePrivateGroupingOwned(ctx, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, Line: q.Line, Character: q.Character, References: true, Definitions: true, Deadline: q.Deadline})
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if old == (sessionruntime.PrivateGroupingOwnedHandoff{}) {
				t.Fatal("CONTROLLED_OLD_OPAQUE_HANDOFF_MISSING")
			}
			owner, accounting := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
			b, err := methodresult.PrivateManagedGroupingOperationBinding(m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
			if err != nil {
				t.Fatal(err)
			}
			p, err := owner.Reserve(context.Background(), fmt.Sprintf("sha256:%064x", 11), b)
			if err != nil {
				t.Fatal(err)
			}
			copy := p
			candidate := b
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "mismatch" {
				candidate.Character++
			}
			if mode == "cancelled-consume" {
				cancel()
			}
			returned, stop, consumeErr := p.Consume(ctx, candidate)
			if stop != nil {
				defer stop()
			}
			_ = returned
			if mode == "cancel-after-consume" {
				cancel()
			}
			if mode == "copied-operation" {
				p = copy
			}
			before := effects()
			bindErr := p.BindOwned(old, fmt.Sprintf("sha256:%064x", 99))
			completeErr := p.Complete(context.Background())
			raw, err := accounting.ReadSelector("inventory-successor/heads/private/v2/current/975a8adf2993cf97097506237528078940e20a0af849c6bd7d8725a1ad4eb0c0.json", 64<<10)
			if err != nil {
				t.Fatal(err)
			}
			var head struct {
				Job         json.RawMessage   `json:"job"`
				Identity    string            `json:"identity"`
				Windows     uint64            `json:"consumed_windows"`
				Disposition string            `json:"disposition"`
				Charged     map[string]uint64 `json:"cumulative_charged"`
			}
			if err = json.Unmarshal(raw, &head); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(pubpath)
			if err != nil {
				t.Fatal(err)
			}
			if head.Disposition != "PENDING" || head.Windows != 1 || head.Charged["calls"] == 0 || effects() != before || len(entries) != 0 {
				t.Fatalf("ASSERT_MANUAL_COMPLETION_CANNOT_OBSERVE: mode=%s consume=%v bind=%v complete=%v disposition=%s windows=%d effects=%d/%d publications=%d", mode, consumeErr, bindErr, completeErr, head.Disposition, head.Windows, before, effects(), len(entries))
			}
			if _, err = owner.Reserve(context.Background(), fmt.Sprintf("sha256:%064x", 12), b); err == nil {
				t.Fatal("ASSERT_FAILED_OPERATION_CANNOT_RESERVE_AGAIN")
			}
			if _, err = inventorysuccessor.OpenPrivateManagedInputOwnerV2(context.Background(), accounting, append(head.Job, '\n'), head.Identity, inventorysuccessor.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}); err == nil {
				t.Fatal("ASSERT_FAILED_OPERATION_REOPEN_STICKY")
			}
			t.Log("ASSERT_MANUAL_COMPLETION_CANNOT_OBSERVE: PASS", mode)
		})
	}
}

func TestPrivateManagedInputFailedFixedPipelineStaysPendingV2(t *testing.T) {
	m, q, effects, _ := managedOperationFixture(t)
	owner, accounting := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
	b, err := methodresult.PrivateManagedGroupingOperationBinding(m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
	if err != nil {
		t.Fatal(err)
	}
	q.Operation, err = owner.Reserve(context.Background(), fmt.Sprintf("sha256:%064x", 51), b)
	if err != nil {
		t.Fatal(err)
	}
	q.Root.Close() // fail publication AFTER actual bounded capture/admission
	got, runErr := methodresult.AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	raw, err := accounting.ReadSelector("inventory-successor/heads/private/v2/current/975a8adf2993cf97097506237528078940e20a0af849c6bd7d8725a1ad4eb0c0.json", 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	var h struct {
		Identity    string            `json:"identity"`
		Job         json.RawMessage   `json:"job"`
		Disposition string            `json:"disposition"`
		Windows     uint64            `json:"consumed_windows"`
		Charged     map[string]uint64 `json:"cumulative_charged"`
	}
	if err = json.Unmarshal(raw, &h); err != nil {
		t.Fatal(err)
	}
	if runErr == nil || got.Input.ByteLength() != 0 || h.Disposition != "PENDING" || h.Windows != 1 || h.Charged["calls"] == 0 || effects() != 4 {
		t.Fatalf("ASSERT_FAILED_FIXED_PIPELINE_CANNOT_OBSERVE: err=%v input=%d disposition=%s windows=%d effects=%d", runErr, got.Input.ByteLength(), h.Disposition, h.Windows, effects())
	}
	// A fresh context/manual helper and reopening cannot turn failure into progress.
	_ = q.Operation.Complete(context.Background())
	if _, err = inventorysuccessor.OpenPrivateManagedInputOwnerV2(context.Background(), accounting, append(h.Job, '\n'), h.Identity, inventorysuccessor.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}); err == nil {
		t.Fatal("ASSERT_FAILED_FIXED_PIPELINE_REOPEN_STICKY")
	}
	if _, err = owner.Reserve(context.Background(), fmt.Sprintf("sha256:%064x", 52), b); err == nil {
		t.Fatal("ASSERT_FAILED_FIXED_PIPELINE_NO_NEXT_RESERVATION")
	}
	t.Log("ASSERT_FAILED_FIXED_PIPELINE_CANNOT_OBSERVE: PASS")
}

func TestPrivateManagedInputActualManagerCommitBoundaryV2(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(fmt.Sprint(uncertain), func(t *testing.T) {
			m, q, effects, pubpath := managedOperationFixture(t)
			owner, accounting := inventorysuccessor.ControlledManagedInputOwnerForTest(t, uncertain)
			b, err := methodresult.PrivateManagedGroupingOperationBinding(m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
			if err != nil {
				t.Fatal(err)
			}
			q.Operation, err = owner.Reserve(context.Background(), fmt.Sprintf("sha256:%064x", 1), b)
			reserveErr := err
			got, err := methodresult.AcquirePrivateManagedGroupingInput(context.Background(), m, q)
			entries, e := os.ReadDir(pubpath)
			if e != nil {
				t.Fatal(e)
			}
			if uncertain {
				if reserveErr == nil || err == nil || effects() != 0 || len(entries) != 0 || got.Input.ByteLength() != 0 {
					t.Fatalf("ASSERT_COMMITTED_ERROR_ZERO_ACTUAL_DOWNSTREAM: reserve=%v entry=%v effects=%d publications=%d", reserveErr, err, effects(), len(entries))
				}
			} else if reserveErr != nil || err != nil || effects() != 4 || len(got.Input.Contributions()) != 2 {
				t.Fatalf("ASSERT_CONFIRMED_ACTUAL_MANAGER_POSITIVE: reserve=%v entry=%v effects=%d", reserveErr, err, effects())
			}
			raw, e := accounting.ReadSelector("inventory-successor/heads/private/v2/current/975a8adf2993cf97097506237528078940e20a0af849c6bd7d8725a1ad4eb0c0.json", 64<<10)
			if e != nil {
				t.Fatal(e)
			}
			var h struct {
				Windows     uint64            `json:"consumed_windows"`
				Charged     map[string]uint64 `json:"cumulative_charged"`
				Disposition string            `json:"disposition"`
			}
			if json.Unmarshal(raw, &h) != nil || h.Windows != 1 || h.Charged["calls"] == 0 {
				t.Fatal("ASSERT_ACTUAL_MANAGER_CHARGED_ONCE")
			}
			if uncertain && h.Disposition != "PENDING" {
				t.Fatal("ASSERT_COMMITTED_ERROR_PENDING_RETAINED")
			}
			if uncertain {
				t.Log("ASSERT_COMMITTED_ERROR_ZERO_ACTUAL_DOWNSTREAM: PASS")
			} else {
				t.Log("ASSERT_CONFIRMED_ACTUAL_MANAGER_POSITIVE: PASS")
			}
		})
	}
}
