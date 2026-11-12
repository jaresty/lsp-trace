//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	tp "lsp-trace/internal/targetpacket"
)

func privateRDDriverDTO() (privateRDConfig, sc.PrivateSelectedEndpointV2, tp.PrivateSourceUnit) {
	c := privateRDConfig{URI: privateRDURI, Workspace: privateRDWorkspace, Revision: "immutable-revision", Privacy: "private"}
	c.SourceLimits.MaxDocumentBytes = 1024
	var e sc.PrivateSelectedEndpointV2
	e.Slot.Mode = "RD"
	e.Intent.Endpoint.Kind = "RESOLVES_TO_DEFINITION"
	e.Intent.Endpoint.Role = "DEFINITION_TARGET"
	u := tp.PrivateSourceUnit{URI: privateRDURI, SourceDigest: privateRDDigest, Revision: c.Revision, Privacy: c.Privacy, Workspace: c.Workspace, PositionEncoding: "utf-16", Body: "synthetic only"}
	u.SelectionRange.Start.Line = 21
	u.SelectionRange.Start.Character = 5
	u.SelectionRange.End.Line = 21
	u.SelectionRange.End.Character = 9
	u.DisplayRange.Start.Line = 20
	u.DisplayRange.End.Line = 22
	return c, e, u
}

// DTO acceptance proves only fixed metadata comparisons, NEVER native provenance.
func TestPrivateRDDriverTargetURI(t *testing.T) {
	c, e, u := privateRDDriverDTO()
	var b privateRDBudget
	if !privateRDTargetData(&b, c, e, u) {
		t.Fatal("ASSERT_RD_PREBOUND_URI: valid DTO refused")
	}
	t.Log("ASSERT_RD_PREBOUND_URI: PASS (metadata DTO only)")
}
func TestPrivateRDDriverTargetRejections(t *testing.T) {
	c, e, u := privateRDDriverDTO()
	cases := []struct {
		name   string
		change func(*sc.PrivateSelectedEndpointV2, *tp.PrivateSourceUnit)
	}{
		{"wrongURI", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.URI = "file:///wrong" }},
		{"wrongDigest", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) {
			u.SourceDigest = strings.TrimPrefix(privateRDDigest, "sha256:")
		}},
		{"revision", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.Revision = "wrong" }},
		{"privacy", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.Privacy = "wrong" }},
		{"workspace", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.Workspace = "wrong" }},
		{"encoding", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.PositionEncoding = "utf-8" }},
		{"self", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) {
			u.SelectionRange.Start.Line = 33
			u.SelectionRange.Start.Character = 1
			u.SelectionRange.End.Line = 33
			u.SelectionRange.End.Character = 2
		}},
		{"selection", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.SelectionRange.End.Character = 10 }},
		{"emptyRange", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) {
			u.DisplayRange.End = u.DisplayRange.Start
		}},
		{"notContaining", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.DisplayRange.Start.Line = 22 }},
		{"emptyBody", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.Body = "" }},
		{"bodyLimit", func(_ *sc.PrivateSelectedEndpointV2, u *tp.PrivateSourceUnit) { u.Body = strings.Repeat("x", 1025) }},
		{"kind", func(e *sc.PrivateSelectedEndpointV2, _ *tp.PrivateSourceUnit) {
			e.Intent.Endpoint.Kind = "REFERENCES_SYMBOL"
		}},
		{"role", func(e *sc.PrivateSelectedEndpointV2, _ *tp.PrivateSourceUnit) { e.Intent.Endpoint.Role = "QUERY" }},
		{"missing", func(e *sc.PrivateSelectedEndpointV2, _ *tp.PrivateSourceUnit) { e.RowPosition = -1 }},
		{"mode", func(e *sc.PrivateSelectedEndpointV2, _ *tp.PrivateSourceUnit) { e.Slot.Mode = "D" }},
	}
	for _, v := range cases {
		t.Run(v.name, func(t *testing.T) {
			ee, uu := e, u
			v.change(&ee, &uu)
			var b privateRDBudget
			if privateRDTargetData(&b, c, ee, uu) {
				t.Fatal("wrong metadata qualified")
			}
		})
	}
	var b privateRDBudget
	if privateRDSelected(&b, c, sc.PrivateSelectedResultV2{}) {
		t.Fatal("zero opaque result accepted")
	}
	t.Log("ASSERT_RD_REJECTIONS: PASS")
}
func TestPrivateRDDriverBodyLengthOnly(t *testing.T) {
	c, e, u := privateRDDriverDTO()
	var a, b privateRDBudget
	u.Body = "aa"
	if !privateRDTargetData(&a, c, e, u) {
		t.Fatal("DTO a")
	}
	u.Body = "\x00\xff"
	if !privateRDTargetData(&b, c, e, u) || a != b {
		t.Fatal("body content affected metadata guard")
	}
	t.Log("ASSERT_RD_BODY_LENGTH_ONLY: PASS")
}
func TestPrivateRDDriverBudget(t *testing.T) {
	var b privateRDBudget
	if !b.equal(strings.Repeat("a", 2048), strings.Repeat("a", 2048)) || b.bytes != 4096 {
		t.Fatal("exact comparison bound")
	}
	if b.equal("x", "x") || !b.failed || b.bytes != 4096 {
		t.Fatal("over comparison bound")
	}
	for _, p := range []struct{ n, cap uint64 }{{4096, 4096}, {256, 256}, {32, 32}} {
		var n uint64
		if !privateRDCharge(&n, p.n, p.cap) || privateRDCharge(&n, 1, p.cap) {
			t.Fatal("exact/over bound")
		}
	}
	n := uint64(math.MaxUint64)
	if privateRDCharge(&n, 1, math.MaxUint64) || n != math.MaxUint64 {
		t.Fatal("overflow")
	}
	b = privateRDBudget{steps: 256}
	if b.step() || !b.failed {
		t.Fatal("steps")
	}
	b = privateRDBudget{nodes: 32}
	if b.node() || !b.failed {
		t.Fatal("nodes")
	}
	b = privateRDBudget{}
	if b.equal(strings.Repeat("x", 4097), "x") || b.bytes != 0 {
		t.Fatal("length before equality")
	}
	t.Log("ASSERT_RD_BUDGET: PASS")
}

