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
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const ExternalProtocolV1 = "lsp-trace.seed-validator.v1"

// ExternalConfig is explicit host authority for the preflight helper. The
// helper is not an LSP provider and receives no inherited environment.
type ExternalConfig struct {
	Protocol      string            `json:"protocol"`
	Identity      ValidatorIdentity `json:"identity"`
	Executable    string            `json:"executable"`
	Arguments     []string          `json:"arguments,omitempty"`
	Directory     string            `json:"directory"`
	Environment   []string          `json:"environment,omitempty"`
	TimeoutMillis int               `json:"timeout_ms"`
	RequestBytes  int               `json:"request_bytes"`
	ResponseBytes int               `json:"response_bytes"`
}

type externalRequest struct {
	Protocol       string   `json:"protocol"`
	Manifest       Manifest `json:"manifest"`
	Workspace      string   `json:"workspace"`
	Source         []byte   `json:"source"`
	ObservedSHA256 string   `json:"observed_sha256"`
	SourceRevision string   `json:"source_revision"`
}

type externalResponse struct {
	Protocol         string            `json:"protocol"`
	Validator        ValidatorIdentity `json:"validator"`
	Status           Status            `json:"status"`
	DeclarationName  string            `json:"declaration_name"`
	DeclaringFile    string            `json:"declaring_file"`
	NameRange        Range             `json:"name_range"`
	FullRange        Range             `json:"full_range"`
	ObservedSHA256   string            `json:"observed_sha256"`
	ObservedRevision string            `json:"observed_revision"`
}

type ExternalValidator struct{ config ExternalConfig }

type ExactRevisionAuthority struct{ Revision string }

func (a ExactRevisionAuthority) Verify(_ context.Context, _ string, revision string) error {
	if a.Revision == "" || revision != a.Revision {
		return errors.New("revision rejected")
	}
	return nil
}

func DecodeExternalConfig(raw []byte) (ExternalConfig, error) {
	var c ExternalConfig
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return c, errors.New("seedbinding: validator config must contain one JSON object")
	}
	return c, nil
}

func NewExternalValidator(c ExternalConfig) (*ExternalValidator, error) {
	if c.Protocol != ExternalProtocolV1 || c.Identity.Language == "" || c.Identity.Authority == "" || c.Identity.Name == "" || c.Identity.Version == "" || !filepath.IsAbs(c.Executable) || filepath.Clean(c.Executable) != c.Executable || !filepath.IsAbs(c.Directory) || filepath.Clean(c.Directory) != c.Directory || c.TimeoutMillis <= 0 || c.RequestBytes <= 0 || c.ResponseBytes <= 0 {
		return nil, errors.New("seedbinding: incomplete external validator authority")
	}
	for _, entry := range c.Environment {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || strings.TrimSpace(key) != key || strings.ContainsAny(key, "\x00=") {
			return nil, errors.New("seedbinding: invalid validator environment authority")
		}
	}
	return &ExternalValidator{config: c}, nil
}
func (v *ExternalValidator) Identity() ValidatorIdentity { return v.config.Identity }
func (v *ExternalValidator) Validate(in ValidationInput) ValidationResult {
	sum := sha256.Sum256(in.Source)
	req := externalRequest{Protocol: ExternalProtocolV1, Manifest: in.Manifest, Workspace: in.Workspace, Source: in.Source, ObservedSHA256: hex.EncodeToString(sum[:]), SourceRevision: in.Manifest.SourceRevision}
	encoded, err := json.Marshal(req)
	if err != nil || len(encoded) > v.config.RequestBytes {
		return ValidationResult{Status: Invalid, PrivateDetail: "validator request invalid"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(v.config.TimeoutMillis)*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, v.config.Executable, v.config.Arguments...)
	cmd.Dir = v.config.Directory
	cmd.Env = append([]string(nil), v.config.Environment...)
	cmd.Stdin = bytes.NewReader(append(encoded, '\n'))
	var stdout bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, remaining: int64(v.config.ResponseBytes)}
	if err := cmd.Run(); err != nil {
		return ValidationResult{Status: Unavailable, PrivateDetail: "validator execution unavailable"}
	}
	var response externalResponse
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return ValidationResult{Status: Invalid, PrivateDetail: "validator response invalid"}
	}
	if response.Protocol != ExternalProtocolV1 || response.Validator != v.config.Identity || response.ObservedSHA256 != req.ObservedSHA256 || response.ObservedRevision != in.Manifest.SourceRevision {
		return ValidationResult{Status: Invalid, PrivateDetail: "validator authority or observation binding invalid"}
	}
	if response.Status != Match {
		if response.Status == Mismatch || response.Status == Unavailable || response.Status == Invalid {
			return ValidationResult{Status: response.Status, PrivateDetail: "validator rejected binding"}
		}
		return ValidationResult{Status: Invalid, PrivateDetail: "validator status invalid"}
	}
	if response.DeclarationName != in.Manifest.ExpectedSymbol || response.DeclaringFile != in.Manifest.ExpectedDeclaringFile || response.FullRange != in.Manifest.ExpectedDeclarationRange || !rangeContains(response.NameRange, in.Manifest.Locator.Line, in.Manifest.Locator.Character) {
		return ValidationResult{Status: Mismatch, PrivateDetail: "semantic declaration mismatch"}
	}
	return ValidationResult{Status: Match}
}

type limitedWriter struct {
	w         io.Writer
	remaining int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("validator output limit exceeded")
	}
	n, err := w.w.Write(p)
	w.remaining -= int64(n)
	return n, err
}
func rangeContains(r Range, line, character uint32) bool {
	if line < r.StartLine || line > r.EndLine {
		return false
	}
	if line == r.StartLine && character < r.StartCharacter {
		return false
	}
	if line == r.EndLine && character > r.EndCharacter {
		return false
	}
	return true
}
