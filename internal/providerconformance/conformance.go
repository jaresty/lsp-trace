// Package providerconformance validates framework-neutral external-provider protocol behavior.
package providerconformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"lsp-trace/internal/provider"
)

const (
	RequestSchema = "lsp-trace.provider-conformance-request.v1"
	ReportSchema  = "lsp-trace.provider-conformance-report.v1"
)

type Outcome string

const (
	OutcomePassed      Outcome = "passed"
	OutcomeUnsupported Outcome = "unsupported"
	OutcomeUnavailable Outcome = "unavailable"
	OutcomePartial     Outcome = "partial"
	OutcomeEmpty       Outcome = "empty"
	OutcomeFailed      Outcome = "failed"
)

type Identity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Capabilities struct {
	Operations []string `json:"operations"`
}
type Limits struct {
	RequestBytes     int           `json:"request_bytes"`
	ResponseBytes    int           `json:"response_bytes"`
	Messages         int           `json:"messages"`
	Observations     int           `json:"observations"`
	Diagnostics      int           `json:"diagnostics"`
	StderrBytes      int           `json:"stderr_bytes"`
	WallTime         time.Duration `json:"-"`
	TerminationGrace time.Duration `json:"-"`
}
type Custody struct {
	OriginalURI        string `json:"original_uri"`
	OriginalRevision   string `json:"original_revision"`
	OriginalDigest     string `json:"original_digest"`
	VirtualURI         string `json:"virtual_uri,omitempty"`
	VirtualRevision    string `json:"virtual_revision,omitempty"`
	VirtualDigest      string `json:"virtual_digest,omitempty"`
	VirtualOriginalURI string `json:"virtual_original_uri,omitempty"`
}
type Request struct {
	SchemaVersion string          `json:"schema_version"`
	RequestID     string          `json:"request_id"`
	Provider      Identity        `json:"provider"`
	Protocol      Identity        `json:"protocol"`
	Capabilities  Capabilities    `json:"capabilities"`
	Limits        Limits          `json:"limits"`
	Custody       Custody         `json:"custody"`
	Payload       json.RawMessage `json:"payload"`
}
type Config struct {
	Executable   string
	Arguments    []string
	Directory    string
	Environment  []string
	Provider     Identity
	Protocol     Identity
	Capabilities Capabilities
	Limits       Limits
}
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}
type Report struct {
	SchemaVersion  string   `json:"schema_version"`
	Outcome        Outcome  `json:"outcome"`
	RequestID      string   `json:"request_id"`
	Provider       Identity `json:"provider"`
	Protocol       Identity `json:"protocol"`
	Checks         []Check  `json:"checks"`
	ResponseDigest string   `json:"response_digest,omitempty"`
	Terminated     bool     `json:"terminated"`
	Reaped         bool     `json:"reaped"`
}

type response struct {
	SchemaVersion string            `json:"schema_version"`
	RequestID     string            `json:"request_id"`
	Provider      Identity          `json:"provider"`
	Protocol      Identity          `json:"protocol"`
	Outcome       Outcome           `json:"outcome"`
	Custody       Custody           `json:"custody"`
	Observations  []json.RawMessage `json:"observations"`
	Diagnostics   []string          `json:"diagnostics"`
}

func Run(ctx context.Context, cfg Config, request Request) Report {
	report := Report{SchemaVersion: ReportSchema, Outcome: OutcomeFailed, RequestID: request.RequestID, Provider: cfg.Provider, Protocol: cfg.Protocol}
	if err := validateDeclaration(cfg, request); err != nil {
		return failed(report, "declaration", err)
	}
	report.Checks = append(report.Checks, Check{Name: "declaration", Passed: true})
	encoded, err := json.Marshal(request)
	if err != nil {
		return failed(report, "request_schema", err)
	}
	if len(encoded) > cfg.Limits.RequestBytes {
		return failed(report, "bounds", errors.New("request byte limit exceeded"))
	}

	first, receipt := execute(ctx, cfg, encoded)
	report.Terminated, report.Reaped = receipt.Terminated, receipt.Reaped
	if receipt.Failure != nil {
		outcome := OutcomeFailed
		if receipt.Failure.Kind == provider.ProviderUnavailable || receipt.Failure.Kind == provider.TimedOut || receipt.Failure.Kind == provider.Canceled {
			outcome = OutcomeUnavailable
		}
		report.Outcome = outcome
		report.Checks = append(report.Checks, Check{Name: "lifecycle", Passed: receipt.Reaped, Detail: receipt.Failure.Reason})
		return report
	}
	report.Reaped = receipt.Reaped
	report.Checks = append(report.Checks, Check{Name: "frame", Passed: receipt.Messages == 1 && receipt.Reaped})
	if err := validateResponse(first, cfg, request); err != nil {
		return failed(report, classify(err), err)
	}
	report.Checks = append(report.Checks,
		Check{Name: "schema", Passed: true}, Check{Name: "identity", Passed: true}, Check{Name: "custody", Passed: true}, Check{Name: "bounds", Passed: true})

	second, replayReceipt := execute(ctx, cfg, encoded)
	if replayReceipt.Failure != nil || !bytes.Equal(first, second) {
		return failed(report, "replay", errors.New("fresh replay response bytes differ"))
	}
	sum := sha256.Sum256(first)
	report.ResponseDigest = fmt.Sprintf("sha256:%x", sum[:])
	report.Checks = append(report.Checks, Check{Name: "replay", Passed: true})
	var decoded response
	_ = json.Unmarshal(first, &decoded)
	report.Outcome = decoded.Outcome
	return report
}

