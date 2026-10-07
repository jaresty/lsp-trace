// Package adr0011acquisition contains a default-off, host-private, test-owned
// method acquisition. It does not qualify production issuance.
package adr0011acquisition

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lsp-trace/internal/adr0011c18"
	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

var ErrDisabled = errors.New("ADR0011 production occurrence owner disabled")
var ErrAcquisition = errors.New("ADR0011 managed acquisition not verified")

type Owner struct {
	manager                            *sessionruntime.Manager
	root                               *publication.Root
	selectedGitPath                    string                 // private test override; empty selects the host git
	preinvokeSelector, preinvokeDigest string                 // test-only expected receipt; never caller supplied
	finalImplementationPin             finalImplementationPin // independently selected by owner; nil in production
	finalTraceTestHook                 publication.BoundFileTrace
	beforeMethodRequestTestHook        func()
	beforeRevisionTestHook             func(*selectedGitObservation, *hostGitExpectation)
	afterRevisionTestHook              func(*selectedGitObservation, *hostGitExpectation)
	afterRevisionVerifiedTestHook      func()
	testEnabled                        bool
	captureFrames                      bool // test-owned, per-run opt-in; never enabled by NewDisabled
	afterCaptureTestHook               func()
	afterKeyGuardTestHook              func()
	afterOwnerReadTestHook             func()
	afterRawPayloadTestHook            func()
	afterTargetResultTestHook          func()
	afterTargetRecordTestHook          func()
	afterResponseReadTestHook          func()
	afterRawRecordTestHook             func()
	afterScannerRecordTestHook         func()
	afterEventsRecordTestHook          func()
	afterPoliciesTestHook              func()
	afterMethodRecordTestHook          func()
}

// ownerInvocation is selected before the associated managed request. It is
// private test-owned context, not an accepted issuance or wire identity.
func ownerInvocation() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", ErrAcquisition
	}
	return hex.EncodeToString(random[:]), nil
}

// NewDisabled is the only production host construction. No external request
// chooses a publication root or enables the owner.
func NewDisabled(manager *sessionruntime.Manager) *Owner { return &Owner{manager: manager} }

// newTestOwner is package-private. Production host construction is disabled.
func newTestOwner(manager *sessionruntime.Manager, root *publication.Root) *Owner {
	return &Owner{manager: manager, root: root, testEnabled: true}
}

type Query struct {
	SessionID       string
	Generation      uint64
	URI             string
	Line, Character uint32
	OccurrenceID    string
	Revision        string
}

// PrivateFinalReceipt is a test-only receipt, never a public issuance contract.
type PrivateFinalReceipt struct {
	FinalRef    targetResultRef
	T, A        int
	OrdinalRefs []string
}

type managedResult struct {
	legacy *adr0011methodresult.ChainReceipt
	final  *PrivateFinalReceipt
}

// Acquire preserves the legacy fail-closed contract and never returns a private final.
func (o *Owner) Acquire(ctx context.Context, q Query) (*adr0011methodresult.ChainReceipt, error) {
	result, err := o.acquireManaged(ctx, q, false)
	if err != nil {
		return nil, err
	}
	return result.legacy, nil
}

// acquirePrivateFinal is default-off and has no transport or public caller.
func (o *Owner) acquirePrivateFinal(ctx context.Context, q Query) (*PrivateFinalReceipt, error) {
	if o == nil || o.finalImplementationPin == nil {
		return nil, ErrAcquisition
	}
	result, err := o.acquireManaged(ctx, q, true)
	if err != nil {
		return nil, err
	}
	return result.final, nil
}

