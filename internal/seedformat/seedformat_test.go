package seedformat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/acquisitionops"
)

func ptr[T any](v T) *T { return &v }

func validJSON() []byte {
	return []byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","defaults":{"down_depth":2,"up_depth":1},"seeds":[{"type":"position","label":"entry","path":"src/a.go","line":3,"column":5},{"type":"symbol","label":"named","path":"src/b.go","symbol":"Run"},{"type":"slice","label":"bounded","target":{"type":"symbol","label":"inner","path":"src/c.go","symbol":"Call"},"down_depth":4,"up_depth":0}]}`)
}

func workspace(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "workspace")
}

func fullLimits() acquisitionops.Limits {
	return acquisitionops.Limits{
		MaxNodes: ptr(100), MaxRequests: ptr(1000), MaxEvidenceBytes: ptr(4 << 20),
		MaxPathWork: ptr(100000), TimeoutMS: ptr(5000), RequestTimeoutMS: ptr(1000),
		MaxResponseBytes: ptr(4 << 20), MaxMessages: ptr(64),
	}
}

func TestDecodeStrictTypedSeeds(t *testing.T) {
	w := workspace(t)
	got, err := Decode(validJSON(), w)
	if err != nil {
		t.Fatalf("ASSERT_TYPED_SEEDS_VALID: %v", err)
	}
	if len(got.Seeds) != 3 || got.Seeds[0].Type != PositionType || got.Seeds[1].Type != SymbolType || got.Seeds[2].Type != SliceType {
		t.Fatalf("ASSERT_TYPED_SEEDS_ORDER_AND_TYPES: %#v", got.Seeds)
	}
	for name, mutate := range map[string]func(map[string]any){
		"version":        func(v map[string]any) { v["schema_version"] = "lsp-trace.seeds.v1" },
		"coordinates":    func(v map[string]any) { v["coordinate_convention"] = "zero-based" },
		"unknown":        func(v map[string]any) { v["extra"] = true },
		"type-confusion": func(v map[string]any) { v["seeds"].([]any)[0].(map[string]any)["symbol"] = "Run" },
		"nested-slice": func(v map[string]any) {
			v["seeds"].([]any)[2].(map[string]any)["target"] = map[string]any{"type": "slice", "label": "nested", "target": map[string]any{"type": "symbol", "label": "inner", "path": "src/c.go", "symbol": "Call"}}
		},
		"zero-coordinate": func(v map[string]any) { v["seeds"].([]any)[0].(map[string]any)["line"] = float64(0) },
		"bad-label":       func(v map[string]any) { v["seeds"].([]any)[0].(map[string]any)["label"] = "not valid" },
		"duplicate-label": func(v map[string]any) { v["seeds"].([]any)[1].(map[string]any)["label"] = "entry" },
		"unclean-path":    func(v map[string]any) { v["seeds"].([]any)[0].(map[string]any)["path"] = "src/../a.go" },
		"escape-path":     func(v map[string]any) { v["seeds"].([]any)[0].(map[string]any)["path"] = "../a.go" },
		"depth-bound":     func(v map[string]any) { v["defaults"].(map[string]any)["down_depth"] = float64(65) },
	} {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			if err := json.Unmarshal(validJSON(), &v); err != nil {
				t.Fatal(err)
			}
			mutate(v)
			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Decode(raw, w); err == nil {
				t.Fatalf("ASSERT_TYPED_SEEDS_REJECT_%s", name)
			}
		})
	}
}

func TestDecodeRejectsDuplicateMembersAndBounds(t *testing.T) {
	w := workspace(t)
	duplicate := bytes.Replace(validJSON(), []byte(`"line":3`), []byte(`"line":3,"line":4`), 1)
	if _, err := Decode(duplicate, w); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("ASSERT_DUPLICATE_MEMBER_REJECTED: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(validJSON(), &v); err != nil {
		t.Fatal(err)
	}
	seed := v["seeds"].([]any)[0]
	seeds := make([]any, MaxSeeds+1)
	for i := range seeds {
		copy := map[string]any{}
		for k, value := range seed.(map[string]any) {
			copy[k] = value
		}
		copy["label"] = "s" + strings.Repeat("x", i) + string(rune('a'+i%26))
		seeds[i] = copy
	}
	v["seeds"] = seeds
	raw, _ := json.Marshal(v)
	if _, err := Decode(raw, w); err == nil {
		t.Fatal("ASSERT_SEED_BOUND_REJECTED")
	}
}

func TestSchemaParity(t *testing.T) {
	w := workspace(t)
	cases := [][]byte{
		validJSON(),
		append(validJSON(), []byte(`{}`)...),
		[]byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","seeds":[]}`),
		[]byte(`{"schema_version":"wrong","coordinate_convention":"one-based","seeds":[{"type":"symbol","label":"a","path":"a.go","symbol":"A"}]}`),
		[]byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","seeds":[{"type":"position","label":"a","path":"a.go","line":0,"column":1}]}`),
		[]byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","seeds":[{"type":"symbol","label":"a","path":"a.go","symbol":"A","line":1}]}`),
		[]byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","seeds":[{"type":"slice","label":"a","target":{"type":"slice","label":"b","target":{"type":"symbol","label":"c","path":"a.go","symbol":"A"}}}]}`),
	}
	for _, raw := range cases {
		_, decodeErr := Decode(raw, w)
		schemaErr := ValidateSchema(raw)
		if (decodeErr == nil) != (schemaErr == nil) {
			t.Fatalf("ASSERT_SCHEMA_RUNTIME_PARITY raw=%s decode=%v schema=%v", raw, decodeErr, schemaErr)
		}
	}
	first := SchemaJSON()
	first[0] ^= 0xff
	if bytes.Equal(first, SchemaJSON()) || !bytes.Equal(SchemaJSON(), SchemaJSON()) {
		t.Fatal("ASSERT_SCHEMA_BYTES_ISOLATED_DETERMINISTIC")
	}
}

