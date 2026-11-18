//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	ai "lsp-trace/internal/applicationinventory"
	dr "lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type InventoryFrameV3 struct {
	Protocol, Operation, Role, RequestDigest, ConfigSHA256, Grant    string
	Sequence                                                         int
	ApplicationSHA256, RunID, SourceRevision, LaunchID, PromptSHA256 string
	Pins                                                             dw.HostPinsV2
	DeadlineUnixNano                                                 int64
	Payload                                                          json.RawMessage
}

func inventoryRoleV3(op string) string {
	switch op {
	case "Describe", "GroupDecision":
		return "interpreter"
	case "Reviewer":
		return "reviewer"
	}
	return ""
}
func parseInventoryFrameV3(raw []byte) (InventoryFrameV3, error) {
	var f InventoryFrameV3
	if len(raw) > 1<<20 || cat.DecodeNativeV3(raw, &f) != nil || f.Protocol != InventoryProtocolV3 || inventoryRoleV3(f.Operation) == "" || f.Role != inventoryRoleV3(f.Operation) || f.Sequence < 1 || f.Sequence > 16 || f.SourceRevision != InventorySourceRevisionV3 || f.LaunchID == "" || len(f.LaunchID) > 256 || f.PromptSHA256 != hash([]byte(InventoryRolePromptV3(f.Role))) || f.DeadlineUnixNano <= 0 || len(f.Payload) == 0 || !json.Valid(f.Payload) || !digestValue(f.ConfigSHA256) || !digestValue(f.Grant) || !digestValue(f.ApplicationSHA256) || !digestValue(f.RunID) || !digestValue(f.Pins.WorkerSHA256) || !digestValue(f.Pins.ModelSHA256) || !digestValue(f.Pins.GrammarSHA256) {
		return f, ErrInvalid
	}
	prompt, e := inventoryPayloadV3(f.Operation, f.Payload)
	if e != nil || f.Pins.PromptSHA256 != prompt {
		return f, ErrInvalid
	}
	d := f.RequestDigest
	f.RequestDigest = ""
	if d != hash(mustJSON(f)) {
		return f, ErrInvalid
	}
	f.RequestDigest = d
	return f, nil
}
func inventoryPayloadV3(op string, raw []byte) (string, error) {
	if op == "Describe" {
		var r dr.Record
		if cat.DecodeNativeV3(raw, &r) != nil || r.Envelope.Protocol != dr.Protocol || r.Envelope.MessageType != dr.MessageType || !inventoryDescribeRevisionV3(r) || r.Envelope.Prompt == "" || len(r.Envelope.Prompt) > 1<<20 || r.Lineage.Authority != 0 || r.Lineage.Accepted || len(r.Envelope.InputSHA256) != 64 {
			return "", ErrInvalid
		}
		d := "sha256:" + r.Envelope.InputSHA256
		if !digestValue(d) {
			return "", ErrInvalid
		}
		return d, nil
	}
	var c cat.NativeCatalogWireV3
	if (op != "GroupDecision" && op != "Reviewer") || cat.DecodeNativeV3(raw, &c) != nil || c.Version != "lsp-trace.native-catalog.v3" || !digestValue(c.Revision) || c.Authority != 0 || c.Accepted || c.Completeness != "UNKNOWN" || c.Absence != "PROHIBITED" {
		return "", ErrInvalid
	}
	return "", nil
}
func inventoryPinsV3(actual, configured dw.HostPinsV2) bool {
	actual.PromptSHA256 = ""
	return actual == configured
}
func ReadInventoryBundleV3(b Bundle) (InventoryFrameV3, error) {
	s, e := mailboxStore(b.Directory)
	if e != nil {
		return InventoryFrameV3{}, e
	}
	defer s.close()
	return readInventoryBundleRootV3(b, s)
}
func readInventoryBundleRootV3(b Bundle, s *store) (InventoryFrameV3, error) {
	if b.Protocol != InventoryProtocolV3 {
		return InventoryFrameV3{}, ErrInvalid
	}
	raw, e := readPhysicalBundleRoot(b, s)
	if e != nil {
		return InventoryFrameV3{}, e
	}
	f, e := parseInventoryFrameV3(raw)
	if e != nil || f.Role != b.Role || uint64(f.Sequence) != b.Sequence || f.RequestDigest != b.RequestDigest {
		return f, ErrInvalid
	}
	return f, nil
}

