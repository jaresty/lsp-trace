package adr0011methodtransport

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"lsp-trace/sessionruntime"
)

// OwnedPair exposes only a copy of one explicitly selected manager-local D/R
// observation. It cannot issue a receipt, terminal ledger, or occurrence.
func (r Result) OwnedPair() (sessionruntime.OwnedMethodPair, bool) {
	if r.status != OutcomeTransportSuccess || r.ownedPair == nil {
		return sessionruntime.OwnedMethodPair{}, false
	}
	pair := *r.ownedPair
	pair.Params = append(json.RawMessage(nil), pair.Params...)
	pair.Result = append(json.RawMessage(nil), pair.Result...)
	if pair.Source != nil {
		source := *pair.Source
		pair.Source = &source
	}
	return pair, true
}

func validateOwnedSourceExpectation(req Request) error {
	expected := req.ExpectedOwnedDocument
	if expected == nil {
		return nil
	}
	if !req.CaptureOwnedMethodPair || expected.URI == "" || expected.Version <= 0 ||
		!strings.HasPrefix(expected.SHA256, "sha256:") || len(expected.SHA256) != 71 {
		return ErrOwnedSourceExpectation
	}
	digest, err := hex.DecodeString(expected.SHA256[7:])
	if err != nil || len(digest) != 32 || expected.SHA256 != "sha256:"+hex.EncodeToString(digest) {
		return ErrOwnedSourceExpectation
	}
	var query struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(req.Params, &query) != nil || query.TextDocument.URI != expected.URI {
		return ErrOwnedSourceExpectation
	}
	return nil
}

var ErrOwnedSourceExpectation = errors.New("owned document expectation invalid")

func ownedPairMatches(req Request, result sessionruntime.RoundTripResult, pair sessionruntime.OwnedMethodPair, local LocalWriteCorrespondence) bool {
	if req.ExpectedOwnedDocument != nil {
		if pair.Source == nil || *pair.Source != *req.ExpectedOwnedDocument {
			return false
		}
	} else if pair.Source != nil {
		return false
	}
	read, accepted := result.CompletedResponseRead()
	write, completed := result.CompletedRequestWrite()
	return accepted && completed && local == LocalWriteMatch && result.Key.ID != 0 &&
		pair.SessionID == req.SessionID && pair.Generation == req.Generation && pair.Key == result.Key &&
		pair.Method == req.Method && bytes.Equal(pair.Params, req.Params) && bytes.Equal(pair.Result, result.Result) &&
		pair.Write == write && pair.Read == read && pair.Read.Key == result.Key && pair.Write.Key == result.Key &&
		pair.Read.SessionID == req.SessionID && pair.Write.SessionID == req.SessionID &&
		pair.Read.Generation == req.Generation && pair.Write.Generation == req.Generation
}
