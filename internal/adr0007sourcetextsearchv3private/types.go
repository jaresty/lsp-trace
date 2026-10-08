package adr0007sourcetextsearchv3private

import "encoding/json"

type Attempt struct {
	SchemaVersion    string           `json:"schema_version"`
	AttemptID        string           `json:"attempt_id"`
	Request          Request          `json:"request"`
	ExecutionControl ExecutionControl `json:"execution_control"`
	SourceInputs     []SourceInput    `json:"source_inputs"`
}
type Request struct {
	SchemaVersion string      `json:"schema_version"`
	Query         string      `json:"query"`
	Sources       []SourceRef `json:"sources"`
	Policy        Policy      `json:"policy"`
	Limits        Limits      `json:"limits"`
	LocationPin   LocationPin `json:"location_pin"`
}
type SourceRef struct {
	Ordinal      uint64 `json:"ordinal"`
	Path         string `json:"path"`
	Revision     string `json:"revision"`
	FileDigest   string `json:"file_digest"`
	ObjectDigest string `json:"object_digest"`
}
type SourceInput struct {
	SchemaVersion string `json:"schema_version"`
	Ordinal       uint64 `json:"ordinal"`
	Path          string `json:"path"`
	Revision      string `json:"revision"`
	FileDigest    string `json:"file_digest"`
	ObjectDigest  string `json:"object_digest"`
	BytesBase64   string `json:"bytes_base64"`
}
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
	MaxTotalBytes  uint64 `json:"max_total_bytes"`
	MaxPathBytes   uint64 `json:"max_path_bytes"`
}
type LocationPin struct {
	SchemaVersion    string `json:"schema_version"`
	Operation        string `json:"operation"`
	ExecutedLocation bool   `json:"executedLocation"`
	DesignCommit     string `json:"design_commit"`
	DesignRootSha256 string `json:"design_root_sha256"`
	ExecutionCommit  string `json:"execution_commit"`
	SealCommit       string `json:"seal_commit"`
	FinalSealSha256  string `json:"final_seal_sha256"`
}
type ExecutionControl struct {
	SchemaVersion string        `json:"schema_version"`
	Observations  []Observation `json:"observations"`
}
type Observation struct {
	PollIndex       uint64 `json:"poll_index"`
	Cancelled       bool   `json:"cancelled"`
	DeadlineExpired bool   `json:"deadline_expired"`
}
type SourceTuple struct {
	Ordinal          uint64 `json:"ordinal"`
	Path             string `json:"path"`
	Revision         string `json:"revision"`
	FileDigest       string `json:"file_digest"`
	ObjectDigest     string `json:"object_digest"`
	SourceByteLength uint64 `json:"source_byte_length,omitempty"`
}
type AdmissionRecord struct {
	SchemaVersion   string        `json:"schema_version"`
	AdmissionSchema string        `json:"admission_schema"`
	AdmissionDigest string        `json:"admission_digest"`
	OrderedSources  []SourceTuple `json:"ordered_sources"`
}
type Pos struct {
	Line      uint64 `json:"line"`
	Character uint64 `json:"character"`
}
type ByteRange struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}
type LSPRange struct {
	Start Pos `json:"start"`
	End   Pos `json:"end"`
}
type Match struct {
	SchemaVersion string      `json:"schema_version"`
	Source        SourceTuple `json:"source"`
	ByteRange     ByteRange   `json:"byte_range"`
	LSPUTF16Range LSPRange    `json:"lsp_utf16_range"`
	Literal       string      `json:"literal"`
}
type RangeUnionCandidate struct {
	SchemaVersion    string  `json:"schema_version"`
	Operation        string  `json:"operation"`
	ExecutedLocation bool    `json:"executedLocation"`
	CandidateOnly    bool    `json:"candidate_only"`
	DesignCommit     string  `json:"design_commit"`
	DesignRootSha256 string  `json:"design_root_sha256"`
	ExecutionCommit  string  `json:"execution_commit"`
	SealCommit       string  `json:"seal_commit"`
	FinalSealSha256  string  `json:"final_seal_sha256"`
	AdmissionDigest  string  `json:"admission_digest"`
	Members          []Match `json:"members"`
	CandidateDigest  string  `json:"candidate_digest"`
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
type Failure struct {
	Code   string         `json:"code"`
	Detail map[string]any `json:"detail"`
}
type Custody struct {
	SchemaVersion        string `json:"schema_version"`
	AttemptID            string `json:"attempt_id"`
	TerminalResultSHA256 string `json:"terminal_result_sha256"`
	TerminalSequence     uint64 `json:"terminal_sequence"`
	TerminalCount        uint64 `json:"terminal_count"`
}
type Replay struct {
	SchemaVersion          string `json:"schema_version"`
	CanonicalAttemptSHA256 string `json:"canonical_attempt_sha256"`
	AdmittedBindingSHA256  string `json:"admitted_binding_sha256"`
	TerminalPreimageSHA256 string `json:"terminal_preimage_sha256"`
	ToolingIdentitySHA256  string `json:"tooling_identity_sha256"`
	FreezeRootSha256       string `json:"freeze_root_sha256"`
	PredecessorLockSHA256  string `json:"predecessor_lock_sha256"`
}
type TerminalResult struct {
	SchemaVersion       string               `json:"schema_version"`
	AttemptID           string               `json:"attempt_id"`
	TerminalSequence    uint64               `json:"terminal_sequence"`
	Outcome             string               `json:"outcome"`
	Failure             *Failure             `json:"failure"`
	Authority           uint64               `json:"authority"`
	Accepted            bool                 `json:"accepted"`
	Completeness        string               `json:"completeness"`
	FeatureIdentity     string               `json:"featureIdentity"`
	Admission           *AdmissionRecord     `json:"admission"`
	Matches             []Match              `json:"matches"`
	RangeUnionCandidate *RangeUnionCandidate `json:"range_union_candidate"`
	Accounting          Accounting           `json:"accounting"`
	Custody             Custody              `json:"custody"`
	Replay              Replay               `json:"replay"`
}

func Canon(v any) []byte { b, _ := json.Marshal(v); return append(b, '\n') }
