//go:build darwin || linux

package applicationinventory

import (
	"context"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
	"lsp-trace/sessionruntime"
)

// NativeBoundaryConfigV1 selects a NEW initial profile. It cannot upgrade an
// existing job or enlarge its 64MiB inventory H budget. Native currencies are a
// separately declared componentwise suballocation, not scalar H or physical IO.
// This profile has one native batch and NO admission/semantic/publication path.
func NativeBoundaryConfigV1(c scopejob.Config) (OwnedInventoryConfig, error) {
	// This new boundary profile funds one enumeration segment, not arbitrary
	// traversal followed by an unfunded native attempt. Legacy profiles unchanged.
	c.Budget.Steps = min(c.Budget.Steps, scopejob.BatchSteps)
	out, e := EnumerationConfig(c)
	if e != nil {
		return out, e
	}
	p := is.NewInventoryNativeProfileV1()
	out.Schema = is.InventoryNativeConfigVersionV1
	out.NativeProfile = &p
	return out, nil
}
func ownedNativeHeadValid(h durableInventoryHead) bool {
	if h.Schema != is.InventoryNativeHeadVersionV1 {
		return false
	}
	if h.Phase == "Creating" {
		return ownedCreationValid(h)
	}
	if h.Native != nil {
		n := h.Native
		if !n.Valid() || n.RunID != h.RunID || h.Phase != "NativeAcquisition" || h.Enumeration == nil || !h.Enumeration.Closed || h.Enumeration.Batches != 1 || h.Sequence != 0 {
			return false
		}
		if n.Outcome == "CLAIMED" {
			if h.Status != "CLAIMED" || !validDurableID(h.TaskID) || h.Deadline != n.Binding.Deadline.UnixNano() {
				return false
			}
		} else if h.Status != "PARTIAL" || h.TaskID != "" || h.Deadline != 0 {
			return false
		}
		q, e := ownedCreationQuote(h.State.Bytes)
		if e != nil || h.Used != q+enumerationFixed+scopejob.BatchBytes+is.InventoryNativeControlBytesV1 || h.Packets != 5 {
			return false
		}
		h.Native = nil
		h.Phase = "Collecting"
		h.Status = "READY"
		h.TaskID = ""
		h.Deadline = 0
	} else if h.Phase == "NativeAcquisition" {
		return false
	}
	h.Schema = ownedHeadVersion
	return ownedHeadValid(h)
}

// InventoryNativeBatchV1 is bounded verified metadata, never source identity or
// authority. Counts describe the whole paid enumeration, not merely this batch.
type InventoryNativeBatchV1 struct {
	RunID, Head, Workspace, PlanIdentity, DenominatorDigest, Policy string
	Ordinal                                                         int
	Members                                                         []sessionruntime.PrivateDiscoveryMember
	Counts                                                          scopejob.Counts
	UnknownRemainder                                                bool
}

