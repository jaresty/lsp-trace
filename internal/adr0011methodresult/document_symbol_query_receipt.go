package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
)

// Private test-owned record. Neither this codec nor its receipt admits a query target.
const privateQueryReceiptVersion = "lsp-trace.private.adr0011-query-receipt.UNADMITTED.v0"
const privateQueryReceiptLimit = 1600000

var ErrPrivateQueryReceipt = errors.New("private unadmitted query receipt invalid")

type DocumentSymbolQueryReceipt struct {
	PairSelector  string
	QuerySelector string
	PairDigest    string
	QueryDigest   string
}

type privateQueryRecord struct {
	Version           string
	ClaimCeiling      string
	PairSelector      string
	PairDigest        string
	WorkspaceRevision string
	RevisionCustody   string
	Query             adr0011querytarget.Query
	Candidate         adr0011querytarget.Candidate
	SessionID         string
	Generation        uint64
	KeyID             uint64
	SourceURI         string
	SourceVersion     int
	SourceDigest      string
}

func privateDigest(raw []byte) string              { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
func sameQuery(a, b adr0011querytarget.Query) bool { return a == b }
func checkPrivateQueryInputs(pair []byte, expected DocumentSymbolPairExpected, q adr0011querytarget.Query, source []byte, revision, custody string) (adr0011querytarget.Candidate, error) {
	if revision == "" || custody != "CALLER_ASSERTED" || len(source) == 0 || expected.Source == nil || expected.Source.SHA256 != privateDigest(source) || expected.Source.URI != q.URI || q.SourceDigest != expected.Source.SHA256 || q.DocumentVersion != strconv.Itoa(expected.Source.Version) || q.SessionID != expected.SessionID || q.Generation != expected.Generation || q.Encoding != "utf-16" || VerifyDocumentSymbolPairCanonical(pair, expected) != nil {
		return adr0011querytarget.Candidate{}, ErrPrivateQueryReceipt
	}
	candidate, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(q, expected.Result)
	if err != nil {
		return adr0011querytarget.Candidate{}, ErrPrivateQueryReceipt
	}
	return candidate, nil
}
func canonicalPrivateQuery(r privateQueryRecord) ([]byte, error) {
	raw, err := json.Marshal(r)
	if err != nil || len(raw)+1 > privateQueryReceiptLimit {
		return nil, ErrPrivateQueryReceipt
	}
	return append(raw, '\n'), nil
}

// PublishDocumentSymbolQueryReceipt is private and test-owned. The caller selects
// a root separate from the workspace; this grants no production publisher role.
func PublishDocumentSymbolQueryReceipt(root *publication.Root, pairSelector, querySelector string, pair []byte, expected DocumentSymbolPairExpected, q adr0011querytarget.Query, source []byte, revision, custody string) (*DocumentSymbolQueryReceipt, error) {
	if root == nil || pairSelector == "" || querySelector == "" || pairSelector == querySelector {
		return nil, ErrPrivateQueryReceipt
	}
	candidate, err := checkPrivateQueryInputs(pair, expected, q, source, revision, custody)
	if err != nil {
		return nil, err
	}
	record := privateQueryRecord{privateQueryReceiptVersion, "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION", pairSelector, privateDigest(pair), revision, custody, q, candidate, expected.SessionID, expected.Generation, expected.KeyID, expected.Source.URI, expected.Source.Version, expected.Source.SHA256}
	queryBytes, err := canonicalPrivateQuery(record)
	if err != nil {
		return nil, err
	}
	pairReceipt, err := publication.PublishBoundFile(root, pairSelector, pair, func(b []byte) error { return VerifyDocumentSymbolPairCanonical(b, expected) })
	if err != nil || pairReceipt == nil {
		return nil, errors.Join(ErrPrivateQueryReceipt, err)
	}
	if pairReceipt.VerificationStatus != "VERIFIED" || pairReceipt.Digest != record.PairDigest {
		return nil, ErrPrivateQueryReceipt
	}
	queryReceipt, err := publication.PublishBoundFile(root, querySelector, queryBytes, func(b []byte) error {
		if !bytes.Equal(b, queryBytes) {
			return ErrPrivateQueryReceipt
		}
		return nil
	})
	if err != nil || queryReceipt == nil {
		return nil, errors.Join(ErrPrivateQueryReceipt, err)
	}
	if queryReceipt.VerificationStatus != "VERIFIED" || queryReceipt.Digest != privateDigest(queryBytes) {
		return nil, ErrPrivateQueryReceipt
	}
	receipt := &DocumentSymbolQueryReceipt{pairSelector, querySelector, pairReceipt.Digest, queryReceipt.Digest}
	if err := ReplayDocumentSymbolQueryReceipt(root, receipt, expected, q, source, revision, custody); err != nil {
		return nil, err
	}
	return receipt, nil
}

// Replay uses only the two bounded immutable selectors and independently supplied
// expected coordinates/source/revision; it never reopens the ambient workspace.
func ReplayDocumentSymbolQueryReceipt(root *publication.Root, receipt *DocumentSymbolQueryReceipt, expected DocumentSymbolPairExpected, q adr0011querytarget.Query, source []byte, revision, custody string) error {
	if root == nil || receipt == nil || receipt.PairSelector == "" || receipt.QuerySelector == "" || receipt.PairSelector == receipt.QuerySelector {
		return ErrPrivateQueryReceipt
	}
	pair, err := publication.ReadVerifiedBoundFile(root, receipt.PairSelector, privateQueryReceiptLimit)
	if err != nil || privateDigest(pair) != receipt.PairDigest {
		return ErrPrivateQueryReceipt
	}
	candidate, err := checkPrivateQueryInputs(pair, expected, q, source, revision, custody)
	if err != nil {
		return err
	}
	raw, err := publication.ReadVerifiedBoundFile(root, receipt.QuerySelector, privateQueryReceiptLimit)
	if err != nil || privateDigest(raw) != receipt.QueryDigest {
		return ErrPrivateQueryReceipt
	}
	record := privateQueryRecord{privateQueryReceiptVersion, "PRIVATE;UNADMITTED;NO_PRODUCER_AUTHENTICATION", receipt.PairSelector, receipt.PairDigest, revision, custody, q, candidate, expected.SessionID, expected.Generation, expected.KeyID, expected.Source.URI, expected.Source.Version, expected.Source.SHA256}
	canonical, err := canonicalPrivateQuery(record)
	if err != nil || !bytes.Equal(raw, canonical) {
		return ErrPrivateQueryReceipt
	}
	var decoded privateQueryRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&decoded) != nil || !sameQuery(decoded.Query, q) || decoded.Candidate != candidate {
		return ErrPrivateQueryReceipt
	}
	return nil
}
