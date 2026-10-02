package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/adr0011builtinprofile"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
)

func fixtureDigest(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}
func fixtureJSON(x any) []byte { b, _ := json.Marshal(x); return b }
func fixtureObject(b []byte) map[string]any {
	var x map[string]any
	_ = json.Unmarshal(b, &x)
	return x
}
func fixtureRef(role, uri, digest string, length int) map[string]any {
	return map[string]any{"role": role, "schema_version": uri, "selector": role + ".json", "digest": digest, "byte_length": length}
}
func fixtureHost(t *testing.T) (*publication.Root, adr0011builtinprofile.Originals, adr0011querytarget.Query) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	p := adr0011builtinprofile.BuiltinV1()
	profile := fixtureObject(p.Profile)
	pd, td, sd := fixtureDigest(p.Profile), fixtureDigest(p.TargetPolicy), fixtureDigest(p.TargetSchema)
	measurement := fixtureJSON(map[string]any{"role": "ADR0011_INSTALLED_EXECUTABLE_MEASUREMENT_V1", "version": 1, "profile_digest": pd, "executable_digest": fixtureDigest([]byte("installed")), "executable_byte_length": 9, "adapter_scope": "LINKED_INTO_INSTALLED_EXECUTABLE_ONLY_V1", "measurement_rule": "PIN_OPENED_EXECUTABLE_FILE_COMPARE_PATH_AT_START_PREWRITE_POSTCAPTURE_V1"})
	md := fixtureDigest(measurement)
	target := profile["target_query"].(map[string]any)
	impl := fixtureDigest(append(append([]byte("ADR0011_DOCUMENT_SYMBOL_TARGET_IMPLEMENTATION_V1"), 0), fixtureJSON(map[string]any{"measurement_digest": md, "target_policy_digest": td, "target_schema_digest": sd, "target_role_uri": target["schema_role_uri"]})...))
	policies := map[string]any{}
	for _, role := range []string{"method", "admission", "privacy", "retention"} {
		policies[role] = profile["policies"].(map[string]any)[role].(map[string]any)["digest"]
	}
	schemaDigest := profile["admission_schema"].(map[string]any)["digest"]
	d := fixtureDigest([]byte("fixture"))
	base := "https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.proposed.schema.json#/$defs/"
	nonce := "cccccccccccccccccccccccccccccccc"
	plan := fixtureJSON(map[string]any{"role": "ADR0011_LOCAL_REFERENCES_PLAN_V1", "version": 1, "profile_digest": pd, "session_id": "synthetic", "generation": 1, "workspace_uri": "file:///fixture", "query_uri": "file:///fixture/a.go", "line": 1, "character": 2, "encoding": "utf-16", "document_version": 1, "document_byte_length": 3, "document_digest": d, "source_ref": fixtureRef("prepared_source", base+"preparedSource", d, 1), "git_root_uri": "file:///fixture", "git_commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "git_before": fixtureRef("host_git_before", base+"hostGit", d, 1), "policy_digests": policies, "schema_digest": schemaDigest, "implementation_digest": md, "invocation_nonce": nonce, "params_digest": d, "target_schema_digest": sd, "target_policy_digest": td, "target_implementation_digest": impl, "limits": map[string]any{"max_messages": 4, "max_wire_bytes": 4096, "timeout_ms": 1000, "max_work": 1000}})
	planID := fixtureDigest(append(append([]byte("ADR0011_LOCAL_REFERENCES_PLAN_V1"), 0), plan...))
	selection := fixtureJSON(map[string]any{"role": "ADR0011_LOCAL_REFERENCES_SELECTION_V1", "version": 1, "profile_digest": pd, "plan_id": planID, "plan_ref": map[string]any{"role": "plan", "schema_version": "https://jaresty.github.io/lsp-trace/schemas/adr0011-builtin-local-references-v1.proposed.schema.json#/$defs/plan", "selector": "plan.json", "digest": fixtureDigest(plan), "byte_length": len(plan)}, "declaration_ref": fixtureRef("production_query_declaration", "https://jaresty.github.io/lsp-trace/schemas/adr0011-production-admission-v1.proposed.schema.json#/$defs/productionQueryDeclaration", d, 1), "implementation_digest": md, "policy_digests": policies, "schema_digest": schemaDigest, "root_identity_digest": d, "session_id": "synthetic", "generation": 1, "invocation_nonce": nonce, "expected_custody_ref": fixtureRef("attempt_host_custody", "https://jaresty.github.io/lsp-trace/schemas/adr0011-production-admission-v1.proposed.schema.json#/$defs/attemptHostCustody", d, 1), "target_schema_digest": sd, "target_policy_digest": td, "target_implementation_digest": impl})
	p.Measurement, p.Plan, p.Selection = measurement, plan, selection
	q := adr0011querytarget.Query{OccurrenceID: fixtureDigest([]byte("occurrence")), URI: "file:///fixture/a.go", Encoding: "utf-16", DocumentVersion: "1", SourceDigest: d, SessionID: "synthetic", Generation: 1, Line: 1, Character: 2}
	return root, p, q
}

var fixtureSymbol = []byte(`[{"name":"outer","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":4,"character":0}},"selectionRange":{"start":{"line":0,"character":1},"end":{"line":3,"character":0}},"children":[{"name":"inner","kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":2,"character":0}},"selectionRange":{"start":{"line":1,"character":1},"end":{"line":1,"character":6}}}]}]`)

func fixtureClaim(t *testing.T, q adr0011querytarget.Query, pre adr0011builtinprofile.Originals) adr0011builtinprofile.Originals {
	t.Helper()
	c, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(q, fixtureSymbol)
	if err != nil {
		t.Fatal(err)
	}
	pre.Descendants = map[string][]byte{privateBuiltinDescendantRole: privateBuiltinDescendant(q, c)}
	return pre
}

func TestPrivateBuiltinOwnerFreezeAndReadback(t *testing.T) {
	root, p, q := fixtureHost(t)
	owner, err := newPrivateBuiltinOwner(root, p, q, "descendant.json")
	if err != nil {
		t.Fatal("ASSERT_HOST_PREWRITE_FREEZE", err)
	}
	p.Plan = bytes.Replace(p.Plan, []byte(`"invocation_nonce":"cccccccccccccccccccccccccccccccc"`), []byte(`"invocation_nonce":"dddddddddddddddddddddddddddddddd"`), 1)
	called := false
	candidate, err := owner.acquirePrivateBuiltin(func(pre adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error) {
		called = true
		if bytes.Equal(pre.Plan, p.Plan) {
			t.Fatal("ASSERT_PREWRITE_HELD_COPY")
		}
		return privateBuiltinSymbolResult{DocumentSymbols: fixtureSymbol, Claim: fixtureClaim(t, q, pre)}, nil
	})
	if err != nil || !called || candidate.SymbolName != "inner" {
		body, readErr := publication.ReadVerifiedBoundFile(root, "descendant.json", 1<<20)
		t.Fatalf("ASSERT_VERIFIED_STRICT_INNER: %v %+v read=%v bytes=%q", err, candidate, readErr, body)
	}
	if _, err = publication.ReadVerifiedBoundFile(root, "descendant.json", 1<<20); err != nil {
		t.Fatal("ASSERT_VERIFIED_DESCENDANT_READBACK", err)
	}
}

func TestPrivateBuiltinOwnerRestoresIndependentDigest(t *testing.T) {
	root, p, q := fixtureHost(t)
	original, err := newPrivateBuiltinOwner(root, p, q, "original.json")
	if err != nil {
		t.Fatal(err)
	}
	retained, digest := original.privateHostSnapshot()
	// The test host keeps this digest outside retained bytes; it is not supplied by the claimant.
	newRoot, _, _ := fixtureHost(t)
	restarted, err := restorePrivateBuiltinOwner(newRoot, retained, digest, q, "restored.json")
	if err != nil {
		t.Fatalf("ASSERT_RESTORE_HELD_V1_BEFORE_CALLBACK: %v", err)
	}
	called := false
	candidate, err := restarted.acquirePrivateBuiltin(func(pre adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error) {
		called = true
		return privateBuiltinSymbolResult{DocumentSymbols: fixtureSymbol, Claim: fixtureClaim(t, q, pre)}, nil
	})
	if err != nil || !called || candidate.SymbolName != "inner" {
		t.Fatalf("ASSERT_RESTART_VERIFIED_REPLAY: %v %+v", err, candidate)
	}
	modified := retained
	plan := fixtureObject(modified.Plan)
	plan["invocation_nonce"] = "dddddddddddddddddddddddddddddddd"
	modified.Plan = fixtureJSON(plan)
	selection := fixtureObject(modified.Selection)
	selection["invocation_nonce"] = "dddddddddddddddddddddddddddddddd"
	selection["plan_id"] = fixtureDigest(append(append([]byte("ADR0011_LOCAL_REFERENCES_PLAN_V1"), 0), modified.Plan...))
	ref := selection["plan_ref"].(map[string]any)
	ref["digest"] = fixtureDigest(modified.Plan)
	ref["byte_length"] = len(modified.Plan)
	modified.Selection = fixtureJSON(selection)
	bad, err := restorePrivateBuiltinOwner(newRoot, modified, digest, q, "altered.json")
	if !errors.Is(err, ErrAcquisition) || bad != nil {
		t.Fatalf("ASSERT_COHERENT_REHASH_RESTORE_REJECTED: %v", err)
	}
	v2 := retained
	v2.Profile = bytes.Replace(v2.Profile, []byte(`"max_acquisition_queries":16`), []byte(`"max_acquisition_queries":17`), 1)
	bad, err = restorePrivateBuiltinOwner(newRoot, v2, digest, q, "v2.json")
	if !errors.Is(err, ErrAcquisition) || bad != nil {
		t.Fatalf("ASSERT_V2_PROFILE_RESTORE_REJECTED: %v", err)
	}
	bad, err = restorePrivateBuiltinOwner(newRoot, retained, digest, q, "../escape.json")
	if !errors.Is(err, ErrAcquisition) || bad != nil {
		t.Fatalf("ASSERT_RESTORE_SELECTOR_SHARED: %v", err)
	}
}

func TestPrivateBuiltinOwnerRequiresFreshReadback(t *testing.T) {
	for _, mode := range []string{"read-error", "altered", "verified"} {
		t.Run(mode, func(t *testing.T) {
			root, p, q := fixtureHost(t)
			owner, err := newPrivateBuiltinOwner(root, p, q, "readback.json")
			if err != nil {
				t.Fatal(err)
			}
			called := 0
			owner.readbackTestHook = func(r *publication.Root, selector string, limit int64) ([]byte, error) {
				called++
				if mode == "read-error" {
					return nil, errors.New("injected read failure")
				}
				observed, e := publication.ReadVerifiedBoundFile(r, selector, limit)
				if mode == "altered" && e == nil {
					observed = append([]byte(nil), observed...)
					observed[0] ^= 1
				}
				return observed, e
			}
			candidate, err := owner.acquirePrivateBuiltin(func(pre adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error) {
				return privateBuiltinSymbolResult{DocumentSymbols: fixtureSymbol, Claim: fixtureClaim(t, q, pre)}, nil
			})
			if called != 1 {
				t.Fatalf("ASSERT_FRESH_READBACK_CALLED_%s: %d", mode, called)
			}
			if mode == "verified" {
				if err != nil || candidate.SymbolName != "inner" {
					t.Fatalf("ASSERT_VERIFIED_READBACK_SUCCESS: %v %+v", err, candidate)
				}
			} else if !errors.Is(err, ErrAcquisition) || candidate != (adr0011querytarget.Candidate{}) {
				t.Fatalf("ASSERT_READBACK_FAILURE_CLOSED_%s: %v %+v", mode, err, candidate)
			}
		})
	}
}

func TestPrivateBuiltinOwnerRejectsHostOverrideBeforeCallback(t *testing.T) {
	root, p, q := fixtureHost(t)
	p.Profile = bytes.Replace(p.Profile, []byte(`"max_acquisition_queries":16`), []byte(`"max_acquisition_queries":17`), 1)
	owner, err := newPrivateBuiltinOwner(root, p, q, "invalid.json")
	if !errors.Is(err, ErrAcquisition) || owner != nil {
		t.Fatalf("ASSERT_INVALID_PROFILE_NO_CALLBACK: %v", err)
	}
	root, p, q = fixtureHost(t)
	q.Character++
	owner, err = newPrivateBuiltinOwner(root, p, q, "invalid-plan.json")
	if !errors.Is(err, ErrAcquisition) || owner != nil {
		t.Fatalf("ASSERT_HOST_QUERY_PLAN_BINDING: %v", err)
	}
	root, p, q = fixtureHost(t)
	owner, err = newPrivateBuiltinOwner(root, p, q, "../escape.json")
	if !errors.Is(err, ErrAcquisition) || owner != nil {
		t.Fatalf("ASSERT_PRECALLBACK_SELECTOR_BOUND: %v", err)
	}
	if got := NewDisabled(nil); got == nil || got.testEnabled || got.root != nil || got.manager != nil {
		t.Fatal("ASSERT_PRODUCTION_OWNER_DEFAULT_OFF")
	}
}

func TestPrivateBuiltinOwnerRejectsSubstitutionAndPublicationFailure(t *testing.T) {
	root, p, q := fixtureHost(t)
	owner, err := newPrivateBuiltinOwner(root, p, q, "descendant.json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.acquirePrivateBuiltin(func(pre adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error) {
		pre.Selection = bytes.Replace(pre.Selection, []byte(`"invocation_nonce":"cccccccccccccccccccccccccccccccc"`), []byte(`"invocation_nonce":"dddddddddddddddddddddddddddddddd"`), 1)
		return privateBuiltinSymbolResult{DocumentSymbols: fixtureSymbol, Claim: fixtureClaim(t, q, pre)}, nil
	})
	if !errors.Is(err, ErrAcquisition) {
		t.Fatalf("ASSERT_COHERENT_CLAIM_OVERRIDE_REJECTED: %v", err)
	}
	owner, err = newPrivateBuiltinOwner(root, p, q, "descendant.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root.Path(), "descendant.json"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = owner.acquirePrivateBuiltin(func(pre adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error) {
		return privateBuiltinSymbolResult{DocumentSymbols: fixtureSymbol, Claim: fixtureClaim(t, q, pre)}, nil
	})
	if !errors.Is(err, ErrAcquisition) {
		t.Fatalf("ASSERT_NONVERIFIED_PUBLICATION_REJECTED: %v", err)
	}
	if _, err = newPrivateBuiltinOwner(root, adr0011builtinprofile.BuiltinV1(), q, "another.json"); !errors.Is(err, ErrAcquisition) {
		t.Fatalf("ASSERT_NO_V2_OR_DEFAULT_FALLBACK: %v", err)
	}
	rootCoherent, pCoherent, qCoherent := fixtureHost(t)
	coherent, err := newPrivateBuiltinOwner(rootCoherent, pCoherent, qCoherent, "coherent.json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = coherent.acquirePrivateBuiltin(func(pre adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error) {
		plan := fixtureObject(pre.Plan)
		plan["invocation_nonce"] = "dddddddddddddddddddddddddddddddd"
		pre.Plan = fixtureJSON(plan)
		selection := fixtureObject(pre.Selection)
		selection["invocation_nonce"] = "dddddddddddddddddddddddddddddddd"
		selection["plan_id"] = fixtureDigest(append(append([]byte("ADR0011_LOCAL_REFERENCES_PLAN_V1"), 0), pre.Plan...))
		ref := selection["plan_ref"].(map[string]any)
		ref["digest"] = fixtureDigest(pre.Plan)
		ref["byte_length"] = len(pre.Plan)
		pre.Selection = fixtureJSON(selection)
		return privateBuiltinSymbolResult{DocumentSymbols: fixtureSymbol, Claim: fixtureClaim(t, qCoherent, pre)}, nil
	})
	if !errors.Is(err, ErrAcquisition) {
		t.Fatalf("ASSERT_COHERENT_REHASH_HELD_REJECTED: %v", err)
	}
	root2, p2, q2 := fixtureHost(t)
	restarted, err := newPrivateBuiltinOwner(root2, p2, q2, "restart.json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = restarted.acquirePrivateBuiltin(func(pre adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error) {
		pre.Descendants = map[string][]byte{privateBuiltinDescendantRole: []byte(`{"v":2}`)}
		return privateBuiltinSymbolResult{DocumentSymbols: fixtureSymbol, Claim: pre}, nil
	})
	if !errors.Is(err, ErrAcquisition) {
		t.Fatalf("ASSERT_RESTART_V2_DESCENDANT_REJECTED: %v", err)
	}
	root3, p3, q3 := fixtureHost(t)
	wrong, err := newPrivateBuiltinOwner(root3, p3, q3, "wrong.json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = wrong.acquirePrivateBuiltin(func(pre adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error) {
		pre = fixtureClaim(t, q3, pre)
		pre.Descendants[privateBuiltinDescendantRole] = []byte(`{"v":2}`)
		return privateBuiltinSymbolResult{DocumentSymbols: fixtureSymbol, Claim: pre}, nil
	})
	if !errors.Is(err, ErrAcquisition) {
		t.Fatalf("ASSERT_DESCENDANT_CLAIM_SUBSTITUTION: %v", err)
	}
}
