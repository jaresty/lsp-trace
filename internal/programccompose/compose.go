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
)

const (
	Version            = "lsp-trace.private.program-c-multi-capture-composite.v1"
	PolicyVersion      = "program-c-multi-capture-policy.v1"
	MaxInputs          = 16
	MaxTotalInputBytes = 256 << 20
	MaxWorkUnits       = 400_000
	maxNodes           = 10_000
	maxOccurrences     = 100_000
	ClaimCeiling       = "STRUCTURAL_SERVER_REPORTED_CALLS_UNION_ONLY;NO_WHOLE_WORKSPACE_COMPLETENESS;NO_FEATURE_IDENTITY;NO_OWNERSHIP;NO_ARCHITECTURE;NO_RUNTIME_EXECUTION;NO_PRODUCER_AUTHENTICATION;NO_PERMISSION;NO_PRODUCTION_AUTHORITY"
)

var PolicyBytes = []byte("program-c-multi-capture-policy.v1\ninputs=2..16\ntotal_input_bytes<=268435456\nsemantic_work_units=inputs+traversed_node_records+traversed_edge_records+traversed_occurrences+traversed_supply_capture_binding_and_native_receipt_records<=400000\nnodes<=10000\noccurrences<=100000\nsame_session_generation_revision_custody_workspace_encoding_provider_language_acquisition_evidence_sensitivity_privacy\ndistinct_invocations_preserved_and_canonical_identity_bound;no_homogeneous_invocation_claim\nexact-typed-id-and-content-digest-equivalence-dedupe;conflict-fails\nno-inference;no-truncation;constituent_source_supply_capture_binding_seeds_traversal_frontier_completeness_diagnostics_preserved\nretained_bytes_never_select_nodes;no_forged_producer_provenance;not_one_native_capture;conservative_claim_ceiling\n")

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
	SessionID, InvocationID                                     string
	Generation                                                  uint64
	SourcePolicy, WorkspaceURI, AnalyzedVersion                 string
	DependencyCompleteness                                      string
	CaptureBudget                                               graphprovenance.CaptureBudgetV2
	Supplies                                                    []graphprovenance.SupplyReceiptV2
	Captures                                                    []graphprovenance.Receipt
	Bindings                                                    []graphprovenance.BindingV2
	Invocation, Seeds, Frontier, Diagnostics, Summary, Slice    json.RawMessage
}

type Compatibility struct {
	WorkspaceURI, SourceRevision, PositionEncoding       string
	RevisionCustody, AcquisitionSemantics, PrivacyPolicy string
	ServerCommand, ServerVersion, LanguageID             string
	EvidenceSemantics, SensitivityPolicy                 json.RawMessage
}

type Completeness struct {
	AllTraversalComplete bool
	AnyTruncated         bool
	WholeWorkspace       bool
	PerInput             []json.RawMessage
}

type WorkAccounting struct {
	Inputs, NodeRecords, EdgeRecords, Occurrences, SupplyRecords, SourceReceiptRecords, CaptureRecords, BindingRecords, NativeReceiptRecords, MergedNodes, SemanticWork int
}

type Artifact struct {
	Version, PolicyVersion, PolicySHA256, CompositeID, OutputSHA256, ClaimCeiling string
	Constituents                                                                  []Constituent
	Compatibility                                                                 Compatibility
	Work                                                                          WorkAccounting
	Nodes                                                                         []graph.Node
	Edges                                                                         []graph.Edge
	Completeness                                                                  Completeness
}

type Result struct {
	Artifact Artifact
	Bytes    []byte
}

type envelope = graphprovenance.EvidenceV5

