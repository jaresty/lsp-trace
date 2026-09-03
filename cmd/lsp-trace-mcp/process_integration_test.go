package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/verification"
)

const (
	resultEnvelopeID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-result.v1.schema.json"
	artifactEnvelopeID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-artifact.v1.schema.json"
	domainEnvelopeID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-domain-error.v1.schema.json"
)

type processCall struct {
	result map[string]any
	env    map[string]any
}

func executableName(name, goos string) string {
	if goos == "windows" && !strings.HasSuffix(name, ".exe") {
		return name + ".exe"
	}
	return name
}

func TestExecutableNameMatchesTargetPlatform(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, goos, want string
	}{
		{name: "lsp-trace", goos: "windows", want: "lsp-trace.exe"},
		{name: "lsp-trace-mcp.exe", goos: "windows", want: "lsp-trace-mcp.exe"},
		{name: "lsp-trace", goos: "darwin", want: "lsp-trace"},
		{name: "lsp-trace-mcp", goos: "linux", want: "lsp-trace-mcp"},
	} {
		t.Run(tt.name+"/"+tt.goos, func(t *testing.T) {
			if got := executableName(tt.name, tt.goos); got != tt.want {
				t.Errorf("ASSERT_PLATFORM_EXECUTABLE_NAME: executableName(%q, %q)=%q want=%q", tt.name, tt.goos, got, tt.want)
			}
		})
	}
}

func buildBinary(t *testing.T, name, packagePath string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("ASSERT_REAL_PROCESS_BINARY: caller path unavailable")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	binary := filepath.Join(t.TempDir(), executableName(name, runtime.GOOS))
	cmd := exec.Command("go", "build", "-o", binary, packagePath)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ASSERT_REAL_PROCESS_BINARY: build %s: %v\n%s", packagePath, err, out)
	}
	return binary
}

func buildMCPBinary(t *testing.T) string {
	return buildBinary(t, "lsp-trace-mcp", "./cmd/lsp-trace-mcp")
}

func runCLIProcess(t *testing.T, binary string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ASSERT_REAL_CLI_PROCESS: args=%q err=%v stderr=%s", args, err, stderr.String())
	}
	return stdout.Bytes()
}

func runMCPProcess(t *testing.T, binary string, args []string, requests []map[string]any) []map[string]any {
	t.Helper()
	var stdin bytes.Buffer
	enc := json.NewEncoder(&stdin)
	enc.SetEscapeHTML(false)
	for _, request := range requests {
		if err := enc.Encode(request); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdin = &stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ASSERT_REAL_PROCESS_STDIO: run: %v stderr=%s", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != len(requests) {
		t.Fatalf("ASSERT_REAL_PROCESS_STDIO: responses=%d requests=%d stdout=%q", len(lines), len(requests), stdout.String())
	}
	responses := make([]map[string]any, len(lines))
	for i, line := range lines {
		if err := json.Unmarshal([]byte(line), &responses[i]); err != nil {
			t.Fatalf("ASSERT_REAL_PROCESS_STDIO: response[%d]=%q: %v", i, line, err)
		}
	}
	return responses
}

func callRequest(id int, name string, arguments map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}}
}

func decodeProcessCall(t *testing.T, response map[string]any) processCall {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("ASSERT_REAL_PROCESS_CALL_RESULT: response=%v", response)
	}
	env, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("ASSERT_REAL_PROCESS_CALL_RESULT: result=%v", result)
	}
	if content, ok := result["content"].([]any); !ok || len(content) != 0 {
		t.Fatalf("ASSERT_EMPTY_OUTER_CONTENT: result=%v", result)
	}
	if result["isError"] != env["isError"] {
		t.Fatalf("ASSERT_EQUAL_ERROR_STATE: outer=%v envelope=%v", result["isError"], env["isError"])
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := mcpcontract.ValidateEnvelopeExclusive(raw); err != nil {
		t.Fatalf("ASSERT_MANIFEST_SELECTED_ENVELOPE: %v envelope=%s", err, raw)
	}
	return processCall{result: result, env: env}
}

