package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedformat"
	"lsp-trace/sessionruntime"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func connectedCursorFixture(t *testing.T, n int) (*censusInventoryCursorStore, censusInventoryCursor, censusInventoryHeader, censussourcecontinuation.Request) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	os.Chmod(root, 0700)
	files := make([]censusacquisition.SourceFile, n)
	for i := range files {
		path := fmt.Sprintf("f%04d.go", i)
		files[i] = censusacquisition.SourceFile{Path: path, URI: (&url.URL{Scheme: "file", Path: filepath.Join(root, path)}).String(), LanguageID: "go"}
	}
	plan, e := censussourceplan.Build(files, root, 16)
	if e != nil {
		t.Fatal(e)
	}
	h := censusInventoryHeader{Schema: "private-connected-inventory-header.v1", SourceRevision: "e1b9c3b13ace77cd015ebe0f6c4bfe63d970ba73", Workspace: root, Workflow: "synthetic-connected", PlanIdentity: plan.Identity, Scope: "full synthetic denominator", Provenance: "offline synthetic metadata only", Plan: plan, UntouchedGaps: []string{}, Exclusions: []string{"protected:2", "qualification:216289", "tests:1542"}}
	h.UntouchedGaps = make([]string, 6540)
	for i := range h.UntouchedGaps {
		h.UntouchedGaps[i] = fmt.Sprintf("synthetic-gap-%04d", i)
	}
	store, objects, heads, closeRoots, e := openCensusInventoryCursor(root, h.Workflow)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { closeRoots() })
	c, e := store.initialize(context.Background(), h)
	if e != nil {
		t.Fatal(e)
	}
	request := censussourcecontinuation.Request{Plan: plan, PartitionSize: 16, WorkflowKey: h.Workflow, HeadKey: h.Workflow, Objects: objects, Heads: heads}
	return &store, c, h, request
}
func connectedTransition(s *censusInventoryCursorStore, c censusInventoryCursor, record string, mode int, to string, q []byte, qd, run string) (censusInventoryCursor, error) {
	target := -1
	for i, t := range c.Targets {
		if t.Record == record {
			target = i
			break
		}
	}
	if target < 0 {
		return c, errCensusInventoryPrivate
	}
	if to == "CLAIMED" && c.Targets[target].Modes[mode].State == "CONFIG_FROZEN" && c.Targets[target].Modes[mode].RunID == "" {
		ticket := c.Targets[target].Modes[mode]
		ticket.RunID = run
		var e error
		c, e = s.transition(context.Background(), c, target, mode, "CONFIG_FROZEN", ticket)
		if e != nil {
			return c, e
		}
	}
	next := c.Targets[target].Modes[mode]
	from := next.State
	next.State = to
	if to == "CONFIG_FROZEN" {
		id, e := s.put(context.Background(), json.RawMessage(q))
		if e != nil {
			return c, e
		}
		next.QSelector = id
		next.QDigest = qd
		next.Attempt = censusInventoryDigest([]byte("synthetic attempt"))
		next.Predecessor = c.Selector
	}
	if to == "CLAIMED" {
		next.RunID = run
	}
	return s.transition(context.Background(), c, target, mode, from, next)
}
func connectedTarget(t *testing.T, h censusInventoryHeader) censusacquisition.PreparedTarget {
	t.Helper()
	f := h.Plan.Partitions[0].Members[0]
	position := lsp.Position{Line: 0, Character: 0}
	seed, e := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "census-000000", Path: f.Path, Line: 1, Column: 1}}}}, h.Workspace)
	if e != nil {
		t.Fatal(e)
	}
	return censusacquisition.PreparedTarget{URI: f.URI, LanguageID: f.LanguageID, Name: "Synthetic", Kind: 12, CensusOrdinal: 0, CanonicalSeedV2: seed, SymbolIdentity: fmt.Sprintf("%s#0:0:12:Synthetic:0", f.Path), Range: lsp.Range{Start: position, End: position}, SelectionRange: lsp.Range{Start: position, End: position}}
}

type connectedMetadataSupplier struct{}

func (connectedMetadataSupplier) Supply(context.Context, censusacquisition.SourceFile) error {
	return nil
}

type connectedMetadataClient struct{ first string }

