//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/publication"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The owner helper is the existing controlled CLI test executable, not a private
// head constructor. Build it with the controlled fixture tag and pass its path.
func TestSubmitCASKilledProcesses(t *testing.T) {
	if dir := os.Getenv("SUBMIT_CAS_ROOT"); dir != "" {
		root, e := publication.OpenRoot(dir)
		if e != nil {
			t.Fatal(e)
		}
		s := DurableStore{Root: root, Directory: dir}
		h, raw, e := s.head(os.Getenv("SUBMIT_CAS_RUN"))
		if e != nil {
			t.Fatal(e)
		}
		if os.Getenv("SUBMIT_CAS_STAGE") == "claim" {
			_, _, e = s.reserve(context.Background(), h, raw, h.Completion.Stages[h.CompletionCursor], true, true)
		} else {
			_, _, _, e = s.claimSubmit(context.Background(), h, raw, 29050)
		}
		if e != nil {
			t.Fatal(e)
		}
		// Both production calls return only after successful CAS verification. No
		// state/answer interpretation or successor storage occurs in this child.
		if e = syscall.Kill(os.Getpid(), syscall.SIGKILL); e != nil {
			t.Fatal(e)
		}
		t.Fatal("SIGKILL returned")
	}
	owner := os.Getenv("LSP_SUBMIT_CAS_OWNER_HELPER")
	if owner == "" {
		t.Skip("requires compiled controlled CLI owner helper")
	}
	for _, stage := range []string{"claim", "topup"} {
		t.Run(stage, func(t *testing.T) {
			home := t.TempDir()
			if e := os.Chmod(home, 0700); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(owner, "-test.run=^TestCensusDefaultDurableInventoryChild$", "-test.count=1")
			cmd.Env = []string{"HOME=" + home, "PATH=", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "DURABLE_INVENTORY_HOME=" + home, "DURABLE_INVENTORY_MODE=three-item", "DURABLE_INVENTORY_ACTION=start"}
			if tmp := os.Getenv("TMPDIR"); tmp != "" {
				cmd.Env = append(cmd.Env, "TMPDIR="+tmp)
			}
			out, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatalf("owner: %v %s", e, out)
			}
			raw, e := os.ReadFile(filepath.Join(home, "response.json"))
			if e != nil {
				t.Fatal(e)
			}
			var task DurableTask
			if json.Unmarshal(raw, &task) != nil || task.Status != "PENDING" {
				t.Fatal("owner task")
			}
			// macOS owner runtime; Linux compilation is only a regression gate.
			dir := filepath.Join(home, "Library", "Application Support", "lsp-trace", "census", "durable")
			root, e := publication.OpenRoot(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer root.Close()
			s := DurableStore{Root: root, Directory: dir}
			before, old, e := s.head(task.RunID)
			if e != nil {
				t.Fatal(e)
			}
			files := snapshotSubmitStore(t, dir)
			expected := before
			base := before.Completion.Stages[before.CompletionCursor]
			expected.CompletionCursor++
			expected.Used += base
			expected.Work += 32 * base
			expected.Packets++
			expected.Status = "CLAIMED"
			var topup, skipped, skippedPackets uint64
			if stage == "topup" {
				plan, _, _, e := submitPlan(before, 29050)
				if e != nil {
					t.Fatal(e)
				}
				// Independent next-evidence construction charge, not an observed delta.
				nextBytes := before.Completion.Inputs[1]
				upper := uint64(1024 + 128*((nextBytes+32767)/32768))
				topup = 14*uint64(nextBytes) + 16*upper + 32768
				if plan.Stages[0] != base+topup {
					t.Fatal("topup oracle")
				}
				for off := before.Delivery.Next; off < before.Input.Bytes; off += 32768 {
					n := min(32768, before.Input.Bytes-off)
					skipped += 8*uint64(before.Delivery.Index.Bytes) + 8*uint64(n) + 16384
					skippedPackets++
				}
				expected.Completion = plan
				expected.SuccessorReserve = 0
				expected.SuccessorPackets = 0
				expected.Delivery.Next = before.Input.Bytes
				expected.DeliverySpent += skipped
				expected.Used += topup + skipped
				expected.Work = 32 * expected.Used
				expected.Packets += skippedPackets
			}
			want, e := encodeInventoryHead(expected)
			if e != nil {
				t.Fatal(e)
			}
			bp, bpk, _ := before.protectedCapacity()
			ep, epk, _ := expected.protectedCapacity()
			t.Logf("CAS_BEFORE stage=%s used=%d work=%d packets=%d cursor=%d protected=%d protectedPackets=%d base=%d topup=%d skipped=%d skippedPackets=%d", stage, before.Used, before.Work, before.Packets, before.CompletionCursor, bp, bpk, base, topup, skipped, skippedPackets)
			cmd = exec.Command(os.Args[0], "-test.run=^TestSubmitCASKilledProcesses$", "-test.count=1")
			cmd.Env = append(os.Environ(), "SUBMIT_CAS_ROOT="+dir, "SUBMIT_CAS_RUN="+task.RunID, "SUBMIT_CAS_STAGE="+stage)
			out, e = cmd.CombinedOutput()
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ProcessState.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Fatalf("not SIGKILL: %v %s", e, out)
			}
			after, got, e := s.head(task.RunID)
			if e != nil || !bytes.Equal(got, want) || bytes.Equal(old, got) {
				t.Fatal("ASSERT_CAS_EXACT_PERSISTED_TRANSFER", e)
			}
			ap, apk, valid := after.protectedCapacity()
			if !valid || ap != ep || apk != epk {
				t.Fatal("protected transfer")
			}
			files[inventoryHeadName(task.RunID)] = got
			if !reflect.DeepEqual(files, snapshotSubmitStore(t, dir)) {
				t.Fatal("ASSERT_NO_PREMATURE_ANSWER_OR_SUCCESSOR")
			}
			observed, e := s.Task(context.Background(), task.RunID, "")
			if e != nil || observed.Status != "UNCERTAIN_SPENT" || observed.TaskID != task.TaskID || observed.SemanticReservedNS != task.SemanticReservedNS || observed.DeadlineUnixNano != task.DeadlineUnixNano {
				t.Fatal("ASSERT_UNCERTAIN_SPENT", e)
			}
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if _, err := s.Submit(context.Background(), task.RunID, task.TaskID, task.Role, []byte(task.Contract.Example)); err == nil {
						t.Error("competing replay accepted")
					}
				}()
			}
			wg.Wait()
			_, replayed, e := s.head(task.RunID)
			if e != nil || !bytes.Equal(got, replayed) || !reflect.DeepEqual(files, snapshotSubmitStore(t, dir)) {
				t.Fatal("ASSERT_REPLAY_NO_REFUND_OR_EFFECTS", e)
			}
			if time.Now().UnixNano() >= before.Deadline {
				t.Fatal("expiry confounded replay")
			}
			t.Logf("ASSERT_SUBMIT_CAS_KILL: PASS stage=%s used=%d work=%d packets=%d cursor=%d protected=%d protectedPackets=%d", stage, after.Used, after.Work, after.Packets, after.CompletionCursor, ap, apk)
		})
	}
}

func snapshotSubmitStore(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	e := filepath.WalkDir(dir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(dir, path)
		out[rel] = raw
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
