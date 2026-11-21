//go:build darwin || linux

package main

import (
	"errors"
	"io"
	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/internal/censusresult"
)

func writeCensusCapacityFailure(w io.Writer, err error) bool {
	var refusal *ai.CapacityRefusal
	if !errors.As(err, &refusal) {
		return false
	}
	d := censusresult.CapacityDiagnostic{SchemaVersion: censusresult.CapacityDiagnosticSchemaVersion, Operation: "start", Reason: refusal.Reason, Action: "STOP"}
	if m := refusal.Metrics; m != nil {
		d.Metrics = &censusresult.CapacityDiagnosticMetrics{Unit: m.Unit, Spent: m.Spent, Protected: m.Protected, Limit: m.Limit, Available: m.Available, Deficit: m.Deficit}
	}
	raw, e := censusresult.MarshalCapacityDiagnostic(d)
	if e != nil {
		return false
	} // existing generic failure remains fail-closed
	_, _ = w.Write(raw)
	return true
}
