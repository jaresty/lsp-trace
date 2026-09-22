package censuscontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/liveprojection"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/internal/v5sourcesnapshotv4"
	"lsp-trace/internal/v5sourcesnapshotv5"
	"lsp-trace/internal/v5sourcesnapshotv6"
)

type CaptureStatus string

const (
	CaptureSucceeded         CaptureStatus = "CAPTURED"
	CaptureSourceUnavailable CaptureStatus = "SOURCE_UNAVAILABLE"
)

type ConstituentCapture struct {
	ConstituentIdentity string
	ConstituentOrdinal  int
	CustodyIdentity     string
	Status              CaptureStatus
	Unavailable         *v5sourcesnapshotv4.SourceUnavailableError
	Capture             v5sourcesnapshotv4.CaptureResult
	Limits              v5sourcesnapshotv4.Limits
	V5Capture           v5sourcesnapshotv5.CaptureResult
	V5Limits            v5sourcesnapshotv5.Limits
	V6Capture           v5sourcesnapshotv6.CaptureResult
	V6Limits            v5sourcesnapshotv6.Limits
	EndpointOutcomes    []EndpointCaptureOutcome
}

type CaptureDiagnosticError struct {
	Code               string
	ConstituentOrdinal int
	TargetCount        int
	CallerCount        int
	CapturedCount      int
	UnavailableCount   int
}

func (e *CaptureDiagnosticError) Error() string { return "continuation capture failed" }

type CaptureResult struct {
	HandoffID    string
	Constituents []ConstituentCapture
	Snapshots    []targetpacket.Snapshot
	objects      map[sourceobject.Identity][]byte
}

type Lookup struct {
	objects map[sourceobject.Identity][]byte
}

func (l Lookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	b, ok := l.objects[id]
	if !ok {
		return sourceobject.Object{}, errors.New("source object missing")
	}
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), b...)}, nil
}

type ManagedDocumentPreparer interface {
	PrepareManagedDocuments(context.Context, string, string, uint64, string, []string, ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error)
}

type ManagedPreparationLimits struct {
	MaxDocuments, MaxMessages, MaxWork int
	MaxDocumentBytes, MaxTotalBytes    int64
}

type ManagedPreparationErrorKind string

const (
	ManagedPreparationAvailability  ManagedPreparationErrorKind = "availability"
	ManagedPreparationResourceLimit ManagedPreparationErrorKind = "resource_limit"
	ManagedPreparationCustody       ManagedPreparationErrorKind = "custody"
	ManagedPreparationMalformed     ManagedPreparationErrorKind = "malformed_supply"
)

type ManagedDocumentPreparationFailure string

const (
	ManagedDocumentPreparationContextCancelled      ManagedDocumentPreparationFailure = "CONTEXT_CANCELLED"
	ManagedDocumentPreparationContextDeadline       ManagedDocumentPreparationFailure = "CONTEXT_DEADLINE"
	ManagedDocumentPreparationStaleGeneration       ManagedDocumentPreparationFailure = "STALE_GENERATION"
	ManagedDocumentPreparationLifecycleConflict     ManagedDocumentPreparationFailure = "LIFECYCLE_CONFLICT"
	ManagedDocumentPreparationResourceExhausted     ManagedDocumentPreparationFailure = "RESOURCE_EXHAUSTED"
	ManagedDocumentPreparationSessionPoisoned       ManagedDocumentPreparationFailure = "SESSION_POISONED"
	ManagedDocumentPreparationSupplyUnavailable     ManagedDocumentPreparationFailure = "DOCUMENT_SUPPLY_UNAVAILABLE"
	ManagedDocumentPreparationURIUnavailable        ManagedDocumentPreparationFailure = "DOCUMENT_URI_UNAVAILABLE"
	ManagedDocumentPreparationOutsideWorkspace      ManagedDocumentPreparationFailure = "DOCUMENT_OUTSIDE_WORKSPACE"
	ManagedDocumentPreparationSourceUnavailable     ManagedDocumentPreparationFailure = "DOCUMENT_SOURCE_UNAVAILABLE"
	ManagedDocumentPreparationLanguageIDUnavailable ManagedDocumentPreparationFailure = "LANGUAGE_ID_UNAVAILABLE"
	ManagedDocumentPreparationSupplyMissing         ManagedDocumentPreparationFailure = "SUPPLY_MISSING"
	ManagedDocumentPreparationUnknown               ManagedDocumentPreparationFailure = "UNKNOWN_DOCUMENT_PREPARATION_FAILURE"
)

