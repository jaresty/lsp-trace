package session

// Availability is the closed lifecycle/live implementation availability vocabulary.
type Availability string

const (
	NotImplemented         Availability = "NOT_IMPLEMENTED"
	ContainmentUnavailable Availability = "CONTAINMENT_UNAVAILABLE"
	RuntimeDisabled        Availability = "RUNTIME_DISABLED"
	Enabled                Availability = "ENABLED"
)

// Failure is the closed set of failure codes applicable to lifecycle operations.
type Failure string

const (
	ToolNotImplemented            Failure = "TOOL_NOT_IMPLEMENTED"
	ProcessContainmentUnavailable Failure = "PROCESS_CONTAINMENT_UNAVAILABLE"
	LiveLSPDisabled               Failure = "LIVE_LSP_DISABLED"
	SessionNotFound               Failure = "SESSION_NOT_FOUND"
	ListCursorInvalid             Failure = "LIST_CURSOR_INVALID"
	StaleGeneration               Failure = "STALE_GENERATION"
	LifecycleConflict             Failure = "LIFECYCLE_CONFLICT"
	SessionReapIncomplete         Failure = "SESSION_REAP_INCOMPLETE"
	SessionStopping               Failure = "SESSION_STOPPING"
	SessionPoisoned               Failure = "SESSION_POISONED"
	ResourceExhausted             Failure = "RESOURCE_EXHAUSTED"
	SpawnFailure                  Failure = "SPAWN_FAILED"
	PipeSetupFailure              Failure = "PIPE_SETUP_FAILED"
	InitializationFailure         Failure = "INITIALIZATION_FAILED"
	InitializationTimeout         Failure = "INITIALIZATION_TIMEOUT"
	SessionCrashed                Failure = "SESSION_CRASHED"
	RequestCancelled              Failure = "REQUEST_CANCELLED"
	RequestTimeout                Failure = "REQUEST_TIMEOUT"
)

// RestartValue is the closed lifecycle intent vocabulary used by restart semantics.
type RestartValue string

const (
	StopIntent    RestartValue = "STOP"
	RestartIntent RestartValue = "RESTART"
	EvictIntent   RestartValue = "EVICT"
)

func PublicEvents() []Event {
	return []Event{FirstStart, RestartAccepted, PipesEstablished, SpawnFailed, SetupFailed, IntentAccepted, Initialized, InitFailed, GracefulIntent, WorkDrained, UnexpectedDeath, Poison, Reaped}
}

func PublicGuards() []Guard {
	return []Guard{Verified, PredecessorReaped, NoChild, PossibleChild, HasWork, NoWork, Any}
}

func AvailabilityValues() []Availability {
	return []Availability{NotImplemented, ContainmentUnavailable, RuntimeDisabled, Enabled}
}

func FailureValues() []Failure {
	return []Failure{
		ToolNotImplemented,
		ProcessContainmentUnavailable,
		LiveLSPDisabled,
		SessionNotFound,
		ListCursorInvalid,
		StaleGeneration,
		LifecycleConflict,
		SessionReapIncomplete,
		SessionStopping,
		SessionPoisoned,
		ResourceExhausted,
		SpawnFailure,
		PipeSetupFailure,
		InitializationFailure,
		InitializationTimeout,
		SessionCrashed,
		RequestCancelled,
		RequestTimeout,
	}
}

func RestartValues() []RestartValue {
	return []RestartValue{StopIntent, RestartIntent, EvictIntent}
}
