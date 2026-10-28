//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/acquisitionmanifest"
	orchestration "lsp-trace/internal/acquisitionorchestration"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// This file is intentionally an external Unix test, not an executable/public
// dispatch route. No package production import can create the private adapter.
func scopedLiveEnvironmentPrivate(get func(string) string) (string, bool, error) {
	switch get("LSP_TRACE_SCOPE_REHEARSAL") {
	case "":
		return "", false, nil
	case "one-native-v5":
		p := get("LSP_TRACE_SCOPE_REHEARSAL_CONFIG")
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return "", true, errors.New("explicit canonical absolute config required")
		}
		return p, true, nil
	default:
		return "", false, errors.New("invalid private rehearsal sentinel")
	}
}
func TestScopedBatchLiveConstituentRehearsalPrivate(t *testing.T) {
	path, on, err := scopedLiveEnvironmentPrivate(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Skip("opt-in only: LSP_TRACE_SCOPE_REHEARSAL=one-native-v5")
	}
	// No ambient defaults or roots are created. A bounded canonical document pins
	// process environment explicitly, rather than inheriting os.Environ drift.
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, pilot.ScopedRehearsalConfigLimitPrivate+1))
	err = errors.Join(readErr, file.Close())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := pilot.ParseScopedRehearsalConfigPrivate(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	// cwd is the package directory in this test binary, not the retained workspace.
	head, err := cmd.Output()
	if err != nil || string(head) != cfg.CodeHEAD+"\n" {
		t.Fatalf("exact executable checkout HEAD mismatch: %v", err)
	}
	for _, args := range [][]string{{"diff", "--quiet"}, {"diff", "--cached", "--quiet"}} {
		if err := exec.CommandContext(ctx, "git", args...).Run(); err != nil {
			t.Fatal("live rehearsal requires clean reviewed executable checkout:", err)
		}
	}
	untracked, err := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard").Output()
	if err != nil || len(untracked) != 0 {
		t.Fatal("live rehearsal refuses untracked executable checkout", err)
	}
	// Every root/finite budget check precedes even protocol initialization.
	h, err := pilot.OpenScopedRehearsalPrivate(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			t.Error("output close uncertainty:", err)
		}
	}()
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	m, err := sessionruntime.New(sessionruntime.Config{Starter: sessionruntime.ManagedStarter{Manager: supervisor}, Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 1024, MaxOperations: 2}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Shutdown(context.Background()); err != nil {
			t.Error("runtime shutdown uncertainty:", err)
		}
	}()
	v, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: cfg.TrustDomain, Workspace: cfg.Workspace, Profile: cfg.Profile, EnvironmentReference: cfg.EnvironmentReference})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(v), LanguageID: "go", Process: managedprocess.Spec{Path: cfg.Server, Args: cfg.Args, Dir: cfg.Workspace, Env: cfg.Environment}})
	if started.Failure != "" || started.SessionID != cfg.Session.SessionID || started.Generation != cfg.Session.Generation {
		t.Fatalf("exact session start mismatch: %+v", started)
	}
	ready := m.BeginReadiness(ctx, started.SessionID, started.Generation, time.Now().Add(5*time.Second))
	ready, ok := m.WaitReadiness(ctx, ready.ID)
	if _, err := validateScopedStartupPrivate(ctx, m, cfg, ready, ok); err != nil {
		t.Fatal(err)
	}
	report, err := executeScopedRehearsalPrivate(ctx, h, m, cfg)
	output, encodeErr := json.Marshal(report)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}

// Readiness is initialize-response evidence; process-spec identity is supplied
// separately by exact-generation Manager.Metadata. Both live and fake execution
// must traverse this validator before the execution owner can be claimed.
func validateScopedStartupPrivate(ctx context.Context, m *sessionruntime.Manager, cfg pilot.ScopedRehearsalConfigPrivate, ready sessionruntime.ReadinessSnapshot, found bool) (sessionruntime.SessionMetadata, error) {
	if ctx == nil || m == nil || !found || ready.Failure != "" || ready.State != sessionruntime.ReadinessReady || ready.SessionID != cfg.Session.SessionID || ready.Generation != cfg.Session.Generation {
		return sessionruntime.SessionMetadata{}, errors.New("private startup exact READY snapshot required")
	}
	metadata, failure := m.Metadata(cfg.Session.SessionID, cfg.Session.Generation)
	if failure != "" || metadata.ServerCommand != cfg.Server {
		return sessionruntime.SessionMetadata{}, fmt.Errorf("private startup exact-generation server metadata required: %s", failure)
	}
	if err := m.WithReadySessionLease(ctx, sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(context.Context) error { return nil }); err != nil {
		return sessionruntime.SessionMetadata{}, err
	}
	return metadata, nil
}

