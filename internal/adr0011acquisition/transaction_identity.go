package adr0011acquisition

import (
	"strconv"

	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/lspwire"
)

const maxCanonicalInteger = 9007199254740991

// referencesTransactionID is a private computation, never an issuance receipt.
// The caller must independently verify the original raw JSON-RPC ID tokens,
// invocation binding, source and revision publications, and query admission.
func referencesTransactionID(sessionID string, generation uint64, key lspwire.RequestKey, invocation, queryOccurrenceID string, sourceRef, revisionRef targetResultRef) (string, error) {
	if sessionID == "" || invocation == "" || !validTargetIdentityString(sessionID) || !validTargetIdentityString(invocation) ||
		!targetResultValidDigest(queryOccurrenceID) || !validTargetIdentityRef(sourceRef, sourceIdentityRole, "source") ||
		!validTargetIdentityRef(revisionRef, revisionIdentityRole, "revision") ||
		generation == 0 || generation > maxCanonicalInteger || key.Generation != generation || key.ID == 0 || key.ID > maxCanonicalInteger {
		return "", errTargetRecord
	}
	requestKey := adr0011requestkey.Encode(key)
	return targetIdentityHash([]byte("REFERENCES_TRANSACTION_V1"), []byte(sessionID), []byte(strconv.FormatUint(generation, 10)), []byte(requestKey), []byte(invocation), []byte(queryOccurrenceID), targetIdentityRefJSON(sourceRef), targetIdentityRefJSON(revisionRef)), nil
}
