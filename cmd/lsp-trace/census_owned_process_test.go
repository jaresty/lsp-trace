//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Server-side test substrate only. The caller test imports no internal package
// and never sees a construction, owner, selected object or admission conversion.
func TestCensusOwnedProcess(t *testing.T) {
	mark := -1
	for i, a := range os.Args {
		if a == "census-owned-process-v1" {
			mark = i
			break
		}
	}
	if mark < 0 {
		t.Skip("explicit controlled executable only")
	}
	source, args := os.Args[mark+1], os.Args[mark+2:]
	if len(args) < 2 || args[0] != "lsp-trace" || args[1] != "census" {
		t.Fatal("public argv required")
	}
	args = args[2:]
	code := func() int {
		deps := productionCensusRunnerDependencies()
		deps.lookPath = func(string) (string, error) {
			return "", errors.New("OS provider resolution forbidden in controlled command test")
		}
		deps.newStarter = func() (sessionruntime.Starter, error) {
			return nil, errors.New("OS provider startup forbidden in controlled command test")
		}
		var fixture *sessionruntime.ControlledInventoryPipeFixture
		// Each process creates the same constructor-owned literal source fixture.
		// Teardown removes it; no caller action recreates/renames/translates evidence.
		needsFixture := len(args) > 0 && (args[0] != "durable" || len(args) > 1 && args[1] == "acquire")
		if needsFixture {
			var e error
			fixture, e = sessionruntime.NewControlledInventoryPipeFixture(source)
			if e != nil {
				t.Fatal(e)
			}
			defer fixture.Close(context.Background())
			if e = os.WriteFile(filepath.Join(source, "go.mod"), []byte("module controlled\n\ngo 1.24\n"), 0600); e != nil {
				t.Fatal(e)
			}
			defer os.Remove(filepath.Join(source, "go.mod"))
			if strings.HasSuffix(source, "-extra") {
				if e = os.WriteFile(filepath.Join(source, "extra.go"), []byte("package fixture\n"), 0600); e != nil {
					t.Fatal(e)
				}
				defer os.Remove(filepath.Join(source, "extra.go"))
			}
		}
		deps.runOwnedOperation = func(a []string, in io.Reader, out, diag io.Writer) int {
			return runCensusOwnerOperationWithFactory(a, in, out, diag, func(batch ai.InventoryNativeBatchV1, end time.Time) (sessionruntime.PrivateDiscoveryConstruction, error) {
				if fixture == nil {
					t.Fatal("fixture setup missing")
				}
				c, e := fixture.Construction(batch.Ordinal, end)
				if e == nil {
					_ = json.NewEncoder(os.Stderr).Encode(struct {
						Marker                string
						Ordinal, Members, PID int
					}{"CONTROLLED_CONSTRUCTOR", batch.Ordinal, len(c.Members), os.Getpid()})
				}
				return c, e
			})
		}
		return runCensusWithDependencies(args, os.Stdout, os.Stderr, deps)
	}()
	os.Exit(code)
}
