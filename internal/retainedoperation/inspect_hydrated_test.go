package retainedoperation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedinspection"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojectionv2"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

type fixture struct {
	raw     []byte
	content []byte
	keys    []retainedinspection.Key
}

type countingLookup struct {
	calls int
	id    sourceobject.Identity
	bytes []byte
	err   error
}

func (l *countingLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	l.calls++
	if l.err != nil {
		return sourceobject.Object{}, l.err
	}
	return sourceobject.Object{Identity: l.id, Bytes: append([]byte(nil), l.bytes...)}, nil
}

func TestProjectionInlineSuccessSchemaParityBodiesAndDedup(t *testing.T) {
	fx := validFixture(t, 2)
	for _, body := range []string{"OMIT", "INCLUDE"} {
		t.Run(body, func(t *testing.T) {
			lookup := &countingLookup{id: identity(fx.content), bytes: fx.content}
			request := projectionRequest(fx, body)
			result, failure := execute(t, request, lookup, operation.Request{})
			if failure != nil {
				t.Fatalf("ASSERT_OPERATION_INLINE_SUCCESS_%s: %v", body, failure)
			}
			if result.ArtifactSchemaID != retainedinspection.SourceProjectionSchemaID {
				t.Fatalf("ASSERT_OPERATION_RESULT_SCHEMA_ID: %q", result.ArtifactSchemaID)
			}
			if !bytes.HasSuffix(result.Artifact, []byte("\n")) || bytes.HasSuffix(result.Artifact, []byte("\n\n")) {
				t.Fatalf("ASSERT_OPERATION_SINGLE_TRAILING_NEWLINE: %q", result.Artifact)
			}
			var artifact any
			if err := json.Unmarshal(bytes.TrimSuffix(result.Artifact, []byte("\n")), &artifact); err != nil {
				t.Fatal(err)
			}
			valueRaw, err := json.Marshal(result.Value)
			if err != nil {
				t.Fatal(err)
			}
			var value any
			if err := json.Unmarshal(valueRaw, &value); err != nil || !reflect.DeepEqual(value, artifact) {
				t.Fatalf("ASSERT_OPERATION_VALUE_ARTIFACT_PARITY: err=%v equal=%v", err, reflect.DeepEqual(value, artifact))
			}
			validateProjectionSchema(t, bytes.TrimSuffix(result.Artifact, []byte("\n")))
			wire := result.Value.(sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding])
			if len(wire.Units) != 2 || lookup.calls != 1 {
				t.Fatalf("ASSERT_OPERATION_DISTINCT_IDENTITY_ONCE: units=%d calls=%d", len(wire.Units), lookup.calls)
			}
			for _, unit := range wire.Units {
				if body == "INCLUDE" && (unit.BodyDisposition != "RETURNED" || unit.Body == "") {
					t.Fatalf("ASSERT_OPERATION_BODY_INCLUDE: %+v", unit)
				}
				if body == "OMIT" && (unit.BodyDisposition != "NOT_REQUESTED" || unit.Body != "") {
					t.Fatalf("ASSERT_OPERATION_BODY_OMIT: %+v", unit)
				}
			}
		})
	}
}

