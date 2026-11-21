//go:build darwin || linux

package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"lsp-trace/internal/scopejob"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

const InventoryNativeHeadVersionV1 = "lsp-trace.durable-inventory-head.v5"
const InventoryNativeConfigVersionV1 = "lsp-trace.inventory-owned-config.v3"
const inventoryNativeProfileVersionV1 = "lsp-trace.inventory-native-boundary-profile.v1"

// Six logical passes of one <=4MiB immutable enumeration segment (parent
// inspection, batch extraction, independent permit validation: read/decode each);
// four <=16KiB head transactions at the existing 32-pass head model; four <=32KiB
// config inspections at the eight-pass model; 32KiB scalar/descriptor bookkeeping.
// Claim and completion are prepaid together. No selected/admission/Seal work.
const InventoryNativeControlBytesV1 uint64 = 6*(4<<20) + 4*32*16384 + 4*8*32768 + 32768

// A profile is a finite declaration, NOT a capability. Only the exact closed
// constructor result is accepted. Coordinates follow countersV2.vector order.
// Child is the unchanged discovery quote; Bootstrap covers additional bounded
// kernel verification and initial publication, not a second charge for Child.
type InventoryNativeProfileV1 struct {
	Schema                       string
	Limits                       sessionruntime.PrivateDiscoveryLimits
	Child, Bootstrap, Allocation [14]uint64
	ControlBytes                 uint64
}

func NewInventoryNativeProfileV1() InventoryNativeProfileV1 {
	l := sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (1<<20 + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}
	now := time.Unix(1, 0)
	h := hashLabelV2("inventory-native-quote-only")
	b := PrivateDiscoveryBindingV2{Revision: "e1b9c3b13ace77cd015ebe0f6c4bfe63d970ba73", PlanIdentity: h, DenominatorDigest: h, MemberDigest: h, Workspace: "/", RuntimeFingerprint: h, PolicyDigest: h, Deadline: now.Add(15 * time.Second), Limits: l}
	child, err := privateDiscoveryChargeV2(b, now)
	if err != nil {
		panic("closed inventory discovery quote invalid")
	}
	// Five additional verify calls: initialize readback, owner open, pending
	// readback, Complete readback, final outcome readback. reserve/observe's two
	// verifications are already inside Child. Each uses the enforced 4096-record,
	// 64MiB-byte, 16384-work verifier bounds. Initialization also probes two heads
	// and publishes authorization + initial + immutable + selected head.
	const meta = recordLimitV2 + 3*headLimitV2
	const verify = 5*(64<<20) + 2*headLimitV2
	bootstrap := countersV2{LogicalBytes: 32 * meta, ReadHashBytes: verify + 32*meta, WriteBytes: 8 * meta, Records: 5*4096 + 4, ParserBytes: verify + 32*meta, Work: 5*16384 + 32*meta, Calls: 5*16384 + 8, OutputBytes: meta}
	total, err := addCountersV2(child, bootstrap)
	if err != nil {
		panic("closed inventory profile overflow")
	}
	return InventoryNativeProfileV1{inventoryNativeProfileVersionV1, l, child.vector(), bootstrap.vector(), total.vector(), InventoryNativeControlBytesV1}
}
func (p InventoryNativeProfileV1) Valid() bool { return p == NewInventoryNativeProfileV1() }

// InventoryNativeClaimV1 is committed accounting data, never execution authority.
type InventoryNativeClaimV1 struct {
	Schema, RunID, ParentHead, ChildNamespace, Policy string
	Ordinal                                           int
	Binding                                           PrivateDiscoveryBindingV2
	Members                                           []sessionruntime.PrivateDiscoveryMember
	Profile                                           InventoryNativeProfileV1
	Allocated                                         [14]uint64
	Outcome, NativeHead                               string
	NativeCharged                                     [14]uint64
}

