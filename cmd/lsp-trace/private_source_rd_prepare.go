//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/inventorysuccessor/sourceoperator"
	"lsp-trace/internal/managedprocess"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

const privateRDURI = "file:///Users/schwa/dev/lsp-trace-variable-v2-TSZxdY/internal/graph/graph.go"
const privateRDWorkspace = "/Users/schwa/dev/lsp-trace-variable-v2-TSZxdY"
const privateRDDigest = "sha256:73cf775bec54f73c3a5465e5ba84371980d2f2a27725350ecf2c83c49761a761"

// Closed, data-only bootstrap, using the existing prepaid canonical owner/profile
// admission contract. No serialized contract, selector, timing, host, output path,
// body, future identifier, evaluator, callback or native grant can be supplied.
// Deadline and RetainUntil are absolute original assertions, never renewed.
type privateRDConfig struct {
	Schema                                       string
	AccountingRoot                               string
	CanonicalJob                                 []byte
	ExpectedHead                                 string
	Verification                                 inv.PrivateManagedInputVerificationV2
	Profile                                      runtimeprofile.Selector
	LanguageID, Provider, ProcessPath, Workspace string
	ProcessArgs                                  []string
	URI, Revision, Privacy, OccurrenceID         string
	RunID, SeedNamespace, Lineage                string
	Seed                                         uint64
	Attempts                                     [3]string
	ContractInput                                cc.ContractInput
	SourceLimits                                 tp.PrivateSourceLimits
	Deadline, RetainUntil                        time.Time
}

// Explicit caller delta, NOT a native grant. A comparison step is one fixed
// predicate/group of scalar coordinates or one bounded string equality. A node
// is one logical input/result/report record visited, not a Go allocation.
// Equal-length string comparison charges BOTH operands before equality; a length
// refusal does not compare content. No spare credit is transferred to reporting.
type privateRDBudget struct {
	bytes, steps, nodes          uint64
	reportBytes, scalars, writes uint64
	failed                       bool
}

func privateRDCharge(used *uint64, n, limit uint64) bool {
	if *used > limit || n > limit-*used {
		return false
	}
	*used += n
	return true
}
func (b *privateRDBudget) step() bool {
	if b.failed || !privateRDCharge(&b.steps, 1, 256) {
		b.failed = true
		return false
	}
	return true
}
func (b *privateRDBudget) node() bool {
	if b.failed || !privateRDCharge(&b.nodes, 1, 32) {
		b.failed = true
		return false
	}
	return true
}
func (b *privateRDBudget) equal(a, c string) bool {
	if !b.step() || len(a) > 4096 || len(c) > 4096 || len(a) != len(c) {
		return false
	}
	n := uint64(len(a)) + uint64(len(c)) // both lengths bounded before addition
	if !privateRDCharge(&b.bytes, n, 4096) {
		b.failed = true
		return false
	}
	return a == c
}

