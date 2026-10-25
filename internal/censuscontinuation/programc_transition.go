package censuscontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/programc"
)

const (
	ProgramCTransitionPlanVersion    = "lsp-trace.census-program-c-transition-plan.private.v1"
	programCTransitionHeadVersion    = "lsp-trace.census-program-c-transition-head.private.v2"
	programCTransitionReceiptVersion = "lsp-trace.census-program-c-transition-receipt.private.v3"
)

var ErrInvalidProgramCTransition = errors.New("invalid census program-c transition")

type ProgramCTransitionStatus string

const (
	ProgramCTransitionCommitted     ProgramCTransitionStatus = "COMMITTED"
	ProgramCTransitionComplete      ProgramCTransitionStatus = "COMPLETE"
	ProgramCTransitionRetained      ProgramCTransitionStatus = "RETAINED"
	ProgramCTransitionConflict      ProgramCTransitionStatus = "CONFLICT"
	ProgramCTransitionIndeterminate ProgramCTransitionStatus = "INDETERMINATE_EXECUTION"
)

type ProgramCTransitionPlanRecord struct {
	Ordinal               int    `json:"ordinal"`
	ReceiptOrdinal        int    `json:"receipt_ordinal"`
	PublicationByteLength int64  `json:"publication_byte_length"`
	BatchOrdinal          int    `json:"batch_ordinal"`
	PublicationSelector   string `json:"publication_selector"`
	ConstituentSelector   string `json:"constituent_selector"`
}
type ProgramCTransitionPlan struct {
	SchemaVersion       string                         `json:"schema_version"`
	Authority           uint8                          `json:"authority"`
	CensusReceiptSHA256 string                         `json:"census_receipt_sha256"`
	LedgerSHA256        string                         `json:"ledger_sha256"`
	Records             []ProgramCTransitionPlanRecord `json:"records"`
}
type ProgramCTransitionPlanSource interface {
	Load(context.Context) ([]byte, error)
}

// NonAuthoritativeCanonicalProgramCTransitionPlanSource is a test-only/raw
// canonical source. Production hosts must use AuthoritativeProgramCTransitionPlanSource.
type NonAuthoritativeCanonicalProgramCTransitionPlanSource struct{ Raw []byte }

func (s NonAuthoritativeCanonicalProgramCTransitionPlanSource) Load(context.Context) ([]byte, error) {
	return bytes.Clone(s.Raw), nil
}

func EncodeProgramCTransitionPlan(p ProgramCTransitionPlan) ([]byte, error) {
	if p.SchemaVersion != ProgramCTransitionPlanVersion || p.Authority != 0 || !transitionValidSHA(p.CensusReceiptSHA256) || !transitionValidSHA(p.LedgerSHA256) || p.Records == nil {
		return nil, ErrInvalidProgramCTransition
	}
	for i, r := range p.Records {
		if r.Ordinal != i || r.ReceiptOrdinal <= 0 || r.PublicationByteLength <= 0 || r.BatchOrdinal < 0 || captureset.ValidatePublicationSelector(r.PublicationSelector) != nil || !transitionValidConstituentSelector(r.ConstituentSelector) {
			return nil, ErrInvalidProgramCTransition
		}
		if i > 0 {
			previous := p.Records[i-1]
			if previous.PublicationByteLength > r.PublicationByteLength ||
				previous.PublicationByteLength == r.PublicationByteLength && previous.ReceiptOrdinal > r.ReceiptOrdinal ||
				previous.PublicationByteLength == r.PublicationByteLength && previous.ReceiptOrdinal == r.ReceiptOrdinal && previous.BatchOrdinal >= r.BatchOrdinal {
				return nil, ErrInvalidProgramCTransition
			}
		}
	}
	return json.Marshal(p)
}
func decodeProgramCTransitionPlan(raw []byte) (ProgramCTransitionPlan, error) {
	var p ProgramCTransitionPlan
	if transitionStrict(raw, &p) != nil {
		return p, ErrInvalidProgramCTransition
	}
	canonical, e := EncodeProgramCTransitionPlan(p)
	if e != nil || !bytes.Equal(canonical, raw) {
		return p, ErrInvalidProgramCTransition
	}
	return p, nil
}

