package adr0007v4contractvalidator

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const TerminalSchemaVersion = "lsp-trace.adr0007.source-text-search.terminal.private.v4"
const AccountingSchemaVersion = "lsp-trace.adr0007.source-text-search.accounting.private.v4"
const CustodySchemaVersion = "lsp-trace.adr0007.source-text-search.custody.private.v4"
const ReplaySchemaVersion = "lsp-trace.adr0007.source-text-search.replay.private.v4"
const CandidateOperation = "RANGE_UNION"
const zeroSHA = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
const MaxIterations = 64
const expectedPayloadManifestSHA = "sha256:94d7171a7cb8d0cd62d77401119d002d5c7627cb595e8cb670e75821a103fa7e"
const expectedToolingCensusSHA = "sha256:7e524efaec40550409eb2756f1b8e45a34dd7a6a94bcbb3a0886723fad2696cf"
const expectedPredecessorLockSHA = "sha256:63aead2715dcdaa49dcd48ad61e38d003ab2df59736a9f2494605eba69273015"

type Bundle struct {
	SchemaBytes              json.RawMessage   `json:"schema_bytes"`
	RawAttemptBytes          string            `json:"raw_attempt_bytes"`
	TerminalBytes            string            `json:"terminal_bytes"`
	AdmittedSourceBytes      map[string]string `json:"admitted_source_bytes"`
	AdmittedBindingBytes     string            `json:"admitted_binding_bytes"`
	ToolingManifestBytes     string            `json:"tooling_manifest_bytes"`
	PredecessorManifestBytes string            `json:"predecessor_manifest_bytes"`
	PayloadFreezeBytes       string            `json:"payload_freeze_binding_bytes"`
	WantErrorCode            string            `json:"want_error_code,omitempty"`
	WantErrorPath            string            `json:"want_error_path,omitempty"`
}

type VError struct{ Code, Path, Msg string }

func (e *VError) Error() string         { return e.Code + " " + e.Path + ": " + e.Msg }
func verr(code, path, msg string) error { return &VError{code, path, msg} }

type Terminal struct {
	SchemaVersion       string     `json:"schema_version"`
	Terminal            string     `json:"terminal"`
	Attempt             Attempt    `json:"attempt"`
	Request             Request    `json:"request"`
	Admission           Admission  `json:"admission"`
	Sources             []Source   `json:"sources"`
	Matches             []Match    `json:"matches"`
	Positions           []Position `json:"positions"`
	RangeUnionCandidate *Candidate `json:"range_union_candidate"`
	Accounting          Accounting `json:"accounting"`
	Failure             *Failure   `json:"failure"`
	Custody             Custody    `json:"custody"`
	Replay              Replay     `json:"replay"`
	Payload             Payload    `json:"payload"`
}
type Attempt struct {
	AttemptID    string `json:"attempt_id"`
	MalformedRaw bool   `json:"malformed_raw"`
}
type Request struct {
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
	Operation             string        `json:"operation"`
	ExecutedLocation      bool          `json:"executedLocation"`
	CandidateOnly         bool          `json:"candidate_only"`
	AdmissionDigest       string        `json:"admission_digest"`
	MemberMatchIDs        []string      `json:"member_match_ids"`
	QualifiedLocationPins []LocationPin `json:"qualified_location_pins"`
	CandidateDigest       string        `json:"candidate_digest"`
}
type LocationPin struct {
	SourceID            string `json:"source_id"`
	StartByte           uint64 `json:"start_byte"`
	EndByte             uint64 `json:"end_byte"`
	StartLine           uint64 `json:"start_line"`
	StartCharacterUTF16 uint64 `json:"start_character_utf16"`
	EndLine             uint64 `json:"end_line"`
	EndCharacterUTF16   uint64 `json:"end_character_utf16"`
}
type Accounting struct {
	SchemaVersion                                                                                                      string `json:"schema_version"`
	JFiles, QQueryBytes, PPathBytes, SSourceBytes, TScannedTuples, MMatches, RRanges, UUTF16Units, BOutputBytes, WWork uint64
	FailureCounters                                                                                                    map[string]uint64 `json:"failure_counters"`
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

var failureNames = []string{"INVALID_INPUT", "CANCELLED", "DEADLINE_EXCEEDED", "ASSOCIATION_FAILED", "ADMISSION_FAILED", "RESOURCE_EXHAUSTED", "OVERFLOW", "INVARIANT_FAILED"}

func (a Accounting) MarshalJSON() ([]byte, error) {
	type raw struct {
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
	return json.Marshal(raw{a.SchemaVersion, a.JFiles, a.QQueryBytes, a.PPathBytes, a.SSourceBytes, a.TScannedTuples, a.MMatches, a.RRanges, a.UUTF16Units, a.BOutputBytes, a.WWork, a.FailureCounters})
}
func (a *Accounting) UnmarshalJSON(b []byte) error {
	type raw struct {
		SchemaVersion   string            `json:"schema_version"`
		JFiles          json.Number       `json:"J_files"`
		QQueryBytes     json.Number       `json:"Q_query_bytes"`
		PPathBytes      json.Number       `json:"P_path_bytes"`
		SSourceBytes    json.Number       `json:"S_source_bytes"`
		TScannedTuples  json.Number       `json:"T_scanned_tuples"`
		MMatches        json.Number       `json:"M_matches"`
		RRanges         json.Number       `json:"R_ranges"`
		UUTF16Units     json.Number       `json:"U_utf16_units"`
		BOutputBytes    json.Number       `json:"B_output_bytes"`
		WWork           json.Number       `json:"W_work"`
		FailureCounters map[string]uint64 `json:"failure_counters"`
	}
	var r raw
	if err := decodeExact(b, &r, []string{"schema_version", "J_files", "Q_query_bytes", "P_path_bytes", "S_source_bytes", "T_scanned_tuples", "M_matches", "R_ranges", "U_utf16_units", "B_output_bytes", "W_work", "failure_counters"}); err != nil {
		return err
	}
	vals := []*uint64{&a.JFiles, &a.QQueryBytes, &a.PPathBytes, &a.SSourceBytes, &a.TScannedTuples, &a.MMatches, &a.RRanges, &a.UUTF16Units, &a.BOutputBytes, &a.WWork}
	nums := []json.Number{r.JFiles, r.QQueryBytes, r.PPathBytes, r.SSourceBytes, r.TScannedTuples, r.MMatches, r.RRanges, r.UUTF16Units, r.BOutputBytes, r.WWork}
	for i, n := range nums {
		u, err := parseU64(n.String())
		if err != nil {
			return err
		}
		*vals[i] = u
	}
	a.SchemaVersion = r.SchemaVersion
	a.FailureCounters = r.FailureCounters
	return nil
}

func ValidateBundleFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return ValidateBundleBytes(b)
}
func ValidateBundleBytes(b []byte) error {
	var bun Bundle
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bun); err != nil {
		return err
	}
	if err := ValidateBundle(bun); err != nil {
		return err
	}
	return nil
}
func ValidateFile(path string) error { return ValidateBundleFile(path) }

