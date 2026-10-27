// Package acquisitionmanifest owns authority-neutral acquisition manifest data
// and prelaunch validation. It does not execute or publish acquisitions.
package acquisitionmanifest

import (
	"fmt"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/operation"
)

const (
	Slice           operation.Name = "slice_v2"
	Incoming        operation.Name = "incoming_v2"
	SliceV3         operation.Name = "slice_v3"
	IncomingV3      operation.Name = "incoming_v3"
	ManifestVersion                = "lsp-trace.seed-manifest.v2"
	MaxInputBytes                  = 256 << 10
)

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
	// Expansion is output-contract gated by Execute. A bare manifest must retain
	// the frozen v2/v3 acquisition behavior.
	return r, acquisition.ValidateRequest(r)
}
