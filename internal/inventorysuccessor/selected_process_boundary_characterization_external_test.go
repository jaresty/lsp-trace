//go:build darwin || linux

package inventorysuccessor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/describerequest"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	tp "lsp-trace/internal/targetpacket"
)

// This is deliberately NOT a retained prepared evidence export contract. These
// public snapshots carry data, never original-owner authority. Keep this test's
// success distinct from the still-unimplemented A-exit/B-Describe-success goal.
type selectedProcessSnapshot struct {
	Schema  string
	PID     int
	Row     sc.PrivateSelectedEndpointV2
	Unit    tp.PrivateSourceUnit
	Request describerequest.Record
	Opaque  json.RawMessage
}

func TestSelectedProcessBoundaryCharacterization(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("existing genuine SELF acquisition fixture is Darwin-only")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	run := func(role, digest string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestSelectedProcessBoundaryChild$", "-test.v", "-test.count=1")
		// B receives no provider configuration, source capability or manager. An
		// empty PATH is a convenience-runtime barrier, not an OS security sandbox.
		cmd.Env = []string{"HOME=" + dir, "PATH=", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off",
			"SELECTED_BOUNDARY_ROLE=" + role, "SELECTED_BOUNDARY_PATH=" + path, "SELECTED_BOUNDARY_DIGEST=" + digest}
		if tmp := os.Getenv("TMPDIR"); tmp != "" {
			cmd.Env = append(cmd.Env, "TMPDIR="+tmp)
		}
		out, err := cmd.CombinedOutput() // returns only after this child exits
		if err != nil {
			t.Fatalf("ASSERT_BOUNDARY_CHILD_%s_EXIT: %v\n%s", role, err, out)
		}
		if len(out) > 64<<10 {
			t.Fatal("ASSERT_BOUNDARY_OUTPUT_BOUND")
		}
		t.Logf("child %s exited pid=%d\n%s", role, cmd.ProcessState.Pid(), out)
		return string(out)
	}
	a := run("A", "") // exactly one genuine selected preparation in this test
	for _, witness := range []string{"ASSERT_BOUNDARY_A_IN_PROCESS_DESCRIBE: PASS", "ASSERT_V3_NATIVE_CLEANUP_BEFORE_EXPOSURE: PASS", "ASSERT_V3_ZERO_EXTRA_ACQUISITION: PASS"} {
		if !strings.Contains(a, witness) {
			t.Fatalf("ASSERT_BOUNDARY_A_WITNESS_MISSING: %s", witness)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 8<<20 {
		t.Fatal("ASSERT_BOUNDARY_SNAPSHOT", err)
	}
	var snapshot selectedProcessSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	// A's t.TempDir cleanup is complete before B starts. Metadata-only check;
	// neither the parent nor B rereads any captured source bytes.
	if _, err := os.Stat(snapshot.Unit.Workspace); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_BOUNDARY_SOURCE_REMOVED: %v", err)
	}
	t.Log("ASSERT_BOUNDARY_A_EXIT_BEFORE_B_SOURCE_ABSENT: PASS")
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	b := run("B", digest)
	for _, witness := range []string{"ASSERT_BOUNDARY_B_EXACT_DATA: PASS", "ASSERT_BOUNDARY_B_DTO_REFUSED: PASS", "CHARACTERIZATION_ONLY; CROSS_PROCESS_SUCCESS_NOT_IMPLEMENTED"} {
		if !strings.Contains(b, witness) {
			t.Fatalf("ASSERT_BOUNDARY_B_WITNESS_MISSING: %s", witness)
		}
	}
	t.Log("CHARACTERIZATION_ONLY; CROSS_PROCESS_SUCCESS_NOT_IMPLEMENTED")
}

func TestSelectedProcessBoundaryChild(t *testing.T) {
	role := os.Getenv("SELECTED_BOUNDARY_ROLE")
	if role == "" {
		t.Skip("subprocess-only characterization helper")
	}
	path := os.Getenv("SELECTED_BOUNDARY_PATH")
	ctx := context.Background()
	switch role {
	case "A":
		r := selectedFixtureV3(t)
		budget, err := sc.NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
		if err != nil {
			t.Fatal(err)
		}
		attempt, err := sc.PrepareSelectedDescribeV3(ctx, r, bindingV3(t, r, budget))
		if err != nil {
			t.Fatal("ASSERT_BOUNDARY_A_IN_PROCESS_PREPARE", err)
		}
		bound, err := sc.CompleteSelectedDescribeV3(ctx, attempt, semanticAdapterV3())
		if err != nil || !bound.Available() {
			t.Fatal("ASSERT_BOUNDARY_A_IN_PROCESS_DESCRIBE", err)
		}
		row, unit, request, ok := r.Selected()
		if !ok {
			t.Fatal("ASSERT_BOUNDARY_A_SELECTED")
		}
		evidence, err := bound.Evidence(ctx)
		if err != nil || evidence.Unit != unit || evidence.Request != request || evidence.Selected.Intent != row.Intent {
			t.Fatal("ASSERT_BOUNDARY_A_EXACT_LINEAGE", err)
		}
		if _, err := sc.CompleteSelectedDescribeV3(ctx, attempt, semanticAdapterV3()); err == nil {
			t.Fatal("ASSERT_BOUNDARY_A_COMPLETION_SPENT")
		}
		opaque, err := json.Marshal(r)
		if err != nil || !bytes.Equal(opaque, []byte("{}")) {
			t.Fatal("ASSERT_BOUNDARY_OPAQUE_NOT_SERIALIZED", string(opaque), err)
		}
		raw, err := json.Marshal(selectedProcessSnapshot{"test-only-selected-data-not-admission.v1", os.Getpid(), row, unit, request, opaque})
		if err != nil || len(raw) > 8<<20 {
			t.Fatal("ASSERT_BOUNDARY_A_ENCODING", err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("ASSERT_BOUNDARY_A_IN_PROCESS_DESCRIBE: PASS; ASSERT_BOUNDARY_A_EXACT_LINEAGE: PASS; ASSERT_BOUNDARY_A_COMPLETION_SPENT: PASS")
	case "B":
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) == 0 || len(raw) > 8<<20 || fmt.Sprintf("%x", sha256.Sum256(raw)) != os.Getenv("SELECTED_BOUNDARY_DIGEST") {
			t.Fatal("ASSERT_BOUNDARY_B_DATA_TRANSPORT", err)
		}
		var snapshot selectedProcessSnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Schema != "test-only-selected-data-not-admission.v1" || snapshot.PID == os.Getpid() || snapshot.PID <= 0 || describerequest.ValidateRecord(snapshot.Request) != nil || snapshot.Unit.ID != snapshot.Request.Envelope.AdmissionID || snapshot.Unit.Body == "" {
			t.Fatal("ASSERT_BOUNDARY_B_EXACT_DATA")
		}
		encoded, err := json.Marshal(snapshot)
		if err != nil || !bytes.Equal(encoded, raw) {
			t.Fatal("ASSERT_BOUNDARY_B_EXACT_ROUNDTRIP", err)
		}
		if _, err := os.Stat(snapshot.Unit.Workspace); !os.IsNotExist(err) {
			t.Fatal("ASSERT_BOUNDARY_B_SOURCE_ABSENT", err)
		}
		// Both direct opaque serialization and the genuine data snapshot remain
		// data-only. No private-state constructor or acquisition API is invoked.
		for _, data := range [][]byte{snapshot.Opaque, raw} {
			var decoded sc.PrivateSelectedResultV2
			if err := json.Unmarshal(data, &decoded); err != nil || decoded.Available() {
				t.Fatal("ASSERT_BOUNDARY_B_NO_RESTORED_OWNER", err)
			}
			budget, err := sc.NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
			if err != nil {
				t.Fatal(err)
			}
			binding := bindingV3(t, decoded, budget)
			// Use the genuine request pin, not an invalid-pin refusal surrogate.
			binding.Pins.PromptSHA256 = "sha256:" + snapshot.Request.Envelope.InputSHA256
			if _, err := sc.PrepareSelectedDescribeV3(ctx, decoded, binding); err != sc.ErrSelectedConsumerV3 {
				t.Fatal("ASSERT_BOUNDARY_B_DTO_REFUSED", err)
			}
		}
		t.Log("ASSERT_BOUNDARY_B_EXACT_DATA: PASS; ASSERT_BOUNDARY_B_DTO_REFUSED: PASS")
		t.Log("CHARACTERIZATION_ONLY; CROSS_PROCESS_SUCCESS_NOT_IMPLEMENTED")
	default:
		t.Fatal("unknown subprocess role")
	}
}
