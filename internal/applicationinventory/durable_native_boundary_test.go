//go:build (darwin || linux) && lsp_trace_controlled_describe_fixture

package applicationinventory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	is "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
	"lsp-trace/sessionruntime"
)

func nativeBoundaryFixture(t *testing.T) (DurableStore, InventoryJobStatus, *sessionruntime.ControlledInventoryPipeFixture, sessionruntime.PrivateDiscoveryConstruction) {
	t.Helper()
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
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if e := f.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	s := ownedTestStore(t)
	cfg, e := NativeBoundaryConfigV1(scopejob.Config{Workspace: dir, Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: scopejob.DefaultBudget()})
	if e != nil {
		t.Fatal(e)
	}
	created, e := s.Create(context.Background(), cfg)
	if e != nil {
		t.Fatal("native profile Create", e)
	}
	ready, e := s.Enumerate(context.Background(), created.RunID, created.Head)
	if e != nil {
		t.Fatal("native profile Enumerate", e)
	}
	c, e := f.Construction(0, time.Now().Add(10*time.Second))
	if e != nil {
		t.Fatal(e)
	}
	batch, e := s.NativeDiscoveryBatchV1(context.Background(), ready.RunID, ready.Head)
	if e != nil || batch.Counts.Selected != 17 || batch.UnknownRemainder || len(batch.Members) != 16 {
		t.Fatalf("ASSERT_VERIFIED_BATCH: FAIL e=%v selected=%d unknown=%v members=%+v fixture=%+v", e, batch.Counts.Selected, batch.UnknownRemainder, batch.Members, c.Members)
	}
	c.Members = batch.Members // exact frozen enumeration order, not fixture filename order
	return s, ready, f, c
}
func nativeBoundaryAbsent(t *testing.T, s DurableStore, run string) {
	t.Helper()
	root, e := s.Root.OpenPrivateChild("inventory-native-"+run+"-000000", false)
	if e == nil {
		_ = root.Close()
		t.Fatal("ASSERT_NO_NATIVE_EFFECT: FAIL")
	}
}
func TestInventoryNativeBoundaryPositive(t *testing.T) {
	s, ready, _, c := nativeBoundaryFixture(t)
	ctx := context.Background()
	starts := 0
	factory := c.NewStarter
	c.NewStarter = func() (sessionruntime.Starter, error) { starts++; return factory() }
	permit, e := s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c)
	if e != nil {
		t.Fatal("ASSERT_PARENT_COMMIT: FAIL", e)
	}
	claimed, e := s.JobStatus(ctx, ready.RunID)
	if e != nil || claimed.Status != "PARTIAL_UNCERTAIN" || claimed.Native == nil || claimed.Used != ready.Used+is.InventoryNativeControlBytesV1 || claimed.Native.Allocated != is.NewInventoryNativeProfileV1().Allocation || starts != 0 {
		t.Fatalf("ASSERT_PARENT_BEFORE_EFFECTS: FAIL e=%v status=%s starts=%d", e, claimed.Status, starts)
	}
	nativeBoundaryAbsent(t, s, ready.RunID)
	op, e := is.OpenInventoryFundedDiscoveryV1(ctx, permit, c)
	if e != nil {
		t.Fatal("ASSERT_NATIVE_RESERVED: FAIL", e)
	}
	if starts != 0 {
		t.Fatal("ASSERT_NATIVE_RESERVE_BEFORE_STARTER: FAIL")
	}
	if _, e = is.OpenInventoryFundedDiscoveryV1(ctx, permit, c); e == nil {
		t.Fatal("ASSERT_PERMIT_ONE_SHOT: FAIL")
	}
	if e = op.Execute(ctx, c, func(ctx context.Context, d *sessionruntime.PrivateDiscoverySession) error {
		for _, m := range c.Members {
			if e := d.Supply(ctx, m.URI, m.LanguageID); e != nil {
				return e
			}
			symbols, e := d.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: m.URI}})
			if e != nil {
				return e
			}
			if len(symbols) != 1 {
				t.Fatal("ASSERT_GENUINE_SYMBOLS: FAIL")
			}
		}
		selected := ""
		for _, m := range c.Members {
			if filepath.Base(m.Path) == "query-01.go" || filepath.Base(m.Path) == "query-17.go" {
				selected = m.URI
				break
			}
		}
		refs, e := d.References(ctx, selected, lsp.Position{Line: 1, Character: 5})
		if e != nil {
			return e
		}
		if len(refs) != 1 || refs[0].URI == selected {
			t.Fatal("ASSERT_GENUINE_R: FAIL")
		}
		return nil
	}); e != nil {
		t.Fatal("ASSERT_FUNDED_STARTER_EXECUTION: FAIL", e)
	}
	if starts != 1 {
		t.Fatal("ASSERT_FUNDED_STARTER_EXECUTION: FAIL", starts)
	}
	if e = op.Complete(ctx); e != nil {
		t.Fatal("ASSERT_PARENT_NATIVE_COMPLETION: FAIL", e)
	}
	if !op.OwnedMethods().Available() {
		t.Fatal("ASSERT_FUNDED_ORIGINALS: FAIL")
	}
	done, e := s.JobStatus(ctx, ready.RunID)
	if e != nil || done.Status != "PARTIAL" || done.Native == nil || done.Native.Outcome != "OBSERVED" || done.Native.NativeHead == "" || done.Used != claimed.Used || done.Packets != claimed.Packets || done.Native.Allocated != claimed.Native.Allocated {
		t.Fatalf("ASSERT_NO_DOUBLE_PARENT_CHARGE: FAIL e=%v status=%s", e, done.Status)
	}
	for i, n := range done.Native.NativeCharged {
		if n > done.Native.Profile.Child[i] || n > done.Native.Allocated[i] {
			t.Fatal("ASSERT_COMPONENTWISE_CONSERVATION: FAIL", i)
		}
	}
	if _, e = s.ReserveAdmission(ctx, done.RunID, done.Head, InventoryAdmissionBinding{}); e == nil {
		t.Fatal("ASSERT_ADMISSION_BLOCKED: FAIL")
	}
	if _, e = s.Seal(ctx, done.RunID, done.Head); e == nil {
		t.Fatal("ASSERT_SEAL_BLOCKED: FAIL")
	}
	if e = op.Complete(ctx); e == nil {
		t.Fatal("ASSERT_COMPLETION_ONE_SHOT: FAIL")
	}
	t.Logf("PROFILE allocation=%v child=%v bootstrap=%v H_control=%d native_charged=%v H_before=%d H_after=%d", done.Native.Allocated, done.Native.Profile.Child, done.Native.Profile.Bootstrap, is.InventoryNativeControlBytesV1, done.Native.NativeCharged, ready.Used, done.Used)
	t.Log("ASSERT_PARENT_BEFORE_EFFECTS: PASS; ASSERT_NATIVE_RESERVED: PASS; ASSERT_FUNDED_STARTER_EXECUTION: PASS; ASSERT_FUNDED_ORIGINALS: PASS; ASSERT_NO_DOUBLE_PARENT_CHARGE: PASS; ASSERT_COMPONENTWISE_CONSERVATION: PASS; one funded batch, not 16+1/publication")
}

