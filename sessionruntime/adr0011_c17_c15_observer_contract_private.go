package sessionruntime

import (
	"errors"
	"math"

	"lsp-trace/internal/session"
)

// privateB4C15BatchObserver receives one immutable value batch for an entire
// ownership admission. The caller never reuses or mutates the supplied slice.
type privateB4C15BatchObserver func([]privateB4C15Event) error

type privateB4C15Event struct {
	TransactionID string
	ObjectID      string
	SiteID        string
	Kind          string
	Sequence      uint64
	Terminal      bool
}

type privateB4C15Operation string
type privateB4C15Disposition string

type privateB4C15ObjectState struct {
	authority uint64
	siteID    string
	terminal  bool
}

// privateB4C15CustodyContext is the sole C15 authority for one exact ADR0011
// private transaction. Identity is caller-supplied and never synthesized here.
// Its methods are called while the owning byte account or Manager lock is held.
type privateB4C15CustodyContext struct {
	sessionID          string
	generation         uint64
	transactionID      string
	observer           privateB4C15BatchObserver
	sequence           uint64
	nextObjectID       uint64
	objects            map[uint64]privateB4C15ObjectState
	batchOwnershipOnly bool
	frozen             bool
}

type privateB4C15ManagerContext struct {
	generation uint64
	observer   privateB4C15BatchObserver
	contexts   map[string]*privateB4C15CustodyContext
}

func newPrivateB4C15CustodyContext(sessionID string, generation uint64, transactionID string, observer privateB4C15BatchObserver) (*privateB4C15CustodyContext, session.Failure) {
	if observer == nil {
		return nil, ""
	}
	if sessionID == "" || generation == 0 || transactionID == "" {
		return nil, session.ResourceExhausted
	}
	return &privateB4C15CustodyContext{
		sessionID: sessionID, generation: generation, transactionID: transactionID,
		observer: observer, objects: make(map[uint64]privateB4C15ObjectState),
	}, ""
}

func (c *privateB4C15CustodyContext) objectName(objectID uint64) string {
	if objectID == 0 {
		return ""
	}
	// Object identity is scoped by the authoritative transaction carried in the
	// same event; the numeric component is allocated only by this context.
	return privateB4C15ObjectName(objectID)
}

func privateB4C15ObjectName(objectID uint64) string {
	const digits = "0123456789"
	var buf [20]byte
	i := len(buf)
	for objectID > 0 {
		i--
		buf[i] = digits[objectID%10]
		objectID /= 10
	}
	return "c15-object-" + string(buf[i:])
}

func (c *privateB4C15CustodyContext) nextSequence() (uint64, session.Failure) {
	if c == nil || c.frozen || c.sequence == math.MaxUint64 {
		return 0, session.ResourceExhausted
	}
	return c.sequence + 1, ""
}

func (c *privateB4C15CustodyContext) observeBatch(events []privateB4C15Event) (err error) {
	candidate := append([]privateB4C15Event(nil), events...)
	defer func() {
		if recover() != nil {
			err = errors.New("C15 batch observer panic")
		}
	}()
	return c.observer(candidate)
}

func (c *privateB4C15CustodyContext) notify(objectID uint64, siteID string, operation privateB4C15Operation, terminal, refusable bool) session.Failure {
	if c == nil {
		return ""
	}
	sequence, failure := c.nextSequence()
	if failure != "" || objectID == 0 || siteID == "" {
		return session.ResourceExhausted
	}
	event := privateB4C15Event{TransactionID: c.transactionID, ObjectID: c.objectName(objectID), SiteID: siteID, Kind: string(operation), Sequence: sequence, Terminal: terminal}
	if err := c.observeBatch([]privateB4C15Event{event}); err != nil && refusable {
		return session.ResourceExhausted
	}
	c.sequence = sequence
	return ""
}

type privateB4C15Creation struct {
	siteID    string
	operation privateB4C15Operation
}

// privateB4C15MutationPlan is implemented by owner-specific typed plans. prepare
// is pure: it validates the authoritative owner and returns only value events,
// prospective context transitions, and an operation that cannot fail or allocate.
type privateB4C15MutationPlan interface {
	preparePrivateB4C15Mutation(*privateB4C15CustodyContext, uint64, uint64) (privateB4C15PreparedMutation, session.Failure)
}

type privateB4C15PreparedMutation struct {
	events      []privateB4C15Event
	transitions []privateB4C15PreparedObjectTransition
	commit      func()
}

type privateB4C15PreparedObjectTransition struct {
	objectID uint64
	state    privateB4C15ObjectState
}

type privateB4C15CreationPlan struct {
	creations []privateB4C15Creation
	objectIDs []uint64
}

