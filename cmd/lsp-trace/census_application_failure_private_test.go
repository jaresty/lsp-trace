//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

func applicationMockCompletedPrivate(t *testing.T, operationMS ...int) (*censusApplicationProgressPrivate, censusInventoryReportPrivate, string, []string) {
	t.Helper()
	base, _, config, args := applicationFixturePrivate(t)
	if len(operationMS) == 1 {
		c, e := loadCensusInventoryConfigPrivate(config)
		if e != nil {
			t.Fatal(e)
		}
		c.OperationTimeoutMS, c.Policy.TimeoutMS = operationMS[0], operationMS[0]
		for i, arg := range args {
			if arg == "--timeout" {
				args[i+1] = fmt.Sprintf("%dms", operationMS[0])
			}
		}
		saveInventoryConfigPrivate(t, config, c)
	}
	deps, _, _ := inventoryTwoDepsPrivate(t)
	first, code := runInventoryCLIPrivate(t, args, deps)
	if code != 0 {
		t.Fatal(first.Failure)
	}
	c, e := loadCensusInventoryConfigPrivate(config)
	if e != nil {
		t.Fatal(e)
	}
	mailbox := filepath.Join(base, "fixture-mailbox")
	if e = os.Mkdir(mailbox, 0700); e != nil {
		t.Fatal(e)
	}
	c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: mailbox, Invocation: "byte-verifier-fixture", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: first.ScopeSHA256, TimeoutMS: 5000, OverallTimeoutMS: 20000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCalls: 64, MaxRequestBytes: 4 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 64 << 20, MaxOutputBytes: 8 << 20}}
	c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	saveInventoryConfigPrivate(t, config, c)
	deps, _, _ = inventoryTwoDepsPrivate(t)
	deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
		d := productionCensusInventoryDependenciesPrivate()
		d.automatic = func(prep.AutomaticHandoffConfigPrivateV2, string, prep.PostPreparationPrivateV2) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
			return inventorySemanticFixturePrivate{"owned-interpreter"}, inventorySemanticFixturePrivate{"owned-reviewer"}, nil
		}
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, d)
	}
	r, code := runInventoryCLIPrivate(t, args, deps)
	if code != 0 {
		t.Fatal(r.Failure, r.Errors)
	}
	// Unit verifier fixture only: this attests a synchronous in-process return,
	// not an actual outer CLI Wait. The separate process test supplies real Wait.
	applicationHostObservationPrivate(t, r, 0, os.Getpid(), true, false)
	c.Automation = nil
	saveInventoryConfigPrivate(t, config, c)
	o, e := parseCensusCLIOptions(args)
	if e != nil {
		t.Fatal(e)
	}
	files, e := enumerateCensusPreflightContextPrivate(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	rows, _ := censusInventoryPlanPrivate(o, c, files)
	var scope struct{ Files []censusInventoryPagePrivate }
	b, e := os.ReadFile(filepath.Join(r.AttemptDirectory, "scope.json"))
	if e != nil || json.Unmarshal(b, &scope) != nil {
		t.Fatal("scope setup")
	}
	// Pure byte-verification setup only. Production computes this digest from
	// its fresh resolved runner and never accepts a DTO as runtime authority.
	var pinned censusApplicationFullPlanPrivate
	planRaw, e := os.ReadFile(filepath.Join(c.Application.Directory, "full-source-plan.json"))
	if e != nil || json.Unmarshal(planRaw, &pinned) != nil {
		t.Fatal("runtime binding setup")
	}
	state, e := openCensusApplicationProgressPrivate(context.Background(), o, c, rows, scope.Files, r.AttemptDirectory, pinned.RuntimeConfigurationSHA256)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = state.store.close() })
	return state, r, config, args
}
func copyApplicationFixturePrivate(t *testing.T, from, to, skip string) {
	t.Helper()
	if e := os.MkdirAll(to, 0700); e != nil {
		t.Fatal(e)
	}
	e := filepath.WalkDir(from, func(path string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(from, path)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		if skip != "" && strings.HasPrefix(rel, skip) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dest := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(dest, raw, 0600)
	})
	if e != nil {
		t.Fatal(e)
	}
}
func cloneApplicationStatePrivate(t *testing.T, original *censusApplicationProgressPrivate) *censusApplicationProgressPrivate {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private-receipts")
	copyApplicationFixturePrivate(t, original.store.path, dir, "")
	store, e := openCensusApplicationStorePrivate(context.Background(), dir, 128<<20)
	if e != nil {
		t.Fatal(e)
	}
	s := *original
	s.store = store
	s.next = 0
	s.verified = nil
	s.failure = ""
	s.claimed = false
	t.Cleanup(func() { _ = store.close() })
	return &s
}
func rewriteApplicationFixturePrivate(t *testing.T, path string, value any) {
	t.Helper()
	raw, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestCensusApplicationReceiptRefusalsPrivate(t *testing.T) {
	original, _, _, _ := applicationMockCompletedPrivate(t)
	for _, kind := range []string{"missing-host", "foreign-host", "out-of-order", "duplicate", "failed-host", "malformed", "foreign-selector", "missing-report", "tiny-read-budget"} {
		t.Run(kind, func(t *testing.T) {
			s := cloneApplicationStatePrivate(t, original)
			hostName := censusApplicationLeafPrivate(0, "host")
			raw, e := os.ReadFile(filepath.Join(s.store.path, hostName))
			if e != nil {
				t.Fatal(e)
			}
			var host censusApplicationHostObservationPrivate
			if e = json.Unmarshal(raw, &host); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "missing-host":
				if e = os.Rename(filepath.Join(s.store.path, hostName), filepath.Join(t.TempDir(), "held-host.json")); e != nil {
					t.Fatal(e)
				}
			case "foreign-host":
				host.FullPlanSHA256 = "sha256:" + strings.Repeat("0", 64)
				rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, hostName), host)
			case "out-of-order":
				if e = os.Rename(filepath.Join(s.store.path, hostName), filepath.Join(s.store.path, censusApplicationLeafPrivate(1, "host"))); e != nil {
					t.Fatal(e)
				}
			case "duplicate":
				for _, suffix := range []string{"claim", "result", "host"} {
					b, e := os.ReadFile(filepath.Join(s.store.path, censusApplicationLeafPrivate(0, suffix)))
					if e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(filepath.Join(s.store.path, censusApplicationLeafPrivate(1, suffix)), b, 0600); e != nil {
						t.Fatal(e)
					}
				}
			case "failed-host":
				host.ExitCode = 1
				rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, hostName), host)
			case "malformed":
				if e = os.WriteFile(filepath.Join(s.store.path, hostName), []byte(`{"Schema":"bad","Schema":"duplicate"}`), 0600); e != nil {
					t.Fatal(e)
				}
			case "foreign-selector":
				if e = os.WriteFile(filepath.Join(s.store.path, "foreign.json"), []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
			case "missing-report":
				claimPath := filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "claim"))
				b, e := os.ReadFile(claimPath)
				if e != nil {
					t.Fatal(e)
				}
				var claim censusApplicationClaimPrivate
				if json.Unmarshal(b, &claim) != nil {
					t.Fatal("claim")
				}
				claim.AttemptDirectory = filepath.Join(s.options.PublicationRoot, "private-census-inventory-absent")
				rewriteApplicationFixturePrivate(t, claimPath, claim)
				b, _ = json.Marshal(claim)
				host.ClaimSHA256 = pilot.Digest(b)
				host.AttemptDirectory = claim.AttemptDirectory
				resultPath := filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "result"))
				b, e = os.ReadFile(resultPath)
				if e != nil {
					t.Fatal(e)
				}
				var result censusApplicationResultPrivate
				_ = json.Unmarshal(b, &result)
				result.ClaimSHA256 = host.ClaimSHA256
				rewriteApplicationFixturePrivate(t, resultPath, result)
				b, _ = json.Marshal(result)
				host.ResultSHA256 = pilot.Digest(b)
				rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, hostName), host)
			case "tiny-read-budget":
				s.store.remaining = 1
			}
			err := s.scan()
			assertInventoryPrivate(t, "ASSERT_APPLICATION_RECEIPT_REFUSED_"+kind, err != nil && s.next < 2)
		})
	}
}

