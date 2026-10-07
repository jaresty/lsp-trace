// Package adr0011c18 coordinates the repository-private ADR0011 C18 stage
// transaction. Activation is explicit; observers only project accepted state.
package adr0011c18

import (
	"context"
	"sync"
)

// Point identifies either a coarse execution boundary or an accepted C18 stage.
type Point uint8

const (
	PointRuntimeEntered Point = iota + 1
	PointRuntimeHandoff
	PointRuntimeReturned
	PointAcquisitionEntered
	PointMethodResultHandoff
	PointMethodResultEntered
	PointMethodResultReturned
	PointAcquisitionReturned

	PointPreflight
	PointDeadline
	PointHeaderFrame
	PointCumulativeWire
	PointMessageCount
	PointDecodedMessage
	PointRawResult
	PointSourceBuffer
	PointRuntimePrefixSealed
	PointObjectEvent
	PointRetention
	PointReadback
	PointTerminalCustody
)

// Outcome identifies the path that resolved terminal custody.
type Outcome uint8

const (
	OutcomeSuccess Outcome = iota + 1
	OutcomeError
	OutcomePanic
)

// Event is emitted only after the corresponding coordinator transition commits.
type Event struct {
	Point       Point
	Transaction string
	Outcome     Outcome
}

type Observer func(Point)
type EventObserver func(Event)

type observerKey struct{}
type eventObserverKey struct{}
type receiptKey struct{}

type eventObserverState struct {
	mu       sync.Mutex
	observer EventObserver
	disabled bool
}

type ownerKind uint8

const (
	ownerRuntime ownerKind = iota + 1
	ownerAcquisition
)

type runtimeState uint8

const (
	runtimeAwaitPreflight runtimeState = iota
	runtimeAwaitDeadline
	runtimeAwaitHeaderFrame
	runtimeAwaitCumulativeWire
	runtimeAwaitMessageCount
	runtimeAfterMessage
	runtimeAwaitRawResult
	runtimeAwaitSourceBuffer
	runtimeComplete
)

// Receipt is a private monotonic transaction. Acquisition owns its receipt;
// runtime and method-result only advance the same context-carried identity.
type Receipt struct {
	mu sync.Mutex

	owner          ownerKind
	transaction    string
	runtimeEntered bool
	runtimeState   runtimeState
	runtimeSealed  bool
	suffixNext     int
	terminalSealed bool

	observer *eventObserverState
}

var suffixOrder = [...]Point{PointObjectEvent, PointRetention, PointReadback}

func WithObserver(parent context.Context, observer Observer) context.Context {
	if observer == nil {
		return parent
	}
	return context.WithValue(parent, observerKey{}, observer)
}

func Notify(ctx context.Context, point Point) {
	if ctx == nil {
		return
	}
	observer, _ := ctx.Value(observerKey{}).(Observer)
	notifyPoint(observer, point)
}

func WithEventObserver(parent context.Context, observer EventObserver) context.Context {
	if observer == nil {
		return parent
	}
	return context.WithValue(parent, eventObserverKey{}, &eventObserverState{observer: observer})
}

// BeginAcquisition explicitly activates one acquisition-owned C18 transaction.
// Reusing an already active context is incompatible and is rejected.
func BeginAcquisition(parent context.Context, transaction string) (context.Context, *Receipt) {
	if parent == nil || transaction == "" || From(parent) != nil {
		return parent, nil
	}
	receipt := &Receipt{owner: ownerAcquisition, transaction: transaction, observer: observerFrom(parent)}
	return context.WithValue(parent, receiptKey{}, receipt), receipt
}

// BeginRuntime explicitly activates direct private runtime, or enters runtime on
// the acquisition-owned receipt already carried by parent. Runtime entry is once.
func BeginRuntime(parent context.Context) (context.Context, *Receipt) {
	if parent == nil {
		return parent, nil
	}
	if receipt := From(parent); receipt != nil {
		receipt.mu.Lock()
		if receipt.owner != ownerAcquisition || receipt.runtimeEntered || receipt.runtimeSealed || receipt.terminalSealed {
			receipt.mu.Unlock()
			return parent, nil
		}
		receipt.runtimeEntered = true
		receipt.runtimeState = runtimeAwaitPreflight
		receipt.mu.Unlock()
		return parent, receipt
	}
	receipt := &Receipt{owner: ownerRuntime, runtimeEntered: true, runtimeState: runtimeAwaitPreflight, observer: observerFrom(parent)}
	return context.WithValue(parent, receiptKey{}, receipt), receipt
}

