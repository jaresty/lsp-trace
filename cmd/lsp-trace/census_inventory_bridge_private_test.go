package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	ai "lsp-trace/internal/applicationinventory"
	inv "lsp-trace/internal/inventorysuccessor"
	product "lsp-trace/internal/privatehostproductv3"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Fresh synthetic canonical test authority only. No retained job, production
// initialization, native process, or external grant is used by these fixtures.
func custodyFundedConfig46(t *testing.T, h censusInventoryHeader, ordinals ...int) censusInventoryBridgeConfig {
	t.Helper()
	accounting := t.TempDir()
	if e := os.Chmod(accounting, 0700); e != nil {
		t.Fatal(e)
	}
	c := censusInventoryBridgeConfig{Schema: "private-connected-inventory-bridge.v1", Action: "discover", Root: h.Workspace, AccountingRoot: accounting, Header: h, Profile: runtimeprofile.Selector{TrustDomain: "test", Workspace: h.Workspace, Profile: "go", EnvironmentReference: "local"}, LanguageID: "go", Deadline: time.Now().Add(10 * time.Second), Limits: censusPrivateDiscoveryLimits(), Verification: inv.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}, Attempt: censusInventoryDigest([]byte(accounting))}
	selected, e := runtimeprofile.Validate(c.Profile)
	if e != nil {
		t.Fatal(e)
	}
	construction := sessionruntime.PrivateDiscoveryConstruction{Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.LanguageID, Process: c.Process}, Deadline: c.Deadline, Limits: c.Limits}
	ordinal := 0
	if len(ordinals) > 0 {
		ordinal = ordinals[0]
	}
	for _, f := range h.Plan.Partitions[ordinal].Members {
		construction.Members = append(construction.Members, sessionruntime.PrivateDiscoveryMember{Path: f.Path, URI: f.URI, LanguageID: f.LanguageID})
	}
	member, runtime := inv.PrivateDiscoveryConstructionIdentityV2(construction)
	raw, _ := censusInventoryMarshal(h, 4<<20)
	binding := inv.PrivateDiscoveryBindingV2{Revision: h.SourceRevision, PlanIdentity: h.Plan.Identity, DenominatorDigest: censusInventoryDigest(raw), MemberDigest: member, Workspace: h.Workspace, RuntimeFingerprint: runtime, Ordinal: ordinal, Deadline: c.Deadline, Limits: c.Limits}
	d := censusInventoryDigest([]byte("synthetic-test-policy"))
	seal := func(schema, s string) ([]byte, string) {
		unsigned := []byte(s + "\n")
		id := censusInventoryDigest([]byte("inventory-successor/private/v2\x00" + schema + "\x00" + string(unsigned)))
		return bytes.Replace(unsigned, []byte(`"identity":""`), []byte(`"identity":"`+id+`"`), 1), id
	}
	job, jid := seal("lsp-trace.inventory-job-identity.private.v2", fmt.Sprintf(`{"schema":"lsp-trace.inventory-job-identity.private.v2","identity":"","stable_id":%q,"application":%q,"scope":%q,"source_authorization":%q,"revision_policy":%q,"evidence_policies":[%q],"provider_policies":[%q],"semantic_policies":[%q],"privacy":%q,"retention":%q,"resource_rules":%q,"origin":"NEW_EMPTY"}`, c.Attempt, d, inv.PrivateDiscoveryScopeDigestV2(), binding.DenominatorDigest, d, d, d, d, d, d, inv.PrivateDiscoveryResourceRulesV2(binding)))
	currencies := []string{"logical_bytes", "read_hash_bytes", "write_bytes", "records", "parser_bytes", "provider_requests", "provider_messages", "work", "active_ns", "calls", "request_bytes", "response_bytes", "wire_bytes", "output_bytes"}
	counters := func(n uint64) string {
		parts := make([]string, len(currencies))
		for i, k := range currencies {
			parts[i] = fmt.Sprintf("%q:%d", k, n)
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	auth, aid := seal("lsp-trace.inventory-job-authorization.private.v2", fmt.Sprintf(`{"schema":"lsp-trace.inventory-job-authorization.private.v2","identity":"","job":%q,"revision":1,"previous_authorization":"NONE","verified_resource_head":"NONE","charged_snapshot":%s,"currency_ceiling":%s,"max_windows":4,"expires_unix_nano":%d,"max_tranche":%s,"max_window_ns":15000000000,"caller_decision_reference":%q,"identity_assurance":"CALLER_ASSERTED_NOT_AUTHENTICATED"}`, jid, counters(0), counters(1000000000000000), time.Now().Add(time.Minute).UnixNano(), counters(1000000000000000), d))
	root, e := publication.OpenRoot(accounting)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	ready, e := inv.InitializePrivateReadyV2(context.Background(), root, job, auth, inv.PrivateReadyDecisionAssertionV2{Job: jid, Authorization: aid, Reference: d}, c.Verification)
	if e != nil {
		t.Fatal("synthetic READY fixture", e)
	}
	c.CanonicalJob = ready.CanonicalJob()
	c.ExpectedHead = ready.ConfirmedHead()
	return c
}
func custodyInvoke46(t *testing.T, c censusInventoryBridgeConfig, deps censusRunnerDependencies) int {
	t.Helper()
	raw, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	var out, err bytes.Buffer
	return runCensusWithDependencies([]string{"--private-inventory-bridge-config", path, "--machine"}, &out, &err, deps)
}
func TestCensusInventoryPrivatePartitionCustody46(t *testing.T) {
	_, _, h, _ := connectedCursorFixture(t, 16)
	a, b := custodyFundedConfig46(t, h), custodyFundedConfig46(t, h)
	var starts atomic.Int32
	deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) {
		starts.Add(1)
		return nil, errors.New("scripted pre-process failure")
	}}
	if custodyInvoke46(t, a, deps) == 0 || starts.Load() != 1 {
		t.Fatal("ASSERT_CUSTODY46_FIRST_FUNDED_CALLBACK", starts.Load())
	}
	if custodyInvoke46(t, b, deps) == 0 || starts.Load() != 1 {
		t.Fatal("ASSERT_CUSTODY46_NO_FRESH_JOB_REPLAY", starts.Load())
	}
	t.Log("ASSERT_CUSTODY46_NO_FRESH_JOB_REPLAY PASS")
	t.Run("concurrent-fresh-jobs", func(t *testing.T) {
		_, _, h, _ := connectedCursorFixture(t, 16)
		a, b := custodyFundedConfig46(t, h), custodyFundedConfig46(t, h)
		entered, release, done := make(chan struct{}), make(chan struct{}), make(chan int, 1)
		var starts atomic.Int32
		deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) {
			if starts.Add(1) == 1 {
				close(entered)
				<-release
			}
			return nil, errors.New("scripted concurrent refusal")
		}}
		go func() { done <- custodyInvoke46(t, a, deps) }()
		<-entered
		loser := custodyInvoke46(t, b, deps)
		close(release)
		winner := <-done
		if loser == 0 || winner == 0 || starts.Load() != 1 {
			t.Fatal("ASSERT_CUSTODY46_CONCURRENT_WINNER_ONLY", starts.Load())
		}
		t.Log("ASSERT_CUSTODY46_CONCURRENT_WINNER_ONLY PASS")
	})
}
func TestCensusInventoryPrivateBridgeUnfundedBeforeStarter(t *testing.T) {
	_, _, h, _ := connectedCursorFixture(t, 16)
	accounting := filepath.Join(t.TempDir(), "accounting")
	if e := os.Mkdir(accounting, 0700); e != nil {
		t.Fatal(e)
	}
	base := filepath.Dir(accounting)
	cfg := censusInventoryBridgeConfig{Schema: "private-connected-inventory-bridge.v1", Action: "discover", Root: base, AccountingRoot: accounting, CanonicalJob: []byte(`{}`), ExpectedHead: censusInventoryDigest([]byte("unfunded")), Attempt: censusInventoryDigest([]byte("attempt")), Verification: inv.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}, Header: h, Profile: runtimeprofile.Selector{TrustDomain: "test", Workspace: h.Workspace, Profile: "go", EnvironmentReference: "local"}, LanguageID: "go", Deadline: time.Now().Add(10 * time.Second), Limits: censusPrivateDiscoveryLimits()}
	raw, e := json.Marshal(cfg)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(base, "config.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	starts := 0
	deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) { starts++; return nil, nil }}
	var out, diagnostic bytes.Buffer
	code := runCensusWithDependencies([]string{"--private-inventory-bridge-config", path, "--machine"}, &out, &diagnostic, deps)
	if code == 0 || starts != 0 || !strings.Contains(diagnostic.String(), "CONFIG") || out.Len() != 0 {
		t.Fatal("ASSERT_CONNECTED_UNFUNDED_NO_START", code, starts, diagnostic.String())
	}
	for _, name := range []string{"census-source-objects-v1", "census-source-heads-v1"} {
		if _, e := os.Stat(filepath.Join(base, name)); !os.IsNotExist(e) {
			t.Fatal("ASSERT_CONNECTED_UNFUNDED_NO_METADATA_ROOT_CREATION", e)
		}
	}
	t.Log("ASSERT_CONNECTED_UNFUNDED_REAL_ENTRY_NO_START PASS")
}
func TestCensusInventoryPrivateBridgeCannotFreezeMetadataAsAuthority(t *testing.T) {
	store, c, h, r := connectedCursorFixture(t, 16)
	c = connectedIndex(t, store, c, h, r, true)
	cfg := censusInventoryBridgeConfig{Root: h.Workspace, Action: "setup", Header: h, Target: 0, Mode: 0}
	cfg.Request = json.RawMessage(`{"metadata_only":true}`)
	var out, diagnostic bytes.Buffer
	if code := runCensusInventoryBridgeLifecycle(cfg, &out, &diagnostic); code == 0 || out.Len() != 0 {
		t.Fatal("ASSERT_CONNECTED_METADATA_NOT_NATIVE_AUTHORITY")
	}
	after, e := store.load(context.Background())
	if e != nil || after.Selector != c.Selector || after.Targets[0].Modes[0].State != "PENDING" {
		t.Fatal("ASSERT_CONNECTED_REFUSAL_NO_CURSOR_EFFECT", e)
	}
	// A bridge marker is not a native owner; the actual controller hook refuses it.
	typed := product.InventoryRequestV3{}
	binding := censusInventoryBridgeBinding{Root: h.Workspace, Key: h.Workflow, Header: c.Header, Index: c.IndexRecord, Target: c.Targets[0].Record, Mode: 0}
	b, _ := json.Marshal(binding)
	typed.Inventory.Manifest.Provenance = censusInventoryBindingPrefix + string(b)
	if claim, e := claimCensusInventoryConnectedQ(typed, "run", censusInventoryDigest([]byte("run"))); e == nil || claim != nil {
		t.Fatal("ASSERT_CONNECTED_DIRECT_CONTROLLER_NO_FAKE_OWNER")
	}
	t.Log("ASSERT_CONNECTED_METADATA_NOT_NATIVE_AUTHORITY PASS")
}

