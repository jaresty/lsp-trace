package stage2scenario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"lsp-trace/internal/session"
)

const (
	ScenarioVersion  = "stage2-scenario-v1"
	LedgerVersion    = "stage2-ledger-v1"
	maxIdentityBytes = 256
)

type Scenario struct {
	Version   string `json:"version"`
	Steps     []Step `json:"steps"`
	canonical []byte
}

type Step struct {
	Op         string              `json:"op"`
	Outcome    string              `json:"outcome,omitempty"`
	Request    string              `json:"request,omitempty"`
	LSPRequest string              `json:"lsp_request,omitempty"`
	Child      string              `json:"child,omitempty"`
	Ordinal    int                 `json:"ordinal,omitempty"`
	Generation int                 `json:"generation,omitempty"`
	Session    string              `json:"session,omitempty"`
	State      string              `json:"state,omitempty"`
	Operation  string              `json:"operation,omitempty"`
	Caller     string              `json:"caller,omitempty"`
	IntentID   string              `json:"intent_id,omitempty"`
	IntentRef  string              `json:"intent_ref,omitempty"`
	Bind       string              `json:"bind,omitempty"`
	ChildRisk  bool                `json:"child_risk,omitempty"`
	HasWork    bool                `json:"has_work,omitempty"`
	Death      bool                `json:"death,omitempty"`
	Reaped     bool                `json:"reaped,omitempty"`
	Ready      bool                `json:"ready,omitempty"`
	Success    bool                `json:"success,omitempty"`
	Expect     *ManagerExpectation `json:"expect,omitempty"`
}

