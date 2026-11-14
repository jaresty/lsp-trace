package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceassembly"
	continuation "lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourcedriver"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/managedprocess"
	product "lsp-trace/internal/privatehostproductv3"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type censusInventoryBridgeConfig struct {
	Schema, Action, Root, AccountingRoot, ExpectedHead, Attempt string
	CanonicalJob                                                []byte
	Verification                                                inv.PrivateManagedInputVerificationV2
	Header                                                      censusInventoryHeader
	Profile                                                     runtimeprofile.Selector
	Process                                                     managedprocess.Spec
	LanguageID                                                  string
	Deadline                                                    time.Time
	Limits                                                      sessionruntime.PrivateDiscoveryLimits
	Target, Mode                                                int
	RunID                                                       string
	ControllerArgs                                              []string
	Request                                                     json.RawMessage
}
type censusInventoryBridgeBinding struct {
	Root, Key, Header, Index, Target string
	Mode                             int
}

const censusInventoryBindingPrefix = "connected-inventory-v1:"

func decodeCensusInventoryBridge(raw []byte) (censusInventoryBridgeConfig, error) {
	var c censusInventoryBridgeConfig
	if censusInventoryDecode(raw, &c, 4<<20) != nil || c.Schema != "private-connected-inventory-bridge.v1" || !filepath.IsAbs(c.Root) || filepath.Clean(c.Root) != c.Root || c.Header.SourceRevision != censusInventorySourceRevision || c.Header.Workspace == "" || c.Header.Workflow == "" || len(c.Header.Workflow) > 4096 || len(c.Request) > 64<<10 || c.Target < 0 || c.Mode < 0 || c.Mode >= 7 {
		return c, errCensusInventoryPrivate
	}
	switch c.Action {
	case "discover", "setup", "register", "wait", "respond", "run", "fail", "finalize":
	default:
		return c, errCensusInventoryPrivate
	}
	if c.Action == "discover" && (!c.Limits.Valid() || !censusInventoryHash(c.ExpectedHead) || !censusInventoryHash(c.Attempt) || len(c.CanonicalJob) == 0 || len(c.CanonicalJob) > 16<<10 || !filepath.IsAbs(c.AccountingRoot)) {
		return c, errCensusInventoryPrivate
	}
	return c, nil
}
func openCensusInventoryCursor(root, key string) (censusInventoryCursorStore, continuation.ImmutableStore, continuation.HeadStore, func() error, error) {
	resolved, e := filepath.EvalSymlinks(root)
	if e != nil || resolved != root || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return censusInventoryCursorStore{}, nil, nil, func() error { return nil }, errCensusInventoryPrivate
	}
	objectRoot, headRoot, closeRoots, e := openCensusNativeCatalogRoots(root)
	if e != nil {
		return censusInventoryCursorStore{}, nil, nil, closeRoots, e
	}
	objects, e := censussourceassembly.NewObjects(objectRoot)
	if e != nil {
		closeRoots()
		return censusInventoryCursorStore{}, nil, nil, func() error { return nil }, e
	}
	heads, e := censussourceassembly.NewHeadsWithParser(headRoot, "connected-inventory/source-heads/v1", continuation.ParseHeadRevision)
	if e != nil {
		closeRoots()
		return censusInventoryCursorStore{}, nil, nil, func() error { return nil }, e
	}
	cursors, e := censussourceassembly.NewHeadsWithParser(headRoot, "connected-inventory/cursors/v1", censusInventoryCursorRevision)
	if e != nil {
		closeRoots()
		return censusInventoryCursorStore{}, nil, nil, func() error { return nil }, e
	}
	return censusInventoryCursorStore{objects, cursors, key, root}, objects, heads, closeRoots, nil
}
func censusInventoryCursorPresent(root string) (bool, error) {
	st, e := os.Lstat(root)
	if e != nil || !st.IsDir() || st.Mode().Perm() != 0700 || st.Mode()&os.ModeSymlink != 0 {
		return false, errCensusInventoryPrivate
	}
	found := 0
	for _, name := range []string{"census-source-objects-v1", "census-source-heads-v1"} {
		st, e := os.Lstat(filepath.Join(root, name))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil || !st.IsDir() || st.Mode().Perm() != 0700 || st.Mode()&os.ModeSymlink != 0 {
			return false, errCensusInventoryPrivate
		}
		found++
	}
	if found == 1 {
		return false, errCensusInventoryPrivate
	}
	return found == 2, nil
}
func runCensusInventoryBridgePrivate(options censusCLIOptions, stdout, stderr io.Writer, deps censusRunnerDependencies) int {
	fail := func(e error) int { return writeCensusFailure(stderr, censusStageConfig, options.Machine, e) }
	raw, e := applicationReadLocatorBounded(options.PrivateInventoryBridgeConfigPath, 4<<20)
	if e != nil {
		return fail(errCensusInventoryPrivate)
	}
	c, e := decodeCensusInventoryBridge(raw)
	if e != nil {
		return fail(e)
	}
	if c.Action != "discover" {
		return runCensusInventoryBridgeLifecycle(c, stdout, stderr)
	}
	// All identity/limit checks and confirmed kernel reservation precede even the
	// Starter constructor. No application HOST/construction budget pays discovery.
	selected, e := runtimeprofile.Validate(c.Profile)
	if e != nil || c.Profile.Workspace != c.Header.Workspace {
		return fail(errCensusInventoryPrivate)
	}
	members := c.Header.Plan.Partitions
	if len(members) == 0 || len(c.Header.Plan.Files) > censusMaxDiscoveredEntries {
		return fail(errCensusInventoryPrivate)
	}
	header := c.Header
	header.Schema = "private-connected-inventory-header.v1"
	headerRaw, e := censusInventoryMarshal(header, 4<<20)
	if e != nil {
		return fail(e)
	}
	ctx, cancel := context.WithDeadline(context.Background(), c.Deadline)
	defer cancel()
	// Reading an existing cursor is bounded, read-only custody selection. A fresh
	// reservation never permits replay of an already claimed source partition.
	present, e := censusInventoryCursorPresent(c.Root)
	if e != nil {
		return fail(e)
	}
	var store censusInventoryCursorStore
	var objects continuation.ImmutableStore
	var heads continuation.HeadStore
	closeRoots := func() error { return nil }
	defer func() { _ = closeRoots() }()
	var cursor censusInventoryCursor
	if present {
		store, objects, heads, closeRoots, e = openCensusInventoryCursor(c.Root, c.Header.Workflow)
		if e != nil {
			return fail(e)
		}
		cursor, e = store.load(ctx)
		if e != nil {
			return fail(e)
		}
	}
	request := continuation.Request{Plan: c.Header.Plan, PartitionSize: 16, WorkflowKey: c.Header.Workflow, HeadKey: c.Header.Workflow, Objects: objects, Heads: heads}
	if cursor.Generation > 0 {
		if censusInventoryDigest(headerRaw) != cursor.Header {
			return fail(errCensusInventoryPrivate)
		}
		previous := cursor.IndexedPrefix
		cursor, e = store.recoverTerminal(ctx, cursor, request)
		if e != nil {
			return fail(e)
		}
		if cursor.IndexedPrefix != previous {
			if closeRoots() != nil {
				return fail(errCensusInventoryPrivate)
			}
			return encodeCensusInventoryProgress(stdout, cursor, "RECOVERED_METADATA_ONLY")
		}
	}
	ordinal := cursor.IndexedPrefix
	if ordinal >= len(members) {
		if closeRoots() != nil {
			return fail(errCensusInventoryPrivate)
		}
		return encodeCensusInventoryProgress(stdout, cursor, "COMPLETE_DISCOVERY_METADATA_ONLY")
	}
	if len(members[ordinal].Members) == 0 || len(members[ordinal].Members) > 16 {
		return fail(errCensusInventoryPrivate)
	}
	construction := sessionruntime.PrivateDiscoveryConstruction{NewStarter: deps.newStarter, Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.LanguageID, Process: c.Process}, Deadline: c.Deadline, Limits: c.Limits}
	for _, f := range members[ordinal].Members {
		construction.Members = append(construction.Members, sessionruntime.PrivateDiscoveryMember{Path: f.Path, URI: f.URI, LanguageID: f.LanguageID})
	}
	memberID, runtimeID := inv.PrivateDiscoveryConstructionIdentityV2(construction)
	binding := inv.PrivateDiscoveryBindingV2{Revision: c.Header.SourceRevision, PlanIdentity: c.Header.Plan.Identity, DenominatorDigest: censusInventoryDigest(headerRaw), MemberDigest: memberID, Workspace: c.Header.Workspace, RuntimeFingerprint: runtimeID, PolicyDigest: inv.PrivateDiscoveryPolicyDigestV2(c.CanonicalJob), Ordinal: ordinal, Deadline: c.Deadline, Limits: c.Limits}
	root, e := publication.OpenRoot(c.AccountingRoot)
	if e != nil {
		return fail(errCensusInventoryPrivate)
	}
	defer root.Close()
	owner, e := inv.OpenPrivateDiscoveryOwnerV2(ctx, root, c.CanonicalJob, c.ExpectedHead, c.Verification)
	if e != nil {
		return fail(errCensusInventoryPrivate)
	}
	op, e := owner.ReservePartition(ctx, binding, c.Attempt)
	if e != nil {
		return fail(errCensusInventoryPrivate)
	}
	defer func() { _ = op.Complete(ctx) }()
	if !present {
		store, objects, heads, closeRoots, e = openCensusInventoryCursor(c.Root, c.Header.Workflow)
		if e != nil {
			return fail(e)
		}
		request.Objects = objects
		request.Heads = heads
	}
	cursor, e = store.initialize(ctx, c.Header)
	if e != nil {
		return fail(e)
	}
	// Consume any unindexed latest receipt BEFORE assembly can overwrite the head.
	cursor, e = store.recoverTerminal(ctx, cursor, request)
	if e != nil {
		return fail(e)
	}
	if cursor.IndexedPrefix != ordinal {
		return fail(errCensusInventoryPrivate)
	}
	cursor, permit, e := store.claimPartition(ctx, cursor, c, request)
	if e != nil {
		return fail(e)
	}
	var claim censusInventoryPartitionClaim
	claimRaw, e := objects.Get(ctx, permit.claim)
	if e != nil || censusInventoryDecode(claimRaw, &claim, 64<<10) != nil {
		return fail(errCensusInventoryPrivate)
	}
	guardedHeads := &censusInventoryPartitionHeads{HeadStore: heads, permit: permit, request: request, expected: claim.SourceHead}
	result, _ := runCensusPrivateInventoryPartition(ctx, censussourceassembly.Request{Plan: c.Header.Plan, PartitionSize: 16, WorkflowKey: c.Header.Workflow, HeadKey: c.Header.Workflow, Objects: objects, Heads: guardedHeads, StepBound: 1, Discover: func(ctx context.Context, o int, files []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
		if o != ordinal || permit.take(ctx, o, files) != nil {
			return censusacquisition.Discovery{}, errCensusInventoryPrivate
		}
		var discovery censusacquisition.Discovery
		e := op.Execute(ctx, construction, func(ctx context.Context, s *sessionruntime.PrivateDiscoverySession) error {
			var e error
			discovery, e = (censusacquisition.DiscoveryAdapter{Workspace: c.Header.Workspace, Files: censusacquisition.StaticFiles(files), Supplier: censusPrivateDiscoverySupplier{s}, Client: censusProductionDiscoveryClient{productionDiscoveryClient: productionDiscoveryClient{privateDiscovery: s}, callHierarchy: s.SupportsCallHierarchy()}, Limits: censusacquisition.DiscoveryLimits{MaxFiles: 16, MaxSymbols: 1024, MaxDepth: 32, MaxStringBytes: 4096}}).Discover(ctx, censusacquisition.SessionIdentity{SessionID: s.SessionID(), Generation: s.Generation()})
			return e
		})
		if e != nil {
			return censusacquisition.Discovery{}, e
		}
		return discovery, nil
	}})
	if result.Stop != censussourcedriver.StopComplete && result.Stop != censussourcedriver.StopBoundReached {
		return fail(errCensusInventoryPrivate)
	}
	cursor, e = store.load(ctx)
	if e != nil {
		return fail(e)
	}
	cursor, e = store.recoverTerminal(ctx, cursor, request)
	if e != nil {
		return fail(e)
	}
	if e = op.Complete(ctx); e != nil {
		return fail(e)
	}
	if e = closeRoots(); e != nil {
		return fail(e)
	}
	return encodeCensusInventoryProgress(stdout, cursor, "PARTITION_PROGRESS_METADATA_ONLY")
}
func encodeCensusInventoryProgress(out io.Writer, c censusInventoryCursor, status string) int {
	e := json.NewEncoder(out).Encode(struct {
		Schema, Status, Cursor                        string
		IndexedPartitions, RetainedTargets, Authority int
		FeatureIdentity, Completeness                 string
	}{"private-connected-inventory-progress.v1", status, c.Selector, c.IndexedPrefix, len(c.Targets), 0, "UNRESOLVED", "UNKNOWN"})
	if e != nil {
		return 1
	}
	return 0
}
func runCensusInventoryBridgeLifecycle(c censusInventoryBridgeConfig, out, errout io.Writer) int {
	fail := func() int { return writeCensusFailure(errout, censusStageConfig, true, errCensusInventoryPrivate) }
	present, e := censusInventoryCursorPresent(c.Root)
	if e != nil || !present {
		return fail()
	}
	store, _, _, closeRoots, e := openCensusInventoryCursor(c.Root, c.Header.Workflow)
	if e != nil {
		return fail()
	}
	defer closeRoots()
	ctx := context.Background()
	cursor, e := store.load(ctx)
	if e != nil || c.Target >= len(cursor.Targets) {
		return fail()
	}
	header := c.Header
	header.Schema = "private-connected-inventory-header.v1"
	headerRaw, e := censusInventoryMarshal(header, 4<<20)
	if e != nil || censusInventoryDigest(headerRaw) != cursor.Header {
		return fail()
	}
	target := cursor.Targets[c.Target]
	if c.Action == "setup" {
		q, e := product.DecodeInventoryRequestV3(c.Request)
		if e != nil || len(q.Inventory.NativeOwners) != 1 || len(q.Inventory.Targets) != 1 || len(q.Inventory.Manifest.Leads) != 1 {
			return fail()
		}
		raw, e := store.objects.Get(ctx, target.Record)
		var retained censusInventoryTargetRecord
		if e != nil || censusInventoryDecode(raw, &retained, 4<<20) != nil {
			return fail()
		}
		requested := q.Inventory.Targets[0]
		if requested.URI != retained.Target.URI || requested.Line != retained.Target.SelectionRange.Start.Line || requested.Character != retained.Target.SelectionRange.Start.Character || requested.Revision != censusInventorySourceRevision || requested.Workspace != c.Header.Workspace {
			return fail()
		}
		// The full immutable header is named by the frozen q, not replaced with a
		// one-target denominator. No discovery value supplies a native owner.
		binding := censusInventoryBridgeBinding{Root: c.Root, Key: c.Header.Workflow, Header: cursor.Header, Index: cursor.IndexRecord, Target: target.Record, Mode: c.Mode}
		encoded, e := censusInventoryMarshal(binding, 8192)
		if e != nil || q.Inventory.Manifest.Scope != c.Header.Scope || q.Inventory.Manifest.Provenance != censusInventoryBindingPrefix+string(encoded) || len(q.Inventory.Manifest.UntouchedGaps) != 1 || q.Inventory.Manifest.UntouchedGaps[0] != "remaining-denominator:"+cursor.Header+":"+cursor.IndexRecord {
			return fail()
		}
		input, e := q.Inventory.NativeOwners[0].DescriptorInput()
		if e != nil || product.InventoryNativeTargetV3(input) != requested || requested.LeadID != retained.Target.SymbolIdentity || censusInventoryNativeMode(q.Inventory.NativeOwners[0]) != c.Mode || !product.InventoryNativeDeadlineValidV3(ctx, q.Inventory.NativeOwners[0].Deadline) {
			return fail()
		}
		state := target.Modes[c.Mode]
		if state.State == "PENDING" {
			next := state
			next.State = "AVAILABLE"
			cursor, e = store.transition(ctx, cursor, c.Target, c.Mode, "PENDING", next)
			if e != nil {
				return fail()
			}
			state = next
		}
		if state.State != "AVAILABLE" || !censusInventoryHash(c.Attempt) {
			return fail()
		}
		qref, e := store.put(ctx, json.RawMessage(c.Request))
		if e != nil {
			return fail()
		}
		state.State = "CONFIG_FROZEN"
		state.QDigest = q.ApprovalSHA256
		state.QSelector = qref
		state.Attempt = c.Attempt
		state.Predecessor = cursor.Selector
		cursor, e = store.transition(ctx, cursor, c.Target, c.Mode, "AVAILABLE", state)
		if e != nil {
			return fail()
		}
		var setupOut bytes.Buffer
		permit := &censusInventorySetupPermit{cursor: cursor.Selector, qDigest: q.ApprovalSHA256}
		code := runApplicationInventoryControllerV3([]string{"setup", "connected"}, c.Request, bytes.NewReader(nil), &setupOut, errout, nil, nil, permit)
		var setup struct{ Schema, RunID string }
		if code != 0 || setupOut.Len() > 64<<10 || json.Unmarshal(setupOut.Bytes(), &setup) != nil || setup.Schema != "private-application-inventory-host-setup.v3" || !censusInventoryHash(setup.RunID) {
			state.State = "INDETERMINATE"
			_, _ = store.transition(ctx, cursor, c.Target, c.Mode, "CONFIG_FROZEN", state)
			return fail()
		}
		state.RunID = setup.RunID
		cursor, e = store.transition(ctx, cursor, c.Target, c.Mode, "CONFIG_FROZEN", state)
		if e != nil {
			return fail()
		}
		if _, e = out.Write(setupOut.Bytes()); e != nil {
			return 1
		}
		return 0
	}
	state := target.Modes[c.Mode]
	if state.State == "CONFIG_FROZEN" && state.RunID == "" {
		state.State = "INDETERMINATE"
		_, _ = store.transition(ctx, cursor, c.Target, c.Mode, "CONFIG_FROZEN", state)
		return fail()
	}
	if state.State != "CONFIG_FROZEN" && state.State != "CLAIMED" && state.State != "INDETERMINATE" && state.State != "COMPLETED" {
		return fail()
	}
	frozen, e := store.objects.Get(ctx, state.QSelector)
	if e != nil || len(frozen) > 64<<10 {
		return fail()
	}
	q, e := product.DecodeInventoryRequestV3(frozen)
	if e != nil || q.ApprovalSHA256 != state.QDigest || !censusInventoryHash(c.RunID) || state.RunID != c.RunID {
		return fail()
	}
	if c.Action == "finalize" {
		if len(c.ControllerArgs) != 3 || c.ControllerArgs[0] != "finalize" || c.ControllerArgs[2] != c.RunID || len(c.ControllerArgs[1]) > 4096 || state.RunID != c.RunID {
			return fail()
		}
		// Invoke the existing F1/F2 wrapper, not an alternate finalization ledger.
		return runPrivateHostApplication(c.ControllerArgs, os.Stdin, out, errout)
	}
	args := []string{c.Action, "connected", c.RunID}
	if c.Action == "register" || c.Action == "wait" || c.Action == "respond" {
		if len(c.ControllerArgs) < 3 || len(c.ControllerArgs) > 16 || c.ControllerArgs[0] != c.Action || c.ControllerArgs[2] != c.RunID {
			return fail()
		}
		for _, arg := range c.ControllerArgs {
			if len(arg) > 4096 {
				return fail()
			}
		}
		return runApplicationInventoryControllerV3(c.ControllerArgs, frozen, os.Stdin, out, errout, nil, nil)
	}
	return runApplicationInventoryControllerV3(args, frozen, bytes.NewReader(nil), out, errout, nil, nil)
}

