//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/filter"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/inspection"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

const partialConfigSchemaPrivate = "lsp-trace.scoped-rehearsal.private.v3"

func scopedPartialEnvironmentPrivate(get func(string) string) (string, bool, error) {
	switch get("LSP_TRACE_PARTIAL_INSPECTION_REHEARSAL") {
	case "":
		return "", false, nil
	case "one-partial-inspection":
		p := get("LSP_TRACE_PARTIAL_INSPECTION_REHEARSAL_CONFIG")
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return "", true, errors.New("explicit canonical absolute config required")
		}
		return p, true, nil
	default:
		return "", false, errors.New("invalid private partial inspection sentinel")
	}
}

// This entrypoint is inert without its separate exact sentinel. Nothing here
// creates roots, inherits ambient process environment, or authorizes a live run.
func TestScopedPartialLiveInspectionRehearsalPrivate(t *testing.T) {
	path, on, err := scopedPartialEnvironmentPrivate(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Skip("opt-in only: separate one-partial-inspection sentinel")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, pilot.ScopedRehearsalConfigLimitPrivate+1))
	if err = errors.Join(readErr, file.Close()); err != nil {
		t.Fatal(err)
	}
	cfg, err := pilot.ParseScopedRehearsalConfigPrivate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := scopedRehearsalPurposeBeforeStartupPrivate(cfg, true, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		head, err := exec.CommandContext(ctx, "git", "rev-parse", "HEAD").Output()
		if err != nil || string(head) != cfg.CodeHEAD+"\n" {
			t.Fatal("exact reviewed executable HEAD required", err)
		}
		for _, args := range [][]string{{"diff", "--quiet"}, {"diff", "--cached", "--quiet"}} {
			if err := exec.CommandContext(ctx, "git", args...).Run(); err != nil {
				t.Fatal("clean reviewed executable checkout required", err)
			}
		}
		untracked, err := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard").Output()
		if err != nil || len(untracked) != 0 {
			t.Fatal("untracked executable checkout refused", err)
		}
		h, err := pilot.OpenScopedRehearsalPrivate(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if e := h.Close(); e != nil {
				t.Error("later output-root close failure:", e)
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
			if e := m.Shutdown(context.Background()); e != nil {
				t.Error("later runtime shutdown failure:", e)
			}
		}()
		v, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: cfg.TrustDomain, Workspace: cfg.Workspace, Profile: cfg.Profile, EnvironmentReference: cfg.EnvironmentReference})
		if err != nil {
			t.Fatal(err)
		}
		started := m.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(v), LanguageID: "go", Process: managedprocess.Spec{Path: cfg.Server, Args: cfg.Args, Dir: cfg.Workspace, Env: cfg.Environment}})
		if started.Failure != "" || started.SessionID != cfg.Session.SessionID || started.Generation != cfg.Session.Generation {
			t.Fatal("exact session startup mismatch")
		}
		report, err := executeScopedPartialRehearsalPrivate(ctx, h, m, cfg)
		encoded, encodeErr := json.Marshal(report)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		t.Log(string(encoded))
		if err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

type scopedPartialReportPrivate struct {
	scopedLiveReportPrivate
	Purpose            string `json:"purpose"`
	Admission          string `json:"core_admission"`
	SourceCompleteness string `json:"source_completeness"`
	CanonicalComplete  bool   `json:"canonical_complete"`
	WithheldCount      int    `json:"policy_withheld_count"`
	// Optional private observation accounting, not a source-carrier wire change.
	SourceDispositionCounts map[string]int `json:"source_disposition_counts,omitempty"`
}

