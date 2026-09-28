package adr0011acquisition

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func syntheticHostGit(t *testing.T, phase string) (selectedGitObservation, hostGitExpectation) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	executable := makeSelectedGit(t, root, "#!/bin/sh\nexit 0\n")
	x := hostGitExpectation{executable, root, testSelectedCommit, phase}
	o := selectedGitObservation{executable: executable, rootURI: selectedGitFileURI(root), cwdURI: selectedGitFileURI(root), root: root, commit: testSelectedCommit}
	out := [3]string{root + "\n", testSelectedCommit + "\n", ""}
	for i := range o.commands {
		o.commands[i] = selectedGitCommand{argv: append([]string(nil), hostGitArgv[i]...), stdout: []byte(out[i]), stderr: []byte{}, exit: 0, at: time.Date(2026, 1, 2, 3, 4, i, 0, time.UTC)}
	}
	return o, x
}
func TestHostGitRetainedSynthetic(t *testing.T) {
	for _, phase := range []string{"BEFORE", "AFTER"} {
		t.Run(phase, func(t *testing.T) {
			o, x := syntheticHostGit(t, phase)
			root := privatePublicationRoot(t)
			verified := hostGitVerifiedStreams{}
			state, e := publishHostGit(root, verified, o, x, reviewedSuccessorSchemaDigest, nil)
			if e != nil || state.stage != "VERIFIED" {
				t.Fatalf("ASSERT_HOST_GIT_VERIFIED: %+v %v", state, e)
			}
			if !replayHostGit(root, state, x, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
				t.Fatal("ASSERT_HOST_GIT_REPLAY")
			}
			b, e := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
			if e != nil {
				t.Fatal(e)
			}
			r, ok := decodeHostGit(b)
			if !ok {
				t.Fatal("record decode")
			}
			if len(r.Commands) != 3 || r.Commands[2].Stdout.Selector != r.Commands[0].Stderr.Selector || r.Commands[0].Stderr.Selector != r.Commands[1].Stderr.Selector {
				t.Fatal("empty stream not deduplicated")
			}
			if empty, e := publication.ReadVerifiedBoundFile(root, r.Commands[2].Stdout.Selector, 1); e != nil || empty == nil || len(empty) != 0 {
				t.Fatalf("missing empty object: %v %v", empty, e)
			}
			for name, mutate := range map[string]func(*hostGitExpectation){"phase": func(v *hostGitExpectation) {
				v.Phase = "AFTER"
				if phase == "AFTER" {
					v.Phase = "BEFORE"
				}
			}, "root": func(v *hostGitExpectation) { v.Root = "/wrong" }, "commit": func(v *hostGitExpectation) { v.Commit = strings.Repeat("a", 40) }, "exec": func(v *hostGitExpectation) { v.Executable.digest = "sha256:" + strings.Repeat("0", 64) }} {
				t.Run(name, func(t *testing.T) {
					other := x
					mutate(&other)
					if replayHostGit(root, state, other, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
						t.Fatal("ASSERT_HOST_GIT_EXPECTATION_REJECT")
					}
				})
			}
			if replayHostGit(root, state, x, "sha256:"+strings.Repeat("0", 64), publication.ReadVerifiedBoundFile) {
				t.Fatal("schema substitution")
			}
			read := func(rt *publication.Root, selector string, limit int64) ([]byte, error) {
				if selector == r.Commands[0].Stderr.Selector {
					return nil, errors.New("missing empty stream")
				}
				return publication.ReadVerifiedBoundFile(rt, selector, limit)
			}
			if replayHostGit(root, state, x, reviewedSuccessorSchemaDigest, read) {
				t.Fatal("missing empty stream accepted")
			}
			read = func(rt *publication.Root, selector string, limit int64) ([]byte, error) {
				b, e := publication.ReadVerifiedBoundFile(rt, selector, limit)
				if selector == r.Commands[0].Stdout.Selector {
					return []byte("/substituted\n"), nil
				}
				return b, e
			}
			if replayHostGit(root, state, x, reviewedSuccessorSchemaDigest, read) {
				t.Fatal("stream substitution accepted")
			}
			read = func(rt *publication.Root, selector string, limit int64) ([]byte, error) {
				if selector == state.selector {
					return []byte(`{}`), nil
				}
				return publication.ReadVerifiedBoundFile(rt, selector, limit)
			}
			if replayHostGit(root, state, x, reviewedSuccessorSchemaDigest, read) {
				t.Fatal("record substitution accepted")
			}
			if _, e := publishHostGit(root, nil, o, x, reviewedSuccessorSchemaDigest, nil); e == nil {
				t.Fatal("no-replace collision accepted")
			}
		})
	}
}
func TestHostGitSharedStreamsAcrossPhases(t *testing.T) {
	o, before := syntheticHostGit(t, "BEFORE")
	root := privatePublicationRoot(t)
	verified := hostGitVerifiedStreams{}
	first, err := publishHostGit(root, verified, o, before, reviewedSuccessorSchemaDigest, nil)
	if err != nil || first.stage != "VERIFIED" || len(verified) != 3 {
		t.Fatalf("before: %+v %v, streams=%d", first, err, len(verified))
	}
	after := before
	after.Phase = "AFTER"
	second, err := publishHostGit(root, verified, o, after, reviewedSuccessorSchemaDigest, nil)
	if err != nil || second.stage != "VERIFIED" || second.selector == first.selector {
		t.Fatalf("after: %+v %v", second, err)
	}
	if !replayHostGit(root, first, before, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) || !replayHostGit(root, second, after, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		t.Fatal("distinct phase replay")
	}
	firstStream := hostGitStreamRef(o.commands[0].stdout).Selector
	entry := verified[firstStream]
	forged := entry
	forged.receipt.CloseStatus = publication.CloseFailed
	verified[firstStream] = forged
	unverified, reuseErr := publishHostGit(root, verified, o, after, reviewedSuccessorSchemaDigest, nil)
	if reuseErr == nil || unverified.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("invalid prior receipt reused: %+v %v", unverified, reuseErr)
	}
	verified[firstStream] = entry
	lost, err := publishHostGit(root, hostGitVerifiedStreams{}, o, after, reviewedSuccessorSchemaDigest, nil)
	if err == nil || lost.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("lost map: %+v %v", lost, err)
	}
	if replayHostGit(root, second, before, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) || replayHostGit(root, privateBodyPublication{selector: second.selector, digest: second.digest, byteCount: second.byteCount, stage: "COMMITTED_UNVERIFIED"}, after, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		t.Fatal("phase or stage substitution")
	}
	corrupt := func(rt *publication.Root, selector string, limit int64) ([]byte, error) {
		if selector == hostGitStreamRef(o.commands[0].stdout).Selector {
			return []byte("corrupt"), nil
		}
		return publication.ReadVerifiedBoundFile(rt, selector, limit)
	}
	state, err := publishHostGit(root, verified, o, after, reviewedSuccessorSchemaDigest, corrupt)
	if err == nil || state.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("corrupt shared stream: %+v %v", state, err)
	}
	if _, ok := verified[hostGitStreamRef(o.commands[0].stdout).Selector]; ok {
		t.Fatal("corrupt entry remained verified")
	}
}

