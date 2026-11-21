//go:build darwin || linux

package inventorysuccessor

import cat "lsp-trace/internal/provisionalfeaturecatalog"

const InventoryConnectedHeadVersionV1 = "lsp-trace.durable-inventory-head.v8"
const InventoryConnectedConfigVersionV1 = "lsp-trace.inventory-owned-config.v6"

// InventoryResumableHeadV1 selects only the two explicitly supported initial
// profiles. It is version dispatch, not an upgrade of persisted v7 jobs.
func InventoryResumableHeadV1(schema string) bool {
	return schema == InventoryResumeHeadVersionV1 || InventoryConnectedHead(schema)
}
func NewInventoryConnectedProfileV1() InventoryResumeProfileV1 {
	p := NewInventoryResumeProfileV1()
	p.Schema = "lsp-trace.inventory-connected-profile.v1"
	p.ParentLimit = cat.InventoryConnectedLimitV1
	return p
}