func TestInventoryNativeBoundaryRefusals(t *testing.T) {
	for _, fault := range []string{"zero", "json", "no-commit", "membership", "permit-membership", "stale", "expired", "lost", "head-changed", "child-failure", "lost-completion", "budget", "profile"} {
		t.Run(fault, func(t *testing.T) {
			s, ready, _, c := nativeBoundaryFixture(t)
			ctx := context.Background()
			starts := 0
			factory := c.NewStarter
			c.NewStarter = func() (sessionruntime.Starter, error) { starts++; return factory() }
			if fault == "zero" || fault == "json" || fault == "no-commit" {
				p := is.InventoryNativePermitV1{}
				if fault == "json" {
					if e := json.Unmarshal([]byte(`{"Allocated":[999999999],"RunID":"fake"}`), &p); e != nil {
						t.Fatal(e)
					}
				}
				if fault == "no-commit" {
					if _, e := is.InventoryNativePermitFromCommitV1(ctx, publication.BoundFileCommit{}); e == nil {
						t.Fatal("ASSERT_NO_PARENT_REFUSED: FAIL")
					}
				}
				if _, e := is.OpenInventoryFundedDiscoveryV1(ctx, p, c); e == nil {
					t.Fatal("ASSERT_NO_PARENT_REFUSED: FAIL")
				}
				nativeBoundaryAbsent(t, s, ready.RunID)
				t.Log("ASSERT_NO_PARENT_REFUSED: PASS")
				return
			}
			if fault == "profile" {
				cfg, _ := NativeBoundaryConfigV1(scopejob.Config{Workspace: c.Start.Profile.Workspace().String(), Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: scopejob.DefaultBudget()})
				for i := range cfg.NativeProfile.Allocation {
					profile := is.NewInventoryNativeProfileV1()
					profile.Allocation[i]++
					cfg.NativeProfile = &profile
					if _, e := s.Create(ctx, cfg); e == nil {
						t.Fatal("ASSERT_PROFILE_TAMPER_REFUSED: FAIL", i)
					}
				}
				t.Log("ASSERT_PROFILE_TAMPER_REFUSED: PASS")
				return
			}
			if fault == "budget" {
				cfg, _ := NativeBoundaryConfigV1(scopejob.Config{Workspace: c.Start.Profile.Workspace().String(), Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: scopejob.DefaultBudget()})
				cfg.BudgetBytes = 40 << 20
				cfg.Enumeration.Budget.LogicalBytes = 40 << 20
				if _, e := s.Create(ctx, cfg); e == nil {
					t.Fatal("ASSERT_BUDGET_REFUSED: FAIL")
				}
				nativeBoundaryAbsent(t, s, ready.RunID)
				t.Log("ASSERT_BUDGET_REFUSED: PASS")
				return
			}
			if fault == "membership" {
				c.Members[0].URI += "-tampered"
			}
			if fault == "stale" {
				ready.Head = "sha256:" + ready.RunID
			}
			if fault == "expired" {
				c.Deadline = time.Now().Add(500 * time.Millisecond)
				c.Start.Deadline = c.Deadline
			}
			permit, e := s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c)
			if fault == "membership" || fault == "stale" {
				if e == nil {
					t.Fatal("ASSERT_PRECLAIM_BINDING_REFUSED: FAIL")
				}
				nativeBoundaryAbsent(t, s, ready.RunID)
				now, e := s.JobStatus(ctx, ready.RunID)
				if e != nil || now.Used != ready.Used {
					t.Fatal("ASSERT_REFUSAL_NO_CLAIM: FAIL", e)
				}
				t.Log("ASSERT_PRECLAIM_BINDING_REFUSED: PASS")
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			charged, e := s.JobStatus(ctx, ready.RunID)
			if e != nil {
				t.Fatal(e)
			}
			switch fault {
			case "permit-membership":
				c.Members[0].URI += "-tampered"
				if _, e = is.OpenInventoryFundedDiscoveryV1(ctx, permit, c); e == nil {
					t.Fatal("ASSERT_PERMIT_MEMBERSHIP_REFUSED: FAIL")
				}
				nativeBoundaryAbsent(t, s, ready.RunID)
			case "lost":
				if _, e = s.ReserveNativeDiscoveryV1(ctx, ready.RunID, charged.Head, c); e == nil {
					t.Fatal("ASSERT_LOST_CLAIM_NO_REMINT: FAIL")
				}
				nativeBoundaryAbsent(t, s, ready.RunID)
			case "expired":
				time.Sleep(time.Until(c.Deadline) + 5*time.Millisecond)
				if _, e = is.OpenInventoryFundedDiscoveryV1(ctx, permit, c); e == nil {
					t.Fatal("ASSERT_EXPIRED_PERMIT_REFUSED: FAIL")
				}
				nativeBoundaryAbsent(t, s, ready.RunID)
			case "head-changed":
				h, old, e := s.head(ready.RunID)
				if e != nil {
					t.Fatal(e)
				}
				h.Native.Policy += "-changed"
				if _, e = s.change(ctx, old, h); e != nil {
					t.Fatal(e)
				}
				if _, e = is.OpenInventoryFundedDiscoveryV1(ctx, permit, c); e == nil {
					t.Fatal("ASSERT_CURRENT_HEAD_REQUIRED: FAIL")
				}
				nativeBoundaryAbsent(t, s, ready.RunID)
			case "lost-completion":
				op, e := is.OpenInventoryFundedDiscoveryV1(ctx, permit, c)
				if e != nil {
					t.Fatal(e)
				}
				if e = op.Execute(ctx, c, func(ctx context.Context, d *sessionruntime.PrivateDiscoverySession) error {
					for _, m := range c.Members {
						if e := d.Supply(ctx, m.URI, m.LanguageID); e != nil {
							return e
						}
						if _, e := d.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: m.URI}}); e != nil {
							return e
						}
					}
					return nil
				}); e != nil {
					t.Fatal(e)
				}
				h, old, e := s.head(ready.RunID)
				if e != nil {
					t.Fatal(e)
				}
				h.TaskID = ready.RunID
				if _, e = s.change(ctx, old, h); e != nil {
					t.Fatal(e)
				}
				if e = op.Complete(ctx); e == nil || op.OwnedMethods().Available() {
					t.Fatal("ASSERT_LOST_COMPLETION_NO_TRANSFER: FAIL")
				}
			case "child-failure":
				op, e := is.OpenInventoryFundedDiscoveryV1(ctx, permit, c)
				if e != nil {
					t.Fatal(e)
				}
				// Actual started/supplied child; consumer failure is not a refusing Starter.
				e = op.Execute(ctx, c, func(ctx context.Context, d *sessionruntime.PrivateDiscoverySession) error {
					if e := d.Supply(ctx, c.Members[0].URI, "go"); e != nil {
						return e
					}
					return context.Canceled
				})
				if e == nil || starts != 1 {
					t.Fatal("ASSERT_ACTUAL_CHILD_FAILURE: FAIL", e, starts)
				}
				if e = op.Complete(ctx); e == nil {
					t.Fatal("ASSERT_CHILD_FAILURE_REPORTED: FAIL")
				}
			}
			now, e := s.JobStatus(ctx, ready.RunID)
			if e != nil || now.Used != charged.Used || now.Native == nil || now.Native.Allocated != charged.Native.Allocated {
				t.Fatal("ASSERT_NO_REFUND: FAIL", e)
			}
			t.Log("ASSERT_NO_REFUND: PASS", fault)
		})
	}
}
func TestInventoryNativeBoundaryForgedCommittedClaim(t *testing.T) {
	s, ready, _, c := nativeBoundaryFixture(t)
	ctx := context.Background()
	h, _, e := s.head(ready.RunID)
	if e != nil {
		t.Fatal(e)
	}
	config, e := s.ownedConfig(h)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c); e != nil {
		t.Fatal(e)
	}
	charged, _, e := s.head(ready.RunID)
	if e != nil {
		t.Fatal(e)
	}
	// Another legitimate parent, then an intentionally forged replacement made
	// through the generic publication CAS API rather than the inventory owner.
	created, e := s.Create(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.Enumerate(ctx, created.RunID, created.Head)
	if e != nil {
		t.Fatal(e)
	}
	target, old, e := s.head(second.RunID)
	if e != nil {
		t.Fatal(e)
	}
	claim := *charged.Native
	claim.RunID = second.RunID
	claim.ParentHead = second.Head
	claim.ChildNamespace = "inventory-native-" + second.RunID + "-000000"
	claim.Binding.PlanIdentity = publication.DigestForTest([]byte("inventory-native-plan.v1/" + second.RunID + "/" + target.Enumeration.Last))
	claim.Binding.DenominatorDigest = "sha256:" + target.Enumeration.Last
	claim.Members = append([]sessionruntime.PrivateDiscoveryMember(nil), claim.Members...)
	claim.Members[0].URI += "-forged"
	encoded, _ := json.Marshal(claim.Members)
	claim.Binding.MemberDigest = publication.DigestForTest(encoded)
	target.Native = &claim
	target.Phase = "NativeAcquisition"
	target.Status = "CLAIMED"
	target.TaskID = second.RunID
	target.Deadline = c.Deadline.UnixNano()
	target.Used += is.InventoryNativeControlBytesV1
	target.Work = target.Used * 32
	target.Packets++
	raw, e := encodeInventoryHead(target)
	if e != nil {
		t.Fatal(e)
	}
	proof, e := publication.CommitBoundFile(ctx, s.Root, inventoryHeadName(second.RunID), old, raw)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = is.InventoryNativePermitFromCommitV1(ctx, proof); e == nil {
		t.Fatal("ASSERT_COMMITTED_DTO_NOT_AUTHORITY: FAIL")
	}
	nativeBoundaryAbsent(t, s, second.RunID)
	t.Log("ASSERT_COMMITTED_DTO_NOT_AUTHORITY: PASS; independent frozen enumeration replay rejected forged member")
}

