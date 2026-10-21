package censusrequest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

const SchemaVersionV2 = "lsp-trace.census-request-receipt.v2"

type SemanticFieldsV2 struct {
	Sources           []string `json:"sources"`
	Includes          []string `json:"includes"`
	Excludes          []string `json:"excludes"`
	PrunedDirectories []string `json:"pruned_directories"`
	DownDepth         uint64   `json:"down_depth"`
	UpDepth           uint64   `json:"up_depth"`
	MaxNodes          uint64   `json:"max_nodes"`
	BatchTargets      uint64   `json:"batch_targets"`
	TimeoutMS         uint64   `json:"timeout_ms"`
	RequestTimeoutMS  uint64   `json:"request_timeout_ms"`
	StopAfter         string   `json:"stop_after,omitempty"`
}

type ReceiptV2 struct {
	SchemaVersion string            `json:"schema_version"`
	Refresh       RefreshCoordinate `json:"refresh_required"`
	Semantic      SemanticFieldsV2  `json:"semantic"`
	Fingerprint   string            `json:"fingerprint"`
}

func NewV2(refresh RefreshCoordinate, semantic SemanticFieldsV2) (ReceiptV2, error) {
	if refresh.SessionID == "" || refresh.Generation == 0 || len(semantic.Sources) == 0 || len(semantic.PrunedDirectories) == 0 || hasBlank(semantic.Sources) || hasBlank(semantic.Includes) || hasBlank(semantic.Excludes) || hasBlank(semantic.PrunedDirectories) {
		return ReceiptV2{}, errors.New("invalid census request v2")
	}
	raw, err := json.Marshal(semantic)
	if err != nil {
		return ReceiptV2{}, err
	}
	digest := sha256.Sum256(raw)
	return ReceiptV2{SchemaVersion: SchemaVersionV2, Refresh: refresh, Semantic: semantic, Fingerprint: "sha256:" + hex.EncodeToString(digest[:])}, nil
}

func MarshalReceiptV2(receipt ReceiptV2) ([]byte, error) {
	if receipt.SchemaVersion != SchemaVersionV2 || receipt.Refresh.SessionID == "" || receipt.Refresh.Generation == 0 || len(receipt.Semantic.PrunedDirectories) == 0 {
		return nil, errors.New("invalid census request receipt v2")
	}
	raw, err := json.Marshal(receipt.Semantic)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	if receipt.Fingerprint != "sha256:"+hex.EncodeToString(digest[:]) {
		return nil, errors.New("invalid census request receipt v2 fingerprint")
	}
	return json.Marshal(receipt)
}
