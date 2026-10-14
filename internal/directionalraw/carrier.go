// Package directionalraw carries private, default-disabled aliases. Manager
// supplies allocation, observation and terminal authority; this package does
// not decide admission, acceptance, or terminal outcomes.
package directionalraw

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

var ErrReservationRefused = errors.New("directional reservation exhausted")

type PreparedSelection struct {
	view        View
	observation Observation
}
type Request struct {
	Method  string
	Ordinal uint64
}
type Reservation struct{ state *state }
type Lease struct{ record *record }
type View struct {
	record     *record
	start, end int
}
type Observation struct {
	Method     string
	Ordinal    uint64
	Length     uint64
	SHA256     [32]byte
	Component  string
	Selected   bool
	RequestID  uint64
	Generation uint64
	Failure    string
}

type contextKey struct{}
type record struct {
	state           *state
	method          string
	id, generation  uint64
	frames          map[string][]byte
	result          []byte
	elements        [][2]int
	bound, complete bool
	failure         string
}
type state struct {
	transactionID string
	mu            sync.Mutex
	records       []*record
	latest        *record
	allocate      func(string, int) ([]byte, error)
	observe       func(Observation)
	terminal      func() bool
	release       func()
	closed        bool
}

func NewReservation(transactionID ...string) Reservation {
	id := ""
	if len(transactionID) == 1 {
		id = transactionID[0]
	}
	return Reservation{state: &state{transactionID: id}}
}
func (r Reservation) IdentityPrivate() string {
	if r.state == nil {
		return ""
	}
	return r.state.transactionID
}
func (r Reservation) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, r)
}
func FromContext(ctx context.Context) (Reservation, bool) {
	r, ok := ctx.Value(contextKey{}).(Reservation)
	return r, ok && r.state != nil
}
func Allowed(method string) bool {
	return method == "textDocument/prepareCallHierarchy" || method == "callHierarchy/incomingCalls" || method == "callHierarchy/outgoingCalls"
}

