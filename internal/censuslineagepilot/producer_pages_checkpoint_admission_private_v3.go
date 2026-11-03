//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/publication"
)

const RetainedCheckpointProfilePrivateV3 = "REOPENED_RETAINED_OBSERVATIONS_NOT_ORIGINAL_CAPTURE_OWNERSHIP"
const retainedCheckpointSchemaPrivateV3 = "lsp-trace.retained-page-checkpoint.private.v3.1"
const RetainedCheckpointPreparationRevisionPrivateV3 = "retained-observational-neutral-math-own-ast-v3.1"

// Closed, canonical, bounded metadata. Index entries are separate paired objects;
// neither the whole enumeration nor all capture arrays occur in this manifest.
// Hashes bind bytes, NOT origin authentication, history or permission.
type RetainedPageCheckpointManifestPrivateV3 struct {
	Schema, Profile, Application, Intent                  string
	PreparationRevision                                   string
	Authority                                             uint8
	Accepted                                              bool
	FeatureIdentity, SourceCompleteness, AbsenceInference string
	LifecycleID, PreviousCheckpoint                       string
	Generation                                            uint64
	Revision                                              PageRevisionPrivateV3
	Index                                                 string
	Reports                                               string
	SemanticStage                                         string
	SelectionSnapshot                                     string
	Objects, Members, Targets                             uint64
	SpecSHA256, ScopeSHA256, OriginSHA256, RuntimeSHA256  string
	Roots                                                 [2]retainedRootPinPrivateV3
	LedgerRoot                                            retainedRootPinPrivateV3
	LedgerGeneration                                      uint64
	LedgerTip                                             string
	Allowance, Charged                                    PageCostPrivateV3
	NextBatch                                             int
}
type retainedRootPinPrivateV3 struct {
	Path          string
	Device, Inode uint64
}
type retainedObjectIndexPrivateV3 struct {
	Schema, Binding, Next string
	Object                uint64
	Role                  string
	Member, Batch         int
	Length                uint64
	SHA256, LastPage      string
	PageCount             uint64
}

// Disclosure is a CURRENT caller decision to process retained observations.
// It grants no old runtime/capture ownership and no automatic budget extension.
type RetainedCheckpointDisclosurePrivateV3 struct{ Application, ConfigSHA256, ContinuationIntent, PreparationRevision string }
type RetainedPageCheckpointOwnerPrivateV3 struct {
	state *retainedCheckpointStatePrivateV3
}
type retainedCheckpointStatePrivateV3 struct {
	self           *retainedCheckpointStatePrivateV3
	mu             sync.Mutex
	manifest       RetainedPageCheckpointManifestPrivateV3
	checkpoint     string
	frozen         FrozenScopePrivateV2 // fresh caller admission, never restored from checkpoint
	roots          ProducerPacketRootsPrivateV2
	ledger         PageLedgerPrivateV3
	failed         bool
	pendingReports string
	savedSelection *retainedSelectedReaderStatePrivateV3
}
type RetainedSelectedBatchPrivateV3 struct {
	state *retainedSelectedBatchStatePrivateV3
}
type retainedSelectedBatchStatePrivateV3 struct {
	self       *retainedSelectedBatchStatePrivateV3
	owner      *retainedCheckpointStatePrivateV3
	ordinal    int
	checkpoint string
	raw        [4][]byte
	source     graphprovenance.SourceWithholdingPrivateV2
}
type RetainedSelectedBatchViewPrivateV3 struct {
	Profile, CheckpointSHA256, ScopeSHA256, OriginSHA256, RuntimeSHA256           string
	Binding, RevisionSHA256                                                       string
	Revision                                                                      PageRevisionPrivateV3
	OriginalNativeV3                                                              []byte
	Ordinal, FullBatchCount, FullTargetCount                                      int
	ObjectSHA256                                                                  [4]string
	CanonicalSeeds, CaptureOrigin, SourceArtifact, ManifestInput, RuntimeMetadata []byte
	Source                                                                        graphprovenance.SourceWithholdingPrivateV2
	Authority                                                                     int
	Accepted                                                                      bool
	SourceCompleteness, FeatureIdentity, AbsenceInference                         string
}

