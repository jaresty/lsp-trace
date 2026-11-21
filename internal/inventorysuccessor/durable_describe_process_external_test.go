//go:build darwin

package inventorysuccessor_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

func TestDurableDescribeProcessBoundary(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	run := func(role string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestDurableDescribeProcessChild$", "-test.v", "-test.count=1")
		cmd.Env = []string{"HOME=" + dir, "PATH=", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "DURABLE_DESCRIBE_ROLE=" + role, "DURABLE_DESCRIBE_STORE=" + dir}
		if tmp := os.Getenv("TMPDIR"); tmp != "" {
			cmd.Env = append(cmd.Env, "TMPDIR="+tmp)
		}
		raw, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("ASSERT_DURABLE_PROCESS_%s: FAIL %v\n%s", role, e, raw)
		}
		t.Logf("process %s exited pid=%d\n%s", role, cmd.ProcessState.Pid(), raw)
		return string(raw)
	}
	a := run("A")
	for _, marker := range []string{"ASSERT_DURABLE_OWNER_ISSUED: PASS", "ASSERT_V3_NATIVE_CLEANUP_BEFORE_EXPOSURE: PASS", "ASSERT_V3_ZERO_EXTRA_ACQUISITION: PASS"} {
		if !strings.Contains(a, marker) {
			t.Fatal("missing producer witness", marker)
		}
	}
	b := run("B")
	for _, marker := range []string{"ASSERT_DURABLE_SOURCE_ABSENT: PASS", "ASSERT_DURABLE_EXACT_PAGES: PASS", "ASSERT_DURABLE_DESCRIBE: PASS", "ASSERT_DURABLE_DUPLICATE_REFUSED: PASS"} {
		if !strings.Contains(b, marker) {
			t.Fatal("missing consumer witness", marker)
		}
	}
	run("C")
	t.Log("ASSERT_DURABLE_A_EXIT_B_SUCCESS_C_REOPEN: PASS")
}

type durableDescribeTestReceipt struct {
	Task      sc.DurableDescribeTaskV1
	Workspace string
	PID       int
	Original  json.RawMessage
}

