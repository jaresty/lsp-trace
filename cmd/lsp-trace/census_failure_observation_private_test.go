//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/sessionruntime"
)

func TestCensusFailureObservationSuccessWirePrivate(t *testing.T) {
	_, _, _, args := inventoryFixturePrivate(t)
	deps, _ := inventoryDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	raw, e := os.ReadFile(filepath.Join(r.AttemptDirectory, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	assertInventoryPrivate(t, "ASSERT_SUCCESS_DIAGNOSTICS_OMITTED", code == 0 && !bytes.Contains(raw, []byte(`"PipelineFailure"`)) && !bytes.Contains(raw, []byte(`"CleanupFailure"`)) && !bytes.Contains(raw, []byte(`"ContinuationFailure"`)))
}

type censusFailureWriterPrivate struct{}

func (censusFailureWriterPrivate) Write([]byte) (int, error) {
	return 0, errors.New("SECRET_DIAGNOSTIC_CANARY_41b")
}

func TestCensusFailureObservationWriterFailurePrivate(t *testing.T) {
	for _, kind := range []string{"report", "metadata"} {
		t.Run(kind, func(t *testing.T) {
			_, src, _, args := inventoryFixturePrivate(t)
			deps, _ := inventoryDepsPrivate(t)
			var attempt string
			deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
				d := productionCensusInventoryDependenciesPrivate()
				original := d.runSession
				d.runSession = func(rc initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
					return original(rc, func(ctx context.Context, r *initializedAcquisitionRuntime) int {
						if e := os.WriteFile(filepath.Join(src, "a.go"), []byte("SECRET_DIAGNOSTIC_CANARY_41b"), 0600); e != nil {
							t.Fatal(e)
						}
						entries, e := os.ReadDir(o.PublicationRoot)
						if e != nil {
							t.Fatal(e)
						}
						for _, entry := range entries {
							if strings.HasPrefix(entry.Name(), "private-census-inventory-") {
								attempt = filepath.Join(o.PublicationRoot, entry.Name())
							}
						}
						if attempt == "" {
							t.Fatal("fixture attempt not found")
						}
						if kind == "report" {
							if e := os.Mkdir(filepath.Join(attempt, "report.json"), 0700); e != nil {
								t.Fatal(e)
							}
						}
						return cb(ctx, r)
					})
				}
				return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, d)
			}
			var out, errout bytes.Buffer
			var writer io.Writer = &out
			if kind == "metadata" {
				writer = censusFailureWriterPrivate{}
			}
			code := runCensusWithDependencies(args, writer, &errout, deps)
			assertInventoryPrivate(t, "ASSERT_FAILURE_WRITER_HUMAN_"+kind, code == 1 && strings.Contains(errout.String(), "stage=ADAPTER_SOURCE_REVALIDATION") && !strings.Contains(errout.String(), "SECRET_DIAGNOSTIC_CANARY_41b"))
			if kind == "report" {
				assertInventoryPrivate(t, "ASSERT_REPORT_WRITER_EXISTING_FAILURE", strings.Contains(errout.String(), "private inventory report retention failed") && out.Len() == 0)
			} else {
				raw, e := os.ReadFile(filepath.Join(attempt, "report.json"))
				if e != nil {
					t.Fatal(e)
				}
				assertInventoryPrivate(t, "ASSERT_METADATA_WRITER_PIPELINE_RETAINED", bytes.Contains(raw, []byte(`"PipelineFailure"`)) && bytes.Contains(raw, []byte(`"ADAPTER_SOURCE_REVALIDATION"`)))
			}
		})
	}
}

