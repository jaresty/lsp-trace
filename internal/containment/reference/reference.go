// Package reference provides synthetic classification for specifications,
// fixtures, and tests. Its Verdict is intentionally a different type from the
// production containment Snapshot and has no authorization semantics.
package reference

type Classification string
type Reason string
type CheckID string

const (
	Verified    Classification = "VERIFIED"
	Unavailable Classification = "UNAVAILABLE"
)

const (
	ReasonUnsupportedPlatform            Reason = "unsupported_platform"
	ReasonUnsupportedProfile             Reason = "unsupported_profile"
	ReasonProbeFailed                    Reason = "probe_failed"
	ReasonIndeterminate                  Reason = "indeterminate"
	ReasonMutableConfiguration           Reason = "mutable_configuration"
	ReasonInsufficientAuthorityIsolation Reason = "insufficient_authority_isolation"
	ReasonNativeTimeout                  Reason = "native_timeout"
)

const (
	CheckPlatformSupport      CheckID = "platform_support"
	CheckPrimitiveProfile     CheckID = "primitive_profile"
	CheckOwnerDeathCleanup    CheckID = "owner_death_cleanup"
	CheckCompleteDescendants  CheckID = "complete_descendants"
	CheckCreationRace         CheckID = "creation_race"
	CheckEscape               CheckID = "escape"
	CheckTransfer             CheckID = "transfer"
	CheckReparent             CheckID = "reparent"
	CheckDelegation           CheckID = "delegation"
	CheckInheritedIOSafety    CheckID = "inherited_io_safety"
	CheckControlAuthority     CheckID = "control_authority"
	CheckCleanupBound         CheckID = "cleanup_bound"
	CheckSurvivorEnumeration  CheckID = "survivor_enumeration"
	CheckDeathObservation     CheckID = "death_observation"
	CheckReap                 CheckID = "reap"
	CheckImmutableAttestation CheckID = "immutable_attestation"
)

// Observation is synthetic fixture input. Booleans classify only the reference
// model and cannot be passed to the production runtime gate.
type Observation struct {
	PlatformSupport, PrimitiveProfile, OwnerDeathCleanup, CompleteDescendants bool
	CreationRace, Escape, Transfer, Reparent, Delegation                      bool
	InheritedIOSafety, ControlAuthority, CleanupBound                         bool
	SurvivorEnumeration, DeathObservation, Reap, ImmutableAttestation         bool
}

type Verdict struct {
	Classification Classification
	Reason         Reason
	FailedCheck    CheckID
}

type obligation struct {
	id     CheckID
	pass   func(Observation) bool
	reason Reason
}

var obligations = [...]obligation{
	{CheckPlatformSupport, func(o Observation) bool { return o.PlatformSupport }, ReasonUnsupportedPlatform},
	{CheckPrimitiveProfile, func(o Observation) bool { return o.PrimitiveProfile }, ReasonUnsupportedProfile},
	{CheckOwnerDeathCleanup, func(o Observation) bool { return o.OwnerDeathCleanup }, ReasonProbeFailed},
	{CheckCompleteDescendants, func(o Observation) bool { return o.CompleteDescendants }, ReasonProbeFailed},
	{CheckCreationRace, func(o Observation) bool { return o.CreationRace }, ReasonProbeFailed},
	{CheckEscape, func(o Observation) bool { return o.Escape }, ReasonInsufficientAuthorityIsolation},
	{CheckTransfer, func(o Observation) bool { return o.Transfer }, ReasonInsufficientAuthorityIsolation},
	{CheckReparent, func(o Observation) bool { return o.Reparent }, ReasonInsufficientAuthorityIsolation},
	{CheckDelegation, func(o Observation) bool { return o.Delegation }, ReasonInsufficientAuthorityIsolation},
	{CheckInheritedIOSafety, func(o Observation) bool { return o.InheritedIOSafety }, ReasonInsufficientAuthorityIsolation},
	{CheckControlAuthority, func(o Observation) bool { return o.ControlAuthority }, ReasonInsufficientAuthorityIsolation},
	{CheckCleanupBound, func(o Observation) bool { return o.CleanupBound }, ReasonNativeTimeout},
	{CheckSurvivorEnumeration, func(o Observation) bool { return o.SurvivorEnumeration }, ReasonProbeFailed},
	{CheckDeathObservation, func(o Observation) bool { return o.DeathObservation }, ReasonProbeFailed},
	{CheckReap, func(o Observation) bool { return o.Reap }, ReasonProbeFailed},
	{CheckImmutableAttestation, func(o Observation) bool { return o.ImmutableAttestation }, ReasonMutableConfiguration},
}

// Evaluate applies the fixed obligation order and returns the first failure.
// A Verified result describes only this reference model.
func Evaluate(observation Observation) Verdict {
	for _, item := range obligations {
		if !item.pass(observation) {
			return Verdict{Classification: Unavailable, Reason: item.reason, FailedCheck: item.id}
		}
	}
	return Verdict{Classification: Verified}
}

func OrderedChecks() []CheckID {
	result := make([]CheckID, len(obligations))
	for i, item := range obligations {
		result[i] = item.id
	}
	return result
}

func Reasons() []Reason {
	return []Reason{
		ReasonUnsupportedPlatform,
		ReasonUnsupportedProfile,
		ReasonProbeFailed,
		ReasonIndeterminate,
		ReasonMutableConfiguration,
		ReasonInsufficientAuthorityIsolation,
		ReasonNativeTimeout,
	}
}

// CompleteObservationForTest is explicitly synthetic and non-authorizing.
func CompleteObservationForTest() Observation {
	return Observation{
		PlatformSupport: true, PrimitiveProfile: true,
		OwnerDeathCleanup: true, CompleteDescendants: true,
		CreationRace: true, Escape: true, Transfer: true, Reparent: true,
		Delegation: true, InheritedIOSafety: true, ControlAuthority: true,
		CleanupBound: true, SurvivorEnumeration: true, DeathObservation: true,
		Reap: true, ImmutableAttestation: true,
	}
}
