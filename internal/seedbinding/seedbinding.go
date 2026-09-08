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
type ValidationInput struct {
	Manifest  Manifest
	Workspace string
	Source    []byte
}
type ValidationResult struct {
	Status        Status
	PrivateDetail string
}

// Validator is a host-selected, language-aware declaration authority. MATCH means
// the retained coordinate resolves the expected declaration name, declaring file,
// and declaration name/range. Core deliberately supplies no generic fallback.
type Validator interface {
	Identity() ValidatorIdentity
	Validate(ValidationInput) ValidationResult
}
type RevisionAuthority interface {
	Verify(context.Context, string, string) error
}
type Outcome struct {
	Status                  Status
	Terminal, PrivateDetail string
	Source                  []byte
}

func Validate(ctx context.Context, workspace string, m Manifest, revision RevisionAuthority, validator Validator) Outcome {
	invalid := func(code, detail string) Outcome {
		return Outcome{Status: Invalid, Terminal: code, PrivateDetail: detail}
	}
	if m.SchemaVersion != VersionV2 || m.ID == "" || m.ExpectedSymbol == "" || m.ExpectedDeclaringFile == "" || m.SourceRevision == "" || len(m.SourceSHA256) != 64 {
		return invalid(LocatorInvalid, "invalid closed v2 manifest")
	}
	u, err := url.Parse(m.Locator.URI)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.String() != m.Locator.URI || filepath.Clean(u.Path) != u.Path {
		return invalid(LocatorInvalid, "noncanonical file URI")
	}
	workspace, err = filepath.EvalSymlinks(workspace)
	if err != nil {
		return invalid(LocatorInvalid, "workspace unavailable")
	}
	candidate, err := filepath.EvalSymlinks(filepath.FromSlash(u.Path))
	if err != nil {
		return invalid(LocatorInvalid, "source unavailable")
	}
	rel, err := filepath.Rel(workspace, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return invalid(LocatorInvalid, "source outside workspace")
	}
	if filepath.ToSlash(rel) != m.ExpectedDeclaringFile {
		return Outcome{Status: Mismatch, Terminal: BindingMismatch, PrivateDetail: "declaring file mismatch"}
	}
	f, err := os.Open(candidate)
	if err != nil {
		return invalid(LocatorInvalid, "source open failed")
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return invalid(LocatorInvalid, "source is not regular")
	}
	source, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		return invalid(LocatorInvalid, "source read failed")
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return Outcome{Status: Mismatch, Terminal: SourceMismatch, PrivateDetail: "descriptor identity changed"}
	}
	digest := sha256.Sum256(source)
	expected, e := hex.DecodeString(m.SourceSHA256)
	if e != nil || !bytes.Equal(digest[:], expected) {
		return Outcome{Status: Mismatch, Terminal: SourceMismatch, PrivateDetail: "digest mismatch"}
	}
	if revision == nil || revision.Verify(ctx, workspace, m.SourceRevision) != nil {
		return Outcome{Status: Mismatch, Terminal: SourceMismatch, PrivateDetail: "revision authority rejected"}
	}
	if !validPosition(source, m.Locator.Line, m.Locator.Character, m.Locator.Encoding) {
		return invalid(LocatorInvalid, "coordinate outside retained source bytes")
	}
	if validator == nil || validator.Identity() != m.Validator {
		return Outcome{Status: Unavailable, Terminal: BindingUnavailable, PrivateDetail: "validator identity unavailable"}
	}
	result := validator.Validate(ValidationInput{Manifest: m, Workspace: workspace, Source: append([]byte(nil), source...)})
	switch result.Status {
	case Match:
		return Outcome{Status: Match, Source: source}
	case Unavailable:
		return Outcome{Status: Unavailable, Terminal: BindingUnavailable, PrivateDetail: result.PrivateDetail}
	case Mismatch, Invalid:
		return Outcome{Status: result.Status, Terminal: BindingMismatch, PrivateDetail: result.PrivateDetail}
	default:
		return Outcome{Status: Unavailable, Terminal: BindingUnavailable, PrivateDetail: "validator returned unknown status"}
	}
}
func validPosition(source []byte, line, character uint32, encoding string) bool {
	if !utf8.Valid(source) {
		return false
	}
	lines := bytes.Split(source, []byte("\n"))
	if uint64(line) >= uint64(len(lines)) {
		return false
	}
	runes := []rune(string(lines[line]))
	var units int
	switch encoding {
	case "utf-8":
		units = len(lines[line])
	case "utf-16":
		units = len(utf16.Encode(runes))
	case "utf-32":
		units = len(runes)
	default:
		return false
	}
	return uint64(character) <= uint64(units)
}

func DecodeV2(raw []byte) (Manifest, error) {
	var m Manifest
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
