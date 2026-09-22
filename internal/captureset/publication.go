package captureset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"lsp-trace/internal/graphprovenance"
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

// NativeV5Authority performs the repository's full native V5 structural and
// semantic admission and derives identity from those exact admitted bytes.
func NativeV5Authority() ExactBytesAuthority {
	return ExactBytesAuthority{AdmitGraphProvenanceV5: func(raw []byte) (string, error) {
		version, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v5")
		if err != nil || version != graphprovenance.VersionV5 {
			if err == nil {
				err = errors.New("not native Graph Provenance V5")
			}
			return "", err
		}
		sum := sha256.Sum256(raw)
		return graphprovenance.VersionV5 + "/sha256/" + hex.EncodeToString(sum[:]), nil
	}}
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
	return CaptureSetSelectorPrefix + strings.TrimPrefix(m.LogicalDigest, "sha256:") + ".bundle"
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
	Selector            string `json:"selector"`
	Disclosure          string `json:"disclosure"`
	ArtifactSHA256      string `json:"artifact_sha256"`
	ByteLength          uint64 `json:"byte_length"`
	Mechanism           string `json:"mechanism"`
	NamespaceAtomic     bool   `json:"namespace_atomic,omitempty"`
	CrashDurability     string `json:"crash_durability,omitempty"`
	DirectorySyncStatus string `json:"directory_sync_status"`
	CloseStatus         string `json:"close_status"`
	ConstituentCount    int    `json:"constituent_count,omitempty"`
	VerificationStatus  string `json:"verification_status"`
}

type PublicationFailure struct {
	Stage               string
	Reason              string
	CandidateSHA256     string
	CandidateByteLength uint64
	CodecCategory       string
	CodecLimit          uint64
	RootSource          string
	TargetExists        *bool
	TargetEqual         *bool
	TempBytesCommitted  bool
	FinalBytesCommitted bool
	Err                 error
}

func (f *PublicationFailure) Error() string {
	if f == nil || f.Err == nil {
		return "capture-set publication failed"
	}
	return "capture-set publication failed: " + f.Err.Error()
}

func (f *PublicationFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.Err
}

type PublicationResult struct {
	Receipt       *PublicationReceipt
	Code          string
	Failure       *PublicationFailure
	LedgerFailure *FailureLedgerWriteFailure
	Err           error
}

type Publisher struct {
	root   *publication.Root
	trace  publication.BoundFileTrace
	ledger *FailureLedger
}

func NewPublisher(root *publication.Root) *Publisher {
	return &Publisher{root: root}
}

func NewPublisherWithTrace(root *publication.Root, trace publication.BoundFileTrace) *Publisher {
	return &Publisher{root: root, trace: trace}
}

func NewPublisherWithFailureLedger(root *publication.Root, ledger *FailureLedger) *Publisher {
	return &Publisher{root: root, ledger: ledger}
}

func NewPublisherWithTraceAndFailureLedger(root *publication.Root, trace publication.BoundFileTrace, ledger *FailureLedger) *Publisher {
	return &Publisher{root: root, trace: trace, ledger: ledger}
}

func FailedFailureLedger(err error) *FailureLedger {
	if err == nil {
		return nil
	}
	return &FailureLedger{write: func(FailureLedgerRecord) error { return err }}
}

func (p *Publisher) recordFailure(result PublicationResult) PublicationResult {
	if p == nil || p.ledger == nil || result.Failure == nil {
		return result
	}
	if err := p.ledger.Record(result.Failure); err != nil {
		result.LedgerFailure = &FailureLedgerWriteFailure{Stage: "LEDGER_WRITE", Reason: "SINK_FAILED", cause: err}
	}
	return result
}

