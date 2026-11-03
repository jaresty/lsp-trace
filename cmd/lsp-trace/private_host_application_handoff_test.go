//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

func handoffBind(r hostApplicationRequest) hostApplicationRequest {
	p := hostApplicationUnitPolicyDigest(r)
	r.Configuration.UnitPolicySHA256, r.Configuration.ProducerConfigSHA256 = p, p
	r.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(r.Configuration)
	r.ApprovalSHA256 = hostApplicationDigest(r)
	return r
}

// Real production decoder/catalog/prepayment/original acquisition, with the
// existing synthetic stdio provider. No model or HOST semantic role is launched.
// Cancellation after source.json prevents waiting for semantic role activation.
func TestHostApplicationHandoffEarlyNative(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	r = readinessUnitTime(t, r, 180000)
	r.UnitEnvelope.MaxCalls = 6
	r.UnitEnvelope.MaxRequestBytes = 128 << 10
	r.UnitEnvelope.MaxTotalBytes = 835584
	r.MaxUnitsPerRun = 1
	for n := 0; n < 80; n++ {
		dir := filepath.Join(r.Configuration.SourceRoot, fmt.Sprintf("bulk%04d", n))
		if e := os.Mkdir(dir, 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, "entry.go"), []byte("package owned\n\nfunc Owned00() {}\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	// Default request: no discovery mode or caller-picked source unit.
	r = handoffBind(r)
	writeApplicationFixture(t, r, path)
	if _, e := readHostApplication(path); e != nil {
		t.Fatalf("binding prerequisite: %v", e)
	}
	tool := buildApplicationTool(t, base)
	if applicationFixtureRace {
		info, e := exec.Command("go", "version", "-m", tool).CombinedOutput()
		if e != nil || !bytes.Contains(info, []byte("-race=true")) {
			t.Fatalf("ASSERT_APPLICATION_HANDOFF_HELPER_RACE A-fail err=%v info=%s", e, info)
		}
		t.Log("ASSERT_APPLICATION_HANDOFF_HELPER_RACE A-pass -race=true")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path)
	cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1")
	var out, diag bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diag
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	native := 0
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	finished := false
	for !finished {
		select {
		case <-timer.C:
			paths, _ := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*", "source.json"))
			if len(paths) > 0 {
				raw, e := os.ReadFile(paths[0])
				var source hostProductRequestV3
				if e == nil && json.Unmarshal(raw, &source) == nil {
					if native == 0 {
						native = source.Configuration.Requested
						_ = cmd.Process.Signal(os.Interrupt)
					}
				}
			}
		case <-done:
			finished = true
		case <-ctx.Done():
			cancel()
			<-done
			finished = true
		}
	}
	a, e := product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatalf("open after child: %v native=%d out=%s diag=%s", e, native, out.String(), diag.String())
	}
	defer a.Close()
	var s applicationState
	if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
		t.Fatal(e)
	}
	if native == 0 || s.DiscoveryFinished || a.Head().Reserved.ActiveNS > uint64(240*time.Second) {
		t.Fatalf("ASSERT_APPLICATION_HANDOFF_DEFAULT_EARLY_NATIVE A-fail native=%d finished=%t entries=%d active=%d out=%s diag=%s", native, s.DiscoveryFinished, s.Entries, a.Head().Reserved.ActiveNS, out.String(), diag.String())
	}
	t.Logf("ASSERT_APPLICATION_HANDOFF_DEFAULT_EARLY_NATIVE A-pass native=%d unfinished=%t entries=%d active=%d", native, !s.DiscoveryFinished, s.Entries, a.Head().Reserved.ActiveNS)
}

