package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
)

var ErrPrivateManagedGrouping = errors.New("private managed grouping input not issued")

// PrivateManagedGroupingInput is structural input only, never policy qualification.
// Public fields are copies; Replay compares them with the private issued originals.
type PrivateManagedGroupingInput struct {
	Input                referencegrouping.AdmittedInput
	TypedInput           programcadmission.TypedInput
	Predecessors         []referencegrouping.PredecessorRecord
	Ledger               []PrivateManagedGroupingOccurrence
	Dispositions         map[string]string
	originals            []referencegrouping.PredecessorRecord
	inputOriginal        []byte
	publications         []privateGroupingPublication
	ledgerOriginal       []PrivateManagedGroupingOccurrence
	dispositionsOriginal map[string]string
}
type privateGroupingPublication struct {
	selector, digest string
	raw              []byte
}
type PrivateManagedGroupingOccurrence struct {
	Identity, Kind, From, To string
	Ordinal                  int
}
type PrivateManagedGroupingRequest struct {
	Session                                          sessionruntime.SessionLeaseRequest
	URI, OccurrenceID, Revision, Privacy             string
	OccurrenceAdmissionPolicyID, CompositionPolicyID string
	Line, Character                                  uint32
	Kinds                                            []programcadmission.RelationKind
	Root                                             *publication.Root
	Deadline                                         time.Time
	// Optional already-admitted CALLS; never relabeled from D/R or constructed here.
	Calls programcadmission.CompositeProjectionAdmission
}

