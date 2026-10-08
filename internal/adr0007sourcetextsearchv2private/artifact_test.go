package adr0007sourcetextsearchv2private

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func TestClosedSchemasAndExamples(t *testing.T) {
	root := filepath.Join(repoRoot(t), "docs", "pilot", "adr0007", "source-text-search-v2")
	schemas, err := filepath.Glob(filepath.Join(root, "schemas", "*.schema.json"))
	if err != nil || len(schemas) < 10 {
		t.Fatalf("schemas=%d err=%v", len(schemas), err)
	}
	for _, p := range schemas {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("schema parse %s: %v", p, err)
		}
		if m["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatalf("not draft2020-12: %s", p)
		}
		if m["additionalProperties"] != false {
			t.Fatalf("not closed: %s", p)
		}
		if _, ok := m["$id"].(string); !ok {
			t.Fatalf("missing id: %s", p)
		}
	}
	examples, _ := filepath.Glob(filepath.Join(root, "examples", "*.json"))
	if len(examples) < 4 {
		t.Fatalf("examples=%d", len(examples))
	}
	for _, p := range examples {
		raw, _ := os.ReadFile(p)
		if !json.Valid(raw) {
			t.Fatalf("invalid canonical json: %s", p)
		}
		if strings.Contains(string(raw), "\n ") {
			t.Fatalf("example is not compact canonical: %s", p)
		}
	}
}

func TestCorpusHasFortyInputOnlyCasesWithExternalOracles(t *testing.T) {
	root := filepath.Join(repoRoot(t), "docs", "pilot", "adr0007", "source-text-search-v2")
	entries, err := os.ReadDir(filepath.Join(root, "corpus"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		count++
		dir := filepath.Join(root, "corpus", e.Name())
		for _, name := range []string{"request.json", "binding.json", "policy.json", "limits.json"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Fatalf("missing %s in %s", name, e.Name())
			}
		}
		if _, err := os.Stat(filepath.Join(root, "oracle", e.Name()+".expected.json")); err != nil {
			t.Fatalf("missing external oracle for %s", e.Name())
		}
	}
	if count < 40 {
		t.Fatalf("case count=%d", count)
	}
}

func TestDestructiveMutationsRejected(t *testing.T) {
	r, f := baseReq([]byte("ababa"))
	mutations := []func(Request) Request{
		func(x Request) Request { x.SchemaVersion = "bad"; return x },
		func(x Request) Request { x.Policy.AllowRegex = true; return x },
		func(x Request) Request { x.Sources[0].Path = "../x"; return x },
		func(x Request) Request { x.Sources[0].AdmissionSchema = "wrong"; return x },
		func(x Request) Request { x.Limits.MaxFiles = 0; return x },
		func(x Request) Request { x.Query = ""; return x },
	}
	for i, m := range mutations {
		x := m(r)
		if _, err := Search(x, []SourceFile{f}); err == nil {
			t.Fatalf("mutation %d accepted", i)
		}
	}
}
