//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/sessionruntime"
)

func TestCensusPrivateShutdownResponseAdapterContext(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancelled-%t", cancelled), func(t *testing.T) {
			_, _, _, args := inventoryFixturePrivate(t)
			deps, _ := inventoryDepsPrivate(t)
			starter, _ := deps.newStarter()
			starter.(inventoryStarterPrivate).child.shutdownDelay = 600 * time.Millisecond
			deps.runPrivateInventory = func(_ censusCLIOptions, cfg censusCoreConfig, _, _ io.Writer) int {
				rc := cfg.runner
				rc.privateShutdownResponseTimeout = 2 * time.Second
				rc.shutdownManagedByCaller = true
				rc.joinAfterStop = func() {}
				return runInitializedAcquisitionSession(rc, func(live context.Context, runtime initializedAcquisitionSession) int {
					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()
					if cancelled {
						cancel()
					}
					stopped := rc.stop(ctx, runtime.SessionID())
					cleanup, done := context.WithTimeout(context.Background(), 3*time.Second)
					defer done()
					if err := rc.manager.Shutdown(cleanup); err != nil {
						t.Fatal(err)
					}
					op, ok := rc.manager.Operation(stopped.IntentID)
					if !ok {
						t.Fatal("missing original stop operation")
					}
					f, ok := op.CleanupObservationPrivateV2()
					if !ok || f.ResponseBudgetSource != "PRIVATE_V2" || f.DeclaredResponseBudget != 2*time.Second || f.EffectiveResponseReadBudget > 100*time.Millisecond || (f.ReaderStarted && !f.ReaderJoined) || f.RetiredTransportReused {
						t.Fatalf("context/owner violation: %+v", f)
					}
					expected := sessionruntime.ShutdownTimeoutPrivateV2
					if cancelled {
						expected = sessionruntime.ShutdownCancelledPrivateV2
					}
					if f.ShutdownRead != expected {
						t.Fatalf("context result: %+v", f)
					}
					if live.Err() != nil {
						t.Fatal("startup cancelled before stop/join")
					}
					return 0
				})
			}
			var out, errs bytes.Buffer
			if code := runCensusWithDependencies(args, &out, &errs, deps); code != 0 {
				t.Fatalf("runner setup: %d %s", code, &errs)
			}
		})
	}
}

func TestCensusPrivateShutdownResponseAdmission(t *testing.T) {
	_, _, path, _ := inventoryFixturePrivate(t)
	for _, ms := range []int{-1, 0, 1, 2000, 2001} {
		c := inventoryConfigPrivate()
		c.ShutdownResponseTimeoutMS = ms
		saveInventoryConfigPrivate(t, path, c)
		_, err := loadCensusInventoryConfigPrivate(path)
		if (err == nil) != (ms >= 0 && ms <= 2000) {
			t.Fatalf("admission %d: %v", ms, err)
		}
	}
	c := inventoryConfigPrivate()
	raw, _ := json.Marshal(c)
	if bytes.Contains(raw, []byte("ShutdownResponseTimeoutMS")) {
		t.Fatal("zero changed canonical config")
	}
	before := censusInventoryScopeBytesPrivate(censusCLIOptions{}, c, nil, nil, nil, nil)
	c.ShutdownResponseTimeoutMS = 2000
	after := censusInventoryScopeBytesPrivate(censusCLIOptions{}, c, nil, nil, nil, nil)
	if bytes.Equal(before, after) || !bytes.Contains(after, []byte(`"ShutdownResponseTimeoutMS":2000`)) {
		t.Fatal("allowance not frozen into scope")
	}
}

// The real CLI factory starts two original owned stdio child processes. Neither
// detached report data nor an outer Wait substitutes for the inner cleanup facts.
func TestCensusPrivateShutdownResponseCLI(t *testing.T) {
	for _, tc := range []struct {
		name, delay, provenance string
		ms                      int
		success                 bool
	}{
		{"late-private", "late", "PRIVATE_V2", 2000, true},
		{"late-zero", "late", "ORDINARY_FIXED", 0, false},
		{"absent-private", "absent", "PRIVATE_V2", 2000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LSP_TRACE_OWNED_SHUTDOWN_DELAY", tc.delay)
			base, _, path, args := inventoryTwoFixturePrivate(t)
			c, err := loadCensusInventoryConfigPrivate(path)
			if err != nil {
				t.Fatal(err)
			}
			c.ShutdownResponseTimeoutMS = tc.ms
			saveInventoryConfigPrivate(t, path, c)
			x := newTerminalProcessRunnerPrivate(t, base, args)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, x.binary, append([]string{"census"}, x.args...)...)
			cmd.Env = append(os.Environ(), "LSP_TRACE_OWNED_TWO_ORIGIN_SERVER=1", "LSP_TRACE_OWNED_TERMINAL_SERVER=0")
			out, runErr := cmd.Output()
			if ctx.Err() != nil {
				t.Fatal("outer wait expired")
			}
			var meta struct{ AttemptDirectory string }
			if json.Unmarshal(out, &meta) != nil || meta.AttemptDirectory == "" {
				t.Fatalf("CLI setup: %v %s", runErr, out)
			}
			raw, err := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
			if err != nil {
				t.Fatal(err)
			}
			var r censusInventoryReportPrivate
			if json.Unmarshal(raw, &r) != nil {
				t.Fatal("report decode")
			}
			if len(r.Origins) == 0 {
				t.Fatal("no original origin reached cleanup")
			}
			for _, o := range r.Origins {
				if o.StopCalls == 0 && !tc.success {
					continue
				} // fail-closed leaves later origins unattempted
				if o.PipelineFailure != nil {
					t.Fatalf("producer failure excludes adapter RED: %+v", o.PipelineFailure)
				}
				f := o.Cleanup
				if f == nil {
					t.Fatal("missing inner lifecycle observation")
				}
				t.Logf("origin=%d cleanup=%+v", o.Ordinal, *f)
				if f.ResponseBudgetSource != tc.provenance {
					t.Fatalf("ASSERT_PRIVATE_ADAPTER: got %s / %s", f.ResponseBudgetSource, f.ShutdownRead)
				}
				declared := 250 * time.Millisecond
				if tc.ms != 0 {
					declared = time.Duration(tc.ms) * time.Millisecond
				}
				if f.DeclaredResponseBudget != declared {
					t.Fatal("wrong declared allowance")
				}
				if !f.ReaderJoined || f.RetiredTransportReused {
					t.Fatal("reader ownership/retirement violation")
				}
				if tc.success {
					if f.ShutdownRead != sessionruntime.ShutdownMatchedPrivateV2 || !o.CleanupVerified || !o.ShutdownJoined {
						t.Fatalf("matched/joined cleanup required: %+v", o)
					}
				} else if f.ShutdownRead != sessionruntime.ShutdownTimeoutPrivateV2 || o.CleanupVerified {
					t.Fatalf("timeout must not become success: %+v", o)
				}
			}
			if tc.success && len(r.Origins) != 2 {
				t.Fatalf("fresh owners not reached: %+v", r.Origins)
			}
		})
	}
}