// The actual existing controller calls this before opening/spending its journal.
// A frozen configuration cannot bypass consumption through the ordinary V3 CLI.
type censusInventorySetupPermit struct {
	used            atomic.Bool
	cursor, qDigest string
}

func claimCensusInventoryConnectedQ(q product.InventoryRequestV3, action, runID string, setupPermit ...*censusInventorySetupPermit) (*censusInventoryConnectedClaim, error) {
	provenance := q.Inventory.Manifest.Provenance
	if len(provenance) < len(censusInventoryBindingPrefix) || provenance[:len(censusInventoryBindingPrefix)] != censusInventoryBindingPrefix {
		return nil, nil
	}
	var binding censusInventoryBridgeBinding
	if censusInventoryDecode([]byte(provenance[len(censusInventoryBindingPrefix):]), &binding, 8192) != nil {
		return nil, errCensusInventoryPrivate
	}
	present, e := censusInventoryCursorPresent(binding.Root)
	if e != nil || !present {
		return nil, errCensusInventoryPrivate
	}
	store, _, _, closeRoots, e := openCensusInventoryCursor(binding.Root, binding.Key)
	if e != nil {
		return nil, e
	}
	claim := &censusInventoryConnectedClaim{store: store, close: closeRoots, mode: binding.Mode, q: q, runID: runID}
	fail := func() (*censusInventoryConnectedClaim, error) { closeRoots(); return nil, errCensusInventoryPrivate }
	c, e := store.load(context.Background())
	if e != nil || c.Header != binding.Header || binding.Mode < 0 || binding.Mode >= 7 {
		return fail()
	}
	target := -1
	for i, t := range c.Targets {
		if t.Record == binding.Target {
			target = i
			break
		}
	}
	if target < 0 {
		return fail()
	}
	state := c.Targets[target].Modes[binding.Mode]
	if state.QDigest != q.ApprovalSHA256 || !censusInventoryHash(binding.Index) || len(q.Inventory.NativeOwners) != 1 || censusInventoryNativeMode(q.Inventory.NativeOwners[0]) != binding.Mode {
		return fail()
	}
	frozen, e := store.objects.Get(context.Background(), state.QSelector)
	expected, _ := json.Marshal(q)
	if e != nil || !bytes.Equal(frozen, expected) {
		return fail()
	}
	claim.target = target
	if action == "setup" {
		if state.State != "CONFIG_FROZEN" || state.RunID != "" || len(setupPermit) != 1 || setupPermit[0] == nil || setupPermit[0].cursor != c.Selector || setupPermit[0].qDigest != q.ApprovalSHA256 || !setupPermit[0].used.CompareAndSwap(false, true) {
			return fail()
		}
		closeRoots()
		return nil, nil
	}
	if state.RunID == "" || state.RunID != runID {
		return fail()
	}
	if action == "register" {
		if state.State != "CONFIG_FROZEN" {
			return fail()
		}
		closeRoots()
		return nil, nil
	}
	if action == "wait" || action == "respond" {
		if state.State != "CONFIG_FROZEN" && (state.RunID != runID || state.State != "CLAIMED" && (action != "wait" || state.State != "COMPLETED" && state.State != "INDETERMINATE")) {
			return fail()
		}
		closeRoots()
		return nil, nil
	}
	if action == "fail" {
		if state.State != "CLAIMED" && state.State != "INDETERMINATE" || state.RunID != runID {
			return fail()
		}
		closeRoots()
		return nil, nil
	}
	if action != "run" || state.State != "CONFIG_FROZEN" || !censusInventoryHash(runID) {
		return fail()
	}
	state.State = "CLAIMED"
	state.RunID = runID
	c, e = store.transition(context.Background(), c, target, binding.Mode, "CONFIG_FROZEN", state)
	if e != nil {
		return fail()
	}
	claim.cursor = c
	return claim, nil
}