func TestHostGitFailureAfterFirstStreamCommit(t *testing.T) {
	o, x := syntheticHostGit(t, "BEFORE")
	root := privatePublicationRoot(t)
	verified := hostGitVerifiedStreams{}
	second := hostGitStreamRef(o.commands[0].stderr).Selector
	read := func(rt *publication.Root, selector string, limit int64) ([]byte, error) {
		if selector == second {
			return nil, os.ErrPermission
		}
		return publication.ReadVerifiedBoundFile(rt, selector, limit)
	}
	state, err := publishHostGit(root, verified, o, x, reviewedSuccessorSchemaDigest, read)
	if err == nil || state.stage != "COMMITTED_UNVERIFIED" || state.selector != second {
		t.Fatalf("last possibly committed stream: %+v %v", state, err)
	}
	if len(verified) != 1 {
		t.Fatalf("only first stream verified: %d", len(verified))
	}
}

func TestHostGitRetainedRejectsSyntheticMutations(t *testing.T) {
	cases := map[string]func(*selectedGitObservation){
		"cwd": func(o *selectedGitObservation) { o.cwdURI = "file:///wrong" }, "argv": func(o *selectedGitObservation) { o.commands[1].argv[2] = "other" }, "exit": func(o *selectedGitObservation) { o.commands[0].exit = 1 }, "timestamp": func(o *selectedGitObservation) { o.commands[2].at = o.commands[0].at.Add(-time.Second) }, "root stdout": func(o *selectedGitObservation) { o.commands[0].stdout = append(o.commands[0].stdout, '\n') }, "status": func(o *selectedGitObservation) { o.commands[2].stdout = []byte("dirty") }, "stderr": func(o *selectedGitObservation) { o.commands[1].stderr = []byte("warning") }, "dirty": func(o *selectedGitObservation) { o.dirty = true }, "over bound": func(o *selectedGitObservation) {
			o.commands[2].stderr = bytes.Repeat([]byte{'x'}, maxSelectedGitStream+1)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			o, x := syntheticHostGit(t, "BEFORE")
			mutate(&o)
			state, e := publishHostGit(privatePublicationRoot(t), hostGitVerifiedStreams{}, o, x, reviewedSuccessorSchemaDigest, nil)
			if e == nil || state.stage != "ABSENT" {
				t.Fatalf("ASSERT_HOST_GIT_REJECT: %+v %v", state, e)
			}
		})
	}
}
func TestHostGitRetainedCanonicalAndReadback(t *testing.T) {
	o, x := syntheticHostGit(t, "BEFORE")
	root := privatePublicationRoot(t)
	fail := func(_ *publication.Root, _ string, _ int64) ([]byte, error) { return nil, os.ErrPermission }
	state, e := publishHostGit(root, hostGitVerifiedStreams{}, o, x, reviewedSuccessorSchemaDigest, fail)
	if e == nil || state.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_HOST_GIT_UNVERIFIED: %+v %v", state, e)
	}
	o, x = syntheticHostGit(t, "AFTER")
	root = privatePublicationRoot(t)
	state, e = publishHostGit(root, hostGitVerifiedStreams{}, o, x, reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
	if e != nil {
		t.Fatal(e)
	}
	r, ok := decodeHostGit(b)
	if !ok {
		t.Fatal("decode")
	}
	canonical, e := canonicalHostGit(r)
	if e != nil || !bytes.Equal(b, canonical) {
		t.Fatal("not canonical")
	}
	if parsed, ok := decodeHostGit(append(append([]byte(nil), b...), '\n')); !ok {
		t.Fatal("valid JSON whitespace should parse")
	} else if recanonical, e := canonicalHostGit(parsed); e != nil || bytes.Equal(recanonical, append(append([]byte(nil), b...), '\n')) {
		t.Fatal("noncanonical bytes accepted")
	}
	if _, ok := decodeHostGit(bytes.Replace(b, []byte(`"dirty":false`), []byte(`"dirty":false,"dirty":false`), 1)); ok {
		t.Fatal("duplicate key accepted")
	}
	if len(o.commands[2].stdout) != 0 {
		t.Fatal("fixture")
	}
	atBound := bytes.Repeat([]byte{'x'}, maxSelectedGitStream)
	if hostGitStreamRef(atBound).ByteLength != maxSelectedGitStream || hostGitStreamRef(nil).ByteLength != 0 {
		t.Fatal("stream bounds")
	}
	r.Commands[2].Stderr = hostGitStreamRef(atBound)
	streams := [3][2][]byte{{[]byte(x.Root + "\n"), nil}, {[]byte(x.Commit + "\n"), nil}, {nil, atBound}}
	if hostGitEligible(r, x, streams) {
		t.Fatal("nonempty stderr accepted at bound")
	}
	streams[2][1] = append(atBound, 'x')
	if hostGitEligible(r, x, streams) {
		t.Fatal("edge+1 accepted")
	}
}
