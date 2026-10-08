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

const base = "docs/pilot/adr0007/source-text-search-v4"
const manifestPath = base + "/PRE_FREEZE_MANIFEST.json"
const dryrunPath = base + "/FREEZE_ENVELOPE.dryrun.json"

const expectedSchemaVersion = "lsp-trace.adr0007.source-text-search.pre-freeze-manifest.private.v1"
const expectedStage = "pre_freeze"
const expectedDryrunSchemaVersion = "lsp-trace.adr0007.source-text-search.freeze-envelope.dryrun.v1"
const expectedDryrunStatus = "DRYRUN_ONLY_NOT_FINAL_FREEZE"

// Commit semantics are intentionally pinned instead of self-referential: the
// manifest and dry-run envelope are excluded from the outer root, while verifier
// source is included and changes the root. A final commit cannot contain its own
// commit hash as a verified JSON field without changing that commit hash again.
const expectedPayloadSourceCommit = "fe90c3dc271bb3cd728aefd9592b4fb9d0ae913d"
const expectedVerifierCommit = "fe90c3dc271bb3cd728aefd9592b4fb9d0ae913d"

type Member struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion        string   `json:"schema_version"`
	Stage                string   `json:"stage"`
	PayloadSourceCommit  string   `json:"payload_source_commit"`
	VerifierCommit       string   `json:"verifier_commit"`
	AllowedRoots         []string `json:"allowed_roots"`
	AllowedExactFiles    []string `json:"allowed_exact_files"`
	ExcludedSelf         string   `json:"excluded_self"`
	FinalFreezeExcluded  string   `json:"final_freeze_excluded"`
	TerminalsEmbedRoot   bool     `json:"terminals_embed_outer_root"`
	NormativeMemberCount int      `json:"normative_member_count"`
	NormativeBytes       int64    `json:"normative_bytes"`
	Members              []Member `json:"members"`
	OuterRootSHA256      string   `json:"outer_root_sha256"`
}

type DryrunEnvelope struct {
	Bytes               int64  `json:"bytes"`
	Members             int    `json:"members"`
	PreFreezeRootSHA256 string `json:"pre_freeze_root_sha256"`
	SchemaVersion       string `json:"schema_version"`
	Status              string `json:"status"`
}

var roots = []string{
	"docs/pilot/adr0007/source-text-search-v4/",
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
	"docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2/",
}

var exactFiles = []string{"go.mod", "go.sum"}

func main() {
	mode := flag.String("mode", "verify", "verify|write|dryrun")
	flag.Parse()
	m, err := buildManifest()
	if err != nil {
		die(err)
	}
	switch *mode {
	case "write":
		if err := writeJSON(manifestPath, m); err != nil {
			die(err)
		}
		if err := writeJSON(dryrunPath, freezeDryrun(m)); err != nil {
			die(err)
		}
		fmt.Printf("wrote %s root=%s members=%d bytes=%d\n", manifestPath, m.OuterRootSHA256, m.NormativeMemberCount, m.NormativeBytes)
	case "dryrun":
		b, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(b))
	case "verify":
		if err := verify(m); err != nil {
			die(err)
		}
		fmt.Printf("PREFREEZE OK root=%s members=%d bytes=%d\n", m.OuterRootSHA256, m.NormativeMemberCount, m.NormativeBytes)
	default:
		die(fmt.Errorf("unknown mode %q", *mode))
	}
}

func buildManifest() (Manifest, error) {
	m := Manifest{SchemaVersion: expectedSchemaVersion, Stage: expectedStage, PayloadSourceCommit: expectedPayloadSourceCommit, VerifierCommit: expectedVerifierCommit, AllowedRoots: append([]string{}, roots...), AllowedExactFiles: append([]string{}, exactFiles...), ExcludedSelf: manifestPath, FinalFreezeExcluded: base + "/FREEZE.json", TerminalsEmbedRoot: false}
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
		role := roleFor(p)
		m.Members = append(m.Members, Member{Path: p, Role: role, Bytes: int64(len(b)), SHA256: sha(b)})
		m.NormativeBytes += int64(len(b))
	}
	m.NormativeMemberCount = len(m.Members)
	m.OuterRootSHA256 = rootDigest(m.Members)
	return m, nil
}