func TestInventoryNativeBoundaryConcurrentConsume(t *testing.T) {
	s, ready, _, c := nativeBoundaryFixture(t)
	ctx := context.Background()
	p, e := s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	wins := make(chan is.InventoryFundedDiscoveryV1, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			op, e := is.OpenInventoryFundedDiscoveryV1(ctx, p, c)
			if e == nil {
				wins <- op
			}
		}()
	}
	wg.Wait()
	close(wins)
	count := 0
	for op := range wins {
		count++
		_ = op.Complete(ctx)
	} // unused native reservation remains spent/FAILED
	if count != 1 {
		t.Fatal("ASSERT_CONCURRENT_PERMIT_ONE_SHOT: FAIL", count)
	}
	t.Log("ASSERT_CONCURRENT_PERMIT_ONE_SHOT: PASS")
}

func TestInventoryNativeBoundaryLegacyCaps(t *testing.T) {
	deadline := time.Now().Add(time.Second)
	if _, e := cat.NewCallerBudgetV3(deadline, 64<<20); e != nil {
		t.Fatal(e)
	}
	if _, e := cat.NewCallerBudgetV3(deadline, (64<<20)+1); e == nil {
		t.Fatal("ASSERT_LEGACY_64M_CAP: FAIL")
	}
	if _, e := cat.NewReservedCallerBudgetV3(deadline, (64<<20)+1, 0, 0); e == nil {
		t.Fatal("ASSERT_LEGACY_RESERVED_64M_CAP: FAIL")
	}
	profile := is.NewInventoryNativeProfileV1()
	if profile.Allocation[0] <= 64<<20 {
		t.Fatal("ASSERT_EXPLICIT_SEPARATE_PROFILE: FAIL")
	}
	t.Log("ASSERT_LEGACY_64M_CAP: PASS; ASSERT_LEGACY_RESERVED_64M_CAP: PASS; no legacy grants enlarged")
}

