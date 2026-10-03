package lspwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type SuccessorIngressLimits struct {
	HeaderBytes, FrameBytes, ConsumptionBytes, AcquisitionBytes                uint64
	MaxReadBytes, PrefetchBytes, HistoryAcquiredBytes, HistoryOutstandingBytes uint64
}
type ImmutableHistoryMetadata struct {
	Identity, Cut         string
	Acquired, Outstanding uint64
}
type SuccessorCapturePolicy interface{ ObserveOriginalFrame([]byte) }
type SuccessorIngressOptions struct {
	Limits  SuccessorIngressLimits
	OwnerID string
	History *ImmutableHistoryMetadata
	Capture SuccessorCapturePolicy
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
	errIngressSealed  = errors.New("successor ingress sealed")
	errIngressHeader  = errors.New("successor ingress header limit")
	errIngressFrame   = errors.New("successor ingress frame limit")
	errIngressAcquire = errors.New("successor ingress acquisition limit")
	errIngressConsume = errors.New("successor ingress consumption limit")
	errIngressReserve = errors.New("successor ingress storage reservation")
	errIngressHistory = errors.New("invalid successor history metadata")
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
	prefetch                      []byte
	sealed                        bool
	terminalOutcome               ingressOutcome
	recorder                      *ingressTestRecorder
	storage                       *ingressStorageLedger
	nextStorageID                 uint64
}

func NewSuccessorIngressReader(rd io.Reader, o SuccessorIngressOptions) (*SuccessorIngressReader, error) {
	if o.Limits.HeaderBytes == 0 || o.Limits.FrameBytes == 0 || o.Limits.ConsumptionBytes == 0 || o.Limits.AcquisitionBytes == 0 || o.Limits.MaxReadBytes == 0 || o.Limits.PrefetchBytes != 4096 {
		return nil, fmt.Errorf("invalid successor limits")
	}
	if h := o.History; h != nil {
		if h.Identity == "" || h.Cut == "" || h.Acquired > o.Limits.HistoryAcquiredBytes || h.Outstanding > o.Limits.HistoryOutstandingBytes {
			return nil, errIngressHistory
		}
		c := *h
		o.History = &c
	}
	r := &SuccessorIngressReader{transport: rd, options: o, storage: newIngressStorageLedger(^uint64(0)), nextStorageID: 1}
	if !r.storage.reserveNew(1, 4096, ingressStoragePrefetch) {
		return nil, errIngressReserve
	}
	r.prefetch = make([]byte, 0, 4096)
	return r, nil
}

type ingressReached struct {
	Preflight, Deadline, Cancellation, Header, Frame, Reserve, Acquisition, Consumption, ShortEOF, Transport, Decode error
	Success                                                                                                          bool
}
type ingressSelection struct {
	Kind ingressOutcome
	Err  error
}

func (r *SuccessorIngressReader) selectReached(x ingressReached) ingressSelection {
	for _, c := range []struct {
		k ingressOutcome
		e error
	}{{ingressOutcomePreflight, x.Preflight}, {ingressOutcomeDeadline, x.Deadline}, {ingressOutcomeCancellation, x.Cancellation}, {ingressOutcomeHeaderLimit, x.Header}, {ingressOutcomeFrameLimit, x.Frame}, {ingressOutcomeReserveStorage, x.Reserve}, {ingressOutcomeAcquisitionLimit, x.Acquisition}, {ingressOutcomeConsumptionLimit, x.Consumption}, {ingressOutcomeTransportShortEOF, x.ShortEOF}, {ingressOutcomeTransport, x.Transport}, {ingressOutcomeDecodeValidation, x.Decode}} {
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
	tmp := append([]byte(nil), r.prefetch[:n]...)
	q := r.step(ingressStepInput{Checkpoint: ingressConsume, Consume: n})
	*dst = append(*dst, tmp[:q.Consumed]...)
	return q.Outcome
}

func (r *SuccessorIngressReader) ReadFrame() (Message, error) {
	if r.sealed {
		return Message{}, errIngressSealed
	}
	headerCapacity := int(r.options.Limits.HeaderBytes) + 1
	if !r.reserve(uint64(headerCapacity), ingressStorageHeader) {
		return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
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
	if !r.reserve(uint64(length), ingressStorageBody) {
		return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
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
	if !r.reserve(uint64(frameCapacity), ingressStorageFrame) {
		return Message{}, r.terminate(ingressReached{Reserve: errIngressReserve}).Outcome
	}
	frame := make([]byte, 0, frameCapacity)
	frame = append(frame, header...)
	frame = append(frame, body...)
	r.observe(ingressEvent{Stage: ingressEventAssemble, Supported: ingressSupportAllocation, Present: ingressSupportAllocation})
	if r.options.Capture != nil {
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
	if m.Kind() == KindInvalid {
		return Message{}, r.decodeFailure(ErrInvalidMessage)
	}
	before := r.validated
	r.validated += uint64(len(frame))
	r.observe(ingressEvent{Stage: ingressEventDecode, Supported: ingressSupportDecode | ingressSupportLedger, Present: ingressSupportDecode | ingressSupportLedger, BeforeV: before, AfterV: r.validated})
	r.observe(ingressEvent{Stage: ingressEventOutcome, Supported: ingressSupportTerminal, Present: ingressSupportTerminal, Outcome: ingressOutcomeSuccess})
	return m, nil
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
