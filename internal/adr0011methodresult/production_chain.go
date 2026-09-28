package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

// A test-owned production-shaped family, separate from every private candidate
// codec. Neither these records nor their replay authenticate a producer or
// qualify production issuance.
const ReferencesSymbolV1 = "REFERENCES_SYMBOL_V1"
const targetVersion = "lsp-trace.adr0011.references-symbol.target.v1"
const methodVersion = "lsp-trace.adr0011.references-symbol.method.v1"
const terminalVersion = "lsp-trace.adr0011.references-symbol.terminal.v1"
const occurrenceVersion = "lsp-trace.adr0011.references-symbol.occurrences.v1"
const chainMaxBytes = 1500000

var ErrChain = errors.New("ADR0011 chain not verified")

// Package-private test seam: emulate committed bytes whose readback failed.
var chainPostCommitTestHook func(string) bool

// Package-private fault observation seams used only by bounded regression tests.
var chainEncodeTestHook func(any) bool
var chainBeforePublishTestHook func(string)

type ChainExpectation struct {
	Query           adr0011querytarget.Query
	Source          []byte
	Revision        string
	Custody         string
	SymbolParams    []byte
	ReferenceParams []byte
	TargetGit       HostGitEvidence
	MethodGit       HostGitEvidence
}
type ChainOccurrence struct {
	ID                     string
	Ordinal                int
	URI                    string
	Range                  Range
	SourceRole             string
	TargetRole             string
	TargetSymbolID         string
	Authority              int
	Accepted               bool
	Completeness           string
	ProducerAuthentication string
}
type ChainLedger struct {
	Kind             string
	N, B, T, E, P, A int
	TargetID         string
	Occurrences      []ChainOccurrence
}
type ChainReceipt struct {
	TargetSelector, MethodSelector, TerminalSelector, OccurrenceSelector string
	TargetDigest, MethodDigest, TerminalDigest, OccurrenceDigest         string
}
type TargetReceipt struct{ Selector, Digest string }
type chainPair struct {
	Version, Method, SessionID, ParamsBase64, ResultBase64, Revision, RevisionCustody, SourceDigest string
	Generation, KeyID                                                                               uint64
	Write                                                                                           sessionruntime.RequestWriteObservation
	Read                                                                                            sessionruntime.ResponseReadObservation
	Source                                                                                          sessionruntime.OwnedDocumentBinding
	Query                                                                                           adr0011querytarget.Query
	TargetID                                                                                        string
	SymbolID                                                                                        string
	TargetGit                                                                                       HostGitEvidence
	MethodGit                                                                                       HostGitEvidence
}
type chainTerminal struct {
	Version, MethodSelector, MethodDigest, TargetSelector, TargetDigest, Outcome, Disposition string
	N, B, T, E, EB, ET, P, A                                                                  int
	EvaluationKey                                                                             sessionruntime.RequestWriteObservation
	ResultDigest                                                                              string
	EvaluationEvents                                                                          []ReferenceEvaluationEvent
	TargetGit, MethodGit                                                                      HostGitEvidence
}
type chainOccurrences struct {
	Version, TerminalSelector, TerminalDigest, TargetSelector, TargetDigest, MethodSelector, MethodDigest string
	Ledger                                                                                                ChainLedger
	TargetGit, MethodGit                                                                                  HostGitEvidence
}

