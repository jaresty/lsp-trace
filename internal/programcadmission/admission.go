// Package programcadmission is the validating typed boundary from an exact
// multi-capture composite to the Program C computation core.
package programcadmission

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/programccompose"
)

const Version = "lsp-trace.private.program-c-composite-leiden-admission.v1"
const Authority = 0
const SourceGraphComplete = "UNKNOWN"

const maxNodes = 10_000
const maxOccurrences = 100_000

// ConstituentReference is a complete local mirror of a validated composite
// constituent. Keeping the mirror in the admission package prevents the
// Program C core from depending directly on the composer while retaining every
// canonical source-binding field.
type ConstituentReference struct {
	Identity, SHA256, SchemaVersion, GraphSHA256, GraphSchemaID               string
	ByteLength, GraphByteLength                                               int
	BytesBase64                                                               string
	SessionID, InvocationID                                                   string
	Generation                                                                uint64
	RevisionCustody                                                           string
	SourcePolicy, WorkspaceURI, AnalyzedVersion                               string
	DependencyCompleteness                                                    string
	CaptureBudget                                                             graphprovenance.CaptureBudgetV2
	Supplies                                                                  []graphprovenance.SupplyReceiptV2
	Captures                                                                  []graphprovenance.Receipt
	Bindings                                                                  []graphprovenance.BindingV2
	Invocation, Seeds, SeedMemberships, Frontier, Diagnostics, Summary, Slice json.RawMessage
}

type CompatibilityReference struct {
	WorkspaceURI, SourceRevision, PositionEncoding       string
	RevisionCustody, AcquisitionSemantics, PrivacyPolicy string
	ServerCommand, ServerVersion, LanguageID             string
	EvidenceSemantics, SensitivityPolicy                 json.RawMessage
}

type ProjectedCall struct {
	OccurrenceID, RelationID, CallerNodeID, CalleeNodeID string
	CallSite                                             graph.Range
}

type Artifact struct {
	Version, AdmissionID, CompositeID, CompositeOutputSHA256, ClaimCeiling string
	Authority                                                              int
	SourceGraphComplete                                                    string
	Compatibility                                                          CompatibilityReference
	Constituents                                                           []ConstituentReference
	Completeness                                                           CompositeCompleteness
	NodeIDs                                                                []string
	Calls                                                                  []ProjectedCall
}

// CompositeCompleteness retains the composite's conservative completeness
// ceiling without representing it as one native capture.
type CompositeCompleteness struct {
	AllTraversalComplete bool
	AnyTruncated         bool
	WholeWorkspace       bool
	PerInput             []json.RawMessage
}

// CompositeSourceBinding preserves the complete, ordered source bindings for
// a validated composite projection. It remains authority 0 and never forges a
// native Graph Provenance V5 identity.
type CompositeSourceBinding struct {
	CompositeID, CompositeOutputSHA256, ClaimCeiling string
	Authority                                        int
	SourceGraphComplete                              string
	Compatibility                                    CompatibilityReference
	Constituents                                     []ConstituentReference
	Completeness                                     CompositeCompleteness
}

// CompositeProjectionAdmission is an opaque, validated, non-native Program C input.
// Its zero value is invalid and only Admit can construct a usable value.
type CompositeProjectionAdmission struct {
	nodeIDs      []string
	occurrences  []Occurrence
	claimCeiling string
	source       CompositeSourceBinding
	valid        bool
}

// Occurrence is immutable computation input copied from one validated,
// server-reported constituent CALLS occurrence.
type Occurrence struct {
	Identity, RelationID, Provider, ProviderVersion, Language string
	From, To                                                  int64
	CallSite                                                  graph.Range
	Weight                                                    float64
}

func (a CompositeProjectionAdmission) Valid() bool { return a.valid }
func (a CompositeProjectionAdmission) NodeIdentities() []string {
	return append([]string(nil), a.nodeIDs...)
}
func (a CompositeProjectionAdmission) Occurrences() []Occurrence {
	return append([]Occurrence(nil), a.occurrences...)
}
func (a CompositeProjectionAdmission) ClaimCeiling() string { return a.claimCeiling }
func (a CompositeProjectionAdmission) SourceBinding() CompositeSourceBinding {
	return cloneSourceBinding(a.source)
}

type Result struct {
	Artifact  Artifact
	Bytes     []byte
	Admission CompositeProjectionAdmission
}

