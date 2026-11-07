package sourcecontinuation

import (
	"context"
	"errors"
	"time"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
)

// SourceAcquisitionRequest supplies no authority. Eligibility requires the exact
// existing source Resume and its actual opaque admission/unit/whole-Record join.
type SourceAcquisitionRequest struct {
	Resume  ResumeRequest
	Record  describerequest.Record
	Attempt SuppliedSourceAttempt
}

// SourceAcquisitionHandoff is process-local source eligibility for a prepare-only
// Runner plan. It is NOT permission for execution or authentic HOST acquisition.
// Only opaque references are retained here: no second source or Record payload.
type SourceAcquisitionHandoff struct{ state *sourceAcquisitionHandoff }
type sourceAcquisitionHandoff struct {
	plan            describeworker.SourceAcquisitionPlan
	expected        *expected
	source          describeworker.SourceAcquisitionContext
	retained, extra uint64
}

func (h SourceAcquisitionHandoff) ID() string {
	if h.state == nil {
		return ""
	}
	return h.state.plan.ID()
}

func acquisitionContext(s *expected, workspace string, sourceDigest string, attempt SuppliedSourceAttempt) describeworker.SourceAcquisitionContext {
	wire := s.contract.PacketMaxResponseBytes()
	capture := s.contract.CaptureLimits()
	if capture.MaxArtifactBytes < wire {
		wire = capture.MaxArtifactBytes
	}
	return describeworker.SourceAcquisitionContext{
		HandoffID: s.handoff.ID(), CheckpointID: attempt.CheckpointID,
		SourceUnitID: attempt.SourceUnitID, SourceDigest: sourceDigest,
		Workspace: workspace, PolicyID: s.request.PolicyID, Deadline: s.request.Deadline,
		SourceLimits: s.request.Limits, MaxWireBytes: wire, MaxParentBytes: capture.MaxParentBytes,
	}
}

func PrepareSourceAcquisition(ctx context.Context, runner *describeworker.Runner, q SourceAcquisitionRequest) (SourceAcquisitionHandoff, error) {
	bad := func(s string) (SourceAcquisitionHandoff, error) { return SourceAcquisitionHandoff{}, errors.New(s) }
	if runner == nil {
		return bad("source acquisition concrete runner required")
	}
	s, err := sourceAcquisitionExpected(ctx, q.Resume, q.Attempt)
	if err != nil {
		return SourceAcquisitionHandoff{}, err
	}
	// Sizing uses a trusted, already retained fixed-width source digest. It is
	// not the selected unit's identity; the actual admission join still follows.
	sizing := acquisitionContext(s, q.Resume.Workspace, s.regeneration.digest, q.Attempt)
	earlyExtra, err := runner.SourceAcquisitionRetentionUpper(q.Record, q.Attempt.ID, sizing)
	if err != nil {
		return SourceAcquisitionHandoff{}, err
	}
	earlyExtra, ok := sourceResponseAdd(earlyExtra, 8192)
	if !ok {
		return bad("source acquisition extra retained overflow")
	}
	earlyRetained, err := sourceAcquisitionEarlyEnvelope(s, earlyExtra, 0)
	if err != nil {
		return SourceAcquisitionHandoff{}, err
	}
	out, sourceDigest, err := sourceResponseJoin(ctx, q.Resume, q.Record, q.Attempt)
	if err != nil {
		return SourceAcquisitionHandoff{}, err
	}
	source := acquisitionContext(s, q.Resume.Workspace, sourceDigest, q.Attempt)
	if source.SourceDigest != sourceDigest || source.HandoffID != out.HandoffID {
		return bad("source acquisition source join mismatch")
	}
	// Size the lower candidate BEFORE its validation/marshal/hash buffers exist.
	// Fixed 8192 is only fixed-shape upper references/scratch; all variable config,
	// Record, attempt and source strings are enumerated in the lower allowance.
	extra, err := runner.SourceAcquisitionRetentionUpper(q.Record, q.Attempt.ID, source)
	if err != nil {
		return SourceAcquisitionHandoff{}, err
	}
	extra, ok = sourceResponseAdd(extra, 8192)
	if !ok {
		return bad("source acquisition extra retained overflow")
	}
	retained, ok := sourceResponseRetainedUpper(s, out)
	if !ok {
		return bad("source acquisition retained source bound")
	}
	if extra != earlyExtra || retained != earlyRetained {
		return bad("source acquisition cached/actual bound mismatch")
	}
	peak, ok := sourceResponseAdd(retained, extra)
	if !ok || source.MaxParentBytes <= 0 || peak > uint64(source.MaxParentBytes) {
		return bad("source acquisition retained-plus-plan ceiling")
	}
	plan, err := runner.PrepareSourceAcquisition(ctx, q.Record, q.Attempt.ID, source)
	if err != nil {
		return SourceAcquisitionHandoff{}, err
	}
	if _, err = runner.ValidateSourceAcquisition(ctx, plan, q.Record, q.Attempt.ID, source); err != nil {
		return SourceAcquisitionHandoff{}, err
	}
	if !sourceAcquisitionPublicationLive(ctx, s.request.Deadline) {
		return bad("source acquisition cancelled or expired at eligible publication")
	}
	return SourceAcquisitionHandoff{&sourceAcquisitionHandoff{plan, s, source, retained, extra}}, nil
}

