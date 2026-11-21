//go:build (darwin || linux) && lsp_trace_controlled_describe_fixture

package applicationinventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/describerequest"
	is "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type resumeProcessResult struct {
	PID, Ordinal, Starts, Symbols int
	Status                        InventoryJobStatus
	Deadline                      int64
}

// Explicit helper. Cross-process inputs are only an owned temporary store path,
// returned RunID and closed test mode; no opaque operation or claim is passed.
func TestInventoryResumeProcess(t *testing.T) {
	n := len(os.Args)
	if n < 5 || os.Args[n-4] != "inventory-resume-v1" {
		t.Skip("explicit pipe-only child")
	}
	dir, run, mode := os.Args[n-3], os.Args[n-2], os.Args[n-1]
	tmp, e := filepath.EvalSymlinks(os.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	real, e := filepath.EvalSymlinks(dir)
	if e != nil || real != dir {
		t.Fatal("canonical temporary directory", e)
	}
	rel, e := filepath.Rel(tmp, dir)
	if e != nil || !filepath.IsLocal(rel) || rel == "." {
		t.Fatal("temporary child only")
	}
	info, e := os.Stat(dir)
	if e != nil || info.Mode().Perm() != 0700 {
		t.Fatal("private directory only")
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	s := DurableStore{Root: root, Directory: dir}
	ctx := context.Background()
	source := filepath.Join(filepath.Dir(dir), "source")
	var st InventoryJobStatus
	if mode != "first" {
		st, e = s.JobStatus(ctx, run)
		if e != nil {
			t.Fatal(e)
		}
	}
	if mode == "refuse" || mode == "corrupt" {
		if _, e = s.NativeDiscoveryBatchV1(ctx, run, st.Head); e == nil {
			t.Fatal("REOPEN_REGRANT")
		}
		t.Log("REOPEN_REFUSED: PASS")
		return
	}
	f, e := sessionruntime.NewControlledInventoryPipeFixture(source)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close(context.Background())
	if mode == "first" {
		cfg, e := ResumableAdmissionConfigV1(scopejob.Config{Workspace: source, Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: scopejob.DefaultBudget()}, ownedTestConfig())
		if e != nil {
			t.Fatal("config", e)
		}
		st, e = s.Create(ctx, cfg)
		if e != nil {
			t.Fatal("create", e)
		}
		st, e = s.Enumerate(ctx, st.RunID, st.Head)
		if e != nil {
			t.Fatal("enumerate", e)
		}
		run = st.RunID
	}
	shape, _, shapeErr := s.head(run)
	if shapeErr != nil {
		t.Fatal(shapeErr)
	}
	if mode == "first" {
		bad := shape
		bad.Input = shape.State
		if ownedHeadValid(bad) {
			t.Fatal("UNEXPECTED_INITIAL_FIELDS")
		}
		bad = shape
		bad.OutputBytes = 1
		if ownedHeadValid(bad) {
			t.Fatal("UNEXPECTED_INITIAL_FIELDS")
		}
	} else {
		terminal := shape
		terminal.Native, terminal.Selected = &shape.Checkpoint.Native, &shape.Checkpoint.Selected
		terminal.Checkpoint = nil
		terminal.Phase, terminal.Status = "Admitted", "PARTIAL"
		if ownedHeadValid(terminal) {
			t.Fatal("MISSING_TERMINAL_CHECKPOINT")
		}
		for _, change := range []func(*durableInventoryHead){func(h *durableInventoryHead) { h.Checkpoint = nil }, func(h *durableInventoryHead) { h.Used-- }, func(h *durableInventoryHead) { h.Work-- }, func(h *durableInventoryHead) { h.Sequence = 0 }, func(h *durableInventoryHead) { h.Schema = is.InventorySelectedHeadVersionV1 }, func(h *durableInventoryHead) { h.Input = h.Checkpoint.Evidence }} {
			bad := shape
			change(&bad)
			if ownedHeadValid(bad) {
				t.Fatal("INVALID_CHECKPOINT_SHAPE")
			}
		}
	}
	batch, e := s.NativeDiscoveryBatchV1(ctx, run, st.Head)
	if e != nil {
		t.Fatal("batch", e)
	}
	ordinal := 0
	want := 16
	if mode != "first" {
		ordinal = 1
		want = 1
	}
	if batch.Ordinal != ordinal || len(batch.Members) != want || ordinal == 1 && filepath.Base(batch.Members[0].Path) != "query-17.go" {
		t.Fatal("EXACT_REMAINING_MEMBER", batch)
	}
	c, e := f.Construction(ordinal, time.Now().Add(5*time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Members) != len(batch.Members) {
		t.Fatal("constructor membership")
	}
	for i := range c.Members {
		if c.Members[i] != batch.Members[i] {
			t.Fatalf("constructor member mismatch index=%d constructor=%+v frozen=%+v", i, c.Members[i], batch.Members[i])
		}
	}
	var starts atomic.Int32
	factory := c.NewStarter
	c.NewStarter = func() (sessionruntime.Starter, error) { starts.Add(1); return factory() }
	wrong := c
	wrong.Members = append([]sessionruntime.PrivateDiscoveryMember(nil), c.Members...)
	wrong.Members[0].URI += "-wrong"
	if _, e = s.ReserveNativeDiscoveryV1(ctx, run, st.Head, wrong); e == nil {
		t.Fatal("WRONG_MEMBERSHIP_ACCEPTED")
	}
	var permit is.InventoryNativePermitV1
	if mode == "compete" {
		var mu sync.Mutex
		var wg sync.WaitGroup
		winners := 0
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p, e := s.ReserveNativeDiscoveryV1(ctx, run, st.Head, c)
				if e == nil {
					mu.Lock()
					winners++
					permit = p
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if winners != 1 {
			t.Fatal("EXCLUSIVE_RESUME", winners)
		}
	} else {
		permit, e = s.ReserveNativeDiscoveryV1(ctx, run, st.Head, c)
		if e != nil {
			t.Fatal("reserve", e)
		}
	}
	if _, e = s.ReserveNativeDiscoveryV1(ctx, run, st.Head, c); e == nil {
		t.Fatal("STALE_RESUME_REPLAY")
	}
	pending, e := s.JobStatus(ctx, run)
	if e != nil || pending.Status != "PARTIAL_UNCERTAIN" {
		t.Fatal("pending", e, pending)
	}
	if _, e = s.NativeDiscoveryBatchV1(ctx, run, pending.Head); e == nil {
		t.Fatal("PENDING_REGRANT")
	}
	if mode == "pending" {
		if starts.Load() != 0 {
			t.Fatal("CLAIM_STARTED_PROVIDER")
		}
		raw, _ := json.Marshal(resumeProcessResult{os.Getpid(), ordinal, 0, 0, pending, c.Deadline.UnixNano()})
		t.Log("CLAIMED_NO_START: PASS")
		t.Log("RESUME_RESULT " + string(raw))
		return
	}
	d, e := is.OpenInventoryFundedDiscoveryV1(ctx, permit, c)
	if e != nil {
		t.Fatal("open", e)
	}
	count := 0
	queryURI := ""
	e = d.Execute(ctx, c, func(ctx context.Context, r *sessionruntime.PrivateDiscoverySession) error {
		for _, m := range c.Members {
			if e := r.Supply(ctx, m.URI, m.LanguageID); e != nil {
				return e
			}
			symbols, e := r.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: m.URI}})
			if e != nil {
				return e
			}
			if len(symbols) != 1 {
				return errors.New("missing genuine symbol")
			}
			count++
			if filepath.Base(m.Path) == "query-01.go" || filepath.Base(m.Path) == "query-17.go" {
				queryURI = m.URI
			}
		}
		refs, e := r.References(ctx, queryURI, lsp.Position{Line: 1, Character: 5})
		if e != nil {
			return e
		}
		if len(refs) != 1 || refs[0].URI == queryURI {
			return errors.New("missing genuine cross-file R")
		}
		return nil
	})
	if e != nil {
		t.Fatal("execute", e)
	}
	if e = d.Complete(ctx); e != nil {
		t.Fatal("complete", e)
	}
	index := -1
	owned := d.OwnedMethods()
	for i := 0; i < owned.GroupingQueryCount(); i++ {
		q, ok := owned.GroupingQuery(i)
		if ok && q.Method == "textDocument/references" && q.URI == queryURI {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("original reference selector")
	}
	st, e = s.JobStatus(ctx, run)
	if e != nil {
		t.Fatal(e)
	}
	config := inventorySelectedConfigTest(t, run, batch.DenominatorDigest)
	if mode == "fail" {
		_, e = s.continueSelectedAdmissionV1(ctx, run, st.Head, d, index, config, func(stage string) error {
			if stage == "before-source" {
				return errors.New("controlled second-batch source failure")
			}
			return nil
		})
		if e == nil {
			t.Fatal("SECOND_FAILURE_ADMITTED")
		}
		st, e = s.JobStatus(ctx, run)
		if e != nil || st.Status != "PARTIAL_UNCERTAIN" || st.Admissions != 1 {
			t.Fatal("SECOND_FAILURE_STATE", st, e)
		}
	} else {
		st, e = s.ContinueSelectedAdmissionV1(ctx, run, st.Head, d, index, config)
		if e != nil {
			observed, statusErr := s.JobStatus(ctx, run)
			t.Fatalf("selected admit: %v status=%+v statusErr=%v files=%+v", e, observed, statusErr, retentionPreflightFiles(t, dir))
		}
		if st.Admissions != ordinal+1 || ordinal == 0 && (st.Phase != "ReadyForAcquisition" || st.Status != "READY" || st.Checkpoint == nil) || ordinal == 1 && (st.Phase != "Admitted" || st.Status != "PARTIAL") {
			t.Fatal("ADMISSION_CHECKPOINT", st)
		}
		h, _, err := s.head(run)
		if err != nil {
			t.Fatal(err)
		}
		nodeRaw, err := publication.ReadObject(root, "inventory-admission-", h.Input)
		var node ownedAdmissionRecord
		if err != nil || strictInventory(nodeRaw, &node) != nil {
			t.Fatal("retained node", err)
		}
		evidence, err := publication.ReadObject(root, "inventory-evidence-", node.Evidence)
		var held struct {
			Accounting is.CompletedSelectedAccountingV1
			Unit       tp.PrivateSourceUnit
			Request    describerequest.Record
		}
		if err != nil || json.Unmarshal(evidence, &held) != nil || held.Accounting.URI != queryURI || held.Accounting.Windows != 3 || held.Accounting.NativeDeadline != c.Deadline.UnixNano() || held.Unit.ID != node.Binding.SourceUnitID || held.Request.RecordID != node.Binding.RequestID || held.Unit.SessionID != owned.GroupingSession().SessionID || describerequest.ValidateRecord(held.Request) != nil {
			t.Fatal("ORIGINAL_IDENTITIES", err)
		}
		originalPath := ""
		for _, m := range c.Members {
			if m.URI == queryURI {
				originalPath = m.Path
			}
		}
		original, err := os.ReadFile(originalPath)
		if err != nil || cat.NativeDigestV3(original) != held.Unit.SourceDigest || held.Request.Envelope.AdmissionID != held.Unit.ID {
			t.Fatal("ORIGINAL_SOURCE_BINDING", err)
		}
		t.Logf("ORIGINAL_IDENTITIES ordinal=%d request=%s source=%s session=%s nativeDeadline=%d windows=%d", ordinal, held.Request.RecordID, held.Unit.SourceDigest, held.Unit.SessionID, held.Accounting.NativeDeadline, held.Accounting.Windows)
		if ordinal == 1 && (node.Previous != h.Checkpoint.Admission || held.Accounting == h.Checkpoint.Selected.Accounting) {
			t.Fatal("SECOND_ADMISSION_CHAIN")
		}
		if _, e = s.ReserveAdmission(ctx, run, st.Head, node.Binding); e == nil {
			t.Fatal("PUBLIC_ADMISSION_REGRANT")
		}
		if _, e = s.ContinueSelectedAdmissionV1(ctx, run, st.Head, d, index, config); e == nil {
			t.Fatal("DUPLICATE_ADMISSION")
		}
	}
	if _, e = s.Seal(ctx, run, st.Head); e == nil {
		t.Fatal("SEAL_AVAILABLE")
	}
	if starts.Load() != 1 || count != want {
		t.Fatal("GENUINE_ACQUISITION_COUNTS", starts.Load(), count)
	}
	if e = f.Close(ctx); e != nil {
		t.Fatal("fixture cleanup", e)
	}
	files, e := os.ReadDir(source)
	if e != nil || len(files) != 0 {
		t.Fatal("fixture remainder", e, len(files))
	}
	if e = root.Close(); e != nil {
		t.Fatal("parent cleanup", e)
	}
	raw, _ := json.Marshal(resumeProcessResult{os.Getpid(), ordinal, int(starts.Load()), count, st, c.Deadline.UnixNano()})
	t.Log("RESUME_RESULT " + string(raw))
}

func runResumeProcess(t *testing.T, dir, run, mode string) (resumeProcessResult, string) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestInventoryResumeProcess$", "-test.count=1", "-test.timeout=20s", "-test.v", "--", "inventory-resume-v1", dir, run, mode)
	cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH"), "TMPDIR=" + os.Getenv("TMPDIR"), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off"}
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("child %s: %v\n%s", mode, e, out)
	}
	t.Logf("CHILD_PROCESS mode=%s output=%s", mode, out)
	var r resumeProcessResult
	for _, line := range strings.Split(string(out), "\n") {
		if _, v, ok := strings.Cut(line, "RESUME_RESULT "); ok {
			if e = json.Unmarshal([]byte(v), &r); e != nil {
				t.Fatal(e)
			}
		}
	}
	return r, string(out)
}