type scopedLiveReportPrivate struct {
	Schema                string                               `json:"schema"`
	Authority             int                                  `json:"authority"`
	Accepted              bool                                 `json:"accepted"`
	Completeness          string                               `json:"completeness"`
	FeatureIdentity       string                               `json:"feature_identity"`
	RevisionQualification string                               `json:"revision_qualification"`
	Outcome               string                               `json:"outcome"`
	AcquireCalls          int                                  `json:"acquire_calls"`
	Constituents          int                                  `json:"core_admitted_constituents"`
	Failure               string                               `json:"failure,omitempty"`
	CloseFailure          string                               `json:"store_proof_close_failure,omitempty"`
	Runtime               sessionruntime.SessionMetadata       `json:"runtime"`
	EffectiveBudget       any                                  `json:"effective_budget"`
	Records               []pilot.ScopedRehearsalRecordPrivate `json:"records"`
	// This receipt lives OUTSIDE the immutable outcome object; no recursive receipt.
	OutcomeReceipt *pilot.ScopedRehearsalRecordPrivate `json:"outcome_receipt,omitempty"`
}

type scopedLiveAcquirerPrivate struct {
	h       *pilot.ScopedRehearsalPrivate
	manager *sessionruntime.Manager
	calls   int
	request []byte
	raw     []byte
	failure error
}

func (a *scopedLiveAcquirerPrivate) AcquireV5(ctx context.Context, b censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	if a.calls != 0 {
		return censusacquisition.AcquiredV5{}, errors.New("private rehearsal prohibits second Acquire")
	}
	a.calls++
	if len(b.Targets) == 0 || len(b.Targets) > 63 || b.DownDepth != 1 || b.UpDepth != 1 {
		return censusacquisition.AcquiredV5{}, errors.New("private one-batch policy")
	}
	manifest := b.AcquisitionManifest(acquisitionmanifest.Limits{})
	request := orchestration.PlannedBatchRequest{SessionID: b.Session.SessionID, Generation: b.Session.Generation, RequestID: b.BatchID, Manifest: manifest, CanonicalSeedsV2: b.CanonicalSeedsV2}
	var err error
	a.request, err = json.Marshal(request)
	if err != nil {
		return censusacquisition.AcquiredV5{}, err
	}
	if len(a.request) > pilot.ScopedRehearsalBudgetFixedPrivate().ArtifactBytes {
		return censusacquisition.AcquiredV5{}, errors.New("canonical request serialized cap before producer")
	}
	selection, err := a.h.Mint(ctx, b.BatchID, manifest, b.CanonicalSeedsV2)
	if err != nil {
		return censusacquisition.AcquiredV5{}, err
	}
	out, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, a.manager, request, selection)
	a.raw = out.RawV5 // Exact returned bytes, including failure-adjacent output; no repair.
	if failure != nil {
		a.failure = failure.Err
		if a.failure == nil {
			a.failure = errors.New(string(failure.Code))
		}
		return censusacquisition.AcquiredV5{}, a.failure
	}
	if len(a.raw) > pilot.ScopedRehearsalBudgetFixedPrivate().ArtifactBytes {
		return censusacquisition.AcquiredV5{}, errors.New("RawV5 serialized cap after producer; no heap-bound claim")
	}
	if !out.BoundedTraversalComplete {
		return censusacquisition.AcquiredV5{}, errors.New("producer traversal incomplete")
	}
	return censusacquisition.AcquiredV5{Session: b.Session, Raw: out.RawV5}, nil
}