func (c InventoryNativeClaimV1) Valid() bool {
	if c.Schema == "lsp-trace.inventory-native-claim.v2" {
		if c.Ordinal < 0 || c.Ordinal > 1 || c.Binding.Ordinal != c.Ordinal || c.ChildNamespace != InventoryNamespaceV1("native", c.RunID, c.Ordinal) || (c.Ordinal == 0 && len(c.Members) != 16) || (c.Ordinal == 1 && len(c.Members) != 1) {
			return false
		}
		c.Schema = "lsp-trace.inventory-native-claim.v1"
		c.Ordinal, c.Binding.Ordinal = 0, 0
		c.ChildNamespace = InventoryNamespaceV1("native", c.RunID, 0)
	}
	if c.Schema != "lsp-trace.inventory-native-claim.v1" || len(c.RunID) != 64 || !hashV2("sha256:"+c.RunID) || !hashV2(c.ParentHead) || c.ChildNamespace != "inventory-native-"+c.RunID+"-000000" || c.Ordinal != 0 || c.Binding.Ordinal != 0 || c.Policy == "" || len(c.Policy) > 256 || !c.Profile.Valid() || c.Allocated != c.Profile.Allocation || len(c.Members) == 0 || len(c.Members) > 16 || c.Binding.Limits != c.Profile.Limits || c.Binding.PolicyDigest != "" {
		return false
	}
	b := c.Binding
	b.PolicyDigest = hashLabelV2("shape-only")
	if _, e := privateDiscoveryChargeV2(b, b.Deadline.Add(-15*time.Second)); e != nil {
		return false
	}
	raw, e := json.Marshal(c.Members)
	if e != nil || len(raw) > 8192 || hashLabelV2(string(raw)) != b.MemberDigest {
		return false
	}
	if c.Outcome == "CLAIMED" {
		return c.NativeHead == "" && c.NativeCharged == ([14]uint64{})
	}
	return (c.Outcome == "OBSERVED" || c.Outcome == "FAILED" || c.Outcome == "INDETERMINATE") && hashV2(c.NativeHead) && countersFromV2(c.NativeCharged).within(countersFromV2(c.Profile.Child))
}

type inventoryParentViewV1 struct {
	Schema, RunID, Phase, Status, TaskID string
	Deadline, RunDeadline                int64
	Sequence                             int
	Limit, Used, Work, Packets           uint64
	State                                publication.StoredObject
	Native                               *InventoryNativeClaimV1
	Selected                             *InventorySelectedClaimV1
	Checkpoint                           *InventoryCheckpointV1
	Input                                publication.StoredObject
	OutputBytes                          uint64
}
type inventoryParentEnvelopeV1 struct {
	Digest  string
	Content json.RawMessage
}

func inventoryParentDecodeV1(raw []byte) (inventoryParentViewV1, map[string]json.RawMessage, error) {
	var env inventoryParentEnvelopeV1
	var h inventoryParentViewV1
	var fields map[string]json.RawMessage
	if len(raw) == 0 || len(raw) > 16384 || cat.DecodeNativeV3(raw, &env) != nil || cat.NativeDigestV3(env.Content) != env.Digest || cat.DecodeNativeV3(env.Content, &fields) != nil || json.Unmarshal(env.Content, &h) != nil || (h.Schema != InventoryNativeHeadVersionV1 && h.Schema != InventorySelectedHeadVersionV1 && h.Schema != InventoryResumeHeadVersionV1) {
		return h, nil, ErrPrivateDiscoveryOperationV2
	}
	if h.Schema != InventoryResumeHeadVersionV1 && (h.Checkpoint != nil || h.Native != nil && h.Native.Schema != "lsp-trace.inventory-native-claim.v1" || h.Selected != nil && h.Selected.Schema != "lsp-trace.inventory-selected-claim.v1") {
		return h, nil, ErrPrivateDiscoveryOperationV2
	}
	return h, fields, nil
}

// InventoryNativePermitV1 cannot be built from numbers, stored JSON or reopen.
// Only a fresh successful parent CAS can supply its consumed publication proof.
type InventoryNativePermitV1 struct{ state *inventoryNativePermitV1 }
type inventoryNativePermitV1 struct {
	used     atomic.Bool
	root     *publication.Root
	selector string
	schema   string
	claimed  []byte
	claim    InventoryNativeClaimV1
}

