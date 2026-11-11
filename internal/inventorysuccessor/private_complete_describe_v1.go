package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	dw "lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
	"strings"
	"sync"
	"time"
)

const PrivateDescribeRequestSchemaV1 = "lsp-trace.private-describe-request.v1"
const PrivateDescribeResponseMarkerV1 = `{"schema":"lsp-trace.private-describe-response.semantic-v2.v1"}`
const PrivateDescribeSuppliedSchemaV1 = "lsp-trace.private-describe-supplied.v1"
const completeDescribeLimitV1 = 1 << 20

var ErrCompleteDescribeV1 = errors.New("private Complete Describe refused or consumed pending uncertain")

type PrivateDescribeRequestV1 struct {
	Schema               string `json:"schema"`
	Mode                 string `json:"mode"`
	QuestionSHA256       string `json:"question_sha256"`
	ResponseSchemaSHA256 string `json:"response_schema_sha256"`
}

// Pure construction before Prepare. Hashes establish equality, not admission.
func NewPrivateDescribeRequestV1(question []byte) (request, responseSchema []byte, err error) {
	if len(question) == 0 || len(question) > 128<<10 {
		return nil, nil, ErrCompleteDescribeV1
	}
	responseSchema = []byte(PrivateDescribeResponseMarkerV1)
	request, err = json.Marshal(PrivateDescribeRequestV1{PrivateDescribeRequestSchemaV1, "R", digest(question), digest(responseSchema)})
	return
}

type CompleteDescribeBindingsV1 struct {
	QuestionSHA256       string `json:"question_sha256"`
	RequestSHA256        string `json:"request_sha256"`
	ResponseSchemaSHA256 string `json:"response_schema_sha256"`
	DefinitionSHA256     string `json:"definition_sha256"`
	ProvenanceSHA256     string `json:"provenance_sha256"`
}
type CompleteDescribeSuppliedV1 struct {
	Schema   string                     `json:"schema"`
	Bindings CompleteDescribeBindingsV1 `json:"bindings"`
	Adapter  []byte                     `json:"adapter"`
}

// All capabilities and anchors are independently admitted TRUSTED inputs. The
// protected set must be complete; no packet or this checker can establish that.
// Bootstrap and capability acquisition are separately trusted caller-funded.
// All capabilities are borrowed, NONCONCURRENT with external use or Close.
type CompleteDescribeRequestV1 struct {
	Root                  *publication.Root
	Open                  PreparedDescribeOpenRequestV1
	ExpectedHead, Attempt string
	Outputs               [2]*publication.ContentRetentionRoot
	DestinationIdentities [2]publication.RetentionDirectoryIdentity
	Source                *publication.SourceProtectionRootPrivateV2
	Protected             []*publication.ObservedRoot
}
type CompleteDescribeOperationV1 struct{ state *completeDescribeOperationV1 }
type completeDescribeOperationV1 struct {
	mu          sync.Mutex
	used        bool
	owner       *PrivateManagedInputOwnerV2
	request     CompleteDescribeRequestV1
	opened      OpenedPreparedDescribeV1
	reservation string
	parent      context.Context
}
type CompleteDescribeObjectV1 struct {
	Schema                                                                       string
	Authority                                                                    int
	Accepted                                                                     bool
	FeatureIdentity, Completeness, AbsenceInference, HOSTCost                    string
	Origin, Execution                                                            string
	RoleLabels                                                                   [2]string
	Anchors                                                                      PreparedDescribeAnchorsV1
	CompletionJob, Reservation                                                   string
	Destinations                                                                 [2]publication.RetentionDirectoryIdentity
	Bindings                                                                     CompleteDescribeBindingsV1
	Question, Request, ResponseSchema, Definition, Provenance, Adapter, Semantic []byte
	AdapterSHA256, SemanticSHA256                                                string
}
type completeDescribeSelectionV1 struct {
	Schema                                                    string
	Anchors                                                   PreparedDescribeAnchorsV1
	Bindings                                                  CompleteDescribeBindingsV1
	AdapterSHA256, SemanticSHA256, CompletionJob, Reservation string
	Destinations                                              [2]publication.RetentionDirectoryIdentity
	ObjectID                                                  string
	ObjectBytes                                               uint64
}

