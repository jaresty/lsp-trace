//go:build darwin || linux

package privatedescribeprep

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
)

const CohortHandoffProtocolPrivateV2 = "PRIVATE_V2_AST_COHORT_LOCAL_HANDOFF.v1"
const CohortModePrivateV2 = "PRIVATE_OPT_IN_SEMANTIC_COHORT_V1"

// A sibling configuration: adding zero-valued fields to the old configuration
// would change its whole-configuration approval digest. Never do that.
type CohortHandoffConfigPrivateV2 struct {
	Enabled                                                           bool
	Mode, Protocol                                                    string
	MaxItems                                                          int
	Directory, Invocation, InterpreterRole, ReviewerRole, ScopeSHA256 string
	TimeoutMS, OverallTimeoutMS, PollMS                               int
	Limits                                                            AutomaticLimitsPrivateV2
	ApprovalSHA256                                                    string
}

func CohortHandoffApprovalPrivateV2(c CohortHandoffConfigPrivateV2) string {
	c.ApprovalSHA256 = ""
	return automaticDigest(c)
}
func (c CohortHandoffConfigPrivateV2) localConfig() AutomaticHandoffConfigPrivateV2 {
	out := AutomaticHandoffConfigPrivateV2{Enabled: c.Enabled, Directory: c.Directory, Invocation: c.Invocation, InterpreterRole: c.InterpreterRole, ReviewerRole: c.ReviewerRole, ScopeSHA256: c.ScopeSHA256, TimeoutMS: c.TimeoutMS, OverallTimeoutMS: c.OverallTimeoutMS, PollMS: c.PollMS, Limits: c.Limits}
	out.ApprovalSHA256 = AutomaticHandoffApprovalPrivateV2(out)
	return out
}
func ValidateCohortHandoffPrivateV2(c CohortHandoffConfigPrivateV2) error {
	if c.Mode != CohortModePrivateV2 || c.Protocol != CohortHandoffProtocolPrivateV2 || c.ApprovalSHA256 != CohortHandoffApprovalPrivateV2(c) || !validCohortLimits(c.Limits, c.MaxItems) || c.TimeoutMS > 60000 || c.OverallTimeoutMS > 180000 || ValidateAutomaticHandoffPrivateV2(c.localConfig()) != nil {
		return errors.New("private cohort exact configuration refused")
	}
	return nil
}

type CohortNavigationBindingPrivateV2 struct {
	Attempt                                             int
	PredecessorDigest, RevisionDigest, CandidatesSHA256 string
}
type CohortHandoffBindingPrivateV2 struct {
	Protocol, Invocation, OwnerNonce                string
	Sequence                                        int
	Stage, RoleID, RequestSHA256, PreparationSHA256 string
	Items                                           []CohortItemBindingPrivateV2
	Navigation                                      *CohortNavigationBindingPrivateV2
	ResponseAllowance                               int
	Deadline                                        string
}
type CohortHandoffRequestPrivateV2 struct {
	Binding CohortHandoffBindingPrivateV2
	Request json.RawMessage
}
type CohortHandoffResponsePrivateV2 struct {
	Binding           CohortHandoffBindingPrivateV2
	PayloadSHA256     string
	PayloadByteLength int
	Payload           []byte
}
type cohortHandoffBackend struct {
	shared       *automaticHandoff
	cfg          CohortHandoffConfigPrivateV2
	role         string
	lastResponse int
	physicalSHA  string
}