type ManagedPreparationDiagnostic struct {
	Failure        ManagedDocumentPreparationFailure `json:"failure"`
	Attempted      int                               `json:"attempted"`
	Succeeded      int                               `json:"succeeded"`
	Planned        int                               `json:"planned"`
	FailingOrdinal int                               `json:"failing_ordinal"`
}

type ManagedPreparationDiagnosticRecorder interface {
	RecordManagedPreparationDiagnostic(ManagedPreparationDiagnostic) error
}
type ManagedPreparationDiagnosticRecorderFunc func(ManagedPreparationDiagnostic) error

func (f ManagedPreparationDiagnosticRecorderFunc) RecordManagedPreparationDiagnostic(d ManagedPreparationDiagnostic) error {
	return f(d)
}

type ManagedPreparationError struct {
	Kind       ManagedPreparationErrorKind
	Cause      error
	Diagnostic ManagedPreparationDiagnostic
}

func (e *ManagedPreparationError) Error() string {
	if e == nil || e.Cause == nil {
		return "managed source preparation failed"
	}
	return "managed source preparation failed: " + e.Cause.Error()
}

func (e *ManagedPreparationError) Unwrap() error { return e.Cause }

type managedPreparationResourceLimitError struct {
	managed  *ManagedPreparationError
	field    string
	observed int64
	limit    int64
}

func (e *managedPreparationResourceLimitError) Error() string { return e.managed.Error() }
func (e *managedPreparationResourceLimitError) Unwrap() error { return e.managed }
func (e *managedPreparationResourceLimitError) ResourceComponent() string {
	return "source_preparation"
}
func (e *managedPreparationResourceLimitError) ResourceField() string    { return e.field }
func (e *managedPreparationResourceLimitError) ResourceLimit() int64     { return e.limit }
func (e *managedPreparationResourceLimitError) ResourceObserved() int64  { return e.observed }
func (e *managedPreparationResourceLimitError) ResourceCategory() string { return "input" }

func managedPreparationError(kind ManagedPreparationErrorKind, message string) error {
	return &ManagedPreparationError{Kind: kind, Cause: errors.New(message)}
}

func managedPreparationDiagnostic(prepared liveprojection.PreparationResult) ManagedPreparationDiagnostic {
	d := ManagedPreparationDiagnostic{Attempted: prepared.Accounting.Attempted, Succeeded: prepared.Accounting.Succeeded, Planned: prepared.Accounting.Planned, FailingOrdinal: -1, Failure: ManagedDocumentPreparationUnknown}
	if prepared.Failure == nil {
		return d
	}
	d.FailingOrdinal = prepared.Failure.FailingOrdinal
	switch prepared.Failure.Dimension {
	case liveprojection.DocumentPreparationContextCancelled:
		d.Failure = ManagedDocumentPreparationContextCancelled
	case liveprojection.DocumentPreparationContextDeadline:
		d.Failure = ManagedDocumentPreparationContextDeadline
	case liveprojection.DocumentPreparationStaleGeneration:
		d.Failure = ManagedDocumentPreparationStaleGeneration
	case liveprojection.DocumentPreparationLifecycleConflict:
		d.Failure = ManagedDocumentPreparationLifecycleConflict
	case liveprojection.DocumentPreparationResourceExhausted:
		d.Failure = ManagedDocumentPreparationResourceExhausted
	case liveprojection.DocumentPreparationSessionPoisoned:
		d.Failure = ManagedDocumentPreparationSessionPoisoned
	case liveprojection.DocumentPreparationSupplyUnavailable:
		d.Failure = ManagedDocumentPreparationSupplyUnavailable
	case liveprojection.DocumentPreparationURIUnavailable:
		d.Failure = ManagedDocumentPreparationURIUnavailable
	case liveprojection.DocumentPreparationOutsideWorkspace:
		d.Failure = ManagedDocumentPreparationOutsideWorkspace
	case liveprojection.DocumentPreparationSourceUnavailable:
		d.Failure = ManagedDocumentPreparationSourceUnavailable
	case liveprojection.DocumentPreparationLanguageIDUnavailable:
		d.Failure = ManagedDocumentPreparationLanguageIDUnavailable
	case liveprojection.DocumentPreparationSupplyMissing:
		d.Failure = ManagedDocumentPreparationSupplyMissing
	}
	return d
}

type ManagedPreparer struct {
	Preparer  liveprojection.DocumentPreparer
	Recorder  ManagedPreparationDiagnosticRecorder
	Workspace interface {
		WorkspaceRoot(string, uint64) (string, bool)
	}
	LanguageResolver interface {
		SessionLanguageID(string, uint64) (string, bool)
	}
	LanguageID string
}

