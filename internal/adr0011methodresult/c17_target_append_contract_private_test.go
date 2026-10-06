package adr0011methodresult

import "lsp-trace/sessionruntime"

// c17TargetAppendBorrow is the provisional test-only boundary for C17 target
// accounting. It does not select a production implementation or write set.
//
// The implementation must derive TARGET_BEGIN and TARGET_TERMINAL internally
// from the transaction-owned borrow. The restricted target owner supplies only
// the original ordinal and the first canonical append callback.
//
// One call reserves the pair before appendCanonical. A callback error or panic
// rolls the pair back; budget refusal does not invoke appendCanonical; repeated
// aliases of a committed ordinal do not recharge.
type c17TargetAppendBorrow interface {
	WithTargetAppendAdmission(originalOrdinal uint64, appendCanonical func() error) error
}

// This compile-time guard preserves the provisional callback-scoped target
// admission seam. It does not establish transaction activation or producer wiring.
var _ c17TargetAppendBorrow = sessionruntime.PrivateB4DefinitionBorrow{}
