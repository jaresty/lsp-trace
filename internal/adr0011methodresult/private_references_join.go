package adr0011methodresult

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

// These private records are consistency evidence only. Neither the peer nor the
// caller-supplied revision authenticates a producer or admits a reference.
const privateReferencesVersion = "lsp-trace.private.adr0011-references-join.UNADMITTED.v0"

var ErrPrivateReferencesJoin = errors.New("private references join invalid")

type PrivateReferenceRelationKind string

const ReferencesSymbol PrivateReferenceRelationKind = "REFERENCES_SYMBOL"

type PrivateReferenceOccurrence struct {
	RelationKind   PrivateReferenceRelationKind
	Identity       string
	Ordinal        int
	SourceRole     string
	URI            string
	Range          Range
	TargetRole     string
	TargetSymbolID string
	TargetURI      string
	TargetRange    adr0011querytarget.Range
}
type PrivateReferencesLedger struct {
	Version, ClaimCeiling                                                        string
	ReferencePairSelector, ReferencePairDigest                                   string
	TargetPairSelector, TargetPairDigest, TargetQuerySelector, TargetQueryDigest string
	Revision, Custody                                                            string
	Query                                                                        adr0011querytarget.Query
	SessionID                                                                    string
	Generation, ReferenceKeyID, TargetKeyID                                      uint64
	Method, ParamsDigest, ResultDigest                                           string
	SourceURI                                                                    string
	SourceVersion                                                                int
	SourceDigest                                                                 string
	N, E, P, A                                                                   int
	Occurrences                                                                  []PrivateReferenceOccurrence
}
type PrivateReferencesReceipt struct{ PairSelector, LedgerSelector, PairDigest, LedgerDigest string }

