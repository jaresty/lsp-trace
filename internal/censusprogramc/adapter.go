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
)

type Stage string

const (
	StagePublication    Stage = "PUBLICATION_PRECONDITION"
	StageReconciliation Stage = "RECONCILIATION"
	StageComposition    Stage = "COMPOSITION"
	StageAdmission      Stage = "ADMISSION"
	StageComputation    Stage = "COMPUTATION"
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
type Result struct {
	CensusID    string
	Publication captureset.PublicationReceipt
	Composite   programccompose.Result
	Admission   programcadmission.Result
	Outcome     programc.Outcome
	Candidates  []Candidate
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
	return Result{CensusID: projection.CensusID, Publication: evidence.Receipt, Composite: cloneComposite(composite), Admission: cloneAdmission(admission), Outcome: cloneOutcome(outcome), Candidates: candidates}, nil
}

func fail(stage Stage, err error) *Failure { return &Failure{Stage: stage, Err: err} }

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
func cloneOutcome(in programc.Outcome) programc.Outcome {
	out := in
	out.Communities = make([]programc.Community, len(in.Communities))
	for i := range in.Communities {
		out.Communities[i].Members = append([]string(nil), in.Communities[i].Members...)
	}
	return out
}

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
