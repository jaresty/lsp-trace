//go:build darwin || linux

package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	rg "lsp-trace/internal/referencegrouping"
	"reflect"
	"sync"
	"time"
)

const InventorySelectedHeadVersionV1 = "lsp-trace.durable-inventory-head.v6"
const InventorySelectedConfigVersionV1 = "lsp-trace.inventory-owned-config.v4"

func inventoryParentLimitV1(schema string) uint64 {
	if InventoryReferenceHead(schema) {
		return cat.InventoryReferenceLimitV1
	}
	if schema == InventoryConnectedHeadVersionV1 {
		return cat.InventoryConnectedLimitV1
	}
	if InventoryResumableHeadV1(schema) {
		return cat.InventoryResumeLimitV1
	}
	if schema == InventorySelectedHeadVersionV1 {
		return cat.InventoryAdmissionLimitV4
	}
	return 64 << 20
}
func inventoryNativeConfigVersionMatchesV1(head, config string, p *InventorySelectedProfileV1) bool {
	if head == InventorySemanticHeadVersionV1 {
		return config == InventorySemanticConfigVersionV1 && p != nil && p.Valid()
	}
	if head == InventoryReferenceHeadVersionV1 {
		return config == InventoryReferenceConfigVersionV1 && p != nil && p.Valid()
	}
	if head == InventoryConnectedHeadVersionV1 {
		return config == InventoryConnectedConfigVersionV1 && p != nil && p.Valid()
	}
	if InventoryResumableHeadV1(head) {
		return config == InventoryResumeConfigVersionV1 && p != nil && p.Valid()
	}
	if head == InventorySelectedHeadVersionV1 {
		return config == InventorySelectedConfigVersionV1 && p != nil && p.Valid()
	}
	return config == InventoryNativeConfigVersionV1 && p == nil
}

type InventorySelectedProfileV1 struct {
	Schema                                    string
	Child, Bootstrap, Allocation              [14]uint64
	ControlBytes, AdmissionBytes, ParentLimit uint64
}

func NewInventorySelectedProfileV1() InventorySelectedProfileV1 {
	now := time.Unix(1, 0)
	end := now.Add(15 * time.Second)
	m, e := privateRetainedMixedQuoteV1(now, end)
	if e != nil {
		panic(e)
	}
	p, e := selectedPreparationChargeV2(rg.AllModeMetadataPrivate{Input: rg.CompositionCommitment{ByteLength: 1 << 20}, Contributions: 1024, Custody: 2048, Predecessors: 1024}, now, end)
	if e != nil {
		panic(e)
	}
	s, e := selectedSourceChargeV2(1024, 64<<20, 64<<20, now, end)
	if e != nil {
		panic(e)
	}
	total, e := addCountersV2(m, p)
	if e != nil {
		panic(e)
	}
	total, e = addCountersV2(total, s)
	if e != nil {
		panic(e)
	}
	// Three extra verifier waves: initialization readback, owner open, final
	// wrapper readback. M/P/S already include their reserve/execute/observe waves.
	// Two initial-head probes and four metadata publications, as in discovery.
	const meta = recordLimitV2 + 3*headLimitV2
	const verify = 3*(64<<20) + 2*headLimitV2
	bootstrap := countersV2{LogicalBytes: 32 * meta, ReadHashBytes: verify + 32*meta, WriteBytes: 8 * meta, Records: 3*4096 + 4, ParserBytes: verify + 32*meta, Work: 3*16384 + 32*meta, Calls: 3*16384 + 8, OutputBytes: meta}
	allocation, e := addCountersV2(total, bootstrap)
	if e != nil {
		panic(e)
	}
	return InventorySelectedProfileV1{"lsp-trace.inventory-selected-profile.v1", total.vector(), bootstrap.vector(), allocation.vector(), cat.InventorySelectedControlBytesV1, cat.InventoryAdmissionBytesV4, cat.InventoryAdmissionLimitV4}
}
func (p InventorySelectedProfileV1) Valid() bool { return p == NewInventorySelectedProfileV1() }

