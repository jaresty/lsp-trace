package sourceprojection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"lsp-trace/sessionruntime"
)

type LiveExpectation struct {
	SessionID        string
	Generation       uint64
	URI              string
	PositionEncoding string
}

type LiveBinding struct {
	Custody          string `json:"custody"`
	SessionID        string `json:"session_id"`
	Generation       uint64 `json:"generation"`
	URI              string `json:"uri"`
	DocumentVersion  int    `json:"document_version"`
	PositionEncoding string `json:"position_encoding"`
	SourceDigest     string `json:"source_digest"`
	SourceByteLength int    `json:"source_byte_length"`
}

type LiveResolution struct {
	Source               Source
	Binding              LiveBinding
	PhysicalProjectionID string
}

type WireResult struct {
	SchemaVersion        string         `json:"schema_version"`
	Authority            int            `json:"authority"`
	SourceGraphComplete  string         `json:"source_graph_complete"`
	GraphFactsAdded      int            `json:"graph_facts_added"`
	CustodyMode          string         `json:"custody_mode"`
	CustodyBinding       LiveBinding    `json:"custody_binding"`
	PhysicalProjectionID string         `json:"physical_projection_id"`
	RequestPolicyID      string         `json:"request_policy_id"`
	Status               string         `json:"status"`
	Units                []Unit         `json:"units"`
	Citations            []Citation     `json:"citations"`
	EmittedSpans         []Span         `json:"emitted_spans"`
	Accounting           Accounting     `json:"accounting"`
	Omissions            []Omission     `json:"omissions"`
	PrivacySummary       PrivacySummary `json:"privacy_summary"`
}

var ErrLiveResolverNotImplemented = errors.New("live source resolver not implemented")
var ErrLiveAssemblyNotImplemented = errors.New("live source projection assembly not implemented")

func AssembleLiveBounded(resolution LiveResolution, requestPolicyID string, core Result, maxResponseBytes int) (WireResult, error) {
	if maxResponseBytes < 1 {
		return WireResult{}, errors.New("live source projection response limit invalid")
	}
	result, err := AssembleLive(resolution, requestPolicyID, core)
	if err != nil {
		return WireResult{}, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return WireResult{}, fmt.Errorf("live source projection response: %w", err)
	}
	if len(raw) > maxResponseBytes {
		return WireResult{}, fmt.Errorf("live source projection response limit: need %d have %d", len(raw), maxResponseBytes)
	}
	return result, nil
}

func AssembleLive(resolution LiveResolution, requestPolicyID string, core Result) (WireResult, error) {
	if resolution.Binding.Custody != "LIVE" || resolution.PhysicalProjectionID == "" || len(requestPolicyID) != len("sha256:")+64 || requestPolicyID[:len("sha256:")] != "sha256:" {
		return WireResult{}, errors.New("live source projection assembly binding invalid")
	}
	return WireResult{
		SchemaVersion:        SchemaVersion,
		Authority:            0,
		SourceGraphComplete:  "UNKNOWN",
		GraphFactsAdded:      0,
		CustodyMode:          "LIVE",
		CustodyBinding:       resolution.Binding,
		PhysicalProjectionID: resolution.PhysicalProjectionID,
		RequestPolicyID:      requestPolicyID,
		Status:               core.Status,
		Units:                core.Units,
		Citations:            core.Citations,
		EmittedSpans:         core.EmittedSpans,
		Accounting:           core.Accounting,
		Omissions:            core.Omissions,
		PrivacySummary:       core.PrivacySummary,
	}, nil
}

func ResolveLive(supply *sessionruntime.DocumentSupply, expected LiveExpectation) (LiveResolution, error) {
	if supply == nil {
		return LiveResolution{}, errors.New("live document supply absent")
	}
	if supply.Classification != "LSP_SUPPLIED" || expected.SessionID == "" || expected.Generation == 0 || expected.URI == "" || expected.PositionEncoding != "utf-16" {
		return LiveResolution{}, errors.New("live document supply policy mismatch")
	}
	if supply.SessionID != expected.SessionID || supply.Generation != expected.Generation || supply.URI != expected.URI || supply.DocumentVersion < 1 {
		return LiveResolution{}, errors.New("live document supply identity mismatch")
	}
	if supply.Method != "textDocument/didOpen" && supply.Method != "textDocument/didChange" {
		return LiveResolution{}, errors.New("live document supply method invalid")
	}
	if len(supply.Content) > sessionruntime.MaxDocumentSupplyBytes || !utf8.Valid(supply.Content) {
		return LiveResolution{}, errors.New("live document supply content invalid")
	}
	content := append([]byte(nil), supply.Content...)
	sourceDigest := digest(content)
	binding := LiveBinding{
		Custody:          "LIVE",
		SessionID:        supply.SessionID,
		Generation:       supply.Generation,
		URI:              supply.URI,
		DocumentVersion:  supply.DocumentVersion,
		PositionEncoding: expected.PositionEncoding,
		SourceDigest:     sourceDigest,
		SourceByteLength: len(content),
	}
	preimage := struct {
		Custody           string `json:"custody"`
		SessionID         string `json:"session_id"`
		Generation        uint64 `json:"generation"`
		URI               string `json:"uri"`
		DocumentVersion   int    `json:"document_version"`
		ContentSHA256     string `json:"content_sha256"`
		ContentByteLength int    `json:"content_byte_length"`
	}{"live", supply.SessionID, supply.Generation, supply.URI, supply.DocumentVersion, sourceDigest, len(content)}
	raw, err := json.Marshal(preimage)
	if err != nil {
		return LiveResolution{}, fmt.Errorf("live physical identity: %w", err)
	}
	return LiveResolution{
		Source:  Source{LogicalSourceID: supply.URI, Digest: sourceDigest, Bytes: content, Available: true},
		Binding: binding, PhysicalProjectionID: digest(raw),
	}, nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
