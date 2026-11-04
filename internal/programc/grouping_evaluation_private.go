package programc

// Evaluation-only adapter. No exported authority callbacks, qualifications,
// source bindings, ordinary Outcomes, or calls-v1 numerical serialization.
import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"

	rg "lsp-trace/internal/referencegrouping"
)

const GroupingEvaluationSchemaPrivate = "lsp-trace.private.grouping-evaluation-result.v1"
const GroupingEvaluationCaseSchemaPrivate = "lsp-trace.private.grouping-evaluation-case.v1"

// Exact integer aggregation precedes conversion; base positive directed pairs
// only, separate layers excluded. Ordering is lexical, resolution is 1, and the
// existing PCG(seed,seed^constant) partitionMath kernel owns numerical behavior.
const GroupingNumericContractPrivate = "typed-directed-base-rational-i64checked-to-f64-v1;N8192;O4096;D1073741824;A1125899906842624;LEIDEN_RESOLUTION_1;EXISTING_SEEDED_PARTITION_MATH"
const groupingNumericMax = 1 << 50
const groupingDenominatorMax = 1 << 30

// Selector bytes are owner-declared selection, not proof of executed selection.
// Freeze this opaque case before evaluation; a newly declared case (even with
// the same label) is a different experiment, not a replacement for the frozen one.
type GroupingEvaluationCaseRequestPrivate struct {
	CaseID, SelectorID, RunID string
	SelectorBytes             []byte
	Input                     rg.AdmittedInput
	Policy                    rg.GroupingPolicy
	ProjectionLimits          rg.CompositionLimits
	Seed                      uint64
}
type groupingSelectorPrivate struct {
	ID, Digest string
	ByteLength int
	Bytes      []byte
	Semantics  string
}
type groupingCaseDocumentPrivate struct {
	Schema, CaseID, RunID, NumericContract, UniverseScope, SourceNodeUniverse string
	Seed                                                                      uint64
	Selector                                                                  groupingSelectorPrivate
	Bindings                                                                  rg.EvaluationBindingsPrivate
	Projection                                                                rg.CompositionCommitment
}
type GroupingEvaluationCasePrivate struct {
	canonical  []byte
	id         string
	doc        groupingCaseDocumentPrivate
	projection rg.EvaluationProjectionPrivate
}
type groupingPairPrivate struct {
	From, To string
	Weight   rg.Rational
}
type groupingResultDocumentPrivate struct {
	Schema, ClaimCeiling, UniverseScope, SourceNodeUniverse, Completeness, PolicyStatus, ActiveProjection string
	Authority                                                                                             int
	Accepted                                                                                              bool
	Case                                                                                                  rg.CompositionCommitment
	Declaration                                                                                           groupingCaseDocumentPrivate
	Pairs                                                                                                 []groupingPairPrivate
	ActiveOccurrenceCount                                                                                 int
	Partition                                                                                             PartitionMathPrivate
}
type GroupingEvaluationResultPrivate struct {
	canonical []byte
	id        string
	doc       groupingResultDocumentPrivate
}

func (c GroupingEvaluationCasePrivate) Valid() bool {
	return len(c.canonical) > 0 && c.projection.Valid()
}
func (c GroupingEvaluationCasePrivate) ID() string { return c.id }
func (c GroupingEvaluationCasePrivate) CanonicalBytes() []byte {
	return append([]byte(nil), c.canonical...)
}
func (c GroupingEvaluationCasePrivate) Projection() rg.EvaluationProjectionPrivate {
	return c.projection
}
func (r GroupingEvaluationResultPrivate) ID() string      { return r.id }
func (r GroupingEvaluationResultPrivate) Digest() string  { return r.id }
func (r GroupingEvaluationResultPrivate) ByteLength() int { return len(r.canonical) }
func (r GroupingEvaluationResultPrivate) CanonicalBytes() []byte {
	return append([]byte(nil), r.canonical...)
}
func (r GroupingEvaluationResultPrivate) Partition() PartitionMathPrivate {
	return ClonePartitionMathPrivate(r.doc.Partition)
}

func NewGroupingEvaluationCasePrivate(r GroupingEvaluationCaseRequestPrivate) (GroupingEvaluationCasePrivate, error) {
	for _, s := range []string{r.CaseID, r.SelectorID, r.RunID} {
		if len(s) == 0 || len(s) > rg.EvaluationMaxStringPrivate || !utf8.ValidString(s) {
			return GroupingEvaluationCasePrivate{}, errors.New("missing or oversized case/selector/run binding")
		}
	}
	if len(r.SelectorBytes) == 0 || len(r.SelectorBytes) > 1<<16 {
		return GroupingEvaluationCasePrivate{}, errors.New("missing or oversized immutable selector bytes")
	}
	p, err := rg.ProjectEvaluationPrivate(r.Input, r.Policy, r.ProjectionLimits)
	if err != nil {
		return GroupingEvaluationCasePrivate{}, err
	}
	doc := groupingCaseDocumentPrivate{Schema: GroupingEvaluationCaseSchemaPrivate, CaseID: r.CaseID, RunID: r.RunID, NumericContract: GroupingNumericContractPrivate, UniverseScope: rg.EvaluationUniversePrivate, SourceNodeUniverse: "UNKNOWN", Seed: r.Seed, Selector: groupingSelectorPrivate{ID: r.SelectorID, Digest: groupingHashPrivate(r.SelectorBytes), ByteLength: len(r.SelectorBytes), Bytes: append([]byte(nil), r.SelectorBytes...), Semantics: "DECLARED_SELECTION_ONLY_NOT_EXECUTION_PROOF"}, Bindings: p.Bindings(), Projection: rg.CompositionCommitment{ID: p.ID(), Digest: p.Digest(), ByteLength: p.ByteLength()}}
	raw, err := groupingCanonicalPrivate(doc)
	if err != nil {
		return GroupingEvaluationCasePrivate{}, err
	}
	return GroupingEvaluationCasePrivate{raw, groupingHashPrivate(raw), doc, p}, nil
}

