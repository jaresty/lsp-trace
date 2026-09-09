// Package acquisitionops is the shared public v2 managed acquisition boundary.
// Legacy incoming/slice operation contracts are deliberately not extended.
package acquisitionops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisition/sessionclient"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/session"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const (
	Slice           operation.Name = "slice_v2"
	Incoming        operation.Name = "incoming_v2"
	SliceV3         operation.Name = "slice_v3"
	IncomingV3      operation.Name = "incoming_v3"
	ManifestVersion                = "lsp-trace.seed-manifest.v2"
	MaxInputBytes                  = 256 << 10
)

type Runtime interface {
	Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure)
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
	Records() []sessionruntime.Record
}
type Executor struct{ runtime Runtime }

type CustodyReceipt struct {
	Provenance    seedbinding.CustodyMode `json:"provenance"`
	Authenticated bool                    `json:"authenticated"`
	HostReceiptID string                  `json:"host_receipt_id,omitempty"`
}

func NewExecutor(r Runtime) *Executor { return &Executor{runtime: r} }

type Target struct {
	ID        string              `json:"id"`
	Locator   acquisition.Locator `json:"locator"`
	DownDepth *int                `json:"down_depth,omitempty"`
	UpDepth   *int                `json:"up_depth,omitempty"`
}
type Limits struct {
	MaxNodes         *int `json:"max_nodes,omitempty"`
	MaxRequests      *int `json:"max_requests,omitempty"`
	MaxEvidenceBytes *int `json:"max_evidence_bytes,omitempty"`
	MaxPathWork      *int `json:"max_path_work,omitempty"`
	TimeoutMS        *int `json:"timeout_ms,omitempty"`
	RequestTimeoutMS *int `json:"request_timeout_ms,omitempty"`
	MaxResponseBytes *int `json:"max_response_bytes,omitempty"`
	MaxMessages      *int `json:"max_messages,omitempty"`
}
type Expansion struct {
	TopmostSiblings bool `json:"topmost_siblings"`
}
type Manifest struct {
	SchemaVersion        string    `json:"schema_version"`
	CoordinateConvention string    `json:"coordinate_convention"`
	Root                 Target    `json:"root"`
	RequiredTargets      []Target  `json:"required_targets"`
	Limits               Limits    `json:"limits,omitempty"`
	Expansion            Expansion `json:"expansion,omitempty"`
}
type Input struct {
	SessionID     string   `json:"session_id"`
	Generation    uint64   `json:"generation"`
	SeedManifest  Manifest `json:"seed_manifest"`
	OutputVersion string   `json:"output_version,omitempty"`
}

