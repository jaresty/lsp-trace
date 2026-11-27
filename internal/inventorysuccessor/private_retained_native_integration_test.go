package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	qt "lsp-trace/internal/adr0011querytarget"
	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

// This responder supplies controlled wire assertions, not provider correctness.
// In particular B -> A is an incoming server assertion, not a source-code claim.
// No owned-method view, grant, completion token or admission is fabricated here.
type retainedNativePipe struct {
	*discoveryKernelChildV2
	done     chan struct{}
	mu       sync.Mutex
	requests []retainedNativeWire
	supplies []json.RawMessage
	starts   int
}
type retainedNativeWire struct {
	ID             uint64
	Method         string
	Params, Result []byte
}

func (p *retainedNativePipe) Teardown(ctx context.Context) managedprocess.TeardownObservation {
	o := p.discoveryKernelChildV2.Teardown(ctx)
	select {
	case <-p.done:
	case <-ctx.Done():
		o.Census.Bounded = false
	}
	return o
}
func (p *retainedNativePipe) Close() managedprocess.ResourceObservation {
	p.Teardown(context.Background())
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func retainedNativeRange(start, end uint32) lsp.Range {
	return lsp.Range{Start: lsp.Position{Line: 1, Character: start}, End: lsp.Position{Line: 1, Character: end}}
}
func retainedNativeItem(uri, name string, end uint32) lsp.CallHierarchyItem {
	return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: uri, Range: retainedNativeRange(0, end), SelectionRange: retainedNativeRange(5, 6)}
}
func newRetainedNativePipe(t *testing.T, a, b, symbolName string) *retainedNativePipe {
	t.Helper()
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	p := &retainedNativePipe{discoveryKernelChildV2: &discoveryKernelChildV2{input: input, stdin: stdin, stdout: stdout, output: output}, done: make(chan struct{})}
	t.Cleanup(func() { p.Close() })
	go func() {
		defer close(p.done)
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(output, lspwire.DefaultLimits())
		for {
			m, err := reader.Read()
			if err != nil || m.Method == "exit" {
				return
			}
			if len(m.ID) == 0 {
				if m.Method == "textDocument/didOpen" {
					p.mu.Lock()
					p.supplies = append(p.supplies, bytes.Clone(m.Params))
					p.mu.Unlock()
				}
				continue
			}
			var value any
			ai, bi := retainedNativeItem(a, "A", 16), retainedNativeItem(b, "B", 11)
			switch m.Method {
			case "initialize":
				value = map[string]any{"serverInfo": map[string]any{"name": "retained-native-pipe", "version": "1"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "definitionProvider": true, "callHierarchyProvider": true}}
			case "shutdown":
				value = nil
			case "textDocument/documentSymbol":
				value = []lsp.DocumentSymbol{{Name: symbolName, Kind: ai.Kind, Range: ai.Range, SelectionRange: ai.SelectionRange}}
			case "textDocument/definition":
				r := bi.SelectionRange
				value = []lsp.Location{{URI: b, Range: &r}}
			case "textDocument/prepareCallHierarchy":
				value = []lsp.CallHierarchyItem{ai}
			case "callHierarchy/incomingCalls":
				value = []lsp.CallHierarchyIncomingCall{{From: bi, FromRanges: []lsp.Range{bi.SelectionRange, bi.SelectionRange}}}
			case "callHierarchy/outgoingCalls":
				value = []lsp.CallHierarchyOutgoingCall{{To: bi, FromRanges: []lsp.Range{retainedNativeRange(11, 12), retainedNativeRange(11, 12)}}}
			default:
				return // Unexpected traffic cannot be mistaken for an empty success.
			}
			result, err := json.Marshal(value)
			if err != nil {
				return
			}
			var id uint64
			if json.Unmarshal(m.ID, &id) != nil {
				return
			}
			p.mu.Lock()
			p.requests = append(p.requests, retainedNativeWire{id, m.Method, bytes.Clone(m.Params), bytes.Clone(result)})
			p.mu.Unlock()
			if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: result}) != nil {
				return
			}
		}
	}()
	return p
}
func (p *retainedNativePipe) wire(t *testing.T) []retainedNativeWire {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	expected := []string{"initialize", "textDocument/documentSymbol", "textDocument/definition", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls", "shutdown"}
	if p.starts != 1 || len(p.requests) != len(expected) {
		t.Fatalf("ASSERT_NATIVE_WIRE: starts=%d requests=%v", p.starts, p.requests)
	}
	ids := map[uint64]bool{}
	for i, r := range p.requests {
		semantic := i > 0 && i < len(expected)-1
		if r.Method != expected[i] || r.ID == 0 || (semantic && ids[r.ID]) {
			t.Fatalf("ASSERT_NATIVE_WIRE: record %d: %+v", i, r)
		}
		if semantic {
			ids[r.ID] = true
		}
	}
	return append([]retainedNativeWire(nil), p.requests...)
}

type retainedNativeFixture struct {
	acquisition                PrivateDiscoveryOperationV2
	discovery                  harnessV2
	discoveryHead              resourceHeadV2
	root                       *publication.Root
	directory, workspace, a, b string
	deadline                   time.Time
	pipe                       *retainedNativePipe
	definition                 int
	config                     PrivateSelectedConfigurationV2
}

func newRetainedNativeFixture(t *testing.T, symbolName string) retainedNativeFixture {
	t.Helper()
	ctx := context.Background()
	f := retainedNativeFixture{definition: -1}
	var err error
	f.workspace, err = filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bodies := []string{"package fixture\nfunc A() { B() }\n", "package fixture\nfunc B() {}\n"}
	if len(bodies[0])+len(bodies[1]) > 128 {
		t.Fatal("source fixture bound")
	}
	uris := make([]string, 2)
	for i, name := range []string{"a.go", "b.go"} {
		path := filepath.Join(f.workspace, name)
		if err = os.WriteFile(path, []byte(bodies[i]), 0600); err != nil {
			t.Fatal(err)
		}
		uris[i] = (&url.URL{Scheme: "file", Path: path}).String()
	}
	f.a, f.b = uris[0], uris[1]
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: f.workspace, Profile: "go", EnvironmentReference: "offline"})
	if err != nil {
		t.Fatal(err)
	}
	f.pipe = newRetainedNativePipe(t, f.a, f.b, symbolName)
	c := sessionruntime.PrivateDiscoveryConstruction{CaptureOwnedMethods: true, CaptureCallDirections: sessionruntime.PrivateCallsIncoming | sessionruntime.PrivateCallsOutgoing, NewStarter: func() (sessionruntime.Starter, error) {
		f.pipe.mu.Lock()
		f.pipe.starts++
		f.pipe.mu.Unlock()
		return discoveryKernelStarterV2{f.pipe}, nil
	}, Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go", Process: managedprocess.Spec{Path: "controlled-io-pipe", Dir: f.workspace, Env: []string{"PATH=/nonexistent"}}}, Deadline: time.Now().Add(10 * time.Second), Limits: sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (sessionruntime.MaxDocumentSupplyBytes + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}, Members: []sessionruntime.PrivateDiscoveryMember{{Path: "a.go", URI: f.a, LanguageID: "go"}, {Path: "b.go", URI: f.b, LanguageID: "go"}}}
	f.deadline = c.Deadline
	discovery, h, binding := discoverySyntheticConfigurationV2(t, -1, &c)
	f.discovery = h
	f.acquisition, err = discovery.ReservePartition(ctx, binding, hashLabelV2("retained-native-discovery"))
	if err != nil {
		t.Fatal("discovery reserve", err)
	}
	before, err := h.k.reopen(ctx, h.j, discovery.limits)
	if err != nil {
		t.Fatal(err)
	}
	err = f.acquisition.Execute(ctx, c, func(ctx context.Context, s *sessionruntime.PrivateDiscoverySession) error {
		for _, uri := range uris {
			if e := s.Supply(ctx, uri, "go"); e != nil {
				return e
			}
		}
		if _, e := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: f.a}}); e != nil {
			return e
		}
		if _, e := s.Definition(ctx, f.a, lsp.Position{Line: 1, Character: 11}); e != nil {
			return e
		}
		_, e := s.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: f.a}, Position: lsp.Position{Line: 1, Character: 5}})
		return e
	})
	if err != nil {
		t.Fatal("discovery execute", err)
	}
	if err = f.acquisition.Complete(ctx); err != nil {
		t.Fatal("discovery complete", err)
	}
	after, err := h.k.reopen(ctx, h.j, discovery.limits)
	if err != nil {
		t.Fatal(err)
	}
	if before.Windows != 1 || before.Charged != after.Charged || after.Windows != before.Windows || after.Disposition != "OBSERVED" {
		t.Fatal("ASSERT_DISCOVERY_ACCOUNTING", before, after)
	}
	f.discoveryHead = after
	retainedNativeReservation(t, h.k, after, f.deadline, false)
	select {
	case <-f.pipe.done:
	default:
		t.Fatal("ASSERT_RESPONDER_JOIN")
	}
	f.pipe.wire(t)
	// The completed teardown joined the responder; the retained notification
	// copies therefore cannot race with these exact source checks.
	f.pipe.mu.Lock()
	supplies := append([]json.RawMessage(nil), f.pipe.supplies...)
	f.pipe.mu.Unlock()
	if len(supplies) != 2 {
		t.Fatal("ASSERT_TWO_SUPPLIED_SOURCES", len(supplies))
	}
	for i, raw := range supplies {
		var notification struct {
			TextDocument struct {
				URI, LanguageID, Text string
				Version               int
			}
		}
		if json.Unmarshal(raw, &notification) != nil || notification.TextDocument.URI != uris[i] || notification.TextDocument.LanguageID != "go" || notification.TextDocument.Version != 1 || notification.TextDocument.Text != bodies[i] {
			t.Fatal("ASSERT_SUPPLIED_SOURCE_BYTES", i)
		}
	}
	owned := f.acquisition.OwnedMethods()
	if owned.NativeCallCount() != 1 {
		t.Fatal("native capture count")
	}
	for i := 0; i < owned.GroupingQueryCount(); i++ {
		q, ok := owned.GroupingQuery(i)
		if ok && q.Method == "textDocument/definition" {
			if q.URI != f.a || q.Line != 1 || q.Character != 11 {
				t.Fatal("definition descriptor", q)
			}
			f.definition = i
		}
	}
	if f.definition < 0 {
		t.Fatal("definition missing")
	}
	f.directory = t.TempDir()
	if err = os.Chmod(f.directory, 0700); err != nil {
		t.Fatal(err)
	}
	f.root, err = publication.OpenRoot(f.directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.root.Close() })
	pin := hashLabelV2("retained-test-contract")
	in := cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, CaptureLimits: cc.ProductionCaptureLimits(), PacketSourcePolicy: sourceprojection.Policy{PolicyID: "CONTROLLED_BOUNDED_CONTRACT"}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, err := cc.NewContinuationContract(in)
	if err != nil {
		t.Fatal(err)
	}
	f.config = PrivateSelectedConfigurationV2{RunID: "retained-native", SeedNamespace: "retained-native", Lineage: "retained-native", Mode: "CD", Seed: 7, Attempts: [3]string{hashLabelV2("retained-native-M"), hashLabelV2("retained-native-P"), hashLabelV2("retained-native-S")}, Contract: contract, ContractInput: in, SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxObjects: 1000, MaxWork: 10000}}
	return f
}

