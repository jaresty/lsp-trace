package censuscontinuation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/targetpacket"
)

type retainedDiagnosticStore struct{ root string }

func (retainedDiagnosticStore) Put(context.Context, []byte) (string, error) {
	return "", errors.New("read-only diagnostic store")
}

func (s retainedDiagnosticStore) Get(_ context.Context, selector string) ([]byte, error) {
	const prefix = "sha256:"
	if !strings.HasPrefix(selector, prefix) {
		return nil, errors.New("invalid diagnostic selector")
	}
	return os.ReadFile(filepath.Join(s.root, strings.TrimPrefix(selector, prefix)))
}

func classifyRetainedCaptureFailure(err error) string {
	if err == nil {
		return "NONE"
	}
	message := err.Error()
	for _, candidate := range []struct {
		needle string
		code   string
	}{
		{"explicit workspace differs", "WORKSPACE_COMPARE"},
		{"position encoding differs", "POSITION_ENCODING"},
		{"Program C constituent set mismatch", "CONSTITUENT_SET"},
		{"constituent graph commitment mismatch", "CONSTITUENT_COMMITMENT"},
		{"duplicate graph node", "DUPLICATE_GRAPH_NODE"},
		{"foreign representative", "ENDPOINT_DERIVATION"},
		{"unknown target", "ENDPOINT_DERIVATION"},
		{"file uri", "EXTERNAL_URI"},
		{"outside workspace", "EXTERNAL_URI"},
		{"occurrence substitution", "SELECTED_RELATION_OCCURRENCE"},
		{"duplicate nomination", "DUPLICATE_NOMINATION"},
		{"limit", "LIMITS"},
		{"artifact limit", "LIMITS_ARTIFACT"},
		{"total source limit", "LIMITS_TOTAL_SOURCE"},
		{"source object missing", "REPLAY_OBJECT_MISSING"},
		{"source substitution", "SOURCE_SUBSTITUTION"},
		{"receipt", "VALIDATION_RECEIPT"},
		{"endpoint binding", "VALIDATION_ENDPOINT_BINDING"},
		{"v5 outcome", "VALIDATION_OUTCOME"},
		{"v5 identity", "VALIDATION_IDENTITY"},
		{"graph scalar substitution", "VALIDATION_GRAPH_SCALAR"},
		{"schema", "SCHEMA_VALIDATION"},
		{"duplicate derived endpoint", "CLOSURE_DUPLICATE_DERIVED_ENDPOINT"},
		{"missing endpoint outcome", "CLOSURE_MISSING_ENDPOINT_OUTCOME"},
		{"substituted endpoint outcome", "CLOSURE_SUBSTITUTED_ENDPOINT_OUTCOME"},
		{"endpoint outcome integrity", "CLOSURE_OUTCOME_INTEGRITY"},
		{"endpoint identity invalid", "CLOSURE_ENDPOINT_IDENTITY"},
		{"outcome", "CLOSURE_OTHER"},
		{"endpoint", "CLOSURE_OTHER"},
	} {
		if strings.Contains(message, candidate.needle) {
			return candidate.code
		}
	}
	return "OTHER"
}

