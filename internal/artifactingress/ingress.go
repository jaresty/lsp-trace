// Package artifactingress admits already-produced immutable hydration artifacts.
// It performs no network, source, or language-server acquisition.
package artifactingress

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"strings"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
	"lsp-trace/internal/verification"
)

const (
	InlineMaxBytes        = 1 << 20
	HydrationCoreMaxBytes = 192 << 20
)

type FailureCode string

const (
	SelectorInvalid FailureCode = "SELECTOR_INVALID"
	CustodyFailed   FailureCode = "CUSTODY_FAILED"
	DigestMismatch  FailureCode = "DIGEST_MISMATCH"
	SchemaMismatch  FailureCode = "SCHEMA_MISMATCH"
	TooLarge        FailureCode = "TOO_LARGE"
	PathDisabled    FailureCode = "PATH_DISABLED"
	PathUnsafe      FailureCode = "PATH_UNSAFE"
	PathReplaced    FailureCode = "PATH_REPLACED"
	NotRegular      FailureCode = "NOT_REGULAR"
)

type Failure struct {
	Code  FailureCode
	cause error
}

func (e *Failure) Error() string               { return string(e.Code) }
func (e *Failure) Unwrap() error               { return e.cause }
func fail(code FailureCode, cause error) error { return &Failure{Code: code, cause: cause} }

// Validator performs the artifact family's exact schema and semantic validation.
type Validator func(schemaID string, artifact []byte) error

type Config struct {
	MaxBytes        int64
	PublicationRoot *publication.Root
	ArtifactStore   *publication.Root
	PrivatePaths    bool
	PrivateRoot     *os.Root
	Validate        Validator
}

type Evidence struct {
	Custody                 string
	Digest                  string
	SchemaID                string
	Generation              string
	SafeSelector            string
	PrivateDiagnosticsToken string
	ByteLength              uint64
}

type Result struct {
	Bytes    []byte
	Evidence Evidence
}

type Expected struct {
	SchemaID   string
	ByteLength uint64
	Generation string
}
type SelectorRequest struct {
	Selector string
	Receipt  publication.Receipt
	Expected Expected
}
type ContentRequest struct {
	ID       string
	Expected Expected
}
type PrivatePathRequest struct {
	Selector string
	Expected Expected
	Digest   string
}

// privateReadHook is a deterministic package-test seam for replacement races.
var privateReadHook func()

func (c Config) check() error {
	if c.MaxBytes <= InlineMaxBytes || c.MaxBytes > HydrationCoreMaxBytes || c.Validate == nil {
		return fail(TooLarge, errors.New("explicit large-artifact bound and validator required"))
	}
	return nil
}

func (c Config) Inline(raw []byte, schemaID, generation string) (Result, error) {
	if len(raw) > InlineMaxBytes {
		return Result{}, fail(TooLarge, errors.New("inline byte limit"))
	}
	if c.Validate == nil {
		return Result{}, fail(SchemaMismatch, errors.New("validator required"))
	}
	if err := c.Validate(schemaID, raw); err != nil {
		return Result{}, fail(SchemaMismatch, err)
	}
	return result(raw, "inline", schemaID, generation, "inline", ""), nil
}

