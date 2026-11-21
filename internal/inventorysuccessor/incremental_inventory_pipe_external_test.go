//go:build lsp_trace_controlled_describe_fixture && (darwin || linux)

package inventorysuccessor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	app "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	so "lsp-trace/internal/inventorysuccessor/sourceoperator"
	pa "lsp-trace/internal/programcadmission"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Only fixed in-memory pipe traffic; the actual producer mints all authority.
func incrementalPipeSelected(t *testing.T, id string) sc.PrivateSelectedResultV2 {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	f, e := sessionruntime.NewControlledDescribePipeFixture(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if e := f.Close(ctx); e != nil {
			t.Error(e)
		}
		s := f.CloseSnapshot()
		if !s.PipeJoined || !s.PhysicalCloseOK || f.Manager().Census().Workers != 0 || f.Manager().Census().Sessions != 0 {
			t.Error("pipe cleanup", s)
		}
	})
	owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
	deadline := time.Now().Add(12 * time.Second)
	q := inv.PrivateMixedInputRequestV2{Manager: f.Manager(), Root: root, Start: f.StartRequest(deadline), URI: f.URI(), OccurrenceID: "incremental-query-" + id, Revision: "literal-revision", Privacy: "private-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 5, Kinds: []pa.RelationKind{pa.TypedReferencesSymbol}}
	d := "sha256:" + strings.Repeat("1", 64)
	input := cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "controlled-source", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, e := cc.NewContinuationContract(input)
	if e != nil {
		t.Fatal(e)
	}
	hash := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
	c := inv.PrivateSelectedConfigurationV2{RunID: "pipe-" + id, SeedNamespace: "pipe-incremental", Lineage: "controlled-" + id, Mode: "R", Seed: 7, Attempts: [3]string{hash(id + "-M"), hash(id + "-P"), hash(id + "-S")}, Contract: contract, ContractInput: input, SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}}
	r, e := so.PrepareSelectedRoleV3(context.Background(), owner, q, c, sc.PrivateSelectedRoleV3{Kind: "REFERENCES_SYMBOL", Role: "REFERENCED_SYMBOL"})
	if e != nil || !r.Available() {
		t.Fatal("pipe selected R", e)
	}
	if f.Manager().Census().Workers != 0 {
		t.Fatal("worker remained")
	}
	return r
}
func incrementalStore(t *testing.T) app.DurableStore {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := root.Close(); e != nil {
			t.Error(e)
		}
	})
	return app.DurableStore{Root: root, Directory: dir}
}
func incrementalConfig() app.OwnedInventoryConfig {
	pin := "sha256:" + strings.Repeat("1", 64)
	return app.OwnedInventoryConfig{Scope: "controlled fixed pipe", Provenance: "not enumeration", Interpreter: "interpreter", Policy: sc.DurableDescribePolicyV1{ReviewMode: cat.ReviewModeSubagentsV3, Reviewer: "reviewer", RolePrompts: [2]string{"describe source", "review source"}}, Pins: dw.HostPinsV2{WorkerSHA256: pin, ModelSHA256: pin, GrammarSHA256: pin}, ResponseLimits: dw.SuppliedResponseLimits{MaxAdapterBytes: 4096, MaxSemanticBytes: 2048}, MaxAdmissions: 2, AdmissionBytes: 4 << 20, OperationWindow: 15 * time.Second, TaskWindow: 20 * time.Second}
}
func reopenIncremental(t *testing.T, s app.DurableStore) app.DurableStore {
	t.Helper()
	r, e := publication.OpenRoot(s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	})
	return app.DurableStore{Root: r, Directory: s.Directory}
}
func TestIncrementalInventoryPipeTwoAdmissions(t *testing.T) {
	ctx := context.Background()
	s := incrementalStore(t)
	j, e := s.Create(ctx, incrementalConfig())
	if e != nil {
		t.Fatal("create", e)
	}
	if j.TaskID != "" || j.JobExpiry != "NONE" || j.Admissions != 0 || j.Used == 0 {
		t.Fatal(j)
	}
	if _, e = s.Task(ctx, j.RunID, ""); e == nil {
		t.Fatal("task before evidence")
	}
	s = reopenIncremental(t, s)
	var inputs [2][]byte
	var units [2]tp.PrivateSourceUnit
	var requests [2]describerequest.Record
	for i := 0; i < 2; i++ {
		r := incrementalPipeSelected(t, fmt.Sprintf("two-%d", i))
		_, u, q, _ := r.Selected()
		units[i] = u
		requests[i] = q
		op, e := s.ReserveAdmission(ctx, j.RunID, j.Head, app.InventoryAdmissionBinding{Index: i, Provenance: "fixed synthetic R pipe", RequestID: q.RecordID, SourceUnitID: u.ID})
		if e != nil {
			t.Fatal("reserve", i, e)
		}
		before := j.Used
		j, e = s.Admit(ctx, op, r)
		if e != nil {
			t.Fatal("admit", i, e)
		}
		if j.Used <= before || j.Admissions != i+1 {
			t.Fatal(j)
		}
		if _, e = s.Admit(ctx, op, r); e == nil {
			t.Fatal("operation replay")
		}
		s = reopenIncremental(t, s)
		readback, e := s.JobStatus(ctx, j.RunID)
		if e != nil || readback != j {
			t.Fatal("reopen", readback, e)
		}
	}
	before := j.Used
	task, e := s.Seal(ctx, j.RunID, j.Head)
	if e != nil {
		t.Fatal("seal", e)
	}
	if task.RunID != j.RunID || task.Used <= before || task.RunDeadlineUnixNano != 0 || task.Phase != "Describe" || task.DeadlineUnixNano <= time.Now().UnixNano() || task.DeadlineUnixNano > time.Now().Add(20*time.Second).UnixNano() || task.Schema != "lsp-trace.durable-inventory-task.v2" {
		t.Fatal(task)
	}
	sealed, e := s.JobStatus(ctx, j.RunID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Seal(ctx, j.RunID, sealed.Head); e == nil {
		t.Fatal("duplicate Seal")
	}
	afterDuplicate, e := s.JobStatus(ctx, j.RunID)
	if e != nil || afterDuplicate != sealed {
		t.Fatal("duplicate Seal changed owner", e)
	}
	for i := 0; i < 2; i++ {
		for at := 0; at < task.Artifacts[0].Bytes; {
			p, e := s.Read(ctx, task.RunID, task.TaskID, "input", at, 32768)
			if e != nil {
				t.Fatal("read", i, e)
			}
			inputs[i] = append(inputs[i], p.Data...)
			at += len(p.Data)
		}
		var retained struct {
			RunID, LeadID string
			Unit          tp.PrivateSourceUnit
			Request       describerequest.Record
		}
		if e = json.Unmarshal(inputs[i], &retained); e != nil {
			t.Fatal(e)
		}
		if retained.RunID != j.RunID || retained.LeadID != fmt.Sprintf("%s-%06d", j.RunID, i) || retained.Unit != units[i] || retained.Request != requests[i] {
			t.Fatal("wrong retained identity")
		}
		if i == 0 {
			task, e = s.Submit(ctx, task.RunID, task.TaskID, task.Role, dw.EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"defines Query","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"empty function","consumer_relative":false},"boundary_contribution":"fixed synthetic fixture","limitations":["controlled mechanics only"]}`))
			if e != nil || task.Phase != "Describe" || task.Sequence != 2 {
				t.Fatal("second lead", task, e)
			}
		}
	}
	if bytes.Equal(inputs[0], inputs[1]) {
		t.Fatal("distinct admissions collapsed")
	}
	t.Log("two genuine retained admissions, same job and ledger, exact source/request identity; first Describe advances to second; no publication claim")
}

// Deliberately replace the exact claimed head with a different valid claimed
// predecessor. The operation must retain bytes, then lose its admission CAS;
// it may not rebase, refund, or admit those orphan bytes on reopen.
func TestIncrementalInventoryPipeInterruptedCommit(t *testing.T) {
	ctx := context.Background()
	s := incrementalStore(t)
	j, e := s.Create(ctx, incrementalConfig())
	if e != nil {
		t.Fatal(e)
	}
	r := incrementalPipeSelected(t, "interrupt")
	_, u, q, _ := r.Selected()
	op, e := s.ReserveAdmission(ctx, j.RunID, j.Head, app.InventoryAdmissionBinding{Index: 0, Provenance: "fixed pipe", RequestID: q.RecordID, SourceUnitID: u.ID})
	if e != nil {
		t.Fatal(e)
	}
	claimed, e := s.JobStatus(ctx, j.RunID)
	if e != nil {
		t.Fatal(e)
	}
	name := "inventory-run-" + j.RunID + ".json"
	old, e := publication.ReadVerifiedBoundFileChecked(s.Root, name, 16384)
	if e != nil {
		t.Fatal(e)
	}
	var envelope struct {
		Digest  string
		Content json.RawMessage
	}
	if e = json.Unmarshal(old, &envelope); e != nil {
		t.Fatal(e)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(envelope.Content, &fields); e != nil {
		t.Fatal(e)
	}
	var issued int64
	if e = json.Unmarshal(fields["Issued"], &issued); e != nil {
		t.Fatal(e)
	}
	envelope.Content = bytes.Replace(envelope.Content, []byte(fmt.Sprintf("\"Issued\":%d", issued)), []byte(fmt.Sprintf("\"Issued\":%d", issued-1)), 1)
	envelope.Digest = cat.NativeDigestV3(envelope.Content)
	changed, e := json.Marshal(envelope)
	if e != nil {
		t.Fatal(e)
	}
	receipt, e := publication.CompareAndReplaceBoundFile(ctx, s.Root, name, publication.BoundFilePredecessor{Selector: name, Digest: cat.NativeDigestV3(old), ByteLength: uint64(len(old))}, changed, func(got []byte) error {
		if !bytes.Equal(got, changed) {
			return fmt.Errorf("changed head")
		}
		return nil
	})
	if e != nil || !receipt.Committed {
		t.Fatal(e)
	}
	if _, e = s.Admit(ctx, op, r); e == nil {
		t.Fatal("stale commit succeeded")
	}
	s = reopenIncremental(t, s)
	after, e := s.JobStatus(ctx, j.RunID)
	if e != nil || after.Status != "PARTIAL_UNCERTAIN" || after.Used != claimed.Used || after.Admissions != 0 {
		t.Fatal(after, e)
	}
	entries, e := os.ReadDir(s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	orphans := 0
	for _, x := range entries {
		if strings.HasPrefix(x.Name(), "inventory-evidence-") {
			orphans++
		}
	}
	if orphans != 1 {
		t.Fatal("expected one unadmitted immutable evidence object", orphans)
	}
	if _, e = s.Seal(ctx, j.RunID, after.Head); e == nil {
		t.Fatal("sealed uncertain")
	}
	if _, e = s.ReserveAdmission(ctx, j.RunID, after.Head, app.InventoryAdmissionBinding{Index: 0, RequestID: q.RecordID, SourceUnitID: u.ID}); e == nil {
		t.Fatal("refunded reservation")
	}
	if _, e = s.Admit(ctx, op, r); e == nil {
		t.Fatal("replayed consumed operation")
	}
	budget, e := sc.NewCallerBudgetV3(time.Now().Add(time.Second), 8<<20)
	if e != nil {
		t.Fatal(e)
	}
	config := incrementalConfig()
	pins := config.Pins
	pins.PromptSHA256 = "sha256:" + q.Envelope.InputSHA256
	if _, _, e = sc.RetainInventoryEvidence(ctx, s.Root, j.RunID, "forbidden-retry", r, sc.CallerDescribeBindingV3{AttemptID: "forbidden-retry", Interpreter: config.Interpreter, Pins: pins, Deadline: time.Now().Add(time.Second), Limits: config.ResponseLimits, Budget: budget}); e == nil {
		t.Fatal("replayed consumed selected authority")
	}
	t.Log("valid claimed predecessor replacement: evidence written, commit refused, orphan not admitted; unchanged spent, terminal partial uncertainty, Seal/replay refused")
}
func TestIncrementalInventoryPipeSealFunding(t *testing.T) {
	ctx := context.Background()
	s := incrementalStore(t)
	c := incrementalConfig()
	c.BudgetBytes = 9 << 20 // Funds length-derived Create, admission and Seal, not completion.
	j, e := s.Create(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	r := incrementalPipeSelected(t, "seal-funding")
	_, u, q, _ := r.Selected()
	op, e := s.ReserveAdmission(ctx, j.RunID, j.Head, app.InventoryAdmissionBinding{Index: 0, Provenance: "controlled funding", RequestID: q.RecordID, SourceUnitID: u.ID})
	if e != nil {
		t.Fatal(e)
	}
	j, e = s.Admit(ctx, op, r)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Seal(ctx, j.RunID, j.Head); e == nil {
		t.Fatal("completion got fresh budget")
	}
	after, e := s.JobStatus(ctx, j.RunID)
	if e != nil || after.Used != j.Used+(2<<20) || after.Status != "PARTIAL_UNCERTAIN" || after.Phase != "Sealing" || after.TaskID != "" {
		t.Fatal(after, e)
	}
	if _, e = s.Task(ctx, j.RunID, ""); e == nil {
		t.Fatal("unfunded task issued")
	}
	entries, e := os.ReadDir(s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	heads := 0
	for _, x := range entries {
		if strings.HasPrefix(x.Name(), "inventory-run-") {
			heads++
		}
	}
	if heads != 1 {
		t.Fatal("alias run", heads)
	}
	t.Log("Seal retains all spending; insufficient completion funding leaves one claimed head and no task")
}
func TestIncrementalInventoryPipeAuthority(t *testing.T) {
	r := incrementalPipeSelected(t, "authority")
	_, u, q, ok := r.Selected()
	if !ok || u.ID == "" || q.Envelope.AdmissionID != u.ID {
		t.Fatal("selected identity")
	}
	t.Log("actual opaque selected R via fixed io.Pipe; no provider process")
}
