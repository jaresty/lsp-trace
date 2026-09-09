// Package seedbinding defines the closed host-validated pre-provider seed identity contract.
package seedbinding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const VersionV2 = "lsp-trace.seed-binding.v2"
const MaxManifestBytes = 1 << 20

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
	Language  string `json:"language"`
	Authority string `json:"authority"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}
type Manifest struct {
	SchemaVersion            string            `json:"schema_version"`
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
}

type CustodyClaim struct {
	Repository, SourceRevision, TargetPath, TargetSourceSHA256 string
}

type RevisionAuthority interface {
	Verify(context.Context, CustodyClaim) error
}

type HostCustodyReceipt struct {
	Authenticated                                              bool
	Repository, SourceRevision, TargetPath, TargetSourceSHA256 string
	Prepared                                                   bool
	PreparedManifestSHA256                                     string
	PreparedAllowedChanges                                     []string
}

type HostReceiptAuthority struct{ Receipt HostCustodyReceipt }

func (a HostReceiptAuthority) Verify(_ context.Context, claim CustodyClaim) error {
	r := a.Receipt
	if !r.Authenticated || r.Repository == "" || r.SourceRevision == "" || r.TargetPath == "" || r.TargetSourceSHA256 == "" {
		return fmt.Errorf("host custody receipt unauthenticated or unavailable")
	}
	if r.Repository != claim.Repository || r.SourceRevision != claim.SourceRevision || r.TargetPath != claim.TargetPath || !strings.EqualFold(r.TargetSourceSHA256, claim.TargetSourceSHA256) {
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
}

// ValidateMechanical authenticates and retains the exact source snapshot before
// any provider attempt. Semantic declaration admission is generation-bound and
// deliberately occurs later over the initialized provider's LSP stream.
func ValidateMechanical(ctx context.Context, workspace string, m Manifest, revision RevisionAuthority) Outcome {
	invalid := func(code, detail string) Outcome {
		return Outcome{Status: Invalid, Terminal: code, PrivateDetail: detail}
	}
	if m.SchemaVersion != VersionV2 || m.ID == "" || m.ExpectedSymbol == "" || m.ExpectedDeclaringFile == "" || m.SourceRevision == "" || len(m.SourceSHA256) != 64 || m.Validator.Language == "" || m.Validator.Authority == "" || m.Validator.Name == "" || m.Validator.Version == "" {
		return invalid(LocatorInvalid, "invalid closed v2 manifest")
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
		return Outcome{Status: Mismatch, Terminal: BindingMismatch, PrivateDetail: "declaring file mismatch"}
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
		return Outcome{Status: Mismatch, Terminal: SourceMismatch, PrivateDetail: "descriptor/path identity changed"}
	}
	digest := sha256.Sum256(source)
	expected, e := hex.DecodeString(m.SourceSHA256)
	if e != nil || !bytes.Equal(digest[:], expected) {
		return Outcome{Status: Mismatch, Terminal: SourceMismatch, PrivateDetail: "digest mismatch"}
	}
	claim := CustodyClaim{Repository: workspace, SourceRevision: m.SourceRevision, TargetPath: m.ExpectedDeclaringFile, TargetSourceSHA256: fmt.Sprintf("%x", digest)}
	if revision == nil || revision.Verify(ctx, claim) != nil {
		return Outcome{Status: Unavailable, Terminal: BindingUnavailable, PrivateDetail: "authenticated custody receipt unavailable or rejected"}
	}
	if !validPosition(source, m.Locator.Line, m.Locator.Character, m.Locator.Encoding) {
		return invalid(LocatorInvalid, "coordinate outside retained source bytes or code-unit boundary")
	}
	r := m.ExpectedDeclarationRange
	if !validPosition(source, r.StartLine, r.StartCharacter, m.Locator.Encoding) || !validPosition(source, r.EndLine, r.EndCharacter, m.Locator.Encoding) ||
		comparePosition(r.StartLine, r.StartCharacter, r.EndLine, r.EndCharacter) > 0 || !manifestRangeContains(r, m.Locator.Line, m.Locator.Character) {
		return invalid(LocatorInvalid, "declaration range invalid for retained source bytes")
	}
	return Outcome{Status: Match, Source: source}
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

func DecodeV2(raw []byte) (Manifest, error) {
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
	if m.SchemaVersion != VersionV2 {
		return m, fmt.Errorf("unsupported seed binding version")
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