func execute(ctx context.Context, cfg Config, request []byte) (json.RawMessage, provider.Receipt) {
	registry := provider.NewRegistry()
	_ = registry.Register(provider.Registration{ID: cfg.Provider.Name + "@" + cfg.Provider.Version, Path: cfg.Executable, Args: append([]string(nil), cfg.Arguments...), Dir: cfg.Directory, Env: append([]string(nil), cfg.Environment...)})
	limits := provider.Limits{RequestBytes: cfg.Limits.RequestBytes, ResponseBytes: cfg.Limits.ResponseBytes, ProtocolMessages: cfg.Limits.Messages, StderrBytes: cfg.Limits.StderrBytes, WallTime: cfg.Limits.WallTime, TerminationGrace: cfg.Limits.TerminationGrace}
	receipt := provider.NewRuntime(registry).Execute(ctx, cfg.Provider.Name+"@"+cfg.Provider.Version, request, limits)
	return append(json.RawMessage(nil), receipt.Response...), receipt
}

func validateDeclaration(cfg Config, req Request) error {
	if !filepath.IsAbs(cfg.Executable) {
		return errors.New("executable must be absolute")
	}
	if cfg.Provider.Name == "" || cfg.Provider.Version == "" || cfg.Protocol.Name == "" || cfg.Protocol.Version == "" || len(cfg.Capabilities.Operations) == 0 {
		return errors.New("declared identity, protocol, and capabilities are required")
	}
	l := cfg.Limits
	if l.RequestBytes <= 0 || l.ResponseBytes <= 0 || l.Messages != 1 || l.Observations <= 0 || l.Diagnostics <= 0 || l.StderrBytes < 0 || l.WallTime <= 0 || l.TerminationGrace <= 0 {
		return errors.New("positive bounded limits and exactly one message are required")
	}
	if req.SchemaVersion != RequestSchema || req.RequestID == "" || !reflect.DeepEqual(req.Provider, cfg.Provider) || !reflect.DeepEqual(req.Protocol, cfg.Protocol) || !reflect.DeepEqual(req.Capabilities, cfg.Capabilities) || !reflect.DeepEqual(req.Limits, cfg.Limits) {
		return errors.New("request schema or declarations mismatch")
	}
	if err := validateCustody(req.Custody); err != nil {
		return err
	}
	return strictJSON(req.Payload)
}

func validateResponse(raw []byte, cfg Config, req Request) error {
	var got response
	if err := strictDecode(raw, &got); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	if got.SchemaVersion != "lsp-trace.provider-conformance-response.v1" {
		return errors.New("schema: unsupported response schema")
	}
	if got.RequestID != req.RequestID || !reflect.DeepEqual(got.Provider, cfg.Provider) || !reflect.DeepEqual(got.Protocol, cfg.Protocol) {
		return errors.New("identity: provider, protocol, or request mismatch")
	}
	if !reflect.DeepEqual(got.Custody, req.Custody) || validateCustody(got.Custody) != nil {
		return errors.New("custody: response custody mismatch")
	}
	if len(got.Observations) > cfg.Limits.Observations || len(got.Diagnostics) > cfg.Limits.Diagnostics {
		return errors.New("bounds: item limit exceeded")
	}
	if !validOutcome(got.Outcome) {
		return errors.New("schema: unsupported outcome")
	}
	if got.Outcome == OutcomeEmpty && len(got.Observations) != 0 {
		return errors.New("schema: empty outcome contains observations")
	}
	return nil
}

func validateCustody(c Custody) error {
	if c.OriginalURI == "" || c.OriginalRevision == "" || c.OriginalDigest == "" {
		return errors.New("original custody required")
	}
	virtual := []string{c.VirtualURI, c.VirtualRevision, c.VirtualDigest, c.VirtualOriginalURI}
	count := 0
	for _, value := range virtual {
		if value != "" {
			count++
		}
	}
	if count != 0 && (count != len(virtual) || c.VirtualOriginalURI != c.OriginalURI || c.VirtualRevision != c.OriginalRevision) {
		return errors.New("virtual custody must be complete and subordinate to original")
	}
	return nil
}
func validOutcome(o Outcome) bool {
	switch o {
	case OutcomePassed, OutcomeUnsupported, OutcomeUnavailable, OutcomePartial, OutcomeEmpty, OutcomeFailed:
		return true
	}
	return false
}
func strictJSON(raw []byte) error { var v any; return strictDecode(raw, &v) }
func strictDecode(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("exactly one JSON value required")
	}
	return nil
}
func classify(err error) string {
	for _, n := range []string{"schema", "identity", "custody", "bounds"} {
		if strings.HasPrefix(err.Error(), n+":") {
			return n
		}
	}
	return "schema"
}
func failed(r Report, name string, err error) Report {
	r.Outcome = OutcomeFailed
	r.Checks = append(r.Checks, Check{Name: name, Passed: false, Detail: err.Error()})
	return r
}

func CanonicalCapabilities(c Capabilities) Capabilities {
	out := Capabilities{Operations: append([]string(nil), c.Operations...)}
	sort.Strings(out.Operations)
	return out
}
