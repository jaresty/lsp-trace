package adr0011acquisition

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func TestRevisionIdentityTwoVerifiedPhases(t *testing.T) {
	observed, expected := syntheticHostGit(t, "BEFORE")
	root := privatePublicationRoot(t)
	streams := hostGitVerifiedStreams{}
	before, err := publishHostGit(root, streams, observed, expected, reviewedSuccessorSchemaDigest, nil)
	if err != nil || before.stage != "VERIFIED" {
		t.Fatalf("ASSERT_REVISION_BEFORE: %+v %v", before, err)
	}
	afterObservation := observed
	for i := range afterObservation.commands {
		afterObservation.commands[i].at = afterObservation.commands[i].at.Add(time.Minute)
	}
	afterExpectation := expected
	afterExpectation.Phase = "AFTER"
	after, err := publishHostGit(root, streams, afterObservation, afterExpectation, reviewedSuccessorSchemaDigest, nil)
	if err != nil || after.stage != "VERIFIED" {
		t.Fatalf("ASSERT_REVISION_AFTER: %+v %v", after, err)
	}
	state, err := publishRevisionIdentity(root, before, after, expected, reviewedSuccessorSchemaDigest, nil)
	if err != nil || state.stage != "VERIFIED" || !replayRevisionIdentity(root, state, before, after, expected, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		t.Fatalf("ASSERT_REVISION_VERIFIED: %+v %v", state, err)
	}
	b, err := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
	if err != nil || !strings.Contains(string(b), `"custody":"CALLER_ASSERTED"`) || !strings.Contains(string(b), before.selector) || !strings.Contains(string(b), after.selector) {
		t.Fatalf("ASSERT_REVISION_EXACT_REFS: %v", err)
	}
	if _, err := canonicalRevisionIdentity(revisionIdentityRecord{revisionIdentityRole, selectedGitFileURI(expected.Root), expected.Commit, "PROVIDER_VERIFIED", revisionHostRef(before), revisionHostRef(after)}); err == nil {
		t.Fatal("ASSERT_REVISION_CUSTODY_UPGRADE")
	}
	for name, check := range map[string]func() bool{
		"swapped-phase": func() bool {
			return replayRevisionIdentity(root, state, after, before, expected, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
		},
		"wrong-commit": func() bool {
			x := expected
			x.Commit = strings.Repeat("a", 40)
			return replayRevisionIdentity(root, state, before, after, x, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
		},
		"wrong-root": func() bool {
			x := expected
			x.Root = "/different"
			return replayRevisionIdentity(root, state, before, after, x, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
		},
		"wrong-schema": func() bool {
			return replayRevisionIdentity(root, state, before, after, expected, "sha256:"+strings.Repeat("0", 64), publication.ReadVerifiedBoundFile)
		},
		"unverified-before": func() bool {
			x := before
			x.stage = "COMMITTED_UNVERIFIED"
			return replayRevisionIdentity(root, state, x, after, expected, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
		},
		"unverified-after": func() bool {
			x := after
			x.stage = "COMMITTED_UNVERIFIED"
			return replayRevisionIdentity(root, state, before, x, expected, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
		},
		"unverified-revision": func() bool {
			x := state
			x.stage = "COMMITTED_UNVERIFIED"
			return replayRevisionIdentity(root, x, before, after, expected, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
		},
		"changed-ref": func() bool {
			x := after
			x.digest = privateDigest([]byte("wrong"))
			return replayRevisionIdentity(root, state, before, x, expected, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
		},
		"missing-empty-stream": func() bool {
			read := func(rt *publication.Root, selector string, n int64) ([]byte, error) {
				if selector == hostGitStreamRef(nil).Selector {
					return nil, os.ErrNotExist
				}
				return publication.ReadVerifiedBoundFile(rt, selector, n)
			}
			return replayRevisionIdentity(root, state, before, after, expected, reviewedSuccessorSchemaDigest, read)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if check() {
				t.Fatal("ASSERT_REVISION_SUBSTITUTION_REJECT")
			}
		})
	}
	if collision, err := publishRevisionIdentity(root, before, after, expected, reviewedSuccessorSchemaDigest, nil); err == nil || collision.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_REVISION_NO_REPLACE: %+v %v", collision, err)
	}
	failedRead := func(*publication.Root, string, int64) ([]byte, error) { return nil, errors.New("injected") }
	if failed, err := publishRevisionIdentity(privatePublicationRoot(t), before, after, expected, reviewedSuccessorSchemaDigest, failedRead); err == nil || failed.stage != "ABSENT" {
		t.Fatalf("ASSERT_REVISION_PRECOMMIT_READ: %+v %v", failed, err)
	}
	if err := os.WriteFile(expected.Executable.path, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if replayRevisionIdentity(root, state, before, after, expected, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		t.Fatal("ASSERT_REVISION_EXECUTABLE_SUBSTITUTION")
	}
}

func TestRevisionIdentityRejectsReversedPhaseTimes(t *testing.T) {
	o, x := syntheticHostGit(t, "BEFORE")
	root := privatePublicationRoot(t)
	streams := hostGitVerifiedStreams{}
	before, err := publishHostGit(root, streams, o, x, reviewedSuccessorSchemaDigest, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range o.commands {
		o.commands[i].at = o.commands[i].at.Add(-time.Hour)
	}
	afterX := x
	afterX.Phase = "AFTER"
	after, err := publishHostGit(root, streams, o, afterX, reviewedSuccessorSchemaDigest, nil)
	if err != nil {
		t.Fatal(err)
	}
	state, err := publishRevisionIdentity(root, before, after, x, reviewedSuccessorSchemaDigest, nil)
	if err == nil || state.stage != "ABSENT" {
		t.Fatalf("ASSERT_REVISION_PHASE_ORDER: %+v %v", state, err)
	}
}