func TestInventoryFreshProcessResume(t *testing.T) {
	for _, mode := range []string{"success", "compete", "fail", "pending", "corrupt"} {
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
			first, out := runResumeProcess(t, dir, "", "first")
			if first.PID == 0 || first.PID == os.Getpid() || first.Status.Checkpoint == nil {
				t.Fatal("FIRST_PROCESS", out)
			}
			before := retentionPreflightFiles(t, dir)
			if mode == "success" {
				delay := time.Until(time.Unix(0, first.Deadline))
				if delay > 0 {
					time.Sleep(delay + time.Millisecond)
				}
			}
			if mode == "corrupt" {
				cp := first.Status.Checkpoint
				path := filepath.Join(dir, "inventory-evidence-"+cp.Evidence.ID+".json")
				if e = os.WriteFile(path, []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
				_, out = runResumeProcess(t, dir, first.Status.RunID, "corrupt")
				if !strings.Contains(out, "REOPEN_REFUSED: PASS") {
					t.Fatal(out)
				}
				files := retentionPreflightFiles(t, dir)
				raw, _ := json.Marshal(files)
				var total int64
				for _, f := range files {
					total += f.Bytes
				}
				t.Logf("FINAL_RESUME_FILES bytes=%d manifest=%s", total, raw)
				return
			}
			second, out := runResumeProcess(t, dir, first.Status.RunID, mode)
			if mode == "pending" || mode == "fail" {
				_, refused := runResumeProcess(t, dir, first.Status.RunID, "refuse")
				if !strings.Contains(refused, "REOPEN_REFUSED: PASS") {
					t.Fatal(refused)
				}
			}
			if second.PID == 0 || second.PID == first.PID || second.Status.RunID != first.Status.RunID || second.Status.Used <= first.Status.Used || second.Status.Limit != first.Status.Limit {
				t.Fatal("FRESH_CUMULATIVE_IDENTITY", out)
			}
			if !reflect.DeepEqual(second.Status.Checkpoint, first.Status.Checkpoint) || !reflect.DeepEqual(second.Status.Enumeration, first.Status.Enumeration) || second.Deadline <= first.Deadline {
				t.Fatal("FROZEN_CHECKPOINT_OR_DENOMINATOR_CHANGED")
			}
			after := retentionPreflightFiles(t, dir)
			m := map[string]retentionPreflightFile{}
			for _, f := range after {
				m[f.Path] = f
			}
			for _, f := range before {
				if f.Path == inventoryHeadName(first.Status.RunID) {
					continue
				}
				if m[f.Path] != f {
					t.Fatal("FIRST_EVIDENCE_CHANGED", f.Path)
				}
			}
			if mode == "success" || mode == "compete" {
				if second.Ordinal != 1 || second.Symbols != 1 || second.Starts != 1 || second.Status.Admissions != 2 {
					t.Fatal("CONNECTED_16_PLUS_1", out)
				}
				_, refused := runResumeProcess(t, dir, first.Status.RunID, "refuse")
				if !strings.Contains(refused, "REOPEN_REFUSED: PASS") {
					t.Fatal(refused)
				}
			}
			admissions, evidences := 0, 0
			for _, f := range after {
				if strings.HasPrefix(f.Path, "inventory-admission-") {
					admissions++
				}
				if strings.HasPrefix(f.Path, "inventory-evidence-") {
					evidences++
				}
			}
			wantAdmissions := 1
			if mode == "success" || mode == "compete" {
				wantAdmissions = 2
			}
			if admissions != wantAdmissions || evidences != wantAdmissions {
				t.Fatal("EXACT_RETENTION_COUNT", admissions, evidences)
			}
			raw, _ := json.Marshal(after)
			var bytes int64
			for _, f := range after {
				bytes += f.Bytes
			}
			t.Logf("FRESH_16_PLUS_1 mode=%s firstPID=%d secondPID=%d firstH=%d finalH=%d limit=%d", mode, first.PID, second.PID, first.Status.Used, second.Status.Used, first.Status.Limit)
			t.Logf("FINAL_RESUME_FILES bytes=%d manifest=%s", bytes, raw)
		})
	}
}

