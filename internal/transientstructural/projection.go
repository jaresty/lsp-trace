package transientstructural

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"lsp-trace/internal/graph"
)

type traversalProjection struct {
	nodes []graph.Node
	edges []graph.Edge
	root  string
}

type admittedProjection struct {
	targetID    string
	nodes       []NodeFact
	occurrences []OccurrenceFact
	accounting  Accounting
}

func project(sessionID string, generation uint64, down, up traversalProjection, bounds BoundsBinding, accounting Accounting) (admittedProjection, error) {
	if down.root == "" || up.root == "" || down.root != up.root {
		return admittedProjection{}, errors.New("invalid target root")
	}
	rawNodes := append(append([]graph.Node(nil), down.nodes...), up.nodes...)
	accounting.Nodes.Observed = len(rawNodes)
	downDepths := directedDepths(down.root, down.edges, bounds.DownDepth, false)
	upDepths := directedDepths(up.root, up.edges, bounds.UpDepth, true)
	reachable := make(map[string]bool, len(downDepths)+len(upDepths))
	for rawID := range downDepths {
		reachable[rawID] = true
	}
	for rawID := range upDepths {
		reachable[rawID] = true
	}

	byID := make(map[string]graph.Node, len(rawNodes))
	duplicateNodes := 0
	for _, node := range rawNodes {
		if node.ID == "" || graph.ValidateItem(node.Item) != nil {
			accounting.Nodes.Rejected++
			continue
		}
		if prior, ok := byID[node.ID]; ok {
			if !graph.SameNodeIdentity(prior, node) {
				accounting.Nodes.Rejected++
				continue
			}
			duplicateNodes++
			continue
		}
		byID[node.ID] = node
	}
	if duplicateNodes > 0 {
		accounting.Nodes.Omitted += duplicateNodes
		addOmission(&accounting, OmissionDuplicate, duplicateNodes)
	}
	unreachableNodes := 0
	for rawID := range byID {
		if !reachable[rawID] {
			delete(byID, rawID)
			unreachableNodes++
		}
	}
	if unreachableNodes > 0 {
		accounting.Nodes.Omitted += unreachableNodes
		addOmission(&accounting, OmissionInvalidResponse, unreachableNodes)
	}
	accounting.Nodes.Admitted = len(byID)

	salt := identitySalt(sessionID, generation)
	opaque := make(map[string]string, len(byID))
	nodeFacts := make([]NodeFact, 0, len(byID))
	for rawID := range byID {
		opaque[rawID] = opaqueID("lsp-trace/transient-structural/node/v1", salt, rawID)
	}
	for rawID := range byID {
		node := byID[rawID]
		nodeFacts = append(nodeFacts, NodeFact{
			ID: opaque[rawID], Witnesses: nodeWitnesses(rawID, down.root, downDepths, upDepths),
			Name: node.Name, Kind: node.Kind, URI: node.URI,
			Range: node.Range, ItemRange: node.Range, SelectionRange: node.SelectionRange,
		})
	}
	sort.Slice(nodeFacts, func(i, j int) bool { return nodeFacts[i].ID < nodeFacts[j].ID })
	partial := admittedProjection{targetID: opaque[down.root], nodes: nodeFacts, accounting: accounting}
	if accounting.Nodes.Rejected != 0 || byID[down.root].ID == "" || unreachableNodes != 0 {
		return partial, errors.New("invalid node response")
	}
	if len(byID) > bounds.MaxNodes {
		over := len(byID) - bounds.MaxNodes
		accounting.Nodes.Admitted -= over
		accounting.Nodes.Omitted += over
		addOmission(&accounting, OmissionNodeBound, over)
		partial.accounting = accounting
		return partial, errors.New("node bound exceeded")
	}

	type occurrence struct {
		fact OccurrenceFact
		raw  string
	}
	occurrences := map[string]occurrence{}
	observeEdges := func(edges []graph.Edge, depths map[string]int, reverse bool, direction Direction, limit int) {
		for _, edge := range edges {
			calls := edge.CallSites
			if len(calls) == 0 {
				calls = []graph.Range{{}}
			}
			for _, site := range calls {
				accounting.Occurrences.Observed++
				from, to := edge.CallerNodeID, edge.CalleeNodeID
				walkFrom, walkTo := from, to
				if reverse {
					walkFrom, walkTo = to, from
				}
				fromDepth, ok := depths[walkFrom]
				if !ok || fromDepth >= limit {
					accounting.Occurrences.Omitted++
					addOmission(&accounting, OmissionDepthBound, 1)
					continue
				}
				toDepth, ok := depths[walkTo]
				if !ok || (walkFrom != walkTo && toDepth > fromDepth+1) {
					accounting.Occurrences.Rejected++
					continue
				}
				if opaque[from] == "" || opaque[to] == "" {
					accounting.Occurrences.Rejected++
					continue
				}
				rawKeyBytes, _ := json.Marshal(site)
				rawKey := string(rawKeyBytes)
				key := "CALLS\x00SERVER_REPORTED\x00" + edge.CallerNodeID + "\x00" + edge.CalleeNodeID + "\x00" + rawKey
				witness := Witness{Direction: direction, Depth: fromDepth + 1}
				if existing, found := occurrences[key]; found {
					existing.fact.Witnesses = appendWitness(existing.fact.Witnesses, witness)
					occurrences[key] = existing
					accounting.Occurrences.Omitted++
					addOmission(&accounting, OmissionDuplicate, 1)
					continue
				}
				occurrences[key] = occurrence{raw: rawKey, fact: OccurrenceFact{
					ID: opaqueID("lsp-trace/transient-structural/occurrence/v1", salt, key), CallerID: opaque[from], CalleeID: opaque[to], Witnesses: []Witness{witness}, URI: byID[from].URI, Range: site,
				}}
			}
		}
	}
	observeEdges(down.edges, downDepths, false, DirectionOutgoing, bounds.DownDepth)
	observeEdges(up.edges, upDepths, true, DirectionIncoming, bounds.UpDepth)
	facts := make([]OccurrenceFact, 0, len(occurrences))
	for _, item := range occurrences {
		item.fact.Witnesses = canonicalWitnesses(item.fact.Witnesses)
		facts = append(facts, item.fact)
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].ID < facts[j].ID })
	accounting.Occurrences.Admitted = len(facts)
	if accounting.Occurrences.Rejected != 0 {
		return admittedProjection{accounting: accounting}, errors.New("invalid occurrence response")
	}
	return admittedProjection{targetID: opaque[down.root], nodes: nodeFacts, occurrences: facts, accounting: accounting}, nil
}

