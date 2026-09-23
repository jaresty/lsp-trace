package adr0011methodresult

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

// OwnerPairCanonicalVersion is private and unadmitted, not a method receipt.
const OwnerPairCanonicalVersion = "lsp-trace.private.adr0011-owner-pair.UNADMITTED.NO_PRODUCER_AUTHENTICATION.v0"
const ownerPairMaxBytes = 1500000

var ErrInvalidOwnerPairCanonical = errors.New("private unadmitted owner pair invalid")

// OwnerPairExpected is supplied independently of the serialized bytes. Neither
// this expectation nor the manager's observations authenticate a producer.
type OwnerPairExpected struct {
	SessionID  string
	Generation uint64
	KeyID      uint64
	Method     string
	Params     []byte
	Result     []byte
	Write      sessionruntime.RequestWriteObservation
	Read       sessionruntime.ResponseReadObservation
	Source     *sessionruntime.OwnedDocumentBinding
}

type ownerPairRecord struct {
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

func validOwnerPair(e OwnerPairExpected) bool {
	if e.SessionID == "" || e.Generation == 0 || e.KeyID == 0 ||
		(e.Method != "textDocument/definition" && e.Method != "textDocument/references") ||
		len(e.Params) == 0 || len(e.Params) > 64<<10 || len(e.Result) == 0 || len(e.Result) > 1<<20 ||
		e.Write.SessionID != e.SessionID || e.Write.Generation != e.Generation || e.Write.Key.Generation != e.Generation || e.Write.Key.ID != e.KeyID || e.Write.Method != e.Method ||
		e.Read.SessionID != e.SessionID || e.Read.Generation != e.Generation || e.Read.Key.Generation != e.Generation || e.Read.Key.ID != e.KeyID ||
		e.Write.FrameBytes <= 0 || e.Read.FrameBytes <= 0 || !validDigest(e.Write.FrameSHA256) || !validDigest(e.Read.FrameSHA256) ||
		strictjson.RejectDuplicates(e.Params) != nil || strictjson.RejectDuplicates(e.Result) != nil || transport.ValidateMethodParams(e.Method, e.Params) != nil || !json.Valid(e.Result) {
		return false
	}
	if e.Source != nil {
		var query struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(e.Params, &query) != nil || e.Source.URI != query.TextDocument.URI ||
			e.Source.Version <= 0 || !validDigest(e.Source.SHA256) {
			return false
		}
	}
	return true
}
func validDigest(s string) bool {
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 {
		return false
	}
	for _, c := range s[7:] {
		if c < '0' || c > '9' && c < 'a' || c > 'f' {
			return false
		}
	}
	return true
}

func BuildOwnerPairCanonical(pair sessionruntime.OwnedMethodPair) ([]byte, error) {
	e := OwnerPairExpected{pair.SessionID, pair.Generation, pair.Key.ID, pair.Method, pair.Params, pair.Result, pair.Write, pair.Read, pair.Source}
	if pair.Key.Generation != pair.Generation || !validOwnerPair(e) {
		return nil, ErrInvalidOwnerPairCanonical
	}
	r := ownerPairRecord{OwnerPairCanonicalVersion, "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION", e.SessionID, e.Generation, e.KeyID, e.Method, base64.StdEncoding.EncodeToString(e.Params), base64.StdEncoding.EncodeToString(e.Result), e.Write, e.Read, e.Source}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, ErrInvalidOwnerPairCanonical
	}
	b = append(b, '\n')
	if len(b) > ownerPairMaxBytes || VerifyOwnerPairCanonical(b, e) != nil {
		return nil, ErrInvalidOwnerPairCanonical
	}
	return b, nil
}

// VerifyOwnerPairCanonical checks a closed canonical encoding against an
// independent expected pair; it does not publish or admit any occurrence.
func VerifyOwnerPairCanonical(raw []byte, expected OwnerPairExpected) error {
	if len(raw) == 0 || len(raw) > ownerPairMaxBytes || !validOwnerPair(expected) || strictjson.RejectDuplicates(raw) != nil {
		return ErrInvalidOwnerPairCanonical
	}
	var r ownerPairRecord
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || d.Decode(new(any)) != io.EOF {
		return ErrInvalidOwnerPairCanonical
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || r.Version != OwnerPairCanonicalVersion || r.ClaimCeiling != "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION" ||
		r.SessionID != expected.SessionID || r.Generation != expected.Generation || r.KeyID != expected.KeyID || r.Method != expected.Method ||
		r.Write != expected.Write || r.Read != expected.Read || !sameSource(r.Source, expected.Source) {
		return ErrInvalidOwnerPairCanonical
	}
	params, err := base64.StdEncoding.DecodeString(r.ParamsBase64)
	if err != nil || !bytes.Equal(params, expected.Params) {
		return ErrInvalidOwnerPairCanonical
	}
	result, err := base64.StdEncoding.DecodeString(r.ResultBase64)
	if err != nil || !bytes.Equal(result, expected.Result) {
		return ErrInvalidOwnerPairCanonical
	}
	return nil
}
func sameSource(a, b *sessionruntime.OwnedDocumentBinding) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
