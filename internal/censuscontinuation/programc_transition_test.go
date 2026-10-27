package censuscontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/publication"
)

type transitionObjects struct {
	m          map[string][]byte
	getErr     map[string]error
	putErr     error
	onGet      func(string)
	puts, gets int
}

func (s *transitionObjects) Put(_ context.Context, b []byte) (string, error) {
	s.puts++
	if s.putErr != nil {
		return "", s.putErr
	}
	if s.m == nil {
		s.m = map[string][]byte{}
	}
	k := transitionDigest(b)
	s.m[k] = bytes.Clone(b)
	return k, nil
}
func (s *transitionObjects) Get(_ context.Context, k string) ([]byte, error) {
	s.gets++
	if s.onGet != nil {
		s.onGet(k)
	}
	if err := s.getErr[k]; err != nil {
		return nil, err
	}
	b, ok := s.m[k]
	if !ok {
		return nil, os.ErrNotExist
	}
	return bytes.Clone(b), nil
}

type transitionHeads struct {
	raw                     []byte
	loadErr                 error
	loads, cas, uncertainAt int
}

func (s *transitionHeads) Load(context.Context, string) ([]byte, error) {
	s.loads++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return bytes.Clone(s.raw), nil
}
func (s *transitionHeads) CompareAndSwap(_ context.Context, _ string, e censussourcecontinuation.HeadRevision, b []byte) censussourcecontinuation.CASResult {
	s.cas++
	cur, _ := ParseProgramCTransitionHeadRevision(s.raw)
	if cur != e {
		return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASNotCommitted}
	}
	s.raw = bytes.Clone(b)
	if s.cas == s.uncertainAt {
		return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitUncertain}
	}
	return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitted}
}

type transitionRunner struct {
	publications, constituents []string
	seeds                      []uint64
	result                     []byte
	outcome                    string
	fail                       error
	instabilityFail            error
	validateFail               error
	onCompute                  func()
}

func (r *transitionRunner) Resolve(_ context.Context, p, c string) ([]byte, error) {
	r.publications = append(r.publications, p)
	r.constituents = append(r.constituents, c)
	if r.fail != nil {
		return nil, r.fail
	}
	return []byte("native-v5"), nil
}
func (r *transitionRunner) Compute(_ []byte, s uint64) (programc.Outcome, *programc.Failure) {
	r.seeds = append(r.seeds, s)
	if r.onCompute != nil {
		r.onCompute()
	}
	if r.fail != nil {
		return programc.Outcome{}, &programc.Failure{Code: programc.CodeComputation, Message: r.fail.Error()}
	}
	out := r.outcome
	if out == "" {
		out = "COMPLETE"
	}
	return programc.Outcome{Outcome: out, Seed: s}, nil
}
func (r *transitionRunner) Instability(_ []programc.Outcome, _ ProgramCTransitionBindings) ([]byte, error) {
	if r.instabilityFail != nil {
		return nil, r.instabilityFail
	}
	return bytes.Clone(r.result), nil
}
func (r *transitionRunner) ValidateInstability(raw []byte, _ []programc.Outcome, _ ProgramCTransitionBindings) error {
	if r.validateFail != nil {
		return r.validateFail
	}
	if !bytes.Equal(raw, r.result) || transitionValidateResult(raw) != nil {
		return ErrInvalidProgramCTransition
	}
	return nil
}

type publicationTransitionRunner struct {
	transitionRunner
	production ProgramCProductionRunner
}

func (r *publicationTransitionRunner) Resolve(ctx context.Context, publication, constituent string) ([]byte, error) {
	r.publications = append(r.publications, publication)
	r.constituents = append(r.constituents, constituent)
	return r.production.Resolve(ctx, publication, constituent)
}

type transitionPlanSource struct {
	raw   []byte
	loads int
}

func (s *transitionPlanSource) Load(context.Context) ([]byte, error) {
	s.loads++
	return bytes.Clone(s.raw), nil
}

type transitionReaderFunc func(context.Context, int64) ([]byte, error)

func (f transitionReaderFunc) ReadBounded(ctx context.Context, limit int64) ([]byte, error) {
	return f(ctx, limit)
}

type transitionFaultContext struct {
	parent context.Context
	done   chan struct{}
	err    error
}

func newTransitionFaultContext() *transitionFaultContext {
	return &transitionFaultContext{parent: context.Background(), done: make(chan struct{})}
}

func (c *transitionFaultContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *transitionFaultContext) Done() <-chan struct{}       { return c.done }
func (c *transitionFaultContext) Err() error                  { return c.err }
func (c *transitionFaultContext) Value(key any) any           { return c.parent.Value(key) }
func (c *transitionFaultContext) fail(err error) {
	if c.err == nil {
		c.err = err
		close(c.done)
	}
}

