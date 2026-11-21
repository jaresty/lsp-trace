//go:build darwin

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

// No source traversal, public start, process, native fixture or real provider.
// The only admitted effect is a constructor spy which immediately refuses.
func TestCensusFolderSelectedScopeAdmission(t *testing.T) {
	for _, tc := range []struct {
		name       string
		selected   int
		invalid    bool
		wantStarts int
	}{
		{"one-of-18", 1, false, 1}, {"boundary-16", 16, false, 1},
		{"boundary-17", 17, false, 0}, {"empty", 0, false, 0}, {"invalid", 1, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			store, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(store, 0700); err != nil {
				t.Fatal(err)
			}
			records := []source.Record{{Name: "src", Kind: source.SourceDirectory, Inclusion: source.Included}}
			for n := 0; n < 18; n++ {
				prefix := "drop"
				if n < tc.selected {
					prefix = "keep"
				}
				records = append(records, source.Record{Name: fmt.Sprintf("src/%s-%02d.go", prefix, n), Kind: source.SourceRegularFile, Inclusion: source.Included})
			}
			original := append([]source.Record(nil), records...)
			o := censusCLIOptions{Workspace: workspace, PublicationRoot: store, Includes: []string{"src/keep-*.go"}, Excludes: []string{"**/*_test.go"}, Timeout: 10 * time.Second, RequestTimeout: 10 * time.Second, MaxNodes: 100}
			if tc.invalid {
				o.Includes = []string{"["}
			}
			selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "public-acquisition", Workspace: workspace, Profile: "cli", EnvironmentReference: "cli"})
			if err != nil {
				t.Fatal(err)
			}
			starts := 0
			cfg := censusCoreConfig{folderPreparation: &censusFolderPreparation{workspace: workspace, runRoot: store, records: records}, folderStarter: func() (sessionruntime.Starter, error) {
				starts++
				return nil, errors.New("controlled constructor refusal; no provider")
			}}
			cfg.runner.start = sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go"}
			var diagnostics bytes.Buffer
			if code := runCensusFolderAcquisition(o, cfg, &diagnostics); code != 1 {
				t.Fatalf("expected controlled refusal, got %d", code)
			}
			if starts != tc.wantStarts {
				t.Fatalf("selected=%d enumerated=18: constructor calls=%d want=%d; %s", tc.selected, starts, tc.wantStarts, diagnostics.String())
			}
			if !reflect.DeepEqual(records, original) {
				t.Fatal("paid preparation records mutated")
			}
			if tc.wantStarts == 0 {
				if _, err := os.Stat(filepath.Join(store, "folder-acquisition")); !os.IsNotExist(err) {
					t.Fatalf("denied scope created acquisition state: %v", err)
				}
			}
		})
	}
}
