// Package captureset defines the private, transport-neutral manifest contract
// for a deterministic set of independently-custodied Graph Provenance V5 captures.
package captureset

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/strictjson"
)

const Version = "lsp-trace.capture-set.v1"
const MaxTargets = 10000
const MaxResources = 10000
const MaxBatchTargets = 63
const NativeV5SchemaID = "lsp-trace.graph-provenance.v5"
const identityDomain = "lsp-trace:capture-set:logical-identity:v1"

//go:embed schema.json
var schemaJSON []byte
var schemaOnce sync.Once
var compiledSchema *jsonschema.Schema
var schemaErr error

type LedgerEntry struct {
	Ordinal     int    `json:"ordinal"`
	Identity    string `json:"identity"`
	Disposition string `json:"disposition"`
}
type Ledger struct {
	Denominator int           `json:"denominator"`
	Entries     []LedgerEntry `json:"entries"`
}
type Target struct {
	CensusOrdinal         int    `json:"census_ordinal"`
	CanonicalSeedV2       string `json:"canonical_seed_v2"`
	CanonicalSeedV2SHA256 string `json:"canonical_seed_v2_sha256"`
}
type Constituent struct {
	ImmutableSelector string `json:"immutable_selector"`
	SchemaID          string `json:"schema_id"`
	SHA256            string `json:"sha256"`
	ByteLength        int    `json:"byte_length"`
	NativeV5Identity  string `json:"native_v5_identity"`
}
type Batch struct {
	Ordinal          int `json:"ordinal"`
	TargetStart      int `json:"target_start"`
	TargetCount      int `json:"target_count"`
	ConstituentIndex int `json:"constituent_index"`
}
type Manifest struct {
	SchemaVersion              string        `json:"schema_version"`
	LogicalDigest              string        `json:"logical_digest"`
	ImmutableSelector          string        `json:"immutable_selector"`
	Disclosure                 string        `json:"disclosure"`
	Authority                  int           `json:"authority"`
	SourceGraphComplete        string        `json:"source_graph_complete"`
	NativeSingleCaptureCustody bool          `json:"native_single_capture_custody"`
	CrossCaptureCalls          []string      `json:"cross_capture_calls"`
	LeidenAdmissible           bool          `json:"leiden_admissible"`
	CensusPolicy               string        `json:"census_policy"`
	DuplicatePolicy            string        `json:"duplicate_policy"`
	FileLedger                 Ledger        `json:"file_ledger"`
	SymbolLedger               Ledger        `json:"symbol_ledger"`
	Targets                    []Target      `json:"targets"`
	Constituents               []Constituent `json:"constituents"`
	Batches                    []Batch       `json:"batches"`
}

func SchemaJSON() []byte { return append([]byte(nil), schemaJSON...) }

func ValidateSchema(raw []byte) error {
	schemaOnce.Do(func() {
		var resource any
		resource, schemaErr = jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
		if schemaErr != nil {
			return
		}
		c := jsonschema.NewCompiler()
		if schemaErr = c.AddResource("schema.json", resource); schemaErr != nil {
			return
		}
		compiledSchema, schemaErr = c.Compile("schema.json")
	})
	if schemaErr != nil {
		return schemaErr
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON value required")
	}
	return compiledSchema.Validate(value)
}

func Prepare(targets []Target, constituents []Constituent, files, symbols Ledger, censusPolicy, duplicatePolicy string) (Manifest, error) {
	if len(targets) < 1 || len(targets) > MaxTargets {
		return Manifest{}, fmt.Errorf("target count outside [1,%d]", MaxTargets)
	}
	ordered := append([]Target(nil), targets...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CanonicalSeedV2 != ordered[j].CanonicalSeedV2 {
			return ordered[i].CanonicalSeedV2 < ordered[j].CanonicalSeedV2
		}
		return ordered[i].CensusOrdinal < ordered[j].CensusOrdinal
	})
	cs := append([]Constituent(nil), constituents...)
	sort.Slice(cs, func(i, j int) bool { return cs[i].ImmutableSelector < cs[j].ImmutableSelector })
	n := (len(ordered) + MaxBatchTargets - 1) / MaxBatchTargets
	if len(cs) != n {
		return Manifest{}, fmt.Errorf("constituent count %d does not match batch count %d", len(cs), n)
	}
	batches := make([]Batch, n)
	for i := range batches {
		count := len(ordered) - i*MaxBatchTargets
		if count > MaxBatchTargets {
			count = MaxBatchTargets
		}
		batches[i] = Batch{Ordinal: i, TargetStart: i * MaxBatchTargets, TargetCount: count, ConstituentIndex: i}
	}
	m := Manifest{SchemaVersion: Version, Disclosure: "PRIVATE", SourceGraphComplete: "UNKNOWN", CrossCaptureCalls: []string{}, CensusPolicy: censusPolicy, DuplicatePolicy: duplicatePolicy, FileLedger: cloneLedger(files), SymbolLedger: cloneLedger(symbols), Targets: ordered, Constituents: cs, Batches: batches}
	if err := validateContent(m, false); err != nil {
		return Manifest{}, err
	}
	setIdentity(&m)
	return m, nil
}

