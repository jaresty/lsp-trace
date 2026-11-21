//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

func TestCensusDurableNormalFolderProcesses(t *testing.T) {
	for _, mode := range []string{"same-thread", "distinct-declared", "expired", "held-input"} {
		t.Run(mode, func(t *testing.T) {
			exe, e := os.Executable()
			if e != nil {
				t.Fatal(e)
			}
			dir := folderInventoryTemp(t)
			for _, name := range []string{"application", "acquisition"} {
				if e := os.Mkdir(filepath.Join(dir, name), 0700); e != nil {
					t.Fatal(e)
				}
			}
			call := func(action string, args ...string) []byte {
				t.Helper()
				timeout := 30 * time.Second
				if action == "held" {
					timeout = 40 * time.Second
				}
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				command := exec.CommandContext(ctx, exe, "-test.run=^TestCensusDurableNormalFolderChild$", "-test.v", "-test.count=1")
				argJSON, _ := json.Marshal(args)
				command.Stdin = bytes.NewReader(dw.EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"participates in controlled checkout","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"supplies a checkout relation","consumer_relative":false},"boundary_contribution":"two-file fixture","limitations":["scripted fixture, not live model or application completeness"]}`))
				command.Env = []string{"HOME=" + dir, "PATH=", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "DURABLE_FOLDER_DIR=" + dir, "DURABLE_FOLDER_ACTION=" + action, "DURABLE_FOLDER_ARGS=" + string(argJSON), "DURABLE_FOLDER_MODE=" + mode}
				if tmp := os.Getenv("TMPDIR"); tmp != "" {
					command.Env = append(command.Env, "TMPDIR="+tmp)
				}
				var raw []byte
				if action == "held" {
					command.Stdin = nil
					pipe, pipeErr := command.StdinPipe()
					if pipeErr != nil {
						t.Fatal(pipeErr)
					}
					var captured bytes.Buffer
					command.Stdout, command.Stderr = &captured, &captured
					if e = command.Start(); e != nil {
						t.Fatal(e)
					}
					_, writeErr := pipe.Write([]byte(`{"partial":`))
					if writeErr != nil {
						t.Fatal(writeErr)
					}
					// Keep stdin open through child exit: the stored task deadline,
					// not parent EOF, must cancel the CLI read.
					e = command.Wait()
					_ = pipe.Close()
					raw = captured.Bytes()
				} else {
					raw, e = command.CombinedOutput()
				}
				if e != nil {
					t.Fatalf("ASSERT_NORMAL_DURABLE_%s: FAIL %v\n%s", action, e, raw)
				}
				t.Logf("%s process exited pid=%d\n%s", action, command.ProcessState.Pid(), raw)
				result, e := os.ReadFile(filepath.Join(dir, "response.json"))
				if e != nil {
					t.Fatal(e)
				}
				return result
			}
			raw := call("start")
			var task sc.DurableDescribeTaskV1
			if json.Unmarshal(raw, &task) != nil || task.Status != "PENDING" || task.Authority != 0 || task.Accepted {
				t.Fatal("pending task", string(raw))
			}
			if mode == "held-input" {
				raw = call("held", task.RunID, task.TaskID, "interpreter")
				var status sc.DurableDescribeTaskV1
				if json.Unmarshal(raw, &status) != nil || status.Status != "EXPIRED" {
					t.Fatal("ASSERT_NORMAL_DURABLE_HELD_INPUT: FAIL", string(raw))
				}
				t.Log("ASSERT_NORMAL_DURABLE_HELD_INPUT: PASS")
				return
			}
			if mode == "expired" {
				time.Sleep(31 * time.Second)
				call("expired", task.RunID, task.TaskID)
				t.Log("ASSERT_NORMAL_DURABLE_EXPIRED_NO_REPLAY: PASS")
				return
			}
			if mode == "same-thread" && task.Interpreter != task.Reviewer || mode == "distinct-declared" && task.Interpreter == task.Reviewer || !strings.Contains(task.Assurance, "REVIEW_NOT_PERFORMED") && !strings.Contains(task.Assurance, "REVIEW_REQUIRED_NOT_PERFORMED") {
				t.Fatal("declared assurance binding", task)
			}
			if mode == "same-thread" {
				time.Sleep(16 * time.Second)
				t.Log("ASSERT_NORMAL_DURABLE_NATIVE_WINDOW_ELAPSED: PASS (real wall time; semantic deadline unchanged)")
			}
			raw = call("task", task.RunID, task.TaskID)
			var status sc.DurableDescribeTaskV1
			if json.Unmarshal(raw, &status) != nil || status.Status != "PENDING" {
				t.Fatal("status")
			}
			for _, artifact := range []string{"source", "request"} {
				raw = call("read", task.RunID, task.TaskID, artifact, "0", "4096")
				var page sc.DurableDescribePageV1
				if json.Unmarshal(raw, &page) != nil || page.ArtifactID != artifact || len(page.Data) == 0 || len(page.Data) > 4096 {
					t.Fatal("individual artifact", artifact)
				}
			}
			var all []byte
			for offset := 0; offset < task.Artifacts[0].Bytes; {
				raw = call("read", task.RunID, task.TaskID, "prepared", strconv.Itoa(offset), "4096")
				var page sc.DurableDescribePageV1
				if json.Unmarshal(raw, &page) != nil || page.Offset != offset || len(page.Data) == 0 || len(page.Data) > 4096 {
					t.Fatal("page")
				}
				all = append(all, page.Data...)
				offset += len(page.Data)
			}
			var prepared struct {
				Unit        struct{ Body, Workspace string }
				Request     json.RawMessage
				RunDeadline int64
				Accounting  struct {
					NativeDeadline  int64
					Windows         uint64
					URI             string
					Line, Character uint32
				}
			}
			if json.Unmarshal(all, &prepared) != nil || !strings.Contains(prepared.Unit.Body, "price()") {
				t.Fatal("original source body", string(all))
			}
			if prepared.Accounting.Windows != 3 || prepared.Accounting.Line != 1 || prepared.Accounting.Character != 29 || !strings.HasSuffix(prepared.Accounting.URI, "/checkout.go") || prepared.RunDeadline-prepared.Accounting.NativeDeadline > int64(165*time.Second) {
				t.Fatal("original query/accounting or application deadline renewed", prepared.Accounting)
			}
			t.Log("ASSERT_NORMAL_DURABLE_QUERY_ACCOUNTING_AND_APP_DEADLINE: PASS")
			if _, e := os.Stat(prepared.Unit.Workspace); !os.IsNotExist(e) {
				t.Fatal("ASSERT_NORMAL_DURABLE_SOURCE_REMOVED: FAIL", e)
			}
			t.Log("ASSERT_NORMAL_DURABLE_SOURCE_REMOVED: PASS")
			call("submit", task.RunID, task.TaskID, "interpreter")
			raw = call("result", task.RunID, task.TaskID)
			var result struct {
				Status       string
				Authority    uint8
				Accepted     bool
				Completeness string
				Description  sc.SelectedEvidenceV3
			}
			if json.Unmarshal(raw, &result) != nil || result.Status != "DESCRIBED_NOT_REVIEWED" || result.Authority != 0 || result.Accepted || result.Completeness != "UNKNOWN" || result.Description.Unit.Body != prepared.Unit.Body {
				t.Fatal("description result", string(raw))
			}
			call("duplicate", task.RunID, task.TaskID, "interpreter")
			t.Log("ASSERT_NORMAL_FOLDER_A_EXIT_B_DESCRIBE: PASS; REVIEW_AND_PUBLICATION_NOT_IMPLEMENTED; declared mode=" + mode)
		})
	}
}

func TestCensusDurableNormalFolderChild(t *testing.T) {
	action := os.Getenv("DURABLE_FOLDER_ACTION")
	if action == "" {
		t.Skip("subprocess only")
	}
	dir := os.Getenv("DURABLE_FOLDER_DIR")
	t.Setenv(censusDurableStoreEnv, filepath.Join(dir, "application"))
	var out, diagnostic bytes.Buffer
	if action == "start" {
		workspace := folderInventoryTemp(t)
		for name, body := range map[string]string{"go.mod": "module controlled.example/checkout\n\ngo 1.26\n", "checkout.go": "package checkout\nfunc Checkout() int { return price() }\n", "price.go": "package checkout\nfunc price() int { return 7 }\n"} {
			if e := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0600); e != nil {
				t.Fatal(e)
			}
		}
		wire := durableOneLeadWire(t, workspace)
		deps := productionCensusRunnerDependencies()
		deps.lookPath = func(string) (string, error) { return "/controlled/checkout-provider", nil }
		deps.newStarter = func() (sessionruntime.Starter, error) { return terminalStarter{child: wire}, nil }
		pin := "sha256:" + strings.Repeat("2", 64)
		hello := censusCallerHello{Protocol: censusCallerProtocol, Interpreter: "main", Reviewer: "main", Pins: dw.HostPinsV2{ModelSHA256: pin, WorkerSHA256: pin, GrammarSHA256: pin}}
		if os.Getenv("DURABLE_FOLDER_MODE") == "distinct-declared" {
			hello.Subagents = true
			hello.Reviewer = "separate-reviewer"
		}
		declaration, _ := json.Marshal(hello)
		t.Setenv(censusDurableCallerEnv, string(declaration))
		code := runCensusWithDependencies([]string{workspace, "--publication-root", filepath.Join(dir, "acquisition"), "--durable-describe", "--machine", "--request-timeout", "30s", "--timeout", "180s"}, &out, &diagnostic, deps)
		if code != 0 {
			t.Fatal("ASSERT_NORMAL_DURABLE_START: FAIL", code, diagnostic.String())
		}
		replies := wire.snapshot()
		counts := map[string]int{}
		keys := map[string]bool{}
		for _, r := range replies {
			key := r.method + string(r.params)
			if keys[key] {
				t.Fatal("repeated provider request", r.method)
			}
			keys[key] = true
			counts[r.method]++
		}
		if counts["initialize"] != 1 || counts["shutdown"] != 1 {
			t.Fatal("provider lifecycle count", counts)
		}
		t.Log("ASSERT_NORMAL_DURABLE_EXACT_PROVIDER_COUNTS:", counts)
		if e := folderInventoryBothOriginals(replies, wire.checkout, wire.price); e != nil {
			t.Fatal("ASSERT_NORMAL_DURABLE_ACQUIRE_ONCE: FAIL", e)
		}
		paths, e := filepath.Glob(filepath.Join(dir, "acquisition", "run-*", "folder-selected-*", "inventory-successor", "heads", "private", "v2", "*.json"))
		if e != nil {
			t.Fatal(e)
		}
		complete := 0
		for _, p := range paths {
			raw, e := os.ReadFile(p)
			var h struct {
				Event, Disposition string
				Windows            uint64 `json:"consumed_windows"`
				Charged            struct {
					Requests uint64 `json:"provider_requests"`
				} `json:"cumulative_charged"`
			}
			if e != nil || json.Unmarshal(raw, &h) != nil {
				t.Fatal("accounting", e)
			}
			if h.Event == "OBSERVE" && h.Disposition == "OBSERVED" && h.Windows == 3 {
				if h.Charged.Requests != 0 {
					t.Fatal("reacquisition")
				}
				complete++
			}
		}
		if complete != 1 {
			t.Fatal("ASSERT_NORMAL_DURABLE_MPS_ONCE: FAIL", complete)
		}
		t.Log("ASSERT_NORMAL_DURABLE_ACQUIRE_ONCE: PASS; ASSERT_NORMAL_DURABLE_MPS_ONCE: PASS; no connected caller")
	} else {
		var args []string
		if json.Unmarshal([]byte(os.Getenv("DURABLE_FOLDER_ARGS")), &args) != nil {
			t.Fatal("args")
		}
		invoke := func(a []string) int {
			return runCensusWithDependencies(append([]string{"durable"}, a...), &out, &diagnostic, productionCensusRunnerDependencies())
		}
		if action == "expired" {
			args = append(args, "interpreter")
			code := invoke(append([]string{"submit"}, args...))
			if code == 0 {
				t.Fatal("expired task accepted")
			}
			out.Reset()
			diagnostic.Reset()
			if invoke(append([]string{"task"}, args[:2]...)) != 0 {
				t.Fatal("expired status", diagnostic.String())
			}
			var task sc.DurableDescribeTaskV1
			if json.Unmarshal(out.Bytes(), &task) != nil || task.Status != "EXPIRED" {
				t.Fatal("expired task status", out.String())
			}
			if e := os.WriteFile(filepath.Join(dir, "response.json"), out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			return
		}
		if action == "held" {
			if invoke(append([]string{"submit"}, args...)) == 0 {
				t.Fatal("held-open transport accepted")
			}
			out.Reset()
			diagnostic.Reset()
			if invoke(append([]string{"task"}, args[:2]...)) != 0 {
				t.Fatal(diagnostic.String())
			}
			if e := os.WriteFile(filepath.Join(dir, "response.json"), out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			return
		}
		command := action
		if action == "duplicate" {
			command = "submit"
		}
		code := invoke(append([]string{command}, args...))
		if action == "duplicate" {
			if code == 0 {
				t.Fatal("duplicate accepted")
			}
			out.WriteString(`{"duplicate":"REFUSED"}`)
		} else if code != 0 {
			t.Fatal("ASSERT_NORMAL_DURABLE_CALL: FAIL", action, diagnostic.String())
		}
	}
	if e := os.WriteFile(filepath.Join(dir, "response.json"), out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
}

// The normal folder has one admitted definition query and observed-empty
// references, so this milestone exercises exactly one genuine M/P/S schedule.
// Only the controlled provider transport differs from production dependencies.
func durableOneLeadWire(t *testing.T, workspace string) *folderInventoryWire {
	t.Helper()
	a, b := io.Pipe()
	c, d := io.Pipe()
	f := &folderInventoryWire{inventoryPipePrivate: &inventoryPipePrivate{in: a, input: b, out: c, output: d, done: make(chan struct{})}, checkout: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "checkout.go")}).String(), price: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "price.go")}).String()}
	go func() {
		defer close(f.done)
		reader := lspwire.NewReader(f.in, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(f.output, lspwire.DefaultLimits())
		for n := 0; n < 128; n++ {
			m, e := reader.Read()
			if e != nil || m.Method == "exit" {
				return
			}
			if m.Method == "initialized" || strings.HasPrefix(m.Method, "textDocument/did") {
				continue
			}
			value, ok := folderInventoryResponse(m, f.checkout, f.price)
			if !ok {
				return
			}
			if m.Method == "textDocument/references" {
				value = []any{}
			}
			raw, e := json.Marshal(value)
			if e != nil {
				return
			}
			if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw}) != nil {
				return
			}
			f.mu.Lock()
			f.replies = append(f.replies, folderInventoryReply{m.Method, m.Params, raw, string(m.ID)})
			f.mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		f.Close()
		select {
		case <-f.done:
		case <-time.After(time.Second):
			t.Error("provider did not join")
		}
	})
	return f
}
