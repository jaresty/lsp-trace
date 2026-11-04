package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
)

// The same controlled CALLS composition shape as referencegrouping's accepted
// referenceCapture fixture. It is supplied existing admission, NOT a fresh
// CALLS observation or a query:true bootstrap for the new managed D/R path.
func groupingCallsFixture(t *testing.T) (programcadmission.CompositeProjectionAdmission, []byte) {
	t.Helper()
	r := graph.Range{End: graph.Position{Character: 1}}
	a := graph.NewNode(graph.Item{Name: "a", Kind: 12, URI: "file:///w/a.go", Range: r, SelectionRange: r})
	r.Start.Line = 1
	r.End.Line = 1
	b := graph.NewNode(graph.Item{Name: "b", Kind: 12, URI: "file:///w/a.go", Range: r, SelectionRange: r})
	capture := func(identity string, nodes []graph.Node, edges []graph.Edge) programccompose.Input {
		seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
		result := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "invocation", SourceRevision: "commit", ServerVersion: "gopls@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Summary: graph.Summary{Complete: false}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
		native, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
		if err != nil {
			t.Fatal(err)
		}
		return programccompose.Input{Bytes: raw, Identity: identity, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), ByteLength: len(raw), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: "workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-test-v1"}}
	}
	input := capture("capture", []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}}, {CallerNodeID: b.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}}}})
	empty := capture("capture-empty", nil, nil)
	composite, err := programccompose.Compose([]programccompose.Input{input, empty})
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return admitted.Admission, composite.Bytes
}
func TestPrivateManagedGroupingMixedZeroMethodsRefusesWithoutDroppingCalls(t *testing.T) {
	calls, _ := groupingCallsFixture(t)
	m, q, _ := groupingFixture(t, false, false)
	q.Calls = calls
	q.Kinds = append(q.Kinds, programcadmission.TypedCalls)
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	if err == nil || got.Input.ByteLength() != 0 {
		t.Fatal("ASSERT_PRIVATE_MIXED_ZERO_METHODS_NOT_SILENTLY_DROPPED")
	}
}

func TestPrivateManagedGroupingMixedExistingCalls(t *testing.T) {
	calls, original := groupingCallsFixture(t)
	m, q, _ := groupingFixture(t, true, true)
	q.Calls = calls
	q.Kinds = append(q.Kinds, programcadmission.TypedCalls)
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Input.Contributions()) != 6 || len(got.Ledger) != 4 {
		t.Fatal("ASSERT_PRIVATE_MIXED_KIND_MULTIPLICITY")
	}
	found := false
	for _, p := range got.Predecessors {
		if p.Role == "RETAINED_COMPOSITE" {
			found = bytes.Equal(p.Bytes, original)
		}
	}
	if !found {
		t.Fatal("ASSERT_PRIVATE_MIXED_ORIGINAL_CALLS_BYTES")
	}
	for _, c := range calls.Occurrences() {
		matched := false
		for _, o := range got.Input.Contributions() {
			if o.OccurrenceIdentity == c.Identity && o.Kind == "CALLS" && o.Ordinal == c.Ordinal {
				matched = true
			}
		}
		if !matched {
			t.Fatal("ASSERT_PRIVATE_MIXED_NO_CALLS_RELABEL")
		}
	}
	if err := got.Replay(q.Root); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_PRIVATE_MIXED_ORIGINAL_CALLS_BYTES: PASS")
}
