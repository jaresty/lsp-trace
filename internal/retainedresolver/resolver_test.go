package retainedresolver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedmanifest"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
)

const assertR02 = "ASSERT_R02_GIT_CONTENT_ADDRESS_DIRTY_NON_GIT_CUSTODY"

type countingObjects struct {
	objects map[sourceobject.Identity]sourceobject.Object
	calls   int
}

func (c *countingObjects) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	c.calls++
	if object, ok := c.objects[id]; ok {
		return object, nil
	}
	return sourceobject.Object{}, &sourceobject.Error{Code: sourceobject.CodeMissing}
}

type countingGit struct {
	object sourceobject.Object
	err    error
	calls  int
}

func (c *countingGit) Get(_ GitBinding, _ sourceobject.Identity) (sourceobject.Object, error) {
	c.calls++
	return c.object, c.err
}

func identity(raw []byte) sourceobject.Identity {
	sum := sha256.Sum256(raw)
	return sourceobject.Identity{Digest: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: uint64(len(raw))}
}

func manifestFixture(t *testing.T, entries []retainedmanifest.EntryInput) ([]byte, []byte, []byte, string, retainedmanifest.Manifest) {
	t.Helper()
	uri := "file:///w/a.go"
	origin := graph.NewNode(graph.Item{Name: "A", Kind: 6, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	candidate := graph.NewNode(graph.Item{Name: "B", Kind: 6, URI: uri, Range: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}, SelectionRange: graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 2, Character: 1}}})
	seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: uri, ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Seeds: []graph.SeedResult{{Label: "seed"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	g.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: uri, SeedLabel: "seed", SeedIdentity: "session:seed:a.go:1:1", Origin: origin, Declaration: &candidate, Candidate: candidate, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: seed.ContentSHA256, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	graphBytes, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	source := graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: "file:///w", AnalyzedVersion: graphprovenance.Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Supplies: []graphprovenance.SupplyReceiptV2{}, Captures: []graphprovenance.Receipt{}, Bindings: []graphprovenance.BindingV2{}}
	capture, err := graphprovenance.CaptureV5(graphBytes, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &source)
	if err != nil {
		t.Fatal(err)
	}
	raw, manifestID, err := retainedmanifest.Build(graphBytes, capture, entries)
	if err != nil {
		t.Fatal(err)
	}
	manifest, admittedID, err := retainedmanifest.Admit(raw, graphBytes, capture)
	if err != nil || admittedID != manifestID {
		t.Fatalf("manifest: %v", err)
	}
	return raw, graphBytes, capture, manifestID, manifest
}

func entryFor(id sourceobject.Identity, class, availability, custody string) retainedmanifest.EntryInput {
	return retainedmanifest.EntryInput{Source: id, StorageClass: class, Qualification: "QUALIFIED", PrivacyClassification: "PUBLIC", Availability: availability, Role: "ENDPOINT", GraphSubjectID: "node-a", LogicalSourceID: "file:///w/a.go", Range: sourceprojection.Range{End: sourceprojection.Position{Character: 1}}, PositionEncoding: "utf-8", PolicyID: "policy-1", CustodyIdentity: custody}
}

func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func realGitFixture(t *testing.T) (string, GitBinding, []byte) {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "config", "user.email", "r02@example.invalid")
	gitRun(t, root, "config", "user.name", "R02")
	body := []byte("package fixture\n\nfunc Exact() {}\n")
	if err := os.WriteFile(filepath.Join(root, "a.go"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "a.go")
	gitRun(t, root, "commit", "-qm", "fixture")
	commit := gitRun(t, root, "rev-parse", "HEAD")
	tree := gitRun(t, root, "rev-parse", "HEAD^{tree}")
	blob := gitRun(t, root, "rev-parse", "HEAD:a.go")
	return root, GitBinding{Commit: commit, Tree: tree, Blob: blob, Path: "a.go"}, body
}

func TestR02RealGitResolvesExactCommitTreeBlobNotDirtyCheckout(t *testing.T) {
	root, binding, committed := realGitFixture(t)
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("neighbor dirty bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id := identity(committed)
	entry := entryFor(id, "GIT_BLOB", "AVAILABLE", GitCustodyIdentity(binding))
	raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
	gitLookup, err := NewGitLookup(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID, Git: &binding}, Dependencies{Git: gitLookup})
	if err != nil || string(out.Object.Bytes) != string(committed) || out.Object.Identity != id {
		t.Fatalf("%s_REAL_GIT: out=%+v err=%v", assertR02, out, err)
	}
}

