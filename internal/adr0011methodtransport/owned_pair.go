package adr0011methodtransport

import (
	"bytes"
	"encoding/json"

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
	return pair, true
}

func ownedPairMatches(req Request, result sessionruntime.RoundTripResult, pair sessionruntime.OwnedMethodPair, local LocalWriteCorrespondence) bool {
	read, accepted := result.CompletedResponseRead()
	write, completed := result.CompletedRequestWrite()
	return accepted && completed && local == LocalWriteMatch && result.Key.ID != 0 &&
		pair.SessionID == req.SessionID && pair.Generation == req.Generation && pair.Key == result.Key &&
		pair.Method == req.Method && bytes.Equal(pair.Params, req.Params) && bytes.Equal(pair.Result, result.Result) &&
		pair.Write == write && pair.Read == read && pair.Read.Key == result.Key && pair.Write.Key == result.Key &&
		pair.Read.SessionID == req.SessionID && pair.Write.SessionID == req.SessionID &&
		pair.Read.Generation == req.Generation && pair.Write.Generation == req.Generation
}
