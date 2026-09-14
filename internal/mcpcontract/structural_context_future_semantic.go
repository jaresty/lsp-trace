package mcpcontract

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"unicode/utf8"
)

const (
	futureMaxCount         = uint64(math.MaxInt64)
	futureMaxNodes         = 10000
	futureMaxEdges         = 100000
	futureMaxURICodePoints = 4096
)

var (
	errFutureShape      = errors.New("semantic-v1: invalid shape")
	errFutureValue      = errors.New("semantic-v1: invalid value")
	errFutureDuplicate  = errors.New("semantic-v1: duplicate identifier")
	errFutureReference  = errors.New("semantic-v1: invalid reference")
	errFutureArithmetic = errors.New("semantic-v1: arithmetic equation failed")
	errFutureState      = errors.New("semantic-v1: invalid state relation")
	futureNodeIDPattern = regexp.MustCompile(`^tn_[0-9a-f]{32}$`)
	futureEdgeIDPattern = regexp.MustCompile(`^te_[0-9a-f]{32}$`)
	// Keep this expression byte-for-byte equivalent to the operation-35 input
	// schema pattern. It intentionally defines a conservative URI subset rather
	// than delegating to a parser whose acceptance JSON Schema cannot mirror.
	futureAbsoluteURIPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*://|[A-Za-z][A-Za-z0-9+.-]*:[^/\x00-\x20\x7f%])[^\x00-\x20\x7f%]*(%[0-9A-Fa-f]{2}[^\x00-\x20\x7f%]*)*$`)
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

// ValidateFutureStructuralSemanticsV1 validates the FUTURE/PROPOSED operation-35
// draft without relying on prior JSON Schema validation. Validation order is fixed:
// input shape/bounds, result shape/constants/policies, analysis identity/references,
// accounting shape/arithmetic, then COMPLETE/EMPTY state. Errors are fixed sentinels
// and never include caller-controlled values. It remains intentionally unregistered.
func ValidateFutureStructuralSemanticsV1(input, result map[string]any) error {
	if err := validateFutureInput(input); err != nil {
		return err
	}
	if err := validateFutureResultHeader(result); err != nil {
		return err
	}
	for policyKey, fields := range map[string][]string{
		"traversal_policy": {"down_depth", "up_depth", "max_nodes"},
		"resource_policy":  {"timeout_ms", "request_timeout_ms", "max_messages", "max_bytes"},
	} {
		policy, _ := objectField(result, policyKey)
		for _, field := range fields {
			if policy[field] != input[field] {
				return errFutureValue
			}
		}
	}
	analysis, _ := objectField(result, "analysis")
	nodes, err := arrayField(analysis, "nodes", futureMaxNodes)
	if err != nil {
		return err
	}
	edges, err := arrayField(analysis, "edges", futureMaxEdges)
	if err != nil {
		return err
	}
	target, _ := stringField(result, "target_node_id", 35, 35, futureNodeIDPattern)
	root, err := stringField(analysis, "root_node_id", 35, 35, futureNodeIDPattern)
	if err != nil || root != target {
		return errFutureReference
	}
	nodeIDs := make(map[string]struct{}, len(nodes))
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok || node == nil || !closed(node, "node_id") {
			return errFutureShape
		}
		id, err := stringField(node, "node_id", 35, 35, futureNodeIDPattern)
		if err != nil {
			return err
		}
		if _, exists := nodeIDs[id]; exists {
			return errFutureDuplicate
		}
		nodeIDs[id] = struct{}{}
	}
	edgeIDs := make(map[string]struct{}, len(edges))
	for _, raw := range edges {
		edge, ok := raw.(map[string]any)
		if !ok || edge == nil || !closed(edge, "edge_id", "caller_node_id", "callee_node_id") {
			return errFutureShape
		}
		id, err := stringField(edge, "edge_id", 35, 35, futureEdgeIDPattern)
		if err != nil {
			return err
		}
		caller, err := stringField(edge, "caller_node_id", 35, 35, futureNodeIDPattern)
		if err != nil {
			return err
		}
		callee, err := stringField(edge, "callee_node_id", 35, 35, futureNodeIDPattern)
		if err != nil {
			return err
		}
		if _, exists := edgeIDs[id]; exists {
			return errFutureDuplicate
		}
		if _, ok := nodeIDs[caller]; !ok {
			return errFutureReference
		}
		if _, ok := nodeIDs[callee]; !ok {
			return errFutureReference
		}
		edgeIDs[id] = struct{}{}
	}
	kind, _ := stringField(analysis, "kind", 1, 32, nil)
	switch kind {
	case "NEIGHBORHOOD":
		if !closed(analysis, "kind", "root_node_id", "nodes", "edges", "incoming_count", "outgoing_count", "frontier_count") {
			return errFutureShape
		}
		for _, k := range []string{"incoming_count", "outgoing_count", "frontier_count"} {
			if _, err := uintField(analysis, k, 0, futureMaxCount); err != nil {
				return err
			}
		}
	case "IMPACT":
		if !closed(analysis, "kind", "root_node_id", "direction", "depth", "nodes", "edges", "reachable_node_ids", "witness_edge_ids") {
			return errFutureShape
		}
		direction, err := enumField(analysis, "direction", "INCOMING", "OUTGOING")
		if err != nil {
			return err
		}
		depth, err := uintField(analysis, "depth", 1, 64)
		if err != nil {
			return err
		}
		limitKey := "down_depth"
		if direction == "INCOMING" {
			limitKey = "up_depth"
		}
		limit, _ := uintField(input, limitKey, 0, 64)
		if depth > limit {
			return errFutureValue
		}
		if err := validateReferenceArray(analysis, "reachable_node_ids", futureMaxNodes, futureNodeIDPattern, nodeIDs); err != nil {
			return err
		}
		if err := validateReferenceArray(analysis, "witness_edge_ids", futureMaxEdges, futureEdgeIDPattern, edgeIDs); err != nil {
			return err
		}
	default:
		return errFutureValue
	}
	accounting, err := objectField(result, "accounting")
	if err != nil {
		return err
	}
	if err := validateFutureAccounting(accounting); err != nil {
		return err
	}
	state, _ := enumField(result, "state", "COMPLETE", "EMPTY")
	if (state == "EMPTY") != (len(edges) == 0) {
		return errFutureState
	}
	return nil
}

