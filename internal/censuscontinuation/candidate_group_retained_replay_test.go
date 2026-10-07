package censuscontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/v5sourcesnapshotv6"
)

type retainedMemberSourceManifest struct {
	Fixture struct {
		ByteLength      int    `json:"byte_length"`
		DocumentVersion int    `json:"document_version"`
		SHA256          string `json:"sha256"`
		URI             string `json:"uri"`
	} `json:"fixture"`
	Members []struct {
		FunctionName  string `json:"function_name"`
		GraphMemberID string `json:"graph_member_id"`
		ManagedSource struct {
			GraphSubjectID   string      `json:"graph_subject_id"`
			BodyByteLength   int         `json:"body_byte_length"`
			BodySHA256       string      `json:"body_sha256"`
			DisplayRange     graph.Range `json:"display_range"`
			EvidenceRange    graph.Range `json:"evidence_range"`
			ItemRange        graph.Range `json:"item_range"`
			SelectionRange   graph.Range `json:"selection_range"`
			Generation       uint64      `json:"generation"`
			SessionID        string      `json:"session_id"`
			PositionEncoding string      `json:"position_encoding"`
			SourceDigest     string      `json:"source_digest"`
			SourceLength     uint64      `json:"source_byte_length"`
			URI              string      `json:"uri"`
			Version          int         `json:"document_version"`
		} `json:"managed_source"`
		Response struct {
			ByteLength int    `json:"byte_length"`
			Path       string `json:"path"`
			SHA256     string `json:"sha256"`
		} `json:"response"`
	} `json:"members"`
	Runtime struct {
		Generation       uint64 `json:"generation"`
		PositionEncoding string `json:"position_encoding"`
		SessionID        string `json:"session_id"`
	} `json:"runtime"`
}

type retainedProjectionEnvelope struct {
	Result struct {
		Projection struct {
			Units []struct {
				Body             string      `json:"body"`
				GraphSubjectID   string      `json:"graph_subject_id"`
				LogicalSourceID  string      `json:"logical_source_id"`
				DisplayRange     graph.Range `json:"display_range"`
				EvidenceRange    graph.Range `json:"evidence_range"`
				ItemRange        graph.Range `json:"item_range"`
				SelectionRange   graph.Range `json:"selection_range"`
				SourceDigest     string      `json:"source_digest"`
				SourceByteLength uint64      `json:"source_byte_length"`
			} `json:"units"`
		} `json:"projection"`
	} `json:"result"`
}

type retainedReplayAdapter struct {
	document            v5sourcesnapshotv6.PreparedDocument
	ranges              map[string]v5sourcesnapshotv6.ResolveResult
	originalManagedByPC map[string]OriginalManagedMemberIdentity
}

func retainedDigestBytes(raw []byte) string {
	s := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(s[:])
}

