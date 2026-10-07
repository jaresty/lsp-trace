package continuationhost

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/publication"
)

const candidateGroupArtifactNamespace = "continuations/candidate-groups/artifacts"

func candidateGroupPath(namespace, id string) (string, error) {
	if !isDigest(id) {
		return "", errors.New("continuationhost: invalid candidate group identity")
	}
	return namespace + "/" + strings.TrimPrefix(id, "sha256:"), nil
}

func (s *Store) publishCandidateGroupObject(ctx context.Context, path string, raw []byte, schema string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.root == nil || s.publisher == nil || int64(len(raw)) > s.maxBytes {
		return errors.New("continuationhost: candidate group rejected")
	}
	result := s.publisher.Publish(publication.Request{Root: s.root, Selector: path, Bytes: append([]byte(nil), raw...), ArtifactSchemaID: schema})
	if result.Failure == nil {
		if result.Receipt == nil || result.Receipt.Digest != Digest(raw) || result.Receipt.ByteLength != uint64(len(raw)) {
			return errors.New("continuationhost: candidate group publication verification failed")
		}
		return nil
	}
	if result.Failure.Code != publication.CodeTargetExists {
		return errors.New("continuationhost: candidate group publication failed")
	}
	existing, err := s.root.ReadSelector(path, s.maxBytes)
	if err != nil || !bytes.Equal(existing, raw) {
		return errors.New("continuationhost: candidate group immutable collision")
	}
	return nil
}

type CandidateGroupPublicationReceipt struct {
	ArtifactID string
	Selector   string
	Digest     string
	ByteLength uint64
}

func (s *Store) PutCandidateGroupWithReceipt(ctx context.Context, raw []byte) (CandidateGroupPublicationReceipt, error) {
	artifact, err := censuscontinuation.ParseCandidateGroup(raw)
	if err != nil {
		return CandidateGroupPublicationReceipt{}, errors.New("continuationhost: candidate group rejected")
	}
	artifactPath, _ := candidateGroupPath(candidateGroupArtifactNamespace, artifact.ID())
	if err := s.publishCandidateGroupObject(ctx, artifactPath, raw, censuscontinuation.CandidateGroupSchema); err != nil {
		return CandidateGroupPublicationReceipt{}, err
	}
	return CandidateGroupPublicationReceipt{ArtifactID: artifact.ID(), Selector: artifactPath, Digest: Digest(raw), ByteLength: uint64(len(raw))}, nil
}

func (s *Store) PutCandidateGroup(ctx context.Context, raw []byte) (string, error) {
	receipt, err := s.PutCandidateGroupWithReceipt(ctx, raw)
	return receipt.ArtifactID, err
}

func (s *Store) GetCandidateGroup(ctx context.Context, artifactID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := candidateGroupPath(candidateGroupArtifactNamespace, artifactID)
	if err != nil || s == nil || s.root == nil {
		return nil, errors.New("continuationhost: invalid candidate group identity")
	}
	raw, err := s.root.ReadSelector(path, s.maxBytes)
	if err != nil {
		return nil, errors.New("continuationhost: candidate group unavailable")
	}
	artifact, err := censuscontinuation.ParseCandidateGroup(raw)
	if err != nil || artifact.ID() != artifactID {
		return nil, errors.New("continuationhost: candidate group integrity mismatch")
	}
	return append([]byte(nil), raw...), nil
}
