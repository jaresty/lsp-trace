// Code generated for the fixed accepted v2 registry; DO NOT EDIT.
package programcrepresentative

import "fmt"

func EncodeReviewTranscriptSnapshotV2(v ReviewTranscriptSnapshotV2) (ReviewTranscriptSnapshotV2, ArtifactBindingDTO, []byte, error) {
	if v.SchemaVersion != "lsp-trace.program-c-representative-review-transcript-snapshot.private.v2" || v.Authority != 0 || v.Accepted {
		return ReviewTranscriptSnapshotV2{}, ArtifactBindingDTO{}, nil, fmt.Errorf("invalid ReviewTranscriptSnapshotV2 version/ceiling")
	}
	v.Identity = ""
	n, err := CanonicalSizeReviewTranscriptSnapshotV2(v)
	pre, err := marshalSizedV2(v, n, err)
	if err != nil {
		return ReviewTranscriptSnapshotV2{}, ArtifactBindingDTO{}, nil, err
	}
	v.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-review-transcript-snapshot:v2", pre)
	n, err = CanonicalSizeReviewTranscriptSnapshotV2(v)
	raw, err := marshalSizedV2(v, n, err)
	if err != nil {
		return ReviewTranscriptSnapshotV2{}, ArtifactBindingDTO{}, nil, err
	}
	return v, multiSeedBindingV2(v.SchemaVersion, v.Identity, raw), raw, nil
}
func ParseReviewTranscriptSnapshotV2(raw []byte) (ReviewTranscriptSnapshotV2, error) {
	v, err := decodeCanonicalTypedV2(raw, CanonicalSizeReviewTranscriptSnapshotV2)
	if err != nil {
		return ReviewTranscriptSnapshotV2{}, err
	}
	sealed, _, _, err := EncodeReviewTranscriptSnapshotV2(v)
	if err != nil || v.Identity != sealed.Identity {
		return ReviewTranscriptSnapshotV2{}, fmt.Errorf("invalid ReviewTranscriptSnapshotV2 identity")
	}
	return v, nil
}
func EncodeReviewTaskSeparationV2(v ReviewTaskSeparationV2) (ReviewTaskSeparationV2, ArtifactBindingDTO, []byte, error) {
	if v.SchemaVersion != "lsp-trace.program-c-representative-review-task-separation.private.v2" || v.Authority != 0 || v.Accepted {
		return ReviewTaskSeparationV2{}, ArtifactBindingDTO{}, nil, fmt.Errorf("invalid ReviewTaskSeparationV2 version/ceiling")
	}
	v.Identity = ""
	n, err := CanonicalSizeReviewTaskSeparationV2(v)
	pre, err := marshalSizedV2(v, n, err)
	if err != nil {
		return ReviewTaskSeparationV2{}, ArtifactBindingDTO{}, nil, err
	}
	v.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-review-task-separation:v2", pre)
	n, err = CanonicalSizeReviewTaskSeparationV2(v)
	raw, err := marshalSizedV2(v, n, err)
	if err != nil {
		return ReviewTaskSeparationV2{}, ArtifactBindingDTO{}, nil, err
	}
	return v, multiSeedBindingV2(v.SchemaVersion, v.Identity, raw), raw, nil
}
func ParseReviewTaskSeparationV2(raw []byte) (ReviewTaskSeparationV2, error) {
	v, err := decodeCanonicalTypedV2(raw, CanonicalSizeReviewTaskSeparationV2)
	if err != nil {
		return ReviewTaskSeparationV2{}, err
	}
	sealed, _, _, err := EncodeReviewTaskSeparationV2(v)
	if err != nil || v.Identity != sealed.Identity {
		return ReviewTaskSeparationV2{}, fmt.Errorf("invalid ReviewTaskSeparationV2 identity")
	}
	return v, nil
}
func EncodeQualificationReviewV2(v QualificationReviewV2) (QualificationReviewV2, ArtifactBindingDTO, []byte, error) {
	if v.SchemaVersion != "lsp-trace.program-c-representative-qualification-review.private.v2" || v.Authority != 0 || v.Accepted || v.Profile != ProfileCallsOnlyFrozenCensusV1 {
		return QualificationReviewV2{}, ArtifactBindingDTO{}, nil, fmt.Errorf("invalid QualificationReviewV2 version/ceiling")
	}
	v.Identity = ""
	n, err := CanonicalSizeQualificationReviewV2(v)
	pre, err := marshalSizedV2(v, n, err)
	if err != nil {
		return QualificationReviewV2{}, ArtifactBindingDTO{}, nil, err
	}
	v.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-qualification-review:v2", pre)
	n, err = CanonicalSizeQualificationReviewV2(v)
	raw, err := marshalSizedV2(v, n, err)
	if err != nil {
		return QualificationReviewV2{}, ArtifactBindingDTO{}, nil, err
	}
	return v, multiSeedBindingV2(v.SchemaVersion, v.Identity, raw), raw, nil
}
func ParseQualificationReviewV2(raw []byte) (QualificationReviewV2, error) {
	v, err := decodeCanonicalTypedV2(raw, CanonicalSizeQualificationReviewV2)
	if err != nil {
		return QualificationReviewV2{}, err
	}
	sealed, _, _, err := EncodeQualificationReviewV2(v)
	if err != nil || v.Identity != sealed.Identity {
		return QualificationReviewV2{}, fmt.Errorf("invalid QualificationReviewV2 identity")
	}
	return v, nil
}
func EncodeQualificationEnvelopeV2(v QualificationEnvelopeV2) (QualificationEnvelopeV2, ArtifactBindingDTO, []byte, error) {
	if v.SchemaVersion != "lsp-trace.program-c-representative-qualification.private.v2" || v.Authority != 0 || v.Accepted || v.Profile != ProfileCallsOnlyFrozenCensusV1 {
		return QualificationEnvelopeV2{}, ArtifactBindingDTO{}, nil, fmt.Errorf("invalid QualificationEnvelopeV2 version/ceiling")
	}
	v.Identity = ""
	n, err := CanonicalSizeQualificationEnvelopeV2(v)
	pre, err := marshalSizedV2(v, n, err)
	if err != nil {
		return QualificationEnvelopeV2{}, ArtifactBindingDTO{}, nil, err
	}
	v.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-qualification:v2", pre)
	n, err = CanonicalSizeQualificationEnvelopeV2(v)
	raw, err := marshalSizedV2(v, n, err)
	if err != nil {
		return QualificationEnvelopeV2{}, ArtifactBindingDTO{}, nil, err
	}
	return v, multiSeedBindingV2(v.SchemaVersion, v.Identity, raw), raw, nil
}
func ParseQualificationEnvelopeV2(raw []byte) (QualificationEnvelopeV2, error) {
	v, err := decodeCanonicalTypedV2(raw, CanonicalSizeQualificationEnvelopeV2)
	if err != nil {
		return QualificationEnvelopeV2{}, err
	}
	sealed, _, _, err := EncodeQualificationEnvelopeV2(v)
	if err != nil || v.Identity != sealed.Identity {
		return QualificationEnvelopeV2{}, fmt.Errorf("invalid QualificationEnvelopeV2 identity")
	}
	return v, nil
}
func EncodeDenominatorEntryV2(v DenominatorEntryV2) (DenominatorEntryV2, ArtifactBindingDTO, []byte, error) {
	if v.SchemaVersion != "lsp-trace.program-c-representative-denominator-entry.private.v2" || v.Authority != 0 || v.Accepted {
		return DenominatorEntryV2{}, ArtifactBindingDTO{}, nil, fmt.Errorf("invalid DenominatorEntryV2 version/ceiling")
	}
	v.Identity = ""
	n, err := CanonicalSizeDenominatorEntryV2(v)
	pre, err := marshalSizedV2(v, n, err)
	if err != nil {
		return DenominatorEntryV2{}, ArtifactBindingDTO{}, nil, err
	}
	v.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-denominator-entry:v2", pre)
	n, err = CanonicalSizeDenominatorEntryV2(v)
	raw, err := marshalSizedV2(v, n, err)
	if err != nil {
		return DenominatorEntryV2{}, ArtifactBindingDTO{}, nil, err
	}
	return v, multiSeedBindingV2(v.SchemaVersion, v.Identity, raw), raw, nil
}
func ParseDenominatorEntryV2(raw []byte) (DenominatorEntryV2, error) {
	v, err := decodeCanonicalTypedV2(raw, CanonicalSizeDenominatorEntryV2)
	if err != nil {
		return DenominatorEntryV2{}, err
	}
	sealed, _, _, err := EncodeDenominatorEntryV2(v)
	if err != nil || v.Identity != sealed.Identity {
		return DenominatorEntryV2{}, fmt.Errorf("invalid DenominatorEntryV2 identity")
	}
	return v, nil
}
func EncodeDenominatorV2(v DenominatorV2) (DenominatorV2, ArtifactBindingDTO, []byte, error) {
	if v.SchemaVersion != "lsp-trace.program-c-representative-denominator.private.v2" || v.Authority != 0 || v.Accepted || v.Profile != ProfileCallsOnlyFrozenCensusV1 {
		return DenominatorV2{}, ArtifactBindingDTO{}, nil, fmt.Errorf("invalid DenominatorV2 version/ceiling")
	}
	v.Identity = ""
	n, err := CanonicalSizeDenominatorV2(v)
	pre, err := marshalSizedV2(v, n, err)
	if err != nil {
		return DenominatorV2{}, ArtifactBindingDTO{}, nil, err
	}
	v.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-denominator:v2", pre)
	n, err = CanonicalSizeDenominatorV2(v)
	raw, err := marshalSizedV2(v, n, err)
	if err != nil {
		return DenominatorV2{}, ArtifactBindingDTO{}, nil, err
	}
	return v, multiSeedBindingV2(v.SchemaVersion, v.Identity, raw), raw, nil
}
func ParseDenominatorV2(raw []byte) (DenominatorV2, error) {
	v, err := decodeCanonicalTypedV2(raw, CanonicalSizeDenominatorV2)
	if err != nil {
		return DenominatorV2{}, err
	}
	sealed, _, _, err := EncodeDenominatorV2(v)
	if err != nil || v.Identity != sealed.Identity {
		return DenominatorV2{}, fmt.Errorf("invalid DenominatorV2 identity")
	}
	return v, nil
}
func EncodeLineageRecordV2(v LineageRecordV2) (LineageRecordV2, ArtifactBindingDTO, []byte, error) {
	if v.SchemaVersion != "lsp-trace.program-c-representative-lineage-record.private.v2" || v.Authority != 0 || v.Accepted {
		return LineageRecordV2{}, ArtifactBindingDTO{}, nil, fmt.Errorf("invalid LineageRecordV2 version/ceiling")
	}
	v.Identity = ""
	n, err := CanonicalSizeLineageRecordV2(v)
	pre, err := marshalSizedV2(v, n, err)
	if err != nil {
		return LineageRecordV2{}, ArtifactBindingDTO{}, nil, err
	}
	v.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-lineage-record:v2", pre)
	n, err = CanonicalSizeLineageRecordV2(v)
	raw, err := marshalSizedV2(v, n, err)
	if err != nil {
		return LineageRecordV2{}, ArtifactBindingDTO{}, nil, err
	}
	return v, multiSeedBindingV2(v.SchemaVersion, v.Identity, raw), raw, nil
}
func ParseLineageRecordV2(raw []byte) (LineageRecordV2, error) {
	v, err := decodeCanonicalTypedV2(raw, CanonicalSizeLineageRecordV2)
	if err != nil {
		return LineageRecordV2{}, err
	}
	sealed, _, _, err := EncodeLineageRecordV2(v)
	if err != nil || v.Identity != sealed.Identity {
		return LineageRecordV2{}, fmt.Errorf("invalid LineageRecordV2 identity")
	}
	return v, nil
}
func EncodeLineageV2(v LineageV2) (LineageV2, ArtifactBindingDTO, []byte, error) {
	if v.SchemaVersion != "lsp-trace.program-c-representative-lineage.private.v2" || v.Authority != 0 || v.Accepted || v.Profile != ProfileCallsOnlyFrozenCensusV1 {
		return LineageV2{}, ArtifactBindingDTO{}, nil, fmt.Errorf("invalid LineageV2 version/ceiling")
	}
	v.Identity = ""
	n, err := CanonicalSizeLineageV2(v)
	pre, err := marshalSizedV2(v, n, err)
	if err != nil {
		return LineageV2{}, ArtifactBindingDTO{}, nil, err
	}
	v.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-lineage:v2", pre)
	n, err = CanonicalSizeLineageV2(v)
	raw, err := marshalSizedV2(v, n, err)
	if err != nil {
		return LineageV2{}, ArtifactBindingDTO{}, nil, err
	}
	return v, multiSeedBindingV2(v.SchemaVersion, v.Identity, raw), raw, nil
}
func ParseLineageV2(raw []byte) (LineageV2, error) {
	v, err := decodeCanonicalTypedV2(raw, CanonicalSizeLineageV2)
	if err != nil {
		return LineageV2{}, err
	}
	sealed, _, _, err := EncodeLineageV2(v)
	if err != nil || v.Identity != sealed.Identity {
		return LineageV2{}, fmt.Errorf("invalid LineageV2 identity")
	}
	return v, nil
}