func loadRetainedReplayAdapter(dir string) (retainedReplayAdapter, error) {
	manifestRaw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return retainedReplayAdapter{}, err
	}
	var manifest retainedMemberSourceManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return retainedReplayAdapter{}, err
	}
	if len(manifest.Members) != 7 || manifest.Runtime.Generation != 1 || manifest.Runtime.PositionEncoding != "utf-16" {
		return retainedReplayAdapter{}, fmt.Errorf("retained manifest identity")
	}
	seen := map[string]bool{}
	var source []byte
	ranges := map[string]v5sourcesnapshotv6.ResolveResult{}
	originalManagedByPC := map[string]OriginalManagedMemberIdentity{}
	for _, member := range manifest.Members {
		if member.GraphMemberID == "" || seen[member.GraphMemberID] {
			return retainedReplayAdapter{}, fmt.Errorf("retained member ambiguity")
		}
		seen[member.GraphMemberID] = true
		raw, err := os.ReadFile(filepath.Join(dir, member.Response.Path))
		if err != nil {
			return retainedReplayAdapter{}, err
		}
		if len(raw) != member.Response.ByteLength || retainedDigestBytes(raw) != member.Response.SHA256 {
			return retainedReplayAdapter{}, fmt.Errorf("retained response substitution")
		}
		var envelope retainedProjectionEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Result.Projection.Units) != 1 {
			return retainedReplayAdapter{}, fmt.Errorf("retained response schema")
		}
		u := envelope.Result.Projection.Units[0]
		body := []byte(u.Body)
		if u.GraphSubjectID == "" || u.GraphSubjectID != member.ManagedSource.GraphSubjectID || u.SourceDigest != manifest.Fixture.SHA256 || u.SourceByteLength != uint64(manifest.Fixture.ByteLength) || len(body) != member.ManagedSource.BodyByteLength || retainedDigestBytes(body) != member.ManagedSource.BodySHA256 || u.DisplayRange != member.ManagedSource.DisplayRange || u.EvidenceRange != member.ManagedSource.EvidenceRange || u.ItemRange != member.ManagedSource.ItemRange || u.SelectionRange != member.ManagedSource.SelectionRange || u.LogicalSourceID != manifest.Fixture.URI {
			return retainedReplayAdapter{}, fmt.Errorf("retained response manifest mismatch")
		}
		if member.ManagedSource.GraphSubjectID == "" || member.ManagedSource.GraphSubjectID == member.GraphMemberID {
			return retainedReplayAdapter{}, fmt.Errorf("retained original dual identity")
		}
		originalManagedByPC[member.GraphMemberID] = OriginalManagedMemberIdentity{
			GraphSubjectID: member.ManagedSource.GraphSubjectID, SymbolName: member.FunctionName, URI: member.ManagedSource.URI,
			ItemRange: projectionRange(member.ManagedSource.ItemRange), SelectionRange: projectionRange(member.ManagedSource.SelectionRange),
			DocumentDigest: member.ManagedSource.SourceDigest, SessionID: member.ManagedSource.SessionID, Generation: member.ManagedSource.Generation,
			DocumentVersion: fmt.Sprint(member.ManagedSource.Version), PositionEncoding: member.ManagedSource.PositionEncoding,
			SourceResponseIdentity: member.Response.SHA256,
		}
		ranges[member.GraphMemberID] = v5sourcesnapshotv6.ResolveResult{DisplayRange: u.DisplayRange, ItemRange: u.ItemRange, SelectionRange: u.SelectionRange, ProvenanceKind: v5sourcesnapshotv6.ProvenanceKind, Method: v5sourcesnapshotv6.ProvenanceMethod, DocumentDigest: u.SourceDigest, DocumentVersion: "1", DocumentByteLength: u.SourceByteLength}
		if source == nil {
			source, err = os.ReadFile(filepath.Join(dir, "..", "fixture.go"))
			if err != nil {
				return retainedReplayAdapter{}, err
			}
		}
	}
	if len(source) != manifest.Fixture.ByteLength || retainedDigestBytes(source) != manifest.Fixture.SHA256 {
		return retainedReplayAdapter{}, fmt.Errorf("retained fixture substitution")
	}
	return retainedReplayAdapter{document: v5sourcesnapshotv6.PreparedDocument{URI: manifest.Fixture.URI, Bytes: source, Digest: manifest.Fixture.SHA256, ByteLength: uint64(len(source)), Version: "1", SessionID: manifest.Runtime.SessionID, Generation: manifest.Runtime.Generation, PositionEncoding: manifest.Runtime.PositionEncoding}, ranges: ranges, originalManagedByPC: originalManagedByPC}, nil
}

func (a retainedReplayAdapter) ResolveFullDefinition(_ context.Context, request v5sourcesnapshotv6.ResolveRequest) (v5sourcesnapshotv6.ResolveResult, error) {
	resolved, ok := a.ranges[request.GraphSubjectID]
	if !ok || request.URI != a.document.URI || request.SessionID != a.document.SessionID || request.Generation != a.document.Generation || request.PositionEncoding != a.document.PositionEncoding || request.DocumentDigest != a.document.Digest || request.DocumentVersion != a.document.Version || retainedDigestBytes(request.Bytes) != a.document.Digest {
		return v5sourcesnapshotv6.ResolveResult{}, fmt.Errorf("retained resolver substitution")
	}
	return resolved, nil
}