func (c Config) FromSelector(req SelectorRequest) (Result, error) {
	if err := c.check(); err != nil {
		return Result{}, err
	}
	if c.PublicationRoot == nil || req.Selector == "" || req.Receipt.VerificationSelector != req.Selector || req.Receipt.PublicationMechanism != publication.VerifiedGenerationMechanism {
		return Result{}, fail(SelectorInvalid, errors.New("verified publication selector required"))
	}
	selectorRaw, err := c.PublicationRoot.ReadSelector(req.Selector, 4096)
	if err != nil {
		return Result{}, fail(CustodyFailed, err)
	}
	sel, err := verification.DecodeSelector(selectorRaw)
	if err != nil || sel.Generation != req.Receipt.Generation {
		return Result{}, fail(SelectorInvalid, err)
	}
	if req.Expected.SchemaID == "" || req.Receipt.ArtifactSchemaID != req.Expected.SchemaID {
		return Result{}, fail(SchemaMismatch, errors.New("publication schema binding mismatch"))
	}
	if req.Expected.Generation == "" || req.Receipt.Generation != req.Expected.Generation {
		return Result{}, fail(SelectorInvalid, errors.New("publication generation binding mismatch"))
	}
	if req.Expected.ByteLength != req.Receipt.ByteLength {
		return Result{}, fail(CustodyFailed, errors.New("publication length binding mismatch"))
	}
	if req.Receipt.ByteLength > uint64(c.MaxBytes) {
		return Result{}, fail(TooLarge, errors.New("artifact exceeds configured bound"))
	}
	artifact, err := c.PublicationRoot.ReadSelector(sel.Generation+"/artifact.json", c.MaxBytes)
	if err != nil {
		return Result{}, classifyRead(err)
	}
	receipt, err := c.PublicationRoot.ReadSelector(sel.Generation+"/receipt.json", InlineMaxBytes)
	if err != nil || verification.VerifyReceipt(artifact, receipt) != nil {
		return Result{}, fail(CustodyFailed, errors.New("exact-byte custody receipt failed"))
	}
	if uint64(len(artifact)) != req.Receipt.ByteLength {
		return Result{}, fail(CustodyFailed, errors.New("publication byte length mismatch"))
	}
	if byteDigest(artifact) != req.Receipt.Digest {
		return Result{}, fail(DigestMismatch, errors.New("publication digest mismatch"))
	}
	if expectedGeneration(artifact) != req.Receipt.Generation {
		return Result{}, fail(DigestMismatch, errors.New("generation/content mismatch"))
	}
	if req.Receipt.ArtifactSchemaID == "" {
		return Result{}, fail(SchemaMismatch, errors.New("artifact schema missing"))
	}
	if err := c.Validate(req.Receipt.ArtifactSchemaID, artifact); err != nil {
		return Result{}, fail(SchemaMismatch, err)
	}
	return result(artifact, publication.VerifiedGenerationMechanism, req.Receipt.ArtifactSchemaID, req.Receipt.Generation, req.Selector, ""), nil
}

func (c Config) FromContent(req ContentRequest) (Result, error) {
	if err := c.check(); err != nil {
		return Result{}, err
	}
	if c.ArtifactStore == nil || !canonicalDigest(req.ID) {
		return Result{}, fail(SelectorInvalid, errors.New("canonical sha256 content ID required"))
	}
	if req.Expected.ByteLength > uint64(c.MaxBytes) {
		return Result{}, fail(TooLarge, errors.New("artifact exceeds configured bound"))
	}
	hexID := strings.TrimPrefix(req.ID, "sha256:")
	artifact, err := c.ArtifactStore.ReadSelector(hexID, c.MaxBytes)
	if err != nil {
		return Result{}, classifyRead(err)
	}
	if byteDigest(artifact) != req.ID {
		return Result{}, fail(DigestMismatch, errors.New("content ID mismatch"))
	}
	if uint64(len(artifact)) != req.Expected.ByteLength {
		return Result{}, fail(CustodyFailed, errors.New("content length mismatch"))
	}
	if req.Expected.SchemaID == "" {
		return Result{}, fail(SchemaMismatch, errors.New("schema missing"))
	}
	if err := c.Validate(req.Expected.SchemaID, artifact); err != nil {
		return Result{}, fail(SchemaMismatch, err)
	}
	return result(artifact, "content_addressed_immutable_store", req.Expected.SchemaID, req.Expected.Generation, req.ID, ""), nil
}