type ManagerExpectation struct {
	Kind            string `json:"kind,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	Victim          string `json:"victim,omitempty"`
	Reservation     string `json:"reservation,omitempty"`
	State           string `json:"state,omitempty"`
	PriorState      string `json:"prior_state,omitempty"`
	Generation      uint64 `json:"generation,omitempty"`
	IntentID        string `json:"intent_id,omitempty"`
	IntentRef       string `json:"intent_ref,omitempty"`
	IntentKind      string `json:"intent_kind,omitempty"`
	Outcome         string `json:"outcome,omitempty"`
	OperationStatus string `json:"operation_status,omitempty"`
	Failure         string `json:"failure,omitempty"`
	Joined          bool   `json:"joined,omitempty"`
	Replayed        bool   `json:"replayed,omitempty"`
	Noop            bool   `json:"noop,omitempty"`
	Detached        *bool  `json:"detached,omitempty"`
}

var opFields = map[string]map[string]bool{
	"startup":             fields("op", "session", "generation", "outcome"),
	"initialize":          fields("op", "session", "generation", "outcome"),
	"request":             fields("op", "session", "generation", "request"),
	"child":               fields("op", "session", "generation", "request", "lsp_request", "child", "ordinal"),
	"respond":             fields("op", "session", "generation", "lsp_request", "child"),
	"timeout":             fields("op", "session", "generation", "request"),
	"cancel":              fields("op", "session", "generation", "request"),
	"cancel_write_failed": fields("op", "session", "generation", "request", "lsp_request", "child"),
	"late_response":       fields("op", "session", "generation", "lsp_request", "child"),
	"crash":               fields("op", "session", "generation", "outcome"),
	"poison":              fields("op", "session", "generation", "outcome"),
	"lifecycle_register":  fields("op", "session", "generation", "state", "reaped", "expect"),
	"lifecycle":           fields("op", "session", "generation", "operation", "caller", "bind", "expect", "child_risk", "has_work"),
	"lifecycle_complete":  fields("op", "session", "intent_id", "intent_ref", "death", "reaped", "ready", "expect"),
	"lifecycle_detach":    fields("op", "session", "intent_id", "intent_ref", "caller", "outcome", "expect"),
	"admit":               fields("op", "session", "expect"),
	"evict_complete":      fields("op", "intent_id", "intent_ref", "success", "expect"),
}

func fields(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

func Parse(data []byte) (Scenario, error) {
	var raw struct {
		Version string            `json:"version"`
		Steps   []json.RawMessage `json:"steps"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return Scenario{}, fmt.Errorf("invalid scenario: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Scenario{}, fmt.Errorf("invalid scenario: trailing JSON value")
	}
	structuralErr := ValidateScenarioStructure(data)
	if raw.Version != ScenarioVersion {
		return Scenario{}, fmt.Errorf("unsupported version %q", raw.Version)
	}
	if raw.Steps == nil {
		return Scenario{}, fmt.Errorf("steps is required")
	}
	// Structural validation is authoritative and always precedes operation
	// vocabulary, field, bounds, uniqueness, and prerequisite semantics.
	if structuralErr != nil {
		return Scenario{}, structuralErr
	}
	s := Scenario{Version: raw.Version, Steps: make([]Step, 0, len(raw.Steps))}
	seenRequests := map[string]bool{}
	seenLSP := map[string]bool{}
	nextOrdinal := map[string]int{}
	declaredChildren := map[string]bool{}
	declaredRequests := map[string]struct {
		session    string
		generation int
	}{}
	for i, r := range raw.Steps {
		var members map[string]json.RawMessage
		if err := json.Unmarshal(r, &members); err != nil {
			return Scenario{}, fmt.Errorf("step %d: invalid object", i)
		}
		var op string
		_ = json.Unmarshal(members["op"], &op)
		allowed, ok := opFields[op]
		if !ok {
			return Scenario{}, fmt.Errorf("step %d: unsupported op %q", i, op)
		}
		keys := make([]string, 0, len(members))
		for k := range members {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !allowed[k] {
				return Scenario{}, fmt.Errorf("step %d: field %q forbidden for op %q", i, k, op)
			}
		}
		var st Step
		d := json.NewDecoder(bytes.NewReader(r))
		d.DisallowUnknownFields()
		if err := d.Decode(&st); err != nil {
			return Scenario{}, fmt.Errorf("step %d: %w", i, err)
		}
		if err := validateStep(i, st, members); err != nil {
			return Scenario{}, err
		}
		key := st.Session + "\x00" + st.Request
		switch st.Op {
		case "request":
			if seenRequests[st.Request] {
				return Scenario{}, fmt.Errorf("step %d: manager request %q is not unique", i, st.Request)
			}
			seenRequests[st.Request] = true
			declaredRequests[st.Request] = struct {
				session    string
				generation int
			}{st.Session, st.Generation}
		case "child":
			p, ok := declaredRequests[st.Request]
			if !ok || p.session != st.Session || p.generation != st.Generation {
				return Scenario{}, fmt.Errorf("step %d: child prerequisite request/session/generation mismatch", i)
			}
			want := nextOrdinal[key] + 1
			if st.Ordinal != want {
				return Scenario{}, fmt.Errorf("step %d: child ordinal %d for request %q; want %d", i, st.Ordinal, st.Request, want)
			}
			lspKey := childKey(st.Session, uint64(st.Generation), st.LSPRequest)
			if seenLSP[lspKey] {
				return Scenario{}, fmt.Errorf("step %d: lsp request %q is not unique in session generation", i, st.LSPRequest)
			}
			seenLSP[lspKey] = true
			declaredChildren[lspKey+"\x00"+st.Child] = true
			nextOrdinal[key] = want
		case "respond", "late_response", "cancel_write_failed":
			if !declaredChildren[childKey(st.Session, uint64(st.Generation), st.LSPRequest)+"\x00"+st.Child] {
				return Scenario{}, fmt.Errorf("step %d: response prerequisite session/generation/lsp_request/child mismatch", i)
			}
		case "cancel", "timeout":
			p, ok := declaredRequests[st.Request]
			if !ok || p.session != st.Session || p.generation != st.Generation {
				return Scenario{}, fmt.Errorf("step %d: terminal prerequisite request/session/generation mismatch", i)
			}
		}
		s.Steps = append(s.Steps, st)
	}
	s.canonical, _ = canonicalJSON(scenarioWire{Version: s.Version, Steps: s.Steps})
	return s, nil
}

type scenarioWire struct {
	Version string `json:"version"`
	Steps   []Step `json:"steps"`
}

