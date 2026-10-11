package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const (
	privateB4C15V2BaseHEAD       = "374e7c7ba31363ebd19800f8d027cd22a290a464"
	privateB4C15V2BaseCount      = 3805
	privateB4C15V2FinalCount     = 3847
	privateB4C15V2AddedCount     = 42
	privateB4C15V2MappingCount   = 44
	privateB4C15V2RegenerateEnv  = "C15_V2_REGENERATE"
	privateB4C15V2RegenerateTest = "TestPrivateB4C15V2SuccessorCatalog"
)

type privateB4C15V2Manifest struct {
	Schema                  string            `json:"schema"`
	BaseHEAD                string            `json:"base_head"`
	Generator               string            `json:"generator"`
	CanonicalCommand        string            `json:"canonical_command"`
	GeneratorInputs         map[string]string `json:"generator_inputs_sha256"`
	PredecessorVerification map[string]string `json:"predecessor_verification_sha256"`
	BaseCount               int               `json:"base_v1_count"`
	FinalCount              int               `json:"final_v2_count"`
	AddedExcluded           int               `json:"added_excluded"`
	Removed                 int               `json:"removed"`
	Reclassified            int               `json:"reclassified"`
	UnchangedMapping        int               `json:"unchanged_mapping"`
	ManagerPropagationClaim bool              `json:"manager_propagation_claim"`
}

func TestPrivateB4C15V2SuccessorCatalog(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	candidates, classifications := generatePrivateB4C15Catalog(t, mapping)
	if got := bytes.Count(candidates, []byte{'\n'}); got != privateB4C15V2FinalCount {
		t.Fatalf("ASSERT_C15_V2_FINAL_CANDIDATES got=%d want=%d", got, privateB4C15V2FinalCount)
	}
	if got := bytes.Count(classifications, []byte{'\n'}); got != privateB4C15V2FinalCount {
		t.Fatalf("ASSERT_C15_V2_FINAL_CLASSIFICATIONS got=%d want=%d", got, privateB4C15V2FinalCount)
	}

	v1Candidates := mustReadC15File(t, "candidates.tsv")
	v1Classifications := mustReadC15File(t, "classifications.tsv")
	assertPrivateB4C15V2Delta(t, v1Candidates, v1Classifications, candidates, classifications)

	manifest := privateB4C15V2Manifest{
		Schema:           "lsp-trace/c15-census-successor-v2",
		BaseHEAD:         privateB4C15V2BaseHEAD,
		Generator:        "sessionruntime/" + filepath.Base("b4_c15_census_v2_successor_private_test.go") + ":" + privateB4C15V2RegenerateTest,
		CanonicalCommand: privateB4C15V2RegenerateEnv + "=1 go test ./sessionruntime -run '^" + privateB4C15V2RegenerateTest + "$' -count=1",
		GeneratorInputs: map[string]string{
			"active-production-go-source":          privateB4C15V2SourceDigest(t),
			"c15-census-v1/operation-mapping.json": privateB4C15MappingSHA256,
		},
		PredecessorVerification: map[string]string{
			"c15-census-v1/candidates.tsv":      privateB4C15CandidateSHA256,
			"c15-census-v1/classifications.tsv": privateB4C15DispositionSHA256,
		},
		BaseCount: privateB4C15V2BaseCount, FinalCount: privateB4C15V2FinalCount,
		AddedExcluded: privateB4C15V2AddedCount, Removed: 0, Reclassified: 0,
		UnchangedMapping: privateB4C15V2MappingCount, ManagerPropagationClaim: false,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes = append(manifestBytes, '\n')

	generated := map[string][]byte{
		"candidates.tsv": candidates, "classifications.tsv": classifications, "manifest.json": manifestBytes,
	}
	if os.Getenv(privateB4C15V2RegenerateEnv) == "1" {
		writePrivateB4C15V2(t, generated)
		return
	}
	assertPrivateB4C15V2Pinned(t, generated)
}

func assertPrivateB4C15V2Delta(t *testing.T, v1Candidates, v1Classifications, v2Candidates, v2Classifications []byte) {
	t.Helper()
	v1c := privateB4C15V2Rows(t, v1Candidates, 2)
	v1d := privateB4C15V2Rows(t, v1Classifications, 5)
	v2c := privateB4C15V2Rows(t, v2Candidates, 2)
	v2d := privateB4C15V2Rows(t, v2Classifications, 5)
	if len(v1c) != privateB4C15V2BaseCount || len(v1d) != privateB4C15V2BaseCount {
		t.Fatalf("ASSERT_C15_V2_BASE_COUNT candidates=%d classifications=%d", len(v1c), len(v1d))
	}
	added := 0
	for id, row := range v1c {
		if got, ok := v2c[id]; !ok || got != row {
			t.Fatalf("ASSERT_C15_V2_V1_CANDIDATE_BYTE_IDENTITY id=%s got=%q want=%q", id, got, row)
		}
		if got, ok := v2d[id]; !ok || got != v1d[id] {
			t.Fatalf("ASSERT_C15_V2_ZERO_REMOVED_OR_RECLASSIFIED id=%s got=%q want=%q", id, got, v1d[id])
		}
	}
	for id := range v2c {
		if _, ok := v1c[id]; ok {
			continue
		}
		added++
		row := strings.Split(v2d[id], "\t")
		if len(row) != 5 || row[1] != "excluded" || row[2] != "" || row[3] == "" || row[4] == "" {
			t.Fatalf("ASSERT_C15_V2_ADDITION_NOT_TYPED_EXCLUDED id=%s row=%q", id, v2d[id])
		}
	}
	if added != privateB4C15V2AddedCount || len(v2c)-len(v1c) != privateB4C15V2AddedCount {
		t.Fatalf("ASSERT_C15_V2_EXACT_DELTA added=%d delta=%d", added, len(v2c)-len(v1c))
	}
	owned := 0
	for _, row := range v2d {
		if strings.Split(row, "\t")[1] == "C15-owned" {
			owned++
		}
	}
	if owned != privateB4C15V2MappingCount {
		t.Fatalf("ASSERT_C15_V2_MAPPING_UNCHANGED got=%d want=%d", owned, privateB4C15V2MappingCount)
	}
}

func privateB4C15V2Rows(t *testing.T, data []byte, fields int) map[string]string {
	t.Helper()
	out := map[string]string{}
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != fields || parts[0] == "" {
			t.Fatalf("ASSERT_C15_V2_TSV line=%d fields=%d row=%q", i+1, len(parts), line)
		}
		if _, exists := out[parts[0]]; exists {
			t.Fatalf("ASSERT_C15_V2_DUPLICATE_ID %s", parts[0])
		}
		out[parts[0]] = line
	}
	return out
}

