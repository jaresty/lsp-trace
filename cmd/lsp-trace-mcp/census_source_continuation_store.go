package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

const censusSourceContinuationMaxBytes = 4 << 20

type censusSourceContinuationObjects struct {
	root      *publication.Root
	publisher *publication.Publisher
}

func newCensusSourceContinuationObjects(root *publication.Root) (*censusSourceContinuationObjects, error) {
	if root == nil || root.ValidatePrivate() != nil {
		return nil, errors.New("invalid private census source-continuation object root")
	}
	return &censusSourceContinuationObjects{root: root, publisher: publication.NewPublisher()}, nil
}

func censusSourceContinuationDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (s *censusSourceContinuationObjects) Put(ctx context.Context, raw []byte) (string, error) {
	if s == nil || s.root == nil || s.publisher == nil || ctx == nil || len(raw) == 0 || len(raw) > censusSourceContinuationMaxBytes {
		return "", errors.New("invalid private census source-continuation object")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selector := censusSourceContinuationDigest(raw)
	result := s.publisher.Publish(publication.Request{Root: s.root, Selector: selector, Bytes: bytes.Clone(raw), ArtifactSchemaID: "lsp-trace.census-source-continuation-object.private.v1"})
	if result.Failure != nil {
		if result.Failure.Code != publication.CodeTargetExists {
			return "", result.Failure
		}
		existing, err := s.Get(ctx, selector)
		if err != nil || !bytes.Equal(existing, raw) {
			return "", errors.Join(errors.New("private census source-continuation object collision"), err)
		}
		return selector, nil
	}
	if result.Receipt == nil || result.Receipt.Digest != selector || result.Receipt.ByteLength != uint64(len(raw)) {
		return "", errors.New("private census source-continuation publication receipt mismatch")
	}
	return selector, nil
}

func (s *censusSourceContinuationObjects) Get(ctx context.Context, selector string) ([]byte, error) {
	if s == nil || s.root == nil || ctx == nil {
		return nil, errors.New("invalid private census source-continuation object store")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := s.root.ReadSelector(selector, censusSourceContinuationMaxBytes)
	if err != nil {
		return nil, err
	}
	if censusSourceContinuationDigest(raw) != selector {
		return nil, errors.New("private census source-continuation object digest mismatch")
	}
	return bytes.Clone(raw), nil
}

type censusSourceContinuationHeads struct{ root *publication.Root }

func newCensusSourceContinuationHeads(root *publication.Root) (*censusSourceContinuationHeads, error) {
	if root == nil || root.ValidatePrivate() != nil {
		return nil, errors.New("invalid private census source-continuation head root")
	}
	return &censusSourceContinuationHeads{root: root}, nil
}

func censusSourceContinuationHeadSelector(key string) (string, error) {
	if key == "" {
		return "", errors.New("private census source-continuation head key required")
	}
	sum := sha256.Sum256([]byte(key))
	return "census-source-continuation/heads/v1/" + hex.EncodeToString(sum[:]) + ".json", nil
}

func (s *censusSourceContinuationHeads) Load(ctx context.Context, key string) ([]byte, error) {
	if s == nil || s.root == nil || ctx == nil {
		return nil, errors.New("invalid private census source-continuation head store")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	selector, err := censusSourceContinuationHeadSelector(key)
	if err != nil {
		return nil, err
	}
	raw, err := s.root.ReadSelectorWithAbsence(selector, censusSourceContinuationMaxBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := censussourcecontinuation.ParseHeadRevision(raw); err != nil {
		return nil, err
	}
	return bytes.Clone(raw), nil
}

func (s *censusSourceContinuationHeads) CompareAndSwap(ctx context.Context, key string, expected censussourcecontinuation.HeadRevision, replacement []byte) censussourcecontinuation.CASResult {
	if s == nil || s.root == nil || ctx == nil || len(replacement) == 0 || len(replacement) > censusSourceContinuationMaxBytes {
		return censussourcecontinuation.CASResult{Err: errors.New("invalid private census source-continuation head replacement")}
	}
	if _, err := censussourcecontinuation.ParseHeadRevision(replacement); err != nil {
		return censussourcecontinuation.CASResult{Err: fmt.Errorf("invalid private census source-continuation head replacement: %w", err)}
	}
	selector, err := censusSourceContinuationHeadSelector(key)
	if err != nil {
		return censussourcecontinuation.CASResult{Err: err}
	}
	predecessor := publication.BoundFilePredecessor{Absent: true}
	current, loadErr := s.root.ReadSelectorWithAbsence(selector, censusSourceContinuationMaxBytes)
	if loadErr == nil {
		revision, parseErr := censussourcecontinuation.ParseHeadRevision(current)
		if parseErr != nil {
			return censussourcecontinuation.CASResult{Err: parseErr}
		}
		if revision != expected {
			return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASNotCommitted}
		}
		predecessor = publication.BoundFilePredecessor{Selector: selector, Digest: publication.DigestForTest(current), ByteLength: uint64(len(current))}
	} else if !errors.Is(loadErr, os.ErrNotExist) {
		return censussourcecontinuation.CASResult{Err: loadErr}
	} else if expected != (censussourcecontinuation.HeadRevision{}) {
		return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASNotCommitted}
	}
	receipt, err := publication.CompareAndReplaceBoundFile(ctx, s.root, selector, predecessor, bytes.Clone(replacement), func(raw []byte) error {
		_, verifyErr := censussourcecontinuation.ParseHeadRevision(raw)
		return verifyErr
	})
	if errors.Is(err, publication.ErrStalePredecessor) {
		return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASNotCommitted}
	}
	if receipt != nil && receipt.Committed && err != nil {
		return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitUncertain, Err: err}
	}
	if err != nil {
		return censussourcecontinuation.CASResult{Err: err}
	}
	return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitted}
}
