// Package sourceobject stores exact source bytes as private, immutable,
// content-addressed local objects. It has no graph, acquisition, or manifest
// context semantics.
package sourceobject

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	"lsp-trace/internal/publication"
)

const (
	// objectMagic is an internal storage-format marker, not caller context.
	objectMagic       = "lsp-trace-source-object-v1\x00"
	envelopeHashBytes = sha256.Size
	headerBytes       = len(objectMagic) + 8
)

const maxSourceBytes = int64(math.MaxInt64) - int64(headerBytes+envelopeHashBytes) - 1

type Code string

const (
	CodeInvalidIdentity Code = "INVALID_IDENTITY"
	CodeMissing         Code = "MISSING"
	CodeCorrupt         Code = "CORRUPT"
	CodePolicy          Code = "POLICY"
	CodePermission      Code = "PERMISSION"
	CodeLimit           Code = "LIMIT"
)

type Error struct {
	Code  Code
	cause error
}

func (e *Error) Error() string { return string(e.Code) }
func (e *Error) Unwrap() error { return e.cause }
func fail(code Code, cause error) error {
	return &Error{Code: code, cause: cause}
}

func IsCode(err error, code Code) bool {
	var typed *Error
	return errors.As(err, &typed) && typed.Code == code
}

type Identity struct {
	Digest     string
	ByteLength uint64
}

type Object struct {
	Identity Identity
	Bytes    []byte
}

type Store struct {
	root     *publication.Root
	maxBytes int64
}

// New binds the identity-only store to one already-opened private publication
// root. maxBytes bounds source bytes; its derived envelope bound must fit int64.
func New(root *publication.Root, maxBytes int64) (*Store, error) {
	if root == nil || maxBytes < 1 {
		return nil, fail(CodePolicy, errors.New("private root and positive source byte limit required"))
	}
	if maxBytes > maxSourceBytes {
		return nil, fail(CodeLimit, errors.New("source byte limit overflows storage envelope bound"))
	}
	if err := root.ValidatePrivate(); err != nil {
		return nil, classifyCustody(err)
	}
	return &Store{root: root, maxBytes: maxBytes}, nil
}

// Publish stores only exact bytes. Future manifest context is deliberately not
// accepted here, so identical bytes always identify and verify the same object.
func (s *Store) Publish(raw []byte) (Identity, error) {
	id := identity(raw)
	if err := s.checkIdentity(id); err != nil {
		return Identity{}, err
	}
	envelope := encode(raw)
	verify := func(stored []byte) error {
		got, err := decode(stored, s.maxBytes)
		if err != nil {
			return err
		}
		if got.Identity != id || !bytes.Equal(got.Bytes, raw) {
			return errors.New("source object exact verification failed")
		}
		return nil
	}
	receipt, err := publication.PublishBoundFile(s.root, selector(id), envelope, verify)
	if err == nil {
		if receipt == nil || receipt.VerificationStatus != "VERIFIED" {
			return Identity{}, fail(CodeCorrupt, errors.New("committed source object verification failed"))
		}
		return id, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return Identity{}, classifyCustody(err)
	}
	existing, getErr := s.Get(id)
	if getErr != nil {
		return Identity{}, getErr
	}
	if existing.Identity != id || !bytes.Equal(existing.Bytes, raw) {
		return Identity{}, fail(CodeCorrupt, errors.New("content identity already has different bytes"))
	}
	return id, nil
}

func (s *Store) Get(id Identity) (Object, error) {
	if err := s.checkIdentity(id); err != nil {
		return Object{}, err
	}
	raw, err := publication.ReadVerifiedBoundFile(s.root, selector(id), s.envelopeLimit())
	if err != nil {
		return Object{}, classifyStoredRead(err)
	}
	object, err := decode(raw, s.maxBytes)
	if err != nil {
		return Object{}, err
	}
	if object.Identity != id {
		return Object{}, fail(CodeCorrupt, errors.New("source object identity mismatch"))
	}
	return object, nil
}