type ProgramCTransitionBindings struct {
	RepositoryCommit     string    `json:"repository_commit"`
	RepositoryTree       string    `json:"repository_tree"`
	FrozenCorpusSHA256   string    `json:"frozen_corpus_sha256"`
	CensusReceiptSHA256  string    `json:"census_receipt_sha256"`
	LedgerSHA256         string    `json:"ledger_sha256"`
	PlanSHA256           string    `json:"plan_sha256"`
	AlgorithmVersion     string    `json:"algorithm_version"`
	Seeds                [2]uint64 `json:"seeds"`
	ReplaysPerSeed       uint8     `json:"replays_per_seed"`
	ParametersSHA256     string    `json:"parameters_sha256"`
	ResourcePolicySHA256 string    `json:"resource_policy_sha256"`
}
type ProgramCTransitionRunner interface {
	Resolve(context.Context, string, string) ([]byte, error)
	Compute([]byte, uint64) (programc.Outcome, *programc.Failure)
	Instability([]programc.Outcome, ProgramCTransitionBindings) ([]byte, error)
	ValidateInstability([]byte, []programc.Outcome, ProgramCTransitionBindings) error
}
type ProgramCTransitionRequest struct {
	WorkflowKey string
	HeadKey     string
	Expected    censussourcecontinuation.HeadRevision
	Bindings    ProgramCTransitionBindings
	PlanSource  ProgramCTransitionPlanSource
	Objects     censussourcecontinuation.ImmutableStore
	Heads       censussourcecontinuation.HeadStore
	Runner      ProgramCTransitionRunner
}
type ProgramCTransitionResult struct {
	Status           ProgramCTransitionStatus
	Head             censussourcecontinuation.HeadRevision
	ReceiptSelector  string
	CommittedPrefix  int
	ConstituentCount int
	Ordinal          int
	ProgramCOutcome  string
	FailureStage     ProgramCTransitionFailureStage
	Failure          string
	ResultSHA256     string
	ResultByteLength int
}

type ProgramCTransitionFailureStage string

const (
	ProgramCTransitionFailureResolve             ProgramCTransitionFailureStage = "RESOLVE"
	ProgramCTransitionFailureCompute             ProgramCTransitionFailureStage = "PROGRAM_C_COMPUTE"
	ProgramCTransitionFailureInstabilityBuild    ProgramCTransitionFailureStage = "INSTABILITY_BUILD"
	ProgramCTransitionFailureInstabilityValidate ProgramCTransitionFailureStage = "INSTABILITY_VALIDATE"
)