func TestRetainedProgramCCaptureFailureClassification(t *testing.T) {
	root := os.Getenv("LSP_TRACE_TEST_CONTINUATION_OBJECT_ROOT")
	selector := os.Getenv("LSP_TRACE_TEST_CHECKPOINT_SELECTOR")
	workspace := os.Getenv("LSP_TRACE_TEST_WORKSPACE")
	if root == "" || selector == "" || workspace == "" {
		t.Skip("retained diagnostic inputs not supplied")
	}
	ctx := context.Background()
	store := retainedDiagnosticStore{root: root}
	raw, err := store.Get(ctx, selector)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := ParseCheckpoint(raw)
	if err != nil {
		t.Fatal(err)
	}
	s := pipelineState{checkpoint: checkpoint, artifacts: checkpoint.Artifacts(), contractID: checkpoint.ContractID(), profileID: checkpoint.ProfileID()}
	if err := loadState(ctx, store, &s); err != nil {
		t.Fatalf("load retained state: %v", err)
	}
	capture, err := CaptureSnapshots(s.handoff, s.program, workspace, s.handoff.PositionEncoding(), s.contract.CaptureLimits())
	captured, unavailable, targets, callers := 0, 0, 0, 0
	for _, constituent := range capture.Constituents {
		for _, outcome := range constituent.EndpointOutcomes {
			if outcome.Endpoint.Role == EndpointRoleTarget {
				targets++
			} else if outcome.Endpoint.Role == EndpointRoleCaller {
				callers++
			}
			if outcome.Status == EndpointStatusCaptured {
				captured++
			} else if outcome.Status == EndpointStatusSourceUnavailable {
				unavailable++
			}
		}
	}
	if err != nil {
		code, ordinal := classifyRetainedCaptureFailure(err), -1
		var diagnostic *CaptureDiagnosticError
		if errors.As(err, &diagnostic) {
			code, ordinal = diagnostic.Code, diagnostic.ConstituentOrdinal
		}
		t.Fatalf("ASSERT_RETAINED_PROGRAM_C_CAPTURE_CLASSIFICATION code=%s ordinal=%d constituents=%d snapshots=%d captured=%d unavailable=%d", code, ordinal, len(capture.Constituents), len(capture.Snapshots), captured, unavailable)
	}
	t.Logf("ASSERT_RETAINED_PROGRAM_C_CAPTURE_CLASSIFICATION code=NONE constituents=%d snapshots=%d targets=%d callers=%d captured=%d unavailable=%d", len(capture.Constituents), len(capture.Snapshots), targets, callers, captured, unavailable)
}

func TestNonemptyRetainedPacketDiagnostic(t *testing.T) {
	root := os.Getenv("LSP_TRACE_TEST_CONTINUATION_OBJECT_ROOT")
	selector := os.Getenv("LSP_TRACE_TEST_CHECKPOINT_SELECTOR")
	if root == "" || selector == "" {
		t.Skip("retained diagnostic inputs not supplied")
	}
	ctx := context.Background()
	store := retainedDiagnosticStore{root: root}
	raw, err := store.Get(ctx, selector)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := ParseCheckpoint(raw)
	if err != nil {
		t.Fatal(err)
	}
	s := pipelineState{
		checkpoint: checkpoint,
		artifacts:  checkpoint.Artifacts(),
		contractID: checkpoint.ContractID(),
		profileID:  checkpoint.ProfileID(),
	}
	if err := loadState(ctx, store, &s); err != nil {
		t.Fatalf("load retained state: %v", err)
	}
	snapshots, lookup, err := ReplaySnapshots(s.capture)
	if err != nil {
		t.Fatalf("replay snapshots: %v", err)
	}
	for _, nomination := range s.program.Representatives.Nominations {
		t.Logf("NOMINATION ordinal=%d identity=%s node=%s", nomination.ConstituentOrdinal, nomination.ConstituentIdentity, nomination.SelectedNode)
	}
	for _, snapshot := range snapshots {
		t.Logf("SNAPSHOT ordinal=%d identity=%s bytes=%d", snapshot.ConstituentOrdinal, snapshot.ConstituentIdentity, len(snapshot.Raw))
	}
	_, err = targetpacket.Build(targetpacket.Request{
		Census:           s.program,
		Snapshots:        snapshots,
		Lookup:           lookup,
		Policy:           s.contract.PacketSourcePolicy(),
		ResolveLimits:    s.contract.PacketResolveLimits(),
		MaxResponseBytes: s.contract.PacketMaxResponseBytes(),
	})
	if err != nil {
		t.Fatalf("ASSERT_NONEMPTY_RETAINED_PACKET: %v; cause=%v", err, errors.Unwrap(err))
	}
}
