package retainedprojection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv2"
)

const (
	CodeInvalidResolveResult Code = "INVALID_RESOLVE_RESULT"
	CodeProjectionFailed     Code = "PROJECTION_FAILED"
	CodeAssemblyFailed       Code = "ASSEMBLY_FAILED"
)

type AssemblyError struct {
	Code   Code
	Key    Key
	Cause  error
	detail string
}

func (e *AssemblyError) Error() string {
	prefix := fmt.Sprintf("retainedprojection: %s", e.Code)
	if e.Key != (Key{}) {
		prefix = fmt.Sprintf("%s: %s\x00%s", prefix, e.Key.GraphSubjectID, e.Key.LogicalSourceID)
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", prefix, e.detail, e.Cause)
	}
	return fmt.Sprintf("%s: %s", prefix, e.detail)
}

func (e *AssemblyError) Unwrap() error { return e.Cause }

func assemblyFail(code Code, key Key, detail string, cause error) error {
	return &AssemblyError{Code: code, Key: key, Cause: cause, detail: detail}
}

func AssembleV2Bounded[T any](resolved ResolveResult, policy sourceprojection.Policy, custodyBinding T, requestPolicyID string, maxResponseBytes int) (sourceprojectionv2.WireResult[T], error) {
	zero := sourceprojectionv2.WireResult[T]{}
	if requestPolicyID == "" || requestPolicyID != policy.PolicyID || maxResponseBytes <= 0 {
		return zero, assemblyFail(CodeInvalidRequest, Key{}, "non-empty matching policy id and positive response bound required", nil)
	}
	candidates, sources, selectedURIs, documents, documentsObserved, acquiredBytes, err := retainedInputs(resolved)
	if err != nil {
		return zero, err
	}
	projection, err := sourceprojection.Project(candidates, sources, policy)
	if err != nil {
		return zero, assemblyFail(CodeProjectionFailed, Key{}, "neutral projection failed", err)
	}
	result, err := sourceprojectionv2.AssembleBounded(sourceprojectionv2.Input{
		TargetURI: resolved.Selections[0].Selection.Key.LogicalSourceID, SelectedURIs: selectedURIs,
		Documents: documents, Candidates: candidates, Projection: projection,
		DocumentsObserved: documentsObserved, TotalAcquiredBytes: acquiredBytes, RequestPolicyID: requestPolicyID,
	}, "RETAINED", custodyBinding, maxResponseBytes)
	if err != nil {
		return zero, assemblyFail(CodeAssemblyFailed, Key{}, "neutral V2 assembly failed", err)
	}
	return result, nil
}

func retainedInputs(resolved ResolveResult) ([]sourceprojection.Candidate, map[string]sourceprojection.Source, []string, []sourceprojectionv2.DocumentSource, int, int, error) {
	if len(resolved.Selections) == 0 {
		return nil, nil, nil, nil, 0, 0, assemblyFail(CodeInvalidResolveResult, Key{}, "non-empty resolved selections required", nil)
	}
	candidates := make([]sourceprojection.Candidate, 0, len(resolved.Selections))
	sources := make(map[string]sourceprojection.Source)
	identities := make(map[sourceobject.Identity]struct{})
	var acquiredBytes uint64
	for i, resolvedSelection := range resolved.Selections {
		selection := resolvedSelection.Selection
		if err := validateResolvedSelection(selection, resolvedSelection.Bytes, i, resolved.Selections); err != nil {
			return nil, nil, nil, nil, 0, 0, err
		}
		logical := selection.Key.LogicalSourceID
		if existing, ok := sources[logical]; ok {
			if existing.Digest != selection.Source.Digest || uint64(len(existing.Bytes)) != selection.Source.ByteLength || !equalBytes(existing.Bytes, resolvedSelection.Bytes) {
				return nil, nil, nil, nil, 0, 0, assemblyFail(CodeSourceMismatch, selection.Key, "logical source uses inconsistent identity or bytes", nil)
			}
		} else {
			sources[logical] = sourceprojection.Source{LogicalSourceID: logical, Digest: selection.Source.Digest, Bytes: append([]byte(nil), resolvedSelection.Bytes...), Available: true}
		}
		if _, seen := identities[selection.Source]; !seen {
			identities[selection.Source] = struct{}{}
			acquiredBytes += selection.Source.ByteLength
			if acquiredBytes > uint64(^uint(0)>>1) {
				return nil, nil, nil, nil, 0, 0, assemblyFail(CodeInvalidResolveResult, selection.Key, "acquired byte accounting overflows int", nil)
			}
		}
		candidate := retainedCandidate(selection)
		candidates = append(candidates, candidate)
	}
	selectedURIs := retainedSourceOrder(resolved.Selections[0].Selection.Key.LogicalSourceID, sources)
	documents := make([]sourceprojectionv2.DocumentSource, 0, len(selectedURIs))
	for _, uri := range selectedURIs {
		source := sources[uri]
		documents = append(documents, sourceprojectionv2.DocumentSource{URI: uri, DocumentVersion: 0, PositionEncoding: "utf-16", SourceDigest: source.Digest, SourceByteLength: len(source.Bytes)})
	}
	return candidates, sources, selectedURIs, documents, len(sources), int(acquiredBytes), nil
}

