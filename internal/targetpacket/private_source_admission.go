package targetpacket

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	methodsyntax "lsp-trace/internal/adr0011methodsyntax"
	qt "lsp-trace/internal/adr0011querytarget"
	rg "lsp-trace/internal/referencegrouping"
	rp "lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceposition"
	"lsp-trace/sessionruntime"
	"time"
)

const PrivateSourceVersion = "lsp-trace.private.source-admission.v1"
const PrivateSourceCeiling = "SOURCE_UNIT_ONLY;AUTHORITY_0;ACCEPTED_FALSE;IDENTITY_UNRESOLVED;COMPLETENESS_UNKNOWN;ABSENCE_PROHIBITED"
const PrivateSourceCandidateCeiling = "NONADMITTING_CANDIDATE;NO_PRODUCER_COMPLETION_OR_SOURCE_ADMISSION"

var ErrPrivateSource = errors.New("private source binding refused")

type PrivateSourceLimits struct{ MaxDocumentBytes, MaxTotalBytes, MaxWork, MaxObjects int }

func (l PrivateSourceLimits) Valid() bool {
	return l.MaxDocumentBytes > 0 && l.MaxDocumentBytes <= 1<<20 && l.MaxTotalBytes > 0 && l.MaxTotalBytes <= 8<<20 && l.MaxWork > 0 && l.MaxWork <= 10000 && l.MaxObjects > 0 && l.MaxObjects <= 1000
}

// CandidateOriginals is only raw material for bounded comparison. It conveys NO
// producer completion or source admission. Even a real Manager token is not the
// inventory producer's Finish/Observe barrier. Only that producer can admit.
type CandidateOriginals struct {
	state    *candidateOriginals
	prepared *preparedSourceCandidate
}
type candidateOriginals struct {
	evidence     sessionruntime.PrivateGroupingSourceEvidence
	input        rg.AdmittedInput
	predecessors []rg.PredecessorRecord
	query        []byte
	deadline     time.Time
	workspace    string
	bound        uint64
}
type projectedSourceNode struct {
	ID             string   `json:"id"`
	URI            string   `json:"uri"`
	SelectionRange qt.Range `json:"selection_range"`
}

// Plain local DTOs deliberately have no validator/UnmarshalJSON methods. The
// initial paid producer already admitted this exact immutable composite. This
// projection does not rebuild directional graphs, compose, or re-admit input.
type sourceCompositeProjection struct {
	Nodes         []projectedSourceNode
	Compatibility struct{ PositionEncoding, SourceRevision, PrivacyPolicy string }
	Constituents  []struct {
		SessionID  string
		Generation uint64
		Captures   []struct {
			ID      string `json:"id"`
			URI     string `json:"uri"`
			Status  string `json:"status"`
			Content []byte `json:"content"`
		}
		Bindings []struct {
			URI         string   `json:"uri"`
			Attribution string   `json:"attribution"`
			ReceiptIDs  []string `json:"receipt_ids"`
		}
	}
}
type preparedSourceCandidate struct {
	view          sessionruntime.PrivateGroupingOwnedView
	contributions []rg.Contribution
	nodes         map[string]projectedSourceNode
	sources       map[string][][]byte
}