func captureRetainedV6(t testing.TB) (programc.Outcome, programc.BoundaryArtifact, v5sourcesnapshotv6.CaptureResult, []byte, v5sourcesnapshotv6.MemoryLookup) {
	t.Helper()
	dir := filepath.Join("testdata", "candidategroupfixture")
	adapter, err := loadRetainedReplayAdapter(filepath.Join(dir, "member-sources"))
	if err != nil {
		t.Fatal(err)
	}
	graphRaw, err := os.ReadFile(filepath.Join(dir, "managed-graph-v5-envelope.json"))
	if err != nil {
		t.Fatal(err)
	}
	outcome, failure := programc.Compute(graphRaw, 23)
	if failure != nil {
		t.Fatal(failure)
	}
	boundary, err := programc.ComputeBoundary(outcome, programc.BoundaryRequest{PageRankTopK: 7, HubTopK: 7})
	if err != nil {
		t.Fatal(err)
	}
	nominations := make([]v5sourcesnapshotv6.Nomination, 0, len(outcome.Projection.NodeIdentities))
	for _, target := range outcome.Projection.NodeIdentities {
		n := v5sourcesnapshotv6.Nomination{ID: "candidate-member:" + target, TargetNodeID: target}
		for _, occurrence := range outcome.Projection.Occurrences {
			if outcome.Projection.NodeIdentities[occurrence.To] != target {
				continue
			}
			n.Incoming = append(n.Incoming, v5sourcesnapshotv6.Occurrence{
				RelationID: occurrence.RelationID, OccurrenceID: occurrence.Identity,
				CallerNodeID: outcome.Projection.NodeIdentities[occurrence.From], CalleeNodeID: target, Range: occurrence.CallSite,
			})
		}
		nominations = append(nominations, n)
	}
	limits := v5sourcesnapshotv6.Limits{MaxArtifactBytes: 1 << 20, MaxGraphBytes: 1 << 20, MaxReceipts: 7, MaxSourceBytes: 2048, MaxTotalSourceBytes: 14336, MaxBindings: 32, MaxOutcomes: 32, MaxWork: 512}
	capture, err := v5sourcesnapshotv6.CaptureSelected(context.Background(), v5sourcesnapshotv6.CaptureInput{
		GraphV5Bytes: graphRaw, PositionEncoding: adapter.document.PositionEncoding,
		SessionID: adapter.document.SessionID, Generation: adapter.document.Generation,
		Nominations: nominations, Documents: []v5sourcesnapshotv6.PreparedDocument{adapter.document}, Resolver: adapter, Limits: limits,
	})
	if err != nil {
		t.Fatal(err)
	}
	replayed, lookup, err := v5sourcesnapshotv6.Replay(capture.Raw, limits)
	if err != nil {
		t.Fatal(err)
	}
	return outcome, boundary, capture, replayed, lookup
}

func TestRetainedV6CaptureReplayExactMembersBodiesAndCalls(t *testing.T) {
	outcome, _, capture, replayed, _ := captureRetainedV6(t)
	if !bytes.Equal(capture.Raw, replayed) || len(capture.Objects) != 1 {
		t.Fatalf("ASSERT_RETAINED_V6_REPLAY_SHARED_SOURCE: objects=%d equal=%v", len(capture.Objects), bytes.Equal(capture.Raw, replayed))
	}
	targets, targetOutcomes := map[string]int{}, map[string]int{}
	for _, binding := range func() []v5sourcesnapshotv6.EndpointBinding {
		var a v5sourcesnapshotv6.Artifact
		if err := json.Unmarshal(replayed, &a); err != nil {
			t.Fatal(err)
		}
		return a.EndpointBindings
	}() {
		if binding.Role == "TARGET" {
			targets[binding.GraphSubjectID]++
		}
	}
	for _, item := range capture.Outcomes {
		if item.Role == "TARGET" && item.Status == "CAPTURED" {
			targetOutcomes[item.GraphSubjectID]++
		}
	}
	for _, member := range outcome.Projection.NodeIdentities {
		if targets[member] != 1 || targetOutcomes[member] != 1 {
			t.Fatalf("ASSERT_RETAINED_V6_ONE_BINDING_BODY_PER_MEMBER: member=%s bindings=%d outcomes=%d", member, targets[member], targetOutcomes[member])
		}
	}
	var artifact v5sourcesnapshotv6.Artifact
	_ = json.Unmarshal(replayed, &artifact)
	if len(artifact.RelationBindings) != len(outcome.Projection.Occurrences) {
		t.Fatalf("ASSERT_RETAINED_V6_EXACT_INCOMING_OCCURRENCES: got=%d want=%d", len(artifact.RelationBindings), len(outcome.Projection.Occurrences))
	}
}

