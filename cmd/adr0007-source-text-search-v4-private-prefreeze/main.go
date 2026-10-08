package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const base = "docs/pilot/adr0007/source-text-search-v4"
const manifestPath = base + "/PRE_FREEZE_MANIFEST.json"
const dryrunPath = base + "/FREEZE_ENVELOPE.dryrun.json"

type Member struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion        string   `json:"schema_version"`
	Stage                string   `json:"stage"`
	HeadCommit           string   `json:"head_commit"`
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
	head := git("rev-parse", "HEAD")
	m := Manifest{SchemaVersion: "lsp-trace.adr0007.source-text-search.pre-freeze-manifest.private.v1", Stage: "pre_freeze", HeadCommit: head, AllowedRoots: append([]string{}, roots...), AllowedExactFiles: append([]string{}, exactFiles...), ExcludedSelf: manifestPath, FinalFreezeExcluded: base + "/FREEZE.json", TerminalsEmbedRoot: false}
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
	if err := json.Unmarshal(disk, &got); err != nil {
		return err
	}
	if got.OuterRootSHA256 != m.OuterRootSHA256 || got.NormativeMemberCount != m.NormativeMemberCount || got.NormativeBytes != m.NormativeBytes {
		return fmt.Errorf("manifest mismatch: got %s/%d/%d want %s/%d/%d", got.OuterRootSHA256, got.NormativeMemberCount, got.NormativeBytes, m.OuterRootSHA256, m.NormativeMemberCount, m.NormativeBytes)
	}
	if !sameMembers(got.Members, m.Members) {
		return fmt.Errorf("manifest member listing mismatch")
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
	var d struct {
		SchemaVersion       string `json:"schema_version"`
		PreFreezeRootSHA256 string `json:"pre_freeze_root_sha256"`
		Status              string `json:"status"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}
	if d.PreFreezeRootSHA256 != m.OuterRootSHA256 {
		return fmt.Errorf("dryrun root mismatch")
	}
	if d.Status != "DRYRUN_ONLY_NOT_FINAL_FREEZE" {
		return fmt.Errorf("dryrun status mismatch")
	}
	return nil
}

func sameMembers(a, b []Member) bool {
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

func freezeDryrun(m Manifest) any {
	return map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.freeze-envelope.dryrun.v1", "pre_freeze_root_sha256": m.OuterRootSHA256, "members": m.NormativeMemberCount, "bytes": m.NormativeBytes, "status": "DRYRUN_ONLY_NOT_FINAL_FREEZE"}
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
