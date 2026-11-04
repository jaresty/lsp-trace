package programcrepresentative

const (
	QualificationEnvelopeSchemaVersionV2 = "lsp-trace.program-c-representative-qualification.private.v2"
	QualificationReviewSchemaVersionV2   = "lsp-trace.program-c-representative-qualification-review.private.v2"
	DenominatorSchemaVersionV2           = "lsp-trace.program-c-representative-denominator.private.v2"
	DenominatorEntrySchemaVersionV2      = "lsp-trace.program-c-representative-denominator-entry.private.v2"
	LineageSchemaVersionV2               = "lsp-trace.program-c-representative-lineage.private.v2"
	LineageRecordSchemaVersionV2         = "lsp-trace.program-c-representative-lineage-record.private.v2"
)

type EvidenceHeadV2 struct {
	SchemaVersion         string                       `json:"schema_version"`
	Selector              string                       `json:"selector"`
	WorkflowKey           string                       `json:"workflow_key"`
	HeadKey               string                       `json:"head_key"`
	Generation            uint64                       `json:"generation"`
	PredecessorHead       string                       `json:"predecessor_head"`
	Subject               ArtifactBindingDTO           `json:"subject"`
	QualificationEnvelope ArtifactBindingDTO           `json:"qualification_envelope"`
	Artifact              ArtifactBindingDTO           `json:"artifact"`
	Denominator           ArtifactBindingDTO           `json:"denominator"`
	ClaimCeiling          QualificationClaimCeilingDTO `json:"claim_ceiling"`
}

