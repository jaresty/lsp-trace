package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	privateB4C15MappingSHA256     = "58995c12ef846b597f397666f6bc6867990da04319b846d27d72868e9b3bbdb4"
	privateB4C15CandidateCount    = 3507
	privateB4C15CandidateSHA256   = "896f0ba0b962de595590375cb122fdbca42e67ba0c908ba36dd479956c872318"
	privateB4C15DispositionSHA256 = "6da54493caef29aa7874bc3a3d5c0534aaba646ea127a2011f5ef3426d932f4a"
	privateB4C15OwnerOpsSHA256    = "6834dc9a157ac944e3141cb85783c36251f92d383c838c205b063a1ac37014ff"
)

type privateB4C15OperationMapping struct {
	Counts struct {
		Owner, Alias, Copy, Total int
	} `json:"counts"`
	Rows []privateB4C15MappingRow `json:"rows"`
}

type privateB4C15MappingRow struct {
	SiteID, Class, LogicalSeam, Path, Enclosing, OperationKind, Fingerprint string
	AccountOperation, Boundary, Capacity, Failure, NilDefaultOff            string
	ASTDigest, BindingFunction, AdjacencyRelation                           string
	AdjacencyIndex                                                          int
}

type privateB4C15Candidate struct{ ID, Digest, Kind, Display string }

func TestPrivateB4C15OfflineCatalogAndCompactProductionContract(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	candidates := readPrivateB4C15TSV(t, "candidates.tsv", 2, privateB4C15CandidateSHA256)
	rows := readPrivateB4C15TSV(t, "classifications.tsv", 5, privateB4C15DispositionSHA256)
	owners := readPrivateB4C15TSV(t, "owner-operations.tsv", 7, privateB4C15OwnerOpsSHA256)
	if len(candidates) != privateB4C15CandidateCount || len(rows) != privateB4C15CandidateCount {
		t.Fatalf("ASSERT_C15_CANDIDATE_CATALOG descriptors=%d rows=%d", len(candidates), len(rows))
	}
	owned := 0
	mapped := map[string]bool{}
	for _, row := range mapping.Rows {
		mapped[row.SiteID] = true
	}
	for _, row := range rows {
		switch row[1] {
		case "C15-owned":
			owned++
			if !mapped[row[0]] || row[2] == "" || row[3] != "" || row[4] != "" {
				t.Fatalf("ASSERT_C15_OWNED_MAPPING_ROW row=%q", row)
			}
		case "excluded":
			if row[2] != "" || row[3] == "" || row[4] == "" {
				t.Fatalf("ASSERT_C15_TYPED_EXCLUSION row=%q", row)
			}
		default:
			t.Fatalf("ASSERT_C15_DISPOSITION row=%q", row)
		}
	}
	if owned != 44 || len(privateB4C15ProductionSites) != 44 {
		t.Fatalf("ASSERT_C15_COMPACT_44 owned=%d sites=%d", owned, len(privateB4C15ProductionSites))
	}
	if len(owners) == 0 || len(privateB4C15OwnerOperations) != len(owners) {
		t.Fatalf("ASSERT_C15_OWNER_OPERATION_RELATION production=%d corpus=%d", len(privateB4C15OwnerOperations), len(owners))
	}
	for i, row := range owners {
		got := privateB4C15OwnerOperations[i]
		if got != (privateB4C15OwnerOperationDescriptor{row[0], row[1], row[2], row[3], row[4], row[5], row[6]}) {
			t.Fatalf("ASSERT_C15_OWNER_OPERATION_ROW_%d got=%+v corpus=%q", i, got, row)
		}
	}
	if len(owners) == len(privateB4C15ProductionSites) {
		t.Logf("owner relation happens to have %d rows; cardinality remains semantically independent", len(owners))
	}
	for i, row := range mapping.Rows {
		got := privateB4C15ProductionSites[i]
		if got != (privateB4C15ProductionSite{row.SiteID, row.Class, row.LogicalSeam, row.Path, row.Enclosing, row.OperationKind, row.Fingerprint}) {
			t.Fatalf("ASSERT_C15_REGISTRY_ROW_%d got=%+v mapping=%+v", i, got, row)
		}
	}
}

