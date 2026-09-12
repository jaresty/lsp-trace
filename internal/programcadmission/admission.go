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

type ConstituentReference struct {
	Identity, SHA256, SchemaVersion, GraphSHA256, GraphSchemaID string
	ByteLength, GraphByteLength                                 int
	SessionID, InvocationID                                     string
	Generation                                                  uint64
	RevisionCustody                                             string
	Supplies                                                    []graphprovenance.SupplyReceiptV2
	Captures                                                    []graphprovenance.Receipt
	Bindings                                                    []graphprovenance.BindingV2
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
	NodeIDs                                                                []string
	Calls                                                                  []ProjectedCall
}

// CompositeProjectionAdmission is an opaque, validated, non-native Program C input.
// Its zero value is invalid and only Admit can construct a usable value.
type CompositeProjectionAdmission struct {
	nodeIDs      []string
	occurrences  []Occurrence
	claimCeiling string
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
	refs := make([]ConstituentReference, len(a.Constituents))
	for i, c := range a.Constituents {
		refs[i] = ConstituentReference{Identity: c.Identity, SHA256: c.SHA256, SchemaVersion: c.SchemaVersion, GraphSHA256: c.GraphSHA256, GraphSchemaID: c.GraphSchemaID, ByteLength: c.ByteLength, GraphByteLength: c.GraphByteLength, SessionID: c.SessionID, InvocationID: c.InvocationID, Generation: c.Generation, RevisionCustody: a.Compatibility.RevisionCustody, Supplies: append([]graphprovenance.SupplyReceiptV2(nil), c.Supplies...), Captures: append([]graphprovenance.Receipt(nil), c.Captures...), Bindings: append([]graphprovenance.BindingV2(nil), c.Bindings...)}
	}
	compat := CompatibilityReference{WorkspaceURI: a.Compatibility.WorkspaceURI, SourceRevision: a.Compatibility.SourceRevision, PositionEncoding: a.Compatibility.PositionEncoding, RevisionCustody: a.Compatibility.RevisionCustody, AcquisitionSemantics: a.Compatibility.AcquisitionSemantics, PrivacyPolicy: a.Compatibility.PrivacyPolicy, ServerCommand: a.Compatibility.ServerCommand, ServerVersion: a.Compatibility.ServerVersion, LanguageID: a.Compatibility.LanguageID, EvidenceSemantics: append(json.RawMessage(nil), a.Compatibility.EvidenceSemantics...), SensitivityPolicy: append(json.RawMessage(nil), a.Compatibility.SensitivityPolicy...)}
	artifact := Artifact{Version: Version, CompositeID: a.CompositeID, CompositeOutputSHA256: a.OutputSHA256, ClaimCeiling: a.ClaimCeiling, Authority: Authority, SourceGraphComplete: SourceGraphComplete, Compatibility: compat, Constituents: refs, NodeIDs: nodeIDs, Calls: calls}
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
	admission := CompositeProjectionAdmission{nodeIDs: nodeIDs, occurrences: occurrences, claimCeiling: a.ClaimCeiling, valid: true}
	return Result{Artifact: artifact, Bytes: encoded, Admission: admission}, nil
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
