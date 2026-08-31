package dispatch

import (
	"context"
	"errors"
	"testing"

	"lsp-trace/internal/lsp"
)

type fakeClient struct {
	supported bool
	prepared  []lsp.TypeHierarchyItem
	subtypes  map[string][]lsp.TypeHierarchyItem
	errors    map[string]error
	seen      []lsp.TypeHierarchyItem
}

func (f *fakeClient) SupportsTypeHierarchy() bool { return f.supported }
func (f *fakeClient) PrepareTypeHierarchy(context.Context, lsp.PrepareTypeHierarchyParams) ([]lsp.TypeHierarchyItem, error) {
	return f.prepared, nil
}
func (f *fakeClient) Subtypes(_ context.Context, item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error) {
	f.seen = append(f.seen, item)
	return f.subtypes[item.Name], f.errors[item.Name]
}

func item(name, uri string, line uint32) lsp.TypeHierarchyItem {
	r := lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 1}}
	return lsp.TypeHierarchyItem{Name: name, URI: uri, Range: r, SelectionRange: r}
}

func TestResolveNormalizesRecursiveFamily(t *testing.T) {
	root, a, b, duplicate := item("I", "file:///i", 0), item("B", "file:///b", 2), item("A", "file:///a", 1), item("A-copy", "file:///a", 1)
	f := &fakeClient{supported: true, prepared: []lsp.TypeHierarchyItem{root}, subtypes: map[string][]lsp.TypeHierarchyItem{"I": {a, b, duplicate}, "A": {}, "B": {}}, errors: map[string]error{}}
	got := Resolve(context.Background(), f, lsp.PrepareTypeHierarchyParams{})
	if got.Root == nil || len(got.Members) != 2 || got.Members[0].URI != "file:///a" || len(got.Associations) != 2 {
		t.Fatalf("ASSERT_FAMILY_DETERMINISTIC_UNIQUE: %#v", got)
	}
	if len(f.seen) == 0 || f.seen[0].Name != "I" {
		t.Fatalf("ASSERT_EXACT_ITEM_FORWARDED: %#v", f.seen)
	}
}

func TestResolveRetainsSuccessOnBranchFailure(t *testing.T) {
	root, a, b := item("I", "file:///i", 0), item("A", "file:///a", 1), item("B", "file:///b", 2)
	f := &fakeClient{supported: true, prepared: []lsp.TypeHierarchyItem{root}, subtypes: map[string][]lsp.TypeHierarchyItem{"I": {a, b}, "A": {}, "B": {}}, errors: map[string]error{"A": errors.New("branch failed")}}
	got := Resolve(context.Background(), f, lsp.PrepareTypeHierarchyParams{})
	if len(got.Members) != 2 || len(got.Failures) != 1 || got.Failures[0].Item.Name != "A" {
		t.Fatalf("ASSERT_PARTIAL_FAILURE_RETAINED: %#v", got)
	}
}

func TestResolveUnsupportedIsEmpty(t *testing.T) {
	got := Resolve(context.Background(), &fakeClient{}, lsp.PrepareTypeHierarchyParams{})
	if got.Root != nil || len(got.Members) != 0 {
		t.Fatalf("ASSERT_UNSUPPORTED_EMPTY: %#v", got)
	}
}