func EncodeCanonical(m Manifest) ([]byte, error) {
	if m.LogicalDigest == "" && m.ImmutableSelector == "" {
		setIdentity(&m)
	}
	if err := validateContent(m, true); err != nil {
		return nil, err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func Decode(raw []byte) (Manifest, error) {
	var m Manifest
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return m, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return m, errors.New("one JSON object required")
	}
	if err := validateContent(m, true); err != nil {
		return Manifest{}, err
	}
	canonical, _ := EncodeCanonical(m)
	if !bytes.Equal(raw, canonical) {
		return Manifest{}, errors.New("manifest is not canonically encoded")
	}
	return m, nil
}

func validateContent(m Manifest, identity bool) error {
	if m.SchemaVersion != Version {
		return errors.New("foreign schema_version")
	}
	if m.Disclosure != "PRIVATE" && m.Disclosure != "REDACTED" {
		return errors.New("disclosure must be PRIVATE or REDACTED")
	}
	if m.Authority != 0 || m.SourceGraphComplete != "UNKNOWN" || m.NativeSingleCaptureCustody || m.LeidenAdmissible || len(m.CrossCaptureCalls) != 0 {
		return errors.New("capture-set authority invariant violated")
	}
	if strings.TrimSpace(m.CensusPolicy) == "" || strings.TrimSpace(m.DuplicatePolicy) == "" {
		return errors.New("policies are required")
	}
	if err := validateLedger("file", m.FileLedger); err != nil {
		return err
	}
	if err := validateLedger("symbol", m.SymbolLedger); err != nil {
		return err
	}
	if m.FileLedger.Denominator > MaxResources {
		return errors.New("file resource bound exceeded")
	}
	if len(m.Targets) < 1 || len(m.Targets) > MaxTargets {
		return errors.New("target bound exceeded")
	}
	seenTarget := map[string]bool{}
	for i, t := range m.Targets {
		if t.CensusOrdinal < 0 || t.CanonicalSeedV2 == "" || t.CanonicalSeedV2SHA256 != rawDigest([]byte(t.CanonicalSeedV2)) {
			return fmt.Errorf("target %d mutation", i)
		}
		key := t.CanonicalSeedV2 + "\x00" + fmt.Sprint(t.CensusOrdinal)
		if seenTarget[key] {
			return errors.New("exact duplicate target")
		}
		seenTarget[key] = true
		if i > 0 && (m.Targets[i-1].CanonicalSeedV2 > t.CanonicalSeedV2 || (m.Targets[i-1].CanonicalSeedV2 == t.CanonicalSeedV2 && m.Targets[i-1].CensusOrdinal > t.CensusOrdinal)) {
			return errors.New("target canonical order mismatch")
		}
	}
	expected := (len(m.Targets) + 62) / 63
	if len(m.Constituents) != expected || len(m.Batches) != expected {
		return errors.New("missing constituent or batch")
	}
	seenC := map[string]bool{}
	for i, c := range m.Constituents {
		if c.SchemaID != NativeV5SchemaID {
			return errors.New("foreign constituent")
		}
		if c.ImmutableSelector == "" || c.NativeV5Identity == "" || c.ByteLength < 1 || !validDigest(c.SHA256) {
			return errors.New("invalid constituent identity")
		}
		if m.Disclosure == "PRIVATE" && c.ImmutableSelector != ConstituentSelectorPrefix+strings.TrimPrefix(c.SHA256, "sha256:") {
			return errors.New("constituent selector was not assigned from exact bytes")
		}
		if m.Disclosure == "REDACTED" && !strings.HasPrefix(c.ImmutableSelector, RedactedValue+":") {
			return errors.New("invalid redacted constituent selector")
		}
		if seenC[c.ImmutableSelector] {
			return errors.New("duplicate constituent")
		}
		seenC[c.ImmutableSelector] = true
		if i > 0 && m.Constituents[i-1].ImmutableSelector >= c.ImmutableSelector {
			return errors.New("reordered constituent")
		}
	}
	pos := 0
	for i, b := range m.Batches {
		want := len(m.Targets) - pos
		if want > 63 {
			want = 63
		}
		if b.Ordinal != i || b.TargetStart != pos || b.TargetCount != want || b.ConstituentIndex != i {
			return errors.New("non-consecutive or non-maximal batch assignment")
		}
		pos += b.TargetCount
	}
	if pos != len(m.Targets) {
		return errors.New("target accounting mismatch")
	}
	if identity {
		d, s := m.LogicalDigest, m.ImmutableSelector
		m.LogicalDigest = ""
		m.ImmutableSelector = ""
		setIdentity(&m)
		if d != m.LogicalDigest || s != m.ImmutableSelector {
			return errors.New("logical identity mutation")
		}
	}
	return nil
}
func validateLedger(name string, l Ledger) error {
	if l.Denominator < 0 || l.Denominator > MaxResources || len(l.Entries) != l.Denominator {
		return fmt.Errorf("%s ledger is not closed", name)
	}
	seen := map[int]bool{}
	for _, e := range l.Entries {
		if e.Ordinal < 0 || e.Ordinal >= l.Denominator || seen[e.Ordinal] || e.Identity == "" || e.Disposition == "" {
			return fmt.Errorf("%s ledger invalid", name)
		}
		seen[e.Ordinal] = true
	}
	return nil
}
func setIdentity(m *Manifest) {
	m.LogicalDigest = ""
	m.ImmutableSelector = ""
	b, _ := json.Marshal(m)
	m.LogicalDigest = digest(identityDomain, b)
	m.ImmutableSelector = "capture-set:" + strings.TrimPrefix(m.LogicalDigest, "sha256:")
}
func cloneLedger(l Ledger) Ledger { l.Entries = append([]LedgerEntry(nil), l.Entries...); return l }
func rawDigest(b []byte) string   { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func digest(domain string, b []byte) string {
	return rawDigest(append(append([]byte(domain), 0), b...))
}
func validDigest(s string) bool {
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil
}
