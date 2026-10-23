package lspwire

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"strconv"
	"strings"

	"lsp-trace/internal/directionalraw"
)

var (
	ErrInvalidFrameCaptureLimit = errors.New("invalid frame capture limit")
	ErrFrameCaptureLimit        = errors.New("frame capture limit exceeded")
)

// ReadFrameObservation is local evidence about one complete decoded frame.
// It contains no raw bytes and does not authenticate the sender.
type ReadFrameObservation struct {
	FrameBytes  int64
	FrameSHA256 string
}

// framedReadCapture hashes only bytes consumed for this frame, not bytes that
// bufio.Reader may have prefetched from later messages. Raw retention is opt-in.
type framedReadCapture struct {
	digest    hash.Hash
	bytes     int64
	maxRaw    int64
	onRetain  func() // optional test-only notification before each raw append
	raw       []byte
	overLimit bool
}

func (c *framedReadCapture) add(p []byte) {
	_, _ = c.digest.Write(p)
	c.bytes += int64(len(p))
	if c.maxRaw <= 0 || c.overLimit {
		return
	}
	if int64(len(p)) > c.maxRaw-int64(len(c.raw)) {
		c.raw = nil
		c.overLimit = true
		return
	}
	if c.onRetain != nil {
		func() {
			defer func() { _ = recover() }()
			c.onRetain()
		}()
	}
	c.raw = append(c.raw, p...)
}

// ReadWithExactFrame returns a private, in-memory copy of the exact consumed
// header and body only for a valid frame within maxBytes. A failed or over-cap
// read returns no message, observation, or bytes; an invalid cap reads nothing.
// The cap limits retained bytes, not total reader allocation or wire resources.
// This method neither persists evidence nor attests to its producer.
// Like Read, this Reader has one consumer and may not be read concurrently.
func (r *Reader) ReadWithExactFrame(maxBytes int64) (Message, ReadFrameObservation, []byte, error) {
	if maxBytes <= 0 {
		return Message{}, ReadFrameObservation{}, nil, ErrInvalidFrameCaptureLimit
	}
	capture := &framedReadCapture{digest: sha256.New(), maxRaw: maxBytes, onRetain: r.retainEntry}
	r.frameCapture = capture
	defer func() { r.frameCapture = nil }()
	message, err := r.Read()
	if err != nil {
		return Message{}, ReadFrameObservation{}, nil, err
	}
	if capture.overLimit {
		return Message{}, ReadFrameObservation{}, nil, ErrFrameCaptureLimit
	}
	return message, ReadFrameObservation{
		FrameBytes:  capture.bytes,
		FrameSHA256: "sha256:" + hex.EncodeToString(capture.digest.Sum(nil)),
	}, append([]byte(nil), capture.raw...), nil
}

// ReadWithFrameIfWithin is an opt-in in-memory read for deferred frame
// selection. Valid over-cap frames remain decoded and observable, but return no
// raw bytes; only a valid within-cap frame returns an independent raw copy.
// The cap bounds retained raw bytes, not total reader allocation or wire work.
// It neither persists evidence nor identifies the accepted request by itself.
func (r *Reader) ReadWithFrameIfWithin(maxBytes int64) (Message, ReadFrameObservation, []byte, bool, error) {
	if maxBytes <= 0 {
		return Message{}, ReadFrameObservation{}, nil, false, ErrInvalidFrameCaptureLimit
	}
	capture := &framedReadCapture{digest: sha256.New(), maxRaw: maxBytes, onRetain: r.retainEntry}
	r.frameCapture = capture
	defer func() { r.frameCapture = nil }()
	message, err := r.Read()
	if err != nil {
		return Message{}, ReadFrameObservation{}, nil, false, err
	}
	observed := ReadFrameObservation{
		FrameBytes:  capture.bytes,
		FrameSHA256: "sha256:" + hex.EncodeToString(capture.digest.Sum(nil)),
	}
	if capture.overLimit {
		return message, observed, nil, false, nil
	}
	return message, observed, append([]byte(nil), capture.raw...), true, nil
}

// ReadWithFrame returns an observation only after a complete valid message.
// Reader has the same single-consumer ownership requirement as Read; neither
// method may be called concurrently on one Reader.
func (r *Reader) ReadWithFrame() (Message, ReadFrameObservation, error) {
	capture := &framedReadCapture{digest: sha256.New()}
	r.frameCapture = capture
	defer func() { r.frameCapture = nil }()
	message, err := r.Read()
	if err != nil {
		return Message{}, ReadFrameObservation{}, err
	}
	return message, ReadFrameObservation{
		FrameBytes:  capture.bytes,
		FrameSHA256: "sha256:" + hex.EncodeToString(capture.digest.Sum(nil)),
	}, nil
}

