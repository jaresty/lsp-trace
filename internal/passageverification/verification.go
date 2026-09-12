// Package passageverification verifies exact passages using already-retained
// lsp-trace evidence. It performs no acquisition and grants no authority beyond
// artifact integrity, graph membership/attribution, and retained source bytes.
package passageverification

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/hydratedevidence"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
	"lsp-trace/internal/v5sourcesnapshot"
)

const (
	MaxBatchRecords  = 235
	MaxArtifactBytes = 192 << 20
	MaxBatchBytes    = 192 << 20
)

type Status string

const (
	Verified                   Status = "VERIFIED"
	NotEvaluated               Status = "NOT_EVALUATED"
	InvalidInput               Status = "INVALID_INPUT"
	ArtifactDigestMismatch     Status = "ARTIFACT_DIGEST_MISMATCH"
	ArtifactInvalid            Status = "ARTIFACT_INVALID"
	UnsupportedArtifactFamily  Status = "UNSUPPORTED_ARTIFACT_FAMILY"
	SelectorCustodyUnavailable Status = "SELECTOR_CUSTODY_NOT_ESTABLISHED"
	InspectionAbsent           Status = "INSPECTION_ABSENT"
	SeedAbsent                 Status = "SEED_ABSENT"
	SeedAmbiguous              Status = "SEED_AMBIGUOUS"
	NodeAbsent                 Status = "NODE_ABSENT"
	NodeAmbiguous              Status = "NODE_AMBIGUOUS"
	NodeNotInSeed              Status = "NODE_NOT_IN_SEED"
	PathMismatch               Status = "PATH_MISMATCH"
	RangeMismatch              Status = "RANGE_MISMATCH"
	InvalidCoordinates         Status = "INVALID_COORDINATES"
	SourceReceiptUnavailable   Status = "SOURCE_RECEIPT_UNAVAILABLE"
	SourceDigestMismatch       Status = "SOURCE_DIGEST_MISMATCH"
	SourceBytesUnavailable     Status = "SOURCE_BYTES_UNAVAILABLE"
	PassageDigestMismatch      Status = "PASSAGE_DIGEST_MISMATCH"
	LimitExceeded              Status = "LIMIT_EXCEEDED"
)

type RangeMode string

const (
	Exact      RangeMode = "EXACT"
	Intersects RangeMode = "INTERSECTS"
)

type Position = hydratedevidence.Position
type Range = hydratedevidence.Range

type ArtifactDescriptor struct {
	AdmissionStatus string `json:"admission_status"`
	ExactSHA256     string `json:"exact_sha256"`
	InspectionID    string `json:"inspection_id"`
}

type Artifact struct {
	Bytes []byte `json:"bytes"`
	// Descriptor may carry an already custody-admitted selector binding. It is
	// checked against the exact bytes and request; omission means direct bytes.
	Descriptor *ArtifactDescriptor `json:"descriptor,omitempty"`
}

type Request struct {
	Artifact               Artifact  `json:"artifact"`
	ExpectedArtifactSHA256 string    `json:"expected_artifact_sha256"`
	InspectionID           string    `json:"inspection_id"`
	SeedLabel              string    `json:"seed_label"`
	NodeID                 string    `json:"node_id"`
	ExpectedURI            string    `json:"expected_uri"`
	RangeMode              RangeMode `json:"range_match_mode"`
	PositionEncoding       string    `json:"position_encoding"`
	PositionConvention     string    `json:"position_convention"`
	ExpectedRange          Range     `json:"expected_range"`
	ExpectedPassageSHA256  string    `json:"expected_passage_sha256"`
}

type Checks struct {
	ArtifactAdmission Status `json:"artifact_admission"`
	ArtifactDigest    Status `json:"artifact_digest"`
	SelectorCustody   Status `json:"selector_custody"`
	Inspection        Status `json:"inspection"`
	Seed              Status `json:"seed"`
	Membership        Status `json:"membership"`
	NodeIdentity      Status `json:"node_identity"`
	Path              Status `json:"path"`
	Range             Status `json:"range"`
	SourceReceipt     Status `json:"source_receipt"`
	SourceDigest      Status `json:"source_digest"`
	PassageBytes      Status `json:"passage_bytes"`
	PassageDigest     Status `json:"passage_digest"`
	BodyCompleteness  Status `json:"body_completeness"`
}

