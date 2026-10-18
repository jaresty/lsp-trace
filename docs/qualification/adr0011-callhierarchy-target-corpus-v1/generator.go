package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const revision = "4b910fc0c517cb7801fb51d17d295897230d11fa"
const tree = "761474bf5d90bbe3468df62b1a68a1fa26dc2c82"
const schema = "ADR0011_CALLHIERARCHY_TARGET_CORPUS_V1"

type Source struct {
	Path           string `json:"path"`
	SHA256         string `json:"sha256"`
	Bytes          int    `json:"bytes"`
	Classification string `json:"classification"`
	Reason         string `json:"reason,omitempty"`
}
type Pos struct {
	Line  int `json:"line"`
	UTF16 int `json:"utf16_character"`
	Byte  int `json:"byte_offset"`
}
type Target struct {
	ID                string `json:"id"`
	WorkspaceRevision string `json:"workspace_revision"`
	SourcePath        string `json:"source_path"`
	SourceSHA256      string `json:"source_sha256"`
	ByteStart         int    `json:"byte_start"`
	ByteEnd           int    `json:"byte_end"`
	Start             Pos    `json:"start"`
	End               Pos    `json:"end"`
	Package           string `json:"package"`
	Receiver          string `json:"receiver"`
	Symbol            string `json:"symbol"`
	BindingMode       string `json:"binding_mode"`
}
type Counts struct {
	TrackedGo               int `json:"tracked_go"`
	Admitted                int `json:"admitted"`
	ExcludedTests           int `json:"excluded_tests"`
	ExcludedTestdata        int `json:"excluded_testdata"`
	ExcludedVendor          int `json:"excluded_vendor"`
	ExcludedGenerated       int `json:"excluded_generated"`
	ExcludedDocumentation   int `json:"excluded_documentation"`
	ExcludedToolingEvidence int `json:"excluded_tooling_evidence"`
	Targets                 int `json:"targets"`
	ParseErrors             int `json:"parse_errors"`
	DuplicateIDs            int `json:"duplicate_ids"`
	Unaccounted             int `json:"unaccounted_tracked_go"`
}
type Meta struct {
	Schema               string `json:"schema"`
	WorkspaceRevision    string `json:"workspace_revision"`
	WorkspaceTree        string `json:"workspace_tree"`
	InclusionContract    string `json:"inclusion_contract"`
	SourceManifestSHA256 string `json:"source_manifest_sha256"`
	TargetsJSONSHA256    string `json:"targets_json_sha256"`
	TargetsTSVSHA256     string `json:"targets_tsv_sha256"`
	GeneratorSHA256      string `json:"generator_sha256"`
	Counts               Counts `json:"counts"`
}

