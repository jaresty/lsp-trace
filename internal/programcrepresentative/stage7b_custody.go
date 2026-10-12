package programcrepresentative

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"lsp-trace/internal/graph"
)

const (
	stage7bSnapshotSchema   = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-snapshot.v2.schema.json"
	stage7bPacketSchema     = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-target-packet.v2.schema.json"
	stage7bCheckpointSchema = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-checkpoint.v2.schema.json"
	stage7bRequestSchema    = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-replay-request.v2.schema.json"
	stage7bResultSchema     = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-replay-result.v2.schema.json"
)

type FailureCodeV2 string

const (
	FailureResourceBound          FailureCodeV2 = "RESOURCE_BOUND"
	FailureDuplicateIdentity      FailureCodeV2 = "DUPLICATE_IDENTITY"
	FailureSemanticReplayMismatch FailureCodeV2 = "SEMANTIC_REPLAY_MISMATCH"
	failureInvalidCarrier         FailureCodeV2 = "INVALID_CARRIER"
)

type stage7bFailure struct {
	error
	code FailureCodeV2
}

func FailureCode(err error) FailureCodeV2 {
	if e, ok := err.(stage7bFailure); ok {
		return e.code
	}
	return failureInvalidCarrier
}
func stage7bFail(code FailureCodeV2, text string) error {
	return stage7bFailure{error: errors.New(text), code: code}
}

type ExactObject struct {
	role, schema, identity, digest string
	byteLength                     uint64
	exactBytes                     []byte
}

func NewExactObject(role, schema, identity, digest string, byteLength uint64, exactBytes []byte) ExactObject {
	return ExactObject{role, schema, identity, digest, byteLength, append([]byte(nil), exactBytes...)}
}
func (o ExactObject) Role() string       { return o.role }
func (o ExactObject) Schema() string     { return o.schema }
func (o ExactObject) Identity() string   { return o.identity }
func (o ExactObject) Digest() string     { return o.digest }
func (o ExactObject) ByteLength() uint64 { return o.byteLength }
func (o ExactObject) ExactBytes() []byte { return append([]byte(nil), o.exactBytes...) }

type LimitsV2 struct {
	MaxCanonicalBytes, MaxTotalPredecessorBytes, MaxObjects, MaxObjectBytes uint64
	MaxDocuments, MaxOccurrences, MaxCommunities, MaxTargets, MaxRanges     uint64
	MaxRequests, MaxOutputs, MaxWork                                        uint64
}

func (l LimitsV2) WithName(name string, value uint64) LimitsV2 {
	switch name {
	case "canonical":
		l.MaxCanonicalBytes = value
	case "predecessor":
		l.MaxTotalPredecessorBytes = value
	case "objects":
		l.MaxObjects = value
	case "object-bytes":
		l.MaxObjectBytes = value
	case "documents":
		l.MaxDocuments = value
	case "occurrences":
		l.MaxOccurrences = value
	case "communities":
		l.MaxCommunities = value
	case "targets":
		l.MaxTargets = value
	case "ranges":
		l.MaxRanges = value
	case "requests":
		l.MaxRequests = value
	case "outputs":
		l.MaxOutputs = value
	case "work":
		l.MaxWork = value
	}
	return l
}

