// Package programccompose deterministically composes compatible immutable
// Graph Provenance V5 captures without manufacturing producer provenance.
package programccompose

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/programc"
)

const (
	Version            = "lsp-trace.private.program-c-multi-capture-composite.v1"
	PolicyVersion      = "program-c-multi-capture-policy.v1"
	MaxInputs          = 16
	MaxTotalInputBytes = 256 << 20
	MaxWorkUnits       = 400_000
	ClaimCeiling       = "STRUCTURAL_SERVER_REPORTED_CALLS_UNION_ONLY;NO_WHOLE_WORKSPACE_COMPLETENESS;NO_FEATURE_IDENTITY;NO_OWNERSHIP;NO_ARCHITECTURE;NO_RUNTIME_EXECUTION;NO_PRODUCER_AUTHENTICATION;NO_PERMISSION;NO_PRODUCTION_AUTHORITY"
)

var PolicyBytes = []byte("program-c-multi-capture-policy.v1\ninputs=2..16\ntotal_input_bytes<=268435456\nwork_units=input_bytes+nodes+occurrences<=400000\nnodes<=10000\noccurrences<=100000\nsame_session_generation_revision_workspace_encoding_provider_language_semantics_sensitivity\nexact-id-equivalent-content-dedupe;conflict-fails\nno-inference;no-truncation;constituents-preserved\n")

func PolicyDigest() string { return digest("lsp-trace:program-c-compose:policy:v1", PolicyBytes) }

type Input struct {
	Bytes      []byte
	Identity   string
	SHA256     string
	ByteLength int
	// ExactMetadata supplies compatibility coordinates absent from the V5
	// envelope. They are caller assertions bound into the composite, not producer
	// authentication, and every field is mandatory and compared exactly.
	ExactMetadata ExactMetadata
}

type ExactMetadata struct {
	WorkspaceIdentity, RevisionCustody, PositionEncoding string
	AcquisitionSemantics, PrivacyPolicy                  string
}

type Constituent struct {
	Identity, SHA256, SchemaVersion, GraphSHA256, GraphSchemaID string
	ByteLength, GraphByteLength                                 int
	BytesBase64                                                 string
	SessionID                                                   string
	Generation                                                  uint64
}

type Compatibility struct {
	WorkspaceURI, SourceRevision, InvocationID, PositionEncoding string
	RevisionCustody, AcquisitionSemantics, PrivacyPolicy         string
	ServerCommand, ServerVersion, LanguageID                     string
	EvidenceSemantics, SensitivityPolicy                         json.RawMessage
}

type Completeness struct {
	AllTraversalComplete bool
	AnyTruncated         bool
	WholeWorkspace       bool
	PerInput             []json.RawMessage
}

type Artifact struct {
	Version, PolicyVersion, PolicySHA256, CompositeID, OutputSHA256, ClaimCeiling string
	Constituents                                                                  []Constituent
	Compatibility                                                                 Compatibility
	Nodes                                                                         []graph.Node
	Edges                                                                         []graph.Edge
	Completeness                                                                  Completeness
}

type Result struct {
	Artifact Artifact
	Bytes    []byte
}

type envelope struct {
	SchemaVersion   string          `json:"schema_version"`
	SessionID       string          `json:"session_id"`
	Generation      uint64          `json:"generation"`
	GraphV5         string          `json:"graph_v5"`
	GraphV5SHA256   string          `json:"graph_v5_sha256"`
	GraphV5SchemaID string          `json:"graph_v5_schema_id"`
	Diagnostics     json.RawMessage `json:"diagnostics"`
}

type native struct {
	SchemaVersion string `json:"schema_version"`
	Invocation    struct {
		WorkspaceURI, LanguageID string
		Server                   graph.ServerInvocation
		Provenance               graph.InvocationProvenance
		Seeds                    []graph.InvocationSeed
	} `json:"invocation"`
	Nodes             []graph.Node           `json:"nodes"`
	Edges             []graph.Edge           `json:"edges"`
	EvidenceSemantics json.RawMessage        `json:"evidence_semantics"`
	SensitivityPolicy json.RawMessage        `json:"sensitivity_policy"`
	EvidenceReceipt   *graph.EvidenceReceipt `json:"evidence_receipt"`
	Summary           json.RawMessage        `json:"summary"`
	PositionEncoding  string                 `json:"position_encoding"`
}

type summary struct {
	TraversalComplete bool `json:"traversal_complete"`
	Truncated         bool `json:"truncated"`
}

type admitted struct {
	in  Input
	c   Constituent
	n   native
	env envelope
}

