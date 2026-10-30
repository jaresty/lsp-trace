//go:build darwin || linux

package main

import (
	"encoding/json"
	"fmt"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func inventoryTwoFixturePrivate(t *testing.T) (string, string, string, []string) {
	t.Helper()
	base, src, path, args := inventoryFixturePrivate(t)
	for i := 0; i < 6; i++ {
		if err := os.WriteFile(filepath.Join(src, fmt.Sprintf("extra%d.go", i)), []byte("package owned\n\nfunc A0() {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := inventoryConfigPrivate()
	c.MaxOrigins = 2
	c.Policy.MaxNominationFiles = 8
	c.Policy.Discovery.MaxFiles = 8
	c.Policy.MaxMembers = 9
	saveInventoryConfigPrivate(t, path, c)
	return base, src, path, args
}

func inventoryTwoDepsPrivate(t *testing.T) (censusRunnerDependencies, *atomic.Int32, *[]*sessionruntime.Manager) {
	t.Helper()
	d := productionCensusRunnerDependencies()
	starts := &atomic.Int32{}
	managers := []*sessionruntime.Manager{}
	d.newStarter = func() (sessionruntime.Starter, error) {
		p, _ := inventoryDepsPrivate(t)
		s, e := p.newStarter()
		if e == nil {
			v := s.(inventoryStarterPrivate)
			v.starts = starts
			s = v
		}
		return s, e
	}
	original := d.newManager
	d.newManager = func(c sessionruntime.Config) (*sessionruntime.Manager, error) {
		m, e := original(c)
		if e == nil {
			managers = append(managers, m)
		}
		return m, e
	}
	d.lookPath = func(string) (string, error) { return "/owned/server", nil }
	return d, starts, &managers
}

func TestCensusPrivateInventoryTwoOriginalOwners(t *testing.T) {
	_, _, _, args := inventoryTwoFixturePrivate(t)
	deps, starts, managers := inventoryTwoDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	seen := map[int]bool{}
	accounted := len(r.Origins) == 2 && r.FileDenominator == 10
	for ordinal, o := range r.Origins {
		for _, i := range o.Files {
			if seen[i] || r.Files[i].Origin != ordinal {
				accounted = false
			}
			seen[i] = true
		}
	}
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_DENOMINATOR", accounted && len(seen) == 9)
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_ORIGINAL_MANAGERS", len(*managers) == 2 && (*managers)[0] != (*managers)[1] && starts.Load() == 2)
	type lifecycle struct {
		Session                         censusacquisition.SessionIdentity
		StopCalls, ShutdownCalls        int
		ShutdownJoined, CleanupVerified bool
	}
	raw, _ := json.Marshal(r.Origins)
	var lives []lifecycle
	_ = json.Unmarshal(raw, &lives)
	t.Logf("observed origin lifecycle: %+v", lives)
	valid := len(lives) == 2
	for _, o := range lives {
		valid = valid && o.Session.SessionID != "" && o.Session.Generation > 0 && o.StopCalls == 1 && o.ShutdownCalls == 1 && o.ShutdownJoined && o.CleanupVerified
	}
	originIDs := map[string]bool{}
	for _, coverage := range r.Coverage {
		if coverage.PrivateV2 != nil {
			originIDs[coverage.PrivateV2.OriginID] = true
		}
	}
	valid = valid && len(originIDs) == 2
	t.Logf("sealed origin namespaces: %v (session keys are stable, not owner IDs)", originIDs)
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_LIFECYCLE", valid)
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_DETACHED_PAIR", code == 0 && len(r.Index) == 8 && r.Outcome != nil && r.Outcome.Status == "VERIFIED_PAIR_PRIVATE_WITH_LIMITS")
}

func TestCensusPrivateInventoryTwoOriginConfig(t *testing.T) {
	_, _, config, _ := inventoryFixturePrivate(t)
	c := inventoryConfigPrivate()
	c.MaxOrigins = 2
	saveInventoryConfigPrivate(t, config, c)
	_, err := loadCensusInventoryConfigPrivate(config)
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_CONFIG_ADMITTED", err == nil)
	c.MaxOrigins = 3
	saveInventoryConfigPrivate(t, config, c)
	_, err = loadCensusInventoryConfigPrivate(config)
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_CONFIG_FINITE", err != nil)
}
