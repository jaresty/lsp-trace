// Package censusprogramc adapts a verified private census capture-set into one
// deterministic Program C composite. It performs no transport or publication I/O.
package censusprogramc

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/programcrepresentative"
)

type Stage string

const (
	StagePublication    Stage = "PUBLICATION_PRECONDITION"
	StageReconciliation Stage = "RECONCILIATION"
	StageComposition    Stage = "COMPOSITION"
	StageAdmission      Stage = "ADMISSION"
	StageComputation    Stage = "COMPUTATION"
	StageRepresentative Stage = "REPRESENTATIVE_QUALIFICATION"
	CandidateStatus           = "PROVISIONAL_STRUCTURAL_CANDIDATE"
)

type Failure struct {
	Stage Stage
	Err   error
}

func (f *Failure) Error() string { return string(f.Stage) + ": " + f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }

type ResolvedConstituent struct {
	ImmutableSelector string
	Bytes             []byte
}

// VerifiedPublication is closed evidence of a committed, successfully verified
// private capture-set. It deliberately does not attest to producer authentication.
type VerifiedPublication struct {
	Receipt  captureset.PublicationReceipt
	Manifest captureset.Manifest
	Resolved []ResolvedConstituent
}

type Request struct {
	Projection  censusacquisition.Projection
	Publication VerifiedPublication
	Metadata    programccompose.ExactMetadata
	Seed        uint64
}

type Candidate struct {
	Status, ClaimCeiling string
	Members              []string
}

// Representative is an additive, correction-oriented structural nomination.
// It retains no semantic identity, ownership, or runtime assertion.
type Representative struct {
	Status, ClaimCeiling, SelectionState                                    string
	Members, PreparedTargets, SCCMembers                                    []string
	CensusID, BatchID, CommunityIdentity, ConstituentIdentity               string
	ConstituentOrdinal, Distance, Authority                                 int
	ExecutionBundleID, SeedLabel, SeedAt, SelectedNode, SourceGraphComplete string
	AllTraversalComplete, AnyTruncated                                      bool
}
type RepresentativeSelection struct {
	State                   string
	Nominations, Unresolved []Representative
}
type Result struct {
	CensusID        string
	Publication     captureset.PublicationReceipt
	Composite       programccompose.Result
	Admission       programcadmission.Result
	Outcome         programc.Outcome
	Candidates      []Candidate
	Representatives RepresentativeSelection
}

func Compose(request Request) (Result, error) {
	projection, err := censusacquisition.CloneProjection(request.Projection)
	if err != nil {
		return Result{}, fail(StageReconciliation, err)
	}
	if err := canonicalResolvedOrder(request.Publication); err != nil {
		return Result{}, fail(StageReconciliation, err)
	}
	evidence, err := cloneAndValidateEvidence(request.Publication)
	if err != nil {
		return Result{}, fail(StagePublication, err)
	}
	if err := reconcile(projection, evidence); err != nil {
		return Result{}, fail(StageReconciliation, err)
	}
	inputs := make([]programccompose.Input, len(projection.Constituents))
	for i, constituent := range projection.Constituents {
		inputs[i] = programccompose.Input{Bytes: append([]byte(nil), constituent.Raw...), Identity: constituent.Identity.ImmutableSelector, SHA256: constituent.Identity.SHA256, ByteLength: constituent.Identity.ByteLength, ExactMetadata: request.Metadata}
	}
	composite, err := programccompose.Compose(inputs)
	if err != nil {
		return Result{}, fail(StageComposition, err)
	}
	admission, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		return Result{}, fail(StageAdmission, err)
	}
	outcome, computation := programc.ComputeComposite(admission.Admission, request.Seed)
	if computation != nil {
		return Result{}, fail(StageComputation, computation)
	}
	candidates := make([]Candidate, len(outcome.Communities))
	for i, community := range outcome.Communities {
		candidates[i] = Candidate{Status: CandidateStatus, ClaimCeiling: outcome.ClaimCeiling, Members: append([]string(nil), community.Members...)}
	}
	communities := outcome.Communities
	if outcome.Outcome == "EMPTY" {
		communities = nil
	}
	qualified, err := programcrepresentative.Select(programcrepresentative.Input{Admission: admission.Admission, Communities: communities})
	if err != nil {
		return Result{}, fail(StageRepresentative, err)
	}
	representatives, err := representativesFromQualification(projection, admission.Admission.SourceBinding(), outcome, qualified)
	if err != nil {
		return Result{}, fail(StageRepresentative, err)
	}
	if outcome.Outcome == "EMPTY" {
		representatives.State = string(programcrepresentative.StateEmpty)
	}
	return Result{CensusID: projection.CensusID, Publication: evidence.Receipt, Composite: cloneComposite(composite), Admission: cloneAdmission(admission), Outcome: cloneOutcome(outcome), Candidates: candidates, Representatives: representatives}, nil
}

