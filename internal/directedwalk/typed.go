package directedwalk

import "errors"

// Kind is an exact candidate relation selector, not an admission verdict.
type Kind string

const (
	Calls                Kind = "CALLS"
	ReferencesSymbol     Kind = "REFERENCES_SYMBOL"
	ResolvesToDefinition Kind = "RESOLVES_TO_DEFINITION"
)

type Direction string

const (
	Down Direction = "DOWN"
	Up   Direction = "UP"
)

type State string

const (
	Witnessed       State = "WITNESSED"
	NodeBound       State = "NODE_BOUND"
	OutsideFrontier State = "OUTSIDE_FRONTIER"
)

// Candidate is caller-supplied and unverified; it carries no custody receipt.
type Candidate struct {
	ID       string
	Kind     Kind
	From, To string
}

// These are provisional private ceilings, not public contracts.
const (
	MaxCandidateLimit = 10000
	MaxWorkLimit      = 1000000
)

var (
	ErrCandidateLimit        = errors.New("candidate limit exceeded")
	ErrInvalidCandidateLimit = errors.New("invalid candidate limit")
	ErrWorkLimit             = errors.New("walk work limit exceeded")
	ErrInvalidWorkLimit      = errors.New("invalid walk work limit")
)

type Request struct {
	Root               string
	Kinds              []Kind // explicit; nil does not select CALLS
	UpDepth, DownDepth int
	MaxNodes           int
	MaxCandidates      int // required, including unselected candidates
	MaxWork            int // required; private logical graph-operation units
	Candidates         []Candidate
}

type Node struct {
	ID                 string
	DownDepth, UpDepth *int
}

type Step struct {
	Direction Direction
	Depth     int
}

type Disposition struct {
	Candidate      Candidate
	State          State
	Witnesses      []Step
	NodeBoundSteps []Step
}

type KindCounts struct {
	Kind                                                   Kind
	Total, Witnessed, NodeBound, OutsideFrontier           int
	DownWitnesses, UpWitnesses, DownNodeBound, UpNodeBound int
}

type Result struct {
	Nodes        []Node
	Occurrences  []Disposition
	Counts       []KindCounts
	Unselected   int
	NodesOmitted int
	WorkUsed     int
}