func validateStep(i int, s Step, members map[string]json.RawMessage) error {
	require := func(names ...string) error {
		for _, n := range names {
			if _, ok := members[n]; !ok {
				return fmt.Errorf("step %d: field %q required for op %q", i, n, s.Op)
			}
		}
		return nil
	}
	positive := func() error {
		if s.Generation <= 0 {
			return fmt.Errorf("step %d: generation must be positive", i)
		}
		return nil
	}
	ids := []struct{ name, value string }{{"session", s.Session}, {"request", s.Request}, {"lsp_request", s.LSPRequest}, {"child", s.Child}, {"caller", s.Caller}, {"intent_id", s.IntentID}, {"intent_ref", s.IntentRef}, {"bind", s.Bind}}
	for _, id := range ids {
		if id.value != "" && len([]byte(id.value)) > maxIdentityBytes {
			return fmt.Errorf("step %d: %s exceeds %d bytes", i, id.name, maxIdentityBytes)
		}
	}
	switch s.Op {
	case "startup", "initialize", "crash", "poison":
		if err := require("session", "generation", "outcome"); err != nil {
			return err
		}
		return positive()
	case "request":
		if err := require("session", "generation", "request"); err != nil {
			return err
		}
		if err := positive(); err != nil {
			return err
		}
	case "child":
		if err := require("session", "generation", "request", "lsp_request", "child", "ordinal"); err != nil {
			return err
		}
		if err := positive(); err != nil {
			return err
		}
		if s.Ordinal <= 0 {
			return fmt.Errorf("step %d: ordinal must be positive", i)
		}
	case "respond", "late_response", "cancel_write_failed":
		if err := require("session", "generation", "lsp_request", "child"); err != nil {
			return err
		}
		return positive()
	case "timeout", "cancel":
		if err := require("session", "generation", "request"); err != nil {
			return err
		}
		return positive()
	case "lifecycle_register":
		if err := require("session", "generation", "state", "expect"); err != nil {
			return err
		}
		if err := positive(); err != nil {
			return err
		}
		if !containsState(s.State) {
			return fmt.Errorf("step %d: state %q is not in closed vocabulary", i, s.State)
		}
	case "lifecycle":
		if err := require("session", "generation", "operation", "caller", "expect"); err != nil {
			return err
		}
		if err := positive(); err != nil {
			return err
		}
		if s.Operation != "STATUS" && s.Operation != "STOP" && s.Operation != "RESTART" {
			return fmt.Errorf("step %d: operation %q is not in closed vocabulary", i, s.Operation)
		}
	case "lifecycle_complete", "lifecycle_detach":
		if err := require("session", "expect"); err != nil {
			return err
		}
		if (s.IntentID == "") == (s.IntentRef == "") {
			return fmt.Errorf("step %d: exactly one of intent_id or intent_ref is required", i)
		}
		if s.Op == "lifecycle_detach" {
			if err := require("caller", "outcome"); err != nil {
				return err
			}
			if s.Outcome != "REQUEST_CANCELLED" && s.Outcome != "REQUEST_TIMEOUT" {
				return fmt.Errorf("step %d: outcome %q is not in closed detach vocabulary", i, s.Outcome)
			}
		}
	case "admit":
		if err := require("session", "expect"); err != nil {
			return err
		}
	case "evict_complete":
		if err := require("expect"); err != nil {
			return err
		}
		if (s.IntentID == "") == (s.IntentRef == "") {
			return fmt.Errorf("step %d: exactly one of intent_id or intent_ref is required", i)
		}
	}
	for _, id := range ids {
		if _, present := members[id.name]; present && id.value == "" {
			return fmt.Errorf("step %d: field %q must be nonempty", i, id.name)
		}
	}
	return nil
}

func containsState(v string) bool {
	for _, s := range session.PublicStates() {
		if string(s) == v {
			return true
		}
	}
	return false
}
func (s Scenario) CanonicalBytes() []byte { return append([]byte(nil), s.canonical...) }

