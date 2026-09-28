package adr0011methodresult

import (
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// ReferenceDiagnosticReceipt is a verified diagnostic, never an occurrence receipt.
type ReferenceDiagnosticReceipt struct {
	TargetSelector, MethodSelector, TerminalSelector string
	TargetDigest, MethodDigest, TerminalDigest       string
}

type referenceDiagnosticTerminal struct {
	Version, MethodSelector, MethodDigest, TargetSelector, TargetDigest string
	Evaluation                                                          ReferenceEvaluation
	TargetGit, MethodGit                                                HostGitEvidence
}

// PublishReferenceDiagnostic only accepts a complete manager-owned method pair.
// The evaluation is bound to its keyed raw response; malformed/limited results
// cannot reach the successful occurrence publication path.
func PublishReferenceDiagnostic(root *publication.Root, verifiedTarget *TargetReceipt, symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (*ReferenceDiagnosticReceipt, error) {
	// The candidate method record includes unredacted ResultBase64. Disable this
	// publication until a redacted diagnostic schema and readback are verified.
	return nil, ErrChain
}

func publishReferenceDiagnosticUnissued(root *publication.Root, verifiedTarget *TargetReceipt, symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (*ReferenceDiagnosticReceipt, error) {
	if root == nil || verifiedTarget == nil {
		return nil, ErrChain
	}
	target, err := expectedTarget(symbol, e)
	if err != nil {
		return nil, ErrChain
	}
	method, err := expectedPair(refs, e.ReferenceParams, "textDocument/references", e)
	if err != nil || target.KeyID == method.KeyID || target.Source != method.Source || target.Revision != method.Revision || !ValidHostGitEvidence(e.MethodGit, e.TargetGit.Before.WorkspaceRoot, e.Revision) {
		return nil, ErrChain
	}
	evaluation := evaluateCompleteReferences(refs.Key, refs.Result)
	if evaluation.Outcome != "MALFORMED" && evaluation.Outcome != "RESOURCE_LIMIT" {
		return nil, ErrChain
	}
	if err = chainRead(root, verifiedTarget.Selector, verifiedTarget.Digest, target); err != nil {
		return nil, ErrChain
	}
	method.TargetID = target.TargetID
	method.SymbolID = target.SymbolID
	method.MethodGit = e.MethodGit
	method.Version = methodVersion
	raw, err := chainBytes(method)
	if err != nil {
		return nil, err
	}
	ms, md, err := chainPublish(root, "method", raw)
	if err != nil {
		return nil, err
	}
	if err = chainRead(root, ms, md, method); err != nil {
		return nil, err
	}
	terminal := referenceDiagnosticTerminal{Version: "lsp-trace.adr0011.references-symbol.diagnostic.v1", MethodSelector: ms, MethodDigest: md, TargetSelector: verifiedTarget.Selector, TargetDigest: verifiedTarget.Digest, Evaluation: evaluation, TargetGit: e.TargetGit, MethodGit: e.MethodGit}
	raw, err = chainBytes(terminal)
	if err != nil {
		return nil, err
	}
	ts, td, err := chainPublish(root, "diagnostic", raw)
	if err != nil {
		return nil, err
	}
	if err = readReferenceDiagnostic(root, ts, td, terminal); err != nil {
		return nil, err
	}
	return &ReferenceDiagnosticReceipt{verifiedTarget.Selector, ms, ts, verifiedTarget.Digest, md, td}, nil
}

func readReferenceDiagnostic(root *publication.Root, selector, digest string, expected referenceDiagnosticTerminal) error {
	raw, err := chainBytes(expected)
	if err != nil {
		return ErrChain
	}
	got, err := publication.ReadVerifiedBoundFile(root, selector, chainMaxBytes)
	if err != nil || selector != chainSelector("diagnostic", raw) || digest != chainDigest(raw) || string(got) != string(raw) {
		return ErrChain
	}
	return nil
}

func ReplayReferenceDiagnostic(root *publication.Root, receipt *ReferenceDiagnosticReceipt, symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (ReferenceEvaluation, error) {
	if root == nil || receipt == nil {
		return ReferenceEvaluation{}, ErrChain
	}
	target, err := expectedTarget(symbol, e)
	if err != nil {
		return ReferenceEvaluation{}, ErrChain
	}
	method, err := expectedPair(refs, e.ReferenceParams, "textDocument/references", e)
	if err != nil || !ValidHostGitEvidence(e.MethodGit, e.TargetGit.Before.WorkspaceRoot, e.Revision) {
		return ReferenceEvaluation{}, ErrChain
	}
	evaluation := evaluateCompleteReferences(refs.Key, refs.Result)
	if evaluation.Outcome != "MALFORMED" && evaluation.Outcome != "RESOURCE_LIMIT" {
		return ReferenceEvaluation{}, ErrChain
	}
	if err = chainRead(root, receipt.TargetSelector, receipt.TargetDigest, target); err != nil {
		return ReferenceEvaluation{}, ErrChain
	}
	method.TargetID = target.TargetID
	method.SymbolID = target.SymbolID
	method.MethodGit = e.MethodGit
	method.Version = methodVersion
	if err = chainRead(root, receipt.MethodSelector, receipt.MethodDigest, method); err != nil {
		return ReferenceEvaluation{}, ErrChain
	}
	terminal := referenceDiagnosticTerminal{Version: "lsp-trace.adr0011.references-symbol.diagnostic.v1", MethodSelector: receipt.MethodSelector, MethodDigest: receipt.MethodDigest, TargetSelector: receipt.TargetSelector, TargetDigest: receipt.TargetDigest, Evaluation: evaluation, TargetGit: e.TargetGit, MethodGit: e.MethodGit}
	if err = readReferenceDiagnostic(root, receipt.TerminalSelector, receipt.TerminalDigest, terminal); err != nil {
		return ReferenceEvaluation{}, ErrChain
	}
	return evaluation, nil
}
