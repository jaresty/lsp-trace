//go:build darwin || linux

// Package privatehostproductv3 is a private existing-host workflow kernel. The
// host supplies semantics through Backend; this package never runs inference.
package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

const Profile = "PRIVATE_EXISTING_HOST_PAGED_INVENTORY_V3_1"
const Protocol = "PRIVATE_V3_OWN_AST_EVIDENCE_CAPSULE_1"

var ErrUnavailable = errors.New("UNAVAILABLE_PRIVATE_HOST_PRODUCT_V3")
var ErrInvalid = errors.New("INVALID_PRIVATE_HOST_PRODUCT_V3")
var ErrPaused = errors.New("PAUSED_BUDGET")
var ErrItemEvidence = errors.New("ITEM_EVIDENCE_BINDING_REFUSED")

type budgetBoundary struct {
	Resource        string
	Observed, Limit uint64
}

func (e *budgetBoundary) Error() string { return "PAUSED_BUDGET" }
func (e *budgetBoundary) Unwrap() error { return ErrPaused }

type EvidenceReference struct{ Family, Ref, InputID string }

// ApprovalSHA256 hashes this exact canonical Go JSON with ApprovalSHA256 empty.
// HumanReceiptSHA256 is a distinct caller observation, not authentication.
// All allowances are explicit. Config history may increase semantic allowance
// only through a new revision bound to the exact prior configuration digest.
type Config struct {
	Profile, Application, ScopeSHA256, ProducerConfigSHA256    string
	ApprovalSHA256, HumanReceiptSHA256, PreviousConfigSHA256   string
	AllowanceRevision                                          uint64
	RolePromptSHA256                                           [2]string
	Store                                                      string
	Requested, StartBatch                                      int
	Exchange                                                   pilot.PageCostPrivateV3
	Preparation                                                prep.Limits
	Seeds                                                      []uint64
	MaxCalls, MaxRequestBytes, MaxResponseBytes, MaxTotalBytes int
}

func ConfigDigest(c Config) string {
	c.ApprovalSHA256 = ""
	b, _ := json.Marshal(c)
	return pilot.Digest(b)
}
func ValidateConfig(c Config) error {
	if c.RolePromptSHA256 != [2]string{PromptDigest("interpreter"), PromptDigest("reviewer")} {
		return ErrInvalid
	}
	l := c.Preparation
	if l.MaxTargets <= 0 || l.MaxTargets > 64 || l.MaxPartBytes < 2048 || l.MaxPartBytes > 2<<20 || l.MaxTotalBytes < l.MaxPartBytes || l.MaxTotalBytes > 8<<20 || l.MaxParts <= 0 || l.MaxParts > 256 || l.MaxSourceBytes <= 0 || l.MaxSourceBytes > 2<<20 {
		return ErrInvalid
	}
	if c.Profile != Profile || c.Application == "" || c.ScopeSHA256 == "" || c.ProducerConfigSHA256 == "" || c.ApprovalSHA256 != ConfigDigest(c) || c.AllowanceRevision == 0 || c.Requested < 1 || c.StartBatch < 0 || c.StartBatch > c.Requested || len(c.Seeds) < 2 || c.MaxCalls < 1 || c.MaxRequestBytes < 1 || c.MaxRequestBytes > 1<<20 || c.MaxResponseBytes < 1 || c.MaxResponseBytes > 64<<10 || c.MaxTotalBytes < 1 || c.MaxTotalBytes > 64<<20 || c.Exchange.LogicalBytes == 0 || c.Exchange.ReadHashBytes == 0 || c.Exchange.WriteBytes == 0 || c.Exchange.Work == 0 {
		return ErrInvalid
	}
	return nil
}