type programCTransitionHead struct {
	SchemaVersion            string                     `json:"schema_version"`
	Selector                 string                     `json:"selector"`
	WorkflowKey              string                     `json:"workflow_key"`
	HeadKey                  string                     `json:"head_key"`
	Authority                uint8                      `json:"authority"`
	Generation               uint64                     `json:"generation"`
	State                    string                     `json:"state"`
	Bindings                 ProgramCTransitionBindings `json:"bindings"`
	CommittedPrefix          int                        `json:"committed_prefix"`
	ConstituentCount         int                        `json:"constituent_count"`
	ClaimOrdinal             int                        `json:"claim_ordinal"`
	ClaimGeneration          uint64                     `json:"claim_generation"`
	ClaimSelector            string                     `json:"claim_selector"`
	ClaimPublicationSelector string                     `json:"claim_publication_selector"`
	ClaimConstituentSelector string                     `json:"claim_constituent_selector"`
	ReceiptSelector          string                     `json:"receipt_selector"`
}
type programCTransitionReceipt struct {
	SchemaVersion           string                         `json:"schema_version"`
	WorkflowKey             string                         `json:"workflow_key"`
	HeadKey                 string                         `json:"head_key"`
	Authority               uint8                          `json:"authority"`
	Bindings                ProgramCTransitionBindings     `json:"bindings"`
	ConstituentCount        int                            `json:"constituent_count"`
	Ordinal                 int                            `json:"ordinal"`
	ReceiptOrdinal          int                            `json:"receipt_ordinal"`
	PublicationByteLength   int64                          `json:"publication_byte_length"`
	BatchOrdinal            int                            `json:"batch_ordinal"`
	PublicationSelector     string                         `json:"publication_selector"`
	ConstituentSelector     string                         `json:"constituent_selector"`
	ClaimGeneration         uint64                         `json:"claim_generation"`
	ClaimSelector           string                         `json:"claim_selector"`
	Outcome                 string                         `json:"outcome"`
	ProgramCOutcome         string                         `json:"program_c_outcome"`
	FailureStage            ProgramCTransitionFailureStage `json:"failure_stage"`
	Failure                 string                         `json:"failure"`
	ResultSHA256            string                         `json:"result_sha256"`
	ResultByteLength        int                            `json:"result_byte_length"`
	ResultBytes             []byte                         `json:"result_bytes"`
	InstabilitySchema       string                         `json:"instability_schema"`
	InstabilityClaimCeiling string                         `json:"instability_claim_ceiling"`
}

