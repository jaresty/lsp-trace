package adr0007sourcetextsearchv2private

const (
	SchemaRequestV2  = "lsp-trace.adr0007.source-text-search.request.private.v2"
	SchemaResultV2   = "lsp-trace.adr0007.source-text-search.result.private.v2"
	SchemaPolicyV2   = "lsp-trace.adr0007.source-text-search.policy.private.v2"
	SchemaLimitsV2   = "lsp-trace.adr0007.source-text-search.limits.private.v2"
	SchemaAccounting = "lsp-trace.adr0007.source-text-search.accounting.private.v2"
	SchemaBindingV2  = "lsp-trace.adr0007.source-text-search.source-binding.private.v2"
	SchemaCustodyV2  = "lsp-trace.adr0007.source-text-search.custody.private.v2"
	SchemaReplayV2   = "lsp-trace.adr0007.source-text-search.replay.private.v2"
	SchemaFreezeV2   = "lsp-trace.adr0007.source-text-search.freeze.private.v2"
)

type Policy struct {
	SchemaVersion         string `json:"schema_version"`
	LiteralMode           string `json:"literal_mode"`
	AllowRegex            bool   `json:"allow_regex"`
	AllowFuzzy            bool   `json:"allow_fuzzy"`
	AllowToken            bool   `json:"allow_token"`
	AllowRank             bool   `json:"allow_rank"`
	AllowModel            bool   `json:"allow_model"`
	AllowBackendSemantics bool   `json:"allow_backend_semantics"`
}

type Limits struct {
	SchemaVersion  string `json:"schema_version"`
	MaxFiles       uint64 `json:"max_files"`
	MaxMatches     uint64 `json:"max_matches"`
	MaxWork        uint64 `json:"max_work"`
	MaxOutputBytes uint64 `json:"max_output_bytes"`
	MaxSourceBytes uint64 `json:"max_source_bytes"`
	MaxPathBytes   uint64 `json:"max_path_bytes"`
}

type SourceBinding struct {
	SchemaVersion    string `json:"schema_version"`
	Path             string `json:"path"`
	RepositoryCommit string `json:"repositoryCommit"`
	Revision         string `json:"revision"`
	FileSHA256       string `json:"file_sha256"`
	GitBlobSHA1      string `json:"gitBlobSha1"`
	ObjectID         string `json:"object_id"`
	AdmissionSchema  string `json:"admission_schema"`
	AdmissionID      string `json:"admission_id"`
}

type Cancel struct {
	Token     string `json:"token"`
	PollAfter uint64 `json:"poll_after"`
}

type Request struct {
	SchemaVersion    string          `json:"schema_version"`
	Query            string          `json:"query"`
	Sources          []SourceBinding `json:"sources"`
	Policy           Policy          `json:"policy"`
	Limits           Limits          `json:"limits"`
	Cancel           Cancel          `json:"cancel"`
	DeadlineUnixNano uint64          `json:"deadline_unix_nano"`
}

type Match struct {
	SchemaVersion  string `json:"schema_version"`
	Path           string `json:"path"`
	ByteStart      uint64 `json:"byte_start"`
	ByteEnd        uint64 `json:"byte_end"`
	LineStart      uint64 `json:"line_start"`
	CharacterStart uint64 `json:"character_start"`
	LineEnd        uint64 `json:"line_end"`
	CharacterEnd   uint64 `json:"character_end"`
	Literal        string `json:"literal"`
}

type Accounting struct {
	SchemaVersion   string            `json:"schema_version"`
	JFiles          uint64            `json:"J_files"`
	QQueryBytes     uint64            `json:"Q_query_bytes"`
	PPathBytes      uint64            `json:"P_path_bytes"`
	SSourceBytes    uint64            `json:"S_source_bytes"`
	TScannedTuples  uint64            `json:"T_scanned_tuples"`
	MMatches        uint64            `json:"M_matches"`
	RRanges         uint64            `json:"R_ranges"`
	UUTF16Units     uint64            `json:"U_utf16_units"`
	BOutputBytes    uint64            `json:"B_output_bytes"`
	WWork           uint64            `json:"W_work"`
	FailureCounters map[string]uint64 `json:"failure_counters"`
}

type Custody struct {
	SchemaVersion string `json:"schema_version"`
	Terminal      bool   `json:"terminal"`
	AttemptID     string `json:"attempt_id"`
	ResultID      string `json:"result_id"`
	ExactlyOnce   bool   `json:"exactly_once"`
}
type Replay struct {
	SchemaVersion          string `json:"schema_version"`
	CanonicalRequestSHA256 string `json:"canonical_request_sha256"`
	CanonicalResultSHA256  string `json:"canonical_result_sha256"`
	ToolingIdentitySHA256  string `json:"tooling_identity_sha256"`
}

type Result struct {
	SchemaVersion   string     `json:"schema_version"`
	Accepted        bool       `json:"accepted"`
	Authority       uint64     `json:"authority"`
	Completeness    string     `json:"completeness"`
	FeatureIdentity string     `json:"featureIdentity"`
	Matches         []Match    `json:"matches"`
	Accounting      Accounting `json:"accounting"`
	Custody         Custody    `json:"custody"`
	Replay          Replay     `json:"replay"`
}