func Compose(inputs []Input) (Result, error) {
	if len(inputs) < 2 || len(inputs) > MaxInputs {
		return Result{}, fmt.Errorf("input count %d outside [2,%d]", len(inputs), MaxInputs)
	}
	total := 0
	items := make([]admitted, len(inputs))
	seenInput := map[string]string{}
	for i, in := range inputs {
		total += len(in.Bytes)
		if total > MaxTotalInputBytes {
			return Result{}, errors.New("total input byte cap exceeded")
		}
		if in.Identity == "" || in.ByteLength != len(in.Bytes) || in.SHA256 != rawDigest(in.Bytes) {
			return Result{}, fmt.Errorf("input %d immutable identity/digest/length mismatch", i)
		}
		m := in.ExactMetadata
		if m.WorkspaceIdentity == "" || m.RevisionCustody == "" || m.PositionEncoding == "" || m.AcquisitionSemantics == "" || m.PrivacyPolicy == "" {
			return Result{}, fmt.Errorf("input %d ambiguous exact metadata", i)
		}
		if previous, ok := seenInput[in.Identity]; ok && previous != in.SHA256 {
			return Result{}, fmt.Errorf("input identity conflict %q", in.Identity)
		}
		seenInput[in.Identity] = in.SHA256
		if v, err := graphprovenance.ValidateFor(in.Bytes, graphprovenance.Family, "v5"); err != nil || v != graphprovenance.VersionV5 {
			return Result{}, fmt.Errorf("input %d invalid graph provenance V5: %w", i, err)
		}
		var e envelope
		if err := strictDecode(in.Bytes, &e); err != nil {
			return Result{}, fmt.Errorf("input %d: %w", i, err)
		}
		gb, err := base64.StdEncoding.DecodeString(e.GraphV5)
		if err != nil {
			return Result{}, err
		}
		var n native
		if err := json.Unmarshal(gb, &n); err != nil {
			return Result{}, fmt.Errorf("input %d graph: %w", i, err)
		}
		items[i] = admitted{in: in, env: e, n: n, c: Constituent{Identity: in.Identity, SHA256: in.SHA256, ByteLength: len(in.Bytes), SchemaVersion: e.SchemaVersion, GraphSHA256: e.GraphV5SHA256, GraphByteLength: len(gb), GraphSchemaID: e.GraphV5SchemaID, BytesBase64: base64.StdEncoding.EncodeToString(in.Bytes), SessionID: e.SessionID, Generation: e.Generation}}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].in.SHA256 != items[j].in.SHA256 {
			return items[i].in.SHA256 < items[j].in.SHA256
		}
		return items[i].in.Identity < items[j].in.Identity
	})
	compat, err := compatible(items)
	if err != nil {
		return Result{}, err
	}
	nodes := map[string]graph.Node{}
	edges := map[string]graph.Edge{}
	occurrenceCount := 0
	per := make([]json.RawMessage, len(items))
	allComplete := true
	anyTruncated := false
	constituents := make([]Constituent, len(items))
	for i, x := range items {
		constituents[i] = x.c
		per[i] = cloneRaw(x.n.Summary)
		var s summary
		if err := json.Unmarshal(x.n.Summary, &s); err != nil {
			return Result{}, err
		}
		allComplete = allComplete && s.TraversalComplete
		anyTruncated = anyTruncated || s.Truncated
		for _, n := range x.n.Nodes {
			if old, ok := nodes[n.ID]; ok && !canonicalEqual(old, n) {
				return Result{}, fmt.Errorf("node id conflict %q", n.ID)
			}
			nodes[n.ID] = n
		}
		for _, e := range x.n.Edges {
			occurrenceCount += len(e.CallSites)
			if occurrenceCount > programc.MaxOccurrences {
				return Result{}, errors.New("occurrence cap exceeded")
			}
			if old, ok := edges[e.RelationID]; ok {
				if !canonicalEqual(old, e) {
					return Result{}, fmt.Errorf("edge id conflict %q", e.RelationID)
				}
			} else {
				edges[e.RelationID] = e
			}
		}
	}
	if err := enforceAggregateCaps(total, len(nodes), occurrenceCount); err != nil {
		return Result{}, err
	}
	ns := make([]graph.Node, 0, len(nodes))
	for _, n := range nodes {
		ns = append(ns, n)
	}
	sort.Slice(ns, func(i, j int) bool { return ns[i].ID < ns[j].ID })
	es := make([]graph.Edge, 0, len(edges))
	for _, e := range edges {
		es = append(es, e)
	}
	sort.Slice(es, func(i, j int) bool { return es[i].RelationID < es[j].RelationID })
	a := Artifact{Version: Version, PolicyVersion: PolicyVersion, PolicySHA256: PolicyDigest(), ClaimCeiling: ClaimCeiling, Constituents: constituents, Compatibility: compat, Nodes: ns, Edges: es, Completeness: Completeness{AllTraversalComplete: allComplete, AnyTruncated: anyTruncated, WholeWorkspace: false, PerInput: per}}
	pre, _ := json.Marshal(a)
	a.CompositeID = digest("lsp-trace:program-c-compose:identity:v1", pre)
	pre, _ = json.Marshal(a)
	a.OutputSHA256 = digest("lsp-trace:program-c-compose:output:v1", pre)
	out, _ := json.Marshal(a)
	out = append(out, '\n')
	return Result{Artifact: a, Bytes: out}, nil
}

