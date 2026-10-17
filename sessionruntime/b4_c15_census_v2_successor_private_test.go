package sessionruntime

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	privateB4C15V2BaseHEAD          = "374e7c7ba31363ebd19800f8d027cd22a290a464"
	privateB4C15V2BaseCount         = 3805
	privateB4C15V2FinalCount        = 3847
	privateB4C15V2AddedCount        = 42
	privateB4C15V2MappingCount      = 44
	privateB4C15V2CandidateSHA256   = "536d11dcfa927d1ce5db24fbd8a956a5170a8f762ee53d8daa3664288172afb2"
	privateB4C15V2DispositionSHA256 = "494d2f0045db3e34b7c03e2a959aaf21e2fee203bfd3768097df7930ddf6b730"
	privateB4C15V2ManifestSHA256    = "548e6af3625b248ab7e8a99b29ac24bbb3f674b0ffdb14365d4dfaaf49c89daf"
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

func TestPrivateB4C15V2HistoricalSuccessorIntegrity(t *testing.T) {
	candidates := readPrivateB4C15V2Pinned(t, "candidates.tsv", privateB4C15V2CandidateSHA256, true)
	classifications := readPrivateB4C15V2Pinned(t, "classifications.tsv", privateB4C15V2DispositionSHA256, true)
	manifestBytes := readPrivateB4C15V2Pinned(t, "manifest.json", privateB4C15V2ManifestSHA256, false)

	var manifest privateB4C15V2Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != "lsp-trace/c15-census-successor-v2" || manifest.BaseHEAD != privateB4C15V2BaseHEAD ||
		manifest.Generator != "sessionruntime/b4_c15_census_v2_successor_private_test.go:TestPrivateB4C15V2SuccessorCatalog" ||
		manifest.CanonicalCommand != "C15_V2_REGENERATE=1 go test ./sessionruntime -run '^TestPrivateB4C15V2SuccessorCatalog$' -count=1" {
		t.Fatalf("ASSERT_C15_V2_HISTORICAL_PROVENANCE %+v", manifest)
	}
	if manifest.GeneratorInputs["active-production-go-source"] != "4db5f57f186415ac60e908c45c3f49c239d6afdc1bde5298fadc8b3ad01e5931" ||
		manifest.GeneratorInputs["c15-census-v1/operation-mapping.json"] != privateB4C15MappingSHA256 ||
		manifest.PredecessorVerification["c15-census-v1/candidates.tsv"] != privateB4C15CandidateSHA256 ||
		manifest.PredecessorVerification["c15-census-v1/classifications.tsv"] != privateB4C15DispositionSHA256 {
		t.Fatalf("ASSERT_C15_V2_HISTORICAL_INPUTS %+v", manifest)
	}
	if manifest.BaseCount != privateB4C15V2BaseCount || manifest.FinalCount != privateB4C15V2FinalCount ||
		manifest.AddedExcluded != privateB4C15V2AddedCount || manifest.Removed != 0 || manifest.Reclassified != 0 ||
		manifest.UnchangedMapping != privateB4C15V2MappingCount || manifest.ManagerPropagationClaim {
		t.Fatalf("ASSERT_C15_V2_HISTORICAL_MEANING %+v", manifest)
	}

	v1Candidates := mustReadC15File(t, "candidates.tsv")
	v1Classifications := mustReadC15File(t, "classifications.tsv")
	assertPrivateB4C15V2Delta(t, v1Candidates, v1Classifications, candidates, classifications)
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

func readPrivateB4C15V2Pinned(t *testing.T, name, wantHash string, hasSidecar bool) []byte {
	t.Helper()
	path := filepath.Join("testdata", "c15-census-v2", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != wantHash {
		t.Fatalf("ASSERT_C15_V2_REPRODUCIBILITY_HASH_%s got=%s want=%s", name, got, wantHash)
	}
	if hasSidecar {
		sidecar, err := os.ReadFile(path + ".sha256")
		if err != nil || strings.TrimSpace(string(sidecar)) != got {
			t.Fatalf("ASSERT_C15_V2_SIDECAR_%s got=%q err=%v", name, strings.TrimSpace(string(sidecar)), err)
		}
	}
	return data
}
