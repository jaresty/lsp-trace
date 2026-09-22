package censuscontinuation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type retainedV6Reference struct {
	ProgramC struct {
		SHA256       string `json:"sha256"`
		ByteLength   int    `json:"byte_length"`
		Nominations  int    `json:"nominations"`
		Unresolved   int    `json:"unresolved"`
		Candidates   int    `json:"candidates"`
		Constituents int    `json:"constituents"`
	} `json:"program_c"`
	Contract struct {
		SHA256      string `json:"sha256"`
		ByteLength  int    `json:"byte_length"`
		MaxReceipts int    `json:"max_receipts"`
		MaxBindings int    `json:"max_bindings"`
		MaxSource   int    `json:"max_source_bytes"`
		MaxTotal    int    `json:"max_total_source_bytes"`
		MaxWork     int    `json:"max_work"`
	} `json:"contract"`
	FailedCheckpoint struct {
		CheckpointID string `json:"checkpoint_id"`
		ObjectSHA256 string `json:"object_sha256"`
	} `json:"failed_checkpoint"`
	Expected struct {
		Category string `json:"category"`
		Field    string `json:"failed_field"`
		Observed int    `json:"observed"`
		Limit    int    `json:"limit"`
	} `json:"expected_first_rejection"`
}

func TestRetainedV6FailureReferenceAndOptionalImmutableReplay(t *testing.T) {
	fixtureRaw, err := os.ReadFile("testdata/retained_v6_64mib_failure.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref retainedV6Reference
	if err := json.Unmarshal(fixtureRaw, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.ProgramC.Nominations != 135 || ref.Expected.Field != "bindings" || ref.Expected.Observed != ref.ProgramC.Nominations || ref.Expected.Limit != 100 || ref.Contract.MaxBindings != 100 || ref.Contract.MaxReceipts != 100 || ref.Contract.MaxSource != 1<<20 || ref.Contract.MaxTotal != 8<<20 || ref.Contract.MaxWork != 10000 {
		t.Fatalf("ASSERT_RETAINED_V6_REFERENCE_ACCOUNTING: %+v", ref)
	}
	root := os.Getenv("LSP_TRACE_RETAINED_V6_FIXTURE_ROOT")
	if root == "" {
		t.Skip("set LSP_TRACE_RETAINED_V6_FIXTURE_ROOT for immutable retained replay")
	}
	objects := filepath.Join(root, "continuations", "objects", "sha256")
	programRaw := mustRetainedObject(t, objects, ref.ProgramC.SHA256, ref.ProgramC.ByteLength)
	contractRaw := mustRetainedObject(t, objects, ref.Contract.SHA256, ref.Contract.ByteLength)
	checkpointRaw := mustRetainedObject(t, objects, ref.FailedCheckpoint.ObjectSHA256, -1)
	contract, err := ParseContinuationContract(contractRaw)
	if err != nil {
		t.Fatal(err)
	}
	limits := contract.CaptureLimits()
	if limits.MaxBindings != ref.Expected.Limit || limits.MaxReceipts != ref.Contract.MaxReceipts || limits.MaxSourceBytes != ref.Contract.MaxSource || limits.MaxTotalSourceBytes != ref.Contract.MaxTotal || limits.MaxWork != ref.Contract.MaxWork {
		t.Fatalf("ASSERT_RETAINED_CONTRACT_LIMITS: %+v", limits)
	}
	var program struct {
		Candidates      []json.RawMessage                                   `json:"candidates"`
		Representatives struct{ Nominations, Unresolved []json.RawMessage } `json:"representatives"`
		SourceBinding   struct{ Constituents []json.RawMessage }            `json:"source_binding"`
	}
	if err := json.Unmarshal(programRaw, &program); err != nil {
		t.Fatal(err)
	}
	if len(program.Representatives.Nominations) != ref.ProgramC.Nominations || len(program.Representatives.Unresolved) != ref.ProgramC.Unresolved || len(program.Candidates) != ref.ProgramC.Candidates || len(program.SourceBinding.Constituents) != ref.ProgramC.Constituents {
		t.Fatalf("ASSERT_RETAINED_PROGRAM_C_CARDINALITY: nominations=%d unresolved=%d candidates=%d constituents=%d", len(program.Representatives.Nominations), len(program.Representatives.Unresolved), len(program.Candidates), len(program.SourceBinding.Constituents))
	}
	checkpoint, err := ParseCheckpoint(checkpointRaw)
	if err != nil || checkpoint.ID() != ref.FailedCheckpoint.CheckpointID || checkpoint.Stage() != StageProgramCComputed || checkpoint.Status() != StatusFailedCapture {
		t.Fatalf("ASSERT_RETAINED_FAILED_CHECKPOINT: checkpoint=%s err=%v", checkpoint.ID(), err)
	}
}

func mustRetainedObject(t *testing.T, root, id string, length int) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, id[len("sha256:"):]))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != id || (length >= 0 && len(raw) != length) {
		t.Fatalf("ASSERT_RETAINED_OBJECT_IDENTITY: got=%s bytes=%d want=%s bytes=%d", got, len(raw), id, length)
	}
	return raw
}
