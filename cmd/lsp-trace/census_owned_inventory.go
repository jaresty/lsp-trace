//go:build darwin

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	ai "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
	dw "lsp-trace/internal/describeworker"
	is "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/managedprocess"
	product "lsp-trace/internal/privatehostproductv3"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/scopejob"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/sessionruntime"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// This command adapter has no second execution ledger/controller. Storage
// reservations are independent of execution authority. All effects and exact
// transitions belong to the existing inventory owner. The constructor callback
// is process-local dependency injection, never a caller DTO or exported handle.
type censusInventoryConstruction func(ai.InventoryNativeBatchV1, time.Time) (sessionruntime.PrivateDiscoveryConstruction, error)
type censusOwnerView struct {
	StoragePolicy                                                string
	Storage                                                      *publication.StorageStatus `json:",omitempty"`
	Schema, Store, RunID, Head, TaskID, Phase, Status, JobExpiry string
	Used, Limit, Packets                                         uint64
	Admissions                                                   int
	Enumeration                                                  *ai.InventoryEnumerationStatus `json:",omitempty"`
	Capability, Qualification                                    string
	Operations                                                   map[string][]string
}

const censusOwnerQualification = "BOUNDED_17_MEMBER_16_PLUS_1_PROFILE; two query-side R admissions only; exactly one Function/Method in each batch's first sorted member; revision labels are not fresh Git attestation; D and CALLS not acquired here; all other feature meaning and provider/whole-repository completeness UNKNOWN; nested Enumeration flags describe the metadata-only layer, not command availability; storage quota when present is owned-store file-content lengths plus live-copy reservations, not allocated blocks or machine resources; operation eligibility is not a capacity grant; actor labels are not authenticated independence"

