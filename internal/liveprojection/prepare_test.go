package liveprojection

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

const (
	assertPrepareExactGeneration = "ASSERT_PREPARE_EXACT_GENERATION"
	assertPrepareCanonicalOrder  = "ASSERT_PREPARE_TARGET_FIRST_THEN_LEXICAL"
	assertPrepareAtMostOnce      = "ASSERT_PREPARE_URI_AT_MOST_ONCE"
	assertPrepareCapture         = "ASSERT_PREPARE_CAPTURE_SUPPLY"
	assertPrepareLimitAtomic     = "ASSERT_PREPARE_LIMIT_ATOMIC"
	assertPrepareDocumentBytes   = "ASSERT_PREPARE_PER_DOCUMENT_BYTE_LIMIT"
	assertPrepareMessages        = "ASSERT_PREPARE_CAPTURED_MESSAGE_LIMIT"
	assertPrepareFailureTerminal = "ASSERT_PREPARE_FAILURE_TERMINAL"
	assertPrepareAccounting      = "ASSERT_PREPARE_ACCOUNTING"
	assertRawSuppliesPrivate     = "ASSERT_RAW_SUPPLIES_PRIVATE"
)

type recordingPreparer struct {
	requests  []sessionruntime.DocumentRequest
	responses map[string]sessionruntime.DocumentResult
}

func (p *recordingPreparer) PrepareDocument(_ context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	p.requests = append(p.requests, request)
	if result, ok := p.responses[request.URI]; ok {
		return result
	}
	return supplied(request, []byte(request.URI))
}

func supplied(request sessionruntime.DocumentRequest, content []byte) sessionruntime.DocumentResult {
	return sessionruntime.DocumentResult{URI: request.URI, Supply: &sessionruntime.DocumentSupply{
		Classification: "LSP_SUPPLIED", SessionID: request.SessionID, Generation: request.Generation,
		URI: request.URI, DocumentVersion: 1, Method: "textDocument/didOpen",
		Content: append([]byte(nil), content...), Params: json.RawMessage(`{"textDocument":{"text":"secret"}}`),
	}}
}

func generousLimits() PreparationLimits {
	return PreparationLimits{MaxDocuments: 8, MaxBytes: 4096, MaxDocumentBytes: 4096, MaxMessages: 8, MaxWork: 8}
}

func TestPrepareExactGenerationCanonicalOrderAndAtMostOnce(t *testing.T) {
	t.Log(assertPrepareExactGeneration, assertPrepareCanonicalOrder, assertPrepareAtMostOnce, assertPrepareCapture, assertPrepareAccounting)
	preparer := &recordingPreparer{}
	plan := []string{"file:///target.go", "file:///a.go", "file:///a.go", "file:///z.go"}
	got := Prepare(context.Background(), preparer, "session-1", 7, "go", plan, generousLimits())

	wantURIs := []string{"file:///target.go", "file:///a.go", "file:///z.go"}
	requestURIs := make([]string, len(preparer.requests))
	for i, request := range preparer.requests {
		requestURIs[i] = request.URI
		if request.SessionID != "session-1" || request.Generation != 7 {
			t.Fatalf("%s: request=%+v", assertPrepareExactGeneration, request)
		}
		if !request.CaptureSupply {
			t.Fatalf("%s: request=%+v", assertPrepareCapture, request)
		}
	}
	seen := map[string]bool{}
	for _, uri := range requestURIs {
		if seen[uri] {
			t.Fatalf("%s: got=%q", assertPrepareAtMostOnce, requestURIs)
		}
		seen[uri] = true
	}
	if !slices.Equal(requestURIs, wantURIs) {
		t.Fatalf("%s: got=%q want=%q", assertPrepareCanonicalOrder, requestURIs, wantURIs)
	}
	if got.Status != PreparationComplete || len(got.Supplies) != len(wantURIs) || got.Accounting.Attempted != 3 || got.Accounting.Succeeded != 3 || got.Accounting.Documents.Observed != 3 || got.Accounting.Work.Observed != 3 {
		t.Fatalf("%s: result=%+v", assertPrepareAccounting, got)
	}
}

