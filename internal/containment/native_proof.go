package containment

// nativeProof is scaffolding for a future platform-owned evidence collector.
// It is deliberately package-private and is not consumed by RuntimeGate.
type nativeProof interface {
	Evidence() nativeEvidence
}

// nativeEvidence can carry bounded diagnostics from a future native probe.
// It contains no authorization state and has no path into production results.
type nativeEvidence struct {
	platform    string
	reason      Reason
	failedCheck CheckID
}