func (c Config) FromPrivatePath(req PrivatePathRequest) (Result, error) {
	if !c.PrivatePaths || c.PrivateRoot == nil {
		return Result{}, fail(PathDisabled, errors.New("private path ingress disabled"))
	}
	if err := c.check(); err != nil {
		return Result{}, err
	}
	if !safeRelative(req.Selector) {
		return Result{}, fail(PathUnsafe, errors.New("unsafe relative selector"))
	}
	if req.Expected.ByteLength > uint64(c.MaxBytes) {
		return Result{}, fail(TooLarge, errors.New("artifact exceeds configured bound"))
	}
	before, err := lstatChain(c.PrivateRoot, req.Selector)
	if err != nil {
		return Result{}, err
	}
	artifact, err := source.ReadRegularInputBounded(c.PrivateRoot, req.Selector, c.MaxBytes)
	if err != nil {
		return Result{}, classifyPrivateRead(err)
	}
	if privateReadHook != nil {
		privateReadHook()
	}
	after, err := lstatChain(c.PrivateRoot, req.Selector)
	if err != nil || !os.SameFile(before, after) {
		return Result{}, fail(PathReplaced, errors.New("private artifact identity changed"))
	}
	rootInfo, err := c.PrivateRoot.Stat(".")
	if err != nil || ambiguousIdentity(rootInfo, after) {
		return Result{}, fail(CustodyFailed, errors.New("private artifact custody ambiguous"))
	}
	if uint64(len(artifact)) != req.Expected.ByteLength {
		return Result{}, fail(CustodyFailed, errors.New("private artifact length mismatch"))
	}
	if !canonicalDigest(req.Digest) || byteDigest(artifact) != req.Digest {
		return Result{}, fail(DigestMismatch, errors.New("private artifact digest mismatch"))
	}
	if req.Expected.SchemaID == "" {
		return Result{}, fail(SchemaMismatch, errors.New("schema missing"))
	}
	if err := c.Validate(req.Expected.SchemaID, artifact); err != nil {
		return Result{}, fail(SchemaMismatch, err)
	}
	tokenSum := sha256.Sum256([]byte(req.Selector + "\x00" + req.Digest))
	token := "private:" + hex.EncodeToString(tokenSum[:])
	return result(artifact, "configured_private_custody_root", req.Expected.SchemaID, req.Expected.Generation, req.Selector, token), nil
}

func result(raw []byte, custody, schemaID, generation, selector, token string) Result {
	owned := append([]byte(nil), raw...)
	return Result{Bytes: owned, Evidence: Evidence{Custody: custody, Digest: byteDigest(owned), SchemaID: schemaID, Generation: generation, SafeSelector: selector, PrivateDiagnosticsToken: token, ByteLength: uint64(len(owned))}}
}
func byteDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func expectedGeneration(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "g-" + hex.EncodeToString(sum[:])
}
func canonicalDigest(id string) bool {
	if len(id) != 71 || !strings.HasPrefix(id, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(id[7:])
	return err == nil && strings.ToLower(id) == id
}
func safeRelative(name string) bool {
	return name != "" && name != "." && name != ".." && !path.IsAbs(name) && path.Clean(name) == name && !strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\\\x00:")
}

func lstatChain(root *os.Root, name string) (os.FileInfo, error) {
	parts := strings.Split(name, "/")
	current := root
	var owned []*os.Root
	defer func() {
		for _, r := range owned {
			_ = r.Close()
		}
	}()
	for i, part := range parts {
		info, err := current.Lstat(part)
		if err != nil {
			return nil, fail(PathUnsafe, errors.New("private selector unavailable"))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fail(PathUnsafe, errors.New("symlink component rejected"))
		}
		if i == len(parts)-1 {
			if !info.Mode().IsRegular() {
				return nil, fail(NotRegular, errors.New("private selector is not regular"))
			}
			return info, nil
		}
		if !info.IsDir() {
			return nil, fail(PathUnsafe, errors.New("non-directory selector component"))
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			return nil, fail(PathUnsafe, errors.New("private selector component rejected"))
		}
		after, err := next.Stat(".")
		if err != nil || !os.SameFile(info, after) {
			next.Close()
			return nil, fail(PathReplaced, errors.New("private selector component changed"))
		}
		owned = append(owned, next)
		current = next
	}
	return nil, fail(PathUnsafe, errors.New("empty private selector"))
}
func classifyRead(err error) error {
	if strings.Contains(err.Error(), "regular") {
		return fail(NotRegular, err)
	}
	if strings.Contains(err.Error(), "limit") {
		return fail(TooLarge, err)
	}
	return fail(CustodyFailed, err)
}
func classifyPrivateRead(err error) error {
	if strings.Contains(err.Error(), "regular") {
		return fail(NotRegular, err)
	}
	if strings.Contains(err.Error(), "limit") {
		return fail(TooLarge, err)
	}
	return fail(PathReplaced, err)
}
