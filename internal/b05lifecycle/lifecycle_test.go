package b05lifecycle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	providerID = "ember-glint@1"
	protocolID = "lsp-trace.provider-observations@1"
	requestID  = "b05-production-lifecycle-1"
)

type lifecycleContract struct {
	SchemaVersion            string   `json:"schema_version"`
	ProviderIdentity         string   `json:"provider_identity"`
	ProtocolIdentity         string   `json:"protocol_identity"`
	RequestIdentity          string   `json:"request_identity"`
	BootstrapExample         string   `json:"bootstrap_example"`
	ProductionBinary         string   `json:"production_binary"`
	RetainedEvidence         string   `json:"retained_evidence"`
	ExpectedObservationKinds []string `json:"expected_observation_kinds"`
	ProhibitedClaims         []string `json:"prohibited_claims"`
	Requirements             []string `json:"requirements"`
}

type retainedLifecycleEvidence struct {
	SchemaVersion       string          `json:"schema_version"`
	Outcome             string          `json:"outcome"`
	ProviderIdentity    string          `json:"provider_identity"`
	ProtocolIdentity    string          `json:"protocol_identity"`
	RequestIdentity     string          `json:"request_identity"`
	ProductionBinary    string          `json:"production_binary"`
	ResponseSHA256      string          `json:"response_sha256"`
	ObservationKinds    []string        `json:"observation_kinds"`
	Coverage            json.RawMessage `json:"coverage"`
	OriginalCustody     json.RawMessage `json:"original_custody"`
	VirtualCustody      json.RawMessage `json:"virtual_custody"`
	ContributorIDs      []string        `json:"contributor_ids"`
	ProhibitedClaims    []string        `json:"prohibited_claims"`
	GraphV3OmissionWant json.RawMessage `json:"graph_v3_omission_expected"`
	GraphV3OmissionGot  json.RawMessage `json:"graph_v3_omission_observed"`
}

func TestProductionEmberProviderCompletesManagedB05Lifecycle(t *testing.T) {
	for _, assertion := range []string{
		"ASSERT_B05_PRODUCTION_PROVIDER_BINARY",
		"ASSERT_B05_BOOTSTRAP_VALIDATION_EXAMPLE",
		"ASSERT_B05_PRODUCTION_PROVIDER_EXECUTED",
		"ASSERT_B05_DETERMINISTIC_REPLAY",
		"ASSERT_B05_EXACT_PROVIDER_PROTOCOL_REQUEST_IDENTITY",
		"ASSERT_B05_IMMUTABLE_ORIGINAL_VIRTUAL_CUSTODY",
		"ASSERT_B05_EXPECTED_QUALIFIED_OBSERVATION_KINDS",
		"ASSERT_B05_EXPLICIT_COVERAGE",
		"ASSERT_B05_PROHIBITED_CLAIMS_ABSENT",
		"ASSERT_B05_ALL_CONTRIBUTOR_IDS",
		"ASSERT_B05_GRAPH_V3_OMISSION_PARITY",
		"ASSERT_B05_RETAINED_PRODUCTION_LIFECYCLE_EVIDENCE",
	} {
		t.Log("ASSERTION: " + assertion)
	}

	root := repositoryRoot(t)
	contract := loadContract(t, root)
	validateBootstrapExample(t, root, contract)

	binary := os.Getenv("LSP_TRACE_EMBER_PROVIDER_BINARY")
	if binary == "" {
		t.Fatal("ASSERT_B05_PRODUCTION_PROVIDER_BINARY: LSP_TRACE_EMBER_PROVIDER_BINARY is required")
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		t.Fatalf("ASSERT_B05_PRODUCTION_PROVIDER_BINARY: %v", err)
	}
	if info, err := os.Stat(absolute); err != nil || info.Size() == 0 || info.Mode()&0111 == 0 {
		t.Fatalf("ASSERT_B05_PRODUCTION_PROVIDER_BINARY: production executable unavailable: %v", err)
	}
	clean := filepath.ToSlash(filepath.Clean(absolute))
	if strings.Contains(clean, "/testdata/") || strings.Contains(strings.ToLower(filepath.Base(clean)), "fake") || filepath.Base(clean) != contract.ProductionBinary {
		t.Fatalf("ASSERT_B05_PRODUCTION_PROVIDER_BINARY: not release production binary: %s", clean)
	}

	request := providerRequest(t)
	first := executeProvider(t, absolute, request)
	second := executeProvider(t, absolute, request)
	if !bytes.Equal(first, second) {
		t.Fatalf("ASSERT_B05_DETERMINISTIC_REPLAY: response bytes differ")
	}
	validateProviderEnvelope(t, first, contract)
	executeManagedMCPLifecycle(t, root, absolute, contract)

	evidencePath := filepath.Join(root, filepath.FromSlash(contract.RetainedEvidence))
	rawEvidence, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("ASSERT_B05_RETAINED_PRODUCTION_LIFECYCLE_EVIDENCE: %v", err)
	}
	var evidence retainedLifecycleEvidence
	strictDecode(t, "ASSERT_B05_RETAINED_PRODUCTION_LIFECYCLE_EVIDENCE", rawEvidence, &evidence)
	validateRetainedEvidence(t, evidence, first, absolute, contract)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func loadContract(t *testing.T, root string) lifecycleContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "internal/b05lifecycle/testdata/production-lifecycle-evidence.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract lifecycleContract
	strictDecode(t, "ASSERT_B05_RETAINED_PRODUCTION_LIFECYCLE_EVIDENCE", raw, &contract)
	if contract.SchemaVersion != "lsp-trace.b05-production-lifecycle-contract.v1" || contract.ProviderIdentity != providerID || contract.ProtocolIdentity != protocolID || contract.RequestIdentity != requestID {
		t.Fatalf("ASSERT_B05_EXACT_PROVIDER_PROTOCOL_REQUEST_IDENTITY: %+v", contract)
	}
	return contract
}