func InventoryNativePermitFromCommitV1(ctx context.Context, proof publication.BoundFileCommit) (InventoryNativePermitV1, error) {
	root, selector, before, after, e := proof.Take(ctx)
	if e != nil {
		return InventoryNativePermitV1{}, e
	}
	old, oldFields, e := inventoryParentDecodeV1(before)
	if e != nil {
		return InventoryNativePermitV1{}, e
	}
	h, fields, e := inventoryParentDecodeV1(after)
	if e != nil || old.Native != nil || old.Phase != "ReadyForAcquisition" || old.Status != "READY" || h.Native == nil || old.Schema != h.Schema || old.Selected != nil || h.Selected != nil || !h.Native.Valid() || h.Native.Outcome != "CLAIMED" || h.RunID != old.RunID || selector != "inventory-run-"+h.RunID+".json" || h.Native.RunID != h.RunID || h.Native.ParentHead != cat.NativeDigestV3(before) || h.Phase != "NativeAcquisition" || h.Status != "CLAIMED" || h.Deadline != h.Native.Binding.Deadline.UnixNano() || !hashV2("sha256:"+h.TaskID) || h.Limit == 0 || h.Limit > inventoryParentLimitV1(h.Schema) || h.Schema == InventorySelectedHeadVersionV1 && h.Limit != inventoryParentLimitV1(h.Schema) || h.Used > h.Limit || h.Work > inventoryParentWorkLimitV1(h.Schema) || h.RunDeadline != 0 || h.Sequence != old.Sequence || h.Native.Ordinal != h.Sequence || h.Sequence != 0 && h.Schema != InventoryResumeHeadVersionV1 || old.Deadline != 0 || old.TaskID != "" || time.Until(h.Native.Binding.Deadline) > 15*time.Second || h.Used != old.Used+InventoryNativeControlForHeadV1(h.Schema) || h.Work != h.Used*32 || h.Packets != old.Packets+1 || !time.Now().Before(h.Native.Binding.Deadline) {
		return InventoryNativePermitV1{}, ErrPrivateDiscoveryOperationV2
	}
	if h.Schema == InventoryResumeHeadVersionV1 {
		if h.Limit != cat.InventoryResumeLimitV1 || h.Native.Schema != "lsp-trace.inventory-native-claim.v2" || h.Sequence < 0 || h.Sequence > 1 {
			return InventoryNativePermitV1{}, ErrPrivateDiscoveryOperationV2
		}
		if h.Sequence == 1 && (old.Checkpoint == nil || old.Input != old.Checkpoint.Admission || old.OutputBytes != old.Checkpoint.OutputBytes || old.Used != old.Checkpoint.Used || VerifyInventoryCheckpointV1(ctx, root, h.RunID, *old.Checkpoint) != nil) {
			return InventoryNativePermitV1{}, ErrPrivateDiscoveryOperationV2
		}
	}
	for _, k := range []string{"Native", "Phase", "Status", "Deadline", "TaskID", "Used", "Work", "Packets"} {
		delete(fields, k)
		delete(oldFields, k)
	}
	if !reflect.DeepEqual(fields, oldFields) {
		return InventoryNativePermitV1{}, ErrPrivateDiscoveryOperationV2
	}
	if !h.State.Valid() || h.State.Bytes > 32768 {
		return InventoryNativePermitV1{}, ErrPrivateDiscoveryOperationV2
	}
	raw, e := publication.ReadObject(root, "inventory-owned-config-", h.State)
	var config struct {
		Schema                string
		NativeProfile         *InventoryNativeProfileV1
		SelectedProfile       *InventorySelectedProfileV1
		ResumeProfile         *InventoryResumeProfileV1
		Enumeration           *scopejob.Config
		Scope, ResourcePolicy string
	}
	if e != nil || json.Unmarshal(raw, &config) != nil || !inventoryNativeConfigVersionMatchesV1(h.Schema, config.Schema, config.SelectedProfile) || !inventoryResumeProfileMatchesV1(h.Schema, config.ResumeProfile) || config.NativeProfile == nil || *config.NativeProfile != h.Native.Profile {
		return InventoryNativePermitV1{}, ErrPrivateDiscoveryOperationV2
	}
	if config.Enumeration == nil || config.Scope != config.Enumeration.Workspace || verifyInventoryNativeEnumerationV1(ctx, root, before, old, *h.Native, *config.Enumeration, config.ResourcePolicy, len(raw)) != nil {
		return InventoryNativePermitV1{}, ErrPrivateDiscoveryOperationV2
	}
	return InventoryNativePermitV1{&inventoryNativePermitV1{root: root, selector: selector, schema: h.Schema, claimed: bytes.Clone(after), claim: *h.Native}}, nil
}

