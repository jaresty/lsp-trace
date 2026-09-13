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
			return legacyCLI{Operation: name, Replacement: "census", Status: "AVAILABLE"}, true
		}
		return legacyCLI{Operation: name, Replacement: "trace", Status: "AVAILABLE"}, true
	case "incoming":
		return legacyCLI{Operation: name, Replacement: "trace", Status: "CONDITIONAL"}, true
	default:
		return legacyCLI{}, false
	}
}

type machineFlagState uint8

const (
	machineFlagOK machineFlagState = iota
	machineFlagDuplicate
	machineFlagInvalid
)

func extractMachineMode(args []string) (clean []string, machine bool, state machineFlagState) {
	if len(args) == 0 {
		return args, false, machineFlagOK
	}
	// Machine mode has one global grammar immediately after a legacy command:
	// --machine and --machine=true enable it; --machine=false disables it.
	// Scanning stops at the first other token, after which every byte belongs to
	// the command FlagSet (notably the value in --server-arg --machine).
	i, seen := 1, false
	for i < len(args) {
		arg := args[i]
		if arg != "--machine" && !strings.HasPrefix(arg, "--machine=") {
			break
		}
		value := true
		switch arg {
		case "--machine", "--machine=true":
		case "--machine=false":
			value = false
		default:
			return append([]string{args[0]}, args[i+1:]...), machine, machineFlagInvalid
		}
		if seen {
			return append([]string{args[0]}, args[i+1:]...), machine || value, machineFlagDuplicate
		}
		seen, machine = true, value
		i++
	}
	clean = append([]string{args[0]}, args[i:]...)
	return clean, machine, machineFlagOK
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