// WriteDirectionalReserved builds exactly one owned frame. Prefix/header are
// ephemeral encoding scratch; params is not re-marshaled. The lease allocator
// reserves BEFORE the sole retained allocation, and body aliases that frame.
func WriteDirectionalReserved(w io.Writer, id json.RawMessage, method string, params []byte, lease directionalraw.Lease, maxBodyBytes int64) (int64, ReadFrameObservation, error) {
	if !json.Valid(params) || !directionalraw.Allowed(method) {
		return 0, ReadFrameObservation{}, ErrInvalidMessage
	}
	prefix := `{"id":` + string(id) + `,"jsonrpc":"2.0","method":` + strconv.Quote(method) + `,"params":`
	bodyLen := len(prefix) + len(params) + 1
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultLimits().MaxBodyBytes
	}
	if int64(bodyLen) > maxBodyBytes {
		return 0, ReadFrameObservation{}, ErrFrameTooLarge
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", bodyLen)
	frame, err := lease.Allocate("request-frame", len(header)+bodyLen)
	if err != nil {
		h := sha256.New()
		h.Write([]byte(header))
		h.Write([]byte(prefix))
		h.Write(params)
		h.Write([]byte{'}'})
		var sum [32]byte
		copy(sum[:], h.Sum(nil))
		lease.RefusedPrivate("request-frame", len(header)+bodyLen, sum)
		return 0, ReadFrameObservation{}, err
	}
	copy(frame, header)
	at := len(header)
	at += copy(frame[at:], prefix)
	at += copy(frame[at:], params)
	frame[at] = '}'
	n, err := w.Write(frame)
	if n < 0 || n > len(frame) {
		return 0, ReadFrameObservation{}, ErrInvalidMessage
	}
	if !lease.Frame("request-frame", n) {
		return 0, ReadFrameObservation{}, ErrInvalidMessage
	}
	if err == nil && n != len(frame) {
		err = io.ErrShortWrite
	}
	sum := sha256.Sum256(frame[:n])
	writtenBody := n - len(header)
	if writtenBody < 0 {
		writtenBody = 0
	}
	return int64(writtenBody), ReadFrameObservation{FrameBytes: int64(n), FrameSHA256: fmt.Sprintf("sha256:%x", sum)}, err
}

// ReadDirectionalReserved uses bounded stack header scratch, then exactly one
// Manager-reserved backing for original header+body. JSON fields are slices of
// that backing, not RawMessage unmarshal copies. Malformed/partial bytes remain
// owned until the generic terminal; ordinary Read is unchanged.
func (r *Reader) ReadDirectionalReserved(maxBytes int64, lease directionalraw.Lease) (Message, ReadFrameObservation, []byte, error) {
	var header [65536]byte
	used := 0
	length := -1
	headerLines := 0
	lineStart := 0
	retainHeader := func(cause error) (Message, ReadFrameObservation, []byte, error) {
		if used == 0 {
			return Message{}, ReadFrameObservation{}, nil, cause
		}
		raw, e := lease.Allocate("response-frame", used)
		if e != nil {
			return Message{}, ReadFrameObservation{}, nil, e
		}
		copy(raw, header[:used])
		lease.Frame("response-frame", used)
		sum := sha256.Sum256(raw)
		return Message{}, ReadFrameObservation{FrameBytes: int64(used), FrameSHA256: fmt.Sprintf("sha256:%x", sum)}, raw, cause
	}
	for {
		if used >= len(header) || int64(used) >= r.limits.MaxHeaderBytes {
			return retainHeader(ErrHeaderTooLarge)
		}
		c, err := r.r.ReadByte()
		if err != nil {
			return retainHeader(err)
		}
		header[used] = c
		used++
		if c != '\n' {
			continue
		}
		line := strings.TrimRight(string(header[lineStart:used]), "\r\n")
		lineStart = used
		if line == "" {
			break
		}
		headerLines++
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return retainHeader(ErrInvalidContentLength)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if length >= 0 {
				return retainHeader(ErrDuplicateContentLength)
			}
			n, e := strconv.Atoi(strings.TrimSpace(value))
			if e != nil || n < 0 {
				return retainHeader(ErrInvalidContentLength)
			}
			length = n
		}
	}
	if length < 0 {
		if headerLines == 0 {
			return retainHeader(ErrEmptyHeaderBlock)
		}
		return retainHeader(ErrNonemptyHeaderMissingContentLength)
	}
	if int64(length) > r.limits.MaxBodyBytes || maxBytes <= int64(used) || int64(length) > maxBytes-int64(used) {
		return retainHeader(ErrFrameTooLarge)
	}
	frame, err := lease.Allocate("response-frame", used+length)
	if err != nil {
		return Message{}, ReadFrameObservation{}, nil, err
	}
	copy(frame, header[:used])
	n, err := io.ReadFull(r.r, frame[used:])
	raw := frame[: used+n : used+n]
	lease.Frame("response-frame", len(raw))
	sum := sha256.Sum256(raw)
	o := ReadFrameObservation{FrameBytes: int64(len(raw)), FrameSHA256: fmt.Sprintf("sha256:%x", sum)}
	if err != nil {
		return Message{}, o, raw, err
	}
	fields, err := directionalraw.Fields(frame[used:])
	if err != nil {
		return Message{}, o, raw, ErrMalformedJSON
	}
	var msg Message
	for k, v := range fields {
		switch k {
		case "jsonrpc":
			if json.Unmarshal(v, &msg.JSONRPC) != nil {
				return Message{}, o, raw, ErrInvalidMessage
			}
		case "id":
			msg.ID = v
		case "method":
			if json.Unmarshal(v, &msg.Method) != nil {
				return Message{}, o, raw, ErrInvalidMessage
			}
		case "params":
			msg.Params = v
		case "result":
			msg.Result = v
		case "error":
			var e RPCError
			ef, er := directionalraw.Fields(v)
			if er != nil {
				return Message{}, o, raw, ErrInvalidMessage
			}
			if json.Unmarshal(ef["code"], &e.Code) != nil || json.Unmarshal(ef["message"], &e.Message) != nil {
				return Message{}, o, raw, ErrInvalidMessage
			}
			e.Data = ef["data"]
			msg.Error = &e
		default:
			return Message{}, o, raw, ErrInvalidMessage
		}
	}
	if msg.JSONRPC != Version {
		return Message{}, o, raw, ErrWrongVersion
	}
	if msg.Kind() == KindInvalid {
		return Message{}, o, raw, ErrInvalidMessage
	}
	return msg, o, raw, nil
}
