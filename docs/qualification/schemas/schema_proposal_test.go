package schemas_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// These tests validate unregistered draft shapes, not custody, admission or
// production schema registration. Temporal and exact-byte checks remain open.
func draftSchema(t *testing.T, path string) (*jsonschema.Compiler, string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	id, ok := doc["$id"].(string)
	if !ok || !strings.HasPrefix(id, "urn:lsp-trace:draft:unregistered:") {
		t.Fatalf("not an unregistered proposal: %q", id)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	return compiler, id
}

func mustCompile(t *testing.T, c *jsonschema.Compiler, uri string) *jsonschema.Schema {
	t.Helper()
	schema, err := c.Compile(uri)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func expectShape(t *testing.T, schema *jsonschema.Schema, label string, value any, valid bool) {
	t.Helper()
	if err := schema.Validate(value); (err == nil) != valid {
		t.Fatalf("%s: expected valid=%v, got %v", label, valid, err)
	}
}

func TestADR0011DefinitionDraftLSPShapes(t *testing.T) {
	c, id := draftSchema(t, "adr0011-definition-lsp.proposed.schema.json")
	params := mustCompile(t, c, id+"#/$defs/params")
	result := mustCompile(t, c, id+"#/$defs/result")
	p := map[string]any{"textDocument": map[string]any{"uri": "file:///tmp/a.go"}, "position": map[string]any{"line": 0, "character": 0}}
	expectShape(t, params, "exact params", p, true)
	p["other"] = true
	expectShape(t, params, "unknown param", p, false)
	r := map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 1}}
	loc := map[string]any{"uri": "file:///tmp/a.go", "range": r}
	link := map[string]any{"targetUri": "file:///tmp/a.go", "targetRange": r, "targetSelectionRange": r}
	for label, value := range map[string]any{"null": nil, "empty": []any{}, "scalar location": loc, "locations": []any{loc}, "links": []any{link}} {
		expectShape(t, result, label, value, true)
	}
	for label, value := range map[string]any{
		"scalar link":    link,
		"mixed array":    []any{loc, link},
		"nested unknown": map[string]any{"uri": "file:///tmp/a.go", "range": map[string]any{"start": map[string]any{"line": 0, "character": 0, "extra": 1}, "end": map[string]any{"line": 0, "character": 1}}},
	} {
		expectShape(t, result, label, value, false)
	}
}