// PublishCaptureSet is the all-or-nothing private publication primitive. The
// supplied byte strings may be in any order; each must be exactly admitted and
// must bijectively match manifest.Constituents before staging begins.
func (p *Publisher) PublishCaptureSet(m Manifest, exactV5 [][]byte, authority ExactBytesAuthority) PublicationResult {
	fail := func(stage, reason string, err error) PublicationResult {
		failure := &PublicationFailure{Stage: stage, Reason: reason, CodecCategory: "CAPTURE_SET_V1", RootSource: "HOST_PUBLICATION_ROOT", Err: err}
		return p.recordFailure(PublicationResult{Failure: failure, Err: failure})
	}
	manifestRaw, err := EncodeCanonical(m)
	if err != nil || m.Disclosure != "PRIVATE" {
		if err == nil {
			err = errors.New("only private capture sets may be published")
		}
		return fail("PRIVATE_VALIDATION", "MANIFEST_REJECTED", err)
	}
	if len(exactV5) != len(m.Constituents) {
		return fail("PRIVATE_VALIDATION", "CONSTITUENT_CARDINALITY", errors.New("constituent byte cardinality mismatch"))
	}
	bySelector := make(map[string][]byte, len(exactV5))
	for _, raw := range exactV5 {
		c, admitErr := authority.Constituent(raw)
		if admitErr != nil {
			return fail("PRIVATE_VALIDATION", "CONSTITUENT_REJECTED", admitErr)
		}
		if _, duplicate := bySelector[c.ImmutableSelector]; duplicate {
			return fail("PRIVATE_VALIDATION", "DUPLICATE_CONSTITUENT", errors.New("duplicate constituent bytes"))
		}
		bySelector[c.ImmutableSelector] = append([]byte(nil), raw...)
	}
	for _, c := range m.Constituents {
		raw, ok := bySelector[c.ImmutableSelector]
		if !ok {
			return fail("PRIVATE_VALIDATION", "CONSTITUENT_ASSOCIATION", errors.New("manifest constituent association mismatch"))
		}
		if verifyErr := authority.VerifyConstituent(c, raw); verifyErr != nil {
			return fail("PRIVATE_VALIDATION", "CONSTITUENT_VERIFY", verifyErr)
		}
	}
	bundleRaw, err := encodePrivateBundle(m, manifestRaw, bySelector)
	if err != nil {
		return fail("ENCODE", "BUNDLE_ENCODE", err)
	}
	sum := sha256.Sum256(bundleRaw)
	candidateDigest := "sha256:" + hex.EncodeToString(sum[:])
	var last, primaryFailure, cleanupFailure publication.BoundFileTraceEvent
	var tempBytesCommitted, finalBytesCommitted bool
	trace := func(event publication.BoundFileTraceEvent) {
		last = event
		if !event.OK {
			if event.Stage == "CLEANUP" {
				cleanupFailure = event
			} else if primaryFailure.Stage == "" {
				primaryFailure = event
			}
		}
		if event.Stage == "TEMP" && event.Result == "CREATED" && event.OK {
			tempBytesCommitted = true
		}
		if event.Stage == "HARDLINK" && event.Result == "INSTALLED" && event.OK {
			finalBytesCommitted = true
		}
		if p.trace != nil {
			p.trace(event)
		}
	}
	selector := CaptureSetPublicationSelector(m)
	receipt, err := publication.PublishBoundFileWithTrace(p.root, selector, bundleRaw, func(committed []byte) error {
		decoded, verifyErr := decodePrivateBundle(committed, authority)
		if verifyErr != nil {
			return verifyErr
		}
		reencoded, verifyErr := encodePrivateBundle(decoded.Manifest, decoded.ManifestRaw, decoded.BySelector)
		if verifyErr != nil || !bytes.Equal(reencoded, committed) {
			return errors.New("committed bundle is not canonical")
		}
		return nil
	}, trace)
	if err != nil {
		code := publication.CodePublicationFailed
		if errors.Is(err, os.ErrExist) {
			code = publication.CodeTargetExists
		}
		failureEvent := primaryFailure
		if failureEvent.Stage == "" {
			failureEvent = cleanupFailure
		}
		if failureEvent.Stage == "" {
			failureEvent = last
		}
		stage := map[string]string{"OPEN_VALIDATE": "PRIVATE_VALIDATION", "CANDIDATE": "CANONICALIZE", "TARGET": "NO_REPLACE", "TEMP": "TEMP_WRITE", "WRITE_FSYNC": "FSYNC", "HARDLINK": "NO_REPLACE", "TARGET_EQUAL": "VERIFY", "RECEIPT": "RECEIPT", "CLEANUP": "CLEANUP"}[failureEvent.Stage]
		if stage == "" {
			stage = "INTERNAL"
		}
		failure := &PublicationFailure{Stage: stage, Reason: failureEvent.Result, CandidateSHA256: candidateDigest, CandidateByteLength: uint64(len(bundleRaw)), CodecCategory: "CAPTURE_SET_V1", CodecLimit: uint64(MaxBundleBytes), RootSource: "HOST_PUBLICATION_ROOT", TempBytesCommitted: tempBytesCommitted, FinalBytesCommitted: finalBytesCommitted, Err: err}
		if failureEvent.Stage == "TARGET" {
			v := failureEvent.Result == "EXISTS"
			failure.TargetExists = &v
		}
		if failureEvent.Stage == "TARGET_EQUAL" {
			v := failureEvent.Result == "EQUAL"
			failure.TargetEqual = &v
		}
		return p.recordFailure(PublicationResult{Code: code, Failure: failure, Err: failure})
	}
	return PublicationResult{Receipt: &PublicationReceipt{
		Selector: selector, Disclosure: "PRIVATE",
		ArtifactSHA256: receipt.Digest, ByteLength: receipt.ByteLength,
		Mechanism: receipt.Mechanism, NamespaceAtomic: receipt.NamespaceAtomic,
		CrashDurability: receipt.CrashDurability, DirectorySyncStatus: receipt.DirectorySyncStatus,
		CloseStatus: receipt.CloseStatus, ConstituentCount: len(m.Constituents),
		VerificationStatus: receipt.VerificationStatus,
	}}
}