// Bootstrap shape walk is bounded before NewContinuationContract serializes its
// data. It preserves nil versus empty source constraints, without normalizing,
// filling, cloning, or manufacturing opaque native admission. This is the same
// bounded metadata domain admitted by the selected SDK constructor (128 values,
// depth 8, <=16 entries, <=256 per string and <=1024 aggregate string bytes).
func privateRDContractShape(v reflect.Value, values, strings *int, depth int) bool {
	if depth > 8 || *values == 0 {
		return false
	}
	*values--
	switch v.Kind() {
	case reflect.String:
		if v.Len() > 256 || v.Len() > *strings {
			return false
		}
		*strings -= v.Len()
		return true
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Struct:
		if v.NumField() > 32 {
			return false
		}
		for i := 0; i < v.NumField(); i++ {
			if !privateRDContractShape(v.Field(i), values, strings, depth+1) {
				return false
			}
		}
		return true
	case reflect.Array, reflect.Slice:
		if v.Len() > 16 || v.Len() > *values {
			return false
		}
		for i := 0; i < v.Len(); i++ {
			if !privateRDContractShape(v.Index(i), values, strings, depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
func decodePrivateRDConfig(raw []byte) (privateRDConfig, bool) {
	var c privateRDConfig
	if len(raw) == 0 || len(raw) > 64<<10 {
		return c, false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return privateRDConfig{}, false
	}
	canonical, e := json.Marshal(c)
	// Existing caller-prepaid bounded canonical bootstrap comparison, not a new
	// post-selection allowance and not a scan of source Body or graph.go.
	if e != nil || !bytes.Equal(bytes.TrimSuffix(raw, []byte("\n")), canonical) {
		return privateRDConfig{}, false
	}
	if c.Schema != "private-source-prepare-rd.v1" || len(c.CanonicalJob) == 0 || len(c.CanonicalJob) > 32<<10 || !filepath.IsAbs(c.AccountingRoot) || filepath.Clean(c.AccountingRoot) != c.AccountingRoot || !filepath.IsAbs(c.ProcessPath) || len(c.ProcessPath) > 4096 || len(c.ProcessArgs) > 64 || c.Workspace != privateRDWorkspace || c.Profile.Workspace != c.Workspace || c.URI != privateRDURI || c.Deadline.IsZero() || c.RetainUntil.Before(c.Deadline) {
		return privateRDConfig{}, false
	}
	for _, s := range c.ProcessArgs {
		if len(s) > 4096 {
			return privateRDConfig{}, false
		}
	}
	if len(c.Profile.Options) > 16 {
		return privateRDConfig{}, false
	}
	for _, o := range c.Profile.Options {
		if len(o.Name) > 256 || len(o.Value) > 256 {
			return privateRDConfig{}, false
		}
	}
	for _, s := range []string{c.Profile.TrustDomain, c.Profile.Workspace, c.Profile.Profile, c.Profile.EnvironmentReference, c.LanguageID, c.Provider, c.Revision, c.Privacy, c.OccurrenceID, c.RunID, c.SeedNamespace, c.Lineage} {
		if len(s) > 256 {
			return privateRDConfig{}, false
		}
	}
	for i, a := range c.Attempts {
		if len(a) != 64 {
			return privateRDConfig{}, false
		}
		for _, v := range a {
			if !(v >= '0' && v <= '9' || v >= 'a' && v <= 'f') {
				return privateRDConfig{}, false
			}
		}
		for j := 0; j < i; j++ {
			if a == c.Attempts[j] {
				return privateRDConfig{}, false
			}
		}
	}
	values, strings := 128, 1024
	if !privateRDContractShape(reflect.ValueOf(c.ContractInput), &values, &strings, 0) {
		return privateRDConfig{}, false
	}
	return c, true
}

// DATA ONLY. Positive DTO tests of this function do not qualify an actual native
// result. Actual full-definition correctness and whole-unit/request Matches are
// established by the existing opaque SDK admission, not this tuple comparison.
// Body is never compared, sliced, hashed, serialized, scanned or copied here.
func privateRDTargetData(b *privateRDBudget, c privateRDConfig, endpoint sc.PrivateSelectedEndpointV2, u tp.PrivateSourceUnit) bool {
	if !b.node() || !b.step() || endpoint.RowPosition < 0 {
		return false
	}
	if !b.equal(endpoint.Slot.Mode, "RD") || !b.equal(endpoint.Intent.Endpoint.Kind, "RESOLVES_TO_DEFINITION") || !b.equal(endpoint.Intent.Endpoint.Role, "DEFINITION_TARGET") {
		return false
	}
	if !b.equal(u.URI, privateRDURI) || !b.equal(u.SourceDigest, privateRDDigest) || !b.equal(u.Revision, c.Revision) || !b.equal(u.Privacy, c.Privacy) || !b.equal(u.Workspace, c.Workspace) || !b.equal(u.PositionEncoding, "utf-16") {
		return false
	}
	s, d := u.SelectionRange, u.DisplayRange
	// Exact immutable target tuple is non-query (query is zero-based 33:1).
	if !b.step() || !(s.Start.Line == 21 && s.Start.Character == 5 && s.End.Line == 21 && s.End.Character == 9) {
		return false
	}
	if !b.step() || !(s.Start.Line != 33 || s.Start.Character != 1) {
		return false
	}
	// Lexicographic native display containment; do NOT guess a display literal.
	if !b.step() || !(d.Start.Line < d.End.Line || d.Start.Line == d.End.Line && d.Start.Character < d.End.Character) {
		return false
	}
	if !b.step() || !(d.Start.Line < s.Start.Line || d.Start.Line == s.Start.Line && d.Start.Character <= s.Start.Character) {
		return false
	}
	if !b.step() || !(d.End.Line > s.End.Line || d.End.Line == s.End.Line && d.End.Character >= s.End.Character) {
		return false
	}
	return b.step() && len(u.Body) > 0 && len(u.Body) <= c.SourceLimits.MaxDocumentBytes
}
func privateRDSelected(b *privateRDBudget, c privateRDConfig, r sc.PrivateSelectedResultV2) bool {
	if !b.step() || !r.Available() {
		return false
	}
	if !b.node() || !b.step() {
		return false
	}
	endpoint, u, _, ok := r.Selected() // by-value structs/string headers, not deep copies
	return ok && privateRDTargetData(b, c, endpoint, u)
}

type privateRDNative uint8

const (
	privateRDNotInvoked privateRDNative = iota
	privateRDNativeFailed
	privateRDNativeAvailable
)

type privateRDTrace uint8

const (
	privateRDTraceNotRequested privateRDTrace = iota
	privateRDTraceComplete
	privateRDTraceInvalid
)

type privateRDOutcome struct {
	native                   privateRDNative
	qualified, cleanup, live bool
	deadline                 time.Time
}

func privateRDFinalState(ctx context.Context, deadline time.Time, out privateRDOutcome, cleanup bool) privateRDOutcome {
	out.cleanup = cleanup
	out.live = ctx != nil && ctx.Err() == nil && time.Now().Before(deadline)
	out.qualified = out.qualified && out.cleanup && out.live
	out.deadline = deadline
	return out
}

// No errors, payload, URI, path, ID, hash, source, opaque methods or cause graph
// can enter this closed eleven-scalar report. The Write is attempted ONCE and
// charged before IO. Any short/error write is terminal; written bytes remain.
func privateRDReport(w io.Writer, b *privateRDBudget, out privateRDOutcome, tr privateRDTrace) int {
	// Reserve the closed formatting path BEFORE projection/encoding: eleven
	// scalar projections, two enum projections, one encoding, one IO handoff,
	// and one terminal status predicate. No content-dependent traversal exists.
	// 512 is a conservative encoded bound, not credit obtained from an R report.
	if b.failed || !privateRDCharge(&b.steps, 16, 256) || !b.node() || !privateRDCharge(&b.scalars, 11, 16) || !privateRDCharge(&b.reportBytes, 512, 4096) || !privateRDCharge(&b.writes, 1, 1) {
		return 1
	}
	native := "NOT_INVOKED"
	switch out.native {
	case privateRDNativeFailed:
		native = "FAILED"
	case privateRDNativeAvailable:
		native = "AVAILABLE"
	}
	traceState := "NOT_REQUESTED"
	switch tr {
	case privateRDTraceComplete:
		traceState = "COMPLETE"
	case privateRDTraceInvalid:
		traceState = "INVALID_INCOMPLETE"
	}
	report := struct {
		Schema, Native, Trace                    string
		Qualified, Cleanup                       bool
		Authority                                int
		Accepted                                 bool
		Consumer, Feature, Completeness, Absence string
	}{"private-rd-report.v1", native, traceState, out.qualified && tr != privateRDTraceInvalid, out.cleanup, 0, false, "UNRESOLVED", "UNRESOLVED", "UNKNOWN", "PROHIBITED"}
	raw, e := json.Marshal(report)
	if e != nil || len(raw)+1 > 512 {
		return 1
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e != nil || n != len(raw) {
		return 1
	}
	if !out.qualified || tr == privateRDTraceInvalid {
		return 1
	}
	return 0
}

func executePrivateRD(ctx context.Context, c privateRDConfig, data io.Closer, b *privateRDBudget) (out privateRDOutcome) {
	var root *publication.Root
	var manager *sessionruntime.Manager
	control, cancel := context.WithDeadline(ctx, c.Deadline)
	defer cancel()
	defer func() {
		clean := true
		if manager != nil {
			if manager.Shutdown(control) != nil {
				clean = false
			}
		}
		if root != nil {
			if root.Close() != nil {
				clean = false
			}
		}
		if data != nil {
			if data.Close() != nil {
				clean = false
			}
		}
		out = privateRDFinalState(ctx, c.Deadline, out, clean)
	}()
	now := time.Now()
	if ctx.Err() != nil || !now.Before(c.Deadline) || c.Deadline.Sub(now) > 15*time.Second || !now.Before(c.RetainUntil) || c.RetainUntil.Sub(now) > 30*time.Minute {
		return out
	}
	var e error
	root, e = publication.OpenRoot(c.AccountingRoot)
	if e != nil {
		return out
	}
	owner, e := inv.OpenPrivateManagedInputOwnerV2(control, root, c.CanonicalJob, c.ExpectedHead, c.Verification)
	if e != nil {
		return out
	}
	selected, e := runtimeprofile.Validate(c.Profile)
	if e != nil {
		return out
	}
	// Same public constructor as selected SDK tests; JSON cannot restore Contract.
	contract, e := cc.NewContinuationContract(c.ContractInput)
	if e != nil {
		return out
	}
	starter, e := productionCensusRunnerDependencies().newStarter()
	if e != nil {
		return out
	}
	manager, e = sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 16, MaxChildren: 16, MaxCancels: 8, MaxTombstones: 32, MaxObservations: 256, MaxOperations: 8}, Starter: starter})
	if e != nil {
		return out
	}
	start := sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.LanguageID, RelationProviders: []string{c.Provider}, Deadline: c.Deadline, Process: managedprocess.Spec{Path: c.ProcessPath, Dir: c.Workspace, Args: c.ProcessArgs}}
	capture := inv.PrivateMixedInputRequestV2{Manager: manager, Root: root, Start: start, URI: c.URI, OccurrenceID: c.OccurrenceID, Revision: c.Revision, Privacy: c.Privacy, OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 33, Character: 1, Kinds: []pa.RelationKind{pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}}
	config := inv.PrivateSelectedConfigurationV2{RunID: c.RunID, SeedNamespace: c.SeedNamespace, Lineage: c.Lineage, Seed: c.Seed, Attempts: c.Attempts, Mode: "RD", Contract: contract, ContractInput: c.ContractInput, SourceLimits: c.SourceLimits}
	// Precharge the fixed native availability observation before invoking SDK;
	// budget refusal here cannot relabel a completed native capture as failed.
	if !b.step() {
		return out
	}
	out.native = privateRDNativeFailed
	r, e := sourceoperator.PrepareRDTargetV2(control, owner, capture, config) // sole native invocation; no retry
	if r.Available() {
		out.native = privateRDNativeAvailable
	}
	if e == nil {
		out.qualified = privateRDSelected(b, c, r)
	}
	return out
}

// Reads only the explicitly supplied 0600 bootstrap config, never source. Owned
// data cleanup occurs before the caller can report any result, on every path.
func privateRDNativeCLI(ctx context.Context, args []string, b *privateRDBudget) (out privateRDOutcome) {
	if len(args) != 2 || args[0] != "source-prepare-rd" {
		return out
	}
	fd, e := syscall.Open(args[1], syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return out
	}
	f := os.NewFile(uintptr(fd), "private-rd-config")
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 64<<10 {
		out.cleanup = f.Close() == nil
		return out
	}
	raw, e := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	c, ok := decodePrivateRDConfig(raw)
	if e != nil || !ok {
		out.cleanup = f.Close() == nil
		return out
	}
	return executePrivateRD(ctx, c, f, b)
}
