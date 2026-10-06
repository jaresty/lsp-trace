package lspwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type SuccessorIngressLimits struct {
	HeaderBytes, FrameBytes, ConsumptionBytes, AcquisitionBytes                uint64
	MaxReadBytes, PrefetchBytes, HistoryAcquiredBytes, HistoryOutstandingBytes uint64
	MessageAttempts, CompleteMessageBytes, ResultTokenBytes                    uint64
}

const MaxImmutableHistoryEntries uint64 = 16387

type ImmutableHistoryMetadata struct {
	Identity, Cut         string
	Entries, CutOrdinal   uint64
	Acquired, Outstanding uint64
}
type SuccessorCapturePolicy interface{ ObserveOriginalFrame([]byte) }
type successorResultCapturePolicy interface{ ObserveOriginalResult([]byte) }

type SuccessorOwnedResultCapturePolicy interface {
	ObserveOriginalResultOwned([]byte, SuccessorAllocationLease)
}

type SuccessorOwnedFrameCapturePolicy interface {
	ObserveOriginalFrameOwned([]byte, SuccessorAllocationLease)
}

// SuccessorAllocationOwner is an optional successor-only allocation seam.
// Nil preserves historical behavior. Each accepted lease owns one independent
// backing allocation until Release, which must be idempotent.
type SuccessorAllocationOwner interface {
	ReserveSuccessorAllocation(role SuccessorAllocationRole, capacity uint64) (SuccessorAllocationLease, error)
}
type SuccessorAllocationLease interface{ Release() }
type SuccessorAllocationRole uint8

const (
	SuccessorAllocationPrefetch SuccessorAllocationRole = iota + 1
	SuccessorAllocationHeader
	SuccessorAllocationBody
	SuccessorAllocationConsumeScratch
	SuccessorAllocationResultToken
	SuccessorAllocationOriginalFrame
	SuccessorAllocationDecodedMessage
)

type SuccessorIngressOptions struct {
	Limits               SuccessorIngressLimits
	OwnerID              string
	History              *ImmutableHistoryMetadata
	Capture              SuccessorCapturePolicy
	AllocationOwner      SuccessorAllocationOwner
	GovernServerRequests bool
}

// SuccessorReadObservation reports scalar lengths from the most recent
// successful read without retaining or reconstructing message backing.
type SuccessorReadObservation struct {
	BodyBytes      uint64
	MessageBytes   uint64
	FrameBytes     uint64
	BodyOffset     uint64
	RequestIDStart uint64
	RequestIDEnd   uint64
}

type ingressOutcome uint8

const (
	ingressOutcomeNone ingressOutcome = iota
	ingressOutcomeSuccess
	ingressOutcomeRefusedSealed
	ingressOutcomePreflight
	ingressOutcomeDeadline
	ingressOutcomeCancellation
	ingressOutcomeHeaderLimit
	ingressOutcomeFrameLimit
	ingressOutcomeReserveStorage
	ingressOutcomeAcquisitionLimit
	ingressOutcomeConsumptionLimit
	ingressOutcomeTransportShortEOF
	ingressOutcomeTransport
	ingressOutcomeDecodeValidation
	ingressOutcomeMessageAttempts
)

type ingressCheckpoint uint8

const (
	ingressPreRead ingressCheckpoint = iota
	ingressPostRead
	ingressConsume
	ingressReserve
	ingressHeader
	ingressFrame
	ingressCapture
	ingressDecode
	ingressSeal
)

type knownFailures struct{ Preflight, Deadline, Cancellation error }
type ingressStepInput struct {
	Checkpoint      ingressCheckpoint
	Returned        []byte
	ReadErr         error
	Known           knownFailures
	Consume         int
	HeaderRemaining int
	BodyRemaining   int
	Requested       int
	ReadDirect      bool
	BeforeUnread    int
}
type ingressStepResult struct {
	Outcome     error
	OutcomeKind ingressOutcome
	Consumed    int
	NeedRead    int
}

var (
	errIngressSealed              = errors.New("successor ingress sealed")
	errIngressHeader              = errors.New("successor ingress header limit")
	errIngressFrame               = errors.New("successor ingress frame limit")
	errIngressAcquire             = errors.New("successor ingress acquisition limit")
	errIngressConsume             = errors.New("successor ingress consumption limit")
	errIngressReserve             = errors.New("successor ingress storage reservation")
	errIngressHistory             = errors.New("invalid successor history metadata")
	errIngressMessageAttempts     = errors.New("successor ingress message attempt limit")
	ErrCompleteMessageTooLarge    = errors.New("successor complete decoded message remarshal limit")
	ErrResultTokenTooLarge        = errors.New("successor exact result token limit")
	ErrInvalidResultToken         = errors.New("successor invalid top-level result token")
	ErrSuccessorAllocationRefused = errors.New("successor decoded-message allocation refused")
)

