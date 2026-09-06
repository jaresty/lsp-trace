package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/observationadapter"
)

type managedGitSessionFixture struct {
	SessionID      string
	Generation     uint64
	RepositoryRoot string
	Commit         string
	OriginalPath   string
	OriginalURI    string
	ContentSHA256  string
	Blob           string
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newManagedGitSessionFixture(t *testing.T) managedGitSessionFixture {
	t.Helper()
	root := t.TempDir()
	gitOutput(t, root, "init", "-q")
	gitOutput(t, root, "config", "user.name", "custody fixture")
	gitOutput(t, root, "config", "user.email", "custody@example.invalid")
	path := filepath.Join(root, "component.gts")
	content := []byte("<Widget @value={{this.itemCount}} />\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "component.gts")
	gitOutput(t, root, "commit", "-qm", "pinned source")
	sum := sha256.Sum256(content)
	return managedGitSessionFixture{
		SessionID:      "managed-git-session",
		Generation:     7,
		RepositoryRoot: root,
		Commit:         gitOutput(t, root, "rev-parse", "HEAD"),
		OriginalPath:   path,
		OriginalURI:    (&url.URL{Scheme: "file", Path: path}).String(),
		ContentSHA256:  hex.EncodeToString(sum[:]),
		Blob:           hex.EncodeToString(sum[:]),
	}
}

func productionSemanticEnvelope(t *testing.T) observationadapter.Envelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "qualification", "retained", "external-provider", "ember-glint.json"))
	if err != nil {
		t.Fatal(err)
	}
	var retained struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(raw, &retained); err != nil {
		t.Fatal(err)
	}
	response, err := base64.StdEncoding.DecodeString(retained.Response)
	if err != nil {
		t.Fatal(err)
	}
	var envelope observationadapter.Envelope
	if err := json.Unmarshal(response, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func revisionBoundSemanticCase(t *testing.T, session managedGitSessionFixture) (*ObservationSemanticAdapter, StrictCollectorRequest, Receipt, observationadapter.Envelope) {
	t.Helper()
	declaration := admissionDeclaration("ember-glint@1", "BINDS_ARGUMENT")
	provisioned := mustProvisionAdmission(t, declaration)
	adapterIdentity := observationadapter.Identity{Name: "ember-template", Version: "1"}
	verifier := NewGitRevisionVerifier([]ManagedGitRevision{{SessionID: "managed-git-session", Generation: session.Generation, RepositoryRoot: session.RepositoryRoot, Commit: session.Commit}})
	adapter, err := NewObservationSemanticAdapter(provisioned, adapterIdentity, verifier)
	if err != nil {
		t.Fatal(err)
	}
	envelope := productionSemanticEnvelope(t)
	envelope.RequestID = session.SessionID + ":" + "7" + ":" + session.OriginalURI
	envelope.Coverage.Denominator = []string{session.OriginalURI}
	envelope.Coverage.Covered = []string{session.OriginalURI}
	envelope.Documents[0].OriginalURI = session.OriginalURI
	envelope.Documents[0].ContentSHA256 = session.ContentSHA256
	envelope.Documents[0].Revision = graph.RevisionIdentity{Kind: "git", Value: session.Commit, Blob: session.Blob, Custody: graph.CustodyProviderProved}
	envelope.Observations[0].OriginalAnchor.URI = session.OriginalURI
	envelope.Observations[0].OriginalAnchor.Revision = session.Commit
	envelope.Observations[0].OriginalAnchor.Blob = session.Blob
	response, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	workspaceRevision, err := json.Marshal(graph.RevisionIdentity{Kind: "git", Value: session.Commit, Custody: graph.CustodyCallerAsserted})
	if err != nil {
		t.Fatal(err)
	}
	request := StrictCollectorRequest{
		SchemaVersion: CollectorRequestSchema,
		ProviderID:    declaration.Identity,
		AdapterID:     "ember-template@1",
		Session:       ManagedSessionCustody{SessionID: session.SessionID, Generation: session.Generation},
		Seed:          SeedCustody{URI: session.OriginalURI, StartMode: "at"},
		Relations:     []string{"BINDS_ARGUMENT"},
		Documents:     DocumentCustody{OriginalURI: session.OriginalURI, WorkspaceRevision: workspaceRevision, FailOnUnknown: true},
		Limits:        CollectorLimits{MaxNodes: 100, MaxMessages: 1, MaxBytes: 1 << 20, TimeoutMS: 30000, RequestTimeoutMS: 30000},
	}
	return adapter, request, Receipt{ProviderID: declaration.Identity, Response: response, Messages: 1, Reaped: true}, envelope
}

func TestRevisionCustodyAuthorityLabel(t *testing.T) {
	for _, tc := range []struct {
		name      string
		workspace graph.RevisionIdentity
		strict    bool
		want      graph.RevisionCustody
	}{
		{"caller assertion", graph.RevisionIdentity{Kind: "git", Custody: graph.CustodyCallerAsserted}, false, graph.CustodyCallerAsserted},
		{"verified Git", graph.RevisionIdentity{Kind: "git", Custody: graph.CustodyCallerAsserted}, true, graph.CustodyProviderProved},
		{"unknown", graph.RevisionIdentity{Kind: "git", Custody: graph.CustodyUnknown}, false, graph.CustodyUnknown},
		{"mock self asserted proof", graph.RevisionIdentity{Kind: "git", Custody: graph.CustodyProviderProved}, false, graph.CustodyUnknown},
		{"no revision", graph.RevisionIdentity{}, false, graph.CustodyUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := newManagedGitSessionFixture(t)
			adapter, request, receipt, envelope := revisionBoundSemanticCase(t, session)
			request.Documents.FailOnUnknown = tc.strict
			if tc.workspace.Kind != "" {
				tc.workspace.Value = session.Commit
			}
			request.Documents.WorkspaceRevision, _ = json.Marshal(tc.workspace)
			// A legacy/mocked provider's self-asserted proof cannot bypass the host boundary.
			if tc.strict {
				envelope.Documents[0].Revision.Custody = graph.CustodyCallerAsserted
			}
			receipt.Response, _ = json.Marshal(envelope)
			raw, err := adapter.Adapt(context.Background(), request, receipt)
			if err != nil {
				t.Fatal(err)
			}
			var result Result
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if got := result.Custody.Documents[0].Revision.Custody; got != tc.want {
				t.Fatalf("ASSERT_REVISION_CUSTODY_HOST_AUTHORITY: got %s want %s", got, tc.want)
			}
			var published graph.DocumentCustodyReceipt
			if err := json.Unmarshal(result.GraphV4.Provenance.Custody, &published); err != nil {
				t.Fatal(err)
			}
			if published.Documents[0].Revision.Custody != tc.want {
				t.Fatal("published custody differs")
			}
			if result.Observations[0].OriginalAnchor != envelope.Observations[0].OriginalAnchor {
				t.Fatal("source anchor changed")
			}
			if result.Observations[0].From.NodeID != envelope.Observations[0].From.NodeID || result.Observations[0].To.NodeID != envelope.Observations[0].To.NodeID {
				t.Fatal("endpoint identity changed")
			}
		})
	}
}