func TestRetainedNativeMixedIntegration(t *testing.T) {
	for _, mode := range []string{"success", "wrong-association", "absent-native"} {
		t.Run(mode, func(t *testing.T) {
			name := "A"
			if mode == "wrong-association" {
				name = "WrongA"
			}
			f := newRetainedNativeFixture(t, name)
			ctx := context.Background()
			wire := f.pipe.wire(t)
			if mode == "absent-native" {
				if _, _, err := OpenLocalFolderSelectedV1(ctx, f.root, f.acquisition, 128, PrivateRetainedNativeCallsV1{}, f.config); err == nil {
					t.Fatal("ASSERT_ABSENT_NATIVE_CALLS")
				}
				owner, q, err := OpenLocalFolderSelectedV1(ctx, f.root, f.acquisition, f.definition, PrivateRetainedNativeCallsV1{}, f.config)
				if err != nil {
					t.Fatal("definition D opening", err)
				}
				if !reflect.DeepEqual(q.Kinds, []pa.RelationKind{pa.TypedResolvesToDefinition}) {
					t.Fatal("ASSERT_DEFINITION_DOWNGRADE", q.Kinds)
				}
				if _, err = NewPrivateSelectedPreparationPlanV2(owner, q, f.config); err == nil {
					t.Fatal("ASSERT_CD_DOWNGRADE_REFUSED")
				}
				ready, e := owner.k.reopen(ctx, owner.job, owner.limits)
				if e != nil || ready.Disposition != "READY" || ready.Windows != 0 || ready.Charged != (countersV2{}) {
					t.Fatal("ASSERT_DOWNGRADE_NO_SPENDING", e)
				}
				if !reflect.DeepEqual(wire, f.pipe.wire(t)) {
					t.Fatal("ASSERT_NO_REQUERY")
				}
				t.Log("ASSERT_ABSENT_NATIVE_CALLS: PASS; ASSERT_CD_DOWNGRADE_REFUSED: PASS")
				return
			}
			native, err := TransferLocalFolderNativeCallsV1(ctx, f.root, f.acquisition, 0)
			if err != nil {
				t.Fatal("native transfer", err)
			}
			summary, ok := native.Summary()
			if !ok || summary.URI != f.a {
				t.Fatal("native summary")
			}
			for i, id := range summary.OriginalRequestIDs {
				if id != wire[i+3].ID {
					t.Fatal("ASSERT_NATIVE_ORIGINAL_IDS", summary, wire)
				}
			}
			// Locate the producer's actual native head by its diagnostic stable selector;
			// no job, authorization or token is issued by this read.
			nativeKernel := &kernelV2{root: f.root}
			var nativeHead resourceHeadV2
			retainedNativeRead(t, nativeKernel, currentSelectorV2(jobV2{StableID: hashLabelV2(fmt.Sprintf("local-folder-native-transfer.v1/%s/%d", f.acquisition.state.attempt, 0))}), &nativeHead)
			retainedNativeReservation(t, nativeKernel, nativeHead, f.deadline, true)
			var prepared []lsp.CallHierarchyItem
			if len(native.state.native.Observations) != 3 || json.Unmarshal(native.state.native.Observations[0].Result, &prepared) != nil || len(prepared) != 1 || !reflect.DeepEqual(prepared[0], retainedNativeItem(f.a, "A", 16)) {
				t.Fatal("ASSERT_NATIVE_A_UNCHANGED")
			}
			if nativeHead.Disposition != "OBSERVED" || nativeHead.Job.Identity == f.discoveryHead.Job.Identity {
				t.Fatal("ASSERT_NATIVE_SEPARATE_FUNDING")
			}
			if _, err = TransferLocalFolderNativeCallsV1(ctx, f.root, f.acquisition, 0); err == nil {
				t.Fatal("ASSERT_NATIVE_REUSE")
			}
			owner, q, err := OpenLocalFolderSelectedV1(ctx, f.root, f.acquisition, f.definition, native, f.config)
			if err != nil {
				t.Fatal("selected open", err)
			}
			if owner.job.Identity == nativeHead.Job.Identity || owner.job.Identity == f.discoveryHead.Job.Identity {
				t.Fatal("ASSERT_M_SEPARATE_FUNDING")
			}
			if !q.Start.Deadline.Equal(f.deadline) {
				t.Fatal("ASSERT_ORIGINAL_DEADLINE")
			}
			if _, _, err = OpenLocalFolderSelectedV1(ctx, f.root, f.acquisition, f.definition, native, f.config); err == nil {
				t.Fatal("ASSERT_SELECTED_REUSE")
			}
			plan, err := NewPrivateSelectedPreparationPlanV2(owner, q, f.config)
			if err != nil {
				t.Fatal("plan", err)
			}
			op, err := owner.ReserveSelectedMixedInputV2(ctx, f.config.Attempts[0], q, plan)
			if err != nil {
				t.Fatal("M reserve", err)
			}
			before, err := owner.k.reopen(ctx, owner.job, owner.limits)
			if err != nil {
				t.Fatal(err)
			}
			retainedNativeReservation(t, owner.k, before, f.deadline, true)
			out, executionErr := ExecuteMixedInputV2(ctx, op, q)
			after, err := owner.k.reopen(ctx, owner.job, owner.limits)
			if err != nil {
				t.Fatal(err)
			}
			if before.Charged != after.Charged || before.Windows != 1 || after.Windows != 1 || before.Charged.ProviderRequests != 0 || before.Charged.ProviderMessages != 0 || before.Charged.WireBytes != 0 {
				t.Fatal("ASSERT_M_ACCOUNTING", before, after)
			}
			if mode == "wrong-association" {
				if native.state.native.ParentInvocation.Target.Column != 5 {
					t.Fatal("native A declaration changed")
				}
				if executionErr == nil || !strings.Contains(executionErr.Error(), "retained native association") || out.retainedCompletion != nil || after.Disposition == "OBSERVED" {
					t.Fatal("ASSERT_ASSOCIATION_REFUSAL", executionErr, after.Disposition)
				}
				if _, err = out.SourceOriginalsV1(); err == nil {
					t.Fatal("association originals escaped")
				}
				t.Log("ASSERT_ASSOCIATION_REFUSAL: PASS after native conversion")
			} else {
				if executionErr != nil {
					t.Fatal("ASSERT_RETAINED_NATIVE_CD_SUCCESS", executionErr)
				}
				if out.retainedCompletion == nil || after.Disposition != "OBSERVED" {
					t.Fatal("completion/readback missing")
				}
				originals, err := out.SourceOriginalsV1()
				if err != nil {
					t.Fatal("SourceOriginals", err)
				}
				if originals.InputID() != out.Input.ID() || originals.Workspace() != f.workspace || !originals.Deadline().Equal(f.deadline) {
					t.Fatal("originals identity")
				}
				retainedNativeAssertOutput(t, f, native, out, wire)
			}
			if _, err = ExecuteMixedInputV2(ctx, op, q); err == nil {
				t.Fatal("ASSERT_M_REUSE")
			}
			reused, err := owner.k.reopen(ctx, owner.job, owner.limits)
			if err != nil || reused.Charged != after.Charged || reused.Windows != after.Windows {
				t.Fatal("ASSERT_REUSE_NO_SPENDING", err)
			}
			nativeAfter, e := nativeKernel.reopen(ctx, nativeHead.Job, verificationLimitsV2{4096, 64 << 20, 16384})
			if e != nil || nativeAfter.Identity != nativeHead.Identity || nativeAfter.Charged != nativeHead.Charged || nativeAfter.Windows != nativeHead.Windows {
				t.Fatal("ASSERT_NATIVE_NO_REFUND_OR_REUSE_SPEND", e)
			}
			discoveryAfter, e := f.discovery.k.reopen(ctx, f.discovery.j, verificationLimitsV2{4096, 64 << 20, 16384})
			if e != nil || discoveryAfter.Identity != f.discoveryHead.Identity || discoveryAfter.Charged != f.discoveryHead.Charged {
				t.Fatal("ASSERT_DISCOVERY_NO_REUSE_SPEND", e)
			}
			if !reflect.DeepEqual(wire, f.pipe.wire(t)) {
				t.Fatal("ASSERT_NO_REQUERY")
			}
			t.Log("ASSERT_RETAINED_NATIVE_CD:", mode, "PASS")
		})
	}
}