// CheckGroupingCasePrivate and CheckGroupingEvaluationPrivate require the exact
// expected immutable declaration. They do not deserialize, trust resealed hashes,
// infer measurements from labels, or admit a foreign experiment under this case.
func CheckGroupingCasePrivate(expected GroupingEvaluationCasePrivate, raw []byte) error {
	if !expected.Valid() || len(raw) > rg.EvaluationMaxBytesPrivate || !bytes.Equal(raw, expected.canonical) {
		return errors.New("frozen case binding mismatch")
	}
	return nil
}
func CheckGroupingEvaluationPrivate(expected GroupingEvaluationCasePrivate, raw []byte) error {
	if !expected.Valid() || len(raw) == 0 || len(raw) > rg.EvaluationMaxBytesPrivate {
		return errors.New("missing or oversized result/case binding")
	}
	actual, err := EvaluateGroupingCasePrivate(expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual.canonical, raw) {
		return errors.New("result is not the exact replay of the frozen case")
	}
	return nil
}

func EvaluateGroupingCasePrivate(c GroupingEvaluationCasePrivate) (GroupingEvaluationResultPrivate, error) {
	if !c.Valid() {
		return GroupingEvaluationResultPrivate{}, errors.New("opaque frozen case required")
	}
	numeric, pairs, err := groupingNumericalProjectionPrivate(c.projection.NodeIdentities(), c.projection.PairContributions())
	if err != nil {
		return GroupingEvaluationResultPrivate{}, err
	}
	// Occurrences contain only real included identity/from/to/weight. No invented
	// relation ID, method, provider, version, language, or call site is supplied.
	// The opaque projection retains original typed kind and custody separately.
	partition, failure := partitionMath(numeric, c.doc.Seed)
	if failure != nil {
		return GroupingEvaluationResultPrivate{}, failure
	}
	active := "NONEMPTY_ACTIVE_PROJECTION"
	if len(numeric.Occurrences) == 0 {
		active = "EMPTY_ACTIVE_PROJECTION_NOT_SOURCE_ABSENCE"
	}
	doc := groupingResultDocumentPrivate{Schema: GroupingEvaluationSchemaPrivate, ClaimCeiling: rg.EvaluationCeilingPrivate, UniverseScope: rg.EvaluationUniversePrivate, SourceNodeUniverse: "UNKNOWN", Completeness: "UNKNOWN", PolicyStatus: "UNQUALIFIED", ActiveProjection: active, Case: rg.CompositionCommitment{ID: c.id, Digest: c.id, ByteLength: len(c.canonical)}, Declaration: c.doc, Pairs: pairs, ActiveOccurrenceCount: len(numeric.Occurrences), Partition: partition}
	raw, err := groupingCanonicalPrivate(doc)
	if err != nil {
		return GroupingEvaluationResultPrivate{}, err
	}
	return GroupingEvaluationResultPrivate{raw, groupingHashPrivate(raw), doc}, nil
}

func groupingCanonicalPrivate(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(raw) >= rg.EvaluationMaxBytesPrivate {
		return nil, errors.New("private canonical result bound")
	}
	return append(raw, '\n'), nil
}
func groupingHashPrivate(raw []byte) string {
	h := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", h)
}

