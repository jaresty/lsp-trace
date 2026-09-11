package gonumleiden

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type frozenExport struct {
	SchemaVersion string `json:"schema_version"`
	Tables        struct {
		Endpoints []struct {
			ID string `json:"id"`
		} `json:"endpoints"`
		Groups []struct {
			Caller        string   `json:"caller_node_id"`
			Callee        string   `json:"callee_node_id"`
			OccurrenceIDs []string `json:"occurrence_ids"`
			Receipt       struct {
				RelationKind        string `json:"relation_kind"`
				EvidenceClass       string `json:"evidence_class"`
				SupportContribution int    `json:"support_contribution"`
			} `json:"receipt"`
		} `json:"groups"`
	} `json:"tables"`
}

func Digest(b []byte) string                    { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func DeriveFixture(raw []byte) (Fixture, error) { return deriveFixture(raw, RetainedExportSHA256) }
func deriveFixture(raw []byte, expectedDigest string) (Fixture, error) {
	d := Digest(raw)
	if d != expectedDigest {
		return Fixture{}, Fail("INPUT_DIGEST_MISMATCH", d)
	}
	var x frozenExport
	if err := json.Unmarshal(raw, &x); err != nil {
		return Fixture{}, Fail("DERIVATION_REJECTED", err.Error())
	}
	if x.SchemaVersion != "lsp-trace.retained-calls.v1" {
		return Fixture{}, Fail("DERIVATION_REJECTED", "schema_version")
	}
	nodeSet := map[string]bool{}
	for _, n := range x.Tables.Endpoints {
		if n.ID == "" || nodeSet[n.ID] {
			return Fixture{}, Fail("DERIVATION_REJECTED", "endpoint identity")
		}
		nodeSet[n.ID] = true
	}
	var edges []Edge
	for _, g := range x.Tables.Groups {
		// This is deliberately exact: no inferred, client-derived, or non-CALL support is admitted.
		if g.Receipt.RelationKind != "CALL_RELATION" || g.Receipt.EvidenceClass != "SERVER_REPORTED_CALL_HIERARCHY" {
			continue
		}
		if !nodeSet[g.Caller] || !nodeSet[g.Callee] || len(g.OccurrenceIDs) == 0 || g.Receipt.SupportContribution <= 0 {
			return Fixture{}, Fail("DERIVATION_REJECTED", "admitted group integrity")
		}
		uniq := map[string]bool{}
		for _, id := range g.OccurrenceIDs {
			if id == "" || uniq[id] {
				return Fixture{}, Fail("DERIVATION_REJECTED", "occurrence identity")
			}
			uniq[id] = true
		}
		edges = append(edges, Edge{From: g.Caller, To: g.Callee, Occurrences: len(g.OccurrenceIDs)})
	}
	if len(edges) == 0 {
		return Fixture{}, Fail("DERIVATION_REJECTED", "no SERVER_REPORTED CALL support")
	}
	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	for i := 1; i < len(edges); i++ {
		if edges[i-1].From == edges[i].From && edges[i-1].To == edges[i].To {
			return Fixture{}, Fail("DERIVATION_REJECTED", fmt.Sprintf("duplicate adjacency %s -> %s", edges[i].From, edges[i].To))
		}
	}
	if len(nodes) > MaxNodes || len(edges) > MaxEdges {
		return Fixture{}, Fail("CAP_BREACH", "derived fixture")
	}
	return Fixture{SourceDigest: d, Nodes: nodes, Edges: edges}, nil
}