func TestInventoryResumeProfile(t *testing.T) {
	expected := (uint64(64) << 20) - is.InventoryNativeControlBytesV1 + 2*cat.InventoryNativeControlBytesV2 + 2*(cat.InventorySelectedControlBytesV1+cat.InventoryAdmissionBytesV4+cat.InventoryCheckpointBytesV1)
	if cat.InventoryResumeLimitV1 != expected {
		t.Fatal("TWO_BATCH_FORMULA")
	}
	native, selected := is.NewInventoryNativeProfileV1(), is.NewInventorySelectedProfileV1()
	var total [14]uint64
	for i := range total {
		sum := native.Allocation[i] + selected.Allocation[i]
		total[i] = 2 * sum
		if sum < native.Allocation[i] || total[i] < sum {
			t.Fatal("NATIVE_OVERFLOW")
		}
	}
	p := is.NewInventoryResumeProfileV1()
	if !p.Valid() || p.CumulativeNative != total || p.ParentLimit != expected {
		t.Fatal("CLOSED_RESUME_PROFILE")
	}
	for _, change := range []func(*is.InventoryResumeProfileV1){func(p *is.InventoryResumeProfileV1) { p.ParentLimit++ }, func(p *is.InventoryResumeProfileV1) { p.Batches++ }, func(p *is.InventoryResumeProfileV1) { p.Checkpoint++ }, func(p *is.InventoryResumeProfileV1) { p.CumulativeNative[0]++ }} {
		bad := p
		change(&bad)
		if bad.Valid() {
			t.Fatal("RESUME_PROFILE_TAMPER")
		}
	}
	t.Logf("RESUME_NATIVE_ALLOCATION discovery=%v selected=%v cumulative=%v", native.Allocation, selected.Allocation, total)
	if cat.InventoryResumeLimitV1 <= cat.InventoryAdmissionLimitV4 || cat.InventoryNativeControlBytesV2 <= is.InventoryNativeControlBytesV1 {
		t.Fatal("INITIAL_PROFILE")
	}
	b, e := cat.NewInventoryAdmissionBudgetV5(time.Now().Add(time.Second), cat.InventoryAdmissionBytesV4, cat.InventoryResumeLimitV1-cat.InventoryAdmissionBytesV4)
	if e != nil {
		t.Fatal(e)
	}
	used, _ := b.Usage()
	if used != cat.InventoryResumeLimitV1-cat.InventoryAdmissionBytesV4 {
		t.Fatal("CUMULATIVE_SPEND_RESET")
	}
	if _, e = cat.NewInventoryAdmissionBudgetV4(time.Now().Add(time.Second), cat.InventoryAdmissionBytesV4, used); e == nil {
		t.Fatal("LEGACY_PROFILE_ENLARGED")
	}
	for _, args := range []struct {
		deadline         time.Time
		allowance, spent uint64
	}{{time.Now().Add(-time.Second), cat.InventoryAdmissionBytesV4, 0}, {time.Now().Add(time.Second), 1, 0}, {time.Now().Add(time.Second), cat.InventoryAdmissionBytesV4, cat.InventoryResumeLimitV1}} {
		if _, e := cat.NewInventoryAdmissionBudgetV5(args.deadline, args.allowance, args.spent); e == nil {
			t.Fatal("INVALID_RESUME_BUDGET")
		}
	}
	t.Log(fmt.Sprintf("RESUME_PROFILE H=%d nativeControl=%d selectedControl=%d admission=%d checkpoint=%d work=%d", cat.InventoryResumeLimitV1, cat.InventoryNativeControlBytesV2, cat.InventorySelectedControlBytesV1, cat.InventoryAdmissionBytesV4, cat.InventoryCheckpointBytesV1, cat.InventoryResumeLimitV1*32))
}