func TestInventoryNativeBoundaryConcurrentClaim(t *testing.T) {
	s, ready, _, c := nativeBoundaryFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	wins := make(chan is.InventoryNativePermitV1, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, e := s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c)
			if e == nil {
				wins <- p
			}
		}()
	}
	wg.Wait()
	close(wins)
	count := 0
	for range wins {
		count++
	}
	now, e := s.JobStatus(ctx, ready.RunID)
	if count != 1 || e != nil || now.Used != ready.Used+is.InventoryNativeControlBytesV1 {
		t.Fatal("ASSERT_SINGLE_CAS_ALLOCATION: FAIL", count, e)
	}
	nativeBoundaryAbsent(t, s, ready.RunID)
	t.Log("ASSERT_SINGLE_CAS_ALLOCATION: PASS")
}

// Retains the independent review's five lifecycle/consumption checks.
func TestInventoryNativeBoundaryLifecycle(t *testing.T) {
	for _, mode := range []string{"complete-before-execute", "nil-open-consumes", "canceled-open-consumes", "nil-execute", "changed-deadline"} {
		t.Run(mode, func(t *testing.T) {
			s, ready, _, c := nativeBoundaryFixture(t)
			ctx := context.Background()
			p, e := s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c)
			if e != nil {
				t.Fatal(e)
			}
			charged, e := s.JobStatus(ctx, ready.RunID)
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "nil-open-consumes", "canceled-open-consumes", "changed-deadline":
				var bad context.Context
				altered := c
				if mode == "canceled-open-consumes" {
					var cancel context.CancelFunc
					bad, cancel = context.WithCancel(ctx)
					cancel()
				}
				if mode == "changed-deadline" {
					bad = ctx
					altered.Deadline = c.Deadline.Add(-time.Millisecond)
				}
				if _, e = is.OpenInventoryFundedDiscoveryV1(bad, p, altered); e == nil {
					t.Fatal("invalid open accepted")
				}
				if _, e = is.OpenInventoryFundedDiscoveryV1(ctx, p, c); e == nil {
					t.Fatal("failed permit reused")
				}
				nativeBoundaryAbsent(t, s, ready.RunID)
			default:
				op, e := is.OpenInventoryFundedDiscoveryV1(ctx, p, c)
				if e != nil {
					t.Fatal(e)
				}
				if op.OwnedMethods().Available() {
					t.Fatal("early originals")
				}
				if mode == "nil-execute" {
					if e = op.Execute(nil, c, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }); e == nil {
						t.Fatal("nil execute accepted")
					}
				}
				if e = op.Complete(ctx); e == nil {
					t.Fatal("unexecuted completion succeeded")
				}
				if e = op.Complete(ctx); e == nil {
					t.Fatal("repeated completion succeeded")
				}
				done, e := s.JobStatus(ctx, ready.RunID)
				if e != nil || done.Native == nil || done.Native.Outcome != "FAILED" || op.OwnedMethods().Available() {
					t.Fatal("incorrect failed completion", e)
				}
			}
			done, e := s.JobStatus(ctx, ready.RunID)
			if e != nil || done.Used != charged.Used || done.Native.Allocated != charged.Native.Allocated {
				t.Fatal("spending changed", e)
			}
		})
	}
}