func validateFutureInput(v map[string]any) error {
	if v == nil || !allowed(v, "session_id", "generation", "uri", "symbol", "line", "character", "down_depth", "up_depth", "max_nodes", "timeout_ms", "request_timeout_ms", "max_messages", "max_bytes", "analysis") || !required(v, "session_id", "generation", "uri", "down_depth", "up_depth", "max_nodes", "timeout_ms", "request_timeout_ms", "max_messages", "max_bytes", "analysis") {
		return errFutureShape
	}
	if _, err := stringField(v, "session_id", 1, 256, nil); err != nil {
		return err
	}
	if _, err := uintField(v, "generation", 1, futureMaxCount); err != nil {
		return err
	}
	uri, err := stringField(v, "uri", 1, futureMaxURICodePoints, nil)
	if err != nil || !futureAbsoluteURIPattern.MatchString(uri) {
		return errFutureValue
	}
	_, hasSymbol := v["symbol"]
	_, hasLine := v["line"]
	_, hasChar := v["character"]
	if hasSymbol == (hasLine || hasChar) || hasLine != hasChar {
		return errFutureShape
	}
	if hasSymbol {
		if _, err := stringField(v, "symbol", 1, 1024, nil); err != nil {
			return err
		}
	} else {
		if _, err := uintField(v, "line", 0, math.MaxUint32); err != nil {
			return err
		}
		if _, err := uintField(v, "character", 0, math.MaxUint32); err != nil {
			return err
		}
	}
	for _, k := range []string{"down_depth", "up_depth"} {
		if _, err := uintField(v, k, 0, 64); err != nil {
			return err
		}
	}
	if _, err := uintField(v, "max_nodes", 1, futureMaxNodes); err != nil {
		return err
	}
	for _, k := range []string{"timeout_ms", "request_timeout_ms"} {
		if _, err := uintField(v, k, 1, 60000); err != nil {
			return err
		}
	}
	timeout, _ := uintField(v, "timeout_ms", 1, 60000)
	requestTimeout, _ := uintField(v, "request_timeout_ms", 1, 60000)
	if requestTimeout > timeout {
		return errFutureValue
	}
	if _, err := uintField(v, "max_messages", 1, futureMaxMessages); err != nil {
		return err
	}
	if _, err := uintField(v, "max_bytes", 1, futureMaxBytes); err != nil {
		return err
	}
	a, err := objectField(v, "analysis")
	if err != nil {
		return err
	}
	kind, err := enumField(a, "kind", "NEIGHBORHOOD", "IMPACT")
	if err != nil {
		return err
	}
	if kind == "NEIGHBORHOOD" {
		if !closed(a, "kind") {
			return errFutureShape
		}
	} else {
		if !closed(a, "kind", "direction", "depth") {
			return errFutureShape
		}
		if _, err := enumField(a, "direction", "INCOMING", "OUTGOING"); err != nil {
			return err
		}
		if _, err := uintField(a, "depth", 1, 64); err != nil {
			return err
		}
	}
	return nil
}

