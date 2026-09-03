package lspwire

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

const Version = "2.0"

var (
	ErrInvalidContentLength   = errors.New("invalid Content-Length")
	ErrDuplicateContentLength = errors.New("duplicate Content-Length")
	ErrMissingContentLength   = errors.New("missing Content-Length")
	ErrFrameTooLarge          = errors.New("frame too large")
	ErrHeaderTooLarge         = errors.New("headers too large")
	ErrMalformedJSON          = errors.New("malformed JSON")
	ErrWrongVersion           = errors.New("wrong JSON-RPC version")
	ErrInvalidMessage         = errors.New("invalid JSON-RPC message")
)

type Limits struct{ MaxBodyBytes, MaxHeaderBytes int64 }

func DefaultLimits() Limits { return Limits{MaxBodyBytes: 16 << 20, MaxHeaderBytes: 64 << 10} }
func (l Limits) normalized() Limits {
	d := DefaultLimits()
	if l.MaxBodyBytes <= 0 {
		l.MaxBodyBytes = d.MaxBodyBytes
	}
	if l.MaxHeaderBytes <= 0 {
		l.MaxHeaderBytes = d.MaxHeaderBytes
	}
	return l
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}
type Kind uint8

const (
	KindInvalid Kind = iota
	KindRequest
	KindNotification
	KindSuccessResponse
	KindErrorResponse
)

func (m Message) Kind() Kind {
	if m.Method != "" {
		if m.Error != nil || m.Result != nil {
			return KindInvalid
		}
		if len(m.ID) > 0 {
			return KindRequest
		}
		return KindNotification
	}
	if len(m.ID) == 0 {
		return KindInvalid
	}
	if m.Error != nil && len(m.Result) == 0 {
		return KindErrorResponse
	}
	if m.Error == nil && m.Result != nil {
		return KindSuccessResponse
	}
	return KindInvalid
}

type Reader struct {
	r      *bufio.Reader
	limits Limits
}

