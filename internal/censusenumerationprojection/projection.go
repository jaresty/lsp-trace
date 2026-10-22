// Package censusenumerationprojection owns the private, incomplete-discovery-only
// enumeration projection. It has no public result or acquisition authority.
package censusenumerationprojection

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

const (
	SchemaVersion         = "lsp-trace.census-enumeration-projection.v1"
	SelectorPrefix        = "census-enumeration-projections/v1/sha256/"
	MaxFiles              = 10000
	MaxSymbols            = 100000
	MaxCanonicalBytes     = 64 << 20
	MaxIdentityInputBytes = 16 << 20
	// MaxWork is an overflow ceiling, not an independently reachable bound:
	// canonical bytes plus identity bytes and cardinality are bounded below it.
	MaxWork         = 128 << 20
	identityDomain  = "lsp-trace:census-enumeration-symbol-identity:v1"
	workspaceDomain = "lsp-trace:census-workspace:v1"
)

type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}
type File struct {
	Ordinal     int    `json:"ordinal"`
	Path        string `json:"path"`
	Disposition string `json:"disposition"`
}
type Symbol struct {
	Ordinal        int      `json:"ordinal"`
	FileOrdinal    int      `json:"file_ordinal"`
	SelectionStart Position `json:"selection_start"`
	Kind           int      `json:"kind"`
	Disposition    string   `json:"disposition"`
	IdentityDigest string   `json:"identity_digest"`
}
type ProviderFailure struct {
	Stage         string `json:"stage"`
	Code          string `json:"code"`
	FileOrdinal   *int   `json:"file_ordinal,omitempty"`
	SymbolOrdinal *int   `json:"symbol_ordinal,omitempty"`
}
type Count struct {
	Disposition string `json:"disposition"`
	Count       int    `json:"count"`
}
type IncompleteCause struct {
	Scope         string `json:"scope"`
	Stage         string `json:"stage"`
	Code          string `json:"code"`
	FileOrdinal   *int   `json:"file_ordinal,omitempty"`
	SymbolOrdinal *int   `json:"symbol_ordinal,omitempty"`
}
type Observation struct {
	Dimension string  `json:"dimension"`
	State     string  `json:"state"`
	Observed  *uint64 `json:"observed,omitempty"`
	Limit     *uint64 `json:"limit,omitempty"`
	Omitted   *uint64 `json:"omitted,omitempty"`
}
type Bounds struct {
	MaxFiles              int    `json:"max_files"`
	MaxSymbols            int    `json:"max_symbols"`
	MaxCanonicalBytes     int    `json:"max_canonical_bytes"`
	MaxIdentityInputBytes int    `json:"max_identity_input_bytes"`
	MaxWork               uint64 `json:"max_work"`
	RequestDownDepth      uint64 `json:"request_down_depth"`
	RequestUpDepth        uint64 `json:"request_up_depth"`
	RequestMaxNodes       uint64 `json:"request_max_nodes"`
	RequestBatchTargets   uint64 `json:"request_batch_targets"`
	RequestTimeoutMS      uint64 `json:"request_timeout_ms"`
	TimeoutMS             uint64 `json:"timeout_ms"`
}
type Projection struct {
	SchemaVersion           string            `json:"schema_version"`
	Status                  string            `json:"status"`
	SessionID               string            `json:"session_id"`
	Generation              uint64            `json:"generation"`
	RequestFingerprint      string            `json:"request_fingerprint"`
	WorkspaceURIDigest      string            `json:"workspace_uri_digest"`
	SymbolScope             string            `json:"symbol_scope"`
	WorkspaceSymbolComplete bool              `json:"workspace_symbol_complete"`
	Authority               int               `json:"authority"`
	SourceGraphComplete     string            `json:"source_graph_complete"`
	AbsenceInference        string            `json:"absence_inference"`
	TargetAcquisition       bool              `json:"target_acquisition"`
	Files                   []File            `json:"files"`
	Symbols                 []Symbol          `json:"symbols"`
	FileDispositionCounts   []Count           `json:"file_disposition_counts"`
	SymbolDispositionCounts []Count           `json:"symbol_disposition_counts"`
	DiscoveryObservations   []Observation     `json:"discovery_observations"`
	ProviderFailures        []ProviderFailure `json:"provider_failures"`
	IncompleteCauses        []IncompleteCause `json:"incomplete_causes"`
	Bounds                  Bounds            `json:"bounds"`
	Work                    uint64            `json:"work"`
}
type Input struct {
	SessionID                                                                                           string
	Generation                                                                                          uint64
	RequestFingerprint, WorkspaceURI                                                                    string
	Files                                                                                               []File
	Symbols                                                                                             []Symbol
	Observations                                                                                        []Observation
	ProviderFailures                                                                                    []ProviderFailure
	IncompleteCauses                                                                                    []IncompleteCause
	RequestDownDepth, RequestUpDepth, RequestMaxNodes, RequestBatchTargets, RequestTimeoutMS, TimeoutMS uint64
}
type EncodingFailureCategory string