type ingressEventStage uint8

const (
	ingressEventRead ingressEventStage = iota + 1
	ingressEventConsume
	ingressEventAssemble
	ingressEventCapture
	ingressEventDecode
	ingressEventSeal
	ingressEventOutcome
	ingressEventReserve
	ingressEventAllocation
)

type ingressSupport uint64

const (
	ingressSupportLedger ingressSupport = 1 << iota
	ingressSupportPrefetch
	ingressSupportReservation
	ingressSupportAllocation
	ingressSupportCapture
	ingressSupportDecode
	ingressSupportTerminal
	ingressSupportHistoryMetadata
	ingressSupportSyntheticInitial
)

type ingressEvent struct {
	Stage                                             ingressEventStage
	Supported, Present                                ingressSupport
	Requested, ReadN                                  int
	BeforeA, AfterA, BeforeC, AfterC, BeforeV, AfterV uint64
	BeforeUnread, AfterUnread                         int
	Outcome                                           ingressOutcome
	Ordinal                                           uint64
}
type ingressTestRecorder struct {
	limit    int
	overflow bool
	events   []ingressEvent
}

func (r *SuccessorIngressReader) observe(e ingressEvent) {
	if r.recorder == nil {
		return
	}
	e.Ordinal = uint64(len(r.recorder.events) + 1)
	if len(r.recorder.events) >= r.recorder.limit {
		r.recorder.overflow = true
		return
	}
	r.recorder.events = append(r.recorder.events, e)
}

type ingressStorageRole uint8

const (
	ingressStoragePrefetch ingressStorageRole = iota + 1
	ingressStorageScratch
	ingressStorageHeader
	ingressStorageBody
	ingressStorageFrame
)

type ingressStorageRecord struct {
	capacity uint64
	role     ingressStorageRole
	aliases  int
}
type ingressStorageLedger struct {
	budget, live, peak uint64
	records            map[uint64]ingressStorageRecord
}

func newIngressStorageLedger(b uint64) *ingressStorageLedger {
	return &ingressStorageLedger{budget: b, records: map[uint64]ingressStorageRecord{}}
}
func (l *ingressStorageLedger) reserveNew(id, capacity uint64, role ingressStorageRole) bool {
	if _, ok := l.records[id]; ok {
		return false
	}
	if role == ingressStoragePrefetch {
		for _, x := range l.records {
			if x.role == role {
				return false
			}
		}
		if capacity > 4096 {
			return false
		}
	}
	if capacity > l.budget-l.live {
		return false
	}
	l.records[id] = ingressStorageRecord{capacity: capacity, role: role, aliases: 1}
	l.live += capacity
	if l.live > l.peak {
		l.peak = l.live
	}
	return true
}
func (l *ingressStorageLedger) alias(id uint64) bool {
	x, ok := l.records[id]
	if !ok {
		return false
	}
	x.aliases++
	l.records[id] = x
	return true
}
func (l *ingressStorageLedger) reserveGrowth(oldID, newID, newCap uint64, role ingressStorageRole) bool {
	x, ok := l.records[oldID]
	if !ok || role == ingressStoragePrefetch || newCap > l.budget-l.live {
		return false
	}
	l.records[newID] = ingressStorageRecord{capacity: newCap, role: role, aliases: 1}
	l.live += newCap
	if l.live > l.peak {
		l.peak = l.live
	}
	if x.aliases <= 1 {
		delete(l.records, oldID)
		l.live -= x.capacity
	} else {
		x.aliases--
		l.records[oldID] = x
	}
	return true
}
func (l *ingressStorageLedger) releaseAlias(id uint64) bool {
	x, ok := l.records[id]
	if !ok {
		return false
	}
	x.aliases--
	if x.aliases == 0 {
		delete(l.records, id)
		l.live -= x.capacity
	} else {
		l.records[id] = x
	}
	return true
}

type SuccessorIngressReader struct {
	transport                     io.Reader
	options                       SuccessorIngressOptions
	acquired, consumed, validated uint64
	messageAttempts               uint64
	prefetch                      []byte
	sealed                        bool
	terminalOutcome               ingressOutcome
	recorder                      *ingressTestRecorder
	storage                       *ingressStorageLedger
	nextStorageID                 uint64
	prefetchAllocation            SuccessorAllocationLease
	allocationClosed              bool
	lastRead                      SuccessorReadObservation
}

