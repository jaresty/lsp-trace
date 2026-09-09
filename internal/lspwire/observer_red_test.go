package lspwire

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

type partialFlushWriter struct {
	bytes.Buffer
	max     int
	flushes int
}

func (w *partialFlushWriter) Write(p []byte) (int, error) {
	if len(p) > w.max {
		p = p[:w.max]
	}
	return w.Buffer.Write(p)
}
func (w *partialFlushWriter) Flush() error { w.flushes++; return nil }

func TestTransportObserverContractRED(t *testing.T) {
	t.Run("closed scalar stages", func(t *testing.T) {
		var events []Event
		body := `{"jsonrpc":"2.0","id":1,"result":{}}`
		if _, err := NewReaderObserved(bytes.NewBufferString(frame(body)), DefaultLimits(), func(e Event) { events = append(events, e) }).Read(); err != nil {
			t.Fatal(err)
		}
		var stages []EventStage
		for _, e := range events {
			stages = append(stages, e.Stage)
		}
		if !reflect.DeepEqual(stages, []EventStage{EventHeaderRead, EventHeaderRead, EventBodyRead, EventDecode}) {
			t.Fatalf("ASSERT_LSPWIRE_OBSERVER_CLOSED_SCALAR_STAGES: %v", stages)
		}
	})
	t.Run("partial bytes and flush", func(t *testing.T) {
		out := &partialFlushWriter{max: 3}
		var events []Event
		msg := Message{JSONRPC: Version, Method: "initialized", Params: json.RawMessage(`{}`)}
		if err := NewWriterObserved(out, DefaultLimits(), func(e Event) { events = append(events, e) }).Write(msg); err != nil {
			t.Fatal(err)
		}
		if out.flushes != 1 || out.String() != frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`) {
			t.Fatalf("ASSERT_LSPWIRE_EXACT_PARTIAL_BYTES_AND_FLUSH: flush=%d wire=%q", out.flushes, out.String())
		}
		if events[len(events)-1].Flush != FlushSucceeded {
			t.Fatalf("ASSERT_LSPWIRE_EXACT_PARTIAL_BYTES_AND_FLUSH: %+v", events)
		}
	})
	t.Run("closed partial body", func(t *testing.T) {
		var got Event
		_, err := NewReaderObserved(bytes.NewBufferString("Content-Length: 9\r\n\r\n{}"), DefaultLimits(), func(e Event) {
			if e.Stage == EventBodyRead {
				got = e
			}
		}).Read()
		if err == nil || got.Bytes != 2 || !got.Closed || err != io.ErrUnexpectedEOF {
			t.Fatalf("ASSERT_LSPWIRE_EXACT_PARTIAL_BYTES_AND_FLUSH: event=%+v err=%v", got, err)
		}
	})
	t.Run("pending lock boundary", func(t *testing.T) {
		var p *Pending
		p = NewPendingObserved(2, func(Event) { _ = p.TombstoneCount() })
		key := p.Begin(1)
		if got := p.Accept(key); got != ResponseAccepted {
			t.Fatalf("ASSERT_LSPWIRE_PENDING_DISPOSITIONS_OUTSIDE_LOCK: %v", got)
		}
	})
}