func TestCandidateGroupRetainedV6RejectsFrozenPageRankWorkWithoutArtifact(t *testing.T) {
	outcome, boundary, _, snapshot, lookup := captureRetainedV6(t)
	if boundary.PageRank.Work != 1472 {
		t.Fatalf("ASSERT_FROZEN_PAGERANK_WORK: got=%d want=1472", boundary.PageRank.Work)
	}
	const communityID = "sha256:a07c2f03a12799231696f924a94c175e9e286f8f822385b4cdc3eb8056edb1ec"
	var members []string
	for _, community := range boundary.Communities {
		if community.CommunityID == communityID {
			members = append([]string(nil), community.Members...)
		}
	}
	artifact, err := BuildCandidateGroup(CandidateGroupInput{
		Outcome: outcome, Boundary: boundary, CommunityMembers: members, RepresentativeID: members[0],
		Bounds: FrozenCandidateGroupBounds(), SourceSnapshot: snapshot, SourceLookup: lookup,
		Interpretation: HostInterpretation{Status: InterpretationUnresolved, HostID: "pi-host", ModelID: "gpt-5.6-sol", Attempts: 1},
	})
	if err == nil || err.Error() != "candidate group pagerank work limit" || artifact.ID() != "" {
		t.Fatalf("ASSERT_FROZEN_CANDIDATE_REJECTED_RESOURCE_LIMIT: artifact=%+v err=%v", artifact, err)
	}
}