func TestPrivateB4C15RegenerateOfflineCatalog(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	candidateOut, classificationOut := generatePrivateB4C15Catalog(t, mapping)
	if os.Getenv("C15_REGENERATE") != "1" {
		assertPrivateB4C15Generated(t, "candidates.tsv", candidateOut, privateB4C15CandidateSHA256)
		assertPrivateB4C15Generated(t, "classifications.tsv", classificationOut, privateB4C15DispositionSHA256)
		return
	}
	writePrivateB4C15CatalogAtomically(t, map[string][]byte{
		"candidates.tsv":      candidateOut,
		"classifications.tsv": classificationOut,
	})
}

func TestPrivateB4C15RegenerationDoesNotReadGeneratedOutputs(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	wantCandidates, wantClassifications := generatePrivateB4C15Catalog(t, mapping)
	dir := filepath.Join("testdata", "c15-census-v1")
	for _, name := range []string{"candidates.tsv", "classifications.tsv"} {
		path := filepath.Join(dir, name)
		if err := os.Rename(path, path+".deleted-test"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Rename(path+".deleted-test", path) })
	}
	gotCandidates, gotClassifications := generatePrivateB4C15Catalog(t, mapping)
	if !bytes.Equal(gotCandidates, wantCandidates) || !bytes.Equal(gotClassifications, wantClassifications) {
		t.Fatal("ASSERT_C15_CLEAN_REGENERATION_WITHOUT_OUTPUT_SNAPSHOTS")
	}
}

func generatePrivateB4C15Catalog(t *testing.T, mapping privateB4C15OperationMapping) ([]byte, []byte) {
	t.Helper()
	var candidateOut, classificationOut bytes.Buffer
	for _, candidate := range discoverPrivateB4C15Candidates(t) {
		fmt.Fprintf(&candidateOut, "%s\t%s\n", candidate.ID, candidate.Digest)
		category := "ephemeral-runtime"
		rationale := candidate.Kind + " " + candidate.Display + " has no privateB4ByteAccountV2 owner operation at this site"
		if strings.Contains(candidate.Display, ".") {
			category, rationale = "external-library", candidate.Display+" is library behavior outside privateB4ByteAccountV2 custody"
		}
		fmt.Fprintf(&classificationOut, "%s\texcluded\t\t%s\t%s\n", candidate.ID, category, rationale)
	}
	for _, row := range mapping.Rows {
		fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(row.Fingerprint)))
		fmt.Fprintf(&candidateOut, "%s\t%s\n", row.SiteID, fingerprint)
		fmt.Fprintf(&classificationOut, "%s\tC15-owned\t%s\t\t\n", row.SiteID, row.AccountOperation)
	}
	return candidateOut.Bytes(), classificationOut.Bytes()
}

func assertPrivateB4C15Generated(t *testing.T, name string, got []byte, wantHash string) {
	t.Helper()
	want := readPinnedFile(t, name, wantHash)
	if !bytes.Equal(got, want) {
		t.Fatalf("ASSERT_C15_CLEAN_REGENERATION_%s", name)
	}
}

func writePrivateB4C15CatalogAtomically(t *testing.T, generated map[string][]byte) {
	t.Helper()
	dir := filepath.Join("testdata", "c15-census-v1")
	stage, err := os.MkdirTemp(dir, ".c15-catalog-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stage)

	names := []string{"candidates.tsv", "classifications.tsv"}
	for _, name := range names {
		data, ok := generated[name]
		if !ok {
			t.Fatalf("ASSERT_C15_GENERATOR_MISSING_%s", name)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data)) + "\n"
		if err := os.WriteFile(filepath.Join(stage, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage, name+".sha256"), []byte(hash), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"candidates.tsv", "candidates.tsv.sha256", "classifications.tsv", "classifications.tsv.sha256"} {
		if err := os.Rename(filepath.Join(stage, name), filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPrivateB4C15ProductionSiteBindings(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	repo := filepath.Clean(filepath.Join(packageDir(t), ".."))
	verification, err := privateC15VerifyPersistedMapping(repo, mapping, privateC15SemanticContracts, nil)
	if err != nil {
		t.Fatalf("ASSERT_C15_PERSISTED_RESOLVED_AGREEMENT: %v", err)
	}
	if len(verification.Resolved) != 44 || verification.UniqueKeys != 44 || verification.PersistedRows != 44 {
		t.Fatalf("ASSERT_C15_SEMANTIC_EXACT44 resolved=%d keys=%d persisted=%d", len(verification.Resolved), verification.UniqueKeys, verification.PersistedRows)
	}
	t.Logf("C15 raw standalone marker uses=%d (diagnostic only; semantic denominator=44; alias43/44 are distinguished by canonical role/call selectors)", verification.RawMarkerUses)
}

func statementIsC15Binding(stmt ast.Stmt) bool {
	a, ok := stmt.(*ast.AssignStmt)
	if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		return false
	}
	lhs, lok := a.Lhs[0].(*ast.Ident)
	rhs, rok := a.Rhs[0].(*ast.Ident)
	return lok && rok && lhs.Name == "_" && strings.HasPrefix(rhs.Name, "privateC15Site")
}

func privateB4C15ExactBindingMatch(row privateB4C15MappingRow, path, function, relation string, index int, digest string) bool {
	return path == row.Path && function == row.BindingFunction && relation == row.AdjacencyRelation &&
		index == row.AdjacencyIndex && relation == adjacencyRelation(index) && (digest == "" || digest == row.ASTDigest)
}

func adjacencyRelation(index int) string {
	if index < 0 {
		return "before"
	}
	if index > 0 {
		return "after"
	}
	return "self"
}

func astDigest(fset *token.FileSet, source []byte, n ast.Node) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(normalizedNodeSource(fset, source, n))))
}