func retainedNativeAssertOutput(t *testing.T, f retainedNativeFixture, n PrivateRetainedNativeCallsV1, out PrivateManagedGroupingInput, wire []retainedNativeWire) {
	t.Helper()
	roles := map[string][]byte{}
	total := len(out.Input.CanonicalBytes())
	for i, p := range out.Predecessors {
		roles[p.Role] = p.Bytes
		total += len(p.Bytes)
		if !reflect.DeepEqual(p, out.originals[i]) {
			t.Fatal("ASSERT_ORIGINAL_CUSTODY", p.Role)
		}
	}
	if len(out.Predecessors)+1 > 8 || total > 1<<20 || len(out.TypedInput.Occurrences) > 1024 {
		t.Fatal("ASSERT_RETAINED_BOUNDS")
	}
	for _, role := range []string{"RETAINED_COMPOSITE", "ACQUISITION_RECEIPT", "NATIVE_EVIDENCE"} {
		if len(roles[role]) == 0 {
			t.Fatal("ASSERT_PREDECESSOR", role)
		}
	}
	if !bytes.Equal(roles["RETAINED_COMPOSITE"], n.state.native.Composite.Bytes) || !bytes.Equal(roles["NATIVE_EVIDENCE"], n.state.native.nativeEvidence) {
		t.Fatal("ASSERT_NATIVE_CUSTODY")
	}
	for _, p := range out.publications {
		raw, err := publication.ReadVerifiedBoundFile(f.root, p.selector, 1<<20)
		if err != nil || !bytes.Equal(raw, p.raw) || privateGroupingDigest(raw) != p.digest {
			t.Fatal("ASSERT_PUBLICATION_READBACK", err)
		}
	}
	var proof rg.PrivateNativeEvidenceV2
	if err := json.Unmarshal(roles["NATIVE_EVIDENCE"], &proof); err != nil {
		t.Fatal(err)
	}
	if len(proof.Requests) != 3 || len(proof.RawJoins) != 4 {
		t.Fatal("ASSERT_NATIVE_MULTIPLICITY", len(proof.Requests), len(proof.RawJoins))
	}
	for i, r := range proof.Requests {
		w := wire[i+3]
		if r.RequestID != w.ID || r.Ordinal != i || r.Status != "OBSERVED_NONEMPTY" || r.Method != w.Method || !bytes.Equal(r.Params, w.Params) || !bytes.Equal(r.Result, w.Result) || len(r.RequestFrame) == 0 || len(r.ResponseFrames) != 1 {
			t.Fatal("ASSERT_NATIVE_WIRE_BYTES", i)
		}
		request, err := lspwire.NewReader(bytes.NewReader(r.RequestFrame), lspwire.DefaultLimits()).Read()
		if err != nil || string(request.ID) != fmt.Sprint(w.ID) || request.Method != w.Method || !bytes.Equal(request.Params, w.Params) {
			t.Fatal("ASSERT_NATIVE_REQUEST_FRAME", i, err)
		}
		response, err := lspwire.NewReader(bytes.NewReader(r.ResponseFrames[0]), lspwire.DefaultLimits()).Read()
		if err != nil || string(response.ID) != fmt.Sprint(w.ID) || response.Error != nil || !bytes.Equal(response.Result, w.Result) {
			t.Fatal("ASSERT_NATIVE_RESPONSE_FRAME", i, err)
		}
	}
	calls, definitions := 0, 0
	for _, o := range out.TypedInput.Occurrences {
		switch o.Kind {
		case pa.TypedCalls:
			calls++
		case pa.TypedResolvesToDefinition:
			definitions++
		default:
			t.Fatal("unexpected kind", o.Kind)
		}
	}
	if calls != 2 || definitions != 1 || len(out.Ledger) != 1 {
		t.Fatal("ASSERT_TYPED_MULTIPLICITY", calls, definitions, len(out.Ledger))
	}
	if out.TypedInput.Authority != 0 || out.TypedInput.SourceGraphComplete != "UNKNOWN" {
		t.Fatal("ASSERT_CLAIM_CEILING")
	}
	t.Logf("actual retained bytes=%d native proof=%d joins=%d CALLS=%d definition=%d original IDs=%v", total, len(roles["NATIVE_EVIDENCE"]), len(proof.RawJoins), calls, definitions, n.state.summary.OriginalRequestIDs)
	retainedNativeAssertIdentity(t, f, n, out, roles, proof, wire)
	t.Log("ASSERT_ORIGINAL_CUSTODY: PASS; ASSERT_NATIVE_WIRE_BYTES: PASS; ASSERT_NATIVE_MAPPINGS: PASS; ASSERT_DEFINITION_IDENTITY: PASS")
}

