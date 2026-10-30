//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

func TestCensusPrivateInventoryUnsupportedRepair(t *testing.T) {
	_, src, config, args := inventoryFixturePrivate(t)
	for _, name := range []string{"README.md", "000.unknown"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte("owned unsupported\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// One unknown path sorts before every Go path, and README before a.go.
	c := inventoryConfigPrivate()
	c.Policy.MaxNominationFiles = 8
	c.Policy.Discovery.MaxFiles = 8
	c.Policy.MaxMembers = 9
	saveInventoryConfigPrivate(t, config, c)
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(src, fmt.Sprintf("extra%d.go", i)), []byte("package owned\n\nfunc A0() {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	deps, starts := inventoryDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	unsupported, supported := 0, 0
	for _, row := range r.Files {
		if row.File.LanguageID == "" && row.Disposition == "UNSUPPORTED_LANGUAGE" && row.Origin == -1 {
			unsupported++
		}
		if row.File.LanguageID == "go" && row.Origin == 0 {
			supported++
		}
	}
	assertInventoryPrivate(t, "ASSERT_REPAIR_UNSUPPORTED_ISOLATION", code == 0 && r.FileDenominator == 11 && unsupported == 2 && supported == 8 && starts.Load() == 1 && len(r.Origins) == 1 && r.Origins[0].Status == "COMPUTED_PRIVATE_PER_BATCH")
}

func TestCensusPrivateInventoryCancellationRepair(t *testing.T) {
	_, _, config, args := inventoryFixturePrivate(t)
	c := inventoryConfigPrivate()
	c.OperationTimeoutMS = 40
	c.Policy.TimeoutMS = 40
	c.Policy.RequestTimeoutMS = 20
	saveInventoryConfigPrivate(t, config, c)
	for i, a := range args {
		if a == "30s" {
			args[i] = "40ms"
		}
		if a == "5s" {
			args[i] = "20ms"
		}
	}
	deps, starts := inventoryDepsPrivate(t)
	cancelled := false
	deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
		d := productionCensusInventoryDependenciesPrivate()
		d.enumerate = func(ctx context.Context, _ censusCLIOptions) ([]censusacquisition.SourceFile, error) {
			select {
			case <-ctx.Done():
				cancelled = true
				return nil, ctx.Err()
			case <-time.After(400 * time.Millisecond):
				return nil, errors.New("owned enumerator safety bound")
			}
		}
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, errout, d)
	}
	start := time.Now()
	r, code := runInventoryCLIPrivate(t, args, deps)
	assertInventoryPrivate(t, "ASSERT_REPAIR_OPERATION_ENUMERATION_CANCEL", cancelled && time.Since(start) < 300*time.Millisecond && code == 1 && r.Failure == "ENUMERATION_FAILED" && r.Status == "RETAINED_FAILURE" && starts.Load() == 0)
}

func TestCensusPrivateInventoryProductionEnumerationCancellationRepair(t *testing.T) {
	_, src, _, _ := inventoryFixturePrivate(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := productionCensusInventoryDependenciesPrivate().enumerate(ctx, censusCLIOptions{Workspace: src, Sources: []string{"."}})
	assertInventoryPrivate(t, "ASSERT_REPAIR_REAL_ENUMERATOR_CANCEL", errors.Is(err, context.Canceled))
}

func TestCensusPrivateInventoryMailboxAliasRepair(t *testing.T) {
	base, src, config, args := inventoryFixturePrivate(t)
	alias := filepath.Join(base, "SOURCE")
	a, e := os.Stat(alias)
	b, e2 := os.Stat(src)
	if e != nil || e2 != nil || !os.SameFile(a, b) {
		t.Skip("case-sensitive filesystem: actual mailbox alias assertion not exercised")
	}
	if e := os.Mkdir(filepath.Join(src, "mailbox"), 0700); e != nil {
		t.Fatal(e)
	}
	c := inventoryConfigPrivate()
	c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: filepath.Join(alias, "mailbox"), Invocation: "owned-alias", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: pilot.Digest([]byte("owned")), TimeoutMS: 3000, OverallTimeoutMS: 15000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCalls: 8, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}}
	c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	saveInventoryConfigPrivate(t, config, c)
	before := inventoryTreeNamesRepair(t, base)
	deps, starts := inventoryDepsPrivate(t)
	var out, errout bytes.Buffer
	code := runCensusWithDependencies(args, &out, &errout, deps)
	after := inventoryTreeNamesRepair(t, base)
	assertInventoryPrivate(t, "ASSERT_REPAIR_MAILBOX_ALIAS_NO_WRITES", code != 0 && starts.Load() == 0 && out.Len() == 0 && reflect.DeepEqual(before, after) && strings.Contains(errout.String(), `"code":"INVALID_CONFIG"`))
}