// acquireManaged owns exactly one documentSymbol/references managed pair.
func (o *Owner) acquireManaged(ctx context.Context, q Query, finish bool) (managed *managedResult, err error) {
	adr0011c18.Notify(ctx, adr0011c18.PointAcquisitionEntered)
	runtimeParent := ctx
	transaction := q.OccurrenceID
	if transaction == "" {
		transaction = "disabled"
	}
	ctx, custody := adr0011c18.BeginAcquisition(ctx, transaction)
	if custody == nil {
		adr0011c18.Notify(ctx, adr0011c18.PointAcquisitionReturned)
		return nil, ErrAcquisition
	}
	defer func() {
		panicValue := recover()
		outcome := adr0011c18.OutcomeError
		if panicValue != nil {
			outcome = adr0011c18.OutcomePanic
		} else if err == nil && managed != nil {
			outcome = adr0011c18.OutcomeSuccess
		}
		sealed := custody.SealTerminal(transaction, outcome)
		adr0011c18.Notify(ctx, adr0011c18.PointAcquisitionReturned)
		if panicValue != nil {
			panic(panicValue)
		}
		if !sealed {
			managed = nil
			err = errors.Join(err, ErrAcquisition)
		}
	}()
	if o == nil || !o.testEnabled || o.manager == nil || o.root == nil {
		return nil, ErrDisabled
	}
	if q.SessionID == "" || q.Generation == 0 || q.URI == "" || q.OccurrenceID == "" || q.Revision == "" {
		return nil, ErrAcquisition
	}
	records := o.manager.Records()
	workspace := ""
	for _, r := range records {
		if r.SessionID == q.SessionID && r.Generation == q.Generation && r.Routing.LanguageID == "go" && r.Routing.ServerProfile == "gopls" && r.Routing.Readiness == "READY" && r.Profile.Workspace().String() == r.Routing.WorkspaceRoot {
			workspace = r.Routing.WorkspaceRoot
		}
	}
	if workspace == "" || strings.TrimSpace(q.Revision) != q.Revision {
		return nil, ErrAcquisition
	}
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace || !canonicalPreparedURI(q.URI, workspace) {
		return nil, ErrAcquisition
	}
	selected, err := selectGitExecutable(o.selectedGitPath)
	if err != nil {
		return nil, ErrAcquisition
	}
	resolved, err := filepath.EvalSymlinks(workspace)
	if err != nil || resolved != workspace {
		return nil, ErrAcquisition
	}
	beforeTarget, err := observeHostGit(ctx, workspace, q.Revision)
	if err != nil {
		return nil, ErrAcquisition
	}
	metadata, failure := o.manager.Metadata(q.SessionID, q.Generation)
	if failure != "" || !metadata.DocumentSymbolSupport || !metadata.ReferencesSupport || metadata.PositionEncoding != "utf-16" {
		return nil, ErrAcquisition
	}
	prepared := o.manager.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: q.SessionID, Generation: q.Generation, URI: q.URI, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
	if prepared.Failure != "" || prepared.Supply == nil || prepared.Supply.URI != q.URI || prepared.Supply.SessionID != q.SessionID || prepared.Supply.Generation != q.Generation || prepared.Supply.DocumentVersion != prepared.Version || prepared.URI != q.URI || prepared.LanguageID != "go" || prepared.Version < 0 || len(prepared.Supply.Content) == 0 {
		return nil, ErrAcquisition
	}
	source := append([]byte(nil), prepared.Supply.Content...)
	binding := sessionruntime.OwnedDocumentBinding{URI: q.URI, Version: prepared.Version, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(source))}
	preinvoke := preinvokeFacts{SessionID: q.SessionID, Generation: q.Generation, Workspace: workspace, URI: q.URI, Line: q.Line, Character: q.Character, Version: prepared.Version, SourceDigest: binding.SHA256, SourceLength: len(source), Source: source, GitRoot: workspace, GitCommit: q.Revision, Executable: selected}
	if !verifyPreinvokeOccurrence(o.root, o.preinvokeSelector, o.preinvokeDigest, preinvoke, q.OccurrenceID) {
		return nil, ErrAcquisition
	}
	symbolParams, err := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": q.URI}})
	if err != nil {
		return nil, ErrAcquisition
	}
	refParams, err := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": q.URI}, "position": map[string]uint32{"line": q.Line, "character": q.Character}, "context": map[string]bool{"includeDeclaration": false}})
	if err != nil {
		return nil, ErrAcquisition
	}
	request := func(method string, params []byte, invocation string) (ownedFrames, error) {
		if !o.captureFrames || invocation == "" {
			return ownedFrames{}, ErrAcquisition
		}
		req := sessionruntime.RoundTripRequest{SessionID: q.SessionID, Generation: q.Generation, Method: method, Params: params, ExpectedOwnedDocument: &binding, CaptureOwnedMethodPair: true, ADR0011PrivateLimitAllocationV1: true, MaxMessages: 4, MaxBytes: 4194304, Deadline: time.Now().Add(15 * time.Second)}
		const frameCap = 2 << 20
		if method == "textDocument/documentSymbol" {
			req.CaptureDocumentSymbolRequestFrameMaxBytes = frameCap
			req.CaptureDocumentSymbolResponseFrameMaxBytes = frameCap
		} else if method == "textDocument/references" {
			req.CaptureMethodRequestFrameMaxBytes = frameCap
			req.CaptureReferencesResponseFrameMaxBytes = frameCap
		} else {
			return ownedFrames{}, ErrAcquisition
		}
		requestCtx := runtimeParent
		var runtimeReceipt *adr0011c18.Receipt
		if method == "textDocument/references" {
			requestCtx, runtimeReceipt = adr0011c18.BeginRuntime(ctx)
			if runtimeReceipt == nil || !runtimeReceipt.ReachRuntime(adr0011c18.PointPreflight) {
				return ownedFrames{}, ErrAcquisition
			}
		}
		if o.beforeMethodRequestTestHook != nil {
			o.beforeMethodRequestTestHook()
		}
		result := o.manager.RoundTrip(requestCtx, req)
		pair, ok := result.CompletedOwnedMethodPair()
		if !ok {
			return ownedFrames{}, ErrAcquisition
		}
		frames, ok := verifiedOwnedFrames(result, pair, method, params, binding, q.SessionID, q.Generation, frameCap)
		if !ok {
			return ownedFrames{}, ErrAcquisition
		}
		if runtimeReceipt != nil && !runtimeReceipt.SealRuntime() {
			return ownedFrames{}, ErrAcquisition
		}
		frames.invocation = invocation
		return frames, nil
	}
	symbolInvocation, err := ownerInvocation()
	if err != nil {
		return nil, err
	}
	symbolFrames, err := request("textDocument/documentSymbol", symbolParams, symbolInvocation)
	if err != nil {
		return nil, err
	}
	afterTarget, err := observeHostGit(ctx, workspace, q.Revision)
	if err != nil {
		return nil, ErrAcquisition
	}
	query := adr0011querytarget.Query{OccurrenceID: q.OccurrenceID, URI: q.URI, Encoding: "utf-16", DocumentVersion: strconv.Itoa(prepared.Version), SourceDigest: binding.SHA256, SessionID: q.SessionID, Generation: q.Generation, Line: q.Line, Character: q.Character}
	if _, err = adr0011querytarget.SelectDocumentSymbolCandidateV1(query, symbolFrames.pair.Result); err != nil {
		return nil, ErrAcquisition
	}
	expected := adr0011methodresult.ChainExpectation{Query: query, Source: source, Revision: q.Revision, Custody: "CALLER_ASSERTED", SymbolParams: symbolParams, ReferenceParams: refParams, TargetGit: adr0011methodresult.HostGitEvidence{Before: beforeTarget, After: afterTarget}}
	beforeMethod, err := observeHostGit(ctx, workspace, q.Revision)
	if err != nil {
		return nil, ErrAcquisition
	}
	refsInvocation, err := ownerInvocation()
	if err != nil || refsInvocation == symbolInvocation {
		return nil, ErrAcquisition
	}
	revisionExpected := hostGitExpectation{Executable: selected, Root: workspace, Commit: q.Revision, Phase: "BEFORE"}
	revisionBefore, err := observeSelectedGit(ctx, selected, workspace, q.Revision)
	if err != nil {
		return nil, ErrAcquisition
	}
	if o.beforeRevisionTestHook != nil {
		o.beforeRevisionTestHook(&revisionBefore, &revisionExpected)
	}
	refsFrames, err := request("textDocument/references", refParams, refsInvocation)
	if err != nil {
		return nil, err
	}
	if o.afterCaptureTestHook != nil {
		o.afterCaptureTestHook()
	}
	afterMethod, err := observeHostGit(ctx, workspace, q.Revision)
	if err != nil {
		return nil, ErrAcquisition
	}
	revisionAfter, err := observeSelectedGit(ctx, selected, workspace, q.Revision)
	if err != nil {
		return nil, ErrAcquisition
	}
	if o.afterRevisionTestHook != nil {
		o.afterRevisionTestHook(&revisionAfter, &revisionExpected)
	}
	expected.MethodGit = adr0011methodresult.HostGitEvidence{Before: beforeMethod, After: afterMethod}
	if symbolFrames.pair.Key == refsFrames.pair.Key {
		return nil, ErrAcquisition
	}
	if !verifyOriginalRequestKey(symbolFrames, symbolFrames.pair, q.SessionID, q.Generation, symbolInvocation, reviewedSuccessorSchemaDigest) ||
		!verifyOriginalRequestKey(refsFrames, refsFrames.pair, q.SessionID, q.Generation, refsInvocation, reviewedSuccessorSchemaDigest) {
		return nil, ErrAcquisition
	}
	if o.afterKeyGuardTestHook != nil {
		o.afterKeyGuardTestHook()
	}
	// These private observations remain inert until a separate final issuance
	// transaction can independently replay their entire dependency closure.
	var refsOwnerRead, symbolOwnerRead privateBodyPublication
	for _, item := range []struct {
		frames     ownedFrames
		invocation string
	}{{symbolFrames, symbolInvocation}, {refsFrames, refsInvocation}} {
		observed, observeErr := publishOwnerRead(o.root, item.frames, item.frames.pair, binding, q.SessionID, q.Generation, item.invocation, reviewedSuccessorSchemaDigest, nil)
		if observeErr != nil || observed.stage != "VERIFIED" {
			return nil, ErrAcquisition
		}
		if item.frames.pair.Method == "textDocument/references" {
			refsOwnerRead = observed
		} else {
			symbolOwnerRead = observed
		}
	}
	if o.afterOwnerReadTestHook != nil {
		o.afterOwnerReadTestHook()
	}
	// Inert synthetic checkpoint; no public receipt or issuance dependency yet.
	raw, rawErr := publishRawResultPayload(o.root, refsOwnerRead, refsFrames.pair, binding, q.SessionID, q.Generation, refsInvocation, reviewedSuccessorSchemaDigest, nil)
	if rawErr != nil || raw.stage != "VERIFIED" {
		return nil, ErrAcquisition
	}
	if o.afterRawPayloadTestHook != nil {
		o.afterRawPayloadTestHook()
	}
	// Private observation only: without accepted response-read/raw refs and an
	// independently pinned implementation digest this cannot form a receipt.
	if _, scanErr := observeRawScanner(o.root, raw, nil); scanErr != nil {
		return nil, ErrAcquisition
	}
	preparedRef, identityRef, sourceErr := publishPreparedSource(o.root, source, q.URI, prepared.Version, workspace, reviewedSuccessorSchemaDigest, nil)
	if sourceErr != nil || preparedRef.stage != "VERIFIED" || identityRef.stage != "VERIFIED" || !replayPreparedSource(o.root, preparedRef, identityRef, source, q.URI, prepared.Version, workspace, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		return nil, ErrAcquisition
	}
	streams := hostGitVerifiedStreams{}
	beforeRevision, beforeErr := publishHostGit(o.root, streams, revisionBefore, revisionExpected, reviewedSuccessorSchemaDigest, nil)
	if beforeErr != nil || beforeRevision.stage != "VERIFIED" {
		return nil, ErrAcquisition
	}
	afterExpected := revisionExpected
	afterExpected.Phase = "AFTER"
	afterRevision, afterErr := publishHostGit(o.root, streams, revisionAfter, afterExpected, reviewedSuccessorSchemaDigest, nil)
	if afterErr != nil || afterRevision.stage != "VERIFIED" {
		return nil, ErrAcquisition
	}
	identity, identityErr := publishRevisionIdentity(o.root, beforeRevision, afterRevision, revisionExpected, reviewedSuccessorSchemaDigest, nil)
	if identityErr != nil || identity.stage != "VERIFIED" || !replayRevisionIdentity(o.root, identity, beforeRevision, afterRevision, revisionExpected, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) || finalizeSelectedGit(selected) != nil {
		return nil, ErrAcquisition
	}
	if o.afterRevisionVerifiedTestHook != nil {
		o.afterRevisionVerifiedTestHook()
	}
	result, resultErr := publishTargetResult(o.root, symbolOwnerRead, symbolFrames.pair, binding, q.SessionID, q.Generation, symbolInvocation, reviewedSuccessorSchemaDigest, query, nil)
	if resultErr != nil || result.stage != "VERIFIED" {
		return nil, ErrAcquisition
	}
	if o.afterTargetResultTestHook != nil {
		o.afterTargetResultTestHook()
	}
	// Synthetic predecessor only: this caller-selected query occurrence is not
	// independently admitted for final references issuance by this checkpoint.
	targetInputs := targetRecordInputs{Frames: symbolFrames, Pair: symbolFrames.pair, Binding: binding, Query: query, Original: source, Workspace: workspace, Git: revisionExpected,
		Prepared: preparedRef, Source: identityRef, Revision: identity, Before: beforeRevision, After: afterRevision, OwnerRead: symbolOwnerRead, Result: result, SchemaDigest: reviewedSuccessorSchemaDigest}
	newTarget, newTargetErr := publishTargetRecord(o.root, targetInputs, nil)
	if newTargetErr != nil || newTarget.stage != "VERIFIED" || !replayTargetRecord(o.root, newTarget, targetInputs, publication.ReadVerifiedBoundFile) {
		return nil, ErrAcquisition
	}
	if o.afterTargetRecordTestHook != nil {
		o.afterTargetRecordTestHook()
	}
	responseInputs := responseReadInputs{TargetInputs: targetInputs, Target: newTarget, Frames: refsFrames, Pair: refsFrames.pair, Binding: binding, Query: query,
		OwnerRead: refsOwnerRead, Payload: raw, SchemaDigest: reviewedSuccessorSchemaDigest}
	responseRecord, responseErr := publishResponseRead(o.root, responseInputs, nil)
	if responseErr != nil || responseRecord.stage != "VERIFIED" || !replayResponseRead(o.root, responseRecord, responseInputs, publication.ReadVerifiedBoundFile) {
		return nil, ErrAcquisition
	}
	if o.afterResponseReadTestHook != nil {
		o.afterResponseReadTestHook()
	}
	rawRecord, rawRecordErr := publishRawResult(o.root, responseRecord, raw, responseInputs, nil)
	if rawRecordErr != nil || rawRecord.stage != "VERIFIED" || !replayRawResult(o.root, rawRecord, responseRecord, raw, responseInputs, publication.ReadVerifiedBoundFile) {
		return nil, ErrAcquisition
	}
	if o.afterRawRecordTestHook != nil {
		o.afterRawRecordTestHook()
	}
	scannerRecord, scannerErr := publishScannerRecord(o.root, responseRecord, rawRecord, raw, responseInputs, nil, nil)
	if scannerErr != nil || scannerRecord.stage != "VERIFIED" || !replayScannerRecord(o.root, scannerRecord, responseRecord, rawRecord, raw, responseInputs, publication.ReadVerifiedBoundFile, nil) {
		return nil, ErrAcquisition
	}
	if o.afterScannerRecordTestHook != nil {
		o.afterScannerRecordTestHook()
	}
	responseExpected, responseOK := responseReadExpected(o.root, responseInputs, publication.ReadVerifiedBoundFile)
	retainedRaw, rawErr := publication.ReadVerifiedBoundFile(o.root, raw.selector, rawResultPayloadLimit)
	scannerObserved, scannerObserveErr := observeRawScanner(o.root, raw, publication.ReadVerifiedBoundFile)
	if !responseOK || rawErr != nil || scannerObserveErr != nil || scannerObserved.form == "MALFORMED" {
		return nil, ErrAcquisition
	}
	transitionIdentity := adr0011methodresult.PrivateTransitionIdentity{Transaction: responseExpected.TransactionID, RequestKey: responseExpected.RequestKey,
		Invocation: responseExpected.InvocationID, ResponseRead: responseRecord.selector, RawSelector: rawRecord.selector, RawDigest: raw.digest}
	adr0011c18.Notify(ctx, adr0011c18.PointMethodResultHandoff)
	evaluation, journal, evalErr := adr0011methodresult.RunPrivateAttachedReferencesContext(ctx, o.root, refsFrames.pair.Key, retainedRaw, transitionIdentity)
	if evalErr != nil || evaluation == nil || len(journal) == 0 {
		return nil, ErrAcquisition
	}
	eventsState, eventsErr := publishEvents(o.root, responseRecord, rawRecord, scannerObserved, evaluation, responseInputs, responseExpected.TransactionID, retainedRaw, journal, publication.ReadVerifiedBoundFile, nil)
	if eventsErr != nil || eventsState.stage != "VERIFIED" || !replayEvents(o.root, eventsState, responseRecord, rawRecord, scannerObserved, responseInputs, responseExpected.TransactionID, retainedRaw, journal, publication.ReadVerifiedBoundFile) {
		return nil, ErrAcquisition
	}
	if o.afterEventsRecordTestHook != nil {
		o.afterEventsRecordTestHook()
	}
	var policies [4]policyPublication
	for i, selected := range policySelections {
		policy, policyErr := publishPolicy(o.root, selected, nil, nil)
		if policyErr != nil || !replayPolicy(o.root, selected, policy, nil, publication.ReadVerifiedBoundFile) {
			return nil, ErrAcquisition
		}
		policies[i] = policy
	}
	if o.afterPoliciesTestHook != nil {
		o.afterPoliciesTestHook()
	}
	methodInputs := methodRecordInputs{ResponseInputs: responseInputs, ResponseRead: responseRecord, RawResult: rawRecord}
	methodState, methodErr := publishMethodRecord(o.root, methodInputs, publication.ReadVerifiedBoundFile)
	if methodErr != nil || methodState.stage != "VERIFIED" || !replayMethodRecord(o.root, methodState, methodInputs, publication.ReadVerifiedBoundFile) {
		return nil, ErrAcquisition
	}
	if o.afterMethodRecordTestHook != nil {
		o.afterMethodRecordTestHook()
	}
	if finish {
		pin := o.finalImplementationPin
		if pin == nil {
			return nil, ErrAcquisition
		}
		implementationDigest, pinErr := pin()
		if pinErr != nil || !validPrivateDigest(implementationDigest) {
			return nil, ErrAcquisition
		}
		x := proposalContextInputs{TargetInputs: targetInputs, ResponseInputs: responseInputs, MethodInputs: methodInputs,
			Method: methodState, Target: newTarget, ResponseRead: responseRecord, RawResult: rawRecord, Scanner: scannerRecord, Events: eventsState,
			Policies: policies, Payload: retainedRaw, Journal: journal, ScannerObservation: scannerObserved, Evaluation: evaluation,
			Git: revisionExpected, SourceBytes: source, SchemaDigest: reviewedSuccessorSchemaDigest,
			ImplementationDigest: implementationDigest, OccurrenceID: q.OccurrenceID}
		if _, err := canonicalProposalContext(o.root, x, publication.ReadVerifiedBoundFile); err != nil {
			return nil, ErrAcquisition
		}
		proposal, err := publishProposalRecord(o.root, x, publication.ReadVerifiedBoundFile, nil)
		if err != nil || !replayProposalRecord(o.root, proposal, x, publication.ReadVerifiedBoundFile) {
			return nil, ErrAcquisition
		}
		candidate, err := publishCandidateRecord(o.root, proposal, x, publication.ReadVerifiedBoundFile, nil)
		if err != nil || !replayCandidateRecord(o.root, candidate, proposal, x, publication.ReadVerifiedBoundFile) {
			return nil, ErrAcquisition
		}
		declaration := finalDeclaration{Selector: o.preinvokeSelector, Digest: o.preinvokeDigest, Facts: preinvoke}
		final, err := publishFinalRecord(o.root, candidate, proposal, x, declaration, pin, publication.ReadVerifiedBoundFile, o.finalTraceTestHook)
		if err != nil || !replayFinalRecord(o.root, final, candidate, proposal, x, declaration, pin, publication.ReadVerifiedBoundFile) {
			return nil, ErrAcquisition
		}
		// Fresh retained readback follows replay; the typed receipt is never inferred
		// from an attempted publication or from an intermediate candidate.
		body, err := publication.ReadVerifiedBoundFile(o.root, final.selector, sourceRecordLimit)
		if err != nil || len(body) != final.byteCount || finalRecordDigest(body) != final.digest {
			return nil, ErrAcquisition
		}
		var issued struct {
			T int `json:"t"`
			A int `json:"a"`
		}
		if json.Unmarshal(body, &issued) != nil || issued.T != 1 || issued.A < 0 || issued.A > 1000 {
			return nil, ErrAcquisition
		}
		candidateBody, err := publication.ReadVerifiedBoundFile(o.root, candidate.selector, sourceRecordLimit)
		if err != nil {
			return nil, ErrAcquisition
		}
		var ordinals struct {
			Occurrences []struct {
				ID      string `json:"occurrence_id"`
				Ordinal int    `json:"ordinal"`
			} `json:"occurrences"`
		}
		if json.Unmarshal(candidateBody, &ordinals) != nil || len(ordinals.Occurrences) != issued.A {
			return nil, ErrAcquisition
		}
		ids := make([]string, len(ordinals.Occurrences))
		for i, occurrence := range ordinals.Occurrences {
			if occurrence.Ordinal != i || occurrence.ID == "" {
				return nil, ErrAcquisition
			}
			ids[i] = occurrence.ID
		}
		return &managedResult{final: &PrivateFinalReceipt{FinalRef: targetRecordRef(finalRecordRole, final), T: issued.T, A: issued.A, OrdinalRefs: ids}}, nil
	}
	target, err := adr0011methodresult.PublishTarget(o.root, symbolFrames.pair, expected)
	if err != nil {
		return nil, ErrAcquisition
	}
	receipt, err := adr0011methodresult.PublishReferences(o.root, target, symbolFrames.pair, refsFrames.pair, expected)
	if err != nil {
		return nil, ErrAcquisition
	}
	return &managedResult{legacy: receipt}, nil
}
