package inventorysuccessor_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// No marker is minted here: the actual Manager envelope selects ManagedStarter
// and the current test executable. The fixture completes the required initial
// documentSymbol schedule before the original BASE lease runs the constructor.
func nativeWindowFixtureV2(t *testing.T, asymmetric bool, duration time.Duration) (*sessionruntime.Manager, inventorysuccessor.PrivateNativeCallsRequestV2, context.Context, context.CancelFunc) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("controlled SELF requires local Darwin; Linux compile-only")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"root.go", "caller.go", "callee.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package fixture\nfunc Root(){ Root() }\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 65536, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 8, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	start := sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go", Deadline: time.Now().Add(duration), Process: managedprocess.Spec{Path: exe, Dir: root, Args: []string{"-test.run=^TestPrivateNativeConstructorSELFResponderV2$", "--", "native-constructor-self-v2", root, fmt.Sprint(asymmetric)}}}
	envelope, err := m.PrivateProviderWindowEnvelope(start, sessionruntime.PrivateProviderWindowPlan{Calls: true})
	if err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithCancel(context.Background())
	ctx, err := m.PrivateProviderWindowContext(parent, envelope, start.Deadline)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_, _ = m.FinishPrivateProviderWindow(ctx)
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		m.Shutdown(cleanup)
	})
	s := m.Start(ctx, start)
	if s.Failure != "" {
		t.Fatalf("window start: %s", s.Failure)
	}
	pending := m.BeginReadiness(ctx, s.SessionID, s.Generation, start.Deadline)
	ready, ok := m.WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("window readiness: %+v", ready)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "root.go")}).String()
	d := m.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: s.SessionID, Generation: s.Generation, URI: uri, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
	if d.Failure != "" {
		t.Fatalf("window initial prepare: %s", d.Failure)
	}
	symbols := m.RoundTrip(ctx, sessionruntime.RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/documentSymbol", Params: []byte(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 65536})
	if symbols.Failure != "" || symbols.ServerError != nil {
		t.Fatalf("window documentSymbol: %+v", symbols)
	}
	line, ch := uint32(1), uint32(6)
	q := inventorysuccessor.PrivateNativeCallsRequestV2{Session: sessionruntime.SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: root}, Directional: sessionruntime.DirectionalCallHierarchyRequestPrivate{SessionID: s.SessionID, Generation: s.Generation, URI: uri, LanguageID: "go", Line: &line, Character: &ch, Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 65536}, Revision: "controlled-window-revision", Privacy: "private-constructor-only"}
	return m, q, ctx, cancel
}

func TestPrivateNativeConstructorWindowJoinPositive(t *testing.T) {
	for _, asym := range []bool{false, true} {
		t.Run(fmt.Sprint(asym), func(t *testing.T) {
			m, q, ctx, _ := nativeWindowFixtureV2(t, asym, 15*time.Second)
			var got inventorysuccessor.PrivateNativeCallsResultV2
			err := m.WithReadySessionLease(ctx, q.Session, func(base context.Context) error {
				var e error
				got, e = inventorysuccessor.ConstructNativeCallsPrivateV2(base, m, q)
				return e
			})
			want := 1
			if asym {
				want = 2
			}
			uris := map[string]bool{}
			for _, v := range got.Views {
				for _, n := range v.Result.Graph.Nodes {
					uris[n.URI] = true
				}
			}
			wantURIs := 1
			if asym {
				wantURIs = 3
			}
			if err != nil || !got.Admission.Admission.Valid() || len(got.RawJoins) != 4 || len(got.Admission.Admission.Occurrences()) != want || len(uris) != wantURIs {
				t.Fatalf("ASSERT_WINDOW_NATIVE_TOPOLOGY_%v: err=%v raw=%d canonical=%d uris=%d", asym, err, len(got.RawJoins), len(got.Admission.Admission.Occurrences()), len(uris))
			}
			for _, o := range got.Admission.Admission.Occurrences() {
				if o.Weight != 1 {
					t.Fatalf("ASSERT_WINDOW_NATIVE_TOPOLOGY_%v: weight=%v", asym, o.Weight)
				}
			}
			t.Logf("ASSERT_WINDOW_NATIVE_TOPOLOGY_%v: PASS", asym)
			completion, e := m.FinishPrivateProviderWindow(ctx)
			if e != nil || !completion.Completed || m.Census().Workers != 0 {
				t.Fatalf("ASSERT_WINDOW_NATIVE_FINISH_%v: completion=%+v err=%v census=%+v", asym, completion, e, m.Census())
			}
			t.Logf("ASSERT_WINDOW_NATIVE_FINISH_%v: PASS", asym)
		})
	}
}

