package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	qt "lsp-trace/internal/adr0011querytarget"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/sourceposition"
)

var ErrPreparedDescribeV1 = errors.New("private prepared Describe refused")

const preparedDescribeNamespaceV1 = "inventory-successor/prepared-describe/private/v1"
const preparedDescribeSchemaV1 = "lsp-trace.prepared-describe.private.v1"
const preparedDescribeRecordMaxV1 uint64 = 16 << 20

// These are trusted-parent policy inputs, never a packet grant. The caller must
// already have paid canonical owner/bootstrap verification under this Decision.
// RecordLimit selects min(configured cap, native hard ceiling); it is NOT credit.
// Deadline is the live persistence window, Expires the independent continuation
// cutoff, and RetentionUntil the explicit finite upper policy bound on Expires.
type PreparedDescribePersistRequestV1 struct {
	Attempt, Nonce, RootPath, InitialHead, Decision string
	Question, Request, ResponseSchema               []byte
	Deadline, Expires, RetentionUntil               time.Time
	RecordLimit                                     uint64
}

// Anchors MUST come from trusted caller configuration, not evaluator packet
// fields. Equality hashes are not authorship. Native authorship is only local
// producer custody under the same-UID trusted root; malicious same-UID writers
// are outside that boundary. CompletionScope is derived from W, never from P.
type PreparedDescribeAnchorsV1 struct {
	RootPath, ProducerJob, InitialHead, Workflow, Manifest, Genesis, Observed, CompletionScope string
}
type PreparedDescribeOpenRequestV1 struct {
	Attempt, Decision string
	Anchors           PreparedDescribeAnchorsV1
	Deadline, Expires time.Time
	RecordLimit       uint64
}

type PreparedDescribePersistOperationV1 struct{ state *preparedDescribePersistV1 }
type preparedDescribePersistV1 struct {
	mu          sync.Mutex
	used        bool
	owner       *PrivateManagedInputOwnerV2
	request     PreparedDescribePersistRequestV1
	record      preparedDescribeRecordV1
	reservation string
	parent      context.Context
}
type PreparedDescribeOpenOperationV1 struct{ state *preparedDescribeOpenV1 }
type preparedDescribeOpenV1 struct {
	mu          sync.Mutex
	used        bool
	owner       *PrivateManagedInputOwnerV2
	request     PreparedDescribeOpenRequestV1
	reservation string
	parent      context.Context
}
type PreparedDescribeV1 struct{ state *preparedDescribeResultV1 }
type OpenedPreparedDescribeV1 struct{ state *preparedDescribeResultV1 }
type preparedDescribeResultV1 struct {
	anchors                PreparedDescribeAnchorsV1
	definition, provenance []byte
}

func (p PreparedDescribeV1) Anchors() PreparedDescribeAnchorsV1 {
	if p.state == nil {
		return PreparedDescribeAnchorsV1{}
	}
	return p.state.anchors
}
func (p OpenedPreparedDescribeV1) Definition() []byte {
	if p.state == nil {
		return nil
	}
	return bytes.Clone(p.state.definition)
}
func (p OpenedPreparedDescribeV1) Provenance() []byte {
	if p.state == nil {
		return nil
	}
	return bytes.Clone(p.state.provenance)
}

// All fields here are durable data, not reconstructed old source authority.
// Query/Input/Predecessors preserve exact original lineage; no old Resume DTO,
// pointer, cleanup assertion, provider handle or callback is serialized.
type preparedDescribeProvenanceV1 struct {
	URI, SourceDigest, SessionID, DocumentVersion, Encoding, SymbolID string
	Workspace, Revision, Privacy                                      string
	Generation                                                        uint64
	Selection, Display                                                qt.Range
}
type preparedDescribeArtifactV1 struct {
	Role, Selector, Schema string
	Bytes                  []byte
}
type preparedDescribeRecordV1 struct {
	Schema, Workflow, CompletionScope, RootPath, InitialHead, Decision            string
	Producer                                                                      jobV2
	CaptureObserved, PreparationReserved, PreparationObserved, PreparationAttempt string
	PersistReserved, PersistAttempt                                               string
	Created, Expires, RetentionUntil                                              int64
	RecordLimit                                                                   uint64
	Mode, RunID, SeedNamespace                                                    string
	Seed                                                                          uint64
	Provenance                                                                    preparedDescribeProvenanceV1
	Definition, Document, Query, Input, Case, Result, Policy                      []byte
	Predecessors                                                                  []preparedDescribeArtifactV1
	Question, Request, ResponseSchema                                             []byte
}
type preparedDescribeGenesisV1 struct {
	Schema, Workflow, Manifest, Reservation string
	At                                      int64
}