// InventoryReadyV3 extends only inventory readiness. The shared legacy Ready
// wire stays unchanged. A declared mode never authenticates context identity.
type InventoryReadyV3 struct {
	Ready
	ReviewMode cat.ReviewModeV3 `json:",omitempty"`
}

func MarkInventoryReadyV3(path string, r Ready, modes ...cat.ReviewModeV3) error {
	var mode cat.ReviewModeV3
	if len(modes) == 1 {
		mode = modes[0]
	}
	if len(modes) > 1 || !mode.Valid() {
		return ErrInvalid
	}
	if r.Protocol != InventoryProtocolV3 || inventoryRoleV3(map[string]string{"interpreter": "Describe", "reviewer": "Reviewer"}[r.Role]) != r.Role || r.LaunchID == "" || len(r.LaunchID) > 256 || r.PromptSHA256 != hash([]byte(InventoryRolePromptV3(r.Role))) {
		return ErrInvalid
	}
	s, e := mailboxStore(path)
	if e != nil {
		return e
	}
	defer s.close()
	other := "interpreter"
	if r.Role == other {
		other = "reviewer"
	}
	raw, e := s.read("ready-"+other+".json", 4096)
	if e == nil {
		var o InventoryReadyV3
		if strict(raw, &o) != nil || o.ReviewMode != mode || (o.LaunchID == r.LaunchID && !mode.AllowsSameActor()) {
			return ErrInvalid
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	return s.write("ready-"+r.Role+".json", mustJSON(InventoryReadyV3{Ready: r, ReviewMode: mode}))
}
func inventoryReadReadyV3(s *store) ([2]InventoryReadyV3, error) {
	var out [2]InventoryReadyV3
	for i, role := range []string{"interpreter", "reviewer"} {
		raw, e := s.read("ready-"+role+".json", 4096)
		r := &out[i]
		if e != nil || strict(raw, r) != nil || !r.ReviewMode.Valid() || r.Protocol != InventoryProtocolV3 || r.Role != role || r.LaunchID == "" || len(r.LaunchID) > 256 || r.PromptSHA256 != hash([]byte(InventoryRolePromptV3(role))) {
			return out, ErrInvalid
		}
	}
	if out[0].ReviewMode != out[1].ReviewMode || (out[0].LaunchID == out[1].LaunchID && !out[0].ReviewMode.AllowsSameActor()) {
		return out, ErrInvalid
	}
	return out, nil
}
func inventoryRootIdentityV3(s *store) (string, error) {
	f, e := s.root.Open(".")
	if e != nil {
		return "", e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return "", e
	}
	v := st.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%d:%d", v.Dev, v.Ino), nil
}

type InventoryBindingV3 struct {
	PlannedCalls, ResponseLimit                                                          uint64
	ApplicationSHA256, ConfigSHA256, Grant, RunID, SourceRevision, Mailbox, RootIdentity string
	Ready                                                                                [2]InventoryReadyV3
	Pins                                                                                 dw.HostPinsV2
}
type inventoryLedgerV3 struct {
	Binding              InventoryBindingV3
	Allowance, Retention ApplicationBounds
	Deadline             int64
	Clock                applicationClockStamp
}
type inventoryExchangeV3 struct {
	Binding                               InventoryBindingV3
	Operation, RequestSHA256, FrameSHA256 string
	Sequence                              uint64
	Reservation                           OperationReservation
}
type InventoryPhysicalReturnV3 struct {
	Schema, EventID, ResponseSHA256, Disposition string
	Bytes, ElapsedNS                             uint64
}

type InventoryTransportV3 struct {
	Journal  *ApplicationJournal
	Request  InventoryRequestV3
	Binding  InventoryBindingV3
	held     *store
	sequence uint64
	last     [2]uint64
	timeout  time.Duration
}

func NewInventoryTransportV3(j *ApplicationJournal, q InventoryRequestV3, runID, mailbox string) (*InventoryTransportV3, error) {
	if j == nil || q.ApprovalSHA256 != InventoryRequestDigestV3(q) || ValidateInventoryRequestV3(q) != nil || !digestValue(runID) || j.Head().ApplicationSHA256 != ApplicationConfigDigest(q.Configuration) || mailbox != InventoryMailboxPathV3(q.Configuration.Store, runID) {
		return nil, ErrInvalid
	}
	s, e := mailboxStore(mailbox)
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*InventoryTransportV3, error) { s.close(); return nil, e }
	ready, e := inventoryReadReadyV3(s)
	if e != nil || ready[0].ReviewMode != q.Inventory.ReviewMode {
		return fail(ErrInvalid)
	}
	identity, e := inventoryRootIdentityV3(s)
	if e != nil {
		return fail(e)
	}
	planned := uint64(min(len(q.Inventory.Manifest.Leads), q.Inventory.MaxNativeAttempts) + 2)
	b := InventoryBindingV3{PlannedCalls: planned, ResponseLimit: (64 << 10) / planned, ApplicationSHA256: j.Head().ApplicationSHA256, ConfigSHA256: q.ApprovalSHA256, Grant: ApplicationGrantDigest(q.Configuration.Grant), RunID: runID, SourceRevision: q.Inventory.SourceRevision, Mailbox: mailbox, RootIdentity: identity, Ready: ready, Pins: q.Inventory.Pins}
	if e = j.admitInventoryV3(b); e != nil {
		return fail(e)
	}
	return &InventoryTransportV3{Journal: j, Request: q, Binding: b, held: s, timeout: time.Duration(q.Inventory.HostWindowMS) * time.Millisecond}, nil
}
func (t *InventoryTransportV3) LastSequences() [2]uint64 { return t.last }

// Reconcile through the original held capability, even after a pathname swap.
// This cannot admit READY or resume semantic execution.
func (t *InventoryTransportV3) ReconcilePhysicalReturnsV3() error {
	if t == nil || t.held == nil {
		return ErrInvalid
	}
	var ledger inventoryLedgerV3
	if e := t.Journal.implementation.object("inventory-ledger", inventoryLedgerIDV3(t.Binding), &ledger); e != nil {
		return e
	}
	if ledger.Binding != t.Binding {
		return ErrInvalid
	}
	if e := t.Journal.claimInventoryRecoveryV3(inventoryLedgerIDV3(t.Binding)); e != nil {
		return e
	}
	return t.Journal.reconcileInventoryRootV3(ledger, t.held)
}
func (t *InventoryTransportV3) Close() {
	if t != nil && t.held != nil {
		t.held.close()
		t.held = nil
	}
}
func inventoryLedgerIDV3(b InventoryBindingV3) string {
	return hash([]byte(b.ApplicationSHA256 + "/inventory-v3/" + b.RunID))
}
func inventoryExchangeIDV3(b InventoryBindingV3, n uint64) string {
	return hash([]byte(fmt.Sprintf("%s/%d", inventoryLedgerIDV3(b), n)))
}
func inventoryAllowanceV3(b InventoryBindingV3) ApplicationBounds {
	return ApplicationBounds{LogicalBytes: 64 << 20, ReadHashBytes: 128 << 20, WriteBytes: 8 << 20, Records: 4096, ParserBytes: 128 << 20, Work: 1 << 20, OutputBytes: 4 << 20, Calls: b.PlannedCalls, RequestBytes: 1 << 20, ResponseBytes: 64 << 10, WireBytes: (1 << 20) + (64 << 10), ActiveNS: uint64(120 * time.Second)}
}
func (a *ApplicationJournal) admitInventoryV3(b InventoryBindingV3) error {
	j := a.implementation
	if j.config.Schema != "private-host-application-config.v3" || b.ApplicationSHA256 != j.head.ApplicationSHA256 || b.Grant != ApplicationGrantDigest(j.config.Grant) || b.SourceRevision != InventorySourceRevisionV3 {
		return ErrInvalid
	}
	now := time.Now()
	first, e := applicationClockNow()
	if e != nil {
		return e
	}
	if e = j.attempt("INVENTORY_V3", now); e != nil {
		return e
	}
	id := inventoryLedgerIDV3(b)
	if j.exists("inventory-ledger", id) {
		return ErrApplicationSpent
	}
	allowance := inventoryAllowanceV3(b)
	retention, _ := applicationTimes(applicationRetentionEnvelope(), 16)
	prepaid, ok := applicationAdd(allowance, retention)
	if !ok {
		return ErrInvalid
	}
	total, ok := applicationAdd(j.head.Reserved, prepaid)
	if !ok || !applicationWithin(total, j.config.Grant.Ceiling) {
		return ErrPaused
	}
	stamp, e := applicationClockNow()
	if e != nil {
		return e
	}
	deadline, e := applicationAdmissionDeadline(now, allowance.ActiveNS, j.config.Grant.ExpiresUnixNano, first, stamp)
	if e != nil {
		return e
	}
	h := j.head
	h.Reserved = total
	return j.commit("PREPAY_INVENTORY_V3", id, h, applicationObject{applicationObjectName("inventory-ledger", id), mustJSON(inventoryLedgerV3{b, allowance, retention, deadline, stamp})})
}
func (t *InventoryTransportV3) Exchange(operation string) ai.ByteExchange {
	return func(ctx context.Context, payload []byte, out io.Writer) error {
		return t.exchange(ctx, operation, payload, out)
	}
}
func (t *InventoryTransportV3) exchange(ctx context.Context, operation string, payload []byte, out io.Writer) error {
	if t == nil || t.held == nil || ctx == nil || ctx.Err() != nil || out == nil || inventoryRoleV3(operation) == "" || len(payload) == 0 || len(payload) > ((1<<20)-32768)/6 || !json.Valid(payload) {
		return ErrInvalid
	}
	// Bound worst JSON escaping/copy/readback before encoding or any mailbox effect.
	if t.sequence >= 16 {
		return ErrPaused
	}
	role := inventoryRoleV3(operation)
	index := 0
	if role == "reviewer" {
		index = 1
	}
	current, e := mailboxStore(t.Binding.Mailbox)
	if e != nil {
		return e
	}
	id, e := inventoryRootIdentityV3(current)
	current.close()
	if e != nil || id != t.Binding.RootIdentity {
		return ErrInvalid
	}
	ready, e := inventoryReadReadyV3(t.held)
	if e != nil || ready != t.Binding.Ready {
		return ErrInvalid
	}
	var ledger inventoryLedgerV3
	j := t.Journal.implementation
	if e = j.object("inventory-ledger", inventoryLedgerIDV3(t.Binding), &ledger); e != nil || ledger.Binding != t.Binding {
		return ErrInvalid
	}
	stamp, e := applicationClockNow()
	if e != nil {
		return e
	}
	remaining, e := applicationIntervalRemaining(applicationUnitLedger{Allowance: ledger.Allowance, ActiveDeadlineUnixNano: ledger.Deadline, Clock: ledger.Clock}, stamp)
	if e != nil || remaining <= 0 {
		return ErrPaused
	}
	deadline := time.Now().Add(min(remaining, t.timeout))
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	f := InventoryFrameV3{Protocol: InventoryProtocolV3, Operation: operation, Role: role, ConfigSHA256: t.Binding.ConfigSHA256, Grant: t.Binding.Grant, Sequence: int(t.sequence + 1), ApplicationSHA256: t.Binding.ApplicationSHA256, RunID: t.Binding.RunID, SourceRevision: t.Binding.SourceRevision, LaunchID: ready[index].LaunchID, PromptSHA256: ready[index].PromptSHA256, Pins: t.Binding.Pins, DeadlineUnixNano: deadline.UnixNano(), Payload: append(json.RawMessage(nil), payload...)}
	prompt, payloadErr := inventoryPayloadV3(operation, payload)
	if payloadErr != nil {
		return payloadErr
	}
	f.Pins.PromptSHA256 = prompt
	f.RequestDigest = hash(mustJSON(f))
	raw := mustJSON(f)
	if len(raw) > 1<<20 {
		return ErrInvalid
	}
	claim, e := t.reserve(f, raw)
	if e != nil {
		return e
	}
	t.sequence++
	t.last[index] = t.sequence
	wait, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	start := time.Now()
	reply, physicalErr := exchangePhysicalRoot(wait, t.held, t.Binding.Mailbox, t.timeout, cohortFrameHeader{Protocol: f.Protocol, Role: f.Role, RequestDigest: f.RequestDigest, ConfigSHA256: f.ConfigSHA256, Grant: f.Grant, Sequence: f.Sequence}, raw)
	disposition := "RETURNED"
	if physicalErr != nil {
		disposition = "RETURNED_WITH_ERROR"
	}
	// Always close the physical return transaction, including zero-byte errors.
	recordErr := t.Journal.recordInventoryReturnV3(claim, reply, disposition, uint64(time.Since(start)))
	if recordErr != nil {
		return errors.Join(recordErr, physicalErr)
	}
	if physicalErr != nil || len(reply) == 0 || uint64(len(reply)) > t.Binding.ResponseLimit || wait.Err() != nil {
		return errors.Join(ErrInvalid, physicalErr, wait.Err())
	}
	if !json.Valid(reply) {
		return ErrInvalid
	}
	n, e := out.Write(reply)
	if e == nil && n != len(reply) {
		e = io.ErrShortWrite
	}
	return e
}
func (t *InventoryTransportV3) reserve(f InventoryFrameV3, raw []byte) (inventoryExchangeV3, error) {
	j := t.Journal.implementation
	var zero inventoryExchangeV3
	if e := j.attempt("INVENTORY_EXCHANGE_V3", time.Now()); e != nil {
		return zero, e
	}
	id := inventoryExchangeIDV3(t.Binding, uint64(f.Sequence))
	if j.exists("inventory-exchange", id) {
		return zero, ErrApplicationSpent
	}
	var used ApplicationBounds
	previousOperation := ""
	for n := uint64(1); n < uint64(f.Sequence); n++ {
		var prev inventoryExchangeV3
		pid := inventoryExchangeIDV3(t.Binding, n)
		if j.object("inventory-exchange", pid, &prev) != nil || !j.exists("inventory-return", pid) {
			return zero, ErrApplicationAmbiguous
		}
		previousOperation = prev.Operation
		var ok bool
		used, ok = applicationAdd(used, prev.Reservation.Bounds)
		if !ok {
			return zero, ErrInvalid
		}
	}
	if previousOperation == "Reviewer" || previousOperation == "GroupDecision" && f.Operation != "Reviewer" || f.Operation == "Reviewer" && previousOperation != "GroupDecision" {
		return zero, ErrInvalid
	}
	// Partition the unchanged cumulative 64KiB response ceiling before effects;
	// native failures do not refund or enlarge the remaining per-slot partition.
	b := ApplicationBounds{Calls: 1, RequestBytes: uint64(len(raw)), ResponseBytes: t.Binding.ResponseLimit, WireBytes: uint64(len(raw)) + t.Binding.ResponseLimit, ActiveNS: uint64(t.timeout)}
	used, ok := applicationAdd(used, b)
	var ledger inventoryLedgerV3
	if !ok || j.object("inventory-ledger", inventoryLedgerIDV3(t.Binding), &ledger) != nil || !applicationWithin(used, ledger.Allowance) {
		return zero, ErrPaused
	}
	r := OperationReservation{EventID: id, Kind: "inventory-exchange.v3", Bounds: b, Sequence: j.head.Sequence + 1}
	claim := inventoryExchangeV3{t.Binding, f.Operation, f.RequestDigest, hash(raw), uint64(f.Sequence), r}
	e := j.commit("INVENTORY_EXCHANGE_V3", id, j.head, applicationObject{applicationObjectName("inventory-exchange", id), mustJSON(claim)})
	return claim, e
}
func (a *ApplicationJournal) recordInventoryReturnV3(c inventoryExchangeV3, raw []byte, disposition string, elapsed uint64) error {
	j := a.implementation
	id := c.Reservation.EventID
	if j.closed || j.poisoned || len(raw) > 65537 || c.Reservation.Kind != "inventory-exchange.v3" {
		return ErrInvalid
	}
	var stored inventoryExchangeV3
	if j.object("inventory-exchange", id, &stored) != nil || stored != c {
		return ErrInvalid
	}
	o := InventoryPhysicalReturnV3{"private-application-inventory-physical-return.v3", id, hash(raw), disposition, uint64(len(raw)), elapsed}
	if j.exists("inventory-return", id) {
		var old InventoryPhysicalReturnV3
		if j.object("inventory-return", id, &old) != nil || old.ResponseSHA256 != o.ResponseSHA256 || old.Bytes != o.Bytes {
			return ErrApplicationAmbiguous
		}
		return nil
	}
	objects := []applicationObject{{applicationObjectName("inventory-return", id), mustJSON(o)}}
	// Raw chunks avoid JSON/base64 expansion exceeding the fixed record limit.
	for start, n := 0, 0; start < len(raw); n++ {
		end := min(start+32768, len(raw))
		objects = append(objects, applicationObject{applicationObjectName(fmt.Sprintf("inventory-response-%d", n), id), append([]byte(nil), raw[start:end]...)})
		start = end
	}
	h := j.head
	actual := ApplicationBounds{Calls: 1, RequestBytes: c.Reservation.Bounds.RequestBytes, ResponseBytes: uint64(len(raw)), WireBytes: c.Reservation.Bounds.RequestBytes + uint64(len(raw)), ActiveNS: elapsed}
	var ok bool
	h.Observed, ok = applicationAdd(h.Observed, actual)
	if !ok {
		return ErrInvalid
	}
	return j.commit("INVENTORY_PHYSICAL_RETURN_V3", id, h, objects...)
}
func WaitInventoryV3(ctx context.Context, path, role string, after uint64, q InventoryRequestV3, runID string) (CohortWait, error) {
	if role != "interpreter" && role != "reviewer" || after > 16 {
		return CohortWait{}, ErrInvalid
	}
	s, e := mailboxStore(path)
	if e != nil {
		return CohortWait{}, e
	}
	defer s.close()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if ctx.Err() != nil {
			return CohortWait{Event: "DEADLINE"}, ctx.Err()
		}
		for n := after + 1; n <= 16; n++ {
			raw, e := s.read(fmt.Sprintf("request-%08d.json", n), 47999)
			if os.IsNotExist(e) {
				continue
			}
			var b Bundle
			if e != nil || strict(raw, &b) != nil || b.Directory != path || b.Sequence != n {
				return CohortWait{}, ErrInvalid
			}
			f, e := readInventoryBundleRootV3(b, s)
			if e != nil || f.ConfigSHA256 != q.ApprovalSHA256 || f.ApplicationSHA256 != ApplicationConfigDigest(q.Configuration) || f.Grant != ApplicationGrantDigest(q.Configuration.Grant) || !inventoryPinsV3(f.Pins, q.Inventory.Pins) || f.RunID != runID {
				return CohortWait{}, ErrInvalid
			}
			ready, readyErr := inventoryReadReadyV3(s)
			idx := 0
			if f.Role == "reviewer" {
				idx = 1
			}
			if readyErr != nil || ready[0].ReviewMode != q.Inventory.ReviewMode || f.LaunchID != ready[idx].LaunchID || f.PromptSHA256 != ready[idx].PromptSHA256 {
				return CohortWait{}, ErrInvalid
			}
			if f.Role == role {
				return CohortWait{Event: "NEEDS_HOST", Bundle: &b}, nil
			}
		}
		select {
		case <-ctx.Done():
			return CohortWait{Event: "DEADLINE"}, ctx.Err()
		case <-tick.C:
		}
	}
}
func SubmitInventoryV3(path, role string, seq uint64, payload []byte, q InventoryRequestV3, runID string) error {
	if seq < 1 || seq > 16 || len(payload) > 65537 {
		return ErrInvalid
	}
	s, e := mailboxStore(path)
	if e != nil {
		return e
	}
	defer s.close()
	raw, e := s.read(fmt.Sprintf("request-%08d.json", seq), 47999)
	var b Bundle
	if e != nil || strict(raw, &b) != nil {
		return ErrInvalid
	}
	f, e := readInventoryBundleRootV3(b, s)
	if e != nil || f.Role != role || b.Directory != path || f.RunID != runID || f.ConfigSHA256 != q.ApprovalSHA256 || f.Grant != ApplicationGrantDigest(q.Configuration.Grant) || !inventoryPinsV3(f.Pins, q.Inventory.Pins) {
		return ErrInvalid
	}
	ready, e := inventoryReadReadyV3(s)
	if e != nil {
		return e
	}
	idx := 0
	if role == "reviewer" {
		idx = 1
	}
	if ready[0].ReviewMode != q.Inventory.ReviewMode || ready[idx].LaunchID != f.LaunchID || ready[idx].PromptSHA256 != f.PromptSHA256 {
		return ErrInvalid
	}
	// Publish invalid semantic bytes too: physical accounting precedes refusal.
	return s.write(fmt.Sprintf("reply-%08d.json", seq), payload)
}

// Legacy retains its exact reader. V3 alone observes a bounded physical prefix
// before classifying a too-large file, so overflow errors are never free bytes.
func readPhysicalReplyRoot(s *store, name string, inventory bool) ([]byte, error) {
	if !inventory {
		return s.read(name, 65537)
	}
	f, e := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); e != nil {
		if errors.Is(e, syscall.EWOULDBLOCK) {
			return nil, os.ErrNotExist
		}
		return nil, ErrInvalid
	}
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 {
		return nil, ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, 65537))
	if st.Size() > 65536 || len(raw) > 65536 {
		return raw, errors.Join(e, ErrInvalid)
	}
	return raw, e
}

// The legacy activation packet's physical path remains compatible with the
// unchanged foreground lifecycle scripts; V3 never assigns cohort semantics.
func InventoryMailboxPathV3(store, runID string) string {
	if !digestValue(runID) {
		return ""
	}
	return filepath.Join(store, "units", runID[7:], "cohort-mailbox")
}