func graphFixture(t *testing.T) []byte {
	t.Helper()
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV3,
		Invocation: graph.Invocation{Seeds: []graph.InvocationSeed{
			{Label: "first", At: "first.go:1:1"},
			{Label: "second", At: "second.go:1:1"},
		}},
		Seeds: []graph.SeedResult{
			{Label: "first", PreparedTargetIDs: []string{}, ReachedNodeIDs: []string{}, ReachedRelationIDs: []string{}},
			{Label: "second", PreparedTargetIDs: []string{}, ReachedNodeIDs: []string{}, ReachedRelationIDs: []string{}},
		},
		Summary: graph.Summary{Complete: true},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func custodyFixture(t *testing.T, artifact []byte) string {
	t.Helper()
	dir := t.TempDir()
	generation := "generation-1"
	generationDir := filepath.Join(dir, generation)
	if err := os.Mkdir(generationDir, 0700); err != nil {
		t.Fatal(err)
	}
	receipt, err := verification.ReceiptBytes(artifact, verification.DirectoryDurabilityChecked)
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string][]byte{
		filepath.Join(dir, "selector.json"):           []byte(`{"generation":"` + generation + `"}` + "\n"),
		filepath.Join(generationDir, "artifact.json"): artifact,
		filepath.Join(generationDir, "receipt.json"):  receipt,
	} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "selector.json")
}

func inlineArtifactBytes(t *testing.T, env map[string]any) []byte {
	t.Helper()
	content, ok := env["content"].(string)
	if !ok {
		t.Fatalf("ASSERT_INLINE_ARTIFACT_CONTENT: envelope=%v", env)
	}
	return []byte(content)
}