func cloneRecord(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var copy map[string]any
	if err := json.Unmarshal(b, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func TestADR0011DefinitionDraftConditionalCounterexamples(t *testing.T) {
	c, id := draftSchema(t, "adr0011-definition-records.proposed.schema.json")
	digest := "sha256:" + strings.Repeat("0", 64)
	ref := map[string]any{"selector": "s", "digest": digest}
	gitCommand := map[string]any{"args": []any{"status"}, "exit": 0, "stdout": "", "stderr": "", "timestamp": "now"}
	git := map[string]any{"version": "HOST_GIT_PROBE_V1", "workspaceRoot": "/tmp", "commit": "c", "clean": true, "custody": "HOST_OBSERVED_GIT", "hostExecutable": "/usr/bin/git", "hostExecutableSHA256": digest, "topLevel": gitCommand, "head": gitCommand, "status": gitCommand, "worktrees": gitCommand}
	session := map[string]any{"id": "s", "generation": 1, "workspaceUri": "file:///tmp"}
	source := map[string]any{"uri": "file:///tmp/a.go", "version": 1, "digest": digest, "length": 1, "payload": ref}
	policy := map[string]any{"id": "p", "digest": digest}
	observation := map[string]any{"sessionId": "s", "generation": 1, "requestKey": "k", "method": "textDocument/definition", "frameBytes": 1, "frameSHA256": digest}
	method := map[string]any{"version": "definition.method.v1", "query": ref, "capability": ref, "session": session, "method": "textDocument/definition", "invocationId": "i", "requestKey": "k", "params": ref, "paramsDigest": digest, "readKind": "MATCHED", "resultPresent": true, "resultLength": 4, "resultPayload": ref, "source": source, "beforeGit": git, "afterGit": git, "revision": map[string]any{"commit": "c", "custody": "CALLER_ASSERTED"}, "provider": "gopls", "adapter": policy, "methodPolicy": policy, "privacyPolicy": policy, "requestWrite": observation, "responseRead": observation}
	ms := mustCompile(t, c, id+"#/$defs/method")
	expectShape(t, ms, "matched result baseline", method, true)
	for _, field := range []string{"requestWrite", "responseRead"} {
		bad := cloneRecord(t, method)
		bad[field] = nil
		expectShape(t, ms, "matched null "+field, bad, false)
	}
	for _, field := range []string{"resultPayload", "resultLength"} {
		bad := cloneRecord(t, method)
		if field == "resultLength" {
			bad[field] = 0
		} else {
			bad[field] = nil
		}
		expectShape(t, ms, "present invalid "+field, bad, false)
	}
	absent := cloneRecord(t, method)
	absent["resultPresent"] = false
	absent["resultPayload"] = nil
	absent["resultLength"] = 0
	expectShape(t, ms, "matched error-compatible absent result", absent, true)
	for _, field := range []string{"resultPayload", "resultLength"} {
		bad := cloneRecord(t, absent)
		if field == "resultLength" {
			bad[field] = 4
		} else {
			bad[field] = ref
		}
		expectShape(t, ms, "absent invalid "+field, bad, false)
	}
	r := map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 1}}
	occ := map[string]any{"relation": "RESOLVES_TO_DEFINITION", "occurrenceId": "o", "candidateId": "c", "queryId": "q", "direction": "QUERY_TO_TARGET", "shape": "LOCATION_LINK_ARRAY", "ordinal": 0, "targetUri": "file:///tmp/a.go", "targetRange": r, "targetSelectionRange": r, "targetSource": source, "authority": 0, "accepted": false, "completeness": "UNKNOWN", "authentication": "NO_PRODUCER_AUTHENTICATION"}
	os := mustCompile(t, c, id+"#/$defs/occurrence")
	expectShape(t, os, "link occurrence", occ, true)
	bad := cloneRecord(t, occ)
	bad["targetSelectionRange"] = nil
	expectShape(t, os, "link null selection", bad, false)
	bad["shape"] = "LOCATION"
	expectShape(t, os, "location null selection", bad, true)
	prefix := map[string]any{"version": "definition.wire-prefix.v1", "method": ref, "requestKey": "k", "writeCompleted": true, "contentLength": 10, "prefixLength": 1, "prefix": ref, "readPhase": "BODY", "cause": "TIMEOUT", "privacy": "PRIVATE_RETAINED", "matchedIdObserved": false, "completeMembers": []any{}}
	ps := mustCompile(t, c, id+"#/$defs/wirePrefix")
	expectShape(t, ps, "retained prefix", prefix, true)
	bad = cloneRecord(t, prefix)
	bad["prefix"] = nil
	expectShape(t, ps, "retained null prefix", bad, false)
	bad["privacy"] = "WITHHELD"
	expectShape(t, ps, "withheld null prefix", bad, true)
	bad["prefix"] = ref
	expectShape(t, ps, "withheld retained prefix", bad, false)
}

func TestADR0011DefinitionDraftRecordRoles(t *testing.T) {
	c, id := draftSchema(t, "adr0011-definition-records.proposed.schema.json")
	for _, role := range []string{"capability", "query", "method", "evaluation", "wirePrefix", "terminal", "occurrences", "dependencies"} {
		mustCompile(t, c, id+"#/$defs/"+role)
	}
	capability := mustCompile(t, c, id+"#/$defs/capability")
	digest := "sha256:" + strings.Repeat("0", 64)
	value := map[string]any{
		"version": "definition.capability.v1", "session": map[string]any{"id": "s", "generation": 1, "workspaceUri": "file:///tmp"},
		"ready": true, "provider": "gopls", "profile": "GO_GOPLS_DEFINITION_EXACT_V1", "executable": "gopls",
		"encoding": "utf-16", "definitionSupported": true, "initializeEvidence": map[string]any{"selector": "s", "digest": digest},
		"policy": map[string]any{"id": "P", "digest": digest},
	}
	expectShape(t, capability, "capability", value, true)
	value["unknown"] = true
	expectShape(t, capability, "unknown capability field", value, false)
}