func preparedDescribeLimitV1(n uint64) uint64 {
	if n > preparedDescribeRecordMaxV1 {
		return preparedDescribeRecordMaxV1
	}
	return n
}
func preparedDescribeWindowV1(now, deadline, expiry time.Time) bool {
	return !now.IsZero() && now.UnixNano() > 0 && now.Before(deadline) && !deadline.After(expiry) && deadline.Sub(now) <= 15*time.Second
}

// Closed logical expenditure, not physical IO/CPU measurement. V is the existing
// 4096-record/64MiB/16384-work native verifier ceiling. Eight V waves cover
// reserve, producer-chain validation/membership, Observe and final readback;
// twenty (64MiB snapshot + selected record cap) passes cover bounded source,
// case/result/policy/predecessor copies, JSON expansion, rejected serialization,
// parsing, hashing and readback. The fixed snapshot allowance is never reduced
// by a tiny selected cap: rejecting an oversized snapshot is still paid. Twelve native publication waves cover the
// reservation/head/genesis CAS and cleanup, including failure paths. No retries,
// refunds or caller-selected grants. All arithmetic is checked via native add.
// ProviderRequests/Messages and Request/Response/WireBytes are exactly zero:
// neither this operation nor Open acquires source or invokes an evaluator.
func preparedDescribeChargeV1(limit uint64, now, deadline time.Time) (countersV2, error) {
	n := preparedDescribeLimitV1(limit)
	if n == 0 || !preparedDescribeWindowV1(now, deadline, deadline) {
		return countersV2{}, ErrPreparedDescribeV1
	}
	mul := func(a, b uint64) (uint64, error) {
		if b != 0 && a > ^uint64(0)/b {
			return 0, ErrPreparedDescribeV1
		}
		return a * b, nil
	}
	const snapshot uint64 = 64 << 20
	if n > ^uint64(0)-snapshot {
		return countersV2{}, ErrPreparedDescribeV1
	}
	data, e := mul(n+snapshot, 20)
	if e != nil {
		return countersV2{}, e
	}
	writes, e := mul(n, 4)
	if e != nil {
		return countersV2{}, e
	}
	c := countersV2{LogicalBytes: data, ReadHashBytes: data, WriteBytes: writes, ParserBytes: data, Work: data, Records: 256, Calls: 256, ActiveNS: uint64(deadline.Sub(now)), OutputBytes: n}
	return addCountersV2(c, countersV2{ReadHashBytes: 8 * (64 << 20), ParserBytes: 8 * (64 << 20), Work: 8 * 16384, Records: 8 * 4096, Calls: 8 * 16384, WriteBytes: 12 * (recordLimitV2 + 2*headLimitV2)})
}

func preparedDescribeTextV1(s string) bool {
	return len(s) > 0 && len(s) <= 4096 && utf8.ValidString(s)
}
func preparedDescribeRequestV1(q PreparedDescribePersistRequestV1, now time.Time) bool {
	if !hashV2(q.Attempt) || !hashV2(q.Nonce) || !hashV2(q.InitialHead) || !hashV2(q.Decision) || len(q.RootPath) > 4096 || !filepath.IsAbs(q.RootPath) || filepath.Clean(q.RootPath) != q.RootPath || q.RecordLimit == 0 {
		return false
	}
	if !preparedDescribeWindowV1(now, q.Deadline, q.Expires) || q.RetentionUntil.IsZero() || q.RetentionUntil.UnixNano() <= 0 || q.Expires.After(q.RetentionUntil) {
		return false
	}
	for _, b := range [][]byte{q.Question, q.Request, q.ResponseSchema} {
		if len(b) == 0 || len(b) > 128<<10 {
			return false
		}
	}
	return true
}