func scopedDiscoveryPolicyPrivate(d censusacquisition.Discovery) error {
	if d.Closure != censusacquisition.DiscoveryClosureSuccessful || len(d.ProviderFailures) != 0 || len(d.IncompleteCauses) != 0 || d.Accounting.Validate() != nil || (len(d.Targets) != 0 && !d.Complete) {
		return censusacquisition.ErrDiscoveryIncomplete
	}
	if len(d.Targets) > 63 {
		return errors.New("private policy refuses >63 targets before Core acquisition")
	}
	return nil
}
func executeScopedRehearsalPrivate(ctx context.Context, h *pilot.ScopedRehearsalPrivate, m *sessionruntime.Manager, cfg pilot.ScopedRehearsalConfigPrivate) (report scopedLiveReportPrivate, err error) {
	report = scopedLiveReportPrivate{Schema: "lsp-trace.scoped-rehearsal-observation.private.v1", Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", RevisionQualification: "CALLER_ASSERTED_NOT_AUTHENTICATED", Outcome: "INDETERMINATE", EffectiveBudget: struct {
		Output                                                                           pilot.ScopedRehearsalBudgetPrivate
		LifetimeSeconds, RequestSeconds, Requests, CaptureBytes, Messages, ResponseBytes int
	}{cfg.Budget, 30, 5, 256, 8 << 20, 64, 2 << 20}}
	ready := m.BeginReadiness(ctx, cfg.Session.SessionID, cfg.Session.Generation, time.Now().Add(5*time.Second))
	ready, found := m.WaitReadiness(ctx, ready.ID)
	var startupErr error
	report.Runtime, startupErr = validateScopedStartupPrivate(ctx, m, cfg, ready, found)
	if startupErr != nil {
		report.Failure = startupErr.Error()
		return report, startupErr
	}
	retain := func(name string, raw []byte) {
		r := h.Retain(ctx, name, raw)
		report.Records = append(report.Records, r)
		if !r.Verified {
			err = errors.Join(err, fmt.Errorf("%s retention indeterminate: %s", name, r.Error))
		}
	}
	configRaw, e := json.Marshal(cfg)
	if e != nil {
		return report, e
	}
	retain("configuration", configRaw)
	retain("input-observation", h.InputObservation())
	var d censusacquisition.Discovery
	var projection censusacquisition.Projection
	acquirer := &scopedLiveAcquirerPrivate{h: h, manager: m}
	operationErr := m.WithReadySessionLease(ctx, sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(leased context.Context) error {
		if err != nil {
			return err
		} // Output preflight failure never authorizes semantic traffic.
		var bindErr error
		_, bindErr = h.Bind(leased, m)
		if bindErr != nil {
			return bindErr
		}
		d, bindErr = h.Discover(leased, cfg.Session)
		if bindErr != nil {
			return bindErr
		}
		if bindErr = scopedDiscoveryPolicyPrivate(d); bindErr != nil {
			return bindErr
		}
		discoveryRaw, encodeErr := json.Marshal(d)
		if encodeErr != nil || len(discoveryRaw) > cfg.Budget.ArtifactBytes {
			return errors.Join(encodeErr, errors.New("discovery serialized cap before Core acquisition"))
		}
		if len(d.Targets) == 0 {
			return nil
		}
		core := censusacquisition.Core{Discoverer: fixedDiscovery{d}, Acquirer: acquirer, Planning: &censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 63}}
		projection, bindErr = core.Run(leased, cfg.Session)
		return bindErr
	})
	report.Runtime, _ = m.Metadata(cfg.Session.SessionID, cfg.Session.Generation)
	report.AcquireCalls = acquirer.calls
	report.Constituents = len(projection.Constituents)
	if claim := h.ClaimObservation(); len(claim) > 0 {
		retain("claim", claim)
	}
	if d.Session.Validate() == nil {
		raw, e := json.Marshal(d)
		if e != nil {
			err = errors.Join(err, e)
		} else {
			retain("discovery", raw)
		}
	}
	if len(acquirer.request) > 0 {
		retain("canonical-request-seeds", acquirer.request)
	}
	if len(acquirer.raw) > 0 {
		retain("exact-native-v5", acquirer.raw)
	}
	if operationErr == nil {
		if len(d.Targets) == 0 && acquirer.calls == 0 && len(projection.Constituents) == 0 {
			report.Outcome = "EMPTY"
		} else if acquirer.calls == 1 && len(projection.Constituents) == 1 && bytes.Equal(projection.Constituents[0].Raw, acquirer.raw) {
			report.Outcome = "ONE_CORE_ADMITTED_CONSTITUENT"
		} else {
			operationErr = errors.New("one constituent admission invariant")
		}
		if len(projection.ManifestBytes) > 0 {
			retain("core-manifest", projection.ManifestBytes)
		}
	}
	err = errors.Join(err, operationErr)
	if closeErr := h.CloseExecution(); closeErr != nil {
		report.CloseFailure = closeErr.Error()
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		report.Outcome = "INDETERMINATE"
		report.Failure = err.Error()
	}
	// Fixed terminal observation links already available receipts; no self receipt.
	raw, e := json.Marshal(report)
	if e != nil || len(raw) > 256<<10 {
		return report, errors.Join(err, e, errors.New("outcome serialized cap"))
	}
	receipt := h.Retain(ctx, "outcome", raw)
	report.OutcomeReceipt = &receipt
	if !receipt.Verified {
		report.Outcome = "INDETERMINATE"
		err = errors.Join(err, errors.New("outcome persistence indeterminate"))
		report.Failure = err.Error()
	}
	return report, err
}