func RetainPrivateSourceCandidate(e sessionruntime.PrivateGroupingSourceEvidence, in rg.AdmittedInput, p []rg.PredecessorRecord, query []byte, deadline time.Time, workspace string) (CandidateOriginals, error) {
	b, ok := e.EncodedBound()
	if !ok || len(workspace) == 0 || len(workspace) > 4096 || in.ID() == "" || in.ByteLength() <= 0 || in.ByteLength() > 1<<20 || len(p) > 8 || len(query) == 0 || len(query) > 1<<20 || deadline.IsZero() {
		return CandidateOriginals{}, ErrPrivateSource
	}
	total := uint64(in.ByteLength() + len(query))
	for _, x := range p {
		if len(x.Bytes) > 1<<20 || len(x.Role) > 4096 || len(x.Selector) > 4096 || len(x.Schema) > 4096 {
			return CandidateOriginals{}, ErrPrivateSource
		}
		total += uint64(len(x.Bytes))
	}
	if total > 3<<20 {
		return CandidateOriginals{}, ErrPrivateSource
	}
	return CandidateOriginals{state: &candidateOriginals{e, in, p, query, deadline, workspace, b + total}}, nil
}
func (o CandidateOriginals) Workspace() string {
	if o.state == nil {
		return ""
	}
	return o.state.workspace
}
func (o CandidateOriginals) InputID() string {
	if o.state == nil {
		return ""
	}
	return o.state.input.ID()
}
func (o CandidateOriginals) Deadline() time.Time {
	if o.state == nil {
		return time.Time{}
	}
	return o.state.deadline
}
func (o CandidateOriginals) SourceByteLength() int {
	if o.state == nil {
		return 0
	}
	return o.state.evidence.SourceByteLength()
}
func (o CandidateOriginals) Bound() uint64 {
	if o.state == nil {
		return 0
	}
	s := o.state
	b, ok := s.evidence.EncodedBound()
	if !ok || len(s.predecessors) > 8 || len(s.query) == 0 || len(s.query) > 1<<20 || s.input.ByteLength() > 1<<20 {
		return 0
	}
	total := uint64(s.input.ByteLength() + len(s.query))
	for _, p := range s.predecessors {
		if len(p.Bytes) > 1<<20 || len(p.Role) > 4096 || len(p.Selector) > 4096 || len(p.Schema) > 4096 {
			return 0
		}
		total += uint64(len(p.Bytes))
	}
	if total > 3<<20 || total+b != s.bound {
		return 0
	}
	return s.bound
}

type PrivateSourceUnit struct {
	Version, ID, InputID, URI, SourceDigest, Revision, Privacy, Workspace string
	SessionID, DocumentVersion, PositionEncoding, SymbolID                string
	Generation                                                            uint64
	DisplayRange, SelectionRange                                          qt.Range
	Body, ClaimCeiling                                                    string
}

// PrivateSourceCandidate is NOT admitted and cannot be passed as producer proof.
// It can only be compared by the upper producer gate to its own exact originals.
type PrivateSourceCandidate struct{ state *sourceCandidate }
type sourceCandidate struct {
	intent    PrivateSourceIntent
	unit      PrivateSourceUnit
	originals CandidateOriginals
}

func (a PrivateSourceCandidate) Valid() bool { return a.state != nil }
func (a PrivateSourceCandidate) Unit() (PrivateSourceUnit, error) {
	if a.state == nil {
		return PrivateSourceUnit{}, ErrPrivateSource
	}
	return a.state.unit, nil
}
func (a PrivateSourceCandidate) Matches(i PrivateSourceIntent, o CandidateOriginals) bool {
	return a.state != nil && a.state.intent == i && o.state != nil && a.state.originals.state == o.state
}
func privateSourceDigest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

type sourceNativePair struct {
	Method, SessionID     string
	Generation, RequestID uint64
	Params, Result        []byte
	Write                 sessionruntime.RequestWriteObservation
	Read                  sessionruntime.ResponseReadObservation
	Source                *sessionruntime.OwnedDocumentBinding
}
type sourceSupply struct {
	Classification, SessionID string
	Generation                uint64
	URI                       string
	DocumentVersion           int
	Method                    string
	Content, Params           []byte
}

