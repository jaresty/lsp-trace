package b05qualification

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const historicalBlob = "0bd7501a1fa9307002fd09c10e4bb3843141df0e"
const historicalPath = "../../qualification/retained/b05/archive/qualification-matrix.v2.git-" + historicalBlob + ".json"

func TestRedLegacyCoordinateConflationIsReproduced(t *testing.T) {
	current, err := os.ReadFile("../../qualification/retained/b05/qualification-matrix.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	header := []byte("blob " + strconvItoa(len(current)) + "\x00")
	sum := sha1.Sum(append(header, current...))
	if hex.EncodeToString(sum[:]) == historicalBlob {
		t.Fatal("RED guard no longer reproduces B05 v2 bytes changed conflation")
	}
	t.Log("RED reproduced: mutable current coordinate triggers `B05 v2 bytes changed` against historical pin")
}

func TestHistoricalAndCurrentEvidenceAreReconciledAdditively(t *testing.T) {
	raw, err := os.ReadFile(historicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 21472 {
		t.Fatalf("ASSERT_B05_HISTORICAL_BYTE_LENGTH: %d", len(raw))
	}
	header := []byte("blob " + strconvItoa(len(raw)) + "\x00")
	sum := sha1.Sum(append(header, raw...))
	if hex.EncodeToString(sum[:]) != historicalBlob {
		t.Fatal("ASSERT_B05_HISTORICAL_BLOB_IMMUTABLE")
	}
	gitRaw, err := exec.Command("git", "cat-file", "blob", historicalBlob).Output()
	if err != nil || !bytes.Equal(raw, gitRaw) {
		t.Fatal("ASSERT_B05_HISTORICAL_GIT_CUSTODY")
	}
	selectionRaw, _ := os.ReadFile("../../qualification/retained/b05/current/release-selection.v1.json")
	var selection struct {
		SelectedEvidence   string `json:"selected_evidence"`
		SelectedEvidenceID string `json:"selected_evidence_id"`
		AdmissionCeiling   string `json:"admission_ceiling"`
	}
	if json.Unmarshal(selectionRaw, &selection) != nil {
		t.Fatal("ASSERT_B05_CURRENT_SELECTION_SCHEMA")
	}
	evidenceRaw, err := os.ReadFile("../../" + selection.SelectedEvidence)
	if err != nil {
		t.Fatal(err)
	}
	var evidence map[string]any
	if json.Unmarshal(evidenceRaw, &evidence) != nil {
		t.Fatal("ASSERT_B05_CURRENT_SCHEMA")
	}
	claimed := evidence["evidence_id"].(string)
	delete(evidence, "evidence_id")
	canonical, _ := json.Marshal(evidence)
	digest := sha256.Sum256(canonical)
	if claimed != "sha256:"+hex.EncodeToString(digest[:]) || claimed != selection.SelectedEvidenceID {
		t.Fatal("ASSERT_B05_CURRENT_LOGICAL_ID")
	}
	lineage := evidence["derived_from"].([]any)[0].(map[string]any)["identity"]
	if lineage != "git-blob:"+historicalBlob {
		t.Fatal("ASSERT_B05_CURRENT_LINEAGE_VALID")
	}
	admission := evidence["admission"].(map[string]any)
	if admission["PROGRAM_B_ADMITTED"] != false || admission["ceiling"] != selection.AdmissionCeiling || selection.AdmissionCeiling != "PROGRAM_B_NOT_ADMITTED" {
		t.Fatal("ASSERT_B05_ADMISSION_CEILING_ENFORCED")
	}
}

func TestGeneratorCannotTargetHistoricalArchive(t *testing.T) {
	raw, err := os.ReadFile("../../scripts/build-b05-qualification-evidence-v3.py")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "archive/") {
		t.Fatal("ASSERT_B05_GENERATOR_ARCHIVE_ISOLATION")
	}
	target := filepath.Clean("../../qualification/retained/b05/current/qualification-evidence.v3.json")
	if strings.Contains(target, string(filepath.Separator)+"archive"+string(filepath.Separator)) {
		t.Fatal("ASSERT_B05_GENERATOR_ARCHIVE_ISOLATION")
	}
}

func strconvItoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return string(b[i:])
}
