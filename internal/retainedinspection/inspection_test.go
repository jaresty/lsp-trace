package retainedinspection

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	hi "lsp-trace/internal/hydratedinspection"
)

const goodDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const goodGen = "g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func validProjection(carrier map[string]any) map[string]any {
	return map[string]any{
		"mode":                     Mode,
		"retained_source_evidence": carrier,
		"selection": map[string]any{
			"target":     map[string]any{"graph_subject_id": "n", "logical_source_id": "file:///a.go"},
			"selections": []any{map[string]any{"graph_subject_id": "n", "logical_source_id": "file:///a.go"}},
		},
		"projection": map[string]any{
			"body": "INCLUDE", "privacy_policy_id": goodDigest,
			"limits": map[string]any{"max_source_bytes": 16 << 20, "max_ranges": 10000, "max_objects": 10000, "max_work": 512 << 20, "max_response_bytes": 16 << 20},
		},
		"resolve_limits": map[string]any{"max_distinct_objects": 10000, "max_unique_source_bytes": 64 << 20, "max_logical_selections": 10000},
	}
}

func raw(v any) []byte { b, _ := json.Marshal(v); return b }
func clone(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	b := raw(v)
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func acceptsSchema(t *testing.T, schema []byte, value any) bool {
	t.Helper()
	var meta map[string]any
	if err := json.Unmarshal(schema, &meta); err != nil {
		t.Fatal(err)
	}
	id := meta["$id"].(string)
	c := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err == nil {
		err = c.AddResource(id, doc)
	}
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	d, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw(value)))
	return err == nil && s.Validate(d) == nil
}

func TestV1BytesAndLanguagePreserved(t *testing.T) {
	beforeIn, beforeOut := append([]byte(nil), hi.InputSchema()...), append([]byte(nil), hi.OutputSchema()...)
	legacy := []string{
		`{"input":"{}"}`,
		`{"input":"{}","page":true}`,
		`{"input":"{}","page":true,"cursor":"e30"}`,
		`{"publication_selector":{"selector":"s","artifact_digest":"` + goodDigest + `","artifact_byte_length":1,"artifact_schema_id":"x","publication_mechanism":"atomic_no_replace_with_verified_generation","generation":"` + goodGen + `","verification_selector":"v"}}`,
		`{"content_addressed_artifact":{"id":"` + goodDigest + `","artifact_byte_length":1,"artifact_schema_id":"x","generation":"` + goodGen + `"}}`,
	}
	for _, s := range legacy {
		b := []byte(s)
		if err := hi.ValidateInputJSON(b); err != nil {
			t.Fatalf("V1 rejected representative: %v", err)
		}
		got, err := Decode(b)
		if err != nil || got.Legacy == nil || got.Projection != nil {
			t.Fatalf("V2 legacy decode: %v", err)
		}
	}
	large := []byte(`{"input":"x","sidecars":["` + strings.Repeat("a", hi.MaxRequestBytes-64) + `"]}`)
	if len(large) >= hi.MaxRequestBytes {
		large = []byte(`{"input":"` + strings.Repeat("a", (1<<20)-1) + `","sidecars":["` + strings.Repeat("b", hi.MaxRequestBytes-(1<<20)-64) + `"]}`)
	}
	if err := hi.ValidateInputJSON(large); err == nil {
		if _, err := Decode(large); err != nil {
			t.Fatalf("V2 narrowed valid V1 byte bound: %v", err)
		}
	}
	if !bytes.Equal(beforeIn, hi.InputSchema()) || !bytes.Equal(beforeOut, hi.OutputSchema()) {
		t.Fatal("V1 schema bytes changed")
	}
}