type privateRDDriverWriter struct {
	calls, n int
	err      error
	seen     []byte
}

func (w *privateRDDriverWriter) Write(p []byte) (int, error) {
	w.calls++
	w.seen = append(w.seen, p...)
	if w.n >= 0 {
		return w.n, w.err
	}
	return len(p), w.err
}
func TestPrivateRDDriverReport(t *testing.T) {
	for _, v := range []struct {
		n int
		e error
	}{{-1, nil}, {0, nil}, {1, io.ErrClosedPipe}} {
		w := &privateRDDriverWriter{n: v.n, err: v.e}
		var b privateRDBudget
		out := privateRDOutcome{native: privateRDNativeAvailable, qualified: true, cleanup: true, live: true}
		code := privateRDReport(w, &b, out, privateRDTraceComplete)
		if w.calls != 1 || b.writes != 1 || b.scalars != 11 || b.reportBytes > 4096 {
			t.Fatal("report limits")
		}
		if (v.n < 0 && v.e == nil) != (code == 0) {
			t.Fatal("short/error report status")
		}
		if privateRDReport(w, &b, out, privateRDTraceComplete) != 1 || w.calls != 1 {
			t.Fatal("second report write")
		}
		if bytes.Contains(w.seen, []byte(privateRDURI)) || bytes.Contains(w.seen, []byte(privateRDDigest)) {
			t.Fatal("payload in report")
		}
	}
	var dst bytes.Buffer
	var b privateRDBudget
	if privateRDReport(&dst, &b, privateRDOutcome{native: privateRDNativeAvailable, qualified: true, cleanup: true}, privateRDTraceInvalid) == 0 {
		t.Fatal("incomplete trace qualified")
	}
	var fields map[string]any
	if json.Unmarshal(bytes.TrimSpace(dst.Bytes()), &fields) != nil || fields["Native"] != "AVAILABLE" || fields["Trace"] != "INVALID_INCOMPLETE" || len(fields) != 11 {
		t.Fatal("trace rewrote native outcome")
	}
	t.Log("ASSERT_RD_REPORT: PASS")
}
func TestPrivateRDDriverTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := privateRDOutcome{native: privateRDNativeAvailable, qualified: true}
	if privateRDFinalState(ctx, time.Now().Add(time.Second), out, true).qualified {
		t.Fatal("canceled")
	}
	if privateRDFinalState(context.Background(), time.Now().Add(-time.Second), out, true).qualified {
		t.Fatal("expired")
	}
	if privateRDFinalState(context.Background(), time.Now().Add(time.Second), out, false).qualified {
		t.Fatal("cleanup")
	}
	if privateRDFinalState(ctx, time.Now().Add(time.Second), out, false).native != privateRDNativeAvailable {
		t.Fatal("native outcome rewritten")
	}
	t.Log("ASSERT_RD_TERMINAL: PASS")
}
func TestPrivateRDDriverConfigRoundTrip(t *testing.T) {
	c, _, _ := privateRDDriverDTO()
	c.Schema = "private-source-prepare-rd.v1"
	c.AccountingRoot = "/synthetic-accounting"
	c.CanonicalJob = []byte("synthetic metadata, not an admitted job")
	c.ProcessPath = "/synthetic-provider"
	c.Profile.Workspace = c.Workspace
	c.Deadline = time.Unix(1000, 0)
	c.RetainUntil = c.Deadline.Add(time.Minute)
	c.Attempts = [3]string{strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)}
	pin := "sha256:" + strings.Repeat("0", 64)
	c.ContractInput = cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, CaptureLimits: cc.ProductionCaptureLimits(), PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	if _, err := cc.NewContinuationContract(c.ContractInput); err != nil {
		t.Fatal("public metadata constructor", err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	decoded, ok := decodePrivateRDConfig(raw)
	if !ok || decoded.URI != c.URI || decoded.Profile.Options != nil || decoded.ProcessArgs != nil {
		t.Fatal("canonical data config round trip/nil preservation")
	}
	if _, ok := decodePrivateRDConfig(append(raw, ' ')); ok {
		t.Fatal("noncanonical trailing space accepted")
	}
	c.Attempts[2] = c.Attempts[1]
	raw, _ = json.Marshal(c)
	if _, ok := decodePrivateRDConfig(raw); ok {
		t.Fatal("duplicate attempt accepted")
	}
	t.Log("ASSERT_RD_CONFIG_DATA: PASS (not job/native admission)")
}

func TestPrivateRDDriverConfigClosed(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`{"Body":"forbidden"}`), []byte(`{"Selector":{}}`), []byte(`{"Contract":{}}`), []byte(`{"Timing":{}}`), bytes.Repeat([]byte("x"), 65537)} {
		if _, ok := decodePrivateRDConfig(raw); ok {
			t.Fatal("unclosed config accepted")
		}
	}
	// These tests never invoke the CLI/native entry, even on invalid arguments.
	t.Log("ASSERT_RD_CONFIG_CLOSED: PASS")
}