func (connectedMetadataClient) SupportsDocumentSymbols() bool { return true }
func (connectedMetadataClient) SupportsCallHierarchy() bool   { return true }
func (c connectedMetadataClient) DocumentSymbols(_ context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	if p.TextDocument.URI != c.first {
		return nil, nil
	}
	return []lsp.DocumentSymbol{{Name: "Synthetic", Kind: 12}}, nil
}
func (c connectedMetadataClient) PrepareCallHierarchy(_ context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return []lsp.CallHierarchyItem{{Name: "Synthetic", Kind: 12, URI: p.TextDocument.URI}}, nil
}
func connectedIndex(t *testing.T, store *censusInventoryCursorStore, c censusInventoryCursor, h censusInventoryHeader, r censussourcecontinuation.Request, nonempty bool) censusInventoryCursor {
	t.Helper()
	terminal := censussourcecontinuation.Terminal{PlanIdentity: h.Plan.Identity, Ordinal: 0, SourceFiles: h.Plan.Partitions[0].Members, Outcome: censussourcecontinuation.OutcomeSuccessfulEmpty, Discovery: censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosureSuccessful}}
	first := ""
	if nonempty {
		terminal.Outcome = censussourcecontinuation.OutcomeCompleteNonempty
		first = h.Plan.Partitions[0].Members[0].URI
	}
	var discoveryErr error
	terminal.Discovery, discoveryErr = (censusacquisition.DiscoveryAdapter{Workspace: h.Workspace, Files: censusacquisition.StaticFiles(h.Plan.Partitions[0].Members), Supplier: connectedMetadataSupplier{}, Client: connectedMetadataClient{first}}).Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: "synthetic-metadata", Generation: 1})
	if discoveryErr != nil || len(terminal.Discovery.FileLedger.Entries) != len(terminal.SourceFiles) {
		t.Fatal("ASSERT_CONNECTED_METADATA_ALL_MEMBERS_ACCOUNTED", discoveryErr)
	}
	// Synthetic metadata custody uses the same claim/permit/receipt association,
	// not a bypass granting native execution or a fake kernel permission.
	cfg := censusInventoryBridgeConfig{Root: store.root, Header: h, CanonicalJob: []byte("synthetic metadata declaration only"), ExpectedHead: censusInventoryDigest([]byte("metadata head")), Attempt: censusInventoryDigest([]byte("metadata attempt")), Deadline: time.Now().Add(10 * time.Second)}
	var e error
	var permit *censusInventoryPartitionPermit
	c, permit, e = store.claimPartition(context.Background(), c, cfg, r)
	if e != nil {
		t.Fatal(e)
	}
	if e = permit.take(context.Background(), 0, terminal.SourceFiles); e != nil {
		t.Fatal(e)
	}
	guarded := &censusInventoryPartitionHeads{HeadStore: r.Heads, permit: permit, request: r}
	r.Heads = guarded
	r.ExistingOrdinalZero = &terminal
	result, e := censussourcecontinuation.CoordinateNext(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	r.Expected = result.Head
	c, e = store.load(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	c, e = store.recoverTerminal(context.Background(), c, r)
	if e != nil {
		t.Fatal(e)
	}
	return c
}

// Metadata-only state tests still reserve a genuine fresh synthetic kernel
// window before custody writes. No operation Execute/provider is used here.
func custodyReserve46(t *testing.T, c censusInventoryBridgeConfig, ordinal int) {
	t.Helper()
	selected, e := runtimeprofile.Validate(c.Profile)
	if e != nil {
		t.Fatal(e)
	}
	construction := sessionruntime.PrivateDiscoveryConstruction{Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.LanguageID, Process: c.Process}, Deadline: c.Deadline, Limits: c.Limits}
	for _, f := range c.Header.Plan.Partitions[ordinal].Members {
		construction.Members = append(construction.Members, sessionruntime.PrivateDiscoveryMember{Path: f.Path, URI: f.URI, LanguageID: f.LanguageID})
	}
	member, runtime := inv.PrivateDiscoveryConstructionIdentityV2(construction)
	header, _ := censusInventoryMarshal(c.Header, 4<<20)
	binding := inv.PrivateDiscoveryBindingV2{Revision: c.Header.SourceRevision, PlanIdentity: c.Header.Plan.Identity, DenominatorDigest: censusInventoryDigest(header), MemberDigest: member, Workspace: c.Header.Workspace, RuntimeFingerprint: runtime, PolicyDigest: inv.PrivateDiscoveryPolicyDigestV2(c.CanonicalJob), Ordinal: ordinal, Deadline: c.Deadline, Limits: c.Limits}
	root, e := publication.OpenRoot(c.AccountingRoot)
	if e != nil {
		t.Fatal(e)
	}
	owner, e := inv.OpenPrivateDiscoveryOwnerV2(context.Background(), root, c.CanonicalJob, c.ExpectedHead, c.Verification)
	if e != nil {
		root.Close()
		t.Fatal(e)
	}
	op, e := owner.ReservePartition(context.Background(), binding, c.Attempt)
	if e != nil {
		root.Close()
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = op.Complete(context.Background()); _ = root.Close() })
}

