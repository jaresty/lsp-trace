// Package relations implements semantics shared by normalized relation artifacts.
package relations

import (
	"fmt"
	"sort"
)

// Observation carries the provenance identities used to determine whether two
// normalized relation occurrences are independently supported.
type Observation struct {
	ID                     string
	UpstreamObservationIDs []string
	ExecutionID            string
	ResponseID             string
	ReceiptID              string
}

// MinimumDependence validates the complete upstream-observation graph and
// returns deterministic connected components over all normative dependence
// sources: shared execution, response, receipt, and transitive upstream
// provenance. Empty provenance identities do not create dependence.
func MinimumDependence(observations []Observation) ([][]string, error) {
	byID := make(map[string]Observation, len(observations))
	for _, observation := range observations {
		if observation.ID == "" {
			return nil, fmt.Errorf("missing observation id")
		}
		if _, exists := byID[observation.ID]; exists {
			return nil, fmt.Errorf("duplicate observation id %q", observation.ID)
		}
		byID[observation.ID] = observation
	}
	for _, observation := range observations {
		for _, upstreamID := range observation.UpstreamObservationIDs {
			if _, exists := byID[upstreamID]; !exists {
				return nil, fmt.Errorf("missing upstream observation %q referenced by %q", upstreamID, observation.ID)
			}
		}
	}

	state := make(map[string]uint8, len(observations))
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("cyclic upstream observations at %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		for _, upstreamID := range byID[id].UpstreamObservationIDs {
			if err := visit(upstreamID); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range byID {
		if err := visit(id); err != nil {
			return nil, err
		}
	}

	parent := make(map[string]string, len(observations))
	for id := range byID {
		parent[id] = id
	}
	var find func(string) string
	find = func(id string) string {
		if parent[id] != id {
			parent[id] = find(parent[id])
		}
		return parent[id]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}

	firstExecution := map[string]string{}
	firstResponse := map[string]string{}
	firstReceipt := map[string]string{}
	share := func(index map[string]string, key, id string) {
		if key == "" {
			return
		}
		if first, exists := index[key]; exists {
			union(first, id)
		} else {
			index[key] = id
		}
	}
	for _, observation := range observations {
		share(firstExecution, observation.ExecutionID, observation.ID)
		share(firstResponse, observation.ResponseID, observation.ID)
		share(firstReceipt, observation.ReceiptID, observation.ID)
		for _, upstreamID := range observation.UpstreamObservationIDs {
			union(observation.ID, upstreamID)
		}
	}

	grouped := make(map[string][]string)
	for id := range byID {
		root := find(id)
		grouped[root] = append(grouped[root], id)
	}
	classes := make([][]string, 0, len(grouped))
	for _, members := range grouped {
		sort.Strings(members)
		classes = append(classes, members)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i][0] < classes[j][0] })
	return classes, nil
}
