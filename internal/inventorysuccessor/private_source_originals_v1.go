package inventorysuccessor

import (
	"bytes"
	"context"
	"lsp-trace/internal/targetpacket"
	"time"
)

// PrivateProducerOriginals is completion-bound producer custody, not a Manager
// token, candidate, DTO, hash or callback. No exported constructor exists.
type PrivateProducerOriginals struct {
	state     *privateProducerOriginalsV1
	candidate targetpacket.CandidateOriginals
}
type privateProducerOriginalsV1 struct {
	candidate targetpacket.CandidateOriginals
	timing    *privateSourceTimingStateV2
}

func mintPrivateProducerOriginals(c targetpacket.CandidateOriginals) PrivateProducerOriginals {
	return PrivateProducerOriginals{&privateProducerOriginalsV1{candidate: c}, c}
}
func (o PrivateProducerOriginals) Bound() uint64 {
	if o.state == nil {
		return 0
	}
	return o.candidate.Bound()
}
func (o PrivateProducerOriginals) Workspace() string {
	if o.state == nil {
		return ""
	}
	return o.candidate.Workspace()
}
func (o PrivateProducerOriginals) InputID() string {
	if o.state == nil {
		return ""
	}
	return o.candidate.InputID()
}
func (o PrivateProducerOriginals) SourceByteLength() int {
	if o.state == nil {
		return 0
	}
	return o.candidate.SourceByteLength()
}
func (o PrivateProducerOriginals) Deadline() time.Time {
	if o.state == nil {
		return time.Time{}
	}
	return o.candidate.Deadline()
}
func (o PrivateProducerOriginals) Same(other PrivateProducerOriginals) bool {
	return o.state != nil && o.state == other.state
}

// SourceOriginalsV1 is the only mint entry. sourceConfirmed is set only after
// successful lease return, Finish Completed/workers=0 and confirmed kernel
// Observe in completeFixed. The retained path instead requires its opaque
// joined-acquisition/M-readback completion token. All public diagnostics must still match the private
// producer originals. Lower Manager consumption cannot enter this barrier.
func (in PrivateManagedGroupingInput) SourceOriginalsV1() (PrivateProducerOriginals, error) {
	if (!in.sourceConfirmed && !in.retainedCompletion.valid(in)) || len(in.inputOriginal) == 0 || len(in.inputOriginal) > 1<<20 || in.Input.ByteLength() != len(in.inputOriginal) || len(in.originals) > 8 || len(in.Predecessors) != len(in.originals) || len(in.sourceQueryOriginal) == 0 || len(in.sourceQueryOriginal) > 1<<20 || in.Input.ID() != in.sourceInputOriginal.ID() || len(in.Ledger) != len(in.ledgerOriginal) || len(in.Dispositions) != len(in.dispositionsOriginal) {
		return PrivateProducerOriginals{}, ErrPrivateManagedGrouping
	}
	for i, p := range in.originals {
		v := in.Predecessors[i]
		if p.Role != v.Role || p.Selector != v.Selector || p.Schema != v.Schema || !bytes.Equal(p.Bytes, v.Bytes) {
			return PrivateProducerOriginals{}, ErrPrivateManagedGrouping
		}
	}
	for i, v := range in.ledgerOriginal {
		if v != in.Ledger[i] {
			return PrivateProducerOriginals{}, ErrPrivateManagedGrouping
		}
	}
	for k, v := range in.dispositionsOriginal {
		if in.Dispositions[k] != v {
			return PrivateProducerOriginals{}, ErrPrivateManagedGrouping
		}
	}
	c, err := targetpacket.RetainPrivateSourceCandidate(in.sourceEvidence, in.sourceInputOriginal, in.originals, in.sourceQueryOriginal, in.replayBinding.Deadline, in.replayBinding.Session.Workspace)
	if in.sourceTiming != nil {
		f := in.sourceTiming.frame
		if !f.Valid() || !f.CaptureDeadline.Equal(in.replayBinding.Deadline) {
			return PrivateProducerOriginals{}, ErrPrivateManagedGrouping
		}
		c, err = targetpacket.RetainPrivateSourceHOSTCandidate(in.sourceEvidence, in.sourceInputOriginal, in.originals, in.sourceQueryOriginal, in.replayBinding.Deadline, f.TotalDeadline, in.replayBinding.Session.Workspace)
	}
	if err != nil {
		return PrivateProducerOriginals{}, err
	}
	o := mintPrivateProducerOriginals(c)
	o.state.timing = in.sourceTiming
	return o, nil
}
func (in PrivateManagedGroupingInput) SourceDeadlineV1() time.Time {
	if in.sourceConfirmed && in.sourceTiming != nil {
		return in.sourceTiming.frame.TotalDeadline
	}
	return in.replayBinding.Deadline
}

// SourcePreparationDeadlineV2 observes M's once-derived preparation child.
func (in PrivateManagedGroupingInput) SourcePreparationDeadlineV2() (time.Time, bool) {
	if !in.sourceConfirmed || in.sourceTiming == nil {
		return time.Time{}, false
	}
	return in.sourceTiming.preparationDeadline, true
}

// HOSTTimingV2 observes a completed producer's immutable policy, never issuance.
func (o PrivateProducerOriginals) HOSTTimingV2() (PrivateSourceTimingFrameV2, bool) {
	if o.state == nil || o.state.timing == nil {
		return PrivateSourceTimingFrameV2{}, false
	}
	f := o.state.timing.frame
	return f, f.Valid() && o.Deadline().Equal(f.TotalDeadline)
}

// HOSTRetainedContextV2 returns the original invocation scope, not the completed
// and canceled capture child. It is never serialized or a replacement context.
func (o PrivateProducerOriginals) HOSTRetainedContextV2() context.Context {
	if o.state == nil || o.state.timing == nil {
		return nil
	}
	return o.state.timing.retainedContext
}
func PreparePrivateProducerOriginals(o PrivateProducerOriginals, l targetpacket.PrivateSourceLimits) (PrivateProducerOriginals, error) {
	if o.state == nil {
		return PrivateProducerOriginals{}, ErrPrivateManagedGrouping
	}
	c, err := targetpacket.PreparePrivateSourceCandidate(o.candidate, l)
	if err != nil {
		return PrivateProducerOriginals{}, err
	}
	o.candidate = c
	return o, nil
}
