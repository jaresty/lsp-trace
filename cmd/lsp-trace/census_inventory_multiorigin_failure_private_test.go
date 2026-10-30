//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/managedprocess"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCensusPrivateInventoryTwoOriginLateSourceDrift(t *testing.T) {
	for _, kind := range []string{"source", "denominator", "approval"} {
		t.Run(kind, func(t *testing.T) {
			base, src, path, args := inventoryTwoFixturePrivate(t)
			firstDeps, _, _ := inventoryTwoDepsPrivate(t)
			first, code := runInventoryCLIPrivate(t, args, firstDeps)
			if code != 0 {
				t.Fatal("preparation setup")
			}
			c, e := loadCensusInventoryConfigPrivate(path)
			if e != nil {
				t.Fatal(e)
			}
			mailbox := filepath.Join(base, "late-drift-mailbox")
			if e = os.Mkdir(mailbox, 0700); e != nil {
				t.Fatal(e)
			}
			c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: mailbox, Invocation: "owned-late-drift", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: first.ScopeSHA256, TimeoutMS: 5000, OverallTimeoutMS: 20000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCalls: 32, MaxRequestBytes: 4 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 64 << 20, MaxOutputBytes: 8 << 20}}
			c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
			saveInventoryConfigPrivate(t, path, c)
			d, _, _ := inventoryTwoDepsPrivate(t)
			start := d.newStarter
			n := 0
			disclosed := false
			d.newStarter = func() (sessionruntime.Starter, error) {
				n++
				if n == 2 {
					switch kind {
					case "source":
						if e := os.WriteFile(filepath.Join(src, "a.go"), []byte("package owned\n\nfunc A0() { /* late source drift */ }\n"), 0600); e != nil {
							t.Error(e)
						}
					case "denominator":
						if e := os.WriteFile(filepath.Join(src, "new.go"), []byte("package owned\n\nfunc A0() {}\n"), 0600); e != nil {
							t.Error(e)
						}
					case "approval":
						c.Automation.Limits.MaxCalls++
						saveInventoryConfigPrivate(t, path, c)
					}
				}
				return start()
			}
			d.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
				x := productionCensusInventoryDependenciesPrivate()
				x.automatic = func(prep.AutomaticHandoffConfigPrivateV2, string, prep.PostPreparationPrivateV2) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
					disclosed = true
					return nil, nil, errors.New("owned disclosure sentinel")
				}
				return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, x)
			}
			r, code := runInventoryCLIPrivate(t, args, d)
			assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_LATE_"+kind+"_DRIFT_REFUSED", code == 1 && !disclosed && r.Failure == "SOURCE_SCOPE_OR_APPROVAL_DRIFT" && r.Origins[0].CleanupVerified && r.Origins[1].CleanupVerified && r.Inventory == nil && r.Outcome == nil)
		})
	}
}

func TestCensusPrivateInventoryContextAliveUntilJoin(t *testing.T) {
	_, _, _, args := inventoryFixturePrivate(t)
	d, _ := inventoryDepsPrivate(t)
	original := d.newStarter
	d.newStarter = func() (sessionruntime.Starter, error) {
		s, e := original()
		if e != nil {
			return s, e
		}
		child := s.(inventoryStarterPrivate).child
		child.shutdownDelay = 30 * time.Millisecond
		return inventoryStartFuncPrivate(func(ctx context.Context, spec managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
			go func() { <-ctx.Done(); child.Close() }()
			return s.Start(ctx, spec)
		}), nil
	}
	r, code := runInventoryCLIPrivate(t, args, d)
	assertInventoryPrivate(t, "ASSERT_ORIGINAL_CONTEXT_ALIVE_UNTIL_JOIN", code == 0 && r.CleanupVerified && r.Origins[0].Cleanup != nil && r.Origins[0].Cleanup.ShutdownRead == sessionruntime.ShutdownMatchedPrivateV2)
}

