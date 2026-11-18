package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcedriver"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/publication"
)

// Every native boundary is inert and observable; these tests cannot start a server.
func censusFolderHostGuardDependencies(calls *[]string) censusNativeCatalogDependencies {
	stop := errors.New("host guard test stop")
	return censusNativeCatalogDependencies{
		enumerate: func(censusCLIOptions) ([]censusacquisition.SourceFile, error) {
			*calls = append(*calls, "enumerate")
			return nil, stop
		},
		buildPlan: func([]censusacquisition.SourceFile, string, int) (censussourceplan.Plan, error) {
			*calls = append(*calls, "buildPlan")
			return censussourceplan.Plan{}, stop
		},
		validatePlan: func(censussourceplan.Plan, int) error {
			*calls = append(*calls, "validatePlan")
			return stop
		},
		runSession: func(initializedAcquisitionRunnerConfig, func(context.Context, *initializedAcquisitionRuntime) int) int {
			*calls = append(*calls, "runSession")
			return 1
		},
		openRoots: func(string) (*publication.Root, *publication.Root, func() error, error) {
			*calls = append(*calls, "openRoots")
			return nil, nil, nil, stop
		},
		discoverPartition: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
			*calls = append(*calls, "discoverPartition")
			return censusacquisition.Discovery{}, stop
		},
		runAssembly: func(context.Context, censussourceassembly.Request) (censussourcedriver.Result, *censussourceassembly.TerminalRecord) {
			*calls = append(*calls, "runAssembly")
			return censussourcedriver.Result{}, nil
		},
	}
}

func TestCensusFolderMissingHostNoNativeEffects(t *testing.T) {
	for _, machine := range []bool{false, true} {
		var calls []string
		var out, errout bytes.Buffer
		runCensusNativeCatalogWithDependencies(censusCLIOptions{FolderInvocation: true, Machine: machine}, censusCoreConfig{}, &out, &errout, censusFolderHostGuardDependencies(&calls))
		if len(calls) != 0 {
			t.Errorf("ASSERT_MISSING_HOST_ZERO_NATIVE_EFFECTS: machine=%v calls=%v", machine, calls)
		} else {
			t.Logf("ASSERT_MISSING_HOST_ZERO_NATIVE_EFFECTS: PASS machine=%v", machine)
		}
	}
}

func TestCensusFolderMissingHostDiagnostic(t *testing.T) {
	for _, machine := range []bool{false, true} {
		for _, locator := range []bool{false, true} {
			name := "human"
			if machine {
				name = "machine"
			}
			if locator {
				name += "/locator"
			}
			t.Run(name, func(t *testing.T) {
				var calls []string
				var out, errout bytes.Buffer
				var stderr io.Writer = &errout
				if locator {
					stderr = censusFolderDiagnosticWriter{Writer: &errout, runPath: "/retained/host-guard-run"}
				}
				code := runCensusNativeCatalogWithDependencies(censusCLIOptions{FolderInvocation: true, Machine: machine}, censusCoreConfig{}, &out, stderr, censusFolderHostGuardDependencies(&calls))
				if code != 1 || out.Len() != 0 {
					t.Fatalf("ASSERT_MISSING_HOST_FAILURE_ENVELOPE: code=%d stdout=%q", code, out.String())
				}
				detail := errout.String()
				if machine {
					var d struct {
						Schema                      string `json:"schema_version"`
						Status, Stage, Code, Detail string
						Retry                       bool
					}
					decoder := json.NewDecoder(&errout)
					if err := decoder.Decode(&d); err != nil {
						t.Fatalf("ASSERT_MISSING_HOST_FAILURE_ENVELOPE: %v", err)
					}
					var extra any
					if decoder.Decode(&extra) != io.EOF || d.Schema != "lsp-trace.census-diagnostic.v1" || d.Status != "FAILED" || d.Stage != "config" || d.Code != "INVALID_CONFIG" || !d.Retry {
						t.Fatalf("ASSERT_MISSING_HOST_FAILURE_ENVELOPE: %+v", d)
					}
					detail = d.Detail
				}
				if !strings.Contains(detail, "caller-hosted inventory connection is unavailable; no native acquisition was started") {
					t.Fatalf("ASSERT_MISSING_HOST_FAILURE_ENVELOPE: missing host reason: %q", detail)
				}
				if locator != strings.Contains(detail, "Retained census run directory: /retained/host-guard-run; locator only, not resume permission or completed inventory") {
					t.Fatalf("ASSERT_MISSING_HOST_FAILURE_ENVELOPE: locator=%v detail=%q", locator, detail)
				}
				t.Log("ASSERT_MISSING_HOST_FAILURE_ENVELOPE: PASS")
			})
		}
	}
}

func TestCensusFolderMissingHostPreservesExplicitNativeAcquisition(t *testing.T) {
	for _, machine := range []bool{false, true} {
		var calls []string
		var out, errout bytes.Buffer
		runCensusNativeCatalogWithDependencies(censusCLIOptions{FolderInvocation: false, Machine: machine}, censusCoreConfig{}, &out, &errout, censusFolderHostGuardDependencies(&calls))
		want := []byte("host guard test stop\n")
		if machine {
			diagnostic, _ := buildCensusCLIDiagnostic(censusStageDiscovery, nil)
			want, _ = marshalCensusCLIDiagnostic(diagnostic)
		}
		if len(calls) != 1 || calls[0] != "enumerate" || !bytes.Equal(errout.Bytes(), want) {
			t.Errorf("ASSERT_EXPLICIT_NATIVE_ACQUISITION_PRESERVED: machine=%v calls=%v stderr=%q", machine, calls, errout.String())
		} else {
			t.Logf("ASSERT_EXPLICIT_NATIVE_ACQUISITION_PRESERVED: PASS machine=%v", machine)
		}
	}
}
