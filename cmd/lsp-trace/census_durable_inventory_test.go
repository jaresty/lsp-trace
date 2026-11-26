//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/sessionruntime"
)

// Historical v2 producer fixture plus public stored-task command compatibility.
// New ordinary entry/operations are exercised by TestCensusOwnedPublicCaller.
func TestCensusHistoricalDurableInventoryProcesses(t *testing.T) {
	for _, mode := range []string{"same-thread", "distinct-declared", "publication-root"} {
		t.Run(mode, func(t *testing.T) { runCensusDefaultDurableInventoryProcesses(t, mode) })
	}
}

// Explicit three-prepared-item qualification. Keep the full publication and
// repeated verified Result assertions: two-item success or first-task issuance
// alone is not this qualification.
func TestCensusThreeItemCapacityQualification(t *testing.T) {
	runCensusDefaultDurableInventoryProcesses(t, "three-item")
}
func TestCensusCompletionTailSurvivesOptionalReads(t *testing.T) {
	runCensusDefaultDurableInventoryProcesses(t, "tail-starvation")
}
func TestCensusCompletionConcurrentReadSubmit(t *testing.T) {
	runCensusDefaultDurableInventoryProcesses(t, "concurrent-first")
}
func runCensusDefaultDurableInventoryProcesses(t *testing.T, mode string) {
	home := folderInventoryTemp(t)
	count := 2
	if mode == "three-item" {
		count = 3
	}
	if mode == "publication-root" {
		if e := os.WriteFile(filepath.Join(home, "Library"), []byte("unavailable per-user ancestry"), 0600); e != nil {
			t.Fatal(e)
		}
		if e := os.Mkdir(filepath.Join(home, "store"), 0700); e != nil {
			t.Fatal(e)
		}
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	call := func(action string, answer []byte, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestCensusDefaultDurableInventoryChild$", "-test.v", "-test.count=1")
		if mode == "publication-root" && action != "start" && action != "operation" {
			args = append(args, "--publication-root", filepath.Join(home, "store"))
		}
		raw, _ := json.Marshal(args)
		cmd.Env = []string{"HOME=" + home, "PATH=", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "DURABLE_INVENTORY_HOME=" + home, "DURABLE_INVENTORY_ACTION=" + action, "DURABLE_INVENTORY_ARGS=" + string(raw)}
		if tmp := os.Getenv("TMPDIR"); tmp != "" {
			cmd.Env = append(cmd.Env, "TMPDIR="+tmp)
		}
		cmd.Env = append(cmd.Env, "DURABLE_INVENTORY_MODE="+mode)
		cmd.Stdin = bytes.NewReader(answer)
		response, e := os.CreateTemp(home, "process-response-")
		if e != nil {
			t.Fatal(e)
		}
		responsePath := response.Name()
		if e = response.Close(); e != nil {
			t.Fatal(e)
		}
		defer os.Remove(responsePath)
		cmd.Env = append(cmd.Env, "DURABLE_INVENTORY_RESPONSE="+responsePath)
		log, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("ASSERT_DEFAULT_DURABLE_PROCESS: FAIL %s %v\n%s", action, e, log)
		}
		t.Logf("%s pid=%d exited: %s", action, cmd.ProcessState.Pid(), log)
		raw, e = os.ReadFile(responsePath)
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	operation := func(answer []byte, argv []string) []byte {
		t.Helper()
		if len(argv) < 5 || argv[0] != "lsp-trace" || argv[1] != "census" || argv[2] != "durable" {
			t.Fatal("invalid returned operation", argv)
		}
		return call("operation", answer, argv[3:]...)
	}
	raw := call("start", nil)
	var task ai.DurableTask
	if json.Unmarshal(raw, &task) != nil || task.Status != "PENDING" || task.Phase != "Describe" {
		t.Fatal("ASSERT_DEFAULT_DURABLE_START: FAIL", string(raw))
	}
	if task.Contract.Version != "lsp-trace.census-task-contract.v1" || len(task.Contract.Operations["read"]) == 0 {
		t.Fatal("self-contained task missing", string(raw))
	}
	if _, e := dw.ParseSuppliedTerminalResponse([]byte(task.Contract.Example), dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}); e != nil {
		t.Fatal("task example grammar", e)
	}
	run := task.RunID
	logCapacity := func(label string) {
		observer := contractProcess{t: t, home: home, mode: mode}
		var envelope struct {
			Content struct {
				Used, Work, Packets, Limit uint64
				CompletionCursor           int
				Completion                 struct {
					Stages          []uint64
					PreparationUsed uint64
				}
			}
		}
		if e := json.Unmarshal(observer.head(run), &envelope); e != nil {
			t.Fatal(e)
		}
		h := envelope.Content
		var protected uint64
		for _, q := range h.Completion.Stages[h.CompletionCursor:] {
			protected += q
		}
		t.Logf("CAPACITY %s prep=%d stages=%v used=%d protected=%d work=%d packets=%d headroom=%d", label, h.Completion.PreparationUsed, h.Completion.Stages, h.Used, protected, h.Work, h.Packets, h.Limit-h.Used-protected)
	}
	logCapacity("start")
	fresh := operation(nil, task.Contract.Operations["current_task"])
	var observed ai.DurableTask
	if json.Unmarshal(fresh, &observed) != nil || observed.TaskID != task.TaskID || observed.Contract.Version != task.Contract.Version {
		t.Fatal("fresh task lookup", string(fresh))
	}
	if mode == "publication-root" {
		argv := observed.Contract.Operations["submit"]
		if len(argv) < 2 || argv[len(argv)-2] != "--publication-root" || argv[len(argv)-1] != filepath.Join(home, "store") {
			t.Fatal("task store context missing", argv)
		}
	}
	described := 0
	lastUsed := task.Used
	lastSeq := task.Sequence
	for task.Status != "PUBLISHED" {
		var input []byte
		for offset := 0; offset < task.Artifacts[0].Bytes; {
			argv := append([]string(nil), task.Contract.Operations["read"]...)
			argv[7] = fmtInt(offset) // documented byte offset in the returned read argv
			raw = operation(nil, argv)
			var page sc.DurableDescribePageV1
			if json.Unmarshal(raw, &page) != nil || page.Offset != offset || len(page.Data) == 0 || len(page.Data) > 32768 {
				t.Fatal("page", string(raw))
			}
			input = append(input, page.Data...)
			offset += len(page.Data)
		}
		var answer []byte
		switch task.Phase {
		case "Describe":
			var evidence struct {
				RunID, LeadID string
				Unit          struct{ Workspace, Body string }
				Accounting    struct{ Windows uint64 }
			}
			if json.Unmarshal(input, &evidence) != nil || evidence.RunID != run || evidence.Accounting.Windows != 3 || evidence.Unit.Body == "" {
				t.Fatal("original binding", string(input))
			}
			if _, e := os.Stat(evidence.Unit.Workspace); !os.IsNotExist(e) {
				t.Fatal("source still present", e)
			}
			// Rehearse returned lifecycle syntax, not semantic/model quality.
			answer = []byte(task.Contract.Example)
			described++
		case "GroupDecision", "Reviewer":
			var catalog cat.NativeCatalogWireV3
			if cat.DecodeNativeV3(input, &catalog) != nil || len(catalog.Evidence) != count {
				t.Fatal("two descriptions missing", string(input))
			}
			d := cat.NativeDecisionV3{Predecessor: catalog.Revision, Actor: task.Actor, Reason: "controlled provisional judgment", Affected: []string{}, Groups: []cat.NativeGroupV3{{ID: "checkout", Label: "checkout proposal", Reason: "first evidence only; remaining membership unresolved", Members: []string{catalog.Evidence[0].ID}}}}
			if task.Phase == "Reviewer" {
				if task.Role != "reviewer" || len(catalog.Groups) != 1 {
					t.Fatal("review ordering")
				}
				d.Affected = []string{"checkout"}
				d.Groups[0].Label = "reviewed checkout proposal"
			}
			answer, _ = json.Marshal(d)
		default:
			t.Fatal("unexpected stage", task.Phase)
		}
		previous := task.TaskID
		recovery := task.Contract.Operations["current_task"]
		if mode == "concurrent-first" && described == 1 {
			var wg sync.WaitGroup
			var outcomes [2][]byte
			wg.Add(2)
			go func() {
				defer wg.Done()
				outcomes[0] = call("probe", nil, "read", run, previous, "input", "0", "32768")
			}()
			go func() { defer wg.Done(); outcomes[1] = call("probe", answer, "submit", run, previous, task.Role) }()
			wg.Wait()
			var submitted struct{ Code int }
			if json.Unmarshal(outcomes[1], &submitted) != nil {
				t.Fatal("concurrent submit output")
			}
			if submitted.Code != 0 {
				// A test-only deliberate retry is allowed only after authoritative
				// observation proves the old task is still unclaimed PENDING.
				var current ai.DurableTask
				if json.Unmarshal(operation(nil, recovery), &current) != nil || current.TaskID != previous || current.Status != "PENDING" {
					t.Fatal("concurrent failure was not an unclaimed task")
				}
				operation(answer, task.Contract.Operations["submit"])
			}
			t.Log("ASSERT_CONCURRENT_READ_SUBMIT_COMPLETION: PASS; predecessor/claim checked")
		} else {
			operation(answer, task.Contract.Operations["submit"]) // discard the successor response
		}
		observer := contractProcess{t: t, home: home, mode: mode}
		beforeObservation := observer.head(run)
		raw = operation(nil, recovery)
		if !bytes.Equal(beforeObservation, observer.head(run)) {
			t.Fatal("returned recovery changed accounting/head")
		}
		if json.Unmarshal(raw, &task) != nil || task.RunID != run || task.PreviousTaskID != previous || task.Sequence != lastSeq+1 || task.Used <= lastUsed || task.Used > 64<<20 || task.Work > 1<<32 || task.Packets > 64 || task.SemanticReservedNS > int64(120*time.Second) {
			t.Fatal("ASSERT_DEFAULT_DURABLE_LEDGER: FAIL", string(raw))
		}
		logCapacity(task.Phase)
		lastSeq, lastUsed = task.Sequence, task.Used
		call("reject", answer, "submit", run, previous, "interpreter")
	}
	if described != count {
		t.Fatal("ASSERT_DEFAULT_DURABLE_DENOMINATOR: FAIL", described)
	}
	// Alter an already-spent quote while repairing the envelope checksum. The
	// immutable completion plan must still fail timed acceptance, before any
	// protected Result transfer. This is not merely a malformed-JSON test.
	{
		storePath := filepath.Join(home, "Library", "Application Support", "lsp-trace", "census", "durable")
		if mode == "publication-root" {
			storePath = filepath.Join(home, "store", "durable")
		}
		path := filepath.Join(storePath, "inventory-run-"+run+".json")
		original, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var env struct {
			Digest  string
			Content json.RawMessage
		}
		var data struct{ Completion struct{ Stages []uint64 } }
		if json.Unmarshal(original, &env) != nil || json.Unmarshal(env.Content, &data) != nil || len(data.Completion.Stages) == 0 {
			t.Fatal("quote head decode")
		}
		from := []byte(`"Stages":[` + strconv.FormatUint(data.Completion.Stages[0], 10))
		to := []byte(`"Stages":[` + strconv.FormatUint(data.Completion.Stages[0]+1, 10))
		env.Content = bytes.Replace(env.Content, from, to, 1)
		env.Digest = cat.NativeDigestV3(env.Content)
		altered, _ := json.Marshal(env)
		if bytes.Equal(original, altered) {
			t.Fatal("quote perturbation missing")
		}
		if e = os.WriteFile(path, altered, 0600); e != nil {
			t.Fatal(e)
		}
		call("reject", nil, "result", run, task.TaskID)
		after, e := os.ReadFile(path)
		if e != nil || !bytes.Equal(after, altered) {
			t.Fatal("invalid quote spent or changed head")
		}
		if e = os.WriteFile(path, original, 0600); e != nil {
			t.Fatal(e)
		}
		t.Log("ASSERT_COMPLETION_QUOTE_BINDING_TAMPER: PASS")
	}
	resultCount := 2
	if mode == "tail-starvation" {
		observer := contractProcess{t: t, home: home, mode: mode}
		denied := false
		for i := 0; i < 65; i++ {
			before := observer.head(run)
			raw = call("probe", nil, "read", run, task.TaskID, "input", "0", "32768")
			var outcome struct{ Code int }
			if json.Unmarshal(raw, &outcome) != nil {
				t.Fatal("probe", string(raw))
			}
			if outcome.Code != 0 {
				if !bytes.Equal(before, observer.head(run)) {
					t.Fatal("denied optional read mutated head")
				}
				denied = true
				break
			}
		}
		if !denied {
			t.Fatal("optional reads never refused")
		}
		resultCount = 1
	}
	for i := 0; i < resultCount; i++ {
		raw = operation(nil, task.Contract.Operations["result"])
		logCapacity("result")
		var report ai.Report
		if json.Unmarshal(raw, &report) != nil || !report.Published || report.Authority != 0 || report.Accepted || report.Completeness != "UNKNOWN" || len(report.Census) != count || len(report.Dispositions) != count || report.SuccessfulDescriptions != count || report.WithMembership != 1 || report.WithoutMembership != count-1 || report.Stages["review"].Successful != 1 || report.PublicationRunReservedBytes == 0 || report.PublicationRunReservedBytes > 64<<20 || report.SemanticReservedNS != task.SemanticReservedNS {
			t.Fatal("ASSERT_DEFAULT_DURABLE_PUBLICATION: FAIL", string(raw))
		}
	}
	if mode == "tail-starvation" {
		observer := contractProcess{t: t, home: home, mode: mode}
		before := observer.head(run)
		operation(nil, task.Contract.Operations["current_task"])
		if !bytes.Equal(before, observer.head(run)) {
			t.Fatal("metadata observation charged")
		}
		t.Log("ASSERT_PROTECTED_RESULT_AFTER_OPTIONAL_DENIAL: PASS")
		return
	}
	// Alter a numeric accounting value without repairing its integrity seal.
	// Syntax, semantic acceptance binding and all ceilings remain otherwise valid.
	storePath := filepath.Join(home, "Library", "Application Support", "lsp-trace", "census", "durable")
	if mode == "publication-root" {
		storePath = filepath.Join(home, "store", "durable")
	}
	headPath := filepath.Join(storePath, "inventory-run-"+run+".json")
	head, e := os.ReadFile(headPath)
	if e != nil {
		t.Fatal(e)
	}
	var envelope struct{ Content struct{ Used uint64 } }
	if json.Unmarshal(head, &envelope) != nil {
		t.Fatal("head")
	}
	corrupt := bytes.Replace(head, []byte(`"Used":`+strconv.FormatUint(envelope.Content.Used, 10)), []byte(`"Used":`+strconv.FormatUint(envelope.Content.Used+1, 10)), 1)
	if bytes.Equal(corrupt, head) {
		t.Fatal("corruption did not target accounting")
	}
	if e = os.WriteFile(headPath, corrupt, 0600); e != nil {
		t.Fatal(e)
	}
	call("reject", nil, "result", run, task.TaskID)
	if e = os.WriteFile(headPath, head, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_DEFAULT_DURABLE_ACCOUNTING_CORRUPTION: PASS")
	manifestPath := filepath.Join(storePath, "inventory-output-"+run, "manifest.json")
	manifest, e := os.ReadFile(manifestPath)
	if e != nil {
		t.Fatal(e)
	}
	var publication ai.PublicationManifestV3
	if json.Unmarshal(manifest, &publication) != nil {
		t.Fatal("manifest")
	}
	publication.CallerBytes++
	corruptManifest, _ := json.Marshal(publication)
	if e = os.WriteFile(manifestPath, corruptManifest, 0600); e != nil {
		t.Fatal(e)
	}
	call("reject", nil, "result", run, task.TaskID)
	if e = os.WriteFile(manifestPath, manifest, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("ASSERT_DEFAULT_DURABLE_MANIFEST_CORRUPTION: PASS")
	if mode == "same-thread" {
		timer := time.NewTimer(max(time.Until(time.Unix(0, task.DeadlineUnixNano))+10*time.Millisecond, 0))
		<-timer.C
		raw = operation(nil, task.Contract.Operations["result"])
		var reopened ai.Report
		if json.Unmarshal(raw, &reopened) != nil || !reopened.Published {
			t.Fatal("expired publication unavailable", string(raw))
		}
		t.Log("ASSERT_DEFAULT_DURABLE_EXPIRED_REOPEN: PASS")
	}
	t.Logf("ASSERT_DEFAULT_DURABLE_MULTI_PROCESS_PUBLICATION: PASS; %d originals, separate sequential review, unresolved denominator conserved, actual publication reopened; Used=%d Work=%d Packets=%d", count, task.Used, task.Work, task.Packets)
}
func fmtInt(n int) string { return strconv.Itoa(n) }
func TestCensusDefaultDurableInventoryChild(t *testing.T) {
	action := os.Getenv("DURABLE_INVENTORY_ACTION")
	if action == "" {
		t.Skip("subprocess only")
	}
	home := os.Getenv("DURABLE_INVENTORY_HOME")
	var out, diagnostic bytes.Buffer
	if action == "start" {
		if os.Getenv("DURABLE_INVENTORY_MODE") == "distinct-declared" {
			hello, e := defaultCensusDeclaration()
			if e != nil {
				t.Fatal(e)
			}
			hello.Interpreter, hello.Reviewer, hello.Subagents = "interpreter", "reviewer", true
			raw, _ := json.Marshal(hello)
			if e = os.Setenv(censusDurableCallerEnv, string(raw)); e != nil {
				t.Fatal(e)
			}
		}
		workspace := folderInventoryTemp(t)
		for name, body := range map[string]string{"go.mod": "module controlled.example/checkout\n\ngo 1.26\n", "checkout.go": "package checkout\nfunc Checkout() int { return price() }\n", "price.go": "package checkout\nfunc price() int { return 7 }\n"} {
			if e := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0600); e != nil {
				t.Fatal(e)
			}
		}
		if os.Getenv("DURABLE_INVENTORY_MODE") == "three-item" {
			if e := os.WriteFile(filepath.Join(workspace, "extra.go"), []byte("package checkout\nfunc ExtraOne() int { return price() }\n"), 0600); e != nil {
				t.Fatal(e)
			}
		}
		if os.Getenv("DURABLE_INVENTORY_MODE") == "three-item" {
			if e := os.WriteFile(filepath.Join(workspace, "extra2.go"), []byte("package checkout\nfunc ExtraTwo() int { return price() }\n"), 0600); e != nil {
				t.Fatal(e)
			}
		}
		scaleFixtureSource(t, workspace)
		wire := newFolderInventoryWire(t, workspace)
		deps := productionCensusRunnerDependencies()
		deps.runOwnedInventory = nil // explicitly retain the historical v2 producer in this fixture, not current ordinary entry
		deps.lookPath = func(string) (string, error) { return "/controlled/checkout-provider", nil }
		deps.newStarter = func() (sessionruntime.Starter, error) { return terminalStarter{child: wire}, nil }
		args := []string{workspace} // Historical producer setup only; public follow-up operations remain current.
		if os.Getenv("DURABLE_INVENTORY_MODE") == "publication-root" {
			args = append(args, "--publication-root", filepath.Join(home, "store"))
			t.Setenv(censusDurableStoreEnv, filepath.Join(home, "missing-compat-store"))
		}
		code := runCensusWithDependencies(args, &out, &diagnostic, deps)
		if code != 0 && os.Getenv("DURABLE_SCALE_PADDING") == "" {
			t.Fatal("ASSERT_DEFAULT_DURABLE_START: FAIL", code, diagnostic.String())
		}
		replies := wire.snapshot()
		if os.Getenv("DURABLE_INVENTORY_MODE") != "three-item" {
			if e := folderInventoryBothOriginals(replies, wire.checkout, wire.price); e != nil {
				t.Fatal(e)
			}
		} else {
			queries := map[string]bool{}
			for _, reply := range replies {
				if reply.method == "textDocument/definition" && string(reply.result) != "[]" {
					queries[string(reply.params)] = true
				}
			}
			if len(queries) < 3 {
				t.Fatalf("three original definition queries missing: %d", len(queries))
			}
		}
		config, e := os.UserConfigDir()
		if e != nil {
			t.Fatal(e)
		}
		acquisitionBase := filepath.Join(config, "lsp-trace", "census")
		if os.Getenv("DURABLE_INVENTORY_MODE") == "publication-root" {
			acquisitionBase = filepath.Join(home, "store")
		}
		if os.Getenv("DURABLE_INVENTORY_MODE") == "three-item" {
			e = folderInventoryThreeOriginals(acquisitionBase, replies)
		} else {
			e = folderInventorySelectedOriginals(acquisitionBase, replies, true)
		}
		if e != nil {
			t.Fatal("original M/P/S", e)
		}
		t.Log("ASSERT_DEFAULT_DURABLE_ORIGINALS_ONCE: PASS")
		if code != 0 {
			out.Reset()
			raw, _ := json.Marshal(struct {
				ScalePretaskRefusal bool
				Code                int
				Diagnostic          string
			}{true, code, diagnostic.String()})
			out.Write(raw)
		}
	} else {
		var args []string
		if json.Unmarshal([]byte(os.Getenv("DURABLE_INVENTORY_ARGS")), &args) != nil {
			t.Fatal("args")
		}
		code := runCensusWithDependencies(append([]string{"durable"}, args...), &out, &diagnostic, productionCensusRunnerDependencies())
		if action == "probe-detail" {
			raw, _ := json.Marshal(struct {
				Code       int
				Output     json.RawMessage
				Diagnostic string
			}{code, append(json.RawMessage(nil), out.Bytes()...), diagnostic.String()})
			out.Reset()
			out.Write(raw)
		} else if action == "probe" {
			out.Reset()
			raw, _ := json.Marshal(struct{ Code int }{code})
			out.Write(raw)
		} else if action == "reject" || action == "reject-detail" {
			if code == 0 {
				t.Fatal("refusal guard accepted request", args)
			}
			out.Reset()
			if action == "reject-detail" {
				out.Write(diagnostic.Bytes())
			} else {
				out.WriteString(`{"refused":true}`)
			}
		} else if code != 0 {
			t.Fatal("durable invocation", code, diagnostic.String())
		}
	}
	responsePath := os.Getenv("DURABLE_INVENTORY_RESPONSE")
	if responsePath == "" {
		responsePath = filepath.Join(home, "response.json")
	}
	if e := os.WriteFile(responsePath, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
}