func TestRealProcessSixOfflineCanonicalAndAliasConformance(t *testing.T) {
	const assertion = "all six production offline handlers execute through the real stdio MCP process with canonical alias equivalence"
	t.Log("ASSERTION: " + assertion)
	binary := buildMCPBinary(t)
	cliBinary := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	graphBytes := graphFixture(t)
	selectorPath := custodyFixture(t, graphBytes)
	fixtureDir := t.TempDir()
	graphPath := filepath.Join(fixtureDir, "graph.json")
	if err := os.WriteFile(graphPath, graphBytes, 0600); err != nil {
		t.Fatal(err)
	}
	inspectionBytes := runCLIProcess(t, cliBinary, "inspect", graphPath, "--all-seeds", "--json")
	inspectionPath := filepath.Join(fixtureDir, "inspection.json")
	if err := os.WriteFile(inspectionPath, inspectionBytes, 0600); err != nil {
		t.Fatal(err)
	}
	filterBytes := runCLIProcess(t, cliBinary, "filter", inspectionPath, "--compare-seeds", "first", "--compare-seeds", "second", "--json")
	schemaBytes := runCLIProcess(t, cliBinary, "schema", "get", "--family", "graph", "--version", "v1")

	validV1 := map[string]any{
		"schema_version": "lsp-trace.graph.v1", "invocation": map[string]any{}, "capabilities": map[string]any{},
		"targets": []any{}, "nodes": []any{}, "edges": []any{}, "terminals": []any{}, "frontier": []any{}, "diagnostics": []any{}, "summary": map[string]any{},
	}
	validBytes := mustJSON(t, validV1)
	validPath := filepath.Join(fixtureDir, "valid-v1.json")
	if err := os.WriteFile(validPath, validBytes, 0600); err != nil {
		t.Fatal(err)
	}
	_ = runCLIProcess(t, cliBinary, "validate", "--family", "graph", "--version", "v1", validPath)
	_ = runCLIProcess(t, cliBinary, "verify", selectorPath)

	cases := []struct {
		canonical, alias, envelopeID string
		arguments                    map[string]any
		wantArtifact                 []byte
		artifactSchemaID             string
	}{
		{"lsp_trace_v1_capabilities", "lsp_trace_capabilities", resultEnvelopeID, map[string]any{}, nil, ""},
		{"lsp_trace_v1_schema_get", "lsp_trace_schema_get", artifactEnvelopeID, map[string]any{"schema": map[string]any{"family": "graph", "version": "v1"}}, schemaBytes, "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v1.schema.json"},
		{"lsp_trace_v1_validate", "lsp_trace_validate", artifactEnvelopeID, map[string]any{"input": validV1, "schema": map[string]any{"family": "graph", "version": "v1"}}, validBytes, "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v1.schema.json"},
		{"lsp_trace_v1_verify", "lsp_trace_verify", artifactEnvelopeID, map[string]any{"input": selectorPath}, graphBytes, "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v3.schema.json"},
		{"lsp_trace_v1_inspect", "lsp_trace_inspect", artifactEnvelopeID, map[string]any{"input": string(graphBytes), "selector": map[string]any{"all_seeds": true}}, inspectionBytes, "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.inspect.v1.schema.json"},
		{"lsp_trace_v1_filter", "lsp_trace_filter", artifactEnvelopeID, map[string]any{"input": string(inspectionBytes), "filter": map[string]any{"compare_seeds": []string{"first", "second"}}}, filterBytes, "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.filter.v1.schema.json"},
	}

	requests := make([]map[string]any, 0, len(cases)*2)
	for i, tc := range cases {
		requests = append(requests, callRequest(i*2+1, tc.canonical, tc.arguments), callRequest(i*2+2, tc.alias, tc.arguments))
	}
	responses := runMCPProcess(t, binary, nil, requests)
	for i, tc := range cases {
		canonical := decodeProcessCall(t, responses[i*2])
		alias := decodeProcessCall(t, responses[i*2+1])
		for spelling, call := range map[string]processCall{"canonical": canonical, "alias": alias} {
			if call.env["tool"] != tc.canonical || call.env["envelope_schema_id"] != tc.envelopeID || call.env["outcome"] != "COMPLETE" || call.env["operation_status"] != "SUCCEEDED" || call.env["isError"] != false {
				t.Errorf("%s: %s %s envelope=%v", assertion, tc.canonical, spelling, call.env)
			}
		}
		canonicalComparable := cloneWithoutRequestID(canonical.env)
		aliasComparable := cloneWithoutRequestID(alias.env)
		if !equalJSON(canonicalComparable, aliasComparable) {
			t.Errorf("ASSERT_CANONICAL_ALIAS_EQUIVALENCE: %s canonical=%v alias=%v", tc.canonical, canonical.env, alias.env)
		}
		if tc.wantArtifact != nil {
			got := inlineArtifactBytes(t, canonical.env)
			if !bytes.Equal(got, tc.wantArtifact) {
				t.Errorf("ASSERT_MCP_CLI_ARTIFACT_BYTE_PARITY: %s\ngot:  %q\nwant: %q", tc.canonical, got, tc.wantArtifact)
			}
			if canonical.env["artifact_schema_id"] != tc.artifactSchemaID {
				t.Errorf("ASSERT_EXACT_ARTIFACT_SCHEMA_ID: %s got=%v want=%s", tc.canonical, canonical.env["artifact_schema_id"], tc.artifactSchemaID)
			}
		}
	}
}

