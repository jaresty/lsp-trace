//go:build darwin

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

func TestHostApplicationReadinessCombinedTwenty(t *testing.T) {
	r, path, base := readinessLiteralFixture(t)
	readinessCombinedTwenty(t, r, path, base, "chosen/deeper/leaf/twenty.go")
}

func readinessCombinedTwenty(t *testing.T, r hostApplicationRequest, path, base, selected string) {
	t.Helper()
	r = readinessUnitTime(t, r, 180000)
	// Explicit proposed envelope, not a change to any default or global capacity.
	r.UnitEnvelope.MaxCalls = 8
	r.UnitEnvelope.MaxRequestBytes = 128 << 10
	r.UnitEnvelope.MaxTotalBytes = (1 << 20) + (64 << 10)
	r.UnitEnvelope.MaxOutputBytes = 1 << 20
	r.Template.ComputationUnitBytes = 1 << 20
	r.Template.Policy.Planning.MaxBatchTargets = 20
	for n, arg := range r.Template.CensusArguments {
		if arg == "--batch-targets" {
			r.Template.CensusArguments[n+1] = "20"
		}
	}
	r.Template.ApplicationAllowance.LogicalBytes = 96 << 30
	r.Template.ApplicationAllowance.ReadHashBytes = 96 << 30
	r.Template.Configuration.Exchange = r.Template.ApplicationAllowance
	r.Template.ExchangeTimeoutMS = 60000
	writeApplicationFixture(t, r, path)
	r, e := readHostApplication(path)
	if e != nil {
		t.Fatal(e)
	}
	tool := buildApplicationTool(t, base)
	started := time.Now()
	markers := []string{"TEST_COUNT_PHYSICAL=1"}
	if os.Getenv("TEST_FIXTURE_WINDOW_BARRIER") != "1" {
		// Explicit production opt-in; the eager helper default remains unchanged.
		if os.Getenv("TEST_FIXTURE_BRIDGE") == "1" {
			markers = append(markers, "TEST_HOST_BRIDGE=1")
		} else {
			markers = append(markers, "TEST_HOST_WRAPPER=1")
		}
	}
	report, e := controlledApplicationRun(t, tool, path, markers...)
	if e != nil || report.Status != "PROVISIONAL" || report.Units != 1 || report.Targets != 20 || report.Calls != 4 || report.Selected != 1 || report.Discovered != 1 || report.Denominator != "1" || report.Reserved.ActiveNS > uint64(240*time.Second) || report.Reserved.Calls != 8 || report.Reserved.RequestBytes > 1<<20 || report.Reserved.ResponseBytes > 64<<10 || report.Reserved.WireBytes > 8<<20 || report.Reserved.OutputBytes > 4<<20 {
		t.Fatalf("ASSERT_COMBINED_LITERAL_UNIT180_TWENTY A-fail report=%+v err=%v", report, e)
	}
	if os.Getenv("TEST_FIXTURE_DISCOVERY_DELAY") == "1" {
		barriers, er := os.ReadFile(os.Getenv("TEST_FIXTURE_DISCOVERY_MARKER"))
		if er != nil || len(barriers) != 4 || time.Since(started) < 8400*time.Millisecond {
			t.Fatalf("ASSERT_PREPAID_DELAY_BEYOND_STARTUP_POOL A-fail barriers=%d error=%v elapsed=%s", len(barriers), er, time.Since(started))
		}
		t.Logf("ASSERT_PREPAID_DELAY_BEYOND_STARTUP_POOL A-pass barriers=%d delay=4x2100ms prepaid=4x3s no_new_entitlement=true", len(barriers))
	}
	roots, e := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*"))
	if e != nil || len(roots) != 1 {
		t.Fatal(roots, e)
	}
	raw, e := os.ReadFile(filepath.Join(roots[0], "source.json"))
	if e != nil {
		t.Fatal(e)
	}
	var source hostProductRequestV3
	if e = json.Unmarshal(raw, &source); e != nil {
		t.Fatal(e)
	}
	sourceMatch := 0
	for n, arg := range source.CensusArguments {
		if arg == "--source" && source.CensusArguments[n+1] == selected {
			sourceMatch++
		}
	}
	if sourceMatch != 1 || source.Configuration.Requested != 20 || source.FileDenominator != 1 || source.ExchangeTimeoutMS != 60000 {
		t.Fatal("ASSERT_COMBINED_NATIVE_ONE_SOURCE_BINDING", source)
	}
	a, e := product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	cp, e := a.VerifyInventory(report.InventoryHead)
	if e != nil {
		t.Fatal(e)
	}
	var page product.ApplicationInventoryPage
	if e = a.GetRecord(cp.PageSHA256, &page); e != nil || len(page.Rows) != 1 || page.Rows[0].Requested != 20 || page.Rows[0].Reviewed != 20 || page.Rows[0].Configuration.ScopeSHA256 != source.Configuration.ScopeSHA256 {
		t.Fatal("ASSERT_COMBINED_NATIVE_TWENTY_REVIEWED", page, e)
	}
	u := product.UnitBinding{ApplicationSHA256: product.ApplicationConfigDigest(r.Configuration), UnitID: filepath.Base(roots[0])}
	u.UnitID = "sha256:" + u.UnitID
	deadline, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID)
	if e != nil {
		t.Fatal(e)
	}
	if time.Until(deadline) > 180*time.Second || time.Until(deadline) < 60*time.Second {
		t.Fatal("ASSERT_COMBINED_STORED_UNIT180_WINDOW", time.Until(deadline))
	}
	waitRefs, e := filepath.Glob(filepath.Join(r.Configuration.Store, "host-wait-unit-*-01.json"))
	if e != nil || len(waitRefs) != 1 {
		t.Fatal("missing actual HOST unit window", waitRefs, e)
	}
	waitName := filepath.Base(waitRefs[0])
	ticket := "sha256:" + strings.TrimSuffix(strings.TrimPrefix(waitName, "host-wait-unit-"), "-01.json")
	hostDeadline, e := applicationWaitChildDeadline(r.Configuration.Store, u.ApplicationSHA256, ticket)
	if e != nil || hostDeadline.After(deadline.Add(5*time.Millisecond)) {
		t.Fatal("ASSERT_COMBINED_HOST_SAME_PREPAID_WINDOW", hostDeadline, deadline, e)
	}
	var cursor applicationState
	if e = a.GetRecord(a.Head().CursorSHA256, &cursor); e != nil {
		t.Fatal(e)
	}
	// Exact native source identity comes from the original Census-derived scope,
	// not a fabricated one-file owner. Re-freezing THIS controlled selected file
	// through the same native enumerator must reproduce the original scope digest.
	options, e := parseCensusCLIOptions(source.CensusArguments)
	if e != nil {
		t.Fatal(e)
	}
	files, e := enumerateCensusPreflightContextPrivate(context.Background(), options)
	if e != nil || len(files) != 1 || files[0].Path != selected {
		t.Fatal("ASSERT_COMBINED_OBSERVED_SOURCE_ID", files, e)
	}
	frozen, e := pilot.FreezeCensusScopePrivateV2(context.Background(), options.Workspace, r.Configuration.RevisionAssertion, files, source.Policy)
	if e != nil || pilot.Digest(frozen.ManifestBytes()) != source.Configuration.ScopeSHA256 {
		t.Fatal("ASSERT_COMBINED_OBSERVED_SOURCE_ID", e)
	}
	t.Logf("ASSERT_COMBINED_OBSERVED_SOURCE_ID A-pass path=%s scope=%s", files[0].Path, source.Configuration.ScopeSHA256)
	t.Logf("ASSERT_COMBINED_LITERAL_UNIT180_TWENTY A-pass calls=%d request=%d response=%d reserved=%+v owned=%+v child-policy=%+v literal=%+v deadline-remaining=%s", report.Calls, report.RequestBytes, report.ResponseBytes, report.Reserved, report.KnownOwnedWork, report.ChildLedgerPolicyDebits, report.LiteralDiscovery, time.Until(deadline))
}