func NewSuccessorIngressReader(rd io.Reader, o SuccessorIngressOptions) (*SuccessorIngressReader, error) {
	if o.Limits.HeaderBytes == 0 || o.Limits.FrameBytes == 0 || o.Limits.ConsumptionBytes == 0 || o.Limits.AcquisitionBytes == 0 || o.Limits.MaxReadBytes == 0 || o.Limits.PrefetchBytes != 4096 {
		return nil, fmt.Errorf("invalid successor limits")
	}
	if h := o.History; h != nil {
		if h.Identity == "" || h.Cut == "" || h.Entries == 0 || h.Entries != h.CutOrdinal || h.Entries > MaxImmutableHistoryEntries ||
			h.Acquired > o.Limits.HistoryAcquiredBytes || h.Outstanding > o.Limits.HistoryOutstandingBytes {
			return nil, errIngressHistory
		}
		c := *h
		o.History = &c
	}
	r := &SuccessorIngressReader{transport: rd, options: o, storage: newIngressStorageLedger(^uint64(0)), nextStorageID: 1}
	if o.AllocationOwner != nil {
		lease, err := o.AllocationOwner.ReserveSuccessorAllocation(SuccessorAllocationPrefetch, 4096)
		if err != nil || lease == nil {
			return nil, errIngressReserve
		}
		r.prefetchAllocation = lease
	}
	if !r.storage.reserveNew(1, 4096, ingressStoragePrefetch) {
		r.Close()
		return nil, errIngressReserve
	}
	r.prefetch = make([]byte, 0, 4096)
	return r, nil
}

// Close releases successor-reader-owned allocation leases. It is idempotent;
// it does not close the underlying transport or release transferred captures.
func (r *SuccessorIngressReader) Close() {
	if r == nil || r.allocationClosed {
		return
	}
	r.allocationClosed = true
	if r.prefetchAllocation != nil {
		r.prefetchAllocation.Release()
		r.prefetchAllocation = nil
	}
}

type ingressReached struct {
	Preflight, Deadline, Cancellation, Header, Frame, Reserve, Acquisition, Consumption, ShortEOF, Transport, Decode, MessageAttempts error
	Success                                                                                                                           bool
}
type ingressSelection struct {
	Kind ingressOutcome
	Err  error
}

func (r *SuccessorIngressReader) selectReached(x ingressReached) ingressSelection {
	for _, c := range []struct {
		k ingressOutcome
		e error
	}{{ingressOutcomePreflight, x.Preflight}, {ingressOutcomeDeadline, x.Deadline}, {ingressOutcomeCancellation, x.Cancellation}, {ingressOutcomeHeaderLimit, x.Header}, {ingressOutcomeFrameLimit, x.Frame}, {ingressOutcomeReserveStorage, x.Reserve}, {ingressOutcomeAcquisitionLimit, x.Acquisition}, {ingressOutcomeConsumptionLimit, x.Consumption}, {ingressOutcomeTransportShortEOF, x.ShortEOF}, {ingressOutcomeTransport, x.Transport}, {ingressOutcomeDecodeValidation, x.Decode}, {ingressOutcomeMessageAttempts, x.MessageAttempts}} {
		if c.e != nil {
			return ingressSelection{c.k, c.e}
		}
	}
	if x.Success {
		return ingressSelection{ingressOutcomeSuccess, nil}
	}
	return ingressSelection{}
}
func (r *SuccessorIngressReader) terminate(x ingressReached) ingressStepResult {
	s := r.selectReached(x)
	if s.Kind == ingressOutcomeSuccess {
		return ingressStepResult{OutcomeKind: s.Kind}
	}
	return r.seal(s.Kind, s.Err)
}
func (r *SuccessorIngressReader) terminateTransport(err error) ingressStepResult {
	if transportOutcome(err, false) == ingressOutcomeTransportShortEOF {
		return r.terminate(ingressReached{ShortEOF: err})
	}
	return r.terminate(ingressReached{Transport: err})
}
func (r *SuccessorIngressReader) reserve(capacity uint64, role ingressStorageRole) bool {
	r.nextStorageID++
	return r.storage.reserveNew(r.nextStorageID, capacity, role)
}

