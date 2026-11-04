package programc

// Offline controlled evidence transport, not an admission owner, provider receipt,
// source-completeness claim, execution grant or qualification. Every slot carries
// original evidence bytes and reconstructs via the existing admission boundary.
import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	rg "lsp-trace/internal/referencegrouping"
)

const GroupingCaseBundleSchemaPrivate = "lsp-trace.private.grouping-case-bundle.v1"
const GroupingCaseBundleMaxBytesPrivate = 8 << 20
const GroupingCaseBundleMaxSlotsPrivate = 128
const GroupingCaseBundleMaxPredecessorsPrivate = 16
const GroupingCaseBundleMaxJudgmentsPrivate = 128

// All byte fields are base64 encoded by encoding/json: exact LF and acquisition
// bytes survive reopening. CaseBytes and JudgmentBytes are mandatory independent
// bindings to the already frozen declarations, checked against constructors.
// SelectorBytes declare selection, not execution of a query. Pairs are independent
// controlled labels, not inferred from the computed partition or authenticated truth.
type GroupingCaseBundleSlotPrivate struct {
	InputBytes, PolicyBytes   []byte
	Predecessors              []rg.PredecessorRecord
	CaseID, SelectorID, RunID string
	SelectorBytes             []byte
	ProjectionLimits          rg.CompositionLimits
	Seed                      uint64
	JudgmentID                string
	Pairs                     []GroupingPairJudgmentPrivate
	CaseBytes, JudgmentBytes  []byte
}
type GroupingCaseBundleRequestPrivate struct {
	Schema, BatchID, ImplementationID string
	ImplementationBytes               []byte
	Slots                             []GroupingCaseBundleSlotPrivate
}
type GroupingCaseBundlePrivate struct {
	canonical []byte
	batch     GroupingMeasurementBatchPrivate
}

func (b GroupingCaseBundlePrivate) CanonicalBytes() []byte {
	return append([]byte(nil), b.canonical...)
}
func (b GroupingCaseBundlePrivate) Commitment() rg.CompositionCommitment {
	return measurementCommitmentPrivate(b.canonical)
}
func (b GroupingCaseBundlePrivate) Batch() GroupingMeasurementBatchPrivate { return b.batch }

// New reconstructs first, then freezes a detached canonical declaration. No
// caller-authored partition or counter is accepted. The returned commitment must
// be kept separately as trusted expectation before reading a candidate bundle.
func NewGroupingCaseBundlePrivate(r GroupingCaseBundleRequestPrivate) (GroupingCaseBundlePrivate, error) {
	if err := bundleRequestBoundsPrivate(r); err != nil {
		return GroupingCaseBundlePrivate{}, err
	}
	batch, err := reconstructBundlePrivate(r)
	if err != nil {
		return GroupingCaseBundlePrivate{}, err
	}
	raw, err := groupingCanonicalPrivate(r)
	if err != nil {
		return GroupingCaseBundlePrivate{}, err
	}
	if err = bundleJSONPreflightPrivate(raw); err != nil {
		return GroupingCaseBundlePrivate{}, err
	}
	return GroupingCaseBundlePrivate{raw, batch}, nil
}

