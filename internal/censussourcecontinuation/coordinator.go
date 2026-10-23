package censussourcecontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceplan"
)

const (
	headVersion    = "lsp-trace.census-source-continuation-head.private.v1"
	receiptVersion = "lsp-trace.census-source-continuation-receipt.private.v1"
)

type Outcome string

const (
	OutcomeCompleteNonempty Outcome = "COMPLETE_NONEMPTY"
	OutcomeSuccessfulEmpty  Outcome = "SUCCESSFUL_EMPTY"
	OutcomePartial          Outcome = "PARTIAL"
	OutcomeFailed           Outcome = "FAILED"
	OutcomeRefused          Outcome = "REFUSED"
	OutcomeUnknown          Outcome = "UNKNOWN"
)

type Status string

const (
	StatusCommitted              Status = "COMMITTED"
	StatusRetained               Status = "RETAINED"
	StatusConflict               Status = "CONFLICT"
	StatusIndeterminateExecution Status = "INDETERMINATE_EXECUTION"
)

type Terminal struct {
	PlanIdentity string
	Authority    uint8
	Ordinal      int
	SourceFiles  []censusacquisition.SourceFile
	Discovery    censusacquisition.Discovery
	Outcome      Outcome
}
type HeadRevision struct {
	Generation uint64
	Selector   string
}

// HeadState is the persisted coordinator state exposed by strict neutral inspection.
type HeadState string

const (
	HeadStateAbsent    HeadState = "ABSENT"
	HeadStateClaimed   HeadState = "CLAIMED"
	HeadStateCommitted HeadState = "COMMITTED"
)

// HeadInspection exposes only strictly validated persisted continuation custody.
type HeadInspection struct {
	Revision        HeadRevision
	State           HeadState
	CommittedPrefix int
	ClaimOrdinal    int
	ReceiptSelector string
}
type CASOutcome string

const (
	CASNotCommitted    CASOutcome = "NOT_COMMITTED"
	CASCommitted       CASOutcome = "COMMITTED"
	CASCommitUncertain CASOutcome = "COMMIT_UNCERTAIN"
)