func (r *SuccessorIngressReader) seal(k ingressOutcome, err error) ingressStepResult {
	if !r.sealed {
		r.sealed = true
		r.terminalOutcome = k
		r.observe(ingressEvent{Stage: ingressEventSeal, Supported: ingressSupportTerminal, Present: ingressSupportTerminal, Outcome: k})
	}
	r.observe(ingressEvent{Stage: ingressEventOutcome, Supported: ingressSupportTerminal, Present: ingressSupportTerminal, Outcome: k})
	return ingressStepResult{Outcome: err, OutcomeKind: k}
}
func (r *SuccessorIngressReader) step(in ingressStepInput) ingressStepResult {
	if r.sealed {
		return ingressStepResult{Outcome: errIngressSealed, OutcomeKind: ingressOutcomeRefusedSealed}
	}
	if in.Checkpoint == ingressPreRead {
		if in.Known.Preflight != nil {
			return r.terminate(ingressReached{Preflight: in.Known.Preflight})
		}
		if r.acquired >= r.options.Limits.AcquisitionBytes {
			return r.terminate(ingressReached{Acquisition: errIngressAcquire})
		}
		room := 4096 - len(r.prefetch)
		n := room
		if n > int(r.options.Limits.MaxReadBytes) {
			n = int(r.options.Limits.MaxReadBytes)
		}
		if rem := int(r.options.Limits.AcquisitionBytes - r.acquired); n > rem {
			n = rem
		}
		if rem := int(r.options.Limits.ConsumptionBytes-r.consumed) + 1; n > rem {
			n = rem
		}
		need := in.HeaderRemaining
		if in.BodyRemaining > 0 {
			need = in.BodyRemaining
		}
		if need > 0 && n > need {
			n = need
		}
		return ingressStepResult{NeedRead: n}
	}
	if in.Checkpoint == ingressPostRead {
		beforeA := r.acquired
		beforeU := len(r.prefetch)
		if in.ReadDirect {
			beforeU = in.BeforeUnread
		}
		if uint64(len(in.Returned)) > r.options.Limits.AcquisitionBytes-r.acquired {
			return r.terminate(ingressReached{Acquisition: errIngressAcquire})
		}
		if !in.ReadDirect && len(in.Returned) > cap(r.prefetch)-len(r.prefetch) {
			return r.terminate(ingressReached{Reserve: errIngressReserve})
		}
		r.acquired += uint64(len(in.Returned))
		if !in.ReadDirect {
			r.prefetch = append(r.prefetch, in.Returned...)
		}
		r.observe(ingressEvent{Stage: ingressEventRead, Supported: ingressSupportLedger | ingressSupportPrefetch, Present: ingressSupportLedger | ingressSupportPrefetch, Requested: in.Requested, ReadN: len(in.Returned), BeforeA: beforeA, AfterA: r.acquired, BeforeUnread: beforeU, AfterUnread: len(r.prefetch)})
		if in.Known.Preflight != nil {
			return r.terminate(ingressReached{Preflight: in.Known.Preflight})
		}
		if in.Known.Deadline != nil {
			return r.terminate(ingressReached{Deadline: in.Known.Deadline})
		}
		if in.Known.Cancellation != nil {
			return r.terminate(ingressReached{Cancellation: in.Known.Cancellation})
		}
	}
	if in.Checkpoint == ingressConsume {
		n := in.Consume
		if n > len(r.prefetch) {
			n = len(r.prefetch)
		}
		allowed := int(r.options.Limits.ConsumptionBytes-r.consumed) + 1
		if n > allowed {
			n = allowed
		}
		before := r.consumed
		copy(r.prefetch, r.prefetch[n:])
		r.prefetch = r.prefetch[:len(r.prefetch)-n]
		r.consumed += uint64(n)
		r.observe(ingressEvent{Stage: ingressEventConsume, Supported: ingressSupportLedger | ingressSupportPrefetch, Present: ingressSupportLedger | ingressSupportPrefetch, BeforeC: before, AfterC: r.consumed, AfterUnread: len(r.prefetch)})
		if r.consumed > r.options.Limits.ConsumptionBytes {
			x := r.terminate(ingressReached{Consumption: errIngressConsume})
			x.Consumed = n
			return x
		}
		return ingressStepResult{Consumed: n}
	}
	return ingressStepResult{}
}

func (r *SuccessorIngressReader) readMore(want int) (error, int) {
	p := r.step(ingressStepInput{Checkpoint: ingressPreRead, HeaderRemaining: want})
	if p.Outcome != nil {
		return p.Outcome, 0
	}
	if p.NeedRead <= 0 {
		return errIngressAcquire, 0
	}
	start := len(r.prefetch)
	writable := r.prefetch[start : start+p.NeedRead]
	n, err := r.transport.Read(writable)
	r.prefetch = r.prefetch[:start+n]
	q := r.step(ingressStepInput{Checkpoint: ingressPostRead, Returned: r.prefetch[start : start+n], ReadErr: err, Requested: len(writable), ReadDirect: true, BeforeUnread: start})
	if q.Outcome != nil {
		return q.Outcome, n
	}
	return err, n
}
func (r *SuccessorIngressReader) consumeInto(dst *[]byte, n int) error {
	q := r.step(ingressStepInput{Checkpoint: ingressConsume, Consume: n})
	if q.Consumed > 0 {
		*dst = append(*dst, r.lastConsumed(q.Consumed)...)
	}
	return q.Outcome
}
func (r *SuccessorIngressReader) lastConsumed(n int) []byte { return nil } // unused; callers copy before step
func (r *SuccessorIngressReader) consumeCopy(dst *[]byte, n int) error {
	if n > len(r.prefetch) {
		n = len(r.prefetch)
	}
	var scratchAllocation SuccessorAllocationLease
	if n > 0 && r.options.AllocationOwner != nil {
		lease, err := r.options.AllocationOwner.ReserveSuccessorAllocation(SuccessorAllocationConsumeScratch, uint64(n))
		if err != nil || lease == nil {
			return r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
		}
		scratchAllocation = lease
	}
	if scratchAllocation != nil {
		defer scratchAllocation.Release()
	}
	tmp := append([]byte(nil), r.prefetch[:n]...)
	q := r.step(ingressStepInput{Checkpoint: ingressConsume, Consume: n})
	*dst = append(*dst, tmp[:q.Consumed]...)
	return q.Outcome
}

