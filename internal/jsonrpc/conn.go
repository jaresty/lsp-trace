package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

const maxMessageBytes = 16 << 20

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("json-rpc error %d: %s", e.Code, e.Message) }

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type TraceEvent struct {
	Sequence  uint64          `json:"sequence"`
	Direction string          `json:"direction"`
	Payload   json.RawMessage `json:"payload"`
}

type TraceFunc func(TraceEvent)

type Conn struct {
	r             *bufio.Reader
	w             io.Writer
	stateMu       sync.Mutex
	writeMu       sync.Mutex
	traceMu       sync.Mutex
	next          int64
	pending       map[int64]chan message
	done          chan struct{}
	err           error
	trace         TraceFunc
	traceSequence uint64
}

func New(r io.Reader, w io.Writer) *Conn { return NewWithTrace(r, w, nil) }
func NewWithTrace(r io.Reader, w io.Writer, trace TraceFunc) *Conn {
	c := &Conn{r: bufio.NewReader(r), w: w, pending: make(map[int64]chan message), done: make(chan struct{}), trace: trace}
	go c.readLoop()
	return c
}

func (c *Conn) Call(ctx context.Context, method string, params, result interface{}) error {
	c.stateMu.Lock()
	c.next++
	id := c.next
	ch := make(chan message, 1)
	c.pending[id] = ch
	c.stateMu.Unlock()
	remove := func() { c.stateMu.Lock(); delete(c.pending, id); c.stateMu.Unlock() }
	if err := c.write(message{JSONRPC: "2.0", ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method, Params: marshalRaw(params)}); err != nil {
		remove()
		return err
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			return m.Error
		}
		if result != nil && len(m.Result) > 0 && string(m.Result) != "null" {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	case <-ctx.Done():
		remove()
		return ctx.Err()
	case <-c.done:
		remove()
		c.stateMu.Lock()
		err := c.err
		c.stateMu.Unlock()
		if err == nil {
			return io.EOF
		}
		return err
	}
}

func (c *Conn) Notify(method string, params interface{}) error {
	return c.write(message{JSONRPC: "2.0", Method: method, Params: marshalRaw(params)})
}

func marshalRaw(v interface{}) json.RawMessage {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

func (c *Conn) record(direction string, body []byte) {
	if c.trace == nil {
		return
	}
	c.traceMu.Lock()
	defer c.traceMu.Unlock()
	c.traceSequence++
	payload := append(json.RawMessage(nil), body...)
	c.trace(TraceEvent{Sequence: c.traceSequence, Direction: direction, Payload: payload})
}

func (c *Conn) write(m message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err = fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		return err
	}
	c.record("send", body)
	return nil
}

func (c *Conn) readLoop() {
	defer close(c.done)
	for {
		m, body, err := readMessageBody(c.r)
		if err != nil {
			c.stateMu.Lock()
			if !errors.Is(err, io.EOF) {
				c.err = err
			}
			c.stateMu.Unlock()
			return
		}
		c.record("receive", body)
		if m.Method != "" {
			if len(m.ID) != 0 {
				_ = c.write(message{JSONRPC: "2.0", ID: m.ID, Error: &Error{Code: -32601, Message: "method not found"}})
			}
			continue
		}
		id, err := strconv.ParseInt(string(m.ID), 10, 64)
		if err != nil {
			continue
		}
		c.stateMu.Lock()
		ch := c.pending[id]
		delete(c.pending, id)
		c.stateMu.Unlock()
		if ch != nil {
			ch <- m
		}
	}
}

func readMessage(r *bufio.Reader) (message, error) {
	m, _, err := readMessageBody(r)
	return m, err
}

func readMessageBody(r *bufio.Reader) (message, []byte, error) {
	length := -1
	seenLength := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return message{}, nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return message{}, nil, fmt.Errorf("invalid header %q", line)
		}
		if strings.EqualFold(name, "Content-Length") {
			if seenLength {
				return message{}, nil, errors.New("duplicate Content-Length")
			}
			seenLength = true
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return message{}, nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
		}
	}
	if length < 0 {
		return message{}, nil, errors.New("missing Content-Length")
	}
	if length > maxMessageBytes {
		return message{}, nil, fmt.Errorf("Content-Length %d exceeds limit %d", length, maxMessageBytes)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return message{}, nil, err
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return message{}, nil, err
	}
	if m.JSONRPC != "2.0" {
		return message{}, nil, fmt.Errorf("unsupported jsonrpc version %q", m.JSONRPC)
	}
	return m, body, nil
}
