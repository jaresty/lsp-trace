package hydratedevidence

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/v5sourcesnapshot"
)

// FocusRequest selects native graph identities, not names or catalog prefixes.
// CorePolicy limits remain mandatory; IncludeBodies here controls body release.
type FocusRequest struct {
	NodeIDs            []string `json:"node_ids"`
	RelationIDs        []string `json:"relation_ids"`
	SiblingRelationIDs []string `json:"sibling_relation_ids"`
	SidecarRecordIDs   []string `json:"sidecar_record_ids"`
	IncludeBodies      bool     `json:"include_bodies"`
	WholeFile          bool     `json:"whole_file"`
	EndpointContext    bool     `json:"endpoint_context"`
	PositionEncoding   string   `json:"position_encoding"`
	CorePolicy         Policy   `json:"core_policy"`
}

func DefaultFocusRequest() FocusRequest { return FocusRequest{CorePolicy: DefaultPolicy()} }

type FocusSite struct {
	Role       string   `json:"role"`
	Pointer    string   `json:"pointer"`
	RecordID   string   `json:"record_id"`
	NodeID     string   `json:"node_id"`
	RelationID string   `json:"relation_id"`
	URI        string   `json:"uri"`
	Status     string   `json:"status"`
	SourceIDs  []string `json:"source_ids"`
	OriginIDs  []string `json:"origin_ids"`
}
type FocusOrigin struct {
	Ordinal       int         `json:"ordinal"`
	Kind          string      `json:"kind"`
	RequestedID   string      `json:"requested_id"`
	Status        string      `json:"status"`
	CallerNodeID  string      `json:"caller_node_id"`
	CalleeNodeID  string      `json:"callee_node_id"`
	CallSiteCount int         `json:"call_site_count"`
	Sites         []FocusSite `json:"sites"`
}
type FocusManifest struct {
	InputDigests             []string      `json:"input_digests"`
	SelectionDigest          string        `json:"selection_digest"`
	DuplicatePolicy          string        `json:"duplicate_policy"`
	ReceiptPolicy            string        `json:"receipt_policy"`
	NodePolicy               string        `json:"node_policy"`
	ExcludedNonSourceRecords int           `json:"excluded_non_source_records"`
	Origins                  []FocusOrigin `json:"origins"`
	Digest                   string        `json:"digest"`
}
type FocusResult struct {
	Manifest FocusManifest `json:"manifest"`
	Request  Request       `json:"request"`
	Bundle   Bundle        `json:"bundle"`
}

// focusGraph is a typed projection of already validated exact GraphBytes. Raw
// ranges preserve unusable coordinates without coercing them into valid zeros.
// Neither names, opaque data nor acquisition request IDs participate in this join.
type focusGraph struct {
	Nodes []struct {
		ID    string          `json:"id"`
		URI   string          `json:"uri"`
		Range json.RawMessage `json:"range"`
	} `json:"nodes"`
	Edges []struct {
		ID        string            `json:"relation_id"`
		Caller    string            `json:"caller_node_id"`
		Callee    string            `json:"callee_node_id"`
		CallSites []json.RawMessage `json:"call_sites"`
	} `json:"edges"`
	SiblingCandidates []struct {
		ID     string `json:"relation_id"`
		Origin struct {
			ID             string          `json:"id"`
			URI            string          `json:"uri"`
			Range          json.RawMessage `json:"range"`
			SelectionRange json.RawMessage `json:"selection_range"`
		} `json:"origin"`
		Declaration struct {
			ID             string          `json:"id"`
			URI            string          `json:"uri"`
			Range          json.RawMessage `json:"range"`
			SelectionRange json.RawMessage `json:"selection_range"`
		} `json:"document_symbol"`
		Candidate struct {
			ID             string          `json:"id"`
			URI            string          `json:"uri"`
			Range          json.RawMessage `json:"range"`
			SelectionRange json.RawMessage `json:"selection_range"`
		} `json:"candidate"`
	} `json:"sibling_candidates"`
}

