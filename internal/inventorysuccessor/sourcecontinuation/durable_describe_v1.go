//go:build darwin || linux

package sourcecontinuation

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"time"

	"lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	tp "lsp-trace/internal/targetpacket"
)

// This is application-owned local-store custody, not an arbitrary bundle importer.
// The application configures Root. Callers supply lookup IDs, never paths/records.
// Same-user malicious store replacement/rollback is outside this trust model.
// No function here reconstructs a native admission, preparation or provider grant.
type DurableDescribeStoreV1 struct{ Root *publication.Root }

var ErrDurableDescribeV1 = errors.New("durable Describe unavailable, stale, expired, corrupt or spent")

const durableDescribeLimit = 8 << 20
const DurableDescribePageLimitV1 = 32 << 10
const durableLogicalLimit = 64 << 20

// Declarations only: neither names nor prompt hashes authenticate actors or
// establish independent review. The future review stage must remain separate.
type DurableDescribePolicyV1 struct {
	ReviewMode  cat.ReviewModeV3
	Reviewer    string
	RolePrompts [2]string
}

func (p DurableDescribePolicyV1) valid(interpreter string) bool {
	if !smallSelectedV3(p.Reviewer) || len(p.RolePrompts[0]) == 0 || len(p.RolePrompts[1]) == 0 || len(p.RolePrompts[0]) > 4096 || len(p.RolePrompts[1]) > 4096 {
		return false
	}
	return p.ReviewMode == cat.ReviewModeSameThreadV3 && p.Reviewer == interpreter || p.ReviewMode == cat.ReviewModeSubagentsV3 && p.Reviewer != interpreter
}
func (p DurableDescribePolicyV1) pins() [2]string {
	return [2]string{durableDigest([]byte(p.RolePrompts[0])), durableDigest([]byte(p.RolePrompts[1]))}
}

type DurableDescribeTaskV1 struct {
	ReviewMode                                       cat.ReviewModeV3
	Reviewer, Assurance                              string
	RolePromptSHA256                                 [2]string
	Schema, RunID, TaskID, Role, Interpreter, Status string
	DeadlineUnixNano                                 int64
	Artifacts                                        []DurableDescribeArtifactV1
	Authority                                        uint8
	Accepted                                         bool
	Completeness                                     string
}
type DurableDescribeArtifactV1 struct {
	ID    string
	Bytes int
}
type DurableDescribePageV1 struct {
	RunID, TaskID, ArtifactID string
	Offset, Total             int
	Data                      []byte
}

// Prepared data is private to this package. Only Issue can write it; read APIs
// return bounded copies. Its provenance is the completed original owner's mint
// plus exact handoff/checkpoint and source/request/host bindings, not a signature.
type durableDescribePreparedV1 struct {
	Schema, RunID, TaskID            string
	Selected                         PrivateSelectedEndpointV2
	Accounting                       inv.CompletedSelectedAccountingV1
	Policy                           DurableDescribePolicyV1
	Unit                             tp.PrivateSourceUnit
	Request                          describerequest.Record
	Handoff, Checkpoint, Interpreter string
	Host                             dw.ResponseHostV2
	Limits                           dw.SuppliedResponseLimits
	RunDeadline, Deadline            int64
}
type durableDescribeHeadV1 struct {
	ReviewMode                                      cat.ReviewModeV3
	Reviewer                                        string
	RolePromptSHA256                                [2]string
	Schema, RunID, TaskID, Prepared, Status, Result string
	PreparedBytes, ResultBytes                      int
	ArtifactBytes                                   [3]int // prepared, source unit, Describe request
	Interpreter                                     string
	Issued, RunDeadline, Deadline, SemanticNS       int64
	Used, Packets, Work                             uint64
}
type durableDescribeSealV1 struct {
	Digest  string
	Content json.RawMessage
}