type Result struct {
	Index               int    `json:"index"`
	Overall             Status `json:"overall"`
	ReacquisitionNeeded bool   `json:"reacquisition_needed"`
	Checks              Checks `json:"checks"`
	// Diagnostic is an enum-like, privacy-safe reason; it never includes paths,
	// source text, artifact fragments, or validator error strings.
	Diagnostic string `json:"diagnostic,omitempty"`
}

type BatchRequest struct {
	Records []Request `json:"records"`
}
type BatchResult struct {
	Results []Result `json:"results"`
}

type native struct {
	SchemaVersion     string                 `json:"schema_version"`
	ExecutionBundleID string                 `json:"execution_bundle_id"`
	Nodes             []graph.Node           `json:"nodes"`
	Seeds             []graph.SeedResult     `json:"seeds"`
	SeedMemberships   []graph.SeedMembership `json:"seed_memberships"`
}

func initial() Checks {
	return Checks{ArtifactAdmission: NotEvaluated, ArtifactDigest: NotEvaluated, SelectorCustody: NotEvaluated, Inspection: NotEvaluated, Seed: NotEvaluated, Membership: NotEvaluated, NodeIdentity: NotEvaluated, Path: NotEvaluated, Range: NotEvaluated, SourceReceipt: NotEvaluated, SourceDigest: NotEvaluated, PassageBytes: NotEvaluated, PassageDigest: NotEvaluated, BodyCompleteness: NotEvaluated}
}
func digest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func fail(r *Result, s Status, diagnostic string) Result {
	r.Overall = s
	r.Diagnostic = diagnostic
	return *r
}

func ValidateRequest(q Request) error {
	if len(q.Artifact.Bytes) == 0 || len(q.Artifact.Bytes) > MaxArtifactBytes || q.ExpectedArtifactSHA256 == "" || q.InspectionID == "" || q.SeedLabel == "" || q.NodeID == "" || q.ExpectedURI == "" || q.ExpectedPassageSHA256 == "" {
		return errors.New("required field/size")
	}
	if q.RangeMode != Exact && q.RangeMode != Intersects {
		return errors.New("range mode")
	}
	if q.PositionEncoding != "utf-8" && q.PositionEncoding != "utf-16" && q.PositionEncoding != "utf-32" {
		return errors.New("encoding")
	}
	if q.PositionConvention != "LSP_ZERO_BASED_END_EXCLUSIVE" {
		return errors.New("position convention")
	}
	if q.ExpectedRange.Start.Line < 0 || q.ExpectedRange.Start.Character < 0 || q.ExpectedRange.End.Line < 0 || q.ExpectedRange.End.Character < 0 {
		return errors.New("coordinates")
	}
	if q.ExpectedRange.End.Line < q.ExpectedRange.Start.Line || (q.ExpectedRange.End.Line == q.ExpectedRange.Start.Line && q.ExpectedRange.End.Character < q.ExpectedRange.Start.Character) {
		return errors.New("reversed coordinates")
	}
	return nil
}