func privateB4C15V2SourceDigest(t *testing.T) string {
	t.Helper()
	dir := packageDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00", name, len(data))
		_, _ = h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func writePrivateB4C15V2(t *testing.T, generated map[string][]byte) {
	t.Helper()
	dir := filepath.Join("testdata", "c15-census-v2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"candidates.tsv", "classifications.tsv", "manifest.json"} {
		data := generated[name]
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(name, ".tsv") {
			sidecar := []byte(fmt.Sprintf("%x\n", sha256.Sum256(data)))
			if err := os.WriteFile(filepath.Join(dir, name+".sha256"), sidecar, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func assertPrivateB4C15V2Pinned(t *testing.T, generated map[string][]byte) {
	t.Helper()
	dir := filepath.Join("testdata", "c15-census-v2")
	for _, name := range []string{"candidates.tsv", "classifications.tsv", "manifest.json"} {
		want, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(generated[name], want) {
			t.Fatalf("ASSERT_C15_V2_CLEAN_REGENERATION_%s", name)
		}
		if strings.HasSuffix(name, ".tsv") {
			sum := fmt.Sprintf("%x", sha256.Sum256(want))
			sidecar, err := os.ReadFile(filepath.Join(dir, name+".sha256"))
			if err != nil || strings.TrimSpace(string(sidecar)) != sum {
				t.Fatalf("ASSERT_C15_V2_SIDECAR_%s got=%q err=%v", name, strings.TrimSpace(string(sidecar)), err)
			}
		}
	}
}