func (r *SuccessorIngressReader) LastReadObservation() SuccessorReadObservation {
	return r.lastRead
}

func (r *SuccessorIngressReader) ReadFrame() (Message, error) {
	r.lastRead = SuccessorReadObservation{}
	if r.sealed {
		return Message{}, errIngressSealed
	}
	r.messageAttempts++
	if limit := r.options.Limits.MessageAttempts; limit > 0 && r.messageAttempts > limit {
		return Message{}, r.terminate(ingressReached{MessageAttempts: errIngressMessageAttempts}).Outcome
	}
	headerCapacity := int(r.options.Limits.HeaderBytes) + 1
	var headerAllocation SuccessorAllocationLease
	if r.options.AllocationOwner != nil {
		lease, err := r.options.AllocationOwner.ReserveSuccessorAllocation(SuccessorAllocationHeader, uint64(headerCapacity))
		if err != nil || lease == nil {
			return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
		}
		headerAllocation = lease
	}
	if !r.reserve(uint64(headerCapacity), ingressStorageHeader) {
		if headerAllocation != nil {
			headerAllocation.Release()
		}
		return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
	}
	if headerAllocation != nil {
		defer headerAllocation.Release()
	}
	header := make([]byte, 0, headerCapacity)
	for {
		if bytes.Index(r.prefetch, []byte("\r\n\r\n")) < 0 && uint64(len(header)+len(r.prefetch)) > r.options.Limits.HeaderBytes {
			n := int(r.options.Limits.HeaderBytes) + 1 - len(header)
			if err := r.consumeCopy(&header, n); err != nil {
				return Message{}, err
			}
			return Message{}, r.terminate(ingressReached{Header: errIngressHeader}).Outcome
		}
		if end := bytes.Index(r.prefetch, []byte("\r\n\r\n")); end >= 0 {
			end += 4
			if uint64(len(header)+end) > r.options.Limits.HeaderBytes {
				_ = r.consumeCopy(&header, end)
				return Message{}, r.terminate(ingressReached{Header: errIngressHeader}).Outcome
			}
			if err := r.consumeCopy(&header, end); err != nil {
				return Message{}, err
			}
			break
		}
		if uint64(len(header)+len(r.prefetch)) >= r.options.Limits.HeaderBytes {
			if len(r.prefetch) > 0 {
				if err := r.consumeCopy(&header, len(r.prefetch)); err != nil {
					return Message{}, err
				}
			}
			need := int(r.options.Limits.HeaderBytes) - len(header) + 1
			err, _ := r.readMore(need)
			if err != nil {
				return Message{}, r.terminateTransport(err).Outcome
			}
			continue
		}
		if len(r.prefetch) > 0 {
			if err := r.consumeCopy(&header, len(r.prefetch)); err != nil {
				return Message{}, err
			}
		}
		remaining := int(r.options.Limits.HeaderBytes) - len(header) + 1
		err, _ := r.readMore(remaining)
		if err != nil {
			return Message{}, r.terminateTransport(err).Outcome
		}
	}
	length, err := scaffoldContentLength(header)
	if err != nil {
		return Message{}, r.terminate(ingressReached{Decode: err}).Outcome
	}
	if uint64(len(header)+length) > r.options.Limits.FrameBytes {
		return Message{}, r.terminate(ingressReached{Frame: errIngressFrame}).Outcome
	}
	var bodyAllocation SuccessorAllocationLease
	if length > 0 && r.options.AllocationOwner != nil {
		lease, err := r.options.AllocationOwner.ReserveSuccessorAllocation(SuccessorAllocationBody, uint64(length))
		if err != nil || lease == nil {
			return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
		}
		bodyAllocation = lease
	}
	if !r.reserve(uint64(length), ingressStorageBody) {
		if bodyAllocation != nil {
			bodyAllocation.Release()
		}
		return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
	}
	if bodyAllocation != nil {
		defer bodyAllocation.Release()
	}
	body := make([]byte, 0, length)
	var pending error
	for len(body) < length {
		if len(r.prefetch) > 0 {
			n := length - len(body)
			if n > len(r.prefetch) {
				n = len(r.prefetch)
			}
			if e := r.consumeCopy(&body, n); e != nil {
				return Message{}, e
			}
			if pending != nil && len(body) < length {
				break
			}
			continue
		}
		e, _ := r.readMore(length - len(body))
		if e != nil {
			pending = e
			if len(r.prefetch) == 0 {
				break
			}
		}
	}
	if len(body) < length {
		if pending == nil {
			pending = io.ErrUnexpectedEOF
		}
		return Message{}, r.terminateTransport(pending).Outcome
	}
	if pending != nil && !errors.Is(pending, io.EOF) {
		return Message{}, r.terminate(ingressReached{Transport: pending}).Outcome
	}
	frameCapacity := len(header) + len(body)
	var frameAllocation SuccessorAllocationLease
	ownedFrameCapture, ownsFrame := r.options.Capture.(SuccessorOwnedFrameCapturePolicy)
	if ownsFrame && r.options.AllocationOwner != nil {
		lease, reserveErr := r.options.AllocationOwner.ReserveSuccessorAllocation(SuccessorAllocationOriginalFrame, uint64(frameCapacity))
		if reserveErr != nil || lease == nil {
			return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
		}
		frameAllocation = lease
	}
	if !r.reserve(uint64(frameCapacity), ingressStorageFrame) {
		if frameAllocation != nil {
			frameAllocation.Release()
		}
		return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
	}
	frame := make([]byte, 0, frameCapacity)
	frame = append(frame, header...)
	frame = append(frame, body...)
	r.observe(ingressEvent{Stage: ingressEventAssemble, Supported: ingressSupportAllocation, Present: ingressSupportAllocation})
	if frameAllocation != nil {
		ownedFrameCapture.ObserveOriginalFrameOwned(frame, frameAllocation)
		frameAllocation = nil
	} else if r.options.Capture != nil {
		r.options.Capture.ObserveOriginalFrame(frame)
	}
	r.observe(ingressEvent{Stage: ingressEventCapture, Supported: ingressSupportCapture, Present: ingressSupportCapture})
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var m Message
	if e := dec.Decode(&m); e != nil {
		return Message{}, r.decodeFailure(fmt.Errorf("%w: %v", ErrMalformedJSON, e))
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return Message{}, r.decodeFailure(ErrMalformedJSON)
	}
	if m.JSONRPC != Version {
		return Message{}, r.decodeFailure(fmt.Errorf("%w: %q", ErrWrongVersion, m.JSONRPC))
	}
	kind := m.Kind()
	if kind == KindInvalid {
		return Message{}, r.decodeFailure(ErrInvalidMessage)
	}
	requestIDStart, requestIDEnd := 0, 0
	if kind == KindRequest && r.options.GovernServerRequests {
		var requestErr error
		requestIDStart, requestIDEnd, requestErr = topLevelRequestIDSpan(body)
		if requestErr != nil {
			return Message{}, r.decodeFailure(requestErr)
		}
	}
	messageBytes := successorMessageEncodedSize(m)
	if limit := r.options.Limits.CompleteMessageBytes; limit > 0 && messageBytes > limit {
		return Message{}, r.decodeFailure(ErrCompleteMessageTooLarge)
	}
	if limit := r.options.Limits.ResultTokenBytes; limit > 0 && m.Kind() == KindSuccessResponse {
		start, end, e := topLevelResultSpan(body)
		if e != nil {
			return Message{}, r.decodeFailure(e)
		}
		if uint64(end-start) > limit {
			return Message{}, r.decodeFailure(ErrResultTokenTooLarge)
		}
		if capture, ok := r.options.Capture.(SuccessorOwnedResultCapturePolicy); ok && end > start && r.options.AllocationOwner != nil {
			lease, reserveErr := r.options.AllocationOwner.ReserveSuccessorAllocation(SuccessorAllocationResultToken, uint64(end-start))
			if reserveErr != nil || lease == nil {
				return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
			}
			capture.ObserveOriginalResultOwned(append([]byte(nil), body[start:end]...), lease)
		} else if capture, ok := r.options.Capture.(successorResultCapturePolicy); ok {
			capture.ObserveOriginalResult(append([]byte(nil), body[start:end]...))
		}
	}
	before := r.validated
	r.validated += uint64(len(frame))
	r.observe(ingressEvent{Stage: ingressEventDecode, Supported: ingressSupportDecode | ingressSupportLedger, Present: ingressSupportDecode | ingressSupportLedger, BeforeV: before, AfterV: r.validated})
	r.lastRead = SuccessorReadObservation{BodyBytes: uint64(len(body)), MessageBytes: messageBytes, FrameBytes: uint64(len(frame)), BodyOffset: uint64(len(header)), RequestIDStart: uint64(requestIDStart), RequestIDEnd: uint64(requestIDEnd)}
	r.observe(ingressEvent{Stage: ingressEventOutcome, Supported: ingressSupportTerminal, Present: ingressSupportTerminal, Outcome: ingressOutcomeSuccess})
	return m, nil
}