func TestProjectionPublicationAndContentIngress(t *testing.T) {
	fx := validFixture(t, 1)
	lookup := func() *countingLookup { return &countingLookup{id: identity(fx.content), bytes: fx.content} }
	t.Run("publication", func(t *testing.T) {
		root, err := publication.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		published := publication.NewPublisher().PublishVerifiedGeneration(root, fx.raw, retainedinspection.SourceSnapshotSchemaID)
		if published.Failure != nil {
			t.Fatal(published.Failure)
		}
		p := published.Receipt
		r := projectionRequest(fx, "OMIT")
		r.RetainedSourceEvidence = retainedinspection.Evidence{PublicationSnapshotV2: &retainedinspection.PublicationSnapshot{Selector: p.VerificationSelector, ArtifactDigest: p.Digest, ArtifactByteLength: p.ByteLength, ArtifactSchemaID: p.ArtifactSchemaID, PublicationMechanism: p.PublicationMechanism, Generation: p.Generation, VerificationSelector: p.VerificationSelector}}
		result, failure := execute(t, r, lookup(), operation.Request{PublicationRoot: root})
		if failure != nil || result.ArtifactSchemaID != retainedinspection.SourceProjectionSchemaID {
			t.Fatalf("ASSERT_OPERATION_PUBLICATION_SUCCESS: result=%+v failure=%v", result, failure)
		}
		for _, mutate := range []func(*retainedinspection.PublicationSnapshot){
			func(v *retainedinspection.PublicationSnapshot) { v.ArtifactDigest = digest([]byte("wrong")) },
			func(v *retainedinspection.PublicationSnapshot) { v.ArtifactByteLength++ },
			func(v *retainedinspection.PublicationSnapshot) { v.Generation = "g-" + strings.Repeat("0", 64) },
		} {
			bad := r
			copyValue := *r.RetainedSourceEvidence.PublicationSnapshotV2
			mutate(&copyValue)
			bad.RetainedSourceEvidence.PublicationSnapshotV2 = &copyValue
			result, failure := execute(t, bad, lookup(), operation.Request{PublicationRoot: root})
			assertTypedFailure(t, result, failure, "INGRESS", "ADMISSION_FAILED")
		}
	})
	t.Run("content", func(t *testing.T) {
		root, err := publication.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		id := digest(fx.raw)
		generation := "g-" + strings.TrimPrefix(id, "sha256:")
		published := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: strings.TrimPrefix(id, "sha256:"), Bytes: fx.raw, ArtifactSchemaID: retainedinspection.SourceSnapshotSchemaID})
		if published.Failure != nil {
			t.Fatal(published.Failure)
		}
		r := projectionRequest(fx, "OMIT")
		r.RetainedSourceEvidence = retainedinspection.Evidence{ContentAddressedSnapshotV2: &retainedinspection.ContentAddressedSnapshot{ID: id, ArtifactByteLength: uint64(len(fx.raw)), ArtifactSchemaID: retainedinspection.SourceSnapshotSchemaID, Generation: generation}}
		result, failure := execute(t, r, lookup(), operation.Request{ArtifactStore: root})
		if failure != nil || result.ArtifactSchemaID != retainedinspection.SourceProjectionSchemaID {
			t.Fatalf("ASSERT_OPERATION_CONTENT_SUCCESS: result=%+v failure=%v", result, failure)
		}
	})
	if err := ValidateIngress("wrong", fx.raw); err == nil {
		t.Fatal("ASSERT_OPERATION_WRONG_SCHEMA_REJECT")
	}
	if err := ValidateIngress(retainedinspection.SourceSnapshotSchemaID, []byte(`{}`)); err != nil {
		t.Fatalf("ASSERT_OPERATION_SCHEMA_ONLY_INGRESS: %v", err)
	}
}