func TestCensusPrivateInventoryTwoOriginWholeDeadline(t *testing.T) {
	_, _, path, args := inventoryTwoFixturePrivate(t)
	c, e := loadCensusInventoryConfigPrivate(path)
	if e != nil {
		t.Fatal(e)
	}
	c.Policy.MaxNominationFiles = 1
	c.Policy.Discovery.MaxFiles = 1
	c.OperationTimeoutMS = 1000
	c.Policy.TimeoutMS = 1000
	c.Policy.RequestTimeoutMS = 100
	saveInventoryConfigPrivate(t, path, c)
	for i, a := range args {
		if a == "30s" {
			args[i] = "1s"
		}
		if a == "5s" {
			args[i] = "100ms"
		}
	}
	d, _, _ := inventoryTwoDepsPrivate(t)
	original := d.newStarter
	n := 0
	cancelled := false
	d.newStarter = func() (sessionruntime.Starter, error) {
		s, e := original()
		n++
		if n == 2 {
			return inventoryStartFuncPrivate(func(ctx context.Context, _ managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
				<-ctx.Done()
				cancelled = errors.Is(ctx.Err(), context.DeadlineExceeded)
				return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed, Err: ctx.Err()}
			}), nil
		}
		return s, e
	}
	start := time.Now()
	r, code := runInventoryCLIPrivate(t, args, d)
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_WHOLE_DEADLINE_CLEANUP", code == 1 && cancelled && time.Since(start) < 1800*time.Millisecond && len(r.Origins) == 2 && r.Origins[0].CleanupVerified && r.Origins[1].ShutdownJoined && r.Outcome == nil)
}

func TestCensusPrivateInventoryFreshFactoryReuseRefused(t *testing.T) {
	_, _, _, args := inventoryTwoFixturePrivate(t)
	d, starts, _ := inventoryTwoDepsPrivate(t)
	d.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
		cfg.privateFreshRunner = func() (initializedAcquisitionRunnerConfig, error) { return cfg.runner, nil }
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, productionCensusInventoryDependenciesPrivate())
	}
	r, code := runInventoryCLIPrivate(t, args, d)
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_REUSED_FACTORY_REFUSED", code == 1 && len(r.Origins) == 2 && r.Origins[1].Status == "FRESH_RUNTIME_IDENTITY_REFUSED" && r.Origins[0].CleanupVerified && r.Outcome == nil && starts.Load() == 1)
}

func TestCensusPrivateInventoryTwoOriginFiniteEmpty(t *testing.T) {
	_, _, path, args := inventoryTwoFixturePrivate(t)
	c, e := loadCensusInventoryConfigPrivate(path)
	if e != nil {
		t.Fatal(e)
	}
	c.Policy.MaxNominationFiles = 1
	c.Policy.Discovery.MaxFiles = 1
	saveInventoryConfigPrivate(t, path, c)
	d, starts, _ := inventoryTwoDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, d)
	gaps := 0
	for _, row := range r.Files {
		if row.Disposition == "RESOURCE_GAP_ORIGIN_LIMIT" {
			gaps++
		}
	}
	var empty censusacquisition.Discovery
	if len(r.Origins) == 2 {
		_ = json.Unmarshal(r.Origins[1].Discovery, &empty)
	}
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_FINITE_EMPTY_GAPS", code == 1 && gaps == 7 && starts.Load() == 2 && len(r.Origins) == 2 && r.Origins[1].Status == "DISCOVERY_OR_ACQUISITION_FAILED" && r.Origins[1].CleanupVerified && r.ApplicationCoverage == "PARTIAL_RESOURCE_GAPS" && len(r.Index) == 0 && r.Outcome == nil && len(empty.EnumerationSymbols) == 1 && string(empty.EnumerationSymbols[0].Disposition) == "non-callable")
}

type inventoryStartFuncPrivate func(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation)

func (f inventoryStartFuncPrivate) Start(c context.Context, s managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return f(c, s)
}