func Validate(raw []byte) (Artifact, error) {
	var a Artifact
	if err := strictDecode(raw, &a); err != nil {
		return a, err
	}
	if a.Version != Version || a.PolicyVersion != PolicyVersion || a.PolicySHA256 != PolicyDigest() || a.ClaimCeiling != ClaimCeiling {
		return a, errors.New("composite policy identity mismatch")
	}
	out := a.OutputSHA256
	a.OutputSHA256 = ""
	pre, _ := json.Marshal(a)
	if out != digest("lsp-trace:program-c-compose:output:v1", pre) {
		return a, errors.New("composite output digest mismatch")
	}
	a.OutputSHA256 = out
	id := a.CompositeID
	a.CompositeID = ""
	a.OutputSHA256 = ""
	pre, _ = json.Marshal(a)
	if id != digest("lsp-trace:program-c-compose:identity:v1", pre) {
		return a, errors.New("composite identity mismatch")
	}
	for _, c := range a.Constituents {
		b, err := base64.StdEncoding.DecodeString(c.BytesBase64)
		if err != nil || len(b) != c.ByteLength || rawDigest(b) != c.SHA256 {
			return a, errors.New("constituent replay mismatch")
		}
		if _, err = graphprovenance.ValidateFor(b, graphprovenance.Family, "v5"); err != nil {
			return a, err
		}
	}
	return a, nil
}

func compatible(xs []admitted) (Compatibility, error) {
	first := xs[0]
	c := Compatibility{WorkspaceURI: first.in.ExactMetadata.WorkspaceIdentity, SourceRevision: first.n.Invocation.Provenance.SourceRevision, InvocationID: first.n.Invocation.Provenance.InvocationID, PositionEncoding: first.in.ExactMetadata.PositionEncoding, RevisionCustody: first.in.ExactMetadata.RevisionCustody, AcquisitionSemantics: first.in.ExactMetadata.AcquisitionSemantics, PrivacyPolicy: first.in.ExactMetadata.PrivacyPolicy, ServerCommand: first.n.Invocation.Server.Command, ServerVersion: first.n.Invocation.Provenance.ServerVersion, LanguageID: language(first.n), EvidenceSemantics: cloneRaw(first.n.EvidenceSemantics), SensitivityPolicy: cloneRaw(first.n.SensitivityPolicy)}
	if c.WorkspaceURI == "" || c.SourceRevision == "" || c.InvocationID == "" || c.ServerCommand == "" || c.ServerVersion == "" || c.LanguageID == "" || len(c.EvidenceSemantics) == 0 || len(c.SensitivityPolicy) == 0 {
		return c, errors.New("ambiguous required compatibility identity")
	}
	for _, x := range xs[1:] {
		if x.env.SessionID != first.env.SessionID || x.env.Generation != first.env.Generation || x.in.ExactMetadata != first.in.ExactMetadata || x.n.Invocation.Provenance.SourceRevision != c.SourceRevision || x.n.Invocation.Provenance.InvocationID != c.InvocationID || x.n.Invocation.Server.Command != c.ServerCommand || x.n.Invocation.Provenance.ServerVersion != c.ServerVersion || language(x.n) != c.LanguageID || !bytes.Equal(x.n.EvidenceSemantics, c.EvidenceSemantics) || !bytes.Equal(x.n.SensitivityPolicy, c.SensitivityPolicy) {
			return c, errors.New("capture compatibility mismatch")
		}
	}
	return c, nil
}
func enforceAggregateCaps(inputBytes, nodes, occurrences int) error {
	if nodes > programc.MaxNodes {
		return errors.New("node cap exceeded")
	}
	if occurrences > programc.MaxOccurrences {
		return errors.New("occurrence cap exceeded")
	}
	if inputBytes+nodes+occurrences > MaxWorkUnits {
		return errors.New("work unit cap exceeded")
	}
	return nil
}

func language(n native) string {
	if n.Invocation.LanguageID != "" {
		return n.Invocation.LanguageID
	}
	if len(n.Invocation.Seeds) > 0 {
		v := n.Invocation.Seeds[0].LanguageID
		for _, s := range n.Invocation.Seeds {
			if s.LanguageID != v {
				return ""
			}
		}
		return v
	}
	return ""
}
func strictDecode(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != nil {
		return nil
	}
	return errors.New("trailing JSON")
}
func canonicalEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func cloneRaw(x json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), x...) }
func rawDigest(b []byte) string                  { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func digest(domain string, b []byte) string {
	s := sha256.Sum256(append(append([]byte(domain), 0), b...))
	return "sha256:" + hex.EncodeToString(s[:])
}