func run(repo string, args ...string) []byte {
	c := exec.Command("git", append([]string{"-C", repo}, args...)...)
	b, e := c.Output()
	if e != nil {
		panic(fmt.Sprintf("git %v: %v", args, e))
	}
	return b
}
func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func write(path string, b []byte) {
	if err := os.WriteFile(path, b, 0644); err != nil {
		panic(err)
	}
}
func pretty(v any) []byte {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	return append(b, '\n')
}
func fieldsHash(fields ...string) string {
	h := sha256.New()
	for _, s := range fields {
		binary.Write(h, binary.BigEndian, uint64(len([]byte(s))))
		h.Write([]byte(s))
	}
	return hex.EncodeToString(h.Sum(nil))
}
func gitBlob(repo, path string) []byte { return run(repo, "show", revision+":"+path) }
func utf16Pos(src []byte, off int) Pos {
	line := 0
	start := 0
	for i, b := range src[:off] {
		if b == '\n' {
			line++
			start = i + 1
		}
	}
	if !utf8.Valid(src[start:off]) {
		panic("invalid UTF-8 source")
	}
	return Pos{Line: line, UTF16: len(utf16.Encode([]rune(string(src[start:off])))), Byte: off}
}
func receiver(fset *token.FileSet, d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return ""
	}
	var b bytes.Buffer
	if err := format.Node(&b, fset, d.Recv.List[0].Type); err != nil {
		panic(err)
	}
	return b.String()
}
func generated(src []byte) bool {
	lines := bytes.Split(src, []byte("\n"))
	if len(lines) > 20 {
		lines = lines[:20]
	}
	return bytes.Contains(bytes.Join(lines, []byte("\n")), []byte("Code generated ")) && bytes.Contains(bytes.Join(lines, []byte("\n")), []byte("DO NOT EDIT."))
}
func classify(path string, src []byte) (string, string) {
	if strings.HasPrefix(path, ".pi/evidence/") {
		return "EXCLUDED_TOOLING_EVIDENCE", "retained .pi evidence artifact, outside frozen implementation-source contract"
	}
	if strings.HasPrefix(path, "docs/") {
		return "EXCLUDED_DOCUMENTATION", "frozen source-manifest contract excludes documentation and retained evidence/pilots"
	}
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if p == "vendor" {
			return "EXCLUDED_VENDOR", "tracked path contains vendor directory"
		}
		if p == "testdata" {
			return "EXCLUDED_TESTDATA", "tracked path contains testdata directory"
		}
	}
	if strings.HasSuffix(path, "_test.go") {
		return "EXCLUDED_TEST", "tracked Go test file"
	}
	if generated(src) {
		return "EXCLUDED_GENERATED", "standard generated-code header in first 20 lines"
	}
	return "ADMITTED", "all other tracked .go files; build constraints and platform suffixes do not exclude"
}
func main() {
	if len(os.Args) != 3 {
		panic("usage: generator REPO OUTPUT")
	}
	repo, out := os.Args[1], os.Args[2]
	if strings.TrimSpace(string(run(repo, "rev-parse", "HEAD"))) != revision {
		panic("wrong HEAD")
	}
	if strings.TrimSpace(string(run(repo, "rev-parse", "HEAD^{tree}"))) != tree {
		panic("wrong tree")
	}
	if len(run(repo, "status", "--porcelain=v1", "--untracked-files=all")) != 0 {
		panic("dirty workspace")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		panic(err)
	}
	var paths []string
	for _, p := range strings.Split(strings.TrimSpace(string(run(repo, "ls-tree", "-r", "--name-only", revision))), "\n") {
		if strings.HasSuffix(p, ".go") {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var sources []Source
	var targets []Target
	counts := Counts{TrackedGo: len(paths)}
	seen := map[string]string{}
	for _, p := range paths {
		src := gitBlob(repo, p)
		cl, reason := classify(p, src)
		s := Source{Path: p, SHA256: hash(src), Bytes: len(src), Classification: cl, Reason: reason}
		sources = append(sources, s)
		switch cl {
		case "EXCLUDED_TEST":
			counts.ExcludedTests++
			continue
		case "EXCLUDED_TESTDATA":
			counts.ExcludedTestdata++
			continue
		case "EXCLUDED_VENDOR":
			counts.ExcludedVendor++
			continue
		case "EXCLUDED_GENERATED":
			counts.ExcludedGenerated++
			continue
		case "EXCLUDED_DOCUMENTATION":
			counts.ExcludedDocumentation++
			continue
		case "EXCLUDED_TOOLING_EVIDENCE":
			counts.ExcludedToolingEvidence++
			continue
		}
		counts.Admitted++
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, p, src, parser.AllErrors)
		if err != nil {
			counts.ParseErrors++
			panic(fmt.Sprintf("parse %s: %v", p, err))
		}
		tf := fs.File(f.Pos())
		for _, decl := range f.Decls {
			d, ok := decl.(*ast.FuncDecl)
			if !ok || d.Name == nil {
				continue
			}
			start := tf.Offset(d.Pos())
			end := tf.Offset(d.End())
			recv := receiver(fs, d)
			id := fieldsHash(revision, s.SHA256, fmt.Sprintf("%d", start), fmt.Sprintf("%d", end), f.Name.Name, recv, d.Name.Name)
			if prior, ok := seen[id]; ok {
				counts.DuplicateIDs++
				panic(fmt.Sprintf("duplicate target ID %s: %s and %s", id, prior, p))
			}
			seen[id] = p
			targets = append(targets, Target{ID: id, WorkspaceRevision: revision, SourcePath: p, SourceSHA256: s.SHA256, ByteStart: start, ByteEnd: end, Start: utf16Pos(src, start), End: utf16Pos(src, end), Package: f.Name.Name, Receiver: recv, Symbol: d.Name.Name, BindingMode: "DIRECT_POSITION"})
		}
	}
	accounted := counts.Admitted + counts.ExcludedTests + counts.ExcludedTestdata + counts.ExcludedVendor + counts.ExcludedGenerated + counts.ExcludedDocumentation + counts.ExcludedToolingEvidence
	counts.Unaccounted = counts.TrackedGo - accounted
	if counts.Unaccounted != 0 {
		panic("unaccounted tracked Go files")
	}
	counts.Targets = len(targets)
	sort.Slice(targets, func(i, j int) bool {
		a, b := targets[i], targets[j]
		if a.SourcePath != b.SourcePath {
			return a.SourcePath < b.SourcePath
		}
		if a.ByteStart != b.ByteStart {
			return a.ByteStart < b.ByteStart
		}
		return a.ID < b.ID
	})
	sm := pretty(struct {
		Schema, WorkspaceRevision, WorkspaceTree string
		Sources                                  []Source `json:"sources"`
	}{schema, revision, tree, sources})
	tj := pretty(struct {
		Schema  string   `json:"schema"`
		Targets []Target `json:"targets"`
	}{schema, targets})
	var tb bytes.Buffer
	w := csv.NewWriter(&tb)
	w.Comma = '\t'
	w.Write([]string{"id", "workspace_revision", "source_path", "source_sha256", "byte_start", "byte_end", "start_line", "start_utf16", "end_line", "end_utf16", "package", "receiver", "symbol", "binding_mode"})
	for _, t := range targets {
		w.Write([]string{t.ID, t.WorkspaceRevision, t.SourcePath, t.SourceSHA256, fmt.Sprint(t.ByteStart), fmt.Sprint(t.ByteEnd), fmt.Sprint(t.Start.Line), fmt.Sprint(t.Start.UTF16), fmt.Sprint(t.End.Line), fmt.Sprint(t.End.UTF16), t.Package, t.Receiver, t.Symbol, t.BindingMode})
	}
	w.Flush()
	if w.Error() != nil {
		panic(w.Error())
	}
	gen, err := os.ReadFile(os.Args[0] + ".source")
	if err != nil {
		panic("generator source must be adjacent to executable as <exe>.source")
	}
	meta := pretty(Meta{Schema: schema, WorkspaceRevision: revision, WorkspaceTree: tree, InclusionContract: "all tracked .go paths at pinned tree; exclude docs retained documentation/evidence/pilots, .pi/evidence retained tooling evidence, explicit vendor/testdata path components, *_test.go, and standard generated header; admit every remainder regardless of build constraints", SourceManifestSHA256: hash(sm), TargetsJSONSHA256: hash(tj), TargetsTSVSHA256: hash(tb.Bytes()), GeneratorSHA256: hash(gen), Counts: counts})
	write(filepath.Join(out, "source-manifest.json"), sm)
	write(filepath.Join(out, "targets.json"), tj)
	write(filepath.Join(out, "targets.tsv"), tb.Bytes())
	write(filepath.Join(out, "metadata.json"), meta)
	write(filepath.Join(out, "generator.go"), gen)
}
