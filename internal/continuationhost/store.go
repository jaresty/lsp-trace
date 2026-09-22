// Package continuationhost owns private host integration for census continuation.
package continuationhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

const (
	objectNamespace         = "continuations/objects/sha256"
	descriptorNamespace     = "continuations/catalogs"
	DescriptorSchemaVersion = "lsp-trace.census-continuation-descriptor.v1"
)

type Store struct {
	root            *publication.Root
	publisher       *publication.Publisher
	maxBytes        int64
	descriptorTrace func(context.Context, publication.TraceEvent)
}

func (s *Store) SetDescriptorTrace(trace func(context.Context, publication.TraceEvent)) {
	if s != nil {
		s.descriptorTrace = trace
	}
}

func (s *Store) traceDescriptor(ctx context.Context, stage, reason string, ok bool) {
	if s != nil && s.descriptorTrace != nil {
		s.descriptorTrace(ctx, publication.TraceEvent{Stage: stage, Reason: reason, OK: ok})
	}
}

type objectLimitError struct {
	limit, observed int64
}

func (e *objectLimitError) Error() string             { return "continuationhost: object rejected" }
func (e *objectLimitError) ResourceComponent() string { return "continuationhost.Store.Put" }
func (e *objectLimitError) ResourceField() string     { return "artifact_bytes" }
func (e *objectLimitError) ResourceLimit() int64      { return e.limit }
func (e *objectLimitError) ResourceObserved() int64   { return e.observed }
func (e *objectLimitError) ResourceCategory() string  { return "output" }

var _ censuscontinuation.Store = (*Store)(nil)

func NewStore(root *publication.Root, maxBytes int64) (*Store, error) {
	if root == nil || maxBytes < 1 || root.ValidatePrivate() != nil {
		return nil, errors.New("continuationhost: invalid store configuration")
	}
	return &Store{root: root, publisher: publication.NewPublisher(), maxBytes: maxBytes}, nil
}

func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func CanonicalSelector(raw []byte) string { return Digest(raw) + ":" + strconv.Itoa(len(raw)) }

func IsCanonicalSelector(selector string) bool {
	if isDigest(selector) {
		return true
	}
	_, _, err := parseSelector(selector)
	return err == nil
}

func IsPublicDescriptorSelector(selector string) bool {
	if len(selector) != 80 || !strings.HasPrefix(selector, "g-") || !strings.HasSuffix(selector, ".selector.json") {
		return false
	}
	hexDigest := strings.TrimSuffix(strings.TrimPrefix(selector, "g-"), ".selector.json")
	if len(hexDigest) != 64 || strings.ToLower(hexDigest) != hexDigest {
		return false
	}
	_, err := hex.DecodeString(hexDigest)
	return err == nil
}

func descriptorDigest(selector string) (string, error) {
	if !IsPublicDescriptorSelector(selector) {
		return "", errors.New("continuationhost: invalid descriptor selector")
	}
	return strings.TrimSuffix(strings.TrimPrefix(selector, "g-"), ".selector.json"), nil
}

func parseSelector(selector string) (string, int64, error) {
	parts := strings.Split(selector, ":")
	if len(parts) != 3 || parts[0] != "sha256" || len(parts[1]) != 64 || strings.ToLower(parts[1]) != parts[1] {
		return "", 0, errors.New("continuationhost: invalid selector")
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", 0, errors.New("continuationhost: invalid selector")
	}
	length, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || length < 0 || parts[2] != strconv.FormatInt(length, 10) {
		return "", 0, errors.New("continuationhost: invalid selector")
	}
	return parts[1], length, nil
}

func objectPath(hexDigest string) string { return objectNamespace + "/" + hexDigest }

func (s *Store) MaxObjectBytes() int64 {
	if s == nil {
		return 0
	}
	return s.maxBytes
}

