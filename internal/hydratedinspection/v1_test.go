package hydratedinspection

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	gp "lsp-trace/internal/graphprovenance"
)

// A separately labeled offline V1 derivation of the pinned V2 fixture. No source
// file is opened, no acquisition is run, and the committed original is unchanged.
// Existing native graph canonicalization/census supply the zero-site distinction.
func TestPublicV1ZeroSiteAndLegacyParity(t *testing.T) {
	original := fixture(t)
	var env gp.EvidenceV2
	if err := json.Unmarshal([]byte(original.Input), &env); err != nil {
		t.Fatal(err)
	}
	g, err := graph.DecodeNativeV3(env.GraphBytes)
	if err != nil {
		t.Fatal(err)
	}
	g.Edges[0].CallSites = []graph.Range{}
	n := g.Nodes[0]
	g.Invocation.Target = graph.Target{URI: n.URI}
	g.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: n.URI + ":0:0", ResolvedURI: n.URI}}
	nodeIDs, relationIDs := []string{}, []string{}
	for _, node := range g.Nodes {
		nodeIDs = append(nodeIDs, node.ID)
	}
	for _, relation := range g.Edges {
		relationIDs = append(relationIDs, relation.RelationID)
	}
	g.Seeds = []graph.SeedResult{{Label: "start", Requested: g.Invocation.Target, PreparedTargetIDs: []string{n.ID}, ReachedNodeIDs: nodeIDs, ReachedRelationIDs: relationIDs}}
	g.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: n.URI, StartingNodeIDs: []string{n.ID}, Layers: []graph.SliceLayer{{NodeIDs: []string{n.ID}}}, FrontierNodeIDs: []string{n.ID}, UpwardStartNodeIDs: []string{n.ID}, OutgoingRelationIDs: relationIDs, TraversalComplete: true}
	g.Canonicalize()
	raw := jsonBytes(t, g)
	bindings, err := gp.Census(raw)
	if err != nil {
		t.Fatal(err)
	}
	hash := func(domain string, b []byte) string {
		return fmt.Sprintf("sha256:%x", sha256.Sum256(append(append([]byte(domain), 0), b...)))
	}
	var header struct {
		Invocation struct {
			Target struct {
				URI string `json:"uri"`
			} `json:"target"`
		} `json:"invocation"`
	}
	if err = json.Unmarshal(raw, &header); err != nil {
		t.Fatal(err)
	}
	e := gp.Evidence{SchemaVersion: gp.Version, GraphBytes: raw, GraphDigest: hash(gp.Version+":graph", raw), WorkspaceURI: env.WorkspaceURI, SeedURI: header.Invocation.Target.URI, SessionID: "offline-derived-v1", Generation: 1, AnalyzedVersion: gp.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", SupplyStatus: "NO_NOTIFICATION_OBSERVATION", Captures: []gp.Receipt{}, Bindings: bindings}
	uris := map[string]bool{}
	for _, b := range bindings {
		if b.Attribution == "SOURCE" {
			uris[b.URI] = true
		}
	}
	byURI := map[string]string{}
	for _, c := range env.Captures {
		if uris[c.URI] {
			c.ID = ""
			c.ID = hash(gp.Version+":receipt", jsonBytes(t, c))
			e.Captures = append(e.Captures, c)
			byURI[c.URI] = c.ID
		}
	}
	sort.Slice(e.Captures, func(i, j int) bool { return e.Captures[i].URI < e.Captures[j].URI })
	for i := range e.Bindings {
		e.Bindings[i].ReceiptIDs = []string{}
		if e.Bindings[i].Attribution == "SOURCE" {
			e.Bindings[i].ReceiptIDs = []string{byURI[e.Bindings[i].URI]}
		}
	}
	bytes := jsonBytes(t, e)
	if _, err = gp.ValidateFor(bytes, gp.Family, "v1"); err != nil {
		t.Fatal(err)
	}
	r := DefaultRequest()
	r.Input = string(bytes)
	r.NodeIDs = []string{g.Nodes[0].ID}
	r.RelationIDs = []string{g.Edges[0].RelationID}
	r.IncludeBodies = true
	r.PositionEncoding = "utf-8"
	v := inspect(t, r)
	if !v.Bundle.Complete || len(v.Manifest.Origins) != 2 || v.Manifest.Origins[1].Status != "NO_CALL_SITES" || len(v.Manifest.Origins[1].Sites) != 0 {
		t.Fatal("PUBLIC_V1_ZERO_SITE FAIL")
	}
	text, err := Text(r, v)
	if err != nil || !strings.Contains(text, "NO_CALL_SITES") {
		t.Fatal("PUBLIC_V1_ZERO_SITE FAIL", err)
	}
	if err = ValidateFull(r, v); err != nil {
		t.Fatal(err)
	}
	t.Log("PUBLIC_V1_ZERO_SITE PASS: zero-site manifest remains visible while Bundle.Complete=true; separately derived V1 fixture, no acquisition")
}