func (p *privateB4C15CreationPlan) preparePrivateB4C15Mutation(c *privateB4C15CustodyContext, sequence, nextObjectID uint64) (privateB4C15PreparedMutation, session.Failure) {
	if p == nil || len(p.objectIDs) < len(p.creations) {
		return privateB4C15PreparedMutation{}, session.ResourceExhausted
	}
	prepared := privateB4C15PreparedMutation{
		events:      make([]privateB4C15Event, len(p.creations)),
		transitions: make([]privateB4C15PreparedObjectTransition, len(p.creations)),
	}
	for i, creation := range p.creations {
		if creation.siteID == "" || creation.operation == "" {
			return privateB4C15PreparedMutation{}, session.ResourceExhausted
		}
		objectID := nextObjectID + uint64(i) + 1
		p.objectIDs[i] = objectID
		prepared.events[i] = privateB4C15Event{TransactionID: c.transactionID, ObjectID: c.objectName(objectID), SiteID: creation.siteID, Kind: string(creation.operation), Sequence: sequence + uint64(i) + 1}
		prepared.transitions[i] = privateB4C15PreparedObjectTransition{objectID: objectID, state: privateB4C15ObjectState{authority: objectID, siteID: creation.siteID}}
	}
	prepared.commit = func() {}
	return prepared, ""
}

// commitPlans validates and prepares every heterogeneous owner plan before the
// first observer admission. Only after all admissions succeed does it commit the
// context, followed by each prospectively infallible owner operation in order.
func (c *privateB4C15CustodyContext) commitPlans(plans ...privateB4C15MutationPlan) session.Failure {
	if c == nil {
		return ""
	}
	if c.frozen {
		return session.ResourceExhausted
	}
	sequence, nextObjectID := c.sequence, c.nextObjectID
	prepared := make([]privateB4C15PreparedMutation, len(plans))
	for i, plan := range plans {
		if plan == nil {
			return session.ResourceExhausted
		}
		mutation, failure := plan.preparePrivateB4C15Mutation(c, sequence, nextObjectID)
		if failure != "" || uint64(len(mutation.events)) > math.MaxUint64-sequence || uint64(len(mutation.transitions)) > math.MaxUint64-nextObjectID || mutation.commit == nil {
			return session.ResourceExhausted
		}
		for _, event := range mutation.events {
			if event.TransactionID != c.transactionID || event.Sequence != sequence+1 || event.ObjectID == "" || event.SiteID == "" || event.Kind == "" {
				return session.ResourceExhausted
			}
			sequence++
		}
		for _, transition := range mutation.transitions {
			if transition.objectID != nextObjectID+1 || transition.state.authority != transition.objectID || transition.state.siteID == "" {
				return session.ResourceExhausted
			}
			nextObjectID++
		}
		prepared[i] = mutation
	}
	events := make([]privateB4C15Event, 0, sequence-c.sequence)
	for _, mutation := range prepared {
		events = append(events, mutation.events...)
	}
	if err := c.observeBatch(events); err != nil {
		return session.ResourceExhausted
	}
	c.sequence, c.nextObjectID = sequence, nextObjectID
	for _, mutation := range prepared {
		for _, transition := range mutation.transitions {
			c.objects[transition.objectID] = transition.state
		}
	}
	for _, mutation := range prepared {
		mutation.commit()
	}
	return ""
}

func (c *privateB4C15CustodyContext) createMany(creations []privateB4C15Creation, objectIDs []uint64) session.Failure {
	plan := &privateB4C15CreationPlan{creations: creations, objectIDs: objectIDs}
	if failure := c.commitPlans(plan); failure != "" {
		clear(objectIDs[:min(len(creations), len(objectIDs))])
		return failure
	}
	return ""
}

func (c *privateB4C15CustodyContext) create(siteID string, operation privateB4C15Operation) (uint64, session.Failure) {
	var objectIDs [1]uint64
	if failure := c.createMany([]privateB4C15Creation{{siteID: siteID, operation: operation}}, objectIDs[:]); failure != "" {
		return 0, failure
	}
	return objectIDs[0], ""
}

func (c *privateB4C15CustodyContext) alias(objectID uint64, siteID string) session.Failure {
	if c == nil {
		return ""
	}
	if c.batchOwnershipOnly && siteID == privateC15SiteC15Alias36OwnedPairCallback {
		return ""
	}
	state, ok := c.objects[objectID]
	if !ok || state.terminal || c.frozen {
		return session.ResourceExhausted
	}
	return c.notify(objectID, siteID, privateB4C15Operation("alias"), false, true)
}

func (c *privateB4C15CustodyContext) transfer(objectID uint64, siteID string) session.Failure {
	if c == nil {
		return ""
	}
	state, ok := c.objects[objectID]
	if !ok || state.terminal || c.frozen {
		return session.ResourceExhausted
	}
	if failure := c.notify(objectID, siteID, privateB4C15Operation("transfer"), false, true); failure != "" {
		return failure
	}
	state.siteID = siteID
	c.objects[objectID] = state
	return ""
}

