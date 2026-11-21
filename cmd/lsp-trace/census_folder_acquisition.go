//go:build darwin

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	product "lsp-trace/internal/privatehostproductv3"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// Acquisition-only normal-folder lane. Preparation records are metadata, not
// provider authority. Exactly one existing bounded partition enclosure is funded
// here; larger scopes refuse before runtime construction rather than resetting
// cumulative balances per partition. No result is promoted to an inventory.
func runCensusFolderAcquisition(o censusCLIOptions, cfg censusCoreConfig, stderr io.Writer, output ...io.Writer) int {
	stdout := io.Discard
	if len(output) == 1 {
		stdout = output[0]
	}
	fail := func(stage censusFailureStage, err error) int {
		return writeCensusFailure(stderr, stage, o.Machine, err)
	}
	p := cfg.folderPreparation
	if p == nil || p.workspace != o.Workspace || p.runRoot != o.PublicationRoot || cfg.folderStarter == nil || len(p.records) > censusMaxDiscoveredEntries {
		return fail(censusStageConfig, errors.New("folder preparation binding unavailable"))
	}
	if o.DownDepth > 1 || o.UpDepth > 1 {
		return fail(censusStageDiscovery, errNativeCatalogBoundReached)
	}
	files := censusacquisition.PreparedWorkspaceFiles(o.Workspace, p.records)
	// Partition only acquisition candidates. The paid preparation records and
	// full discovery denominator remain intact, including excluded/unavailable
	// entries. This metadata selection grants no runtime or original custody.
	selected, err := censusacquisition.SelectedSourceFiles(o.Workspace, files, censusacquisition.Filters{Includes: o.Includes, Excludes: o.Excludes})
	if err != nil {
		return fail(censusStageDiscovery, err)
	}
	plan, err := censussourceplan.Build(selected, o.Workspace, censusNativeSourcePartitionSize)
	if err != nil {
		return fail(censusStageDiscovery, err)
	}
	if err := censussourceplan.Validate(plan, censusNativeSourcePartitionSize); err != nil {
		return fail(censusStageDiscovery, err)
	}
	if len(plan.Partitions) != 1 {
		return fail(censusStageDiscovery, errNativeCatalogBoundReached)
	}
	// Hash bounded metadata using the existing checked marshal. This binding is
	// neither a source-content revision nor a claim of filesystem completeness.
	prepared, err := censusInventoryMarshal(p.records, 4<<20)
	if err != nil {
		return fail(censusStageDiscovery, err)
	}
	// One shared window is conservatively no longer than either caller timeout;
	// no per-request refresh can renew it.
	deadline := time.Now().Add(min(o.Timeout, o.RequestTimeout, 15*time.Second))
	if o.DurableDescribe || o.DurableInventory {
		if o.durableDeadline.IsZero() || !time.Now().Before(o.durableDeadline) {
			return fail(censusStageConfig, errNativeCatalogDeadlineReached)
		}
		if o.durableDeadline.Before(deadline) {
			deadline = o.durableDeadline
		}
	}
	parent, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	start := cfg.runner.start
	start.Deadline = deadline
	construction := sessionruntime.PrivateDiscoveryConstruction{CaptureOwnedMethods: true, NewStarter: cfg.folderStarter, Start: start, Limits: censusPrivateDiscoveryLimits(), Deadline: deadline}
	// Automatic inventory has broad structural scope. Legacy query depths do
	// not silently remove a direction; the original one-hop/resource caps remain.
	construction.CaptureCallDirections = sessionruntime.PrivateCallsIncoming | sessionruntime.PrivateCallsOutgoing
	for _, f := range plan.Files {
		construction.Members = append(construction.Members, sessionruntime.PrivateDiscoveryMember{Path: f.Path, URI: f.URI, LanguageID: f.LanguageID})
	}
	members, runtime := inventorysuccessor.PrivateDiscoveryConstructionIdentityV2(construction)
	binding := inventorysuccessor.PrivateDiscoveryBindingV2{Revision: censusInventorySourceRevision, PlanIdentity: censusInventoryDigest([]byte(plan.Identity)), DenominatorDigest: censusInventoryDigest(prepared), MemberDigest: members, Workspace: o.Workspace, RuntimeFingerprint: runtime, Deadline: deadline, Limits: construction.Limits}
	path, err := censusOwnedDirectory(o.PublicationRoot, "folder-acquisition", true)
	if err != nil {
		return fail(censusStageConfig, err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		return fail(censusStageConfig, err)
	}
	op, err := inventorysuccessor.OpenLocalFolderDiscoveryV1(ctx, root, binding, censusInventoryDigest([]byte(o.PublicationRoot)))
	if err != nil {
		return fail(censusStageAcquisition, errors.Join(err, root.Close()))
	}
	// Execute cannot invoke NewStarter until Reserve and independent readback
	// have succeeded. Its concrete runtime retains custody through cleanup.
	err = op.Execute(ctx, construction, func(ctx context.Context, s *sessionruntime.PrivateDiscoverySession) error {
		discovery, err := (censusacquisition.DiscoveryAdapter{Workspace: o.Workspace, Filters: censusacquisition.Filters{Includes: o.Includes, Excludes: o.Excludes}, Files: censusacquisition.StaticFiles(files), Supplier: censusPrivateDiscoverySupplier{s: s}, Client: s, Limits: censusacquisition.DiscoveryLimits{MaxFiles: censusNativeSourcePartitionSize, MaxSymbols: o.MaxNodes, MaxDepth: censusMaxSourceDepth, MaxStringBytes: 1 << 20}}).Discover(ctx, censusacquisition.SessionIdentity{SessionID: s.SessionID(), Generation: s.Generation()})
		if err != nil {
			return err
		}
		if !discovery.Complete {
			return errors.New("folder discovery incomplete; structural evidence unavailable")
		}
		return acquireCensusFolderRelations(ctx, s, discovery, o.MaxNodes, true, true)
	})
	// Complete records FAILED/INDETERMINATE without refund when execution fails;
	// successful readback is required before considering acquisition closed.
	completion := op.Complete(ctx)
	if err := errors.Join(err, completion, root.Close()); err != nil {
		return fail(censusStageAcquisition, err)
	}
	if !op.OwnedMethods().Available() {
		return fail(censusStageAcquisition, errors.New("folder owned method originals unavailable"))
	}
	if construction.CaptureCallDirections != 0 && !op.OwnedMethods().NativeCallsAvailable() {
		return fail(censusStageAcquisition, errors.New("folder directional originals unavailable"))
	}
	// Every retained callable gets its own one-shot, separately bounded native
	// conversion. Discovery credit is not reused and no provider is started.
	var natives []inventorysuccessor.PrivateRetainedNativeCallsV1
	for index := 0; index < op.OwnedMethods().NativeCallCount(); index++ {
		path, err := censusOwnedDirectory(o.PublicationRoot, fmt.Sprintf("folder-native-%03d", index), true)
		if err != nil {
			return fail(censusStageAssembly, err)
		}
		nativeRoot, err := publication.OpenRoot(path)
		if err != nil {
			return fail(censusStageAssembly, err)
		}
		native, transferErr := inventorysuccessor.TransferLocalFolderNativeCallsV1(ctx, nativeRoot, op, index)
		if err := errors.Join(transferErr, nativeRoot.Close()); err != nil {
			return fail(censusStageAssembly, err)
		}
		natives = append(natives, native)
	}
	// The controller alone schedules M/P/S while the original selected roots are
	// held. Never eagerly prepare and then replay those one-shot owners.
	var terminal bytes.Buffer
	if err := withCensusFolderRetained(ctx, o, op, natives, func(inputs product.InventoryNativeInputsV3, targets []product.InventoryTargetV3, leads []ai.Lead) error {
		grant, err := product.InventoryLocalGrantV3(deadline, uint64(len(leads)+2), applicationCoordinatorQuote(1, 16))
		if err != nil {
			return err
		}
		path, err := censusOwnedDirectory(o.PublicationRoot, "folder-controller", true)
		if err != nil {
			return err
		}
		root, err := publication.OpenRoot(path)
		if err != nil {
			return err
		}
		b := grant.Ceiling
		bounds := [14]uint64{b.LogicalBytes, b.ReadHashBytes, b.WriteBytes, b.Records, b.ParserBytes, b.ProviderRequests, b.ProviderMessages, b.Work, b.ActiveNS, b.Calls, b.RequestBytes, b.ResponseBytes, b.WireBytes, b.OutputBytes}
		err = inventorysuccessor.WithLocalFolderControllerV1(ctx, root, op, bounds, func(admitted [14]uint64, producer string) error {
			if admitted != bounds {
				return product.ErrInvalid
			}
			return runCensusFolderController(ctx, o, inputs, targets, leads, &terminal, grant, producer)
		})
		return errors.Join(err, root.Close())
	}); err != nil {
		return fail(censusStageAssembly, err)
	}
	// Cleanup is part of success: do not expose the terminal until retained roots
	// have closed, including errors joined by withCensusFolderRetained.
	if _, err := stdout.Write(terminal.Bytes()); err != nil {
		return fail(censusStageAssembly, err)
	}
	return 0
}

func acquireCensusFolderRelations(ctx context.Context, s *sessionruntime.PrivateDiscoverySession, d censusacquisition.Discovery, maxTargets int, outgoingEnabled, incomingEnabled bool) error {
	if maxTargets < 1 || len(d.Targets) > maxTargets {
		return errNativeCatalogBoundReached
	}
	type positionKey struct {
		uri      string
		position lsp.Position
	}
	definitions := make(map[positionKey]bool)
	definition := func(uri string, position lsp.Position) error {
		key := positionKey{uri, position}
		if definitions[key] {
			return nil
		}
		if len(definitions) >= maxTargets {
			return errNativeCatalogBoundReached
		}
		// The runtime independently rejects an unsupplied/out-of-scope URI. A
		// foreign occurrence is unavailable, never silently an empty relation.
		if _, err := s.Definition(ctx, uri, position); err != nil {
			return err
		}
		definitions[key] = true
		return nil
	}
	for _, target := range d.Targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		references, err := s.References(ctx, target.URI, target.Position())
		if err != nil {
			return err
		}
		if len(references) > maxTargets {
			return errNativeCatalogBoundReached
		}
		for _, location := range references {
			if location.Range == nil {
				return errors.New("reference location unavailable")
			}
			if err := definition(location.URI, location.Range.Start); err != nil {
				return err
			}
		}
		items, err := s.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: target.URI}, Position: target.Position()})
		if err != nil {
			return err
		}
		if len(items) > maxTargets {
			return errNativeCatalogBoundReached
		}
		for _, item := range items {
			var outgoing []lsp.CallHierarchyOutgoingCall
			if outgoingEnabled {
				outgoing, err = s.OutgoingCalls(ctx, item)
				if err != nil {
					return err
				}
			}
			if len(outgoing) > maxTargets {
				return errNativeCatalogBoundReached
			}
			for _, call := range outgoing {
				if len(call.FromRanges) > maxTargets {
					return errNativeCatalogBoundReached
				}
				for _, from := range call.FromRanges {
					if err := definition(item.URI, from.Start); err != nil {
						return err
					}
				}
			}
			if incomingEnabled {
				if _, err := s.IncomingCalls(ctx, item); err != nil {
					return err
				}
			}
		}
		// Definitions of unreferenced declarations are still observations; null
		// remains the provider's observed-empty result, not inferred absence.
		if len(references) == 0 {
			if err := definition(target.URI, target.Position()); err != nil {
				return err
			}
		}
	}
	return nil
}
