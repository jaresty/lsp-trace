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

type Conn struct {
	r       *bufio.Reader
	w       io.Writer
	mu      sync.Mutex
	next    int64
	pending map[int64]chan message
	done    chan struct{}
	err     error
}

func New(r io.Reader, w io.Writer) *Conn {
	c := &Conn{r: bufio.NewReader(r), w: w, pending: make(map[int64]chan message), done: make(chan struct{})}
	go c.readLoop()
	return c
}

func (c *Conn) Call(ctx context.Context, method string, params, result interface{}) error {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan message, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.write(message{JSONRPC: "2.0", ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method, Params: marshalRaw(params)}); err != nil {
		return err
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			return m.Error
		}
		if result != nil && string(m.Result) != "null" {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.err
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

func (c *Conn) write(m message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

func (c *Conn) readLoop() {
	defer close(c.done)
	for {
		m, err := readMessage(c.r)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.err = err
			}
			return
		}
		if len(m.ID) == 0 {
			continue
		}
		id, err := strconv.ParseInt(string(m.ID), 10, 64)
		if err != nil {
			continue
		}
		c.mu.Lock()
		ch := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	}
}

func readMessage(r *bufio.Reader) (message, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return message{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return message{}, fmt.Errorf("invalid header %q", line)
		}
		if strings.EqualFold(name, "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return message{}, err
			}
		}
	}
	if length < 0 {
		return message{}, errors.New("missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return message{}, err
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return message{}, err
	}
	return m, nil
}