// DecodeManifest applies the same closed, bounded decoding used by MCP. It reads
// bytes only; the CLI owns file access and MCP never accepts a manifest filepath.
func DecodeManifest(raw []byte, mode operation.Name) (Manifest, error) {
	var m Manifest
	if err := decode(raw, &m); err != nil {
		return m, err
	}
	_, err := m.Request(mode)
	return m, err
}
func decode(raw []byte, v any) error {
	if len(raw) > MaxInputBytes {
		return fmt.Errorf("acquisition input exceeds %d bytes", MaxInputBytes)
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return fmt.Errorf("object required")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("one JSON object required")
	}
	// Null is not omission: rejecting it also preserves explicit coordinate and
	// count presence through pointer decoding.
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	var check func(any) error
	check = func(v any) error {
		switch x := v.(type) {
		case nil:
			return fmt.Errorf("null acquisition member is not permitted")
		case map[string]any:
			// Preserve present-versus-omitted selector semantics before typed
			// decoding can turn an empty string into an omitted selector.
			for _, key := range []string{"symbol", "language_id"} {
				if value, present := x[key]; present && value == "" {
					return fmt.Errorf("%s must not be empty when present", key)
				}
			}
			for _, v := range x {
				if err := check(v); err != nil {
					return err
				}
			}
		case []any:
			for _, v := range x {
				if err := check(v); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return check(value)
}
func val(p *int, fallback int) int {
	if p != nil {
		return *p
	}
	return fallback
}

// Request is a prelaunch normalizer. Its context is a validation placeholder,
// replaced only by exact host metadata in Execute, never caller authority.
func (m Manifest) Request(mode operation.Name) (acquisition.Request, error) {
	r := acquisition.Request{Context: acquisition.AcquisitionContext{ID: "preflight", SessionID: "preflight", Generation: 1, PositionEncoding: "utf-16"}}
	switch mode {
	case Slice, SliceV3:
		r.Mode = acquisition.Slice
	case Incoming, IncomingV3:
		r.Mode = acquisition.Incoming
	default:
		return r, fmt.Errorf("unsupported acquisition operation %q", mode)
	}
	if m.SchemaVersion != ManifestVersion {
		return r, fmt.Errorf("unsupported seed manifest version %q", m.SchemaVersion)
	}
	if m.CoordinateConvention != "zero-based-session" {
		return r, fmt.Errorf("coordinate_convention must be zero-based-session")
	}
	if m.RequiredTargets == nil {
		return r, fmt.Errorf("required_targets array is required (may be empty)")
	}
	target := func(t Target) acquisition.Target {
		return acquisition.Target{ID: t.ID, Locator: t.Locator, DownDepth: val(t.DownDepth, 2), UpDepth: val(t.UpDepth, 2)}
	}
	r.Root = target(m.Root)
	r.RequiredTargets = make([]acquisition.Target, len(m.RequiredTargets))
	for i, t := range m.RequiredTargets {
		r.RequiredTargets[i] = target(t)
	}
	l := m.Limits
	timeout, requestTimeout := val(l.TimeoutMS, 5000), val(l.RequestTimeoutMS, 1000)
	if timeout < 1 || timeout > 60000 || requestTimeout < 1 || requestTimeout > 60000 {
		return r, fmt.Errorf("timeouts must be 1..60000 milliseconds")
	}
	r.Limits = acquisition.Limits{MaxNodes: val(l.MaxNodes, 100), MaxRequests: val(l.MaxRequests, 1000), MaxEvidenceBytes: val(l.MaxEvidenceBytes, 4<<20), MaxPathWork: val(l.MaxPathWork, 100000), Timeout: time.Duration(timeout) * time.Millisecond, RequestTimeout: time.Duration(requestTimeout) * time.Millisecond, MaxResponseBytes: val(l.MaxResponseBytes, 4<<20), MaxMessages: val(l.MaxMessages, 64)}
	r.TopmostSiblings = m.Expansion.TopmostSiblings
	return r, acquisition.ValidateRequest(r)
}

func (e *Executor) Execute(ctx context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	fail := func(code string, err error) (operation.Result, *operation.Failure) {
		return operation.Result{}, &operation.Failure{Code: code, Err: err}
	}
	if op.Name != Slice && op.Name != Incoming && op.Name != SliceV3 && op.Name != IncomingV3 {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var in Input
	if err := decode(op.Input, &in); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	req, err := in.SeedManifest.Request(op.Name)
	if err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	if in.SessionID == "" || in.Generation == 0 {
		return fail(operation.FailureInvalidInput, fmt.Errorf("session_id and exact generation are required"))
	}
	if e == nil || e.runtime == nil {
		return fail(operation.FailureInternal, fmt.Errorf("managed runtime required"))
	}
	id, generation, failure := incomingops.ResolveSession(e.runtime, in.SessionID, in.Generation)
	if failure != "" {
		return fail(string(failure), nil)
	}
	metadata, failure := e.runtime.Metadata(id, generation)
	if failure != "" {
		return fail(string(failure), nil)
	}
	workspace := ""
	for _, record := range e.runtime.Records() {
		if record.SessionID == id && record.Generation == generation {
			if workspace != "" {
				return fail(operation.FailureInternal, fmt.Errorf("ambiguous host workspace"))
			}
			workspace = record.Profile.Workspace().String()
		}
	}
	if workspace == "" {
		return fail(operation.FailureInternal, fmt.Errorf("host workspace unavailable"))
	}
	if _, ok := e.runtime.(interface {
		PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
	}); !ok {
		return fail(operation.FailureInternal, fmt.Errorf("managed document supply unavailable"))
	}
	req.Context = acquisition.AcquisitionContext{SessionID: id, Generation: generation, PositionEncoding: metadata.PositionEncoding}
	// Stable request/context identity, not authenticated acquisition-event identity.
	identity, _ := json.Marshal(req)
	req.Context.ID = fmt.Sprintf("public-acquisition-v2:%x", sha256.Sum256(identity))
	if err := acquisition.ValidateRequest(req); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	if binding, ok := e.runtime.(interface {
		SeedBindingRequested(string, uint64) bool
		AdmitSeedBinding(context.Context, string, uint64, time.Time, int, int64) seedbinding.ValidationResult
	}); ok && binding.SeedBindingRequested(id, generation) {
		if req.Limits.MaxRequests < 1 {
			return fail(string(session.ResourceExhausted), fmt.Errorf("semantic seed admission requires one shared acquisition request"))
		}
		deadline := time.Now().Add(req.Limits.RequestTimeout)
		admission := binding.AdmitSeedBinding(ctx, id, generation, deadline, req.Limits.MaxMessages, int64(req.Limits.MaxResponseBytes))
		req.Limits.MaxRequests--
		switch admission.Status {
		case seedbinding.Match:
		case seedbinding.Mismatch:
			return fail(seedbinding.BindingMismatch, nil)
		case seedbinding.Unavailable:
			return fail(seedbinding.BindingUnavailable, nil)
		default:
			return fail("SEED_BINDING_INVALID", nil)
		}
	}
	var custodyReceipt *CustodyReceipt
	if provenanceRuntime, ok := e.runtime.(interface {
		SeedCustodyProvenance(string, uint64) (seedbinding.CustodyMode, bool)
	}); ok {
		if provenance, found := provenanceRuntime.SeedCustodyProvenance(id, generation); found {
			if provenance != seedbinding.VerifiedHost && provenance != seedbinding.CallerAssertedLocal {
				return fail("OUTPUT_VALIDATION_FAILED", fmt.Errorf("invalid retained seed custody provenance"))
			}
			custodyReceipt = &CustodyReceipt{Provenance: provenance, Authenticated: provenance == seedbinding.VerifiedHost}
		}
	}
	wantsV5 := in.OutputVersion == graphprovenance.VersionV5
	if in.OutputVersion != "" && !wantsV5 {
		return fail(operation.FailureInvalidInput, fmt.Errorf("unsupported output_version %q", in.OutputVersion))
	}
	if wantsV5 && ((op.Name != SliceV3 && op.Name != IncomingV3) || !in.SeedManifest.Expansion.TopmostSiblings) {
		return fail(operation.FailureInvalidInput, fmt.Errorf("graph-provenance v5 requires current v3 route and expansion.topmost_siblings=true"))
	}
	result, err := acquisition.Acquire(ctx, sessionclient.New(e.runtime), req)
	if err != nil {
		return fail("OUTPUT_VALIDATION_FAILED", err)
	}
	siblings := result.Graph.SiblingCandidates
	raw, err := graphprovenance.CaptureV2(ctx, result, workspace)
	if err != nil {
		return fail("OUTPUT_VALIDATION_FAILED", err)
	}
	if op.Name == SliceV3 || op.Name == IncomingV3 {
		query := manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}
		if diagnostics, ok := e.runtime.(interface {
			Diagnostics(string, uint64) manageddiagnostic.QueryResult
		}); ok {
			query = diagnostics.Diagnostics(id, generation)
		}
		if wantsV5 {
			var carrier struct {
				GraphBytes []byte `json:"graph_bytes"`
			}
			if unmarshalErr := json.Unmarshal(raw, &carrier); unmarshalErr != nil {
				return fail("OUTPUT_VALIDATION_FAILED", unmarshalErr)
			}
			enriched, decodeErr := graph.DecodeNativeV3(carrier.GraphBytes)
			if decodeErr != nil {
				return fail("OUTPUT_VALIDATION_FAILED", decodeErr)
			}
			enriched.SchemaVersion = graph.SchemaVersionV5
			enriched.Invocation.Expansion.TopmostSiblings = true
			enriched.SiblingCandidates = siblings
			if len(enriched.SiblingCandidates) == 0 {
				return fail("OUTPUT_VALIDATION_FAILED", fmt.Errorf("topmost sibling expansion produced no exact relations"))
			}
			seeds := map[string]graph.InvocationSeed{}
			for _, seed := range enriched.Invocation.Seeds {
				seeds[seed.Label] = seed
			}
			for i := range enriched.SiblingCandidates {
				candidate := &enriched.SiblingCandidates[i]
				seed := seeds[candidate.SeedLabel]
				candidate.SeedIdentity = enriched.Invocation.Provenance.InvocationID + ":" + seed.Label + ":" + seed.At
				candidate.ProviderEvidence = []string{fmt.Sprintf("command=%s;server_version=%s;invocation=%s", enriched.Invocation.Server.Command, enriched.Invocation.Provenance.ServerVersion, enriched.Invocation.Provenance.InvocationID)}
				candidate.SourceDigests = []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}
				candidate.Custody = graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: seed.ContentSHA256, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}
				if custodyReceipt != nil && custodyReceipt.Provenance == seedbinding.VerifiedHost {
					receiptRuntime, ok := e.runtime.(interface {
						SeedCustodyReceipt(string, uint64) (string, bool)
					})
					if !ok {
						return fail("OUTPUT_VALIDATION_FAILED", fmt.Errorf("verified host custody requires an exact host receipt identifier"))
					}
					receiptID, found := receiptRuntime.SeedCustodyReceipt(id, generation)
					if !found || receiptID == "" {
						return fail("OUTPUT_VALIDATION_FAILED", fmt.Errorf("verified host custody requires an exact host receipt identifier"))
					}
					custodyReceipt.HostReceiptID = receiptID
					candidate.Custody.Class = graph.SourceCustodyVerifiedHost
					candidate.Custody.HostReceiptID = receiptID
				}
				if custodyReceipt == nil {
					candidate.Custody.Class = graph.SourceCustodyMissing
					candidate.Custody.SourceContentSHA256 = ""
				}
			}
			enriched.Canonicalize()
			native, marshalErr := json.Marshal(enriched)
			if marshalErr != nil {
				return fail("OUTPUT_VALIDATION_FAILED", marshalErr)
			}
			raw, err = graphprovenance.CaptureV5(native, id, generation, query)
		} else {
			raw, err = graphprovenance.CaptureV3(raw, id, generation, query)
		}
		if err != nil {
			return fail("OUTPUT_VALIDATION_FAILED", err)
		}
	}
	opResult := operation.Result{Artifact: raw}
	if custodyReceipt != nil {
		opResult.CustodyReceipt = custodyReceipt
	}
	return opResult, nil
}