func validateResolvedSelection(selection Selection, raw []byte, ordinal int, all []ResolvedSelection) error {
	key := selection.Key
	if key.GraphSubjectID == "" || key.LogicalSourceID == "" || selection.Ordinal != ordinal {
		return assemblyFail(CodeInvalidResolveResult, key, "invalid key or ordinal", nil)
	}
	wantRole := "ADDITIONAL"
	if ordinal == 0 {
		wantRole = "TARGET"
	}
	if selection.Role != wantRole || (ordinal > 1 && !lessKey(all[ordinal-1].Selection.Key, key)) || (ordinal == 1 && key == all[0].Selection.Key) {
		return assemblyFail(CodeInvalidResolveResult, key, "resolved selections are not canonical target-first order", nil)
	}
	if selection.PositionEncoding != "utf-16" {
		return assemblyFail(CodeUnsupportedEncoding, key, "retained projection requires utf-16", nil)
	}
	if selection.ItemRange == nil || selection.SelectionRange == nil {
		return assemblyFail(CodeInvalidResolveResult, key, "item and selection ranges required", nil)
	}
	if !validRange(selection.DisplayRange) || !validRange(*selection.ItemRange) || !validRange(*selection.SelectionRange) || !contains(selection.DisplayRange, *selection.ItemRange) || !contains(*selection.ItemRange, *selection.SelectionRange) {
		return assemblyFail(CodeIncompatibleRange, key, "selection, item, and display ranges are invalid or incompatible", nil)
	}
	if selection.DisplayProvenance.Kind == "" || selection.DisplayRangePolicy == "" {
		return assemblyFail(CodeInvalidProvenance, key, "display provenance kind and policy required", nil)
	}
	if !canonicalResolveDigest(selection.Source.Digest) || selection.Source.ByteLength != uint64(len(raw)) {
		return assemblyFail(CodeSourceMismatch, key, "source identity does not match bytes", nil)
	}
	sum := sha256.Sum256(raw)
	if selection.Source.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return assemblyFail(CodeSourceMismatch, key, "source digest does not match bytes", nil)
	}
	return nil
}

func retainedCandidate(selection Selection) sourceprojection.Candidate {
	candidate := sourceprojection.Candidate{
		Role: "ENDPOINT", GraphSubjectID: selection.Key.GraphSubjectID, LogicalSourceID: selection.Key.LogicalSourceID,
		Range: projectionRangeRetained(selection.DisplayRange), EvidenceRange: projectionRangeRetained(*selection.SelectionRange),
		ItemRange: projectionRangeRetained(*selection.ItemRange), SelectionRange: projectionRangeRetained(*selection.SelectionRange),
		DisplayProvenance: selection.DisplayProvenance.Kind, PositionEncoding: selection.PositionEncoding, PrivacyClassification: "PUBLIC",
	}
	candidate.UnitID = retainedCanonicalID(struct {
		Role, GraphSubjectID, LogicalSourceID                                                string
		DisplayRange, EvidenceRange, ItemRange, SelectionRange                               sourceprojection.Range
		SourceDigest                                                                         string
		SourceByteLength                                                                     uint64
		PositionEncoding, DisplayProvenanceKind, DisplayProvenanceMethod, DisplayRangePolicy string
	}{candidate.Role, candidate.GraphSubjectID, candidate.LogicalSourceID, candidate.Range, candidate.EvidenceRange, candidate.ItemRange, candidate.SelectionRange, selection.Source.Digest, selection.Source.ByteLength, selection.PositionEncoding, selection.DisplayProvenance.Kind, selection.DisplayProvenance.Method, selection.DisplayRangePolicy})
	candidate.CitationID = retainedCanonicalID(struct{ UnitID, Role, Subject string }{candidate.UnitID, candidate.Role, candidate.GraphSubjectID})
	return candidate
}

func projectionRangeRetained(value graph.Range) sourceprojection.Range {
	return sourceprojection.Range{Start: sourceprojection.Position{Line: value.Start.Line, Character: value.Start.Character}, End: sourceprojection.Position{Line: value.End.Line, Character: value.End.Character}}
}

func retainedCanonicalID(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func retainedSourceOrder(target string, sources map[string]sourceprojection.Source) []string {
	remaining := make([]string, 0, len(sources)-1)
	for uri := range sources {
		if uri != target {
			remaining = append(remaining, uri)
		}
	}
	sort.Strings(remaining)
	return append([]string{target}, remaining...)
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