func (s *Store) Put(ctx context.Context, raw []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s == nil || s.root == nil {
		return "", errors.New("continuationhost: object rejected")
	}
	if int64(len(raw)) > s.maxBytes {
		return "", &objectLimitError{limit: s.maxBytes, observed: int64(len(raw))}
	}
	copyBytes := append([]byte(nil), raw...)
	selector := Digest(copyBytes)
	hexDigest := strings.TrimPrefix(selector, "sha256:")
	result := s.publisher.Publish(publication.Request{Root: s.root, Selector: objectPath(hexDigest), Bytes: copyBytes, ArtifactSchemaID: "lsp-trace.census-continuation-object.v1"})
	if result.Failure != nil {
		if result.Failure.Code != publication.CodeTargetExists {
			return "", errors.New("continuationhost: publication failed")
		}
		existing, err := s.Get(ctx, selector)
		if err != nil || !bytes.Equal(existing, copyBytes) {
			return "", errors.New("continuationhost: immutable collision")
		}
		return selector, nil
	}
	if result.Receipt == nil || result.Receipt.Digest != Digest(copyBytes) || result.Receipt.ByteLength != uint64(len(copyBytes)) {
		return "", errors.New("continuationhost: publication verification failed")
	}
	return selector, nil
}

func (s *Store) Get(ctx context.Context, selector string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hexDigest, expectedLength, err := storeObjectIdentity(selector)
	if err != nil || s == nil || s.root == nil || expectedLength > s.maxBytes {
		return nil, errors.New("continuationhost: invalid selector")
	}
	raw, err := s.root.ReadSelector(objectPath(hexDigest), s.maxBytes)
	if err != nil {
		return nil, errors.New("continuationhost: object unavailable")
	}
	if (expectedLength >= 0 && int64(len(raw)) != expectedLength) || Digest(raw) != "sha256:"+hexDigest {
		return nil, errors.New("continuationhost: object integrity mismatch")
	}
	return append([]byte(nil), raw...), nil
}

func isDigest(selector string) bool {
	if len(selector) != 71 || !strings.HasPrefix(selector, "sha256:") {
		return false
	}
	hexDigest := strings.TrimPrefix(selector, "sha256:")
	_, err := hex.DecodeString(hexDigest)
	return err == nil && strings.ToLower(hexDigest) == hexDigest
}

func storeObjectIdentity(selector string) (string, int64, error) {
	if isDigest(selector) {
		return strings.TrimPrefix(selector, "sha256:"), -1, nil
	}
	return parseSelector(selector)
}

type DescriptorInput struct {
	CatalogSelector    string
	CheckpointSelector string
	CompositeSelector  string
}

type DescriptorPublicationCause string

const (
	DescriptorCauseCanonicalization     DescriptorPublicationCause = "CANONICALIZATION"
	DescriptorCauseEncoding             DescriptorPublicationCause = "ENCODING"
	DescriptorCauseByteCeiling          DescriptorPublicationCause = "BYTE_CEILING"
	DescriptorCauseSelectorConstruction DescriptorPublicationCause = "SELECTOR_CONSTRUCTION"
	DescriptorCauseRootIdentity         DescriptorPublicationCause = "ROOT_IDENTITY"
	DescriptorCauseCatalogNamespace     DescriptorPublicationCause = "CATALOG_NAMESPACE"
	DescriptorCausePermission           DescriptorPublicationCause = "PERMISSION"
	DescriptorCauseTemporaryCreate      DescriptorPublicationCause = "TEMPORARY_CREATE"
	DescriptorCauseStageWriteSync       DescriptorPublicationCause = "STAGE_WRITE_SYNC"
	DescriptorCauseAtomicInstall        DescriptorPublicationCause = "ATOMIC_INSTALL"
	DescriptorCausePublicationReread    DescriptorPublicationCause = "PUBLICATION_REREAD"
	DescriptorCauseExistingUnreadable   DescriptorPublicationCause = "EXISTING_UNREADABLE"
	DescriptorCauseImmutableCollision   DescriptorPublicationCause = "IMMUTABLE_COLLISION"
	DescriptorCauseReceiptVerification  DescriptorPublicationCause = "RECEIPT_VERIFICATION"
)