func TestDurableDescribeProcessChild(t *testing.T) {
	role := os.Getenv("DURABLE_DESCRIBE_ROLE")
	if role == "" {
		t.Skip("subprocess only")
	}
	dir := os.Getenv("DURABLE_DESCRIBE_STORE")
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := root.Close(); e != nil {
			t.Error(e)
		}
	}()
	store := sc.DurableDescribeStoreV1{Root: root}
	ctx := context.Background()
	path := filepath.Join(dir, "test-receipt.json")
	if role == "A" {
		selected := selectedFixtureV3(t)
		runDeadline := time.Now().Add(90 * time.Second)
		budget, e := sc.NewCallerBudgetV3(runDeadline, 64<<20)
		if e != nil {
			t.Fatal(e)
		}
		binding := bindingV3(t, selected, budget)
		binding.Deadline = time.Now().Add(30 * time.Second)
		policy := sc.DurableDescribePolicyV1{ReviewMode: cat.ReviewModeSameThreadV3, Reviewer: binding.Interpreter, RolePrompts: [2]string{"Describe the supplied source without claiming completeness.", "Review separately; same-thread assurance only."}}
		encoded, _ := json.Marshal(selected)
		var dto sc.PrivateSelectedResultV2
		if json.Unmarshal(encoded, &dto) != nil {
			t.Fatal("opaque DTO decode")
		}
		if _, e := store.Issue(ctx, dto, binding, runDeadline, policy); e == nil {
			t.Fatal("ASSERT_DURABLE_DTO_REFUSED: FAIL")
		}
		t.Log("ASSERT_DURABLE_DTO_REFUSED: PASS")
		task, e := store.Issue(ctx, selected, binding, runDeadline, policy)
		if e != nil {
			t.Fatal("ASSERT_DURABLE_OWNER_ISSUED: FAIL", e)
		}
		_, unit, request, ok := selected.Selected()
		if !ok {
			t.Fatal("original unavailable")
		}
		original, _ := json.Marshal(struct {
			Unit    any
			Request any
		}{unit, request})
		receipt, _ := json.Marshal(durableDescribeTestReceipt{task, unit.Workspace, os.Getpid(), original})
		if e = os.WriteFile(path, receipt, 0600); e != nil {
			t.Fatal(e)
		}
		freshBudget, e := sc.NewCallerBudgetV3(runDeadline, 64<<20)
		if e != nil {
			t.Fatal(e)
		}
		repeatBinding := binding
		repeatBinding.Budget = freshBudget
		if _, e := store.Issue(ctx, selected, repeatBinding, runDeadline, policy); e == nil {
			t.Fatal("ASSERT_DURABLE_ISSUANCE_ONCE: FAIL")
		}
		t.Log("ASSERT_DURABLE_OWNER_ISSUED: PASS; ASSERT_DURABLE_ISSUANCE_ONCE: PASS")
		return
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var receipt durableDescribeTestReceipt
	if json.Unmarshal(raw, &receipt) != nil || receipt.PID == os.Getpid() {
		t.Fatal("receipt")
	}
	if _, e = os.Stat(receipt.Workspace); !os.IsNotExist(e) {
		t.Fatal("ASSERT_DURABLE_SOURCE_ABSENT: FAIL", e)
	}
	t.Log("ASSERT_DURABLE_SOURCE_ABSENT: PASS")
	task := receipt.Task
	if role == "C" {
		raw, e := store.Result(ctx, task.RunID, task.TaskID)
		if e != nil || len(raw) == 0 {
			t.Fatal("ASSERT_DURABLE_RESULT_REOPEN: FAIL", e)
		}
		t.Log("ASSERT_DURABLE_RESULT_REOPEN: PASS")
		return
	}
	if role == "claim-and-wait" {
		if e := store.Submit(ctx, task.RunID, task.TaskID, "interpreter", []byte("bad\n")); e == nil {
			t.Fatal("malformed accepted")
		}
		_, _ = os.Stdout.WriteString("CLAIM_SPENT_READY\n")
		var barrier [1]byte
		_, _ = os.Stdin.Read(barrier[:])
		return
	}
	if role == "spent" {
		status, e := store.Task(ctx, task.RunID, task.TaskID)
		if e != nil || status.Status != "UNCERTAIN_SPENT" {
			t.Fatal("ASSERT_DURABLE_KILLED_CLAIM_SPENT: FAIL", e, status.Status)
		}
		if e := store.Submit(ctx, task.RunID, task.TaskID, "interpreter", semanticAdapterV3()); e == nil {
			t.Fatal("ASSERT_DURABLE_KILLED_CLAIM_SPENT: FAIL replay")
		}
		t.Log("ASSERT_DURABLE_KILLED_CLAIM_SPENT: PASS")
		return
	}
	if role == "compete" {
		var barrier [1]byte
		if _, e := os.Stdin.Read(barrier[:]); e != nil {
			t.Fatal(e)
		}
		if e := store.Submit(ctx, task.RunID, task.TaskID, "interpreter", semanticAdapterV3()); e == nil {
			t.Log("ASSERT_DURABLE_PROCESS_SUBMIT: WON")
		} else {
			t.Log("ASSERT_DURABLE_PROCESS_SUBMIT: REFUSED")
		}
		return
	}
	if role != "B" {
		t.Fatal("role")
	}
	if _, e := store.Task(ctx, strings.Repeat("0", 64), task.TaskID); e == nil {
		t.Fatal("foreign run accepted")
	}
	if _, e := store.Task(ctx, task.RunID, strings.Repeat("0", 64)); e == nil {
		t.Fatal("foreign task accepted")
	}
	if _, e := store.Read(ctx, task.RunID, task.TaskID, "foreign", 0, 1); e == nil {
		t.Fatal("foreign artifact accepted")
	}
	if e := store.Submit(ctx, task.RunID, task.TaskID, "reviewer", semanticAdapterV3()); e == nil {
		t.Fatal("foreign role accepted")
	}
	// Perturb the actual owner-issued store, not a private-state constructor.
	entries, e := os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "prepared-") && !strings.HasPrefix(entry.Name(), "describe-") {
			continue
		}
		name := filepath.Join(dir, entry.Name())
		original, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		for _, bad := range [][]byte{original[:len(original)/2], bytes.Replace(original, []byte(`"Schema"`), []byte(`"Schemb"`), 1)} {
			if e = os.WriteFile(name, bad, 0600); e != nil {
				t.Fatal(e)
			}
			_, e = store.Read(ctx, task.RunID, task.TaskID, "prepared", 0, 4096)
			if restore := os.WriteFile(name, original, 0600); restore != nil {
				t.Fatal(restore)
			}
			if e == nil {
				t.Fatal("ASSERT_DURABLE_CORRUPT_REFUSED: FAIL")
			}
			t.Log("ASSERT_DURABLE_CORRUPT_REFUSED: PASS", entry.Name())
		}
	}
	// These isolated filesystem copies are test-only state perturbations under
	// the accepted same-user trust assumption, not a production import API.
	faultDir := filepath.Join(dir, "malformed-control")
	if e = os.Mkdir(faultDir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, e := os.ReadFile(filepath.Join(dir, entry.Name()))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(faultDir, entry.Name()), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	faultRoot, e := publication.OpenRoot(faultDir)
	if e != nil {
		t.Fatal(e)
	}
	fault := sc.DurableDescribeStoreV1{Root: faultRoot}
	if e = fault.Submit(ctx, task.RunID, task.TaskID, "interpreter", []byte("bad\n")); e == nil {
		t.Fatal("malformed accepted")
	}
	if e = fault.Submit(ctx, task.RunID, task.TaskID, "interpreter", semanticAdapterV3()); e == nil {
		t.Fatal("malformed task replayed")
	}
	spent, e := fault.Task(ctx, task.RunID, task.TaskID)
	if e != nil || spent.Status != "UNCERTAIN_SPENT" {
		t.Fatal("ASSERT_DURABLE_MALFORMED_SPENT: FAIL", e, spent.Status)
	}
	if e = faultRoot.Close(); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_DURABLE_MALFORMED_SPENT: PASS")
	for _, bounds := range [][2]int{{-1, 1}, {0, 0}, {0, sc.DurableDescribePageLimitV1 + 1}, {task.Artifacts[0].Bytes, 1}} {
		if _, e := store.Read(ctx, task.RunID, task.TaskID, "prepared", bounds[0], bounds[1]); e == nil {
			t.Fatal("invalid page bounds accepted", bounds)
		}
	}
	limitDir := filepath.Join(dir, "packet-limit-control")
	if e = os.Mkdir(limitDir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, e := os.ReadFile(filepath.Join(dir, entry.Name()))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(limitDir, entry.Name()), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	limitRoot, e := publication.OpenRoot(limitDir)
	if e != nil {
		t.Fatal(e)
	}
	limited := sc.DurableDescribeStoreV1{Root: limitRoot}
	reads := 0
	for ; reads < 65; reads++ {
		if _, e = limited.Read(ctx, task.RunID, task.TaskID, "prepared", 0, 1); e != nil {
			break
		}
	}
	if reads == 0 || reads >= 65 {
		t.Fatal("packet limit not enforced", reads)
	}
	if _, e = limited.Read(ctx, task.RunID, task.TaskID, "prepared", 0, 1); e == nil {
		t.Fatal("packet limit reset")
	}
	if e = limitRoot.Close(); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_DURABLE_PACKET_AND_PAGE_BOUNDS: PASS; reads before exhaustion=", reads)
	var content []byte
	for offset := 0; offset < task.Artifacts[0].Bytes; {
		page, e := store.Read(ctx, task.RunID, task.TaskID, "prepared", offset, 4096)
		if e != nil {
			t.Fatal("ASSERT_DURABLE_EXACT_PAGES: FAIL", e)
		}
		if page.Offset != offset || len(page.Data) == 0 || len(page.Data) > 4096 {
			t.Fatal("page bound")
		}
		content = append(content, page.Data...)
		offset += len(page.Data)
	}
	var p struct {
		Unit    json.RawMessage
		Request json.RawMessage
	}
	if json.Unmarshal(content, &p) != nil {
		t.Fatal("prepared decode")
	}
	exact, _ := json.Marshal(struct {
		Unit    json.RawMessage
		Request json.RawMessage
	}{p.Unit, p.Request})
	if !bytes.Equal(exact, receipt.Original) {
		t.Fatal("ASSERT_DURABLE_EXACT_PAGES: FAIL exact original mismatch")
	}
	t.Log("ASSERT_DURABLE_EXACT_PAGES: PASS")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- store.Submit(ctx, task.RunID, task.TaskID, "interpreter", semanticAdapterV3())
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("ASSERT_DURABLE_CONCURRENT_ONCE: FAIL", success)
	}
	t.Log("ASSERT_DURABLE_CONCURRENT_ONCE: PASS")
	result, e := store.Result(ctx, task.RunID, task.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	var evidence sc.SelectedEvidenceV3
	if json.Unmarshal(result, &evidence) != nil {
		t.Fatal("description decode")
	}
	resultOriginal, _ := json.Marshal(struct {
		Unit    any
		Request any
	}{evidence.Unit, evidence.Request})
	if !bytes.Equal(resultOriginal, receipt.Original) {
		t.Fatal("ASSERT_DURABLE_DESCRIBE: FAIL changed source/request")
	}
	t.Log("ASSERT_DURABLE_DESCRIBE: PASS")
	if e := store.Submit(ctx, task.RunID, task.TaskID, "interpreter", semanticAdapterV3()); e == nil {
		t.Fatal("ASSERT_DURABLE_DUPLICATE_REFUSED: FAIL")
	}
	t.Log("ASSERT_DURABLE_DUPLICATE_REFUSED: PASS")
	confirmation := filepath.Join(dir, "confirmed-"+task.RunID+".json")
	saved, e := os.ReadFile(confirmation)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(confirmation); e != nil {
		t.Fatal(e)
	}
	if _, e = store.Result(ctx, task.RunID, task.TaskID); e == nil {
		t.Fatal("uncertain final result accepted")
	}
	uncertain, e := store.Task(ctx, task.RunID, task.TaskID)
	if e != nil || uncertain.Status != "UNCERTAIN_SPENT" {
		t.Fatal("uncertain final status", e)
	}
	if e = os.WriteFile(confirmation, saved, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_DURABLE_UNCONFIRMED_FINAL_REFUSED: PASS")
}