func ValidateBundle(b Bundle) error {
	if err := validateNamedArtifactRoles(b); err != nil {
		return err
	}
	if err := walkSchema(b.SchemaBytes); err != nil {
		return err
	}
	if !utf8.ValidString(b.TerminalBytes) {
		return verr("INVALID_INPUT", "/terminal_bytes", "invalid utf8")
	}
	var rawTerminal any
	rdec := json.NewDecoder(strings.NewReader(b.TerminalBytes))
	rdec.UseNumber()
	if err := rdec.Decode(&rawTerminal); err != nil {
		return verr("INVALID_INPUT", "/terminal_bytes", err.Error())
	}
	if err := validateSchemaValue(b.SchemaBytes, rawTerminal); err != nil {
		return err
	}
	var t Terminal
	dec := json.NewDecoder(strings.NewReader(b.TerminalBytes))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return verr("INVALID_INPUT", "/terminal_bytes", err.Error())
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return verr("INVALID_INPUT", "/terminal_bytes", "trailing data")
	}
	canon, err := CanonicalJSON(t)
	if err != nil {
		return err
	}
	if canon != b.TerminalBytes {
		return verr("INVALID_INPUT", "/terminal_bytes", "non-canonical")
	}
	if t.SchemaVersion != TerminalSchemaVersion {
		return verr("INVARIANT_FAILED", "/schema_version", "bad schema")
	}
	if t.Accounting.SchemaVersion != AccountingSchemaVersion {
		return verr("INVARIANT_FAILED", "/accounting/schema_version", "bad accounting schema")
	}
	if t.Custody.SchemaVersion != CustodySchemaVersion {
		return verr("INVARIANT_FAILED", "/custody/schema_version", "bad custody schema")
	}
	if t.Replay.SchemaVersion != ReplaySchemaVersion {
		return verr("INVARIANT_FAILED", "/replay/schema_version", "bad replay schema")
	}
	if got := sha(b.RawAttemptBytes); t.Replay.CanonicalAttemptSHA256 != got {
		return verr("INVARIANT_FAILED", "/replay/canonical_attempt_sha256", "raw attempt digest mismatch")
	}
	if t.Attempt.MalformedRaw && t.Attempt.AttemptID != "attempt-raw-sha256-"+strings.TrimPrefix(t.Replay.CanonicalAttemptSHA256, "sha256:") {
		return verr("INVARIANT_FAILED", "/attempt/attempt_id", "malformed raw digest identity")
	}
	if t.Custody.AttemptID != t.Attempt.AttemptID || t.Custody.TerminalSequence0 != 0 || t.Custody.TerminalCount1 != 1 {
		return verr("INVARIANT_FAILED", "/custody", "bad custody constants")
	}
	if t.Replay.AdmittedBindingSHA256 != sha(b.AdmittedBindingBytes) {
		return verr("INVARIANT_FAILED", "/replay/admitted_binding_sha256", "admitted binding pin mismatch")
	}
	if t.Replay.ToolingIdentitySHA256 != sha(b.ToolingManifestBytes) || t.Replay.PredecessorLockSHA256 != sha(b.PredecessorManifestBytes) || t.Replay.FreezeBindingSHA256 != sha(b.PayloadFreezeBytes) || t.Payload.FreezeBindingSHA256 != sha(b.PayloadFreezeBytes) || t.Payload.PayloadDigest != sha(b.PayloadFreezeBytes) {
		return verr("INVARIANT_FAILED", "/replay", "manifest pin mismatch")
	}
	if err := validateFailure(t); err != nil {
		return err
	}
	if err := validateSourcesMatches(t, b); err != nil {
		return err
	}
	if err := validateCandidate(t); err != nil {
		return err
	}
	if err := validateAccounting(t, b.TerminalBytes); err != nil {
		return err
	}
	pre := normalizedTerminalPreimage(t)
	if t.Replay.TerminalPreimageSHA256 != sha(pre) || t.Custody.TerminalResultSHA256 != t.Replay.TerminalPreimageSHA256 {
		return verr("INVARIANT_FAILED", "/custody/terminal_result_sha256", "preimage digest mismatch")
	}
	derivedBytes, err := Derive(deriveInputFromBundle(b))
	if err != nil {
		return err
	}
	if derivedBytes != b.TerminalBytes {
		return verr("INVARIANT_FAILED", "/terminal_bytes", "terminal does not equal derived canonical terminal")
	}
	return nil
}

