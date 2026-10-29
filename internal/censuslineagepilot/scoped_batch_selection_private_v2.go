package censuslineagepilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/sessionruntime"
)

// Only origin.Batch returns these handles. Value copies SHARE the original
// authority and spending state. JSON cannot reconstruct executable provenance.
type ScopeBatchSelectionPrivateV2 struct{ state *batchSelectionStatePrivateV2 }
type batchSelectionStatePrivateV2 struct {
	self                     *batchSelectionStatePrivateV2
	mu                       sync.Mutex
	spent                    bool
	failure                  error
	origin                   *enumerationOriginStatePrivateV2
	ordinal                  int
	binding                  acquisitionauthority.Binding
	seeds                    []byte
	authority                acquisitionauthority.SeedAuthority
	beforeReleasePrivateTest func(directionalraw.Reservation)         // nil except owned release-failure fixture
	afterClientPrivateTest   func(*scopedBatchOperationClientPrivate) // nil except owned revocation fixture
}

func (ScopeBatchSelectionPrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (o EnumerationOriginPrivateV2) Batch(ordinal int) (ScopeBatchSelectionPrivateV2, error) {
	if !o.valid() || ordinal < 0 || ordinal >= len(o.state.batches) {
		return ScopeBatchSelectionPrivateV2{}, ErrInvalid
	}
	s := o.state.batches[ordinal]
	if s == nil || s.self != s || s.ordinal != ordinal || s.origin != o.state {
		return ScopeBatchSelectionPrivateV2{}, ErrInvalid
	}
	return ScopeBatchSelectionPrivateV2{s}, nil
}
func scopeBatchRequestPrivateV2(a *scopeRuntimeAdapter, b acquisitionauthority.Binding) (acquisition.Request, error) {
	expected, err := validateScopedBatchBindingPrivate(a, b)
	if err != nil {
		return acquisition.Request{}, err
	}
	metadata, failure := a.manager.Metadata(b.SessionID, b.Generation)
	if failure != "" {
		return acquisition.Request{}, scopeAdapterFailure(failure, "")
	}
	expected.Context = acquisition.AcquisitionContext{SessionID: b.SessionID, Generation: b.Generation, PositionEncoding: metadata.PositionEncoding}
	raw, _ := json.Marshal(expected)
	expected.Context.ID = fmt.Sprintf("public-acquisition-v2:%x", sha256.Sum256(raw))
	return expected, nil
}

// This entry accepts no DTO, new authority, callback, permit, or client. The
// restricted V5-labelled input pin is reused as neutral seed-manifest validation
// only: execution NEVER enters the ordinary V5 engine/admission route.
func AcquireScopeBatchPrivateV2(ctx context.Context, m *sessionruntime.Manager, o EnumerationOriginPrivateV2, b ScopeBatchSelectionPrivateV2) (OpaqueSuccessfulV2Capture, error) {
	if b.state == nil || !o.valid() {
		return OpaqueSuccessfulV2Capture{}, ErrInvalid
	}
	req, err := scopeBatchRequestPrivateV2(o.state.adapter, b.state.binding)
	if err != nil {
		return OpaqueSuccessfulV2Capture{}, err
	}
	return acquireScopeBatchPrivateV2(ctx, m, o, b, b.state.binding, req)
}

// Package-private actual-binding seam exists for negative owned tests, not minting.
func acquireScopeBatchPrivateV2(ctx context.Context, m *sessionruntime.Manager, o EnumerationOriginPrivateV2, b ScopeBatchSelectionPrivateV2, actual acquisitionauthority.Binding, req acquisition.Request) (out OpaqueSuccessfulV2Capture, err error) {
	if ctx == nil || m == nil || !o.valid() || b.state == nil || b.state.self != b.state {
		return out, ErrInvalid
	}
	if _, nested := directionalraw.FromContext(ctx); nested {
		return out, ErrInvalid
	}
	s := b.state
	s.mu.Lock()
	locked := true
	defer func() {
		if locked {
			s.mu.Unlock()
		}
	}()
	a := o.state.adapter
	if s.spent || s.origin != o.state || s.ordinal < 0 || s.ordinal >= len(o.state.batches) || o.state.batches[s.ordinal] != s || a.manager != m || a.guard != nil || a.guardV2 == nil || !a.frozenV2.valid() || a.lifetimeV2 != o.state.lifetime || !sameScopedBatchBindingPrivate(actual, s.binding) {
		return out, ErrInvalid
	}
	if len(actual.Input) == 0 || len(actual.Input) > 128<<10 {
		return out, ErrInvalid
	}
	expected, err := scopeBatchRequestPrivateV2(a, s.binding)
	if err != nil {
		return out, err
	}
	if !reflect.DeepEqual(expected, req) || m.SeedBindingRequested(actual.SessionID, actual.Generation) {
		return out, ErrInvalid
	}
	if err = a.check(o.state.lifetime); err != nil {
		return out, err
	}
	if err = a.check(ctx); err != nil {
		return out, err
	}
	// All capability and query-scope preflight happens BEFORE touching the original
	// authority. This also applies the stricter source-capture byte bounds.
	totalSource := 0
	for _, file := range a.frozenV2.state.files {
		totalSource += len(file.Bytes)
		if len(file.Bytes) > 1<<20 || totalSource > 4<<20 {
			return out, ErrInvalid
		}
	}
	bound, err := graphprovenance.BindSourcePrivateV2(ctx, m, a.guardV2, actual.SessionID, actual.Generation, actual.Workspace)
	if err != nil {
		return out, err
	}
	declaration, err := bound.FrozenQueryDeclarationPrivateV2(ctx)
	if err != nil {
		return out, err
	}
	if !reflect.DeepEqual(declaration.MembersPrivateV2(), o.state.queryURIs) {
		return out, ErrInvalid
	}
	expected, err = expected.WithFrozenQueryDeclarationPrivateV2(declaration)
	if err != nil {
		return out, err
	}
	if err = a.check(ctx); err != nil {
		return out, err
	}
	reservation := directionalraw.NewReservation(actual.RequestID)
	if reservation.IdentityPrivate() != actual.RequestID || reservation.Closed() || reservation.Attached() {
		reservation.Release()
		return out, ErrInvalid
	}
	// Exhaust first: even a Consume/grant comparison failure cannot make this
	// selection replayable. There is no replacement authority, reset, or retry.
	s.spent = true
	grant, err := acquisitionauthority.ConsumeSeedAuthority(s.authority, actual)
	if err == nil && (grant.Provenance != seedbinding.CallerAssertedLocal || grant.Authenticated || grant.HostReceiptID != "" || !grant.RetainSeedSpec || !bytes.Equal(grant.SeedSpec, s.seeds)) {
		err = ErrInvalid
	}
	if err != nil {
		s.failure = err
		s.mu.Unlock()
		locked = false
		reservation.Release()
		return out, err
	}
	s.mu.Unlock()
	locked = false
	releaseAttempted := false
	defer func() {
		if err != nil {
			s.mu.Lock()
			s.failure = err
			s.mu.Unlock()
		}
		if !releaseAttempted {
			reservation.Release()
		}
	}()
	opCtx, cancel := context.WithCancel(reservation.Context(ctx))
	stop := context.AfterFunc(o.state.lifetime, cancel)
	defer stop()
	defer cancel()
	client := &scopedBatchOperationClientPrivate{adapter: a, client: a.nativeDirectionalClientPrivate(), reservation: reservation, operation: opCtx, lifetime: o.state.lifetime, ac: expected.Context}
	client.active.Store(true)
	defer client.active.Store(false)
	if s.afterClientPrivateTest != nil {
		s.afterClientPrivateTest(client)
	}
	result, err := acquisition.Acquire(opCtx, client, expected)
	if err == nil {
		err = client.check(opCtx)
	}
	var artifact []byte
	if err == nil {
		if a.beforePartialCapturePrivateTest != nil {
			a.beforePartialCapturePrivateTest(&result)
		}
		artifact, err = graphprovenance.CaptureSourceWithholdingPrivateV2(opCtx, result, bound)
	}
	if err == nil {
		err = client.check(opCtx)
	}
	client.active.Store(false)
	if s.beforeReleasePrivateTest != nil {
		s.beforeReleasePrivateTest(reservation)
	}
	attached := reservation.Attached()
	releaseAttempted = true
	released := reservation.Release()
	if err == nil && (!attached || !released) {
		err = ErrInvalid
	}
	if err == nil {
		err = a.check(opCtx)
	}
	if err != nil {
		return OpaqueSuccessfulV2Capture{}, err
	}
	frame := captureOriginFramePrivateV2{Schema: "lsp-trace.capture-origin.private.v2", EnumerationOrigin: json.RawMessage(o.state.raw), EnumerationOriginSHA256: Digest(o.state.raw), Ordinal: s.ordinal, SeedSHA256: Digest(s.seeds), SeedByteLength: uint64(len(s.seeds)), ArtifactSHA256: Digest(artifact), ArtifactByteLength: uint64(len(artifact)), Source: "UNKNOWN", Feature: "UNRESOLVED", Absence: "PROHIBITED"}
	raw, err := json.Marshal(frame)
	if err != nil || len(raw) > scopeMetadataPrivateV2 {
		return OpaqueSuccessfulV2Capture{}, ErrInvalid
	}
	if err = a.check(opCtx); err != nil {
		return OpaqueSuccessfulV2Capture{}, err
	}
	st := &successfulCaptureStatePrivateV2{origin: raw, seeds: bytes.Clone(s.seeds), artifact: bytes.Clone(artifact)}
	st.self = st
	return OpaqueSuccessfulV2Capture{st}, nil
}

type captureOriginFramePrivateV2 struct {
	Schema                  string          `json:"schema"`
	EnumerationOrigin       json.RawMessage `json:"enumeration_origin"`
	EnumerationOriginSHA256 string          `json:"enumeration_origin_sha256"`
	Ordinal                 int             `json:"batch_ordinal"`
	SeedSHA256              string          `json:"seed_sha256"`
	SeedByteLength          uint64          `json:"seed_byte_length"`
	ArtifactSHA256          string          `json:"artifact_sha256"`
	ArtifactByteLength      uint64          `json:"artifact_byte_length"`
	Authority               uint8           `json:"authority"`
	Accepted                bool            `json:"accepted"`
	Source                  string          `json:"source_completeness"`
	Feature                 string          `json:"feature_identity"`
	Absence                 string          `json:"absence_inference"`
}

// Ephemeral producer lineage ONLY. Not publication, retention, qualification,
// authenticated history, source-analysis authority, or ordinary admission.
type OpaqueSuccessfulV2Capture struct {
	state *successfulCaptureStatePrivateV2
}
type successfulCaptureStatePrivateV2 struct {
	self                    *successfulCaptureStatePrivateV2
	origin, seeds, artifact []byte
}

func (c OpaqueSuccessfulV2Capture) valid() bool                { return c.state != nil && c.state.self == c.state }
func (OpaqueSuccessfulV2Capture) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (c OpaqueSuccessfulV2Capture) OriginBytes() []byte {
	if !c.valid() {
		return nil
	}
	return bytes.Clone(c.state.origin)
}
func (c OpaqueSuccessfulV2Capture) SeedBytes() []byte {
	if !c.valid() {
		return nil
	}
	return bytes.Clone(c.state.seeds)
}
func (c OpaqueSuccessfulV2Capture) ArtifactBytes() []byte {
	if !c.valid() {
		return nil
	}
	return bytes.Clone(c.state.artifact)
}
func (c OpaqueSuccessfulV2Capture) ArtifactDigest() string {
	if !c.valid() {
		return ""
	}
	return Digest(c.state.artifact)
}
func (c OpaqueSuccessfulV2Capture) ArtifactByteLength() uint64 {
	if !c.valid() {
		return 0
	}
	return uint64(len(c.state.artifact))
}