func (b *cohortHandoffBackend) Identity() string             { return b.role }
func (b *cohortHandoffBackend) automaticOwner() *postState   { return b.shared.owner }
func (b *cohortHandoffBackend) DeadlinePrivateV2() time.Time { return b.shared.deadline }
func (b *cohortHandoffBackend) wireResponseBytes() int       { return b.lastResponse }
func (b *cohortHandoffBackend) cohortPhysicalResponse() (string, int) {
	return b.physicalSHA, b.lastResponse
}
func (b *cohortHandoffBackend) frame(r CohortRequestPrivateV2, allowance int) CohortHandoffRequestPrivateV2 {
	s := b.shared
	if s.exchangeDeadline.IsZero() {
		s.exchangeDeadline = minTime(s.deadline, time.Now().Add(time.Duration(s.cfg.TimeoutMS)*time.Millisecond))
	}
	raw, _ := json.Marshal(r)
	binding := CohortHandoffBindingPrivateV2{Protocol: CohortHandoffProtocolPrivateV2, Invocation: s.cfg.Invocation, OwnerNonce: s.nonce, Sequence: s.sequence + 1, Stage: r.Stage, RoleID: b.role, RequestSHA256: pilot.Digest(raw), PreparationSHA256: r.PreparationSHA256, Items: postClone(r.ItemBindings), ResponseAllowance: allowance, Deadline: s.exchangeDeadline.Format(time.RFC3339Nano)}
	if r.Navigation != nil {
		n := r.Navigation
		binding.Navigation = &CohortNavigationBindingPrivateV2{n.Attempt, n.PredecessorDigest, n.RevisionDigest, automaticDigest(n.Candidates)}
	}
	return CohortHandoffRequestPrivateV2{binding, raw}
}
func (b *cohortHandoffBackend) cohortWireSize(r CohortRequestPrivateV2, allowance int) (int, error) {
	raw, e := json.Marshal(b.frame(r, allowance))
	return len(raw), e
}
func validCohortRequest(r CohortRequestPrivateV2, maxItems int) bool {
	if r.Protocol != CohortProtocolPrivateV2 {
		return false
	}
	switch r.Stage {
	case "INTERPRET_COHORT", "REVIEW_TECHNICAL_COHORT":
		if r.Navigation != nil || len(r.Items) == 0 || len(r.Items) > maxItems || len(r.Items) != len(r.ItemBindings) {
			return false
		}
		seen := map[postKey]bool{}
		for j, item := range r.Items {
			key := postKey{item.InputSHA256, item.Target}
			want := CohortItemBindingPrivateV2{item.InputSHA256, item.Target, item.Attempt, item.PredecessorDigest, automaticDigest(item)}
			if seen[key] || want != r.ItemBindings[j] || item.Attempt < 0 || item.Attempt > 1 {
				return false
			}
			seen[key] = true
			if r.Stage == "INTERPRET_COHORT" && (item.Interpretation != nil || item.InterpretationDigest != "") {
				return false
			}
			if r.Stage == "REVIEW_TECHNICAL_COHORT" && (item.Interpretation == nil || item.InterpretationDigest == "") {
				return false
			}
		}
	case "NAVIGATE", "REVIEW_NAVIGATION":
		n := r.Navigation
		if r.Items != nil || r.ItemBindings != nil || n == nil || n.Protocol != r.Protocol || n.Stage != r.Stage || n.PreparationSHA256 != r.PreparationSHA256 || n.Attempt < 0 || n.Attempt > 1 || n.InputSHA256 != "" || n.Target != "" {
			return false
		}
	default:
		return false
	}
	return true
}
func NewCohortHandoffPrivateV2(c CohortHandoffConfigPrivateV2, scope string, h PostPreparationPrivateV2) (CohortBackendPrivateV2, CohortBackendPrivateV2, error) {
	if e := ValidateCohortHandoffPrivateV2(c); e != nil {
		return nil, nil, e
	}
	if c.ScopeSHA256 != scope || h.state == nil {
		return nil, nil, errors.New("cohort handoff scope/owner mismatch")
	}
	r, info, e := openHandoff(c.Directory, true)
	if e != nil {
		return nil, nil, e
	}
	nonce := make([]byte, 32)
	if _, e = rand.Read(nonce); e != nil {
		r.Close()
		return nil, nil, e
	}
	s := &automaticHandoff{cfg: c.localConfig(), root: r, info: info, owner: h.state, nonce: hex.EncodeToString(nonce), deadline: time.Now().Add(time.Duration(c.OverallTimeoutMS) * time.Millisecond), consumed: map[string]os.FileInfo{}}
	raw, e := json.Marshal(struct {
		Protocol, Invocation, OwnerNonce, ScopeSHA256, PreparationSHA256 string
		PreparationByteLength                                            int
		Limits                                                           AutomaticLimitsPrivateV2
		Deadline                                                         string
		Preparation                                                      json.RawMessage
		CohortConfiguration                                              CohortHandoffConfigPrivateV2
	}{CohortHandoffProtocolPrivateV2, c.Invocation, s.nonce, scope, h.state.sha, len(h.state.prepared), c.Limits, s.deadline.Format(time.RFC3339Nano), bytes.Clone(h.state.prepared), c})
	if e == nil {
		e = s.publish("preparation.json", raw)
	}
	if e != nil {
		r.Close()
		return nil, nil, e
	}
	return &cohortHandoffBackend{shared: s, cfg: c, role: c.InterpreterRole}, &cohortHandoffBackend{shared: s, cfg: c, role: c.ReviewerRole}, nil
}
func (b *cohortHandoffBackend) ExchangeCohort(ctx context.Context, r CohortRequestPrivateV2, cap int) (payload []byte, err error) {
	s := b.shared
	b.lastResponse = 0
	b.physicalSHA = ""
	defer func() {
		s.exchangeDeadline = time.Time{}
		if err != nil {
			s.failed = true
			err = automaticHandoffFailure{automaticClass(err)}
		}
	}()
	if ctx == nil || cap < 1 || cap > s.cfg.Limits.MaxResponseBytes || s.sequence >= s.cfg.Limits.MaxCalls || s.owner == nil || r.PreparationSHA256 != s.owner.sha || !validCohortRequest(r, b.cfg.MaxItems) {
		return nil, AutomaticBackendErrorPrivateV2{"UNAVAILABLE"}
	}
	if e := s.check(); e != nil {
		return nil, e
	}
	if ((r.Stage == "INTERPRET_COHORT" || r.Stage == "NAVIGATE") && b.role != s.cfg.InterpreterRole) || ((r.Stage == "REVIEW_TECHNICAL_COHORT" || r.Stage == "REVIEW_NAVIGATION") && b.role != s.cfg.ReviewerRole) {
		return nil, errors.New("cohort handoff role mismatch")
	}
	frame := b.frame(r, cap)
	ctx, cancel := context.WithDeadline(ctx, s.exchangeDeadline)
	defer cancel()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, _ := json.Marshal(frame)
	if len(raw) > s.cfg.Limits.MaxRequestBytes {
		return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
	}
	name := fmt.Sprintf("request-%06d.json", frame.Binding.Sequence)
	responseName := fmt.Sprintf("response-%06d.json", frame.Binding.Sequence)
	if _, e := s.root.Lstat(responseName); !errors.Is(e, os.ErrNotExist) {
		return nil, errors.New("cohort handoff early response")
	}
	if e := s.publish(name, raw); e != nil {
		return nil, e
	}
	s.sequence++
	ticker := time.NewTicker(time.Duration(s.cfg.PollMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if e := s.check(); e != nil {
			return nil, e
		}
		raw, e := s.read(responseName, cap)
		b.lastResponse = len(raw)
		b.physicalSHA = pilot.Digest(raw)
		if errors.Is(e, os.ErrNotExist) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ticker.C:
				continue
			}
		}
		if len(raw) > cap {
			return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
		}
		if e != nil {
			return nil, e
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		var response CohortHandoffResponsePrivateV2
		if strictAutomatic(raw, cap, &response) != nil || automaticDigest(response.Binding) != automaticDigest(frame.Binding) || response.PayloadByteLength != len(response.Payload) || response.PayloadSHA256 != pilot.Digest(response.Payload) {
			return nil, errors.New("cohort handoff frame binding/digest refused")
		}
		return bytes.Clone(response.Payload), nil
	}
}
func FinishCohortHandoffPrivateV2(backend CohortBackendPrivateV2, outcome *PostPublicationOutcomePrivateV2, runErr error) error {
	b, ok := backend.(*cohortHandoffBackend)
	if !ok {
		return nil
	}
	s := b.shared
	if s.closed {
		return errors.New("cohort handoff already closed")
	}
	checkErr := s.check()
	if !time.Now().Before(s.deadline) {
		checkErr = errors.Join(checkErr, context.DeadlineExceeded)
	}
	if runErr == nil && checkErr != nil {
		runErr = checkErr
	}
	status, failure, sha, length := "FAILED_CLOSED", automaticClass(runErr), "", 0
	if runErr == nil && !s.failed && outcome != nil && outcome.Status == "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" {
		var envelope postPublicationEnvelopePrivateV2
		content := outcome.EnvelopeBytes()
		if len(content) == 0 || len(content) != int(outcome.ByteLength) || pilot.Digest(content) != outcome.EnvelopeSHA256 || json.Unmarshal(content, &envelope) != nil || envelope.PreparationSHA256 != s.owner.sha || envelope.Inventory == nil || envelope.Inventory.Protocol != CohortProtocolPrivateV2 {
			checkErr = errors.Join(checkErr, errors.New("cohort terminal outcome binding refused"))
		} else {
			status = "VERIFIED_PAIR_PRIVATE_V2_AST"
			failure = ""
			sha = outcome.EnvelopeSHA256
			length = int(outcome.ByteLength)
		}
	}
	raw, _ := json.Marshal(struct {
		Protocol, Invocation, OwnerNonce, PreparationSHA256, Status, Failure, EnvelopeSHA256 string
		ByteLength, Exchanges                                                                int
	}{CohortHandoffProtocolPrivateV2, s.cfg.Invocation, s.nonce, s.owner.sha, status, failure, sha, length, s.sequence})
	e := s.publish("terminal.json", raw)
	s.closed = true
	s.owner = nil
	return errors.Join(checkErr, e, s.root.Close())
}