func TestProjectionPipelineFailureMappingsAndSafety(t *testing.T) {
	fx := validFixture(t, 1)
	ingressBytes, ingressFailure := ingress(operation.Request{}, retainedinspection.Evidence{})
	if ingressBytes != nil {
		t.Fatalf("ASSERT_OPERATION_NO_CARRIER_ZERO_BYTES: %q", ingressBytes)
	}
	assertTypedFailure(t, operation.Result{}, ingressFailure, "INGRESS", "ADMISSION_FAILED")

	ingressBytes, ingressFailure = ingress(operation.Request{}, retainedinspection.Evidence{ContentAddressedSnapshotV2: &retainedinspection.ContentAddressedSnapshot{ArtifactSchemaID: "wrong"}})
	if ingressBytes != nil {
		t.Fatalf("ASSERT_OPERATION_WRONG_SCHEMA_ZERO_BYTES: %q", ingressBytes)
	}
	assertTypedFailure(t, operation.Result{}, ingressFailure, "INGRESS", "ADMISSION_FAILED")
	r := projectionRequest(fx, "OMIT")
	result, failure := execute(t, r, nil, operation.Request{})
	assertTypedFailure(t, result, failure, "RESOLVE", "POLICY")

	bad := r
	bad.RetainedSourceEvidence.InlineSnapshotV2 = `{}`
	lookup := &countingLookup{}
	result, failure = execute(t, bad, lookup, operation.Request{})
	assertTypedFailure(t, result, failure, "ADMIT", "ADMISSION_FAILED")
	if lookup.calls != 0 {
		t.Fatalf("ASSERT_OPERATION_NO_LOOKUP_BEFORE_ADMIT: %d", lookup.calls)
	}

	missing := r
	missing.Selection.Target.GraphSubjectID = "missing"
	missing.Selection.Selections[0] = missing.Selection.Target
	lookup = &countingLookup{}
	result, failure = execute(t, missing, lookup, operation.Request{})
	assertTypedFailure(t, result, failure, "SELECT", "MISSING_SUBJECT_BINDING")
	if lookup.calls != 0 {
		t.Fatalf("ASSERT_OPERATION_NO_LOOKUP_BEFORE_SELECT: %d", lookup.calls)
	}

	states := retainedinspection.ErrorStates()
	allowed := map[string]bool{}
	for _, state := range states {
		allowed[state] = true
	}
	for _, code := range []sourceobject.Code{sourceobject.CodeInvalidIdentity, sourceobject.CodeMissing, sourceobject.CodeCorrupt, sourceobject.CodePolicy, sourceobject.CodePermission, sourceobject.CodeLimit} {
		result, failure := classify("RESOLVE", &sourceobject.Error{Code: code})
		assertTypedFailure(t, result, failure, "RESOLVE", string(code))
		if !allowed[string(code)] {
			t.Fatalf("ASSERT_OPERATION_STATE_VOCABULARY: %s", code)
		}
	}
	for _, code := range []retainedprojection.Code{retainedprojection.CodeInvalidPlan, retainedprojection.CodeReturnedIdentityMismatch, retainedprojection.CodeResolvedLengthMismatch, retainedprojection.CodeResolvedDigestMismatch, retainedprojection.CodeResolveDistinctLimit, retainedprojection.CodeResolveSourceBytesLimit, retainedprojection.CodeResolveSelectionLimit} {
		result, failure := classify("RESOLVE", &retainedprojection.Error{Code: code})
		assertTypedFailure(t, result, failure, "RESOLVE", string(code))
		if !allowed[string(code)] {
			t.Fatalf("ASSERT_OPERATION_STATE_VOCABULARY: %s", code)
		}
	}
	for _, tc := range []struct {
		code  retainedprojection.Code
		phase string
	}{{retainedprojection.CodeProjectionFailed, "PROJECT"}, {retainedprojection.CodeAssemblyFailed, "ASSEMBLE"}, {retainedprojection.CodeInvalidResolveResult, "ASSEMBLE"}} {
		result, failure := classifyAssembly(&retainedprojection.AssemblyError{Code: tc.code})
		assertTypedFailure(t, result, failure, tc.phase, string(tc.code))
	}
	result, failure = classify("PROJECT", &retainedprojection.Error{Code: retainedprojection.CodeInvalidPlan})
	assertTypedFailure(t, result, failure, "PROJECT", "INVALID_PLAN")

	workLimited := projectionRequest(fx, "OMIT")
	workLimited.Projection.Limits.MaxWork = 0
	result, failure = execute(t, workLimited, &countingLookup{id: identity(fx.content), bytes: fx.content}, operation.Request{})
	assertTypedFailure(t, result, failure, "PROJECT", "PROJECTION_FAILED")
}