// Attach is invoked by Manager only after its session/generation/lifecycle preflight.
func (r Reservation) Attach(allocate func(string, int) ([]byte, error), observe func(Observation), terminal func() bool, release func()) bool {
	if r.state == nil {
		return false
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	if s.allocate != nil {
		return true
	}
	s.allocate, s.observe, s.terminal, s.release = allocate, observe, terminal, release
	return true
}
func (r Reservation) Attached() bool {
	if r.state == nil {
		return false
	}
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	return r.state.allocate != nil && !r.state.closed
}
func (r Reservation) Same(other Reservation) bool { return r.state != nil && r.state == other.state }
func (r Reservation) Closed() bool {
	if r.state == nil {
		return true
	}
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	return r.state.closed
}
func (r Reservation) Begin(method string, id, generation uint64) Lease {
	if !Allowed(method) || r.state == nil {
		return Lease{}
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.allocate == nil {
		return Lease{}
	}
	p := &record{state: s, method: method, id: id, generation: generation, frames: make(map[string][]byte)}
	s.records = append(s.records, p)
	return Lease{p}
}

// Allocate returns the sole transport-writable backing BEFORE publication.
// Manager's callback reserves the exact capacity before it performs make.
func (l Lease) Allocate(component string, n int) ([]byte, error) {
	if l.record == nil {
		return nil, errors.New("directional lease unavailable")
	}
	p := l.record
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if component == "response-frame" && p.frames[component] != nil {
		p.frames["external-frame-"+fmt.Sprint(len(p.frames))] = p.frames[component]
		delete(p.frames, component)
	}
	if s.closed || p.complete || p.frames[component] != nil || n <= 0 {
		return nil, errors.New("directional allocation refused")
	}
	b, err := s.allocate(component, n)
	if err != nil {
		return nil, err
	}
	if len(b) != n || cap(b) != n {
		return nil, errors.New("directional backing mismatch")
	}
	p.frames[component] = b
	return b, nil
}
func (l Lease) Frame(component string, n int) bool {
	if l.record == nil {
		return false
	}
	p := l.record
	s := p.state
	s.mu.Lock()
	b := p.frames[component]
	if s.closed || n < 0 || n > len(b) {
		s.mu.Unlock()
		return false
	}
	// Keep full reserved backing for terminal clearing; only the observation
	// commits the actually acquired/emitted prefix length.
	o := Observation{Method: p.method, RequestID: p.id, Generation: p.generation, Component: component, Length: uint64(n), SHA256: sha256.Sum256(b[:n])}
	fn := s.observe
	s.mu.Unlock()
	if fn != nil {
		fn(o)
	}
	return true
}

// Result accepts only an alias into a reserved response-frame backing.
func (l Lease) Result(raw []byte) bool {
	if l.record == nil {
		return false
	}
	p := l.record
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	b := p.frames["response-frame"]
	if len(raw) == 0 || len(b) == 0 {
		return false
	}
	found := false
	for i := 0; i+len(raw) <= len(b); i++ {
		if &b[i] == &raw[0] {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	p.result = raw[:len(raw):len(raw)]
	return true
}

// RefusedPrivate commits proposed, not acquired, frame bytes without retaining
// a frame allocation or performing transport I/O.
func (l Lease) RefusedPrivate(component string, n int, sum [32]byte) {
	if l.record == nil {
		return
	}
	p := l.record
	s := p.state
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	o := Observation{Method: p.method, RequestID: p.id, Generation: p.generation, Component: component + "-reservation-refusal", Length: uint64(n), SHA256: sum}
	fn := s.observe
	s.mu.Unlock()
	if fn != nil {
		fn(o)
	}
}
func (l Lease) Complete(failure string) {
	if l.record == nil {
		return
	}
	s := l.record.state
	s.mu.Lock()
	if !s.closed {
		l.record.complete = true
		l.record.failure = failure
		s.latest = l.record
	}
	s.mu.Unlock()
}
func (l Lease) Decode(target any) error {
	if l.record == nil {
		return errors.New("directional lease unavailable")
	}
	s := l.record.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || l.record.result == nil {
		return errors.New("directional result unavailable")
	}
	d := json.NewDecoder(bytes.NewReader(l.record.result))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing result")
	}
	return nil
}
func (l Lease) Null() bool {
	if l.record == nil {
		return false
	}
	s := l.record.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed && bytes.Equal(bytes.TrimSpace(l.record.result), []byte("null"))
}

// BindElementsPrivate resolves lexical offsets under the lease lock. No raw
// mutable slices or caller-owned offset storage cross the acquisition boundary.
func (l Lease) BindElementsPrivate() error {
	if l.record == nil {
		return errors.New("missing result")
	}
	p := l.record
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || p.bound {
		return errors.New("duplicate binding")
	}
	var spans [][2]int
	if !bytes.Equal(bytes.TrimSpace(p.result), []byte("null")) {
		var err error
		spans, err = Elements(p.result)
		if err != nil {
			return err
		}
	}
	p.elements = spans
	p.bound = true
	return nil
}

type resultReader struct {
	record   *record
	position int
}

func (r *resultReader) Read(dst []byte) (int, error) {
	if r.record == nil {
		return 0, io.EOF
	}
	s := r.record.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || r.position >= len(r.record.result) {
		return 0, io.EOF
	}
	n := copy(dst, r.record.result[r.position:])
	r.position += n
	return n, nil
}

// ReaderPrivate is a lease-checked immutable stream, not a mutable backing view.
// Reads into caller-owned decoder scratch are not retained raw custody copies.
func (l Lease) ReaderPrivate() io.Reader { return &resultReader{record: l.record} }
func Select(ctx context.Context, q Request) (PreparedSelection, bool) {
	r, ok := FromContext(ctx)
	if !ok {
		return PreparedSelection{}, false
	}
	s := r.state
	s.mu.Lock()
	p := s.latest
	if s.closed || p == nil || p.method != q.Method || !p.bound || q.Ordinal >= uint64(len(p.elements)) {
		s.mu.Unlock()
		return PreparedSelection{}, false
	}
	span := p.elements[q.Ordinal]
	v := View{p, span[0], span[1]}
	o := Observation{Method: p.method, Ordinal: q.Ordinal, RequestID: p.id, Generation: p.generation, Component: "element", Selected: true, Length: uint64(span[1] - span[0]), SHA256: sha256.Sum256(p.result[span[0]:span[1]])}
	fn := s.observe
	s.mu.Unlock()
	if fn != nil {
		fn(o)
	}
	return PreparedSelection{v, o}, true
}
func (p PreparedSelection) View() View               { return p.view }
func (p PreparedSelection) Observation() Observation { return p.observation }

// SharesBackingPrivate checks exact reserved-frame aliasing without exposing a
// mutable byte slice or interpreting pointer identity as semantic authority.
func (v View) SharesBackingPrivate() bool {
	if v.record == nil {
		return false
	}
	s := v.record.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	raw := v.record.result[v.start:v.end]
	if len(raw) == 0 {
		return false
	}
	for _, frame := range v.record.frames {
		for i := 0; i+len(raw) <= len(frame); i++ {
			if &raw[0] == &frame[i] {
				return true
			}
		}
	}
	return false
}
func (v View) Len() int {
	if v.record == nil {
		return 0
	}
	s := v.record.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0
	}
	return v.end - v.start
}
func (v View) At(i int) (byte, bool) {
	if v.record == nil {
		return 0, false
	}
	s := v.record.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || i < 0 || i >= v.end-v.start {
		return 0, false
	}
	return v.record.result[v.start+i], true
}
func (v View) Equal(raw []byte) bool {
	if v.record == nil {
		return false
	}
	s := v.record.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed && bytes.Equal(v.record.result[v.start:v.end], raw)
}
func (r Reservation) Release() bool {
	if r.state == nil {
		return false
	}
	s := r.state
	s.mu.Lock()
	fn := s.terminal
	s.mu.Unlock()
	if fn == nil {
		return false
	}
	return fn()
}

// Terminate is called only by the generic Manager owner at its sole terminal.
func (r Reservation) Terminate() bool {
	if r.state == nil {
		return false
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.closed = true
	for _, p := range s.records {
		for k, b := range p.frames {
			clear(b)
			delete(p.frames, k)
		}
		p.result = nil
		p.elements = nil
	}
	if s.release != nil {
		s.release()
	}
	return true
}

// ValueEnd is a zero-copy JSON token locator. It validates syntax but never
// decodes a RawMessage; indices are offsets into the original retained backing.
func ValueEnd(raw []byte, start int) (int, error) {
	i := start
	if i >= len(raw) {
		return 0, errors.New("missing value")
	}
	inString, escape := false, false
	depth := 0
	for ; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inString = false
				if depth == 0 {
					i++
					break
				}
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '{' || c == '[' {
			depth++
			continue
		}
		if c == '}' || c == ']' {
			if depth == 0 {
				break
			}
			depth--
			if depth == 0 {
				i++
				break
			}
			continue
		}
		if depth == 0 && (c == ',' || c == ' ' || c == '\n' || c == '\r' || c == '\t') {
			break
		}
	}
	if i <= start || !json.Valid(raw[start:i]) {
		return 0, errors.New("invalid JSON value")
	}
	return i, nil
}
func skip(raw []byte, i int) int {
	for i < len(raw) && (raw[i] == ' ' || raw[i] == '\n' || raw[i] == '\r' || raw[i] == '\t') {
		i++
	}
	return i
}
func Elements(raw []byte) ([][2]int, error) {
	i := skip(raw, 0)
	if i >= len(raw) || raw[i] != '[' {
		return nil, errors.New("not array")
	}
	i++
	var out [][2]int
	for {
		i = skip(raw, i)
		if i < len(raw) && raw[i] == ']' {
			if skip(raw, i+1) != len(raw) {
				return nil, errors.New("trailing array")
			}
			return out, nil
		}
		start := i
		end, err := ValueEnd(raw, i)
		if err != nil {
			return nil, err
		}
		out = append(out, [2]int{start, end})
		i = skip(raw, end)
		if i >= len(raw) {
			return nil, errors.New("missing array end")
		}
		if raw[i] == ']' {
			continue
		}
		if raw[i] != ',' {
			return nil, errors.New("missing comma")
		}
		i++
		if skip(raw, i) < len(raw) && raw[skip(raw, i)] == ']' {
			return nil, errors.New("trailing comma")
		}
	}
}
func Fields(raw []byte) (map[string][]byte, error) {
	i := skip(raw, 0)
	if i >= len(raw) || raw[i] != '{' {
		return nil, errors.New("not object")
	}
	i++
	out := make(map[string][]byte)
	for {
		i = skip(raw, i)
		if i < len(raw) && raw[i] == '}' {
			if skip(raw, i+1) != len(raw) {
				return nil, errors.New("trailing object")
			}
			return out, nil
		}
		end, err := ValueEnd(raw, i)
		if err != nil {
			return nil, err
		}
		var key string
		if json.Unmarshal(raw[i:end], &key) != nil {
			return nil, errors.New("invalid key")
		}
		if _, ok := out[key]; ok {
			return nil, errors.New("duplicate key")
		}
		i = skip(raw, end)
		if i >= len(raw) || raw[i] != ':' {
			return nil, errors.New("missing colon")
		}
		i = skip(raw, i+1)
		end, err = ValueEnd(raw, i)
		if err != nil {
			return nil, err
		}
		out[key] = raw[i:end:end]
		i = skip(raw, end)
		if i >= len(raw) {
			return nil, errors.New("missing object end")
		}
		if raw[i] == '}' {
			continue
		}
		if raw[i] != ',' {
			return nil, errors.New("missing comma")
		}
		i++
	}
}