type InventorySelectedClaimV1 struct {
	Schema, RunID, ParentHead, Namespace, Configuration string
	Index                                               int
	Deadline                                            int64
	Profile                                             InventorySelectedProfileV1
	Allocated                                           [14]uint64
	Outcome                                             string
	Accounting                                          CompletedSelectedAccountingV1
	Ordinal                                             int `json:",omitempty"`
}

func (c InventorySelectedClaimV1) Valid() bool {
	if c.Schema == "lsp-trace.inventory-selected-claim.v2" {
		if c.Ordinal < 0 || c.Ordinal > 1 || c.Namespace != InventoryNamespaceV1("selected", c.RunID, c.Ordinal) {
			return false
		}
		c.Schema = "lsp-trace.inventory-selected-claim.v1"
		c.Ordinal = 0
		c.Namespace = InventoryNamespaceV1("selected", c.RunID, 0)
	}
	if c.Ordinal != 0 {
		return false
	}
	if c.Schema != "lsp-trace.inventory-selected-claim.v1" || !hashV2("sha256:"+c.RunID) || !hashV2(c.ParentHead) || !hashV2(c.Configuration) || c.Namespace != "inventory-selected-"+c.RunID+"-000000" || c.Index < 0 || c.Index >= 128 || c.Deadline <= 0 || !c.Profile.Valid() || c.Allocated != c.Profile.Allocation {
		return false
	}
	if c.Outcome == "CLAIMED" {
		return c.Accounting == (CompletedSelectedAccountingV1{})
	}
	a := c.Accounting
	return c.Outcome == "OBSERVED" && a.Disposition == "OBSERVED" && a.Event == "OBSERVE" && a.Windows == 3 && a.NativeDeadline == c.Deadline && hashV2(a.Head) && countersFromV2(a.Charged).within(countersFromV2(c.Profile.Child))
}
func InventorySelectedConfigurationDigestV1(c PrivateSelectedConfigurationV2) (string, error) {
	if !selectedConfigValidV2(c) || c.Mode != "R" {
		return "", ErrPrivateSelectedV2
	}
	c, e := selectedCheckedContractV2(c)
	if e != nil {
		return "", e
	}
	raw, e := json.Marshal(c)
	if e != nil || len(raw) > 32768 {
		return "", ErrPrivateSelectedV2
	}
	return hashLabelV2(string(raw)), nil
}

// Private funding injection: only OpenInventoryFundedSelectedV1 can construct
// this value after consuming a current parent publication proof.
type InventoryFundedSelectedV1 struct{ state *inventoryFundedSelectedV1 }
type inventoryFundedSelectedV1 struct {
	mu                        sync.Mutex
	executing, used, finished bool
	root                      *publication.Root
	owner                     *PrivateManagedInputOwnerV2
	request                   PrivateMixedInputRequestV2
	config                    PrivateSelectedConfigurationV2
	claim                     InventorySelectedClaimV1
	source                    PrivateSelectedSourceOperationV2
}