func Verify(q Request) Result {
	r := Result{Checks: initial()}
	if ValidateRequest(q) != nil {
		return fail(&r, InvalidInput, "REQUEST_INVALID")
	}
	if digest(q.Artifact.Bytes) != q.ExpectedArtifactSHA256 {
		r.Checks.ArtifactDigest = ArtifactDigestMismatch
		return fail(&r, ArtifactDigestMismatch, "ARTIFACT_DIGEST")
	}
	r.Checks.ArtifactDigest = Verified
	if d := q.Artifact.Descriptor; d != nil && d.AdmissionStatus == "CUSTODY_ADMITTED" && d.ExactSHA256 == q.ExpectedArtifactSHA256 && d.InspectionID == q.InspectionID {
		r.Checks.SelectorCustody = Verified
	} else {
		r.Checks.SelectorCustody = SelectorCustodyUnavailable
	}

	graphBytes, provenance, admission := admit(q.Artifact.Bytes)
	if admission != Verified {
		r.Checks.ArtifactAdmission = admission
		// Preserve a precise selector diagnostic when a semantically rejected
		// carrier nevertheless has an unambiguous, bounded top-level shape.
		if rawGraph, ok := uncheckedGraph(q.Artifact.Bytes); ok {
			var candidate native
			if json.Unmarshal(rawGraph, &candidate) == nil {
				count := 0
				for _, seed := range candidate.Seeds {
					if seed.Label == q.SeedLabel {
						count++
					}
				}
				if count > 1 {
					r.Checks.Seed = SeedAmbiguous
				}
			}
		}
		return fail(&r, admission, "ARTIFACT_ADMISSION")
	}
	r.Checks.ArtifactAdmission = Verified
	var g native
	if json.Unmarshal(graphBytes, &g) != nil {
		return fail(&r, ArtifactInvalid, "GRAPH_DECODE")
	}
	if g.ExecutionBundleID != q.InspectionID {
		r.Checks.Inspection = InspectionAbsent
		return fail(&r, InspectionAbsent, "INSPECTION_ID")
	}
	r.Checks.Inspection = Verified
	seedCount := 0
	var seed graph.SeedResult
	for _, s := range g.Seeds {
		if s.Label == q.SeedLabel {
			seedCount++
			seed = s
		}
	}
	if seedCount == 0 {
		r.Checks.Seed = SeedAbsent
		return fail(&r, SeedAbsent, "SEED_LABEL")
	}
	if seedCount > 1 {
		r.Checks.Seed = SeedAmbiguous
		return fail(&r, SeedAmbiguous, "SEED_LABEL")
	}
	r.Checks.Seed = Verified
	nodeCount := 0
	var node graph.Node
	for _, n := range g.Nodes {
		if n.ID == q.NodeID {
			nodeCount++
			node = n
		}
	}
	if nodeCount == 0 {
		r.Checks.NodeIdentity = NodeAbsent
		return fail(&r, NodeAbsent, "NODE_ID")
	}
	if nodeCount > 1 {
		r.Checks.NodeIdentity = NodeAmbiguous
		return fail(&r, NodeAmbiguous, "NODE_ID")
	}
	r.Checks.NodeIdentity = Verified
	member := false
	for _, id := range seed.ReachedNodeIDs {
		if id == q.NodeID {
			member = true
		}
	}
	for _, m := range g.SeedMemberships {
		if m.SeedLabel == q.SeedLabel && m.EvidenceKind == "REACHED_NODE" && m.EndpointID == q.NodeID {
			member = true
		}
	}
	if !member {
		r.Checks.Membership = NodeNotInSeed
		return fail(&r, NodeNotInSeed, "SEED_MEMBERSHIP")
	}
	r.Checks.Membership = Verified
	if node.URI != q.ExpectedURI {
		r.Checks.Path = PathMismatch
		return fail(&r, PathMismatch, "URI")
	}
	r.Checks.Path = Verified
	nr := Range{Start: Position{Line: int(node.Range.Start.Line), Character: int(node.Range.Start.Character)}, End: Position{Line: int(node.Range.End.Line), Character: int(node.Range.End.Character)}}
	if !rangeMatches(nr, q.ExpectedRange, q.RangeMode) {
		r.Checks.Range = RangeMismatch
		return fail(&r, RangeMismatch, "RANGE")
	}
	r.Checks.Range = Verified
	if !provenance {
		r.Checks.SourceReceipt = SourceReceiptUnavailable
		r.Checks.SourceDigest = SourceBytesUnavailable
		r.Checks.PassageBytes = SourceBytesUnavailable
		r.Checks.PassageDigest = SourceBytesUnavailable
		r.ReacquisitionNeeded = true
		return fail(&r, SourceBytesUnavailable, "NO_RETAINED_SOURCE")
	}
	return verifySource(q, r)
}

func uncheckedGraph(raw []byte) ([]byte, bool) {
	var h struct {
		SchemaVersion string `json:"schema_version"`
		GraphV5       string `json:"graph_v5"`
		GraphV5Bytes  []byte `json:"graph_v5_bytes"`
	}
	if json.Unmarshal(raw, &h) != nil {
		return nil, false
	}
	if h.SchemaVersion == graph.SchemaVersionV3 {
		return raw, true
	}
	if h.SchemaVersion == graphprovenance.VersionV5 {
		b, err := base64.StdEncoding.DecodeString(h.GraphV5)
		return b, err == nil
	}
	if h.SchemaVersion == v5sourcesnapshot.Version {
		return uncheckedGraph(h.GraphV5Bytes)
	}
	return nil, false
}