func normalizedNodeSource(fset *token.FileSet, source []byte, n ast.Node) string {
	file := fset.File(n.Pos())
	start, end := file.Offset(n.Pos()), file.Offset(n.End())
	return strings.ToLower(regexp.MustCompile(`\s+`).ReplaceAllString(string(source[start:end]), ""))
}

func enclosingFunction(file *ast.File, pos token.Pos) string {
	name := "<package>"
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Pos() <= pos && pos <= fn.End() {
			name = fn.Name.Name
		}
		return true
	})
	return name
}

func mappingEnclosingContains(mapping, fn string) bool {
	return strings.Contains(strings.ToLower(mapping), strings.ToLower(fn))
}

func privateB4C15ConstName(site string) string {
	parts := regexp.MustCompile(`[A-Za-z0-9]+`).FindAllString(site, -1)
	var b strings.Builder
	b.WriteString("privateC15Site")
	for _, part := range parts {
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}

func readPrivateB4C15Mapping(t *testing.T) privateB4C15OperationMapping {
	t.Helper()
	data := readPinnedFile(t, "operation-mapping.json", privateB4C15MappingSHA256)
	var mapping privateB4C15OperationMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		t.Fatal(err)
	}
	if mapping.Counts.Owner != 15 || mapping.Counts.Alias != 7 || mapping.Counts.Copy != 22 || mapping.Counts.Total != 44 || len(mapping.Rows) != 44 {
		t.Fatalf("ASSERT_C15_MAPPING_COUNTS %+v rows=%d", mapping.Counts, len(mapping.Rows))
	}
	return mapping
}

func readPrivateB4C15TSV(t *testing.T, name string, fields int, wantHash string) [][]string {
	t.Helper()
	data := readPinnedFile(t, name, wantHash)
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	out := make([][]string, 0, len(lines))
	for i, line := range lines {
		row := strings.Split(line, "\t")
		if len(row) != fields {
			t.Fatalf("ASSERT_C15_TSV_FIELD_COUNT_%s line=%d got=%d want=%d", name, i+1, len(row), fields)
		}
		out = append(out, row)
	}
	return out
}

func readPinnedFile(t *testing.T, name, wantHash string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "c15-census-v1", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != wantHash {
		t.Fatalf("ASSERT_C15_REPRODUCIBILITY_HASH_%s got=%s want=%s", name, got, wantHash)
	}
	sidecar, err := os.ReadFile(path + ".sha256")
	if err != nil || strings.TrimSpace(string(sidecar)) != got {
		t.Fatalf("ASSERT_C15_SIDECAR_%s got=%q err=%v", name, strings.TrimSpace(string(sidecar)), err)
	}
	return data
}

func packageDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

func TestPrivateB4C15MappingOrderIsDeterministic(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	seen := map[string]bool{}
	classes := map[string]int{}
	for _, row := range mapping.Rows {
		if seen[row.SiteID] {
			t.Fatalf("ASSERT_C15_DUPLICATE_MAPPING_ID %s", row.SiteID)
		}
		seen[row.SiteID] = true
		classes[row.Class]++
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if classes["owner"] != 15 || classes["alias"] != 7 || classes["copy"] != 22 || len(keys) != 44 {
		t.Fatalf("ASSERT_C15_MAPPING_CLASS_COUNTS classes=%v keys=%d", classes, len(keys))
	}
}