// The native discovery handle never leaves its original wrapper. A serialized
// claim is insufficient: fresh committed CAS and that completed owner are needed.
func OpenInventoryFundedSelectedV1(ctx context.Context, proof publication.BoundFileCommit, discovery InventoryFundedDiscoveryV1, config PrivateSelectedConfigurationV2) (InventoryFundedSelectedV1, error) {
	bad := func() (InventoryFundedSelectedV1, error) { return InventoryFundedSelectedV1{}, ErrPrivateSelectedV2 }
	root, selector, before, after, e := proof.Take(ctx)
	if e != nil {
		return bad()
	}
	old, oldFields, e := inventoryParentDecodeV1(before)
	if e != nil {
		return bad()
	}
	h, fields, e := inventoryParentDecodeV1(after)
	if e != nil || (h.Schema != InventorySelectedHeadVersionV1 && !InventoryResumableHeadV1(h.Schema)) || old.Schema != h.Schema || old.Native == nil || !old.Native.Valid() || old.Native.Outcome != "OBSERVED" || old.Selected != nil || h.Selected == nil || !h.Selected.Valid() || h.Selected.Outcome != "CLAIMED" || h.Selected.RunID != h.RunID || h.Selected.ParentHead != cat.NativeDigestV3(before) || selector != "inventory-run-"+h.RunID+".json" || old.Phase != "NativeAcquisition" || old.Status != "PARTIAL" || h.Phase != "SelectedAdmission" || h.Status != "CLAIMED" || h.Limit != inventoryParentLimitV1(h.Schema) || h.Used != old.Used+InventorySelectedControlForHeadV1(h.Schema) || h.Used > h.Limit || h.Work != h.Used*32 || h.Packets != old.Packets+1 || h.Deadline != h.Selected.Deadline || h.Deadline != old.Native.Binding.Deadline.UnixNano() || !hashV2("sha256:"+h.TaskID) || ctx == nil || ctx.Err() != nil || !time.Now().Before(old.Native.Binding.Deadline) {
		return bad()
	}
	if InventoryResumableHeadV1(h.Schema) && (h.Selected.Schema != "lsp-trace.inventory-selected-claim.v2" || h.Selected.Ordinal != old.Native.Ordinal || h.Selected.Ordinal != h.Sequence) {
		return bad()
	}
	for _, k := range []string{"Selected", "Phase", "Status", "Deadline", "TaskID", "Used", "Work", "Packets"} {
		delete(fields, k)
		delete(oldFields, k)
	}
	if !reflect.DeepEqual(fields, oldFields) || discovery.state == nil {
		return bad()
	}
	d := discovery.state
	d.mu.Lock()
	completed := d.completed
	parent := d.completedParent // immutable after successful finalization
	d.mu.Unlock()
	if !completed || !bytes.Equal(before, parent) || root != d.permit.root || h.RunID != d.permit.claim.RunID || old.Native.ParentHead != d.permit.claim.ParentHead || !reflect.DeepEqual(old.Native.Binding, d.permit.claim.Binding) || old.Native.NativeHead != d.operation.state.owner.head {
		return bad()
	}
	digest, e := InventorySelectedConfigurationDigestV1(config)
	if e != nil || digest != h.Selected.Configuration || config.ExpectedSourceAuthorization != old.Native.Binding.DenominatorDigest {
		return bad()
	}
	config, e = selectedCheckedContractV2(config)
	if e != nil {
		return bad()
	}
	if !h.State.Valid() || h.State.Bytes > 32768 {
		return bad()
	}
	raw, e := publication.ReadObject(root, "inventory-owned-config-", h.State)
	var c struct {
		Schema          string
		SelectedProfile *InventorySelectedProfileV1
		ResumeProfile   *InventoryResumeProfileV1
	}
	if e != nil || json.Unmarshal(raw, &c) != nil || !inventoryNativeConfigVersionMatchesV1(h.Schema, c.Schema, c.SelectedProfile) || !inventoryResumeProfileMatchesV1(h.Schema, c.ResumeProfile) || c.SelectedProfile == nil || *c.SelectedProfile != h.Selected.Profile {
		return bad()
	}
	current, e := publication.ReadVerifiedBoundFileCheckedContext(ctx, root, selector, int64(len(after)))
	if e != nil || !bytes.Equal(current, after) {
		return bad()
	}
	child, e := root.OpenPrivateChild(h.Selected.Namespace, true)
	if e != nil {
		return InventoryFundedSelectedV1{}, e
	}
	owner, q, e := openLocalFolderSelectedV1(ctx, child, d.operation, h.Selected.Index, PrivateRetainedNativeCallsV1{}, config, &inventorySelectedFundingV1{cat.NativeDigestV3(after), countersFromV2(h.Selected.Profile.Child)})
	if e != nil {
		return InventoryFundedSelectedV1{}, errors.Join(e, child.Close())
	}
	return InventoryFundedSelectedV1{&inventoryFundedSelectedV1{root: child, owner: owner, request: q, config: config, claim: *h.Selected}}, nil
}

