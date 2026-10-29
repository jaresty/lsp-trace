package privatedescribeprep

// This file implements private, read-only mechanical accounting. Caller text,
// including source comments, is data, never instructions or semantic authority.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// PostPreparationLimits are mandatory invocation ceilings, not public defaults.
// Conservative encoding allowances can reject an otherwise smaller JSON output.
type PostPreparationLimits struct {
	MaxConstituents, MaxLedgerTargets, MaxSubmissions, MaxReviews        int
	MaxClaimsPerItem, MaxCitationsPerClaim, MaxStringBytes, MaxItemBytes int
	MaxInputBytes, MaxOutputBytes, MaxEvidenceRecords, MaxWork           int
}

func (l PostPreparationLimits) valid() bool {
	for _, p := range [][2]int{{l.MaxConstituents, 64}, {l.MaxLedgerTargets, 4096}, {l.MaxSubmissions, 256}, {l.MaxReviews, 256}, {l.MaxClaimsPerItem, 16}, {l.MaxCitationsPerClaim, 32}, {l.MaxStringBytes, 64 << 10}, {l.MaxItemBytes, 1 << 20}, {l.MaxInputBytes, 16 << 20}, {l.MaxOutputBytes, 16 << 20}, {l.MaxEvidenceRecords, 262144}, {l.MaxWork, 10000000}} {
		if p[0] <= 0 || p[0] > p[1] {
			return false
		}
	}
	return true
}

// PrivateCeilings must be explicit on caller DTOs. No accepted status exists.
type PrivateCeilings struct {
	Authority                           int
	Accepted                            bool
	FeatureIdentity, SourceCompleteness string
	OrdinaryNative, Describe, Workflow  string
}

func privateCeilings() PrivateCeilings {
	return PrivateCeilings{FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", OrdinaryNative: "NOT_ATTEMPTED", Describe: "NOT_ATTEMPTED", Workflow: "NOT_ATTEMPTED"}
}
func (c PrivateCeilings) valid() bool { return c == privateCeilings() }

type Citation struct{ Handle, SourceQuote string }
type TechnicalClaim struct {
	Text      string
	Citations []Citation
}

// CallerSubmission is a prospective private DTO, not an ordinary request or
// transport, and not an importer for historical ad-hoc technical descriptions.
type CallerSubmission struct {
	PrivateCeilings
	PreparationSHA256, InputSHA256, Target, PacketID           string
	OrderedPartSHA256                                          []string
	AuthorTag, Disposition, WorkingLabel, TechnicalDescription string
	Claims                                                     []TechnicalClaim
	Limitations, Alternatives, RejectionCondition              string
}
type CallerReview struct {
	PrivateCeilings
	PreparationSHA256, InputSHA256, Target, SubmissionSHA256 string
	ReviewerAuthorTag, Verdict, Findings                     string
}
type EvidenceHandle struct{ Handle, Kind string }
type PacketManifest struct {
	InputSHA256, Target, PacketID string
	OrderedPartSHA256             []string
	Handles                       []EvidenceHandle
}
type LedgerTarget struct {
	PrivateCeilings
	TargetIndex
	SubmissionStatus, SubmissionSHA256, SubmissionReason string
	ReviewStatus, ReviewReason, Independence             string
	Submission                                           *CallerSubmission
	Review                                               *CallerReview
}
type StageCoverage struct {
	Coverage
	Pending, Submitted, Abstained, Rejected int
}
type TechnicalLead struct {
	PrivateCeilings
	ID, Status, Support, Independence string
	SubmissionSHA256                  string
	Submission                        CallerSubmission
	Review                            CallerReview
	// These exact own-packet rows retain missing-source/callsite limitations.
	MissingGaps []Record
}
type StageLedger struct {
	PrivateCeilings
	PreparationSHA256, GroupingPrecondition, NominationCensus string
	Coverage                                                  []StageCoverage
	Targets                                                   []LedgerTarget
	Leads                                                     []TechnicalLead
}

// PostPreparation has no JSON/Result constructor and no exported mutable fields.
// It is not spent execution authority or a replay handle. Copies share only
// immutable constructor-retained state; getters and outcomes are detached.
type PostPreparation struct{ state *postState }
type postKey struct{ input, target string }
type postEvidence struct{ kind, body string }
type postPacket struct {
	manifest PacketManifest
	evidence map[string]postEvidence
	gaps     []Record
}
type postState struct {
	limits    PostPreparationLimits
	prepared  []byte
	sha       string
	coverage  []Coverage
	index     []TargetIndex
	packets   map[postKey]postPacket
	manifests []PacketManifest
	records   int
}
type postBudget struct{ bytes, work int }

func (b *postBudget) use(n int) error {
	if n < 0 || n > b.bytes {
		return fmt.Errorf("private post-preparation byte budget")
	}
	b.bytes -= n
	return nil
}
func (b *postBudget) step(ctx context.Context, n int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n < 0 || n > b.work {
		return fmt.Errorf("private post-preparation work budget")
	}
	b.work -= n
	return nil
}
func (b *postBudget) text(s string, cap int) error {
	// Length BEFORE UTF-8 scan, arithmetic, allocation, copying or encoding.
	if len(s) > cap {
		return fmt.Errorf("private post-preparation string bound")
	}
	if err := b.use(len(s)*6 + 16); err != nil {
		return err
	}
	if !utf8.ValidString(s) {
		return fmt.Errorf("private post-preparation UTF8")
	}
	return nil
}
func postContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("nil private post-preparation context")
	}
	return ctx.Err()
}