func executeScopedPartialRehearsalPrivate(ctx context.Context, h *pilot.ScopedRehearsalPrivate, m *sessionruntime.Manager, cfg pilot.ScopedRehearsalConfigPrivate) (report scopedPartialReportPrivate, err error) {
	report = scopedPartialReportPrivate{scopedLiveReportPrivate: scopedLiveReportPrivate{Schema: "lsp-trace.scoped-partial-inspection-observation.private.v1", Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", RevisionQualification: "CALLER_ASSERTED_NOT_AUTHENTICATED", Outcome: "INDETERMINATE", EffectiveBudget: struct {
		Output                                                                           pilot.ScopedRehearsalBudgetPrivate
		LifetimeSeconds, RequestSeconds, Requests, CaptureBytes, Messages, ResponseBytes int
	}{cfg.Budget, 30, 5, 256, 8 << 20, 64, 2 << 20}, Diagnostics: &scopedTerminalDiagnosticsPrivate{Events: []sessionruntime.RetainedScopeDiagnosticPrivate{}}}, Purpose: "PARTIAL_INSPECTION", Admission: "NOT_ATTEMPTED", SourceCompleteness: "UNKNOWN"}
	if cfg.Schema != partialConfigSchemaPrivate || !h.MatchesConfigPrivate(cfg) {
		return report, errors.New("partial purpose configuration/claim mismatch")
	}
	report.DiscoveryPolicy, err = cfg.DiscoveryPolicyObservationPrivate()
	if err != nil {
		return report, err
	}
	ready := m.BeginReadiness(ctx, cfg.Session.SessionID, cfg.Session.Generation, time.Now().Add(5*time.Second))
	ready, found := m.WaitReadiness(ctx, ready.ID)
	report.Runtime, err = validateScopedStartupPrivate(ctx, m, cfg, ready, found)
	if err != nil {
		report.Failure = err.Error()
		return report, err
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
	var discovery censusacquisition.Discovery
	var requestRaw, artifact, views, sourceFailureObservation []byte
	operationErr := m.WithReadySessionLease(ctx, sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(leased context.Context) error {
		defer func() {
			events, overflow, available := h.Diagnostics()
			if available {
				if events == nil {
					events = []sessionruntime.RetainedScopeDiagnosticPrivate{}
				}
				report.Diagnostics = &scopedTerminalDiagnosticsPrivate{available, events, overflow}
			}
		}()
		if err != nil {
			return err
		}
		if _, e := h.Bind(leased, m); e != nil {
			return e
		}
		var e error
		discovery, e = h.Discover(leased, cfg.Session)
		if e != nil {
			return e
		}
		if e = scopedDiscoveryPolicyPrivate(discovery); e != nil {
			return e
		}
		encoded, e := json.Marshal(discovery)
		if e != nil || len(encoded) > cfg.Budget.ArtifactBytes {
			return errors.Join(e, errors.New("discovery serialized cap"))
		}
		if len(discovery.Targets) == 0 {
			return nil
		}
		core := censusacquisition.Core{Planning: &censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 63}}
		batches, e := core.PlanDiscovery(cfg.Session, discovery)
		if e != nil {
			return e
		}
		if len(batches) != 1 {
			return errors.New("partial inspection requires one canonical batch")
		}
		batch := batches[0]
		manifest := pilot.ScopedPartialManifestPrivate(batch)
		requestRaw, e = json.Marshal(struct {
			Batch    censusacquisition.BatchRequest `json:"batch"`
			Manifest acquisitionmanifest.Manifest   `json:"manifest"`
		}{batch, manifest})
		if e != nil || len(requestRaw) > cfg.Budget.ArtifactBytes {
			return errors.Join(e, errors.New("request serialized cap"))
		}
		report.AcquireCalls++
		var seeds []byte
		artifact, seeds, e = h.AcquirePartialPrivate(leased, m, batch)
		if e != nil {
			var captureFailure *pilot.ScopedPartialCaptureFailurePrivate
			if errors.As(e, &captureFailure) {
				report.SourceDispositionCounts = captureFailure.StatusCounts
				report.WithheldCount = captureFailure.StatusCounts[graphprovenance.PolicyWithheldPrivateV1]
				// Eligible bounded failure metadata only; never a successful artifact.
				sourceFailureObservation, _ = json.Marshal(captureFailure)
			}
			return e
		}
		if !bytes.Equal(seeds, batch.CanonicalSeedsV2) {
			return errors.New("canonical seed drift")
		}
		if len(artifact) == 0 || len(artifact) > cfg.Budget.ArtifactBytes {
			return errors.New("partial artifact serialized cap after producer; no heap bound claim")
		}
		carrier, e := graphprovenance.ParseSourceWithholdingPrivateV1(artifact)
		if e != nil {
			return e
		}
		report.SourceDispositionCounts = map[string]int{}
		for _, capture := range carrier.Evidence.Captures {
			report.SourceDispositionCounts[capture.Status]++
		}
		report.WithheldCount = carrier.WithheldCount
		report.CanonicalComplete = carrier.Evidence.Acquisition.Graph.Summary.Complete
		projected, e := inspection.ProjectAllSeeds(carrier.Evidence.GraphBytes)
		if e != nil {
			return e
		}
		if e = inspection.ValidateAllSeedAccounting(projected); e != nil {
			return e
		}
		projectionRaw, e := json.Marshal(projected)
		if e != nil || len(projectionRaw) > cfg.Budget.ArtifactBytes {
			return errors.Join(e, errors.New("inspection serialized cap before write"))
		}
		// Original canonical target labels, never context-only graph nodes or features.
		left, right := manifest.Root.ID, manifest.Root.ID
		if len(manifest.RequiredTargets) > 0 {
			right = manifest.RequiredTargets[0].ID
		}
		compared, e := filter.ProjectPairwise(projectionRaw, left, right)
		if e != nil {
			return e
		}
		views, e = json.Marshal(struct {
			Inspection any `json:"inspection"`
			Filter     any `json:"filter"`
		}{projected, compared})
		if e != nil || len(views) > cfg.Budget.ArtifactBytes {
			views = nil
			return errors.Join(e, errors.New("inspection/filter serialized cap before write"))
		}
		return nil
	})
	report.Runtime, _ = m.Metadata(cfg.Session.SessionID, cfg.Session.Generation)
	if claim := h.ClaimObservation(); len(claim) > 0 {
		retain("claim", claim)
	}
	if discovery.Session.Validate() == nil {
		raw, e := json.Marshal(discovery)
		if e != nil {
			err = errors.Join(err, e)
		} else {
			retain("discovery", raw)
		}
	}
	if len(requestRaw) > 0 {
		retain("canonical-request-seeds", requestRaw)
	}
	if len(sourceFailureObservation) > 0 {
		retain("source-failure-observation", sourceFailureObservation)
	}
	if len(artifact) > 0 {
		retain("bounded-partial-artifact", artifact)
	}
	if len(views) > 0 {
		retain("inspection-filter-views", views)
	}
	if operationErr == nil {
		if len(discovery.Targets) == 0 && report.AcquireCalls == 0 {
			report.Outcome = "EMPTY"
		} else if report.AcquireCalls == 1 && len(artifact) > 0 && len(views) > 0 {
			report.Outcome = "PARTIAL_INSPECTION_RETAINED"
		} else {
			operationErr = errors.New("partial retention invariant")
		}
	}
	err = errors.Join(err, operationErr)
	if e := h.CloseExecution(); e != nil {
		report.CloseFailure = e.Error()
		err = errors.Join(err, e)
	}
	if err != nil {
		report.Outcome = "INDETERMINATE"
		report.Failure = err.Error()
	}
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