type DescriptorPublicationError struct{ cause DescriptorPublicationCause }

func (e *DescriptorPublicationError) Error() string {
	return "continuationhost: descriptor publication failed"
}
func (e *DescriptorPublicationError) Cause() DescriptorPublicationCause { return e.cause }

func descriptorFailure(cause DescriptorPublicationCause) error {
	return &DescriptorPublicationError{cause: cause}
}

func classifyPublicationFailure(failure *publication.Failure) DescriptorPublicationCause {
	if failure == nil {
		return DescriptorCauseReceiptVerification
	}
	if errors.Is(failure, os.ErrPermission) {
		return DescriptorCausePermission
	}
	if failure.Code == publication.CodeOutputSelectorUnsafe {
		cause := errors.Unwrap(failure)
		if cause != nil && strings.Contains(cause.Error(), "root identity changed") {
			return DescriptorCauseRootIdentity
		}
		if cause != nil && strings.Contains(cause.Error(), "selector component") {
			return DescriptorCauseCatalogNamespace
		}
		return DescriptorCauseSelectorConstruction
	}
	switch failure.Stage {
	case "stage":
		if !failure.Cleanup {
			return DescriptorCauseTemporaryCreate
		}
		return DescriptorCauseStageWriteSync
	case "install":
		return DescriptorCauseAtomicInstall
	case "reread":
		return DescriptorCausePublicationReread
	default:
		return DescriptorCauseAtomicInstall
	}
}

type descriptorWire struct {
	SchemaVersion      string `json:"schema_version"`
	CatalogSelector    string `json:"catalog_selector"`
	CheckpointSelector string `json:"checkpoint_selector"`
	CompositeSelector  string `json:"composite_selector"`
}