// Validate the consumed nonlocal preparation under preparation-before-owner
// lock ordering. Only immutable bindings are inspected before admission; payload
// copies/hashes/parses happen after the NEW reservation. The old operation is
// never executed, consumed, canceled, or finished again.
func (o *PrivateManagedInputOwnerV2) ReservePreparedDescribeV1(ctx context.Context, preparation PrivateSourceRPreparationOperationV1, prepared PreparedSourceRV1, q PreparedDescribePersistRequestV1) (PreparedDescribePersistOperationV1, error) {
	fail := func() (PreparedDescribePersistOperationV1, error) {
		return PreparedDescribePersistOperationV1{}, ErrPreparedDescribeV1
	}
	now := time.Now()
	if o == nil || ctx == nil || ctx.Err() != nil || preparation.state == nil || prepared.state == nil || !preparedDescribeRequestV1(q, now) {
		return fail()
	}
	s := preparation.state
	s.mu.Lock()
	defer s.mu.Unlock()
	p := prepared.state
	in := p.input
	r := s.request
	if s.owner != o || !s.used || s.local != nil || r.Local != nil || r.Root == nil || !in.sourceConfirmed || p.originals.state == nil || !p.frozen.Valid() || q.Deadline.After(r.Deadline) || !now.Before(r.Deadline) || r.Input.sourceEvidence != in.sourceEvidence || r.Input.sourceInputOriginal.ID() != in.sourceInputOriginal.ID() || r.Input.replayBinding.Mode != "REPLAY" || r.Input.replayBinding.KindCount != 1 || r.Input.replayBinding.Kinds != [3]string{"REFERENCES_SYMBOL", "", ""} || !samePrivateManagedBindingV2(r.Input.replayBinding, in.replayBinding) || len(in.inputOriginal) == 0 || len(in.inputOriginal) > 1<<20 || len(in.sourceQueryOriginal) == 0 || len(in.sourceQueryOriginal) > 1<<20 || len(in.originals) > 8 || len(p.objects) != 1 || len(p.objects[0].Bytes) == 0 || len(p.objects[0].Bytes) > 1<<20 {
		return fail()
	}
	if r.Workspace != p.originalTarget.workspace || r.Revision != p.originalTarget.revision || r.Workspace != p.originals.Workspace() || p.originals.InputID() != in.sourceInputOriginal.ID() {
		return fail()
	}
	// Snapshot only scalar bindings while both locks follow the existing order.
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.k.root != r.Root || o.k.root.Path() != q.RootPath || o.uncertain {
		return fail()
	}
	charge, e := preparedDescribeChargeV1(q.RecordLimit, now, q.Deadline)
	if e != nil {
		return fail()
	}
	k := preparedDescribeKernelV1(o)
	h, e := k.reserve(ctx, o.job, o.head, q.Attempt, charge, q.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if e != nil {
		if errors.Is(e, errUncertainV2) {
			o.uncertain = true
		}
		return fail()
	}
	o.head = h.Identity
	// From here onward every payload pass, including rejected bytes, is prepaid.
	rawCase := p.frozen.CanonicalBytes()
	var declaration struct {
		RunID    string
		Seed     uint64
		Selector struct{ ID string }
	}
	if len(rawCase) == 0 || len(rawCase) > 8<<20 || json.Unmarshal(rawCase, &declaration) != nil || declaration.RunID != r.RunID || declaration.Seed != r.Seed || declaration.Selector.ID != r.SeedNamespace || pc.CheckGroupingEvaluationPrivate(p.frozen, p.result.CanonicalBytes()) != nil {
		return fail()
	}
	t := p.originalTarget
	record := preparedDescribeRecordV1{Schema: preparedDescribeSchemaV1, Mode: "R", Producer: o.job, RootPath: q.RootPath, InitialHead: q.InitialHead, Decision: q.Decision, CaptureObserved: r.ExpectedHead, PreparationReserved: s.head, PreparationObserved: h.Previous, PreparationAttempt: r.Attempt, RunID: r.RunID, SeedNamespace: r.SeedNamespace, Seed: r.Seed, Provenance: preparedDescribeProvenanceV1{t.uri, t.sourceDigest, t.sessionID, t.documentVersion, t.encoding, t.symbolID, t.workspace, t.revision, t.privacy, t.generation, t.selection, t.display}, Document: bytes.Clone(p.objects[0].Bytes), Query: bytes.Clone(in.sourceQueryOriginal), Input: bytes.Clone(in.inputOriginal), Case: rawCase, Result: p.result.CanonicalBytes(), Policy: p.policy.CanonicalBytes()}
	for _, a := range in.originals {
		if len(a.Bytes) == 0 || len(a.Bytes) > 1<<20 {
			return fail()
		}
		record.Predecessors = append(record.Predecessors, preparedDescribeArtifactV1{a.Role, a.Selector, a.Schema, bytes.Clone(a.Bytes)})
	}
	start, end, e := preparedDescribeOffsetsV1(record.Document, record.Provenance)
	if e != nil || start == end || digest(record.Document) != t.sourceDigest || uint64(len(record.Document)) != p.objects[0].Identity.ByteLength || t.sourceDigest != p.objects[0].Identity.Digest {
		return fail()
	}
	record.Definition = bytes.Clone(record.Document[start:end])
	return preparedDescribeFreezeV1(ctx, o, q, record, h)
}

// Private closed-source freeze boundary. Its caller already owns the owner mutex
// and NEW native reservation. It cannot be invoked through JSON or a callback;
// in-package tests may assemble controlled opaque producer fixtures here without
// invoking a real Manager/provider or pretending their JSON is production proof.
func preparedDescribeFreezeV1(ctx context.Context, o *PrivateManagedInputOwnerV2, q PreparedDescribePersistRequestV1, r preparedDescribeRecordV1, h resourceHeadV2) (PreparedDescribePersistOperationV1, error) {
	if ctx.Err() != nil || !preparedDescribeRequestV1(q, time.Now()) || h.Event != "RESERVE" || h.Attempt != q.Attempt || h.Identity != o.head || h.Previous != r.PreparationObserved {
		return PreparedDescribePersistOperationV1{}, ErrPreparedDescribeV1
	}
	r.Workflow = hashLabelV2("prepared-describe/workflow/v1\x00" + o.job.Identity + q.Attempt + q.Nonce)
	r.CompletionScope = hashLabelV2("prepared-describe/completion/v1\x00" + r.Workflow)
	r.PersistReserved = h.Identity
	r.PersistAttempt = q.Attempt
	r.Created = h.At
	r.Expires = q.Expires.UnixNano()
	r.RetentionUntil = q.RetentionUntil.UnixNano()
	r.RecordLimit = preparedDescribeLimitV1(q.RecordLimit)
	r.Question = bytes.Clone(q.Question)
	r.Request = bytes.Clone(q.Request)
	r.ResponseSchema = bytes.Clone(q.ResponseSchema)
	if !preparedDescribeRecordValidV1(r) {
		return PreparedDescribePersistOperationV1{}, ErrPreparedDescribeV1
	}
	return PreparedDescribePersistOperationV1{&preparedDescribePersistV1{owner: o, request: q, record: r, reservation: h.Identity, parent: ctx}}, nil
}

func preparedDescribeOffsetsV1(document []byte, p preparedDescribeProvenanceV1) (int, int, error) {
	convert := func(r qt.Range) sourceposition.Range {
		return sourceposition.Range{Start: sourceposition.Position{Line: uint32(r.Start.Line), Character: uint32(r.Start.Character)}, End: sourceposition.Position{Line: uint32(r.End.Line), Character: uint32(r.End.Character)}}
	}
	a, b, e := sourceposition.Offsets(document, p.Encoding, convert(p.Display))
	if e != nil {
		return 0, 0, e
	}
	c, d, e := sourceposition.Offsets(document, p.Encoding, convert(p.Selection))
	if e != nil || c < a || d > b || c == d {
		return 0, 0, ErrPreparedDescribeV1
	}
	return a, b, nil
}
func preparedDescribeRecordValidV1(r preparedDescribeRecordV1) bool {
	if r.Schema != preparedDescribeSchemaV1 || r.Mode != "R" || !validJobV2(r.Producer) || r.RecordLimit == 0 || r.RecordLimit > preparedDescribeRecordMaxV1 || r.Created <= 0 || r.Created >= r.Expires || r.Expires > r.RetentionUntil || len(r.Document) == 0 || len(r.Document) > 1<<20 || len(r.Definition) == 0 || len(r.Definition) > 1<<20 || len(r.Input) == 0 || len(r.Input) > 1<<20 || len(r.Query) == 0 || len(r.Query) > 1<<20 || len(r.Predecessors) > 8 || r.Provenance.Generation == 0 {
		return false
	}
	for _, s := range []string{r.InitialHead, r.Decision, r.Workflow, r.CompletionScope, r.CaptureObserved, r.PreparationReserved, r.PreparationObserved, r.PreparationAttempt, r.PersistReserved, r.PersistAttempt, r.Provenance.SourceDigest} {
		if !hashV2(s) {
			return false
		}
	}
	if r.CompletionScope != hashLabelV2("prepared-describe/completion/v1\x00"+r.Workflow) {
		return false
	}
	for _, s := range []string{r.RootPath, r.RunID, r.SeedNamespace, r.Provenance.URI, r.Provenance.SessionID, r.Provenance.DocumentVersion, r.Provenance.SymbolID, r.Provenance.Workspace, r.Provenance.Revision, r.Provenance.Privacy} {
		if !preparedDescribeTextV1(s) {
			return false
		}
	}
	if !filepath.IsAbs(r.RootPath) || filepath.Clean(r.RootPath) != r.RootPath {
		return false
	}
	for _, b := range [][]byte{r.Question, r.Request, r.ResponseSchema} {
		if len(b) == 0 || len(b) > 128<<10 || !utf8.Valid(b) {
			return false
		}
	}
	for _, b := range [][]byte{r.Case, r.Result, r.Policy} {
		if len(b) > 8<<20 {
			return false
		}
	}
	for _, a := range r.Predecessors {
		if !preparedDescribeTextV1(a.Role) || !preparedDescribeTextV1(a.Selector) || !preparedDescribeTextV1(a.Schema) || len(a.Bytes) == 0 || len(a.Bytes) > 1<<20 {
			return false
		}
	}
	start, end, e := preparedDescribeOffsetsV1(r.Document, r.Provenance)
	return e == nil && digest(r.Document) == r.Provenance.SourceDigest && bytes.Equal(r.Definition, r.Document[start:end])
}
func preparedDescribeEncodeV1(v any, limit uint64) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil || uint64(len(b))+1 > limit {
		return nil, ErrPreparedDescribeV1
	}
	return append(b, '\n'), nil
}
func preparedDescribeDecodeV1(b []byte, v any, limit uint64) error {
	if len(b) == 0 || uint64(len(b)) > limit || !utf8.Valid(b) {
		return ErrPreparedDescribeV1
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return ErrPreparedDescribeV1
	}
	canonical, e := preparedDescribeEncodeV1(v, limit)
	if e != nil || !bytes.Equal(b, canonical) {
		return ErrPreparedDescribeV1
	}
	return nil
}
func preparedDescribeSelectorV1(kind, id string) string {
	return recordSelectorV2(preparedDescribeNamespaceV1+"/"+kind, id)
}
func preparedDescribeReceiptV1(r *publication.BoundFileReceipt, selector string, b []byte) bool {
	return r != nil && r.FinalSelector == selector && r.Digest == digest(b) && r.ByteLength == uint64(len(b)) && r.Mechanism == publication.BoundFileMechanism && r.NamespaceAtomic && r.VerificationStatus == "VERIFIED" && r.CloseStatus == publication.CloseComplete && r.DirectorySyncStatus == publication.DirectorySyncComplete
}

