//go:build darwin || linux

package provisionalfeaturecatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/publication"
)

// ReopenStoredNativeV3 accepts only an ID in an application-owned store. There is
// deliberately no raw DTO-to-catalog constructor. The durable controller owns
// predecessor/claim/sequence checks; this restores semantic data, not native grants.
func ReopenStoredNativeV3(ctx context.Context, root *publication.Root, ref publication.StoredObject, budget NativeBudgetV3, interpreter string, corrected bool) (NativeCatalogV3, error) {
	if !ref.Valid() || budget.Reserve(ctx, uint64(ref.Bytes)*8+16384) != nil {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	raw, e := publication.ReadObject(root, "inventory-catalog-", ref)
	var w NativeCatalogWireV3
	if e != nil || DecodeNativeV3(raw, &w) != nil || w.Version != "lsp-trace.native-catalog.v3" || w.Authority != 0 || w.Accepted || w.Status != "PROVISIONAL" || w.Completeness != "UNKNOWN" || !w.ReviewMode.Valid() || len(w.Evidence) > 100 || len(w.Groups) > 100 || interpreter == "" {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	canonical, e := json.Marshal(w)
	if e != nil || !bytes.Equal(raw, canonical) {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	revision := w.Revision
	w.Revision = ""
	identity, e := json.Marshal(w)
	w.Revision = revision
	if e != nil || NativeDigestV3(identity) != revision {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	return NativeCatalogV3{&nativeCatalogStateV3{wire: w, raw: raw, budget: budget, interpreter: interpreter, corrected: corrected}}, nil
}
