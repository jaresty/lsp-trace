//go:build darwin || linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"lsp-trace/internal/inventorysuccessor/sourceoperator"
)

// All strings are assigned here from closed enums or literals, never raw inputs.
// Field order is deliberately fixed; this record grants no semantic credit.
type privateSourceFailureDiagnosticV1 struct {
	Schema                  string `json:"schema"`
	Stage                   string `json:"stage"`
	OperatorClass           string `json:"operator_class"`
	ExecutionError          bool   `json:"execution_error"`
	ShutdownError           bool   `json:"shutdown_error"`
	CloseError              bool   `json:"close_error"`
	TotalContextError       bool   `json:"total_context_error"`
	CleanupContextError     bool   `json:"cleanup_context_error"`
	TerminalDeadlineExpired bool   `json:"terminal_deadline_expired"`
	TailInvalid             bool   `json:"tail_invalid"`
	ExecutionClass          string `json:"execution_class"`
	ShutdownClass           string `json:"shutdown_class"`
	CloseClass              string `json:"close_class"`
	TotalContextClass       string `json:"total_context_class"`
	CleanupContextClass     string `json:"cleanup_context_class"`
	PublicationCloseError   bool   `json:"publication_close_error"`
	PendingFinalizeError    bool   `json:"pending_finalize_error"`
	OperatorTerminalError   bool   `json:"operator_terminal_error"`
}

func privateSourceDiagnosticV1(out sourceoperator.Outcome, executionErr, shutdownErr, closeErr, totalErr, cleanupErr error, contractFailed, tailInvalid bool, now, deadline time.Time) privateSourceFailureDiagnosticV1 {
	f := out.FailureDiagnosticV1()
	stage := f.Stage.String() // closed enum sanitization at the writer boundary
	if contractFailed {
		stage = "contract"
	}
	class := func(e error) string { return sourceoperator.ClassifyFailureV1(e).String() }
	return privateSourceFailureDiagnosticV1{
		Schema: "SOURCE_FAILURE_DIAGNOSTIC_V1", Stage: stage, OperatorClass: f.Class.String(),
		ExecutionError: executionErr != nil, ShutdownError: shutdownErr != nil, CloseError: closeErr != nil,
		TotalContextError: totalErr != nil, CleanupContextError: cleanupErr != nil, TerminalDeadlineExpired: !now.Before(deadline), TailInvalid: tailInvalid,
		ExecutionClass: class(executionErr), ShutdownClass: class(shutdownErr), CloseClass: class(closeErr), TotalContextClass: class(totalErr), CleanupContextClass: class(cleanupErr),
		PublicationCloseError: f.PublicationClose, PendingFinalizeError: f.PendingFinalize, OperatorTerminalError: f.Terminal,
	}
}
func (d privateSourceFailureDiagnosticV1) failed() bool {
	return d.ExecutionError || d.TailInvalid || d.ShutdownError || d.CloseError || d.TotalContextError || d.CleanupContextError || d.TerminalDeadlineExpired
}

// The only output addition is a single fixed-schema record on a failed terminal.
func finishPrivateSourceV1(out sourceoperator.Outcome, d privateSourceFailureDiagnosticV1, stdout, stderr io.Writer) int {
	failed := d.failed()
	if failed {
		out.Disposition = "FAILED_RETAINED"
	}
	class := ""
	if json.NewEncoder(stdout).Encode(out) != nil {
		class = "OUTPUT_REFUSED"
	} else if failed {
		class = "EXECUTION_REFUSED_RETAINED"
	}
	if class != "" {
		fmt.Fprintln(stderr, "private source application:", class)
	}
	if failed {
		d.Schema = "SOURCE_FAILURE_DIAGNOSTIC_V1"
		switch d.Stage {
		case "contract", "guard", "publication_open", "capture_reserve", "capture_execute", "preparation_reserve", "preparation_execute", "operator", "publication", "unknown":
		default:
			d.Stage = "unknown"
		}
		safeClass := func(c string) string {
			switch c {
			case "none", "canceled", "deadline_exceeded", "refused", "native_aggregate", "other":
				return c
			default:
				return "other"
			}
		}
		d.OperatorClass = safeClass(d.OperatorClass)
		d.ExecutionClass = safeClass(d.ExecutionClass)
		d.ShutdownClass = safeClass(d.ShutdownClass)
		d.CloseClass = safeClass(d.CloseClass)
		d.TotalContextClass = safeClass(d.TotalContextClass)
		d.CleanupContextClass = safeClass(d.CleanupContextClass)
		_ = json.NewEncoder(stderr).Encode(d)
	}
	if class != "" {
		return 1
	}
	return 0
}
