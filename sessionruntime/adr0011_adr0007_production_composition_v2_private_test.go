package sessionruntime_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"lsp-trace/incomingops"
	adr0007 "lsp-trace/internal/adr0007sourcetextsearchv4private"
	composition "lsp-trace/internal/adr0011adr0007composition"
	compositionruntime "lsp-trace/internal/adr0011adr0007compositionruntime"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

const fixedWorkspaceURIPrivate = "file:///private/tmp/adr0011-fixed-workspace/calls.go"
const fixedWorkspacePathPrivate = "/private/tmp/adr0011-fixed-workspace"
const expectedIncomingRequestIDPrivate = uint64(4)
const expectedOutgoingRequestIDPrivate = uint64(7)
const expectedIncomingRequestBodyPrivate = `{"id":4,"jsonrpc":"2.0","method":"callHierarchy/incomingCalls","params":{"item":{"name":"Target","kind":12,"uri":"file:///private/tmp/adr0011-fixed-workspace/calls.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}}}}}`
const expectedOutgoingRequestBodyPrivate = `{"id":7,"jsonrpc":"2.0","method":"callHierarchy/outgoingCalls","params":{"item":{"name":"Target","kind":12,"uri":"file:///private/tmp/adr0011-fixed-workspace/calls.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}}}}}`
const expectedIncomingRequestBytesPrivate = 350
const expectedIncomingResponseBytesPrivate = 33554082
const expectedOutgoingRequestBytesPrivate = 350
const expectedOutgoingResponseBytesPrivate = 33554082
const expectedIncomingTotalBytesPrivate = expectedIncomingRequestBytesPrivate + expectedIncomingResponseBytesPrivate
const expectedOutgoingTotalBytesPrivate = expectedOutgoingRequestBytesPrivate + expectedOutgoingResponseBytesPrivate
const expectedRootTotalBytesPrivate = expectedIncomingTotalBytesPrivate + expectedOutgoingTotalBytesPrivate
const exactDirectionBytesPrivate = expectedIncomingTotalBytesPrivate
const expectedIncomingRequestSHA256Private = "6597867748ca95c13d93ce4713a66eac7298dde97e2676e921c57b8eaac82b8b"
const expectedIncomingResponseSHA256Private = "17a5c51335cca2598952aaee84aedb785cd9596d3a0f5aedeaba92bc0459c18a"
const expectedOutgoingRequestSHA256Private = "16b99dd104b9833d42418376e02a9271a2246c91f91c83bde2f00819364d6430"
const expectedOutgoingResponseSHA256Private = "76a3bae9a074b33df6cad1c7c4f52bfcf84d853967f3214d687b31b5cbe135f5"

type wireRecordPrivate struct {
	Method string
	Kind   string
	ID     uint64
	Raw    []byte
	Bytes  uint64
	SHA256 [32]byte
}

type scriptedTransportPrivate struct {
	input               *io.PipeReader
	stdin               *io.PipeWriter
	output              *io.PipeWriter
	stdout              *io.PipeReader
	mu                  sync.Mutex
	records             []wireRecordPrivate
	incomingVariant     string
	outgoingVariant     string
	exactDirectional    bool
	prepareItem         []byte
	cancelAfterIncoming context.CancelFunc
	directionalEntered  chan wireRecordPrivate
	directionalRelease  chan struct{}
	directionalBlock    sync.Once
}

func newScriptedTransportPrivate(incomingVariant, outgoingVariant string, exact bool) *scriptedTransportPrivate {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &scriptedTransportPrivate{input: input, stdin: stdin, output: output, stdout: stdout, incomingVariant: incomingVariant, outgoingVariant: outgoingVariant, exactDirectional: exact, prepareItem: callItemPrivate()}
	go child.serve()
	return child
}

func (c *scriptedTransportPrivate) record(method, kind string, id uint64, raw []byte) {
	copyRaw := append([]byte(nil), raw...)
	c.mu.Lock()
	c.records = append(c.records, wireRecordPrivate{Method: method, Kind: kind, ID: id, Raw: copyRaw, Bytes: uint64(len(copyRaw)), SHA256: sha256.Sum256(copyRaw)})
	c.mu.Unlock()
}

func (c *scriptedTransportPrivate) writeResponse(method string, id uint64, raw []byte) {
	c.record(method, "response", id, raw)
	_, _ = c.output.Write(raw)
}

func framePrivate(body []byte) []byte {
	return append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
}

func resultFramePrivate(id uint64, result []byte) []byte {
	body := append([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":`, id)), result...)
	body = append(body, '}')
	return framePrivate(body)
}

func errorFramePrivate(id uint64, message string) []byte {
	return framePrivate([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"error":{"code":-32603,"message":%q}}`, id, message)))
}

func callItemPrivate() []byte {
	return []byte(`{"name":"Target","kind":12,"uri":"file:///private/tmp/adr0011-fixed-workspace/calls.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}}}`)
}

func sizedDirectionalResponsePrivate(id uint64, direction string, total int) []byte {
	endpoint := "from"
	if direction == "outgoing" {
		endpoint = "to"
	}
	for padding := total; padding >= 0; padding-- {
		item := bytes.Replace(callItemPrivate(), []byte(`"kind":12,`), []byte(`"kind":12,"detail":"`+string(bytes.Repeat([]byte{'x'}, padding))+`",`), 1)
		result := []byte(`[{"` + endpoint + `":` + string(item) + `,"fromRanges":[]}]`)
		frame := resultFramePrivate(id, result)
		if len(frame) == total {
			return frame
		}
		if len(frame) < total {
			padding += total - len(frame) + 1
		}
	}
	panic("unable to size directional response")
}

func (c *scriptedTransportPrivate) serve() {
	reader := lspwire.NewReader(c.input, lspwire.Limits{MaxBodyBytes: 64 << 20, MaxHeaderBytes: 65536})
	for {
		message, _, raw, _, err := reader.ReadWithFrameIfWithin(64 << 20)
		if err != nil {
			return
		}
		id, _ := strconv.ParseUint(string(message.ID), 10, 64)
		c.record(message.Method, "request", id, raw)
		switch message.Method {
		case "initialize":
			c.writeResponse(message.Method, id, resultFramePrivate(id, []byte(`{"capabilities":{"positionEncoding":"utf-16","callHierarchyProvider":true}}`)))
		case "initialized", "textDocument/didOpen", "textDocument/didChange":
			continue
		case "textDocument/documentSymbol":
			c.writeResponse(message.Method, id, resultFramePrivate(id, []byte(`[{"name":"Target","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}}}]`)))
		case "textDocument/prepareCallHierarchy":
			c.writeResponse(message.Method, id, resultFramePrivate(id, append(append([]byte{'['}, c.prepareItem...), ']')))
		case "callHierarchy/incomingCalls":
			c.directionalReply(id, "incoming", c.incomingVariant, len(raw))
			if c.cancelAfterIncoming != nil {
				c.cancelAfterIncoming()
			}
		case "callHierarchy/outgoingCalls":
			c.directionalReply(id, "outgoing", c.outgoingVariant, len(raw))
		case "shutdown":
			c.writeResponse(message.Method, id, resultFramePrivate(id, []byte(`null`)))
		case "exit":
			return
		}
	}
}

func (c *scriptedTransportPrivate) directionalReply(id uint64, direction, variant string, requestBytes int) {
	if c.directionalEntered != nil && c.directionalRelease != nil {
		c.directionalBlock.Do(func() {
			c.directionalEntered <- wireRecordPrivate{Method: "callHierarchy/" + direction + "Calls", Kind: "request", ID: id}
			<-c.directionalRelease
		})
	}
	if variant == "error" {
		c.writeResponse("callHierarchy/"+direction+"Calls", id, errorFramePrivate(id, direction+" scripted failure"))
		return
	}
	if variant == "empty" {
		c.writeResponse("callHierarchy/"+direction+"Calls", id, resultFramePrivate(id, []byte(`[]`)))
		return
	}
	if c.exactDirectional {
		c.writeResponse("callHierarchy/"+direction+"Calls", id, sizedDirectionalResponsePrivate(id, direction, exactDirectionBytesPrivate-requestBytes))
		return
	}
	endpoint := "from"
	if direction == "outgoing" {
		endpoint = "to"
	}
	c.writeResponse("callHierarchy/"+direction+"Calls", id, resultFramePrivate(id, []byte(`[{"`+endpoint+`":`+string(callItemPrivate())+`,"fromRanges":[]}]`)))
}

func (c *scriptedTransportPrivate) Stdin() io.WriteCloser { return c.stdin }
func (c *scriptedTransportPrivate) Stdout() io.ReadCloser { return c.stdout }
func (c *scriptedTransportPrivate) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *scriptedTransportPrivate) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func mustHashPrivate(t *testing.T, encoded string) [32]byte {
	t.Helper()
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != sha256.Size {
		t.Fatalf("invalid frozen SHA-256 pin %q: %v", encoded, err)
	}
	var out [32]byte
	copy(out[:], raw)
	return out
}

func (c *scriptedTransportPrivate) snapshot() []wireRecordPrivate {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]wireRecordPrivate(nil), c.records...)
}

func (c *scriptedTransportPrivate) bytesFor(method, kind string) uint64 {
	var total uint64
	for _, record := range c.snapshot() {
		if record.Method == method && record.Kind == kind {
			total += record.Bytes
		}
	}
	return total
}

func (c *scriptedTransportPrivate) countFor(method, kind string) int {
	count := 0
	for _, record := range c.snapshot() {
		if record.Method == method && record.Kind == kind {
			count++
		}
	}
	return count
}

type starterPrivate struct{ child sessionruntime.Child }

func (s starterPrivate) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func readyManagerPrivate(t *testing.T, child *scriptedTransportPrivate) (*sessionruntime.Manager, sessionruntime.StartResult, string) {
	t.Helper()
	workspace := fixedWorkspacePathPrivate
	if err := os.RemoveAll(workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workspace) })
	path := filepath.Join(workspace, "calls.go")
	if err := os.WriteFile(path, []byte("package calls\nfunc Target() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri, err := source.FileURI(path)
	if err != nil || uri != fixedWorkspaceURIPrivate {
		t.Fatalf("ASSERT_FIXED_WORKSPACE_URI_MATCHES_FROZEN_INPUT: uri=%q err=%v", uri, err)
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Wire: lspwire.Limits{MaxBodyBytes: 64 << 20, MaxHeaderBytes: 65536}, Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 16, MaxChildren: 2, MaxCancels: 16, MaxTombstones: 32, MaxObservations: 256, MaxOperations: 16}, Starter: starterPrivate{child: child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Shutdown(context.Background()) })
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), LanguageID: "go"})
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(2*time.Second))
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
		t.Fatalf("ASSERT_READY_INITIALIZE_COMPLETES: found=%t ready=%+v", found, ready)
	}
	records := manager.Records()
	if len(records) != 1 || records[0].State != session.Ready || records[0].Generation != started.Generation {
		t.Fatalf("ASSERT_READY_RECORD_EXACT_GENERATION: %+v", records)
	}
	deadline := time.Now().Add(time.Second)
	for len(child.snapshot()) < 3 {
		if time.Now().After(deadline) {
			t.Fatal("ASSERT_STARTUP_INITIALIZED_HANDOFF")
		}
		runtime.Gosched()
	}
	startup := child.snapshot()
	if len(startup) != 3 || startup[0].Method != "initialize" || startup[0].Kind != "request" || startup[1].Method != "initialize" || startup[1].Kind != "response" || startup[2].Method != "initialized" || startup[2].Kind != "request" {
		t.Fatalf("ASSERT_STARTUP_EXACT_METHOD_SEQUENCE: %+v", startup)
	}
	return manager, started, uri
}

