package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/internal/strictjson"
)

// OwnedDocumentBinding identifies one supplied, generation-local document.
// It does not establish which version a server analyzed.
type OwnedDocumentBinding struct {
	URI     string
	Version int
	SHA256  string
}

func matchesOwnedDocument(req RoundTripRequest, documents map[string]openDocument) bool {
	expected := req.ExpectedOwnedDocument
	if expected == nil || !eligibleOwnedMethodRequest(req) || expected.URI == "" || expected.Version <= 0 || expected.SHA256 == "" {
		return false
	}
	uri, valid := ownedQueryURI(req)
	if !valid || uri != expected.URI {
		return false
	}
	current, present := documents[expected.URI]
	if !present || current.version != expected.Version || current.supply == nil {
		return false
	}
	supply := current.supply
	if supply.Classification != "LSP_SUPPLIED" || supply.SessionID != req.SessionID || supply.Generation != req.Generation ||
		supply.URI != expected.URI || supply.DocumentVersion != expected.Version ||
		(supply.Method != "textDocument/didOpen" && supply.Method != "textDocument/didChange") ||
		sha256.Sum256(supply.Content) != current.digest {
		return false
	}
	return expected.SHA256 == fmt.Sprintf("sha256:%x", current.digest)
}

// OwnedMethodPair is an in-memory, manager-local observation of one keyed
// definition/references/documentSymbol transaction. It is not a published method receipt,
// provider authentication, terminal ledger, or admitted occurrence.
type OwnedMethodPair struct {
	SessionID  string
	Generation uint64
	Key        lspwire.RequestKey
	Method     string
	Params     json.RawMessage
	Result     json.RawMessage
	Write      RequestWriteObservation
	Read       ResponseReadObservation
	Source     *OwnedDocumentBinding
}

// emitOwnedMethodPairTestHook isolates a package-only observer from the retained pair
// and from transport success. The hook is never configured by production hosts.
func emitOwnedMethodPairTestHook(hook func(OwnedMethodPair), pair OwnedMethodPair) {
	defer func() { _ = recover() }()
	pair.Params = append(json.RawMessage(nil), pair.Params...)
	pair.Result = append(json.RawMessage(nil), pair.Result...)
	if pair.Source != nil {
		source := *pair.Source
		pair.Source = &source
	}
	hook(pair)
}

