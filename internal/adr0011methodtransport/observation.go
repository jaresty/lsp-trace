package adr0011methodtransport

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"lsp-trace/internal/lspwire"
)

// RawResultDisposition records local handling of runtime-supplied result bytes,
// not provider-authenticated response custody or a parsed member disposition.
type RawResultDisposition string

const (
	RawResultNotInvoked RawResultDisposition = "NOT_INVOKED"
	RawResultWithheld   RawResultDisposition = "WITHHELD"
	RawResultAbsent     RawResultDisposition = "ABSENT"
	RawResultRetained   RawResultDisposition = "RETAINED"
)

// TransactionObservation is an in-memory, unverified diagnostic projection.
// Declared fields come from a caller request; Reported fields come from the
// Runtime interface, which may be a fake or otherwise unverified producer.
// It is not an immutable receipt, provider attestation, query-target identity,
// raw-result digest, proof of a completed wire write, or admission decision.
// In particular ParamsSHA256 is only a digest of copied input bytes, not a
// canonical receipt identity or a replay artifact.
type TransactionObservation struct {
	DeclaredSessionID          string
	DeclaredGeneration         uint64
	DeclaredMethod             string
	DeclaredQueryURI           string
	DeclaredLine               uint32
	DeclaredCharacter          uint32
	DeclaredIncludeDeclaration bool
	IncludeDeclarationPresent  bool
	DeclaredDeadline           time.Time
	DeclaredMaxMessages        int
	DeclaredMaxBytes           int64
	ParamsSHA256               string
	ParamsBytes                int

	MetadataObserved         bool
	ReportedMethodAdvertised bool
	ReportedPositionEncoding string
	ReportedProviderName     string
	ReportedProviderVersion  string
	RoundTripCalled          bool
	LocalWriteCorrespondence LocalWriteCorrespondence
	ReportedKey              lspwire.RequestKey
	ReportedRequestMessages  int
	ReportedRequestBytes     int64
	ReportedResponseMessages int
	ReportedResponseBytes    int64
	RawResultDisposition     RawResultDisposition
	RawResultBytes           int
	RawResultSHA256          string
}

// Called only after request parameter validation, on the private copied bytes.
func newTransactionObservation(req Request) (TransactionObservation, error) {
	var query struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position *struct {
			Line      uint32 `json:"line"`
			Character uint32 `json:"character"`
		} `json:"position"`
		Context *struct {
			IncludeDeclaration bool `json:"includeDeclaration"`
		} `json:"context"`
	}
	// validateParams has already checked this exact copy and required members.
	if err := json.Unmarshal(req.Params, &query); err != nil {
		return TransactionObservation{}, err
	}
	sum := sha256.Sum256(req.Params)
	observation := TransactionObservation{
		DeclaredSessionID: req.SessionID, DeclaredGeneration: req.Generation,
		DeclaredMethod: req.Method, DeclaredQueryURI: query.TextDocument.URI,
		DeclaredDeadline: req.Deadline, DeclaredMaxMessages: req.MaxMessages,
		DeclaredMaxBytes: req.MaxBytes, ParamsSHA256: fmt.Sprintf("sha256:%x", sum),
		ParamsBytes: len(req.Params), RawResultDisposition: RawResultNotInvoked,
		LocalWriteCorrespondence: LocalWriteNotObserved,
	}
	if query.Position != nil {
		observation.DeclaredLine = query.Position.Line
		observation.DeclaredCharacter = query.Position.Character
	}
	if query.Context != nil {
		observation.IncludeDeclarationPresent = true
		observation.DeclaredIncludeDeclaration = query.Context.IncludeDeclaration
	}
	return observation, nil
}