// Each injected error follows real sealed production work; no successful result
// or owner is manufactured by the failure hook.
func TestCensusFailureObservationDerivedBatchOrdinalPrivate(t *testing.T) {
	for _, target := range []string{"PROGRAM_C_PROJECTION", "INSTABILITY_COMPUTATION", "PROTECTED_ROOT_OBSERVATION"} {
		t.Run(target, func(t *testing.T) {
			_, _, config, args := inventoryFixturePrivate(t)
			c := inventoryConfigPrivate()
			c.Policy.MaxNominationFiles = 3
			c.Policy.Discovery.MaxFiles = 3
			c.Policy.MaxMembers = 4
			saveInventoryConfigPrivate(t, config, c)
			deps, _ := inventoryDepsPrivate(t)
			var completed []int
			injected := false
			deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
				d := productionCensusInventoryDependenciesPrivate()
				d.derivedFailure = func(stage string, batch int) error {
					if stage == target && (batch == 1 || batch == -1) {
						injected = true
						return errors.New("SECRET_DIAGNOSTIC_CANARY_41b")
					}
					if stage == "INSTABILITY_COMPUTATION" {
						completed = append(completed, batch)
					}
					return nil
				}
				return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, d)
			}
			r, code := runInventoryCLIPrivate(t, args, deps)
			if !injected || len(completed) == 0 || completed[0] != 0 || code != 1 || len(r.Origins) != 1 {
				t.Fatalf("fixture did not reach target after batch zero: injected=%v completed=%v code=%d", injected, completed, code)
			}
			f := r.Origins[0].PipelineFailure
			if f == nil || f.Stage != target {
				t.Fatalf("fixture target diagnostic missing: %+v", f)
			}
			if target == "PROTECTED_ROOT_OBSERVATION" {
				if len(completed) < 2 || completed[1] != 1 {
					t.Fatal("fixture did not complete batch one")
				}
				assertInventoryPrivate(t, "ASSERT_DERIVED_NONBATCH_ORDINAL_ABSENT", f.BatchOrdinal == nil)
			} else {
				assertInventoryPrivate(t, "ASSERT_DERIVED_BATCH_ONE_"+target, f.BatchOrdinal != nil && *f.BatchOrdinal == 1)
			}
		})
	}
}

