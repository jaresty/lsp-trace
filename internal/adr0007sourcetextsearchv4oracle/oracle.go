package adr0007sourcetextsearchv4oracle

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	BaseDir               = "docs/pilot/adr0007/source-text-search-v4"
	CasesDir              = BaseDir + "/cases"
	OracleDir             = BaseDir + "/oracle"
	ProvPath              = OracleDir + "/ORACLE_PROVENANCE.json"
	TerminalSchema        = "lsp-trace.adr0007.source-text-search.terminal-result.oracle.v4"
	PinnedAdmissionSchema = "lsp-trace.adr0007.source-admission.private.v2"
)

type Terminal struct {
	SchemaVersion      string              `json:"schema_version"`
	AttemptID          string              `json:"attempt_id,omitempty"`
	Outcome            string              `json:"outcome"`
	Failure            *Failure            `json:"failure,omitempty"`
	Replay             Replay              `json:"replay"`
	Accounting         Accounting          `json:"accounting"`
	Admission          *Binding            `json:"admission,omitempty"`
	Matches            []Match             `json:"matches,omitempty"`
	LocationCandidates []LocationCandidate `json:"location_candidates,omitempty"`
}
type Failure struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}
type Replay struct {
	ToolingIdentitySHA256 string `json:"tooling_identity_sha256"`
	PredecessorLockSHA256 string `json:"predecessor_lock_sha256"`
	PayloadID             string `json:"payload_id"`
	TerminalDigest        string `json:"terminal_digest,omitempty"`
}
type Accounting struct {
	Files       int `json:"files"`
	SourceBytes int `json:"source_bytes"`
	TotalBytes  int `json:"total_bytes"`
	Matches     int `json:"matches"`
	Work        int `json:"work"`
	OutputBytes int `json:"output_bytes"`
}
type Match struct {
	Path        string `json:"path"`
	Revision    string `json:"revision"`
	ByteStart   int    `json:"byte_start"`
	ByteEnd     int    `json:"byte_end"`
	Line        int    `json:"line"`
	UTF16Column int    `json:"utf16_column"`
	Text        string `json:"text"`
}
type LocationCandidate struct {
	Pin   string `json:"pin"`
	Value any    `json:"value"`
}

type Attempt struct {
	SchemaVersion    string           `json:"schema_version"`
	AttemptID        string           `json:"attempt_id"`
	ExecutionControl ExecutionControl `json:"execution_control"`
	Request          Request          `json:"request"`
	SourceInputs     []SourceInput    `json:"source_inputs"`
}
type ExecutionControl struct {
	SchemaVersion string   `json:"schema_version"`
	Observations  []string `json:"observations"`
}
type Request struct {
	SchemaVersion string         `json:"schema_version"`
	Query         string         `json:"query"`
	Sources       []SourceRef    `json:"sources"`
	Limits        Limits         `json:"limits"`
	Policy        Policy         `json:"policy"`
	LocationPin   map[string]any `json:"location_pin"`
}
type Limits struct {
	SchemaVersion  string `json:"schema_version"`
	MaxFiles       int    `json:"max_files"`
	MaxMatches     int    `json:"max_matches"`
	MaxOutputBytes int    `json:"max_output_bytes"`
	MaxPathBytes   int    `json:"max_path_bytes"`
	MaxSourceBytes int    `json:"max_source_bytes"`
	MaxTotalBytes  int    `json:"max_total_bytes"`
	MaxWork        int    `json:"max_work"`
}
type Policy struct {
	SchemaVersion         string `json:"schema_version"`
	AllowBackendSemantics bool   `json:"allow_backend_semantics"`
	AllowFuzzy            bool   `json:"allow_fuzzy"`
	AllowModel            bool   `json:"allow_model"`
	AllowRank             bool   `json:"allow_rank"`
	AllowRegex            bool   `json:"allow_regex"`
	AllowToken            bool   `json:"allow_token"`
	LiteralMode           string `json:"literal_mode"`
}
type SourceRef struct {
	Path         string `json:"path"`
	Revision     string `json:"revision"`
	FileDigest   string `json:"file_digest"`
	ObjectDigest string `json:"object_digest"`
	Ordinal      int    `json:"ordinal"`
}
type SourceInput struct {
	SchemaVersion string `json:"schema_version"`
	Path          string `json:"path"`
	Revision      string `json:"revision"`
	FileDigest    string `json:"file_digest"`
	ObjectDigest  string `json:"object_digest"`
	BytesBase64   string `json:"bytes_base64"`
	Ordinal       int    `json:"ordinal"`
}