func fail(stage Stage, err error) *Failure { return &Failure{Stage: stage, Err: err} }

func representativesFromQualification(projection censusacquisition.Projection, source programcadmission.CompositeSourceBinding, outcome programc.Outcome, qualified programcrepresentative.Output) (RepresentativeSelection, error) {
	if len(source.Constituents) != len(projection.Constituents) {
		return RepresentativeSelection{}, errors.New("representative constituent lineage cardinality mismatch")
	}
	for i, constituent := range source.Constituents {
		if constituent.Identity != projection.Constituents[i].Identity.ImmutableSelector {
			return RepresentativeSelection{}, errors.New("representative constituent lineage mismatch")
		}
	}
	byIdentity := make(map[string]struct {
		ordinal int
		batch   string
	}, len(projection.Constituents))
	for i, c := range projection.Constituents {
		byIdentity[c.Identity.ImmutableSelector] = struct {
			ordinal int
			batch   string
		}{i, c.BatchID}
	}
	base := func(identity string, ordinal int, members []string) (Representative, error) {
		lineage, ok := byIdentity[identity]
		if !ok || ordinal != lineage.ordinal {
			return Representative{}, errors.New("representative constituent identity or ordinal mismatch")
		}
		return Representative{Status: CandidateStatus, ClaimCeiling: outcome.ClaimCeiling, Members: append([]string(nil), members...), CensusID: projection.CensusID, ConstituentIdentity: identity, ConstituentOrdinal: lineage.ordinal, BatchID: lineage.batch, Authority: source.Authority, SourceGraphComplete: source.SourceGraphComplete, AllTraversalComplete: source.Completeness.AllTraversalComplete, AnyTruncated: source.Completeness.AnyTruncated}, nil
	}
	out := RepresentativeSelection{State: string(qualified.State), Nominations: make([]Representative, 0, len(qualified.Nominations)), Unresolved: make([]Representative, 0, len(qualified.Unresolved))}
	for _, n := range qualified.Nominations {
		c, err := base(n.ConstituentIdentity, n.ConstituentOrdinal, n.CommunityMembers)
		if err != nil {
			return RepresentativeSelection{}, err
		}
		c.SelectionState = string(n.State)
		c.CommunityIdentity = n.CommunityIdentity
		c.ExecutionBundleID = n.ExecutionBundleID
		c.SeedLabel = n.SeedLabel
		c.SeedAt = n.SeedAt
		c.PreparedTargets = append([]string(nil), n.PreparedTargets...)
		c.SelectedNode = n.SelectedNode
		c.Distance = n.Distance
		c.SCCMembers = append([]string(nil), n.SCCMembers...)
		out.Nominations = append(out.Nominations, c)
	}
	for _, u := range qualified.Unresolved {
		c, err := base(u.ConstituentIdentity, u.ConstituentOrdinal, u.CommunityMembers)
		if err != nil {
			return RepresentativeSelection{}, err
		}
		c.Status = ""
		c.SelectionState = string(u.State)
		c.CommunityIdentity = u.CommunityIdentity
		c.ExecutionBundleID = u.ExecutionBundleID
		c.SeedLabel, c.SeedAt = u.SeedLabel, u.SeedAt
		c.PreparedTargets = append([]string(nil), u.PreparedTargets...)
		out.Unresolved = append(out.Unresolved, c)
	}
	return out, nil
}

