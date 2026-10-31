package privatedescribeprep

// Cohorts are transport membership only. Every interpretation, citation,
// predecessor and review remains attributed to one sealed preparation row.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"

	pilot "lsp-trace/internal/censuslineagepilot"
)

const CohortProtocolPrivateV2 = "private-provisional-inventory-cohort.v1"

type CohortItemPrivateV2 struct {
	InputSHA256, Target  string
	Attempt              int
	PredecessorDigest    string
	Packet               *Packet
	Bodies               []OwnBodyPrivateV2
	Feedback             string
	PriorRevisions       []AutomaticRevisionPrivateV2
	Interpretation       *AutomaticInterpretationPrivateV2
	InterpretationDigest string
}
type CohortItemBindingPrivateV2 struct {
	InputSHA256, Target              string
	Attempt                          int
	PredecessorDigest, RequestSHA256 string
}
type CohortRequestPrivateV2 struct {
	Protocol, Stage, PreparationSHA256 string
	Items                              []CohortItemPrivateV2
	ItemBindings                       []CohortItemBindingPrivateV2
	PostLimits                         PostPreparationLimits
	LedgerQuoteChunkBytes              int
	Navigation                         *AutomaticRequestPrivateV2
}
type CohortResponseItemPrivateV2 struct {
	Binding        CohortItemBindingPrivateV2
	Interpretation *AutomaticInterpretationPrivateV2
	Review         *AutomaticReviewPrivateV2
}
type CohortResponsePrivateV2 struct {
	Protocol, Stage, PreparationSHA256 string
	Items                              []CohortResponseItemPrivateV2
}
type CohortBackendPrivateV2 interface {
	Identity() string
	ExchangeCohort(context.Context, CohortRequestPrivateV2, int) ([]byte, error)
}

// Optional wire accounting is implemented only by the immutable local handoff.
// Fixture backends are charged their exact typed request and response bytes.
type cohortWireSizer interface {
	cohortWireSize(CohortRequestPrivateV2, int) (int, error)
}
type CohortFrameEvidencePrivateV2 struct {
	Stage, ResponseSHA256      string
	ResponseByteLength         int
	Response                   json.RawMessage
	PhysicalResponseSHA256     string
	PhysicalResponseByteLength int
	ItemResponseSHA256         []string
}
type CohortEvidencePrivateV2 struct {
	MaxItems int
	Frames   []CohortFrameEvidencePrivateV2
}

func validCohortLimits(l AutomaticLimitsPrivateV2, maxItems int) bool {
	return l.valid() && l.MaxCorrections <= 1 && l.MaxCalls <= 16 && l.MaxRequestBytes <= 1<<20 && l.MaxResponseBytes <= 64<<10 && l.MaxTotalBytes <= 8<<20 && l.MaxOutputBytes <= 4<<20 && maxItems > 0 && maxItems <= 63
}
func cohortRequest(stage, sha string, items []CohortItemPrivateV2, l PostPreparationLimits) CohortRequestPrivateV2 {
	r := CohortRequestPrivateV2{Protocol: CohortProtocolPrivateV2, Stage: stage, PreparationSHA256: sha, Items: postClone(items), PostLimits: l, LedgerQuoteChunkBytes: automaticLedgerQuoteChunkBytes}
	for _, item := range r.Items {
		r.ItemBindings = append(r.ItemBindings, CohortItemBindingPrivateV2{item.InputSHA256, item.Target, item.Attempt, item.PredecessorDigest, automaticDigest(item)})
	}
	return r
}
func cohortSize(backend CohortBackendPrivateV2, r CohortRequestPrivateV2, cap int) (int, error) {
	if w, ok := backend.(cohortWireSizer); ok {
		return w.cohortWireSize(r, cap)
	}
	raw, e := json.Marshal(r)
	return len(raw), e
}

