package adr0007sourcetextsearchv1

const (
	SchemaInputV1     = "lsp-trace.adr0007.source-text-search.input.v1"
	SchemaResultV1    = "lsp-trace.adr0007.source-text-search.result.v1"
	SchemaFailureV1   = "lsp-trace.adr0007.source-text-search.failure.v1"
	SchemaLocationV1  = "lsp-trace.adr0007.source-text-search.location-composition.v1"
	SchemaAdmissionV1 = "lsp-trace.adr0007.source-text-search.source-admission-binding.v1"
	SchemaFreezeV1    = "lsp-trace.adr0007.source-text-search.freeze.v1"
)

type SourceBinding struct {
	Path        string `json:"path"`
	Revision    string `json:"revision"`
	FileSHA256  string `json:"file_sha256"`
	ObjectID    string `json:"object_id"`
	AdmissionID string `json:"admission_id"`
	Seal        string `json:"seal"`
}

type Limits struct {
	MaxMatches     int `json:"max_matches"`
	MaxOutputBytes int `json:"max_output_bytes"`
	MaxWork        int `json:"max_work"`
	MaxPathBytes   int `json:"max_path_bytes"`
	MaxSourceBytes int `json:"max_source_bytes"`
}

type Input struct {
	SchemaVersion string        `json:"schema_version"`
	Query         string        `json:"query"`
	Source        SourceBinding `json:"source"`
	Limits        Limits        `json:"limits"`
}

type Range struct {
	ByteStart  int `json:"byte_start"`
	ByteEnd    int `json:"byte_end"`
	UTF16Start int `json:"utf16_start"`
	UTF16End   int `json:"utf16_end"`
}

type Match struct {
	Path        string `json:"path"`
	Revision    string `json:"revision"`
	FileSHA256  string `json:"file_sha256"`
	ObjectID    string `json:"object_id"`
	AdmissionID string `json:"admission_id"`
	Seal        string `json:"seal"`
	Range       Range  `json:"range"`
	Literal     string `json:"literal"`
}

type Accounting struct {
	Files       int `json:"files"`
	Bytes       int `json:"bytes"`
	QueryBytes  int `json:"query_bytes"`
	PathBytes   int `json:"path_bytes"`
	Matches     int `json:"matches"`
	OutputBytes int `json:"output_bytes"`
	Work        int `json:"work"`
}

type Result struct {
	SchemaVersion   string     `json:"schema_version"`
	Accepted        bool       `json:"accepted"`
	Authority       int        `json:"authority"`
	Completeness    string     `json:"completeness"`
	FeatureIdentity string     `json:"feature_identity"`
	Matches         []Match    `json:"matches"`
	Accounting      Accounting `json:"accounting"`
}

type Failure struct {
	SchemaVersion   string     `json:"schema_version"`
	Accepted        bool       `json:"accepted"`
	Authority       int        `json:"authority"`
	Completeness    string     `json:"completeness"`
	FeatureIdentity string     `json:"feature_identity"`
	Code            string     `json:"code"`
	Message         string     `json:"message"`
	Accounting      Accounting `json:"accounting"`
}
