package sourcecontinuation

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync/atomic"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	inv "lsp-trace/internal/inventorysuccessor"
	rg "lsp-trace/internal/referencegrouping"
	tp "lsp-trace/internal/targetpacket"
)

var ErrPrivateSelectedRowV2 = errors.New("private selected endpoint row refused")

// An explicit selector is an exact original ROW, not a source-span selector.
// RowPosition must be -1 for tuple-only uniqueness, or the original zero-based
// lower intent position for truly identical duplicate tuples. All fields must
// correspond; a row number alone is never a selector or an admission.
type PrivateSelectedEndpointV2 struct {
	RowPosition int
	Slot        cc.PrivateAllModeSlot
	Intent      tp.PrivateSourceIntent
}

type PrivateSelectedResultV2 struct{ state *selectedNativeResultV2 }
type selectedNativeResultV2 struct {
	durableIssued atomic.Bool // one owner-issued durable task, including failed/uncertain issuance
	accounting    inv.CompletedSelectedAccountingV1
	selected      PrivateSelectedEndpointV2
	unit          tp.PrivateSourceUnit
	request       describerequest.Record
	admission     inv.PrivateSourceAdmission
	metadata      cc.PrivateAllModeResult
	sources       PrivateAllModeSourcesResult
}

func (r PrivateSelectedResultV2) Available() bool { return r.state != nil }

// Data-only output; no method can restore or reuse the completed operation.
func (r PrivateSelectedResultV2) Selected() (PrivateSelectedEndpointV2, tp.PrivateSourceUnit, describerequest.Record, bool) {
	if r.state == nil {
		return PrivateSelectedEndpointV2{}, tp.PrivateSourceUnit{}, describerequest.Record{}, false
	}
	return r.state.selected, r.state.unit, r.state.request, true
}

// Fixed local store: no external Store interface/callback, filesystem roots,
// persistence, worker or host. Put/Get budgets also apply on rejecting paths.
type selectedStoreV2 struct {
	memory             *cc.MemoryStore
	cap, total, parent int64
	puts, gets         int
	lower              []byte // exact producer-emitted third material, never a restored authority
	closed             bool
}

func newSelectedStoreV2(cap, parent int64) (*selectedStoreV2, error) {
	if cap <= 0 || cap > 8<<20 || parent <= 0 || parent > 64<<20 {
		return nil, inv.ErrPrivateSelectedBoundsV2
	}
	return &selectedStoreV2{memory: cc.NewMemoryStore(), cap: cap, parent: parent}, nil
}
func (s *selectedStoreV2) MaxObjectBytes() int64 {
	if s == nil || s.closed {
		return 0
	}
	return s.cap
}
func (s *selectedStoreV2) Put(ctx context.Context, b []byte) (string, error) {
	if s == nil || s.closed || ctx == nil || ctx.Err() != nil || s.puts >= 7 || len(b) == 0 || int64(len(b)) > s.cap || int64(len(b)) > 2*s.parent-s.total {
		return "", inv.ErrPrivateSelectedBoundsV2
	}
	s.puts++
	s.total += int64(len(b))
	if s.puts == 3 {
		s.lower = b
	}
	return s.memory.Put(ctx, b)
}
func (s *selectedStoreV2) Get(ctx context.Context, id string) ([]byte, error) {
	if s == nil || s.closed || ctx == nil || ctx.Err() != nil || s.gets >= 7 || len(id) != 71 {
		return nil, inv.ErrPrivateSelectedBoundsV2
	}
	s.gets++
	return s.memory.Get(ctx, id)
}
func (s *selectedStoreV2) close() error {
	if s == nil || s.closed {
		return inv.ErrPrivateSelectedV2
	}
	s.closed = true
	s.lower = nil
	s.memory = nil
	return nil
}

func selectedSelectorBoundV2(s PrivateSelectedEndpointV2) bool {
	if s.RowPosition < -1 || s.RowPosition >= 1000 {
		return false
	}
	i := s.Intent
	for _, v := range []string{s.Slot.Mode, s.Slot.RunID, s.Slot.SeedNamespace, i.NominationID, i.Mode, i.RunID, i.InputID, i.CaseID, i.PolicyID, i.ResultID, i.Lineage, i.BindingState, i.Requirement, i.ClaimCeiling, i.Gap, i.Endpoint.Node, i.Endpoint.Kind, i.Endpoint.Occurrence, i.Endpoint.Role, i.Endpoint.Predecessor, i.Endpoint.Custody, i.Endpoint.Disposition} {
		if len(v) > 4096 {
			return false
		}
	}
	return true
}

