package programc

// This file exposes source-neutral mathematical mechanics to private adapters.
// None of these types/functions admits provenance, creates custody, or returns an
// ordinary Program C Outcome, SourceBinding, profile, or claim ceiling.
import (
	"encoding/json"
	"math"

	"lsp-trace/internal/schema"
)

// CallsMathPrivate is a detached numerical projection, NOT an admission permit.
// Node indices are the lexical ordering of NodeIdentities; each occurrence has
// unit weight and PairWeights is exactly the directed occurrence count.
type CallsMathPrivate struct {
	NodeIdentities []string
	Occurrences    []Occurrence
	PairWeights    map[Pair]float64
}

// PartitionMathPrivate contains mathematical results only, never source claims.
type PartitionMathPrivate struct {
	Outcome, Algorithm, LogicalDigest string
	Resolution                        float64
	Seed                              uint64
	Communities                       []Community
}

// ProjectNativeCallsMathPrivate validates native V5 semantics then applies the
// same CALLS interpretation as Project. Native validity is NOT source admission.
// The private byte cap is independent of, and does not widen, ordinary gates.
func ProjectNativeCallsMathPrivate(raw []byte) (CallsMathPrivate, *Failure) {
	return projectNativeCallsMathPrivate(raw, false)
}

// ProjectNativeCallsMathPrivateV2 is a distinct, source-neutral numerical policy.
// Native V5 validation and typed relation/endpoint checks are unchanged. Empty
// call-site lists contribute zero unit occurrences, never a synthetic reference
// or an absence claim. This conveys no producer or ordinary admission.
func ProjectNativeCallsMathPrivateV2(raw []byte) (CallsMathPrivate, *Failure) {
	return projectNativeCallsMathPrivate(raw, true)
}
func projectNativeCallsMathPrivate(raw []byte, allowZeroSite bool) (CallsMathPrivate, *Failure) {
	if len(raw) > 8<<20 {
		return CallsMathPrivate{}, &Failure{Code: CodeResourceLimit, Message: "private native math byte cap"}
	}
	if version, err := schema.ValidateFor(raw, schema.FamilyGraph, "v5"); err != nil || version != "lsp-trace.graph.v5" {
		return CallsMathPrivate{}, &Failure{Code: CodeInvalidCalls, Message: "validated native V5 semantics required"}
	}
	var doc native
	if err := json.Unmarshal(raw, &doc); err != nil {
		return CallsMathPrivate{}, &Failure{Code: CodeInvalidCalls, Message: err.Error()}
	}
	p, failure := projectNativeCallsPolicyPrivate(doc, allowZeroSite)
	if failure != nil {
		return CallsMathPrivate{}, failure
	}
	out := CallsMathPrivate{p.NodeIdentities, p.Occurrences, p.PairWeights}
	if failure := validateCallsMathPrivate(out); failure != nil {
		return CallsMathPrivate{}, failure
	}
	return out, nil
}

// PartitionCallsMathPrivate accepts numerical data only. It supplies NO admission
// or authority. The same unexported partitionMath implements ordinary Compute.
func PartitionCallsMathPrivate(in CallsMathPrivate, seed uint64) (PartitionMathPrivate, *Failure) {
	if failure := validateCallsMathPrivate(in); failure != nil {
		return PartitionMathPrivate{}, failure
	}
	return partitionMath(Projection{NodeIdentities: in.NodeIdentities, Occurrences: in.Occurrences, PairWeights: in.PairWeights}, seed)
}

func validateCallsMathPrivate(in CallsMathPrivate) *Failure {
	if f := enforceCaps(len(in.NodeIdentities), len(in.Occurrences)); f != nil {
		return f
	}
	bad := func() *Failure {
		return &Failure{Code: CodeInvalidCalls, Message: "invalid private numerical CALLS projection"}
	}
	for i, id := range in.NodeIdentities {
		if id == "" || (i > 0 && in.NodeIdentities[i-1] >= id) {
			return bad()
		}
	}
	counts := make(map[Pair]float64)
	seen := make(map[string]bool)
	for _, o := range in.Occurrences {
		if o.Identity == "" || o.RelationID == "" || o.Provider == "" || o.From < 0 || o.To < 0 || o.From >= int64(len(in.NodeIdentities)) || o.To >= int64(len(in.NodeIdentities)) || o.Weight != 1 || seen[o.Identity] {
			return bad()
		}
		if o.CallSite.Start.Line > o.CallSite.End.Line || (o.CallSite.Start.Line == o.CallSite.End.Line && o.CallSite.Start.Character > o.CallSite.End.Character) {
			return bad()
		}
		seen[o.Identity] = true
		counts[Pair{From: o.From, To: o.To}]++
	}
	if len(counts) != len(in.PairWeights) {
		return bad()
	}
	for p, weight := range in.PairWeights {
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight <= 0 || counts[p] != weight {
			return bad()
		}
	}
	return nil
}

func CloneCallsMathPrivate(in CallsMathPrivate) CallsMathPrivate {
	out := CallsMathPrivate{NodeIdentities: append([]string(nil), in.NodeIdentities...), Occurrences: append([]Occurrence(nil), in.Occurrences...), PairWeights: make(map[Pair]float64, len(in.PairWeights))}
	for p, w := range in.PairWeights {
		out.PairWeights[p] = w
	}
	return out
}

func ClonePartitionMathPrivate(in PartitionMathPrivate) PartitionMathPrivate {
	out := in
	out.Communities = make([]Community, len(in.Communities))
	for i, c := range in.Communities {
		out.Communities[i].Members = append([]string(nil), c.Members...)
	}
	return out
}
