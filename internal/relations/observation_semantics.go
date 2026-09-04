package relations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const ObservationSchemaV1 = "lsp-trace.observation.v1"

type RelationKind string

const (
	RelationCalls          RelationKind = "CALLS"
	RelationBindsArgument  RelationKind = "BINDS_ARGUMENT"
	RelationPassesCallback RelationKind = "PASSES_CALLBACK"
	RelationInvokesTask    RelationKind = "INVOKES_TASK"
	RelationTriggersReload RelationKind = "TRIGGERS_RELOAD"
	RelationUpdatesState   RelationKind = "UPDATES_STATE"
	RelationRendersFrom    RelationKind = "RENDERS_FROM"
)

func (k RelationKind) Valid() bool {
	switch k {
	case RelationCalls, RelationBindsArgument, RelationPassesCallback, RelationInvokesTask, RelationTriggersReload, RelationUpdatesState, RelationRendersFrom:
		return true
	default:
		return false
	}
}

type Authority string

const (
	AuthorityServerReported       Authority = "SERVER_REPORTED"
	AuthoritySourceDerivedAdapter Authority = "SOURCE_DERIVED_ADAPTER"
	AuthorityCallerAsserted       Authority = "CALLER_ASSERTED"
)

func (a Authority) Valid() bool {
	return a == AuthorityServerReported || a == AuthoritySourceDerivedAdapter || a == AuthorityCallerAsserted
}

type EndpointRole string

const (
	RoleCaller            EndpointRole = "CALLER"
	RoleCallee            EndpointRole = "CALLEE"
	RoleSourceExpression  EndpointRole = "SOURCE_EXPRESSION"
	RoleBoundArgument     EndpointRole = "BOUND_ARGUMENT"
	RoleCallableReference EndpointRole = "CALLABLE_REFERENCE"
	RoleCallbackParameter EndpointRole = "CALLBACK_PARAMETER"
	RoleTaskInvocation    EndpointRole = "TASK_INVOCATION"
	RoleTask              EndpointRole = "TASK"
	RoleReloadRequest     EndpointRole = "RELOAD_REQUEST"
	RoleReloadTarget      EndpointRole = "RELOAD_TARGET"
	RoleStateProducer     EndpointRole = "STATE_PRODUCER"
	RoleStateValue        EndpointRole = "STATE_VALUE"
	RoleRenderExpression  EndpointRole = "RENDER_EXPRESSION"
	RoleReadValue         EndpointRole = "READ_VALUE"
)

type Claim string

const (
	ClaimSourceDependencyRelation Claim = "source_dependency_relation"
	ClaimCallbackInvocation       Claim = "callback_invocation"
	ClaimRuntimeExecution         Claim = "runtime_execution"
	ClaimRepaint                  Claim = "repaint"
	ClaimFeatureIdentity          Claim = "feature_identity"
	ClaimWholeSourceCompleteness  Claim = "whole_source_completeness"
)

var ProhibitedClaims = []Claim{ClaimRuntimeExecution, ClaimCallbackInvocation, ClaimRepaint, ClaimFeatureIdentity, ClaimWholeSourceCompleteness}

type ProducerIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Endpoint struct {
	NodeID string       `json:"node_id"`
	Role   EndpointRole `json:"role"`
}

type SourceAnchor struct {
	URI, Revision, Blob                              string
	StartLine, StartCharacter, EndLine, EndCharacter int
}

type SemanticObservation struct {
	SchemaVersion            string
	Kind                     RelationKind
	Authority                Authority
	Provider                 ProducerIdentity
	From, To                 Endpoint
	Anchors                  []SourceAnchor
	Supports, DoesNotSupport []Claim
	RequestID, Timestamp     string
}

func endpointRoles(kind RelationKind) (EndpointRole, EndpointRole) {
	switch kind {
	case RelationCalls:
		return RoleCaller, RoleCallee
	case RelationBindsArgument:
		return RoleSourceExpression, RoleBoundArgument
	case RelationPassesCallback:
		return RoleCallableReference, RoleCallbackParameter
	case RelationInvokesTask:
		return RoleTaskInvocation, RoleTask
	case RelationTriggersReload:
		return RoleReloadRequest, RoleReloadTarget
	case RelationUpdatesState:
		return RoleStateProducer, RoleStateValue
	case RelationRendersFrom:
		return RoleRenderExpression, RoleReadValue
	default:
		return "", ""
	}
}