const (
	EncodingBoundExhaustion EncodingFailureCategory = "BOUND_EXHAUSTION"
	EncodingInvalidInput    EncodingFailureCategory = "INVALID_INPUT"
	EncodingOtherFailure    EncodingFailureCategory = "OTHER_ENCODING_FAILURE"
)

type encodingFailure struct{ category EncodingFailureCategory }

func (e encodingFailure) Error() string { return string(e.category) }
func EncodingFailureCategoryOf(err error) EncodingFailureCategory {
	var failure encodingFailure
	if errors.As(err, &failure) {
		return failure.category
	}
	return EncodingOtherFailure
}
func encodingError(category EncodingFailureCategory) error {
	return encodingFailure{category: category}
}

type Receipt struct {
	SchemaVersion           string                  `json:"schema_version"`
	Status                  string                  `json:"status"`
	Selector                string                  `json:"selector,omitempty"`
	Digest                  string                  `json:"digest,omitempty"`
	ByteLength              uint64                  `json:"byte_length,omitempty"`
	Mechanism               string                  `json:"mechanism,omitempty"`
	NamespaceAtomic         bool                    `json:"namespace_atomic,omitempty"`
	VerificationStatus      string                  `json:"verification_status,omitempty"`
	DirectorySyncStatus     string                  `json:"directory_sync_status,omitempty"`
	CloseStatus             string                  `json:"close_status,omitempty"`
	UnavailableReason       string                  `json:"unavailable_reason,omitempty"`
	EncodingFailureCategory EncodingFailureCategory `json:"-"`
}

func SymbolIdentityDigest(filePath string, start Position, kind int, _ string, ordinal int) (string, int, error) {
	if filePath == "" || ordinal < 0 || kind < 0 {
		return "", 0, errors.New("invalid symbol identity")
	}
	var b bytes.Buffer
	b.WriteString(identityDomain)
	b.WriteByte(0)
	if len(filePath) > math.MaxUint32 {
		return "", 0, errors.New("identity too large")
	}
	_ = binary.Write(&b, binary.BigEndian, uint32(len(filePath)))
	b.WriteString(filePath)
	for _, n := range []uint64{uint64(start.Line), uint64(start.Character), uint64(kind), uint64(ordinal)} {
		_ = binary.Write(&b, binary.BigEndian, n)
	}
	s := sha256.Sum256(b.Bytes())
	return "sha256:" + hex.EncodeToString(s[:]), b.Len(), nil
}
func WorkspaceDigest(uri string) (string, error) {
	u, e := url.Parse(uri)
	if e != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.String() != uri {
		return "", errors.New("canonical workspace URI required")
	}
	s := sha256.Sum256([]byte(workspaceDomain + "\x00" + uri))
	return "sha256:" + hex.EncodeToString(s[:]), nil
}

var (
	encodeCanonicalLimit = MaxCanonicalBytes
	decodeCanonicalLimit = MaxCanonicalBytes
	identityInputLimit   = MaxIdentityInputBytes
)