type custodyCASBarrier46 struct {
	censussourcecontinuation.HeadStore
	entered chan<- struct{}
	release <-chan struct{}
}

func (h *custodyCASBarrier46) CompareAndSwap(ctx context.Context, key string, expected censussourcecontinuation.HeadRevision, raw []byte) censussourcecontinuation.CASResult {
	h.entered <- struct{}{}
	<-h.release
	return h.HeadStore.CompareAndSwap(ctx, key, expected, raw)
}

type custodyTamperedObjects46 struct {
	censussourcecontinuation.ImmutableStore
	key string
	raw []byte
}

func (s custodyTamperedObjects46) Get(ctx context.Context, key string) ([]byte, error) {
	if key == s.key {
		return append([]byte(nil), s.raw...), nil
	}
	return s.ImmutableStore.Get(ctx, key)
}

type custodyLostRead46 struct {
	censussourcecontinuation.HeadStore
	fail bool
}

func (h *custodyLostRead46) CompareAndSwap(ctx context.Context, key string, expected censussourcecontinuation.HeadRevision, raw []byte) censussourcecontinuation.CASResult {
	r := h.HeadStore.CompareAndSwap(ctx, key, expected, raw)
	if r.Outcome == censussourcecontinuation.CASCommitted {
		h.fail = true
	}
	return r
}
func (h *custodyLostRead46) Load(ctx context.Context, key string) ([]byte, error) {
	if h.fail {
		h.fail = false
		return nil, errors.New("scripted crash after custody CAS")
	}
	return h.HeadStore.Load(ctx, key)
}
func TestCensusInventoryPrivatePartitionCustodyState46(t *testing.T) {
	t.Run("same-predecessor-CAS-race", func(t *testing.T) {
		s, c, h, r := connectedCursorFixture(t, 16)
		a, b := custodyFundedConfig46(t, h), custodyFundedConfig46(t, h)
		custodyReserve46(t, a, 0)
		custodyReserve46(t, b, 0)
		entered, release := make(chan struct{}, 2), make(chan struct{})
		racing := *s
		racing.heads = &custodyCASBarrier46{HeadStore: s.heads, entered: entered, release: release}
		done := make(chan *censusInventoryPartitionPermit, 2)
		for _, cfg := range []censusInventoryBridgeConfig{a, b} {
			go func(cfg censusInventoryBridgeConfig) {
				_, p, _ := racing.claimPartition(context.Background(), c, cfg, r)
				done <- p
			}(cfg)
		}
		<-entered
		<-entered
		close(release)
		effects := 0
		for i := 0; i < 2; i++ {
			p := <-done
			if p != nil {
				if e := p.take(context.Background(), 0, h.Plan.Partitions[0].Members); e != nil {
					t.Fatal(e)
				}
				effects++
			}
		}
		if effects != 1 {
			t.Fatal("ASSERT_CUSTODY46_SAME_PREDECESSOR_CAS_WINNER", effects)
		}
		t.Log("ASSERT_CUSTODY46_SAME_PREDECESSOR_CAS_WINNER PASS")
	})
	t.Run("crash-before-callback", func(t *testing.T) {
		s, c, h, r := connectedCursorFixture(t, 16)
		cfg := custodyFundedConfig46(t, h)
		custodyReserve46(t, cfg, 0)
		fault := *s
		fault.heads = &custodyLostRead46{HeadStore: s.heads}
		_, permit, e := fault.claimPartition(context.Background(), c, cfg, r)
		if e == nil || permit != nil {
			t.Fatal("ASSERT_CUSTODY46_AMBIGUOUS_NO_PERMIT")
		}
		fresh, objects, heads, closeRoots, e := openCensusInventoryCursor(s.root, s.key)
		if e != nil {
			t.Fatal(e)
		}
		defer closeRoots()
		loaded, e := fresh.load(context.Background())
		if e != nil || len(loaded.Partitions) != 1 {
			t.Fatal("ASSERT_CUSTODY46_CRASH_CLAIM_DURABLE", e)
		}
		r.Objects = objects
		r.Heads = heads
		if _, e = fresh.recoverTerminal(context.Background(), loaded, r); e == nil {
			t.Fatal("ASSERT_CUSTODY46_CRASH_REOPEN_BLOCKED")
		}
		starts := 0
		next := custodyFundedConfig46(t, h)
		if custodyInvoke46(t, next, censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) { starts++; return nil, errors.New("never") }}) == 0 || starts != 0 {
			t.Fatal("ASSERT_CUSTODY46_CRASH_NO_START", starts)
		}
		if _, e = s.initialize(context.Background(), h); e != nil {
			t.Fatal(e)
		}
		after, _ := s.load(context.Background())
		if after.Selector != loaded.Selector {
			t.Fatal("ASSERT_CUSTODY46_INITIALIZE_NO_RESET")
		}
		t.Log("ASSERT_CUSTODY46_CRASH_NO_START PASS")
	})
	t.Run("receipt-index-next-and-bindings", func(t *testing.T) {
		s, c, h, r := connectedCursorFixture(t, 32)
		calls := 0
		for ordinal := 0; ordinal < 2; ordinal++ {
			cfg := custodyFundedConfig46(t, h, ordinal)
			custodyReserve46(t, cfg, ordinal)
			var permit *censusInventoryPartitionPermit
			var e error
			c, permit, e = s.claimPartition(context.Background(), c, cfg, r)
			if e != nil {
				t.Fatal(e)
			}
			// A second fresh funded declaration cannot convert CLAIMED to CLAIMED.
			other := custodyFundedConfig46(t, h, ordinal)
			custodyReserve46(t, other, ordinal)
			if _, p, e := s.claimPartition(context.Background(), c, other, r); e == nil || p != nil {
				t.Fatal("ASSERT_CUSTODY46_CLAIMED_NOT_PERMISSION")
			}
			raw, _ := r.Heads.Load(context.Background(), r.HeadKey)
			head, e := censussourcecontinuation.InspectHead(raw, h.Workflow, h.Plan.Identity, len(h.Plan.Partitions))
			if e != nil {
				t.Fatal(e)
			}
			guarded := &censusInventoryPartitionHeads{HeadStore: r.Heads, permit: permit, request: r, expected: head.Revision}
			result, _ := runCensusPrivateInventoryPartition(context.Background(), censussourceassembly.Request{Plan: h.Plan, PartitionSize: 16, WorkflowKey: h.Workflow, HeadKey: h.Workflow, Objects: s.objects, Heads: guarded, StepBound: 1, Discover: func(ctx context.Context, o int, files []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
				if e := permit.take(ctx, o, files); e != nil {
					return censusacquisition.Discovery{}, e
				}
				copyPermit := *permit
				if copyPermit.take(ctx, o, files) == nil {
					t.Fatal("ASSERT_CUSTODY46_COPIED_PERMIT_ONCE")
				}
				calls++
				return (censusacquisition.DiscoveryAdapter{Workspace: h.Workspace, Files: censusacquisition.StaticFiles(files), Supplier: connectedMetadataSupplier{}, Client: connectedMetadataClient{}}).Discover(ctx, censusacquisition.SessionIdentity{SessionID: "synthetic-custody", Generation: 1})
			}})
			if result.Err != nil {
				t.Fatal("ASSERT_CUSTODY46_GUARDED_ASSEMBLY", result.Err)
			}
			// Reopen after receipt publication, deliberately before indexing. Recovery is
			// read-only with respect to discovery and continuation; only cursor advances.
			reopened, objects, heads, closeRoots, e := openCensusInventoryCursor(s.root, s.key)
			if e != nil {
				t.Fatal(e)
			}
			current, e := reopened.load(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			rr := r
			rr.Objects = objects
			rr.Heads = heads
			bad := current
			bad.Partitions = append([]censusInventoryPartitionState(nil), current.Partitions...)
			bad.Partitions[ordinal].Receipt = censusInventoryDigest([]byte("foreign receipt"))
			if _, e := reopened.recoverTerminal(context.Background(), bad, rr); e == nil {
				t.Fatal("ASSERT_CUSTODY46_FOREIGN_RECEIPT_REFUSED")
			}
			c, e = reopened.recoverTerminal(context.Background(), current, rr)
			if e != nil || c.IndexedPrefix != ordinal+1 || c.Partitions[ordinal].State != "INDEXED" || calls != ordinal+1 {
				t.Fatal("ASSERT_CUSTODY46_RECEIPT_RECOVERY_NO_EFFECT", e, calls)
			}
			_ = closeRoots()
		}
		if len(h.UntouchedGaps) != 6540 || len(c.Partitions) != 2 || len(c.Index) != 2 {
			t.Fatal("ASSERT_CUSTODY46_DENOMINATOR_PRESERVED")
		}
		t.Log("ASSERT_CUSTODY46_RECEIPT_RECOVERY_NO_EFFECT PASS; ASSERT_CUSTODY46_COPIED_PERMIT_ONCE PASS")
	})
	t.Run("tampered-immutable-claims", func(t *testing.T) {
		s, c, h, r := connectedCursorFixture(t, 16)
		cfg := custodyFundedConfig46(t, h)
		custodyReserve46(t, cfg, 0)
		claimed, p, e := s.claimPartition(context.Background(), c, cfg, r)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := s.objects.Get(context.Background(), p.claim)
		var original censusInventoryPartitionClaim
		if json.Unmarshal(raw, &original) != nil {
			t.Fatal("claim")
		}
		mutations := []func(*censusInventoryPartitionClaim){func(x *censusInventoryPartitionClaim) { x.Header = censusInventoryDigest([]byte("alien header")) }, func(x *censusInventoryPartitionClaim) { x.Ordinal++ }, func(x *censusInventoryPartitionClaim) {
			x.CursorPredecessor = censusInventoryDigest([]byte("alien cursor"))
		}, func(x *censusInventoryPartitionClaim) {
			x.Attempt = censusInventoryDigest([]byte("alien funding attempt"))
		}, func(x *censusInventoryPartitionClaim) { x.SourceHead.Generation++ }, func(x *censusInventoryPartitionClaim) { x.Root += "/alias" }, func(x *censusInventoryPartitionClaim) { x.Members = censusInventoryDigest([]byte("alien members")) }}
		for i, mutate := range mutations {
			x := original
			mutate(&x)
			badRaw, _ := json.Marshal(x)
			bad := *s
			bad.objects = custodyTamperedObjects46{ImmutableStore: s.objects, key: p.claim, raw: badRaw}
			if _, e = bad.load(context.Background()); e == nil {
				t.Fatal("ASSERT_CUSTODY46_TAMPERED_CLAIM_REFUSED", i)
			}
		}
		if _, e = s.load(context.Background()); e != nil {
			t.Fatal(e)
		}
		if _, e = s.recoverTerminal(context.Background(), claimed, r); e == nil {
			t.Fatal("ASSERT_CUSTODY46_PARSED_CLAIM_NO_PERMISSION")
		}
		t.Log("ASSERT_CUSTODY46_TAMPERED_CLAIM_REFUSED PASS")
	})
	t.Run("expired-permit-no-renewal", func(t *testing.T) {
		s, c, h, r := connectedCursorFixture(t, 16)
		cfg := custodyFundedConfig46(t, h)
		custodyReserve46(t, cfg, 0)
		_, permit, e := s.claimPartition(context.Background(), c, cfg, r)
		if e != nil {
			t.Fatal(e)
		}
		permit.deadline = time.Now().Add(-time.Second)
		if permit.take(context.Background(), 0, h.Plan.Partitions[0].Members) == nil {
			t.Fatal("ASSERT_CUSTODY46_EXPIRED_PERMIT")
		}
		loaded, _ := s.load(context.Background())
		if _, e = s.recoverTerminal(context.Background(), loaded, r); e == nil {
			t.Fatal("ASSERT_CUSTODY46_EXPIRED_NO_REPLAY")
		}
		t.Log("ASSERT_CUSTODY46_EXPIRED_NO_REPLAY PASS")
	})
}
func TestCensusInventoryPrivateCursorEmptyRestartAndDenominator(t *testing.T) {
	store, c, h, r := connectedCursorFixture(t, 1099)
	h.UntouchedGaps = append(h.UntouchedGaps, "unauthorized-denominator-change")
	// A different full denominator cannot be silently substituted after initialize.
	if _, e := store.initialize(context.Background(), h); e == nil {
		t.Fatal("ASSERT_CONNECTED_DENOMINATOR_SUBSTITUTION_REFUSED")
	}
	raw, e := store.objects.Get(context.Background(), c.Header)
	if e != nil {
		t.Fatal(e)
	}
	if censusInventoryDecode(raw, &h, 4<<20) != nil {
		t.Fatal("header")
	}
	if len(h.Plan.Files) != 1099 || len(h.Plan.Partitions) != 69 || len(h.UntouchedGaps) != 6540 || len(h.Exclusions) != 3 {
		t.Fatal("ASSERT_CONNECTED_FULL_SOURCE_DENOMINATOR")
	}
	c = connectedIndex(t, store, c, h, r, false)
	reloaded, e := store.load(context.Background())
	if e != nil || !reflect.DeepEqual(c, reloaded) || c.IndexedPrefix != 1 || len(c.Targets) != 0 || len(c.Index) != 1 || c.Index[0].Targets != nil || c.IndexRecord == "" {
		t.Fatal("ASSERT_CONNECTED_EMPTY_INDEX_DURABLE", e)
	}
	r.Expected = censussourcecontinuation.HeadRevision{} // recovery always inspects current exact head itself
	again, e := store.recoverTerminal(context.Background(), reloaded, r)
	if e != nil || !reflect.DeepEqual(again, reloaded) {
		t.Fatal("ASSERT_CONNECTED_RESTART_NO_DISCOVERY", e)
	}
	t.Log("ASSERT_CONNECTED_EMPTY_INDEX_FULL_DENOMINATOR PASS")
}
func TestCensusInventoryPrivateCursorFreezeClaimNoReplay(t *testing.T) {
	store, c, h, r := connectedCursorFixture(t, 16)
	c = connectedIndex(t, store, c, h, r, true)
	if len(c.Targets) != 1 {
		t.Fatal("target")
	}
	target := c.Targets[0].Record
	q := []byte(`{"synthetic":"not native authority"}`)
	qd := censusInventoryDigest(q)
	var e error
	c, e = connectedTransition(store, c, target, 0, "AVAILABLE", nil, "", "")
	if e != nil {
		t.Fatal(e)
	}
	c, e = connectedTransition(store, c, target, 0, "CONFIG_FROZEN", q, qd, "")
	if e != nil {
		t.Fatal(e)
	}
	unbound := c.Targets[0].Modes[0]
	unbound.State = "CLAIMED"
	unbound.RunID = censusInventoryDigest([]byte("run"))
	if _, e := store.transition(context.Background(), c, 0, 0, "CONFIG_FROZEN", unbound); e == nil {
		t.Fatal("ASSERT_CONNECTED_SETUP_CRASH_CANNOT_CLAIM")
	}
	c, e = connectedTransition(store, c, target, 0, "CLAIMED", nil, "", censusInventoryDigest([]byte("run")))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = connectedTransition(store, c, target, 0, "CLAIMED", nil, "", censusInventoryDigest([]byte("run"))); e == nil {
		t.Fatal("ASSERT_CONNECTED_UNIQUE_CLAIM_NO_REPLAY")
	}
	c, e = connectedTransition(store, c, target, 0, "INDETERMINATE", nil, "", "")
	if e != nil {
		t.Fatal(e)
	}
	restart, e := store.load(context.Background())
	if e != nil || restart.Targets[0].Modes[0].State != "INDETERMINATE" {
		t.Fatal("ASSERT_CONNECTED_CRASH_NO_REACQUIRE", e)
	}
	for mode := 1; mode < 7; mode++ {
		if restart.Targets[0].Modes[mode].State != "PENDING" {
			t.Fatal("ASSERT_CONNECTED_ONE_MODE_NOT_SEVEN")
		}
	}
	t.Log("ASSERT_CONNECTED_FREEZE_CLAIM_NO_REPLAY PASS")
}
func TestCensusInventoryPrivateCursorCASAndTargetBinding(t *testing.T) {
	store, c, h, r := connectedCursorFixture(t, 16)
	c = connectedIndex(t, store, c, h, r, true)
	stale := c
	advanced, e := connectedTransition(store, c, c.Targets[0].Record, 0, "AVAILABLE", nil, "", "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = connectedTransition(store, stale, stale.Targets[0].Record, 1, "AVAILABLE", nil, "", ""); e == nil {
		t.Fatal("ASSERT_CONNECTED_STALE_CAS_REFUSED")
	}
	target := connectedTarget(t, h)
	target.CanonicalSeedV2 = []byte(`{}`)
	if validateCensusInventoryTarget(target, h, 0) == nil {
		t.Fatal("ASSERT_CONNECTED_SEED_BINDING_REFUSED")
	}
	target = connectedTarget(t, h)
	target.URI = "file:///foreign.go"
	if validateCensusInventoryTarget(target, h, 0) == nil {
		t.Fatal("ASSERT_CONNECTED_URI_BINDING_REFUSED")
	}
	raw, _ := json.Marshal(advanced)
	var detached censusInventoryCursor
	if censusInventoryDecode(raw, &detached, 4<<20) != nil {
		t.Fatal("canonical")
	}
	t.Log("ASSERT_CONNECTED_CURSOR_CAS_TARGET_BINDING PASS")
}
func TestCensusInventoryPrivateAssemblyOnePartitionAndLastNil(t *testing.T) {
	store, _, h, r := connectedCursorFixture(t, 16)
	terminal := censussourcecontinuation.Terminal{PlanIdentity: h.Plan.Identity, Ordinal: 0, SourceFiles: h.Plan.Partitions[0].Members, Outcome: censussourcecontinuation.OutcomeSuccessfulEmpty, Discovery: censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosureSuccessful}}
	calls := 0
	request := censussourceassembly.Request{Plan: h.Plan, PartitionSize: 16, WorkflowKey: h.Workflow, HeadKey: h.Workflow, Objects: store.objects, Heads: r.Heads, StepBound: 1, Discover: func(context.Context, int, []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
		calls++
		return terminal.Discovery, nil
	}}
	first, last := runCensusPrivateInventoryPartition(context.Background(), request)
	if first.Err != nil || last == nil || calls != 1 {
		t.Fatal("ASSERT_CONNECTED_REAL_ASSEMBLY_SINGLE_STEP", first.Err, calls)
	}
	second, last := runCensusPrivateInventoryPartition(context.Background(), request)
	if second.Err != nil || last != nil || calls != 1 {
		t.Fatal("ASSERT_CONNECTED_LAST_NIL_NO_REDISCOVERY", second.Err, calls)
	}
	raw, e := r.Heads.Load(context.Background(), r.HeadKey)
	if e != nil {
		t.Fatal(e)
	}
	inspected, e := censussourcecontinuation.InspectHead(raw, r.WorkflowKey, h.Plan.Identity, 1)
	if e != nil {
		t.Fatal(e)
	}
	r.Expected = inspected.Revision
	_, recovered, e := censussourcecontinuation.ReadHeadTerminal(context.Background(), r)
	if e != nil || recovered == nil || recovered.Outcome != censussourcecontinuation.OutcomeSuccessfulEmpty {
		t.Fatal("ASSERT_CONNECTED_LAST_NIL_HEAD_RECEIPT_RECOVERY", e)
	}
	t.Log("ASSERT_CONNECTED_REAL_ASSEMBLY_LAST_NIL_RECOVERY PASS")
}
