package adr0011methodresult

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

// DocumentSymbolPairCanonicalVersion denotes a private, unadmitted pair; it
// does not authenticate a producer, issue a target, or authorize publication.
const DocumentSymbolPairCanonicalVersion = "lsp-trace.private.adr0011-document-symbol-pair.UNADMITTED.NO_PRODUCER_AUTHENTICATION.v0"
const documentSymbolPairMaxBytes = 1500000
const documentSymbolPairMethod = "textDocument/documentSymbol"

var ErrInvalidDocumentSymbolPairCanonical = errors.New("private unadmitted documentSymbol pair invalid")

// DocumentSymbolPairExpected is supplied independently from encoded bytes.
type DocumentSymbolPairExpected struct {
	SessionID         string
	Generation, KeyID uint64
	Method            string
	Params, Result    []byte
	Write             sessionruntime.RequestWriteObservation
	Read              sessionruntime.ResponseReadObservation
	Source            *sessionruntime.OwnedDocumentBinding
}

type documentSymbolPairRecord struct {
	Version      string
	ClaimCeiling string
	SessionID    string
	Generation   uint64
	KeyID        uint64
	Method       string
	ParamsBase64 string
	ResultBase64 string
	Write        sessionruntime.RequestWriteObservation
	Read         sessionruntime.ResponseReadObservation
	Source       *sessionruntime.OwnedDocumentBinding
}

func validDocumentSymbolPair(e DocumentSymbolPairExpected) bool {
	if e.SessionID == "" || e.Generation == 0 || e.KeyID == 0 || e.Method != documentSymbolPairMethod ||
		len(e.Params) == 0 || len(e.Params) > 64<<10 || len(e.Result) == 0 || len(e.Result) > 1<<20 || e.Source == nil ||
		e.Write.SessionID != e.SessionID || e.Write.Generation != e.Generation || e.Write.Key.Generation != e.Generation || e.Write.Key.ID != e.KeyID || e.Write.Method != e.Method ||
		e.Read.SessionID != e.SessionID || e.Read.Generation != e.Generation || e.Read.Key.Generation != e.Generation || e.Read.Key.ID != e.KeyID ||
		e.Write.FrameBytes <= 0 || e.Read.FrameBytes <= 0 || !validDigest(e.Write.FrameSHA256) || !validDigest(e.Read.FrameSHA256) ||
		e.Source.Version <= 0 || !validDigest(e.Source.SHA256) ||
		strictjson.RejectDuplicates(e.Params) != nil || !validDocumentSymbolParams(e.Params) ||
		adr0011querytarget.ValidateDocumentSymbolResultV1(e.Result) != nil {
		return false
	}
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	return json.Unmarshal(e.Params, &params) == nil && e.Source.URI == params.TextDocument.URI
}

func validDocumentSymbolParams(raw []byte) bool {
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil || len(root) != 1 {
		return false
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(root["textDocument"], &doc) != nil || len(doc) != 1 {
		return false
	}
	var uri string
	return json.Unmarshal(doc["uri"], &uri) == nil && uri != ""
}

func BuildDocumentSymbolPairCanonical(pair sessionruntime.OwnedMethodPair) ([]byte, error) {
	e := DocumentSymbolPairExpected{pair.SessionID, pair.Generation, pair.Key.ID, pair.Method, pair.Params, pair.Result, pair.Write, pair.Read, pair.Source}
	if pair.Key.Generation != pair.Generation || !validDocumentSymbolPair(e) {
		return nil, ErrInvalidDocumentSymbolPairCanonical
	}
	r := documentSymbolPairRecord{DocumentSymbolPairCanonicalVersion, "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION", e.SessionID, e.Generation, e.KeyID, e.Method, base64.StdEncoding.EncodeToString(e.Params), base64.StdEncoding.EncodeToString(e.Result), e.Write, e.Read, e.Source}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, ErrInvalidDocumentSymbolPairCanonical
	}
	raw = append(raw, '\n')
	if len(raw) > documentSymbolPairMaxBytes || VerifyDocumentSymbolPairCanonical(raw, e) != nil {
		return nil, ErrInvalidDocumentSymbolPairCanonical
	}
	return raw, nil
}

// VerifyDocumentSymbolPairCanonical checks exact independently expected fields
// and canonical bytes; it does not admit or publish a method result.
func VerifyDocumentSymbolPairCanonical(raw []byte, expected DocumentSymbolPairExpected) error {
	if len(raw) == 0 || len(raw) > documentSymbolPairMaxBytes || !validDocumentSymbolPair(expected) || strictjson.RejectDuplicates(raw) != nil {
		return ErrInvalidDocumentSymbolPairCanonical
	}
	var r documentSymbolPairRecord
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&r) != nil || dec.Decode(new(any)) != io.EOF {
		return ErrInvalidDocumentSymbolPairCanonical
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || r.Version != DocumentSymbolPairCanonicalVersion || r.ClaimCeiling != "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION" ||
		r.SessionID != expected.SessionID || r.Generation != expected.Generation || r.KeyID != expected.KeyID || r.Method != expected.Method ||
		r.Write != expected.Write || r.Read != expected.Read || !sameSource(r.Source, expected.Source) {
		return ErrInvalidDocumentSymbolPairCanonical
	}
	params, err := base64.StdEncoding.DecodeString(r.ParamsBase64)
	if err != nil || !bytes.Equal(params, expected.Params) {
		return ErrInvalidDocumentSymbolPairCanonical
	}
	result, err := base64.StdEncoding.DecodeString(r.ResultBase64)
	if err != nil || !bytes.Equal(result, expected.Result) {
		return ErrInvalidDocumentSymbolPairCanonical
	}
	return nil
}