type SelectedSource struct {
	Path, Revision, FileDigest, ObjectDigest string
	Bytes                                    []byte
}
type Binding struct {
	Schema          string               `json:"schema"`
	AdmissionDigest string               `json:"admissionDigest"`
	Sources         []SelectedSourceJSON `json:"sources"`
}
type SelectedSourceJSON struct {
	Path         string `json:"path"`
	Revision     string `json:"revision"`
	FileDigest   string `json:"fileDigest"`
	ObjectDigest string `json:"objectDigest"`
	BytesBase64  string `json:"bytes_base64,omitempty"`
}

type Row struct {
	ID              string `json:"id"`
	Requirement     string `json:"requirement"`
	Stimulus        string `json:"stimulus"`
	Assertion       string `json:"assertion"`
	ExpectedOutcome string `json:"expected_outcome"`
	ExpectedCode    string `json:"expected_code"`
}

type Manifest struct {
	SchemaVersion         string `json:"schema_version"`
	Stage                 string `json:"stage"`
	ToolingDigest         string `json:"tooling_digest"`
	PredecessorLockDigest string `json:"predecessor_lock_digest"`
	Members               []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Bytes  int    `json:"bytes"`
	} `json:"members"`
	Excludes []string `json:"excludes"`
}

func SHA(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }

func payloadID(m Manifest) string {
	parts := []string{m.SchemaVersion, m.Stage, m.ToolingDigest, m.PredecessorLockDigest}
	for _, e := range m.Members {
		parts = append(parts, e.Path, e.SHA256, fmt.Sprint(e.Bytes))
	}
	return SHA([]byte(strings.Join(parts, "\x00")))
}

func EvaluateFile(path string) ([]byte, error) {
	m, err := ReadManifest()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tr := EvaluateBytes(raw, m)
	b, err := Canonical(tr)
	if err != nil {
		return nil, err
	}
	var with Terminal
	_ = json.Unmarshal(b, &with)
	with.Replay.TerminalDigest = SHA(b)
	return Canonical(with)
}