// SuccessorMessageRetainedCapacity reports the exact logical bytes retained by
// a project-owned decoded message clone. Decoder-internal transient storage is
// deliberately outside this ownership boundary.
func SuccessorMessageRetainedCapacity(m Message) uint64 {
	capacity := uint64(len(m.JSONRPC) + len(m.ID) + len(m.Method) + len(m.Params) + len(m.Result))
	if m.Error != nil {
		capacity += uint64(len(m.Error.Message) + len(m.Error.Data))
	}
	return capacity
}

// CloneSuccessorMessageOwned converts one already-decoded accepted message
// from opaque decoder storage into exact project-owned backing. The complete
// retained byte capacity is reserved before any clone allocation.
func CloneSuccessorMessageOwned(m Message, owner SuccessorAllocationOwner) (Message, SuccessorAllocationLease, error) {
	if owner == nil {
		return Message{}, nil, ErrSuccessorAllocationRefused
	}
	capacity := SuccessorMessageRetainedCapacity(m)
	lease, err := owner.ReserveSuccessorAllocation(SuccessorAllocationDecodedMessage, capacity)
	if err != nil || lease == nil {
		return Message{}, nil, ErrSuccessorAllocationRefused
	}
	cloneRaw := func(src json.RawMessage) json.RawMessage {
		if src == nil {
			return nil
		}
		dst := make(json.RawMessage, len(src))
		copy(dst, src)
		return dst
	}
	clone := Message{
		JSONRPC: strings.Clone(m.JSONRPC),
		ID:      cloneRaw(m.ID),
		Method:  strings.Clone(m.Method),
		Params:  cloneRaw(m.Params),
		Result:  cloneRaw(m.Result),
	}
	if m.Error != nil {
		clone.Error = &RPCError{Code: m.Error.Code, Message: strings.Clone(m.Error.Message), Data: cloneRaw(m.Error.Data)}
	}
	return clone, lease, nil
}

