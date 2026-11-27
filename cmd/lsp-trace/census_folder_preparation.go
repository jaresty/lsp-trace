//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"slices"
	"time"

	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
)

// A process-local verified traversal result, not execution funding or native
// evidence. Only the normal runner transfers it to the separate acquisition lane.
type censusFolderPreparation struct {
	workspace, runRoot string
	records            []source.Record
}

func prepareCensusFolder(o censusCLIOptions, deadline time.Time) (*censusFolderPreparation, error) {
	sorted := func(xs []string) []string { ys := slices.Clone(xs); slices.Sort(ys); return slices.Compact(ys) }
	root, err := publication.OpenRoot(o.PublicationRoot)
	if err != nil {
		return nil, err
	}
	records, err := inventorysuccessor.PrepareRecursiveFolder(context.Background(), root, inventorysuccessor.PrivatePreparationIntentV1{
		Schema: "private-preparation-intent.v1", SourceRoot: o.Workspace,
		Attempt: rawDigest([]byte(o.PublicationRoot)), Deadline: deadline,
		Roots: sorted(o.Sources), Includes: sorted(o.Includes), Excludes: sorted(o.Excludes), PrunedDirectories: sorted(o.PrunedDirectories),
		Limits: source.Limits{MaxEntries: censusMaxDiscoveredEntries, MaxAccepted: censusMaxDiscoveredEntries, MaxWork: censusMaxEnumerationWork, MaxPathBytes: censusMaxSourcePathBytes, MaxDepth: censusMaxSourceDepth},
	})
	if err := errors.Join(err, root.Close()); err != nil {
		return nil, err
	}
	return &censusFolderPreparation{workspace: o.Workspace, runRoot: o.PublicationRoot, records: records}, nil
}
