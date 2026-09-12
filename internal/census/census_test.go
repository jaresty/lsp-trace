package census

import (
	"encoding/json"
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

func TestValidatePreStartInputs(t *testing.T) {
	root := privateRoot(t)
	valid := Config{WorkspaceRoot: t.TempDir(), PublicationRoot: root, Format: OutputHuman, Includes: []string{"src/**"}, Excludes: []string{"vendor/"}}
	closer, err := Validate(valid)
	if err != nil {
		t.Fatalf("ASSERT_CENSUS_VALID_CONFIG: %v", err)
	}
	if closer == nil {
		t.Fatal("ASSERT_CENSUS_PINNED_ROOT: nil closer")
	}
	closer.Close()

	public := filepath.Join(t.TempDir(), "public")
	if err := os.Mkdir(public, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"required-root", func(c *Config) { c.PublicationRoot = "" }},
		{"absolute-root", func(c *Config) { c.PublicationRoot = "relative" }},
		{"clean-root", func(c *Config) {
			c.PublicationRoot = root + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(root)
		}},
		{"private-root", func(c *Config) { c.PublicationRoot = public }},
		{"include", func(c *Config) { c.Includes = []string{"!secret/**"} }},
		{"exclude", func(c *Config) { c.Excludes = []string{"src/["} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.mutate(&c)
			if closer, err := Validate(c); err == nil {
				if closer != nil {
					closer.Close()
				}
				t.Fatal("ASSERT_CENSUS_PRESTART_REJECTION: accepted")
			}
		})
	}
}

func TestBatchTargetsCanonicalMaximalBoundaries(t *testing.T) {
	for _, n := range []int{1, 63, 64, 126, 127} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			targets := make([]captureset.Target, n)
			for i := range targets {
				targets[i] = captureset.Target{CensusOrdinal: n - i - 1, CanonicalSeedV2: fmt.Sprintf("seed-%04d", n-i-1)}
			}
			batches := BatchTargets(targets)
			wantBatches := (n + captureset.MaxBatchTargets - 1) / captureset.MaxBatchTargets
			if len(batches) != wantBatches {
				t.Fatalf("ASSERT_CENSUS_MAXIMAL_BATCH_COUNT: n=%d got=%d want=%d", n, len(batches), wantBatches)
			}
			var got []captureset.Target
			for i, batch := range batches {
				if batch.Ordinal != i || len(batch.Targets) == 0 || len(batch.Targets) > captureset.MaxBatchTargets {
					t.Fatalf("ASSERT_CENSUS_BATCH_BOUND: n=%d batch=%+v", n, batch)
				}
				if i < len(batches)-1 && len(batch.Targets) != captureset.MaxBatchTargets {
					t.Fatalf("ASSERT_CENSUS_MAXIMAL_BATCH: n=%d size=%d", n, len(batch.Targets))
				}
				got = append(got, batch.Targets...)
			}
			for i, target := range got {
				if target.CanonicalSeedV2 != fmt.Sprintf("seed-%04d", i) {
					t.Fatalf("ASSERT_CENSUS_CANONICAL_ORDER: n=%d index=%d got=%q", n, i, target.CanonicalSeedV2)
				}
			}
			if !reflect.DeepEqual(targets[0].CanonicalSeedV2, fmt.Sprintf("seed-%04d", n-1)) {
				t.Fatal("ASSERT_CENSUS_INPUT_IMMUTABLE")
			}
		})
	}
}

func TestOutputProjectionsArePrivacySafe(t *testing.T) {
	result := Result{Status: "READY", Accounting: Accounting{Targets: 64, Batches: 2}}
	machine, err := json.Marshal(ProjectMachine(result))
	if err != nil {
		t.Fatal(err)
	}
	human := fmt.Sprintf("%+v", ProjectHuman(result))
	for _, secret := range []string{"/private/root", "seed-private", "artifact.json"} {
		if strings.Contains(string(machine), secret) || strings.Contains(human, secret) {
			t.Fatalf("ASSERT_CENSUS_PROJECTION_PRIVATE: leaked %q", secret)
		}
	}
	if !strings.Contains(string(machine), `"schema_version":"`+ResultVersion+`"`) {
		t.Fatalf("ASSERT_CENSUS_MACHINE_VERSION: %s", machine)
	}
}