func transitionSHA(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
func transitionStrict(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if e := d.Decode(&struct{}{}); e != io.EOF {
		return ErrInvalidProgramCTransition
	}
	b, e := json.Marshal(v)
	if e != nil || !bytes.Equal(b, raw) {
		return ErrInvalidProgramCTransition
	}
	return nil
}
func transitionSelector(h *programCTransitionHead) ([]byte, error) {
	h.Selector = ""
	b, e := json.Marshal(h)
	if e != nil {
		return nil, e
	}
	h.Selector = transitionSHA(b)
	return json.Marshal(h)
}
func ParseProgramCTransitionHeadRevision(raw []byte) (censussourcecontinuation.HeadRevision, error) {
	if len(raw) == 0 {
		return censussourcecontinuation.HeadRevision{}, nil
	}
	var h programCTransitionHead
	if transitionStrict(raw, &h) != nil || h.SchemaVersion != programCTransitionHeadVersion {
		return censussourcecontinuation.HeadRevision{}, ErrInvalidProgramCTransition
	}
	c := h
	b, e := transitionSelector(&c)
	if e != nil || !bytes.Equal(b, raw) {
		return censussourcecontinuation.HeadRevision{}, ErrInvalidProgramCTransition
	}
	return censussourcecontinuation.HeadRevision{Generation: h.Generation, Selector: h.Selector}, nil
}
func transitionValidSHA(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	digest := s[7:]
	if strings.ToLower(digest) != digest {
		return false
	}
	_, e := hex.DecodeString(digest)
	return e == nil
}
func transitionValidConstituentSelector(s string) bool {
	if !strings.HasPrefix(s, captureset.ConstituentSelectorPrefix) {
		return false
	}
	d := strings.TrimPrefix(s, captureset.ConstituentSelectorPrefix)
	if len(d) != 64 {
		return false
	}
	_, e := hex.DecodeString(d)
	return e == nil
}
func transitionValidRepositoryRevision(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			if c < 'a' || c > 'f' {
				return false
			}
		}
	}
	return true
}
func transitionValidBindings(b ProgramCTransitionBindings, p ProgramCTransitionPlan, raw []byte) bool {
	return transitionValidRepositoryRevision(b.RepositoryCommit) && transitionValidRepositoryRevision(b.RepositoryTree) && transitionValidSHA(b.FrozenCorpusSHA256) && b.CensusReceiptSHA256 == p.CensusReceiptSHA256 && b.LedgerSHA256 == p.LedgerSHA256 && b.PlanSHA256 == transitionSHA(raw) && b.AlgorithmVersion != "" && b.Seeds == [2]uint64{1, 2} && b.ReplaysPerSeed == 3 && transitionValidSHA(b.ParametersSHA256) && transitionValidSHA(b.ResourcePolicySHA256)
}
func transitionValidateHead(h programCTransitionHead, r ProgramCTransitionRequest, count int) error {
	if h.SchemaVersion != programCTransitionHeadVersion || h.WorkflowKey != r.WorkflowKey || h.HeadKey != r.HeadKey || h.Authority != 0 || h.Bindings != r.Bindings || h.Generation == math.MaxUint64 || h.ConstituentCount != count || h.CommittedPrefix < 0 || h.CommittedPrefix > count || h.ClaimOrdinal < -1 {
		return ErrInvalidProgramCTransition
	}
	switch h.State {
	case "ABSENT":
		if h.Generation != 0 || h.CommittedPrefix != 0 || h.ClaimOrdinal != -1 || h.ClaimGeneration != 0 || h.ClaimSelector != "" || h.ClaimPublicationSelector != "" || h.ClaimConstituentSelector != "" || h.ReceiptSelector != "" {
			return ErrInvalidProgramCTransition
		}
	case "CLAIMED":
		if h.ClaimOrdinal != h.CommittedPrefix || h.ClaimOrdinal >= count || h.ClaimGeneration == 0 || h.ClaimGeneration > h.Generation || (h.ClaimSelector == "" && h.ClaimGeneration != 1) || captureset.ValidatePublicationSelector(h.ClaimPublicationSelector) != nil || !transitionValidConstituentSelector(h.ClaimConstituentSelector) || (h.ReceiptSelector != "" && !transitionValidSHA(h.ReceiptSelector)) {
			return ErrInvalidProgramCTransition
		}
	case "COMMITTED":
		if h.CommittedPrefix == 0 || h.ClaimOrdinal != h.CommittedPrefix-1 || h.ClaimGeneration == 0 || h.ClaimGeneration >= h.Generation || (h.ClaimSelector == "" && h.ClaimGeneration != 1) || !transitionValidSHA(h.ReceiptSelector) {
			return ErrInvalidProgramCTransition
		}
	default:
		return ErrInvalidProgramCTransition
	}
	return nil
}
func transitionResult(h programCTransitionHead, s ProgramCTransitionStatus, r programCTransitionReceipt) ProgramCTransitionResult {
	return ProgramCTransitionResult{Status: s, Head: censussourcecontinuation.HeadRevision{Generation: h.Generation, Selector: h.Selector}, ReceiptSelector: h.ReceiptSelector, CommittedPrefix: h.CommittedPrefix, ConstituentCount: h.ConstituentCount, Ordinal: h.ClaimOrdinal, ProgramCOutcome: r.ProgramCOutcome, FailureStage: r.FailureStage, Failure: r.Failure, ResultSHA256: r.ResultSHA256, ResultByteLength: r.ResultByteLength}
}
func transitionCAS(ctx context.Context, r ProgramCTransitionRequest, e censussourcecontinuation.HeadRevision, h programCTransitionHead) (programCTransitionHead, censussourcecontinuation.CASOutcome, error) {
	raw, x := transitionSelector(&h)
	if x != nil {
		return h, "", x
	}
	cr := r.Heads.CompareAndSwap(ctx, r.HeadKey, e, raw)
	switch cr.Outcome {
	case censussourcecontinuation.CASCommitted:
		if cr.Err != nil {
			return h, "", ErrInvalidProgramCTransition
		}
	case censussourcecontinuation.CASNotCommitted:
		if cr.Err != nil {
			return h, "", cr.Err
		}
	case censussourcecontinuation.CASCommitUncertain:
	default:
		return h, "", ErrInvalidProgramCTransition
	}
	return h, cr.Outcome, nil
}
func transitionPut(ctx context.Context, s censussourcecontinuation.ImmutableStore, raw []byte) (string, error) {
	k, e := s.Put(ctx, raw)
	if e != nil {
		return "", e
	}
	got, e := s.Get(ctx, k)
	if e != nil || k != transitionSHA(raw) || !bytes.Equal(got, raw) {
		return "", ErrInvalidProgramCTransition
	}
	return k, nil
}

