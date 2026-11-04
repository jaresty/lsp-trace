package inventorysuccessor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"

	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/publication"
)

type MultiSeedObjectsV2 struct {
	root      *publication.Root
	onPublish func()
}

func NewMultiSeedObjectsV2(root *publication.Root) (*MultiSeedObjectsV2, error) {
	if root == nil || root.ValidatePrivate() != nil {
		return nil, ErrInvalid
	}
	return &MultiSeedObjectsV2{root: root}, nil
}
func multiSeedObjectSelectorV2(b programcrepresentative.ArtifactBindingDTO) (string, error) {
	if b.ByteLength == 0 || b.ByteLength > programcrepresentative.MaxArtifactCanonicalBytesV2 || !sha256Pattern.MatchString(b.SHA256) {
		return "", ErrInvalid
	}
	family, suffix := "", ".json"
	switch b.SchemaVersion {
	case headVersion:
		family = "successor-predecessors"
	case programcrepresentative.MechanicsSchemaVersionV2:
		family = "mechanics"
	case programcrepresentative.CompositionSchemaVersionV2:
		family = "composition"
	case programcrepresentative.GroupingSchemaVersionV2:
		family = "grouping"
	case programcrepresentative.ResultSchemaVersionV2:
		family = "results"
	case programcrepresentative.DenominatorSchemaVersionV2, programcrepresentative.DenominatorEntrySchemaVersionV2:
		family = "denominator"
	case programcrepresentative.LineageSchemaVersionV2, programcrepresentative.LineageRecordSchemaVersionV2:
		family = "lineage"
	case "lsp-trace.community.v1":
		family = "communities"
		if b.Identity != "" {
			return "", ErrInvalid
		}
	case "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2":
		family = "qualification"
		suffix = ".jsonl"
		if b.Identity != "" {
			return "", ErrInvalid
		}
	case programcrepresentative.QualificationSubjectSchemaVersionV2, programcrepresentative.Stage6PolicySchemaVersionV2, programcrepresentative.Stage6LimitsSchemaVersionV2, programcrepresentative.Stage6FinalizationSchemaVersionV2, programcrepresentative.QualificationEnvelopeSchemaVersionV2, programcrepresentative.QualificationReviewSchemaVersionV2, "lsp-trace.program-c-representative-review-transcript-snapshot.private.v2", "lsp-trace.program-c-representative-review-task-separation.private.v2":
		family = "qualification"
	default:
		return "", ErrInvalid
	}
	if b.Identity == "" && b.SchemaVersion != "lsp-trace.community.v1" && suffix != ".jsonl" {
		return "", ErrInvalid
	}
	if b.Identity != "" && !sha256Pattern.MatchString(b.Identity) {
		return "", ErrInvalid
	}
	return "program-c-representative/" + family + "/objects/private/v2/" + strings.TrimPrefix(b.SHA256, "sha256:") + suffix, nil
}
func (s *MultiSeedObjectsV2) Retain(ctx context.Context, b programcrepresentative.ArtifactBindingDTO, raw []byte) error {
	if s == nil || s.root == nil || ctx == nil {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	selector, err := multiSeedObjectSelectorV2(b)
	if err != nil {
		return err
	}
	if err = programcrepresentative.VerifyRawArtifactBindingV2(b, raw); err != nil {
		return err
	}
	if b.SchemaVersion == headVersion {
		revision, err := ParseHeadRevision(raw)
		if err != nil || revision.Generation != 1 || revision.Selector != b.Identity {
			return ErrInvalid
		}
	}
	existing, err := s.root.ReadSelectorWithAbsence(selector, int64(b.ByteLength))
	if err == nil {
		if !bytes.Equal(existing, raw) {
			return ErrInvalid
		}
		_, err = s.Resolve(ctx, b)
		return err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if s.onPublish != nil {
		s.onPublish()
	}
	result := publication.NewPublisher().Publish(publication.Request{Root: s.root, Selector: selector, Bytes: raw, ArtifactSchemaID: b.SchemaVersion})
	if result.Failure != nil && result.Failure.Code != publication.CodeTargetExists {
		return result.Failure
	}
	if result.Failure == nil && (result.Receipt == nil || result.Receipt.Digest != b.SHA256 || result.Receipt.ByteLength != b.ByteLength) {
		return ErrInvalid
	}
	retained, err := s.Resolve(ctx, b)
	if err != nil || !bytes.Equal(retained, raw) {
		return errors.Join(ErrInvalid, err)
	}
	return ctx.Err()
}
func (s *MultiSeedObjectsV2) Resolve(ctx context.Context, b programcrepresentative.ArtifactBindingDTO) ([]byte, error) {
	if s == nil || s.root == nil || ctx == nil {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	selector, err := multiSeedObjectSelectorV2(b)
	if err != nil {
		return nil, err
	}
	raw, err := s.root.ReadSelector(selector, int64(b.ByteLength))
	if err != nil {
		return nil, err
	}
	if err = programcrepresentative.VerifyRawArtifactBindingV2(b, raw); err != nil {
		return nil, err
	}
	if b.SchemaVersion == headVersion {
		revision, err := ParseHeadRevision(raw)
		if err != nil || revision.Generation != 1 || revision.Selector != b.Identity {
			return nil, ErrInvalid
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *MultiSeedObjectsV2) RetainSuccessorPredecessorV2(ctx context.Context, loaded []byte, workflowKey, headKey string, identity WorkflowIdentity, programCHead string) (programcrepresentative.ArtifactBindingDTO, error) {
	zero := programcrepresentative.ArtifactBindingDTO{}
	if ctx == nil || headKey == "" || uint64(len(loaded)) > programcrepresentative.MaxArtifactCanonicalBytesV2 {
		return zero, ErrInvalid
	}
	revision, err := ParseHeadRevision(loaded)
	if err != nil || revision.Generation != 1 {
		return zero, ErrInvalid
	}
	var value head
	if strict(loaded, &value) != nil || value.WorkflowKey != workflowKey || value.Identity != identity || value.PredecessorHead != programCHead {
		return zero, ErrInvalid
	}
	binding := programcrepresentative.ArtifactBindingDTO{SchemaVersion: headVersion, Identity: revision.Selector, SHA256: digest(loaded), ByteLength: uint64(len(loaded))}
	if err = s.Retain(ctx, binding, loaded); err != nil {
		return zero, err
	}
	raw, err := s.ResolveSuccessorPredecessorV2(ctx, binding, workflowKey, headKey, identity, programCHead)
	if err != nil || !bytes.Equal(raw, loaded) {
		return zero, ErrInvalid
	}
	return binding, nil
}
func (s *MultiSeedObjectsV2) ResolveSuccessorPredecessorV2(ctx context.Context, b programcrepresentative.ArtifactBindingDTO, workflowKey, headKey string, identity WorkflowIdentity, programCHead string) ([]byte, error) {
	if b.SchemaVersion != headVersion || headKey == "" {
		return nil, ErrInvalid
	}
	raw, err := s.Resolve(ctx, b)
	if err != nil {
		return nil, err
	}
	revision, err := ParseHeadRevision(raw)
	var value head
	if err != nil || revision.Generation != 1 || revision.Selector != b.Identity || strict(raw, &value) != nil || value.WorkflowKey != workflowKey || value.Identity != identity || value.PredecessorHead != programCHead {
		return nil, ErrInvalid
	}
	return raw, nil
}
