package acquisitionmanifest_test

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/operation"
)

// Assignment of pointers and function signatures requires identity, not conversion.
var (
	_ *acquisitionops.Manifest                                                            = (*acquisitionmanifest.Manifest)(nil)
	_ *acquisitionengine.Manifest                                                         = (*acquisitionmanifest.Manifest)(nil)
	_ *acquisitionops.Target                                                              = (*acquisitionmanifest.Target)(nil)
	_ *acquisitionengine.Target                                                           = (*acquisitionmanifest.Target)(nil)
	_ *acquisitionops.Limits                                                              = (*acquisitionmanifest.Limits)(nil)
	_ *acquisitionengine.Limits                                                           = (*acquisitionmanifest.Limits)(nil)
	_ *acquisitionops.Expansion                                                           = (*acquisitionmanifest.Expansion)(nil)
	_ *acquisitionengine.Expansion                                                        = (*acquisitionmanifest.Expansion)(nil)
	_ func(censusacquisition.BatchRequest, acquisitionops.Limits) acquisitionops.Manifest = censusacquisition.BatchRequest.AcquisitionManifest
	_ func(acquisitionops.Manifest, operation.Name) (acquisition.Request, error)          = acquisitionmanifest.Manifest.Request
)

const wirePrefix = `{"schema_version":"lsp-trace.seed-manifest.v2","coordinate_convention":"zero-based-session","root":{"id":"root","locator":{"uri":"file:///fixture/a.go","line":0,"character":0}},"required_targets":`

func fixture() acquisitionmanifest.Manifest {
	zero := uint32(0)
	return acquisitionmanifest.Manifest{SchemaVersion: acquisitionmanifest.ManifestVersion, CoordinateConvention: "zero-based-session", Root: acquisitionmanifest.Target{ID: "root", Locator: acquisition.Locator{URI: "file:///fixture/a.go", Line: &zero, Character: &zero}}, RequiredTargets: []acquisitionmanifest.Target{}}
}
func TestManifestWireContract(t *testing.T) {
	for _, nilTargets := range []bool{false, true} {
		m := fixture()
		targets := "[]"
		if nilTargets {
			m.RequiredTargets = nil
			targets = "null"
		}
		got, err := json.Marshal(m)
		want := wirePrefix + targets + `,"limits":{},"expansion":{"topmost_siblings":false}}`
		if err != nil || string(got) != want {
			t.Fatalf("manifest-wire: got %s error %v want %s", got, err, want)
		}
	}
	t.Log("manifest-wire: PASS")
}
func TestManifestRequestContract(t *testing.T) {
	for _, mode := range []operation.Name{acquisitionops.Slice, acquisitionops.Incoming, acquisitionops.SliceV3, acquisitionops.IncomingV3} {
		m := fixture()
		got, err := m.Request(mode)
		wantMode := acquisition.Slice
		if mode == acquisitionops.Incoming || mode == acquisitionops.IncomingV3 {
			wantMode = acquisition.Incoming
		}
		want := acquisition.Request{Context: acquisition.AcquisitionContext{ID: "preflight", SessionID: "preflight", Generation: 1, PositionEncoding: "utf-16"}, Mode: wantMode, Root: acquisition.Target{ID: "root", Locator: m.Root.Locator, DownDepth: 2, UpDepth: 2}, RequiredTargets: []acquisition.Target{}, Limits: acquisition.Limits{MaxNodes: 100, MaxRequests: 1000, MaxEvidenceBytes: 4 << 20, MaxPathWork: 100000, Timeout: 5 * time.Second, RequestTimeout: time.Second, MaxResponseBytes: 4 << 20, MaxMessages: 64}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("manifest-defaults: mode %s got %#v error %v want %#v", mode, got, err, want)
		}
	}
	t.Log("manifest-defaults: PASS")
}
func TestManifestRejectionOrder(t *testing.T) {
	m := fixture()
	zero := 0
	m.SchemaVersion = "bad"
	m.CoordinateConvention = "bad"
	m.RequiredTargets = nil
	m.Limits.TimeoutMS = &zero
	checks := []struct {
		mode operation.Name
		want string
		fix  func()
	}{
		{"bad", `unsupported acquisition operation "bad"`, func() {}},
		{acquisitionops.Slice, `unsupported seed manifest version "bad"`, func() { m.SchemaVersion = acquisitionmanifest.ManifestVersion }},
		{acquisitionops.Slice, "coordinate_convention must be zero-based-session", func() { m.CoordinateConvention = "zero-based-session" }},
		{acquisitionops.Slice, "required_targets array is required (may be empty)", func() { m.RequiredTargets = []acquisitionmanifest.Target{} }},
		{acquisitionops.Slice, "timeouts must be 1..60000 milliseconds", func() { m.Limits.TimeoutMS = nil }},
	}
	for _, check := range checks {
		_, err := m.Request(check.mode)
		if err == nil || err.Error() != check.want {
			t.Fatalf("manifest-rejections: got %v want %q", err, check.want)
		}
		check.fix()
	}
	if _, err := m.Request(acquisitionops.Slice); err != nil {
		t.Fatal(err)
	}
	t.Log("manifest-rejections: PASS")
}
func TestManifestProductionDependencyBoundary(t *testing.T) {
	for _, pkg := range []string{"lsp-trace/internal/censusacquisition", "lsp-trace/internal/acquisitionmanifest"} {
		out, err := exec.Command("go", "list", "-f", `{{join .Deps "\n"}}`, pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("dependency query: %v: %s", err, out)
		}
		for _, dep := range strings.Fields(string(out)) {
			for _, forbidden := range []string{"lsp-trace/internal/acquisitionengine", "lsp-trace/acquisitionops", "lsp-trace/internal/censuslineagepilot", "lsp-trace/internal/acquisitionorchestration"} {
				if dep == forbidden {
					t.Fatalf("manifest-dependencies: %s reaches %s", pkg, dep)
				}
			}
			if pkg == "lsp-trace/internal/acquisitionmanifest" && dep == "lsp-trace/internal/censusacquisition" {
				t.Fatalf("manifest-dependencies: lower package reaches censusacquisition")
			}
		}
	}
	t.Log("manifest-dependencies: PASS")
}