func TestRevisionBoundCustodyComposition(t *testing.T) {
	t.Run("unknown provider assertion rejected by strict verifier", func(t *testing.T) {
		session := newManagedGitSessionFixture(t)
		adapter, request, receipt, envelope := revisionBoundSemanticCase(t, session)
		envelope.Documents[0].Revision.Custody = graph.CustodyUnknown
		receipt.Response, _ = json.Marshal(envelope)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal("unknown provider assertion passed strict verifier")
		}
	})
	t.Run("invalid provider authority rejected before normalization", func(t *testing.T) {
		session := newManagedGitSessionFixture(t)
		adapter, request, receipt, envelope := revisionBoundSemanticCase(t, session)
		request.Documents.FailOnUnknown = false
		envelope.Documents[0].Revision.Custody = "FORGED"
		receipt.Response, _ = json.Marshal(envelope)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal("invalid authority normalized into legal custody")
		}
	})
	t.Run("additional document has no host proof", func(t *testing.T) {
		session := newManagedGitSessionFixture(t)
		adapter, request, _, envelope := revisionBoundSemanticCase(t, session)
		extra := envelope.Documents[0]
		extra.DocumentID = "extra"
		extra.OriginalURI += ".extra"
		envelope.Documents = append(envelope.Documents, extra)
		if err := validateSemanticCustody(context.Background(), adapter.verifier, request, envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Documents[0].Revision.Custody != graph.CustodyProviderProved || envelope.Documents[1].Revision.Custody != graph.CustodyUnknown {
			t.Fatal("proof escaped matching document")
		}
	})
	t.Run("matching committed blob", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_MATCHING_COMMITTED_BLOB_BINDS"
		session := newManagedGitSessionFixture(t)
		adapter, request, receipt, _ := revisionBoundSemanticCase(t, session)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err != nil {
			t.Fatalf("%s: %v", assertion, err)
		}
		t.Log("PASS " + assertion)
	})

	t.Run("dirty worktree uses historical blob", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_DIRTY_WORKTREE_USES_PINNED_GIT_OBJECT"
		session := newManagedGitSessionFixture(t)
		adapter, request, receipt, _ := revisionBoundSemanticCase(t, session)
		if err := os.WriteFile(session.OriginalPath, []byte("dirty worktree bytes\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := adapter.Adapt(context.Background(), request, receipt); err != nil {
			t.Fatalf("%s: pinned commit rejected because worktree is dirty: %v", assertion, err)
		}
		t.Log("PASS " + assertion)
	})

	t.Run("commit mismatch", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_COMMIT_MISMATCH_REJECTED"
		session := newManagedGitSessionFixture(t)
		adapter, request, receipt, envelope := revisionBoundSemanticCase(t, session)
		envelope.Documents[0].Revision.Value = strings.Repeat("d", 40)
		receipt.Response, _ = json.Marshal(envelope)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal(assertion + ": accepted provider commit mismatch")
		}
		t.Log("PASS " + assertion)
	})

	t.Run("content mismatch", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_CONTENT_SHA256_MISMATCH_REJECTED"
		session := newManagedGitSessionFixture(t)
		adapter, request, receipt, envelope := revisionBoundSemanticCase(t, session)
		envelope.Documents[0].ContentSHA256 = strings.Repeat("a", 64)
		receipt.Response, _ = json.Marshal(envelope)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal(assertion + ": accepted provider content digest that does not match pinned blob bytes")
		}
		t.Log("PASS " + assertion)
	})

	t.Run("path outside repository", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_PATH_OUTSIDE_SESSION_REPOSITORY_REJECTED"
		session := newManagedGitSessionFixture(t)
		outside := filepath.Join(t.TempDir(), "outside.gts")
		if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		session.OriginalPath = outside
		session.OriginalURI = (&url.URL{Scheme: "file", Path: outside}).String()
		adapter, request, receipt, _ := revisionBoundSemanticCase(t, session)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal(assertion + ": accepted original path outside authenticated session repository")
		}
		t.Log("PASS " + assertion)
	})

	t.Run("missing git repository", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_MISSING_GIT_REJECTED"
		session := newManagedGitSessionFixture(t)
		nonRepository := t.TempDir()
		session.RepositoryRoot = nonRepository
		session.OriginalPath = filepath.Join(nonRepository, "component.gts")
		if err := os.WriteFile(session.OriginalPath, []byte("not in git\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		session.OriginalURI = (&url.URL{Scheme: "file", Path: session.OriginalPath}).String()
		adapter, request, receipt, _ := revisionBoundSemanticCase(t, session)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal(assertion + ": accepted managed session without a Git repository")
		}
		t.Log("PASS " + assertion)
	})

	t.Run("unavailable commit object", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_UNAVAILABLE_GIT_OBJECT_REJECTED"
		session := newManagedGitSessionFixture(t)
		session.Commit = strings.Repeat("f", 40)
		adapter, request, receipt, _ := revisionBoundSemanticCase(t, session)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal(assertion + ": accepted commit absent from authenticated repository object database")
		}
		t.Log("PASS " + assertion)
	})

	t.Run("caller asserted request alone", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_CALLER_ASSERTED_ALONE_NEVER_UPGRADES"
		session := newManagedGitSessionFixture(t)
		session.SessionID = "unmanaged-caller-claim"
		adapter, request, receipt, _ := revisionBoundSemanticCase(t, session)
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal(assertion + ": accepted caller assertion without authenticated managed-session metadata")
		}
		t.Log("PASS " + assertion)
	})

	t.Run("provider cannot assert request git authority", func(t *testing.T) {
		const assertion = "ASSERT_REVISION_CUSTODY_PROVIDER_GIT_IDENTITY_NEVER_UPGRADES_REQUEST"
		session := newManagedGitSessionFixture(t)
		adapter, request, receipt, _ := revisionBoundSemanticCase(t, session)
		request.Documents.WorkspaceRevision, _ = json.Marshal(graph.RevisionIdentity{Kind: "git", Value: session.Commit, Custody: graph.CustodyProviderProved})
		if _, err := adapter.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal(assertion + ": accepted provider-proved request Git identity without host proof")
		}
		t.Log("PASS " + assertion)
	})
}
