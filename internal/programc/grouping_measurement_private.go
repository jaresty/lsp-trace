package programc

// Controlled diagnostic integration only; never a qualification, source binding,
// execution grant, scientific criterion, or accepted semantic inventory.
import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"unicode/utf8"

	rg "lsp-trace/internal/referencegrouping"
)

const GroupingMeasurementContractPrivate = "CONTROLLED_UNORDERED_DISTINCT_ENDPOINT_PAIRS_V1;SAME_POSITIVE;DIFFERENT_NEGATIVE;ABSTAIN_EXCLUDED_FROM_CONFUSION;UNJUDGED_RETAINED;SUM_EXACT_COUNTS_NOT_MEAN_RATES;NO_QUALITY_CRITERIA"
const GroupingMeasurementMaxSlotsPrivate = 128
const GroupingMeasurementMaxJudgmentsPrivate = 4096
const groupingMeasurementMaxCount = 1 << 30

// Notification only: no source, case, partition, counters, report or mutable
// identity is exposed. Production leaves this nil. Controlled test workers may
// observe/block at these checkpoints; notification is never completion proof.
type groupingMeasurementStagePrivate uint8

const (
	measurementCheckerEntryPrivate groupingMeasurementStagePrivate = iota + 1
	measurementCaseEvaluatedPrivate
)

var groupingMeasurementObserverPrivate func(groupingMeasurementStagePrivate, int)

func notifyGroupingMeasurementPrivate(stage groupingMeasurementStagePrivate, ordinal int) {
	observer := groupingMeasurementObserverPrivate
	if observer == nil {
		return
	}
	// Diagnostics cannot skip required work or turn a panic into fake success.
	// This mirrors notification isolation: only the observer panic is recovered.
	defer func() { _ = recover() }()
	observer(stage, ordinal)
}

// A label is a controlled independent declaration, not source-derived truth.
type GroupingPairJudgmentPrivate struct{ Left, Right, Label string }
type groupingJudgmentRowPrivate struct{ ID, Left, Right, Label string }
type groupingJudgmentDocumentPrivate struct {
	Schema, JudgmentID, UniverseScope, SourceNodeUniverse, Semantics string
	Input                                                            rg.CompositionCommitment
	Nodes                                                            []string
	Pairs                                                            []groupingJudgmentRowPrivate
}
type GroupingJudgmentsPrivate struct {
	canonical []byte
	doc       groupingJudgmentDocumentPrivate
}

// The same frozen input/universe judgment can be shared across policy/seed runs.
// Actual measurements still commit the exact case, probe policy and run result.
func NewGroupingJudgmentsPrivate(c GroupingEvaluationCasePrivate, id string, pairs []GroupingPairJudgmentPrivate) (GroupingJudgmentsPrivate, error) {
	bad := func() (GroupingJudgmentsPrivate, error) {
		return GroupingJudgmentsPrivate{}, errors.New("invalid controlled judgment binding or bound")
	}
	if !c.Valid() || !measurementStringPrivate(id) || len(pairs) > GroupingMeasurementMaxJudgmentsPrivate {
		return bad()
	}
	for _, p := range pairs {
		if !measurementStringPrivate(p.Left) || !measurementStringPrivate(p.Right) || (p.Label != "SAME" && p.Label != "DIFFERENT" && p.Label != "ABSTAIN") {
			return bad()
		}
	}
	nodes := c.Projection().NodeIdentities()
	rows := make([]groupingJudgmentRowPrivate, 0, len(pairs))
	for _, p := range pairs {
		a, b := p.Left, p.Right
		if a > b {
			a, b = b, a
		}
		if a == b || !measurementNodePrivate(nodes, a) || !measurementNodePrivate(nodes, b) {
			return bad()
		}
		key, _ := json.Marshal([2]string{a, b})
		rows = append(rows, groupingJudgmentRowPrivate{groupingHashPrivate(key), a, b, p.Label})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Left != rows[j].Left {
			return rows[i].Left < rows[j].Left
		}
		return rows[i].Right < rows[j].Right
	})
	for i := 1; i < len(rows); i++ {
		if rows[i].Left == rows[i-1].Left && rows[i].Right == rows[i-1].Right {
			return bad()
		}
	}
	doc := groupingJudgmentDocumentPrivate{"lsp-trace.private.grouping-judgments.v1", id, rg.EvaluationUniversePrivate, "UNKNOWN", "CONTROLLED_INDEPENDENT_DECLARATION_NOT_SEMANTIC_TRUTH", c.doc.Bindings.Input, nodes, rows}
	raw, err := groupingCanonicalPrivate(doc)
	if err != nil {
		return GroupingJudgmentsPrivate{}, err
	}
	return GroupingJudgmentsPrivate{raw, doc}, nil
}
func (j GroupingJudgmentsPrivate) ID() string {
	if len(j.canonical) == 0 {
		return ""
	}
	return groupingHashPrivate(j.canonical)
}
func (j GroupingJudgmentsPrivate) CanonicalBytes() []byte { return append([]byte(nil), j.canonical...) }

