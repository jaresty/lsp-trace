package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"os"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
)

func targetRecordFixture(t *testing.T) (*publication.Root, targetRecordInputs) {
	t.Helper()
	root := privatePublicationRoot(t)
	f, p, b := ownerReadFixture(t, "textDocument/documentSymbol")
	b.SHA256 = privateDigest([]byte("source"))
	p.Source = &b
	body := []byte(`{"jsonrpc":"2.0","id":31,"result":` + selectedSymbol + `}`)
	f.response = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
	p.Result = []byte(selectedSymbol)
	p.Read.FrameBytes = int64(len(f.response))
	p.Read.FrameSHA256 = privateDigest(f.response)
	f.pair = p
	var ok bool
	f.resultSpan, ok = exactOwnedSpan(f.response, "result", p.Method, p.Key.ID)
	if !ok {
		t.Fatal("span")
	}
	owner, e := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	observed, git := syntheticHostGit(t, "BEFORE")
	git.Root = "/"
	observed.root = "/"
	observed.rootURI = selectedGitFileURI("/")
	observed.cwdURI = observed.rootURI
	observed.commands[0].stdout = []byte("/\n")
	facts := preinvokeFacts{SessionID: "owned", Generation: 7, Workspace: "/", URI: b.URI, Line: 1, Character: 4, Version: b.Version, SourceDigest: b.SHA256, SourceLength: len([]byte("source")), Source: []byte("source"), GitRoot: git.Root, GitCommit: git.Commit, Executable: git.Executable}
	_, occurrenceID, e := preinvokeCanonical(facts)
	if e != nil {
		t.Fatal(e)
	}
	q := adr0011querytarget.Query{OccurrenceID: occurrenceID, URI: b.URI, Encoding: "utf-16", DocumentVersion: "1", SourceDigest: b.SHA256, SessionID: "owned", Generation: 7, Line: 1, Character: 4}
	result, e := publishTargetResult(root, owner, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, q, nil)
	if e != nil {
		t.Fatal(e)
	}
	prepared, source, e := publishPreparedSource(root, []byte("source"), b.URI, b.Version, "/", reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	streams := hostGitVerifiedStreams{}
	before, e := publishHostGit(root, streams, observed, git, reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	afterObservation := observed
	for i := range afterObservation.commands {
		afterObservation.commands[i].at = afterObservation.commands[i].at.Add(time.Minute)
	}
	afterGit := git
	afterGit.Phase = "AFTER"
	after, e := publishHostGit(root, streams, afterObservation, afterGit, reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	revision, e := publishRevisionIdentity(root, before, after, git, reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	return root, targetRecordInputs{Frames: f, Pair: p, Binding: b, Query: q, Original: []byte("source"), Workspace: "/", Git: git, Prepared: prepared, Source: source, Revision: revision, Before: before, After: after, OwnerRead: owner, Result: result, SchemaDigest: reviewedSuccessorSchemaDigest}
}

func TestPrivateTargetRecordPrecommitFailureRemainsAbsent(t *testing.T) {
	root, x := targetRecordFixture(t)
	reads := 0
	read := func(r *publication.Root, selector string, n int64) ([]byte, error) {
		b, err := publication.ReadVerifiedBoundFile(r, selector, n)
		if err == nil && strings.Contains(selector, "-target-result-v1-") {
			reads++
			if reads == 2 {
				if closeErr := root.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
			}
		}
		return b, err
	}
	state, err := publishTargetRecord(root, x, read)
	if reads != 2 || err == nil || state.stage != "ABSENT" {
		t.Fatalf("ASSERT_TARGET_RECORD_PRECOMMIT_ABSENT: reads=%d state=%+v err=%v", reads, state, err)
	}
}

func TestPrivateTargetRecordSynthetic(t *testing.T) {
	root, x := targetRecordFixture(t)
	state, e := publishTargetRecord(root, x, nil)
	if e != nil || state.stage != "VERIFIED" {
		t.Fatalf("ASSERT_TARGET_RECORD_PUBLISH: %+v %v", state, e)
	}
	raw, e := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
	if e != nil || !bytes.HasSuffix(raw, []byte("\n")) || bytes.HasSuffix(raw, []byte("\n\n")) || privateDigest(raw) != state.digest || !replayTargetRecord(root, state, x, nil) {
		t.Fatal("ASSERT_TARGET_RECORD_EXACT_REPLAY", e)
	}
	if !bytes.Contains(raw, []byte(`"SourceRef":{"selector":`)) || !bytes.Contains(raw, []byte(`"DisplayRange":{"start":{"line":`)) || !bytes.Contains(raw, []byte(`"SelectionRange":{"start":{"line":`)) {
		t.Fatalf("ASSERT_TARGET_RECORD_SCHEMA_NESTING: %s", raw)
	}
	contractBytes, schemaErr := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	var contract any
	if schemaErr = json.Unmarshal(contractBytes, &contract); schemaErr != nil {
		t.Fatal(schemaErr)
	}
	id := contract.(map[string]any)["$id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if schemaErr = compiler.AddResource(id, contract); schemaErr != nil {
		t.Fatal(schemaErr)
	}
	shape, schemaErr := compiler.Compile(id + "#/$defs/targetRecord")
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	var decoded any
	if schemaErr = json.Unmarshal(raw, &decoded); schemaErr != nil {
		t.Fatal(schemaErr)
	}
	if schemaErr = shape.Validate(decoded); schemaErr != nil {
		t.Fatalf("ASSERT_TARGET_RECORD_SUCCESSOR_SCHEMA: %v", schemaErr)
	}
	unverified := state
	unverified.stage = "COMMITTED_UNVERIFIED"
	if replayTargetRecord(root, unverified, x, nil) {
		t.Fatal("ASSERT_TARGET_RECORD_UNVERIFIED_REPLAY")
	}
	cases := map[string]func(*targetRecordInputs){
		"raw-key": func(y *targetRecordInputs) {
			y.Frames.request = bytes.Replace(y.Frames.request, []byte(`"id":31`), []byte(`"id":32`), 1)
		},
		"original-write-body": func(y *targetRecordInputs) {
			original, ok := exactFrameBody(y.Frames.request)
			if !ok {
				t.Fatal("fixture frame")
			}
			changed := append([]byte{' '}, original...)
			y.Frames.request = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(changed), changed))
			y.Pair.Write.FrameBytes = int64(len(y.Frames.request))
			y.Pair.Write.FrameSHA256 = privateDigest(y.Frames.request)
			y.Frames.pair.Write = y.Pair.Write
		},
		"original-read-body": func(y *targetRecordInputs) {
			original, ok := exactFrameBody(y.Frames.response)
			if !ok {
				t.Fatal("fixture frame")
			}
			changed := append([]byte{' '}, original...)
			y.Frames.response = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(changed), changed))
			y.Pair.Read.FrameBytes = int64(len(y.Frames.response))
			y.Pair.Read.FrameSHA256 = privateDigest(y.Frames.response)
			y.Frames.pair.Read = y.Pair.Read
		},
		"source":        func(y *targetRecordInputs) { y.Original = []byte("substituted") },
		"revision":      func(y *targetRecordInputs) { y.Git.Commit = strings.Repeat("a", 40) },
		"missing-owner": func(y *targetRecordInputs) { y.OwnerRead.stage = "ABSENT" },
		"result":        func(y *targetRecordInputs) { y.Result.digest = "sha256:" + strings.Repeat("0", 64) },
		"query-id":      func(y *targetRecordInputs) { y.Query.OccurrenceID = "owned-1" },
		"query-point":   func(y *targetRecordInputs) { y.Query.Character = 9 },
		"ref-order":     func(y *targetRecordInputs) { y.Source, y.Revision = y.Revision, y.Source },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			y := x
			mutate(&y)
			if replayTargetRecord(root, state, y, nil) {
				t.Fatal("ASSERT_TARGET_RECORD_REJECT_" + name)
			}
		})
	}
	for name, mutation := range map[string][]byte{"extra-field": bytes.Replace(raw, []byte(`"ResultDigest":`), []byte(`"Extra":1,"ResultDigest":`), 1), "extra-lf": append(append([]byte(nil), raw...), '\n'), "order": bytes.Replace(raw, []byte(`"Version":"`), []byte(`"Method":"textDocument/documentSymbol","Version":"`), 1)} {
		t.Run(name, func(t *testing.T) {
			y := state
			y.digest = privateDigest(mutation)
			y.selector = targetRecordSelector(y.digest)
			y.byteCount = len(mutation)
			receipt, e := publication.PublishBoundFile(root, y.selector, mutation, func([]byte) error { return nil })
			if e != nil || receipt == nil {
				t.Fatal(e)
			}
			if replayTargetRecord(root, y, x, nil) {
				t.Fatal("ASSERT_TARGET_RECORD_BYTES_REJECT_" + name)
			}
		})
	}
	missing := func(r *publication.Root, s string, n int64) ([]byte, error) {
		if strings.Contains(s, "-owner-read-") {
			return nil, errors.New("missing")
		}
		return publication.ReadVerifiedBoundFile(r, s, n)
	}
	if replayTargetRecord(root, state, x, missing) {
		t.Fatal("ASSERT_TARGET_RECORD_MISSING_BODY")
	}
	changedResult := func(r *publication.Root, s string, n int64) ([]byte, error) {
		body, err := publication.ReadVerifiedBoundFile(r, s, n)
		if err == nil && strings.Contains(s, "-target-result-v1-") {
			body = append([]byte(nil), body...)
			body[0] ^= 1
		}
		return body, err
	}
	if replayTargetRecord(root, state, x, changedResult) {
		t.Fatal("ASSERT_TARGET_RECORD_SUBSTITUTED_RESULT")
	}
	tied := []byte(`[{"name":"one","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":3,"character":0}},"selectionRange":{"start":{"line":1,"character":2},"end":{"line":1,"character":8}}},{"name":"two","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":3,"character":0}},"selectionRange":{"start":{"line":1,"character":2},"end":{"line":1,"character":8}}}]`)
	if _, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(x.Query, tied); err == nil {
		t.Fatal("ASSERT_TARGET_RECORD_TIED_SELECTION")
	}
	again, e := publishTargetRecord(root, x, nil)
	if e == nil || again.stage != "COMMITTED_UNVERIFIED" || again.selector != state.selector {
		t.Fatalf("ASSERT_TARGET_RECORD_NO_REPLACE: %+v %v", again, e)
	}
}