func EvaluateBytes(raw []byte, m Manifest) Terminal {
	base := Terminal{SchemaVersion: TerminalSchema, Outcome: "FAILED", Replay: Replay{ToolingIdentitySHA256: m.ToolingDigest, PredecessorLockSHA256: m.PredecessorLockDigest, PayloadID: payloadID(m)}}
	if err := rejectDuplicateKeys(raw); err != nil {
		return fail(base, "INVALID_INPUT", err.Error())
	}
	var a Attempt
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return fail(base, "INVALID_INPUT", err.Error())
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return fail(base, "INVALID_INPUT", "trailing JSON data")
	}
	base.AttemptID = a.AttemptID
	if err := validateAttempt(a); err != nil {
		return fail(base, "INVALID_INPUT", err.Error())
	}
	if term, stop := poll(a, base, "before_admission"); stop {
		return term
	}
	sel := make([]SelectedSource, 0, len(a.SourceInputs))
	byOrd := map[int]SourceInput{}
	for _, si := range a.SourceInputs {
		if _, ok := byOrd[si.Ordinal]; ok {
			return fail(base, "ASSOCIATION_FAILED", "duplicate source input ordinal")
		}
		byOrd[si.Ordinal] = si
	}
	for _, sr := range a.Request.Sources {
		si, ok := byOrd[sr.Ordinal]
		if !ok {
			return fail(base, "ASSOCIATION_FAILED", "missing source input ordinal")
		}
		if si.Path != sr.Path || si.Revision != sr.Revision {
			return fail(base, "ASSOCIATION_FAILED", "source tuple mismatch")
		}
		bb, err := base64.StdEncoding.DecodeString(si.BytesBase64)
		if err != nil {
			return fail(base, "INVALID_INPUT", "bad source bytes base64")
		}
		sel = append(sel, SelectedSource{Path: sr.Path, Revision: sr.Revision, FileDigest: sr.FileDigest, ObjectDigest: sr.ObjectDigest, Bytes: bb})
	}
	if len(byOrd) != len(a.Request.Sources) {
		return fail(base, "ASSOCIATION_FAILED", "extra source input ordinal")
	}
	adm := Admit(sel, a.Request.Limits)
	base.Accounting.Files = len(sel)
	for _, s := range sel {
		base.Accounting.SourceBytes += len(s.Bytes)
		base.Accounting.TotalBytes += len(s.Bytes)
		base.Accounting.Work += len(s.Bytes)
	}
	if adm.Outcome != "COMPLETE" {
		code := "ADMISSION_FAILED"
		if adm.Outcome == "RESOURCE_LIMIT" {
			code = "RESOURCE_EXHAUSTED"
		}
		return fail(base, code, adm.Detail)
	}
	base.Admission = adm.Binding
	if term, stop := poll(a, base, "after_admission"); stop {
		return term
	}
	q := []byte(a.Request.Query)
	matches := []Match{}
	for _, s := range adm.SourcesBytes {
		off := 0
		for {
			i := bytes.Index(s.Bytes[off:], q)
			if i < 0 {
				break
			}
			st := off + i
			en := st + len(q)
			matches = append(matches, makeMatch(s, st, en))
			base.Accounting.Work++
			if a.Request.Limits.MaxMatches > 0 && len(matches) > a.Request.Limits.MaxMatches {
				return fail(base, "RESOURCE_EXHAUSTED", "matches")
			}
			off = st + 1
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].ByteStart < matches[j].ByteStart
	})
	base.Matches = matches
	base.Accounting.Matches = len(matches)
	base.Outcome = "COMPLETE"
	base.LocationCandidates = locationCandidates(a.Request.LocationPin)
	b, _ := Canonical(base)
	base.Accounting.OutputBytes = len(b)
	if a.Request.Limits.MaxOutputBytes > 0 && base.Accounting.OutputBytes > a.Request.Limits.MaxOutputBytes {
		return fail(base, "RESOURCE_EXHAUSTED", "output")
	}
	return base
}

func validateAttempt(a Attempt) error {
	if a.SchemaVersion == "" || a.AttemptID == "" {
		return errors.New("missing attempt fields")
	}
	r := a.Request
	if r.Query == "" {
		return errors.New("empty query")
	}
	if !utf8.ValidString(r.Query) {
		return errors.New("query utf8")
	}
	if len(r.Sources) == 0 {
		return errors.New("empty source set")
	}
	l := r.Limits
	if l.MaxFiles < 1 || l.MaxMatches < 1 || l.MaxOutputBytes < 1 || l.MaxPathBytes < 1 || l.MaxSourceBytes < 1 || l.MaxTotalBytes < 1 || l.MaxWork < 1 {
		return errors.New("nonpositive limit")
	}
	if len(r.Sources) > l.MaxFiles {
		return fmt.Errorf("max files")
	}
	for _, s := range r.Sources {
		if len(s.Path) > l.MaxPathBytes {
			return fmt.Errorf("max path")
		}
		if s.Ordinal < 1 {
			return fmt.Errorf("bad ordinal")
		}
	}
	p := r.Policy
	if p.AllowBackendSemantics || p.AllowFuzzy || p.AllowModel || p.AllowRank || p.AllowRegex || p.AllowToken || p.LiteralMode != "byte-literal" {
		return errors.New("policy")
	}
	return nil
}
func poll(a Attempt, base Terminal, state string) (Terminal, bool) {
	for _, o := range a.ExecutionControl.Observations {
		if o == state+":cancel" || o == "cancel:"+state || o == "cancel" {
			return fail(base, "CANCELLED", state), true
		}
		if o == state+":deadline" || o == "deadline:"+state || o == "deadline" {
			return fail(base, "DEADLINE_EXCEEDED", state), true
		}
	}
	return base, false
}
func fail(t Terminal, code, detail string) Terminal {
	t.Outcome = "FAILED"
	t.Failure = &Failure{Code: code, Detail: detail}
	return t
}

