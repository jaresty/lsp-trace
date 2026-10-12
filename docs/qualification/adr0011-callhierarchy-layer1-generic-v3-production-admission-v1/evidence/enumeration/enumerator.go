package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const adr0011C15AcceptedPatchSHA = "e78f86d07354f03517b4d25ca42079cda0dc1d3114cf5ee4d3b4a5a4904137e0"

type adr0011C15Location struct {
	ID, Path, Enclosing, Kind, Digest          string
	StartLine, StartColumn, EndLine, EndColumn int
}

type adr0011C15Parsed struct {
	Order []string
	Rows  map[string]string
}

type adr0011C15Metadata struct {
	Schema                             string `json:"schema"`
	Head                               string `json:"head"`
	AcceptedPatchSHA256                string `json:"accepted_patch_sha256"`
	EnumeratorSHA256                   string `json:"enumerator_sha256"`
	BaselineCandidatesSHA256           string `json:"baseline_candidates_sha256"`
	BaselineClassificationsSHA256      string `json:"baseline_classifications_sha256"`
	GeneratedCandidatesSHA256          string `json:"generated_candidates_sha256"`
	GeneratedClassificationsSHA256     string `json:"generated_classifications_sha256"`
	BaselineCandidatesBytes            int    `json:"baseline_candidates_bytes"`
	BaselineClassificationsBytes       int    `json:"baseline_classifications_bytes"`
	GeneratedCandidatesBytes           int    `json:"generated_candidates_bytes"`
	GeneratedClassificationsBytes      int    `json:"generated_classifications_bytes"`
	BaselineCount                      int    `json:"baseline_count"`
	GeneratedCount                     int    `json:"generated_count"`
	Added                              int    `json:"added"`
	Removed                            int    `json:"removed"`
	Changed                            int    `json:"changed"`
	Unchanged                          int    `json:"unchanged"`
	BaselineDuplicateIDs               int    `json:"baseline_duplicate_ids"`
	GeneratedDuplicateIDs              int    `json:"generated_duplicate_ids"`
	BaselineKeySetsEqual               bool   `json:"baseline_key_sets_equal"`
	GeneratedKeySetsEqual              bool   `json:"generated_key_sets_equal"`
	CanonicalDiscoveryMatchesGenerated bool   `json:"canonical_discovery_matches_generated"`
	SourceLocationsComplete            bool   `json:"source_locations_complete"`
	DeterministicRerunEqual            bool   `json:"deterministic_rerun_equal"`
	OrderingOnlyDifference             bool   `json:"ordering_only_difference"`
	ClassificationDisposition          string `json:"classification_disposition"`
}