func TestProjectionResolveAndResponseBoundaries(t *testing.T) {
	fx := validFixture(t, 1)
	base := projectionRequest(fx, "OMIT")
	id := identity(fx.content)
	cases := []struct {
		name, state string
		mutate      func(*retainedinspection.Request)
		lookup      *countingLookup
	}{
		{"bytes", "RESOLVE_UNIQUE_SOURCE_BYTES_LIMIT", func(r *retainedinspection.Request) { r.ResolveLimits.MaxUniqueSourceBytes = 1 }, &countingLookup{id: id, bytes: fx.content}},
		{"identity", "RETURNED_IDENTITY_MISMATCH", func(*retainedinspection.Request) {}, &countingLookup{id: sourceobject.Identity{Digest: digest([]byte("x")), ByteLength: 1}, bytes: []byte("x")}},
		{"length", "RESOLVED_LENGTH_MISMATCH", func(*retainedinspection.Request) {}, &countingLookup{id: id, bytes: fx.content[:len(fx.content)-1]}},
		{"digest", "RESOLVED_DIGEST_MISMATCH", func(*retainedinspection.Request) {}, &countingLookup{id: id, bytes: append([]byte(nil), fx.content...)}},
	}
	cases[len(cases)-1].lookup.bytes[0] ^= 1
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.mutate(&r)
			result, failure := execute(t, r, tc.lookup, operation.Request{})
			assertTypedFailure(t, result, failure, "RESOLVE", tc.state)
		})
	}

	lookup := &countingLookup{id: id, bytes: fx.content}
	full, failure := execute(t, base, lookup, operation.Request{})
	if failure != nil {
		t.Fatal(failure)
	}
	exact := base
	exact.Projection.Limits.MaxResponseBytes = uint64(len(bytes.TrimSuffix(full.Artifact, []byte("\n"))))
	if _, failure = execute(t, exact, &countingLookup{id: id, bytes: fx.content}, operation.Request{}); failure != nil {
		t.Fatalf("ASSERT_OPERATION_RESPONSE_EXACT_BOUNDARY: %v", failure)
	}
	exact.Projection.Limits.MaxResponseBytes--
	result, failure := execute(t, exact, &countingLookup{id: id, bytes: fx.content}, operation.Request{})
	assertTypedFailure(t, result, failure, "ASSEMBLE", "ASSEMBLY_FAILED")
}

func TestConstructedHandlerPreservesRepresentativeLegacyResults(t *testing.T) {
	requests := []operation.Request{
		{Input: json.RawMessage(`{"input":"{\"schema_version\":\"lsp-trace.graph.v1\",\"nodes\":[],\"relations\":[]}","node_ids":[]}`)},
		{Input: json.RawMessage(`{"input":"{}","node_ids":["missing"],"page":true}`)},
		{Input: json.RawMessage(`{"input":"{}","node_ids":["missing"],"cursor":"bad"}`)},
	}
	for i, request := range requests {
		want, wantFailure := operation.InspectHydratedHandler(context.Background(), request)
		got, gotFailure := NewInspectHydratedHandler(nil)(context.Background(), request)
		if !reflect.DeepEqual(got, want) || !equalFailure(gotFailure, wantFailure) {
			t.Fatalf("ASSERT_OPERATION_LEGACY_EQUAL_%d: got=%+v/%v want=%+v/%v", i, got, gotFailure, want, wantFailure)
		}
		if got.ArtifactSchemaID != "" {
			t.Fatalf("ASSERT_OPERATION_LEGACY_SCHEMA_ZERO_%d: %q", i, got.ArtifactSchemaID)
		}
	}
}

func projectionRequest(fx fixture, body string) retainedinspection.Request {
	return retainedinspection.Request{Mode: retainedinspection.Mode, RetainedSourceEvidence: retainedinspection.Evidence{InlineSnapshotV2: string(fx.raw)}, Selection: retainedinspection.Selection{Target: fx.keys[0], Selections: append([]retainedinspection.Key(nil), fx.keys...)}, Projection: retainedinspection.Projection{Body: body, PrivacyPolicyID: digest([]byte("policy")), Limits: retainedinspection.ProjectionLimits{MaxSourceBytes: 1 << 20, MaxRanges: 32, MaxObjects: 32, MaxWork: 32, MaxResponseBytes: 1 << 20}}, ResolveLimits: retainedinspection.ResolveLimits{MaxDistinctObjects: 32, MaxUniqueSourceBytes: 1 << 20, MaxLogicalSelections: 32}}
}