func validateFutureResultHeader(v map[string]any) error {
	keys := []string{"schema_version", "phase", "state", "evidence_class", "authority", "source_graph_complete", "retained", "replayable", "publication_eligible", "hydration_eligible", "claim_ceiling", "canonical_session_id", "generation", "target_node_id", "position_encoding", "traversal_policy", "resource_policy", "analysis_policy", "graph_digest", "accounting", "analysis"}
	if v == nil || !closed(v, keys...) {
		return errFutureShape
	}
	constants := map[string]string{"schema_version": "lsp-trace.transient-structural-result.v1", "phase": "DELIVERY_CHECK", "evidence_class": "TRANSIENT_LIVE", "source_graph_complete": "UNKNOWN", "claim_ceiling": "Under the named managed session generation, exact target, server responses, traversal bounds, and analysis policy, this bounded server-reported call graph has the reported structural properties."}
	for k, want := range constants {
		got, ok := v[k].(string)
		if !ok || got != want {
			return errFutureValue
		}
	}
	if _, err := enumField(v, "state", "COMPLETE", "EMPTY"); err != nil {
		return err
	}
	if n, err := uintField(v, "authority", 0, 0); err != nil || n != 0 {
		return errFutureValue
	}
	for _, k := range []string{"retained", "replayable", "publication_eligible", "hydration_eligible"} {
		b, ok := v[k].(bool)
		if !ok || b {
			return errFutureValue
		}
	}
	if _, err := stringField(v, "canonical_session_id", 35, 35, regexp.MustCompile(`^ts_[0-9a-f]{32}$`)); err != nil {
		return err
	}
	if _, err := uintField(v, "generation", 1, futureMaxCount); err != nil {
		return err
	}
	if _, err := stringField(v, "target_node_id", 35, 35, futureNodeIDPattern); err != nil {
		return err
	}
	if _, err := enumField(v, "position_encoding", "utf-8", "utf-16", "utf-32"); err != nil {
		return err
	}
	if _, err := stringField(v, "graph_digest", 71, 71, regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)); err != nil {
		return err
	}
	if _, err := validatePolicy(v, "traversal_policy", futureTraversalPolicyID, futureTraversalPolicyDigest, []bound{{"down_depth", 0, 64}, {"up_depth", 0, 64}, {"max_nodes", 1, 10000}}, false); err != nil {
		return err
	}
	if _, err := validatePolicy(v, "resource_policy", futureResourcePolicyID, futureResourcePolicyDigest, []bound{{"timeout_ms", 1, 60000}, {"request_timeout_ms", 1, 60000}, {"max_messages", 1, futureMaxMessages}, {"max_bytes", 1, futureMaxBytes}}, false); err != nil {
		return err
	}
	policyID, err := validatePolicy(v, "analysis_policy", "", "", nil, true)
	if err != nil {
		return err
	}
	analysis, err := objectField(v, "analysis")
	if err != nil {
		return err
	}
	kind, err := enumField(analysis, "kind", "NEIGHBORHOOD", "IMPACT")
	if err != nil {
		return err
	}
	if (policyID == futureNeighborhoodID) != (kind == "NEIGHBORHOOD") {
		return errFutureValue
	}
	return nil
}

type bound struct {
	key      string
	min, max uint64
}

