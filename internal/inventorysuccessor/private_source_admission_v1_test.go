package inventorysuccessor

import (
	"context"
	"lsp-trace/internal/programcadmission"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/sourceobject"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
	"testing"
)

// Test-variant only. Real Manager nil-consume/Snapshot evidence is deliberately
// assembled through the public lower candidate route, NOT a completion mint.
func ControlledNonProducerCandidateForTest(t *testing.T, e sessionruntime.PrivateGroupingSourceEvidence, v sessionruntime.PrivateGroupingOwnedView) {
	t.Helper()
	q := PrivateManagedGroupingRequest{Session: sessionruntime.SessionLeaseRequest{SessionID: v.Query.SessionID, Generation: v.Query.Generation, Workspace: v.Workspace}, URI: v.Query.URI, OccurrenceID: v.Query.OccurrenceID, Revision: v.Revision, Privacy: v.Privacy, Line: v.Query.Line, Character: v.Query.Character, Deadline: v.Deadline, OccurrenceAdmissionPolicyID: "ordinal", CompositionPolicyID: "identity", Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}}
	raw, err := privateGroupingQueryBytes(v)
	if err != nil {
		t.Fatal(err)
	}
	in, err := privateGroupingBuild(v, q, raw)
	if err != nil {
		t.Fatal(err)
	}
	c, err := tp.RetainPrivateSourceCandidate(e, in.Input, in.Predecessors, raw, v.Deadline, v.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	limits := tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}
	c, err = tp.PreparePrivateSourceCandidate(c, limits)
	if err != nil {
		t.Fatal(err)
	}
	var i tp.PrivateSourceIntent
	for _, r := range in.Input.Contributions() {
		if r.Kind == "REFERENCES_SYMBOL" {
			i = tp.PrivateSourceIntent{NominationID: "n", Mode: "RD", RunID: "r", InputID: in.Input.ID(), PolicyID: "p", CaseID: "c", ResultID: "r", Lineage: "l", BindingState: "UNRESOLVED", Requirement: "REQUIRES_ADMITTED_SOURCE_BINDING_AND_FULL_DEFINITION", ClaimCeiling: "", Endpoint: tp.PrivateTypedEndpoint{Kind: r.Kind, Node: r.To, Role: "REFERENCED_SYMBOL", Occurrence: r.OccurrenceIdentity, Ordinal: r.Ordinal, Predecessor: in.Input.PredecessorIdentityForContribution(r), Custody: "CALLER_ASSERTED", Disposition: "INCLUDED"}}
			break
		}
	}
	i.ClaimCeiling = rg.EvaluationCeilingPrivate
	candidate, gap := tp.CalculatePrivateEndpointSource(i, c, nil, []sourceobject.Object{{Identity: sourceobject.Identity{Digest: v.Query.SourceDigest, ByteLength: uint64(len(v.Source))}, Bytes: v.Source}}, limits)
	if !candidate.Valid() {
		t.Fatal("ASSERT_MANAGER_ONLY_CANDIDATE_POSITIVE", gap)
	}
	u, err := candidate.Unit()
	if err != nil || u.ClaimCeiling != tp.PrivateSourceCandidateCeiling || u.ID != "" {
		t.Fatal("ASSERT_LOWER_CANDIDATE_NOT_ADMISSION")
	}
	if _, err := in.SourceOriginalsV1(); err == nil {
		t.Fatal("ASSERT_MANAGER_CONSUME_NOT_PRODUCER_COMPLETION")
	}
	if _, err := mintPrivateSourceAdmission(i, PrivateProducerOriginals{}, candidate); err == nil {
		t.Fatal("ASSERT_CANDIDATE_CANNOT_MINT_UPPER_ADMISSION")
	}
	t.Log("ASSERT_MANAGER_ONLY_CANDIDATE_POSITIVE: PASS; ASSERT_MANAGER_CONSUME_NOT_PRODUCER_COMPLETION: PASS; ASSERT_CANDIDATE_CANNOT_MINT_UPPER_ADMISSION: PASS")
}

// Remove the unneeded native-replay payload only in this lower candidate. The
// exact privately admitted input and exact retained composite remain unchanged.
// Re-admission would reject this state; bounded source projection must not need it.
func ControlledSourceProjectionNoReadmitForTest(t *testing.T, in PrivateManagedGroupingInput) {
	t.Helper()
	p := privateGroupingClonePredecessors(in.originals)
	found := false
	for n := range p {
		if p[n].Role == "NATIVE_EVIDENCE" {
			p[n].Bytes = []byte("not replay evidence")
			found = true
		}
	}
	if !found {
		t.Fatal("ASSERT_SOURCE_NO_READMIT_NONVACUOUS")
	}
	c, err := tp.RetainPrivateSourceCandidate(in.sourceEvidence, in.sourceInputOriginal, p, in.sourceQueryOriginal, in.replayBinding.Deadline, in.replayBinding.Session.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tp.PreparePrivateSourceCandidate(c, tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxObjects: 1000, MaxWork: 10000})
	if err != nil {
		t.Fatal("ASSERT_SOURCE_PROJECTS_ALREADY_ADMITTED_INPUT", err)
	}
	t.Log("ASSERT_SOURCE_PROJECTS_ALREADY_ADMITTED_INPUT: PASS")
}

// This reaches the real private issue continuation, including paid initial
// admission/publication and token take, but deliberately stops before Observe.
func ControlledPendingSourceIssueForTest(t *testing.T, ctx context.Context, m *sessionruntime.Manager, h sessionruntime.PrivateGroupingOwnedHandoff, q PrivateManagedGroupingRequest, o *PrivateManagedInputOwnerV2) PrivateManagedGroupingInput {
	t.Helper()
	b, err := PrivateManagedGroupingOperationBinding(m, q, "ISSUE", h)
	if err != nil {
		t.Fatal(err)
	}
	op, err := o.Reserve(ctx, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", b)
	if err != nil {
		t.Fatal(err)
	}
	execution, cancel, err := op.consumeFixed(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	defer op.abortFixed()
	q.Operation = op
	in, err := issuePrivateManagedGroupingInput(execution, m, h, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.inputOriginal) == 0 || in.sourceEvidence.SourceByteLength() == 0 {
		t.Fatal("ASSERT_PENDING_REAL_ISSUE_NONVACUOUS")
	}
	if _, err := in.SourceOriginalsV1(); err == nil {
		t.Error("ASSERT_PENDING_PRODUCER_NO_ORIGINALS")
	}
	cancelled, stop := context.WithCancel(execution)
	stop()
	if op.completeFixed(cancelled) == nil {
		t.Fatal("ASSERT_FAILED_OBSERVE_REFUSED")
	}
	if _, err := in.SourceOriginalsV1(); err == nil {
		t.Error("ASSERT_FAILED_COMPLETION_NO_ORIGINALS")
	}
	if !t.Failed() {
		t.Log("ASSERT_PENDING_PRODUCER_NO_ORIGINALS: PASS; ASSERT_FAILED_COMPLETION_NO_ORIGINALS: PASS")
	}
	return in
}
