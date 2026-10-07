package groupqualificationv1

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

func generateGroupFreeze(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "freeze")
	if _, err := GenerateArtifacts(root, "."); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArtifacts(root); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestFreezeGeneratorVerifierTwiceByteIdentical(t *testing.T) {
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	ida, e := GenerateArtifacts(a, ".")
	if e != nil {
		t.Fatal(e)
	}
	idb, e := GenerateArtifacts(b, ".")
	if e != nil {
		t.Fatal(e)
	}
	if ida != idb {
		t.Fatalf("ids %s %s", ida, idb)
	}
	fa, _ := freezeTree(a)
	fb, _ := freezeTree(b)
	if !reflect.DeepEqual(fa, fb) {
		t.Fatal("trees differ")
	}
	for _, f := range fa {
		aa, _ := os.ReadFile(filepath.Join(a, f.Path))
		bb, _ := os.ReadFile(filepath.Join(b, f.Path))
		if !bytes.Equal(aa, bb) {
			t.Fatalf("%s differs", f.Path)
		}
	}
}
func TestFreezeExact24CausalAccounting(t *testing.T) {
	root := generateGroupFreeze(t)
	entries, e := os.ReadDir(filepath.Join(root, "cases"))
	if e != nil || len(entries) != 24 {
		t.Fatalf("count=%d err=%v", len(entries), e)
	}
	names := make([]string, 0, 24)
	successes, terminals := 0, 0
	for _, entry := range entries {
		names = append(names, entry.Name())
		var exp freezeExpected
		if e = freezeRead(filepath.Join(root, "cases", entry.Name(), "EXPECTED.json"), &exp); e != nil {
			t.Fatal(e)
		}
		if exp.Success {
			successes++
			for _, n := range []string{"RESULT.json", "REVIEW_REQUEST.json", "REVIEW_RAW.json", "REVIEW_ATTEMPT.json", "REVIEW.json", "ACCOUNT.json"} {
				if _, e = os.Stat(filepath.Join(root, "cases", entry.Name(), n)); e != nil {
					t.Fatalf("%s/%s", entry.Name(), n)
				}
			}
		} else if exp.Committed {
			if _, e = os.Stat(filepath.Join(root, "cases", entry.Name(), "RESULT.json")); e != nil {
				t.Fatalf("committed %s lacks result", entry.Name())
			}
		} else {
			terminals++
			if _, e = os.Stat(filepath.Join(root, "cases", entry.Name(), "RESULT.json")); !os.IsNotExist(e) {
				t.Fatalf("terminal %s has result", entry.Name())
			}
		}
	}
	if !sort.StringsAreSorted(names) || successes == 0 || terminals == 0 {
		t.Fatalf("sorted=%v success=%d terminal=%d", sort.StringsAreSorted(names), successes, terminals)
	}
}
func TestFreezeMetadataAndSources(t *testing.T) {
	root := generateGroupFreeze(t)
	b, e := os.ReadFile(filepath.Join(root, "FREEZE.json"))
	if e != nil {
		t.Fatal(e)
	}
	var f DesignFreeze
	if e = freezeStrict(b, &f); e != nil {
		t.Fatal(e)
	}
	if f.Status != "DESIGN_FROZEN_NON_DISPATCHING" || f.DispatchAllowed || f.GroupExecuted || f.GroupDesignGO || !strings.HasPrefix(f.FreezeID, "group-freeze-") {
		t.Fatalf("%+v", f)
	}
	if len(f.Sources) != 4 {
		t.Fatal(f.Sources)
	}
	for _, x := range f.Files {
		if x.Path == "FREEZE.json" {
			t.Fatal("self included")
		}
	}
	bindings := FreezeSourceBindings()
	for _, k := range []string{"integratedMainTree", "describeIntegrationCommit", "searchIntegrationCommit", "searchFreezeIdentity", "groupDesignCommit", "groupCoreCommit", "searchCase01Request", "searchCase01Result", "searchCase01Producer", "searchCase01ReviewRequest", "searchCase01ReviewAttempt", "searchCase01Account", "searchCase01Custody"} {
		if bindings[k] == "" {
			t.Fatal(k)
		}
	}
}
func TestFreezePublishedSchemasClosedDraft(t *testing.T) {
	for name, s := range FreezeSchemas() {
		if s["$schema"] != "https://json-schema.org/draft/2020-12/schema" || s["$id"] != Version+".schema."+name {
			t.Fatal(name)
		}
		assertFreezeClosed(t, name, s)
	}
}
func assertFreezeClosed(t *testing.T, path string, s map[string]any) {
	t.Helper()
	if s["type"] == "object" {
		if s["additionalProperties"] != false {
			t.Fatalf("open %s", path)
		}
		p, ok := s["properties"].(map[string]any)
		if !ok {
			t.Fatalf("properties %s", path)
		}
		for n, v := range p {
			assertFreezeClosed(t, path+"."+n, v.(map[string]any))
		}
	}
	if s["type"] == "array" {
		assertFreezeClosed(t, path+"[]", s["items"].(map[string]any))
	}
	if variants, ok := s["anyOf"].([]any); ok {
		for i, v := range variants {
			assertFreezeClosed(t, path+string(rune('0'+i)), v.(map[string]any))
		}
	}
}
func TestFreezeStrictAdversarial(t *testing.T) {
	r := freezeRequest()
	valid := freezeCanonical(r)
	if _, e := ParseGroupRequest(valid); e != nil {
		t.Fatal(e)
	}
	for n, b := range map[string][]byte{"duplicate": []byte(`{"SchemaVersion":"x","SchemaVersion":"y"}` + "\n"), "unknown": []byte(`{"unknown":true}` + "\n"), "trailing": append(append([]byte{}, valid...), 'x'), "invalid_utf8": {0xff, '\n'}} {
		t.Run(n, func(t *testing.T) {
			if _, e := ParseGroupRequest(b); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	for n, s := range map[string]string{"syntax": "!!!", "padding": "YQ=", "decoded_plus_one": base64.StdEncoding.EncodeToString(make([]byte, MaxResponseBytes+1))} {
		t.Run(n, func(t *testing.T) {
			if VerifyRawBase64(s, MaxResponseBytes) == nil {
				t.Fatal("accepted")
			}
		})
	}
	if VerifyRawBase64(base64.StdEncoding.EncodeToString(make([]byte, MaxResponseBytes)), MaxResponseBytes) != nil {
		t.Fatal("boundary")
	}
	for _, s := range []string{strings.Repeat("x", 256), strings.Repeat("é", 128)} {
		if !utf8.ValidString(s) || len([]byte(s)) != 256 || !validID(s) {
			t.Fatal("256 rejected")
		}
	}
	for _, s := range []string{strings.Repeat("x", 257), strings.Repeat("é", 129)} {
		if validID(s) {
			t.Fatal("overflow accepted")
		}
	}
}
func TestFreezeMutationAndReadOnly(t *testing.T) {
	root := generateGroupFreeze(t)
	casePath := filepath.Join(root, "cases", "01-complete-two-candidates", "REQUEST.json")
	b, _ := os.ReadFile(casePath)
	var v map[string]any
	json.Unmarshal(b, &v)
	v["unknown"] = true
	b, _ = json.Marshal(v)
	os.WriteFile(casePath, append(b, '\n'), 0644)
	if _, e := GenerateFreeze(root, "."); e != nil {
		t.Fatal(e)
	}
	if VerifyArtifacts(root) == nil {
		t.Fatal("mutation accepted")
	}
	root = generateGroupFreeze(t)
	if e := MakeReadOnly(root); e != nil {
		t.Fatal(e)
	}
	filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			t.Fatal(e)
		}
		if i.Mode().Perm()&0222 != 0 {
			t.Fatalf("writable %s", p)
		}
		return nil
	})
	if e := filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if i.IsDir() {
			return os.Chmod(p, 0755)
		}
		return os.Chmod(p, 0644)
	}); e != nil {
		t.Fatal(e)
	}
}
func TestEverySchemaRuntimeParityAndAdversarialMatrix(t *testing.T) {
	schemas := FreezeSchemas()
	samples, e := projectionSamples()
	if e != nil {
		t.Fatal(e)
	}
	if len(schemas) != len(persistedProjectionTypes) || len(samples) != len(schemas) {
		t.Fatalf("schemas=%d types=%d samples=%d", len(schemas), len(persistedProjectionTypes), len(samples))
	}
	for name, schema := range schemas {
		t.Run(name, func(t *testing.T) {
			var valid any
			b, _ := json.Marshal(samples[name])
			if e := json.Unmarshal(b, &valid); e != nil {
				t.Fatal(e)
			}
			if e := schemaValidateValue(schema, valid); e != nil {
				t.Fatalf("valid rejected: %v", e)
			}
			obj := valid.(map[string]any)
			mut := cloneJSONValue(obj).(map[string]any)
			mut["unknown_field"] = true
			assertSchemaRejects(t, schema, mut, "unknown")
			required := stringSlice(schema["required"])
			if len(required) == 0 {
				t.Fatal("no required fields")
			}
			mut = cloneJSONValue(obj).(map[string]any)
			delete(mut, required[0])
			assertSchemaRejects(t, schema, mut, "omitted required")
			mut = cloneJSONValue(obj).(map[string]any)
			mut[required[0]] = map[string]any{"wrong": true}
			assertSchemaRejects(t, schema, mut, "wrong type")
			for _, kind := range []string{"nested_unknown", "enum", "digest", "cardinality", "identity"} {
				candidate := cloneJSONValue(obj)
				if mutateSchemaCandidate(schema, candidate, kind) {
					assertSchemaRejects(t, schema, candidate, kind)
				}
			}
		})
	}
}
func cloneJSONValue(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
func assertSchemaRejects(t *testing.T, s map[string]any, v any, label string) {
	t.Helper()
	if e := schemaValidateValue(s, v); e == nil {
		t.Fatalf("accepted %s", label)
	}
}
func mutateSchemaCandidate(s map[string]any, v any, kind string) bool {
	if variants, ok := s["anyOf"].([]any); ok {
		for _, x := range variants {
			if mutateSchemaCandidate(x.(map[string]any), v, kind) {
				return true
			}
		}
		return false
	}
	typ, _ := s["type"].(string)
	switch typ {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return false
		}
		props, _ := s["properties"].(map[string]any)
		names := make([]string, 0, len(props))
		for n := range props {
			names = append(names, n)
		}
		sort.Strings(names)
		if kind == "nested_unknown" {
			for _, n := range names {
				child := props[n].(map[string]any)
				if child["type"] == "object" {
					if nested, ok := obj[n].(map[string]any); ok {
						nested["unknown_nested"] = true
						return true
					}
				}
				if mutateSchemaCandidate(child, obj[n], kind) {
					return true
				}
			}
			return false
		}
		for _, n := range names {
			child := props[n].(map[string]any)
			if kind == "enum" && child["enum"] != nil {
				obj[n] = "NOT_AN_ENUM"
				return true
			}
			if kind == "digest" && child["pattern"] == `^sha256:[0-9a-f]{64}$` {
				obj[n] = "bad"
				return true
			}
			if kind == "identity" {
				if max, ok := schemaInt(child["x-maxUtf8Bytes"]); ok && max == MaxIdentityBytes {
					obj[n] = strings.Repeat("x", MaxIdentityBytes+1)
					return true
				}
			}
			if kind == "cardinality" {
				if child["type"] == "array" {
					if min, ok := schemaInt(child["minItems"]); ok && min > 0 {
						obj[n] = []any{}
						return true
					}
				}
			}
			if mutateSchemaCandidate(child, obj[n], kind) {
				return true
			}
		}
	case "array":
		a, ok := v.([]any)
		if !ok {
			return false
		}
		for _, item := range a {
			if mutateSchemaCandidate(s["items"].(map[string]any), item, kind) {
				return true
			}
		}
	}
	return false
}
func TestSchemaFieldParity(t *testing.T) {
	for name, typ := range persistedProjectionTypes {
		assertTypeSchemaParity(t, name, typ, FreezeSchemas()[name])
	}
}
func assertTypeSchemaParity(t *testing.T, path string, typ reflect.Type, s map[string]any) {
	t.Helper()
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		variants := s["anyOf"].([]any)
		s = variants[0].(map[string]any)
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	props := s["properties"].(map[string]any)
	required := stringSlice(s["required"])
	want := []string{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue
		}
		n := f.Name
		if tag := f.Tag.Get("json"); tag != "" {
			if x := strings.Split(tag, ",")[0]; x != "" && x != "-" {
				n = x
			}
		}
		want = append(want, n)
		if _, ok := props[n]; !ok {
			t.Fatalf("%s missing %s", path, n)
		}
	}
	sort.Strings(want)
	sort.Strings(required)
	if !reflect.DeepEqual(want, required) || len(props) != len(want) {
		t.Fatalf("%s fields=%v required=%v properties=%d", path, want, required, len(props))
	}
}

