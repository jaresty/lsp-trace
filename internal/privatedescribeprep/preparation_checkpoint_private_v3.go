//go:build darwin || linux

package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"lsp-trace/internal/acquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

const RequestTypeRetainedCheckpointPrivateV3 = "PRIVATE_REOPENED_RETAINED_OBSERVATIONAL_OWN_AST_V3_1"

type OwnASTReportRetainedCheckpointPrivateV3 struct {
	RequestType                                                      string
	Source                                                           derived.SourceBindingRetainedCheckpointPrivateV3
	AssessmentSHA256                                                 string
	Assessment                                                       json.RawMessage
	Limits                                                           Limits
	FullTargetCount, FullBatchCount, BatchTargetCount, PreparedCount int
	SourceInputID                                                    string
	Bodies                                                           []OwnASTBodyPrivateV3
	Authority                                                        int
	Accepted                                                         bool
	FeatureIdentity, SourceCompleteness, AbsenceInference            string
}
type PreparedRetainedCheckpointPrivateV3 struct {
	state *preparedCheckpointStatePrivateV3
}
type preparedCheckpointStatePrivateV3 struct {
	self                       *preparedCheckpointStatePrivateV3
	projection                 derived.ProjectionRetainedCheckpointPrivateV3
	raw, canonical, assessment []byte
}
type OwnASTReaderRetainedCheckpointPrivateV3 struct {
	state *ownASTCheckpointStatePrivateV3
}
type ownASTCheckpointStatePrivateV3 struct {
	self                    *ownASTCheckpointStatePrivateV3
	mu                      sync.Mutex
	projection              derived.ProjectionRetainedCheckpointPrivateV3
	canonical               []byte
	maxBytes, reservedBytes int
	targets                 []acquisition.TargetResult
	report                  OwnASTReportRetainedCheckpointPrivateV3
	generation              uint64
	complete, failed        bool
}

func (PreparedRetainedCheckpointPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, pilot.ErrInvalid
}
func (OwnASTReaderRetainedCheckpointPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, pilot.ErrInvalid
}
func NewOwnASTReaderRetainedCheckpointPrivateV3(ctx context.Context, p derived.ProjectionRetainedCheckpointPrivateV3, a derived.AssessmentRetainedCheckpointPrivateV3, limits Limits, exchange pilot.PageCostPrivateV3) (OwnASTReaderRetainedCheckpointPrivateV3, error) {
	if ctx == nil || !limits.valid() {
		return OwnASTReaderRetainedCheckpointPrivateV3{}, pilot.ErrInvalid
	}
	before := p.BudgetSpent()
	_, assessment, e := derived.ValidateInstabilityRetainedCheckpointPrivateV3(ctx, p, a, exchange)
	if e != nil {
		return OwnASTReaderRetainedCheckpointPrivateV3{}, e
	}
	exchange, e = pilot.RemainingPageExchangePrivateV3(before, p.BudgetSpent(), exchange)
	if e != nil {
		return OwnASTReaderRetainedCheckpointPrivateV3{}, e
	}
	v, e := p.Read(ctx, exchange)
	if e != nil {
		return OwnASTReaderRetainedCheckpointPrivateV3{}, e
	}
	r := OwnASTReportRetainedCheckpointPrivateV3{RequestType: RequestTypeRetainedCheckpointPrivateV3, Source: v.Source, AssessmentSHA256: pilot.Digest(assessment), Assessment: assessment, Limits: limits, FullTargetCount: v.Source.FullTargetCount, FullBatchCount: v.Source.FullBatchCount, BatchTargetCount: v.Source.BatchTargetCount, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	targets := v.Selected.Source.Evidence.Acquisition.Targets
	if len(targets) > limits.MaxTargets {
		return OwnASTReaderRetainedCheckpointPrivateV3{}, pilot.ErrPagesUnitPrivateV3
	}
	maxBytes := limits.MaxPartBytes
	if limits.MaxTotalBytes < maxBytes {
		maxBytes = limits.MaxTotalBytes
	}
	if unit := int(v.Selected.Revision.Policy.ComputationUnitBytes); unit < maxBytes {
		maxBytes = unit
	}
	// A fixed-width placeholder admits the complete source graph BEFORE the
	// source-input digest serializes it. The real digest has identical width.
	r.SourceInputID = pilot.Digest(nil)
	if e := admitOwnASTCheckpointHeaderPrivateV3(ctx, r, maxBytes); e != nil {
		return OwnASTReaderRetainedCheckpointPrivateV3{}, e
	}
	r.SourceInputID = digest(struct {
		Request    string
		Source     derived.SourceBindingRetainedCheckpointPrivateV3
		Assessment string
	}{RequestTypeRetainedCheckpointPrivateV3, v.Source, r.AssessmentSHA256})
	header, e := marshalOwnASTCheckpointHeaderPrivateV3(ctx, r, maxBytes, json.Marshal)
	if e != nil {
		return OwnASTReaderRetainedCheckpointPrivateV3{}, e
	}
	s := &ownASTCheckpointStatePrivateV3{projection: p, canonical: v.DerivedCanonicalV5, targets: targets, report: r, maxBytes: maxBytes, reservedBytes: len(header) + len(assessment) + 256}
	s.self = s
	return OwnASTReaderRetainedCheckpointPrivateV3{s}, nil
}

// Detached serialization helper: no ownership is manufactured here.
func marshalOwnASTCheckpointHeaderPrivateV3(ctx context.Context, r OwnASTReportRetainedCheckpointPrivateV3, maxBytes int, marshal func(any) ([]byte, error)) ([]byte, error) {
	if e := admitOwnASTCheckpointHeaderPrivateV3(ctx, r, maxBytes); e != nil {
		return nil, e
	}
	metadata := r
	metadata.Assessment = nil
	header, e := marshal(metadata)
	if e != nil {
		return nil, e
	}
	if len(header) > maxBytes-256 || len(r.Assessment) > maxBytes-256-len(header) {
		return nil, pilot.ErrPagesUnitPrivateV3
	}
	return header, ctx.Err()
}

// Concrete header field graph only: no reflection, encoder buffer, or owner
// reconstruction. 8192 covers all fixed key/punctuation/null/bool fields and
// worst-width integers (including Limits) of these concrete structs; each string
// byte receives six bytes for JSON control/HTML/invalid-UTF8/U+2028 escaping.
// Bodies must still be empty. Assessment is already canonical json.Marshal output
// from the separately admitted assessment family and is not encoded into header.
// Subtractive checks prevent integer overflow before any full serialization.
func admitOwnASTCheckpointHeaderPrivateV3(ctx context.Context, r OwnASTReportRetainedCheckpointPrivateV3, maxBytes int) error {
	if ctx == nil {
		return pilot.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if len(r.Bodies) != 0 {
		return pilot.ErrInvalid
	}
	if maxBytes < 8192+256 || len(r.Assessment) > maxBytes-8192-256 {
		return pilot.ErrPagesUnitPrivateV3
	}
	left := maxBytes - 8192 - 256 - len(r.Assessment)
	s := r.Source.SourceBindingPrivateV3
	strings := []string{r.RequestType, r.AssessmentSHA256, r.SourceInputID, r.FeatureIdentity, r.SourceCompleteness, r.AbsenceInference,
		s.Admission, s.Profile, s.ProfileSHA256, s.ClaimCeiling, s.PageBindingSHA256, s.PageRevisionSHA256, s.SourceScopeSHA256, s.EnumerationOriginSHA256,
		s.SourcePolicySHA256, s.RuntimeMetadataSHA256, s.OriginalNativeV3SHA256, s.DerivedCanonicalV5SHA256, s.SessionID, s.WorkspaceURI, s.PositionEncoding, s.ProviderCommand,
		s.FeatureIdentity, s.SourceCompleteness, s.AbsenceInference, r.Source.CheckpointSHA256, r.Source.InputFamily}
	for _, v := range s.SelectedObjectSHA256 {
		if len(v) > left/6 {
			return pilot.ErrPagesUnitPrivateV3
		}
		left -= 6 * len(v)
	}
	for _, v := range strings {
		if e := ctx.Err(); e != nil {
			return e
		}
		if len(v) > left/6 {
			return pilot.ErrPagesUnitPrivateV3
		}
		left -= 6 * len(v)
	}
	return ctx.Err()
}

func (c OwnASTReaderRetainedCheckpointPrivateV3) Advance(ctx context.Context, generation uint64, exchange pilot.PageCostPrivateV3) (OwnASTProgressPrivateV3, PreparedRetainedCheckpointPrivateV3, error) {
	if c.state == nil || c.state.self != c.state {
		return OwnASTProgressPrivateV3{}, PreparedRetainedCheckpointPrivateV3{}, pilot.ErrInvalid
	}
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	progress := func(status string) OwnASTProgressPrivateV3 {
		return OwnASTProgressPrivateV3{status, len(s.report.Bodies), len(s.targets), s.projection.BudgetSpent()}
	}
	if ctx == nil || s.complete || s.failed || generation != s.generation+1 {
		return progress("INVALID"), PreparedRetainedCheckpointPrivateV3{}, pilot.ErrInvalid
	}
	s.generation = generation
	fail := func(e error) (OwnASTProgressPrivateV3, PreparedRetainedCheckpointPrivateV3, error) {
		status := "FAILED"
		if errors.Is(e, pilot.ErrPagesPausedPrivateV3) {
			status = "PAUSED_BUDGET"
		} else if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
			status = "CANCELLED"
		} else {
			s.failed = true
		}
		return progress(status), PreparedRetainedCheckpointPrivateV3{}, e
	}
	before := s.projection.BudgetSpent()
	if _, e := s.projection.Read(ctx, exchange); e != nil {
		return fail(e)
	}
	var e error
	exchange, e = pilot.RemainingPageExchangePrivateV3(before, s.projection.BudgetSpent(), exchange)
	if e != nil {
		return fail(e)
	}
	for len(s.report.Bodies) < len(s.targets) {
		if e = ctx.Err(); e != nil {
			return fail(e)
		}
		t := s.targets[len(s.report.Bodies)]
		n := t.Resolution.Identity
		if n == nil {
			return fail(fmt.Errorf("retained unresolved target"))
		}
		before = s.projection.BudgetSpent()
		body, err := s.projection.OwnSource(ctx, n.URI, exchange)
		if err != nil {
			return fail(err)
		}
		exchange, err = pilot.RemainingPageExchangePrivateV3(before, s.projection.BudgetSpent(), exchange)
		if err != nil {
			return fail(err)
		}
		if len(body) > s.report.Limits.MaxSourceBytes {
			return fail(pilot.ErrPagesUnitPrivateV3)
		}
		// Pure declaration parsing follows both retained-byte and FRESH source admission.
		// No ambient path, retained DTO body fallback, or FULL_DEFINITION promotion.
		d, gap := declaration(body, *n, s.report.Source.PositionEncoding)
		if gap != "" {
			return fail(fmt.Errorf("retained own AST gap: %s", gap))
		}
		b := OwnASTBodyPrivateV3{Target: t.Requested.ID, NodeID: n.ID, SourceURI: n.URI, SourceBodySHA256: pilot.Digest(body), OwnBodySHA256: pilot.Digest(d.Bytes), Declaration: d}
		bound := 4096 + 6*(len(b.Target)+len(b.NodeID)+len(b.SourceURI)+len(b.SourceBodySHA256)+len(b.OwnBodySHA256)+len(d.Kind)+len(d.Encoding)+len(d.Comments)+len(d.Body)) + 4*((len(d.Bytes)+2)/3)
		if bound > s.maxBytes-s.reservedBytes {
			return fail(pilot.ErrPagesUnitPrivateV3)
		}
		s.reservedBytes += bound
		s.report.Bodies = append(s.report.Bodies, b)
		s.report.PreparedCount++
	}
	raw, e := json.Marshal(s.report)
	if e != nil {
		return fail(e)
	}
	if len(raw) > s.maxBytes || len(raw) > s.reservedBytes {
		return fail(pilot.ErrPagesUnitPrivateV3)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	st := &preparedCheckpointStatePrivateV3{projection: s.projection, raw: raw, canonical: s.canonical, assessment: s.report.Assessment}
	st.self = st
	s.complete = true
	return progress("COMPLETE_REOPENED_OBSERVATIONAL_OWN_AST_PRIVATE_V3"), PreparedRetainedCheckpointPrivateV3{st}, nil
}
func PrivateOwnASTPrepareRetainedCheckpointV3(ctx context.Context, p derived.ProjectionRetainedCheckpointPrivateV3, a derived.AssessmentRetainedCheckpointPrivateV3, limits Limits, exchange pilot.PageCostPrivateV3) (PreparedRetainedCheckpointPrivateV3, error) {
	before := p.BudgetSpent()
	c, e := NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, a, limits, exchange)
	if e != nil {
		return PreparedRetainedCheckpointPrivateV3{}, e
	}
	exchange, e = pilot.RemainingPageExchangePrivateV3(before, p.BudgetSpent(), exchange)
	if e != nil {
		return PreparedRetainedCheckpointPrivateV3{}, e
	}
	_, prepared, e := c.Advance(ctx, 1, exchange)
	return prepared, e
}
func (p PreparedRetainedCheckpointPrivateV3) valid() bool {
	return p.state != nil && p.state.self == p.state && len(p.state.raw) > 0
}
func (p PreparedRetainedCheckpointPrivateV3) Read(ctx context.Context, exchange pilot.PageCostPrivateV3) ([]byte, error) {
	if !p.valid() {
		return nil, pilot.ErrInvalid
	}
	if _, e := p.state.projection.Read(ctx, exchange); e != nil {
		return nil, e
	}
	return bytes.Clone(p.state.raw), nil
}

type PreparedPublicationRetainedCheckpointPrivateV3 struct {
	state *preparedCheckpointPublicationStatePrivateV3
}
type preparedCheckpointPublicationStatePrivateV3 struct {
	self             *preparedCheckpointPublicationStatePrivateV3
	mu               sync.Mutex
	prepared         *preparedCheckpointStatePrivateV3
	generation       uint64
	references       []string
	complete, failed bool
}

func (PreparedPublicationRetainedCheckpointPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, pilot.ErrInvalid
}
func (p PreparedRetainedCheckpointPrivateV3) Publication(ctx context.Context, exchange pilot.PageCostPrivateV3) (PreparedPublicationRetainedCheckpointPrivateV3, error) {
	if !p.valid() {
		return PreparedPublicationRetainedCheckpointPrivateV3{}, pilot.ErrInvalid
	}
	if _, e := p.state.projection.Read(ctx, exchange); e != nil {
		return PreparedPublicationRetainedCheckpointPrivateV3{}, e
	}
	s := &preparedCheckpointPublicationStatePrivateV3{prepared: p.state}
	s.self = s
	return PreparedPublicationRetainedCheckpointPrivateV3{s}, nil
}
func (c PreparedPublicationRetainedCheckpointPrivateV3) Advance(ctx context.Context, generation uint64, exchange pilot.PageCostPrivateV3) (PreparedPublicationProgressPrivateV3, error) {
	if c.state == nil || c.state.self != c.state {
		return PreparedPublicationProgressPrivateV3{}, pilot.ErrInvalid
	}
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	progress := func(status string) PreparedPublicationProgressPrivateV3 {
		p := PreparedPublicationProgressPrivateV3{Status: status, VerifiedReferences: len(s.references), Cumulative: s.prepared.projection.BudgetSpent()}
		if len(s.references) > 0 {
			p.CanonicalSHA256 = s.references[0]
		}
		if len(s.references) > 1 {
			p.AssessmentSHA256 = s.references[1]
		}
		if len(s.references) > 2 {
			p.PreparedSHA256 = s.references[2]
		}
		return p
	}
	if ctx == nil || s.complete || s.failed || generation != s.generation+1 {
		return progress("INVALID"), pilot.ErrInvalid
	}
	s.generation = generation
	raw := [3][]byte{s.prepared.canonical, s.prepared.assessment, s.prepared.raw}
	for len(s.references) < 3 {
		before := s.prepared.projection.BudgetSpent()
		id, e := s.prepared.projection.RetainPreparedBytes(ctx, raw[len(s.references)], exchange)
		if e != nil {
			status := "FAILED"
			if errors.Is(e, pilot.ErrPagesPausedPrivateV3) {
				status = "PAUSED_BUDGET"
			} else if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
				status = "CANCELLED"
			} else {
				s.failed = true
			}
			return progress(status), e
		}
		s.references = append(s.references, id)
		exchange, e = pilot.RemainingPageExchangePrivateV3(before, s.prepared.projection.BudgetSpent(), exchange)
		if e != nil {
			return progress("PAUSED_BUDGET"), e
		}
	}
	s.complete = true
	return progress("PUBLISHED_REOPENED_OBSERVATIONAL_PREPARED_PRIVATE_V3"), nil
}