func attemptPrivate() adr0007.Attempt {
	content := []byte("needle alpha needle beta needle\n")
	digest := adr0007.Digest(content)
	return adr0007.Attempt{SchemaVersion: adr0007.AttemptSchema, AttemptID: "attempt-external-end-to-end", Request: adr0007.Request{SchemaVersion: adr0007.RequestSchema, Query: "needle", Sources: []adr0007.SourceRef{{Ordinal: 1, Path: "tmp/adr0011-e2e.go", Revision: "rev", FileDigest: digest, ObjectDigest: digest}}, Policy: adr0007.Policy{SchemaVersion: adr0007.PolicySchema, LiteralMode: "byte-literal"}, Limits: adr0007.Limits{SchemaVersion: adr0007.LimitsSchema, MaxFiles: 1, MaxMatches: 10, MaxWork: 1_000_000, MaxOutputBytes: 100_000, MaxSourceBytes: 1000, MaxTotalBytes: 1000, MaxPathBytes: 1000}, LocationPin: adr0007.LocationPin{SchemaVersion: adr0007.LocationPinSchema, Operation: "RANGE_UNION", ExecutedLocation: false, DesignCommit: "c943a484060462121c6f0929d4182b053ff95ab5", DesignRootSha256: "sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d", ExecutionCommit: "f0f8b49aa368bea9b3e6d105eef5cb2614221067", SealCommit: "16f40dcb03a234b00db80059a7eef400495e9d97", FinalSealSha256: "sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6", SourceAdmissionPin: adr0007.SourceAdmissionPin{RepositoryCommit: "af2ce89321afc94c937636f841bb98b8977b6496", Path: "internal/sourceadmissionv2/admission.go", Bytes: 3519, SHA256: "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da", GitBlobSHA1: "4953fab89d911e2352fa6c2253497c07c8e9a777", Symbols: []string{"Admit", "CanonicalPath", "Digest"}}, CompleteLocationPins: []string{"design_commit", "design_root_sha256", "execution_commit", "seal_commit", "final_seal_sha256", "source_admission_pin"}}}, ExecutionControl: adr0007.ExecutionControl{SchemaVersion: adr0007.ExecutionControlSchema}, SourceInputs: []adr0007.SourceInput{{SchemaVersion: adr0007.SourceInputSchema, Ordinal: 1, Path: "tmp/adr0011-e2e.go", Revision: "rev", FileDigest: digest, ObjectDigest: digest, BytesBase64: "bmVlZGxlIGFscGhhIG5lZWRsZSBiZXRhIG5lZWRsZQo="}}}
}

func privateRequestPrivate(t *testing.T, started sessionruntime.StartResult, uri string, plan directionalraw.BidirectionalPlanPrivate, explicit bool) incomingops.ProductionPrivateRequest {
	t.Helper()
	relations := ""
	if explicit {
		relations = `,"relations":["CALLS"]`
	}
	op := operation.Request{Name: incomingops.OperationIncoming, Input: json.RawMessage(fmt.Sprintf(`{"session_id":%q,"generation":%d,"uri":%q,"symbol":"Target"%s,"max_depth":1,"max_nodes":8,"timeout_ms":5000,"request_timeout_ms":2000}`, started.SessionID, started.Generation, uri, relations))}
	attempt := attemptPrivate()
	canonical := bytes.TrimSpace(adr0007.Canon(attempt))
	raw := append([]byte(" \n\t"), canonical...)
	raw = append(raw, '\n')
	terminal := adr0007.EvaluateRaw(raw)
	ids := terminal.RangeUnionCandidate.MemberMatchIDs
	root, err := directionalraw.NewBidirectionalRootPrivate(directionalraw.NewReservation("external-e2e"), plan)
	if errors.Is(err, directionalraw.ErrBehaviorNotImplementedPrivate) {
		t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_EXTERNAL_E2E")
	}
	if err != nil {
		t.Fatal(err)
	}
	facets := []compositionruntime.FacetObservationPrivate{{Facet: "D", ObservationState: "OBSERVED_NONEMPTY"}, {Facet: "R", ObservationState: "OBSERVED_EMPTY"}, {Facet: "CALLS_INCOMING", ObservationState: "OBSERVED_NONEMPTY"}, {Facet: "CALLS_OUTGOING", ObservationState: "OBSERVED_NONEMPTY"}}
	marks := []composition.DependencyMark{{Direction: "incoming", MatchID: ids[0], Action: composition.ActionOrder, CandidateOrdinal: 0, OneFinalDispositionOrdinals: composition.CanonicalOneFinalDispositionOrdinals, OrdinalIdentitySHA256: composition.IncomingOrdinalIdentitySHA256, RelationKind: "CALLS", ProfileID: composition.IncomingProfileID, EvidenceSHA256: composition.IncomingEvidenceSHA256}, {Direction: "outgoing", MatchID: ids[1], Action: composition.ActionOrder, CandidateOrdinal: 1, OneFinalDispositionOrdinals: composition.CanonicalOneFinalDispositionOrdinals, OrdinalIdentitySHA256: composition.OutgoingOrdinalIdentitySHA256, RelationKind: "CALLS", ProfileID: composition.OutgoingProfileID, EvidenceSHA256: composition.OutgoingEvidenceSHA256}}
	return incomingops.ProductionPrivateRequest{Operation: op, Coordinate: compositionruntime.CoordinateRequestPrivate{Attempt: attempt, RawAttempt: raw, Envelopes: []composition.DirectionEnvelope{composition.AcceptedSuccessEnvelope("incoming"), composition.AcceptedSuccessEnvelope("outgoing")}, Marks: marks, Facets: facets, Root: root}}
}

