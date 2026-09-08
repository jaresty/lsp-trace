package manageddiagnostic

import (
	"sync"
	"time"
)

type EventKind uint8

const (
	EventOperation EventKind = iota + 1
	EventTerminal
)

type Event struct {
	Sequence  uint64
	ElapsedNS int64
	Kind      EventKind
	Code      uint16
	Count     int64
	Flag      bool
}

type EventSnapshot struct {
	Events  []Event
	Omitted uint64
	Late    uint64
	Closed  bool
}

type EventCollector struct {
	mu      sync.Mutex
	start   time.Time
	now     func() time.Time
	cap     int
	next    uint64
	events  []Event
	omitted uint64
	late    uint64
	closed  bool
}

func NewEventCollector(capacity int, now func() time.Time) *EventCollector {
	if capacity < 1 {
		capacity = 1
	}
	if now == nil {
		now = time.Now
	}
	return &EventCollector{start: now(), now: now, cap: capacity}
}

type OperationHandle struct {
	collector *EventCollector
	code      uint16
	once      sync.Once
}

func (c *EventCollector) Begin(code uint16) *OperationHandle {
	if c == nil {
		return &OperationHandle{}
	}
	c.record(Event{Kind: EventOperation, Code: code})
	return &OperationHandle{collector: c, code: code}
}
func (h *OperationHandle) End(code uint16, count int64, flag bool) {
	if h == nil {
		return
	}
	h.once.Do(func() {
		if h.collector != nil {
			h.collector.record(Event{Kind: EventOperation, Code: code, Count: count, Flag: flag})
		}
	})
}
func (c *EventCollector) Record(code uint16, count int64, flag bool) {
	if c != nil {
		c.record(Event{Kind: EventOperation, Code: code, Count: count, Flag: flag})
	}
}
func (c *EventCollector) Terminal(code uint16) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		c.late++
		return false
	}
	c.closed = true
	c.appendLocked(Event{Kind: EventTerminal, Code: code}, true)
	return true
}
func (c *EventCollector) record(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		c.late++
		return
	}
	c.appendLocked(e, false)
}
func (c *EventCollector) appendLocked(e Event, terminal bool) {
	limit := c.cap
	if !terminal {
		limit--
	}
	if len(c.events) >= limit {
		if terminal && len(c.events) == c.cap {
			c.events = c.events[:c.cap-1]
		}
		if !terminal {
			c.omitted++
			return
		}
	}
	c.next++
	e.Sequence = c.next
	e.ElapsedNS = c.now().Sub(c.start).Nanoseconds()
	if e.ElapsedNS < 0 {
		e.ElapsedNS = 0
	}
	c.events = append(c.events, e)
}
func (c *EventCollector) Close(code uint16) EventSnapshot { c.Terminal(code); return c.Snapshot() }
func (c *EventCollector) Snapshot() EventSnapshot {
	if c == nil {
		return EventSnapshot{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return EventSnapshot{Events: append([]Event(nil), c.events...), Omitted: c.omitted, Late: c.late, Closed: c.closed}
}
