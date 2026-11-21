//go:build (darwin || linux) && lsp_trace_controlled_describe_fixture

package applicationinventory

import (
	"context"
	"encoding/json"
	"errors"
	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	is "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/scopejob"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func inventorySelectedConfigTest(t *testing.T, run, denominator string) is.PrivateSelectedConfigurationV2 {
	t.Helper()
	d := "sha256:" + strings.Repeat("1", 64)
	input := cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "controlled-source", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, e := cc.NewContinuationContract(input)
	if e != nil {
		t.Fatal(e)
	}
	return is.PrivateSelectedConfigurationV2{RunID: run, SeedNamespace: "inventory-selected-v1", Lineage: "controlled-owned", Mode: "R", Seed: 7, Attempts: [3]string{cat.NativeDigestV3([]byte(run + "/M")), cat.NativeDigestV3([]byte(run + "/P")), cat.NativeDigestV3([]byte(run + "/S"))}, Contract: contract, ContractInput: input, SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxObjects: 1000, MaxWork: 10000}, ExpectedSourceAuthorization: denominator}
}
func TestInventorySelectedProfile(t *testing.T) {
	p := is.NewInventorySelectedProfileV1()
	if !p.Valid() || p.ParentLimit != 64<<20+p.ControlBytes+p.AdmissionBytes || p.AdmissionBytes != 8<<20 {
		t.Fatal("PROFILE_ENVELOPE")
	}
	for i := range p.Child {
		for field := 0; field < 3; field++ {
			q := p
			switch field {
			case 0:
				q.Child[i]++
			case 1:
				q.Bootstrap[i]++
			case 2:
				q.Allocation[i]++
			}
			if q.Valid() {
				t.Fatal("PROFILE_TAMPER", field, i)
			}
		}
		if p.Allocation[i] != p.Child[i]+p.Bootstrap[i] {
			t.Fatal("PROFILE_CONSERVATION", i)
		}
	}
	if p.Child[5] != 0 || p.Child[6] != 0 || p.Child[8] != uint64(46*time.Second) || p.Child[12] != 0 {
		t.Fatal("PROFILE_NO_PROVIDER_OR_RENEWAL")
	}
	if _, e := cat.NewReservedCallerBudgetV3(time.Now().Add(time.Second), 8<<20, 64<<20, 0); e == nil {
		t.Fatal("LEGACY_LIMIT_ENLARGED")
	}
	b, e := cat.NewInventoryAdmissionBudgetV4(time.Now().Add(time.Second), p.AdmissionBytes, p.ParentLimit-p.AdmissionBytes)
	if e != nil {
		t.Fatal(e)
	}
	n, _ := b.Usage()
	if n != p.ParentLimit-p.AdmissionBytes {
		t.Fatal("SPEND_RESET")
	}
	if _, e := cat.NewInventoryAdmissionBudgetV4(time.Now().Add(-time.Second), p.AdmissionBytes, n); e == nil {
		t.Fatal("EXPIRED_BUDGET")
	}
	if _, e := is.OpenInventoryFundedSelectedV1(context.Background(), publication.BoundFileCommit{}, is.InventoryFundedDiscoveryV1{}, is.PrivateSelectedConfigurationV2{}); e == nil {
		t.Fatal("MISSING_FUNDING")
	}
	t.Logf("CLOSED_SELECTED_PROFILE: PASS H_limit=%d control=%d admit=%d child=%v bootstrap=%v allocation=%v", p.ParentLimit, p.ControlBytes, p.AdmissionBytes, p.Child, p.Bootstrap, p.Allocation)
}