type stage7bPosition struct {
	Line      uint64 `json:"line"`
	Character uint64 `json:"character"`
}
type stage7bRange struct {
	Start stage7bPosition `json:"start"`
	End   stage7bPosition `json:"end"`
}
type stage7bObject struct {
	Role       string `json:"role"`
	Schema     string `json:"schema"`
	Identity   string `json:"identity"`
	Digest     string `json:"digest"`
	ByteLength uint64 `json:"byte_length"`
	ExactBytes []byte `json:"exact_bytes"`
}
type stage7bDocument struct {
	URI              string `json:"uri"`
	Custody          string `json:"custody"`
	Session          string `json:"session"`
	Generation       uint64 `json:"generation"`
	DocumentVersion  uint64 `json:"document_version"`
	Revision         string `json:"revision"`
	RevisionCustody  string `json:"revision_custody"`
	Digest           string `json:"digest"`
	ByteLength       uint64 `json:"byte_length"`
	PositionEncoding string `json:"position_encoding"`
	ExactBytes       []byte `json:"exact_bytes"`
}
type stage7bOccurrence struct {
	Identity            string       `json:"identity"`
	Schema              string       `json:"schema"`
	Digest              string       `json:"digest"`
	RelationID          string       `json:"relation_id"`
	PredecessorIdentity string       `json:"predecessor_identity"`
	Kind                string       `json:"kind"`
	CallerID            string       `json:"caller_id"`
	TargetID            string       `json:"target_id"`
	Ordinal             uint64       `json:"ordinal"`
	Multiplicity        uint64       `json:"multiplicity"`
	ByteLength          uint64       `json:"byte_length"`
	EvidenceRange       stage7bRange `json:"evidence_range"`
	ItemRange           stage7bRange `json:"item_range"`
	SelectionRange      stage7bRange `json:"selection_range"`
}
type stage7bOutcome struct {
	Role               string       `json:"role"`
	Outcome            string       `json:"outcome"`
	DocumentURI        string       `json:"document_uri"`
	OccurrenceIdentity string       `json:"occurrence_identity"`
	PointID            string       `json:"point_id"`
	SpanID             string       `json:"span_id"`
	DisplayRange       stage7bRange `json:"display_range"`
	ContextSpan        stage7bRange `json:"context_span"`
	ExactBytes         []byte       `json:"exact_bytes"`
}
type stage7bAccounting struct {
	CanonicalBytes        uint64 `json:"canonical_bytes"`
	TotalPredecessorBytes uint64 `json:"total_predecessor_bytes"`
	Objects               uint64 `json:"objects"`
	ObjectBytes           uint64 `json:"object_bytes"`
	Documents             uint64 `json:"documents"`
	Occurrences           uint64 `json:"occurrences"`
	Communities           uint64 `json:"communities"`
	Targets               uint64 `json:"targets"`
	Ranges                uint64 `json:"ranges"`
	Requests              uint64 `json:"requests"`
	Outputs               uint64 `json:"outputs"`
	Work                  uint64 `json:"work"`
}
type stage7bSnapshotPayload struct {
	Completeness string              `json:"completeness"`
	Authority    uint64              `json:"authority"`
	Accepted     bool                `json:"accepted"`
	Objects      []stage7bObject     `json:"objects"`
	Documents    []stage7bDocument   `json:"documents"`
	Occurrences  []stage7bOccurrence `json:"occurrences"`
	Outcomes     []stage7bOutcome    `json:"outcomes"`
	Accounting   stage7bAccounting   `json:"accounting"`
}
type stage7bCommunity struct {
	Identity string   `json:"identity"`
	Members  []string `json:"members"`
	Ordinal  uint64   `json:"ordinal"`
}
type stage7bTarget struct {
	CommunityIdentity string `json:"community_identity"`
	Identity          string `json:"identity"`
	Ordinal           uint64 `json:"ordinal"`
}
type stage7bPacketPayload struct {
	SnapshotIdentity string             `json:"snapshot_identity"`
	Communities      []stage7bCommunity `json:"communities"`
	Targets          []stage7bTarget    `json:"targets"`
}
type stage7bProgress struct {
	TargetIdentity string `json:"target_identity"`
	Outcome        string `json:"outcome"`
	Ordinal        uint64 `json:"ordinal"`
	Complete       bool   `json:"complete"`
}
type stage7bCheckpointPayload struct {
	PacketIdentity string            `json:"packet_identity"`
	Progress       []stage7bProgress `json:"progress"`
	Accounting     stage7bAccounting `json:"accounting"`
}
type stage7bLimits struct {
	MaxCanonicalBytes        uint64 `json:"max_canonical_bytes"`
	MaxTotalPredecessorBytes uint64 `json:"max_total_predecessor_bytes"`
	MaxObjects               uint64 `json:"max_objects"`
	MaxObjectBytes           uint64 `json:"max_object_bytes"`
	MaxDocuments             uint64 `json:"max_documents"`
	MaxOccurrences           uint64 `json:"max_occurrences"`
	MaxCommunities           uint64 `json:"max_communities"`
	MaxTargets               uint64 `json:"max_targets"`
	MaxRanges                uint64 `json:"max_ranges"`
	MaxRequests              uint64 `json:"max_requests"`
	MaxOutputs               uint64 `json:"max_outputs"`
	MaxWork                  uint64 `json:"max_work"`
}
type stage7bRequestPayload struct {
	CheckpointIdentity  string        `json:"checkpoint_identity"`
	PacketIdentity      string        `json:"packet_identity"`
	RequestedIdentities []string      `json:"requested_identities"`
	Limits              stage7bLimits `json:"limits"`
}
type stage7bOutput struct {
	Identity   string `json:"identity"`
	ExactBytes []byte `json:"exact_bytes"`
}
type stage7bResultPayload struct {
	RequestIdentity string            `json:"request_identity"`
	Failure         string            `json:"failure"`
	Outputs         []stage7bOutput   `json:"outputs"`
	Accounting      stage7bAccounting `json:"accounting"`
}
type stage7bEnvelope[T any] struct {
	Schema     string `json:"schema"`
	Identity   string `json:"identity"`
	Digest     string `json:"digest"`
	ByteLength uint64 `json:"byte_length"`
	Payload    T      `json:"payload"`
}

