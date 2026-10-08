package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const oldBase = "docs/pilot/adr0007/source-text-search-v4"
const newBase = "docs/pilot/adr0007/source-text-search-v4-successor-freeze"
const manifestPath = newBase + "/SUCCESSOR_MANIFEST.json"
const censusPath = newBase + "/SUCCESSOR_TOOLING_CENSUS.json"
const envelopePath = newBase + "/SUCCESSOR_FREEZE.json"

const expectedSchemaVersion = "lsp-trace.adr0007.source-text-search.successor-freeze-manifest.private.v1"
const expectedEnvelopeSchemaVersion = "lsp-trace.adr0007.source-text-search.successor-freeze-envelope.private.v1"
const expectedCensusSchemaVersion = "lsp-trace.adr0007.source-text-search.successor-tooling-census.private.v1"
const expectedStage = "successor_freeze_candidate"
const expectedStatus = "SOURCE_TEXT_SEARCH_V4_SUCCESSOR_FREEZE_CANDIDATE"
const expectedHead = "43f0c6149bd6"
const predecessorFreezeRoot = "sha256:01df412bc5cce1e0adb824560fd7e5d2ff7f6c2621c2176062b4a170d763d367"
const predecessorFreezeDigest = "sha256:197b814ab6796ed35c9f59bfc70e6105ba2048db4556ae4ab6105151fe45bf2e"

var roots = []string{
	oldBase + "/",
	newBase + "/reviews/",
	"internal/adr0007sourcetextsearchv4private/",
	"internal/adr0007sourcetextsearchv4oracle/",
	"internal/adr0007v4contractvalidator/",
	"cmd/adr0007-source-text-search-v4-contract-validate/",
	"cmd/adr0007-source-text-search-v4-private-check/",
	"cmd/adr0007-source-text-search-v4-private-corpusgen/",
	"cmd/adr0007-source-text-search-v4-private-evaluate/",
	"cmd/adr0007-source-text-search-v4-private-mutations/",
	"cmd/adr0007-source-text-search-v4-private-oracle/",
	"cmd/adr0007-source-text-search-v4-private-oracle-check/",
	"cmd/adr0007-source-text-search-v4-private-oracle-freeze/",
	"cmd/adr0007-source-text-search-v4-private-prefreeze/",
	"cmd/adr0007-source-text-search-v4-successor-freeze/",
	"docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2/",
}

var exactFiles = []string{"go.mod", "go.sum"}

type Member struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	SchemaVersion           string   `json:"schema_version"`
	Stage                   string   `json:"stage"`
	SourceHead              string   `json:"source_head"`
	PredecessorFreezeRoot   string   `json:"predecessor_freeze_root_sha256"`
	PredecessorFreezeDigest string   `json:"predecessor_freeze_json_sha256"`
	AllowedRoots            []string `json:"allowed_roots"`
	AllowedExactFiles       []string `json:"allowed_exact_files"`
	ExcludedSelf            []string `json:"excluded_self"`
	NormativeMemberCount    int      `json:"normative_member_count"`
	NormativeBytes          int64    `json:"normative_bytes"`
	Members                 []Member `json:"members"`
	OuterRootSHA256         string   `json:"outer_root_sha256"`
}
type Envelope struct {
	SchemaVersion                    string    `json:"schema_version"`
	Status                           string    `json:"status"`
	Stage                            string    `json:"stage"`
	SuccessorRootSHA256              string    `json:"successor_root_sha256"`
	SuccessorManifest                string    `json:"successor_manifest"`
	SuccessorManifestSHA256          string    `json:"successor_manifest_sha256"`
	SuccessorCensus                  string    `json:"successor_census"`
	SuccessorCensusSHA256            string    `json:"successor_census_sha256"`
	PredecessorFreezeRoot            string    `json:"predecessor_freeze_root_sha256"`
	PredecessorFreezeDigest          string    `json:"predecessor_freeze_json_sha256"`
	OracleIndependenceVerdict        string    `json:"oracle_independence_exact_verdict"`
	OracleIndependenceArtifact       string    `json:"oracle_independence_artifact"`
	OracleIndependenceArtifactSHA256 string    `json:"oracle_independence_artifact_sha256"`
	RawDeriveVerdict                 string    `json:"raw_derive_exact_verdict"`
	RawDeriveArtifact                string    `json:"raw_derive_artifact"`
	RawDeriveArtifactSHA256          string    `json:"raw_derive_artifact_sha256"`
	Authority                        Authority `json:"authority"`
}