func TestTranslateCompleteDeterministicAndCollisionSafe(t *testing.T) {
	w := workspace(t)
	raw := bytes.Replace(validJSON(), []byte(`"label":"named"`), []byte(`"label":"root"`), 1)
	file, err := Decode(raw, w)
	if err != nil {
		t.Fatal(err)
	}
	opts := TranslateOptions{Workspace: w, Limits: fullLimits(), TopmostSiblings: true}
	first, err := Translate(file, opts)
	if err != nil {
		t.Fatalf("ASSERT_TRANSLATION_VALID: %v", err)
	}
	second, err := Translate(file, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("ASSERT_TRANSLATION_DETERMINISTIC")
	}
	encoded1, _ := json.Marshal(first)
	encoded2, _ := json.Marshal(second)
	if !bytes.Equal(encoded1, encoded2) {
		t.Fatal("ASSERT_TRANSLATION_BYTES_DETERMINISTIC")
	}
	all := append([]acquisitionops.Target{first.Root}, first.RequiredTargets...)
	if len(all) != len(file.Seeds) {
		t.Fatalf("ASSERT_TRANSLATION_PRESERVES_ALL_SEEDS: %d", len(all))
	}
	wantIDs := []string{"root", "seed-root", "bounded"}
	wantDepths := [][2]int{{2, 1}, {2, 1}, {4, 0}}
	for i, target := range all {
		if target.ID != wantIDs[i] {
			t.Fatalf("ASSERT_COLLISION_SAFE_ORDER index=%d got=%q", i, target.ID)
		}
		if target.DownDepth == nil || target.UpDepth == nil || *target.DownDepth != wantDepths[i][0] || *target.UpDepth != wantDepths[i][1] {
			t.Fatalf("ASSERT_DEPTH_PRECEDENCE index=%d target=%#v", i, target)
		}
	}
	if all[0].Locator.Line == nil || all[0].Locator.Character == nil || *all[0].Locator.Line != 2 || *all[0].Locator.Character != 4 {
		t.Fatalf("ASSERT_ONE_BASED_TRANSLATED: %#v", all[0].Locator)
	}
	if all[2].Locator.Symbol != "Call" || !first.Expansion.TopmostSiblings || first.SchemaVersion != acquisitionops.ManifestVersion || first.CoordinateConvention != "zero-based-session" {
		t.Fatalf("ASSERT_TRANSLATION_MANIFEST_COMPLETE: %#v", first)
	}
}

func TestTranslateCollisionChainAndGeneratedProperties(t *testing.T) {
	w := workspace(t)
	raw := []byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","defaults":{"down_depth":0,"up_depth":64},"seeds":[{"type":"symbol","label":"first","path":"a.go","symbol":"A"},{"type":"symbol","label":"root","path":"b.go","symbol":"B"},{"type":"symbol","label":"seed-root","path":"c.go","symbol":"C"}]}`)
	file, err := Decode(raw, w)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Translate(file, TranslateOptions{Workspace: w, Limits: fullLimits()})
	if err != nil {
		t.Fatal(err)
	}
	all := append([]acquisitionops.Target{manifest.Root}, manifest.RequiredTargets...)
	if got := []string{all[0].ID, all[1].ID, all[2].ID}; !reflect.DeepEqual(got, []string{"root", "seed-root", "seed-root-2"}) {
		t.Fatalf("ASSERT_COLLISION_CHAIN_SAFE: %v", got)
	}
	for depth := 0; depth <= MaxDepth; depth++ {
		generated := []byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","defaults":{"down_depth":` + fmt.Sprint(depth) + `,"up_depth":` + fmt.Sprint(MaxDepth-depth) + `},"seeds":[{"type":"position","label":"generated","path":"dir/file.go","line":` + fmt.Sprint(depth+1) + `,"column":1}]}`)
		decoded, err := Decode(generated, w)
		if err != nil {
			t.Fatalf("ASSERT_GENERATED_DECODE depth=%d: %v", depth, err)
		}
		got, err := Translate(decoded, TranslateOptions{Workspace: w, Limits: fullLimits()})
		if err != nil {
			t.Fatalf("ASSERT_GENERATED_TRANSLATE depth=%d: %v", depth, err)
		}
		if *got.Root.DownDepth != depth || *got.Root.UpDepth != MaxDepth-depth || *got.Root.Locator.Line != uint32(depth) {
			t.Fatalf("ASSERT_GENERATED_EXACT depth=%d target=%#v", depth, got.Root)
		}
	}
}

func TestTranslateRequiresWorkspaceAndGlobalLimits(t *testing.T) {
	w := workspace(t)
	file, err := Decode(validJSON(), w)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Translate(file, TranslateOptions{Workspace: w}); err == nil {
		t.Fatal("ASSERT_GLOBAL_LIMITS_REQUIRED")
	}
	if _, err := Translate(file, TranslateOptions{Workspace: "relative", Limits: fullLimits()}); err == nil {
		t.Fatal("ASSERT_CANONICAL_WORKSPACE_REQUIRED")
	}
}