func TestADR0011CallHierarchyC15DeltaEnumerate(t *testing.T) {
	out := adr0011C15OutputDir(t)
	repo := filepath.Dir(packageDir(t))
	stage := t.TempDir()

	head := adr0011C15Git(t, repo, "rev-parse", "HEAD")
	if head != "ce4a1838f8a10bafc13429031e55df7e4dd812c6" {
		t.Fatalf("HEAD=%s", head)
	}
	patch := adr0011C15GitBytes(t, repo, "diff", "--binary", "--", "sessionruntime/b4_callhierarchy_layer1_generic_v3_test.go", "sessionruntime/b4_owned_method_pair_lease.go", "sessionruntime/owned_method_pair.go", "sessionruntime/sessionruntime.go")
	if adr0011C15SHA(patch) != adr0011C15AcceptedPatchSHA {
		t.Fatalf("accepted patch sha=%s", adr0011C15SHA(patch))
	}
	changed := strings.Fields(adr0011C15Git(t, repo, "diff", "--name-only"))
	wantChanged := []string{"sessionruntime/b4_callhierarchy_layer1_generic_v3_test.go", "sessionruntime/b4_owned_method_pair_lease.go", "sessionruntime/owned_method_pair.go", "sessionruntime/sessionruntime.go"}
	if !equalStrings(changed, wantChanged) {
		t.Fatalf("unexpected pre-enumerator changes=%v", changed)
	}

	mapping := readPrivateB4C15Mapping(t)
	if len(mapping.Rows) != 44 || mapping.Counts.Total != 44 || mapping.Counts.Owner != 15 || mapping.Counts.Alias != 7 || mapping.Counts.Copy != 22 {
		t.Fatalf("mapping counts=%+v rows=%d", mapping.Counts, len(mapping.Rows))
	}
	discovered := discoverPrivateB4C15Candidates(t)
	generatedCandidates, generatedClassifications := generatePrivateB4C15Catalog(t, mapping)
	discovered2 := discoverPrivateB4C15Candidates(t)
	generatedCandidates2, generatedClassifications2 := generatePrivateB4C15Catalog(t, mapping)
	if !bytes.Equal(generatedCandidates, generatedCandidates2) || !bytes.Equal(generatedClassifications, generatedClassifications2) {
		t.Fatal("canonical deterministic rerun mismatch")
	}
	if len(discovered) != len(discovered2) {
		t.Fatal("canonical discovery rerun count mismatch")
	}

	baselineCandidates := adr0011C15Read(t, filepath.Join(repo, "sessionruntime/testdata/c15-census-v2/candidates.tsv"))
	baselineClassifications := adr0011C15Read(t, filepath.Join(repo, "sessionruntime/testdata/c15-census-v2/classifications.tsv"))
	adr0011C15VerifySidecar(t, baselineCandidates, adr0011C15Read(t, filepath.Join(repo, "sessionruntime/testdata/c15-census-v2/candidates.tsv.sha256")))
	adr0011C15VerifySidecar(t, baselineClassifications, adr0011C15Read(t, filepath.Join(repo, "sessionruntime/testdata/c15-census-v2/classifications.tsv.sha256")))
	bc := adr0011C15Parse(t, baselineCandidates, 2, "baseline candidates")
	bd := adr0011C15Parse(t, baselineClassifications, 5, "baseline classifications")
	gc := adr0011C15Parse(t, generatedCandidates, 2, "generated candidates")
	gd := adr0011C15Parse(t, generatedClassifications, 5, "generated classifications")
	if len(bc.Rows) != 3847 || len(bd.Rows) != 3847 {
		t.Fatalf("baseline counts=%d/%d", len(bc.Rows), len(bd.Rows))
	}
	if len(gc.Rows) != 4154 || len(gd.Rows) != 4154 {
		t.Fatalf("generated counts=%d/%d want=4154", len(gc.Rows), len(gd.Rows))
	}
	if !adr0011C15SameKeys(bc.Rows, bd.Rows) || !adr0011C15SameKeys(gc.Rows, gd.Rows) {
		t.Fatal("candidate/classification key-set mismatch")
	}
	if len(discovered)+len(mapping.Rows) != len(gc.Rows) {
		t.Fatalf("canonical discovery reconciliation discovered=%d mapping=%d generated=%d", len(discovered), len(mapping.Rows), len(gc.Rows))
	}
	for _, c := range discovered {
		if gc.Rows[c.ID] != c.ID+"\t"+c.Digest {
			t.Fatalf("canonical candidate mismatch id=%s", c.ID)
		}
	}

	locations := adr0011C15CorrelateLocations(t, repo, discovered)
	for id, location := range adr0011C15CorrelateMappingLocations(t, repo, mapping) {
		locations[id] = location
	}
	baselineLocations := adr0011C15CorrelateHEADLocations(t, repo, bc, mapping)
	added, removed, changedSet, unchanged := []string{}, []string{}, []string{}, []string{}
	for id := range gc.Rows {
		if _, ok := bc.Rows[id]; !ok {
			added = append(added, id)
		}
	}
	for id := range bc.Rows {
		if _, ok := gc.Rows[id]; !ok {
			removed = append(removed, id)
		}
	}
	for id := range gc.Rows {
		if _, ok := bc.Rows[id]; ok {
			if gc.Rows[id] != bc.Rows[id] || gd.Rows[id] != bd.Rows[id] {
				changedSet = append(changedSet, id)
			} else {
				unchanged = append(unchanged, id)
			}
		}
	}
	for _, set := range [][]string{added, removed, changedSet, unchanged} {
		sort.Strings(set)
	}
	if len(added)+len(removed)+len(changedSet)+len(unchanged) != len(gc.Rows)+len(removed) {
		t.Fatal("delta reconciliation failed")
	}

	artifacts := map[string][]byte{}
	artifacts["candidates.tsv"] = generatedCandidates
	artifacts["classifications.tsv"] = generatedClassifications
	artifacts["candidates.tsv.sha256"] = []byte(adr0011C15SHA(generatedCandidates) + "\n")
	artifacts["classifications.tsv.sha256"] = []byte(adr0011C15SHA(generatedClassifications) + "\n")
	artifacts["all-source-locations.tsv"] = adr0011C15LocationsTSV(t, gc, mapping, locations)
	artifacts["ADDED.tsv"] = adr0011C15DeltaTSV(t, added, "ADDED", bc, bd, gc, gd, mapping, locations, baselineLocations)
	artifacts["REMOVED.tsv"] = adr0011C15DeltaTSV(t, removed, "REMOVED", bc, bd, gc, gd, mapping, locations, baselineLocations)
	artifacts["CHANGED.tsv"] = adr0011C15DeltaTSV(t, changedSet, "CHANGED", bc, bd, gc, gd, mapping, locations, baselineLocations)
	artifacts["UNCHANGED.tsv"] = adr0011C15DeltaTSV(t, unchanged, "UNCHANGED", bc, bd, gc, gd, mapping, locations, baselineLocations)

	enumeratorPath := filepath.Join(repo, "sessionruntime/adr0011_callhierarchy_c15_delta_enumerator_private_test.go")
	enumerator := adr0011C15Read(t, enumeratorPath)
	meta := adr0011C15Metadata{Schema: "lsp-trace/adr0011-c15-delta-enumeration-v1", Head: head, AcceptedPatchSHA256: adr0011C15AcceptedPatchSHA, EnumeratorSHA256: adr0011C15SHA(enumerator), BaselineCandidatesSHA256: adr0011C15SHA(baselineCandidates), BaselineClassificationsSHA256: adr0011C15SHA(baselineClassifications), GeneratedCandidatesSHA256: adr0011C15SHA(generatedCandidates), GeneratedClassificationsSHA256: adr0011C15SHA(generatedClassifications), BaselineCandidatesBytes: len(baselineCandidates), BaselineClassificationsBytes: len(baselineClassifications), GeneratedCandidatesBytes: len(generatedCandidates), GeneratedClassificationsBytes: len(generatedClassifications), BaselineCount: len(bc.Rows), GeneratedCount: len(gc.Rows), Added: len(added), Removed: len(removed), Changed: len(changedSet), Unchanged: len(unchanged), BaselineKeySetsEqual: true, GeneratedKeySetsEqual: true, CanonicalDiscoveryMatchesGenerated: true, SourceLocationsComplete: true, DeterministicRerunEqual: true, OrderingOnlyDifference: adr0011C15OrderingOnly(bc.Order, gc.Order, bc.Rows, gc.Rows), ClassificationDisposition: "UNRESOLVED_REVIEW_REQUIRED"}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	metaBytes = append(metaBytes, '\n')
	artifacts["metadata.json"] = metaBytes
	artifacts["enumerator.go"] = enumerator
	artifacts["enumerator.go.sha256"] = []byte(adr0011C15SHA(enumerator) + "\n")
	artifacts["accepted4.patch"] = patch
	artifacts["accepted4.patch.sha256"] = []byte(adr0011C15SHA(patch) + "\n")
	artifacts["COMMANDS.txt"] = []byte("ADR0011_C15_DELTA_OUTPUT=" + out + " go test ./sessionruntime -run '^TestADR0011CallHierarchyC15DeltaEnumerate$' -count=1 -v\n")
	artifacts["RESULT.md"] = adr0011C15Result(meta)

	for name, data := range artifacts {
		adr0011C15Write(t, filepath.Join(stage, name), data)
	}
	manifest := adr0011C15Manifest(artifacts)
	adr0011C15Write(t, filepath.Join(stage, "MANIFEST.sha256"), manifest)
	artifacts["MANIFEST.sha256"] = manifest
	for name, data := range artifacts {
		adr0011C15Write(t, filepath.Join(out, name), data)
	}
}

