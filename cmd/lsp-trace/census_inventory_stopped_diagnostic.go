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
func censusInventoryStoppedDiagnostic(stop censussourcedriver.Stop, cause error, snapshots ...sessionruntime.PrivateDiscoveryResponseFacts) (censusresult.Diagnostic, error) {
	d, known := inv.PrivateDiscoveryOperationRefusalV2(cause)
	if !known {
		d, known = sessionruntime.PrivateDiscoveryRefusalFromError(cause)
	}
	var refusal *sessionruntime.PrivateDiscoveryRefusal
	if known {
		refusal = &d
	}
	if len(snapshots) > 1 || len(snapshots) == 1 && !snapshots[0].Valid() {
		return censusresult.Diagnostic{}, errors.New("invalid private discovery diagnostic")
	}
	// Execute may succeed while assembly retains a per-symbol failure with nil
	// cause. An explicit owned snapshot is independent of that error channel.
	// Unavailable is not an observed zero; preserve the legacy getter fallback.
	var facts sessionruntime.PrivateDiscoveryResponseFacts
	var available bool
	if len(snapshots) == 1 && snapshots[0].Available {
		facts, available = snapshots[0], true
	} else {
		facts, available = inv.PrivateDiscoveryOperationResponseFactsV2(cause)
	}
	return censusInventoryStoppedDiagnosticResponsesValue(stop, refusal, facts, available)
}

// The pure value seam carries the same closed facts as the actual operation
// error getter, without requiring a native execution to test CLI serialization.
func censusInventoryStoppedDiagnosticResponsesValue(stop censussourcedriver.Stop, refusal *sessionruntime.PrivateDiscoveryRefusal, facts sessionruntime.PrivateDiscoveryResponseFacts, available bool) (censusresult.Diagnostic, error) {
	out, err := censusInventoryStoppedDiagnosticValue(stop, refusal)
	if err != nil {
		return out, err
	}
	if available && out.Stage == censusresult.StageDiscovery {
		summary, e := sessionruntime.PrivateDiscoverySummarize(facts)
		if e != nil {
			return censusresult.Diagnostic{}, errors.New("invalid private discovery diagnostic")
		}
		out.PrivateDiscoveryResponses = &summary
	}
	return out, nil
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