// ValidateArtifact applies the core's exact artifact-family admission without
// performing passage selection or granting custody or authority.
func ValidateArtifact(raw []byte) error {
	_, _, status := admit(raw)
	if status != Verified {
		return fmt.Errorf("artifact admission: %s", status)
	}
	return nil
}

func admit(raw []byte) ([]byte, bool, Status) {
	var h struct {
		SchemaVersion string `json:"schema_version"`
		GraphV5       string `json:"graph_v5"`
		GraphV5Bytes  []byte `json:"graph_v5_bytes"`
	}
	if json.Unmarshal(raw, &h) != nil {
		return nil, false, ArtifactInvalid
	}
	switch h.SchemaVersion {
	case graph.SchemaVersionV3:
		if _, err := schema.ValidateFor(raw, schema.FamilyGraph, "v3"); err != nil {
			return nil, false, ArtifactInvalid
		}
		if _, err := graph.DecodeNativeV3(raw); err != nil {
			return nil, false, ArtifactInvalid
		}
		return raw, false, Verified
	case graphprovenance.VersionV5:
		if _, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v5"); err != nil {
			return nil, true, ArtifactInvalid
		}
		b, err := base64.StdEncoding.DecodeString(h.GraphV5)
		if err != nil {
			return nil, true, ArtifactInvalid
		}
		return b, true, Verified
	case v5sourcesnapshot.Version:
		if _, err := v5sourcesnapshot.Validate(raw); err != nil {
			return nil, true, ArtifactInvalid
		}
		b, ok := uncheckedGraph(h.GraphV5Bytes)
		if !ok {
			return nil, true, ArtifactInvalid
		}
		return b, true, Verified
	default:
		return nil, false, UnsupportedArtifactFamily
	}
}

func rangeMatches(a, b Range, mode RangeMode) bool {
	cmp := func(x, y Position) int {
		if x.Line < y.Line {
			return -1
		}
		if x.Line > y.Line {
			return 1
		}
		if x.Character < y.Character {
			return -1
		}
		if x.Character > y.Character {
			return 1
		}
		return 0
	}
	if mode == Exact {
		return cmp(a.Start, b.Start) == 0 && cmp(a.End, b.End) == 0
	}
	return cmp(a.Start, b.End) < 0 && cmp(b.Start, a.End) < 0
}