func adr0011C15OutputDir(t *testing.T) string {
	t.Helper()
	raw := os.Getenv("ADR0011_C15_DELTA_OUTPUT")
	if raw == "" {
		t.Fatal("ADR0011_C15_DELTA_OUTPUT unset")
	}
	out, err := filepath.Abs(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err = filepath.EvalSymlinks(out)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("output nonempty: %s", out)
	}
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = packageDir(t)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		root, _ := filepath.Abs(strings.TrimPrefix(line, "worktree "))
		rel, err := filepath.Rel(root, out)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			t.Fatalf("output inside worktree %s", root)
		}
	}
	return out
}

func adr0011C15CorrelateLocations(t *testing.T, repo string, canonical []privateB4C15Candidate) map[string]adr0011C15Location {
	t.Helper()
	dir := filepath.Join(repo, "sessionruntime")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		ok, err := build.Default.MatchFile(dir, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	out := map[string]adr0011C15Location{}
	canonicalByID := map[string]privateB4C15Candidate{}
	for _, c := range canonical {
		canonicalByID[c.ID] = c
	}
	counts := map[string]int{}
	for _, name := range names {
		src := adr0011C15Read(t, filepath.Join(dir, name))
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, src, parser.AllErrors)
		if err != nil {
			t.Fatal(err)
		}
		fileKey := strings.TrimSuffix(strings.ReplaceAll(name, "_", "."), ".go")
		ast.Inspect(f, func(n ast.Node) bool {
			var kind, display string
			switch x := n.(type) {
			case *ast.CallExpr:
				display = privateB4C15ExprName(x.Fun)
				kind = "call"
				if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "make" || id.Name == "append") {
					kind = "builtin-" + id.Name
				}
			case *ast.CompositeLit:
				kind, display = "composite-literal", "composite.literal"
			default:
				return true
			}
			fn := strings.ToLower(enclosingFunction(f, n.Pos()))
			base := "alloc." + fileKey + "." + fn + "." + kind + "." + strings.ToLower(strings.ReplaceAll(display, "_", "."))
			counts[base]++
			id := fmt.Sprintf("%s.%d", base, counts[base])
			c, ok := canonicalByID[id]
			if !ok {
				t.Fatalf("correlation produced noncanonical id=%s", id)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(normalizedNodeSource(fset, src, n))))
			if digest != c.Digest || kind != c.Kind || display != c.Display {
				t.Fatalf("correlation mismatch id=%s", id)
			}
			sp, ep := fset.Position(n.Pos()), fset.Position(n.End())
			out[id] = adr0011C15Location{id, name, fn, kind, digest, sp.Line, sp.Column, ep.Line, ep.Column}
			return true
		})
	}
	if len(out) != len(canonical) {
		t.Fatalf("location completeness=%d/%d", len(out), len(canonical))
	}
	return out
}

