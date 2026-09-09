package lspwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriterObserverMatchesWireOrderUnderConcurrency(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	var bodyBytes []int64
	firstFrame := make(chan struct{})
	release := make(chan struct{})
	var blocked atomic.Bool
	writer := NewWriterObserved(&out, DefaultLimits(), func(e Event) {
		if e.Stage == EventFrame && blocked.CompareAndSwap(false, true) {
			close(firstFrame)
			<-release
		}
		if e.Stage == EventBodyWrite {
			mu.Lock()
			bodyBytes = append(bodyBytes, e.Bytes)
			mu.Unlock()
		}
	})
	firstDone := make(chan error, 1)
	go func() { firstDone <- writer.Write(Message{ID: json.RawMessage(`1`), Result: json.RawMessage(`null`)}) }()
	<-firstFrame
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- writer.Write(Message{ID: json.RawMessage(`22`), Result: json.RawMessage(`null`)})
	}()
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("second write did not complete")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	firstLen := int64(len(`{"jsonrpc":"2.0","id":1,"result":null}`))
	secondLen := int64(len(`{"jsonrpc":"2.0","id":22,"result":null}`))
	mu.Lock()
	defer mu.Unlock()
	if len(bodyBytes) != 2 || bodyBytes[0] != firstLen || bodyBytes[1] != secondLen {
		t.Fatalf("ASSERT_LSPWIRE_WRITER_EVENT_WIRE_ORDER: %v", bodyBytes)
	}
}

func TestWriterObserverAllowsReentry(t *testing.T) {
	var out bytes.Buffer
	var writer *Writer
	done := make(chan struct{})
	var entered atomic.Bool
	writer = NewWriterObserved(&out, DefaultLimits(), func(e Event) {
		if e.Stage == EventBodyWrite && entered.CompareAndSwap(false, true) {
			if err := writer.Write(Message{ID: json.RawMessage(`2`), Result: json.RawMessage(`null`)}); err != nil {
				t.Errorf("reentrant write: %v", err)
			}
			close(done)
		}
	})
	if err := writer.Write(Message{ID: json.RawMessage(`1`), Result: json.RawMessage(`null`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ASSERT_LSPWIRE_WRITER_REENTRANT_NO_DEADLOCK")
	}
}

func TestReaderDecodeFailuresAreClosed(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"malformed", `{"jsonrpc":`, ErrMalformedJSON},
		{"invalid-shape", `{"jsonrpc":"2.0","id":1}`, ErrInvalidMessage},
		{"wrong-version", `{"jsonrpc":"1.0","id":1,"result":null}`, ErrWrongVersion},
		{"trailing-content", `{"jsonrpc":"2.0","id":1,"result":null}{}`, ErrMalformedJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var decode []Event
			_, err := NewReaderObserved(bytes.NewBufferString(frame(tc.body)), DefaultLimits(), func(e Event) {
				if e.Stage == EventDecode {
					decode = append(decode, e)
				}
			}).Read()
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want=%v", err, tc.want)
			}
			if len(decode) != 1 || !decode[0].Closed || decode[0].Kind != KindInvalid {
				t.Fatalf("ASSERT_LSPWIRE_READER_DECODE_FAILED_CLOSED: %+v", decode)
			}
		})
	}
}

func TestPendingGenerationHistoryBoundedAndEvictionHonest(t *testing.T) {
	p := NewPending(3)
	for generation := uint64(1); generation <= 1000; generation++ {
		key := p.Begin(generation)
		if got := p.Accept(key); got != ResponseAccepted {
			t.Fatalf("accept generation %d: %v", generation, got)
		}
	}
	p.mu.Lock()
	count := len(p.generations)
	p.mu.Unlock()
	if count > 3 {
		t.Fatalf("ASSERT_LSPWIRE_PENDING_GENERATIONS_BOUNDED: %d", count)
	}
	if got := p.Accept(ResponseKey{Generation: 1, ID: 1}); got != ResponseWrongGeneration {
		t.Fatalf("ASSERT_LSPWIRE_PENDING_EVICTED_HISTORICAL_DISPOSITION: %v", got)
	}

	active := NewPending(1)
	first := active.Begin(10)
	second := active.Begin(20)
	active.mu.Lock()
	_, hasFirst := active.generations[first.Generation]
	_, hasSecond := active.generations[second.Generation]
	active.mu.Unlock()
	if !hasFirst || !hasSecond {
		t.Fatal("ASSERT_LSPWIRE_PENDING_ACTIVE_GENERATIONS_PRESERVED")
	}

	concurrent := NewPending(8)
	var wg sync.WaitGroup
	for generation := uint64(1); generation <= 100; generation++ {
		generation := generation
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := concurrent.Begin(generation)
			if got := concurrent.Accept(key); got != ResponseAccepted {
				t.Errorf("concurrent accept generation %d: %v", generation, got)
			}
		}()
	}
	wg.Wait()
	concurrent.mu.Lock()
	count = len(concurrent.generations)
	concurrent.mu.Unlock()
	if count > 8 {
		t.Fatalf("ASSERT_LSPWIRE_PENDING_CONCURRENT_GENERATIONS_BOUNDED: %d", count)
	}
}
