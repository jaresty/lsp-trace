package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"time"
	"unsafe"

	"lsp-trace/internal/discoveryfailure"
	"lsp-trace/internal/lsp"
)

// Caller assertions only, never server strings or runtime/authority evidence.
// An absent value has an explicit availability bit, not an asserted zero hash.
// Semantic version fields avoid arbitrary version/build labels entirely.
type PrivateDiscoveryFingerprints struct {
	Preverified        bool     `json:"preverified"`
	VersionAvailable   bool     `json:"version_available"`
	Major              uint16   `json:"major"`
	Minor              uint16   `json:"minor"`
	Patch              uint16   `json:"patch"`
	BinaryAvailable    bool     `json:"binary_available"`
	BinarySHA256       [32]byte `json:"binary_sha256"`
	WorkspaceAvailable bool     `json:"workspace_available"`
	WorkspaceSHA256    [32]byte `json:"workspace_sha256"`
	ConfigAvailable    bool     `json:"config_available"`
	ConfigSHA256       [32]byte `json:"config_sha256"`
}

func (f PrivateDiscoveryFingerprints) Valid() bool {
	if !f.Preverified {
		return f == (PrivateDiscoveryFingerprints{})
	}
	return (f.VersionAvailable || f.Major == 0 && f.Minor == 0 && f.Patch == 0) &&
		(f.BinaryAvailable || f.BinarySHA256 == ([32]byte{})) &&
		(f.WorkspaceAvailable || f.WorkspaceSHA256 == ([32]byte{})) &&
		(f.ConfigAvailable || f.ConfigSHA256 == ([32]byte{}))
}

type PrivateDiscoveryMethod uint8

const (
	PrivateMethodUnknown PrivateDiscoveryMethod = iota
	PrivateMethodDocumentSymbol
	PrivateMethodPrepareCallHierarchy
)

type PrivateDiscoveryEncoding uint8

const (
	PrivateEncodingUnknown PrivateDiscoveryEncoding = iota
	PrivateEncodingUTF8
	PrivateEncodingUTF16
	PrivateEncodingUTF32
	PrivateEncodingUnsupported
)

type PrivateDiscoveryEncodingOrigin uint8

const (
	PrivateEncodingOriginUnknown PrivateDiscoveryEncodingOrigin = iota
	PrivateEncodingProtocolDefault
	PrivateEncodingAdvertised
	PrivateEncodingInvalidField
)

type PrivateDiscoverySessionContext struct {
	Fingerprints   PrivateDiscoveryFingerprints   `json:"fingerprints"`
	Encoding       PrivateDiscoveryEncoding       `json:"encoding"`
	EncodingOrigin PrivateDiscoveryEncodingOrigin `json:"encoding_origin"`
}

func (s PrivateDiscoverySessionContext) valid() bool {
	return s.Fingerprints.Valid() && s.Encoding <= PrivateEncodingUnsupported && s.EncodingOrigin <= PrivateEncodingInvalidField &&
		((s.Encoding == PrivateEncodingUnknown) == (s.EncodingOrigin == PrivateEncodingOriginUnknown)) &&
		(s.EncodingOrigin != PrivateEncodingProtocolDefault || s.Encoding == PrivateEncodingUTF16) &&
		(s.EncodingOrigin != PrivateEncodingInvalidField || s.Encoding == PrivateEncodingUnsupported)
}

type PrivateDiscoveryCorrelation struct {
	Method            PrivateDiscoveryMethod `json:"method"`
	FileAvailable     bool                   `json:"file_available"`
	FileOrdinal       int64                  `json:"file_ordinal"`
	TargetAvailable   bool                   `json:"target_available"`
	TargetOrdinal     int64                  `json:"target_ordinal"`
	PositionAvailable bool                   `json:"position_available"`
	Line              int64                  `json:"line"`
	Character         int64                  `json:"character"`
	ContentAvailable  bool                   `json:"content_available"`
	ContentSHA256     [32]byte               `json:"content_sha256"`
}

func (c PrivateDiscoveryCorrelation) valid() bool {
	return c.Method <= PrivateMethodPrepareCallHierarchy &&
		(!c.FileAvailable && c.FileOrdinal == 0 || c.FileAvailable && c.FileOrdinal >= 0 && c.FileOrdinal < 16) &&
		(!c.TargetAvailable && c.TargetOrdinal == 0 || c.TargetAvailable && c.TargetOrdinal >= 0 && c.TargetOrdinal < 65536) &&
		(!c.PositionAvailable && c.Line == 0 && c.Character == 0 || c.PositionAvailable && c.Method == PrivateMethodPrepareCallHierarchy && c.Line >= 0 && c.Line <= 1<<32-1 && c.Character >= 0 && c.Character <= 1<<32-1) &&
		(c.ContentAvailable || c.ContentSHA256 == ([32]byte{}))
}

