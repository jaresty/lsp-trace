//go:build darwin || linux

package inventorysuccessor

const InventorySemanticHeadVersionV1 = "lsp-trace.durable-inventory-head.v10"
const InventorySemanticConfigVersionV1 = "lsp-trace.inventory-owned-config.v8"

func InventoryReferenceHead(schema string) bool {
	return schema == InventoryReferenceHeadVersionV1 || schema == InventorySemanticHeadVersionV1
}
func NewInventorySemanticProfileV1() InventoryResumeProfileV1 {
	p := NewInventoryReferenceProfileV1()
	p.Schema = "lsp-trace.inventory-connected-profile.v3"
	return p
}
