//go:build (darwin || linux) && lsp_trace_controlled_describe_fixture

package applicationinventory

import (
	"context"
	"encoding/json"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInventoryConnectedPublication(t *testing.T) { runConnectedPublication(t, false) }
func runConnectedPublication(t *testing.T, references bool) {
	prefix := "connected-"
	if references {
		prefix = "references-"
	}
	modes := []string{"success", "missing", "tampered", "bad-answer"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			base, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			dir := filepath.Join(base, "store")
			for _, p := range []string{dir, filepath.Join(base, "source")} {
				if e = os.Mkdir(p, 0700); e != nil {
					t.Fatal(e)
				}
			}
			childPrefix := prefix
			first, _ := runResumeProcess(t, dir, "", childPrefix+"first")
			if mode == "success" {
				if wait := time.Until(time.Unix(0, first.Deadline)); wait > 0 {
					time.Sleep(wait + time.Millisecond)
				}
			}
			second, _ := runResumeProcess(t, dir, first.Status.RunID, childPrefix+"success")
			if first.PID == second.PID || first.Symbols != 16 || second.Symbols != 1 || first.Starts != 1 || second.Starts != 1 || second.Status.Used <= first.Status.Used {
				t.Fatal("genuine cumulative 16+1")
			}
			root, e := publication.OpenRoot(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer root.Close()
			s := DurableStore{Root: root, Directory: dir}
			ctx := context.Background()
			run := second.Status.RunID
			h, old, e := s.head(run)
			if e != nil {
				t.Fatal(e)
			}
			if h.Limit != h.connectedLimit() || h.Used > h.acquisitionLimit() {
				t.Fatal("initial connected profile")
			}
			before := retentionPreflightFiles(t, dir)
			if mode == "missing" || mode == "tampered" {
				evidenceRef := h.Checkpoint.Evidence
				if mode == "tampered" {
					raw, err := publication.ReadObject(root, "inventory-admission-", h.Input)
					var admission ownedAdmissionRecord
					if err != nil || strictInventory(raw, &admission) != nil {
						t.Fatal("second admission", err)
					}
					evidenceRef = admission.Evidence
				}
				p := filepath.Join(dir, "inventory-evidence-"+evidenceRef.ID+".json")
				if mode == "missing" {
					e = os.Remove(p)
				} else {
					e = os.WriteFile(p, []byte("{}"), 0600)
				}
				if e != nil {
					t.Fatal(e)
				}
				if _, e = s.Seal(ctx, run, cat.NativeDigestV3(old)); e == nil {
					t.Fatal("CORRUPT_SEAL")
				}
				after, _, err := s.head(run)
				if err != nil || after.Phase != "Sealing" || after.Status != "CLAIMED" || after.Used != h.Used+h.connectedSealBytes() {
					t.Fatal("SEAL_FAILURE_NOT_SPENT", err)
				}
				if _, e = s.Seal(ctx, run, cat.NativeDigestV3(old)); e == nil {
					t.Fatal("SEAL_FAILURE_REPLAY")
				}
				if e = root.Close(); e != nil {
					t.Fatal("cleanup", e)
				}
				census, _ := json.Marshal(retentionPreflightFiles(t, dir))
				t.Log("CONNECTED_FILES_JSON " + string(census))
				t.Logf("CONNECTED_REFUSAL mode=%s spent=%d", mode, after.Used)
				return
			}
			task, e := s.Seal(ctx, run, cat.NativeDigestV3(old))
			if e != nil {
				observed, _, headErr := s.head(run)
				t.Fatalf("CONNECTED_SEAL %v phase=%s status=%s headErr=%v files=%+v", e, observed.Phase, observed.Status, headErr, retentionPreflightFiles(t, dir))
			}
			if task.Phase != "Describe" || task.Status != "PENDING" || task.RunDeadlineUnixNano != 0 || task.Used != h.Used+h.connectedSealBytes() {
				t.Fatal("SEALED_TASK", task)
			}
			if _, e = s.Seal(ctx, run, cat.NativeDigestV3(old)); e == nil {
				t.Fatal("DUPLICATE_SEAL")
			}
			sealed, _, err := s.head(run)
			if err != nil {
				t.Fatal(err)
			}
			for _, mutate := range []func(*durableInventoryHead){
				func(x *durableInventoryHead) { x.Completion.Inputs[0] = cat.InventoryConnectedEvidenceV1 + 1 },
				func(x *durableInventoryHead) { x.Completion.Payloads[1] = cat.InventoryConnectedPayloadV1 + 1 },
				func(x *durableInventoryHead) { x.Completion.InitialStateBytes = cat.InventoryConnectedStateV1 + 1 },
				func(x *durableInventoryHead) { x.Completion.Manifest = h.manifestLimit() + 1 },
				func(x *durableInventoryHead) {
					x.Completion.Limits.MaxAdapterBytes = cat.InventoryConnectedAdapterV1 + 1
				},
				func(x *durableInventoryHead) {
					x.Completion.Limits.MaxSemanticBytes = cat.InventoryConnectedSemanticV1 + 1
				},
				func(x *durableInventoryHead) { x.Used -= h.Used; x.Work = x.Used * 32 },
			} {
				bad := sealed
				bad.Completion.Inputs = append([]int(nil), sealed.Completion.Inputs...)
				bad.Completion.Payloads = append([]uint64(nil), sealed.Completion.Payloads...)
				mutate(&bad)
				if bad.completionValid() {
					t.Fatal("CONNECTED_BOUND_OR_PRIOR_SPEND_BYPASS")
				}
			}
			otherConfig, err := s.ownedConfig(h)
			if err != nil {
				t.Fatal(err)
			}
			other, err := s.Create(ctx, otherConfig)
			if err != nil {
				t.Fatal("cross-job setup", err)
			}
			if _, e = s.Submit(ctx, other.RunID, task.TaskID, task.Role, nil); e == nil {
				t.Fatal("CROSS_JOB_SUBMISSION")
			}
			if _, e = s.Submit(ctx, run, task.TaskID, "reviewer", nil); e == nil {
				t.Fatal("WRONG_ROLE_SUBMISSION")
			}
			if mode == "bad-answer" {
				if _, e = s.Submit(ctx, run, task.TaskID, task.Role, []byte("invalid")); e == nil {
					t.Fatal("BAD_ANSWER_ACCEPTED")
				}
				got, e := s.Task(ctx, run, task.TaskID)
				if e != nil || got.Status != "UNCERTAIN_SPENT" || got.Used <= task.Used {
					t.Fatal("BAD_ANSWER_UNSPENT", e)
				}
				for _, ref := range []publication.StoredObject{h.Checkpoint.Evidence, h.Checkpoint.Admission, h.Input} {
					prefix := "inventory-admission-"
					if ref == h.Checkpoint.Evidence {
						prefix = "inventory-evidence-"
					}
					if _, e = publication.ReadObject(root, prefix, ref); e != nil {
						t.Fatal("prior evidence lost", e)
					}
				}
				if e = root.Close(); e != nil {
					t.Fatal("cleanup", e)
				}
				census, _ := json.Marshal(retentionPreflightFiles(t, dir))
				t.Log("CONNECTED_FILES_JSON " + string(census))
				t.Logf("CONNECTED_REFUSAL mode=%s spent=%d", mode, got.Used)
				return
			}
			descriptions, groups, reviews := 0, 0, 0
			interpreter := ""
			reviewer := ""
			for task.Phase != "Published" {
				current := task
				var input []byte
				for offset := 0; offset < task.Artifacts[0].Bytes; {
					page, err := s.Read(ctx, run, task.TaskID, "input", offset, 32768)
					if err != nil {
						t.Fatal("READ", err)
					}
					input = append(input, page.Data...)
					offset += len(page.Data)
				}
				packet, _ := json.Marshal(struct {
					Task  DurableTask
					Input json.RawMessage
				}{task, json.RawMessage(input)})
				t.Log("CONNECTED_PACKET " + string(packet))
				if references {
					checkOriginalReferenceInput(t, task, input)
				}
				var answer []byte
				if task.Phase == "Describe" {
					if !strings.Contains(string(input), "Query") {
						t.Fatal("missing controlled original query evidence")
					}
					interpreter = task.Actor
					descriptions++
					semantic := `{"verdict":"COMPLETE","target_role":"controlled fixture query function","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"source-visible controlled fixture operation; mechanical plumbing witness only","consumer_relative":false},"boundary_contribution":"query-side retained reference evidence; not complete application coverage","limitations":["deterministic test submission, not independent semantic judgment","consumer identity and feature membership unresolved"]}`
					answer = dw.EncodeAdapterResultV2(semantic)
				} else {
					var c struct {
						Revision string `json:"revision"`
					}
					if json.Unmarshal(input, &c) != nil || c.Revision == "" {
						t.Fatal("catalog revision")
					}
					if task.Phase == "Reviewer" {
						reviewer = task.Actor
						reviews++
						if reviewer == interpreter {
							t.Fatal("SAME_REVIEW_ACTOR")
						}
					} else {
						groups++
					}
					answer, _ = json.Marshal(cat.NativeDecisionV3{Predecessor: c.Revision, Actor: task.Actor, Reason: "mechanical test only; feature membership unresolved", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
				}
				task, e = s.Submit(ctx, run, task.TaskID, task.Role, answer)
				if e != nil {
					t.Fatalf("SUBMIT phase=%s: %v", current.Phase, e)
				}
				if _, e = s.Submit(ctx, run, current.TaskID, current.Role, answer); e == nil {
					t.Fatal("STALE_DUPLICATE_SUBMISSION")
				}
			}
			report, e := s.Result(ctx, run, task.TaskID)
			if e != nil {
				t.Fatal("PUBLICATION_READBACK", e)
			}
			finalTask, err := s.Task(ctx, run, task.TaskID)
			if err != nil || finalTask.Used <= task.Used || finalTask.Used > h.connectedLimit() {
				t.Fatal("readback cumulative spend", err)
			}
			raw, _ := json.Marshal(report)
			if references {
				checkReferenceResult(t, report)
			}
			if !report.Published || report.Accepted || report.Authority != 0 || report.Completeness != "UNKNOWN" || descriptions != 2 || groups != 1 || reviews != 1 || !strings.Contains(string(raw), "PAID_SCOPE_17_MEMBERS") || !strings.Contains(string(raw), "query-17.go") || !strings.Contains(string(raw), "NOT_FULL_D_R_CALL_COVERAGE") {
				t.Fatal("QUALIFIED_PUBLICATION", string(raw))
			}
			afterFiles := retentionPreflightFiles(t, dir)
			afterMap := map[string]retentionPreflightFile{}
			for _, entry := range afterFiles {
				afterMap[entry.Path] = entry
			}
			for _, entry := range before {
				if strings.HasPrefix(entry.Path, "inventory-run-") {
					continue
				}
				if got, ok := afterMap[entry.Path]; !ok || got != entry {
					t.Fatal("prior immutable artifact changed", entry.Path)
				}
			}
			t.Logf("CONNECTED_PUBLICATION processA=%d processB=%d symbols=17 R=2 admissions=2 describes=%d groupTasks=%d reviewTasks=%d independentSemanticJudgments=0 publication=1 used=%d", first.PID, second.PID, descriptions, groups, reviews, finalTask.Used)
			if e = root.Close(); e != nil {
				t.Fatal("parent root cleanup", e)
			}
			files := retentionPreflightFiles(t, dir)
			census, _ := json.Marshal(files)
			t.Log("CONNECTED_FILES_JSON " + string(census))
		})
	}
}

func TestInventoryConnectedIncomplete(t *testing.T) { runConnectedIncomplete(t, false) }
func runConnectedIncomplete(t *testing.T, references bool) {
	prefix := "connected-"
	if references {
		prefix = "references-"
	}
	for _, mode := range []string{"pending", "fail"} {
		t.Run(mode, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(base, "store")
			for _, p := range []string{dir, filepath.Join(base, "source")} {
				if err = os.Mkdir(p, 0700); err != nil {
					t.Fatal(err)
				}
			}
			first, _ := runResumeProcess(t, dir, "", prefix+"first")
			before := retentionPreflightFiles(t, dir)
			second, _ := runResumeProcess(t, dir, first.Status.RunID, prefix+mode)
			root, err := publication.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			s := DurableStore{Root: root, Directory: dir}
			ctx := context.Background()
			if second.Status.Status != "PARTIAL_UNCERTAIN" || second.Status.Admissions != 1 || second.Status.Used <= first.Status.Used {
				t.Fatal("INCOMPLETE_NOT_SPENT")
			}
			if _, err = s.Seal(ctx, first.Status.RunID, second.Status.Head); err == nil {
				t.Fatal("INCOMPLETE_SEAL")
			}
			if _, err = s.NativeDiscoveryBatchV1(ctx, first.Status.RunID, second.Status.Head); err == nil {
				t.Fatal("INCOMPLETE_REGRANT")
			}
			for _, f := range before {
				if strings.HasPrefix(f.Path, "inventory-run-") {
					continue
				}
				raw, err := os.ReadFile(filepath.Join(dir, f.Path))
				if err != nil || cat.NativeDigestV3(raw) != "sha256:"+f.SHA256 {
					t.Fatal("FIRST_EVIDENCE_CHANGED", f.Path, err)
				}
			}
			if err = root.Close(); err != nil {
				t.Fatal(err)
			}
			census, _ := json.Marshal(retentionPreflightFiles(t, dir))
			t.Log("CONNECTED_FILES_JSON " + string(census))
		})
	}
}

func TestInventoryConnectedQuoteMatchesProfile(t *testing.T) {
	bytes, packets := connectedCandidateQuote(t, cat.InventoryConnectedEvidenceV1, cat.InventoryConnectedPayloadV1, cat.InventoryConnectedStateV1, cat.InventoryConnectedManifestV1)
	if bytes != cat.InventoryConnectedDeliveryBytesV1 || packets != 19 || 8+1+packets > 64 || cat.InventoryConnectedLimitV1 != cat.InventoryResumeLimitV1+cat.InventoryConnectedSealBytesV1+bytes {
		t.Fatal("CONNECTED_PROFILE_FORMULA", bytes, cat.InventoryConnectedDeliveryBytesV1, packets)
	}
	b, err := cat.NewInventoryConnectedBudgetV1(time.Now().Add(time.Second), cat.InventoryConnectedSealBytesV1, cat.InventoryResumeLimitV1, 0)
	if err != nil {
		t.Fatal(err)
	}
	prior, _ := b.Usage()
	if prior != cat.InventoryResumeLimitV1 {
		t.Fatal("CUMULATIVE_SPEND_RESET")
	}
	if _, err = cat.NewReservedCallerBudgetV3(time.Now().Add(time.Second), cat.InventoryConnectedSealBytesV1, prior, 0); err == nil {
		t.Fatal("LEGACY_ENCLOSURE_ENLARGED")
	}
	t.Logf("CONNECTED_PROFILE acquisition=%d seal=%d semantic=%d H=%d work=%d packets=28", cat.InventoryResumeLimitV1, cat.InventoryConnectedSealBytesV1, bytes, cat.InventoryConnectedLimitV1, cat.InventoryConnectedLimitV1*32)
}