func privateReferencesExpected(root *publication.Root, pairSelector string, pair []byte, expected OwnerPairExpected, target *DocumentSymbolQueryReceipt, symbolExpected DocumentSymbolPairExpected, query adr0011querytarget.Query, source []byte, revision, custody string) (PrivateReferencesLedger, error) {
	if root == nil || target == nil || pairSelector == "" || pairSelector == target.PairSelector || pairSelector == target.QuerySelector || expected.Method != transport.MethodReferences || expected.Source == nil || symbolExpected.Source == nil || expected.KeyID == symbolExpected.KeyID || expected.SessionID != symbolExpected.SessionID || expected.Generation != symbolExpected.Generation || *expected.Source != *symbolExpected.Source || expected.Source.SHA256 != privateDigest(source) || expected.Source.URI != query.URI || query.DocumentVersion != strconv.Itoa(expected.Source.Version) || query.SessionID != expected.SessionID || query.Generation != expected.Generation || query.Encoding != "utf-16" || query.SourceDigest != expected.Source.SHA256 || revision == "" || custody != "CALLER_ASSERTED" || VerifyOwnerPairCanonical(pair, expected) != nil || ReplayDocumentSymbolQueryReceipt(root, target, symbolExpected, query, source, revision, custody) != nil {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
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
	if strictjson.RejectDuplicates(expected.Params) != nil || json.Unmarshal(expected.Params, &params) != nil || params.TextDocument.URI != query.URI || params.Position.Line != query.Line || params.Position.Character != query.Character || params.Context.IncludeDeclaration == nil || *params.Context.IncludeDeclaration {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	candidate, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(query, symbolExpected.Result)
	if err != nil {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	l := PrivateReferencesLedger{Version: privateReferencesVersion, ClaimCeiling: "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION;NO_COMPLETENESS", ReferencePairSelector: pairSelector, ReferencePairDigest: privateDigest(pair), TargetPairSelector: target.PairSelector, TargetPairDigest: target.PairDigest, TargetQuerySelector: target.QuerySelector, TargetQueryDigest: target.QueryDigest, Revision: revision, Custody: custody, Query: query, SessionID: expected.SessionID, Generation: expected.Generation, ReferenceKeyID: expected.KeyID, TargetKeyID: symbolExpected.KeyID, Method: expected.Method, ParamsDigest: privateDigest(expected.Params), ResultDigest: privateDigest(expected.Result), SourceURI: expected.Source.URI, SourceVersion: expected.Source.Version, SourceDigest: expected.Source.SHA256, N: 1, Occurrences: []PrivateReferenceOccurrence{}}
	// The owner-pair validator rejects malformed members before a ledger exists.
	var members []json.RawMessage
	if bytes.Equal(bytes.TrimSpace(expected.Result), []byte("null")) {
		return l, nil
	}
	if json.Unmarshal(expected.Result, &members) != nil || members == nil || len(members) > ownerPairMaxCandidates {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	l.E = len(members)
	parsed, failure := parseRawUntrusted(expected.Method, expected.Result, ownerPairMaxCandidates)
	if failure != nil {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	l.P = len(parsed.Items)
	for _, item := range parsed.Items {
		l.Occurrences = append(l.Occurrences, PrivateReferenceOccurrence{RelationKind: ReferencesSymbol, Identity: fmt.Sprintf("%s:%d:%d:%d", query.OccurrenceID, expected.Generation, expected.KeyID, item.Ordinal), Ordinal: item.Ordinal, SourceRole: "REFERENCING_OCCURRENCE", URI: item.URI, Range: item.Range, TargetRole: "REFERENCED_SYMBOL", TargetSymbolID: candidate.SymbolID, TargetURI: candidate.QueryURI, TargetRange: candidate.SelectionRange})
	}
	l.A = len(l.Occurrences)
	return l, nil
}
func canonicalReferencesLedger(l PrivateReferencesLedger) ([]byte, error) {
	b, e := json.Marshal(l)
	if e != nil || len(b)+1 > privateQueryReceiptLimit {
		return nil, ErrPrivateReferencesJoin
	}
	return append(b, '\n'), nil
}

// PublishPrivateReferencesJoin binds a verified no-replace owner pair and its
// independently replayable target to an immutable, ordinal-preserving ledger.
func PublishPrivateReferencesJoin(root *publication.Root, pairSelector, ledgerSelector string, pair []byte, expected OwnerPairExpected, target *DocumentSymbolQueryReceipt, symbolExpected DocumentSymbolPairExpected, query adr0011querytarget.Query, source []byte, revision, custody string) (*PrivateReferencesReceipt, error) {
	if ledgerSelector == "" || ledgerSelector == pairSelector || target == nil || ledgerSelector == target.PairSelector || ledgerSelector == target.QuerySelector {
		return nil, ErrPrivateReferencesJoin
	}
	l, err := privateReferencesExpected(root, pairSelector, pair, expected, target, symbolExpected, query, source, revision, custody)
	if err != nil {
		return nil, err
	}
	raw, err := canonicalReferencesLedger(l)
	if err != nil {
		return nil, err
	}
	pr, err := publication.PublishBoundFile(root, pairSelector, pair, func(b []byte) error { return VerifyOwnerPairCanonical(b, expected) })
	if err != nil || pr == nil || pr.VerificationStatus != "VERIFIED" || pr.Digest != l.ReferencePairDigest {
		return nil, ErrPrivateReferencesJoin
	}
	lr, err := publication.PublishBoundFile(root, ledgerSelector, raw, func(b []byte) error {
		if !bytes.Equal(b, raw) {
			return ErrPrivateReferencesJoin
		}
		return nil
	})
	if err != nil || lr == nil || lr.VerificationStatus != "VERIFIED" || lr.Digest != privateDigest(raw) {
		return nil, ErrPrivateReferencesJoin
	}
	receipt := &PrivateReferencesReceipt{pairSelector, ledgerSelector, pr.Digest, lr.Digest}
	if _, err = ReplayPrivateReferencesJoin(root, receipt, expected, target, symbolExpected, query, source, revision, custody); err != nil {
		return nil, err
	}
	return receipt, nil
}
func ReplayPrivateReferencesJoin(root *publication.Root, receipt *PrivateReferencesReceipt, expected OwnerPairExpected, target *DocumentSymbolQueryReceipt, symbolExpected DocumentSymbolPairExpected, query adr0011querytarget.Query, source []byte, revision, custody string) (PrivateReferencesLedger, error) {
	if root == nil || receipt == nil || receipt.PairSelector == "" || receipt.LedgerSelector == "" || receipt.PairSelector == receipt.LedgerSelector || !validDigest(receipt.PairDigest) || !validDigest(receipt.LedgerDigest) {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	pair, err := publication.ReadVerifiedBoundFile(root, receipt.PairSelector, privateQueryReceiptLimit)
	if err != nil || privateDigest(pair) != receipt.PairDigest {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	expectedLedger, err := privateReferencesExpected(root, receipt.PairSelector, pair, expected, target, symbolExpected, query, source, revision, custody)
	if err != nil {
		return PrivateReferencesLedger{}, err
	}
	raw, err := publication.ReadVerifiedBoundFile(root, receipt.LedgerSelector, privateQueryReceiptLimit)
	if err != nil || privateDigest(raw) != receipt.LedgerDigest {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	canonical, err := canonicalReferencesLedger(expectedLedger)
	if err != nil || !bytes.Equal(raw, canonical) || strictjson.RejectDuplicates(raw) != nil {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	var decoded PrivateReferencesLedger
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&decoded) != nil || d.Decode(new(any)) != io.EOF {
		return PrivateReferencesLedger{}, ErrPrivateReferencesJoin
	}
	return expectedLedger, nil
}
