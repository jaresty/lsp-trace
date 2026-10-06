package sessionruntime

import (
	"testing"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
)

func TestPrivateDiagnosticMetadataOwnedCopyAndAtomicTransfer(t *testing.T) {
	m := newC15DiagnosticManager(t)
	g := m.newDiagnosticGeneration("attempt", "session", 1)
	account := mustPrivateB4ByteAccountV2(t)
	before := account.snapshot()
	h, c := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account)
	if !h.private.qualified || c == nil || len(m.diagnosticOperations) != 0 || len(m.diagnosticOrder) != 0 {
		t.Fatal("ASSERT_C15_PRIVATE_CREATE_FIXED_ONLY")
	}
	eventBytes := uint64(m.limits.MaxObservations) * manageddiagnostic.EventElementBytes()
	if got := account.snapshot().Live; got != before.Live+eventBytes {
		t.Fatalf("ASSERT_C15_PRIVATE_CREATE_EVENT_CHARGE got=%d want=%d", got, before.Live+eventBytes)
	}
	methodBytes := []byte("textDocument/definition")
	uriBytes := []byte("file:///owned.go")
	encodingBytes := []byte("utf-16")
	providerBytes := []byte("provider")
	versionBytes := []byte("1.2.3")
	commandBytes := []byte("server --stdio")
	metadata := SessionMetadata{PositionEncoding: string(encodingBytes), ProviderName: string(providerBytes), ProviderVersion: string(versionBytes), ServerCommand: string(commandBytes)}
	metadataBytes := uint64(len(methodBytes) + len(uriBytes) + len(encodingBytes) + len(providerBytes) + len(versionBytes) + len(commandBytes))
	m.describeDiagnosticOperation(h, string(methodBytes), string(uriBytes), metadata)
	if got := account.snapshot().Live; got != before.Live+eventBytes+metadataBytes {
		t.Fatalf("ASSERT_C15_PRIVATE_METADATA_CHARGE got=%d want=%d", got, before.Live+eventBytes+metadataBytes)
	}
	for _, b := range [][]byte{methodBytes, uriBytes, encodingBytes, providerBytes, versionBytes, commandBytes} {
		for i := range b {
			b[i] = 'x'
		}
	}
	account.requestTerminalRelease()
	combinedBefore := account.snapshot().Live - privateB4ByteLedgerV2TableBytes + m.managerDiagnosticOwner.snapshot().Live
	m.completeDiagnosticOperation(h, c, diagnosticEventTerminalResponse)
	combinedAfter := account.snapshot().Live + m.managerDiagnosticOwner.snapshot().Live
	if combinedAfter != combinedBefore || account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_PRIVATE_METADATA_TRANSFER before=%d after=%d account=%+v", combinedBefore, combinedAfter, account.snapshot())
	}
	s, ok := m.DiagnosticSnapshotFor("attempt", h)
	if !ok || s.Method != "textDocument/definition" || s.DocumentURI != "file:///owned.go" || s.Initialize.PositionEncoding != "utf-16" || s.Initialize.ProviderName != "provider" || s.Initialize.ProviderVersion != "1.2.3" || s.Initialize.ServerCommand != "server --stdio" {
		t.Fatalf("ASSERT_C15_PRIVATE_METADATA_OWNED snapshot=%+v ok=%t", s, ok)
	}
}

func TestPrivateDiagnosticMetadataInitialReserveFailureOmitsCompletely(t *testing.T) {
	m := newC15DiagnosticManager(t)
	g := m.newDiagnosticGeneration("attempt", "session", 1)
	account := mustPrivateB4ByteAccountV2(t)
	h, c := m.newPrivateB4DiagnosticOperation(g, managedprocess.Identity{}, account)
	fill, failure := account.reserve(privateB4MaxOwnedBytes - account.snapshot().Live)
	if failure != "" {
		t.Fatal(failure)
	}
	beforeManager := m.managerDiagnosticOwner.snapshot()
	m.describeDiagnosticOperation(h, "m", "u", SessionMetadata{})
	if m.privateDiagnosticHistory.resolve(h.private, h.sequence) != nil || len(m.diagnosticOperations) != 0 || len(m.diagnosticOrder) != 0 || m.managerDiagnosticOwner.snapshot() != beforeManager || m.diagnosticOmissions != 1 {
		t.Fatal("ASSERT_C15_PRIVATE_METADATA_REFUSAL_ATOMIC")
	}
	if _, ok := c.DetachEventBacking(); ok {
		t.Fatal("ASSERT_C15_PRIVATE_METADATA_REFUSAL_BACKING_RETAINED")
	}
	fill.release()
	account.requestTerminalRelease()
	if account.snapshot().Live != 0 {
		t.Fatalf("ASSERT_C15_PRIVATE_METADATA_REFUSAL_QUIESCENCE %+v", account.snapshot())
	}
}
