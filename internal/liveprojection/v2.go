package liveprojection

import (
	"errors"
	"strings"

	"lsp-trace/internal/sourceprojectionv2"
)

const V2SchemaVersion = sourceprojectionv2.SchemaVersion

type V2LiveBinding struct {
	Custody          string `json:"custody"`
	SessionID        string `json:"session_id"`
	Generation       uint64 `json:"generation"`
	PositionEncoding string `json:"position_encoding"`
}

type V2DocumentSelection = sourceprojectionv2.DocumentSelection
type V2DocumentBinding = sourceprojectionv2.DocumentBinding
type V2DocumentAccounting = sourceprojectionv2.DocumentAccounting
type V2DisplayProvenance = sourceprojectionv2.DisplayProvenance
type V2Unit = sourceprojectionv2.Unit
type V2Citation = sourceprojectionv2.Citation
type V2WireResult = sourceprojectionv2.WireResult[V2LiveBinding]

func AssembleV2Bounded(composed CompositionResult, prepared PreparationResult, targetURI string, selectedURIs []string, requestPolicyID string, maxResponseBytes int) (V2WireResult, error) {
	if composed.Status != CompositionComplete || prepared.Status != PreparationComplete {
		return V2WireResult{}, errors.New("liveprojection: V2 assembly requires complete composition and preparation")
	}
	if targetURI == "" || len(selectedURIs) == 0 || selectedURIs[0] != targetURI {
		return V2WireResult{}, errors.New("liveprojection: V2 document selection must be target-first")
	}
	documents := make([]sourceprojectionv2.DocumentSource, 0, len(composed.Resolutions))
	for _, resolution := range composed.Resolutions {
		binding := resolution.Binding
		documents = append(documents, sourceprojectionv2.DocumentSource{
			URI: binding.URI, DocumentVersion: binding.DocumentVersion, PositionEncoding: binding.PositionEncoding,
			SourceDigest: binding.SourceDigest, SourceByteLength: binding.SourceByteLength,
		})
	}
	binding := composed.Resolutions[0].Binding
	result, err := sourceprojectionv2.AssembleBounded(sourceprojectionv2.Input{
		TargetURI: targetURI, SelectedURIs: selectedURIs, Documents: documents,
		Candidates: composed.Candidates, Projection: composed.Projection,
		DocumentsObserved: prepared.Accounting.Documents.Observed, TotalAcquiredBytes: prepared.Accounting.Bytes.Observed,
		RequestPolicyID: requestPolicyID,
	}, "LIVE", V2LiveBinding{Custody: "LIVE", SessionID: binding.SessionID, Generation: binding.Generation, PositionEncoding: "utf-16"}, maxResponseBytes)
	if err != nil {
		return V2WireResult{}, errors.New(strings.Replace(err.Error(), "sourceprojectionv2:", "liveprojection:", 1))
	}
	return result, nil
}