func verifySource(q Request, r Result) Result {
	p := hydratedevidence.DefaultPolicy()
	p.IncludeBodies = true
	cat, err := hydratedevidence.Inspect(hydratedevidence.Input{Artifact: q.Artifact.Bytes}, p)
	if err != nil {
		return fail(&r, ArtifactInvalid, "SOURCE_ADMISSION")
	}
	type pair struct {
		rec    hydratedevidence.Record
		source hydratedevidence.Source
	}
	pairs := []pair{}
	for _, rec := range cat.Records {
		if rec.NativeID != q.NodeID || rec.Range == nil {
			continue
		}
		for _, sid := range rec.SourceIDs {
			for _, src := range cat.Sources {
				if src.ID == sid && src.URI == q.ExpectedURI {
					pairs = append(pairs, pair{rec, src})
				}
			}
		}
	}
	if len(pairs) == 0 {
		r.Checks.SourceReceipt = SourceReceiptUnavailable
		r.Checks.SourceDigest = SourceBytesUnavailable
		r.Checks.PassageBytes = SourceBytesUnavailable
		r.Checks.PassageDigest = SourceBytesUnavailable
		r.ReacquisitionNeeded = true
		return fail(&r, SourceBytesUnavailable, "NODE_SOURCE_BINDING")
	}
	// Several independently retained receipts may support the same native node.
	// Prefer retained bytes deterministically; multiplicity is not node ambiguity.
	sort.Slice(pairs, func(i, j int) bool {
		ir, jr := pairs[i].source.State == "RETAINED_BYTES", pairs[j].source.State == "RETAINED_BYTES"
		if ir != jr {
			return ir
		}
		if pairs[i].source.ID != pairs[j].source.ID {
			return pairs[i].source.ID < pairs[j].source.ID
		}
		return pairs[i].rec.ID < pairs[j].rec.ID
	})
	x := pairs[0]
	r.Checks.SourceReceipt = Verified
	if x.source.State != "RETAINED_BYTES" {
		r.Checks.SourceDigest = SourceBytesUnavailable
		r.Checks.PassageBytes = SourceBytesUnavailable
		r.Checks.PassageDigest = SourceBytesUnavailable
		r.ReacquisitionNeeded = true
		return fail(&r, SourceBytesUnavailable, "SOURCE_NOT_RETAINED")
	}
	r.Checks.SourceDigest = Verified
	req := hydratedevidence.Request{Policy: p, Selections: []hydratedevidence.Selection{{ID: "passage", RecordID: x.rec.ID, SourceID: x.source.ID, Mode: "SPAN", Encoding: q.PositionEncoding, Range: &q.ExpectedRange}}}
	b, err := hydratedevidence.Hydrate(hydratedevidence.Input{Artifact: q.Artifact.Bytes}, req)
	if err != nil {
		return fail(&r, ArtifactInvalid, "HYDRATION_ADMISSION")
	}
	if len(b.Origins) != 1 || b.Origins[0].Status == "INVALID_COORDINATES" {
		r.Checks.PassageBytes = InvalidCoordinates
		return fail(&r, InvalidCoordinates, "PASSAGE_COORDINATES")
	}
	if len(b.Spans) != 1 {
		r.Checks.PassageBytes = SourceBytesUnavailable
		r.ReacquisitionNeeded = true
		return fail(&r, SourceBytesUnavailable, "PASSAGE_NOT_RETAINED")
	}
	r.Checks.PassageBytes = Verified
	if b.Spans[0].ContentHash != q.ExpectedPassageSHA256 {
		r.Checks.PassageDigest = PassageDigestMismatch
		return fail(&r, PassageDigestMismatch, "PASSAGE_DIGEST")
	}
	r.Checks.PassageDigest = Verified
	r.Overall = Verified
	return r
}

func VerifyBatch(b BatchRequest) BatchResult {
	out := BatchResult{Results: make([]Result, len(b.Records))}
	if len(b.Records) > MaxBatchRecords {
		for i := range out.Results {
			out.Results[i] = Result{Index: i, Overall: LimitExceeded, Checks: initial(), Diagnostic: "BATCH_RECORD_LIMIT"}
		}
		return out
	}
	total := 0
	for _, q := range b.Records {
		if len(q.Artifact.Bytes) > MaxBatchBytes-total {
			for i := range out.Results {
				out.Results[i] = Result{Index: i, Overall: LimitExceeded, Checks: initial(), Diagnostic: "BATCH_BYTE_LIMIT"}
			}
			return out
		}
		total += len(q.Artifact.Bytes)
	}
	for i, q := range b.Records {
		out.Results[i] = Verify(q)
		out.Results[i].Index = i
	}
	return out
}

// MarshalStrictBatch decodes a transport-neutral request with unknown fields rejected.
func MarshalStrictBatch(raw []byte) (BatchResult, error) {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return BatchResult{}, err
	}
	var b BatchRequest
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&b); err != nil {
		return BatchResult{}, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return BatchResult{}, errors.New("trailing JSON")
	}
	return VerifyBatch(b), nil
}

// StableStatuses returns the finite status vocabulary in lexical order.
func StableStatuses() []Status {
	s := []Status{Verified, NotEvaluated, InvalidInput, ArtifactDigestMismatch, ArtifactInvalid, UnsupportedArtifactFamily, SelectorCustodyUnavailable, InspectionAbsent, SeedAbsent, SeedAmbiguous, NodeAbsent, NodeAmbiguous, NodeNotInSeed, PathMismatch, RangeMismatch, InvalidCoordinates, SourceReceiptUnavailable, SourceDigestMismatch, SourceBytesUnavailable, PassageDigestMismatch, LimitExceeded}
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s
}