func (s *Store) checkIdentity(id Identity) error {
	if s == nil || s.root == nil || s.maxBytes < 1 || s.maxBytes > maxSourceBytes {
		return fail(CodePolicy, errors.New("source object store is unavailable"))
	}
	if !canonicalDigest(id.Digest) {
		return fail(CodeInvalidIdentity, errors.New("canonical sha256 identity required"))
	}
	if id.ByteLength > uint64(s.maxBytes) {
		return fail(CodeLimit, errors.New("source object exceeds configured byte limit"))
	}
	if err := s.root.ValidatePrivate(); err != nil {
		return classifyCustody(err)
	}
	return nil
}

// envelopeLimit leaves room for ReadVerifiedBoundFile's one-byte lookahead
// because New rejects maxBytes > maxSourceBytes.
func (s *Store) envelopeLimit() int64 {
	return s.maxBytes + int64(headerBytes+envelopeHashBytes)
}

func identity(raw []byte) Identity {
	sum := sha256.Sum256(raw)
	return Identity{Digest: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: uint64(len(raw))}
}

func selector(id Identity) string { return strings.TrimPrefix(id.Digest, "sha256:") }

func canonicalDigest(digest string) bool {
	if len(digest) != 71 || !strings.HasPrefix(digest, "sha256:") || strings.ToLower(digest) != digest {
		return false
	}
	_, err := hex.DecodeString(digest[7:])
	return err == nil
}

func encode(raw []byte) []byte {
	out := make([]byte, 0, headerBytes+len(raw)+envelopeHashBytes)
	out = append(out, objectMagic...)
	var sourceLength [8]byte
	binary.BigEndian.PutUint64(sourceLength[:], uint64(len(raw)))
	out = append(out, sourceLength[:]...)
	out = append(out, raw...)
	sum := sha256.Sum256(out)
	return append(out, sum[:]...)
}

func decode(raw []byte, maxBytes int64) (Object, error) {
	if len(raw) < headerBytes+envelopeHashBytes || string(raw[:len(objectMagic)]) != objectMagic {
		return Object{}, fail(CodeCorrupt, errors.New("source object header invalid"))
	}
	sourceLength := binary.BigEndian.Uint64(raw[len(objectMagic):headerBytes])
	if sourceLength > uint64(maxBytes) {
		return Object{}, fail(CodeLimit, errors.New("source object exceeds configured byte limit"))
	}
	expected := uint64(headerBytes) + sourceLength + envelopeHashBytes
	if expected != uint64(len(raw)) {
		return Object{}, fail(CodeCorrupt, errors.New("source object envelope length mismatch"))
	}
	payloadEnd := len(raw) - envelopeHashBytes
	wantHash := sha256.Sum256(raw[:payloadEnd])
	if !bytes.Equal(raw[payloadEnd:], wantHash[:]) {
		return Object{}, fail(CodeCorrupt, errors.New("source object envelope digest mismatch"))
	}
	body := append([]byte(nil), raw[headerBytes:payloadEnd]...)
	return Object{Identity: identity(body), Bytes: body}, nil
}

func classifyStoredRead(err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fail(CodeMissing, err)
	case errors.Is(err, os.ErrPermission):
		return fail(CodePermission, err)
	case errors.Is(err, publication.ErrBoundFileLimit):
		return fail(CodeLimit, err)
	case errors.Is(err, publication.ErrBoundFilePolicy):
		return fail(CodePolicy, err)
	default:
		return fail(CodePolicy, fmt.Errorf("private source object read failed: %w", err))
	}
}

func classifyCustody(err error) error {
	if errors.Is(err, os.ErrPermission) {
		return fail(CodePermission, err)
	}
	return fail(CodePolicy, fmt.Errorf("private source object custody failed: %w", err))
}
