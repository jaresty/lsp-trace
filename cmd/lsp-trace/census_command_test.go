package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/publication"
)

func validCensusProjection() censusacquisition.Projection {
	return censusacquisition.Projection{Session: censusacquisition.SessionIdentity{SessionID: "session", Generation: 2}, CensusID: "census", Manifest: captureset.Manifest{LogicalDigest: "capture-set", FileLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private/file.go", Disposition: "selected"}}}, SymbolLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private.symbol", Disposition: "selected"}}}, Targets: []captureset.Target{{CensusOrdinal: 0}}, Batches: []captureset.Batch{{Ordinal: 0, TargetCount: 1}}}}
}
func validCensusReceipt() publication.BoundFileReceipt {
	return publication.BoundFileReceipt{FinalSelector: "private/census.json", Digest: "sha256:" + strings.Repeat("a", 64), ByteLength: 12, VerificationStatus: "VERIFIED", DirectorySyncStatus: publication.DirectorySyncComplete, CloseStatus: publication.CloseComplete}
}

func TestCensusCLIResultClosedDeterministicPrivateProjection(t *testing.T) {
	p := validCensusProjection()
	r, err := buildCensusCLIResult(p, validCensusReceipt())
	if err != nil {
		t.Fatal(err)
	}
	a, err := marshalCensusCLIResult(r)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := marshalCensusCLIResult(r)
	if string(a) != string(b) {
		t.Fatal("ASSERT_CENSUS_DETERMINISTIC_BYTES")
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(a, &raw); err != nil {
		t.Fatal(err)
	}
	want := []string{"authority", "batch_count", "capture_set_id", "census_id", "cross_capture_calls", "file_accounting", "generation", "leiden_admissible", "native_aggregate_custody", "publication", "schema_version", "session_id", "source_graph_complete", "status", "symbol_accounting", "target_count"}
	got := make([]string, 0, len(raw))
	for k := range raw {
		got = append(got, k)
	}
	sortStrings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys=%v", got)
	}
	if strings.Contains(string(a), "private/file.go") || strings.Contains(string(a), "private.symbol") || strings.Contains(string(a), "Workspace") {
		t.Fatal("ASSERT_CENSUS_PRIVATE_VALUE_LEAK")
	}
	if r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.NativeAggregateCustody || r.CrossCaptureCalls == nil || len(r.CrossCaptureCalls) != 0 || r.LeidenAdmissible {
		t.Fatal("ASSERT_CENSUS_FIXED_CEILING")
	}
}
func sortStrings(v []string) {
	for i := range v {
		for j := i + 1; j < len(v); j++ {
			if v[j] < v[i] {
				v[i], v[j] = v[j], v[i]
			}
		}
	}
}

func TestCensusCLIResultRejectsInvalidBoundsPrivacyAndAccounting(t *testing.T) {
	cases := []func(*censusacquisition.Projection, *publication.BoundFileReceipt){
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) { p.Session.Generation = 0 },
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) {
			p.Manifest.FileLedger.Entries[0].Ordinal = 1
		},
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) { r.FinalSelector = "../escape" },
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) { r.Digest = "private" },
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) {
			r.VerificationStatus = "FAILED"
		},
		func(p *censusacquisition.Projection, r *publication.BoundFileReceipt) {
			r.CloseStatus = publication.CloseNotAttempted
		},
	}
	for i, mut := range cases {
		p, r := validCensusProjection(), validCensusReceipt()
		mut(&p, &r)
		if _, err := buildCensusCLIResult(p, r); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestCensusCLIDiagnosticClosedMatrix(t *testing.T) {
	stages := []censusFailureStage{censusStageSyntax, censusStageConfig, censusStageDiscovery, censusStageAcquisition, censusStageAssembly, censusStagePublication, censusStageCommitted}
	for _, s := range stages {
		d, err := buildCensusCLIDiagnostic(s, nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := marshalCensusCLIDiagnostic(d)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(string(raw), "\n") {
			t.Fatal("not jsonl")
		}
		if strings.Contains(string(raw), "error") || s == censusStageCommitted && (d.Status != "SUCCEEDED_DEGRADED" || d.Retry) {
			t.Fatalf("diagnostic=%s", raw)
		}
	}
	n := 3
	if _, err := buildCensusCLIDiagnostic(censusStageAcquisition, &n); err != nil {
		t.Fatal(err)
	}
	if _, err := buildCensusCLIDiagnostic(censusStageDiscovery, &n); err == nil {
		t.Fatal("ordinal accepted outside acquisition")
	}
	if _, err := buildCensusCLIDiagnostic("unknown", nil); err == nil {
		t.Fatal("unknown stage accepted")
	}
}

func TestParseCensusCLIOptionsPureDefaultsMachineAndPreflight(t *testing.T) {
	beforeEnv := os.Environ()
	beforeWD, _ := os.Getwd()
	args := []string{"census", "--machine", "--workspace", "work", "--source", ".", "--publication-root", "/private", "--server", "gopls", "--server-arg", "--machine"}
	o, err := parseCensusCLIOptions(args)
	if err != nil {
		t.Fatal(err)
	}
	if o.DownDepth != 1 || o.UpDepth != 0 || !o.Machine || len(o.ServerArgs) != 1 || o.ServerArgs[0] != "--machine" {
		t.Fatalf("options=%+v", o)
	}
	afterWD, _ := os.Getwd()
	if beforeWD != afterWD || !reflect.DeepEqual(beforeEnv, os.Environ()) {
		t.Fatal("ASSERT_CENSUS_PARSE_SIDE_EFFECT")
	}
	bad := [][]string{{"census", "--machine", "--machine=true"}, {"census", "positional"}, {"census", "--workspace", "w", "--source", ".", "--publication-root", "/p", "--server", "x", "--profile", "p"}, {"census", "--workspace", "w", "--source", ".", "--publication-root", "/p", "--profile", "p", "--down-depth", "-1"}}
	for i, a := range bad {
		if _, err := parseCensusCLIOptions(a); err == nil {
			t.Fatalf("bad %d accepted", i)
		}
	}
	h, err := parseCensusCLIOptions([]string{"census", "--machine=false", "--help"})
	if err != nil || !h.Help || h.Machine {
		t.Fatalf("help=%+v err=%v", h, err)
	}
}