func TestRealProcessDomainErrorsAreManifestConformant(t *testing.T) {
	const assertion = "real-process operation failures use manifest-selected domain envelopes and aliases preserve canonical identity"
	t.Log("ASSERTION: " + assertion)
	binary := buildMCPBinary(t)
	graphBytes := graphFixture(t)
	requests := []map[string]any{
		callRequest(1, "lsp_trace_v1_inspect", map[string]any{"input": string(graphBytes), "selector": map[string]any{"seed": "missing"}}),
		callRequest(2, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "future", "version": "v1"}}),
		callRequest(3, "lsp_trace_schema_get", map[string]any{"schema": map[string]any{"family": "future", "version": "v1"}}),
		callRequest(4, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v1"}, "output_selector": "disabled.json"}),
	}
	responses := runMCPProcess(t, binary, nil, requests)
	calls := make([]processCall, len(responses))
	for i, response := range responses {
		calls[i] = decodeProcessCall(t, response)
		if calls[i].env["envelope_schema_id"] != domainEnvelopeID || calls[i].env["outcome"] != "DOMAIN_ERROR" || calls[i].env["operation_status"] != "FAILED" || calls[i].env["isError"] != true {
			t.Errorf("%s: response[%d]=%v", assertion, i, calls[i].env)
		}
		if _, hasContent := calls[i].env["content"]; hasContent {
			t.Errorf("ASSERT_ERROR_HAS_NO_ARTIFACT: response[%d]=%v", i, calls[i].env)
		}
	}
	if calls[0].env["code"] != "INPUT_INVALID" || calls[1].env["code"] != "INPUT_FAMILY_MISMATCH" || calls[3].env["code"] != "OUTPUT_SELECTOR_UNSAFE" {
		t.Errorf("ASSERT_DOMAIN_ERROR_CLASSIFICATION: calls=%v", calls)
	}
	if calls[1].env["tool"] != "lsp_trace_v1_schema_get" || !equalJSON(cloneWithoutRequestID(calls[1].env), cloneWithoutRequestID(calls[2].env)) {
		t.Errorf("ASSERT_CANONICAL_ALIAS_ERROR_EQUIVALENCE: canonical=%v alias=%v", calls[1].env, calls[2].env)
	}
}

func TestRealProcessPublicationConfigurationAndLaterStagesReserved(t *testing.T) {
	const assertion = "selector publication capability is conditional on a configured pinned root while later stages remain NOT_IMPLEMENTED regardless of live flag"
	t.Log("ASSERTION: " + assertion)
	binary := buildMCPBinary(t)
	for _, args := range [][]string{nil, {"--enable-live-lsp"}} {
		responses := runMCPProcess(t, binary, args, []map[string]any{
			callRequest(1, "lsp_trace_capabilities", map[string]any{}),
			callRequest(2, "lsp_trace_incoming", map[string]any{}),
			callRequest(3, "lsp_session_v1_list", map[string]any{}),
		})
		capability := decodeProcessCall(t, responses[0]).env["result"].(map[string]any)
		if capability["selector_publication_supported"] != false {
			t.Errorf("ASSERT_PUBLICATION_DISABLED_WITHOUT_ROOT: args=%v result=%v", args, capability)
		}
		for _, response := range responses[1:] {
			call := decodeProcessCall(t, response)
			if call.env["code"] != "TOOL_NOT_IMPLEMENTED" || call.env["operation_status"] != "FAILED" || call.env["outcome"] != "DOMAIN_ERROR" || call.env["isError"] != true {
				t.Errorf("%s: args=%v envelope=%v", assertion, args, call.env)
			}
		}
	}
}

