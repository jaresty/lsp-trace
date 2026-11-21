//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"context"
	"encoding/json"
	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/internal/censusresult"
	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type contractProcess struct {
	t               *testing.T
	home, mode, exe string
}

func newContractProcess(t *testing.T, mode string) (*contractProcess, ai.DurableTask) {
	t.Helper()
	p := &contractProcess{t: t, home: folderInventoryTemp(t), mode: mode}
	var err error
	p.exe, err = os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if mode == "publication-root" {
		if err = os.WriteFile(filepath.Join(p.home, "Library"), []byte("blocked per-user store"), 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.Mkdir(filepath.Join(p.home, "store"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	raw := p.call("start", nil, nil)
	var task ai.DurableTask
	if json.Unmarshal(raw, &task) != nil || task.Status != "PENDING" {
		t.Fatal("start task", string(raw))
	}
	return p, task
}
func (p *contractProcess) call(action string, answer []byte, argv []string) []byte {
	p.t.Helper()
	args := []string{}
	if action != "start" {
		if len(argv) < 5 || strings.Join(argv[:3], " ") != "lsp-trace census durable" {
			p.t.Fatal("returned operation invalid", argv)
		}
		args = argv[3:]
	}
	encoded, _ := json.Marshal(args)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.exe, "-test.run=^TestCensusDefaultDurableInventoryChild$", "-test.count=1")
	cmd.Env = []string{"HOME=" + p.home, "PATH=", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "DURABLE_INVENTORY_HOME=" + p.home, "DURABLE_INVENTORY_MODE=" + p.mode, "DURABLE_INVENTORY_ACTION=" + action, "DURABLE_INVENTORY_ARGS=" + string(encoded)}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		cmd.Env = append(cmd.Env, "TMPDIR="+tmp)
	}
	if padding := os.Getenv("DURABLE_SCALE_PADDING"); padding != "" {
		cmd.Env = append(cmd.Env, "DURABLE_SCALE_PADDING="+padding)
		cmd.Args = append(cmd.Args, "-test.v")
	}
	cmd.Stdin = bytes.NewReader(answer)
	log, err := cmd.CombinedOutput()
	if err != nil {
		p.t.Fatalf("returned operation execution failed: %v %v %s", argv, err, log)
	}
	if action == "start" && os.Getenv("DURABLE_SCALE_PADDING") != "" {
		p.t.Logf("SCALE_OWNER: %s", log)
	}
	raw, err := os.ReadFile(filepath.Join(p.home, "response.json"))
	if err != nil {
		p.t.Fatal(err)
	}
	return raw
}
func (p *contractProcess) observe(argv []string) ai.DurableTask {
	p.t.Helper()
	raw := p.call("call", nil, argv)
	var task ai.DurableTask
	if json.Unmarshal(raw, &task) != nil {
		p.t.Fatal("observe task", string(raw))
	}
	return task
}
func (p *contractProcess) head(run string) []byte {
	p.t.Helper()
	root := filepath.Join(p.home, "Library", "Application Support", "lsp-trace", "census", "durable")
	if p.mode == "publication-root" {
		root = filepath.Join(p.home, "store", "durable")
	}
	raw, err := os.ReadFile(filepath.Join(root, "inventory-run-"+run+".json"))
	if err != nil {
		p.t.Fatal(err)
	}
	return raw
}
func TestCensusReturnedDescribeExampleCompletes(t *testing.T) {
	for _, mode := range []string{"same-thread", "publication-root"} {
		t.Run(mode, func(t *testing.T) {
			p, task := newContractProcess(t, mode)
			pageRaw := p.call("call", nil, task.Contract.Operations["read"])
			var page sc.DurableDescribePageV1
			var evidence struct {
				Unit    struct{ Body string }
				Request struct{ Envelope struct{ Prompt string } }
			}
			if json.Unmarshal(pageRaw, &page) != nil || json.Unmarshal(page.Data, &evidence) != nil || evidence.Unit.Body == "" || evidence.Request.Envelope.Prompt == "" {
				t.Fatal("genuine retained input missing")
			}
			// Grammar/lifecycle specimen only; not a claim this prose is a real model judgment.
			raw := p.call("call", []byte(task.Contract.Example), task.Contract.Operations["submit"])
			var next ai.DurableTask
			if json.Unmarshal(raw, &next) != nil || next.PreviousTaskID != task.TaskID || next.Sequence != task.Sequence+1 || next.Status != "PENDING" {
				t.Fatal("ASSERT_RETURNED_EXAMPLE_ADVANCES", string(raw))
			}
			t.Log("ASSERT_RETURNED_EXAMPLE_ADVANCES: PASS")
		})
	}
}
func TestCensusReturnedRecoveryAfterLostOutput(t *testing.T) {
	for _, mode := range []string{"same-thread", "publication-root"} {
		t.Run(mode, func(t *testing.T) {
			p, task := newContractProcess(t, mode)
			// Independent recovery guard: a known admitted fixture isolates stale lookup
			// from the separate returned-example regression above.
			answer := dw.EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"controlled fixture","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"controlled fixture","consumer_relative":false},"boundary_contribution":"controlled fixture","limitations":[]}`)
			p.call("call", answer, task.Contract.Operations["submit"]) // deliberately discard successor
			before := p.head(task.RunID)
			next := p.observe(task.Contract.Operations["task"])
			after := p.head(task.RunID)
			if next.PreviousTaskID != task.TaskID || next.Sequence != task.Sequence+1 || next.Status != "PENDING" || !bytes.Equal(before, after) {
				t.Fatal("ASSERT_RETURNED_RECOVERY_NO_REPLAY_OR_CHARGE", next)
			}
			t.Log("ASSERT_RETURNED_RECOVERY_NO_REPLAY_OR_CHARGE: PASS")
		})
	}
}
func TestCensusReturnedReadAccounting(t *testing.T) {
	p, task := newContractProcess(t, "publication-root")
	if task.Contract.RunByteLimit != 64<<20 || task.Contract.RunPacketLimit != 64 {
		t.Fatal("stored cumulative limits missing", task.Contract)
	}
	for _, text := range []string{"$.Unit.Body", "$.Request.Envelope.Prompt", "verified immutable chunk delivery", "stop and report inability", "do not fabricate COMPLETE"} {
		if !strings.Contains(task.Contract.Rules, text) {
			t.Fatal("returned guidance missing", text)
		}
	}
	for _, limit := range []int{32768, 1} {
		argv := append([]string(nil), task.Contract.Operations["read"]...)
		argv[8] = fmtInt(limit)
		if limit == 1 {
			argv[7] = fmtInt(task.Artifacts[0].Bytes - 1)
		}
		p.call("call", nil, argv)
		next := p.observe(task.Contract.Operations["current_task"])
		offset := 0
		if limit == 1 {
			offset = task.Artifacts[0].Bytes - 1
		}
		count := min(limit, task.Artifacts[0].Bytes-offset)
		verified := min(task.Artifacts[0].Bytes, ((offset+count-1)/32768+1)*32768) - (offset/32768)*32768
		charge := uint64(8*task.InputIndexBytes + 6*verified + 2*count + 16384)
		if next.Used != task.Used+charge || next.Packets != task.Packets+1 || next.Work != task.Work+32*charge || next.Sequence != task.Sequence || next.SemanticReservedNS != task.SemanticReservedNS || next.DeadlineUnixNano != task.DeadlineUnixNano {
			t.Fatal("ASSERT_VERIFIED_CHUNK_PAGE_CHARGE", task, next, charge)
		}
		task = next
	}
	t.Log("ASSERT_VERIFIED_CHUNK_PAGE_CHARGE: PASS; full page and optional one-byte tail")
}

func TestCensusReturnedDiagnosticRecovery(t *testing.T) {
	p, task := newContractProcess(t, "publication-root")
	for _, run := range []string{task.RunID, "../../untrusted", strings.Repeat("0", 64)} {
		argv := []string{"lsp-trace", "census", "durable", "submit", run, task.TaskID, "wrong-role", "--publication-root", filepath.Join(p.home, "store")}
		raw := p.call("reject-detail", nil, argv)
		d := checkOperationBytes(t, raw)
		if d.Retry || bytes.Contains(raw, []byte(task.TaskID)) || bytes.Contains(raw, []byte(p.home)) {
			t.Fatal("ASSERT_DIAGNOSTIC_NO_AUTOMATIC_REPEAT", string(raw))
		}
		if run == task.RunID {
			if d.Reason != censusresult.OperationUnconfirmed || d.Action != "OBSERVE_CURRENT_TASK" || d.Recovery == nil || d.Recovery.RunID != run || d.Recovery.StoreContext != "SAME_SELECTION" {
				t.Fatal("ASSERT_DIAGNOSTIC_TRUSTED_CURRENT_RUN", string(raw))
			}
		} else if d.Recovery != nil || d.Action != "STOP" || d.Reason != censusresult.RunUnavailable {
			t.Fatal("untrusted recovery", string(raw))
		}
	}
	t.Log("ASSERT_DIAGNOSTIC_NO_AUTOMATIC_REPEAT: PASS; trusted current run only")
}

func TestCensusReturnedDescribeDisallowedVariantsSpent(t *testing.T) {
	for _, variant := range []string{"abstained", "resolved", "consumer-relative"} {
		t.Run(variant, func(t *testing.T) {
			p, task := newContractProcess(t, "publication-root")
			candidate, err := dw.ParseSuppliedTerminalResponse([]byte(task.Contract.Example), dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768})
			if err != nil {
				t.Fatal(err)
			}
			s := candidate.Semantic()
			s.Verdict = "COMPLETE"
			switch variant {
			case "abstained":
				s.Verdict = "ABSTAINED"
			case "resolved":
				s.ConsumerNeed = dw.ConsumerNeedV2{Status: "RESOLVED", Value: "unsupported consumer"}
			case "consumer-relative":
				s.ProvidedBehavior.ConsumerRelative = true
			}
			semantic, _ := json.Marshal(s)
			answer := dw.EncodeAdapterResultV2(string(semantic))
			if _, err = dw.ParseSuppliedTerminalResponse(answer, dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}); err != nil {
				t.Fatal("variant must be parser-valid", err)
			}
			p.call("reject", answer, task.Contract.Operations["submit"])
			spent := p.observe(task.Contract.Operations["task"])
			skippedPackets := uint64((task.Artifacts[0].Bytes - task.RequiredInputNextOffset + 32767) / 32768)
			if spent.Status != "UNCERTAIN_SPENT" || spent.TaskID != task.TaskID || spent.Sequence != task.Sequence || spent.Packets != task.Packets+1+skippedPackets || spent.RequiredInputNextOffset != task.Artifacts[0].Bytes || spent.Used <= task.Used {
				t.Fatal("ASSERT_DISALLOWED_VARIANT_SPENT", spent)
			}
			before := p.head(task.RunID)
			p.call("reject", []byte(task.Contract.Example), task.Contract.Operations["submit"])
			if !bytes.Equal(before, p.head(task.RunID)) {
				t.Fatal("spent replay charged or changed head")
			}
			t.Log("ASSERT_DISALLOWED_VARIANT_SPENT: PASS", variant)
		})
	}
}