func chainDigest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func chainBytes(x any) ([]byte, error) {
	if chainEncodeTestHook != nil && !chainEncodeTestHook(x) {
		return nil, ErrChain
	}
	b, err := json.Marshal(x)
	if err != nil || len(b)+1 > chainMaxBytes {
		return nil, ErrChain
	}
	return append(b, '\n'), nil
}
func chainSelector(role string, b []byte) string {
	return fmt.Sprintf("adr0011-%s-%x.json", role, sha256.Sum256(b))
}
func chainPublish(root *publication.Root, role string, b []byte) (string, string, error) {
	if chainBeforePublishTestHook != nil {
		chainBeforePublishTestHook(role)
	}
	selector := chainSelector(role, b)
	receipt, err := publication.PublishBoundFile(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return ErrChain
		}
		return nil
	})
	if err != nil || receipt == nil || receipt.VerificationStatus != "VERIFIED" || receipt.Digest != chainDigest(b) || chainPostCommitTestHook != nil && !chainPostCommitTestHook(role) {
		return "", "", ErrChain
	}
	got, err := publication.ReadVerifiedBoundFile(root, selector, chainMaxBytes)
	if err != nil || !bytes.Equal(got, b) {
		return "", "", ErrChain
	}
	return selector, receipt.Digest, nil
}
func chainRead(root *publication.Root, selector, digest string, expected any) error {
	if selector == "" || !validDigest(digest) {
		return ErrChain
	}
	raw, err := publication.ReadVerifiedBoundFile(root, selector, chainMaxBytes)
	if err != nil || chainDigest(raw) != digest || selector != chainSelector(chainRole(expected), raw) || strictjson.RejectDuplicates(raw) != nil {
		return ErrChain
	}
	canonical, err := chainBytes(expected)
	if err != nil || !bytes.Equal(raw, canonical) {
		return ErrChain
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	switch x := expected.(type) {
	case chainPair:
		var y chainPair
		if decoder.Decode(&y) != nil || y.Version != x.Version {
			return ErrChain
		}
	case chainTerminal:
		var y chainTerminal
		if decoder.Decode(&y) != nil || y.Version != x.Version {
			return ErrChain
		}
	case chainOccurrences:
		var y chainOccurrences
		if decoder.Decode(&y) != nil || y.Version != x.Version {
			return ErrChain
		}
	default:
		return ErrChain
	}
	if decoder.Decode(new(any)) != io.EOF {
		return ErrChain
	}
	return nil
}
func chainRole(x any) string {
	switch v := x.(type) {
	case chainPair:
		if v.Version == targetVersion {
			return "target"
		}
		return "method"
	case chainTerminal:
		return "terminal"
	default:
		return "occurrences"
	}
}
func expectedPair(pair sessionruntime.OwnedMethodPair, params []byte, method string, e ChainExpectation) (chainPair, error) {
	q := e.Query
	if !ValidHostGitEvidence(e.TargetGit, e.TargetGit.Before.WorkspaceRoot, e.Revision) || pair.Source == nil || q.SessionID == "" || q.Generation == 0 || q.URI == "" || q.OccurrenceID == "" || q.Encoding != "utf-16" || e.Revision == "" || e.Custody != "CALLER_ASSERTED" || len(e.Source) == 0 || len(params) == 0 || !bytes.Equal(params, pair.Params) || pair.Method != method || pair.SessionID != q.SessionID || pair.Generation != q.Generation || pair.Key.Generation != q.Generation || pair.Key.ID == 0 || pair.Source.URI != q.URI || pair.Source.Version <= 0 || strconv.Itoa(pair.Source.Version) != q.DocumentVersion || pair.Source.SHA256 != chainDigest(e.Source) || q.SourceDigest != pair.Source.SHA256 || pair.Write.SessionID != pair.SessionID || pair.Read.SessionID != pair.SessionID || pair.Write.Generation != pair.Generation || pair.Read.Generation != pair.Generation || pair.Write.Key != pair.Key || pair.Read.Key != pair.Key || pair.Write.Method != method || pair.Write.FrameBytes <= 0 || pair.Read.FrameBytes <= 0 || !validDigest(pair.Write.FrameSHA256) || !validDigest(pair.Read.FrameSHA256) || len(pair.Result) == 0 || len(pair.Result) > 1<<20 || len(params) > 64<<10 || strictjson.RejectDuplicates(params) != nil {
		return chainPair{}, ErrChain
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(params, &root) != nil {
		return chainPair{}, ErrChain
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(root["textDocument"], &doc) != nil || len(doc) != 1 {
		return chainPair{}, ErrChain
	}
	var uri string
	if json.Unmarshal(doc["uri"], &uri) != nil || uri != q.URI {
		return chainPair{}, ErrChain
	}
	if method == "textDocument/documentSymbol" {
		if len(root) != 1 || adr0011querytarget.ValidateDocumentSymbolResultV1(pair.Result) != nil {
			return chainPair{}, ErrChain
		}
	} else {
		if method != "textDocument/references" || len(root) != 3 {
			return chainPair{}, ErrChain
		}
		var point map[string]json.RawMessage
		if json.Unmarshal(root["position"], &point) != nil || len(point) != 2 {
			return chainPair{}, ErrChain
		}
		var line, character uint32
		if json.Unmarshal(point["line"], &line) != nil || json.Unmarshal(point["character"], &character) != nil || line != q.Line || character != q.Character {
			return chainPair{}, ErrChain
		}
		var context map[string]json.RawMessage
		if json.Unmarshal(root["context"], &context) != nil || len(context) != 1 {
			return chainPair{}, ErrChain
		}
		var include bool
		if json.Unmarshal(context["includeDeclaration"], &include) != nil || include {
			return chainPair{}, ErrChain
		}
	}
	return chainPair{Method: method, SessionID: pair.SessionID, Generation: pair.Generation, KeyID: pair.Key.ID, ParamsBase64: base64.StdEncoding.EncodeToString(params), ResultBase64: base64.StdEncoding.EncodeToString(pair.Result), Revision: e.Revision, RevisionCustody: e.Custody, SourceDigest: q.SourceDigest, Write: pair.Write, Read: pair.Read, Source: *pair.Source, Query: q, TargetGit: e.TargetGit}, nil
}
func expectedTarget(symbol sessionruntime.OwnedMethodPair, e ChainExpectation) (chainPair, error) {
	target, err := expectedPair(symbol, e.SymbolParams, "textDocument/documentSymbol", e)
	if err != nil {
		return chainPair{}, ErrChain
	}
	candidate, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(e.Query, symbol.Result)
	if err != nil {
		return chainPair{}, ErrChain
	}
	// Derive new family IDs from the verified target data, not the private candidate identity.
	symbolIdentity := struct {
		Query              adr0011querytarget.Query
		Name               string
		Kind               int
		Display, Selection adr0011querytarget.Range
		ResultDigest       string
	}{e.Query, candidate.SymbolName, candidate.SymbolKind, candidate.DisplayRange, candidate.SelectionRange, chainDigest(symbol.Result)}
	idBytes, err := chainBytes(symbolIdentity)
	if err != nil {
		return chainPair{}, err
	}
	target.SymbolID = chainDigest(append([]byte("adr0011:referenced-symbol:v1\x00"), idBytes...))
	target.TargetID = chainDigest(append([]byte("adr0011:target:v1\x00"), []byte(target.SymbolID+e.Query.OccurrenceID)...))
	target.Version = targetVersion
	return target, nil
}
func expectedChain(symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (chainPair, chainPair, ChainLedger, error) {
	target, err := expectedTarget(symbol, e)
	if err != nil {
		return chainPair{}, chainPair{}, ChainLedger{}, ErrChain
	}
	method, err := expectedPair(refs, e.ReferenceParams, "textDocument/references", e)
	if err != nil || target.KeyID == method.KeyID || target.Source != method.Source || target.Revision != method.Revision {
		return chainPair{}, chainPair{}, ChainLedger{}, ErrChain
	}
	if !ValidHostGitEvidence(e.MethodGit, e.TargetGit.Before.WorkspaceRoot, e.Revision) {
		return chainPair{}, chainPair{}, ChainLedger{}, ErrChain
	}
	method.TargetID = target.TargetID
	method.SymbolID = target.SymbolID
	method.MethodGit = e.MethodGit
	evaluation := evaluateCompleteReferences(refs.Key, refs.Result)
	if evaluation.Outcome != "COMPLETE" && evaluation.Outcome != "COMPLETE_EMPTY" {
		return chainPair{}, chainPair{}, ChainLedger{}, ErrChain
	}
	ledger := ChainLedger{Kind: ReferencesSymbolV1, N: evaluation.N, B: evaluation.B, T: evaluation.T, E: evaluation.E, P: evaluation.P, A: evaluation.A, TargetID: target.TargetID, Occurrences: make([]ChainOccurrence, 0, evaluation.P)}
	for _, item := range evaluation.Items {
		identity := fmt.Sprintf("%s:%s:%d:%d:%d", target.TargetID, refs.SessionID, refs.Generation, refs.Key.ID, item.Ordinal)
		ledger.Occurrences = append(ledger.Occurrences, ChainOccurrence{ID: chainDigest([]byte(identity)), Ordinal: item.Ordinal, URI: item.URI, Range: item.Range, SourceRole: "REFERENCING_OCCURRENCE", TargetRole: "REFERENCED_SYMBOL", TargetSymbolID: target.SymbolID, Completeness: "UNKNOWN", ProducerAuthentication: "NO_PRODUCER_AUTHENTICATION"})
	}
	method.Version = methodVersion
	return target, method, ledger, nil
}

// PublishChain is test-owned. The caller must supply separate manager-owned
// transactions; a fabricated in-memory pair alone has no producer authority.
func PublishTarget(root *publication.Root, symbol sessionruntime.OwnedMethodPair, e ChainExpectation) (*TargetReceipt, error) {
	if root == nil {
		return nil, ErrChain
	}
	target, err := expectedTarget(symbol, e)
	if err != nil {
		return nil, err
	}
	raw, err := chainBytes(target)
	if err != nil {
		return nil, err
	}
	selector, digest, err := chainPublish(root, "target", raw)
	if err != nil {
		return nil, err
	}
	if err = chainRead(root, selector, digest, target); err != nil {
		return nil, err
	}
	return &TargetReceipt{selector, digest}, nil
}
func PublishChain(root *publication.Root, symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (*ChainReceipt, error) {
	target, err := PublishTarget(root, symbol, e)
	if err != nil {
		return nil, err
	}
	return PublishReferences(root, target, symbol, refs, e)
}
func PublishReferences(root *publication.Root, verifiedTarget *TargetReceipt, symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (*ChainReceipt, error) {
	// No independently verified final readback/issuance barrier is installed.
	// Fail before publishing a method, terminal, or occurrence candidate.
	return nil, ErrChain
}

func publishReferencesUnissued(root *publication.Root, verifiedTarget *TargetReceipt, symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (*ChainReceipt, error) {
	if root == nil || verifiedTarget == nil {
		return nil, ErrChain
	}
	target, method, ledger, err := expectedChain(symbol, refs, e)
	if err != nil {
		return nil, err
	}
	ts, td := verifiedTarget.Selector, verifiedTarget.Digest
	// A newly supplied independent target expectation must replay before the
	// references method can issue any terminal or occurrence publication.
	if err = chainRead(root, ts, td, target); err != nil {
		return nil, err
	}
	mb, err := chainBytes(method)
	if err != nil {
		return nil, err
	}
	ms, md, err := chainPublish(root, "method", mb)
	if err != nil {
		return nil, err
	}
	if err = chainRead(root, ms, md, method); err != nil {
		return nil, err
	}
	outcome, disposition := "COMPLETE", "ITEMS"
	if ledger.E == 0 {
		outcome, disposition = "COMPLETE_EMPTY", "EMPTY"
	}
	evaluation := evaluateCompleteReferences(refs.Key, refs.Result)
	terminal := chainTerminal{Version: terminalVersion, MethodSelector: ms, MethodDigest: md, TargetSelector: ts, TargetDigest: td, Outcome: outcome, Disposition: disposition, N: evaluation.N, B: evaluation.B, T: evaluation.T, E: evaluation.E, EB: evaluation.EB, ET: evaluation.ET, P: evaluation.P, A: evaluation.A, EvaluationKey: refs.Write, ResultDigest: evaluation.RawDigest, EvaluationEvents: evaluation.Events, TargetGit: e.TargetGit, MethodGit: e.MethodGit}
	traw, err := chainBytes(terminal)
	if err != nil {
		return nil, err
	}
	tls, tld, err := chainPublish(root, "terminal", traw)
	if err != nil {
		return nil, err
	}
	if err = chainRead(root, tls, tld, terminal); err != nil {
		return nil, err
	}
	occurrences := chainOccurrences{Version: occurrenceVersion, TerminalSelector: tls, TerminalDigest: tld, TargetSelector: ts, TargetDigest: td, MethodSelector: ms, MethodDigest: md, Ledger: ledger, TargetGit: e.TargetGit, MethodGit: e.MethodGit}
	ob, err := chainBytes(occurrences)
	if err != nil {
		return nil, err
	}
	os, od, err := chainPublish(root, "occurrences", ob)
	if err != nil {
		return nil, err
	}
	receipt := &ChainReceipt{ts, ms, tls, os, td, md, tld, od}
	if _, err = ReplayChain(root, receipt, symbol, refs, e); err != nil {
		return nil, err
	}
	return receipt, nil
}
func ReplayChain(root *publication.Root, receipt *ChainReceipt, symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (ChainLedger, error) {
	zero := ChainLedger{}
	if root == nil || receipt == nil {
		return zero, ErrChain
	}
	target, method, ledger, err := expectedChain(symbol, refs, e)
	if err != nil {
		return zero, ErrChain
	}
	if err = chainRead(root, receipt.TargetSelector, receipt.TargetDigest, target); err != nil {
		return zero, ErrChain
	}
	if err = chainRead(root, receipt.MethodSelector, receipt.MethodDigest, method); err != nil {
		return zero, ErrChain
	}
	outcome, disposition := "COMPLETE", "ITEMS"
	if ledger.E == 0 {
		outcome, disposition = "COMPLETE_EMPTY", "EMPTY"
	}
	evaluation := evaluateCompleteReferences(refs.Key, refs.Result)
	terminal := chainTerminal{Version: terminalVersion, MethodSelector: receipt.MethodSelector, MethodDigest: receipt.MethodDigest, TargetSelector: receipt.TargetSelector, TargetDigest: receipt.TargetDigest, Outcome: outcome, Disposition: disposition, N: evaluation.N, B: evaluation.B, T: evaluation.T, E: evaluation.E, EB: evaluation.EB, ET: evaluation.ET, P: evaluation.P, A: evaluation.A, EvaluationKey: refs.Write, ResultDigest: evaluation.RawDigest, EvaluationEvents: evaluation.Events, TargetGit: e.TargetGit, MethodGit: e.MethodGit}
	if err = chainRead(root, receipt.TerminalSelector, receipt.TerminalDigest, terminal); err != nil {
		return zero, ErrChain
	}
	occurrences := chainOccurrences{Version: occurrenceVersion, TerminalSelector: receipt.TerminalSelector, TerminalDigest: receipt.TerminalDigest, TargetSelector: receipt.TargetSelector, TargetDigest: receipt.TargetDigest, MethodSelector: receipt.MethodSelector, MethodDigest: receipt.MethodDigest, Ledger: ledger, TargetGit: e.TargetGit, MethodGit: e.MethodGit}
	if err = chainRead(root, receipt.OccurrenceSelector, receipt.OccurrenceDigest, occurrences); err != nil {
		return zero, ErrChain
	}
	return ledger, nil
}
