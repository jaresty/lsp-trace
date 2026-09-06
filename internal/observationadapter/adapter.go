// Package observationadapter validates provider-reported observation envelopes and
// adapts them to graph-v4 relations. Provider source text is deliberately opaque.
package observationadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/relations"
)

const (
	ProtocolName              = "lsp-trace.provider-observations"
	ProtocolVersion           = "1"
	AuthorityProviderReported = "PROVIDER_REPORTED"
)

type Identity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Anchor struct {
	DocumentID string      `json:"document_id"`
	URI        string      `json:"uri"`
	Revision   string      `json:"revision"`
	Blob       string      `json:"blob"`
	MappingID  string      `json:"mapping_id,omitempty"`
	Range      graph.Range `json:"range"`
}

type ReportedObservation struct {
	Kind           relations.RelationKind `json:"kind"`
	From           relations.Endpoint     `json:"from"`
	To             relations.Endpoint     `json:"to"`
	OriginalAnchor Anchor                 `json:"original_anchor"`
	VirtualAnchor  *Anchor                `json:"virtual_anchor,omitempty"`
	Supports       []relations.Claim      `json:"supports"`
	DoesNotSupport []relations.Claim      `json:"does_not_support"`
}

type Envelope struct {
	Diagnostics  []graph.Diagnostic           `json:"diagnostics,omitempty"`
	Provider     Identity                     `json:"provider"`
	Protocol     Identity                     `json:"protocol"`
	Adapter      Identity                     `json:"adapter"`
	Authority    string                       `json:"authority"`
	Coverage     relations.Coverage           `json:"coverage"`
	Failure      relations.FailureKind        `json:"failure,omitempty"`
	Documents    []graph.SourceDocumentRecord `json:"documents"`
	Observations []ReportedObservation        `json:"observations"`
	RequestID    string                       `json:"request_id,omitempty"`
	Timestamp    string                       `json:"timestamp,omitempty"`
	Source       string                       `json:"source,omitempty"` // opaque; never parsed
}

type Observation struct {
	ObservationID string `json:"observation_id"`
	Authority     string `json:"authority"`
	ReportedObservation
}

type Result struct {
	Observations []Observation                `json:"observations"`
	Coverage     relations.Coverage           `json:"coverage"`
	Failure      relations.FailureKind        `json:"failure,omitempty"`
	Custody      graph.DocumentCustodyReceipt `json:"custody"`
	GraphV4      graph.NormalizedRelations    `json:"graph_v4"`
}

func Adapt(envelope Envelope) (Result, error) {
	result := Result{Coverage: envelope.Coverage, Failure: envelope.Failure}
	if err := validateIdentity("provider", envelope.Provider); err != nil {
		return result, err
	}
	if envelope.Protocol != (Identity{Name: ProtocolName, Version: ProtocolVersion}) {
		return result, fmt.Errorf("ASSERT_ADAPTER_PROTOCOL_IDENTITY: unsupported protocol %q version %q", envelope.Protocol.Name, envelope.Protocol.Version)
	}
	if err := validateIdentity("adapter", envelope.Adapter); err != nil {
		return result, err
	}
	if envelope.Authority != AuthorityProviderReported {
		return result, fmt.Errorf("ASSERT_PROVIDER_REPORTED_AUTHORITY: unsupported authority %q", envelope.Authority)
	}
	if err := envelope.Coverage.Validate(); err != nil {
		return result, fmt.Errorf("coverage: %w", err)
	}
	if envelope.Failure != "" && !envelope.Failure.Valid() {
		return result, fmt.Errorf("ASSERT_FAILURE_VOCABULARY_CLOSED: %q", envelope.Failure)
	}

	custody, err := graph.ValidateDocumentCustody(graph.DocumentCustodyRequest{Documents: envelope.Documents})
	if err != nil {
		return result, fmt.Errorf("document custody: %w", err)
	}
	result.Custody = custody
	documents := make(map[string]graph.SourceDocumentRecord, len(custody.Documents))
	for _, document := range custody.Documents {
		documents[document.DocumentID] = document
	}

	observations := make([]Observation, 0, len(envelope.Observations))
	groups := map[string][]Observation{}
	for _, reported := range envelope.Observations {
		observation, err := adaptObservation(envelope, reported, documents)
		if err != nil {
			return result, err
		}
		observations = append(observations, observation)
		keyBytes, _ := json.Marshal(struct {
			Kind     relations.RelationKind
			From, To relations.Endpoint
		}{reported.Kind, reported.From, reported.To})
		groups[string(keyBytes)] = append(groups[string(keyBytes)], observation)
	}
	sort.Slice(observations, func(i, j int) bool { return observations[i].ObservationID < observations[j].ObservationID })
	result.Observations = observations

	relationsOut := make([]graph.Relation, 0, len(groups))
	for _, group := range groups {
		first := group[0]
		anchors := make([]graph.RelationAnchor, 0, len(group)*2)
		ids := make([]string, 0, len(group))
		for _, observation := range group {
			ids = append(ids, observation.ObservationID)
			anchors = append(anchors, relationAnchor(observation.OriginalAnchor))
			if observation.VirtualAnchor != nil {
				anchors = append(anchors, relationAnchor(*observation.VirtualAnchor))
			}
		}
		supports := claimsToStrings(first.Supports)
		doesNotSupport := claimsToStrings(first.DoesNotSupport)
		relationsOut = append(relationsOut, graph.NewRelation(graph.Relation{
			Kind: string(first.Kind), From: first.From.NodeID, To: first.To.NodeID,
			EvidenceClass: graph.EvidenceSourceAdapter,
			Adapter:       &graph.RelationAdapter{Name: envelope.Adapter.Name, Version: envelope.Adapter.Version},
			Anchors:       anchors, Confidence: "EXACT", Supports: supports, DoesNotSupport: doesNotSupport,
			ContributingObservationIDs: ids,
		}))
	}
	sort.Slice(relationsOut, func(i, j int) bool { return relationsOut[i].RelationID < relationsOut[j].RelationID })
	result.GraphV4 = graph.NormalizedRelations{SchemaVersion: graph.NormalizedRelationsSchemaVersion, ArtifactKind: graph.NormalizedRelationsArtifactKind, Relations: relationsOut}
	return result, nil
}