func successorMessageEncodedSize(m Message) uint64 {
	size := uint64(len(`{"jsonrpc":`)) + jsonStringEncodedSize(m.JSONRPC)
	if len(m.ID) != 0 {
		size += uint64(len(`,"id":`)) + rawMessageEncodedSize(m.ID)
	}
	if m.Method != "" {
		size += uint64(len(`,"method":`)) + jsonStringEncodedSize(m.Method)
	}
	if len(m.Params) != 0 {
		size += uint64(len(`,"params":`)) + rawMessageEncodedSize(m.Params)
	}
	if len(m.Result) != 0 {
		size += uint64(len(`,"result":`)) + rawMessageEncodedSize(m.Result)
	}
	if m.Error != nil {
		size += uint64(len(`,"error":{"code":`)) + decimalIntSize(m.Error.Code)
		size += uint64(len(`,"message":`)) + jsonStringEncodedSize(m.Error.Message)
		if len(m.Error.Data) != 0 {
			size += uint64(len(`,"data":`)) + rawMessageEncodedSize(m.Error.Data)
		}
		size++
	}
	return size + 1
}

func rawMessageEncodedSize(raw []byte) uint64 {
	size := uint64(0)
	inString, escaped := false, false
	for i := 0; i < len(raw); {
		c := raw[i]
		if !inString {
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
				i++
				continue
			}
			size++
			i++
			if c == '"' {
				inString = true
			}
			continue
		}
		if escaped {
			size++
			i++
			escaped = false
			continue
		}
		if c == '\\' {
			size++
			i++
			escaped = true
			continue
		}
		if c == '"' {
			size++
			i++
			inString = false
			continue
		}
		if c == '<' || c == '>' || c == '&' {
			size += 6
			i++
			continue
		}
		r, n := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && n == 1 {
			size += 3
		} else if r == '\u2028' || r == '\u2029' {
			size += 6
		} else {
			size += uint64(n)
		}
		i += n
	}
	return size
}