func NewPostPreparation(ctx context.Context, inputs []Input, prepLimits Limits, postLimits PostPreparationLimits) (PostPreparation, error) {
	if err := postContext(ctx); err != nil {
		return PostPreparation{}, err
	}
	if !postLimits.valid() || len(inputs) == 0 || len(inputs) > postLimits.MaxConstituents {
		return PostPreparation{}, fmt.Errorf("private post-preparation invocation limits")
	}
	// Prepare owns sealed input/assessment validation and all selection preflight.
	r, err := Prepare(ctx, inputs, prepLimits)
	if err != nil {
		return PostPreparation{}, err
	}
	return postFromPrepared(ctx, r, postLimits)
}

// Only NewPostPreparation calls this helper in production. Unit fixtures using
// it test mechanics, NOT sealed producer qualification or DTO admission.
func postFromPrepared(ctx context.Context, r Result, l PostPreparationLimits) (PostPreparation, error) {
	return postFromPreparedProfile(ctx, r, l, RequestType)
}

// Version is supplied only by package-owned sealed constructors, never callers.
func postFromPreparedProfile(ctx context.Context, r Result, l PostPreparationLimits, expectedRequestType string) (PostPreparation, error) {
	stage := "shape"
	fail := func() (PostPreparation, error) {
		return PostPreparation{}, fmt.Errorf("invalid private prepared accounting: %s", stage)
	}
	if postContext(ctx) != nil || !l.valid() || !r.Limits.valid() || r.RequestType != expectedRequestType || r.Authority != 0 || r.Accepted || r.FeatureIdentity != "UNRESOLVED" || r.SourceCompleteness != "UNKNOWN" || len(r.Coverage) == 0 || len(r.Coverage) > l.MaxConstituents || len(r.Index) > l.MaxLedgerTargets || len(r.Packets) > 64 {
		return fail()
	}
	// Count ALL evidence before constructing any evidence or identity maps.
	b := postBudget{bytes: l.MaxInputBytes, work: l.MaxWork}
	records, parts, payload := 0, 0, 0
	for _, p := range r.Packets {
		if len(p.Parts) == 0 || len(p.Parts) > r.Limits.MaxParts || len(p.Parts) != len(p.PartSHA256) {
			return fail()
		}
		for _, part := range p.Parts {
			if err := b.step(ctx, 1); err != nil {
				return PostPreparation{}, err
			}
			if len(part.Records) > l.MaxEvidenceRecords-records {
				return fail()
			}
			records += len(part.Records)
			parts++
			for _, rec := range part.Records {
				if err := b.step(ctx, 1); err != nil {
					return PostPreparation{}, err
				}
				if len(rec.Value) > r.Limits.MaxPartBytes || len(rec.Handle) > 4096 || len(rec.Kind) > 128 {
					return fail()
				}
			}
		}
	}
	if parts > r.Limits.MaxParts {
		return fail()
	}
	// Result is locally produced and already finite; exact canonical bytes are
	// retained only after shape/count bounds. No exported DTO-to-state escape.
	raw := r.CanonicalBytes()
	if len(raw) > r.Limits.MaxTotalBytes || len(raw) > l.MaxOutputBytes || b.use(len(raw)) != nil {
		return fail()
	}
	s := &postState{limits: l, prepared: bytes.Clone(raw), sha: pilot.Digest(raw), records: records, packets: make(map[postKey]postPacket)}
	stage = "coverage"
	cv := make(map[string]int, len(r.Coverage))
	counts := make([][4]int, len(r.Coverage))
	for i, c := range r.Coverage {
		if err := b.step(ctx, 1); err != nil {
			return PostPreparation{}, err
		}
		if _, ok := cv[c.InputSHA256]; ok {
			return fail()
		}
		if len(c.InputSHA256) != 71 || c.Requested != len(c.Selection) || c.Requested != c.Prepared+c.SourceGap+c.ResourceGap {
			return fail()
		}
		cv[c.InputSHA256] = i
	}
	stage = "index"
	seen := make(map[postKey]bool, len(r.Index))
	last := make(map[string]int)
	for _, ix := range r.Index {
		if err := b.step(ctx, 1); err != nil {
			return PostPreparation{}, err
		}
		ci, ok := cv[ix.InputSHA256]
		if !ok || ix.OriginalOrdinal < 0 || ix.OriginalOrdinal >= r.Coverage[ci].OriginalTargets {
			return fail()
		}
		k := postKey{ix.InputSHA256, ix.Target}
		if err := b.step(ctx, len(r.Coverage[ci].Selection)); err != nil {
			return PostPreparation{}, err
		}
		if seen[k] || !slices.Contains(r.Coverage[ci].Selection, ix.Target) {
			return fail()
		}
		seen[k] = true
		if n, ok := last[ix.InputSHA256]; ok && ix.OriginalOrdinal <= n {
			return fail()
		}
		last[ix.InputSHA256] = ix.OriginalOrdinal
		counts[ci][0]++
		switch ix.Status {
		case "PREPARED":
			counts[ci][1]++
			if ix.PacketID == "" {
				return fail()
			}
		case "SOURCE_GAP":
			counts[ci][2]++
		case "RESOURCE_GAP":
			counts[ci][3]++
		default:
			return fail()
		}
	}
	for i, c := range r.Coverage {
		if counts[i] != [4]int{c.Requested, c.Prepared, c.SourceGap, c.ResourceGap} {
			return fail()
		}
	}
	stage = "packets"
	for _, p := range r.Packets {
		k := postKey{p.InputSHA256, p.Target}
		if _, ok := s.packets[k]; ok || !seen[k] {
			return fail()
		}
		owned := false
		if err := b.step(ctx, len(r.Index)); err != nil {
			return PostPreparation{}, err
		}
		for _, ix := range r.Index {
			if ix.InputSHA256 == k.input && ix.Target == k.target && ix.Status == "PREPARED" && ix.PacketID == p.ID && ix.OriginalOrdinal == p.OriginalOrdinal {
				owned = true
			}
		}
		if !owned {
			return fail()
		}
		pp := postPacket{manifest: PacketManifest{InputSHA256: p.InputSHA256, Target: p.Target, PacketID: p.ID, OrderedPartSHA256: slices.Clone(p.PartSHA256)}, evidence: make(map[string]postEvidence)}
		for i, part := range p.Parts {
			if err := b.step(ctx, 1); err != nil {
				return PostPreparation{}, err
			}
			prev, next := 0, 0
			if i > 0 {
				prev = i
			}
			if i+1 < len(p.Parts) {
				next = i + 2
			}
			if part.Index != i+1 || part.Previous != prev || part.Next != next || part.RequestType != expectedRequestType || part.PacketID != p.ID || part.InputSHA256 != k.input || part.Target != k.target || part.ScopeSHA256 != p.ScopeSHA256 {
				return fail()
			}
			raw, err := json.Marshal(part)
			if err != nil || len(raw) > r.Limits.MaxPartBytes || pilot.Digest(raw) != p.PartSHA256[i] {
				return fail()
			}
			payload += len(raw)
			for _, rec := range part.Records {
				if err := b.step(ctx, 1); err != nil {
					return PostPreparation{}, err
				}
				ev := postEvidence{kind: rec.Kind}
				if rec.Kind == "RETAINED_SOURCE_AST_DECLARATION" {
					var d Declaration
					if json.Unmarshal(rec.Value, &d) != nil {
						return fail()
					}
					ev.body = d.Body
				}
				if prior, ok := pp.evidence[rec.Handle]; ok && prior != ev {
					// Prepare may use the same handle for an endpoint's START and
					// FRONTIER rows. Preserve both manifest rows; the handle still
					// denotes only own-packet evidence, but cannot support a quote.
					ev = postEvidence{kind: "MULTIPLE_OWN_RECORDS"}
				}
				pp.evidence[rec.Handle] = ev
				pp.manifest.Handles = append(pp.manifest.Handles, EvidenceHandle{rec.Handle, rec.Kind})
				if rec.Kind == "GAP" || rec.Kind == "SOURCE" || rec.Kind == "CALL_ENDPOINT_SOURCE_AVAILABILITY" {
					pp.gaps = append(pp.gaps, Record{rec.Kind, rec.Handle, bytes.Clone(rec.Value)})
				}
			}
		}
		s.packets[k] = pp
		s.manifests = append(s.manifests, pp.manifest)
	}
	if payload != r.PartBytes || parts != r.PartCount {
		return fail()
	}
	for _, ix := range r.Index {
		if ix.Status == "PREPARED" {
			if _, ok := s.packets[postKey{ix.InputSHA256, ix.Target}]; !ok {
				return fail()
			}
		}
	}
	s.coverage = postClone(r.Coverage)
	s.index = slices.Clone(r.Index)
	return PostPreparation{s}, nil
}