type Authority struct {
	AuthorityLevel           int    `json:"authority_level"`
	Accepted                 bool   `json:"accepted"`
	DesignGOSelfIssued       bool   `json:"design_go_self_issued"`
	QualificationExecuted    bool   `json:"qualification_executed"`
	FinalReviseVerdictCommit string `json:"final_revise_verdict_commit"`
}
type Census struct {
	SchemaVersion       string   `json:"schema_version"`
	Tool                string   `json:"tool"`
	Commands            []string `json:"commands"`
	SemanticBindings    []string `json:"semantic_bindings"`
	MutationsAuthorized []string `json:"mutations_authorized"`
}

func main() {
	mode := flag.String("mode", "verify", "write|verify|dryrun")
	flag.Parse()
	m, err := buildManifest()
	if err != nil {
		die(err)
	}
	c := buildCensus()
	e, err := buildEnvelope(m, c)
	if err != nil {
		die(err)
	}
	switch *mode {
	case "write":
		if err := writeJSON(manifestPath, m); err != nil {
			die(err)
		}
		if err := writeJSON(censusPath, c); err != nil {
			die(err)
		}
		e, _ = buildEnvelope(m, c)
		if err := writeJSON(envelopePath, e); err != nil {
			die(err)
		}
		fmt.Printf("wrote successor root=%s members=%d bytes=%d\n", m.OuterRootSHA256, m.NormativeMemberCount, m.NormativeBytes)
	case "dryrun":
		b, _ := json.MarshalIndent(struct {
			Manifest Manifest `json:"manifest"`
			Census   Census   `json:"census"`
			Envelope Envelope `json:"envelope"`
		}{m, c, e}, "", "  ")
		fmt.Println(string(b))
	case "verify":
		if err := verify(m, c, e); err != nil {
			die(err)
		}
		fmt.Printf("SUCCESSOR FREEZE OK root=%s manifest=%s census=%s envelope=%s members=%d bytes=%d\n", m.OuterRootSHA256, shaFile(manifestPath), shaFile(censusPath), shaFile(envelopePath), m.NormativeMemberCount, m.NormativeBytes)
	default:
		die(fmt.Errorf("unknown mode %q", *mode))
	}
}
func buildManifest() (Manifest, error) {
	m := Manifest{SchemaVersion: expectedSchemaVersion, Stage: expectedStage, SourceHead: expectedHead, PredecessorFreezeRoot: predecessorFreezeRoot, PredecessorFreezeDigest: predecessorFreezeDigest, AllowedRoots: append([]string{}, roots...), AllowedExactFiles: append([]string{}, exactFiles...), ExcludedSelf: []string{manifestPath, censusPath, envelopePath}}
	paths, err := collectPaths()
	if err != nil {
		return m, err
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return m, err
		}
		info, err := os.Lstat(p)
		if err != nil {
			return m, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return m, fmt.Errorf("symlink included: %s", p)
		}
		m.Members = append(m.Members, Member{p, roleFor(p), int64(len(b)), sha(b)})
		m.NormativeBytes += int64(len(b))
	}
	m.NormativeMemberCount = len(m.Members)
	m.OuterRootSHA256 = rootDigest(m.Members)
	return m, nil
}
func buildCensus() Census {
	return Census{SchemaVersion: expectedCensusSchemaVersion, Tool: "cmd/adr0007-source-text-search-v4-successor-freeze", Commands: []string{"go run ./cmd/adr0007-source-text-search-v4-successor-freeze -mode write", "go run ./cmd/adr0007-source-text-search-v4-successor-freeze -mode verify"}, SemanticBindings: []string{"old v4 docs/contracts/cases/oracle bytes", "v4 production package", "v4 oracle package", "v4 contract validator package", "pinned sourceadmissionv2 bytes"}, MutationsAuthorized: []string{"additive successor namespace files", "additive successor verifier command"}}
}
func buildEnvelope(m Manifest, c Census) (Envelope, error) {
	oi := newBase + "/reviews/SOURCE_TEXT_SEARCH_V4_ORACLE_INDEPENDENCE_GO.agent21eab7ae.json"
	rd := newBase + "/reviews/RAW_DERIVE_GO.agent196900df.json"
	return Envelope{SchemaVersion: expectedEnvelopeSchemaVersion, Status: expectedStatus, Stage: expectedStage, SuccessorRootSHA256: m.OuterRootSHA256, SuccessorManifest: manifestPath, SuccessorManifestSHA256: shaFile(manifestPath), SuccessorCensus: censusPath, SuccessorCensusSHA256: shaFile(censusPath), PredecessorFreezeRoot: predecessorFreezeRoot, PredecessorFreezeDigest: predecessorFreezeDigest, OracleIndependenceVerdict: "SOURCE_TEXT_SEARCH_V4_ORACLE_INDEPENDENCE_GO", OracleIndependenceArtifact: oi, OracleIndependenceArtifactSHA256: shaFile(oi), RawDeriveVerdict: "RAW_DERIVE_GO", RawDeriveArtifact: rd, RawDeriveArtifactSHA256: shaFile(rd), Authority: Authority{AuthorityLevel: 0, Accepted: false, DesignGOSelfIssued: false, QualificationExecuted: false, FinalReviseVerdictCommit: "b9b4900e"}}, nil
}
func collectPaths() ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) error {
		clean := filepath.ToSlash(filepath.Clean(p))
		if clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
			return fmt.Errorf("path escape: %s", p)
		}
		if clean == manifestPath || clean == censusPath || clean == envelopePath {
			return nil
		}
		if !allowed(clean) {
			return fmt.Errorf("extra path outside allowed roots: %s", clean)
		}
		if seen[clean] {
			return fmt.Errorf("duplicate path: %s", clean)
		}
		seen[clean] = true
		out = append(out, clean)
		return nil
	}
	for _, r := range roots {
		if err := filepath.WalkDir(strings.TrimSuffix(r, "/"), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink under allowed root: %s", filepath.ToSlash(p))
			}
			if d.IsDir() {
				return nil
			}
			return add(filepath.ToSlash(p))
		}); err != nil {
			return nil, err
		}
	}
	for _, p := range exactFiles {
		if err := add(p); err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}
