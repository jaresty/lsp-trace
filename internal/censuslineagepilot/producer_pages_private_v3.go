//go:build darwin || linux

package censuslineagepilot

// This unactivated successor is deliberately NOT a ProducerPacketPrivateV2.
// Pages bind raw byte intervals, not JSON values. The linked coverage index is
// hash-bound, NOT authenticated; it establishes neither feature identity nor
// source completeness. Original opaque capabilities remain the admission owner.
import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"

	"lsp-trace/internal/publication"
)

const ProducerPagesSchemaPrivateV3 = "lsp-trace.producer-pages.private.v3"
const producerPageHeaderPrivateV3 = 4096
const producerPageSafetyPrivateV3 = 1 << 20

var ErrPagesPausedPrivateV3 = errors.New("PAUSED_BUDGET_PRIVATE_V3")
var ErrPagesUnitPrivateV3 = errors.New("UNSUPPORTED_COMPUTATION_UNIT_PRIVATE_V3")

// Resource dimensions are independent reservation units, not heap/CPU telemetry.
// Reservation is nonrefundable, including failure/cancellation. There are NO
// defaults: page size, exchange allowance and total application allowance must
// all be explicitly supplied. Increasing an allowance is a new caller decision.
type PageCostPrivateV3 struct {
	LogicalBytes  uint64 `json:"logical_bytes"`
	ReadHashBytes uint64 `json:"read_hash_bytes"`
	WriteBytes    uint64 `json:"write_bytes"`
	Work          uint64 `json:"work"`
}

func (a PageCostPrivateV3) fits(b PageCostPrivateV3) bool {
	return a.LogicalBytes <= b.LogicalBytes && a.ReadHashBytes <= b.ReadHashBytes && a.WriteBytes <= b.WriteBytes && a.Work <= b.Work
}
func (a PageCostPrivateV3) plus(b PageCostPrivateV3) (PageCostPrivateV3, error) {
	av := []uint64{a.LogicalBytes, a.ReadHashBytes, a.WriteBytes, a.Work}
	bv := []uint64{b.LogicalBytes, b.ReadHashBytes, b.WriteBytes, b.Work}
	for i := range av {
		if bv[i] > math.MaxUint64-av[i] {
			return PageCostPrivateV3{}, ErrInvalid
		}
	}
	return PageCostPrivateV3{a.LogicalBytes + b.LogicalBytes, a.ReadHashBytes + b.ReadHashBytes, a.WriteBytes + b.WriteBytes, a.Work + b.Work}, nil
}
func (a PageCostPrivateV3) minus(b PageCostPrivateV3) PageCostPrivateV3 {
	return PageCostPrivateV3{a.LogicalBytes - b.LogicalBytes, a.ReadHashBytes - b.ReadHashBytes, a.WriteBytes - b.WriteBytes, a.Work - b.Work}
}

type PagePolicyPrivateV3 struct {
	PayloadBytes uint64 `json:"payload_bytes"`
	// An explicit safety limit for parsers that still require contiguous objects.
	// Retention segments larger objects; this field never discards retained bytes.
	ComputationUnitBytes uint64 `json:"computation_unit_bytes"`
	ConfigSHA256         string `json:"config_sha256"`
}