func Build(in Input) (Projection, error) {
	wd, err := WorkspaceDigest(in.WorkspaceURI)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{SchemaVersion: SchemaVersion, Status: "INCOMPLETE", SessionID: in.SessionID, Generation: in.Generation, RequestFingerprint: in.RequestFingerprint, WorkspaceURIDigest: wd, SymbolScope: "ENUMERATED_FROM_SUCCESSFUL_DOCUMENT_SYMBOL_RESPONSES", WorkspaceSymbolComplete: false, Authority: 0, SourceGraphComplete: "UNKNOWN", AbsenceInference: "PROHIBITED", TargetAcquisition: false, Files: append([]File(nil), in.Files...), Symbols: append([]Symbol(nil), in.Symbols...), DiscoveryObservations: append([]Observation(nil), in.Observations...), ProviderFailures: append([]ProviderFailure(nil), in.ProviderFailures...), IncompleteCauses: append([]IncompleteCause(nil), in.IncompleteCauses...), Bounds: Bounds{MaxFiles: MaxFiles, MaxSymbols: MaxSymbols, MaxCanonicalBytes: MaxCanonicalBytes, MaxIdentityInputBytes: MaxIdentityInputBytes, MaxWork: MaxWork, RequestDownDepth: in.RequestDownDepth, RequestUpDepth: in.RequestUpDepth, RequestMaxNodes: in.RequestMaxNodes, RequestBatchTargets: in.RequestBatchTargets, RequestTimeoutMS: in.RequestTimeoutMS, TimeoutMS: in.TimeoutMS}}
	cloneProjectionPointers(&p)
	sort.Slice(p.ProviderFailures, func(i, j int) bool { return failureRank(p.ProviderFailures[i]) < failureRank(p.ProviderFailures[j]) })
	sort.Slice(p.IncompleteCauses, func(i, j int) bool { return causeRank(p.IncompleteCauses[i]) < causeRank(p.IncompleteCauses[j]) })
	p.DiscoveryObservations, err = canonicalObservations(p.DiscoveryObservations)
	if err != nil {
		return Projection{}, err
	}
	p.FileDispositionCounts = countsFiles(p.Files)
	p.SymbolDispositionCounts = countsSymbols(p.Symbols)
	raw, err := encodeWithWork(p)
	if err != nil {
		return Projection{}, err
	}
	return Decode(raw)
}
func cloneProjectionPointers(p *Projection) {
	clone := func(v *uint64) *uint64 {
		if v == nil {
			return nil
		}
		n := *v
		return &n
	}
	cloneInt := func(v *int) *int {
		if v == nil {
			return nil
		}
		n := *v
		return &n
	}
	for i := range p.DiscoveryObservations {
		p.DiscoveryObservations[i].Observed = clone(p.DiscoveryObservations[i].Observed)
		p.DiscoveryObservations[i].Limit = clone(p.DiscoveryObservations[i].Limit)
		p.DiscoveryObservations[i].Omitted = clone(p.DiscoveryObservations[i].Omitted)
	}
	for i := range p.ProviderFailures {
		p.ProviderFailures[i].FileOrdinal = cloneInt(p.ProviderFailures[i].FileOrdinal)
		p.ProviderFailures[i].SymbolOrdinal = cloneInt(p.ProviderFailures[i].SymbolOrdinal)
	}
	for i := range p.IncompleteCauses {
		p.IncompleteCauses[i].FileOrdinal = cloneInt(p.IncompleteCauses[i].FileOrdinal)
		p.IncompleteCauses[i].SymbolOrdinal = cloneInt(p.IncompleteCauses[i].SymbolOrdinal)
	}
}

func countsFiles(v []File) []Count {
	m := map[string]int{}
	for _, e := range v {
		m[e.Disposition]++
	}
	return sortedCounts(m)
}
func countsSymbols(v []Symbol) []Count {
	m := map[string]int{}
	for _, e := range v {
		m[e.Disposition]++
	}
	return sortedCounts(m)
}
func sortedCounts(m map[string]int) []Count {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return dispositionRank(keys[i]) < dispositionRank(keys[j]) })
	out := make([]Count, 0, len(keys))
	for _, k := range keys {
		out = append(out, Count{k, m[k]})
	}
	return out
}

var observationOrder = []string{"NODE_CEILING", "DEPTH_BOUNDARY", "TARGET_SOURCE_OMISSIONS", "PROVIDER_COMPLETENESS", "TIMEOUT_REQUEST_BUDGET", "MESSAGE_BYTE_BUDGET"}

func canonicalObservations(in []Observation) ([]Observation, error) {
	by := make(map[string]Observation, len(in))
	valid := map[string]bool{}
	for _, dimension := range observationOrder {
		valid[dimension] = true
	}
	for _, o := range in {
		if !valid[o.Dimension] {
			return nil, errors.New("unknown observation dimension")
		}
		if _, exists := by[o.Dimension]; exists {
			return nil, errors.New("duplicate observation dimension")
		}
		by[o.Dimension] = o
	}
	out := make([]Observation, len(observationOrder))
	for i, dimension := range observationOrder {
		o, ok := by[dimension]
		if !ok {
			o = Observation{Dimension: dimension, State: "UNKNOWN"}
		}
		out[i] = o
	}
	return out, nil
}

