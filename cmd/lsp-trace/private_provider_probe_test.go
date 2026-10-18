package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
