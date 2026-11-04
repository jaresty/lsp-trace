package programcrepresentative

const (
	CompositionSchemaVersionV2          = "lsp-trace.program-c-representative-composition.private.v2"
	GroupingSchemaVersionV2             = "lsp-trace.program-c-representative-grouping.private.v2"
	DerivedQualificationSchemaVersionV2 = "lsp-trace.program-c-representative-derived-qualification.private.v2"
	ResultSchemaVersionV2               = "lsp-trace.program-c-representative-result.private.v2"
	Stage6PolicySchemaVersionV2         = "lsp-trace.program-c-representative-stage6-policy.private.v2"
	Stage6LimitsSchemaVersionV2         = "lsp-trace.program-c-representative-stage6-limits.private.v2"
	Stage6FinalizationSchemaVersionV2   = "lsp-trace.program-c-representative-stage6-finalization.private.v2"
	AcceptedDownstreamAmendmentSHA256V2 = "sha256:5a68281152c8345eb8251e720c10b153112d912f3ca344376a15bd098808bb4c"
)

type CompositionV2 struct {
	SchemaVersion         string                 `json:"schema_version"`
	Profile               string                 `json:"profile"`
	Mechanics             MechanicsIdentityDTOv2 `json:"mechanics"`
	Custody               CustodyIdentityDTO     `json:"custody"`
	Policy                ArtifactBindingDTO     `json:"policy"`
	QualificationEnvelope ArtifactBindingDTO     `json:"qualification_envelope"`
	Authority             uint8                  `json:"authority"`
	Accepted              bool                   `json:"accepted"`
	Identity              string                 `json:"identity"`
}
type GroupingV2 struct {
	SchemaVersion    string                        `json:"schema_version"`
	Profile          string                        `json:"profile"`
	Composition      ArtifactBindingDTO            `json:"composition"`
	Limits           ArtifactBindingDTO            `json:"limits"`
	Algorithm        string                        `json:"algorithm"`
	NumericContract  string                        `json:"numeric_contract"`
	SelectionContext SelectionContextIdentityDTOv2 `json:"selection_context"`
	Authority        uint8                         `json:"authority"`
	Accepted         bool                          `json:"accepted"`
	Identity         string                        `json:"identity"`
}
type DerivedQualificationV2 struct {
	SchemaVersion string             `json:"schema_version"`
	Profile       string             `json:"profile"`
	Composition   ArtifactBindingDTO `json:"composition"`
	Grouping      ArtifactBindingDTO `json:"grouping"`
	Envelope      ArtifactBindingDTO `json:"envelope"`
	ValidFrom     uint64             `json:"valid_from"`
	ValidThrough  uint64             `json:"valid_through"`
	Authority     uint8              `json:"authority"`
	Accepted      bool               `json:"accepted"`
	Identity      string             `json:"identity"`
}
type PredecessorV2 struct {
	OccurrenceID string   `json:"occurrence_id"`
	RelationID   string   `json:"relation_id"`
	CallerID     string   `json:"caller_id"`
	TargetID     string   `json:"target_id"`
	CallSite     RangeDTO `json:"call_site"`
}
type RecordV2 struct {
	Outcome              string          `json:"outcome"`
	SelectedNode         string          `json:"selected_node"`
	Distance             int             `json:"distance"`
	SCCMembers           []string        `json:"scc_members"`
	IncomingPredecessors []PredecessorV2 `json:"incoming_predecessors"`
}
type GroupResultV2 struct {
	InvocationSeedOrdinal int      `json:"invocation_seed_ordinal"`
	SeedGroupIdentity     string   `json:"seed_group_identity"`
	Record                RecordV2 `json:"record"`
}
type GroupAccountingV2 struct {
	Expected        uint64 `json:"expected"`
	Selected        uint64 `json:"selected"`
	NoCalls         uint64 `json:"no_calls"`
	CallsIneligible uint64 `json:"calls_ineligible"`
	Unresolved      uint64 `json:"unresolved"`
}
type ResultV2 struct {
	SchemaVersion   string                 `json:"schema_version"`
	Profile         string                 `json:"profile"`
	Composition     CompositionV2          `json:"composition"`
	Grouping        GroupingV2             `json:"grouping"`
	Qualification   DerivedQualificationV2 `json:"qualification"`
	Terminal        string                 `json:"terminal"`
	GroupResults    []GroupResultV2        `json:"group_results"`
	GroupAccounting GroupAccountingV2      `json:"group_accounting"`
	Authority       uint8                  `json:"authority"`
	Accepted        bool                   `json:"accepted"`
	Identity        string                 `json:"identity"`
}
type Stage6PolicyV2 struct {
	SchemaVersion               string   `json:"schema_version"`
	Profile                     string   `json:"profile"`
	AdmittedKind                string   `json:"admitted_kind"`
	AdmittedDisposition         string   `json:"admitted_disposition"`
	SelfLoopDisposition         string   `json:"self_loop_disposition"`
	ContextNodeScope            string   `json:"context_node_scope"`
	RelationOccurrenceClosure   string   `json:"relation_occurrence_closure"`
	GraphDirection              string   `json:"graph_direction"`
	StronglyConnectedComponents string   `json:"strongly_connected_components"`
	PreparedRootDistance        string   `json:"prepared_root_distance"`
	CandidateScope              string   `json:"candidate_scope"`
	PreparedRootScope           string   `json:"prepared_root_scope"`
	SelectedSCCMembersScope     string   `json:"selected_scc_members_scope"`
	PredecessorScope            string   `json:"predecessor_scope"`
	NoCallsOutcome              string   `json:"no_calls_outcome"`
	SelectionFailureOutcome     string   `json:"selection_failure_outcome"`
	SelectedOutcome             string   `json:"selected_outcome"`
	SelectionOrder              []string `json:"selection_order"`
	PredecessorOrder            []string `json:"predecessor_order"`
	OuterTerminalRule           string   `json:"outer_terminal_rule"`
	GroupMappingRule            string   `json:"group_mapping_rule"`
	NoOuterWinner               bool     `json:"no_outer_winner"`
	Authority                   uint8    `json:"authority"`
	Accepted                    bool     `json:"accepted"`
	Identity                    string   `json:"identity"`
}
type Stage6LimitsV2 struct {
	SchemaVersion          string `json:"schema_version"`
	Profile                string `json:"profile"`
	MaxCommunities         uint64 `json:"max_communities"`
	MaxMembers             uint64 `json:"max_members"`
	MaxOccurrences         uint64 `json:"max_occurrences"`
	MaxCandidates          uint64 `json:"max_candidates"`
	MaxSeedGroups          uint64 `json:"max_seed_groups"`
	MaxMemberships         uint64 `json:"max_memberships"`
	MaxMembershipsPerGroup uint64 `json:"max_memberships_per_group"`
	MaxGroupOccurrences    uint64 `json:"max_group_occurrences"`
	MaxSumGroupOccurrences uint64 `json:"max_sum_group_occurrences"`
	MaxStringBytes         uint64 `json:"max_string_bytes"`
	MaxOuterWork           uint64 `json:"max_outer_work"`
	MaxRunSourceWork       uint64 `json:"max_run_source_work"`
	MaxCanonicalBytes      uint64 `json:"max_canonical_bytes"`
	Authority              uint8  `json:"authority"`
	Accepted               bool   `json:"accepted"`
	Identity               string `json:"identity"`
}
type Stage6FinalizationV2 struct {
	SchemaVersion         string   `json:"schema_version"`
	Profile               string   `json:"profile"`
	CompositionSchema     string   `json:"composition_schema"`
	GroupingSchema        string   `json:"grouping_schema"`
	QualificationSchema   string   `json:"qualification_schema"`
	ResultSchema          string   `json:"result_schema"`
	MechanicsSchema       string   `json:"mechanics_schema"`
	ContextSchema         string   `json:"context_schema"`
	CompositionMappings   []string `json:"composition_mappings"`
	GroupingMappings      []string `json:"grouping_mappings"`
	GroupMappings         []string `json:"group_mappings"`
	QualificationMappings []string `json:"qualification_mappings"`
	FinalizationOrder     []string `json:"finalization_order"`
	Authority             uint8    `json:"authority"`
	Accepted              bool     `json:"accepted"`
	Identity              string   `json:"identity"`
}