// Elapsed is observed once at the actual boundary, never during serialization.
// Remaining is clamped at zero after the immutable deadline; WithinDeadline
// distinguishes an event at/after expiry from an event with positive residual.
// With no original anchor/deadline all fields remain unavailable/zero.
type PrivateDiscoveryEventTime struct {
	Available      bool  `json:"available"`
	ElapsedNS      int64 `json:"elapsed_ns"`
	RemainingNS    int64 `json:"remaining_ns"`
	WithinDeadline bool  `json:"within_deadline"`
}

func (e PrivateDiscoveryEventTime) valid() bool {
	if !e.Available {
		return e == (PrivateDiscoveryEventTime{})
	}
	return e.ElapsedNS >= 0 && e.RemainingNS >= 0 && (e.WithinDeadline == (e.RemainingNS > 0))
}

type PrivateDiscoveryDecodeOutcome uint8

const (
	PrivateDecodeNotAttempted PrivateDiscoveryDecodeOutcome = iota
	PrivateDecodeSucceeded
	PrivateDecodeRejected
	PrivateDecodeRefused
)

// DecodeAttempted means the admitted decoder was called and returned; Refused
// explicitly does not assert a completed schema scan or successful conversion.
type PrivateDiscoveryReadStages struct {
	FrameComplete   bool                          `json:"frame_complete"`
	Received        PrivateDiscoveryEventTime     `json:"received"`
	DecodeAttempted bool                          `json:"decode_attempted"`
	Decoded         PrivateDiscoveryEventTime     `json:"decoded"`
	Outcome         PrivateDiscoveryDecodeOutcome `json:"outcome"`
}

func (s PrivateDiscoveryReadStages) valid() bool {
	return s.Received.valid() && s.Decoded.valid() && s.Outcome <= PrivateDecodeRefused &&
		(s.FrameComplete || s.Received == (PrivateDiscoveryEventTime{})) && (!s.DecodeAttempted || s.FrameComplete) &&
		(s.DecodeAttempted == (s.Outcome != PrivateDecodeNotAttempted)) &&
		(s.DecodeAttempted || s.Decoded == (PrivateDiscoveryEventTime{}))
}

type PrivateDiscoveryClassifiedStatus uint8

const (
	PrivateClassificationUnknown PrivateDiscoveryClassifiedStatus = iota
	PrivateClassificationSuccess
	PrivateClassificationFailure
)

type PrivateDiscoveryClassification struct {
	Observed           bool                             `json:"observed"`
	At                 PrivateDiscoveryEventTime        `json:"at"`
	Status             PrivateDiscoveryClassifiedStatus `json:"status"`
	Cause              uint8                            `json:"cause"`
	ServerCodeCategory uint8                            `json:"server_code_category"`
}

func (c PrivateDiscoveryClassification) valid() bool {
	if !c.Observed {
		return c == (PrivateDiscoveryClassification{})
	}
	return c.At.valid() && (c.ServerCodeCategory == 0 || c.Cause == 3) && c.Status >= PrivateClassificationSuccess && c.Status <= PrivateClassificationFailure && c.Cause <= 7 && c.ServerCodeCategory <= 12 &&
		(c.Status != PrivateClassificationSuccess || c.Cause == 0 && c.ServerCodeCategory == 0)
}

// Explicit backing/copy and context-node allowance for at most 128 requests and
// 16 supplied-document digests, paid together with the compiler-sized store.
const privateDiscoveryContextCharge = uint64(16*1024+128*512) + 8*uint64(unsafe.Sizeof(PrivateDiscoverySessionContext{}))

func privateMethod(method string) PrivateDiscoveryMethod {
	switch method {
	case "textDocument/documentSymbol":
		return PrivateMethodDocumentSymbol
	case "textDocument/prepareCallHierarchy":
		return PrivateMethodPrepareCallHierarchy
	}
	return PrivateMethodUnknown
}

type privateDiscoveryCorrelationKey struct{}
type privateDiscoveryAdapterKey struct{}
type privateDiscoveryAdapterContext struct {
	owner        *PrivateDiscoverySession
	file, target int
}