func validateBootstrapExample(t *testing.T, root string, contract lifecycleContract) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(contract.BootstrapExample)))
	if err != nil {
		t.Fatalf("ASSERT_B05_BOOTSTRAP_VALIDATION_EXAMPLE: %v", err)
	}
	var bootstrap struct {
		Version   int               `json:"version"`
		Processes []json.RawMessage `json:"processes"`
		Providers []struct {
			SchemaVersion string                         `json:"schema_version"`
			Identity      string                         `json:"identity"`
			Version       string                         `json:"version"`
			Protocol      struct{ Name, Version string } `json:"protocol"`
			Execution     struct {
				Path      string `json:"path"`
				Directory string `json:"directory"`
			} `json:"execution"`
		} `json:"providers"`
	}
	strictDecode(t, "ASSERT_B05_BOOTSTRAP_VALIDATION_EXAMPLE", raw, &bootstrap)
	if bootstrap.Version != 1 || len(bootstrap.Processes) != 1 || len(bootstrap.Providers) != 1 || bootstrap.Providers[0].Identity+"@"+bootstrap.Providers[0].Version != providerID || bootstrap.Providers[0].Protocol.Name+"@"+bootstrap.Providers[0].Protocol.Version != protocolID || bootstrap.Providers[0].Execution.Path != "${LSP_TRACE_EMBER_PROVIDER_BINARY}" || bootstrap.Providers[0].Execution.Directory != "${B05_WORKSPACE}" {
		t.Fatalf("ASSERT_B05_BOOTSTRAP_VALIDATION_EXAMPLE: invalid production declaration: %+v", bootstrap)
	}
}