func (p ManagedPreparer) PrepareManagedDocuments(ctx context.Context, workspace, sessionID string, generation uint64, encoding string, uris []string, limits ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error) {
	if p.Preparer == nil || p.Workspace == nil {
		return nil, managedPreparationError(ManagedPreparationAvailability, "managed preparer unavailable")
	}
	routedWorkspace, ok := p.Workspace.WorkspaceRoot(sessionID, generation)
	if !ok || filepath.Clean(routedWorkspace) != filepath.Clean(workspace) {
		return nil, managedPreparationError(ManagedPreparationAvailability, "managed workspace routing unavailable")
	}
	languageID := p.LanguageID
	if languageID == "" && p.LanguageResolver != nil {
		languageID, ok = p.LanguageResolver.SessionLanguageID(sessionID, generation)
		if !ok || languageID == "" {
			diagnostic := ManagedPreparationDiagnostic{Failure: ManagedDocumentPreparationLanguageIDUnavailable, Planned: len(uris), FailingOrdinal: -1}
			if p.Recorder != nil {
				_ = p.Recorder.RecordManagedPreparationDiagnostic(diagnostic)
			}
			return nil, &ManagedPreparationError{Kind: ManagedPreparationAvailability, Cause: errors.New("managed document preparation unavailable"), Diagnostic: diagnostic}
		}
	}
	prepared := liveprojection.Prepare(ctx, p.Preparer, sessionID, generation, languageID, uris, liveprojection.PreparationLimits{MaxDocuments: limits.MaxDocuments, MaxBytes: int(limits.MaxTotalBytes), MaxDocumentBytes: int(limits.MaxDocumentBytes), MaxMessages: limits.MaxMessages, MaxWork: limits.MaxWork})
	if prepared.Status == liveprojection.PreparationResourceLimit {
		return nil, &managedPreparationResourceLimitError{managed: &ManagedPreparationError{Kind: ManagedPreparationResourceLimit, Cause: errors.New("managed document preparation resource limit")}, field: prepared.ResourceLimitField, observed: int64(prepared.ResourceLimitObserved), limit: int64(prepared.ResourceLimitValue)}
	}
	if prepared.Status != liveprojection.PreparationComplete {
		diagnostic := managedPreparationDiagnostic(prepared)
		if p.Recorder != nil {
			_ = p.Recorder.RecordManagedPreparationDiagnostic(diagnostic)
		}
		return nil, &ManagedPreparationError{Kind: ManagedPreparationAvailability, Cause: errors.New("managed document preparation unavailable"), Diagnostic: diagnostic}
	}
	out := make([]v5sourcesnapshotv6.PreparedDocument, 0, len(prepared.Supplies))
	for _, supply := range prepared.Supplies {
		if supply.SessionID != sessionID || supply.Generation != generation {
			return nil, managedPreparationError(ManagedPreparationCustody, "managed document custody mismatch")
		}
		out = append(out, v5sourcesnapshotv6.PreparedDocument{URI: supply.URI, Bytes: append([]byte(nil), supply.Content...), Digest: digestBytes(supply.Content), ByteLength: uint64(len(supply.Content)), Version: fmt.Sprint(supply.DocumentVersion), SessionID: sessionID, Generation: generation, PositionEncoding: encoding})
	}
	return out, nil
}

type FreshCaptureDependencies struct {
	Context  context.Context
	Preparer ManagedDocumentPreparer
	Resolver v5sourcesnapshotv6.FullDefinitionResolver
	Limits   ManagedPreparationLimits
}

func captureFresh(h CommittedHandoff, program censusprogramc.Result, workspace, encoding string, limits v5sourcesnapshotv3.Limits, deps *FreshCaptureDependencies) (CaptureResult, error) {
	if deps == nil {
		return CaptureResult{}, managedPreparationError(ManagedPreparationAvailability, "managed capture dependencies unavailable")
	}
	return CaptureSnapshots(h, program, workspace, encoding, limits, *deps)
}

