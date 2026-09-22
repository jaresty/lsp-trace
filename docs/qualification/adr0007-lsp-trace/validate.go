package main

import (
	"context"
	"fmt"
	"os"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 4 {
		fail("usage: validate PUBLICATION_ROOT DESCRIPTOR_SELECTOR REVIEW_OUTPUT")
	}
	root, err := publication.OpenRoot(os.Args[1])
	if err != nil {
		fail("ASSERT_PUBLICATION_ROOT_OPEN: %v", err)
	}
	defer root.Close()
	store, err := continuationhost.NewStore(root, 16<<20)
	if err != nil {
		fail("ASSERT_CONTINUATION_STORE_OPEN: %v", err)
	}
	ctx := context.Background()
	descriptor, err := store.ResolveDescriptor(ctx, os.Args[2])
	if err != nil {
		fail("ASSERT_PUBLIC_DESCRIPTOR_RESOLVE: %v", err)
	}
	checkpointRaw, err := store.Get(ctx, descriptor.CheckpointSelector)
	if err != nil {
		fail("ASSERT_CHECKPOINT_GET: %v", err)
	}
	checkpoint, err := censuscontinuation.ParseCheckpoint(checkpointRaw)
	if err != nil {
		fail("ASSERT_CHECKPOINT_STRICT_PARSE: %v", err)
	}
	chain, chainErr := censuscontinuation.VerifyChain(ctx, store, descriptor.CheckpointSelector)
	var catalogSelector, compositeSelector string
	for _, artifact := range checkpoint.Artifacts() {
		switch artifact.Kind {
		case "catalog":
			catalogSelector = artifact.ID
		case "composite":
			compositeSelector = artifact.ID
		}
	}
	if catalogSelector == "" || compositeSelector == "" {
		fail("ASSERT_FINAL_TYPED_ARTIFACTS catalog=%q composite=%q", catalogSelector, compositeSelector)
	}
	catalogRaw, err := store.Get(ctx, catalogSelector)
	if err != nil {
		fail("ASSERT_CATALOG_GET: %v", err)
	}
	catalog, err := provisionalfeaturecatalog.Parse(catalogRaw)
	if err != nil {
		fail("ASSERT_CATALOG_STRICT_PARSE: %v", err)
	}
	compositeRaw, err := store.Get(ctx, compositeSelector)
	if err != nil {
		fail("ASSERT_COMPOSITE_GET: %v", err)
	}
	composite, err := censuscontinuation.ParseComposite(compositeRaw)
	if err != nil {
		fail("ASSERT_COMPOSITE_STRICT_PARSE: %v", err)
	}
	review, err := provisionalfeaturecatalog.RenderReview(catalog)
	if err != nil {
		fail("ASSERT_RENDER_REVIEW: %v", err)
	}
	if err := os.WriteFile(os.Args[3], []byte(review), 0o600); err != nil {
		fail("ASSERT_REVIEW_WRITE: %v", err)
	}
	accounting := catalog.Accounting()
	fmt.Printf("DESCRIPTOR catalog=%s checkpoint=%s composite=%s\n", descriptor.CatalogSelector, descriptor.CheckpointSelector, descriptor.CompositeSelector)
	fmt.Printf("TYPED catalog=%s composite=%s checkpoint_id=%s stage=%s status=%s chain=%d\n", catalogSelector, compositeSelector, checkpoint.ID(), checkpoint.Stage(), checkpoint.Status(), len(chain))
	fmt.Printf("CATALOG id=%s outcome=%s total=%d complete=%d abstained=%d invalid_input=%d model_unavailable=%d context_limit=%d output_invalid=%d timeout=%d cancelled=%d resource_limit=%d backend_failure=%d policy_mismatch=%d duplicate_input=%d\n", catalog.ID(), catalog.Outcome(), accounting.Total, accounting.Complete, accounting.Abstained, accounting.InvalidInput, accounting.ModelUnavailable, accounting.ContextLimit, accounting.OutputInvalid, accounting.Timeout, accounting.Cancelled, accounting.ResourceLimit, accounting.BackendFailure, accounting.PolicyMismatch, accounting.DuplicateInput)
	fmt.Printf("COMPOSITE id=%s catalog_id=%s checkpoint=%s authority=%d accepted=%t completeness=%s\n", composite.ID(), composite.CatalogID(), composite.CheckpointSelector(), composite.Authority(), composite.Accepted(), composite.Completeness())
	if chainErr != nil {
		fail("ASSERT_CHECKPOINT_CHAIN_VERIFY: %v", chainErr)
	}
	if len(chain) == 0 {
		fail("ASSERT_CHECKPOINT_CHAIN_NONEMPTY")
	}
	if descriptor.CatalogSelector != catalogSelector {
		fail("ASSERT_PUBLIC_DESCRIPTOR_CATALOG_SELECTOR: descriptor=%s typed_catalog=%s", descriptor.CatalogSelector, catalogSelector)
	}
	if descriptor.CompositeSelector != compositeSelector {
		fail("ASSERT_PUBLIC_DESCRIPTOR_COMPOSITE_SELECTOR: descriptor=%s typed_composite=%s", descriptor.CompositeSelector, compositeSelector)
	}
	fmt.Println("QUALIFICATION_STRICT_VALIDATION_COMPLETE")
}
