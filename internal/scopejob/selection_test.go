package scopejob

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSelectionAssociations(t *testing.T) {
	p, err := FreezeSelection([]string{"typescriptreact", "typescript"}, map[string]string{"*.mts": "typescript", "*.go": "typescript", "src/*.tsx": "typescriptreact"})
	if err != nil {
		t.Fatal(err)
	}
	if p.DefaultLanguage != "typescriptreact" || !reflect.DeepEqual(p.LanguageIDs, []string{"typescript", "typescriptreact"}) {
		t.Fatal("default changed by sorting", p)
	}
	for file, want := range map[string]string{"a.ts": "typescript", "nested/b.tsx": "typescriptreact", "a.go": "typescript", "a.mts": "typescript", "a.cts": "", "data.json": "", "src/b.tsx": "typescriptreact"} {
		got, e := p.Resolve(file)
		if e != nil || got != want {
			t.Fatalf("%s: %s %v want %s", file, got, e, want)
		}
	}
	p, err = FreezeSelection([]string{"typescript"}, map[string]string{"a.*": "go"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Resolve("a.ts"); !errors.Is(err, ErrAssociationConflict) {
		t.Fatalf("overlap did not reject: %v", err)
	}
	p, err = FreezeSelection([]string{"custom.lang+1"}, map[string]string{"Makefile": "custom.lang+1"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := p.Resolve("nested/Makefile"); got != "custom.lang+1" {
		t.Fatal(got)
	}
}
func TestSelectionStrictDeclaration(t *testing.T) {
	for _, ids := range [][]string{nil, {""}, {"go", "go"}, {"Go"}, {" go"}, {"go x"}, {strings.Repeat("x", 129)}} {
		if _, e := FreezeSelection(ids, nil); e == nil {
			t.Fatalf("accepted ids %q", ids)
		}
	}
	for _, pattern := range []string{"", " ", "*.go ", "[", "**/*.go", "a//b", "/a", "../a", "a\\b", "a/", strings.Repeat("x", 129)} {
		if _, e := FreezeSelection([]string{"go"}, map[string]string{pattern: "go"}); e == nil {
			t.Fatalf("accepted pattern %q", pattern)
		}
	}
	if _, e := FreezeSelection([]string{"go"}, map[string]string{"*.x": ""}); e == nil {
		t.Fatal("empty language")
	}
	many := map[string]string{}
	for i := 0; i < 65; i++ {
		many[strings.Repeat("x", i+1)] = "go"
	}
	if _, e := FreezeSelection([]string{"go"}, many); e == nil {
		t.Fatal("unbounded rules")
	}
}
func TestSelectionReplayAndFunding(t *testing.T) {
	w, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p, e := FreezeSelection([]string{"typescript", "typescriptreact"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, e := Canonical(Config{Workspace: w, Roots: []string{"."}, Profile: "ts", Language: "typescript", Selection: p, Budget: DefaultBudget()})
	if e != nil {
		t.Fatal(e)
	}
	p.Associations[0].Language = "mutated"
	if c.Selection.Associations[0].Language == "mutated" {
		t.Fatal("canonical retained caller pointer")
	}
	for _, name := range []string{"a.ts", "b.tsx", "c.go", "data.json"} {
		if e = os.WriteFile(filepath.Join(w, name), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	cur, e := OpenEnumeration(c)
	if e != nil {
		t.Fatal(e)
	}
	defer cur.Close()
	b := cur.Batch(context.Background(), 128)
	var replay EnumerationReplay
	if !b.Done || replay.Apply(context.Background(), c, b, 128) != nil {
		t.Fatal(b)
	}
	for i, r := range b.Records {
		if r.Path == "b.tsx" {
			if r.LanguageID != "typescriptreact" {
				t.Fatal(r)
			}
			b.Records[i].LanguageID = "typescript"
		}
	}
	if (&EnumerationReplay{}).Apply(context.Background(), c, b, 128) == nil {
		t.Fatal("accepted clobbered recorded language")
	}
	legacy := c
	legacy.Selection = nil
	legacy.Profile = "gopls"
	legacy.Language = "go"
	a, _ := EnumerationWork(legacy, 128)
	n, _ := EnumerationWork(c, 128)
	if n <= a {
		t.Fatal("matching work free", n, a)
	}
	c.Budget.Work = n - 1
	if _, ok := EnumerationWork(c, 128); ok {
		t.Fatal("underfunded match accepted")
	}
	raw, _ := json.Marshal(legacy)
	if strings.Contains(string(raw), "selection_policy") {
		t.Fatal("legacy bytes changed")
	}
}
func TestSelectionConflictPartial(t *testing.T) {
	w, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(w, "a.ts"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	p, e := FreezeSelection([]string{"typescript"}, map[string]string{"a.*": "go"})
	if e != nil {
		t.Fatal(e)
	}
	c, e := Canonical(Config{Workspace: w, Roots: []string{"."}, Profile: "ts", Language: "typescript", Selection: p, Budget: DefaultBudget()})
	if e != nil {
		t.Fatal(e)
	}
	cur, e := OpenEnumeration(c)
	if e != nil {
		t.Fatal(e)
	}
	defer cur.Close()
	b := cur.Batch(context.Background(), 128)
	if b.Done || b.Reason != "FILE_ASSOCIATION_CONFLICT" {
		t.Fatal(b)
	}
	if e = (&EnumerationReplay{}).Apply(context.Background(), c, b, 128); e != nil {
		t.Fatal("partial replay", e)
	}
}