// focusPlan re-admits original input and derives every expected carrier from
// native array indices. It never accepts a presented manifest as a mapping oracle.
func focusPlan(input Input, f FocusRequest) (FocusManifest, Request, error) {
	m := FocusManifest{}
	p := f.CorePolicy
	p.IncludeBodies = f.IncludeBodies // A core policy cannot bypass the focus opt-in.
	r := Request{Policy: p, Selections: []Selection{}}
	fail := func(e error) (FocusManifest, Request, error) { return FocusManifest{}, Request{}, e }
	if err := checkPolicy(p); err != nil {
		return fail(err)
	}
	if len(f.NodeIDs)+len(f.RelationIDs)+len(f.SiblingRelationIDs)+len(f.SidecarRecordIDs) > p.MaxOrigins {
		return fail(errors.New("focused origin input budget"))
	}
	raw, err := json.Marshal(f)
	if err != nil || len(raw) > 4<<20 {
		return fail(errors.New("focused selection byte budget"))
	}
	for _, ids := range [][]string{f.NodeIDs, f.RelationIDs, f.SiblingRelationIDs, f.SidecarRecordIDs} {
		for _, id := range ids {
			if len(id) > 1024 {
				return fail(errors.New("focused ID byte budget"))
			}
		}
	}
	a, err := admit(input, p)
	if err != nil {
		return fail(err)
	}
	var env struct {
		SchemaVersion string `json:"schema_version"`
		GraphBytes    []byte `json:"graph_bytes"`
	}
	if err = json.Unmarshal(input.Artifact, &env); err != nil {
		return fail(err)
	}
	graphBytes := env.GraphBytes
	prefix := ""
	if env.SchemaVersion == graphprovenance.VersionV2 {
		prefix = "/graph"
	}
	if env.SchemaVersion == v5sourcesnapshot.Version {
		var snapshot v5sourcesnapshot.Artifact
		var v5 graphprovenance.EvidenceV5
		if err = json.Unmarshal(input.Artifact, &snapshot); err != nil {
			return fail(err)
		}
		if err = json.Unmarshal(snapshot.GraphV5Bytes, &v5); err != nil {
			return fail(err)
		}
		graphBytes, err = base64.StdEncoding.DecodeString(v5.GraphV5)
		if err != nil {
			return fail(err)
		}
		prefix = "/graph"
	}
	var g focusGraph
	if err = json.Unmarshal(graphBytes, &g); err != nil {
		return fail(err)
	}
	assertedRecords := map[string]bool{}
	for _, raw := range input.Sidecars {
		var side Sidecar
		if err := json.Unmarshal(raw, &side); err != nil {
			return fail(err)
		}
		for _, rec := range side.Records {
			assertedRecords["sidecar:"+Digest(raw)+":"+rec.ID] = true
		}
	}
	nodes := map[string][]int{}
	edges := map[string][]int{}
	siblings := map[string][]int{}
	for i, n := range g.Nodes {
		nodes[n.ID] = append(nodes[n.ID], i)
	}
	for i, e := range g.Edges {
		edges[e.ID] = append(edges[e.ID], i)
	}
	for i, sibling := range g.SiblingCandidates {
		siblings[sibling.ID] = append(siblings[sibling.ID], i)
	}
	m = FocusManifest{InputDigests: a.InputDigests, SelectionDigest: valueDigest(f), DuplicatePolicy: "PRESERVE_OCCURRENCES", ReceiptPolicy: "ALL_EXACT_BOUND_RECEIPTS", NodePolicy: "RETAINED_NODE_RANGE", Origins: []FocusOrigin{}}
	for _, rec := range a.Records {
		if rec.SourceAttribution == "NON_SOURCE" {
			m.ExcludedNonSourceRecords++
		}
	}
	var planErr error
	siteCount := 0
	add := func(o *FocusOrigin, site FocusSite, nativeRange json.RawMessage, asserted bool) {
		if planErr != nil {
			return
		}
		if siteCount >= p.MaxOrigins {
			planErr = errors.New("focused expanded site budget")
			return
		}
		siteCount++
		site.SourceIDs = []string{}
		site.OriginIDs = []string{}
		rec, ok := a.recordMap[site.RecordID]
		site.Status = "NO_RETAINED_BINDING"
		if !ok {
			o.Sites = append(o.Sites, site)
			return
		}
		if rec.SourceAttribution == "NON_SOURCE" {
			site.Status = "NON_SOURCE_EXCLUDED"
			o.Sites = append(o.Sites, site)
			return
		}
		if !asserted {
			// Pointer equality alone is insufficient: require native identity and range
			// agreement with the typed node/edge carrier, and exact URI per receipt.
			identityOK := rec.NativeID == site.NodeID
			if site.Role == "CALL_SITE" {
				identityOK = includes(rec.RelationshipReferences, site.RelationID)
			} else if site.RelationID != "" {
				identityOK = identityOK && includes(rec.RelationshipReferences, site.RelationID)
			}
			if rec.Authority != Native || rec.Pointer != site.Pointer || !identityOK {
				planErr = errors.New("focused native binding identity mismatch")
				return
			}
			var rg Range
			if json.Unmarshal(nativeRange, &rg) == nil && string(nativeRange) != "null" {
				if rec.Range == nil || !same(*rec.Range, rg) {
					planErr = errors.New("focused retained range mismatch")
					return
				}
			} else if rec.Range != nil {
				planErr = errors.New("focused unusable range mismatch")
				return
			}
		}
		site.Status = "MAPPED"
		if len(rec.SourceIDs) == 0 {
			site.Status = "NO_BOUND_SOURCE"
		}
		if rec.Range == nil && !f.WholeFile {
			site.Status = "NO_RETAINED_RANGE"
		}
		if rec.AnchorStatus == "INVALID_COORDINATES" {
			site.Status = "INVALID_COORDINATES"
		}
		ids := append([]string{}, rec.SourceIDs...)
		sort.Strings(ids)
		for _, sid := range ids {
			src := a.sourceMap[sid]
			if !asserted && src.URI != site.URI {
				planErr = errors.New("focused source URI mismatch")
				return
			}
			if len(r.Selections) >= p.MaxOrigins {
				planErr = errors.New("focused expanded origin budget")
				return
			}
			id := fmt.Sprintf("focus:%d:%d:%d", o.Ordinal, len(o.Sites), len(site.OriginIDs))
			mode := "RETAINED_RANGE"
			if f.WholeFile {
				mode = "WHOLE_FILE"
			}
			r.Selections = append(r.Selections, Selection{ID: id, RecordID: rec.ID, SourceID: sid, Mode: mode, Encoding: f.PositionEncoding})
			site.SourceIDs = append(site.SourceIDs, sid)
			site.OriginIDs = append(site.OriginIDs, id)
		}
		o.Sites = append(o.Sites, site)
	}
	addNode := func(o *FocusOrigin, id, role string) {
		indices := nodes[id]
		if len(indices) != 1 {
			o.Sites = append(o.Sites, FocusSite{Role: role, NodeID: id, Status: "UNKNOWN_OR_AMBIGUOUS_NODE", SourceIDs: []string{}, OriginIDs: []string{}})
			return
		}
		i := indices[0]
		n := g.Nodes[i]
		ptr := fmt.Sprintf("%s/nodes/%d/range", prefix, i)
		add(o, FocusSite{Role: role, Pointer: ptr, RecordID: "native:" + ptr, NodeID: n.ID, URI: n.URI}, n.Range, false)
	}
	for kind, ids := range [][]string{f.NodeIDs, f.RelationIDs, f.SiblingRelationIDs, f.SidecarRecordIDs} {
		for _, id := range ids {
			o := FocusOrigin{Ordinal: len(m.Origins), Kind: []string{"NODE", "RELATION", "SIBLING_RELATION", "SIDECAR_RECORD"}[kind], RequestedID: id, Status: "UNKNOWN_ID", Sites: []FocusSite{}}
			switch kind {
			case 0:
				if len(nodes[id]) > 1 {
					o.Status = "AMBIGUOUS_ID"
				} else if len(nodes[id]) == 1 {
					o.Status = "MAPPED"
					addNode(&o, id, "NODE_RANGE")
				}
			case 1:
				if len(edges[id]) > 1 {
					o.Status = "AMBIGUOUS_ID"
				} else if len(edges[id]) == 1 {
					i := edges[id][0]
					e := g.Edges[i]
					o.Status = "MAPPED"
					o.CallerNodeID = e.Caller
					o.CalleeNodeID = e.Callee
					o.CallSiteCount = len(e.CallSites)
					if len(e.CallSites) == 0 {
						o.Status = "NO_CALL_SITES"
					}
					callerURI := ""
					if ns := nodes[e.Caller]; len(ns) == 1 {
						callerURI = g.Nodes[ns[0]].URI
					}
					for j, rg := range e.CallSites {
						ptr := fmt.Sprintf("%s/edges/%d/call_sites/%d", prefix, i, j)
						add(&o, FocusSite{Role: "CALL_SITE", Pointer: ptr, RecordID: "native:" + ptr, NodeID: e.Caller, RelationID: e.ID, URI: callerURI}, rg, false)
					}
					if f.EndpointContext {
						addNode(&o, e.Caller, "CALLER_RANGE")
						addNode(&o, e.Callee, "CALLEE_RANGE")
					}
				}
			case 2:
				if len(siblings[id]) > 1 {
					o.Status = "AMBIGUOUS_ID"
				} else if len(siblings[id]) == 1 {
					i := siblings[id]
					sibling := g.SiblingCandidates[i[0]]
					o.Status = "MAPPED"
					for _, endpoint := range []struct {
						name, role, nodeID, uri          string
						declarationRange, selectionRange json.RawMessage
					}{{"origin", "ORIGIN", sibling.Origin.ID, sibling.Origin.URI, sibling.Origin.Range, sibling.Origin.SelectionRange}, {"document_symbol", "DECLARATION", sibling.Declaration.ID, sibling.Declaration.URI, sibling.Declaration.Range, sibling.Declaration.SelectionRange}, {"candidate", "PREPARED", sibling.Candidate.ID, sibling.Candidate.URI, sibling.Candidate.Range, sibling.Candidate.SelectionRange}} {
						for _, site := range []struct {
							field, role string
							rg          json.RawMessage
						}{{"range", endpoint.role + "_DECLARATION_RANGE", endpoint.declarationRange}, {"selection_range", endpoint.role + "_SELECTION_RANGE", endpoint.selectionRange}} {
							ptr := fmt.Sprintf("%s/sibling_candidates/%d/%s/%s", prefix, i[0], endpoint.name, site.field)
							add(&o, FocusSite{Role: site.role, Pointer: ptr, RecordID: "native:" + ptr, NodeID: endpoint.nodeID, RelationID: sibling.ID, URI: endpoint.uri}, site.rg, false)
						}
					}
				}
			case 3:
				if rec, ok := a.recordMap[id]; ok {
					// Only the existing sidecar record contract is admitted here. Receipt
					// handles and native catalog pointers are not sidecar record selectors.
					if rec.Authority != Caller || !assertedRecords[id] {
						o.Status = "UNSUPPORTED_RECORD_TYPE"
					} else {
						o.Status = "MAPPED"
						add(&o, FocusSite{Role: "ASSERTED_RECORD", Pointer: rec.Pointer, RecordID: rec.ID}, nil, true)
					}
				}
			}
			if planErr != nil {
				return fail(planErr)
			}
			m.Origins = append(m.Origins, o)
		}
	}
	if err := checkRequest(r); err != nil {
		return fail(err)
	}
	m.Digest = valueDigest(m)
	return m, r, nil
}

