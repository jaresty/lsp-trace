//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	dw "lsp-trace/internal/describeworker"
	is "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

const durableRunLimit uint64 = 64 << 20
const durableStepLimit uint64 = 8 << 20

var ErrDurableInventory = errors.New("durable inventory unavailable, stale, expired, corrupt or spent")

// Directory and Root are application-owned configuration, never request paths.
type DurableStore struct {
	Root      *publication.Root
	Directory string
}
type durableInventoryState struct {
	Schema, RunID, Interpreter, Reviewer string
	Policy                               sc.DurableDescribePolicyV1
	Report                               Report
	Cursor                               int
	Prepared, Descriptions               []publication.StoredObject
	PreparedAdmissions                   []publication.StoredObject `json:",omitempty"`
	Base, Proposed, Corrected            publication.StoredObject
	ManifestCost                         uint64
}
type durableInventoryHead struct {
	Schema, RunID, TaskID, PreviousTaskID, Phase, Status string
	Sequence                                             int
	State, Input, Report, Manifest                       publication.StoredObject
	InputPrefix, Role, Actor, Prompt                     string
	Issued, RunDeadline, Deadline, Window, SemanticNS    int64
	Used, Work, Packets, Limit, OutputBytes              uint64
	Acceptance                                           string
	AcceptanceDeadline                                   int64
	Completion                                           durableCompletionPlan
	CompletionCursor                                     int
	Delivery                                             inventoryDelivery
	DeliverySpent                                        uint64
	SuccessorReserve, SuccessorPackets                   uint64
	Creation                                             *ownedCreationIntent         `json:",omitempty"`
	Enumeration                                          *ownedEnumeration            `json:",omitempty"`
	Native                                               *is.InventoryNativeClaimV1   `json:",omitempty"`
	Selected                                             *is.InventorySelectedClaimV1 `json:",omitempty"`
	Checkpoint                                           *is.InventoryCheckpointV1    `json:",omitempty"`
	Closure                                              *connectedClosureV1          `json:",omitempty"`
}
type DurableTask struct {
	Contract                                                                                TaskContract
	Schema, RunID, TaskID, PreviousTaskID, Phase, Status, Role, Actor, Prompt, PromptSHA256 string
	Sequence                                                                                int
	DeadlineUnixNano, RunDeadlineUnixNano                                                   int64
	Used, Work, Packets                                                                     uint64
	SemanticReservedNS                                                                      int64
	RequiredInputNextOffset                                                                 int
	InputIndexBytes                                                                         int
	Artifacts                                                                               []sc.DurableDescribeArtifactV1
	Authority                                                                               int
	Accepted                                                                                bool
	Completeness                                                                            string
}