// Admit revalidates the exact canonical composite and constructs both a
// separately identified conservative artifact and the opaque computation input.
func Admit(composite []byte) (Result, error) {
	a, err := programccompose.Validate(composite)
	if err != nil {
		return Result{}, fmt.Errorf("composite admission: %w", err)
	}
	// Validate intentionally clears identity fields while recomputing them. Read
	// those fields only from the bytes that have just passed exact replay.
	var verified programccompose.Artifact
	if err := json.Unmarshal(composite, &verified); err != nil {
		return Result{}, fmt.Errorf("composite admission: %w", err)
	}
	a.CompositeID, a.OutputSHA256 = verified.CompositeID, verified.OutputSHA256
	if a.CompositeID == "" || a.OutputSHA256 == "" {
		return Result{}, errors.New("composite admission: missing canonical composite identity")
	}
	if len(a.Nodes) > maxNodes {
		return Result{}, errors.New("composite admission: node cap exceeded")
	}
	nodeIDs := make([]string, len(a.Nodes))
	numeric := make(map[string]int64, len(a.Nodes))
	for i, n := range a.Nodes {
		if n.ID == "" || (i > 0 && a.Nodes[i-1].ID >= n.ID) {
			return Result{}, errors.New("composite admission: noncanonical node identity")
		}
		nodeIDs[i], numeric[n.ID] = n.ID, int64(i)
	}
	calls := make([]ProjectedCall, 0)
	occurrences := make([]Occurrence, 0)
	for i, e := range a.Edges {
		if e.RelationID == "" || (i > 0 && a.Edges[i-1].RelationID >= e.RelationID) {
			return Result{}, errors.New("composite admission: noncanonical call evidence")
		}
		from, fromOK := numeric[e.CallerNodeID]
		to, toOK := numeric[e.CalleeNodeID]
		if !fromOK || !toOK || len(e.CallSites) == 0 {
			return Result{}, errors.New("composite admission: invalid CALLS endpoints or evidence")
		}
		for ordinal, site := range e.CallSites {
			id := occurrenceIdentity(a.CompositeID, e.RelationID, ordinal, site)
			calls = append(calls, ProjectedCall{id, e.RelationID, e.CallerNodeID, e.CalleeNodeID, site})
			occurrences = append(occurrences, Occurrence{Identity: id, RelationID: e.RelationID, Provider: a.Compatibility.ServerCommand, ProviderVersion: a.Compatibility.ServerVersion, Language: a.Compatibility.LanguageID, From: from, To: to, CallSite: site, Weight: 1})
			if len(occurrences) > maxOccurrences {
				return Result{}, errors.New("composite admission: occurrence cap exceeded")
			}
		}
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].OccurrenceID < calls[j].OccurrenceID })
	sort.Slice(occurrences, func(i, j int) bool { return occurrences[i].Identity < occurrences[j].Identity })
	refs := constituentReferences(a.Constituents)
	for i := range refs {
		refs[i].RevisionCustody = a.Compatibility.RevisionCustody
	}
	compat := CompatibilityReference{WorkspaceURI: a.Compatibility.WorkspaceURI, SourceRevision: a.Compatibility.SourceRevision, PositionEncoding: a.Compatibility.PositionEncoding, RevisionCustody: a.Compatibility.RevisionCustody, AcquisitionSemantics: a.Compatibility.AcquisitionSemantics, PrivacyPolicy: a.Compatibility.PrivacyPolicy, ServerCommand: a.Compatibility.ServerCommand, ServerVersion: a.Compatibility.ServerVersion, LanguageID: a.Compatibility.LanguageID, EvidenceSemantics: append(json.RawMessage(nil), a.Compatibility.EvidenceSemantics...), SensitivityPolicy: append(json.RawMessage(nil), a.Compatibility.SensitivityPolicy...)}
	completeness := CompositeCompleteness{AllTraversalComplete: a.Completeness.AllTraversalComplete, AnyTruncated: a.Completeness.AnyTruncated, WholeWorkspace: a.Completeness.WholeWorkspace, PerInput: cloneRawMessages(a.Completeness.PerInput)}
	source := CompositeSourceBinding{CompositeID: a.CompositeID, CompositeOutputSHA256: a.OutputSHA256, ClaimCeiling: a.ClaimCeiling, Authority: Authority, SourceGraphComplete: SourceGraphComplete, Compatibility: compat, Constituents: refs, Completeness: completeness}
	artifact := Artifact{Version: Version, CompositeID: a.CompositeID, CompositeOutputSHA256: a.OutputSHA256, ClaimCeiling: a.ClaimCeiling, Authority: Authority, SourceGraphComplete: SourceGraphComplete, Compatibility: compat, Constituents: refs, Completeness: completeness, NodeIDs: nodeIDs, Calls: calls}
	pre, err := json.Marshal(artifact)
	if err != nil {
		return Result{}, err
	}
	artifact.AdmissionID = digest("lsp-trace:program-c:composite-admission:identity:v1", pre)
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return Result{}, err
	}
	encoded = append(encoded, '\n')
	admission := CompositeProjectionAdmission{nodeIDs: nodeIDs, occurrences: occurrences, claimCeiling: a.ClaimCeiling, source: source, valid: true}
	return Result{Artifact: artifact, Bytes: encoded, Admission: admission}, nil
}

// CloneCompositeSourceBinding returns a defensive copy of validated composite custody.
func CloneCompositeSourceBinding(source CompositeSourceBinding) CompositeSourceBinding {
	return cloneSourceBinding(source)
}