func dispositionRank(s string) string {
	order := []string{"processed", "selected", "excluded", "unreadable", "unsupported", "document-symbol-failed", "non-callable", "prepare-missing", "preparation-failed", "incomplete"}
	for i, v := range order {
		if s == v {
			return fmt.Sprintf("%02d", i)
		}
	}
	return "99:" + s
}

func failureRank(f ProviderFailure) string {
	stage := enumRank(f.Stage, []string{"DOCUMENT_SUPPLY", "DOCUMENT_SYMBOL", "DOCUMENT_SYMBOL_FLATTEN", "CALL_HIERARCHY_CAPABILITY", "PREPARE_CALL_HIERARCHY"})
	code := enumRank(f.Code, []string{"UNAVAILABLE", "REQUEST_FAILED", "UNSUPPORTED", "NODE_LIMIT", "DEPTH_LIMIT", "STRING_BYTE_LIMIT", "RESULT_MISMATCH", "CANCELED"})
	return fmt.Sprintf("%08d:%08d:%02d:%02d", ordinalOrMax(f.FileOrdinal, MaxFiles), ordinalOrMax(f.SymbolOrdinal, MaxSymbols), stage, code)
}
func causeRank(c IncompleteCause) string {
	scope := enumRank(c.Scope, []string{"FILE", "SYMBOL"})
	stage := enumRank(c.Stage, []string{"DISCOVERY", "DOCUMENT_SUPPLY", "DOCUMENT_SYMBOL_FLATTEN", "PREPARE_CALL_HIERARCHY"})
	code := enumRank(c.Code, []string{"CANCELED", "FILE_LIMIT", "SYMBOL_LIMIT", "TARGET_LIMIT", "SESSION_POISONED", "LIFECYCLE_CONFLICT", "SESSION_UNAVAILABLE", "NODE_LIMIT", "DEPTH_LIMIT", "STRING_BYTE_LIMIT", "RESULT_MISMATCH", "COORDINATE_OVERFLOW", "CHILD_INCOMPLETE"})
	return fmt.Sprintf("%02d:%08d:%08d:%02d:%02d", scope, ordinalOrMax(c.FileOrdinal, MaxFiles), ordinalOrMax(c.SymbolOrdinal, MaxSymbols), stage, code)
}
func enumRank(value string, order []string) int {
	for i, candidate := range order {
		if value == candidate {
			return i
		}
	}
	return len(order)
}
func ordinalOrMax(v *int, max int) int {
	if v != nil {
		return *v
	}
	return max + 1
}