// Execute keeps root ownership across the synchronous source adapter. No lock
// crosses the adapter; callback reentry into Complete refuses without closing.
func (o InventoryFundedSelectedV1) Execute(ctx context.Context, consume func(context.Context, PrivateSelectedSourceOperationV2) error) error {
	if o.state == nil || ctx == nil || consume == nil {
		return ErrPrivateSelectedV2
	}
	s := o.state
	s.mu.Lock()
	if s.used || s.finished {
		s.mu.Unlock()
		return ErrPrivateSelectedV2
	}
	s.used = true
	s.executing = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.executing = false; s.mu.Unlock() }()
	source, e := s.prepare(ctx)
	if e != nil {
		return e
	}
	return consume(ctx, source)
}
func (s *inventoryFundedSelectedV1) prepare(ctx context.Context) (PrivateSelectedSourceOperationV2, error) {
	plan, e := NewPrivateSelectedPreparationPlanV2(s.owner, s.request, s.config)
	if e != nil {
		return PrivateSelectedSourceOperationV2{}, e
	}
	m, e := s.owner.ReserveSelectedMixedInputV2(ctx, s.config.Attempts[0], s.request, plan)
	if e != nil {
		return PrivateSelectedSourceOperationV2{}, e
	}
	captured, e := ExecuteSelectedMixedInputV2(ctx, m, s.request, plan)
	if e != nil {
		return PrivateSelectedSourceOperationV2{}, e
	}
	p, e := s.owner.ReserveSelectedPreparationV2(ctx, captured)
	if e != nil {
		return PrivateSelectedSourceOperationV2{}, e
	}
	prepared, e := ExecuteSelectedPreparationV2(ctx, p)
	if e != nil {
		return PrivateSelectedSourceOperationV2{}, e
	}
	source, e := s.owner.ReserveSelectedSourceV2(ctx, prepared)
	s.mu.Lock()
	s.source = source
	s.mu.Unlock()
	return source, e
}

// Complete closes before releasing accounting. A failed readback or close never
// becomes an admission permission. Cancellation/uncertainty remains spent.
func (o InventoryFundedSelectedV1) Complete(ctx context.Context) (CompletedSelectedAccountingV1, error) {
	if o.state == nil {
		return CompletedSelectedAccountingV1{}, ErrPrivateSelectedV2
	}
	s := o.state
	s.mu.Lock()
	if s.finished || s.executing {
		s.mu.Unlock()
		return CompletedSelectedAccountingV1{}, ErrPrivateSelectedV2
	}
	s.finished = true
	source := s.source
	s.mu.Unlock()
	a, ok := source.CompletedAccountingV1()
	if ctx == nil {
		return CompletedSelectedAccountingV1{}, errors.Join(ErrPrivateSelectedV2, s.root.Close())
	}
	ctx, cancel := context.WithDeadline(ctx, time.Unix(0, s.claim.Deadline))
	defer cancel()
	head, e := s.owner.k.reopen(ctx, s.owner.job, s.owner.limits)
	closeErr := s.root.Close()
	if !ok || e != nil || closeErr != nil || ctx.Err() != nil || head.Identity != a.Head || head.Disposition != "OBSERVED" || head.Windows != 3 || head.Charged.vector() != a.Charged || !head.Charged.within(countersFromV2(s.claim.Profile.Child)) {
		return CompletedSelectedAccountingV1{}, errors.Join(ErrPrivateSelectedV2, e, closeErr)
	}
	return a, nil
}