func TestScopedPartialRehearsalExact25Private(t *testing.T) {
	h, m, cfg, f := pilot.NewScopedPartialExact25RehearsalFixturePrivate(t)
	report, e := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if report.Outcome != "PARTIAL_INSPECTION_RETAINED" || report.WithheldCount != 1 || report.CanonicalComplete || report.Constituents != 0 || len(report.Records) != 7 || !report.OutcomeReceipt.Verified {
		t.Fatalf("ASSERT_EXACT25_PARTIAL_RETAINED: %+v", report)
	}
	foundArtifact, foundSeeds, foundViews := false, false, false
	for _, record := range append(report.Records, *report.OutcomeReceipt) {
		if !record.Verified {
			t.Fatal("ASSERT_EXACT25_PAIRED_RECORD", record.Name)
		}
		raw, e := os.ReadFile(filepath.Join(cfg.OutputRoots[0], record.ID))
		if e != nil {
			t.Fatal(e)
		}
		backup, e := os.ReadFile(filepath.Join(cfg.OutputRoots[1], record.ID))
		if e != nil || !bytes.Equal(raw, backup) || pilot.Digest(raw) != record.ID {
			t.Fatal("ASSERT_EXACT25_PAIRED_READHASH")
		}
		switch record.Name {
		case "bounded-partial-artifact":
			carrier, e := graphprovenance.ParseSourceWithholdingPrivateV1(raw)
			if e != nil {
				t.Fatal(e)
			}
			r := carrier.Evidence.Acquisition
			if len(r.Targets) != 25 || len(r.Graph.Edges) != 3 || len(r.Graph.Frontier) != 2 || len(carrier.Evidence.Captures) != 6 {
				t.Fatalf("ASSERT_EXACT25_CARRIER_COUNTS targets=%d edges=%d frontier=%d captures=%d summary=%+v", len(r.Targets), len(r.Graph.Edges), len(r.Graph.Frontier), len(carrier.Evidence.Captures), r.Graph.Summary)
			}
			readable := 0
			for _, c := range carrier.Evidence.Captures {
				if c.Status == "READABLE" {
					readable++
				}
			}
			if readable != 5 || carrier.WithheldCount != 1 {
				t.Fatal("ASSERT_EXACT25_SOURCE_COUNTS")
			}
			for _, q := range r.Requests {
				if strings.Contains(q.Method, "incoming") && q.NodeID == r.Targets[0].Resolution.Identity.ID {
					t.Fatal("ASSERT_EXACT25_NO_INCOMING_RESEED")
				}
			}
			foundArtifact = true
		case "canonical-request-seeds":
			var request struct {
				Batch censusacquisition.BatchRequest `json:"batch"`
			}
			if json.Unmarshal(raw, &request) != nil || len(request.Batch.Targets) != 25 || fmt.Sprintf("%x", sha256.Sum256(request.Batch.CanonicalSeedsV2)) != "733e8b43cf7e595790e42c61ad4dc45441b48b0ae82a6b93bddfd11b16a95d87" {
				t.Fatal("ASSERT_EXACT25_CANONICAL_CORE_SEEDS")
			}
			foundSeeds = true
		case "inspection-filter-views":
			if !bytes.Contains(raw, []byte(`"target-000009"`)) || !bytes.Contains(raw, []byte(`"UNKNOWN"`)) {
				t.Fatal("ASSERT_EXACT25_EXISTING_VIEW_BOUNDARY")
			}
			foundViews = true
		}
	}
	if !foundArtifact || !foundSeeds || !foundViews {
		t.Fatal("ASSERT_EXACT25_RECORD_PLAN")
	}
	semantic, supplies := f.Traffic()
	if semantic != 104 || supplies != 6 {
		t.Fatalf("ASSERT_EXACT25_SIX_SUPPLIES semantic=%d supplies=%d", semantic, supplies)
	}
	methods := map[string]int{}
	for _, q := range f.RequestsPrivate() {
		methods[q.Method]++
		if bytes.Contains(q.Params, []byte("outside.go")) {
			t.Fatal("ASSERT_EXACT25_NO_EXTERNAL_QUERY")
		}
	}
	if methods["textDocument/documentSymbol"] != 4 || methods["textDocument/prepareCallHierarchy"] != 50 || methods["callHierarchy/outgoingCalls"] != 25 || methods["callHierarchy/incomingCalls"] != 25 {
		t.Fatalf("ASSERT_EXACT25_FINITE_TRAFFIC: %v", methods)
	}
	live, _, _, _, general, occupied := m.DirectionalAccountingPrivate()
	active, closed, started, released := m.DirectionalGenericTerminalPrivate()
	if live != 0 || general != 0 || occupied || active != 0 || !closed || !started || !released {
		t.Fatal("ASSERT_EXACT25_REAL_RELEASE")
	}
	t.Log("ASSERT_EXACT25_PARTIAL_RETAINED: PASS; canonical25; requests=104; records=8; readable=5 withheld=1; CALLS=3 frontier=2; no admission")
}