func cloneSourceBinding(source CompositeSourceBinding) CompositeSourceBinding {
	clone := source
	clone.Compatibility.EvidenceSemantics = append(json.RawMessage(nil), source.Compatibility.EvidenceSemantics...)
	clone.Compatibility.SensitivityPolicy = append(json.RawMessage(nil), source.Compatibility.SensitivityPolicy...)
	clone.Constituents = cloneConstituentReferences(source.Constituents)
	clone.Completeness.PerInput = cloneRawMessages(source.Completeness.PerInput)
	return clone
}

func constituentReferences(in []programccompose.Constituent) []ConstituentReference {
	out := make([]ConstituentReference, len(in))
	for i, c := range in {
		out[i] = ConstituentReference{Identity: c.Identity, SHA256: c.SHA256, SchemaVersion: c.SchemaVersion, GraphSHA256: c.GraphSHA256, GraphSchemaID: c.GraphSchemaID, ByteLength: c.ByteLength, GraphByteLength: c.GraphByteLength, BytesBase64: c.BytesBase64, SessionID: c.SessionID, InvocationID: c.InvocationID, Generation: c.Generation, SourcePolicy: c.SourcePolicy, WorkspaceURI: c.WorkspaceURI, AnalyzedVersion: c.AnalyzedVersion, DependencyCompleteness: c.DependencyCompleteness, CaptureBudget: c.CaptureBudget, Supplies: cloneSupplies(c.Supplies), Captures: cloneReceipts(c.Captures), Bindings: cloneBindings(c.Bindings), Invocation: cloneRaw(c.Invocation), Seeds: cloneRaw(c.Seeds), SeedMemberships: cloneRaw(c.SeedMemberships), Frontier: cloneRaw(c.Frontier), Diagnostics: cloneRaw(c.Diagnostics), Summary: cloneRaw(c.Summary), Slice: cloneRaw(c.Slice)}
	}
	return out
}

func cloneConstituentReferences(in []ConstituentReference) []ConstituentReference {
	out := append([]ConstituentReference(nil), in...)
	for i := range out {
		out[i].Supplies = cloneSupplies(in[i].Supplies)
		out[i].Captures = cloneReceipts(in[i].Captures)
		out[i].Bindings = cloneBindings(in[i].Bindings)
		out[i].Invocation = cloneRaw(in[i].Invocation)
		out[i].Seeds = cloneRaw(in[i].Seeds)
		out[i].SeedMemberships = cloneRaw(in[i].SeedMemberships)
		out[i].Frontier = cloneRaw(in[i].Frontier)
		out[i].Diagnostics = cloneRaw(in[i].Diagnostics)
		out[i].Summary = cloneRaw(in[i].Summary)
		out[i].Slice = cloneRaw(in[i].Slice)
	}
	return out
}

func cloneSupplies(in []graphprovenance.SupplyReceiptV2) []graphprovenance.SupplyReceiptV2 {
	out := append([]graphprovenance.SupplyReceiptV2(nil), in...)
	for i := range out {
		out[i].Observation.Observation = cloneRaw(in[i].Observation.Observation)
		if in[i].Receipt != nil {
			r := cloneReceipt(*in[i].Receipt)
			out[i].Receipt = &r
		}
	}
	return out
}

func cloneReceipts(in []graphprovenance.Receipt) []graphprovenance.Receipt {
	if in == nil {
		return nil
	}
	out := make([]graphprovenance.Receipt, len(in))
	for i := range in {
		out[i] = cloneReceipt(in[i])
	}
	return out
}

func cloneReceipt(in graphprovenance.Receipt) graphprovenance.Receipt {
	out := in
	out.Content = append([]byte(nil), in.Content...)
	out.CanonicalReceipt = append([]byte(nil), in.CanonicalReceipt...)
	if in.Supply != nil {
		s := *in.Supply
		s.Params = cloneRaw(in.Supply.Params)
		out.Supply = &s
	}
	return out
}

func cloneBindings(in []graphprovenance.BindingV2) []graphprovenance.BindingV2 {
	out := append([]graphprovenance.BindingV2(nil), in...)
	for i := range out {
		out[i].ReceiptIDs = append([]string(nil), in[i].ReceiptIDs...)
	}
	return out
}

func cloneRaw(in json.RawMessage) json.RawMessage {
	if in == nil {
		return nil
	}
	out := make(json.RawMessage, len(in))
	copy(out, in)
	return out
}

func cloneRawMessages(in []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, len(in))
	for i := range in {
		out[i] = append(json.RawMessage(nil), in[i]...)
	}
	return out
}

func occurrenceIdentity(compositeID, relationID string, ordinal int, site graph.Range) string {
	b, _ := json.Marshal(struct {
		CompositeID, RelationID string
		Ordinal                 int
		Site                    graph.Range
	}{compositeID, relationID, ordinal, site})
	return digest("lsp-trace:program-c:composite-call-occurrence:v1", b)
}

func digest(domain string, b []byte) string {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte{0})
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(b)))
	h.Write(size[:])
	h.Write(b)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
