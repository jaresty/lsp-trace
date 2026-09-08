// Package manageddiagnostic owns bounded, privacy-safe observations for managed
// session generations. It is internal-only and performs no public registration.
package manageddiagnostic

import (
	"errors"
	"sync"
	"time"
)

type Status string

const (
	Observed    Status = "observed"
	Unavailable Status = "unavailable"
	Withheld    Status = "withheld"
)

type Fact[T any] struct {
	Status Status `json:"status"`
	Value  T      `json:"value,omitempty"`
}

type Phase string

const (
	PhaseSpawn              Phase = "spawn"
	PhaseInitializeWrite    Phase = "initialize-write"
	PhaseInitializeResponse Phase = "initialize-response"
	PhaseRequestDispatch    Phase = "request-dispatch"
	PhaseReadinessComplete  Phase = "readiness-complete"
	PhaseDocumentSupply     Phase = "document-supply"
	PhaseCapabilityCheck    Phase = "capability-check"
)

type Substep string

const SubstepInitializedNotification Substep = "initialized-notification"

type Terminal string

const (
	TerminalResponseReceived Terminal = "response-received"
	TerminalProtocolError    Terminal = "protocol-error"
	TerminalTransportClosed  Terminal = "transport-closed"
	TerminalCancelled        Terminal = "cancelled"
	TerminalDeadlineExceeded Terminal = "deadline-exceeded"
	TerminalProcessExited    Terminal = "process-exited"
	TerminalUnknown          Terminal = "unknown"
)

type IOState string

const (
	IOUnavailable IOState = "unavailable"
	IOAttempted   IOState = "attempted"
	IOComplete    IOState = "complete"
	IOFailed      IOState = "failed"
)

type IOFacts struct {
	State    IOState `json:"state"`
	Messages int     `json:"messages"`
	Bytes    int64   `json:"bytes"`
}
type Timing struct {
	Scope               string `json:"scope"`
	Start, End, Elapsed Fact[int64]
}
type Limits struct {
	RequestedDeadlineNS, EffectiveDeadlineNS   Fact[int64]
	RequestedMaxBytes, EffectiveMaxBytes       Fact[int64]
	RequestedMaxMessages, EffectiveMaxMessages Fact[int]
}
type RequestFacts struct {
	Method                    Fact[string]
	TargetID, CallerID        Fact[string]
	OwnerSequence, ProtocolID Fact[uint64]
}
type ProcessExit struct {
	Status                Status
	ExitCode              Fact[int]
	ObservedBeforeCleanup Fact[bool]
	CleanupInduced        Fact[bool]
}
type Stderr struct {
	Status            Status
	ByteCap           int64
	ObservedByteCount Fact[int64]
	Truncated         Fact[bool]
}
type Record struct {
	SessionID               string
	Generation, Sequence    uint64
	Phase                   Phase
	Substep                 Fact[Substep]
	Terminal                Terminal
	Reason                  Fact[string]
	Timing                  Timing
	Limits                  Limits
	Request                 RequestFacts
	Read, Write             IOFacts
	CallHierarchy           Fact[bool]
	DocumentSupplyCompleted Fact[bool]
	ProcessExit             ProcessExit
	Stderr                  Stderr
	NumericRPCCode          Fact[int]
	SafeSubcodes            []string
}

func (r Record) Clone() Record { r.SafeSubcodes = append([]string(nil), r.SafeSubcodes...); return r }

func validStatus(s Status) bool { return s == Observed || s == Unavailable || s == Withheld }

func validFact[T comparable](f Fact[T]) bool {
	var zero T
	// A wholly zero fact is the Go representation of an omitted partial fact.
	// Any explicit unavailable/withheld fact must likewise carry no value.
	if f.Status == "" {
		return f.Value == zero
	}
	if !validStatus(f.Status) {
		return false
	}
	return f.Status == Observed || f.Value == zero
}

