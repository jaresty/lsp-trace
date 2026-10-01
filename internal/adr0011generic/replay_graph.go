package adr0011generic

import (
	"fmt"
	"strings"
)

// All B2 records and held selections are private synthetic inputs. Neither
// a successful comparison nor a PROCESS record authenticates an LSP responder.
type syntheticTransactionIdentity struct {
	session, transaction string
	generation           uint64
}

type syntheticPredecessor struct {
	role, selector, digest string
}

type syntheticGraphRole struct {
	role, selector, digest, method, profile string
	identity                                syntheticTransactionIdentity
	predecessors                            []syntheticPredecessor
}

type syntheticGraphSelection struct {
	identity            syntheticTransactionIdentity
	method              string
	querySourceSelector string
	roles               []syntheticPredecessor // separately held; never inferred from claimant records
}

// The base V2 schema's 266 bound is only the outer ceiling. Each B2 role
// retains its own cardinality; READBACK is not a B2 pre-readback input.
func v2PredecessorBounds(role string) (min, max int, selected bool) {
	switch role {
	case "POLICY", "SCHEMA", "SOURCE", "PROCESS":
		return 0, 0, true
	case "QUERY":
		return 3, 3, true
	case "CAPABILITY_EVENTS":
		return 0, 1, true
	case "REQUEST_WRITE":
		return 3, 4, true
	case "INBOUND_FRAMES":
		return 1, 1, true
	case "RESULT_READ":
		return 2, 2, true
	case "TARGET_EVENTS":
		return 1, 256, true
	case "TERMINAL":
		return 9, 264, true // eight fixed originals + at least one SOURCE
	default:
		return 0, 0, false
	}
}