func CaptureSnapshots(h CommittedHandoff, program censusprogramc.Result, workspace, positionEncoding string, limits v5sourcesnapshotv3.Limits, fresh ...FreshCaptureDependencies) (CaptureResult, error) {
	if err := h.Validate(); err != nil {
		return CaptureResult{}, err
	}
	projection, err := h.Projection()
	if err != nil {
		return CaptureResult{}, err
	}
	wu, err := url.Parse(h.WorkspaceIdentity().URI)
	if err != nil || wu.Path != workspace {
		return CaptureResult{}, errors.New("explicit workspace differs from committed custody")
	}
	if positionEncoding == "" || positionEncoding != h.PositionEncoding() {
		return CaptureResult{}, errors.New("position encoding differs from committed custody")
	}
	if program.CensusID != h.CensusID() || len(program.Admission.Artifact.Constituents) != len(projection.Constituents) {
		return CaptureResult{}, errors.New("Program C constituent set mismatch")
	}
	projectionByIdentity := make(map[string]int, len(projection.Constituents))
	for index, constituent := range projection.Constituents {
		if _, duplicate := projectionByIdentity[constituent.Identity.ImmutableSelector]; duplicate {
			return CaptureResult{}, errors.New("duplicate committed constituent identity")
		}
		projectionByIdentity[constituent.Identity.ImmutableSelector] = index
	}
	byOrdinal := map[int][]censusprogramc.Representative{}
	for _, r := range program.Representatives.Nominations {
		byOrdinal[r.ConstituentOrdinal] = append(byOrdinal[r.ConstituentOrdinal], r)
	}
	ordinals := make([]int, 0, len(byOrdinal))
	for ordinal := range byOrdinal {
		ordinals = append(ordinals, ordinal)
	}
	sort.Ints(ordinals)
	var managedDocuments map[string]v5sourcesnapshotv6.PreparedDocument
	if len(fresh) > 0 {
		deps := fresh[0]
		if len(fresh) != 1 || deps.Preparer == nil || deps.Resolver == nil || deps.Context == nil || deps.Limits.MaxDocuments <= 0 || deps.Limits.MaxMessages <= 0 || deps.Limits.MaxWork <= 0 || deps.Limits.MaxDocumentBytes <= 0 || deps.Limits.MaxTotalBytes <= 0 {
			return CaptureResult{}, errors.New("invalid fresh capture dependencies")
		}
		allURIs := make([]string, 0)
		seenURI := map[string]bool{}
		for _, ordinal := range ordinals {
			if ordinal < 0 || ordinal >= len(projection.Constituents) {
				return CaptureResult{}, errors.New("foreign representative constituent")
			}
			pc := program.Admission.Artifact.Constituents[ordinal]
			projectionIndex, ok := projectionByIdentity[pc.Identity]
			if !ok {
				return CaptureResult{}, &CaptureDiagnosticError{Code: "CONSTITUENT_IDENTITY", ConstituentOrdinal: ordinal}
			}
			c := projection.Constituents[projectionIndex]
			native, graphV5, decodeErr := decodeNative(c.Raw)
			if decodeErr != nil {
				return CaptureResult{}, decodeErr
			}
			if pc.Identity != c.Identity.ImmutableSelector || pc.SHA256 != digestBytes(c.Raw) || pc.ByteLength != len(c.Raw) || pc.GraphSHA256 != digestBytes(graphV5) || pc.GraphByteLength != len(graphV5) {
				return CaptureResult{}, &CaptureDiagnosticError{Code: "CONSTITUENT_COMMITMENT", ConstituentOrdinal: ordinal}
			}
			nodes := make(map[string]graph.Node, len(native.Nodes))
			for _, node := range native.Nodes {
				if _, duplicate := nodes[node.ID]; duplicate {
					return CaptureResult{}, errors.New("duplicate graph node")
				}
				nodes[node.ID] = node
			}
			for _, nomination := range byOrdinal[ordinal] {
				if node, ok := nodes[nomination.SelectedNode]; ok && !seenURI[node.URI] {
					allURIs = append(allURIs, node.URI)
					seenURI[node.URI] = true
				}
				for _, predecessor := range nomination.IncomingPredecessors {
					if node, ok := nodes[predecessor.CallerID]; ok && !seenURI[node.URI] {
						allURIs = append(allURIs, node.URI)
						seenURI[node.URI] = true
					}
				}
			}
		}
		sort.Strings(allURIs)
		eligibleURIs := make([]string, 0, len(allURIs))
		for _, uri := range allURIs {
			if eligibleFreshManagedDocumentURI(uri, workspace) {
				eligibleURIs = append(eligibleURIs, uri)
			}
		}
		if len(eligibleURIs) > deps.Limits.MaxDocuments {
			return CaptureResult{}, errors.New("managed document count limit")
		}
		prepared, prepareErr := deps.Preparer.PrepareManagedDocuments(deps.Context, workspace, projection.Session.SessionID, projection.Session.Generation, positionEncoding, eligibleURIs, deps.Limits)
		if prepareErr != nil {
			return CaptureResult{}, prepareErr
		}
		managedDocuments = make(map[string]v5sourcesnapshotv6.PreparedDocument, len(prepared))
		requested := make(map[string]bool, len(eligibleURIs))
		for _, uri := range eligibleURIs {
			requested[uri] = true
		}
		for _, document := range prepared {
			if !requested[document.URI] || document.URI == "" || document.Digest != digestBytes(document.Bytes) || document.ByteLength != uint64(len(document.Bytes)) || document.PositionEncoding != positionEncoding {
				return CaptureResult{}, managedPreparationError(ManagedPreparationMalformed, "malformed managed document supply")
			}
			if document.SessionID != projection.Session.SessionID || document.Generation != projection.Session.Generation {
				return CaptureResult{}, managedPreparationError(ManagedPreparationCustody, "managed document custody mismatch")
			}
			if _, duplicate := managedDocuments[document.URI]; duplicate {
				return CaptureResult{}, managedPreparationError(ManagedPreparationMalformed, "duplicate managed document supply")
			}
			managedDocuments[document.URI] = document
		}
		for _, uri := range eligibleURIs {
			if _, ok := managedDocuments[uri]; !ok {
				return CaptureResult{}, managedPreparationError(ManagedPreparationAvailability, "managed document unavailable")
			}
		}
	}

	out := CaptureResult{HandoffID: h.HandoffID(), objects: map[sourceobject.Identity][]byte{}}
	for _, ordinal := range ordinals {
		if ordinal < 0 || ordinal >= len(projection.Constituents) {
			return CaptureResult{}, errors.New("foreign representative constituent")
		}
		pc := program.Admission.Artifact.Constituents[ordinal]
		projectionIndex, ok := projectionByIdentity[pc.Identity]
		if !ok {
			return CaptureResult{}, &CaptureDiagnosticError{Code: "CONSTITUENT_IDENTITY", ConstituentOrdinal: ordinal}
		}
		c := projection.Constituents[projectionIndex]
		native, graphV5, err := decodeNative(c.Raw)
		if err != nil {
			return CaptureResult{}, err
		}
		if pc.Identity != c.Identity.ImmutableSelector || pc.SHA256 != digestBytes(c.Raw) || pc.ByteLength != len(c.Raw) || pc.GraphSHA256 != digestBytes(graphV5) || pc.GraphByteLength != len(graphV5) {
			code := "CONSTITUENT_COMMITMENT"
			switch {
			case pc.Identity != c.Identity.ImmutableSelector:
				code = "CONSTITUENT_IDENTITY"
			case pc.SHA256 != digestBytes(c.Raw) || pc.ByteLength != len(c.Raw):
				code = "CONSTITUENT_ENVELOPE"
			case pc.GraphSHA256 != digestBytes(graphV5) || pc.GraphByteLength != len(graphV5):
				code = "CONSTITUENT_GRAPH"
			}
			return CaptureResult{}, &CaptureDiagnosticError{Code: code, ConstituentOrdinal: ordinal}
		}

		if err != nil {
			return CaptureResult{}, err
		}
		nodes := map[string]graph.Node{}
		for _, n := range native.Nodes {
			if _, dup := nodes[n.ID]; dup {
				return CaptureResult{}, errors.New("duplicate graph node")
			}
			nodes[n.ID] = n
		}
		nominations := byOrdinal[ordinal]
		selected := make([]v5sourcesnapshotv5.Nomination, 0, len(nominations))
		for _, nomination := range nominations {
			incoming := make([]v5sourcesnapshotv5.Occurrence, 0, len(nomination.IncomingPredecessors))
			for _, predecessor := range nomination.IncomingPredecessors {
				incoming = append(incoming, v5sourcesnapshotv5.Occurrence{RelationID: predecessor.RelationID, OccurrenceID: predecessor.OccurrenceID, CallerNodeID: predecessor.CallerID, CalleeNodeID: predecessor.TargetID, Range: predecessor.CallSite})
			}
			selected = append(selected, v5sourcesnapshotv5.Nomination{ID: stableNominationID(nomination), TargetNodeID: nomination.SelectedNode, Incoming: incoming})
		}
		v5limits := v5sourcesnapshotv5.Limits{MaxArtifactBytes: limits.MaxArtifactBytes * 2, MaxGraphBytes: limits.MaxArtifactBytes, MaxReceipts: limits.MaxReceipts, MaxSourceBytes: limits.MaxSourceBytes, MaxTotalSourceBytes: limits.MaxTotalSourceBytes, MaxBindings: limits.MaxBindings, MaxOutcomes: limits.MaxBindings, MaxWork: limits.MaxWork}
		var captured v5sourcesnapshotv5.CaptureResult
		if len(fresh) == 0 {
			captured, err = v5sourcesnapshotv5.CaptureSelected(v5sourcesnapshotv5.CaptureInput{GraphV5Bytes: c.Raw, Workspace: workspace, PositionEncoding: positionEncoding, Nominations: selected, Limits: v5limits})
		}
		var v6captured v5sourcesnapshotv6.CaptureResult
		var v6limits v5sourcesnapshotv6.Limits
		if len(fresh) > 0 {
			deps := fresh[0]
			uris := make([]string, 0)
			seenURI := map[string]bool{}
			v6nominations := make([]v5sourcesnapshotv6.Nomination, 0, len(nominations))
			for _, nomination := range nominations {
				if n := nodes[nomination.SelectedNode]; !seenURI[n.URI] {
					uris = append(uris, n.URI)
					seenURI[n.URI] = true
				}
				incoming := make([]v5sourcesnapshotv6.Occurrence, 0, len(nomination.IncomingPredecessors))
				for _, predecessor := range nomination.IncomingPredecessors {
					if n := nodes[predecessor.CallerID]; !seenURI[n.URI] {
						uris = append(uris, n.URI)
						seenURI[n.URI] = true
					}
					incoming = append(incoming, v5sourcesnapshotv6.Occurrence{RelationID: predecessor.RelationID, OccurrenceID: predecessor.OccurrenceID, CallerNodeID: predecessor.CallerID, CalleeNodeID: predecessor.TargetID, Range: predecessor.CallSite})
				}
				v6nominations = append(v6nominations, v5sourcesnapshotv6.Nomination{ID: stableNominationID(nomination), TargetNodeID: nomination.SelectedNode, Incoming: incoming})
			}
			sort.Strings(uris)
			documents := make([]v5sourcesnapshotv6.PreparedDocument, 0, len(uris))
			for _, uri := range uris {
				if !eligibleFreshManagedDocumentURI(uri, workspace) {
					continue
				}
				if document, ok := managedDocuments[uri]; ok {
					documents = append(documents, document)
				}
			}
			v6limits = v5sourcesnapshotv6.Limits{MaxArtifactBytes: limits.MaxArtifactBytes * 2, MaxGraphBytes: limits.MaxArtifactBytes, MaxReceipts: limits.MaxReceipts, MaxSourceBytes: limits.MaxSourceBytes, MaxTotalSourceBytes: limits.MaxTotalSourceBytes, MaxBindings: limits.MaxBindings, MaxOutcomes: limits.MaxBindings, MaxWork: limits.MaxWork}
			v6captured, err = v5sourcesnapshotv6.CaptureSelected(deps.Context, v5sourcesnapshotv6.CaptureInput{GraphV5Bytes: c.Raw, PositionEncoding: positionEncoding, SessionID: projection.Session.SessionID, Generation: projection.Session.Generation, Nominations: v6nominations, Documents: documents, Resolver: deps.Resolver, Limits: v6limits})
		}
		if err != nil {
			return CaptureResult{}, err
		}
		if len(fresh) > 0 {
			captured = v5sourcesnapshotv5.CaptureResult{}
		}
		var raw []byte
		var replayObjects []sourceobject.Object
		if len(fresh) > 0 {
			var v6lookup v5sourcesnapshotv6.MemoryLookup
			raw, v6lookup, err = v5sourcesnapshotv6.Replay(v6captured.Raw, v6limits)
			for _, o := range v6captured.Objects {
				got, e := v6lookup.Get(o.Identity)
				if e != nil {
					return CaptureResult{}, e
				}
				replayObjects = append(replayObjects, got)
			}
		} else {
			var lookup v5sourcesnapshotv5.MemoryLookup
			raw, lookup, err = v5sourcesnapshotv5.Replay(captured.Raw, v5limits)
			for _, o := range captured.Objects {
				got, e := lookup.Get(o.Identity)
				if e != nil {
					return CaptureResult{}, e
				}
				replayObjects = append(replayObjects, got)
			}
		}
		if err != nil {
			return CaptureResult{}, err
		}
		selectedOutcomes := captured.Outcomes
		if len(fresh) > 0 {
			selectedOutcomes = make([]v5sourcesnapshotv5.Outcome, len(v6captured.Outcomes))
			for i, o := range v6captured.Outcomes {
				selectedOutcomes[i] = v5sourcesnapshotv5.Outcome{NominationID: o.NominationID, Role: o.Role, GraphSubjectID: o.GraphSubjectID, LogicalSourceDigest: o.LogicalSourceDigest, Status: o.Status, Code: continuationSourceCode(o.Code), CanonicalOrdinal: o.CanonicalOrdinal, Authority: o.Authority, Accepted: o.Accepted, Completeness: o.Completeness}
			}
		}
		derived := make([]EndpointIdentity, 0, len(selectedOutcomes))
		outcomes := make([]EndpointCaptureOutcome, 0, len(selectedOutcomes))
		for _, outcome := range selectedOutcomes {
			endpoint := EndpointIdentity{NominationID: outcome.NominationID, Role: outcome.Role, ConstituentIdentity: pc.Identity, ConstituentOrdinal: ordinal, GraphSubjectID: outcome.GraphSubjectID, NodeID: outcome.GraphSubjectID, LogicalSourceDigest: outcome.LogicalSourceDigest}
			derived = append(derived, endpoint)
			persisted, e := NewEndpointCaptureOutcome(endpoint, outcome.Status, outcome.Code)
			if e != nil {
				return CaptureResult{}, e
			}
			outcomes = append(outcomes, persisted)
		}
		outcomes, err = ValidateEndpointCaptureClosure(derived, outcomes)
		if err != nil {
			return CaptureResult{}, err
		}
		for _, got := range replayObjects {
			if prior, ok := out.objects[got.Identity]; ok && !bytes.Equal(prior, got.Bytes) {
				return CaptureResult{}, errors.New("coordinated source substitution")
			}
			out.objects[got.Identity] = append([]byte(nil), got.Bytes...)
		}
		out.Constituents = append(out.Constituents, ConstituentCapture{ConstituentIdentity: pc.Identity, ConstituentOrdinal: ordinal, CustodyIdentity: h.HandoffID(), Status: CaptureSucceeded, V5Capture: captured, V5Limits: v5limits, V6Capture: v6captured, V6Limits: v6limits, EndpointOutcomes: outcomes})
		out.Snapshots = append(out.Snapshots, targetpacket.Snapshot{ConstituentIdentity: pc.Identity, ConstituentOrdinal: ordinal, Raw: raw})
	}
	return cloneContinuationCapture(out), nil
}