// These helpers read actual producer records; they never seal or issue records.
func retainedNativeRead(t *testing.T, k *kernelV2, selector string, dst any) {
	t.Helper()
	raw, err := k.root.ReadSelector(selector, headLimitV2)
	if err != nil {
		t.Fatal("accounting read", err)
	}
	if err = parseV2(raw, dst); err != nil {
		t.Fatal("accounting parse", err)
	}
}
func retainedNativeReservation(t *testing.T, k *kernelV2, h resourceHeadV2, deadline time.Time, conversion bool) {
	t.Helper()
	checked, err := k.reopen(context.Background(), h.Job, verificationLimitsV2{4096, 64 << 20, 16384})
	if err != nil || checked.Identity != h.Identity || checked.Charged != h.Charged || h.Windows != 1 {
		t.Fatal("ASSERT_ACTUAL_RESERVATION_READBACK", err)
	}
	var r reservationV2
	retainedNativeRead(t, k, h.ReservationSelector, &r)
	var a authorizationV2
	retainedNativeRead(t, k, h.AuthorizationSelector, &a)
	if r.Job != h.Job.Identity || r.Attempt != h.Attempt || r.Tranche != h.Charged || r.Charged != h.Charged || r.Deadline != deadline.UnixNano() || r.Authorization != h.Authorization {
		t.Fatal("ASSERT_ACTUAL_RESERVATION", r, h)
	}
	if conversion {
		if r.Tranche.ProviderRequests != 0 || r.Tranche.ProviderMessages != 0 || r.Tranche.WireBytes != 0 || r.Tranche.RequestBytes != 0 || r.Tranche.ResponseBytes != 0 || a.Expires != deadline.UnixNano() {
			t.Fatal("ASSERT_CONVERSION_NO_PROVIDER", r, a)
		}
		for _, v := range []uint64{r.Tranche.LogicalBytes, r.Tranche.ReadHashBytes, r.Tranche.WriteBytes, r.Tranche.Records, r.Tranche.ParserBytes, r.Tranche.Work, r.Tranche.ActiveNS, r.Tranche.Calls, r.Tranche.OutputBytes} {
			if v == 0 {
				t.Fatal("ASSERT_CONVERSION_LOGICAL_FUNDING", r.Tranche)
			}
		}
	} else if r.Tranche.ProviderRequests == 0 || r.Tranche.ProviderMessages == 0 || r.Tranche.WireBytes == 0 {
		t.Fatal("ASSERT_DISCOVERY_PAID_PROVIDER", r.Tranche)
	}
}

