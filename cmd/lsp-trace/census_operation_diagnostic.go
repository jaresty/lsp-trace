package main

import (
	"errors"
	"io"
	"os"

	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/publication"
)

type censusOperationDiagnosticWriter struct{ io.Writer }

// Only checks that establish a closed reason construct this carrier. Raw causes
// remain internal and are never serialized into an operation diagnostic.
type censusStoreConfigError struct {
	reason censusresult.OperationReason
	cause  error
}

func (e *censusStoreConfigError) Error() string { return string(e.reason) }
func (e *censusStoreConfigError) Unwrap() error { return e.cause }
func censusStoreError(reason censusresult.OperationReason, cause error) error {
	return &censusStoreConfigError{reason: reason, cause: cause}
}
func censusStoreIOError(e error) error {
	if e == nil {
		return nil
	}
	var typed *censusStoreConfigError
	if errors.As(e, &typed) {
		return e
	}
	reason := censusresult.StoreIO
	if errors.Is(e, os.ErrPermission) || errors.Is(e, publication.ErrRootPrivacy) || errors.Is(e, publication.ErrRootOwner) {
		reason = censusresult.StorePermissions
	}
	return censusStoreError(reason, e)
}
func censusConfigReason(e error) censusresult.OperationReason {
	var typed *censusStoreConfigError
	if errors.As(e, &typed) && censusresult.IsOperationConfigReason(typed.reason) {
		return typed.reason
	}
	return censusresult.ConfigUnclassified
}
func writeCensusOperationFailure(w io.Writer, operation string, reason censusresult.OperationReason, trustedRun string) int {
	d := censusresult.OperationDiagnostic{SchemaVersion: censusresult.OperationDiagnosticSchemaVersion, Operation: operation, Reason: reason, Action: "STOP"}
	if reason == censusresult.UnsupportedStoredVersion {
		d.SchemaVersion = censusresult.StoredVersionDiagnosticSchemaVersion
	}
	if censusresult.IsOperationConfigReason(reason) {
		d.Action = "CORRECT_CONFIG"
	}
	if reason == censusresult.OperationUnconfirmed && trustedRun != "" {
		d.Action = "OBSERVE_CURRENT_TASK"
		d.Recovery = &censusresult.OperationRecovery{RunID: trustedRun, StoreContext: "SAME_SELECTION"}
	}
	raw, e := censusresult.MarshalOperationDiagnostic(d)
	if e != nil {
		// Fail closed even if a future caller constructs an inconsistent combination.
		raw, e = censusresult.MarshalOperationDiagnostic(censusresult.OperationDiagnostic{SchemaVersion: censusresult.OperationDiagnosticSchemaVersion, Operation: "unknown", Reason: censusresult.InvalidOperation, Action: "STOP"})
		if e != nil {
			return 1
		}
	}
	_, _ = w.Write(raw)
	return 1
}
