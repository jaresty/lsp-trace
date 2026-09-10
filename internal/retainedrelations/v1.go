// Package retainedrelations exports non-CALLS relations without widening CALLS semantics.
package retainedrelations

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
)

const Version = "lsp-trace.retained-relations.v1"
const Family = "lsp-trace.retained-relations"
const Policy = "EXACT_GRAPH_PROVENANCE_V5_DISTINCT_CALLS_AND_SIBLING_CANDIDATES"
const MaxInputBytes = graphprovenance.MaxEnvelopeBytesV5
const MaxOutputBytes = 384 << 20
const MaxRelations = 8192

type Calls struct {
	RelationID        string        `json:"relation_id"`
	ExecutionBundleID string        `json:"execution_bundle_id"`
	CallerNodeID      string        `json:"caller_node_id"`
	CalleeNodeID      string        `json:"callee_node_id"`
	CallSites         []graph.Range `json:"call_sites"`
	EvidenceRole      string        `json:"evidence_role"`
	Support           int           `json:"support"`
}
type Sibling struct {
	RelationID        string                      `json:"relation_id"`
	ExecutionBundleID string                      `json:"execution_bundle_id"`
	SeedURI           string                      `json:"seed_uri"`
	SeedLabel         string                      `json:"seed_label"`
	SeedIdentity      string                      `json:"seed_identity"`
	Direction         string                      `json:"direction"`
	Kind              string                      `json:"kind"`
	Origin            graph.Node                  `json:"origin"`
	Declaration       graph.Node                  `json:"document_symbol"`
	PreparedCandidate graph.Node                  `json:"prepared_candidate"`
	ProviderEvidence  []string                    `json:"provider_evidence"`
	LSPEvidence       []string                    `json:"lsp_evidence"`
	SourceDigests     []string                    `json:"source_digests"`
	Custody           graph.SourceCustodyEvidence `json:"custody"`
	SeedMembershipIDs []string                    `json:"seed_membership_ids"`
	EvidenceRole      string                      `json:"evidence_role"`
	Support           int                         `json:"support"`
}
type Tables struct {
	Calls             []Calls   `json:"calls"`
	SiblingCandidates []Sibling `json:"sibling_candidates"`
}
type Artifact struct {
	SchemaVersion string `json:"schema_version"`
	Policy        string `json:"policy"`
	ParentBytes   []byte `json:"parent_bytes"`
	ParentDigest  string `json:"parent_digest"`
	SessionID     string `json:"session_id"`
	Generation    uint64 `json:"generation"`
	Tables        Tables `json:"tables"`
}
type native struct {
	Edges             []graph.Edge             `json:"edges"`
	SiblingCandidates []graph.SiblingCandidate `json:"sibling_candidates"`
	SeedMemberships   []graph.SeedMembership   `json:"seed_memberships"`
}

func digest(raw []byte) string { s := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(s[:]) }
func tables(n native) (Tables, error) {
	if len(n.Edges)+len(n.SiblingCandidates) > MaxRelations {
		return Tables{}, errors.New("retained-relations relation limit")
	}
	out := Tables{Calls: []Calls{}, SiblingCandidates: []Sibling{}}
	for _, e := range n.Edges {
		out.Calls = append(out.Calls, Calls{
			RelationID: e.RelationID, ExecutionBundleID: e.ExecutionBundleID,
			CallerNodeID: e.CallerNodeID, CalleeNodeID: e.CalleeNodeID,
			CallSites: append([]graph.Range{}, e.CallSites...), EvidenceRole: "CALL_SUPPORT", Support: 1,
		})
	}
	for _, s := range n.SiblingCandidates {
		if s.Declaration == nil {
			return Tables{}, errors.New("retained-relations sibling declaration required")
		}
		ids := []string{}
		for _, m := range n.SeedMemberships {
			if m.EvidenceKind == "SIBLING_CANDIDATE" && m.EndpointID == s.RelationID {
				ids = append(ids, m.MembershipID)
			}
		}
		sort.Strings(ids)
		out.SiblingCandidates = append(out.SiblingCandidates, Sibling{
			RelationID: s.RelationID, ExecutionBundleID: s.ExecutionBundleID,
			SeedURI: s.SeedURI, SeedLabel: s.SeedLabel, SeedIdentity: s.SeedIdentity,
			Direction: s.Direction, Kind: s.Kind, Origin: s.Origin, Declaration: *s.Declaration,
			PreparedCandidate: s.Candidate, ProviderEvidence: append([]string{}, s.ProviderEvidence...),
			LSPEvidence: append([]string{}, s.LSPEvidence...), SourceDigests: append([]string{}, s.SourceDigests...),
			Custody: s.Custody, SeedMembershipIDs: ids, EvidenceRole: "DISCOVERY_ONLY", Support: 0,
		})
	}
	return out, nil
}
func Export(input []byte) ([]byte, error) {
	if len(input) > MaxInputBytes {
		return nil, errors.New("retained-relations input byte limit")
	}
	if v, err := graphprovenance.ValidateFor(input, graphprovenance.Family, "v5"); err != nil || v != graphprovenance.VersionV5 {
		return nil, fmt.Errorf("verified graph-provenance v5 required: %w", err)
	}
	var p graphprovenance.EvidenceV5
	if err := json.Unmarshal(input, &p); err != nil {
		return nil, err
	}
	nativeBytes, err := base64.StdEncoding.DecodeString(p.GraphV5)
	if err != nil {
		return nil, err
	}
	var n native
	d := json.NewDecoder(bytes.NewReader(nativeBytes))
	if err = d.Decode(&n); err != nil {
		return nil, err
	}
	t, err := tables(n)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(Artifact{Version, Policy, append([]byte{}, input...), digest(input), p.SessionID, p.Generation, t})
	if err != nil {
		return nil, err
	}
	if len(raw)+1 > MaxOutputBytes {
		return nil, errors.New("retained-relations output byte limit")
	}
	raw = append(raw, '\n')
	if _, err = Validate(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
func Validate(raw []byte) (string, error) {
	return ValidateFor(raw, Family, "v1")
}

func ValidateFor(raw []byte, family, version string) (string, error) {
	if len(raw) > MaxOutputBytes {
		return "", errors.New("retained-relations output byte limit")
	}
	if family != Family && family != schema.FamilyRetainedRelations {
		return "", fmt.Errorf("retained-relations family mismatch: %q", family)
	}
	if _, err := schema.ValidateStructure(raw, schema.FamilyRetainedRelations, version); err != nil {
		return "", err
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return "", err
	}
	var a Artifact
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return "", err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return "", errors.New("retained-relations trailing JSON")
	}
	if a.SchemaVersion != Version || a.Policy != Policy || a.ParentDigest != digest(a.ParentBytes) {
		return "", errors.New("retained-relations identity mismatch")
	}
	if _, err := graphprovenance.ValidateFor(a.ParentBytes, graphprovenance.Family, "v5"); err != nil {
		return "", err
	}
	var p graphprovenance.EvidenceV5
	if err := json.Unmarshal(a.ParentBytes, &p); err != nil {
		return "", err
	}
	if p.SessionID != a.SessionID || p.Generation != a.Generation {
		return "", errors.New("retained-relations parent custody mismatch")
	}
	nb, _ := base64.StdEncoding.DecodeString(p.GraphV5)
	var n native
	if err := json.Unmarshal(nb, &n); err != nil {
		return "", err
	}
	want, err := tables(n)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(want, a.Tables) {
		return "", errors.New("retained-relations derived tables mismatch")
	}
	return Version, nil
}
