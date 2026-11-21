package sourcecontinuation

import inv "lsp-trace/internal/inventorysuccessor"

// InventoryAccountingMatchesV1 compares data with this genuine result's held
// completed operation. It constructs neither authority nor retained evidence.
func (r PrivateSelectedResultV2) InventoryAccountingMatchesV1(a inv.CompletedSelectedAccountingV1) bool {
	return r.state != nil && a.Windows == 3 && a.Head != "" && r.state.accounting == a
}
