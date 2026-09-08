package retainedcalls

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedpath"
	"lsp-trace/internal/schema"
)

const VersionV2 = "lsp-trace.retained-calls.v2"
const PolicyV2 = "EXACT_ENVELOPE_NATIVE_GROUP_DISTINCT_SITE_TYPED_ACQUISITION_V2"

// These are export limits, not coordinator limits or future analysis limits.
const MaxBytesV2 = 384 << 20
const MaxNodesV2 = 4096
const MaxGroupsV2 = 8192
const MaxNativeBytesV2 = 8 << 20

// LimitErrorV2 rejects the entire input; export never truncates evidence.
type LimitErrorV2 struct {
	Resource string
	Limit    int
}

func (e *LimitErrorV2) Error() string {
	return fmt.Sprintf("retained-calls V2 %s limit %d", e.Resource, e.Limit)
}

// FieldV2 preserves an explicitly named native non-CALLS field. No row contains
// graph_bytes or a hidden second graph. Values retain JSON number lexemes.
type FieldV2 struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}
type ContextV2 struct {
	ID            string                          `json:"id"`
	InputDigest   string                          `json:"input_digest"`
	GraphDigest   string                          `json:"graph_digest"`
	WorkspaceURI  string                          `json:"workspace_uri"`
	CaptureBudget graphprovenance.CaptureBudgetV2 `json:"capture_budget"`
}
type GroupV2 struct {
	ID                string                 `json:"id"`
	NativeRelationID  string                 `json:"native_relation_id"`
	NativePointer     string                 `json:"native_pointer"`
	ExecutionBundleID string                 `json:"execution_bundle_id"`
	CallerNodeID      string                 `json:"caller_node_id"`
	CalleeNodeID      string                 `json:"callee_node_id"`
	CallsiteState     string                 `json:"callsite_state"`
	OccurrenceIDs     []string               `json:"occurrence_ids"`
	Receipt           graph.EvidenceRelation `json:"receipt"`
}
type OccurrenceV2 struct {
	ID             string      `json:"id"`
	GroupID        string      `json:"group_id"`
	NativePointer  string      `json:"native_pointer"`
	BindingPointer string      `json:"binding_pointer"`
	Range          graph.Range `json:"range"`
}

// ConnectionV2 retains native node IDs and distinctly names exported IDs. The
// original native witness remains in Targets; these are explicit foreign keys.
type ConnectionV2 struct {
	TargetID      string     `json:"target_id"`
	Nodes         []string   `json:"nodes"`
	GroupIDs      []string   `json:"export_group_ids"`
	OccurrenceIDs [][]string `json:"export_occurrence_ids"`
}
type AcquisitionV2 struct {
	Policy              string                        `json:"policy"`
	PathProjection      string                        `json:"path_projection"`
	Request             acquisition.Request           `json:"request"`
	Targets             []acquisition.TargetResult    `json:"targets"`
	Requests            []acquisition.RequestRecord   `json:"requests"`
	Supplies            []acquisition.Supply          `json:"supplies"`
	EdgeObservations    []acquisition.EdgeObservation `json:"edge_observations"`
	Usage               acquisition.Usage             `json:"usage"`
	AcquisitionComplete bool                          `json:"acquisition_complete"`
}
type TablesV2 struct {
	Context               ContextV2                         `json:"context"`
	Endpoints             []graph.Node                      `json:"endpoints"`
	Groups                []GroupV2                         `json:"groups"`
	Occurrences           []OccurrenceV2                    `json:"occurrences"`
	NativeNullArrays      []string                          `json:"native_null_arrays"`
	NativeFields          []FieldV2                         `json:"native_fields"`
	Memberships           []graph.SeedMembership            `json:"memberships"`
	ReceiptPresent        bool                              `json:"receipt_present"`
	SupportTotal          int                               `json:"support_total"`
	AcquisitionNullArrays []string                          `json:"acquisition_null_arrays"`
	Acquisition           AcquisitionV2                     `json:"acquisition"`
	Connections           []ConnectionV2                    `json:"connections"`
	Bindings              []graphprovenance.BindingV2       `json:"bindings"`
	Supplies              []graphprovenance.SupplyReceiptV2 `json:"supplies"`
	Captures              []graphprovenance.Receipt         `json:"captures"`
}
type EvidenceV2 struct {
	SchemaVersion string   `json:"schema_version"`
	Policy        string   `json:"policy"`
	InputBytes    []byte   `json:"input_bytes"`
	InputDigest   string   `json:"input_digest"`
	Tables        TablesV2 `json:"tables"`
}