// BeginSuffix borrows only an already sealed runtime receipt. It never activates
// a transaction and cannot bypass runtime-prefix verification.
func BeginSuffix(parent context.Context) (context.Context, *Receipt) {
	receipt := From(parent)
	if receipt == nil {
		return parent, nil
	}
	receipt.mu.Lock()
	valid := receipt.runtimeSealed && !receipt.terminalSealed
	receipt.mu.Unlock()
	if !valid {
		return parent, nil
	}
	return parent, receipt
}

func From(ctx context.Context) *Receipt {
	if ctx == nil {
		return nil
	}
	receipt, _ := ctx.Value(receiptKey{}).(*Receipt)
	return receipt
}

// ReachRuntime accepts only the current state. The cumulative/message pair may
// start another explicit wire-message iteration from runtimeAfterMessage.
func (r *Receipt) ReachRuntime(point Point) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	if !r.runtimeEntered || r.runtimeSealed || r.terminalSealed {
		r.mu.Unlock()
		return false
	}
	accepted := false
	switch point {
	case PointPreflight:
		accepted = r.runtimeState == runtimeAwaitPreflight
		if accepted {
			r.runtimeState = runtimeAwaitDeadline
		}
	case PointDeadline:
		accepted = r.runtimeState == runtimeAwaitDeadline
		if accepted {
			r.runtimeState = runtimeAwaitHeaderFrame
		}
	case PointHeaderFrame:
		accepted = r.runtimeState == runtimeAwaitHeaderFrame
		if accepted {
			r.runtimeState = runtimeAwaitCumulativeWire
		}
	case PointCumulativeWire:
		accepted = r.runtimeState == runtimeAwaitCumulativeWire || r.runtimeState == runtimeAfterMessage
		if accepted {
			r.runtimeState = runtimeAwaitMessageCount
		}
	case PointMessageCount:
		accepted = r.runtimeState == runtimeAwaitMessageCount
		if accepted {
			r.runtimeState = runtimeAfterMessage
		}
	case PointDecodedMessage:
		accepted = r.runtimeState == runtimeAfterMessage
		if accepted {
			r.runtimeState = runtimeAwaitRawResult
		}
	case PointRawResult:
		accepted = r.runtimeState == runtimeAwaitRawResult
		if accepted {
			r.runtimeState = runtimeAwaitSourceBuffer
		}
	case PointSourceBuffer:
		accepted = r.runtimeState == runtimeAwaitSourceBuffer
		if accepted {
			r.runtimeState = runtimeComplete
		}
	}
	if !accepted {
		r.mu.Unlock()
		return false
	}
	observer := r.observer
	r.mu.Unlock()
	notifyEvent(observer, Event{Point: point})
	return true
}

func (r *Receipt) SealRuntime() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	if r.runtimeSealed || r.terminalSealed || r.runtimeState != runtimeComplete {
		r.mu.Unlock()
		return false
	}
	r.runtimeSealed = true
	observer := r.observer
	r.mu.Unlock()
	notifyEvent(observer, Event{Point: PointRuntimePrefixSealed})
	return true
}

func (r *Receipt) ReachSuffix(point Point) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	if !r.runtimeSealed || r.terminalSealed || r.suffixNext >= len(suffixOrder) || suffixOrder[r.suffixNext] != point {
		r.mu.Unlock()
		return false
	}
	r.suffixNext++
	observer := r.observer
	r.mu.Unlock()
	notifyEvent(observer, Event{Point: point})
	return true
}

func (r *Receipt) SealTerminal(transaction string, outcome Outcome) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	if r.owner != ownerAcquisition || r.terminalSealed || transaction == "" || transaction != r.transaction || outcome < OutcomeSuccess || outcome > OutcomePanic ||
		(outcome == OutcomeSuccess && (!r.runtimeSealed || r.suffixNext != len(suffixOrder))) {
		r.mu.Unlock()
		return false
	}
	r.terminalSealed = true
	observer := r.observer
	r.mu.Unlock()
	notifyEvent(observer, Event{Point: PointTerminalCustody, Transaction: transaction, Outcome: outcome})
	return true
}

func observerFrom(ctx context.Context) *eventObserverState {
	observer, _ := ctx.Value(eventObserverKey{}).(*eventObserverState)
	return observer
}

func notifyPoint(observer Observer, point Point) {
	if observer == nil {
		return
	}
	func() {
		defer func() { _ = recover() }()
		observer(point)
	}()
}

func notifyEvent(state *eventObserverState, event Event) {
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.disabled {
		return
	}
	func() {
		defer func() {
			if recover() != nil {
				state.disabled = true
			}
		}()
		state.observer(event)
	}()
}

// NotifyEvent is observer-only compatibility. It cannot mutate a receipt.
func NotifyEvent(ctx context.Context, event Event) {
	if ctx == nil {
		return
	}
	notifyEvent(observerFrom(ctx), event)
}