type Task struct {
	Application, ConfigSHA256, ScopeSHA256         string
	Revision                                       uint64
	MaxRequestBytes, MaxResponseBytes              int
	Protocol, Role, InputID, Target, RequestDigest string
	Sequence                                       uint64
	Attempt, Predecessor                           string
	InputFamily, PreparedSHA256, CheckpointSHA256  string
	OriginalRawReferences                          [4]string
	// Body is the complete own AST body, not the original raw graph. Raw originals
	// stay byte-exact in paired page retention. No statement claims they were read.
	Body, OwnBodySHA256 string
	StructuralFacts     string
	Proposal            *Response `json:",omitempty"`
}
type Response struct {
	Protocol, RequestDigest, Label, Explanation, Alternative, RejectionCondition, Quote string
	Start, End                                                                          int
	Supported                                                                           bool
}
type Backend interface {
	Exchange(context.Context, Task) (Response, error)
}
type TargetGap struct{ Target, Phase, Class string }

type Candidate struct {
	ID, InputID, Target, Status, WorkingLabel, Explanation string
	Revision                                               int
	Predecessors                                           []string
	InventoryStateDelta                                    string
	EvidenceRefs                                           []string
	Evidence                                               []EvidenceReference
	Alternatives, Gaps                                     []string
	RejectionCondition, Quote                              string
	QuoteStart, QuoteEnd                                   int
}
type Failure struct {
	Phase, Class, Resource string
	Observed, Limit        uint64
	ExactOriginalError     string
}
type Spend struct {
	Calls, RequestBytes, ResponseBytes int
	PolicyReservation                  pilot.PageCostPrivateV3
	Excluded                           []string
}
type Lifecycle struct {
	OriginalStopCalls, OriginalJoinCalls      int
	InnerGraceful, OuterOriginalProcessWaited bool
	RoleCleanup                               string
}

// AdmissionObservation is report DATA, never an owner or a validation seal.
// UNKNOWN means no successful current check is asserted, even if some earlier
// admission steps completed before a later refusal.
type AdmissionObservation struct {
	CurrentValidation, LedgerValidation, SourceValidation string
	Coverage, Counts, LedgerObservation, ResourceCounts   string
	PriorMachinePaths, PriorHumanPaths                    [2]string
	PriorPublicationValidation                            string `json:",omitempty"`
	UnadmittedPublications                                bool   `json:",omitempty"`
}

type Report struct {
	Schema, Profile, Status, Application, ConfigSHA256    string
	Authority                                             uint8
	Accepted                                              bool
	FeatureIdentity, SourceCompleteness, AbsenceInference string
	Requested, Completed, Gaps, Pending                   int
	Candidates                                            []Candidate
	Unassigned                                            []string
	NavigationHypotheses                                  []string
	TargetGaps                                            []TargetGap
	Spend                                                 Spend
	Failure                                               *Failure `json:",omitempty"`
	FirstFailure                                          *Failure `json:",omitempty"`
	CheckpointSHA256, LedgerTip                           string
	LedgerGeneration                                      uint64
	Resume                                                string
	MachinePaths, HumanPaths                              [2]string
	ReportRevision                                        int
	NextBatch                                             int
	Lifecycle                                             Lifecycle
	Admission                                             *AdmissionObservation `json:",omitempty"`
}
type ownBody struct{ Target, NodeID, SourceURI, OwnBodySHA256, Body string }

func ownBodies(bodies []prep.OwnASTBodyPrivateV3) []ownBody {
	out := make([]ownBody, 0, len(bodies))
	for _, b := range bodies {
		out = append(out, ownBody{b.Target, b.NodeID, b.SourceURI, b.OwnBodySHA256, b.Declaration.Body})
	}
	return out
}

type preparedObservation struct {
	Batch               int
	InputID, Checkpoint string
	References          [3]string
	RawReferences       [4]string
	Bodies              []ownBody
}
type progress struct {
	Config                             Config
	Calls, RequestBytes, ResponseBytes int
	Candidates                         []Candidate
	Observations                       []preparedObservation
	CompletedBatches                   int
	LastCheckpoint                     []byte
	LastStatus                         string
	Inflight                           bool
}

