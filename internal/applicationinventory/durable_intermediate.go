//go:build darwin || linux

package applicationinventory

import (
	"context"

	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

// This private adapter is NOT selected by Start/Submit. No state.v1 or head.v2
// field is reinterpreted. Activation/persisting this selection needs a separately
// versioned state and reviewed lifecycle integration, not an old-head migration.
// Input remains the full semantic hash/length; Record is only compact storage.
type intermediateInventoryCatalog struct {
	Record publication.StoredObject
	Input  publication.StoredObject
}

func (s DurableStore) intermediateOwner(ctx context.Context, v durableInventoryState, b *cat.CallerBudgetV3) (cat.IntermediateCatalogV3, error) {
	ds, e := s.descriptions(ctx, v, b)
	if e != nil {
		return cat.IntermediateCatalogV3{}, e
	}
	native := make([]cat.NativeBoundDescriptionV3, len(ds))
	for i := range ds {
		native[i] = ds[i]
	}
	return cat.BuildIntermediateNativeV3(ctx, b, v.Interpreter, native, v.Policy.ReviewMode)
}

func applyIntermediateInventoryDecision(ctx context.Context, v durableInventoryState, c cat.IntermediateCatalogV3, raw []byte, review bool, b *cat.CallerBudgetV3) (cat.IntermediateCatalogV3, error) {
	actor := v.Interpreter
	if review {
		actor = v.Reviewer
	}
	if e := checkInventoryDecisionActor(ctx, b, raw, review, actor); e != nil {
		return cat.IntermediateCatalogV3{}, e
	}
	return c.Apply(ctx, raw, review)
}

func (s DurableStore) retainIntermediate(ctx context.Context, c cat.IntermediateCatalogV3, b *cat.CallerBudgetV3) (intermediateInventoryCatalog, error) {
	raw, e := c.Catalog().Bytes(ctx)
	if e != nil || b.Reserve(ctx, 2*uint64(len(raw))+4096) != nil {
		return intermediateInventoryCatalog{}, ErrDurableInventory
	}
	input := publication.StoredObject{ID: cat.NativeDigestV3(raw)[7:], Bytes: len(raw)}
	ref, e := c.Store(ctx, s.Root)
	if e != nil {
		return intermediateInventoryCatalog{}, e
	}
	return intermediateInventoryCatalog{Record: ref, Input: input}, nil
}

func (s DurableStore) reopenIntermediate(ctx context.Context, v durableInventoryState, selection intermediateInventoryCatalog, phase string, b *cat.CallerBudgetV3) (cat.IntermediateCatalogV3, error) {
	// Select full identity from controller state, never from candidate metadata.
	var full publication.StoredObject
	corrected := false
	switch phase {
	case "GroupDecision":
		full = v.Base
	case "Reviewer":
		full = v.Proposed
	case "Published":
		full, corrected = v.Corrected, true
	default:
		return cat.IntermediateCatalogV3{}, ErrDurableInventory
	}
	if selection.Input != full || !full.Valid() {
		return cat.IntermediateCatalogV3{}, ErrDurableInventory
	}
	owner, e := s.intermediateOwner(ctx, v, b)
	if e != nil {
		return cat.IntermediateCatalogV3{}, e
	}
	return owner.Reopen(ctx, s.Root, selection.Record, full, corrected)
}

// Full task materialization deliberately retains the existing full object and
// chunk charges. It is separate from compact revision storage; there is no
// compact-length discount on task Input, delivery, search or publication.
func (s DurableStore) materializeIntermediateInput(ctx context.Context, c cat.IntermediateCatalogV3, selection intermediateInventoryCatalog, b *cat.CallerBudgetV3) error {
	if !selection.Input.Valid() || b.Reserve(ctx, 8*uint64(selection.Input.Bytes)+16384) != nil {
		return ErrDurableInventory
	}
	raw, e := c.Catalog().Bytes(ctx)
	if e != nil || len(raw) != selection.Input.Bytes || cat.NativeDigestV3(raw) != "sha256:"+selection.Input.ID {
		return ErrDurableInventory
	}
	got, e := publication.StoreObject(s.Root, "inventory-catalog-", raw)
	if e != nil || got != selection.Input {
		return ErrDurableInventory
	}
	return nil
}
