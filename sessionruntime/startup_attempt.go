package sessionruntime

import (
	"strconv"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/session"
)

func (m *Manager) beginStartupAttempt() (manageddiagnostic.StartupAttemptID, uint64, int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startupAttemptSeq++
	sequence := m.startupAttemptSeq
	id := m.startupAttemptIDSource(sequence)
	if id == "" {
		id = manageddiagnostic.StartupAttemptID("startup-invalid-source-" + strconv.FormatUint(sequence, 10))
	}
	return id, sequence, m.now().UnixNano()
}

func staticStartupReason(result StartResult) string {
	if result.Failure == session.RequestTimeout {
		return "request-timeout"
	}
	if result.Failure == session.RequestCancelled {
		return "request-cancelled"
	}
	if result.Failure == session.ResourceExhausted {
		return "resource-exhausted"
	}
	if result.Failure == session.LifecycleConflict {
		return "lifecycle-conflict"
	}
	if result.Failure == session.ProcessContainmentUnavailable {
		return "containment-unavailable"
	}
	if result.Failure == "" && result.Generation > 0 {
		return "startup-admitted"
	}
	switch result.Start.Reason {
	case "command factory unavailable":
		return "command-factory-unavailable"
	case "command construction failed":
		return "command-construction-failed"
	case "stdin pipe failed":
		return "stdin-pipe-failed"
	case "stdout pipe failed":
		return "stdout-pipe-failed"
	case "process start failed":
		return "process-start-failed"
	case "local Darwin supervision unavailable":
		return "local-supervision-unavailable"
	case "containment unavailable":
		return "containment-unavailable"
	default:
		return "startup-failed"
	}
}

func (m *Manager) finishStartupAttempt(id manageddiagnostic.StartupAttemptID, sequence uint64, started int64, result StartResult) {
	if m.diagnostics == nil {
		return
	}
	ended := m.now().UnixNano()
	if ended < started {
		ended = started
	}
	reason := staticStartupReason(result)
	r := manageddiagnostic.StartupAttemptRecord{
		SchemaVersion: manageddiagnostic.StartupAttemptSchemaVersion,
		AttemptID:     id, Sequence: sequence, Outcome: manageddiagnostic.StartupFailed,
		Reason:       manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: reason},
		Timing:       manageddiagnostic.Timing{Scope: "startup-attempt", Start: manageddiagnostic.Fact[int64]{Status: manageddiagnostic.Observed, Value: started}, End: manageddiagnostic.Fact[int64]{Status: manageddiagnostic.Observed, Value: ended}, Elapsed: manageddiagnostic.Fact[int64]{Status: manageddiagnostic.Observed, Value: ended - started}},
		Stderr:       manageddiagnostic.Stderr{Status: manageddiagnostic.Withheld},
		ProcessExit:  manageddiagnostic.ProcessExit{Status: manageddiagnostic.Unavailable},
		SafeSubcodes: []string{reason},
	}
	if result.Failure == "" && result.SessionID != "" && result.Generation > 0 {
		r.Outcome = manageddiagnostic.StartupAdmitted
		r.Admission = &manageddiagnostic.StartupAdmission{SessionID: result.SessionID, Generation: result.Generation}
	}
	m.diagnostics.RecordStartupAttempt(r)
}
