package adr0011methodresult

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

func TestCanonicalCandidateRejectsTransportSuccessWithoutManagedWrite(t *testing.T) {
	req := fixtureRequest(transport.MethodDefinition, false)
	fake := &fixtureRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, PositionEncoding: "utf-16", ProviderName: "fake"}, response: sessionruntime.RoundTripResult{
		Key: lspwire.RequestKey{Generation: req.Generation, ID: 1}, Result: json.RawMessage(`null`), Messages: 1, Bytes: 40,
	}}
	wire := transport.New(fake).Execute(context.Background(), req)
	if wire.Outcome() != transport.OutcomeTransportSuccess {
		t.Fatalf("fixture is not a transport success: %s", wire.Outcome())
	}
	if _, err := BuildCanonicalCandidate(req, wire, 3); !errors.Is(err, ErrInvalidCanonicalCandidate) {
		t.Fatalf("fake runtime without managed completed write issued candidate: %v", err)
	}
}
