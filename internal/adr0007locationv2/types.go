// Package adr0007locationv2 is the private non-dispatching Location v2 design contract.
package adr0007locationv2

import "lsp-trace/internal/sourceadmissionv2"

const Schema = "lsp-trace.adr0007.location-intersection.private.v2"

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

type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type PathRanges struct {
	Path   string  `json:"path"`
	Ranges []Range `json:"ranges"`
}
type Selector struct {
	Kind        SelectorKind `json:"kind"`
	Path        string       `json:"path,omitempty"`
	Union       []PathRanges `json:"union,omitempty"`
	FrozenPaths []string     `json:"frozenPaths,omitempty"`
}
type Candidate struct {
	ID            string  `json:"id"`
	Path          string  `json:"path"`
	Revision      string  `json:"revision"`
	FileDigest    string  `json:"fileDigest"`
	ObjectDigest  string  `json:"objectDigest"`
	Ranges        []Range `json:"ranges"`
	Available     bool    `json:"available"`
	PolicyAllowed bool    `json:"policyAllowed"`
	Score         int64   `json:"score"`
}
type Request struct {
	Schema          string      `json:"schema"`
	ID              string      `json:"id"`
	Relation        Relation    `json:"relation"`
	Selector        Selector    `json:"selector"`
	AdmissionDigest string      `json:"admissionDigest"`
	TopK            int         `json:"topK"`
	Members         []Candidate `json:"members"`
	PolicyDigest    string      `json:"policyDigest"`
}
type Witness struct {
	Path           string `json:"path"`
	SelectorRange  Range  `json:"selectorRange"`
	CandidateRange Range  `json:"candidateRange"`
	Intersection   Range  `json:"intersection"`
	Revision       string `json:"revision"`
	FileDigest     string `json:"fileDigest"`
	ObjectDigest   string `json:"objectDigest"`
}
type MemberOutcome struct {
	Ordinal   int               `json:"ordinal"`
	MemberID  string            `json:"memberId"`
	Outcome   MemberOutcomeKind `json:"outcome"`
	Witnesses []Witness         `json:"witnesses"`
}
type RankedResult struct {
	Ordinal  int    `json:"ordinal"`
	MemberID string `json:"memberId"`
	Score    int64  `json:"score"`
}
type Counters struct {
	Input               int    `json:"input"`
	Eligible            int    `json:"eligible"`
	Ineligible          int    `json:"ineligible"`
	UnavailableLocation int    `json:"unavailableLocation"`
	InvalidLocation     int    `json:"invalidLocation"`
	DuplicateMember     int    `json:"duplicateMember"`
	FilteredByPolicy    int    `json:"filteredByPolicy"`
	Ranked              int    `json:"ranked"`
	Witnesses           int    `json:"witnesses"`
	Work                uint64 `json:"work"`
	SourceBytes         uint64 `json:"sourceBytes"`
	OutputBytes         uint64 `json:"outputBytes"`
}
type Result struct {
	Schema    string           `json:"schema"`
	RequestID string           `json:"requestId"`
	Outcome   OperationOutcome `json:"outcome"`
	Members   []MemberOutcome  `json:"members"`
	Ranked    []RankedResult   `json:"ranked"`
	Counters  Counters         `json:"counters"`
	Detail    string           `json:"detail"`
}
type Limits struct {
	MaxRequestBytes, MaxMembers, MaxPaths, MaxSelectorRanges, MaxMemberRanges, MaxPrefixExpansion, MaxWitnesses, MaxSourceBytes, MaxOutputBytes uint64
	MaxWork                                                                                                                                     uint64
}
type Control interface {
	Cancelled() bool
	DeadlineExceeded() bool
}
type Options struct {
	Limits               Limits
	Control              Control
	ExpectedPolicyDigest string
}
type Admission = sourceadmissionv2.Binding
