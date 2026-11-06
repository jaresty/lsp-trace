package inventorysuccessor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

func nativeConstructorFixtureV2(t *testing.T, asymmetric bool, supervised ...bool) (*sessionruntime.Manager, inventorysuccessor.PrivateNativeCallsRequestV2, func() []string) {
	t.Helper()
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	uris := make([]string, 3)
	for i, name := range []string{"root.go", "caller.go", "callee.go"} {
		path := filepath.Join(workspace, name)
		if err = os.WriteFile(path, []byte("package fixture\nfunc Root(){ Root() }\n"), 0644); err != nil {
			t.Fatal(err)
		}
		uris[i] = (&url.URL{Scheme: "file", Path: path}).String()
	}
	item := func(uri, name string) lsp.CallHierarchyItem {
		return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: uri, Range: lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 21}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 9}}}
	}
	root := item(uris[0], "Root")
	caller, callee := root, root
	if asymmetric {
		caller = item(uris[1], "Caller")
		callee = item(uris[2], "Callee")
	}
	site := lsp.Range{Start: lsp.Position{Line: 1, Character: 13}, End: lsp.Position{Line: 1, Character: 17}}
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	child := &managedOperationPipe{in, stdin, out, stdout}
	var mu sync.Mutex
	methods := []string{}
	go func() {
		reader := lspwire.NewReader(in, lspwire.DefaultLimits())
		for {
			msg, e := reader.Read()
			if e != nil {
				return
			}
			if len(msg.ID) == 0 {
				continue
			}
			mu.Lock()
			methods = append(methods, msg.Method)
			mu.Unlock()
			var result any
			switch msg.Method {
			case "initialize":
				result = map[string]any{"serverInfo": map[string]string{"name": "controlled-io-pipe", "version": "native-constructor-v2"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "callHierarchyProvider": true}}
			case "textDocument/prepareCallHierarchy":
				result = []lsp.CallHierarchyItem{root}
			case "callHierarchy/incomingCalls":
				result = []lsp.CallHierarchyIncomingCall{{From: caller, FromRanges: []lsp.Range{site, site}}}
			case "callHierarchy/outgoingCalls":
				outSite := site
				if len(supervised) > 1 && supervised[1] {
					outSite.Start.Character++
				}
				if len(supervised) > 2 && supervised[2] {
					if err := os.Remove(filepath.Join(workspace, "root.go")); err != nil {
						return
					}
				}
				result = []lsp.CallHierarchyOutgoingCall{{To: callee, FromRanges: []lsp.Range{outSite, outSite}}}
			}
			raw, _ := json.Marshal(result)
			body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, raw)
			if _, e = fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(body), body); e != nil {
				return
			}
		}
	}()
	var starter sessionruntime.Starter = managedOperationStarter{child}
	if len(supervised) > 0 && supervised[0] {
		supervisor, e := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 65536, GracePeriod: 100 * time.Millisecond})
		if e != nil {
			t.Fatal(e)
		}
		starter = sessionruntime.ManagedStarter{Manager: supervisor}
	}
	m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 4, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Teardown(context.Background()); child.Close(); m.Shutdown(context.Background()) })
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	spec := managedprocess.Spec{Path: executable, Dir: workspace, Env: []string{}}
	if len(supervised) > 0 && supervised[0] {
		spec.Args = []string{"-test.run=^TestPrivateNativeConstructorSELFResponderV2$", "--", "native-constructor-self-v2", workspace, fmt.Sprint(asymmetric)}
	}
	s := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go", Process: spec})
	if s.Failure != "" {
		t.Fatal(s.Failure)
	}
	pending := m.BeginReadiness(context.Background(), s.SessionID, s.Generation, time.Now().Add(time.Second))
	ready, ok := m.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatal("readiness", ready)
	}
	line, character := uint32(1), uint32(6)
	q := inventorysuccessor.PrivateNativeCallsRequestV2{Session: sessionruntime.SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: workspace}, Directional: sessionruntime.DirectionalCallHierarchyRequestPrivate{SessionID: s.SessionID, Generation: s.Generation, URI: uris[0], LanguageID: "go", Line: &line, Character: &character, Deadline: time.Now().Add(15 * time.Second), MaxMessages: 4, MaxBytes: 65536}, Revision: "controlled-declared-revision", Privacy: "private-constructor-only"}
	return m, q, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), methods...) }
}