func postClone[T any](v T) T {
	b, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}
func (h PostPreparation) PreparedBytes() ([]byte, error) {
	if h.state == nil {
		return nil, fmt.Errorf("unsealed private post-preparation handle")
	}
	return bytes.Clone(h.state.prepared), nil
}
func (h PostPreparation) PreparationSHA256() (string, error) {
	if h.state == nil {
		return "", fmt.Errorf("unsealed private post-preparation handle")
	}
	return h.state.sha, nil
}
func (h PostPreparation) Manifest() ([]PacketManifest, error) {
	if h.state == nil {
		return nil, fmt.Errorf("unsealed private post-preparation handle")
	}
	return postClone(h.state.manifests), nil
}

// preflightCall traverses caller-owned shapes BEFORE any map, clone or marshal.
// Estimates include six-byte JSON escaping plus scalar/container overhead.
func (s *postState) preflightCall(ctx context.Context, subs []CallerSubmission, reviews []CallerReview) (int, int, error) {
	l := s.limits
	b := postBudget{bytes: l.MaxInputBytes, work: l.MaxWork}
	if len(subs) > l.MaxSubmissions || len(reviews) > l.MaxReviews || s.records > l.MaxEvidenceRecords {
		return 0, 0, fmt.Errorf("private caller count bound")
	}
	if err := b.use(len(s.prepared)); err != nil {
		return 0, 0, err
	}
	start := b.bytes
	stringsCheck := func(ss ...string) error {
		for _, v := range ss {
			if err := b.text(v, l.MaxStringBytes); err != nil {
				return err
			}
		}
		return nil
	}
	ceilings := func(c PrivateCeilings) error {
		return stringsCheck(c.FeatureIdentity, c.SourceCompleteness, c.OrdinaryNative, c.Describe, c.Workflow)
	}
	for _, v := range subs {
		if err := b.step(ctx, 1); err != nil {
			return 0, 0, err
		}
		if len(v.OrderedPartSHA256) > 256 || len(v.Claims) > l.MaxClaimsPerItem {
			return 0, 0, fmt.Errorf("private submission shape bound")
		}
		before := b.bytes
		if err := b.use(2048); err != nil {
			return 0, 0, err
		}
		if err := ceilings(v.PrivateCeilings); err != nil {
			return 0, 0, err
		}
		if err := stringsCheck(v.PreparationSHA256, v.InputSHA256, v.Target, v.PacketID, v.AuthorTag, v.Disposition, v.WorkingLabel, v.TechnicalDescription, v.Limitations, v.Alternatives, v.RejectionCondition); err != nil {
			return 0, 0, err
		}
		for _, p := range v.OrderedPartSHA256 {
			if err := b.step(ctx, 1); err != nil {
				return 0, 0, err
			}
			if err := stringsCheck(p); err != nil {
				return 0, 0, err
			}
		}
		for _, c := range v.Claims {
			if err := b.step(ctx, 1); err != nil {
				return 0, 0, err
			}
			if len(c.Citations) > l.MaxCitationsPerClaim {
				return 0, 0, fmt.Errorf("private citation count bound")
			}
			if err := b.use(128); err != nil {
				return 0, 0, err
			}
			if err := stringsCheck(c.Text); err != nil {
				return 0, 0, err
			}
			for _, ref := range c.Citations {
				if err := b.step(ctx, 1); err != nil {
					return 0, 0, err
				}
				if err := b.text(ref.SourceQuote, 240); err != nil {
					return 0, 0, err
				}
				if err := stringsCheck(ref.Handle); err != nil {
					return 0, 0, err
				}
				if err := b.use(128); err != nil {
					return 0, 0, err
				}
			}
		}
		if before-b.bytes > l.MaxItemBytes {
			return 0, 0, fmt.Errorf("private submission item byte bound")
		}
	}
	for _, v := range reviews {
		if err := b.step(ctx, 1); err != nil {
			return 0, 0, err
		}
		before := b.bytes
		if err := b.use(1024); err != nil {
			return 0, 0, err
		}
		if err := ceilings(v.PrivateCeilings); err != nil {
			return 0, 0, err
		}
		if err := stringsCheck(v.PreparationSHA256, v.InputSHA256, v.Target, v.SubmissionSHA256, v.ReviewerAuthorTag, v.Verdict, v.Findings); err != nil {
			return 0, 0, err
		}
		if before-b.bytes > l.MaxItemBytes {
			return 0, 0, fmt.Errorf("private review item byte bound")
		}
	}
	// Reserve the COMPLETE index and coverage, all packet gap rows potentially
	// inherited by leads, and both DTO copies in outcomes and provisional leads.
	// Subtraction-based reservation prevents overflow and precedes output marshal.
	out := postBudget{bytes: l.MaxOutputBytes}
	for _, n := range []int{4096, len(s.prepared) * 2, (start - b.bytes) * 2, len(s.index) * 2048, len(s.coverage) * 1024} {
		if err := out.use(n); err != nil {
			return 0, 0, fmt.Errorf("private complete ledger output estimate: %w", err)
		}
	}
	return l.MaxOutputBytes - out.bytes, b.work, nil
}

