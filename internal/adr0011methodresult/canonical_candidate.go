package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/strictjson"
)

// CanonicalCandidateVersion is a private, unadmitted query/result replay probe,
// NOT the ADR 0011 method-transaction receipt or a grouping-input version.
const CanonicalCandidateVersion = "lsp-trace.private.adr0011-query-result-candidate.v0"
const maxCandidateBytes = 2 << 20

var ErrInvalidCanonicalCandidate = errors.New("unadmitted canonical query/result candidate invalid")

// CanonicalCandidate keeps the exact validated parameter and result JSON bytes
// separately from JSON serialization, which otherwise compacts RawMessage.
// All metadata is reported by the local runtime/transport and forgeable by a
// privileged same-process producer. No field is a query-target identity or a
// terminal-member disposition.
type CanonicalCandidate struct {
	Version, ID, ClaimCeiling                       string
	SessionID, Method, QueryURI                     string
	Generation, KeyID                               uint64
	QueryLine, QueryCharacter                       uint32
	IncludeDeclaration, IncludeDeclarationPresent   bool
	PositionEncoding, ProviderName, ProviderVersion string
	MaxMessages, MaxCandidates, ItemCount           int
	MaxBytes, DeadlineUnixNano                      int64
	ParamsBase64, ParamsSHA256                      string
	ResultBase64, ResultSHA256                      string
	Null                                            bool
}