func validatePolicy(parent map[string]any, key, policyID, policyDigest string, bounds []bound, analysis bool) (string, error) {
	p, err := objectField(parent, key)
	if err != nil {
		return "", err
	}
	keys := []string{"policy_id", "policy_version", "policy_status", "policy_digest"}
	if !analysis {
		for _, b := range bounds {
			keys = append(keys, b.key)
		}
	}
	if !closed(p, keys...) {
		return "", errFutureShape
	}
	id, ok := p["policy_id"].(string)
	if !ok {
		return "", errFutureValue
	}
	if analysis {
		if id != futureNeighborhoodID && id != futureImpactID {
			return "", errFutureValue
		}
		if p["policy_version"] != futureAnalysisVersion {
			return "", errFutureValue
		}
		if id == futureNeighborhoodID {
			policyDigest = futureNeighborhoodPolicyDigest
		} else {
			policyDigest = futureImpactPolicyDigest
		}
	} else if id != policyID {
		return "", errFutureValue
	}
	if p["policy_version"] != futureAnalysisVersion {
		return "", errFutureValue
	}
	if p["policy_status"] != futurePolicyStatus || p["policy_digest"] != policyDigest {
		return "", errFutureValue
	}
	for _, b := range bounds {
		if _, err := uintField(p, b.key, b.min, b.max); err != nil {
			return "", err
		}
	}
	return id, nil
}

func validateFutureAccounting(a map[string]any) error {
	counts := []string{"request_attempted", "request_succeeded", "request_failed", "request_cancelled", "prepared_attempted", "prepared_returned", "prepared_empty", "prepared_failed", "node_observed", "node_admitted", "node_rejected", "node_omitted", "occurrence_observed", "occurrence_admitted", "occurrence_rejected", "occurrence_omitted", "frontier_observed", "frontier_expanded", "frontier_unexpanded", "deduplicated_nodes", "deduplicated_occurrences"}
	reasons := []string{"request_omission_reasons", "node_omission_reasons", "occurrence_omission_reasons", "frontier_omission_reasons"}
	keys := append(append([]string{}, counts...), reasons...)
	keys = append(keys, "truncated")
	if a == nil || !closed(a, keys...) {
		return errFutureShape
	}
	if b, ok := a["truncated"].(bool); !ok || b {
		return errFutureValue
	}
	for _, k := range counts {
		if _, err := uintField(a, k, 0, futureMaxCount); err != nil {
			return err
		}
	}
	for _, eq := range [][]string{{"node_observed", "node_admitted", "node_rejected", "node_omitted"}, {"occurrence_observed", "occurrence_admitted", "occurrence_rejected", "occurrence_omitted"}, {"frontier_observed", "frontier_expanded", "frontier_unexpanded"}, {"request_attempted", "request_succeeded", "request_failed", "request_cancelled"}, {"prepared_attempted", "prepared_returned", "prepared_empty", "prepared_failed"}} {
		if err := equation(a, eq[0], eq[1:]...); err != nil {
			return err
		}
	}
	pairs := [][2]string{{"node_omitted", "node_omission_reasons"}, {"occurrence_omitted", "occurrence_omission_reasons"}, {"frontier_unexpanded", "frontier_omission_reasons"}}
	for _, p := range pairs {
		sum, err := reasonSum(a, p[1])
		if err != nil {
			return err
		}
		total, _ := uintField(a, p[0], 0, futureMaxCount)
		if sum != total {
			return errFutureArithmetic
		}
	}
	sum, err := reasonSum(a, "request_omission_reasons")
	if err != nil {
		return err
	}
	failed, _ := uintField(a, "request_failed", 0, futureMaxCount)
	cancelled, _ := uintField(a, "request_cancelled", 0, futureMaxCount)
	want, ok := add(failed, cancelled)
	if !ok || sum != want {
		return errFutureArithmetic
	}
	return nil
}

func reasonSum(parent map[string]any, key string) (uint64, error) {
	r, err := objectField(parent, key)
	if err != nil {
		return 0, err
	}
	names := []string{"DEPTH_BOUND", "NODE_BOUND", "REQUEST_BOUND", "TIMEOUT", "CANCELLATION", "UNSUPPORTED_RESPONSE", "MALFORMED_RESPONSE", "DEDUPLICATION"}
	if !closed(r, names...) {
		return 0, errFutureShape
	}
	var sum uint64
	for _, k := range names {
		n, err := uintField(r, k, 0, futureMaxCount)
		if err != nil {
			return 0, err
		}
		var ok bool
		sum, ok = add(sum, n)
		if !ok {
			return 0, errFutureArithmetic
		}
	}
	return sum, nil
}
func equation(v map[string]any, total string, parts ...string) error {
	want, _ := uintField(v, total, 0, futureMaxCount)
	var sum uint64
	for _, k := range parts {
		n, _ := uintField(v, k, 0, futureMaxCount)
		var ok bool
		sum, ok = add(sum, n)
		if !ok {
			return errFutureArithmetic
		}
	}
	if sum != want {
		return errFutureArithmetic
	}
	return nil
}
func add(a, b uint64) (uint64, bool) {
	if b > futureMaxCount || a > futureMaxCount-b {
		return 0, false
	}
	return a + b, true
}

