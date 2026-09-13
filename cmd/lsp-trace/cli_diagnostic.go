package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	cliDiagnosticVersion = "lsp-trace.cli-diagnostic.v1"
	cliRemovalRelease    = "UNSCHEDULED"
)

const (
	cliCodeLegacyOperation = "CLI_LEGACY_OPERATION"
	cliCodeInvocationError = "CLI_INVOCATION_ERROR"
)

type cliDiagnostic struct {
	SchemaVersion  string `json:"schema_version"`
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Operation      string `json:"operation"`
	Replacement    string `json:"replacement"`
	Status         string `json:"replacement_status"`
	RemovalRelease string `json:"removal_release"`
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
			return legacyCLI{Operation: name, Replacement: "census", Status: "FUTURE/PROPOSED_UNAVAILABLE"}, true
		}
		return legacyCLI{Operation: name, Replacement: "trace", Status: "AVAILABLE"}, true
	case "incoming":
		return legacyCLI{Operation: name, Replacement: "trace", Status: "CONDITIONAL"}, true
	default:
		return legacyCLI{}, false
	}
}

func extractMachineMode(args []string) (clean []string, machine, duplicate bool) {
	if len(args) == 0 {
		return args, false, false
	}
	// --machine is a diagnostic-mode flag only in the unambiguous global
	// position immediately after a legacy command. Once command arguments
	// begin, every token belongs to the command FlagSet (for example,
	// --server-arg --machine) and is preserved byte-for-byte.
	i := 1
	for i < len(args) && args[i] == "--machine" {
		if machine {
			duplicate = true
		}
		machine = true
		i++
	}
	clean = append([]string{args[0]}, args[i:]...)
	return clean, machine, duplicate
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
		writeCLIDiagnostic(w, cliDiagnostic{SchemaVersion: cliDiagnosticVersion, Code: cliCodeLegacyOperation, Severity: "warning", Operation: legacy.Operation, Replacement: legacy.Replacement, Status: legacy.Status, RemovalRelease: cliRemovalRelease})
		return
	}
	qualifier := ""
	if legacy.Status != "AVAILABLE" {
		qualifier = "; equivalence is conditional, so retain legacy dispatch when bounds, identity, accounting, bytes, or custody differ"
	}
	fmt.Fprintf(w, "warning: %s is deprecated; migrate to %s%s\n", legacy.Operation, legacy.Replacement, qualifier)
}

func writeMachineInvocationError(w io.Writer, legacy legacyCLI) {
	writeCLIDiagnostic(w, cliDiagnostic{SchemaVersion: cliDiagnosticVersion, Code: cliCodeInvocationError, Severity: "error", Operation: legacy.Operation, Replacement: legacy.Replacement, Status: legacy.Status, RemovalRelease: cliRemovalRelease})
}
