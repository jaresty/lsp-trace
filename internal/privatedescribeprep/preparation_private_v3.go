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

const RequestTypePrivateV3 = "PRIVATE_PAGED_OWN_AST_PREPARATION_V3"

type OwnASTBodyPrivateV3 struct {
	Target, NodeID, SourceURI, SourceBodySHA256, OwnBodySHA256 string
	Declaration                                                Declaration
}
type OwnASTReportPrivateV3 struct {
	RequestType                                                      string
	Source                                                           derived.SourceBindingPrivateV3
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
type PreparedPrivateV3 struct{ state *preparedStatePrivateV3 }
type preparedStatePrivateV3 struct {
	self                       *preparedStatePrivateV3
	projection                 derived.ProjectionPrivateV3
	raw, canonical, assessment []byte
}

func (PreparedPrivateV3) MarshalJSON() ([]byte, error) { return nil, fmt.Errorf("opaque prepared V3") }

type OwnASTReaderPrivateV3 struct{ state *ownASTReaderStatePrivateV3 }
type ownASTReaderStatePrivateV3 struct {
	self                    *ownASTReaderStatePrivateV3
	mu                      sync.Mutex
	projection              derived.ProjectionPrivateV3
	canonical               []byte
	maxBytes, reservedBytes int
	targets                 []acquisition.TargetResult
	report                  OwnASTReportPrivateV3
	generation              uint64
	complete, failed        bool
}
type OwnASTProgressPrivateV3 struct {
	Status                string
	Prepared, Denominator int
	Cumulative            pilot.PageCostPrivateV3
}

func (OwnASTReaderPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("opaque V3 own AST cursor")
}

// An explicit sibling schema: a publication is one bounded prepared batch object,
// not a V2 post-preparation owner or its old host wire. MaxPartBytes bounds that
// one indivisible object; MaxParts admits one object, not hidden chunk expansion.
func NewOwnASTReaderPrivateV3(ctx context.Context, p derived.ProjectionPrivateV3, a derived.AssessmentPrivateV3, limits Limits, exchange pilot.PageCostPrivateV3) (OwnASTReaderPrivateV3, error) {
	if ctx == nil || !limits.valid() {
		return OwnASTReaderPrivateV3{}, fmt.Errorf("V3 explicit bounded preparation limits required")
	}
	before := p.BudgetSpent()
	_, assessment, e := derived.ValidateInstabilityPrivateV3(ctx, p, a, exchange)
	if e != nil {
		return OwnASTReaderPrivateV3{}, e
	}
	exchange, e = pilot.RemainingPageExchangePrivateV3(before, p.BudgetSpent(), exchange)
	if e != nil {
		return OwnASTReaderPrivateV3{}, e
	}
	v, e := p.Read(ctx, exchange)
	if e != nil {
		return OwnASTReaderPrivateV3{}, e
	}
	r := OwnASTReportPrivateV3{RequestType: RequestTypePrivateV3, Source: v.Source, AssessmentSHA256: pilot.Digest(assessment), Assessment: assessment, Limits: limits, FullTargetCount: v.Source.FullTargetCount, FullBatchCount: v.Source.FullBatchCount, BatchTargetCount: v.Source.BatchTargetCount, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	r.SourceInputID = digest(struct {
		Request    string
		Source     derived.SourceBindingPrivateV3
		Assessment string
	}{RequestTypePrivateV3, v.Source, r.AssessmentSHA256})
	targets := v.Selected.Source.Evidence.Acquisition.Targets
	if len(targets) > limits.MaxTargets {
		return OwnASTReaderPrivateV3{}, pilot.ErrPagesUnitPrivateV3
	}
	maxBytes := limits.MaxPartBytes
	if limits.MaxTotalBytes < maxBytes {
		maxBytes = limits.MaxTotalBytes
	}
	if unit := int(v.Selected.Revision.Policy.ComputationUnitBytes); unit < maxBytes {
		maxBytes = unit
	}
	// Header is bounded before adding any AST bodies; +256 covers the change
	// from nil to a finite body array and all finite counter/length digits.
	metadata := r
	metadata.Assessment = nil
	header, e := json.Marshal(metadata)
	if e != nil {
		return OwnASTReaderPrivateV3{}, e
	}
	if len(header) > maxBytes-256 || len(assessment) > maxBytes-256-len(header) {
		return OwnASTReaderPrivateV3{}, pilot.ErrPagesUnitPrivateV3
	}
	st := &ownASTReaderStatePrivateV3{projection: p, canonical: v.DerivedCanonicalV5, targets: targets, report: r, maxBytes: maxBytes, reservedBytes: len(header) + len(assessment) + 256}
	st.self = st
	return OwnASTReaderPrivateV3{st}, nil
}

// One generation per attempt. Completed target facts survive later source,
// cancellation or budget failure. No partial body bytes escape on a pause.
func (c OwnASTReaderPrivateV3) Advance(ctx context.Context, generation uint64, exchange pilot.PageCostPrivateV3) (OwnASTProgressPrivateV3, PreparedPrivateV3, error) {
	if c.state == nil || c.state.self != c.state {
		return OwnASTProgressPrivateV3{}, PreparedPrivateV3{}, pilot.ErrInvalid
	}
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	progress := func(status string) OwnASTProgressPrivateV3 {
		return OwnASTProgressPrivateV3{status, len(s.report.Bodies), len(s.targets), s.projection.BudgetSpent()}
	}
	if ctx == nil || s.complete || s.failed || generation != s.generation+1 {
		return progress("INVALID"), PreparedPrivateV3{}, pilot.ErrInvalid
	}
	s.generation = generation
	fail := func(e error) (OwnASTProgressPrivateV3, PreparedPrivateV3, error) {
		status := "FAILED"
		if errors.Is(e, pilot.ErrPagesPausedPrivateV3) {
			status = "PAUSED_BUDGET"
		} else if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
			status = "CANCELLED"
		} else {
			s.failed = true
		}
		return progress(status), PreparedPrivateV3{}, e
	}
	// This stage reservation precedes all preparation metadata/hash/parse work,
	// including the final seal after a prior exchange prepared the last body.
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
			return fail(fmt.Errorf("V3 unresolved own target"))
		}
		// No detached capture content or ambient path is a source-body fallback.
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
		d, gap := declaration(body, *n, s.report.Source.PositionEncoding)
		if gap != "" {
			return fail(fmt.Errorf("V3 own AST source gap: %s", gap))
		}
		b := OwnASTBodyPrivateV3{Target: t.Requested.ID, NodeID: n.ID, SourceURI: n.URI, SourceBodySHA256: pilot.Digest(body), OwnBodySHA256: pilot.Digest(d.Bytes), Declaration: d}
		// JSON strings expand by at most six bytes per input byte, binary data
		// by four per three, and these fixed structs have bounded numeric/range
		// metadata. Reserve the conservative bound BEFORE growing final JSON.
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
	st := &preparedStatePrivateV3{projection: s.projection, raw: raw, canonical: s.canonical, assessment: s.report.Assessment}
	st.self = st
	s.complete = true
	return progress("COMPLETE_OWN_AST_PRIVATE_V3"), PreparedPrivateV3{st}, nil
}
func PrivateOwnASTPrepareV3(ctx context.Context, p derived.ProjectionPrivateV3, a derived.AssessmentPrivateV3, limits Limits, exchange pilot.PageCostPrivateV3) (PreparedPrivateV3, error) {
	before := p.BudgetSpent()
	c, e := NewOwnASTReaderPrivateV3(ctx, p, a, limits, exchange)
	if e != nil {
		return PreparedPrivateV3{}, e
	}
	exchange, e = pilot.RemainingPageExchangePrivateV3(before, p.BudgetSpent(), exchange)
	if e != nil {
		return PreparedPrivateV3{}, e
	}
	_, prepared, e := c.Advance(ctx, 1, exchange)
	return prepared, e
}
func (p PreparedPrivateV3) valid() bool {
	return p.state != nil && p.state.self == p.state && len(p.state.raw) > 0
}
func (p PreparedPrivateV3) Read(ctx context.Context, exchange pilot.PageCostPrivateV3) ([]byte, error) {
	if !p.valid() {
		return nil, fmt.Errorf("sealed V3 prepared result required")
	}
	if _, e := p.state.projection.Read(ctx, exchange); e != nil {
		return nil, e
	}
	return bytes.Clone(p.state.raw), nil
}