// This reaches the actual setup/run/publication controller with a metadata-only
// owner declaration whose accounting root does not exist. The native operation
// must refuse; no native proof, provider, real HOST, or semantic success is made.
func TestCensusInventoryPrivateBridgeActualControllerRefusalPublication(t *testing.T) {
	store, c, h, r := connectedCursorFixture(t, 16)
	c = connectedIndex(t, store, c, h, r, true)
	q := ownerCommandDataV3(t)
	raw, e := store.objects.Get(context.Background(), c.Targets[0].Record)
	if e != nil {
		t.Fatal(e)
	}
	var target censusInventoryTargetRecord
	if censusInventoryDecode(raw, &target, 4<<20) != nil {
		t.Fatal("target")
	}
	q.Configuration.SourceRoot = h.Workspace
	owner := q.Inventory.NativeOwners[0]
	owner.Profile.Workspace = h.Workspace
	owner.URI = target.Target.URI
	owner.Lead.ID = target.Target.SymbolIdentity
	owner.Line = target.Target.SelectionRange.Start.Line
	owner.Character = target.Target.SelectionRange.Start.Character
	q.Inventory.NativeOwners[0] = owner
	q.Inventory.Manifest.Leads = []ai.Lead{owner.Lead}
	q.Inventory.Manifest.Scope = h.Scope
	binding := censusInventoryBridgeBinding{Root: h.Workspace, Key: h.Workflow, Header: c.Header, Index: c.IndexRecord, Target: c.Targets[0].Record, Mode: 5}
	encoded, _ := json.Marshal(binding)
	q.Inventory.Manifest.Provenance = censusInventoryBindingPrefix + string(encoded)
	q.Inventory.Manifest.UntouchedGaps = []string{"remaining-denominator:" + c.Header + ":" + c.IndexRecord}
	ownerCommandSealV3(t, &q)
	frozen, e := json.Marshal(q)
	if e != nil {
		t.Fatal(e)
	}
	cfg := censusInventoryBridgeConfig{Root: h.Workspace, Header: h, Action: "setup", Target: 0, Mode: 5, Attempt: censusInventoryDigest([]byte("connected-setup")), Request: frozen}
	badHeader := cfg
	badHeader.Header.UntouchedGaps = append([]string(nil), cfg.Header.UntouchedGaps...)
	badHeader.Header.UntouchedGaps[0] = "rewritten-denominator"
	var rejected, rejectedDiagnostic bytes.Buffer
	if code := runCensusInventoryBridgeLifecycle(badHeader, &rejected, &rejectedDiagnostic); code == 0 {
		t.Fatal("ASSERT_CONNECTED_FULL_HEADER_IMMUTABLE_BEFORE_SETUP")
	}
	unchanged, headerErr := store.load(context.Background())
	if headerErr != nil || unchanged.Selector != c.Selector {
		t.Fatal("ASSERT_CONNECTED_BAD_HEADER_NO_CURSOR_EFFECT", headerErr)
	}
	var out, diagnostic bytes.Buffer
	if code := runCensusInventoryBridgeLifecycle(cfg, &out, &diagnostic); code != 0 {
		t.Fatal("ASSERT_CONNECTED_ACTUAL_SETUP", code, diagnostic.String())
	}
	var ticket struct{ RunID string }
	if json.Unmarshal(out.Bytes(), &ticket) != nil || !censusInventoryHash(ticket.RunID) {
		t.Fatal("ticket")
	}
	state, e := store.load(context.Background())
	if e != nil || state.Targets[0].Modes[5].RunID != ticket.RunID || state.Targets[0].Modes[5].State != "CONFIG_FROZEN" {
		t.Fatal("ASSERT_CONNECTED_ACTUAL_TICKET_FROZEN", e)
	}
	path := filepath.Join(filepath.Dir(q.Configuration.Store), "connected-q.json")
	inventoryTestWrite(t, path, q)
	if code, _, _ := inventoryTestEntry([]string{"setup", path}, nil); code == 0 {
		t.Fatal("ASSERT_CONNECTED_DIRECT_SETUP_REPLAY_REFUSED")
	}
	for i, role := range []string{"interpreter", "reviewer"} {
		if code, _, e := inventoryTestEntry([]string{"register", path, ticket.RunID, role, "synthetic-" + role, q.Configuration.RolePromptSHA256[i]}, nil); code != 0 {
			t.Fatal("register", e)
		}
	}
	var legacy inventoryTestRequest
	if json.Unmarshal(frozen, &legacy) != nil {
		t.Fatal("scripted helper metadata")
	}
	terminal := inventoryTestRun(t, inventoryTestSession{path: path, q: legacy, runID: ticket.RunID}, nil)
	after, e := store.load(context.Background())
	if e != nil || after.Targets[0].Modes[5].State != "COMPLETED" || after.Targets[0].Modes[5].Publication != terminal.InventoryHead {
		t.Fatal("ASSERT_CONNECTED_ACTUAL_PUBLICATION_READBACK_COMPLETES_ONLY_SELECTED_MODE", e)
	}
	for mode, m := range after.Targets[0].Modes {
		if mode != 5 && m.State != "PENDING" {
			t.Fatal("ASSERT_CONNECTED_NOT_SEVEN_MODE_SUCCESS")
		}
	}
	if terminal.SuccessfulDescriptions != 0 || terminal.Completeness != "UNKNOWN" || terminal.Absence != "PROHIBITED" || terminal.Accounting == "" {
		t.Fatal("ASSERT_CONNECTED_REFUSAL_NOT_SEMANTIC_QUALIFICATION")
	}
	if _, e := os.Stat(owner.AccountingRoot); !os.IsNotExist(e) {
		t.Fatal("ASSERT_CONNECTED_NO_NATIVE_LEDGER_CREATED", e)
	}
	if code, _, _ := inventoryTestEntry([]string{"run", path, ticket.RunID}, nil); code == 0 {
		t.Fatal("ASSERT_CONNECTED_ACTUAL_RUN_NO_REPLAY")
	}
	again, e := store.load(context.Background())
	if e != nil || again.Selector != after.Selector {
		t.Fatal("ASSERT_CONNECTED_REPLAY_NO_CURSOR_EFFECT", e)
	}
	t.Log("ASSERT_CONNECTED_ACTUAL_CONTROLLER_REFUSAL_PUBLICATION PASS (no native qualification)")
}
func TestCensusInventoryPrivateBridgeModesAndDefaultOff(t *testing.T) {
	for mode := 0; mode < 7; mode++ {
		owner := product.InventoryNativeOwnerV3{}
		bits := [7]int{6, 1, 7, 4, 5, 2, 3}[mode]
		for bit, kind := range []pa.RelationKind{pa.TypedCalls, pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition} {
			if bits&(1<<bit) != 0 {
				owner.Kinds = append(owner.Kinds, kind)
			}
		}
		if censusInventoryNativeMode(owner) != mode || censusInventoryModes[mode] == "" {
			t.Fatal("ASSERT_CONNECTED_SELECTED_ACTUAL_MODE", mode)
		}
	}
	options, e := parseCensusCLIOptions([]string{"census", "--workspace", "/synthetic", "--server", "never-started", "--publication-root", "/synthetic/publication", "--machine"})
	if e != nil || options.PrivateInventoryBridgeConfigPath != "" {
		t.Fatal("ASSERT_CONNECTED_DEFAULT_OFF", e)
	}
	cfg := censusInventoryBridgeConfig{Action: "discover"}
	raw, _ := json.Marshal(cfg)
	if _, e := decodeCensusInventoryBridge(raw); e == nil {
		t.Fatal("ASSERT_CONNECTED_NO_IMPLICIT_ENABLE")
	}
	// Source/gap metadata is not a successful semantic disposition for any mode.
	report, _ := applicationInventoryReconcileV3(ai.Manifest{Scope: "synthetic-full", Provenance: "metadata only", Leads: []ai.Lead{{ID: "unattempted", Provenance: "not native"}}}, ai.Report{})
	if len(report.Dispositions) != 1 || report.Dispositions[0].Status != "UNTOUCHED" || report.SuccessfulDescriptions != 0 {
		t.Fatal("ASSERT_CONNECTED_UNATTEMPTED_NOT_OBSERVED_EMPTY")
	}
	t.Log("ASSERT_CONNECTED_DEFAULT_OFF_SEVEN_DISTINCT_MODES PASS")
}
