package containment

// RuntimeGate is the production containment gate. It intentionally has no
// injectable probe, fixture adapter, setter, refresh, or deserializer.
type RuntimeGate struct {
	result runtimeResult
}

// Snapshot is a bounded, fail-closed production availability input. It has no
// VERIFIED constructor or conversion from the reference package.
type Snapshot struct {
	Classification Classification
	Platform       string
	Reason         Reason
	FailedCheck    CheckID
}

// NewRuntimeGate executes the platform path directly. Every current platform
// path is unavailable; future native authorization requires a separate review.
func NewRuntimeGate() RuntimeGate { return RuntimeGate{result: platformProbe()} }

func (g RuntimeGate) Snapshot() Snapshot {
	return Snapshot{
		Classification: g.result.classification,
		Platform:       g.result.platform,
		Reason:         g.result.reason,
		FailedCheck:    g.result.failedCheck,
	}
}
