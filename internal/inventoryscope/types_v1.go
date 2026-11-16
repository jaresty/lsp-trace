// Package inventoryscope qualifies a caller-supplied complete candidate ledger.
// It performs no discovery, filesystem access, adapter execution or authorization.
package inventoryscope

import "errors"

const SchemaV1 = "inventory-scope/v1"
const (
	PolicyConsensusV1       = "evidence-consensus/v1"
	PolicyIntentFirstV1     = "intent-first/v1"
	ResolutionIntentFirstV1 = "caller-intent-first/v1"
)

type MembershipV1 string

const (
	IncludedV1   MembershipV1 = "INCLUDED"
	ExcludedV1   MembershipV1 = "EXCLUDED"
	UnresolvedV1 MembershipV1 = "UNRESOLVED"
)

type ObservationKindV1 string

const (
	MembershipFactV1      ObservationKindV1 = "MEMBERSHIP"
	BuildMembershipV1     ObservationKindV1 = "BUILD_MEMBERSHIP"
	AnalysisFailureV1     ObservationKindV1 = "ANALYSIS_FAILURE"
	UnreadableV1          ObservationKindV1 = "UNREADABLE"
	UnsupportedLanguageV1 ObservationKindV1 = "UNSUPPORTED_LANGUAGE"
	ResourceGapV1         ObservationKindV1 = "RESOURCE_GAP"
)

type ReasonV1 string

const (
	ReasonMissingV1          ReasonV1 = "MISSING_MEMBERSHIP"
	ReasonConflictV1         ReasonV1 = "CONFLICTING_MEMBERSHIP"
	ReasonEvidenceIncludedV1 ReasonV1 = "EVIDENCE_INCLUDED"
	ReasonEvidenceExcludedV1 ReasonV1 = "EVIDENCE_EXCLUDED"
	ReasonCallerIncludedV1   ReasonV1 = "CALLER_INCLUDED"
	ReasonCallerExcludedV1   ReasonV1 = "CALLER_EXCLUDED"
	ReasonOutsideRootV1      ReasonV1 = "POLICY_OUTSIDE_DECLARED_ROOT"
	ReasonResolvedV1         ReasonV1 = "CALLER_RESOLVED"
)

// ArtifactRefV1 is caller-owned metadata, never a path to follow or a grant.
type ArtifactRefV1 struct {
	ArtifactID string `json:"artifact_id"`
	Digest     string `json:"digest"`
}

type CandidateV1 struct {
	OriginalOrdinal       int           `json:"original_ordinal"`
	CanonicalRelativePath string        `json:"canonical_relative_path"`
	CandidateID           string        `json:"candidate_id"`
	LanguageID            string        `json:"language_id,omitempty"`
	DiscoveryProvenance   ArtifactRefV1 `json:"discovery_provenance"`
}

type DeclarationV1 struct {
	Subject          string   `json:"subject"`
	ApplicationRoots []string `json:"application_roots"`
	// RootBoundary is explicit caller intent. Roots alone are not evidence.
	RootBoundary bool `json:"root_boundary"`
	// Includes and Excludes are exact canonical relative paths, not globs.
	Includes     []string `json:"includes"`
	Excludes     []string `json:"excludes"`
	SourceID     string   `json:"source_id"`
	ConfigDigest string   `json:"config_digest"`
	PolicyID     string   `json:"policy_id"`
}

type ObservationV1 struct {
	ObservationID    string            `json:"observation_id"`
	CandidateID      string            `json:"candidate_id"`
	Assertion        MembershipV1      `json:"assertion"`
	Kind             ObservationKindV1 `json:"kind"`
	ProducerIdentity string            `json:"producer_identity"`
	ProducerVersion  string            `json:"producer_version"`
	Artifact         ArtifactRefV1     `json:"artifact"`
	SourceID         string            `json:"source_id"`
	ConfigDigest     string            `json:"config_digest"`
}

type ResolutionV1 struct {
	ResolutionID    string       `json:"resolution_id"`
	CandidateID     string       `json:"candidate_id"`
	ObservationRefs []string     `json:"observation_refs"`
	Rule            string       `json:"rule"`
	Outcome         MembershipV1 `json:"outcome"`
	CallerActionRef string       `json:"caller_action_ref"`
}

