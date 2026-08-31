package dispatch

import (
	"context"
	"fmt"
	"sort"

	"lsp-trace/internal/lsp"
)

type Client interface {
	SupportsTypeHierarchy() bool
	PrepareTypeHierarchy(context.Context, lsp.PrepareTypeHierarchyParams) ([]lsp.TypeHierarchyItem, error)
	Subtypes(context.Context, lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error)
}

type Association struct {
	Interface      lsp.TypeHierarchyItem `json:"interface"`
	Implementation lsp.TypeHierarchyItem `json:"implementation"`
}

type Failure struct {
	Item    lsp.TypeHierarchyItem `json:"item"`
	Message string                `json:"message"`
}

type Family struct {
	Root         *lsp.TypeHierarchyItem  `json:"root,omitempty"`
	Members      []lsp.TypeHierarchyItem `json:"members,omitempty"`
	Associations []Association           `json:"associations,omitempty"`
	Failures     []Failure               `json:"failures,omitempty"`
}

func Resolve(ctx context.Context, client Client, params lsp.PrepareTypeHierarchyParams) Family {
	if !client.SupportsTypeHierarchy() {
		return Family{}
	}
	prepared, err := client.PrepareTypeHierarchy(ctx, params)
	if err != nil {
		return Family{Failures: []Failure{{Message: err.Error()}}}
	}
	if len(prepared) == 0 {
		return Family{}
	}
	root := prepared[0]
	out := Family{Root: &root}
	seen := map[string]bool{itemKey(root): true}
	members := map[string]lsp.TypeHierarchyItem{}
	queue := []lsp.TypeHierarchyItem{root}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		children, err := client.Subtypes(ctx, item)
		if err != nil {
			out.Failures = append(out.Failures, Failure{Item: item, Message: err.Error()})
			continue
		}
		sortItems(children)
		for _, child := range children {
			key := itemKey(child)
			if key == itemKey(root) || seen[key] {
				continue
			}
			seen[key] = true
			members[key] = child
			queue = append(queue, child)
		}
	}
	for _, member := range members {
		out.Members = append(out.Members, member)
	}
	sortItems(out.Members)
	for _, member := range out.Members {
		out.Associations = append(out.Associations, Association{Interface: root, Implementation: member})
	}
	sort.Slice(out.Failures, func(i, j int) bool {
		if a, b := itemKey(out.Failures[i].Item), itemKey(out.Failures[j].Item); a != b {
			return a < b
		}
		return out.Failures[i].Message < out.Failures[j].Message
	})
	return out
}

func itemKey(item lsp.TypeHierarchyItem) string {
	return fmt.Sprintf("%s\x00%d:%d-%d:%d", item.URI, item.SelectionRange.Start.Line, item.SelectionRange.Start.Character, item.SelectionRange.End.Line, item.SelectionRange.End.Character)
}

func sortItems(items []lsp.TypeHierarchyItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if a, b := itemKey(items[i]), itemKey(items[j]); a != b {
			return a < b
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Detail < items[j].Detail
	})
}