func retainedNativeAssertIdentity(t *testing.T, f retainedNativeFixture, n PrivateRetainedNativeCallsV1, out PrivateManagedGroupingInput, roles map[string][]byte, proof rg.PrivateNativeEvidenceV2, wire []retainedNativeWire) {
	t.Helper()
	session := f.acquisition.OwnedMethods().GroupingSession()
	if session.SessionID == "" || session.Generation == 0 || session.Workspace != f.workspace {
		t.Fatal("ASSERT_SESSION_IDENTITY")
	}
	bound, ok := out.sourceEvidence.EncodedBound()
	if !ok || bound > 1<<20 {
		t.Fatal("ASSERT_SOURCE_ENCODING_BOUND")
	}
	v, err := out.sourceEvidence.Snapshot(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	source := []byte("package fixture\nfunc A() { B() }\n")
	if !bytes.Equal(v.Source, source) || !bytes.Equal(v.Supply.Content, source) || v.Supply.URI != f.a || v.Supply.DocumentVersion != 1 || v.Supply.SessionID != session.SessionID || v.Supply.Generation != session.Generation {
		t.Fatal("ASSERT_SOURCE_IDENTITY")
	}
	if v.Query.URI != f.a || v.Query.Line != 1 || v.Query.Character != 11 || v.Query.DocumentVersion != "1" || v.Query.Encoding != "utf-16" || v.Query.SourceDigest != privateGroupingDigest(source) || v.Query.SessionID != session.SessionID || v.Query.Generation != session.Generation || v.Query.OccurrenceID != out.replayBinding.Occurrence {
		t.Fatal("ASSERT_QUERY_IDENTITY", v.Query)
	}
	if v.Workspace != f.workspace || v.Revision != f.acquisition.state.binding.Revision || v.Privacy != "PRIVATE" || !v.Deadline.Equal(f.deadline) || v.Metadata.PositionEncoding != "utf-16" || v.Metadata.ProviderName != "retained-native-pipe" || v.Metadata.ProviderVersion != "1" {
		t.Fatal("ASSERT_PROVENANCE_IDENTITY")
	}
	if v.Target.Version != qt.OccurrenceCandidateVersionV1 || v.Target.SymbolName != "A" || v.Target.QueryURI != f.a || v.Target.QueryOccurrenceID != v.Query.OccurrenceID || v.Target.QueryCharacter != 11 || v.Target.QueryLine != 1 || v.Target.SelectionRange.Start.Character != 5 || v.Target.SelectionRange.End.Character != 6 || v.Target.SourceDigest != v.Query.SourceDigest || v.Target.DocumentVersion != v.Query.DocumentVersion || v.Target.SessionID != session.SessionID || v.Target.Generation != session.Generation || v.Target.Encoding != "utf-16" {
		t.Fatal("ASSERT_OCCURRENCE_CONTAINER", v.Target)
	}
	if len(v.Methods) != 1 {
		t.Fatal("ASSERT_DEFINITION_PAIR_COUNT")
	}
	for i, p := range []sessionruntime.OwnedMethodPair{v.Symbol, v.Methods[0]} {
		w := wire[i+1]
		if p.Key.ID != w.ID || p.Method != w.Method || p.SessionID != session.SessionID || p.Generation != session.Generation || p.Key.Generation != session.Generation || !bytes.Equal(p.Params, w.Params) || !bytes.Equal(p.Result, w.Result) || p.Source == nil || p.Source.URI != f.a || p.Source.Version != 1 || p.Source.SHA256 != v.Query.SourceDigest {
			t.Fatal("ASSERT_METHOD_ORIGINAL_BYTES", i)
		}
	}
	var definitionParams lsp.TextDocumentPositionParams
	if json.Unmarshal(wire[2].Params, &definitionParams) != nil || definitionParams.TextDocument.URI != f.a || definitionParams.Position != (lsp.Position{Line: 1, Character: 11}) {
		t.Fatal("ASSERT_DEFINITION_PARAMS")
	}
	var locations []lsp.Location
	if json.Unmarshal(wire[2].Result, &locations) != nil || len(locations) != 1 || locations[0].URI != f.b || locations[0].Range == nil || *locations[0].Range != retainedNativeRange(5, 6) {
		t.Fatal("ASSERT_DEFINITION_B_ENDPOINT")
	}
	// Decode only diagnostics, never an admission or completion token.
	var query struct {
		Query                                 qt.Query
		Target                                qt.Candidate
		Source                                []byte
		Supply                                privateGroupingSupply
		Metadata                              sessionruntime.SessionMetadata
		Revision, Custody, Privacy, Workspace string
		Deadline                              time.Time
		Symbol                                privateGroupingNative
		Methods                               []privateGroupingNative
	}
	if json.Unmarshal(roles["METHOD_QUERY"], &query) != nil || query.Query != v.Query || query.Target != v.Target || !bytes.Equal(query.Source, v.Source) || query.Custody != "CALLER_ASSERTED" || query.Revision != v.Revision || query.Privacy != v.Privacy || query.Workspace != v.Workspace || !query.Deadline.Equal(f.deadline) || query.Metadata != v.Metadata || len(query.Methods) != 1 || !reflect.DeepEqual(query.Symbol, privateGroupingNativePair(v.Symbol)) || !reflect.DeepEqual(query.Methods[0], privateGroupingNativePair(v.Methods[0])) {
		t.Fatal("ASSERT_QUERY_PREDECESSOR")
	}
	if !reflect.DeepEqual(query.Supply, privateGroupingSupply{v.Supply.Classification, v.Supply.SessionID, v.Supply.Generation, v.Supply.URI, v.Supply.DocumentVersion, v.Supply.Method, v.Supply.Content, []byte(v.Supply.Params)}) {
		t.Fatal("ASSERT_QUERY_SUPPLY_CUSTODY")
	}
	var envelope pa.UntrustedMethodEvidenceEnvelope
	if json.Unmarshal(roles["METHOD_EVIDENCE"], &envelope) != nil || envelope.Authority != 0 || envelope.Accepted || envelope.Completeness != "UNKNOWN" || len(envelope.Occurrences) != 1 {
		t.Fatal("ASSERT_METHOD_CLAIM_CEILING")
	}
	var ceiling struct {
		Authority    int    `json:"authority"`
		Accepted     bool   `json:"accepted"`
		Completeness string `json:"completeness"`
	}
	if json.Unmarshal(out.Input.CanonicalBytes(), &ceiling) != nil || ceiling.Authority != 0 || ceiling.Accepted || ceiling.Completeness != "UNKNOWN" {
		t.Fatal("ASSERT_INPUT_CLAIM_CEILING")
	}
	endpointRange := Range{}
	endpointRange.Start.Line, endpointRange.End.Line = 1, 1
	endpointRange.Start.Character, endpointRange.End.Character = 5, 6
	endpointRaw, err := json.Marshal(struct {
		URI         string
		Range       Range
		TargetRange *Range
	}{f.b, endpointRange, nil})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := privateGroupingDigest(append(endpointRaw, '\n'))
	from := qt.DefinitionQueryNodeV1(v.Query, v.Target)
	identity := privateGroupingDigest([]byte(fmt.Sprintf("private-owned-grouping\x00%s\x00%d\x00%d\x00%s\x00%d", session.SessionID, session.Generation, wire[2].ID, wire[2].Method, 0)))
	if from == v.Target.SymbolID || from == "" {
		t.Fatal("ASSERT_DEFINITION_OCCURRENCE_DISTINCT")
	}
	expected := pa.TypedOccurrence{Identity: identity, From: from, To: endpoint, Kind: pa.TypedResolvesToDefinition, Ordinal: 0}
	found := false
	for _, o := range out.TypedInput.Occurrences {
		if o.Kind == pa.TypedResolvesToDefinition {
			found = true
			if o != expected {
				t.Fatal("ASSERT_DEFINITION_IDENTITY", o, expected)
			}
		}
	}
	if !found || out.Ledger[0] != (PrivateManagedGroupingOccurrence{identity, string(pa.TypedResolvesToDefinition), from, endpoint, 0}) {
		t.Fatal("ASSERT_DEFINITION_LEDGER")
	}
	e := envelope.Occurrences[0]
	if e.Identity != identity || e.From != from || e.To != endpoint || e.Ordinal != 0 || e.QueryDigest != privateGroupingDigest(roles["METHOD_QUERY"]) || e.EvidenceDigest != privateGroupingDigest(wire[2].Result) || e.SessionID != session.SessionID || e.Generation != session.Generation || e.Revision != v.Revision || e.PositionEncoding != "utf-16" {
		t.Fatal("ASSERT_DEFINITION_EVIDENCE_MAPPING")
	}
	var mapping struct {
		Mappings []struct {
			Identity, Kind, From, To string
			Ordinal                  int
			QuerySelector            string `json:"query_selector"`
			EvidenceSelector         string `json:"evidence_selector"`
		}
	}
	if json.Unmarshal(roles["METHOD_MAPPING_RECEIPT"], &mapping) != nil || len(mapping.Mappings) != 1 {
		t.Fatal("ASSERT_DEFINITION_MAPPING_RECEIPT")
	}
	m := mapping.Mappings[0]
	if m.Identity != identity || m.Kind != string(pa.TypedResolvesToDefinition) || m.From != from || m.To != endpoint || m.Ordinal != 0 || m.QuerySelector != "methods/query.json" || m.EvidenceSelector != "methods/evidence.json" {
		t.Fatal("ASSERT_DEFINITION_MAPPING_RECEIPT", m)
	}
	native := n.state.native
	if !reflect.DeepEqual(native.ParentSupply, v.Supply) {
		t.Fatal("ASSERT_NATIVE_SUPPLY_CUSTODY")
	}
	supplyBytes, err := json.Marshal(v.Supply)
	if err != nil || !bytes.Equal(supplyBytes, proof.Parent.SupplyObservation) {
		t.Fatal("ASSERT_NATIVE_SUPPLY_BYTES", err)
	}
	if proof.Invocation.WorkspaceURI != (&url.URL{Scheme: "file", Path: f.workspace}).String() || proof.Invocation.Provenance.SourceRevision != v.Revision || proof.Invocation.LanguageID != "go" || proof.Invocation.Target.URI != f.a || proof.Invocation.Target.Line != 1 || proof.Invocation.Target.Column != 5 {
		t.Fatal("ASSERT_NATIVE_INVOCATION_IDENTITY")
	}
	if proof.Parent.SessionID != session.SessionID || proof.Parent.Generation != session.Generation || proof.Parent.URI != f.a || proof.Parent.PositionEncoding != "utf-16" || !reflect.DeepEqual(proof.OriginalOrdinals, [2][]int{{-1, 0, 1}, {-1, 0, 2}}) {
		t.Fatal("ASSERT_NATIVE_PARENT_IDENTITY", proof.OriginalOrdinals)
	}
	for _, r := range proof.Requests {
		if r.Generation != session.Generation {
			t.Fatal("ASSERT_NATIVE_GENERATION")
		}
	}
	node := func(i lsp.CallHierarchyItem) string {
		return graph.NewNode(graph.Item{Name: i.Name, Kind: i.Kind, URI: i.URI, Range: privateNativeRangeV2(i.Range), SelectionRange: privateNativeRangeV2(i.SelectionRange)}).ID
	}
	a, b := node(retainedNativeItem(f.a, "A", 16)), node(retainedNativeItem(f.b, "B", 11))
	for direction := 0; direction < 2; direction++ {
		edges := native.Views[direction].Result.Graph.Edges
		if len(edges) != 1 || len(edges[0].CallSites) != 1 {
			t.Fatal("ASSERT_CANONICAL_EDGE_COUNT", direction)
		}
		edge := edges[0]
		caller, callee, label, site := b, a, "INCOMING", privateNativeRangeV2(retainedNativeRange(5, 6))
		if direction == 1 {
			caller, callee, label, site = a, b, "OUTGOING", privateNativeRangeV2(retainedNativeRange(11, 12))
		}
		if edge.CallerNodeID != caller || edge.CalleeNodeID != callee || edge.CallSites[0] != site {
			t.Fatal("ASSERT_NATIVE_B_ENDPOINT", direction)
		}
		for ordinal := 0; ordinal < 2; ordinal++ {
			expectedJoin := rg.PrivateNativeRawJoinV2{OriginalRequestOrdinal: direction + 1, ElementOrdinal: 0, RangeOrdinal: ordinal, ViewIndex: direction, EdgeLocalOrdinal: 0, Direction: label, RelationID: edge.RelationID, Range: site}
			if proof.RawJoins[2*direction+ordinal] != expectedJoin {
				t.Fatal("ASSERT_NATIVE_MAPPINGS", direction, ordinal, proof.RawJoins[2*direction+ordinal], expectedJoin)
			}
		}
		count := 0
		for _, o := range out.TypedInput.Occurrences {
			if o.Kind == pa.TypedCalls && o.RelationID == edge.RelationID {
				count++
				if o.From != caller || o.To != callee || o.Ordinal != 0 {
					t.Fatal("ASSERT_NATIVE_TYPED_MAPPING", o)
				}
			}
		}
		if count != 1 {
			t.Fatal("ASSERT_NATIVE_TYPED_MAPPING_COUNT", direction, count)
		}
	}
	nativeOccurrences := native.Admission.Admission.Occurrences()
	nativeNodes := native.Admission.Admission.NodeIdentities()
	if len(nativeOccurrences) != 2 {
		t.Fatal("ASSERT_NATIVE_ADMITTED_OCCURRENCES")
	}
	for _, original := range nativeOccurrences {
		matches := 0
		for _, o := range out.TypedInput.Occurrences {
			if o.Kind == pa.TypedCalls && o.Identity == original.Identity {
				matches++
				if o.RelationID != original.RelationID || o.Ordinal != original.Ordinal || o.From != nativeNodes[original.From] || o.To != nativeNodes[original.To] || original.Weight != 1 {
					t.Fatal("ASSERT_NATIVE_OCCURRENCE_IDENTITY")
				}
			}
		}
		if matches != 1 {
			t.Fatal("ASSERT_NATIVE_OCCURRENCE_IDENTITY", original.Identity)
		}
	}
	receipt, err := json.Marshal(native.Admission.Admission.SourceBinding())
	if err != nil || !bytes.Equal(append(receipt, '\n'), roles["ACQUISITION_RECEIPT"]) {
		t.Fatal("ASSERT_ACQUISITION_RECEIPT_CUSTODY", err)
	}
	for _, pair := range []struct {
		selector string
		raw      []byte
	}{{n.state.summary.CompositeSelector, native.Composite.Bytes}, {n.state.summary.ProofSelector, native.nativeEvidence}} {
		raw, err := publication.ReadVerifiedBoundFile(f.root, pair.selector, 1<<20)
		if err != nil || !bytes.Equal(raw, pair.raw) {
			t.Fatal("ASSERT_NATIVE_PUBLICATION", err)
		}
	}
	t.Logf("identity session=%s generation=%d source=%s occurrence=%s containing=%s B=%s", session.SessionID, session.Generation, v.Query.SourceDigest, from, v.Target.SymbolID, endpoint)
}
