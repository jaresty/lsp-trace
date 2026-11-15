package main

import (
	"errors"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/censussourcedriver"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/sessionruntime"
)

// censusInventoryStoppedDiagnostic is the connected bridge's pure failure mapper.
// It observes values only: it cannot recover, index, advance custody, or grant retry.
func censusInventoryStoppedDiagnostic(stop censussourcedriver.Stop, cause error) (censusresult.Diagnostic, error) {
	d, known := inv.PrivateDiscoveryOperationRefusalV2(cause)
	if !known {
		d, known = sessionruntime.PrivateDiscoveryRefusalFromError(cause)
	}
	if known {
		return censusInventoryStoppedDiagnosticValue(stop, &d)
	}
	return censusInventoryStoppedDiagnosticValue(stop, nil)
}

// This value boundary permits testing refusal coexistence without constructing
// an operation, Manager, reservation, or any authority-bearing error producer.
func censusInventoryStoppedDiagnosticValue(stop censussourcedriver.Stop, refusal *sessionruntime.PrivateDiscoveryRefusal) (censusresult.Diagnostic, error) {
	// Stop is authoritative for this diagnostic even when Err is nil or opaque.
	// Retained may represent PARTIAL, UNKNOWN, REFUSED, or FAILED; do not strengthen it.
	var code censusresult.Code
	switch stop {
	case censussourcedriver.StopRetained:
		code = censusresult.CodeDiscoveryRetained
	case censussourcedriver.StopIndeterminateExecution:
		code = censusresult.CodeDiscoveryIndeterminate
	}
	if code == "" && refusal == nil {
		return censusresult.NewDiagnostic(censusresult.StageConfig, nil)
	}
	out, err := censusresult.NewDiagnostic(censusresult.StageDiscovery, nil)
	if err != nil {
		return out, err
	}
	if code != "" {
		out.Code = code
	}
	out.Retry = false
	if refusal != nil {
		if !refusal.Valid() {
			return censusresult.Diagnostic{}, errors.New("invalid private discovery diagnostic")
		}
		value := *refusal // immutable value snapshot; no caller-owned alias
		out.PrivateDiscoveryRefusal = &value
	}
	return out, nil
}
