//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/manageddiagnostic"
	product "lsp-trace/internal/privatehostproductv3"
)

// Future attempts use v2; historical v1 receipts are neither reinterpreted nor rewritten.
const applicationFailureSchemaV2 = "private-application-acquisition-failure.v2"
const applicationFailureSelector = "acquisition-failure.json"
const applicationStartupSelector = "original-startup.json"
const applicationFailureMaxBytes = 4096

// Private observations, not owner completion, origin authority, or authentication.
// ProductApplication is the unit-derived product identity, NOT the outer app ID.
// RequestSHA256 binds the pre-discovery request (Requested may subsequently change).
// These two fixed artifacts use the ALREADY prepaid child/bootstrap metadata
// partition (16 MiB write), and parent's coordinator record lane (64 MiB read,
// 16 MiB write). No new envelope/refund: <=68 KiB writes, <=132 KiB child
// publication/readback plus <=68 KiB parent reads. Temporary publication bytes
// are the final bytes, not a second payload write. This is a policy bound, not
// a claim of exact hardware IO, CPU, provider work, or model billing.
type applicationFailure struct {
	Schema, RequestSHA256, ProductApplication, ScopeSHA256, ProducerConfigSHA256 string
	Phase, Class, ContextStatus                                                  string
	RunSessionExit                                                               *int
	StopCalls, JoinCalls                                                         int
	CleanupStatus                                                                string
	StartupStatus, StartupSHA256                                                 string
	StartupAttemptID                                                             string
	ScopeFailure                                                                 applicationScopeFailure
}

// Detached accounting only: not origin authority, completion, or a discovery payload.
// MemberOrdinal is explicitly nullable; the admitted scope contains at most 16 members.
type applicationScopeFailure struct {
	Status, Stage, Class, Subcause, DiscoveryAvailability string
	MemberOrdinal                                         *int
}

func unavailableApplicationScopeFailure() applicationScopeFailure {
	return applicationScopeFailure{Status: "OBSERVATION_UNAVAILABLE", Stage: "UNKNOWN", Class: "UNKNOWN", Subcause: "UNKNOWN", DiscoveryAvailability: "UNKNOWN"}
}

func applicationScopeFailureStage(s string) bool {
	switch s {
	case "ADAPTER_ADMISSION", "ADAPTER_ACTIVE_LEASE", "ADAPTER_SOURCE_REVALIDATION", "ADAPTER_WORKSPACE_BINDING", "ADAPTER_METADATA", "ADAPTER_MEMBER_ACTIVE_LEASE", "ADAPTER_MEMBER_PREPARE", "ADAPTER_MEMBER_SUPPLY_VALIDATION", "ADAPTER_RETAINED_GUARD_BINDING", "ADAPTER_GUARD_REVALIDATION", "DISCOVERY_ADMISSION", "DOCUMENT_SYMBOL_DISCOVERY", "DISCOVERY_SERIALIZATION", "TERMINAL_DISCOVERY", "DISCOVERY_ACCOUNTING", "DISCOVERY_SCOPE_REVALIDATION", "DISCOVERY_BATCH_PLANNING", "DISCOVERY_SOURCE_BINDING", "DISCOVERY_QUERY_DECLARATION", "DISCOVERY_ORIGIN_ENCODING", "DISCOVERY_BATCH_BINDING", "DISCOVERY_FINAL_REVALIDATION":
		return true
	}
	return false
}

