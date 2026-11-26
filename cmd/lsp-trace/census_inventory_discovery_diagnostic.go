package main

import (
	"io"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/censussourcedriver"
	"lsp-trace/sessionruntime"
)

// Pure diagnostic forwarding: no cursor, authority, terminal, or retry effect.
func censusInventoryDiscoveryDiagnostic(d sessionruntime.PrivateDiscoveryRefusal) (censusresult.Diagnostic, error) {
	out, err := censusresult.NewDiagnostic(censusresult.StageDiscovery, nil)
	if err != nil || !d.Valid() {
		return censusresult.Diagnostic{}, errCensusInventoryPrivate
	}
	out.Retry = false
	out.PrivateDiscoveryRefusal = &d
	return out, nil
}
func writeCensusInventoryDiscoveryFailure(stderr io.Writer, machine bool, cause error, stop censussourcedriver.Stop, snapshots ...sessionruntime.PrivateDiscoveryResponseFacts) int {
	out, err := censusInventoryStoppedDiagnostic(stop, cause, snapshots...)
	if err != nil || out.Code == censusresult.CodeInvalidConfig {
		return writeCensusFailure(stderr, censusStageConfig, machine, errCensusInventoryPrivate)
	}
	if machine {
		raw, err := censusresult.MarshalDiagnostic(out)
		if err == nil {
			_, _ = stderr.Write(raw)
		}
	} else {
		switch out.Code {
		case censusresult.CodeDiscoveryRetained:
			_, _ = io.WriteString(stderr, "private discovery retained; no retry granted\n")
		case censusresult.CodeDiscoveryIndeterminate:
			_, _ = io.WriteString(stderr, "private discovery indeterminate; no retry granted\n")
		default:
			_, _ = io.WriteString(stderr, "private discovery enclosure refused\n")
		}
	}
	return 1
}