func allowed(p string) bool {
	for _, r := range roots {
		if strings.HasPrefix(p, r) {
			return true
		}
	}
	for _, f := range exactFiles {
		if p == f {
			return true
		}
	}
	return false
}
func roleFor(p string) string {
	switch {
	case strings.HasPrefix(p, newBase+"/reviews/"):
		return "successor_review_evidence"
	case strings.HasPrefix(p, "cmd/adr0007-source-text-search-v4-successor-freeze/"):
		return "successor_verifier"
	case strings.HasPrefix(p, oldBase+"/contracts/"):
		return "contract"
	case strings.HasPrefix(p, oldBase+"/reviews/"):
		return "predecessor_review_evidence"
	case strings.HasPrefix(p, oldBase+"/cases/"):
		return "case_raw_source"
	case strings.HasPrefix(p, oldBase+"/oracle/") && strings.HasSuffix(p, ".expected.json"):
		return "oracle_expected"
	case strings.HasPrefix(p, oldBase+"/oracle/") && strings.HasSuffix(p, ".validator-bundle.json"):
		return "oracle_bundle"
	case strings.HasPrefix(p, oldBase+"/oracle/"):
		return "oracle_provenance"
	case strings.HasPrefix(p, "internal/adr0007sourcetextsearchv4private/"):
		return "production_package"
	case strings.HasPrefix(p, "internal/adr0007sourcetextsearchv4oracle/"):
		return "oracle_package"
	case strings.HasPrefix(p, "internal/adr0007v4contractvalidator/"):
		return "contract_validator_package"
	case strings.HasPrefix(p, "cmd/adr0007-source-text-search-v4-"):
		return "v4_command"
	case strings.HasPrefix(p, "docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2/"):
		return "pinned_sourceadmission_bytes"
	case p == "go.mod" || p == "go.sum":
		return "go_dependency_lock"
	default:
		return "v4_documentation"
	}
}
func verify(m Manifest, c Census, e Envelope) error {
	var gm Manifest
	if err := readStrict(manifestPath, &gm); err != nil {
		return err
	}
	if !reflect.DeepEqual(gm, m) {
		return errors.New("successor manifest mismatch")
	}
	var gc Census
	if err := readStrict(censusPath, &gc); err != nil {
		return err
	}
	if !reflect.DeepEqual(gc, c) {
		return errors.New("successor census mismatch")
	}
	var ge Envelope
	if err := readStrict(envelopePath, &ge); err != nil {
		return err
	}
	wantE, _ := buildEnvelope(m, c)
	if !reflect.DeepEqual(ge, wantE) {
		return errors.New("successor envelope mismatch")
	}
	if shaFile(oldBase+"/FREEZE.json") != predecessorFreezeDigest {
		return errors.New("predecessor FREEZE digest mismatch")
	}
	if err := checkReviewArtifacts(); err != nil {
		return err
	}
	if err := ensureRole(m, "successor_review_evidence", 2); err != nil {
		return err
	}
	if err := ensureRole(m, "successor_verifier", 1); err != nil {
		return err
	}
	if err := ensureRole(m, "production_package", 1); err != nil {
		return err
	}
	if err := ensureRole(m, "oracle_expected", 50); err != nil {
		return err
	}
	if err := ensureRole(m, "oracle_bundle", 50); err != nil {
		return err
	}
	if err := checkCases(); err != nil {
		return err
	}
	return compareTriple()
}
func checkReviewArtifacts() error {
	var oi struct {
		SchemaVersion  string         `json:"schema_version"`
		Kind           string         `json:"kind"`
		ExactVerdict   string         `json:"exact_verdict"`
		ReviewerAgent  string         `json:"reviewer_agent"`
		ReviewedCommit string         `json:"reviewed_commit"`
		Scope          map[string]any `json:"scope"`
		OverlapStats   map[string]any `json:"overlap_stats"`
		Limitations    []string       `json:"limitations"`
	}
	if err := readStrict(newBase+"/reviews/SOURCE_TEXT_SEARCH_V4_ORACLE_INDEPENDENCE_GO.agent21eab7ae.json", &oi); err != nil {
		return err
	}
	if oi.ExactVerdict != "SOURCE_TEXT_SEARCH_V4_ORACLE_INDEPENDENCE_GO" || oi.ReviewerAgent != "agent21eab7ae-95cf-4fc" || oi.ReviewedCommit != "0660860f" {
		return errors.New("oracle independence artifact identity/verdict mismatch")
	}
	if fmt.Sprint(oi.OverlapStats["complete_cases"]) != "21" || fmt.Sprint(oi.OverlapStats["failed_cases"]) != "29" {
		return errors.New("oracle independence overlap stats mismatch")
	}
	var rd struct {
		SchemaVersion  string         `json:"schema_version"`
		Kind           string         `json:"kind"`
		ExactVerdict   string         `json:"exact_verdict"`
		ReviewerAgent  string         `json:"reviewer_agent"`
		ContractCommit string         `json:"contract_commit"`
		Scope          map[string]any `json:"scope"`
	}
	if err := readStrict(newBase+"/reviews/RAW_DERIVE_GO.agent196900df.json", &rd); err != nil {
		return err
	}
	if rd.ExactVerdict != "RAW_DERIVE_GO" || rd.ReviewerAgent != "agent196900df" || rd.ContractCommit != "471346f1" {
		return errors.New("raw derive artifact identity/verdict mismatch")
	}
	return nil
}
func checkCases() error {
	b, err := os.ReadFile(oldBase + "/cases/CASE_MATRIX.json")
	if err != nil {
		return err
	}
	var rows []struct {
		ID              string `json:"id"`
		ExpectedOutcome string `json:"expected_outcome"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return err
	}
	if len(rows) != 50 {
		return fmt.Errorf("case count=%d want 50", len(rows))
	}
	complete, failed := 0, 0
	for _, r := range rows {
		if r.ExpectedOutcome == "COMPLETE" {
			complete++
		} else if r.ExpectedOutcome == "FAILED" {
			failed++
		}
		if _, err := os.Stat(oldBase + "/oracle/" + r.ID + ".expected.json"); err != nil {
			return err
		}
		if _, err := os.Stat(oldBase + "/oracle/" + r.ID + ".validator-bundle.json"); err != nil {
			return err
		}
	}
	if complete != 21 || failed != 29 {
		return fmt.Errorf("case split complete/failed=%d/%d want 21/29", complete, failed)
	}
	return nil
}
func compareTriple() error {
	b, err := os.ReadFile(oldBase + "/cases/CASE_MATRIX.json")
	if err != nil {
		return err
	}
	var rows []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return err
	}
	for _, r := range rows {
		attempt := oldBase + "/cases/" + r.ID + "/attempt.json"
		prod := run("go", "run", "./cmd/adr0007-source-text-search-v4-private-evaluate", attempt)
		exp, err := os.ReadFile(oldBase + "/oracle/" + r.ID + ".expected.json")
		if err != nil {
			return err
		}
		if !bytes.Equal(bytes.TrimSpace(prod), bytes.TrimSpace(exp)) {
			return fmt.Errorf("production != oracle %s", r.ID)
		}
		der := run("go", "run", "./cmd/adr0007-source-text-search-v4-contract-validate", oldBase+"/oracle/"+r.ID+".validator-bundle.json", "--derive")
		if !bytes.Equal(bytes.TrimSpace(der), bytes.TrimSpace(exp)) {
			return fmt.Errorf("derive != oracle %s", r.ID)
		}
	}
	return nil
}
func ensureRole(m Manifest, role string, min int) error {
	n := 0
	for _, x := range m.Members {
		if x.Role == role {
			n++
		}
	}
	if n < min {
		return fmt.Errorf("role %s count %d < %d", role, n, min)
	}
	return nil
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0644)
}
func readStrict(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := rejectDuplicateKeys(b); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing json data")
	}
	return nil
}
func rejectDuplicateKeys(b []byte) error {
	var walk func(json.RawMessage) error
	walk = func(raw json.RawMessage) error {
		dec := json.NewDecoder(bytes.NewReader(raw))
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch d := tok.(type) {
		case json.Delim:
			if d == '{' {
				seen := map[string]bool{}
				for dec.More() {
					kt, err := dec.Token()
					if err != nil {
						return err
					}
					k := kt.(string)
					if seen[k] {
						return fmt.Errorf("duplicate key %q", k)
					}
					seen[k] = true
					var child json.RawMessage
					if err := dec.Decode(&child); err != nil {
						return err
					}
					if err := walk(child); err != nil {
						return err
					}
				}
				_, err = dec.Token()
				return err
			}
			if d == '[' {
				for dec.More() {
					var child json.RawMessage
					if err := dec.Decode(&child); err != nil {
						return err
					}
					if err := walk(child); err != nil {
						return err
					}
				}
				_, err = dec.Token()
				return err
			}
		}
		return nil
	}
	var top json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&top); err != nil {
		return err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing json data")
	}
	return walk(top)
}
func rootDigest(ms []Member) string {
	h := sha256.New()
	for _, m := range ms {
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00%s\n", m.Path, m.Role, m.Bytes, m.SHA256)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func sha(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func shaFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "sha256:missing:" + path
	}
	return sha(b)
}
func run(name string, args ...string) []byte {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		die(fmt.Errorf("%s %v failed: %w\n%s", name, args, err, out))
	}
	return out
}
func die(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
