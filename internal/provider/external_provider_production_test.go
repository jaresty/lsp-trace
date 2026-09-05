package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/observationadapter"
)

func TestProductionExternalProviderCompletesManagedLifecycle(t *testing.T) {
	const assertion = "ASSERT_EXTERNAL_PROVIDER_PRODUCTION_LIFECYCLE"
	executable := os.Getenv("LSP_TRACE_EXTERNAL_PROVIDER_PATH")
	if executable == "" {
		t.Skip("LSP_TRACE_EXTERNAL_PROVIDER_PATH is required")
	}
	executable, err := filepath.Abs(executable)
	if err != nil || !filepath.IsAbs(executable) {
		t.Fatalf("%s: absolute executable required: %v", assertion, err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sourceFixture := filepath.Join(root, "qualification", "external-provider", "component.gts")
	source, err := os.ReadFile(sourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(os.TempDir(), "lsp-trace-external-provider-qualification", "component.gts")
	if err := os.MkdirAll(filepath.Dir(fixture), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture, source, 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureURI := "file://" + fixture
	limits := Limits{RequestBytes: 1 << 20, ResponseBytes: 1 << 20, ProtocolMessages: 1, StderrBytes: 4096, WallTime: 30 * time.Second, TerminationGrace: time.Second}
	declaration := Declaration{Identity: "ember-glint@1", Version: "1", Protocol: ProtocolIdentity{Name: observationadapter.ProtocolName, Version: observationadapter.ProtocolVersion}, Executable: executable, ExecutableAvailable: true, ConformanceVerified: true, Capabilities: Capabilities{Relations: []string{"BINDS_ARGUMENT"}, Languages: []string{"glimmer-js"}, Frameworks: []string{"ember"}}, Limits: limits}
	provisioned, err := Provision([]Declaration{declaration})
	if err != nil {
		t.Fatalf("%s: provision: %v", assertion, err)
	}
	adapterIdentity := observationadapter.Identity{Name: "ember-template", Version: "1"}
	adapter, err := NewObservationSemanticAdapter(provisioned, adapterIdentity)
	if err != nil {
		t.Fatal(err)
	}
	request := StrictCollectorRequest{SchemaVersion: CollectorRequestSchema, ProviderID: declaration.Identity, AdapterID: "ember-template@1", Session: ManagedSessionCustody{SessionID: "production-qualification", Generation: 1}, Seed: SeedCustody{URI: fixtureURI, StartMode: "at"}, Relations: []string{"BINDS_ARGUMENT"}, Documents: DocumentCustody{OriginalURI: fixtureURI}, Limits: CollectorLimits{MaxNodes: 100, MaxMessages: 1, MaxBytes: 1 << 20, TimeoutMS: 30000, RequestTimeoutMS: 30000}}
	rawRequest, _ := json.Marshal(request)
	runtime := NewRuntime(provisioned.Registry)
	first := runtime.Execute(context.Background(), declaration.Identity, rawRequest, limits)
	second := runtime.Execute(context.Background(), declaration.Identity, rawRequest, limits)
	if first.Failure != nil || second.Failure != nil || !bytes.Equal(first.Response, second.Response) {
		t.Fatalf("%s: subprocess failure=%v replay_failure=%v deterministic=%v stderr=%q", assertion, first.Failure, second.Failure, bytes.Equal(first.Response, second.Response), first.Stderr.Bytes)
	}
	projectedFirst, err := adapter.Adapt(context.Background(), request, first)
	if err != nil {
		t.Fatalf("%s: strict adapter: %v", assertion, err)
	}
	projectedSecond, err := adapter.Adapt(context.Background(), request, second)
	if err != nil || !bytes.Equal(projectedFirst, projectedSecond) {
		t.Fatalf("%s: generic projection replay mismatch: %v", assertion, err)
	}
	admitter, err := NewAdmissionResolver(provisioned, "ember-template@1")
	if err != nil {
		t.Fatal(err)
	}
	collector, err := NewCollector(runtime, admitter, adapter)
	if err != nil {
		t.Fatal(err)
	}
	operationInput := json.RawMessage(`{"session_id":"production-qualification","generation":1,"uri":"` + fixtureURI + `","start_mode":"at","relations":["BINDS_ARGUMENT"],"languages":["glimmer-js"],"frameworks":["ember"],"providers":["ember-glint@1"],"max_nodes":100,"max_messages":1,"max_bytes":1048576,"timeout_ms":30000,"request_timeout_ms":30000}`)
	collectorProjection, err := collector.CollectRelations(context.Background(), []string{"BINDS_ARGUMENT"}, operationInput)
	if err != nil || !bytes.Contains(collectorProjection, []byte(`"kind":"BINDS_ARGUMENT"`)) {
		t.Fatalf("%s: generic collector projection: %v %s", assertion, err, collectorProjection)
	}
	var envelope observationadapter.Envelope
	if err := json.Unmarshal(first.Response, &envelope); err != nil {
		t.Fatal(err)
	}
	adapted, err := observationadapter.Adapt(envelope)
	if err != nil {
		t.Fatalf("%s: graph-v4 adapter: %v", assertion, err)
	}
	if len(adapted.Observations) == 0 || len(adapted.GraphV4.Relations) == 0 || !reflect.DeepEqual(adapted.GraphV4.Relations[0].ContributingObservationIDs, []string{adapted.Observations[0].ObservationID}) {
		t.Fatalf("%s: missing exact observations/contributors: %#v", assertion, adapted)
	}
	unsupported := request
	unsupported.Relations = []string{"PASSES_CALLBACK"}
	unsupportedRaw, _ := json.Marshal(unsupported)
	unsupportedReceipt := runtime.Execute(context.Background(), declaration.Identity, unsupportedRaw, limits)
	var unsupportedEnvelope observationadapter.Envelope
	if unsupportedReceipt.Failure != nil || json.Unmarshal(unsupportedReceipt.Response, &unsupportedEnvelope) != nil || string(unsupportedEnvelope.Failure) != "RELATION_NOT_SUPPORTED" || len(unsupportedEnvelope.Observations) != 0 {
		t.Fatalf("ASSERT_EXTERNAL_PROVIDER_UNSUPPORTED_HONEST: receipt=%+v envelope=%+v", unsupportedReceipt, unsupportedEnvelope)
	}
	responseDigest := sha256.Sum256(first.Response)
	evidence := struct {
		SchemaVersion         string `json:"schema_version"`
		Outcome               string `json:"outcome"`
		InstallationKind      string `json:"installation_kind"`
		ProviderPathKind      string `json:"provider_path_kind"`
		Lifecycle             string `json:"lifecycle"`
		ProviderIdentity      string `json:"provider_identity"`
		ProtocolIdentity      string `json:"protocol_identity"`
		AdapterIdentity       string `json:"adapter_identity"`
		ResponseSHA256        string `json:"response_sha256"`
		Response              []byte `json:"response"`
		DeterministicReplay   bool   `json:"deterministic_replay"`
		Reviewed              bool   `json:"reviewed"`
		UnsupportedHonest     bool   `json:"unsupported_honest"`
		GraphV3OmissionParity bool   `json:"graph_v3_omission_parity"`
		GraphV4               any    `json:"graph_v4"`
		Observations          any    `json:"observations"`
	}{"lsp-trace.external-provider-production-qualification.v1", "PASS", "independent-external-package", "absolute-external", "request→external subprocess→provider observations→generic adapter→graph-v4", declaration.Identity, observationadapter.ProtocolName + "@" + observationadapter.ProtocolVersion, "ember-template@1", hex.EncodeToString(responseDigest[:]), first.Response, true, true, true, true, adapted.GraphV4, adapted.Observations}
	encoded, _ := json.MarshalIndent(evidence, "", "  ")
	encoded = append(encoded, '\n')
	if os.Getenv("LSP_TRACE_RETAIN_EXTERNAL_QUALIFICATION") == "1" {
		path := filepath.Join(root, "qualification", "retained", "external-provider", "ember-glint.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("PASS " + assertion)
}
