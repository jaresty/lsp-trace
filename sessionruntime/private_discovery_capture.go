package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"unsafe"
)

// These values are private diagnostic availability, never canonical receipts.
var errPrivateCaptureCloseUncertain = errors.New("private capture close uncertain")

type PrivateCaptureStatus uint8

const (
	CaptureDisabled PrivateCaptureStatus = iota
	CaptureNotCaptured
	CaptureAbsent
	CaptureEmpty
	CaptureComplete
	CaptureTruncated
	CaptureFailed
	CaptureUnavailable
)

type PrivateCaptureReason uint8

const (
	CaptureNoReason PrivateCaptureReason = iota
	CaptureNotSelected
	CaptureNotErrorObject
	CaptureQuotaExhausted
	CaptureSinkFailure
	CaptureDeadline
)

type PrivateCaptureRecord struct {
	Status          PrivateCaptureStatus `json:"status"`
	Reason          PrivateCaptureReason `json:"reason"`
	Artifact        uint8                `json:"artifact"`
	Bytes           uint16               `json:"bytes"`
	BytesAvailable  bool                 `json:"bytes_available"`
	DigestAvailable bool                 `json:"digest_available"`
	OriginalBytes   int64                `json:"original_error_object_bytes"`
	PrefixSHA256    [32]byte             `json:"prefix_sha256"`
}

func (r PrivateCaptureRecord) Valid() bool {
	if r.Status > CaptureUnavailable || r.Reason > CaptureDeadline || r.Artifact > 2 || r.Bytes > 8192 || r.OriginalBytes < 0 || r.OriginalBytes > 16<<20 {
		return false
	}
	zero := r.Artifact == 0 && r.Bytes == 0 && r.OriginalBytes == 0 && r.PrefixSHA256 == [32]byte{}
	switch r.Status {
	case CaptureDisabled, CaptureAbsent:
		return zero && !r.BytesAvailable && !r.DigestAvailable && r.Reason == CaptureNoReason
	case CaptureEmpty:
		return zero && r.BytesAvailable && !r.DigestAvailable && r.Reason == CaptureNoReason
	case CaptureNotCaptured:
		return zero && !r.BytesAvailable && !r.DigestAvailable && (r.Reason == CaptureNoReason || r.Reason == CaptureNotSelected || r.Reason == CaptureNotErrorObject)
	case CaptureUnavailable:
		return zero && !r.BytesAvailable && !r.DigestAvailable && r.Reason == CaptureQuotaExhausted
	case CaptureFailed:
		return !r.BytesAvailable && !r.DigestAvailable && r.Artifact > 0 && r.OriginalBytes >= 2 && r.Bytes == 0 && r.PrefixSHA256 == [32]byte{} && (r.Reason == CaptureSinkFailure || r.Reason == CaptureDeadline)
	case CaptureComplete:
		return r.BytesAvailable && r.DigestAvailable && r.Reason == CaptureNoReason && r.Artifact > 0 && r.Bytes >= 2 && int64(r.Bytes) == r.OriginalBytes
	case CaptureTruncated:
		return r.BytesAvailable && r.DigestAvailable && r.Reason == CaptureNoReason && r.Artifact > 0 && r.Bytes == 8192 && r.OriginalBytes > 8192
	}
	return false
}
func (r PrivateCaptureRecord) MarshalJSON() ([]byte, error) {
	if !r.Valid() {
		return nil, errors.New("invalid private capture record")
	}
	type value PrivateCaptureRecord
	return json.Marshal(value(r))
}

// Nil means OFF. Destination is operator-owned configuration, never server data.
// No permission is inferred from the presence of a server error.
type PrivateDiscoveryCaptureOptions struct {
	Permission  bool
	Destination string
}

// Quote is a request against the actual operation's already reserved metadata
// allowance, not a grant. Zero provider/request/response/wire currencies mean
// capture may not perform protocol effects. Runtime backing/work is paid again
// against the concrete discovery window before allocation.
type PrivateDiscoveryCaptureQuote struct {
	LogicalBytes, ReadHashBytes, WriteBytes, Records, ParserBytes uint64
	ProviderRequests, ProviderMessages, Work, ActiveNS, Calls     uint64
	RequestBytes, ResponseBytes, WireBytes, OutputBytes           uint64
}

func PrivateDiscoveryCaptureQuoteForOptions(o *PrivateDiscoveryCaptureOptions) (PrivateDiscoveryCaptureQuote, bool) {
	if o == nil {
		return PrivateDiscoveryCaptureQuote{}, true
	}
	if !o.Permission || !privateCapturePathSyntax(o.Destination) {
		return PrivateDiscoveryCaptureQuote{}, false
	}
	n := uint64(unsafe.Sizeof(privateDiscoveryCapture{}))
	work := 32 * (n + 16384 + 4096 + 48000)
	return PrivateDiscoveryCaptureQuote{LogicalBytes: work, ReadHashBytes: 16384, WriteBytes: 16384 + 48000, Records: 5, ParserBytes: 4096 + 48000, Work: work, Calls: 1024, OutputBytes: 16384 + 48000}, true
}