func CoordinateProgramCTransition(ctx context.Context, r ProgramCTransitionRequest) (ProgramCTransitionResult, error) {
	if ctx == nil || r.Objects == nil || r.Heads == nil || r.Runner == nil || r.PlanSource == nil || r.WorkflowKey == "" || r.HeadKey == "" {
		return ProgramCTransitionResult{}, ErrInvalidProgramCTransition
	}
	planRaw, e := r.PlanSource.Load(ctx)
	if e != nil {
		return ProgramCTransitionResult{}, e
	}
	plan, e := decodeProgramCTransitionPlan(planRaw)
	if e != nil || !transitionValidBindings(r.Bindings, plan, planRaw) {
		return ProgramCTransitionResult{}, ErrInvalidProgramCTransition
	}
	raw, e := r.Heads.Load(ctx, r.HeadKey)
	if e != nil {
		return ProgramCTransitionResult{}, e
	}
	h := programCTransitionHead{SchemaVersion: programCTransitionHeadVersion, WorkflowKey: r.WorkflowKey, HeadKey: r.HeadKey, Authority: 0, State: "ABSENT", Bindings: r.Bindings, ConstituentCount: len(plan.Records), ClaimOrdinal: -1}
	if len(raw) > 0 {
		if transitionStrict(raw, &h) != nil {
			return ProgramCTransitionResult{}, ErrInvalidProgramCTransition
		}
		c := h
		b, _ := transitionSelector(&c)
		if !bytes.Equal(b, raw) {
			return ProgramCTransitionResult{}, ErrInvalidProgramCTransition
		}
	}
	if transitionValidateHead(h, r, len(plan.Records)) != nil {
		return ProgramCTransitionResult{}, ErrInvalidProgramCTransition
	}
	cur := censussourcecontinuation.HeadRevision{Generation: h.Generation, Selector: h.Selector}
	if cur != r.Expected {
		return transitionResult(h, ProgramCTransitionConflict, programCTransitionReceipt{}), nil
	}
	if h.State == "CLAIMED" {
		if h.ReceiptSelector == "" {
			return transitionResult(h, ProgramCTransitionIndeterminate, programCTransitionReceipt{}), nil
		}
		return transitionCommitReceipt(ctx, r, h, cur, plan)
	}
	if h.CommittedPrefix == h.ConstituentCount {
		return transitionResult(h, ProgramCTransitionComplete, programCTransitionReceipt{}), nil
	}
	record := plan.Records[h.CommittedPrefix]
	claim := h
	claim.Generation++
	claim.State = "CLAIMED"
	claim.ClaimOrdinal = record.Ordinal
	claim.ClaimGeneration = claim.Generation
	claim.ClaimSelector = cur.Selector
	claim.ClaimPublicationSelector = record.PublicationSelector
	claim.ClaimConstituentSelector = record.ConstituentSelector
	claim.ReceiptSelector = ""
	claim, out, e := transitionCAS(ctx, r, cur, claim)
	if e != nil {
		return ProgramCTransitionResult{}, e
	}
	if out == censussourcecontinuation.CASCommitUncertain {
		return transitionResult(claim, ProgramCTransitionIndeterminate, programCTransitionReceipt{}), nil
	}
	if out == censussourcecontinuation.CASNotCommitted {
		return transitionResult(h, ProgramCTransitionConflict, programCTransitionReceipt{}), nil
	}
	receipt := programCTransitionReceipt{SchemaVersion: programCTransitionReceiptVersion, WorkflowKey: r.WorkflowKey, HeadKey: r.HeadKey, Authority: 0, Bindings: r.Bindings, ConstituentCount: h.ConstituentCount, Ordinal: record.Ordinal, ReceiptOrdinal: record.ReceiptOrdinal, PublicationByteLength: record.PublicationByteLength, BatchOrdinal: record.BatchOrdinal, PublicationSelector: record.PublicationSelector, ConstituentSelector: record.ConstituentSelector, ClaimGeneration: claim.ClaimGeneration, ClaimSelector: claim.ClaimSelector, InstabilitySchema: programc.InstabilityVersionV2, InstabilityClaimCeiling: programc.InstabilityClaimCeilingV2}
	input, runErr := r.Runner.Resolve(ctx, record.PublicationSelector, record.ConstituentSelector)
	failureStage := ProgramCTransitionFailureStage("")
	if runErr != nil {
		failureStage = ProgramCTransitionFailureResolve
	}
	var outcomes []programc.Outcome
	if runErr == nil {
		for _, seed := range []uint64{1, 1, 1, 2, 2, 2} {
			o, f := r.Runner.Compute(input, seed)
			if f != nil || (o.Outcome != "COMPLETE" && o.Outcome != "EMPTY") {
				failureStage = ProgramCTransitionFailureCompute
				if f != nil {
					runErr = f
				} else {
					runErr = errors.New("program c run neither COMPLETE nor EMPTY")
				}
				break
			}
			if len(outcomes) > 0 && o.Outcome != outcomes[0].Outcome {
				failureStage = ProgramCTransitionFailureCompute
				runErr = errors.New("mixed program c outcomes")
				break
			}
			outcomes = append(outcomes, o)
		}
	}
	if runErr == nil {
		receipt.ProgramCOutcome = outcomes[0].Outcome
		receipt.ResultBytes, runErr = r.Runner.Instability(outcomes, r.Bindings)
		if runErr != nil {
			failureStage = ProgramCTransitionFailureInstabilityBuild
			receipt.ResultBytes = nil
		} else if runErr = r.Runner.ValidateInstability(receipt.ResultBytes, outcomes, r.Bindings); runErr != nil {
			failureStage = ProgramCTransitionFailureInstabilityValidate
			receipt.ResultBytes = nil
		}
	}
	if runErr != nil {
		receipt.Outcome = "FAILED"
		receipt.FailureStage = failureStage
		receipt.Failure = runErr.Error()
	} else if transitionValidateResult(receipt.ResultBytes) != nil {
		return ProgramCTransitionResult{}, ErrInvalidProgramCTransition
	} else {
		receipt.Outcome = "SUCCEEDED"
		receipt.ResultSHA256 = transitionSHA(receipt.ResultBytes)
		receipt.ResultByteLength = len(receipt.ResultBytes)
	}
	receiptRaw, _ := json.Marshal(receipt)
	receiptSelector, e := transitionPut(ctx, r.Objects, receiptRaw)
	if e != nil {
		return ProgramCTransitionResult{}, e
	}
	linked := claim
	linked.Generation++
	linked.ReceiptSelector = receiptSelector
	linked, out, e = transitionCAS(ctx, r, censussourcecontinuation.HeadRevision{Generation: claim.Generation, Selector: claim.Selector}, linked)
	if e != nil {
		return ProgramCTransitionResult{}, e
	}
	if out == censussourcecontinuation.CASCommitUncertain {
		return transitionResult(linked, ProgramCTransitionIndeterminate, receipt), nil
	}
	if out == censussourcecontinuation.CASNotCommitted {
		return transitionResult(claim, ProgramCTransitionConflict, receipt), nil
	}
	if receipt.Outcome == "FAILED" {
		return transitionResult(linked, ProgramCTransitionRetained, receipt), nil
	}
	return transitionCommitReceipt(ctx, r, linked, censussourcecontinuation.HeadRevision{Generation: linked.Generation, Selector: linked.Selector}, plan)
}
func transitionValidateResult(raw []byte) error {
	var x programc.InstabilityArtifact
	if transitionStrict(raw, &x) != nil || x.SchemaVersion != programc.InstabilityVersionV2 || x.ClaimCeiling != programc.InstabilityClaimCeilingV2 || x.StructuralClaimCeiling != programc.InstabilityStructuralCeiling {
		return ErrInvalidProgramCTransition
	}
	return nil
}
func transitionLoadReceipt(ctx context.Context, r ProgramCTransitionRequest, h programCTransitionHead, p ProgramCTransitionPlan) (programCTransitionReceipt, error) {
	raw, e := r.Objects.Get(ctx, h.ReceiptSelector)
	if e != nil || transitionSHA(raw) != h.ReceiptSelector {
		return programCTransitionReceipt{}, ErrInvalidProgramCTransition
	}
	var x programCTransitionReceipt
	if transitionStrict(raw, &x) != nil || x.SchemaVersion != programCTransitionReceiptVersion || x.WorkflowKey != r.WorkflowKey || x.HeadKey != r.HeadKey || x.Authority != 0 || x.Bindings != r.Bindings || x.ConstituentCount != len(p.Records) || x.Ordinal != h.ClaimOrdinal || x.Ordinal < 0 || x.Ordinal >= len(p.Records) || x.PublicationSelector != h.ClaimPublicationSelector || x.ConstituentSelector != h.ClaimConstituentSelector || x.ReceiptOrdinal != p.Records[x.Ordinal].ReceiptOrdinal || x.PublicationByteLength != p.Records[x.Ordinal].PublicationByteLength || x.BatchOrdinal != p.Records[x.Ordinal].BatchOrdinal || x.PublicationSelector != p.Records[x.Ordinal].PublicationSelector || x.ConstituentSelector != p.Records[x.Ordinal].ConstituentSelector || x.ClaimGeneration != h.ClaimGeneration || x.ClaimSelector != h.ClaimSelector || x.InstabilitySchema != programc.InstabilityVersionV2 || x.InstabilityClaimCeiling != programc.InstabilityClaimCeilingV2 {
		return x, ErrInvalidProgramCTransition
	}
	if !validTransitionReceiptOutcome(x) {
		return x, ErrInvalidProgramCTransition
	}
	return x, nil
}