func validateReferenceArray(parent map[string]any, key string, max int, pattern *regexp.Regexp, admitted map[string]struct{}) error {
	values, err := arrayField(parent, key, max)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		id, ok := raw.(string)
		if !ok {
			return errFutureValue
		}
		count, withinMax := boundedRuneCount(id, 35)
		if !withinMax || count != 35 || !pattern.MatchString(id) {
			return errFutureValue
		}
		if _, ok := seen[id]; ok {
			return errFutureDuplicate
		}
		if _, ok := admitted[id]; !ok {
			return errFutureReference
		}
		seen[id] = struct{}{}
	}
	return nil
}
func closed(v map[string]any, keys ...string) bool {
	return v != nil && len(v) == len(keys) && required(v, keys...)
}
func required(v map[string]any, keys ...string) bool {
	if v == nil {
		return false
	}
	for _, k := range keys {
		if _, ok := v[k]; !ok {
			return false
		}
	}
	return true
}
func allowed(v map[string]any, keys ...string) bool {
	if v == nil || len(v) > len(keys) {
		return false
	}
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	for k := range v {
		if _, ok := set[k]; !ok {
			return false
		}
	}
	return true
}
func objectField(v map[string]any, key string) (map[string]any, error) {
	raw, ok := v[key]
	if !ok {
		return nil, errFutureShape
	}
	out, ok := raw.(map[string]any)
	if !ok || out == nil {
		return nil, errFutureShape
	}
	return out, nil
}
func arrayField(v map[string]any, key string, max int) ([]any, error) {
	raw, ok := v[key]
	if !ok {
		return nil, errFutureShape
	}
	out, ok := raw.([]any)
	if !ok || out == nil || len(out) > max {
		return nil, errFutureShape
	}
	return out, nil
}
func stringField(v map[string]any, key string, min, max int, pattern *regexp.Regexp) (string, error) {
	raw, ok := v[key]
	if !ok {
		return "", errFutureShape
	}
	s, ok := raw.(string)
	if !ok {
		return "", errFutureValue
	}
	count, withinMax := boundedRuneCount(s, max)
	if !withinMax || count < min || (pattern != nil && !pattern.MatchString(s)) {
		return "", errFutureValue
	}
	return s, nil
}

func boundedRuneCount(s string, max int) (int, bool) {
	count := 0
	for len(s) > 0 {
		if count == max {
			return count, false
		}
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			return count, false
		}
		s = s[size:]
		count++
	}
	return count, true
}

func enumField(v map[string]any, key string, allowed ...string) (string, error) {
	s, err := stringField(v, key, 1, 128, nil)
	if err != nil {
		return "", err
	}
	for _, a := range allowed {
		if s == a {
			return s, nil
		}
	}
	return "", errFutureValue
}
func uintField(v map[string]any, key string, min, max uint64) (uint64, error) {
	raw, ok := v[key]
	if !ok {
		return 0, errFutureShape
	}
	var n uint64
	switch x := raw.(type) {
	case int:
		if x < 0 {
			return 0, errFutureValue
		}
		n = uint64(x)
	case int8:
		if x < 0 {
			return 0, errFutureValue
		}
		n = uint64(x)
	case int16:
		if x < 0 {
			return 0, errFutureValue
		}
		n = uint64(x)
	case int32:
		if x < 0 {
			return 0, errFutureValue
		}
		n = uint64(x)
	case int64:
		if x < 0 {
			return 0, errFutureValue
		}
		n = uint64(x)
	case uint:
		n = uint64(x)
	case uint8:
		n = uint64(x)
	case uint16:
		n = uint64(x)
	case uint32:
		n = uint64(x)
	case uint64:
		n = x
	default:
		return 0, errFutureValue
	}
	if n < min || n > max {
		return 0, errFutureValue
	}
	return n, nil
}
