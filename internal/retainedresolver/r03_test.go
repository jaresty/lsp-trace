package retainedresolver

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedlifecycle"
	"lsp-trace/internal/retainedmanifest"
	"lsp-trace/internal/sourceobject"
)

const assertR03 = "ASSERT_R03_NO_HISTORICAL_FALLBACK_LIFECYCLE"

type fixedLifecycle struct {
	state retainedlifecycle.State
	calls int
}

func (f *fixedLifecycle) State(retainedlifecycle.Key) retainedlifecycle.State {
	f.calls++
	return f.state
}

func TestR03ManifestTerminalStatesDoNotLookup(t *testing.T) {
	body := []byte("retained bytes")
	id := identity(body)
	for _, tc := range []struct {
		availability string
		class        string
		custody      string
		code         Code
	}{
		{"WITHHELD", "CONTENT_ADDRESS", ContentCustodyIdentity("CONTENT_ADDRESS", id), CodeWithheld},
		{"COLLECTED", "CONTENT_ADDRESS", ContentCustodyIdentity("CONTENT_ADDRESS", id), CodeCollected},
		{"UNAVAILABLE", "UNAVAILABLE", UnavailableCustodyIdentity(id), CodeUnavailable},
	} {
		t.Run(tc.availability, func(t *testing.T) {
			entry := entryFor(id, tc.class, tc.availability, tc.custody)
			raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
			objects, gitLookup := &countingObjects{}, &countingGit{}
			beforeManifest, beforeGraph := append([]byte(nil), raw...), append([]byte(nil), graphBytes...)
			_, err := Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID}, Dependencies{Git: gitLookup, Objects: objects})
			if !IsCode(err, tc.code) || IsCode(err, CodeLimit) || objects.calls != 0 || gitLookup.calls != 0 {
				t.Fatalf("%s_%s: objects=%d git=%d err=%v", assertR03, tc.availability, objects.calls, gitLookup.calls, err)
			}
			if !bytes.Equal(raw, beforeManifest) || !bytes.Equal(graphBytes, beforeGraph) {
				t.Fatalf("%s_%s_CUSTODY_MUTATED", assertR03, tc.availability)
			}
		})
	}
}

func TestR03TypedObjectFailuresRemainDistinctFromLimit(t *testing.T) {
	body := []byte("retained bytes")
	id := identity(body)
	entry := entryFor(id, "CONTENT_ADDRESS", "AVAILABLE", ContentCustodyIdentity("CONTENT_ADDRESS", id))
	raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
	for _, tc := range []struct {
		name       string
		sourceCode sourceobject.Code
		want       Code
	}{
		{"missing", sourceobject.CodeMissing, CodeMissing},
		{"corrupt", sourceobject.CodeCorrupt, CodeCorrupt},
		{"limit", sourceobject.CodeLimit, CodeLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			objects := &countingObjects{objects: map[sourceobject.Identity]sourceobject.Object{}}
			objects.objects = nil
			lookup := objectErrorLookup{err: &sourceobject.Error{Code: tc.sourceCode}}
			_, err := Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID}, Dependencies{Objects: &lookup})
			if !IsCode(err, tc.want) || (tc.want != CodeLimit && IsCode(err, CodeLimit)) || lookup.calls != 1 {
				t.Fatalf("%s_%s: calls=%d err=%v", assertR03, tc.name, lookup.calls, err)
			}
			_ = objects
		})
	}
}

type objectErrorLookup struct {
	err   error
	calls int
}

func (o *objectErrorLookup) Get(sourceobject.Identity) (sourceobject.Object, error) {
	o.calls++
	return sourceobject.Object{}, o.err
}

func TestR03RealSourceObjectDeletionAndCorruptionAreExact(t *testing.T) {
	body := []byte("retained physical bytes")
	id := identity(body)
	entry := entryFor(id, "CONTENT_ADDRESS", "AVAILABLE", ContentCustodyIdentity("CONTENT_ADDRESS", id))
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
	if published, err := objects.Publish(body); err != nil || published != id {
		t.Fatalf("publish=%+v err=%v", published, err)
	}
	selector := filepath.Join(rootDir, strings.TrimPrefix(id.Digest, "sha256:"))
	if err := os.Remove(selector); err != nil {
		t.Fatal(err)
	}
	request := Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID}
	if _, err := Resolve(raw, graphBytes, capture, request, Dependencies{Objects: objects}); !IsCode(err, CodeMissing) || IsCode(err, CodeLimit) {
		t.Fatalf("%s_REAL_MISSING: %v", assertR03, err)
	}
	if _, err := objects.Publish(body); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(selector, []byte("corrupt envelope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(raw, graphBytes, capture, request, Dependencies{Objects: objects}); !IsCode(err, CodeCorrupt) || IsCode(err, CodeLimit) {
		t.Fatalf("%s_REAL_CORRUPT: %v", assertR03, err)
	}
}

func TestR03ExactLifecycleStatesTerminateBeforeResolver(t *testing.T) {
	root, binding, committed := realGitFixture(t)
	_ = root
	id := identity(committed)
	entry := entryFor(id, "GIT_BLOB", "AVAILABLE", GitCustodyIdentity(binding))
	raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
	for _, tc := range []struct {
		state retainedlifecycle.State
		code  Code
	}{
		{retainedlifecycle.StateShallowHistory, CodeShallowHistory},
		{retainedlifecycle.StateRewrittenHistory, CodeRewrittenHistory},
		{retainedlifecycle.StateGCCollected, CodeGCCollected},
		{retainedlifecycle.StateDeleted, CodeDeleted},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			lifecycle := &fixedLifecycle{state: tc.state}
			gitLookup := &countingGit{err: errors.New("must not be called")}
			_, err := Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID, Git: &binding}, Dependencies{Git: gitLookup, Lifecycle: lifecycle})
			if !IsCode(err, tc.code) || IsCode(err, CodeLimit) || lifecycle.calls != 1 || gitLookup.calls != 0 {
				t.Fatalf("%s_%s: lifecycle=%d git=%d err=%v", assertR03, tc.state, lifecycle.calls, gitLookup.calls, err)
			}
		})
	}
}

func TestR03UnknownLifecycleDoesNotRelabelGenericFailure(t *testing.T) {
	root, binding, committed := realGitFixture(t)
	_ = root
	id := identity(committed)
	entry := entryFor(id, "GIT_BLOB", "AVAILABLE", GitCustodyIdentity(binding))
	raw, graphBytes, capture, manifestID, manifest := manifestFixture(t, []retainedmanifest.EntryInput{entry})
	lifecycle := &fixedLifecycle{state: retainedlifecycle.StateUnknown}
	gitLookup := &countingGit{err: errors.New("generic Git failure")}
	_, err := Resolve(raw, graphBytes, capture, Request{ManifestDigest: manifestID, EntryID: manifest.Entries[0].EntryID, Git: &binding}, Dependencies{Git: gitLookup, Lifecycle: lifecycle})
	if !IsCode(err, CodeResolutionFailed) || lifecycle.calls != 1 || gitLookup.calls != 1 {
		t.Fatalf("%s_UNKNOWN: lifecycle=%d git=%d err=%v", assertR03, lifecycle.calls, gitLookup.calls, err)
	}
}