func TestScopedPartialRehearsalRetainedFailuresPrivate(t *testing.T) {
	for _, fault := range []string{"source-alias", "backup-read", "backup-hash", "backup-close"} {
		t.Run(fault, func(t *testing.T) {
			h, m, cfg, _ := pilot.NewScopedPartialExact25RehearsalFixturePrivate(t, fault)
			report, e := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
			if e == nil || report.Outcome != "INDETERMINATE" || report.Constituents != 0 || report.Failure == "" {
				t.Fatal("ASSERT_PARTIAL_FAILURE_TRUTHFUL", report, e)
			}
			artifactFound := false
			for _, r := range report.Records {
				if r.Name == "bounded-partial-artifact" {
					artifactFound = true
					if fault != "source-alias" && r.Verified {
						t.Fatal("ASSERT_PARTIAL_PAIR_NOT_VERIFIED")
					}
					if fault != "source-alias" && r.PrimaryReceipt == nil {
						t.Fatal("ASSERT_PARTIAL_PRIMARY_RECEIPT_PRESERVED")
					}
				}
			}
			if fault == "source-alias" {
				if artifactFound {
					t.Fatal("ASSERT_ALIAS_ZERO_ARTIFACT")
				}
				var captureFailure *pilot.ScopedPartialCaptureFailurePrivate
				if !errors.As(e, &captureFailure) || (captureFailure.Status != "UNREADABLE" && captureFailure.Status != "NONREGULAR") || captureFailure.RowOrdinal < 0 || report.SourceDispositionCounts[captureFailure.Status] != 1 || !strings.Contains(report.Failure, "status="+captureFailure.Status+" row_ordinal=") {
					t.Fatal("ASSERT_ALIAS_PRECISE_CLOSED_FAILURE", report.Failure)
				}
				found := false
				for _, r := range report.Records {
					if r.Name == "source-failure-observation" && r.Verified {
						primary, readErr := os.ReadFile(filepath.Join(cfg.OutputRoots[0], r.ID))
						backup, backupErr := os.ReadFile(filepath.Join(cfg.OutputRoots[1], r.ID))
						expected, _ := json.Marshal(captureFailure)
						if readErr != nil || backupErr != nil || !bytes.Equal(primary, backup) || !bytes.Equal(primary, expected) || pilot.Digest(primary) != r.ID {
							t.Fatal("ASSERT_ALIAS_FAILURE_PAIRED_METADATA")
						}
						total := 0
						for _, n := range captureFailure.StatusCounts {
							total += n
						}
						if total != 6 {
							t.Fatal("ASSERT_FAILURE_ALL_ROWS_ACCOUNTED", total)
						}
						found = true
					}
				}
				if !found {
					t.Fatal("ASSERT_ALIAS_FAILURE_OBSERVATION")
				}
			}
			live, _, _, _, general, occupied := m.DirectionalAccountingPrivate()
			active, closed, started, released := m.DirectionalGenericTerminalPrivate()
			if live != 0 || general != 0 || occupied || active != 0 || !closed || !started || !released {
				t.Fatal("ASSERT_PARTIAL_FAILURE_RELEASE")
			}
		})
	}
}