func newDurableID() (string, error) {
	var b [32]byte
	_, e := rand.Read(b[:])
	return hex.EncodeToString(b[:]), e
}
func validDurableID(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func inventoryHeadName(run string) string { return "inventory-run-" + run + ".json" }
func strictInventory(raw []byte, v any) error {
	if cat.DecodeNativeV3(raw, v) != nil {
		return ErrDurableInventory
	}
	canonical, e := json.Marshal(v)
	if e != nil || !bytes.Equal(canonical, raw) {
		return ErrDurableInventory
	}
	return nil
}

type inventoryHeadEnvelope struct {
	Digest  string
	Content json.RawMessage
}

func encodeInventoryHead(h durableInventoryHead) ([]byte, error) {
	content, e := json.Marshal(h)
	if e != nil {
		return nil, e
	}
	return json.Marshal(inventoryHeadEnvelope{cat.NativeDigestV3(content), content})
}
func decodeInventoryHead(raw []byte, h *durableInventoryHead) error {
	var envelope inventoryHeadEnvelope
	if strictInventory(raw, &envelope) != nil || cat.NativeDigestV3(envelope.Content) != envelope.Digest {
		return ErrDurableInventory
	}
	return decodeVersionedInventoryHead(envelope.Content, h)
}
func (s DurableStore) head(run string) (durableInventoryHead, []byte, error) {
	return s.headContext(context.Background(), run)
}
func (s DurableStore) headContext(ctx context.Context, run string) (durableInventoryHead, []byte, error) {
	var h durableInventoryHead
	if !validDurableID(run) {
		return h, nil, ErrDurableInventory
	}
	raw, e := publication.ReadVerifiedBoundFileCheckedContext(ctx, s.Root, inventoryHeadName(run), ownedBootstrapMaxBytes)
	if e != nil {
		return h, nil, ErrDurableInventory
	}
	if e = decodeInventoryHead(raw, &h); e != nil {
		return durableInventoryHead{}, nil, e
	}
	if len(raw) > inventoryHeadLimit(h) || h.Phase != "Creating" && h.Creation != nil {
		return h, nil, ErrDurableInventory
	}
	if ownedPhase(h) {
		if h.RunID != run || !ownedHeadValid(h) {
			return h, nil, ErrDurableInventory
		}
		return h, raw, nil
	}
	if (h.Schema != durableHeadVersion && !h.independentTaskLeases()) || h.RunID != run || !validDurableID(h.TaskID) || h.Sequence < 1 || h.Sequence > 102 || !h.State.Valid() || !h.Input.Valid() || h.Limit == 0 || h.Limit > h.runLimit() || h.Used > h.Limit || h.OutputBytes > 32<<20 || h.Phase == "Published" && (!h.Manifest.Valid() || h.Manifest.Bytes > 65536) || h.Work > h.workLimit() || h.Packets > 64 || h.Window <= 0 || h.Window > int64(30*time.Second) || h.SemanticNS < 0 || h.SemanticNS > int64(120*time.Second) || !validInventoryDeadlines(h) {
		return h, nil, ErrDurableInventory
	}
	if h.Status != "READY" && h.Status != "CLAIMED" {
		return h, nil, ErrDurableInventory
	}
	switch h.Phase {
	case "Describe":
		if h.InputPrefix != "inventory-evidence-" || h.Role != "interpreter" {
			return h, nil, ErrDurableInventory
		}
	case "GroupDecision", "Reviewer", "Published":
		if h.InputPrefix != "inventory-catalog-" {
			return h, nil, ErrDurableInventory
		}
	default:
		return h, nil, ErrDurableInventory
	}
	if !h.completionValid() || h.Enumeration != nil && !enumerationHeadValid(h) {
		return h, nil, ErrDurableInventory
	}
	return h, raw, nil
}

// Usage reservations deliberately do not change this semantic acceptance binding.
func inventoryAcceptance(h durableInventoryHead) []byte {
	h.Used, h.Work, h.Packets = 0, 0, 0
	h.CompletionCursor = 0
	h.Delivery.Next, h.DeliverySpent = 0, 0
	h.Status, h.Acceptance = "", ""
	raw, _ := json.Marshal(h)
	return raw
}
func (s DurableStore) accepted(h durableInventoryHead) error {
	if h.Acceptance == "" {
		return ErrDurableInventory
	}
	return publication.VerifyTimedCompletion(s.Root, h.Acceptance, inventoryAcceptance(h), time.Unix(0, h.AcceptanceDeadline))
}
func (s DurableStore) change(ctx context.Context, old []byte, h durableInventoryHead) ([]byte, error) {
	if !h.completionValid() {
		return nil, ErrDurableInventory
	}
	raw, e := encodeInventoryHead(h)
	if e != nil {
		return nil, e
	}
	if (h.independentTaskLeases() || h.Schema == is.InventoryNativeHeadVersionV1 || h.Schema == is.InventorySelectedHeadVersionV1 || is.InventoryResumableHeadV1(h.Schema)) && (len(raw) > inventoryHeadLimit(h) || ctx == nil || ctx.Err() != nil) {
		return nil, ErrDurableInventory
	}
	pred := publication.BoundFilePredecessor{Absent: old == nil, Selector: inventoryHeadName(h.RunID), Digest: cat.NativeDigestV3(old), ByteLength: uint64(len(old))}
	r, e := publication.CompareAndReplaceBoundFile(ctx, s.Root, inventoryHeadName(h.RunID), pred, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return ErrDurableInventory
		}
		return nil
	})
	if e != nil || r == nil || !r.Committed || r.Outcome != publication.CASOutcomeCommitted || r.VerificationStatus != "VERIFIED" || r.DirectorySyncStatus != publication.DirectorySyncComplete || r.CloseStatus != publication.CloseComplete || ctx.Err() != nil {
		return nil, errors.Join(ErrDurableInventory, e)
	}
	return raw, nil
}
func (s DurableStore) reserve(ctx context.Context, h durableInventoryHead, old []byte, n uint64, claim, required bool) (durableInventoryHead, []byte, error) {
	protected, packets, valid := h.protectedCapacity()
	if !valid || !h.completionValid() {
		return h, nil, ErrDurableInventory
	}
	if required {
		if h.CompletionCursor >= len(h.Completion.Stages) || n != h.Completion.Stages[h.CompletionCursor] {
			return h, nil, ErrDurableInventory
		}
	} else if n > h.Limit-h.Used-protected || h.Packets+packets >= 64 || n > (h.workLimit()-h.Work-protected*32)/32 {
		return h, nil, ErrDurableInventory
	}
	if ctx == nil || ctx.Err() != nil || h.Status != "READY" || s.accepted(h) != nil || n > h.Limit-h.Used || h.Packets >= 64 || n > (h.workLimit()-h.Work)/32 {
		return h, nil, ErrDurableInventory
	}
	if h.Phase != "Published" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.Unix(0, h.operationDeadline()))
		defer cancel()
	}
	if required {
		h.CompletionCursor++
	}
	h.Used += n
	h.Work += n * 32
	h.Packets++
	if claim {
		h.Status = "CLAIMED"
	}
	raw, e := s.change(ctx, old, h)
	return h, raw, e
}
func (s DurableStore) state(ctx context.Context, h durableInventoryHead, b *sc.CallerBudgetV3) (durableInventoryState, error) {
	var v durableInventoryState
	if b.Reserve(ctx, uint64(h.State.Bytes)*8+32768) != nil {
		return v, ErrDurableInventory
	}
	raw, e := publication.ReadObject(s.Root, "inventory-state-", h.State)
	if e != nil || strictInventory(raw, &v) != nil || (h.independentTaskLeases() && v.Schema != "lsp-trace.inventory-state.v2" || !h.independentTaskLeases() && v.Schema != "lsp-trace.inventory-state.v1") || v.RunID != h.RunID || len(v.Prepared) != len(v.Report.Census) || len(v.Report.Dispositions) != len(v.Prepared) || v.Cursor < 0 || v.Cursor > len(v.Prepared) || len(v.Descriptions) > v.Cursor || v.Report.Authority != 0 || v.Report.Accepted || v.Report.Completeness != "UNKNOWN" {
		return v, ErrDurableInventory
	}
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		if len(v.PreparedAdmissions) != 2 {
			return v, ErrDurableInventory
		}
	} else if len(v.PreparedAdmissions) != 0 {
		return v, ErrDurableInventory
	}
	return v, nil
}
func (s DurableStore) saveState(ctx context.Context, v durableInventoryState, b *sc.CallerBudgetV3) (publication.StoredObject, error) {
	raw, e := json.Marshal(v)
	if e != nil || b.Reserve(ctx, uint64(len(raw))*8+32768) != nil {
		return publication.StoredObject{}, ErrDurableInventory
	}
	return publication.StoreObject(s.Root, "inventory-state-", raw)
}
func (s DurableStore) descriptions(ctx context.Context, v durableInventoryState, b *sc.CallerBudgetV3) ([]sc.BoundSelectedDescriptionV3, error) {
	// Accepted state owns the ordered description references. Validate its
	// structural index before reads; reopening is not a historical preparation audit.
	n := len(v.Report.Census)
	if len(v.RunID) != 64 || n > 10000 || len(v.Prepared) != n || len(v.Report.Dispositions) != n || v.Cursor < 0 || v.Cursor > n || len(v.Descriptions) > 100 || v.Report.SuccessfulDescriptions != len(v.Descriptions) || b.Reserve(ctx, uint64(n)*1024+4096) != nil {
		return nil, ErrDurableInventory
	}
	if !validDurableID(v.RunID) {
		return nil, ErrDurableInventory
	}
	seen := make(map[string]bool, n)
	count := 0
	for i, disposition := range v.Report.Dispositions {
		lead := v.Report.Census[i].ID
		if len(lead) == 0 || len(lead) > 256 || seen[lead] || disposition.LeadID != lead {
			return nil, ErrDurableInventory
		}
		seen[lead] = true
		switch disposition.Status {
		case "DESCRIBED_UNMATCHED", "DESCRIBED_WITH_MEMBERSHIP":
			if i >= v.Cursor || !v.Prepared[i].Valid() {
				return nil, ErrDurableInventory
			}
			count++
		case "PREPARED":
			if i < v.Cursor || !v.Prepared[i].Valid() {
				return nil, ErrDurableInventory
			}
		case "UNTOUCHED", "OBSERVED_EMPTY", "UNAVAILABLE", "PREPARATION_FAILED":
			if v.Prepared[i] != (publication.StoredObject{}) {
				return nil, ErrDurableInventory
			}
		default:
			return nil, ErrDurableInventory
		}
	}
	if count != len(v.Descriptions) || v.Cursor < n && v.Report.Dispositions[v.Cursor].Status != "PREPARED" {
		return nil, ErrDurableInventory
	}
	out := make([]sc.BoundSelectedDescriptionV3, 0, count)
	for _, disposition := range v.Report.Dispositions {
		if disposition.Status != "DESCRIBED_UNMATCHED" && disposition.Status != "DESCRIBED_WITH_MEMBERSHIP" {
			continue
		}
		var d sc.BoundSelectedDescriptionV3
		var e error
		if len(v.PreparedAdmissions) == 2 {
			i := len(out)
			d, e = sc.ReopenInventoryReferenceDescriptionV1(ctx, s.Root, v.Descriptions[i], b, v.RunID, disposition.LeadID, v.PreparedAdmissions[i], v.Prepared[i])
		} else {
			d, e = sc.ReopenInventoryDescriptionForRunV1(ctx, s.Root, v.Descriptions[len(out)], b, v.RunID, disposition.LeadID)
		}
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, nil
}
func (s DurableStore) base(ctx context.Context, v *durableInventoryState, b *sc.CallerBudgetV3) (publication.StoredObject, error) {
	ds, e := s.descriptions(ctx, *v, b)
	if e != nil {
		return publication.StoredObject{}, e
	}
	native := make([]cat.NativeBoundDescriptionV3, len(ds))
	for i := range ds {
		native[i] = ds[i]
	}
	c, e := cat.BuildNativeV3(ctx, b, v.Interpreter, native, v.Policy.ReviewMode)
	recordInventoryStage(&v.Report, "catalog", e == nil)
	if e != nil {
		return publication.StoredObject{}, e
	}
	raw, e := c.Bytes(ctx)
	if e != nil {
		return publication.StoredObject{}, e
	}
	return publication.StoreObject(s.Root, "inventory-catalog-", raw)
}
func nextPrepared(v *durableInventoryState) {
	for v.Cursor < len(v.Prepared) && !v.Prepared[v.Cursor].Valid() {
		v.Cursor++
	}
}
func (s DurableStore) issueNext(h *durableInventoryHead, v durableInventoryState) error {
	id, e := newDurableID()
	if e != nil {
		return e
	}
	h.PreviousTaskID = h.TaskID
	h.TaskID = id
	h.Sequence++
	h.Status = "READY"
	if h.Phase != "Published" {
		if h.SemanticNS > int64(120*time.Second)-h.Window {
			return ErrDurableInventory
		}
		h.SemanticNS += h.Window
		h.Deadline = time.Now().Add(time.Duration(h.Window)).UnixNano()
		if !h.independentTaskLeases() {
			h.Deadline = min(h.Deadline, h.RunDeadline)
		}
		if h.Deadline <= time.Now().UnixNano() {
			return ErrDurableInventory
		}
	}
	h.Actor = v.Interpreter
	h.Role = "interpreter"
	h.Prompt = v.Policy.RolePrompts[0]
	if h.Phase == "Reviewer" {
		h.Actor = v.Reviewer
		h.Role = "reviewer"
		h.Prompt = v.Policy.RolePrompts[1]
	}
	h.InputPrefix = "inventory-catalog-"
	switch h.Phase {
	case "Describe":
		h.Input = v.Prepared[v.Cursor]
		h.InputPrefix = "inventory-evidence-"
	case "GroupDecision":
		h.Input = v.Base
	case "Reviewer":
		h.Input = v.Proposed
	case "Published":
		h.Input = v.Corrected
	default:
		return ErrDurableInventory
	}
	return nil
}

// Start prepares every admitted original before returning. It never calls the
// one-item Issue loop, and creates exactly one persistent accounting ledger.
func (s DurableStore) Start(ctx context.Context, q Request, policy sc.DurableDescribePolicyV1, runDeadline time.Time) (DurableTask, error) {
	bad := func(e error) (DurableTask, error) { return DurableTask{}, e }
	if ctx == nil || q.Preparer == nil || !q.ReviewMode.Valid() || q.Interpreter == "" || q.ReviewerIdentity == "" || policy.ReviewMode != q.ReviewMode || policy.Reviewer != q.ReviewerIdentity || (!q.ReviewMode.AllowsSameActor() && q.Interpreter == q.ReviewerIdentity) || !filepath.IsAbs(s.Directory) || time.Until(runDeadline) <= 0 || time.Until(runDeadline) > 180*time.Second {
		return bad(ErrDurableInventory)
	}
	if q.BudgetBytes == 0 {
		q.BudgetBytes = durableRunLimit
	}
	b, e := sc.NewCallerBudgetV3(runDeadline, q.BudgetBytes)
	if e != nil {
		return bad(e)
	}
	out := initialInventoryReport()
	out.ReviewMode = q.ReviewMode
	out, n, e := admitInventoryManifest(ctx, q, b, out)
	if e != nil {
		return bad(e)
	}
	run, e := newDurableID()
	if e != nil {
		return bad(e)
	}
	v := durableInventoryState{Schema: "lsp-trace.inventory-state.v1", RunID: run, Interpreter: q.Interpreter, Reviewer: q.ReviewerIdentity, Policy: policy, Report: out, Prepared: make([]publication.StoredObject, len(out.Census)), Descriptions: []publication.StoredObject{}, ManifestCost: n}
	if q.ResponseLimits == (dw.SuppliedResponseLimits{}) {
		q.ResponseLimits = dw.SuppliedResponseLimits{MaxAdapterBytes: 64 << 10, MaxSemanticBytes: 32 << 10}
	}
	if b.Reserve(ctx, uint64(len(v.Prepared))*8+4096) != nil {
		return bad(ErrDurableInventory)
	}
	sealPayloads := make([]uint64, len(v.Prepared))
	for i, l := range out.Census {
		if i >= q.MaxNativeAttempts {
			break
		}
		if b.Reserve(ctx, 4096) != nil {
			return bad(ErrDurableInventory)
		}
		v.Report.Dispositions[i] = Disposition{l.ID, "PREPARATION_FAILED", "CALLER_PREPARER_REFUSED"}
		native, e := q.Preparer(ctx, l)
		if e != nil || ctx.Err() != nil {
			recordInventoryStage(&v.Report, "native_preparation", false)
			continue
		}
		if native.Status == "OBSERVED_EMPTY" || native.Status == "UNAVAILABLE" {
			recordInventoryStage(&v.Report, "native_preparation", true)
			v.Report.Dispositions[i] = Disposition{l.ID, native.Status, "NATIVE_PREPARER_REPORTED_NOT_WHOLE_REPOSITORY"}
			continue
		}
		if native.Status != "SELECTED" || !native.Selected.Available() {
			recordInventoryStage(&v.Report, "native_preparation", false)
			continue
		}
		retained, e := native.Selected.CallerRetentionUpperV3(ctx, b)
		// PrepareSelectedDescribeV3 reserves this retained upper in its existing
		// caller budget below. It is a policy check here, not a second charge.
		if e != nil || retained > q.NativeRetainedUpper {
			return bad(ErrDurableInventory)
		}
		_, _, request, _ := native.Selected.Selected()
		pins := q.Pins
		pins.PromptSHA256 = "sha256:" + request.Envelope.InputSHA256
		binding := sc.CallerDescribeBindingV3{AttemptID: l.ID, Interpreter: q.Interpreter, Pins: pins, Deadline: time.Now().Add(min(30*time.Second, time.Until(runDeadline))), Limits: q.ResponseLimits, Budget: b}
		ref, payload, e := sc.RetainInventoryEvidence(ctx, s.Root, run, l.ID, native.Selected, binding)
		if e != nil {
			return bad(e)
		}
		v.Prepared[i] = ref
		sealPayloads[i] = payload
		recordInventoryStage(&v.Report, "native_preparation", true)
		v.Report.Dispositions[i] = Disposition{l.ID, "PREPARED", "DESCRIBE_PENDING"}
	}
	nextPrepared(&v)
	window := q.HostWindow
	if window == 0 {
		window = 30 * time.Second
	}
	window = min(window, 30*time.Second, 120*time.Second/time.Duration(len(v.Prepared)+2))
	h := durableInventoryHead{Schema: durableHeadVersion, RunID: run, Issued: time.Now().UTC().UnixNano(), RunDeadline: runDeadline.UnixNano(), Window: int64(window), Phase: "Describe"}
	if v.Cursor == len(v.Prepared) {
		h.Phase = "GroupDecision"
		v.Base, e = s.base(ctx, &v, b)
		if e != nil {
			return bad(e)
		}
	}
	h.State, e = s.saveState(ctx, v, b)
	if e != nil {
		return bad(e)
	}
	// Charge bounded plan construction before traversing the owned reference
	// collection or allocating its stage arrays.
	if b.Reserve(ctx, durablePlanningQuote(len(v.Prepared))) != nil {
		return bad(ErrDurableInventory)
	}
	if e = s.issueNext(&h, v); e != nil {
		return bad(e)
	}
	if e = s.buildDelivery(ctx, &h, b); e != nil {
		return bad(e)
	}
	h.Limit = q.BudgetBytes
	h.Used, _ = b.Usage()
	h.Work = h.Used * 32
	h.Completion, e = quoteDurableCompletion(v.Prepared, h.State.Bytes, v.ManifestCost, q.ResponseLimits, sealPayloads)
	h.Completion.PreparationUsed = h.Used
	if e == nil {
		h.SuccessorReserve, h.SuccessorPackets, e = successorEscrow(h)
	}
	if e != nil {
		return bad(e)
	}
	if e = h.startCapacityRefusal(); e != nil {
		return bad(e)
	}
	h.Acceptance = "inventory-accepted-" + run + "-" + h.TaskID
	h.AcceptanceDeadline = h.Deadline
	if _, e = s.change(ctx, nil, h); e != nil {
		return bad(e)
	}
	if e = publication.PublishTimedCompletion(ctx, s.Root, h.Acceptance, inventoryAcceptance(h), time.Unix(0, h.AcceptanceDeadline)); e != nil {
		return bad(e)
	}
	return s.Task(ctx, run, "")
}
func (s DurableStore) Task(ctx context.Context, run, task string) (DurableTask, error) {
	h, _, e := s.head(run)
	if e != nil {
		return DurableTask{}, e
	}
	if ctx == nil || ctx.Err() != nil || ownedPhase(h) || task != "" && task != h.TaskID {
		return DurableTask{}, ErrDurableInventory
	}
	status := "PENDING"
	if h.Status == "CLAIMED" || s.accepted(h) != nil {
		status = "UNCERTAIN_SPENT"
	} else if h.Phase == "Published" {
		status = "PUBLISHED"
	} else if time.Now().UnixNano() >= h.operationDeadline() {
		status = "EXPIRED"
	}
	t := DurableTask{Schema: "lsp-trace.durable-inventory-task.v1", RunID: run, TaskID: h.TaskID, PreviousTaskID: h.PreviousTaskID, Phase: h.Phase, Status: status, Role: h.Role, Actor: h.Actor, Prompt: h.Prompt, PromptSHA256: cat.NativeDigestV3([]byte(h.Prompt)), Sequence: h.Sequence, DeadlineUnixNano: h.Deadline, RunDeadlineUnixNano: h.RunDeadline, Used: h.Used, Work: h.Work, Packets: h.Packets, SemanticReservedNS: h.SemanticNS, Artifacts: []sc.DurableDescribeArtifactV1{{ID: "input", Bytes: h.Input.Bytes}}, Completeness: "UNKNOWN"}
	t.RequiredInputNextOffset = h.Delivery.Next
	t.InputIndexBytes = h.Delivery.Index.Bytes
	if h.independentTaskLeases() {
		t.Schema = "lsp-trace.durable-inventory-task.v2"
	}
	t.Contract = taskContract(t)
	if h.independentTaskLeases() && h.Phase == "Describe" {
		t.Contract.MaxSubmissionBytes = h.Completion.Limits.MaxAdapterBytes
		t.Contract.MaxSemanticBytes = h.Completion.Limits.MaxSemanticBytes
		t.Contract.Rules += " For this configured job, MaxSubmissionBytes and MaxSemanticBytes are the binding ceilings even where the general grammar describes larger absolute maxima."
	}
	t.Contract.RunByteLimit = h.Limit
	if is.InventoryConnectedHead(h.Schema) {
		t.Contract.Operations = nil // Slice 1 is API-only; do not advertise CLI dispatch.
		t.Contract.Rules = strings.ReplaceAll(t.Contract.Rules, "RunByteLimit is the stored run byte ceiling (at most 67108864);", "RunByteLimit is the stored initial connected-profile ceiling;")
		t.Contract.Rules = strings.TrimPrefix(t.Contract.Rules, "Operations are argv arrays, not shell strings. ")
		t.Contract.Rules = "API-only connected profile: call DurableStore.Task, Read, Submit and Result with this RunID and current TaskID. No CLI operation is activated. RunByteLimit is the stored cumulative profile limit, not the historical 64MiB enclosure. Distinct configured actors are declarations, not authenticated independent judgment. " + t.Contract.Rules
	}
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		t.Contract.Rules = strings.ReplaceAll(t.Contract.Rules, "$.Request.Envelope.Prompt", "$.Request.envelope.prompt")
		t.Contract.Rules += " Original R observations are at $.OriginalReferences for Describe and $.evidence[*].OriginalReferences for grouping/review; the latter also carries $.evidence[*].InventoryAdmission with actual admission/evidence/witness digests. QUERY_TO_REPORTED_REFERENCE_LOCATION is query/result direction, not CALLS or feature membership. Target source is NOT_CAPTURED_BY_THIS_WITNESS; provider/global completeness remains unknown. Co-presence does not establish an edge or a feature. Read all supplied evidence and preserve these qualifications."
	}
	return t, nil
}

