//go:build darwin

package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

const methodWirePeerEnv = "ADR0011_METHOD_WIRE_PEER"
const malformedSecondLocation = `{"uri":"file:///w/b.go","uri":"file:///w/c.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`

// The test binary is a test-owned process, not a production fake LSP or a
// provider-authenticated source of method or occurrence evidence.
func TestADR0011ManagedMethodPeer(t *testing.T) {
	if os.Getenv(methodWirePeerEnv) != "1" {
		return
	}
	reader := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(os.Stdout, lspwire.DefaultLimits())
	for {
		message, err := reader.Read()
		if err != nil {
			os.Exit(2)
		}
		var raw json.RawMessage
		switch message.Method {
		case "initialize":
			if os.Getenv("ADR0011_METHOD_WIRE_CAPS") == "none" {
				raw = json.RawMessage(`{"capabilities":{"positionEncoding":"utf-16","definitionProvider":false,"referencesProvider":false},"serverInfo":{"name":"adr0011-test-peer","version":"1"}}`)
			} else {
				raw = json.RawMessage(`{"capabilities":{"positionEncoding":"utf-16","definitionProvider":true,"referencesProvider":true},"serverInfo":{"name":"adr0011-test-peer","version":"1"}}`)
			}
		case "initialized":
			continue
		case transport.MethodDefinition:
			switch os.Getenv("ADR0011_METHOD_WIRE_CASE") {
			case "D-04":
				raw = json.RawMessage(`[` + loc + `,` + loc + `]`)
			case "D-05":
				raw = json.RawMessage(`[` + link + `,` + link + `]`)
			case "D-07":
				raw = json.RawMessage(`[` + loc + `,` + malformedSecondLocation + `]`)
			default:
				raw = json.RawMessage(`null`)
			}
		case transport.MethodReferences:
			var p struct {
				Context *struct {
					IncludeDeclaration *bool `json:"includeDeclaration"`
				} `json:"context"`
			}
			if json.Unmarshal(message.Params, &p) != nil || p.Context == nil || p.Context.IncludeDeclaration == nil || *p.Context.IncludeDeclaration {
				os.Exit(3)
			}
			switch os.Getenv("ADR0011_METHOD_WIRE_CASE") {
			case "R-05":
				raw = json.RawMessage(`[` + loc + `,` + loc + `]`)
			case "R-07":
				raw = json.RawMessage(`[` + loc + `,` + malformedSecondLocation + `]`)
			default:
				raw = json.RawMessage(`[]`)
			}
		case "shutdown":
			raw = json.RawMessage(`null`)
		case "exit":
			os.Exit(0)
		default:
			if len(message.ID) == 0 {
				continue
			}
			_ = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Error: &lspwire.RPCError{Code: -32601, Message: "test method not found"}})
			continue
		}
		if message.Method == transport.MethodDefinition || message.Method == transport.MethodReferences {
			trace := os.Getenv("ADR0011_METHOD_WIRE_TRACE")
			line := fmt.Sprintf("%s|sha256:%x\n", message.Method, sha256.Sum256(message.Params))
			f, err := os.OpenFile(trace, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				os.Exit(4)
			}
			if _, err := f.WriteString(line); err != nil {
				_ = f.Close()
				os.Exit(4)
			}
			if err := f.Close(); err != nil {
				os.Exit(4)
			}
		}
		if err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: raw}); err != nil {
			os.Exit(5)
		}
	}
}

func startManagedMethodPeer(t *testing.T, modes ...string) (*sessionruntime.Manager, sessionruntime.StartResult, string) {
	return startManagedMethodPeerWithCapabilities(t, true, modes...)
}

func startManagedMethodPeerWithCapabilities(t *testing.T, advertised bool, modes ...string) (*sessionruntime.Manager, sessionruntime.StartResult, string) {
	t.Helper()
	mode := ""
	if len(modes) != 0 {
		mode = modes[0]
	}
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 16},
		Starter: sessionruntime.ManagedStarter{Manager: supervisor},
	})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/workspace", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(t.TempDir(), "method-trace")
	capabilities := "both"
	if !advertised {
		capabilities = "none"
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{
		Profile: runtimeprofile.Resolve(validated),
		Process: managedprocess.Spec{Path: executable, Args: []string{"-test.run=^TestADR0011ManagedMethodPeer$"},
			Env: append(os.Environ(), methodWirePeerEnv+"=1", "ADR0011_METHOD_WIRE_TRACE="+trace, "ADR0011_METHOD_WIRE_CASE="+mode, "ADR0011_METHOD_WIRE_CAPS="+capabilities)},
	})
	if started.Failure != "" || started.Start.Evidence != managedprocess.LocalDarwinSupervisionOnly {
		t.Fatalf("ASSERT_ADR0011_MANAGED_START: %+v", started)
	}
	t.Cleanup(func() {
		stop := manager.Stop(context.Background(), started.SessionID, "adr0011-managed-fixture")
		if stop.Failure != "" {
			t.Errorf("ASSERT_ADR0011_MANAGED_STOP: %+v", stop)
			return
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			operation, ok := manager.Operation(stop.IntentID)
			if ok && operation.State != sessionruntime.OperationPending {
				if operation.State != sessionruntime.OperationComplete || operation.Failure != "" {
					t.Errorf("ASSERT_ADR0011_MANAGED_REAP: %+v", operation)
				}
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Error("ASSERT_ADR0011_MANAGED_REAP: operation deadline exceeded")
	})
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(10*time.Second))
	if pending.State != sessionruntime.ReadinessPending {
		t.Fatalf("ASSERT_ADR0011_MANAGED_READINESS_PENDING: %+v", pending)
	}
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" ||
		ready.Metadata.DefinitionSupport != advertised || ready.Metadata.ReferencesSupport != advertised {
		t.Fatalf("ASSERT_ADR0011_MANAGED_CAPABILITY_READY: found=%v state=%s failure=%s definition=%v references=%v want=%v", found, ready.State, ready.Failure, ready.Metadata.DefinitionSupport, ready.Metadata.ReferencesSupport, advertised)
	}
	return manager, started, trace
}