func TestPrepareLimitsAreIndependentAndAtomic(t *testing.T) {
	t.Log(assertPrepareLimitAtomic, assertPrepareAccounting)
	plan := []string{"file:///target.go", "file:///a.go"}
	for _, tc := range []struct {
		name   string
		limits PreparationLimits
		calls  int
	}{
		{name: "documents", limits: PreparationLimits{MaxDocuments: 1, MaxBytes: 4096, MaxDocumentBytes: 4096, MaxMessages: 8, MaxWork: 8}},
		{name: "work", limits: PreparationLimits{MaxDocuments: 8, MaxBytes: 4096, MaxDocumentBytes: 4096, MaxMessages: 8, MaxWork: 1}},
		{name: "bytes", limits: PreparationLimits{MaxDocuments: 8, MaxBytes: 3, MaxDocumentBytes: 4096, MaxMessages: 8, MaxWork: 8}, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preparer := &recordingPreparer{responses: map[string]sessionruntime.DocumentResult{
				"file:///target.go": supplied(sessionruntime.DocumentRequest{SessionID: "s", Generation: 2, URI: "file:///target.go", CaptureSupply: true}, []byte("four")),
			}}
			got := Prepare(context.Background(), preparer, "s", 2, "go", plan, tc.limits)
			if got.Status != PreparationResourceLimit || len(got.Supplies) != 0 || len(preparer.requests) != tc.calls {
				t.Fatalf("%s: case=%s calls=%d result=%+v", assertPrepareLimitAtomic, tc.name, len(preparer.requests), got)
			}
			if got.Accounting.Documents.Limit != tc.limits.MaxDocuments || got.Accounting.Bytes.Limit != tc.limits.MaxBytes || got.Accounting.Work.Limit != tc.limits.MaxWork {
				t.Fatalf("%s: case=%s accounting=%+v", assertPrepareAccounting, tc.name, got.Accounting)
			}
		})
	}
}

func TestPreparePerDocumentAndMessageLimitsAreAtomic(t *testing.T) {
	t.Log(assertPrepareDocumentBytes, assertPrepareMessages)
	plan := []string{"file:///target.go", "file:///a.go"}
	for _, tc := range []struct {
		name   string
		limits PreparationLimits
		calls  int
	}{
		{name: "document-bytes", limits: PreparationLimits{MaxDocuments: 8, MaxBytes: 4096, MaxDocumentBytes: 3, MaxMessages: 8, MaxWork: 8}, calls: 1},
		{name: "messages", limits: PreparationLimits{MaxDocuments: 8, MaxBytes: 4096, MaxDocumentBytes: 4096, MaxMessages: 1, MaxWork: 8}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preparer := &recordingPreparer{responses: map[string]sessionruntime.DocumentResult{"file:///target.go": supplied(sessionruntime.DocumentRequest{SessionID: "s", Generation: 2, URI: "file:///target.go", CaptureSupply: true}, []byte("four"))}}
			got := Prepare(context.Background(), preparer, "s", 2, "go", plan, tc.limits)
			if got.Status != PreparationResourceLimit || len(got.Supplies) != 0 || len(preparer.requests) != tc.calls {
				t.Fatalf("case=%s result=%+v calls=%d", tc.name, got, len(preparer.requests))
			}
		})
	}
}

func TestPrepareFailureIsTerminalWithoutPartialSupplies(t *testing.T) {
	t.Log(assertPrepareFailureTerminal)
	preparer := &recordingPreparer{responses: map[string]sessionruntime.DocumentResult{
		"file:///a.go": {URI: "file:///a.go", Failure: session.StaleGeneration},
	}}
	got := Prepare(context.Background(), preparer, "s", 3, "go", []string{"file:///target.go", "file:///a.go", "file:///z.go"}, generousLimits())
	if got.Status != PreparationFailed || got.Failure == nil || got.Failure.URI != "file:///a.go" || got.Failure.Failure != session.StaleGeneration || len(preparer.requests) != 2 || len(got.Supplies) != 0 {
		t.Fatalf("%s: requests=%+v result=%+v", assertPrepareFailureTerminal, preparer.requests, got)
	}
}

func TestPrepareRawSuppliesAreNotJSON(t *testing.T) {
	t.Log(assertRawSuppliesPrivate)
	got := Prepare(context.Background(), &recordingPreparer{}, "s", 1, "go", []string{"file:///target.go"}, generousLimits())
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	for _, forbidden := range []string{"secret", "Content", "Params", "Supplies", "LSP_SUPPLIED"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("%s: found=%q json=%s", assertRawSuppliesPrivate, forbidden, encoded)
		}
	}
}