// This callback must be installed by the genuine reserved operation before the
// runtime is entered. It does not authenticate direct runner calls or mint funds.
type PrivateDiscoveryCaptureAdmission func(PrivateDiscoveryCaptureQuote) error

type privateDiscoveryCapture struct {
	mu        sync.Mutex
	root      *os.File
	buffers   [2][8192]byte
	count     uint8
	uncertain bool
	open      func(int) (io.WriteCloser, error)
}

func (c *privateDiscoveryCapture) capture(ctx context.Context, raw []byte, start, end int) PrivateCaptureRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	if start < 0 || end <= start || end > len(raw) || end-start < 2 || raw[start] != '{' || raw[end-1] != '}' {
		return PrivateCaptureRecord{Status: CaptureNotCaptured, Reason: CaptureNotErrorObject}
	}
	if c.count == 2 {
		return PrivateCaptureRecord{Status: CaptureUnavailable, Reason: CaptureQuotaExhausted}
	}
	slot := c.count
	c.count++
	r := PrivateCaptureRecord{Status: CaptureFailed, Reason: CaptureSinkFailure, Artifact: slot + 1, OriginalBytes: int64(end - start)}
	if ctx == nil || ctx.Err() != nil {
		r.Reason = CaptureDeadline
		return r
	}
	n := min(end-start, 8192)
	copy(c.buffers[slot][:n], raw[start:start+n])
	// Only the prepaid prefix is hashed. Never describe this as a response hash.
	sum := sha256.Sum256(c.buffers[slot][:n])
	if ctx.Err() != nil {
		r.Reason = CaptureDeadline
		c.uncertain = true
		return r
	}
	var sink io.WriteCloser
	var err error
	if c.open != nil {
		sink, err = c.open(int(slot))
	} else if c.root != nil {
		sink, err = privateCaptureOpenFile(ctx, c.root, slot)
	}
	if err != nil || sink == nil {
		if errors.Is(err, errPrivateCaptureCloseUncertain) {
			c.uncertain = true
		}
		if ctx.Err() != nil {
			r.Reason = CaptureDeadline
			c.uncertain = true
		}
		return r
	}
	written := 0
	if ctx.Err() == nil {
		written, err = sink.Write(c.buffers[slot][:n])
	} else {
		err = ctx.Err()
	}
	closeErr := sink.Close()
	if closeErr != nil {
		c.uncertain = true
	}
	if ctx.Err() != nil {
		r.Reason = CaptureDeadline
		c.uncertain = true
		return r
	}
	if err != nil || closeErr != nil || written != n {
		return r
	}
	r.Status = CaptureComplete
	if end-start > 8192 {
		r.Status = CaptureTruncated
	}
	r.Reason = CaptureNoReason
	r.Bytes = uint16(n)
	r.BytesAvailable = true
	r.DigestAvailable = true
	r.PrefixSHA256 = sum
	return r
}

// Kept local so ordinary transport and ordinary Reader cannot opt themselves in.
func privateCapturePathSyntax(path string) bool {
	if len(path) < 2 || len(path) > 4096 || path[0] != '/' || path[len(path)-1] == '/' {
		return false
	}
	count := 0
	for at := 1; at < len(path); {
		end := at
		for end < len(path) && path[end] != '/' {
			end++
		}
		part := path[at:end]
		if part == "" || part == "." || part == ".." {
			return false
		}
		count++
		if count > 128 {
			return false
		}
		at = end + 1
	}
	return true
}

func (w *privateDiscoveryWindow) prepayCapture(o *PrivateDiscoveryCaptureOptions, admit PrivateDiscoveryCaptureAdmission) bool {
	if o == nil {
		return true
	}
	options := *o
	q, ok := PrivateDiscoveryCaptureQuoteForOptions(&options)
	if !ok || admit == nil || w.ctx.Err() != nil || admit(q) != nil {
		return false
	}
	// Funding precedes directory opens and the compiler-sized fixed buffers.
	n := uint64(unsafe.Sizeof(privateDiscoveryCapture{})) + 16384
	w.mu.Lock()
	paid := w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, n}, privateDiscoveryCharge{&w.remaining.Work, q.Work})
	w.mu.Unlock()
	if !paid {
		return false
	}
	root, err := openPrivateCaptureDirectory(w.ctx, options.Destination)
	if err != nil {
		if errors.Is(err, errPrivateCaptureCloseUncertain) {
			w.mu.Lock()
			w.capturePreparationUncertain = true
			w.mu.Unlock()
		}
		return false
	}
	w.capture = &privateDiscoveryCapture{root: root}
	if w.ctx.Err() != nil {
		w.capture.close(w.ctx)
		return false
	}
	return true
}

func (c *privateDiscoveryCapture) close(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root != nil {
		if c.root.Close() != nil {
			c.uncertain = true
		}
		c.root = nil
	}
	if ctx.Err() != nil {
		c.uncertain = true
	}
	return !c.uncertain
}
