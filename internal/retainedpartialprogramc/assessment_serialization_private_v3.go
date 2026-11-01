//go:build darwin || linux

package retainedpartialprogramc

import (
	"context"
	"encoding/json"

	pilot "lsp-trace/internal/censuslineagepilot"
)

const assessmentReportBytesPrivateV3 = 16 << 20
const assessmentWalkWorkPrivateV3 = 1_000_000

// A concrete, non-recursive walk of this report's current field graph. Fixed
// object allowances cover all field names, colons, commas, braces and scalar
// widths (32 bytes covers every JSON int/uint64/finite float64; bool <= 5).
// Values of every string field, including repeated partition members, are
// counted separately. Six bytes per input byte plus quotes bounds JSON's HTML,
// control, quote/backslash, invalid UTF-8 and U+2028/U+2029 escaping without
// scanning or allocating an encoded string. Nil lists cost 4; non-nil lists
// cost at most 4+length before their elements. These are conservative admission
// limits, so otherwise encodable reports may be refused below the actual cap.
//
// There is no reflection, custom-Marshaler dispatch, or generic encoding
// framework here. The schema/field tests must change if these concrete types
// gain fields or custom JSON behavior. The caller's existing original-ledger
// stage reservation covers this bounded count pass; this does not reset it or
// claim that logical reservations measure heap/CPU or practical scalability.
func assessmentEncodedBoundPrivateV3(ctx context.Context, report InstabilityReportPrivateV3) (int, error) {
	if ctx == nil {
		return 0, pilot.ErrInvalid
	}
	b := assessmentBoundPrivateV3{ctx: ctx, remaining: assessmentReportBytesPrivateV3, work: assessmentWalkWorkPrivateV3}
	// Envelope, Source (including all numeric/bool fields), and Math scalars.
	if err := b.take(64 + 4096 + 4096); err != nil {
		return 0, err
	}
	s, a := report.Source, report.Math
	for _, value := range [...]string{
		s.Admission, s.Profile, s.ProfileSHA256, s.ClaimCeiling,
		s.PageBindingSHA256, s.PageRevisionSHA256, s.SourceScopeSHA256, s.EnumerationOriginSHA256,
		s.SourcePolicySHA256, s.RuntimeMetadataSHA256, s.OriginalNativeV3SHA256, s.DerivedCanonicalV5SHA256,
		s.SessionID, s.WorkspaceURI, s.PositionEncoding, s.ProviderCommand,
		s.FeatureIdentity, s.SourceCompleteness, s.AbsenceInference,
		a.Profile, a.PolicySHA256, a.RequestSHA256, a.Digest,
		a.ProjectionProfile, a.ProjectionDigest, a.InputSHA256, a.NativeGraphSHA256,
		a.ClaimCeiling, a.Admission, a.SourceCompleteness, a.FeatureIdentity, a.Outcome, a.Error,
	} {
		if err := b.text(value); err != nil {
			return 0, err
		}
	}
	if err := b.list(len(s.SelectedObjectSHA256)); err != nil {
		return 0, err
	}
	for _, value := range s.SelectedObjectSHA256 {
		if err := b.text(value); err != nil {
			return 0, err
		}
	}
	if err := b.list(len(a.Seeds)); err != nil {
		return 0, err
	}
	for range a.Seeds {
		if err := b.take(20); err != nil {
			return 0, err
		}
	}
	if err := b.list(len(a.Runs)); err != nil {
		return 0, err
	}
	for _, run := range a.Runs {
		// Run and nested Partition keys, scalar widths and punctuation.
		if err := b.take(1024); err != nil {
			return 0, err
		}
		for _, value := range [...]string{run.ID, run.Status, run.Error, run.Partition.Outcome, run.Partition.Algorithm, run.Partition.LogicalDigest} {
			if err := b.text(value); err != nil {
				return 0, err
			}
		}
		if err := b.list(len(run.Partition.Communities)); err != nil {
			return 0, err
		}
		for _, community := range run.Partition.Communities {
			if err := b.take(64); err != nil {
				return 0, err
			}
			if err := b.list(len(community.Members)); err != nil {
				return 0, err
			}
			for _, member := range community.Members {
				if err := b.text(member); err != nil {
					return 0, err
				}
			}
		}
	}
	if err := b.list(len(a.Pairs)); err != nil {
		return 0, err
	}
	for _, pair := range a.Pairs {
		// Pair and nested Metrics keys, scalar widths and punctuation.
		if err := b.take(2048); err != nil {
			return 0, err
		}
		for _, value := range [...]string{pair.Left, pair.Right, pair.Status, pair.Metrics.Outcome, pair.Metrics.Guard} {
			if err := b.text(value); err != nil {
				return 0, err
			}
		}
	}
	return assessmentReportBytesPrivateV3 - b.remaining, nil
}

type assessmentBoundPrivateV3 struct {
	ctx             context.Context
	remaining, work int
}

func (b *assessmentBoundPrivateV3) take(n int) error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if b.work <= 0 || n < 0 || n > b.remaining {
		return pilot.ErrPagesUnitPrivateV3
	}
	b.work--
	b.remaining -= n
	return nil
}

func (b *assessmentBoundPrivateV3) text(value string) error {
	if err := b.take(2); err != nil {
		return err
	}
	// Divide before multiplying: no encoded-size arithmetic can overflow.
	if len(value) > b.remaining/6 {
		return pilot.ErrPagesUnitPrivateV3
	}
	b.remaining -= 6 * len(value)
	return nil
}

func (b *assessmentBoundPrivateV3) list(length int) error {
	if err := b.take(4); err != nil {
		return err
	}
	// Reserve minimum element work and punctuation BEFORE entering any list.
	// Each element's take/text calls then debit the same monotone work counter.
	if length < 0 || length >= b.work {
		return pilot.ErrPagesUnitPrivateV3
	}
	if err := b.take(length); err != nil {
		return err
	}
	b.work -= length
	return nil
}

// beforeMarshal is a private observation/refusal seam, not an encoder override:
// it cannot supply bytes or create an assessment/owner. Production passes nil.
func marshalAssessmentPrivateV3(ctx context.Context, report InstabilityReportPrivateV3, beforeMarshal func() error) ([]byte, error) {
	if _, err := assessmentEncodedBoundPrivateV3(ctx, report); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if beforeMarshal != nil {
		if err := beforeMarshal(); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	if len(raw) > assessmentReportBytesPrivateV3 {
		return nil, pilot.ErrPagesUnitPrivateV3
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}
