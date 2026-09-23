package sessionruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"

	"lsp-trace/internal/lspwire"
)

// RequestWriteObservation is manager-local evidence of a completed framed write.
// It does not establish provider receipt, provider authentication, or replay custody.
type RequestWriteObservation struct {
	SessionID   string
	Generation  uint64
	Key         lspwire.RequestKey
	Method      string
	FrameBytes  int64
	FrameSHA256 string
}

// CompletedRequestWrite returns a value copy only when the owned framed writer
// was observed to return successfully. It is not an occurrence-admission receipt.
func (r RoundTripResult) CompletedRequestWrite() (RequestWriteObservation, bool) {
	if r.requestWrite == nil {
		return RequestWriteObservation{}, false
	}
	return *r.requestWrite, true
}

// framedWriteCapture hashes bytes reported written to the owned child, including
// the Content-Length header. It is never used as completion evidence by itself:
// partial writes and framing errors can leave a nonempty, unconfirmed digest.
type framedWriteCapture struct {
	io.Writer
	digest    hash.Hash
	bytes     int64
	maxRaw    int64
	raw       []byte
	overLimit bool
}

func newFramedWriteCapture(w io.Writer) *framedWriteCapture {
	return newFramedWriteCaptureBounded(w, 0)
}

// A positive cap retains only bytes actually reported written; zero keeps the
// historical digest-only behavior. The manager decides completion and selection.
func newFramedWriteCaptureBounded(w io.Writer, maxRaw int64) *framedWriteCapture {
	return &framedWriteCapture{Writer: w, digest: sha256.New(), maxRaw: maxRaw}
}

func (w *framedWriteCapture) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if n > 0 && n <= len(p) {
		_, _ = w.digest.Write(p[:n])
		w.bytes += int64(n)
		if w.maxRaw > 0 && !w.overLimit {
			if int64(n) > w.maxRaw-int64(len(w.raw)) {
				w.raw = nil
				w.overLimit = true
			} else {
				w.raw = append(w.raw, p[:n]...)
			}
		}
	}
	return n, err
}

func (w *framedWriteCapture) retainedFrame() ([]byte, bool) {
	if w.maxRaw <= 0 || w.overLimit || w.bytes == 0 || int64(len(w.raw)) != w.bytes {
		return nil, false
	}
	return append([]byte(nil), w.raw...), true
}

func (w *framedWriteCapture) sha256() string {
	return "sha256:" + hex.EncodeToString(w.digest.Sum(nil))
}