func executePrivate(t *testing.T, ctx context.Context, manager *sessionruntime.Manager, req incomingops.ProductionPrivateRequest) incomingops.ProductionPrivateResult {
	t.Helper()
	entry, err := incomingops.NewADR0011ADR0007ProductionPrivate(incomingops.NewExecutor(manager))
	if errors.Is(err, incomingops.ErrBehaviorNotImplementedPrivate) {
		t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_EXTERNAL_ENTRY")
	}
	if err != nil {
		t.Fatal(err)
	}
	result, err := entry.ExecutePrivate(ctx, req)
	if errors.Is(err, incomingops.ErrBehaviorNotImplementedPrivate) {
		t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_EXTERNAL_EXECUTE")
	}
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestADR0011ADR0007ProductionPrivateFalseTerminalReleasePropagates(t *testing.T) {
	manager, started, uri := readyManagerPrivate(t, newScriptedTransportPrivate("empty", "empty", false))
	req := privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 1, OutgoingBytes: 1}, false)
	reservation := directionalraw.NewReservation("false-terminal-production")
	terminalCalls := 0
	if !reservation.AttachPrivate(func(string, string, int) ([]byte, error) { return nil, nil }, func(directionalraw.Observation) {}, func() bool {
		terminalCalls++
		return false
	}, func() {}) {
		t.Fatal("ASSERT_FALSE_TERMINAL_PRODUCTION_ATTACH")
	}
	root, err := directionalraw.NewBidirectionalRootPrivate(reservation, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 1, OutgoingBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	req.Coordinate.Root = root
	entry, err := incomingops.NewADR0011ADR0007ProductionPrivate(incomingops.NewExecutor(manager))
	if err != nil {
		t.Fatal(err)
	}
	result, err := entry.ExecutePrivate(context.Background(), req)
	if !errors.Is(err, directionalraw.ErrTerminalReleasePrivate) || result.FailurePrivate() == nil || result.FailurePrivate().Code != "DIRECTIONAL_RELEASE_FAILED" {
		t.Fatalf("ASSERT_FALSE_TERMINAL_PRODUCTION_PROPAGATED: result=%+v err=%v", result, err)
	}
	snapshot, _ := root.SnapshotPrivate()
	if terminalCalls != 1 || snapshot.Released || snapshot.ReleaseCount != 0 {
		t.Fatalf("ASSERT_FALSE_TERMINAL_PRODUCTION_EXACT_ONCE: calls=%d snapshot=%+v", terminalCalls, snapshot)
	}
}

func targetRecordsPrivate(records []wireRecordPrivate) []wireRecordPrivate {
	var out []wireRecordPrivate
	for _, record := range records {
		if record.Method != "initialize" && record.Method != "initialized" && record.Method != "shutdown" && record.Method != "exit" {
			out = append(out, record)
		}
	}
	return out
}