type censusInventoryConnectedClaim struct {
	store        censusInventoryCursorStore
	close        func() error
	cursor       censusInventoryCursor
	target, mode int
	q            product.InventoryRequestV3
	runID        string
	completed    bool
}

func (c *censusInventoryConnectedClaim) complete(a *product.ApplicationJournal, id string) error {
	if c == nil || !censusInventoryHash(id) || a == nil || a.Head().LastCompletePublicationSHA256 != id || a.Head().CursorSHA256 != id {
		return errCensusInventoryPrivate
	}
	// The sole caller has closed the original writer and reopened this exact q
	// through OpenApplication: Head is the original fixed-tail verifier's readback.
	var p product.InventoryPublicationV3
	if a.GetRecord(id, &p) != nil || p.RunID != c.runID || p.RequestSHA256 != c.q.ApprovalSHA256 || product.VerifyInventoryPublicationV3(p, c.q) != nil {
		return errCensusInventoryPrivate
	}
	state := c.cursor.Targets[c.target].Modes[c.mode]
	state.State = "COMPLETED"
	state.Publication = id
	next, e := c.store.transition(context.Background(), c.cursor, c.target, c.mode, "CLAIMED", state)
	if e != nil {
		return e
	}
	c.cursor = next
	c.completed = true
	return nil
}
func (c *censusInventoryConnectedClaim) finish() {
	if c == nil {
		return
	}
	if !c.completed {
		state := c.cursor.Targets[c.target].Modes[c.mode]
		state.State = "INDETERMINATE"
		_, _ = c.store.transition(context.Background(), c.cursor, c.target, c.mode, "CLAIMED", state)
	}
	_ = c.close()
}
func censusInventoryNativeMode(owner product.InventoryNativeOwnerV3) int {
	bits := 0
	for _, kind := range owner.Kinds {
		bit := 0
		switch string(kind) {
		case "CALLS":
			bit = 1
		case "REFERENCES_SYMBOL":
			bit = 2
		case "RESOLVES_TO_DEFINITION":
			bit = 4
		default:
			return -1
		}
		if bits&bit != 0 {
			return -1
		}
		bits |= bit
	}
	name := [8]string{"", "C", "R", "CR", "D", "CD", "RD", "CRD"}[bits]
	for i, mode := range censusInventoryModes {
		if mode == name {
			return i
		}
	}
	return -1
}