func censusOwnerStatus(s ai.DurableStore, ctx context.Context, run string) (censusOwnerView, error) {
	st, e := s.JobStatus(ctx, run)
	if e != nil {
		return censusOwnerView{}, e
	}
	v := censusOwnerView{Schema: "lsp-trace.census-owner.v1", Store: s.Directory, RunID: st.RunID, Head: st.Head, TaskID: st.TaskID, Phase: st.Phase, Status: st.Status, JobExpiry: st.JobExpiry, Used: st.Used, Limit: st.Limit, Packets: st.Packets, Admissions: st.Admissions, Enumeration: st.Enumeration, Capability: "UNSUPPORTED_SCOPE_OR_PROFILE", Qualification: censusOwnerQualification, Operations: map[string][]string{"status": censusOwnerArgv(s.Directory, "status", run)}}
	v.StoragePolicy = "HISTORICAL_NO_STORAGE_QUOTA"
	v.Storage, e = publication.ReadStorageStatus(ctx, s.Root)
	if v.Storage != nil {
		v.StoragePolicy = "OWNED_STORE_CONTENT_LENGTH_V1"
	}
	if e != nil {
		return censusOwnerView{}, e
	}
	if st.Status == "PARTIAL_UNCERTAIN" {
		v.Capability = "SPENT_OR_IN_PROGRESS_NO_REPLAY"
		return v, nil
	}
	if st.TaskID != "" {
		v.Capability = "SEMANTIC_OR_PUBLICATION"
		v.Operations["task"] = censusOwnerArgv(s.Directory, "task", run, st.TaskID)
		return v, nil
	}
	if st.Limit != cat.InventoryReferenceLimitV1 {
		return v, nil
	}
	if st.Phase == "Admitted" && st.Status == "PARTIAL" && st.Admissions == 2 {
		v.Capability = "READY_TO_SEAL"
		v.Operations["seal"] = censusOwnerArgv(s.Directory, "seal", run, st.Head)
		return v, nil
	}
	if st.Phase == "ReadyForAcquisition" && st.Status == "READY" && st.Enumeration != nil && st.Enumeration.Closed && !st.Enumeration.UnknownRemainder && st.Enumeration.Counts.Selected == 17 {
		v.Capability = "BOUNDED_ACQUISITION_READY"
		v.Operations["acquire"] = censusOwnerArgv(s.Directory, "acquire", run, st.Head)
	}
	return v, nil
}
func runCensusOwnerStart(o censusCLIOptions, out, diagnostic io.Writer) int {
	fail := func(e error) int { return writeCensusFailure(diagnostic, censusStageConfig, true, e) }
	if o.StorageBytes < 128<<10 || o.StorageBytes > 1<<50 {
		return fail(publication.ErrStorageIntegrity)
	}
	if o.OwnedLegacyControls {
		return fail(errors.New("bounded owner uses frozen limits and external semantic tasks; legacy acquisition/catalog controls require explicit --workspace route"))
	}
	if o.InventoryInterpreter == "" || o.InventoryReviewer == "" || o.InventoryInterpreter == o.InventoryReviewer {
		return fail(ai.ErrDurableInventory)
	}
	timing, e := cat.NewInventorySemanticPolicyV1(o.SemanticWindow)
	if e != nil {
		return fail(e)
	}
	workspace, e := filepath.Abs(o.Workspace)
	if e != nil {
		return fail(e)
	}
	workspace, e = filepath.EvalSymlinks(workspace)
	if e != nil {
		return fail(e)
	}
	if o.Profile == "" && o.Server == "" {
		for _, marker := range []string{"package.json", "Cargo.toml", "pyproject.toml", "CMakeLists.txt"} {
			if _, err := os.Lstat(filepath.Join(workspace, marker)); err == nil || !os.IsNotExist(err) {
				return fail(errors.New("ambiguous default project profile; declare --profile explicitly"))
			}
		}
	}
	profile, selection, e := freezeCensusSelection(workspace, o.Profile)
	if e != nil {
		return fail(e)
	}
	enumeration, e := scopejob.Canonical(scopejob.Config{Workspace: workspace, Roots: o.Sources, Includes: o.Includes, Excludes: o.Excludes, Prunes: o.PrunedDirectories, Profile: profile, Language: selection.DefaultLanguage, Selection: selection, Budget: scopejob.DefaultBudget()})
	if e != nil {
		return fail(e)
	}
	config, e := ai.EnumerationConfig(enumeration)
	if e != nil {
		return fail(e)
	}
	// Unsupported explicit runtime overrides remain metadata-only; never silently
	// replace their requested backend or drop eligible files to fit this profile.
	if profile == "gopls" && o.Server == "" && o.ConfigPath == "" && len(o.ServerArgs) == 0 {
		pin := cat.NativeDigestV3([]byte("UNSPECIFIED_CALLER_DECLARATION_NO_EXECUTION_AUTHENTICATION"))
		a := ai.OwnedInventoryConfig{Provenance: "ordinary census bounded original-R admission; not complete feature coverage", Interpreter: o.InventoryInterpreter, Policy: sc.DurableDescribePolicyV1{Reviewer: o.InventoryReviewer, RolePrompts: [2]string{product.InventoryRolePromptV3("interpreter"), product.InventoryRolePromptV3("reviewer")}}, Pins: dw.HostPinsV2{ModelSHA256: pin, WorkerSHA256: pin, GrammarSHA256: pin}, ResponseLimits: dw.SuppliedResponseLimits{MaxAdapterBytes: cat.InventoryConnectedAdapterV1, MaxSemanticBytes: cat.InventoryConnectedSemanticV1}, TaskWindow: 30 * time.Second}
		config, e = ai.SemanticAdmissionConfigV1(enumeration, a, timing)
		if e != nil {
			return fail(e)
		}
	}
	config.EnumerationRequireGoRoot = o.Profile == "" && o.Server == ""
	dir, _, e := preflightCensusQuotaStore(workspace, o.PublicationRoot)
	if e != nil {
		return fail(e)
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		return fail(e)
	}
	defer root.Close()
	s := ai.DurableStore{Root: root, Directory: dir}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if e = publication.ConfigureStorage(ctx, root, o.StorageBytes); e != nil {
		return fail(e)
	}
	st, e := s.Create(ctx, config)
	if st.RunID != "" {
		_ = json.NewEncoder(diagnostic).Encode(struct {
			Schema, RunID, Store string
			StatusOperation      []string
		}{"lsp-trace.census-owner-start.v1", st.RunID, dir, censusOwnerArgv(dir, "status", st.RunID)})
	}
	if e == nil {
		st, e = s.Enumerate(ctx, st.RunID, st.Head)
	}
	if e != nil {
		return fail(e)
	}
	v, e := censusOwnerStatus(s, ctx, st.RunID)
	e = errors.Join(e, root.Close())
	if e != nil {
		return fail(e)
	}
	if e = json.NewEncoder(out).Encode(v); e != nil {
		return fail(e)
	}
	return 0
}
func runCensusOwnerOperation(args []string, input io.Reader, out, diagnostic io.Writer) int {
	return runCensusOwnerOperationWithFactory(args, input, out, diagnostic, censusProductionInventoryConstruction)
}
func runCensusOwnerOperationWithFactory(args []string, input io.Reader, out, diagnostic io.Writer, construct censusInventoryConstruction) int {
	if len(args) == 0 || (args[0] != "status" && args[0] != "acquire" && args[0] != "seal") {
		return runCensusDurable(args, input, out, diagnostic)
	}
	fail := func(e error) int {
		status := "REFUSED_OR_UNCERTAIN"
		if errors.Is(e, publication.ErrStorageCapacity) {
			status = "STORAGE_CAPACITY_REFUSED_OR_UNCERTAIN"
		}
		_ = json.NewEncoder(diagnostic).Encode(struct{ Schema, Status, Reason string }{"lsp-trace.census-owner-refusal.v1", status, "Configuration, custody, integrity, lease or accounting refused/unconfirmed. Observe the previously returned status/current-task operation and Storage accounting; exhaustion may preserve an accepted prefix or leave an attempt spent. No replay, reset, capacity extension or automatic retry."})
		return 1
	}
	// Closed public grammar. No caller manifest, executable, capability or source
	// object is accepted. All non-status mutation requires the exact head token.
	n := len(args)
	if n < 4 || args[n-2] != "--store" || args[n-1] == "" || (args[0] == "status" && n != 4) || (args[0] != "status" && n != 5) {
		return fail(ai.ErrDurableInventory)
	}
	root, dir, e := openSelectedCensusStore("", args[n-1])
	if e != nil {
		return fail(e)
	}
	defer root.Close()
	s := ai.DurableStore{Root: root, Directory: dir}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	run := args[1]
	switch args[0] {
	case "acquire":
		e = censusAcquireInventoryBatch(ctx, s, run, args[2], construct)
	case "seal":
		var task ai.DurableTask
		task, e = s.Seal(ctx, run, args[2])
		e = errors.Join(e, root.Close())
		if e != nil {
			return fail(e)
		}
		if e = json.NewEncoder(out).Encode(censusOwnerTask(task, dir)); e != nil {
			return fail(e)
		}
		return 0
	}
	v, statusErr := censusOwnerStatus(s, ctx, run)
	if statusErr == nil {
		if err := json.NewEncoder(out).Encode(v); err != nil {
			e = errors.Join(e, err)
		}
	}
	e = errors.Join(e, statusErr, root.Close())
	if e != nil {
		return fail(e)
	}
	return 0
}
func censusAcquireInventoryBatch(ctx context.Context, s ai.DurableStore, run, expected string, construct censusInventoryConstruction) error {
	batch, e := s.NativeDiscoveryBatchV1(ctx, run, expected)
	if e != nil {
		return e
	}
	if construct == nil {
		return errors.New("native provider construction unavailable")
	}
	for _, m := range batch.Members {
		if m.LanguageID != "go" {
			return errors.New("bounded command backend supports Go only; paid population not reduced")
		}
	}
	deadline := time.Now().Add(15 * time.Second)
	c, e := construct(batch, deadline)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithDeadline(ctx, c.Deadline)
	defer cancel()
	permit, e := s.ReserveNativeDiscoveryV1(ctx, run, expected, c)
	if e != nil {
		return e
	}
	discovery, e := is.OpenInventoryFundedDiscoveryV1(ctx, permit, c)
	if e != nil {
		return e
	}
	var queryURI string
	e = discovery.Execute(ctx, c, func(ctx context.Context, r *sessionruntime.PrivateDiscoverySession) error {
		for i, m := range batch.Members {
			if err := r.Supply(ctx, m.URI, m.LanguageID); err != nil {
				return err
			}
			symbols, err := r.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: m.URI}})
			if err != nil {
				return err
			}
			if i == 0 {
				// No name literals, fixture rows or caller-provided offsets. Ambiguity is
				// refused rather than normalizing a selection into native authority.
				if len(symbols) != 1 || (symbols[0].Kind != 12 && symbols[0].Kind != 6) {
					return errors.New("first member must have exactly one Function (12) or Method (6) in bounded profile")
				}
				queryURI = m.URI
				if _, err = r.References(ctx, m.URI, symbols[0].SelectionRange.Start); err != nil {
					return err
				}
			}
		}
		return nil
	})
	complete := discovery.Complete(ctx)
	if e = errors.Join(e, complete); e != nil {
		return e
	}
	methods := discovery.OwnedMethods()
	index := -1
	for i := 0; i < methods.GroupingQueryCount(); i++ {
		q, ok := methods.GroupingQuery(i)
		if ok && q.Method == "textDocument/references" && q.URI == queryURI {
			if index != -1 {
				return errors.New("ambiguous original query")
			}
			index = i
		}
	}
	if index < 0 {
		return errors.New("original reference query unavailable")
	}
	st, e := s.JobStatus(ctx, run)
	if e != nil {
		return e
	}
	config, e := censusInventorySelectedPolicy(run, batch.DenominatorDigest)
	if e != nil {
		return e
	}
	_, e = s.ContinueSelectedAdmissionV1(ctx, run, st.Head, discovery, index, config)
	return e
}
func censusInventorySelectedPolicy(run, denominator string) (is.PrivateSelectedConfigurationV2, error) {
	d := cat.NativeDigestV3([]byte("ordinary-census-original-R-policy.v1; declarations not execution authentication"))
	input := cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "census-source-v1", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, e := cc.NewContinuationContract(input)
	if e != nil {
		return is.PrivateSelectedConfigurationV2{}, e
	}
	return is.PrivateSelectedConfigurationV2{RunID: run, SeedNamespace: "inventory-selected-v1", Lineage: "ordinary-census-owned", Mode: "R", Seed: 7, Attempts: [3]string{cat.NativeDigestV3([]byte(run + "/M")), cat.NativeDigestV3([]byte(run + "/P")), cat.NativeDigestV3([]byte(run + "/S"))}, Contract: contract, ContractInput: input, SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxObjects: 1000, MaxWork: 10000}, ExpectedSourceAuthorization: denominator}, nil
}
func censusProductionInventoryConstruction(batch ai.InventoryNativeBatchV1, deadline time.Time) (sessionruntime.PrivateDiscoveryConstruction, error) {
	profile, e := loadRequestedProfile(batch.Workspace, profileFlags{Name: "gopls"})
	if e != nil {
		return sessionruntime.PrivateDiscoveryConstruction{}, e
	}
	command := profile.Command
	if command == "" {
		command = "gopls"
	}
	command, e = exec.LookPath(command)
	if e != nil {
		return sessionruntime.PrivateDiscoveryConstruction{}, e
	}
	command, e = filepath.Abs(command)
	if e != nil {
		return sessionruntime.PrivateDiscoveryConstruction{}, e
	}
	selected, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "public-acquisition", Workspace: batch.Workspace, Profile: "gopls", EnvironmentReference: "cli"})
	if e != nil {
		return sessionruntime.PrivateDiscoveryConstruction{}, e
	}
	return sessionruntime.PrivateDiscoveryConstruction{CaptureOwnedMethods: true, Members: batch.Members, Limits: is.NewInventoryNativeProfileV1().Limits, Deadline: deadline, Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go", Process: managedprocess.Spec{Path: command, Args: profile.Args, Dir: batch.Workspace, Env: append(os.Environ(), profile.Environment...)}, Deadline: deadline}, NewStarter: productionCensusRunnerDependencies().newStarter}, nil
}