type GroupingMeasurementSlotPrivate struct {
	Case      GroupingEvaluationCasePrivate
	Judgments GroupingJudgmentsPrivate
}
type GroupingMeasurementBatchRequestPrivate struct {
	BatchID, ImplementationID string
	ImplementationBytes       []byte
	Slots                     []GroupingMeasurementSlotPrivate
}
type groupingMeasurementScheduleSlotPrivate struct {
	Case      []byte
	Judgments []byte
	Arm       string
}
type groupingMeasurementSchedulePrivate struct {
	Schema, BatchID, Contract, ImplementationSemantics string
	Implementation                                     rg.CompositionCommitment
	ImplementationID                                   string
	ImplementationBytes                                []byte
	Slots                                              []groupingMeasurementScheduleSlotPrivate
}
type GroupingMeasurementBatchPrivate struct {
	canonical []byte
	doc       groupingMeasurementSchedulePrivate
	slots     []GroupingMeasurementSlotPrivate
}

func (b GroupingMeasurementBatchPrivate) ID() string {
	if len(b.canonical) == 0 {
		return ""
	}
	return groupingHashPrivate(b.canonical)
}
func (b GroupingMeasurementBatchPrivate) CanonicalBytes() []byte {
	return append([]byte(nil), b.canonical...)
}
func measurementStringPrivate(s string) bool {
	return s != "" && len(s) <= rg.EvaluationMaxStringPrivate && utf8.ValidString(s)
}
func measurementNodePrivate(nodes []string, s string) bool {
	i := sort.SearchStrings(nodes, s)
	return i < len(nodes) && nodes[i] == s
}
func measurementCommitmentPrivate(raw []byte) rg.CompositionCommitment {
	h := groupingHashPrivate(raw)
	return rg.CompositionCommitment{ID: h, Digest: h, ByteLength: len(raw)}
}
func measurementArmPrivate(c GroupingEvaluationCasePrivate) string {
	kinds := map[string]bool{}
	for _, o := range c.projection.Occurrences() {
		kinds[o.Kind] = true
	}
	out := make([]string, 0, len(kinds))
	for k := range kinds {
		out = append(out, k)
	}
	sort.Strings(out)
	raw, _ := json.Marshal(out)
	return string(raw)
}
func NewGroupingMeasurementBatchPrivate(r GroupingMeasurementBatchRequestPrivate) (GroupingMeasurementBatchPrivate, error) {
	bad := func(s string) (GroupingMeasurementBatchPrivate, error) {
		return GroupingMeasurementBatchPrivate{}, &GroupingMeasurementFailurePrivate{"BLOCKED_SCHEDULE", s}
	}
	if len(r.Slots) == 0 || len(r.ImplementationBytes) == 0 {
		return GroupingMeasurementBatchPrivate{}, &GroupingMeasurementFailurePrivate{"INCOMPLETE_SCHEDULE", "missing schedule or implementation declaration"}
	}
	if !measurementStringPrivate(r.BatchID) || !measurementStringPrivate(r.ImplementationID) || len(r.ImplementationBytes) > 1<<16 || len(r.Slots) > GroupingMeasurementMaxSlotsPrivate {
		return bad("missing/oversized immutable schedule or implementation")
	}
	// All budgets are admitted before derived schedule allocation, sorting or pair
	// arithmetic. Existing opaque cases already enforce endpoint/string bounds.
	budget, totalPairs := len(r.ImplementationBytes), 0
	for _, s := range r.Slots {
		if !s.Case.Valid() || len(s.Judgments.canonical) == 0 {
			return bad("missing case/judgments")
		}
		c, j := s.Case, s.Judgments
		if c.doc.Bindings.Input != j.doc.Input || !equalMeasurementNodesPrivate(c.projection.NodeIdentities(), j.doc.Nodes) {
			return bad("judgments do not bind exact input/universe")
		}
		// Conservative aggregate encoded-work budget, not kernel RAM qualification.
		for _, n := range []int{len(c.canonical), c.projection.ByteLength(), len(j.canonical)} {
			if n > rg.EvaluationMaxBytesPrivate/8 || budget > rg.EvaluationMaxBytesPrivate-8*n {
				return bad("aggregate byte bound")
			}
			budget += 8 * n
		}
		n := len(j.doc.Nodes)
		if n > rg.EvaluationMaxNodesPrivate {
			return bad("node bound")
		}
		if n > 0 && n-1 > groupingMeasurementMaxCount/n {
			return bad("pair product bound")
		}
		count := n * (n - 1) / 2
		if totalPairs > groupingMeasurementMaxCount-count {
			return bad("aggregate count bound")
		}
		totalPairs += count
	}
	slots := make([]GroupingMeasurementSlotPrivate, len(r.Slots))
	copy(slots, r.Slots)
	doc := groupingMeasurementSchedulePrivate{Schema: "lsp-trace.private.grouping-measurement-schedule.v1", BatchID: r.BatchID, Contract: GroupingMeasurementContractPrivate, ImplementationSemantics: "DECLARED_EXACT_BYTES_NOT_BINARY_OR_EXECUTION_ATTESTATION", Implementation: measurementCommitmentPrivate(r.ImplementationBytes), ImplementationID: r.ImplementationID, ImplementationBytes: append([]byte(nil), r.ImplementationBytes...)}
	seen := make(map[[2]string]bool, len(slots))
	for _, s := range slots {
		key := [2]string{s.Case.doc.CaseID, s.Case.doc.RunID}
		if seen[key] {
			return bad("duplicate case/run slot")
		}
		seen[key] = true
		doc.Slots = append(doc.Slots, groupingMeasurementScheduleSlotPrivate{s.Case.CanonicalBytes(), s.Judgments.CanonicalBytes(), measurementArmPrivate(s.Case)})
	}
	raw, err := groupingCanonicalPrivate(doc)
	if err != nil {
		return GroupingMeasurementBatchPrivate{}, err
	}
	return GroupingMeasurementBatchPrivate{raw, doc, slots}, nil
}
func equalMeasurementNodesPrivate(a, b []string) bool {
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

type GroupingPairCountsPrivate struct{ UniversePairs, Declared, Decisive, Abstained, Unjudged, TruePositive, FalsePositive, TrueNegative, FalseNegative int }
type GroupingMeasurementRatePrivate struct {
	Code, Status, Aggregation string
	Numerator, Denominator    int
}
type GroupingMeasurementRowPrivate struct {
	Case, Result, Judgments rg.CompositionCommitment
	Arm, Status             string
	ResultBytes             []byte
	Counts                  GroupingPairCountsPrivate
	Rates                   []GroupingMeasurementRatePrivate
}
type GroupingMeasurementArmPrivate struct {
	Arm, Status string
	Slots       int
	Counts      GroupingPairCountsPrivate
	Rates       []GroupingMeasurementRatePrivate
}
type GroupingMeasurementDocumentPrivate struct {
	Schema, Contract, ClaimCeiling, FeatureIdentity, Completeness, SourceNodeUniverse, AbsenceInference, UniverseScope, DownstreamReadiness string
	Authority                                                                                                                               int
	Accepted                                                                                                                                bool
	Schedule                                                                                                                                rg.CompositionCommitment
	ScheduleBytes                                                                                                                           []byte
	Rows                                                                                                                                    []GroupingMeasurementRowPrivate
	Arms                                                                                                                                    []GroupingMeasurementArmPrivate
	NotComputed                                                                                                                             []string
}
type GroupingMeasurementResultPrivate struct{ canonical []byte }

func (r GroupingMeasurementResultPrivate) ID() string {
	if len(r.canonical) == 0 {
		return ""
	}
	return groupingHashPrivate(r.canonical)
}
func (r GroupingMeasurementResultPrivate) CanonicalBytes() []byte {
	return append([]byte(nil), r.canonical...)
}
func (r GroupingMeasurementResultPrivate) Document() GroupingMeasurementDocumentPrivate {
	var d GroupingMeasurementDocumentPrivate
	_ = json.Unmarshal(r.canonical, &d)
	return d
}

// Sequential execution only. The immutable schedule is the expected experiment;
// neither caller partitions nor caller counters are execution inputs.
func ExecuteGroupingMeasurementBatchPrivate(b GroupingMeasurementBatchPrivate) (GroupingMeasurementResultPrivate, error) {
	if len(b.canonical) == 0 || len(b.slots) == 0 || len(b.slots) > GroupingMeasurementMaxSlotsPrivate {
		return GroupingMeasurementResultPrivate{}, &GroupingMeasurementFailurePrivate{"INCOMPLETE_SCHEDULE", "opaque nonempty schedule required"}
	}
	d := GroupingMeasurementDocumentPrivate{Schema: "lsp-trace.private.grouping-measurements.v1", Contract: GroupingMeasurementContractPrivate, ClaimCeiling: rg.EvaluationCeilingPrivate, FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN", SourceNodeUniverse: "UNKNOWN", AbsenceInference: "PROHIBITED", UniverseScope: rg.EvaluationUniversePrivate, DownstreamReadiness: "INCOMPLETE", Schedule: measurementCommitmentPrivate(b.canonical), ScheduleBytes: b.CanonicalBytes()}
	// Codes below are the existing report's required inventory. None has an
	// approved experimental definition here. Pair diagnostics do not fill them.
	d.NotComputed = []string{"FALSE_MERGE_RATE", "FRAGMENTATION_RATE", "STABILITY_RATE", "UNMATCHED_NO_COMMUNITY_RATE", "SHARED_NODE_EFFECT_RATE", "HIGH_DEGREE_COLLAPSE_RATE", "RESOURCE_LIMIT_COMPLIANCE", "DETERMINISTIC_REPLAY", "INPUT_PERMUTATION_STABILITY", "CROSS_SEED_INSTABILITY_CALLS_ONLY", "CROSS_SEED_INSTABILITY_REFERENCES_ONLY", "CROSS_SEED_INSTABILITY_DEFINITIONS_ONLY", "CROSS_SEED_INSTABILITY_REFERENCES_PLUS_DEFINITIONS", "CROSS_SEED_INSTABILITY_CALLS_PLUS_REFERENCES", "CROSS_SEED_INSTABILITY_CALLS_PLUS_DEFINITIONS", "CROSS_SEED_INSTABILITY_CALLS_PLUS_REFERENCES_PLUS_DEFINITIONS"}
	arms := make(map[string]GroupingMeasurementArmPrivate)
	for _, slot := range b.slots {
		c := slot.Case
		if err := CheckGroupingCasePrivate(c, c.CanonicalBytes()); err != nil {
			return GroupingMeasurementResultPrivate{}, err
		}
		result, err := EvaluateGroupingCasePrivate(c)
		if err != nil {
			return GroupingMeasurementResultPrivate{}, err
		}
		if err = CheckGroupingEvaluationPrivate(c, result.CanonicalBytes()); err != nil {
			return GroupingMeasurementResultPrivate{}, err
		}
		counts, err := measureGroupingPairsPrivate(result.Partition(), slot.Judgments.doc)
		if err != nil {
			return GroupingMeasurementResultPrivate{}, err
		}
		arm := measurementArmPrivate(c)
		row := GroupingMeasurementRowPrivate{Case: measurementCommitmentPrivate(c.canonical), Result: measurementCommitmentPrivate(result.canonical), Judgments: measurementCommitmentPrivate(slot.Judgments.canonical), Arm: arm, Status: measurementCoveragePrivate(counts), ResultBytes: result.CanonicalBytes(), Counts: counts, Rates: measurementRatesPrivate(counts)}
		d.Rows = append(d.Rows, row)
		a := arms[arm]
		a.Arm = arm
		a.Slots++
		a.Counts, err = addMeasurementCountsPrivate(a.Counts, counts)
		if err != nil {
			return GroupingMeasurementResultPrivate{}, err
		}
		arms[arm] = a
		notifyGroupingMeasurementPrivate(measurementCaseEvaluatedPrivate, len(d.Rows))
	}
	keys := make([]string, 0, len(arms))
	for k := range arms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := arms[k]
		a.Status = measurementCoveragePrivate(a.Counts)
		a.Rates = measurementRatesPrivate(a.Counts)
		d.Arms = append(d.Arms, a)
	}
	raw, err := groupingCanonicalPrivate(d)
	if err != nil {
		return GroupingMeasurementResultPrivate{}, err
	}
	return GroupingMeasurementResultPrivate{raw}, nil
}

// Typed blocking is distinct from a zero-denominator diagnostic state, which
// remains represented as NOT_COMPUTED rather than a favorable numeric value.
type GroupingMeasurementFailurePrivate struct{ Code, Detail string }

func (e *GroupingMeasurementFailurePrivate) Error() string { return e.Code + ": " + e.Detail }
func measureGroupingPairsPrivate(p PartitionMathPrivate, j groupingJudgmentDocumentPrivate) (GroupingPairCountsPrivate, error) {
	bad := func() (GroupingPairCountsPrivate, error) {
		return GroupingPairCountsPrivate{}, &GroupingMeasurementFailurePrivate{"BLOCKED_PARTITION_UNIVERSE", "partition must cover exact judged endpoint universe once"}
	}
	if len(j.Nodes) > rg.EvaluationMaxNodesPrivate || len(j.Pairs) > GroupingMeasurementMaxJudgmentsPrivate {
		return bad()
	}
	membership := make(map[string]int, len(j.Nodes))
	for i, c := range p.Communities {
		for _, n := range c.Members {
			if _, ok := membership[n]; ok || !measurementNodePrivate(j.Nodes, n) {
				return bad()
			}
			membership[n] = i
		}
	}
	if len(membership) != len(j.Nodes) {
		return bad()
	}
	n := len(j.Nodes)
	if n > 0 && n-1 > groupingMeasurementMaxCount/n {
		return bad()
	}
	counts := GroupingPairCountsPrivate{UniversePairs: n * (n - 1) / 2, Declared: len(j.Pairs)}
	for _, pair := range j.Pairs {
		together := membership[pair.Left] == membership[pair.Right]
		switch pair.Label {
		case "ABSTAIN":
			counts.Abstained++
		case "SAME":
			if together {
				counts.TruePositive++
			} else {
				counts.FalseNegative++
			}
		case "DIFFERENT":
			if together {
				counts.FalsePositive++
			} else {
				counts.TrueNegative++
			}
		default:
			return bad()
		}
	}
	counts.Decisive = counts.TruePositive + counts.FalsePositive + counts.TrueNegative + counts.FalseNegative
	counts.Unjudged = counts.UniversePairs - counts.Declared
	if counts.Unjudged < 0 {
		return bad()
	}
	return counts, nil
}
func addMeasurementCountsPrivate(a, b GroupingPairCountsPrivate) (GroupingPairCountsPrivate, error) {
	x := []int{a.UniversePairs, a.Declared, a.Decisive, a.Abstained, a.Unjudged, a.TruePositive, a.FalsePositive, a.TrueNegative, a.FalseNegative}
	y := []int{b.UniversePairs, b.Declared, b.Decisive, b.Abstained, b.Unjudged, b.TruePositive, b.FalsePositive, b.TrueNegative, b.FalseNegative}
	for i, v := range y {
		if v < 0 || v > groupingMeasurementMaxCount || x[i] > groupingMeasurementMaxCount-v {
			return GroupingPairCountsPrivate{}, &GroupingMeasurementFailurePrivate{"BLOCKED_COUNT_BOUND", "aggregate exact count bound"}
		}
		x[i] += v
	}
	return GroupingPairCountsPrivate{x[0], x[1], x[2], x[3], x[4], x[5], x[6], x[7], x[8]}, nil
}
func measurementCoveragePrivate(c GroupingPairCountsPrivate) string {
	if c.Decisive == 0 {
		return "INCOMPLETE_NO_DECISIVE_JUDGMENTS"
	}
	if c.Unjudged > 0 || c.Abstained > 0 {
		return "INCOMPLETE_FINITE_JUDGED_DOMAIN"
	}
	return "FINITE_DECLARED_DOMAIN_COVERED_NOT_SEMANTIC_TRUTH"
}
func measurementRatesPrivate(c GroupingPairCountsPrivate) []GroupingMeasurementRatePrivate {
	// Every reduced fraction preserves its unreduced denominator in Counts.
	// Aggregation sums counters across scheduled runs, not independent samples,
	// unique-pair coverage across runs, averages of fractions, or acceptance tests.
	specs := []struct {
		code string
		n, d int
	}{
		{"CONTROLLED_PAIR_FALSE_MERGE", c.FalsePositive, c.TruePositive + c.FalsePositive},
		{"CONTROLLED_PAIR_FRAGMENTATION", c.FalseNegative, c.TruePositive + c.FalseNegative},
		{"CONTROLLED_DECLARED_PAIR_COVERAGE", c.Declared, c.UniversePairs},
		{"CONTROLLED_DECISIVE_PAIR_COVERAGE", c.Decisive, c.UniversePairs},
		{"CONTROLLED_ABSTENTION", c.Abstained, c.Declared},
	}
	out := make([]GroupingMeasurementRatePrivate, 0, len(specs))
	for _, s := range specs {
		r := GroupingMeasurementRatePrivate{Code: s.code, Status: "NOT_COMPUTED_ZERO_DENOMINATOR", Aggregation: "SUM_EXACT_COUNTS", Numerator: s.n, Denominator: s.d}
		if s.d > 0 {
			// Actual existing downstream types and reduction, with a diagnostic code.
			m, err := rg.SumExactCountsPrivate(s.code, []rg.ExactCountObservationPrivate{{CaseID: "controlled-count-domain", MetricCode: s.code, Numerator: s.n, Denominator: s.d}})
			if err == nil {
				r.Numerator = m.Numerator
				r.Denominator = m.Denominator
				r.Status = "COMPUTED_CONTROLLED_DIAGNOSTIC"
			} else {
				r.Status = "NOT_COMPUTED_INVALID_COUNTS"
			}
		}
		out = append(out, r)
	}
	return out
}
func CheckGroupingMeasurementBatchPrivate(b GroupingMeasurementBatchPrivate, raw []byte) error {
	notifyGroupingMeasurementPrivate(measurementCheckerEntryPrivate, 0)
	if len(b.canonical) == 0 || len(raw) == 0 || len(raw) > rg.EvaluationMaxBytesPrivate {
		return errors.New("missing/oversized expected batch or report")
	}
	result, err := ExecuteGroupingMeasurementBatchPrivate(b)
	if err != nil {
		return err
	}
	if !bytes.Equal(result.canonical, raw) {
		return errors.New("report is not exact scheduled evaluation and measurement replay")
	}
	return nil
}