func execute(t *testing.T, request retainedinspection.Request, lookup retainedprojection.Lookup, base operation.Request) (operation.Result, *operation.Failure) {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	base.Input = raw
	return NewInspectHydratedHandler(lookup)(context.Background(), base)
}

func assertTypedFailure(t *testing.T, result operation.Result, failure *operation.Failure, phase, state string) {
	t.Helper()
	if failure == nil || failure.Code != state || !reflect.DeepEqual(result, operation.Result{}) {
		t.Fatalf("ASSERT_OPERATION_TYPED_ATOMIC_FAILURE: result=%+v failure=%v", result, failure)
	}
	var typed *Failure
	if !errors.As(failure, &typed) || typed.Phase != phase || typed.State != state {
		t.Fatalf("ASSERT_OPERATION_TYPED_ATOMIC_FAILURE: typed=%+v want=%s/%s", typed, phase, state)
	}
	allowed := false
	for _, candidate := range retainedinspection.ErrorStates() {
		allowed = allowed || candidate == state
	}
	if !allowed {
		t.Fatalf("ASSERT_OPERATION_FICTIONAL_STATE: %s", state)
	}
	for _, d := range failure.Diagnostics {
		if len(d) > 1024 || strings.Contains(d, "/") || strings.Contains(d, "file:") || strings.Contains(d, "selector") || strings.Contains(d, "package a") || strings.Contains(d, "operation not permitted") {
			t.Fatalf("ASSERT_OPERATION_SAFE_DIAGNOSTIC: %q", d)
		}
	}
}

func equalFailure(a, b *operation.Failure) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Code == b.Code && reflect.DeepEqual(a.Diagnostics, b.Diagnostics) && a.Error() == b.Error()
}