// Publication first retains the referenced canonical graph and assessment, then
// the prepared object. Saved progress is original-owner, in-memory only.
type PreparedPublicationPrivateV3 struct {
	state *preparedPublicationStatePrivateV3
}
type preparedPublicationStatePrivateV3 struct {
	self             *preparedPublicationStatePrivateV3
	mu               sync.Mutex
	prepared         *preparedStatePrivateV3
	generation       uint64
	references       []string
	complete, failed bool
}
type PreparedPublicationProgressPrivateV3 struct {
	Status                                            string
	VerifiedReferences                                int
	PreparedSHA256, CanonicalSHA256, AssessmentSHA256 string
	Cumulative                                        pilot.PageCostPrivateV3
}

func (PreparedPublicationPrivateV3) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("opaque V3 publication cursor")
}
func (p PreparedPrivateV3) Publication(ctx context.Context, exchange pilot.PageCostPrivateV3) (PreparedPublicationPrivateV3, error) {
	if !p.valid() {
		return PreparedPublicationPrivateV3{}, pilot.ErrInvalid
	}
	if _, e := p.state.projection.Read(ctx, exchange); e != nil {
		return PreparedPublicationPrivateV3{}, e
	}
	s := &preparedPublicationStatePrivateV3{prepared: p.state}
	s.self = s
	return PreparedPublicationPrivateV3{s}, nil
}
func (c PreparedPublicationPrivateV3) Advance(ctx context.Context, generation uint64, exchange pilot.PageCostPrivateV3) (PreparedPublicationProgressPrivateV3, error) {
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
	for len(s.references) < len(raw) {
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
	return progress("PUBLISHED_PREPARED_PRIVATE_V3"), nil
}
func (p PreparedPrivateV3) Publish(ctx context.Context, exchange pilot.PageCostPrivateV3) (string, error) {
	if !p.valid() {
		return "", fmt.Errorf("sealed V3 prepared result required")
	}
	before := p.state.projection.BudgetSpent()
	c, e := p.Publication(ctx, exchange)
	if e != nil {
		return "", e
	}
	exchange, e = pilot.RemainingPageExchangePrivateV3(before, p.state.projection.BudgetSpent(), exchange)
	if e != nil {
		return "", e
	}
	r, e := c.Advance(ctx, 1, exchange)
	return r.PreparedSHA256, e
}