func boundedApplicationScopeFailure(f applicationScopeFailure) applicationScopeFailure {
	unavailable := unavailableApplicationScopeFailure()
	if f.Status != "OBSERVATION_AVAILABLE" || !applicationScopeFailureStage(f.Stage) || !applicationFailureClass(f.Class) {
		return unavailable
	}
	switch f.Subcause {
	case "UNKNOWN", "MANAGER_CLOSED", "SESSION_NOT_READY", "PROTOCOL_OWNED", "LEASE_CLOSING", "LEASE_TOKEN_MISMATCH", "LANGUAGE_ID_CONFLICT":
	default:
		return unavailable
	}
	switch f.DiscoveryAvailability {
	case "NOT_REACHED_ADAPTER", "NOT_REACHED_DISCOVERY", "AVAILABLE_ERROR_REPORT", "DISCOVERY_REPORT_UNAVAILABLE", "DISCOVERY_ENCODING_FAILED", "DISCOVERY_ENCODING_BOUND":
	default:
		return unavailable
	}
	if f.MemberOrdinal != nil {
		if *f.MemberOrdinal < 0 || *f.MemberOrdinal >= 16 {
			return unavailable
		}
		n := *f.MemberOrdinal
		f.MemberOrdinal = &n
	}
	return f
}

func (f *applicationFailure) observeScopeFailure(o pilot.ScopeFailureObservationPrivateV2, present bool) {
	f.ScopeFailure = unavailableApplicationScopeFailure()
	if present {
		f.ScopeFailure = boundedApplicationScopeFailure(applicationScopeFailure{Status: "OBSERVATION_AVAILABLE", Stage: o.Stage, Class: o.Class, Subcause: o.Subcause, DiscoveryAvailability: o.DiscoveryAvailability, MemberOrdinal: o.MemberOrdinal})
	}
}

func newApplicationFailure(r hostProductRequestV3) applicationFailure {
	return applicationFailure{Schema: applicationFailureSchemaV2, ScopeFailure: unavailableApplicationScopeFailure(), RequestSHA256: hostProductRequestDigestV3(r), ProductApplication: r.Configuration.Application, ScopeSHA256: r.Configuration.ScopeSHA256, ProducerConfigSHA256: r.Configuration.ProducerConfigSHA256, ContextStatus: "UNKNOWN", CleanupStatus: "UNKNOWN", StartupStatus: "UNAVAILABLE"}
}

func applicationFailurePhase(s string) bool {
	switch s {
	case "ORIGINAL_RETENTION_ROOTS", "ORIGINAL_SESSION_START", "ORIGINAL_SESSION_LEASE", "ORIGINAL_CENSUS_DISCOVERY", "ORIGINAL_DISCOVERY_BINDING", "ORIGINAL_CAPTURE_BATCH", "ORIGINAL_PAGED_OWNER", "ORIGINAL_PAGES", "ORIGINAL_CHECKPOINT_EXPORT", "ORIGINAL_TRANSFER":
		return true
	}
	return false
}
func applicationFailureClass(s string) bool {
	switch s {
	case "UNKNOWN", "CANCELLED", "DEADLINE_EXCEEDED", "INVALID_UNSPECIFIED", "DISCOVERY_INCOMPLETE", "DISCOVERY_EMPTY", "CAUSE_TRAVERSAL_BOUND", "REQUEST_CANCELLED", "REQUEST_TIMEOUT", "RESOURCE_EXHAUSTED", "DOCUMENT_SUPPLY_UNAVAILABLE", "DOCUMENT_SOURCE_UNAVAILABLE", "DOCUMENT_URI_UNAVAILABLE", "DOCUMENT_OUTSIDE_WORKSPACE", "LANGUAGE_ID_UNAVAILABLE", "LIFECYCLE_CONFLICT", "SESSION_POISONED", "SESSION_NOT_FOUND", "STALE_GENERATION", "RUNTIME_FAILURE_UNKNOWN":
		return true
	}
	return false
}
func validateApplicationFailure(raw []byte) error {
	if len(raw) == 0 || len(raw) > applicationFailureMaxBytes {
		return product.ErrInvalid
	}
	var f applicationFailure
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&f) != nil || d.Decode(new(any)) != io.EOF {
		return product.ErrInvalid
	}
	canonical, _ := json.Marshal(f)
	// Canonical closed object also rejects duplicates, aliases, omitted fields,
	// trailing data and case-insensitive JSON field matches.
	if !bytes.Equal(raw, canonical) || f.Schema != applicationFailureSchemaV2 || !validApplicationDigest(f.RequestSHA256) || !validApplicationDigest(f.ProductApplication) || !validApplicationDigest(f.ScopeSHA256) || !validApplicationDigest(f.ProducerConfigSHA256) || !applicationFailurePhase(f.Phase) || !applicationFailureClass(f.Class) || f.StopCalls < 0 || f.StopCalls > 1 || f.JoinCalls < 0 || f.JoinCalls > 1 || (f.RunSessionExit != nil && (*f.RunSessionExit < 0 || *f.RunSessionExit > 255)) {
		return product.ErrInvalid
	}
	switch f.ContextStatus {
	case "UNKNOWN", "ACTIVE", "CANCELLED", "DEADLINE_EXCEEDED":
	default:
		return product.ErrInvalid
	}
	switch f.CleanupStatus {
	case "UNKNOWN":
		if f.StopCalls != 0 || f.JoinCalls != 0 {
			return product.ErrInvalid
		}
	case "OBSERVED_OK", "OBSERVED_FAILED":
		if f.StopCalls != 1 || f.JoinCalls != 1 {
			return product.ErrInvalid
		}
	default:
		return product.ErrInvalid
	}
	if f.StartupAttemptID != "" && !validApplicationDigest("sha256:"+f.StartupAttemptID) {
		return product.ErrInvalid
	}
	switch f.StartupStatus {
	case "AVAILABLE":
		if !validApplicationDigest(f.StartupSHA256) || f.StartupAttemptID == "" {
			return product.ErrInvalid
		}
	case "UNAVAILABLE":
		if f.StartupSHA256 != "" {
			return product.ErrInvalid
		}
	default:
		return product.ErrInvalid
	}
	return nil
}