func TestScopedLiveRehearsalSentinelPrivate(t *testing.T) {
	for _, value := range []string{"", "one-native-v5", "1", "one-native-v5 "} {
		_, on, err := scopedLiveEnvironmentPrivate(func(k string) string {
			if k == "LSP_TRACE_SCOPE_REHEARSAL" {
				return value
			}
			return "/explicit/config.json"
		})
		if value == "" && (on || err != nil) || value == "one-native-v5" && (!on || err != nil) || value != "" && value != "one-native-v5" && err == nil {
			t.Fatalf("sentinel %q", value)
		}
	}
}
func TestScopedLiveRehearsalDefaultFakePrivate(t *testing.T) {
	h, m, cfg, f := pilot.NewScopedRehearsalFixturePrivate(t)
	f.Fault("rehearsal-empty-directions")
	report, err := executeScopedRehearsalPrivate(context.Background(), h, m, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Outcome != "ONE_CORE_ADMITTED_CONSTITUENT" || report.AcquireCalls != 1 || report.Constituents != 1 || report.OutcomeReceipt == nil || !report.OutcomeReceipt.Verified {
		t.Fatalf("ASSERT_REHEARSAL_ONE_ACTUAL_CORE: %+v", report)
	}
	if len(report.Records) != 7 {
		t.Fatalf("fixed recordset: %d", len(report.Records))
	}
	for _, r := range report.Records {
		if !r.Verified {
			t.Fatalf("ASSERT_REHEARSAL_TWO_COPIES: %+v", r)
		}
	}
	semantic, prepared := f.Traffic()
	if semantic == 0 || prepared != 6 {
		t.Fatalf("traffic semantic=%d preparation=%d", semantic, prepared)
	}
	t.Log("ASSERT_REHEARSAL_ONE_ACTUAL_CORE: PASS")
}
func TestScopedLiveRehearsalEmptyAndIncompletePrivate(t *testing.T) {
	for _, kind := range []string{"rehearsal-empty-discovery", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			h, m, cfg, f := pilot.NewScopedRehearsalFixturePrivate(t)
			f.Fault(kind)
			report, err := executeScopedRehearsalPrivate(context.Background(), h, m, cfg)
			if report.AcquireCalls != 0 || report.Constituents != 0 {
				t.Fatal("empty/incomplete acquired")
			}
			if kind == "rehearsal-empty-discovery" {
				if err != nil || report.Outcome != "EMPTY" {
					t.Fatalf("empty: %+v %v", report, err)
				}
			} else {
				if err == nil || report.Outcome == "EMPTY" {
					t.Fatal("incomplete became empty")
				}
			}
			if report.OutcomeReceipt == nil || !report.OutcomeReceipt.Verified {
				t.Fatal("immutable observation missing")
			}
		})
	}
}
func TestScopedLiveRehearsalDiscoveryPolicyPrivate(t *testing.T) {
	for _, n := range []int{0, 63, 64} {
		for _, complete := range []bool{false, true} {
			d := censusacquisition.Discovery{Complete: complete, Closure: censusacquisition.DiscoveryClosureSuccessful, Targets: make([]censusacquisition.PreparedTarget, n)}
			err := scopedDiscoveryPolicyPrivate(d)
			if (err == nil) != ((complete || n == 0) && n <= 63) {
				t.Fatal(n, complete, err)
			}
		}
	}
}
func TestScopedLiveRehearsalConfigPrivate(t *testing.T) {
	_, _, cfg, _ := pilot.NewScopedRehearsalFixturePrivate(t)
	// Parser-only paths: never opened. Runtime fixture roots remain disposable.
	cfg.OutputRoots = [3]string{"/explicit/primary", "/explicit/backup", "/explicit/coordinator"}
	raw, _ := json.Marshal(cfg)
	if _, err := pilot.ParseScopedRehearsalConfigPrivate(raw); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append(bytes.Clone(raw), ' '), bytes.Replace(raw, []byte(`"schema":`), []byte(`"unknown":`), 1), []byte(`{"schema":"a","schema":"b"}`), bytes.Repeat([]byte("x"), pilot.ScopedRehearsalConfigLimitPrivate+1)} {
		if _, err := pilot.ParseScopedRehearsalConfigPrivate(bad); err == nil {
			t.Fatal("strict config accepted")
		}
	}
	cfg.Budget.TotalBytes++
	raw, _ = json.Marshal(cfg)
	if _, err := pilot.ParseScopedRehearsalConfigPrivate(raw); err == nil {
		t.Fatal("budget drift")
	}
}