// CompleteSuppliedSourceAcquisition accepts controlled supplied text only. The
// adopted binder performs exact Resume and canonical terminal parsing once. The
// private extra allowance accounts for this handoff while the binder is active.
func CompleteSuppliedSourceAcquisition(ctx context.Context, runner *describeworker.Runner, h SourceAcquisitionHandoff, q SuppliedSourceResponseRequest) (BoundSuppliedSourceResponse, error) {
	bad := func(s string) (BoundSuppliedSourceResponse, error) {
		return BoundSuppliedSourceResponse{}, errors.New(s)
	}
	s := h.state
	if s == nil || runner == nil || q.Resume.PrivateSources == nil || q.Resume.PrivateSources.Expected.state != s.expected {
		return bad("source acquisition upper owner/expected mismatch")
	}
	// Exact full Record equality in the runner occurs before any caller Record
	// marshal. Additional attempt fields and original opaque dependencies are
	// checked by the shared adopted join, not inferred from a public plan ID.
	// Compare caller metadata without concatenating or copying it. The bounded
	// source context was frozen only after the actual source admission join.
	if q.Resume.Workspace != s.source.Workspace || q.Attempt.CheckpointID != s.source.CheckpointID || q.Attempt.SourceUnitID != s.source.SourceUnitID {
		return bad("source acquisition source identity mismatch")
	}
	source := s.source
	lower, err := runner.ValidateSourceAcquisition(ctx, s.plan, q.Record, q.Attempt.ID, source)
	if err != nil {
		return BoundSuppliedSourceResponse{}, err
	}
	extra, ok := sourceResponseAdd(lower, 8192)
	if !ok || extra != s.extra {
		return bad("source acquisition retained plan mismatch")
	}
	peak, ok := sourceResponseAdd(s.retained, extra)
	if !ok || source.MaxParentBytes <= 0 || peak > uint64(source.MaxParentBytes) {
		return bad("source acquisition retained-plus-plan ceiling")
	}
	if err = runner.ValidateSourceAcquisitionResponseLimits(s.plan, q.Limits); err != nil {
		return BoundSuppliedSourceResponse{}, err
	}
	bound, err := bindSuppliedSourceResponse(ctx, q, extra)
	if err != nil {
		return BoundSuppliedSourceResponse{}, err
	}
	if !sourceAcquisitionPublicationLive(ctx, s.expected.request.Deadline) {
		return bad("source acquisition cancelled or expired at completion")
	}
	return bound, nil
}

// Header/scalar checks only. In particular, do not call the source preparation
// preflight's map construction, Unit(), Resume, or any payload serialization.
func sourceAcquisitionExpected(ctx context.Context, r ResumeRequest, attempt SuppliedSourceAttempt) (*expected, error) {
	bad := func() (*expected, error) { return nil, errors.New("source acquisition early expected/context/cache") }
	if ctx == nil || ctx.Err() != nil || r.PrivateSources == nil || r.Worker != nil || r.WorkerV2 != nil || r.FreshCapture != nil || r.PrivateAllMode != nil || r.UseCommittedWorkspace {
		return bad()
	}
	s := r.PrivateSources.Expected.state
	if s == nil || s.stop != StopAfterPrivateSourceRequests || r.StopAfter != s.stop || r.PrivateSources.Version != PrivateAllModeSourcesVersion || r.Selector != s.checkpoint || attempt.CheckpointID != s.checkpoint || !r.PrivateSources.ExpectedHandoff.Same(s.handoff) || s.regeneration.workspace == "" || r.Workspace != s.regeneration.workspace || s.request.Deadline.IsZero() || !time.Now().Before(s.request.Deadline) {
		return bad()
	}
	if d, ok := ctx.Deadline(); ok && d.After(s.request.Deadline) {
		return bad()
	}
	if s.regeneration.response == 0 || s.regeneration.source < s.regeneration.response || s.regeneration.lower == 0 || len(s.regeneration.digest) != 71 {
		return bad()
	}
	return s, nil
}