// This comparison does not retrieve originals, issue selectors, or validate
// source/capability/result semantics. A nil error means synthetic graph agreement
// with an independently held selection only; it is not a verified terminal.
func validateSyntheticGraph(claimed []syntheticGraphRole, held syntheticGraphSelection) error {
	var profile string
	switch held.method {
	case "textDocument/references":
		profile = "GENERIC_LSP_REFERENCES_EXACT_V2"
	case "textDocument/definition":
		profile = "GENERIC_LSP_DEFINITION_EXACT_V2"
	default:
		return fmt.Errorf("unsupported held method %q", held.method)
	}
	if held.identity.session == "" || held.identity.generation == 0 || held.identity.transaction == "" {
		return fmt.Errorf("incomplete held transaction identity")
	}
	if len(held.roles) == 0 || len(held.roles) > 266 {
		return fmt.Errorf("held role bound")
	}

	selected := make(map[string]syntheticPredecessor, len(held.roles))
	var selectedInventory []syntheticRole
	sourceCount := 0
	process := false
	for _, ref := range held.roles {
		if !strings.HasPrefix(ref.selector, "sha256:") || !validateSHA(strings.TrimPrefix(ref.selector, "sha256:")) ||
			!strings.HasPrefix(ref.digest, "sha256:") || !validateSHA(strings.TrimPrefix(ref.digest, "sha256:")) {
			return fmt.Errorf("invalid held selector or digest")
		}
		if _, duplicate := selected[ref.selector]; duplicate {
			return fmt.Errorf("duplicate held selector")
		}
		selected[ref.selector] = ref
		selectedInventory = append(selectedInventory, syntheticRole{role: ref.role})
		if ref.role == "SOURCE" {
			sourceCount++
		}
		if ref.role == "PROCESS" {
			process = true
		}
	}
	inventory := syntheticRoleSelection{sourceCount: sourceCount, process: process}
	if err := validateRoleInventory(selectedInventory, inventory); err != nil {
		return fmt.Errorf("held role selection: %w", err)
	}
	policyRole := "REFERENCES"
	if held.method == "textDocument/definition" {
		policyRole = "DEFINITION"
	}
	for _, pin := range []struct{ role, original string }{{"SCHEMA", "SCHEMA"}, {"POLICY", policyRole}} {
		original, err := loadOriginal(pin.original)
		if err != nil {
			return fmt.Errorf("held %s original unavailable: %w", pin.role, err)
		}
		selectedDigest := "sha256:" + digest(original)
		matched := false
		for _, ref := range held.roles {
			if ref.role == pin.role {
				matched = true
				if ref.digest != selectedDigest {
					return fmt.Errorf("held %s original pin mismatch", pin.role)
				}
			}
		}
		if !matched {
			return fmt.Errorf("held %s original missing", pin.role)
		}
	}
	if source, ok := selected[held.querySourceSelector]; !ok || source.role != "SOURCE" {
		return fmt.Errorf("held query SOURCE selector missing")
	}
	if len(claimed) == 0 || len(claimed) > 266 {
		return fmt.Errorf("claimant role bound")
	}
	claimInventory := make([]syntheticRole, 0, len(claimed))
	for _, record := range claimed {
		claimInventory = append(claimInventory, syntheticRole{role: record.role})
	}
	if err := validateRoleInventory(claimInventory, inventory); err != nil {
		return fmt.Errorf("claimant role inventory: %w", err)
	}
	// Exact accepted V2 role bounds. The B1 inventory separately excludes
	// READBACK and holds 1–256 distinct SOURCE selectors.
	for _, record := range claimed {
		min, max, ok := v2PredecessorBounds(record.role)
		if !ok {
			return fmt.Errorf("role %s not in private B2 predecessor envelope", record.role)
		}
		if len(record.predecessors) > max {
			return fmt.Errorf("role %s predecessor limit exceeded: %d > %d", record.role, len(record.predecessors), max)
		}
		if len(record.predecessors) < min {
			return fmt.Errorf("role %s predecessor set missing mandatory edge(s): %d < %d", record.role, len(record.predecessors), min)
		}
	}

	bySelector := make(map[string]syntheticGraphRole, len(claimed))
	for _, record := range claimed {
		if record.identity != held.identity {
			return fmt.Errorf("role %s transaction identity mismatch", record.role)
		}
		if record.method != held.method {
			return fmt.Errorf("role %s method substitution", record.role)
		}
		if record.profile != profile {
			return fmt.Errorf("role %s profile substitution", record.role)
		}
		ref, ok := selected[record.selector]
		if !ok {
			return fmt.Errorf("role %s selector not independently held", record.role)
		}
		if record.role != ref.role {
			return fmt.Errorf("role mismatch at selected selector")
		}
		if record.digest != ref.digest {
			return fmt.Errorf("role %s digest substitution (PROCESS is contextual, not responder authentication)", record.role)
		}
		if _, duplicate := bySelector[record.selector]; duplicate {
			return fmt.Errorf("duplicate claimant selector")
		}
		bySelector[record.selector] = record
	}
	if len(bySelector) != len(selected) {
		return fmt.Errorf("selected role selector missing")
	}

	// First validate each actual edge against its selected target. A cycle then
	// receives its own verdict even if it also changes the required edge set.
	edges := make(map[string][]string, len(claimed))
	for _, record := range claimed {
		seen := make(map[string]bool, len(record.predecessors))
		for _, predecessor := range record.predecessors {
			target, found := bySelector[predecessor.selector]
			if !found {
				return fmt.Errorf("predecessor selector missing or substituted")
			}
			if predecessor.role != target.role {
				return fmt.Errorf("predecessor role mismatch")
			}
			if predecessor.digest != target.digest {
				return fmt.Errorf("predecessor digest mismatch")
			}
			if seen[predecessor.selector] {
				return fmt.Errorf("duplicate predecessor")
			}
			seen[predecessor.selector] = true
			edges[record.selector] = append(edges[record.selector], predecessor.selector)
		}
	}
	colors := make(map[string]uint8, len(claimed))
	var visit func(string) error
	visit = func(selector string) error {
		switch colors[selector] {
		case 1:
			return fmt.Errorf("predecessor cycle detected")
		case 2:
			return nil
		}
		colors[selector] = 1
		for _, previous := range edges[selector] {
			if err := visit(previous); err != nil {
				return err
			}
		}
		colors[selector] = 2
		return nil
	}
	for selector := range bySelector {
		if err := visit(selector); err != nil {
			return err
		}
	}

	roles := make(map[string]syntheticPredecessor, len(selected))
	var targets []syntheticPredecessor
	for _, ref := range held.roles {
		if ref.role == "SOURCE" {
			if ref.selector != held.querySourceSelector {
				targets = append(targets, ref)
			}
		} else {
			roles[ref.role] = ref
		}
	}
	predecessors := func(names ...string) []syntheticPredecessor {
		out := make([]syntheticPredecessor, 0, len(names))
		for _, name := range names {
			out = append(out, roles[name])
		}
		return out
	}
	for _, record := range claimed {
		var required []syntheticPredecessor
		optionalProcess := false
		switch record.role {
		case "QUERY":
			required = append(predecessors("POLICY", "SCHEMA"), selected[held.querySourceSelector])
		case "CAPABILITY_EVENTS":
			optionalProcess = process
		case "REQUEST_WRITE":
			required = predecessors("QUERY", "CAPABILITY_EVENTS", "POLICY")
			optionalProcess = process
		case "INBOUND_FRAMES":
			required = predecessors("REQUEST_WRITE")
		case "RESULT_READ":
			required = predecessors("INBOUND_FRAMES", "REQUEST_WRITE")
		case "TARGET_EVENTS":
			required = append(predecessors("RESULT_READ"), targets...)
		case "TERMINAL":
			required = predecessors("POLICY", "SCHEMA", "QUERY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS")
			required = append(required, selected[held.querySourceSelector])
			required = append(required, targets...)
			optionalProcess = process
		case "POLICY", "SCHEMA", "SOURCE", "PROCESS":
			// No predecessors are required or permitted for these roots.
		default:
			return fmt.Errorf("unselected graph role %s", record.role)
		}
		expected := make(map[string]syntheticPredecessor, len(required))
		for _, ref := range required {
			expected[ref.selector] = ref
		}
		for _, actual := range record.predecessors {
			if optionalProcess && actual.selector == roles["PROCESS"].selector {
				continue // contextual edge may be absent; it never authenticates the response
			}
			ref, found := expected[actual.selector]
			if !found || ref != actual {
				return fmt.Errorf("role %s predecessor set substitution", record.role)
			}
			delete(expected, actual.selector)
		}
		if len(expected) != 0 {
			return fmt.Errorf("role %s predecessor set missing %d required edge(s)", record.role, len(expected))
		}
	}
	return nil
}