func directedDepths(root string, edges []graph.Edge, maxDepth int, reverse bool) map[string]int {
	depths := map[string]int{root: 0}
	adjacency := map[string][]string{}
	for _, edge := range edges {
		from, to := edge.CallerNodeID, edge.CalleeNodeID
		if reverse {
			from, to = to, from
		}
		adjacency[from] = append(adjacency[from], to)
	}
	for from := range adjacency {
		sort.Strings(adjacency[from])
	}
	queue := []string{root}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		depth := depths[from]
		if depth >= maxDepth {
			continue
		}
		for _, to := range adjacency[from] {
			if old, exists := depths[to]; exists && old <= depth+1 {
				continue
			}
			depths[to] = depth + 1
			queue = append(queue, to)
		}
	}
	return depths
}

func nodeWitnesses(rawID, root string, down, up map[string]int) []Witness {
	if rawID == root {
		return []Witness{{Direction: DirectionRoot, Depth: 0}}
	}
	var out []Witness
	if depth, ok := up[rawID]; ok {
		out = append(out, Witness{Direction: DirectionIncoming, Depth: depth})
	}
	if depth, ok := down[rawID]; ok {
		out = append(out, Witness{Direction: DirectionOutgoing, Depth: depth})
	}
	return canonicalWitnesses(out)
}

func appendWitness(in []Witness, witness Witness) []Witness {
	for i, existing := range in {
		if existing.Direction == witness.Direction {
			if witness.Depth < existing.Depth {
				in[i] = witness
			}
			return in
		}
	}
	return append(in, witness)
}

func canonicalWitnesses(in []Witness) []Witness {
	sort.Slice(in, func(i, j int) bool {
		if in[i].Direction != in[j].Direction {
			return in[i].Direction < in[j].Direction
		}
		return in[i].Depth < in[j].Depth
	})
	return in
}

func addOmission(accounting *Accounting, reason OmissionReason, count int) {
	if count <= 0 {
		return
	}
	for i := range accounting.Omissions {
		if accounting.Omissions[i].Reason == reason {
			accounting.Omissions[i].Count += count
			return
		}
	}
	accounting.Omissions = append(accounting.Omissions, OmissionCount{Reason: reason, Count: count})
	sort.Slice(accounting.Omissions, func(i, j int) bool { return accounting.Omissions[i].Reason < accounting.Omissions[j].Reason })
}

func identitySalt(sessionID string, generation uint64) []byte {
	h := sha256.New()
	writeHashField(h, "lsp-trace/transient-structural/salt/v1")
	writeHashField(h, sessionID)
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], generation)
	h.Write(raw[:])
	return h.Sum(nil)
}

func opaqueID(domain string, salt []byte, raw string) string {
	h := sha256.New()
	writeHashField(h, domain)
	h.Write(salt)
	writeHashField(h, raw)
	return hex.EncodeToString(h.Sum(nil))
}

type hashWriter interface{ Write([]byte) (int, error) }

func writeHashField(h hashWriter, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = h.Write(size[:])
	_, _ = h.Write([]byte(value))
}

func graphDigest(projection admittedProjection, policy PolicyBinding, bounds BoundsBinding) string {
	canonical := struct {
		Domain      string
		TargetID    string
		Nodes       []NodeFact
		Occurrences []OccurrenceFact
		Policy      PolicyBinding
		Bounds      BoundsBinding
	}{"lsp-trace/transient-structural/graph/v1", projection.targetID, projection.nodes, projection.occurrences, policy, bounds}
	encoded, _ := json.Marshal(canonical)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}
