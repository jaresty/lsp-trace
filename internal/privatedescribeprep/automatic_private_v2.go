package privatedescribeprep

// Automatic private interpretation is a continuation of the sealed preparation,
// never a constructor from saved diagnostics or new acquisition authority.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
)

const AutomaticProtocolPrivateV2 = "private-provisional-inventory.v2"

// Existing CallerSubmission gate; original automatic quotes are NOT truncated.
const automaticLedgerQuoteChunkBytes = 240

// Bounds are explicit; no retry, budget expansion or fallback is implicit.
type AutomaticLimitsPrivateV2 struct {
	MaxCorrections   int
	MaxCalls         int
	MaxRequestBytes  int
	MaxResponseBytes int
	MaxTotalBytes    int
	MaxOutputBytes   int
}

func (l AutomaticLimitsPrivateV2) valid() bool {
	return l.MaxCorrections >= 0 && l.MaxCorrections <= 3 && l.MaxCalls > 0 && l.MaxCalls <= 4096 && l.MaxRequestBytes > 0 && l.MaxRequestBytes <= 16<<20 && l.MaxResponseBytes > 0 && l.MaxResponseBytes <= 1<<20 && l.MaxTotalBytes > 0 && l.MaxTotalBytes <= 64<<20 && l.MaxOutputBytes > 0 && l.MaxOutputBytes <= 16<<20
}

// Exchange implementations must obey context and the response byte ceiling.
// Identity is caller asserted, not authenticated independence. Requests are data.
type AutomaticBackendPrivateV2 interface {
	Identity() string
	Exchange(context.Context, AutomaticRequestPrivateV2, int) ([]byte, error)
}
type AutomaticBackendErrorPrivateV2 struct{ Class string }

func (e AutomaticBackendErrorPrivateV2) Error() string { return e.Class }

type OwnBodyPrivateV2 struct{ Handle, Body string }
type ExactQuotePrivateV2 struct {
	Handle     string
	Start, End int
	Quote      string
}
type AutomaticClaimPrivateV2 struct {
	Text      string
	Citations []ExactQuotePrivateV2
}
type AutomaticInterpretationPrivateV2 struct {
	Disposition, Label, Description, Limitations, Alternatives, RejectionCondition string
	Claims                                                                         []AutomaticClaimPrivateV2
}
type AutomaticReviewPrivateV2 struct{ Digest, Verdict, Findings string }
type NavigationPrivateV2 struct {
	ID                             string
	Candidates                     []string
	Rationale, WithdrawalCondition string
}

// One typed protocol used by owned fixtures and the private local host handoff.
// Bodies are own-target AST, not FULL_DEFINITION, MethodResult or native custody.
type AutomaticRequestPrivateV2 struct {
	Protocol, Stage, PreparationSHA256, InputSHA256, Target, PredecessorDigest string
	Attempt                                                                    int
	Bodies                                                                     []OwnBodyPrivateV2
	Packet                                                                     *Packet
	Revision                                                                   *AutomaticInterpretationPrivateV2
	RevisionDigest                                                             string
	Candidates                                                                 []AutomaticCandidatePrivateV2
	Navigation                                                                 []NavigationPrivateV2
	Feedback                                                                   string
	PostLimits                                                                 PostPreparationLimits
	LedgerQuoteChunkBytes                                                      int
	PriorRevisions                                                             []AutomaticRevisionPrivateV2
	PriorNavigationRevisions                                                   []AutomaticNavigationRevisionPrivateV2
}
type AutomaticRevisionPrivateV2 struct {
	Digest, PredecessorDigest, Status, Findings string
	ResponseSHA256                              string
	ResponseByteLength                          int
	Response                                    json.RawMessage
	Interpretation                              *AutomaticInterpretationPrivateV2
	Review                                      *AutomaticReviewPrivateV2
}
type AutomaticRowPrivateV2 struct {
	InputSHA256, Target, Status, Reason, CandidateID string
	Revisions                                        []AutomaticRevisionPrivateV2
}
type AutomaticCandidatePrivateV2 struct {
	PrivateCeilings
	ID, InputSHA256, Target, RevisionDigest, Label, Description string
	Claims                                                      []AutomaticClaimPrivateV2
	Limitations, Alternatives, RejectionCondition               string
}
type AutomaticNavigationRevisionPrivateV2 struct {
	Digest, PredecessorDigest, Status, Findings string
	ResponseSHA256                              string
	ResponseByteLength                          int
	Response                                    json.RawMessage
	Hypotheses                                  []NavigationPrivateV2
	Review                                      *AutomaticReviewPrivateV2
}
type AutomaticInventoryPrivateV2 struct {
	Cohort *CohortEvidencePrivateV2 `json:",omitempty"`
	PrivateCeilings
	Protocol, Status, Failure, PreparationSHA256, Absence, Independence string
	Rows                                                                []AutomaticRowPrivateV2
	Candidates                                                          []AutomaticCandidatePrivateV2
	Navigation                                                          []NavigationPrivateV2
	NavigationRevisions                                                 []AutomaticNavigationRevisionPrivateV2
	Calls, Bytes                                                        int
}