type CASResult struct {
	Outcome CASOutcome
	Err     error
}
type ImmutableStore interface {
	Put(context.Context, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
}
type HeadStore interface {
	Load(context.Context, string) ([]byte, error)
	CompareAndSwap(context.Context, string, HeadRevision, []byte) CASResult
}
type Request struct {
	Plan                 censussourceplan.Plan
	PartitionSize        int
	WorkflowKey, HeadKey string
	Expected             HeadRevision
	ExistingOrdinalZero  *Terminal
	Objects              ImmutableStore
	Heads                HeadStore
	Discover             func(context.Context, int, []censusacquisition.SourceFile) (Terminal, error)
}
type Result struct {
	Status          Status
	CommittedPrefix int
	Ordinal         int
	ReceiptSelector string
	Head            HeadRevision
	Outcome         Outcome
}

var ErrInvalid = errors.New("invalid census source continuation")

type head struct {
	SchemaVersion   string `json:"schema_version"`
	Selector        string `json:"selector"`
	WorkflowKey     string `json:"workflow_key"`
	PlanIdentity    string `json:"plan_identity"`
	Authority       uint8  `json:"authority"`
	Generation      uint64 `json:"generation"`
	State           string `json:"state"`
	CommittedPrefix int    `json:"committed_prefix"`
	ClaimOrdinal    int    `json:"claim_ordinal"`
	ClaimGeneration uint64 `json:"claim_generation"`
	ClaimSelector   string `json:"claim_selector"`
	IdempotencyKey  string `json:"idempotency_key"`
	ReceiptSelector string `json:"receipt_selector"`
}
type receipt struct {
	SchemaVersion   string   `json:"schema_version"`
	WorkflowKey     string   `json:"workflow_key"`
	ClaimGeneration uint64   `json:"claim_generation"`
	ClaimSelector   string   `json:"claim_selector"`
	IdempotencyKey  string   `json:"idempotency_key"`
	Terminal        Terminal `json:"terminal"`
	NilPaths        []string `json:"nil_paths"`
}

func digest(b []byte) string       { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func encode(v any) ([]byte, error) { return json.Marshal(v) }
func strict(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalid
	}
	b, err := json.Marshal(v)
	if err != nil || !bytes.Equal(b, raw) {
		return ErrInvalid
	}
	return nil
}
func setHeadSelector(h *head) ([]byte, error) {
	h.Selector = ""
	b, e := encode(h)
	if e != nil {
		return nil, e
	}
	h.Selector = digest(b)
	return encode(h)
}
func ParseHeadRevision(raw []byte) (HeadRevision, error) {
	if len(raw) == 0 {
		return HeadRevision{}, nil
	}
	var h head
	if strict(raw, &h) != nil || h.SchemaVersion != headVersion {
		return HeadRevision{}, ErrInvalid
	}
	copy := h
	b, e := setHeadSelector(&copy)
	if e != nil || !bytes.Equal(b, raw) {
		return HeadRevision{}, ErrInvalid
	}
	return HeadRevision{h.Generation, h.Selector}, nil
}

// InspectHead strictly parses one persisted head and validates its request-bound custody.
func InspectHead(raw []byte, workflowKey, planIdentity string, partitions int) (HeadInspection, error) {
	if workflowKey == "" || planIdentity == "" || partitions < 0 {
		return HeadInspection{}, ErrInvalid
	}
	if len(raw) == 0 {
		return HeadInspection{State: HeadStateAbsent, ClaimOrdinal: -1}, nil
	}
	var h head
	if strict(raw, &h) != nil || h.SchemaVersion != headVersion || h.WorkflowKey != workflowKey || h.PlanIdentity != planIdentity || h.Authority != 0 || validateHead(h, partitions) != nil {
		return HeadInspection{}, ErrInvalid
	}
	copy := h
	canonical, err := setHeadSelector(&copy)
	if err != nil || !bytes.Equal(canonical, raw) {
		return HeadInspection{}, ErrInvalid
	}
	return HeadInspection{
		Revision:        HeadRevision{Generation: h.Generation, Selector: h.Selector},
		State:           HeadState(h.State),
		CommittedPrefix: h.CommittedPrefix,
		ClaimOrdinal:    h.ClaimOrdinal,
		ReceiptSelector: h.ReceiptSelector,
	}, nil
}

func validSelector(s string) bool {
	if len(s) != len("sha256:")+sha256.Size*2 || !bytes.HasPrefix([]byte(s), []byte("sha256:")) {
		return false
	}
	_, err := hex.DecodeString(s[len("sha256:"):])
	return err == nil
}
func validateHead(h head, partitions int) error {
	if h.Generation == ^uint64(0) || h.CommittedPrefix < 0 || h.CommittedPrefix > partitions || h.ClaimOrdinal < -1 {
		return ErrInvalid
	}
	switch h.State {
	case "ABSENT":
		if h.ClaimOrdinal != -1 || h.ClaimGeneration != 0 || h.ClaimSelector != "" || h.IdempotencyKey != "" || h.ReceiptSelector != "" {
			return ErrInvalid
		}
	case "CLAIMED":
		if h.CommittedPrefix >= partitions || h.ClaimOrdinal != h.CommittedPrefix || h.ClaimGeneration > h.Generation || h.IdempotencyKey == "" || (h.ClaimGeneration == 0) != (h.ClaimSelector == "") || h.ReceiptSelector != "" && !validSelector(h.ReceiptSelector) {
			return ErrInvalid
		}
	case "COMMITTED":
		if h.CommittedPrefix == 0 || h.ClaimOrdinal != h.CommittedPrefix-1 || h.ClaimGeneration >= h.Generation || h.IdempotencyKey == "" || (h.ClaimGeneration == 0) != (h.ClaimSelector == "") || !validSelector(h.ReceiptSelector) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
func nextGeneration(g uint64) (uint64, error) {
	if g == ^uint64(0) {
		return 0, ErrInvalid
	}
	return g + 1, nil
}
func nilPaths(v any) []string {
	var out []string
	var walk func(reflect.Value, string)
	walk = func(x reflect.Value, p string) {
		if !x.IsValid() {
			return
		}
		if x.Kind() == reflect.Interface {
			x = x.Elem()
		}
		switch x.Kind() {
		case reflect.Pointer:
			if x.IsNil() {
				out = append(out, p)
				return
			}
			walk(x.Elem(), p)
		case reflect.Slice:
			if x.IsNil() {
				out = append(out, p)
				return
			}
			for i := 0; i < x.Len(); i++ {
				walk(x.Index(i), fmt.Sprintf("%s[%d]", p, i))
			}
		case reflect.Struct:
			t := x.Type()
			for i := 0; i < x.NumField(); i++ {
				walk(x.Field(i), p+"."+t.Field(i).Name)
			}
		}
	}
	walk(reflect.ValueOf(v), "terminal")
	return out
}
func EncodeReceipt(workflow string, g uint64, selector, idempotency string, t Terminal) ([]byte, error) {
	if validateTerminalSemantics(t) != nil {
		return nil, ErrInvalid
	}
	r := receipt{receiptVersion, workflow, g, selector, idempotency, cloneTerminal(t), nil}
	r.NilPaths = nilPaths(r.Terminal)
	return encode(r)
}
func decodeReceipt(raw []byte, workflow string, p censussourceplan.Plan, ordinal int) (receipt, error) {
	var r receipt
	if strict(raw, &r) != nil || r.SchemaVersion != receiptVersion || r.WorkflowKey != workflow {
		return receipt{}, ErrInvalid
	}
	t := r.Terminal
	if t.PlanIdentity != p.Identity || t.Authority != 0 || t.Ordinal != ordinal || ordinal < 0 || ordinal >= len(p.Partitions) || !reflect.DeepEqual(t.SourceFiles, p.Partitions[ordinal].Members) || !reflect.DeepEqual(r.NilPaths, nilPaths(t)) || validateTerminalSemantics(t) != nil {
		return receipt{}, ErrInvalid
	}
	r.Terminal = cloneTerminal(t)
	return r, nil
}
func DecodeReceipt(raw []byte, workflow string, p censussourceplan.Plan, ordinal int) (Terminal, error) {
	r, err := decodeReceipt(raw, workflow, p, ordinal)
	return r.Terminal, err
}
func validOutcome(o Outcome) bool { return o == OutcomeCompleteNonempty || o == OutcomeSuccessfulEmpty }
func validateTerminalSemantics(t Terminal) error {
	switch t.Outcome {
	case OutcomeCompleteNonempty, OutcomeSuccessfulEmpty, OutcomePartial, OutcomeFailed, OutcomeRefused, OutcomeUnknown:
	default:
		return ErrInvalid
	}
	if censusacquisition.ValidateTerminalDiscovery(t.Discovery, string(t.Outcome)) != nil {
		return ErrInvalid
	}
	return nil
}
func validateTerminal(t Terminal, p censussourceplan.Plan, o int) error {
	if o < 0 || o >= len(p.Partitions) || t.PlanIdentity != p.Identity || t.Authority != 0 || t.Ordinal != o || !reflect.DeepEqual(t.SourceFiles, p.Partitions[o].Members) {
		return ErrInvalid
	}
	return validateTerminalSemantics(t)
}
func putVerified(ctx context.Context, s ImmutableStore, b []byte) (string, error) {
	k, e := s.Put(ctx, b)
	if e != nil {
		return "", e
	}
	got, e := s.Get(ctx, k)
	if e != nil || k != digest(b) || !bytes.Equal(got, b) {
		return "", ErrInvalid
	}
	return k, nil
}
func result(h head, status Status, o Outcome) Result {
	return Result{status, h.CommittedPrefix, h.ClaimOrdinal, h.ReceiptSelector, HeadRevision{h.Generation, h.Selector}, o}
}
func validateCASResult(cr CASResult) (CASOutcome, error) {
	switch cr.Outcome {
	case CASCommitUncertain:
		return cr.Outcome, nil
	case CASNotCommitted:
		if cr.Err != nil {
			return "", cr.Err
		}
		return cr.Outcome, nil
	case CASCommitted:
		if cr.Err != nil {
			return "", ErrInvalid
		}
		return cr.Outcome, nil
	default:
		return "", ErrInvalid
	}
}
func cas(ctx context.Context, r Request, w HeadRevision, n head) (head, CASResult, error) {
	raw, e := setHeadSelector(&n)
	if e != nil {
		return head{}, CASResult{}, e
	}
	cr := r.Heads.CompareAndSwap(ctx, r.HeadKey, w, raw)
	return n, cr, nil
}
func CoordinateNext(ctx context.Context, r Request) (Result, error) {
	if ctx == nil || r.Objects == nil || r.Heads == nil || r.WorkflowKey == "" || r.HeadKey == "" || censussourceplan.Validate(r.Plan, r.PartitionSize) != nil {
		return Result{}, ErrInvalid
	}
	raw, e := r.Heads.Load(ctx, r.HeadKey)
	if e != nil {
		return Result{}, e
	}
	var h head
	if len(raw) > 0 {
		if strict(raw, &h) != nil || h.SchemaVersion != headVersion || h.WorkflowKey != r.WorkflowKey || h.PlanIdentity != r.Plan.Identity || h.Authority != 0 {
			return Result{}, ErrInvalid
		}
		c := h
		b, _ := setHeadSelector(&c)
		if !bytes.Equal(b, raw) {
			return Result{}, ErrInvalid
		}
	} else {
		h = head{SchemaVersion: headVersion, WorkflowKey: r.WorkflowKey, PlanIdentity: r.Plan.Identity, Authority: 0, State: "ABSENT", ClaimOrdinal: -1}
	}
	if validateHead(h, len(r.Plan.Partitions)) != nil {
		return Result{}, ErrInvalid
	}
	cur := HeadRevision{h.Generation, h.Selector}
	if cur != r.Expected {
		return result(h, StatusConflict, ""), nil
	}
	if h.State == "CLAIMED" {
		if h.ReceiptSelector == "" {
			return result(h, StatusIndeterminateExecution, ""), nil
		}
		rr, e := r.Objects.Get(ctx, h.ReceiptSelector)
		if e != nil || digest(rr) != h.ReceiptSelector {
			return Result{}, ErrInvalid
		}
		receipt, e := decodeReceipt(rr, r.WorkflowKey, r.Plan, h.ClaimOrdinal)
		if e != nil || receipt.ClaimGeneration != h.ClaimGeneration || receipt.ClaimSelector != h.ClaimSelector || receipt.IdempotencyKey != h.IdempotencyKey {
			return Result{}, ErrInvalid
		}
		t := receipt.Terminal
		if !validOutcome(t.Outcome) {
			return result(h, StatusRetained, t.Outcome), nil
		}
		n := h
		n.Generation, e = nextGeneration(h.Generation)
		if e != nil {
			return Result{}, e
		}
		n.State = "COMMITTED"
		n.CommittedPrefix = h.ClaimOrdinal + 1
		nh, cr, e := cas(ctx, r, cur, n)
		if e != nil {
			return Result{}, e
		}
		casOutcome, e := validateCASResult(cr)
		if e != nil {
			return Result{}, e
		}
		if casOutcome == CASCommitUncertain {
			return result(nh, StatusIndeterminateExecution, t.Outcome), nil
		}
		if casOutcome == CASNotCommitted {
			return result(h, StatusConflict, t.Outcome), nil
		}
		return result(nh, StatusCommitted, t.Outcome), nil
	}
	ordinal := h.CommittedPrefix
	if ordinal >= len(r.Plan.Partitions) {
		return result(h, StatusCommitted, ""), nil
	}
	if ordinal == 0 {
		if r.ExistingOrdinalZero == nil {
			return result(h, StatusConflict, ""), nil
		}
		return commitTerminal(ctx, r, h, cur, cloneTerminal(*r.ExistingOrdinalZero), false)
	}
	claimGeneration, e := nextGeneration(h.Generation)
	if e != nil {
		return Result{}, e
	}
	id := fmt.Sprintf("%s:%d:%d", r.WorkflowKey, ordinal, claimGeneration)
	claim := h
	claim.Generation = claimGeneration
	claim.State = "CLAIMED"
	claim.ClaimOrdinal = ordinal
	claim.ClaimGeneration = claim.Generation
	claim.ClaimSelector = cur.Selector
	claim.IdempotencyKey = id
	claim.ReceiptSelector = ""
	ch, cr, e := cas(ctx, r, cur, claim)
	if e != nil {
		return Result{}, e
	}
	casOutcome, e := validateCASResult(cr)
	if e != nil {
		return Result{}, e
	}
	if casOutcome == CASCommitUncertain {
		return result(ch, StatusIndeterminateExecution, ""), nil
	}
	if casOutcome == CASNotCommitted {
		return result(h, StatusConflict, ""), nil
	}
	if r.Discover == nil {
		return result(ch, StatusIndeterminateExecution, ""), nil
	}
	t, e := r.Discover(ctx, ordinal, append([]censusacquisition.SourceFile(nil), r.Plan.Partitions[ordinal].Members...))
	if e != nil {
		return result(ch, StatusIndeterminateExecution, ""), nil
	}
	return commitTerminal(ctx, r, ch, HeadRevision{ch.Generation, ch.Selector}, t, true)
}
func commitTerminal(ctx context.Context, r Request, h head, w HeadRevision, t Terminal, claimed bool) (Result, error) {
	if e := validateTerminal(t, r.Plan, h.CommittedPrefix); e != nil {
		return Result{}, e
	}
	linkedGeneration, e := nextGeneration(h.Generation)
	if e != nil {
		return Result{}, e
	}
	committedGeneration, e := nextGeneration(linkedGeneration)
	if e != nil {
		return Result{}, e
	}
	if !claimed {
		h.ClaimOrdinal = t.Ordinal
		h.ClaimGeneration = h.Generation
		h.ClaimSelector = w.Selector
		h.IdempotencyKey = fmt.Sprintf("%s:%d:%d", r.WorkflowKey, t.Ordinal, h.Generation)
	}
	raw, e := EncodeReceipt(r.WorkflowKey, h.ClaimGeneration, h.ClaimSelector, h.IdempotencyKey, t)
	if e != nil {
		return Result{}, e
	}
	sel, e := putVerified(ctx, r.Objects, raw)
	if e != nil {
		return Result{}, e
	}
	linked := h
	linked.Generation = linkedGeneration
	linked.State = "CLAIMED"
	linked.ReceiptSelector = sel
	lh, cr, e := cas(ctx, r, w, linked)
	if e != nil {
		return Result{}, e
	}
	casOutcome, e := validateCASResult(cr)
	if e != nil {
		return Result{}, e
	}
	if casOutcome == CASCommitUncertain {
		return result(lh, StatusIndeterminateExecution, t.Outcome), nil
	}
	if casOutcome == CASNotCommitted {
		return result(h, StatusConflict, t.Outcome), nil
	}
	if !validOutcome(t.Outcome) {
		return result(lh, StatusRetained, t.Outcome), nil
	}
	comm := lh
	comm.Generation = committedGeneration
	comm.State = "COMMITTED"
	comm.CommittedPrefix = t.Ordinal + 1
	fh, cr, e := cas(ctx, r, HeadRevision{lh.Generation, lh.Selector}, comm)
	if e != nil {
		return Result{}, e
	}
	casOutcome, e = validateCASResult(cr)
	if e != nil {
		return Result{}, e
	}
	if casOutcome == CASCommitUncertain {
		return result(fh, StatusIndeterminateExecution, t.Outcome), nil
	}
	if casOutcome == CASNotCommitted {
		return result(lh, StatusConflict, t.Outcome), nil
	}
	return result(fh, StatusCommitted, t.Outcome), nil
}
func cloneTerminal(t Terminal) Terminal {
	b, _ := json.Marshal(t)
	var c Terminal
	_ = json.Unmarshal(b, &c)
	return c
}
