package sessionruntime

import (
	"encoding/json"
	"unsafe"

	"lsp-trace/internal/lspwire"
)

// Only frameRecord constructs this metadata from the actual concrete child and
// its admission closure. It is not an exported funding receipt or bypass flag.
type privateDiscoveryFrame struct {
	owner       *privateDiscoveryWindow
	child       *privateDiscoveryChild
	message     lspwire.Message
	observation lspwire.ReadFrameObservation
	raw         []byte
	canonical   uint64
}

func (w *privateDiscoveryWindow) frameRecord(c *privateDiscoveryChild) (*privateDiscoveryFrame, error) {
	w.mu.Lock()
	cap := min(w.limits.FrameBytes, w.remaining.ResponseBytes, w.remaining.WireBytes)
	valid := c != nil && c.window == w && w.child == c
	if !valid || cap == 0 || cap > uint64(1<<24+1<<16) {
		if cap == 0 && w.limits.FrameBytes != 0 && valid {
			field := &w.remaining.ResponseBytes
			if w.remaining.WireBytes == 0 {
				field = &w.remaining.WireBytes
			}
			w.reserveLocked(PrivateDiscoveryFrame, privateDiscoveryCharge{field, 1})
		} else {
			w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryFrame})
		}
		w.mu.Unlock()
		return nil, ErrPrivateDiscovery
	}
	limits := lspwire.Limits{MaxBodyBytes: int64(w.limits.FrameBytes), MaxHeaderBytes: int64(w.limits.HeaderBytes)}
	w.mu.Unlock()
	admit := func(q lspwire.PrivateDiscoveryCharge) error {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.child != c || c.window != w {
			w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryFrame})
			return ErrPrivateDiscovery
		}
		if !w.reserveLocked(PrivateDiscoveryFrame, privateDiscoveryCharge{&w.remaining.Messages, q.Messages}, privateDiscoveryCharge{&w.remaining.RetainedBytes, q.Retained}, privateDiscoveryCharge{&w.remaining.Work, q.Work}) {
			return ErrPrivateDiscovery
		}
		return nil
	}
	m, o, raw, count, err := lspwire.ReadPrivateDiscoveryFrame(w.ctx, c.Stdout(), limits, int64(cap), admit)
	if err != nil {
		return nil, err
	}
	// The channel carries a pointer, not an unfunded duplicate envelope.
	// K includes padding. Reserve before the explicit runtime record object.
	k := uint64(unsafe.Sizeof(privateDiscoveryFrame{}))
	if err := admit(lspwire.PrivateDiscoveryCharge{Retained: k, Work: 3*k + 4*uint64(unsafe.Sizeof((*privateDiscoveryFrame)(nil)))}); err != nil {
		return nil, err
	}
	f := &privateDiscoveryFrame{w, c, m, o, raw, count}
	if w.ctx.Err() != nil {
		return nil, ErrPrivateDiscovery
	}
	return f, nil
}
func (f *privateDiscoveryFrame) owned(w *privateDiscoveryWindow) bool {
	return f != nil && w != nil && f.owner == w && f.child != nil && f.child.window == w && f.child == w.child
}

type privateDiscoverySlots struct {
	owner                    *privateDiscoveryWindow
	notifications, responses []lspwire.Message
	n, r                     int
}

func (w *privateDiscoveryWindow) consumerSlots() (*privateDiscoverySlots, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := w.remaining.Messages
	if n > 4096 {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryAccounting})
		return nil, ErrPrivateDiscovery
	}
	size := uint64(unsafe.Sizeof(lspwire.Message{}))
	// n <=4096 and the compiler layout is small; check rather than assume.
	metadata := uint64(unsafe.Sizeof(privateDiscoverySlots{})) + uint64(unsafe.Sizeof((*privateDiscoveryFrame)(nil)))
	if size == 0 || size > ^uint64(0)/2 || n > (^uint64(0)-metadata)/(2*size) {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryAccounting})
		return nil, ErrPrivateDiscovery
	}
	cost := 2*n*size + metadata
	if !w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, cost}, privateDiscoveryCharge{&w.remaining.Work, cost + 2*uint64(unsafe.Sizeof(privateDiscoverySlots{}))}) {
		return nil, ErrPrivateDiscovery
	}
	return &privateDiscoverySlots{owner: w, notifications: make([]lspwire.Message, int(n)), responses: make([]lspwire.Message, int(n))}, nil
}
func (s *privateDiscoverySlots) store(f *privateDiscoveryFrame, notification bool) ([]lspwire.Message, error) {
	w := s.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	if !f.owned(w) || w.failed || w.ctx.Err() != nil {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryAccounting})
		return nil, ErrPrivateDiscovery
	}
	slots, n := s.responses, &s.r
	if notification {
		slots, n = s.notifications, &s.n
	}
	if *n >= len(slots) {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryAccounting})
		return nil, ErrPrivateDiscovery
	}
	if !w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.Work, 2 * uint64(unsafe.Sizeof(lspwire.Message{}))}) {
		return nil, ErrPrivateDiscovery
	}
	slots[*n] = f.message
	*n++
	return slots[:*n], nil
}
func (w *privateDiscoveryWindow) selectedResult(f *privateDiscoveryFrame) (json.RawMessage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !f.owned(w) {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryAccounting})
		return nil, ErrPrivateDiscovery
	}
	n := uint64(len(f.message.Result))
	if !w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, n}, privateDiscoveryCharge{&w.remaining.Work, 3 * n}) {
		return nil, ErrPrivateDiscovery
	}
	if f.message.Result == nil {
		return nil, nil
	}
	out := make(json.RawMessage, int(n))
	copy(out, f.message.Result)
	return out, nil
}

// ParseUint base-10 semantics, including saturated diagnostic value on overflow.
// JSON raw IDs need not be numbers; syntax failure returns zero, as ParseUint.
func privateDiscoveryID(b []byte) (uint64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	var n uint64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		d := uint64(c - '0')
		if n > (^uint64(0)-d)/10 {
			return ^uint64(0), false
		}
		n = 10*n + d
	}
	return n, true
}
func privateDiscoveryIDEqual(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i, c := range b {
		if c != s[i] {
			return false
		}
	}
	return true
}