func selectedSlotV2(c inv.PrivateSelectedConfigurationV2) cc.PrivateAllModeSlot {
	modes := [7]string{"RD", "C", "CRD", "D", "CD", "R", "CR"}
	for i, m := range modes {
		if m == c.Mode {
			return cc.PrivateAllModeSlot{Ordinal: i, Mode: m, RunID: c.RunID, SeedNamespace: c.SeedNamespace, Seed: c.Seed}
		}
	}
	return cc.PrivateAllModeSlot{Ordinal: -1}
}

// Exact metadata is read only from the original concrete store after whole
// Resume verifies every regenerated material byte. Neither these DTOs nor
// AdmissionID/request equality confer source admission. Actual Matches below is
// still mandatory against Run's retained native admissions.
func selectSelectedRowV2(meta cc.PrivateAllModeResult, source PrivateAllModeSourcesResult, c inv.PrivateSelectedConfigurationV2, selector *PrivateSelectedEndpointV2) (PrivateSelectedEndpointV2, error) {
	return selectSelectedRowCriterionV2(meta, source, c, selector, false)
}

// Closed selection policy only, never source authority. The legacy nil/exact
// entry always passes false; no callback or general selector language exists.
func selectSelectedRowCriterionV2(meta cc.PrivateAllModeResult, source PrivateAllModeSourcesResult, c inv.PrivateSelectedConfigurationV2, selector *PrivateSelectedEndpointV2, dTarget bool) (PrivateSelectedEndpointV2, error) {
	return selectSelectedRowRoleV3(meta, source, c, selector, dTarget, nil)
}
func selectSelectedRowRoleV3(meta cc.PrivateAllModeResult, source PrivateAllModeSourcesResult, c inv.PrivateSelectedConfigurationV2, selector *PrivateSelectedEndpointV2, dTarget bool, role *PrivateSelectedRoleV3) (PrivateSelectedEndpointV2, error) {
	if role != nil && (selector != nil || dTarget || !role.matchesMode(c.Mode)) {
		return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
	}
	if dTarget && (selector != nil || c.Mode != "RD") {
		return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
	}
	slot := selectedSlotV2(c)
	if slot.Ordinal < 0 || len(meta.Intents) == 0 || len(meta.Intents) > 1000 || len(meta.Nominations) > 1000 || len(source.Outcomes) != len(meta.Intents) || meta.Counts.Ready != 1 || meta.Counts.Missing != 6 {
		return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
	}
	expected := map[tp.PrivateSourceIntent]bool{}
	ids := map[string]bool{}
	endpointCount := 0
	for _, n := range meta.Nominations {
		if ids[n.ID] || n.Status != "PROVISIONAL_STRUCTURAL_HYPOTHESIS" || n.ClaimCeiling != rg.EvaluationCeilingPrivate || n.RejectionCondition != "SPLIT_OR_WITHDRAW_IF_ADMITTED_SOURCE_REVIEW_CONTRADICTS_MEMBERSHIP" || n.Gap != "" || n.Mode != c.Mode || n.RunID != c.RunID || n.Lineage != c.Lineage || n.Seed != c.Seed || n.Slot != slot.Ordinal {
			return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
		}
		ids[n.ID] = true
		endpointCount += len(n.Endpoints)
		if endpointCount > 1000 {
			return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
		}
		intents, e := tp.BuildPrivateSourceIntents(n, 1000)
		if e != nil {
			return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
		}
		for _, i := range intents {
			expected[i] = true
		}
	}
	chosen := -1
	for row, i := range meta.Intents {
		if i.Mode != c.Mode || i.RunID != c.RunID || i.Seed != c.Seed || i.Slot != slot.Ordinal || i.Lineage != c.Lineage || i.Gap != "NO_ADMITTED_SOURCE_BINDING" {
			continue
		}
		outcome := source.Outcomes[row]
		if outcome.Intent != row {
			return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
		}
		if outcome.Status != "SOURCE_ADMITTED" || outcome.AdmissionID == "" || outcome.RequestID == "" {
			continue
		}
		if !expected[i] {
			return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
		}
		candidate := PrivateSelectedEndpointV2{RowPosition: row, Slot: slot, Intent: i}
		if role != nil {
			if i.Endpoint.Kind != role.Kind || i.Endpoint.Role != role.Role {
				continue
			}
		} else if dTarget {
			if i.Endpoint.Kind != "RESOLVES_TO_DEFINITION" || i.Endpoint.Role != "DEFINITION_TARGET" {
				continue
			}
		} else if selector != nil {
			if selector.RowPosition < -1 || selector.Slot != slot || selector.Intent != i || selector.RowPosition >= 0 && selector.RowPosition != row {
				continue
			}
		}
		if chosen != -1 {
			return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
		}
		chosen = candidate.RowPosition
	}
	if chosen < 0 {
		return PrivateSelectedEndpointV2{}, ErrPrivateSelectedRowV2
	}
	return PrivateSelectedEndpointV2{RowPosition: chosen, Slot: slot, Intent: meta.Intents[chosen]}, nil
}