func ReplaySnapshots(in CaptureResult) ([]targetpacket.Snapshot, Lookup, error) {
	objects := map[sourceobject.Identity][]byte{}
	snapshots := make([]targetpacket.Snapshot, 0, len(in.Constituents))
	last := -1
	for _, c := range in.Constituents {
		if c.ConstituentOrdinal <= last || c.CustodyIdentity != in.HandoffID {
			return nil, Lookup{}, errors.New("noncanonical continuation capture")
		}
		last = c.ConstituentOrdinal
		if c.Status == CaptureSourceUnavailable {
			continue
		}
		if c.Status != CaptureSucceeded {
			return nil, Lookup{}, errors.New("unknown continuation capture status")
		}
		isV6 := len(c.V6Capture.Raw) > 0
		if isV6 == (len(c.V5Capture.Raw) > 0) {
			return nil, Lookup{}, errors.New("exactly one persisted capture version required")
		}
		selectedOutcomes := c.V5Capture.Outcomes
		if isV6 {
			selectedOutcomes = make([]v5sourcesnapshotv5.Outcome, len(c.V6Capture.Outcomes))
			for i, o := range c.V6Capture.Outcomes {
				selectedOutcomes[i] = v5sourcesnapshotv5.Outcome{NominationID: o.NominationID, Role: o.Role, GraphSubjectID: o.GraphSubjectID, LogicalSourceDigest: o.LogicalSourceDigest, Status: o.Status, Code: continuationSourceCode(o.Code), CanonicalOrdinal: o.CanonicalOrdinal, Authority: o.Authority, Accepted: o.Accepted, Completeness: o.Completeness}
			}
		}
		if len(c.EndpointOutcomes) != len(selectedOutcomes) {
			return nil, Lookup{}, errors.New("invalid persisted endpoint capture")
		}
		derived := make([]EndpointIdentity, 0, len(selectedOutcomes))
		for _, outcome := range selectedOutcomes {
			derived = append(derived, EndpointIdentity{NominationID: outcome.NominationID, Role: outcome.Role, ConstituentIdentity: c.ConstituentIdentity, ConstituentOrdinal: c.ConstituentOrdinal, GraphSubjectID: outcome.GraphSubjectID, NodeID: outcome.GraphSubjectID, LogicalSourceDigest: outcome.LogicalSourceDigest})
		}
		closed, err := ValidateEndpointCaptureClosure(derived, c.EndpointOutcomes)
		if err != nil || !equalJSON(closed, c.EndpointOutcomes) {
			return nil, Lookup{}, errors.New("persisted endpoint outcome closure mismatch")
		}
		expectedOutcome := make(map[string]v5sourcesnapshotv5.Outcome, len(selectedOutcomes))
		for i, endpoint := range derived {
			expectedOutcome[endpointKey(endpoint)] = selectedOutcomes[i]
		}
		for _, outcome := range closed {
			expected := expectedOutcome[endpointKey(outcome.Endpoint)]
			if outcome.Status != expected.Status || outcome.Code != expected.Code {
				return nil, Lookup{}, errors.New("persisted endpoint outcome semantic mismatch")
			}
		}
		var raw []byte
		var replayObjects []sourceobject.Object
		if isV6 {
			var lookup v5sourcesnapshotv6.MemoryLookup
			raw, lookup, err = v5sourcesnapshotv6.Replay(c.V6Capture.Raw, c.V6Limits)
			var artifact v5sourcesnapshotv6.Artifact
			if err == nil {
				err = json.Unmarshal(c.V6Capture.Raw, &artifact)
			}
			for _, receipt := range artifact.Receipts {
				identity := sourceobject.Identity{Digest: receipt.ContentDigest, ByteLength: uint64(len(receipt.Content))}
				got, e := lookup.Get(identity)
				if e != nil {
					return nil, Lookup{}, e
				}
				replayObjects = append(replayObjects, got)
			}
		} else {
			var lookup v5sourcesnapshotv5.MemoryLookup
			raw, lookup, err = v5sourcesnapshotv5.Replay(c.V5Capture.Raw, c.V5Limits)
			for _, o := range c.V5Capture.Objects {
				got, e := lookup.Get(o.Identity)
				if e != nil {
					return nil, Lookup{}, e
				}
				replayObjects = append(replayObjects, got)
			}
		}
		if err != nil {
			return nil, Lookup{}, err
		}
		for _, o := range replayObjects {
			if prior, ok := objects[o.Identity]; ok && !bytes.Equal(prior, o.Bytes) {
				return nil, Lookup{}, errors.New("source object substitution")
			}
			objects[o.Identity] = append([]byte(nil), o.Bytes...)
		}
		snapshots = append(snapshots, targetpacket.Snapshot{ConstituentIdentity: c.ConstituentIdentity, ConstituentOrdinal: c.ConstituentOrdinal, Raw: raw})
	}
	return snapshots, Lookup{objects}, nil
}