// Captures the real manager return for a test assertion without enabling raw
// request/response frame retention or changing the private transport result.
type methodWireObservedRuntime struct {
	*sessionruntime.Manager
	last sessionruntime.RoundTripResult
}

func (r *methodWireObservedRuntime) RoundTrip(ctx context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	r.last = r.Manager.RoundTrip(ctx, req)
	return r.last
}

func TestADR0011CanonicalQueryResultCandidateWithoutFrames(t *testing.T) {
	for _, tc := range []struct {
		method, mode, raw string
		count             int
		null              bool
	}{
		{transport.MethodDefinition, "", `null`, 0, true},
		{transport.MethodReferences, "", `[]`, 0, false},
		{transport.MethodReferences, "R-05", `[` + loc + `,` + loc + `]`, 2, false},
	} {
		t.Run(tc.method+"/"+tc.mode, func(t *testing.T) {
			manager, started, _ := startManagedMethodPeer(t, tc.mode)
			observed := &methodWireObservedRuntime{Manager: manager}
			req := fixtureRequest(tc.method, false)
			req.SessionID, req.Generation = started.SessionID, started.Generation
			req.Deadline = time.Now().Add(5 * time.Second)
			wire := transport.New(observed).Execute(context.Background(), req)
			candidate, err := BuildCanonicalCandidate(req, wire, 8)
			if err != nil {
				t.Fatalf("bounded managed candidate failed: %v outcome=%s", err, wire.Outcome())
			}
			verified, err := VerifyCanonicalCandidate(candidate)
			result, resultErr := CandidateResultBytes(candidate)
			if err != nil || resultErr != nil || verified.ItemCount != tc.count || verified.Null != tc.null || !bytes.Equal(result, []byte(tc.raw)) || verified.QueryURI != "file:///w/q.go" || verified.Generation != started.Generation {
				t.Fatalf("unadmitted replay mismatch: %+v %v %v", verified, err, resultErr)
			}
			queryID := "predeclared-query"
			form := EnvelopeArray
			if tc.null {
				form = EnvelopeNull
			}
			member := CandidateQueryMember{OccurrenceID: queryID, Began: true,
				Envelope: EnvelopeObservation{Known: true, Form: form, Count: tc.count}, Terminal: CandidateEmpty}
			if tc.count != 0 {
				member.Terminal = CandidateItems
				for ordinal := 0; ordinal < tc.count; ordinal++ {
					member.Elements = append(member.Elements, CandidateElement{Ordinal: ordinal, Began: true, Terminal: CandidateValidElement})
				}
			}
			ledger := CandidateLedger{Queries: []CandidateQuery{{OccurrenceID: queryID, SessionID: req.SessionID,
				Generation: req.Generation, Method: req.Method, ParamsSHA256: verified.ParamsSHA256}},
				Members: []CandidateQueryMember{member}}
			if err := CheckCandidateTransactionLedger(candidate, ledger, queryID); err != nil {
				t.Fatalf("candidate/ledger balance rejected: %v", err)
			}
			wrong := ledger
			wrong.Members = append([]CandidateQueryMember(nil), ledger.Members...)
			wrong.Members[0].Envelope.Count++
			if err := CheckCandidateTransactionLedger(candidate, wrong, queryID); !errors.Is(err, ErrCandidateJoin) {
				t.Fatalf("fabricated result element accepted: %v", err)
			}
			wrong = ledger
			wrong.Queries = append([]CandidateQuery(nil), ledger.Queries...)
			wrong.Queries[0].OccurrenceID = "result-inferred-target"
			if err := CheckCandidateTransactionLedger(candidate, wrong, queryID); !errors.Is(err, ErrCandidateJoin) {
				t.Fatalf("substituted query occurrence accepted: %v", err)
			}
			if _, ok := observed.last.CompletedMethodRequestFrame(); ok {
				t.Fatal("candidate unexpectedly required optional request frame")
			}
			if _, ok := observed.last.CompletedMethodResponseFrame(); ok {
				t.Fatal("candidate unexpectedly required optional response frame")
			}
			changed := append([]byte(nil), candidate...)
			changed[len(changed)-2] ^= 1
			if _, err := VerifyCanonicalCandidate(changed); !errors.Is(err, ErrInvalidCanonicalCandidate) {
				t.Fatalf("altered canonical bytes accepted: %v", err)
			}
			wrongQuery := verified
			wrongQuery.QueryURI = "file:///w/other.go"
			wrongQuery.ID = ""
			pre, _ := json.Marshal(wrongQuery)
			wrongQuery.ID = queryResultCandidateDigest(pre)
			forged, _ := json.Marshal(wrongQuery)
			if _, err := VerifyCanonicalCandidate(append(forged, '\n')); !errors.Is(err, ErrInvalidCanonicalCandidate) {
				t.Fatalf("query substitution accepted after recomputing digest: %v", err)
			}
			if tc.null {
				// Coordinated same-process replacement of payload, parse fields and
				// candidate digest remains possible. Replay is not authentication.
				substituted := verified
				substituted.ResultBase64 = base64.StdEncoding.EncodeToString([]byte(`[]`))
				substituted.ResultSHA256 = rawSHA([]byte(`[]`))
				substituted.Null = false
				substituted.ID = ""
				pre, _ := json.Marshal(substituted)
				substituted.ID = queryResultCandidateDigest(pre)
				forged, _ := json.Marshal(substituted)
				if _, err := VerifyCanonicalCandidate(append(forged, '\n')); err != nil {
					t.Fatalf("falsification control did not demonstrate ceiling: %v", err)
				}
				forgedLedger := ledger
				forgedLedger.Members = append([]CandidateQueryMember(nil), ledger.Members...)
				forgedLedger.Members[0].Envelope.Form = EnvelopeArray
				if err := CheckCandidateTransactionLedger(append(forged, '\n'), forgedLedger, queryID); err != nil {
					t.Fatalf("joined falsification control did not demonstrate ceiling: %v", err)
				}
			}
		})
	}
}

