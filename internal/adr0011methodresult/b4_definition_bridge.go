package adr0011methodresult

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/strictjson"
)

type DefinitionBridgeStatus string

const (
	DefinitionBridgeUnimplemented         DefinitionBridgeStatus = "BRIDGE_UNIMPLEMENTED" // retained scaffold vocabulary; never issued
	DefinitionBridgeCorrespondenceInvalid DefinitionBridgeStatus = "CORRESPONDENCE_INVALID"
	DefinitionBridgeResponseInvalid       DefinitionBridgeStatus = "RESPONSE_INVALID"
	DefinitionBridgeMalformedResult       DefinitionBridgeStatus = "MALFORMED_RESULT"
	DefinitionBridgeChronologyBlocked     DefinitionBridgeStatus = "CHRONOLOGY_BLOCKED"
	DefinitionBridgeCandidateEmpty        DefinitionBridgeStatus = "CANDIDATE_EMPTY"
	DefinitionBridgeCandidateItems        DefinitionBridgeStatus = "CANDIDATE_ITEMS"
)

type DefinitionBridgeInput struct {
	Replay            v5.B4bFullCandidateInput
	ResponseFrame     []byte
	QueryOccurrenceID string
	TargetSources     map[string][]byte
}

type DefinitionBridgeResult struct {
	Status             DefinitionBridgeStatus
	ChronologyTerminal string
	Candidates         []DefinitionCandidate
}

// CheckB4DefinitionBridge is a private synthetic replay, not a producer receipt.
// Neither the response nor internally constructed ledger authenticates an LSP server.
func CheckB4DefinitionBridge(in DefinitionBridgeInput) DefinitionBridgeResult {
	out := DefinitionBridgeResult{Status: DefinitionBridgeCorrespondenceInvalid}
	w := in.Replay.Write
	if v5.CheckB4a(w) != nil || w.Method != "textDocument/definition" ||
		len(in.QueryOccurrenceID) == 0 || len(in.QueryOccurrenceID) > 1024 ||
		w.Line > math.MaxUint32 || w.Character > math.MaxUint32 {
		return out
	}
	out.ChronologyTerminal = v5.CheckB4bDefinitionSuccessor(in.Replay)
	out.Status = DefinitionBridgeResponseInvalid
	result, ok := bridgeResponseResult(in)
	if !ok {
		return out
	}
	// Parse the entire result before allowing any prefix of a malformed array
	// to enter a candidate or a ledger. Chronology is retained independently.
	out.Status = DefinitionBridgeMalformedResult
	parsed, failure := parseRawUntrusted(w.Method, result, 1000)
	if failure != nil {
		return out
	}
	if len(parsed.Items) != 0 && out.ChronologyTerminal != "SUPPORTED" {
		out.Status = DefinitionBridgeChronologyBlocked
		return out
	}
	for _, item := range parsed.Items {
		source, exists := in.TargetSources[item.URI]
		if !exists || !bridgeRangeWithinSource(source, item.Range) ||
			(item.TargetRange != nil && !bridgeRangeWithinSource(source, *item.TargetRange)) {
			return out
		}
	}
	out.Status = DefinitionBridgeCorrespondenceInvalid
	id, err := strconv.ParseUint(string(w.RequestID), 10, 64)
	if err != nil || id == 0 || id > math.MaxInt64 || len(result) == 0 || len(result) > 1<<20 {
		return out
	}
	candidate := CanonicalCandidate{
		Version: CanonicalCandidateVersion, ClaimCeiling: "UNADMITTED;NO_PRODUCER_AUTHENTICATION",
		SessionID: w.Session, Generation: w.Generation, KeyID: id,
		Method: w.Method, QueryURI: w.URI, QueryLine: uint32(w.Line), QueryCharacter: uint32(w.Character),
		PositionEncoding: w.Encoding, ProviderName: "private-synthetic-b4-definition-replay",
		MaxMessages: 1, MaxBytes: int64(len(result)), MaxCandidates: 1000, DeadlineUnixNano: 1,
		ParamsBase64: base64.StdEncoding.EncodeToString(w.RequestParams), ParamsSHA256: rawSHA(w.RequestParams),
		ResultBase64: base64.StdEncoding.EncodeToString(result), ResultSHA256: rawSHA(result),
		ItemCount: len(parsed.Items), Null: parsed.Null,
	}
	pre, err := json.Marshal(candidate)
	if err != nil {
		return out
	}
	candidate.ID = queryResultCandidateDigest(pre)
	raw, err := json.Marshal(candidate)
	if err != nil {
		return out
	}
	raw = append(raw, '\n')
	form := EnvelopeArray
	if parsed.Null {
		form = EnvelopeNull
	} else if bytes.TrimSpace(result)[0] == '{' {
		form = EnvelopeDefinitionScalar
	}
	member := CandidateQueryMember{OccurrenceID: in.QueryOccurrenceID, Began: true,
		Envelope: EnvelopeObservation{Known: true, Form: form, Count: len(parsed.Items)}, Terminal: CandidateEmpty}
	if len(parsed.Items) != 0 {
		member.Terminal = CandidateItems
		for ordinal := range parsed.Items {
			member.Elements = append(member.Elements, CandidateElement{Ordinal: ordinal, Began: true, Terminal: CandidateValidElement})
		}
	}
	ledger := CandidateLedger{
		Queries: []CandidateQuery{{OccurrenceID: in.QueryOccurrenceID, SessionID: w.Session, Generation: w.Generation,
			Method: w.Method, ParamsSHA256: candidate.ParamsSHA256}},
		Members: []CandidateQueryMember{member},
	}
	items, err := ProjectDefinitionCandidates(raw, ledger, in.QueryOccurrenceID)
	if err != nil {
		return out
	}
	out.Candidates = items
	out.Status = DefinitionBridgeCandidateEmpty
	if len(items) != 0 {
		out.Status = DefinitionBridgeCandidateItems
	}
	return out
}