// HydrateFocused performs no acquisition and preserves the entire admitted core
// source catalog. MAPPED means an identity join, not delivered/complete context;
// Sites' OriginIDs lead to the core's exact privacy/availability/budget outcomes.
func HydrateFocused(input Input, f FocusRequest) (FocusResult, error) {
	m, r, err := focusPlan(input, f)
	if err != nil {
		return FocusResult{}, err
	}
	b, err := Hydrate(input, r)
	if err != nil {
		return FocusResult{}, err
	}
	result := FocusResult{Manifest: m, Request: r, Bundle: b}
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > r.Policy.MaxOutputBytes {
		return FocusResult{}, errors.New("focused output byte budget")
	}
	return result, nil
}

// ValidateFocused independently re-admits the externally supplied input and
// selection, reconstructing all expected native carriers (including zero-site
// groups) before comparing mappings. It calls core Validate, never Hydrate or
// HydrateFocused. The typed projection is shared, not a second native parser.
func ValidateFocused(input Input, f FocusRequest, result FocusResult) error {
	m, r, err := focusPlan(input, f)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > r.Policy.MaxOutputBytes {
		return errors.New("focused output byte budget")
	}
	if !same(m, result.Manifest) || !same(r, result.Request) {
		return errors.New("focused manifest/selection semantic mismatch")
	}
	if err := auditNativeFocus(input, f, result); err != nil {
		return err
	}
	return Validate(input, r, result.Bundle)
}