func TestADR0011ManagedMethodWireNullAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		name, method, raw string
		include           bool
		wantNull          bool
	}{
		{"D-03-managed-null", transport.MethodDefinition, `null`, false, true},
		{"R-02-managed-empty", transport.MethodReferences, `[]`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, started, trace := startManagedMethodPeer(t)
			req := fixtureRequest(tc.method, tc.include)
			req.SessionID, req.Generation = started.SessionID, started.Generation
			req.Deadline = time.Now().Add(5 * time.Second)
			observedRuntime := &methodWireObservedRuntime{Manager: manager}
			wire := transport.New(observedRuntime).Execute(context.Background(), req)
			parsed, failed := Parse(wire, 3)
			obs, present := wire.Observation()
			if wire.Outcome() != transport.OutcomeTransportSuccess || !present || failed != nil ||
				string(wire.Raw()) != tc.raw || !obs.ReportedMethodAdvertised ||
				obs.LocalWriteCorrespondence != transport.LocalWriteMatch ||
				obs.RawResultDisposition != transport.RawResultRetained ||
				obs.DeclaredMethod != tc.method || obs.DeclaredGeneration != started.Generation ||
				len(parsed.Items) != 0 || parsed.Null != tc.wantNull {
				t.Fatalf("ASSERT_ADR0011_MANAGED_WIRE_PARSE: case=%s outcome=%s present=%v failure=%v raw=%s localWrite=%s null=%v items=%d", tc.name, wire.Outcome(), present, failed, wire.Raw(), obs.LocalWriteCorrespondence, parsed.Null, len(parsed.Items))
			}
			if tc.method == transport.MethodReferences && (!obs.IncludeDeclarationPresent || obs.DeclaredIncludeDeclaration) {
				t.Fatal("ASSERT_ADR0011_MANAGED_REFERENCES_CONTEXT: false includeDeclaration not preserved")
			}
			if _, ok := observedRuntime.last.CompletedMethodRequestFrame(); ok {
				t.Fatal("ASSERT_ADR0011_DEFAULT_OFF_REQUEST_FRAME: unexpected optional capture")
			}
			if _, ok := observedRuntime.last.CompletedMethodResponseFrame(); ok {
				t.Fatal("ASSERT_ADR0011_DEFAULT_OFF_RESPONSE_FRAME: unexpected optional capture")
			}
			if _, ok := observedRuntime.last.CompletedRequestWrite(); !ok {
				t.Fatal("ASSERT_ADR0011_COMPLETED_WRITE_WITHOUT_RAW_FRAMES")
			}
			if _, ok := observedRuntime.last.CompletedResponseRead(); !ok {
				t.Fatal("ASSERT_ADR0011_KEYED_READ_WITHOUT_RAW_FRAMES")
			}
			logged, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			wantLine := fmt.Sprintf("%s|sha256:%x", tc.method, sha256.Sum256(req.Params))
			if strings.TrimSpace(string(logged)) != wantLine {
				t.Fatalf("ASSERT_ADR0011_MANAGED_PEER_METHOD: unexpected method or params digest")
			}
		})
	}
}
