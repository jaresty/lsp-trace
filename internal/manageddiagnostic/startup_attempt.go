package manageddiagnostic

import (
	"errors"
	"strconv"
)

// StartupAttemptID identifies one host-observed Start invocation. It is not a
// session identity, generation, authentication claim, or cross-process ID.
type StartupAttemptID string

const StartupAttemptSchemaVersion = "managed-startup-attempt/v1"

type StartupOutcome string

const (
	StartupFailed   StartupOutcome = "failed"
	StartupAdmitted StartupOutcome = "admitted"
)

type StartupAdmission struct {
	SessionID  string `json:"session_id"`
	Generation uint64 `json:"generation"`
}

type StartupAttemptRecord struct {
	SchemaVersion string            `json:"schema_version"`
	AttemptID     StartupAttemptID  `json:"attempt_id"`
	Sequence      uint64            `json:"sequence"`
	Outcome       StartupOutcome    `json:"outcome"`
	Reason        Fact[string]      `json:"reason"`
	Timing        Timing            `json:"timing"`
	Limits        Limits            `json:"limits"`
	Stderr        Stderr            `json:"stderr"`
	ProcessExit   ProcessExit       `json:"process_exit"`
	SafeSubcodes  []string          `json:"safe_subcodes,omitempty"`
	Admission     *StartupAdmission `json:"admission,omitempty"`
}

func (r StartupAttemptRecord) Clone() StartupAttemptRecord {
	r.SafeSubcodes = append([]string(nil), r.SafeSubcodes...)
	if r.Admission != nil {
		a := *r.Admission
		r.Admission = &a
	}
	return r
}

func ValidateStartupAttempt(r StartupAttemptRecord) error {
	if r.SchemaVersion != StartupAttemptSchemaVersion {
		return errors.New("unknown startup attempt schema version")
	}
	if r.AttemptID == "" || r.Sequence == 0 {
		return errors.New("startup attempt identity is required")
	}
	if _, err := strconv.ParseUint(string(r.AttemptID), 10, 64); err == nil {
		return errors.New("startup attempt ID must not encode a generation")
	}
	if r.Outcome != StartupFailed && r.Outcome != StartupAdmitted {
		return errors.New("unknown startup outcome")
	}
	if !validFact(r.Reason) || !validStatus(r.Stderr.Status) || !validFact(r.Stderr.ObservedByteCount) || !validFact(r.Stderr.Truncated) || !validStatus(r.ProcessExit.Status) || !validFact(r.ProcessExit.ExitCode) || !validFact(r.ProcessExit.ObservedBeforeCleanup) || !validFact(r.ProcessExit.CleanupInduced) {
		return errors.New("invalid startup fact status or hidden value")
	}
	if r.Stderr.ByteCap < 0 || r.Stderr.ObservedByteCount.Value < 0 {
		return errors.New("negative startup stderr accounting")
	}
	if r.Stderr.Status != Observed && (r.Stderr.ByteCap != 0 || r.Stderr.ObservedByteCount.Status == Observed || r.Stderr.Truncated.Status == Observed) {
		return errors.New("unavailable startup stderr carries observed detail")
	}
	if r.ProcessExit.Status != Observed && (r.ProcessExit.ExitCode.Status == Observed || r.ProcessExit.ObservedBeforeCleanup.Status == Observed || r.ProcessExit.CleanupInduced.Status == Observed) {
		return errors.New("unavailable startup process exit carries observed detail")
	}
	if r.Timing.Elapsed.Status == Observed {
		if r.Timing.Elapsed.Value < 0 {
			return errors.New("negative startup elapsed")
		}
		if r.Timing.Start.Status == Observed && r.Timing.End.Status == Observed && r.Timing.End.Value-r.Timing.Start.Value != r.Timing.Elapsed.Value {
			return errors.New("startup elapsed mismatch")
		}
	}
	if r.Outcome == StartupFailed && r.Admission != nil {
		return errors.New("failed startup cannot carry admission")
	}
	if r.Outcome == StartupAdmitted && (r.Admission == nil || r.Admission.SessionID == "" || r.Admission.Generation == 0) {
		return errors.New("admitted startup requires exact generation")
	}
	if r.Outcome == StartupAdmitted && r.Reason.Status == Observed && r.Reason.Value != "startup-admitted" {
		return errors.New("admitted startup carries failure reason")
	}
	if r.Outcome == StartupFailed && r.ProcessExit.Status == Observed && r.ProcessExit.CleanupInduced.Status == Observed && r.ProcessExit.CleanupInduced.Value {
		return errors.New("failed startup cannot claim cleanup-induced exit")
	}
	if r.ProcessExit.Status == Observed && r.ProcessExit.ObservedBeforeCleanup.Status != Observed {
		return errors.New("startup process exit chronology required")
	}
	return nil
}

type AttemptQueryStatus string

const (
	AttemptAvailable   AttemptQueryStatus = "available"
	AttemptUnavailable AttemptQueryStatus = "unavailable"
	AttemptEvicted     AttemptQueryStatus = "evicted"
)

type StartupAttemptQuery struct {
	Status AttemptQueryStatus
	Record *StartupAttemptRecord
}

func startupAttemptSize(r StartupAttemptRecord) int {
	size := 256 + len(r.SchemaVersion) + len(r.AttemptID) + len(r.Reason.Value) + len(r.Timing.Scope) + len(r.SafeSubcodes)*32
	if r.Admission != nil {
		size += len(r.Admission.SessionID) + 8
	}
	return size
}

func (s *Store) RecordStartupAttempt(r StartupAttemptRecord) bool {
	if s == nil || ValidateStartupAttempt(r) != nil || s.bounds.MaxRecords <= 0 || s.bounds.MaxBytes <= 0 {
		return false
	}
	r = r.Clone()
	size := startupAttemptSize(r)
	if size > s.bounds.MaxBytes {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.attempts[r.AttemptID]; exists {
		return false
	}
	for len(s.attemptOrder) >= s.bounds.MaxRecords || s.attemptBytes+size > s.bounds.MaxBytes {
		old := s.attemptOrder[0]
		s.attemptOrder = s.attemptOrder[1:]
		s.attemptBytes -= startupAttemptSize(s.attempts[old])
		delete(s.attempts, old)
		s.evictedAttempts[old] = struct{}{}
		s.evictedAttemptOrder = append(s.evictedAttemptOrder, old)
		if len(s.evictedAttemptOrder) > s.bounds.MaxRecords {
			forgotten := s.evictedAttemptOrder[0]
			s.evictedAttemptOrder = s.evictedAttemptOrder[1:]
			delete(s.evictedAttempts, forgotten)
		}
	}
	s.attempts[r.AttemptID] = r
	s.attemptOrder = append(s.attemptOrder, r.AttemptID)
	s.attemptBytes += size
	return true
}

func (s *Store) StartupAttempt(id StartupAttemptID) StartupAttemptQuery {
	if s == nil || id == "" {
		return StartupAttemptQuery{Status: AttemptUnavailable}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.attempts[id]; ok {
		c := r.Clone()
		return StartupAttemptQuery{Status: AttemptAvailable, Record: &c}
	}
	if _, ok := s.evictedAttempts[id]; ok {
		return StartupAttemptQuery{Status: AttemptEvicted}
	}
	return StartupAttemptQuery{Status: AttemptUnavailable}
}
