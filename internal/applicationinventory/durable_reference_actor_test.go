//go:build (darwin || linux) && lsp_trace_controlled_describe_fixture

package applicationinventory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	dw "lsp-trace/internal/describeworker"
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type referencePreparedInput struct {
	LeadID    string
	Evidence  publication.StoredObject
	Admission publication.StoredObject
	Input     json.RawMessage
}
type referencePreparedExchange struct {
	Schema, RunID string
	Inputs        []referencePreparedInput
}
type referenceActorPacket struct {
	Task        DurableTask
	InputSHA256 string
	Input       json.RawMessage
}
type referenceActorReply struct {
	RunID, TaskID, Role, Actor, InputSHA256 string
	Answer                                  []byte
}

func referenceActorRead(path string, max int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() > max {
		return nil, fmt.Errorf("bounded regular actor file required")
	}
	raw, e := io.ReadAll(io.LimitReader(f, max+1))
	if e != nil || int64(len(raw)) > max {
		return nil, fmt.Errorf("actor file bound")
	}
	return raw, nil
}
func referenceActorWrite(t *testing.T, path string, v any) {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path+".tmp", b, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(path+".tmp", path); e != nil {
		t.Fatal(e)
	}
}

// Explicit controlled test driver, not CLI/product dispatch or a new controller.
// prepare leaves only a clean acquired v9 or explicitly selected v10 job
// (no semantic lease issued). The semantic marker selects 10m/four stages.
// execute starts existing Seal exactly once and accepts separately supplied
// input-exact replies under the actual returned deadlines. It never authors one.
func TestInventoryReferenceActorExchange(t *testing.T) {
	n := len(os.Args)
	if n < 4 || (os.Args[n-3] != "inventory-reference-actors-v1" && os.Args[n-3] != "inventory-semantic-actors-v1") {
		t.Skip("explicit controlled actor exchange only")
	}
	semantic := os.Args[n-3] == "inventory-semantic-actors-v1"
	prefix, headSchema, preparedSchema := "", is.InventoryReferenceHeadVersionV1, "controlled-reference-offline-inputs.v1"
	if semantic {
		prefix = "semantic-"
		headSchema = is.InventorySemanticHeadVersionV1
		preparedSchema = "controlled-semantic-offline-inputs.v1"
	}
	action, dir := os.Args[n-2], os.Args[n-1]
	temp, e := filepath.EvalSymlinks(os.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	real, e := filepath.EvalSymlinks(dir)
	rel, er := filepath.Rel(temp, dir)
	if e != nil || er != nil || real != dir || !filepath.IsLocal(rel) || !strings.HasPrefix(filepath.Base(dir), "lsp-reference-actors-") {
		t.Fatal("private canonical controlled temporary exchange only")
	}
	st, e := os.Lstat(dir)
	if e != nil || !st.IsDir() || st.Mode().Perm() != 0700 {
		t.Fatal("private exchange required")
	}
	store := filepath.Join(dir, "store")
	if action == "prepare" {
		files, e := os.ReadDir(dir)
		if e != nil || len(files) != 0 {
			t.Fatal("new empty exchange only")
		}
		for _, p := range []string{store, filepath.Join(dir, "source"), filepath.Join(dir, "tasks"), filepath.Join(dir, "responses")} {
			if e = os.Mkdir(p, 0700); e != nil {
				t.Fatal(e)
			}
		}
		first, _ := runResumeProcess(t, store, "", prefix+"references-actors-first")
		if wait := time.Until(time.Unix(0, first.Deadline)); wait > 0 {
			time.Sleep(wait + time.Millisecond)
		}
		second, _ := runResumeProcess(t, store, first.Status.RunID, prefix+"references-actors-success")
		root, e := publication.OpenRoot(store)
		if e != nil {
			t.Fatal(e)
		}
		defer root.Close()
		s := DurableStore{Root: root, Directory: store}
		h, _, e := s.head(second.Status.RunID)
		if e != nil || h.Schema != headSchema || h.Phase != "Admitted" || h.Sequence != 2 || h.Status != "PARTIAL" {
			t.Fatal("clean acquired matching-version job required", e)
		}
		prepared := referencePreparedExchange{Schema: preparedSchema, RunID: h.RunID, Inputs: make([]referencePreparedInput, 2)}
		ref := h.Input
		for i := 1; i >= 0; i-- {
			raw, e := publication.ReadObject(root, "inventory-admission-", ref)
			var a ownedAdmissionRecord
			if e != nil || strictInventory(raw, &a) != nil || a.Schema != h.admissionSchema() || a.RunID != h.RunID || a.Index != i {
				t.Fatal("admission", e)
			}
			raw, e = publication.ReadObject(root, "inventory-evidence-", a.Evidence)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = is.ValidateInventoryReferenceEvidenceV1(raw, h.RunID, a.LeadID); e != nil {
				t.Fatal(e)
			}
			prepared.Inputs[i] = referencePreparedInput{a.LeadID, a.Evidence, ref, raw}
			ref = a.Previous
		}
		referenceActorWrite(t, filepath.Join(dir, "prepared.json"), prepared)
		if e = root.Close(); e != nil {
			t.Fatal(e)
		}
		referenceActorWrite(t, filepath.Join(dir, "prepare-file-manifest.json"), retentionPreflightFiles(t, store))
		t.Logf("ACTOR_PREPARED run=%s semantic_tasks=0 job_expiry=NONE firstPID=%d secondPID=%d", h.RunID, first.PID, second.PID)
		return
	}
	if action != "execute" {
		t.Fatal("closed actions: prepare or execute")
	}
	raw, e := referenceActorRead(filepath.Join(dir, "prepared.json"), 128<<10)
	var prepared referencePreparedExchange
	if e != nil || cat.DecodeNativeV3(raw, &prepared) != nil || prepared.Schema != preparedSchema || len(prepared.Inputs) != 2 {
		t.Fatal("prepared exchange", e)
	}
	root, e := publication.OpenRoot(store)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	s := DurableStore{Root: root, Directory: store}
	ctx := context.Background()
	h, old, e := s.head(prepared.RunID)
	if e != nil || h.Schema != headSchema || h.Phase != "Admitted" || h.Status != "PARTIAL" || h.Sequence != 2 {
		t.Fatal("execute requires original clean acquired matching-version job; no task renewal", e)
	}
	if h.Checkpoint == nil || prepared.Inputs[0].Admission != h.Checkpoint.Admission || prepared.Inputs[1].Admission != h.Input {
		t.Fatal("offline admission binding changed")
	}
	task, e := s.Seal(ctx, h.RunID, cat.NativeDigestV3(old))
	if e != nil {
		t.Fatal(e)
	}
	for task.Phase != "Published" {
		task, e = s.Task(ctx, task.RunID, task.TaskID)
		if e != nil {
			t.Fatal(e)
		}
		var input []byte
		for offset := 0; offset < task.Artifacts[0].Bytes; {
			p, e := s.Read(ctx, task.RunID, task.TaskID, "input", offset, 32768)
			if e != nil {
				t.Fatal(e)
			}
			if len(p.Data) == 0 || p.Total != task.Artifacts[0].Bytes {
				t.Fatal("incomplete public input")
			}
			input = append(input, p.Data...)
			offset += len(p.Data)
		}
		digest := cat.NativeDigestV3(input)
		if task.Phase == "Describe" {
			expected := prepared.Inputs[task.Sequence-1]
			if digest != "sha256:"+expected.Evidence.ID || digest != cat.NativeDigestV3(expected.Input) {
				t.Fatal("offline input does not exactly match fresh public Read")
			}
		}
		packet := referenceActorPacket{task, digest, input}
		referenceActorWrite(t, filepath.Join(dir, "tasks", task.TaskID+".json"), packet)
		t.Logf("ACTOR_TASK phase=%s task=%s actor=%s input=%s deadline=%d", task.Phase, task.TaskID, task.Actor, digest, task.DeadlineUnixNano)
		replyPath := filepath.Join(dir, "responses", task.TaskID+".json")
		var reply referenceActorReply
		for {
			if time.Now().UnixNano() >= task.DeadlineUnixNano {
				t.Fatal("ACTOR_LEASE_EXPIRED; no reset or replay")
			}
			raw, e = referenceActorRead(replyPath, 96<<10)
			if e == nil {
				break
			}
			if !os.IsNotExist(e) {
				t.Fatal(e)
			}
			time.Sleep(20 * time.Millisecond)
		}
		if cat.DecodeNativeV3(raw, &reply) != nil || !referenceReplyMatches(reply, task, digest) {
			t.Fatal("exact live actor reply binding required")
		}
		task, e = s.Submit(ctx, task.RunID, task.TaskID, task.Role, reply.Answer)
		if e != nil {
			t.Fatal(e)
		}
	}
	report, e := s.Result(ctx, task.RunID, task.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	checkReferenceResult(t, report)
	if semantic {
		if report.SemanticReservedNS != int64(40*time.Minute) {
			t.Fatal("FINAL_SEMANTIC_RESERVATION", report.SemanticReservedNS)
		}
		raw, _ := json.Marshal(report)
		t.Log("SEMANTIC_REPORT_JSON " + string(raw))
	}
	referenceActorWrite(t, filepath.Join(dir, "result.json"), report)
	if e = root.Close(); e != nil {
		t.Fatal(e)
	}
	referenceActorWrite(t, filepath.Join(dir, "execute-file-manifest.json"), retentionPreflightFiles(t, store))
	t.Log("ACTOR_PUBLICATION_READBACK; authorship is externally asserted, not authenticated by this driver")
}
func referenceReplyMatches(r referenceActorReply, t DurableTask, digest string) bool {
	return r.RunID == t.RunID && r.TaskID == t.TaskID && r.Role == t.Role && r.Actor == t.Actor && r.InputSHA256 == digest
}

// Only the transport/control route is exercised. These replies are explicitly
// deterministic fixture replies, not independent semantic judgments.
func TestInventoryReferenceActorDriver(t *testing.T) { referenceActorDriver(t, false) }
func TestInventorySemanticActorDriver(t *testing.T)  { referenceActorDriver(t, true) }
func referenceActorDriver(t *testing.T, semantic bool) {
	dir, e := os.MkdirTemp("", "lsp-reference-actors-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	dir, e = filepath.EvalSymlinks(dir)
	if e != nil {
		t.Fatal(e)
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	args := []string{"-test.run=^TestInventoryReferenceActorExchange$", "-test.v", "inventory-reference-actors-v1", "prepare", dir}
	if semantic {
		args[2] = "inventory-semantic-actors-v1"
	}
	raw, e := exec.CommandContext(ctx, exe, args...).CombinedOutput()
	t.Log("ACTOR_DRIVER_PREPARE " + string(raw))
	if e != nil {
		t.Fatal(e)
	}
	args[len(args)-2] = "execute"
	cmd := exec.CommandContext(ctx, exe, args...)
	var log strings.Builder
	cmd.Stdout = &log
	cmd.Stderr = &log
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	seen := map[string]bool{}
	for {
		select {
		case e := <-done:
			t.Log("ACTOR_DRIVER_EXECUTE " + log.String())
			if e != nil || len(seen) != 4 {
				t.Fatal("ACTOR_DRIVER", e, len(seen))
			}
			files, _ := json.Marshal(retentionPreflightFiles(t, filepath.Join(dir, "store")))
			t.Log("CONNECTED_FILES_JSON " + string(files))
			return
		default:
		}
		paths, e := filepath.Glob(filepath.Join(dir, "tasks", "*.json"))
		if e != nil {
			t.Fatal(e)
		}
		for _, p := range paths {
			if seen[p] {
				continue
			}
			raw, e := referenceActorRead(p, 128<<10)
			var packet referenceActorPacket
			if e != nil || cat.DecodeNativeV3(raw, &packet) != nil {
				t.Fatal("actor packet", e)
			}
			if cat.NativeDigestV3(packet.Input) != packet.InputSHA256 {
				t.Fatal("ACTOR_PACKET_BYTES_CHANGED")
			}
			task := packet.Task
			if semantic {
				checkSemanticTask(t, task, 10*time.Minute)
			} else if task.SemanticPolicy != nil || task.SemanticReservedNS > int64(120*time.Second) {
				t.Fatal("HISTORICAL_POLICY_CHANGED")
			}
			var answer []byte
			if task.Phase == "Describe" {
				answer = dw.EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"controlled fixture query","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"controlled source-visible operation","consumer_relative":false},"boundary_contribution":"retained query-side reference observation","limitations":["mechanical route test; no independent judgment"]}`)
			} else {
				var c struct{ Revision string }
				if json.Unmarshal(packet.Input, &c) != nil {
					t.Fatal("catalog")
				}
				answer, _ = json.Marshal(cat.NativeDecisionV3{Predecessor: c.Revision, Actor: task.Actor, Reason: "mechanical actor route only; feature membership unresolved", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
			}
			reply := referenceActorReply{task.RunID, task.TaskID, task.Role, task.Actor, packet.InputSHA256, answer}
			for _, field := range []string{"run", "task", "role", "actor", "input"} {
				bad := reply
				switch field {
				case "run":
					bad.RunID += "x"
				case "task":
					bad.TaskID += "x"
				case "role":
					bad.Role += "x"
				case "actor":
					bad.Actor += "x"
				case "input":
					bad.InputSHA256 += "x"
				}
				if referenceReplyMatches(bad, task, packet.InputSHA256) {
					t.Fatal("ACTOR_BINDING_ACCEPTED", field)
				}
			}
			checkOriginalReferenceInput(t, task, packet.Input)
			t.Log("ACTOR_DRIVER_PACKET " + string(raw))
			referenceActorWrite(t, filepath.Join(dir, "responses", task.TaskID+".json"), reply)
			seen[p] = true
		}
		time.Sleep(10 * time.Millisecond)
	}
}
