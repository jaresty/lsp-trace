//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

func TestVariableCallerV2ExecuteOnceRequiresValidatedConfig(t *testing.T) {
	c, path, git := variableOwnedConfig(t, 1, 0)
	c.Purpose = "WRONG_PURPOSE"
	variableSave(t, path, c)
	calls := 0
	_, e := variableOwnedRun(context.Background(), "execute-once", path, git, func() (*sessionruntime.Manager, error) { calls++; return nil, nil }, variableClose)
	if e == nil || !strings.Contains(e.Error(), "purpose") || calls != 0 {
		t.Fatal("explicit execute-once must reach strict config validation before any manager", e, calls)
	}
	variableEmpty(t, c)
}

func TestVariableCallerV2CanonicalPlaceholder(t *testing.T) {
	b, e := os.ReadFile("private_variable_scope_v2.example.json")
	if e != nil {
		t.Fatal(e)
	}
	var c variableCallerConfig
	if e = variableJSON(b, &c); e != nil {
		t.Fatal("canonical example shape", e)
	}
	round, e := json.Marshal(c)
	if e != nil || !bytes.Equal(round, b) {
		t.Fatal("exact canonical example")
	}
	if e = variableValidate(c); e == nil {
		t.Fatal("placeholder became execution config")
	}
}
func TestVariableCallerV2ClaimedCancellation(t *testing.T) {
	c, path, git := variableOwnedConfig(t, 1, 0)
	factory, starts, _, _ := variableOwnedFactory(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	wrapped := func() (*sessionruntime.Manager, error) { calls++; cancel(); return factory() }
	report, e := variableOwnedRun(ctx, "execute-once", path, git, wrapped, variableClose)
	if e == nil || calls != 1 || starts.Load() != 0 || report.Phase != "FAILED_SPENT" {
		t.Fatal("claimed cancellation", e, calls, starts.Load(), report.Phase)
	}
	if _, e = os.Stat(c.AttemptMarkerPath); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(c.ReportPath)
	if e != nil || !bytes.Contains(b, []byte("FAILED_SPENT")) {
		t.Fatal("canceled final report", e)
	}
	if _, e := variableOwnedRun(context.Background(), "execute-once", path, git, wrapped, variableClose); e == nil || calls != 1 {
		t.Fatal("canceled claim replay")
	}
}
func TestVariableCallerV2LateReportExists(t *testing.T) {
	c, path, git := variableOwnedConfig(t, 1, 0)
	factory, starts, _, _ := variableOwnedFactory(t, "")
	sentinel := []byte("owned foreign report: preserve, never overwrite")
	wrapped := func() (*sessionruntime.Manager, error) {
		if e := os.WriteFile(c.ReportPath, sentinel, 0600); e != nil {
			t.Fatal(e)
		}
		return factory()
	}
	report, e := variableOwnedRun(context.Background(), "execute-once", path, git, wrapped, variableClose)
	if e == nil || !errors.Is(e, os.ErrExist) || starts.Load() != 1 || report.Outcome == nil {
		t.Fatal("late report existence", e, starts.Load())
	}
	actual, e := os.ReadFile(c.ReportPath)
	if e != nil || !bytes.Equal(actual, sentinel) {
		t.Fatal("existing report overwritten")
	}
	if _, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose); e == nil || starts.Load() != 1 {
		t.Fatal("report failure replay")
	}
}
func TestVariableCallerV2Source0755TemporaryRoot(t *testing.T) {
	c, path, git := variableOwnedConfig(t, 1, 0)
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(root, 0755); e != nil {
		t.Fatal(e)
	}
	body, e := os.ReadFile(filepath.Join(c.Scope.Root, c.Scope.Members[0].CanonicalRelativePath))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, c.Scope.Members[0].CanonicalRelativePath), body, 0600); e != nil {
		t.Fatal(e)
	}
	c.Scope.Root = root
	c.Selector.Workspace = root
	v, e := runtimeprofile.Validate(c.Selector)
	if e != nil {
		t.Fatal(e)
	}
	c.Session.SessionID = runtimeprofile.Resolve(v).SessionKey().String()
	variableSave(t, path, c)
	if _, e = variableOwnedRun(context.Background(), "preflight", path, git, nil, nil); e != nil {
		t.Fatal("source protection must not impose output-root rules", e)
	}
	variableEmpty(t, c)
}

func TestVariableCallerV2RealCodeRootBinding(t *testing.T) {
	if _, e := variableGit(context.Background(), "/ABSOLUTE/NOT/THE/CALLER/CHECKOUT", "rev-parse", "--show-toplevel"); e == nil {
		t.Fatal("real runner accepted foreign code root")
	}
}

func TestVariableCallerV2NoFollowAndExplicitBounds(t *testing.T) {
	for _, kind := range []string{"config-alias", "array-bound", "string-bound", "duplicate-nested", "partition-zero", "policy-zero", "null-env", "fresh-output", "report-outside", "marker-output"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableOwnedConfig(t, 1, 0)
			switch kind {
			case "config-alias":
				link := filepath.Join(filepath.Dir(path), "config-link")
				if e := os.Symlink(path, link); e != nil {
					t.Fatal(e)
				}
				path = link
			case "array-bound":
				for len(c.Environment) < 33 {
					c.Environment = append(c.Environment, "X=1")
				}
				variableSave(t, path, c)
			case "string-bound":
				c.Server = strings.Repeat("a", 8193)
				variableSave(t, path, c)
			case "duplicate-nested":
				b, _ := json.Marshal(c)
				b = bytes.Replace(b, []byte(`"scope":{"root":`), []byte(`"scope":{"root":"duplicate","root":`), 1)
				if e := os.WriteFile(path, b, 0600); e != nil {
					t.Fatal(e)
				}
			case "partition-zero":
				c.Scope.PartitionSize = 0
				variableSave(t, path, c)
			case "policy-zero":
				c.Scope.EffectivePolicy.MaxTargets = 0
				variableSave(t, path, c)
			case "null-env":
				c.Environment = nil
				variableSave(t, path, c)
			case "fresh-output":
				if e := os.WriteFile(filepath.Join(c.PostOutputs[0], "existing"), []byte("preserve"), 0600); e != nil {
					t.Fatal(e)
				}
			case "report-outside":
				c.ReportPath = filepath.Join(filepath.Dir(path), "report.json")
				variableSave(t, path, c)
			case "marker-output":
				c.AttemptMarkerPath = filepath.Join(c.ProducerOutputs[0], "attempt.json")
				variableSave(t, path, c)
			}
			count := 0
			if _, e := variableOwnedRun(context.Background(), "execute-once", path, git, func() (*sessionruntime.Manager, error) { count++; return nil, nil }, variableClose); e == nil || count != 0 {
				t.Fatal("preflight boundary", kind, e, count)
			}
		})
	}
}
