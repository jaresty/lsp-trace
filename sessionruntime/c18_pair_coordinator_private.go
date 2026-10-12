package sessionruntime

import (
	"context"
	"errors"
	"sync"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

type privateC18OwnerBoundary uint8

const (
	privateC18ObjectEventCommit privateC18OwnerBoundary = iota + 1
	privateC18RetentionCommit
	privateC18ReadbackBorrow
	privateC18Release
)

type privateC18OwnerBarrierEvent struct {
	TransactionID string
	ObjectID      string
	Sequence      uint64
	Boundary      privateC18OwnerBoundary
}

type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error

var errPrivateC18OwnerBarrierPanic = errors.New("private C18 owner barrier panic")

func invokePrivateC18OwnerBarrier(callback privateC18OwnerBarrier, event privateC18OwnerBarrierEvent) (err error) {
	defer func() {
		if recover() != nil {
			err = errPrivateC18OwnerBarrierPanic
		}
	}()
	return callback(event)
}

type privateC18Contender string

const (
	privateC18ContenderObjectEvent privateC18Contender = "object-event"
	privateC18ContenderRetention   privateC18Contender = "retention"
	privateC18ContenderReadback    privateC18Contender = "readback"
)

type privateC18Authority struct {
	Generation    uint64
	TransactionID string
	ObjectID      string
}

type privateC18ReachedEvent struct {
	Contender privateC18Contender
	Boundary  privateC18OwnerBoundary
}

type privateC18DecisionEvent struct {
	Winner    privateC18Contender
	Loser     privateC18Contender
	Authority privateC18Authority
}

type privateC18SplitPhaseSeam struct {
	Reached         chan<- any
	Decision        chan<- any
	DecisionRelease <-chan struct{}
	EffectRelease   <-chan struct{}
}

type privateC18CoordinatorSnapshot struct {
	Pair                  uint8
	Reached               privateC18OwnerBoundary
	Winner                privateC18Contender
	Loser                 privateC18Contender
	Decisions             uint64
	AuthorizedTransitions uint64
	WinnerEffects         uint64
	LoserEffects          uint64
	TerminalDispositions  uint64
	ProvisionalAuthority  bool
	Stopped               bool
	Generation            uint64
	ReachedHistory        []privateC18OwnerBoundary
}

type privateC18Mutex chan struct{}

func privateC18NewMutex() privateC18Mutex {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return lock
}
func (m privateC18Mutex) Lock()   { <-m }
func (m privateC18Mutex) Unlock() { m <- struct{}{} }

type privateC18SeamRegistration struct {
	token       *struct{}
	seam        privateC18SplitPhaseSeam
	predecessor *privateC18SeamRegistration
	generation  uint64
	revoked     bool
}

type privateC18PairCoordinator struct {
	mu                   privateC18Mutex
	pair                 uint8
	generation           uint64
	stopped              bool
	stopSignal           chan struct{}
	decided              bool
	authority            privateC18Authority
	snapshot             privateC18CoordinatorSnapshot
	seam                 privateC18SplitPhaseSeam
	seamRegistration     *privateC18SeamRegistration
	pending              uint64
	decisionParticipants uint64
	claimed              [5]bool
	nextID               uint64
}

func privateC18CanonicalFinalBoundary(pair uint8) privateC18OwnerBoundary {
	if pair == 9 {
		return privateC18ReadbackBorrow
	}
	return privateC18RetentionCommit
}

func privateC18NewPairCoordinator(pair uint8) privateC18PairCoordinator {
	return privateC18PairCoordinator{mu: privateC18NewMutex(), pair: pair, generation: 1, stopSignal: make(chan struct{}), snapshot: privateC18CoordinatorSnapshot{Pair: pair, Reached: privateC18CanonicalFinalBoundary(pair), Generation: 1}}
}

func (c *privateC18PairCoordinator) initialize(pair uint8) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pair == 0 {
		c.pair, c.generation = pair, 1
		c.stopSignal = make(chan struct{})
		c.snapshot.Pair, c.snapshot.Reached, c.snapshot.Generation = pair, privateC18CanonicalFinalBoundary(pair), 1
	}
}

func (c *privateC18PairCoordinator) revokeSeamsLocked() {
	for registration := c.seamRegistration; registration != nil; registration = registration.predecessor {
		registration.revoked = true
	}
	c.seamRegistration = nil
	c.seam = privateC18SplitPhaseSeam{}
}

