package censusresult

import cd "lsp-trace/internal/capacitydiagnostic"

const CapacityDiagnosticSchemaVersion = cd.CapacityDiagnosticSchemaVersion

type CapacityDiagnostic = cd.CapacityDiagnostic
type CapacityDiagnosticMetrics = cd.CapacityDiagnosticMetrics

func ValidateCapacityDiagnostic(d CapacityDiagnostic) error { return cd.ValidateCapacityDiagnostic(d) }
func MarshalCapacityDiagnostic(d CapacityDiagnostic) ([]byte, error) {
	return cd.MarshalCapacityDiagnostic(d)
}
func DecodeCapacityDiagnostic(raw []byte) (CapacityDiagnostic, error) {
	return cd.DecodeCapacityDiagnostic(raw)
}
