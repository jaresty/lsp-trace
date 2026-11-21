//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	dw "lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/publication"
)

func chunkFixture(t *testing.T, raw []byte, tight bool) (DurableStore, durableInventoryHead) {
	t.Helper()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	s := DurableStore{Root: root, Directory: dir}
	ref, e := publication.StoreObject(root, "inventory-catalog-", raw)
	if e != nil {
		t.Fatal(e)
	}
	state, e := publication.StoreObject(root, "inventory-state-", []byte(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	h := durableInventoryHead{Schema: durableHeadVersion, RunID: strings.Repeat("a", 64), TaskID: strings.Repeat("b", 64), Phase: "GroupDecision", Status: "READY", Sequence: 1, Input: ref, State: state, InputPrefix: "inventory-catalog-", Role: "interpreter", Actor: "a", Issued: now.UnixNano(), RunDeadline: now.Add(180 * time.Second).UnixNano(), Deadline: now.Add(30 * time.Second).UnixNano(), Window: int64(30 * time.Second), SemanticNS: int64(30 * time.Second), Limit: durableRunLimit}
	h.Completion, e = quoteDurableCompletion(nil, 2, 0, dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := sc.NewCallerBudgetV3(time.Unix(0, h.RunDeadline), durableRunLimit)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.buildDelivery(context.Background(), &h, b); e != nil {
		t.Fatal(e)
	}
	h.Used, _ = b.Usage()
	h.Work = h.Used * 32
	h.Completion.PreparationUsed = h.Used
	if tight {
		// Independent oracle: each original byte is verified once (6x) and copied
		// once (2x); every canonical page also verifies the index and pays overhead.
		n := 8*uint64(len(raw)) + uint64((len(raw)+32767)/32768)*(8*uint64(h.Delivery.Index.Bytes)+16384)
		for _, stage := range h.Completion.Stages {
			n += stage
		}
		h.Limit = h.Used + n
	}
	h.Acceptance = "inventory-accepted-" + h.RunID + "-" + h.TaskID
	h.AcceptanceDeadline = h.Deadline
	if _, e = s.change(context.Background(), nil, h); e != nil {
		t.Fatal(e)
	}
	if e = publication.PublishTimedCompletion(context.Background(), root, h.Acceptance, inventoryAcceptance(h), time.Unix(0, h.AcceptanceDeadline)); e != nil {
		t.Fatal(e)
	}
	return s, h
}
func TestChunkBoundariesAndVariableOffsets(t *testing.T) {
	for _, n := range []int{32767, 32768, 32769, 98309} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			raw := bytes.Repeat([]byte("€世界"), n/9+1)[:n]
			s, h := chunkFixture(t, raw, false)
			for _, span := range [][2]int{{0, 1}, {min(n-1, 32767), 2}, {min(n-1, 32760), 32768}, {n - 1, 32768}} {
				p, e := s.Read(context.Background(), h.RunID, h.TaskID, "input", span[0], span[1])
				if e != nil || !bytes.Equal(p.Data, raw[span[0]:min(n, span[0]+span[1])]) {
					t.Fatal("ASSERT_CHUNK_RANGE", span, e)
				}
			}
			var got []byte
			for off := 0; off < n; off += 32768 {
				p, e := s.Read(context.Background(), h.RunID, h.TaskID, "input", off, 32768)
				if e != nil {
					t.Fatal(e)
				}
				got = append(got, p.Data...)
			}
			if !bytes.Equal(got, raw) {
				t.Fatal("ASSERT_CHUNK_RECONSTRUCTION")
			}
			next, _, e := s.head(h.RunID)
			if e != nil || next.Delivery.Next != n || next.CompletionCursor != 0 {
				t.Fatal("ASSERT_READ_CURSOR", e)
			}
			t.Log("ASSERT_CHUNK_RECONSTRUCTION: PASS; arbitrary UTF-8 byte splits")
		})
	}
}
func TestChunkOptionalStarvationAndDuplicate(t *testing.T) {
	raw := bytes.Repeat([]byte("x"), 70000)
	s, h := chunkFixture(t, raw, true)
	original, _, _ := s.head(h.RunID)
	for _, span := range [][2]int{{0, 1}, {1, 32768}, {32768, 32768}} {
		if _, e := s.Read(context.Background(), h.RunID, h.TaskID, "input", span[0], span[1]); e == nil {
			t.Fatal("optional stole protected tail")
		}
	}
	for off := 0; off < len(raw); off += 32768 {
		if _, e := s.Read(context.Background(), h.RunID, h.TaskID, "input", off, 32768); e != nil {
			t.Fatal("required tail denied", e)
		}
	}
	if _, e := s.Read(context.Background(), h.RunID, h.TaskID, "input", 0, 32768); e == nil {
		t.Fatal("duplicate free")
	}
	after, _, e := s.head(h.RunID)
	if e != nil || after.Delivery.Next != len(raw) || after.Packets != 3 || after.CompletionCursor != original.CompletionCursor || after.Used != original.Used+after.DeliverySpent {
		t.Fatal("reservation conservation", e)
	}
	n, _, _ := after.protectedCapacity()
	if after.Used+n != after.Limit {
		t.Fatal("tail refund")
	}
	t.Log("ASSERT_OPTIONAL_TAIL: PASS")
}
func TestChunkConcurrentFirstReservation(t *testing.T) {
	s, h := chunkFixture(t, bytes.Repeat([]byte("x"), 65536), true)
	var wins atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, e := s.Read(context.Background(), h.RunID, h.TaskID, "input", 0, 32768); e == nil {
				wins.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	after, _, e := s.head(h.RunID)
	if e != nil || wins.Load() != 1 || after.Delivery.Next != 32768 || after.Packets != 1 {
		t.Fatal("ASSERT_ONE_RESERVATION", wins.Load(), e)
	}
	if _, e := s.Read(context.Background(), h.RunID, h.TaskID, "input", 32768, 32768); e != nil {
		t.Fatal("tail", e)
	}
	t.Log("ASSERT_ONE_RESERVATION: PASS")
}
func TestChunkCorruptionRefusesAndSpends(t *testing.T) {
	for _, mode := range []string{"missing", "bytes", "index", "cross-task", "order", "truncated", "head-substitution"} {
		t.Run(mode, func(t *testing.T) {
			raw := append(bytes.Repeat([]byte("a"), 32768), bytes.Repeat([]byte("b"), 32768)...)
			s, h := chunkFixture(t, raw, false)
			idx, e := s.readChunkIndex(h)
			if e != nil {
				t.Fatal(e)
			}
			indexPath := filepath.Join(s.Directory, "inventory-chunk-index-"+h.Delivery.Index.ID+".json")
			switch mode {
			case "missing":
				e = os.Remove(filepath.Join(s.Directory, "inventory-chunk-"+idx.Chunks[0].ID+".json"))
			case "bytes":
				e = os.WriteFile(filepath.Join(s.Directory, "inventory-chunk-"+idx.Chunks[0].ID+".json"), bytes.Repeat([]byte("c"), 32768), 0600)
			case "index":
				e = os.WriteFile(indexPath, []byte(`{}`), 0600)
			default:
				if mode == "cross-task" {
					idx.TaskID = strings.Repeat("c", 64)
				} else if mode == "order" {
					idx.Chunks[0], idx.Chunks[1] = idx.Chunks[1], idx.Chunks[0]
				} else {
					idx.Chunks = idx.Chunks[:1]
				}
				encoded, _ := json.Marshal(idx)
				if mode == "head-substitution" {
					ref, err := publication.StoreObject(s.Root, "inventory-chunk-index-", encoded)
					if err != nil {
						t.Fatal(err)
					}
					h.Delivery.Index = ref
					sealed, _ := encodeInventoryHead(h)
					e = os.WriteFile(filepath.Join(s.Directory, inventoryHeadName(h.RunID)), sealed, 0600)
				} else {
					e = os.WriteFile(indexPath, encoded, 0600)
				}
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.Read(context.Background(), h.RunID, h.TaskID, "input", 0, 32768); e == nil {
				t.Fatal("ASSERT_CORRUPTION_REFUSAL", mode)
			}
			if mode != "head-substitution" {
				after, _, e := s.head(h.RunID)
				if e != nil || after.Delivery.Next != 32768 || after.Used <= h.Used {
					t.Fatal("failed verification refunded", e)
				}
			}
			t.Log("ASSERT_CORRUPTION_REFUSAL: PASS")
		})
	}
}
func TestChunkReadDoesNotReopenOriginal(t *testing.T) {
	s, h := chunkFixture(t, bytes.Repeat([]byte("x"), 32769), false)
	if e := os.Remove(filepath.Join(s.Directory, "inventory-catalog-"+h.Input.ID+".json")); e != nil {
		t.Fatal(e)
	}
	for off := 0; off < h.Input.Bytes; off += 32768 {
		if _, e := s.Read(context.Background(), h.RunID, h.TaskID, "input", off, 32768); e != nil {
			t.Fatal("page reopened original", e)
		}
	}
	// This isolates delivery only: semantic consumers still require the full object.
	t.Log("ASSERT_NO_FULL_OBJECT_REREAD: PASS")
}

func TestChunkStagedAdmissionRefusesBeforeAnswerEffects(t *testing.T) {
	s, h := chunkFixture(t, []byte("input"), true)
	before, e := os.ReadDir(s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	_, old, e := s.head(h.RunID)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, _, e = s.claimSubmit(context.Background(), h, old, 65536); e == nil {
		t.Fatal("unfunded next delivery admitted")
	}
	after, _, e := s.head(h.RunID)
	if e != nil || after.Status != "CLAIMED" || after.CompletionCursor != 1 || after.Used != h.Used+h.Completion.Stages[0] || after.Packets != 1 {
		t.Fatal("preflight failure not conservatively spent", e)
	}
	files, e := os.ReadDir(s.Directory)
	if e != nil || len(files) != len(before) {
		t.Fatal("answer objects written before admission", e)
	}
	t.Log("ASSERT_ADMISSION_BEFORE_ANSWER_EFFECTS: PASS")
}

func TestChunkInvalidQuotes(t *testing.T) {
	for _, v := range [][4]int{{0, 1, 0, 1}, {8<<20 + 1, 1, 0, 1}, {1, 0, 0, 1}, {1, 999999, 0, 1}, {1, 1, -1, 1}, {1, 1, 1, 1}, {1, 1, 0, 0}, {1, 1, 0, 32769}} {
		if _, _, e := chunkPageQuote(v[0], v[1], v[2], v[3]); e == nil {
			t.Fatal("invalid quote", v)
		}
	}
}
func TestChunkKilledAfterReservation(t *testing.T) {
	if dir := os.Getenv("LSP_CHUNK_KILL_ROOT"); dir != "" {
		root, e := publication.OpenRoot(dir)
		if e != nil {
			t.Fatal(e)
		}
		s := DurableStore{Root: root, Directory: dir}
		h, old, e := s.head(strings.Repeat("a", 64))
		if e != nil {
			t.Fatal(e)
		}
		if _, _, e = s.reserveChunkRead(context.Background(), h, old, 0, 32768); e != nil {
			t.Fatal(e)
		}
		syscall.Kill(os.Getpid(), syscall.SIGKILL)
		t.Fatal("kill returned")
		return
	}
	s, h := chunkFixture(t, bytes.Repeat([]byte("x"), 65536), true)
	cmd := exec.Command(os.Args[0], "-test.run=^TestChunkKilledAfterReservation$")
	cmd.Env = append(os.Environ(), "LSP_CHUNK_KILL_ROOT="+s.Directory)
	out, e := cmd.CombinedOutput()
	if e == nil {
		t.Fatal("child survived", string(out))
	}
	ee, ok := e.(*exec.ExitError)
	if !ok || ee.ProcessState.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		t.Fatal("wrong child failure", e, string(out))
	}
	after, _, e := s.head(h.RunID)
	if e != nil || after.Delivery.Next != 32768 || after.Packets != 1 || after.Used <= h.Used {
		t.Fatal("kill refunded", e)
	}
	task, e := s.Task(context.Background(), h.RunID, "")
	if e != nil || task.RequiredInputNextOffset != 32768 || task.Status != "PENDING" {
		t.Fatal("lost response observation", e)
	}
	if _, e = s.Read(context.Background(), h.RunID, h.TaskID, "input", 0, 32768); e == nil {
		t.Fatal("lost response got free retry")
	}
	if _, e = s.Read(context.Background(), h.RunID, h.TaskID, "input", 32768, 32768); e != nil {
		t.Fatal("required tail lost", e)
	}
	t.Log("ASSERT_KILL_RESERVATION_SPENT: PASS")
}