func validateNamedArtifactRoles(b Bundle) error {
	if sha(b.PayloadFreezeBytes) != expectedPayloadManifestSHA || sha(b.ToolingManifestBytes) != expectedToolingCensusSHA || sha(b.PredecessorManifestBytes) != expectedPredecessorLockSHA {
		return verr("INVARIANT_FAILED", "/artifact_roles", "unexpected named artifact bytes")
	}
	var payload struct {
		Members []struct {
			Path   string `json:"path"`
			Bytes  uint64 `json:"bytes"`
			SHA256 string `json:"sha256"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(b.PayloadFreezeBytes), &payload); err != nil || len(payload.Members) == 0 || payload.Members[0].Path == "" || payload.Members[0].SHA256 == "" {
		return verr("INVARIANT_FAILED", "/payload_freeze_binding_bytes", "bad payload manifest")
	}
	var tooling struct {
		SchemaVersion string   `json:"schema_version"`
		Stage         string   `json:"stage"`
		Members       []string `json:"members"`
	}
	if err := json.Unmarshal([]byte(b.ToolingManifestBytes), &tooling); err != nil || tooling.SchemaVersion != "lsp-trace.adr0007.source-text-search.tooling-census.private.v4" || tooling.Stage == "" || len(tooling.Members) == 0 {
		return verr("INVARIANT_FAILED", "/tooling_manifest_bytes", "bad tooling census")
	}
	if !strings.Contains(b.PredecessorManifestBytes, "ADR0007") && !strings.Contains(b.PredecessorManifestBytes, "source-text-search") {
		return verr("INVARIANT_FAILED", "/predecessor_manifest_bytes", "bad predecessor lock")
	}
	return nil
}

type DeriveInput struct {
	RawAttemptBytes          string
	AdmittedSourceBytes      map[string]string
	AdmittedBindingBytes     string
	ToolingManifestBytes     string
	PredecessorManifestBytes string
	PayloadFreezeBytes       string
}

type rawAttempt struct {
	AttemptID        string                  `json:"attempt_id"`
	SchemaVersion    string                  `json:"schema_version"`
	Request          rawRequest              `json:"request"`
	ExecutionControl rawControl              `json:"execution_control"`
	SourceInputs     []rawSourceInput        `json:"source_inputs"`
	TestControl      *ArithmeticProbeControl `json:"test_control,omitempty"`
}
type ArithmeticProbeControl struct {
	SchemaVersion string       `json:"schema_version"`
	Counter       ProbeCounter `json:"counter"`
	Initial       uint64       `json:"initial"`
	Increment     uint64       `json:"increment"`
}
type ProbeCounter string

const (
	ProbeCounterBOutputBytes   ProbeCounter = "B_output_bytes"
	ProbeCounterWWork          ProbeCounter = "W_work"
	ProbeCounterTScannedTuples ProbeCounter = "T_scanned_tuples"
)

func (c *ArithmeticProbeControl) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if _, legacy := raw["initial_B_output_bytes"]; legacy {
		if len(raw) != 2 {
			return errors.New("test_control legacy shape")
		}
		var schema string
		if err := json.Unmarshal(raw["schema_version"], &schema); err != nil {
			return err
		}
		var n json.Number
		if err := json.Unmarshal(raw["initial_B_output_bytes"], &n); err != nil {
			return err
		}
		u, err := parseU64(n.String())
		if err != nil {
			return err
		}
		*c = ArithmeticProbeControl{SchemaVersion: schema, Counter: ProbeCounterBOutputBytes, Initial: u, Increment: 1}
		return c.validate()
	}
	if len(raw) != 4 {
		return errors.New("test_control shape")
	}
	type alias ArithmeticProbeControl
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*c = ArithmeticProbeControl(a)
	return c.validate()
}

func (c ArithmeticProbeControl) validate() error {
	if c.SchemaVersion != "lsp-trace.adr0007.source-text-search.test-control.private.v4" {
		return errors.New("test_control schema_version")
	}
	switch c.Counter {
	case ProbeCounterBOutputBytes, ProbeCounterWWork, ProbeCounterTScannedTuples:
		return nil
	default:
		return errors.New("test_control counter")
	}
}

type rawRequest struct {
	SchemaVersion string         `json:"schema_version"`
	Query         string         `json:"query"`
	Policy        rawPolicy      `json:"policy"`
	LocationPin   rawLocationPin `json:"location_pin"`
	Limits        rawLimits      `json:"limits"`
	Sources       []rawSourceRef `json:"sources"`
}
type rawPolicy struct {
	SchemaVersion         string `json:"schema_version"`
	LiteralMode           string `json:"literal_mode"`
	AllowRegex            bool   `json:"allow_regex"`
	AllowFuzzy            bool   `json:"allow_fuzzy"`
	AllowToken            bool   `json:"allow_token"`
	AllowRank             bool   `json:"allow_rank"`
	AllowModel            bool   `json:"allow_model"`
	AllowBackendSemantics bool   `json:"allow_backend_semantics"`
}
type sourceAdmissionPin struct {
	Bytes            uint64   `json:"bytes"`
	GitBlobSHA1      string   `json:"git_blob_sha1"`
	Path             string   `json:"path"`
	RepositoryCommit string   `json:"repository_commit"`
	SHA256           string   `json:"sha256"`
	Symbols          []string `json:"symbols"`
}
type rawLocationPin struct {
	SchemaVersion        string             `json:"schema_version"`
	Operation            string             `json:"operation"`
	ExecutedLocation     bool               `json:"executedLocation"`
	CompleteLocationPins []string           `json:"complete_location_pins"`
	DesignCommit         string             `json:"design_commit"`
	DesignRootSHA256     string             `json:"design_root_sha256"`
	ExecutionCommit      string             `json:"execution_commit"`
	SealCommit           string             `json:"seal_commit"`
	FinalSealSHA256      string             `json:"final_seal_sha256"`
	SourceAdmissionPin   sourceAdmissionPin `json:"source_admission_pin"`
}
type rawLimits struct {
	SchemaVersion  string `json:"schema_version"`
	MaxFiles       uint64 `json:"max_files"`
	MaxMatches     uint64 `json:"max_matches"`
	MaxOutputBytes uint64 `json:"max_output_bytes"`
	MaxPathBytes   uint64 `json:"max_path_bytes"`
	MaxSourceBytes uint64 `json:"max_source_bytes"`
	MaxTotalBytes  uint64 `json:"max_total_bytes"`
	MaxWork        uint64 `json:"max_work"`
}
type rawSourceRef struct {
	Path         string `json:"path"`
	Revision     string `json:"revision"`
	FileDigest   string `json:"file_digest"`
	ObjectDigest string `json:"object_digest"`
	Ordinal      uint64 `json:"ordinal"`
}
type rawSourceInput struct {
	SchemaVersion string `json:"schema_version"`
	Path          string `json:"path"`
	Revision      string `json:"revision"`
	FileDigest    string `json:"file_digest"`
	ObjectDigest  string `json:"object_digest"`
	Ordinal       uint64 `json:"ordinal"`
	BytesBase64   string `json:"bytes_base64"`
}
type rawControl struct {
	SchemaVersion string           `json:"schema_version"`
	Observations  []rawObservation `json:"observations"`
}
type rawObservation struct {
	PollIndex       uint64 `json:"poll_index"`
	Cancelled       bool   `json:"cancelled,omitempty"`
	DeadlineExpired bool   `json:"deadline_expired,omitempty"`
}

type admittedSource struct {
	ref  rawSourceRef
	data []byte
}

type derivationRuntime struct {
	poll uint64
	obs  []rawObservation
}

type sourceAdmissionBinding struct {
	Schema          string                  `json:"schema"`
	AdmissionDigest string                  `json:"admissionDigest"`
	Sources         []sourceAdmissionMember `json:"sources"`
}
type sourceAdmissionMember struct {
	Path         string `json:"path"`
	Revision     string `json:"revision"`
	FileDigest   string `json:"fileDigest"`
	ObjectDigest string `json:"objectDigest"`
	BytesBase64  string `json:"bytes_base64"`
}

func deriveInputFromBundle(b Bundle) DeriveInput {
	return DeriveInput{RawAttemptBytes: b.RawAttemptBytes, AdmittedSourceBytes: b.AdmittedSourceBytes, AdmittedBindingBytes: b.AdmittedBindingBytes, ToolingManifestBytes: b.ToolingManifestBytes, PredecessorManifestBytes: b.PredecessorManifestBytes, PayloadFreezeBytes: b.PayloadFreezeBytes}
}

func Derive(in DeriveInput) (string, error) {
	if err := validateNamedArtifactRoles(Bundle{ToolingManifestBytes: in.ToolingManifestBytes, PredecessorManifestBytes: in.PredecessorManifestBytes, PayloadFreezeBytes: in.PayloadFreezeBytes}); err != nil {
		return "", err
	}
	attempt, err := parseRawAttempt(in.RawAttemptBytes)
	if err != nil {
		return deriveFailure(in, "INVALID_INPUT", map[string]any{"reason": err.Error()}, false, nil, "invalid")
	}
	if err := validatePolicyLocation(attempt); err != nil {
		return deriveFailure(in, "INVALID_INPUT", map[string]any{"reason": err.Error()}, false, nil, nonemptyQuery(attempt.Request.Query))
	}
	if attempt.Request.Query == "" || len(attempt.Request.Sources) == 0 || len(attempt.SourceInputs) == 0 {
		return deriveFailure(in, "INVALID_INPUT", map[string]any{"reason": "request"}, false, nil, nonemptyQuery(attempt.Request.Query))
	}
	if badControl(attempt.ExecutionControl.Observations) {
		return deriveFailure(in, "INVALID_INPUT", map[string]any{"reason": "execution_control"}, false, nil, attempt.Request.Query)
	}
	rt := &derivationRuntime{obs: attempt.ExecutionControl.Observations}
	if code, detail := rt.pollControl(); code != "" {
		return deriveFailure(in, code, detail, false, nil, attempt.Request.Query)
	}
	selected, assocErr := associateSources(attempt, in.AdmittedSourceBytes)
	if assocErr != nil {
		return deriveFailure(in, "ASSOCIATION_FAILED", map[string]any{"source_id": assocErr.Error()}, false, nil, attempt.Request.Query)
	}
	admitted, binding, admDetail, admOK, resource := admitSources(selected, attempt.Request.Limits)
	if err := copyExactAdmittedSourceBytes(admitted, in.AdmittedSourceBytes); err != nil {
		return deriveFailure(in, "ASSOCIATION_FAILED", map[string]any{"source_id": err.Error()}, false, nil, attempt.Request.Query)
	}
	if !admOK {
		code := "ADMISSION_FAILED"
		detail := map[string]any{"source_id": admDetail}
		if resource {
			code = "RESOURCE_EXHAUSTED"
			detail = map[string]any{"limit": admDetail}
		}
		return deriveFailure(in, code, detail, true, sourcesFromAdmitted(admitted), attempt.Request.Query)
	}
	if in.AdmittedBindingBytes != binding {
		return deriveFailure(in, "ADMISSION_FAILED", map[string]any{"source_id": "admitted_binding_bytes"}, true, sourcesFromAdmitted(admitted), attempt.Request.Query)
	}
	t := baseTerminal(in, attempt.AttemptID, attempt.Request.Query)
	t.Sources = sourcesFromAdmitted(admitted)
	for _, src := range t.Sources {
		if src.PathBytes > attempt.Request.Limits.MaxPathBytes {
			return deriveFailureWithBase(in, t, "RESOURCE_EXHAUSTED", map[string]any{"limit": "max_path_bytes"}, true)
		}
	}
	t.Admission = Admission{Completed: true, AdmissionDigest: admissionDigest(admitted), AdmittedSourceIDs: sourceIDs(t.Sources)}
	t.Replay.AdmittedBindingSHA256 = sha(in.AdmittedBindingBytes)
	if code, detail := rt.pollControl(); code != "" {
		return deriveFailureWithBase(in, t, code, detail, false)
	}
	qbytes := []byte(attempt.Request.Query)
	matchOrdinal := uint64(0)
	for si, s := range admitted {
		data := s.data
		for off := 0; off+len(qbytes) <= len(data); off++ {
			if !charge(&t.Accounting.TScannedTuples, 1) {
				return deriveFailureWithBase(in, t, "OVERFLOW", map[string]any{"counter": "T_scanned_tuples"}, true)
			}
			if code, detail := rt.pollControl(); code != "" {
				return deriveFailureWithBase(in, t, code, detail, false)
			}
			if bytes.Equal(data[off:off+len(qbytes)], qbytes) {
				if matchOrdinal+1 > attempt.Request.Limits.MaxMatches {
					return deriveFailureWithBase(in, t, "RESOURCE_EXHAUSTED", map[string]any{"limit": "max_matches"}, true)
				}
				src := t.Sources[si]
				id := deriveMatchID(src.SourceID, matchOrdinal, uint64(off), uint64(off+len(qbytes)), attempt.Request.Query)
				m := Match{MatchID: id, SourceID: src.SourceID, PathBytes: src.PathBytes, StartByte: uint64(off), EndByte: uint64(off + len(qbytes)), Ordinal: matchOrdinal, Literal: attempt.Request.Query}
				p := positionFor(data, m.StartByte, m.EndByte)
				p.MatchID = id
				units := uint64(len(utf16.Encode([]rune(m.Literal))))
				if !charge(&t.Accounting.MMatches, 1) || !charge(&t.Accounting.RRanges, 1) || !charge(&t.Accounting.UUTF16Units, units) {
					return deriveFailureWithBase(in, t, "OVERFLOW", map[string]any{"counter": "match_accounting"}, true)
				}
				t.Matches = append(t.Matches, m)
				t.Positions = append(t.Positions, *p)
				matchOrdinal++
				if code, detail := rt.pollControl(); code != "" {
					return deriveFailureWithBase(in, t, code, detail, false)
				}
			}
		}
	}
	sortMatchesPositions(t.Matches, t.Positions, t.Sources)
	t.RangeUnionCandidate = deriveCandidateWithPin(t, attempt.Request.LocationPin)
	if code, detail := rt.pollControl(); code != "" {
		return deriveFailureWithBase(in, t, code, detail, false)
	}
	finalizeTerminal(&t)
	if code, detail := rt.pollControl(); code != "" {
		return deriveFailureWithBase(in, t, code, detail, false)
	}
	if err := rt.allConsumed(); err != nil {
		return deriveFailureWithBase(in, t, "INVALID_INPUT", map[string]any{"reason": err.Error()}, true)
	}
	if t.Accounting.WWork > attempt.Request.Limits.MaxWork || t.Accounting.BOutputBytes > attempt.Request.Limits.MaxOutputBytes {
		return deriveFailureWithBase(in, t, "RESOURCE_EXHAUSTED", map[string]any{"limit": "post_scan"}, true)
	}
	if overflow, counter, initial, increment := overflowRequested(attempt); overflow {
		if _, ok := checkedAdd(initial, increment); !ok {
			return deriveFailureWithBase(in, t, "OVERFLOW", map[string]any{"counter": string(counter)}, true)
		}
	}
	return CanonicalJSON(t)
}

func nonemptyQuery(q string) string {
	if q == "" {
		return "invalid"
	}
	return q
}

func parseRawAttempt(raw string) (rawAttempt, error) {
	if err := rejectDuplicateJSON([]byte(raw)); err != nil {
		return rawAttempt{}, err
	}
	var r rawAttempt
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return rawAttempt{}, err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return rawAttempt{}, errors.New("trailing data")
	}
	return r, nil
}

func rejectDuplicateJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					kt, err := dec.Token()
					if err != nil {
						return err
					}
					k, ok := kt.(string)
					if !ok {
						return errors.New("object key")
					}
					if seen[k] {
						return fmt.Errorf("duplicate field %s", k)
					}
					seen[k] = true
					if err := walk(); err != nil {
						return err
					}
				}
				_, err := dec.Token()
				return err
			case '[':
				for dec.More() {
					if err := walk(); err != nil {
						return err
					}
				}
				_, err := dec.Token()
				return err
			}
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}

func validatePolicyLocation(a rawAttempt) error {
	p := a.Request.Policy
	if p.SchemaVersion != "lsp-trace.adr0007.source-text-search.policy.private.v4" || p.LiteralMode != "byte-literal" || p.AllowRegex || p.AllowFuzzy || p.AllowToken || p.AllowRank || p.AllowModel || p.AllowBackendSemantics {
		return errors.New("policy")
	}
	lp := a.Request.LocationPin
	want := []string{"design_commit", "design_root_sha256", "execution_commit", "seal_commit", "final_seal_sha256", "source_admission_pin"}
	if lp.SchemaVersion != "lsp-trace.adr0007.source-text-search.location-pin.private.v4" || lp.Operation != CandidateOperation || lp.ExecutedLocation || len(lp.CompleteLocationPins) != len(want) || lp.DesignCommit != "c943a484060462121c6f0929d4182b053ff95ab5" || lp.DesignRootSHA256 != "sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d" || lp.ExecutionCommit != "f0f8b49aa368bea9b3e6d105eef5cb2614221067" || lp.SealCommit != "16f40dcb03a234b00db80059a7eef400495e9d97" || lp.FinalSealSHA256 != "sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6" {
		return errors.New("location_pin")
	}
	for i := range want {
		if lp.CompleteLocationPins[i] != want[i] {
			return errors.New("location_pin_order")
		}
	}
	pin := lp.SourceAdmissionPin
	if pin.RepositoryCommit != "af2ce89321afc94c937636f841bb98b8977b6496" || pin.Path != "internal/sourceadmissionv2/admission.go" || pin.Bytes != 3519 || pin.SHA256 != "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da" || pin.GitBlobSHA1 != "4953fab89d911e2352fa6c2253497c07c8e9a777" || len(pin.Symbols) != 3 || pin.Symbols[0] != "Admit" || pin.Symbols[1] != "CanonicalPath" || pin.Symbols[2] != "Digest" {
		return errors.New("source_admission_pin")
	}
	return nil
}

func badControl(obs []rawObservation) bool {
	seen := map[uint64]bool{}
	last := uint64(0)
	for i, o := range obs {
		if seen[o.PollIndex] {
			return true
		}
		seen[o.PollIndex] = true
		if i > 0 && o.PollIndex <= last {
			return true
		}
		last = o.PollIndex
	}
	return false
}
func (rt *derivationRuntime) allConsumed() error {
	for _, o := range rt.obs {
		if o.PollIndex >= rt.poll {
			return fmt.Errorf("unconsumed_poll:%d", o.PollIndex)
		}
	}
	return nil
}
func (rt *derivationRuntime) pollControl() (string, map[string]any) {
	poll := rt.poll
	rt.poll++
	for _, o := range rt.obs {
		if o.PollIndex == poll {
			if o.DeadlineExpired {
				return "DEADLINE_EXCEEDED", map[string]any{"deadline": fmt.Sprintf("poll:%d", poll)}
			}
			if o.Cancelled {
				return "CANCELLED", map[string]any{"control": fmt.Sprintf("poll:%d", poll)}
			}
		}
	}
	return "", nil
}

func associateSources(a rawAttempt, exact map[string]string) ([]admittedSource, error) {
	inputs := map[uint64]rawSourceInput{}
	for _, in := range a.SourceInputs {
		if _, dup := inputs[in.Ordinal]; dup {
			return nil, fmt.Errorf("ordinal:%d", in.Ordinal)
		}
		inputs[in.Ordinal] = in
	}
	out := make([]admittedSource, 0, len(a.Request.Sources))
	for _, ref := range a.Request.Sources {
		in, ok := inputs[ref.Ordinal]
		if !ok {
			return nil, fmt.Errorf("ordinal:%d", ref.Ordinal)
		}
		if in.Path != ref.Path || in.Revision != ref.Revision || in.FileDigest != ref.FileDigest || in.ObjectDigest != ref.ObjectDigest {
			return nil, fmt.Errorf("ordinal:%d", ref.Ordinal)
		}
		rawData, err := base64.StdEncoding.DecodeString(in.BytesBase64)
		if err != nil {
			return nil, fmt.Errorf("ordinal:%d", ref.Ordinal)
		}
		data := rawData
		if exact != nil {
			// source IDs are path-sort ordinals; compute after a provisional admission below when possible.
			data = rawData
		}
		out = append(out, admittedSource{ref: ref, data: data})
	}
	if len(inputs) != len(a.Request.Sources) {
		return nil, errors.New("extra_source_input")
	}
	return out, nil
}

func admitSources(in []admittedSource, l rawLimits) ([]admittedSource, string, string, bool, bool) {
	if len(in) == 0 || l.MaxFiles < 1 || l.MaxSourceBytes < 1 || l.MaxTotalBytes < 1 {
		return nil, "", "limits or sources", false, false
	}
	if uint64(len(in)) > l.MaxFiles {
		return nil, "", "max_files", false, true
	}
	seen := map[string]bool{}
	total := uint64(0)
	out := append([]admittedSource(nil), in...)
	for i, s := range out {
		if !canonicalPath(s.ref.Path) || s.ref.Revision == "" || len(s.data) == 0 || !utf8.Valid(s.data) {
			return nil, "", s.ref.Path, false, false
		}
		if seen[s.ref.Path] {
			return nil, "", s.ref.Path, false, false
		}
		seen[s.ref.Path] = true
		if uint64(len(s.data)) > l.MaxSourceBytes {
			return nil, "", "max_source_bytes", false, true
		}
		total += uint64(len(s.data))
		if total > l.MaxTotalBytes {
			return nil, "", "max_total_bytes", false, true
		}
		d := shaBytes(s.data)
		if s.ref.FileDigest != "" && s.ref.FileDigest != d {
			return nil, "", s.ref.Path, false, false
		}
		if s.ref.ObjectDigest != "" && s.ref.ObjectDigest != d {
			return nil, "", s.ref.Path, false, false
		}
		out[i].ref.FileDigest = d
		out[i].ref.ObjectDigest = d
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare([]byte(out[i].ref.Path), []byte(out[j].ref.Path)) < 0 })
	return out, sourceAdmissionBindingBytes(out), "", true, false
}
func canonicalPath(p string) bool {
	return p != "" && utf8.ValidString(p) && norm.NFC.IsNormalString(p) && !strings.Contains(p, "\\") && !strings.Contains(p, "//") && !strings.Contains(p, ":") && !strings.HasPrefix(p, "/") && p != "." && p != ".." && !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") && !strings.Contains(p, "/./") && !strings.Contains(p, "/../") && path.Clean(p) == p
}
func admissionDigest(in []admittedSource) string {
	h := sha256.New()
	h.Write([]byte("lsp-trace.adr0007.source-admission.private.v2"))
	for _, s := range in {
		for _, v := range []string{s.ref.Path, s.ref.Revision, s.ref.FileDigest, s.ref.ObjectDigest} {
			h.Write([]byte{0})
			h.Write([]byte(v))
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func sourceAdmissionBindingBytes(in []admittedSource) string {
	b := sourceAdmissionBinding{Schema: "lsp-trace.adr0007.source-admission.private.v2", AdmissionDigest: admissionDigest(in)}
	for _, s := range in {
		b.Sources = append(b.Sources, sourceAdmissionMember{Path: s.ref.Path, Revision: s.ref.Revision, FileDigest: s.ref.FileDigest, ObjectDigest: s.ref.ObjectDigest, BytesBase64: base64.StdEncoding.EncodeToString(s.data)})
	}
	out, _ := CanonicalJSON(b)
	return out
}
func sourcesFromAdmitted(in []admittedSource) []Source {
	out := make([]Source, 0, len(in))
	for i, s := range in {
		out = append(out, Source{SourceID: deriveSourceID(s.ref.Path, s.ref.Revision, uint64(i)), LogicalURI: "file:///" + s.ref.Path, PathBytes: uint64(len([]byte(s.ref.Path))), ByteLength: uint64(len(s.data)), ContentSHA256: shaBytes(s.data), Ordinal: uint64(i)})
	}
	return out
}
func deriveSourceID(path, rev string, ord uint64) string {
	return sha(fmt.Sprintf("source\x00%s\x00%s\x00%d", path, rev, ord))
}
func charge(v *uint64, d uint64) bool {
	n, ok := checkedAdd(*v, d)
	if ok {
		*v = n
	}
	return ok
}
func checkedAdd(a, b uint64) (uint64, bool) {
	if a > math.MaxUint64-b {
		return 0, false
	}
	return a + b, true
}
func sortMatchesPositions(ms []Match, ps []Position, sources []Source) {
	pos := map[string]Position{}
	pathByID := map[string]string{}
	ordByID := map[string]uint64{}
	for _, s := range sources {
		posixPath := strings.TrimPrefix(s.LogicalURI, "file:///")
		pathByID[s.SourceID] = posixPath
		ordByID[s.SourceID] = s.Ordinal
	}
	for _, p := range ps {
		pos[p.MatchID] = p
	}
	sort.Slice(ms, func(i, j int) bool {
		if cmp := bytes.Compare([]byte(pathByID[ms[i].SourceID]), []byte(pathByID[ms[j].SourceID])); cmp != 0 {
			return cmp < 0
		}
		if ms[i].StartByte != ms[j].StartByte {
			return ms[i].StartByte < ms[j].StartByte
		}
		if ms[i].EndByte != ms[j].EndByte {
			return ms[i].EndByte < ms[j].EndByte
		}
		if ordByID[ms[i].SourceID] != ordByID[ms[j].SourceID] {
			return ordByID[ms[i].SourceID] < ordByID[ms[j].SourceID]
		}
		return ms[i].Ordinal < ms[j].Ordinal
	})
	for i, m := range ms {
		ps[i] = pos[m.MatchID]
	}
}
func copyExactAdmittedSourceBytes(in []admittedSource, exact map[string]string) error {
	if exact == nil {
		return nil
	}
	if len(in) != len(exact) {
		return errors.New("admitted_source_bytes_count")
	}
	for i := range in {
		id := deriveSourceID(in[i].ref.Path, in[i].ref.Revision, uint64(i))
		got, ok := exact[id]
		if !ok {
			return fmt.Errorf("missing:%s", id)
		}
		if got != string(in[i].data) {
			return fmt.Errorf("mismatch:%s", id)
		}
		in[i].data = append([]byte(nil), []byte(got)...)
	}
	return nil
}

func baseTerminal(in DeriveInput, attemptID, query string) Terminal {
	freeze := sha(in.PayloadFreezeBytes)
	return Terminal{SchemaVersion: TerminalSchemaVersion, Terminal: "COMPLETE", Attempt: Attempt{AttemptID: attemptID, MalformedRaw: false}, Request: Request{Query: query}, Sources: []Source{}, Matches: []Match{}, Positions: []Position{}, Accounting: Accounting{SchemaVersion: AccountingSchemaVersion}, Custody: Custody{SchemaVersion: CustodySchemaVersion, AttemptID: attemptID, TerminalSequence0: 0, TerminalCount1: 1}, Replay: Replay{SchemaVersion: ReplaySchemaVersion, CanonicalAttemptSHA256: sha(in.RawAttemptBytes), ToolingIdentitySHA256: sha(in.ToolingManifestBytes), FreezeBindingSHA256: freeze, PredecessorLockSHA256: sha(in.PredecessorManifestBytes), AdmittedBindingSHA256: zeroSHA}, Payload: Payload{PayloadDigest: freeze, FreezeBindingSHA256: freeze}}
}
func deriveFailure(in DeriveInput, code string, detail map[string]any, includeAdmission bool, sources []Source, query string) (string, error) {
	t := baseTerminal(in, "attempt-raw-sha256-"+strings.TrimPrefix(sha(in.RawAttemptBytes), "sha256:"), query)
	t.Sources = append([]Source{}, sources...)
	return deriveFailureWithBase(in, t, code, detail, includeAdmission)
}
func deriveFailureWithBase(in DeriveInput, t Terminal, code string, detail map[string]any, includeAdmission bool) (string, error) {
	t.Terminal = "FAILED"
	t.Attempt.MalformedRaw = code == "INVALID_INPUT"
	t.Matches = []Match{}
	t.Positions = []Position{}
	t.RangeUnionCandidate = nil
	stage := map[string]string{"INVALID_INPUT": "raw", "CANCELLED": "control", "DEADLINE_EXCEEDED": "control", "ASSOCIATION_FAILED": "association", "ADMISSION_FAILED": "admission", "RESOURCE_EXHAUSTED": "scan", "OVERFLOW": "accounting", "INVARIANT_FAILED": "invariant"}[code]
	t.Failure = &Failure{Code: code, Stage: stage, Detail: detail}
	if includeAdmission {
		t.Admission.Completed = true
		if t.Admission.AdmissionDigest == "" || t.Admission.AdmissionDigest == zeroSHA {
			t.Admission.AdmissionDigest = sha(in.AdmittedBindingBytes)
			t.Replay.AdmittedBindingSHA256 = t.Admission.AdmissionDigest
		}
		t.Admission.AdmittedSourceIDs = sourceIDs(t.Sources)
	} else {
		t.Admission = Admission{Completed: false, AdmissionDigest: zeroSHA, AdmittedSourceIDs: sourceIDs(t.Sources)}
		t.Replay.AdmittedBindingSHA256 = sha(in.AdmittedBindingBytes)
	}
	finalizeTerminal(&t)
	return CanonicalJSON(t)
}
func overflowRequested(a rawAttempt) (bool, ProbeCounter, uint64, uint64) {
	if a.TestControl == nil {
		return false, "", 0, 0
	}
	return true, a.TestControl.Counter, a.TestControl.Initial, a.TestControl.Increment
}
func finalizeTerminal(t *Terminal) {
	t.Accounting = deriveAccounting(*t, "")
	pre := normalizedTerminalPreimage(*t)
	t.Replay.TerminalPreimageSHA256 = sha(pre)
	t.Custody.TerminalResultSHA256 = t.Replay.TerminalPreimageSHA256
	t.Accounting = deriveAccounting(*t, "")
	pre = normalizedTerminalPreimage(*t)
	t.Replay.TerminalPreimageSHA256 = sha(pre)
	t.Custody.TerminalResultSHA256 = t.Replay.TerminalPreimageSHA256
}
func deriveCandidateWithPin(t Terminal, lp rawLocationPin) *Candidate {
	c := deriveCandidate(t)
	c.CandidateDigest = candidateDigest(c, lp)
	return c
}

func candidateDigest(c *Candidate, lp rawLocationPin) string {
	zero := *c
	zero.CandidateDigest = zeroSHA
	preimage := map[string]any{
		"schema_version":          lp.SchemaVersion,
		"operation":               lp.Operation,
		"executedLocation":        lp.ExecutedLocation,
		"candidate_only":          zero.CandidateOnly,
		"complete_location_pins":  append([]string(nil), lp.CompleteLocationPins...),
		"design_commit":           lp.DesignCommit,
		"design_root_sha256":      lp.DesignRootSHA256,
		"execution_commit":        lp.ExecutionCommit,
		"seal_commit":             lp.SealCommit,
		"final_seal_sha256":       lp.FinalSealSHA256,
		"source_admission_pin":    lp.SourceAdmissionPin,
		"admission_digest":        zero.AdmissionDigest,
		"member_match_ids":        append([]string(nil), zero.MemberMatchIDs...),
		"qualified_location_pins": append([]LocationPin(nil), zero.QualifiedLocationPins...),
		"candidate":               zero,
	}
	s, _ := CanonicalJSON(preimage)
	return sha(s)
}

func deriveCandidate(t Terminal) *Candidate {
	c := &Candidate{Operation: CandidateOperation, ExecutedLocation: false, CandidateOnly: true, AdmissionDigest: t.Admission.AdmissionDigest}
	h := sha256.New()
	for i, m := range t.Matches {
		c.MemberMatchIDs = append(c.MemberMatchIDs, m.MatchID)
		p := t.Positions[i]
		c.QualifiedLocationPins = append(c.QualifiedLocationPins, LocationPin{SourceID: m.SourceID, StartByte: m.StartByte, EndByte: m.EndByte, StartLine: p.StartLine, StartCharacterUTF16: p.StartCharacterUTF16, EndLine: p.EndLine, EndCharacterUTF16: p.EndCharacterUTF16})
		fmt.Fprintf(h, "%s:%s:%d:%d:%d\n", m.MatchID, m.SourceID, m.PathBytes, m.StartByte, m.EndByte)
	}
	c.CandidateDigest = "sha256:" + hex.EncodeToString(h.Sum(nil))
	return c
}

func deriveAccounting(t Terminal, term string) Accounting {
	a := t.Accounting
	a.SchemaVersion = AccountingSchemaVersion
	a.JFiles = uint64(len(t.Sources))
	a.QQueryBytes = uint64(len([]byte(t.Request.Query)))
	prevT, prevM, prevR, prevU := a.TScannedTuples, a.MMatches, a.RRanges, a.UUTF16Units
	a.PPathBytes, a.SSourceBytes = 0, 0
	for _, s := range t.Sources {
		a.PPathBytes += s.PathBytes
		a.SSourceBytes += s.ByteLength
	}
	a.TScannedTuples = prevT
	if prevM != 0 || t.Terminal == "FAILED" {
		a.MMatches = prevM
	} else {
		a.MMatches = uint64(len(t.Matches))
	}
	if prevR != 0 || t.Terminal == "FAILED" {
		a.RRanges = prevR
	} else {
		a.RRanges = uint64(len(t.Positions))
	}
	if prevU != 0 || t.Terminal == "FAILED" {
		a.UUTF16Units = prevU
	} else {
		a.UUTF16Units = 0
		for _, m := range t.Matches {
			a.UUTF16Units += uint64(len(utf16.Encode([]rune(m.Literal))))
		}
	}
	a.FailureCounters = map[string]uint64{}
	for _, k := range failureNames {
		a.FailureCounters[k] = 0
	}
	if t.Terminal == "FAILED" && t.Failure != nil {
		a.FailureCounters[t.Failure.Code] = 1
	}
	if b, ok := fixedPointB(t); ok {
		a.BOutputBytes = b
	} else {
		a.BOutputBytes = uint64(len([]byte(term)))
	}
	if w, ok := weightedW(a); ok {
		a.WWork = w
	}
	return a
}

func existingMatchID(ms []Match, sourceID string, start, end, ord uint64) string {
	for _, m := range ms {
		if m.SourceID == sourceID && m.StartByte == start && m.EndByte == end && m.Ordinal == ord {
			return m.MatchID
		}
	}
	return ""
}
func deriveMatchID(sourceID string, ordinal, start, end uint64, literal string) string {
	return sha(fmt.Sprintf("match\x00%s\x00%d\x00%d\x00%d\x00%s", sourceID, ordinal, start, end, literal))
}

func validateSchemaValue(schemaBytes []byte, value any) error {
	var schema any
	dec := json.NewDecoder(bytes.NewReader(schemaBytes))
	dec.UseNumber()
	if err := dec.Decode(&schema); err != nil {
		return verr("INVALID_INPUT", "/schema_bytes", err.Error())
	}
	return evalSchema(schema, value, "")
}

func evalSchema(schema any, value any, path string) error {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	if c, ok := m["const"]; ok && !jsonEqual(c, value) {
		return verr("INVALID_INPUT", pathOrRoot(path), "const mismatch")
	}
	if e, ok := m["enum"].([]any); ok {
		found := false
		for _, x := range e {
			if jsonEqual(x, value) {
				found = true
			}
		}
		if !found {
			return verr("INVALID_INPUT", pathOrRoot(path), "enum mismatch")
		}
	}
	if typ, ok := m["type"]; ok && !typeOK(typ, value) {
		return verr("INVALID_INPUT", pathOrRoot(path), "type mismatch")
	}
	if one, ok := m["oneOf"].([]any); ok {
		n := 0
		for _, sub := range one {
			if err := evalSchema(sub, value, path); err == nil {
				n++
			}
		}
		if n != 1 {
			return verr("INVALID_INPUT", pathOrRoot(path), "oneOf mismatch")
		}
	}
	if ifs, ok := m["if"]; ok {
		if evalSchema(ifs, value, path) == nil {
			if th, ok := m["then"]; ok {
				if err := evalSchema(th, value, path); err != nil {
					return err
				}
			}
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		obj, ok := value.(map[string]any)
		if !ok {
			return verr("INVALID_INPUT", pathOrRoot(path), "object required")
		}
		if req, ok := m["required"].([]any); ok {
			for _, r := range req {
				k := r.(string)
				if _, exists := obj[k]; !exists {
					return verr("INVALID_INPUT", pathJoin(path, k), "required missing")
				}
			}
		}
		if add, ok := m["additionalProperties"].(bool); ok && !add {
			for k := range obj {
				if _, known := props[k]; !known {
					return verr("INVALID_INPUT", pathJoin(path, k), "additional property")
				}
			}
		}
		for k, sub := range props {
			if v, exists := obj[k]; exists {
				if err := evalSchema(sub, v, pathJoin(path, k)); err != nil {
					return err
				}
			}
		}
	}
	if items, ok := m["items"]; ok {
		arr, ok := value.([]any)
		if !ok {
			return verr("INVALID_INPUT", pathOrRoot(path), "array required")
		}
		for i, v := range arr {
			if err := evalSchema(items, v, fmt.Sprintf("%s/%d", pathOrRoot(path), i)); err != nil {
				return err
			}
		}
	}
	if min, ok := num(m["minItems"]); ok {
		if arr, ok := value.([]any); ok && uint64(len(arr)) < min {
			return verr("INVALID_INPUT", pathOrRoot(path), "minItems")
		}
	}
	if max, ok := num(m["maxItems"]); ok {
		if arr, ok := value.([]any); ok && uint64(len(arr)) > max {
			return verr("INVALID_INPUT", pathOrRoot(path), "maxItems")
		}
	}
	if min, ok := num(m["minimum"]); ok {
		if n, ok := asU(value); ok && n < min {
			return verr("INVALID_INPUT", pathOrRoot(path), "minimum")
		}
	}
	if max, ok := num(m["maximum"]); ok {
		if n, ok := asU(value); ok && n > max {
			return verr("INVALID_INPUT", pathOrRoot(path), "maximum")
		}
	}
	if ml, ok := num(m["minLength"]); ok {
		if st, ok := value.(string); ok && uint64(len(st)) < ml {
			return verr("INVALID_INPUT", pathOrRoot(path), "minLength")
		}
	}
	if pat, ok := m["pattern"].(string); ok {
		if st, ok := value.(string); ok {
			matched, _ := regexp.MatchString(pat, st)
			if !matched {
				return verr("INVALID_INPUT", pathOrRoot(path), "pattern")
			}
		}
	}
	return nil
}
func pathOrRoot(p string) string {
	if p == "" {
		return "/"
	}
	return p
}
func pathJoin(p, k string) string {
	if p == "" {
		return "/" + k
	}
	return p + "/" + k
}
func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}
func num(v any) (uint64, bool) {
	switch x := v.(type) {
	case json.Number:
		u, err := parseU64(x.String())
		return u, err == nil
	case float64:
		return uint64(x), true
	}
	return 0, false
}
func asU(v any) (uint64, bool) {
	if n, ok := v.(json.Number); ok {
		u, err := parseU64(n.String())
		return u, err == nil
	}
	return 0, false
}
func typeOK(t any, v any) bool {
	if arr, ok := t.([]any); ok {
		for _, x := range arr {
			if typeOK(x, v) {
				return true
			}
		}
		return false
	}
	s, ok := t.(string)
	if !ok {
		return true
	}
	switch s {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "null":
		return v == nil
	case "integer":
		_, ok := v.(json.Number)
		return ok
	}
	return true
}

func validateFailure(t Terminal) error {
	sum := uint64(0)
	for _, k := range failureNames {
		v, ok := t.Accounting.FailureCounters[k]
		if !ok {
			return verr("INVARIANT_FAILED", "/accounting/failure_counters/"+k, "missing")
		}
		sum += v
	}
	if len(t.Accounting.FailureCounters) != len(failureNames) {
		return verr("INVARIANT_FAILED", "/accounting/failure_counters", "extra counter")
	}
	if t.Terminal == "COMPLETE" {
		if t.Failure != nil || sum != 0 {
			return verr("INVARIANT_FAILED", "/failure", "complete failure state")
		}
		return nil
	}
	if t.Terminal != "FAILED" {
		return verr("INVARIANT_FAILED", "/terminal", "bad terminal")
	}
	if t.Failure == nil || sum != 1 || t.Accounting.FailureCounters[t.Failure.Code] != 1 {
		return verr("INVARIANT_FAILED", "/failure", "failed counter/code mismatch")
	}
	stage := map[string]string{"INVALID_INPUT": "raw", "CANCELLED": "control", "DEADLINE_EXCEEDED": "control", "ASSOCIATION_FAILED": "association", "ADMISSION_FAILED": "admission", "RESOURCE_EXHAUSTED": "scan", "OVERFLOW": "accounting", "INVARIANT_FAILED": "invariant"}[t.Failure.Code]
	if stage == "" || t.Failure.Stage != stage {
		return verr("INVARIANT_FAILED", "/failure/stage", "bad stage")
	}
	req := map[string][]string{"INVALID_INPUT": {"reason"}, "CANCELLED": {"control"}, "DEADLINE_EXCEEDED": {"deadline"}, "ASSOCIATION_FAILED": {"source_id"}, "ADMISSION_FAILED": {"source_id"}, "RESOURCE_EXHAUSTED": {"limit"}, "OVERFLOW": {"counter"}, "INVARIANT_FAILED": {"invariant"}}[t.Failure.Code]
	if len(t.Failure.Detail) != len(req) {
		return verr("INVARIANT_FAILED", "/failure/detail", "detail shape")
	}
	for _, k := range req {
		if _, ok := t.Failure.Detail[k]; !ok {
			return verr("INVARIANT_FAILED", "/failure/detail/"+k, "missing")
		}
	}
	early := map[string]bool{"INVALID_INPUT": true, "CANCELLED": true, "DEADLINE_EXCEEDED": true, "ASSOCIATION_FAILED": true}
	if early[t.Failure.Code] && t.Admission.Completed {
		return verr("INVARIANT_FAILED", "/admission", "early failure must not have admission")
	}
	if !early[t.Failure.Code] && !t.Admission.Completed {
		return verr("INVARIANT_FAILED", "/admission", "late failure requires admission")
	}
	if len(t.Matches) != 0 || t.RangeUnionCandidate != nil {
		return verr("INVARIANT_FAILED", "/matches", "failed terminal must not retain matches")
	}
	return nil
}

func validateSourcesMatches(t Terminal, b Bundle) error {
	src := map[string]Source{}
	if t.Admission.Completed {
		if len(t.Admission.AdmittedSourceIDs) != len(t.Sources) {
			return verr("ADMISSION_FAILED", "/admission/admitted_source_ids", "membership count")
		}
		for i, s := range t.Sources {
			if t.Admission.AdmittedSourceIDs[i] != s.SourceID {
				return verr("ADMISSION_FAILED", "/admission/admitted_source_ids", "membership order")
			}
		}
	}
	for _, s := range t.Sources {
		data, ok := b.AdmittedSourceBytes[s.SourceID]
		if !ok {
			return verr("ASSOCIATION_FAILED", "/sources/"+s.SourceID, "missing source bytes")
		}
		if uint64(len([]byte(data))) != s.ByteLength || sha(data) != s.ContentSHA256 {
			return verr("ASSOCIATION_FAILED", "/sources/"+s.SourceID, "source metadata mismatch")
		}
		src[s.SourceID] = s
	}
	prevPathRank, prevStart, prevEnd, prevOrd := "", uint64(0), uint64(0), uint64(0)
	for i, m := range t.Matches {
		s, ok := src[m.SourceID]
		if !ok {
			return verr("ASSOCIATION_FAILED", "/matches", "unknown source")
		}
		if m.StartByte >= m.EndByte {
			return verr("INVARIANT_FAILED", "/matches/start_byte", "start must be < end")
		}
		data := []byte(b.AdmittedSourceBytes[m.SourceID])
		if m.EndByte > uint64(len(data)) {
			return verr("ASSOCIATION_FAILED", "/matches/end_byte", "out of source")
		}
		if string(data[m.StartByte:m.EndByte]) != m.Literal || m.Literal != t.Request.Query {
			return verr("ASSOCIATION_FAILED", "/matches/literal", "literal mismatch")
		}
		if m.PathBytes != s.PathBytes {
			return verr("ASSOCIATION_FAILED", "/matches/path_bytes", "path mismatch")
		}
		pathRank := sourcePathRank(s)
		if i > 0 && (bytes.Compare([]byte(pathRank), []byte(prevPathRank)) < 0 || (pathRank == prevPathRank && (m.StartByte < prevStart || (m.StartByte == prevStart && (m.EndByte < prevEnd || (m.EndByte == prevEnd && m.Ordinal <= prevOrd)))))) {
			return verr("INVARIANT_FAILED", "/matches", "deterministic order")
		}
		prevPathRank, prevStart, prevEnd, prevOrd = pathRank, m.StartByte, m.EndByte, m.Ordinal
	}
	if len(t.Positions) != len(t.Matches) {
		return verr("ASSOCIATION_FAILED", "/positions", "position count")
	}
	posSeen := map[string]bool{}
	for _, p := range t.Positions {
		if posSeen[p.MatchID] {
			return verr("ASSOCIATION_FAILED", "/positions/"+p.MatchID, "duplicate position")
		}
		posSeen[p.MatchID] = true
		var mm *Match
		for i := range t.Matches {
			if t.Matches[i].MatchID == p.MatchID {
				mm = &t.Matches[i]
			}
		}
		if mm == nil {
			return verr("ASSOCIATION_FAILED", "/positions", "no match")
		}
		got := positionFor([]byte(b.AdmittedSourceBytes[mm.SourceID]), mm.StartByte, mm.EndByte)
		got.MatchID = p.MatchID
		if *got != p {
			return verr("ASSOCIATION_FAILED", "/positions/"+p.MatchID, "utf16 position mismatch")
		}
	}
	return nil
}

func sourcePathRank(s Source) string { return strings.TrimPrefix(s.LogicalURI, "file:///") }

func validateCandidate(t Terminal) error {
	if t.Terminal == "COMPLETE" {
		c := t.RangeUnionCandidate
		if c == nil {
			return verr("INVARIANT_FAILED", "/range_union_candidate", "required")
		}
		if c.Operation != CandidateOperation || c.ExecutedLocation || !c.CandidateOnly || c.AdmissionDigest != t.Admission.AdmissionDigest {
			return verr("INVARIANT_FAILED", "/range_union_candidate", "bad candidate constants")
		}
		if len(c.MemberMatchIDs) != len(t.Matches) || len(c.QualifiedLocationPins) != len(t.Matches) {
			return verr("INVARIANT_FAILED", "/range_union_candidate/member_match_ids", "member count")
		}
		for i, m := range t.Matches {
			if c.MemberMatchIDs[i] != m.MatchID {
				return verr("INVARIANT_FAILED", "/range_union_candidate/member_match_ids", "not ordered matches")
			}
			pin := c.QualifiedLocationPins[i]
			pos := t.Positions[i]
			if pin.SourceID != m.SourceID || pin.StartByte != m.StartByte || pin.EndByte != m.EndByte || pin.StartLine != pos.StartLine || pin.StartCharacterUTF16 != pos.StartCharacterUTF16 || pin.EndLine != pos.EndLine || pin.EndCharacterUTF16 != pos.EndCharacterUTF16 {
				return verr("INVARIANT_FAILED", "/range_union_candidate/qualified_location_pins", "pin mismatch")
			}
		}
		if !strings.HasPrefix(c.CandidateDigest, "sha256:") {
			return verr("INVARIANT_FAILED", "/range_union_candidate/candidate_digest", "bad digest")
		}
	}
	return nil
}
func validateAccounting(t Terminal, term string) error {
	a := t.Accounting
	w, ok := weightedW(a)
	if !ok {
		return verr("OVERFLOW", "/accounting/W_work", "overflow")
	}
	if a.WWork != w {
		return verr("INVARIANT_FAILED", "/accounting/W_work", "bad weighted formula")
	}
	if uint64(len([]byte(term))) != a.BOutputBytes {
		return verr("INVARIANT_FAILED", "/accounting/B_output_bytes", "encoded length mismatch")
	}
	if got, ok := fixedPointB(t); !ok {
		return verr("INVARIANT_FAILED", "/accounting/B_output_bytes", "fixed point not reached")
	} else if got != a.BOutputBytes {
		return verr("INVARIANT_FAILED", "/accounting/B_output_bytes", "fixed point mismatch")
	}
	return nil
}
func weightedW(a Accounting) (uint64, bool) {
	vals := []struct{ v, m uint64 }{{a.JFiles, 3}, {a.QQueryBytes, 5}, {a.PPathBytes, 7}, {a.SSourceBytes, 1}, {a.TScannedTuples, 11}, {a.MMatches, 13}, {a.RRanges, 17}, {a.UUTF16Units, 19}, {a.BOutputBytes, 31}}
	total := uint64(50)
	for _, x := range vals {
		if x.v != 0 && x.v > math.MaxUint64/x.m {
			return 0, false
		}
		p := x.v * x.m
		if total > math.MaxUint64-p {
			return 0, false
		}
		total += p
	}
	return total, true
}
func fixedPointB(t Terminal) (uint64, bool) {
	x := t
	x.Custody.TerminalResultSHA256 = zeroSHA
	x.Replay.TerminalPreimageSHA256 = zeroSHA
	last := uint64(0)
	for i := 0; i < MaxIterations; i++ {
		b, _ := CanonicalJSON(x)
		n := uint64(len([]byte(b)))
		if n < last {
			return 0, false
		}
		x.Accounting.BOutputBytes = n
		w, ok := weightedW(x.Accounting)
		if !ok {
			return 0, false
		}
		x.Accounting.WWork = w
		if n == last {
			return n, true
		}
		last = n
	}
	return 0, false
}
func normalizedTerminalPreimage(t Terminal) string {
	x := t
	x.Custody.TerminalResultSHA256 = zeroSHA
	x.Replay.TerminalPreimageSHA256 = zeroSHA
	s, _ := CanonicalJSON(x)
	return s
}

func positionFor(data []byte, start, end uint64) *Position {
	sl, sc := lineChar(data, start)
	el, ec := lineChar(data, end)
	return &Position{StartLine: sl, StartCharacterUTF16: sc, EndLine: el, EndCharacterUTF16: ec}
}
func lineChar(data []byte, off uint64) (uint64, uint64) {
	line, ch := uint64(0), uint64(0)
	for i := uint64(0); i < off; {
		if data[i] == '\r' {
			if i+1 < off && data[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
			line++
			ch = 0
			continue
		}
		if data[i] == '\n' {
			i++
			line++
			ch = 0
			continue
		}
		r, n := utf8.DecodeRune(data[i:])
		ch += uint64(len(utf16.Encode([]rune{r})))
		i += uint64(n)
	}
	return line, ch
}
func digestOrEmpty(s string) string {
	if s == "" {
		return zeroSHA
	}
	if !strings.HasSuffix(s, "\n") {
		return ""
	}
	return sha(s)
}
func sourceIDs(sources []Source) []string {
	ids := make([]string, 0, len(sources))
	for _, s := range sources {
		ids = append(ids, s.SourceID)
	}
	return ids
}
func shaBytes(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func sha(s string) string      { h := sha256.Sum256([]byte(s)); return "sha256:" + hex.EncodeToString(h[:]) }
func parseU64(s string) (uint64, error) {
	if strings.ContainsAny(s, ".-+eE") {
		return 0, errors.New("not uint64")
	}
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not uint64")
		}
		d := uint64(c - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, errors.New("uint64 overflow")
		}
		n = n*10 + d
	}
	return n, nil
}

func CanonicalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var x any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&x); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	writeCanon(&buf, x)
	buf.WriteByte('\n')
	return buf.String(), nil
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
func decodeExact(b []byte, dst any, required []string) error {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	allow := map[string]bool{}
	for _, k := range required {
		allow[k] = true
		if _, ok := raw[k]; !ok {
			return fmt.Errorf("missing %s", k)
		}
	}
	for k := range raw {
		if !allow[k] {
			return fmt.Errorf("unknown %s", k)
		}
	}
	dec = json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
func walkSchema(b []byte) error {
	var s map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&s); err != nil {
		return verr("INVALID_INPUT", "/schema_bytes", err.Error())
	}
	if s["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		return verr("INVARIANT_FAILED", "/schema_bytes/$schema", "not draft2020-12")
	}
	if s["$id"] != TerminalSchemaVersion {
		return verr("INVARIANT_FAILED", "/schema_bytes/$id", "wrong id")
	}
	req, ok := s["required"].([]any)
	if !ok || len(req) < 10 {
		return verr("INVARIANT_FAILED", "/schema_bytes/required", "schema drift")
	}
	props, ok := s["properties"].(map[string]any)
	if !ok {
		return verr("INVARIANT_FAILED", "/schema_bytes/properties", "schema drift")
	}
	for _, k := range []string{"accounting", "custody", "replay", "range_union_candidate"} {
		if _, ok := props[k]; !ok {
			return verr("INVARIANT_FAILED", "/schema_bytes/properties/"+k, "missing")
		}
	}
	return nil
}