// AcquirePrivateManagedGroupingInput is an additive private, operator-driven
// entry. It requires an existing READY session and never acquires/starts one.
// Shared job reservations and supervised evaluation deadlines are NOT wired here.
func AcquirePrivateManagedGroupingInput(ctx context.Context, m *sessionruntime.Manager, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	if !privateGroupingRequestFieldsValid(q) || ctx == nil || m == nil || q.Root == nil {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	ctx, cancel := context.WithDeadline(ctx, q.Deadline)
	defer cancel()
	refs, defs := false, false
	seen := map[programcadmission.RelationKind]bool{}
	for _, k := range q.Kinds {
		if seen[k] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		seen[k] = true
		switch k {
		case programcadmission.TypedReferencesSymbol:
			refs = true
		case programcadmission.TypedResolvesToDefinition:
			defs = true
		case programcadmission.TypedCalls:
		default:
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	if !refs && !defs {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	var out PrivateManagedGroupingInput
	err := m.WithReadySessionLease(ctx, q.Session, func(leased context.Context) error {
		h, err := m.CapturePrivateGroupingOwned(leased, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, Line: q.Line, Character: q.Character, References: refs, Definitions: defs, Deadline: q.Deadline})
		if err != nil {
			return err
		}
		out, err = IssuePrivateManagedGroupingInput(leased, m, h, q)
		return err
	})
	if err != nil {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateManagedGrouping, err)
	}
	return out, nil
}

// IssuePrivateManagedGroupingInput consumes opaque current runtime custody, not
// an OwnedMethodPair snapshot or a caller-supplied query receipt. It must run
// inside the same active session lease that created h. Once the consume callback
// is admitted, failure retires the handle too.
func IssuePrivateManagedGroupingInput(ctx context.Context, m *sessionruntime.Manager, h sessionruntime.PrivateGroupingOwnedHandoff, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	if !privateGroupingRequestFieldsValid(q) || ctx == nil || m == nil || q.Root == nil || q.Deadline.IsZero() || !time.Now().Before(q.Deadline) {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	ctx, cancel := context.WithDeadline(ctx, q.Deadline)
	defer cancel()
	var out PrivateManagedGroupingInput
	err := m.ConsumePrivateGroupingOwned(ctx, h, func(v sessionruntime.PrivateGroupingOwnedView) error {
		if !q.Deadline.Equal(v.Deadline) {
			return ErrPrivateManagedGrouping
		}
		if v.Workspace != q.Session.Workspace || v.Query.SessionID != q.Session.SessionID || v.Query.Generation != q.Session.Generation || v.Query.URI != q.URI || v.Query.OccurrenceID != q.OccurrenceID || v.Query.Line != q.Line || v.Query.Character != q.Character || v.Revision != q.Revision || v.Privacy != q.Privacy {
			return ErrPrivateManagedGrouping
		}
		// Re-select from the exact owned result before any publication or admission.
		target, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(v.Query, v.Symbol.Result)
		if err != nil || target != v.Target || v.Symbol.Source == nil || v.Query.SourceDigest != v.Symbol.Source.SHA256 || v.Query.SourceDigest != privateGroupingDigest(v.Source) {
			return ErrPrivateManagedGrouping
		}
		queryOriginal, err := privateGroupingQueryBytes(v)
		if err != nil {
			return err
		}
		out, err = privateGroupingBuild(v, q, queryOriginal)
		if err != nil {
			return err
		}
		if len(out.Ledger) == 0 {
			return nil
		} // no false grouping claim for empty/unavailable
		// Final exact-byte publication/readback precedes accepted return. This reuses
		// the existing bound-file mechanism; it is not a new authority framework.
		for _, p := range out.Predecessors {
			if err := ctx.Err(); err != nil {
				return err
			}
			selector, digest, err := chainPublish(q.Root, "private-grouping", p.Bytes)
			if err != nil {
				return err
			}
			out.publications = append(out.publications, privateGroupingPublication{selector, digest, append([]byte(nil), p.Bytes...)})
		}
		raw := out.Input.CanonicalBytes()
		selector, digest, err := chainPublish(q.Root, "private-grouping-input", raw)
		if err != nil {
			return err
		}
		out.publications = append(out.publications, privateGroupingPublication{selector, digest, append([]byte(nil), raw...)})
		out.originals = privateGroupingClonePredecessors(out.Predecessors)
		out.ledgerOriginal = append([]PrivateManagedGroupingOccurrence(nil), out.Ledger...)
		out.dispositionsOriginal = make(map[string]string, len(out.Dispositions))
		for method, disposition := range out.Dispositions {
			out.dispositionsOriginal[method] = disposition
		}
		out.inputOriginal = append([]byte(nil), raw...)
		return out.Replay(q.Root)
	})
	if err != nil {
		return PrivateManagedGroupingInput{}, errors.Join(ErrPrivateManagedGrouping, err)
	}
	return out, nil
}

// Replay checks exact issued bytes and their original bound-file readbacks; the
// ordinary grouping parser alone does not authenticate native query execution.
func (in PrivateManagedGroupingInput) Replay(root *publication.Root) error {
	if root == nil || len(in.inputOriginal) == 0 || len(in.originals) != len(in.Predecessors) || !bytes.Equal(in.Input.CanonicalBytes(), in.inputOriginal) {
		return ErrPrivateManagedGrouping
	}
	if len(in.Ledger) != len(in.ledgerOriginal) || len(in.Dispositions) != len(in.dispositionsOriginal) {
		return ErrPrivateManagedGrouping
	}
	for i, item := range in.ledgerOriginal {
		if in.Ledger[i] != item {
			return ErrPrivateManagedGrouping
		}
	}
	for method, disposition := range in.dispositionsOriginal {
		if in.Dispositions[method] != disposition {
			return ErrPrivateManagedGrouping
		}
	}
	for i, p := range in.originals {
		candidate := in.Predecessors[i]
		if p.Role != candidate.Role || p.Selector != candidate.Selector || p.Schema != candidate.Schema || !bytes.Equal(p.Bytes, candidate.Bytes) {
			return ErrPrivateManagedGrouping
		}
	}
	if len(in.publications) != len(in.originals)+1 {
		return ErrPrivateManagedGrouping
	}
	for _, p := range in.publications {
		raw, err := publication.ReadVerifiedBoundFile(root, p.selector, chainMaxBytes)
		if err != nil || privateGroupingDigest(raw) != p.digest || !bytes.Equal(raw, p.raw) {
			return ErrPrivateManagedGrouping
		}
	}
	admitted, err := referencegrouping.ParseAndAdmit(in.inputOriginal, in.originals, privateGroupingParseLimits())
	if err != nil || !admitted.MatchesTypedInput(in.TypedInput) {
		return ErrPrivateManagedGrouping
	}
	return nil
}

// Native byte slices are encoded as base64 by encoding/json. RawMessage would
// silently normalize lexical originals, so it is deliberately not used here.
type privateGroupingNative struct {
	Method, SessionID     string
	Generation, RequestID uint64
	Params, Result        []byte
	Write                 sessionruntime.RequestWriteObservation
	Read                  sessionruntime.ResponseReadObservation
	Source                *sessionruntime.OwnedDocumentBinding
}

func privateGroupingNativePair(p sessionruntime.OwnedMethodPair) privateGroupingNative {
	return privateGroupingNative{p.Method, p.SessionID, p.Generation, p.Key.ID, []byte(p.Params), []byte(p.Result), p.Write, p.Read, p.Source}
}

type privateGroupingSupply struct {
	Classification, SessionID string
	Generation                uint64
	URI                       string
	DocumentVersion           int
	Method                    string
	Content, Params           []byte
}

func privateGroupingQueryBytes(v sessionruntime.PrivateGroupingOwnedView) ([]byte, error) {
	bound, ok := sessionruntime.PrivateGroupingQueryEncodedBound(v)
	if !ok {
		return nil, ErrPrivateManagedGrouping
	}
	methods := make([]privateGroupingNative, len(v.Methods))
	for i, p := range v.Methods {
		methods[i] = privateGroupingNativePair(p)
	}
	return privateGroupingJSON(struct {
		Query                      adr0011querytarget.Query
		Target                     adr0011querytarget.Candidate
		Source                     []byte
		Supply                     privateGroupingSupply
		Metadata                   sessionruntime.SessionMetadata
		Revision, Custody, Privacy string
		Workspace                  string
		Deadline                   time.Time
		Symbol                     privateGroupingNative
		Methods                    []privateGroupingNative
		Unavailable                []string
	}{v.Query, v.Target, v.Source, privateGroupingSupply{v.Supply.Classification, v.Supply.SessionID, v.Supply.Generation, v.Supply.URI, v.Supply.DocumentVersion, v.Supply.Method, v.Supply.Content, []byte(v.Supply.Params)}, v.Metadata, v.Revision, "CALLER_ASSERTED", v.Privacy, v.Workspace, v.Deadline, privateGroupingNativePair(v.Symbol), methods, v.Unavailable}, bound)
}
func privateGroupingBuild(v sessionruntime.PrivateGroupingOwnedView, q PrivateManagedGroupingRequest, query []byte) (PrivateManagedGroupingInput, error) {
	out := PrivateManagedGroupingInput{Dispositions: map[string]string{}, Ledger: []PrivateManagedGroupingOccurrence{}}
	// This reports the current provider capability, not the provenance of an
	// optional separately admitted CALLS composite supplied by the operator.
	if !v.Metadata.CallHierarchySupport {
		out.Dispositions["CALLS"] = "UNAVAILABLE"
	}
	for _, m := range v.Unavailable {
		out.Dispositions[m] = "UNAVAILABLE"
	}
	envelope := programcadmission.UntrustedMethodEvidenceEnvelope{Family: programcadmission.MethodEvidenceFamily, Version: programcadmission.MethodEvidenceFamily, ClaimCeiling: programcadmission.MethodEvidenceClaimCeiling, Completeness: "UNKNOWN", Provider: v.Metadata.ProviderName, ProviderVersion: v.Metadata.ProviderVersion, SessionID: v.Query.SessionID, Generation: v.Query.Generation, Revision: v.Revision, Custody: "CALLER_ASSERTED", PositionEncoding: v.Query.Encoding}
	// Missing provider metadata is unavailable provenance, not invented identity.
	if envelope.Provider == "" || envelope.ProviderVersion == "" {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	selected := map[programcadmission.RelationKind]bool{}
	for _, kind := range q.Kinds {
		if kind != programcadmission.TypedCalls && kind != programcadmission.TypedReferencesSymbol && kind != programcadmission.TypedResolvesToDefinition {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		if selected[kind] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		selected[kind] = true
	}
	observed := make(map[programcadmission.RelationKind]bool)
	for _, method := range v.Unavailable {
		if method == "textDocument/references" {
			observed[programcadmission.TypedReferencesSymbol] = true
		} else if method == "textDocument/definition" {
			observed[programcadmission.TypedResolvesToDefinition] = true
		} else {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	for _, pair := range v.Methods {
		if pair.Method == "textDocument/references" {
			observed[programcadmission.TypedReferencesSymbol] = true
		} else if pair.Method == "textDocument/definition" {
			observed[programcadmission.TypedResolvesToDefinition] = true
		} else {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	for _, kind := range []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition} {
		if selected[kind] != observed[kind] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	for _, pair := range v.Methods {
		kind, sr, tr := programcadmission.TypedReferencesSymbol, "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
		if pair.Method == "textDocument/definition" {
			kind, sr, tr = programcadmission.TypedResolvesToDefinition, "QUERY_SYMBOL", "DEFINITION_TARGET"
		}
		if !selected[kind] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		parsed, failure := parseRawUntrusted(pair.Method, pair.Result, 256)
		if failure != nil {
			return PrivateManagedGroupingInput{}, failure
		}
		if len(parsed.Items) == 0 {
			out.Dispositions[pair.Method] = "OBSERVED_EMPTY"
		} else {
			out.Dispositions[pair.Method] = "OBSERVED_NONEMPTY"
		}
		for _, item := range parsed.Items {
			endpointBound, ok := privateGroupingStringsBound(1024, item.URI)
			if !ok {
				return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
			}
			endpointRaw, err := privateGroupingJSON(struct {
				URI         string
				Range       Range
				TargetRange *Range
			}{item.URI, item.Range, item.TargetRange}, endpointBound)
			if err != nil {
				return PrivateManagedGroupingInput{}, err
			}
			endpoint := privateGroupingDigest(endpointRaw)
			from, to := endpoint, v.Target.SymbolID
			if kind == programcadmission.TypedResolvesToDefinition {
				from, to = v.Target.SymbolID, endpoint
			}
			identity := privateGroupingDigest([]byte(fmt.Sprintf("private-owned-grouping\x00%s\x00%d\x00%d\x00%s\x00%d", pair.SessionID, pair.Generation, pair.Key.ID, pair.Method, item.Ordinal)))
			o := programcadmission.MethodOccurrenceEvidence{Identity: identity, From: from, To: to, Kind: kind, Ordinal: item.Ordinal, SourceRole: sr, TargetRole: tr, Method: pair.Method, QuerySelector: "methods/query.json", QueryDigest: privateGroupingDigest(query), QueryLength: len(query), EvidenceSelector: "methods/evidence.json", EvidenceDigest: privateGroupingDigest(pair.Result), EvidenceLength: len(pair.Result), Provider: envelope.Provider, ProviderVersion: envelope.ProviderVersion, SessionID: envelope.SessionID, Generation: envelope.Generation, Revision: envelope.Revision, Custody: envelope.Custody, PositionEncoding: envelope.PositionEncoding}
			envelope.Occurrences = append(envelope.Occurrences, o)
			out.Ledger = append(out.Ledger, PrivateManagedGroupingOccurrence{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal})
		}
	}
	if len(out.Ledger) == 0 {
		// The unchanged typed boundary rejects selected zero-occurrence method
		// families. Do not silently discard a separately supplied CALLS input.
		if selected[programcadmission.TypedCalls] {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		return out, nil
	}
	methods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(envelope)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	typed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: q.Kinds, Calls: q.Calls})
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	out.TypedInput = typed
	return privateGroupingAssemble(out, query, methods, q)
}
func privateGroupingAssemble(out PrivateManagedGroupingInput, query []byte, methods programcadmission.MethodProjectionAdmission, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	type mapping struct {
		Identity         string `json:"identity"`
		Kind             string `json:"kind"`
		From             string `json:"from"`
		To               string `json:"to"`
		Ordinal          int    `json:"ordinal"`
		QuerySelector    string `json:"query_selector"`
		EvidenceSelector string `json:"evidence_selector"`
	}
	methodRows := methods.Occurrences() // existing admitted getter; <= 2*256 parsed items
	if len(methodRows) > 512 || len(out.TypedInput.Occurrences) > 1024 {
		return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
	}
	mappingBound := uint64(1024)
	for _, o := range methodRows {
		var ok bool
		mappingBound, ok = privateGroupingStringsBound(mappingBound, o.Identity, string(o.Kind), o.From, o.To, o.QuerySelector, o.EvidenceSelector)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		mappingBound, ok = sessionruntime.PrivateGroupingCheckedEncodedSize(mappingBound, 0, 0, 256)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	mappings := make([]mapping, 0, len(methodRows))
	for _, o := range methodRows {
		mappings = append(mappings, mapping{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.QuerySelector, o.EvidenceSelector})
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Identity < mappings[j].Identity })
	mapped, err := privateGroupingJSON(struct {
		Family   string    `json:"family"`
		Version  string    `json:"version"`
		Mappings []mapping `json:"mappings"`
	}{"METHOD_MAPPING_RECEIPT", "v1", mappings}, mappingBound)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	type row struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	ledgerBound := uint64(1024)
	for _, o := range out.TypedInput.Occurrences {
		var ok bool
		ledgerBound, ok = privateGroupingStringsBound(ledgerBound, o.Identity, string(o.Kind), o.From, o.To, o.RelationID)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		ledgerBound, ok = sessionruntime.PrivateGroupingCheckedEncodedSize(ledgerBound, 0, 0, 256)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
	}
	rows := make([]row, len(out.TypedInput.Occurrences))
	custody := []referencegrouping.EndpointCustody{}
	seen := map[string]bool{}
	outcomes := []referencegrouping.CandidateOutcome{}
	for i, o := range out.TypedInput.Occurrences {
		rows[i] = row{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.RelationID}
		outcomes = append(outcomes, referencegrouping.CandidateOutcome{OccurrenceIdentity: o.Identity, Disposition: "ADMITTED"})
		selector := "methods/evidence.json"
		if o.Kind == programcadmission.TypedCalls {
			selector = "calls/acquisition.json"
		}
		for _, endpoint := range []string{o.From, o.To} {
			key := selector + "\x00" + endpoint
			if !seen[key] {
				seen[key] = true
				custody = append(custody, referencegrouping.EndpointCustody{Endpoint: endpoint, PredecessorSelector: selector, Custody: "CALLER_ASSERTED"})
			}
		}
	}
	ledger, err := privateGroupingJSON(struct {
		Domain      string `json:"domain"`
		Version     string `json:"version"`
		Claim       string `json:"claim"`
		Occurrences []row  `json:"occurrences"`
	}{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows}, ledgerBound)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	out.Predecessors = []referencegrouping.PredecessorRecord{{Role: "METHOD_QUERY", Selector: "methods/query.json", Schema: "lsp-trace.private.program-c-method-query.v0", Bytes: query}, {Role: "METHOD_EVIDENCE", Selector: "methods/evidence.json", Schema: programcadmission.MethodEvidenceFamily, Bytes: methods.CanonicalBytes()}, {Role: "METHOD_MAPPING_RECEIPT", Selector: "methods/mapping-receipt.json", Schema: "lsp-trace.private.program-c-method-mapping-receipt.v1", Bytes: mapped}, {Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger}}
	projection, err := programcadmission.PreparePrivateLeidenProjection(out.TypedInput)
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	if retained := projection.RetainedCallsCanonicalBytes(); len(retained) > 0 {
		// SourceBinding is the admission package's closed mirror/subset of the
		// retained composite (no caller-added fields). Six times its original
		// JSON bytes bounds re-escaping; 16 KiB covers added fixed identity keys
		// and scalar envelope fields. Check before cloning/marshalling metadata.
		bindingBound, ok := sessionruntime.PrivateGroupingCheckedEncodedSize(16<<10, uint64(len(retained)), 6, 0)
		if !ok {
			return PrivateManagedGroupingInput{}, ErrPrivateManagedGrouping
		}
		binding, err := privateGroupingJSON(projection.CompositeSourceBinding(), bindingBound)
		if err != nil {
			return PrivateManagedGroupingInput{}, err
		}
		out.Predecessors = append(out.Predecessors, referencegrouping.PredecessorRecord{Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: retained}, referencegrouping.PredecessorRecord{Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: binding})
	}
	n := len(rows)
	artifact, err := referencegrouping.BuildInput(referencegrouping.InputRequest{TypedInput: out.TypedInput, Predecessors: out.Predecessors, OccurrenceLedgerBytes: ledger, EndpointCustody: custody, OccurrenceAdmissionPolicyID: q.OccurrenceAdmissionPolicyID, CompositionPolicyID: q.CompositionPolicyID, PrivacyPolicy: q.Privacy, EligibleKinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Limits: referencegrouping.ResourceLimits{MaxArtifacts: 8, MaxNodes: 1024, MaxOccurrences: 1024, MaxContributions: 1024, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}, CandidateOutcomes: outcomes, Accounting: referencegrouping.Accounting{Candidates: n, Included: n, Terminal: true}})
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	out.Input, err = referencegrouping.ParseAndAdmit(artifact.CanonicalBytes(), out.Predecessors, privateGroupingParseLimits())
	if err != nil {
		return PrivateManagedGroupingInput{}, err
	}
	return out, nil
}
func privateGroupingParseLimits() referencegrouping.ParseLimits {
	return referencegrouping.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 8, MaxNodes: 1024, MaxOccurrences: 1024, MaxContributions: 1024, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}
}
func privateGroupingDigest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

// Every call site supplies its typed, checked preflight bound; this function
// only marshals an already-admitted value. The post-check is a backstop, not the
// admission mechanism. It is deliberately not a replaceable serializer hook.
func privateGroupingJSON(v any, admittedBound uint64) ([]byte, error) {
	if admittedBound == 0 || admittedBound > uint64(chainMaxBytes) {
		return nil, ErrPrivateManagedGrouping
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(raw) >= chainMaxBytes || uint64(len(raw)) >= admittedBound {
		return nil, ErrPrivateManagedGrouping
	}
	return append(raw, '\n'), nil
}
func privateGroupingStringsBound(total uint64, fields ...string) (uint64, bool) {
	for _, field := range fields {
		if field != "" && !sessionruntime.PrivateGroupingFieldValid(field) {
			return 0, false
		}
		var ok bool
		total, ok = sessionruntime.PrivateGroupingCheckedEncodedSize(total, uint64(len(field)), 6, 2)
		if !ok {
			return 0, false
		}
	}
	return total, true
}
func privateGroupingRequestFieldsValid(q PrivateManagedGroupingRequest) bool {
	if len(q.Kinds) == 0 || len(q.Kinds) > sessionruntime.PrivateGroupingMaxKinds || q.Session.Generation == 0 {
		return false
	}
	for _, field := range []string{q.Session.SessionID, q.Session.Workspace, q.URI, q.OccurrenceID, q.Revision, q.Privacy, q.OccurrenceAdmissionPolicyID, q.CompositionPolicyID} {
		if !sessionruntime.PrivateGroupingFieldValid(field) {
			return false
		}
	}
	for i, kind := range q.Kinds {
		if kind != programcadmission.TypedCalls && kind != programcadmission.TypedReferencesSymbol && kind != programcadmission.TypedResolvesToDefinition {
			return false
		}
		for j := 0; j < i; j++ {
			if q.Kinds[j] == kind {
				return false
			}
		}
	}
	// The existing opaque admission getter owns its own finite predecessor cap;
	// reject before deriving typed/output collections from a large CALLS set.
	if q.Calls.Valid() && len(q.Calls.Occurrences()) > 512 {
		return false
	}
	return true
}

func privateGroupingClonePredecessors(in []referencegrouping.PredecessorRecord) []referencegrouping.PredecessorRecord {
	out := append([]referencegrouping.PredecessorRecord(nil), in...)
	for i := range out {
		out[i].Bytes = append([]byte(nil), out[i].Bytes...)
	}
	return out
}