func validIO(io IOFacts) bool {
	switch io.State {
	case IOUnavailable:
		return io.Messages == 0 && io.Bytes == 0
	case IOAttempted, IOComplete, IOFailed:
		return io.Messages >= 0 && io.Bytes >= 0
	default:
		return false
	}
}

type phaseRule struct {
	terminals       map[Terminal]bool
	substep         string
	matchedResponse bool
	io              string
}

var diagnosticPhaseRules = map[Phase]phaseRule{
	PhaseSpawn:              {terminals: terminalSet(TerminalProtocolError, TerminalTransportClosed, TerminalCancelled, TerminalDeadlineExceeded, TerminalProcessExited, TerminalUnknown), substep: "none", io: "optional"},
	PhaseInitializeWrite:    {terminals: terminalSet(TerminalProtocolError, TerminalTransportClosed, TerminalCancelled, TerminalDeadlineExceeded, TerminalProcessExited, TerminalUnknown), substep: "optional", io: "optional"},
	PhaseInitializeResponse: {terminals: terminalSet(TerminalResponseReceived, TerminalProtocolError, TerminalTransportClosed, TerminalCancelled, TerminalDeadlineExceeded, TerminalProcessExited, TerminalUnknown), substep: "none", io: "optional"},
	PhaseRequestDispatch:    {terminals: terminalSet(TerminalResponseReceived, TerminalProtocolError, TerminalTransportClosed, TerminalCancelled, TerminalDeadlineExceeded, TerminalProcessExited, TerminalUnknown), substep: "none", matchedResponse: true, io: "optional"},
	PhaseReadinessComplete:  {terminals: terminalSet(TerminalResponseReceived), substep: "required", matchedResponse: true, io: "round-trip"},
	PhaseDocumentSupply:     {terminals: terminalSet(TerminalResponseReceived, TerminalProtocolError), substep: "none", io: "write-only"},
	PhaseCapabilityCheck:    {terminals: terminalSet(TerminalResponseReceived, TerminalProtocolError), substep: "none", io: "none"},
}