func TestInventorySelectedAdmission(t *testing.T) {
	for _, mode := range []string{"success", "selected-failure", "expired-admit", "missing-owner", "stale-head", "changed-completion"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			dir, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			if e = os.Chmod(dir, 0700); e != nil {
				t.Fatal(e)
			}
			f, e := sessionruntime.NewControlledInventoryPipeFixture(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer func() {
				c, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if e := f.Close(c); e != nil {
					t.Error(e)
				}
			}()
			s := ownedTestStore(t)
			cfg, e := SelectedAdmissionConfigV1(scopejob.Config{Workspace: dir, Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: scopejob.DefaultBudget()}, ownedTestConfig())
			if e != nil {
				t.Fatal("config", e)
			}
			ready, e := s.Create(ctx, cfg)
			if e != nil {
				t.Fatal("create", e)
			}
			ready, e = s.Enumerate(ctx, ready.RunID, ready.Head)
			if e != nil {
				t.Fatal("enumerate", e)
			}
			batch, e := s.NativeDiscoveryBatchV1(ctx, ready.RunID, ready.Head)
			if e != nil || len(batch.Members) != 16 {
				t.Fatal("batch", e)
			}
			c, e := f.Construction(0, time.Now().Add(10*time.Second))
			if e != nil {
				t.Fatal(e)
			}
			c.Members = batch.Members
			if mode == "expired-admit" {
				c.Deadline = time.Now().Add(3 * time.Second)
				c.Start.Deadline = c.Deadline
			}
			starts := 0
			factory := c.NewStarter
			c.NewStarter = func() (sessionruntime.Starter, error) { starts++; return factory() }
			p, e := s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c)
			if e != nil {
				t.Fatal("discovery reserve", e)
			}
			d, e := is.OpenInventoryFundedDiscoveryV1(ctx, p, c)
			if e != nil {
				t.Fatal("discovery open", e)
			}
			selectedConfig := inventorySelectedConfigTest(t, ready.RunID, batch.DenominatorDigest)
			pending, _ := s.JobStatus(ctx, ready.RunID)
			if _, e = s.ContinueSelectedAdmissionV1(ctx, ready.RunID, pending.Head, d, 0, selectedConfig); e == nil {
				t.Fatal("EARLY_ADMISSION")
			}
			if d.OwnedMethods().Available() {
				t.Fatal("EARLY_ORIGINALS")
			}
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
						return errors.New("genuine symbols missing")
					}
					if filepath.Base(m.Path) == "query-01.go" || filepath.Base(m.Path) == "query-17.go" {
						queryURI = m.URI
					}
				}
				refs, e := r.References(ctx, queryURI, lsp.Position{Line: 1, Character: 5})
				if e == nil && (len(refs) != 1 || refs[0].URI == queryURI) {
					return errors.New("genuine cross-file reference missing")
				}
				return e
			})
			if e != nil {
				t.Fatal("discovery execute", e)
			}
			if e = d.Complete(ctx); e != nil {
				t.Fatal("discovery complete", e)
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
				t.Fatal("NO_ORIGINAL_REFERENCE_SELECTOR")
			}
			prior, e := s.JobStatus(ctx, ready.RunID)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "changed-completion" {
				h, old, e := s.head(ready.RunID)
				if e != nil {
					t.Fatal(e)
				}
				h.Issued++
				if _, e = s.change(ctx, old, h); e != nil {
					t.Fatal(e)
				}
				prior, e = s.JobStatus(ctx, ready.RunID)
				if e != nil {
					t.Fatal(e)
				}
			}
			owner := d
			if mode == "missing-owner" {
				owner = is.InventoryFundedDiscoveryV1{}
			}
			head := prior.Head
			if mode == "stale-head" {
				head = ready.Head
			}
			fault := func(stage string) error {
				if mode == "selected-failure" && stage == "before-source" {
					return errors.New("controlled selected failure after M/P")
				}
				if mode == "expired-admit" && stage == "before-admit" {
					<-time.After(time.Until(c.Deadline) + time.Millisecond)
				}
				return nil
			}
			done, err := s.continueSelectedAdmissionV1(ctx, ready.RunID, head, owner, index, selectedConfig, fault)
			actual, e := s.JobStatus(ctx, ready.RunID)
			if e != nil {
				t.Fatal("final status", e)
			}
			expectedUsed := prior.Used + cat.InventorySelectedControlBytesV1 + cat.InventoryAdmissionBytesV4
			if mode == "stale-head" {
				expectedUsed = prior.Used
			}
			if actual.Used != expectedUsed || starts != 1 {
				t.Fatalf("SPEND_OR_PROVIDER: FAIL mode=%s used=%d expected=%d starts=%d", mode, actual.Used, expectedUsed, starts)
			}
			if mode != "success" {
				if err == nil || actual.Admissions != 0 || actual.Phase == "Admitted" {
					t.Fatalf("FAILURE_ADMITTED: FAIL %+v %v", actual, err)
				}
				if mode != "stale-head" && actual.Status != "PARTIAL_UNCERTAIN" {
					t.Fatal("FAILURE_NOT_UNCERTAIN", actual)
				}
				if mode == "stale-head" || mode == "missing-owner" || mode == "changed-completion" {
					child, e := s.Root.OpenPrivateChild("inventory-selected-"+ready.RunID+"-000000", false)
					if e == nil {
						_ = child.Close()
						t.Fatal("UNFUNDED_SELECTED_EFFECT")
					}
				}
				t.Log("FAILURE_SPENT_NO_ADMISSION: PASS", mode, err)
				return
			}
			if err != nil || done.Admissions != 1 || done.Phase != "Admitted" || done.Status != "PARTIAL" || done.Selected == nil || done.Selected.Accounting.Windows != 3 {
				t.Fatalf("CONNECTED_ADMISSION: FAIL err=%v actual_status=%s actual_admissions=%d", err, actual.Status, actual.Admissions)
			}
			h, _, e := s.head(done.RunID)
			if e != nil {
				t.Fatal(e)
			}
			nodeRaw, e := publication.ReadObject(s.Root, "inventory-admission-", h.Input)
			var node ownedAdmissionRecord
			if e != nil || strictInventory(nodeRaw, &node) != nil {
				t.Fatal("admission record", e)
			}
			evidence, e := publication.ReadObject(s.Root, "inventory-evidence-", node.Evidence)
			var held struct {
				Accounting is.CompletedSelectedAccountingV1
				Unit       tp.PrivateSourceUnit
				Request    describerequest.Record
			}
			if e != nil || json.Unmarshal(evidence, &held) != nil || held.Accounting != done.Selected.Accounting || held.Accounting.URI != queryURI || held.Unit.ID != node.Binding.SourceUnitID || held.Request.RecordID != node.Binding.RequestID || held.Unit.SourceDigest == "" || held.Unit.SessionID == "" || held.Unit.Body == "" {
				t.Fatal("ORIGINAL_IDENTITIES: FAIL", e)
			}
			originalPath := ""
			for _, m := range c.Members {
				if m.URI == queryURI {
					originalPath = m.Path
				}
			}
			originalBytes, e := os.ReadFile(originalPath)
			if e != nil || cat.NativeDigestV3(originalBytes) != held.Unit.SourceDigest || held.Unit.SessionID != owned.GroupingSession().SessionID || describerequest.ValidateRecord(held.Request) != nil || held.Request.Envelope.AdmissionID != held.Unit.ID {
				t.Fatal("ORIGINAL_SOURCE_REQUEST_BINDING", e)
			}
			if _, e = s.ContinueSelectedAdmissionV1(ctx, done.RunID, done.Head, d, 0, selectedConfig); e == nil {
				t.Fatal("REPLAY")
			}
			if _, e = s.ReserveAdmission(ctx, done.RunID, done.Head, node.Binding); e == nil {
				t.Fatal("DUPLICATE_ADMISSION")
			}
			if _, e = s.Seal(ctx, done.RunID, done.Head); e == nil {
				t.Fatal("SEAL_ENABLED")
			}
			if _, e = s.NativeDiscoveryBatchV1(ctx, done.RunID, done.Head); e == nil {
				t.Fatal("BATCH2_ENABLED")
			}
			cleanup, cancelCleanup := context.WithTimeout(ctx, time.Second)
			if e := f.Close(cleanup); e != nil {
				t.Fatal("FIXTURE_CLEANUP", e)
			}
			cancelCleanup()
			if files := retentionPreflightFiles(t, dir); len(files) != 0 {
				t.Fatal("SOURCE_CLEANUP_REMAINDER", files)
			}
			files := retentionPreflightFiles(t, s.Directory)
			admissions, evidences := 0, 0
			for _, f := range files {
				if strings.HasPrefix(f.Path, "inventory-admission-") {
					admissions++
				}
				if strings.HasPrefix(f.Path, "inventory-evidence-") {
					evidences++
				}
			}
			if admissions != 1 || evidences != 1 {
				t.Fatal("EXACTLY_ONE_RETENTION", admissions, evidences)
			}
			reopened, e := publication.OpenRoot(s.Directory)
			if e != nil {
				t.Fatal(e)
			}
			fresh := DurableStore{Root: reopened, Directory: s.Directory}
			status, e := fresh.JobStatus(ctx, done.RunID)
			ce := reopened.Close()
			if e != nil || ce != nil || !reflect.DeepEqual(actual, status) || !reflect.DeepEqual(files, retentionPreflightFiles(t, s.Directory)) {
				t.Fatal("REOPEN_CHANGED", e, ce)
			}
			var total int64
			for _, f := range files {
				total += f.Bytes
			}
			manifest, _ := json.Marshal(files)
			t.Logf("FINAL_REGULAR_FILES bytes=%d manifest=%s; allocated/peak/IO unknown", total, manifest)
			t.Logf("CONNECTED_ADMISSION: PASS; ORIGINAL_IDENTITIES: PASS request=%s source=%s session=%s originalDeadline=%d H=%d windows=%d native=%v", node.Binding.RequestID, held.Unit.SourceDigest, held.Unit.SessionID, held.Accounting.NativeDeadline, done.Used, held.Accounting.Windows, held.Accounting.Charged)
		})
	}
}
