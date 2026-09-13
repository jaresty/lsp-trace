package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const cliDiagnosticVersion = "lsp-trace.cli-diagnostic.v1"

const (
	cliCodeLegacyOperation = "CLI_LEGACY_OPERATION"
	cliCodeInvocationError = "CLI_INVOCATION_ERROR"
)

type cliDiagnostic struct {
	SchemaVersion string `json:"schema_version"`
	Code          string `json:"code"`
	Severity      string `json:"severity"`
	Operation     string `json:"operation"`
	Replacement   string `json:"replacement"`
	Status        string `json:"replacement_status"`
}

type legacyCLI struct {
	Operation   string
	Replacement string
	Status      string
}

func legacyOperation(name string, args []string) (legacyCLI, bool) {
	switch name {
	case "slice":
		fileCensus, exactSymbol := false, false
		for _, arg := range args {
			fileCensus = fileCensus || arg == "--from-file" || strings.HasPrefix(arg, "--from-file=")
			exactSymbol = exactSymbol || arg == "--symbol" || strings.HasPrefix(arg, "--symbol=")
		}
		if fileCensus && !exactSymbol {
			return legacyCLI{Operation: name, Replacement: "census", Status: "FUTURE/PROPOSED"}, true
		}
		return legacyCLI{Operation: name, Replacement: "trace", Status: "AVAILABLE"}, true
	case "incoming":
		return legacyCLI{Operation: name, Replacement: "trace", Status: "CONDITIONAL"}, true
	default:
		return legacyCLI{}, false
	}
}

func extractMachineMode(args []string) ([]string, bool, error) {
	if len(args) == 0 {
		return args, false, nil
	}
	out := make([]string, 0, len(args))
	out = append(out, args[0])
	machine := false
	for _, arg := range args[1:] {
		if arg != "--machine" {
			out = append(out, arg)
			continue
		}
		if machine {
			return nil, false, fmt.Errorf("duplicate --machine")
		}
		machine = true
	}
	return out, machine, nil
}

func writeCLIDiagnostic(w io.Writer, d cliDiagnostic) {
	// The closed value-only schema intentionally cannot carry paths, source,
	// environment values, server stderr, or arbitrary error strings.
	data, err := json.Marshal(d)
	if err != nil {
		return
	}
	_, _ = w.Write(append(data, '\n'))
}

func writeLegacyWarning(w io.Writer, legacy legacyCLI, machine bool) {
	if machine {
		writeCLIDiagnostic(w, cliDiagnostic{SchemaVersion: cliDiagnosticVersion, Code: cliCodeLegacyOperation, Severity: "warning", Operation: legacy.Operation, Replacement: legacy.Replacement, Status: legacy.Status})
		return
	}
	qualifier := ""
	if legacy.Status != "AVAILABLE" {
		qualifier = "; equivalence is conditional, so retain legacy dispatch when bounds, identity, accounting, bytes, or custody differ"
	}
	fmt.Fprintf(w, "warning: %s is deprecated; migrate to %s%s\n", legacy.Operation, legacy.Replacement, qualifier)
}

func writeMachineInvocationError(w io.Writer, legacy legacyCLI) {
	writeCLIDiagnostic(w, cliDiagnostic{SchemaVersion: cliDiagnosticVersion, Code: cliCodeInvocationError, Severity: "error", Operation: legacy.Operation, Replacement: legacy.Replacement, Status: legacy.Status})
}