// Reads ONLY a fixed basename beneath one already-owned auxiliary directory.
// No caller-controlled selector, symlink parents, special file, hardlink, or
// unbounded read. Root/leaf identity is checked before and after the read.
func readApplicationFailureFile(rootPath, selector string, limit int) ([]byte, error) {
	if selector != applicationFailureSelector && selector != applicationStartupSelector {
		return nil, product.ErrInvalid
	}
	actual, e := filepath.EvalSymlinks(rootPath)
	st, se := os.Lstat(rootPath)
	if e != nil || se != nil || actual != rootPath || !st.IsDir() || st.Mode().Perm() != 0700 {
		return nil, product.ErrInvalid
	}
	root, e := os.OpenRoot(rootPath)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	held, e := root.Stat(".")
	if e != nil || !os.SameFile(st, held) {
		return nil, product.ErrInvalid
	}
	f, e := root.OpenFile(selector, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() <= 0 || info.Size() > int64(limit) || info.Sys().(*syscall.Stat_t).Nlink != 1 {
		return nil, product.ErrInvalid
	}
	if applicationPublicationLock(f) != nil {
		return nil, product.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	last, le := f.Stat()
	visible, ve := root.Lstat(selector)
	now, ne := os.Lstat(rootPath)
	actual, ae := filepath.EvalSymlinks(rootPath)
	if e != nil || le != nil || ve != nil || ne != nil || ae != nil || actual != rootPath || !os.SameFile(st, now) || now.Mode().Perm() != 0700 || !os.SameFile(last, visible) || applicationFileIdentity(info) != applicationFileIdentity(last) || int64(len(raw)) != info.Size() {
		return nil, product.ErrInvalid
	}
	return raw, nil
}

// The startup sink validates typed fields, but its public validator intentionally
// accepts JSON whitespace. Reject duplicate object keys separately before use.
func applicationDiagnosticUniqueJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 32 {
			return false
		}
		token, e := d.Token()
		if e != nil {
			return false
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				k, ok := key.(string)
				if e != nil || !ok || seen[strings.ToLower(k)] {
					return false
				}
				seen[strings.ToLower(k)] = true
				if !value(depth + 1) {
					return false
				}
			}
			end, e := d.Token()
			return e == nil && end == json.Delim('}')
		case '[':
			for d.More() {
				if !value(depth + 1) {
					return false
				}
			}
			end, e := d.Token()
			return e == nil && end == json.Delim(']')
		}
		return false
	}
	if !value(0) {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}