// Optional source-adapter hook. No client interface changes, strings, request
// retargeting, URI parsing, or server-ID inference. Invalid inputs stay unknown.
// The returned adapter context is caller-owned, like caller-owned snapshots.
// Internal request contexts are separately admitted below before allocation.
func (s *PrivateDiscoverySession) DiscoveryDiagnosticContext(ctx context.Context, file, target int) context.Context {
	if s == nil || ctx == nil || file < 0 || file >= 16 || target < -1 || target >= 65536 {
		return ctx
	}
	return context.WithValue(ctx, privateDiscoveryAdapterKey{}, privateDiscoveryAdapterContext{s, file, target})
}
func (w *privateDiscoveryWindow) admitDiagnosticContext() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, 512}, privateDiscoveryCharge{&w.remaining.Work, 512})
}
func (s *PrivateDiscoverySession) diagnosticContext(ctx context.Context, uri string) context.Context {
	if !s.client.w.admitDiagnosticContext() {
		return s.client.w.ctx
	}
	c := PrivateDiscoveryCorrelation{}
	s.mu.Lock()
	for i, f := range s.members {
		if f.URI != uri || i >= len(s.digests) {
			continue
		}
		c.FileAvailable, c.FileOrdinal = true, int64(i)
		c.ContentAvailable, c.ContentSHA256 = s.digestAvailable[i], s.digests[i]
		if ctx != nil {
			if a, ok := ctx.Value(privateDiscoveryAdapterKey{}).(privateDiscoveryAdapterContext); ok && a.owner == s {
				c.FileOrdinal = int64(a.file)
				if a.target >= 0 {
					c.TargetAvailable, c.TargetOrdinal = true, int64(a.target)
				}
			}
		}
		break
	}
	s.mu.Unlock()
	return context.WithValue(s.client.w.ctx, privateDiscoveryCorrelationKey{}, c)
}

// Called while the document owner still holds its successful-result boundary.
// Ordinary results are unchanged. No fresh hash, map lookup, source read or
// session/native pointer is retained in this scalar handoff.
func privateDiscoverySupplyDigest(ctx context.Context, result *DocumentResult, digest [32]byte) {
	if privateDiscoveryFrom(ctx) != nil && result.Failure == "" && result.Supply != nil {
		result.privateDiscoveryDigest, result.privateDiscoveryDigestAvailable = digest, true
	}
}