func durableDigest(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
func durableID(s string) bool {
	if len(s) != 64 {
		return false
	}
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func durableRandom() (string, error) {
	var b [32]byte
	_, e := rand.Read(b[:])
	return hex.EncodeToString(b[:]), e
}
func durableStrict(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return ErrDurableDescribeV1
	}
	canonical, e := json.Marshal(v)
	if e != nil || !bytes.Equal(canonical, b) {
		return ErrDurableDescribeV1
	}
	return nil
}
func durableSeal(v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	return json.Marshal(durableDescribeSealV1{durableDigest(b), b})
}
func durableUnseal(b []byte, v any) error {
	var s durableDescribeSealV1
	if durableStrict(b, &s) != nil || durableDigest(s.Content) != s.Digest {
		return ErrDurableDescribeV1
	}
	return durableStrict(s.Content, v)
}
func durableReceipt(r *publication.BoundFileReceipt, e error) error {
	if e != nil {
		return e
	}
	if r == nil || !r.NamespaceAtomic || r.VerificationStatus != "VERIFIED" || r.DirectorySyncStatus != publication.DirectorySyncComplete || r.CloseStatus != publication.CloseComplete {
		return ErrDurableDescribeV1
	}
	return nil
}
func (s DurableDescribeStoreV1) put(name string, b []byte) error {
	r, e := publication.PublishBoundFile(s.Root, name, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return ErrDurableDescribeV1
		}
		return nil
	})
	if e = durableReceipt(r, e); e != nil {
		return e
	}
	got, e := publication.ReadVerifiedBoundFileChecked(s.Root, name, int64(len(b)))
	if e != nil || !bytes.Equal(got, b) {
		return ErrDurableDescribeV1
	}
	return nil
}
func durableHeadName(run string) string { return "describe-" + run + ".json" }
func (s DurableDescribeStoreV1) head(run, task string) (durableDescribeHeadV1, []byte, error) {
	var h durableDescribeHeadV1
	if !durableID(run) || !durableID(task) {
		return h, nil, ErrDurableDescribeV1
	}
	raw, e := publication.ReadVerifiedBoundFileChecked(s.Root, durableHeadName(run), 16384)
	if e != nil || durableUnseal(raw, &h) != nil || h.Schema != "lsp-trace.durable-describe-head.v1" || h.RunID != run || h.TaskID != task || !durableID(h.Prepared) || h.PreparedBytes < 1 || h.PreparedBytes > durableDescribeLimit || h.Used > durableLogicalLimit || h.Packets > 64 || h.Work > 1<<32 || h.Deadline <= h.Issued || h.Deadline > h.RunDeadline || h.RunDeadline-h.Issued > int64(180*time.Second) || h.SemanticNS <= 0 || h.SemanticNS > int64(30*time.Second) || h.Deadline-h.Issued > h.SemanticNS || !smallSelectedV3(h.Interpreter) {
		return h, nil, ErrDurableDescribeV1
	}
	if h.ReviewMode != cat.ReviewModeSameThreadV3 && h.ReviewMode != cat.ReviewModeSubagentsV3 || !smallSelectedV3(h.Reviewer) || (h.ReviewMode == cat.ReviewModeSameThreadV3) != (h.Reviewer == h.Interpreter) {
		return h, nil, ErrDurableDescribeV1
	}
	if h.ArtifactBytes[0] != h.PreparedBytes || h.ArtifactBytes[1] < 1 || h.ArtifactBytes[2] < 1 || h.ArtifactBytes[1] > h.PreparedBytes || h.ArtifactBytes[2] > h.PreparedBytes {
		return h, nil, ErrDurableDescribeV1
	}
	switch h.Status {
	case "PENDING", "CLAIMED", "COMPLETE":
	default:
		return h, nil, ErrDurableDescribeV1
	}
	if h.Status == "COMPLETE" && (!durableID(h.Result) || h.ResultBytes < 1 || h.ResultBytes > durableDescribeLimit) || h.Status != "COMPLETE" && (h.Result != "" || h.ResultBytes != 0) {
		return h, nil, ErrDurableDescribeV1
	}
	return h, raw, nil
}

