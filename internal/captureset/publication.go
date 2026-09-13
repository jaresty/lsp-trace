package captureset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"lsp-trace/internal/publication"
)

const (
	ConstituentSelectorPrefix = "graph-provenance-v5/sha256/"
	CaptureSetSelectorPrefix  = "capture-sets/v1/sha256/"
	RedactedValue             = "!redacted"
)

// ExactBytesAuthority is the sole authority in this package that assigns all
// constituent metadata. The injected admission must perform native V5
// structural and semantic checks and derive the native identity from the exact
// admitted bytes.
type ExactBytesAuthority struct {
	AdmitGraphProvenanceV5 func([]byte) (string, error)
}

func (a ExactBytesAuthority) Constituent(raw []byte) (Constituent, error) {
	if a.AdmitGraphProvenanceV5 == nil {
		return Constituent{}, errors.New("native V5 admission required")
	}
	nativeV5Identity, err := a.AdmitGraphProvenanceV5(raw)
	if err != nil {
		return Constituent{}, fmt.Errorf("admit native V5 constituent: %w", err)
	}
	if nativeV5Identity == "" {
		return Constituent{}, errors.New("native V5 admission returned empty identity")
	}
	sum := sha256.Sum256(raw)
	hexsum := hex.EncodeToString(sum[:])
	return Constituent{ImmutableSelector: ConstituentSelectorPrefix + hexsum, SchemaID: NativeV5SchemaID, SHA256: "sha256:" + hexsum, ByteLength: len(raw), NativeV5Identity: nativeV5Identity}, nil
}

func (a ExactBytesAuthority) VerifyConstituent(c Constituent, raw []byte) error {
	assigned, err := a.Constituent(raw)
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
	Selector         string `json:"selector"`
	Disclosure       string `json:"disclosure"`
	ArtifactSHA256   string `json:"artifact_sha256"`
	ByteLength       uint64 `json:"byte_length"`
	Mechanism        string `json:"mechanism"`
	NamespaceAtomic  bool   `json:"namespace_atomic,omitempty"`
	CrashDurability  string `json:"crash_durability,omitempty"`
	ConstituentCount int    `json:"constituent_count,omitempty"`
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

// PublishCaptureSet is the all-or-nothing private publication primitive. The
// supplied byte strings may be in any order; each must be exactly admitted and
// must bijectively match manifest.Constituents before staging begins.
func (p *Publisher) PublishCaptureSet(m Manifest, exactV5 [][]byte, authority ExactBytesAuthority) PublicationResult {
	manifestRaw, err := EncodeCanonical(m)
	if err != nil || m.Disclosure != "PRIVATE" {
		if err == nil {
			err = errors.New("only private capture sets may be published")
		}
		return PublicationResult{Err: err}
	}
	if len(exactV5) != len(m.Constituents) {
		return PublicationResult{Err: errors.New("constituent byte cardinality mismatch")}
	}
	bySelector := make(map[string][]byte, len(exactV5))
	for _, raw := range exactV5 {
		c, admitErr := authority.Constituent(raw)
		if admitErr != nil {
			return PublicationResult{Err: admitErr}
		}
		if _, duplicate := bySelector[c.ImmutableSelector]; duplicate {
			return PublicationResult{Err: errors.New("duplicate constituent bytes")}
		}
		bySelector[c.ImmutableSelector] = append([]byte(nil), raw...)
	}
	files := make([]publication.GenerationFile, 0, len(m.Constituents)+1)
	files = append(files, publication.GenerationFile{Name: "manifest.json", Bytes: manifestRaw})
	for _, c := range m.Constituents {
		raw, ok := bySelector[c.ImmutableSelector]
		if !ok {
			return PublicationResult{Err: errors.New("manifest constituent association mismatch")}
		}
		if verifyErr := authority.VerifyConstituent(c, raw); verifyErr != nil {
			return PublicationResult{Err: verifyErr}
		}
		files = append(files, publication.GenerationFile{Name: constituentFileName(c), Bytes: raw})
	}
	generationSelector := strings.TrimSuffix(CaptureSetPublicationSelector(m), "/manifest.json")
	receipt, err := publication.PublishGeneration(publication.GenerationRequest{Root: p.root, FinalSelector: generationSelector, Files: files})
	if err != nil {
		code := publication.CodePublicationFailed
		if errors.Is(err, os.ErrExist) {
			code = publication.CodeTargetExists
		}
		return PublicationResult{Code: code, Err: err}
	}
	sum := sha256.Sum256(manifestRaw)
	return PublicationResult{Receipt: &PublicationReceipt{
		Selector: CaptureSetPublicationSelector(m), Disclosure: "PRIVATE",
		ArtifactSHA256: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: uint64(len(manifestRaw)),
		Mechanism: receipt.Mechanism, NamespaceAtomic: receipt.NamespaceAtomic,
		CrashDurability: receipt.CrashDurability, ConstituentCount: len(m.Constituents),
	}}
}

func constituentFileName(c Constituent) string {
	return "constituents/" + strings.TrimPrefix(c.SHA256, "sha256:") + ".json"
}

// ResolveConstituent resolves bytes only through an already committed final
// capture-set selector and verifies that the requested constituent is associated.
func (p *Publisher) ResolveConstituent(finalSelector, immutableSelector string) ([]byte, error) {
	m, err := p.Verify(finalSelector)
	if err != nil {
		return nil, err
	}
	for _, c := range m.Constituents {
		if c.ImmutableSelector != immutableSelector {
			continue
		}
		generation := strings.TrimSuffix(finalSelector, "/manifest.json")
		raw, readErr := p.root.ReadSelector(generation+"/"+constituentFileName(c), int64(c.ByteLength)+1)
		if readErr != nil {
			return nil, readErr
		}
		if len(raw) != c.ByteLength || rawDigest(raw) != c.SHA256 {
			return nil, errors.New("constituent exact bytes mismatch")
		}
		return raw, nil
	}
	return nil, errors.New("constituent is not associated with capture set")
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

func ValidatePublicationSelector(selector string) error {
	if !strings.HasPrefix(selector, CaptureSetSelectorPrefix) || !strings.HasSuffix(selector, "/manifest.json") {
		return errors.New("non-canonical capture-set selector")
	}
	digest := strings.TrimSuffix(strings.TrimPrefix(selector, CaptureSetSelectorPrefix), "/manifest.json")
	if len(digest) != 64 || strings.ToLower(digest) != digest {
		return errors.New("non-canonical capture-set selector")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return errors.New("non-canonical capture-set selector")
	}
	return nil
}

func (p *Publisher) Verify(selector string) (Manifest, error) {
	if err := ValidatePublicationSelector(selector); err != nil {
		return Manifest{}, err
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