func TestSchemaArtifactValidationInventory(t *testing.T) {
	root := generateGroupFreeze(t)
	counts, e := ValidateAllSchemaArtifacts(root)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"GroupRequest", "ProducerRaw", "ProducerAttempt", "Result", "ReviewRequest", "ReviewerRaw", "ReviewerAttempt", "Review", "Account", "DeliveryConflict", "ReviewerConflict"} {
		if counts[name] == 0 {
			t.Fatalf("%s has no generated artifact validation", name)
		}
	}
	t.Logf("SCHEMA_ARTIFACT_COUNTS %v", counts)
}

func TestAttemptBase64AndConditionalProjection(t *testing.T) {
	s := FreezeSchemas()["ProducerAttempt"]
	r := freezeRequest()
	normalRaw := freezeCanonical(ProducerRaw{Evidence: r.Evidence})
	a, _, e := NewWriter().Ingest(r, normalRaw, Digest(normalRaw), Control{})
	if e != nil || validateProjection(s, a) != nil {
		t.Fatalf("normal: %v", e)
	}
	plus := bytes.Repeat([]byte{' '}, MaxResponseBytes+1)
	terminal, _, e := NewWriter().Ingest(r, plus, Digest(plus), Control{})
	if e == nil || e.Error() != "RAW_SIZE" || len(terminal.RawBytes) != MaxResponseBytes+1 || !reflect.DeepEqual(terminal.States, []string{"RECEIVED", "TERMINAL_INVALID"}) || terminal.ResultID != "" {
		t.Fatalf("plus-one %+v %v", terminal, e)
	}
	if e = validateProjection(s, terminal); e != nil {
		t.Fatalf("allowed exact plus-one: %v", e)
	}
	for name, mutate := range map[string]func(*Attempt){"plus_two": func(x *Attempt) { x.RawBytes = make([]byte, MaxResponseBytes+2) }, "wrong_reason": func(x *Attempt) { x.TerminalReason = "UTF8" }, "wrong_states": func(x *Attempt) { x.States = []string{"RECEIVED", "ADAPTED", "COMMITTED"} }, "with_result": func(x *Attempt) { x.ResultID = "result" }} {
		t.Run(name, func(t *testing.T) {
			x := terminal
			mutate(&x)
			if validateProjection(s, x) == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, encoded := range []string{"!!!", "YQ="} {
		if VerifyRawBase64(encoded, MaxResponseBytes) == nil {
			t.Fatal("malformed base64 accepted")
		}
		x := a
		var projection map[string]any
		b, _ := json.Marshal(x)
		_ = json.Unmarshal(b, &projection)
		projection["RawBytes"] = encoded
		if schemaValidateValue(s, projection) == nil {
			t.Fatal("schema accepted malformed base64")
		}
	}
	reviewer := FreezeSchemas()["ReviewerAttempt"]
	sample, _ := projectionSamples()
	ra := sample["ReviewerAttempt"].(ReviewerAttempt)
	ra.RawBytes = make([]byte, MaxResponseBytes+1)
	if validateProjection(reviewer, ra) == nil {
		t.Fatal("reviewer plus-one accepted")
	}
}

func TestSchemaIdentityUTF8ByteBoundaries(t *testing.T) {
	s := FreezeSchemas()["GroupRequest"]
	r := freezeRequest()
	for name, id := range map[string]string{"ascii_256": strings.Repeat("x", 256), "multibyte_256": strings.Repeat("é", 128)} {
		t.Run(name, func(t *testing.T) {
			x := r
			x.RequestID = id
			if validateProjection(s, x) != nil {
				t.Fatal("boundary rejected")
			}
		})
	}
	for name, id := range map[string]string{"ascii_257": strings.Repeat("x", 257), "multibyte_over_256": strings.Repeat("é", 129)} {
		t.Run(name, func(t *testing.T) {
			x := r
			x.RequestID = id
			if validateProjection(s, x) == nil {
				t.Fatal("overflow accepted")
			}
		})
	}
}

func TestAllRuntimeParsersRejectMalformedFrames(t *testing.T) {
	r := freezeRequest()
	producer := freezeCanonical(ProducerRaw{Evidence: r.Evidence})
	reviewer := freezeCanonical(ReviewerRaw{Checks: allChecks(true), Reasons: map[string]string{}})
	parsers := map[string]func([]byte) error{"request": func(b []byte) error { _, e := ParseGroupRequest(b); return e }, "producer": func(b []byte) error { _, e := ParseProducerRaw(b); return e }, "reviewer": func(b []byte) error { _, e := ParseReviewerRaw(b); return e }}
	valid := map[string][]byte{"request": freezeCanonical(r), "producer": producer, "reviewer": reviewer}
	for name, parse := range parsers {
		if e := parse(valid[name]); e != nil {
			t.Fatal(e)
		}
		for label, b := range map[string][]byte{"duplicate": []byte(`{"x":1,"x":2}` + "\n"), "trailing": append(append([]byte{}, valid[name]...), 'x'), "invalid_utf8": {0xff, '\n'}, "unknown": []byte(`{"unknown":true}` + "\n")} {
			t.Run(name+"/"+label, func(t *testing.T) {
				if parse(b) == nil {
					t.Fatal("accepted")
				}
			})
		}
	}
}

func TestFreezeNoForbiddenDependency(t *testing.T) {
	for _, n := range []string{"freeze.go", "freeze_test.go"} {
		b, e := os.ReadFile(n)
		if e != nil {
			t.Fatal(e)
		}
		s := strings.ToLower(string(b))
		for _, bad := range []string{"pi-" + "coding", "open" + "ai", "anthro" + "pic", "model." + "invoke", "provider" + ".", "candidate" + "group", "os/" + "exec"} {
			if strings.Contains(s, bad) {
				t.Fatalf("%s contains %s", n, bad)
			}
		}
	}
}