func TestR02DirtyAndNonGitRequireRetainedImmutableObjects(t *testing.T) {
	for _, class := range []string{"CONTENT_ADDRESS", "EMBEDDED_IMMUTABLE"} {
		t.Run(class, func(t *testing.T) {
			body := []byte("unsaved or non-git bytes")
			id := identity(body)
			entry := entryFor(id, class, "AVAILABLE", ContentCustodyIdentity(class, id))
			raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
			rootDir := t.TempDir()
			if err := os.Chmod(rootDir, 0o700); err != nil {
				t.Fatal(err)
			}
			root, err := publication.OpenRoot(rootDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = root.Close() })
			objects, err := sourceobject.New(root, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			published, err := objects.Publish(body)
			if err != nil || published != id {
				t.Fatalf("publish: id=%+v err=%v", published, err)
			}
			gitLookup := &countingGit{}
			out, err := Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID}, Dependencies{Git: gitLookup, Objects: objects})
			if err != nil || string(out.Object.Bytes) != string(body) || gitLookup.calls != 0 {
				t.Fatalf("%s_%s: out=%+v git=%d err=%v", assertR02, class, out, gitLookup.calls, err)
			}
		})
	}
}

func TestR02DigestOnlyDirtySourceIsUnavailableWithoutLookup(t *testing.T) {
	id := identity([]byte("digest-only dirty"))
	entry := entryFor(id, "UNAVAILABLE", "UNAVAILABLE", UnavailableCustodyIdentity(id))
	raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
	objects, gitLookup := &countingObjects{}, &countingGit{}
	_, err := Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID}, Dependencies{Git: gitLookup, Objects: objects})
	if !IsCode(err, CodeUnavailable) || objects.calls != 0 || gitLookup.calls != 0 {
		t.Fatalf("%s_DIGEST_ONLY: objects=%d git=%d err=%v", assertR02, objects.calls, gitLookup.calls, err)
	}
}

func TestR02FailClosedMutationCorpus(t *testing.T) {
	root, binding, committed := realGitFixture(t)
	id := identity(committed)
	entry := entryFor(id, "GIT_BLOB", "AVAILABLE", GitCustodyIdentity(binding))
	raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
	gitLookup, err := NewGitLookup(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	base := Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID, Git: &binding}
	mutations := []struct {
		name   string
		mutate func(*Request)
	}{
		{"manifest", func(r *Request) { r.ManifestDigest = "sha256:" + strings.Repeat("0", 64) }},
		{"entry", func(r *Request) { r.EntryID = "sha256:" + strings.Repeat("0", 64) }},
		{"commit", func(r *Request) { b := *r.Git; b.Commit = strings.Repeat("0", 40); r.Git = &b }},
		{"tree", func(r *Request) { b := *r.Git; b.Tree = strings.Repeat("0", 40); r.Git = &b }},
		{"blob", func(r *Request) { b := *r.Git; b.Blob = strings.Repeat("0", 40); r.Git = &b }},
		{"path", func(r *Request) { b := *r.Git; b.Path = "neighbor.go"; r.Git = &b }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			if _, err := Resolve(raw, graphBytes, capture, req, Dependencies{Git: gitLookup}); err == nil {
				t.Fatalf("%s_MUTATION_%s", assertR02, tc.name)
			}
		})
	}
}

func TestR02AlternateClassAndReturnedBytesCannotSatisfyRetrieval(t *testing.T) {
	body := []byte("retained bytes")
	id := identity(body)
	entry := entryFor(id, "CONTENT_ADDRESS", "AVAILABLE", ContentCustodyIdentity("CONTENT_ADDRESS", id))
	raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
	wrong := identity([]byte("neighbor bytes"))
	objects := &countingObjects{objects: map[sourceobject.Identity]sourceobject.Object{id: {Identity: wrong, Bytes: []byte("neighbor bytes")}}}
	binding := GitBinding{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), Blob: strings.Repeat("c", 40), Path: "a.go"}
	_, err := Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID, Git: &binding}, Dependencies{Objects: objects})
	if !IsCode(err, CodeClassMismatch) || objects.calls != 0 {
		t.Fatalf("%s_ALTERNATE_CLASS: calls=%d err=%v", assertR02, objects.calls, err)
	}
	_, err = Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID}, Dependencies{Objects: objects})
	if !IsCode(err, CodeIdentityMismatch) {
		t.Fatalf("%s_NEIGHBOR_BYTES: %v", assertR02, err)
	}
}
