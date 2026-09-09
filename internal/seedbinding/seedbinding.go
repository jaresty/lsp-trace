// Package seedbinding defines the closed host-validated pre-provider seed identity contract.
package seedbinding

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const VersionV2 = "lsp-trace.seed-binding.v2"
const VersionV3 = "lsp-trace.seed-binding.v3"
const MaxManifestBytes = 1 << 20

type CustodyMode string

const (
	VerifiedHost        CustodyMode = "VERIFIED_HOST"
	CallerAssertedLocal CustodyMode = "CALLER_ASSERTED_LOCAL"
)
const HostPreparedPolicyVersionV1 uint32 = 1

type Status string

const (
	Match       Status = "MATCH"
	Mismatch    Status = "MISMATCH"
	Unavailable Status = "UNAVAILABLE"
	Invalid     Status = "INVALID"
)
const (
	LocatorInvalid     = "SEED_LOCATOR_INVALID"
	SourceMismatch     = "SEED_SOURCE_MISMATCH"
	BindingMismatch    = "SEED_BINDING_MISMATCH"
	BindingUnavailable = "SEED_BINDING_UNAVAILABLE"
)

type Locator struct {
	URI       string `json:"uri"`
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
	Encoding  string `json:"encoding"`
}
type Range struct {
	StartLine      uint32 `json:"start_line"`
	StartCharacter uint32 `json:"start_character"`
	EndLine        uint32 `json:"end_line"`
	EndCharacter   uint32 `json:"end_character"`
}
type ValidatorIdentity struct {
	Language         string `json:"language"`
	Authority        string `json:"authority"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Class            string `json:"provider_class,omitempty"`
	ExecutableSHA256 string `json:"executable_sha256,omitempty"`
	PayloadSHA256    string `json:"payload_sha256,omitempty"`
	ConfigSHA256     string `json:"config_sha256,omitempty"`
}

// ProviderIdentity is the exact host-observed identity of the selected managed
// provider. It is kept outside historical seed-manifest JSON so v1/v2/v3 bytes
// remain unchanged.
type ProviderIdentity struct {
	Class, Authority, Name, Version               string
	ExecutableSHA256, PayloadSHA256, ConfigSHA256 string
}

func VerifyProviderIdentity(want, got ProviderIdentity) error {
	if want.Class == "" || want.Authority == "" || want.Name == "" || want.Version == "" || want.ExecutableSHA256 == "" || want.PayloadSHA256 == "" || want.ConfigSHA256 == "" {
		return fmt.Errorf("provider identity incomplete")
	}
	if want != got {
		return fmt.Errorf("selected provider identity mismatch")
	}
	return nil
}

type Manifest struct {
	SchemaVersion            string            `json:"schema_version"`
	CustodyMode              CustodyMode       `json:"custody_mode,omitempty"`
	PreparedManifestSHA256   string            `json:"prepared_manifest_sha256,omitempty"`
	ID                       string            `json:"id"`
	Locator                  Locator           `json:"locator"`
	ExpectedSymbol           string            `json:"expected_symbol"`
	ExpectedDeclaringFile    string            `json:"expected_declaring_file"`
	ExpectedDeclarationRange Range             `json:"expected_declaration_range"`
	SourceRevision           string            `json:"source_revision"`
	SourceSHA256             string            `json:"source_sha256"`
	Validator                ValidatorIdentity `json:"validator"`
}
type ValidationResult struct {
	Status        Status
	PrivateDetail string
	Provenance    CustodyMode
}

type CustodyClaim struct {
	Repository, SourceRevision, TargetPath, TargetSourceSHA256, SeedManifestSHA256 string
}

type RevisionAuthority interface {
	Verify(context.Context, CustodyClaim) error
}

type PreparedModificationManifest struct {
	Version                                                                 uint32
	FinderSHA256, SourceArchiveSHA256, SourceTreeSHA256, PreparedTreeSHA256 string
	AllowedChanges                                                          []string
	TargetPath, TargetUnchangedSHA256                                       string
	SourceCommit, Adaptations                                               string // Adaptations is retained for historical source compatibility.
	AdaptationIDs                                                           []string
	PolicyID, AssessmentID, ContextID                                       string
}

func CanonicalPreparedManifest(m PreparedModificationManifest) ([]byte, error) {
	adaptationsPresent := len(m.AdaptationIDs) != 0 || m.Adaptations != ""
	if m.Version != HostPreparedPolicyVersionV1 || m.FinderSHA256 == "" || m.SourceArchiveSHA256 == "" || m.SourceTreeSHA256 == "" || m.PreparedTreeSHA256 == "" || m.TargetPath == "" || m.TargetUnchangedSHA256 == "" || m.SourceCommit == "" || !adaptationsPresent || m.SourceCommit == m.Adaptations {
		return nil, fmt.Errorf("prepared modification manifest incomplete")
	}
	return json.Marshal(m)
}

func framedSigningBytes(domain string, canonical []byte) []byte {
	out := make([]byte, 0, 4+len(domain)+8+len(canonical))
	var n [8]byte
	binary.BigEndian.PutUint32(n[:4], uint32(len(domain)))
	out = append(out, n[:4]...)
	out = append(out, domain...)
	binary.BigEndian.PutUint64(n[:], uint64(len(canonical)))
	out = append(out, n[:]...)
	return append(out, canonical...)
}

func PreparedManifestSigningBytes(canonical []byte) []byte {
	return framedSigningBytes("lsp-trace:prepared-modification-manifest:v1", canonical)
}

// HostPreparedPolicy is signed by a host root that is independent of request and
// command-line input. Signature is excluded from the canonical signed payload.
type HostPreparedPolicy struct {
	Version                             uint32
	PolicyID, AssessmentID, ContextID   string
	Prepared                            bool
	Repository, SourceCommit            string
	RequiredAdaptations, AllowedChanges []string
	TargetPath, TargetUnchangedSHA256   string
	ValidFrom, ValidUntil               time.Time
	Signature                           []byte
}

func CanonicalHostPreparedPolicy(p HostPreparedPolicy) ([]byte, error) {
	p.Signature = nil
	if p.Version != HostPreparedPolicyVersionV1 || p.PolicyID == "" || p.AssessmentID == "" || p.ContextID == "" || !p.Prepared || p.Repository == "" || p.SourceCommit == "" || len(p.RequiredAdaptations) == 0 || p.TargetPath == "" || p.TargetUnchangedSHA256 == "" || p.ValidFrom.IsZero() || !p.ValidUntil.After(p.ValidFrom) {
		return nil, fmt.Errorf("host prepared policy incomplete")
	}
	return json.Marshal(p)
}

func HostPreparedPolicySigningBytes(canonical []byte) []byte {
	return framedSigningBytes("lsp-trace:host-prepared-policy:v1", canonical)
}

// VerifiedPreparedContext is opaque outside this package. It can only be
// produced by VerifyPrepared under an anchored host root.
type VerifiedPreparedContext struct{ digest [sha256.Size]byte }

func (v VerifiedPreparedContext) valid() bool { return v.digest != [sha256.Size]byte{} }

type HostRootSet struct {
	Version uint32
	Keys    map[string]ed25519.PublicKey
}

func (r HostRootSet) Verify(rootID string, policy HostPreparedPolicy, receipt HostCustodyReceipt, now time.Time) (VerifiedPreparedContext, error) {
	key, ok := r.Keys[rootID]
	if r.Version != HostPreparedPolicyVersionV1 || policy.Version != r.Version || !ok || len(key) != ed25519.PublicKeySize {
		return VerifiedPreparedContext{}, fmt.Errorf("unsupported host root version")
	}
	return VerifyPrepared(policy, receipt, key, now)
}

func VerifyPrepared(policy HostPreparedPolicy, receipt HostCustodyReceipt, root ed25519.PublicKey, now time.Time) (VerifiedPreparedContext, error) {
	if policy.Version != HostPreparedPolicyVersionV1 || receipt.Version != policy.Version {
		return VerifiedPreparedContext{}, fmt.Errorf("unsupported or mismatched prepared policy version")
	}
	canonical, err := CanonicalHostPreparedPolicy(policy)
	if err != nil || len(root) != ed25519.PublicKeySize || len(policy.Signature) != ed25519.SignatureSize || !ed25519.Verify(root, HostPreparedPolicySigningBytes(canonical), policy.Signature) {
		return VerifiedPreparedContext{}, fmt.Errorf("host prepared policy signature mismatch")
	}
	if now.Before(policy.ValidFrom) || !now.Before(policy.ValidUntil) {
		return VerifiedPreparedContext{}, fmt.Errorf("host prepared policy outside validity")
	}
	if !receipt.Prepared || receipt.PolicyID != policy.PolicyID || receipt.AssessmentID != policy.AssessmentID || receipt.ContextID != policy.ContextID || receipt.Repository != policy.Repository || receipt.SourceRevision != policy.SourceCommit || receipt.TargetPath != policy.TargetPath || !strings.EqualFold(receipt.TargetSourceSHA256, policy.TargetUnchangedSHA256) || !equalStrings(receipt.PreparedAllowedChanges, policy.AllowedChanges) {
		return VerifiedPreparedContext{}, fmt.Errorf("custody receipt does not exactly match host prepared policy")
	}
	if err := receipt.VerifyPrepared(root, receipt.TargetPath); err != nil {
		return VerifiedPreparedContext{}, err
	}
	var manifest PreparedModificationManifest
	if err := json.Unmarshal(receipt.PreparedManifest, &manifest); err != nil || manifest.Version != policy.Version || manifest.PolicyID != policy.PolicyID || manifest.AssessmentID != policy.AssessmentID || manifest.ContextID != policy.ContextID || manifest.SourceCommit != policy.SourceCommit || !equalStrings(manifest.AdaptationIDs, policy.RequiredAdaptations) || !equalStrings(manifest.AllowedChanges, policy.AllowedChanges) {
		return VerifiedPreparedContext{}, fmt.Errorf("prepared manifest does not exactly match host policy")
	}
	digest := sha256.Sum256(append(HostPreparedPolicySigningBytes(canonical), receipt.Signature...))
	return VerifiedPreparedContext{digest: digest}, nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type HostCustodyReceipt struct {
	Version                                                    uint32
	Authenticated                                              bool
	Repository, SourceRevision, TargetPath, TargetSourceSHA256 string
	SeedManifestSHA256                                         string
	Prepared                                                   bool
	PreparedManifestSHA256                                     string
	PreparedAllowedChanges                                     []string
	PreparedManifest                                           []byte
	PreparedManifestSignature                                  []byte
	KeyID                                                      string
	PolicyID, AssessmentID, ContextID                          string
	Signature                                                  []byte
}

func CustodyReceiptSigningBytes(r HostCustodyReceipt) ([]byte, error) {
	r.Authenticated = false
	r.Signature = nil
	canonical, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return framedSigningBytes("lsp-trace:seed-custody-receipt:v1", canonical), nil
}

func CustodyReceiptDigest(r HostCustodyReceipt) (string, error) {
	payload, err := CustodyReceiptSigningBytes(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func (r HostCustodyReceipt) VerifyPrepared(publicKey ed25519.PublicKey, target string) error {
	if !r.Prepared {
		return nil
	}
	if r.Version != HostPreparedPolicyVersionV1 {
		return fmt.Errorf("unsupported prepared receipt version")
	}
	if len(publicKey) != ed25519.PublicKeySize || len(r.PreparedManifest) == 0 || len(r.PreparedManifestSignature) != ed25519.SignatureSize {
		return fmt.Errorf("prepared-copy trust unavailable")
	}
	var m PreparedModificationManifest
	if err := json.Unmarshal(r.PreparedManifest, &m); err != nil {
		return fmt.Errorf("prepared modification manifest invalid")
	}
	if m.Version != r.Version {
		return fmt.Errorf("prepared manifest version mismatch")
	}
	canonical, err := CanonicalPreparedManifest(m)
	if err != nil || !bytes.Equal(canonical, r.PreparedManifest) {
		return fmt.Errorf("prepared modification manifest is not exact canonical bytes")
	}
	sum := sha256.Sum256(canonical)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), r.PreparedManifestSHA256) {
		return fmt.Errorf("prepared modification manifest digest mismatch")
	}
	if !ed25519.Verify(publicKey, PreparedManifestSigningBytes(canonical), r.PreparedManifestSignature) {
		return fmt.Errorf("prepared modification manifest signature mismatch")
	}
	if filepath.Clean(m.TargetPath) != filepath.Clean(target) || m.TargetUnchangedSHA256 != r.TargetSourceSHA256 {
		return fmt.Errorf("prepared target unchanged binding mismatch")
	}
	for _, changed := range m.AllowedChanges {
		if filepath.Clean(changed) == filepath.Clean(target) {
			return fmt.Errorf("prepared-copy manifest may not alter seed target")
		}
	}
	return nil
}

type HostReceiptAuthority struct {
	Receipt         HostCustodyReceipt
	PreparedContext VerifiedPreparedContext
}

func (a HostReceiptAuthority) Verify(_ context.Context, claim CustodyClaim) error {
	r := a.Receipt
	if r.Prepared && !a.PreparedContext.valid() {
		return fmt.Errorf("verified prepared context unavailable")
	}
	if !r.Authenticated || r.Repository == "" || r.SourceRevision == "" || r.TargetPath == "" || r.TargetSourceSHA256 == "" || r.SeedManifestSHA256 == "" {
		return fmt.Errorf("host custody receipt unauthenticated or unavailable")
	}
	if r.Repository != claim.Repository || r.SourceRevision != claim.SourceRevision || r.TargetPath != claim.TargetPath || !strings.EqualFold(r.TargetSourceSHA256, claim.TargetSourceSHA256) || !strings.EqualFold(r.SeedManifestSHA256, claim.SeedManifestSHA256) {
		return fmt.Errorf("host custody receipt mismatch")
	}
	if r.Prepared && r.PreparedManifestSHA256 == "" {
		return fmt.Errorf("prepared-copy receipt lacks modification manifest")
	}
	for _, changed := range r.PreparedAllowedChanges {
		if filepath.Clean(changed) == filepath.Clean(claim.TargetPath) {
			return fmt.Errorf("prepared-copy manifest may not alter seed target")
		}
	}
	return nil
}

type Outcome struct {
	Status                  Status
	Terminal, PrivateDetail string
	Source                  []byte
	Provenance              CustodyMode
}

// ValidateMechanical authenticates and retains the exact source snapshot before
// any provider attempt. Semantic declaration admission is generation-bound and
// deliberately occurs later over the initialized provider's LSP stream.
func ValidateMechanical(ctx context.Context, workspace string, m Manifest, revision RevisionAuthority) Outcome {
	mode := m.CustodyMode
	if m.SchemaVersion == VersionV2 {
		mode = VerifiedHost
	}
	invalid := func(code, detail string) Outcome {
		return Outcome{Status: Invalid, Terminal: code, PrivateDetail: detail, Provenance: mode}
	}
	if m.SchemaVersion != VersionV2 && m.SchemaVersion != VersionV3 {
		return invalid(LocatorInvalid, "unsupported seed manifest version")
	}
	if m.SchemaVersion == VersionV2 && (m.CustodyMode != "" || m.PreparedManifestSHA256 != "") {
		return invalid(LocatorInvalid, "invalid closed v2 manifest")
	}
	if m.SchemaVersion == VersionV3 && mode != VerifiedHost && mode != CallerAssertedLocal {
		return invalid(LocatorInvalid, "v3 custody_mode must be explicit")
	}
	if m.ID == "" || m.ExpectedSymbol == "" || m.ExpectedDeclaringFile == "" || strings.TrimSpace(m.SourceRevision) == "" || strings.TrimSpace(m.SourceRevision) != m.SourceRevision || len(m.SourceSHA256) != 64 || m.Validator.Language == "" || m.Validator.Authority == "" || m.Validator.Name == "" || m.Validator.Version == "" {
		return invalid(LocatorInvalid, "seed manifest binding incomplete")
	}
	if mode == CallerAssertedLocal {
		sourceDigest, err := hex.DecodeString(m.SourceSHA256)
		if err != nil || len(sourceDigest) != sha256.Size || m.SourceSHA256 != strings.ToLower(m.SourceSHA256) {
			return invalid(SourceMismatch, "CALLER_ASSERTED source digest is not canonical sha256")
		}
		if m.Validator.Class == "" || m.Validator.ExecutableSHA256 == "" || m.Validator.PayloadSHA256 == "" || m.Validator.ConfigSHA256 == "" {
			return invalid(BindingMismatch, "CALLER_ASSERTED provider identity pin incomplete")
		}
		if m.PreparedManifestSHA256 != "" {
			decoded, err := hex.DecodeString(m.PreparedManifestSHA256)
			if err != nil || len(decoded) != sha256.Size || m.PreparedManifestSHA256 != strings.ToLower(m.PreparedManifestSHA256) {
				return invalid(BindingMismatch, "CALLER_ASSERTED prepared manifest digest invalid")
			}
		}
	}
	u, err := url.Parse(m.Locator.URI)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.String() != m.Locator.URI || filepath.Clean(u.Path) != u.Path {
		return invalid(LocatorInvalid, "noncanonical file URI")
	}
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		return invalid(LocatorInvalid, "workspace unavailable")
	}
	candidate := filepath.FromSlash(u.Path)
	rel, err := filepath.Rel(workspace, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return invalid(LocatorInvalid, "source outside workspace")
	}
	if filepath.ToSlash(rel) != m.ExpectedDeclaringFile {
		return Outcome{Status: Mismatch, Terminal: BindingMismatch, PrivateDetail: "declaring file mismatch", Provenance: mode}
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return invalid(LocatorInvalid, "workspace descriptor unavailable")
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(m.ExpectedDeclaringFile))
	if err != nil {
		return invalid(LocatorInvalid, "source rooted open failed")
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return invalid(LocatorInvalid, "source is not regular")
	}
	const maxSourceBytes = 64 << 20
	source, err := io.ReadAll(io.LimitReader(f, maxSourceBytes+1))
	if err != nil {
		return invalid(LocatorInvalid, "source read failed")
	}
	if len(source) > maxSourceBytes {
		return invalid(LocatorInvalid, "source exceeds retained byte limit")
	}
	after, err := f.Stat()
	pathAfter, pathErr := root.Stat(filepath.FromSlash(m.ExpectedDeclaringFile))
	if err != nil || pathErr != nil || !os.SameFile(before, after) || !os.SameFile(after, pathAfter) {
		return Outcome{Status: Mismatch, Terminal: SourceMismatch, PrivateDetail: "descriptor/path identity changed", Provenance: mode}
	}
	digest := sha256.Sum256(source)
	expected, e := hex.DecodeString(m.SourceSHA256)
	if e != nil || !bytes.Equal(digest[:], expected) {
		return Outcome{Status: Mismatch, Terminal: SourceMismatch, PrivateDetail: "digest mismatch", Provenance: mode}
	}
	manifestBytes, err := json.Marshal(m)
	if err != nil {
		return invalid(LocatorInvalid, "seed manifest cannot be canonicalized")
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	claim := CustodyClaim{Repository: workspace, SourceRevision: m.SourceRevision, TargetPath: m.ExpectedDeclaringFile, TargetSourceSHA256: fmt.Sprintf("%x", digest), SeedManifestSHA256: fmt.Sprintf("%x", manifestDigest)}
	if mode == VerifiedHost {
		if revision == nil || revision.Verify(ctx, claim) != nil {
			return Outcome{Status: Unavailable, Terminal: BindingUnavailable, PrivateDetail: "VERIFIED_HOST custody receipt unavailable or rejected", Provenance: mode}
		}
	} else if revision != nil {
		return invalid(BindingMismatch, "CALLER_ASSERTED_LOCAL may not be promoted by host or digest")
	}
	if !validPosition(source, m.Locator.Line, m.Locator.Character, m.Locator.Encoding) {
		return invalid(LocatorInvalid, "coordinate outside retained source bytes or code-unit boundary")
	}
	r := m.ExpectedDeclarationRange
	if !validPosition(source, r.StartLine, r.StartCharacter, m.Locator.Encoding) || !validPosition(source, r.EndLine, r.EndCharacter, m.Locator.Encoding) ||
		comparePosition(r.StartLine, r.StartCharacter, r.EndLine, r.EndCharacter) > 0 || !manifestRangeContains(r, m.Locator.Line, m.Locator.Character) {
		return invalid(LocatorInvalid, "declaration range invalid for retained source bytes")
	}
	detail := "VERIFIED_HOST mechanical seed binding matched"
	if mode == CallerAssertedLocal {
		detail = "CALLER_ASSERTED mechanical seed binding matched; not VERIFIED or AUTHENTICATED"
	}
	return Outcome{Status: Match, Source: source, PrivateDetail: detail, Provenance: mode}
}
func validPosition(source []byte, line, character uint32, encoding string) bool {
	if !utf8.Valid(source) {
		return false
	}
	lines := bytes.Split(source, []byte("\n"))
	if uint64(line) >= uint64(len(lines)) {
		return false
	}
	lineBytes := lines[line]
	runes := []rune(string(lineBytes))
	boundaries := map[uint32]struct{}{0: {}}
	var units uint32
	for _, r := range runes {
		switch encoding {
		case "utf-8":
			units += uint32(utf8.RuneLen(r))
		case "utf-16":
			units += uint32(len(utf16.Encode([]rune{r})))
		case "utf-32":
			units++
		default:
			return false
		}
		boundaries[units] = struct{}{}
	}
	_, ok := boundaries[character]
	return ok
}

func comparePosition(aLine, aCharacter, bLine, bCharacter uint32) int {
	if aLine < bLine || aLine == bLine && aCharacter < bCharacter {
		return -1
	}
	if aLine == bLine && aCharacter == bCharacter {
		return 0
	}
	return 1
}

func manifestRangeContains(r Range, line, character uint32) bool {
	return comparePosition(r.StartLine, r.StartCharacter, line, character) <= 0 && comparePosition(line, character, r.EndLine, r.EndCharacter) <= 0
}

func DecodeV3(raw []byte) (Manifest, error) {
	m, err := decodeManifest(raw)
	if err != nil {
		return m, err
	}
	if m.SchemaVersion != VersionV3 || (m.CustodyMode != VerifiedHost && m.CustodyMode != CallerAssertedLocal) {
		return m, fmt.Errorf("unsupported or implicit seed binding version/custody mode")
	}
	return m, nil
}

func DecodeV2(raw []byte) (Manifest, error) {
	m, err := decodeManifest(raw)
	if err != nil {
		return m, err
	}
	if m.SchemaVersion != VersionV2 || m.CustodyMode != "" || m.PreparedManifestSHA256 != "" {
		return m, fmt.Errorf("unsupported seed binding version")
	}
	return m, nil
}

func decodeManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if len(raw) > MaxManifestBytes {
		return m, fmt.Errorf("seed binding exceeds byte limit")
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return m, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return m, fmt.Errorf("one JSON object required")
	}
	return m, nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]struct{}{}
			for d.More() {
				keyToken, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("object key invalid")
				}
				if _, exists := seen[key]; exists {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case '[':
			for d.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		default:
			return fmt.Errorf("invalid JSON delimiter")
		}
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("one JSON value required")
	}
	return nil
}
