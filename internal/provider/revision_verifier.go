package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"lsp-trace/internal/graph"
)

// RevisionVerifier is host-owned authority for revision-bound document custody.
// Providers cannot construct or influence an implementation of this interface.
type RevisionVerifier interface {
	VerifyRevision(context.Context, StrictCollectorRequest, graph.SourceDocumentRecord) error
}

// RevisionVerifierFunc adapts a function to the host-owned verifier seam.
type RevisionVerifierFunc func(context.Context, StrictCollectorRequest, graph.SourceDocumentRecord) error

func (f RevisionVerifierFunc) VerifyRevision(ctx context.Context, request StrictCollectorRequest, document graph.SourceDocumentRecord) error {
	return f(ctx, request, document)
}

// RevisionCustodyError marks a failed host custody proof.
type RevisionCustodyError struct{ Err error }

func (e *RevisionCustodyError) Error() string { return "relation custody failed: " + e.Err.Error() }
func (e *RevisionCustodyError) Unwrap() error { return e.Err }

func IsRevisionCustodyError(err error) bool {
	var target *RevisionCustodyError
	return errors.As(err, &target)
}

// ManagedGitRevision is authenticated metadata captured by the host for one
// exact managed-session generation.
type ManagedGitRevision struct {
	SessionID      string
	Generation     uint64
	RepositoryRoot string
	Commit         string
}

type GitRevisionVerifier struct {
	revisions map[string]ManagedGitRevision
}

func NewGitRevisionVerifier(revisions []ManagedGitRevision) *GitRevisionVerifier {
	indexed := make(map[string]ManagedGitRevision, len(revisions))
	for _, revision := range revisions {
		indexed[revisionKey(revision.SessionID, revision.Generation)] = revision
	}
	return &GitRevisionVerifier{revisions: indexed}
}

func revisionKey(sessionID string, generation uint64) string {
	return fmt.Sprintf("%s\x00%d", sessionID, generation)
}

func custodyError(format string, args ...any) error {
	return &RevisionCustodyError{Err: fmt.Errorf(format, args...)}
}

func (v *GitRevisionVerifier) VerifyRevision(ctx context.Context, request StrictCollectorRequest, document graph.SourceDocumentRecord) error {
	if v == nil {
		return custodyError("host revision verifier is unavailable")
	}
	metadata, ok := v.revisions[revisionKey(request.Session.SessionID, request.Session.Generation)]
	if !ok || metadata.RepositoryRoot == "" || metadata.Commit == "" {
		return custodyError("authenticated managed-session Git metadata is unavailable")
	}
	var workspace graph.RevisionIdentity
	if len(request.Documents.WorkspaceRevision) == 0 {
		return custodyError("request omitted Git workspace revision")
	}
	if err := decodeCollectorInput(request.Documents.WorkspaceRevision, &workspace); err != nil {
		return custodyError("request workspace revision: %v", err)
	}
	if workspace.Kind != "git" || workspace.Custody != graph.CustodyCallerAsserted {
		return custodyError("request Git revision must be caller asserted")
	}
	if workspace.Value != metadata.Commit || document.Revision.Kind != "git" || document.Revision.Value != metadata.Commit {
		return custodyError("Git commit does not match authenticated managed-session revision")
	}
	// The provider may honestly carry the caller's assertion; proof is established
	// here from managed-session metadata and Git bytes, never from that label.
	if document.Revision.Custody != graph.CustodyCallerAsserted && document.Revision.Custody != graph.CustodyProviderProved {
		return custodyError("provider document omitted revision assertion")
	}
	parsed, err := url.Parse(document.OriginalURI)
	if err != nil || parsed.Scheme != "file" || parsed.Host != "" {
		return custodyError("original document must be a local file URI")
	}
	root, err := filepath.Abs(metadata.RepositoryRoot)
	if err != nil {
		return custodyError("repository root: %v", err)
	}
	original, err := filepath.Abs(filepath.FromSlash(parsed.Path))
	if err != nil {
		return custodyError("original path: %v", err)
	}
	relative, err := filepath.Rel(root, original)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return custodyError("original path escapes authenticated repository")
	}
	gitPath := filepath.ToSlash(relative)
	if gitPath == "." || strings.Contains(gitPath, ":") {
		return custodyError("original path is not a safe Git object path")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "show", metadata.Commit+":"+gitPath)
	content, err := cmd.Output()
	if err != nil {
		return custodyError("Git object is unavailable: %v", err)
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	if document.ContentSHA256 != digest {
		return custodyError("provider content SHA-256 does not match pinned Git object")
	}
	if document.Revision.Blob != digest {
		return custodyError("provider blob content digest does not match pinned Git object bytes")
	}
	return nil
}
