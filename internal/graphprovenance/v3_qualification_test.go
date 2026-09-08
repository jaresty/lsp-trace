package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/manageddiagnostic"
)

type qualificationDiagnostics struct {
	queries int
	result  manageddiagnostic.QueryResult
}

func (f *qualificationDiagnostics) Diagnostics(string, uint64) manageddiagnostic.QueryResult {
	f.queries++
	return f.result
}

func qualifyV3(v2 []byte, session string, generation uint64, f *qualificationDiagnostics, inherited error) ([]byte, error) {
	if inherited != nil || v2 == nil {
		return nil, inherited
	}
	q := f.Diagnostics(session, generation)
	return CaptureV3(v2, session, generation, q)
}

func TestFR23DeterministicPublicOutcomeMatrix(t *testing.T) {
	result, root := coordinatorV2Fixture(t, nil)
	v2, err := CaptureV2(context.Background(), result, root)
	if err != nil {
		t.Fatal(err)
	}
	session, generation := result.Request.Context.SessionID, result.Request.Context.Generation
	requestRecord := func(terminal manageddiagnostic.Terminal) manageddiagnostic.Record {
		r := manageddiagnostic.RecordRequest(manageddiagnostic.RequestObservation{
			SessionID: session, Generation: generation, Sequence: 1, Method: "textDocument/prepareCallHierarchy",
			RequestedDeadline: 1, EffectiveDeadline: 1, RequestedMaxBytes: 1024, EffectiveMaxBytes: 1024,
			RequestedMaxMessages: 1, EffectiveMaxMessages: 1, Clock: func() int64 { return 7 },
		}, func() manageddiagnostic.RequestOutcome {
			out := manageddiagnostic.RequestOutcome{Terminal: terminal, ReadState: manageddiagnostic.IOFailed, WriteState: manageddiagnostic.IOComplete, RequestBytes: 12}
			if terminal == manageddiagnostic.TerminalResponseReceived || terminal == manageddiagnostic.TerminalProtocolError {
				out.ReadState, out.ResponseMessages, out.ResponseBytes = manageddiagnostic.IOComplete, 1, 2
			}
			if terminal == manageddiagnostic.TerminalProtocolError {
				out.NumericRPCCode = manageddiagnostic.Fact[int]{Status: manageddiagnostic.Observed, Value: -32601}
			}
			return out
		})
		if terminal == manageddiagnostic.TerminalProcessExited {
			r.ProcessExit = manageddiagnostic.ProcessExit{Status: manageddiagnostic.Observed, ExitCode: manageddiagnostic.Fact[int]{Status: manageddiagnostic.Observed, Value: 86}, ObservedBeforeCleanup: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Observed, Value: true}, CleanupInduced: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Observed}}
		}
		return r
	}
	capability := manageddiagnostic.RecordCapabilityCheck(manageddiagnostic.CapabilityObservation{SessionID: session, Generation: generation, Sequence: 1, Supported: false})
	capability.Read.State, capability.Write.State = manageddiagnostic.IOUnavailable, manageddiagnostic.IOUnavailable
	document := manageddiagnostic.RecordDocumentSupply(manageddiagnostic.DocumentSupplyObservation{SessionID: session, Generation: generation, Sequence: 1, Completed: true, Method: "textDocument/didOpen"})
	document.Read.State, document.Write = manageddiagnostic.IOUnavailable, manageddiagnostic.IOFacts{State: manageddiagnostic.IOComplete, Messages: 1, Bytes: 2}

	rows := []struct {
		name string
		q    manageddiagnostic.QueryResult
	}{
		{"success", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{requestRecord(manageddiagnostic.TerminalResponseReceived)}}},
		{"protocol-error", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{requestRecord(manageddiagnostic.TerminalProtocolError)}}},
		{"deadline", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{requestRecord(manageddiagnostic.TerminalDeadlineExceeded)}}},
		{"process-exit", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{requestRecord(manageddiagnostic.TerminalProcessExited)}}},
		{"transport-closure", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{requestRecord(manageddiagnostic.TerminalTransportClosed)}}},
		{"capability-unsupported", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{capability}}},
		{"document-supply", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: []manageddiagnostic.Record{document}}},
		{"unavailable", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}},
		{"evicted", manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryEvicted, Records: []manageddiagnostic.Record{}, EvictedRecords: 1}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			fake := &qualificationDiagnostics{result: row.q}
			raw, err := qualifyV3(v2, session, generation, fake, nil)
			if err != nil || fake.queries != 1 {
				t.Fatalf("ASSERT_FR23_V3_OUTCOME_QUERY_ONCE: queries=%d err=%v", fake.queries, err)
			}
			if _, err := ValidateFor(raw, Family, "v3"); err != nil {
				t.Fatalf("ASSERT_FR23_V3_OUTCOME_ADMITTED: %v", err)
			}
			var evidence EvidenceV3
			if err := json.Unmarshal(raw, &evidence); err != nil {
				t.Fatal(err)
			}
			projected, err := json.Marshal(evidence.Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			lower := bytes.ToLower(projected)
			for _, forbidden := range []string{"raw_stderr", "rpc_params", "source", "environment", "command", "arguments", "path", "uri", "matched", "unmatched", "late", "read-loop", "write-start", "write-complete", "secret"} {
				if bytes.Contains(lower, []byte(forbidden)) {
					t.Fatalf("ASSERT_FR23_V3_OUTCOME_PRIVACY/%s", forbidden)
				}
			}
			var doc map[string]any
			if json.Unmarshal(raw, &doc) != nil || !strings.HasPrefix(doc["schema_version"].(string), "lsp-trace.graph-provenance.v3") {
				t.Fatal("ASSERT_FR23_V3_OUTCOME_PUBLIC_ARTIFACT")
			}
		})
	}
	for _, name := range []string{"pre-v2-protocol", "pre-v2-deadline", "pre-v2-process-exit", "pre-v2-transport", "pre-v2-capability", "pre-v2-document-unavailable"} {
		t.Run(name, func(t *testing.T) {
			inherited := errors.New("inherited-no-artifact")
			fake := &qualificationDiagnostics{}
			raw, err := qualifyV3(nil, session, generation, fake, inherited)
			if raw != nil || !errors.Is(err, inherited) || fake.queries != 0 {
				t.Fatalf("ASSERT_FR23_V3_PRE_V2_INHERITED: raw=%d queries=%d err=%v", len(raw), fake.queries, err)
			}
		})
	}
}