// Walk projects caller-supplied candidate occurrences onto one shared node
// budget, preflight candidate-input limit, and logical graph-operation meter.
// It is pure graph arithmetic, NOT evidence admission or a bound on CPU time,
// input bytes, or upstream acquisition; neither input nor output has custody.
// Only explicit selected kinds participate. No inverse method is issued.
func Walk(req Request) (Result, error) {
	// Cardinality is checked before candidate contents or graph allocations.
	// Include unselected kinds so selection cannot bypass the input bound.
	if req.MaxCandidates < 1 || req.MaxCandidates > MaxCandidateLimit {
		return Result{}, ErrInvalidCandidateLimit
	}
	if len(req.Candidates) > req.MaxCandidates {
		return Result{}, ErrCandidateLimit
	}
	if req.MaxWork < 1 || req.MaxWork > MaxWorkLimit {
		return Result{}, ErrInvalidWorkLimit
	}
	if req.Root == "" || req.UpDepth < 0 || req.DownDepth < 0 || req.MaxNodes < 1 || len(req.Kinds) == 0 {
		return Result{}, errors.New("invalid candidate walk request")
	}
	meter := &workMeter{limit: req.MaxWork}
	selected := make(map[Kind]bool, len(req.Kinds))
	for _, kind := range req.Kinds {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		if !validKind(kind) || selected[kind] {
			return Result{}, errors.New("invalid candidate kind selection")
		}
		selected[kind] = true
	}
	seen := make(map[string]bool, len(req.Candidates))
	candidates := make([]Candidate, 0, len(req.Candidates))
	for _, candidate := range req.Candidates {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		if candidate.ID == "" || candidate.From == "" || candidate.To == "" || !validKind(candidate.Kind) || seen[candidate.ID] {
			return Result{}, errors.New("invalid candidate occurrence")
		}
		seen[candidate.ID] = true
		if selected[candidate.Kind] {
			candidates = append(candidates, candidate)
		}
	}
	// Sorting both inputs and outputs prevents caller order from influencing the
	// tie-break or assigning a different disposition to parallel occurrences.
	if err := sortMetered(candidates, func(a, b Candidate) bool { return a.ID < b.ID }, meter); err != nil {
		return Result{}, err
	}
	pairs := make([]Edge, len(candidates))
	for i, candidate := range candidates {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		pairs[i] = Edge{From: candidate.From, To: candidate.To}
	}
	fullDown, err := depthsWithWork(req.Root, pairs, req.DownDepth, false, meter)
	if err != nil {
		return Result{}, err
	}
	fullUp, err := depthsWithWork(req.Root, pairs, req.UpDepth, true, meter)
	if err != nil {
		return Result{}, err
	}
	ids := make([]string, 0, len(fullDown)+len(fullUp))
	minimum := make(map[string]int, len(fullDown)+len(fullUp))
	for id, depth := range fullDown {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		ids = append(ids, id)
		minimum[id] = depth
	}
	for id, depth := range fullUp {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		if previous, exists := minimum[id]; !exists {
			ids = append(ids, id)
			minimum[id] = depth
		} else if depth < previous {
			minimum[id] = depth
		}
	}
	if err := sortMetered(ids, func(a, b string) bool {
		if minimum[a] != minimum[b] {
			return minimum[a] < minimum[b]
		}
		return a < b
	}, meter); err != nil {
		return Result{}, err
	}
	if req.MaxNodes < len(ids) {
		ids = ids[:req.MaxNodes]
	}
	retained := make(map[string]bool, len(ids))
	for _, id := range ids {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		retained[id] = true
	}
	retainedPairs := make([]Edge, 0, len(pairs))
	for _, pair := range pairs {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		if retained[pair.From] && retained[pair.To] {
			retainedPairs = append(retainedPairs, pair)
		}
	}
	// Recompute on the induced subgraph: a selected node must not carry a
	// directional depth whose only path passes through an omitted node.
	down, err := depthsWithWork(req.Root, retainedPairs, req.DownDepth, false, meter)
	if err != nil {
		return Result{}, err
	}
	up, err := depthsWithWork(req.Root, retainedPairs, req.UpDepth, true, meter)
	if err != nil {
		return Result{}, err
	}
	result := Result{Nodes: make([]Node, 0, len(ids)), Occurrences: make([]Disposition, 0, len(candidates)),
		Unselected: len(req.Candidates) - len(candidates), NodesOmitted: len(minimum) - len(ids)}
	for _, id := range ids {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		node := Node{ID: id}
		if depth, ok := down[id]; ok {
			value := depth
			node.DownDepth = &value
		}
		if depth, ok := up[id]; ok {
			value := depth
			node.UpDepth = &value
		}
		result.Nodes = append(result.Nodes, node)
	}
	counts := make(map[Kind]*KindCounts, len(selected))
	for kind := range selected {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		counts[kind] = &KindCounts{Kind: kind}
	}
	for _, candidate := range candidates {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		entry := Disposition{Candidate: candidate}
		count := counts[candidate.Kind]
		count.Total++
		for _, direction := range []Direction{Down, Up} {
			if err := meter.charge(); err != nil {
				return Result{}, err
			}
			from, to, limit, full, admitted := candidate.From, candidate.To, req.DownDepth, fullDown, down
			if direction == Up {
				from, to, limit, full, admitted = candidate.To, candidate.From, req.UpDepth, fullUp, up
			}
			fullDepth, fullReached := full[from]
			if !fullReached || fullDepth >= limit {
				continue
			}
			if depth, reached := admitted[from]; reached && depth < limit && retained[to] {
				entry.Witnesses = append(entry.Witnesses, Step{Direction: direction, Depth: depth + 1})
				if direction == Down {
					count.DownWitnesses++
				} else {
					count.UpWitnesses++
				}
			} else {
				entry.NodeBoundSteps = append(entry.NodeBoundSteps, Step{Direction: direction, Depth: fullDepth + 1})
				if direction == Down {
					count.DownNodeBound++
				} else {
					count.UpNodeBound++
				}
			}
		}
		switch {
		case len(entry.Witnesses) != 0:
			entry.State = Witnessed
			count.Witnessed++
		case len(entry.NodeBoundSteps) != 0:
			entry.State = NodeBound
			count.NodeBound++
		default:
			entry.State = OutsideFrontier
			count.OutsideFrontier++
		}
		result.Occurrences = append(result.Occurrences, entry)
	}
	for kind := range counts {
		if err := meter.charge(); err != nil {
			return Result{}, err
		}
		result.Counts = append(result.Counts, *counts[kind])
	}
	if err := sortMetered(result.Counts, func(a, b KindCounts) bool { return a.Kind < b.Kind }, meter); err != nil {
		return Result{}, err
	}
	result.WorkUsed = meter.used
	return result, nil
}

func validKind(kind Kind) bool {
	return kind == Calls || kind == ReferencesSymbol || kind == ResolvesToDefinition
}