func TestInventoryNativeBoundaryCallbackGetter(t *testing.T) {
	s, ready, _, c := nativeBoundaryFixture(t)
	ctx := context.Background()
	p, e := s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c)
	if e != nil {
		t.Fatal(e)
	}
	op, e := is.OpenInventoryFundedDiscoveryV1(ctx, p, c)
	if e != nil {
		t.Fatal(e)
	}
	result := make(chan bool, 1)
	blocked, joined, early, direct := false, false, false, false
	executeErr := op.Execute(ctx, c, func(ctx context.Context, d *sessionruntime.PrivateDiscoverySession) error {
		// Preflight the reviewer's lock cycle without stranding a goroutine in RED.
		// On GREEN, exercise the real synchronous callback getter immediately below.
		go func() { result <- op.OwnedMethods().Available() }()
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		select {
		case early = <-result:
			joined = true
		case <-timer.C:
			blocked = true
			return context.Canceled
		}
		if early {
			return context.Canceled
		}
		early = op.OwnedMethods().Available() // direct callback call, not a goroutine
		direct = true
		if early {
			return context.Canceled
		}
		for _, m := range c.Members {
			if e := d.Supply(ctx, m.URI, m.LanguageID); e != nil {
				return e
			}
			if _, e := d.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: m.URI}}); e != nil {
				return e
			}
		}
		return nil
	})
	// On the old implementation callback return releases the mutex, permitting
	// this bounded join. Complete then closes the native root before reporting RED.
	if !joined {
		select {
		case <-result:
		case <-time.After(time.Second):
			t.Fatal("callback getter did not join after Execute")
		}
	}
	completeErr := op.Complete(ctx)
	if blocked {
		t.Fatal("CALLBACK_GETTER_BLOCKED: OwnedMethods cannot return until its own enclosing Execute callback returns")
	}
	if early || !direct || executeErr != nil || completeErr != nil || !op.OwnedMethods().Available() {
		t.Fatalf("CALLBACK_GETTER: FAIL early=%v direct=%v execute=%v complete=%v", early, direct, executeErr, completeErr)
	}
	t.Log("CALLBACK_GETTER: PASS direct callback returned unavailable; originals released only after valid completion; probe joined")
}

