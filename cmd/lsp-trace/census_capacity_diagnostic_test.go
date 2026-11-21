//go:build darwin || linux

package main

import (
	"bytes"
	"errors"
	ai "lsp-trace/internal/applicationinventory"
	cr "lsp-trace/internal/censusresult"
	"testing"
)

func TestCensusCapacityDiagnosticCarrier(t *testing.T) {
	refusal := &ai.CapacityRefusal{Reason: ai.CapacityBytes, Metrics: &ai.CapacityMetrics{Unit: "bytes", Spent: 14825416, Protected: 54463091, Limit: 67108864, Available: 52283448, Deficit: 2179643}}
	var out bytes.Buffer
	err := errors.Join(refusal, errors.New("secret/path identity env content"))
	if code := writeCensusFailure(censusOperationDiagnosticWriter{&out}, censusStageAssembly, true, err); code != 1 {
		t.Fatal(code)
	}
	d, e := cr.DecodeCapacityDiagnostic(out.Bytes())
	if e != nil || d.Metrics == nil || d.Metrics.Deficit != 2179643 {
		t.Fatalf("typed carrier: %v %s", e, out.String())
	}
	if bytes.Contains(out.Bytes(), []byte("secret")) || bytes.Contains(out.Bytes(), []byte("run_id")) {
		t.Fatal("leak")
	}
	out.Reset()
	writeCensusFailure(censusOperationDiagnosticWriter{&out}, censusStageAssembly, true, errors.New("untyped"))
	if _, e := cr.DecodeOperationDiagnostic(out.Bytes()); e != nil {
		t.Fatal("generic diagnostic changed", e)
	}
}