func assertExactWirePrivate(t *testing.T, manager *sessionruntime.Manager, child *scriptedTransportPrivate) {
	t.Helper()
	records := targetRecordsPrivate(child.snapshot())
	methods := []string{"textDocument/didOpen", "textDocument/documentSymbol", "textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls", "callHierarchy/outgoingCalls"}
	if len(records) != len(methods) {
		t.Fatalf("ASSERT_WIRE_EXACT_METHOD_ORDER_EXCLUDING_STARTUP: got=%d records=%+v", len(records), records)
	}
	for i, record := range records {
		if record.Method != methods[i] || record.Bytes != uint64(len(record.Raw)) || record.SHA256 != sha256.Sum256(record.Raw) {
			t.Fatalf("ASSERT_WIRE_RECORD_SELF_CONSISTENCY[%d]: %+v", i, record)
		}
	}
	directional := make([]wireRecordPrivate, 0, 4)
	for _, record := range records {
		if record.Method == "callHierarchy/incomingCalls" || record.Method == "callHierarchy/outgoingCalls" {
			directional = append(directional, record)
		}
	}
	if len(directional) != 4 {
		t.Fatalf("ASSERT_WIRE_DIRECTIONAL_CARDINALITY: got=%d", len(directional))
	}
	for i, want := range []struct{ method, kind string }{{"callHierarchy/incomingCalls", "request"}, {"callHierarchy/incomingCalls", "response"}, {"callHierarchy/outgoingCalls", "request"}, {"callHierarchy/outgoingCalls", "response"}} {
		if directional[i].Method != want.method || directional[i].Kind != want.kind {
			t.Fatalf("ASSERT_WIRE_DIRECTIONAL_ORDER[%d]: got=%+v want=%+v", i, directional[i], want)
		}
	}
	if directional[0].ID != directional[1].ID || directional[2].ID != directional[3].ID || directional[0].ID == directional[2].ID {
		t.Fatalf("ASSERT_WIRE_REQUEST_RESPONSE_KEY_CUSTODY: %+v", directional)
	}
	var observations []directionalraw.Observation
	for _, observation := range manager.DirectionalObservationsPrivate() {
		if observation.Method == "callHierarchy/incomingCalls" || observation.Method == "callHierarchy/outgoingCalls" {
			observations = append(observations, observation)
		}
	}
	if len(observations) != len(directional) {
		t.Fatalf("ASSERT_WIRE_MANAGER_OBSERVATION_CARDINALITY: got=%d want=%d", len(observations), len(directional))
	}
	used := make([]bool, len(observations))
	for i, record := range directional {
		matches := 0
		for j, observation := range observations {
			kind := "request"
			if observation.Component == "response-frame" {
				kind = "response"
			} else if observation.Component != "request-frame" {
				continue
			}
			if !used[j] && record.Method == observation.Method && record.Kind == kind && record.Bytes == observation.Length && record.SHA256 == observation.SHA256 {
				used[j] = true
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("ASSERT_WIRE_MANAGER_BIJECTION_NO_REUSED_MATCHES[%d]: matches=%d used=%v", i, matches, used)
		}
	}
	matched := 0
	for _, value := range used {
		if value {
			matched++
		}
	}
	if matched != len(directional) {
		t.Fatalf("ASSERT_WIRE_MANAGER_BIJECTION_CARDINALITY: matched=%d directional=%d", matched, len(directional))
	}
	preparation := child.bytesFor("textDocument/documentSymbol", "request") + child.bytesFor("textDocument/documentSymbol", "response") + child.bytesFor("textDocument/prepareCallHierarchy", "request") + child.bytesFor("textDocument/prepareCallHierarchy", "response")
	if preparation == 0 {
		t.Fatal("ASSERT_PREPARATION_ACCOUNTING_EXPLICIT_AND_EXCLUDED_FROM_DIRECTION_TOTALS")
	}
}

func retryWirePinPrivate(t *testing.T, scenario string) (sequence, failureDirectional, retryDirectional []wireRecordPrivate) {
	t.Helper()
	sequence = []wireRecordPrivate{
		{Method: "textDocument/documentSymbol", Kind: "request"},
		{Method: "textDocument/documentSymbol", Kind: "response"},
		{Method: "textDocument/prepareCallHierarchy", Kind: "request"},
		{Method: "textDocument/prepareCallHierarchy", Kind: "response"},
		{Method: "callHierarchy/incomingCalls", Kind: "request"},
		{Method: "callHierarchy/incomingCalls", Kind: "response"},
		{Method: "textDocument/documentSymbol", Kind: "request"},
		{Method: "textDocument/documentSymbol", Kind: "response"},
		{Method: "textDocument/prepareCallHierarchy", Kind: "request"},
		{Method: "textDocument/prepareCallHierarchy", Kind: "response"},
		{Method: "callHierarchy/outgoingCalls", Kind: "request"},
		{Method: "callHierarchy/outgoingCalls", Kind: "response"},
	}
	switch scenario {
	case "cancel-before":
		sequence = append([]wireRecordPrivate{{Method: "textDocument/didOpen", Kind: "request"}}, sequence...)
		retryDirectional = []wireRecordPrivate{
			{Method: "callHierarchy/incomingCalls", Kind: "request", Bytes: 350, SHA256: mustHashPrivate(t, "6597867748ca95c13d93ce4713a66eac7298dde97e2676e921c57b8eaac82b8b")},
			{Method: "callHierarchy/incomingCalls", Kind: "response", Bytes: 329, SHA256: mustHashPrivate(t, "6c515127b5bdd30e36cc1e20fc483180016ff417914e0bb05bc50ffcb8b2c38f")},
			{Method: "callHierarchy/outgoingCalls", Kind: "request", Bytes: 350, SHA256: mustHashPrivate(t, "16b99dd104b9833d42418376e02a9271a2246c91f91c83bde2f00819364d6430")},
			{Method: "callHierarchy/outgoingCalls", Kind: "response", Bytes: 327, SHA256: mustHashPrivate(t, "6ff9e9833f36a803a73409625f7e0098557e5ed3c504a183fd90a96b70df89fa")},
		}
	case "cancel-between":
		failureDirectional = []wireRecordPrivate{
			{Method: "callHierarchy/incomingCalls", Kind: "request", Bytes: 350, SHA256: mustHashPrivate(t, "6597867748ca95c13d93ce4713a66eac7298dde97e2676e921c57b8eaac82b8b")},
			{Method: "callHierarchy/incomingCalls", Kind: "response", Bytes: 329, SHA256: mustHashPrivate(t, "6c515127b5bdd30e36cc1e20fc483180016ff417914e0bb05bc50ffcb8b2c38f")},
		}
		retryDirectional = []wireRecordPrivate{
			{Method: "callHierarchy/incomingCalls", Kind: "request", Bytes: 350, SHA256: mustHashPrivate(t, "91f39d226cb32cbaf0dbde7e460adef2feebf5be77b761d2aee57d7d479716c9")},
			{Method: "callHierarchy/incomingCalls", Kind: "response", Bytes: 329, SHA256: mustHashPrivate(t, "ef213c1234df5de6b77c10767fe0d3765779fb94c1638913b0f4365649d8757e")},
			{Method: "callHierarchy/outgoingCalls", Kind: "request", Bytes: 351, SHA256: mustHashPrivate(t, "51e8d19750bfc22b50cdac7eb1cf9023055e6cac0ce66bc5664dfb1e224aecdd")},
			{Method: "callHierarchy/outgoingCalls", Kind: "response", Bytes: 328, SHA256: mustHashPrivate(t, "496eb91fe7519d088f2411a893086de734b16d07dfaf08f3e06e657e3f18e434")},
		}
	case "partial-outgoing-error":
		failureDirectional = []wireRecordPrivate{
			{Method: "callHierarchy/incomingCalls", Kind: "request", Bytes: 350, SHA256: mustHashPrivate(t, "6597867748ca95c13d93ce4713a66eac7298dde97e2676e921c57b8eaac82b8b")},
			{Method: "callHierarchy/incomingCalls", Kind: "response", Bytes: 329, SHA256: mustHashPrivate(t, "6c515127b5bdd30e36cc1e20fc483180016ff417914e0bb05bc50ffcb8b2c38f")},
			{Method: "callHierarchy/outgoingCalls", Kind: "request", Bytes: 350, SHA256: mustHashPrivate(t, "16b99dd104b9833d42418376e02a9271a2246c91f91c83bde2f00819364d6430")},
			{Method: "callHierarchy/outgoingCalls", Kind: "response", Bytes: 108, SHA256: mustHashPrivate(t, "c629aaa98728cb3405b713f558e05f0d14b83798f338b650d0ab253eb19019bc")},
		}
		retryDirectional = []wireRecordPrivate{
			{Method: "callHierarchy/incomingCalls", Kind: "request", Bytes: 351, SHA256: mustHashPrivate(t, "0a6b1855344704b7271c3e46bc815e0130c31f5f420800c2dce329da8db178a7")},
			{Method: "callHierarchy/incomingCalls", Kind: "response", Bytes: 330, SHA256: mustHashPrivate(t, "49bd6532e4cde2fb5164af0aa50664146173ffbce30b91d80e66788da0dc413f")},
			{Method: "callHierarchy/outgoingCalls", Kind: "request", Bytes: 351, SHA256: mustHashPrivate(t, "2721ab48c58790768490216cdbfb0cf319c8ac4c3a0449b1f79bf830fe0ccbfd")},
			{Method: "callHierarchy/outgoingCalls", Kind: "response", Bytes: 328, SHA256: mustHashPrivate(t, "2276b19223a031364439e09468305350af13dbea9439667b41a62c7ae38443a4")},
		}
	default:
		t.Fatalf("ASSERT_RETRY_WIRE_FROZEN_SCENARIO: %q", scenario)
	}
	return sequence, failureDirectional, retryDirectional
}

type retryAccountingPinPrivate struct {
	FailureIncoming, FailureOutgoing, FailureRoot uint64
	RetryIncoming, RetryOutgoing, RetryRoot       uint64
	ManagerCumulative                             uint64
}

func frozenDirectionalTotalsPrivate(records []wireRecordPrivate, marker string) (incoming, outgoing, root uint64) {
	for _, record := range records {
		switch record.Method {
		case "callHierarchy/incomingCalls":
			incoming += record.Bytes
		case "callHierarchy/outgoingCalls":
			outgoing += record.Bytes
		default:
			panic(marker + ": unexpected directional method " + record.Method)
		}
	}
	return incoming, outgoing, incoming + outgoing
}

func retryAccountingPinPrivateFor(t *testing.T, scenario string) retryAccountingPinPrivate {
	t.Helper()
	_, failureDirectional, retryDirectional := retryWirePinPrivate(t, scenario)
	failureIncoming, failureOutgoing, failureRoot := frozenDirectionalTotalsPrivate(failureDirectional, "ASSERT_RETRY_ACCOUNTING_FROZEN_FIRST_ATTEMPT")
	retryIncoming, retryOutgoing, retryRoot := frozenDirectionalTotalsPrivate(retryDirectional, "ASSERT_RETRY_ACCOUNTING_FROZEN_SUCCESSFUL_RETRY")
	return retryAccountingPinPrivate{
		FailureIncoming: failureIncoming, FailureOutgoing: failureOutgoing, FailureRoot: failureRoot,
		RetryIncoming: retryIncoming, RetryOutgoing: retryOutgoing, RetryRoot: retryRoot,
		ManagerCumulative: failureRoot + retryRoot,
	}
}

func directionalOnlyObservationsPrivate(values []directionalraw.Observation) []directionalraw.Observation {
	var out []directionalraw.Observation
	for _, observation := range values {
		if observation.Method == "callHierarchy/incomingCalls" || observation.Method == "callHierarchy/outgoingCalls" {
			out = append(out, observation)
		}
	}
	return out
}

func assertExactRetryWirePrivate(t *testing.T, child *scriptedTransportPrivate, start int, observations []directionalraw.Observation, scenario, marker string) {
	t.Helper()
	all := child.snapshot()
	if start < 0 || start > len(all) {
		t.Fatalf("%s: invalid start=%d records=%d", marker, start, len(all))
	}
	retry := targetRecordsPrivate(all[start:])
	counts := map[string]int{}
	directional := make([]wireRecordPrivate, 0, 4)
	for _, record := range retry {
		counts[record.Method+"/"+record.Kind]++
		if record.Bytes != uint64(len(record.Raw)) || record.SHA256 != sha256.Sum256(record.Raw) {
			t.Fatalf("%s: non-authentic record %+v", marker, record)
		}
		if record.Method == "callHierarchy/incomingCalls" || record.Method == "callHierarchy/outgoingCalls" {
			directional = append(directional, record)
		}
	}
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		if counts[method+"/request"] != 1 || counts[method+"/response"] != 1 {
			t.Fatalf("%s: exact retry method count %s records=%+v", marker, method, retry)
		}
	}
	if len(directional) != 4 || len(observations) != 4 {
		t.Fatalf("ASSERT_RETRY_WIRE_DIRECTIONAL_CARDINALITY[%s]: records=%d observations=%d", scenario, len(directional), len(observations))
	}
	used := make([]bool, len(observations))
	for i, record := range directional {
		matches := 0
		for j, observation := range observations {
			kind := "request"
			if observation.Component == "response-frame" {
				kind = "response"
			} else if observation.Component != "request-frame" {
				continue
			}
			if !used[j] && record.Method == observation.Method && record.Kind == kind && record.Bytes == observation.Length && record.SHA256 == observation.SHA256 {
				used[j] = true
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("ASSERT_RETRY_WIRE_MANAGER_BIJECTION_NO_REUSED_MATCHES[%s][%d]: matches=%d used=%v", scenario, i, matches, used)
		}
	}
}

func TestScriptedTransportDirectionalVariantsPrivate(t *testing.T) {
	cases := []struct {
		name, incoming, outgoing string
		wantFailure              bool
	}{
		{name: "incoming-nonempty-outgoing-empty", incoming: "nonempty", outgoing: "empty"},
		{name: "incoming-empty-outgoing-nonempty", incoming: "empty", outgoing: "nonempty"},
		{name: "incoming-error", incoming: "error", outgoing: "nonempty", wantFailure: true},
		{name: "outgoing-error", incoming: "nonempty", outgoing: "error", wantFailure: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			child := newScriptedTransportPrivate(tc.incoming, tc.outgoing, false)
			manager, started, uri := readyManagerPrivate(t, child)
			result := executePrivate(t, context.Background(), manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false))
			if (result.FailurePrivate() != nil) != tc.wantFailure {
				t.Fatalf("ASSERT_SCRIPTED_NONEMPTY_EMPTY_ERROR_VARIANTS[%s]: failure=%+v", tc.name, result.FailurePrivate())
			}
			if child.countFor("callHierarchy/incomingCalls", "request") != 1 {
				t.Fatalf("ASSERT_VARIANT_INCOMING_REAL_TRANSPORT[%s]: records=%+v", tc.name, child.snapshot())
			}
			if tc.incoming == "error" {
				if child.countFor("callHierarchy/outgoingCalls", "request") != 0 {
					t.Fatal("ASSERT_INCOMING_ERROR_STOPS_OUTGOING")
				}
			} else if child.countFor("callHierarchy/outgoingCalls", "request") != 1 {
				t.Fatal("ASSERT_VARIANT_OUTGOING_REAL_TRANSPORT")
			}
		})
	}
}