func (o SemanticObservation) Validate() error {
	if o.SchemaVersion != ObservationSchemaV1 {
		return fmt.Errorf("unsupported observation schema version %q", o.SchemaVersion)
	}
	if !o.Kind.Valid() {
		return fmt.Errorf("unknown relation kind %q", o.Kind)
	}
	if !o.Authority.Valid() {
		return fmt.Errorf("unknown authority %q", o.Authority)
	}
	if o.Kind == RelationCalls && o.Authority != AuthorityServerReported {
		return fmt.Errorf("CALLS must be server reported")
	}
	if o.Provider.Name == "" || o.Provider.Version == "" {
		return fmt.Errorf("provider identity requires name and version")
	}
	from, to := endpointRoles(o.Kind)
	if o.From.NodeID == "" || o.To.NodeID == "" || o.From.Role != from || o.To.Role != to {
		return fmt.Errorf("invalid endpoints for %s", o.Kind)
	}
	if o.Authority == AuthoritySourceDerivedAdapter && len(o.Anchors) == 0 {
		return fmt.Errorf("source-derived observation requires an anchor")
	}
	for _, anchor := range o.Anchors {
		if anchor.URI == "" || anchor.Revision == "" || anchor.Blob == "" {
			return fmt.Errorf("anchor requires uri, revision, and blob identity")
		}
		if anchor.StartLine < 0 || anchor.StartCharacter < 0 || anchor.EndLine < anchor.StartLine || anchor.EndCharacter < 0 {
			return fmt.Errorf("anchor has invalid range")
		}
	}
	prohibited := make(map[Claim]bool, len(ProhibitedClaims))
	for _, claim := range ProhibitedClaims {
		prohibited[claim] = true
	}
	for _, claim := range o.Supports {
		if prohibited[claim] {
			return fmt.Errorf("relation cannot support %q", claim)
		}
	}
	disclaimed := make(map[Claim]bool, len(o.DoesNotSupport))
	for _, claim := range o.DoesNotSupport {
		disclaimed[claim] = true
	}
	for _, claim := range ProhibitedClaims {
		if !disclaimed[claim] {
			return fmt.Errorf("missing prohibited claim %q", claim)
		}
	}
	return nil
}

func (o SemanticObservation) CanonicalID() (string, error) {
	if err := o.Validate(); err != nil {
		return "", err
	}
	anchors := append([]SourceAnchor(nil), o.Anchors...)
	sort.Slice(anchors, func(i, j int) bool {
		a, _ := json.Marshal(anchors[i])
		b, _ := json.Marshal(anchors[j])
		return string(a) < string(b)
	})
	identity := struct {
		SchemaVersion string
		Kind          RelationKind
		Authority     Authority
		Provider      ProducerIdentity
		From, To      Endpoint
		Anchors       []SourceAnchor
	}{o.SchemaVersion, o.Kind, o.Authority, o.Provider, o.From, o.To, anchors}
	data, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

type CoverageStatus string

const (
	CoverageCompleteWithinBounds CoverageStatus = "COMPLETE_WITHIN_BOUNDS"
	CoveragePartial              CoverageStatus = "PARTIAL"
	CoverageUnknown              CoverageStatus = "UNKNOWN"
)

type Coverage struct {
	Status               CoverageStatus
	Denominator, Covered []string
	CoveredCount         int
}

func (c Coverage) Validate() error {
	if c.Status != CoverageCompleteWithinBounds && c.Status != CoveragePartial && c.Status != CoverageUnknown {
		return fmt.Errorf("unknown coverage status %q", c.Status)
	}
	if len(c.Denominator) == 0 {
		return fmt.Errorf("coverage denominator is required")
	}
	denominator := map[string]bool{}
	for _, id := range c.Denominator {
		if id == "" || denominator[id] {
			return fmt.Errorf("invalid denominator member %q", id)
		}
		denominator[id] = true
	}
	seen := map[string]bool{}
	for _, id := range c.Covered {
		if !denominator[id] {
			return fmt.Errorf("covered member %q is outside denominator", id)
		}
		if seen[id] {
			return fmt.Errorf("duplicate covered member %q", id)
		}
		seen[id] = true
	}
	if c.CoveredCount != len(c.Covered) {
		return fmt.Errorf("covered count mismatch")
	}
	if c.Status == CoverageCompleteWithinBounds && len(c.Covered) != len(c.Denominator) {
		return fmt.Errorf("complete coverage does not exhaust denominator")
	}
	return nil
}

type FailureKind string

const (
	FailurePrepareReturnedNoItem   FailureKind = "PREPARE_RETURNED_NO_ITEM"
	FailureRelationNotSupported    FailureKind = "RELATION_NOT_SUPPORTED"
	FailureAdapterNotAvailable     FailureKind = "ADAPTER_NOT_AVAILABLE"
	FailureNoRelationsWithinBounds FailureKind = "NO_RELATIONS_WITHIN_BOUNDS"
	FailureTransportFailed         FailureKind = "TRANSPORT_FAILED"
	FailureTruncated               FailureKind = "TRUNCATED"
)

func (f FailureKind) Valid() bool {
	switch f {
	case FailurePrepareReturnedNoItem, FailureRelationNotSupported, FailureAdapterNotAvailable, FailureNoRelationsWithinBounds, FailureTransportFailed, FailureTruncated:
		return true
	default:
		return false
	}
}

func CanonicalContributingObservationIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one contributing observation is required")
	}
	result := append([]string(nil), ids...)
	sort.Strings(result)
	for i, id := range result {
		if id == "" {
			return nil, fmt.Errorf("empty contributing observation id")
		}
		if i > 0 && result[i-1] == id {
			return nil, fmt.Errorf("duplicate contributing observation id %q", id)
		}
	}
	return result, nil
}