func TestPrivateNativeConstructorDiscriminatorTopologies(t *testing.T) {
	for _, asymmetric := range []bool{false, true} {
		name := "SELF"
		if asymmetric {
			name = "ASYMMETRIC"
		}
		t.Run(name, func(t *testing.T) {
			m, q, methods := nativeConstructorFixtureV2(t, asymmetric)
			var got inventorysuccessor.PrivateNativeCallsResultV2
			err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
				var e error
				got, e = inventorysuccessor.ConstructNativeCallsPrivateV2(ctx, m, q)
				return e
			})
			if err != nil || !got.Admission.Admission.Valid() {
				t.Fatalf("ASSERT_NATIVE_CONSTRUCTOR_ADMITTED_%s: err=%v valid=%v", name, err, got.Admission.Admission.Valid())
			}
			t.Logf("ASSERT_NATIVE_CONSTRUCTOR_ADMITTED_%s: PASS", name)
			if string(got.Composite.Artifact.Constituents[0].Invocation) != string(got.Composite.Artifact.Constituents[1].Invocation) {
				t.Fatal("ASSERT_SAME_ENTIRE_PARENT_INVOCATION")
			}
			t.Logf("ASSERT_SAME_ENTIRE_PARENT_INVOCATION_%s: PASS", name)
			for i, v := range got.Views {
				var envelope graphprovenance.EvidenceV5
				if e := json.Unmarshal(got.Inputs[i].Bytes, &envelope); e != nil {
					t.Fatal(e)
				}
				raw, e := base64.StdEncoding.DecodeString(envelope.GraphV5)
				if e != nil {
					t.Fatal(e)
				}
				expected := v.Result.Graph
				expected.SchemaVersion = graph.SchemaVersionV5
				expected.Invocation = got.ParentInvocation
				expected.Capabilities.CallHierarchyProvider = true
				expected.Canonicalize()
				wantNative, e := json.Marshal(expected)
				if e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(raw, wantNative) {
					t.Fatalf("ASSERT_VIEW_NATIVE_CORRESPONDENCE_%s_%d", name, i)
				}
				t.Logf("ASSERT_VIEW_NATIVE_CORRESPONDENCE_%s_%d: PASS", name, i)
				if e := acquisition.ValidateResult(v.Result); e != nil {
					t.Fatalf("ASSERT_VIEW_REPLAY_%s_%d: %v", name, i, e)
				}
				if e := graphprovenance.ValidateV2(got.Sources[i]); e != nil {
					t.Fatalf("ASSERT_ORDINARY_SOURCE_%s_%d: %v", name, i, e)
				}
			}
			if len(got.RawJoins) != 4 {
				t.Fatalf("ASSERT_FOUR_RAW_RANGES_%s: got=%d", name, len(got.RawJoins))
			}
			t.Logf("ASSERT_FOUR_RAW_RANGES_%s: PASS", name)
			if bytes.Equal(got.Inputs[0].Bytes, got.Inputs[1].Bytes) {
				t.Fatal("ASSERT_NOT_TWO_COPIES_OF_ONE_INPUT")
			}
			want := 1
			if asymmetric {
				want = 2
			}
			occurrences := got.Admission.Admission.Occurrences()
			for _, j := range got.RawJoins {
				matches := 0
				for _, o := range occurrences {
					if j.RelationID == o.RelationID && j.EdgeLocalOrdinal == o.Ordinal && j.Range == o.CallSite {
						matches++
					}
				}
				if matches != 1 {
					t.Fatalf("ASSERT_EDGE_LOCAL_JOIN_%s: matches=%d relation=%s local=%d", name, matches, j.RelationID, j.EdgeLocalOrdinal)
				}
			}
			t.Logf("ASSERT_EDGE_LOCAL_JOIN_%s: PASS", name)
			if len(occurrences) != want {
				t.Fatalf("ASSERT_CANONICAL_OCCURRENCES_%s: got=%d want=%d", name, len(occurrences), want)
			}
			for _, o := range occurrences {
				if o.Weight != 1 {
					t.Fatalf("ASSERT_UNIT_WEIGHT_%s: %v", name, o.Weight)
				}
			}
			attempts := 0
			union := map[string]bool{}
			charged := 0
			for index, s := range got.Sources {
				attempts += s.CaptureBudget.Attempts
				actualBytes := 0
				for _, c := range s.Captures {
					union[c.URI] = true
					actualBytes += len(c.Content)
					t.Logf("SOURCE_CAPTURE_%s_view%d: URI=%s class=%s status=%s bytes=%d", name, index, c.URI, c.Classification, c.Status, len(c.Content))
				}
				if actualBytes != s.CaptureBudget.ChargedBytes {
					t.Fatalf("ASSERT_ACTUAL_SOURCE_BYTES_%s_%d: actual=%d charged=%d", name, index, actualBytes, s.CaptureBudget.ChargedBytes)
				}
				charged += actualBytes
			}
			wantURIs := 1
			if asymmetric {
				wantURIs = 3
			}
			if len(union) != wantURIs {
				t.Fatalf("ASSERT_SOURCE_URI_COVERAGE_%s: %d != %d", name, len(union), wantURIs)
			}
			for _, o := range got.Observations {
				if sha256.Sum256(o.Result) != o.Wire.ResultSHA256Private || uint64(len(o.Result)) != o.Wire.ResultLengthPrivate {
					t.Fatalf("ASSERT_ACTUAL_WIRE_COMMITMENT_%s: ordinal=%d", name, o.Ordinal)
				}
			}
			t.Logf("SOURCE_WORK_%s: initialSupplyBytes=%d postTraversalAttempts=%d postTraversalBytes=%d passes=2 maxFilesPerPass=64 maxFileBytes=1048576 maxTotalBytesPerPass=4194304", name, len(got.ParentSupply.Content), attempts, charged)
			wantReads := 2
			if asymmetric {
				wantReads = 4
			}
			if attempts != wantReads {
				t.Fatalf("ASSERT_ACTUAL_SOURCE_PASSES_%s: %d != %d", name, attempts, wantReads)
			}
			actual := methods()
			if len(actual) != 4 || actual[1] != "textDocument/prepareCallHierarchy" || actual[2] != "callHierarchy/incomingCalls" || actual[3] != "callHierarchy/outgoingCalls" {
				t.Fatalf("ASSERT_EXACT_THREE_RPC_%s: %v", name, actual)
			}
			t.Logf("ASSERT_CANONICAL_OCCURRENCES_%s: PASS; raw=4 canonical=%d weight=1 sourceAttempts=%d RPC=%v", name, want, attempts, actual)
		})
	}
}
