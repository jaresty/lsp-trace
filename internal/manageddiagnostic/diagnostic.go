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
func Validate(r Record) error {
	if r.SessionID == "" || r.Generation == 0 || r.Sequence == 0 {
		return errors.New("diagnostic identity is required")
	}
	switch r.Phase {
	case PhaseSpawn, PhaseInitializeWrite, PhaseInitializeResponse, PhaseDocumentSupply, PhaseCapabilityCheck:
	default:
		return errors.New("unknown phase")
	}
	switch r.Terminal {
	case TerminalResponseReceived, TerminalProtocolError, TerminalTransportClosed, TerminalCancelled, TerminalDeadlineExceeded, TerminalProcessExited, TerminalUnknown:
	default:
		return errors.New("unknown terminal")
	}
	if !validStatus(r.Reason.Status) || !validStatus(r.Request.Method.Status) || !validStatus(r.CallHierarchy.Status) || !validStatus(r.DocumentSupplyCompleted.Status) || !validStatus(r.ProcessExit.Status) || !validStatus(r.Stderr.Status) {
		return errors.New("optional fact status required")
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
	if r.Terminal == TerminalProcessExited && r.ProcessExit.Status != Observed {
		return errors.New("process exit must be independently observed")
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
	mu      sync.RWMutex
	bounds  Bounds
	buckets map[generationKey]bucket
}

func NewStore(b Bounds) *Store { return &Store{bounds: b, buckets: make(map[generationKey]bucket)} }
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
	r := Record{SessionID: o.SessionID, Generation: o.Generation, Sequence: o.Sequence, Phase: PhaseInitializeResponse, Terminal: result.Terminal, Reason: fstr(result.Reason), Timing: Timing{Scope: "round-trip", Start: Fact[int64]{Observed, start}, End: Fact[int64]{Observed, end}, Elapsed: Fact[int64]{Observed, end - start}}, Request: RequestFacts{Method: fstr(o.Method), TargetID: fstr(o.TargetID), CallerID: fstr(o.CallerID), OwnerSequence: Fact[uint64]{Observed, o.Sequence}, ProtocolID: Fact[uint64]{Observed, o.ProtocolID}}, Read: IOFacts{State: result.ReadState, Bytes: result.ResponseBytes}, Write: IOFacts{State: result.WriteState, Messages: 1, Bytes: result.RequestBytes}, NumericRPCCode: result.NumericRPCCode, CallHierarchy: Fact[bool]{Status: Unavailable}, DocumentSupplyCompleted: Fact[bool]{Status: Unavailable}, ProcessExit: ProcessExit{Status: Unavailable}, Stderr: Stderr{Status: Withheld}}
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