// Detached result; output availability still requires the outer cleanup barrier.
type CompletedDescribeV1 struct {
	Authority                                                 int
	Accepted                                                  bool
	FeatureIdentity, Completeness, AbsenceInference, HOSTCost string
	ObjectID, SelectionSelector                               string
	ObjectBytes                                               uint64
	Destinations                                              [2]publication.RetentionDirectoryIdentity
	Copies                                                    [2]publication.ProducerPacketCopyObservationPrivateV2
}

func completeDescribeDecodeV1(raw []byte, v any, limit int) error {
	if len(raw) == 0 || len(raw) > limit {
		return ErrCompleteDescribeV1
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return ErrCompleteDescribeV1
	}
	canonical, e := json.Marshal(v)
	if e != nil || !bytes.Equal(raw, canonical) {
		return ErrCompleteDescribeV1
	}
	return nil
}

// Closed quote = 4 * the unchanged conservative 16MiB prepared Describe quote,
// with checked native addition. Complete's verifier/Open has its OWN reservation.
// <=15 protected roots, <=256 components/root, <=8 topology checks and two
// 1MiB publish/readback waves fit the base's fixed snapshot/record/work waves.
// Four waves cover bounded adapter/semantic parsing, original clones, canonical
// expansion/rejection, hashing, global selection/write/readback and native final
// Observe/reopen. Five provider/wire currencies remain zero. No retries/refunds.
func completeDescribeChargeV1(now, deadline time.Time) (countersV2, error) {
	base, e := preparedDescribeChargeV1(preparedDescribeRecordMaxV1, now, deadline)
	if e != nil {
		return countersV2{}, e
	}
	var c countersV2
	for i := 0; i < 4; i++ {
		c, e = addCountersV2(c, base)
		if e != nil {
			return countersV2{}, e
		}
	}
	return c, nil
}
func (o *PrivateManagedInputOwnerV2) ReserveCompleteDescribeV1(ctx context.Context, q CompleteDescribeRequestV1) (CompleteDescribeOperationV1, error) {
	fail := func() (CompleteDescribeOperationV1, error) {
		return CompleteDescribeOperationV1{}, ErrCompleteDescribeV1
	}
	if o == nil || ctx == nil || ctx.Err() != nil || !hashV2(q.Attempt) || !hashV2(q.ExpectedHead) || len(q.Protected) < 1 || len(q.Protected) > 15 {
		return fail()
	}
	o.mu.Lock()
	valid := o.k != nil && o.k.root == q.Root && o.head == q.ExpectedHead && !o.uncertain
	o.mu.Unlock()
	if !valid {
		return fail()
	}
	// Native Open separately pays retained proof reads and its new opaque metadata.
	op, e := o.ReservePreparedDescribeOpenV1(ctx, q.Open)
	if e != nil {
		return fail()
	}
	opened, e := OpenPreparedDescribeV1(ctx, op)
	if e != nil {
		return fail()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	now := time.Now()
	charge, e := completeDescribeChargeV1(now, q.Open.Deadline)
	if e != nil {
		return fail()
	}
	k := preparedDescribeKernelV1(o)
	h, e := k.reserve(ctx, o.job, o.head, q.Attempt, charge, q.Open.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if e != nil {
		if errors.Is(e, errUncertainV2) {
			o.uncertain = true
		}
		return fail()
	}
	o.head = h.Identity
	// No supplied byte copy, parse, hash, topology or destination inspection before
	// the NEW Complete reservation. Failed admission never refunds the prior Open.
	q.Protected = append([]*publication.ObservedRoot(nil), q.Protected...)
	if ctx.Err() != nil || !time.Now().Before(q.Open.Deadline) {
		return fail()
	}
	return CompleteDescribeOperationV1{&completeDescribeOperationV1{owner: o, request: q, opened: opened, reservation: h.Identity, parent: ctx}}, nil
}
func completeDescribeTopologyV1(ctx context.Context, q CompleteDescribeRequestV1, workspace string) error {
	if q.Root == nil || q.Root.ValidatePrivate() != nil {
		return ErrCompleteDescribeV1
	}
	source, e := q.Source.PathPrivateV2()
	if e != nil || source != workspace {
		return ErrCompleteDescribeV1
	}
	found := false
	for _, p := range q.Protected {
		path, _, e := p.CanonicalIdentityPrivateV2()
		if e != nil {
			return e
		}
		if path == q.Root.Path() {
			found = true
		}
	}
	if !found {
		return ErrCompleteDescribeV1
	}
	for i, r := range q.Outputs {
		id, e := r.DirectoryIdentity()
		if e != nil || id != q.DestinationIdentities[i] || id.Inode == 0 {
			return ErrCompleteDescribeV1
		}
	}
	return publication.CheckProducerPacketRetentionTopologyPrivateV2(ctx, q.Outputs, q.Source, q.Protected)
}
func completeDescribeSelectorV1(a PreparedDescribeAnchorsV1) string {
	// Trusted producer anchors, independent of the new completion job or answer.
	key := digest([]byte("lsp-trace/private-complete-describe/workflow/v1\x00" + a.ProducerJob + "\x00" + a.Genesis + "\x00" + a.Manifest + "\x00" + a.Workflow))
	return ".private-complete-describe-v1-" + strings.TrimPrefix(key, "sha256:") + ".json"
}
func completeDescribeReceiptV1(r *publication.BoundFileReceipt, selector string, raw []byte) bool {
	return preparedDescribeReceiptV1(r, selector, raw) && r.CrashDurability == publication.DirectorySyncComplete
}
func completeDescribeSelectV1(ctx context.Context, q CompleteDescribeRequestV1, raw []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	selector := completeDescribeSelectorV1(q.Open.Anchors)
	receipt, e := publication.PublishBoundFile(q.Root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return ErrCompleteDescribeV1
		}
		return nil
	})
	// Every existing selection refuses, even an identical answer under a fresh
	// completion job. The flat absent-only install is the global arbitration.
	if e != nil || !completeDescribeReceiptV1(receipt, selector, raw) || ctx.Err() != nil {
		return ErrCompleteDescribeV1
	}
	got, e := publication.ReadVerifiedBoundFileChecked(q.Root, selector, completeDescribeLimitV1)
	if e != nil || !bytes.Equal(got, raw) || ctx.Err() != nil {
		return ErrCompleteDescribeV1
	}
	return nil
}