type readinessStopContext struct {
	context.Context
	stop func() bool
}

func (c readinessStopContext) Err() error {
	if c.stop() {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestHostApplicationReadinessLiteralRestart(t *testing.T) {
	for _, mode := range []string{"resume", "growth", "symlink", "ancestor-alias"} {
		t.Run(mode, func(t *testing.T) {
			r, p, _ := readinessLiteralFixture(t)
			writeApplicationFixture(t, r, p)
			r, e := readHostApplication(p)
			if e != nil {
				t.Fatal(e)
			}
			a, e := product.CreateApplication(r.Configuration, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			st, e := os.Stat(r.Configuration.SourceRoot)
			if e != nil {
				t.Fatal(e)
			}
			s := applicationState{Schema: "private-application-cursor.v1", ApplicationSHA256: product.ApplicationConfigDigest(r.Configuration), Directory: ".", DirectoryIdentity: applicationFileIdentity(st)}
			if e = saveApplicationState(a, s); e != nil {
				t.Fatal(e)
			}
			// Cancel ONLY after an actual catalog/cursor transition to the next directory.
			ctx := readinessStopContext{context.Background(), func() bool { return s.Directory != "." }}
			if e = advanceApplicationDiscovery(ctx, r, a, &s); !errors.Is(e, context.Canceled) {
				t.Fatal("expected owned interruption", e)
			}
			if s.SelectedCount != 0 || s.LiteralDiscovery.CandidatePages != 1 {
				t.Fatal("not interrupted at intended cursor", s)
			}
			a.Close()
			if mode == "growth" {
				if e = os.WriteFile(filepath.Join(r.Configuration.SourceRoot, "new.go"), []byte("package new\n"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "ancestor-alias" {
				old := filepath.Join(r.Configuration.SourceRoot, "chosen", "deeper")
				if e = os.Rename(old, filepath.Join(r.Configuration.SourceRoot, "chosen", "Deeper")); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "symlink" {
				old := filepath.Join(r.Configuration.SourceRoot, "chosen")
				moved := old + "-old"
				if e = os.Rename(old, moved); e != nil {
					t.Fatal(e)
				}
				if e = os.Symlink(moved, old); e != nil {
					t.Fatal(e)
				}
			}
			a, e = product.OpenApplication(r.Configuration, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
				t.Fatal(e)
			}
			before := a.Head().Reserved.ActiveNS
			e = advanceApplicationDiscovery(context.Background(), r, a, &s)
			if mode == "resume" {
				if e != nil || !s.DiscoveryFinished || s.SelectedCount != 1 {
					t.Fatal("ASSERT_LITERAL_RESTART_RESUME", s, e)
				}
			} else if e == nil || s.DiscoveryFinished || s.SelectedCount != 0 {
				t.Fatal("ASSERT_LITERAL_RESTART_STALE_REFUSED", s, e)
			}
			if a.Head().Reserved.ActiveNS <= before {
				t.Fatal("restart validation was not prepaid")
			}
			t.Logf("ASSERT_LITERAL_RESTART A-pass mode=%s finished=%t selected=%d", mode, s.DiscoveryFinished, s.SelectedCount)
		})
	}
}

func TestHostApplicationReadinessExplicitShortNoReset(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	tool := buildApplicationTool(t, base)
	r = readinessUnitTime(t, r, 3000)
	writeApplicationFixture(t, r, path)
	r, e := readHostApplication(path)
	if e != nil {
		t.Fatal(e)
	}
	a, e := product.CreateApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	u, source, _, e := readinessAdmission(t, r, a, "explicit-short-blocked")
	if e != nil {
		t.Fatal(e)
	}
	st, e := os.Stat(filepath.Join(r.Configuration.SourceRoot, "unit0.go"))
	if e != nil {
		t.Fatal(e)
	}
	entry := applicationCatalogEntry{ApplicationSHA256: u.ApplicationSHA256, Path: "unit0.go", Identity: applicationFileIdentity(st)}
	id, e := a.PutRecord(struct {
		Unit   product.UnitBinding
		Entry  applicationCatalogEntry
		Source hostProductRequestV3
	}{u, entry, source})
	if e != nil {
		t.Fatal(e)
	}
	deadline, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID)
	if e != nil {
		t.Fatal(e)
	}
	a.Close()
	time.Sleep(time.Until(deadline) - 1200*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	root := filepath.Join(r.Configuration.Store, "units", u.UnitID[7:])
	cmd := exec.CommandContext(ctx, tool, "private-host-application", "acquire", path, id, root)
	cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1", "LSP_HOST_APPLICATION_BLOCK_DOC=1")
	start := time.Now()
	_, runErr := cmd.CombinedOutput()
	cap := time.Until(deadline) + time.Since(start) + 500*time.Millisecond
	if applicationFixtureRace {
		cap += time.Second
	}
	if runErr == nil || time.Since(start) < 200*time.Millisecond || time.Since(start) > cap {
		t.Fatal("ASSERT_EXPLICIT_UNIT_NO_RESET_BLOCKED_CHILD", runErr, time.Since(start), cap)
	}
	if _, e = os.Stat(filepath.Join(root, "application-original-claim.json")); e != nil {
		t.Fatal("did not reach original child", e)
	}
	if _, e = os.Stat(filepath.Join(root, "auxiliary", "ledger")); e != nil {
		t.Fatal("did not reach controlled provider", e)
	}
	t.Logf("ASSERT_EXPLICIT_UNIT_NO_RESET_BLOCKED_CHILD A-pass elapsed=%s", time.Since(start))
}