func NewReader(r io.Reader, limits Limits) *Reader {
	return &Reader{r: bufio.NewReader(r), limits: limits.normalized()}
}
func (r *Reader) Read() (Message, error) {
	length := -1
	seenLength := false
	var used int64
	for {
		line, err := r.r.ReadString('\n')
		if err != nil {
			return Message{}, err
		}
		used += int64(len(line))
		if used > r.limits.MaxHeaderBytes {
			return Message{}, ErrHeaderTooLarge
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return Message{}, fmt.Errorf("%w: %q", ErrInvalidContentLength, line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if seenLength {
				return Message{}, ErrDuplicateContentLength
			}
			seenLength = true
			n, e := strconv.Atoi(strings.TrimSpace(value))
			if e != nil || n < 0 {
				return Message{}, fmt.Errorf("%w: %q", ErrInvalidContentLength, value)
			}
			length = n
		}
	}
	if length < 0 {
		return Message{}, ErrMissingContentLength
	}
	if int64(length) > r.limits.MaxBodyBytes {
		return Message{}, fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, length, r.limits.MaxBodyBytes)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r.r, body); err != nil {
		return Message{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var m Message
	if err := dec.Decode(&m); err != nil {
		return Message{}, fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return Message{}, ErrMalformedJSON
	}
	if m.JSONRPC != Version {
		return Message{}, fmt.Errorf("%w: %q", ErrWrongVersion, m.JSONRPC)
	}
	if m.Kind() == KindInvalid {
		return Message{}, ErrInvalidMessage
	}
	return m, nil
}

type Writer struct {
	w      io.Writer
	limits Limits
	mu     sync.Mutex
}

func NewWriter(w io.Writer, limits Limits) *Writer { return &Writer{w: w, limits: limits.normalized()} }
func (w *Writer) Write(m Message) error {
	if m.JSONRPC == "" {
		m.JSONRPC = Version
	}
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if int64(len(body)) > w.limits.MaxBodyBytes {
		return ErrFrameTooLarge
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = fmt.Fprintf(w.w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

type RequestKey struct {
	Generation uint64
	ID         uint64
}
type ResponseKey = RequestKey
type ResponseDisposition uint8

const (
	ResponseAccepted ResponseDisposition = iota
	ResponseUnknown
	ResponseDuplicate
	ResponseWrongGeneration
)

type CancelState uint8

const (
	CancelWritten CancelState = iota
	CancelAlreadyWritten
	CancelNotPending
)

type pendingEntry struct {
	generation    uint64
	cancelWritten bool
}
type Pending struct {
	mu          sync.Mutex
	next        uint64
	cap         int
	active      map[uint64]pendingEntry
	tomb        map[RequestKey]struct{}
	order       []RequestKey
	generations map[uint64]struct{}
}

func NewPending(tombstoneCapacity int) *Pending {
	if tombstoneCapacity < 0 {
		tombstoneCapacity = 0
	}
	return &Pending{cap: tombstoneCapacity, active: map[uint64]pendingEntry{}, tomb: map[RequestKey]struct{}{}, generations: map[uint64]struct{}{}}
}
func (p *Pending) Begin(g uint64) RequestKey {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	p.active[p.next] = pendingEntry{generation: g}
	p.generations[g] = struct{}{}
	return RequestKey{g, p.next}
}
func (p *Pending) Accept(k ResponseKey) ResponseDisposition {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.tomb[k]; ok {
		return ResponseDuplicate
	}
	e, ok := p.active[k.ID]
	if !ok {
		if _, seen := p.generations[k.Generation]; !seen && len(p.generations) > 0 {
			return ResponseWrongGeneration
		}
		return ResponseUnknown
	}
	if e.generation != k.Generation {
		return ResponseWrongGeneration
	}
	delete(p.active, k.ID)
	p.addTomb(k)
	return ResponseAccepted
}
func (p *Pending) addTomb(k RequestKey) {
	if p.cap == 0 {
		return
	}
	p.tomb[k] = struct{}{}
	p.order = append(p.order, k)
	if len(p.order) > p.cap {
		old := p.order[0]
		p.order = p.order[1:]
		delete(p.tomb, old)
	}
}
func (p *Pending) TombstoneCount() int { p.mu.Lock(); defer p.mu.Unlock(); return len(p.tomb) }
func (p *Pending) Cancel(w *Writer, k RequestKey) (CancelState, error) {
	p.mu.Lock()
	e, ok := p.active[k.ID]
	if !ok || e.generation != k.Generation {
		p.mu.Unlock()
		return CancelNotPending, nil
	}
	if e.cancelWritten {
		p.mu.Unlock()
		return CancelAlreadyWritten, nil
	}
	e.cancelWritten = true
	p.active[k.ID] = e
	p.mu.Unlock()
	id := json.RawMessage(strconv.FormatUint(k.ID, 10))
	err := w.Write(Message{JSONRPC: Version, Method: "$/cancelRequest", Params: json.RawMessage(fmt.Sprintf(`{"id":%s}`, id))})
	if err != nil {
		p.mu.Lock()
		e.cancelWritten = false
		p.active[k.ID] = e
		p.mu.Unlock()
		return CancelNotPending, err
	}
	return CancelWritten, nil
}

type InitializeConfig struct {
	ProcessID     int    `json:"processId"`
	RootURI       string `json:"rootUri,omitempty"`
	ClientName    string `json:"-"`
	ClientVersion string `json:"-"`
	Trace         string `json:"trace,omitempty"`
}
type initializeParams struct {
	ProcessID    int            `json:"processId"`
	ClientInfo   clientInfo     `json:"clientInfo"`
	RootURI      string         `json:"rootUri,omitempty"`
	Capabilities map[string]any `json:"capabilities"`
	Trace        string         `json:"trace,omitempty"`
}
type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

func InitializeRequest(k RequestKey, c InitializeConfig) (Message, error) {
	params := initializeParams{ProcessID: c.ProcessID, ClientInfo: clientInfo{c.ClientName, c.ClientVersion}, RootURI: c.RootURI, Capabilities: map[string]any{}, Trace: c.Trace}
	b, err := json.Marshal(params)
	if err != nil {
		return Message{}, err
	}
	return Message{JSONRPC: Version, ID: json.RawMessage(strconv.FormatUint(k.ID, 10)), Method: "initialize", Params: b}, nil
}
func InitializedNotification() Message {
	return Message{JSONRPC: Version, Method: "initialized", Params: json.RawMessage(`{}`)}
}
