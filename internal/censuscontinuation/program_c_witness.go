package censuscontinuation

import (
	"bytes"
	"encoding/json"
	"errors"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
)

const ProgramCWitnessSchema = "lsp-trace.census-continuation-program-c-witness.v1"

// ProgramCWitness is the closed, stable continuation projection of a Program C
// result. It deliberately excludes opaque admission state; resume reconstructs
// that state from the committed handoff and contract seed.
type ProgramCWitness struct {
	SchemaVersion         string                                   `json:"schema_version"`
	WitnessID             string                                   `json:"witness_id"`
	CensusID              string                                   `json:"census_id"`
	Publication           captureset.PublicationReceipt            `json:"publication"`
	CompositeID           string                                   `json:"composite_id"`
	CompositeOutputSHA256 string                                   `json:"composite_output_sha256"`
	CompositeBytesDigest  string                                   `json:"composite_bytes_digest"`
	CompositeByteLength   uint64                                   `json:"composite_byte_length"`
	Outcome               ProgramCOutcomeWitness                   `json:"outcome"`
	Candidates            []censusprogramc.Candidate               `json:"candidates"`
	Representatives       censusprogramc.RepresentativeSelection   `json:"representatives"`
	SourceBinding         programcadmission.CompositeSourceBinding `json:"source_binding"`
}

type ProgramCOutcomeWitness struct {
	Outcome, ProfileID, ProfileDigest, Algorithm, LogicalDigest, ClaimCeiling string
	Resolution                                                                float64
	Seed                                                                      uint64
	Communities                                                               []programc.Community
}

func NewProgramCWitness(result censusprogramc.Result) (ProgramCWitness, error) {
	w := ProgramCWitness{
		SchemaVersion:         ProgramCWitnessSchema,
		CensusID:              result.CensusID,
		Publication:           result.Publication,
		CompositeID:           result.Composite.Artifact.CompositeID,
		CompositeOutputSHA256: result.Composite.Artifact.OutputSHA256,
		CompositeBytesDigest:  digestOf(result.Composite.Bytes),
		CompositeByteLength:   uint64(len(result.Composite.Bytes)),
		Outcome:               outcomeWitness(result.Outcome),
		Candidates:            cloneCandidates(result.Candidates),
		Representatives:       cloneRepresentatives(result.Representatives),
		SourceBinding:         result.Admission.Admission.SourceBinding(),
	}
	w.WitnessID = identityJSON("lsp-trace:census-continuation-program-c-witness:v1", w, func(v *ProgramCWitness) { v.WitnessID = "" })
	if err := w.Validate(); err != nil {
		return ProgramCWitness{}, err
	}
	return w, nil
}

func (w ProgramCWitness) Validate() error {
	if w.SchemaVersion != ProgramCWitnessSchema || !validDigest(w.WitnessID) || w.CensusID == "" || !validDigest(w.CompositeID) || !validDigest(w.CompositeOutputSHA256) || !validDigest(w.CompositeBytesDigest) || w.CompositeByteLength == 0 {
		return errors.New("program C witness integrity mismatch")
	}
	want := identityJSON("lsp-trace:census-continuation-program-c-witness:v1", w, func(v *ProgramCWitness) { v.WitnessID = "" })
	if want != w.WitnessID {
		return errors.New("program C witness identity mismatch")
	}
	return nil
}

func (w ProgramCWitness) Bytes() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(w)
}

func ParseProgramCWitness(raw []byte) (ProgramCWitness, error) {
	var w ProgramCWitness
	if err := strictCanonical(raw, &w); err != nil {
		return ProgramCWitness{}, err
	}
	return w, w.Validate()
}

func verifyProgramCWitness(result censusprogramc.Result, raw []byte) (ProgramCWitness, error) {
	persisted, err := ParseProgramCWitness(raw)
	if err != nil {
		return ProgramCWitness{}, err
	}
	recomputed, err := NewProgramCWitness(result)
	if err != nil {
		return ProgramCWitness{}, err
	}
	recomputedBytes, err := recomputed.Bytes()
	if err != nil {
		return ProgramCWitness{}, err
	}
	if !bytes.Equal(raw, recomputedBytes) {
		return ProgramCWitness{}, errors.New("program C witness recomputation mismatch")
	}
	return persisted, nil
}

func outcomeWitness(in programc.Outcome) ProgramCOutcomeWitness {
	out := ProgramCOutcomeWitness{Outcome: in.Outcome, ProfileID: in.ProfileID, ProfileDigest: in.ProfileDigest, Algorithm: in.Algorithm, LogicalDigest: in.LogicalDigest, ClaimCeiling: in.ClaimCeiling, Resolution: in.Resolution, Seed: in.Seed, Communities: make([]programc.Community, len(in.Communities))}
	for i := range in.Communities {
		out.Communities[i].Members = append([]string(nil), in.Communities[i].Members...)
	}
	return out
}

func cloneCandidates(in []censusprogramc.Candidate) []censusprogramc.Candidate {
	out := append([]censusprogramc.Candidate(nil), in...)
	for i := range out {
		out[i].Members = append([]string(nil), in[i].Members...)
	}
	return out
}

func cloneRepresentatives(in censusprogramc.RepresentativeSelection) censusprogramc.RepresentativeSelection {
	out := censusprogramc.RepresentativeSelection{State: in.State, Nominations: append([]censusprogramc.Representative(nil), in.Nominations...), Unresolved: append([]censusprogramc.Representative(nil), in.Unresolved...)}
	clone := func(dst []censusprogramc.Representative, src []censusprogramc.Representative) {
		for i := range dst {
			dst[i].Members = append([]string(nil), src[i].Members...)
			dst[i].PreparedTargets = append([]string(nil), src[i].PreparedTargets...)
			dst[i].SCCMembers = append([]string(nil), src[i].SCCMembers...)
			dst[i].IncomingPredecessors = append([]censusprogramc.RepresentativePredecessor(nil), src[i].IncomingPredecessors...)
		}
	}
	clone(out.Nominations, in.Nominations)
	clone(out.Unresolved, in.Unresolved)
	return out
}