func encodeWithWork(p Projection) ([]byte, error) {
	identity := uint64(0)
	for _, s := range p.Symbols {
		if s.FileOrdinal < 0 || s.FileOrdinal >= len(p.Files) {
			return nil, encodingError(EncodingInvalidInput)
		}
		var ok bool
		identity, ok = add(identity, uint64(identityInputSize(s, p.Files[s.FileOrdinal].Path)))
		if !ok || identity > uint64(identityInputLimit) {
			return nil, encodingError(EncodingBoundExhaustion)
		}
	}
	base, ok := add(uint64(len(p.Files)), uint64(len(p.Symbols)))
	if !ok {
		return nil, encodingError(EncodingBoundExhaustion)
	}
	base, ok = add(base, identity)
	if !ok {
		return nil, encodingError(EncodingBoundExhaustion)
	}
	p.Work = 0
	for {
		raw, err := json.Marshal(p)
		if err != nil {
			return nil, encodingError(EncodingOtherFailure)
		}
		if len(raw) > encodeCanonicalLimit {
			return nil, encodingError(EncodingBoundExhaustion)
		}
		work, ok := add(base, uint64(len(raw)))
		if !ok || work > MaxWork {
			return nil, encodingError(EncodingBoundExhaustion)
		}
		if p.Work == work {
			return raw, nil
		}
		p.Work = work
	}
}
func identityInputSize(s Symbol, filePath string) int {
	return len(identityDomain) + 1 + 4 + len(filePath) + 8*4
}
func add(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, false
	}
	return a + b, true
}
func EncodeCanonical(p Projection) ([]byte, error) {
	if p.Work != 0 {
		raw, err := json.Marshal(p)
		if err != nil || validate(p, raw) != nil {
			return nil, encodingError(EncodingInvalidInput)
		}
	}
	raw, e := encodeWithWork(p)
	if e != nil {
		return nil, e
	}
	q, e := Decode(raw)
	if e != nil {
		return nil, e
	}
	raw2, e := json.Marshal(q)
	if e != nil || !bytes.Equal(raw, raw2) {
		return nil, errors.New("noncanonical projection")
	}
	return raw, nil
}
func Decode(raw []byte) (Projection, error) {
	var p Projection
	if len(raw) == 0 || !withinLimit(uint64(len(raw)), decodeCanonicalLimit) || strictjson.RejectDuplicates(raw) != nil {
		return p, errors.New("invalid projection JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil {
		return Projection{}, errors.New("invalid projection JSON")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return Projection{}, errors.New("trailing projection JSON")
	}
	if err := validate(p, raw); err != nil {
		return Projection{}, err
	}
	canonical, _ := json.Marshal(p)
	if !bytes.Equal(raw, canonical) {
		return Projection{}, errors.New("noncanonical projection")
	}
	return p, nil
}
func validate(p Projection, raw []byte) error {
	if p.SchemaVersion != SchemaVersion || p.Status != "INCOMPLETE" || p.SessionID == "" || p.Generation == 0 || !digest(p.RequestFingerprint) || !digest(p.WorkspaceURIDigest) || p.SymbolScope != "ENUMERATED_FROM_SUCCESSFUL_DOCUMENT_SYMBOL_RESPONSES" || p.WorkspaceSymbolComplete || p.Authority != 0 || p.SourceGraphComplete != "UNKNOWN" || p.AbsenceInference != "PROHIBITED" || p.TargetAcquisition {
		return errors.New("invalid projection invariants")
	}
	if p.Bounds.MaxFiles != MaxFiles || p.Bounds.MaxSymbols != MaxSymbols || p.Bounds.MaxCanonicalBytes != MaxCanonicalBytes || p.Bounds.MaxIdentityInputBytes != MaxIdentityInputBytes || p.Bounds.MaxWork != MaxWork || len(p.Files) > MaxFiles || len(p.Symbols) > MaxSymbols {
		return errors.New("invalid projection bounds")
	}
	identityBytes := uint64(0)
	fileDisposition := map[int]string{}
	for i, f := range p.Files {
		if f.Ordinal != i || !validWorkspaceRelativePath(f.Path) || !validFileDisposition(f.Disposition) {
			return errors.New("invalid file ledger")
		}
		fileDisposition[i] = f.Disposition
	}
	for i, s := range p.Symbols {
		if s.Ordinal != i || s.FileOrdinal < 0 || s.FileOrdinal >= len(p.Files) || !validSymbolDisposition(s.Disposition) || !digest(s.IdentityDigest) {
			return errors.New("invalid symbol ledger")
		}
		filePath := p.Files[s.FileOrdinal].Path
		var ok bool
		identityBytes, ok = add(identityBytes, uint64(identityInputSize(s, filePath)))
		if !ok {
			return errors.New("identity input overflow")
		}
		wantDigest, _, err := SymbolIdentityDigest(filePath, s.SelectionStart, s.Kind, "", s.Ordinal)
		if err != nil || s.IdentityDigest != wantDigest {
			return errors.New("symbol identity digest mismatch")
		}
	}
	if !withinLimit(identityBytes, identityInputLimit) {
		return errors.New("identity input limit exceeded")
	}
	if !equalCounts(p.FileDispositionCounts, countsFiles(p.Files)) || !equalCounts(p.SymbolDispositionCounts, countsSymbols(p.Symbols)) {
		return errors.New("disposition count mismatch")
	}
	fileFailures := make(map[int]int)
	symbolFailures := make(map[int]int)
	for i, f := range p.ProviderFailures {
		if !validProviderPair(f) || (f.FileOrdinal == nil) == (f.SymbolOrdinal == nil) || i > 0 && failureRank(p.ProviderFailures[i-1]) >= failureRank(f) {
			return errors.New("invalid provider failure")
		}
		if f.FileOrdinal != nil {
			if *f.FileOrdinal < 0 || *f.FileOrdinal >= len(p.Files) || !fileFailureMatches(f, fileDisposition[*f.FileOrdinal]) {
				return errors.New("provider failure reconciliation")
			}
			fileFailures[*f.FileOrdinal]++
		}
		if f.SymbolOrdinal != nil {
			if *f.SymbolOrdinal < 0 || *f.SymbolOrdinal >= len(p.Symbols) || !symbolFailureMatches(f, p.Symbols[*f.SymbolOrdinal].Disposition) {
				return errors.New("provider failure reconciliation")
			}
			symbolFailures[*f.SymbolOrdinal]++
		}
	}
	if err := validateObservations(p.DiscoveryObservations); err != nil {
		return err
	}
	if err := validateCauses(p); err != nil {
		return err
	}
	for i, f := range p.Files {
		required := f.Disposition == "unreadable" || f.Disposition == "unsupported" || f.Disposition == "document-symbol-failed"
		if required && fileFailures[i] != 1 || !required && f.Disposition != "incomplete" && fileFailures[i] != 0 || fileFailures[i] > 1 {
			return errors.New("provider failure reconciliation")
		}
	}
	for i, s := range p.Symbols {
		required := s.Disposition == "unsupported" || s.Disposition == "prepare-missing" || s.Disposition == "preparation-failed"
		if required && symbolFailures[i] != 1 || !required && s.Disposition != "incomplete" && symbolFailures[i] != 0 || symbolFailures[i] > 1 {
			return errors.New("provider failure reconciliation")
		}
	}
	work, ok := add(uint64(len(p.Files)), uint64(len(p.Symbols)))
	if !ok {
		return errors.New("work overflow")
	}
	work, ok = add(work, identityBytes)
	if !ok {
		return errors.New("work overflow")
	}
	work, ok = add(work, uint64(len(raw)))
	if !ok || work != p.Work || !withinLimit(work, MaxWork) {
		return errors.New("work mismatch")
	}
	return nil
}
func failureKey(f ProviderFailure) string {
	file, symbol := MaxFiles+1, MaxSymbols+1
	if f.FileOrdinal != nil {
		file = *f.FileOrdinal
	}
	if f.SymbolOrdinal != nil {
		symbol = *f.SymbolOrdinal
	}
	return fmt.Sprintf("%08d:%08d:%s:%s", file, symbol, f.Stage, f.Code)
}
func fileFailureMatches(f ProviderFailure, disposition string) bool {
	switch f.Stage {
	case "DOCUMENT_SUPPLY":
		return disposition == "unreadable" && (f.Code == "UNAVAILABLE" || f.Code == "REQUEST_FAILED" || f.Code == "CANCELED") || disposition == "incomplete" && f.Code == "CANCELED"
	case "DOCUMENT_SYMBOL":
		return disposition == "unsupported" && f.Code == "UNSUPPORTED" || disposition == "document-symbol-failed" && (f.Code == "REQUEST_FAILED" || f.Code == "CANCELED") || disposition == "incomplete" && f.Code == "CANCELED"
	case "DOCUMENT_SYMBOL_FLATTEN":
		return disposition == "incomplete"
	default:
		return false
	}
}
func symbolFailureMatches(f ProviderFailure, disposition string) bool {
	switch f.Stage {
	case "CALL_HIERARCHY_CAPABILITY":
		return f.Code == "UNSUPPORTED" && disposition == "unsupported"
	case "PREPARE_CALL_HIERARCHY":
		return f.Code == "UNAVAILABLE" && disposition == "prepare-missing" || f.Code == "REQUEST_FAILED" && disposition == "preparation-failed" || f.Code == "CANCELED" && (disposition == "preparation-failed" || disposition == "incomplete") || f.Code == "RESULT_MISMATCH" && disposition == "incomplete"
	default:
		return false
	}
}
func validFileDisposition(s string) bool {
	switch s {
	case "processed", "selected", "excluded", "unreadable", "unsupported", "document-symbol-failed", "incomplete":
		return true
	}
	return false
}
func validSymbolDisposition(s string) bool {
	switch s {
	case "selected", "non-callable", "unsupported", "prepare-missing", "preparation-failed", "incomplete":
		return true
	}
	return false
}
func validProviderPair(f ProviderFailure) bool {
	pairs := map[string]map[string]bool{
		"DOCUMENT_SUPPLY":           {"UNAVAILABLE": true, "REQUEST_FAILED": true, "CANCELED": true},
		"DOCUMENT_SYMBOL":           {"UNSUPPORTED": true, "REQUEST_FAILED": true, "CANCELED": true},
		"DOCUMENT_SYMBOL_FLATTEN":   {"NODE_LIMIT": true, "DEPTH_LIMIT": true, "STRING_BYTE_LIMIT": true},
		"CALL_HIERARCHY_CAPABILITY": {"UNSUPPORTED": true},
		"PREPARE_CALL_HIERARCHY":    {"UNAVAILABLE": true, "REQUEST_FAILED": true, "CANCELED": true, "RESULT_MISMATCH": true},
	}
	return pairs[f.Stage][f.Code]
}
func validateObservations(v []Observation) error {
	if len(v) != len(observationOrder) {
		return errors.New("invalid observation cardinality")
	}
	for i, o := range v {
		if o.Dimension != observationOrder[i] || (o.State != "KNOWN" && o.State != "UNKNOWN") {
			return errors.New("invalid observation vocabulary")
		}
		if o.State == "UNKNOWN" {
			if o.Observed != nil || o.Limit != nil || o.Omitted != nil {
				return errors.New("unknown observation has numeric values")
			}
			continue
		}
		valid := false
		switch o.Dimension {
		case "NODE_CEILING":
			valid = o.Observed != nil && o.Limit != nil && o.Omitted != nil
		case "DEPTH_BOUNDARY", "MESSAGE_BYTE_BUDGET":
			valid = o.Observed == nil && o.Limit != nil && o.Omitted != nil
		default:
			valid = o.Observed == nil && o.Limit == nil && o.Omitted != nil
		}
		if !valid {
			return errors.New("invalid known observation numeric shape")
		}
	}
	return nil
}
func validCausePair(c IncompleteCause) bool {
	if c.Scope == "FILE" {
		if c.FileOrdinal == nil || c.SymbolOrdinal != nil {
			return false
		}
		return c.Stage == "DISCOVERY" && (c.Code == "CANCELED" || c.Code == "FILE_LIMIT" || c.Code == "CHILD_INCOMPLETE") || c.Stage == "DOCUMENT_SUPPLY" && (c.Code == "SESSION_POISONED" || c.Code == "LIFECYCLE_CONFLICT" || c.Code == "SESSION_UNAVAILABLE") || c.Stage == "DOCUMENT_SYMBOL_FLATTEN" && (c.Code == "NODE_LIMIT" || c.Code == "DEPTH_LIMIT" || c.Code == "STRING_BYTE_LIMIT")
	}
	if c.Scope == "SYMBOL" {
		if c.SymbolOrdinal == nil || c.FileOrdinal != nil {
			return false
		}
		return c.Stage == "DISCOVERY" && (c.Code == "CANCELED" || c.Code == "SYMBOL_LIMIT" || c.Code == "TARGET_LIMIT" || c.Code == "COORDINATE_OVERFLOW") || c.Stage == "PREPARE_CALL_HIERARCHY" && c.Code == "RESULT_MISMATCH"
	}
	return false
}
func validateCauses(p Projection) error {
	fc, sc := map[int]int{}, map[int]int{}
	for i, c := range p.IncompleteCauses {
		if !validCausePair(c) || i > 0 && causeRank(p.IncompleteCauses[i-1]) >= causeRank(c) {
			return errors.New("invalid incomplete cause")
		}
		if c.FileOrdinal != nil {
			if *c.FileOrdinal < 0 || *c.FileOrdinal >= len(p.Files) || p.Files[*c.FileOrdinal].Disposition != "incomplete" {
				return errors.New("cause reconciliation")
			}
			fc[*c.FileOrdinal]++
		}
		if c.SymbolOrdinal != nil {
			if *c.SymbolOrdinal < 0 || *c.SymbolOrdinal >= len(p.Symbols) || p.Symbols[*c.SymbolOrdinal].Disposition != "incomplete" {
				return errors.New("cause reconciliation")
			}
			sc[*c.SymbolOrdinal]++
		}
	}
	for i, f := range p.Files {
		if (f.Disposition == "incomplete") != (fc[i] == 1) {
			return errors.New("file incomplete cause cardinality")
		}
	}
	for i, s := range p.Symbols {
		if (s.Disposition == "incomplete") != (sc[i] == 1) {
			return errors.New("symbol incomplete cause cardinality")
		}
	}
	return nil
}

func validStage(s string) bool {
	switch s {
	case "DOCUMENT_SUPPLY", "DOCUMENT_SYMBOL", "DOCUMENT_SYMBOL_FLATTEN", "CALL_HIERARCHY_CAPABILITY", "PREPARE_CALL_HIERARCHY":
		return true
	}
	return false
}
func validCode(s string) bool {
	switch s {
	case "UNAVAILABLE", "REQUEST_FAILED", "UNSUPPORTED", "NODE_LIMIT", "DEPTH_LIMIT", "STRING_BYTE_LIMIT", "RESULT_MISMATCH", "CANCELED":
		return true
	}
	return false
}
func withinLimit(value uint64, limit int) bool {
	return limit >= 0 && value <= uint64(limit)
}

func validWorkspaceRelativePath(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, 0) || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || path.IsAbs(value) || path.Clean(value) != value {
		return false
	}
	parts := strings.Split(value, "/")
	for i, part := range parts {
		if part == "" || part == "." || part == ".." || i == 0 && strings.Contains(part, ":") {
			return false
		}
	}
	return true
}