func transitionDigest(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
func transitionBindings(planRaw []byte) ProgramCTransitionBindings {
	return ProgramCTransitionBindings{RepositoryCommit: "7c25c029b46b0f918bd1869f3b364e8a757525c2", RepositoryTree: "77df528576ef9bf7b05e29cc5a53038003050c6c", FrozenCorpusSHA256: "sha256:" + strings.Repeat("1", 64), CensusReceiptSHA256: "sha256:" + strings.Repeat("2", 64), LedgerSHA256: "sha256:" + strings.Repeat("3", 64), PlanSHA256: transitionDigest(planRaw), AlgorithmVersion: "leiden/v1", Seeds: [2]uint64{1, 2}, ReplaysPerSeed: 3, ParametersSHA256: "sha256:" + strings.Repeat("4", 64), ResourcePolicySHA256: "sha256:" + strings.Repeat("5", 64)}
}
func transitionPlan(t *testing.T, records []ProgramCTransitionPlanRecord) []byte {
	t.Helper()
	records = append([]ProgramCTransitionPlanRecord(nil), records...)
	for i := range records {
		if records[i].ReceiptOrdinal == 0 {
			records[i].ReceiptOrdinal = i + 1
		}
		if records[i].PublicationByteLength == 0 {
			records[i].PublicationByteLength = int64(i + 1)
		}
	}
	raw, err := EncodeProgramCTransitionPlan(ProgramCTransitionPlan{SchemaVersion: ProgramCTransitionPlanVersion, Authority: 0, CensusReceiptSHA256: "sha256:" + strings.Repeat("2", 64), LedgerSHA256: "sha256:" + strings.Repeat("3", 64), Records: records})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func transitionResultBytes(outcome string) []byte {
	b, _ := json.Marshal(programc.InstabilityArtifact{SchemaVersion: programc.InstabilityVersionV2, Outcome: outcome, PairwiseComparisons: []programc.InstabilityPair{}, ClaimCeiling: programc.InstabilityClaimCeilingV2, StructuralClaimCeiling: programc.InstabilityStructuralCeiling})
	return b
}
func transitionV3IncompleteResultBytes() []byte {
	pairs := make([]programc.InstabilityPairV3, 0, 15)
	for left := 'a'; left <= 'f'; left++ {
		for right := left + 1; right <= 'f'; right++ {
			states, work := uint64(131072), uint64(2228224)
			pairs = append(pairs, programc.InstabilityPairV3{LeftRunID: string(left), RightRunID: string(right), Outcome: "BLOCKED_MATCHING_RESOURCE", GuardKind: "MATCHING_WORK_LIMIT", MatchingSmallerCommunityCount: 17, MatchingLargerCommunityCount: 17, MatchingStateCount: &states, MatchingWork: &work, MatchingWorkLimit: 1000000, Reason: "exact community matching exceeds frozen resource policy"})
		}
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	b, _ := json.Marshal(programc.InstabilityArtifactV3{
		SchemaVersion:      programc.InstabilityVersionV3,
		ComparisonIdentity: programc.InstabilityIdentity{AdmittedGraphSHA256: digest, ProjectionSHA256: digest, ProjectionPolicyID: "calls-v1", AlgorithmName: "leiden", AlgorithmVersion: "v", ParametersCanonicalSHA256: digest, ResourcePolicySHA256: digest},
		Outcome:            "INCOMPLETE", Thresholds: programc.InstabilityThresholds{NodeReassignmentRateMax: .05, UnmatchedCommunityRateMax: .05, VariationOfInformationBitsMax: .1, PairwiseJaccardMinimumMin: .8},
		PolicyVersion: programc.InstabilityPolicyVersion, Policy: programc.InstabilityPolicy, PolicySHA256: programc.InstabilityPolicyDigest(),
		RunAccounting:       programc.InstabilityAccountingV3{DeclaredSeedCount: 2, DeclaredRunCount: 6, CompletedRunCount: 6, ExpectedPairCount: 15, BlockedMatchingResourcePairCount: 15},
		PairwiseComparisons: pairs, ClaimCeiling: programc.InstabilityClaimCeilingV3, StructuralClaimCeiling: programc.InstabilityStructuralCeiling,
	})
	return b
}
func transitionRequest(raw []byte, o *transitionObjects, h *transitionHeads, r ProgramCTransitionRunner) ProgramCTransitionRequest {
	return ProgramCTransitionRequest{WorkflowKey: "workflow", HeadKey: "head", Bindings: transitionBindings(raw), PlanSource: &transitionPlanSource{raw: raw}, Objects: o, Heads: h, Runner: r}
}
func records2() []ProgramCTransitionPlanRecord {
	return []ProgramCTransitionPlanRecord{{Ordinal: 0, PublicationSelector: "capture-sets/v1/sha256/" + strings.Repeat("a", 64) + ".bundle", ConstituentSelector: "graph-provenance-v5/sha256/" + strings.Repeat("b", 64)}, {Ordinal: 1, PublicationSelector: "capture-sets/v1/sha256/" + strings.Repeat("c", 64) + ".bundle", ConstituentSelector: "graph-provenance-v5/sha256/" + strings.Repeat("d", 64)}}
}

func TestProgramCTransitionPlanRejectsNonCanonicalLineageOrder(t *testing.T) {
	records := records2()
	records[0].ReceiptOrdinal, records[0].PublicationByteLength = 2, 2
	records[1].ReceiptOrdinal, records[1].PublicationByteLength = 1, 1
	if _, err := EncodeProgramCTransitionPlan(ProgramCTransitionPlan{SchemaVersion: ProgramCTransitionPlanVersion, Authority: 0, CensusReceiptSHA256: "sha256:" + strings.Repeat("2", 64), LedgerSHA256: "sha256:" + strings.Repeat("3", 64), Records: records}); !errors.Is(err, ErrInvalidProgramCTransition) {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_PLAN_ORDER_REJECTED err=%v", err)
	}
	records[0].ReceiptOrdinal, records[0].PublicationByteLength = 1, 1
	records[1].ReceiptOrdinal, records[1].PublicationByteLength = 1, 1
	if _, err := EncodeProgramCTransitionPlan(ProgramCTransitionPlan{SchemaVersion: ProgramCTransitionPlanVersion, Authority: 0, CensusReceiptSHA256: "sha256:" + strings.Repeat("2", 64), LedgerSHA256: "sha256:" + strings.Repeat("3", 64), Records: records}); !errors.Is(err, ErrInvalidProgramCTransition) {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_PLAN_DUPLICATE_ORDER_KEY_REJECTED err=%v", err)
	}
}

func TestProgramCTransitionInternalOrdinalProgressionAndCompletion(t *testing.T) {
	raw := transitionPlan(t, records2())
	o, h := &transitionObjects{}, &transitionHeads{}
	r := &transitionRunner{result: transitionResultBytes("STABLE")}
	req := transitionRequest(raw, o, h, r)
	first, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || first.Status != ProgramCTransitionCommitted || first.CommittedPrefix != 1 || first.ConstituentCount != 2 || first.Ordinal != 0 || r.constituents[0] != records2()[0].ConstituentSelector {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_INTERNAL_ORDINAL_ZERO first=%+v selectors=%v err=%v", first, r.constituents, err)
	}
	req.Expected = first.Head
	second, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || second.Status != ProgramCTransitionCommitted || second.CommittedPrefix != 2 || second.Ordinal != 1 || !reflect.DeepEqual(r.constituents, []string{records2()[0].ConstituentSelector, records2()[1].ConstituentSelector}) {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_N_TO_N_PLUS_ONE second=%+v selectors=%v err=%v", second, r.constituents, err)
	}
	req.Expected = second.Head
	puts, seeds, resolves := o.puts, len(r.seeds), len(r.constituents)
	done, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || done.Status != ProgramCTransitionComplete || done.CommittedPrefix != 2 || o.puts != puts || len(r.seeds) != seeds || len(r.constituents) != resolves {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_COMPLETION_NO_EFFECT done=%+v err=%v", done, err)
	}
}

func TestProgramCTransitionRequestHasNoCallerSelection(t *testing.T) {
	typ := reflect.TypeOf(ProgramCTransitionRequest{})
	for _, name := range []string{"ConstituentSelector", "PublicationSelector", "Processed", "ProcessedSet"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_NO_CALLER_SELECTOR field=%s", name)
		}
	}
}

func TestProgramCTransitionV3MatchingResourceIncompleteAdvancesOnce(t *testing.T) {
	raw := transitionPlan(t, records2()[:1])
	o, h := &transitionObjects{}, &transitionHeads{}
	r := &transitionRunner{result: transitionV3IncompleteResultBytes()}
	req := transitionRequest(raw, o, h, r)
	first, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || first.Status != ProgramCTransitionCommitted || first.CommittedPrefix != 1 || first.Disposition != "COMMITTED_INCOMPLETE_MATCHING_RESOURCE" {
		t.Fatalf("ASSERT_PROGRAM_C_V3_INCOMPLETE_ADVANCES first=%+v err=%v", first, err)
	}
	puts, resolves, computes, cas := o.puts, len(r.constituents), len(r.seeds), h.cas
	req.Expected = first.Head
	second, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || second.Status != ProgramCTransitionComplete || second.CommittedPrefix != 1 || second.Disposition != "COMMITTED_INCOMPLETE_MATCHING_RESOURCE" || o.puts != puts || len(r.constituents) != resolves || len(r.seeds) != computes || h.cas != cas {
		t.Fatalf("ASSERT_PROGRAM_C_V3_INCOMPLETE_REOPEN_NO_REPLAY second=%+v puts=%d/%d resolves=%d/%d computes=%d/%d cas=%d/%d err=%v", second, o.puts, puts, len(r.constituents), resolves, len(r.seeds), computes, h.cas, cas, err)
	}
}

func TestProgramCTransitionEmptyAdvancesDistinctly(t *testing.T) {
	raw := transitionPlan(t, records2()[:1])
	o, h := &transitionObjects{}, &transitionHeads{}
	r := &transitionRunner{outcome: "EMPTY", result: transitionResultBytes("STABLE")}
	got, err := CoordinateProgramCTransition(context.Background(), transitionRequest(raw, o, h, r))
	if err != nil || got.Status != ProgramCTransitionCommitted || got.ProgramCOutcome != "EMPTY" || got.CommittedPrefix != 1 || !reflect.DeepEqual(r.seeds, []uint64{1, 1, 1, 2, 2, 2}) {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_EMPTY_ACCEPTED got=%+v seeds=%v err=%v", got, r.seeds, err)
	}
}

func TestProgramCProductionRunnerEmptyInstabilityIsIncomplete(t *testing.T) {
	native, err := json.Marshal(graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Provenance: graph.InvocationProvenance{InvocationID: "empty-instability", SourceRevision: "rev", ServerVersion: "v"}},
		Summary:       graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	exact, err := graphprovenance.CaptureV5(native, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	outcomes := make([]programc.Outcome, 0, 6)
	for _, seed := range []uint64{1, 2} {
		for replay := 0; replay < 3; replay++ {
			outcome, failure := programc.Compute(exact, seed)
			if failure != nil || outcome.Outcome != "EMPTY" {
				t.Fatalf("ASSERT_PROGRAM_C_EMPTY_FIXTURE outcome=%+v failure=%v", outcome, failure)
			}
			outcomes = append(outcomes, outcome)
		}
	}
	bindings := transitionBindings(transitionPlan(t, records2()[:1]))
	raw, err := (ProgramCProductionRunner{}).Instability(outcomes, bindings)
	if err != nil {
		t.Fatal(err)
	}
	var artifact programc.InstabilityArtifact
	if err = json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.SchemaVersion != programc.InstabilityVersionV2 || artifact.Outcome != "INCOMPLETE" || artifact.RunAccounting.IncompleteRunCount != 6 || artifact.RunAccounting.CompletedRunCount != 0 || artifact.RunAccounting.BlockedPairCount != 15 || artifact.RunAccounting.CompletedPairCount != 0 {
		t.Fatalf("ASSERT_PROGRAM_C_EMPTY_INSTABILITY_INCOMPLETE artifact=%+v", artifact)
	}
	if err = (ProgramCProductionRunner{}).ValidateInstability(raw, outcomes, bindings); err != nil {
		t.Fatalf("ASSERT_PROGRAM_C_EMPTY_INSTABILITY_VALID err=%v", err)
	}
}

func TestProgramCProductionRunnerNonemptyBytesMatchPreEmptyBranch(t *testing.T) {
	node := func(name string, line uint32) graph.Node {
		rangeValue := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 1}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: rangeValue, SelectionRange: rangeValue})
	}
	a, b := node("a", 0), node("b", 1)
	native, err := json.Marshal(graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Provenance: graph.InvocationProvenance{InvocationID: "nonempty-instability", SourceRevision: "rev", ServerVersion: "v"}},
		Summary:       graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
		Nodes: []graph.Node{a, b}, Edges: []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	exact, err := graphprovenance.CaptureV5(native, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	outcomes := make([]programc.Outcome, 0, 6)
	for _, seed := range []uint64{1, 2} {
		for replay := 0; replay < 3; replay++ {
			outcome, failure := programc.Compute(exact, seed)
			if failure != nil || outcome.Outcome != "COMPLETE" {
				t.Fatalf("ASSERT_PROGRAM_C_NONEMPTY_FIXTURE outcome=%+v failure=%v", outcome, failure)
			}
			outcomes = append(outcomes, outcome)
		}
	}
	bindings := transitionBindings(transitionPlan(t, records2()[:1]))
	got, err := (ProgramCProductionRunner{}).Instability(outcomes, bindings)
	if err != nil {
		t.Fatal(err)
	}
	id := programc.InstabilityIdentity{AdmittedGraphSHA256: outcomes[0].Source.InputSHA256, ProjectionSHA256: programc.ProfileDigest, ProjectionPolicyID: programc.ProfileID, AlgorithmName: outcomes[0].Algorithm, AlgorithmVersion: bindings.AlgorithmVersion, ParametersCanonicalSHA256: bindings.ParametersSHA256, ResourcePolicySHA256: bindings.ResourcePolicySHA256}
	runs := make([]programc.InstabilityRun, 0, 6)
	for i, outcome := range outcomes {
		community, buildErr := programc.BuildCommunityArtifact(outcome, id)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		boundaryRequest := programc.BoundaryRequest{PageRankTopK: 1, HubTopK: 1}
		boundary, buildErr := programc.ComputeBoundary(outcome, boundaryRequest)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		boundaryRaw, _ := json.Marshal(boundary)
		runs = append(runs, programc.InstabilityRun{RunID: fmt.Sprintf("seed-%020d-replay-%d", outcome.Seed, i%3+1), Seed: outcome.Seed, Status: "COMPLETE", Community: outcome, CommunityArtifact: community, BoundaryArtifact: boundaryRaw, BoundaryRequest: boundaryRequest})
	}
	legacy, err := programc.ComputeInstabilityV2(programc.InstabilityRequest{Identity: id, DeclaredSeeds: []uint64{1, 2}, Runs: runs})
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(legacy)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("ASSERT_PROGRAM_C_NONEMPTY_BYTES_UNCHANGED got_sha=%s want_sha=%s err=%v", transitionDigest(got), transitionDigest(want), err)
	}
}

