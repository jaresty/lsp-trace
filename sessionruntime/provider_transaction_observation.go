package sessionruntime

// ProviderTransactionLifecycle is the closed private lifecycle vocabulary used
// by the scalar provider transaction observation.
type ProviderTransactionLifecycle string

const (
	ProviderTransactionLifecycleReady    ProviderTransactionLifecycle = "READY"
	ProviderTransactionLifecyclePoisoned ProviderTransactionLifecycle = "POISONED"
)

// ProviderTransactionObservation contains only privacy-reduced scalar facts
// owned by one completed document-symbol transaction.
type ProviderTransactionObservation struct {
	SelectedResponseObserved       bool
	RequestWriteCompleted          bool
	ProviderMessagesProcessed      uint64
	ProviderResponseWriteCompleted bool
	FailurePostEffect              bool
	TerminalPoisonObserved         bool
	TerminalLifecycle              ProviderTransactionLifecycle
}

type privateProviderTransactionCapture struct {
	enabled                  bool
	selectedResponseObserved bool
	observation              ProviderTransactionObservation
	present                  bool
}

// ProviderTransactionObservation returns a value copy only when private
// capture was explicitly enabled for a completed document-symbol transaction.
func (r RoundTripResult) ProviderTransactionObservation() (ProviderTransactionObservation, bool) {
	return r.providerTransaction.observation, r.providerTransaction.present
}