func digest(s string) bool {
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil && strings.ToLower(s) == s
}
func equalCounts(a, b []Count) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func Selector(raw []byte) string {
	s := sha256.Sum256(raw)
	return SelectorPrefix + hex.EncodeToString(s[:]) + ".json"
}

var publishBoundFile = publication.PublishBoundFile

func Publish(root *publication.Root, p Projection) (Receipt, error) {
	raw, e := EncodeCanonical(p)
	if e != nil {
		return Receipt{}, e
	}
	selector := Selector(raw)
	r, e := publishBoundFile(root, selector, raw, func(got []byte) error {
		q, x := Decode(got)
		if x != nil {
			return x
		}
		again, x := EncodeCanonical(q)
		if x != nil || !bytes.Equal(got, again) {
			return errors.New("committed projection not canonical")
		}
		return nil
	})
	if e != nil {
		return Receipt{}, e
	}
	return Receipt{SchemaVersion: SchemaVersion, Status: "PUBLISHED", Selector: r.FinalSelector, Digest: r.Digest, ByteLength: r.ByteLength, Mechanism: r.Mechanism, NamespaceAtomic: r.NamespaceAtomic, VerificationStatus: r.VerificationStatus, DirectorySyncStatus: r.DirectorySyncStatus, CloseStatus: r.CloseStatus}, nil
}
func Unavailable(reason string) Receipt {
	return Receipt{SchemaVersion: SchemaVersion, Status: "UNAVAILABLE", UnavailableReason: reason}
}
func UnavailableEncodingFailure(category EncodingFailureCategory) (Receipt, error) {
	switch category {
	case EncodingBoundExhaustion, EncodingInvalidInput, EncodingOtherFailure:
		return Receipt{SchemaVersion: SchemaVersion, Status: "UNAVAILABLE", UnavailableReason: "ENCODE_FAILED", EncodingFailureCategory: category}, nil
	default:
		return Receipt{}, errors.New("invalid encoding failure category")
	}
}
func MarshalReceipt(r Receipt) ([]byte, error) {
	if r.SchemaVersion != SchemaVersion || (r.Status != "PUBLISHED" && r.Status != "UNAVAILABLE") {
		return nil, errors.New("invalid receipt")
	}
	if r.Status == "UNAVAILABLE" {
		switch r.UnavailableReason {
		case "ENCODE_FAILED", "PUBLICATION_FAILED", "CANCELED_PRE_RECONCILIATION", "CANCELED_PRE_PUBLICATION":
		default:
			return nil, fmt.Errorf("invalid unavailable reason")
		}
		if r.Selector != "" || r.Digest != "" || r.ByteLength != 0 || r.Mechanism != "" || r.NamespaceAtomic || r.VerificationStatus != "" || r.DirectorySyncStatus != "" || r.CloseStatus != "" {
			return nil, errors.New("unavailable receipt has publication fields")
		}
	} else {
		if r.UnavailableReason != "" || r.Selector == "" || r.Digest == "" || r.ByteLength == 0 || r.Mechanism == "" || !r.NamespaceAtomic || r.VerificationStatus == "" || r.DirectorySyncStatus == "" || r.CloseStatus == "" {
			return nil, errors.New("incomplete published receipt")
		}
	}
	b, e := json.Marshal(r)
	return append(b, '\n'), e
}