func TestCensusPrivateInventoryClassificationPrecedenceRepair(t *testing.T) {
	files := []censusacquisition.SourceFile{{Path: "excluded.md"}, {Path: "unavailable.md", Disposition: "unreadable"}, {Path: "README.md"}, {Path: "a.go", LanguageID: "go"}}
	rows, groups := censusInventoryPlanPrivate(censusCLIOptions{Excludes: []string{"excluded.md"}}, inventoryConfigPrivate(), files)
	got := map[string]string{}
	for _, r := range rows {
		got[r.File.Path] = r.Disposition
	}
	assertInventoryPrivate(t, "ASSERT_REPAIR_CLASSIFICATION_PRECEDENCE", got["excluded.md"] == "EXCLUDED" && got["unavailable.md"] == "unreadable" && got["README.md"] == "UNSUPPORTED_LANGUAGE" && len(groups) == 1 && len(groups[0]) == 1 && rows[groups[0][0]].File.Path == "a.go")
}

func inventoryTreeNamesRepair(t *testing.T, root string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		n, _ := filepath.Rel(root, p)
		names = append(names, n)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}
func TestCensusPrivateInventoryCaseAliasRepair(t *testing.T) {
	for _, kind := range []string{"equal", "publication-below", "publication-above"} {
		t.Run(kind, func(t *testing.T) {
			base, src, _, args := inventoryFixturePrivate(t)
			alias := filepath.Join(base, "SOURCE")
			a, e := os.Stat(alias)
			b, e2 := os.Stat(src)
			if e != nil || e2 != nil || !os.SameFile(a, b) {
				t.Skip("case-sensitive filesystem: actual case-alias assertion not exercised")
			}
			pub := alias
			if kind == "publication-below" {
				if e := os.Mkdir(filepath.Join(src, "output"), 0700); e != nil {
					t.Fatal(e)
				}
				pub = filepath.Join(alias, "output")
			}
			if kind == "publication-above" {
				if e := os.Mkdir(filepath.Join(src, "child"), 0700); e != nil {
					t.Fatal(e)
				}
				src = filepath.Join(alias, "child")
				pub = filepath.Join(base, "source")
			}
			for i, a := range args {
				if a == "--workspace" {
					args[i+1] = src
				}
				if a == "--publication-root" {
					args[i+1] = pub
				}
			}
			before := inventoryTreeNamesRepair(t, filepath.Join(base, "source"))
			deps, starts := inventoryDepsPrivate(t)
			var out, errout bytes.Buffer
			code := runCensusWithDependencies(args, &out, &errout, deps)
			after := inventoryTreeNamesRepair(t, filepath.Join(base, "source"))
			assertInventoryPrivate(t, "ASSERT_REPAIR_CASE_ALIAS_NO_WRITES", code != 0 && starts.Load() == 0 && reflect.DeepEqual(before, after) && strings.Contains(errout.String(), `"code":"INVALID_CONFIG"`) && out.Len() == 0)
		})
	}
}