type record struct {
	LedgerVersion string              `json:"ledger_version"`
	RecordType    string              `json:"record_type"`
	Event         *eventRecord        `json:"event,omitempty"`
	MCPReference  *mcpReferenceRecord `json:"mcp_reference_result,omitempty"`
	Ownership     *ownershipRecord    `json:"child_ownership,omitempty"`
	Intent        *intentRecord       `json:"lifecycle_intent_history,omitempty"`
	Session       *sessionRecord      `json:"session_generation_history,omitempty"`
	Census        *censusRecord       `json:"final_resource_census,omitempty"`
}
type eventRecord struct {
	EventSeq         uint64 `json:"event_seq"`
	Kind             string `json:"kind"`
	SessionID        string `json:"session_id,omitempty"`
	Generation       uint64 `json:"generation,omitempty"`
	ManagerRequestID string `json:"manager_request_id,omitempty"`
	LSPRequestID     string `json:"lsp_request_id,omitempty"`
}
type mcpReferenceRecord struct {
	SimulatedReference bool   `json:"simulated_reference"`
	Step               int    `json:"step"`
	Operation          string `json:"operation"`
	Kind               string `json:"kind,omitempty"`
	SessionID          string `json:"session_id,omitempty"`
	Victim             string `json:"victim,omitempty"`
	ReservationID      string `json:"reservation_id,omitempty"`
	State              string `json:"state,omitempty"`
	PriorState         string `json:"prior_state,omitempty"`
	Generation         uint64 `json:"generation,omitempty"`
	IntentID           string `json:"intent_id,omitempty"`
	IntentKind         string `json:"intent_kind,omitempty"`
	Outcome            string `json:"outcome,omitempty"`
	OperationStatus    string `json:"operation_status,omitempty"`
	Failure            string `json:"failure,omitempty"`
	Joined             bool   `json:"joined"`
	Replayed           bool   `json:"replayed"`
	Noop               bool   `json:"noop"`
	Detached           *bool  `json:"detached,omitempty"`
}
type ownershipRecord struct {
	SessionID              string `json:"session_id"`
	Generation             uint64 `json:"generation"`
	ManagerRequestID       string `json:"manager_request_id"`
	LSPRequestID           string `json:"lsp_request_id,omitempty"`
	ParentManagerRequestID string `json:"parent_manager_request_id,omitempty"`
	Ordinal                uint64 `json:"ordinal,omitempty"`
	ResponseState          string `json:"response_state"`
	CancelState            string `json:"cancel_state"`
	DispatchEventSeq       uint64 `json:"dispatch_event_seq,omitempty"`
	ResponseEventSeq       uint64 `json:"response_event_seq,omitempty"`
	CancelEventSeq         uint64 `json:"cancel_event_seq,omitempty"`
	TerminalEventSeq       uint64 `json:"terminal_event_seq,omitempty"`
	TombstoneEventSeq      uint64 `json:"tombstone_event_seq,omitempty"`
}
type intentRecord struct {
	EventSeq   uint64 `json:"event_seq"`
	SessionID  string `json:"session_id"`
	Generation uint64 `json:"generation"`
	Action     string `json:"action"`
	IntentID   string `json:"intent_id,omitempty"`
	CallerID   string `json:"caller_id,omitempty"`
	State      string `json:"state"`
	IntentKind string `json:"intent_kind,omitempty"`
	Failure    string `json:"failure,omitempty"`
}
type sessionRecord struct {
	EventSeq   uint64 `json:"event_seq"`
	SessionID  string `json:"session_id"`
	Generation uint64 `json:"generation"`
	State      string `json:"state"`
}
type censusRecord struct {
	LiveSessions             uint64 `json:"live_sessions"`
	TerminalSessions         uint64 `json:"terminal_sessions"`
	Generations              uint64 `json:"generations"`
	LifecycleLedgerRetained  uint64 `json:"lifecycle_ledger_retained"`
	LifecycleLedgerOmitted   uint64 `json:"lifecycle_ledger_omitted"`
	LifecycleLedgerTruncated bool   `json:"lifecycle_ledger_truncated"`
	TombstonesRetained       uint64 `json:"tombstones_retained"`
	TombstonesEvicted        uint64 `json:"tombstones_evicted"`
	TombstonesConsumed       uint64 `json:"tombstones_consumed"`
	Waiters                  uint64 `json:"waiters"`
	Reservations             uint64 `json:"reservations"`
	Intents                  uint64 `json:"intents"`
	Callers                  uint64 `json:"callers"`
	ManagerRequests          uint64 `json:"manager_requests"`
	LSPRequests              uint64 `json:"lsp_requests"`
	Tombstones               uint64 `json:"tombstones"`
	FakeChildren             uint64 `json:"fake_children"`
	FakePipes                uint64 `json:"fake_pipes"`
	FakeGoroutines           uint64 `json:"fake_goroutines"`
	OSProcessesExercised     bool   `json:"os_processes_exercised"`
}

type requestModel struct {
	session    string
	generation uint64
	terminal   bool
	children   []*childModel
}
type childModel struct {
	id, child, session, parent                                string
	generation                                                uint64
	ordinal                                                   uint64
	response, cancel                                          string
	dispatch, responseEvent, cancelEvent, terminal, tombstone uint64
	tombstoneConsumed, tombstoneEvicted                       bool
}

