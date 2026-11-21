//go:build darwin

package main

import (
	"context"
	"errors"
	"fmt"
	ai "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	product "lsp-trace/internal/privatehostproductv3"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

// These are the existing selected-source limits, not extra provider credit or
// a completeness boundary. Each query owns a separate, nonrenewable M/P/S job.
func censusFolderSelectedConfiguration(root string, index int) (inv.PrivateSelectedConfigurationV2, error) {
	pin := digestCatalog("normal-folder-retained-selected-v1")
	input := cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, ProgramCSeed: 9, ResponseVersion: dw.ResponseVersionV2, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "normal-folder-retained-source-v1", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, e := cc.NewContinuationContract(input)
	if e != nil {
		return inv.PrivateSelectedConfigurationV2{}, e
	}
	name := digestCatalog(fmt.Sprintf("%s/selected/%d", root, index))
	return inv.PrivateSelectedConfigurationV2{RunID: name, SeedNamespace: name + "/seed", Lineage: name + "/lineage", Seed: 7, Attempts: [3]string{digestCatalog(name + "/M"), digestCatalog(name + "/P"), digestCatalog(name + "/S")}, Contract: contract, ContractInput: input, SourceLimits: targetpacket.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}}, nil
}

// This unit exercises the existing native preparer over retained inputs. It does
// not publish a catalog or claim that selected source is semantic acceptance.
func prepareCensusFolderRetained(ctx context.Context, o censusCLIOptions, acquisition inv.PrivateDiscoveryOperationV2, natives []inv.PrivateRetainedNativeCallsV1) (results []sc.PrivateSelectedResultV2, err error) {
	err = withCensusFolderRetained(ctx, o, acquisition, natives, func(inputs product.InventoryNativeInputsV3, targets []product.InventoryTargetV3, leads []ai.Lead) error {
		prepare := inputs.Preparer(o.Workspace, censusInventorySourceRevision, targets...)
		var failures error
		for _, lead := range leads {
			result, e := prepare(ctx, lead)
			if e != nil {
				failures = errors.Join(failures, fmt.Errorf("retained selected preparation: %w", e))
				continue
			}
			if _, _, _, ok := result.Selected.Selected(); !ok {
				failures = errors.Join(failures, errors.New("retained selected source unavailable"))
				continue
			}
			results = append(results, result.Selected)
		}
		return failures
	})
	if err != nil {
		results = nil
	}
	return results, err
}

// withCensusFolderRetained holds original roots through the consuming controller.
// Constructing these opaque inputs does not execute M/P/S. The callback owns the
// sole preparation schedule and must finish using the inputs before returning.
func withCensusFolderRetained(ctx context.Context, o censusCLIOptions, acquisition inv.PrivateDiscoveryOperationV2, natives []inv.PrivateRetainedNativeCallsV1, consume func(product.InventoryNativeInputsV3, []product.InventoryTargetV3, []ai.Lead) error) (err error) {
	if consume == nil {
		return errors.New("retained input consumer unavailable")
	}
	var roots []*publication.Root
	defer func() {
		for _, root := range roots {
			err = errors.Join(err, root.Close())
		}
	}()
	inputs := product.InventoryNativeInputsV3{}
	var targets []product.InventoryTargetV3
	var leads []ai.Lead
	owned := acquisition.OwnedMethods()
	for index := 0; index < owned.GroupingQueryCount(); index++ {
		query, ok := owned.GroupingQuery(index)
		if !ok || query.ObservedEmpty || (query.Method != "textDocument/definition" && query.Method != "textDocument/references") {
			continue
		}
		var native inv.PrivateRetainedNativeCallsV1
		matching := 0
		for _, n := range natives {
			summary, ok := n.Summary()
			if ok && summary.URI == query.URI {
				native = n
				matching++
			}
		}
		// Ambiguity is unavailable, never a first-match choice or manufactured C.
		if matching != 1 {
			return errors.New("retained native query association unavailable")
		}
		path, e := censusOwnedDirectory(o.PublicationRoot, fmt.Sprintf("folder-selected-%03d", index), true)
		if e != nil {
			return e
		}
		root, e := publication.OpenRoot(path)
		if e != nil {
			return e
		}
		roots = append(roots, root)
		config, e := censusFolderSelectedConfiguration(o.PublicationRoot, index)
		if e != nil {
			return e
		}
		owner, capture, e := inv.OpenLocalFolderSelectedV1(ctx, root, acquisition, index, native, config)
		if e != nil {
			return fmt.Errorf("retained selected admission: %w", e)
		}
		lead := ai.Lead{ID: digestCatalog(fmt.Sprintf("%s/original/%d", o.PublicationRoot, index)), Provenance: "same paid acquisition; retained " + query.Method + " and both actual CALLS directions; not complete application coverage"}
		role := sc.PrivateSelectedRoleV3{Kind: "RESOLVES_TO_DEFINITION", Role: "QUERY_OCCURRENCE"}
		if query.Method == "textDocument/references" {
			role = sc.PrivateSelectedRoleV3{Kind: "REFERENCES_SYMBOL", Role: "REFERENCED_SYMBOL"}
		}
		input := product.InventoryNativeInputV3{Lead: lead, Owner: owner, Capture: capture, Configuration: config, Role: role}
		inputs[lead.ID] = input
		targets = append(targets, product.InventoryNativeTargetV3(input))
		leads = append(leads, lead)
	}
	if len(leads) == 0 {
		return errors.New("retained selected inputs unavailable; empty is not completeness")
	}
	return consume(inputs, targets, leads)
}