func TestScopedPartialRehearsalOldDriverRefusesPurposePrivate(t *testing.T) {
	h, m, cfg, f := pilot.NewScopedPartialRehearsalFixturePrivate(t)
	report, e := executeScopedRehearsalPrivate(context.Background(), h, m, cfg)
	semantic, prepared := f.Traffic()
	if e == nil || report.Constituents != 0 || report.AcquireCalls != 0 || len(report.Records) != 0 || semantic != 0 || prepared != 0 {
		t.Fatal("ASSERT_OLD_DRIVER_PARTIAL_PURPOSE_REFUSED")
	}
}

func TestScopedPartialRehearsalSentinelPrivate(t *testing.T) {
	for _, v := range []string{"", "one-partial-inspection", "one-native-v5", "1", "one-partial-inspection "} {
		_, on, e := scopedPartialEnvironmentPrivate(func(k string) string {
			if k == "LSP_TRACE_PARTIAL_INSPECTION_REHEARSAL" {
				return v
			}
			return "/explicit/config.json"
		})
		if v == "" && (on || e != nil) || v == "one-partial-inspection" && (!on || e != nil) || v != "" && v != "one-partial-inspection" && e == nil {
			t.Fatalf("sentinel %q", v)
		}
	}
}

func TestScopedPartialRehearsalFakePrivate(t *testing.T) {
	h, m, cfg, f := pilot.NewScopedPartialRehearsalFixturePrivate(t)
	f.Fault("rehearsal-empty-directions")
	report, e := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if report.Outcome != "PARTIAL_INSPECTION_RETAINED" || report.Constituents != 0 || report.Admission != "NOT_ATTEMPTED" || report.Accepted || report.Authority != 0 || len(report.Records) != 7 || report.OutcomeReceipt == nil || !report.OutcomeReceipt.Verified {
		t.Fatalf("ASSERT_PARTIAL_RETAINED: %+v", report)
	}
	for _, r := range append(report.Records, *report.OutcomeReceipt) {
		if !r.Verified {
			t.Fatal("ASSERT_PARTIAL_PAIRED", r.Name)
		}
		for _, root := range cfg.OutputRoots[:2] {
			raw, e := os.ReadFile(filepath.Join(root, r.ID))
			if e != nil || pilot.Digest(raw) != r.ID {
				t.Fatal("ASSERT_PARTIAL_READ_HASH", e)
			}
		}
	}
	if report.DiscoveryPolicy.Name != "one-partial-inspection-private-v1" || report.DiscoveryPolicy.MaxSymbols != 256 {
		t.Fatal("ASSERT_PARTIAL_PURPOSE_POLICY")
	}
	t.Log("ASSERT_PARTIAL_RETAINED: PASS; records=8; core_admitted_constituents=0")
}