func TestCensusPrivateInventoryTwoOriginTerminalFailures(t *testing.T) {
	for _, kind := range []string{"startup", "acquisition", "retention"} {
		t.Run(kind, func(t *testing.T) {
			_, _, _, args := inventoryTwoFixturePrivate(t)
			d, _, managers := inventoryTwoDepsPrivate(t)
			originalStarter := d.newStarter
			calls := 0
			d.newStarter = func() (sessionruntime.Starter, error) {
				s, e := originalStarter()
				calls++
				if calls == 2 && kind == "startup" {
					return inventoryStartFuncPrivate(func(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
						return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed, Err: errors.New("owned second start failure")}
					}), nil
				}
				if calls == 2 && kind == "acquisition" {
					s.(inventoryStarterPrivate).child.failCalls.Store(true)
				}
				return s, e
			}
			if kind == "retention" {
				d.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
					x := productionCensusInventoryDependenciesPrivate()
					original := x.runSession
					n := 0
					x.runSession = func(c initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
						n++
						return original(c, func(ctx context.Context, r *initializedAcquisitionRuntime) int {
							if n == 2 {
								entries, e := os.ReadDir(o.PublicationRoot)
								if e != nil {
									t.Error(e)
								}
								for _, entry := range entries {
									if entry.IsDir() {
										if e := os.Mkdir(filepath.Join(o.PublicationRoot, entry.Name(), "producer-1-0"), 0700); e != nil {
											t.Error(e)
										}
									}
								}
							}
							return cb(ctx, r)
						})
					}
					return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, x)
				}
			}
			r, code := runInventoryCLIPrivate(t, args, d)
			valid := code == 1 && len(r.Origins) == 2 && r.Outcome == nil && len(*managers) == 2 && r.Origins[0].CleanupVerified && r.Origins[0].Status == "COMPUTED_PRIVATE_PER_BATCH" && r.Origins[1].ShutdownCalls == 1 && r.Origins[1].ShutdownJoined
			if kind != "startup" {
				valid = valid && r.Origins[1].StopCalls == 1 && r.Origins[1].CleanupVerified
			}
			assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_FAILURE_CLEANUP_"+kind, valid)
			if kind == "retention" {
				assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_RETENTION_FAILURE_DISPOSITION", r.Origins[1].Status == "PRODUCER_RETENTION_ROOT_FAILED")
			}
		})
	}
}

func TestCensusPrivateInventoryTwoOriginStartupDeadline(t *testing.T) {
	_, _, path, args := inventoryTwoFixturePrivate(t)
	c, err := loadCensusInventoryConfigPrivate(path)
	if err != nil {
		t.Fatal(err)
	}
	c.OperationTimeoutMS = 30000
	saveInventoryConfigPrivate(t, path, c)
	d, _, _ := inventoryTwoDepsPrivate(t)
	var whole, second time.Time
	originalStarter := d.newStarter
	n := 0
	d.newStarter = func() (sessionruntime.Starter, error) {
		s, e := originalStarter()
		n++
		if n == 2 {
			return inventoryStartFuncPrivate(func(ctx context.Context, _ managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
				second, _ = ctx.Deadline()
				return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed, Err: errors.New("owned deadline observation")}
			}), nil
		}
		return s, e
	}
	d.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
		x := productionCensusInventoryDependenciesPrivate()
		enumerate := x.enumerate
		x.enumerate = func(ctx context.Context, o censusCLIOptions) ([]censusacquisition.SourceFile, error) {
			whole, _ = ctx.Deadline()
			return enumerate(ctx, o)
		}
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, x)
	}
	r, code := runInventoryCLIPrivate(t, args, d)
	// Both configured bounds are 30s. Time already spent on origin zero must
	// not be granted anew to origin one's startup.
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_STARTUP_SHARED_DEADLINE", code == 1 && !whole.IsZero() && !second.IsZero() && !second.After(whole) && r.Origins[1].ShutdownJoined)
}

func TestCensusPrivateInventoryForeignRuntimeRefusal(t *testing.T) {
	_, _, _, args := inventoryTwoFixturePrivate(t)
	d, _, _ := inventoryTwoDepsPrivate(t)
	d.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
		x := productionCensusInventoryDependenciesPrivate()
		run := x.runSession
		n := 0
		x.runSession = func(c initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
			n++
			return run(c, func(ctx context.Context, r *initializedAcquisitionRuntime) int {
				if n == 2 {
					clone := *r
					foreign, e := sessionruntime.New(sessionruntime.Config{Starter: inventoryStartFuncPrivate(func(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
						return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed}
					}), Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64}})
					if e != nil {
						t.Fatal(e)
					}
					defer foreign.Shutdown(context.Background())
					clone.privateAcquisitionRuntime = &privateAcquisitionRuntime{manager: foreign}
					return cb(ctx, &clone)
				}
				return cb(ctx, r)
			})
		}
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, x)
	}
	r, code := runInventoryCLIPrivate(t, args, d)
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_FOREIGN_RUNTIME_REFUSED", code == 1 && r.Outcome == nil && len(r.Origins) == 2 && r.Origins[1].Status == "FOREIGN_RUNTIME_REFUSED" && r.Origins[1].CleanupVerified)
}
