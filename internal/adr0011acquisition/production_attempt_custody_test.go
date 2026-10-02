//go:build darwin

package adr0011acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

func productionFixtureOriginal(t *testing.T, root *publication.Root, role, schema, selector string, raw []byte) productionAttemptRef {
	t.Helper()
	receipt, err := publication.PublishBoundFileWithTrace(root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return errors.New("not equal")
		}
		return nil
	}, nil)
	if err != nil || receipt == nil || receipt.VerificationStatus != "VERIFIED" {
		t.Fatalf("fixture selected original: %v %+v", err, receipt)
	}
	return productionAttemptRef{role, schema, selector, privateDigest(raw), len(raw)}
}
func productionFixturePre(t *testing.T, root *publication.Root, sessionID string, generation uint64) productionAttemptPrewrite {
	t.Helper()
	d := privateDigest([]byte("fixture"))
	declaration, err := json.Marshal(map[string]any{
		"role": "ADR0011_PRODUCTION_QUERY_DECLARATION_V1", "version": 1, "plan_id": d, "session_id": sessionID, "generation": generation,
		"workspace_uri": "file:///fixture", "query_uri": "file:///fixture/main.go", "line": 1, "character": 2, "encoding": "utf-16",
		"document_version": 1, "document_byte_length": 10, "document_digest": d, "git_root_uri": "file:///fixture",
		"git_commit": strings.Repeat("a", 40), "git_before": map[string]any{"role": "host_git_before", "schema_version": "https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.proposed.schema.json#/$defs/hostGit", "selector": "git-before.json", "digest": d, "byte_length": 1},
		"implementation_digest": d, "schema_digest": d, "policy_digests": map[string]string{"method": d, "admission": d, "privacy": d, "retention": d},
		"invocation_nonce": strings.Repeat("b", 32), "source_selector": "source.bin", "source_digest": d, "occurrence_id": d,
	})
	if err != nil {
		t.Fatal(err)
	}
	declarationRef := productionFixtureOriginal(t, root, "production_query_declaration", productionAttemptSchema+"productionQueryDeclaration", "selected-declaration.json", declaration)
	policy, err := json.Marshal(map[string]any{"role": "ADR0011_ATTEMPT_WITNESS_POLICY_V1", "version": 1, "declaration_ref": declarationRef, "selected_roles": []string{"attempt_initiation", "attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge", "attempt_request_key_observation"}, "host_selected_before_invocation": true})
	if err != nil {
		t.Fatal(err)
	}
	productionAssertRawSchema(t, policy, "attemptWitnessPolicy")
	productionAssertRawSchema(t, declaration, "productionQueryDeclaration")
	schemaOriginal, err := os.ReadFile("../../docs/qualification/schemas/adr0011-production-admission-v1.proposed.schema.json")
	if err != nil || privateDigest(schemaOriginal) != productionAttemptSchemaDigest {
		t.Fatalf("ASSERT_SCHEMA_PIN: %v", err)
	}
	return productionAttemptPrewrite{
		SchemaOriginal: schemaOriginal,
		PolicyRef:      productionFixtureOriginal(t, root, "attempt_witness_policy", productionAttemptSchema+"attemptWitnessPolicy", "selected-policy.json", policy),
		DeclarationRef: declarationRef,
		PolicyOriginal: policy, DeclarationOriginal: declaration, SessionID: sessionID, Generation: generation, Invocation: privateDigest([]byte("synthetic-invocation")),
	}
}
func productionHostFixture(t *testing.T, root *publication.Root, pre productionAttemptPrewrite) productionAttemptHostSelection {
	t.Helper()
	raw, err := productionAttemptExpected(root, pre, nil)
	if err != nil {
		t.Fatal(err)
	}
	return productionAttemptHostSelection{Selected: pre, ExpectedOriginal: append([]byte(nil), raw...), ExpectedRef: productionAttemptRef{"attempt_host_custody", productionAttemptSchema + "attemptHostCustody", "adr0011-attempt_host_custody-v2-" + strings.TrimPrefix(privateDigest(raw), "sha256:") + ".json", privateDigest(raw), len(raw)}}
}
func productionAssertSchema(t *testing.T, root *publication.Root, p productionAttemptPublication, def string) map[string]any {
	t.Helper()
	raw, err := publication.ReadVerifiedBoundFile(root, p.selector, productionAttemptLimit)
	if err != nil {
		t.Fatal(err)
	}
	return productionAssertRawSchema(t, raw, def)
}
func productionAssertRawSchema(t *testing.T, raw []byte, def string) map[string]any {
	t.Helper()
	contract, err := os.ReadFile("../../docs/qualification/schemas/adr0011-production-admission-v1.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = json.Unmarshal(contract, &document); err != nil {
		t.Fatal(err)
	}
	id := document["$id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err = compiler.AddResource(id, document); err != nil {
		t.Fatal(err)
	}
	shape, err := compiler.Compile(id + "#/$defs/" + def)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if err = shape.Validate(value); err != nil {
		t.Fatalf("ASSERT_PRODUCTION_SCHEMA_%s: %v raw=%s", def, err, raw)
	}
	return value.(map[string]any)
}
func TestProductionAttemptCustodyNoWrite(t *testing.T) {
	root := privatePublicationRoot(t)
	pre := productionFixturePre(t, root, "session", 1)
	host := productionHostFixture(t, root, pre)
	custody, err := publishProductionAttemptPrewrite(root, pre, host, nil)
	if err != nil || custody.stage != "VERIFIED" {
		t.Fatalf("ASSERT_PREWRITE_VERIFIED: %+v %v", custody, err)
	}
	productionAssertSchema(t, root, custody, "attemptHostCustody")
	observation, err := observeProductionAttemptWrite(root, pre, host, custody, sessionruntime.RoundTripResult{}, nil)
	if !errors.Is(err, errProductionAttemptWriteUnavailable) || observation.stage != "ABSENT" || observation.selector != "" {
		t.Fatalf("ASSERT_NO_WRITE_OBSERVATION_ABSENT: %+v %v", observation, err)
	}
	// N/B/T/E accounting belongs to a later ledger slice, not this helper.
}
func TestProductionAttemptCustodyConflictAndReadback(t *testing.T) {
	root := privatePublicationRoot(t)
	pre := productionFixturePre(t, root, "session", 1)
	host := productionHostFixture(t, root, pre)
	for _, fixture := range []struct {
		name   string
		mutate func(*productionAttemptPrewrite)
	}{
		{"policy-role-only", func(p *productionAttemptPrewrite) {
			p.PolicyOriginal = []byte(`{"role":"ADR0011_ATTEMPT_WITNESS_POLICY_V1"}`)
			p.PolicyRef = productionFixtureOriginal(t, root, "attempt_witness_policy", productionAttemptSchema+"attemptWitnessPolicy", "bad-policy.json", p.PolicyOriginal)
		}},
		{"declaration-role-only", func(p *productionAttemptPrewrite) {
			p.DeclarationOriginal = []byte(`{"role":"ADR0011_PRODUCTION_QUERY_DECLARATION_V1"}`)
			p.DeclarationRef = productionFixtureOriginal(t, root, "production_query_declaration", productionAttemptSchema+"productionQueryDeclaration", "bad-declaration.json", p.DeclarationOriginal)
		}},
		{"nested-duplicate", func(p *productionAttemptPrewrite) {
			p.DeclarationOriginal = bytes.Replace(p.DeclarationOriginal, []byte(`"method":`), []byte(`"method":"sha256:bad","method":`), 1)
			p.DeclarationRef = productionFixtureOriginal(t, root, "production_query_declaration", productionAttemptSchema+"productionQueryDeclaration", "duplicate-declaration.json", p.DeclarationOriginal)
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			bad := pre
			fixture.mutate(&bad)
			got, e := publishProductionAttemptPrewrite(root, bad, host, nil)
			if e == nil || got.stage != "ABSENT" {
				t.Fatalf("ASSERT_ORIGINAL_SCHEMA_REJECTED: %+v %v", got, e)
			}
		})
	}
	substituted := pre
	substituted.PolicyOriginal = append(append([]byte(nil), pre.PolicyOriginal...), '\n')
	substituted.PolicyRef = productionFixtureOriginal(t, root, "attempt_witness_policy", productionAttemptSchema+"attemptWitnessPolicy", "substituted-policy.json", substituted.PolicyOriginal)
	productionAssertRawSchema(t, substituted.PolicyOriginal, "attemptWitnessPolicy")
	if got, e := publishProductionAttemptPrewrite(root, substituted, host, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_HOST_HELD_COHERENT_SUBSTITUTION: %+v %v", got, e)
	}
	// Rehash both originals and rebind the substituted policy to the new declaration.
	both := pre
	both.DeclarationOriginal = bytes.Replace(pre.DeclarationOriginal, []byte(`"line":1`), []byte(`"line":2`), 1)
	if bytes.Equal(both.DeclarationOriginal, pre.DeclarationOriginal) {
		t.Fatal("ASSERT_SUBSTITUTION_SETUP")
	}
	both.DeclarationRef = productionFixtureOriginal(t, root, "production_query_declaration", productionAttemptSchema+"productionQueryDeclaration", "substituted-declaration.json", both.DeclarationOriginal)
	var policy map[string]any
	if e := json.Unmarshal(pre.PolicyOriginal, &policy); e != nil {
		t.Fatal(e)
	}
	policy["declaration_ref"] = both.DeclarationRef
	var marshalErr error
	both.PolicyOriginal, marshalErr = json.Marshal(policy)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	both.PolicyRef = productionFixtureOriginal(t, root, "attempt_witness_policy", productionAttemptSchema+"attemptWitnessPolicy", "substituted-bound-policy.json", both.PolicyOriginal)
	productionAssertRawSchema(t, both.DeclarationOriginal, "productionQueryDeclaration")
	productionAssertRawSchema(t, both.PolicyOriginal, "attemptWitnessPolicy")
	if got, e := publishProductionAttemptPrewrite(root, both, host, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_HOST_HELD_DOUBLE_SUBSTITUTION: %+v %v", got, e)
	}
	badHost := host
	badHost.ExpectedRef.Digest = privateDigest([]byte("other-host-label"))
	if got, e := publishProductionAttemptPrewrite(root, pre, badHost, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_EXPECTED_HOST_REF_REJECTED: %+v %v", got, e)
	}
	badHost = host
	badHost.ExpectedOriginal = append([]byte(nil), host.ExpectedOriginal...)
	badHost.ExpectedOriginal[0] ^= 1
	if got, e := publishProductionAttemptPrewrite(root, pre, badHost, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_EXPECTED_HOST_BYTES_REJECTED: %+v %v", got, e)
	}
	first, err := publishProductionAttemptPrewrite(root, pre, host, nil)
	if err != nil || first.stage != "VERIFIED" {
		t.Fatalf("ASSERT_FIRST_VERIFIED: %+v %v", first, err)
	}
	second, err := publishProductionAttemptPrewrite(root, pre, host, nil)
	if err == nil || second.stage != "ABSENT" {
		t.Fatalf("ASSERT_CONFLICT_ABSENT: %+v %v", second, err)
	}
	bad := pre
	bad.PolicyRef.Digest = privateDigest([]byte("forged"))
	if got, e := publishProductionAttemptPrewrite(root, bad, host, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_REF_DIGEST_REJECTED: %+v %v", got, e)
	}
	bad = pre
	bad.PolicyOriginal = []byte(`{"role":"different"}`)
	if got, e := publishProductionAttemptPrewrite(root, bad, host, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_ORIGINAL_BYTES_REJECTED: %+v %v", got, e)
	}
	bad = pre
	bad.PolicyRef.Role = "attempt_request_key_observation"
	if got, e := publishProductionAttemptPrewrite(root, bad, host, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_REF_ROLE_REJECTED: %+v %v", got, e)
	}
	failRoot := privatePublicationRoot(t)
	fpre := productionFixturePre(t, failRoot, "session", 1)
	fhost := productionHostFixture(t, failRoot, fpre)
	failed, e := publishProductionAttemptPrewrite(failRoot, fpre, fhost, func(r *publication.Root, s string, n int64) ([]byte, error) {
		if strings.Contains(s, "attempt_host_custody") {
			return nil, errors.New("injected")
		}
		return publication.ReadVerifiedBoundFile(r, s, n)
	})
	if e == nil || failed.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_AFTER_COMMIT_READBACK: %+v %v", failed, e)
	}
	if got, e := publishProductionAttemptPrewrite(failRoot, fpre, fhost, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_NO_RETRY_VERIFIED: %+v %v", got, e)
	}
}
func TestProductionAttemptCustodyManagedWrite(t *testing.T) {
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 16}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown(context.Background())
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "gopls", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), LanguageID: "go", Process: managedprocess.Spec{Path: executable, Args: []string{"-test.run=^TestADR0011AcquisitionPeer$"}, Env: append(os.Environ(), "ADR0011_ACQUISITION_PEER=1")}})
	if started.Failure != "" {
		t.Fatalf("start: %+v", started)
	}
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(10*time.Second))
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("readiness: %+v %v", ready, found)
	}
	root := privatePublicationRoot(t)
	pre := productionFixturePre(t, root, started.SessionID, started.Generation)
	host := productionHostFixture(t, root, pre)
	custody, err := publishProductionAttemptPrewrite(root, pre, host, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := manager.RoundTrip(context.Background(), sessionruntime.RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/references", Params: json.RawMessage(`{"textDocument":{"uri":"file:///fixture/query.go"},"position":{"line":1,"character":5},"context":{"includeDeclaration":true}}`), Deadline: time.Now().Add(5 * time.Second), MaxMessages: 8, MaxBytes: 65536, CaptureMethodRequestFrameMaxBytes: 4096})
	if result.Failure != "" || result.ServerError != nil {
		t.Fatalf("roundtrip: failure=%s server=%v", result.Failure, result.ServerError)
	}
	wrong := result
	wrong.Key.ID++
	if got, e := observeProductionAttemptWrite(root, pre, host, custody, wrong, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_MANAGER_KEY_MISMATCH: %+v %v", got, e)
	}
	wrongPre := pre
	wrongPre.Invocation = privateDigest([]byte("other"))
	if got, e := observeProductionAttemptWrite(root, wrongPre, host, custody, result, nil); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_PREWRITE_MISMATCH: %+v %v", got, e)
	}
	if got, e := observeProductionAttemptWrite(root, pre, host, custody, result, func(r *publication.Root, s string, n int64) ([]byte, error) {
		if s == custody.selector {
			return []byte("wrong"), nil
		}
		return publication.ReadVerifiedBoundFile(r, s, n)
	}); e == nil || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_CUSTODY_BYTES_MISMATCH: %+v %v", got, e)
	}
	observation, err := observeProductionAttemptWrite(root, pre, host, custody, result, nil)
	if err != nil || observation.stage != "VERIFIED" {
		t.Fatalf("ASSERT_MANAGED_POSTWRITE_VERIFIED: %+v %v", observation, err)
	}
	fields := productionAssertSchema(t, root, observation, "attemptRequestKeyObservation")
	if fields["request_key"] != adr0011requestkey.Encode(result.Key) {
		t.Fatalf("ASSERT_CANONICAL_MANAGER_KEY: %v", fields["request_key"])
	}
	selector := fields["write_selector"].(string)
	if !strings.HasSuffix(selector, ".bin") {
		t.Fatalf("ASSERT_BINARY_FRAME_SELECTOR: %s", selector)
	}
	frame, ok := result.CompletedMethodRequestFrame()
	if !ok {
		t.Fatal("missing manager frame")
	}
	retained, e := publication.ReadVerifiedBoundFile(root, selector, productionAttemptLimit)
	if e != nil || !bytes.Equal(retained, frame) {
		t.Fatalf("ASSERT_MANAGER_FRAME_EXACT: %v", e)
	}
	// Completed WRITE without the separately opted-in retained frame is not sufficient.
	withoutFrame := manager.RoundTrip(context.Background(), sessionruntime.RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/references", Params: json.RawMessage(`{"textDocument":{"uri":"file:///fixture/query.go"},"position":{"line":1,"character":5},"context":{"includeDeclaration":true}}`), Deadline: time.Now().Add(5 * time.Second), MaxMessages: 8, MaxBytes: 65536})
	if _, wrote := withoutFrame.CompletedRequestWrite(); !wrote {
		t.Fatal("ASSERT_COMPLETED_WRITE_FIXTURE")
	}
	if _, retained := withoutFrame.CompletedMethodRequestFrame(); retained {
		t.Fatal("ASSERT_FRAME_NOT_RETAINED_FIXTURE")
	}
	if got, e := observeProductionAttemptWrite(root, pre, host, custody, withoutFrame, nil); !errors.Is(e, errProductionAttemptWriteUnavailable) || got.stage != "ABSENT" {
		t.Fatalf("ASSERT_WRITE_WITHOUT_EXACT_FRAME_ABSENT: %+v %v", got, e)
	}
}
