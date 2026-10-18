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
	MaxWork               = 128 << 20
	identityDomain        = "lsp-trace:census-enumeration-symbol-identity:v1"
	workspaceDomain       = "lsp-trace:census-workspace:v1"
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
	RequestDownDepth, RequestUpDepth, RequestMaxNodes, RequestBatchTargets, RequestTimeoutMS, TimeoutMS uint64
}
type Receipt struct {
	SchemaVersion       string `json:"schema_version"`
	Status              string `json:"status"`
	Selector            string `json:"selector,omitempty"`
	Digest              string `json:"digest,omitempty"`
	ByteLength          uint64 `json:"byte_length,omitempty"`
	Mechanism           string `json:"mechanism,omitempty"`
	NamespaceAtomic     bool   `json:"namespace_atomic,omitempty"`
	VerificationStatus  string `json:"verification_status,omitempty"`
	DirectorySyncStatus string `json:"directory_sync_status,omitempty"`
	CloseStatus         string `json:"close_status,omitempty"`
	UnavailableReason   string `json:"unavailable_reason,omitempty"`
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
func Build(in Input) (Projection, error) {
	wd, err := WorkspaceDigest(in.WorkspaceURI)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{SchemaVersion: SchemaVersion, Status: "INCOMPLETE", SessionID: in.SessionID, Generation: in.Generation, RequestFingerprint: in.RequestFingerprint, WorkspaceURIDigest: wd, SymbolScope: "ENUMERATED_FROM_SUCCESSFUL_DOCUMENT_SYMBOL_RESPONSES", WorkspaceSymbolComplete: false, Authority: 0, SourceGraphComplete: "UNKNOWN", AbsenceInference: "PROHIBITED", TargetAcquisition: false, Files: append([]File(nil), in.Files...), Symbols: append([]Symbol(nil), in.Symbols...), DiscoveryObservations: append([]Observation(nil), in.Observations...), ProviderFailures: append([]ProviderFailure(nil), in.ProviderFailures...), Bounds: Bounds{MaxFiles: MaxFiles, MaxSymbols: MaxSymbols, MaxCanonicalBytes: MaxCanonicalBytes, MaxIdentityInputBytes: MaxIdentityInputBytes, MaxWork: MaxWork, RequestDownDepth: in.RequestDownDepth, RequestUpDepth: in.RequestUpDepth, RequestMaxNodes: in.RequestMaxNodes, RequestBatchTargets: in.RequestBatchTargets, RequestTimeoutMS: in.RequestTimeoutMS, TimeoutMS: in.TimeoutMS}}
	sort.Slice(p.ProviderFailures, func(i, j int) bool { return failureKey(p.ProviderFailures[i]) < failureKey(p.ProviderFailures[j]) })
	p.FileDispositionCounts = countsFiles(p.Files)
	p.SymbolDispositionCounts = countsSymbols(p.Symbols)
	raw, err := encodeWithWork(p)
	if err != nil {
		return Projection{}, err
	}
	return Decode(raw)
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
	sort.Strings(keys)
	out := make([]Count, 0, len(keys))
	for _, k := range keys {
		out = append(out, Count{k, m[k]})
	}
	return out
}
func encodeWithWork(p Projection) ([]byte, error) {
	p.Work = 0
	base, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	identity := 0
	for _, s := range p.Symbols {
		if s.FileOrdinal < 0 || s.FileOrdinal >= len(p.Files) {
			return nil, errors.New("invalid symbol file ordinal")
		}
		identity += identityInputSize(s, p.Files[s.FileOrdinal].Path)
	}
	work, ok := add(uint64(len(p.Files)+len(p.Symbols)), uint64(identity))
	if !ok {
		return nil, errors.New("work overflow")
	}
	work, ok = add(work, uint64(len(base)))
	if !ok || work > MaxWork {
		return nil, errors.New("work limit exceeded")
	}
	p.Work = work
	raw, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	delta := len(raw) - len(base)
	p.Work, ok = add(p.Work, uint64(delta))
	if !ok || p.Work > MaxWork {
		return nil, errors.New("work limit exceeded")
	}
	return json.Marshal(p)
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
			return nil, errors.New("invalid projection work")
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
	if len(raw) == 0 || len(raw) > MaxCanonicalBytes || strictjson.RejectDuplicates(raw) != nil {
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
	identityBytes := 0
	fileDisposition := map[int]string{}
	for i, f := range p.Files {
		if f.Ordinal != i || f.Path == "" || path.IsAbs(f.Path) || path.Clean(f.Path) != f.Path || strings.Contains(f.Path, "\\") || f.Disposition == "" {
			return errors.New("invalid file ledger")
		}
		fileDisposition[i] = f.Disposition
	}
	for i, s := range p.Symbols {
		if s.Ordinal != i || s.FileOrdinal < 0 || s.FileOrdinal >= len(p.Files) || s.Disposition == "" || !digest(s.IdentityDigest) {
			return errors.New("invalid symbol ledger")
		}
		filePath := p.Files[s.FileOrdinal].Path
		identityBytes += identityInputSize(s, filePath)
		wantDigest, _, err := SymbolIdentityDigest(filePath, s.SelectionStart, s.Kind, "", s.Ordinal)
		if err != nil || s.IdentityDigest != wantDigest {
			return errors.New("symbol identity digest mismatch")
		}
	}
	if identityBytes > MaxIdentityInputBytes {
		return errors.New("identity input limit exceeded")
	}
	if !equalCounts(p.FileDispositionCounts, countsFiles(p.Files)) || !equalCounts(p.SymbolDispositionCounts, countsSymbols(p.Symbols)) {
		return errors.New("disposition count mismatch")
	}
	for i, f := range p.ProviderFailures {
		if !validStage(f.Stage) || !validCode(f.Code) || (f.FileOrdinal == nil) == (f.SymbolOrdinal == nil) || i > 0 && failureKey(p.ProviderFailures[i-1]) >= failureKey(f) {
			return errors.New("invalid provider failure")
		}
		if f.FileOrdinal != nil {
			if *f.FileOrdinal < 0 || *f.FileOrdinal >= len(p.Files) || !fileFailureMatches(f, fileDisposition[*f.FileOrdinal]) {
				return errors.New("provider failure reconciliation")
			}
		}
		if f.SymbolOrdinal != nil {
			if *f.SymbolOrdinal < 0 || *f.SymbolOrdinal >= len(p.Symbols) || !symbolFailureMatches(f, p.Symbols[*f.SymbolOrdinal].Disposition) {
				return errors.New("provider failure reconciliation")
			}
		}
	}
	q := p
	q.Work = 0
	base, _ := json.Marshal(q)
	work, ok := add(uint64(len(p.Files)+len(p.Symbols)), uint64(identityBytes))
	if !ok {
		return errors.New("work overflow")
	}
	work, ok = add(work, uint64(len(base)))
	if !ok {
		return errors.New("work overflow")
	}
	work, ok = add(work, uint64(len(raw)-len(base)))
	if !ok || work != p.Work || work > MaxWork {
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
		return disposition == "unreadable"
	case "DOCUMENT_SYMBOL":
		return disposition == "unsupported" || disposition == "document-symbol-failed"
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
		return f.Code == "UNAVAILABLE" && disposition == "prepare-missing" || (f.Code == "REQUEST_FAILED" || f.Code == "CANCELED") && disposition == "preparation-failed" || f.Code == "RESULT_MISMATCH" && disposition == "incomplete"
	default:
		return false
	}
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
func Publish(root *publication.Root, p Projection) (Receipt, error) {
	raw, e := EncodeCanonical(p)
	if e != nil {
		return Receipt{}, e
	}
	selector := Selector(raw)
	r, e := publication.PublishBoundFile(root, selector, raw, func(got []byte) error {
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
	}
	b, e := json.Marshal(r)
	return append(b, '\n'), e
}