func (s *postState) validateSubmission(v CallerSubmission, p postPacket) string {
	if !v.PrivateCeilings.valid() {
		return "CEILINGS_NOT_ZERO_AUTHORITY"
	}
	if v.PreparationSHA256 != s.sha || v.PacketID != p.manifest.PacketID || !slices.Equal(v.OrderedPartSHA256, p.manifest.OrderedPartSHA256) {
		return "PREPARATION_OR_PACKET_MANIFEST_MISMATCH"
	}
	if strings.TrimSpace(v.AuthorTag) == "" {
		return "MISSING_AUTHOR_TAG"
	}
	switch v.Disposition {
	case "ABSTAINED":
		if strings.TrimSpace(v.Limitations) == "" || len(v.Claims) != 0 {
			return "ABSTENTION_REQUIRES_LIMITATIONS_NO_CLAIMS"
		}
	case "DESCRIBED":
		for _, text := range []string{v.WorkingLabel, v.TechnicalDescription, v.Alternatives, v.RejectionCondition} {
			if strings.TrimSpace(text) == "" {
				return "MISSING_DESCRIPTION_ALTERNATIVE_OR_FALSIFIER"
			}
		}
		if len(v.Claims) == 0 {
			return "MISSING_CLAIMS"
		}
		for _, c := range v.Claims {
			if strings.TrimSpace(c.Text) == "" || len(c.Citations) == 0 {
				return "CLAIM_REQUIRES_TEXT_AND_CITATIONS"
			}
			for _, ref := range c.Citations {
				ev, ok := p.evidence[ref.Handle]
				if !ok {
					return "FOREIGN_OR_UNKNOWN_HANDLE"
				}
				if ref.SourceQuote != "" && (ev.kind != "RETAINED_SOURCE_AST_DECLARATION" || !strings.Contains(ev.body, ref.SourceQuote)) {
					return "QUOTE_NOT_OWN_LITERAL_DECLARATION_SUBSTRING"
				}
			}
		}
	default:
		return "UNKNOWN_DISPOSITION"
	}
	return ""
}

