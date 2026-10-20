package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/publication"
)

type privateCensusCoordinatorStore struct {
	root      *publication.Root
	publisher *publication.Publisher
}

func newPrivateCensusCoordinatorStore(root *publication.Root) (*privateCensusCoordinatorStore, error) {
	if root == nil || root.ValidatePrivate() != nil {
		return nil, errors.New("invalid private census coordinator store root")
	}
	return &privateCensusCoordinatorStore{root: root, publisher: publication.NewPublisher()}, nil
}

func privateCensusObjectSelector(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validPrivateCensusObjectSelector(selector string) bool {
	if len(selector) != len("sha256:")+sha256.Size*2 || selector[:len("sha256:")] != "sha256:" {
		return false
	}
	decoded, err := hex.DecodeString(selector[len("sha256:"):])
	return err == nil && len(decoded) == sha256.Size && selector == "sha256:"+hex.EncodeToString(decoded)
}

func (s *privateCensusCoordinatorStore) MaxObjectBytes() int64 {
	return censuschunking.MaxCanonicalBytes
}

func (s *privateCensusCoordinatorStore) Put(ctx context.Context, raw []byte) (string, error) {
	if s == nil || s.root == nil || s.publisher == nil || ctx == nil {
		return "", errors.New("invalid private census coordinator store")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(raw) == 0 || int64(len(raw)) > censuschunking.MaxCanonicalBytes {
		return "", errors.New("private census coordinator object limit exceeded")
	}
	selector := privateCensusObjectSelector(raw)
	result := s.publisher.Publish(publication.Request{Root: s.root, Selector: selector, Bytes: bytes.Clone(raw), ArtifactSchemaID: "lsp-trace.census-chunk-coordinator-object.private"})
	if result.Failure != nil {
		if result.Failure.Code != publication.CodeTargetExists {
			return "", result.Failure
		}
		existing, err := s.Get(ctx, selector)
		if err != nil || !bytes.Equal(existing, raw) {
			return "", errors.Join(errors.New("private census coordinator object collision"), err)
		}
		return selector, nil
	}
	if result.Receipt == nil || result.Receipt.Digest != selector || result.Receipt.ByteLength != uint64(len(raw)) {
		return "", errors.New("private census coordinator publication receipt mismatch")
	}
	readback, err := s.Get(ctx, selector)
	if err != nil || !bytes.Equal(readback, raw) {
		return "", errors.Join(errors.New("private census coordinator publication readback mismatch"), err)
	}
	return selector, nil
}

func (s *privateCensusCoordinatorStore) Get(ctx context.Context, selector string) ([]byte, error) {
	if s == nil || s.root == nil || ctx == nil || !validPrivateCensusObjectSelector(selector) {
		return nil, errors.New("invalid private census coordinator object selector")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := s.root.ReadSelector(selector, censuschunking.MaxCanonicalBytes)
	if err != nil {
		return nil, err
	}
	if privateCensusObjectSelector(raw) != selector {
		return nil, errors.New("private census coordinator object digest mismatch")
	}
	return bytes.Clone(raw), nil
}

type privateCensusHeadStore struct{ root *publication.Root }

func newPrivateCensusHeadStore(root *publication.Root) (*privateCensusHeadStore, error) {
	if root == nil || root.ValidatePrivate() != nil {
		return nil, errors.New("invalid private census head store root")
	}
	return &privateCensusHeadStore{root: root}, nil
}

func privateCensusHeadSelector(key string) (string, error) {
	if key == "" {
		return "", errors.New("private census head key required")
	}
	sum := sha256.Sum256([]byte(key))
	return "census-chunk-head-registers/v1/" + hex.EncodeToString(sum[:]) + ".json", nil
}

func (s *privateCensusHeadStore) Load(ctx context.Context, key string) ([]byte, error) {
	if s == nil || s.root == nil || ctx == nil {
		return nil, errors.New("invalid private census head store")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	selector, err := privateCensusHeadSelector(key)
	if err != nil {
		return nil, err
	}
	raw, err := s.root.ReadSelector(selector, censuschunking.MaxCanonicalBytes)
	if err != nil {
		return nil, err
	}
	if _, err := censuschunking.ParseCoordinatorHeadRevision(raw); err != nil {
		return nil, err
	}
	return bytes.Clone(raw), nil
}

func (s *privateCensusHeadStore) CompareAndSwap(ctx context.Context, key string, expected censuschunking.HeadRevision, replacement []byte) error {
	if s == nil || s.root == nil || ctx == nil {
		return errors.New("invalid private census head store")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(replacement) == 0 || int64(len(replacement)) > censuschunking.MaxCanonicalBytes {
		return errors.New("private census head replacement limit exceeded")
	}
	if _, err := censuschunking.ParseCoordinatorHeadRevision(replacement); err != nil {
		return fmt.Errorf("invalid private census head replacement: %w", err)
	}
	selector, err := privateCensusHeadSelector(key)
	if err != nil {
		return err
	}
	predecessor := publication.BoundFilePredecessor{Absent: true}
	current, loadErr := s.root.ReadSelectorWithAbsence(selector, censuschunking.MaxCanonicalBytes)
	if loadErr == nil {
		revision, parseErr := censuschunking.ParseCoordinatorHeadRevision(current)
		if parseErr != nil {
			return fmt.Errorf("invalid current private census head: %w", parseErr)
		}
		if revision != expected {
			return publication.ErrStalePredecessor
		}
		predecessor = publication.BoundFilePredecessor{Selector: selector, Digest: publication.DigestForTest(current), ByteLength: uint64(len(current))}
	} else if !errors.Is(loadErr, os.ErrNotExist) {
		return fmt.Errorf("read current private census head: %w", loadErr)
	} else if expected != (censuschunking.HeadRevision{}) {
		return publication.ErrStalePredecessor
	}
	receipt, err := publication.CompareAndReplaceBoundFile(ctx, s.root, selector, predecessor, bytes.Clone(replacement), func(raw []byte) error {
		_, verifyErr := censuschunking.ParseCoordinatorHeadRevision(raw)
		return verifyErr
	})
	if err != nil {
		return err
	}
	if receipt == nil || !receipt.Committed || receipt.Outcome != publication.CASOutcomeCommitted {
		return errors.New("private census head replacement not committed")
	}
	return nil
}

type privateHeldCensusSession struct {
	expected censuschunking.ExpectedSession
	core     censusacquisition.Core
}

func newPrivateHeldCensusSession(expected censuschunking.ExpectedSession, core censusacquisition.Core) (*privateHeldCensusSession, error) {
	if strings.TrimSpace(expected.SessionID) == "" || expected.Generation == 0 || !canonicalPrivateWorkspaceURI(expected.WorkspaceURI) {
		return nil, errors.New("invalid held census session identity")
	}
	if core.Discoverer == nil || core.Acquirer == nil {
		return nil, errors.New("invalid held census session core")
	}
	return &privateHeldCensusSession{expected: expected, core: core}, nil
}

func canonicalPrivateWorkspaceURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" || u.Path == "" || !strings.HasPrefix(u.Path, "/") || path.Clean(u.Path) != u.Path {
		return false
	}
	canonical := (&url.URL{Scheme: "file", Path: u.Path}).String()
	return canonical == raw
}

func (s *privateHeldCensusSession) WithReadySession(ctx context.Context, expected censuschunking.ExpectedSession, callback func(censusacquisition.Core) error) error {
	if s == nil || ctx == nil || callback == nil {
		return errors.New("invalid held census session")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if expected != s.expected {
		return errors.New("held census session identity mismatch")
	}
	return callback(s.core)
}
