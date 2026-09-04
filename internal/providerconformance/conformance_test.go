package providerconformance

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	assertInput     = "ASSERT_GENERIC_CONFORMANCE_HOST_DECLARATION"
	assertWire      = "ASSERT_GENERIC_CONFORMANCE_ONE_FRAME_SCHEMA_IDENTITY_CUSTODY"
	assertLifecycle = "ASSERT_GENERIC_CONFORMANCE_BOUNDS_CANCEL_REAP"
	assertReplay    = "ASSERT_GENERIC_CONFORMANCE_DETERMINISTIC_REPLAY"
	assertOutcomes  = "ASSERT_GENERIC_CONFORMANCE_EXPLICIT_OUTCOMES"
)

func TestMain(m *testing.M) {
	if os.Getenv("LSP_TRACE_CONFORMANCE_HELPER") != "" {
		runProviderConformanceHelper()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestProviderConformanceHelper(t *testing.T) {}

func runProviderConformanceHelper() {
	body, err := readFrame(os.Stdin)
	if err != nil {
		os.Exit(3)
	}
	var req Request
	if json.Unmarshal(body, &req) != nil {
		os.Exit(4)
	}
	mode := os.Getenv("LSP_TRACE_CONFORMANCE_MODE")
	if mode == "hang" {
		for {
			time.Sleep(time.Hour)
		}
	}
	outcome := OutcomePassed
	observations := []json.RawMessage{json.RawMessage(`{"kind":"generic"}`)}
	if mode == "empty" {
		outcome, observations = OutcomeEmpty, nil
	}
	if mode == "partial" {
		outcome = OutcomePartial
	}
	if mode == "unsupported" {
		outcome = OutcomeUnsupported
	}
	if mode == "failed" {
		outcome = OutcomeFailed
	}
	response := map[string]any{
		"schema_version": "lsp-trace.provider-conformance-response.v1",
		"request_id":     req.RequestID,
		"provider":       req.Provider,
		"protocol":       req.Protocol,
		"outcome":        outcome,
		"custody":        req.Custody,
		"observations":   observations,
		"diagnostics":    []string{},
	}
	if mode == "wrong-identity" {
		response["request_id"] = "wrong"
	}
	if mode == "wrong-custody" {
		response["custody"] = Custody{OriginalURI: "file:///other", OriginalRevision: "r1", OriginalDigest: "sha256:other"}
	}
	if mode == "too-many" {
		response["observations"] = make([]json.RawMessage, 9)
	}
	encoded, _ := json.Marshal(response)
	fmt.Printf("Content-Length: %d\r\n\r\n%s", len(encoded), encoded)
	if mode == "two-frames" {
		fmt.Printf("Content-Length: %d\r\n\r\n%s", len(encoded), encoded)
	}
}

func readFrame(r io.Reader) ([]byte, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	var n int
	if _, err = fmt.Sscanf(line, "Content-Length: %d\r\n", &n); err != nil {
		return nil, err
	}
	if line, err = br.ReadString('\n'); err != nil || line != "\r\n" {
		return nil, fmt.Errorf("separator")
	}
	body := make([]byte, n)
	_, err = io.ReadFull(br, body)
	return body, err
}

func validFixture(t *testing.T, mode string) (Config, Request) {
	t.Helper()
	exe, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	limits := Limits{RequestBytes: 64 << 10, ResponseBytes: 64 << 10, Messages: 1, Observations: 8, Diagnostics: 8, StderrBytes: 4096, WallTime: 10 * time.Second, TerminationGrace: 100 * time.Millisecond}
	provider := Identity{Name: "generic-provider", Version: "1"}
	protocol := Identity{Name: "generic-protocol", Version: "1"}
	custody := Custody{OriginalURI: "file:///source", OriginalRevision: "r1", OriginalDigest: "sha256:original", VirtualURI: "virtual:///source", VirtualRevision: "r1", VirtualDigest: "sha256:virtual", VirtualOriginalURI: "file:///source"}
	cfg := Config{Executable: exe, Arguments: []string{"-test.run=TestProviderConformanceHelper"}, Environment: []string{"LSP_TRACE_CONFORMANCE_HELPER=1", "LSP_TRACE_CONFORMANCE_MODE=" + mode}, Provider: provider, Protocol: protocol, Capabilities: Capabilities{Operations: []string{"observe"}}, Limits: limits}
	req := Request{SchemaVersion: RequestSchema, RequestID: "request-1", Provider: provider, Protocol: protocol, Capabilities: cfg.Capabilities, Limits: limits, Custody: custody, Payload: json.RawMessage(`{"operation":"observe"}`)}
	return cfg, req
}

func TestGenericProviderConformance(t *testing.T) {
	t.Run(assertInput, func(t *testing.T) {
		cfg, req := validFixture(t, "passed")
		cfg.Executable = "relative"
		if got := Run(context.Background(), cfg, req); got.Outcome == OutcomeFailed && checkPassed(got, "declaration") {
			t.Fatalf("%s: relative executable accepted: %#v", assertInput, got)
		}
		cfg, req = validFixture(t, "passed")
		if got := Run(context.Background(), cfg, req); got.Outcome != OutcomePassed {
			t.Fatalf("%s: valid declaration rejected: %#v", assertInput, got)
		}
	})
	t.Run(assertWire, func(t *testing.T) {
		cfg, req := validFixture(t, "wrong-identity")
		if got := Run(context.Background(), cfg, req); got.Outcome != OutcomeFailed || checkPassed(got, "identity") {
			t.Fatalf("%s: mismatch not rejected: %#v", assertWire, got)
		}
		for _, mode := range []string{"wrong-custody", "two-frames"} {
			cfg, req = validFixture(t, mode)
			if got := Run(context.Background(), cfg, req); got.Outcome != OutcomeFailed {
				t.Fatalf("%s: mode=%s accepted: %#v", assertWire, mode, got)
			}
		}
		cfg, req = validFixture(t, "passed")
		if got := Run(context.Background(), cfg, req); got.Outcome != OutcomePassed || !checkPassed(got, "frame") || !checkPassed(got, "schema") || !checkPassed(got, "identity") || !checkPassed(got, "custody") {
			t.Fatalf("%s: valid response rejected: %#v", assertWire, got)
		}
	})
	t.Run(assertLifecycle, func(t *testing.T) {
		cfg, req := validFixture(t, "hang")
		cfg.Limits.WallTime = 20 * time.Millisecond
		req.Limits = cfg.Limits
		start := time.Now()
		got := Run(context.Background(), cfg, req)
		if got.Outcome != OutcomeUnavailable || !got.Terminated || !got.Reaped || time.Since(start) > time.Second {
			t.Fatalf("%s: timeout=%#v elapsed=%s", assertLifecycle, got, time.Since(start))
		}
		cfg, req = validFixture(t, "hang")
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		if got = Run(cancelCtx, cfg, req); got.Outcome != OutcomeUnavailable {
			t.Fatalf("%s: cancellation=%#v", assertLifecycle, got)
		}
		cfg, req = validFixture(t, "too-many")
		if got = Run(context.Background(), cfg, req); got.Outcome != OutcomeFailed || checkPassed(got, "bounds") {
			t.Fatalf("%s: bounds=%#v", assertLifecycle, got)
		}
	})
	t.Run(assertReplay, func(t *testing.T) {
		cfg, req := validFixture(t, "passed")
		got := Run(context.Background(), cfg, req)
		if got.Outcome != OutcomePassed || !checkPassed(got, "replay") || got.ResponseDigest == "" {
			t.Fatalf("%s: %#v", assertReplay, got)
		}
	})
	t.Run(assertOutcomes, func(t *testing.T) {
		for _, outcome := range []Outcome{OutcomeUnsupported, OutcomePartial, OutcomeEmpty, OutcomeFailed} {
			cfg, req := validFixture(t, string(outcome))
			got := Run(context.Background(), cfg, req)
			if got.Outcome != outcome {
				t.Fatalf("%s: mode=%s got=%#v", assertOutcomes, outcome, got)
			}
		}
		cfg, req := validFixture(t, "passed")
		cfg.Executable = filepath.Join(t.TempDir(), "missing")
		if got := Run(context.Background(), cfg, req); got.Outcome != OutcomeUnavailable {
			t.Fatalf("%s: unavailable=%#v", assertOutcomes, got)
		}
	})
}

func checkPassed(report Report, name string) bool {
	for _, c := range report.Checks {
		if c.Name == name {
			return c.Passed
		}
	}
	return false
}

func TestFrameworkNeutralSource(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(body))
		for _, forbidden := range []string{"em" + "ber", "gli" + "nt"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("ASSERT_GENERIC_CONFORMANCE_FRAMEWORK_NEUTRAL: %s contains forbidden vocabulary %q", entry.Name(), forbidden)
			}
		}
	}
}
