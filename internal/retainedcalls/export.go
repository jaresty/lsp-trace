// Package retainedcalls exports retained CALLS occurrences, not acquisition events
// or authenticated semantic identities. Historical relation IDs remain group keys.
package retainedcalls

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"unicode/utf8"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
)

const Family = "retained-calls"
const Version = "lsp-trace.retained-calls.v1"
const MaxBytes = 128 << 20
const MaxRows = 50000

// Context records bundle-scoped history. Envelope excludes graph_bytes and source
// tables; Bundle excludes the separately reconstructed graph tables.
type Context struct {
	ID          string          `json:"id"`
	GraphDigest string          `json:"graph_digest"`
	Envelope    json.RawMessage `json:"envelope"`
	Bundle      json.RawMessage `json:"bundle"`
}
type Group struct {
	ContextID             string                 `json:"context_id"`
	RelationID            string                 `json:"relation_id"`
	ExecutionBundleID     string                 `json:"execution_bundle_id"`
	CallerNodeID          string                 `json:"caller_node_id"`
	CalleeNodeID          string                 `json:"callee_node_id"`
	Pointer               string                 `json:"pointer"`
	CallsiteState         string                 `json:"callsite_state"`
	OccurrenceIDs         []string               `json:"occurrence_ids"`
	Receipt               graph.EvidenceRelation `json:"receipt"`
	Memberships           []graph.SeedMembership `json:"memberships"`
	OutgoingParticipation bool                   `json:"outgoing_participation"`
}
type Occurrence struct {
	ID           string                  `json:"id"`
	ContextID    string                  `json:"context_id"`
	RelationID   string                  `json:"relation_id"`
	CallerNodeID string                  `json:"caller_node_id"`
	CalleeNodeID string                  `json:"callee_node_id"`
	CallerURI    string                  `json:"caller_uri"`
	Range        graph.Range             `json:"range"`
	Binding      graphprovenance.Binding `json:"binding"`
}
type Tables struct {
	Contexts        []Context                 `json:"contexts"`
	Endpoints       []graph.Node              `json:"endpoints"`
	Groups          []Group                   `json:"groups"`
	Occurrences     []Occurrence              `json:"occurrences"`
	NodeMemberships []graph.SeedMembership    `json:"node_memberships"`
	Bindings        []graphprovenance.Binding `json:"bindings"`
	Supply          *graphprovenance.Receipt  `json:"supply"`
	Captures        []graphprovenance.Receipt `json:"captures"`
	SupportTotal    int                       `json:"support_total"`
}
type Ceilings struct {
	AnalyzedVersion        string `json:"analyzed_version"`
	DependencyCompleteness string `json:"dependency_completeness"`
	AcquisitionMethod      string `json:"per_callsite_acquisition_method"`
	RequestID              string `json:"per_callsite_request_id"`
	ResponseID             string `json:"per_callsite_response_id"`
	ProviderVersion        string `json:"authenticated_provider_version"`
	ProviderQualification  string `json:"provider_qualification"`
	SourceAuthentication   string `json:"analyzed_source_authentication"`
	IndependentSupport     string `json:"independent_numeric_support"`
	RepeatedReports        string `json:"repeated_identical_report_count"`
	IncomingSuccesses      string `json:"incoming_request_successes_scope"`
	SupplyMethod           string `json:"supply_method_scope"`
	Forgery                string `json:"coherent_public_forgery"`
}

func ceilings() Ceilings {
	return Ceilings{graphprovenance.Unverified, "UNKNOWN_INCOMPLETE", "UNAVAILABLE", "UNAVAILABLE", "UNAVAILABLE", "UNAVAILABLE", "UNAVAILABLE", "UNAVAILABLE", "UNAVAILABLE", "UNAVAILABLE", "BUNDLE_DERIVED_METRIC_NOT_REQUEST_EVIDENCE", "DOCUMENT_NOTIFICATION_NOT_CALL_ACQUISITION", "NOT_INDEPENDENTLY_REAUTHENTICATED"}
}

type Evidence struct {
	SchemaVersion string   `json:"schema_version"`
	InputBytes    []byte   `json:"input_bytes"`
	InputDigest   string   `json:"input_digest"`
	Tables        Tables   `json:"tables"`
	Ceilings      Ceilings `json:"ceilings"`
}

func digest(domain string, raw []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(append(append([]byte(domain), 0), raw...)))
}