func (p PagePolicyPrivateV3) valid() bool {
	return p.PayloadBytes > 0 && p.PayloadBytes <= producerPageSafetyPrivateV3 && p.ComputationUnitBytes > 0 && p.ComputationUnitBytes <= 16<<20 && validPageDigestPrivateV3(p.ConfigSHA256)
}
func validPageDigestPrivateV3(s string) bool {
	if len(s) != 71 || s[:7] != "sha256:" {
		return false
	}
	for _, c := range s[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

type pageObjectPrivateV3 struct {
	role          string
	member, batch int
	raw           []byte
}

// Owner and ledger cannot be decoded from a report/checkpoint. All successive
// exchanges and selected reads use the SAME ledger, including failed attempts.
type ProducerPagesPrivateV3 struct{ state *producerPagesStatePrivateV3 }
type producerPagesStatePrivateV3 struct {
	self         *producerPagesStatePrivateV3
	mu           sync.Mutex
	frozen       FrozenScopePrivateV2
	origin       EnumerationOriginPrivateV2
	captures     []OpaqueSuccessfulV2Capture
	roots        ProducerPacketRootsPrivateV2
	policy       PagePolicyPrivateV3
	objects      []pageObjectPrivateV3 // bounded by ORIGINAL scope profile, not application scale
	binding      string
	ledger       PageLedgerPrivateV3
	generation   uint64
	lastRevision string
	cursor       PageCursorPrivateV3
	ops          scopeRetentionOps
}

func (ProducerPagesPrivateV3) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }

type PageCursorPrivateV3 struct {
	PageCount uint64 `json:"page_count"`
	Object    uint64 `json:"object"`
	Offset    uint64 `json:"offset"`
	RawBytes  uint64 `json:"raw_bytes"`
	Last      string `json:"last_page_sha256"`
}
type PageRevisionPrivateV3 struct {
	Schema            string              `json:"schema"`
	Binding           string              `json:"binding_sha256"`
	Policy            PagePolicyPrivateV3 `json:"policy"`
	Generation        uint64              `json:"generation"`
	Previous          string              `json:"previous_revision_sha256"`
	Cursor            PageCursorPrivateV3 `json:"cursor"`
	ObjectDenominator uint64              `json:"object_denominator"`
	BatchDenominator  uint64              `json:"batch_denominator"`
	Status            string              `json:"status"`
	Spent             PageCostPrivateV3   `json:"cumulative_reserved"`
	Authority         uint8               `json:"authority"`
	Accepted          bool                `json:"accepted"`
	Feature           string              `json:"feature_identity"`
	Source            string              `json:"source_completeness"`
	Absence           string              `json:"absence_inference"`
	Qualification     string              `json:"qualification"`
}
type pageHeaderPrivateV3 struct {
	Schema   string `json:"schema"`
	Binding  string `json:"binding_sha256"`
	Previous string `json:"previous_page_sha256"`
	Index    uint64 `json:"index"`
	Object   uint64 `json:"object"`
	Role     string `json:"role"`
	Member   int    `json:"member"`
	Batch    int    `json:"batch"`
	Length   uint64 `json:"object_byte_length"`
	Offset   uint64 `json:"offset"`
}

// NewProducerPagesPrivateV3 admits ONLY the original frozen scope, enumeration,
// and complete ordered original success owners. No SourceManager/guard replay,
// DTO restoration, packet allocation, or legacy profile override occurs here.
func NewProducerPagesPrivateV3(ctx context.Context, f FrozenScopePrivateV2, o EnumerationOriginPrivateV2, captures []OpaqueSuccessfulV2Capture, roots ProducerPacketRootsPrivateV2, p PagePolicyPrivateV3, ledger PageLedgerPrivateV3) (ProducerPagesPrivateV3, error) {
	if ctx == nil || ctx.Err() != nil || ledger.state == nil || roots.state == nil || roots.state.self != roots.state || !p.valid() || !f.valid() || !o.valid() || o.state.frozen.state != f.state || len(captures) == 0 || len(captures) != len(o.state.batches) {
		return ProducerPagesPrivateV3{}, ErrInvalid
	}
	// These are original producer safety bounds, not total application bounds.
	if len(captures) > 63 || len(f.state.files) > 16 {
		return ProducerPagesPrivateV3{}, ErrInvalid
	}
	for i, c := range captures {
		if !successfulProducerLineagePrivateV2(f, o, c) || c.state.ordinal != i {
			return ProducerPagesPrivateV3{}, ErrInvalid
		}
	}
	// Constructor reservations precede metadata encoding, cloning and topology IO.
	left := ledger.state.allowance
	initial := PageCostPrivateV3{LogicalBytes: 4 * scopeMetadataPrivateV2, ReadHashBytes: 8 * scopeMetadataPrivateV2, Work: 2*producerPacketTopologyWorkPrivateV2 + 512}
	if err := ledger.state.reserve(initial, &left); err != nil {
		return ProducerPagesPrivateV3{}, err
	}
	if err := publication.CheckProducerPacketRetentionTopologyPrivateV2(ctx, roots.state.outputs, roots.state.source, append(append([]*publication.ObservedRoot(nil), roots.state.additional...), ledger.state.observed)); err != nil {
		return ProducerPagesPrivateV3{}, err
	}
	s := &producerPagesStatePrivateV3{frozen: f, origin: o, captures: append([]OpaqueSuccessfulV2Capture(nil), captures...), roots: roots, policy: p, ledger: ledger, ops: producerPacketRetentionOpsPrivateV2()}
	// Metadata source objects are bounded by the existing original producer. No
	// encoded artifact or all-origin packet is formed, even for oversized artifacts.
	s.objects = []pageObjectPrivateV3{{"FROZEN_SPEC", -1, -1, f.state.specBytes}, {"FROZEN_MANIFEST", -1, -1, f.state.manifest}, {"NOMINATION_SELECTION", -1, -1, f.state.selection}, {"FILE_PLAN", -1, -1, f.state.filePlan}}
	policy, _ := json.Marshal(f.state.spec.EffectivePolicy)
	meta, _ := json.Marshal(o.state.adapter.metadata)
	s.objects = append(s.objects, pageObjectPrivateV3{"EFFECTIVE_POLICY", -1, -1, policy}, pageObjectPrivateV3{"ENUMERATION_ORIGIN", -1, -1, o.state.raw}, pageObjectPrivateV3{"RUNTIME_METADATA", -1, -1, meta})
	for i, file := range f.state.files {
		s.objects = append(s.objects, pageObjectPrivateV3{"OBSERVED_SOURCE_BODY", i, -1, file.Bytes})
	}
	for i, c := range captures {
		s.objects = append(s.objects, pageObjectPrivateV3{"CANONICAL_BATCH_SEEDS", -1, i, c.state.seeds}, pageObjectPrivateV3{"FULL_CAPTURE_ORIGIN", -1, i, c.state.origin}, pageObjectPrivateV3{"SOURCE_ARTIFACT_PRIVATE_V2", -1, i, c.state.artifact}, pageObjectPrivateV3{"CANONICAL_MANIFEST_INPUT", -1, i, o.state.batches[i].binding.Input})
	}
	if err := roots.check(ctx, f); err != nil {
		return ProducerPagesPrivateV3{}, err
	}
	pin, _ := json.Marshal(struct {
		Schema, Spec, Manifest, Origin, Runtime, Application string
		Policy                                               PagePolicyPrivateV3
	}{ProducerPagesSchemaPrivateV3, Digest(f.state.specBytes), Digest(f.state.manifest), Digest(o.state.raw), Digest(meta), ledger.state.application, p})
	s.binding = Digest(pin)
	s.self = s
	return ProducerPagesPrivateV3{s}, nil
}
func (s *producerPagesStatePrivateV3) valid() bool {
	return s != nil && s.self == s && s.frozen.valid() && s.origin.valid() && s.origin.state.frozen.state == s.frozen.state
}
func (s *producerPagesStatePrivateV3) reserve(c PageCostPrivateV3, left *PageCostPrivateV3) error {
	return s.ledger.state.reserve(c, left)
}

// Reserve BEFORE the unchanged FrozenScopePrivateV2.revalidate reader. That
// reader hashes/clones CURRENT bytes before comparing with the frozen bytes, so
// frozen lengths cannot bound failed work. Its descriptor-bound LimitReader
// admits at most min(members*FileBytes, TotalBytes), plus a growth sentinel per
// member. This also covers growth after stat; no ambient-path stat shortcut or
// new source permission is used here.
//
// One reservation covers up to TWO revalidations. The 24 logical / 32 read-hash
// passes conservatively cover ReadAll's intermediate/final buffers, clone and
// comparison/hash scans, with scopeMetadataPrivateV2 carried through those same
// passes for bounded descriptor/path/result metadata and small-buffer overhead.
// This is nonrefundable logical work, not heap/CPU telemetry. Intentionally pay
// policy maxima even for tiny unchanged sources; never enlarge the allowance.
func (s *producerPagesStatePrivateV3) sourceCheckCost() (PageCostPrivateV3, error) {
	if s == nil || !s.frozen.valid() {
		return PageCostPrivateV3{}, ErrInvalid
	}
	p := s.frozen.state.spec.EffectivePolicy
	members := len(s.frozen.state.files)
	// Validate positive bounded integers BEFORE unsigned conversion/multiplication:
	// members <= 16, FileBytes <= 2 MiB, TotalBytes <= 8 MiB. Thus all subsequent
	// additions/products (including sentinels, metadata and passes) cannot overflow.
	if p.validate() != nil || members < 1 || members > 16 || members > p.MaxMembers {
		return PageCostPrivateV3{}, ErrInvalid
	}
	n := uint64(p.TotalBytes)
	if files := uint64(members) * uint64(p.FileBytes); files < n {
		n = files
	}
	n += uint64(members) + scopeMetadataPrivateV2
	return PageCostPrivateV3{LogicalBytes: 24 * n, ReadHashBytes: 32 * n, Work: 2 * producerPacketTopologyWorkPrivateV2}, nil
}

// Retention uses the unchanged common pair algorithm. These reservations include
// its 16 logical / 32 read-hash passes, plus frame encoding, hash and comparisons;
// writes include two physical objects. No fictional lower-copy accounting.
func pagePairCostPrivateV3(n uint64) PageCostPrivateV3 {
	return PageCostPrivateV3{18 * (n + 1), 36 * (n + 1), 2 * n, producerPacketTopologyWorkPrivateV2 + 32}
}
func (s *producerPagesStatePrivateV3) retain(ctx context.Context, raw []byte) (string, error) {
	id := Digest(raw)
	if e := s.roots.check(ctx, s.frozen); e != nil {
		return "", e
	}
	_, err := retainScopePairBytes(ctx, scopeRetentionObject{id, raw}, id, 0, s.roots.state.outputs[0], s.roots.state.outputs[1], s.ops)
	if err != nil {
		return "", err
	}
	if err = publication.CheckProducerPacketObjectsDistinctPrivateV2(ctx, s.roots.state.outputs, id); err != nil {
		return "", err
	}
	if err = s.roots.check(ctx, s.frozen); err != nil {
		return "", err
	}
	return id, ctx.Err()
}
func encodePagePrivateV3(h pageHeaderPrivateV3, raw []byte) ([]byte, error) {
	header, e := json.Marshal(h)
	if e != nil || len(header) > producerPageHeaderPrivateV3 {
		return nil, ErrInvalid
	}
	out := make([]byte, 4+len(header)+len(raw))
	binary.BigEndian.PutUint32(out, uint32(len(header)))
	copy(out[4:], header)
	copy(out[4+len(header):], raw)
	return out, nil
}
func decodePagePrivateV3(raw []byte) (h pageHeaderPrivateV3, body []byte, err error) {
	if len(raw) < 4 {
		return h, nil, ErrInvalid
	}
	n := int(binary.BigEndian.Uint32(raw))
	if n < 1 || n > producerPageHeaderPrivateV3 || n > len(raw)-4 {
		return h, nil, ErrInvalid
	}
	if producerPacketStrictPrivateV2(raw[4:4+n], &h) != nil {
		return h, nil, ErrInvalid
	}
	return h, raw[4+n:], nil
}

// Advance is an exchange, not an application. The cumulative ledger never resets
// when a new exchange/generation starts. A generation is spent on entry; using an
// old generation refuses before work. Revision publication is reserved FIRST so
// budget pauses can retain a useful immutable prefix. Cancellation may prevent
// publication, but cannot erase previously published revisions/pages.
func (p ProducerPagesPrivateV3) Advance(ctx context.Context, generation uint64, exchange PageCostPrivateV3, previous string) (PageRevisionPrivateV3, string, error) {
	if p.state == nil {
		return PageRevisionPrivateV3{}, "", ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || ctx == nil || ctx.Err() != nil || generation != s.generation+1 {
		return PageRevisionPrivateV3{}, "", ErrInvalid
	}
	if s.generation > 0 && previous != sRevisionDigestPrivateV3(s) {
		return PageRevisionPrivateV3{}, "", ErrInvalid
	}
	if s.generation == 0 && previous != "" {
		return PageRevisionPrivateV3{}, "", ErrInvalid
	}
	left := exchange
	sourceCost, err := s.sourceCheckCost()
	if err != nil {
		return s.revision("FAILED", previous), "", err
	}
	entryCost, err := pagePairCostPrivateV3(producerPageHeaderPrivateV3).plus(sourceCost)
	if err != nil {
		return s.revision("FAILED", previous), "", err
	}
	entryCost, _ = entryCost.plus(pageJournalCostPrivateV3)
	if e := s.reserve(entryCost, &left); e != nil {
		return s.revision("PAUSED_BUDGET", previous), "", e
	}
	s.generation = generation
	if e := s.claimAttempt(generation, previous); e != nil {
		return s.revision("FAILED", previous), "", e
	}
	if e := s.frozen.revalidate(ctx); e != nil {
		return s.revision("FAILED", previous), "", e
	}
	status := "COMPLETE_ORIGIN_PAGES"
	var cause error
	for s.cursor.Object < uint64(len(s.objects)) {
		if e := ctx.Err(); e != nil {
			cause = e
			status = "CANCELLED"
			break
		}
		obj := s.objects[s.cursor.Object]
		if s.cursor.Offset > uint64(len(obj.raw)) {
			cause = ErrInvalid
			status = "FAILED"
			break
		}
		n := uint64(len(obj.raw)) - s.cursor.Offset
		if n > s.policy.PayloadBytes {
			n = s.policy.PayloadBytes
		}
		// Zero-byte objects have a real indexed frame; their bytes are never omitted.
		cost := pagePairCostPrivateV3(n + producerPageHeaderPrivateV3 + 4)
		if e := s.reserve(cost, &left); e != nil {
			cause = e
			status = "PAUSED_BUDGET"
			break
		}
		h := pageHeaderPrivateV3{ProducerPagesSchemaPrivateV3, s.binding, s.cursor.Last, s.cursor.PageCount, s.cursor.Object, obj.role, obj.member, obj.batch, uint64(len(obj.raw)), s.cursor.Offset}
		frame, e := encodePagePrivateV3(h, obj.raw[s.cursor.Offset:s.cursor.Offset+n])
		if e != nil {
			cause = e
			status = "FAILED"
			break
		}
		id, e := s.retain(ctx, frame)
		if e != nil {
			cause = e
			status = "FAILED"
			break
		}
		s.cursor.Last = id
		s.cursor.PageCount++
		s.cursor.RawBytes += n
		s.cursor.Offset += n
		if s.cursor.Offset == uint64(len(obj.raw)) {
			s.cursor.Object++
			s.cursor.Offset = 0
		}
	}
	if e := s.frozen.revalidate(ctx); e != nil {
		cause = errors.Join(cause, e)
		status = "FAILED"
	}
	r := s.revision(status, previous)
	raw, _ := json.Marshal(r)
	if len(raw) > producerPageHeaderPrivateV3 {
		return r, "", ErrInvalid
	}
	id, e := s.retain(ctx, raw)
	if e != nil {
		return r, "", errors.Join(cause, e)
	}
	s.lastRevision = id
	if e := s.checkpoint(id); e != nil {
		return r, id, errors.Join(cause, e)
	}
	return r, id, cause
}
func sRevisionDigestPrivateV3(s *producerPagesStatePrivateV3) string { return s.lastRevision }
func (s *producerPagesStatePrivateV3) spent() PageCostPrivateV3 {
	_, _, spent := s.ledger.Tip()
	return spent
}
func (s *producerPagesStatePrivateV3) revision(status, previous string) PageRevisionPrivateV3 {
	return PageRevisionPrivateV3{Schema: ProducerPagesSchemaPrivateV3, Binding: s.binding, Policy: s.policy, Generation: s.generation, Previous: previous, Cursor: s.cursor, ObjectDenominator: uint64(len(s.objects)), BatchDenominator: uint64(len(s.captures)), Status: status, Spent: s.spent(), Feature: "UNRESOLVED", Source: "UNKNOWN", Absence: "PROHIBITED", Qualification: "ORIGINAL_OPAQUE_OWNERS_ONLY_HASH_INDEX_NOT_AUTHENTICATED"}
}

// VerifyPrefix checks every ordered binding against original owner bytes. It
// keeps one frame in memory: the index is itself paged, not an unbounded header.
// A partial prefix does not imply complete source coverage or semantic results.
func (p ProducerPagesPrivateV3) VerifyPrefix(ctx context.Context, r PageRevisionPrivateV3, exchange PageCostPrivateV3) error {
	if p.state == nil {
		return ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.verify(ctx, r, &exchange, nil, nil)
}
func (s *producerPagesStatePrivateV3) verify(ctx context.Context, r PageRevisionPrivateV3, left *PageCostPrivateV3, visit func(pageHeaderPrivateV3, []byte) error, progress *pageVerifyStatePrivateV3) error {
	if !s.valid() || ctx == nil || ctx.Err() != nil || r.Schema != ProducerPagesSchemaPrivateV3 || r.Binding != s.binding || r.Policy != s.policy || r.Authority != 0 || r.Accepted || r.Feature != "UNRESOLVED" || r.Source != "UNKNOWN" || r.Absence != "PROHIBITED" || r.ObjectDenominator != uint64(len(s.objects)) || r.BatchDenominator != uint64(len(s.captures)) || r.Cursor.Object > uint64(len(s.objects)) || r.Cursor.Object == uint64(len(s.objects)) && r.Cursor.Offset != 0 {
		return ErrInvalid
	}
	if r.Status == "COMPLETE_ORIGIN_PAGES" && (r.Cursor.Object != r.ObjectDenominator || r.Cursor.Offset != 0) {
		return ErrInvalid
	}
	if r.Status != "COMPLETE_ORIGIN_PAGES" && r.Status != "PAUSED_BUDGET" && r.Status != "FAILED" && r.Status != "CANCELLED" {
		return ErrInvalid
	}
	for i, c := range s.captures {
		if !successfulProducerLineagePrivateV2(s.frozen, s.origin, c) || c.state.ordinal != i {
			return ErrInvalid
		}
	}
	if e := s.verifyRevision(ctx, r, left); e != nil {
		return e
	}
	sourceCost, err := s.sourceCheckCost()
	if err != nil {
		return err
	}
	if e := s.reserve(sourceCost, left); e != nil {
		return e
	}
	if e := s.frozen.revalidate(ctx); e != nil {
		return e
	}
	cur := r.Cursor
	id := cur.Last
	if progress != nil {
		cur = progress.remaining
		id = progress.last
	}
	for cur.PageCount > 0 {
		max := s.policy.PayloadBytes + producerPageHeaderPrivateV3 + 4
		if err := s.reserve(PageCostPrivateV3{2 * (max + 1), 8 * (max + 1), 0, producerPacketTopologyWorkPrivateV2 + 16}, left); err != nil {
			return err
		}
		if err := s.roots.check(ctx, s.frozen); err != nil {
			return err
		}
		var raw []byte
		for _, root := range s.roots.state.outputs {
			b, e := s.ops.read(ctx, root, id, int64(max))
			if e != nil {
				return e
			}
			if Digest(b) != id || raw != nil && !bytes.Equal(raw, b) {
				return ErrInvalid
			}
			raw = b
		}
		if err := publication.CheckProducerPacketObjectsDistinctPrivateV2(ctx, s.roots.state.outputs, id); err != nil {
			return err
		}
		h, b, e := decodePagePrivateV3(raw)
		if e != nil {
			return e
		}
		if h.Schema != ProducerPagesSchemaPrivateV3 || h.Binding != s.binding || h.Index != cur.PageCount-1 || h.Object >= uint64(len(s.objects)) {
			return ErrInvalid
		}
		obj := s.objects[h.Object]
		if h.Role != obj.role || h.Member != obj.member || h.Batch != obj.batch || h.Length != uint64(len(obj.raw)) || h.Offset > h.Length || uint64(len(b)) > h.Length-h.Offset || uint64(len(b)) > s.policy.PayloadBytes {
			return ErrInvalid
		}
		end := h.Offset + uint64(len(b))
		if cur.Offset == 0 {
			if h.Object+1 != cur.Object || end != h.Length {
				return ErrInvalid
			}
		} else if h.Object != cur.Object || end != cur.Offset {
			return ErrInvalid
		}
		if h.Length > 0 && len(b) == 0 || !bytes.Equal(b, obj.raw[h.Offset:end]) || uint64(len(b)) > cur.RawBytes {
			return ErrInvalid
		}
		if visit != nil {
			if e := visit(h, b); e != nil {
				return e
			}
		}
		cur.PageCount--
		cur.Object = h.Object
		cur.Offset = h.Offset
		cur.RawBytes -= uint64(len(b))
		id = h.Previous
		if progress != nil {
			progress.remaining = cur
			progress.last = id
		}
	}
	if id != "" || cur.Object != 0 || cur.Offset != 0 || cur.RawBytes != 0 {
		return ErrInvalid
	}
	if e := s.frozen.revalidate(ctx); e != nil {
		return e
	}
	return s.roots.check(ctx, s.frozen)
}

func revisionReadCostPrivateV3() PageCostPrivateV3 {
	return PageCostPrivateV3{LogicalBytes: 4 * producerPageHeaderPrivateV3, ReadHashBytes: 12 * producerPageHeaderPrivateV3, Work: producerPacketTopologyWorkPrivateV2 + 32}
}
func (s *producerPagesStatePrivateV3) verifyRevision(ctx context.Context, r PageRevisionPrivateV3, left *PageCostPrivateV3) error {
	if e := s.reserve(revisionReadCostPrivateV3(), left); e != nil {
		return e
	}
	raw, e := json.Marshal(r)
	if e != nil || len(raw) > producerPageHeaderPrivateV3 {
		return ErrInvalid
	}
	id := Digest(raw)
	for _, root := range s.roots.state.outputs {
		b, e := s.ops.read(ctx, root, id, producerPageHeaderPrivateV3)
		if e != nil {
			return e
		}
		if !bytes.Equal(b, raw) || Digest(b) != id {
			return ErrInvalid
		}
	}
	return publication.CheckProducerPacketObjectsDistinctPrivateV2(ctx, s.roots.state.outputs, id)
}

// ReadObject returns one bounded detached original-byte object AFTER complete
// prefix verification. It never constructs the legacy packet qualification or
// AST/Native owner. Oversized objects remain fully retained but report a truthful
// computation-unit gap instead of silent truncation or an enlarged legacy cap.
func (p ProducerPagesPrivateV3) ReadObject(ctx context.Context, r PageRevisionPrivateV3, object uint64, exchange PageCostPrivateV3) ([]byte, error) {
	if p.state == nil {
		return nil, ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || object >= uint64(len(s.objects)) || object >= r.Cursor.Object {
		return nil, ErrInvalid
	}
	n := uint64(len(s.objects[object].raw))
	if n > s.policy.ComputationUnitBytes {
		return nil, ErrPagesUnitPrivateV3
	}
	if err := s.reserve(PageCostPrivateV3{LogicalBytes: n, ReadHashBytes: n, Work: 1}, &exchange); err != nil {
		return nil, err
	}
	out := make([]byte, n)
	err := s.verify(ctx, r, &exchange, func(h pageHeaderPrivateV3, b []byte) error {
		if h.Object == object {
			copy(out[h.Offset:], b)
		}
		return nil
	}, nil)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p ProducerPagesPrivateV3) BudgetSpent() PageCostPrivateV3 {
	if p.state == nil {
		return PageCostPrivateV3{}
	}
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	return p.state.spent()
}
func (p ProducerPagesPrivateV3) String() string {
	return fmt.Sprintf("%s (private, not activated)", ProducerPagesSchemaPrivateV3)
}
