package sessionruntime

import "lsp-trace/internal/lspwire"

// methodCandidateObservation is only a package-private test-owned owner-path
// observation. Its bytes are not a receipt, producer attestation, or admission.
// Production callers cannot configure or receive this hook.
type methodCandidateObservation struct {
	SessionID     string
	Generation    uint64
	Key           lspwire.RequestKey
	Method        string
	RequestWrite  RequestWriteObservation
	ResponseRead  ResponseReadObservation
	RequestFrame  []byte
	ResponseFrame []byte
}

func emitMethodCandidateTestHook(hook func(methodCandidateObservation), candidate methodCandidateObservation) {
	// A test probe is never permitted to change the managed transaction outcome.
	defer func() { _ = recover() }()
	hook(candidate)
}