func decimalIntSize(n int) uint64 {
	if n == 0 {
		return 1
	}
	size := uint64(0)
	if n < 0 {
		size++
	}
	for ; n != 0; n /= 10 {
		size++
	}
	return size
}

func jsonStringEncodedSize(s string) uint64 {
	size := uint64(2)
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			i++
			switch c {
			case '\\', '"', '\b', '\f', '\n', '\r', '\t':
				size += 2
			default:
				if c < 0x20 || c == '<' || c == '>' || c == '&' {
					size += 6
				} else {
					size++
				}
			}
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 {
			size += 3
			i++
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			size += 6
		} else {
			size += uint64(n)
		}
		i += n
	}
	return size
}

func topLevelRequestIDSpan(body []byte) (int, int, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return 0, 0, ErrMalformedJSON
	}
	var seen uint8
	found, start, end := false, 0, 0
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return 0, 0, ErrMalformedJSON
		}
		name, ok := key.(string)
		if !ok {
			return 0, 0, ErrMalformedJSON
		}
		var bit uint8
		switch name {
		case "jsonrpc":
			bit = 1 << 0
		case "id":
			bit = 1 << 1
		case "method":
			bit = 1 << 2
		case "params":
			bit = 1 << 3
		case "result":
			bit = 1 << 4
		case "error":
			bit = 1 << 5
		}
		if bit != 0 && seen&bit != 0 {
			return 0, 0, ErrMalformedJSON
		}
		seen |= bit
		valueStart := int(dec.InputOffset())
		for valueStart < len(body) && (body[valueStart] == ' ' || body[valueStart] == '\t' || body[valueStart] == '\r' || body[valueStart] == '\n' || body[valueStart] == ':') {
			valueStart++
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return 0, 0, ErrMalformedJSON
		}
		if name == "id" {
			found, start, end = true, valueStart, int(dec.InputOffset())
		}
	}
	if _, err := dec.Token(); err != nil {
		return 0, 0, ErrMalformedJSON
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || !found || !validPrivateRequestIDToken(body[start:end]) {
		return 0, 0, ErrInvalidMessage
	}
	return start, end, nil
}

func validPrivateRequestIDToken(token []byte) bool {
	if len(token) >= 2 && token[0] == '"' && token[len(token)-1] == '"' {
		return true
	}
	if len(token) == 0 {
		return false
	}
	i := 0
	if token[0] == '-' {
		i++
		if i == len(token) {
			return false
		}
	}
	if token[i] == '0' {
		return i+1 == len(token)
	}
	if token[i] < '1' || token[i] > '9' {
		return false
	}
	for i++; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return false
		}
	}
	return true
}

func topLevelResultSpan(body []byte) (int, int, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return 0, 0, ErrInvalidResultToken
	}
	found, start, end := false, 0, 0
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return 0, 0, ErrInvalidResultToken
		}
		name, ok := key.(string)
		if !ok {
			return 0, 0, ErrInvalidResultToken
		}
		valueStart := int(dec.InputOffset())
		for valueStart < len(body) && (body[valueStart] == ' ' || body[valueStart] == '\t' || body[valueStart] == '\r' || body[valueStart] == '\n' || body[valueStart] == ':') {
			valueStart++
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return 0, 0, ErrInvalidResultToken
		}
		valueEnd := int(dec.InputOffset())
		if name == "result" {
			if found {
				return 0, 0, ErrInvalidResultToken
			}
			found, start, end = true, valueStart, valueEnd
		}
	}
	if _, err := dec.Token(); err != nil {
		return 0, 0, ErrInvalidResultToken
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || !found || start < 0 || end < start || end > len(body) {
		return 0, 0, ErrInvalidResultToken
	}
	return start, end, nil
}

func (r *SuccessorIngressReader) decodeFailure(err error) error {
	r.observe(ingressEvent{Stage: ingressEventDecode, Supported: ingressSupportDecode, Present: ingressSupportDecode, Outcome: ingressOutcomeDecodeValidation})
	return r.terminate(ingressReached{Decode: err}).Outcome
}
func transportOutcome(err error, short bool) ingressOutcome {
	if short || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return ingressOutcomeTransportShortEOF
	}
	return ingressOutcomeTransport
}

func scaffoldContentLength(header []byte) (int, error) {
	length := -1
	for _, line := range strings.Split(string(header[:len(header)-4]), "\r\n") {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return 0, fmt.Errorf("%w: %q", ErrInvalidContentLength, line)
		}
		if !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue
		}
		if length >= 0 {
			return 0, ErrDuplicateContentLength
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 0 {
			return 0, fmt.Errorf("%w: %q", ErrInvalidContentLength, value)
		}
		length = n
	}
	if length < 0 {
		return 0, ErrMissingContentLength
	}
	return length, nil
}