func handoffCatalogFixture(t *testing.T, literal bool, count int) (hostApplicationRequest, *product.ApplicationJournal, applicationState) {
	t.Helper()
	r, _, _ := hostApplicationCLIFixture(t)
	for n := 0; n < 3; n++ {
		if e := os.Remove(filepath.Join(r.Configuration.SourceRoot, fmt.Sprintf("unit%d.go", n))); e != nil {
			t.Fatal(e)
		}
	}
	args := []string{}
	for n := 0; n < len(r.Template.CensusArguments); n++ {
		if r.Template.CensusArguments[n] == "--include" {
			n++
			continue
		}
		args = append(args, r.Template.CensusArguments[n])
	}
	if literal {
		if e := os.Mkdir(filepath.Join(r.Configuration.SourceRoot, "chosen"), 0700); e != nil {
			t.Fatal(e)
		}
	} else {
		args = append(args, "--include", "**.go")
	}
	for n := 0; n < count; n++ {
		name := fmt.Sprintf("entry%03d.go", n)
		if literal {
			name = filepath.Join("chosen", name)
			args = append(args, "--include", filepath.ToSlash(name))
		}
		if e := os.WriteFile(filepath.Join(r.Configuration.SourceRoot, name), []byte("package owned\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r.Template.CensusArguments = args
	r = handoffBind(r)
	a, e := product.CreateApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	st, e := os.Lstat(r.Configuration.SourceRoot)
	if e != nil {
		t.Fatal(e)
	}
	s := applicationState{Schema: "private-application-cursor.v1", ApplicationSHA256: product.ApplicationConfigDigest(r.Configuration), Directory: ".", DirectoryIdentity: applicationFileIdentity(st)}
	if e = saveApplicationState(a, s); e != nil {
		t.Fatal(e)
	}
	return r, a, s
}

func TestHostApplicationHandoffRestart(t *testing.T) {
	for _, literal := range []bool{false, true} {
		t.Run(fmt.Sprint(literal), func(t *testing.T) {
			r, a, s := handoffCatalogFixture(t, literal, 41)
			defer func() { a.Close() }()
			if e := advanceApplicationDiscovery(context.Background(), r, a, &s); e != nil {
				t.Fatal(e)
			}
			if s.Entries != 8 || s.DiscoveryFinished || s.Selected == "" {
				t.Fatalf("ASSERT_APPLICATION_HANDOFF_PAGE A-fail entries=%d finished=%t", s.Entries, s.DiscoveryFinished)
			}
			selected := s.Selected
			active := a.Head().Reserved.ActiveNS
			entries := s.Entries
			if e := advanceApplicationDiscovery(context.Background(), r, a, &s); e != nil {
				t.Fatal(e)
			}
			if s.Selected != selected || s.Entries != entries || a.Head().Reserved.ActiveNS-active != uint64(time.Second) {
				t.Fatal("ASSERT_APPLICATION_HANDOFF_PENDING A-fail")
			}
			seen := map[string]bool{}
			for pass := 0; pass < 10; pass++ {
				for s.Selected != "" {
					var entry applicationCatalogEntry
					if e := a.GetRecord(s.Selected, &entry); e != nil {
						t.Fatal(e)
					}
					if seen[entry.Path] {
						t.Fatalf("ASSERT_APPLICATION_HANDOFF_RESTART A-fail duplicate %s", entry.Path)
					}
					seen[entry.Path] = true
					s.Selected = entry.Next
				}
				if e := saveApplicationState(a, s); e != nil {
					t.Fatal(e)
				}
				if e := a.Close(); e != nil {
					t.Fatal(e)
				}
				var e error
				a, e = product.OpenApplication(r.Configuration, time.Now())
				if e != nil {
					t.Fatal(e)
				}
				if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
					t.Fatal(e)
				}
				if s.DiscoveryFinished {
					break
				}
				before := a.Head().Reserved.ActiveNS
				if e = advanceApplicationDiscovery(context.Background(), r, a, &s); e != nil {
					t.Fatal(e)
				}
				if a.Head().Reserved.ActiveNS <= before {
					t.Fatal("resume reused paid claim")
				}
			}
			if len(seen) != 41 || s.Entries != 41 || !s.DiscoveryFinished || s.Selected != "" {
				t.Fatalf("ASSERT_APPLICATION_HANDOFF_RESTART A-fail seen=%d state=%+v", len(seen), s)
			}
			if literal && (s.LiteralDiscovery.MaxPageNames > 32 || s.LiteralDiscovery.DirectoryReads != 0 || s.LiteralDiscovery.CandidatePages < 2) {
				t.Fatal("literal scan or unbounded candidate page")
			}
			t.Logf("ASSERT_APPLICATION_HANDOFF_RESTART A-pass paths=%d literal=%t counters=%+v", len(seen), literal, s.LiteralDiscovery)
		})
	}
}

// The boundary trigger observes a real committed selected cursor, not an Err
// call count. It is instance-local: no package hook or cross-test race.
type handoffBoundaryContext struct {
	context.Context
	journal *product.ApplicationJournal
	fired   bool
	action  func() error
	err     error
}

func (c *handoffBoundaryContext) Err() error {
	if !c.fired && c.journal.Head().CursorSHA256 != "" {
		var s applicationState
		if c.journal.GetRecord(c.journal.Head().CursorSHA256, &s) == nil && s.Selected != "" {
			c.fired = true
			c.err = c.action()
		}
	}
	if c.err != nil {
		return c.err
	}
	return c.Context.Err()
}

func TestHostApplicationHandoffBoundaryRefusal(t *testing.T) {
	for _, literal := range []bool{false, true} {
		for _, kind := range []string{"cancel", "directory-change", "root-symlink"} {
			t.Run(fmt.Sprintf("%t/%s", literal, kind), func(t *testing.T) {
				r, a, s := handoffCatalogFixture(t, literal, 41)
				defer a.Close()
				ctx := &handoffBoundaryContext{Context: context.Background(), journal: a}
				ctx.action = func() error {
					switch kind {
					case "cancel":
						return context.Canceled
					case "root-symlink":
						old := r.Configuration.SourceRoot + "-moved"
						if e := os.Rename(r.Configuration.SourceRoot, old); e != nil {
							t.Fatal(e)
						}
						if e := os.Symlink(old, r.Configuration.SourceRoot); e != nil {
							t.Fatal(e)
						}
					default:
						dir := filepath.Join(r.Configuration.SourceRoot, s.Directory)
						if e := os.WriteFile(filepath.Join(dir, "changed"), []byte("boundary mutation"), 0600); e != nil {
							t.Fatal(e)
						}
					}
					return nil
				}
				e := advanceApplicationDiscovery(ctx, r, a, &s)
				if !ctx.fired || e == nil || a.Head().Reserved.Calls != 0 || s.Selected == "" || s.DiscoveryFinished {
					t.Fatalf("ASSERT_APPLICATION_HANDOFF_BOUNDARY A-fail fired=%t err=%v state=%+v", ctx.fired, e, s)
				}
				if kind == "cancel" && !errors.Is(e, context.Canceled) {
					t.Fatal(e)
				}
				t.Logf("ASSERT_APPLICATION_HANDOFF_BOUNDARY A-pass literal=%t mutation=%s err=%v", literal, kind, e)
			})
		}
	}
}

func TestHostApplicationHandoffPendingRefusal(t *testing.T) {
	for _, literal := range []bool{false, true} {
		for _, kind := range []string{"cancel", "directory-change", "directory-symlink"} {
			t.Run(fmt.Sprintf("%t/%s", literal, kind), func(t *testing.T) {
				r, a, s := handoffCatalogFixture(t, literal, 41)
				if e := advanceApplicationDiscovery(context.Background(), r, a, &s); e != nil {
					t.Fatal(e)
				}
				if e := a.Close(); e != nil {
					t.Fatal(e)
				}
				var e error
				a, e = product.OpenApplication(r.Configuration, time.Now())
				if e != nil {
					t.Fatal(e)
				}
				defer a.Close()
				if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
					t.Fatal(e)
				}
				selected := s.Selected
				entries := s.Entries
				ctx := context.Background()
				if kind == "cancel" {
					c, cancel := context.WithCancel(ctx)
					cancel()
					ctx = c
				} else {
					dir := filepath.Join(r.Configuration.SourceRoot, s.Directory)
					if kind == "directory-symlink" {
						if e = os.Rename(dir, dir+"-moved"); e != nil {
							t.Fatal(e)
						}
						if e = os.Symlink(dir+"-moved", dir); e != nil {
							t.Fatal(e)
						}
					} else if e = os.WriteFile(filepath.Join(dir, "mutation"), []byte("changed"), 0600); e != nil {
						t.Fatal(e)
					}
				}
				e = advanceApplicationDiscovery(ctx, r, a, &s)
				if e == nil || s.Selected != selected || s.Entries != entries || a.Head().Reserved.Calls != 0 {
					t.Fatalf("ASSERT_APPLICATION_HANDOFF_PENDING_REFUSAL A-fail err=%v", e)
				}
				t.Logf("ASSERT_APPLICATION_HANDOFF_PENDING_REFUSAL A-pass literal=%t mutation=%s", literal, kind)
			})
		}
	}
}

func TestHostApplicationHandoffLiteralAncestorRefusal(t *testing.T) {
	r, _, _ := readinessLiteralFixture(t)
	r = handoffBind(r)
	a, e := product.CreateApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	st, e := os.Lstat(r.Configuration.SourceRoot)
	if e != nil {
		t.Fatal(e)
	}
	s := applicationState{Schema: "private-application-cursor.v1", ApplicationSHA256: product.ApplicationConfigDigest(r.Configuration), Directory: ".", DirectoryIdentity: applicationFileIdentity(st)}
	if e = saveApplicationState(a, s); e != nil {
		t.Fatal(e)
	}
	if e = advanceApplicationDiscovery(context.Background(), r, a, &s); e != nil {
		t.Fatal(e)
	}
	if s.Selected == "" || s.DiscoveryFinished || s.Directory != "chosen/deeper/leaf" {
		t.Fatal("expected pending deep literal cursor", s)
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	a, e = product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
		t.Fatal(e)
	}
	rootBefore, e := os.Lstat(r.Configuration.SourceRoot)
	if e != nil {
		t.Fatal(e)
	}
	leafPath := filepath.Join(r.Configuration.SourceRoot, s.Directory)
	leafBefore, e := os.Lstat(leafPath)
	if e != nil {
		t.Fatal(e)
	}
	// Only an intermediate ancestor changes. Root/current leaf stay identical;
	// refusal therefore cannot be supplied by checking just those two endpoints.
	if e = os.WriteFile(filepath.Join(r.Configuration.SourceRoot, "chosen", "ancestor-change"), []byte("changed ancestor"), 0600); e != nil {
		t.Fatal(e)
	}
	rootAfter, e := os.Lstat(r.Configuration.SourceRoot)
	if e != nil {
		t.Fatal(e)
	}
	leafAfter, e := os.Lstat(leafPath)
	if e != nil {
		t.Fatal(e)
	}
	if applicationFileIdentity(rootBefore) != applicationFileIdentity(rootAfter) || applicationFileIdentity(leafBefore) != applicationFileIdentity(leafAfter) {
		t.Fatal("ancestor discriminator changed an endpoint")
	}
	before := a.Head().Reserved.ActiveNS
	e = advanceApplicationDiscovery(context.Background(), r, a, &s)
	if e == nil || s.Selected == "" || s.DiscoveryFinished || a.Head().Reserved.Calls != 0 || a.Head().Reserved.ActiveNS <= before {
		t.Fatalf("ASSERT_APPLICATION_HANDOFF_ANCESTOR_REFUSAL A-fail err=%v state=%+v", e, s)
	}
	t.Log("ASSERT_APPLICATION_HANDOFF_ANCESTOR_REFUSAL A-pass unchanged-root-and-leaf=true")
}

// Controlled full native/retained/publication pipeline under the normal path.
// Role responses are predetermined fixture answers, not real model judgments.
func TestHostApplicationHandoffPublishedPartial(t *testing.T) {
	for _, literal := range []bool{false, true} {
		t.Run(fmt.Sprint(literal), func(t *testing.T) {
			var r hostApplicationRequest
			var path, base string
			selected := "twenty.go"
			if literal {
				r, path, base = readinessLiteralFixture(t)
				selected = "chosen/deeper/leaf/twenty.go"
			} else {
				r, path, base = compatibilityOneFile(t)
			}
			r = readinessUnitTime(t, r, 180000)
			r.UnitEnvelope.MaxCalls = 6
			r.UnitEnvelope.MaxRequestBytes = 128 << 10
			r.UnitEnvelope.MaxTotalBytes = 835584
			r.UnitEnvelope.MaxOutputBytes = 1 << 20
			r.Template.ComputationUnitBytes = 1 << 20
			r.Template.Policy.Planning.MaxBatchTargets = 20
			for n, arg := range r.Template.CensusArguments {
				if arg == "--batch-targets" {
					r.Template.CensusArguments[n+1] = "20"
				}
			}
			// Existing controlled twenty-target calibration, not a production allowance change.
			r.Template.ApplicationAllowance.LogicalBytes = 96 << 30
			r.Template.ApplicationAllowance.ReadHashBytes = 96 << 30
			r.Template.Configuration.Exchange = r.Template.ApplicationAllowance
			r.MaxUnitsPerRun = 1
			writeApplicationFixture(t, r, path)
			var e error
			r, e = readHostApplication(path)
			if e != nil {
				t.Fatal(e)
			}
			report, e := controlledApplicationRun(t, buildApplicationTool(t, base), path, "TEST_COUNT_PHYSICAL=1")
			if e != nil || report.Status != "PAUSED_UNIT_SLICE" || report.Denominator != "UNKNOWN" || report.Units != 1 || report.Targets != 20 || report.Calls != 4 || report.SourceGaps != 0 || report.Reserved.Calls != 6 || report.Reserved.ActiveNS > uint64(240*time.Second) || report.Authority != 0 || report.Accepted || report.SourceCompleteness != "UNKNOWN" || report.AbsenceInference != "PROHIBITED" {
				t.Fatalf("ASSERT_APPLICATION_HANDOFF_PUBLISHED_PARTIAL A-fail report=%+v err=%v", report, e)
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
			if e = a.GetRecord(cp.PageSHA256, &page); e != nil || len(page.Rows) != 1 || page.Rows[0].Requested != 20 || page.Rows[0].Reviewed != 20 {
				t.Fatalf("ASSERT_APPLICATION_HANDOFF_VERIFIED_NATIVE_TWENTY A-fail page=%+v err=%v", page, e)
			}
			var s applicationState
			if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil || s.DiscoveryFinished || s.Selected != "" || s.Inflight != "" {
				t.Fatalf("ASSERT_APPLICATION_HANDOFF_PUBLISHED_PARTIAL A-fail cursor=%+v err=%v", s, e)
			}
			roots, e := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*"))
			if e != nil || len(roots) != 1 {
				t.Fatal(roots, e)
			}
			raw, e := os.ReadFile(filepath.Join(roots[0], "source.json"))
			var source hostProductRequestV3
			if e != nil || json.Unmarshal(raw, &source) != nil || source.Configuration.Requested != 20 || source.FileDenominator != 1 {
				t.Fatal("native source metadata", e)
			}
			options, e := parseCensusCLIOptions(source.CensusArguments)
			if e != nil {
				t.Fatal(e)
			}
			files, e := enumerateCensusPreflightContextPrivate(context.Background(), options)
			if e != nil || len(files) != 1 || files[0].Path != selected {
				t.Fatal("original native file binding", files, e)
			}
			frozen, e := pilot.FreezeCensusScopePrivateV2(context.Background(), options.Workspace, r.Configuration.RevisionAssertion, files, source.Policy)
			if e != nil || pilot.Digest(frozen.ManifestBytes()) != source.Configuration.ScopeSHA256 {
				t.Fatal("original native freeze binding", e)
			}
			t.Logf("ASSERT_APPLICATION_HANDOFF_PUBLISHED_PARTIAL A-pass literal=%t native=%d reviewed=%d calls=%d active=%d", literal, page.Rows[0].Requested, page.Rows[0].Reviewed, report.Calls, report.Reserved.ActiveNS)
		})
	}
}

func TestHostApplicationHandoffGapStatus(t *testing.T) {
	// Zero native targets are genuine controlled source gaps, not invented units.
	// A one-attempt slice ends with no selected pointer but unfinished discovery.
	tool := buildApplicationTool(t, t.TempDir())
	for _, max := range []int{1, 16} {
		for _, literal := range []bool{false, true} {
			t.Run(fmt.Sprintf("max%d/literal%t", max, literal), func(t *testing.T) {
				r, a, _ := handoffCatalogFixture(t, literal, 1)
				a.Close()
				r.MaxUnitsPerRun = max
				r = handoffBind(r)
				path := filepath.Join(filepath.Dir(r.Configuration.Store), "gap-request.json")
				writeApplicationFixture(t, r, path)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path)
				cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1")
				var out, diag bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &diag
				e := cmd.Run()
				var terminal applicationTerminal
				want := "PAUSED_UNIT_SLICE"
				if max == 16 {
					want = "PROVISIONAL"
				}
				if e != nil || json.Unmarshal(out.Bytes(), &terminal) != nil || terminal.Status != want || terminal.SourceGaps != 1 || terminal.Units != 0 || terminal.Calls != 0 || terminal.Accepted || terminal.Authority != 0 || terminal.SourceCompleteness != "UNKNOWN" || terminal.AbsenceInference != "PROHIBITED" {
					t.Fatalf("ASSERT_APPLICATION_HANDOFF_GAP_STATUS A-fail max=%d err=%v terminal=%+v diag=%s", max, e, terminal, diag.String())
				}
				a, e = product.OpenApplication(r.Configuration, time.Now())
				if e != nil {
					t.Fatal(e)
				}
				defer a.Close()
				var s applicationState
				if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
					t.Fatal(e)
				}
				if s.Selected != "" || s.DiscoveryFinished != (max == 16) {
					t.Fatalf("ASSERT_APPLICATION_HANDOFF_GAP_STATUS A-fail state=%+v", s)
				}
				t.Logf("ASSERT_APPLICATION_HANDOFF_GAP_STATUS A-pass max=%d literal=%t finished=%t selected-empty=true gaps=%d", max, literal, s.DiscoveryFinished, s.SourceGaps)
			})
		}
	}
}