// Wrap the existing instance-local CAS seam with stricter receipt requirements;
// never change the borrowed owner's kernel or install global hooks.
func preparedDescribeKernelV1(o *PrivateManagedInputOwnerV2) *kernelV2 {
	k := *o.k
	cas := k.cas
	if cas == nil {
		cas = publication.CompareAndReplaceBoundFile
	}
	k.cas = func(ctx context.Context, root *publication.Root, s string, p publication.BoundFilePredecessor, b []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		r, e := cas(ctx, root, s, p, b, v)
		if r != nil && r.Committed && (!preparedDescribeReceiptV1(r.BoundFileReceipt, s, b) || e != nil || r.Outcome != publication.CASOutcomeCommitted) {
			return r, errUncertainV2
		}
		return r, e
	}
	return &k
}
func preparedDescribeReadV1(ctx context.Context, k *kernelV2, kind, id string, limit uint64) ([]byte, error) {
	if ctx.Err() != nil || !hashV2(id) {
		return nil, ErrPreparedDescribeV1
	}
	return publication.ReadVerifiedBoundFileChecked(k.root, preparedDescribeSelectorV1(kind, id), int64(limit))
}
func preparedDescribeAncestorV1(ctx context.Context, k *kernelV2, j jobV2, from resourceHeadV2, wanted string, b *verificationBudgetV2) error {
	for {
		if from.Identity == wanted {
			return nil
		}
		if from.Previous == noneV2 {
			return ErrPreparedDescribeV1
		}
		var p resourceHeadV2
		if k.read(ctx, recordSelectorV2(headsNamespaceV2, from.Previous), &p, b) != nil || p.Job.Identity != j.Identity {
			return ErrPreparedDescribeV1
		}
		from = p
	}
}
func preparedDescribeProducerV1(ctx context.Context, k *kernelV2, r preparedDescribeRecordV1, selected resourceHeadV2, observed string, l verificationLimitsV2) error {
	b, e := budgetV2(l)
	if e != nil {
		return e
	}
	var initial, capture, prep, ready, res resourceHeadV2
	for id, out := range map[string]*resourceHeadV2{r.InitialHead: &initial, r.CaptureObserved: &capture, r.PreparationReserved: &prep, r.PreparationObserved: &ready, r.PersistReserved: &res} {
		if k.read(ctx, recordSelectorV2(headsNamespaceV2, id), out, b) != nil || out.Job.Identity != r.Producer.Identity {
			return ErrPreparedDescribeV1
		}
	}
	if initial.Event != "INITIALIZE" || capture.Event != "OBSERVE" || capture.Disposition != "OBSERVED" || prep.Event != "RESERVE" || prep.Previous != capture.Identity || prep.Attempt != r.PreparationAttempt || ready.Event != "OBSERVE" || ready.Disposition != "OBSERVED" || ready.Previous != prep.Identity || ready.Attempt != prep.Attempt || res.Event != "RESERVE" || res.Previous != ready.Identity || res.Attempt != r.PersistAttempt || res.At != r.Created {
		return ErrPreparedDescribeV1
	}
	marker, e := k.root.ReadSelector(initialSelectorV2(r.Producer), headLimitV2)
	expected, e2 := sealV2(&initial)
	if e != nil || e2 != nil || !bytes.Equal(marker, expected) {
		return ErrPreparedDescribeV1
	}
	var a authorizationV2
	if k.read(ctx, res.AuthorizationSelector, &a, b) != nil || a.Decision != r.Decision {
		return ErrPreparedDescribeV1
	}
	if observed != "" {
		var h resourceHeadV2
		if k.read(ctx, recordSelectorV2(headsNamespaceV2, observed), &h, b) != nil || h.Event != "OBSERVE" || h.Disposition != "OBSERVED" || h.Previous != res.Identity || h.Attempt != res.Attempt {
			return ErrPreparedDescribeV1
		}
		return preparedDescribeAncestorV1(ctx, k, r.Producer, selected, observed, b)
	}
	if selected.Identity != res.Identity {
		return ErrPreparedDescribeV1
	}
	return nil
}
func preparedDescribeContextsV1(ctx, parent context.Context, deadline time.Time) (context.Context, context.CancelFunc, error) {
	if ctx == nil || parent == nil || ctx.Err() != nil || parent.Err() != nil || !time.Now().Before(deadline) {
		return nil, nil, ErrPreparedDescribeV1
	}
	control, cancel := context.WithDeadline(parent, deadline)
	stop := context.AfterFunc(ctx, cancel)
	return control, func() { stop(); cancel() }, nil
}