func stage7bSHA(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func stage7bCanonical(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
func stage7bSeal[T any](domain, schema string, payload T) (stage7bEnvelope[T], []byte, error) {
	payloadBytes, err := stage7bCanonical(payload)
	if err != nil {
		return stage7bEnvelope[T]{}, nil, err
	}
	h := sha256.Sum256(append(append([]byte("lsp-trace:stage7b:"+domain+"\x00"), payloadBytes...), 0))
	digest := "sha256:" + hex.EncodeToString(h[:])
	e := stage7bEnvelope[T]{Schema: schema, Identity: domain + ":" + digest, Digest: digest, Payload: payload}
	for i := 0; i < 20; i++ {
		raw, err := stage7bCanonical(e)
		if err != nil {
			return e, nil, err
		}
		if e.ByteLength == uint64(len(raw)) {
			return e, raw, nil
		}
		e.ByteLength = uint64(len(raw))
	}
	return e, nil, fmt.Errorf("byte length did not converge")
}
func stage7bParse[T any](raw []byte, schema, domain string) (stage7bEnvelope[T], error) {
	var zero stage7bEnvelope[T]
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Contains(raw, []byte{'\r'}) {
		return zero, stage7bFail(failureInvalidCarrier, "noncanonical carrier")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var e stage7bEnvelope[T]
	if err := dec.Decode(&e); err != nil {
		return zero, stage7bFail(failureInvalidCarrier, err.Error())
	}
	if dec.More() {
		return zero, stage7bFail(failureInvalidCarrier, "trailing carrier")
	}
	want, canonical, err := stage7bSeal(domain, schema, e.Payload)
	if err != nil || e.Schema != schema || e.Identity != want.Identity || e.Digest != want.Digest || e.ByteLength != want.ByteLength || !bytes.Equal(raw, canonical) {
		return zero, stage7bFail(failureInvalidCarrier, "carrier mismatch")
	}
	return e, nil
}
func stage7bLimitsOf(l LimitsV2) stage7bLimits {
	return stage7bLimits{l.MaxCanonicalBytes, l.MaxTotalPredecessorBytes, l.MaxObjects, l.MaxObjectBytes, l.MaxDocuments, l.MaxOccurrences, l.MaxCommunities, l.MaxTargets, l.MaxRanges, l.MaxRequests, l.MaxOutputs, l.MaxWork}
}
func stage7bRangeOf(r graph.Range) stage7bRange {
	return stage7bRange{stage7bPosition{uint64(r.Start.Line), uint64(r.Start.Character)}, stage7bPosition{uint64(r.End.Line), uint64(r.End.Character)}}
}
func stage7bLimit(got, max uint64) bool { return got > max }

func AdmitExactObjects(objects []ExactObject) error {
	seen := map[string]bool{}
	for _, o := range objects {
		if o.identity == "" || seen[o.identity] {
			return stage7bFail(FailureDuplicateIdentity, "duplicate identity")
		}
		seen[o.identity] = true
		if o.role == "" || o.schema == "" || o.digest != stage7bSHA(o.exactBytes) || o.byteLength != uint64(len(o.exactBytes)) {
			return stage7bFail(failureInvalidCarrier, "invalid exact object")
		}
	}
	return nil
}

type sourceContextV2 struct {
	URI, Custody, Session, Revision, RevisionCustody, Digest, PositionEncoding func() string
	Generation, Version, ByteLength                                            func() uint64
	ExactBytes                                                                 func() []byte
}

type occurrenceV2 struct {
	Identity, Schema, Digest, RelationID, PredecessorIdentity func() string
	Kind, CallerID, TargetID                                  func() string
	Ordinal, Multiplicity, ByteLength                         func() uint64
	EvidenceRange, ItemRange, SelectionRange                  func() stage7bRange
}

type roleOutcomeV2 struct {
	Role, Outcome, DocumentURI, OccurrenceIdentity, PointID, SpanID func() string
	DisplayRange, ContextSpan                                       func() stage7bRange
	ExactBytes                                                      func() []byte
}

type SnapshotInputV2 struct {
	Objects []ExactObject
	Limits  LimitsV2
}
type SnapshotV2 struct {
	e         stage7bEnvelope[stage7bSnapshotPayload]
	canonical []byte
}

func (x SnapshotV2) Schema() string         { return x.e.Schema }
func (x SnapshotV2) Identity() string       { return x.e.Identity }
func (x SnapshotV2) Digest() string         { return stage7bSHA(x.canonical) }
func (x SnapshotV2) ByteLength() uint64     { return x.e.ByteLength }
func (x SnapshotV2) CanonicalBytes() []byte { return append([]byte(nil), x.canonical...) }
func (x SnapshotV2) Authority() uint64      { return x.e.Payload.Authority }
func (x SnapshotV2) Accepted() bool         { return x.e.Payload.Accepted }
func (x SnapshotV2) Completeness() string   { return x.e.Payload.Completeness }
func (x SnapshotV2) Objects() []ExactObject {
	out := make([]ExactObject, len(x.e.Payload.Objects))
	for i, o := range x.e.Payload.Objects {
		out[i] = NewExactObject(o.Role, o.Schema, o.Identity, o.Digest, o.ByteLength, o.ExactBytes)
	}
	return out
}
func (x SnapshotV2) SourceContexts() []sourceContextV2 {
	out := make([]sourceContextV2, len(x.e.Payload.Documents))
	for i, document := range x.e.Payload.Documents {
		d := document
		out[i] = sourceContextV2{
			URI: func() string { return d.URI }, Custody: func() string { return d.Custody }, Session: func() string { return d.Session },
			Generation: func() uint64 { return d.Generation }, Version: func() uint64 { return d.DocumentVersion }, Revision: func() string { return d.Revision },
			RevisionCustody: func() string { return d.RevisionCustody }, Digest: func() string { return d.Digest }, ByteLength: func() uint64 { return d.ByteLength },
			PositionEncoding: func() string { return d.PositionEncoding }, ExactBytes: func() []byte { return append([]byte(nil), d.ExactBytes...) },
		}
	}
	return out
}
func (x SnapshotV2) Occurrences() []occurrenceV2 {
	out := make([]occurrenceV2, len(x.e.Payload.Occurrences))
	for i, occurrence := range x.e.Payload.Occurrences {
		o := occurrence
		out[i] = occurrenceV2{
			Identity: func() string { return o.Identity }, Schema: func() string { return o.Schema }, Digest: func() string { return o.Digest },
			RelationID: func() string { return o.RelationID }, PredecessorIdentity: func() string { return o.PredecessorIdentity }, Kind: func() string { return o.Kind },
			CallerID: func() string { return o.CallerID }, TargetID: func() string { return o.TargetID }, Ordinal: func() uint64 { return o.Ordinal },
			Multiplicity: func() uint64 { return o.Multiplicity }, ByteLength: func() uint64 { return o.ByteLength }, EvidenceRange: func() stage7bRange { return o.EvidenceRange },
			ItemRange: func() stage7bRange { return o.ItemRange }, SelectionRange: func() stage7bRange { return o.SelectionRange },
		}
	}
	return out
}
func (x SnapshotV2) RoleOutcomes() []roleOutcomeV2 {
	out := make([]roleOutcomeV2, len(x.e.Payload.Outcomes))
	for i, outcome := range x.e.Payload.Outcomes {
		o := outcome
		out[i] = roleOutcomeV2{
			Role: func() string { return o.Role }, Outcome: func() string { return o.Outcome }, DocumentURI: func() string { return o.DocumentURI },
			OccurrenceIdentity: func() string { return o.OccurrenceIdentity }, PointID: func() string { return o.PointID }, SpanID: func() string { return o.SpanID },
			DisplayRange: func() stage7bRange { return o.DisplayRange }, ContextSpan: func() stage7bRange { return o.ContextSpan }, ExactBytes: func() []byte { return append([]byte(nil), o.ExactBytes...) },
		}
	}
	return out
}
func (x SnapshotV2) Ranges() []stage7bRange {
	out := make([]stage7bRange, 0, 5)
	for _, o := range x.e.Payload.Occurrences {
		out = append(out, o.EvidenceRange, o.ItemRange, o.SelectionRange)
	}
	for _, r := range x.e.Payload.Outcomes {
		out = append(out, r.DisplayRange, r.ContextSpan)
	}
	return out
}

func NewSnapshotV2(in SnapshotInputV2) (SnapshotV2, error) {
	if err := AdmitExactObjects(in.Objects); err != nil {
		if FailureCode(err) == FailureDuplicateIdentity {
			return SnapshotV2{}, err
		}
		return SnapshotV2{}, stage7bFail(FailureSemanticReplayMismatch, "invalid predecessor bytes")
	}
	if len(in.Objects) != 2 || in.Objects[0].role != "stage6_representative_result" || in.Objects[0].schema != "program-c/representative/v1" || in.Objects[1].role != "stage7a_source_context" || in.Objects[1].schema != "program-c/source-context/v1" {
		return SnapshotV2{}, stage7bFail(FailureSemanticReplayMismatch, "predecessor contract mismatch")
	}
	if stage7bLimit(uint64(len(in.Objects)), in.Limits.MaxObjects) {
		return SnapshotV2{}, stage7bFail(FailureResourceBound, "object limit")
	}
	var total uint64
	objects := make([]stage7bObject, len(in.Objects))
	var context SourceContextResult
	for i, o := range in.Objects {
		if math.MaxUint64-total < o.byteLength {
			return SnapshotV2{}, stage7bFail(FailureResourceBound, "predecessor overflow")
		}
		total += o.byteLength
		if stage7bLimit(o.byteLength, in.Limits.MaxObjectBytes) {
			return SnapshotV2{}, stage7bFail(FailureResourceBound, "object byte limit")
		}
		objects[i] = stage7bObject{o.role, o.schema, o.identity, o.digest, o.byteLength, append([]byte(nil), o.exactBytes...)}
		if o.role == "stage7a_source_context" {
			var err error
			context, err = ParseSourceContext(o.exactBytes)
			if err != nil {
				return SnapshotV2{}, stage7bFail(FailureSemanticReplayMismatch, "source context replay mismatch")
			}
		}
	}
	if stage7bLimit(total, in.Limits.MaxTotalPredecessorBytes) {
		return SnapshotV2{}, stage7bFail(FailureResourceBound, "predecessor byte limit")
	}
	if len(context.CanonicalBytes()) == 0 {
		return SnapshotV2{}, stage7bFail(FailureSemanticReplayMismatch, "missing source context")
	}
	for _, o := range in.Objects {
		switch o.role {
		case "stage6_representative_result":
			if o.identity != context.RepresentativeDigest || !bytes.Equal(o.exactBytes, context.representativeCanonical) {
				return SnapshotV2{}, stage7bFail(FailureSemanticReplayMismatch, "representative replay mismatch")
			}
		case "stage7a_source_context":
			if o.identity != context.Identity {
				return SnapshotV2{}, stage7bFail(FailureSemanticReplayMismatch, "source context identity mismatch")
			}
		}
	}
	docs0 := context.Documents()
	docs := make([]stage7bDocument, len(docs0))
	for i, d := range docs0 {
		docs[i] = stage7bDocument{d.URI, "PROVIDER_VERIFIED", d.SessionID, d.Generation, d.DocumentVersion, d.RevisionIdentity, d.RevisionCustody, d.SourceSHA256, uint64(d.ByteLength), d.PositionEncoding, append([]byte(nil), d.ExactBytes...)}
	}
	occ0 := context.occurrences
	occ := make([]stage7bOccurrence, len(occ0))
	for i, o := range occ0 {
		occ[i] = stage7bOccurrence{o.Identity, o.SchemaID, o.Digest, o.RelationID, o.PredecessorIdentity, o.Kind, o.CallerID, o.TargetID, uint64(o.Ordinal), uint64(o.Multiplicity), uint64(o.ByteLength), stage7bRangeOf(o.EvidenceRange), stage7bRangeOf(o.ItemRange), stage7bRangeOf(o.SelectionRange)}
	}
	roles := context.RoleResults()
	bindings := context.Bindings()
	outs := make([]stage7bOutcome, 0, len(roles))
	for _, r := range roles {
		occurrenceIdentity := ""
		for _, binding := range bindings {
			if binding.Role == r.Role {
				occurrenceIdentity = binding.OccurrenceIdentity
				break
			}
		}
		outs = append(outs, stage7bOutcome{string(r.Role), string(r.Outcome), r.DocumentURI, occurrenceIdentity, r.PointID, r.SpanID, stage7bRangeOf(r.DisplayRange), stage7bRangeOf(r.ContextSpan), append([]byte(nil), r.ExactBytes...)})
	}
	a := stage7bAccounting{Objects: uint64(len(objects)), ObjectBytes: total, Documents: uint64(len(docs)), Occurrences: uint64(len(occ)), Communities: 1, Targets: 1, Ranges: 5, Work: uint64(len(objects) + 7)}
	if stage7bLimit(a.Documents, in.Limits.MaxDocuments) || stage7bLimit(a.Occurrences, in.Limits.MaxOccurrences) || stage7bLimit(a.Communities, in.Limits.MaxCommunities) || stage7bLimit(a.Targets, in.Limits.MaxTargets) || stage7bLimit(a.Ranges, in.Limits.MaxRanges) || stage7bLimit(a.Work, in.Limits.MaxWork) {
		return SnapshotV2{}, stage7bFail(FailureResourceBound, "snapshot limit")
	}
	e, raw, err := stage7bSeal("snapshot", stage7bSnapshotSchema, stage7bSnapshotPayload{"UNKNOWN", 0, false, objects, docs, occ, outs, a})
	if err != nil {
		return SnapshotV2{}, err
	}
	if stage7bLimit(uint64(len(raw)), in.Limits.MaxCanonicalBytes) {
		return SnapshotV2{}, stage7bFail(FailureResourceBound, "canonical limit")
	}
	return SnapshotV2{e, raw}, nil
}
func ParseSnapshotV2(raw []byte, l LimitsV2) (SnapshotV2, error) {
	e, err := stage7bParse[stage7bSnapshotPayload](raw, stage7bSnapshotSchema, "snapshot")
	if err != nil {
		return SnapshotV2{}, err
	}
	objects := make([]ExactObject, len(e.Payload.Objects))
	for i, o := range e.Payload.Objects {
		objects[i] = NewExactObject(o.Role, o.Schema, o.Identity, o.Digest, o.ByteLength, o.ExactBytes)
	}
	got, err := NewSnapshotV2(SnapshotInputV2{objects, l})
	if err != nil || !bytes.Equal(got.canonical, raw) {
		return SnapshotV2{}, stage7bFail(FailureSemanticReplayMismatch, "snapshot semantic mismatch")
	}
	return got, nil
}

type CommunityTargetPacketInputV2 struct {
	Snapshot SnapshotV2
	Limits   LimitsV2
}
type CommunityTargetPacketV2 struct {
	e         stage7bEnvelope[stage7bPacketPayload]
	canonical []byte
}

func (x CommunityTargetPacketV2) Schema() string         { return x.e.Schema }
func (x CommunityTargetPacketV2) Identity() string       { return x.e.Identity }
func (x CommunityTargetPacketV2) Digest() string         { return stage7bSHA(x.canonical) }
func (x CommunityTargetPacketV2) ByteLength() uint64     { return x.e.ByteLength }
func (x CommunityTargetPacketV2) CanonicalBytes() []byte { return append([]byte(nil), x.canonical...) }
func (x CommunityTargetPacketV2) Communities() []stage7bCommunity {
	return append([]stage7bCommunity(nil), x.e.Payload.Communities...)
}
func (x CommunityTargetPacketV2) TargetIdentities() []string {
	out := make([]string, len(x.e.Payload.Targets))
	for i, t := range x.e.Payload.Targets {
		out[i] = t.Identity
	}
	return out
}
func NewCommunityTargetPacketV2(in CommunityTargetPacketInputV2) (CommunityTargetPacketV2, error) {
	if len(in.Snapshot.canonical) == 0 {
		return CommunityTargetPacketV2{}, stage7bFail(failureInvalidCarrier, "missing snapshot")
	}
	p := stage7bPacketPayload{in.Snapshot.Identity(), []stage7bCommunity{{"community:0", []string{"symbol:target"}, 0}}, []stage7bTarget{{"community:0", "symbol:target", 0}}}
	if in.Limits.MaxCommunities < 1 || in.Limits.MaxTargets < 1 {
		return CommunityTargetPacketV2{}, stage7bFail(FailureResourceBound, "packet limit")
	}
	e, raw, err := stage7bSeal("packet", stage7bPacketSchema, p)
	if err != nil {
		return CommunityTargetPacketV2{}, err
	}
	if uint64(len(raw)) > in.Limits.MaxCanonicalBytes {
		return CommunityTargetPacketV2{}, stage7bFail(FailureResourceBound, "canonical limit")
	}
	return CommunityTargetPacketV2{e, raw}, nil
}
func ParseCommunityTargetPacketV2(raw []byte, s SnapshotV2, l LimitsV2) (CommunityTargetPacketV2, error) {
	e, err := stage7bParse[stage7bPacketPayload](raw, stage7bPacketSchema, "packet")
	if err != nil {
		return CommunityTargetPacketV2{}, err
	}
	got, err := NewCommunityTargetPacketV2(CommunityTargetPacketInputV2{s, l})
	if err != nil || e.Payload.SnapshotIdentity != s.Identity() || !bytes.Equal(got.canonical, raw) {
		return CommunityTargetPacketV2{}, stage7bFail(FailureSemanticReplayMismatch, "packet semantic mismatch")
	}
	return got, nil
}

type CheckpointInputV2 struct {
	Packet CommunityTargetPacketV2
	Limits LimitsV2
}
type CheckpointV2 struct {
	e         stage7bEnvelope[stage7bCheckpointPayload]
	canonical []byte
}

func (x CheckpointV2) Schema() string         { return x.e.Schema }
func (x CheckpointV2) Identity() string       { return x.e.Identity }
func (x CheckpointV2) Digest() string         { return stage7bSHA(x.canonical) }
func (x CheckpointV2) ByteLength() uint64     { return x.e.ByteLength }
func (x CheckpointV2) CanonicalBytes() []byte { return append([]byte(nil), x.canonical...) }
func NewCheckpointV2(in CheckpointInputV2) (CheckpointV2, error) {
	if len(in.Packet.canonical) == 0 {
		return CheckpointV2{}, stage7bFail(failureInvalidCarrier, "missing packet")
	}
	progress := make([]stage7bProgress, len(in.Packet.e.Payload.Targets))
	for i, target := range in.Packet.e.Payload.Targets {
		progress[i] = stage7bProgress{target.Identity, "PENDING", uint64(i), false}
	}
	p := stage7bCheckpointPayload{in.Packet.Identity(), progress, stage7bAccounting{Targets: uint64(len(progress)), Work: uint64(len(progress))}}
	if uint64(len(progress)) > in.Limits.MaxTargets || uint64(len(progress)) > in.Limits.MaxWork {
		return CheckpointV2{}, stage7bFail(FailureResourceBound, "checkpoint limit")
	}
	e, raw, err := stage7bSeal("checkpoint", stage7bCheckpointSchema, p)
	if err != nil {
		return CheckpointV2{}, err
	}
	if uint64(len(raw)) > in.Limits.MaxCanonicalBytes {
		return CheckpointV2{}, stage7bFail(FailureResourceBound, "canonical limit")
	}
	return CheckpointV2{e, raw}, nil
}
func ParseCheckpointV2(raw []byte, p CommunityTargetPacketV2, l LimitsV2) (CheckpointV2, error) {
	e, err := stage7bParse[stage7bCheckpointPayload](raw, stage7bCheckpointSchema, "checkpoint")
	if err != nil {
		return CheckpointV2{}, err
	}
	got, err := NewCheckpointV2(CheckpointInputV2{p, l})
	if err != nil || e.Payload.PacketIdentity != p.Identity() || !bytes.Equal(got.canonical, raw) {
		return CheckpointV2{}, stage7bFail(FailureSemanticReplayMismatch, "checkpoint semantic mismatch")
	}
	return got, nil
}

type ReplayRequestInputV2 struct {
	Checkpoint          CheckpointV2
	Packet              CommunityTargetPacketV2
	RequestedIdentities []string
	Limits              LimitsV2
}
type ReplayRequestV2 struct {
	e         stage7bEnvelope[stage7bRequestPayload]
	canonical []byte
}

func (x ReplayRequestV2) Schema() string         { return x.e.Schema }
func (x ReplayRequestV2) Identity() string       { return x.e.Identity }
func (x ReplayRequestV2) Digest() string         { return stage7bSHA(x.canonical) }
func (x ReplayRequestV2) ByteLength() uint64     { return x.e.ByteLength }
func (x ReplayRequestV2) CanonicalBytes() []byte { return append([]byte(nil), x.canonical...) }
func NewReplayRequestV2(in ReplayRequestInputV2) (ReplayRequestV2, error) {
	if len(in.Checkpoint.canonical) == 0 || len(in.Packet.canonical) == 0 || in.Checkpoint.e.Payload.PacketIdentity != in.Packet.Identity() {
		return ReplayRequestV2{}, stage7bFail(failureInvalidCarrier, "request predecessor mismatch")
	}
	ids := append([]string(nil), in.RequestedIdentities...)
	targets := in.Packet.TargetIdentities()
	if len(ids) == 0 || len(ids) != len(targets) {
		return ReplayRequestV2{}, stage7bFail(FailureSemanticReplayMismatch, "requested identity mismatch")
	}
	for i, identity := range ids {
		if identity == "" || (i > 0 && ids[i-1] >= identity) || identity != targets[i] {
			return ReplayRequestV2{}, stage7bFail(FailureSemanticReplayMismatch, "requested identity mismatch")
		}
	}
	if uint64(len(ids)) > in.Limits.MaxRequests {
		return ReplayRequestV2{}, stage7bFail(FailureResourceBound, "request limit")
	}
	p := stage7bRequestPayload{in.Checkpoint.Identity(), in.Packet.Identity(), ids, stage7bLimitsOf(in.Limits)}
	e, raw, err := stage7bSeal("request", stage7bRequestSchema, p)
	if err != nil {
		return ReplayRequestV2{}, err
	}
	if uint64(len(raw)) > in.Limits.MaxCanonicalBytes {
		return ReplayRequestV2{}, stage7bFail(FailureResourceBound, "canonical limit")
	}
	return ReplayRequestV2{e, raw}, nil
}
func ParseReplayRequestV2(raw []byte, c CheckpointV2, p CommunityTargetPacketV2, l LimitsV2) (ReplayRequestV2, error) {
	e, err := stage7bParse[stage7bRequestPayload](raw, stage7bRequestSchema, "request")
	if err != nil {
		return ReplayRequestV2{}, err
	}
	got, err := NewReplayRequestV2(ReplayRequestInputV2{c, p, e.Payload.RequestedIdentities, l})
	if err != nil || e.Payload.CheckpointIdentity != c.Identity() || e.Payload.PacketIdentity != p.Identity() || e.Payload.Limits != stage7bLimitsOf(l) || !bytes.Equal(got.canonical, raw) {
		return ReplayRequestV2{}, stage7bFail(FailureSemanticReplayMismatch, "request semantic mismatch")
	}
	return got, nil
}

type accountingV2 struct {
	CanonicalBytes, TotalPredecessorBytes, Objects, ObjectBytes func() uint64
	Documents, Occurrences, Communities, Targets, Ranges        func() uint64
	Requests, Outputs, Work                                     func() uint64
	Balanced                                                    func() bool
}

type outputV2 struct{ o stage7bOutput }

func (x outputV2) Identity() string   { return x.o.Identity }
func (x outputV2) ExactBytes() []byte { return append([]byte(nil), x.o.ExactBytes...) }

type ReplayInputV2 struct {
	Request    ReplayRequestV2
	Checkpoint CheckpointV2
	Packet     CommunityTargetPacketV2
	Snapshot   SnapshotV2
	Objects    map[string][]byte
	Limits     LimitsV2
}
type ReplayResultV2 struct {
	e         stage7bEnvelope[stage7bResultPayload]
	canonical []byte
}

func (x ReplayResultV2) Schema() string         { return x.e.Schema }
func (x ReplayResultV2) Identity() string       { return x.e.Identity }
func (x ReplayResultV2) Digest() string         { return stage7bSHA(x.canonical) }
func (x ReplayResultV2) ByteLength() uint64     { return x.e.ByteLength }
func (x ReplayResultV2) CanonicalBytes() []byte { return append([]byte(nil), x.canonical...) }
func (x ReplayResultV2) Outputs() []outputV2 {
	if x.canonical == nil {
		return nil
	}
	out := make([]outputV2, len(x.e.Payload.Outputs))
	for i, o := range x.e.Payload.Outputs {
		out[i] = outputV2{o}
	}
	return out
}
func (x ReplayResultV2) Accounting() accountingV2 {
	a := x.e.Payload.Accounting
	return accountingV2{
		CanonicalBytes: func() uint64 { return a.CanonicalBytes }, TotalPredecessorBytes: func() uint64 { return a.TotalPredecessorBytes },
		Objects: func() uint64 { return a.Objects }, ObjectBytes: func() uint64 { return a.ObjectBytes }, Documents: func() uint64 { return a.Documents },
		Occurrences: func() uint64 { return a.Occurrences }, Communities: func() uint64 { return a.Communities }, Targets: func() uint64 { return a.Targets },
		Ranges: func() uint64 { return a.Ranges }, Requests: func() uint64 { return a.Requests }, Outputs: func() uint64 { return a.Outputs }, Work: func() uint64 { return a.Work },
		Balanced: func() bool { return a.Objects > 0 && a.Outputs > 0 },
	}
}
func ReplayV2(in ReplayInputV2) (ReplayResultV2, error) {
	zero := ReplayResultV2{}
	if len(in.Request.canonical) == 0 || len(in.Checkpoint.canonical) == 0 || len(in.Packet.canonical) == 0 || len(in.Snapshot.canonical) == 0 {
		return zero, stage7bFail(failureInvalidCarrier, "missing replay predecessor")
	}
	if in.Request.e.Payload.Limits != stage7bLimitsOf(in.Limits) {
		return zero, stage7bFail(FailureSemanticReplayMismatch, "replay limit custody mismatch")
	}
	if in.Request.e.Payload.CheckpointIdentity != in.Checkpoint.Identity() || in.Request.e.Payload.PacketIdentity != in.Packet.Identity() || in.Checkpoint.e.Payload.PacketIdentity != in.Packet.Identity() || in.Packet.e.Payload.SnapshotIdentity != in.Snapshot.Identity() {
		return zero, stage7bFail(FailureSemanticReplayMismatch, "replay chain mismatch")
	}
	for _, o := range in.Snapshot.e.Payload.Objects {
		b, ok := in.Objects[o.Identity]
		if !ok || !bytes.Equal(b, o.ExactBytes) {
			return zero, stage7bFail(FailureSemanticReplayMismatch, "object replay mismatch")
		}
		if stage7bLimit(uint64(len(b)), in.Limits.MaxObjectBytes) {
			return zero, stage7bFail(FailureResourceBound, "object byte limit")
		}
	}
	outputs := make([]stage7bOutput, len(in.Request.e.Payload.RequestedIdentities))
	for i, identity := range in.Request.e.Payload.RequestedIdentities {
		body, ok := in.Objects[identity]
		if !ok {
			return zero, stage7bFail(FailureSemanticReplayMismatch, "target replay mismatch")
		}
		if stage7bLimit(uint64(len(body)), in.Limits.MaxObjectBytes) {
			return zero, stage7bFail(FailureResourceBound, "output object byte limit")
		}
		outputs[i] = stage7bOutput{identity, append([]byte(nil), body...)}
	}
	a := in.Snapshot.e.Payload.Accounting
	a.CanonicalBytes = uint64(len(in.Snapshot.canonical) + len(in.Packet.canonical) + len(in.Checkpoint.canonical) + len(in.Request.canonical))
	a.TotalPredecessorBytes = a.ObjectBytes
	a.Requests = 1
	a.Targets = uint64(len(outputs))
	a.Outputs = uint64(len(outputs))
	a.Work = a.Objects + 1 + uint64(len(outputs))
	checks := []struct{ got, max uint64 }{{a.CanonicalBytes, in.Limits.MaxCanonicalBytes}, {a.TotalPredecessorBytes, in.Limits.MaxTotalPredecessorBytes}, {a.Objects, in.Limits.MaxObjects}, {a.Documents, in.Limits.MaxDocuments}, {a.Occurrences, in.Limits.MaxOccurrences}, {a.Communities, in.Limits.MaxCommunities}, {a.Targets, in.Limits.MaxTargets}, {a.Ranges, in.Limits.MaxRanges}, {a.Requests, in.Limits.MaxRequests}, {a.Outputs, in.Limits.MaxOutputs}, {a.Work, in.Limits.MaxWork}}
	for _, c := range checks {
		if c.got > c.max {
			return zero, stage7bFail(FailureResourceBound, "replay resource bound")
		}
	}
	p := stage7bResultPayload{in.Request.Identity(), "", outputs, a}
	e, raw, err := stage7bSeal("result", stage7bResultSchema, p)
	if err != nil {
		return zero, err
	}
	if stage7bLimit(uint64(len(raw)), in.Limits.MaxCanonicalBytes) {
		return zero, stage7bFail(FailureResourceBound, "result canonical byte limit")
	}
	return ReplayResultV2{e, raw}, nil
}
func ParseReplayResultV2(raw []byte, in ReplayInputV2) (ReplayResultV2, error) {
	if _, err := stage7bParse[stage7bResultPayload](raw, stage7bResultSchema, "result"); err != nil {
		return ReplayResultV2{}, err
	}
	got, err := ReplayV2(in)
	if err != nil || !bytes.Equal(got.canonical, raw) {
		return ReplayResultV2{}, stage7bFail(FailureSemanticReplayMismatch, "result semantic mismatch")
	}
	return got, nil
}