func TestProjectionCarriersAndSemantics(t *testing.T) {
	carriers := []map[string]any{
		{"inline_snapshot_v2": "{}"},
		{"publication_snapshot_v2": map[string]any{"selector": "s", "artifact_digest": goodDigest, "artifact_byte_length": 1, "artifact_schema_id": SourceSnapshotSchemaID, "publication_mechanism": "atomic_no_replace_with_verified_generation", "generation": goodGen, "verification_selector": "v"}},
		{"content_addressed_snapshot_v2": map[string]any{"id": goodDigest, "artifact_byte_length": 1, "artifact_schema_id": SourceSnapshotSchemaID, "generation": goodGen}},
	}
	for _, c := range carriers {
		v := validProjection(c)
		got, err := Decode(raw(v))
		if err != nil || got.Projection == nil || got.Legacy != nil {
			t.Fatalf("valid projection carrier rejected: %v", err)
		}
		if err := hi.ValidateInputJSON(raw(v)); err == nil {
			t.Fatal("projection accepted under V1")
		}
	}
	bad := []map[string]any{}
	wrong := validProjection(map[string]any{"content_addressed_snapshot_v2": map[string]any{"id": goodDigest, "artifact_byte_length": 1, "artifact_schema_id": "wrong", "generation": goodGen}})
	bad = append(bad, wrong)
	for _, field := range []string{"input", "sidecars", "page", "cursor", "path", "root"} {
		v := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
		v[field] = map[bool]any{true: true, false: "legacy"}[field == "page"]
		bad = append(bad, v)
	}
	for _, mode := range []any{"WRONG", nil} {
		v := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
		if mode == nil {
			delete(v, "mode")
		} else {
			v["mode"] = mode
		}
		bad = append(bad, v)
	}
	mixed := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
	mixed["publication_selector"] = map[string]any{}
	bad = append(bad, mixed)
	float := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
	float["projection"].(map[string]any)["limits"].(map[string]any)["max_work"] = 1.5
	bad = append(bad, float)
	negative := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
	negative["projection"].(map[string]any)["limits"].(map[string]any)["max_ranges"] = -1
	bad = append(bad, negative)
	zero := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
	zero["resolve_limits"].(map[string]any)["max_distinct_objects"] = 0
	bad = append(bad, zero)
	over := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
	over["projection"].(map[string]any)["limits"].(map[string]any)["max_source_bytes"] = (16 << 20) + 1
	bad = append(bad, over)
	digest := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
	digest["projection"].(map[string]any)["privacy_policy_id"] = "SHA256:AA"
	bad = append(bad, digest)
	for _, v := range bad {
		if _, err := Decode(raw(v)); err == nil {
			t.Fatalf("invalid projection accepted: %s", raw(v))
		}
	}

	dup := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
	sel := dup["selection"].(map[string]any)
	sel["selections"] = append(sel["selections"].([]any), map[string]any{"graph_subject_id": "n", "logical_source_id": "file:///a.go"})
	if _, err := Decode(raw(dup)); err == nil {
		t.Fatal("duplicate selection accepted")
	}
	absent := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
	absent["selection"].(map[string]any)["selections"] = []any{map[string]any{"graph_subject_id": "other", "logical_source_id": "file:///b.go"}}
	if _, err := Decode(raw(absent)); err == nil {
		t.Fatal("absent target accepted")
	}
}

