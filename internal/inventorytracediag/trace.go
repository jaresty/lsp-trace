// Package inventorytracediag emits opt-in diagnostics, never trace ownership or authority.
package inventorytracediag

import (
	"context"
	"runtime/trace"
)

// Code is a closed diagnostic vocabulary, not a provider/error classification API.
type Code uint8

const (
	Invalid Code = iota
	Execute
	Replay
	Window
	Start
	Readiness
	Capture
	Finish
	Observe
	ExecuteRefused
	DeferredFinish
	DeferredFinishDone
	LeaseCallback
	CaptureRefused
	NativeCallsAttempt
	NativeCallsRefused
	NativeCallsReturned
	IssueAttempt
	IssueRefused
	LedgerUnavailable
	LedgerEmpty
	Confirmed
	Issue
	OwnedDeadlineRefused
	OwnedIdentityRefused
	OwnedTargetRefused
	BindOwnedRefused
	QueryBytesRefused
	BuildAttempt
	BuildRefused
	PublicationRefused
	ReplayAttempt
	ConsumeRefused
	TakeEvidenceRefused
	Build
	ProviderMetadataRefused
	KindRefused
	DuplicateKindRefused
	MethodUnavailableRefused
	MethodIdentityRefused
	MethodCoverageRefused
	MethodSelectionRefused
	MethodParseRefused
	ReferencesEmpty
	ReferencesNonempty
	DefinitionsEmpty
	DefinitionsNonempty
	EmptyCallsCompositionRefused
	TypedCallsAttempt
	TypedCallsRefused
	MethodsAdmissionRefused
	TypedMethodsAttempt
	TypedMethodsRefused
	Assemble
	ProjectionAttempt
	ProjectionRefused
	BuildInputAttempt
	BuildInputRefused
	ParseAdmitAttempt
	ParseAdmitRefused
	ReadinessRegion
	ReadinessRefused
	CapabilityUnsupported
	NativeProofWithoutCalls
	CompositeBindingBoundRefused
	CompositeBindingJSONRefused
	NativePredecessorCountRefused
	NativePredecessorBytesRefused
	NativeAggregateBoundRefused
)

var codeNames = [...]string{
	"INVALID", "EXECUTE", "REPLAY", "WINDOW", "START", "READINESS", "CAPTURE", "FINISH", "OBSERVE", "EXECUTE_REFUSED", "DEFERRED_FINISH", "DEFERRED_FINISH_DONE", "LEASE_CALLBACK", "CAPTURE_REFUSED", "NATIVE_CALLS_ATTEMPT", "NATIVE_CALLS_REFUSED", "NATIVE_CALLS_RETURNED", "ISSUE_ATTEMPT", "ISSUE_REFUSED", "LEDGER_UNAVAILABLE", "LEDGER_EMPTY", "CONFIRMED", "ISSUE", "OWNED_DEADLINE_REFUSED", "OWNED_IDENTITY_REFUSED", "OWNED_TARGET_REFUSED", "BIND_OWNED_REFUSED", "QUERY_BYTES_REFUSED", "BUILD_ATTEMPT", "BUILD_REFUSED", "PUBLICATION_REFUSED", "REPLAY_ATTEMPT", "CONSUME_REFUSED", "TAKE_EVIDENCE_REFUSED", "BUILD", "PROVIDER_METADATA_REFUSED", "KIND_REFUSED", "DUPLICATE_KIND_REFUSED", "METHOD_UNAVAILABLE_REFUSED", "METHOD_IDENTITY_REFUSED", "METHOD_COVERAGE_REFUSED", "METHOD_SELECTION_REFUSED", "METHOD_PARSE_REFUSED", "REFERENCES_EMPTY", "REFERENCES_NONEMPTY", "DEFINITIONS_EMPTY", "DEFINITIONS_NONEMPTY", "EMPTY_CALLS_COMPOSITION_REFUSED", "TYPED_CALLS_ATTEMPT", "TYPED_CALLS_REFUSED", "METHODS_ADMISSION_REFUSED", "TYPED_METHODS_ATTEMPT", "TYPED_METHODS_REFUSED", "ASSEMBLE", "PROJECTION_ATTEMPT", "PROJECTION_REFUSED", "BUILD_INPUT_ATTEMPT", "BUILD_INPUT_REFUSED", "PARSE_ADMIT_ATTEMPT", "PARSE_ADMIT_REFUSED", "READINESS_REGION", "READINESS_REFUSED", "CAPABILITY_UNSUPPORTED", "NATIVE_PROOF_WITHOUT_CALLS", "COMPOSITE_BINDING_BOUND_REFUSED", "COMPOSITE_BINDING_JSON_REFUSED", "NATIVE_PREDECESSOR_COUNT_REFUSED", "NATIVE_PREDECESSOR_BYTES_REFUSED", "NATIVE_AGGREGATE_BOUND_REFUSED",
}