func queryResultCandidateDigest(b []byte) string {
	h := sha256.New()
	h.Write([]byte("lsp-trace:adr0011:unadmitted-query-result-candidate:v0\x00"))
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(b)))
	h.Write(size[:])
	h.Write(b)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func rawSHA(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// BuildCanonicalCandidate requires a local write match and a bounded successful
// result, but a fake Runtime can still supply self-consistent values. Its bytes
// are a falsifiable *candidate*, never a method receipt or occurrence admission.
func BuildCanonicalCandidate(req transport.Request, wire transport.Result, maxCandidates int) ([]byte, error) {
	obs, ok := wire.Observation()
	raw := wire.Raw()
	if !ok || wire.Outcome() != transport.OutcomeTransportSuccess || wire.Method() != req.Method ||
		obs.LocalWriteCorrespondence != transport.LocalWriteMatch || !obs.MetadataObserved || !obs.ReportedMethodAdvertised || !obs.RoundTripCalled ||
		obs.RawResultDisposition != transport.RawResultRetained || raw == nil ||
		obs.DeclaredSessionID != req.SessionID || obs.DeclaredGeneration != req.Generation || obs.DeclaredMethod != req.Method ||
		obs.ParamsBytes != len(req.Params) || obs.ParamsSHA256 != rawSHA(req.Params) ||
		obs.RawResultBytes != len(raw) || obs.RawResultSHA256 != rawSHA(raw) ||
		obs.ReportedKey.Generation != req.Generation || obs.ReportedKey.ID == 0 ||
		req.SessionID == "" || req.Generation == 0 || req.Deadline.IsZero() ||
		req.MaxMessages < 1 || req.MaxMessages > 64 || req.MaxBytes < 1 || req.MaxBytes > 1<<20 ||
		int64(len(raw)) > req.MaxBytes || maxCandidates < 1 || maxCandidates > 1000 ||
		obs.ReportedPositionEncoding == "" || obs.ReportedProviderName == "" || len(req.Params) > 64<<10 {
		return nil, ErrInvalidCanonicalCandidate
	}
	if transport.ValidateMethodParams(req.Method, req.Params) != nil {
		return nil, ErrInvalidCanonicalCandidate
	}
	parsed, failure := parseRawUntrusted(req.Method, raw, maxCandidates)
	if failure != nil {
		return nil, ErrInvalidCanonicalCandidate
	}
	c := CanonicalCandidate{Version: CanonicalCandidateVersion, ClaimCeiling: "UNADMITTED;NO_PRODUCER_AUTHENTICATION",
		SessionID: req.SessionID, Method: req.Method, QueryURI: obs.DeclaredQueryURI,
		Generation: req.Generation, KeyID: obs.ReportedKey.ID, QueryLine: obs.DeclaredLine, QueryCharacter: obs.DeclaredCharacter,
		IncludeDeclaration: obs.DeclaredIncludeDeclaration, IncludeDeclarationPresent: obs.IncludeDeclarationPresent,
		PositionEncoding: obs.ReportedPositionEncoding, ProviderName: obs.ReportedProviderName, ProviderVersion: obs.ReportedProviderVersion,
		MaxMessages: req.MaxMessages, MaxBytes: req.MaxBytes, DeadlineUnixNano: req.Deadline.UnixNano(), MaxCandidates: maxCandidates,
		ParamsBase64: base64.StdEncoding.EncodeToString(req.Params), ParamsSHA256: rawSHA(req.Params),
		ResultBase64: base64.StdEncoding.EncodeToString(raw), ResultSHA256: rawSHA(raw), ItemCount: len(parsed.Items), Null: parsed.Null}
	pre, _ := json.Marshal(c)
	c.ID = queryResultCandidateDigest(pre)
	encoded, _ := json.Marshal(c)
	encoded = append(encoded, '\n')
	if _, err := VerifyCanonicalCandidate(encoded); err != nil {
		return nil, err
	}
	return encoded, nil
}

// VerifyCanonicalCandidate replays byte/identity and parsing consistency only.
// It never asserts that the named provider returned these bytes.
func VerifyCanonicalCandidate(raw []byte) (CanonicalCandidate, error) {
	bad := func() (CanonicalCandidate, error) { return CanonicalCandidate{}, ErrInvalidCanonicalCandidate }
	if len(raw) < 1 || len(raw) > maxCandidateBytes || strictjson.RejectDuplicates(raw) != nil {
		return bad()
	}
	var c CanonicalCandidate
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&c) != nil || c.Version != CanonicalCandidateVersion || c.ClaimCeiling != "UNADMITTED;NO_PRODUCER_AUTHENTICATION" ||
		c.SessionID == "" || c.Generation == 0 || c.KeyID == 0 || c.QueryURI == "" || c.PositionEncoding == "" || c.ProviderName == "" ||
		c.DeadlineUnixNano <= 0 || c.MaxMessages < 1 || c.MaxMessages > 64 || c.MaxBytes < 1 || c.MaxBytes > 1<<20 ||
		c.MaxCandidates < 1 || c.MaxCandidates > 1000 || c.ItemCount < 0 || c.ItemCount > c.MaxCandidates {
		return bad()
	}
	canonical, _ := json.Marshal(c)
	if !bytes.Equal(raw, append(canonical, '\n')) {
		return bad()
	}
	id := c.ID
	c.ID = ""
	pre, _ := json.Marshal(c)
	if id != queryResultCandidateDigest(pre) {
		return bad()
	}
	params, err := base64.StdEncoding.DecodeString(c.ParamsBase64)
	if err != nil || len(params) > 64<<10 || c.ParamsSHA256 != rawSHA(params) || transport.ValidateMethodParams(c.Method, params) != nil {
		return bad()
	}
	result, err := base64.StdEncoding.DecodeString(c.ResultBase64)
	if err != nil || len(result) == 0 || int64(len(result)) > c.MaxBytes || c.ResultSHA256 != rawSHA(result) {
		return bad()
	}
	var query struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct{ Line, Character uint32 } `json:"position"`
		Context  *struct {
			IncludeDeclaration bool `json:"includeDeclaration"`
		} `json:"context"`
	}
	if json.Unmarshal(params, &query) != nil || query.TextDocument.URI != c.QueryURI || query.Position.Line != c.QueryLine || query.Position.Character != c.QueryCharacter ||
		(query.Context != nil) != c.IncludeDeclarationPresent || query.Context != nil && query.Context.IncludeDeclaration != c.IncludeDeclaration {
		return bad()
	}
	parsed, failure := parseRawUntrusted(c.Method, result, c.MaxCandidates)
	if failure != nil || len(parsed.Items) != c.ItemCount || parsed.Null != c.Null {
		return bad()
	}
	c.ID = id
	return c, nil
}

// CandidateResultBytes copies the exact retained JSON result only after the
// candidate has passed local offline replay; never use it as admitted evidence.
func CandidateResultBytes(raw []byte) ([]byte, error) {
	c, err := VerifyCanonicalCandidate(raw)
	if err != nil {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(c.ResultBase64)
	if err != nil {
		return nil, fmt.Errorf("candidate replay: %w", ErrInvalidCanonicalCandidate)
	}
	return b, nil
}
