//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// Each negative fixture receives its own physical copies. Re-sealing updates
// claim -> report -> result -> host, so refusal cannot be an ancestor hash miss.
func applicationRepairClonePrivate(t *testing.T, original *censusApplicationProgressPrivate, r censusInventoryReportPrivate) (*censusApplicationProgressPrivate, censusInventoryReportPrivate, func(censusInventoryReportPrivate)) {
	t.Helper()
	s := cloneApplicationStatePrivate(t, original)
	pub := filepath.Join(t.TempDir(), "publication")
	attempt := filepath.Join(pub, "private-census-inventory-repair")
	copyApplicationFixturePrivate(t, r.AttemptDirectory, attempt, "")
	s.options.PublicationRoot = pub
	var claim censusApplicationClaimPrivate
	raw, e := os.ReadFile(filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "claim")))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &claim); e != nil {
		t.Fatal(e)
	}
	claim.AttemptDirectory = attempt
	rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "claim")), claim)
	raw, _ = json.Marshal(claim)
	claimSHA := pilot.Digest(raw)
	r.AttemptDirectory = attempt
	binding := *r.ApplicationBinding
	binding.ClaimSHA256 = claimSHA
	r.ApplicationBinding = &binding
	reseal := func(report censusInventoryReportPrivate) {
		rewriteApplicationFixturePrivate(t, filepath.Join(attempt, "report.json"), report)
		raw, _ := json.Marshal(report)
		reportSHA := pilot.Digest(raw)
		result := censusApplicationResultPrivate{"lsp-trace.census-application-result.private.v1", claimSHA, reportSHA, report.Status, ""}
		rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "result")), result)
		raw, _ = json.Marshal(result)
		host := censusApplicationHostObservationPrivate{Schema: "lsp-trace.census-application-host-process.private.v1", Trust: censusApplicationHostTrustPrivate, FullPlanSHA256: s.id, Segment: 0, ProcessID: claim.ProcessID, Joined: true, AttemptDirectory: attempt, ClaimSHA256: claimSHA, ResultSHA256: pilot.Digest(raw), ReportSHA256: reportSHA, ScopeSHA256: claim.ScopeSHA256, NativeSHA256: report.Outcome.EnvelopeSHA256, NativeBytes: report.Outcome.ByteLength}
		rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, censusApplicationLeafPrivate(0, "host")), host)
	}
	reseal(r)
	if _, e = s.verify(0); e != nil {
		t.Fatal("resealed control must reach acceptance", e)
	}
	return s, r, reseal
}

func TestCensusApplicationRepairTerminalPrivate(t *testing.T) {
	original, report, _, _ := applicationMockCompletedPrivate(t)
	for _, kind := range []string{"missing", "escape", "absolute", "wrong-prefix", "entries", "reorder-rows", "future-complete", "selected-origin", "metadata", "inline", "duplicate-page"} {
		t.Run(kind, func(t *testing.T) {
			s, r, reseal := applicationRepairClonePrivate(t, original, report)
			r.FilePages = append([]censusInventoryPagePrivate(nil), r.FilePages...)
			page := r.FilePages[0]
			raw, e := os.ReadFile(filepath.Join(r.AttemptDirectory, page.Selector))
			if e != nil {
				t.Fatal(e)
			}
			var rows []censusInventoryFilePrivate
			if e = json.Unmarshal(raw, &rows); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "missing":
				r.FilePages = nil
			case "escape", "absolute", "wrong-prefix":
				name := "other.json"
				path := filepath.Join(r.AttemptDirectory, name)
				if kind != "wrong-prefix" {
					dir := filepath.Join(filepath.Dir(r.AttemptDirectory), "outside")
					if e = os.Mkdir(dir, 0700); e != nil {
						t.Fatal(e)
					}
					path = filepath.Join(dir, name)
				}
				if e = os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
				if kind == "escape" {
					page.Selector = "../outside/other.json"
				} else if kind == "absolute" {
					page.Selector = path
				} else {
					page.Selector = name
				}
				r.FilePages[0] = page
			case "entries":
				r.FilePages[0].Entries++
			case "duplicate-page":
				r.FilePages = append(r.FilePages, page)
			case "inline":
				r.Files = append([]censusInventoryFilePrivate(nil), r.Files...)
				r.Files[0].Origin = 99
			default:
				switch kind {
				case "reorder-rows":
					rows[0], rows[1] = rows[1], rows[0]
				case "future-complete":
					rows[40].Disposition = "COMPUTED_PRIVATE_PER_BATCH"
				case "selected-origin":
					rows[0].Origin = 99
				case "metadata":
					rows[0].File.Path = "wrong.go"
				}
				raw, _ = json.Marshal(rows)
				if e = os.WriteFile(filepath.Join(r.AttemptDirectory, page.Selector), raw, 0600); e != nil {
					t.Fatal(e)
				}
				page.SHA256 = pilot.Digest(raw)
				page.ByteLength = len(raw)
				r.FilePages[0] = page
				r.Files = nil // Inline convenience must not mask terminal-content validation.
			}
			reseal(r)
			_, e = s.verify(0)
			reached := e == nil || strings.Contains(e.Error(), "terminal denominator")
			assertInventoryPrivate(t, "ASSERT_REPAIR_TERMINAL_"+kind, e != nil && reached)
			t.Logf("target=terminal denominator kind=%s observed=%v", kind, e)
		})
	}
}