func validTransitionReceiptOutcome(x programCTransitionReceipt) bool {
	if x.Outcome == "SUCCEEDED" {
		return (x.ProgramCOutcome == "COMPLETE" || x.ProgramCOutcome == "EMPTY") && transitionValidateResult(x.ResultBytes) == nil && x.ResultSHA256 == transitionSHA(x.ResultBytes) && x.ResultByteLength == len(x.ResultBytes) && x.FailureStage == "" && x.Failure == ""
	}
	if x.Outcome != "FAILED" || x.Failure == "" || len(x.ResultBytes) != 0 || x.ResultSHA256 != "" || x.ResultByteLength != 0 {
		return false
	}
	switch x.FailureStage {
	case ProgramCTransitionFailureResolve, ProgramCTransitionFailureCompute:
		return x.ProgramCOutcome == ""
	case ProgramCTransitionFailureInstabilityBuild, ProgramCTransitionFailureInstabilityValidate:
		return x.ProgramCOutcome == "COMPLETE" || x.ProgramCOutcome == "EMPTY"
	default:
		return false
	}
}
func transitionCommitReceipt(ctx context.Context, r ProgramCTransitionRequest, h programCTransitionHead, e censussourcecontinuation.HeadRevision, p ProgramCTransitionPlan) (ProgramCTransitionResult, error) {
	receipt, x := transitionLoadReceipt(ctx, r, h, p)
	if x != nil {
		return ProgramCTransitionResult{}, x
	}
	if receipt.Outcome == "FAILED" {
		return transitionResult(h, ProgramCTransitionRetained, receipt), nil
	}
	comm := h
	comm.Generation++
	comm.State = "COMMITTED"
	comm.CommittedPrefix = h.ClaimOrdinal + 1
	comm, out, x := transitionCAS(ctx, r, e, comm)
	if x != nil {
		return ProgramCTransitionResult{}, x
	}
	if out == censussourcecontinuation.CASCommitUncertain {
		return transitionResult(comm, ProgramCTransitionIndeterminate, receipt), nil
	}
	if out == censussourcecontinuation.CASNotCommitted {
		return transitionResult(h, ProgramCTransitionConflict, receipt), nil
	}
	return transitionResult(comm, ProgramCTransitionCommitted, receipt), nil
}