func TestProgramCTransitionReceiptMetadataAndCrashRecovery(t *testing.T) {
	raw := transitionPlan(t, records2()[:1])
	o, h := &transitionObjects{}, &transitionHeads{uncertainAt: 2}
	r := &transitionRunner{result: transitionResultBytes("STABLE")}
	req := transitionRequest(raw, o, h, r)
	first, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || first.Status != ProgramCTransitionIndeterminate {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	puts, calls := o.puts, len(r.seeds)
	req.Expected, _ = ParseProgramCTransitionHeadRevision(h.raw)
	h.uncertainAt = 0
	got, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || got.Status != ProgramCTransitionCommitted || o.puts != puts || len(r.seeds) != calls {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_RECEIPT_LINKED_NO_REPLAY got=%+v err=%v", got, err)
	}
	// Mutate declared metadata while preserving canonical JSON and object selector mapping.
	raw = transitionPlan(t, records2()[:1])
	o, h = &transitionObjects{}, &transitionHeads{uncertainAt: 2}
	r = &transitionRunner{result: transitionResultBytes("STABLE")}
	req = transitionRequest(raw, o, h, r)
	_, _ = CoordinateProgramCTransition(context.Background(), req)
	var receiptKey string
	for k, b := range o.m {
		var x map[string]any
		if json.Unmarshal(b, &x) == nil && x["instability_schema"] != nil {
			x["instability_schema"] = "lsp-trace.community-instability.v1"
			mut, _ := json.Marshal(x)
			delete(o.m, k)
			receiptKey = transitionDigest(mut)
			o.m[receiptKey] = mut
		}
	}
	var head programCTransitionHead
	_ = transitionStrict(h.raw, &head)
	head.ReceiptSelector = receiptKey
	h.raw, _ = transitionSelector(&head)
	req.Expected, _ = ParseProgramCTransitionHeadRevision(h.raw)
	if _, err = CoordinateProgramCTransition(context.Background(), req); !errors.Is(err, ErrInvalidProgramCTransition) {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_RECEIPT_METADATA_MUTATION err=%v", err)
	}
}

func TestProgramCTransitionPostComputeFailureRetainedWithoutReplay(t *testing.T) {
	raw := transitionPlan(t, records2()[:1])
	o, h := &transitionObjects{}, &transitionHeads{}
	r := &transitionRunner{instabilityFail: errors.New("downstream instability failed")}
	req := transitionRequest(raw, o, h, r)
	first, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || first.Status != ProgramCTransitionRetained || first.ProgramCOutcome != "COMPLETE" || first.FailureStage != ProgramCTransitionFailureInstabilityBuild || first.Failure != "downstream instability failed" || first.ResultSHA256 != "" || first.ResultByteLength != 0 || first.CommittedPrefix != 0 {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_POST_COMPUTE_FAILURE_RETAINED first=%+v err=%v", first, err)
	}
	puts, resolves, computes, cas := o.puts, len(r.constituents), len(r.seeds), h.cas
	// Reconstruct store handles over the same durable bytes to model close/reopen.
	reopenedObjects := &transitionObjects{m: o.m, puts: o.puts, gets: o.gets}
	reopenedHeads := &transitionHeads{raw: bytes.Clone(h.raw), cas: h.cas}
	req.Objects, req.Heads, req.Expected = reopenedObjects, reopenedHeads, first.Head
	second, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || second.Status != ProgramCTransitionRetained || second != first || reopenedObjects.puts != puts || len(r.constituents) != resolves || len(r.seeds) != computes || reopenedHeads.cas != cas {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_POST_COMPUTE_FAILURE_REOPEN_NO_REPLAY second=%+v first=%+v puts=%d/%d resolves=%d/%d computes=%d/%d cas=%d/%d err=%v", second, first, reopenedObjects.puts, puts, len(r.constituents), resolves, len(r.seeds), computes, reopenedHeads.cas, cas, err)
	}
}

func TestProgramCTransitionReceiptStageCombinationsReject(t *testing.T) {
	base := programCTransitionReceipt{Outcome: "FAILED", Failure: "x"}
	invalid := []programCTransitionReceipt{
		func() programCTransitionReceipt {
			x := base
			x.FailureStage = ProgramCTransitionFailureResolve
			x.ProgramCOutcome = "COMPLETE"
			return x
		}(),
		func() programCTransitionReceipt {
			x := base
			x.FailureStage = ProgramCTransitionFailureInstabilityBuild
			return x
		}(),
		func() programCTransitionReceipt {
			x := base
			x.FailureStage = ProgramCTransitionFailureInstabilityBuild
			x.ProgramCOutcome = "COMPLETE"
			x.ResultBytes = []byte("x")
			return x
		}(),
		{Outcome: "SUCCEEDED", ProgramCOutcome: "COMPLETE", FailureStage: ProgramCTransitionFailureInstabilityValidate, Failure: "x"},
	}
	for i, receipt := range invalid {
		if validTransitionReceiptOutcome(receipt) {
			t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_INVALID_STAGE_COMBINATION index=%d receipt=%+v", i, receipt)
		}
	}
}

func TestProgramCTransitionPlanMismatchAndFailureRetention(t *testing.T) {
	raw := transitionPlan(t, records2()[:1])
	o, h := &transitionObjects{}, &transitionHeads{}
	r := &transitionRunner{fail: errors.New("terminal")}
	req := transitionRequest(raw, o, h, r)
	got, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || got.Status != ProgramCTransitionRetained || got.CommittedPrefix != 0 {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_FAILURE_NO_ADVANCE got=%+v err=%v", got, err)
	}
	foreign := transitionPlan(t, []ProgramCTransitionPlanRecord{{Ordinal: 0, PublicationSelector: records2()[0].PublicationSelector, ConstituentSelector: records2()[1].ConstituentSelector}})
	req = transitionRequest(foreign, &transitionObjects{}, &transitionHeads{}, &transitionRunner{})
	req.Bindings.PlanSHA256 = transitionDigest(raw)
	if _, err = CoordinateProgramCTransition(context.Background(), req); !errors.Is(err, ErrInvalidProgramCTransition) {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_PLAN_DIGEST_FAIL_CLOSED err=%v", err)
	}
}

func TestVerifyProgramCTransitionCompletionLoadsHeadAndReceipt(t *testing.T) {
	planRaw := transitionPlan(t, records2())
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	first, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil || first.Status != ProgramCTransitionCommitted {
		t.Fatalf("ASSERT_PROGRAM_C_VERIFY_SETUP_FIRST result=%+v err=%v", first, err)
	}
	request.Expected = first.Head
	second, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil || second.Status != ProgramCTransitionCommitted {
		t.Fatalf("ASSERT_PROGRAM_C_VERIFY_SETUP_SECOND result=%+v err=%v", second, err)
	}
	verify := func() (ProgramCTransitionResult, error) {
		return verifyProgramCTransitionCompletion(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
	}
	verified, err := verify()
	if err != nil || verified.Status != ProgramCTransitionComplete || verified.Head != second.Head || verified.CommittedPrefix != 2 || verified.ConstituentCount != 2 || verified.ReceiptSelector == "" {
		t.Fatalf("ASSERT_PROGRAM_C_VERIFIED_COMPLETION result=%+v err=%v", verified, err)
	}
	_, inventory, err := verifyProgramCTransitionReceiptInventory(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner)
	if err != nil || len(inventory) != 2 {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_COMPLETE entries=%d err=%v", len(inventory), err)
	}
	for ordinal, entry := range inventory {
		if entry.ordinal != ordinal || entry.selector == "" || len(entry.canonicalReceipt) == 0 || entry.value.Ordinal != ordinal || !bytes.Equal(entry.nativeV5, []byte("native-v5")) || len(entry.outcomes) != 6 {
			t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_ENTRY ordinal=%d entry=%+v", ordinal, entry)
		}
	}

	delete(objects.m, verified.ReceiptSelector)
	if _, err = verify(); !errors.Is(err, ErrInvalidProgramCTransition) {
		t.Fatalf("ASSERT_PROGRAM_C_VERIFY_REQUIRES_RECEIPT err=%v", err)
	}
}

func TestVerifyProgramCTransitionCompletionRejectsMissingEarlierReceiptWithoutWrites(t *testing.T) {
	planRaw := transitionPlan(t, records2())
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	first, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil || first.Status != ProgramCTransitionCommitted || first.ReceiptSelector == "" {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_SETUP_FIRST result=%+v err=%v", first, err)
	}
	request.Expected = first.Head
	second, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil || second.Status != ProgramCTransitionCommitted {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_SETUP_SECOND result=%+v err=%v", second, err)
	}
	delete(objects.m, first.ReceiptSelector)
	puts, swaps := objects.puts, heads.cas
	_, err = verifyProgramCTransitionCompletion(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
	if !errors.Is(err, ErrInvalidProgramCTransition) {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_MISSING_EARLIER_REJECTED err=%v", err)
	}
	if objects.puts != puts || heads.cas != swaps {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_READ_ONLY puts=%d want=%d swaps=%d want=%d", objects.puts, puts, heads.cas, swaps)
	}
}

func TestVerifyProgramCTransitionCompletionRejectsTamperedEarlierReceipt(t *testing.T) {
	planRaw := transitionPlan(t, records2())
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	first, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Expected = first.Head
	if _, err = CoordinateProgramCTransition(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	objects.m[first.ReceiptSelector] = []byte(`{"malformed":true}`)
	puts, swaps := objects.puts, heads.cas
	_, err = verifyProgramCTransitionCompletion(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
	if !errors.Is(err, ErrInvalidProgramCTransition) || objects.puts != puts || heads.cas != swaps {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_TAMPERED_EARLIER_REJECTED err=%v puts=%d/%d swaps=%d/%d", err, objects.puts, puts, heads.cas, swaps)
	}
}

func TestVerifyProgramCTransitionCompletionRejectsDualHistoricalReceiptCandidates(t *testing.T) {
	planRaw := transitionPlan(t, records2())
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	first, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Expected = first.Head
	if _, err = CoordinateProgramCTransition(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var historical programCTransitionReceipt
	if err = transitionStrict(objects.m[first.ReceiptSelector], &historical); err != nil {
		t.Fatal(err)
	}
	historical.SchemaVersion = programCTransitionReceiptVersionV3
	historicalRaw, _ := json.Marshal(historical)
	objects.m[transitionDigest(historicalRaw)] = historicalRaw
	puts, swaps := objects.puts, heads.cas
	_, err = verifyProgramCTransitionCompletion(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
	if !errors.Is(err, ErrInvalidProgramCTransition) || objects.puts != puts || heads.cas != swaps {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_DUAL_CANDIDATE_REJECTED err=%v puts=%d/%d swaps=%d/%d", err, objects.puts, puts, heads.cas, swaps)
	}
}

func TestVerifyProgramCTransitionCompletionV3InstabilityProbesOnlyV4Receipt(t *testing.T) {
	planRaw := transitionPlan(t, records2()[:1])
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionV3IncompleteResultBytes()}
	request := transitionRequest(planRaw, objects, heads, runner)
	committed, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var invalidHistorical programCTransitionReceipt
	if err = transitionStrict(objects.m[committed.ReceiptSelector], &invalidHistorical); err != nil {
		t.Fatal(err)
	}
	invalidHistorical.SchemaVersion = programCTransitionReceiptVersionV3
	invalidHistoricalRaw, _ := json.Marshal(invalidHistorical)
	objects.m[transitionDigest(invalidHistoricalRaw)] = invalidHistoricalRaw
	puts, swaps, gets := objects.puts, heads.cas, objects.gets
	objects.onGet = func(selector string) {
		if selector != committed.ReceiptSelector {
			t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_V3_PROBED_NON_V4 selector=%s", selector)
		}
	}
	verified, err := verifyProgramCTransitionCompletion(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
	if err != nil || verified.Status != ProgramCTransitionComplete || objects.puts != puts || heads.cas != swaps || objects.gets != gets+1 {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_V3_ONLY_V4 result=%+v err=%v puts=%d/%d swaps=%d/%d gets=%d/%d", verified, err, objects.puts, puts, heads.cas, swaps, objects.gets, gets+1)
	}
}

func TestVerifyProgramCTransitionCompletionRejectsCandidateStorageErrorWithoutWrites(t *testing.T) {
	planRaw := transitionPlan(t, records2()[:1])
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	committed, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var historical programCTransitionReceipt
	if err = transitionStrict(objects.m[committed.ReceiptSelector], &historical); err != nil {
		t.Fatal(err)
	}
	historical.SchemaVersion = programCTransitionReceiptVersionV3
	historicalRaw, _ := json.Marshal(historical)
	fault := os.ErrPermission
	objects.getErr = map[string]error{transitionDigest(historicalRaw): fault}
	puts, swaps := objects.puts, heads.cas
	_, err = verifyProgramCTransitionCompletion(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
	if !errors.Is(err, fault) || objects.puts != puts || heads.cas != swaps {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_STORAGE_ERROR_FAILS_CLOSED err=%v puts=%d/%d swaps=%d/%d", err, objects.puts, puts, heads.cas, swaps)
	}
}

func TestVerifyProgramCTransitionCompletionPreservesCancellationWithoutWrites(t *testing.T) {
	for _, contextErr := range []error{context.Canceled, context.DeadlineExceeded} {
		name := strings.TrimPrefix(contextErr.Error(), "context ")
		t.Run(name+"/pre-load", func(t *testing.T) {
			objects, heads := &transitionObjects{}, &transitionHeads{}
			ctx := newTransitionFaultContext()
			ctx.fail(contextErr)
			source := &AuthoritativeProgramCTransitionPlanSource{Receipt: ExactProgramCTransitionInput{
				Reader:     transitionReaderFunc(func(ctx context.Context, _ int64) ([]byte, error) { return nil, ctx.Err() }),
				ByteLength: 1,
				SHA256:     transitionDigest([]byte("x")),
			}}
			_, err := VerifyProgramCTransitionCompletion(ctx, ProgramCTransitionVerificationRequest{PlanSource: source, Objects: objects, Heads: heads})
			if !errors.Is(err, contextErr) || objects.puts != 0 || heads.cas != 0 {
				t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_PRELOAD_CANCELLATION err=%v want=%v puts=%d swaps=%d", err, contextErr, objects.puts, heads.cas)
			}
		})

		t.Run(name+"/mid-recompute", func(t *testing.T) {
			planRaw := transitionPlan(t, records2()[:1])
			objects, heads := &transitionObjects{}, &transitionHeads{}
			setupRunner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
			request := transitionRequest(planRaw, objects, heads, setupRunner)
			if _, err := CoordinateProgramCTransition(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			ctx := newTransitionFaultContext()
			runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE"), onCompute: func() { ctx.fail(contextErr) }}
			objects.onGet = func(string) { t.Fatal("candidate probing occurred after mid-recompute cancellation") }
			puts, swaps := objects.puts, heads.cas
			_, err := verifyProgramCTransitionCompletion(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
			if !errors.Is(err, contextErr) || objects.puts != puts || heads.cas != swaps {
				t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_MID_RECOMPUTE_CANCELLATION err=%v want=%v puts=%d/%d swaps=%d/%d", err, contextErr, objects.puts, puts, heads.cas, swaps)
			}
		})

		t.Run(name+"/candidate-probe", func(t *testing.T) {
			planRaw := transitionPlan(t, records2()[:1])
			objects, heads := &transitionObjects{}, &transitionHeads{}
			runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
			request := transitionRequest(planRaw, objects, heads, runner)
			committed, err := CoordinateProgramCTransition(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			ctx := newTransitionFaultContext()
			gets := objects.gets
			objects.onGet = func(selector string) {
				if selector == committed.ReceiptSelector {
					t.Fatal("stored v4 candidate was probed after cancellation on historical-v3 candidate")
				}
				ctx.fail(contextErr)
			}
			puts, swaps := objects.puts, heads.cas
			_, err = verifyProgramCTransitionCompletion(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
			if !errors.Is(err, contextErr) || objects.puts != puts || heads.cas != swaps || objects.gets != gets+1 {
				t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_PROBE_CANCELLATION err=%v want=%v puts=%d/%d swaps=%d/%d gets=%d/%d", err, contextErr, objects.puts, puts, heads.cas, swaps, objects.gets, gets+1)
			}
		})
	}
}

func TestVerifyProgramCTransitionCompletionAcceptsHistoricalV3OnlyFullChain(t *testing.T) {
	planRaw := transitionPlan(t, records2())
	plan, err := decodeProgramCTransitionPlan(planRaw)
	if err != nil {
		t.Fatal(err)
	}
	objects, heads := &transitionObjects{m: map[string][]byte{}}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	bound := ProgramCTransitionRequest{WorkflowKey: request.WorkflowKey, HeadKey: request.HeadKey, Bindings: request.Bindings, Objects: objects, Runner: runner}
	reconstructed := programCTransitionHead{SchemaVersion: programCTransitionHeadVersion, WorkflowKey: request.WorkflowKey, HeadKey: request.HeadKey, Authority: 0, State: "ABSENT", Bindings: request.Bindings, ConstituentCount: len(plan.Records), ClaimOrdinal: -1}
	var terminalRaw []byte
	for _, record := range plan.Records {
		claim := reconstructed
		claim.Generation++
		claim.State = "CLAIMED"
		claim.ClaimOrdinal = record.Ordinal
		claim.ClaimGeneration = claim.Generation
		claim.ClaimSelector = reconstructed.Selector
		claim.ClaimPublicationSelector = record.PublicationSelector
		claim.ClaimConstituentSelector = record.ConstituentSelector
		claim.ReceiptSelector = ""
		if _, err = transitionSelector(&claim); err != nil {
			t.Fatal(err)
		}
		computed, computeErr := transitionComputeReceipt(context.Background(), bound, claim, record)
		if computeErr != nil {
			t.Fatal(computeErr)
		}
		historical := computed.receipt
		historical.SchemaVersion = programCTransitionReceiptVersionV3
		historicalRaw, marshalErr := json.Marshal(historical)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		historicalSelector := transitionDigest(historicalRaw)
		objects.m[historicalSelector] = historicalRaw
		linked := claim
		linked.Generation++
		linked.ReceiptSelector = historicalSelector
		if _, err = transitionSelector(&linked); err != nil {
			t.Fatal(err)
		}
		committed := linked
		committed.Generation++
		committed.State = "COMMITTED"
		committed.CommittedPrefix = record.Ordinal + 1
		terminalRaw, err = transitionSelector(&committed)
		if err != nil {
			t.Fatal(err)
		}
		reconstructed = committed
	}
	heads.raw = bytes.Clone(terminalRaw)
	puts, swaps := objects.puts, heads.cas
	verified, inventory, err := verifyProgramCTransitionReceiptInventory(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner)
	wantHead := censussourcecontinuation.HeadRevision{Generation: reconstructed.Generation, Selector: reconstructed.Selector}
	if err != nil || verified.Status != ProgramCTransitionComplete || verified.Head != wantHead || len(inventory) != len(plan.Records) || !bytes.Equal(heads.raw, terminalRaw) {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_HISTORICAL_V3_FULL_CHAIN result=%+v want_head=%+v entries=%d terminal_equal=%t err=%v", verified, wantHead, len(inventory), bytes.Equal(heads.raw, terminalRaw), err)
	}
	for ordinal, entry := range inventory {
		if entry.ordinal != ordinal || entry.value.SchemaVersion != programCTransitionReceiptVersionV3 {
			t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_HISTORICAL_V3_ENTRY ordinal=%d entry=%+v", ordinal, entry)
		}
	}
	if objects.puts != puts || heads.cas != swaps {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_HISTORICAL_V3_READ_ONLY puts=%d/%d swaps=%d/%d", objects.puts, puts, heads.cas, swaps)
	}
}

func TestVerifyProgramCTransitionCompletionRejectsProductionTestSeam(t *testing.T) {
	source := &AuthoritativeProgramCTransitionPlanSource{VerifyPublication: func(string, captureset.ExactBytesAuthority) (captureset.Manifest, error) {
		return captureset.Manifest{}, nil
	}}
	_, err := VerifyProgramCTransitionCompletion(context.Background(), ProgramCTransitionVerificationRequest{PlanSource: source})
	if !errors.Is(err, ErrInvalidProgramCTransition) {
		t.Fatalf("ASSERT_PROGRAM_C_VERIFY_PRODUCTION_SEAM_REJECTED err=%v", err)
	}
}

func TestVerifyProgramCTransitionCompletionRejectsFinalHeadMismatch(t *testing.T) {
	planRaw := transitionPlan(t, records2())
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	first, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Expected = first.Head
	if _, err = CoordinateProgramCTransition(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var terminal programCTransitionHead
	if err = transitionStrict(heads.raw, &terminal); err != nil {
		t.Fatal(err)
	}
	terminal.ReceiptSelector = first.ReceiptSelector
	heads.raw, _ = transitionSelector(&terminal)
	puts, swaps := objects.puts, heads.cas
	_, err = verifyProgramCTransitionCompletion(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, request.PlanSource, objects, heads, runner)
	if !errors.Is(err, ErrInvalidProgramCTransition) || objects.puts != puts || heads.cas != swaps {
		t.Fatalf("ASSERT_PROGRAM_C_RECEIPT_INVENTORY_FINAL_HEAD_MISMATCH_REJECTED err=%v puts=%d/%d swaps=%d/%d", err, objects.puts, puts, heads.cas, swaps)
	}
}

func TestProgramCTransitionRealPublicationBackedRunner(t *testing.T) {
	native := func(id string) []byte {
		b, _ := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Provenance: graph.InvocationProvenance{InvocationID: id, SourceRevision: "rev", ServerVersion: "v"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}})
		raw, err := graphprovenance.CaptureV5(b, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	exact := [][]byte{native("a"), native("b")}
	authority := captureset.NativeV5Authority()
	cs := make([]captureset.Constituent, 2)
	for i := range exact {
		var err error
		cs[i], err = authority.Constituent(exact[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	ledger := captureset.Ledger{Denominator: 2, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "a", Disposition: "processed"}, {Ordinal: 1, Identity: "b", Disposition: "processed"}}}
	targets := make([]captureset.Target, 64)
	for i := range targets {
		seed := fmt.Sprintf("seed-%d", i)
		targets[i] = captureset.Target{CensusOrdinal: i, CanonicalSeedV2: seed, CanonicalSeedV2SHA256: transitionDigest([]byte(seed))}
	}
	manifest, err := captureset.Prepare(targets, cs, ledger, ledger, "census", "duplicates")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = captureset.AssociateBatches(manifest, cs)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publisher := captureset.NewPublisher(root)
	published := publisher.PublishCaptureSet(manifest, exact, authority)
	if published.Err != nil {
		t.Fatal(published.Err)
	}
	source := &AuthoritativeProgramCTransitionPlanSource{resolver: &programCPublicationResolver{bySelector: map[string]programCVerifiedPublication{published.Receipt.Selector: {rootPath: dir, selector: published.Receipt.Selector, binaryDigest: published.Receipt.ArtifactSHA256, binaryLength: int64(published.Receipt.ByteLength), manifest: manifest}}}}
	runner := &publicationTransitionRunner{transitionRunner: transitionRunner{result: transitionResultBytes("STABLE")}, production: NewProgramCProductionRunner(source)}
	records := make([]ProgramCTransitionPlanRecord, len(cs))
	for i, c := range cs {
		records[i] = ProgramCTransitionPlanRecord{Ordinal: i, PublicationSelector: published.Receipt.Selector, ConstituentSelector: c.ImmutableSelector}
	}
	planRaw := transitionPlan(t, records)
	o, h := &transitionObjects{}, &transitionHeads{}
	req := transitionRequest(planRaw, o, h, runner)
	first, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || first.Ordinal != 0 || first.CommittedPrefix != 1 {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_REAL_PUBLICATION_ORDINAL_ZERO first=%+v err=%v", first, err)
	}
	req.Expected = first.Head
	second, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || second.Ordinal != 1 || second.CommittedPrefix != 2 || !reflect.DeepEqual(runner.constituents, []string{cs[0].ImmutableSelector, cs[1].ImmutableSelector}) {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_REAL_PUBLICATION_N_PLUS_ONE second=%+v selectors=%v err=%v", second, runner.constituents, err)
	}
}

func TestProgramCTransitionClosedSchemas(t *testing.T) {
	for _, v := range []any{ProgramCTransitionBindings{}, ProgramCTransitionPlan{}, programCTransitionHead{}, programCTransitionReceipt{}} {
		b, _ := json.Marshal(v)
		lower := strings.ToLower(string(b))
		for _, word := range []string{"semantic", "candidate", "nomination", "describe", "grouping", "cross_partition", "processed_set"} {
			if strings.Contains(lower, word) {
				t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_NO_SEMANTIC_FIELDS word=%s json=%s", word, b)
			}
		}
	}
}