// Witnesses the original READY callback, not a reconstructed failure or an
// assertion about the cause of any real application attempt.
func TestCensusFailureObservationOriginalCallbackPrivate(t *testing.T) {
	for _, kind := range []string{"source", "member", "member-language", "lease", "batch", "late-batch", "source-and-cleanup", "cleanup-only"} {
		t.Run(kind, func(t *testing.T) {
			_, src, config, args := inventoryFixturePrivate(t)
			if kind == "late-batch" {
				c := inventoryConfigPrivate()
				c.Policy.MaxNominationFiles = 3
				c.Policy.Discovery.MaxFiles = 3
				c.Policy.MaxMembers = 4
				saveInventoryConfigPrivate(t, config, c)
			}
			deps, _ := inventoryDepsPrivate(t)
			const canary = "SECRET_DIAGNOSTIC_CANARY_41b"
			var cancelMember context.CancelFunc
			newManager := deps.newManager
			deps.newManager = func(c sessionruntime.Config) (*sessionruntime.Manager, error) {
				if kind == "member" {
					c.DocumentFinalHook = func() { cancelMember() }
				}
				return newManager(c)
			}
			starter, _ := deps.newStarter()
			child := starter.(inventoryStarterPrivate).child
			if kind == "batch" {
				child.failCalls.Store(true)
			}
			if kind == "late-batch" {
				child.failCallOrdinal = 2
			}
			if kind == "source-and-cleanup" || kind == "cleanup-only" {
				child.shutdownDelay = 400 * time.Millisecond
			}
			deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
				d := productionCensusInventoryDependenciesPrivate()
				original := d.runSession
				d.runSession = func(rc initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
					return original(rc, func(ctx context.Context, runtime *initializedAcquisitionRuntime) int {
						if kind == "source" || kind == "source-and-cleanup" {
							if e := os.WriteFile(filepath.Join(src, "a.go"), []byte(canary), 0600); e != nil {
								t.Fatal(e)
							}
						}
						if kind == "member-language" {
							r := runtime.manager.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: runtime.SessionID(), Generation: runtime.Generation(), URI: "file://" + filepath.Join(src, "b.go"), LanguageID: "typescript", CaptureSupply: true})
							if r.Failure != "" {
								t.Fatal("fixture conflicting language preparation failed")
							}
						}
						if kind == "member" {
							var cancel context.CancelFunc
							ctx, cancel = context.WithCancel(ctx)
							defer cancel()
							cancelMember = cancel
						}
						if kind == "lease" {
							code := -1
							e := runtime.manager.WithReadySessionLease(ctx, sessionruntime.SessionLeaseRequest{SessionID: runtime.SessionID(), Generation: runtime.Generation(), Workspace: src}, func(held context.Context) error { code = cb(held, runtime); return nil })
							if e != nil || code == -1 {
								t.Fatal("fixture outer lease failed")
							}
							return code
						}
						return cb(ctx, runtime)
					})
				}
				return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, d)
			}
			var out, errout bytes.Buffer
			code := runCensusWithDependencies(args, &out, &errout, deps)
			var meta struct{ AttemptDirectory string }
			if json.Unmarshal(out.Bytes(), &meta) != nil || meta.AttemptDirectory == "" {
				t.Fatalf("fixture metadata code=%d", code)
			}
			raw, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
			if e != nil {
				t.Fatal(e)
			}
			var r struct {
				Status  string
				Origins []struct {
					PipelineFailure *struct {
						Stage, Class, Subcause, DiscoveryAvailability string
						MemberOrdinal, BatchOrdinal                   *int
						OriginOrdinal                                 int
						LeaseEntered                                  bool
					}
					CleanupFailure           *struct{ Class string }
					Discovery                json.RawMessage
					StopCalls, ShutdownCalls int
				}
				Outcome json.RawMessage
			}
			if e = json.Unmarshal(raw, &r); e != nil {
				t.Fatal(e)
			}
			if code != 1 || r.Status != "RETAINED_FAILURE" || len(r.Origins) != 1 {
				t.Fatalf("fixture did not reach retained callback failure code=%d status=%s", code, r.Status)
			}
			f := r.Origins[0].PipelineFailure
			stage, availability, entered := "ADAPTER_SOURCE_REVALIDATION", "NOT_REACHED_DISCOVERY", true
			switch kind {
			case "member", "member-language":
				stage = "ADAPTER_MEMBER_PREPARE"
			case "lease":
				stage = "LEASE_ADMISSION"
				availability = "NOT_REACHED_ADAPTER"
				entered = false
			case "batch", "late-batch":
				stage = "CAPTURE_BATCH"
				availability = "AVAILABLE_ERROR_REPORT"
			}
			ok := f != nil && f.Stage == stage && f.Class != "" && f.DiscoveryAvailability == availability && f.OriginOrdinal == 0 && f.LeaseEntered == entered
			if kind == "member" || kind == "member-language" {
				n := 0
				if kind == "member-language" {
					n = 1
				}
				ok = ok && f != nil && f.MemberOrdinal != nil && *f.MemberOrdinal == n
				if kind == "member-language" {
					ok = ok && f.Class == "LIFECYCLE_CONFLICT" && f.Subcause == "LANGUAGE_ID_CONFLICT"
				}
			}
			if kind == "batch" || kind == "late-batch" {
				n := 0
				if kind == "late-batch" {
					n = 1
				}
				ok = ok && f != nil && f.BatchOrdinal != nil && *f.BatchOrdinal == n
			}
			if kind == "cleanup-only" {
				assertInventoryPrivate(t, "ASSERT_NO_FABRICATED_PIPELINE_"+kind, f == nil)
			} else {
				assertInventoryPrivate(t, "ASSERT_ORIGINAL_CALLBACK_DIAGNOSTIC_"+kind, ok)
			}
			cleanup := r.Origins[0].CleanupFailure
			assertInventoryPrivate(t, "ASSERT_INDEPENDENT_CLEANUP_"+kind, (cleanup != nil) == (kind == "source-and-cleanup" || kind == "cleanup-only") && r.Origins[0].StopCalls == 1 && r.Origins[0].ShutdownCalls == 1)
			assertInventoryPrivate(t, "ASSERT_DIAGNOSTIC_NO_SECRET_"+kind, !strings.Contains(string(raw), canary) && !strings.Contains(errout.String(), canary) && !strings.Contains(out.String(), canary))
			if f != nil {
				b, _ := json.Marshal(f)
				if strings.Contains(string(b), src) || strings.Contains(string(b), "file://") {
					t.Fatal("diagnostic leaked source path")
				}
			}
		})
	}
}