func validateProjectionSchema(t *testing.T, raw []byte) {
	t.Helper()
	schemaRaw, err := os.ReadFile("../schema/schemas/lsp-trace.source-projection.v2.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaRaw))
	if err != nil {
		t.Fatal(err)
	}
	if err = compiler.AddResource(retainedinspection.SourceProjectionSchemaID, doc); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(retainedinspection.SourceProjectionSchemaID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err = compiled.Validate(value); err != nil {
		t.Fatalf("ASSERT_OPERATION_PROJECTION_SCHEMA: %v", err)
	}
}

func validFixture(t *testing.T, count int) fixture {
	t.Helper()
	root := t.TempDir()
	content := []byte("package a\nfunc A() {}\n")
	zero := uint32(0)
	nodes := make([]graph.Node, 0, count)
	siblings := make([]graph.SiblingCandidate, 0, count)
	invocationSeeds := make([]graph.InvocationSeed, 0, count)
	seedResults := make([]graph.SeedResult, 0, count)
	uris := make([]string, 0, count)
	for i := 0; i < count; i++ {
		path := filepath.Join(root, string(rune('a'+i))+".go")
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
		uri := (&url.URL{Scheme: "file", Path: path}).String()
		uris = append(uris, uri)
		node := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Line: 1, Character: 11}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}})
		origin := graph.NewNode(graph.Item{Name: "Origin", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
		declaration := node
		label := fmt.Sprintf("seed-%d", i)
		at := fmt.Sprintf("%c.go:1:1", 'a'+i)
		nodes = append(nodes, node)
		invocationSeeds = append(invocationSeeds, graph.InvocationSeed{Label: label, At: at, ResolvedURI: uri, ContentSHA256: digest(content), LanguageID: "go"})
		seedResults = append(seedResults, graph.SeedResult{Label: label, ReachedNodeIDs: []string{node.ID}})
		siblings = append(siblings, graph.SiblingCandidate{SeedURI: uri, SeedLabel: label, SeedLabels: []string{label}, SeedIdentity: "session:" + label + ":" + at, Origin: origin, Declaration: &declaration, Candidate: node, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + digest(content), "origin=" + digest(content)}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: digest(content), ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}})
	}
	target := acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: uris[0], Line: &zero, Character: &zero}, DownDepth: 1, UpDepth: 1}
	req := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "context", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}, Root: target, Limits: acquisition.Limits{MaxNodes: 100, MaxRequests: 10, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 16}}
	client := acquisition.NewWireClient(func(_ context.Context, wire acquisition.WireRequest) (json.RawMessage, error) {
		if wire.Method == "textDocument/prepareCallHierarchy" {
			return json.Marshal([]lsp.CallHierarchyItem{{Name: "A", Kind: 12, URI: uris[0], Range: lsp.Range{End: lsp.Position{Line: 1, Character: 11}}, SelectionRange: lsp.Range{End: lsp.Position{Line: 1, Character: 6}}}})
		}
		return json.RawMessage(`[]`), nil
	})
	acquired, err := acquisition.Acquire(context.Background(), client, req)
	if err != nil {
		t.Fatal(err)
	}
	v2raw, err := graphprovenance.CaptureV2(context.Background(), acquired, root)
	if err != nil {
		t.Fatal(err)
	}
	var v2 graphprovenance.EvidenceV2
	if err = json.Unmarshal(v2raw, &v2); err != nil {
		t.Fatal(err)
	}
	g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: invocationSeeds, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Nodes: nodes, Seeds: seedResults, Summary: graph.Summary{NodeCount: len(nodes), Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}, SiblingCandidates: siblings}
	native, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	v5, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &v2)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := v5sourcesnapshot.Build(v5, root, "utf-16")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot v5sourcesnapshot.Artifact
	if err = json.Unmarshal(parent, &snapshot); err != nil {
		t.Fatal(err)
	}
	displays := make([]v5sourcesnapshotv2.DisplayBinding, 0, count)
	keys := make([]retainedinspection.Key, 0, count)
	seen := map[retainedinspection.Key]bool{}
	for _, binding := range snapshot.Bindings {
		key := retainedinspection.Key{GraphSubjectID: binding.NodeID, LogicalSourceID: binding.URI}
		if seen[key] || len(binding.ReceiptIDs) == 0 {
			continue
		}
		seen[key] = true
		displays = append(displays, v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: binding.NodeID, LogicalSourceID: binding.URI, DisplayRange: binding.Range, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: binding.ReceiptIDs[0], SourceDigest: binding.SourceDigest, PositionEncoding: snapshot.PositionEncoding, Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody})
		keys = append(keys, key)
		if len(keys) == count {
			break
		}
	}
	sort.Slice(displays, func(i, j int) bool {
		if displays[i].GraphSubjectID != displays[j].GraphSubjectID {
			return displays[i].GraphSubjectID < displays[j].GraphSubjectID
		}
		return displays[i].LogicalSourceID < displays[j].LogicalSourceID
	})
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].GraphSubjectID != keys[j].GraphSubjectID {
			return keys[i].GraphSubjectID < keys[j].GraphSubjectID
		}
		return keys[i].LogicalSourceID < keys[j].LogicalSourceID
	})
	artifact := v5sourcesnapshotv2.Artifact{SchemaVersion: v5sourcesnapshotv2.Version, Policy: v5sourcesnapshotv2.Policy, ParentSchemaVersion: v5sourcesnapshot.Version, ParentSnapshotDigest: digest(parent), ParentSnapshot: parent, DisplayBindings: displays}
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = v5sourcesnapshotv2.Validate(raw); err != nil {
		t.Fatalf("fixture invalid: %v", err)
	}
	if len(keys) != count {
		t.Fatalf("fixture keys=%d want=%d", len(keys), count)
	}
	return fixture{raw: raw, content: content, keys: keys}
}

func identity(raw []byte) sourceobject.Identity {
	return sourceobject.Identity{Digest: digest(raw), ByteLength: uint64(len(raw))}
}
func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