func canonicalResolvedOrder(in VerifiedPublication) error {
	if len(in.Resolved) != len(in.Manifest.Constituents) {
		return nil
	}
	for i, item := range in.Resolved {
		if item.ImmutableSelector != in.Manifest.Constituents[i].ImmutableSelector {
			return errors.New("resolved constituents are not canonically selector ordered")
		}
	}
	return nil
}

func cloneAndValidateEvidence(in VerifiedPublication) (captureset.VerifiedPublicationEvidence, error) {
	exact := make([]captureset.ExactConstituent, len(in.Resolved))
	for i, r := range in.Resolved {
		exact[i] = captureset.ExactConstituent{ImmutableSelector: r.ImmutableSelector, Bytes: append([]byte(nil), r.Bytes...)}
	}
	return captureset.VerifyPublicationEvidence(in.Receipt, in.Manifest, exact)
}

func reconcile(p censusacquisition.Projection, e captureset.VerifiedPublicationEvidence) error {
	manifestRaw, err := captureset.EncodeCanonical(e.Manifest)
	if err != nil || !bytes.Equal(manifestRaw, p.ManifestBytes) {
		return errors.New("verified manifest does not exactly match projection")
	}
	if len(e.Manifest.Constituents) != len(p.Constituents) || len(e.Constituents) != len(p.Constituents) {
		return errors.New("verified constituent cardinality mismatch")
	}
	resolved := make(map[string][]byte, len(e.Constituents))
	for i, r := range e.Constituents {
		// Resolve evidence is canonically selector ordered. This compares selector
		// identity before using the selector-keyed map, so ordering is never a
		// substitute for immutable identity or batch association.
		if r.ImmutableSelector == "" || r.ImmutableSelector != e.Manifest.Constituents[i].ImmutableSelector || resolved[r.ImmutableSelector] != nil {
			return errors.New("missing, duplicate, or reordered verified constituent")
		}
		resolved[r.ImmutableSelector] = r.Bytes
	}
	for i, c := range p.Constituents {
		if c.Ordinal != i || c.BatchID != p.Batches[i].BatchID || p.Batches[i].Session != p.Session {
			return errors.New("projection batch association mismatch")
		}
		mi := p.Manifest.Batches[i].ConstituentIndex
		if mi < 0 || mi >= len(e.Manifest.Constituents) || e.Manifest.Constituents[mi] != c.Identity {
			return errors.New("verified batch constituent association mismatch")
		}
		raw, ok := resolved[c.Identity.ImmutableSelector]
		if !ok || !bytes.Equal(raw, c.Raw) {
			return errors.New("verified constituent bytes mismatch")
		}
		if err := captureset.NativeV5Authority().VerifyConstituent(c.Identity, raw); err != nil {
			return fmt.Errorf("verified constituent identity: %w", err)
		}
	}
	return nil
}

func cloneComposite(in programccompose.Result) programccompose.Result {
	out := in
	out.Bytes = append([]byte(nil), in.Bytes...)
	return out
}
func cloneAdmission(in programcadmission.Result) programcadmission.Result {
	out := in
	out.Bytes = append([]byte(nil), in.Bytes...)
	return out
}
func cloneOutcome(in programc.Outcome) programc.Outcome { return programc.CloneOutcome(in) }

// CanonicalSelectors returns a cloned selector order for callers that need to
// construct closed evidence without relying on positional correspondence.
func CanonicalSelectors(manifest captureset.Manifest) []string {
	out := make([]string, len(manifest.Constituents))
	for i := range manifest.Constituents {
		out[i] = manifest.Constituents[i].ImmutableSelector
	}
	sort.Strings(out)
	return out
}