func (s *Store) PublishDescriptor(ctx context.Context, input DescriptorInput) (string, error) {
	if s != nil {
		s.traceDescriptor(ctx, "ENTER", "START", true)
	}
	if err := ctx.Err(); err != nil {
		if s != nil {
			s.traceDescriptor(ctx, "EXIT", "CONTEXT_DONE", false)
		}
		return "", err
	}
	if s == nil || s.root == nil || s.publisher == nil || !IsCanonicalSelector(input.CatalogSelector) || !IsCanonicalSelector(input.CheckpointSelector) || !IsCanonicalSelector(input.CompositeSelector) {
		if s != nil {
			s.traceDescriptor(ctx, "DESCRIPTOR_CANONICAL", "INVALID", false)
			s.traceDescriptor(ctx, "EXIT", "CANONICALIZATION_FAILED", false)
		}
		return "", descriptorFailure(DescriptorCauseCanonicalization)
	}
	s.traceDescriptor(ctx, "DESCRIPTOR_CANONICAL", "VALID", true)
	checkpointRaw, checkpointErr := s.Get(ctx, input.CheckpointSelector)
	if checkpointErr != nil {
		s.traceDescriptor(ctx, "CHECKPOINT_OBJECT", "ABSENT_OR_INVALID", false)
	} else if checkpoint, parseErr := censuscontinuation.ParseCheckpoint(checkpointRaw); parseErr != nil || checkpoint.Stage() == "" || checkpoint.Status() == "" {
		s.traceDescriptor(ctx, "CHECKPOINT_OBJECT", "EXISTS_INVALID", false)
	} else {
		s.traceDescriptor(ctx, "CHECKPOINT_OBJECT", "EXISTS_VALID", true)
	}
	raw, err := json.Marshal(descriptorWire{DescriptorSchemaVersion, input.CatalogSelector, input.CheckpointSelector, input.CompositeSelector})
	if err != nil {
		s.traceDescriptor(ctx, "DESCRIPTOR_VALIDATION", "ENCODING_FAILED", false)
		s.traceDescriptor(ctx, "EXIT", "ENCODING_FAILED", false)
		return "", descriptorFailure(DescriptorCauseEncoding)
	}
	s.traceDescriptor(ctx, "DESCRIPTOR_VALIDATION", "VALID", true)
	if int64(len(raw)) > s.maxBytes {
		s.traceDescriptor(ctx, "BYTE_CEILING", "EXCEEDED", false)
		s.traceDescriptor(ctx, "EXIT", "BYTE_CEILING", false)
		return "", descriptorFailure(DescriptorCauseByteCeiling)
	}
	s.traceDescriptor(ctx, "BYTE_CEILING", "WITHIN", true)
	sum := sha256.Sum256(raw)
	privateSelector := fmt.Sprintf("%s/%x.json", descriptorNamespace, sum[:])
	selector := fmt.Sprintf("g-%x.selector.json", sum[:])
	if !IsPublicDescriptorSelector(selector) {
		s.traceDescriptor(ctx, "SELECTOR_CONSTRUCTION", "INVALID", false)
		s.traceDescriptor(ctx, "EXIT", "SELECTOR_CONSTRUCTION_FAILED", false)
		return "", descriptorFailure(DescriptorCauseSelectorConstruction)
	}
	s.traceDescriptor(ctx, "SELECTOR_CONSTRUCTION", "VALID", true)
	var publicationTrace func(publication.TraceEvent)
	if s.descriptorTrace != nil {
		publicationTrace = func(event publication.TraceEvent) { s.descriptorTrace(ctx, event) }
	}
	result := s.publisher.Publish(publication.Request{Root: s.root, Selector: privateSelector, Bytes: raw, ArtifactSchemaID: DescriptorSchemaVersion, Trace: publicationTrace})
	if result.Failure != nil {
		if result.Failure.Code != publication.CodeTargetExists {
			return "", descriptorFailure(classifyPublicationFailure(result.Failure))
		}
		existing, readErr := s.root.ReadSelector(privateSelector, s.maxBytes)
		if readErr != nil {
			s.traceDescriptor(ctx, "TARGET_EQUAL", "UNREADABLE", false)
			s.traceDescriptor(ctx, "EXIT", "EXISTING_UNREADABLE", false)
			return "", descriptorFailure(DescriptorCauseExistingUnreadable)
		}
		if !bytes.Equal(existing, raw) {
			s.traceDescriptor(ctx, "TARGET_EQUAL", "NOT_EQUAL", false)
			s.traceDescriptor(ctx, "EXIT", "IMMUTABLE_COLLISION", false)
			return "", descriptorFailure(DescriptorCauseImmutableCollision)
		}
		s.traceDescriptor(ctx, "TARGET_EQUAL", "EQUAL", true)
		s.traceDescriptor(ctx, "EXIT", "IDEMPOTENT", true)
		return selector, nil
	}
	if result.Receipt == nil || result.Receipt.Digest != Digest(raw) || result.Receipt.ByteLength != uint64(len(raw)) || result.Receipt.ArtifactSchemaID != DescriptorSchemaVersion || result.Receipt.PublicationMechanism != publication.PublicationMechanism {
		s.traceDescriptor(ctx, "EXIT", "RECEIPT_INVALID", false)
		return "", descriptorFailure(DescriptorCauseReceiptVerification)
	}
	s.traceDescriptor(ctx, "EXIT", "PUBLISHED", true)
	return selector, nil
}