func TestDirectionalPreparedItemExactLexicalIdentityPrivate(t *testing.T) {
	child := newScriptedTransportPrivate("nonempty", "nonempty", false)
	child.prepareItem = []byte(`{ "data" : {"n":1e0,"escaped":"\u0054arget"}, "selectionRange":{"end":{"character":6,"line":0},"start":{"character":0,"line":0}}, "range" : {"end":{"character":6,"line":0},"start":{"character":0,"line":0}}, "uri":"file:///private/tmp/adr0011-fixed-workspace/calls.go", "kind":12, "name":"Target" }`)
	manager, started, uri := readyManagerPrivate(t, child)
	result := executePrivate(t, context.Background(), manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false))
	if result.FailurePrivate() != nil {
		t.Fatalf("ASSERT_NONCANONICAL_PREPARE_EXECUTION: %+v", result.FailurePrivate())
	}
	if child.countFor("textDocument/prepareCallHierarchy", "request") != 1 {
		t.Fatalf("ASSERT_NONCANONICAL_EXACTLY_ONE_PREPARATION: %+v", child.snapshot())
	}
	seen := 0
	for _, record := range child.snapshot() {
		if record.Kind != "request" || (record.Method != "callHierarchy/incomingCalls" && record.Method != "callHierarchy/outgoingCalls") {
			continue
		}
		bodyAt := bytes.Index(record.Raw, []byte("\r\n\r\n"))
		if bodyAt < 0 {
			t.Fatal("ASSERT_NONCANONICAL_REQUEST_FRAME_BODY")
		}
		var envelope struct {
			Params struct {
				Item json.RawMessage `json:"item"`
			} `json:"params"`
		}
		if err := json.Unmarshal(record.Raw[bodyAt+4:], &envelope); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(envelope.Params.Item, child.prepareItem) {
			t.Fatalf("ASSERT_NONCANONICAL_PREPARED_TOKEN_EXACT[%s]: got=%s want=%s", record.Method, envelope.Params.Item, child.prepareItem)
		}
		var item struct{ Name, URI string }
		if err := json.Unmarshal(envelope.Params.Item, &item); err != nil || item.Name != "Target" || item.URI != uri {
			t.Fatalf("ASSERT_NONCANONICAL_TYPED_IDENTITY[%s]: item=%+v err=%v", record.Method, item, err)
		}
		seen++
	}
	if seen != 2 {
		t.Fatalf("ASSERT_NONCANONICAL_BOTH_DIRECTIONS: got=%d", seen)
	}
}

func TestDirectionalCandidateOverlapRefusalIsolationPrivate(t *testing.T) {
	child := newScriptedTransportPrivate("empty", "empty", false)
	child.directionalEntered = make(chan wireRecordPrivate, 1)
	child.directionalRelease = make(chan struct{})
	manager, started, _ := readyManagerPrivate(t, child)
	params := json.RawMessage(`{"item":{"name":"Target","kind":12,"uri":"file:///private/tmp/adr0011-fixed-workspace/calls.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}}}}`)
	firstReservation := directionalraw.NewReservation("candidate-overlap-first")
	secondReservation := directionalraw.NewReservation("candidate-overlap-second")
	type outcome struct {
		result sessionruntime.RoundTripResult
		lease  directionalraw.Lease
	}
	firstDone := make(chan outcome, 1)
	go func() {
		result, lease := manager.RoundTripDirectionalPrivate(firstReservation.Context(context.Background()), sessionruntime.RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "callHierarchy/incomingCalls", Params: params, Deadline: time.Now().Add(2 * time.Second), MaxMessages: 8, MaxBytes: 4096})
		firstDone <- outcome{result: result, lease: lease}
	}()
	entered := <-child.directionalEntered
	second, secondLease := manager.RoundTripDirectionalPrivate(secondReservation.Context(context.Background()), sessionruntime.RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "callHierarchy/incomingCalls", Params: params, Deadline: time.Now().Add(2 * time.Second), MaxMessages: 8, MaxBytes: 4096})
	if second.Failure != session.ResourceExhausted && second.Failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_OVERLAP_SECOND_CANDIDATE_DETERMINISTICALLY_REFUSED: %+v", second)
	}
	if metadata, ok := secondLease.CompletedMetadataPrivate(); ok || metadata.RequestIDPrivate != 0 {
		t.Fatalf("ASSERT_REFUSED_CANDIDATE_HAS_NO_RECEIPT: metadata=%+v ok=%t", metadata, ok)
	}
	close(child.directionalRelease)
	first := <-firstDone
	if first.result.Failure != "" || first.result.ServerError != nil {
		t.Fatalf("ASSERT_FIRST_OVERLAPPING_CANDIDATE_COMPLETES: %+v", first.result)
	}
	if err := first.lease.BindElementsPrivate(); err != nil {
		t.Fatal(err)
	}
	metadata, ok := first.lease.CompletedMetadataPrivate()
	if !ok || metadata.RequestIDPrivate != entered.ID || metadata.GenerationPrivate != started.Generation {
		t.Fatalf("ASSERT_FIRST_RECEIPT_EXACT_REQUEST_GENERATION: metadata=%+v entered=%+v generation=%d", metadata, entered, started.Generation)
	}
	observations := manager.DirectionalObservationsPrivate()
	if len(observations) == 0 {
		t.Fatal("ASSERT_FIRST_CANDIDATE_HAS_OWN_OBSERVATIONS")
	}
	for _, observation := range observations {
		if observation.RequestID != entered.ID || observation.Generation != started.Generation {
			t.Fatalf("ASSERT_NO_RECEIPT_OBSERVATION_CROSS_ATTRIBUTION: observation=%+v entered=%+v generation=%d", observation, entered, started.Generation)
		}
	}
	if !firstReservation.Release() {
		t.Fatal("ASSERT_OVERLAP_ADMITTED_RESERVATION_RELEASE")
	}
	_ = secondReservation.Release()
}