// Caller holds s.mu. Copy the exact successful owner's result, not a later
// mutable document-map entry. Missing handoff stays unavailable, never zero.
func (s *PrivateDiscoverySession) captureSuppliedDigestLocked(uri string, result DocumentResult) {
	if !result.privateDiscoveryDigestAvailable || result.Failure != "" || result.Supply == nil {
		return
	}
	for i, member := range s.members {
		if member.URI == uri && i < len(s.digests) {
			s.digests[i] = result.privateDiscoveryDigest
			s.digestAvailable[i] = true
			return
		}
	}
}
func privateCorrelation(ctx context.Context, method string) PrivateDiscoveryCorrelation {
	c, _ := ctx.Value(privateDiscoveryCorrelationKey{}).(PrivateDiscoveryCorrelation)
	c.Method = privateMethod(method)
	if !c.valid() {
		return PrivateDiscoveryCorrelation{Method: c.Method}
	}
	return c
}
func privatePositionContext(ctx context.Context, p any) context.Context {
	c, _ := ctx.Value(privateDiscoveryCorrelationKey{}).(PrivateDiscoveryCorrelation)
	if p, ok := p.(lsp.PrepareCallHierarchyParams); ok {
		c.PositionAvailable = true
		c.Line, c.Character = int64(p.Position.Line), int64(p.Position.Character)
	}
	return context.WithValue(ctx, privateDiscoveryCorrelationKey{}, c)
}
func privateEventTime(anchor, deadline time.Time, elapsed time.Duration) PrivateDiscoveryEventTime {
	if anchor.IsZero() || deadline.IsZero() || elapsed < 0 || !deadline.After(anchor) {
		return PrivateDiscoveryEventTime{}
	}
	remaining := deadline.Sub(anchor) - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return PrivateDiscoveryEventTime{true, int64(elapsed), int64(remaining), remaining > 0}
}
func (w *privateDiscoveryWindow) eventTime() PrivateDiscoveryEventTime {
	if w.diagnosticAnchor.IsZero() {
		return PrivateDiscoveryEventTime{}
	}
	return privateEventTime(w.diagnosticAnchor, w.diagnosticDeadline, time.Since(w.diagnosticAnchor))
}
func (w *privateDiscoveryWindow) sentResponseFacts(i int) {
	at := w.eventTime()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.responseFacts != nil && i >= 0 && i < int(w.responseFacts.Count) {
		w.responseFacts.Records[i].Sent = true
		w.responseFacts.Records[i].SentAt = at
	}
}
func (w *privateDiscoveryWindow) classifyResponseFacts(i int, err error) {
	at := w.eventTime()
	c := PrivateDiscoveryClassification{Observed: true, At: at, Status: PrivateClassificationSuccess}
	if err != nil {
		c.Status = PrivateClassificationFailure
		cause, category := discoveryfailure.DetailsFromError(err)
		switch cause {
		case discoveryfailure.Canceled:
			c.Cause = 1
		case discoveryfailure.Deadline:
			c.Cause = 2
		case discoveryfailure.RPCServer:
			c.Cause = 3
		case discoveryfailure.SessionUnavailable:
			c.Cause = 4
		case discoveryfailure.SessionCrashed:
			c.Cause = 5
		case discoveryfailure.EnclosureBudget:
			c.Cause = 6
		case discoveryfailure.InvalidResponse:
			c.Cause = 7
		}
		c.ServerCodeCategory = uint8(category)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.responseFacts != nil && i >= 0 && i < int(w.responseFacts.Count) {
		w.responseFacts.Records[i].Classification = c
	}
}
func privateEncoding(value *string) (PrivateDiscoveryEncoding, PrivateDiscoveryEncodingOrigin) {
	if value == nil {
		return PrivateEncodingUTF16, PrivateEncodingProtocolDefault
	}
	e := PrivateEncodingUnsupported
	switch *value {
	case "utf-8":
		e = PrivateEncodingUTF8
	case "utf-16":
		e = PrivateEncodingUTF16
	case "utf-32":
		e = PrivateEncodingUTF32
	}
	return e, PrivateEncodingAdvertised
}

// This field decoder preserves legacy string/null acceptance while retaining
// actual field presence at the initialization boundary. Omission never invokes
// UnmarshalJSON. No server value escapes into the fixed diagnostic snapshot.
type privateInitializationEncoding struct {
	Value            string
	Present, Invalid bool
}

func (e *privateInitializationEncoding) UnmarshalJSON(raw []byte) error {
	e.Present = true
	trimmed := bytes.TrimSpace(raw)
	// Observe type before returning the original decoder error. No raw value
	// enters the fixed diagnostic carrier; JSON syntax is checked by Unmarshal
	// before it invokes this field decoder.
	e.Invalid = len(trimmed) == 0 || trimmed[0] != '"'
	return json.Unmarshal(raw, &e.Value)
}

// Same schema and decoder as the readiness boundary; no second parse. The
// window receives only observed field provenance on failure, not an invented
// protocol default when a syntax/type error prevented field observation.
type privateInitializationResult = struct {
	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Capabilities struct {
		PositionEncoding        privateInitializationEncoding `json:"positionEncoding"`
		CallHierarchyProvider   json.RawMessage               `json:"callHierarchyProvider"`
		DocumentSymbolProvider  json.RawMessage               `json:"documentSymbolProvider"`
		WorkspaceSymbolProvider json.RawMessage               `json:"workspaceSymbolProvider"`
		DefinitionProvider      json.RawMessage               `json:"definitionProvider"`
		ReferencesProvider      json.RawMessage               `json:"referencesProvider"`
	} `json:"capabilities"`
}

func decodePrivateInitialization(raw []byte, w *privateDiscoveryWindow) (initialized privateInitializationResult, err error) {
	err = json.Unmarshal(raw, &initialized)
	if w != nil && (err == nil || initialized.Capabilities.PositionEncoding.Present) {
		w.initializedEncoding(initialized.Capabilities.PositionEncoding)
	}
	return
}

func (w *privateDiscoveryWindow) initializedEncoding(value privateInitializationEncoding) {
	var v *string
	if value.Present {
		v = &value.Value
	}
	e, o := privateEncoding(v)
	if value.Invalid {
		e, o = PrivateEncodingUnsupported, PrivateEncodingInvalidField
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.responseFacts != nil {
		w.responseFacts.Context.Encoding = e
		w.responseFacts.Context.EncodingOrigin = o
	}
}
