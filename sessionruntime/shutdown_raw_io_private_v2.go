package sessionruntime

import (
	"io"
	"sync"

	"lsp-trace/internal/lspwire"
)

// shutdownRawIOPrivateV2 observes only this shutdown exchange. It never retains
// bytes, adds reads, or changes the wire reader's buffering. Snapshots at the
// deadline and after the sole reader joins have deliberately different names.
type shutdownRawIOPrivateV2 struct {
	mu                                                                     sync.Mutex
	calls, bytes, errors                                                   uint64
	bodyReads, bodyBytes, closedBodyReads, decodeAttempts, rejectedDecodes uint64
	started, completed, inRead, overflow                                   bool
}

func shutdownAddPrivateV2(dst *uint64, n uint64, overflow *bool) {
	if ^uint64(0)-*dst < n {
		*dst = ^uint64(0)
		*overflow = true
		return
	}
	*dst += n
}

type shutdownCountingReaderPrivateV2 struct {
	source io.Reader
	stats  *shutdownRawIOPrivateV2
}

func (r shutdownCountingReaderPrivateV2) Read(p []byte) (int, error) {
	s := r.stats
	s.mu.Lock()
	shutdownAddPrivateV2(&s.calls, 1, &s.overflow)
	s.inRead = true
	s.mu.Unlock()
	n, err := r.source.Read(p)
	s.mu.Lock()
	if n > 0 {
		shutdownAddPrivateV2(&s.bytes, uint64(n), &s.overflow)
	}
	if err != nil {
		shutdownAddPrivateV2(&s.errors, 1, &s.overflow)
	}
	s.inRead = false
	s.mu.Unlock()
	return n, err
}

// Existing wire events carry only scalars. A rejected decode implies a complete
// body reached the decoder; a closed body read with no decode remains partial.
func (s *shutdownRawIOPrivateV2) wireEvent(event lspwire.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch event.Stage {
	case lspwire.EventBodyRead:
		shutdownAddPrivateV2(&s.bodyReads, 1, &s.overflow)
		if event.Bytes > 0 {
			shutdownAddPrivateV2(&s.bodyBytes, uint64(event.Bytes), &s.overflow)
		}
		if event.Closed {
			shutdownAddPrivateV2(&s.closedBodyReads, 1, &s.overflow)
		}
	case lspwire.EventDecode:
		shutdownAddPrivateV2(&s.decodeAttempts, 1, &s.overflow)
		if event.Kind == lspwire.KindInvalid {
			shutdownAddPrivateV2(&s.rejectedDecodes, 1, &s.overflow)
		}
	}
}
func (s *shutdownRawIOPrivateV2) start()    { s.mu.Lock(); s.started = true; s.mu.Unlock() }
func (s *shutdownRawIOPrivateV2) complete() { s.mu.Lock(); s.completed = true; s.mu.Unlock() }
func (s *shutdownRawIOPrivateV2) snapshot(f *LifecycleCleanupObservationPrivateV2, final bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if final {
		f.RawReadCalls, f.RawReadBytes, f.RawReadErrors = s.calls, s.bytes, s.errors
		f.BodyReadEvents, f.BodyReadBytes, f.ClosedBodyReadEvents = s.bodyReads, s.bodyBytes, s.closedBodyReads
		f.DecodeAttempts, f.RejectedDecodes = s.decodeAttempts, s.rejectedDecodes
		f.ReaderStarted, f.ReaderCompleted, f.ReaderJoined = s.started, s.completed, s.completed
		f.RawIOFinalAfterJoin = s.completed
	} else {
		f.RawReadCallsAtDeadline, f.RawReadBytesAtDeadline = s.calls, s.bytes
		f.ReaderStartedAtDeadline = s.started
		f.ReaderActiveAtDeadline = s.started && !s.completed
		f.UnderlyingReadActiveAtDeadline = s.inRead
	}
	f.RawCounterOverflow = f.RawCounterOverflow || s.overflow
}

type shutdownCountingWriterPrivateV2 struct {
	source       io.Writer
	calls, bytes uint64
	overflow     bool
}

// Preserve exactly the optional capability that the existing wire Writer uses.
// Advertising Flush for a non-flushing source would itself change observations.
func (w *shutdownCountingWriterPrivateV2) wireWriter() io.Writer {
	if f, ok := w.source.(interface{ Flush() error }); ok {
		return shutdownFlushingWriterPrivateV2{shutdownCountingWriterPrivateV2: w, flusher: f}
	}
	return w
}

type shutdownFlushingWriterPrivateV2 struct {
	*shutdownCountingWriterPrivateV2
	flusher interface{ Flush() error }
}

func (w shutdownFlushingWriterPrivateV2) Flush() error { return w.flusher.Flush() }

func (w *shutdownCountingWriterPrivateV2) Write(p []byte) (int, error) {
	shutdownAddPrivateV2(&w.calls, 1, &w.overflow)
	n, err := w.source.Write(p)
	if n > 0 {
		shutdownAddPrivateV2(&w.bytes, uint64(n), &w.overflow)
	}
	return n, err
}