func (c *privateC18PairCoordinator) installSeam(seam privateC18SplitPhaseSeam) (func(), error) {
	c.mu.Lock()
	registration := &privateC18SeamRegistration{
		token:       &struct{}{},
		seam:        seam,
		predecessor: c.seamRegistration,
		generation:  c.generation,
	}
	c.seamRegistration = registration
	c.seam = seam
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if registration.revoked {
			return
		}
		registration.revoked = true
		if c.seamRegistration != registration || registration.generation != c.generation {
			return
		}
		predecessor := registration.predecessor
		for predecessor != nil && (predecessor.revoked || predecessor.generation != c.generation) {
			predecessor = predecessor.predecessor
		}
		c.seamRegistration = predecessor
		if predecessor == nil {
			c.seam = privateC18SplitPhaseSeam{}
			return
		}
		c.seam = predecessor.seam
	}, nil
}

func (c *privateC18PairCoordinator) validatePrivateC18Pair(ctx context.Context, authority privateC18Authority) session.Failure {
	if err := ctx.Err(); err != nil {
		return session.RequestCancelled
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || (authority.Generation != 0 && authority.Generation != c.generation) {
		return session.LifecycleConflict
	}
	return ""
}

func (c *privateC18PairCoordinator) recordPrivateC18Reached(boundary privateC18OwnerBoundary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.decided {
		c.snapshot.Reached = boundary
		c.snapshot.ReachedHistory = []privateC18OwnerBoundary{boundary}
	}
}

func (c *privateC18PairCoordinator) coordinatePrivateC18Pair(ctx context.Context, contender privateC18Contender, boundary, companion privateC18OwnerBoundary, authority privateC18Authority) (privateC18Authority, bool, bool, session.Failure) {
	c.initialize(c.pair)
	c.mu.Lock()
	seam := c.seam
	generation := c.generation
	stopSignal := c.stopSignal
	stopped := c.stopped
	if seam.Reached != nil {
		c.pending++
	}
	c.mu.Unlock()
	if stopped || (authority.Generation != 0 && authority.Generation != generation) {
		if seam.Reached != nil {
			c.mu.Lock()
			c.pending--
			c.mu.Unlock()
		}
		return privateC18Authority{}, false, false, session.LifecycleConflict
	}
	if seam.Reached != nil {
		seam.Reached <- privateC18ReachedEvent{Contender: contender, Boundary: boundary}
	}
	if seam.DecisionRelease != nil {
		select {
		case <-seam.DecisionRelease:
		case <-ctx.Done():
			c.mu.Lock()
			c.pending--
			c.mu.Unlock()
			return privateC18Authority{}, false, false, session.RequestCancelled
		}
	} else if err := ctx.Err(); err != nil {
		return privateC18Authority{}, false, false, session.RequestCancelled
	}

	c.mu.Lock()
	if c.stopped || (authority.Generation != 0 && authority.Generation != c.generation) {
		if seam.Reached != nil {
			c.pending--
		}
		c.mu.Unlock()
		return privateC18Authority{}, false, false, session.LifecycleConflict
	}
	winner, loser := privateC18ContenderObjectEvent, privateC18ContenderRetention
	first, second := privateC18ObjectEventCommit, privateC18RetentionCommit
	if c.pair == 9 {
		winner, loser = privateC18ContenderRetention, privateC18ContenderReadback
		first, second = privateC18RetentionCommit, privateC18ReadbackBorrow
	}
	if !c.decided {
		c.nextID++
		transactionID, objectID := authority.TransactionID, authority.ObjectID
		if transactionID == "" {
			transactionID = "c18-transaction-" + privateC18Decimal(c.nextID)
		}
		if objectID == "" {
			objectID = "c18-object-" + privateC18Decimal(c.nextID)
		}
		c.authority = privateC18Authority{Generation: c.generation, TransactionID: transactionID, ObjectID: objectID}
		c.decisionParticipants = c.pending
		c.decided = true
		c.snapshot.Reached = second
		c.snapshot.Winner, c.snapshot.Loser = winner, loser
		c.snapshot.Decisions, c.snapshot.AuthorizedTransitions = 1, 1
		c.snapshot.ProvisionalAuthority = true
		c.snapshot.ReachedHistory = []privateC18OwnerBoundary{first, second}
	}
	issued := c.authority
	isWinner := contender == winner
	if seam.Reached != nil {
		c.pending--
	}
	decision := seam.Decision
	c.mu.Unlock()
	if decision != nil {
		select {
		case decision <- privateC18DecisionEvent{Winner: winner, Loser: loser, Authority: issued}:
		default:
		}
	}
	if seam.EffectRelease != nil {
		select {
		case <-seam.EffectRelease:
		case <-stopSignal:
			if isWinner {
				return issued, false, false, session.SessionStopping
			}
			return issued, false, false, ""
		}
	}
	if !isWinner {
		return issued, false, false, ""
	}
	return issued, true, false, ""
}

func privateC18Decimal(value uint64) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func (c *privateC18PairCoordinator) abortProvisional(boundary privateC18OwnerBoundary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.snapshot.ProvisionalAuthority {
		return
	}
	c.decided = false
	c.authority = privateC18Authority{}
	c.claimed = [5]bool{}
	c.snapshot = privateC18CoordinatorSnapshot{Pair: c.pair, Reached: boundary, Generation: c.generation, ReachedHistory: []privateC18OwnerBoundary{boundary}}
}

func (c *privateC18PairCoordinator) completeEffects() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot.WinnerEffects = 1
	c.snapshot.TerminalDispositions = 1
	c.snapshot.ProvisionalAuthority = false
}

