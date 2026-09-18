// Package programcrepresentative mechanically qualifies provisional, seed-local
// representative nominations. It does not establish any semantic identity.
package programcrepresentative

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
)

type State string

const (
	StateSelected   State = "SELECTED"
	StateUnresolved State = "UNRESOLVED"
	StateEmpty      State = "EMPTY"
)

type Input struct {
	Admission   programcadmission.CompositeProjectionAdmission
	Communities []programc.Community
}
type Nomination struct {
	CommunityIdentity, ConstituentIdentity, ExecutionBundleID, SeedLabel, SeedAt string
	ConstituentOrdinal                                                           int
	PreparedTargets, CommunityMembers                                            []string
	SelectedNode                                                                 string
	Distance                                                                     int
	SCCMembers                                                                   []string
	State                                                                        State
}
type Unresolved struct {
	CommunityIdentity, ConstituentIdentity, ExecutionBundleID, SeedLabel, SeedAt string
	ConstituentOrdinal                                                           int
	PreparedTargets, CommunityMembers                                            []string
	State                                                                        State
}
type Output struct {
	State       State
	Nominations []Nomination
	Unresolved  []Unresolved
}

type seed struct {
	label, at, bundle string
	prepared, reached map[string]bool
	relations         map[string]bool
}

func Select(in Input) (Output, error) {
	if !in.Admission.Valid() {
		return Output{}, fmt.Errorf("validated composite admission required")
	}
	nodeIDs := in.Admission.NodeIdentities()
	occurrences := in.Admission.Occurrences()
	constituents := in.Admission.SourceBinding().Constituents
	if len(nodeIDs) == 0 && len(occurrences) == 0 && len(in.Communities) == 0 {
		return Output{State: StateEmpty}, nil
	}
	nodes := map[string]bool{}
	for _, n := range nodeIDs {
		if n == "" || nodes[n] {
			return Output{}, fmt.Errorf("invalid admitted node")
		}
		nodes[n] = true
	}
	byRelation := map[string][]programcadmission.Occurrence{}
	for _, o := range occurrences {
		if o.Identity == "" || o.RelationID == "" || o.From < 0 || o.To < 0 || int(o.From) >= len(nodeIDs) || int(o.To) >= len(nodeIDs) || o.Weight != 1 {
			return Output{}, fmt.Errorf("invalid admitted occurrence")
		}
		byRelation[o.RelationID] = append(byRelation[o.RelationID], o)
	}
	communityIDs := map[string]bool{}
	for _, community := range in.Communities {
		members := append([]string(nil), community.Members...)
		if len(members) == 0 {
			return Output{}, fmt.Errorf("empty community")
		}
		seen := map[string]bool{}
		for _, member := range members {
			if member == "" || seen[member] || !nodes[member] {
				return Output{}, fmt.Errorf("noncanonical community")
			}
			seen[member] = true
		}
		sort.Strings(members)
		id := communityIdentity(members)
		if communityIDs[id] {
			return Output{}, fmt.Errorf("duplicate community identity")
		}
		communityIDs[id] = true
	}
	out := Output{State: StateSelected}
	for ordinal, c := range constituents {
		seeds, err := decode(c, nodes, byRelation)
		if err != nil {
			return Output{}, err
		}
		for _, community := range in.Communities {
			members := append([]string(nil), community.Members...)
			seenMembers := map[string]bool{}
			for _, m := range members {
				if m == "" || seenMembers[m] {
					return Output{}, fmt.Errorf("noncanonical community members")
				}
				seenMembers[m] = true
				if !nodes[m] {
					return Output{}, fmt.Errorf("community endpoint not admitted")
				}
			}
			sort.Strings(members)
			communityID := communityIdentity(members)
			for _, s := range seeds {
				eligible := intersect(members, s.reached)
				selected, distance, scc, ok := choose(nodeIDs, byRelation, s, eligible)
				if !ok {
					out.Unresolved = append(out.Unresolved, Unresolved{CommunityIdentity: communityID, ConstituentIdentity: c.Identity, ConstituentOrdinal: ordinal, ExecutionBundleID: s.bundle, SeedLabel: s.label, SeedAt: s.at, PreparedTargets: keys(s.prepared), CommunityMembers: append([]string(nil), members...), State: StateUnresolved})
					continue
				}
				out.Nominations = append(out.Nominations, Nomination{CommunityIdentity: communityID, ConstituentIdentity: c.Identity, ConstituentOrdinal: ordinal, ExecutionBundleID: s.bundle, SeedLabel: s.label, SeedAt: s.at, PreparedTargets: keys(s.prepared), CommunityMembers: members, SelectedNode: selected, Distance: distance, SCCMembers: scc, State: StateSelected})
			}
		}
	}
	if len(in.Communities) == 0 {
		out.State = StateEmpty
	} else if len(out.Nominations) == 0 {
		out.State = StateUnresolved
	}
	sort.Slice(out.Nominations, func(i, j int) bool {
		a, b := out.Nominations[i], out.Nominations[j]
		if a.CommunityIdentity != b.CommunityIdentity {
			return a.CommunityIdentity < b.CommunityIdentity
		}
		if a.ConstituentOrdinal != b.ConstituentOrdinal {
			return a.ConstituentOrdinal < b.ConstituentOrdinal
		}
		if a.ConstituentIdentity != b.ConstituentIdentity {
			return a.ConstituentIdentity < b.ConstituentIdentity
		}
		if a.ExecutionBundleID != b.ExecutionBundleID {
			return a.ExecutionBundleID < b.ExecutionBundleID
		}
		if a.SeedAt != b.SeedAt {
			return a.SeedAt < b.SeedAt
		}
		if a.SeedLabel != b.SeedLabel {
			return a.SeedLabel < b.SeedLabel
		}
		return a.SelectedNode < b.SelectedNode
	})
	sort.Slice(out.Unresolved, func(i, j int) bool {
		a, b := out.Unresolved[i], out.Unresolved[j]
		if a.CommunityIdentity != b.CommunityIdentity {
			return a.CommunityIdentity < b.CommunityIdentity
		}
		if a.ConstituentOrdinal != b.ConstituentOrdinal {
			return a.ConstituentOrdinal < b.ConstituentOrdinal
		}
		if a.ConstituentIdentity != b.ConstituentIdentity {
			return a.ConstituentIdentity < b.ConstituentIdentity
		}
		if a.ExecutionBundleID != b.ExecutionBundleID {
			return a.ExecutionBundleID < b.ExecutionBundleID
		}
		if a.SeedAt != b.SeedAt {
			return a.SeedAt < b.SeedAt
		}
		if a.SeedLabel != b.SeedLabel {
			return a.SeedLabel < b.SeedLabel
		}
		return compare(a.PreparedTargets, b.PreparedTargets) < 0
	})
	return out, nil
}
func decode(c programcadmission.ConstituentReference, nodes map[string]bool, relations map[string][]programcadmission.Occurrence) ([]*seed, error) {
	var ms []graph.SeedMembership
	d := json.NewDecoder(bytesReader(c.SeedMemberships))
	d.DisallowUnknownFields()
	if err := d.Decode(&ms); err != nil {
		return nil, fmt.Errorf("malformed seed memberships: %w", err)
	}
	if err := requireEOF(d); err != nil {
		return nil, fmt.Errorf("malformed seed memberships: %w", err)
	}
	if len(ms) == 0 {
		return nil, nil
	}
	bundles := map[string]bool{}
	by := map[string]*seed{}
	ids := map[string]graph.SeedMembership{}
	for _, m := range ms {
		if m.MembershipID == "" || m.ExecutionBundleID == "" || m.SeedLabel == "" || m.SeedAt == "" || m.EndpointID == "" {
			return nil, fmt.Errorf("incomplete seed membership")
		}
		bundles[m.ExecutionBundleID] = true
		if prior, ok := ids[m.MembershipID]; ok {
			if prior != m {
				return nil, fmt.Errorf("conflicting seed membership identity")
			}
			return nil, fmt.Errorf("duplicate seed membership identity")
		}
		ids[m.MembershipID] = m
		key := m.SeedLabel + "\x00" + m.SeedAt
		s := by[key]
		if s == nil {
			s = &seed{m.SeedLabel, m.SeedAt, m.ExecutionBundleID, map[string]bool{}, map[string]bool{}, map[string]bool{}}
			by[key] = s
		}
		switch m.EvidenceKind {
		case "PREPARED_TARGET":
			if !nodes[m.EndpointID] {
				return nil, fmt.Errorf("prepared endpoint not admitted")
			}
			if s.prepared[m.EndpointID] {
				return nil, fmt.Errorf("duplicate prepared membership")
			}
			s.prepared[m.EndpointID] = true
		case "REACHED_NODE":
			if !nodes[m.EndpointID] {
				return nil, fmt.Errorf("reached endpoint not admitted")
			}
			if s.reached[m.EndpointID] {
				return nil, fmt.Errorf("duplicate reached membership")
			}
			s.reached[m.EndpointID] = true
		case "CALL_RELATION":
			if len(relations[m.EndpointID]) == 0 {
				return nil, fmt.Errorf("relation endpoint not admitted")
			}
			if s.relations[m.EndpointID] {
				return nil, fmt.Errorf("duplicate relation membership")
			}
			s.relations[m.EndpointID] = true
		default:
			return nil, fmt.Errorf("unknown seed membership kind %q", m.EvidenceKind)
		}
	}
	if len(bundles) != 1 {
		return nil, fmt.Errorf("conflicting execution bundle membership")
	}
	out := make([]*seed, 0, len(by))
	for _, s := range by {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].label != out[j].label {
			return out[i].label < out[j].label
		}
		return out[i].at < out[j].at
	})
	return out, nil
}
func choose(nodes []string, rel map[string][]programcadmission.Occurrence, s *seed, eligible []string) (string, int, []string, bool) {
	// Only relation-membered occurrences whose endpoints are reached/prepared are admitted.
	adj := map[string][]string{}
	active := map[string]bool{}
	for n := range s.prepared {
		active[n] = true
	}
	for n := range s.reached {
		active[n] = true
	}
	for id := range s.relations {
		for _, o := range rel[id] {
			a, b := nodes[o.From], nodes[o.To]
			if active[a] && active[b] {
				adj[a] = append(adj[a], b)
			}
		}
	}
	for n := range adj {
		sort.Strings(adj[n])
	}
	comp, groups := scc(active, adj)
	roots := map[int]bool{}
	for n := range s.prepared {
		roots[comp[n]] = true
	}
	if len(roots) == 0 {
		return "", 0, nil, false
	}
	cadj := map[int]map[int]bool{}
	for a, bs := range adj {
		for _, b := range bs {
			x, y := comp[a], comp[b]
			if x != y {
				if cadj[x] == nil {
					cadj[x] = map[int]bool{}
				}
				cadj[x][y] = true
			}
		}
	}
	dist := map[int]int{}
	q := []int{}
	for x := range roots {
		dist[x] = 0
		q = append(q, x)
	}
	sort.Ints(q)
	for len(q) > 0 {
		x := q[0]
		q = q[1:]
		next := []int{}
		for y := range cadj[x] {
			next = append(next, y)
		}
		sort.Ints(next)
		for _, y := range next {
			if _, ok := dist[y]; !ok {
				dist[y] = dist[x] + 1
				q = append(q, y)
			}
		}
	}
	best := ""
	bestD := int(^uint(0) >> 1)
	for _, n := range eligible {
		d, ok := dist[comp[n]]
		if !ok {
			continue
		}
		if d < bestD || (d == bestD && (best == "" || n < best)) {
			best, bestD = n, d
		}
	}
	if best == "" {
		return "", 0, nil, false
	}
	return best, bestD, append([]string(nil), groups[comp[best]]...), true
}
func scc(active map[string]bool, adj map[string][]string) (map[string]int, map[int][]string) {
	idx, low := map[string]int{}, map[string]int{}
	on := map[string]bool{}
	stack := []string{}
	comp := map[string]int{}
	groups := map[int][]string{}
	next, cid := 0, 0
	var visit func(string)
	visit = func(v string) {
		idx[v] = next
		low[v] = next
		next++
		stack = append(stack, v)
		on[v] = true
		for _, w := range adj[v] {
			if _, ok := idx[w]; !ok {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if on[w] && idx[w] < low[v] {
				low[v] = idx[w]
			}
		}
		if low[v] == idx[v] {
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				comp[w] = cid
				groups[cid] = append(groups[cid], w)
				if w == v {
					break
				}
			}
			sort.Strings(groups[cid])
			cid++
		}
	}
	ns := keys(active)
	for _, n := range ns {
		if _, ok := idx[n]; !ok {
			visit(n)
		}
	}
	return comp, groups
}
func keys(m map[string]bool) []string {
	o := make([]string, 0, len(m))
	for k := range m {
		o = append(o, k)
	}
	sort.Strings(o)
	return o
}
func intersect(xs []string, m map[string]bool) []string {
	o := []string{}
	for _, x := range xs {
		if m[x] {
			o = append(o, x)
		}
	}
	return o
}
func uniqueSorted(in []string) []string {
	m := map[string]bool{}
	for _, x := range in {
		m[x] = true
	}
	return keys(m)
}
func communityIdentity(members []string) string { b, _ := json.Marshal(members); return string(b) }
func compare(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

type rawReader struct {
	b []byte
	i int
}

func requireEOF(d *json.Decoder) error {
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func bytesReader(b []byte) *rawReader { return &rawReader{b: b} }
func (r *rawReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
