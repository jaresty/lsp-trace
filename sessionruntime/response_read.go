package sessionruntime

import "lsp-trace/internal/lspwire"

// ResponseReadObservation is manager-local evidence about an accepted decoded
// response and its exact consumed frame. It does not authenticate the provider,
// establish immutable custody, or admit method occurrences.
type ResponseReadObservation struct {
	SessionID   string
	Generation  uint64
	Key         lspwire.RequestKey
	FrameBytes  int64
	FrameSHA256 string
}

// CompletedResponseRead returns a value copy only after the manager has
// accepted the matching response and passed its selected response bound.
func (r RoundTripResult) CompletedResponseRead() (ResponseReadObservation, bool) {
	if r.responseRead == nil {
		return ResponseReadObservation{}, false
	}
	return *r.responseRead, true
}
