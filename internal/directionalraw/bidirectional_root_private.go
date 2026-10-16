package directionalraw

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
)

var ErrBehaviorNotImplementedPrivate = errors.New("adr0011/adr0007 behavior not implemented")
var ErrTerminalReleasePrivate = errors.New("manager terminal release failed")

const DirectionLimitPrivate = uint64(32 << 20)
const BidirectionalLimitPrivate = uint64(64 << 20)

type BidirectionalPlanPrivate struct{ IncomingBytes, OutgoingBytes uint64 }
type DirectionReceiptPrivate struct {
	Direction                                     string
	PreparationBytes, RequestBytes, ResponseBytes uint64
	RequestSHA256, ResponseSHA256                 [32]byte
	AdmittedBytes, ReleasedBytes                  uint64
	RefusedBeforeWrite                            bool
}
type ChildSnapshotPrivate struct {
	Direction                                                         string
	LimitBytes, LiveBytes, AdmittedBytes, ReleasedBytes, ReleaseCount uint64
	Released                                                          bool
}
type RootSnapshotPrivate struct {
	LiveBytes, IncomingLiveBytes, OutgoingLiveBytes, IncomingAdmittedBytes, OutgoingAdmittedBytes, IncomingReleasedBytes, OutgoingReleasedBytes, RootAdmittedBytes, RootReleasedBytes, ReleaseCount uint64
	Released                                                                                                                                                                                        bool
}

type bidirectionalState struct {
	mu                       sync.Mutex
	reservation              Reservation
	limits                   [2]uint64
	requested                [2]uint64
	live, admitted, released [2]uint64
	preparation              [2]uint64
	receipts                 [2]DirectionReceiptPrivate
	releaseCount             uint64
	done                     bool
}
type BidirectionalRootPrivate struct{ state *bidirectionalState }
type ChildPrivate struct {
	state *bidirectionalState
	index int
}
type ChildViewPrivate struct {
	state *bidirectionalState
	index int
}

func directionIndex(d string) (int, bool) {
	if d == "incoming" {
		return 0, true
	}
	if d == "outgoing" {
		return 1, true
	}
	return 0, false
}
func NewBidirectionalRootPrivate(r Reservation, p BidirectionalPlanPrivate) (BidirectionalRootPrivate, error) {
	if r.state == nil {
		return BidirectionalRootPrivate{}, ErrReservationRefused
	}
	s := &bidirectionalState{reservation: r, limits: [2]uint64{DirectionLimitPrivate, DirectionLimitPrivate}, requested: [2]uint64{p.IncomingBytes, p.OutgoingBytes}}
	s.receipts[0].Direction = "incoming"
	s.receipts[1].Direction = "outgoing"
	return BidirectionalRootPrivate{s}, nil
}
func (r BidirectionalRootPrivate) Context(ctx context.Context) (context.Context, error) {
	if r.state == nil {
		return nil, ErrReservationRefused
	}
	return r.state.reservation.Context(ctx), nil
}
func (r BidirectionalRootPrivate) ChildPrivate(d string) (ChildPrivate, error) {
	i, ok := directionIndex(d)
	if r.state == nil || !ok {
		return ChildPrivate{}, ErrReservationRefused
	}
	return ChildPrivate{r.state, i}, nil
}
func (r BidirectionalRootPrivate) PreflightDirectionPrivate(d string) error {
	c, e := r.ChildPrivate(d)
	if e != nil {
		return e
	}
	if c.state.requested[c.index] > DirectionLimitPrivate {
		return ErrReservationRefused
	}
	return nil
}
func (c ChildPrivate) ReservePrivate(n uint64) error {
	if c.state == nil {
		return ErrReservationRefused
	}
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done || n == 0 || n > s.limits[c.index]-s.live[c.index] {
		s.receipts[c.index].RefusedBeforeWrite = true
		return ErrReservationRefused
	}
	s.live[c.index] += n
	s.admitted[c.index] += n
	s.receipts[c.index].AdmittedBytes += n
	return nil
}
func (c ChildPrivate) RecordPreparationPrivate(n uint64) error {
	if c.state == nil {
		return ErrReservationRefused
	}
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if c.state.done {
		return ErrReservationRefused
	}
	c.state.preparation[c.index] += n
	c.state.receipts[c.index].PreparationBytes += n
	return nil
}
func (c ChildPrivate) RecordWirePrivate(req, resp []byte) error {
	if c.state == nil {
		return ErrReservationRefused
	}
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return ErrReservationRefused
	}
	q := &s.receipts[c.index]
	q.RequestBytes += uint64(len(req))
	q.ResponseBytes += uint64(len(resp))
	q.RequestSHA256 = sha256.Sum256(req)
	q.ResponseSHA256 = sha256.Sum256(resp)
	return nil
}