// durablePageQuote uses only validated scalar lengths before the persistent
// reservation or read. The 8 MiB stored-object bound makes both products and the
// offset+count sum representable even on 32-bit hosts. Verification still reads
// the complete object; only the returned page-copy term shrinks at the tail.
func durablePageQuote(total, offset, limit int) (uint64, int, error) {
	if total < 1 || total > 8<<20 || offset < 0 || offset >= total || limit < 1 || limit > 32<<10 {
		return 0, 0, ErrDurableInventory
	}
	count := min(limit, total-offset)
	return uint64(total)*6 + uint64(count)*2 + 16384, count, nil
}

func (s DurableStore) Read(ctx context.Context, run, task, artifact string, offset, limit int) (sc.DurableDescribePageV1, error) {
	h, old, e := s.head(run)
	if e != nil {
		return sc.DurableDescribePageV1{}, e
	}
	if ownedPhase(h) || task != h.TaskID || artifact != "input" || offset < 0 || offset >= h.Input.Bytes || limit < 1 || limit > 32<<10 {
		return sc.DurableDescribePageV1{}, ErrDurableInventory
	}
	if h.Delivery.Index.Valid() {
		return s.readChunks(ctx, h, old, offset, limit)
	}
	quote, count, e := durablePageQuote(h.Input.Bytes, offset, limit)
	if e != nil {
		return sc.DurableDescribePageV1{}, e
	}
	h, _, e = s.reserve(ctx, h, old, quote, false, false)
	if e != nil {
		return sc.DurableDescribePageV1{}, e
	}
	raw, e := publication.ReadObject(s.Root, h.InputPrefix, h.Input)
	if e != nil || ctx.Err() != nil || h.Phase != "Published" && time.Now().UnixNano() >= h.Deadline {
		return sc.DurableDescribePageV1{}, ErrDurableInventory
	}
	return sc.DurableDescribePageV1{RunID: run, TaskID: task, ArtifactID: artifact, Offset: offset, Total: len(raw), Data: append([]byte(nil), raw[offset:offset+count]...)}, nil
}
func (s DurableStore) Submit(ctx context.Context, run, task, role string, answer []byte) (DurableTask, error) {
	bad := func(e error) (DurableTask, error) { return DurableTask{}, e }
	h, old, e := s.head(run)
	if e != nil {
		return bad(e)
	}
	if ctx == nil || ownedPhase(h) || task != h.TaskID || role != h.Role || h.Phase == "Published" {
		return bad(ErrDurableInventory)
	}
	ctx, cancel := context.WithDeadline(ctx, time.Unix(0, h.operationDeadline()))
	defer cancel()
	// Reserve a bounded stage slice in the single ledger before decoding stored
	// state or judgments. Actual work cannot exceed this already-spent slice.
	h, claimed, allocation, e := s.claimSubmit(ctx, h, old, len(answer))
	if e != nil {
		return bad(e)
	}
	b, e := h.sliceBudget(time.Unix(0, h.budgetDeadline()), allocation, h.Used-allocation, time.Duration(h.SemanticNS))
	if e != nil {
		return bad(e)
	}
	// Account inside the already-funded outer slice, not a second run charge.
	if b.Reserve(ctx, durablePlanningQuote(len(h.Completion.Inputs))) != nil {
		return bad(ErrDurableInventory)
	}
	v, e := s.state(ctx, h, b)
	if e != nil {
		return bad(e)
	}
	if len(answer) > 64<<10 {
		return bad(ErrDurableInventory)
	}
	switch h.Phase {
	case "Describe":
		if v.Cursor >= len(v.Prepared) || h.Input != v.Prepared[v.Cursor] {
			return bad(ErrDurableInventory)
		}
		var d sc.BoundSelectedDescriptionV3
		if h.Schema == is.InventoryReferenceHeadVersionV1 {
			d, e = sc.CompleteInventoryReferenceEvidenceV1(ctx, s.Root, h.Input, v.PreparedAdmissions[v.Cursor], run, v.Report.Census[v.Cursor].ID, v.Interpreter, time.Unix(0, h.Deadline), b, answer)
		} else {
			d, e = sc.CompleteInventoryEvidence(ctx, s.Root, h.Input, run, v.Report.Census[v.Cursor].ID, v.Interpreter, time.Unix(0, h.Deadline), b, answer)
		}
		if e != nil {
			return bad(e)
		}
		raw, e := d.Bytes(ctx)
		if e != nil {
			return bad(e)
		}
		ref, e := publication.StoreObject(s.Root, "inventory-description-", raw)
		if e != nil {
			return bad(e)
		}
		v.Descriptions = append(v.Descriptions, ref)
		recordInventoryDescription(&v.Report, v.Cursor)
		v.Cursor++
		nextPrepared(&v)
		if v.Cursor == len(v.Prepared) {
			h.Phase = "GroupDecision"
			v.Base, e = s.base(ctx, &v, b)
			if e != nil {
				return bad(e)
			}
		}
	case "GroupDecision", "Reviewer":
		review := h.Phase == "Reviewer"
		ref := v.Base
		if review {
			ref = v.Proposed
		}
		if h.Input != ref {
			return bad(ErrDurableInventory)
		}
		c, e := cat.ReopenStoredNativeV3(ctx, s.Root, ref, b, v.Interpreter, false)
		if e != nil {
			return bad(e)
		}
		actor := v.Interpreter
		if review {
			actor = v.Reviewer
		}
		next, e := applyInventoryDecision(ctx, b, c, answer, review, actor)
		if e != nil {
			return bad(e)
		}
		stage := "group_decision"
		if review {
			stage = "review"
		}
		recordInventoryStage(&v.Report, stage, true)
		recordInventoryStage(&v.Report, "catalog", true)
		raw, e := next.Bytes(ctx)
		if e != nil {
			return bad(e)
		}
		stored, e := publication.StoreObject(s.Root, "inventory-catalog-", raw)
		if e != nil {
			return bad(e)
		}
		if !review {
			v.Proposed = stored
			h.Phase = "Reviewer"
		} else {
			v.Corrected = stored
			ds, e := s.descriptions(ctx, v, b)
			if e != nil {
				return bad(e)
			}
			report, finalRaw, e := reconcileInventory(ctx, b, v.Report, ds, next)
			if e != nil {
				return bad(e)
			}
			baseRaw, e := publication.ReadObject(s.Root, "inventory-catalog-", v.Base)
			if e != nil {
				return bad(e)
			}
			proposedRaw, e := publication.ReadObject(s.Root, "inventory-catalog-", v.Proposed)
			if e != nil {
				return bad(e)
			}
			report.SemanticReservedNS = h.SemanticNS
			report.PublicationRunReservedBytes = h.Used
			report.PublicationRunWork = h.Work
			report.PublicationRunPackets = h.Packets
			report.CallerBytes = h.Used
			report, e = publishInventory(ctx, Request{Destination: filepath.Join(s.Directory, "inventory-output-"+run)}, b, report, v.ManifestCost, baseRaw, proposedRaw, finalRaw)
			if e != nil {
				return bad(e)
			}
			// The publication manifest keeps its charged suballocation accounting;
			// the authoritative run ledger records all reservations across processes.
			v.Report = report
			h.Phase = "Published"
			reportRaw, e := json.Marshal(report)
			if e != nil {
				return bad(e)
			}
			h.Report, e = publication.StoreObject(s.Root, "inventory-report-", reportRaw)
			h.OutputBytes = uint64(v.Base.Bytes + v.Proposed.Bytes + v.Corrected.Bytes + h.Report.Bytes)
			if e != nil {
				return bad(e)
			}
			// Bind the actual publication manifest too, including its accounting metadata.
			if b.Reserve(ctx, 8*durableManifestUpper) != nil {
				return bad(ErrDurableInventory)
			}
			publishedRoot, e := publication.OpenRoot(filepath.Join(s.Directory, "inventory-output-"+run))
			if e != nil {
				return bad(e)
			}
			manifestRaw, e := publication.ReadVerifiedBoundFileChecked(publishedRoot, "manifest.json", durableManifestUpper)
			e = errors.Join(e, publishedRoot.Close())
			if e != nil {
				return bad(e)
			}
			h.Manifest, e = publication.StoreObject(s.Root, "inventory-publication-manifest-", manifestRaw)
			if e != nil {
				return bad(e)
			}
		}
	default:
		return bad(ErrDurableInventory)
	}
	priorDeadline := h.Deadline
	if e = s.issueNext(&h, v); e != nil {
		return bad(e)
	}
	if e = s.buildDelivery(ctx, &h, b); e != nil {
		return bad(e)
	}
	h.SuccessorReserve, h.SuccessorPackets, e = successorEscrow(h)
	if e != nil {
		return bad(e)
	}
	h.State, e = s.saveState(ctx, v, b)
	if e != nil {
		return bad(e)
	}
	h.Acceptance = "inventory-accepted-" + run + "-" + h.TaskID
	h.AcceptanceDeadline = priorDeadline
	if _, e = s.change(ctx, claimed, h); e != nil {
		return bad(e)
	}
	if e = publication.PublishTimedCompletion(ctx, s.Root, h.Acceptance, inventoryAcceptance(h), time.Unix(0, priorDeadline)); e != nil {
		return bad(e)
	}
	return s.Task(ctx, run, "")
}

