package sessionruntime

import (
	"testing"
	"unsafe"
)

func TestPrivateDiagnosticHistoryQualifiedLayout(t *testing.T) {
	if !privateDiagnosticHistoryRepresentationSupported() ||
		unsafe.Sizeof(privateDiagnosticHistorySlot{}) != 528 || unsafe.Alignof(privateDiagnosticHistorySlot{}) != 8 ||
		unsafe.Sizeof(privateDiagnosticHistory{}) != 542744 || unsafe.Alignof(privateDiagnosticHistory{}) != 8 ||
		unsafe.Sizeof(privateDiagnosticHistoryRef{}) != 16 || unsafe.Alignof(privateDiagnosticHistoryRef{}) != 8 ||
		unsafe.Sizeof(DiagnosticOperationHandle{}) != 72 || unsafe.Alignof(DiagnosticOperationHandle{}) != 8 ||
		unsafe.Offsetof(privateDiagnosticHistory{}.slots) != 0 || unsafe.Offsetof(privateDiagnosticHistory{}.order) != 540672 ||
		managerPrivateDiagnosticHistoryTableBytes() != 542744 {
		t.Fatal("ASSERT_C15_PRIVATE_DIAGNOSTIC_HISTORY_LAYOUT")
	}
}