// RecordWireMetadataPrivate records authenticated evidence for bytes retained
// and hashed by Manager. It does not claim that this composition root owns raw
// backing; ReservePrivate separately tracks logical admitted/released bytes.
func (c ChildPrivate) RecordWireMetadataPrivate(requestLength, responseLength uint64, requestSHA256, responseSHA256 [32]byte) error {
	if c.state == nil || requestLength == 0 || responseLength == 0 {
		return ErrReservationRefused
	}
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return ErrReservationRefused
	}
	q := &s.receipts[c.index]
	q.RequestBytes += requestLength
	q.ResponseBytes += responseLength
	q.RequestSHA256 = requestSHA256
	q.ResponseSHA256 = responseSHA256
	return nil
}
func (c ChildPrivate) SnapshotPrivate() (ChildSnapshotPrivate, error) {
	if c.state == nil {
		return ChildSnapshotPrivate{}, ErrReservationRefused
	}
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	d := "incoming"
	if c.index == 1 {
		d = "outgoing"
	}
	return ChildSnapshotPrivate{d, s.limits[c.index], s.live[c.index], s.admitted[c.index], s.released[c.index], s.releaseCount, s.done}, nil
}
func (c ChildPrivate) ViewPrivate() (ChildViewPrivate, error) {
	if c.state == nil {
		return ChildViewPrivate{}, ErrReservationRefused
	}
	return ChildViewPrivate{c.state, c.index}, nil
}
func (v ChildViewPrivate) LiveBytesPrivate() (uint64, bool, error) {
	if v.state == nil {
		return 0, false, ErrReservationRefused
	}
	v.state.mu.Lock()
	defer v.state.mu.Unlock()
	if v.state.done {
		return 0, false, nil
	}
	return v.state.live[v.index], true, nil
}
func (r BidirectionalRootPrivate) ReceiptsPrivate() ([]DirectionReceiptPrivate, error) {
	if r.state == nil {
		return nil, ErrReservationRefused
	}
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	return []DirectionReceiptPrivate{r.state.receipts[0], r.state.receipts[1]}, nil
}
func (r BidirectionalRootPrivate) SnapshotPrivate() (RootSnapshotPrivate, error) {
	if r.state == nil {
		return RootSnapshotPrivate{Released: true, ReleaseCount: 1}, nil
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return RootSnapshotPrivate{LiveBytes: s.live[0] + s.live[1], IncomingLiveBytes: s.live[0], OutgoingLiveBytes: s.live[1], IncomingAdmittedBytes: s.admitted[0], OutgoingAdmittedBytes: s.admitted[1], IncomingReleasedBytes: s.released[0], OutgoingReleasedBytes: s.released[1], RootAdmittedBytes: s.admitted[0] + s.admitted[1], RootReleasedBytes: s.released[0] + s.released[1], ReleaseCount: s.releaseCount, Released: s.done}, nil
}
func (r BidirectionalRootPrivate) Release() error {
	if r.state == nil {
		return nil
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return nil
	}
	if s.reservation.Attached() && !s.reservation.Release() {
		return ErrTerminalReleasePrivate
	}
	s.done = true
	s.releaseCount++
	for i := 0; i < 2; i++ {
		s.released[i] += s.live[i]
		s.receipts[i].ReleasedBytes += s.live[i]
		s.live[i] = 0
	}
	return nil
}