func TestInventoryNativeBoundaryConcurrentLifecycle(t *testing.T) {
	for _, mode := range []string{"success", "canceled", "fixture-close"} {
		t.Run(mode, func(t *testing.T) {
			s, ready, fixture, c := nativeBoundaryFixture(t)
			ctx := context.Background()
			p, e := s.ReserveNativeDiscoveryV1(ctx, ready.RunID, ready.Head, c)
			if e != nil {
				t.Fatal(e)
			}
			charged, e := s.JobStatus(ctx, ready.RunID)
			if e != nil {
				t.Fatal(e)
			}
			op, e := is.OpenInventoryFundedDiscoveryV1(ctx, p, c)
			if e != nil {
				t.Fatal(e)
			}
			runCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			entered, release, abandon := make(chan struct{}), make(chan struct{}), make(chan struct{})
			execution := make(chan error, 1)
			executionJoined := make(chan struct{})
			callbackFailures := make(chan string, 3)
			go func() {
				defer close(executionJoined)
				execution <- op.Execute(runCtx, c, func(ctx context.Context, d *sessionruntime.PrivateDiscoverySession) error {
					close(entered)
					<-release // test-owned barrier, always released by the bounded watchdog
					select {
					case <-abandon:
						return context.Canceled
					default:
					}
					// These are synchronous reentrant calls, not just concurrent callers.
					if op.Complete(ctx) == nil {
						callbackFailures <- "reentrant completion accepted"
						return context.Canceled
					}
					if op.Execute(ctx, c, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }) == nil {
						callbackFailures <- "reentrant execution accepted"
						return context.Canceled
					}
					if op.OwnedMethods().Available() {
						callbackFailures <- "early callback originals"
						return context.Canceled
					}
					for _, m := range c.Members {
						if e := d.Supply(ctx, m.URI, m.LanguageID); e != nil {
							return e
						}
						if _, e := d.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: m.URI}}); e != nil {
							return e
						}
					}
					return nil
				})
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				close(abandon)
				close(release)
				cancel()
				select {
				case <-executionJoined:
				case <-time.After(2 * time.Second):
					t.Fatal("execution startup did not join")
				}
				_ = op.Complete(ctx)
				t.Fatal("execution never reached callback")
			}
			const contenders = 12
			results := make(chan string, contenders)
			var workers sync.WaitGroup
			for i := 0; i < contenders; i++ {
				workers.Add(1)
				go func(kind int) {
					defer workers.Done()
					copy := op
					failure := ""
					switch kind % 3 {
					case 0:
						if copy.Complete(ctx) == nil {
							failure = "active completion accepted"
						}
					case 1:
						if copy.Execute(ctx, c, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }) == nil {
							failure = "duplicate execution accepted"
						}
					case 2:
						if copy.OwnedMethods().Available() {
							failure = "active originals escaped"
						}
					}
					results <- failure
				}(i)
			}
			timer := time.NewTimer(500 * time.Millisecond)
			received := 0
			blocked := false
			failures := []string{}
		collect:
			for received < contenders {
				select {
				case failure := <-results:
					received++
					if failure != "" {
						failures = append(failures, failure)
					}
				case <-timer.C:
					blocked = true
					break collect
				}
			}
			timer.Stop()
			if blocked {
				close(abandon)
			}
			if mode == "canceled" {
				cancel()
			}
			if mode == "fixture-close" {
				closeCtx, closeCancel := context.WithTimeout(ctx, time.Second)
				e = fixture.Close(closeCtx)
				closeCancel()
				if e != nil {
					failures = append(failures, "fixture close: "+e.Error())
				}
			}
			close(release)
			select {
			case <-executionJoined:
			case <-time.After(3 * time.Second):
				t.Fatal("execution did not join after release")
			}
			executeErr := <-execution
			// Even a regressed lock may unblock only once the callback exits. Join all
			// contenders before reporting the watchdog failure, as in the reviewer probe.
			for received < contenders {
				select {
				case failure := <-results:
					received++
					if failure != "" {
						failures = append(failures, failure)
					}
				case <-time.After(time.Second):
					t.Fatal("active lifecycle contender did not join")
				}
			}
			workers.Wait()
			if blocked {
				_ = op.Complete(ctx)
				t.Fatal("ACTIVE_LIFECYCLE_BLOCKED: contenders waited for callback return")
			}
			if op.OwnedMethods().Available() {
				failures = append(failures, "originals before completion")
			}
			if len(callbackFailures) > 0 {
				failures = append(failures, <-callbackFailures)
			}
			// Compete for finalization only after Execute and its cleanup have returned.
			// Concurrent getters must never see originals before parent publication.
			completions := make(chan error, 4)
			getterFailures := make(chan string, 4)
			start := make(chan struct{})
			var finalizers sync.WaitGroup
			for i := 0; i < 4; i++ {
				finalizers.Add(2)
				go func() { defer finalizers.Done(); <-start; completions <- op.Complete(ctx) }()
				go func() {
					defer finalizers.Done()
					<-start
					failure := ""
					if op.OwnedMethods().Available() {
						status, e := s.JobStatus(ctx, ready.RunID)
						if e != nil || status.Native == nil || status.Native.Outcome != "OBSERVED" {
							failure = "originals before committed native outcome"
						}
					}
					getterFailures <- failure
				}()
			}
			close(start)
			winners := 0
			for i := 0; i < 4; i++ {
				select {
				case e := <-completions:
					if e == nil {
						winners++
					}
				case <-time.After(2 * time.Second):
					t.Fatal("completion did not return")
				}
			}
			for i := 0; i < 4; i++ {
				select {
				case failure := <-getterFailures:
					if failure != "" {
						failures = append(failures, failure)
					}
				case <-time.After(time.Second):
					t.Fatal("completion getter did not return")
				}
			}
			finalizers.Wait()
			done, e := s.JobStatus(ctx, ready.RunID)
			if e != nil || done.Native == nil || done.Used != charged.Used || done.Native.Allocated != charged.Native.Allocated {
				t.Fatal("LIFECYCLE_NO_REFUND: FAIL", e)
			}
			if mode == "success" {
				if executeErr != nil || winners != 1 || done.Native.Outcome != "OBSERVED" || !op.OwnedMethods().Available() {
					t.Fatalf("ACTIVE_COMPLETION_CONSUMED: execution=%v completion_winners=%d outcome=%s", executeErr, winners, done.Native.Outcome)
				}
			} else if executeErr == nil || winners != 0 || done.Native.NativeHead == "" || done.Native.Outcome == "OBSERVED" || op.OwnedMethods().Available() {
				t.Fatalf("FAILED_LIFECYCLE_ESCAPED: execution=%v winners=%d outcome=%s", executeErr, winners, done.Native.Outcome)
			}
			if op.Complete(ctx) == nil || op.Execute(ctx, c, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }) == nil {
				t.Fatal("terminal lifecycle replay accepted")
			}
			if len(failures) > 0 {
				t.Fatal("LIFECYCLE_ISOLATION: FAIL", failures)
			}
			t.Log("LIFECYCLE_ISOLATION: PASS active calls refuse without waiting or closing; exclusive completion; no early evidence/refund; all workers joined", mode)
		})
	}
}
