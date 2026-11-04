package referencegrouping

// Detached candidate evaluation has no route to qualification or admission.
import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const EvaluationUniversePrivate = "CONTRIBUTION_ENDPOINTS_ONLY"
const EvaluationProjectionSchemaPrivate = "lsp-trace.private.grouping-candidate-projection.v1"
const EvaluationCeilingPrivate = "PRIVATE_NONADMISSIBLE;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;SOURCE_NODE_UNIVERSE_UNKNOWN;NO_FEATURE_OR_SOURCE_CLAIMS;NO_DESCRIBE;NO_QUALIFICATION"
const EvaluationMaxOccurrencesPrivate = 4096
const EvaluationMaxNodesPrivate = 8192
const EvaluationMaxBytesPrivate = 8 << 20
const EvaluationMaxStringPrivate = 4096

// EvaluationBindingsPrivate commits exact canonical bytes, not claimed digests.
type EvaluationBindingsPrivate struct {
	Input, Policy CompositionCommitment
}
type evaluationProjectionDocument struct {
	Schema, ClaimCeiling, UniverseScope, SourceNodeUniverse, Completeness, PolicyStatus string
	Authority                                                                           int
	Accepted                                                                            bool
	Bindings                                                                            EvaluationBindingsPrivate
	Nodes                                                                               []string
	Occurrences                                                                         []CompositionOccurrence
	PairContributions, SeparateLayerContributions                                       []PairContribution
	Custody                                                                             []EndpointCustody
	Accounting                                                                          compositionAccountingJSON
	PairWeightDigest                                                                    string
}

// EvaluationProjectionPrivate is opaque and detached. Its bytes are never a
// CompositionReceipt, VerifiedQualification, SourceBinding, or admitted Outcome.
type EvaluationProjectionPrivate struct {
	canonical []byte
	id        string
	doc       evaluationProjectionDocument
}

func (p EvaluationProjectionPrivate) Valid() bool     { return len(p.canonical) > 0 && p.id != "" }
func (p EvaluationProjectionPrivate) ID() string      { return p.id }
func (p EvaluationProjectionPrivate) Digest() string  { return p.id }
func (p EvaluationProjectionPrivate) ByteLength() int { return len(p.canonical) }
func (p EvaluationProjectionPrivate) CanonicalBytes() []byte {
	return append([]byte(nil), p.canonical...)
}
func (p EvaluationProjectionPrivate) Bindings() EvaluationBindingsPrivate { return p.doc.Bindings }
func (p EvaluationProjectionPrivate) NodeIdentities() []string {
	return append([]string(nil), p.doc.Nodes...)
}
func (p EvaluationProjectionPrivate) Occurrences() []CompositionOccurrence {
	return append([]CompositionOccurrence(nil), p.doc.Occurrences...)
}
func (p EvaluationProjectionPrivate) PairContributions() []PairContribution {
	return append([]PairContribution(nil), p.doc.PairContributions...)
}
func (p EvaluationProjectionPrivate) SeparateLayerContributions() []PairContribution {
	return append([]PairContribution(nil), p.doc.SeparateLayerContributions...)
}
func (p EvaluationProjectionPrivate) Custody() []EndpointCustody {
	return append([]EndpointCustody(nil), p.doc.Custody...)
}
func (p EvaluationProjectionPrivate) PairWeightDigest() string { return p.doc.PairWeightDigest }
func (p EvaluationProjectionPrivate) Accounting() CompositionAccounting {
	a := p.doc.Accounting
	return CompositionAccounting{a.Included, a.Excluded, a.Capped, a.Normalized, a.Downweighted, a.SeparateLayer, a.Duplicate, a.Terminal, a.Truncated}
}

// CheckEvaluationProjectionPrivate compares against an expected opaque value,
// never a caller-resealed hash. It deliberately does not decode untrusted JSON.
func CheckEvaluationProjectionPrivate(expected EvaluationProjectionPrivate, raw []byte) error {
	if !expected.Valid() || len(raw) > EvaluationMaxBytesPrivate || !bytes.Equal(expected.canonical, raw) {
		return errors.New("candidate projection binding mismatch")
	}
	return nil
}