// Shared fixed consumer path. Begin consumes the paid S operation; Run creates
// native admissions; whole Resume compares the exact expected handoff, original
// handles, definitions (none in this envelope), checkpoint and every material byte.
// Store cleanup, Observe and exact accounting readback precede result minting.
func PrepareSelectedV2(ctx context.Context, op inv.PrivateSelectedSourceOperationV2, selector *PrivateSelectedEndpointV2) (PrivateSelectedResultV2, error) {
	return prepareSelectedV2(ctx, op, selector, false)
}

// PrepareSelectedDTargetV2 selects a unique actual admitted RD definition target.
// The role is a filter, not authority: native Matches and all readback/Finish
// guards below remain mandatory. A self-definition is not excluded; expected
// target/body and non-query qualification belong to a separately funded driver.
func PrepareSelectedDTargetV2(ctx context.Context, op inv.PrivateSelectedSourceOperationV2) (PrivateSelectedResultV2, error) {
	return prepareSelectedV2(ctx, op, nil, true)
}

func prepareSelectedV2(ctx context.Context, op inv.PrivateSelectedSourceOperationV2, selector *PrivateSelectedEndpointV2, dTarget bool) (PrivateSelectedResultV2, error) {
	return prepareSelectedRoleCriterionV3(ctx, op, selector, dTarget, nil)
}
func prepareSelectedRoleCriterionV3(ctx context.Context, op inv.PrivateSelectedSourceOperationV2, selector *PrivateSelectedEndpointV2, dTarget bool, role *PrivateSelectedRoleV3) (result PrivateSelectedResultV2, err error) {
	if selector != nil {
		copy := *selector
		selector = &copy
	}
	handoff, c, original, objects, scope, err := op.Begin(ctx)
	if err != nil {
		return result, err
	}
	execution, cancel := context.WithDeadline(scope, original.Deadline())
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	if role != nil && (selector != nil || dTarget || !role.matchesMode(c.Mode)) || dTarget && (selector != nil || c.Mode != "RD") || selector != nil && !selectedSelectorBoundV2(*selector) {
		_ = op.Finish(execution, ErrPrivateSelectedRowV2)
		return result, ErrPrivateSelectedRowV2
	}
	limits := c.Contract.CaptureLimits()
	store, err := newSelectedStoreV2(min(int64(limits.MaxArtifactBytes), int64(8<<20)), int64(limits.MaxParentBytes))
	if err != nil {
		_ = op.Finish(execution, err)
		return result, err
	}
	var candidate *selectedNativeResultV2
	defer func() {
		cleanup := store.close()
		err = errors.Join(err, cleanup)
		if execution.Err() != nil || ctx.Err() != nil || !time.Now().Before(original.Deadline()) {
			err = errors.Join(err, inv.ErrPrivateSelectedV2)
		}
		err = op.Finish(execution, err)
		if err == nil && candidate != nil && execution.Err() == nil && ctx.Err() == nil && time.Now().Before(original.Deadline()) {
			var complete bool
			candidate.accounting, complete = op.CompletedAccountingV1()
			if !complete {
				err = inv.ErrPrivateSelectedV2
				return
			}
			result = PrivateSelectedResultV2{candidate}
		} else {
			result = PrivateSelectedResultV2{}
			if err == nil {
				err = inv.ErrPrivateSelectedV2
			}
		}
	}()
	sources := PrivateAllModeSourcesRequest{PolicyID: c.Contract.PacketSourcePolicy().PolicyID, Limits: c.SourceLimits, Deadline: original.Deadline(), Originals: []inv.PrivateProducerOriginals{original}, Objects: objects}
	pause := Run(execution, Request{PrivateSources: &sources, PrivateAllMode: &handoff, Workspace: original.Workspace(), Contract: c.Contract, Store: store, StopAfter: StopAfterPrivateSourceRequests})
	if pause.Err != nil {
		return result, pause.Err
	}
	if pause.PrivateSources == nil {
		return result, ErrPrivateSelectedRowV2
	}
	expected := pause.PrivateSources.Expected
	resumed := Resume(execution, ResumeRequest{PrivateSources: &PrivateAllModeSourcesResume{Version: PrivateAllModeSourcesVersion, Expected: expected, Originals: sources.Originals, Definitions: sources.Definitions, ExpectedHandoff: handoff}, Selector: pause.PrivateSources.CheckpointID, Workspace: original.Workspace(), Store: store, StopAfter: StopAfterPrivateSourceRequests})
	if resumed.Err != nil {
		return result, resumed.Err
	}
	if resumed.PrivateSources == nil || store.puts != 7 || store.gets != 7 || !sameSelectedReadbackV2(*pause.PrivateSources, *resumed.PrivateSources) {
		return result, ErrPrivateSelectedRowV2
	}
	var meta cc.PrivateAllModeResult
	if len(store.lower) == 0 || int64(len(store.lower)) > store.cap || json.Unmarshal(store.lower, &meta) != nil || meta.Version != cc.PrivateAllModeVersion || meta.HandoffID != handoff.ID() || meta.ClaimCeiling != rg.EvaluationCeilingPrivate {
		return result, ErrPrivateSelectedRowV2
	}
	row, err := selectSelectedRowRoleV3(meta, *resumed.PrivateSources, c, selector, dTarget, role)
	if err != nil {
		return result, err
	}
	outcome := resumed.PrivateSources.Outcomes[row.RowPosition]
	var admission inv.PrivateSourceAdmission
	for _, a := range expected.state.admissions {
		if a.Matches(row.Intent) {
			u, e := a.Unit()
			if e == nil && u.ID == outcome.AdmissionID && u.InputID == "" && row.Intent.InputID == original.InputID() {
				admission = a
				break
			}
		}
	}
	if !admission.Valid() || !admission.Matches(row.Intent) {
		return result, ErrPrivateSelectedRowV2
	}
	unit, err := admission.Unit()
	if err != nil {
		return result, err
	}
	var record describerequest.Record
	found := 0
	for _, r := range resumed.PrivateSources.Requests {
		if r.RecordID == outcome.RequestID && r.Envelope.AdmissionID == unit.ID {
			record = r
			found++
		}
	}
	if found != 1 {
		return result, ErrPrivateSelectedRowV2
	}
	candidate = &selectedNativeResultV2{selected: row, unit: unit, request: record, admission: admission, metadata: meta, sources: *resumed.PrivateSources}
	return result, nil
}

// Resume already checks generated material bytes; this independent bounded
// comparison also prevents a returned body or request from being substituted.
func sameSelectedReadbackV2(a, b PrivateAllModeSourcesResult) bool {
	if a.CheckpointID == "" || a.CheckpointID != b.CheckpointID || a.HandoffID != b.HandoffID || a.Version != b.Version || a.Stage != b.Stage || a.ClaimCeiling != b.ClaimCeiling || a.Durability != b.Durability || a.Counts != b.Counts || len(a.Units) != len(b.Units) || len(a.Requests) != len(b.Requests) || len(a.Outcomes) != len(b.Outcomes) {
		return false
	}
	for i, u := range a.Units {
		if u != b.Units[i] {
			return false
		}
	}
	for i, o := range a.Outcomes {
		if o != b.Outcomes[i] {
			return false
		}
	}
	return reflect.DeepEqual(a.Requests, b.Requests)
}