func (c *privateB4C15CustodyContext) releaseReference(objectID uint64, siteID string, terminal bool) {
	if c == nil || c.frozen {
		return
	}
	state, ok := c.objects[objectID]
	if !ok || state.terminal {
		return
	}
	if c.notify(objectID, siteID, privateB4C15Operation("release"), terminal, false) != "" {
		return
	}
	if terminal {
		state.terminal = true
		c.objects[objectID] = state
	}
}

func (c *privateB4C15CustodyContext) release(objectID uint64, siteID string) {
	c.releaseReference(objectID, siteID, true)
}

func (c *privateB4C15CustodyContext) freeze() {
	if c == nil || c.frozen {
		return
	}
	for objectID, state := range c.objects {
		if !state.terminal {
			c.release(objectID, state.siteID)
		}
	}
	c.frozen = true
}

func (a *privateB4ByteAccountV2) installPrivateB4C15BatchObserverChecked(observer privateB4C15BatchObserver) session.Failure {
	if a == nil {
		return session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if observer == nil {
		a.c15 = nil
		return ""
	}
	if a.c15 == nil {
		a.c15 = &privateB4C15CustodyContext{
			sessionID: "c15-batch-observer", generation: 1, transactionID: "c15-batch-observer",
			observer: privateB4C15BatchObserver(observer), objects: make(map[uint64]privateB4C15ObjectState), batchOwnershipOnly: true,
		}
		a.c15Leases = make(map[privateB4ByteLease]privateB4C15LeaseState)
		return ""
	}
	if a.c15.frozen {
		return session.ResourceExhausted
	}
	a.c15.observer = privateB4C15BatchObserver(observer)
	a.c15.batchOwnershipOnly = true
	return ""
}

func (a *privateB4ByteAccountV2) observePrivateB4C15CustodySite(siteID string) session.Failure {
	if a == nil || siteID == "" {
		return session.ResourceExhausted
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminalRequested || a.backingReleased || (a.c15 != nil && a.c15.frozen) {
		return session.ResourceExhausted
	}
	return ""
}

func (a *privateB4ByteAccountV2) setPrivateC15CustodySequenceForTest(sequence uint64) {
	if a == nil {
		return
	}
	a.mu.Lock()
	if a.c15 != nil {
		a.c15.sequence = sequence
	}
	a.mu.Unlock()
}

func (m *Manager) installPrivateB4C15BatchObserverChecked(sessionID string, generation uint64, observer privateB4C15BatchObserver) session.Failure {
	if m == nil || sessionID == "" || generation == 0 {
		return session.ResourceExhausted
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[sessionID]
	if r == nil || r.record.Generation != generation {
		return session.ResourceExhausted
	}
	if m.privateB4C15Contexts == nil {
		m.privateB4C15Contexts = make(map[string]privateB4C15ManagerContext)
	}
	entry := m.privateB4C15Contexts[sessionID]
	if entry.generation != 0 && entry.generation != generation {
		return session.ResourceExhausted
	}
	entry.generation = generation
	entry.observer = observer
	if entry.contexts == nil {
		entry.contexts = make(map[string]*privateB4C15CustodyContext)
	}
	m.privateB4C15Contexts[sessionID] = entry
	return ""
}

func (m *Manager) privateB4C15ContextFor(sessionID string, generation uint64, transactionID string) (*privateB4C15CustodyContext, session.Failure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.privateB4C15Contexts[sessionID]
	if !ok || entry.generation != generation || entry.observer == nil {
		return nil, ""
	}
	if context := entry.contexts[transactionID]; context != nil {
		return context, ""
	}
	context, failure := newPrivateB4C15CustodyContext(sessionID, generation, transactionID, entry.observer)
	if failure != "" {
		return nil, failure
	}
	entry.contexts[transactionID] = context
	m.privateB4C15Contexts[sessionID] = entry
	return context, ""
}

func (m *Manager) retirePrivateB4C15ContextLocked(sessionID string, generation uint64) {
	entry, ok := m.privateB4C15Contexts[sessionID]
	if !ok || entry.generation != generation {
		return
	}
	for _, context := range entry.contexts {
		context.freeze()
	}
	delete(m.privateB4C15Contexts, sessionID)
}

func installPrivateB4C15BatchObserverForTest(m *Manager, sessionID string, generation uint64, observer privateB4C15BatchObserver) session.Failure {
	if m == nil {
		return session.ResourceExhausted
	}
	return m.installPrivateB4C15BatchObserverChecked(sessionID, generation, observer)
}