// The claim is deliberately later than the absent prefix. Counts alone cannot
// distinguish the wrong attribution because both segments contain 16 files.
func TestCensusApplicationRepairBlockedIdentityPrivate(t *testing.T) {
	original, report, _, _ := applicationMockCompletedPrivate(t)
	for _, kind := range []string{"bound", "foreign-plan", "wrong-schema", "wrong-ordinal"} {
		t.Run(kind, func(t *testing.T) {
			s := cloneApplicationStatePrivate(t, original)
			s.blocked = nil
			for _, suffix := range []string{"claim", "result", "host"} {
				if e := os.Remove(filepath.Join(s.store.path, censusApplicationLeafPrivate(0, suffix))); e != nil {
					t.Fatal(e)
				}
			}
			groups := s.plan.groups(1)
			manifests := make([]json.RawMessage, len(groups))
			for j, origin := range s.plan.Segments[1] {
				manifests[j] = s.plan.Origins[origin].Manifest
			}
			scope := censusInventoryScopeBytesPrivate(s.options, s.plan.Configuration, s.pages, manifests, groups, &censusApplicationScopePrivate{s.id, 1})
			config, _ := json.Marshal(report.Configuration)
			claim := censusApplicationClaimPrivate{"lsp-trace.census-application-claim.private.v1", s.id, 1, os.Getpid(), report.AttemptDirectory, pilot.Digest(scope), pilot.Digest(config)}
			switch kind {
			case "foreign-plan":
				claim.FullPlanSHA256 = "foreign"
			case "wrong-schema":
				claim.Schema = "foreign"
			case "wrong-ordinal":
				claim.Segment = 0
			}
			rewriteApplicationFixturePrivate(t, filepath.Join(s.store.path, censusApplicationLeafPrivate(1, "claim")), claim)
			if s.scan() == nil {
				t.Fatal("fixture must stop on later claim")
			}
			attempt := filepath.Join(t.TempDir(), "accounting")
			if e := os.Mkdir(attempt, 0700); e != nil {
				t.Fatal(e)
			}
			s.store.ctx = context.Background()
			if e := s.accounting(attempt); e != nil {
				t.Fatal(e)
			}
			raw, e := os.ReadFile(filepath.Join(attempt, "application-accounting.json"))
			if e != nil {
				t.Fatal(e)
			}
			var view censusApplicationAccountingPrivate
			if e = json.Unmarshal(raw, &view); e != nil {
				t.Fatal(e)
			}
			expected := map[int]bool{}
			if kind == "bound" {
				for _, g := range groups {
					for _, i := range g {
						expected[i] = true
					}
				}
			}
			valid := view.CompletedFiles == 0 && view.ExcludedFiles == 1 && view.GapFiles == len(expected) && view.UnattemptedFiles == 71-len(expected) && s.next == 0
			for i, row := range view.Files {
				if expected[i] {
					valid = valid && row.Disposition == "GAP_SPENT_OR_UNVERIFIED_SEGMENT"
				} else if row.Disposition != "EXCLUDED" {
					valid = valid && row.Disposition == "UNATTEMPTED"
				}
			}
			assertInventoryPrivate(t, "ASSERT_REPAIR_BLOCKED_IDENTITY_"+kind, valid)
		})
	}
}
