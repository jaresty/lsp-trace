//go:build darwin || linux

package applicationinventory

import (
	"context"
	"lsp-trace/internal/scopejob"
	"os"
	"path/filepath"
	"testing"
)

func TestInventorySelectionFrozenPolicy(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 0)
	p, e := scopejob.FreezeSelection([]string{"typescriptreact", "typescript"}, map[string]string{"*.mts": "typescript"})
	if e != nil {
		t.Fatal(e)
	}
	c.Enumeration.Profile = "ts"
	c.Enumeration.Language = p.DefaultLanguage
	c.Enumeration.Selection = p
	c.ResourcePolicy = selectionResourcePolicy
	for _, n := range []string{"a.ts", "b.tsx", "c.mts", "d.go", "x.json"} {
		if e = os.WriteFile(filepath.Join(c.Scope, n), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	ctx := context.Background()
	j, e := s.Create(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	p.LanguageIDs[0] = "go"
	p.Associations = nil
	p.DefaultLanguage = "go"
	if e = os.WriteFile(filepath.Join(c.Scope, ".lsp-trace.toml"), []byte("malformed changed config"), 0600); e != nil {
		t.Fatal(e)
	}
	got, e := s.Enumerate(ctx, j.RunID, j.Head)
	if e != nil {
		t.Fatal(got, e)
	}
	if !got.Enumeration.Closed || got.Enumeration.Counts.Selected != 3 || got.Enumeration.Counts.UnsupportedLanguage != 1 || got.Enumeration.AcquisitionImplemented {
		t.Fatal(got)
	}
	h, _, e := s.head(j.RunID)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := s.enumerationConfig(ctx, h)
	if e != nil || frozen.Enumeration.Selection.DefaultLanguage != "typescriptreact" || len(frozen.Enumeration.Selection.Associations) != 11 {
		t.Fatal(frozen, e)
	}
	if _, e = s.JobStatus(ctx, j.RunID); e != nil {
		t.Fatal(e)
	}
}
func TestInventorySelectionPaidConflict(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 0)
	p, e := scopejob.FreezeSelection([]string{"typescript"}, map[string]string{"a.*": "go"})
	if e != nil {
		t.Fatal(e)
	}
	c.Enumeration.Profile = "ts"
	c.Enumeration.Language = "typescript"
	c.Enumeration.Selection = p
	c.ResourcePolicy = selectionResourcePolicy
	if e = os.WriteFile(filepath.Join(c.Scope, "a.ts"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	created, got := enumerationRun(t, s, c)
	if got.Phase != "EnumerationPartial" || got.Enumeration.Reason != "FILE_ASSOCIATION_CONFLICT" || got.Enumeration.Closed || got.Used <= created.Used {
		t.Fatal(got)
	}
	if _, e = s.JobStatus(context.Background(), got.RunID); e != nil {
		t.Fatal(e)
	}
}
func TestInventorySelectionUnderfundedBeforeEffects(t *testing.T) {
	s := ownedTestStore(t)
	c := enumerationTestConfig(t, 0)
	p, e := scopejob.FreezeSelection([]string{"go"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	c.Enumeration.Selection = p
	c.ResourcePolicy = selectionResourcePolicy
	c.Enumeration.Budget.Work = 1
	if _, e = s.Create(context.Background(), c); e == nil {
		t.Fatal("unfunded policy admitted")
	}
	names, e := os.ReadDir(s.Directory)
	if e != nil || len(names) != 0 {
		t.Fatal(names, e)
	}
}