type admitResult struct {
	Outcome      string
	Binding      *Binding
	Detail       string
	SourcesBytes []SelectedSource
}

func Admit(in []SelectedSource, l Limits) admitResult {
	if len(in) == 0 || l.MaxFiles < 1 || l.MaxSourceBytes < 1 || l.MaxTotalBytes < 1 {
		return admitResult{Outcome: "INVALID_REQUEST", Detail: "limits or sources"}
	}
	if len(in) > l.MaxFiles {
		return admitResult{Outcome: "RESOURCE_LIMIT", Detail: "sources"}
	}
	out := append([]SelectedSource(nil), in...)
	seen := map[string]bool{}
	total := 0
	for i, s := range out {
		if !CanonicalPath(s.Path) || s.Revision == "" || len(s.Bytes) == 0 || !utf8.Valid(s.Bytes) || len(s.Bytes) > l.MaxSourceBytes {
			return admitResult{Outcome: "INVALID_SOURCE", Detail: "binding"}
		}
		if seen[s.Path] {
			return admitResult{Outcome: "DUPLICATE_SOURCE", Detail: s.Path}
		}
		seen[s.Path] = true
		total += len(s.Bytes)
		if total > l.MaxTotalBytes {
			return admitResult{Outcome: "RESOURCE_LIMIT", Detail: "bytes"}
		}
		d := SHA(s.Bytes)
		if s.FileDigest != "" && s.FileDigest != d {
			return admitResult{Outcome: "INVALID_SOURCE", Detail: "file digest"}
		}
		if s.ObjectDigest != "" && s.ObjectDigest != d {
			return admitResult{Outcome: "INVALID_SOURCE", Detail: "object digest"}
		}
		out[i].FileDigest = d
		out[i].ObjectDigest = d
		out[i].Bytes = append([]byte(nil), s.Bytes...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	h := sha256.New()
	h.Write([]byte(PinnedAdmissionSchema))
	js := []SelectedSourceJSON{}
	for _, s := range out {
		for _, v := range []string{s.Path, s.Revision, s.FileDigest, s.ObjectDigest} {
			h.Write([]byte{0})
			h.Write([]byte(v))
		}
		js = append(js, SelectedSourceJSON{Path: s.Path, Revision: s.Revision, FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest})
	}
	return admitResult{Outcome: "COMPLETE", Binding: &Binding{Schema: PinnedAdmissionSchema, AdmissionDigest: "sha256:" + hex.EncodeToString(h.Sum(nil)), Sources: js}, SourcesBytes: out}
}
func CanonicalPath(p string) bool {
	return p != "" && utf8.ValidString(p) && norm.NFC.IsNormalString(p) && !strings.Contains(p, "\\") && !strings.Contains(p, "//") && !strings.Contains(p, ":") && !strings.HasPrefix(p, "/") && p != "." && p != ".." && !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") && !strings.Contains(p, "/./") && !strings.Contains(p, "/../") && path.Clean(p) == p
}
func makeMatch(s SelectedSource, st, en int) Match {
	line, col := LineUTF16(s.Bytes, st)
	return Match{Path: s.Path, Revision: s.Revision, ByteStart: st, ByteEnd: en, Line: line, UTF16Column: col, Text: string(s.Bytes[st:en])}
}
func LineUTF16(b []byte, off int) (int, int) {
	line, col := 0, 0
	i := 0
	for i < off {
		r, sz := utf8.DecodeRune(b[i:])
		if r == '\r' {
			line++
			col = 0
			if i+sz < off && b[i+sz] == '\n' {
				i += sz + 1
			} else {
				i += sz
			}
			continue
		}
		if r == '\n' {
			line++
			col = 0
			i += sz
			continue
		}
		col += len(utf16.Encode([]rune{r}))
		i += sz
	}
	return line, col
}
func locationCandidates(pin map[string]any) []LocationCandidate {
	keys := []string{"design_commit", "design_root_sha256", "execution_commit", "seal_commit", "final_seal_sha256", "source_admission_pin"}
	out := []LocationCandidate{}
	for _, k := range keys {
		if v, ok := pin[k]; ok {
			out = append(out, LocationCandidate{Pin: k, Value: v})
		}
	}
	return out
}
func Canonical(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
func ReadManifest() (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(BaseDir + "/PAYLOAD_MANIFEST.json")
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

func rejectDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	return scanValue(dec)
}

func scanValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			ktok, err := dec.Token()
			if err != nil {
				return err
			}
			k, ok := ktok.(string)
			if !ok {
				return fmt.Errorf("object key is not string")
			}
			if seen[k] {
				return fmt.Errorf("duplicate key %q", k)
			}
			seen[k] = true
			if err := scanValue(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	case '[':
		for dec.More() {
			if err := scanValue(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	default:
		return fmt.Errorf("unexpected delimiter %q", d)
	}
}

func ReadRows(root string) ([]Row, error) {
	b, err := os.ReadFile(filepath.Join(root, "CASE_MATRIX.json"))
	if err != nil {
		return nil, err
	}
	var r []Row
	err = json.Unmarshal(b, &r)
	return r, err
}
func ExpectedPath(id string) string { return filepath.Join(OracleDir, id+".expected.json") }
func Freeze(root string) error {
	rows, err := ReadRows(root)
	if err != nil {
		return err
	}
	m, err := ReadManifest()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(OracleDir, 0755); err != nil {
		return err
	}
	prov := []string{BaseDir + "/FREEZE_DESIGN.md", BaseDir + "/PAYLOAD_MANIFEST.json", filepath.Join(root, "CASE_MATRIX.json"), "docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2/admission.go"}
	for _, r := range rows {
		p := filepath.Join(root, r.ID, "attempt.json")
		b, err := EvaluateFile(p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(ExpectedPath(r.ID), b, 0644); err != nil {
			return err
		}
		prov = append(prov, p)
		src := filepath.Join(root, r.ID, "source", "a.txt")
		if _, err := os.Stat(src); err == nil {
			prov = append(prov, src)
		}
	}
	sort.Strings(prov)
	return writeProvenance(prov, m)
}
func writeProvenance(paths []string, m Manifest) error {
	type P struct {
		SchemaVersion  string   `json:"schema_version"`
		ForbiddenGlobs []string `json:"forbidden_globs"`
		PathsRead      []string `json:"paths_read"`
		PayloadID      string   `json:"payload_id"`
	}
	p := P{"lsp-trace.adr0007.source-text-search.oracle-provenance.v1", []string{"internal/adr0007sourcetextsearchv4private/**", "cmd/adr0007-source-text-search-v4-private-evaluate/**", "generated production terminal outputs"}, paths, payloadID(m)}
	b, _ := Canonical(p)
	return os.WriteFile(ProvPath, b, 0644)
}
func Check(root string) error {
	rows, err := ReadRows(root)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.ID] = true
		b, err := EvaluateFile(filepath.Join(root, r.ID, "attempt.json"))
		if err != nil {
			return err
		}
		exp, err := os.ReadFile(ExpectedPath(r.ID))
		if err != nil {
			return err
		}
		if !bytes.Equal(b, exp) {
			return fmt.Errorf("oracle mismatch %s", r.ID)
		}
	}
	ents, err := os.ReadDir(OracleDir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if e.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in oracle: %s", e.Name())
		}
		if strings.HasSuffix(e.Name(), ".expected.json") {
			id := strings.TrimSuffix(e.Name(), ".expected.json")
			if !seen[id] {
				return fmt.Errorf("extra expected %s", e.Name())
			}
		}
	}
	return CheckProvenance()
}
func CheckProvenance() error {
	b, err := os.ReadFile(ProvPath)
	if err != nil {
		return err
	}
	var p struct {
		PathsRead []string `json:"paths_read"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	for _, x := range p.PathsRead {
		if strings.HasPrefix(x, "internal/adr0007sourcetextsearchv4private/") || strings.HasPrefix(x, "cmd/adr0007-source-text-search-v4-private-evaluate/") || strings.Contains(x, "production terminal") {
			return fmt.Errorf("forbidden provenance path %s", x)
		}
	}
	return nil
}
