package censussourceassembly

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

const MaxBytes = 4 << 20

const objectSchemaID = "lsp-trace.census-source-continuation-object.private.v1"

type Objects struct {
	root      *publication.Root
	publisher *publication.Publisher
}

func NewObjects(root *publication.Root) (*Objects, error) {
	if root == nil || root.ValidatePrivate() != nil {
		return nil, errors.New("invalid private census source-continuation object root")
	}
	return &Objects{root: root, publisher: publication.NewPublisher()}, nil
}

func ObjectSelector(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (s *Objects) Put(ctx context.Context, raw []byte) (string, error) {
	if s == nil || s.root == nil || s.publisher == nil || ctx == nil || len(raw) == 0 || len(raw) > MaxBytes {
		return "", errors.New("invalid private census source-continuation object")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selector := ObjectSelector(raw)
	result := s.publisher.Publish(publication.Request{Root: s.root, Selector: selector, Bytes: bytes.Clone(raw), ArtifactSchemaID: objectSchemaID})
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

func (s *Objects) Get(ctx context.Context, selector string) ([]byte, error) {
	if s == nil || s.root == nil || ctx == nil {
		return nil, errors.New("invalid private census source-continuation object store")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := s.root.ReadSelector(selector, MaxBytes)
	if err != nil {
		return nil, err
	}
	if ObjectSelector(raw) != selector {
		return nil, errors.New("private census source-continuation object digest mismatch")
	}
	return bytes.Clone(raw), nil
}

type compareAndReplaceFunc func(context.Context, *publication.Root, string, publication.BoundFilePredecessor, []byte, func([]byte) error) (*publication.CompareAndReplaceReceipt, error)

type Heads struct {
	root              *publication.Root
	compareAndReplace compareAndReplaceFunc
}

func NewHeads(root *publication.Root) (*Heads, error) {
	if root == nil || root.ValidatePrivate() != nil {
		return nil, errors.New("invalid private census source-continuation head root")
	}
	return &Heads{root: root, compareAndReplace: publication.CompareAndReplaceBoundFile}, nil
}

func HeadSelector(key string) (string, error) {
	if key == "" {
		return "", errors.New("private census source-continuation head key required")
	}
	sum := sha256.Sum256([]byte(key))
	return "census-source-continuation/heads/v1/" + hex.EncodeToString(sum[:]) + ".json", nil
}

func (s *Heads) Load(ctx context.Context, key string) ([]byte, error) {
	if s == nil || s.root == nil || ctx == nil {
		return nil, errors.New("invalid private census source-continuation head store")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	selector, err := HeadSelector(key)
	if err != nil {
		return nil, err
	}
	raw, err := s.root.ReadSelectorWithAbsence(selector, MaxBytes)
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

func (s *Heads) CompareAndSwap(ctx context.Context, key string, expected censussourcecontinuation.HeadRevision, replacement []byte) censussourcecontinuation.CASResult {
	if s == nil || s.root == nil || s.compareAndReplace == nil || ctx == nil || len(replacement) == 0 || len(replacement) > MaxBytes {
		return censussourcecontinuation.CASResult{Err: errors.New("invalid private census source-continuation head replacement")}
	}
	if _, err := censussourcecontinuation.ParseHeadRevision(replacement); err != nil {
		return censussourcecontinuation.CASResult{Err: fmt.Errorf("invalid private census source-continuation head replacement: %w", err)}
	}
	selector, err := HeadSelector(key)
	if err != nil {
		return censussourcecontinuation.CASResult{Err: err}
	}
	predecessor := publication.BoundFilePredecessor{Absent: true}
	current, loadErr := s.root.ReadSelectorWithAbsence(selector, MaxBytes)
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
	receipt, err := s.compareAndReplace(ctx, s.root, selector, predecessor, bytes.Clone(replacement), func(raw []byte) error {
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