type Interpreter struct {
	manager      *session.Manager
	events       uint64
	bindings     map[string]string
	requests     map[string]*requestModel
	children     map[string]*childModel
	records      []record
	sessions     map[string]uint64
	intents      map[string]bool
	callers      map[string]bool
	reservations map[string]bool
}

func NewInterpreter(maxSessions int) (*Interpreter, error) {
	m, e := session.NewManager(session.ManagerConfig{MaxSessions: maxSessions})
	if e != nil {
		return nil, e
	}
	return &Interpreter{manager: m, bindings: map[string]string{}, requests: map[string]*requestModel{}, children: map[string]*childModel{}, sessions: map[string]uint64{}, intents: map[string]bool{}, callers: map[string]bool{}, reservations: map[string]bool{}}, nil
}
func (i *Interpreter) emit(r record) {
	r.LedgerVersion = LedgerVersion
	i.records = append(i.records, r)
}
func (i *Interpreter) event(kind, sid string, g uint64, mrid, lspid string) uint64 {
	i.events++
	i.emit(record{RecordType: "event", Event: &eventRecord{EventSeq: i.events, Kind: kind, SessionID: sid, Generation: g, ManagerRequestID: mrid, LSPRequestID: lspid}})
	return i.events
}
func childKey(sessionID string, generation uint64, lspID string) string {
	return fmt.Sprintf("%s\x00%d\x00%s", sessionID, generation, lspID)
}