type ReviewActorV2 struct {
	IdentityKind        string             `json:"identity_kind"`
	HarnessActorID      string             `json:"harness_actor_id"`
	AttributionEvidence ArtifactBindingDTO `json:"attribution_evidence"`
	Identity            string             `json:"identity"`
}
type ReviewAssignmentV2 struct {
	SchemaVersion  string             `json:"schema_version"`
	ActorKind      string             `json:"actor_kind"`
	HarnessActorID string             `json:"harness_actor_id"`
	ParentActorID  string             `json:"parent_actor_id"`
	Subject        ArtifactBindingDTO `json:"subject"`
	Scope          []string           `json:"scope"`
}
type ReviewTerminalV2 struct {
	SchemaVersion  string             `json:"schema_version"`
	ActorKind      string             `json:"actor_kind"`
	HarnessActorID string             `json:"harness_actor_id"`
	ParentActorID  string             `json:"parent_actor_id"`
	Subject        ArtifactBindingDTO `json:"subject"`
	Scope          []string           `json:"scope"`
	Verdict        string             `json:"verdict"`
}
type ReviewTranscriptSnapshotV2 struct {
	SchemaVersion           string             `json:"schema_version"`
	ActorKind               string             `json:"actor_kind"`
	HarnessActorID          string             `json:"harness_actor_id"`
	ParentActorID           string             `json:"parent_actor_id"`
	RawTranscriptSelector   string             `json:"raw_transcript_selector"`
	RawTranscriptSHA256     string             `json:"raw_transcript_sha256"`
	RawTranscriptByteLength uint64             `json:"raw_transcript_byte_length"`
	AssignmentSubject       ArtifactBindingDTO `json:"assignment_subject"`
	AssignmentScope         []string           `json:"assignment_scope"`
	TerminalSubject         ArtifactBindingDTO `json:"terminal_subject"`
	TerminalScope           []string           `json:"terminal_scope"`
	TerminalVerdict         string             `json:"terminal_verdict"`
	Authority               uint8              `json:"authority"`
	Accepted                bool               `json:"accepted"`
	Identity                string             `json:"identity"`
}
type ReviewTaskSeparationV2 struct {
	SchemaVersion           string             `json:"schema_version"`
	WriterSessionID         string             `json:"writer_session_id"`
	ReviewerTaskID          string             `json:"reviewer_task_id"`
	ReviewerParentSessionID string             `json:"reviewer_parent_session_id"`
	ReviewerIsolated        bool               `json:"reviewer_isolated"`
	Subject                 ArtifactBindingDTO `json:"subject"`
	ReviewScope             []string           `json:"review_scope"`
	Authority               uint8              `json:"authority"`
	Accepted                bool               `json:"accepted"`
	Identity                string             `json:"identity"`
}
type QualificationReviewV2 struct {
	SchemaVersion      string             `json:"schema_version"`
	Profile            string             `json:"profile"`
	Subject            ArtifactBindingDTO `json:"subject"`
	ReviewScope        []string           `json:"review_scope"`
	Verdict            string             `json:"verdict"`
	Writer             ReviewActorV2      `json:"writer"`
	Reviewer           ReviewActorV2      `json:"reviewer"`
	SeparationMethod   string             `json:"separation_method"`
	SeparationEvidence ArtifactBindingDTO `json:"separation_evidence"`
	Authority          uint8              `json:"authority"`
	Accepted           bool               `json:"accepted"`
	Identity           string             `json:"identity"`
}
type QualificationEnvelopeV2 struct {
	SchemaVersion            string             `json:"schema_version"`
	Profile                  string             `json:"profile"`
	Subject                  ArtifactBindingDTO `json:"subject"`
	IndependentReviewReceipt ArtifactBindingDTO `json:"independent_review_receipt"`
	Stage6Policy             ArtifactBindingDTO `json:"stage6_policy"`
	Stage6Limits             ArtifactBindingDTO `json:"stage6_limits"`
	Stage6Finalization       ArtifactBindingDTO `json:"stage6_finalization"`
	QualificationEpoch       uint64             `json:"qualification_epoch"`
	Authority                uint8              `json:"authority"`
	Accepted                 bool               `json:"accepted"`
	Identity                 string             `json:"identity"`
}
type RunTerminalV2 struct {
	RunID             string             `json:"run_id"`
	Seed              uint64             `json:"seed"`
	ReplayOrdinal     uint8              `json:"replay_ordinal"`
	Outcome           string             `json:"outcome"`
	CommunityArtifact ArtifactBindingDTO `json:"community_artifact"`
	CommunityCount    uint64             `json:"community_count"`
}
type ConstituentV2 struct {
	ConstituentOrdinal       int                `json:"constituent_ordinal"`
	Receipt                  ArtifactBindingDTO `json:"receipt"`
	PublicationSelector      string             `json:"publication_selector"`
	ConstituentSelector      string             `json:"constituent_selector"`
	NativeV5SHA256           string             `json:"native_v5_sha256"`
	NativeV5ByteLength       uint64             `json:"native_v5_byte_length"`
	InstabilitySHA256        string             `json:"instability_sha256"`
	InstabilityByteLength    uint64             `json:"instability_byte_length"`
	InstabilitySchemaVersion string             `json:"instability_schema_version"`
	ProgramCOutcome          string             `json:"program_c_outcome"`
	Disposition              string             `json:"disposition"`
	Classification           string             `json:"classification"`
	Runs                     []RunTerminalV2    `json:"runs"`
}
type DenominatorEntryV2 struct {
	SchemaVersion         string             `json:"schema_version"`
	ConstituentOrdinal    int                `json:"constituent_ordinal"`
	Receipt               ArtifactBindingDTO `json:"receipt"`
	Custody               CustodyIdentityDTO `json:"custody"`
	RunID                 string             `json:"run_id"`
	Seed                  uint64             `json:"seed"`
	ReplayOrdinal         uint8              `json:"replay_ordinal"`
	CommunityOrdinal      int                `json:"community_ordinal"`
	CommunityArtifact     ArtifactBindingDTO `json:"community_artifact"`
	Mechanics             ArtifactBindingDTO `json:"mechanics"`
	QualificationEnvelope ArtifactBindingDTO `json:"qualification_envelope"`
	Authority             uint8              `json:"authority"`
	Accepted              bool               `json:"accepted"`
	Identity              string             `json:"identity"`
}
type DenominatorAccountingV2 struct {
	P                    uint64 `json:"p"`
	C                    uint64 `json:"c"`
	E                    uint64 `json:"e"`
	R                    uint64 `json:"r"`
	I                    uint64 `json:"i"`
	Evaluated            uint64 `json:"evaluated"`
	Excluded             uint64 `json:"excluded"`
	Unresolved           uint64 `json:"unresolved"`
	TerminalRuns         uint64 `json:"terminal_runs"`
	EligibleRuns         uint64 `json:"eligible_runs"`
	EmptyRuns            uint64 `json:"empty_runs"`
	ResourceExcludedRuns uint64 `json:"resource_excluded_runs"`
	D                    uint64 `json:"d"`
	G                    uint64 `json:"g"`
}
type DenominatorV2 struct {
	SchemaVersion         string                           `json:"schema_version"`
	Profile               string                           `json:"profile"`
	Subject               ArtifactBindingDTO               `json:"subject"`
	QualificationEnvelope ArtifactBindingDTO               `json:"qualification_envelope"`
	CorpusIdentity        CorpusIdentityDTO                `json:"corpus_identity"`
	WorkflowIdentity      QualificationWorkflowIdentityDTO `json:"workflow_identity"`
	TerminalProgramCHead  TerminalProgramCHeadDTO          `json:"terminal_program_c_head"`
	Constituents          []ConstituentV2                  `json:"constituents"`
	Entries               []DenominatorEntryV2             `json:"entries"`
	Accounting            DenominatorAccountingV2          `json:"accounting"`
	ClaimCeiling          QualificationClaimCeilingDTO     `json:"claim_ceiling"`
	Authority             uint8                            `json:"authority"`
	Accepted              bool                             `json:"accepted"`
	Identity              string                           `json:"identity"`
}
type LineageRecordV2 struct {
	SchemaVersion            string             `json:"schema_version"`
	DenominatorEntryIdentity string             `json:"denominator_entry_identity"`
	Mechanics                ArtifactBindingDTO `json:"mechanics"`
	Result                   ArtifactBindingDTO `json:"result"`
	QualificationEnvelope    ArtifactBindingDTO `json:"qualification_envelope"`
	Authority                uint8              `json:"authority"`
	Accepted                 bool               `json:"accepted"`
	Identity                 string             `json:"identity"`
}
type LineageAccountingV2 struct {
	D                      uint64 `json:"d"`
	TerminalOuterResults   uint64 `json:"terminal_outer_results"`
	UnresolvedOuterResults uint64 `json:"unresolved_outer_results"`
	G                      uint64 `json:"g"`
	SelectedGroups         uint64 `json:"selected_groups"`
	NoCallsGroups          uint64 `json:"no_calls_groups"`
	CallsIneligibleGroups  uint64 `json:"calls_ineligible_groups"`
	UnresolvedGroups       uint64 `json:"unresolved_groups"`
}
type LineageV2 struct {
	SchemaVersion         string                       `json:"schema_version"`
	Profile               string                       `json:"profile"`
	Denominator           ArtifactBindingDTO           `json:"denominator"`
	QualificationEnvelope ArtifactBindingDTO           `json:"qualification_envelope"`
	Records               []LineageRecordV2            `json:"records"`
	Accounting            LineageAccountingV2          `json:"accounting"`
	ClaimCeiling          QualificationClaimCeilingDTO `json:"claim_ceiling"`
	Authority             uint8                        `json:"authority"`
	Accepted              bool                         `json:"accepted"`
	Identity              string                       `json:"identity"`
}