func Enabled() bool        { return trace.IsEnabled() }
func Valid(code Code) bool { return code > Invalid && int(code) < len(codeNames) }

// loggingContext never replaces an operation's actual context. Background has
// goroutine-local attribution only, not the caller's task attribution.
func loggingContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
func Event(ctx context.Context, code Code) {
	if !trace.IsEnabled() || !Valid(code) {
		return
	}
	trace.Log(loggingContext(ctx), "inventory.checkpoint", codeNames[code])
}

// Region is zero-value safe; its disabled form neither allocates nor builds text.
type Region struct{ region *trace.Region }

func Begin(ctx context.Context, code Code) Region {
	if !trace.IsEnabled() || !Valid(code) {
		return Region{}
	}
	return Region{trace.StartRegion(loggingContext(ctx), codeNames[code])}
}
func (r Region) End() {
	if r.region != nil {
		r.region.End()
	}
}

// Mask is bounded to document symbols/CALLS/references/definitions, bits0..3.
type Mask uint8

const (
	DocumentSymbols Mask = 1 << iota
	Calls
	References
	Definitions
	All = DocumentSymbols | Calls | References | Definitions
)

var maskNames = [...]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15"}

func ValidMask(m Mask) bool { return m&^All == 0 }

// CapabilityMasks is pure; observed includes every parsed support flag, even if
// unrequested. Missing includes requested flags only, with symbols mandatory.
func CapabilityMasks(calls, references, definitions, haveSymbols, haveCalls, haveReferences, haveDefinitions bool) (requested, observed, missing Mask) {
	requested = DocumentSymbols
	if calls {
		requested |= Calls
	}
	if references {
		requested |= References
	}
	if definitions {
		requested |= Definitions
	}
	if haveSymbols {
		observed |= DocumentSymbols
	}
	if haveCalls {
		observed |= Calls
	}
	if haveReferences {
		observed |= References
	}
	if haveDefinitions {
		observed |= Definitions
	}
	return requested, observed, requested &^ observed
}
func Capabilities(ctx context.Context, requested, observed Mask) {
	if !trace.IsEnabled() || !ValidMask(requested) || !ValidMask(observed) {
		return
	}
	ctx = loggingContext(ctx) // logging only; no actual operation sees this value
	trace.Log(ctx, "inventory.cap.requested", maskNames[requested])
	trace.Log(ctx, "inventory.cap.observed", maskNames[observed])
	trace.Log(ctx, "inventory.cap.missing", maskNames[requested&^observed])
}

// MethodState receives booleans only, never provider bytes or method strings.
func MethodState(ctx context.Context, definition, empty bool) {
	if !trace.IsEnabled() {
		return
	}
	code := ReferencesNonempty
	if definition {
		code = DefinitionsNonempty
	}
	if empty {
		if definition {
			code = DefinitionsEmpty
		} else {
			code = ReferencesEmpty
		}
	}
	Event(ctx, code)
}