func providerRequest(t *testing.T) []byte {
	t.Helper()
	request := map[string]any{
		"schema_version": "lsp-trace.provider-request.v1", "request_id": requestID,
		"provider_id": providerID, "protocol": map[string]any{"name": "lsp-trace.provider-observations", "version": "1"},
		"session":   map[string]any{"session_id": "b05-production", "generation": 1},
		"seed":      map[string]any{"uri": "file:///workspace/app/components/uploader.gts"},
		"relations": []string{"BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK", "TRIGGERS_RELOAD", "UPDATES_STATE", "RENDERS_FROM"},
		"documents": map[string]any{"original_uri": "file:///workspace/app/components/uploader.gts", "workspace_revision": map[string]any{"kind": "git", "value": strings.Repeat("1", 40), "custody": "CALLER_ASSERTED"}},
		"limits":    map[string]any{"max_nodes": 100, "max_bytes": 1048576, "timeout_ms": 30000},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func executeProvider(t *testing.T, binary string, request []byte) []byte {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Stdin = bytes.NewReader(request)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ASSERT_B05_PRODUCTION_PROVIDER_EXECUTED: %v stderr=%s", err, stderr.String())
	}
	lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n"))
	if len(lines) != 1 || len(lines[0]) == 0 {
		t.Fatalf("ASSERT_B05_PRODUCTION_PROVIDER_EXECUTED: expected one response, got %d", len(lines))
	}
	return append([]byte(nil), lines[0]...)
}

func executeManagedMCPLifecycle(t *testing.T, root, providerBinary string, contract lifecycleContract) {
	t.Helper()
	binDir := t.TempDir()
	build := func(name, packagePath string) string {
		t.Helper()
		output := filepath.Join(binDir, name)
		cmd := exec.Command("go", "build", "-trimpath", "-o", output, packagePath)
		cmd.Dir = root
		if combined, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ASSERT_B05_PRODUCTION_PROVIDER_EXECUTED: build %s: %v output=%s", packagePath, err, combined)
		}
		return output
	}
	mcpBinary := build("lsp-trace-mcp", "./cmd/lsp-trace-mcp")
	languageServer := build("b05-language-server-fixture", "./cmd/fake-lsp")
	workspace := t.TempDir()
	config := map[string]any{
		"version": 1,
		"processes": []any{map[string]any{
			"alias":     "ember-b05",
			"profile":   map[string]any{"trust_domain": "b05-production-lifecycle", "workspace": workspace, "profile": "ember-glint", "environment_reference": "release-qualification"},
			"execution": map[string]any{"path": languageServer, "directory": workspace},
		}},
		"providers": []any{map[string]any{
			"schema_version": "lsp-trace.bootstrap-provider.v1", "identity": providerID, "version": "1",
			"protocol":     map[string]any{"name": "lsp-trace.provider-observations", "version": "1"},
			"execution":    map[string]any{"path": providerBinary, "directory": workspace},
			"capabilities": map[string]any{"relations": contract.ExpectedObservationKinds, "languages": []string{"javascript", "typescript", "handlebars"}, "frameworks": []string{"ember", "glimmer"}},
			"limits":       map[string]any{"request_bytes": 1048576, "response_bytes": 8388608, "protocol_messages": 2, "stderr_bytes": 65536, "wall_time_ms": 30000, "termination_grace_ms": 1000},
		}},
	}
	configBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "bootstrap.json")
	if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	arguments := map[string]any{
		"session_id": "ember-b05", "generation": 1, "uri": "file:///workspace/app/components/uploader.gts", "line": 0, "character": 0,
		"max_depth": 2, "max_nodes": 100, "timeout_ms": 30000, "request_timeout_ms": 30000,
		"relations": contract.ExpectedObservationKinds, "providers": []string{providerID},
		"workspace_revision": map[string]any{"kind": "git", "commit": strings.Repeat("1", 40), "custody": "CALLER_ASSERTED"},
	}
	call := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "lsp_trace_v1_incoming", "arguments": arguments}}
	callBytes, _ := json.Marshal(call)
	cmd := exec.Command(mcpBinary, "--bootstrap-config", configPath)
	cmd.Stdin = bytes.NewReader(append(callBytes, '\n'))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ASSERT_B05_PRODUCTION_PROVIDER_EXECUTED: managed MCP lifecycle: %v stderr=%s", err, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"operation_status":"SUCCEEDED"`)) || !bytes.Contains(stdout.Bytes(), []byte(providerID)) {
		t.Fatalf("ASSERT_B05_PRODUCTION_PROVIDER_EXECUTED: managed MCP response lacks successful production-provider evidence: %s", stdout.String())
	}
}