func applicationStartupMatches(raw []byte, attemptID string) bool {
	if !validApplicationDigest("sha256:"+attemptID) || !applicationDiagnosticUniqueJSON(raw) || manageddiagnostic.ValidateStartupDiagnostics(raw) != nil {
		return false
	}
	var d struct {
		Attempt struct {
			AttemptID string `json:"attempt_id"`
		} `json:"attempt"`
	}
	return json.Unmarshal(raw, &d) == nil && d.Attempt.AttemptID == attemptID
}

func publishApplicationFailure(ctx context.Context, r hostProductRequestV3, rootIdentity os.FileInfo, f applicationFailure, phase string, cause error, code *int, stops, joins int, cleanupOK bool, startupOwned bool) error {
	f.ScopeFailure = boundedApplicationScopeFailure(f.ScopeFailure)
	f.Phase, f.Class = phase, pilot.ScopeFailureClassPrivateV2(cause)
	f.RunSessionExit, f.StopCalls, f.JoinCalls = code, stops, joins
	if ctx.Err() == nil {
		f.ContextStatus = "ACTIVE"
	} else if ctx.Err() == context.Canceled {
		f.ContextStatus = "CANCELLED"
	} else if ctx.Err() == context.DeadlineExceeded {
		f.ContextStatus = "DEADLINE_EXCEEDED"
	}
	if stops == 1 && joins == 1 {
		f.CleanupStatus = "OBSERVED_FAILED"
		if cleanupOK {
			f.CleanupStatus = "OBSERVED_OK"
		}
	}
	st, e := os.Lstat(r.AuxiliaryRoot)
	canonical, canonicalErr := filepath.EvalSymlinks(r.AuxiliaryRoot)
	if e != nil || canonicalErr != nil || canonical != r.AuxiliaryRoot || rootIdentity == nil || !os.SameFile(rootIdentity, st) {
		return product.ErrInvalid
	}
	if startupOwned {
		raw, e := readApplicationFailureFile(r.AuxiliaryRoot, applicationStartupSelector, manageddiagnostic.StartupDiagnosticsMaxBytes)
		if e == nil && applicationStartupMatches(raw, f.StartupAttemptID) {
			f.StartupStatus = "AVAILABLE"
			f.StartupSHA256 = pilot.Digest(raw)
		}
	}
	raw, e := json.Marshal(f)
	if e != nil {
		return e
	}
	// Background publication deliberately survives context expiry: finite private
	// metadata only, no provider invocation, continuation, lease, or ledger reuse.
	return manageddiagnostic.PublishHardened(r.AuxiliaryRoot, applicationFailureSelector, raw, validateApplicationFailure)
}

type applicationFailureProjection struct {
	Status, Phase, Class, ContextStatus, StartupStatus, StartupOutcome, StartupRecordsStatus, StartupPhase, StartupTerminal string
	ScopeFailure                                                                                                            applicationScopeFailure
}