// Verify the frozen metadata independently of the caller that performed CAS.
// The publication proof proves a committed replacement, not domain validity.
func verifyInventoryNativeEnumerationV1(ctx context.Context, root *publication.Root, before []byte, h inventoryParentViewV1, claim InventoryNativeClaimV1, config scopejob.Config, policy string, configBytes int) error {
	canonical, e := scopejob.Canonical(config)
	if e != nil || !reflect.DeepEqual(canonical, config) || config.JobDeadline != nil || config.Budget.Steps > scopejob.BatchSteps || config.Budget.LogicalBytes != min(h.Limit, 64<<20) || configBytes < 1 || configBytes > 32768 {
		return ErrPrivateDiscoveryOperationV2
	}
	_, fields, e := inventoryParentDecodeV1(before)
	if e != nil {
		return e
	}
	var enumeration struct {
		Policy                           string
		Config                           publication.StoredObject
		Claims, Batches                  int
		Steps, PolicyWork, MetadataBytes uint64
		Last                             string
		Closed                           bool
		Reason                           string
	}
	if cat.DecodeNativeV3(fields["Enumeration"], &enumeration) != nil || enumeration.Config != h.State || enumeration.Claims != 1 || enumeration.Batches != 1 || !enumeration.Closed || enumeration.Reason != "" || enumeration.Policy != policy || !hashV2("sha256:"+enumeration.Last) {
		return ErrPrivateDiscoveryOperationV2
	}
	expectedPolicy := "lsp-trace.inventory-enumeration-resource.v1"
	schema := "lsp-trace.inventory-scope-segment.v1"
	if config.Selection != nil {
		expectedPolicy = "lsp-trace.inventory-enumeration-resource.v2"
		schema = "lsp-trace.inventory-scope-segment.v2"
	}
	if policy != expectedPolicy || claim.Policy != policy+"/"+h.State.ID || claim.Binding.Workspace != config.Workspace || claim.Binding.DenominatorDigest != "sha256:"+enumeration.Last || claim.Binding.PlanIdentity != hashLabelV2("inventory-native-plan.v1/"+h.RunID+"/"+enumeration.Last) {
		return ErrPrivateDiscoveryOperationV2
	}
	// Exact versioned creation + first enumeration charges, not a merely positive
	// numeric budget. No historical re-costing or inferred grants are admitted.
	const creationMinimum = 32*(65536+16384) + 32768
	const enumerationFixed = 4 * (scopejob.HeadBytes + scopejob.StopBytes)
	expectedUsed := creationMinimum + 8*uint64(configBytes) + enumerationFixed + scopejob.BatchBytes
	expectedPackets := uint64(4)
	if h.Schema == InventoryResumeHeadVersionV1 && h.Sequence == 1 {
		expectedUsed += InventoryNativeControlForHeadV1(h.Schema) + InventorySelectedControlForHeadV1(h.Schema)
		expectedPackets += 2
	}
	if h.Used != expectedUsed || h.Work != h.Used*32 || h.Packets != expectedPackets {
		return ErrPrivateDiscoveryOperationV2
	}
	raw, e := publication.ReadVerifiedBoundFileCheckedContext(ctx, root, "inventory-scope-"+h.RunID+"/segment-00000000.json", int64(scopejob.SegmentBytes))
	var segment struct {
		Schema, RunID, Previous string
		Index                   int
		Steps                   uint64
		Batch                   scopejob.EnumerationBatch
	}
	if e != nil || cat.NativeDigestV3(raw) != "sha256:"+enumeration.Last || cat.DecodeNativeV3(raw, &segment) != nil || segment.Schema != schema || segment.RunID != h.RunID || segment.Previous != "" || segment.Index != 0 || segment.Steps != min(scopejob.BatchSteps, config.Budget.Steps) || !segment.Batch.Done || segment.Batch.Reason != "" || enumeration.MetadataBytes != uint64(len(raw)) || enumeration.Steps != segment.Steps {
		return ErrPrivateDiscoveryOperationV2
	}
	work, ok := scopejob.EnumerationWork(config, segment.Steps)
	if !ok || work != enumeration.PolicyWork {
		return ErrPrivateDiscoveryOperationV2
	}
	var replay scopejob.EnumerationReplay
	if replay.Apply(ctx, config, segment.Batch, segment.Steps) != nil {
		return ErrPrivateDiscoveryOperationV2
	}
	members := make([]sessionruntime.PrivateDiscoveryMember, 0, 16)
	supported := 0
	for _, record := range segment.Batch.Records {
		if record.Role != "SUPPORTED_UNACQUIRED" {
			continue
		}
		supported++
		if h.Schema == InventoryResumeHeadVersionV1 && supported > 17 {
			return ErrPrivateDiscoveryOperationV2
		}
		if h.Schema != InventoryResumeHeadVersionV1 && len(members) == 16 {
			break
		}
		language := record.LanguageID
		if config.Selection == nil {
			language = config.Language
		}
		if language == "" || !filepath.IsLocal(record.Path) {
			return ErrPrivateDiscoveryOperationV2
		}
		path := filepath.Join(config.Workspace, record.Path)
		members = append(members, sessionruntime.PrivateDiscoveryMember{Path: path, URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: language})
	}
	if h.Schema == InventoryResumeHeadVersionV1 {
		if supported != 17 {
			return ErrPrivateDiscoveryOperationV2
		}
		sort.Slice(members, func(i, j int) bool { return members[i].Path < members[j].Path })
		members = members[claim.Ordinal*16 : min((claim.Ordinal+1)*16, 17)]
	}
	if !reflect.DeepEqual(members, claim.Members) {
		return ErrPrivateDiscoveryOperationV2
	}
	return ctx.Err()
}