func validateProviderEnvelope(t *testing.T, raw []byte, contract lifecycleContract) {
	t.Helper()
	var envelope struct {
		Provider     struct{ Name, Version string } `json:"provider"`
		Protocol     struct{ Name, Version string } `json:"protocol"`
		RequestID    string                         `json:"request_id"`
		Coverage     any                            `json:"coverage"`
		Documents    []any                          `json:"documents"`
		Observations []struct {
			ID             string   `json:"id"`
			Kind           string   `json:"kind"`
			DoesNotSupport []string `json:"does_not_support"`
		} `json:"observations"`
	}
	strictDecode(t, "ASSERT_B05_PRODUCTION_PROVIDER_EXECUTED", raw, &envelope)
	if envelope.Provider.Name+"@"+envelope.Provider.Version != contract.ProviderIdentity || envelope.Protocol.Name+"@"+envelope.Protocol.Version != contract.ProtocolIdentity || envelope.RequestID != contract.RequestIdentity {
		t.Fatalf("ASSERT_B05_EXACT_PROVIDER_PROTOCOL_REQUEST_IDENTITY: provider=%+v protocol=%+v request=%q", envelope.Provider, envelope.Protocol, envelope.RequestID)
	}
	if envelope.Coverage == nil {
		t.Fatal("ASSERT_B05_EXPLICIT_COVERAGE: coverage absent")
	}
	if len(envelope.Documents) < 2 {
		t.Fatal("ASSERT_B05_IMMUTABLE_ORIGINAL_VIRTUAL_CUSTODY: original and virtual documents required")
	}
	kinds, contributors := []string{}, []string{}
	for _, observation := range envelope.Observations {
		kinds = append(kinds, observation.Kind)
		if observation.ID == "" {
			t.Fatal("ASSERT_B05_ALL_CONTRIBUTOR_IDS: observation id absent")
		}
		contributors = append(contributors, observation.ID)
		for _, prohibited := range contract.ProhibitedClaims {
			if !contains(observation.DoesNotSupport, prohibited) {
				t.Fatalf("ASSERT_B05_PROHIBITED_CLAIMS_ABSENT: observation %q omits %q non-entailment", observation.ID, prohibited)
			}
		}
	}
	if !sameSet(kinds, contract.ExpectedObservationKinds) {
		t.Fatalf("ASSERT_B05_EXPECTED_QUALIFIED_OBSERVATION_KINDS: got=%v want=%v", kinds, contract.ExpectedObservationKinds)
	}
	if len(contributors) != len(envelope.Observations) {
		t.Fatal("ASSERT_B05_ALL_CONTRIBUTOR_IDS: incomplete contributor identities")
	}
}

func validateRetainedEvidence(t *testing.T, evidence retainedLifecycleEvidence, response []byte, binary string, contract lifecycleContract) {
	t.Helper()
	digest := sha256.Sum256(response)
	if evidence.SchemaVersion != "lsp-trace.b05-production-lifecycle-evidence.v1" || evidence.Outcome != "PASS" || evidence.ProviderIdentity != providerID || evidence.ProtocolIdentity != protocolID || evidence.RequestIdentity != requestID || evidence.ProductionBinary != filepath.Base(binary) {
		t.Fatalf("ASSERT_B05_RETAINED_PRODUCTION_LIFECYCLE_EVIDENCE: identity/outcome mismatch: %+v", evidence)
	}
	if evidence.ResponseSHA256 != "sha256:"+hex.EncodeToString(digest[:]) {
		t.Fatal("ASSERT_B05_DETERMINISTIC_REPLAY: retained response digest differs")
	}
	if !sameSet(evidence.ObservationKinds, contract.ExpectedObservationKinds) {
		t.Fatal("ASSERT_B05_EXPECTED_QUALIFIED_OBSERVATION_KINDS: retained kinds differ")
	}
	if len(evidence.Coverage) == 0 {
		t.Fatal("ASSERT_B05_EXPLICIT_COVERAGE: retained coverage absent")
	}
	if len(evidence.OriginalCustody) == 0 || len(evidence.VirtualCustody) == 0 {
		t.Fatal("ASSERT_B05_IMMUTABLE_ORIGINAL_VIRTUAL_CUSTODY: retained custody absent")
	}
	if len(evidence.ContributorIDs) == 0 {
		t.Fatal("ASSERT_B05_ALL_CONTRIBUTOR_IDS: retained contributor ids absent")
	}
	if !sameSet(evidence.ProhibitedClaims, contract.ProhibitedClaims) {
		t.Fatal("ASSERT_B05_PROHIBITED_CLAIMS_ABSENT: retained prohibition set differs")
	}
	if len(evidence.GraphV3OmissionWant) == 0 || !bytes.Equal(evidence.GraphV3OmissionWant, evidence.GraphV3OmissionGot) {
		t.Fatal("ASSERT_B05_GRAPH_V3_OMISSION_PARITY: exact retained graph-v3 bytes differ")
	}
}

func strictDecode(t *testing.T, assertion string, raw []byte, target any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	if decoder.More() {
		t.Fatalf("%s: trailing JSON", assertion)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sameSet(got, want []string) bool {
	got, want = append([]string(nil), got...), append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	return reflect.DeepEqual(got, want)
}

func Example_bootstrapProductionEmberProvider() {
	fmt.Println("LSP_TRACE_EMBER_PROVIDER_BINARY=/release/lsp-trace-provider-ember-glint lsp-trace-mcp --bootstrap-config internal/b05lifecycle/testdata/bootstrap.production.example.json")
	// Output:
	// LSP_TRACE_EMBER_PROVIDER_BINARY=/release/lsp-trace-provider-ember-glint lsp-trace-mcp --bootstrap-config internal/b05lifecycle/testdata/bootstrap.production.example.json
}
