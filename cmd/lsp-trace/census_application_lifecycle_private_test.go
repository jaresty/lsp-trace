//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"

	prep "lsp-trace/internal/privatedescribeprep"
)

func TestCensusApplicationVerifiedPrefixAccountingPrivate(t *testing.T) {
	s, _, _, _ := applicationMockCompletedPrivate(t)
	attempt := filepath.Join(t.TempDir(), "accounting")
	if e := os.Mkdir(attempt, 0700); e != nil {
		t.Fatal(e)
	}
	if e := s.accounting(attempt); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(attempt, "application-accounting.json"))
	if e != nil {
		t.Fatal(e)
	}
	var view censusApplicationAccountingPrivate
	if json.Unmarshal(raw, &view) != nil {
		t.Fatal("accounting parse")
	}
	assertInventoryPrivate(t, "ASSERT_APPLICATION_VERIFIED_PREFIX_PARTITIONS", view.FileDenominator == 72 && view.CompletedFiles == 16 && view.ExcludedFiles == 1 && view.GapFiles == 0 && view.UnattemptedFiles == 55 && len(view.Verified) == 1)
}
func TestCensusApplicationLaterLifecyclePrivate(t *testing.T) {
	for _, kind := range []string{"retention", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			s, first, config, args := applicationMockCompletedPrivate(t, 5000)
			deps, _, _ := inventoryTwoDepsPrivate(t)
			prepared, code := runInventoryCLIPrivate(t, args, deps)
			if code != 0 || prepared.ApplicationBinding.Segment != 1 {
				t.Fatal("later preparation", prepared.Failure)
			}
			c, e := loadCensusInventoryConfigPrivate(config)
			if e != nil {
				t.Fatal(e)
			}
			a := *first.Configuration.Automation
			a.Directory = filepath.Join(filepath.Dir(config), "late-mailbox")
			a.Invocation = "late-independent-fixture"
			a.ScopeSHA256 = prepared.ScopeSHA256
			if e = os.Mkdir(a.Directory, 0700); e != nil {
				t.Fatal(e)
			}
			a.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(a)
			c.Automation = &a
			saveInventoryConfigPrivate(t, config, c)
			deps, _, _ = inventoryTwoDepsPrivate(t)
			var deadlines []time.Time
			deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
				d := productionCensusInventoryDependenciesPrivate()
				d.automatic = func(prep.AutomaticHandoffConfigPrivateV2, string, prep.PostPreparationPrivateV2) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
					return inventorySemanticFixturePrivate{"owned-interpreter"}, inventorySemanticFixturePrivate{"owned-reviewer"}, nil
				}
				if kind == "retention" {
					enumerate := d.enumerate
					injected := false
					d.enumerate = func(ctx context.Context, o censusCLIOptions) ([]censusacquisition.SourceFile, error) {
						if !injected {
							entries, e := os.ReadDir(o.PublicationRoot)
							if e != nil {
								return nil, e
							}
							for _, entry := range entries {
								if entry.IsDir() && strings.HasPrefix(entry.Name(), "private-census-inventory-") {
									attempt := filepath.Join(o.PublicationRoot, entry.Name())
									if _, e := os.Stat(filepath.Join(attempt, "report.json")); os.IsNotExist(e) {
										if e = os.Mkdir(filepath.Join(attempt, "inventory-0"), 0700); e != nil {
											return nil, e
										}
										injected = true
										break
									}
								}
							}
						}
						return enumerate(ctx, o)
					}
				}
				original := d.runSession
				d.runSession = func(rc initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
					deadline, _ := rc.parent.Deadline()
					deadlines = append(deadlines, deadline)
					if kind == "deadline" && len(deadlines) == 2 {
						<-rc.parent.Done()
						return 1
					}
					return original(rc, cb)
				}
				return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, d)
			}
			started := time.Now()
			r, code := runInventoryCLIPrivate(t, args, deps)
			assertInventoryPrivate(t, "ASSERT_APPLICATION_LATER_"+kind, code == 1 && r.ApplicationBinding.Segment == 1 && r.Origins[0].CleanupVerified && r.Origins[1].ShutdownJoined && r.Outcome == nil && len(deadlines) == 2 && deadlines[0].Equal(deadlines[1]) && time.Since(started) < 7*time.Second)
			c.Automation = nil
			saveInventoryConfigPrivate(t, config, c)
			deps, starts, _ := inventoryTwoDepsPrivate(t)
			stopped, code := runInventoryCLIPrivate(t, args, deps)
			assertInventoryPrivate(t, "ASSERT_APPLICATION_LATER_SPENT_"+kind, code == 1 && starts.Load() == 0 && stopped.ApplicationBinding.Segment == 1 && s.next == 1)
		})
	}
}