// ResolveConstituent resolves bytes only through an already committed final
// capture-set selector and verifies that the requested constituent is associated.
func (p *Publisher) ResolveConstituent(finalSelector, immutableSelector string, authority ExactBytesAuthority) ([]byte, error) {
	m, err := p.Verify(finalSelector, authority)
	if err != nil {
		return nil, err
	}
	for _, c := range m.Constituents {
		if c.ImmutableSelector != immutableSelector {
			continue
		}
		raw, readErr := publication.ReadBoundFile(p.root, finalSelector, MaxBundleBytes)
		if readErr != nil {
			return nil, readErr
		}
		bundle, decodeErr := decodePrivateBundle(raw, authority)
		if decodeErr != nil {
			return nil, decodeErr
		}
		constituentRaw, ok := bundle.BySelector[c.ImmutableSelector]
		if !ok {
			return nil, errors.New("constituent is not associated with capture set")
		}
		return append([]byte(nil), constituentRaw...), nil
	}
	return nil, errors.New("constituent is not associated with capture set")
}

func (p *Publisher) Publish(m Manifest) PublicationResult {
	return PublicationResult{Err: errors.New("private capture-set publication requires exact V5 constituent bytes")}
}

func ValidatePublicationSelector(selector string) error {
	if !strings.HasPrefix(selector, CaptureSetSelectorPrefix) || !strings.HasSuffix(selector, ".bundle") {
		return errors.New("non-canonical capture-set selector")
	}
	digest := strings.TrimSuffix(strings.TrimPrefix(selector, CaptureSetSelectorPrefix), ".bundle")
	if len(digest) != 64 || strings.ToLower(digest) != digest {
		return errors.New("non-canonical capture-set selector")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return errors.New("non-canonical capture-set selector")
	}
	return nil
}

func (p *Publisher) Verify(selector string, authority ExactBytesAuthority) (Manifest, error) {
	if err := ValidatePublicationSelector(selector); err != nil {
		return Manifest{}, err
	}
	raw, err := publication.ReadBoundFile(p.root, selector, MaxBundleBytes)
	if err != nil {
		return Manifest{}, err
	}
	bundle, err := decodePrivateBundle(raw, authority)
	if err != nil {
		return Manifest{}, err
	}
	if CaptureSetPublicationSelector(bundle.Manifest) != selector {
		return Manifest{}, errors.New("capture-set selector identity mismatch")
	}
	return bundle.Manifest, nil
}
