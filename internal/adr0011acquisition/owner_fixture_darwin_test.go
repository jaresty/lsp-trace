//go:build darwin

package adr0011acquisition

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/adr0011c18"
	"lsp-trace/internal/adr0011work"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type ownerAcquisitionFixture struct {
	Context      context.Context
	Owner        *Owner
	Query        Query
	Manager      *sessionruntime.Manager
	Root         *publication.Root
	Facts        preinvokeFacts
	Selector     string
	Digest       string
	OccurrenceID string
	Workspace    string
	Git          func(...string) string
}

func newOwnerAcquisitionFixture(t *testing.T) *ownerAcquisitionFixture {
	t.Helper()
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(workspace, "query.go")
	if err := os.WriteFile(file, []byte("package fixture\nfunc Query() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = workspace
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "add", "query.go")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	commit := strings.TrimSpace(git("rev-parse", "HEAD"))
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 16}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Shutdown(context.Background()) })
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "gopls", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), LanguageID: "go", Process: managedprocess.Spec{Path: executable, Args: []string{"-test.run=^TestADR0011AcquisitionPeer$"}, Env: append(os.Environ(), "ADR0011_ACQUISITION_PEER=1")}})
	if started.Failure != "" {
		t.Fatalf("fixture start: %+v", started)
	}
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(10*time.Second))
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
		t.Fatalf("fixture readiness: %+v found=%v", ready, found)
	}
	dir := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	canonical, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if observed, probeErr := observeHostGit(context.Background(), canonical, commit); probeErr != nil {
		t.Fatalf("ASSERT_ADR0011_HOST_GIT_CLEAN_PROBE: err=%v observation=%+v", probeErr, observed)
	}
	prepared := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
	if prepared.Failure != "" || prepared.Supply == nil {
		t.Fatalf("preinvoke prepare: %+v", prepared)
	}
	selected, err := selectGitExecutable("")
	if err != nil {
		t.Fatal(err)
	}
	facts := preinvokeFacts{SessionID: started.SessionID, Generation: started.Generation, Workspace: canonical, URI: uri, Line: 1, Character: 6, Version: prepared.Version, Source: append([]byte(nil), prepared.Supply.Content...), SourceDigest: privateDigest(prepared.Supply.Content), SourceLength: len(prepared.Supply.Content), GitRoot: canonical, GitCommit: commit, Executable: selected}
	repeated := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
	if repeated.Failure != "" || repeated.Version != prepared.Version || repeated.Supply == nil || privateDigest(repeated.Supply.Content) != facts.SourceDigest {
		t.Fatalf("ASSERT_ADR0011_PREPARE_REPEAT_STABLE: first=%d second=%d failure=%s", prepared.Version, repeated.Version, repeated.Failure)
	}
	selector, digest, occurrenceID, err := publishPreinvokeOccurrence(root, facts)
	if err != nil {
		t.Fatal(err)
	}
	owner := newTestOwner(manager, root)
	owner.preinvokeSelector, owner.preinvokeDigest = selector, digest
	owner.captureFrames = true
	query := Query{SessionID: started.SessionID, Generation: started.Generation, URI: uri, Line: 1, Character: 6, OccurrenceID: occurrenceID, Revision: commit}
	return &ownerAcquisitionFixture{Context: context.Background(), Owner: owner, Query: query, Manager: manager, Root: root, Facts: facts, Selector: selector, Digest: digest, OccurrenceID: occurrenceID, Workspace: workspace, Git: git}
}

func TestOwnerAcquisitionFixtureContract(t *testing.T) {
	fixture := newOwnerAcquisitionFixture(t)
	if fixture.Owner == nil || fixture.Query.OccurrenceID == "" || fixture.Context == nil {
		t.Fatalf("ASSERT_OWNER_FIXTURE_SURFACE: owner=%p occurrence=%q context_nil=%t", fixture.Owner, fixture.Query.OccurrenceID, fixture.Context == nil)
	}
	account := adr0011work.New(fixture.Query.OccurrenceID)
	ctx := adr0011work.WithAccount(fixture.Context, account)
	accountObserved := false
	ctx = adr0011c18.WithObserver(ctx, func(point adr0011c18.Point) {
		if point != adr0011c18.PointAcquisitionEntered {
			return
		}
		observed, ok := adr0011work.AccountFromContext(ctx)
		accountObserved = ok && observed == account
	})
	fixture.Owner.finalImplementationPin = reviewedSyntheticSourcePin
	receipt, err := fixture.Owner.acquirePrivateFinal(ctx, fixture.Query)
	if err != nil || receipt == nil || receipt.T != 1 || receipt.A != 2 || len(receipt.OrdinalRefs) != 2 {
		t.Fatalf("ASSERT_OWNER_FIXTURE_BASELINE_FINAL_ORDINALS: receipt=%+v err=%v", receipt, err)
	}
	if !accountObserved {
		t.Fatal("ASSERT_OWNER_FIXTURE_CALLER_ACCOUNT_CONTEXT: account was not observable on Owner entry")
	}
}