// RunRetained admits only the actual opaque retained owner. It never manufactures
// PostPreparationPrivateV2 or revives an original Manager/capture capability.
func RunRetained(ctx context.Context, c Config, owner pilot.RetainedPageCheckpointOwnerPrivateV3, backend Backend) (out Report, err error) {
	if ctx == nil || ValidateConfig(c) != nil || backend == nil {
		return out, ErrInvalid
	}
	manifest, _, _, _, e := owner.HostFrontierPrivateV3()
	if e != nil || manifest.Application != c.Application || manifest.ScopeSHA256 != c.ScopeSHA256 || manifest.Revision.Policy.ConfigSHA256 != c.ProducerConfigSHA256 || int(manifest.Targets) != c.Requested || manifest.NextBatch != c.StartBatch {
		return out, ErrInvalid
	}
	st, e := openStore(c)
	if e != nil {
		return out, e
	}
	defer st.close()
	state, e := st.load(c)
	if e != nil {
		return out, e
	}
	// Reserve before subsequent bounded deterministic metadata/body processing.
	charge := func(n int) error {
		if n < 0 || n > 16<<20 {
			return ErrInvalid
		}
		cost := pilot.PageCostPrivateV3{LogicalBytes: uint64(n)*16 + 65536, ReadHashBytes: uint64(n)*32 + 65536, WriteBytes: uint64(n)*4 + 65536, Work: uint64(n) + 1024}
		return owner.ReserveHostWorkPrivateV3(ctx, cost, c.Exchange)
	}
	lastGood := state
	save := func() error {
		e := st.save(state)
		if e == nil {
			lastGood = state
		}
		return e
	}
	phase := "ADMITTED_RETAINED_OBSERVATION"
	currentTarget := ""
	finish := func(cause error) (Report, error) {
		if !admitProgress(state) {
			calls, requestBytes, responseBytes, inflight := state.Calls, state.RequestBytes, state.ResponseBytes, state.Inflight
			state = lastGood
			state.Calls = calls
			state.RequestBytes = requestBytes
			state.ResponseBytes = responseBytes
			state.Inflight = inflight
			cause = errors.Join(cause, &budgetBoundary{Resource: "application_progress_field_graph", Limit: stateLimit})
		}
		status := "PROVISIONAL"
		if cause != nil {
			switch {
			case errors.Is(cause, ErrPaused), errors.Is(cause, pilot.ErrPagesPausedPrivateV3):
				status = "PAUSED_BUDGET"
			case errors.Is(cause, context.Canceled), errors.Is(cause, context.DeadlineExceeded):
				status = "CANCELLED"
			default:
				status = "FAILED"
			}
		}
		m, g, tip, spent, frontErr := owner.HostFrontierPrivateV3()
		if frontErr != nil {
			return out, errors.Join(cause, frontErr)
		}
		out = Report{Schema: "private-host-product-report.v3.1", Profile: Profile, Status: status, Application: c.Application, ConfigSHA256: ConfigDigest(c), FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", Requested: c.Requested, Completed: len(state.Candidates), Pending: c.Requested - len(state.Candidates), Candidates: state.Candidates, Unassigned: []string{}, NavigationHypotheses: []string{}, Spend: Spend{Calls: state.Calls, RequestBytes: state.RequestBytes, ResponseBytes: state.ResponseBytes, PolicyReservation: spent, Excluded: []string{"original source enumeration/acquisition/session", "caller bootstrap verification budget (separate)", "model inference billing/tokens/host orchestration", "application-store/report IO not fully reserved"}}, CheckpointSHA256: pilot.Digest(state.LastCheckpoint), LedgerTip: tip, LedgerGeneration: g}
		if observed, ok := backend.(interface{ Lifecycle() Lifecycle }); ok {
			out.Lifecycle = observed.Lifecycle()
		} else {
			out.Lifecycle.RoleCleanup = "UNOBSERVED_HOST_TOOL_CONTRACT"
		}
		out.NextBatch = m.NextBatch
		if len(state.LastCheckpoint) == 0 {
			out.CheckpointSHA256 = pilot.Digest(mustJSON(m))
		}
		for _, candidate := range out.Candidates {
			out.Unassigned = append(out.Unassigned, candidate.ID)
		}
		if cause != nil {
			out.Failure = &Failure{Phase: phase, Class: status, Resource: "UNKNOWN", ExactOriginalError: "UNKNOWN"}
			var boundary *budgetBoundary
			if errors.As(cause, &boundary) {
				out.Failure.Resource = boundary.Resource
				out.Failure.Observed = boundary.Observed
				out.Failure.Limit = boundary.Limit
			}
			if errors.Is(cause, ErrItemEvidence) {
				out.Failure.Class = ErrItemEvidence.Error()
			}
			if status == "FAILED" && currentTarget != "" {
				out.Gaps = 1
				out.Pending--
				out.TargetGaps = []TargetGap{{currentTarget, phase, out.Failure.Class}}
			}
		}
		if out.Failure != nil {
			out.FirstFailure = out.Failure
		}
		out.Resume = fmt.Sprintf("%s:%d:%s", c.Application, len(state.Candidates), out.CheckpointSHA256)
		state.LastStatus = status
		if e := save(); e != nil {
			return out, errors.Join(cause, e)
		}
		if e := st.deliver(&out); e != nil {
			return out, errors.Join(cause, e)
		}
		return out, cause
	}
	// Prior immutable candidate rows remain useful even after subsequent failure.
	// A prepared observation can resume semantics only after exact paired readback
	// through this newly admitted owner; decoding the report is not a new seal.
	if state.Inflight {
		phase = "SPENT_HOST_EXCHANGE_NO_AUTORETRY"
		return finish(ErrInvalid)
	}
	for oi := range state.Observations {
		obs := state.Observations[oi]
		phase = "RETAINED_PREPARED_OBSERVATION_READBACK"
		raw, e := owner.ReadOutput(ctx, obs.References[2], min(uint64(c.Preparation.MaxPartBytes), manifest.Revision.Policy.ComputationUnitBytes), c.Exchange)
		if e != nil {
			return finish(e)
		}
		var r prep.OwnASTReportRetainedCheckpointPrivateV3
		if json.Unmarshal(raw, &r) != nil || r.SourceInputID != obs.InputID || !bytes.Equal(mustJSON(ownBodies(r.Bodies)), mustJSON(obs.Bodies)) {
			return finish(ErrInvalid)
		}
	}
	process := func(obs preparedObservation) error {
		for _, body := range obs.Bodies {
			input := obs.InputID + "/" + body.Target
			done := false
			for _, v := range state.Candidates {
				if v.InputID == input {
					done = true
				}
			}
			if done {
				continue
			}
			if e := ctx.Err(); e != nil {
				return e
			}
			currentTarget = body.Target
			if state.Calls > c.MaxCalls-2 {
				return &budgetBoundary{Resource: "requested_cumulative_physical_calls", Observed: uint64(state.Calls) + 2, Limit: uint64(c.MaxCalls)}
			}
			if e := charge(len(body.Body)); e != nil {
				return e
			}
			t := Task{Application: c.Application, ConfigSHA256: ConfigDigest(c), ScopeSHA256: c.ScopeSHA256, Revision: 1, MaxRequestBytes: c.MaxRequestBytes, MaxResponseBytes: c.MaxResponseBytes, Protocol: Protocol, Role: "interpreter", InputID: input, Target: body.Target, Attempt: fmt.Sprintf("grant-%d/batch-%d/item-%d", c.AllowanceRevision, obs.Batch, len(state.Candidates)), InputFamily: pilot.RetainedCheckpointProfilePrivateV3, PreparedSHA256: obs.References[2], CheckpointSHA256: obs.Checkpoint, OriginalRawReferences: obs.RawReferences, Body: body.Body, OwnBodySHA256: body.OwnBodySHA256, StructuralFacts: "Per-seed own-source AST observation only. CALLS, if referenced, are server-reported original evidence retained outside this capsule; no union or inferred CALLS. Raw graph bytes are NOT supplied in this request."}
			var proposal Response
			for _, role := range []string{"interpreter", "reviewer"} {
				t.Role = role
				t.Sequence = uint64(state.Calls + 1)
				t.RequestDigest = ""
				if !admitTask(t, c.MaxRequestBytes) {
					return pilot.ErrPagesUnitPrivateV3
				}
				t.RequestDigest = pilot.Digest(mustJSON(t))
				raw := mustJSON(t)
				if len(raw) > c.MaxRequestBytes || state.RequestBytes+state.ResponseBytes > c.MaxTotalBytes-len(raw) {
					return ErrPaused
				}
				state.Calls++
				state.Inflight = true
				state.RequestBytes += len(raw)
				// Durable reservation/claim precedes disclosure. A lost response is never
				// retried automatically with the same attempt/sequence.
				if e := st.claim(t); e != nil {
					return e
				}
				if e := save(); e != nil {
					return e
				}
				response, e := backend.Exchange(ctx, t)
				if e != nil {
					return e
				}
				if !admitResponse(response, c.MaxResponseBytes) {
					return pilot.ErrPagesUnitPrivateV3
				}
				raw = mustJSON(response)
				if len(raw) > c.MaxResponseBytes || state.RequestBytes+state.ResponseBytes > c.MaxTotalBytes-len(raw) {
					return ErrPaused
				}
				state.ResponseBytes += len(raw)
				if response.Protocol != Protocol || response.RequestDigest != t.RequestDigest || response.Start < 0 || response.End < response.Start || response.End > len(t.Body) || !utf8.ValidString(t.Body[:response.Start]) || !utf8.ValidString(t.Body[:response.End]) || t.Body[response.Start:response.End] != response.Quote || response.Quote == "" || strings.TrimSpace(response.Alternative) == "" || strings.TrimSpace(response.RejectionCondition) == "" || strings.TrimSpace(response.Explanation) == "" || strings.TrimSpace(response.Label) == "" {
					return ErrItemEvidence
				}
				if e := st.response(t, response); e != nil {
					return e
				}
				state.Inflight = false
				if role == "interpreter" {
					proposal = response
					t.Proposal = &proposal
				} else if !response.Supported {
					return ErrItemEvidence
				} else {
					// A supported narrowing remains within this same item's validated
					// body/citation. Do not discard the reviewer's correction.
					proposal = response
				}
			}
			candidate := Candidate{ID: "candidate-" + pilot.Digest([]byte(input))[7:23], InputID: input, Target: body.Target, Status: "PROVISIONAL", WorkingLabel: proposal.Label, Explanation: proposal.Explanation, Revision: 1, Predecessors: []string{}, InventoryStateDelta: "ADD_PROVISIONAL_CANDIDATE", EvidenceRefs: []string{obs.References[2], obs.References[1], obs.References[0], body.NodeID, body.OwnBodySHA256}, Alternatives: []string{proposal.Alternative}, Gaps: []string{"Product purpose and identity require external adjudication; source completeness UNKNOWN"}, RejectionCondition: proposal.RejectionCondition, Quote: proposal.Quote, QuoteStart: proposal.Start, QuoteEnd: proposal.End}
			candidate.EvidenceRefs = append(candidate.EvidenceRefs, obs.RawReferences[:]...)
			families := []string{"V3_RETAINED_PREPARED_OBSERVATION", "V3_RETAINED_ASSESSMENT", "V3_DERIVED_CANONICAL", "SOURCE_SPECIFIC_NODE_ID", "OWN_AST_BODY", "ORIGINAL_RAW_BATCH_SEEDS", "ORIGINAL_RAW_CAPTURE_ORIGIN", "ORIGINAL_RAW_SOURCE_ARTIFACT", "ORIGINAL_RAW_MANIFEST_INPUT"}
			for index, ref := range candidate.EvidenceRefs {
				candidate.Evidence = append(candidate.Evidence, EvidenceReference{families[index], ref, input})
			}
			state.Candidates = append(state.Candidates, candidate)
			currentTarget = ""
			if e := save(); e != nil {
				return e
			}
		}
		return nil
	}
	for _, obs := range state.Observations {
		phase = "HOST_SEMANTIC_EXCHANGE"
		if e := process(obs); e != nil {
			return finish(e)
		}
	}
	for i := manifest.NextBatch; i < int(manifest.Revision.BatchDenominator); i++ {
		if e := ctx.Err(); e != nil {
			return finish(e)
		}
		phase = "V3_SELECTED_BATCH"
		batch, e := owner.SelectedBatch(ctx, i, c.Exchange)
		if e != nil {
			return finish(e)
		}
		view, e := batch.Read(ctx, c.Exchange)
		if e != nil {
			return finish(e)
		}
		phase = "V3_TYPED_COMPUTATION"
		p, e := derived.ProjectRetainedCheckpointPrivateV3(ctx, batch, c.Exchange)
		if e != nil {
			return finish(e)
		}
		a, e := derived.ComputeInstabilityRetainedCheckpointPrivateV3(ctx, p, c.Seeds, c.Exchange)
		if e != nil {
			return finish(e)
		}
		phase = "V3_OWN_AST_PREPARATION"
		reader, e := prep.NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, a, c.Preparation, c.Exchange)
		if e != nil {
			return finish(e)
		}
		_, prepared, e := reader.Advance(ctx, 1, c.Exchange)
		if e != nil {
			return finish(e)
		}
		raw, e := prepared.Read(ctx, c.Exchange)
		if e != nil {
			return finish(e)
		}
		var r prep.OwnASTReportRetainedCheckpointPrivateV3
		if json.Unmarshal(raw, &r) != nil || r.PreparedCount != len(r.Bodies) || r.Accepted || r.Authority != 0 || r.RequestType != prep.RequestTypeRetainedCheckpointPrivateV3 {
			return finish(ErrInvalid)
		}
		phase = "V3_PAIRED_PREPARED_PUBLICATION"
		pub, e := prepared.Publication(ctx, c.Exchange)
		if e != nil {
			return finish(e)
		}
		refs, e := pub.Advance(ctx, 1, c.Exchange)
		if e != nil {
			return finish(e)
		}
		obs := preparedObservation{Batch: i, InputID: r.SourceInputID, Checkpoint: r.Source.CheckpointSHA256, References: [3]string{refs.CanonicalSHA256, refs.AssessmentSHA256, refs.PreparedSHA256}, RawReferences: view.ObjectSHA256, Bodies: ownBodies(r.Bodies)}
		if e = charge(len(raw)); e != nil {
			return finish(e)
		}
		// Save useful publication references BEFORE further semantic or unit work.
		state.Observations = append(state.Observations, obs)
		if e = save(); e != nil {
			return finish(e)
		}
		if e = owner.RecordPublishedReferences(ctx, batch, obs.References, c.Exchange); e != nil {
			return finish(e)
		}
		next, e := owner.CheckpointCompletedBatch(ctx, batch, pilot.Digest([]byte(fmt.Sprintf("%s/next/%d", c.Application, i))), c.Exchange)
		if e != nil {
			return finish(e)
		}
		state.LastCheckpoint = next
		state.CompletedBatches = i + 1
		if e = save(); e != nil {
			return finish(e)
		}
		phase = "HOST_SEMANTIC_EXCHANGE"
		if e = process(obs); e != nil {
			return finish(e)
		}
	}
	return finish(nil)
}
func mustJSON(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic("private concrete DTO serialization")
	}
	return b
}