func terminalSet(values ...Terminal) map[Terminal]bool {
	out := make(map[Terminal]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func completedMessages(io IOFacts, minimum int) bool {
	return io.State == IOComplete && io.Messages >= minimum
}

func validatePhaseMatrix(r Record) error {
	rule, ok := diagnosticPhaseRules[r.Phase]
	if !ok || !rule.terminals[r.Terminal] {
		return errors.New("terminal does not belong to phase")
	}
	if rule.substep == "required" && (r.Substep.Status != Observed || r.Substep.Value != SubstepInitializedNotification) {
		return errors.New("phase requires observed substep")
	}
	if rule.substep == "none" && r.Substep.Status == Observed {
		return errors.New("substep does not belong to phase")
	}
	if r.Terminal == TerminalProcessExited {
		if r.ProcessExit.Status != Observed {
			return errors.New("process exit must be independently observed")
		}
	} else if r.ProcessExit.Status == Observed {
		return errors.New("process exit does not belong to terminal")
	}
	matched := r.Terminal == TerminalResponseReceived || (rule.matchedResponse && r.Terminal == TerminalProtocolError)
	if matched && rule.io != "none" && rule.io != "write-only" && (!completedMessages(r.Write, 1) || !completedMessages(r.Read, 1)) {
		return errors.New("matched response requires completed request and response IO")
	}
	if (r.Limits.RequestedDeadlineNS.Status == Observed && r.Limits.EffectiveDeadlineNS.Status != Observed) ||
		(r.Limits.RequestedMaxBytes.Status == Observed && r.Limits.EffectiveMaxBytes.Status != Observed) ||
		(r.Limits.RequestedMaxMessages.Status == Observed && r.Limits.EffectiveMaxMessages.Status != Observed) {
		return errors.New("requested limit requires effective execution limit")
	}
	switch rule.io {
	case "none":
		if r.Read.State != IOUnavailable || r.Write.State != IOUnavailable {
			return errors.New("phase does not carry IO")
		}
	case "write-only":
		if r.Read.State != IOUnavailable || r.Write.Messages < 1 || (r.Write.State != IOComplete && r.Write.State != IOFailed) {
			return errors.New("document supply requires one completed or failed write")
		}
	}
	return nil
}

func Validate(r Record) error {
	if r.SessionID == "" || r.Generation == 0 || r.Sequence == 0 {
		return errors.New("diagnostic identity is required")
	}
	if _, ok := diagnosticPhaseRules[r.Phase]; !ok {
		return errors.New("unknown phase")
	}
	switch r.Terminal {
	case TerminalResponseReceived, TerminalProtocolError, TerminalTransportClosed, TerminalCancelled, TerminalDeadlineExceeded, TerminalProcessExited, TerminalUnknown:
	default:
		return errors.New("unknown terminal")
	}
	if !validFact(r.Reason) || !validFact(r.Substep) || !validFact(r.Timing.Start) || !validFact(r.Timing.End) || !validFact(r.Timing.Elapsed) ||
		!validFact(r.Limits.RequestedDeadlineNS) || !validFact(r.Limits.EffectiveDeadlineNS) || !validFact(r.Limits.RequestedMaxBytes) || !validFact(r.Limits.EffectiveMaxBytes) || !validFact(r.Limits.RequestedMaxMessages) || !validFact(r.Limits.EffectiveMaxMessages) ||
		!validFact(r.Request.Method) || !validFact(r.Request.TargetID) || !validFact(r.Request.CallerID) || !validFact(r.Request.OwnerSequence) || !validFact(r.Request.ProtocolID) ||
		!validFact(r.CallHierarchy) || !validFact(r.DocumentSupplyCompleted) || !validStatus(r.ProcessExit.Status) || !validFact(r.ProcessExit.ExitCode) || !validFact(r.ProcessExit.ObservedBeforeCleanup) || !validFact(r.ProcessExit.CleanupInduced) ||
		!validStatus(r.Stderr.Status) || !validFact(r.Stderr.ObservedByteCount) || !validFact(r.Stderr.Truncated) || !validFact(r.NumericRPCCode) {
		return errors.New("invalid fact status or hidden value")
	}
	if !validIO(r.Read) || !validIO(r.Write) {
		return errors.New("invalid IO accounting")
	}
	if r.Stderr.ByteCap < 0 || r.Stderr.ObservedByteCount.Value < 0 || r.Limits.RequestedDeadlineNS.Value < 0 || r.Limits.EffectiveDeadlineNS.Value < 0 || r.Limits.RequestedMaxBytes.Value < 0 || r.Limits.EffectiveMaxBytes.Value < 0 || r.Limits.RequestedMaxMessages.Value < 0 || r.Limits.EffectiveMaxMessages.Value < 0 {
		return errors.New("negative diagnostic value")
	}
	if r.Substep.Status == Observed && r.Substep.Value != SubstepInitializedNotification {
		return errors.New("unknown substep")
	}
	if err := validatePhaseMatrix(r); err != nil {
		return err
	}
	if r.ProcessExit.Status != Observed && (r.ProcessExit.ExitCode.Status == Observed || r.ProcessExit.ObservedBeforeCleanup.Status == Observed || r.ProcessExit.CleanupInduced.Status == Observed) {
		return errors.New("unavailable process exit carries observed detail")
	}
	if r.Stderr.Status != Observed && (r.Stderr.ByteCap != 0 || r.Stderr.ObservedByteCount.Status == Observed || r.Stderr.Truncated.Status == Observed) {
		return errors.New("unavailable stderr carries observed detail")
	}
	if r.Timing.Elapsed.Status == Observed {
		if r.Timing.Elapsed.Value < 0 {
			return errors.New("negative elapsed")
		}
		if r.Timing.Start.Status == Observed && r.Timing.End.Status == Observed && r.Timing.End.Value-r.Timing.Start.Value != r.Timing.Elapsed.Value {
			return errors.New("elapsed mismatch")
		}
	}
	if r.Terminal == TerminalDeadlineExceeded && r.Phase == PhaseInitializeResponse && r.Reason.Value == "initialize-success" {
		return errors.New("response and timeout conflict")
	}
	if r.Phase == PhaseCapabilityCheck && r.Reason.Status == Observed && r.Reason.Value == "unsupported-call-hierarchy" && r.CallHierarchy.Status != Observed {
		return errors.New("unsupported capability requires observed negotiation")
	}
	if r.ProcessExit.Status == Observed && r.ProcessExit.ObservedBeforeCleanup.Status != Observed {
		return errors.New("process exit chronology required")
	}
	return nil
}

type Bounds struct{ MaxRecords, MaxBytes int }
type QueryStatus string

const (
	QueryAvailable   QueryStatus = "available"
	QueryUnavailable QueryStatus = "unavailable"
	QueryEvicted     QueryStatus = "evicted"
)

type QueryResult struct {
	Status         QueryStatus
	Records        []Record
	EvictedRecords uint64
}
type generationKey struct {
	session    string
	generation uint64
}
type bucket struct {
	records []Record
	bytes   int
	evicted uint64
}
type Store struct {
	mu                  sync.RWMutex
	bounds              Bounds
	buckets             map[generationKey]bucket
	attempts            map[StartupAttemptID]StartupAttemptRecord
	attemptOrder        []StartupAttemptID
	attemptBytes        int
	evictedAttempts     map[StartupAttemptID]struct{}
	evictedAttemptOrder []StartupAttemptID
}

func NewStore(b Bounds) *Store {
	return &Store{bounds: b, buckets: make(map[generationKey]bucket), attempts: make(map[StartupAttemptID]StartupAttemptRecord), evictedAttempts: make(map[StartupAttemptID]struct{})}
}
func recordSize(r Record) int {
	return 256 + len(r.SessionID) + len(r.Timing.Scope) + len(r.Reason.Value) + len(r.Request.Method.Value) + len(r.Request.TargetID.Value) + len(r.Request.CallerID.Value) + len(r.SafeSubcodes)*32
}
func (s *Store) Record(r Record) bool {
	if s == nil || Validate(r) != nil || s.bounds.MaxRecords <= 0 || s.bounds.MaxBytes <= 0 {
		return false
	}
	r = r.Clone()
	size := recordSize(r)
	if size > s.bounds.MaxBytes {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := generationKey{r.SessionID, r.Generation}
	b := s.buckets[k]
	for len(b.records) >= s.bounds.MaxRecords || b.bytes+size > s.bounds.MaxBytes {
		b.bytes -= recordSize(b.records[0])
		b.records = b.records[1:]
		b.evicted++
	}
	b.records = append(b.records, r)
	b.bytes += size
	s.buckets[k] = b
	return true
}
func (s *Store) Query(session string, generation uint64) QueryResult {
	if s == nil {
		return QueryResult{Status: QueryUnavailable}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.buckets[generationKey{session, generation}]
	if !ok {
		return QueryResult{Status: QueryUnavailable}
	}
	out := make([]Record, len(b.records))
	for i := range b.records {
		out[i] = b.records[i].Clone()
	}
	status := QueryAvailable
	if len(out) == 0 && b.evicted > 0 {
		status = QueryEvicted
	}
	return QueryResult{Status: status, Records: out, EvictedRecords: b.evicted}
}

type RequestObservation struct {
	SessionID                                  string
	Generation, Sequence                       uint64
	Method                                     string
	ProtocolID                                 uint64
	TargetID, CallerID                         string
	RequestedDeadline, EffectiveDeadline       time.Duration
	RequestedMaxBytes, EffectiveMaxBytes       int64
	RequestedMaxMessages, EffectiveMaxMessages int
	Clock                                      func() int64
}
type RequestOutcome struct {
	Terminal                    Terminal
	Reason                      string
	ReadState, WriteState       IOState
	RequestBytes, ResponseBytes int64
	ResponseMessages            int
	NumericRPCCode              Fact[int]
}

func RecordRequest(o RequestObservation, call func() RequestOutcome) Record {
	clock := o.Clock
	if clock == nil {
		clock = func() int64 { return time.Now().UnixNano() }
	}
	start := clock()
	result := call()
	end := clock()
	fstr := func(v string) Fact[string] {
		if v == "" {
			return Fact[string]{Status: Unavailable}
		}
		return Fact[string]{Status: Observed, Value: v}
	}
	r := Record{SessionID: o.SessionID, Generation: o.Generation, Sequence: o.Sequence, Phase: PhaseRequestDispatch, Terminal: result.Terminal, Reason: fstr(result.Reason), Timing: Timing{Scope: "round-trip", Start: Fact[int64]{Observed, start}, End: Fact[int64]{Observed, end}, Elapsed: Fact[int64]{Observed, end - start}}, Request: RequestFacts{Method: fstr(o.Method), TargetID: fstr(o.TargetID), CallerID: fstr(o.CallerID), OwnerSequence: Fact[uint64]{Observed, o.Sequence}, ProtocolID: Fact[uint64]{Observed, o.ProtocolID}}, Read: IOFacts{State: result.ReadState, Messages: result.ResponseMessages, Bytes: result.ResponseBytes}, Write: IOFacts{State: result.WriteState, Messages: 1, Bytes: result.RequestBytes}, NumericRPCCode: result.NumericRPCCode, CallHierarchy: Fact[bool]{Status: Unavailable}, DocumentSupplyCompleted: Fact[bool]{Status: Unavailable}, ProcessExit: ProcessExit{Status: Unavailable}, Stderr: Stderr{Status: Withheld}}
	r.Limits = Limits{Fact[int64]{Observed, int64(o.RequestedDeadline)}, Fact[int64]{Observed, int64(o.EffectiveDeadline)}, Fact[int64]{Observed, o.RequestedMaxBytes}, Fact[int64]{Observed, o.EffectiveMaxBytes}, Fact[int]{Observed, o.RequestedMaxMessages}, Fact[int]{Observed, o.EffectiveMaxMessages}}
	return r
}

type CapabilityObservation struct {
	SessionID            string
	Generation, Sequence uint64
	Supported            bool
}

func RecordCapabilityCheck(o CapabilityObservation) Record {
	return Record{SessionID: o.SessionID, Generation: o.Generation, Sequence: o.Sequence, Phase: PhaseCapabilityCheck, Terminal: TerminalResponseReceived, Reason: Fact[string]{Observed, "capability-checked"}, CallHierarchy: Fact[bool]{Observed, o.Supported}, DocumentSupplyCompleted: Fact[bool]{Status: Unavailable}, ProcessExit: ProcessExit{Status: Unavailable}, Stderr: Stderr{Status: Withheld}, Request: RequestFacts{Method: Fact[string]{Status: Unavailable}, TargetID: Fact[string]{Status: Unavailable}, CallerID: Fact[string]{Status: Unavailable}, OwnerSequence: Fact[uint64]{Observed, o.Sequence}, ProtocolID: Fact[uint64]{Status: Unavailable}}}
}

type DocumentSupplyObservation struct {
	SessionID            string
	Generation, Sequence uint64
	Completed            bool
	Method               string
}

func RecordDocumentSupply(o DocumentSupplyObservation) Record {
	r := RecordCapabilityCheck(CapabilityObservation{o.SessionID, o.Generation, o.Sequence, false})
	r.Phase = PhaseDocumentSupply
	r.CallHierarchy = Fact[bool]{Status: Unavailable}
	r.DocumentSupplyCompleted = Fact[bool]{Observed, o.Completed}
	r.Request.Method = Fact[string]{Observed, o.Method}
	r.Reason = Fact[string]{Observed, "document-supply"}
	return r
}