func (i *Interpreter) intentID(s Step) (string, error) {
	if s.IntentID != "" {
		return s.IntentID, nil
	}
	id := i.bindings[s.IntentRef]
	if id == "" {
		return "", fmt.Errorf("intent_ref %q is unbound", s.IntentRef)
	}
	return id, nil
}
func (i *Interpreter) Execute(s Scenario) ([]byte, error) {
	for n, st := range s.Steps {
		if err := i.step(n, st); err != nil {
			return nil, fmt.Errorf("step %d: %w", n, err)
		}
	}
	lifecycle := i.manager.LifecycleLedger()
	generationHistory := map[string]sessionRecord{}
	for _, e := range lifecycle {
		i.emit(record{RecordType: "lifecycle_intent_history", Intent: &intentRecord{EventSeq: e.Seq, SessionID: e.SessionID, Generation: e.Generation, Action: e.Action, IntentID: e.IntentID, CallerID: e.CallerID, State: string(e.State), IntentKind: string(e.Kind), Failure: string(e.Failure)}})
		key := fmt.Sprintf("%s\x00%020d", e.SessionID, e.Generation)
		generationHistory[key] = sessionRecord{EventSeq: e.Seq, SessionID: e.SessionID, Generation: e.Generation, State: string(e.State)}
	}
	history := make([]sessionRecord, 0, len(generationHistory))
	latestBySession := map[string]sessionRecord{}
	for _, r := range generationHistory {
		history = append(history, r)
		latest, ok := latestBySession[r.SessionID]
		if !ok || r.Generation > latest.Generation {
			latestBySession[r.SessionID] = r
		}
	}
	// event_seq is session-local Manager history. Cross-session ordering uses
	// the explicit stable (session_id, generation) identity instead.
	sort.Slice(history, func(a, b int) bool {
		if history[a].SessionID != history[b].SessionID {
			return history[a].SessionID < history[b].SessionID
		}
		return history[a].Generation < history[b].Generation
	})
	for n := range history {
		r := history[n]
		i.emit(record{RecordType: "session_generation_history", Session: &r})
	}
	requestIDs := make([]string, 0, len(i.requests))
	for requestID := range i.requests {
		requestIDs = append(requestIDs, requestID)
	}
	sort.Strings(requestIDs)
	for _, requestID := range requestIDs {
		i.emitOwnership(i.requests[requestID])
	}
	i.applyTombstoneRetention()
	live, terminal := uint64(0), uint64(0)
	for _, current := range latestBySession {
		switch current.State {
		case string(session.Stopped), string(session.Crashed), string(session.Poisoned):
			terminal++
		default:
			live++
		}
	}
	retainedTombstones, evictedTombstones, consumedTombstones := i.tombstoneCounts()
	ledgerRetained := uint64(len(lifecycle))
	// Manager exposes a bounded projection but not an exact pre-truncation count.
	// Equality with the bound is disclosed as truncated with a conservative
	// omitted lower bound rather than falsely claiming completeness.
	ledgerOmitted := uint64(0)
	ledgerTruncated := len(lifecycle) == 1024
	if ledgerTruncated {
		ledgerOmitted = 1
	}
	c := &censusRecord{LiveSessions: live, TerminalSessions: terminal, Generations: uint64(len(generationHistory)), LifecycleLedgerRetained: ledgerRetained, LifecycleLedgerOmitted: ledgerOmitted, LifecycleLedgerTruncated: ledgerTruncated, TombstonesRetained: retainedTombstones, TombstonesEvicted: evictedTombstones, TombstonesConsumed: consumedTombstones, Waiters: 0, Reservations: uint64(len(i.reservations)), Intents: uint64(len(i.intents)), Callers: uint64(len(i.callers)), ManagerRequests: uint64(len(i.requests)), LSPRequests: uint64(len(i.children)), Tombstones: retainedTombstones, FakeChildren: 0, FakePipes: 0, FakeGoroutines: 0, OSProcessesExercised: false}
	i.emit(record{RecordType: "final_resource_census", Census: c})
	encoded, err := encodeJSONL(i.records)
	if err != nil {
		return nil, err
	}
	if err := ValidateLedgerJSONL(encoded); err != nil {
		return nil, fmt.Errorf("encoded ledger failed structural validation: %w", err)
	}
	return encoded, nil
}
func (i *Interpreter) step(n int, s Step) error {
	switch s.Op {
	case "startup", "initialize", "crash", "poison":
		i.event(s.Op, s.Session, uint64(s.Generation), "", "")
	case "admit":
		a := i.manager.Admit(s.Session, 0)
		got := admissionReference(a)
		i.emitMCP(n, s, got)
		if err := expect(s.Expect, got, i.bindings); err != nil {
			return err
		}
		if a.Kind == session.AdmissionFree {
			i.sessions[s.Session] = 1
		}
		if a.Kind == session.AdmissionEvict && a.Reservation != "" {
			i.reservations[a.Reservation] = true
		}
		return nil
	case "lifecycle_register":
		before := i.manager.LifecycleLedger()
		i.manager.RegisterLifecycle(s.Session, uint64(s.Generation), session.State(s.State), s.Reaped)
		after := i.manager.LifecycleLedger()
		if len(after) == 0 {
			return fmt.Errorf("manager lifecycle registration was not accepted")
		}
		registered := after[len(after)-1]
		if len(before) != 0 && before[len(before)-1].Seq == registered.Seq && before[len(before)-1].SessionID == registered.SessionID {
			return fmt.Errorf("manager lifecycle registration was not accepted")
		}
		if registered.Action != "register" || registered.SessionID != s.Session || registered.Generation != uint64(s.Generation) || string(registered.State) != s.State {
			return fmt.Errorf("manager lifecycle registration exact success mismatch")
		}
		got := ManagerExpectation{State: s.State, Generation: uint64(s.Generation), Outcome: "COMPLETE", OperationStatus: "SUCCEEDED"}
		i.emitMCP(n, s, got)
		if err := expect(s.Expect, got, i.bindings); err != nil {
			return err
		}
		i.sessions[s.Session] = uint64(s.Generation)
		return nil
	case "lifecycle":
		r := i.manager.Lifecycle(session.LifecycleRequest{SessionID: s.Session, Generation: uint64(s.Generation), Operation: session.LifecycleOperation(s.Operation), CallerID: s.Caller, ChildRisk: s.ChildRisk, HasWork: s.HasWork})
		got := lifecycleReference(r)
		if s.Bind != "" && r.IntentID != "" {
			if i.bindings[s.Bind] != "" && i.bindings[s.Bind] != r.IntentID {
				return fmt.Errorf("binding %q already bound", s.Bind)
			}
			i.bindings[s.Bind] = r.IntentID
		}
		if r.IntentID != "" {
			i.intents[r.IntentID] = true
		}
		if s.Caller != "" {
			i.callers[s.Session+"\x00"+r.IntentID+"\x00"+s.Caller] = true
		}
		i.emitMCP(n, s, got)
		return expect(s.Expect, got, i.bindings)
	case "lifecycle_complete":
		id, e := i.intentID(s)
		if e != nil {
			return e
		}
		r := i.manager.CompleteLifecycle(s.Session, id, s.Death, s.Reaped, s.Ready)
		got := lifecycleReference(r)
		i.emitMCP(n, s, got)
		if err := expect(s.Expect, got, i.bindings); err != nil {
			return err
		}
		if got.OperationStatus == "SUCCEEDED" && got.Failure == "" && got.Generation > 0 {
			i.sessions[s.Session] = got.Generation
		}
		return nil
	case "lifecycle_detach":
		id, e := i.intentID(s)
		if e != nil {
			return e
		}
		b := i.manager.DetachLifecycleCaller(s.Session, id, s.Caller, session.Failure(s.Outcome))
		got := ManagerExpectation{IntentID: id, Detached: &b}
		i.emitMCP(n, s, got)
		return expect(s.Expect, got, i.bindings)
	case "evict_complete":
		id, e := i.intentID(s)
		if e != nil {
			return e
		}
		a := i.manager.CompleteEviction(id, s.Success)
		got := admissionReference(a)
		i.emitMCP(n, s, got)
		if err := expect(s.Expect, got, i.bindings); err != nil {
			return err
		}
		if s.Success && a.Kind == session.AdmissionFree {
			delete(i.reservations, id)
		}
		return nil
	case "request":
		i.event("request_admitted", s.Session, uint64(s.Generation), s.Request, "")
		i.requests[s.Request] = &requestModel{session: s.Session, generation: uint64(s.Generation)}
	case "child":
		r := i.requests[s.Request]
		if r == nil || r.session != s.Session || r.generation != uint64(s.Generation) {
			return fmt.Errorf("child prerequisite request/session/generation mismatch")
		}
		c := &childModel{id: s.LSPRequest, child: s.Child, session: s.Session, parent: s.Request, generation: uint64(s.Generation), ordinal: uint64(s.Ordinal), response: "UNRESOLVED", cancel: "NOT_REQUESTED"}
		c.dispatch = i.event("lsp_dispatch", s.Session, c.generation, s.Request, c.id)
		r.children = append(r.children, c)
		i.children[childKey(s.Session, c.generation, c.id)] = c
	case "respond":
		c := i.children[childKey(s.Session, uint64(s.Generation), s.LSPRequest)]
		if c == nil || c.child != s.Child {
			return fmt.Errorf("response prerequisite session/generation/lsp_request/child mismatch")
		}
		if c.response != "UNRESOLVED" {
			return fmt.Errorf("duplicate response")
		}
		c.response = "RESPONDED"
		c.responseEvent = i.event("lsp_response", s.Session, c.generation, c.parent, c.id)
		c.terminal = c.responseEvent
	case "cancel", "timeout":
		r := i.requests[s.Request]
		if r == nil || r.session != s.Session || r.generation != uint64(s.Generation) {
			return fmt.Errorf("terminal prerequisite request/session/generation mismatch")
		}
		seq := i.event(s.Op, s.Session, r.generation, s.Request, "")
		r.terminal = true
		for _, c := range r.children {
			if c.response == "UNRESOLVED" {
				c.cancel = "SENT"
				c.cancelEvent = seq
				c.terminal = seq
				c.response = "TOMBSTONED"
				c.tombstone = i.event("tombstone", s.Session, c.generation, c.parent, c.id)
			}
		}
	case "cancel_write_failed":
		c := i.children[childKey(s.Session, uint64(s.Generation), s.LSPRequest)]
		if c == nil || c.child != s.Child || c.parent != s.Request || c.cancel != "SENT" {
			return fmt.Errorf("cancel-write prerequisite exact tuple/state mismatch")
		}
		c.cancel = "WRITE_FAILED"
		c.cancelEvent = i.event("cancel_write_failed", s.Session, c.generation, c.parent, c.id)
	case "late_response":
		c := i.children[childKey(s.Session, uint64(s.Generation), s.LSPRequest)]
		if c == nil || c.child != s.Child || c.response != "TOMBSTONED" {
			return fmt.Errorf("unknown or ambiguous late response")
		}
		c.response = "RESPONDED"
		c.tombstoneConsumed = true
		c.responseEvent = i.event("late_response_discarded", s.Session, c.generation, c.parent, c.id)
	}
	return nil
}
func (i *Interpreter) emitMCP(n int, s Step, g ManagerExpectation) {
	i.emit(record{RecordType: "mcp_reference_result", MCPReference: &mcpReferenceRecord{SimulatedReference: true, Step: n, Operation: s.Op, Kind: g.Kind, SessionID: g.SessionID, Victim: g.Victim, ReservationID: g.Reservation, State: g.State, PriorState: g.PriorState, Generation: g.Generation, IntentID: g.IntentID, IntentKind: g.IntentKind, Outcome: g.Outcome, OperationStatus: g.OperationStatus, Failure: g.Failure, Joined: g.Joined, Replayed: g.Replayed, Noop: g.Noop, Detached: g.Detached}})
}
func (i *Interpreter) emitOwnership(r *requestModel) {
	kids := append([]*childModel(nil), r.children...)
	sort.Slice(kids, func(a, b int) bool { return kids[a].ordinal < kids[b].ordinal })
	for _, c := range kids {
		i.emit(record{RecordType: "child_ownership", Ownership: &ownershipRecord{SessionID: r.session, Generation: r.generation, ManagerRequestID: c.parent, LSPRequestID: c.id, ParentManagerRequestID: c.parent, Ordinal: c.ordinal, ResponseState: c.response, CancelState: c.cancel, DispatchEventSeq: c.dispatch, ResponseEventSeq: c.responseEvent, CancelEventSeq: c.cancelEvent, TerminalEventSeq: c.terminal, TombstoneEventSeq: c.tombstone}})
	}
}