func adr0011C15CorrelateMappingLocations(t *testing.T, repo string, mapping privateB4C15OperationMapping) map[string]adr0011C15Location {
	t.Helper()
	verified, err := privateC15VerifyPersistedMapping(repo, mapping, privateC15SemanticContracts, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows := adr0011C15MappingByID(mapping)
	out := map[string]adr0011C15Location{}
	for _, occurrence := range verified.Resolved {
		row := rows[occurrence.Key.SiteID]
		src := adr0011C15Read(t, filepath.Join(repo, filepath.FromSlash(occurrence.Path)))
		startOffset, endOffset := int(occurrence.Operation.Pos())-1, int(occurrence.Operation.End())-1
		if startOffset < 0 || endOffset < startOffset || endOffset > len(src) {
			t.Fatalf("mapping offsets site=%s start=%d end=%d bytes=%d", row.SiteID, startOffset, endOffset, len(src))
		}
		sl, sc := adr0011C15LineColumn(src, startOffset)
		el, ec := adr0011C15LineColumn(src, endOffset)
		out[row.SiteID] = adr0011C15Location{row.SiteID, row.Path, row.BindingFunction, row.OperationKind, row.ASTDigest, sl, sc, el, ec}
	}
	if len(out) != 44 {
		t.Fatalf("mapping location completeness=%d/44", len(out))
	}
	return out
}

func adr0011C15LineColumn(src []byte, offset int) (line, column int) {
	line, column = 1, 1
	for _, b := range src[:offset] {
		if b == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return line, column
}

func adr0011C15CorrelateHEADLocations(t *testing.T, repo string, baseline adr0011C15Parsed, mapping privateB4C15OperationMapping) map[string]adr0011C15Location {
	t.Helper()
	mapped := adr0011C15MappingByID(mapping)
	canonical := map[string]privateB4C15Candidate{}
	for id, row := range baseline.Rows {
		if _, ok := mapped[id]; ok {
			continue
		}
		parts := strings.Split(row, "\t")
		canonical[id] = privateB4C15Candidate{ID: id, Digest: parts[1]}
	}
	namesRaw := adr0011C15Git(t, repo, "ls-tree", "-r", "--name-only", "HEAD", "--", "sessionruntime")
	var names []string
	for _, path := range strings.Split(namesRaw, "\n") {
		name := strings.TrimPrefix(path, "sessionruntime/")
		if strings.Contains(name, "/") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		ok, err := build.Default.MatchFile(filepath.Join(repo, "sessionruntime"), name)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out, counts := map[string]adr0011C15Location{}, map[string]int{}
	for _, name := range names {
		src := adr0011C15GitBytes(t, repo, "show", "HEAD:sessionruntime/"+name)
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, src, parser.AllErrors)
		if err != nil {
			t.Fatal(err)
		}
		fileKey := strings.TrimSuffix(strings.ReplaceAll(name, "_", "."), ".go")
		ast.Inspect(f, func(n ast.Node) bool {
			var kind, display string
			switch x := n.(type) {
			case *ast.CallExpr:
				display = privateB4C15ExprName(x.Fun)
				kind = "call"
				if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "make" || id.Name == "append") {
					kind = "builtin-" + id.Name
				}
			case *ast.CompositeLit:
				kind, display = "composite-literal", "composite.literal"
			default:
				return true
			}
			fn := strings.ToLower(enclosingFunction(f, n.Pos()))
			base := "alloc." + fileKey + "." + fn + "." + kind + "." + strings.ToLower(strings.ReplaceAll(display, "_", "."))
			counts[base]++
			id := fmt.Sprintf("%s.%d", base, counts[base])
			c, ok := canonical[id]
			if !ok {
				return true
			}
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(normalizedNodeSource(fset, src, n))))
			if digest != c.Digest {
				t.Fatalf("HEAD correlation mismatch id=%s", id)
			}
			sp, ep := fset.Position(n.Pos()), fset.Position(n.End())
			out[id] = adr0011C15Location{id, name, fn, kind, digest, sp.Line, sp.Column, ep.Line, ep.Column}
			return true
		})
	}
	return out
}

