package adr0011acquisition

import (
	"lsp-trace/internal/publication"
	"testing"
)

func TestPreinvokeRequiresSeparatePreparedBytes(t *testing.T) {
	executable, err := selectGitExecutable("")
	if err != nil {
		t.Fatal(err)
	}
	facts := preinvokeFacts{SessionID: "synthetic", Generation: 1, Workspace: "/tmp/test-worktree", URI: "file:///tmp/test-worktree/a.go", Line: 0, Character: 1, Version: 1, Source: []byte("package a\n"), SourceLength: 10, GitRoot: "/tmp/test-worktree", GitCommit: "0123456789012345678901234567890123456789", Executable: executable}
	facts.SourceDigest = privateDigest(facts.Source)
	root := privatePublicationRoot(t)
	selector, digest, id, err := publishPreinvokeOccurrence(root, facts)
	if err != nil || !verifyPreinvokeOccurrence(root, selector, digest, facts, id) {
		t.Fatal("complete declaration", err)
	}
	record, err := publication.ReadVerifiedBoundFile(root, selector, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	missing := privatePublicationRoot(t)
	if _, err = publication.PublishBoundFile(missing, selector, record, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if verifyPreinvokeOccurrence(missing, selector, digest, facts, id) {
		t.Fatal("missing prepared bytes admitted")
	}
}
