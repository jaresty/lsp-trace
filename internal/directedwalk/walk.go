// Package directedwalk computes relation-neutral graph depths only. It never
// acquires, admits, or retypes evidence and does not produce a relation ledger.
package directedwalk

import "sort"

// Edge is an oriented endpoint pair supplied by a separately admitted source.
// It has no relation or custody semantics of its own.
type Edge struct{ From, To string }

// Depths computes shortest bounded directed node depths. Reverse follows
// admitted incoming endpoint pairs; it never issues an inverse LSP request.
// Duplicate pairs do not change depth and the input slice is not mutated.
func Depths(root string, edges []Edge, maxDepth int, reverse bool) map[string]int {
	depths, _ := depthsWithWork(root, edges, maxDepth, reverse, nil)
	return depths
}

// depthsWithWork is the same traversal used by CALLS projection and the
// private mixed candidate walk. Only the latter supplies a work meter.
func depthsWithWork(root string, edges []Edge, maxDepth int, reverse bool, meter *workMeter) (map[string]int, error) {
	depths := map[string]int{root: 0}
	adjacency := map[string][]string{}
	for _, edge := range edges {
		if err := meter.charge(); err != nil {
			return nil, err
		}
		from, to := edge.From, edge.To
		if reverse {
			from, to = to, from
		}
		adjacency[from] = append(adjacency[from], to)
	}
	for from := range adjacency {
		if err := meter.charge(); err != nil {
			return nil, err
		}
		if meter == nil {
			sort.Strings(adjacency[from])
		} else if err := sortMetered(adjacency[from], func(a, b string) bool { return a < b }, meter); err != nil {
			return nil, err
		}
	}
	queue := []string{root}
	for len(queue) > 0 {
		if err := meter.charge(); err != nil {
			return nil, err
		}
		from := queue[0]
		queue = queue[1:]
		depth := depths[from]
		if depth >= maxDepth {
			continue
		}
		for _, to := range adjacency[from] {
			if err := meter.charge(); err != nil {
				return nil, err
			}
			if old, exists := depths[to]; exists && old <= depth+1 {
				continue
			}
			depths[to] = depth + 1
			queue = append(queue, to)
		}
	}
	return depths, nil
}