func sourcePair(p sessionruntime.OwnedMethodPair) sourceNativePair {
	return sourceNativePair{p.Method, p.SessionID, p.Generation, p.Key.ID, p.Params, p.Result, p.Write, p.Read, p.Source}
}
func sourceQueryBytes(v sessionruntime.PrivateGroupingOwnedView) []byte {
	methods := make([]sourceNativePair, len(v.Methods))
	for i, p := range v.Methods {
		methods[i] = sourcePair(p)
	}
	raw, _ := json.Marshal(struct {
		Query                      qt.Query
		Target                     qt.Candidate
		Source                     []byte
		Supply                     sourceSupply
		Metadata                   sessionruntime.SessionMetadata
		Revision, Custody, Privacy string
		Workspace                  string
		Deadline                   time.Time
		Symbol                     sourceNativePair
		Methods                    []sourceNativePair
		Unavailable                []string
	}{v.Query, v.Target, v.Source, sourceSupply{v.Supply.Classification, v.Supply.SessionID, v.Supply.Generation, v.Supply.URI, v.Supply.DocumentVersion, v.Supply.Method, v.Supply.Content, v.Supply.Params}, v.Metadata, v.Revision, "CALLER_ASSERTED", v.Privacy, v.Workspace, v.Deadline, sourcePair(v.Symbol), methods, v.Unavailable})
	return append(raw, '\n')
}
func sourceRange(r methodsyntax.Range) qt.Range {
	return qt.Range{Start: qt.Position{Line: r.Start.Line, Character: r.Start.Character}, End: qt.Position{Line: r.End.Line, Character: r.End.Character}}
}
func sameScope(a, b sessionruntime.PrivateGroupingOwnedView) bool {
	return a.Query.SessionID == b.Query.SessionID && a.Query.Generation == b.Query.Generation && a.Query.Encoding == b.Query.Encoding && a.Revision == b.Revision && a.Privacy == b.Privacy && a.Workspace == b.Workspace
}
func exactSourceView(v sessionruntime.PrivateGroupingOwnedView) bool {
	q := v.Query
	target, err := qt.SelectDocumentSymbolCandidateV1(q, v.Symbol.Result)
	return err == nil && target == v.Target && v.Symbol.Method == "textDocument/documentSymbol" && v.Symbol.SessionID == q.SessionID && v.Symbol.Generation == q.Generation && v.Symbol.Source != nil && v.Symbol.Source.URI == q.URI && fmt.Sprint(v.Symbol.Source.Version) == q.DocumentVersion && v.Symbol.Source.SHA256 == q.SourceDigest && v.Supply.Classification == "LSP_SUPPLIED" && v.Supply.SessionID == q.SessionID && v.Supply.Generation == q.Generation && v.Supply.URI == q.URI && fmt.Sprint(v.Supply.DocumentVersion) == q.DocumentVersion && bytes.Equal(v.Source, v.Supply.Content) && privateSourceDigest(v.Source) == q.SourceDigest && v.Metadata.PositionEncoding == q.Encoding
}

func PreparePrivateSourceCandidate(original CandidateOriginals, limits PrivateSourceLimits) (CandidateOriginals, error) {
	bad := func(s string) (CandidateOriginals, error) { return CandidateOriginals{}, errors.New(s) }
	if !limits.Valid() || original.state == nil || original.Bound() == 0 || original.Bound() > 8<<20 || original.SourceByteLength() > limits.MaxDocumentBytes {
		return bad("SOURCE_BOUND")
	}
	o := original.state
	v, err := o.evidence.Snapshot(sessionruntime.PrivateGroupingQueryMaxBytes)
	if err != nil || len(v.Source) > limits.MaxDocumentBytes || !v.Deadline.Equal(o.deadline) || v.Workspace != o.workspace || !exactSourceView(v) || !bytes.Equal(sourceQueryBytes(v), o.query) {
		return bad("ORIGINAL_QUERY_PROOF_MISMATCH")
	}
	// Contributions are already privately admitted. Never parse/re-admit them here.
	cs := o.input.Contributions()
	if len(cs)+32 > limits.MaxWork {
		return bad("SOURCE_WORK_BOUND")
	}
	nodes := map[string]projectedSourceNode{}
	sources := map[string][][]byte{}
	for _, p := range o.predecessors {
		if p.Role != "RETAINED_COMPOSITE" {
			continue
		}
		// Finite byte bound BEFORE decoding; each decoded collection and loop is also
		// bounded. Count JSON structural tokens before allocation, not after decode.
		if len(p.Bytes) == 0 || len(p.Bytes) > 1<<20 {
			return bad("SOURCE_COMPOSITE_BOUND")
		}
		work := len(cs) + 32
		quoted, escaped := false, false
		for _, ch := range p.Bytes {
			if quoted {
				if escaped {
					escaped = false
				} else if ch == '\\' {
					escaped = true
				} else if ch == '"' {
					quoted = false
				}
				continue
			}
			if ch == '"' {
				quoted = true
			}
			if ch == '{' || ch == '[' || ch == ',' {
				work++
			}
			if work > limits.MaxWork {
				return bad("SOURCE_PROJECTION_WORK_BOUND")
			}
		}
		var c sourceCompositeProjection
		if json.Unmarshal(p.Bytes, &c) != nil {
			return bad("SOURCE_COMPOSITE_PROJECTION")
		}
		if len(c.Nodes) > limits.MaxObjects || len(c.Constituents) > limits.MaxObjects {
			return bad("SOURCE_PROJECTION_OBJECT_BOUND")
		}
		if c.Compatibility.PositionEncoding != v.Query.Encoding || c.Compatibility.SourceRevision != v.Revision || c.Compatibility.PrivacyPolicy != v.Privacy {
			return bad("NATIVE_SOURCE_SCOPE_MISMATCH")
		}
		loops := 0
		for _, part := range c.Constituents {
			if part.SessionID != v.Query.SessionID || part.Generation != v.Query.Generation {
				return bad("NATIVE_SOURCE_SESSION_MISMATCH")
			}
			for _, binding := range part.Bindings {
				if binding.Attribution != "SOURCE" {
					continue
				}
				for _, receipt := range part.Captures {
					loops++
					if loops > limits.MaxWork {
						return bad("SOURCE_WORK_BOUND")
					}
					if receipt.URI != binding.URI || receipt.Status != "READABLE" {
						continue
					}
					for _, id := range binding.ReceiptIDs {
						loops++
						if loops > limits.MaxWork {
							return bad("SOURCE_WORK_BOUND")
						}
						if id == receipt.ID {
							sources[binding.URI] = append(sources[binding.URI], receipt.Content)
						}
					}
				}
			}
		}
		for _, n := range c.Nodes {
			nodes[n.ID] = n
		}
	}
	original.prepared = &preparedSourceCandidate{v, cs, nodes, sources}
	return original, nil
}