// This context is test-only observation apparatus, not a production permission
// or work hook. It pauses an actual ordinary CaptureV2 context check after TAKE,
// before reading source. Values, cancellation and original deadline are relayed.
// A stack match is checked at entry so a test cannot accidentally stall readiness.
type nativeSourcePauseV2 struct {
	context.Context
	entered, release chan struct{}
	once             sync.Once
}

func (p *nativeSourcePauseV2) Err() error {
	pcs := make([]uintptr, 24)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if strings.Contains(f.Function, "graphprovenance.CaptureV2") {
			p.once.Do(func() { close(p.entered); <-p.release })
			break
		}
		if !more {
			break
		}
	}
	return p.Context.Err()
}

func TestPrivateNativeConstructorWindowJoinTrackedWork(t *testing.T) {
	for _, mode := range []string{"concurrent", "cancelled", "expired"} {
		t.Run(mode, func(t *testing.T) {
			duration := 15 * time.Second
			if mode == "expired" {
				duration = 2 * time.Second
			}
			m, q, ctx, cancel := nativeWindowFixtureV2(t, false, duration)
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			var base context.Context
			done := make(chan error, 1)
			actual := make(chan error, 1)
			go func() {
				done <- m.WithReadySessionLease(ctx, q.Session, func(lease context.Context) error {
					base = lease
					got, e := inventorysuccessor.ConstructNativeCallsPrivateV2(&nativeSourcePauseV2{Context: lease, entered: entered, release: release}, m, q)
					if e != nil && got.Admission.Admission.Valid() {
						actual <- fmt.Errorf("report published on failure")
						return e
					}
					actual <- e
					return e
				})
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("ASSERT_WINDOW_ACTUAL_SOURCE_ENTERED")
			}
			if mode == "concurrent" {
				d := m.PrepareDocument(base, sessionruntime.DocumentRequest{SessionID: q.Session.SessionID, Generation: q.Session.Generation, URI: q.Directional.URI, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
				r := m.RoundTrip(base, sessionruntime.RoundTripRequest{SessionID: q.Session.SessionID, Generation: q.Session.Generation, Method: "textDocument/documentSymbol", Params: []byte(`{}`), Deadline: q.Directional.Deadline, MaxMessages: 4, MaxBytes: 65536})
				if d.Failure == "" || r.Failure == "" {
					t.Fatalf("ASSERT_WINDOW_SAME_LEASE_EFFECT_REFUSED: prepare=%s wire=%s", d.Failure, r.Failure)
				}
				t.Log("ASSERT_WINDOW_SAME_LEASE_EFFECT_REFUSED: PASS")
				unblock()
				if e := <-actual; e == nil {
					t.Fatal("ASSERT_WINDOW_CONCURRENT_INVALIDATION_REFUSED")
				}
				if e := <-done; e == nil {
					t.Fatal("ASSERT_WINDOW_CONCURRENT_INVALIDATION_REFUSED")
				}
				c, e := m.FinishPrivateProviderWindow(ctx)
				if e == nil || c.Completed {
					t.Fatalf("ASSERT_WINDOW_CONCURRENT_INVALIDATION_REFUSED: %+v %v", c, e)
				}
				t.Log("ASSERT_WINDOW_CONCURRENT_INVALIDATION_REFUSED: PASS")
				return
			}
			if mode == "cancelled" {
				cancel()
			} else {
				<-ctx.Done()
			}
			if e := <-done; e == nil {
				t.Fatal("ASSERT_WINDOW_EXPIRED_CALLER_REFUSED")
			}
			select {
			case <-actual:
				t.Fatal("ASSERT_WINDOW_ACTUAL_WORK_RETAINED: actual returned before release")
			default:
			}
			if m.Census().Workers < 2 {
				t.Fatalf("ASSERT_WINDOW_ACTUAL_WORK_RETAINED: %+v", m.Census())
			}
			t.Logf("ASSERT_WINDOW_ACTUAL_WORK_RETAINED_%s: PASS workers=%d", mode, m.Census().Workers)
			c, e := m.FinishPrivateProviderWindow(ctx)
			if e == nil || c.Completed {
				t.Fatal("ASSERT_WINDOW_EXPIRED_FINISH_REFUSED")
			}
			t.Logf("ASSERT_WINDOW_EXPIRED_FINISH_REFUSED_%s: PASS", mode)
			unblock()
			if e := <-actual; e == nil {
				t.Fatal("ASSERT_WINDOW_NO_LATE_ADMISSION")
			}
			t.Logf("ASSERT_WINDOW_NO_LATE_ADMISSION_%s: PASS", mode)
			joinDeadline := time.NewTimer(2 * time.Second)
			defer joinDeadline.Stop()
			joined := time.NewTicker(time.Millisecond)
			defer joined.Stop()
			for m.Census().Workers >= 2 {
				select {
				case <-joined.C:
				case <-joinDeadline.C:
					t.Fatalf("ASSERT_WINDOW_ACTUAL_WORK_JOINED_%s: %+v", mode, m.Census())
				}
			}
			t.Logf("ASSERT_WINDOW_ACTUAL_WORK_JOINED_%s: PASS", mode)
			// The failed window may retain FAILEDUNJOINED cleanup resources; do not
			// misrepresent failure as successful terminal closure. Actual fixed work did join.
		})
	}
}

func TestPrivateNativeConstructorWindowJoinSurface(t *testing.T) {
	// The public surface itself is the requirement: no function accepted by TAKE
	// or an exported Consume replacement. This is intentionally a structural guard.
	typ := reflect.TypeOf((*sessionruntime.Manager)(nil))
	if _, ok := typ.MethodByName("ConsumeDirectionalEvidencePrivateV2"); ok {
		t.Fatal("ASSERT_WINDOW_NO_CALLBACK_SURFACE")
	}
	take, ok := typ.MethodByName("TakeDirectionalEvidencePrivateV2")
	if !ok {
		t.Fatal("ASSERT_WINDOW_NO_CALLBACK_SURFACE")
	}
	for i := 0; i < take.Type.NumIn(); i++ {
		if take.Type.In(i).Kind() == reflect.Func {
			t.Fatal("ASSERT_WINDOW_NO_CALLBACK_SURFACE")
		}
	}
	// Also reject a function hidden inside a newly introduced TAKE input DTO.
	for i := 1; i < take.Type.NumIn(); i++ {
		if take.Type.In(i).Kind() == reflect.Struct {
			for j := 0; j < take.Type.In(i).NumField(); j++ {
				if take.Type.In(i).Field(j).Type.Kind() == reflect.Func {
					t.Fatal("ASSERT_WINDOW_NO_CALLBACK_SURFACE")
				}
			}
		}
	}
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "sessionruntime", "private_directional_evidence_v2.go")
	f, e := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && ast.IsExported(fn.Name.Name) {
			for _, field := range fn.Type.Params.List {
				if _, ok := field.Type.(*ast.FuncType); ok {
					t.Fatal("ASSERT_WINDOW_NO_CALLBACK_SURFACE")
				}
			}
		}
	}
	t.Log("ASSERT_WINDOW_NO_CALLBACK_SURFACE: PASS")
}