func TestADR0011ADR0007ProductionCompositionV2Private(t *testing.T) {
	t.Run("external-end-to-end-exact32-each", func(t *testing.T) {
		child := newScriptedTransportPrivate("nonempty", "nonempty", true)
		manager, started, uri := readyManagerPrivate(t, child)
		req := privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: exactDirectionBytesPrivate, OutgoingBytes: exactDirectionBytesPrivate}, false)
		req.Operation.Input = bytes.Replace(req.Operation.Input, []byte(`"request_timeout_ms":2000`), []byte(`"request_timeout_ms":60000`), 1)
		result := executePrivate(t, context.Background(), manager, req)
		if result.FailurePrivate() != nil || result.CoordinationPrivate().RefusalPrivate() != nil || !result.CoordinationPrivate().AcceptedPrivate() || result.CoordinationPrivate().CandidatePrivate() == nil {
			t.Fatalf("ASSERT_EXTERNAL_ENTRY_REAL_EXECUTOR_TO_ACCEPTED_COMPOSE: failure=%+v coordination=%+v", result.FailurePrivate(), result.CoordinationPrivate())
		}
		assertExactWirePrivate(t, manager, child)
		if child.bytesFor("callHierarchy/incomingCalls", "request")+child.bytesFor("callHierarchy/incomingCalls", "response") != exactDirectionBytesPrivate || child.bytesFor("callHierarchy/outgoingCalls", "request")+child.bytesFor("callHierarchy/outgoingCalls", "response") != exactDirectionBytesPrivate {
			t.Fatalf("ASSERT_TRANSPORT_EXACT32_INCOMING_OUTGOING: records=%+v", child.snapshot())
		}
		if live, cumulative, allocations, releases, general, occupied := manager.DirectionalAccountingPrivate(); live != 0 || cumulative != expectedRootTotalBytesPrivate || allocations != 2 || occupied || releases != 1 || general != 0 {
			t.Fatalf("ASSERT_AGGREGATE64_DIRECTION_RECEIPTS_RELEASE_NO_TRANSFER_PREP_COUNTERS: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t", live, cumulative, allocations, releases, general, occupied)
		}
		lease := result.CoordinationPrivate().LeaseStatePrivate()
		if !lease.Released || lease.IncomingAdmitted != expectedIncomingTotalBytesPrivate || lease.OutgoingAdmitted != expectedOutgoingTotalBytesPrivate || lease.RootAdmitted != expectedRootTotalBytesPrivate || lease.IncomingReleased != expectedIncomingTotalBytesPrivate || lease.OutgoingReleased != expectedOutgoingTotalBytesPrivate || lease.RootReleased != expectedRootTotalBytesPrivate {
			t.Fatalf("ASSERT_CUMULATIVE_ADMITTED_RELEASED_EXACT_IN32_OUT32_ROOT64: %+v", lease)
		}
		if lease.IncomingLive != 0 || lease.OutgoingLive != 0 || lease.RootLive != 0 || lease.RootReleaseCount != 1 {
			t.Fatalf("ASSERT_AUTOMATIC_CLEANUP_LIVE_ZERO_RELEASE_ONCE: %+v", lease)
		}
		if result.CoordinationPrivate().CandidatePrivate().Completeness != "UNKNOWN" || result.CoordinationPrivate().CandidatePrivate().FeatureIdentity != "UNRESOLVED" {
			t.Fatalf("ASSERT_UNKNOWN_UNRESOLVED: %+v", result.CoordinationPrivate())
		}
	})

	t.Run("incoming-plus-one-prewire", func(t *testing.T) {
		child := newScriptedTransportPrivate("empty", "empty", false)
		manager, started, uri := readyManagerPrivate(t, child)
		result := executePrivate(t, context.Background(), manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: exactDirectionBytesPrivate + 1, OutgoingBytes: 1}, false))
		if result.FailurePrivate() == nil || result.FailurePrivate().Code != "INCOMING_DIRECTION_LIMIT_EXCEEDED" || result.CoordinationPrivate().SuppressionPrivate() == nil || result.CoordinationPrivate().SuppressionPrivate().Code != "DIRECTION_LIMIT_EXCEEDED" || result.CoordinationPrivate().SuppressionPrivate().Stage != "incoming_preflight" || result.CoordinationPrivate().SuppressionPrivate().Direction != "incoming" || result.CoordinationPrivate().SuppressionPrivate().Family != "GENERIC" {
			t.Fatalf("ASSERT_PLUS_ONE_INCOMING_EXACT_TYPED_FAILURE: %+v", result)
		}
		if child.bytesFor("callHierarchy/incomingCalls", "request") != 0 || child.bytesFor("callHierarchy/incomingCalls", "response") != 0 || child.bytesFor("callHierarchy/outgoingCalls", "request") != 0 || child.bytesFor("callHierarchy/outgoingCalls", "response") != 0 {
			t.Fatalf("ASSERT_PLUS_ONE_ZERO_TARGET_DIRECTION_WRITE_BYTES[incoming]: records=%+v", child.snapshot())
		}
		lease := result.CoordinationPrivate().LeaseStatePrivate()
		if !lease.Released || lease.RootReleaseCount != 1 || lease.IncomingAdmitted != 0 || lease.OutgoingAdmitted != 0 || lease.RootAdmitted != 0 || lease.IncomingReleased != 0 || lease.OutgoingReleased != 0 || lease.RootReleased != 0 || lease.IncomingLive != 0 || lease.OutgoingLive != 0 || lease.RootLive != 0 {
			t.Fatalf("ASSERT_PLUS_ONE_INCOMING_RELEASE_ONCE_ALL_LIVE_ZERO: %+v", lease)
		}
		incomingFailureLive, incomingFailureCumulative, incomingFailureAllocations, incomingFailureReleases, incomingFailureGeneral, incomingFailureOccupied := manager.DirectionalAccountingPrivate()
		if incomingFailureLive != 0 || incomingFailureCumulative != 0 || incomingFailureAllocations != 0 || incomingFailureReleases != 0 || incomingFailureGeneral != 0 || incomingFailureOccupied != false {
			t.Fatalf("ASSERT_PLUS_ONE_INCOMING_MANAGER_DIRECTIONAL_ACCOUNTING_ZERO: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t", incomingFailureLive, incomingFailureCumulative, incomingFailureAllocations, incomingFailureReleases, incomingFailureGeneral, incomingFailureOccupied)
		}
	})

	t.Run("outgoing-plus-one-prewire", func(t *testing.T) {
		child := newScriptedTransportPrivate("empty", "empty", false)
		manager, started, uri := readyManagerPrivate(t, child)
		result := executePrivate(t, context.Background(), manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 1, OutgoingBytes: exactDirectionBytesPrivate + 1}, false))
		if result.FailurePrivate() == nil || result.FailurePrivate().Code != "OUTGOING_DIRECTION_LIMIT_EXCEEDED" || result.CoordinationPrivate().SuppressionPrivate() == nil || result.CoordinationPrivate().SuppressionPrivate().Code != "DIRECTION_LIMIT_EXCEEDED" || result.CoordinationPrivate().SuppressionPrivate().Stage != "outgoing_preflight" || result.CoordinationPrivate().SuppressionPrivate().Direction != "outgoing" || result.CoordinationPrivate().SuppressionPrivate().Family != "GENERIC" {
			t.Fatalf("ASSERT_PLUS_ONE_OUTGOING_EXACT_TYPED_FAILURE: %+v", result)
		}
		if child.bytesFor("callHierarchy/incomingCalls", "request") != 0 || child.bytesFor("callHierarchy/incomingCalls", "response") != 0 || child.bytesFor("callHierarchy/outgoingCalls", "request") != 0 || child.bytesFor("callHierarchy/outgoingCalls", "response") != 0 {
			t.Fatalf("ASSERT_PLUS_ONE_OUTGOING_ZERO_DIRECTIONAL_WIRE_EFFECT: records=%+v", child.snapshot())
		}
		lease := result.CoordinationPrivate().LeaseStatePrivate()
		if !lease.Released || lease.RootReleaseCount != 1 || lease.IncomingAdmitted != 0 || lease.IncomingReleased != 0 || lease.OutgoingAdmitted != 0 || lease.OutgoingReleased != 0 || lease.RootAdmitted != 0 || lease.RootReleased != 0 || lease.IncomingLive != 0 || lease.OutgoingLive != 0 || lease.RootLive != 0 {
			t.Fatalf("ASSERT_PLUS_ONE_OUTGOING_RELEASE_ONCE_ZERO_EFFECT: %+v", lease)
		}
		outgoingFailureLive, outgoingFailureCumulative, outgoingFailureAllocations, outgoingFailureReleases, outgoingFailureGeneral, outgoingFailureOccupied := manager.DirectionalAccountingPrivate()
		if outgoingFailureLive != 0 || outgoingFailureCumulative != 0 || outgoingFailureAllocations != 0 || outgoingFailureReleases != 0 || outgoingFailureGeneral != 0 || outgoingFailureOccupied {
			t.Fatalf("ASSERT_PLUS_ONE_OUTGOING_MANAGER_DIRECTIONAL_ACCOUNTING_ZERO: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t", outgoingFailureLive, outgoingFailureCumulative, outgoingFailureAllocations, outgoingFailureReleases, outgoingFailureGeneral, outgoingFailureOccupied)
		}
	})

	t.Run("cancel-before-first", func(t *testing.T) {
		child := newScriptedTransportPrivate("nonempty", "nonempty", false)
		manager, started, uri := readyManagerPrivate(t, child)
		pin := retryAccountingPinPrivateFor(t, "cancel-before")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		failed := executePrivate(t, ctx, manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false))
		if failed.FailurePrivate() == nil || failed.FailurePrivate().Code != "CANCELLED_BEFORE_DIRECTION" || failed.CoordinationPrivate().SuppressionPrivate() == nil || failed.CoordinationPrivate().SuppressionPrivate().Code != "CANCELLED_BEFORE_DIRECTION" || failed.CoordinationPrivate().SuppressionPrivate().Stage != "incoming_preflight" || failed.CoordinationPrivate().SuppressionPrivate().Direction != "incoming" || failed.CoordinationPrivate().SuppressionPrivate().Family != "GENERIC" {
			t.Fatalf("ASSERT_CANCEL_BEFORE_EXACT_TYPED_CODE_STAGE_DIRECTION_FAMILY: %+v", failed)
		}
		if child.bytesFor("callHierarchy/incomingCalls", "request") != 0 || child.bytesFor("callHierarchy/incomingCalls", "response") != 0 || child.bytesFor("callHierarchy/outgoingCalls", "request") != 0 || child.bytesFor("callHierarchy/outgoingCalls", "response") != 0 {
			t.Fatalf("ASSERT_CANCEL_BEFORE_ZERO_DIRECTIONAL_WRITE_BYTES: %+v", child.snapshot())
		}
		lease := failed.CoordinationPrivate().LeaseStatePrivate()
		if !lease.Released || lease.RootReleaseCount != 1 || lease.IncomingAdmitted != pin.FailureIncoming || lease.OutgoingAdmitted != pin.FailureOutgoing || lease.RootAdmitted != pin.FailureRoot || lease.IncomingReleased != pin.FailureIncoming || lease.OutgoingReleased != pin.FailureOutgoing || lease.RootReleased != pin.FailureRoot || lease.IncomingLive != 0 || lease.OutgoingLive != 0 || lease.RootLive != 0 {
			t.Fatalf("ASSERT_CANCEL_BEFORE_RELEASE_ONCE_ALL_LIVE_ZERO: %+v pin=%+v", lease, pin)
		}
		cancelBeforeFailureLive, cancelBeforeFailureCumulative, cancelBeforeFailureAllocations, cancelBeforeFailureReleases, cancelBeforeFailureGeneral, cancelBeforeFailureOccupied := manager.DirectionalAccountingPrivate()
		if cancelBeforeFailureLive != 0 || cancelBeforeFailureCumulative != pin.FailureRoot || cancelBeforeFailureAllocations != 0 || cancelBeforeFailureReleases != 0 || cancelBeforeFailureGeneral != 0 || cancelBeforeFailureOccupied != false {
			t.Fatalf("ASSERT_CANCEL_BEFORE_FAILURE_MANAGER_DIRECTIONAL_ACCOUNTING: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t", cancelBeforeFailureLive, cancelBeforeFailureCumulative, cancelBeforeFailureAllocations, cancelBeforeFailureReleases, cancelBeforeFailureGeneral, cancelBeforeFailureOccupied)
		}
		retryStart := len(child.snapshot())
		succeeded := executePrivate(t, context.Background(), manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false))
		if succeeded.FailurePrivate() != nil || succeeded.CoordinationPrivate().CandidatePrivate() == nil || !succeeded.CoordinationPrivate().AcceptedPrivate() {
			t.Fatalf("ASSERT_SECOND_COMPLETE_REQUEST_SAME_SESSION_GENERATION_AFTER_CANCEL_BEFORE: %+v", succeeded)
		}
		successLease := succeeded.CoordinationPrivate().LeaseStatePrivate()
		if !successLease.Released || successLease.RootReleaseCount != 1 || successLease.IncomingAdmitted == 0 || successLease.OutgoingAdmitted == 0 || successLease.IncomingAdmitted != successLease.IncomingReleased || successLease.OutgoingAdmitted != successLease.OutgoingReleased || successLease.RootAdmitted != successLease.IncomingAdmitted+successLease.OutgoingAdmitted || successLease.RootReleased != successLease.RootAdmitted || successLease.IncomingLive != 0 || successLease.OutgoingLive != 0 || successLease.RootLive != 0 {
			t.Fatalf("ASSERT_CANCEL_BEFORE_SUCCESS_RELEASE_ONCE_ALL_LIVE_ZERO: %+v", successLease)
		}
		cancelBeforeRetryLive, cancelBeforeRetryCumulative, cancelBeforeRetryAllocations, cancelBeforeRetryReleases, cancelBeforeRetryGeneral, cancelBeforeRetryOccupied := manager.DirectionalAccountingPrivate()
		if cancelBeforeRetryLive != 0 || cancelBeforeRetryCumulative < successLease.RootAdmitted || cancelBeforeRetryAllocations != 2 || cancelBeforeRetryReleases != 1 || cancelBeforeRetryGeneral != 0 || cancelBeforeRetryOccupied {
			t.Fatalf("ASSERT_CANCEL_BEFORE_RETRY_MANAGER_DIRECTIONAL_ACCOUNTING_AGGREGATE_MANAGER_VS_DIRECTIONAL_LEASE: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t lease=%+v", cancelBeforeRetryLive, cancelBeforeRetryCumulative, cancelBeforeRetryAllocations, cancelBeforeRetryReleases, cancelBeforeRetryGeneral, cancelBeforeRetryOccupied, successLease)
		}
		retryObservations := manager.DirectionalObservationsPrivate()
		assertExactRetryWirePrivate(t, child, retryStart, directionalOnlyObservationsPrivate(retryObservations), "cancel-before", "ASSERT_CANCEL_BEFORE_EXACT_SAME_GENERATION_RETRY_WIRE_SEQUENCE")
		records := manager.Records()
		if len(records) != 1 || records[0].State != session.Ready || records[0].Generation != started.Generation {
			t.Fatalf("ASSERT_CANCEL_BEFORE_READY_SESSION_REUSED_EXACT_GENERATION: %+v", records)
		}
	})

	t.Run("cancel-between-directions", func(t *testing.T) {
		child := newScriptedTransportPrivate("nonempty", "nonempty", false)
		manager, started, uri := readyManagerPrivate(t, child)
		pin := retryAccountingPinPrivateFor(t, "cancel-between")
		ctx, cancel := context.WithCancel(context.Background())
		child.cancelAfterIncoming = cancel
		failed := executePrivate(t, ctx, manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false))
		if failed.FailurePrivate() == nil || failed.FailurePrivate().Code != "CANCELLED_BETWEEN_DIRECTIONS" || failed.CoordinationPrivate().SuppressionPrivate() == nil || failed.CoordinationPrivate().SuppressionPrivate().Code != "CANCELLED_BETWEEN_DIRECTIONS" || failed.CoordinationPrivate().SuppressionPrivate().Stage != "outgoing_preflight" || failed.CoordinationPrivate().SuppressionPrivate().Direction != "outgoing" || failed.CoordinationPrivate().SuppressionPrivate().Family != "GENERIC" {
			t.Fatalf("ASSERT_CANCEL_BETWEEN_EXACT_TYPED_CODE_STAGE_DIRECTION_FAMILY: failure=%+v suppression=%+v incoming_req=%d incoming_resp=%d outgoing_req=%d outgoing_resp=%d", failed.FailurePrivate(), failed.CoordinationPrivate().SuppressionPrivate(), child.bytesFor("callHierarchy/incomingCalls", "request"), child.bytesFor("callHierarchy/incomingCalls", "response"), child.bytesFor("callHierarchy/outgoingCalls", "request"), child.bytesFor("callHierarchy/outgoingCalls", "response"))
		}
		if child.bytesFor("callHierarchy/incomingCalls", "request") == 0 || child.bytesFor("callHierarchy/incomingCalls", "response") == 0 || child.bytesFor("callHierarchy/outgoingCalls", "request") != 0 || child.bytesFor("callHierarchy/outgoingCalls", "response") != 0 {
			t.Fatalf("ASSERT_CANCEL_BETWEEN_ZERO_OUTGOING_WRITE_BYTES: %+v", child.snapshot())
		}
		lease := failed.CoordinationPrivate().LeaseStatePrivate()
		if !lease.Released || lease.RootReleaseCount != 1 || lease.IncomingAdmitted != pin.FailureIncoming || lease.OutgoingAdmitted != pin.FailureOutgoing || lease.RootAdmitted != pin.FailureRoot || lease.IncomingReleased != pin.FailureIncoming || lease.OutgoingReleased != pin.FailureOutgoing || lease.RootReleased != pin.FailureRoot || lease.IncomingLive != 0 || lease.OutgoingLive != 0 || lease.RootLive != 0 {
			t.Fatalf("ASSERT_CANCEL_BETWEEN_RELEASE_ONCE_ALL_LIVE_ZERO: %+v pin=%+v", lease, pin)
		}
		cancelBetweenFailureLive, cancelBetweenFailureCumulative, cancelBetweenFailureAllocations, cancelBetweenFailureReleases, cancelBetweenFailureGeneral, cancelBetweenFailureOccupied := manager.DirectionalAccountingPrivate()
		if cancelBetweenFailureLive != 0 || cancelBetweenFailureCumulative != pin.FailureRoot || cancelBetweenFailureAllocations != 1 || cancelBetweenFailureReleases != 1 || cancelBetweenFailureGeneral != 0 || cancelBetweenFailureOccupied != false {
			t.Fatalf("ASSERT_CANCEL_BETWEEN_FAILURE_MANAGER_DIRECTIONAL_ACCOUNTING: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t", cancelBetweenFailureLive, cancelBetweenFailureCumulative, cancelBetweenFailureAllocations, cancelBetweenFailureReleases, cancelBetweenFailureGeneral, cancelBetweenFailureOccupied)
		}
		child.cancelAfterIncoming = nil
		retryStart := len(child.snapshot())
		succeeded := executePrivate(t, context.Background(), manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false))
		if succeeded.FailurePrivate() != nil || succeeded.CoordinationPrivate().CandidatePrivate() == nil || !succeeded.CoordinationPrivate().AcceptedPrivate() {
			t.Fatalf("ASSERT_SECOND_COMPLETE_REQUEST_SAME_SESSION_GENERATION_AFTER_CANCEL_BETWEEN: %+v", succeeded)
		}
		successLease := succeeded.CoordinationPrivate().LeaseStatePrivate()
		if !successLease.Released || successLease.RootReleaseCount != 1 || successLease.IncomingAdmitted == 0 || successLease.OutgoingAdmitted == 0 || successLease.IncomingAdmitted != successLease.IncomingReleased || successLease.OutgoingAdmitted != successLease.OutgoingReleased || successLease.RootAdmitted != successLease.IncomingAdmitted+successLease.OutgoingAdmitted || successLease.RootReleased != successLease.RootAdmitted || successLease.IncomingLive != 0 || successLease.OutgoingLive != 0 || successLease.RootLive != 0 {
			t.Fatalf("ASSERT_CANCEL_BETWEEN_SUCCESS_RELEASE_ONCE_ALL_LIVE_ZERO: %+v", successLease)
		}
		cancelBetweenRetryLive, cancelBetweenRetryCumulative, cancelBetweenRetryAllocations, cancelBetweenRetryReleases, cancelBetweenRetryGeneral, cancelBetweenRetryOccupied := manager.DirectionalAccountingPrivate()
		if cancelBetweenRetryLive != 0 || cancelBetweenRetryCumulative < successLease.RootAdmitted || cancelBetweenRetryAllocations != 3 || cancelBetweenRetryReleases != 2 || cancelBetweenRetryGeneral != 0 || cancelBetweenRetryOccupied {
			t.Fatalf("ASSERT_CANCEL_BETWEEN_RETRY_MANAGER_DIRECTIONAL_ACCOUNTING_AGGREGATE_MANAGER_VS_DIRECTIONAL_LEASE: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t lease=%+v", cancelBetweenRetryLive, cancelBetweenRetryCumulative, cancelBetweenRetryAllocations, cancelBetweenRetryReleases, cancelBetweenRetryGeneral, cancelBetweenRetryOccupied, successLease)
		}
		retryObservations := manager.DirectionalObservationsPrivate()
		assertExactRetryWirePrivate(t, child, retryStart, directionalOnlyObservationsPrivate(retryObservations), "cancel-between", "ASSERT_CANCEL_BETWEEN_EXACT_SAME_GENERATION_RETRY_WIRE_SEQUENCE")
		records := manager.Records()
		if len(records) != 1 || records[0].State != session.Ready || records[0].Generation != started.Generation {
			t.Fatalf("ASSERT_CANCEL_BETWEEN_READY_SESSION_REUSED_EXACT_GENERATION: %+v", records)
		}
	})

	t.Run("partial-outgoing-error-cleanup-and-ready-reuse", func(t *testing.T) {
		child := newScriptedTransportPrivate("nonempty", "error", false)
		manager, started, uri := readyManagerPrivate(t, child)
		pin := retryAccountingPinPrivateFor(t, "partial-outgoing-error")
		failed := executePrivate(t, context.Background(), manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, true))
		if failed.FailurePrivate() == nil || failed.FailurePrivate().Code != "OUTGOING_CALLS_FAILED" || failed.CoordinationPrivate().CandidatePrivate() != nil || failed.CoordinationPrivate().SuppressionPrivate() == nil || failed.CoordinationPrivate().SuppressionPrivate().Code != "OUTGOING_RUNTIME_FAILED" || failed.CoordinationPrivate().SuppressionPrivate().Stage != "outgoing_traversal" || failed.CoordinationPrivate().SuppressionPrivate().Direction != "outgoing" || failed.CoordinationPrivate().SuppressionPrivate().Family != "GENERIC" {
			t.Fatalf("ASSERT_PARTIAL_OUTGOING_EXACT_TYPED_FAILURE_AND_SUPPRESSION: %+v", failed)
		}
		if failed.CoordinationPrivate().CompositionPrivate().CompositionCandidate != nil {
			t.Fatalf("ASSERT_CANDIDATE_SUPPRESSION_TYPED_NOT_SUBSTRING: %+v", failed.CoordinationPrivate().CompositionPrivate().CompositionCandidate)
		}
		if child.countFor("callHierarchy/incomingCalls", "request") != 1 || child.countFor("callHierarchy/incomingCalls", "response") != 1 || child.countFor("callHierarchy/outgoingCalls", "request") != 1 || child.countFor("callHierarchy/outgoingCalls", "response") != 1 {
			t.Fatalf("ASSERT_PARTIAL_OUTGOING_EXACT_FIRST_ATTEMPT_WIRE_SEQUENCE: %+v", child.snapshot())
		}
		lease := failed.CoordinationPrivate().LeaseStatePrivate()
		if !lease.Released || lease.RootReleaseCount != 1 || lease.IncomingAdmitted != pin.FailureIncoming || lease.OutgoingAdmitted != pin.FailureOutgoing || lease.RootAdmitted != pin.FailureRoot || lease.IncomingReleased != pin.FailureIncoming || lease.OutgoingReleased != pin.FailureOutgoing || lease.RootReleased != pin.FailureRoot || lease.IncomingLive != 0 || lease.OutgoingLive != 0 || lease.RootLive != 0 {
			t.Fatalf("ASSERT_PARTIAL_AUTOMATIC_CLEANUP_BEFORE_MANUAL_RELEASE: %+v pin=%+v", lease, pin)
		}
		partialFailureLive, partialFailureCumulative, partialFailureAllocations, partialFailureReleases, partialFailureGeneral, partialFailureOccupied := manager.DirectionalAccountingPrivate()
		if partialFailureLive != 0 || partialFailureCumulative != pin.FailureRoot || partialFailureAllocations != 2 || partialFailureReleases != 1 || partialFailureGeneral != 0 || partialFailureOccupied != false {
			t.Fatalf("ASSERT_PARTIAL_FAILURE_MANAGER_DIRECTIONAL_ACCOUNTING: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t", partialFailureLive, partialFailureCumulative, partialFailureAllocations, partialFailureReleases, partialFailureGeneral, partialFailureOccupied)
		}
		child.outgoingVariant = "nonempty"
		child.cancelAfterIncoming = nil
		retryStart := len(child.snapshot())
		succeeded := executePrivate(t, context.Background(), manager, privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false))
		if succeeded.FailurePrivate() != nil || succeeded.CoordinationPrivate().CandidatePrivate() == nil || !succeeded.CoordinationPrivate().AcceptedPrivate() {
			t.Fatalf("ASSERT_SECOND_COMPLETE_REQUEST_SAME_READY_SESSION_AFTER_PARTIAL: %+v", succeeded)
		}
		successLease := succeeded.CoordinationPrivate().LeaseStatePrivate()
		if !successLease.Released || successLease.RootReleaseCount != 1 || successLease.IncomingAdmitted == 0 || successLease.OutgoingAdmitted == 0 || successLease.IncomingAdmitted != successLease.IncomingReleased || successLease.OutgoingAdmitted != successLease.OutgoingReleased || successLease.RootAdmitted != successLease.IncomingAdmitted+successLease.OutgoingAdmitted || successLease.RootReleased != successLease.RootAdmitted || successLease.IncomingLive != 0 || successLease.OutgoingLive != 0 || successLease.RootLive != 0 {
			t.Fatalf("ASSERT_PARTIAL_SUCCESS_RELEASE_ONCE_ALL_LIVE_ZERO: %+v", successLease)
		}
		partialRetryLive, partialRetryCumulative, partialRetryAllocations, partialRetryReleases, partialRetryGeneral, partialRetryOccupied := manager.DirectionalAccountingPrivate()
		if partialRetryLive != 0 || partialRetryCumulative < successLease.RootAdmitted || partialRetryAllocations != 4 || partialRetryReleases != 2 || partialRetryGeneral != 0 || partialRetryOccupied {
			t.Fatalf("ASSERT_PARTIAL_RETRY_MANAGER_DIRECTIONAL_ACCOUNTING_AGGREGATE_MANAGER_VS_DIRECTIONAL_LEASE: live=%d cumulative=%d allocations=%d releases=%d general=%d occupied=%t lease=%+v", partialRetryLive, partialRetryCumulative, partialRetryAllocations, partialRetryReleases, partialRetryGeneral, partialRetryOccupied, successLease)
		}
		records := manager.Records()
		if len(records) != 1 || records[0].State != session.Ready || records[0].Generation != started.Generation {
			t.Fatalf("ASSERT_READY_SESSION_REUSED_EXACT_GENERATION_AFTER_PARTIAL: %+v", records)
		}
		retryObservations := manager.DirectionalObservationsPrivate()
		assertExactRetryWirePrivate(t, child, retryStart, directionalOnlyObservationsPrivate(retryObservations), "partial-outgoing-error", "ASSERT_PARTIAL_EXACT_SAME_GENERATION_RETRY_WIRE_SEQUENCE")
	})
}
