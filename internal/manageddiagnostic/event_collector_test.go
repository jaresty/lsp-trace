package manageddiagnostic

import (
	"sync"
	"testing"
	"time"
)

func TestEventCollectorBoundedTerminalImmutable(t *testing.T) {
	n := time.Unix(0, 0)
	clock := func() time.Time { n = n.Add(time.Nanosecond); return n }
	c := NewEventCollector(3, clock)
	c.Record(1, 0, false)
	c.Record(2, 0, false)
	c.Record(3, 0, false)
	if !c.Terminal(9) {
		t.Fatal("ASSERT_EVENT_COLLECTOR_RESERVED_TERMINAL")
	}
	c.Record(4, 0, false)
	s := c.Snapshot()
	if len(s.Events) != 3 || s.Events[2].Kind != EventTerminal || s.Omitted != 1 || s.Late != 1 {
		t.Fatalf("ASSERT_EVENT_COLLECTOR_RESERVED_TERMINAL: %+v", s)
	}
	s.Events[0].Code = 99
	if c.Snapshot().Events[0].Code == 99 {
		t.Fatal("ASSERT_EVENT_COLLECTOR_IMMUTABLE_SNAPSHOT")
	}
}

func TestEventCollectorRaceAndFirstTerminal(t *testing.T) {
	c := NewEventCollector(32, nil)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%10 == 0 {
				c.Terminal(uint16(i + 1))
			} else {
				h := c.Begin(uint16(i))
				h.End(uint16(i), int64(i), true)
			}
		}(i)
	}
	wg.Wait()
	s := c.Snapshot()
	terminals := 0
	for i, e := range s.Events {
		if e.Sequence != uint64(i+1) {
			t.Fatal("ASSERT_EVENT_COLLECTOR_MONOTONIC")
		}
		if e.Kind == EventTerminal {
			terminals++
		}
	}
	if terminals != 1 || !s.Closed {
		t.Fatalf("ASSERT_EVENT_COLLECTOR_FIRST_TERMINAL: %+v", s)
	}
}