func ProjectEvaluationPrivate(input AdmittedInput, policy GroupingPolicy, limits CompositionLimits) (EvaluationProjectionPrivate, error) {
	// Inspect bounded opaque internals before copying, sorting, allocating derived
	// collections, or performing rational products. Production limits are unchanged.
	if err := validateCompositionLimits(limits); err != nil {
		return EvaluationProjectionPrivate{}, err
	}
	if limits.MaxRawBytes > EvaluationMaxBytesPrivate || limits.MaxCanonicalBytes > EvaluationMaxBytesPrivate || limits.MaxOccurrences > EvaluationMaxOccurrencesPrivate || limits.MaxPairContributions > EvaluationMaxOccurrencesPrivate || limits.MaxPairs > EvaluationMaxOccurrencesPrivate || limits.MaxStringBytes > EvaluationMaxStringPrivate || limits.MaxDenominator > 1<<30 || limits.MaxAccumulator > 1<<50 {
		return EvaluationProjectionPrivate{}, errors.New("private evaluation limits exceed hard bounds")
	}
	d := input.artifact.doc
	if len(input.artifact.canonical) == 0 || len(policy.canonical) == 0 {
		return EvaluationProjectionPrivate{}, errors.New("opaque admitted input and candidate policy required")
	}
	if len(input.artifact.canonical) > limits.MaxRawBytes || len(policy.canonical) > limits.MaxRawBytes || len(d.Contributions) > limits.MaxOccurrences || len(d.Contributions) > limits.MaxPairContributions || len(d.EndpointCustody) > 2*EvaluationMaxOccurrencesPrivate || maxDocumentStringBytes(d) > limits.MaxStringBytes || maxPolicyString(policy.doc) > limits.MaxStringBytes {
		return EvaluationProjectionPrivate{}, errors.New("private evaluation input bound")
	}
	bindings := EvaluationBindingsPrivate{Input: CompositionCommitment{input.ID(), input.Digest(), input.ByteLength()}, Policy: CompositionCommitment{policy.ID(), policy.Digest(), policy.ByteLength()}}
	projected, err := projectPolicyDocument(input, policy, policy.doc.Epoch, CompositionBindings{InputID: input.ID(), InputDigest: input.Digest(), InputByteLength: input.ByteLength(), PolicyID: policy.ID(), PolicyDigest: policy.Digest(), PolicyByteLength: policy.ByteLength()}, limits)
	if err != nil {
		return EvaluationProjectionPrivate{}, err
	}
	if maxCompositionString(projected) > limits.MaxStringBytes {
		return EvaluationProjectionPrivate{}, errors.New("private projection string bound")
	}
	if err := validateCompositionReplay(projected); err != nil {
		return EvaluationProjectionPrivate{}, err
	}
	nodes := make(map[string]struct{}, 2*len(d.Contributions))
	for _, c := range d.Contributions {
		nodes[c.From] = struct{}{}
		nodes[c.To] = struct{}{}
	}
	if len(nodes) > EvaluationMaxNodesPrivate {
		return EvaluationProjectionPrivate{}, errors.New("private endpoint universe bound")
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	doc := evaluationProjectionDocument{Schema: EvaluationProjectionSchemaPrivate, ClaimCeiling: EvaluationCeilingPrivate, UniverseScope: EvaluationUniversePrivate, SourceNodeUniverse: "UNKNOWN", Completeness: "UNKNOWN", PolicyStatus: "UNQUALIFIED", Bindings: bindings, Nodes: ids, Occurrences: projected.Occurrences, PairContributions: projected.PairContributions, SeparateLayerContributions: projected.SeparateLayerContributions, Custody: projected.Custody, Accounting: projected.Accounting, PairWeightDigest: projected.PairWeightDigest}
	raw, err := json.Marshal(doc)
	if err != nil {
		return EvaluationProjectionPrivate{}, err
	}
	raw = append(raw, '\n')
	if len(raw) > limits.MaxCanonicalBytes || len(raw) > EvaluationMaxBytesPrivate {
		return EvaluationProjectionPrivate{}, errors.New("private projection canonical bound")
	}
	h := sha256.Sum256(raw)
	return EvaluationProjectionPrivate{raw, fmt.Sprintf("sha256:%x", h), doc}, nil
}

// CalibrationProbesPrivate retains hypotheses only. No winner, thresholds,
// campaign authorization, held-out labels, runtime, or policy qualification.
func CalibrationProbesPrivate(epoch uint64) ([]GroupingPolicy, error) {
	out := make([]GroupingPolicy, 0, 5)
	for _, probe := range []string{"H", "U", "L", "B", "N"} {
		rules := make([]GroupingPolicyRule, 0, 3)
		for _, kind := range []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
			r := GroupingPolicyRule{RelationKind: kind, Action: "INCLUDE", RationaleCode: "EVALUATION_PROBE_" + probe, Weight: Rational{1, 1}}
			if kind != "CALLS" {
				switch probe {
				case "H":
					r.Action = "EXCLUDE"
					r.Weight = Rational{0, 1}
				case "L":
					r.Action = "DOWNWEIGHT"
					r.Weight = Rational{1, 4}
				case "B":
					r.Action = "CAP"
					r.Cap = 4
				case "N":
					r.Action = "NORMALIZE"
					r.Normalization = "BY_KIND_MULTIPLICITY"
				}
			}
			rules = append(rules, r)
		}
		p, err := BuildGroupingPolicy(GroupingPolicyRequest{PolicyID: "PRIVATE_CALIBRATION_" + probe, Epoch: epoch, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1, MaxDenominator: 4, MaxCap: 4}})
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