// ProjectionV2 is assembled exclusively from table rows. Result includes every
// requested target, coverage state and original native connection witness.
type ProjectionV2 struct {
	Result        acquisition.Result
	Bindings      []graphprovenance.BindingV2
	Supplies      []graphprovenance.SupplyReceiptV2
	Captures      []graphprovenance.Receipt
	WorkspaceURI  string
	CaptureBudget graphprovenance.CaptureBudgetV2
}

func contextIDV2(inputDigest string) string {
	return digest(VersionV2+":context", canonical([]string{PolicyV2, inputDigest}))
}
func groupIDV2(context string, g GroupV2) string {
	return digest(VersionV2+":group", canonical([]string{context, g.NativeRelationID, g.NativePointer, g.ExecutionBundleID, g.CallerNodeID, g.CalleeNodeID}))
}
func occurrenceIDV2(context string, o OccurrenceV2) string {
	return digest(VersionV2+":occurrence", canonical([]any{context, o.GroupID, o.NativePointer, o.BindingPointer, o.Range}))
}
func checkLimitsV2(e graphprovenance.EvidenceV2) error {
	if len(e.GraphBytes) > MaxNativeBytesV2 {
		return &LimitErrorV2{"native bytes", MaxNativeBytesV2}
	}
	if len(e.Acquisition.Graph.Nodes) > MaxNodesV2 {
		return &LimitErrorV2{"nodes", MaxNodesV2}
	}
	if len(e.Acquisition.Graph.Edges) > MaxGroupsV2 {
		return &LimitErrorV2{"groups", MaxGroupsV2}
	}
	return nil
}
func descriptorV2(r acquisition.Result) AcquisitionV2 {
	return AcquisitionV2{r.Policy, r.PathProjection, r.Request, append([]acquisition.TargetResult{}, r.Targets...), append([]acquisition.RequestRecord{}, r.Requests...), append([]acquisition.Supply{}, r.Supplies...), append([]acquisition.EdgeObservation{}, r.EdgeObservations...), r.Usage, r.AcquisitionComplete}
}
func (a AcquisitionV2) result(g graph.Result) acquisition.Result {
	// Replay's historical empty supply accumulator is nil, whereas the export
	// table contract requires an array. This restores allocation, not evidence.
	if len(a.Supplies) == 0 {
		a.Supplies = nil
	}
	if len(a.EdgeObservations) == 0 {
		a.EdgeObservations = nil
	}
	return acquisition.Result{Policy: a.Policy, PathProjection: a.PathProjection, Request: a.Request, Graph: g, Targets: a.Targets, Requests: a.Requests, Supplies: a.Supplies, EdgeObservations: a.EdgeObservations, Usage: a.Usage, AcquisitionComplete: a.AcquisitionComplete}
}
func extractTablesV2(e graphprovenance.EvidenceV2, inputDigest string) (TablesV2, error) {
	t := TablesV2{Context: ContextV2{contextIDV2(inputDigest), inputDigest, e.GraphDigest, e.WorkspaceURI, e.CaptureBudget}, Endpoints: append([]graph.Node{}, e.Acquisition.Graph.Nodes...), Groups: []GroupV2{}, Occurrences: []OccurrenceV2{}, NativeFields: []FieldV2{}, Memberships: []graph.SeedMembership{}, Acquisition: descriptorV2(e.Acquisition), Connections: []ConnectionV2{}, Bindings: e.Bindings, Supplies: e.Supplies, Captures: e.Captures}
	normalized, markers, err := normalizeArraysV2(t.Acquisition)
	if err != nil {
		return t, err
	}
	t.Acquisition, t.AcquisitionNullArrays = normalized, markers
	var n native
	if err := json.Unmarshal(e.GraphBytes, &n); err != nil {
		return t, err
	}
	t.Memberships = append(t.Memberships, n.Memberships...)
	receipts := map[string]graph.EvidenceRelation{}
	if n.Receipt != nil {
		t.ReceiptPresent = true
		t.SupportTotal = n.Receipt.SupportTotal
		for _, r := range n.Receipt.Relations {
			receipts[r.RelationID] = r
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(e.GraphBytes, &fields); err != nil {
		return t, err
	}
	t.NativeNullArrays = []string{}
	for _, k := range []string{"nodes", "edges", "evidence_receipt", "seed_memberships"} {
		if (k == "nodes" || k == "edges") && bytes.Equal(fields[k], []byte("null")) {
			t.NativeNullArrays = append(t.NativeNullArrays, k)
		}
		delete(fields, k)
	}
	sort.Strings(t.NativeNullArrays)
	names := []string{}
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.NativeFields = append(t.NativeFields, FieldV2{name, fields[name]})
	}
	for i, edge := range n.Edges {
		g := GroupV2{NativeRelationID: edge.RelationID, NativePointer: fmt.Sprintf("/edges/%d", i), ExecutionBundleID: edge.ExecutionBundleID, CallerNodeID: edge.CallerNodeID, CalleeNodeID: edge.CalleeNodeID, CallsiteState: "UNREPORTED", OccurrenceIDs: []string{}, Receipt: receipts[edge.RelationID]}
		g.ID = groupIDV2(t.Context.ID, g)
		for j, site := range edge.CallSites {
			pointer := fmt.Sprintf("%s/call_sites/%d", g.NativePointer, j)
			o := OccurrenceV2{GroupID: g.ID, NativePointer: pointer, BindingPointer: "/graph" + pointer, Range: site}
			o.ID = occurrenceIDV2(t.Context.ID, o)
			t.Occurrences = append(t.Occurrences, o)
			g.OccurrenceIDs = append(g.OccurrenceIDs, o.ID)
		}
		if len(g.OccurrenceIDs) > 0 {
			g.CallsiteState = "RETAINED_DISTINCT_RANGES"
		}
		t.Groups = append(t.Groups, g)
	}
	t.Connections, err = mapConnectionsV2(t.Acquisition.Targets, t.Groups, t.Occurrences)
	return t, err
}
func mapConnectionsV2(targets []acquisition.TargetResult, groups []GroupV2, occurrences []OccurrenceV2) ([]ConnectionV2, error) {
	gs := map[string]string{}
	os := map[string]string{}
	for _, g := range groups {
		gs[g.NativeRelationID] = g.ID
	}
	for _, o := range occurrences {
		os[o.NativePointer] = o.ID
	}
	out := []ConnectionV2{}
	for _, target := range targets {
		p := target.Connection.Path
		c := ConnectionV2{TargetID: target.Requested.ID, Nodes: append([]string{}, p.Nodes...), GroupIDs: []string{}, OccurrenceIDs: [][]string{}}
		for _, id := range p.GroupIDs {
			mapped, ok := gs[id]
			if !ok {
				return nil, errors.New("V2 missing native witness group")
			}
			c.GroupIDs = append(c.GroupIDs, mapped)
		}
		for _, ids := range p.OccurrenceIDs {
			mapped := []string{}
			for _, id := range ids {
				v, ok := os[id]
				if !ok {
					return nil, errors.New("V2 missing native witness occurrence")
				}
				mapped = append(mapped, v)
			}
			c.OccurrenceIDs = append(c.OccurrenceIDs, mapped)
		}
		out = append(out, c)
	}
	return out, nil
}

// ExportV2 admits only explicit graph-provenance V2. The exact original envelope
// bytes bind the context, never a re-marshaled or graph-only approximation.
func ExportV2(input []byte) ([]byte, error) {
	if len(input) > graphprovenance.MaxEnvelopeBytesV2 {
		return nil, &LimitErrorV2{"input bytes", graphprovenance.MaxEnvelopeBytesV2}
	}
	if _, err := graphprovenance.ValidateFor(input, graphprovenance.Family, "v2"); err != nil {
		return nil, err
	}
	var e graphprovenance.EvidenceV2
	if err := json.Unmarshal(input, &e); err != nil {
		return nil, err
	}
	if err := checkLimitsV2(e); err != nil {
		return nil, err
	}
	id := digest(VersionV2+":input", input)
	tables, err := extractTablesV2(e, id)
	if err != nil {
		return nil, err
	}
	// Independent conservation check, not another call to the table producer.
	p, err := ReconstructV2(tables)
	if err != nil {
		return nil, err
	}
	if err = compareProjectionV2(e, p); err != nil {
		return nil, err
	}
	out, err := json.Marshal(EvidenceV2{VersionV2, PolicyV2, append([]byte(nil), input...), id, tables})
	if err != nil {
		return nil, err
	}
	if len(out)+1 > MaxBytesV2 {
		return nil, &LimitErrorV2{"output bytes", MaxBytesV2}
	}
	return append(out, '\n'), nil
}
func compareProjectionV2(e graphprovenance.EvidenceV2, p ProjectionV2) error {
	raw, err := json.Marshal(p.Result.Graph)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, e.GraphBytes) {
		return errors.New("V2 independent native reconstruction mismatch")
	}
	// Coordinator-native projection is extracted directly from the admitted input,
	// independently of group extraction, mapping production and table census.
	if !bytes.Equal(canonical(descriptorV2(e.Acquisition)), canonical(descriptorV2(p.Result))) || !bytes.Equal(canonical(e.Bindings), canonical(p.Bindings)) || !bytes.Equal(canonical(e.Supplies), canonical(p.Supplies)) || !bytes.Equal(canonical(e.Captures), canonical(p.Captures)) || e.WorkspaceURI != p.WorkspaceURI || e.CaptureBudget != p.CaptureBudget {
		return errors.New("V2 independent acquisition/source reconstruction mismatch")
	}
	return nil
}