// ownedQueryURI rejects aliases and duplicate keys before comparing a
// predeclared source to the exact manager-owned method query.
func ownedQueryURI(req RoundTripRequest) (string, bool) {
	if strictjson.RejectDuplicates(req.Params) != nil {
		return "", false
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(req.Params, &root) != nil {
		return "", false
	}
	if req.Method == "textDocument/documentSymbol" {
		if len(root) != 1 {
			return "", false
		}
	} else if len(root) != 2 && len(root) != 3 {
		return "", false
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(root["textDocument"], &doc) != nil || len(doc) != 1 {
		return "", false
	}
	var uri string
	if json.Unmarshal(doc["uri"], &uri) != nil || uri == "" {
		return "", false
	}
	if req.Method == "textDocument/documentSymbol" {
		return uri, true
	}
	var position map[string]json.RawMessage
	if json.Unmarshal(root["position"], &position) != nil || len(position) != 2 {
		return "", false
	}
	var line, character uint32
	if json.Unmarshal(position["line"], &line) != nil || json.Unmarshal(position["character"], &character) != nil {
		return "", false
	}
	if req.Method == "textDocument/references" {
		var context map[string]json.RawMessage
		if json.Unmarshal(root["context"], &context) != nil || len(root) != 3 || len(context) != 1 {
			return "", false
		}
		var include bool
		if json.Unmarshal(context["includeDeclaration"], &include) != nil {
			return "", false
		}
	} else if len(root) != 2 {
		return "", false
	}
	return uri, true
}

// CompletedOwnedMethodPair returns a copy only for an opted-in, bounded,
// successful managed transaction. A zero value cannot issue method evidence.
func (r RoundTripResult) CompletedOwnedMethodPair() (OwnedMethodPair, bool) {
	if r.Failure != "" || r.ServerError != nil || r.ownedMethodPair == nil {
		return OwnedMethodPair{}, false
	}
	p := *r.ownedMethodPair
	p.Params = append(json.RawMessage(nil), p.Params...)
	p.Result = append(json.RawMessage(nil), p.Result...)
	if p.Source != nil {
		source := *p.Source
		p.Source = &source
	}
	return p, true
}

func eligibleADR0011PrivateLimitRequest(req RoundTripRequest) bool {
	return req.ADR0011PrivateLimitAllocationV1 && req.CaptureOwnedMethodPair && req.ExpectedOwnedDocument != nil &&
		(req.Method == "textDocument/documentSymbol" || req.Method == "textDocument/references") &&
		req.SessionID != "" && req.Generation != 0 && req.MaxMessages >= 1 && req.MaxMessages <= 4 &&
		req.MaxBytes >= 1 && req.MaxBytes <= 4194304 && len(req.Params) >= 1 && len(req.Params) <= 65536
}

func eligibleOwnedMethodRequest(req RoundTripRequest) bool {
	// Keep the historical predicate a distinct branch, including marked
	// definition requests and previously opted-in unmarked callers.
	historical := req.CaptureOwnedMethodPair && (req.Method == "textDocument/definition" || req.Method == "textDocument/references" || req.Method == "textDocument/documentSymbol") &&
		req.SessionID != "" && req.Generation != 0 && req.MaxMessages >= 1 && req.MaxMessages <= 64 &&
		req.MaxBytes >= 1 && req.MaxBytes <= 1<<20 && len(req.Params) >= 1 && len(req.Params) <= 64<<10
	return (historical && (!req.ADR0011PrivateLimitAllocationV1 || req.Method == "textDocument/definition")) || eligibleADR0011PrivateLimitRequest(req)
}

func validOwnedMethodPair(req RoundTripRequest, r RoundTripResult, raw json.RawMessage) bool {
	return eligibleOwnedMethodRequest(req) && r.ServerError == nil && r.Key.Generation == req.Generation && r.Key.ID != 0 &&
		r.RequestMessages == 1 && r.RequestBytes >= 1 && r.RequestBytes <= req.MaxBytes &&
		r.Messages >= 1 && r.Messages <= req.MaxMessages && r.Bytes >= 1 && r.Bytes <= req.MaxBytes &&
		len(raw) >= 1 && int64(len(raw)) <= req.MaxBytes && bytes.Equal(raw, r.Result) &&
		r.requestWrite != nil && r.responseRead != nil && r.requestWrite.FrameBytes >= 1 && r.responseRead.FrameBytes >= 1 &&
		r.requestWrite.SessionID == req.SessionID && r.responseRead.SessionID == req.SessionID &&
		r.requestWrite.Generation == req.Generation && r.responseRead.Generation == req.Generation &&
		r.requestWrite.Key == r.Key && r.responseRead.Key == r.Key && r.requestWrite.Method == req.Method
}

func ownedMethodPairValue(req RoundTripRequest, r RoundTripResult, params, raw json.RawMessage) OwnedMethodPair {
	pair := OwnedMethodPair{SessionID: req.SessionID, Generation: req.Generation, Key: r.Key, Method: req.Method,
		Params: params, Result: raw, Write: *r.requestWrite, Read: *r.responseRead}
	if req.ExpectedOwnedDocument != nil {
		source := *req.ExpectedOwnedDocument
		pair.Source = &source
	}
	return pair
}

func buildPrivateOwnedMethodPairOwner(account *privateB4ByteAccountV2, req RoundTripRequest, r RoundTripResult, params, raw json.RawMessage) (*privateB4OwnedMethodPairOwner, session.Failure) {
	if !validOwnedMethodPair(req, r, raw) {
		return nil, session.ResourceExhausted
	}
	return newPrivateB4OwnedMethodPairOwner(account, ownedMethodPairValue(req, r, params, raw))
}

func buildOwnedMethodPair(req RoundTripRequest, r RoundTripResult, params, raw json.RawMessage) *OwnedMethodPair {
	if !validOwnedMethodPair(req, r, raw) {
		return nil
	}
	value := ownedMethodPairValue(req, r, params, raw)
	value.Params = append(json.RawMessage(nil), params...)
	value.Result = append(json.RawMessage(nil), raw...)
	return &value
}

// The declarations below are a package-private vocabulary for a future causal
// Call Hierarchy observation path. This increment does not construct any value.
type callHierarchyWorkspaceCustody struct {
	managerNonce [32]byte
}
type callHierarchySessionIdentity string
type callHierarchyGeneration uint64
type callHierarchyProfileIdentity string
type callHierarchyBindingIdentity string
type callHierarchyPredecessorIdentity string
type callHierarchyMethodIdentity string
type callHierarchySelectedOrdinal uint64

type callHierarchyJSONRPCRequestKeyKind uint8

const (
	callHierarchyJSONRPCRequestKeyUnavailable callHierarchyJSONRPCRequestKeyKind = iota
	callHierarchyJSONRPCRequestKeyString
	callHierarchyJSONRPCRequestKeyNumber
)

type callHierarchyJSONRPCRequestKey struct {
	kind        callHierarchyJSONRPCRequestKeyKind
	stringValue string
	numberBytes []byte
}

type callHierarchyRequestIdentity struct {
	workspace  callHierarchyWorkspaceCustody
	session    callHierarchySessionIdentity
	generation callHierarchyGeneration
	requestKey callHierarchyJSONRPCRequestKey
}

type callHierarchyTransactionIdentity struct {
	request callHierarchyRequestIdentity
	local   [32]byte
}

type callHierarchyPredecessorCommitment struct {
	identity         callHierarchyPredecessorIdentity
	transaction      callHierarchyTransactionIdentity
	requestIdentity  callHierarchyRequestIdentity
	requestKey       callHierarchyJSONRPCRequestKey
	method           callHierarchyMethodIdentity
	requestOrdinal   uint64
	commitmentSHA256 [32]byte
}

type callHierarchyPredecessorChain struct {
	commitments []callHierarchyPredecessorCommitment
}

type callHierarchySelectedItemCommitment struct {
	ordinal       callHierarchySelectedOrdinal
	sourceRequest callHierarchyRequestIdentity
	sourceFrame   callHierarchyTransactionIdentity
	sourceOffset  uint64
	sourceLength  uint64
	exactBytes    []byte
	exactSHA256   [32]byte
}

type callHierarchyHandoffCommitment struct {
	selected         callHierarchySelectedItemCommitment
	successorRequest callHierarchyRequestIdentity
	successor        callHierarchyTransactionIdentity
	successorOffset  uint64
	successorLength  uint64
	successorBytes   []byte
	successorSHA256  [32]byte
}

type callHierarchyJSONTokenForm uint8

const (
	callHierarchyJSONTokenFormUnavailable callHierarchyJSONTokenForm = iota
	callHierarchyJSONTokenFormNull
	callHierarchyJSONTokenFormEmptyArray
	callHierarchyJSONTokenFormOther
)

type callHierarchyOriginalRequestFrameObservation struct {
	requestIdentity callHierarchyRequestIdentity
	frameBytes      []byte
	frameLength     uint64
	frameSHA256     [32]byte
	bodyOffset      uint64
	bodyLength      uint64
	bodySHA256      [32]byte
	paramsOffset    uint64
	paramsLength    uint64
	paramsSHA256    [32]byte
	paramsForm      callHierarchyJSONTokenForm
}

type callHierarchyOriginalResponseFrameObservation struct {
	requestIdentity callHierarchyRequestIdentity
	frameBytes      []byte
	frameLength     uint64
	frameSHA256     [32]byte
	bodyOffset      uint64
	bodyLength      uint64
	bodySHA256      [32]byte
	resultOffset    uint64
	resultLength    uint64
	resultSHA256    [32]byte
	resultForm      callHierarchyJSONTokenForm
}

type callHierarchyMethodAllowlist struct {
	methods []callHierarchyMethodIdentity
}

type callHierarchyObservationTerminal uint8

const (
	callHierarchyObservationTerminalUnavailable callHierarchyObservationTerminal = iota
	callHierarchyObservationTerminalComplete
	callHierarchyObservationTerminalSuccessfulEmpty
	callHierarchyObservationTerminalTransportError
	callHierarchyObservationTerminalProtocolError
	callHierarchyObservationTerminalMissingPayload
	callHierarchyObservationTerminalMalformed
	callHierarchyObservationTerminalPartialCapture
	callHierarchyObservationTerminalLimitRefused
	callHierarchyObservationTerminalCancelled
)

// valid remains deliberately false until terminal validation is reviewed without
// changing the regenerated C15 catalog.
func (callHierarchyObservationTerminal) valid() bool { return false }

type callHierarchyMeasure struct {
	available bool
	value     uint64
}

type callHierarchyObservationLimits struct {
	maxBytes       uint64
	maxObjects     uint64
	maxWork        uint64
	maxDiagnostics uint64
}

type callHierarchyObservationAccounting struct {
	bytes       callHierarchyMeasure
	objects     callHierarchyMeasure
	work        callHierarchyMeasure
	diagnostics callHierarchyMeasure
}

// validAgainst is an inert future validation seam. A zero scaffold value must
// not masquerade as validated accounting.
func (callHierarchyObservationAccounting) validAgainst(callHierarchyObservationLimits) bool {
	return false
}

type callHierarchyObservationIdentity struct {
	request     callHierarchyRequestIdentity
	profile     callHierarchyProfileIdentity
	binding     callHierarchyBindingIdentity
	transaction callHierarchyTransactionIdentity
	method      callHierarchyMethodIdentity
}

type callHierarchyObservationRequest struct {
	identity     callHierarchyObservationIdentity
	allowlist    callHierarchyMethodAllowlist
	predecessors callHierarchyPredecessorChain
	selected     *callHierarchySelectedItemCommitment
	handoff      *callHierarchyHandoffCommitment
	limits       callHierarchyObservationLimits
}

type callHierarchyObservation struct {
	identity     callHierarchyObservationIdentity
	predecessors callHierarchyPredecessorChain
	request      *callHierarchyOriginalRequestFrameObservation
	response     *callHierarchyOriginalResponseFrameObservation
	selected     *callHierarchySelectedItemCommitment
	handoff      *callHierarchyHandoffCommitment
	terminal     callHierarchyObservationTerminal
	accounting   callHierarchyObservationAccounting
}

type callHierarchyObservationView struct {
	identity     callHierarchyObservationIdentity
	predecessors []callHierarchyPredecessorCommitment
	request      *callHierarchyOriginalRequestFrameObservation
	response     *callHierarchyOriginalResponseFrameObservation
	selected     *callHierarchySelectedItemCommitment
	handoff      *callHierarchyHandoffCommitment
	terminal     callHierarchyObservationTerminal
	accounting   callHierarchyObservationAccounting
}

func (callHierarchyPredecessorChain) copyCommitments() []callHierarchyPredecessorCommitment {
	return nil
}

func newCallHierarchySelectedItemCommitment(callHierarchySelectedOrdinal, callHierarchyRequestIdentity, callHierarchyTransactionIdentity, uint64, uint64, []byte, callHierarchyObservationLimits) (callHierarchySelectedItemCommitment, bool) {
	var zero callHierarchySelectedItemCommitment
	return zero, false
}

func (callHierarchySelectedItemCommitment) copyBytes() []byte { return nil }

func newCallHierarchyHandoffCommitment(callHierarchySelectedItemCommitment, callHierarchyRequestIdentity, callHierarchyTransactionIdentity, uint64, uint64, []byte, callHierarchyObservationLimits) (callHierarchyHandoffCommitment, bool) {
	var zero callHierarchyHandoffCommitment
	return zero, false
}

func (callHierarchyHandoffCommitment) copySuccessorBytes() []byte { return nil }

// Payload form is eventually derived from the exact payload span; callers do
// not supply an authoritative enum value.
func newCallHierarchyOriginalRequestFrameObservation(callHierarchyRequestIdentity, []byte, uint64, uint64, uint64, uint64, callHierarchyObservationLimits) (callHierarchyOriginalRequestFrameObservation, bool) {
	var zero callHierarchyOriginalRequestFrameObservation
	return zero, false
}

func newCallHierarchyOriginalResponseFrameObservation(callHierarchyRequestIdentity, []byte, uint64, uint64, uint64, uint64, callHierarchyObservationLimits) (callHierarchyOriginalResponseFrameObservation, bool) {
	var zero callHierarchyOriginalResponseFrameObservation
	return zero, false
}

func (callHierarchyOriginalRequestFrameObservation) copyFrameBytes() []byte   { return nil }
func (callHierarchyOriginalRequestFrameObservation) copyParamsBytes() []byte  { return nil }
func (callHierarchyOriginalResponseFrameObservation) copyFrameBytes() []byte  { return nil }
func (callHierarchyOriginalResponseFrameObservation) copyResultBytes() []byte { return nil }
func (callHierarchyMethodAllowlist) copyMethods() []callHierarchyMethodIdentity {
	return nil
}

// privateCallHierarchyRequestIdentity is the only mint seam. It intentionally
// returns no identity until Manager-owned workspace/session/generation and exact
// JSON-RPC key custody are connected in a separately reviewed behavior phase.
func (*Manager) privateCallHierarchyRequestIdentity() (callHierarchyRequestIdentity, bool) {
	var zero callHierarchyRequestIdentity
	return zero, false
}

func newCallHierarchyPredecessorChain([]callHierarchyPredecessorCommitment, callHierarchyObservationLimits) (callHierarchyPredecessorChain, bool) {
	var zero callHierarchyPredecessorChain
	return zero, false
}

func newCallHierarchyMethodAllowlist([]callHierarchyMethodIdentity, callHierarchyObservationLimits) (callHierarchyMethodAllowlist, bool) {
	var zero callHierarchyMethodAllowlist
	return zero, false
}

func (callHierarchyMethodAllowlist) permits(callHierarchyMethodIdentity) bool { return false }

func newCallHierarchyObservationLimits(uint64, uint64, uint64, uint64) (callHierarchyObservationLimits, bool) {
	var zero callHierarchyObservationLimits
	return zero, false
}

func newCallHierarchyObservationRequest(callHierarchyObservationIdentity, callHierarchyMethodAllowlist, callHierarchyPredecessorChain, *callHierarchySelectedItemCommitment, *callHierarchyHandoffCommitment, callHierarchyObservationLimits) (callHierarchyObservationRequest, bool) {
	var zero callHierarchyObservationRequest
	return zero, false
}
