package programc_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
	rg "lsp-trace/internal/referencegrouping"
	"sort"
	"strings"
	"testing"
)

// Controlled serialized fixtures, NOT authentic provider execution receipts.
func bundleFixtureInput(t *testing.T, kinds []programcadmission.RelationKind, rich ...bool) (rg.AdmittedInput, []rg.PredecessorRecord) {
	t.Helper()
	node := func(name string, line uint32) graph.Node {
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 1}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///controlled/fixture", Range: r, SelectionRange: r})
	}
	a, b := node("a", 0), node("b", 1)
	inactive := node("inactive-definition-endpoint", 2)
	callsNeeded, methodsNeeded := false, false
	for _, k := range kinds {
		callsNeeded = callsNeeded || k == programcadmission.TypedCalls
		methodsNeeded = methodsNeeded || k != programcadmission.TypedCalls
	}
	var calls programcadmission.CompositeProjectionAdmission
	if callsNeeded {
		capture := func(nodes []graph.Node, edges []graph.Edge, id string) programccompose.Input {
			seed := graph.InvocationSeed{Label: "seed", At: "fixture:1:1", ResolvedURI: "file:///controlled/fixture", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
			r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///controlled", Server: graph.ServerInvocation{Command: "controlled"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "controlled-invocation", SourceRevision: "controlled-commit", ServerVersion: "controlled@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Summary: graph.Summary{Complete: false}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
			native, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := graphprovenance.CaptureV5(native, "controlled-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
			if e != nil {
				t.Fatal(e)
			}
			h := sha256.Sum256(raw)
			return programccompose.Input{Bytes: raw, Identity: id, SHA256: fmt.Sprintf("sha256:%x", h), ByteLength: len(raw), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: "controlled-workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "controlled-private"}}
		}
		edges := []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}}, {CallerNodeID: b.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}}}}
		composite, e := programccompose.Compose([]programccompose.Input{capture([]graph.Node{a, b}, edges, "calls"), capture(nil, nil, "empty")})
		if e != nil {
			t.Fatal(e)
		}
		admitted, e := programcadmission.Admit(composite.Bytes)
		if e != nil {
			t.Fatal(e)
		}
		calls = admitted.Admission
	}
	var methods programcadmission.MethodProjectionAdmission
	if methodsNeeded {
		makeOccurrence := func(id string, k programcadmission.RelationKind, ordinal int, from, to string) programcadmission.MethodOccurrenceEvidence {
			method := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "textDocument/references", programcadmission.TypedResolvesToDefinition: "textDocument/definition"}[k]
			sr := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "REFERENCING_OCCURRENCE", programcadmission.TypedResolvesToDefinition: "QUERY_SYMBOL"}[k]
			tr := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "REFERENCED_SYMBOL", programcadmission.TypedResolvesToDefinition: "DEFINITION_TARGET"}[k]
			q, e := []byte("{\"query\":true}\n"), []byte("{\"evidence\":true}\n")
			qh, eh := sha256.Sum256(q), sha256.Sum256(e)
			return programcadmission.MethodOccurrenceEvidence{Identity: id, Kind: k, Ordinal: ordinal, From: from, To: to, SourceRole: sr, TargetRole: tr, Method: method, QuerySelector: "methods/query.json", QueryDigest: fmt.Sprintf("sha256:%x", qh), QueryLength: len(q), EvidenceSelector: "methods/evidence.json", EvidenceDigest: fmt.Sprintf("sha256:%x", eh), EvidenceLength: len(e), Provider: "controlled-provider", ProviderVersion: "v1", SessionID: "controlled-session", Generation: 1, Revision: "controlled-commit", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16"}
		}
		occurrences := []programcadmission.MethodOccurrenceEvidence{makeOccurrence("ref-0", programcadmission.TypedReferencesSymbol, 0, a.ID, b.ID), makeOccurrence("ref-1", programcadmission.TypedReferencesSymbol, 1, a.ID, a.ID), makeOccurrence("def-0", programcadmission.TypedResolvesToDefinition, 0, a.ID, b.ID)}
		// More than four per kind distinguishes whole-kind CAP4 from a per-pair
		// cap, and repeated/reverse endpoints exercise typed multiplicity.
		if len(rich) > 0 && rich[0] {
			for j := 2; j < 6; j++ {
				from, to := a.ID, b.ID
				if j == 5 {
					from, to = b.ID, a.ID
				}
				occurrences = append(occurrences, makeOccurrence(fmt.Sprintf("ref-%d", j), programcadmission.TypedReferencesSymbol, j, from, to))
			}
			for j := 1; j < 5; j++ {
				occurrences = append(occurrences, makeOccurrence(fmt.Sprintf("def-%d", j), programcadmission.TypedResolvesToDefinition, j, b.ID, inactive.ID))
			}
		}
		var e error
		methods, e = programcadmission.AdmitUntrustedMethodEvidenceEnvelope(programcadmission.UntrustedMethodEvidenceEnvelope{Family: programcadmission.MethodEvidenceFamily, Version: programcadmission.MethodEvidenceFamily, ClaimCeiling: programcadmission.MethodEvidenceClaimCeiling, Authority: 0, Accepted: false, Completeness: "UNKNOWN", Provider: "controlled-provider", ProviderVersion: "v1", SessionID: "controlled-session", Generation: 1, Revision: "controlled-commit", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16", Occurrences: occurrences})
		if e != nil {
			t.Fatal(e)
		}
	}
	typed, e := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls, Methods: methods, EnableMethods: methodsNeeded, Kinds: kinds})
	if e != nil {
		t.Fatal(e)
	}
	projection, e := programcadmission.PreparePrivateLeidenProjection(typed)
	if e != nil {
		t.Fatal(e)
	}
	marshal := func(v any) []byte {
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append(b, '\n')
	}
	type ledgerOccurrence struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	rows := make([]ledgerOccurrence, len(typed.Occurrences))
	outcomes := make([]rg.CandidateOutcome, len(rows))
	for i, o := range typed.Occurrences {
		rows[i] = ledgerOccurrence{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.RelationID}
		outcomes[i] = rg.CandidateOutcome{OccurrenceIdentity: o.Identity, Disposition: "ADMITTED"}
	}
	ledger := marshal(struct {
		Domain      string             `json:"domain"`
		Version     string             `json:"version"`
		Claim       string             `json:"claim"`
		Occurrences []ledgerOccurrence `json:"occurrences"`
	}{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows})
	predecessors := []rg.PredecessorRecord{{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger}}
	custody := []rg.EndpointCustody{}
	if callsNeeded {
		predecessors = append(predecessors, rg.PredecessorRecord{Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: marshal(projection.CompositeSourceBinding())}, rg.PredecessorRecord{Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: projection.RetainedCallsCanonicalBytes()})
		for _, n := range []graph.Node{a, b} {
			custody = append(custody, rg.EndpointCustody{Endpoint: n.ID, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"})
		}
	}
	if methodsNeeded {
		type mapping struct {
			Identity         string `json:"identity"`
			Kind             string `json:"kind"`
			From             string `json:"from"`
			To               string `json:"to"`
			Ordinal          int    `json:"ordinal"`
			QuerySelector    string `json:"query_selector"`
			EvidenceSelector string `json:"evidence_selector"`
		}
		mappings := []mapping{}
		for _, o := range projection.MethodEvidence() {
			mappings = append(mappings, mapping{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.QuerySelector, o.EvidenceSelector})
		}
		sort.Slice(mappings, func(i, j int) bool { return mappings[i].Identity < mappings[j].Identity })
		receipt := marshal(struct {
			Family   string    `json:"family"`
			Version  string    `json:"version"`
			Mappings []mapping `json:"mappings"`
		}{"METHOD_MAPPING_RECEIPT", "v1", mappings})
		predecessors = append(predecessors, rg.PredecessorRecord{Role: "METHOD_QUERY", Selector: "methods/query.json", Schema: "lsp-trace.private.program-c-method-query.v0", Bytes: []byte("{\"query\":true}\n")}, rg.PredecessorRecord{Role: "METHOD_EVIDENCE", Selector: "methods/evidence.json", Schema: programcadmission.MethodEvidenceFamily, Bytes: projection.MethodCanonicalBytes()}, rg.PredecessorRecord{Role: "METHOD_MAPPING_RECEIPT", Selector: "methods/mapping-receipt.json", Schema: "lsp-trace.private.program-c-method-mapping-receipt.v1", Bytes: receipt})
		methodNodes := []graph.Node{a, b}
		if len(rich) > 0 && rich[0] {
			methodNodes = append(methodNodes, inactive)
		}
		for _, n := range methodNodes {
			custody = append(custody, rg.EndpointCustody{Endpoint: n.ID, PredecessorSelector: "methods/evidence.json", Custody: "CALLER_ASSERTED"})
		}
	}
	artifact, e := rg.BuildInput(rg.InputRequest{TypedInput: typed, Predecessors: predecessors, OccurrenceLedgerBytes: ledger, EndpointCustody: custody, OccurrenceAdmissionPolicyID: "controlled-occurrence-v1", CompositionPolicyID: "controlled-identity-v1", PrivacyPolicy: "controlled-private", EligibleKinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Limits: rg.ResourceLimits{MaxArtifacts: 16, MaxNodes: 32, MaxOccurrences: 32, MaxContributions: 32, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}, CandidateOutcomes: outcomes, Accounting: rg.Accounting{Candidates: len(rows), Included: len(rows), Terminal: true}})
	if e != nil {
		t.Fatal(e)
	}
	admitted, e := rg.AdmitInput(artifact, predecessors)
	if e != nil {
		t.Fatal(e)
	}
	return admitted, predecessors
}
