package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func compileLocalMetricsSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	const base = "https://jaresty.github.io/lsp-trace/schemas/"
	for _, name := range []string{
		"lsp-trace.local-normative-analysis.v1.schema.json",
		"lsp-trace.local-normative-metrics.v1.schema.json",
	} {
		raw, err := os.ReadFile("schemas/" + name)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := compiler.AddResource(base+name, doc); err != nil {
			t.Fatal(err)
		}
	}
	s, err := compiler.Compile(base + "lsp-trace.local-normative-metrics.v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLocalMetricsSchemaRecursivelyClosed(t *testing.T) {
	const assertion = "ASSERT_METRICS_SCHEMA_RECURSIVELY_CLOSED"
	raw, err := os.ReadFile("schemas/lsp-trace.local-normative-metrics.v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	var walk func(any, string)
	walk = func(value any, path string) {
		switch value := value.(type) {
		case map[string]any:
			if typ, ok := value["type"]; ok && typ == "object" {
				properties, propertiesOK := value["properties"].(map[string]any)
				required, requiredOK := value["required"].([]any)
				requiredSet := map[string]bool{}
				for _, name := range required {
					requiredSet[name.(string)] = true
				}
				closed := propertiesOK && requiredOK && value["additionalProperties"] == false && len(requiredSet) == len(properties)
				for name := range properties {
					closed = closed && requiredSet[name]
				}
				if !closed {
					t.Fatalf("%s: open object at %s", assertion, path)
				}
			}
			if typ, ok := value["type"]; ok && typ == "array" {
				if _, ok := value["items"]; !ok {
					t.Fatalf("%s: unconstrained array at %s", assertion, path)
				}
			}
			for key, child := range value {
				walk(child, path+"/"+key)
			}
		case []any:
			for i, child := range value {
				walk(child, path+"/"+string(rune('0'+i)))
			}
		}
	}
	walk(root, "$")
}

func TestLocalMetricsSchemaRejectsNestedMutations(t *testing.T) {
	const assertion = "ASSERT_METRICS_SCHEMA_NESTED_MUTATION_REJECTED"
	schema := compileLocalMetricsSchema(t)
	valid := map[string]any{
		"Family": "normative-analytics", "Version": "local-v1", "Scope": "LOCAL_SYNTHETIC_FIXTURE_QUALIFIED_PACKAGE_PRIVATE_UNSHIPPED", "BuildRevision": "rev",
		"InputDigest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Operation": "METRICS", "SelectedRelations": []any{"CALLS"}, "Status": "COMPLETE",
		"Accounting": map[string]any{"DecoderUnits": 1, "NodeUnits": 2, "EdgeUnits": 1, "SelectionUnits": 1, "KernelUnits": 3, "Units": 8, "Limit": 8},
		"Analysis":   nil, "Ranking": nil, "Digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"Metrics": map[string]any{"Schema": "lsp-trace.local-normative-metrics.v1", "CountingMode": "edge-instances; structural-relations=unique(from,to,relation); independent-support=unique(validated-authority,custody,provenance-id,support-group)", "DensityDenominatorMode": "ordered-distinct-node-pairs n*(n-1)", "NodeCount": 2, "EdgeInstanceCount": 1, "UniqueStructuralRelationCount": 1, "IndependentSupportCount": 1, "Density": map[string]any{"Numerator": 1, "Denominator": 2}, "Omissions": []any{}, "Nodes": []any{map[string]any{"Node": "a", "InDegree": 0, "OutDegree": 1}, map[string]any{"Node": "b", "InDegree": 1, "OutDegree": 0}}},
	}
	if err := schema.Validate(valid); err != nil {
		t.Fatalf("%s: valid metrics rejected: %v", assertion, err)
	}
	mutations := map[string]func(map[string]any){
		"density-unknown":           func(m map[string]any) { m["Density"].(map[string]any)["Unknown"] = 0 },
		"density-missing-numerator": func(m map[string]any) { delete(m["Density"].(map[string]any), "Numerator") },
		"density-denominator-range": func(m map[string]any) { m["Density"].(map[string]any)["Denominator"] = 1 },
		"density-numerator-type":    func(m map[string]any) { m["Density"].(map[string]any)["Numerator"] = "1" },
		"omission-unknown": func(m map[string]any) {
			m["Omissions"] = []any{map[string]any{"Metric": "directed_density", "Reason": "requires at least two nodes", "Unknown": true}}
		},
		"omission-missing-reason": func(m map[string]any) { m["Omissions"] = []any{map[string]any{"Metric": "directed_density"}} },
		"omission-metric-constant": func(m map[string]any) {
			m["Omissions"] = []any{map[string]any{"Metric": "other", "Reason": "requires at least two nodes"}}
		},
		"omission-item-type":   func(m map[string]any) { m["Omissions"] = []any{"directed_density"} },
		"node-unknown":         func(m map[string]any) { m["Nodes"].([]any)[0].(map[string]any)["Unknown"] = 0 },
		"node-missing-degree":  func(m map[string]any) { delete(m["Nodes"].([]any)[0].(map[string]any), "InDegree") },
		"node-name-min-length": func(m map[string]any) { m["Nodes"].([]any)[0].(map[string]any)["Node"] = "" },
		"node-degree-range":    func(m map[string]any) { m["Nodes"].([]any)[0].(map[string]any)["OutDegree"] = -1 },
		"node-item-type":       func(m map[string]any) { m["Nodes"] = []any{"a"} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(valid)
			var candidate map[string]any
			json.Unmarshal(raw, &candidate)
			mutate(candidate["Metrics"].(map[string]any))
			if err := schema.Validate(candidate); err == nil {
				t.Fatalf("%s: %s accepted", assertion, name)
			}
		})
	}
}