// Opaque successful continuation. Fields are not importable from JSON; all
// public observations are detached copies. Failure observations confer nothing.
type AutomaticResultPrivateV2 struct {
	owner       *postState
	inventory   AutomaticInventoryPrivateV2
	submissions []CallerSubmission
	reviews     []CallerReview
	human       string
}

func (a AutomaticResultPrivateV2) Inventory() AutomaticInventoryPrivateV2 {
	return postClone(a.inventory)
}
func (a AutomaticResultPrivateV2) Human() string { return a.human }

// RetainAutomaticPrivateV2 reuses every existing V2 root, mode, alias, hash and
// no-replace gate. Both copies contain the final machine AND human inventory.
func RetainAutomaticPrivateV2(ctx context.Context, h PostPreparationPrivateV2, a AutomaticResultPrivateV2, outputs [2]*publication.ContentRetentionRoot, protected []*publication.ObservedRoot) (PostPublicationOutcomePrivateV2, error) {
	if a.owner == nil || a.owner != h.state || a.inventory.Status == "FAILED" {
		return PostPublicationOutcomePrivateV2{}, errors.New("unsealed or foreign automatic result")
	}
	h.automatic = &a
	return RetainPostPreparationPrivateV2(ctx, h, a.submissions, a.reviews, outputs, protected)
}
func (AutomaticResultPrivateV2) MarshalJSON() ([]byte, error) {
	return nil, errors.New("opaque automatic continuation")
}

func strictAutomatic(raw []byte, cap int, out any) error {
	if len(raw) == 0 || len(raw) > cap || !utf8.Valid(raw) {
		return AutomaticBackendErrorPrivateV2{"MALFORMED"}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return AutomaticBackendErrorPrivateV2{"MALFORMED"}
	}
	// Canonical equality rejects duplicate keys, casing aliases, omitted fields and
	// trailing objects. Backends must return the exact typed protocol, not prose.
	b, e := json.Marshal(out)
	if e != nil || !bytes.Equal(raw, b) {
		return AutomaticBackendErrorPrivateV2{"MALFORMED"}
	}
	return nil
}
func automaticDigest(v any) string { b, _ := json.Marshal(v); return pilot.Digest(b) }
func automaticClass(e error) string {
	if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
		return "CANCELLED"
	}
	var b AutomaticBackendErrorPrivateV2
	if errors.As(e, &b) {
		switch b.Class {
		case "UNAVAILABLE", "MALFORMED", "BUDGET_EXHAUSTED", "UNSUPPORTED", "REJECTED", "ABSTAINED", "CANCELLED":
			return b.Class
		}
	}
	return "UNAVAILABLE" // No external error/body/credential text is echoed.
}

type automaticBudget struct {
	limits       AutomaticLimitsPrivateV2
	calls, bytes int
	terminal     error
}