// CalculatePrivateEndpointSource is a pure, NONADMITTING comparison. It cannot
// promote Manager evidence into inventory producer completion or eligibility.
func CalculatePrivateEndpointSource(intent PrivateSourceIntent, original CandidateOriginals, definitions []sessionruntime.PrivateGroupingSourceEvidence, objects []sourceobject.Object, limits PrivateSourceLimits) (PrivateSourceCandidate, string) {
	gap := func(s string) (PrivateSourceCandidate, string) { return PrivateSourceCandidate{}, s }
	if !limits.Valid() || original.state == nil {
		return gap("NO_OPAQUE_PRODUCER_ORIGINALS")
	}
	o := original.state
	if intent.BindingState != "UNRESOLVED" || intent.Requirement != "REQUIRES_ADMITTED_SOURCE_BINDING_AND_FULL_DEFINITION" || intent.ClaimCeiling != rg.EvaluationCeilingPrivate || intent.InputID != o.input.ID() || intent.NominationID == "" || intent.Mode == "" || intent.RunID == "" || intent.PolicyID == "" || intent.CaseID == "" || intent.ResultID == "" || intent.Lineage == "" || intent.Slot < 0 || intent.Community < 0 || !privateEndpointRole(intent.Endpoint.Kind, intent.Endpoint.Role) {
		return gap("INTENT_OR_INPUT_BINDING_MISMATCH")
	}
	if _, err := rg.ReadAllModeMetadataPrivate(o.input); err != nil {
		return gap("INPUT_BOUND")
	}
	for _, s := range []string{intent.NominationID, intent.Mode, intent.RunID, intent.InputID, intent.PolicyID, intent.CaseID, intent.ResultID, intent.Lineage, intent.Endpoint.Node, intent.Endpoint.Kind, intent.Endpoint.Role, intent.Endpoint.Occurrence, intent.Endpoint.Predecessor, intent.Endpoint.Custody, intent.Endpoint.Disposition} {
		if len(s) > 4096 {
			return gap("INTENT_BOUND")
		}
	}
	if len(definitions) > limits.MaxObjects || len(objects) > limits.MaxObjects || original.Bound() > 8<<20 || len(definitions)+len(objects)+len(o.predecessors)+32 > limits.MaxWork || original.Bound() == 0 || original.SourceByteLength() > limits.MaxDocumentBytes {
		return gap("SOURCE_WORK_BOUND")
	}
	total := uint64(0)
	for _, p := range definitions {
		b, ok := p.EncodedBound()
		if !ok || b > sessionruntime.PrivateGroupingQueryMaxBytes || p.SourceByteLength() > limits.MaxDocumentBytes || b > uint64(8<<20)-total {
			return gap("INVALID_DEFINITION_PROOF")
		}
		total += b
	}
	total = 0
	for _, obj := range objects {
		if len(obj.Bytes) > limits.MaxDocumentBytes || obj.Identity.ByteLength > uint64(limits.MaxDocumentBytes) || len(obj.Identity.Digest) > 71 || uint64(len(obj.Bytes)) > uint64(limits.MaxTotalBytes)-total {
			return gap("SOURCE_BYTES_BOUND")
		}
		total += uint64(len(obj.Bytes))
	}
	// No fallback parsing, replay, composition, or preparation from an endpoint.
	if original.prepared == nil {
		return gap("SOURCE_CANDIDATE_NOT_PREPARED")
	}
	v := original.prepared.view
	var contribution rg.Contribution
	found := 0
	for _, c := range original.prepared.contributions {
		if c.OccurrenceIdentity == intent.Endpoint.Occurrence && c.Kind == intent.Endpoint.Kind && c.Ordinal == intent.Endpoint.Ordinal {
			contribution = c
			found++
		}
	}
	e := intent.Endpoint
	if found != 1 || e.Custody != "CALLER_ASSERTED" {
		return gap("EXACT_OCCURRENCE_MISMATCH")
	}
	if e.Disposition == "EXCLUDED" {
		return gap("GROUPING_ENDPOINT_EXCLUDED")
	}
	switch e.Disposition {
	case "INCLUDED", "DOWNWEIGHTED", "CAPPED", "NORMALIZED", "SEPARATE_LAYER":
	default:
		return gap("EVALUATION_DISPOSITION_MISMATCH")
	}
	fromRole, toRole := "CALLER_OCCURRENCE", "CALLEE_TARGET"
	if e.Kind == "REFERENCES_SYMBOL" {
		fromRole, toRole = "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
	}
	if e.Kind == "RESOLVES_TO_DEFINITION" {
		fromRole, toRole = "QUERY_OCCURRENCE", "DEFINITION_TARGET"
	}
	if !((e.Node == contribution.From && e.Role == fromRole) || (e.Node == contribution.To && e.Role == toRole)) {
		return gap("ENDPOINT_ROLE_MISMATCH")
	}
	if e.Predecessor != o.input.PredecessorIdentityForContribution(contribution) && e.Predecessor != contribution.MethodEvidenceSelector && e.Predecessor != contribution.AcquisitionReceiptSelector {
		return gap("ENDPOINT_PREDECESSOR_MISMATCH")
	}
	uri := v.Query.URI
	point := qt.Position{Line: v.Query.Line, Character: v.Query.Character}
	var exactSelection, exactDisplay *qt.Range
	if e.Kind == "CALLS" {
		n, ok := original.prepared.nodes[e.Node]
		if !ok {
			return gap("NATIVE_NODE_NAMESPACE_MISMATCH")
		}
		uri = n.URI
		r := n.SelectionRange
		point = r.Start
		exactSelection = &r
	} else {
		matched := false
		for _, pair := range v.Methods {
			method := "textDocument/references"
			if e.Kind == "RESOLVES_TO_DEFINITION" {
				method = "textDocument/definition"
			}
			if pair.Method != method {
				continue
			}
			parsed, failure := methodsyntax.ParseRaw(pair.Method, pair.Result, 256)
			if failure != nil {
				return gap("ORIGINAL_METHOD_INVALID")
			}
			for _, item := range parsed.Items {
				occurrence := privateSourceDigest([]byte(fmt.Sprintf("private-owned-grouping\x00%s\x00%d\x00%d\x00%s\x00%d", pair.SessionID, pair.Generation, pair.Key.ID, pair.Method, item.Ordinal)))
				if occurrence != e.Occurrence {
					continue
				}
				endpointRaw, _ := json.Marshal(struct {
					URI         string
					Range       methodsyntax.Range
					TargetRange *methodsyntax.Range
				}{item.URI, item.Range, item.TargetRange})
				endpoint := privateSourceDigest(append(endpointRaw, '\n'))
				rawFrom, rawTo := endpoint, v.Target.SymbolID
				if e.Kind == "RESOLVES_TO_DEFINITION" {
					rawFrom, rawTo = v.Target.SymbolID, endpoint
				}
				if contribution.From != rawFrom || contribution.To != rawTo || item.Ordinal != e.Ordinal || pair.SessionID != v.Query.SessionID || pair.Generation != v.Query.Generation || pair.Source == nil || pair.Source.URI != v.Query.URI || pair.Source.SHA256 != v.Query.SourceDigest {
					return gap("RAW_ENDPOINT_OR_LOCAL_ORDINAL_MISMATCH")
				}
				matched = true
				querySide := e.Kind == "REFERENCES_SYMBOL" && e.Role == "REFERENCED_SYMBOL" || e.Kind == "RESOLVES_TO_DEFINITION" && e.Role == "QUERY_OCCURRENCE"
				if !querySide {
					uri = item.URI
					r := sourceRange(item.Range)
					point = r.Start
					exactSelection = &r
					if item.TargetRange != nil {
						full := sourceRange(*item.TargetRange)
						exactDisplay = &full
					}
				}
			}
		}
		if !matched {
			return gap("ORIGINAL_METHOD_OCCURRENCE_UNAVAILABLE")
		}
	}
	proofs := append([]sessionruntime.PrivateGroupingSourceEvidence{o.evidence}, definitions...)
	for _, proof := range proofs {
		sv := v
		var er error
		if proof != o.evidence {
			sv, er = proof.Snapshot(sessionruntime.PrivateGroupingQueryMaxBytes)
		}
		if er != nil || len(sv.Source) > limits.MaxDocumentBytes || sv.Query.URI != uri || !sameScope(v, sv) || sv.Query.URI == v.Query.URI && (sv.Query.DocumentVersion != v.Query.DocumentVersion || sv.Query.SourceDigest != v.Query.SourceDigest) || !exactSourceView(sv) {
			continue
		}
		if e.Kind == "CALLS" {
			matched := false
			for _, raw := range original.prepared.sources[uri] {
				if bytes.Equal(raw, sv.Source) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		q := sv.Query
		q.Line, q.Character = point.Line, point.Character
		symbol, er := qt.SelectDocumentSymbolCandidateV1(q, sv.Symbol.Result)
		if er != nil || exactSelection != nil && symbol.SelectionRange != *exactSelection || exactDisplay != nil && symbol.DisplayRange != *exactDisplay {
			continue
		}
		id := sourceobject.Identity{Digest: sv.Query.SourceDigest, ByteLength: uint64(len(sv.Source))}
		key := rp.Key{GraphSubjectID: symbol.SymbolID, LogicalSourceID: uri}
		resolved, er := rp.Resolve(rp.Plan{Ordering: rp.Ordering, Target: key, Selections: []rp.Selection{{Role: "TARGET", Key: key, Source: id}}}, privateSourceObjects(objects), rp.ResolveLimits{MaxDistinctObjects: uint64(limits.MaxObjects), MaxUniqueSourceBytes: uint64(limits.MaxTotalBytes), MaxLogicalSelections: uint64(limits.MaxObjects)})
		if er != nil || len(resolved.Selections) != 1 || !bytes.Equal(resolved.Selections[0].Bytes, sv.Source) {
			continue
		}
		r := symbol.DisplayRange
		start, end, er := sourceposition.Offsets(sv.Source, sv.Query.Encoding, sourceposition.Range{Start: sourceposition.Position{Line: r.Start.Line, Character: r.Start.Character}, End: sourceposition.Position{Line: r.End.Line, Character: r.End.Character}})
		if er != nil || start >= end {
			continue
		}
		unit := PrivateSourceUnit{Version: PrivateSourceVersion, URI: uri, SourceDigest: id.Digest, Revision: sv.Revision, Privacy: sv.Privacy, Workspace: sv.Workspace, SessionID: sv.Query.SessionID, DocumentVersion: sv.Query.DocumentVersion, PositionEncoding: sv.Query.Encoding, SymbolID: symbol.SymbolID, Generation: sv.Query.Generation, DisplayRange: symbol.DisplayRange, SelectionRange: symbol.SelectionRange, Body: string(sv.Source[start:end]), ClaimCeiling: PrivateSourceCandidateCeiling}
		// Identity is finalized ONLY by inventorysuccessor after its completion gate.
		return PrivateSourceCandidate{&sourceCandidate{intent, unit, original}}, ""
	}
	return gap("OWN_FULL_DEFINITION_OR_VERIFIED_SOURCE_UNAVAILABLE")
}

type privateSourceObjects []sourceobject.Object

func (objects privateSourceObjects) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	var result sourceobject.Object
	found := 0
	for _, o := range objects {
		if o.Identity == id {
			result = o
			found++
		}
	}
	if found != 1 {
		return sourceobject.Object{}, ErrPrivateSource
	}
	return result, nil
}