// The response has no transaction field: its only held binding is the exact
// numeric ID of the B4a-verified completed WRITE, which itself binds transaction
// and frame ordinal. This does not establish same-transaction producer custody.
func bridgeResponseResult(in DefinitionBridgeInput) ([]byte, bool) {
	w := in.Replay.Write
	if len(in.ResponseFrame) == 0 || len(in.ResponseFrame) > 1<<20 ||
		strictjson.RejectDuplicates(in.ResponseFrame) != nil || !json.Valid(in.ResponseFrame) ||
		len(w.RequestID) == 0 || w.RequestID[0] < '0' || w.RequestID[0] > '9' {
		return nil, false
	}
	var response map[string]json.RawMessage
	if json.Unmarshal(in.ResponseFrame, &response) != nil || len(response) != 3 ||
		string(response["jsonrpc"]) != `"2.0"` || len(response["result"]) == 0 ||
		len(response["id"]) == 0 || response["id"][0] < '0' || response["id"][0] > '9' {
		return nil, false
	}
	requestID, requestError := strconv.ParseUint(string(w.RequestID), 10, 64)
	responseID, responseError := strconv.ParseUint(string(response["id"]), 10, 64)
	if requestError != nil || responseError != nil || requestID == 0 || requestID != responseID ||
		!bytes.Equal(w.RequestID, response["id"]) {
		return nil, false
	}
	return response["result"], true
}

func bridgeRangeWithinSource(source []byte, r Range) bool {
	if !utf8.Valid(source) || r.Start.Line > r.End.Line || r.Start.Line == r.End.Line && r.Start.Character > r.End.Character {
		return false
	}
	lines := strings.Split(string(source), "\n")
	for _, p := range []Position{r.Start, r.End} {
		if uint64(p.Line) >= uint64(len(lines)) {
			return false
		}
		line := strings.TrimSuffix(lines[p.Line], "\r")
		if uint64(p.Character) > uint64(len(utf16.Encode([]rune(line)))) {
			return false
		}
	}
	return true
}