// Determine the actual allowance and physical frame size together. Allowance is
// encoded in the frame, so its decimal width is part of admission, not metadata.
func (b *automaticBudget) cohortAdmission(backend CohortBackendPrivateV2, r CohortRequestPrivateV2) (int, int, error) {
	allowance := b.limits.MaxResponseBytes
	for n := 0; n < 16; n++ {
		size, e := cohortSize(backend, r, allowance)
		if e != nil {
			return 0, 0, e
		}
		if size < 0 {
			return 0, 0, AutomaticBackendErrorPrivateV2{"MALFORMED"}
		}
		remaining := b.limits.MaxTotalBytes - b.bytes - size
		next := min(allowance, remaining)
		if size > b.limits.MaxRequestBytes || next < 1 {
			return size, next, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
		}
		if next == allowance {
			return size, allowance, nil
		}
		allowance = next
	}
	return 0, 0, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
}
func (b *automaticBudget) cohortCall(ctx context.Context, backend CohortBackendPrivateV2, r CohortRequestPrivateV2) (raw []byte, err error) {
	if b.terminal != nil {
		return nil, b.terminal
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if backend == nil {
		return nil, AutomaticBackendErrorPrivateV2{"UNAVAILABLE"}
	}
	size, allowance, e := b.cohortAdmission(backend, r)
	if e != nil {
		return nil, e
	}
	if b.calls >= b.limits.MaxCalls {
		return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
	}
	b.calls++
	b.bytes += size
	raw, err = backend.ExchangeCohort(ctx, postClone(r), allowance)
	var terminal interface{ TerminalHandoffFailure() }
	if errors.As(err, &terminal) {
		b.terminal = err
	}
	consumed := len(raw)
	if w, ok := backend.(interface{ wireResponseBytes() int }); ok {
		consumed = w.wireResponseBytes()
	}
	if consumed < 0 {
		return nil, AutomaticBackendErrorPrivateV2{"MALFORMED"}
	}
	b.bytes += min(consumed, b.limits.MaxTotalBytes-b.bytes)
	if consumed > allowance {
		return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
	}
	if err != nil {
		return bytes.Clone(raw), err
	}
	if e := ctx.Err(); e != nil {
		return bytes.Clone(raw), e
	}
	return bytes.Clone(raw), nil
}

// Largest contiguous prefix, never dropping or splitting a row. Review packing
// runs again over review requests, including interpretation and prior history.
func (b *automaticBudget) cohortPack(backend CohortBackendPrivateV2, stage, sha string, items []CohortItemPrivateV2, maxItems int, l PostPreparationLimits) (CohortRequestPrivateV2, int, error) {
	var best CohortRequestPrivateV2
	count := 0
	for n := 1; n <= len(items) && n <= maxItems; n++ {
		req := cohortRequest(stage, sha, items[:n], l)
		if _, _, e := b.cohortAdmission(backend, req); e != nil {
			if count == 0 {
				return best, 0, e
			}
			break
		}
		best = req
		count = n
	}
	if count == 0 {
		return best, 0, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
	}
	return best, count, nil
}
func cohortResponse(raw []byte, r CohortRequestPrivateV2, cap int) (CohortResponsePrivateV2, error) {
	var out CohortResponsePrivateV2
	if strictAutomatic(raw, cap, &out) != nil || out.Protocol != r.Protocol || out.Stage != r.Stage || out.PreparationSHA256 != r.PreparationSHA256 || len(out.Items) != len(r.Items) {
		return out, AutomaticBackendErrorPrivateV2{"MALFORMED"}
	}
	// Validate the entire ordered vector before accepting even the first item.
	for j, item := range out.Items {
		if item.Binding != r.ItemBindings[j] || (r.Stage == "INTERPRET_COHORT" && (item.Interpretation == nil || item.Review != nil)) || (r.Stage == "REVIEW_TECHNICAL_COHORT" && (item.Interpretation != nil || item.Review == nil)) {
			return out, AutomaticBackendErrorPrivateV2{"MALFORMED"}
		}
	}
	return out, nil
}
func cohortFrameEvidence(raw []byte, r CohortRequestPrivateV2, out CohortResponsePrivateV2, backend CohortBackendPrivateV2) CohortFrameEvidencePrivateV2 {
	f := CohortFrameEvidencePrivateV2{Stage: r.Stage, ResponseSHA256: pilot.Digest(raw), ResponseByteLength: len(raw)}
	if json.Valid(raw) {
		f.Response = bytes.Clone(raw)
	}
	if w, ok := backend.(interface{ cohortPhysicalResponse() (string, int) }); ok {
		f.PhysicalResponseSHA256, f.PhysicalResponseByteLength = w.cohortPhysicalResponse()
	}
	for _, item := range out.Items {
		f.ItemResponseSHA256 = append(f.ItemResponseSHA256, automaticDigest(item))
	}
	return f
}

type cohortRow struct {
	item     CohortItemPrivateV2
	own      map[string]string
	row      int
	approved *AutomaticInterpretationPrivateV2
}

func AutomateCohortPrivateV2(ctx context.Context, h PostPreparationPrivateV2, l AutomaticLimitsPrivateV2, maxItems int, interpreter, reviewer CohortBackendPrivateV2) (a AutomaticResultPrivateV2, err error) {
	a.inventory = AutomaticInventoryPrivateV2{PrivateCeilings: privateCeilings(), Protocol: CohortProtocolPrivateV2, Status: "FAILED", Absence: "PROHIBITED", Independence: "CONFIGURED_DISTINCT_IDENTITIES_NOT_AUTHENTICATED", Cohort: &CohortEvidencePrivateV2{MaxItems: maxItems}}
	if ctx == nil || h.state == nil || !validCohortLimits(l, maxItems) {
		a.inventory.Failure = "INVOCATION_INVALID"
		return a, errors.New(a.inventory.Failure)
	}
	s := h.state
	a.inventory.PreparationSHA256 = s.sha
	for _, ix := range s.index {
		a.inventory.Rows = append(a.inventory.Rows, AutomaticRowPrivateV2{InputSHA256: ix.InputSHA256, Target: ix.Target, Status: ix.Status, Reason: ix.Gap})
	}
	b := automaticBudget{limits: l}
	defer func() {
		a.inventory.Calls = b.calls
		a.inventory.Bytes = b.bytes
		if a.inventory.Status == "FAILED" {
			a.owner = nil
			a.inventory.Candidates = nil
			a.inventory.Navigation = nil
			for j := range a.inventory.Rows {
				row := &a.inventory.Rows[j]
				if row.Status == "PREPARED" || row.Status == "PROVISIONAL" {
					row.Status = a.inventory.Failure
					row.Reason = a.inventory.Failure
					row.CandidateID = ""
				}
			}
		}
		a.human = renderAutomatic(a.inventory)
	}()
	fail := func(e error) (AutomaticResultPrivateV2, error) {
		a.inventory.Failure = automaticClass(e)
		if b.terminal != nil {
			a.inventory.Failure = "HANDOFF_TERMINAL"
		}
		return a, e
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if interpreter == nil || reviewer == nil || interpreter.Identity() == "" || reviewer.Identity() == "" {
		return fail(AutomaticBackendErrorPrivateV2{"UNAVAILABLE"})
	}
	if interpreter.Identity() == reviewer.Identity() {
		a.inventory.Failure = "INDEPENDENCE_REFUSED"
		return a, errors.New(a.inventory.Failure)
	}
	for _, backend := range []CohortBackendPrivateV2{interpreter, reviewer} {
		if bound, ok := backend.(interface{ automaticOwner() *postState }); ok && bound.automaticOwner() != s {
			a.inventory.Failure = "HANDOFF_FOREIGN_OWNER"
			return a, errors.New(a.inventory.Failure)
		}
	}
	var prepared Result
	if json.Unmarshal(s.prepared, &prepared) != nil {
		a.inventory.Failure = "PREPARATION_INVALID"
		return a, errors.New(a.inventory.Failure)
	}
	packets := map[postKey]Packet{}
	for _, p := range prepared.Packets {
		packets[postKey{p.InputSHA256, p.Target}] = p
	}
	states := make([]cohortRow, len(a.inventory.Rows))
	var pending []int
	unresolved := false
	for j := range a.inventory.Rows {
		row := &a.inventory.Rows[j]
		if row.Status != "PREPARED" {
			unresolved = true
			continue
		}
		key := postKey{row.InputSHA256, row.Target}
		p := packets[key]
		st := cohortRow{row: j, item: CohortItemPrivateV2{InputSHA256: key.input, Target: key.target, Packet: &p}, own: map[string]string{}}
		for _, m := range s.packets[key].manifest.Handles {
			e := s.packets[key].evidence[m.Handle]
			if e.kind == "RETAINED_SOURCE_AST_DECLARATION" {
				if _, ok := st.own[m.Handle]; !ok {
					st.own[m.Handle] = e.body
					st.item.Bodies = append(st.item.Bodies, OwnBodyPrivateV2{m.Handle, e.body})
				}
			}
		}
		states[j] = st
		if len(st.item.Bodies) == 0 {
			row.Status = "UNSUPPORTED"
			row.Reason = "NO_OWN_AST_BODY"
			unresolved = true
		} else {
			pending = append(pending, j)
		}
	}
	for attempt := 0; attempt <= l.MaxCorrections && len(pending) > 0; attempt++ {
		var reviewPending []int
		for start := 0; start < len(pending); {
			var items []CohortItemPrivateV2
			for _, j := range pending[start:] {
				st := &states[j]
				st.item.Attempt = attempt
				st.item.PriorRevisions = postClone(a.inventory.Rows[j].Revisions)
				items = append(items, st.item)
			}
			req, n, e := b.cohortPack(interpreter, "INTERPRET_COHORT", s.sha, items, maxItems, s.limits)
			if e != nil {
				return fail(e)
			}
			raw, e := b.cohortCall(ctx, interpreter, req)
			if e != nil {
				a.inventory.Cohort.Frames = append(a.inventory.Cohort.Frames, cohortFrameEvidence(raw, req, CohortResponsePrivateV2{}, interpreter))
				return fail(e)
			}
			out, e := cohortResponse(raw, req, l.MaxResponseBytes)
			a.inventory.Cohort.Frames = append(a.inventory.Cohort.Frames, cohortFrameEvidence(raw, req, out, interpreter))
			if e != nil {
				return fail(e)
			}
			for k, response := range out.Items {
				j := pending[start+k]
				st := &states[j]
				row := &a.inventory.Rows[j]
				v := *response.Interpretation
				itemRaw, _ := json.Marshal(response)
				sha := pilot.Digest(itemRaw)
				rev := AutomaticRevisionPrivateV2{PredecessorDigest: st.item.PredecessorDigest, Response: itemRaw, ResponseSHA256: sha, ResponseByteLength: len(itemRaw), Interpretation: &v}
				rev.Digest = automaticDigest(struct{ Previous, RawDigest string }{rev.PredecessorDigest, sha})
				rev.Status = validateExactAutomatic(v, st.own, s.limits)
				row.Revisions = append(row.Revisions, rev)
				if rev.Status == "" {
					st.item.Interpretation = &v
					st.item.InterpretationDigest = rev.Digest
					reviewPending = append(reviewPending, j)
				} else {
					row.Status = rev.Status
					row.Reason = rev.Status
				}
			}
			start += n
		}
		for start := 0; start < len(reviewPending); {
			var items []CohortItemPrivateV2
			for _, j := range reviewPending[start:] {
				items = append(items, states[j].item)
			}
			req, n, e := b.cohortPack(reviewer, "REVIEW_TECHNICAL_COHORT", s.sha, items, maxItems, s.limits)
			if e != nil {
				return fail(e)
			}
			raw, e := b.cohortCall(ctx, reviewer, req)
			if e != nil {
				a.inventory.Cohort.Frames = append(a.inventory.Cohort.Frames, cohortFrameEvidence(raw, req, CohortResponsePrivateV2{}, reviewer))
				return fail(e)
			}
			out, e := cohortResponse(raw, req, l.MaxResponseBytes)
			a.inventory.Cohort.Frames = append(a.inventory.Cohort.Frames, cohortFrameEvidence(raw, req, out, reviewer))
			if e != nil {
				return fail(e)
			}
			for k, response := range out.Items {
				j := reviewPending[start+k]
				row := &a.inventory.Rows[j]
				rev := &row.Revisions[len(row.Revisions)-1]
				reviewRaw, _ := json.Marshal(response.Review)
				v, status := automaticReview(reviewRaw, l.MaxResponseBytes, rev.Digest, s.limits.MaxStringBytes)
				rev.Review = &v
				rev.Findings = v.Findings
				rev.Status = status
				if status == "" {
					rev.Status = "APPROVED"
					row.Status = "PROVISIONAL"
					row.Reason = "CONFIGURED_REVIEW_NOT_SEMANTIC_TRUTH"
					states[j].approved = states[j].item.Interpretation
				} else {
					row.Status = status
					row.Reason = status
				}
			}
			start += n
		}
		var next []int
		for _, j := range pending {
			row := &a.inventory.Rows[j]
			if row.Status == "PROVISIONAL" {
				continue
			}
			rev := row.Revisions[len(row.Revisions)-1]
			st := &states[j]
			st.item.PredecessorDigest = rev.Digest
			st.item.Feedback = rev.Status + ": " + rev.Findings
			st.item.Interpretation = nil
			st.item.InterpretationDigest = ""
			if attempt < l.MaxCorrections && (rev.Status == "MALFORMED" || rev.Status == "REJECTED") {
				next = append(next, j)
			} else {
				unresolved = true
			}
		}
		pending = next
	}
	// Assemble in original denominator order, not review completion order.
	for j := range states {
		st := &states[j]
		if st.approved == nil {
			continue
		}
		row := &a.inventory.Rows[j]
		v := *st.approved
		rev := row.Revisions[len(row.Revisions)-1]
		pp := s.packets[postKey{row.InputSHA256, row.Target}]
		sub := CallerSubmission{PrivateCeilings: privateCeilings(), PreparationSHA256: s.sha, InputSHA256: row.InputSHA256, Target: row.Target, PacketID: pp.manifest.PacketID, OrderedPartSHA256: slices.Clone(pp.manifest.OrderedPartSHA256), AuthorTag: interpreter.Identity(), Disposition: "DESCRIBED", WorkingLabel: v.Label, TechnicalDescription: v.Description, Limitations: v.Limitations, Alternatives: v.Alternatives, RejectionCondition: v.RejectionCondition}
		sub.Claims, err = automaticLegacyClaims(v, s.limits)
		if err != nil {
			return fail(err)
		}
		a.submissions = append(a.submissions, sub)
		a.reviews = append(a.reviews, CallerReview{PrivateCeilings: privateCeilings(), PreparationSHA256: s.sha, InputSHA256: row.InputSHA256, Target: row.Target, SubmissionSHA256: automaticDigest(sub), ReviewerAuthorTag: reviewer.Identity(), Verdict: "SUPPORT_WITH_LIMITS", Findings: rev.Findings})
		id := automaticDigest(struct{ InputSHA256, Target string }{row.InputSHA256, row.Target})
		row.CandidateID = id
		a.inventory.Candidates = append(a.inventory.Candidates, AutomaticCandidatePrivateV2{PrivateCeilings: privateCeilings(), ID: id, InputSHA256: row.InputSHA256, Target: row.Target, RevisionDigest: rev.Digest, Label: v.Label, Description: v.Description, Claims: postClone(v.Claims), Limitations: v.Limitations, Alternatives: v.Alternatives, RejectionCondition: v.RejectionCondition})
	}
	if len(a.inventory.Candidates) > 0 {
		previous, feedback := "", ""
		for attempt := 0; attempt <= l.MaxCorrections; attempt++ {
			nav := AutomaticRequestPrivateV2{Protocol: CohortProtocolPrivateV2, Stage: "NAVIGATE", PreparationSHA256: s.sha, Candidates: postClone(a.inventory.Candidates), Attempt: attempt, PredecessorDigest: previous, Feedback: feedback, PostLimits: s.limits, LedgerQuoteChunkBytes: automaticLedgerQuoteChunkBytes, PriorNavigationRevisions: postClone(a.inventory.NavigationRevisions)}
			req := cohortRequest("NAVIGATE", s.sha, nil, s.limits)
			req.Navigation = &nav
			raw, e := b.cohortCall(ctx, interpreter, req)
			a.inventory.Cohort.Frames = append(a.inventory.Cohort.Frames, cohortFrameEvidence(raw, req, CohortResponsePrivateV2{}, interpreter))
			if e != nil {
				return fail(e)
			}
			nr := AutomaticNavigationRevisionPrivateV2{Response: bytes.Clone(raw), PredecessorDigest: previous, ResponseSHA256: pilot.Digest(raw), ResponseByteLength: len(raw)}
			nr.Digest = automaticDigest(struct{ Previous, RawDigest string }{previous, nr.ResponseSHA256})
			var groups []NavigationPrivateV2
			status := ""
			if strictAutomatic(raw, l.MaxResponseBytes, &groups) != nil {
				status = "MALFORMED"
			} else {
				status = validateNavigation(groups, a.inventory.Candidates)
			}
			if !json.Valid(nr.Response) {
				nr.Response = nil
			}
			nr.Hypotheses = postClone(groups)
			if status == "" {
				nav.Stage = "REVIEW_NAVIGATION"
				nav.Navigation = groups
				nav.RevisionDigest = nr.Digest
				req.Stage = nav.Stage
				rr, e := b.cohortCall(ctx, reviewer, req)
				a.inventory.Cohort.Frames = append(a.inventory.Cohort.Frames, cohortFrameEvidence(rr, req, CohortResponsePrivateV2{}, reviewer))
				if e != nil {
					return fail(e)
				}
				v, st := automaticReview(rr, l.MaxResponseBytes, nr.Digest, s.limits.MaxStringBytes)
				nr.Review = &v
				nr.Findings = v.Findings
				status = st
			}
			nr.Status = status
			if status == "" {
				nr.Status = "APPROVED"
				a.inventory.Navigation = postClone(groups)
			}
			a.inventory.NavigationRevisions = append(a.inventory.NavigationRevisions, nr)
			if status == "" {
				break
			}
			previous = nr.Digest
			feedback = status + ": " + nr.Findings
			if status == "ABSTAINED" || attempt == l.MaxCorrections {
				unresolved = true
				break
			}
		}
	}
	a.inventory.Status = "PROVISIONAL_COMPLETE"
	if unresolved {
		a.inventory.Status = "PROVISIONAL_WITH_UNRESOLVED"
	}
	a.inventory.Calls = b.calls
	a.inventory.Bytes = b.bytes
	encoded, _ := json.Marshal(a.inventory)
	human, _ := json.Marshal(renderAutomatic(a.inventory))
	if len(encoded) > l.MaxOutputBytes || len(human) > l.MaxOutputBytes-len(encoded) {
		a.inventory.Status = "FAILED"
		return fail(AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"})
	}
	ledger, e := h.Reconcile(ctx, a.submissions, a.reviews)
	if e != nil || len(ledger.Leads) != len(a.inventory.Candidates) {
		a.inventory.Status = "FAILED"
		a.inventory.Failure = "RECONCILIATION_FAILED"
		return a, errors.New(a.inventory.Failure)
	}
	a.owner = s
	return a, nil
}