func (b *automaticBudget) call(ctx context.Context, backend AutomaticBackendPrivateV2, req AutomaticRequestPrivateV2) ([]byte, error) {
	if b.terminal != nil {
		return nil, b.terminal
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, e := json.Marshal(req)
	if e != nil {
		return nil, AutomaticBackendErrorPrivateV2{"MALFORMED"}
	}
	requestBytes := len(raw)
	if wire, ok := backend.(interface {
		wireSize(AutomaticRequestPrivateV2, int) (int, error)
	}); ok {
		requestBytes, e = wire.wireSize(req, b.limits.MaxResponseBytes)
		if e != nil {
			return nil, e
		}
	}
	if requestBytes < 0 {
		return nil, AutomaticBackendErrorPrivateV2{"MALFORMED"}
	}
	if b.calls >= b.limits.MaxCalls || requestBytes > b.limits.MaxRequestBytes || requestBytes > b.limits.MaxTotalBytes-b.bytes {
		return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
	}
	if backend == nil {
		return nil, AutomaticBackendErrorPrivateV2{"UNAVAILABLE"}
	}
	remaining := b.limits.MaxTotalBytes - b.bytes - requestBytes
	responseLimit := min(b.limits.MaxResponseBytes, remaining)
	if responseLimit < 1 {
		return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
	}
	b.calls++
	b.bytes += requestBytes
	response, e := backend.Exchange(ctx, postClone(req), responseLimit)
	var terminal interface{ TerminalHandoffFailure() }
	if errors.As(e, &terminal) {
		b.terminal = e
	}
	responseBytes := len(response)
	if wire, ok := backend.(interface{ wireResponseBytes() int }); ok {
		responseBytes = wire.wireResponseBytes()
	}
	if responseBytes < 0 {
		return nil, AutomaticBackendErrorPrivateV2{"MALFORMED"}
	}
	// Resource consumption is charged even when evidence is refused or reading
	// failed. Saturation excludes only a possible final transport lookahead byte
	// beyond the aggregate allowance, never an entire refused response prefix.
	b.bytes += min(responseBytes, remaining)
	if responseBytes > responseLimit {
		return nil, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
	}
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return bytes.Clone(response), nil
}
func validateExactAutomatic(v AutomaticInterpretationPrivateV2, own map[string]string, l PostPreparationLimits) string {
	for _, s := range []string{v.Label, v.Description, v.Limitations, v.Alternatives, v.RejectionCondition} {
		if len(s) > l.MaxStringBytes {
			return "MALFORMED"
		}
	}
	if v.Disposition == "UNSUPPORTED" && strings.TrimSpace(v.Limitations) != "" && len(v.Claims) == 0 {
		return "UNSUPPORTED"
	}
	if v.Disposition != "DESCRIBED" || len(v.Claims) == 0 || len(v.Claims) > l.MaxClaimsPerItem {
		return "MALFORMED"
	}
	for _, s := range []string{v.Label, v.Description, v.Limitations, v.Alternatives, v.RejectionCondition} {
		if strings.TrimSpace(s) == "" || len(s) > l.MaxStringBytes {
			return "MALFORMED"
		}
	}
	for _, c := range v.Claims {
		if strings.TrimSpace(c.Text) == "" || len(c.Text) > l.MaxStringBytes || len(c.Citations) == 0 || len(c.Citations) > l.MaxCitationsPerClaim {
			return "MALFORMED"
		}
		for _, q := range c.Citations {
			body, ok := own[q.Handle]
			if !ok || len(q.Handle) > l.MaxStringBytes || q.Start < 0 || q.End <= q.Start || q.End > len(body) || len(q.Quote) > l.MaxStringBytes || body[q.Start:q.End] != q.Quote {
				return "MALFORMED"
			}
		}
	}
	if _, e := automaticLegacyClaims(v, l); e != nil {
		return "MALFORMED"
	}
	return ""
}

// Deterministic, ordered, lossless representation adapter. UTF-8-safe chunks
// preserve the original claim and quote in Inventory; old ledger gates stay put.
func automaticLegacyClaims(v AutomaticInterpretationPrivateV2, l PostPreparationLimits) ([]TechnicalClaim, error) {
	var claims []TechnicalClaim
	for _, c := range v.Claims {
		claim := TechnicalClaim{Text: c.Text}
		for _, q := range c.Citations {
			for start := 0; start < len(q.Quote); {
				end := min(start+automaticLedgerQuoteChunkBytes, len(q.Quote))
				for end < len(q.Quote) && !utf8.RuneStart(q.Quote[end]) {
					end--
				}
				if end <= start || len(claim.Citations) >= l.MaxCitationsPerClaim {
					return nil, AutomaticBackendErrorPrivateV2{"MALFORMED"}
				}
				claim.Citations = append(claim.Citations, Citation{Handle: q.Handle, SourceQuote: q.Quote[start:end]})
				start = end
			}
		}
		claims = append(claims, claim)
	}
	return claims, nil
}
func automaticReview(raw []byte, cap int, sha string, textCap int) (AutomaticReviewPrivateV2, string) {
	var v AutomaticReviewPrivateV2
	if strictAutomatic(raw, cap, &v) != nil || v.Digest != sha || strings.TrimSpace(v.Findings) == "" || len(v.Findings) > textCap {
		return v, "MALFORMED"
	}
	switch v.Verdict {
	case "APPROVE":
		return v, ""
	case "REJECT":
		return v, "REJECTED"
	case "ABSTAIN":
		return v, "ABSTAINED"
	}
	return v, "MALFORMED"
}

// AutomatePrivateV2 never acquires sources. It reads only immutable own packet
// records from h, invokes bounded backends, and retains every attempted revision.
// No unavailable backend is replaced with a manual-pending or invented success.
func AutomatePrivateV2(ctx context.Context, h PostPreparationPrivateV2, l AutomaticLimitsPrivateV2, interpreter, reviewer AutomaticBackendPrivateV2) (a AutomaticResultPrivateV2, err error) {
	a.inventory = AutomaticInventoryPrivateV2{PrivateCeilings: privateCeilings(), Protocol: AutomaticProtocolPrivateV2, Status: "FAILED", Absence: "PROHIBITED", Independence: "CONFIGURED_DISTINCT_IDENTITIES_NOT_AUTHENTICATED"}
	if ctx == nil || h.state == nil || !l.valid() {
		a.inventory.Failure = "INVOCATION_INVALID"
		return a, errors.New(a.inventory.Failure)
	}
	s := h.state
	a.inventory.PreparationSHA256 = s.sha
	// Allocate the complete denominator before the first backend call.
	for _, ix := range s.index {
		a.inventory.Rows = append(a.inventory.Rows, AutomaticRowPrivateV2{InputSHA256: ix.InputSHA256, Target: ix.Target, Status: ix.Status, Reason: ix.Gap})
	}
	b := automaticBudget{limits: l}
	defer func() {
		if a.inventory.Status == "FAILED" {
			a.owner = nil
			a.inventory.Candidates = nil
			a.inventory.Navigation = nil
			for i := range a.inventory.Rows {
				if a.inventory.Rows[i].Status == "PREPARED" || a.inventory.Rows[i].Status == "PROVISIONAL" {
					a.inventory.Rows[i].Status = a.inventory.Failure
					a.inventory.Rows[i].Reason = a.inventory.Failure
					a.inventory.Rows[i].CandidateID = ""
				}
			}
		}
		a.inventory.Calls = b.calls
		a.inventory.Bytes = b.bytes
		a.human = renderAutomatic(a.inventory)
	}()
	if e := ctx.Err(); e != nil {
		a.inventory.Failure = "CANCELLED"
		return a, e
	}
	if interpreter == nil || reviewer == nil || interpreter.Identity() == "" || reviewer.Identity() == "" {
		a.inventory.Failure = "UNAVAILABLE"
		return a, AutomaticBackendErrorPrivateV2{"UNAVAILABLE"}
	}
	for _, backend := range []AutomaticBackendPrivateV2{interpreter, reviewer} {
		if bound, ok := backend.(interface{ automaticOwner() *postState }); ok && bound.automaticOwner() != s {
			a.inventory.Failure = "HANDOFF_FOREIGN_OWNER"
			return a, errors.New(a.inventory.Failure)
		}
	}
	if interpreter.Identity() == reviewer.Identity() {
		a.inventory.Failure = "INDEPENDENCE_REFUSED"
		return a, errors.New(a.inventory.Failure)
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
	unresolved := false
	for i := range a.inventory.Rows {
		row := &a.inventory.Rows[i]
		if row.Status != "PREPARED" {
			unresolved = true
			continue
		}
		key := postKey{row.InputSHA256, row.Target}
		pp := s.packets[key]
		packet := packets[key]
		own := map[string]string{}
		var bodies []OwnBodyPrivateV2
		// Manifest order is retained; no map iteration changes request attribution.
		seen := map[string]bool{}
		for _, m := range pp.manifest.Handles {
			e := pp.evidence[m.Handle]
			if e.kind == "RETAINED_SOURCE_AST_DECLARATION" && !seen[m.Handle] {
				seen[m.Handle] = true
				own[m.Handle] = e.body
				bodies = append(bodies, OwnBodyPrivateV2{m.Handle, e.body})
			}
		}
		if len(bodies) == 0 {
			row.Status = "UNSUPPORTED"
			row.Reason = "NO_OWN_AST_BODY"
			unresolved = true
			continue
		}
		feedback, previous := "", ""
		for attempt := 0; attempt <= l.MaxCorrections; attempt++ {
			req := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "INTERPRET", PreparationSHA256: s.sha, InputSHA256: key.input, Target: key.target, Attempt: attempt, PredecessorDigest: previous, Bodies: bodies, Packet: &packet, Feedback: feedback}
			req.PostLimits = s.limits
			req.LedgerQuoteChunkBytes = automaticLedgerQuoteChunkBytes
			req.PriorRevisions = postClone(row.Revisions)
			raw, e := b.call(ctx, interpreter, req)
			rev := AutomaticRevisionPrivateV2{PredecessorDigest: previous, Response: bytes.Clone(raw), ResponseSHA256: pilot.Digest(raw), ResponseByteLength: len(raw)}
			var v AutomaticInterpretationPrivateV2
			status := ""
			if e != nil {
				status = automaticClass(e)
			} else if strictAutomatic(raw, l.MaxResponseBytes, &v) != nil {
				status = "MALFORMED"
			} else {
				rev.Interpretation = &v
				status = validateExactAutomatic(v, own, s.limits)
			}
			// Even malformed and failed attempts have immutable predecessor-bound IDs.
			rev.Digest = automaticDigest(struct{ Previous, RawDigest string }{previous, rev.ResponseSHA256})
			if !json.Valid(rev.Response) {
				rev.Response = nil
			}
			if status == "" {
				req.Stage = "REVIEW_TECHNICAL"
				req.Revision = &v
				req.RevisionDigest = rev.Digest
				reviewRaw, re := b.call(ctx, reviewer, req)
				if re != nil {
					status = automaticClass(re)
				} else {
					review, st := automaticReview(reviewRaw, l.MaxResponseBytes, rev.Digest, s.limits.MaxStringBytes)
					status = st
					rev.Review = &review
					rev.Findings = review.Findings
				}
			}
			rev.Status = status
			if status == "" {
				rev.Status = "APPROVED"
			}
			row.Revisions = append(row.Revisions, postClone(rev))
			if b.terminal != nil {
				a.inventory.Failure = "HANDOFF_TERMINAL"
				return a, b.terminal
			}
			previous = rev.Digest
			feedback = status + ": " + rev.Findings
			if status == "" {
				sub := CallerSubmission{PrivateCeilings: privateCeilings(), PreparationSHA256: s.sha, InputSHA256: key.input, Target: key.target, PacketID: pp.manifest.PacketID, OrderedPartSHA256: slices.Clone(pp.manifest.OrderedPartSHA256), AuthorTag: interpreter.Identity(), Disposition: "DESCRIBED", WorkingLabel: v.Label, TechnicalDescription: v.Description, Limitations: v.Limitations, Alternatives: v.Alternatives, RejectionCondition: v.RejectionCondition}
				// Already checked before review; conversion is pure and deterministic.
				var conversionErr error
				sub.Claims, conversionErr = automaticLegacyClaims(v, s.limits)
				if conversionErr != nil {
					a.inventory.Status = "FAILED"
					a.inventory.Failure = "LEGACY_ADAPTATION_FAILED"
					return a, conversionErr
				}
				review := CallerReview{PrivateCeilings: privateCeilings(), PreparationSHA256: s.sha, InputSHA256: key.input, Target: key.target, SubmissionSHA256: automaticDigest(sub), ReviewerAuthorTag: reviewer.Identity(), Verdict: "SUPPORT_WITH_LIMITS", Findings: rev.Findings}
				a.submissions = append(a.submissions, sub)
				a.reviews = append(a.reviews, review)
				id := automaticDigest(struct{ InputSHA256, Target string }{key.input, key.target})
				a.inventory.Candidates = append(a.inventory.Candidates, AutomaticCandidatePrivateV2{PrivateCeilings: privateCeilings(), ID: id, InputSHA256: key.input, Target: key.target, RevisionDigest: rev.Digest, Label: v.Label, Description: v.Description, Claims: postClone(v.Claims), Limitations: v.Limitations, Alternatives: v.Alternatives, RejectionCondition: v.RejectionCondition})
				row.Status = "PROVISIONAL"
				row.CandidateID = id
				row.Reason = "CONFIGURED_REVIEW_NOT_SEMANTIC_TRUTH"
				break
			}
			row.Status = status
			row.Reason = status
			if status == "UNAVAILABLE" || status == "CANCELLED" || status == "BUDGET_EXHAUSTED" || status == "UNSUPPORTED" || status == "ABSTAINED" {
				break
			}
		}
		if row.Status != "PROVISIONAL" {
			unresolved = true
		}
	}
	// Navigation is a separate reversible interpretation, never identity merging.
	if len(a.inventory.Candidates) > 0 {
		previous, feedback := "", ""
		for attempt := 0; attempt <= l.MaxCorrections; attempt++ {
			req := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE", PreparationSHA256: s.sha, Candidates: postClone(a.inventory.Candidates), Attempt: attempt, PredecessorDigest: previous, Feedback: feedback}
			req.PostLimits = s.limits
			req.LedgerQuoteChunkBytes = automaticLedgerQuoteChunkBytes
			req.PriorNavigationRevisions = postClone(a.inventory.NavigationRevisions)
			raw, e := b.call(ctx, interpreter, req)
			nr := AutomaticNavigationRevisionPrivateV2{Response: bytes.Clone(raw), Digest: automaticDigest(struct{ Previous, RawDigest string }{previous, pilot.Digest(raw)}), PredecessorDigest: previous, ResponseSHA256: pilot.Digest(raw), ResponseByteLength: len(raw)}
			var groups []NavigationPrivateV2
			status := ""
			if e != nil {
				status = automaticClass(e)
			} else if strictAutomatic(raw, l.MaxResponseBytes, &groups) != nil {
				status = "MALFORMED"
			} else {
				status = validateNavigation(groups, a.inventory.Candidates)
			}
			if !json.Valid(nr.Response) {
				nr.Response = nil
			}
			nr.Hypotheses = postClone(groups)
			if status == "" {
				req.Stage = "REVIEW_NAVIGATION"
				req.Navigation = groups
				req.RevisionDigest = nr.Digest
				rr, re := b.call(ctx, reviewer, req)
				if re != nil {
					status = automaticClass(re)
				} else {
					v, st := automaticReview(rr, l.MaxResponseBytes, nr.Digest, s.limits.MaxStringBytes)
					nr.Review = &v
					nr.Findings = v.Findings
					status = st
				}
			}
			nr.Status = status
			if status == "" {
				nr.Status = "APPROVED"
				a.inventory.Navigation = postClone(groups)
			}
			a.inventory.NavigationRevisions = append(a.inventory.NavigationRevisions, nr)
			if b.terminal != nil {
				a.inventory.Failure = "HANDOFF_TERMINAL"
				return a, b.terminal
			}
			previous = nr.Digest
			feedback = status + ": " + nr.Findings
			if status == "" {
				break
			}
			if status == "UNAVAILABLE" || status == "BUDGET_EXHAUSTED" || status == "CANCELLED" || status == "ABSTAINED" || attempt == l.MaxCorrections {
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
	humanEncoded, _ := json.Marshal(renderAutomatic(a.inventory))
	if len(encoded) > l.MaxOutputBytes || len(humanEncoded) > l.MaxOutputBytes-len(encoded) {
		a.inventory.Status = "FAILED"
		a.inventory.Failure = "BUDGET_EXHAUSTED"
		return a, AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}
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
func validateNavigation(groups []NavigationPrivateV2, candidates []AutomaticCandidatePrivateV2) string {
	if groups == nil || len(groups) > len(candidates) {
		return "MALFORMED"
	}
	known := map[string]bool{}
	for _, c := range candidates {
		known[c.ID] = true
	}
	ids := map[string]bool{}
	for _, g := range groups {
		if g.ID == "" || ids[g.ID] || len(g.Candidates) == 0 || strings.TrimSpace(g.Rationale) == "" || strings.TrimSpace(g.WithdrawalCondition) == "" {
			return "MALFORMED"
		}
		ids[g.ID] = true
		seen := map[string]bool{}
		for _, id := range g.Candidates {
			if !known[id] || seen[id] {
				return "MALFORMED"
			}
			seen[id] = true
		}
	}
	return ""
}

// Withdrawal returns a detached revision; candidates and their IDs are unchanged.
func WithdrawNavigationPrivateV2(in AutomaticInventoryPrivateV2, id string) AutomaticInventoryPrivateV2 {
	out := postClone(in)
	out.Navigation = slices.DeleteFunc(out.Navigation, func(g NavigationPrivateV2) bool { return g.ID == id })
	return out
}
func renderAutomatic(in AutomaticInventoryPrivateV2) string {
	var b strings.Builder
	b.WriteString("# PROVISIONAL inventory\n\nFeature identity UNRESOLVED; accepted=false; authority=0; source UNKNOWN; absence PROHIBITED. AST is not FULL_DEFINITION or native custody. Reviews are assertions, not authenticated independence or semantic truth.\n\n")
	fmt.Fprintf(&b, "Status: %s\n\n", in.Status)
	for _, r := range in.Rows {
		fmt.Fprintf(&b, "- %s / %s: %s (%s)\n", r.InputSHA256, r.Target, r.Status, r.Reason)
		for j, rev := range r.Revisions {
			finding := rev.Findings
			if rev.Status == "MALFORMED" {
				finding = "Malformed interpretation/review response; see machine revision diagnostics"
			}
			fmt.Fprintf(&b, "  - Attempt %d: %s; revision %s; %s\n", j, rev.Status, rev.Digest, finding)
			if rev.Status == "UNSUPPORTED" && rev.Interpretation != nil {
				fmt.Fprintf(&b, "    Limitation: %s\n", rev.Interpretation.Limitations)
			}
		}
	}
	for _, c := range in.Candidates {
		fmt.Fprintf(&b, "\n## %s\n\nID: %s\n\n%s\n\nLimitations: %s\nAlternatives: %s\nRejection condition: %s\n", c.Label, c.ID, c.Description, c.Limitations, c.Alternatives, c.RejectionCondition)
		for _, claim := range c.Claims {
			fmt.Fprintf(&b, "\n- %s\n", claim.Text)
			for _, q := range claim.Citations {
				fmt.Fprintf(&b, "  - %s bytes [%d,%d): %q\n", q.Handle, q.Start, q.End, q.Quote)
			}
		}
	}
	b.WriteString("\n## Reversible navigation hypotheses\n")
	for j, rev := range in.NavigationRevisions {
		finding := rev.Findings
		if rev.Status == "MALFORMED" {
			finding = "Malformed navigation/review response; see machine revision diagnostics"
		}
		fmt.Fprintf(&b, "\n- Navigation attempt %d: %s; %s\n", j, rev.Status, finding)
	}
	for _, g := range in.Navigation {
		fmt.Fprintf(&b, "\n- %s: %s; withdraw when %s; candidates %v\n", g.ID, g.Rationale, g.WithdrawalCondition, g.Candidates)
	}
	return b.String()
}