// Bound mutations by the authoritative predecessor, not by a renewed request
// context. Verified COMPLETE reads charge usage without reopening semantic work.
func durableMutationContext(ctx context.Context, h durableDescribeHeadV1) (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, time.Unix(0, min(h.Deadline, h.RunDeadline)))
}

func (s DurableDescribeStoreV1) change(ctx context.Context, old []byte, h durableDescribeHeadV1) ([]byte, error) {
	var predecessor durableDescribeHeadV1
	if ctx == nil || ctx.Err() != nil || durableUnseal(old, &predecessor) != nil {
		return nil, ErrDurableDescribeV1
	}
	if predecessor.Status != "COMPLETE" {
		var cancel context.CancelFunc
		ctx, cancel = durableMutationContext(ctx, predecessor)
		defer cancel()
	}
	raw, e := durableSeal(h)
	if e != nil {
		return nil, e
	}
	name := durableHeadName(h.RunID)
	r, e := publication.CompareAndReplaceBoundFile(ctx, s.Root, name, publication.BoundFilePredecessor{Selector: name, Digest: durableDigest(old), ByteLength: uint64(len(old))}, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return ErrDurableDescribeV1
		}
		return nil
	})
	if e != nil || r == nil || !r.Committed || r.Outcome != publication.CASOutcomeCommitted {
		return nil, ErrDurableDescribeV1
	}
	if durableReceipt(r.BoundFileReceipt, nil) != nil || ctx.Err() != nil {
		return nil, ErrDurableDescribeV1
	}
	return raw, nil
}
func (s DurableDescribeStoreV1) prepared(h durableDescribeHeadV1) (durableDescribePreparedV1, error) {
	var p durableDescribePreparedV1
	raw, e := publication.ReadVerifiedBoundFileChecked(s.Root, "prepared-"+h.Prepared+".json", int64(h.PreparedBytes))
	if e != nil || durableDigest(raw) != "sha256:"+h.Prepared || len(raw) != h.PreparedBytes || durableStrict(raw, &p) != nil || p.Schema != "lsp-trace.durable-describe-prepared.v1" || p.RunID != h.RunID || p.TaskID != h.TaskID || p.Deadline != h.Deadline || p.RunDeadline != h.RunDeadline || p.Interpreter != h.Interpreter || p.Handoff == "" || p.Checkpoint == "" || p.Accounting.Head == "" || p.Accounting.Event != "OBSERVE" || p.Accounting.Disposition != "OBSERVED" || p.Accounting.Windows != 3 || !p.Policy.valid(p.Interpreter) || p.Policy.ReviewMode != h.ReviewMode || p.Policy.Reviewer != h.Reviewer || p.Policy.pins() != h.RolePromptSHA256 || !p.Limits.Valid() || !selectedSelectorBoundV2(p.Selected) || describerequest.ValidateRecord(p.Request) != nil || p.Unit.ID != p.Request.Envelope.AdmissionID || p.Host.RequestRecordID != p.Request.RecordID || p.Host.Pins.PromptSHA256 != "sha256:"+p.Request.Envelope.InputSHA256 {
		return p, ErrDurableDescribeV1
	}
	return p, nil
}

