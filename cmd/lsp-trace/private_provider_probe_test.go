package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func probePath(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(d, 0700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(d, "probe.json")
}

func TestPrivateProviderProbeFlagIsHiddenDuplicateRejectingAndFreshMachineOnly(t *testing.T) {
	base := []string{"census", "--workspace", ".", "--server", "gopls", "--publication-root", probePath(t), "--machine"}
	path := probePath(t)
	for name, args := range map[string][]string{
		"duplicate":  append(append([]string{}, base...), "--private-provider-probe-path", path, "--private-provider-probe-path", path),
		"nonmachine": append(append([]string{}, base[:len(base)-1]...), "--private-provider-probe-path", path),
		"catalog":    append(append([]string{}, base...), "--private-provider-probe-path", path, "--catalog"),
		"resume":     {"census", "--machine", "--resume", "sha256:" + strings.Repeat("a", 64), "--catalog-config", "x", "--publication-root", path, "--private-provider-probe-path", path},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseCensusCLIOptions(args); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	got, err := parseCensusCLIOptions(append(base, "--private-provider-probe-path", path))
	if err != nil || got.PrivateProviderProbePath != path {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if strings.Contains(censusUsage, "private-provider-probe") {
		t.Fatal("private flag disclosed")
	}
}

func TestPrivateProviderProbeMappingsPrecedenceAndRPCBoundaries(t *testing.T) {
	for _, f := range []session.Failure{session.SessionNotFound, session.StaleGeneration, session.LifecycleConflict, session.SessionStopping, session.SessionPoisoned, session.ResourceExhausted, session.SessionCrashed, session.RequestCancelled, session.RequestTimeout} {
		c, v := privateSessionFailureCause(f)
		if c != "TYPED_SESSION_RUNTIME_FAILURE" || v != string(f) {
			t.Fatalf("%s %s", c, v)
		}
	}
	if _, v := privateSessionFailureCause(session.Failure("secret")); v != "OTHER_TYPED_FAILURE" {
		t.Fatal(v)
	}
	for _, f := range []session.Failure{sessionruntime.DocumentSupplyUnavailable, sessionruntime.DocumentURIUnavailable, sessionruntime.DocumentOutsideWorkspace, sessionruntime.DocumentSourceUnavailable, sessionruntime.LanguageIDUnavailable} {
		c, v := privateDocumentFailureCause(f)
		if c != "DOCUMENT_FAILURE" || v != string(f) {
			t.Fatal(f)
		}
	}
	if _, v := privateDocumentFailureCause(session.Failure("secret")); v != "OTHER_TYPED_FAILURE" {
		t.Fatal(v)
	}
	cases := map[int]string{-32700: "PARSE_ERROR", -32600: "INVALID_REQUEST", -32601: "METHOD_NOT_FOUND", -32602: "INVALID_PARAMS", -32603: "INTERNAL_ERROR", -32099: "SERVER_ERROR", -32000: "SERVER_ERROR", -32100: "OTHER", -31999: "OTHER"}
	for code, want := range cases {
		c, v := privateRPCErrorCause(&lspwire.RPCError{Code: code, Message: "forbidden"})
		if c != "JSON_RPC_CODE_BUCKET" || v != want {
			t.Fatalf("%d %s", code, v)
		}
	}
}

func TestPrivateProviderProbeChronologyCountsBoundsCanonicalAndPrivacy(t *testing.T) {
	p, err := newPrivateProviderProbe(probePath(t))
	if err != nil {
		t.Fatal(err)
	}
	p.observe("DOCUMENT_SYMBOL", "DECODE_FAILURE", "DECODE_FAILURE", false)
	p.observe("DOCUMENT_SYMBOL", "DECODE_FAILURE", "DECODE_FAILURE", false)
	p.observe("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", "DOCUMENT_SOURCE_UNAVAILABLE", true)
	p.observe("DOCUMENT_SUPPLY", "DOCUMENT_FAILURE", "DOCUMENT_URI_UNAVAILABLE", true)
	if err = p.finalize(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p.path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := parsePrivateProviderProbe(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.Schema != privateProviderProbeSchema || a.Status != "COMPLETE" || a.FileDenominator != 2 || a.RerunCount != 0 || a.Work != 4 || len(a.Records) != 2 || a.Records[0].Phase != "DOCUMENT_SYMBOL" || a.Records[1].Phase != "DOCUMENT_SUPPLY" || a.Records[0].Sequence >= a.Records[1].Sequence {
		t.Fatalf("%+v", a)
	}
	want := []privateProviderProbeCount{{"DECODE_FAILURE", 2}, {"DOCUMENT_SOURCE_UNAVAILABLE", 1}, {"DOCUMENT_URI_UNAVAILABLE", 1}}
	if !reflect.DeepEqual(a.CauseCounts, want) {
		t.Fatalf("%+v", a.CauseCounts)
	}
	for _, s := range []string{"file://", "/private/", "sha256", "session_id", "generation", "forbidden", "message", "params", "result", "stderr"} {
		if strings.Contains(string(raw), s) {
			t.Fatalf("privacy sentinel %q in %s", s, raw)
		}
	}
	var loose map[string]any
	if json.Unmarshal(raw, &loose) != nil {
		t.Fatal("json")
	}
	loose["extra"] = true
	bad, _ := json.Marshal(loose)
	if _, err = parsePrivateProviderProbe(bad); err == nil {
		t.Fatal("unknown field accepted")
	}
	p.mu.Lock()
	p.work = math.MaxUint64
	p.mu.Unlock()
	p.observeSuccess(false)
	if p.artifact().Status != "INCOMPLETE" {
		t.Fatal("overflow not incomplete")
	}
}

func TestPrivateProviderProbeProductionDiscoveryMalformedResultIsOneDecodeFailure(t *testing.T) {
	p := &privateProviderProbe{counts: make(map[string]uint64)}
	r := &privateAcquisitionRuntime{probe: p}
	calls := 0
	client := productionDiscoveryClient{
		runtime: r, sessionID: "session", generation: 1,
		roundTrip: func(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
			calls++
			return sessionruntime.RoundTripResult{Result: json.RawMessage(`{`)}
		},
	}
	if _, err := client.DocumentSymbols(context.Background(), lsp.DocumentSymbolParams{}); err == nil {
		t.Fatal("malformed result accepted")
	}
	a := p.artifact()
	if calls != 1 || a.Work != 1 || p.sequence != 1 || len(a.Records) != 1 || a.Records[0].Sequence != 1 || a.Records[0].Class != "DECODE_FAILURE" || len(a.CauseCounts) != 1 || a.CauseCounts[0] != (privateProviderProbeCount{"DECODE_FAILURE", 1}) {
		t.Fatalf("calls=%d sequence=%d artifact=%+v", calls, p.sequence, a)
	}
}

func TestPrivateProviderProbeRoundTripFinalizationIsExclusiveAndPrecedent(t *testing.T) {
	cases := []struct {
		name       string
		result     sessionruntime.RoundTripResult
		decodeErr  error
		wantCause  string
		wantClass  string
		wantRecord int
	}{
		{"decode", sessionruntime.RoundTripResult{Result: json.RawMessage(`{`)}, errors.New("decode"), "DECODE_FAILURE", "DECODE_FAILURE", 1},
		{"rpc-beats-decode", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32601}, Result: json.RawMessage(`{`)}, errors.New("decode"), "METHOD_NOT_FOUND", "JSON_RPC_CODE_BUCKET", 1},
		{"typed-beats-rpc-and-decode", sessionruntime.RoundTripResult{Failure: session.RequestTimeout, ServerError: &lspwire.RPCError{Code: -32601}, Result: json.RawMessage(`{`)}, errors.New("decode"), string(session.RequestTimeout), "TYPED_SESSION_RUNTIME_FAILURE", 1},
		{"success", sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}, nil, "", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &privateProviderProbe{counts: make(map[string]uint64)}
			r := &privateAcquisitionRuntime{probe: p}
			r.observeRoundTrip("textDocument/documentSymbol", tc.result, tc.decodeErr)
			a := p.artifact()
			if a.Work != 1 || p.sequence != 1 || len(a.Records) != tc.wantRecord || len(a.CauseCounts) != tc.wantRecord {
				t.Fatalf("artifact=%+v sequence=%d", a, p.sequence)
			}
			if tc.wantRecord == 1 && (a.Records[0].Sequence != 1 || a.Records[0].Cause != tc.wantCause || a.Records[0].Class != tc.wantClass || a.CauseCounts[0].Count != 1) {
				t.Fatalf("artifact=%+v", a)
			}
		})
	}
}

func TestPrivateProviderProbeRoundTripSequenceAndFullCountsOutliveRetention(t *testing.T) {
	p := &privateProviderProbe{counts: make(map[string]uint64)}
	r := &privateAcquisitionRuntime{probe: p}
	r.observeRoundTrip("textDocument/documentSymbol", sessionruntime.RoundTripResult{Result: json.RawMessage(`{`)}, errors.New("decode"))
	r.observeRoundTrip("textDocument/prepareCallHierarchy", sessionruntime.RoundTripResult{ServerError: &lspwire.RPCError{Code: -32603}}, nil)
	r.observeRoundTrip("callHierarchy/outgoingCalls", sessionruntime.RoundTripResult{Failure: session.ResourceExhausted}, nil)
	a := p.artifact()
	if a.Work != 3 || len(a.Records) != 1 || a.Records[0].Sequence != 1 {
		t.Fatalf("artifact=%+v", a)
	}
	want := []privateProviderProbeCount{{"DECODE_FAILURE", 1}, {"INTERNAL_ERROR", 1}, {string(session.ResourceExhausted), 1}}
	if !reflect.DeepEqual(a.CauseCounts, want) {
		t.Fatalf("counts=%+v", a.CauseCounts)
	}
}

func TestPrivateProviderProbeCustodyAndWriteFailureAreClosed(t *testing.T) {
	d := t.TempDir()
	if _, err := newPrivateProviderProbe(filepath.Join(d, "x")); err == nil {
		t.Fatal("0755 parent accepted")
	}
	path := probePath(t)
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := newPrivateProviderProbe(path); err == nil {
		t.Fatal("0644 file accepted")
	}
	path = probePath(t)
	p, err := newPrivateProviderProbe(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err = p.finalize(); err == nil {
		t.Fatal("write custody failure accepted")
	}
}