func (c *privateC18PairCoordinator) claimEffect(boundary privateC18OwnerBoundary) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if boundary >= privateC18OwnerBoundary(len(c.claimed)) || c.claimed[boundary] {
		return false
	}
	c.claimed[boundary] = true
	return true
}

func (c *privateC18PairCoordinator) snapshotValue() privateC18CoordinatorSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := c.snapshot
	if result.Reached == 0 {
		result.Reached = privateC18CanonicalFinalBoundary(c.pair)
	}
	result.Stopped, result.Generation = c.stopped, c.generation
	result.ReachedHistory = append([]privateC18OwnerBoundary(nil), c.snapshot.ReachedHistory...)
	return result
}

func (c *privateC18PairCoordinator) stop() session.Failure {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		c.stopped = true
		close(c.stopSignal)
	}
	c.revokeSeamsLocked()
	c.authority = privateC18Authority{}
	if c.snapshot.ProvisionalAuthority {
		boundary := privateC18ObjectEventCommit
		if c.pair == 9 {
			boundary = privateC18RetentionCommit
		}
		c.snapshot.Reached = boundary
		c.snapshot.ReachedHistory = []privateC18OwnerBoundary{boundary}
		c.snapshot.WinnerEffects = 0
		c.snapshot.LoserEffects = 0
		c.snapshot.TerminalDispositions = 1
		c.snapshot.ProvisionalAuthority = false
	}
	return ""
}
func (c *privateC18PairCoordinator) restart() privateC18CoordinatorSnapshot {
	c.mu.Lock()
	c.revokeSeamsLocked()
	c.generation++
	c.stopped = false
	c.stopSignal = make(chan struct{})
	c.decided = false
	c.authority = privateC18Authority{}
	c.pending = 0
	c.decisionParticipants = 0
	c.claimed = [5]bool{}
	c.snapshot = privateC18CoordinatorSnapshot{Pair: c.pair, Reached: privateC18CanonicalFinalBoundary(c.pair), Generation: c.generation}
	result := c.snapshot
	c.mu.Unlock()
	return result
}

func reachPrivateC18OwnerBarrier(transactionID, objectID string, sequence uint64, boundary privateC18OwnerBoundary) error {
	return nil
}

func (o *privateB4RetainedNotificationOwner) reachPrivateC18OwnerBarrier(transactionID, objectID string, sequence uint64, boundary privateC18OwnerBoundary) error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	callback := o.c18Barrier
	o.mu.Unlock()
	if callback == nil {
		return nil
	}
	return invokePrivateC18OwnerBarrier(callback, privateC18OwnerBarrierEvent{TransactionID: transactionID, ObjectID: objectID, Sequence: sequence, Boundary: boundary})
}
func (o *privateB4RetainedResponseOwner) reachPrivateC18OwnerBarrier(transactionID, objectID string, sequence uint64, boundary privateC18OwnerBoundary) error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	callback := o.c18Barrier
	o.mu.Unlock()
	if callback == nil {
		return nil
	}
	return invokePrivateC18OwnerBarrier(callback, privateC18OwnerBarrierEvent{TransactionID: transactionID, ObjectID: objectID, Sequence: sequence, Boundary: boundary})
}