// Issue is a genuine owner-side write after selected cleanup/Finish/accounting
// readback. It uses the existing verifier, never a DTO-to-native constructor.
// Caller wait time is inside the absolute <=30s task wall deadline. No renewal.
func (s DurableDescribeStoreV1) Issue(ctx context.Context, r PrivateSelectedResultV2, b CallerDescribeBindingV3, runDeadline time.Time, policy DurableDescribePolicyV1) (DurableDescribeTaskV1, error) {
	bad := func(e error) (DurableDescribeTaskV1, error) { return DurableDescribeTaskV1{}, e }
	if !policy.valid(b.Interpreter) || !liveSelectedV3(ctx, runDeadline) || time.Until(runDeadline) > 180*time.Second || b.Deadline.After(runDeadline) || s.Root == nil || s.Root.ValidatePrivate() != nil {
		return bad(ErrDurableDescribeV1)
	}
	if r.state == nil || !r.state.durableIssued.CompareAndSwap(false, true) {
		return bad(ErrDurableDescribeV1)
	}
	issued := time.Now().UTC() // persisted wall time; do not compare a monotonic duration to Unix timestamps
	// Reserve the entire caller wait/interpretation interval once, independently
	// from native lifetime; restarts cannot reset this reservation.
	window := b.Deadline.Sub(issued)
	_, cancel, e := b.Budget.Exchange(ctx, window)
	if e != nil {
		return bad(e)
	}
	cancel()
	a, e := PrepareSelectedDescribeV3(ctx, r, b)
	if e != nil {
		return bad(e)
	}
	run, e := durableRandom()
	if e != nil {
		return bad(e)
	}
	task, e := durableRandom()
	if e != nil {
		return bad(e)
	}
	v := a.state
	p := durableDescribePreparedV1{Schema: "lsp-trace.durable-describe-prepared.v1", RunID: run, TaskID: task, Selected: v.selected, Accounting: r.state.accounting, Policy: policy, Unit: v.unit, Request: v.request, Handoff: v.handoff, Checkpoint: v.checkpoint, Interpreter: b.Interpreter, Host: v.host, Limits: b.Limits, RunDeadline: runDeadline.UnixNano(), Deadline: b.Deadline.UnixNano()}
	raw, e := json.Marshal(p)
	if e != nil || len(raw) > durableDescribeLimit || b.Budget.Reserve(ctx, uint64(len(raw))*6+16384) != nil {
		return bad(ErrDurableDescribeV1)
	}
	unitRaw, e := json.Marshal(p.Unit)
	if e != nil {
		return bad(e)
	}
	requestRaw, e := json.Marshal(p.Request)
	if e != nil {
		return bad(e)
	}
	if b.Budget.Reserve(ctx, uint64(len(unitRaw)+len(requestRaw))*2+16384) != nil {
		return bad(ErrDurableDescribeV1)
	}
	used, semantic := b.Budget.Usage()
	if semantic != window {
		return bad(ErrDurableDescribeV1)
	} // this one-item run cannot reuse a caller exchange budget
	id := durableDigest(raw)[7:]
	if e = s.put("prepared-"+id+".json", raw); e != nil {
		return bad(e)
	}
	h := durableDescribeHeadV1{ReviewMode: policy.ReviewMode, Reviewer: policy.Reviewer, RolePromptSHA256: policy.pins(), Schema: "lsp-trace.durable-describe-head.v1", RunID: run, TaskID: task, Prepared: id, PreparedBytes: len(raw), Deadline: p.Deadline, Status: "PENDING", Used: used, Work: used * 32, Issued: issued.UnixNano(), RunDeadline: p.RunDeadline, SemanticNS: int64(semantic), Interpreter: p.Interpreter, ArtifactBytes: [3]int{len(raw), len(unitRaw), len(requestRaw)}}
	head, e := durableSeal(h)
	if e != nil {
		return bad(e)
	}
	if e = s.put(durableHeadName(run), head); e != nil {
		return bad(e)
	}
	checked, e := s.prepared(h)
	if e != nil || !reflect.DeepEqual(checked, p) {
		// JSON canonicalization must not silently normalize an original value.
		return bad(ErrDurableDescribeV1)
	}
	if !liveSelectedV3(ctx, b.Deadline) {
		return bad(ErrDurableDescribeV1)
	}
	return s.Task(ctx, run, task)
}
func (s DurableDescribeStoreV1) reserve(ctx context.Context, h durableDescribeHeadV1, old []byte, n uint64, claim bool) (durableDescribeHeadV1, []byte, error) {
	if !liveSelectedV3(ctx, time.Unix(0, h.Deadline)) || h.Status != "PENDING" || n > durableLogicalLimit-h.Used || h.Packets >= 64 {
		return h, nil, ErrDurableDescribeV1
	}
	if n > ((1<<32)-h.Work)/32 {
		return h, nil, ErrDurableDescribeV1
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

// Task is bounded metadata/status. Duplicate status requests do not spend an
// answer or replay work. Evidence reads and submissions are separately charged.
func (s DurableDescribeStoreV1) Task(ctx context.Context, run, task string) (DurableDescribeTaskV1, error) {
	h, _, e := s.head(run, task)
	if e != nil || ctx == nil || ctx.Err() != nil {
		return DurableDescribeTaskV1{}, ErrDurableDescribeV1
	}
	status := h.Status
	if status == "COMPLETE" && s.confirmed(h) != nil {
		status = "UNCERTAIN_SPENT"
	}
	if status == "CLAIMED" {
		status = "UNCERTAIN_SPENT"
	}
	if status == "PENDING" && !time.Now().Before(time.Unix(0, h.Deadline)) {
		status = "EXPIRED"
	}
	assurance := "SAME_THREAD_SEQUENTIAL_REVIEW_REQUIRED_NOT_PERFORMED"
	if h.ReviewMode == cat.ReviewModeSubagentsV3 {
		assurance = "DISTINCT_ROLES_DECLARED_UNVERIFIED_REVIEW_NOT_PERFORMED"
	}
	return DurableDescribeTaskV1{ReviewMode: h.ReviewMode, Reviewer: h.Reviewer, RolePromptSHA256: h.RolePromptSHA256, Assurance: assurance, Schema: "lsp-trace.durable-describe-task.v1", RunID: run, TaskID: task, Role: "interpreter", Interpreter: h.Interpreter, Status: status, DeadlineUnixNano: h.Deadline, Artifacts: []DurableDescribeArtifactV1{{ID: "prepared", Bytes: h.ArtifactBytes[0]}, {ID: "source", Bytes: h.ArtifactBytes[1]}, {ID: "request", Bytes: h.ArtifactBytes[2]}}, Completeness: "UNKNOWN"}, nil
}

// Read returns at most 32KiB. The immutable prepared record contains separate
// named Unit and Request fields, with exact original source/query/prompt pins.
func (s DurableDescribeStoreV1) Read(ctx context.Context, run, task, artifact string, offset, limit int) (DurableDescribePageV1, error) {
	bad := func() (DurableDescribePageV1, error) { return DurableDescribePageV1{}, ErrDurableDescribeV1 }
	index := map[string]int{"prepared": 0, "source": 1, "request": 2}
	i, known := index[artifact]
	if !known || offset < 0 || limit < 1 || limit > DurableDescribePageLimitV1 {
		return bad()
	}
	h, old, e := s.head(run, task)
	if e != nil || offset >= h.ArtifactBytes[i] {
		return bad()
	}
	h, _, e = s.reserve(ctx, h, old, uint64(h.PreparedBytes)*5+uint64(limit)*2+16384, false)
	if e != nil {
		return bad()
	}
	p, e := s.prepared(h)
	if e != nil {
		return bad()
	}
	var value any = p
	if artifact == "source" {
		value = p.Unit
	}
	if artifact == "request" {
		value = p.Request
	}
	raw, e := json.Marshal(value)
	if e != nil || len(raw) != h.ArtifactBytes[i] || !liveSelectedV3(ctx, time.Unix(0, h.Deadline)) {
		return bad()
	}
	end := min(offset+limit, len(raw))
	return DurableDescribePageV1{RunID: run, TaskID: task, ArtifactID: artifact, Offset: offset, Total: len(raw), Data: append([]byte(nil), raw[offset:end]...)}, nil
}

// Submit claims before parsing even malformed answers. A crash or uncertain
// commit leaves CLAIMED non-replayable. No native state exists in this process.
func (s DurableDescribeStoreV1) Submit(ctx context.Context, run, task, role string, answer []byte) error {
	if role != "interpreter" {
		return ErrDurableDescribeV1
	}
	h, old, e := s.head(run, task)
	if e != nil {
		return e
	}
	if ctx == nil {
		return ErrDurableDescribeV1
	}
	ctx, cancel := durableMutationContext(ctx, h)
	defer cancel()
	// Reserve the full bounded parsing/completion allowance before reading payload.
	const completion = 16 << 20
	h, claimed, e := s.reserve(ctx, h, old, uint64(h.PreparedBytes)*5+completion, true)
	if e != nil {
		return e
	}
	p, e := s.prepared(h)
	if e != nil {
		return e
	}
	budget, e := NewCallerBudgetV3(time.Unix(0, p.Deadline), completion)
	if e != nil {
		return e
	}
	b := CallerDescribeBindingV3{AttemptID: p.Host.AttemptID, Interpreter: p.Interpreter, Pins: p.Host.Pins, Deadline: time.Unix(0, p.Deadline), Limits: p.Limits, Budget: budget}
	a := SelectedDescribeV3{&selectedDescribeStateV3{selected: p.Selected, unit: p.Unit, request: p.Request, handoff: p.Handoff, checkpoint: p.Checkpoint, binding: b, host: p.Host}}
	result, e := CompleteSelectedDescribeV3(ctx, a, answer)
	if e != nil {
		return e
	}
	raw, e := result.Bytes(ctx)
	if e != nil || len(raw) > durableDescribeLimit {
		return ErrDurableDescribeV1
	}
	id := durableDigest(raw)[7:]
	if e = s.put("description-"+id+".json", raw); e != nil {
		return e
	}
	if !liveSelectedV3(ctx, b.Deadline) {
		return ErrDurableDescribeV1
	}
	h.Status = "COMPLETE"
	h.Result = id
	h.ResultBytes = len(raw)
	_, e = s.change(ctx, claimed, h)
	if e != nil {
		return e
	}
	// No confirmation is issued following a late CAS failure. Restart may only
	// observe that uncertain completion, never replay its answer.
	return s.confirm(ctx, h)
}

// The confirmation has its own context-aware, absent-predecessor CAS. In
// particular, waiting on its OS lock cannot extend the task lifetime. A failed
// acknowledgement cannot refund the COMPLETE/CLAIMED head or repeat semantics.
func (s DurableDescribeStoreV1) confirm(ctx context.Context, h durableDescribeHeadV1) error {
	return publication.PublishTimedCompletion(ctx, s.Root, "confirmed-"+h.RunID+".json", durableConfirmation(h), time.Unix(0, min(h.Deadline, h.RunDeadline)))
}

// Result is a verified data-only description, not accepted inventory or native
// authority. COMPLETE results may be reopened after expiry; no work is repeated.
func durableConfirmation(h durableDescribeHeadV1) []byte {
	return []byte(h.RunID + "\n" + h.TaskID + "\n" + h.Result + "\n")
}
func (s DurableDescribeStoreV1) confirmed(h durableDescribeHeadV1) error {
	return publication.VerifyTimedCompletion(s.Root, "confirmed-"+h.RunID+".json", durableConfirmation(h), time.Unix(0, min(h.Deadline, h.RunDeadline)))
}
func (s DurableDescribeStoreV1) Result(ctx context.Context, run, task string) ([]byte, error) {
	h, old, e := s.head(run, task)
	if e != nil || h.Status != "COMPLETE" || ctx == nil || ctx.Err() != nil || s.confirmed(h) != nil {
		return nil, ErrDurableDescribeV1
	}
	n := uint64(h.ResultBytes)*4 + 16384
	if h.Packets >= 64 || n > durableLogicalLimit-h.Used || n > ((1<<32)-h.Work)/32 {
		return nil, ErrDurableDescribeV1
	}
	h.Packets++
	h.Used += n
	h.Work += n * 32
	if _, e = s.change(ctx, old, h); e != nil {
		return nil, e
	}
	raw, e := publication.ReadVerifiedBoundFileChecked(s.Root, "description-"+h.Result+".json", int64(h.ResultBytes))
	if e != nil || durableDigest(raw) != "sha256:"+h.Result {
		return nil, ErrDurableDescribeV1
	}
	return raw, nil
}
