// Package adr0007locationv1 implements a private, non-dispatching ADR0007 design contract.
package adr0007locationv1

import "lsp-trace/internal/sourceadmissionv1"

const Schema = "lsp-trace.adr0007.location-intersection.private.v1"

type SelectorKind string

const (
	ExactFile  SelectorKind = "EXACT_FILE"
	RangeUnion SelectorKind = "RANGE_UNION"
	PathPrefix SelectorKind = "PATH_PREFIX"
)

type Relation string

const (
	Intersects  Relation = "INTERSECTS"
	ContainedBy Relation = "CONTAINED_BY"
	Contains    Relation = "CONTAINS"
)

type OperationOutcome string

const (
	Complete                   OperationOutcome = "COMPLETE"
	SourceAdmissionUnavailable OperationOutcome = "SOURCE_ADMISSION_UNAVAILABLE"
	SourceAdmissionMismatch    OperationOutcome = "SOURCE_ADMISSION_MISMATCH"
	InvalidRequest             OperationOutcome = "INVALID_REQUEST"
	InvalidSelector            OperationOutcome = "INVALID_SELECTOR"
	InvalidRange               OperationOutcome = "INVALID_RANGE"
	Cancelled                  OperationOutcome = "CANCELLED"
	Timeout                    OperationOutcome = "TIMEOUT"
	ResourceLimit              OperationOutcome = "RESOURCE_LIMIT"
	BackendFailure             OperationOutcome = "BACKEND_FAILURE"
	PolicyMismatch             OperationOutcome = "POLICY_MISMATCH"
)

type MemberOutcomeKind string

const (
	Eligible            MemberOutcomeKind = "ELIGIBLE"
	Ineligible          MemberOutcomeKind = "INELIGIBLE"
	UnavailableLocation MemberOutcomeKind = "UNAVAILABLE_LOCATION"
	InvalidLocation     MemberOutcomeKind = "INVALID_LOCATION"
	DuplicateMember     MemberOutcomeKind = "DUPLICATE_MEMBER"
	FilteredByPolicy    MemberOutcomeKind = "FILTERED_BY_POLICY"
)

type Position struct{ Line, Character uint32 }
type Range struct{ Start, End Position }
type PathRanges struct {
	Path   string
	Ranges []Range
}
type Selector struct {
	Kind        SelectorKind
	Path        string
	Union       []PathRanges
	FrozenPaths []string
}
type Request struct {
	ID              string
	Relation        Relation
	Selector        Selector
	AdmissionDigest string
	TopK            int
	Members         []Member
	PolicyDigest    string
}
type Member struct {
	ID, Path, Revision, FileDigest, ObjectDigest string
	Ranges                                       []Range
	Available, PolicyAllowed                     bool
	Score                                        float64
}
type Witness struct {
	Path                                        string
	SelectorRange, CandidateRange, Intersection Range
	Revision, FileDigest, ObjectDigest          string
}
type MemberOutcome struct {
	Ordinal   int
	MemberID  string
	Outcome   MemberOutcomeKind
	Witnesses []Witness
}
type RankedResult struct {
	Ordinal  int
	MemberID string
	Score    float64
}
type Accounting struct{ Input, Eligible, Ineligible, Unavailable, Invalid, Duplicate, Filtered, Ranked, Witnesses, Work int }
type Result struct {
	Schema, RequestID string
	Outcome           OperationOutcome
	Members           []MemberOutcome
	Ranked            []RankedResult
	Accounting        Accounting
	Detail            string
}
type Limits struct{ MaxRequestBytes, MaxMembers, MaxPaths, MaxRanges, MaxPrefixExpansion, MaxWitnesses, MaxSourceBytes, MaxOutputBytes, MaxWork int }
type Control interface {
	Cancelled() bool
	DeadlineExceeded() bool
}
type Options struct {
	Limits               Limits
	Control              Control
	ExpectedPolicyDigest string
}
type Admission = sourceadmissionv1.Binding