// InventoryFundedDiscoveryV1 owns the native child and the already-paid parent
// completion. No public method initializes without consuming the opaque permit.
type InventoryFundedDiscoveryV1 struct{ state *inventoryFundedDiscoveryV1 }
type inventoryFundedDiscoveryV1 struct {
	mu        sync.Mutex
	permit    *inventoryNativePermitV1
	root      *publication.Root
	operation PrivateDiscoveryOperationV2
	// executing protects the native root until the underlying call unwinds.
	// finished claims exclusive finalization, including a failed completion.
	executing, finished, completed bool
	completedParent                []byte // v6-only exact completion predecessor for selected custody
}

func OpenInventoryFundedDiscoveryV1(ctx context.Context, p InventoryNativePermitV1, c sessionruntime.PrivateDiscoveryConstruction) (InventoryFundedDiscoveryV1, error) {
	if p.state == nil || !p.state.used.CompareAndSwap(false, true) {
		return InventoryFundedDiscoveryV1{}, ErrPrivateDiscoveryOperationV2
	}
	s := p.state
	b := s.claim.Binding
	members, runtime := PrivateDiscoveryConstructionIdentityV2(c)
	if ctx == nil || ctx.Err() != nil || !time.Now().Before(b.Deadline) || time.Until(b.Deadline) > 15*time.Second || c.Start.Deadline.After(b.Deadline) || members != b.MemberDigest || runtime != b.RuntimeFingerprint || !c.Deadline.Equal(b.Deadline) || c.Limits != b.Limits || c.Start.Profile.Workspace().String() != b.Workspace {
		return InventoryFundedDiscoveryV1{}, ErrPrivateDiscoveryOperationV2
	}
	ctx, cancel := context.WithDeadline(ctx, b.Deadline)
	defer cancel()
	got, e := publication.ReadVerifiedBoundFileCheckedContext(ctx, s.root, s.selector, int64(len(s.claimed)))
	if e != nil || !bytes.Equal(got, s.claimed) {
		return InventoryFundedDiscoveryV1{}, ErrPrivateDiscoveryOperationV2
	}
	// First effect after the committed claim and current readback. Namespace is
	// fixed by RunID/ordinal, not a caller-selected native store.
	child, e := s.root.OpenPrivateChild(s.claim.ChildNamespace, true)
	if e != nil {
		return InventoryFundedDiscoveryV1{}, e
	}
	fail := func(e error) (InventoryFundedDiscoveryV1, error) {
		return InventoryFundedDiscoveryV1{}, errors.Join(e, child.Close())
	}
	domain := "inventory-parent-native.v1/" + s.claim.RunID + "/" + s.claim.ParentHead
	attempt := hashLabelV2(domain + "/0")
	j := jobV2{Schema: jobSchemaV2, StableID: hashLabelV2(domain), Application: hashLabelV2("inventory-parent-native.v1"), Scope: PrivateDiscoveryScopeDigestV2(), SourceAuthorization: b.DenominatorDigest, RevisionPolicy: hashLabelV2(b.Revision), EvidencePolicies: []string{hashLabelV2(s.claim.Policy)}, ProviderPolicies: []string{hashLabelV2(b.RuntimeFingerprint)}, SemanticPolicies: []string{hashLabelV2("NO_ADMISSION_OR_PUBLICATION")}, Privacy: hashLabelV2("private"), Retention: hashLabelV2("no-replay"), ResourceRules: PrivateDiscoveryResourceRulesV2(b), Origin: "NEW_EMPTY"}
	job, e := sealV2(&j)
	if e != nil {
		return fail(e)
	}
	b.PolicyDigest = PrivateDiscoveryPolicyDigestV2(job)
	now := time.Now()
	charge, e := privateDiscoveryChargeV2(b, now)
	if e != nil || !charge.within(countersFromV2(s.claim.Profile.Child)) {
		return fail(ErrPrivateDiscoveryOperationV2)
	}
	// Authority comes from the consumed/read-verified parent allocation, not an
	// invocation-consent decision. Native accounting is consumption INSIDE it.
	decision := hashLabelV2(string(s.claimed))
	ceiling := countersFromV2(s.claim.Profile.Child)
	a := authorizationV2{Schema: authorizationSchemaV2, Job: j.Identity, Revision: 1, Previous: noneV2, BoundHead: noneV2, Ceiling: ceiling, MaxTranche: ceiling, MaxWindows: 1, MaxWindowNS: ceiling.ActiveNS, Expires: b.Deadline.UnixNano(), Decision: decision, Assurance: assuranceV2}
	auth, e := sealV2(&a)
	if e != nil {
		return fail(e)
	}
	v := PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}
	ready, e := InitializePrivateReadyV2(ctx, child, job, auth, PrivateReadyDecisionAssertionV2{Job: j.Identity, Authorization: a.Identity, Reference: decision}, v)
	if e != nil {
		return fail(e)
	}
	owner, e := OpenPrivateDiscoveryOwnerV2(ctx, child, job, ready.ConfirmedHead(), v)
	if e != nil {
		return fail(e)
	}
	op, e := owner.ReservePartition(ctx, b, attempt)
	if e != nil {
		return fail(e)
	}
	head, e := owner.k.reopen(ctx, owner.job, owner.limits)
	if e != nil || head.Identity != owner.head || head.Disposition != "PENDING" || head.Attempt != attempt || !head.Charged.within(ceiling) {
		return fail(ErrPrivateDiscoveryOperationV2)
	}
	return InventoryFundedDiscoveryV1{&inventoryFundedDiscoveryV1{permit: s, root: child, operation: op}}, nil
}
func (o InventoryFundedDiscoveryV1) Execute(ctx context.Context, c sessionruntime.PrivateDiscoveryConstruction, consume func(context.Context, *sessionruntime.PrivateDiscoverySession) error) error {
	if o.state == nil {
		return ErrPrivateDiscoveryOperationV2
	}
	s := o.state
	s.mu.Lock()
	if s.finished || s.executing {
		s.mu.Unlock()
		return ErrPrivateDiscoveryOperationV2
	}
	s.executing = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.executing = false
		s.mu.Unlock()
	}()
	// The underlying operation retains its one-shot/invalid-input contract.
	// No wrapper lock crosses native work or the caller's synchronous callback.
	return s.operation.Execute(ctx, c, consume)
}
func (o InventoryFundedDiscoveryV1) Complete(ctx context.Context) error {
	if o.state == nil || ctx == nil {
		return ErrPrivateDiscoveryOperationV2
	}
	s := o.state
	s.mu.Lock()
	if s.finished || s.executing {
		s.mu.Unlock()
		return ErrPrivateDiscoveryOperationV2
	}
	// Refuse active work rather than wait (including callback reentry). Only
	// this claimant may complete/close; losing callers cannot consume cleanup.
	s.finished = true
	s.mu.Unlock()
	ctx, cancel := context.WithDeadline(ctx, s.permit.claim.Binding.Deadline)
	defer cancel()
	nativeErr := s.operation.Complete(ctx)
	owner := s.operation.state.owner
	head, e := owner.k.reopen(ctx, owner.job, owner.limits)
	closeErr := s.root.Close()
	if e != nil || closeErr != nil || head.Identity != owner.head || !head.Charged.within(countersFromV2(s.permit.claim.Profile.Child)) {
		return errors.Join(ErrPrivateDiscoveryOperationV2, nativeErr, e, closeErr)
	}
	if head.Disposition != "OBSERVED" && head.Disposition != "FAILED" && head.Disposition != "INDETERMINATE" {
		return ErrPrivateDiscoveryOperationV2
	}
	_, fields, e := inventoryParentDecodeV1(s.permit.claimed)
	if e != nil {
		return e
	}
	claim := s.permit.claim
	claim.Outcome = head.Disposition
	claim.NativeHead = head.Identity
	claim.NativeCharged = head.Charged.vector()
	set := func(k string, v any) { fields[k], _ = json.Marshal(v) }
	set("Native", claim)
	set("Status", "PARTIAL")
	set("TaskID", "")
	set("Deadline", int64(0))
	// Preserve the parent's canonical field order, not map serialization order.
	var env inventoryParentEnvelopeV1
	_ = json.Unmarshal(s.permit.claimed, &env)
	for _, k := range []string{"Native", "Status", "TaskID", "Deadline"} {
		var previous map[string]json.RawMessage
		_ = json.Unmarshal(env.Content, &previous)
		oldField, _ := json.Marshal(k)
		needle := append(append(bytes.Clone(oldField), ':'), previous[k]...)
		replacement := append(append(bytes.Clone(oldField), ':'), fields[k]...)
		env.Content = bytes.Replace(env.Content, needle, replacement, 1)
	}
	env.Digest = cat.NativeDigestV3(env.Content)
	raw, e := json.Marshal(env)
	if e != nil || len(raw) > 16384 {
		return ErrPrivateDiscoveryOperationV2
	}
	pred := publication.BoundFilePredecessor{Selector: s.permit.selector, Digest: cat.NativeDigestV3(s.permit.claimed), ByteLength: uint64(len(s.permit.claimed))}
	r, e := publication.CompareAndReplaceBoundFile(ctx, s.permit.root, s.permit.selector, pred, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return ErrPrivateDiscoveryOperationV2
		}
		return nil
	})
	if e != nil || r == nil || !r.Committed || r.Outcome != publication.CASOutcomeCommitted || r.VerificationStatus != "VERIFIED" || r.DirectorySyncStatus != publication.DirectorySyncComplete || r.CloseStatus != publication.CloseComplete {
		return errors.Join(ErrPrivateDiscoveryOperationV2, e)
	}
	s.mu.Lock()
	s.completed = nativeErr == nil && head.Disposition == "OBSERVED"
	if s.completed && (s.permit.schema == InventorySelectedHeadVersionV1 || s.permit.schema == InventoryResumeHeadVersionV1) {
		s.completedParent = bytes.Clone(raw)
	}
	s.mu.Unlock()
	return nativeErr
}
func (o InventoryFundedDiscoveryV1) OwnedMethods() sessionruntime.PrivateDiscoveryOwnedMethods {
	if o.state == nil {
		return sessionruntime.PrivateDiscoveryOwnedMethods{}
	}
	o.state.mu.Lock()
	completed := o.state.completed
	o.state.mu.Unlock()
	if !completed {
		return sessionruntime.PrivateDiscoveryOwnedMethods{}
	}
	return o.state.operation.OwnedMethods()
}