// canonical uses decoded JSON objects with lexically sorted keys, array order
// retained, Go encoding/json escaping and compact numbers. No raw JSON escapes or
// whitespace survive; embedded byte strings (in particular graph_bytes) do.
func canonical(v any) []byte {
	raw, _ := json.Marshal(v)
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&decoded)
	raw, _ = json.Marshal(decoded)
	return raw
}
func occurrenceID(o Occurrence) string {
	return digest(Version+":occurrence", canonical([]any{o.ContextID, o.RelationID, o.CallerNodeID, o.CalleeNodeID, o.CallerURI, o.Range}))
}
func Export(input []byte) ([]byte, error) {
	if len(input) > graphprovenance.MaxEnvelopeBytes {
		return nil, errors.New("provenance envelope byte limit")
	}
	if !utf8.Valid(input) {
		return nil, errors.New("invalid UTF-8 input")
	}
	if _, err := graphprovenance.ValidateFor(input, graphprovenance.Family, "v1"); err != nil {
		return nil, err
	}
	var e graphprovenance.Evidence
	if err := json.Unmarshal(input, &e); err != nil {
		return nil, err
	}
	tables, err := extractTables(e)
	if err != nil {
		return nil, err
	}
	out := Evidence{Version, append([]byte(nil), input...), digest(Version+":input", input), tables, ceilings()}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxBytes {
		return nil, errors.New("retained calls byte budget exceeded")
	}
	return append(raw, '\n'), nil
}

type native struct {
	Nodes       []graph.Node           `json:"nodes"`
	Edges       []graph.Edge           `json:"edges"`
	Receipt     *graph.EvidenceReceipt `json:"evidence_receipt"`
	Memberships []graph.SeedMembership `json:"seed_memberships"`
	Slice       *graph.SliceEvidence   `json:"slice"`
}

func extractTables(e graphprovenance.Evidence) (Tables, error) {
	var n native
	if err := json.Unmarshal(e.GraphBytes, &n); err != nil {
		return Tables{}, err
	}
	contextID := digest(Version+":context", canonical(e))
	var envelope, bundle map[string]any
	envelopeDecoder := json.NewDecoder(bytes.NewReader(canonical(e)))
	envelopeDecoder.UseNumber()
	_ = envelopeDecoder.Decode(&envelope)
	bundleDecoder := json.NewDecoder(bytes.NewReader(e.GraphBytes))
	bundleDecoder.UseNumber()
	_ = bundleDecoder.Decode(&bundle)
	for _, k := range []string{"graph_bytes", "bindings", "supply", "captures"} {
		delete(envelope, k)
	}
	for _, k := range []string{"nodes", "edges", "evidence_receipt", "seed_memberships"} {
		delete(bundle, k)
	}
	t := Tables{Contexts: []Context{{contextID, e.GraphDigest, canonical(envelope), canonical(bundle)}}, Endpoints: n.Nodes, Groups: []Group{}, Occurrences: []Occurrence{}, NodeMemberships: []graph.SeedMembership{}, Bindings: e.Bindings, Supply: e.Supply, Captures: e.Captures}
	receipts := map[string]graph.EvidenceRelation{}
	if n.Receipt != nil {
		t.SupportTotal = n.Receipt.SupportTotal
		for _, r := range n.Receipt.Relations {
			receipts[r.RelationID] = r
		}
	}
	memberships := map[string][]graph.SeedMembership{}
	for _, m := range n.Memberships {
		if m.EvidenceKind == "CALL_RELATION" {
			memberships[m.EndpointID] = append(memberships[m.EndpointID], m)
		} else {
			t.NodeMemberships = append(t.NodeMemberships, m)
		}
	}
	uris := map[string]string{}
	for _, n := range n.Nodes {
		uris[n.ID] = n.URI
	}
	bindings := map[string]graphprovenance.Binding{}
	for _, b := range e.Bindings {
		bindings[b.Pointer] = b
	}
	outgoing := map[string]bool{}
	if n.Slice != nil {
		for _, id := range n.Slice.OutgoingRelationIDs {
			outgoing[id] = true
		}
	}
	seenGroups := map[string]bool{}
	for i, edge := range n.Edges {
		if seenGroups[edge.RelationID] {
			return Tables{}, errors.New("duplicate historical relation group")
		}
		seenGroups[edge.RelationID] = true
		g := Group{ContextID: contextID, RelationID: edge.RelationID, ExecutionBundleID: edge.ExecutionBundleID, CallerNodeID: edge.CallerNodeID, CalleeNodeID: edge.CalleeNodeID, Pointer: fmt.Sprintf("/edges/%d", i), CallsiteState: "UNREPORTED", OccurrenceIDs: []string{}, Receipt: receipts[edge.RelationID], Memberships: append([]graph.SeedMembership{}, memberships[edge.RelationID]...), OutgoingParticipation: outgoing[edge.RelationID]}
		seen := map[graph.Range]bool{}
		for j, site := range edge.CallSites {
			if seen[site] {
				return Tables{}, errors.New("non-distinct retained callsite ranges")
			}
			seen[site] = true
			o := Occurrence{ContextID: contextID, RelationID: edge.RelationID, CallerNodeID: edge.CallerNodeID, CalleeNodeID: edge.CalleeNodeID, CallerURI: uris[edge.CallerNodeID], Range: site, Binding: bindings[fmt.Sprintf("%s/call_sites/%d", g.Pointer, j)]}
			o.ID = occurrenceID(o)
			g.OccurrenceIDs = append(g.OccurrenceIDs, o.ID)
			t.Occurrences = append(t.Occurrences, o)
		}
		if len(g.OccurrenceIDs) > 0 {
			g.CallsiteState = "RETAINED_DISTINCT_RANGES"
		}
		t.Groups = append(t.Groups, g)
	}
	if len(t.Groups) > MaxRows || len(t.Occurrences) > MaxRows || len(t.Endpoints) > MaxRows {
		return Tables{}, errors.New("retained calls row budget exceeded")
	}
	return t, nil
}