func CommitPreparedDescribeV1(ctx context.Context, op PreparedDescribePersistOperationV1) (PreparedDescribeV1, error) {
	fail := func() (PreparedDescribeV1, error) { return PreparedDescribeV1{}, ErrPreparedDescribeV1 }
	if op.state == nil {
		return fail()
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return fail()
	}
	s.used = true
	control, cancel, e := preparedDescribeContextsV1(ctx, s.parent, s.request.Deadline)
	if e != nil {
		return fail()
	}
	defer cancel()
	o := s.owner
	if o == nil {
		return fail()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	r := s.record
	if o.k == nil || o.uncertain || o.head != s.reservation || o.k.root.Path() != r.RootPath || !preparedDescribeRecordValidV1(r) || !time.Now().Before(time.Unix(0, r.Expires)) {
		return fail()
	}
	k := preparedDescribeKernelV1(o)
	head, e := k.reopen(control, o.job, o.limits)
	if e != nil || head.At > time.Now().UnixNano() || preparedDescribeProducerV1(control, k, r, head, "", o.limits) != nil {
		return fail()
	}
	data, closeData := context.WithCancel(control)
	raw, e := preparedDescribeEncodeV1(r, r.RecordLimit)
	if e != nil {
		closeData()
		return fail()
	}
	id := digest(raw)
	selector := preparedDescribeSelectorV1("manifests", id)
	receipt, e := publication.PublishBoundFile(k.root, selector, raw, func(b []byte) error {
		var x preparedDescribeRecordV1
		if preparedDescribeDecodeV1(b, &x, r.RecordLimit) != nil || !preparedDescribeRecordValidV1(x) {
			return ErrPreparedDescribeV1
		}
		return nil
	})
	// No reuse/repair path: even a byte-identical preexisting target refuses.
	if e != nil || !preparedDescribeReceiptV1(receipt, selector, raw) || data.Err() != nil {
		closeData()
		return fail()
	}
	got, e := preparedDescribeReadV1(data, k, "manifests", id, r.RecordLimit)
	closeData()
	if e != nil || !bytes.Equal(got, raw) || data.Err() != context.Canceled || control.Err() != nil {
		return fail()
	}
	// Data handles have closed and data child is canceled before genesis selection.
	// The borrowed root remains live for control CAS and native Observe.
	genesis := preparedDescribeGenesisV1{preparedDescribeSchemaV1 + "/genesis", r.Workflow, id, r.PersistReserved, r.Created}
	graw, e := preparedDescribeEncodeV1(genesis, recordLimitV2)
	if e != nil {
		return fail()
	}
	gid := digest(graw)
	gs := preparedDescribeSelectorV1("genesis", gid)
	gr, e := publication.PublishBoundFile(k.root, gs, graw, func(b []byte) error {
		var x preparedDescribeGenesisV1
		return preparedDescribeDecodeV1(b, &x, recordLimitV2)
	})
	if e != nil || !preparedDescribeReceiptV1(gr, gs, graw) {
		return fail()
	}
	selected := preparedDescribeSelectorV1("selected", r.Workflow)
	cas := k.cas
	cr, e := cas(control, k.root, selected, publication.BoundFilePredecessor{Absent: true}, graw, func(b []byte) error {
		var x preparedDescribeGenesisV1
		return preparedDescribeDecodeV1(b, &x, recordLimitV2)
	})
	if e != nil || cr == nil || !cr.Committed || cr.Outcome != publication.CASOutcomeCommitted {
		if cr != nil && cr.Committed {
			o.uncertain = true
		}
		return fail()
	}
	observed, e := k.observe(control, o.job, s.reservation, r.PersistAttempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if e != nil {
		o.uncertain = true
		return fail()
	}
	o.head = observed.Identity
	verified, e := k.reopen(control, o.job, o.limits)
	if e != nil || verified.Identity != observed.Identity || control.Err() != nil || ctx.Err() != nil || !time.Now().Before(s.request.Deadline) {
		o.uncertain = true
		return fail()
	}
	provenance, e := preparedDescribeEncodeV1(r.Provenance, 64<<10)
	if e != nil {
		return fail()
	}
	cancel()
	if control.Err() != context.Canceled || ctx.Err() != nil || k.root.ValidatePrivate() != nil {
		return fail()
	}
	anchors := PreparedDescribeAnchorsV1{r.RootPath, r.Producer.Identity, r.InitialHead, r.Workflow, id, gid, observed.Identity, r.CompletionScope}
	return PreparedDescribeV1{&preparedDescribeResultV1{anchors, bytes.Clone(r.Definition), provenance}}, nil
}

func preparedDescribeAnchorsValidV1(a PreparedDescribeAnchorsV1) bool {
	if len(a.RootPath) > 4096 || !filepath.IsAbs(a.RootPath) || filepath.Clean(a.RootPath) != a.RootPath {
		return false
	}
	for _, s := range []string{a.ProducerJob, a.InitialHead, a.Workflow, a.Manifest, a.Genesis, a.Observed, a.CompletionScope} {
		if !hashV2(s) {
			return false
		}
	}
	return a.CompletionScope == hashLabelV2("prepared-describe/completion/v1\x00"+a.Workflow)
}
func (o *PrivateManagedInputOwnerV2) ReservePreparedDescribeOpenV1(ctx context.Context, q PreparedDescribeOpenRequestV1) (PreparedDescribeOpenOperationV1, error) {
	fail := func() (PreparedDescribeOpenOperationV1, error) {
		return PreparedDescribeOpenOperationV1{}, ErrPreparedDescribeV1
	}
	now := time.Now()
	if o == nil || ctx == nil || ctx.Err() != nil || !hashV2(q.Attempt) || !hashV2(q.Decision) || !preparedDescribeAnchorsValidV1(q.Anchors) || !preparedDescribeWindowV1(now, q.Deadline, q.Expires) {
		return fail()
	}
	c, e := preparedDescribeChargeV1(q.RecordLimit, now, q.Deadline)
	if e != nil {
		return fail()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || o.k.root.Path() != q.Anchors.RootPath || o.job.Scope != q.Anchors.CompletionScope || o.job.Identity == q.Anchors.ProducerJob {
		return fail()
	}
	k := preparedDescribeKernelV1(o)
	h, e := k.reserve(ctx, o.job, o.head, q.Attempt, c, q.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if e != nil {
		if errors.Is(e, errUncertainV2) {
			o.uncertain = true
		}
		return fail()
	}
	o.head = h.Identity
	if ctx.Err() != nil || !time.Now().Before(q.Deadline) {
		return fail()
	}
	return PreparedDescribeOpenOperationV1{&preparedDescribeOpenV1{owner: o, request: q, reservation: h.Identity, parent: ctx}}, nil
}
func OpenPreparedDescribeV1(ctx context.Context, op PreparedDescribeOpenOperationV1) (OpenedPreparedDescribeV1, error) {
	fail := func() (OpenedPreparedDescribeV1, error) { return OpenedPreparedDescribeV1{}, ErrPreparedDescribeV1 }
	if op.state == nil {
		return fail()
	}
	s := op.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return fail()
	}
	s.used = true
	control, cancel, e := preparedDescribeContextsV1(ctx, s.parent, s.request.Deadline)
	if e != nil {
		return fail()
	}
	defer cancel()
	o := s.owner
	if o == nil {
		return fail()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	q := s.request
	a := q.Anchors
	if o.k == nil || o.uncertain || o.head != s.reservation || !time.Now().Before(q.Expires) {
		return fail()
	}
	k := preparedDescribeKernelV1(o)
	own, e := k.reopen(control, o.job, o.limits)
	if e != nil || own.Identity != s.reservation || own.At > time.Now().UnixNano() {
		return fail()
	}
	b, e := budgetV2(o.limits)
	if e != nil {
		return fail()
	}
	auth, e := k.currentAuthorization(control, own, b)
	if e != nil || auth.Decision != q.Decision {
		return fail()
	}
	data, closeData := context.WithCancel(control)
	defer closeData()
	graw, e := preparedDescribeReadV1(data, k, "genesis", a.Genesis, recordLimitV2)
	if e != nil || digest(graw) != a.Genesis {
		return fail()
	}
	var genesis preparedDescribeGenesisV1
	if preparedDescribeDecodeV1(graw, &genesis, recordLimitV2) != nil || genesis.Schema != preparedDescribeSchemaV1+"/genesis" || genesis.Manifest != a.Manifest || genesis.Workflow != a.Workflow {
		return fail()
	}
	selected, e := preparedDescribeReadV1(data, k, "selected", a.Workflow, recordLimitV2)
	if e != nil || !bytes.Equal(selected, graw) {
		return fail()
	}
	raw, e := preparedDescribeReadV1(data, k, "manifests", a.Manifest, preparedDescribeLimitV1(q.RecordLimit))
	if e != nil || digest(raw) != a.Manifest {
		return fail()
	}
	var r preparedDescribeRecordV1
	if preparedDescribeDecodeV1(raw, &r, preparedDescribeLimitV1(q.RecordLimit)) != nil || !preparedDescribeRecordValidV1(r) || r.Workflow != a.Workflow || r.CompletionScope != a.CompletionScope || r.RootPath != a.RootPath || r.InitialHead != a.InitialHead || r.Producer.Identity != a.ProducerJob || r.PersistReserved != genesis.Reservation || r.Created != genesis.At || r.Expires != q.Expires.UnixNano() || time.Now().UnixNano() < r.Created || !time.Now().Before(time.Unix(0, r.Expires)) {
		return fail()
	}
	producer, e := k.reopen(data, r.Producer, o.limits)
	if e != nil || producer.At > time.Now().UnixNano() || preparedDescribeProducerV1(data, k, r, producer, a.Observed, o.limits) != nil {
		return fail()
	}
	provenance, e := preparedDescribeEncodeV1(r.Provenance, 64<<10)
	if e != nil {
		return fail()
	}
	closeData()
	if data.Err() != context.Canceled || control.Err() != nil || ctx.Err() != nil {
		return fail()
	}
	observed, e := k.observe(control, o.job, s.reservation, q.Attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if e != nil {
		o.uncertain = true
		return fail()
	}
	o.head = observed.Identity
	verified, e := k.reopen(control, o.job, o.limits)
	if e != nil || verified.Identity != observed.Identity || ctx.Err() != nil || !time.Now().Before(q.Deadline) {
		o.uncertain = true
		return fail()
	}
	cancel()
	if control.Err() != context.Canceled || ctx.Err() != nil || k.root.ValidatePrivate() != nil {
		return fail()
	}
	return OpenedPreparedDescribeV1{&preparedDescribeResultV1{a, bytes.Clone(r.Definition), provenance}}, nil
}
