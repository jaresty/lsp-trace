package adr0011methodresult

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

// This private, test-owned terminal is consistency evidence only. It never
// admits a typed occurrence or authenticates a producer or complete search.
const privateMalformedTerminalVersion = "lsp-trace.private.adr0011-malformed-terminal.UNADMITTED.v0"
const privateMalformedTerminalLimit = 1500000

var ErrPrivateMalformedTerminal = errors.New("private malformed terminal invalid")

type PrivateMalformedTerminal struct {
	Version         string
	ClaimCeiling    string
	Terminal        string
	Transport       string
	SessionID       string
	Generation      uint64
	KeyID           uint64
	Method          string
	ParamsBase64    string
	ResultBase64    string
	ResultLength    int
	ResultDigest    string
	Write           sessionruntime.RequestWriteObservation
	Read            sessionruntime.ResponseReadObservation
	Source          sessionruntime.OwnedDocumentBinding
	QueryLine       uint32
	QueryCharacter  uint32
	Revision        string
	RevisionCustody string
	N, E, P, A      int
	ErrorOrdinal    int
}
type PrivateMalformedTerminalReceipt struct{ Selector, Digest string }

func malformedTerminalExpected(e OwnerPairExpected, source []byte, line, character uint32, revision, custody string) (PrivateMalformedTerminal, error) {
	if e.SessionID == "" || e.Generation == 0 || e.KeyID == 0 || e.Method != transport.MethodReferences || len(e.Params) == 0 || len(e.Params) > 64<<10 || len(e.Result) == 0 || len(e.Result) > 1<<20 || len(source) == 0 || e.Source == nil || e.Source.Version <= 0 || e.Source.SHA256 != privateDigest(source) || revision == "" || custody != "CALLER_ASSERTED" ||
		e.Write.SessionID != e.SessionID || e.Write.Generation != e.Generation || e.Write.Key.Generation != e.Generation || e.Write.Key.ID != e.KeyID || e.Write.Method != e.Method || e.Read.SessionID != e.SessionID || e.Read.Generation != e.Generation || e.Read.Key.Generation != e.Generation || e.Read.Key.ID != e.KeyID || e.Write.FrameBytes <= 0 || e.Read.FrameBytes <= 0 || !validDigest(e.Write.FrameSHA256) || !validDigest(e.Read.FrameSHA256) ||
		strictjson.RejectDuplicates(e.Params) != nil || transport.ValidateMethodParams(e.Method, e.Params) != nil || !json.Valid(e.Result) {
		return PrivateMalformedTerminal{}, ErrPrivateMalformedTerminal
	}
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      uint32 `json:"line"`
			Character uint32 `json:"character"`
		} `json:"position"`
		Context struct {
			IncludeDeclaration *bool `json:"includeDeclaration"`
		} `json:"context"`
	}
	if json.Unmarshal(e.Params, &params) != nil || params.TextDocument.URI != e.Source.URI || params.Position.Line != line || params.Position.Character != character || params.Context.IncludeDeclaration == nil || *params.Context.IncludeDeclaration {
		return PrivateMalformedTerminal{}, ErrPrivateMalformedTerminal
	}
	var members []json.RawMessage
	if json.Unmarshal(e.Result, &members) != nil || members == nil || len(members) != 2 {
		return PrivateMalformedTerminal{}, ErrPrivateMalformedTerminal
	}
	parsed, failure := parseRawUntrusted(e.Method, e.Result, ownerPairMaxCandidates)
	if failure == nil || failure.Code != FailureMalformed || failure.Ordinal != 1 || len(parsed.Items) != 0 {
		return PrivateMalformedTerminal{}, ErrPrivateMalformedTerminal
	}
	first, firstFailure := parseRawUntrusted(e.Method, append(append([]byte{'['}, members[0]...), ']'), ownerPairMaxCandidates)
	if firstFailure != nil || len(first.Items) != 1 {
		return PrivateMalformedTerminal{}, ErrPrivateMalformedTerminal
	}
	return PrivateMalformedTerminal{Version: privateMalformedTerminalVersion, ClaimCeiling: "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION;NO_COMPLETENESS;NO_CALLS;NO_SOURCE_CONTEXT", Terminal: "MALFORMED", Transport: "SUCCESS", SessionID: e.SessionID, Generation: e.Generation, KeyID: e.KeyID, Method: e.Method, ParamsBase64: base64.StdEncoding.EncodeToString(e.Params), ResultBase64: base64.StdEncoding.EncodeToString(e.Result), ResultLength: len(e.Result), ResultDigest: privateDigest(e.Result), Write: e.Write, Read: e.Read, Source: *e.Source, QueryLine: line, QueryCharacter: character, Revision: revision, RevisionCustody: custody, N: 1, E: 2, P: 1, A: 0, ErrorOrdinal: 1}, nil
}
func canonicalMalformedTerminal(r PrivateMalformedTerminal) ([]byte, error) {
	raw, err := json.Marshal(r)
	if err != nil || len(raw)+1 > privateMalformedTerminalLimit {
		return nil, ErrPrivateMalformedTerminal
	}
	return append(raw, '\n'), nil
}
func BuildPrivateMalformedTerminal(pair sessionruntime.OwnedMethodPair, e OwnerPairExpected, source []byte, line, character uint32, revision, custody string) ([]byte, error) {
	if pair.SessionID != e.SessionID || pair.Generation != e.Generation || pair.Key.Generation != e.Generation || pair.Key.ID != e.KeyID || pair.Method != e.Method || !bytes.Equal(pair.Params, e.Params) || !bytes.Equal(pair.Result, e.Result) || pair.Write != e.Write || pair.Read != e.Read || !sameSource(pair.Source, e.Source) {
		return nil, ErrPrivateMalformedTerminal
	}
	r, err := malformedTerminalExpected(e, source, line, character, revision, custody)
	if err != nil {
		return nil, err
	}
	raw, err := canonicalMalformedTerminal(r)
	if err != nil {
		return nil, err
	}
	if _, err = VerifyPrivateMalformedTerminal(raw, e, source, line, character, revision, custody); err != nil {
		return nil, err
	}
	return raw, nil
}
func VerifyPrivateMalformedTerminal(raw []byte, e OwnerPairExpected, source []byte, line, character uint32, revision, custody string) (PrivateMalformedTerminal, error) {
	fail := func() (PrivateMalformedTerminal, error) {
		return PrivateMalformedTerminal{}, ErrPrivateMalformedTerminal
	}
	if len(raw) == 0 || len(raw) > privateMalformedTerminalLimit || strictjson.RejectDuplicates(raw) != nil {
		return fail()
	}
	expected, err := malformedTerminalExpected(e, source, line, character, revision, custody)
	if err != nil {
		return fail()
	}
	var got PrivateMalformedTerminal
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&got) != nil || dec.Decode(new(any)) != io.EOF {
		return fail()
	}
	canonical, err := canonicalMalformedTerminal(got)
	if err != nil || !bytes.Equal(canonical, raw) {
		return fail()
	}
	// Equality of the canonical record ties every independently supplied field,
	// including the exact raw result and method key, to the offline replay.
	want, err := canonicalMalformedTerminal(expected)
	if err != nil || !bytes.Equal(want, raw) {
		return fail()
	}
	if got.Source.Version <= 0 || got.Source.URI == "" || got.Source.SHA256 != privateDigest(source) {
		return fail()
	}
	return got, nil
}
func PublishPrivateMalformedTerminal(root *publication.Root, selector string, raw []byte, e OwnerPairExpected, source []byte, line, character uint32, revision, custody string) (*PrivateMalformedTerminalReceipt, error) {
	if root == nil || selector == "" {
		return nil, ErrPrivateMalformedTerminal
	}
	if _, err := VerifyPrivateMalformedTerminal(raw, e, source, line, character, revision, custody); err != nil {
		return nil, err
	}
	receipt, err := publication.PublishBoundFile(root, selector, raw, func(b []byte) error {
		_, err := VerifyPrivateMalformedTerminal(b, e, source, line, character, revision, custody)
		return err
	})
	if err != nil {
		return nil, err
	}
	if receipt == nil || receipt.VerificationStatus != "VERIFIED" || receipt.Digest != privateDigest(raw) {
		return nil, ErrPrivateMalformedTerminal
	}
	return &PrivateMalformedTerminalReceipt{selector, receipt.Digest}, nil
}
func ReplayPrivateMalformedTerminal(root *publication.Root, receipt *PrivateMalformedTerminalReceipt, e OwnerPairExpected, source []byte, line, character uint32, revision, custody string) (PrivateMalformedTerminal, error) {
	if root == nil || receipt == nil || receipt.Selector == "" || !validDigest(receipt.Digest) {
		return PrivateMalformedTerminal{}, ErrPrivateMalformedTerminal
	}
	raw, err := publication.ReadVerifiedBoundFile(root, receipt.Selector, privateMalformedTerminalLimit)
	if err != nil || privateDigest(raw) != receipt.Digest {
		return PrivateMalformedTerminal{}, ErrPrivateMalformedTerminal
	}
	return VerifyPrivateMalformedTerminal(raw, e, source, line, character, revision, custody)
}