func (s DurableStore) NativeDiscoveryBatchV1(ctx context.Context, run, expectedHead string) (InventoryNativeBatchV1, error) {
	if ctx == nil {
		return InventoryNativeBatchV1{}, ErrDurableInventory
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	h, raw, e := s.headContext(ctx, run)
	if e != nil || (h.Schema != is.InventoryNativeHeadVersionV1 && h.Schema != is.InventorySelectedHeadVersionV1 && !is.InventoryResumableHeadV1(h.Schema)) || h.Status != "READY" || h.Phase != "ReadyForAcquisition" || h.Native != nil || cat.NativeDigestV3(raw) != expectedHead || h.Enumeration == nil || h.Enumeration.Batches != 1 {
		return InventoryNativeBatchV1{}, ErrDurableInventory
	}
	if is.InventoryResumableHeadV1(h.Schema) && s.verifyResumeCheckpointV1(ctx, h) != nil {
		return InventoryNativeBatchV1{}, ErrDurableInventory
	}
	return s.nativeBatchAt(ctx, h, expectedHead)
}
func (s DurableStore) nativeBatchAt(ctx context.Context, h durableInventoryHead, head string) (InventoryNativeBatchV1, error) {
	status, e := s.inspectEnumeration(ctx, h)
	if e != nil || !status.Closed || h.Enumeration.Batches != 1 {
		return InventoryNativeBatchV1{}, ErrDurableInventory
	}
	config, e := s.enumerationConfig(ctx, h)
	if e != nil || config.NativeProfile == nil || !config.NativeProfile.Valid() {
		return InventoryNativeBatchV1{}, ErrDurableInventory
	}
	raw, e := publication.ReadVerifiedBoundFileCheckedContext(ctx, s.Root, enumerationName(h.RunID, 0), int64(scopejob.SegmentBytes))
	var segment inventoryScopeSegment
	if e != nil || strictInventory(raw, &segment) != nil || cat.NativeDigestV3(raw) != "sha256:"+h.Enumeration.Last {
		return InventoryNativeBatchV1{}, ErrDurableInventory
	}
	batch := InventoryNativeBatchV1{RunID: h.RunID, Head: head, Workspace: config.Scope, PlanIdentity: cat.NativeDigestV3([]byte("inventory-native-plan.v1/" + h.RunID + "/" + h.Enumeration.Last)), DenominatorDigest: "sha256:" + h.Enumeration.Last, Policy: h.Enumeration.Policy + "/" + h.State.ID, Counts: status.Counts, UnknownRemainder: status.UnknownRemainder}
	if is.InventoryResumableHeadV1(h.Schema) {
		batch.Ordinal = h.Sequence
	}
	supported := 0
	for _, r := range segment.Batch.Records {
		if r.Role != "SUPPORTED_UNACQUIRED" {
			continue
		}
		supported++
		if is.InventoryResumableHeadV1(h.Schema) && supported > 17 {
			return InventoryNativeBatchV1{}, ErrDurableInventory
		}
		if !is.InventoryResumableHeadV1(h.Schema) && len(batch.Members) == 16 {
			break
		}
		language := r.LanguageID
		if config.Enumeration.Selection == nil {
			language = config.Enumeration.Language
		}
		if language == "" || !filepath.IsLocal(r.Path) {
			return InventoryNativeBatchV1{}, ErrDurableInventory
		}
		path := filepath.Join(config.Scope, r.Path)
		batch.Members = append(batch.Members, sessionruntime.PrivateDiscoveryMember{Path: path, URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: language})
	}
	if is.InventoryResumableHeadV1(h.Schema) {
		if supported != 17 {
			return InventoryNativeBatchV1{}, ErrDurableInventory
		}
		sort.Slice(batch.Members, func(i, j int) bool { return batch.Members[i].Path < batch.Members[j].Path })
		batch.Members = batch.Members[batch.Ordinal*16 : min((batch.Ordinal+1)*16, 17)]
	}
	if len(batch.Members) == 0 {
		return InventoryNativeBatchV1{}, ErrDurableInventory
	}
	return batch, nil
}

// ReserveNativeDiscoveryV1 is the only inventory transition yielding the opaque
// permit. Exact predecessor CAS serializes competitors; it never retries/rebases.
// No source/native/Starter effects occur here. Metadata is checked against the
// frozen enumeration selection before the charge, not trusted from construction.
func (s DurableStore) ReserveNativeDiscoveryV1(ctx context.Context, run, expectedHead string, c sessionruntime.PrivateDiscoveryConstruction) (is.InventoryNativePermitV1, error) {
	bad := func() (is.InventoryNativePermitV1, error) { return is.InventoryNativePermitV1{}, ErrDurableInventory }
	if ctx == nil || ctx.Err() != nil {
		return bad()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	h, old, e := s.headContext(ctx, run)
	if e != nil || (h.Schema != is.InventoryNativeHeadVersionV1 && h.Schema != is.InventorySelectedHeadVersionV1 && !is.InventoryResumableHeadV1(h.Schema)) || h.Phase != "ReadyForAcquisition" || h.Status != "READY" || h.Native != nil || cat.NativeDigestV3(old) != expectedHead || h.Enumeration == nil || h.Enumeration.Batches != 1 {
		return bad()
	}
	if is.InventoryResumableHeadV1(h.Schema) && s.verifyResumeCheckpointV1(ctx, h) != nil {
		return bad()
	}
	batch, e := s.nativeBatchAt(ctx, h, expectedHead)
	if e != nil {
		return bad()
	}
	config, e := s.enumerationConfig(ctx, h)
	if e != nil {
		return bad()
	}
	members, runtime := is.PrivateDiscoveryConstructionIdentityV2(c)
	if !reflect.DeepEqual(c.Members, batch.Members) || c.Start.Profile.Workspace().String() != batch.Workspace || c.Limits != config.NativeProfile.Limits || members == "" || runtime == "" || !time.Now().Before(c.Deadline) || time.Until(c.Deadline) > 15*time.Second || c.Start.Deadline.After(c.Deadline) || is.InventoryNativeControlForHeadV1(h.Schema) > h.Limit-h.Used || h.Packets >= 64 {
		return bad()
	}
	binding := is.PrivateDiscoveryBindingV2{Revision: "e1b9c3b13ace77cd015ebe0f6c4bfe63d970ba73", PlanIdentity: batch.PlanIdentity, DenominatorDigest: batch.DenominatorDigest, MemberDigest: members, Workspace: batch.Workspace, RuntimeFingerprint: runtime, Ordinal: 0, Deadline: c.Deadline, Limits: c.Limits}
	claim := &is.InventoryNativeClaimV1{Schema: "lsp-trace.inventory-native-claim.v1", RunID: run, ParentHead: expectedHead, ChildNamespace: "inventory-native-" + run + "-000000", Policy: batch.Policy, Ordinal: 0, Binding: binding, Members: append([]sessionruntime.PrivateDiscoveryMember(nil), batch.Members...), Profile: *config.NativeProfile, Allocated: config.NativeProfile.Allocation, Outcome: "CLAIMED"}
	if is.InventoryResumableHeadV1(h.Schema) {
		claim.Schema = "lsp-trace.inventory-native-claim.v2"
		claim.Ordinal, claim.Binding.Ordinal = batch.Ordinal, batch.Ordinal
		claim.ChildNamespace = is.InventoryNamespaceV1("native", run, batch.Ordinal)
	}
	if !claim.Valid() {
		return bad()
	}
	h.Native = claim
	h.Phase = "NativeAcquisition"
	h.Status = "CLAIMED"
	h.Deadline = c.Deadline.UnixNano()
	h.TaskID, e = newDurableID()
	if e != nil {
		return bad()
	}
	h.Used += is.InventoryNativeControlForHeadV1(h.Schema)
	h.Work = h.Used * 32
	h.Packets++
	if !h.completionValid() {
		return bad()
	}
	raw, e := encodeInventoryHead(h)
	if e != nil || len(raw) > inventoryHeadLimit(h) {
		return bad()
	}
	// Decode before commit as well; replacement encoding must preserve bindings.
	var round durableInventoryHead
	if decodeInventoryHead(raw, &round) != nil || !ownedHeadValid(round) {
		return bad()
	}
	ctx, deadlineCancel := context.WithDeadline(ctx, c.Deadline)
	defer deadlineCancel()
	proof, e := publication.CommitBoundFile(ctx, s.Root, inventoryHeadName(run), old, raw)
	if e != nil {
		return is.InventoryNativePermitV1{}, e
	}
	return is.InventoryNativePermitFromCommitV1(ctx, proof)
}
