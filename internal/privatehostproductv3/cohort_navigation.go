//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"
)

const CohortNavigationProtocol = "PRIVATE_V3_REVIEWED_COHORT_NAVIGATION_1"

type CohortRevision struct {
	ItemID       string
	Revision     int
	ReviewSHA256 string
}
type CohortNavigationCandidate struct {
	Revision                                   CohortRevision
	Label, Explanation, Alternative, Falsifier string
}
type CohortNavigation struct {
	ID, Label, Explanation, Alternative, Falsifier, Withdrawal, Revisit string
	Members                                                             []CohortRevision
	Supported                                                           bool
}
type CohortNavigationTask struct {
	Protocol, ConfigSHA256, Grant, Role, RequestDigest string
	Sequence                                           int
	Candidates                                         []CohortNavigationCandidate
	Proposal                                           []CohortNavigation
}
type CohortNavigationResponse struct {
	Protocol, RequestDigest string
	Groups                  []CohortNavigation
}

func navigationCandidates(outcomes []CohortOutcome) []CohortNavigationCandidate {
	out := []CohortNavigationCandidate{}
	for _, o := range outcomes {
		if o.Status != "REVIEWED" || len(o.History) < 2 {
			continue
		}
		d := o.History[len(o.History)-1]
		out = append(out, CohortNavigationCandidate{CohortRevision{o.Item.ItemID, o.Item.Revision, hash(mustJSON(d))}, d.Label, d.Explanation, d.Alternative, d.Falsifier})
	}
	return out
}
func DecodeCohortNavigation(raw []byte) (CohortNavigationResponse, error) {
	var r CohortNavigationResponse
	if len(raw) > 64<<10 || !utf8.Valid(raw) || !json.Valid(raw) {
		return r, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if cohortSchema(d, reflect.TypeOf(r), 0) != nil || d.Decode(new(any)) != io.EOF || json.Unmarshal(raw, &r) != nil || r.Protocol != CohortNavigationProtocol {
		return CohortNavigationResponse{}, ErrInvalid
	}
	return r, nil
}
func validateCohortNavigation(t CohortNavigationTask, r CohortNavigationResponse) error {
	if r.Protocol != CohortNavigationProtocol || r.RequestDigest != t.RequestDigest || len(r.Groups) > 10 {
		return ErrInvalid
	}
	refs := map[CohortRevision]bool{}
	for _, candidate := range t.Candidates {
		refs[candidate.Revision] = true
	}
	if t.Role == "reviewer" && len(r.Groups) != len(t.Proposal) {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for n, g := range r.Groups {
		if seen[g.ID] || len(g.Members) < 2 || len(g.Members) > 10 {
			return ErrInvalid
		}
		seen[g.ID] = true
		for _, value := range []string{g.ID, g.Label, g.Explanation, g.Alternative, g.Falsifier, g.Withdrawal, g.Revisit} {
			if strings.TrimSpace(value) == "" || len(value) > 1024 {
				return ErrInvalid
			}
		}
		own := map[string]bool{}
		for _, member := range g.Members {
			if !refs[member] || own[member.ItemID] {
				return ErrInvalid
			}
			own[member.ItemID] = true
		}
		if t.Role == "reviewer" && (g.ID != t.Proposal[n].ID || !bytes.Equal(mustJSON(g.Members), mustJSON(t.Proposal[n].Members))) {
			return ErrInvalid
		}
	}
	return nil
}
func bindNavigation(c CohortConfig, candidates []CohortNavigationCandidate, proposal []CohortNavigation, role string, seq int) CohortNavigationTask {
	t := CohortNavigationTask{Protocol: CohortNavigationProtocol, ConfigSHA256: CohortConfigDigest(c), Grant: c.Grant, Role: role, Sequence: seq, Candidates: candidates, Proposal: proposal}
	t.RequestDigest = hash(mustJSON(t))
	return t
}

// Shared physical accounting for the optional navigation family. It uses the
// SAME progress counters, claim namespace, ledger wrapper and grant as cohorts.
func cohortNavigationExchange(ctx context.Context, c CohortConfig, p *cohortProgress, s *store, b CohortBackend, t CohortNavigationTask) (CohortNavigationResponse, error) {
	raw := mustJSON(t)
	if len(raw) > c.MaxRequestBytes || p.Calls >= c.MaxCalls || p.RequestBytes+p.ResponseBytes > c.MaxTotalBytes-len(raw)-c.MaxResponseBytes || p.ResponseBytes > c.MaxOutputBytes-c.MaxResponseBytes {
		return CohortNavigationResponse{}, ErrPaused
	}
	if e := ctx.Err(); e != nil {
		return CohortNavigationResponse{}, e
	}
	if e := s.write(fmt.Sprintf("claim/claim-%08d.json", t.Sequence), raw); e != nil {
		return CohortNavigationResponse{}, e
	}
	p.Calls++
	p.RequestBytes += len(raw)
	p.Inflight = true
	if e := cohortSave(s, *p); e != nil {
		return CohortNavigationResponse{}, e
	}
	reply, backendErr := b.ExchangeCohort(ctx, raw)
	reply, retainErr := retainCohortReturn(s, c, p, t.Sequence, raw, reply, backendErr == nil)
	if retainErr != nil {
		return CohortNavigationResponse{}, retainErr
	}
	if backendErr != nil {
		return CohortNavigationResponse{}, backendErr
	}
	if len(reply) > c.MaxResponseBytes || p.RequestBytes+p.ResponseBytes > c.MaxTotalBytes || p.ResponseBytes > c.MaxOutputBytes {
		return CohortNavigationResponse{}, ErrPaused
	}
	r, e := DecodeCohortNavigation(reply)
	if e != nil {
		return r, e
	}
	if e = validateCohortNavigation(t, r); e != nil {
		return r, e
	}
	p.Inflight = false
	p.NavigationHistory = append(p.NavigationHistory, r)
	return r, cohortSave(s, *p)
}
func runCohortNavigation(ctx context.Context, c CohortConfig, p *cohortProgress, s *store, b CohortBackend) error {
	if !c.Navigation {
		p.NavigationStatus = "NOT_REQUESTED"
		return nil
	}
	candidates := navigationCandidates(p.Outcomes)
	if len(candidates) < 2 {
		p.NavigationStatus = "UNAVAILABLE_REVIEWED_CANDIDATES"
		return nil
	}
	if ValidateCohortConfig(c) != nil || p.Calls < 0 || p.Calls > c.MaxCalls-2 || p.RequestBytes < 0 || p.ResponseBytes < 0 || p.ResponseBytes > c.MaxOutputBytes || p.ResponseBytes > c.MaxTotalBytes || p.RequestBytes > c.MaxTotalBytes-p.ResponseBytes {
		p.NavigationStatus = "UNAVAILABLE_REMAINING_GRANT"
		return nil
	}
	t := bindNavigation(c, candidates, []CohortNavigation{}, "interpreter", p.Calls+1)
	proposalBytes := len(mustJSON(t))
	reviewBase := bindNavigation(c, candidates, []CohortNavigation{}, "reviewer", p.Calls+2)
	// JSON re-encoding can expand a literal '<', '>' or '&' to six bytes.
	// Bound by BOTH the physical response ceiling and the protocol's field,
	// member and group maxima. This is admission slack, never a charged payload.
	longest := CohortRevision{}
	for _, candidate := range candidates {
		if len(mustJSON(candidate.Revision)) > len(mustJSON(longest)) {
			longest = candidate.Revision
		}
	}
	maxGroup := CohortNavigation{ID: strings.Repeat("\x00", 1024), Label: strings.Repeat("\x00", 1024), Explanation: strings.Repeat("\x00", 1024), Alternative: strings.Repeat("\x00", 1024), Falsifier: strings.Repeat("\x00", 1024), Withdrawal: strings.Repeat("\x00", 1024), Revisit: strings.Repeat("\x00", 1024)}
	for n := 0; n < 10; n++ {
		maxGroup.Members = append(maxGroup.Members, longest)
	}
	maxGroups := make([]CohortNavigation, 10)
	for n := range maxGroups {
		maxGroups[n] = maxGroup
	}
	reviewBytes := len(mustJSON(reviewBase)) + min(6*c.MaxResponseBytes, len(mustJSON(maxGroups)))
	if proposalBytes > c.MaxRequestBytes || reviewBytes > c.MaxRequestBytes {
		p.NavigationStatus = "UNAVAILABLE_ENVELOPE"
		return nil
	}
	if 2*c.MaxResponseBytes > c.MaxOutputBytes-p.ResponseBytes || proposalBytes+reviewBytes+2*c.MaxResponseBytes > c.MaxTotalBytes-p.RequestBytes-p.ResponseBytes {
		p.NavigationStatus = "UNAVAILABLE_REMAINING_GRANT"
		return nil
	}
	p.NavigationStatus = "PENDING_REVIEW"
	proposal, e := cohortNavigationExchange(ctx, c, p, s, b, t)
	if e != nil {
		return e
	}
	t = bindNavigation(c, candidates, proposal.Groups, "reviewer", p.Calls+1)
	if _, e = cohortNavigationExchange(ctx, c, p, s, b, t); e != nil {
		return e
	}
	p.NavigationStatus = "REVIEWED_HYPOTHESES_NOT_FEATURE_IDENTITY"
	return nil
}
func verifyNavigationExchange(s *store, c CohortConfig, r CohortReport, seq int, raw []byte, history *[]CohortNavigationResponse) (int, int, error) {
	if !c.Navigation || len(*history) > 1 {
		return 0, 0, ErrInvalid
	}
	var t CohortNavigationTask
	if strict(raw, &t) != nil || t.Protocol != CohortNavigationProtocol || t.Sequence != seq || t.ConfigSHA256 != CohortConfigDigest(c) || t.Grant != c.Grant || !bytes.Equal(mustJSON(t.Candidates), mustJSON(navigationCandidates(r.Outcomes))) {
		return 0, 0, ErrInvalid
	}
	role := "interpreter"
	proposal := []CohortNavigation{}
	if len(*history) == 1 {
		role = "reviewer"
		proposal = (*history)[0].Groups
	}
	if t.Role != role || !bytes.Equal(mustJSON(t.Proposal), mustJSON(proposal)) {
		return 0, 0, ErrInvalid
	}
	digest := t.RequestDigest
	t.RequestDigest = ""
	if hash(mustJSON(t)) != digest {
		return 0, 0, ErrInvalid
	}
	t.RequestDigest = digest
	reply, e := observedCohortReply(s, c, seq, raw)
	if os.IsNotExist(e) && seq == r.Calls && r.Inflight {
		spent, spendErr := ambiguousMissingReplySpend(s, seq)
		return len(raw), spent, spendErr
	}
	if e != nil {
		return 0, 0, e
	}
	rr, e := s.read(fmt.Sprintf("host/claim-%08d.json", seq), 4096)
	if os.IsNotExist(e) && seq == r.Calls && r.Inflight {
		return len(raw), len(reply), nil
	}
	var receipt cohortReceipt
	if e != nil || strict(rr, &receipt) != nil || receipt.Sequence != seq || receipt.ResponseSHA256 != hash(reply) || receipt.Bytes != len(reply) {
		return 0, 0, ErrInvalid
	}
	response, e := DecodeCohortNavigation(reply)
	if !receipt.BackendOK || len(reply) > c.MaxResponseBytes || seq == r.Calls && r.Inflight || e != nil || validateCohortNavigation(t, response) != nil {
		if seq != r.Calls {
			return 0, 0, ErrInvalid
		}
		return len(raw), len(reply), nil
	}
	*history = append(*history, response)
	return len(raw), len(reply), nil
}