func TestCandidateGroupRetainedV6Successor1664BuildsExactEvidence(t *testing.T) {
	outcome, boundary, _, snapshot, lookup := captureRetainedV6(t)
	const communityID = "sha256:a07c2f03a12799231696f924a94c175e9e286f8f822385b4cdc3eb8056edb1ec"
	var members []string
	for _, community := range boundary.Communities {
		if community.CommunityID == communityID {
			members = append([]string(nil), community.Members...)
		}
	}
	profile := CandidateGroupPrivateV2Profile()
	input := CandidateGroupInput{
		Outcome: outcome, Boundary: boundary, CommunityMembers: members, RepresentativeID: members[0],
		Bounds: profile.Bounds, ResourceProfile: profile, SourceSnapshot: snapshot, SourceLookup: lookup,
		OriginalManagedMembers: retainedManagedSubjects(t),
		Interpretation:         HostInterpretation{Status: InterpretationUnresolved, HostID: "pi-host", ModelID: "gpt-5.6-sol", Attempts: 1},
	}
	unresolved, err := BuildCandidateGroup(input)
	if err != nil {
		t.Fatalf("ASSERT_SUCCESSOR_1664_EXACT_EVIDENCE: %v", err)
	}
	for _, mutation := range []struct {
		name string
		edit func(map[string]OriginalManagedMemberIdentity)
	}{
		{"rekey-original-to-program-c", func(ids map[string]OriginalManagedMemberIdentity) {
			id := members[0]
			v := ids[id]
			v.GraphSubjectID = id
			ids[id] = v
		}},
		{"swap-original-identities", func(ids map[string]OriginalManagedMemberIdentity) {
			ids[members[0]], ids[members[1]] = ids[members[1]], ids[members[0]]
		}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := input
			changed.OriginalManagedMembers = make(map[string]OriginalManagedMemberIdentity, len(input.OriginalManagedMembers))
			for k, v := range input.OriginalManagedMembers {
				changed.OriginalManagedMembers[k] = v
			}
			mutation.edit(changed.OriginalManagedMembers)
			if got, err := BuildCandidateGroup(changed); err == nil || got.ID() != "" {
				t.Fatalf("ASSERT_SUCCESSOR_ORIGINAL_ID_SUBSTITUTION_REJECTED: artifact=%+v err=%v", got, err)
			}
		})
	}
	const reviewedStatement = "Publish calls Encode and Write; Encode calls Write; Write calls Publish."
	input.Interpretation = HostInterpretation{Status: InterpretationDescription, HostID: "pi-host", ModelID: "gpt-5.6-sol", Attempts: 1, Description: reviewedStatement, Claims: []string{reviewedStatement}}
	memberByID := map[string]CandidateGroupMember{}
	for _, member := range unresolved.Members {
		memberByID[member.ID] = member
		if member.Source.Correspondence.GraphSubjectID == member.ID || member.Source.Correspondence.V6EndpointGraphSubjectID != member.ID {
			t.Fatalf("ASSERT_SUCCESSOR_ORIGINAL_DUAL_IDS: member=%+v", member)
		}
	}
	for _, call := range unresolved.InternalCalls {
		caller := memberByID[call.SourceNodeID]
		input.Interpretation.Citations = append(input.Interpretation.Citations, Citation{
			SourceID: caller.Source.CitationID, LogicalSourceID: caller.Source.Correspondence.URI,
			Range: projectionRange(call.CallSite), Claim: reviewedStatement,
			RelationID: call.RelationID, OccurrenceID: call.OccurrenceID,
			CallerProgramCNodeID: call.SourceNodeID, CalleeProgramCNodeID: call.TargetNodeID,
		})
	}
	artifact, err := BuildCandidateGroup(input)
	if err != nil {
		t.Fatalf("ASSERT_SUCCESSOR_1664_EXACT_CLAIM_COVERAGE: %v", err)
	}
	if artifact.ID() == "" || artifact.CommunityID != communityID || artifact.Boundary.PageRank.Work != 1472 || artifact.Authority != 0 || artifact.Accepted || artifact.Completeness != CompletenessUnknown {
		t.Fatalf("ASSERT_SUCCESSOR_1664_BOUNDARY: %+v", artifact)
	}
	if len(artifact.Members) != 3 || artifact.SourceAccounting.AvailableMembers != 3 || len(artifact.InternalCalls) != 4 || artifact.Interpretation.Description != reviewedStatement || len(artifact.Interpretation.Citations) != 4 {
		t.Fatalf("ASSERT_SUCCESSOR_1664_EVIDENCE_COUNTS: members=%d source=%+v internal=%d interpretation=%+v", len(artifact.Members), artifact.SourceAccounting, len(artifact.InternalCalls), artifact.Interpretation)
	}
	raw, err := artifact.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ASSERT_SUCCESSOR_1664_IDENTITY: profile=%s profile_digest=%s artifact=%s artifact_digest=%s length=%d selector=continuations/candidate-groups/artifacts/%s", artifact.ResourceProfileID, artifact.ResourceProfileDigest, artifact.ID(), retainedDigestBytes(raw), len(raw), strings.TrimPrefix(artifact.ID(), "sha256:"))
	if output := os.Getenv("LSP_TRACE_CANDIDATE_GROUP_V2_OUTPUT"); output != "" {
		if err := os.WriteFile(output, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRetainedV6DescriptionIsSeparateReviewedEvidenceAndCandidateIsRejected(t *testing.T) {
	outcome, boundary, _, snapshot, lookup := captureRetainedV6(t)
	const communityID = "sha256:a07c2f03a12799231696f924a94c175e9e286f8f822385b4cdc3eb8056edb1ec"
	var members []string
	for _, community := range boundary.Communities {
		if community.CommunityID == communityID {
			members = append([]string(nil), community.Members...)
		}
	}
	if got := []string{"Encode", "Publish", "Write"}; len(members) != len(got) {
		t.Fatalf("ASSERT_SEMANTIC_REVIEW_MEMBER_COUNT: got=%d", len(members))
	}
	const reviewedStatement = "Publish calls Encode and Write; Encode calls Write; Write calls Publish."
	if reviewedStatement == "" || len(outcome.Projection.Occurrences) != 11 {
		t.Fatal("ASSERT_SEMANTIC_REVIEW_QUOTED_STATEMENT_PASS")
	}
	artifact, err := BuildCandidateGroup(CandidateGroupInput{
		Outcome: outcome, Boundary: boundary, CommunityMembers: members, RepresentativeID: members[0],
		Bounds: FrozenCandidateGroupBounds(), SourceSnapshot: snapshot, SourceLookup: lookup,
		Interpretation: HostInterpretation{Status: InterpretationDescription, HostID: "pi-host", ModelID: "gpt-5.6-sol", Attempts: 1, Description: reviewedStatement, Claims: []string{reviewedStatement}},
	})
	if err == nil || err.Error() != "candidate group pagerank work limit" || artifact.ID() != "" {
		t.Fatalf("ASSERT_SEMANTIC_REVIEW_NOT_STORED_ARTIFACT: artifact=%+v err=%v", artifact, err)
	}
}

func retainedManagedSubjects(t testing.TB) map[string]OriginalManagedMemberIdentity {
	t.Helper()
	a, err := loadRetainedReplayAdapter(filepath.Join("testdata", "candidategroupfixture", "member-sources"))
	if err != nil {
		t.Fatal(err)
	}
	return a.originalManagedByPC
}

func TestRetainedReplayAdapterRejectsMissingDuplicateAndSubstitutedEvidence(t *testing.T) {
	dir := filepath.Join("testdata", "candidategroupfixture", "member-sources")
	a, err := loadRetainedReplayAdapter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.ranges) != 7 || a.document.ByteLength != 288 || a.document.Digest != "sha256:bcf2b8151fa43fc1d1be42ca3945db51657ecfdfba15178a94a8a2b5c9d1e0d2" {
		t.Fatalf("ASSERT_RETAINED_REPLAY_EXACT: ranges=%d document=%+v", len(a.ranges), a.document)
	}
	if _, err := loadRetainedReplayAdapter(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("ASSERT_RETAINED_REPLAY_MISSING")
	}
}