// Projection is reconstructed from tables alone, without input or graph bytes.
// Receipt support remains once per historical relation, independent of ranges.
type Projection struct {
	Endpoints   []graph.Node
	Edges       []graph.Edge
	Receipt     *graph.EvidenceReceipt
	Memberships []graph.SeedMembership
	Bindings    []graphprovenance.Binding
	Supply      *graphprovenance.Receipt
	Captures    []graphprovenance.Receipt
}

func Reconstruct(t Tables) (Projection, error) {
	if len(t.Contexts) != 1 || len(t.Groups) > MaxRows || len(t.Occurrences) > MaxRows || len(t.Endpoints) > MaxRows || len(t.Bindings) > MaxRows || len(t.Captures) > MaxRows || len(t.NodeMemberships) > MaxRows {
		return Projection{}, errors.New("table context or row budget")
	}
	p := Projection{Endpoints: t.Endpoints, Edges: []graph.Edge{}, Memberships: append([]graph.SeedMembership{}, t.NodeMemberships...), Bindings: t.Bindings, Supply: t.Supply, Captures: t.Captures}
	nodes := map[string]graph.Node{}
	for _, n := range t.Endpoints {
		if _, ok := nodes[n.ID]; ok {
			return Projection{}, errors.New("duplicate endpoint")
		}
		nodes[n.ID] = n
	}
	occurrences := map[string]Occurrence{}
	for _, o := range t.Occurrences {
		if _, ok := occurrences[o.ID]; ok {
			return Projection{}, errors.New("duplicate occurrence")
		}
		if o.ID != occurrenceID(o) {
			return Projection{}, errors.New("occurrence identity mismatch")
		}
		occurrences[o.ID] = o
	}
	used := map[string]bool{}
	groups := map[string]bool{}
	receipts := map[string]graphprovenance.Receipt{}
	sourceReceipts := append([]graphprovenance.Receipt{}, t.Captures...)
	if t.Supply != nil {
		sourceReceipts = append(sourceReceipts, *t.Supply)
	}
	for _, r := range sourceReceipts {
		if _, ok := receipts[r.ID]; ok {
			return Projection{}, errors.New("duplicate source receipt")
		}
		receipts[r.ID] = r
	}
	bindings := map[string]graphprovenance.Binding{}
	for _, b := range t.Bindings {
		if b.Attribution == "SOURCE" && len(b.ReceiptIDs) == 0 {
			return Projection{}, errors.New("missing source receipt join")
		}
		for _, id := range b.ReceiptIDs {
			r, ok := receipts[id]
			if !ok || r.URI != b.URI {
				return Projection{}, errors.New("source receipt URI join mismatch")
			}
		}
		if _, ok := bindings[b.Pointer]; ok {
			return Projection{}, errors.New("duplicate binding")
		}
		bindings[b.Pointer] = b
	}
	relations := []graph.EvidenceRelation{}
	support := 0
	for _, g := range t.Groups {
		if groups[g.RelationID] || g.ContextID != t.Contexts[0].ID {
			return Projection{}, errors.New("duplicate group or context mismatch")
		}
		groups[g.RelationID] = true
		caller, cok := nodes[g.CallerNodeID]
		_, eok := nodes[g.CalleeNodeID]
		if !cok || !eok {
			return Projection{}, errors.New("missing endpoint")
		}
		edge := graph.Edge{RelationID: g.RelationID, ExecutionBundleID: g.ExecutionBundleID, CallerNodeID: g.CallerNodeID, CalleeNodeID: g.CalleeNodeID, CallSites: []graph.Range{}}
		if (len(g.OccurrenceIDs) == 0 && g.CallsiteState != "UNREPORTED") || (len(g.OccurrenceIDs) > 0 && g.CallsiteState != "RETAINED_DISTINCT_RANGES") {
			return Projection{}, errors.New("callsite classification mismatch")
		}
		sites := map[graph.Range]bool{}
		for j, id := range g.OccurrenceIDs {
			o, ok := occurrences[id]
			if !ok || used[id] || sites[o.Range] || o.ContextID != g.ContextID || o.RelationID != g.RelationID || o.CallerNodeID != g.CallerNodeID || o.CalleeNodeID != g.CalleeNodeID || o.CallerURI != caller.URI {
				return Projection{}, errors.New("occurrence group join mismatch")
			}
			if o.Binding.Pointer != fmt.Sprintf("%s/call_sites/%d", g.Pointer, j) || o.Binding.URI != caller.URI || !reflect.DeepEqual(bindings[o.Binding.Pointer], o.Binding) {
				return Projection{}, errors.New("occurrence source binding mismatch")
			}
			used[id] = true
			sites[o.Range] = true
			edge.CallSites = append(edge.CallSites, o.Range)
		}
		sort.Slice(edge.CallSites, func(i, j int) bool { return lessRange(edge.CallSites[i], edge.CallSites[j]) })
		p.Edges = append(p.Edges, edge)
		if g.Receipt.RelationID != g.RelationID || g.Receipt.CallerNodeID != g.CallerNodeID || g.Receipt.CalleeNodeID != g.CalleeNodeID {
			return Projection{}, errors.New("group receipt mismatch")
		}
		relations = append(relations, g.Receipt)
		support += g.Receipt.SupportContribution
		for _, m := range g.Memberships {
			if m.EndpointID != g.RelationID || m.EvidenceKind != "CALL_RELATION" || m.ExecutionBundleID != g.ExecutionBundleID {
				return Projection{}, errors.New("group membership mismatch")
			}
			p.Memberships = append(p.Memberships, m)
		}
	}
	if len(used) != len(occurrences) || support != t.SupportTotal {
		return Projection{}, errors.New("unused occurrence or support mismatch")
	}
	if len(relations) > 0 {
		sort.Slice(relations, func(i, j int) bool { return relations[i].RelationID < relations[j].RelationID })
		p.Receipt = &graph.EvidenceReceipt{SupportTotal: support, Relations: relations}
	}
	sort.Slice(p.Memberships, func(i, j int) bool { return p.Memberships[i].MembershipID < p.Memberships[j].MembershipID })
	return p, nil
}
func lessRange(a, b graph.Range) bool {
	x, y := [4]uint32{a.Start.Line, a.Start.Character, a.End.Line, a.End.Character}, [4]uint32{b.Start.Line, b.Start.Character, b.End.Line, b.End.Character}
	for i := range x {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return false
}
func ValidateFor(raw []byte, family, version string) (string, error) {
	if family != Family {
		return graphprovenance.ValidateFor(raw, family, version)
	}
	if len(raw) > MaxBytes {
		return "", errors.New("retained calls byte budget exceeded")
	}
	if !utf8.Valid(raw) {
		return "", errors.New("invalid UTF-8 export")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return "", err
	}
	admitted, err := schema.ValidateStructure(raw, family, version)
	if err != nil {
		return "", err
	}
	var e Evidence
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&e); err != nil {
		return "", err
	}
	if !bytes.Equal(canonical(json.RawMessage(raw)), canonical(e)) {
		return "", errors.New("noncanonical field names or missing table fields")
	}
	if e.SchemaVersion != Version || e.Ceilings != ceilings() || e.InputDigest != digest(Version+":input", e.InputBytes) {
		return "", errors.New("retained calls version/ceiling/input digest mismatch")
	}
	if !utf8.Valid(e.InputBytes) {
		return "", errors.New("invalid UTF-8 embedded input")
	}
	if _, err = graphprovenance.ValidateFor(e.InputBytes, graphprovenance.Family, "v1"); err != nil {
		return "", err
	}
	var input graphprovenance.Evidence
	_ = json.Unmarshal(e.InputBytes, &input)
	want, err := extractTables(input)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(canonical(want), canonical(e.Tables)) {
		return "", errors.New("retained calls table projection mismatch")
	}
	got, err := Reconstruct(e.Tables)
	if err != nil {
		return "", err
	}
	// Independent native extraction, not a second invocation of table export.
	var n native
	if err = json.Unmarshal(input.GraphBytes, &n); err != nil {
		return "", err
	}
	if n.Edges == nil {
		n.Edges = []graph.Edge{}
	}
	if n.Memberships == nil {
		n.Memberships = []graph.SeedMembership{}
	}
	expected := Projection{n.Nodes, n.Edges, n.Receipt, n.Memberships, input.Bindings, input.Supply, input.Captures}
	if !bytes.Equal(canonical(expected), canonical(got)) {
		return "", errors.New("tables-only roundtrip mismatch")
	}
	return admitted.Version, nil
}