func TestScopedPartialRehearsalEmptyFailureAndPurposePrivate(t *testing.T) {
	for _, fault := range []string{"rehearsal-empty-discovery", "malformed", "provider-budget", "wrong-purpose"} {
		t.Run(fault, func(t *testing.T) {
			h, m, cfg, f := pilot.NewScopedPartialRehearsalFixturePrivate(t)
			if fault == "wrong-purpose" {
				cfg.Schema = "lsp-trace.scoped-rehearsal.private.v2"
			} else if fault == "provider-budget" {
				pilot.ArmScopedPartialAcquisitionBudgetFailurePrivate(f)
			} else {
				f.Fault(fault)
			}
			report, e := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
			if fault == "rehearsal-empty-discovery" {
				if e != nil || report.Outcome != "EMPTY" {
					t.Fatal(report, e)
				}
			} else if e == nil || report.Outcome == "EMPTY" {
				t.Fatal("failure became EMPTY")
			}
			wantCalls := 0
			if fault == "provider-budget" {
				wantCalls = 1
			}
			if report.AcquireCalls != wantCalls || report.Constituents != 0 {
				t.Fatal("unexpected acquisition/admission", report.AcquireCalls, report.Constituents)
			}
			if fault == "wrong-purpose" {
				semantic, prep := f.Traffic()
				if semantic != 0 || prep != 0 || len(report.Records) != 0 {
					t.Fatal("purpose mismatch traffic")
				}
			}
		})
	}
}
