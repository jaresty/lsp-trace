package mcp

import (
	"os"
	"reflect"
	"sort"
	"testing"
)

// adr0004OperationNumbers is the compatibility numbering established by the
// manifest-extension order. It is deliberately not derived from Registry.Tools,
// whose public presentation is lexical. Operation 31 is composition and 32 is
// A-08 instability; insertions and renumbering are compatibility breaks.
var adr0004OperationNumbers = []string{
	"lsp_trace_v1_capabilities",                 // 1
	"lsp_trace_v1_schema_get",                   // 2
	"lsp_trace_v1_validate",                     // 3
	"lsp_trace_v1_verify",                       // 4
	"lsp_trace_v1_inspect",                      // 5
	"lsp_trace_v1_filter",                       // 6
	"lsp_trace_v1_execute",                      // 7
	"lsp_session_v1_list",                       // 8
	"lsp_session_v1_status",                     // 9
	"lsp_session_v1_restart",                    // 10
	"lsp_session_v1_stop",                       // 11
	"lsp_trace_v1_incoming",                     // 12
	"lsp_trace_v1_slice",                        // 13
	"lsp_trace_v1_export_retained_calls",        // 14
	"lsp_trace_v1_bounded_retained_analysis",    // 15
	"lsp_trace_v1_bounded_retained_metrics",     // 16
	"lsp_trace_v1_bounded_retained_ranking",     // 17
	"lsp_trace_v1_inspect_hydrated",             // 18
	"lsp_trace_v2_export_retained_calls",        // 19
	"lsp_trace_v2_verify_retained_calls",        // 20
	"lsp_trace_v2_slice",                        // 21
	"lsp_trace_v2_incoming",                     // 22
	"lsp_trace_v2_verify",                       // 23
	"lsp_trace_v3_slice",                        // 24
	"lsp_trace_v3_incoming",                     // 25
	"lsp_trace_v2_bounded_retained_analysis",    // 26
	"lsp_trace_v2_bounded_retained_metrics",     // 27
	"lsp_trace_v2_bounded_retained_ranking",     // 28
	"lsp_trace_v1_custody_execute",              // 29
	"lsp_trace_v1_program_c_leiden",             // 30
	"lsp_trace_v1_program_c_compose",            // 31
	"lsp_trace_v1_program_c_instability",        // 32
	"lsp_trace_v1_trace",                        // 33
	"lsp_trace_v1_census",                       // 34
	"lsp_trace_v1_structural_context",           // 35
	"lsp_trace_v2_structural_context",           // 36
	"lsp_trace_v1_structural_delta",             // 37
	"lsp_trace_v1_context_churn",                // 38
	"lsp_trace_v1_context_symbol_churn",         // 39
	"lsp_trace_v1_context_symbol_churn_capture", // 40
	"lsp_trace_v1_structural_context_symbol",    // 41
	"lsp_session_v1_derive_workspace",           // 42
	"lsp_trace_v2_structural_context_symbol",    // 43
}

var adr0004DefaultCurrent = []string{
	"lsp_trace_v1_capabilities",
	"lsp_trace_v1_census",
	"lsp_trace_v1_context_churn",
	"lsp_trace_v1_context_symbol_churn",
	"lsp_trace_v1_context_symbol_churn_capture",
	"lsp_trace_v1_execute",
	"lsp_trace_v1_inspect_hydrated",
	"lsp_trace_v1_program_c_leiden",
	"lsp_trace_v1_structural_context",
	"lsp_trace_v1_structural_context_symbol",
	"lsp_trace_v1_structural_delta",
	"lsp_trace_v1_trace",
	"lsp_trace_v1_verify",
	"lsp_trace_v2_structural_context",
	"lsp_trace_v2_structural_context_symbol",
}

var adr0004AdvancedOnly = []string{
	"lsp_session_v1_derive_workspace", "lsp_session_v1_list", "lsp_session_v1_restart", "lsp_session_v1_status", "lsp_session_v1_stop",
	"lsp_trace_v1_bounded_retained_analysis", "lsp_trace_v1_bounded_retained_metrics", "lsp_trace_v1_bounded_retained_ranking",
	"lsp_trace_v1_custody_execute", "lsp_trace_v1_export_retained_calls", "lsp_trace_v1_filter", "lsp_trace_v1_inspect",
	"lsp_trace_v1_program_c_compose", "lsp_trace_v1_program_c_instability", "lsp_trace_v1_schema_get", "lsp_trace_v1_validate",
	"lsp_trace_v2_bounded_retained_analysis", "lsp_trace_v2_bounded_retained_metrics", "lsp_trace_v2_bounded_retained_ranking",
	"lsp_trace_v2_export_retained_calls", "lsp_trace_v2_verify", "lsp_trace_v2_verify_retained_calls",
}

var adr0004HiddenLegacy = []string{
	"lsp_trace_v1_incoming", "lsp_trace_v1_slice",
	"lsp_trace_v2_incoming", "lsp_trace_v2_slice",
	"lsp_trace_v3_incoming", "lsp_trace_v3_slice",
}