func cloneContinuationCapture(in CaptureResult) CaptureResult {
	out := in
	out.Constituents = append([]ConstituentCapture(nil), in.Constituents...)
	out.Snapshots = make([]targetpacket.Snapshot, len(in.Snapshots))
	for i, s := range in.Snapshots {
		out.Snapshots[i] = targetpacket.Snapshot{ConstituentIdentity: s.ConstituentIdentity, ConstituentOrdinal: s.ConstituentOrdinal, Raw: append([]byte(nil), s.Raw...)}
	}
	out.objects = map[sourceobject.Identity][]byte{}
	for id, b := range in.objects {
		out.objects[id] = append([]byte(nil), b...)
	}
	return out
}
func decodeNative(raw []byte) (graph.Result, []byte, error) {
	var v5 graphprovenance.EvidenceV5
	var native graph.Result
	if _, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v5"); err != nil {
		return native, nil, err
	}
	if err := json.Unmarshal(raw, &v5); err != nil {
		return native, nil, err
	}
	b, err := base64.StdEncoding.DecodeString(v5.GraphV5)
	if err != nil {
		return native, nil, err
	}
	if err := json.Unmarshal(b, &native); err != nil {
		return native, nil, err
	}
	return native, b, nil
}
func eligibleFreshManagedDocumentURI(raw, workspace string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || !filepath.IsAbs(u.Path) {
		return true
	}
	canonical := (&url.URL{Scheme: "file", Path: filepath.Clean(u.Path)}).String()
	if raw != canonical {
		return true
	}
	rel, err := filepath.Rel(filepath.Clean(workspace), filepath.Clean(u.Path))
	if err != nil || rel == ".." || filepath.IsAbs(rel) || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func continuationSourceCode(code string) string {
	if code == v5sourcesnapshotv6.DisplayUnavailableCode {
		return SourceCodeExactEndpointUnavailable
	}
	return code
}

func stableNominationID(r censusprogramc.Representative) string {
	b, _ := json.Marshal(r)
	return digestBytes(append([]byte("lsp-trace:census-nomination:v1\x00"), b...))
}

func digestBytes(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }

var _ = fmt.Sprintf