// Import's expectation is NOT obtained from candidate bytes or an embedded
// self-resealed manifest. Checking the expected length/hash precedes JSON work.
// This detects substitutions; it does not authenticate who supplied expectation.
func ImportGroupingCaseBundlePrivate(expected rg.CompositionCommitment, raw []byte) (GroupingCaseBundlePrivate, error) {
	if len(raw) == 0 || len(raw) > GroupingCaseBundleMaxBytesPrivate || expected.ByteLength != len(raw) || expected.ID == "" || expected.ID != expected.Digest || groupingHashPrivate(raw) != expected.Digest {
		return GroupingCaseBundlePrivate{}, errors.New("expected offline bundle commitment mismatch")
	}
	if err := bundleJSONPreflightPrivate(raw); err != nil {
		return GroupingCaseBundlePrivate{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r GroupingCaseBundleRequestPrivate
	if err := dec.Decode(&r); err != nil {
		return GroupingCaseBundlePrivate{}, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return GroupingCaseBundlePrivate{}, errors.New("trailing offline bundle bytes")
	}
	// Exact canonical encoding also rejects duplicate keys, case-folded aliases,
	// whitespace, escaped keys, alternative numbers, and missing final LF.
	canonical, err := groupingCanonicalPrivate(r)
	if err != nil || !bytes.Equal(canonical, raw) {
		return GroupingCaseBundlePrivate{}, errors.New("noncanonical offline bundle")
	}
	return NewGroupingCaseBundlePrivate(r)
}

// Export/replay uses the adopted measurement result bytes without wrapping,
// rewriting, or trusting counters in a decoded report. Checking actually repeats
// the existing batch execution (including evaluator replay), not a resource proof.
func (b GroupingCaseBundlePrivate) ExecuteReport() (GroupingMeasurementResultPrivate, error) {
	return ExecuteGroupingMeasurementBatchPrivate(b.batch)
}
func (b GroupingCaseBundlePrivate) CheckReport(raw []byte) error {
	return CheckGroupingMeasurementBatchPrivate(b.batch, raw)
}

func bundleRequestBoundsPrivate(r GroupingCaseBundleRequestPrivate) error {
	bad := func() error { return errors.New("offline bundle declaration bound or binding") }
	if r.Schema != GroupingCaseBundleSchemaPrivate || len(r.Slots) == 0 || len(r.Slots) > GroupingCaseBundleMaxSlotsPrivate || len(r.ImplementationBytes) == 0 || len(r.ImplementationBytes) > 1<<16 {
		return bad()
	}
	// Conservative encoded-work budget, admitted before derived slices, maps,
	// marshaling, sorting or constructors. Each multiplication is guarded first.
	budget := 0
	add := func(n, factor int) bool {
		if n < 0 || n > GroupingCaseBundleMaxBytesPrivate/factor || budget > GroupingCaseBundleMaxBytesPrivate-n*factor {
			return false
		}
		budget += n * factor
		return true
	}
	text := func(s string) bool { return measurementStringPrivate(s) && add(len(s), 6) }
	blob := func(b []byte, max int) bool { return len(b) > 0 && len(b) <= max && add(len(b), 2) }
	if !text(r.Schema) || !text(r.BatchID) || !text(r.ImplementationID) || !blob(r.ImplementationBytes, 1<<16) {
		return bad()
	}
	for _, s := range r.Slots {
		if !add(4096, 1) || len(s.Predecessors) == 0 || len(s.Predecessors) > GroupingCaseBundleMaxPredecessorsPrivate || len(s.Pairs) > GroupingCaseBundleMaxJudgmentsPrivate {
			return bad()
		}
		for _, id := range []string{s.CaseID, s.SelectorID, s.RunID, s.JudgmentID} {
			if !text(id) {
				return bad()
			}
		}
		if !blob(s.InputBytes, 1<<20) || !blob(s.PolicyBytes, 1<<16) || !blob(s.SelectorBytes, 1<<16) || !blob(s.CaseBytes, 1<<20) || !blob(s.JudgmentBytes, 1<<20) {
			return bad()
		}
		for _, p := range s.Predecessors {
			if !text(p.Role) || !text(p.Selector) || !text(p.Schema) || !blob(p.Bytes, 1<<20) || len(p.ClaimedDigest) > 4096 || !utf8.ValidString(p.ClaimedDigest) || !add(len(p.ClaimedDigest), 6) {
				return bad()
			}
		}
		for _, p := range s.Pairs {
			if !text(p.Left) || !text(p.Right) || !text(p.Label) {
				return bad()
			}
		}
		l := s.ProjectionLimits
		if l.MaxRawBytes <= 0 || l.MaxRawBytes > 1<<20 || l.MaxOccurrences <= 0 || l.MaxOccurrences > 4096 || l.MaxPairContributions <= 0 || l.MaxPairContributions > 4096 || l.MaxPairs <= 0 || l.MaxPairs > 4096 || l.MaxCanonicalBytes <= 0 || l.MaxCanonicalBytes > 1<<20 || l.MaxStringBytes <= 0 || l.MaxStringBytes > 4096 || l.MaxDenominator <= 0 || l.MaxDenominator > 1<<30 || l.MaxAccumulator <= 0 || l.MaxAccumulator > 1<<50 {
			return bad()
		}
	}
	return nil
}
func reconstructBundlePrivate(r GroupingCaseBundleRequestPrivate) (GroupingMeasurementBatchPrivate, error) {
	slots := make([]GroupingMeasurementSlotPrivate, 0, len(r.Slots))
	for _, s := range r.Slots {
		input, err := rg.ParseAndAdmit(s.InputBytes, s.Predecessors, rg.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 16, MaxNodes: 8192, MaxOccurrences: 4096, MaxContributions: 4096, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096})
		if err != nil {
			return GroupingMeasurementBatchPrivate{}, err
		}
		policy, err := rg.ParseGroupingPolicy(s.PolicyBytes, rg.GroupingPolicyParseLimits{MaxRawBytes: 1 << 16, MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 4096})
		if err != nil {
			return GroupingMeasurementBatchPrivate{}, err
		}
		c, err := NewGroupingEvaluationCasePrivate(GroupingEvaluationCaseRequestPrivate{CaseID: s.CaseID, SelectorID: s.SelectorID, RunID: s.RunID, SelectorBytes: s.SelectorBytes, Input: input, Policy: policy, ProjectionLimits: s.ProjectionLimits, Seed: s.Seed})
		if err != nil {
			return GroupingMeasurementBatchPrivate{}, err
		}
		if err = CheckGroupingCasePrivate(c, s.CaseBytes); err != nil {
			return GroupingMeasurementBatchPrivate{}, err
		}
		j, err := NewGroupingJudgmentsPrivate(c, s.JudgmentID, s.Pairs)
		if err != nil {
			return GroupingMeasurementBatchPrivate{}, err
		}
		if !bytes.Equal(j.CanonicalBytes(), s.JudgmentBytes) {
			return GroupingMeasurementBatchPrivate{}, errors.New("frozen offline judgments mismatch")
		}
		slots = append(slots, GroupingMeasurementSlotPrivate{Case: c, Judgments: j})
	}
	return NewGroupingMeasurementBatchPrivate(GroupingMeasurementBatchRequestPrivate{BatchID: r.BatchID, ImplementationID: r.ImplementationID, ImplementationBytes: r.ImplementationBytes, Slots: slots})
}

// Allocation-free lexical preflight bounds the entire document before typed JSON
// decoding. No token strings, key maps, recursive heap trees or base64 buffers are
// allocated here. All arrays are capped at 128 (judgments included), depth at 16,
// predecessors at 16, tokens at 65536, encoded strings at 2 MiB, numeric
// lexemes at 20 bytes. Total decoded bytes cannot exceed the 8 MiB raw ceiling.
// Canonical/typed validation follows; this scanner intentionally admits a superset
// of JSON grammar but cannot admit work beyond those fixed limits.
func bundleJSONPreflightPrivate(raw []byte) error {
	if len(raw) == 0 || len(raw) > GroupingCaseBundleMaxBytesPrivate {
		return errors.New("offline raw byte bound")
	}
	var arrays [16]int
	var kinds [16]byte
	var caps [16]int
	var seen [16]uint64
	keys := [...]string{"Schema", "BatchID", "ImplementationID", "ImplementationBytes", "Slots", "InputBytes", "PolicyBytes", "Predecessors", "CaseID", "SelectorID", "RunID", "SelectorBytes", "ProjectionLimits", "Seed", "JudgmentID", "Pairs", "CaseBytes", "JudgmentBytes", "Role", "Selector", "Bytes", "ClaimedDigest", "MaxRawBytes", "MaxOccurrences", "MaxPairContributions", "MaxPairs", "MaxCanonicalBytes", "MaxStringBytes", "MaxDenominator", "MaxAccumulator", "Left", "Right", "Label"}
	nextCap := 128
	depth, tokens := 0, 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' || c == ':' {
			continue
		}
		tokens++
		if tokens > 65536 {
			return errors.New("offline token bound")
		}
		switch c {
		case '{', '[':
			if depth == 16 {
				return errors.New("offline depth bound")
			}
			kinds[depth] = c
			arrays[depth] = 0
			seen[depth] = 0
			caps[depth] = nextCap
			nextCap = 128
			depth++
		case '}', ']':
			if depth == 0 || (c == '}' && kinds[depth-1] != '{') || (c == ']' && kinds[depth-1] != '[') {
				return errors.New("offline nesting")
			}
			depth--
		case ',':
			if depth > 0 && kinds[depth-1] == '[' {
				arrays[depth-1]++
				if arrays[depth-1] >= caps[depth-1] {
					return errors.New("offline array bound")
				}
			}
		case '"':
			start := i
			i++
			for ; i < len(raw); i++ {
				if raw[i] == '\\' {
					i++
					continue
				}
				if raw[i] == '"' {
					break
				}
				if i-start > 2<<20 {
					return errors.New("offline encoded string bound")
				}
			}
			if i >= len(raw) || i-start > 2<<20 {
				return errors.New("offline string framing or bound")
			}
			look := i + 1
			for look < len(raw) && (raw[look] == ' ' || raw[look] == '\n' || raw[look] == '\r' || raw[look] == '\t') {
				look++
			}
			if look < len(raw) && raw[look] == ':' {
				if depth == 0 || kinds[depth-1] != '{' {
					return errors.New("offline key position")
				}
				key := -1
				for k, name := range keys {
					if string(raw[start+1:i]) == name {
						key = k
						break
					}
				}
				if key < 0 || seen[depth-1]&(uint64(1)<<key) != 0 {
					return errors.New("offline unknown/duplicate/noncanonical key")
				}
				seen[depth-1] |= uint64(1) << key
				nextCap = 128
				if keys[key] == "Predecessors" {
					nextCap = GroupingCaseBundleMaxPredecessorsPrivate
				}
			}
		default:
			start := i
			for i+1 < len(raw) && raw[i+1] != ',' && raw[i+1] != ']' && raw[i+1] != '}' && raw[i+1] != ' ' && raw[i+1] != '\n' && raw[i+1] != '\t' && raw[i+1] != '\r' {
				i++
			}
			if i-start >= 20 {
				return errors.New("offline numeric/literal bound")
			}
		}
	}
	if depth != 0 {
		return errors.New("offline truncated document")
	}
	return nil
}