func (RetainedPageCheckpointOwnerPrivateV3) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (RetainedSelectedBatchPrivateV3) MarshalJSON() ([]byte, error)       { return nil, ErrInvalid }
func canonicalRetainedPrivateV3(raw []byte, out any) error {
	if len(raw) == 0 || len(raw) > producerPageHeaderPrivateV3 || producerPacketStrictPrivateV2(raw, out) != nil {
		return ErrInvalid
	}
	b, e := json.Marshal(out)
	if e != nil || !bytes.Equal(b, raw) {
		return ErrInvalid
	}
	return nil
}
func retainedAnchorNamePrivateV3(id string) string { return "retained-checkpoint-" + id[7:] + ".json" }
func retainedClaimNamePrivateV3(id string) string  { return "retained-claim-" + id[7:] + ".json" }
func checkpointReadCostPrivateV3(n uint64) PageCostPrivateV3 {
	return PageCostPrivateV3{LogicalBytes: 8 * (n + 1), ReadHashBytes: 16 * (n + 1), Work: producerPacketTopologyWorkPrivateV2 + 64}
}

// Export is original-owner-only. It does not alter original pages/checkpoints.
// Exporting an incomplete object refuses: partial bytes never become complete.
func (p ProducerPagesPrivateV3) ExportRetainedCheckpointPrivateV3(ctx context.Context, r PageRevisionPrivateV3, intent string, exchange PageCostPrivateV3) ([]byte, error) {
	if p.state == nil {
		return nil, ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || ctx == nil || ctx.Err() != nil || !validPageDigestPrivateV3(intent) || r.Cursor.Offset != 0 || r.Cursor.Object == 0 {
		return nil, ErrInvalid
	}
	// Verify all raw page intervals against ACTUAL original bytes before export.
	index := ""
	e := s.verify(ctx, r, &exchange, func(h pageHeaderPrivateV3, b []byte) error {
		if h.Offset+uint64(len(b)) != h.Length {
			return nil
		}
		obj := s.objects[h.Object]
		d := retainedObjectIndexPrivateV3{retainedCheckpointSchemaPrivateV3, s.binding, index, h.Object, h.Role, h.Member, h.Batch, h.Length, "", "", h.Index + 1}
		// Hashing original object bytes is reserved independently of page verification.
		if e := s.reserve(checkpointReadCostPrivateV3(h.Length+2*producerPageHeaderPrivateV3+4), &exchange); e != nil {
			return e
		}
		d.SHA256 = Digest(obj.raw)
		// The visitor is called on the exact frame; recreate its exact canonical frame.
		frame, e := encodePagePrivateV3(h, b)
		if e != nil {
			return e
		}
		d.LastPage = Digest(frame)
		raw, _ := json.Marshal(d)
		if len(raw) > producerPageHeaderPrivateV3 {
			return ErrInvalid
		}
		if e = s.reserve(pagePairCostPrivateV3(uint64(len(raw))), &exchange); e != nil {
			return e
		}
		index, e = s.retain(ctx, raw)
		return e
	}, nil)
	if e != nil {
		return nil, e
	}
	m := RetainedPageCheckpointManifestPrivateV3{Schema: retainedCheckpointSchemaPrivateV3, PreparationRevision: RetainedCheckpointPreparationRevisionPrivateV3, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", Profile: RetainedCheckpointProfilePrivateV3, Application: s.ledger.state.application, Intent: intent, Generation: 1, Revision: r, Index: index, Objects: r.Cursor.Object, Members: uint64(len(s.frozen.state.files)), SpecSHA256: Digest(s.frozen.state.specBytes), ScopeSHA256: Digest(s.frozen.state.manifest), OriginSHA256: Digest(s.origin.state.raw)}
	if e = s.reserve(PageCostPrivateV3{LogicalBytes: 256 * uint64(len(s.origin.state.raw)), ReadHashBytes: 256 * uint64(len(s.origin.state.raw)), Work: 1000000}, &exchange); e != nil {
		return nil, e
	}
	m.Targets, e = countRetainedEnumerationTargetsPrivateV3(ctx, s.origin.state.raw)
	if e != nil {
		return nil, e
	}
	meta, _ := json.Marshal(s.origin.state.adapter.metadata)
	m.RuntimeSHA256 = Digest(meta)
	m.LifecycleID = Digest(mustPageJSONPrivateV3(struct{ Schema, Application, Binding, InitialIntent string }{m.Schema, m.Application, r.Binding, intent}))
	obs, e := publication.ObserveProducerPacketPairCustodyPrivateV2(ctx, s.roots.state.outputs, Digest(mustPageJSONPrivateV3(r)))
	if e != nil {
		return nil, e
	}
	for i, v := range obs {
		m.Roots[i] = retainedRootPinPrivateV3{v.RootPath, v.RootDevice, v.RootInode}
	}
	return writeRetainedCheckpointPrivateV3(ctx, m, s.ledger, s.roots, s.frozen, &exchange)
}
func writeRetainedCheckpointPrivateV3(ctx context.Context, m RetainedPageCheckpointManifestPrivateV3, l PageLedgerPrivateV3, roots ProducerPacketRootsPrivateV2, f FrozenScopePrivateV2, left *PageCostPrivateV3) ([]byte, error) {
	// Reserve journal and maximum encoded manifest pair BEFORE encoding or IO.
	cost, _ := pagePairCostPrivateV3(producerPageHeaderPrivateV3).plus(pageJournalCostPrivateV3)
	cost, _ = cost.plus(checkpointReadCostPrivateV3(scopeMetadataPrivateV2))
	if e := l.state.reserve(cost, left); e != nil {
		return nil, e
	}
	m.LedgerGeneration, m.LedgerTip, m.Charged = l.Tip()
	path, identity, e := l.state.observed.CanonicalIdentityPrivateV2()
	if e != nil {
		return nil, e
	}
	m.LedgerRoot = retainedRootPinPrivateV3{path, identity.Device, identity.Inode}
	m.Allowance = l.state.allowance
	raw, e := json.Marshal(m)
	if e != nil || len(raw) > producerPageHeaderPrivateV3 {
		return nil, ErrInvalid
	}
	if e = roots.check(ctx, f); e != nil {
		return nil, e
	}
	id := Digest(raw)
	if _, e = retainScopePairBytes(ctx, scopeRetentionObject{id, raw}, id, 0, roots.state.outputs[0], roots.state.outputs[1], producerPacketRetentionOpsPrivateV2()); e != nil {
		return nil, e
	}
	if e = publication.CheckProducerPacketObjectsDistinctPrivateV2(ctx, roots.state.outputs, id); e != nil {
		return nil, e
	}
	// Anchor binds the manifest to the private journal namespace, not to history.
	anchor, _ := json.Marshal(struct{ Schema, Checkpoint string }{retainedCheckpointSchemaPrivateV3, id})
	if e = l.state.createNamed(retainedAnchorNamePrivateV3(id), anchor); e != nil {
		return nil, e
	}
	if !validPageDigestPrivateV3(m.LifecycleID) || m.Generation == 0 {
		return nil, ErrInvalid
	}
	if e = l.state.createNamed(retainedLifecycleNamePrivateV3(m.LifecycleID, m.Generation), anchor); e != nil {
		return nil, e
	}
	return raw, ctx.Err()
}

// Admit accepts untrusted manifest bytes plus CURRENT caller grants and a freshly
// validated-current ledger capability. Bootstrap verification precedes source;
// this constructor verifies the COMPLETE reservation chain through the actual
// latest tip, not arbitrary caller numbers. Later charges survive but do not advance
// admitted semantic progress. A torn/missing record refuses. The caller
// loads the bounded manifest; this API charges parsing/cloning/verification here.
// It never accepts a decoded original owner and never constructs one internally.
func AdmitRetainedPageCheckpointPrivateV3(ctx context.Context, raw []byte, f FrozenScopePrivateV2, roots ProducerPacketRootsPrivateV2, current ValidatedCurrentLedgerPrivateV3, d RetainedCheckpointDisclosurePrivateV3, exchange PageCostPrivateV3) (RetainedPageCheckpointOwnerPrivateV3, error) {
	if !current.valid() {
		return RetainedPageCheckpointOwnerPrivateV3{}, ErrInvalid
	}
	l := current.PageLedgerPrivateV3
	if ctx == nil || ctx.Err() != nil || !f.valid() || roots.state == nil || l.state == nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, ErrInvalid
	}
	g, tip, _ := l.Tip()
	if e := l.state.reserve(checkpointReadCostPrivateV3(4*scopeMetadataPrivateV2), &exchange); e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	var m RetainedPageCheckpointManifestPrivateV3
	if canonicalRetainedPrivateV3(raw, &m) != nil || m.Schema != retainedCheckpointSchemaPrivateV3 || m.Authority != 0 || m.Accepted || m.FeatureIdentity != "UNRESOLVED" || m.SourceCompleteness != "UNKNOWN" || m.AbsenceInference != "PROHIBITED" || m.Profile != RetainedCheckpointProfilePrivateV3 || m.PreparationRevision != RetainedCheckpointPreparationRevisionPrivateV3 || m.PreparationRevision != d.PreparationRevision || m.Targets == 0 || m.Targets > 10000 || m.Application != d.Application || m.Application != l.state.application || m.Intent != d.ContinuationIntent || !validPageDigestPrivateV3(m.Intent) || m.Revision.Policy.ConfigSHA256 != d.ConfigSHA256 || m.Allowance != l.state.allowance || m.Generation == 0 || m.Generation == ^uint64(0) || !validPageDigestPrivateV3(m.LifecycleID) || m.Generation == 1 && m.PreviousCheckpoint != "" || m.Generation > 1 && !validPageDigestPrivateV3(m.PreviousCheckpoint) || m.LedgerGeneration == ^uint64(0) || g < m.LedgerGeneration+1 || m.SpecSHA256 != Digest(f.state.specBytes) || m.ScopeSHA256 != Digest(f.state.manifest) || m.Members != uint64(len(f.state.files)) || m.Members == 0 || m.Members > 16 || m.NextBatch < 0 || uint64(m.NextBatch) > m.Revision.BatchDenominator || !m.Revision.Policy.valid() || m.Objects != m.Revision.Cursor.Object || m.Objects == 0 || m.Objects > m.Revision.ObjectDenominator || m.Revision.ObjectDenominator != 7+m.Members+4*m.Revision.BatchDenominator || m.Revision.BatchDenominator == 0 || m.Revision.BatchDenominator > 63 || m.Revision.Cursor.Offset != 0 || !validPageDigestPrivateV3(m.Index) {
		return RetainedPageCheckpointOwnerPrivateV3{}, ErrInvalid
	}
	rr, rb, e := readLedgerRecordPrivateV3(l.state.root, g)
	if e != nil || Digest(rb) != tip || rr.Generation != g {
		return RetainedPageCheckpointOwnerPrivateV3{}, ErrInvalid
	}
	old, ob, e := readLedgerRecordPrivateV3(l.state.root, m.LedgerGeneration)
	if e != nil || Digest(ob) != m.LedgerTip || old.Cumulative != m.Charged || old.Application != m.Application || old.Allowance != m.Allowance {
		return RetainedPageCheckpointOwnerPrivateV3{}, ErrInvalid
	}
	path, identity, e := l.state.observed.CanonicalIdentityPrivateV2()
	if e != nil || m.LedgerRoot != (retainedRootPinPrivateV3{path, identity.Device, identity.Inode}) {
		return RetainedPageCheckpointOwnerPrivateV3{}, ErrInvalid
	}
	if e = verifyRetainedLedgerChainPrivateV3(ctx, l, g, tip, &exchange); e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	id := Digest(raw)
	var anchor struct{ Schema, Checkpoint string }
	if e = l.state.readNamed(retainedAnchorNamePrivateV3(id), &anchor); e != nil || anchor.Schema != m.Schema || anchor.Checkpoint != id {
		return RetainedPageCheckpointOwnerPrivateV3{}, ErrInvalid
	}
	if e = verifyRetainedLifecyclePrivateV3(l, m, id); e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	st := &retainedCheckpointStatePrivateV3{manifest: m, checkpoint: id, frozen: f, roots: roots, ledger: l}
	st.self = st
	if e = st.checkSource(ctx, &exchange); e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	got, e := st.readPair(ctx, id, producerPageHeaderPrivateV3, &exchange)
	if e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	if !bytes.Equal(got, raw) {
		return RetainedPageCheckpointOwnerPrivateV3{}, ErrInvalid
	}
	if e = st.verifyIndexAndPages(ctx, &exchange); e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	if e = st.verifyReports(ctx, &exchange); e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	if m.SelectionSnapshot != "" {
		selected := &retainedSelectedReaderStatePrivateV3{owner: st, ordinal: m.NextBatch, checkpoint: id}
		selected.self = selected
		if e = selected.restore(ctx, m.SelectionSnapshot, &exchange); e != nil {
			return RetainedPageCheckpointOwnerPrivateV3{}, e
		}
		st.savedSelection = selected
	}
	// Create-only exclusive continuation, permanently spent even on later failure.
	if e = l.state.reserve(pageJournalCostPrivateV3, &exchange); e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	claim, _ := json.Marshal(struct {
		Schema, Checkpoint, Intent string
		Generation                 uint64
	}{m.Schema, id, m.Intent, m.Generation})
	if e = l.state.createNamed(retainedClaimNamePrivateV3(id), claim); e != nil {
		return RetainedPageCheckpointOwnerPrivateV3{}, e
	}
	return RetainedPageCheckpointOwnerPrivateV3{st}, nil
}
func (s *retainedCheckpointStatePrivateV3) valid() bool {
	return s != nil && s.self == s && !s.failed && s.frozen.valid() && s.ledger.state != nil
}
func (s *retainedCheckpointStatePrivateV3) checkSource(ctx context.Context, left *PageCostPrivateV3) error {
	if !s.valid() || ctx == nil {
		return ErrInvalid
	}
	// Same conservative policy-max formula, with no original producer constructed.
	cost, e := retainedSourceReservationPrivateV3(s.frozen)
	if e != nil {
		return e
	}
	if e = s.ledger.state.reserve(cost, left); e != nil {
		return e
	}
	if e = s.frozen.revalidate(ctx); e != nil {
		return e
	}
	return s.roots.check(ctx, s.frozen)
}
func (s *retainedCheckpointStatePrivateV3) readPair(ctx context.Context, id string, max uint64, left *PageCostPrivateV3) ([]byte, error) {
	if !validPageDigestPrivateV3(id) || max > 16<<20 {
		return nil, ErrInvalid
	}
	if e := s.ledger.state.reserve(checkpointReadCostPrivateV3(max), left); e != nil {
		return nil, e
	}
	if e := s.roots.check(ctx, s.frozen); e != nil {
		return nil, e
	}
	obs, e := publication.ObserveProducerPacketPairCustodyPrivateV2(ctx, s.roots.state.outputs, id)
	if e != nil {
		return nil, e
	}
	var raw []byte
	for i, r := range s.roots.state.outputs {
		if s.manifest.Roots[i] != (retainedRootPinPrivateV3{obs[i].RootPath, obs[i].RootDevice, obs[i].RootInode}) {
			return nil, ErrInvalid
		}
		b, e := r.ReadProducerPacketPrivateV2(ctx, id, int64(max))
		if e != nil {
			return nil, e
		}
		if Digest(b) != id || raw != nil && !bytes.Equal(raw, b) {
			return nil, ErrInvalid
		}
		raw = b
	}
	return raw, ctx.Err()
}
func retainedRolePrivateV3(object, members uint64) (string, int, int) {
	roles := [...]string{"FROZEN_SPEC", "FROZEN_MANIFEST", "NOMINATION_SELECTION", "FILE_PLAN", "EFFECTIVE_POLICY", "ENUMERATION_ORIGIN", "RUNTIME_METADATA"}
	if object < 7 {
		return roles[object], -1, -1
	}
	if object < 7+members {
		return "OBSERVED_SOURCE_BODY", int(object - 7), -1
	}
	x := object - 7 - members
	return [...]string{"CANONICAL_BATCH_SEEDS", "FULL_CAPTURE_ORIGIN", "SOURCE_ARTIFACT_PRIVATE_V2", "CANONICAL_MANIFEST_INPUT"}[x%4], -1, int(x / 4)
}
func (s *retainedCheckpointStatePrivateV3) descriptor(ctx context.Context, id string, object uint64, left *PageCostPrivateV3) (retainedObjectIndexPrivateV3, error) {
	var v retainedObjectIndexPrivateV3
	raw, e := s.readPair(ctx, id, producerPageHeaderPrivateV3, left)
	if e != nil {
		return v, e
	}
	if canonicalRetainedPrivateV3(raw, &v) != nil || v.Schema != retainedCheckpointSchemaPrivateV3 || v.Binding != s.manifest.Revision.Binding || v.Object != object || !validPageDigestPrivateV3(v.SHA256) || !validPageDigestPrivateV3(v.LastPage) || v.PageCount == 0 || v.PageCount > s.manifest.Revision.Cursor.PageCount {
		return v, ErrInvalid
	}
	role, member, batch := retainedRolePrivateV3(object, s.manifest.Members)
	if v.Role != role || v.Member != member || v.Batch != batch {
		return v, ErrInvalid
	}
	return v, nil
}
func (s *retainedCheckpointStatePrivateV3) findObject(ctx context.Context, object uint64, left *PageCostPrivateV3) (retainedObjectIndexPrivateV3, error) {
	if object >= s.manifest.Objects {
		return retainedObjectIndexPrivateV3{}, ErrInvalid
	}
	id := s.manifest.Index
	for i := uint64(0); i <= object; i++ {
		v, e := s.descriptor(ctx, id, i, left)
		if e != nil {
			return v, e
		}
		if i == object {
			return v, nil
		}
		id = v.Next
	}
	return retainedObjectIndexPrivateV3{}, ErrInvalid
}
func (s *retainedCheckpointStatePrivateV3) verifyIndexAndPages(ctx context.Context, left *PageCostPrivateV3) error {
	m := s.manifest
	r := m.Revision
	if r.Schema != ProducerPagesSchemaPrivateV3 || r.Authority != 0 || r.Accepted || r.Feature != "UNRESOLVED" || r.Source != "UNKNOWN" || r.Absence != "PROHIBITED" || r.Cursor.PageCount == 0 || !validPageDigestPrivateV3(r.Cursor.Last) || r.Generation == 0 || r.Status == "COMPLETE_ORIGIN_PAGES" && r.Cursor.Object != r.ObjectDenominator {
		return ErrInvalid
	}
	if r.Status != "COMPLETE_ORIGIN_PAGES" && r.Status != "PAUSED_BUDGET" && r.Status != "CANCELLED" && r.Status != "FAILED" {
		return ErrInvalid
	}
	// Linked metadata is bounded one entry at a time. There is no all-origin array.
	id := m.Index
	for i := uint64(0); i < m.Objects; i++ {
		v, e := s.descriptor(ctx, id, i, left)
		if e != nil {
			return e
		}
		id = v.Next
	}
	if id != "" {
		return ErrInvalid
	}
	cur := r.Cursor
	id = cur.Last
	for cur.PageCount > 0 {
		raw, e := s.readPair(ctx, id, r.Policy.PayloadBytes+producerPageHeaderPrivateV3+4, left)
		if e != nil {
			return e
		}
		h, b, e := decodePagePrivateV3(raw)
		if e != nil {
			return e
		}
		role, member, batch := retainedRolePrivateV3(h.Object, m.Members)
		if h.Schema != ProducerPagesSchemaPrivateV3 || h.Binding != r.Binding || h.Index != cur.PageCount-1 || h.Object >= m.Objects || h.Role != role || h.Member != member || h.Batch != batch || h.Offset > h.Length || uint64(len(b)) > h.Length-h.Offset || uint64(len(b)) > r.Policy.PayloadBytes || h.Length > 0 && len(b) == 0 || uint64(len(b)) > cur.RawBytes {
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
		cur.PageCount--
		cur.Object = h.Object
		cur.Offset = h.Offset
		cur.RawBytes -= uint64(len(b))
		id = h.Previous
	}
	if id != "" || cur.Object != 0 || cur.Offset != 0 || cur.RawBytes != 0 {
		return ErrInvalid
	}
	return s.checkSource(ctx, left)
}

// SelectedObject reassembles one exact object from its pinned terminal page. It
// never returns partial bytes. One unit is capped by the unchanged explicit
// computation-unit policy. Retry rescans this object, not the whole origin.
func (p RetainedPageCheckpointOwnerPrivateV3) SelectedObject(ctx context.Context, object uint64, exchange PageCostPrivateV3) ([]byte, error) {
	s := p.state
	if !s.valid() {
		return nil, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.selectedObject(ctx, object, &exchange)
}
func (s *retainedCheckpointStatePrivateV3) selectedObject(ctx context.Context, object uint64, left *PageCostPrivateV3) ([]byte, error) {
	if e := s.checkSource(ctx, left); e != nil {
		return nil, e
	}
	d, e := s.findObject(ctx, object, left)
	if e != nil {
		return nil, e
	}
	if d.Length > s.manifest.Revision.Policy.ComputationUnitBytes {
		return nil, ErrPagesUnitPrivateV3
	}
	if e = s.ledger.state.reserve(selectedObjectCostPrivateV3(d.Length), left); e != nil {
		return nil, e
	}
	out := make([]byte, d.Length)
	remaining := d.Length
	id := d.LastPage
	count := d.PageCount
	for {
		raw, e := s.readPair(ctx, id, s.manifest.Revision.Policy.PayloadBytes+producerPageHeaderPrivateV3+4, left)
		if e != nil {
			return nil, e
		}
		h, b, e := decodePagePrivateV3(raw)
		if e != nil {
			return nil, e
		}
		if count == 0 || h.Index != count-1 || h.Schema != ProducerPagesSchemaPrivateV3 || h.Binding != d.Binding || h.Object != object || h.Role != d.Role || h.Member != d.Member || h.Batch != d.Batch || h.Length != d.Length || h.Offset > remaining || uint64(len(b)) != remaining-h.Offset || uint64(len(b)) > s.manifest.Revision.Policy.PayloadBytes || d.Length > 0 && len(b) == 0 {
			return nil, ErrInvalid
		}
		copy(out[h.Offset:remaining], b)
		remaining = h.Offset
		id = h.Previous
		count--
		if remaining == 0 {
			break
		}
	}
	if Digest(out) != d.SHA256 {
		return nil, ErrInvalid
	}
	if e = s.checkSource(ctx, left); e != nil {
		return nil, e
	}
	return out, nil
}
func (s *retainedCheckpointStatePrivateV3) stageCost() PageCostPrivateV3 {
	n := uint64(scopeMetadataPrivateV2) + 4*s.manifest.Revision.Policy.ComputationUnitBytes
	for _, f := range s.frozen.state.files {
		n += uint64(len(f.Bytes))
	}
	return PageCostPrivateV3{LogicalBytes: 256 * n, ReadHashBytes: 256 * n, Work: 1000000}
}
func (b RetainedSelectedBatchPrivateV3) Read(ctx context.Context, exchange PageCostPrivateV3) (RetainedSelectedBatchViewPrivateV3, error) {
	st := b.state
	if st == nil || st.self != st || !st.owner.valid() {
		return RetainedSelectedBatchViewPrivateV3{}, ErrInvalid
	}
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ledger.state.reserve(s.stageCost(), &exchange); e != nil {
		return RetainedSelectedBatchViewPrivateV3{}, e
	}
	if e := s.checkSource(ctx, &exchange); e != nil {
		return RetainedSelectedBatchViewPrivateV3{}, e
	}
	meta, e := s.selectedObject(ctx, 6, &exchange)
	if e != nil {
		return RetainedSelectedBatchViewPrivateV3{}, e
	}
	if Digest(meta) != s.manifest.RuntimeSHA256 {
		return RetainedSelectedBatchViewPrivateV3{}, ErrInvalid
	}
	v := RetainedSelectedBatchViewPrivateV3{Profile: RetainedCheckpointProfilePrivateV3, CheckpointSHA256: st.checkpoint, Binding: s.manifest.Revision.Binding, RevisionSHA256: Digest(mustPageJSONPrivateV3(s.manifest.Revision)), Revision: s.manifest.Revision, FullTargetCount: int(s.manifest.Targets), ScopeSHA256: s.manifest.ScopeSHA256, OriginSHA256: s.manifest.OriginSHA256, RuntimeSHA256: s.manifest.RuntimeSHA256, Ordinal: st.ordinal, FullBatchCount: int(s.manifest.Revision.BatchDenominator), CanonicalSeeds: bytes.Clone(st.raw[0]), CaptureOrigin: bytes.Clone(st.raw[1]), SourceArtifact: bytes.Clone(st.raw[2]), ManifestInput: bytes.Clone(st.raw[3]), RuntimeMetadata: meta, SourceCompleteness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", AbsenceInference: "PROHIBITED"}
	// Parse a detached source under the charged stage, not an alias to sealed state.
	v.Source, e = graphprovenance.ParseSourceWithholdingPrivateV2(v.SourceArtifact)
	v.OriginalNativeV3 = v.Source.Evidence.GraphBytes
	if e != nil {
		return v, e
	}
	for j := range st.raw {
		v.ObjectSHA256[j] = Digest(st.raw[j])
	}
	return v, ctx.Err()
}

// RetainedSelectedBatchBindingPrivateV3 contains assertions, not an owner or a
// reusable evidence capability. Only the sealed batch can revalidate them.
type RetainedSelectedBatchBindingPrivateV3 struct {
	CheckpointSHA256, RuntimeSHA256 string
	ObjectSHA256                    [4]string
}

// RevalidateBindingPrivateV3 preserves Read's three fresh source validations and
// runtime paired-custody read without reparsing or cloning the sealed objects.
// Its bounded byte-work price is policy accounting, not a physical CPU bound.
func (b RetainedSelectedBatchPrivateV3) RevalidateBindingPrivateV3(ctx context.Context, expected RetainedSelectedBatchBindingPrivateV3, exchange PageCostPrivateV3) error {
	if ctx == nil {
		return ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	st := b.state
	if st == nil || st.self != st || st.owner == nil {
		return ErrInvalid
	}
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || !s.manifest.Revision.Policy.valid() || st.checkpoint != s.checkpoint || !validPageDigestPrivateV3(expected.CheckpointSHA256) || !validPageDigestPrivateV3(expected.RuntimeSHA256) || expected.CheckpointSHA256 != st.checkpoint || expected.RuntimeSHA256 != s.manifest.RuntimeSHA256 {
		return ErrInvalid
	}
	for _, id := range expected.ObjectSHA256 {
		if !validPageDigestPrivateV3(id) {
			return ErrInvalid
		}
	}
	// Each selected raw was allocated only after descriptor.Length <= U in
	// RetainedSelectedReaderPrivateV3.Advance. Recheck the sealed lengths here.
	// U <= 16 MiB and exactly four objects imply n <= 64 MiB; all arithmetic
	// below is uint64 and remains below 2^31, except the fixed topology term.
	u := s.manifest.Revision.Policy.ComputationUnitBytes
	var n uint64
	for _, raw := range st.raw {
		if uint64(len(raw)) > u {
			return ErrPagesUnitPrivateV3
		}
		n += uint64(len(raw))
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	// The maximum n also bounds both multiplications; explicitly check the
	// topology addition independently before constructing the reservation.
	if n > 4*(16<<20) || producerPacketTopologyWorkPrivateV2 > ^uint64(0)-n-64 {
		return ErrInvalid
	}
	cost := PageCostPrivateV3{LogicalBytes: 8 * (n + 1), ReadHashBytes: 16 * (n + 1), Work: n + producerPacketTopologyWorkPrivateV2 + 64}
	if e := s.ledger.state.reserve(cost, &exchange); e != nil {
		return e
	}
	if e := s.checkSource(ctx, &exchange); e != nil {
		return e
	}
	meta, e := s.selectedObject(ctx, 6, &exchange)
	if e != nil {
		return e
	}
	if Digest(meta) != s.manifest.RuntimeSHA256 {
		return ErrInvalid
	}
	for j, raw := range st.raw {
		if e := ctx.Err(); e != nil {
			return e
		}
		if Digest(raw) != expected.ObjectSHA256[j] {
			return ErrInvalid
		}
	}
	if st.checkpoint != expected.CheckpointSHA256 || s.checkpoint != expected.CheckpointSHA256 || s.manifest.RuntimeSHA256 != expected.RuntimeSHA256 {
		return ErrInvalid
	}
	return ctx.Err()
}

func (b RetainedSelectedBatchPrivateV3) OwnSource(ctx context.Context, uri string, exchange PageCostPrivateV3) ([]byte, error) {
	st := b.state
	if st == nil || st.self != st || !st.owner.valid() {
		return nil, ErrInvalid
	}
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ledger.state.reserve(s.stageCost(), &exchange); e != nil {
		return nil, e
	}
	for i, f := range s.frozen.state.files {
		if f.URI == uri && f.Role == "NOMINATION" {
			raw, e := s.selectedObject(ctx, 7+uint64(i), &exchange)
			if e != nil {
				return nil, e
			}
			if !bytes.Equal(raw, f.Bytes) {
				return nil, ErrInvalid
			}
			count := 0
			for _, r := range st.source.Evidence.Captures {
				if r.URI == uri {
					if r.Status != "READABLE" || !bytes.Equal(raw, r.Content) {
						return nil, ErrInvalid
					}
					count++
				}
			}
			if count != 1 {
				return nil, ErrInvalid
			}
			return raw, nil
		}
	}
	return nil, ErrInvalid
}

// CheckpointCompletedBatch commits only a complete selected unit. Earlier raw
// checkpoints/pages are immutable. A crash before this point leaves the older
// checkpoint stale against newer reservations and therefore refuses reopening.
func (p RetainedPageCheckpointOwnerPrivateV3) CheckpointCompletedBatch(ctx context.Context, b RetainedSelectedBatchPrivateV3, intent string, exchange PageCostPrivateV3) ([]byte, error) {
	s := p.state
	if !s.valid() || b.state == nil || b.state.self != b.state || b.state.owner != s || !validPageDigestPrivateV3(intent) {
		return nil, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.state.ordinal != s.manifest.NextBatch || s.manifest.Generation == ^uint64(0) {
		return nil, ErrInvalid
	}
	if e := s.checkSource(ctx, &exchange); e != nil {
		return nil, e
	}
	m := s.manifest
	m.Generation++
	m.PreviousCheckpoint = s.checkpoint
	m.NextBatch++
	m.SelectionSnapshot = ""
	m.Intent = intent
	if s.pendingReports != "" {
		m.Reports = s.pendingReports
		m.SemanticStage = "SELECTED_WITH_VERIFIED_OUTPUT_REFERENCES"
	} else {
		m.SemanticStage = "SELECTED_OBSERVATIONS"
	}
	raw, e := writeRetainedCheckpointPrivateV3(ctx, m, s.ledger, s.roots, s.frozen, &exchange)
	if e != nil {
		return nil, e
	}
	if e = canonicalRetainedPrivateV3(raw, &m); e != nil {
		return nil, e
	}
	s.manifest = m
	s.checkpoint = Digest(raw)
	s.pendingReports = ""
	s.savedSelection = nil
	return raw, nil
}
func (p RetainedPageCheckpointOwnerPrivateV3) BudgetSpent() PageCostPrivateV3 {
	if !p.state.valid() {
		return PageCostPrivateV3{}
	}
	_, _, v := p.state.ledger.Tip()
	return v
}
func (p RetainedPageCheckpointOwnerPrivateV3) Profile() string {
	if !p.state.valid() {
		return ""
	}
	return RetainedCheckpointProfilePrivateV3
}