func collectPaths() ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) error {
		clean := filepath.ToSlash(filepath.Clean(p))
		if clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
			return fmt.Errorf("path escape: %s", p)
		}
		if clean == manifestPath || clean == dryrunPath || clean == base+"/FREEZE.json" {
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
	case strings.HasPrefix(p, base+"/contracts/"):
		return "contract"
	case strings.HasPrefix(p, base+"/schemas/"):
		return "schema"
	case strings.HasPrefix(p, base+"/reviews/"):
		return "review_verdict_evidence"
	case strings.HasPrefix(p, base+"/cases/"):
		return "case_raw_source"
	case strings.HasPrefix(p, base+"/oracle/") && strings.HasSuffix(p, ".expected.json"):
		return "oracle_expected"
	case strings.HasPrefix(p, base+"/oracle/") && strings.HasSuffix(p, ".validator-bundle.json"):
		return "oracle_bundle"
	case strings.HasPrefix(p, base+"/oracle/"):
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
	case strings.HasSuffix(p, "MANIFEST.json") || strings.HasSuffix(p, "CENSUS.json"):
		return "inner_manifest"
	default:
		return "v4_documentation"
	}
}

func verify(m Manifest) error {
	disk, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var got Manifest
	if err := strictUnmarshal(disk, &got); err != nil {
		return fmt.Errorf("manifest decode: %w", err)
	}
	if err := compareManifest(got, m); err != nil {
		return err
	}
	if err := ensureRole(m, "production_package", 1); err != nil {
		return err
	}
	if err := ensureRole(m, "oracle_package", 1); err != nil {
		return err
	}
	if err := ensureRole(m, "contract_validator_package", 1); err != nil {
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
	if err := compareDeriveOracle(); err != nil {
		return err
	}
	if err := checkDryrun(m); err != nil {
		return err
	}
	return nil
}

func compareManifest(got, want Manifest) error {
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"schema_version", got.SchemaVersion, expectedSchemaVersion},
		{"stage", got.Stage, expectedStage},
		{"payload_source_commit", got.PayloadSourceCommit, expectedPayloadSourceCommit},
		{"verifier_commit", got.VerifierCommit, expectedVerifierCommit},
		{"allowed_roots", got.AllowedRoots, roots},
		{"allowed_exact_files", got.AllowedExactFiles, exactFiles},
		{"excluded_self", got.ExcludedSelf, manifestPath},
		{"final_freeze_excluded", got.FinalFreezeExcluded, base + "/FREEZE.json"},
		{"terminals_embed_outer_root", got.TerminalsEmbedRoot, false},
		{"normative_member_count", got.NormativeMemberCount, want.NormativeMemberCount},
		{"normative_bytes", got.NormativeBytes, want.NormativeBytes},
		{"members", got.Members, want.Members},
		{"outer_root_sha256", got.OuterRootSHA256, want.OuterRootSHA256},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			return fmt.Errorf("manifest %s mismatch: got %v want %v", c.name, c.got, c.want)
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

func checkCases() error {
	b, err := os.ReadFile(base + "/cases/CASE_MATRIX.json")
	if err != nil {
		return err
	}
	var rows []struct {
		ID              string `json:"id"`
		ExpectedOutcome string `json:"expected_outcome"`
		ExpectedCode    string `json:"expected_code"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return err
	}
	if len(rows) != 50 {
		return fmt.Errorf("case matrix count=%d want 50", len(rows))
	}
	complete, failed := 0, 0
	for _, r := range rows {
		if r.ExpectedOutcome == "COMPLETE" {
			complete++
		} else if r.ExpectedOutcome == "FAILED" {
			failed++
		}
		if _, err := os.Stat(base + "/oracle/" + r.ID + ".expected.json"); err != nil {
			return err
		}
		if _, err := os.Stat(base + "/oracle/" + r.ID + ".validator-bundle.json"); err != nil {
			return err
		}
	}
	if complete != 21 || failed != 29 {
		return fmt.Errorf("case split complete/failed=%d/%d want 21/29", complete, failed)
	}
	return nil
}

func compareDeriveOracle() error {
	b, err := os.ReadFile(base + "/cases/CASE_MATRIX.json")
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
		attempt := base + "/cases/" + r.ID + "/attempt.json"
		prod := run("go", "run", "./cmd/adr0007-source-text-search-v4-private-evaluate", attempt)
		exp, err := os.ReadFile(base + "/oracle/" + r.ID + ".expected.json")
		if err != nil {
			return err
		}
		if !bytes.Equal(bytes.TrimSpace(prod), bytes.TrimSpace(exp)) {
			return fmt.Errorf("production != oracle %s", r.ID)
		}
		bun := base + "/oracle/" + r.ID + ".validator-bundle.json"
		der := run("go", "run", "./cmd/adr0007-source-text-search-v4-contract-validate", bun, "--derive")
		if !bytes.Equal(bytes.TrimSpace(der), bytes.TrimSpace(exp)) {
			return fmt.Errorf("derive != oracle %s", r.ID)
		}
	}
	return nil
}

func checkDryrun(m Manifest) error {
	b, err := os.ReadFile(dryrunPath)
	if err != nil {
		return err
	}
	var got DryrunEnvelope
	if err := strictUnmarshal(b, &got); err != nil {
		return fmt.Errorf("dryrun decode: %w", err)
	}
	want := freezeDryrun(m)
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"schema_version", got.SchemaVersion, expectedDryrunSchemaVersion},
		{"pre_freeze_root_sha256", got.PreFreezeRootSHA256, want.PreFreezeRootSHA256},
		{"members", got.Members, want.Members},
		{"bytes", got.Bytes, want.Bytes},
		{"status", got.Status, expectedDryrunStatus},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			return fmt.Errorf("dryrun %s mismatch: got %v want %v", c.name, c.got, c.want)
		}
	}
	return nil
}

func freezeDryrun(m Manifest) DryrunEnvelope {
	return DryrunEnvelope{SchemaVersion: expectedDryrunSchemaVersion, PreFreezeRootSHA256: m.OuterRootSHA256, Members: m.NormativeMemberCount, Bytes: m.NormativeBytes, Status: expectedDryrunStatus}
}

func strictUnmarshal(b []byte, v any) error {
	if err := rejectDuplicateKeys(b); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing data after JSON value")
		}
		return err
	}
	return nil
}

func rejectDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	var stack []map[string]struct{}
	var pendingKey []bool
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				stack = append(stack, map[string]struct{}{})
				pendingKey = append(pendingKey, true)
			case '}':
				stack = stack[:len(stack)-1]
				pendingKey = pendingKey[:len(pendingKey)-1]
				if len(pendingKey) > 0 && !pendingKey[len(pendingKey)-1] {
					pendingKey[len(pendingKey)-1] = true
				}
			case '[':
				if len(pendingKey) > 0 && !pendingKey[len(pendingKey)-1] {
					pendingKey[len(pendingKey)-1] = true
				}
			case ']':
			}
		case string:
			if len(stack) > 0 && pendingKey[len(pendingKey)-1] {
				keys := stack[len(stack)-1]
				if _, exists := keys[t]; exists {
					return fmt.Errorf("duplicate JSON object key %q", t)
				}
				keys[t] = struct{}{}
				pendingKey[len(pendingKey)-1] = false
			} else if len(pendingKey) > 0 {
				pendingKey[len(pendingKey)-1] = true
			}
		default:
			if len(pendingKey) > 0 && !pendingKey[len(pendingKey)-1] {
				pendingKey[len(pendingKey)-1] = true
			}
		}
	}
}

func rootDigest(ms []Member) string {
	h := sha256.New()
	for _, m := range ms {
		h.Write([]byte(m.Path))
		h.Write([]byte{0})
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(fmt.Sprint(m.Bytes)))
		h.Write([]byte{0})
		h.Write([]byte(m.SHA256))
		h.Write([]byte{10})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func sha(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func writeJSON(p string, v any) error {
	b, _ := json.MarshalIndent(v, "", "  ")
	b = append(b, '\n')
	return os.WriteFile(p, b, 0644)
}
func git(args ...string) string {
	return strings.TrimSpace(string(run("git", args...)))
}
func run(name string, args ...string) []byte {
	c := exec.Command(name, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		die(fmt.Errorf("%s %v: %w\n%s", name, args, err, out))
	}
	return out
}
func die(err error) {
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