func TestRealProcessConfiguredRootPublication(t *testing.T) {
	const assertion = "configured-root canonical and alias calls publish exact bytes without overwrite and emit schema-valid path-redacted receipts and failures"
	t.Log("ASSERTION: " + assertion)
	binary := buildMCPBinary(t)
	cliBinary := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	root := t.TempDir()
	expected := runCLIProcess(t, cliBinary, "schema", "get", "--family", "graph", "--version", "v1")
	occupied := []byte("caller-owned")
	if err := os.WriteFile(filepath.Join(root, "occupied.json"), occupied, 0600); err != nil {
		t.Fatal(err)
	}
	secret := "secret-selector"
	requests := []map[string]any{
		callRequest(1, "lsp_trace_v1_capabilities", map[string]any{}),
		callRequest(2, "lsp_trace_capabilities", map[string]any{}),
		callRequest(3, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v1"}, "output_selector": "canonical.json"}),
		callRequest(4, "lsp_trace_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v1"}, "output_selector": "alias.json"}),
		callRequest(5, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v1"}, "output_selector": "nested/output.json"}),
		callRequest(6, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v1"}, "output_selector": "occupied.json"}),
		callRequest(7, "lsp_trace_v1_schema_get", map[string]any{"schema": map[string]any{"family": "graph", "version": "v1"}, "output_selector": "../" + secret}),
	}
	responses := runMCPProcess(t, binary, []string{"--publication-root", root}, requests)

	for _, index := range []int{0, 1} {
		capability := decodeProcessCall(t, responses[index]).env["result"].(map[string]any)
		if capability["selector_publication_supported"] != true {
			t.Errorf("ASSERT_PUBLICATION_ENABLED_WITH_ROOT: response[%d]=%v", index, capability)
		}
	}
	published := make([]processCall, 3)
	for i := range published {
		published[i] = decodeProcessCall(t, responses[i+2])
		if published[i].env["outcome"] != "COMPLETE" || published[i].env["operation_status"] != "SUCCEEDED" || published[i].env["isError"] != false {
			t.Errorf("ASSERT_PUBLICATION_SUCCESS: response[%d]=%v", i+2, published[i].env)
		}
		if _, ok := published[i].env["content"]; ok {
			t.Errorf("ASSERT_SELECTOR_EXCLUDES_INLINE_CONTENT: response[%d]=%v", i+2, published[i].env)
		}
	}
	if !equalJSON(cloneWithoutRequestID(published[0].env), cloneWithoutRequestID(published[1].env)) {
		t.Errorf("ASSERT_CANONICAL_ALIAS_PUBLICATION_EQUIVALENCE: canonical=%v alias=%v", published[0].env, published[1].env)
	}
	sum := sha256.Sum256(expected)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	for i, call := range published {
		receipt, ok := call.env["publication_receipt"].(map[string]any)
		if !ok || receipt["artifact_digest"] != wantDigest || receipt["artifact_byte_length"] != float64(len(expected)) || receipt["artifact_schema_id"] != "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v1.schema.json" || receipt["publication_mechanism"] != "atomic_no_replace" {
			t.Errorf("ASSERT_EXACT_PATH_REDACTED_RECEIPT: response[%d]=%v", i+2, call.env)
		}
	}
	for _, selector := range []string{"canonical.json", "alias.json", "nested/output.json"} {
		got, err := os.ReadFile(filepath.Join(root, selector))
		if err != nil || !bytes.Equal(got, expected) {
			t.Errorf("ASSERT_PUBLISHED_BYTES: selector=%q err=%v got=%q want=%q", selector, err, got, expected)
		}
	}
	collision := decodeProcessCall(t, responses[5]).env
	if collision["outcome"] != "PUBLICATION_ERROR" || collision["operation_status"] != "FAILED" || collision["code"] != "PUBLICATION_FAILED" || collision["artifact_digest"] != wantDigest || collision["artifact_byte_length"] != float64(len(expected)) {
		t.Errorf("ASSERT_COLLISION_PUBLICATION_ERROR: envelope=%v", collision)
	}
	if got, err := os.ReadFile(filepath.Join(root, "occupied.json")); err != nil || !bytes.Equal(got, occupied) {
		t.Errorf("ASSERT_NO_OVERWRITE: err=%v got=%q want=%q", err, got, occupied)
	}
	unsafe := decodeProcessCall(t, responses[6]).env
	if unsafe["outcome"] != "DOMAIN_ERROR" || unsafe["code"] != "OUTPUT_SELECTOR_UNSAFE" {
		t.Errorf("ASSERT_UNSAFE_SELECTOR: envelope=%v", unsafe)
	}
	for i, response := range responses {
		raw, _ := json.Marshal(response)
		if bytes.Contains(raw, []byte(root)) || bytes.Contains(raw, []byte(secret)) {
			t.Errorf("ASSERT_NO_PATH_ECHO: response[%d]=%s", i, raw)
		}
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func cloneWithoutRequestID(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)-1)
	for key, value := range in {
		if key != "request_id" {
			out[key] = value
		}
	}
	return out
}

func equalJSON(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return bytes.Equal(left, right)
}