func loadApplicationFailure(unitRoot string, unitIdentity os.FileInfo, source hostProductRequestV3) applicationFailureProjection {
	unavailable := applicationFailureProjection{ScopeFailure: unavailableApplicationScopeFailure(), Status: "UNAVAILABLE", Phase: "UNKNOWN", Class: "UNKNOWN", ContextStatus: "UNKNOWN", StartupStatus: "UNAVAILABLE", StartupOutcome: "UNKNOWN", StartupRecordsStatus: "UNAVAILABLE", StartupPhase: "UNKNOWN", StartupTerminal: "UNKNOWN"}
	st, e := os.Lstat(unitRoot)
	if e != nil || unitIdentity == nil || !os.SameFile(st, unitIdentity) || !st.IsDir() || st.Mode().Perm() != 0700 || source.AuxiliaryRoot != filepath.Join(unitRoot, "auxiliary") {
		return unavailable
	}
	raw, e := readApplicationFailureFile(source.AuxiliaryRoot, applicationFailureSelector, applicationFailureMaxBytes)
	if e != nil || validateApplicationFailure(raw) != nil {
		return unavailable
	}
	var f applicationFailure
	_ = json.Unmarshal(raw, &f)
	want := newApplicationFailure(source)
	if f.RequestSHA256 != want.RequestSHA256 || f.ProductApplication != want.ProductApplication || f.ScopeSHA256 != want.ScopeSHA256 || f.ProducerConfigSHA256 != want.ProducerConfigSHA256 {
		return unavailable
	}
	p := unavailable
	p.Status = "AVAILABLE"
	p.Phase = f.Phase
	p.Class = f.Class
	p.ContextStatus = f.ContextStatus
	// Unknown/future detached labels do not erase a valid general receipt.
	p.ScopeFailure = boundedApplicationScopeFailure(f.ScopeFailure)
	if f.StartupStatus == "AVAILABLE" {
		startup, e := readApplicationFailureFile(source.AuxiliaryRoot, applicationStartupSelector, manageddiagnostic.StartupDiagnosticsMaxBytes)
		if e == nil && pilot.Digest(startup) == f.StartupSHA256 && applicationStartupMatches(startup, f.StartupAttemptID) {
			var d struct {
				Attempt       struct{ Outcome string }           `json:"attempt"`
				RecordsStatus string                             `json:"records_status"`
				Records       []struct{ Phase, Terminal string } `json:"records"`
			}
			if json.Unmarshal(startup, &d) == nil {
				p.StartupStatus = "AVAILABLE"
				if d.RecordsStatus == "available" {
					p.StartupRecordsStatus = "AVAILABLE"
				}
				switch d.Attempt.Outcome {
				case "failed":
					p.StartupOutcome = "STARTUP_FAILED"
				case "admitted":
					p.StartupOutcome = "STARTUP_ADMITTED"
				}
				// Terminal labels have already passed the existing closed phase rules.
				for _, r := range d.Records {
					if r.Terminal != "response-received" {
						p.StartupPhase = r.Phase
						p.StartupTerminal = r.Terminal
						break
					}
				}
			}
		}
	}
	last, e := os.Lstat(unitRoot)
	if e != nil || !os.SameFile(unitIdentity, last) {
		return unavailable
	}
	return p
}
func reportApplicationFailure(w io.Writer, unitRoot string, identity os.FileInfo, source hostProductRequestV3) {
	p := loadApplicationFailure(unitRoot, identity, source)
	ordinal := "UNKNOWN"
	if p.ScopeFailure.MemberOrdinal != nil {
		ordinal = fmt.Sprintf("%d", *p.ScopeFailure.MemberOrdinal)
	}
	fmt.Fprintf(w, "private application: SafeDiagnosticStatus=%s Phase=%s Class=%s ContextStatus=%s StartupStatus=%s StartupOutcome=%s StartupRecordsStatus=%s StartupPhase=%s StartupTerminal=%s DetachedStatus=%s DetachedStage=%s DetachedClass=%s DetachedSubcause=%s DiscoveryAvailability=%s MemberOrdinal=%s\n", p.Status, p.Phase, p.Class, p.ContextStatus, p.StartupStatus, p.StartupOutcome, p.StartupRecordsStatus, p.StartupPhase, p.StartupTerminal, p.ScopeFailure.Status, p.ScopeFailure.Stage, p.ScopeFailure.Class, p.ScopeFailure.Subcause, p.ScopeFailure.DiscoveryAvailability, ordinal)
}