func (s *Store) ResolveDescriptor(ctx context.Context, selector string) (DescriptorInput, error) {
	if err := ctx.Err(); err != nil {
		return DescriptorInput{}, err
	}
	digest, err := descriptorDigest(selector)
	if err != nil || s == nil || s.root == nil {
		return DescriptorInput{}, errors.New("continuationhost: invalid descriptor selector")
	}
	raw, err := s.root.ReadSelector(descriptorNamespace+"/"+digest+".json", s.maxBytes)
	if err != nil {
		return DescriptorInput{}, errors.New("continuationhost: descriptor unavailable")
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != digest {
		return DescriptorInput{}, errors.New("continuationhost: descriptor integrity mismatch")
	}
	var wire descriptorWire
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&wire) != nil || wire.SchemaVersion != DescriptorSchemaVersion || !IsCanonicalSelector(wire.CatalogSelector) || !IsCanonicalSelector(wire.CheckpointSelector) || !IsCanonicalSelector(wire.CompositeSelector) {
		return DescriptorInput{}, errors.New("continuationhost: invalid descriptor")
	}
	resolved := DescriptorInput{CatalogSelector: wire.CatalogSelector, CheckpointSelector: wire.CheckpointSelector, CompositeSelector: wire.CompositeSelector}
	if err := s.validateDescriptorTargets(ctx, resolved); err != nil {
		return DescriptorInput{}, err
	}
	return resolved, nil
}

func (s *Store) validateDescriptorTargets(ctx context.Context, descriptor DescriptorInput) error {
	if descriptor.CatalogSelector == descriptor.CheckpointSelector && descriptor.CheckpointSelector == descriptor.CompositeSelector {
		checkpointRaw, err := s.Get(ctx, descriptor.CheckpointSelector)
		if err != nil {
			return errors.New("continuationhost: descriptor checkpoint unavailable")
		}
		checkpoint, err := censuscontinuation.ParseCheckpoint(checkpointRaw)
		if err != nil || checkpoint.Stage() != censuscontinuation.StageRequestsRendered || checkpoint.Status() != censuscontinuation.StatusRunning {
			return errors.New("continuationhost: stop checkpoint mismatch")
		}
		return nil
	}
	if descriptor.CatalogSelector == descriptor.CheckpointSelector || descriptor.CatalogSelector == descriptor.CompositeSelector || descriptor.CheckpointSelector == descriptor.CompositeSelector {
		return errors.New("continuationhost: descriptor target identity mismatch")
	}
	catalogRaw, err := s.Get(ctx, descriptor.CatalogSelector)
	if err != nil {
		return errors.New("continuationhost: descriptor catalog unavailable")
	}
	catalog, err := provisionalfeaturecatalog.Parse(catalogRaw)
	if err != nil {
		return errors.New("continuationhost: descriptor catalog invalid")
	}
	checkpointRaw, err := s.Get(ctx, descriptor.CheckpointSelector)
	if err != nil {
		return errors.New("continuationhost: descriptor checkpoint unavailable")
	}
	checkpoint, err := censuscontinuation.ParseCheckpoint(checkpointRaw)
	if err != nil || checkpoint.Stage() != censuscontinuation.StageCatalogCommitted || checkpoint.Status() != censuscontinuation.StatusComplete {
		return errors.New("continuationhost: descriptor checkpoint invalid")
	}
	compositeRaw, err := s.Get(ctx, descriptor.CompositeSelector)
	if err != nil {
		return errors.New("continuationhost: descriptor composite unavailable")
	}
	composite, err := censuscontinuation.ParseComposite(compositeRaw)
	if err != nil || composite.CatalogID() != catalog.ID() {
		return errors.New("continuationhost: descriptor composite invalid")
	}
	var catalogID, compositeID string
	for _, artifact := range checkpoint.Artifacts() {
		switch artifact.Kind {
		case "catalog":
			catalogID = artifact.ID
		case "composite":
			compositeID = artifact.ID
		}
	}
	if catalogID != descriptor.CatalogSelector || compositeID != descriptor.CompositeSelector {
		return errors.New("continuationhost: descriptor target identity mismatch")
	}
	return nil
}