// Reconcile is pure and deterministic; an earlier result never changes this
// state. Reviews pin a canonical submission digest from an earlier invocation.
// Unknown identities and ambiguous duplicate keys reject the WHOLE invocation.
func (h PostPreparation) Reconcile(ctx context.Context, submissions []CallerSubmission, reviews []CallerReview) (StageLedger, error) {
	return reconcilePostState(ctx, h.state, submissions, reviews)
}

func reconcilePostState(ctx context.Context, s *postState, submissions []CallerSubmission, reviews []CallerReview) (StageLedger, error) {
	if err := postContext(ctx); err != nil {
		return StageLedger{}, err
	}
	if s == nil {
		return StageLedger{}, fmt.Errorf("unsealed private post-preparation handle")
	}
	_, work, err := s.preflightCall(ctx, submissions, reviews)
	if err != nil {
		return StageLedger{}, err
	}
	b := postBudget{work: work}
	positions := make(map[postKey]int, len(s.index))
	for i, ix := range s.index {
		if err := b.step(ctx, 1); err != nil {
			return StageLedger{}, err
		}
		positions[postKey{ix.InputSHA256, ix.Target}] = i
	}
	subAt := make(map[postKey]int, len(submissions))
	reviewAt := make(map[postKey]int, len(reviews))
	for i, v := range submissions {
		if err := b.step(ctx, 1); err != nil {
			return StageLedger{}, err
		}
		k := postKey{v.InputSHA256, v.Target}
		if _, ok := positions[k]; !ok {
			return StageLedger{}, fmt.Errorf("foreign or unknown submission identity")
		}
		if _, ok := s.packets[k]; !ok {
			return StageLedger{}, fmt.Errorf("submission targets unprepared identity")
		}
		if _, ok := subAt[k]; ok {
			return StageLedger{}, fmt.Errorf("duplicate submission identity")
		}
		subAt[k] = i
	}
	for i, v := range reviews {
		if err := b.step(ctx, 1); err != nil {
			return StageLedger{}, err
		}
		k := postKey{v.InputSHA256, v.Target}
		if _, ok := positions[k]; !ok {
			return StageLedger{}, fmt.Errorf("foreign or unknown review identity")
		}
		if _, ok := s.packets[k]; !ok {
			return StageLedger{}, fmt.Errorf("review targets unprepared identity")
		}
		if _, ok := reviewAt[k]; ok {
			return StageLedger{}, fmt.Errorf("duplicate review identity")
		}
		reviewAt[k] = i
	}
	out := StageLedger{PrivateCeilings: privateCeilings(), PreparationSHA256: s.sha, GroupingPrecondition: "UNRESOLVED_NOT_CHECKED", NominationCensus: "NOT_ADMITTED"}
	for _, c := range s.coverage {
		out.Coverage = append(out.Coverage, StageCoverage{Coverage: postClone(c)})
	}
	for _, ix := range s.index {
		if err := b.step(ctx, 1); err != nil {
			return StageLedger{}, err
		}
		row := LedgerTarget{PrivateCeilings: privateCeilings(), TargetIndex: ix, ReviewStatus: "PENDING"}
		k := postKey{ix.InputSHA256, ix.Target}
		if ix.Status == "PREPARED" {
			row.SubmissionStatus = "PENDING"
			if n, ok := subAt[k]; ok {
				v := submissions[n]
				// Charge source substring searches by their full bounded haystack
				// before executing them; no uncharged repeated source scan.
				for _, claim := range v.Claims {
					for _, ref := range claim.Citations {
						if ref.SourceQuote != "" {
							if err := b.step(ctx, len(s.packets[k].evidence[ref.Handle].body)); err != nil {
								return StageLedger{}, err
							}
						}
					}
				}
				row.SubmissionReason = s.validateSubmission(v, s.packets[k])
				if row.SubmissionReason == "" {
					row.Submission = ptr(postClone(v))
				}
				raw, _ := json.Marshal(v)
				row.SubmissionSHA256 = pilot.Digest(raw)
				row.SubmissionStatus = v.Disposition
				if v.Disposition == "DESCRIBED" {
					row.SubmissionStatus = "SUBMITTED"
				}
				if row.SubmissionReason != "" {
					row.SubmissionStatus = "REJECTED"
				}
			}
			if n, ok := reviewAt[k]; ok {
				v := reviews[n]
				row.ReviewStatus = "REJECTED"
				switch {
				case row.Submission == nil || row.SubmissionStatus == "REJECTED":
					row.ReviewReason = "NO_VALID_SUBMISSION"
				case !v.PrivateCeilings.valid():
					row.ReviewReason = "CEILINGS_NOT_ZERO_AUTHORITY"
				case v.PreparationSHA256 != s.sha || v.SubmissionSHA256 != row.SubmissionSHA256:
					row.ReviewReason = "STALE_OR_MISMATCHED_REVIEW_PIN"
				case strings.TrimSpace(v.ReviewerAuthorTag) == "" || v.ReviewerAuthorTag == row.Submission.AuthorTag:
					row.ReviewReason = "MISSING_OR_SAME_AUTHOR_TAG"
				case strings.TrimSpace(v.Findings) == "":
					row.ReviewReason = "MISSING_REVIEW_FINDINGS"
				default:
					switch v.Verdict {
					case "SUPPORT_WITH_LIMITS":
						row.ReviewStatus = "RECORDED_SUPPORT_WITH_LIMITS"
					case "CHANGES_REQUIRED":
						row.ReviewStatus = "CHANGES_REQUIRED"
					case "ABSTAIN":
						row.ReviewStatus = "ABSTAIN"
					default:
						row.ReviewReason = "UNKNOWN_REVIEW_VERDICT"
					}
					if row.ReviewReason == "" {
						row.Review = ptr(postClone(v))
						row.Independence = "CALLER_ASSERTED_INDEPENDENCE_NOT_AUTHENTICATED"
					}
				}
			}
			if row.SubmissionStatus == "SUBMITTED" && row.ReviewStatus == "RECORDED_SUPPORT_WITH_LIMITS" {
				idBytes, _ := json.Marshal(struct{ InputSHA256, Target string }{k.input, k.target})
				out.Leads = append(out.Leads, TechnicalLead{PrivateCeilings: privateCeilings(), ID: pilot.Digest(idBytes), Status: "PROVISIONAL", Support: "CALLER_REVIEW_RECORDED_NOT_VERIFIED_SEMANTIC_SUPPORT", Independence: row.Independence, SubmissionSHA256: row.SubmissionSHA256, Submission: postClone(*row.Submission), Review: *row.Review, MissingGaps: postClone(s.packets[k].gaps)})
			}
			for i := range out.Coverage {
				if out.Coverage[i].InputSHA256 != ix.InputSHA256 {
					continue
				}
				switch row.SubmissionStatus {
				case "PENDING":
					out.Coverage[i].Pending++
				case "SUBMITTED":
					out.Coverage[i].Submitted++
				case "ABSTAINED":
					out.Coverage[i].Abstained++
				case "REJECTED":
					out.Coverage[i].Rejected++
				}
			}
		}
		out.Targets = append(out.Targets, row)
	}
	for _, c := range out.Coverage {
		if c.Requested != c.Prepared+c.SourceGap+c.ResourceGap || c.Prepared != c.Pending+c.Submitted+c.Abstained+c.Rejected {
			return StageLedger{}, fmt.Errorf("private terminal accounting failure")
		}
	}
	if err := ctx.Err(); err != nil {
		return StageLedger{}, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return StageLedger{}, err
	}
	if len(raw) > s.limits.MaxOutputBytes {
		return StageLedger{}, fmt.Errorf("private ledger output bytes")
	}
	return out, nil
}
func ptr[T any](v T) *T { return &v }
