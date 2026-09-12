package census

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
)

func privateRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestConfigDefaultsAndDeferredSpellingValidation(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace-does-not-need-to-exist")
	cfg := DefaultConfig(workspace)
	cfg.PublicationRoot = privateRoot(t)
	if !reflect.DeepEqual(cfg.SourceRoots, []string{"."}) || cfg.DownDepth != 1 || cfg.UpDepth != 0 {
		t.Fatalf("defaults: %+v", cfg)
	}
	if cfg.Limits != (Limits{MaxNodes: 10000, Timeout: DefaultTimeout, RequestTimeout: DefaultRequestTimeout}) {
		t.Fatalf("limits: %+v", cfg.Limits)
	}
	root, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("lexically valid nonexistent workspace should defer resolution: %v", err)
	}
	root.Close()

	cases := []func(*Config){
		func(c *Config) { c.Workspace += string(os.PathSeparator) + "." },
		func(c *Config) { c.SourceRoots = []string{""} },
		func(c *Config) { c.SourceRoots = []string{"../escape"} },
		func(c *Config) { c.SourceRoots = []string{"src/../src"} },
		func(c *Config) { c.Includes = []string{"!secret/**"} },
		func(c *Config) { c.DownDepth = -1 },
		func(c *Config) { c.Limits.RequestTimeout = 0 },
		func(c *Config) { c.Output = "xml" },
	}
	for i, mutate := range cases {
		c := cfg
		mutate(&c)
		if root, err := ValidateConfig(c); err == nil {
			root.Close()
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestAccountingRequiresClosedTerminalDispositions(t *testing.T) {
	valid := Accounting{
		FileDenominator: 8,
		Files: []FileEntry{
			{0, FileSelected}, {1, FileExcluded}, {2, FileForbidden}, {3, FileUnreadable},
			{4, FileUnsupported}, {5, FileDocumentSymbolFailed}, {6, FileOmitted}, {7, FileIncomplete},
		},
		SymbolDenominator: 6,
		Symbols: []SymbolEntry{
			{0, SymbolSelected}, {1, SymbolUnsupported}, {2, SymbolPreparationFailed},
			{3, SymbolNonCallable}, {4, SymbolOmitted}, {5, SymbolIncomplete},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid accounting: %v", err)
	}
	cases := []Accounting{valid, valid, valid, valid}
	cases[0].Files = append([]FileEntry(nil), valid.Files[:7]...)
	cases[1].Files = append([]FileEntry(nil), valid.Files...)
	cases[1].Files[7].Ordinal = 6
	cases[2].Symbols = append([]SymbolEntry(nil), valid.Symbols...)
	cases[2].Symbols[5].Disposition = "prepared"
	cases[3].SymbolDenominator = 5
	for i, accounting := range cases {
		if err := accounting.Validate(); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func target(ordinal int, seed string) captureset.Target {
	sum := sha256.Sum256([]byte(seed))
	return captureset.Target{CensusOrdinal: ordinal, CanonicalSeedV2: seed, CanonicalSeedV2SHA256: "sha256:" + hex.EncodeToString(sum[:])}
}

func TestPlanTargetsValidationAndDeterministicMaximalBatches(t *testing.T) {
	for _, n := range []int{1, 63, 64, 126, 127} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			targets := make([]captureset.Target, n)
			for i := range targets {
				targets[i] = target(n-i-1, fmt.Sprintf("seed-%04d", n-i-1))
			}
			plan, err := PlanTargets(targets)
			if err != nil {
				t.Fatal(err)
			}
			if plan.PolicyIdentity != BatchPolicyIdentity || len(plan.Batches) != (n+62)/63 {
				t.Fatalf("plan identity/count: %+v", plan)
			}
			var got []captureset.Target
			for i, batch := range plan.Batches {
				if batch.Ordinal != i || len(batch.Targets) == 0 || len(batch.Targets) > 63 || (i < len(plan.Batches)-1 && len(batch.Targets) != 63) {
					t.Fatalf("non-maximal batch: %+v", batch)
				}
				got = append(got, batch.Targets...)
			}
			for i := range got {
				if got[i].CanonicalSeedV2 != fmt.Sprintf("seed-%04d", i) {
					t.Fatalf("order[%d]=%q", i, got[i].CanonicalSeedV2)
				}
			}
			if targets[0].CanonicalSeedV2 != fmt.Sprintf("seed-%04d", n-1) {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestPlanTargetsRejectsDuplicateCensusOrdinal(t *testing.T) {
	targets := []captureset.Target{target(0, "seed"), target(0, "other")}
	if _, err := PlanTargets(targets); err == nil || !strings.Contains(err.Error(), "duplicate census ordinal") {
		t.Fatalf("duplicate census ordinal accepted: %v", err)
	}
}

func TestPlanTargetsRejectsInvalidSets(t *testing.T) {
	valid := target(0, "seed")
	duplicateIdentity := []captureset.Target{valid, target(1, "seed")}
	duplicateDigest := []captureset.Target{valid, target(1, "other")}
	duplicateDigest[1].CanonicalSeedV2SHA256 = valid.CanonicalSeedV2SHA256
	malformed := valid
	malformed.CanonicalSeedV2SHA256 = "sha256:bad"
	over := make([]captureset.Target, captureset.MaxTargets+1)
	for i := range over {
		over[i] = target(i, fmt.Sprintf("seed-%d", i))
	}
	for i, targets := range [][]captureset.Target{nil, over, []captureset.Target{malformed}, duplicateIdentity, duplicateDigest} {
		if _, err := PlanTargets(targets); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}