type InputV1 struct {
	Schema       string          `json:"schema"`
	Declaration  DeclarationV1   `json:"declaration"`
	Candidates   []CandidateV1   `json:"candidates"`
	Observations []ObservationV1 `json:"observations"`
	Resolutions  []ResolutionV1  `json:"resolutions"`
}

type EntryV1 struct {
	CandidateID   string       `json:"candidate_id"`
	Membership    MembershipV1 `json:"membership"`
	Reason        ReasonV1     `json:"reason"`
	EvidenceRefs  []string     `json:"evidence_refs"`
	ResolutionRef string       `json:"resolution_ref,omitempty"`
}

type ManifestV1 struct {
	Schema                    string    `json:"schema"`
	OriginalDenominatorCount  int       `json:"original_denominator_count"`
	OriginalDenominatorDigest string    `json:"original_denominator_digest"`
	DeclarationDigest         string    `json:"declaration_digest"`
	ConfigDigest              string    `json:"config_digest"`
	PolicyDigest              string    `json:"policy_digest"`
	EvidenceDigest            string    `json:"evidence_digest"`
	ResolutionDigest          string    `json:"resolution_digest"`
	Entries                   []EntryV1 `json:"entries"`
	IncludedCount             int       `json:"included_count"`
	ExcludedCount             int       `json:"excluded_count"`
	UnresolvedCount           int       `json:"unresolved_count"`
	ScopeDigest               string    `json:"scope_digest"`
}

type SelectionStateV1 string

const (
	ProposedV1       SelectionStateV1 = "PROPOSED"
	CallerSelectedV1 SelectionStateV1 = "CALLER_SELECTED"
)

type SelectionV1 struct {
	State           SelectionStateV1 `json:"state"`
	ManifestDigest  string           `json:"manifest_digest"`
	CallerActionRef string           `json:"caller_action_ref,omitempty"`
}

// LimitsV1 bounds caller data before copying, sorting or hashing. Zero fields
// select conservative defaults; positive overrides may only lower hard ceilings.
type LimitsV1 struct {
	MaxInputBytes                                                 int64
	MaxCandidates, MaxObservations, MaxResolutions, MaxReferences int
	MaxUTF8Bytes, MaxWork                                         int
}

var (
	ErrInvalidV1   = errors.New("invalid inventory scope v1")
	ErrLimitV1     = errors.New("inventory scope v1 limit")
	ErrSelectionV1 = errors.New("inventory scope v1 selection required")
)

func (l LimitsV1) EffectiveV1() (LimitsV1, error) {
	d := LimitsV1{32 << 20, 10000, 50000, 10000, 200000, 16 << 20, 2000000}
	if l.MaxInputBytes < 0 || l.MaxInputBytes > d.MaxInputBytes {
		return l, ErrLimitV1
	}
	if l.MaxInputBytes == 0 {
		l.MaxInputBytes = d.MaxInputBytes
	}
	p := []*int{&l.MaxCandidates, &l.MaxObservations, &l.MaxResolutions, &l.MaxReferences, &l.MaxUTF8Bytes, &l.MaxWork}
	caps := []int{d.MaxCandidates, d.MaxObservations, d.MaxResolutions, d.MaxReferences, d.MaxUTF8Bytes, d.MaxWork}
	for i, v := range p {
		if *v < 0 || *v > caps[i] {
			return l, ErrLimitV1
		}
		if *v == 0 {
			*v = caps[i]
		}
	}
	return l, nil
}

func ValidateSelectionV1(s SelectionV1, m ManifestV1) error {
	if !validDigest(m.ScopeDigest) || s.ManifestDigest != m.ScopeDigest {
		return ErrSelectionV1
	}
	switch s.State {
	case ProposedV1:
		if s.CallerActionRef != "" {
			return ErrSelectionV1
		}
	case CallerSelectedV1:
		if !validID(s.CallerActionRef) {
			return ErrSelectionV1
		}
	default:
		return ErrSelectionV1
	}
	return nil
}