func TestHostApplicationHandoffDiscoverDrains(t *testing.T) {
	for _, literal := range []bool{false, true} {
		t.Run(fmt.Sprint(literal), func(t *testing.T) {
			r, a, s := handoffCatalogFixture(t, literal, 41)
			defer a.Close()
			app := product.ApplicationConfigDigest(r.Configuration)
			if e := advanceApplicationDiscoveryWithHandoff(context.Background(), r, a, &s, false); e != nil {
				t.Fatal(e)
			}
			if !s.DiscoveryFinished || s.Entries != 41 || s.ApplicationSHA256 != app || a.Head().ApplicationSHA256 != app {
				t.Fatal("ASSERT_APPLICATION_HANDOFF_DISCOVER A-fail")
			}
			t.Log("ASSERT_APPLICATION_HANDOFF_DISCOVER A-pass")
			// Fresh cursor: this also exercises the actual discover dispatch, not
			// merely the suppress-yield helper on an already-complete catalog.
			r2, a2, _ := handoffCatalogFixture(t, literal, 41)
			a2.Close()
			path := filepath.Join(filepath.Dir(r2.Configuration.Store), "discover-request.json")
			writeApplicationFixture(t, r2, path)
			var out, diag bytes.Buffer
			code := runPrivateHostApplication([]string{"discover", path}, bytes.NewReader(nil), &out, &diag)
			var terminal applicationTerminal
			if code != 0 || json.Unmarshal(out.Bytes(), &terminal) != nil || terminal.Status != "DISCOVERY_COMPLETE" || terminal.Discovered != 41 || terminal.Selected != 41 || terminal.Units != 0 || terminal.Calls != 0 || terminal.Denominator != "41" {
				t.Fatalf("ASSERT_APPLICATION_HANDOFF_DISCOVER_CLI A-fail code=%d terminal=%+v diag=%s", code, terminal, diag.String())
			}
			t.Log("ASSERT_APPLICATION_HANDOFF_DISCOVER_CLI A-pass")
		})
	}
}