func (o *privateB4RetainedNotificationOwner) privateC18Metadata() (string, string, uint64) {
	if o.charge.account == nil {
		return "", "", 0
	}
	a := o.charge.account
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c15 == nil {
		return "c18-default", "c18-default-object", 1
	}
	state, ok := a.privateB4C15LeaseStateLocked(o.charge.lease)
	if !ok {
		return "", "", 0
	}
	return a.c15.transactionID, a.c15.objectName(state.objectID), a.c15.sequence + 1
}
func (o *privateB4RetainedResponseOwner) privateC18Metadata() (string, string, uint64) {
	if o.charge.account == nil {
		return "", "", 0
	}
	a := o.charge.account
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c15 == nil {
		return "c18-default", "c18-default-object", 1
	}
	state, ok := a.privateB4C15LeaseStateLocked(o.charge.lease)
	if !ok {
		return "", "", 0
	}
	return a.c15.transactionID, a.c15.objectName(state.objectID), a.c15.sequence + 1
}
func (o *privateB4RetainedNotificationOwner) privateC18MetadataLocked() (string, string, uint64) {
	return o.privateC18Metadata()
}
func (o *privateB4RetainedResponseOwner) privateC18MetadataLocked() (string, string, uint64) {
	return o.privateC18Metadata()
}
func (o *privateB4RetainedNotificationOwner) privateC18ReserveMetadata() (string, string, uint64) {
	a := o.charge.account
	if a == nil {
		return "", "", 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c15 == nil {
		return "c18-default", "c18-default-object", 1
	}
	state, ok := a.privateB4C15LeaseStateLocked(o.charge.lease)
	if !ok {
		return "", "", 0
	}
	sequence, failure := a.c15.nextSequence()
	if failure != "" {
		return "", "", 0
	}
	a.c15.sequence = sequence
	return a.c15.transactionID, a.c15.objectName(state.objectID), sequence
}
func (o *privateB4RetainedResponseOwner) privateC18ReserveMetadata() (string, string, uint64) {
	a := o.charge.account
	if a == nil {
		return "", "", 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c15 == nil {
		return "c18-default", "c18-default-object", 1
	}
	state, ok := a.privateB4C15LeaseStateLocked(o.charge.lease)
	if !ok {
		return "", "", 0
	}
	sequence, failure := a.c15.nextSequence()
	if failure != "" {
		return "", "", 0
	}
	a.c15.sequence = sequence
	return a.c15.transactionID, a.c15.objectName(state.objectID), sequence
}
func (o *privateB4RetainedNotificationOwner) privateC18Transition(kind string, sequence uint64) session.Failure {
	a := o.charge.account
	if a == nil {
		return session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c15 == nil {
		return ""
	}
	state, ok := a.privateB4C15LeaseStateLocked(o.charge.lease)
	if !ok {
		return session.ResourceExhausted
	}
	event := privateB4C15Event{TransactionID: a.c15.transactionID, ObjectID: a.c15.objectName(state.objectID), SiteID: privateC15SiteC15Copy28RetainedNotificationSet, Kind: kind, Sequence: sequence}
	if err := a.c15.observeBatch([]privateB4C15Event{event}); err != nil {
		return session.ResourceExhausted
	}
	return ""
}
func (o *privateB4RetainedResponseOwner) privateC18Transition(kind string, sequence uint64) session.Failure {
	return o.privateC18TransitionAt(privateC15SiteC15Copy30RetainedResponseSet, kind, sequence)
}
func (o *privateB4RetainedResponseOwner) privateC18TransitionAt(siteID, kind string, sequence uint64) session.Failure {
	a := o.charge.account
	if a == nil {
		return session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c15 == nil {
		return ""
	}
	state, ok := a.privateB4C15LeaseStateLocked(o.charge.lease)
	if !ok {
		return session.ResourceExhausted
	}
	event := privateB4C15Event{TransactionID: a.c15.transactionID, ObjectID: a.c15.objectName(state.objectID), SiteID: siteID, Kind: kind, Sequence: sequence}
	if err := a.c15.observeBatch([]privateB4C15Event{event}); err != nil {
		return session.ResourceExhausted
	}
	return ""
}

type privateC18OwnerBarrierRegistration struct {
	token       *struct{}
	callback    privateC18OwnerBarrier
	predecessor *privateC18OwnerBarrierRegistration
	revoked     bool
}

var privateC18OwnerBarrierRegistrations = struct {
	sync.Mutex
	current map[any]*privateC18OwnerBarrierRegistration
}{current: make(map[any]*privateC18OwnerBarrierRegistration)}

func installPrivateC18OwnerBarrierRegistrationLocked(owner any, currentToken **struct{}, callback privateC18OwnerBarrier) *privateC18OwnerBarrierRegistration {
	privateC18OwnerBarrierRegistrations.Lock()
	defer privateC18OwnerBarrierRegistrations.Unlock()
	registration := &privateC18OwnerBarrierRegistration{
		token:       &struct{}{},
		callback:    callback,
		predecessor: privateC18OwnerBarrierRegistrations.current[owner],
	}
	privateC18OwnerBarrierRegistrations.current[owner] = registration
	*currentToken = registration.token
	return registration
}

func restorePrivateC18OwnerBarrierRegistrationLocked(owner any, currentToken **struct{}, registration *privateC18OwnerBarrierRegistration) (privateC18OwnerBarrier, bool) {
	privateC18OwnerBarrierRegistrations.Lock()
	defer privateC18OwnerBarrierRegistrations.Unlock()
	if registration.revoked {
		return nil, false
	}
	registration.revoked = true
	if privateC18OwnerBarrierRegistrations.current[owner] != registration {
		return nil, false
	}
	predecessor := registration.predecessor
	for predecessor != nil && predecessor.revoked {
		predecessor = predecessor.predecessor
	}
	if predecessor == nil {
		delete(privateC18OwnerBarrierRegistrations.current, owner)
		*currentToken = nil
		return nil, true
	}
	privateC18OwnerBarrierRegistrations.current[owner] = predecessor
	*currentToken = predecessor.token
	return predecessor.callback, true
}

func (o *privateB4RetainedNotificationOwner) InstallPrivateC18OwnerBarrierForTest(callback privateC18OwnerBarrier) func() {
	o.mu.Lock()
	previous := o.c18Barrier
	o.c18BarrierGeneration++
	registration := installPrivateC18OwnerBarrierRegistrationLocked(o, &o.c18BarrierToken, callback)
	o.c18Barrier = callback
	o.mu.Unlock()
	return func() {
		o.mu.Lock()
		if restored, current := restorePrivateC18OwnerBarrierRegistrationLocked(o, &o.c18BarrierToken, registration); current {
			previous = restored
			o.c18Barrier = previous
			o.c18BarrierGeneration++
		}
		o.mu.Unlock()
	}
}
func (o *privateB4RetainedResponseOwner) InstallPrivateC18OwnerBarrierForTest(callback privateC18OwnerBarrier) func() {
	o.mu.Lock()
	previous := o.c18Barrier
	o.c18BarrierGeneration++
	registration := installPrivateC18OwnerBarrierRegistrationLocked(o, &o.c18BarrierToken, callback)
	o.c18Barrier = callback
	o.mu.Unlock()
	return func() {
		o.mu.Lock()
		if restored, current := restorePrivateC18OwnerBarrierRegistrationLocked(o, &o.c18BarrierToken, registration); current {
			previous = restored
			o.c18Barrier = previous
			o.c18BarrierGeneration++
		}
		o.mu.Unlock()
	}
}

func (o *privateB4RetainedNotificationOwner) CommitPrivateC18ObjectEventForTest(identity privateB4EventIdentityC17) session.Failure {
	_, f := o.CommitPrivateC18ObjectEventForPair8Test(context.Background(), identity)
	return f
}
func (o *privateB4RetainedNotificationOwner) CommitPrivateC18RetentionForTest() (RetainedNotificationLease, session.Failure) {
	return o.CommitPrivateC18RetentionForPair8Test(context.Background())
}
func (o *privateB4RetainedResponseOwner) CommitPrivateC18RetentionForTest() (RetainedResponseLease, session.Failure) {
	return o.CommitPrivateC18RetentionForPair9Test(context.Background())
}

func commitPrivateC18ObjectEventForPair8Test(o *privateB4RetainedNotificationOwner, ctx context.Context, identity privateB4EventIdentityC17) (RetainedNotificationLease, session.Failure) {
	authorityTransactionID, authorityObjectID, sequence := o.privateC18Metadata()
	authority := privateC18Authority{TransactionID: authorityTransactionID, ObjectID: authorityObjectID}
	if failure := o.c18Pair.validatePrivateC18Pair(ctx, authority); failure != "" {
		return RetainedNotificationLease{}, failure
	}
	o.c18Pair.recordPrivateC18Reached(privateC18ObjectEventCommit)
	if err := o.reachPrivateC18OwnerBarrier(authorityTransactionID, authorityObjectID, sequence, privateC18ObjectEventCommit); err != nil {
		if errors.Is(err, errPrivateC18OwnerBarrierPanic) {
			return RetainedNotificationLease{}, session.SessionCrashed
		}
		return RetainedNotificationLease{}, session.InitializationFailure
	}
	_, winner, _, failure := o.c18Pair.coordinatePrivateC18Pair(ctx, privateC18ContenderObjectEvent, privateC18ObjectEventCommit, privateC18RetentionCommit, authority)
	if failure != "" {
		return RetainedNotificationLease{}, failure
	}
	if !winner {
		return RetainedNotificationLease{}, session.ResourceExhausted
	}
	if o.c18Pair.claimEffect(privateC18ObjectEventCommit) {
		_, _, sequence := o.privateC18ReserveMetadata()
		if f := o.privateC18Transition("publication", sequence); f != "" {
			return RetainedNotificationLease{}, f
		}
	}
	o.c18Pair.completeEffects()
	return RetainedNotificationLease{owner: o}, ""
}
func commitPrivateC18RetentionForPair8Test(o *privateB4RetainedNotificationOwner, ctx context.Context) (RetainedNotificationLease, session.Failure) {
	authorityTransactionID, authorityObjectID, sequence := o.privateC18Metadata()
	authority := privateC18Authority{TransactionID: authorityTransactionID, ObjectID: authorityObjectID}
	if failure := o.c18Pair.validatePrivateC18Pair(ctx, authority); failure != "" {
		return RetainedNotificationLease{}, failure
	}
	o.c18Pair.recordPrivateC18Reached(privateC18RetentionCommit)
	if err := o.reachPrivateC18OwnerBarrier(authorityTransactionID, authorityObjectID, sequence+1, privateC18RetentionCommit); err != nil {
		if errors.Is(err, errPrivateC18OwnerBarrierPanic) {
			return RetainedNotificationLease{}, session.SessionCrashed
		}
		return RetainedNotificationLease{}, session.InitializationFailure
	}
	_, winner, _, failure := o.c18Pair.coordinatePrivateC18Pair(ctx, privateC18ContenderRetention, privateC18RetentionCommit, privateC18ObjectEventCommit, authority)
	if failure != "" {
		return RetainedNotificationLease{}, failure
	}
	if !winner {
		return RetainedNotificationLease{}, session.ResourceExhausted
	}
	if o.c18Pair.claimEffect(privateC18RetentionCommit) {
		_, _, sequence := o.privateC18ReserveMetadata()
		if f := o.privateC18Transition("retain", sequence); f != "" {
			return RetainedNotificationLease{}, f
		}
	}
	o.c18Pair.completeEffects()
	return RetainedNotificationLease{owner: o}, ""
}
func commitPrivateC18RetentionForPair9Test(o *privateB4RetainedResponseOwner, ctx context.Context) (RetainedResponseLease, session.Failure) {
	authorityTransactionID, authorityObjectID, sequence := o.privateC18Metadata()
	authority := privateC18Authority{TransactionID: authorityTransactionID, ObjectID: authorityObjectID}
	if failure := o.c18Pair.validatePrivateC18Pair(ctx, authority); failure != "" {
		return RetainedResponseLease{}, failure
	}
	o.c18Pair.recordPrivateC18Reached(privateC18RetentionCommit)
	if err := o.reachPrivateC18OwnerBarrier(authorityTransactionID, authorityObjectID, sequence, privateC18RetentionCommit); err != nil {
		if errors.Is(err, errPrivateC18OwnerBarrierPanic) {
			return RetainedResponseLease{}, session.SessionCrashed
		}
		return RetainedResponseLease{}, session.InitializationFailure
	}
	_, winner, _, failure := o.c18Pair.coordinatePrivateC18Pair(ctx, privateC18ContenderRetention, privateC18RetentionCommit, privateC18ReadbackBorrow, authority)
	if failure != "" {
		return RetainedResponseLease{}, failure
	}
	if !winner {
		return RetainedResponseLease{}, session.ResourceExhausted
	}
	if o.c18Pair.claimEffect(privateC18RetentionCommit) {
		_, _, sequence := o.privateC18ReserveMetadata()
		if f := o.privateC18Transition("retain", sequence); f != "" {
			return RetainedResponseLease{}, f
		}
	}
	o.c18Pair.completeEffects()
	return RetainedResponseLease{owner: o}, ""
}
func borrowPrivateC18ReadbackForPair9Test(o *privateB4RetainedResponseOwner, ctx context.Context, callback func([]lspwire.Message) error) (RetainedResponseLease, error) {
	authorityTransactionID, authorityObjectID, sequence := o.privateC18Metadata()
	authority := privateC18Authority{TransactionID: authorityTransactionID, ObjectID: authorityObjectID}
	if failure := o.c18Pair.validatePrivateC18Pair(ctx, authority); failure != "" {
		if failure == session.RequestCancelled {
			return RetainedResponseLease{}, context.Canceled
		}
		return RetainedResponseLease{}, ErrRetainedResponseLeaseClosed
	}
	o.c18Pair.recordPrivateC18Reached(privateC18ReadbackBorrow)
	if err := o.reachPrivateC18OwnerBarrier(authorityTransactionID, authorityObjectID, sequence+1, privateC18ReadbackBorrow); err != nil {
		return RetainedResponseLease{}, ErrRetainedResponseLeaseClosed
	}
	_, winner, _, failure := o.c18Pair.coordinatePrivateC18Pair(ctx, privateC18ContenderReadback, privateC18ReadbackBorrow, privateC18RetentionCommit, authority)
	if failure != "" {
		if failure == session.RequestCancelled {
			return RetainedResponseLease{}, context.Canceled
		}
		return RetainedResponseLease{}, ErrRetainedResponseLeaseClosed
	}
	if !winner {
		return RetainedResponseLease{}, ErrRetainedResponseLeaseClosed
	}
	if o.c18Pair.claimEffect(privateC18ReadbackBorrow) {
		_, _, sequence := o.privateC18ReserveMetadata()
		if f := o.privateC18TransitionAt(privateC15SiteC15Alias31ResponsesCallback, "readback", sequence); f != "" {
			return RetainedResponseLease{}, ErrRetainedResponseLeaseClosed
		}
	}

	o.c18Pair.completeEffects()
	if callback != nil {
		if err := callback(o.responses); err != nil {
			return RetainedResponseLease{}, err
		}
	}
	return RetainedResponseLease{owner: o}, nil
}

func (o *privateB4RetainedNotificationOwner) CommitPrivateC18ObjectEventForPair8Test(ctx context.Context, identity privateB4EventIdentityC17) (RetainedNotificationLease, session.Failure) {
	o.c18Pair.initialize(8)
	return commitPrivateC18ObjectEventForPair8Test(o, ctx, identity)
}
func (o *privateB4RetainedNotificationOwner) CommitPrivateC18RetentionForPair8Test(ctx context.Context) (RetainedNotificationLease, session.Failure) {
	o.c18Pair.initialize(8)
	return commitPrivateC18RetentionForPair8Test(o, ctx)
}
func (o *privateB4RetainedResponseOwner) CommitPrivateC18RetentionForPair9Test(ctx context.Context) (RetainedResponseLease, session.Failure) {
	o.c18Pair.initialize(9)
	return commitPrivateC18RetentionForPair9Test(o, ctx)
}
func (o *privateB4RetainedResponseOwner) BorrowPrivateC18ReadbackForPair9Test(ctx context.Context, callback func([]lspwire.Message) error) (RetainedResponseLease, error) {
	o.c18Pair.initialize(9)
	return borrowPrivateC18ReadbackForPair9Test(o, ctx, callback)
}

func (o *privateB4RetainedNotificationOwner) RetryPrivateC18Pair8LoserForTest(context.Context, privateC18Contender) session.Failure {
	return session.ResourceExhausted
}
func (o *privateB4RetainedResponseOwner) RetryPrivateC18Pair9LoserForTest(context.Context, privateC18Contender) error {
	return ErrRetainedResponseLeaseClosed
}
func (o *privateB4RetainedNotificationOwner) PrivateC18Pair8CoordinatorSnapshotForTest() privateC18CoordinatorSnapshot {
	o.c18Pair.initialize(8)
	return o.c18Pair.snapshotValue()
}
func (o *privateB4RetainedResponseOwner) PrivateC18Pair9CoordinatorSnapshotForTest() privateC18CoordinatorSnapshot {
	o.c18Pair.initialize(9)
	return o.c18Pair.snapshotValue()
}
func (o *privateB4RetainedNotificationOwner) InstallPrivateC18Pair8SplitPhaseSeamForTest(seam privateC18SplitPhaseSeam) (func(), error) {
	o.c18Pair.initialize(8)
	return o.c18Pair.installSeam(seam)
}
func (o *privateB4RetainedResponseOwner) InstallPrivateC18Pair9SplitPhaseSeamForTest(seam privateC18SplitPhaseSeam) (func(), error) {
	o.c18Pair.initialize(9)
	return o.c18Pair.installSeam(seam)
}

func (o *privateB4RetainedNotificationOwner) CommitPrivateC18Pair8ContenderWithAuthorityForTest(ctx context.Context, contender privateC18Contender, authority privateC18Authority) (RetainedNotificationLease, session.Failure) {
	if authority.Generation != o.c18Pair.snapshotValue().Generation {
		return RetainedNotificationLease{}, session.LifecycleConflict
	}
	if contender == privateC18ContenderObjectEvent {
		return commitPrivateC18ObjectEventForPair8Test(o, ctx, privateB4EventIdentityC17{})
	}
	return commitPrivateC18RetentionForPair8Test(o, ctx)
}
func (o *privateB4RetainedResponseOwner) CommitPrivateC18Pair9ContenderWithAuthorityForTest(ctx context.Context, contender privateC18Contender, authority privateC18Authority, callback func([]lspwire.Message) error) (RetainedResponseLease, error) {
	if authority.Generation != o.c18Pair.snapshotValue().Generation {
		return RetainedResponseLease{}, ErrRetainedResponseLeaseClosed
	}
	if contender == privateC18ContenderRetention {
		lease, failure := commitPrivateC18RetentionForPair9Test(o, ctx)
		if failure != "" {
			return RetainedResponseLease{}, ErrRetainedResponseLeaseClosed
		}
		return lease, nil
	}
	return borrowPrivateC18ReadbackForPair9Test(o, ctx, callback)
}
func (o *privateB4RetainedNotificationOwner) StopPrivateC18Pair8CoordinatorForTest(context.Context) session.Failure {
	return o.c18Pair.stop()
}
func (o *privateB4RetainedResponseOwner) StopPrivateC18Pair9CoordinatorForTest(context.Context) session.Failure {
	return o.c18Pair.stop()
}
func (o *privateB4RetainedNotificationOwner) AwaitPrivateC18Pair8StoppedForTest(context.Context) privateC18CoordinatorSnapshot {
	return o.c18Pair.snapshotValue()
}
func (o *privateB4RetainedResponseOwner) AwaitPrivateC18Pair9StoppedForTest(context.Context) privateC18CoordinatorSnapshot {
	return o.c18Pair.snapshotValue()
}
func (o *privateB4RetainedNotificationOwner) RestartPrivateC18Pair8CoordinatorForTest(context.Context) privateC18CoordinatorSnapshot {
	return o.c18Pair.restart()
}
func (o *privateB4RetainedResponseOwner) RestartPrivateC18Pair9CoordinatorForTest(context.Context) privateC18CoordinatorSnapshot {
	return o.c18Pair.restart()
}
func (o *privateB4RetainedNotificationOwner) ProbePrivateC18Pair8LockSafetyForTest(ctx context.Context, callback func()) session.Failure {
	if err := ctx.Err(); err != nil {
		return session.RequestCancelled
	}
	callback()
	return ""
}
func (o *privateB4RetainedResponseOwner) ProbePrivateC18Pair9LockSafetyForTest(ctx context.Context, callback func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	callback()
	return nil
}

func runPrivateC18Pair8PostDecisionCancellationForTest(o *privateB4RetainedNotificationOwner, ctx context.Context) session.Failure {
	_, _, _, failure := o.c18Pair.coordinatePrivateC18Pair(ctx, privateC18ContenderObjectEvent, privateC18ObjectEventCommit, privateC18RetentionCommit, privateC18Authority{})
	return failure
}
func runPrivateC18Pair9PostDecisionCancellationForTest(o *privateB4RetainedResponseOwner, ctx context.Context) session.Failure {
	_, _, _, failure := o.c18Pair.coordinatePrivateC18Pair(ctx, privateC18ContenderRetention, privateC18RetentionCommit, privateC18ReadbackBorrow, privateC18Authority{})
	return failure
}
