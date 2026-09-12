package captureset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"lsp-trace/internal/publication"
)

const (
	ConstituentSelectorPrefix = "graph-provenance-v5/sha256/"
	CaptureSetSelectorPrefix  = "capture-sets/v1/sha256/"
	RedactedValue             = "!redacted"
)

// ExactBytesAuthority is the sole authority in this package that assigns a
// constituent selector. The injected validator must perform the native V5
// structural and semantic checks; exact-byte identity is assigned only after it
// succeeds.
type ExactBytesAuthority struct {
	VerifyGraphProvenanceV5 func([]byte) error
}

func (a ExactBytesAuthority) Constituent(raw []byte, nativeV5Identity string) (Constituent, error) {
	if a.VerifyGraphProvenanceV5 == nil {
		return Constituent{}, errors.New("native V5 verifier required")
	}
	if err := a.VerifyGraphProvenanceV5(raw); err != nil {
		return Constituent{}, fmt.Errorf("verify native V5 constituent: %w", err)
	}
	if nativeV5Identity == "" {
		return Constituent{}, errors.New("native V5 identity required")
	}
	sum := sha256.Sum256(raw)
	hexsum := hex.EncodeToString(sum[:])
	return Constituent{ImmutableSelector: ConstituentSelectorPrefix + hexsum, SchemaID: NativeV5SchemaID, SHA256: "sha256:" + hexsum, ByteLength: len(raw), NativeV5Identity: nativeV5Identity}, nil
}

func (a ExactBytesAuthority) VerifyConstituent(c Constituent, raw []byte) error {
	assigned, err := a.Constituent(raw, c.NativeV5Identity)
	if err != nil {
		return err
	}
	if c != assigned {
		return errors.New("constituent metadata was not assigned by exact-byte authority")
	}
	return nil
}

// CaptureSetPublicationSelector maps a manifest identity into the sole v1
// publication namespace. It is a selector beneath a pinned publication.Root,
// never an ambient filesystem path.
func CaptureSetPublicationSelector(m Manifest) string {
	return CaptureSetSelectorPrefix + strings.TrimPrefix(m.LogicalDigest, "sha256:") + "/manifest.json"
}

// Redact applies the complete v1 field policy. It redacts resource identities,
// canonical seed material, and constituent locator/native identity. Digests,
// accounting, policies, batches, and claim ceilings remain unchanged. The
// redacted view is then assigned a new logical identity.
func Redact(m Manifest) (Manifest, error) {
	if err := validateContent(m, true); err != nil {
		return Manifest{}, err
	}
	r := m
	r.Disclosure = "REDACTED"
	r.FileLedger = cloneLedger(m.FileLedger)
	r.SymbolLedger = cloneLedger(m.SymbolLedger)
	for i := range r.FileLedger.Entries {
		r.FileLedger.Entries[i].Identity = fmt.Sprintf("%s:%08d", RedactedValue, i)
	}
	for i := range r.SymbolLedger.Entries {
		r.SymbolLedger.Entries[i].Identity = fmt.Sprintf("%s:%08d", RedactedValue, i)
	}
	r.Targets = append([]Target(nil), m.Targets...)
	for i := range r.Targets {
		value := fmt.Sprintf("%s:%08d", RedactedValue, i)
		r.Targets[i].CanonicalSeedV2 = value
		r.Targets[i].CanonicalSeedV2SHA256 = rawDigest([]byte(value))
	}
	r.Constituents = append([]Constituent(nil), m.Constituents...)
	for i := range r.Constituents {
		value := fmt.Sprintf("%s:%08d", RedactedValue, i)
		r.Constituents[i].ImmutableSelector = value
		r.Constituents[i].NativeV5Identity = value
	}
	r.LogicalDigest, r.ImmutableSelector = "", ""
	setIdentity(&r)
	if r.LogicalDigest == m.LogicalDigest || r.ImmutableSelector == m.ImmutableSelector {
		return Manifest{}, errors.New("redacted identity collision")
	}
	if err := validateContent(r, true); err != nil {
		return Manifest{}, err
	}
	return r, nil
}

type PublicationReceipt struct {
	Selector       string `json:"selector"`
	Disclosure     string `json:"disclosure"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	ByteLength     uint64 `json:"byte_length"`
	Mechanism      string `json:"mechanism"`
}

type PublicationResult struct {
	Receipt *PublicationReceipt
	Code    string
	Err     error
}

type Publisher struct {
	root      *publication.Root
	publisher *publication.Publisher
}

func NewPublisher(root *publication.Root) *Publisher {
	return &Publisher{root: root, publisher: publication.NewPublisher()}
}

func (p *Publisher) Publish(m Manifest) PublicationResult {
	raw, err := EncodeCanonical(m)
	if err != nil {
		return PublicationResult{Err: err}
	}
	selector := CaptureSetPublicationSelector(m)
	result := p.publisher.Publish(publication.Request{Root: p.root, Selector: selector, Bytes: raw, ArtifactSchemaID: Version})
	if err := result.Err(); err != nil {
		return PublicationResult{Code: result.Failure.Code, Err: err}
	}
	return PublicationResult{Receipt: &PublicationReceipt{Selector: selector, Disclosure: "PRIVATE", ArtifactSHA256: result.Receipt.Digest, ByteLength: result.Receipt.ByteLength, Mechanism: result.Receipt.PublicationMechanism}}
}

func (p *Publisher) Verify(selector string) (Manifest, error) {
	if !strings.HasPrefix(selector, CaptureSetSelectorPrefix) || !strings.HasSuffix(selector, "/manifest.json") {
		return Manifest{}, errors.New("non-canonical capture-set selector")
	}
	raw, err := p.root.ReadSelector(selector, 32<<20)
	if err != nil {
		return Manifest{}, err
	}
	m, err := Decode(raw)
	if err != nil {
		return Manifest{}, err
	}
	if CaptureSetPublicationSelector(m) != selector {
		return Manifest{}, errors.New("capture-set selector identity mismatch")
	}
	canonical, _ := EncodeCanonical(m)
	if !bytes.Equal(raw, canonical) {
		return Manifest{}, errors.New("capture-set exact bytes mismatch")
	}
	return m, nil
}