func adr0011C15Parse(t *testing.T, data []byte, fields int, label string) adr0011C15Parsed {
	t.Helper()
	p := adr0011C15Parsed{Rows: map[string]string{}}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("%s missing final newline", label)
	}
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != fields || parts[0] == "" {
			t.Fatalf("%s malformed row %d", label, i+1)
		}
		if _, ok := p.Rows[parts[0]]; ok {
			t.Fatalf("%s duplicate id=%s", label, parts[0])
		}
		p.Order = append(p.Order, parts[0])
		p.Rows[parts[0]] = line
	}
	return p
}
func adr0011C15SameKeys(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
func adr0011C15OrderingOnly(a, b []string, ar, br map[string]string) bool {
	if !adr0011C15SameKeys(ar, br) {
		return false
	}
	for k := range ar {
		if ar[k] != br[k] {
			return false
		}
	}
	return !equalStrings(a, b)
}
func equalStrings(a, b []string) bool {
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
func adr0011C15Read(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func adr0011C15Write(t *testing.T, path string, data []byte) {
	t.Helper()
	if e := os.WriteFile(path, data, 0o644); e != nil {
		t.Fatal(e)
	}
}
func adr0011C15SHA(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func adr0011C15VerifySidecar(t *testing.T, data, side []byte) {
	t.Helper()
	if strings.TrimSpace(string(side)) != adr0011C15SHA(data) {
		t.Fatal("sidecar mismatch")
	}
}
func adr0011C15Git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(string(adr0011C15GitBytes(t, repo, args...)))
}
func adr0011C15GitBytes(t *testing.T, repo string, args ...string) []byte {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = repo
	b, e := c.Output()
	if e != nil {
		t.Fatalf("git %v: %v", args, e)
	}
	return b
}

func adr0011C15MappingByID(mapping privateB4C15OperationMapping) map[string]privateB4C15MappingRow {
	out := map[string]privateB4C15MappingRow{}
	for _, r := range mapping.Rows {
		out[r.SiteID] = r
	}
	return out
}
func adr0011C15Evidence(id string, mapping map[string]privateB4C15MappingRow, locations map[string]adr0011C15Location) (path, span, enclosing, kind, digest, provenance string) {
	if l, ok := locations[id]; ok {
		return "sessionruntime/" + l.Path, fmt.Sprintf("%d:%d-%d:%d", l.StartLine, l.StartColumn, l.EndLine, l.EndColumn), l.Enclosing, l.Kind, l.Digest, "canonical-discovery+exact-AST-correlation"
	}
	if r, ok := mapping[id]; ok {
		return r.Path, "canonical-mapping-operation", r.BindingFunction, r.OperationKind, r.ASTDigest, "accepted-44-row-mapping"
	}
	return "ABSENT", "ABSENT", "ABSENT", "ABSENT", "ABSENT", "absent"
}
func adr0011C15DeltaTSV(t *testing.T, ids []string, delta string, bc, bd, gc, gd adr0011C15Parsed, mapping privateB4C15OperationMapping, locations, baselineLocations map[string]adr0011C15Location) []byte {
	t.Helper()
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Comma = '\t'
	_ = w.Write([]string{"candidate_id", "delta", "baseline_candidate_row", "generated_candidate_row", "baseline_classification_row", "generated_classification_row", "source_path", "source_span", "enclosing", "ast_kind", "ast_hash", "provenance", "experiment_disposition"})
	m := adr0011C15MappingByID(mapping)
	for _, id := range ids {
		p, s, e, k, h, prov := adr0011C15Evidence(id, m, locations)
		if delta == "REMOVED" {
			p, s, e, k, h, prov = adr0011C15Evidence(id, m, baselineLocations)
			if prov == "canonical-discovery+exact-AST-correlation" {
				prov = "frozen-baseline-id+committed-HEAD-exact-AST-correlation"
			}
		}
		_ = w.Write([]string{id, delta, orAbsent(bc.Rows[id]), orAbsent(gc.Rows[id]), orAbsent(bd.Rows[id]), orAbsent(gd.Rows[id]), p, s, e, k, h, prov, func() string {
			if delta == "UNCHANGED" {
				return "UNCHANGED"
			}
			return "UNRESOLVED_REVIEW_REQUIRED"
		}()})
	}
	w.Flush()
	if w.Error() != nil {
		t.Fatal(w.Error())
	}
	return b.Bytes()
}
func adr0011C15LocationsTSV(t *testing.T, gc adr0011C15Parsed, mapping privateB4C15OperationMapping, locations map[string]adr0011C15Location) []byte {
	t.Helper()
	ids := append([]string(nil), gc.Order...)
	m := adr0011C15MappingByID(mapping)
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Comma = '\t'
	_ = w.Write([]string{"candidate_id", "source_path", "source_span", "enclosing", "ast_kind", "ast_hash", "provenance"})
	for _, id := range ids {
		p, s, e, k, h, prov := adr0011C15Evidence(id, m, locations)
		_ = w.Write([]string{id, p, s, e, k, h, prov})
	}
	w.Flush()
	if w.Error() != nil {
		t.Fatal(w.Error())
	}
	return b.Bytes()
}
func orAbsent(s string) string {
	if s == "" {
		return "ABSENT"
	}
	return s
}
func adr0011C15Manifest(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "%s  %s\n", adr0011C15SHA(files[n]), n)
	}
	return []byte(b.String())
}
func adr0011C15Result(m adr0011C15Metadata) []byte {
	return []byte(fmt.Sprintf("# C15 Delta Enumeration Result\n\n- Generated candidates/classifications: %d/%d\n- Delta: ADDED=%d REMOVED=%d CHANGED=%d UNCHANGED=%d\n- Duplicate candidate IDs: baseline=%d generated=%d\n- Candidate/classification key-set equality: baseline=%t generated=%t\n- Canonical discovery reconciliation: %t\n- Source-location completeness: %t\n- Deterministic in-process rerun equality: %t\n- Accepted patch SHA-256: `%s`\n\n## Disposition\n\n`C15_DELTA_BLOCKED_PENDING_ROW_ADJUDICATION`\n\nThe canonical catalog and exact candidate-ID delta are complete. Every affected row is preserved with canonical old/new classification and provenance, but this mechanical enumerator does not infer NECESSARY, REUSE, or FALSE_POSITIVE from syntax. Independent semantic review is required before a C15_DELTA_GO candidate can be issued.\n\n## Coverage limitations\n\nThis evidence covers build-active non-test Go source selected by the canonical C15 v2 helper and the accepted 44-row mapping. It does not authorize classification changes, artifact succession, implementation, commits, or directional behavior. Mapping-row source spans come from the canonical semantic mapping verifier's uniquely resolved operations; ordinary candidate spans are independently correlated back to canonical candidate IDs and AST digests.\n", m.GeneratedCount, m.GeneratedCount, m.Added, m.Removed, m.Changed, m.Unchanged, m.BaselineDuplicateIDs, m.GeneratedDuplicateIDs, m.BaselineKeySetsEqual, m.GeneratedKeySetsEqual, m.CanonicalDiscoveryMatchesGenerated, m.SourceLocationsComplete, m.DeterministicRerunEqual, m.AcceptedPatchSHA256))
}

var _ = strconv.Itoa
