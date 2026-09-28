package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/publication"
)

func finalSyntheticFixture(t *testing.T, token string) (*publication.Root, proposalContextInputs, finalDeclaration, privateBodyPublication, privateBodyPublication, finalImplementationPin) {
	t.Helper()
	root, x := proposalRecordFixture(t, token)
	f := preinvokeFacts{SessionID: x.ResponseInputs.Pair.SessionID, Generation: x.ResponseInputs.Pair.Generation, Workspace: x.TargetInputs.Workspace, URI: x.TargetInputs.Query.URI, Line: x.TargetInputs.Query.Line, Character: x.TargetInputs.Query.Character, Version: x.TargetInputs.Binding.Version, Source: append([]byte(nil), x.SourceBytes...), SourceLength: len(x.SourceBytes), SourceDigest: privateDigest(x.SourceBytes), GitRoot: x.Git.Root, GitCommit: x.Git.Commit, Executable: x.Git.Executable}
	selector, digest, id, err := publishPreinvokeOccurrence(root, f)
	if err != nil || id != x.OccurrenceID || !verifyPreinvokeOccurrence(root, selector, digest, f, id) {
		t.Fatalf("ASSERT_FINAL_PREINVOKE: id=%s expected=%s err=%v", id, x.OccurrenceID, err)
	}
	proposal, err := publishProposalRecord(root, x, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := publishCandidateRecord(root, proposal, x, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Reviewed local source-byte pin: production still passes nil and fails closed.
	pin := finalImplementationPin(reviewedSyntheticSourcePin)
	return root, x, finalDeclaration{selector, digest, f}, proposal, candidate, pin
}

func TestPrivateFinalSynthetic(t *testing.T) {
	contract, err := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if err = json.Unmarshal(contract, &schema); err != nil {
		t.Fatal(err)
	}
	id := schema.(map[string]any)["$id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err = compiler.AddResource(id, schema); err != nil {
		t.Fatal(err)
	}
	shape, err := compiler.Compile(id + "#/$defs/final")
	if err != nil {
		t.Fatal(err)
	}
	var empty []byte
	for _, token := range []string{"null", "[]", "[" + eventsLocation + "," + eventsLocation + "]"} {
		t.Run(token, func(t *testing.T) {
			root, x, declaration, proposal, candidate, pin := finalSyntheticFixture(t, token)
			state, err := publishFinalRecord(root, candidate, proposal, x, declaration, pin, nil, nil)
			if err != nil || state.stage != "VERIFIED" || !replayFinalRecord(root, state, candidate, proposal, x, declaration, pin, nil) {
				t.Fatalf("ASSERT_FINAL_VERIFIED: %+v %v", state, err)
			}
			raw, err := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
			if err != nil || state.digest != finalRecordDigest(raw) || state.selector != finalRecordSelector(state.digest) {
				t.Fatal("ASSERT_FINAL_EXACT", err)
			}
			var decoded any
			if err = json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if err = shape.Validate(decoded); err != nil {
				t.Fatalf("ASSERT_FINAL_SCHEMA: %v", err)
			}
			v := decoded.(map[string]any)
			p := x.ScannerObservation.knownE
			if v["t"] != float64(1) || v["a"] != float64(p) || v["p"] != float64(p) || len(v["dependency_refs"].([]any)) != 21 {
				t.Fatalf("ASSERT_FINAL_COUNTS_REFS: %s", raw)
			}
			foundDeclaration := false
			for _, item := range v["dependency_refs"].([]any) {
				ref := item.(map[string]any)
				if ref["schema_version"] == "QUERY_OCCURRENCE_TEST_V1" && ref["selector"] == declaration.Selector && ref["digest"] == declaration.Digest {
					foundDeclaration = true
				}
			}
			if !foundDeclaration {
				t.Fatal("ASSERT_FINAL_DECLARATION_REF")
			}
			if token == "null" {
				empty = append([]byte(nil), raw...)
			}
			if token == "[]" && bytes.Equal(raw, empty) {
				t.Fatal("ASSERT_FINAL_EMPTY_IDENTITIES")
			}
			if p == 2 {
				candidateBytes, e := publication.ReadVerifiedBoundFile(root, candidate.selector, sourceRecordLimit)
				if e != nil {
					t.Fatal(e)
				}
				var c struct {
					Occurrences []struct {
						Ordinal      int    `json:"ordinal"`
						OccurrenceID string `json:"occurrence_id"`
					} `json:"occurrences"`
				}
				if json.Unmarshal(candidateBytes, &c) != nil || len(c.Occurrences) != 2 || c.Occurrences[0].Ordinal != 0 || c.Occurrences[1].Ordinal != 1 || c.Occurrences[0].OccurrenceID == c.Occurrences[1].OccurrenceID {
					t.Fatal("ASSERT_FINAL_REPEATED_ORDINAL")
				}
			}
			unverified := state
			unverified.stage = "COMMITTED_UNVERIFIED"
			if replayFinalRecord(root, unverified, candidate, proposal, x, declaration, pin, nil) {
				t.Fatal("ASSERT_FINAL_NO_UNVERIFIED")
			}
			for name, change := range map[string]func(*proposalContextInputs){
				"source":          func(y *proposalContextInputs) { y.SourceBytes = []byte("changed") },
				"raw":             func(y *proposalContextInputs) { y.Payload = []byte("changed") },
				"journal":         func(y *proposalContextInputs) { y.Journal = []byte("changed") },
				"target":          func(y *proposalContextInputs) { y.Target.digest = privateDigest([]byte("changed")) },
				"method":          func(y *proposalContextInputs) { y.Method.digest = privateDigest([]byte("changed")) },
				"events":          func(y *proposalContextInputs) { y.Events.digest = privateDigest([]byte("changed")) },
				"prepared":        func(y *proposalContextInputs) { y.TargetInputs.Prepared.digest = privateDigest([]byte("changed")) },
				"source-identity": func(y *proposalContextInputs) { y.TargetInputs.Source.digest = privateDigest([]byte("changed")) },
				"revision":        func(y *proposalContextInputs) { y.TargetInputs.Revision.digest = privateDigest([]byte("changed")) },
				"git-before":      func(y *proposalContextInputs) { y.TargetInputs.Before.digest = privateDigest([]byte("changed")) },
				"git-after":       func(y *proposalContextInputs) { y.TargetInputs.After.digest = privateDigest([]byte("changed")) },
				"target-result":   func(y *proposalContextInputs) { y.TargetInputs.Result.digest = privateDigest([]byte("changed")) },
				"target-owner":    func(y *proposalContextInputs) { y.TargetInputs.OwnerRead.digest = privateDigest([]byte("changed")) },
				"response-owner":  func(y *proposalContextInputs) { y.ResponseInputs.OwnerRead.digest = privateDigest([]byte("changed")) },
				"response-read":   func(y *proposalContextInputs) { y.ResponseRead.digest = privateDigest([]byte("changed")) },
				"raw-result":      func(y *proposalContextInputs) { y.RawResult.digest = privateDigest([]byte("changed")) },
				"scanner":         func(y *proposalContextInputs) { y.Scanner.digest = privateDigest([]byte("changed")) },
				"policy-0":        func(y *proposalContextInputs) { y.Policies[0].Record.digest = privateDigest([]byte("changed")) },
				"policy-1":        func(y *proposalContextInputs) { y.Policies[1].Record.digest = privateDigest([]byte("changed")) },
				"policy-2":        func(y *proposalContextInputs) { y.Policies[2].Record.digest = privateDigest([]byte("changed")) },
				"policy-3":        func(y *proposalContextInputs) { y.Policies[3].Record.digest = privateDigest([]byte("changed")) },
			} {
				t.Run(name, func(t *testing.T) {
					y := x
					change(&y)
					if replayFinalRecord(root, state, candidate, proposal, y, declaration, pin, nil) {
						t.Fatal("ASSERT_FINAL_REJECT_SUBSTITUTION")
					}
				})
			}
			for _, item := range v["dependency_refs"].([]any) {
				selector := item.(map[string]any)["selector"].(string)
				// The test declaration is read by its own exact-byte verifier;
				// every other dependency passes through the supplied reader.
				if selector == declaration.Selector {
					continue
				}
				t.Run("missing-"+selector, func(t *testing.T) {
					missing := func(r *publication.Root, s string, n int64) ([]byte, error) {
						if s == selector {
							return nil, errors.New("missing")
						}
						return publication.ReadVerifiedBoundFile(r, s, n)
					}
					if replayFinalRecord(root, state, candidate, proposal, x, declaration, pin, missing) {
						t.Fatal("ASSERT_FINAL_MISSING_BYTE")
					}
				})
			}
			if replayFinalRecord(root, state, candidate, proposal, x, declaration, nil, nil) {
				t.Fatal("ASSERT_FINAL_NO_PRODUCTION_PIN")
			}
			again, e := publishFinalRecord(root, candidate, proposal, x, declaration, pin, nil, nil)
			if e == nil || again.stage != "COMMITTED_UNVERIFIED" || again.selector != state.selector {
				t.Fatalf("ASSERT_FINAL_COLLISION: %+v %v", again, e)
			}
		})
	}
}

func TestPrivateFinalCommitStages(t *testing.T) {
	root, x, d, p, c, pin := finalSyntheticFixture(t, "[]")
	wrong := d
	wrong.Facts.Character++
	absent, err := publishFinalRecord(root, c, p, x, wrong, pin, nil, nil)
	if err == nil || absent.stage != "ABSENT" {
		t.Fatalf("ASSERT_FINAL_PRECOMMIT: %+v %v", absent, err)
	}
	body, err := canonicalFinalRecord(root, c, p, x, d, pin, nil)
	if err != nil {
		t.Fatal(err)
	}
	selector := finalRecordSelector(finalRecordDigest(body))
	fired := false
	trace := func(event publication.BoundFileTraceEvent) {
		if !fired && event.Stage == "HARDLINK" && event.Result == "INSTALLED" {
			fired = true
			_ = os.Remove(filepath.Join(root.Path(), selector))
		}
	}
	state, err := publishFinalRecord(root, c, p, x, d, pin, nil, trace)
	if !fired || err == nil || state.stage != "COMMITTED_UNVERIFIED" || replayFinalRecord(root, state, c, p, x, d, pin, nil) {
		t.Fatalf("ASSERT_FINAL_POSTCOMMIT: %+v %v", state, err)
	}
}
