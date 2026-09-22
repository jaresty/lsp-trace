// Package censusrequest owns the transport-neutral normalized census request receipt.
package censusrequest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"lsp-trace/internal/strictjson"
)

const SchemaVersion = "lsp-trace.census-request-receipt.v1"

const (
	DefaultDownDepth        uint64 = 1
	DefaultUpDepth          uint64 = 0
	DefaultMaxNodes         uint64 = 10000
	DefaultBatchTargets     uint64 = 63
	DefaultTimeoutMS        uint64 = 60000
	DefaultRequestTimeoutMS uint64 = 30000
)

func DefaultSources() []string { return []string{"."} }

type RefreshCoordinate struct {
	SessionID  string `json:"session_id"`
	Generation uint64 `json:"generation"`
}

type SemanticFields struct {
	Sources          []string `json:"sources"`
	Includes         []string `json:"includes"`
	Excludes         []string `json:"excludes"`
	DownDepth        uint64   `json:"down_depth"`
	UpDepth          uint64   `json:"up_depth"`
	MaxNodes         uint64   `json:"max_nodes"`
	BatchTargets     uint64   `json:"batch_targets"`
	TimeoutMS        uint64   `json:"timeout_ms"`
	RequestTimeoutMS uint64   `json:"request_timeout_ms"`
	StopAfter        string   `json:"stop_after,omitempty"`
}

type Receipt struct {
	SchemaVersion string            `json:"schema_version"`
	Refresh       RefreshCoordinate `json:"refresh_required"`
	Semantic      SemanticFields    `json:"semantic"`
	Fingerprint   string            `json:"fingerprint"`
	CanonicalJSON []byte            `json:"-"`
}

type wireRequest struct {
	SessionID        string   `json:"session_id"`
	Generation       uint64   `json:"generation"`
	Sources          []string `json:"sources"`
	Includes         []string `json:"includes,omitempty"`
	Excludes         []string `json:"excludes,omitempty"`
	DownDepth        *uint64  `json:"down_depth,omitempty"`
	UpDepth          *uint64  `json:"up_depth,omitempty"`
	MaxNodes         *uint64  `json:"max_nodes,omitempty"`
	BatchTargets     *uint64  `json:"batch_targets,omitempty"`
	TimeoutMS        *uint64  `json:"timeout_ms,omitempty"`
	RequestTimeoutMS *uint64  `json:"request_timeout_ms,omitempty"`
	StopAfter        string   `json:"stop_after,omitempty"`
}

type normalizedWire struct {
	SessionID        string   `json:"session_id"`
	Generation       uint64   `json:"generation"`
	Sources          []string `json:"sources"`
	Includes         []string `json:"includes"`
	Excludes         []string `json:"excludes"`
	DownDepth        uint64   `json:"down_depth"`
	UpDepth          uint64   `json:"up_depth"`
	MaxNodes         uint64   `json:"max_nodes"`
	BatchTargets     uint64   `json:"batch_targets"`
	TimeoutMS        uint64   `json:"timeout_ms"`
	RequestTimeoutMS uint64   `json:"request_timeout_ms"`
	StopAfter        string   `json:"stop_after,omitempty"`
}

func New(refresh RefreshCoordinate, semantic SemanticFields) (Receipt, error) {
	wire := normalizedWire{refresh.SessionID, refresh.Generation, clone(semantic.Sources), clone(semantic.Includes), clone(semantic.Excludes), semantic.DownDepth, semantic.UpDepth, semantic.MaxNodes, semantic.BatchTargets, semantic.TimeoutMS, semantic.RequestTimeoutMS, semantic.StopAfter}
	raw, err := json.Marshal(wire)
	if err != nil {
		return Receipt{}, err
	}
	return Decode(raw)
}

func Decode(raw []byte) (Receipt, error) {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return Receipt{}, errors.New("invalid census request JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var in wireRequest
	if err := dec.Decode(&in); err != nil {
		return Receipt{}, errors.New("invalid census request JSON")
	}
	if err := requireEOF(dec); err != nil {
		return Receipt{}, err
	}
	if strings.TrimSpace(in.SessionID) == "" || in.Generation == 0 || len(in.Sources) == 0 || hasBlank(in.Sources) || hasBlank(in.Includes) || hasBlank(in.Excludes) {
		return Receipt{}, errors.New("invalid census request")
	}
	semantic := SemanticFields{
		Sources: clone(in.Sources), Includes: clone(in.Includes), Excludes: clone(in.Excludes),
		DownDepth: value(in.DownDepth, DefaultDownDepth), UpDepth: value(in.UpDepth, DefaultUpDepth), MaxNodes: value(in.MaxNodes, DefaultMaxNodes), BatchTargets: value(in.BatchTargets, DefaultBatchTargets),
		TimeoutMS: value(in.TimeoutMS, DefaultTimeoutMS), RequestTimeoutMS: value(in.RequestTimeoutMS, DefaultRequestTimeoutMS), StopAfter: in.StopAfter,
	}
	if semantic.DownDepth > 64 || semantic.UpDepth > 64 || semantic.MaxNodes < 1 || semantic.MaxNodes > 10000 || semantic.BatchTargets < 1 || semantic.BatchTargets > 63 || semantic.TimeoutMS < 1 || semantic.TimeoutMS > 60000 || semantic.RequestTimeoutMS < 1 || semantic.RequestTimeoutMS > semantic.TimeoutMS || semantic.StopAfter != "" && semantic.StopAfter != "describe-requests" {
		return Receipt{}, errors.New("invalid census request bounds")
	}
	normalized := normalizedWire{in.SessionID, in.Generation, semantic.Sources, semantic.Includes, semantic.Excludes, semantic.DownDepth, semantic.UpDepth, semantic.MaxNodes, semantic.BatchTargets, semantic.TimeoutMS, semantic.RequestTimeoutMS, semantic.StopAfter}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return Receipt{}, err
	}
	meaning, err := json.Marshal(semantic)
	if err != nil {
		return Receipt{}, err
	}
	digest := sha256.Sum256(meaning)
	return Receipt{SchemaVersion: SchemaVersion, Refresh: RefreshCoordinate{in.SessionID, in.Generation}, Semantic: semantic, Fingerprint: "sha256:" + hex.EncodeToString(digest[:]), CanonicalJSON: canonical}, nil
}

func MarshalReceipt(receipt Receipt) ([]byte, error) {
	if receipt.SchemaVersion != SchemaVersion || receipt.Refresh.SessionID == "" || receipt.Refresh.Generation == 0 || receipt.Fingerprint == "" {
		return nil, errors.New("invalid census request receipt")
	}
	semantic, err := json.Marshal(receipt.Semantic)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(semantic)
	if receipt.Fingerprint != "sha256:"+hex.EncodeToString(digest[:]) {
		return nil, errors.New("invalid census request receipt fingerprint")
	}
	return json.Marshal(receipt)
}

func requireEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("trailing census request JSON")
	}
	return nil
}

func value(v *uint64, fallback uint64) uint64 {
	if v == nil {
		return fallback
	}
	return *v
}
func clone(v []string) []string {
	out := make([]string, len(v))
	copy(out, v)
	return out
}
func hasBlank(v []string) bool {
	for _, s := range v {
		if strings.TrimSpace(s) == "" {
			return true
		}
	}
	return false
}
