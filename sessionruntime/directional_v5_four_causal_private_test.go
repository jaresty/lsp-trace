package sessionruntime_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"lsp-trace/incomingops"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const c4Budget = 32 << 20
const c4URI = "file:///workspace/main.go"

type c4Packet struct {
	kind string
	id   uint64
	raw  []byte
}

// c4WriteProbe observes Manager transport calls and offered raw bytes BEFORE
// the provider's frame decoder sees them, including incomplete prefix writes.
// It also counts bytes actually emitted by the underlying pipe writer.
// Only PLUS_ONE installs it; the other three fixture paths are unchanged.
type c4WriteProbe struct {
	target  io.WriteCloser
	mu      sync.Mutex
	calls   uint64
	raw     []byte
	written uint64
}

type c4WriteSnapshot struct {
	calls   uint64
	raw     []byte
	written uint64
}

func (p *c4WriteProbe) Write(raw []byte) (int, error) {
	p.mu.Lock()
	p.calls++
	p.raw = append(p.raw, raw...)
	p.mu.Unlock()
	n, err := p.target.Write(raw)
	p.mu.Lock()
	p.written += uint64(n)
	p.mu.Unlock()
	return n, err
}
func (p *c4WriteProbe) Close() error { return p.target.Close() }
func (p *c4WriteProbe) snapshot() c4WriteSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return c4WriteSnapshot{p.calls, append([]byte(nil), p.raw...), p.written}
}

type c4Child struct {
	probe   *c4WriteProbe
	in      *io.PipeReader
	stdin   *io.PipeWriter
	out     *io.PipeWriter
	stdout  *io.PipeReader
	mu      sync.Mutex
	mode    string
	source  []byte
	packets []c4Packet
}

func (c *c4Child) Stdin() io.WriteCloser {
	if c.probe != nil {
		return c.probe
	}
	return c.stdin
}
func (c *c4Child) Stdout() io.ReadCloser { return c.stdout }
func (c *c4Child) Close() managedprocess.ResourceObservation {
	c.stdin.Close()
	c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *c4Child) Teardown(context.Context) managedprocess.TeardownObservation {
	c.in.Close()
	c.out.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}

type c4Starter struct{ child *c4Child }

