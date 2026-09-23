package lspwire

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
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
	capture := &framedReadCapture{digest: sha256.New(), maxRaw: maxBytes}
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
	capture := &framedReadCapture{digest: sha256.New(), maxRaw: maxBytes}
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