type ProgramCProductionRunner struct {
	source *AuthoritativeProgramCTransitionPlanSource
}

func NewProgramCProductionRunner(source *AuthoritativeProgramCTransitionPlanSource) ProgramCProductionRunner {
	return ProgramCProductionRunner{source: source}
}

func (r ProgramCProductionRunner) Resolve(_ context.Context, p, c string) ([]byte, error) {
	if r.source == nil || r.source.resolver == nil {
		return nil, ErrInvalidProgramCTransition
	}
	return r.source.resolver.resolve(p, c)
}
func (ProgramCProductionRunner) Compute(b []byte, s uint64) (programc.Outcome, *programc.Failure) {
	return programc.Compute(b, s)
}
func (ProgramCProductionRunner) Instability(outcomes []programc.Outcome, b ProgramCTransitionBindings) ([]byte, error) {
	if len(outcomes) != 6 {
		return nil, ErrInvalidProgramCTransition
	}
	id := programc.InstabilityIdentity{AdmittedGraphSHA256: outcomes[0].Source.InputSHA256, ProjectionSHA256: programc.ProfileDigest, ProjectionPolicyID: programc.ProfileID, AlgorithmName: outcomes[0].Algorithm, AlgorithmVersion: b.AlgorithmVersion, ParametersCanonicalSHA256: b.ParametersSHA256, ResourcePolicySHA256: b.ResourcePolicySHA256}
	runs := make([]programc.InstabilityRun, 0, 6)
	for i, o := range outcomes {
		community, e := programc.BuildCommunityArtifact(o, id)
		if e != nil {
			return nil, e
		}
		br := programc.BoundaryRequest{PageRankTopK: 1, HubTopK: 1}
		boundary, e := programc.ComputeBoundary(o, br)
		if e != nil {
			return nil, e
		}
		boundaryRaw, _ := json.Marshal(boundary)
		runs = append(runs, programc.InstabilityRun{RunID: fmt.Sprintf("seed-%020d-replay-%d", o.Seed, i%3+1), Seed: o.Seed, Status: "COMPLETE", Community: o, CommunityArtifact: community, BoundaryArtifact: boundaryRaw, BoundaryRequest: br})
	}
	req := programc.InstabilityRequest{Identity: id, DeclaredSeeds: []uint64{1, 2}, Runs: runs}
	artifact, e := programc.ComputeInstabilityV2(req)
	if e != nil {
		return nil, e
	}
	raw, _ := json.Marshal(artifact)
	if e = programc.ValidateInstabilityV2(raw, req); e != nil {
		return nil, e
	}
	return raw, nil
}
func (r ProgramCProductionRunner) ValidateInstability(raw []byte, outcomes []programc.Outcome, b ProgramCTransitionBindings) error {
	expected, err := r.Instability(outcomes, b)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, expected) {
		return ErrInvalidProgramCTransition
	}
	return nil
}
