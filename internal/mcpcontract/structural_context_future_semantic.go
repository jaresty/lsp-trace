package mcpcontract

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewFutureStructuralCorrelationID returns a host-generated opaque operation-35
// correlation ID. One invocation is reused by success and domain-error envelope
// construction for the same request; caller input is deliberately not accepted.
func NewFutureStructuralCorrelationID() (string, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("generate structural correlation id: %w", err)
	}
	return "sc_" + hex.EncodeToString(entropy[:]), nil
}

// ValidateFutureStructuralSemanticsV1 is a pure validator for the FUTURE/PROPOSED
// operation-35 draft. It is intentionally not connected to the MCP registry or
// transport. Registration MUST remain blocked until every producer invokes this
// validator (or an equivalent validator conforming to the versioned requirements).
func ValidateFutureStructuralSemanticsV1(input, result map[string]any) error {
	analysis, ok := result["analysis"].(map[string]any)
	if !ok {
		return fmt.Errorf("semantic-v1: analysis is required")
	}
	target, _ := result["target_node_id"].(string)
	if root, _ := analysis["root_node_id"].(string); root != target {
		return fmt.Errorf("semantic-v1: root_node_id must equal target_node_id")
	}

	nodes, _ := analysis["nodes"].([]any)
	edges, _ := analysis["edges"].([]any)
	nodeIDs := map[string]bool{}
	edgeIDs := map[string]bool{}
	for _, raw := range nodes {
		if node, ok := raw.(map[string]any); ok {
			nodeIDs[futureStringValue(node["node_id"])] = true
		}
	}
	for _, raw := range edges {
		edge, ok := raw.(map[string]any)
		if !ok || !nodeIDs[futureStringValue(edge["caller_node_id"])] || !nodeIDs[futureStringValue(edge["callee_node_id"])] {
			return fmt.Errorf("semantic-v1: edge endpoint must reference an admitted node")
		}
		edgeIDs[futureStringValue(edge["edge_id"])] = true
	}
	if kind, _ := analysis["kind"].(string); kind == "IMPACT" {
		direction := futureStringValue(analysis["direction"])
		limitKey := map[string]string{"INCOMING": "up_depth", "OUTGOING": "down_depth"}[direction]
		if futureInteger(analysis["depth"]) > futureInteger(input[limitKey]) {
			return fmt.Errorf("semantic-v1: impact depth exceeds directional traversal depth")
		}
		for _, id := range futureStringSlice(analysis["reachable_node_ids"]) {
			if !nodeIDs[id] {
				return fmt.Errorf("semantic-v1: reachable node reference is not admitted")
			}
		}
		for _, id := range futureStringSlice(analysis["witness_edge_ids"]) {
			if !edgeIDs[id] {
				return fmt.Errorf("semantic-v1: witness edge reference is not admitted")
			}
		}
	}

	accounting, ok := result["accounting"].(map[string]any)
	if !ok {
		return fmt.Errorf("semantic-v1: accounting is required")
	}
	if err := futureEquation(accounting, "node_observed", "node_admitted", "node_rejected", "node_omitted"); err != nil {
		return err
	}
	if err := futureEquation(accounting, "occurrence_observed", "occurrence_admitted", "occurrence_rejected", "occurrence_omitted"); err != nil {
		return err
	}
	if err := futureEquation(accounting, "frontier_observed", "frontier_expanded", "frontier_unexpanded"); err != nil {
		return err
	}
	if err := futureEquation(accounting, "request_attempted", "request_succeeded", "request_failed", "request_cancelled"); err != nil {
		return err
	}
	if err := futureEquation(accounting, "prepared_attempted", "prepared_returned", "prepared_empty", "prepared_failed"); err != nil {
		return err
	}
	for _, pair := range [][2]string{{"node_omitted", "node_omission_reasons"}, {"occurrence_omitted", "occurrence_omission_reasons"}, {"frontier_unexpanded", "frontier_omission_reasons"}} {
		reasons, ok := accounting[pair[1]].(map[string]any)
		if !ok {
			return fmt.Errorf("semantic-v1: %s is required", pair[1])
		}
		sum := 0
		for _, value := range reasons {
			sum += futureInteger(value)
		}
		if sum != futureInteger(accounting[pair[0]]) {
			return fmt.Errorf("semantic-v1: %s must equal sum(%s)", pair[0], pair[1])
		}
	}
	requestReasons, ok := accounting["request_omission_reasons"].(map[string]any)
	if !ok {
		return fmt.Errorf("semantic-v1: request_omission_reasons is required")
	}
	requestReasonSum := 0
	for _, value := range requestReasons {
		requestReasonSum += futureInteger(value)
	}
	if requestReasonSum != futureInteger(accounting["request_failed"])+futureInteger(accounting["request_cancelled"]) {
		return fmt.Errorf("semantic-v1: request omission reasons must partition failed and cancelled requests")
	}
	if (futureStringValue(result["state"]) == "EMPTY") != (len(edges) == 0) {
		return fmt.Errorf("semantic-v1: EMPTY iff zero CALLS edges are admitted")
	}
	return nil
}

func futureEquation(values map[string]any, total string, parts ...string) error {
	sum := 0
	for _, part := range parts {
		sum += futureInteger(values[part])
	}
	if futureInteger(values[total]) != sum {
		return fmt.Errorf("semantic-v1: %s arithmetic equation failed", total)
	}
	return nil
}
func futureInteger(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}
func futureStringValue(value any) string { valueString, _ := value.(string); return valueString }
func futureStringSlice(value any) []string {
	values, _ := value.([]any)
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, futureStringValue(value))
	}
	return out
}