func TestADR0004RegressionGuardCanonicalExecuteKeepsHistorical33AndAppends34(t *testing.T) {
	r := NewRegistry(false)
	if len(adr0004OperationNumbers) != 43 {
		t.Fatalf("REGRESSION: operation fixture has %d entries, want current 43", len(adr0004OperationNumbers))
	}
	if adr0004OperationNumbers[30] != "lsp_trace_v1_program_c_compose" || adr0004OperationNumbers[31] != "lsp_trace_v1_program_c_instability" {
		t.Fatalf("REGRESSION: historical operations 31/32 changed: %q, %q", adr0004OperationNumbers[30], adr0004OperationNumbers[31])
	}
	if adr0004OperationNumbers[32] != "lsp_trace_v1_trace" || adr0004OperationNumbers[33] != "lsp_trace_v1_census" || adr0004OperationNumbers[34] != "lsp_trace_v1_structural_context" {
		t.Fatalf("REGRESSION: operations 33/34/35 changed: %q/%q/%q", adr0004OperationNumbers[32], adr0004OperationNumbers[33], adr0004OperationNumbers[34])
	}

	got := toolNames(r.Tools())
	want := append([]string(nil), adr0004OperationNumbers...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("REGRESSION: canonical append-only operation inventory changed\n got: %v\nwant: %v", got, want)
	}
	for number, name := range adr0004OperationNumbers {
		if tool, ok := r.ResolveCanonical(name); !ok || tool.Name != name {
			t.Errorf("REGRESSION: operation %d %q is not canonically dispatchable", number+1, name)
		}
	}

	execute, ok := r.ResolveCanonical("lsp_trace_v1_execute")
	if !ok {
		t.Fatal("REGRESSION: canonical execute operation is missing")
	}
	request := execute.InputSchema["properties"].(map[string]any)["request"].(map[string]any)
	branches := request["oneOf"].([]any)
	seen := make(map[string]bool, len(branches))
	for _, raw := range branches {
		branch := raw.(map[string]any)
		properties := branch["properties"].(map[string]any)
		operation := properties["operation"].(map[string]any)["const"].(string)
		seen[operation] = true
	}
	if len(seen) != 42 {
		t.Fatalf("REGRESSION: canonical execute has %d operation branches, want 42", len(seen))
	}
	for _, name := range adr0004OperationNumbers {
		if name != "lsp_trace_v1_execute" && !seen[name] {
			t.Errorf("REGRESSION: canonical execute lost branch for %q", name)
		}
	}
}

func TestADR0004RegressionGuardProfilePartitionCoversNumbered34(t *testing.T) {
	classified := append(append(append([]string{}, adr0004DefaultCurrent...), adr0004AdvancedOnly...), adr0004HiddenLegacy...)
	sort.Strings(classified)
	want := append([]string(nil), adr0004OperationNumbers...)
	sort.Strings(want)
	if !reflect.DeepEqual(classified, want) {
		t.Fatalf("REGRESSION: profile contract is not an exact disjoint partition\n got: %v\nwant: %v", classified, want)
	}
}

func TestADR0004CurrentAdvertisementProfiles(t *testing.T) {
	gotDefault := toolNames(NewRegistryWithProfile(false, ToolProfileDefault).Advertised())
	if !reflect.DeepEqual(gotDefault, adr0004DefaultCurrent) {
		t.Errorf("default current advertisement\n got: %v\nwant: %v", gotDefault, adr0004DefaultCurrent)
	}

	advancedTarget := append(append([]string{}, adr0004DefaultCurrent...), adr0004AdvancedOnly...)
	sort.Strings(advancedTarget)
	gotAdvanced := toolNames(NewRegistryWithProfile(false, ToolProfileAdvanced).Advertised())
	if !reflect.DeepEqual(gotAdvanced, advancedTarget) {
		t.Errorf("advanced current advertisement\n got: %v\nwant: %v", gotAdvanced, advancedTarget)
	}

	for _, profile := range []ToolProfile{ToolProfileDefault, ToolProfileAdvanced} {
		r := NewRegistryWithProfile(false, profile)
		advertised := make(map[string]bool, len(r.Advertised()))
		for _, tool := range r.Advertised() {
			advertised[tool.Name] = true
		}
		for _, name := range adr0004HiddenLegacy {
			if advertised[name] {
				t.Errorf("hidden legacy operation %q advertised by %s", name, profile)
			}
			if _, ok := r.ResolveCanonical(name); !ok {
				t.Errorf("hidden legacy operation %q is no longer canonically dispatchable in %s", name, profile)
			}
		}
	}
}

func TestADR0004IntentionalREDFutureDefaultAdvertisement(t *testing.T) {
	if os.Getenv("LSP_TRACE_RUN_ADR0004_RED_GUARDS") != "1" {
		t.Skip("INTENTIONAL RED: ADR 0004 target profiles require unimplemented lsp_trace_v1_discover")
	}

	defaultTarget := append([]string{"lsp_trace_v1_discover"}, adr0004DefaultCurrent...)
	sort.Strings(defaultTarget)
	gotDefault := toolNames(NewRegistryWithProfile(false, ToolProfileDefault).Advertised())
	if !reflect.DeepEqual(gotDefault, defaultTarget) {
		t.Errorf("INTENTIONAL RED: future default target advertisement\n got: %v\nwant: %v", gotDefault, defaultTarget)
	}
}