// Result verifies the real publication's four artifacts against its manifest.
// It never reruns semantic transitions, including after task/run expiry.
func (s DurableStore) Result(ctx context.Context, run, task string) (Report, error) {
	h, old, e := s.head(run)
	if e != nil {
		return Report{}, e
	}
	if h.Phase != "Published" || h.TaskID != task || !h.Report.Valid() {
		return Report{}, ErrDurableInventory
	}
	allocation := h.OutputBytes*8 + 8*uint64(h.Manifest.Bytes)
	required := h.CompletionCursor < len(h.Completion.Stages)
	if required {
		if allocation > h.Completion.Stages[h.CompletionCursor] {
			return Report{}, ErrDurableInventory
		}
		allocation = h.Completion.Stages[h.CompletionCursor]
	}
	h, _, e = s.reserve(ctx, h, old, allocation, false, required)
	if e != nil {
		return Report{}, e
	}
	root, e := publication.OpenRoot(filepath.Join(s.Directory, "inventory-output-"+run))
	if e != nil {
		return Report{}, e
	}
	defer root.Close()
	raw, e := publication.ReadVerifiedBoundFileChecked(root, "manifest.json", int64(h.Manifest.Bytes))
	var m PublicationManifestV3
	if e != nil || len(raw) != h.Manifest.Bytes || cat.NativeDigestV3(raw) != "sha256:"+h.Manifest.ID || strictInventory(raw, &m) != nil || len(m.Artifacts) != 4 || m.Authority != 0 || m.Accepted || m.Completeness != "UNKNOWN" {
		return Report{}, ErrDurableInventory
	}
	expected := map[string]bool{"catalog-base.json": true, "catalog-proposed.json": true, "catalog-corrected.json": true, "report.json": true}
	var report Report
	remaining := h.OutputBytes
	for _, a := range m.Artifacts {
		if !expected[a.Name] || a.Bytes < 1 || a.Bytes > 8<<20 || uint64(a.Bytes) > remaining {
			return Report{}, ErrDurableInventory
		}
		delete(expected, a.Name)
		remaining -= uint64(a.Bytes)
		raw, e = publication.ReadVerifiedBoundFileChecked(root, a.Name, int64(a.Bytes))
		if e != nil || len(raw) != a.Bytes || cat.NativeDigestV3(raw) != a.SHA256 {
			return Report{}, ErrDurableInventory
		}
		if a.Name == "report.json" {
			if a.SHA256 != "sha256:"+h.Report.ID || strictInventory(raw, &report) != nil {
				return Report{}, ErrDurableInventory
			}
		}
		if a.Name == "catalog-corrected.json" && a.SHA256 != "sha256:"+h.Input.ID {
			return Report{}, ErrDurableInventory
		}
	}
	if remaining != 0 || !report.Published || report.Authority != 0 || report.Accepted || report.Completeness != "UNKNOWN" || ctx.Err() != nil {
		return Report{}, ErrDurableInventory
	}
	if is.InventoryConnectedHead(h.Schema) {
		if e := root.Close(); e != nil {
			return Report{}, e
		}
	}
	return report, nil
}