const (
	maxRetainedTombstones = 4
	maxTombstoneEventAge  = 8
)

func (i *Interpreter) applyTombstoneRetention() {
	var retained []*childModel
	for _, c := range i.children {
		if c.tombstone == 0 || c.tombstoneEvicted {
			continue
		}
		if i.events-c.tombstone > maxTombstoneEventAge {
			c.tombstoneEvicted = true
			continue
		}
		retained = append(retained, c)
	}
	sort.Slice(retained, func(a, b int) bool {
		if retained[a].tombstone != retained[b].tombstone {
			return retained[a].tombstone < retained[b].tombstone
		}
		return childKey(retained[a].session, retained[a].generation, retained[a].id) < childKey(retained[b].session, retained[b].generation, retained[b].id)
	})
	for len(retained) > maxRetainedTombstones {
		retained[0].tombstoneEvicted = true
		retained = retained[1:]
	}
}

func (i *Interpreter) tombstoneCounts() (retained, evicted, consumed uint64) {
	for _, c := range i.children {
		if c.tombstone == 0 {
			continue
		}
		if c.tombstoneEvicted {
			evicted++
		} else {
			retained++
		}
		if c.tombstoneConsumed {
			consumed++
		}
	}
	return
}
func admissionReference(a session.Admission) ManagerExpectation {
	return ManagerExpectation{Kind: string(a.Kind), SessionID: a.SessionID, Victim: a.Victim, Reservation: a.Reservation}
}
func lifecycleReference(r session.LifecycleResult) ManagerExpectation {
	return ManagerExpectation{State: string(r.State), PriorState: string(r.PriorState), Generation: r.Generation, IntentID: r.IntentID, IntentKind: string(r.Kind), Outcome: string(r.Outcome), OperationStatus: string(r.OperationStatus), Failure: string(r.Failure), Joined: r.Joined, Replayed: r.Replayed, Noop: r.Noop}
}
func expect(w *ManagerExpectation, g ManagerExpectation, b map[string]string) error {
	if w == nil {
		return fmt.Errorf("manager operation requires expect")
	}
	x := *w
	if x.IntentRef != "" {
		x.IntentID = b[x.IntentRef]
		x.IntentRef = ""
	}
	wb, _ := canonicalJSON(x)
	gb, _ := canonicalJSON(g)
	if !bytes.Equal(wb, gb) {
		return fmt.Errorf("manager outcome mismatch: got %s want %s", gb, wb)
	}
	return nil
}
func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
func encodeJSONL(rs []record) ([]byte, error) {
	var out strings.Builder
	for _, r := range rs {
		b, e := canonicalJSON(r)
		if e != nil {
			return nil, e
		}
		out.Write(b)
		out.WriteByte('\n')
	}
	return []byte(out.String()), nil
}

type IntegratedLedgers struct{ Scenario, Ownership, Intent, TerminalHistory, ResourceCensus []byte }

func ReplayIntegrated(s Scenario) (IntegratedLedgers, error) {
	i, e := NewInterpreter(max(1, len(s.Steps)))
	if e != nil {
		return IntegratedLedgers{}, e
	}
	all, e := i.Execute(s)
	if e != nil {
		return IntegratedLedgers{}, e
	}
	return IntegratedLedgers{Scenario: s.CanonicalBytes(), Ownership: all, Intent: all, TerminalHistory: all, ResourceCensus: all}, nil
}