func TestProjectionEveryNumericBoundary(t *testing.T) {
	for _, tc := range []struct {
		group string
		field string
		bad   any
	}{
		{"projection", "max_source_bytes", (16 << 20) + 1},
		{"projection", "max_ranges", 10001},
		{"projection", "max_objects", 10001},
		{"projection", "max_work", (512 << 20) + 1},
		{"projection", "max_response_bytes", 0},
		{"projection", "max_response_bytes", (16 << 20) + 1},
		{"resolve", "max_distinct_objects", 0},
		{"resolve", "max_distinct_objects", 10001},
		{"resolve", "max_unique_source_bytes", 0},
		{"resolve", "max_unique_source_bytes", (64 << 20) + 1},
		{"resolve", "max_logical_selections", 0},
		{"resolve", "max_logical_selections", 10001},
	} {
		v := validProjection(map[string]any{"inline_snapshot_v2": "{}"})
		if tc.group == "projection" {
			v["projection"].(map[string]any)["limits"].(map[string]any)[tc.field] = tc.bad
		} else {
			v["resolve_limits"].(map[string]any)[tc.field] = tc.bad
		}
		if _, err := Decode(raw(v)); err == nil {
			t.Fatalf("accepted %s.%s=%v", tc.group, tc.field, tc.bad)
		}
	}
	tooLarge := validProjection(map[string]any{"inline_snapshot_v2": strings.Repeat("x", (1<<20)+1)})
	if _, err := Decode(raw(tooLarge)); err == nil {
		t.Fatal("inline snapshot overflow accepted")
	}
	mixed := validProjection(map[string]any{"inline_snapshot_v2": "{}", "content_addressed_snapshot_v2": map[string]any{"id": goodDigest, "artifact_byte_length": 1, "artifact_schema_id": SourceSnapshotSchemaID, "generation": goodGen}})
	if _, err := Decode(raw(mixed)); err == nil {
		t.Fatal("multiple projection carriers accepted")
	}
	wrongPublication := validProjection(map[string]any{"publication_snapshot_v2": map[string]any{"selector": "s", "artifact_digest": goodDigest, "artifact_byte_length": 1, "artifact_schema_id": "wrong", "publication_mechanism": "atomic_no_replace_with_verified_generation", "generation": goodGen, "verification_selector": "v"}})
	if _, err := Decode(raw(wrongPublication)); err == nil {
		t.Fatal("wrong publication snapshot schema accepted")
	}
}

func TestEnvelopeConventionsAndClosure(t *testing.T) {
	artifact := map[string]any{"envelope_version": "1", "envelope_schema_id": ArtifactEnvelopeSchemaID, "tool": "lsp_trace_v1_inspect_hydrated", "request_id": "r", "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "artifact_schema_id": SourceProjectionSchemaID, "content": "{}"}
	if !acceptsSchema(t, ArtifactEnvelopeSchema(), artifact) {
		t.Fatal("valid artifact envelope rejected")
	}
	for _, mutate := range []func(map[string]any){func(v map[string]any) { v["content"] = map[string]any{} }, func(v map[string]any) { v["artifact_schema_id"] = "wrong" }, func(v map[string]any) { v["tool"] = "other" }, func(v map[string]any) { v["operation"] = 41 }} {
		v := clone(t, artifact)
		mutate(v)
		if acceptsSchema(t, ArtifactEnvelopeSchema(), v) {
			t.Fatalf("invalid artifact accepted: %v", v)
		}
	}
	errorEnvelope := map[string]any{"envelope_version": "1", "envelope_schema_id": DomainErrorEnvelopeSchemaID, "tool": "lsp_trace_v1_inspect_hydrated", "request_id": "r", "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "phase": "RESOLVE", "state": "MISSING", "selection_key": map[string]any{"graph_subject_id": "n", "logical_source_id": "file:///a.go"}, "diagnostics": []any{"safe"}}
	if !acceptsSchema(t, DomainErrorEnvelopeSchema(), errorEnvelope) {
		t.Fatal("valid domain error rejected")
	}
	for _, mutate := range []func(map[string]any){func(v map[string]any) { v["outcome"] = "COMPLETE" }, func(v map[string]any) { v["phase"] = "OTHER" }, func(v map[string]any) { v["state"] = "SOURCE_OBJECT_MISSING" }, func(v map[string]any) { v["path"] = "/secret" }, func(v map[string]any) { v["selector"] = "secret" }, func(v map[string]any) { v["diagnostics"] = []any{strings.Repeat("x", 1025)} }, func(v map[string]any) { v["operation"] = 41 }} {
		v := clone(t, errorEnvelope)
		mutate(v)
		if acceptsSchema(t, DomainErrorEnvelopeSchema(), v) {
			t.Fatalf("invalid error accepted: %v", v)
		}
	}
	for _, state := range ErrorStates() {
		v := clone(t, errorEnvelope)
		v["state"] = state
		if !acceptsSchema(t, DomainErrorEnvelopeSchema(), v) {
			t.Fatalf("state %s rejected", state)
		}
	}
}
