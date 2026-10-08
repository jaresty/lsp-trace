package adr0007sourcetextsearchv4private

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

const (
	AttemptSchema          = "lsp-trace.adr0007.source-text-search.attempt.private.v4"
	RequestSchema          = "lsp-trace.adr0007.source-text-search.request.private.v4"
	PolicySchema           = "lsp-trace.adr0007.source-text-search.policy.private.v4"
	LimitsSchema           = "lsp-trace.adr0007.source-text-search.limits.private.v4"
	LocationPinSchema      = "lsp-trace.adr0007.source-text-search.location-pin.private.v4"
	ExecutionControlSchema = "lsp-trace.adr0007.source-text-search.execution-control.private.v4"
	SourceInputSchema      = "lsp-trace.adr0007.source-text-search.source-input.private.v4"
	TerminalSchema         = "lsp-trace.adr0007.source-text-search.terminal.private.v4"
	AccountingSchema       = "lsp-trace.adr0007.source-text-search.accounting.private.v4"
	CustodySchema          = "lsp-trace.adr0007.source-text-search.custody.private.v4"
	ReplaySchema           = "lsp-trace.adr0007.source-text-search.replay.private.v4"
	AdmissionRecordSchema  = "lsp-trace.adr0007.source-text-search.admission-record.private.v4"
	FreezeBindingSchema    = "lsp-trace.adr0007.source-text-search.external-freeze-binding.private.v1"
	zeroSHA                = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

type Attempt struct {
	SchemaVersion         string                 `json:"schema_version"`
	AttemptID             string                 `json:"attempt_id"`
	Request               Request                `json:"request"`
	ExecutionControl      ExecutionControl       `json:"execution_control"`
	SourceInputs          []SourceInput          `json:"source_inputs"`
	ExternalFreezeBinding *ExternalFreezeBinding `json:"external_freeze_binding,omitempty"`
	TestControl           *TestControl           `json:"test_control,omitempty"`
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
	SchemaVersion        string             `json:"schema_version"`
	Operation            string             `json:"operation"`
	ExecutedLocation     bool               `json:"executedLocation"`
	DesignCommit         string             `json:"design_commit"`
	DesignRootSha256     string             `json:"design_root_sha256"`
	ExecutionCommit      string             `json:"execution_commit"`
	SealCommit           string             `json:"seal_commit"`
	FinalSealSha256      string             `json:"final_seal_sha256"`
	SourceAdmissionPin   SourceAdmissionPin `json:"source_admission_pin"`
	CompleteLocationPins []string           `json:"complete_location_pins"`
}
type SourceAdmissionPin struct {
	RepositoryCommit string   `json:"repository_commit"`
	Path             string   `json:"path"`
	Bytes            uint64   `json:"bytes"`
	SHA256           string   `json:"sha256"`
	GitBlobSHA1      string   `json:"git_blob_sha1"`
	Symbols          []string `json:"symbols"`
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
type ExternalFreezeBinding struct {
	SchemaVersion        string `json:"schema_version"`
	Mode                 string `json:"mode"`
	FreezeRootSha256     string `json:"freeze_root_sha256"`
	DesignIdentitySHA256 string `json:"design_identity_sha256"`
}
type TestControl struct {
	SchemaVersion       string `json:"schema_version"`
	InitialBOutputBytes uint64 `json:"initial_B_output_bytes"`
}

type TerminalResult struct {
	SchemaVersion       string          `json:"schema_version"`
	Terminal            string          `json:"terminal"`
	Attempt             TerminalAttempt `json:"attempt"`
	Request             TerminalRequest `json:"request"`
	Admission           Admission       `json:"admission"`
	Sources             []Source        `json:"sources"`
	Matches             []Match         `json:"matches"`
	Positions           []Position      `json:"positions"`
	RangeUnionCandidate *Candidate      `json:"range_union_candidate"`
	Accounting          Accounting      `json:"accounting"`
	Failure             *Failure        `json:"failure"`
	Custody             Custody         `json:"custody"`
	Replay              Replay          `json:"replay"`
	Payload             Payload         `json:"payload"`
}
type TerminalAttempt struct {
	AttemptID    string `json:"attempt_id"`
	MalformedRaw bool   `json:"malformed_raw"`
}
type TerminalRequest struct {
	Query string `json:"query"`
}
type Admission struct {
	Completed         bool     `json:"completed"`
	AdmissionDigest   string   `json:"admission_digest"`
	AdmittedSourceIDs []string `json:"admitted_source_ids"`
}
type Source struct {
	SourceID      string `json:"source_id"`
	LogicalURI    string `json:"logical_uri"`
	PathBytes     uint64 `json:"path_bytes"`
	ByteLength    uint64 `json:"byte_length"`
	ContentSHA256 string `json:"content_sha256"`
	Ordinal       uint64 `json:"ordinal"`
}
type Match struct {
	MatchID   string `json:"match_id"`
	SourceID  string `json:"source_id"`
	PathBytes uint64 `json:"path_bytes"`
	StartByte uint64 `json:"start_byte"`
	EndByte   uint64 `json:"end_byte"`
	Ordinal   uint64 `json:"ordinal"`
	Literal   string `json:"literal"`
}
type Position struct {
	MatchID             string `json:"match_id"`
	StartLine           uint64 `json:"start_line"`
	StartCharacterUTF16 uint64 `json:"start_character_utf16"`
	EndLine             uint64 `json:"end_line"`
	EndCharacterUTF16   uint64 `json:"end_character_utf16"`
}
type Candidate struct {
	Operation             string              `json:"operation"`
	ExecutedLocation      bool                `json:"executedLocation"`
	CandidateOnly         bool                `json:"candidate_only"`
	AdmissionDigest       string              `json:"admission_digest"`
	MemberMatchIDs        []string            `json:"member_match_ids"`
	QualifiedLocationPins []QualifiedLocation `json:"qualified_location_pins"`
	CandidateDigest       string              `json:"candidate_digest"`
}
type QualifiedLocation struct {
	SourceID            string `json:"source_id"`
	StartByte           uint64 `json:"start_byte"`
	EndByte             uint64 `json:"end_byte"`
	StartLine           uint64 `json:"start_line"`
	StartCharacterUTF16 uint64 `json:"start_character_utf16"`
	EndLine             uint64 `json:"end_line"`
	EndCharacterUTF16   uint64 `json:"end_character_utf16"`
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
	Stage  string         `json:"stage"`
	Detail map[string]any `json:"detail"`
}
type Custody struct {
	SchemaVersion        string `json:"schema_version"`
	AttemptID            string `json:"attempt_id"`
	TerminalResultSHA256 string `json:"terminal_result_sha256"`
	TerminalSequence0    uint64 `json:"terminal_sequence0"`
	TerminalCount1       uint64 `json:"terminal_count1"`
}
type Replay struct {
	SchemaVersion          string `json:"schema_version"`
	CanonicalAttemptSHA256 string `json:"canonical_attempt_sha256"`
	AdmittedBindingSHA256  string `json:"admitted_binding_sha256"`
	TerminalPreimageSHA256 string `json:"terminal_preimage_sha256"`
	ToolingIdentitySHA256  string `json:"tooling_identity_sha256"`
	FreezeBindingSHA256    string `json:"freeze_binding_sha256"`
	PredecessorLockSHA256  string `json:"predecessor_lock_sha256"`
}
type Payload struct {
	PayloadDigest       string `json:"payload_digest"`
	FreezeBindingSHA256 string `json:"freeze_binding_sha256"`
}

type SourceTuple struct {
	Ordinal          uint64 `json:"ordinal"`
	Path             string `json:"path"`
	Revision         string `json:"revision"`
	FileDigest       string `json:"file_digest"`
	ObjectDigest     string `json:"object_digest"`
	SourceByteLength uint64 `json:"source_byte_length"`
}
type AdmissionRecord struct {
	SchemaVersion   string        `json:"schema_version"`
	AdmissionSchema string        `json:"admission_schema"`
	AdmissionDigest string        `json:"admission_digest"`
	OrderedSources  []SourceTuple `json:"ordered_sources"`
}
type Pos struct{ Line, Character uint64 }

func Canon(v any) []byte {
	b, _ := json.Marshal(v)
	var x any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	_ = dec.Decode(&x)
	var buf bytes.Buffer
	writeCanon(&buf, x)
	buf.WriteByte('\n')
	return buf.Bytes()
}
func writeCanon(buf *bytes.Buffer, v any) {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		b, _ := json.Marshal(x)
		buf.Write(b)
	case json.Number:
		buf.WriteString(x.String())
	case float64:
		buf.WriteString(fmt.Sprintf("%.0f", x))
	case []any:
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanon(buf, e)
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteByte(':')
			writeCanon(buf, x[k])
		}
		buf.WriteByte('}')
	}
}