// auditNativeFocus is deliberately separate from focusPlan's carrier emission.
// It checks exhaustive native coverage against original graph arrays, including
// every site and every receipt/selection foreign key. Thus a shared-plan omission
// cannot become valid merely by rehashing both the manifest and core request.
// Admission has already succeeded; this is not an alternate native validator.
func auditNativeFocus(input Input, f FocusRequest, result FocusResult) error {
	bad := func() error { return errors.New("focused independent native coverage mismatch") }
	var env struct {
		SchemaVersion string `json:"schema_version"`
		GraphBytes    []byte `json:"graph_bytes"`
	}
	if e := json.Unmarshal(input.Artifact, &env); e != nil {
		return e
	}
	graphBytes := env.GraphBytes
	prefix := ""
	if env.SchemaVersion == graphprovenance.VersionV2 {
		prefix = "/graph"
	}
	if env.SchemaVersion == v5sourcesnapshot.Version {
		var snapshot v5sourcesnapshot.Artifact
		var v5 graphprovenance.EvidenceV5
		if e := json.Unmarshal(input.Artifact, &snapshot); e != nil {
			return e
		}
		if e := json.Unmarshal(snapshot.GraphV5Bytes, &v5); e != nil {
			return e
		}
		var e error
		graphBytes, e = base64.StdEncoding.DecodeString(v5.GraphV5)
		if e != nil {
			return e
		}
		prefix = "/graph"
	}
	var g focusGraph
	if e := json.Unmarshal(graphBytes, &g); e != nil {
		return e
	}
	a, e := admit(input, result.Request.Policy)
	if e != nil {
		return e
	}
	selections := map[string]Selection{}
	for _, s := range result.Request.Selections {
		selections[s.ID] = s
	}
	used := map[string]bool{}
	check := func(site FocusSite, pointer, role, node, relation, uri string) bool {
		if site.Pointer != pointer || site.Role != role || site.NodeID != node || site.RelationID != relation || site.URI != uri || site.RecordID != "native:"+pointer {
			return false
		}
		rec, exists := a.recordMap[site.RecordID]
		if !exists {
			return site.Status == "NO_RETAINED_BINDING" && len(site.OriginIDs) == 0 && len(site.SourceIDs) == 0
		}
		if rec.SourceAttribution == "NON_SOURCE" {
			return site.Status == "NON_SOURCE_EXCLUDED" && len(site.OriginIDs) == 0 && len(site.SourceIDs) == 0
		}
		ids := append([]string{}, rec.SourceIDs...)
		sort.Strings(ids)
		if !same(ids, site.SourceIDs) || len(ids) != len(site.OriginIDs) {
			return false
		}
		for i, id := range site.OriginIDs {
			s, ok := selections[id]
			if !ok || used[id] || s.RecordID != rec.ID || s.SourceID != ids[i] || a.sourceMap[s.SourceID].URI != uri {
				return false
			}
			used[id] = true
		}
		return true
	}
	ordinal := 0
	for _, id := range f.NodeIDs {
		if ordinal >= len(result.Manifest.Origins) {
			return bad()
		}
		o := result.Manifest.Origins[ordinal]
		ordinal++
		if o.RequestedID != id || o.Kind != "NODE" {
			return bad()
		}
		found := 0
		for i, n := range g.Nodes {
			if n.ID == id {
				found++
				if len(o.Sites) != 1 || !check(o.Sites[0], fmt.Sprintf("%s/nodes/%d/range", prefix, i), "NODE_RANGE", id, "", n.URI) {
					return bad()
				}
			}
		}
		if found == 0 && (o.Status != "UNKNOWN_ID" || len(o.Sites) != 0) {
			return bad()
		}
	}
	for _, id := range f.RelationIDs {
		if ordinal >= len(result.Manifest.Origins) {
			return bad()
		}
		o := result.Manifest.Origins[ordinal]
		ordinal++
		if o.RequestedID != id || o.Kind != "RELATION" {
			return bad()
		}
		found := 0
		for i, edge := range g.Edges {
			if edge.ID != id {
				continue
			}
			found++
			want := len(edge.CallSites)
			if f.EndpointContext {
				want += 2
			}
			if o.CallSiteCount != len(edge.CallSites) || len(o.Sites) != want || o.CallerNodeID != edge.Caller || o.CalleeNodeID != edge.Callee {
				return bad()
			}
			if len(edge.CallSites) == 0 && o.Status != "NO_CALL_SITES" {
				return bad()
			}
			uri := ""
			for _, n := range g.Nodes {
				if n.ID == edge.Caller {
					uri = n.URI
				}
			}
			for j := range edge.CallSites {
				if !check(o.Sites[j], fmt.Sprintf("%s/edges/%d/call_sites/%d", prefix, i, j), "CALL_SITE", edge.Caller, id, uri) {
					return bad()
				}
			}
			if f.EndpointContext {
				for k, nid := range []string{edge.Caller, edge.Callee} {
					for j, n := range g.Nodes {
						if n.ID == nid {
							if !check(o.Sites[len(edge.CallSites)+k], fmt.Sprintf("%s/nodes/%d/range", prefix, j), []string{"CALLER_RANGE", "CALLEE_RANGE"}[k], nid, "", n.URI) {
								return bad()
							}
						}
					}
				}
			}
		}
		if found == 0 && (o.Status != "UNKNOWN_ID" || len(o.Sites) != 0) {
			return bad()
		}
	}
	for _, id := range f.SiblingRelationIDs {
		if ordinal >= len(result.Manifest.Origins) {
			return bad()
		}
		o := result.Manifest.Origins[ordinal]
		ordinal++
		if o.RequestedID != id || o.Kind != "SIBLING_RELATION" {
			return bad()
		}
		found := 0
		for i, sibling := range g.SiblingCandidates {
			if sibling.ID != id {
				continue
			}
			found++
			if len(o.Sites) != 6 {
				return bad()
			}
			siteIndex := 0
			for _, endpoint := range []struct {
				name, role, nodeID, uri string
			}{{"origin", "ORIGIN", sibling.Origin.ID, sibling.Origin.URI}, {"document_symbol", "DECLARATION", sibling.Declaration.ID, sibling.Declaration.URI}, {"candidate", "PREPARED", sibling.Candidate.ID, sibling.Candidate.URI}} {
				for _, rangeRow := range []struct{ field, role string }{{"range", endpoint.role + "_DECLARATION_RANGE"}, {"selection_range", endpoint.role + "_SELECTION_RANGE"}} {
					pointer := fmt.Sprintf("%s/sibling_candidates/%d/%s/%s", prefix, i, endpoint.name, rangeRow.field)
					if !check(o.Sites[siteIndex], pointer, rangeRow.role, endpoint.nodeID, id, endpoint.uri) {
						return bad()
					}
					siteIndex++
				}
			}
		}
		if found == 0 && (o.Status != "UNKNOWN_ID" || len(o.Sites) != 0) {
			return bad()
		}
	}
	// Sidecar mappings are re-derived by focusPlan under the existing asserted
	// contract; native selection IDs cannot be reused by an asserted mapping.
	for _, o := range result.Manifest.Origins[ordinal:] {
		for _, site := range o.Sites {
			for _, id := range site.OriginIDs {
				if used[id] {
					return bad()
				}
				used[id] = true
			}
		}
	}
	if len(used) != len(selections) {
		return bad()
	}
	return nil
}