func groupingNumericalProjectionPrivate(nodes []string, contributions []rg.PairContribution) (Projection, []groupingPairPrivate, error) {
	bad := func(msg string) (Projection, []groupingPairPrivate, error) { return Projection{}, nil, errors.New(msg) }
	// Bounds precede all derived map/slice allocations, sorting and arithmetic.
	if len(nodes) > rg.EvaluationMaxNodesPrivate || len(contributions) > rg.EvaluationMaxOccurrencesPrivate {
		return bad("private numerical count bound")
	}
	for i, id := range nodes {
		if id == "" || len(id) > rg.EvaluationMaxStringPrivate || (i > 0 && nodes[i-1] >= id) {
			return bad("invalid canonical numerical endpoint universe")
		}
	}
	for _, c := range contributions {
		if c.OccurrenceIdentity == "" || len(c.OccurrenceIdentity) > rg.EvaluationMaxStringPrivate || c.From == "" || c.To == "" || len(c.From) > rg.EvaluationMaxStringPrivate || len(c.To) > rg.EvaluationMaxStringPrivate || c.Layer != "" || (c.Kind != "CALLS" && c.Kind != "REFERENCES_SYMBOL" && c.Kind != "RESOLVES_TO_DEFINITION") {
			return bad("invalid detached numerical record")
		}
		if !groupingValidRationalPrivate(c.Weight) {
			return bad("invalid or unbounded rational contribution")
		}
	}
	p := Projection{NodeIdentities: append([]string(nil), nodes...), NodeIDs: make(map[string]int64, len(nodes)), PairWeights: make(map[Pair]float64)}
	for i, id := range nodes {
		p.NodeIDs[id] = int64(i)
	}
	sums := make(map[Pair]rg.Rational)
	seen := make(map[string]bool, len(contributions))
	total := rg.Rational{Numerator: 0, Denominator: 1}
	for _, c := range contributions {
		from, ok1 := p.NodeIDs[c.From]
		to, ok2 := p.NodeIDs[c.To]
		if !ok1 || !ok2 || seen[c.OccurrenceIdentity] {
			return bad("invalid endpoint or duplicate numeric identity")
		}
		seen[c.OccurrenceIdentity] = true
		pair := Pair{From: from, To: to}
		sum := sums[pair]
		if sum.Denominator == 0 {
			sum = rg.Rational{Numerator: 0, Denominator: 1}
		}
		var err error
		sums[pair], err = groupingAddRationalPrivate(sum, c.Weight)
		if err != nil {
			return bad(err.Error())
		}
		total, err = groupingAddRationalPrivate(total, c.Weight)
		if err != nil {
			return bad(err.Error())
		}
	}
	// No float conversion occurs until every exact pair and total sum passes.
	// Graph arcs use the aggregated pair conversion, never rounded float sums.
	for _, c := range contributions {
		w, err := groupingFloatPrivate(c.Weight)
		if err != nil {
			return bad(err.Error())
		}
		p.Occurrences = append(p.Occurrences, Occurrence{Identity: c.OccurrenceIdentity, From: p.NodeIDs[c.From], To: p.NodeIDs[c.To], Weight: w})
	}
	keys := make([]Pair, 0, len(sums))
	for pair := range sums {
		keys = append(keys, pair)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].From != keys[j].From {
			return keys[i].From < keys[j].From
		}
		return keys[i].To < keys[j].To
	})
	pairs := make([]groupingPairPrivate, 0, len(keys))
	for _, pair := range keys {
		w, err := groupingFloatPrivate(sums[pair])
		if err != nil {
			return bad(err.Error())
		}
		p.PairWeights[pair] = w
		pairs = append(pairs, groupingPairPrivate{From: nodes[pair.From], To: nodes[pair.To], Weight: sums[pair]})
	}
	return p, pairs, nil
}
func groupingGCDPrivate(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
func groupingValidRationalPrivate(r rg.Rational) bool {
	return r.Numerator > 0 && r.Numerator <= groupingNumericMax && r.Denominator > 0 && r.Denominator <= groupingDenominatorMax && groupingGCDPrivate(r.Numerator, r.Denominator) == 1
}
func groupingAddRationalPrivate(a, b rg.Rational) (rg.Rational, error) {
	// a may be the zero accumulator; b must be a positive reduced contribution.
	if a.Numerator < 0 || a.Numerator > groupingNumericMax || a.Denominator <= 0 || a.Denominator > groupingDenominatorMax || !groupingValidRationalPrivate(b) {
		return rg.Rational{}, errors.New("invalid exact rational accumulator")
	}
	g := groupingGCDPrivate(a.Denominator, b.Denominator)
	af, bf := b.Denominator/g, a.Denominator/g
	maxInt := int(^uint(0) >> 1)
	if a.Denominator > maxInt/af || (a.Numerator != 0 && a.Numerator > maxInt/af) || b.Numerator > maxInt/bf {
		return rg.Rational{}, errors.New("exact rational crossproduct overflow")
	}
	den := a.Denominator * af
	an, bn := a.Numerator*af, b.Numerator*bf
	if an > maxInt-bn {
		return rg.Rational{}, errors.New("exact rational sum overflow")
	}
	n := an + bn
	g = groupingGCDPrivate(n, den)
	n /= g
	den /= g
	if n > groupingNumericMax || den > groupingDenominatorMax {
		return rg.Rational{}, errors.New("exact rational aggregate bound")
	}
	return rg.Rational{Numerator: n, Denominator: den}, nil
}
func groupingFloatPrivate(r rg.Rational) (float64, error) {
	if !groupingValidRationalPrivate(r) {
		return 0, errors.New("invalid rational before float conversion")
	}
	f := float64(r.Numerator) / float64(r.Denominator)
	if err := groupingFinitePositivePrivate(f); err != nil {
		return 0, err
	}
	return f, nil
}
func groupingFinitePositivePrivate(f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
		return errors.New("float overflow, underflow or nonpositive weight")
	}
	return nil
}
