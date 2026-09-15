package sessionruntime

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

type deriveStarter struct {
	calls int
	specs []managedprocess.Spec
}

type deriveChildStarter struct{ child Child }

func (s deriveChildStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func (s *deriveStarter) Start(_ context.Context, spec managedprocess.Spec) (Child, managedprocess.StartObservation) {
	s.calls++
	s.specs = append(s.specs, spec)
	return nil, managedprocess.StartObservation{Kind: managedprocess.StartUnavailable}
}

func TestDeriveWorkspaceRejectsNonReadyBeforeGitOrSpawn(t *testing.T) {
	const assertion = "ASSERT_DERIVE_PARENT_READY_BEFORE_GIT_OR_SPAWN"
	starter := &deriveStarter{}
	gitCalls := 0
	m, err := New(Config{Limits: Limits{MaxSessions: 2, MaxRequests: 1, MaxChildren: 2, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 8}, Starter: starter, GitWorktreeList: func(context.Context, string) ([]byte, error) { gitCalls++; return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "t", Workspace: "/repo/main", Profile: "p", EnvironmentReference: "e"})
	m.sessions["parent"] = &runtimeSession{record: Record{SessionID: "parent", Generation: 1, State: session.Initializing, Profile: runtimeprofile.Resolve(p)}, spec: managedprocess.Spec{Path: "/bin/lsp", Dir: "/repo/main"}}
	got := m.DeriveWorkspace(context.Background(), DeriveWorkspaceRequest{SessionID: "parent", Generation: 1, WorkspaceURI: "file:///repo/other"})
	if got.Failure != session.Failure("SESSION_NOT_READY") || gitCalls != 0 || starter.calls != 0 {
		t.Fatalf("%s: result=%+v git=%d spawn=%d", assertion, got, gitCalls, starter.calls)
	}
}

func TestCanonicalLocalWorkspaceURIIsStrict(t *testing.T) {
	const assertion = "ASSERT_DERIVE_WORKSPACE_URI_STRICT_LOCAL_CANONICAL_IDENTITY"
	real := stableTempDir(t)
	canonical := (&url.URL{Scheme: "file", Path: real}).String()
	if path, got, err := canonicalLocalWorkspaceURI(canonical); err != nil || path != real || got != canonical {
		t.Fatalf("%s: canonical result=(%q,%q,%v)", assertion, path, got, err)
	}
	for _, raw := range []string{"https://example.test/repo", canonical + "?x=1", canonical + "#x", "file://host" + real, canonical + "/.."} {
		if _, _, err := canonicalLocalWorkspaceURI(raw); err == nil {
			t.Errorf("%s: accepted %q", assertion, raw)
		}
	}
	link := filepath.Join(filepath.Dir(real), "workspace-link")
	if err := os.Symlink(real, link); err == nil {
		defer os.Remove(link)
		if _, _, err := canonicalLocalWorkspaceURI((&url.URL{Scheme: "file", Path: link}).String()); err == nil {
			t.Errorf("%s: accepted symlink identity", assertion)
		}
	}
}

func TestParseWorktreeListRejectsAmbiguousOrMalformedRecords(t *testing.T) {
	const assertion = "ASSERT_DERIVE_GIT_PORCELAIN_STRICT_CLOSED_PARSER"
	valid := []byte("worktree /repo/main\nHEAD abc\n\nworktree /repo/other\nHEAD def\n")
	if got, err := parseWorktreeList(valid); err != nil || !reflect.DeepEqual(got, []string{"/repo/main", "/repo/other"}) {
		t.Fatalf("%s: valid result=%v err=%v", assertion, got, err)
	}
	for _, raw := range [][]byte{nil, []byte("worktree /repo/main\nHEAD abc"), []byte("HEAD abc\nworktree /repo/main\n"), []byte("worktree relative\nHEAD abc\n"), []byte("worktree /repo/main/..\nHEAD abc\n"), []byte("worktree /repo/main\nHEAD a\n\nworktree /repo/main\nHEAD b\n")} {
		if _, err := parseWorktreeList(raw); err == nil {
			t.Errorf("%s: accepted %q", assertion, raw)
		}
	}
}

func TestDeriveWorkspaceAppliesOwnedGitDeadline(t *testing.T) {
	const assertion = "ASSERT_DERIVE_GIT_QUERY_HAS_OWNED_DEADLINE"
	parent, target := stableTempDir(t), stableTempDir(t)
	deadlineSeen := false
	m := newDeriveTestManager(t, &deriveStarter{}, parent, func(ctx context.Context, _ string) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		deadlineSeen = ok && time.Until(deadline) > 0 && time.Until(deadline) <= 5*time.Second
		return nil, context.DeadlineExceeded
	})
	got := m.DeriveWorkspace(context.Background(), DeriveWorkspaceRequest{SessionID: "parent", Generation: 1, WorkspaceURI: (&url.URL{Scheme: "file", Path: target}).String()})
	if got.Failure != session.Failure("GIT_WORKTREE_QUERY_FAILED") || !deadlineSeen {
		t.Fatalf("%s: result=%+v deadline_seen=%v", assertion, got, deadlineSeen)
	}
}

func TestParseWorktreeListRejectsRecordLimit(t *testing.T) {
	const assertion = "ASSERT_DERIVE_GIT_PORCELAIN_RECORD_LIMIT"
	var raw strings.Builder
	for i := 0; i < 1025; i++ {
		fmt.Fprintf(&raw, "worktree /repo/w%d\nHEAD %d\n\n", i, i)
	}
	if _, err := parseWorktreeList([]byte(raw.String())); err == nil {
		t.Fatal(assertion)
	}
}

func TestDeriveWorkspaceInheritsExactPrivateProcessSpecAndConfinesTarget(t *testing.T) {
	const assertion = "ASSERT_DERIVE_EXACT_PRIVATE_PARENT_LAUNCH_INHERITANCE"
	parent, target := stableTempDir(t), stableTempDir(t)
	starter := &deriveStarter{}
	m := newDeriveTestManager(t, starter, parent, func(context.Context, string) ([]byte, error) {
		return []byte(fmt.Sprintf("worktree %s\nHEAD a\n\nworktree %s\nHEAD b\n", parent, target)), nil
	})
	want := managedprocess.Spec{Path: "/private/lsp", Args: []string{"--private", "value"}, Dir: parent, Env: []string{"TOKEN=secret"}}
	m.sessions["parent"].spec = want
	got := m.DeriveWorkspace(context.Background(), DeriveWorkspaceRequest{SessionID: "parent", Generation: 1, WorkspaceURI: (&url.URL{Scheme: "file", Path: target}).String()})
	want.Dir = target
	if got.Failure == "" || starter.calls != 1 || !reflect.DeepEqual(starter.specs[0], want) {
		t.Fatalf("%s: result=%+v calls=%d spec=%+v want=%+v", assertion, got, starter.calls, starter.specs, want)
	}
}

func TestDeriveWorkspaceFailedReadinessReleasesDerivedSession(t *testing.T) {
	const assertion = "ASSERT_DERIVE_FAILED_READINESS_CLEANUP"
	parent, target := stableTempDir(t), stableTempDir(t)
	m := newDeriveTestManager(t, deriveChildStarter{child: newReadinessChild("error")}, parent, func(context.Context, string) ([]byte, error) {
		return []byte(fmt.Sprintf("worktree %s\nHEAD a\n\nworktree %s\nHEAD b\n", parent, target)), nil
	})
	got := m.DeriveWorkspace(context.Background(), DeriveWorkspaceRequest{SessionID: "parent", Generation: 1, WorkspaceURI: (&url.URL{Scheme: "file", Path: target}).String()})
	if got.Failure != session.InitializationFailure {
		t.Fatalf("%s: result=%+v", assertion, got)
	}
	records := m.Records()
	if len(records) != 1 || records[0].SessionID != "parent" {
		t.Fatalf("%s: records=%+v", assertion, records)
	}
}

func TestDeriveWorkspaceRechecksParentAfterGitBeforeSpawn(t *testing.T) {
	const assertion = "ASSERT_DERIVE_CONCURRENT_PARENT_CHANGE_REJECTED_BEFORE_SPAWN"
	parent, target := stableTempDir(t), stableTempDir(t)
	starter := &deriveStarter{}
	var m *Manager
	m = newDeriveTestManager(t, starter, parent, func(context.Context, string) ([]byte, error) {
		m.mu.Lock()
		m.sessions["parent"].record.State = session.Poisoned
		m.mu.Unlock()
		return []byte(fmt.Sprintf("worktree %s\nHEAD a\n\nworktree %s\nHEAD b\n", parent, target)), nil
	})
	got := m.DeriveWorkspace(context.Background(), DeriveWorkspaceRequest{SessionID: "parent", Generation: 1, WorkspaceURI: (&url.URL{Scheme: "file", Path: target}).String()})
	if got.Failure != session.LifecycleConflict || starter.calls != 0 {
		t.Fatalf("%s: result=%+v spawn=%d", assertion, got, starter.calls)
	}
}

func stableTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", ".derive-workspace-")
	if err != nil {
		t.Fatal(err)
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(absolute) })
	return absolute
}

func newDeriveTestManager(t *testing.T, starter Starter, parent string, gitList func(context.Context, string) ([]byte, error)) *Manager {
	t.Helper()
	m, err := New(Config{Limits: Limits{MaxSessions: 3, MaxRequests: 1, MaxChildren: 3, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16}, Starter: starter, GitWorktreeList: gitList})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "private-trust", Workspace: parent, Profile: "private-profile", EnvironmentReference: "private-env"})
	if err != nil {
		t.Fatal(err)
	}
	m.sessions["parent"] = &runtimeSession{record: Record{SessionID: "parent", Generation: 1, State: session.Ready, Profile: runtimeprofile.Resolve(profile)}, spec: managedprocess.Spec{Path: "/private/lsp", Dir: parent}, languageID: "go"}
	return m
}
