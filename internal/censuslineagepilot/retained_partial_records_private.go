package censuslineagepilot

// Neutral historical record definitions only. These values describe retained
// observations; none constructs a runtime, scope guard, claim or admission.
import (
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

type ScopedRehearsalBudgetPrivate struct {
	ArtifactBytes int `json:"artifact_bytes"`
	TotalBytes    int `json:"total_bytes"`
	Objects       int `json:"objects"`
	StagingBytes  int `json:"staging_bytes"`
	ReadHashBytes int `json:"read_hash_bytes"`
}

type ScopedRehearsalConfigPrivate struct {
	Schema               string                                    `json:"schema"`
	InputRoots           [3]string                                 `json:"input_roots"`
	Locator              scopeRestartLocator                       `json:"restart_locator"`
	OutputRoots          [3]string                                 `json:"output_roots"`
	OutputIdentities     [3]publication.RetentionDirectoryIdentity `json:"output_identities"`
	Workspace            string                                    `json:"workspace"`
	CodeHEAD             string                                    `json:"executable_head"`
	Session              censusacquisition.SessionIdentity         `json:"session"`
	TrustDomain          string                                    `json:"trust_domain"`
	Profile              string                                    `json:"profile"`
	EnvironmentReference string                                    `json:"environment_reference"`
	Server               string                                    `json:"server"`
	Args                 []string                                  `json:"args"`
	Environment          []string                                  `json:"environment"`
	Budget               ScopedRehearsalBudgetPrivate              `json:"budget"`
}

type ScopedDiscoveryPolicyObservationPrivate struct {
	Name       string `json:"name"`
	MaxSymbols int    `json:"max_symbols"`
	MaxFiles   int    `json:"max_files"`
	MaxDepth   int    `json:"max_depth"`
	MaxTargets int    `json:"max_targets"`
}

type ScopedRehearsalRecordPrivate struct {
	Name           string                        `json:"name"`
	ID             string                        `json:"id"`
	Bytes          int                           `json:"bytes"`
	Phase          string                        `json:"phase"`
	Primary        string                        `json:"primary"`
	Backup         string                        `json:"backup"`
	PrimaryReceipt *publication.BoundFileReceipt `json:"primary_receipt,omitempty"`
	BackupReceipt  *publication.BoundFileReceipt `json:"backup_receipt,omitempty"`
	Verified       bool                          `json:"verified"`
	Error          string                        `json:"error,omitempty"`
}

type ScopedTerminalDiagnosticsObservationPrivate struct {
	Available bool                                            `json:"available"`
	Events    []sessionruntime.RetainedScopeDiagnosticPrivate `json:"events"`
	Overflow  bool                                            `json:"overflow"`
}

type ScopedPartialEffectiveBudgetObservationPrivate struct {
	Output                                                                           ScopedRehearsalBudgetPrivate
	LifetimeSeconds, RequestSeconds, Requests, CaptureBytes, Messages, ResponseBytes int
}

type ScopedLiveOutcomeObservationPrivate struct {
	Schema                string                                         `json:"schema"`
	Authority             int                                            `json:"authority"`
	Accepted              bool                                           `json:"accepted"`
	Completeness          string                                         `json:"completeness"`
	FeatureIdentity       string                                         `json:"feature_identity"`
	RevisionQualification string                                         `json:"revision_qualification"`
	Outcome               string                                         `json:"outcome"`
	AcquireCalls          int                                            `json:"acquire_calls"`
	Constituents          int                                            `json:"core_admitted_constituents"`
	Failure               string                                         `json:"failure,omitempty"`
	CloseFailure          string                                         `json:"store_proof_close_failure,omitempty"`
	Runtime               sessionruntime.SessionMetadata                 `json:"runtime"`
	EffectiveBudget       ScopedPartialEffectiveBudgetObservationPrivate `json:"effective_budget"`
	Records               []ScopedRehearsalRecordPrivate                 `json:"records"`
	DiscoveryPolicy       *ScopedDiscoveryPolicyObservationPrivate       `json:"discovery_policy,omitempty"`
	Diagnostics           *ScopedTerminalDiagnosticsObservationPrivate   `json:"diagnostics,omitempty"`
	OutcomeReceipt        *ScopedRehearsalRecordPrivate                  `json:"outcome_receipt,omitempty"`
}

type ScopedPartialOutcomeObservationPrivate struct {
	ScopedLiveOutcomeObservationPrivate
	Purpose                 string         `json:"purpose"`
	Admission               string         `json:"core_admission"`
	SourceCompleteness      string         `json:"source_completeness"`
	CanonicalComplete       bool           `json:"canonical_complete"`
	WithheldCount           int            `json:"policy_withheld_count"`
	SourceDispositionCounts map[string]int `json:"source_disposition_counts,omitempty"`
}