// Re-seal only the metadata of an explicitly negative byte-verifier fixture.
// Original produced pairs/receipts are never edited, unlinked, or replaced.
func TestCensusApplicationPairRefusalsPrivate(t *testing.T) {
	original, r, _, _ := applicationMockCompletedPrivate(t)
	for _, kind := range []string{"missing", "alias", "byte-change", "producer-missing", "report-change", "undersized-artifact"} {
		t.Run(kind, func(t *testing.T) {
			s := cloneApplicationStatePrivate(t, original)
			pub := filepath.Join(t.TempDir(), "publication")
			attempt := filepath.Join(pub, "private-census-inventory-negative")
			skip := ""
			if kind == "missing" || kind == "alias" {
				skip = "inventory-1"
			}
			if kind == "producer-missing" {
				skip = "producer-0-1"
			}
			copyApplicationFixturePrivate(t, r.AttemptDirectory, attempt, skip)
			if kind == "alias" {
				if e := os.Mkdir(filepath.Join(attempt, "inventory-1"), 0700); e != nil {
					t.Fatal(e)
				}
				names, e := os.ReadDir(filepath.Join(attempt, "inventory-0"))
				if e != nil || len(names) != 1 {
					t.Fatal("pair setup")
				}
				if e = os.Link(filepath.Join(attempt, "inventory-0", names[0].Name()), filepath.Join(attempt, "inventory-1", names[0].Name())); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "byte-change" {
				names, e := os.ReadDir(filepath.Join(attempt, "inventory-1"))
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(attempt, "inventory-1", names[0].Name()), []byte("wrong bytes"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			s.options.PublicationRoot = pub
			claimPath := filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "claim"))
			b, e := os.ReadFile(claimPath)
			if e != nil {
				t.Fatal(e)
			}
			var claim censusApplicationClaimPrivate
			_ = json.Unmarshal(b, &claim)
			claim.AttemptDirectory = attempt
			rewriteApplicationFixturePrivate(t, claimPath, claim)
			b, _ = json.Marshal(claim)
			claimSHA := pilot.Digest(b)
			report := r
			report.AttemptDirectory = attempt
			binding := *r.ApplicationBinding
			binding.ClaimSHA256 = claimSHA
			report.ApplicationBinding = &binding
			rewriteApplicationFixturePrivate(t, filepath.Join(attempt, "report.json"), report)
			b, _ = json.Marshal(report)
			reportSHA := pilot.Digest(b)
			result := censusApplicationResultPrivate{"lsp-trace.census-application-result.private.v1", claimSHA, reportSHA, r.Status, ""}
			rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "result")), result)
			b, _ = json.Marshal(result)
			resultSHA := pilot.Digest(b)
			host := censusApplicationHostObservationPrivate{Schema: "lsp-trace.census-application-host-process.private.v1", Trust: censusApplicationHostTrustPrivate, FullPlanSHA256: s.id, Segment: 0, ProcessID: claim.ProcessID, ExitCode: 0, Joined: true, AttemptDirectory: attempt, ClaimSHA256: claimSHA, ResultSHA256: resultSHA, ReportSHA256: reportSHA, ScopeSHA256: claim.ScopeSHA256, NativeSHA256: r.Outcome.EnvelopeSHA256, NativeBytes: r.Outcome.ByteLength}
			rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "host")), host)
			if kind == "report-change" {
				report.Status = "altered"
				rewriteApplicationFixturePrivate(t, filepath.Join(attempt, "report.json"), report)
			}
			if kind == "undersized-artifact" {
				config := *s.config.Application
				config.MaxArtifactBytes = 1
				s.config.Application = &config
			}
			_, err := s.verify(0)
			assertInventoryPrivate(t, "ASSERT_APPLICATION_CONSTITUENT_REFUSED_"+kind, err != nil)
		})
	}
}
func TestCensusApplicationPlanDriftPrivate(t *testing.T) {
	for _, kind := range []string{"source", "configuration", "controls", "runtime", "approval"} {
		t.Run(kind, func(t *testing.T) {
			base, src, config, args := applicationFixturePrivate(t)
			deps, _, _ := inventoryTwoDepsPrivate(t)
			first, code := runInventoryCLIPrivate(t, args, deps)
			if code != 0 {
				t.Fatal(first.Failure)
			}
			c, e := loadCensusInventoryConfigPrivate(config)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "source":
				if e = os.WriteFile(filepath.Join(src, "eligible-061.go"), []byte("package owned\n\nfunc A0() { /* late partition */ }\n"), 0600); e != nil {
					t.Fatal(e)
				}
			case "configuration":
				c.Seeds = []uint64{44, 45}
				saveInventoryConfigPrivate(t, config, c)
			case "controls":
				args = append(args, "--server-arg", "changed-control")
			case "approval":
				mailbox := filepath.Join(base, "wrong-scope-mailbox")
				if e = os.Mkdir(mailbox, 0700); e != nil {
					t.Fatal(e)
				}
				c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: mailbox, Invocation: "wrong-scope", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: "sha256:" + strings.Repeat("0", 64), TimeoutMS: 5000, OverallTimeoutMS: 20000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCalls: 64, MaxRequestBytes: 4 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 64 << 20, MaxOutputBytes: 8 << 20}}
				c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
				saveInventoryConfigPrivate(t, config, c)
			}
			deps, starts, _ := inventoryTwoDepsPrivate(t)
			if kind == "runtime" {
				deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
					cfg.runner.start.Process.Args = append(cfg.runner.start.Process.Args, "changed-resolved-runtime")
					return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, productionCensusInventoryDependenciesPrivate())
				}
			}
			r, code := runInventoryCLIPrivate(t, args, deps)
			assertInventoryPrivate(t, "ASSERT_APPLICATION_DRIFT_BEFORE_ACQUISITION_"+kind, code == 1 && starts.Load() == 0 && r.Outcome == nil)
		})
	}
}
func TestCensusApplicationBoundedIOPrivate(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink", "oversize", "cancel", "stale-parent", "budget"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "root")
			if e := os.Mkdir(dir, 0700); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s, e := openCensusApplicationStorePrivate(ctx, dir, 32)
			if e != nil {
				t.Fatal(e)
			}
			defer s.close()
			path := filepath.Join(dir, "receipt.json")
			switch kind {
			case "fifo":
				if e = syscall.Mkfifo(path, 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				if e = os.Symlink("other.json", path); e != nil {
					t.Fatal(e)
				}
			case "oversize":
				if e = os.WriteFile(path, make([]byte, 33), 0600); e != nil {
					t.Fatal(e)
				}
			default:
				if e = os.WriteFile(path, []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "cancel" {
				cancel()
			}
			if kind == "budget" {
				s.remaining = 1
			}
			if kind == "stale-parent" {
				if e = os.Rename(dir, filepath.Join(parent, "retired")); e != nil {
					t.Fatal(e)
				}
				if e = os.Mkdir(dir, 0700); e != nil {
					t.Fatal(e)
				}
			}
			started := time.Now()
			_, err := s.read("receipt.json", 32)
			assertInventoryPrivate(t, "ASSERT_APPLICATION_BOUNDED_IO_"+kind, err != nil && time.Since(started) < time.Second)
		})
	}
}
func TestCensusApplicationPlanAccountingLimitsPrivate(t *testing.T) {
	for _, kind := range []string{"plan-bytes", "segments", "accounting-bytes", "classification"} {
		t.Run(kind, func(t *testing.T) {
			_, src, config, args := applicationFixturePrivate(t)
			c, e := loadCensusInventoryConfigPrivate(config)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "plan-bytes" {
				c.Application.MaxPlanBytes = 1
			}
			if kind == "segments" {
				c.Application.MaxSegments = 1
			}
			if kind == "accounting-bytes" {
				c.Application.MaxArtifactBytes = 1
			}
			if kind == "classification" {
				if e = os.WriteFile(filepath.Join(src, "README.unknown"), []byte("owned metadata fixture"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			saveInventoryConfigPrivate(t, config, c)
			deps, starts, _ := inventoryTwoDepsPrivate(t)
			r, code := runInventoryCLIPrivate(t, args, deps)
			if kind == "classification" {
				raw, e := os.ReadFile(filepath.Join(r.AttemptDirectory, "application-accounting.json"))
				if e != nil {
					t.Fatal(e)
				}
				var view censusApplicationAccountingPrivate
				if json.Unmarshal(raw, &view) != nil {
					t.Fatal("accounting")
				}
				assertInventoryPrivate(t, "ASSERT_APPLICATION_CLASSIFICATION_ACCOUNTED", code == 0 && view.FileDenominator == 73 && view.ExcludedFiles == 2 && view.UnattemptedFiles == 71)
			} else {
				assertInventoryPrivate(t, "ASSERT_APPLICATION_LIMIT_REFUSED_"+kind, code == 1 && (kind == "accounting-bytes" || starts.Load() == 0))
			}
		})
	}
}