// CompleteDescribeV1 borrows immutable supplied bytes AFTER native reservation.
// Never mutate the bytes/capabilities concurrently. A failed read or abandoned
// reservation keeps its debit. Successful pair visibility is not atomic, a crash
// guarantee, or semantic acceptance; outer owners must close before availability.
func CompleteDescribeV1(ctx context.Context, op CompleteDescribeOperationV1, submission []byte) (CompletedDescribeV1, error) {
	fail := func() (CompletedDescribeV1, error) { return CompletedDescribeV1{}, ErrCompleteDescribeV1 }
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
	control, cancel, e := preparedDescribeContextsV1(ctx, s.parent, s.request.Open.Deadline)
	if e != nil {
		return fail()
	}
	defer cancel()
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	q := s.request
	if o.k == nil || o.uncertain || o.head != s.reservation || o.k.root != q.Root || s.opened.originals == nil || s.opened.state == nil || !time.Now().Before(q.Open.Expires) {
		return fail()
	}
	original := s.opened.originals
	retained := s.opened.state
	var req PrivateDescribeRequestV1
	if completeDescribeDecodeV1(original.request, &req, 128<<10) != nil || req.Schema != PrivateDescribeRequestSchemaV1 || req.Mode != "R" || req.QuestionSHA256 != digest(original.question) || req.ResponseSchemaSHA256 != digest(original.responseSchema) || string(original.responseSchema) != PrivateDescribeResponseMarkerV1 {
		return fail()
	}
	binding := CompleteDescribeBindingsV1{digest(original.question), digest(original.request), digest(original.responseSchema), digest(retained.definition), digest(retained.provenance)}
	var supplied CompleteDescribeSuppliedV1
	if completeDescribeDecodeV1(submission, &supplied, completeDescribeLimitV1) != nil || supplied.Schema != PrivateDescribeSuppliedSchemaV1 || supplied.Bindings != binding || len(supplied.Adapter) > 512<<10 {
		return fail()
	}
	candidate, e := dw.ParseSuppliedTerminalResponse(supplied.Adapter, dw.SuppliedResponseLimits{MaxAdapterBytes: 512 << 10, MaxSemanticBytes: 64 << 10})
	if e != nil {
		return fail()
	}
	var provenance preparedDescribeProvenanceV1
	if preparedDescribeDecodeV1(retained.provenance, &provenance, 64<<10) != nil || completeDescribeTopologyV1(control, q, provenance.Workspace) != nil {
		return fail()
	}
	obj := CompleteDescribeObjectV1{Schema: "lsp-trace.private-complete-describe-object.v1", FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN", AbsenceInference: "PROHIBITED", HOSTCost: "UNKNOWN_NOT_FREE", Origin: dw.SuppliedTerminalResponseOnly, Execution: dw.NoProcessOrHostExecutionVerified, RoleLabels: [2]string{"SOURCE_PROVIDER_ATTRIBUTION_ONLY", "EVALUATOR_ATTRIBUTION_ONLY"}, Anchors: q.Open.Anchors, CompletionJob: o.job.Identity, Reservation: s.reservation, Destinations: q.DestinationIdentities, Bindings: binding, Question: original.question, Request: original.request, ResponseSchema: original.responseSchema, Definition: retained.definition, Provenance: retained.provenance, Adapter: supplied.Adapter, Semantic: candidate.CanonicalSemantic(), AdapterSHA256: candidate.RawAdapterDigest(), SemanticSHA256: candidate.CanonicalSemanticDigest()}
	raw, e := preparedDescribeEncodeV1(obj, completeDescribeLimitV1)
	if e != nil {
		return fail()
	}
	id := digest(raw)
	selection := completeDescribeSelectionV1{"lsp-trace.private-complete-describe-selection.v1", q.Open.Anchors, binding, obj.AdapterSHA256, obj.SemanticSHA256, o.job.Identity, s.reservation, q.DestinationIdentities, id, uint64(len(raw))}
	selected, e := preparedDescribeEncodeV1(selection, completeDescribeLimitV1)
	if e != nil {
		return fail()
	}
	if completeDescribeSelectV1(control, q, selected) != nil {
		return fail()
	}
	// Selection is permanent. ANY subsequent failure is consumed; neither this
	// operation nor a later completion job may finish a missing object or refund.
	for _, r := range q.Outputs {
		if completeDescribeTopologyV1(control, q, provenance.Workspace) != nil {
			return fail()
		}
		receipt, e := r.Publish(control, id, raw)
		if e != nil || !completeDescribeReceiptV1(receipt, id, raw) {
			return fail()
		}
		got, e := r.Read(control, id, int64(len(raw)))
		if e != nil || !bytes.Equal(got, raw) {
			return fail()
		}
	}
	if completeDescribeTopologyV1(control, q, provenance.Workspace) != nil {
		return fail()
	}
	copies, e := publication.ObserveProducerPacketPairCustodyPrivateV2(control, q.Outputs, id)
	if e != nil {
		return fail()
	}
	k := preparedDescribeKernelV1(o)
	observed, e := k.observe(control, o.job, s.reservation, q.Attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if e != nil {
		o.uncertain = true
		return fail()
	}
	o.head = observed.Identity
	checked, e := k.reopen(control, o.job, o.limits)
	if e != nil || checked.Identity != observed.Identity {
		o.uncertain = true
		return fail()
	}
	// Observe success never repairs selection after a later topology/terminal fault.
	terminalErr := errors.Join(control.Err(), ctx.Err(), s.parent.Err(), completeDescribeTopologyV1(control, q, provenance.Workspace))
	cancel()
	if terminalErr != nil || control.Err() != context.Canceled || !time.Now().Before(q.Open.Deadline) || !time.Now().Before(q.Open.Expires) || q.Root.ValidatePrivate() != nil {
		return fail()
	}
	return CompletedDescribeV1{FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN", AbsenceInference: "PROHIBITED", HOSTCost: "UNKNOWN_NOT_FREE", ObjectID: id, SelectionSelector: completeDescribeSelectorV1(q.Open.Anchors), ObjectBytes: uint64(len(raw)), Destinations: q.DestinationIdentities, Copies: copies}, nil
}
