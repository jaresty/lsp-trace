package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/lspwire"
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
// definition/references transaction. It is not a published method receipt,
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

// ownedQueryURI rejects aliases and duplicate keys before comparing a
// predeclared source to the exact manager-owned method query.
func ownedQueryURI(req RoundTripRequest) (string, bool) {
	if strictjson.RejectDuplicates(req.Params) != nil {
		return "", false
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(req.Params, &root) != nil || len(root) != 2 && len(root) != 3 {
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
// successful managed transaction. A zero value cannot issue D/R evidence.
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

func eligibleOwnedMethodRequest(req RoundTripRequest) bool {
	return req.CaptureOwnedMethodPair && (req.Method == "textDocument/definition" || req.Method == "textDocument/references") &&
		req.SessionID != "" && req.Generation != 0 && req.MaxMessages >= 1 && req.MaxMessages <= 64 &&
		req.MaxBytes >= 1 && req.MaxBytes <= 1<<20 && len(req.Params) >= 1 && len(req.Params) <= 64<<10
}

func buildOwnedMethodPair(req RoundTripRequest, r RoundTripResult, params, raw json.RawMessage) *OwnedMethodPair {
	if !eligibleOwnedMethodRequest(req) || r.ServerError != nil || r.Key.Generation != req.Generation || r.Key.ID == 0 ||
		r.RequestMessages != 1 || r.RequestBytes < 1 || r.RequestBytes > req.MaxBytes ||
		r.Messages < 1 || r.Messages > req.MaxMessages || r.Bytes < 1 || r.Bytes > req.MaxBytes ||
		len(raw) < 1 || int64(len(raw)) > req.MaxBytes || !bytes.Equal(raw, r.Result) ||
		r.requestWrite == nil || r.responseRead == nil || r.requestWrite.FrameBytes < 1 || r.responseRead.FrameBytes < 1 ||
		r.requestWrite.SessionID != req.SessionID || r.responseRead.SessionID != req.SessionID ||
		r.requestWrite.Generation != req.Generation || r.responseRead.Generation != req.Generation ||
		r.requestWrite.Key != r.Key || r.responseRead.Key != r.Key || r.requestWrite.Method != req.Method {
		return nil
	}
	pair := &OwnedMethodPair{SessionID: req.SessionID, Generation: req.Generation, Key: r.Key, Method: req.Method,
		Params: append(json.RawMessage(nil), params...), Result: append(json.RawMessage(nil), raw...),
		Write: *r.requestWrite, Read: *r.responseRead}
	if req.ExpectedOwnedDocument != nil {
		source := *req.ExpectedOwnedDocument
		pair.Source = &source
	}
	return pair
}
