// Package communityregister aggregates retained structural Leiden partitions.
// It makes no semantic, ownership, feature, or runtime-call claims.
package communityregister

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcpresentation"
	"lsp-trace/internal/schema"
)

const Version = "lsp-trace.technical-community-register.v1"
const identityDomain = "lsp-trace/technical-community-register/member-set/v1\x00"
const StabilityPolicy = "EXACT_CANONICAL_MEMBER_SET_ACROSS_RETAINED_PARTITIONS/v1"

type Occurrence struct {
	PartitionID      string `json:"partition_id"`
	LocalCommunityID string `json:"local_community_id"`
	Seed             uint64 `json:"seed"`
}
type Stability struct {
	Status      string `json:"status"`
	Policy      string `json:"policy"`
	Numerator   int    `json:"numerator"`
	Denominator int    `json:"denominator"`
	Reason      string `json:"reason,omitempty"`
}
type Community struct {
	CommunityID string       `json:"community_id"`
	Members     []string     `json:"members"`
	Occurrences []Occurrence `json:"occurrences"`
	Stability   Stability    `json:"stability"`
}
type Register struct {
	SchemaVersion       string      `json:"schema_version"`
	Authority           int         `json:"authority"`
	SourceGraphComplete string      `json:"source_graph_complete"`
	SourceGraphSHA256   string      `json:"source_graph_sha256"`
	PartitionCount      int         `json:"partition_count"`
	Stability           Stability   `json:"stability"`
	Communities         []Community `json:"communities"`
}

func Aggregate(graphRaw []byte, partitionRaw ...[]byte) (Register, error) {
	projection, failure := programc.Project(graphRaw)
	if failure != nil {
		return Register{}, fmt.Errorf("graph admission: %w", failure)
	}
	if len(partitionRaw) == 0 {
		return Register{}, fmt.Errorf("at least one --partition is required")
	}
	graphDigest := projection.Source.InputSHA256
	admitted := append([]string(nil), projection.NodeIdentities...)
	sort.Strings(admitted)
	seenPartitions := map[string]bool{}
	byMembers := map[string]*Community{}
	for _, raw := range partitionRaw {
		if err := programcpresentation.ValidateJSON(raw); err != nil {
			return Register{}, fmt.Errorf("partition admission: %w", err)
		}
		var p programcpresentation.Artifact
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			return Register{}, err
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			return Register{}, fmt.Errorf("partition must contain one document")
		}
		canonical, err := programcpresentation.Handle(programcpresentation.Request{
			Input: graphRaw, Seed: p.Seed,
			PageRankTopK: p.Request.PageRankTopK, HubTopK: p.Request.HubTopK,
		})
		if err != nil {
			return Register{}, fmt.Errorf("partition compatibility recomputation: %w", err)
		}
		if !reflect.DeepEqual(p, canonical) {
			return Register{}, fmt.Errorf("partition %q is incompatible with the admitted graph", p.PartitionSHA256)
		}
		if seenPartitions[p.PartitionSHA256] {
			return Register{}, fmt.Errorf("duplicate partition %q", p.PartitionSHA256)
		}
		seenPartitions[p.PartitionSHA256] = true
		covered := make([]string, 0, len(admitted))
		localSeen := map[string]bool{}
		for _, pc := range p.Communities {
			members := make([]string, len(pc.Members))
			for i, n := range pc.Members {
				members[i] = n.NodeID
				if localSeen[n.NodeID] {
					return Register{}, fmt.Errorf("partition %q duplicate member %q", p.PartitionSHA256, n.NodeID)
				}
				localSeen[n.NodeID] = true
			}
			sort.Strings(members)
			covered = append(covered, members...)
			keyBytes, _ := json.Marshal(members)
			key := string(keyBytes)
			c := byMembers[key]
			if c == nil {
				sum := sha256.Sum256(append(append([]byte(identityDomain), []byte(graphDigest)...), keyBytes...))
				c = &Community{CommunityID: fmt.Sprintf("sha256:%x", sum), Members: members}
				byMembers[key] = c
			}
			c.Occurrences = append(c.Occurrences, Occurrence{p.PartitionSHA256, pc.CommunityID, p.Seed})
		}
		sort.Strings(covered)
		if !equal(covered, admitted) {
			return Register{}, fmt.Errorf("partition %q incomplete or graph-mismatched node coverage", p.PartitionSHA256)
		}
	}
	communities := make([]Community, 0, len(byMembers))
	comparisons := len(partitionRaw) - 1
	for _, c := range byMembers {
		sort.Slice(c.Occurrences, func(i, j int) bool { return c.Occurrences[i].PartitionID < c.Occurrences[j].PartitionID })
		c.Stability = stability(len(c.Occurrences)-1, comparisons, len(partitionRaw))
		communities = append(communities, *c)
	}
	sort.Slice(communities, func(i, j int) bool { return compare(communities[i].Members, communities[j].Members) < 0 })
	r := Register{Version, 0, "UNKNOWN", graphDigest, len(partitionRaw), stability(0, comparisons, len(partitionRaw)), communities}
	if len(partitionRaw) > 1 {
		r.Stability.Status = "UNKNOWN"
		r.Stability.Reason = "PER_COMMUNITY_STATUS"
	}
	return r, nil
}
func stability(n, d, count int) Stability {
	s := Stability{Policy: StabilityPolicy, Numerator: n, Denominator: d}
	if count == 1 {
		s.Status = "UNKNOWN"
		s.Reason = "INSUFFICIENT_COMPARABLE_PARTITIONS"
	} else if n == d {
		s.Status = "STABLE"
	} else {
		s.Status = "UNSTABLE"
	}
	return s
}
func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
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
func JSON(r Register) ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	if _, err = schema.ValidateFor(b, schema.FamilyTechnicalCommunityRegister, "v1"); err != nil {
		return nil, err
	}
	return b, nil
}