// Both phase peaks include simultaneously held source/Runner data. During the
// upper phase the lower metadata result remains held; lower raw materials do
// not survive lower Resume. This is a logical liveness bound, not Go GC/heap FIT.
func sourceAcquisitionEarlyEnvelope(s *expected, extra, raw uint64) (uint64, error) {
	bad := func() (uint64, error) { return 0, errors.New("source acquisition early regeneration ceiling") }
	if s == nil || extra == 0 || s.regeneration.response == 0 || s.regeneration.source < s.regeneration.response || s.regeneration.lower == 0 {
		return bad()
	}
	handoff, ok := s.handoff.RetainedPayloadUpper()
	if !ok {
		return bad()
	}
	cap := s.contract.CaptureLimits().MaxParentBytes
	if cap <= 0 {
		return bad()
	}
	_, ok = sourceAcquisitionEnvelopePeak(handoff, s.regeneration.source, s.regeneration.lower, extra, raw, uint64(cap))
	if !ok {
		return bad()
	}
	response, ok := sourceResponseAdd(handoff, s.regeneration.response)
	if !ok {
		return bad()
	}
	return response, nil
}

// Checked source-specific scalar arithmetic shared with the boundary guards.
// Both phase values already include their simultaneously held source payloads.
func sourceAcquisitionEnvelopePeak(handoff, source, lower, extra, raw, parent uint64) (uint64, bool) {
	if handoff == 0 || source == 0 || lower == 0 || extra == 0 || parent == 0 {
		return 0, false
	}
	peak := source
	if lower > peak {
		peak = lower
	}
	peak, ok := sourceResponseAdd(handoff, peak, extra, raw)
	return peak, ok && peak <= parent
}

// Acquisition completion reserves the existing terminal parse/output envelope
// before joining as well. Ordinary zero-extra supplied binding does not use this
// private cache path. Fitting any bound is NOT source/Runner admission.
func sourceAcquisitionSuppliedPreflight(ctx context.Context, q SuppliedSourceResponseRequest, extra uint64) error {
	s, err := sourceAcquisitionExpected(ctx, q.Resume, q.Attempt)
	if err != nil {
		return err
	}
	retained, err := sourceAcquisitionEarlyEnvelope(s, extra, uint64(len(q.AdapterLine)))
	if err != nil {
		return err
	}
	retained, ok := sourceResponseAdd(retained, extra)
	if !ok {
		return errors.New("source acquisition early retained overflow")
	}
	record, ok := sourceResponseRecordUpper(q.Record)
	if !ok {
		return errors.New("source acquisition early record overflow")
	}
	metadata, ok := sourceResponseStringBytes(s.handoff.ID(), s.checkpoint, q.Attempt.SourceUnitID, s.regeneration.digest, q.Resume.Workspace, q.Attempt.ID)
	if !ok {
		return errors.New("source acquisition early metadata overflow")
	}
	wire := s.contract.PacketMaxResponseBytes()
	capture := s.contract.CaptureLimits()
	if capture.MaxArtifactBytes < wire {
		wire = capture.MaxArtifactBytes
	}
	if wire <= 0 || capture.MaxParentBytes <= 0 {
		return errors.New("source acquisition early ceilings")
	}
	_, err = sourceResponseAggregateBound(retained, record, metadata, uint64(len(q.AdapterLine)), uint64(q.Limits.MaxSemanticBytes), uint64(wire), uint64(capture.MaxParentBytes))
	return err
}

// Logical final publication liveness, not an atomic OS-clock/physical heap claim.
func sourceAcquisitionPublicationLive(ctx context.Context, deadline time.Time) bool {
	return ctx != nil && ctx.Err() == nil && !deadline.IsZero() && time.Now().Before(deadline)
}