// ReconstructV2 receives no embedded input or native graph bytes. It validates
// joins, reconstructs the native graph from explicit rows, and runs coordinator
// and provenance semantic admission on that reconstruction without source I/O.
func ReconstructV2(t TablesV2) (ProjectionV2, error) {
	fail := func(s string) (ProjectionV2, error) { return ProjectionV2{}, errors.New(s) }
	if len(t.Endpoints) > MaxNodesV2 {
		return ProjectionV2{}, &LimitErrorV2{"nodes", MaxNodesV2}
	}
	if len(t.Groups) > MaxGroupsV2 {
		return ProjectionV2{}, &LimitErrorV2{"groups", MaxGroupsV2}
	}
	if len(t.Occurrences) > MaxRows || len(t.Bindings) > MaxRows || len(t.Memberships) > 1000000 || len(t.NativeFields) > 128 || len(t.Captures) > MaxRows {
		return fail("V2 table row limit")
	}
	if t.Context.ID != contextIDV2(t.Context.InputDigest) {
		return fail("V2 context identity mismatch")
	}
	tableRaw, err := json.Marshal(t)
	if err != nil {
		return ProjectionV2{}, err
	}
	if err = preflightExportV2(tableRaw, MaxBytesV2); err != nil {
		return ProjectionV2{}, err
	}
	if t.Endpoints == nil || t.Groups == nil || t.Occurrences == nil || t.NativeFields == nil || t.Memberships == nil || t.Connections == nil || t.Bindings == nil || t.Supplies == nil || t.Captures == nil || t.Acquisition.Targets == nil || t.Acquisition.Requests == nil || t.Acquisition.Supplies == nil || t.Acquisition.EdgeObservations == nil {
		return fail("V2 table arrays must not be null")
	}
	fields := map[string]json.RawMessage{}
	previous := ""
	for _, f := range t.NativeFields {
		if f.Name <= previous || f.Name == "nodes" || f.Name == "edges" || f.Name == "evidence_receipt" || f.Name == "seed_memberships" {
			return fail("V2 invalid native projection field")
		}
		previous = f.Name
		fields[f.Name] = f.Value
	}
	put := func(name string, v any) { fields[name], _ = json.Marshal(v) }
	put("nodes", t.Endpoints)
	occurrences := map[string]OccurrenceV2{}
	for _, o := range t.Occurrences {
		if _, ok := occurrences[o.ID]; ok || o.ID != occurrenceIDV2(t.Context.ID, o) {
			return fail("V2 duplicate/invalid occurrence identity")
		}
		occurrences[o.ID] = o
	}
	nodes := map[string]graph.Node{}
	for _, n := range t.Endpoints {
		if _, ok := nodes[n.ID]; ok {
			return fail("V2 duplicate endpoint")
		}
		nodes[n.ID] = n
	}
	groups := map[string]bool{}
	used := map[string]bool{}
	bindings := map[string]graphprovenance.BindingV2{}
	for _, b := range t.Bindings {
		if _, ok := bindings[b.Pointer]; ok {
			return fail("V2 duplicate binding")
		}
		bindings[b.Pointer] = b
	}
	edges := []graph.Edge{}
	relations := []graph.EvidenceRelation{}
	support := 0
	for i, g := range t.Groups {
		if groups[g.NativeRelationID] || g.NativePointer != fmt.Sprintf("/edges/%d", i) || g.ID != groupIDV2(t.Context.ID, g) || g.OccurrenceIDs == nil {
			return fail("V2 group identity/order mismatch")
		}
		groups[g.NativeRelationID] = true
		caller, cok := nodes[g.CallerNodeID]
		_, eok := nodes[g.CalleeNodeID]
		if !cok || !eok {
			return fail("V2 missing endpoint")
		}
		edge := graph.Edge{RelationID: g.NativeRelationID, ExecutionBundleID: g.ExecutionBundleID, CallerNodeID: g.CallerNodeID, CalleeNodeID: g.CalleeNodeID, CallSites: []graph.Range{}}
		state := "UNREPORTED"
		if len(g.OccurrenceIDs) > 0 {
			state = "RETAINED_DISTINCT_RANGES"
		}
		if state != g.CallsiteState {
			return fail("V2 site state mismatch")
		}
		for j, id := range g.OccurrenceIDs {
			o, ok := occurrences[id]
			pointer := fmt.Sprintf("%s/call_sites/%d", g.NativePointer, j)
			if !ok || used[id] || o.GroupID != g.ID || o.NativePointer != pointer || o.BindingPointer != "/graph"+pointer {
				return fail("V2 occurrence mapping mismatch")
			}
			b, ok := bindings[o.BindingPointer]
			if !ok || b.URI != caller.URI || b.AnchorStatus != "VALID_COORDINATES" {
				return fail("V2 occurrence source join mismatch")
			}
			if j > 0 && !lessRange(edge.CallSites[j-1], o.Range) {
				return fail("V2 non-distinct/noncanonical sites")
			}
			used[id] = true
			edge.CallSites = append(edge.CallSites, o.Range)
		}
		if g.Receipt.RelationID != g.NativeRelationID || g.Receipt.CallerNodeID != g.CallerNodeID || g.Receipt.CalleeNodeID != g.CalleeNodeID {
			return fail("V2 group receipt join mismatch")
		}
		support += g.Receipt.SupportContribution
		relations = append(relations, g.Receipt)
		edges = append(edges, edge)
	}
	if len(used) != len(occurrences) || support != t.SupportTotal {
		return fail("V2 orphan occurrence/support mismatch")
	}
	put("edges", edges)
	sort.Slice(relations, func(i, j int) bool { return relations[i].RelationID < relations[j].RelationID })
	if t.ReceiptPresent {
		put("evidence_receipt", graph.EvidenceReceipt{SupportTotal: support, Relations: relations})
	} else if len(relations) > 0 || support != 0 {
		return fail("V2 missing receipt")
	}
	put("seed_memberships", t.Memberships)
	if t.NativeNullArrays == nil {
		return fail("V2 missing native null markers")
	}
	priorNull := ""
	for _, name := range t.NativeNullArrays {
		if name <= priorNull || (name != "nodes" && name != "edges") || (name == "nodes" && len(t.Endpoints) != 0) || (name == "edges" && len(edges) != 0) {
			return fail("V2 invalid native null marker")
		}
		priorNull = name
		fields[name] = json.RawMessage("null")
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return ProjectionV2{}, err
	}
	if len(raw) > MaxNativeBytesV2 {
		return ProjectionV2{}, &LimitErrorV2{"native bytes", MaxNativeBytesV2}
	}
	if _, err = schema.Validate(raw, "v3"); err != nil {
		return ProjectionV2{}, err
	}
	g, err := graph.DecodeNativeV3(raw)
	if err != nil {
		return ProjectionV2{}, err
	}
	nativeBytes, err := json.Marshal(g)
	if err != nil {
		return ProjectionV2{}, err
	}
	if digest(graphprovenance.VersionV2+":graph", nativeBytes) != t.Context.GraphDigest {
		return fail("V2 reconstructed native digest mismatch")
	}
	a, err := restoreArraysV2(t.Acquisition, t.AcquisitionNullArrays)
	if err != nil {
		return ProjectionV2{}, err
	}
	r := a.result(g)
	if err = acquisition.ValidateResult(r); err != nil {
		return ProjectionV2{}, err
	}
	// Verify exported witnesses independently of mapConnectionsV2's producer:
	// traverse the native PathInput edges and prove each original native path.
	_, pathEdges := acquisition.PathInput(g, r.Request.Context.ID)
	if len(t.Connections) != len(r.Targets) {
		return fail("V2 missing connection target")
	}
	for i, target := range r.Targets {
		c := t.Connections[i]
		path := target.Connection.Path
		if c.TargetID != target.Requested.ID || !bytes.Equal(canonical(c.Nodes), canonical(append([]string{}, path.Nodes...))) || len(c.GroupIDs) != len(path.GroupIDs) || len(c.OccurrenceIDs) != len(path.OccurrenceIDs) || c.GroupIDs == nil || c.OccurrenceIDs == nil {
			return fail("V2 connection target/nodes mismatch")
		}
		for j, nativeID := range path.GroupIDs {
			found := false
			for k, e := range pathEdges {
				if e.GroupID == nativeID {
					if c.GroupIDs[j] != t.Groups[k].ID || j >= len(c.OccurrenceIDs) || !bytes.Equal(canonical(c.OccurrenceIDs[j]), canonical(t.Groups[k].OccurrenceIDs)) {
						return fail("V2 connection export witness mismatch")
					}
					found = true
				}
			}
			if !found {
				return fail("V2 connection native witness missing")
			}
		}
		if target.Connection.Status == "FOUND" {
			if err = retainedpath.Prove(pathEdges, target.Connection.From, target.Connection.To, "FOUND", path); err != nil {
				return ProjectionV2{}, err
			}
		}
	}
	e := graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, GraphBytes: nativeBytes, GraphDigest: t.Context.GraphDigest, WorkspaceURI: t.Context.WorkspaceURI, CaptureBudget: t.Context.CaptureBudget, AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Acquisition: r, Bindings: t.Bindings, Supplies: t.Supplies, Captures: t.Captures}
	if err = graphprovenance.ValidateV2(e); err != nil {
		return ProjectionV2{}, err
	}
	return ProjectionV2{r, t.Bindings, t.Supplies, t.Captures, t.Context.WorkspaceURI, t.Context.CaptureBudget}, nil
}
func validateForV2(raw []byte) (string, error) {
	if err := preflightExportV2(raw, MaxBytesV2); err != nil {
		return "", err
	}
	var e EvidenceV2
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return "", err
	}
	if e.SchemaVersion != VersionV2 || e.Policy != PolicyV2 || e.InputDigest != digest(VersionV2+":input", e.InputBytes) || e.Tables.Context.InputDigest != e.InputDigest {
		return "", errors.New("V2 export identity/policy mismatch")
	}
	if _, err := graphprovenance.ValidateFor(e.InputBytes, graphprovenance.Family, "v2"); err != nil {
		return "", err
	}
	if _, err := schema.ValidateStructure(raw, Family, "v2"); err != nil {
		return "", err
	}
	if !bytes.Equal(canonical(json.RawMessage(raw)), canonical(e)) {
		return "", errors.New("V2 export missing/unknown/aliased fields")
	}
	var input graphprovenance.EvidenceV2
	if err := json.Unmarshal(e.InputBytes, &input); err != nil {
		return "", err
	}
	if err := checkLimitsV2(input); err != nil {
		return "", err
	}
	expected, err := extractTablesV2(input, e.InputDigest)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(canonical(expected), canonical(e.Tables)) {
		return "", errors.New("V2 required input-derived tables mismatch")
	}
	reconstructed, err := ReconstructV2(e.Tables)
	if err != nil {
		return "", err
	}
	if err = compareProjectionV2(input, reconstructed); err != nil {
		return "", err
	}
	return VersionV2, nil
}