func (s c4Starter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}
func c4Item(detail string) []byte { // Deliberately preserve provider lexical bytes, not typed re-marshalling.
	return []byte(`{ "name":"source","kind":12,"uri":"` + c4URI + `","detail":"` + detail + `","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}} }`)
}
func c4Array(source []byte) []byte {
	decoy := bytes.Replace(c4Item(""), []byte(`"source"`), []byte(`"decoy"`), 1)
	return append(append(append([]byte("[\n"), decoy...), []byte(",\t")...), append(source, []byte("\n]")...)...)
}
func c4Frame(body []byte) []byte {
	return append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
}
func c4Reply(id uint64, result []byte) []byte {
	return c4Frame(append(append([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":`, id)), result...), '}'))
}
func c4Request(id uint64, method string, params []byte) []byte {
	return c4Frame(append(append([]byte(fmt.Sprintf(`{"id":%d,"jsonrpc":"2.0","method":%q,"params":`, id, method)), params...), '}'))
}
func c4OversizeItem() lsp.CallHierarchyItem {
	var item lsp.CallHierarchyItem
	json.Unmarshal(c4Item(""), &item)
	encode := func(i lsp.CallHierarchyItem) int {
		p, _ := json.Marshal(lsp.CallHierarchyIncomingCallsParams{Item: i})
		return len(c4Request(2, "callHierarchy/incomingCalls", p))
	}
	overhead := encode(item)
	n := (c4Budget + 1 - overhead - 64) / 6
	item.Detail = strings.Repeat("<", n)
	rest := c4Budget + 1 - encode(item)
	item.Detail += strings.Repeat("x", rest)
	if encode(item) != c4Budget+1 {
		panic("incorrect prospective-frame oracle")
	}
	return item
}
func (c *c4Child) serve() {
	reader := lspwire.NewReader(c.in, lspwire.Limits{MaxBodyBytes: 64 << 20, MaxHeaderBytes: 65536})
	for {
		msg, _, raw, _, err := reader.ReadWithFrameIfWithin(64 << 20)
		if err != nil {
			return
		}
		if len(msg.ID) == 0 {
			continue
		}
		id, _ := strconv.ParseUint(string(msg.ID), 10, 64)
		if msg.Method == "initialize" {
			reply := c4Reply(id, []byte(`{"capabilities":{"positionEncoding":"utf-16","callHierarchyProvider":true}}`))
			if _, err = c.out.Write(reply); err != nil {
				return
			}
			continue
		}
		c.mu.Lock()
		c.packets = append(c.packets, c4Packet{"request-frame", id, raw})
		c.mu.Unlock()
		var frames [][]byte
		if msg.Method == "textDocument/prepareCallHierarchy" {
			source := c4Item("")
			if c.mode == "exact" {
				goal := c4Budget - len(raw)
				empty := c4Reply(id, c4Array(source))
				padding := goal - len(empty)
				source = c4Item(strings.Repeat("x", padding))
				reply := c4Reply(id, c4Array(source))
				source = c4Item(strings.Repeat("x", padding+goal-len(reply)))
			}
			if c.mode == "over" {
				source = c4Item(c4OversizeItem().Detail)
			}
			c.mu.Lock()
			c.source = source
			c.mu.Unlock()
			frames = [][]byte{c4Reply(id, c4Array(source))}
		} else if c.mode == "partial" {
			notification := c4Frame([]byte(`{"jsonrpc":"2.0","method":"window/logMessage","params":{"message":"external"}}`))
			unmatched := c4Reply(999, []byte(`null`))
			full := c4Reply(id, []byte(`[{"from":`+string(c4Item(""))+`,"fromRanges":[]}]`))
			frames = [][]byte{notification, unmatched, full[:len(full)-9]}
		} else {
			frames = [][]byte{c4Reply(id, []byte(`null`))}
		}
		c.mu.Lock()
		for _, frame := range frames {
			c.packets = append(c.packets, c4Packet{"response-frame", id, frame})
		}
		c.mu.Unlock()
		for _, frame := range frames {
			if _, err = c.out.Write(frame); err != nil {
				return
			}
		}
		if c.mode == "partial" && msg.Method == "callHierarchy/incomingCalls" {
			c.out.Close()
			return
		}
	}
}
func c4Setup(t *testing.T, mode string) (*sessionruntime.Manager, sessionruntime.StartResult, *c4Child, *incomingops.SessionClient) {
	t.Helper()
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	c := &c4Child{in: in, stdin: stdin, out: out, stdout: stdout, mode: mode}
	if mode == "over" {
		c.probe = &c4WriteProbe{target: stdin}
	}
	go c.serve()
	m, err := sessionruntime.New(sessionruntime.Config{Starter: c4Starter{c}, Wire: lspwire.Limits{MaxBodyBytes: 64 << 20, MaxHeaderBytes: 65536}, Limits: sessionruntime.Limits{MaxSessions: 2, MaxRequests: 2, MaxChildren: 4, MaxCancels: 4, MaxTombstones: 8, MaxObservations: 64, MaxOperations: 4}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/workspace", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selector)})
	if s.Failure != "" {
		t.Fatal(s.Failure)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	op := m.BeginReadiness(ctx, s.SessionID, s.Generation, time.Now().Add(4*time.Second))
	ready, ok := m.WaitReadiness(ctx, op.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("ASSERT_C4_REAL_MANAGER: %+v", ready)
	}
	client := incomingops.NewSessionClientWithWireLimits(m, s.SessionID, s.Generation, 10*time.Second, incomingops.WireLimits{MaxBytes: 64 << 20, MaxMessages: 8})
	return m, s, c, client
}
func c4Prepare(t *testing.T, m *sessionruntime.Manager, c *c4Child, client *incomingops.SessionClient, r directionalraw.Reservation) (lsp.CallHierarchyItem, directionalraw.View) {
	t.Helper()
	ctx := r.Context(context.Background())
	items, err := client.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: c4URI}})
	if err != nil || len(items) != 2 {
		t.Fatalf("ASSERT_C4_PRODUCTION_ENTRY: %v count=%d", err, len(items))
	}
	t.Log("ASSERT_C4_PRODUCTION_ENTRY PASS")
	selection, ok := directionalraw.Select(ctx, directionalraw.Request{Method: "textDocument/prepareCallHierarchy", Ordinal: 1})
	c.mu.Lock()
	source := c.source
	c.mu.Unlock()
	view := selection.View()
	if !ok || !view.SharesBackingPrivate() || !view.Equal(source) || selection.Observation().SHA256 != sha256.Sum256(source) || selection.Observation().Length != uint64(len(source)) {
		t.Fatal("ASSERT_C4_SOURCE_ORDINAL_SPAN")
	}
	return items[1], view
}
func c4Observe(t *testing.T, m *sessionruntime.Manager, c *c4Child) []directionalraw.Observation {
	t.Helper()
	obs := m.DirectionalObservationsPrivate()
	if len(obs) == 0 {
		t.Fatal("ASSERT_C4_MANAGER_OBSERVATION")
	}
	c.mu.Lock()
	packets := append([]c4Packet(nil), c.packets...)
	source := c.source
	c.mu.Unlock()
	used := map[int]bool{}
	for _, o := range obs {
		if o.Component == "element" {
			if o.Ordinal != 1 || o.SHA256 != sha256.Sum256(source) || o.Length != uint64(len(source)) {
				t.Fatal("ASSERT_C4_SOURCE_ORDINAL_SPAN")
			}
			continue
		}
		if o.Component == "request-frame-reservation-refusal" {
			continue
		}
		found := false
		for i, p := range packets {
			if !used[i] && p.id == o.RequestID && p.kind == o.Component {
				used[i] = true
				found = true
				if o.Length != uint64(len(p.raw)) || o.SHA256 != sha256.Sum256(p.raw) {
					t.Fatalf("ASSERT_C4_EXACT_ACQUIRED_DIGEST: %+v", o)
				}
				break
			}
		}
		if !found {
			t.Fatal("ASSERT_C4_EXACT_PENDING_OBSERVATION")
		}
	}
	// Equality is bidirectional: a missing request observation must not pass
	// merely because every remaining observation matches some provider packet.
	if c.mode == "exact" {
		for i := range packets {
			if !used[i] {
				t.Fatalf("ASSERT_C4_EXACT32_COMPLETE_CORRESPONDENCE: missing packet %d kind=%s", i, packets[i].kind)
			}
		}
		t.Log("ASSERT_C4_EXACT32_COMPLETE_CORRESPONDENCE PASS")
	}
	return obs
}
func c4Release(t *testing.T, m *sessionruntime.Manager, r directionalraw.Reservation, view directionalraw.View) map[string]any {
	t.Helper()
	before, _, allocs, _, generalBefore, _ := m.DirectionalAccountingPrivate()
	events := m.DirectionalC15EventsPrivate()
	created := map[string]int{}
	for _, e := range events {
		if e.Kind == "allocation" {
			created[e.ObjectID]++
		}
	}
	if len(created) != int(allocs)+1 {
		t.Fatal("ASSERT_C4_RESERVATION_OBJECT_RECORD")
	}
	if !r.Release() || r.Release() {
		t.Fatal("ASSERT_C4_TERMINAL_EXACT_ONCE")
	}
	active, closed, started, released := m.DirectionalGenericTerminalPrivate()
	live, cumulative, _, releases, general, occupied := m.DirectionalAccountingPrivate()
	if active != 0 || !closed || !started || !released || !r.Closed() || live != 0 || general != 0 || occupied || view.Len() != 0 {
		t.Fatal("ASSERT_C4_GENERIC_ACCOUNT_RELEASE")
	}
	terminal := map[string]int{}
	for _, e := range m.DirectionalC15EventsPrivate() {
		if e.Kind == "release" && e.Terminal {
			terminal[e.ObjectID]++
		}
	}
	for id, count := range created {
		if count != 1 || terminal[id] != 1 {
			t.Fatal("ASSERT_C4_EXACT_OBJECT_RELEASE")
		}
	}
	return map[string]any{"live_before": before, "live_after": live, "general_table_live_before": generalBefore, "cumulative": cumulative, "retained_allocations": allocs, "partition_releases": releases, "terminal_wins": 1, "generic_released": released, "object_creations": created, "object_terminal_releases": terminal}
}
func c4Receipt(t *testing.T, name string, obs []directionalraw.Observation, release map[string]any, extra map[string]any) {
	t.Helper()
	rows := []map[string]any{}
	for _, o := range obs {
		var ordinal any
		if o.Selected {
			ordinal = o.Ordinal
		}
		rows = append(rows, map[string]any{"component": o.Component, "request_id": o.RequestID, "generation": o.Generation, "source_ordinal": ordinal, "selected": o.Selected, "length": o.Length, "sha256": fmt.Sprintf("%x", o.SHA256), "failure": o.Failure})
	}
	extra["source_ordinal"] = 1
	extra["measurement_scope"] = "directional raw-frame custody; general bookkeeping is separately reported"
	receipt := map[string]any{"witness": name, "pid": os.Getpid(), "entry": "incomingops.SessionClient.call -> private Runtime -> actual Manager -> shared transport", "production_go": false, "observations": rows, "release": release, "facts": extra}
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("ADR0011_V5_CAUSAL_RECEIPT"); path != "" {
		if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("ASSERT_C4_%s PASS", name)
}
func TestV5CausalExact32MiB(t *testing.T) {
	m, _, c, client := c4Setup(t, "exact")
	r := directionalraw.NewReservation("c4-exact")
	_, view := c4Prepare(t, m, c, client, r)
	live, cumulative, allocations, _, general, occupied := m.DirectionalAccountingPrivate()
	if live != c4Budget || cumulative != c4Budget || allocations != 2 || general != 24600 || !occupied {
		t.Fatalf("ASSERT_C4_EXACT32_RESERVATION: live=%d cumulative=%d allocations=%d", live, cumulative, allocations)
	}
	obs := c4Observe(t, m, c)
	boundaries := m.DirectionalAllocationBoundariesPrivate()
	c.mu.Lock()
	packets := append([]c4Packet(nil), c.packets...)
	c.mu.Unlock()
	if len(boundaries) != len(packets) || len(boundaries) != 2 {
		t.Fatal("ASSERT_C4_ALLOCATION_BOUNDARY")
	}
	var total uint64
	frames := []map[string]any{}
	for i, p := range packets {
		total += uint64(len(p.raw))
		b := boundaries[i]
		assertion := "ASSERT_C4_REQUEST_ALLOCATION_BOUNDARY"
		if p.kind == "response-frame" {
			assertion = "ASSERT_C4_RESPONSE_ALLOCATION_BOUNDARY"
		}
		if b.Component != p.kind || b.Length != len(p.raw) || b.Live != total || b.Cumulative != total || b.Allocations != uint64(i+1) || b.OwnershipObjects != i+1 || !b.Occupied {
			t.Fatalf("%s: before make snapshot=%+v expected_reserved=%d expected_objects=%d", assertion, b, total, i+1)
		}
		t.Logf("%s PASS", assertion)
		frames = append(frames, map[string]any{"component": p.kind, "request_id": p.id, "length": len(p.raw), "sha256": fmt.Sprintf("%x", sha256.Sum256(p.raw))})
	}
	release := c4Release(t, m, r, view)
	c4Receipt(t, "EXACT32", obs, release, map[string]any{"accepted": true, "limit": c4Budget, "allocation_boundaries": boundaries, "actual_frames": frames})
}
func TestV5CausalOver32MiBPreEffect(t *testing.T) {
	m, s, c, client := c4Setup(t, "over")
	sourceR := directionalraw.NewReservation("c4-over-source")
	item, view := c4Prepare(t, m, c, client, sourceR)
	sourceObs := c4Observe(t, m, c)
	sourceRelease := c4Release(t, m, sourceR, view)
	r := directionalraw.NewReservation("c4-over-request")
	wireBefore := c.probe.snapshot()
	c.mu.Lock()
	before := len(c.packets)
	c.mu.Unlock()
	_, _, err := client.IncomingCalls(r.Context(context.Background()), item)
	if err == nil || !strings.Contains(err.Error(), string(session.ResourceExhausted)) {
		t.Fatalf("ASSERT_C4_PLUS_ONE_PRE_EFFECT: %v", err)
	}
	live, cumulative, allocations, _, _, occupied := m.DirectionalAccountingPrivate()
	c.mu.Lock()
	after := len(c.packets)
	c.mu.Unlock()
	wireAfter := c.probe.snapshot()
	writeCalls := wireAfter.calls - wireBefore.calls
	raw := wireAfter.raw[len(wireBefore.raw):]
	writtenBytes := wireAfter.written - wireBefore.written
	if writeCalls != 0 || len(raw) != 0 || writtenBytes != 0 {
		t.Fatalf("ASSERT_C4_PLUS_ONE_ZERO_WIRE: calls=%d raw_bytes=%d written_bytes=%d raw_sha256=%x", writeCalls, len(raw), writtenBytes, sha256.Sum256(raw))
	}
	_, failure := m.Metadata(s.SessionID, s.Generation)
	if live != 0 || cumulative != 0 || allocations != 0 || occupied || before != after || failure != "" {
		t.Fatal("ASSERT_C4_PLUS_ONE_ZERO_EFFECT")
	}
	params, _ := json.Marshal(lsp.CallHierarchyIncomingCallsParams{Item: item})
	proposed := c4Request(2, "callHierarchy/incomingCalls", params)
	obs := m.DirectionalObservationsPrivate()
	if len(proposed) != c4Budget+1 || len(obs) != 1 || obs[0].Component != "request-frame-reservation-refusal" || obs[0].Length != uint64(len(proposed)) || obs[0].SHA256 != sha256.Sum256(proposed) || obs[0].Failure != string(session.ResourceExhausted) {
		t.Fatal("ASSERT_C4_PROPOSED_COMMITMENT")
	}
	release := c4Release(t, m, r, directionalraw.View{})

	// Use the SAME client, actual Manager, session ID and generation after
	// refusal/release, not a replacement session or a metadata-only assertion.
	var reuseItem lsp.CallHierarchyItem
	if err := json.Unmarshal(c4Item(""), &reuseItem); err != nil {
		t.Fatal(err)
	}
	calls, null, reuseErr := client.IncomingCalls(context.Background(), reuseItem)
	_, reuseFailure := m.Metadata(s.SessionID, s.Generation)
	wireReused := c.probe.snapshot()
	reuseRaw := wireReused.raw[len(wireAfter.raw):]
	c.mu.Lock()
	reusePackets := append([]c4Packet(nil), c.packets[after:]...)
	c.mu.Unlock()
	if reuseErr != nil || !null || len(calls) != 0 || reuseFailure != "" || wireReused.calls <= wireAfter.calls || len(reusePackets) != 2 || reusePackets[0].kind != "request-frame" || reusePackets[1].kind != "response-frame" || reusePackets[0].id <= obs[0].RequestID || reusePackets[0].id != reusePackets[1].id || !bytes.Equal(reuseRaw, reusePackets[0].raw) || wireReused.written-wireAfter.written != uint64(len(reuseRaw)) {
		t.Fatalf("ASSERT_C4_PLUS_ONE_SAME_READY_REUSE: err=%v ready_failure=%s null=%v calls=%d packets=%d", reuseErr, reuseFailure, null, wireReused.calls-wireAfter.calls, len(reusePackets))
	}
	c4Receipt(t, "PLUS_ONE", append(sourceObs, obs...), release, map[string]any{
		"proposed_not_acquired": true, "attempted_bytes": len(proposed), "wire_effect_delta": after - before, "source_release": sourceRelease,
		"reservation": map[string]any{"live": live, "cumulative": cumulative, "retained_allocations": allocations, "occupied": occupied},
		"refusal":     map[string]any{"failure": string(session.ResourceExhausted), "proposed_length": len(proposed), "proposed_sha256": fmt.Sprintf("%x", sha256.Sum256(proposed))},
		"zero_wire":   map[string]any{"measurement": "Manager stdin Write calls and raw bytes before provider frame decoding", "write_calls": writeCalls, "raw_bytes": len(raw), "written_bytes": writtenBytes, "raw_sha256": fmt.Sprintf("%x", sha256.Sum256(raw)), "decoded_packet_delta": after - before},
		"reuse":       map[string]any{"same_client": true, "same_manager": true, "session_id_before": s.SessionID, "session_id_after": s.SessionID, "generation_before": s.Generation, "generation_after": s.Generation, "ready_before": failure == "", "ready_after": reuseFailure == "", "success": reuseErr == nil, "null": null, "request_id": reusePackets[0].id, "write_calls": wireReused.calls - wireAfter.calls, "raw_bytes": len(reuseRaw), "written_bytes": wireReused.written - wireAfter.written, "raw_sha256": fmt.Sprintf("%x", sha256.Sum256(reuseRaw)), "decoded_request_sha256": fmt.Sprintf("%x", sha256.Sum256(reusePackets[0].raw))},
	})
}
func TestV5CausalPartialExternalFailure(t *testing.T) {
	m, _, c, client := c4Setup(t, "partial")
	r := directionalraw.NewReservation("c4-partial")
	item, view := c4Prepare(t, m, c, client, r)
	_, _, err := client.IncomingCalls(r.Context(context.Background()), item)
	failure := m.DirectionalFailurePrivate()
	if err == nil || err.Error() != string(session.SessionPoisoned) || failure != session.SessionPoisoned || failure == session.ResourceExhausted {
		t.Fatalf("ASSERT_C4_PARTIAL_EXTERNAL_TYPED_FAILURE: client=%v typed=%q expected=%q (not %q)", err, failure, session.SessionPoisoned, session.ResourceExhausted)
	}
	obs := c4Observe(t, m, c)
	responseFrames := 0
	partial := false
	for _, o := range obs {
		if o.Method == "callHierarchy/incomingCalls" && o.Component == "response-frame" {
			responseFrames++
			partial = partial || o.Failure != ""
		}
	}
	if responseFrames != 3 || !partial {
		t.Fatal("ASSERT_C4_PARTIAL_EXTERNAL_SPANS")
	}
	for _, o := range obs {
		if o.Method == "callHierarchy/incomingCalls" && o.Failure != string(session.SessionPoisoned) {
			t.Fatalf("ASSERT_C4_PARTIAL_EXTERNAL_TYPED_FAILURE: component=%s label=%q expected=%q", o.Component, o.Failure, session.SessionPoisoned)
		}
	}
	t.Log("ASSERT_C4_PARTIAL_EXTERNAL_TYPED_FAILURE PASS")
	release := c4Release(t, m, r, view)
	c4Receipt(t, "PARTIAL_EXTERNAL", obs, release, map[string]any{"failed": true, "external_plus_partial_frames": responseFrames, "failure": failure, "refusal_failure": session.ResourceExhausted})
}
func TestV5CausalEarlyTerminalOnce(t *testing.T) {
	m, _, c, client := c4Setup(t, "early")
	r := directionalraw.NewReservation("c4-early")
	item, view := c4Prepare(t, m, c, client, r)
	calls, null, err := client.IncomingCalls(r.Context(context.Background()), item)
	if err != nil || !null || len(calls) != 0 {
		t.Fatal("ASSERT_C4_EARLY_RETURN")
	}
	obs := c4Observe(t, m, c)
	release := c4Release(t, m, r, view)
	c4Receipt(t, "EARLY_TERMINAL", obs, release, map[string]any{"null_early_return": null})
}