type native struct {
	SchemaVersion         string          `json:"schema_version"`
	ExecutionBundleID     json.RawMessage `json:"execution_bundle_id"`
	Tool                  json.RawMessage `json:"tool"`
	Invocation            json.RawMessage `json:"invocation"`
	Identity              json.RawMessage `json:"identity"`
	SensitivityPolicy     json.RawMessage `json:"sensitivity_policy"`
	ProcessContext        json.RawMessage `json:"process_context"`
	EvidenceSemantics     json.RawMessage `json:"evidence_semantics"`
	EvidenceReceipt       json.RawMessage `json:"evidence_receipt"`
	SeedMemberships       json.RawMessage `json:"seed_memberships"`
	ReplayInputManifest   json.RawMessage `json:"replay_input_manifest"`
	PortableLocators      json.RawMessage `json:"portable_locators"`
	Capabilities          json.RawMessage `json:"capabilities"`
	CapabilityQuality     json.RawMessage `json:"capability_quality"`
	Targets               json.RawMessage `json:"targets"`
	Nodes                 []graph.Node    `json:"nodes"`
	Edges                 []graph.Edge    `json:"edges"`
	Terminals             json.RawMessage `json:"terminals"`
	Frontier              json.RawMessage `json:"frontier"`
	Diagnostics           json.RawMessage `json:"diagnostics"`
	SiblingCandidates     json.RawMessage `json:"sibling_candidates"`
	DispatchRelationships json.RawMessage `json:"dispatch_relationships"`
	Seeds                 json.RawMessage `json:"seeds"`
	Slice                 json.RawMessage `json:"slice"`
	Summary               json.RawMessage `json:"summary"`
	TraceReceipt          json.RawMessage `json:"trace_receipt"`
	inv                   graph.Invocation
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
		n, err := parseNative(gb)
		if err != nil {
			return Result{}, fmt.Errorf("input %d graph: %w", i, err)
		}
		items[i] = admitted{in: in, env: e, n: n, c: constituent(in, e, gb, n)}
	}
	if err := validateSourceRecords(items); err != nil {
		return Result{}, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].c.InvocationID != items[j].c.InvocationID {
			return items[i].c.InvocationID < items[j].c.InvocationID
		}
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
	work := WorkAccounting{Inputs: len(items)}
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
		work.NodeRecords += len(x.n.Nodes)
		work.EdgeRecords += len(x.n.Edges)
		work.SupplyRecords += len(x.env.Supplies)
		for _, supply := range x.env.Supplies {
			if supply.Receipt != nil {
				work.SourceReceiptRecords++
			}
		}
		work.CaptureRecords += len(x.env.Captures)
		work.BindingRecords += len(x.env.Bindings)
		work.NativeReceiptRecords += receiptRecordCount(x.n.EvidenceReceipt)
		for _, n := range x.n.Nodes {
			if old, ok := nodes[n.ID]; ok && !canonicalEqual(old, n) {
				return Result{}, fmt.Errorf("node id conflict %q", n.ID)
			}
			nodes[n.ID] = n
		}
		for _, e := range x.n.Edges {
			occurrenceCount += len(e.CallSites)
			work.Occurrences += len(e.CallSites)
			if occurrenceCount > maxOccurrences {
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
	work.MergedNodes = len(nodes)
	work.SemanticWork = semanticWork(work)
	if err := enforceResourceCaps(total, work); err != nil {
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
	a := Artifact{Version: Version, PolicyVersion: PolicyVersion, PolicySHA256: PolicyDigest(), ClaimCeiling: ClaimCeiling, Constituents: constituents, Compatibility: compat, Work: work, Nodes: ns, Edges: es, Completeness: Completeness{AllTraversalComplete: allComplete, AnyTruncated: anyTruncated, WholeWorkspace: false, PerInput: per}}
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
	replayInputs := make([]Input, len(a.Constituents))
	for i, c := range a.Constituents {
		if i > 0 && constituentAfter(a.Constituents[i-1], c) {
			return a, errors.New("constituent canonical order mismatch")
		}
		b, err := base64.StdEncoding.DecodeString(c.BytesBase64)
		if err != nil || len(b) != c.ByteLength || rawDigest(b) != c.SHA256 {
			return a, errors.New("constituent replay mismatch")
		}
		if _, err = graphprovenance.ValidateFor(b, graphprovenance.Family, "v5"); err != nil {
			return a, err
		}
		var e envelope
		if err = strictDecode(b, &e); err != nil {
			return a, err
		}
		gb, decodeErr := base64.StdEncoding.DecodeString(e.GraphV5)
		if decodeErr != nil {
			return a, decodeErr
		}
		n, parseErr := parseNative(gb)
		if parseErr != nil {
			return a, parseErr
		}
		want := constituent(Input{Bytes: b, Identity: c.Identity, SHA256: c.SHA256, ByteLength: len(b)}, e, gb, n)
		if !canonicalEqual(c, want) {
			return a, errors.New("constituent projected binding mismatch")
		}
		replayInputs[i] = Input{Bytes: b, Identity: c.Identity, SHA256: c.SHA256, ByteLength: c.ByteLength, ExactMetadata: ExactMetadata{WorkspaceIdentity: a.Compatibility.WorkspaceURI, RevisionCustody: a.Compatibility.RevisionCustody, PositionEncoding: a.Compatibility.PositionEncoding, AcquisitionSemantics: a.Compatibility.AcquisitionSemantics, PrivacyPolicy: a.Compatibility.PrivacyPolicy}}
	}
	if a.Work.SemanticWork != semanticWork(a.Work) {
		return a, errors.New("semantic work accounting mismatch")
	}
	replayed, err := Compose(replayInputs)
	if err != nil {
		return a, fmt.Errorf("constituent recomposition failed: %w", err)
	}
	if !bytes.Equal(raw, replayed.Bytes) {
		return a, errors.New("composite does not equal canonical constituent recomposition")
	}
	return a, nil
}

func compatible(xs []admitted) (Compatibility, error) {
	first := xs[0]
	c := Compatibility{WorkspaceURI: first.in.ExactMetadata.WorkspaceIdentity, SourceRevision: first.n.inv.Provenance.SourceRevision, PositionEncoding: first.in.ExactMetadata.PositionEncoding, RevisionCustody: first.in.ExactMetadata.RevisionCustody, AcquisitionSemantics: first.in.ExactMetadata.AcquisitionSemantics, PrivacyPolicy: first.in.ExactMetadata.PrivacyPolicy, ServerCommand: first.n.inv.Server.Command, ServerVersion: first.n.inv.Provenance.ServerVersion, LanguageID: language(first.n), EvidenceSemantics: cloneRaw(first.n.EvidenceSemantics), SensitivityPolicy: cloneRaw(first.n.SensitivityPolicy)}
	if c.WorkspaceURI == "" || c.SourceRevision == "" || first.n.inv.Provenance.InvocationID == "" || c.ServerCommand == "" || c.ServerVersion == "" || c.LanguageID == "" || len(c.EvidenceSemantics) == 0 || len(c.SensitivityPolicy) == 0 {
		return c, errors.New("ambiguous required compatibility identity")
	}
	for _, x := range xs[1:] {
		if x.n.inv.Provenance.InvocationID == "" || x.env.SessionID != first.env.SessionID || x.env.Generation != first.env.Generation || x.in.ExactMetadata != first.in.ExactMetadata || x.n.inv.Provenance.SourceRevision != c.SourceRevision || x.n.inv.Server.Command != c.ServerCommand || x.n.inv.Provenance.ServerVersion != c.ServerVersion || language(x.n) != c.LanguageID || !bytes.Equal(x.n.EvidenceSemantics, c.EvidenceSemantics) || !bytes.Equal(x.n.SensitivityPolicy, c.SensitivityPolicy) {
			return c, errors.New("capture compatibility mismatch")
		}
	}
	return c, nil
}
func enforceResourceCaps(inputBytes int, work WorkAccounting) error {
	if inputBytes > MaxTotalInputBytes {
		return errors.New("total input byte cap exceeded")
	}
	if work.MergedNodes > maxNodes {
		return errors.New("node cap exceeded")
	}
	if work.Occurrences > maxOccurrences {
		return errors.New("occurrence cap exceeded")
	}
	if semanticWork(work) > MaxWorkUnits {
		return errors.New("semantic work unit cap exceeded")
	}
	return nil
}

func semanticWork(w WorkAccounting) int {
	return w.Inputs + w.NodeRecords + w.EdgeRecords + w.Occurrences + w.SupplyRecords + w.SourceReceiptRecords + w.CaptureRecords + w.BindingRecords + w.NativeReceiptRecords
}

func language(n native) string {
	if n.inv.LanguageID != "" {
		return n.inv.LanguageID
	}
	if len(n.inv.Seeds) > 0 {
		v := n.inv.Seeds[0].LanguageID
		for _, s := range n.inv.Seeds {
			if s.LanguageID != v {
				return ""
			}
		}
		return v
	}
	return ""
}
func parseNative(b []byte) (native, error) {
	var n native
	if err := strictDecode(b, &n); err != nil {
		return native{}, fmt.Errorf("unsupported Graph V5 shape: %w", err)
	}
	if err := strictDecode(n.Invocation, &n.inv); err != nil {
		return native{}, fmt.Errorf("invocation: %w", err)
	}
	return n, nil
}

func constituent(in Input, e envelope, gb []byte, n native) Constituent {
	supplies := append([]graphprovenance.SupplyReceiptV2(nil), e.Supplies...)
	captures := append([]graphprovenance.Receipt(nil), e.Captures...)
	bindings := append([]graphprovenance.BindingV2(nil), e.Bindings...)
	sort.Slice(supplies, func(i, j int) bool { return supplies[i].RequestID < supplies[j].RequestID })
	sort.Slice(captures, func(i, j int) bool { return captures[i].ID < captures[j].ID })
	sort.Slice(bindings, func(i, j int) bool {
		a, _ := json.Marshal(bindings[i])
		b, _ := json.Marshal(bindings[j])
		return bytes.Compare(a, b) < 0
	})
	return Constituent{Identity: in.Identity, SHA256: in.SHA256, ByteLength: len(in.Bytes), SchemaVersion: e.SchemaVersion, GraphSHA256: e.GraphV5SHA256, GraphByteLength: len(gb), GraphSchemaID: e.GraphV5SchemaID, BytesBase64: base64.StdEncoding.EncodeToString(in.Bytes), SessionID: e.SessionID, Generation: e.Generation, InvocationID: n.inv.Provenance.InvocationID, SourcePolicy: e.SourcePolicy, WorkspaceURI: e.WorkspaceURI, AnalyzedVersion: e.AnalyzedVersion, DependencyCompleteness: e.DependencyCompleteness, CaptureBudget: e.CaptureBudget, Supplies: supplies, Captures: captures, Bindings: bindings, Invocation: cloneRaw(n.Invocation), Seeds: cloneRaw(n.Seeds), Frontier: cloneRaw(n.Frontier), Diagnostics: cloneRaw(n.Diagnostics), Summary: cloneRaw(n.Summary), Slice: cloneRaw(n.Slice)}
}

func validateSourceRecords(items []admitted) error {
	receiptIDs := map[string]any{}
	requestIDs := map[string]any{}
	contentDigests := map[string]any{}
	bindings := map[string]any{}
	for _, x := range items {
		for _, s := range x.env.Supplies {
			if err := exactRecord(requestIDs, "supply request id", s.RequestID, s); err != nil {
				return err
			}
			if s.Receipt != nil {
				if err := exactRecord(receiptIDs, "receipt id", s.Receipt.ID, *s.Receipt); err != nil {
					return err
				}
				if err := exactRecord(contentDigests, "content digest", rawDigest(s.Receipt.Content), *s.Receipt); err != nil {
					return err
				}
			}
		}
		for _, r := range x.env.Captures {
			if err := exactRecord(receiptIDs, "receipt id", r.ID, r); err != nil {
				return err
			}
			if err := exactRecord(contentDigests, "content digest", rawDigest(r.Content), r); err != nil {
				return err
			}
		}
		for _, b := range x.env.Bindings {
			keyBytes, _ := json.Marshal(b)
			if err := exactRecord(bindings, "binding", string(keyBytes), b); err != nil {
				return err
			}
		}
	}
	return nil
}

func exactRecord(seen map[string]any, kind, key string, value any) error {
	if key == "" {
		return fmt.Errorf("empty %s", kind)
	}
	if old, ok := seen[key]; ok && !canonicalEqual(old, value) {
		return fmt.Errorf("%s conflict %q", kind, key)
	}
	seen[key] = value
	return nil
}

func receiptRecordCount(raw json.RawMessage) int {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0
	}
	var receipt graph.EvidenceReceipt
	if err := strictDecode(raw, &receipt); err != nil {
		// The enclosing V5 validation has already accepted the bytes. Returning the
		// cap-saturating value fails closed if this old adapter cannot count them.
		return MaxWorkUnits + 1
	}
	return 1 + len(receipt.Relations)
}

func constituentAfter(a, b Constituent) bool {
	if a.InvocationID != b.InvocationID {
		return a.InvocationID > b.InvocationID
	}
	if a.SHA256 != b.SHA256 {
		return a.SHA256 > b.SHA256
	}
	return a.Identity > b.Identity
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
