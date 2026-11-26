//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func cliQuota(t *testing.T, c *publicCLI, v publicOwnerReply) *publication.StorageStatus {
	t.Helper()
	start := time.Now()
	raw := c.call(v.Operations["status"], nil, true)
	var result struct{ Storage *publication.StorageStatus }
	if json.Unmarshal(raw, &result) != nil || result.Storage == nil {
		t.Fatal("missing quota status")
	}
	t.Logf("STATUS_OBSERVATION_NS %d", time.Since(start).Nanoseconds())
	b, _ := json.Marshal(result.Storage)
	t.Log("CLI_STORAGE_MEASUREMENT " + string(b))
	return result.Storage
}
func quotaFixtureGrowth(t *testing.T, store string, n int, name string) {
	t.Helper()
	r, e := publication.OpenRoot(store)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	raw := bytes.Repeat([]byte("controlled retained fixture growth; not semantic evidence\n"), n/56+1)
	raw = raw[:n]
	if _, e = publication.PublishBoundFile(r, name, raw, func(b []byte) error {
		if !bytes.Equal(b, raw) {
			return fmt.Errorf("growth changed")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	t.Logf("CONTROLLED_RETAINED_GROWTH bytes=%d sha256=%x", len(raw), sha256.Sum256(raw))
}
func fillQuota(t *testing.T, store string) { fillQuotaTo(t, store, 0) }
func fillQuotaTo(t *testing.T, store string, remaining uint64) {
	t.Helper()
	r, e := publication.OpenRoot(store)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	for i := 0; i < 100; i++ {
		v, e := publication.ReadStorageStatus(context.Background(), r)
		if e != nil {
			t.Fatal(e)
		}
		if v.AvailableBytes < remaining+512 {
			return
		}
		n := (v.AvailableBytes - remaining) / 2
		if n > 65536 {
			n = 65536
		}
		raw := bytes.Repeat([]byte("g"), int(n))
		if _, e = publication.PublishBoundFile(r, fmt.Sprintf("controlled-growth-%03d", i), raw, func([]byte) error { return nil }); e != nil {
			t.Fatal(e)
		}
	}
	t.Fatal("bounded fill failed")
}
func quotaPrefix(t *testing.T, store string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	e := filepath.WalkDir(store, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type().IsRegular() && !strings.HasPrefix(d.Name(), ".") {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			out[p] = sha256.Sum256(b)
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func assertQuotaPrefix(t *testing.T, prefix map[string][32]byte) {
	t.Helper()
	for p, d := range prefix {
		b, e := os.ReadFile(p)
		if e != nil || sha256.Sum256(b) != d {
			t.Fatal("ACCEPTED_PREFIX_CHANGED", p, e)
		}
	}
}
func TestCensusStorageQuotaPublication(t *testing.T) {
	c := newPublicCLI(t)
	v := c.start("--storage-bytes", "2097152")
	before := cliQuota(t, c, v)
	if before.Limit != 2<<20 {
		t.Fatal("explicit capacity")
	}
	quotaFixtureGrowth(t, v.Store, 512<<10, "controlled-retained-growth.bin")
	v = c.acquire(c.acquire(v))
	task := c.task(v.Operations["seal"], nil)
	stages := 0
	for task.Phase != "Published" {
		input := c.input(task)
		answer := []byte(task.Contract.Example)
		if task.Phase != "Describe" {
			var x struct{ Revision string }
			if json.Unmarshal(input, &x) != nil {
				t.Fatal("input")
			}
			answer = bytes.ReplaceAll(answer, []byte("CURRENT_INPUT_REVISION"), []byte(x.Revision))
		}
		task = c.task(task.Contract.Operations["submit"], answer)
		stages++
	}
	raw := c.call(task.Contract.Operations["result"], nil, true)
	var result struct {
		Published, Accepted               bool
		Authority, SuccessfulDescriptions int
	}
	if json.Unmarshal(raw, &result) != nil || !result.Published || result.Accepted || result.Authority != 0 || result.SuccessfulDescriptions != 2 || stages != 4 {
		t.Fatal("publication qualification")
	}
	end := cliQuota(t, c, v)
	if end.FileBytes < 512<<10 || end.FileBytes+end.ReservedBytes+end.ControlReserve > end.Limit || end.PeakReservedBytes+end.ControlReserve > end.Limit || end.SampledPeakFileBytes < end.FileBytes {
		t.Fatal("CAPACITY_ACCOUNTING", end)
	}
	var summed uint64
	filepath.WalkDir(v.Store, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type().IsRegular() {
			s, e := d.Info()
			if e != nil {
				return e
			}
			summed += uint64(s.Size())
		}
		return nil
	})
	if summed != end.FileBytes {
		t.Fatal("FILE_LENGTH_CENSUS_DISAGREES", summed, end.FileBytes)
	}
	t.Logf("QUOTA_PUBLICATION_COMPLETE semantic_stages=%d independent_judgments=0 owned_content_bytes=%d allocated_block_sum=%d", stages, end.FileBytes, end.AllocatedBytes)
}
func TestCensusStorageQuotaExhaustion(t *testing.T) {
	for _, phase := range []string{"batch2", "semantic"} {
		t.Run(phase, func(t *testing.T) {
			c := newPublicCLI(t)
			v := c.acquire(c.start("--storage-bytes", "1048576"))
			var task publicTaskReply
			if phase == "semantic" {
				v = c.acquire(v)
				task = c.task(v.Operations["seal"], nil)
				c.input(task)
			}
			fillQuota(t, v.Store)
			prefix := quotaPrefix(t, v.Store)
			before := cliQuota(t, c, v)
			if phase == "batch2" {
				c.call(v.Operations["acquire"], nil, false)
			} else {
				c.call(task.Contract.Operations["submit"], []byte(task.Contract.Example), false)
			}
			assertQuotaPrefix(t, prefix)
			after := cliQuota(t, c, v)
			if after.Limit != before.Limit || after.FileBytes != before.FileBytes || after.AvailableBytes != before.AvailableBytes {
				t.Fatal("quota exhaustion enlarged/mutated accepted storage", before, after)
			}
			if phase == "batch2" {
				var current publicOwnerReply
				if json.Unmarshal(c.call(v.Operations["status"], nil, true), &current) != nil || current.Admissions != 1 || current.Head != v.Head {
					t.Fatal("FIRST_ADMISSION_NOT_PRESERVED")
				}
			}
			t.Logf("QUOTA_EXHAUSTED_ACCEPTED_PREFIX_PRESERVED phase=%s files=%d available=%d", phase, len(prefix), after.AvailableBytes)
		})
	}
}
func TestCensusStorageQuotaSpentSemantic(t *testing.T) {
	c := newPublicCLI(t)
	v := c.acquire(c.acquire(c.start("--storage-bytes", "1048576")))
	task := c.task(v.Operations["seal"], nil)
	c.input(task)
	fillQuotaTo(t, v.Store, 12000)
	prefix := quotaPrefix(t, v.Store)
	delete(prefix, filepath.Join(v.Store, "inventory-run-"+v.RunID+".json"))
	before := cliQuota(t, c, v)
	c.call(task.Contract.Operations["submit"], []byte(task.Contract.Example), false)
	current := c.task(task.Contract.Operations["current_task"], nil)
	if current.Status != "UNCERTAIN_SPENT" || current.TaskID != task.TaskID || current.DeadlineUnixNano != task.DeadlineUnixNano {
		t.Fatal("SEMANTIC_EXHAUSTION_REGRANTED", current)
	}
	assertQuotaPrefix(t, prefix)
	after := cliQuota(t, c, v)
	if after.Limit != before.Limit || after.FileBytes+after.ReservedBytes+after.ControlReserve > after.Limit {
		t.Fatal("semantic quota enlarged", after)
	}
	c.call(task.Contract.Operations["submit"], []byte(task.Contract.Example), false)
	t.Log("SEMANTIC_MID_COMPLETION_EXHAUSTION_SPENT_NO_REPLAY")
}
func TestCensusStorageQuotaConcurrentCommands(t *testing.T) {
	a, b := newPublicCLI(t), newPublicCLI(t)
	b.store = a.store
	va := a.start("--storage-bytes", "2097152")
	vb := b.start("--storage-bytes", "2097152")
	if va.Store != vb.Store || va.RunID == vb.RunID {
		t.Fatal("shared store setup")
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); va = a.acquire(va) }()
	go func() { defer wg.Done(); vb = b.acquire(vb) }()
	wg.Wait()
	if va.Admissions != 1 || vb.Admissions != 1 {
		t.Fatal("concurrent controlled CLI acquisition")
	}
	sa, sb := cliQuota(t, a, va), cliQuota(t, b, vb)
	if *sa != *sb || sa.FileBytes+sa.ReservedBytes+sa.ControlReserve > sa.Limit || sa.PeakReservedBytes+sa.ControlReserve > sa.Limit {
		t.Fatal("CLI_CROSS_PROCESS_QUOTA_DISAGREEMENT", sa, sb)
	}
	t.Log("TWO_DISTINCT_CLI_JOBS_SHARED_QUOTA_CONCURRENT_FIRST_BATCHES")
}
func TestCensusStorageQuotaConfig(t *testing.T) {
	c := newPublicCLI(t)
	v := c.start()
	if cliQuota(t, c, v).Limit != publication.DefaultStorageBytes {
		t.Fatal("DEFAULT_NOT_2GIB")
	}
	before := repairTree(t, v.Store)
	c.call([]string{"lsp-trace", "census", ".", "--publication-root", c.store, "--storage-bytes", "4194304"}, nil, false)
	if repairTree(t, v.Store) != before {
		t.Fatal("POLICY_MISMATCH_MUTATED_STORE")
	}
	other := newPublicCLI(t)
	other.call([]string{"lsp-trace", "census", ".", "--publication-root", other.store, "--storage-bytes", "1"}, nil, false)
}