func validateIdentity(label string, identity Identity) error {
	if identity.Name == "" || identity.Version == "" {
		return fmt.Errorf("ASSERT_ADAPTER_IDENTITY_FAILS_CLOSED_%s: name and version required", label)
	}
	return nil
}

func adaptObservation(envelope Envelope, reported ReportedObservation, documents map[string]graph.SourceDocumentRecord) (Observation, error) {
	if reported.Kind == relations.RelationCalls {
		return Observation{}, fmt.Errorf("ASSERT_NON_CALLS_PROVIDER_OBSERVATION_NEVER_PROMOTED_TO_CALLS")
	}
	document, ok := documents[reported.OriginalAnchor.DocumentID]
	if !ok || reported.OriginalAnchor.URI != document.OriginalURI || reported.OriginalAnchor.MappingID != "" {
		return Observation{}, fmt.Errorf("ASSERT_ORIGINAL_ANCHOR_CUSTODY: original anchor does not match immutable document")
	}
	if reported.VirtualAnchor != nil {
		if document.Mapping == nil || reported.VirtualAnchor.URI != document.VirtualURI || reported.VirtualAnchor.MappingID == "" || reported.VirtualAnchor.MappingID != document.Mapping.MappingID {
			return Observation{}, fmt.Errorf("ASSERT_SYNTHETIC_VIRTUAL_PROVENANCE_REJECTED")
		}
	}
	semantic := relations.SemanticObservation{
		SchemaVersion: relations.ObservationSchemaV1, Kind: reported.Kind,
		Authority: relations.AuthoritySourceDerivedAdapter,
		Provider:  relations.ProducerIdentity{Name: envelope.Adapter.Name, Version: envelope.Adapter.Version},
		From:      reported.From, To: reported.To, Anchors: []relations.SourceAnchor{semanticAnchor(reported.OriginalAnchor)},
		Supports: reported.Supports, DoesNotSupport: reported.DoesNotSupport,
	}
	if reported.VirtualAnchor != nil {
		semantic.Anchors = append(semantic.Anchors, semanticAnchor(*reported.VirtualAnchor))
	}
	id, err := semantic.CanonicalID()
	if err != nil {
		return Observation{}, fmt.Errorf("provider observation: %w", err)
	}
	// Include provider/protocol identity as well as semantic identity, while excluding volatile envelope fields.
	identity, _ := json.Marshal(struct {
		Provider, Protocol, Adapter Identity
		SemanticID                  string
	}{envelope.Provider, envelope.Protocol, envelope.Adapter, id})
	sum := sha256.Sum256(identity)
	return Observation{ObservationID: "sha256:" + hex.EncodeToString(sum[:]), Authority: AuthorityProviderReported, ReportedObservation: reported}, nil
}

func semanticAnchor(anchor Anchor) relations.SourceAnchor {
	return relations.SourceAnchor{URI: anchor.URI, Revision: anchor.Revision, Blob: anchor.Blob, StartLine: int(anchor.Range.Start.Line), StartCharacter: int(anchor.Range.Start.Character), EndLine: int(anchor.Range.End.Line), EndCharacter: int(anchor.Range.End.Character)}
}
func relationAnchor(anchor Anchor) graph.RelationAnchor {
	return graph.RelationAnchor{URI: anchor.URI, Revision: anchor.Revision, Blob: anchor.Blob, Range: anchor.Range}
}
func claimsToStrings(claims []relations.Claim) []string {
	out := make([]string, len(claims))
	for i, claim := range claims {
		out[i] = string(claim)
	}
	return out
}
