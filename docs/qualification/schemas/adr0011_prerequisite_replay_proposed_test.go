package schemas_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Proposal-only oracle: no production publication, filesystem mutation, or issuance.
// Inputs are independent owner expectations, original bytes, and external verification facts.
const admissionID = "https://jaresty.github.io/lsp-trace/schemas/adr0011-production-admission-v1.proposed.schema.json"
const lifecycleID = "https://jaresty.github.io/lsp-trace/schemas/adr0011-lifecycle-v1.proposed.schema.json"
const digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func proposalRole(t *testing.T, file, id, role string) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" || doc["$id"] != id {
		t.Fatal("schema revision mismatch")
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err = c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(id + "#/$defs/" + role)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func cloneProposal(x map[string]any) map[string]any {
	b, _ := json.Marshal(x)
	var y map[string]any
	_ = json.Unmarshal(b, &y)
	return y
}

// lifecycleProposalRef is fixture-only; admission identities use admissionRef.
func lifecycleProposalRef(role string) map[string]any {
	def, ok := map[string]string{"intent": "fenceIntent", "child": "tombstoneChild", "root": "tombstoneRoot", "attempt": "unlinkAttempt", "dependent": "ref"}[role]
	if !ok {
		panic("unknown lifecycle fixture role: " + role)
	}
	return map[string]any{"role": role, "schema_version": lifecycleID + "#/$defs/" + def, "selector": "ref-" + role + ".json", "digest": digestA, "byte_length": 1}
}

const historicalID = "https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.proposed.schema.json"

// Admission roles are inventory labels, not replacements for predecessor record roles.
var historicalDefinitions = map[string]string{
	"candidate": "candidate", "proposal": "proposal", "events": "events", "scanner": "scanner",
	"raw_result": "rawResult", "response_read": "responseRead", "references_method_record": "methodRecord",
	"target_record": "targetRecord", "document_symbol_target_result": "targetResult",
	"owner_read_symbol": "ownerRead", "owner_read_references": "ownerRead", "prepared_source": "preparedSource",
	"source_identity": "sourceIdentity", "revision_identity": "revisionIdentity",
	"host_git_before": "hostGit", "host_git_after": "hostGit",
	"policy_method": "policy", "policy_admission": "policy", "policy_privacy": "policy", "policy_retention": "policy",
}

func admissionRoleURI(role string) string {
	if role == "production_query_declaration" {
		return admissionID + "#/$defs/productionQueryDeclaration"
	}
	return historicalID + "#/$defs/" + historicalDefinitions[role]
}
func admissionRef(role string) map[string]any {
	return map[string]any{"role": role, "schema_version": admissionRoleURI(role), "selector": "ref-" + role + ".json", "digest": digestA, "byte_length": 1}
}
func admissionRoles() []string {
	return strings.Split("candidate proposal events scanner raw_result response_read references_method_record target_record document_symbol_target_result owner_read_symbol owner_read_references prepared_source source_identity revision_identity host_git_before host_git_after policy_method policy_admission policy_privacy policy_retention production_query_declaration", " ")
}
func admissionRefs() []any {
	var refs []any
	for _, r := range admissionRoles() {
		refs = append(refs, admissionRef(r))
	}
	sort.Slice(refs, func(i, j int) bool {
		a, _ := json.Marshal(refs[i])
		b, _ := json.Marshal(refs[j])
		return bytes.Compare(a, b) < 0
	})
	return refs
}
func declaration() map[string]any {
	return map[string]any{"role": "ADR0011_PRODUCTION_QUERY_DECLARATION_V1", "version": 1, "plan_id": digestA, "session_id": "s", "generation": 1, "workspace_uri": "file:///tmp", "query_uri": "file:///tmp/a.go", "line": 0, "character": 0, "encoding": "utf-16", "document_version": 1, "document_byte_length": 1, "document_digest": digestA, "git_root_uri": "file:///tmp", "git_commit": strings.Repeat("a", 40), "git_before": admissionRef("host_git_before"), "implementation_digest": digestA, "schema_digest": digestA, "policy_digests": map[string]any{"method": digestA, "admission": digestA, "privacy": digestA, "retention": digestA}, "invocation_nonce": strings.Repeat("a", 32), "source_selector": "source.json", "source_digest": digestA, "occurrence_id": digestA}
}
func ancillaryRef(role, definition string) map[string]any {
	return map[string]any{"role": role, "schema_version": admissionID + "#/$defs/" + definition, "selector": "ref-" + role + ".json", "digest": digestA, "byte_length": 1}
}
func attemptRef(role, definition string) map[string]any {
	return map[string]any{"role": role, "schema_version": admissionID + "#/$defs/" + definition, "selector": "ref-" + role + ".json", "digest": digestA, "byte_length": 1}
}
func attemptManifestRef() map[string]any {
	return attemptRef("attempt_evidence_manifest", "attemptEvidenceManifest")
}
func attemptCustodyRef() map[string]any {
	return attemptRef("attempt_host_custody", "attemptHostCustody")
}
func attemptPolicyRef() map[string]any {
	return attemptRef("attempt_witness_policy", "attemptWitnessPolicy")
}
func ancillaryRefs() []any {
	return []any{ancillaryRef("production_terminal_ledger", "productionTerminalLedger"), ancillaryRef("production_occurrence_ledger", "productionOccurrenceLedger"), ancillaryRef("production_accounting_record", "productionAccountingRecord")}
}
func inventory() map[string]any {
	return map[string]any{"role": "ADR0011_PRODUCTION_DEPENDENCY_INVENTORY_V1", "version": 1, "inventory_id": digestA, "declaration_id": digestA, "dependency_refs": admissionRefs(), "ancillary_refs": ancillaryRefs(), "attempt_manifest_ref": attemptManifestRef()}
}
func finalAdmission() map[string]any {
	return map[string]any{"role": "ADR0011_PRODUCTION_ADMISSION_FINAL_V1", "version": 1, "context": map[string]any{"target_digest": digestA, "declaration_id": digestA, "request_key": "k", "invocation_id": digestA, "result_digest": digestA, "accounting_digest": digestA, "p": 1, "a": 1, "inventory_id": digestA, "inventory_digest": digestA, "inventory_byte_length": 1, "inventory_selector": "inventory.json", "method_outcome": "COMPLETE", "member_disposition": "ITEMS"}, "dependency_refs": admissionRefs()}
}
func lifecycleBase(role string) map[string]any {
	return map[string]any{"role": role, "version": 1, "decision_id": digestA, "root_identity": map[string]any{"device": 1, "inode": 1, "root_uri": "file:///tmp"}, "epoch": 1, "previous_epoch_digest": digestA, "record_id": digestA}
}
func TestADR0011LifecycleFixtureRefUnknownRole(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("unknown lifecycle fixture role admitted")
		}
	}()
	lifecycleProposalRef("candidate")
}

func lifecycleFixtures() map[string]map[string]any {
	intent := lifecycleProposalRef("intent")
	dep := lifecycleProposalRef("dependent")
	child := lifecycleProposalRef("child")
	attempt := lifecycleProposalRef("attempt")
	m := map[string]map[string]any{}
	add := func(name, role string) map[string]any { x := lifecycleBase(role); m[name] = x; return x }
	x := add("fenceIntent", "ADR0011_FENCE_INTENT_V1")
	x["action"] = "PRIVACY_REVOKE"
	x["selected_dependencies"] = []any{dep}
	x["dependent_set"] = []any{dep}
	x["expected_policy_digest"] = digestA
	x = add("tombstoneChild", "ADR0011_TOMBSTONE_CHILD_V1")
	x["intent_ref"] = intent
	x["dependent_ref"] = dep
	x = add("tombstoneRoot", "ADR0011_TOMBSTONE_ROOT_V1")
	x["intent_ref"] = intent
	x["dependent_count"] = 1
	x["children"] = []any{map[string]any{"dependent_ref": dep, "child_ref": child}}
	x = add("unlinkAttempt", "ADR0011_UNLINK_ATTEMPT_V1")
	x["intent_ref"] = intent
	x["target_ref"] = dep
	x["ordinal"] = 0
	x["pre_state"] = "MATCHED"
	x["syscall_outcome"] = "UNLINKED"
	x["post_state"] = "ABSENT"
	x["directory_sync"] = "SUCCEEDED"
	x["close_outcome"] = "SUCCEEDED"
	x = add("cleanupTerminal", "ADR0011_CLEANUP_TERMINAL_V1")
	x["intent_ref"] = intent
	x["outcome"] = "REMOVED_VERIFIED"
	x["attempts"] = []any{attempt}
	x["tombstone_root_ref"] = lifecycleProposalRef("root")
	x = add("reconciliation", "ADR0011_RECONCILIATION_V1")
	x["intent_ref"] = intent
	x["observations"] = []any{map[string]any{"expected_ref": dep, "status": "MATCHED"}}
	x["outcome"] = "KEEP_FENCE"
	return m
}
func TestADR0011PrerequisiteRoleShapes(t *testing.T) {
	families := []struct {
		file, id string
		fixtures map[string]map[string]any
	}{{"adr0011-production-admission-v1.proposed.schema.json", admissionID, map[string]map[string]any{"productionQueryDeclaration": declaration(), "productionDependencyInventory": inventory(), "productionAdmissionFinal": finalAdmission()}}, {"adr0011-lifecycle-v1.proposed.schema.json", lifecycleID, lifecycleFixtures()}}
	for _, family := range families {
		for role, valid := range family.fixtures {
			t.Run(role, func(t *testing.T) {
				s := proposalRole(t, family.file, family.id, role)
				if err := s.Validate(valid); err != nil {
					t.Fatal(err)
				}
				for _, tc := range []struct {
					name   string
					change func(map[string]any)
				}{{"missing role", func(x map[string]any) { delete(x, "role") }}, {"wrong role", func(x map[string]any) { x["role"] = "UNKNOWN" }}, {"extra", func(x map[string]any) { x["unexpected"] = true }}, {"missing required", func(x map[string]any) {
					delete(x, "record_id")
					delete(x, "dependency_refs")
					delete(x, "occurrence_id")
					delete(x, "inventory_id")
				}}} {
					t.Run(tc.name, func(t *testing.T) {
						x := cloneProposal(valid)
						tc.change(x)
						if err := s.Validate(x); err == nil {
							t.Fatal("accepted invalid shape")
						}
					})
				}
			})
		}
	}
	t.Run("uri format assertion", func(t *testing.T) {
		s := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionQueryDeclaration")
		x := declaration()
		x["query_uri"] = "relative"
		if s.Validate(x) == nil {
			t.Fatal("relative URI accepted")
		}
	})
	t.Run("role multiplicity", func(t *testing.T) {
		s := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionAdmissionFinal")
		for _, change := range []func([]any) []any{func(r []any) []any { return r[:20] }, func(r []any) []any { return append(r, admissionRef("candidate")) }, func(r []any) []any { r[0] = r[1]; return r }} {
			x := finalAdmission()
			x["dependency_refs"] = change(x["dependency_refs"].([]any))
			if s.Validate(x) == nil {
				t.Fatal("accepted ref cardinality/multiplicity")
			}
		}
	})
	t.Run("ancillary inventory roles", func(t *testing.T) {
		s := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionDependencyInventory")
		for _, tc := range []struct {
			name   string
			change func(map[string]any)
		}{
			{"omitted", func(x map[string]any) { x["ancillary_refs"] = ancillaryRefs()[:2] }},
			{"duplicated", func(x map[string]any) { r := ancillaryRefs(); r[2] = r[0]; x["ancillary_refs"] = r }},
			{"substituted URI", func(x map[string]any) {
				x["ancillary_refs"].([]any)[0].(map[string]any)["schema_version"] = admissionID + "#/$defs/productionAccountingRecord"
			}},
			{"unknown URI", func(x map[string]any) {
				x["ancillary_refs"].([]any)[0].(map[string]any)["schema_version"] = admissionID + "#/$defs/unknown"
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				x := inventory()
				tc.change(x)
				if s.Validate(x) == nil {
					t.Fatal("invalid ancillary selection accepted")
				}
			})
		}
	})
	t.Run("ledger back edge and occurrence array", func(t *testing.T) {
		policy := declaration()["policy_digests"]
		base := func(role string) map[string]any {
			return map[string]any{"role": role, "version": 1, "declaration_id": digestA, "request_key": "k", "invocation_id": digestA, "target": map[string]any{"status": "PRESENT", "digest": digestA}, "result": map[string]any{"status": "PRESENT", "digest": digestA}, "policy_digests": policy, "historical_predecessor_refs": []any{admissionRef("candidate")}, "attempt_manifest_ref": attemptManifestRef()}
		}
		terminal := base("ADR0011_PRODUCTION_TERMINAL_LEDGER_V1")
		terminal["b"], terminal["t"], terminal["outcome"], terminal["disposition"] = 1, 1, "COMPLETE", "ITEMS"
		occ := base("ADR0011_PRODUCTION_OCCURRENCE_LEDGER_V1")
		occ["target_digest"], occ["result_digest"] = digestA, digestA
		delete(occ, "target")
		delete(occ, "result")
		occ["p"], occ["parsed_occurrences"], occ["admitted_occurrences"] = 0, []any{}, []any{}
		for role, x := range map[string]map[string]any{"productionTerminalLedger": terminal, "productionOccurrenceLedger": occ} {
			s := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, role)
			if err := s.Validate(x); err != nil {
				t.Fatalf("%s: %v", role, err)
			}
			y := cloneProposal(x)
			y["historical_predecessor_refs"] = []any{ancillaryRef("production_accounting_record", "productionAccountingRecord")}
			if s.Validate(y) == nil {
				t.Fatalf("%s accepted accounting back edge", role)
			}
			y = cloneProposal(x)
			y["inventory_ref"] = ancillaryRef("production_accounting_record", "productionAccountingRecord")
			if s.Validate(y) == nil {
				t.Fatalf("%s accepted inventory/final edge", role)
			}
		}
		accounting := base("ADR0011_PRODUCTION_ACCOUNTING_RECORD_V1")
		accounting["terminal_ledger_ref"] = ancillaryRef("production_terminal_ledger", "productionTerminalLedger")
		accounting["occurrence_ledger_ref"] = ancillaryRef("production_occurrence_ledger", "productionOccurrenceLedger")
		for _, count := range []string{"n", "b", "t"} {
			accounting[count] = 1
		}
		for _, count := range []string{"e", "e_b", "e_t", "p", "a"} {
			accounting[count] = 0
		}
		accounting["disposition"], accounting["outcome"] = "EMPTY", "COMPLETE_EMPTY"
		accountingSchema := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionAccountingRecord")
		if err := accountingSchema.Validate(accounting); err != nil {
			t.Fatal(err)
		}
		wrong := cloneProposal(accounting)
		wrong["terminal_ledger_ref"] = ancillaryRef("production_occurrence_ledger", "productionOccurrenceLedger")
		if accountingSchema.Validate(wrong) == nil {
			t.Fatal("swapped accounting ledger role accepted")
		}
		incomplete := cloneProposal(accounting)
		incomplete["b"], incomplete["t"], incomplete["e"], incomplete["outcome"], incomplete["disposition"] = 0, 0, "UNKNOWN", nil, "INCOMPLETE"
		incomplete["target"], incomplete["result"] = map[string]any{"status": "ABSENT_NOT_OBSERVED"}, map[string]any{"status": "ABSENT_NOT_OBSERVED"}
		delete(incomplete, "occurrence_ledger_ref")
		if err := accountingSchema.Validate(incomplete); err != nil {
			t.Fatalf("incomplete accounting shape: %v", err)
		}
		incomplete["disposition"] = "TARGET_IDENTITY_UNRESOLVED"
		if err := accountingSchema.Validate(incomplete); err != nil {
			t.Fatalf("unresolved target accounting requires fake digest: %v", err)
		}
		incomplete["target"] = map[string]any{"status": "ABSENT_NOT_OBSERVED", "digest": digestA}
		if accountingSchema.Validate(incomplete) == nil {
			t.Fatal("placeholder target digest accepted")
		}
		incomplete["target"] = map[string]any{"status": "ABSENT_NOT_OBSERVED"}
		incomplete["disposition"] = "INCOMPLETE"
		incomplete["outcome"] = "COMPLETE_EMPTY"
		if accountingSchema.Validate(incomplete) == nil {
			t.Fatal("incomplete accounting fabricated method outcome")
		}
		malformed := cloneProposal(accounting)
		malformed["e"], malformed["outcome"], malformed["disposition"] = "UNKNOWN", "MALFORMED", "MALFORMED"
		if err := accountingSchema.Validate(malformed); err != nil {
			t.Fatalf("malformed unknown-E shape: %v", err)
		}
		malformed["p"] = 1
		if accountingSchema.Validate(malformed) == nil {
			t.Fatal("malformed whole-result parsed occurrence accepted")
		}
		completeUnknown := cloneProposal(accounting)
		completeUnknown["outcome"] = "COMPLETE"
		completeUnknown["disposition"] = "ITEMS"
		completeUnknown["p"] = 1
		completeUnknown["e"] = "UNKNOWN"
		if accountingSchema.Validate(completeUnknown) == nil {
			t.Fatal("COMPLETE/ITEMS with UNKNOWN E accepted")
		}
		completeNoAdmission := cloneProposal(accounting)
		completeNoAdmission["outcome"] = "COMPLETE"
		completeNoAdmission["disposition"] = "ITEMS"
		completeNoAdmission["e"] = 1
		completeNoAdmission["p"] = 1
		completeNoAdmission["a"] = 0
		if err := accountingSchema.Validate(completeNoAdmission); err != nil {
			t.Fatalf("retained COMPLETE with A=0 rejected: %v", err)
		}
		finalSchema := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionAdmissionFinal")
		noAdmissionFinal := finalAdmission()
		noAdmissionFinal["context"].(map[string]any)["a"] = 0
		if finalSchema.Validate(noAdmissionFinal) == nil {
			t.Fatal("COMPLETE/P>0/A=0 final admitted")
		}
		emptyFinal := finalAdmission()
		ctx := emptyFinal["context"].(map[string]any)
		ctx["method_outcome"] = "COMPLETE_EMPTY"
		ctx["member_disposition"] = "EMPTY"
		ctx["p"], ctx["a"] = 0, 0
		if err := finalSchema.Validate(emptyFinal); err != nil {
			t.Fatalf("verified zero-occurrence final shape rejected: %v", err)
		}
		s := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionOccurrenceLedger")
		delete(occ, "admitted_occurrences")
		if s.Validate(occ) == nil {
			t.Fatal("absent admitted occurrence array accepted")
		}
	})
	t.Run("attempt witness selection and nonissuance shapes", func(t *testing.T) {
		role := func(name string) *jsonschema.Schema {
			return proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, name)
		}
		policy := map[string]any{"role": "ADR0011_ATTEMPT_WITNESS_POLICY_V1", "version": 1, "declaration_ref": admissionRef("production_query_declaration"), "selected_roles": []any{"attempt_initiation", "attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge", "attempt_request_key_observation"}, "host_selected_before_invocation": true}
		if err := role("attemptWitnessPolicy").Validate(policy); err != nil {
			t.Fatal(err)
		}
		bad := cloneProposal(policy)
		bad["host_selected_before_invocation"] = false
		if role("attemptWitnessPolicy").Validate(bad) == nil {
			t.Fatal("post-invocation policy accepted")
		}
		manifest := map[string]any{"role": "ADR0011_ATTEMPT_EVIDENCE_MANIFEST_V1", "version": 1, "policy_ref": attemptPolicyRef(), "declaration_id": digestA, "invocation_id": digestA, "key_observation": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "initiation_ref": attemptRef("attempt_initiation", "attemptInitiation"), "custody_ref": attemptCustodyRef(), "begin": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "terminal": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "scanner": map[string]any{"status": "ABSENT_NOT_OBSERVED"}}
		baseWitness := func(name string) map[string]any {
			return map[string]any{"role": name, "version": 1, "declaration_id": digestA, "request_key": "k", "invocation_id": digestA, "policy_ref": attemptPolicyRef()}
		}
		init := baseWitness("ADR0011_ATTEMPT_INITIATION_V1")
		delete(init, "request_key")
		init["host_session_id"], init["host_generation"] = "s", 1
		init["declared_n"] = 1
		init["declaration_ref"] = admissionRef("production_query_declaration")
		begin := baseWitness("ADR0011_ATTEMPT_QUERY_BEGIN_V1")
		begin["query_ordinal"] = 0
		begin["key_observation_ref"] = attemptRef("attempt_request_key_observation", "attemptRequestKeyObservation")
		begin["response_key"] = "k"
		begin["evaluation_began"] = true
		journal := baseWitness("ADR0011_ATTEMPT_TERMINAL_JOURNAL_V1")
		journal["begin_ref"] = attemptRef("attempt_query_begin", "attemptQueryBegin")
		journal["query_ordinal"] = 0
		journal["member_disposition"] = "FAILED"
		journal["method_outcome"] = "PROVIDER_FAILURE"
		for name, original := range map[string]map[string]any{"attemptInitiation": init, "attemptQueryBegin": begin, "attemptTerminalJournal": journal} {
			s := role(name)
			if err := s.Validate(original); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			invalid := cloneProposal(original)
			invalid["role"] = "UNKNOWN"
			if s.Validate(invalid) == nil {
				t.Fatalf("wrong %s role accepted", name)
			}
		}
		journal["method_outcome"] = "COMPLETE"
		if role("attemptTerminalJournal").Validate(journal) == nil {
			t.Fatal("COMPLETE with FAILED member accepted")
		}
		journal["method_outcome"] = "PUBLICATION_UNVERIFIED"
		if role("attemptTerminalJournal").Validate(journal) == nil {
			t.Fatal("publication state admitted as method outcome")
		}
		custody := map[string]any{"role": "ADR0011_ATTEMPT_HOST_CUSTODY_V1", "version": 1, "policy_ref": attemptPolicyRef(), "declaration_ref": admissionRef("production_query_declaration"), "invocation_id": digestA, "host_session_id": "s", "host_generation": 1, "selection_phase": "PRE_INVOCATION", "policy_readback_digest": digestA, "declaration_readback_digest": digestA}
		if err := role("attemptHostCustody").Validate(custody); err != nil {
			t.Fatal(err)
		}
		custody["request_key"] = "fabricated-prewrite"
		if role("attemptHostCustody").Validate(custody) == nil || role("attemptInitiation").Validate(func() map[string]any { x := cloneProposal(init); x["request_key"] = "fabricated-prewrite"; return x }()) == nil {
			t.Fatal("prewrite records accepted a postwrite key")
		}
		delete(custody, "request_key")
		custody["selection_phase"] = "POST_RESPONSE"
		if role("attemptHostCustody").Validate(custody) == nil {
			t.Fatal("post-response custody shape accepted")
		}
		keyObservation := map[string]any{"role": "ADR0011_ATTEMPT_REQUEST_KEY_OBSERVATION_V1", "version": 1, "policy_ref": attemptPolicyRef(), "declaration_id": digestA, "custody_ref": attemptCustodyRef(), "request_key": "k", "invocation_id": digestA, "host_session_id": "s", "host_generation": 1, "wire_id": 1, "write_selector": "write.json", "write_digest": digestA, "write_byte_length": 1, "observation_phase": "COMPLETED_FRAMED_WRITE"}
		if err := role("attemptRequestKeyObservation").Validate(keyObservation); err != nil {
			t.Fatal(err)
		}
		if err := role("attemptEvidenceManifest").Validate(manifest); err != nil {
			t.Fatal(err)
		}
		incompleteInventory := map[string]any{"role": "ADR0011_ATTEMPT_INCOMPLETE_INVENTORY_V1", "version": 1, "custody_ref": attemptCustodyRef(), "attempt_manifest_ref": attemptManifestRef(), "declaration_id": digestA, "request_key": "k", "invocation_id": digestA, "state": "TARGET_IDENTITY_UNRESOLVED", "target": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "result": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "observed_predecessor_refs": []any{}}
		delete(incompleteInventory, "request_key")
		incompleteSchema := role("attemptIncompleteInventory")
		if err := incompleteSchema.Validate(incompleteInventory); err != nil {
			t.Fatalf("incomplete inventory requires fake 21 refs: %v", err)
		}
		fake := cloneProposal(incompleteInventory)
		fake["target"] = map[string]any{"status": "PRESENT", "digest": digestA}
		if incompleteSchema.Validate(fake) == nil {
			t.Fatal("unresolved target with invented target accepted")
		}
		fake = cloneProposal(incompleteInventory)
		fake["dependency_refs"] = admissionRefs()
		if incompleteSchema.Validate(fake) == nil {
			t.Fatal("incomplete inventory accepted 21 final refs")
		}
		wrongCustody := cloneProposal(manifest)
		wrongCustody["custody_ref"] = attemptPolicyRef()
		if role("attemptEvidenceManifest").Validate(wrongCustody) == nil {
			t.Fatal("manifest substituted custody role accepted")
		}
		for _, field := range []string{"begin", "terminal", "scanner"} {
			x := cloneProposal(manifest)
			x[field] = map[string]any{"status": "PRESENT"}
			if role("attemptEvidenceManifest").Validate(x) == nil {
				t.Fatalf("%s missing original accepted", field)
			}
		}
		x := cloneProposal(manifest)
		x["terminal"] = map[string]any{"status": "PRESENT", "ref": attemptRef("attempt_query_begin", "attemptQueryBegin")}
		if role("attemptEvidenceManifest").Validate(x) == nil {
			t.Fatal("terminal role substitution accepted")
		}
		x = cloneProposal(manifest)
		x["begin"] = map[string]any{"status": "ABSENT_NOT_OBSERVED"}
		x["terminal"] = map[string]any{"status": "PRESENT", "ref": attemptRef("attempt_terminal_journal", "attemptTerminalJournal")}
		if role("attemptEvidenceManifest").Validate(x) == nil {
			t.Fatal("terminal without keyed begin accepted")
		}
		x = cloneProposal(manifest)
		x["final_ref"] = attemptManifestRef()
		if role("attemptEvidenceManifest").Validate(x) == nil {
			t.Fatal("manifest final back-edge accepted")
		}
		scanner := map[string]any{"role": "ADR0011_ATTEMPT_SCANNER_KNOWLEDGE_V1", "version": 1, "declaration_id": digestA, "request_key": "k", "invocation_id": digestA, "policy_ref": attemptPolicyRef(), "begin_ref": attemptRef("attempt_query_begin", "attemptQueryBegin"), "e": "UNKNOWN", "e_b": 0, "e_t": 0, "top_level_state": "UNKNOWN"}
		if err := role("attemptScannerKnowledge").Validate(scanner); err != nil {
			t.Fatal(err)
		}
		scanner["top_level_state"] = "COMPLETE_BOUNDED"
		if role("attemptScannerKnowledge").Validate(scanner) == nil {
			t.Fatal("complete top-level with UNKNOWN count accepted")
		}
		scanner["top_level_state"] = "UNKNOWN"
		scanner["e"] = "ZERO"
		if role("attemptScannerKnowledge").Validate(scanner) == nil {
			t.Fatal("unknown scanner count substituted")
		}
		for _, value := range []any{"PUBLICATION_UNVERIFIED", "TARGET_IDENTITY_UNRESOLVED", "INCOMPLETE", "PROVIDER_FAILURE"} {
			x := finalAdmission()
			x["context"].(map[string]any)["method_outcome"] = value
			if role("productionAdmissionFinal").Validate(x) == nil {
				t.Fatalf("nonissuable final accepted %v", value)
			}
		}
		x = finalAdmission()
		x["context"].(map[string]any)["verification"] = "VERIFIED"
		if role("productionAdmissionFinal").Validate(x) == nil {
			t.Fatal("final self-attested verification accepted")
		}
		terminal := map[string]any{"role": "ADR0011_PRODUCTION_TERMINAL_LEDGER_V1", "version": 1, "declaration_id": digestA, "request_key": "k", "invocation_id": digestA, "target": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "result": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "policy_digests": declaration()["policy_digests"], "b": 0, "t": 0, "outcome": nil, "disposition": "INCOMPLETE", "historical_predecessor_refs": []any{admissionRef("candidate")}, "attempt_manifest_ref": attemptManifestRef()}
		if err := role("productionTerminalLedger").Validate(terminal); err != nil {
			t.Fatal(err)
		}
		terminal["outcome"] = "COMPLETE"
		if role("productionTerminalLedger").Validate(terminal) == nil {
			t.Fatal("no-terminal method outcome accepted")
		}
		terminal["b"], terminal["t"], terminal["target"], terminal["result"] = 1, 1, map[string]any{"status": "PRESENT", "digest": digestA}, map[string]any{"status": "PRESENT", "digest": digestA}
		terminal["disposition"] = "ITEMS"
		if err := role("productionTerminalLedger").Validate(terminal); err != nil {
			t.Fatalf("complete items terminal: %v", err)
		}
		terminal["outcome"] = nil
		if role("productionTerminalLedger").Validate(terminal) == nil {
			t.Fatal("terminal T=1 with null method outcome accepted")
		}
		terminal["outcome"] = "COMPLETE"
		terminal["disposition"] = "EMPTY"
		if role("productionTerminalLedger").Validate(terminal) == nil {
			t.Fatal("COMPLETE with EMPTY accepted")
		}
		terminal["outcome"] = "COMPLETE_EMPTY"
		terminal["disposition"] = "EMPTY"
		if err := role("productionTerminalLedger").Validate(terminal); err != nil {
			t.Fatalf("complete empty terminal: %v", err)
		}
		terminal["t"] = 0
		if role("productionTerminalLedger").Validate(terminal) == nil {
			t.Fatal("COMPLETE_EMPTY without terminal accepted")
		}
	})
	t.Run("empty terminal attempts", func(t *testing.T) {
		s := proposalRole(t, "adr0011-lifecycle-v1.proposed.schema.json", lifecycleID, "cleanupTerminal")
		x := lifecycleFixtures()["cleanupTerminal"]
		x["attempts"] = []any{}
		if s.Validate(x) == nil {
			t.Fatal("accepted empty attempts")
		}
	})
}

// Proposal-only host-selection discriminator. The expected reference comes from
// owner-held pre-invocation state, not from either claimant or its manifest.
func proposalSelectedOriginal(expected map[string]any, observed []byte, preInvocationSelected bool) error {
	if !preInvocationSelected {
		return fmt.Errorf("host selection did not precede invocation")
	}
	if _, err := canonicalProposal(observed); err != nil {
		return err
	}
	h := sha256.Sum256(observed)
	if expected["digest"] != "sha256:"+hex.EncodeToString(h[:]) || fmt.Sprint(expected["byte_length"]) != fmt.Sprint(len(observed)) {
		return fmt.Errorf("owner-selected original differs")
	}
	return nil
}

// The owner retains the completed framed WRITE separately from the claimant graph.
// A valid claimant rehash cannot replace the owner-selected wire/key pair.
func replayObservedAttemptKey(observation map[string]any, write []byte, expectedKey string, expectedWireID int, custodyRef map[string]any) error {
	if observation["request_key"] != expectedKey || fmt.Sprint(observation["wire_id"]) != fmt.Sprint(float64(expectedWireID)) && fmt.Sprint(observation["wire_id"]) != fmt.Sprint(expectedWireID) || !reflect.DeepEqual(observation["custody_ref"], custodyRef) || observation["write_digest"] != canonicalDigest(write) || fmt.Sprint(observation["write_byte_length"]) != fmt.Sprint(float64(len(write))) && fmt.Sprint(observation["write_byte_length"]) != fmt.Sprint(len(write)) {
		return fmt.Errorf("observed WRITE/frame differs from independently retained owner expectation")
	}
	return nil
}

func TestADR0011PrerequisiteHostSelectionSubstitution(t *testing.T) {
	policy := map[string]any{"role": "ADR0011_ATTEMPT_WITNESS_POLICY_V1", "version": 1, "declaration_ref": admissionRef("production_query_declaration"), "selected_roles": []any{"attempt_initiation", "attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge", "attempt_request_key_observation"}, "host_selected_before_invocation": true}
	original, _ := json.Marshal(policy)
	h := sha256.Sum256(original)
	expected := attemptPolicyRef()
	expected["digest"] = "sha256:" + hex.EncodeToString(h[:])
	expected["byte_length"] = len(original)
	if err := proposalSelectedOriginal(expected, original, true); err != nil {
		t.Fatal(err)
	}
	altered := cloneProposal(policy)
	altered["declaration_ref"].(map[string]any)["digest"] = digestB
	if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "attemptWitnessPolicy").Validate(altered); err != nil {
		t.Fatalf("counterexample must remain schema-valid: %v", err)
	}
	changed, _ := json.Marshal(altered)
	rehashed := cloneProposal(expected)
	changedHash := sha256.Sum256(changed)
	rehashed["digest"] = "sha256:" + hex.EncodeToString(changedHash[:])
	rehashed["byte_length"] = len(changed)
	if err := proposalSelectedOriginal(rehashed, changed, true); err != nil {
		t.Fatalf("claimant self-consistency control: %v", err)
	}
	if err := proposalSelectedOriginal(expected, changed, true); err == nil {
		t.Fatal("rehash substitution accepted against host-selected original")
	}
	if err := proposalSelectedOriginal(expected, original, false); err == nil {
		t.Fatal("post-invocation selection accepted")
	}
}

// Canonical original bytes are an independent codec condition, not JSON Schema validation.
func canonicalProposal(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := uniqueJSON(dec); err != nil {
		return nil, err
	}
	var extra any
	if dec.Decode(&extra) == nil {
		return nil, fmt.Errorf("trailing JSON")
	}
	var x map[string]any
	if err := json.Unmarshal(raw, &x); err != nil {
		return nil, err
	}
	b, err := json.Marshal(x)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(raw, b) {
		return nil, fmt.Errorf("noncanonical original bytes")
	}
	return x, nil
}
func uniqueJSON(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return fmt.Errorf("duplicate/nonstring key")
			}
			seen[s] = true
			if err := uniqueJSON(dec); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := uniqueJSON(dec); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected delimiter")
	}
	_, err = dec.Token()
	return err
}
func roleDigest(domain string, x map[string]any, idField string) string {
	c := cloneProposal(x)
	delete(c, idField)
	b, _ := json.Marshal(c)
	h := sha256.Sum256(append([]byte(domain+"\x00"), b...))
	return "sha256:" + hex.EncodeToString(h[:])
}
func TestADR0011PostWriteIdentityAndNoWrite(t *testing.T) {
	role := func(name string) *jsonschema.Schema {
		return proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, name)
	}
	policy := map[string]any{"role": "ADR0011_ATTEMPT_WITNESS_POLICY_V1", "version": 1, "declaration_ref": admissionRef("production_query_declaration"), "selected_roles": []any{"attempt_initiation", "attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge", "attempt_request_key_observation"}, "host_selected_before_invocation": true}
	custody := map[string]any{"role": "ADR0011_ATTEMPT_HOST_CUSTODY_V1", "version": 1, "policy_ref": attemptPolicyRef(), "declaration_ref": admissionRef("production_query_declaration"), "invocation_id": digestA, "host_session_id": "s", "host_generation": 1, "selection_phase": "PRE_INVOCATION", "policy_readback_digest": digestA, "declaration_readback_digest": digestA}
	init := map[string]any{"role": "ADR0011_ATTEMPT_INITIATION_V1", "version": 1, "declaration_id": digestA, "invocation_id": digestA, "policy_ref": attemptPolicyRef(), "declared_n": 1, "declaration_ref": admissionRef("production_query_declaration"), "host_session_id": "s", "host_generation": 1}
	manifest := map[string]any{"role": "ADR0011_ATTEMPT_EVIDENCE_MANIFEST_V1", "version": 1, "policy_ref": attemptPolicyRef(), "declaration_id": digestA, "invocation_id": digestA, "initiation_ref": attemptRef("attempt_initiation", "attemptInitiation"), "custody_ref": attemptCustodyRef(), "key_observation": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "begin": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "terminal": map[string]any{"status": "ABSENT_NOT_OBSERVED"}, "scanner": map[string]any{"status": "ABSENT_NOT_OBSERVED"}}
	for name, obj := range map[string]map[string]any{"attemptWitnessPolicy": policy, "attemptHostCustody": custody, "attemptInitiation": init, "attemptEvidenceManifest": manifest} {
		if err := role(name).Validate(obj); err != nil {
			t.Fatalf("honest no-WRITE %s: %v", name, err)
		}
	}
	absent := map[string]any{"status": "ABSENT_NOT_OBSERVED"}
	terminal := map[string]any{"role": "ADR0011_PRODUCTION_TERMINAL_LEDGER_V1", "version": 1, "declaration_id": digestA, "invocation_id": digestA, "target": absent, "result": absent, "policy_digests": declaration()["policy_digests"], "b": 0, "t": 0, "outcome": nil, "disposition": "INCOMPLETE", "historical_predecessor_refs": []any{}, "attempt_manifest_ref": attemptManifestRef()}
	accounting := map[string]any{"role": "ADR0011_PRODUCTION_ACCOUNTING_RECORD_V1", "version": 1, "declaration_id": digestA, "invocation_id": digestA, "target": absent, "result": absent, "policy_digests": declaration()["policy_digests"], "terminal_ledger_ref": ancillaryRef("production_terminal_ledger", "productionTerminalLedger"), "historical_predecessor_refs": []any{}, "attempt_manifest_ref": attemptManifestRef(), "n": 1, "b": 0, "t": 0, "e": "UNKNOWN", "e_b": 0, "e_t": 0, "p": 0, "a": 0, "disposition": "INCOMPLETE", "outcome": nil}
	incomplete := map[string]any{"role": "ADR0011_ATTEMPT_INCOMPLETE_INVENTORY_V1", "version": 1, "custody_ref": attemptCustodyRef(), "attempt_manifest_ref": attemptManifestRef(), "declaration_id": digestA, "invocation_id": digestA, "state": "INCOMPLETE", "target": absent, "result": absent, "observed_predecessor_refs": []any{}}
	for name, obj := range map[string]map[string]any{"productionTerminalLedger": terminal, "productionAccountingRecord": accounting, "attemptIncompleteInventory": incomplete} {
		if err := role(name).Validate(obj); err != nil {
			t.Fatalf("N=1 B=T=0 unknown no-WRITE %s: %v", name, err)
		}
	}
	for name, obj := range map[string]map[string]any{"productionTerminalLedger": terminal, "productionAccountingRecord": accounting} {
		bad := cloneProposal(obj)
		bad["b"], bad["t"], bad["outcome"], bad["disposition"] = 1, 1, "COMPLETE", "ITEMS"
		if role(name).Validate(bad) == nil {
			t.Fatalf("keyless completed %s accepted", name)
		}
	}
	for _, field := range []string{"request_key", "begin", "terminal", "scanner"} {
		bad := cloneProposal(manifest)
		if field == "request_key" {
			bad[field] = "fabricated"
		} else {
			bad[field] = map[string]any{"status": "PRESENT", "ref": attemptRef("attempt_query_begin", "attemptQueryBegin")}
		}
		if role("attemptEvidenceManifest").Validate(bad) == nil {
			t.Fatalf("no-WRITE manifest accepted %s", field)
		}
	}
	write, _, _ := oneLocationWire()
	observation := map[string]any{"role": "ADR0011_ATTEMPT_REQUEST_KEY_OBSERVATION_V1", "version": 1, "policy_ref": attemptPolicyRef(), "declaration_id": digestA, "custody_ref": attemptCustodyRef(), "request_key": oneLocationRequestKey, "invocation_id": digestA, "host_session_id": "s", "host_generation": 1, "wire_id": 1, "write_selector": "original-write.json", "write_digest": canonicalDigest(write), "write_byte_length": len(write), "observation_phase": "COMPLETED_FRAMED_WRITE"}
	manifest["key_observation"] = map[string]any{"status": "PRESENT", "ref": attemptRef("attempt_request_key_observation", "attemptRequestKeyObservation")}
	manifest["request_key"] = oneLocationRequestKey
	begin := map[string]any{"role": "ADR0011_ATTEMPT_QUERY_BEGIN_V1", "version": 1, "declaration_id": digestA, "request_key": oneLocationRequestKey, "invocation_id": digestA, "policy_ref": attemptPolicyRef(), "query_ordinal": 0, "response_key": oneLocationRequestKey, "key_observation_ref": attemptRef("attempt_request_key_observation", "attemptRequestKeyObservation"), "evaluation_began": true}
	for name, obj := range map[string]map[string]any{"attemptRequestKeyObservation": observation, "attemptEvidenceManifest": manifest, "attemptQueryBegin": begin} {
		if err := role(name).Validate(obj); err != nil {
			t.Fatalf("postwrite positive %s: %v", name, err)
		}
	}
	if err := replayObservedAttemptKey(observation, write, oneLocationRequestKey, 1, attemptCustodyRef()); err != nil {
		t.Fatal(err)
	}
	forged := cloneProposal(observation)
	forged["request_key"] = "claimant-rehashed-key"
	forged["wire_id"] = 2
	forgedBytes, _ := json.Marshal(forged)
	forgedRef := attemptRef("attempt_request_key_observation", "attemptRequestKeyObservation")
	forgedRef["digest"], forgedRef["byte_length"] = canonicalDigest(forgedBytes), len(forgedBytes)
	manifest["request_key"] = forged["request_key"]
	manifest["key_observation"].(map[string]any)["ref"] = forgedRef
	begin["request_key"], begin["response_key"], begin["key_observation_ref"] = forged["request_key"], forged["request_key"], forgedRef
	for name, obj := range map[string]map[string]any{"attemptRequestKeyObservation": forged, "attemptEvidenceManifest": manifest, "attemptQueryBegin": begin} {
		if err := role(name).Validate(obj); err != nil {
			t.Fatalf("forgery must clear schema before owner replay %s: %v", name, err)
		}
	}
	if err := replayObservedAttemptKey(forged, write, oneLocationRequestKey, 1, attemptCustodyRef()); err == nil {
		t.Fatal("fully rehashed claimant key accepted against owner-observed WRITE")
	}
}

func TestADR0011PrerequisiteOriginalBytes(t *testing.T) {
	b, _ := json.Marshal(declaration())
	if _, err := canonicalProposal(b); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append([]byte(" "), b...), append(append([]byte{}, b...), []byte("\n")...), []byte(`{"a":1,"a":2}`)} {
		if _, err := canonicalProposal(bad); err == nil {
			t.Fatalf("accepted noncanonical %q", bad)
		}
	}
}

// admissionExpectation is selected outside the claimant records. No method or final
// receipt is produced by this model; nil permits a future implementation to proceed.
type admissionExpectation struct {
	Plan                                                    map[string]any
	Refs                                                    []any
	Inventory                                               map[string]any
	PolicyBytes                                             map[string][]byte
	Stored                                                  map[string][]byte
	ExpectedClosure                                         map[string]map[string]any
	SelectedRoles                                           map[string]string
	InventorySelector, FinalSelector, FinalDigest           string
	ExpectedTargetDigest, ExpectedResultDigest              string
	ExpectedAccountingDigest, ExpectedRequestKey            string
	ExpectedInvocationID                                    string
	InvocationAfterDeclaration, TargetUnique, FinalReadback bool
}

func replayAdmission(d, i, f map[string]any, e admissionExpectation) error {
	fail := func() error {
		_, _, line, _ := runtime.Caller(1)
		return fmt.Errorf("admission replay rejected at %d; no new T/A", line)
	}
	if !e.InvocationAfterDeclaration || !e.TargetUnique || !e.FinalReadback {
		return fail()
	}
	if d["occurrence_id"] != roleDigest("ADR0011_PRODUCTION_QUERY_DECLARATION_V1", d, "occurrence_id") || i["inventory_id"] != roleDigest("ADR0011_PRODUCTION_DEPENDENCY_INVENTORY_V1", i, "inventory_id") {
		return fail()
	}
	for k, v := range e.Plan {
		a, _ := json.Marshal(d[k])
		b, _ := json.Marshal(v)
		if !bytes.Equal(a, b) {
			return fail()
		}
	}
	refs := f["dependency_refs"].([]any)
	if !reflect.DeepEqual(refs, e.Refs) || !reflect.DeepEqual(i["dependency_refs"], e.Refs) || len(refs) != 21 {
		return fail()
	}
	seen := map[string]bool{}
	var prev []byte
	for _, r := range refs {
		m := r.(map[string]any)
		role := m["role"].(string)
		if seen[role] {
			return fail()
		}
		seen[role] = true
		b, _ := json.Marshal(m)
		if prev != nil && bytes.Compare(prev, b) >= 0 {
			return fail()
		}
		prev = b
		if m["schema_version"] != e.SelectedRoles[role] {
			return fail()
		}
		body, ok := e.Stored[m["selector"].(string)]
		if !ok {
			return fail()
		}
		if m["digest"] != originalAdmissionDigest(role, body) || fmt.Sprint(m["byte_length"]) != fmt.Sprint(len(body)) {
			return fmt.Errorf("immediate %s: digest=%v actual=%v length=%v actual=%d", role, m["digest"], originalAdmissionDigest(role, body), m["byte_length"], len(body))
		}

	}
	for _, r := range admissionRoles() {
		if !seen[r] {
			return fail()
		}
	}
	// Traverse the independently expected complete closure from immediate roots.
	visited := map[string]bool{}
	active := map[string]bool{}
	var walk func(map[string]any) error
	walk = func(ref map[string]any) error {
		selector, ok := ref["selector"].(string)
		if !ok || selector == "" {
			return fail()
		}
		expected := e.ExpectedClosure[selector]
		if expected == nil {
			return fail()
		}
		if ref["role"] == nil {
			var child map[string]any
			if err := json.Unmarshal(e.Stored[selector], &child); err != nil {
				return fail()
			}
			version := child["schema_version"]
			if version == nil {
				version = child["Version"]
			}
			if ref["schema_version"] != version {
				return fail()
			}
			ref = map[string]any{"role": expected["role"], "schema_version": expected["schema_version"], "selector": selector, "digest": ref["digest"], "byte_length": expected["byte_length"]}
		}
		rawRef, _ := json.Marshal(ref)
		rawExpected, _ := json.Marshal(expected)
		if !bytes.Equal(rawRef, rawExpected) {
			return fail()
		}
		if active[selector] {
			return fail()
		}
		if visited[selector] {
			return nil
		}
		active[selector] = true
		defer delete(active, selector)
		role, ok := ref["role"].(string)
		if !ok || ref["schema_version"] != e.SelectedRoles[role] {
			return fail()
		}
		body, ok := e.Stored[selector]
		if !ok {
			return fail()
		}
		original, err := canonicalProposal(body)
		if err != nil || fmt.Sprint(ref["byte_length"]) != fmt.Sprint(len(body)) {
			return fail()
		}
		if err := validateAdmissionOriginal(role, original); err != nil {
			return fmt.Errorf("selected historical role %s: %w", role, err)
		}
		if strings.HasPrefix(role, "policy_") {
			payload := original["policy_bytes_ref"].(map[string]any)
			pinned := e.PolicyBytes[role]
			h := sha256.Sum256(pinned)
			if !bytes.Equal(e.Stored[payload["selector"].(string)], pinned) || payload["digest"] != "sha256:"+hex.EncodeToString(h[:]) || payload["selector"] != "adr0011-references-policy-bytes-v1-"+hex.EncodeToString(h[:])+".json" {
				return fmt.Errorf("policy payload %s", role)
			}
		}
		if ref["digest"] != originalAdmissionDigest(role, body) {
			return fail()
		}
		for _, child := range nestedAdmissionRefs(original) {
			if err := walk(child); err != nil {
				return fmt.Errorf("nested %s: %w", selector, err)
			}
		}
		visited[selector] = true
		return nil
	}
	for _, root := range refs {
		if err := walk(root.(map[string]any)); err != nil {
			return fmt.Errorf("closure: %w", err)
		}
	}
	if len(visited) != len(e.ExpectedClosure) {
		return fail()
	}
	ctx := f["context"].(map[string]any)
	ib, _ := json.Marshal(i)
	ih := sha256.Sum256(ib)
	fb, _ := json.Marshal(f)
	fh := sha256.Sum256(fb)
	if e.FinalDigest != roleDigest("ADR0011_PRODUCTION_ADMISSION_FINAL_V1", f, "") || e.FinalSelector != "adr0011-production-final-v1-"+hex.EncodeToString(fh[:])+".json" {
		return fail()
	}
	if ctx["inventory_id"] != i["inventory_id"] || ctx["inventory_digest"] != "sha256:"+hex.EncodeToString(ih[:]) || ctx["inventory_byte_length"] != len(ib) || ctx["inventory_selector"] != e.InventorySelector || ctx["declaration_id"] != d["occurrence_id"] || ctx["p"] != ctx["a"] {
		return fail()
	}
	for _, field := range []struct{ name, expected string }{
		{"target_digest", e.ExpectedTargetDigest},
		{"result_digest", e.ExpectedResultDigest},
		{"accounting_digest", e.ExpectedAccountingDigest},
		{"request_key", e.ExpectedRequestKey},
		{"invocation_id", e.ExpectedInvocationID},
	} {
		if field.expected == "" || ctx[field.name] != field.expected {
			return fmt.Errorf("final context %s differs from independent evidence; no new T/A", field.name)
		}
	}
	return nil
}
func originalAdmissionDigest(role string, body []byte) string {
	if role == "production_query_declaration" {
		h := sha256.Sum256(append([]byte("ADR0011_PRODUCTION_QUERY_DECLARATION_V1\x00"), body...))
		return "sha256:" + hex.EncodeToString(h[:])
	}
	var record map[string]any
	if err := json.Unmarshal(body, &record); err != nil {
		return ""
	}
	version, _ := record["schema_version"].(string)
	if version == "" {
		version, _ = record["Version"].(string)
	}
	if version == "" {
		return ""
	}
	h := sha256.Sum256(append([]byte(version+"\x00"), body...))
	return "sha256:" + hex.EncodeToString(h[:])
}
func validateAdmissionOriginal(role string, record map[string]any) error {
	id := historicalID
	file := "adr0011-references-issuance-records.proposed.schema.json"
	if role == "production_query_declaration" {
		id, file = admissionID, "adr0011-production-admission-v1.proposed.schema.json"
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err = json.Unmarshal(b, &doc); err != nil {
		return err
	}
	if doc["$id"] != id {
		return fmt.Errorf("pinned schema ID changed")
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err = c.AddResource(id, doc); err != nil {
		return err
	}
	s, err := c.Compile(admissionRoleURI(role))
	if err != nil {
		return err
	}
	if err = s.Validate(record); err != nil {
		return err
	}
	version := map[string]string{"candidate": "REFERENCES_OCCURRENCE_CANDIDATE_V1", "proposal": "REFERENCES_EVALUATION_PROPOSAL_V1", "events": "REFERENCES_EVALUATOR_EVENTS_V1", "scanner": "REFERENCES_SCANNER_OBSERVATION_V1", "raw_result": "REFERENCES_RAW_RESULT_V1", "response_read": "REFERENCES_RESPONSE_READ_V1", "references_method_record": "lsp-trace.adr0011.references-symbol.method.v1", "target_record": "lsp-trace.adr0011.references-symbol.target.v1", "document_symbol_target_result": "REFERENCES_TARGET_RESULT_V1", "owner_read_symbol": "REFERENCES_OWNER_READ_OBSERVATION_V1", "owner_read_references": "REFERENCES_OWNER_READ_OBSERVATION_V1", "prepared_source": "REFERENCES_PREPARED_SOURCE_V1", "source_identity": "REFERENCES_SOURCE_IDENTITY_V1", "revision_identity": "REFERENCES_REVISION_IDENTITY_V1", "host_git_before": "REFERENCES_HOST_GIT_OBSERVATION_V1", "host_git_after": "REFERENCES_HOST_GIT_OBSERVATION_V1", "policy_method": "REFERENCES_METHOD_POLICY_V1", "policy_admission": "REFERENCES_ADMISSION_POLICY_V1", "policy_privacy": "LOCAL_QUALIFICATION_PRIVACY_V1", "policy_retention": "REFERENCES_RETENTION_POLICY_V1"}[role]
	if role == "production_query_declaration" {
		if record["role"] == "ADR0011_PRODUCTION_QUERY_DECLARATION_V1" {
			return nil
		}
		return fmt.Errorf("wrong declaration role")
	}
	if version == "" {
		return fmt.Errorf("unselected role %s", role)
	}
	if record["schema_version"] == version || record["Version"] == version {
		return nil
	}
	return fmt.Errorf("wrong original version %s", role)
}

// Find historical record refs at every field depth, without treating raw payload
// selectors (which have no schema_version) as record refs.
func nestedAdmissionRefs(value any) []map[string]any {
	var refs []map[string]any
	var visit func(any)
	visit = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if _, ok := x["schema_version"]; ok {
				if _, ok := x["selector"]; ok {
					refs = append(refs, x)
					return
				}
			}
			for _, child := range x {
				visit(child)
			}
		case []any:
			for _, child := range x {
				visit(child)
			}
		}
	}
	visit(value)
	return refs
}

// Complete historical records: their inventory labels are not fields of the originals.
func admissionOriginal(role string, declarationRecord map[string]any) map[string]any {
	d := digestA
	ref := func(v string) map[string]any {
		selector := "adr0011-" + v + ".json"
		if v == "lsp-trace.adr0011.references-symbol.target.v1" {
			selector = "adr0011-target-" + strings.TrimPrefix(d, "sha256:") + ".json"
		}
		if v == "lsp-trace.adr0011.references-symbol.method.v1" {
			selector = "adr0011-method-" + strings.TrimPrefix(d, "sha256:") + ".json"
		}
		return map[string]any{"selector": selector, "digest": d, "schema_version": v}
	}
	point := map[string]any{"line": 0, "character": 0}
	rng := map[string]any{"start": point, "end": point}
	policyVersions := map[string]string{"policy_method": "REFERENCES_METHOD_POLICY_V1", "policy_admission": "REFERENCES_ADMISSION_POLICY_V1", "policy_privacy": "LOCAL_QUALIFICATION_PRIVACY_V1", "policy_retention": "REFERENCES_RETENTION_POLICY_V1"}
	if v, ok := policyVersions[role]; ok {
		return map[string]any{"schema_version": v, "policy_bytes_ref": map[string]any{"selector": "adr0011-references-policy-bytes-v1-" + strings.TrimPrefix(d, "sha256:") + ".json", "digest": d}}
	}
	switch role {
	case "production_query_declaration":
		return cloneProposal(declarationRecord)
	case "candidate":
		return map[string]any{"schema_version": "REFERENCES_OCCURRENCE_CANDIDATE_V1", "context": admissionContext(), "proposal_ref": ref("REFERENCES_EVALUATION_PROPOSAL_V1"), "occurrences": []any{}, "proposed_p": 0}
	case "proposal":
		return map[string]any{"schema_version": "REFERENCES_EVALUATION_PROPOSAL_V1", "context": admissionContext(), "raw_result_ref": ref("REFERENCES_RAW_RESULT_V1"), "raw_result_presence": "PRESENT", "raw_result_digest": d, "raw_result_byte_length": 2, "scanner_observation_ref": ref("REFERENCES_SCANNER_OBSERVATION_V1"), "top_level_form": "ARRAY", "declared_n": 1, "evaluator_event_ref": ref("REFERENCES_EVALUATOR_EVENTS_V1"), "observed_b": 1, "known_e": 0, "observed_e_b": 0, "observed_e_t": 0, "whole_result_p": 0, "proposed_outcome": "COMPLETE_EMPTY", "proposed_disposition": "EMPTY"}
	case "events":
		return map[string]any{"schema_version": "REFERENCES_EVALUATOR_EVENTS_V1", "transaction_id": d, "request_key": "k", "invocation_id": "i", "response_read_ref": ref("REFERENCES_RESPONSE_READ_V1"), "raw_result_ref": ref("REFERENCES_RAW_RESULT_V1"), "events": []any{map[string]any{"sequence": 0, "kind": "QUERY_BEGIN", "ordinal": nil, "terminal_disposition": "NONE"}, map[string]any{"sequence": 1, "kind": "QUERY_TERMINAL", "ordinal": nil, "terminal_disposition": "EMPTY"}}}
	case "scanner":
		return map[string]any{"schema_version": "REFERENCES_SCANNER_OBSERVATION_V1", "response_read_ref": ref("REFERENCES_RESPONSE_READ_V1"), "raw_result_ref": ref("REFERENCES_RAW_RESULT_V1"), "scanner_implementation_digest": d, "top_level_form": "ARRAY", "known_e": 0, "work_units": 1}
	case "raw_result":
		return map[string]any{"schema_version": "REFERENCES_RAW_RESULT_V1", "response_read_ref": ref("REFERENCES_RESPONSE_READ_V1"), "payload_selector": "adr0011-references-raw-payload-v1-" + strings.TrimPrefix(d, "sha256:") + ".bin", "payload_digest": d, "payload_byte_length": 2, "access": "OWNER_ONLY"}
	case "response_read":
		return map[string]any{"schema_version": "REFERENCES_RESPONSE_READ_V1", "transaction_id": d, "request_key": "k", "invocation_id": "i", "session_id": "s", "generation": 1, "method": "textDocument/references", "target_ref": ref("lsp-trace.adr0011.references-symbol.target.v1"), "write_digest": d, "read_digest": d, "owner_read_ref": ref("REFERENCES_OWNER_READ_OBSERVATION_V1"), "result_presence": "PRESENT", "raw_result_digest": d, "raw_result_byte_length": 2}
	case "references_method_record":
		return map[string]any{"Version": "lsp-trace.adr0011.references-symbol.method.v1", "Method": "textDocument/references", "SessionID": "s", "Generation": 1, "RequestKey": "k", "InvocationID": "i", "ParamsDigest": d, "TargetRef": ref("lsp-trace.adr0011.references-symbol.target.v1"), "SourceRef": ref("REFERENCES_SOURCE_IDENTITY_V1"), "RevisionRef": ref("REFERENCES_REVISION_IDENTITY_V1"), "ResponseReadRef": ref("REFERENCES_RESPONSE_READ_V1"), "RawResultRef": ref("REFERENCES_RAW_RESULT_V1")}
	case "target_record":
		return map[string]any{"Version": "lsp-trace.adr0011.references-symbol.target.v1", "Method": "textDocument/documentSymbol", "SessionID": "s", "Generation": 1, "RequestKey": "k", "ParamsDigest": d, "SourceRef": ref("REFERENCES_SOURCE_IDENTITY_V1"), "RevisionRef": ref("REFERENCES_REVISION_IDENTITY_V1"), "OwnerReadRef": ref("REFERENCES_OWNER_READ_OBSERVATION_V1"), "TargetResultRef": map[string]any{"selector": "adr0011-references-issuance-v1-target-result-" + strings.TrimPrefix(d, "sha256:") + ".json", "digest": d, "schema_version": "REFERENCES_TARGET_RESULT_V1"}, "QueryOccurrenceID": d, "QueryURI": "file:///tmp/a.go", "QueryLine": 0, "QueryCharacter": 0, "Encoding": "utf-16", "SymbolName": "A", "SymbolKind": 12, "DisplayRange": rng, "SelectionRange": rng, "TargetID": d, "SymbolID": d, "ResultDigest": d}
	case "document_symbol_target_result":
		return map[string]any{"schema_version": "REFERENCES_TARGET_RESULT_V1", "owner_read_ref": ref("REFERENCES_OWNER_READ_OBSERVATION_V1"), "payload_selector": "adr0011-references-target-result-v1-" + strings.TrimPrefix(d, "sha256:") + ".bin", "payload_digest": d, "payload_byte_length": 2, "access": "OWNER_ONLY"}
	case "owner_read_symbol", "owner_read_references":
		method := "textDocument/documentSymbol"
		if role == "owner_read_references" {
			method = "textDocument/references"
		}
		return map[string]any{"schema_version": "REFERENCES_OWNER_READ_OBSERVATION_V1", "session_id": "s", "generation": 1, "request_key": "k", "invocation_id": "i", "method": method, "wire_id": 1, "write_selector": "adr0011-references-write-v1-" + strings.TrimPrefix(d, "sha256:") + ".bin", "write_digest": d, "write_byte_length": 100, "params_offset": 20, "params_byte_length": 20, "params_digest": d, "read_selector": "adr0011-references-read-v1-" + strings.TrimPrefix(d, "sha256:") + ".bin", "read_digest": d, "read_byte_length": 100, "result_presence": "PRESENT", "result_offset": 20, "result_byte_length": 20, "result_digest": d}
	case "prepared_source":
		return map[string]any{"schema_version": "REFERENCES_PREPARED_SOURCE_V1", "uri": "file:///tmp/a.go", "version": 1, "encoding": "utf-16", "text_digest": d, "text_byte_length": 1}
	case "source_identity":
		return map[string]any{"schema_version": "REFERENCES_SOURCE_IDENTITY_V1", "prepared_uri": "file:///tmp/a.go", "prepared_version": 1, "prepared_digest": d, "prepared_source_ref": ref("REFERENCES_PREPARED_SOURCE_V1"), "position_encoding": "utf-16"}
	case "revision_identity":
		return map[string]any{"schema_version": "REFERENCES_REVISION_IDENTITY_V1", "root_uri": "file:///tmp", "commit": strings.Repeat("a", 40), "custody": "CALLER_ASSERTED", "host_git_before_ref": ref("REFERENCES_HOST_GIT_OBSERVATION_V1"), "host_git_after_ref": ref("REFERENCES_HOST_GIT_OBSERVATION_V1")}
	case "host_git_before", "host_git_after":
		phase := "BEFORE"
		if role == "host_git_after" {
			phase = "AFTER"
		}
		output := map[string]any{"selector": "adr0011-references-host-git-output-v1-" + strings.TrimPrefix(d, "sha256:") + ".bin", "digest": d, "byte_length": 0}
		command := func(args ...string) map[string]any {
			return map[string]any{"argv": args, "exit_status": 0, "stdout": output, "stderr": output, "observed_at": "2026-09-24T03:00:00Z"}
		}
		return map[string]any{"schema_version": "REFERENCES_HOST_GIT_OBSERVATION_V1", "root_uri": "file:///tmp", "commit": strings.Repeat("a", 40), "dirty": false, "observation_phase": phase, "custody": "HOST_OBSERVED_GIT", "executable_uri": "file:///usr/bin/git", "executable_digest": d, "cwd_uri": "file:///tmp", "commands": []any{command("git", "rev-parse", "--show-toplevel"), command("git", "rev-parse", "HEAD"), command("git", "status", "--porcelain=v1", "--untracked-files=all")}}
	}
	panic("unselected historical role: " + role)
}
func admissionContext() map[string]any {
	d := digestA
	ref := func(v string) map[string]any {
		selector := "adr0011-" + v + ".json"
		if v == "lsp-trace.adr0011.references-symbol.target.v1" {
			selector = "adr0011-target-" + strings.TrimPrefix(d, "sha256:") + ".json"
		}
		if v == "lsp-trace.adr0011.references-symbol.method.v1" {
			selector = "adr0011-method-" + strings.TrimPrefix(d, "sha256:") + ".json"
		}
		return map[string]any{"selector": selector, "digest": d, "schema_version": v}
	}
	return map[string]any{"transaction_id": d, "query_occurrence_id": d, "method_ref": ref("lsp-trace.adr0011.references-symbol.method.v1"), "target_ref": ref("lsp-trace.adr0011.references-symbol.target.v1"), "source_identity_ref": ref("REFERENCES_SOURCE_IDENTITY_V1"), "revision_identity_ref": ref("REFERENCES_REVISION_IDENTITY_V1"), "method_policy_ref": ref("REFERENCES_METHOD_POLICY_V1"), "admission_policy_ref": ref("REFERENCES_ADMISSION_POLICY_V1"), "privacy_policy_ref": ref("LOCAL_QUALIFICATION_PRIVACY_V1"), "retention_policy_ref": ref("REFERENCES_RETENTION_POLICY_V1"), "implementation_digest": d, "schema_digests": []any{map[string]any{"role": "proposal", "digest": d}, map[string]any{"role": "candidate", "digest": d}, map[string]any{"role": "final", "digest": d}, map[string]any{"role": "event", "digest": d}}, "session_id": "s", "generation": 1, "request_key": "k", "invocation_id": "i"}
}

// The new fixture uses a retained synthetic raw result, never provider material.
var oneLocationPayload = []byte(`[{"uri":"file:///tmp/synthetic.go","range":{"start":{"line":2,"character":3},"end":{"line":2,"character":7}}}]`)
var oneLocationSource = []byte("package synthetic\nfunc A() {}\n")

const oneLocationRequestKey = "lsp-trace.request-key.v1:g=1;id=1"

func oneLocationWire() (write, read, params []byte) {
	return oneLocationWireResult(oneLocationPayload)
}

func oneLocationWireResult(readResult []byte) (write, read, params []byte) {
	return oneLocationWireVariant(readResult, false)
}

func oneLocationWireVariant(readResult []byte, withError bool) (write, read, params []byte) {
	params = []byte(`{"textDocument":{"uri":"file:///tmp/a.go"},"position":{"line":0,"character":0},"context":{"includeDeclaration":false}}`)
	write = append([]byte(`{"jsonrpc":"2.0","id":1,"method":"textDocument/references","params":`), params...)
	write = append(write, '}')
	read = append([]byte(`{"jsonrpc":"2.0","id":1,"result":`), readResult...)
	if withError {
		read = append(read, []byte(`,"error":{"code":-32603,"message":"failure"}`)...)
	}
	read = append(read, '}')
	return
}

func oneLocationID(context map[string]any, uri string, rng map[string]any) string {
	canonical := func(x any) []byte { b, _ := json.Marshal(x); return b }
	parts := [][]byte{[]byte("REFERENCES_OCCURRENCE_V1"), []byte(context["transaction_id"].(string)), []byte(context["query_occurrence_id"].(string)), canonical(context["method_ref"]), canonical(context["target_ref"]), []byte(strconv.Itoa(0)), canonical(uri), canonical(rng)}
	h := sha256.New()
	for _, part := range parts {
		var size [8]byte
		for j := 7; j >= 0; j-- {
			size[j] = byte(len(part) >> (8 * (7 - j)))
		}
		h.Write(size[:])
		h.Write(part)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func populateOneLocationHistorical(role string, original map[string]any, readResult []byte, readError bool, selectedWrite ...[]byte) {
	// The historical invocation string is unrestricted; the new admission
	// schema requires the same independently selected identity as a digest.
	var replaceInvocation func(any)
	replaceInvocation = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for k, child := range v {
				if (k == "invocation_id" || k == "InvocationID") && child == "i" {
					v[k] = digestA
				} else {
					replaceInvocation(child)
				}
			}
		case []any:
			for _, child := range v {
				replaceInvocation(child)
			}
		}
	}
	replaceInvocation(original)
	var replaceKey func(any)
	replaceKey = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for k, child := range v {
				if (k == "request_key" || k == "RequestKey") && child == "k" {
					v[k] = oneLocationRequestKey
				} else {
					replaceKey(child)
				}
			}
		case []any:
			for _, child := range v {
				replaceKey(child)
			}
		}
	}
	replaceKey(original)
	sourceDigest := canonicalDigest(oneLocationSource)
	switch role {
	case "production_query_declaration":
		original["document_digest"], original["document_byte_length"], original["source_digest"] = sourceDigest, len(oneLocationSource), sourceDigest
		policyDigests := map[string]any{}
		for _, name := range []string{"method", "admission", "privacy", "retention"} {
			body, err := os.ReadFile("../policies/adr0011-references-" + name + "-policy-v1.proposed.json")
			if err != nil {
				panic(err)
			}
			policyDigests[name] = canonicalDigest(body)
		}
		original["policy_digests"] = policyDigests
	case "prepared_source":
		original["text_digest"], original["text_byte_length"] = sourceDigest, len(oneLocationSource)
	case "source_identity":
		original["prepared_digest"] = sourceDigest
	}
	payloadHash := sha256.Sum256(oneLocationPayload)
	payloadDigest := "sha256:" + hex.EncodeToString(payloadHash[:])
	switch role {
	case "candidate":
		ctx := original["context"].(map[string]any)
		rng := map[string]any{"start": map[string]any{"line": 2, "character": 3}, "end": map[string]any{"line": 2, "character": 7}}
		uri := "file:///tmp/synthetic.go"
		original["occurrences"] = []any{map[string]any{"ordinal": 0, "returned_uri": uri, "returned_range": rng, "occurrence_id": oneLocationID(ctx, uri, rng)}}
		original["proposed_p"] = 1
	case "proposal":
		original["raw_result_digest"], original["raw_result_byte_length"] = payloadDigest, len(oneLocationPayload)
		original["known_e"], original["observed_e_b"], original["observed_e_t"], original["whole_result_p"] = 1, 1, 1, 1
		original["proposed_outcome"], original["proposed_disposition"] = "COMPLETE", "ITEMS"
	case "events":
		original["events"] = []any{map[string]any{"sequence": 0, "kind": "QUERY_BEGIN", "ordinal": nil, "terminal_disposition": "NONE"}, map[string]any{"sequence": 1, "kind": "ELEMENT_BEGIN", "ordinal": 0, "terminal_disposition": "NONE"}, map[string]any{"sequence": 2, "kind": "ELEMENT_TERMINAL", "ordinal": 0, "terminal_disposition": "VALID_PENDING_ADMISSION"}, map[string]any{"sequence": 3, "kind": "QUERY_TERMINAL", "ordinal": nil, "terminal_disposition": "ITEMS"}}
	case "scanner":
		original["known_e"], original["work_units"] = 1, 2
	case "raw_result":
		original["payload_digest"], original["payload_byte_length"], original["payload_selector"] = payloadDigest, len(oneLocationPayload), "adr0011-references-raw-payload-v1-"+hex.EncodeToString(payloadHash[:])+".bin"
	case "response_read":
		write, read, _ := oneLocationWireVariant(readResult, readError)
		if len(selectedWrite) != 0 {
			write = selectedWrite[0]
		}
		original["raw_result_digest"], original["raw_result_byte_length"] = canonicalDigest(readResult), len(readResult)
		original["write_digest"], original["read_digest"] = canonicalDigest(write), canonicalDigest(read)
	case "references_method_record":
		_, _, params := oneLocationWire()
		if len(selectedWrite) != 0 {
			var request struct {
				Params json.RawMessage `json:"params"`
			}
			_ = json.Unmarshal(selectedWrite[0], &request)
			params = request.Params
		}
		original["ParamsDigest"] = canonicalDigest(params)
	case "owner_read_references":
		write, read, params := oneLocationWireVariant(readResult, readError)
		if len(selectedWrite) != 0 {
			write = selectedWrite[0]
			var request struct {
				Params json.RawMessage `json:"params"`
			}
			_ = json.Unmarshal(write, &request)
			params = request.Params
		}
		original["write_digest"], original["write_byte_length"] = canonicalDigest(write), len(write)
		original["read_digest"], original["read_byte_length"] = canonicalDigest(read), len(read)
		original["write_selector"] = "adr0011-references-write-v1-" + strings.TrimPrefix(canonicalDigest(write), "sha256:") + ".bin"
		original["read_selector"] = "adr0011-references-read-v1-" + strings.TrimPrefix(canonicalDigest(read), "sha256:") + ".bin"
		original["params_offset"], original["params_byte_length"], original["params_digest"] = bytes.Index(write, params), len(params), canonicalDigest(params)
		original["result_offset"], original["result_byte_length"], original["result_digest"] = bytes.Index(read, readResult), len(readResult), canonicalDigest(readResult)
	}
}

func validAdmissionModel() (map[string]any, map[string]any, map[string]any, admissionExpectation) {
	return admissionModel(false)
}

func admissionModel(items bool) (map[string]any, map[string]any, map[string]any, admissionExpectation) {
	return admissionModelRead(items, oneLocationPayload)
}

func admissionModelRead(items bool, readResult []byte) (map[string]any, map[string]any, map[string]any, admissionExpectation) {
	return admissionModelWire(items, readResult, false)
}

func admissionModelWire(items bool, readResult []byte, readError bool, selectedWrite ...[]byte) (map[string]any, map[string]any, map[string]any, admissionExpectation) {
	d := declaration()
	i := inventory()
	f := finalAdmission()
	refs := admissionRefs()
	e := admissionExpectation{PolicyBytes: map[string][]byte{}, Stored: map[string][]byte{}, SelectedRoles: map[string]string{}, ExpectedClosure: map[string]map[string]any{}, InventorySelector: "inventory.json", InvocationAfterDeclaration: true, TargetUnique: true, FinalReadback: true}
	built := map[string]map[string]any{}
	active := map[string]bool{}
	var build func(string) map[string]any
	build = func(role string) map[string]any {
		if ref := built[role]; ref != nil {
			return cloneProposal(ref)
		}
		if active[role] {
			panic("cyclic historical schema graph at " + role)
		}
		active[role] = true
		original := admissionOriginal(role, d)
		if items {
			populateOneLocationHistorical(role, original, readResult, readError, selectedWrite...)
		}
		if items && role == "owner_read_references" {
			write, read, _ := oneLocationWireVariant(readResult, readError)
			if len(selectedWrite) != 0 {
				write = selectedWrite[0]
			}
			e.Stored[original["write_selector"].(string)] = append([]byte(nil), write...)
			e.Stored[original["read_selector"].(string)] = append([]byte(nil), read...)
		}
		if items && role == "raw_result" {
			e.Stored[original["payload_selector"].(string)] = append([]byte(nil), oneLocationPayload...)
		}
		if strings.HasPrefix(role, "policy_") {
			name := strings.TrimPrefix(role, "policy_")
			payload, err := os.ReadFile("../policies/adr0011-references-" + name + "-policy-v1.proposed.json")
			if err != nil {
				panic(err)
			}
			h := sha256.Sum256(payload)
			selector := "adr0011-references-policy-bytes-v1-" + hex.EncodeToString(h[:]) + ".json"
			original["policy_bytes_ref"] = map[string]any{"selector": selector, "digest": "sha256:" + hex.EncodeToString(h[:])}
			e.PolicyBytes[role], e.Stored[selector] = payload, payload
		}
		// The historical schema uses role versions inside nested refs, not
		// admission inventory labels. Disambiguate shared owner/Git versions
		// from the containing record and the field's original purpose.
		var bind func(any, string)
		bind = func(value any, field string) {
			switch x := value.(type) {
			case map[string]any:
				if version, ok := x["schema_version"].(string); ok && x["selector"] != nil {
					if nestedRole, ok := x["role"].(string); ok && strings.HasPrefix(version, historicalID) {
						for k, v := range build(nestedRole) {
							x[k] = v
						}
						return
					}
					childRole := ""
					for candidate, v := range map[string]string{"candidate": "REFERENCES_OCCURRENCE_CANDIDATE_V1", "proposal": "REFERENCES_EVALUATION_PROPOSAL_V1", "events": "REFERENCES_EVALUATOR_EVENTS_V1", "scanner": "REFERENCES_SCANNER_OBSERVATION_V1", "raw_result": "REFERENCES_RAW_RESULT_V1", "response_read": "REFERENCES_RESPONSE_READ_V1", "references_method_record": "lsp-trace.adr0011.references-symbol.method.v1", "target_record": "lsp-trace.adr0011.references-symbol.target.v1", "document_symbol_target_result": "REFERENCES_TARGET_RESULT_V1", "prepared_source": "REFERENCES_PREPARED_SOURCE_V1", "source_identity": "REFERENCES_SOURCE_IDENTITY_V1", "revision_identity": "REFERENCES_REVISION_IDENTITY_V1", "policy_method": "REFERENCES_METHOD_POLICY_V1", "policy_admission": "REFERENCES_ADMISSION_POLICY_V1", "policy_privacy": "LOCAL_QUALIFICATION_PRIVACY_V1", "policy_retention": "REFERENCES_RETENTION_POLICY_V1"} {
						if version == v {
							childRole = candidate
							break
						}
					}
					if version == "REFERENCES_OWNER_READ_OBSERVATION_V1" {
						childRole = "owner_read_references"
						if role == "target_record" || role == "document_symbol_target_result" || role == "production_query_declaration" {
							childRole = "owner_read_symbol"
						}
					}
					if version == "REFERENCES_HOST_GIT_OBSERVATION_V1" {
						childRole = "host_git_before"
						if strings.Contains(strings.ToLower(field), "after") {
							childRole = "host_git_after"
						}
					}
					if childRole == "" {
						panic("unselected nested role " + version)
					}
					selected := build(childRole)
					x["selector"], x["digest"] = selected["selector"], selected["digest"]
					return
				}
				for k, v := range x {
					bind(v, k)
				}
			case []any:
				for _, v := range x {
					bind(v, field)
				}
			}
		}
		bind(original, "")
		if items && role != "production_query_declaration" {
			if ctx, ok := original["context"].(map[string]any); ok {
				ctx["query_occurrence_id"] = d["occurrence_id"]
			}
			if role == "target_record" {
				original["QueryOccurrenceID"] = d["occurrence_id"]
			}
		}
		if items && role == "candidate" {
			occ := original["occurrences"].([]any)[0].(map[string]any)
			occ["occurrence_id"] = oneLocationID(original["context"].(map[string]any), occ["returned_uri"].(string), occ["returned_range"].(map[string]any))
		}
		if role == "production_query_declaration" {
			original["occurrence_id"] = roleDigest("ADR0011_PRODUCTION_QUERY_DECLARATION_V1", original, "occurrence_id")
			d = original
		}
		body, _ := json.Marshal(original)
		ref := admissionRef(role)
		ref["digest"], ref["byte_length"] = originalAdmissionDigest(role, body), len(body)
		hash := sha256.Sum256(body)
		switch role {
		case "references_method_record":
			ref["selector"] = "adr0011-method-" + hex.EncodeToString(hash[:]) + ".json"
		case "target_record":
			ref["selector"] = "adr0011-target-" + hex.EncodeToString(hash[:]) + ".json"
		case "document_symbol_target_result":
			ref["selector"] = "adr0011-references-issuance-v1-target-result-" + hex.EncodeToString(hash[:]) + ".json"
		}
		selector := ref["selector"].(string)
		e.Stored[selector], e.ExpectedClosure[selector], e.SelectedRoles[role] = body, cloneProposal(ref), admissionRoleURI(role)
		built[role] = ref
		delete(active, role)
		return cloneProposal(ref)
	}
	if items {
		build("production_query_declaration")
		e.Stored["synthetic-source-original"] = append([]byte(nil), oneLocationSource...)
	}
	for _, r := range refs {
		m := r.(map[string]any)
		for k, v := range build(m["role"].(string)) {
			m[k] = v
		}
	}
	e.Plan = cloneProposal(d)
	sort.Slice(refs, func(a, b int) bool {
		x, _ := json.Marshal(refs[a])
		y, _ := json.Marshal(refs[b])
		return bytes.Compare(x, y) < 0
	})
	e.Refs = refs
	// Expectations come from retained selected originals and the host plan, never f.
	selectedRef := func(role string) map[string]any {
		for _, item := range refs {
			ref := item.(map[string]any)
			if ref["role"] == role {
				return ref
			}
		}
		panic("missing selected role " + role)
	}
	e.ExpectedTargetDigest = selectedRef("target_record")["digest"].(string)
	e.ExpectedResultDigest = selectedRef("raw_result")["digest"].(string)
	e.ExpectedAccountingDigest = selectedRef("candidate")["digest"].(string)
	var method map[string]any
	methodSelector := selectedRef("references_method_record")["selector"].(string)
	if err := json.Unmarshal(e.Stored[methodSelector], &method); err != nil {
		panic(err)
	}
	e.ExpectedRequestKey = method["RequestKey"].(string)
	e.ExpectedInvocationID = e.Plan["occurrence_id"].(string)
	if items {
		e.ExpectedInvocationID = method["InvocationID"].(string)
	}

	i["declaration_id"] = d["occurrence_id"]
	i["dependency_refs"] = refs
	i["inventory_id"] = roleDigest("ADR0011_PRODUCTION_DEPENDENCY_INVENTORY_V1", i, "inventory_id")
	f["dependency_refs"] = refs
	ctx := f["context"].(map[string]any)
	ctx["declaration_id"] = d["occurrence_id"]
	ctx["target_digest"] = e.ExpectedTargetDigest
	ctx["result_digest"] = e.ExpectedResultDigest
	ctx["accounting_digest"] = e.ExpectedAccountingDigest
	ctx["request_key"] = e.ExpectedRequestKey
	ctx["invocation_id"] = e.ExpectedInvocationID
	ctx["inventory_id"] = i["inventory_id"]
	b, _ := json.Marshal(i)
	h := sha256.Sum256(b)
	ctx["inventory_digest"] = "sha256:" + hex.EncodeToString(h[:])
	ctx["inventory_byte_length"] = len(b)
	fb, _ := json.Marshal(f)
	fh := sha256.Sum256(fb)
	e.FinalDigest = roleDigest("ADR0011_PRODUCTION_ADMISSION_FINAL_V1", f, "")
	e.FinalSelector = "adr0011-production-final-v1-" + hex.EncodeToString(fh[:]) + ".json"
	return d, i, f, e
}

// The positive chain retains complete historical records and the four pinned
// raw policy payloads reached through their immutable envelope records.
func transitiveAdmissionModel() (map[string]any, map[string]any, map[string]any, admissionExpectation) {
	return validAdmissionModel()
}
func TestADR0011AdmissionFinalIndependentContext(t *testing.T) {
	for _, field := range []string{"target_digest", "result_digest", "accounting_digest", "request_key", "invocation_id"} {
		t.Run(field, func(t *testing.T) {
			d, i, f, e := validAdmissionModel()
			if err := replayAdmission(d, i, f, e); err != nil {
				t.Fatalf("unchanged positive rejected: %v", err)
			}
			ctx := f["context"].(map[string]any)
			if field == "request_key" {
				ctx[field] = "other-request"
			} else {
				ctx[field] = digestB
			}
			fb, _ := json.Marshal(f)
			fh := sha256.Sum256(fb)
			e.FinalDigest = roleDigest("ADR0011_PRODUCTION_ADMISSION_FINAL_V1", f, "")
			e.FinalSelector = "adr0011-production-final-v1-" + hex.EncodeToString(fh[:]) + ".json"
			if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionAdmissionFinal").Validate(f); err != nil {
				t.Fatalf("mutated final fails shape before identity guard: %v", err)
			}
			if err := replayAdmission(d, i, f, e); err == nil || !strings.Contains(err.Error(), "final context "+field+" differs") {
				t.Fatalf("expected independent %s guard after closure and final digest/selector checks: %v", field, err)
			}
		})
	}
}

func TestADR0011AdmissionProposalReplay(t *testing.T) {
	t.Run("nonempty transitive closure positive", func(t *testing.T) {
		d, i, f, e := transitiveAdmissionModel()
		for role, obj := range map[string]map[string]any{"productionQueryDeclaration": d, "productionDependencyInventory": i, "productionAdmissionFinal": f} {
			if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, role).Validate(obj); err != nil {
				t.Fatalf("schema-invalid positive %s: %v", role, err)
			}
		}
		refSchema := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "ref")
		for selector, selected := range e.ExpectedClosure {
			if err := refSchema.Validate(selected); err != nil {
				t.Fatalf("schema-invalid selected ref %s: %v", selector, err)
			}
			original, err := canonicalProposal(e.Stored[selector])
			if err != nil {
				t.Fatal(err)
			}
			role := selected["role"].(string)
			if err := validateAdmissionOriginal(role, original); err != nil {
				t.Fatalf("schema-invalid original %s: %v", selector, err)
			}
		}
		if err := replayAdmission(d, i, f, e); err != nil {
			t.Fatalf("valid nonempty closure rejected: %v", err)
		}
	})
	for name, mutate := range map[string]func(map[string]any){
		"nested substituted digest": func(candidate map[string]any) {
			candidate["context"].(map[string]any)["method_ref"].(map[string]any)["digest"] = digestB
		},
		"nested missing bytes": func(candidate map[string]any) {
			candidate["context"].(map[string]any)["method_ref"].(map[string]any)["selector"] = "adr0011-method-" + strings.Repeat("b", 64) + ".json"
		},
		"nested wrong schema role (historical schema rejects first)": func(candidate map[string]any) {
			candidate["context"].(map[string]any)["privacy_policy_ref"].(map[string]any)["schema_version"] = "REFERENCES_METHOD_POLICY_V1"
		},
	} {
		t.Run(name+" after immediate guards pass", func(t *testing.T) {
			d, i, f, e := transitiveAdmissionModel()
			selector := "ref-candidate.json"
			candidate, err := canonicalProposal(e.Stored[selector])
			if err != nil {
				t.Fatal(err)
			}
			mutate(candidate)
			body, _ := json.Marshal(candidate)
			e.Stored[selector] = body
			for _, r := range e.Refs {
				m := r.(map[string]any)
				if m["selector"] == selector {
					m["digest"], m["byte_length"] = originalAdmissionDigest("candidate", body), len(body)
					e.ExpectedClosure[selector] = cloneProposal(m)
				}
			}
			i["inventory_id"] = roleDigest("ADR0011_PRODUCTION_DEPENDENCY_INVENTORY_V1", i, "inventory_id")
			ctx := f["context"].(map[string]any)
			ctx["inventory_id"] = i["inventory_id"]
			ib, _ := json.Marshal(i)
			ih := sha256.Sum256(ib)
			ctx["inventory_digest"], ctx["inventory_byte_length"] = "sha256:"+hex.EncodeToString(ih[:]), len(ib)
			fb, _ := json.Marshal(f)
			fh := sha256.Sum256(fb)
			e.FinalDigest = roleDigest("ADR0011_PRODUCTION_ADMISSION_FINAL_V1", f, "")
			e.FinalSelector = "adr0011-production-final-v1-" + hex.EncodeToString(fh[:]) + ".json"
			if err := validateAdmissionOriginal("candidate", candidate); err != nil {
				if strings.Contains(name, "historical schema rejects first") {
					return
				}
				t.Fatal(err)
			}
			if strings.Contains(name, "historical schema rejects first") {
				t.Fatal("wrong-schema fixture passed its historical role schema")
			}
			if err := replayAdmission(d, i, f, e); err == nil || !strings.Contains(err.Error(), "nested") {
				t.Fatalf("corruption did not reach traversal: %v", err)
			}
		})
	}
	for name, mutate := range map[string]func(*admissionExpectation){
		"selected role URI":           func(e *admissionExpectation) { e.SelectedRoles["proposal"] = lifecycleID + "#/$defs/ref" },
		"selected ancestor identity":  func(e *admissionExpectation) { e.ExpectedClosure["ref-proposal.json"]["digest"] = digestB },
		"selected inventory selector": func(e *admissionExpectation) { e.InventorySelector = "other.json" },
		"selected final selector":     func(e *admissionExpectation) { e.FinalSelector = "other.json" },
	} {
		t.Run(name, func(t *testing.T) {
			d, i, f, e := transitiveAdmissionModel()
			mutate(&e)
			if replayAdmission(d, i, f, e) == nil {
				t.Fatal("accepted selected expectation mismatch")
			}
		})
	}
	// These five substitutions reject at the immediate root guard; they do not
	// claim to exercise transitive traversal.
	for name, mutate := range map[string]func(*admissionExpectation){
		"minimal-role substitution": func(e *admissionExpectation) {
			e.Stored["ref-proposal.json"] = []byte(`{"schema_version":"REFERENCES_EVALUATION_PROPOSAL_V1"}`)
		},
		"wrong predecessor schema ID": func(e *admissionExpectation) {
			e.SelectedRoles["proposal"] = historicalID + "#/$defs/candidate"
		},
		"truncated original bytes": func(e *admissionExpectation) {
			b := e.Stored["ref-proposal.json"]
			e.Stored["ref-proposal.json"] = append([]byte(nil), b[:len(b)-1]...)
		},
		"equivalent reserialization": func(e *admissionExpectation) {
			b := e.Stored["ref-proposal.json"]
			e.Stored["ref-proposal.json"] = append([]byte(" "), b...)
		},
		"dependency-role swap": func(e *admissionExpectation) {
			e.Stored["ref-host_git_before.json"], e.Stored["ref-host_git_after.json"] = e.Stored["ref-host_git_after.json"], e.Stored["ref-host_git_before.json"]
		},
	} {
		t.Run(name, func(t *testing.T) {
			d, i, f, e := transitiveAdmissionModel()
			mutate(&e)
			if replayAdmission(d, i, f, e) == nil {
				t.Fatal("corrupt historical closure admitted")
			}
		})
	}
	t.Run("missing immediate ancestor original", func(t *testing.T) {
		d, i, f, e := transitiveAdmissionModel()
		delete(e.Stored, "ref-proposal.json")
		if replayAdmission(d, i, f, e) == nil {
			t.Fatal("missing ancestor original accepted")
		}
	})
	t.Run("substituted immediate ancestor original", func(t *testing.T) {
		d, i, f, e := transitiveAdmissionModel()
		e.Stored["ref-proposal.json"] = []byte(`{"role":"other"}`)
		if replayAdmission(d, i, f, e) == nil {
			t.Fatal("substituted ancestor original accepted")
		}
	})
	t.Run("guard exact roles and transitive original-byte closure", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			mutate func(map[string]any, map[string]any, map[string]any, *admissionExpectation)
		}{
			{"twenty immediate roles with intact closure", func(d, i, f map[string]any, e *admissionExpectation) {
				refs := append([]any{}, e.Refs[:20]...)
				e.Refs = refs
				i["dependency_refs"], f["dependency_refs"] = refs, refs
				i["inventory_id"] = roleDigest("ADR0011_PRODUCTION_DEPENDENCY_INVENTORY_V1", i, "inventory_id")
				ctx := f["context"].(map[string]any)
				ctx["inventory_id"] = i["inventory_id"]
				b, _ := json.Marshal(i)
				h := sha256.Sum256(b)
				ctx["inventory_digest"], ctx["inventory_byte_length"] = "sha256:"+hex.EncodeToString(h[:]), len(b)
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				d, i, f, e := validAdmissionModel()
				tc.mutate(d, i, f, &e)
				if replayAdmission(d, i, f, e) == nil {
					t.Fatal("admission guard accepted violating state")
				}
			})
		}
	})
	d, i, f, e := validAdmissionModel()
	if err := replayAdmission(d, i, f, e); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(map[string]any, map[string]any, map[string]any, *admissionExpectation){"altered final bytes": func(d, i, f map[string]any, e *admissionExpectation) {
		f["context"].(map[string]any)["result_digest"] = digestB
	}, "altered final selector": func(d, i, f map[string]any, e *admissionExpectation) {
		e.FinalSelector = "other.json"
	}, "substituted occurrence ID": func(d, i, f map[string]any, e *admissionExpectation) { d["occurrence_id"] = digestB }, "caller-selected plan": func(d, i, f map[string]any, e *admissionExpectation) { d["plan_id"] = digestB }, "invocation before declaration": func(d, i, f map[string]any, e *admissionExpectation) { e.InvocationAfterDeclaration = false }, "wrong ref order": func(d, i, f map[string]any, e *admissionExpectation) {
		r := f["dependency_refs"].([]any)
		r[0], r[1] = r[1], r[0]
	}, "swapped owner role": func(d, i, f map[string]any, e *admissionExpectation) {
		r := f["dependency_refs"].([]any)
		for _, x := range r {
			m := x.(map[string]any)
			if m["role"] == "owner_read_symbol" {
				m["selector"] = "wrong.json"
			}
		}
	}, "swapped Git role": func(d, i, f map[string]any, e *admissionExpectation) {
		r := f["dependency_refs"].([]any)
		for _, x := range r {
			m := x.(map[string]any)
			if m["role"] == "host_git_before" {
				m["selector"] = "ref-host_git_after.json"
			}
		}
	}, "policy predecessor changed": func(d, i, f map[string]any, e *admissionExpectation) {
		e.Stored["ref-policy_privacy.json"] = []byte("altered")
	}, "ambiguous target": func(d, i, f map[string]any, e *admissionExpectation) { e.TargetUnique = false }, "committed unverified final": func(d, i, f map[string]any, e *admissionExpectation) { e.FinalReadback = false }}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d, i, f, e := validAdmissionModel()
			mutate(d, i, f, &e)
			for role, x := range map[string]map[string]any{"productionQueryDeclaration": d, "productionDependencyInventory": i, "productionAdmissionFinal": f} {
				s := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, role)
				if err := s.Validate(x); err != nil {
					t.Fatalf("semantic fixture must be schema-valid: %v", err)
				}
			}
			if replayAdmission(d, i, f, e) == nil {
				t.Fatal("unexpected T/A eligibility")
			}
		})
	}
}

// oneLocationReplay freezes the host's choices separately from inventory/final
// claimants. The maps in originals are the independent readback boundary.
type oneLocationReplay struct {
	d, inventory, final map[string]any
	e                   admissionExpectation
	originals           map[string][]byte
	selected            map[string]map[string]any
	// Invocation is frozen prewrite; key is independently observed from the completed framed WRITE.
	frozenRequestKey, frozenInvocation string
	// Owner-selected originals in observed WRITE then READ order, external to the graph.
	observedFrames     [2][]byte
	selectedRawPayload []byte
	verified           bool
}

func canonicalDigest(body []byte) string {
	h := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(h[:])
}

func newOneLocationReplay(t *testing.T) oneLocationReplay {
	return newOneLocationReplayRead(t, oneLocationPayload)
}

func newOneLocationReplayRead(t *testing.T, readResult []byte) oneLocationReplay {
	return newOneLocationReplayWire(t, readResult, false)
}

func newOneLocationReplayWire(t *testing.T, readResult []byte, readError bool, selectedWrite ...[]byte) oneLocationReplay {
	t.Helper()
	d, i, f, e := admissionModelWire(true, readResult, readError, selectedWrite...)
	write, read, _ := oneLocationWireVariant(readResult, readError)
	if len(selectedWrite) != 0 {
		write = selectedWrite[0]
	}
	resultRef := selectedAdmissionRef(e.Refs, "raw_result")
	result, err := canonicalProposal(e.Stored[resultRef["selector"].(string)])
	if err != nil {
		t.Fatal(err)
	}
	selectedRaw := append([]byte(nil), e.Stored[result["payload_selector"].(string)]...)
	x := oneLocationReplay{d: d, inventory: i, final: f, e: e, originals: map[string][]byte{}, selected: map[string]map[string]any{}, frozenRequestKey: oneLocationRequestKey, frozenInvocation: digestA, observedFrames: [2][]byte{append([]byte(nil), write...), append([]byte(nil), read...)}, selectedRawPayload: selectedRaw, verified: true}
	// A host owns these originals before it selects the invocation. None is
	// recovered from the claimant's inventory, ledgers, or final receipt.
	store := func(role, definition string, record map[string]any) map[string]any {
		body, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		ref := attemptRef(role, definition)
		if strings.HasPrefix(role, "production_") {
			ref = ancillaryRef(role, definition)
		}
		ref["digest"], ref["byte_length"] = canonicalDigest(body), len(body)
		x.originals[role] = append([]byte(nil), body...)
		x.selected[role] = cloneProposal(ref)
		return cloneProposal(ref)
	}
	declarationRef := selectedAdmissionRef(e.Refs, "production_query_declaration")
	key, invocation := x.frozenRequestKey, x.frozenInvocation
	policyRef := store("attempt_witness_policy", "attemptWitnessPolicy", map[string]any{
		"role": "ADR0011_ATTEMPT_WITNESS_POLICY_V1", "version": 1,
		"declaration_ref": declarationRef, "selected_roles": []any{"attempt_initiation", "attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge", "attempt_request_key_observation"}, "host_selected_before_invocation": true,
	})
	custodyRef := store("attempt_host_custody", "attemptHostCustody", map[string]any{
		"role": "ADR0011_ATTEMPT_HOST_CUSTODY_V1", "version": 1, "policy_ref": policyRef,
		"declaration_ref": declarationRef, "invocation_id": invocation,
		"host_session_id": "s", "host_generation": 1, "selection_phase": "PRE_INVOCATION",
		"policy_readback_digest": policyRef["digest"], "declaration_readback_digest": canonicalDigest(e.Stored[declarationRef["selector"].(string)]),
	})
	base := func(role string) map[string]any {
		return map[string]any{"role": role, "version": 1, "declaration_id": d["occurrence_id"], "request_key": key, "invocation_id": invocation, "policy_ref": policyRef}
	}
	init := base("ADR0011_ATTEMPT_INITIATION_V1")
	delete(init, "request_key")
	init["host_session_id"], init["host_generation"] = "s", 1
	init["declared_n"], init["declaration_ref"] = 1, declarationRef
	initRef := store("attempt_initiation", "attemptInitiation", init)
	keyObservationRef := store("attempt_request_key_observation", "attemptRequestKeyObservation", map[string]any{
		"role": "ADR0011_ATTEMPT_REQUEST_KEY_OBSERVATION_V1", "version": 1, "policy_ref": policyRef,
		"declaration_id": d["occurrence_id"], "custody_ref": custodyRef, "request_key": key,
		"invocation_id": invocation, "host_session_id": "s", "host_generation": 1,
		"wire_id": 1, "write_selector": "original-write.json", "write_digest": canonicalDigest(write),
		"write_byte_length": len(write), "observation_phase": "COMPLETED_FRAMED_WRITE",
	})
	begin := base("ADR0011_ATTEMPT_QUERY_BEGIN_V1")
	begin["key_observation_ref"] = keyObservationRef
	begin["query_ordinal"], begin["response_key"], begin["evaluation_began"] = 0, key, true
	beginRef := store("attempt_query_begin", "attemptQueryBegin", begin)
	terminal := base("ADR0011_ATTEMPT_TERMINAL_JOURNAL_V1")
	terminal["begin_ref"], terminal["query_ordinal"] = beginRef, 0
	terminal["member_disposition"], terminal["method_outcome"] = "ITEMS", "COMPLETE"
	terminalRef := store("attempt_terminal_journal", "attemptTerminalJournal", terminal)
	scanner := base("ADR0011_ATTEMPT_SCANNER_KNOWLEDGE_V1")
	scanner["begin_ref"], scanner["e"], scanner["e_b"], scanner["e_t"], scanner["top_level_state"] = beginRef, 1, 1, 1, "COMPLETE_BOUNDED"
	scannerRef := store("attempt_scanner_knowledge", "attemptScannerKnowledge", scanner)
	manifestRef := store("attempt_evidence_manifest", "attemptEvidenceManifest", map[string]any{
		"role": "ADR0011_ATTEMPT_EVIDENCE_MANIFEST_V1", "version": 1, "policy_ref": policyRef,
		"declaration_id": d["occurrence_id"], "request_key": key, "invocation_id": invocation,
		"initiation_ref": initRef, "custody_ref": custodyRef,
		"key_observation": map[string]any{"status": "PRESENT", "ref": keyObservationRef},
		"begin":           map[string]any{"status": "PRESENT", "ref": beginRef},
		"terminal":        map[string]any{"status": "PRESENT", "ref": terminalRef},
		"scanner":         map[string]any{"status": "PRESENT", "ref": scannerRef},
	})
	candidateRef := selectedAdmissionRef(e.Refs, "candidate")
	targetRef := selectedAdmissionRef(e.Refs, "target_record")
	observed := func(digest any) map[string]any { return map[string]any{"status": "PRESENT", "digest": digest} }
	ledgerBase := func(role string) map[string]any {
		return map[string]any{"role": role, "version": 1, "declaration_id": d["occurrence_id"], "request_key": key, "invocation_id": invocation,
			"policy_digests": d["policy_digests"], "historical_predecessor_refs": []any{candidateRef}, "attempt_manifest_ref": manifestRef}
	}
	terminalLedger := ledgerBase("ADR0011_PRODUCTION_TERMINAL_LEDGER_V1")
	terminalLedger["target"], terminalLedger["result"] = observed(targetRef["digest"]), observed(resultRef["digest"])
	terminalLedger["b"], terminalLedger["t"], terminalLedger["outcome"], terminalLedger["disposition"] = 1, 1, "COMPLETE", "ITEMS"
	terminalLedgerRef := store("production_terminal_ledger", "productionTerminalLedger", terminalLedger)
	candidate, err := canonicalProposal(e.Stored[candidateRef["selector"].(string)])
	if err != nil {
		t.Fatal(err)
	}
	occurrence := candidate["occurrences"].([]any)[0].(map[string]any)["occurrence_id"].(string)
	occurrenceLedger := ledgerBase("ADR0011_PRODUCTION_OCCURRENCE_LEDGER_V1")
	occurrenceLedger["target_digest"], occurrenceLedger["result_digest"] = targetRef["digest"], resultRef["digest"]
	occurrenceLedger["p"] = 1
	occurrenceLedger["parsed_occurrences"] = []any{map[string]any{"identity": occurrence, "evidence_refs": []any{candidateRef}}}
	occurrenceLedger["admitted_occurrences"] = []any{occurrence}
	occurrenceLedgerRef := store("production_occurrence_ledger", "productionOccurrenceLedger", occurrenceLedger)
	accounting := ledgerBase("ADR0011_PRODUCTION_ACCOUNTING_RECORD_V1")
	accounting["target"], accounting["result"] = observed(targetRef["digest"]), observed(resultRef["digest"])
	accounting["terminal_ledger_ref"], accounting["occurrence_ledger_ref"] = terminalLedgerRef, occurrenceLedgerRef
	for _, name := range []string{"n", "b", "t", "e", "e_b", "e_t", "p", "a"} {
		accounting[name] = 1
	}
	accounting["disposition"], accounting["outcome"] = "ITEMS", "COMPLETE"
	accountingRef := store("production_accounting_record", "productionAccountingRecord", accounting)
	i["attempt_manifest_ref"] = manifestRef
	i["ancillary_refs"] = []any{terminalLedgerRef, occurrenceLedgerRef, accountingRef}
	i["inventory_id"] = roleDigest("ADR0011_PRODUCTION_DEPENDENCY_INVENTORY_V1", i, "inventory_id")
	ctx := f["context"].(map[string]any)
	ctx["invocation_id"], ctx["accounting_digest"] = invocation, accountingRef["digest"]
	ctx["inventory_id"] = i["inventory_id"]
	ib, _ := json.Marshal(i)
	ctx["inventory_digest"], ctx["inventory_byte_length"] = canonicalDigest(ib), len(ib)
	x.e.ExpectedAccountingDigest = accountingRef["digest"].(string)
	repinOneLocationFinal(&x)
	return x
}

// A response_key is the canonical generation/wire-ID request key recovered
// from the independently observed write/read pair, not an arbitrary label in
// the begin record. The pair's ordered originals and expected key are external
// host inputs, not chosen from the manifest or final.
func matchedOneLocationResponse(x oneLocationReplay, owner, response map[string]any, begin map[string]any) error {
	write, read := x.observedFrames[0], x.observedFrames[1]
	if len(write) == 0 || len(read) == 0 {
		return fmt.Errorf("matched response: ordered write/read originals absent")
	}
	for _, frame := range [][]byte{write, read} {
		dec := json.NewDecoder(bytes.NewReader(frame))
		if err := uniqueJSON(dec); err != nil {
			return fmt.Errorf("matched response: duplicate/malformed frame: %w", err)
		}
	}
	var request struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	var reply struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
	}
	if err := validOneLocationWriteEnvelope(write); err != nil {
		return err
	}
	if json.Unmarshal(write, &request) != nil || json.Unmarshal(read, &reply) != nil || request.Method != "textDocument/references" {
		return fmt.Errorf("matched response: method/frame mismatch")
	}
	wireID, err := strconv.ParseUint(string(request.ID), 10, 64)
	if err != nil || wireID == 0 || !bytes.Equal(request.ID, reply.ID) {
		return fmt.Errorf("matched response: write/read ID mismatch")
	}
	generation := owner["generation"]
	if generation != float64(1) || owner["wire_id"] != float64(wireID) || owner["session_id"] != "s" || owner["method"] != request.Method || owner["result_presence"] != "PRESENT" {
		return fmt.Errorf("matched response: owner generation/wire mismatch")
	}
	derivedKey := fmt.Sprintf("lsp-trace.request-key.v1:g=%d;id=%d", 1, wireID)
	if derivedKey != x.frozenRequestKey || owner["request_key"] != derivedKey || owner["invocation_id"] != x.frozenInvocation || response["request_key"] != derivedKey || response["invocation_id"] != x.frozenInvocation || begin["request_key"] != derivedKey || begin["response_key"] != derivedKey {
		return fmt.Errorf("cross-record matched-response key mismatch")
	}
	paramsOffset, resultOffset := bytes.Index(write, request.Params), bytes.Index(read, reply.Result)
	if paramsOffset < 0 || resultOffset < 0 || owner["params_offset"] != float64(paramsOffset) || owner["result_offset"] != float64(resultOffset) || owner["params_byte_length"] != float64(len(request.Params)) || owner["result_byte_length"] != float64(len(reply.Result)) || owner["params_digest"] != canonicalDigest(request.Params) || owner["result_digest"] != canonicalDigest(reply.Result) {
		return fmt.Errorf("matched response: span mismatch")
	}
	uri, line, character, err := exactOneLocationParams(request.Params)
	if err != nil {
		return fmt.Errorf("WRITE nested params not valid: %w", err)
	}
	if uri != x.d["query_uri"] || fmt.Sprint(line) != fmt.Sprint(x.d["line"]) || fmt.Sprint(character) != fmt.Sprint(x.d["character"]) {
		return fmt.Errorf("matched response: declared params mismatch")
	}
	for index, body := range [][]byte{write, read} {
		kind := "write"
		if index == 1 {
			kind = "read"
		}
		selector := owner[kind+"_selector"].(string)
		if owner[kind+"_digest"] != canonicalDigest(body) || owner[kind+"_byte_length"] != float64(len(body)) || selector != "adr0011-references-"+kind+"-v1-"+strings.TrimPrefix(canonicalDigest(body), "sha256:")+".bin" || !bytes.Equal(x.e.Stored[selector], body) {
			return fmt.Errorf("matched response: %s original readback mismatch", kind)
		}
	}
	if response["write_digest"] != owner["write_digest"] || response["read_digest"] != owner["read_digest"] || response["raw_result_digest"] != canonicalDigest(reply.Result) || response["raw_result_byte_length"] != float64(len(reply.Result)) {
		return fmt.Errorf("matched response: response-read binding mismatch")
	}
	return nil
}

// The independently selected WRITE must be exactly one JSON-RPC 2.0 request.
// A struct decode alone discards unknown members and does not require jsonrpc.
func validOneLocationWriteEnvelope(body []byte) error {
	fail := func() error { return fmt.Errorf("WRITE request envelope not valid") }
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(body))); err != nil {
		return fail()
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || len(fields) != 4 {
		return fail()
	}
	if !bytes.Equal(fields["jsonrpc"], []byte(`"2.0"`)) ||
		!bytes.Equal(fields["method"], []byte(`"textDocument/references"`)) ||
		len(fields["params"]) == 0 {
		return fail()
	}
	id, err := strconv.ParseUint(string(fields["id"]), 10, 64)
	if err != nil || id == 0 {
		return fail()
	}
	return nil
}

// The selected request's params are closed at every level. The whole-frame
// uniqueJSON check above rejects duplicate names, including nested duplicates.
func exactOneLocationParams(body []byte) (string, uint64, uint64, error) {
	fail := func() (string, uint64, uint64, error) {
		return "", 0, 0, fmt.Errorf("exact textDocument/position/context shape required")
	}
	object := func(raw []byte, keys ...string) (map[string]json.RawMessage, bool) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || len(fields) != len(keys) || fields == nil {
			return nil, false
		}
		for _, key := range keys {
			if _, ok := fields[key]; !ok {
				return nil, false
			}
		}
		return fields, true
	}
	params, ok := object(body, "textDocument", "position", "context")
	if !ok {
		return fail()
	}
	doc, ok := object(params["textDocument"], "uri")
	if !ok {
		return fail()
	}
	position, ok := object(params["position"], "line", "character")
	if !ok {
		return fail()
	}
	context, ok := object(params["context"], "includeDeclaration")
	if !ok || !bytes.Equal(context["includeDeclaration"], []byte("false")) {
		return fail()
	}
	var uri string
	if json.Unmarshal(doc["uri"], &uri) != nil || uri == "" || doc["uri"][0] != '"' {
		return fail()
	}
	line, err := strconv.ParseUint(string(position["line"]), 10, 64)
	if err != nil {
		return fail()
	}
	character, err := strconv.ParseUint(string(position["character"]), 10, 64)
	if err != nil {
		return fail()
	}
	return uri, line, character, nil
}

// The historical owner span can contain a valid result alongside an error.
// Only the exact successful JSON-RPC envelope admits COMPLETE; unknown or
// duplicate members are not silently discarded by a struct decode.
func successfulOneLocationReadEnvelope(body []byte, wireID uint64) error {
	fail := func() error { return fmt.Errorf("READ response envelope not successful") }
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := uniqueJSON(dec); err != nil {
		return fail()
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || len(fields) != 3 {
		return fail()
	}
	if !bytes.Equal(fields["jsonrpc"], []byte(`"2.0"`)) || !bytes.Equal(fields["id"], []byte(strconv.FormatUint(wireID, 10))) || len(fields["result"]) == 0 {
		return fail()
	}
	return nil
}

func repinOneLocationFinal(x *oneLocationReplay) {
	body, _ := json.Marshal(x.final)
	x.e.FinalDigest = roleDigest("ADR0011_PRODUCTION_ADMISSION_FINAL_V1", x.final, "")
	x.e.FinalSelector = "adr0011-production-final-v1-" + strings.TrimPrefix(canonicalDigest(body), "sha256:") + ".json"
}

func replayOneLocation(t *testing.T, x oneLocationReplay) error {
	t.Helper()
	if !x.verified || !x.e.FinalReadback {
		return fmt.Errorf("unverified final readback")
	}
	if err := replayAdmission(x.d, x.inventory, x.final, x.e); err != nil {
		return err
	}
	read := func(role string) (map[string]any, error) {
		ref := x.selected[role]
		body := append([]byte(nil), x.originals[role]...) // fresh external readback, not claimant bytes
		if ref == nil || len(body) == 0 {
			return nil, fmt.Errorf("missing host selected %s", role)
		}
		if err := proposalSelectedOriginal(ref, body, true); err != nil {
			return nil, fmt.Errorf("original %s: %w", role, err)
		}
		record, err := canonicalProposal(body)
		if err != nil {
			return nil, err
		}
		definition := strings.TrimPrefix(ref["schema_version"].(string), admissionID+"#/$defs/")
		if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, definition).Validate(record); err != nil {
			return nil, fmt.Errorf("schema %s: %w", role, err)
		}
		return record, nil
	}
	roles := []string{"attempt_witness_policy", "attempt_host_custody", "attempt_initiation", "attempt_request_key_observation", "attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge", "attempt_evidence_manifest", "production_terminal_ledger", "production_occurrence_ledger", "production_accounting_record"}
	records := map[string]map[string]any{}
	for _, role := range roles {
		record, err := read(role)
		if err != nil {
			return err
		}
		records[role] = record
	}
	refEquals := func(found any, role string) bool {
		a, _ := json.Marshal(found)
		b, _ := json.Marshal(x.selected[role])
		return bytes.Equal(a, b)
	}
	policy, custody, manifest := records["attempt_witness_policy"], records["attempt_host_custody"], records["attempt_evidence_manifest"]
	declarationRef := selectedAdmissionRef(x.e.Refs, "production_query_declaration")
	for _, name := range []string{"method", "admission", "privacy", "retention"} {
		if x.d["policy_digests"].(map[string]any)[name] != canonicalDigest(x.e.PolicyBytes["policy_"+name]) {
			return fmt.Errorf("policy payload digest mismatch %s", name)
		}
	}
	if !refEquals(custody["policy_ref"], "attempt_witness_policy") || !reflect.DeepEqual(policy["declaration_ref"], declarationRef) || !reflect.DeepEqual(custody["declaration_ref"], declarationRef) || custody["selection_phase"] != "PRE_INVOCATION" || custody["policy_readback_digest"] != x.selected["attempt_witness_policy"]["digest"] || custody["declaration_readback_digest"] != canonicalDigest(x.e.Stored[declarationRef["selector"].(string)]) {
		return fmt.Errorf("preinvoke custody mismatch")
	}
	for _, role := range []string{"attempt_initiation", "attempt_request_key_observation", "attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge"} {
		if !refEquals(records[role]["policy_ref"], "attempt_witness_policy") {
			return fmt.Errorf("attempt policy mismatch %s", role)
		}
	}
	for field, role := range map[string]string{"initiation_ref": "attempt_initiation", "custody_ref": "attempt_host_custody", "policy_ref": "attempt_witness_policy"} {
		if !refEquals(manifest[field], role) {
			return fmt.Errorf("manifest %s mismatch", field)
		}
	}
	for field, role := range map[string]string{"key_observation": "attempt_request_key_observation", "begin": "attempt_query_begin", "terminal": "attempt_terminal_journal", "scanner": "attempt_scanner_knowledge"} {
		observed := manifest[field].(map[string]any)
		if observed["status"] != "PRESENT" || !refEquals(observed["ref"], role) {
			return fmt.Errorf("manifest %s absent or mismatched", field)
		}
	}
	if !refEquals(records["attempt_terminal_journal"]["begin_ref"], "attempt_query_begin") || !refEquals(records["attempt_scanner_knowledge"]["begin_ref"], "attempt_query_begin") {
		return fmt.Errorf("attempt begin mismatch")
	}
	key, invocation := x.frozenRequestKey, x.frozenInvocation
	if x.e.ExpectedInvocationID != invocation {
		return fmt.Errorf("frozen invocation mismatch")
	}
	if records["attempt_host_custody"]["request_key"] != nil || records["attempt_initiation"]["request_key"] != nil {
		return fmt.Errorf("prewrite records claim postwrite request key")
	}
	observation := records["attempt_request_key_observation"]
	if err := replayObservedAttemptKey(observation, x.observedFrames[0], key, 1, x.selected["attempt_host_custody"]); err != nil {
		return err
	}
	if !refEquals(observation["policy_ref"], "attempt_witness_policy") || !refEquals(observation["custody_ref"], "attempt_host_custody") || !refEquals(records["attempt_query_begin"]["key_observation_ref"], "attempt_request_key_observation") {
		return fmt.Errorf("postwrite observation binding mismatch")
	}
	for _, role := range roles[1:] {
		r := records[role]
		if r["invocation_id"] != invocation || (role != "attempt_host_custody" && role != "attempt_initiation" && r["request_key"] != key) {
			return fmt.Errorf("%s request_key/invocation_id mismatch", role)
		}
		if role != "attempt_host_custody" && r["declaration_id"] != x.d["occurrence_id"] {
			return fmt.Errorf("%s declaration mismatch", role)
		}
	}
	ownerRef := selectedAdmissionRef(x.e.Refs, "owner_read_references")
	owner, err := canonicalProposal(x.e.Stored[ownerRef["selector"].(string)])
	if err != nil {
		return err
	}
	responseRef := selectedAdmissionRef(x.e.Refs, "response_read")
	response, err := canonicalProposal(x.e.Stored[responseRef["selector"].(string)])
	if err != nil {
		return err
	}
	if err := matchedOneLocationResponse(x, owner, response, records["attempt_query_begin"]); err != nil {
		return err
	}
	scanner := records["attempt_scanner_knowledge"]
	if scanner["top_level_state"] != "COMPLETE_BOUNDED" {
		return fmt.Errorf("scanner absent or UNKNOWN")
	}
	candidateRef := selectedAdmissionRef(x.e.Refs, "candidate")
	candidate, err := canonicalProposal(x.e.Stored[candidateRef["selector"].(string)])
	if err != nil {
		return err
	}
	proposalRef := selectedAdmissionRef(x.e.Refs, "proposal")
	proposal, err := canonicalProposal(x.e.Stored[proposalRef["selector"].(string)])
	if err != nil {
		return err
	}
	resultRef := selectedAdmissionRef(x.e.Refs, "raw_result")
	preparedRef := selectedAdmissionRef(x.e.Refs, "prepared_source")
	prepared, err := canonicalProposal(x.e.Stored[preparedRef["selector"].(string)])
	if err != nil {
		return err
	}
	source := append([]byte(nil), x.e.Stored["synthetic-source-original"]...)
	if !bytes.Equal(source, oneLocationSource) || canonicalDigest(source) != prepared["text_digest"] || canonicalDigest(source) != x.d["document_digest"] || len(source) != int(prepared["text_byte_length"].(float64)) || fmt.Sprint(len(source)) != fmt.Sprint(x.d["document_byte_length"]) {
		return fmt.Errorf("source original mismatch")
	}
	eventsRef := selectedAdmissionRef(x.e.Refs, "events")
	events, err := canonicalProposal(x.e.Stored[eventsRef["selector"].(string)])
	if err != nil {
		return err
	}
	sequence := events["events"].([]any)
	if events["transaction_id"] != candidate["context"].(map[string]any)["transaction_id"] || events["request_key"] != key || events["invocation_id"] != invocation || !reflect.DeepEqual(events["response_read_ref"], map[string]any{"selector": responseRef["selector"], "digest": responseRef["digest"], "schema_version": "REFERENCES_RESPONSE_READ_V1"}) || !reflect.DeepEqual(events["raw_result_ref"], map[string]any{"selector": resultRef["selector"], "digest": resultRef["digest"], "schema_version": "REFERENCES_RAW_RESULT_V1"}) {
		return fmt.Errorf("historical event transaction/response mismatch")
	}
	begun, ended := map[int]bool{}, map[int]bool{}
	eB, eT := 0, 0
	if len(sequence) < 2 {
		return fmt.Errorf("historical event sequence length")
	}
	for index, value := range sequence {
		event := value.(map[string]any)
		if event["sequence"] != float64(index) {
			return fmt.Errorf("historical event sequence mismatch")
		}
		switch event["kind"] {
		case "QUERY_BEGIN":
			if index != 0 || event["ordinal"] != nil || event["terminal_disposition"] != "NONE" {
				return fmt.Errorf("historical query begin mismatch")
			}
		case "ELEMENT_BEGIN":
			ordinal := int(event["ordinal"].(float64))
			if index == 0 || begun[ordinal] || ended[ordinal] || ordinal != eB || event["terminal_disposition"] != "NONE" {
				return fmt.Errorf("historical element begin ordinal mismatch")
			}
			begun[ordinal] = true
			eB++
		case "ELEMENT_TERMINAL":
			ordinal := int(event["ordinal"].(float64))
			if !begun[ordinal] || ended[ordinal] || event["terminal_disposition"] != "VALID_PENDING_ADMISSION" {
				return fmt.Errorf("historical element terminal ordinal mismatch")
			}
			ended[ordinal] = true
			eT++
		case "QUERY_TERMINAL":
			if index != len(sequence)-1 || event["ordinal"] != nil || event["terminal_disposition"] != "ITEMS" {
				return fmt.Errorf("historical query terminal mismatch")
			}
		default:
			return fmt.Errorf("historical event kind mismatch")
		}
	}
	if sequence[len(sequence)-1].(map[string]any)["kind"] != "QUERY_TERMINAL" || eB != eT {
		return fmt.Errorf("historical event completion mismatch")
	}
	raw, err := canonicalProposal(x.e.Stored[resultRef["selector"].(string)])
	if err != nil {
		return err
	}
	payload := append([]byte(nil), x.e.Stored[raw["payload_selector"].(string)]...)
	if canonicalDigest(payload) != raw["payload_digest"] || canonicalDigest(payload) != proposal["raw_result_digest"] || len(payload) != int(raw["payload_byte_length"].(float64)) || len(payload) != int(proposal["raw_result_byte_length"].(float64)) || !bytes.Equal(payload, x.selectedRawPayload) {
		return fmt.Errorf("raw payload original mismatch")
	}
	// Neither receipt digest is authority to substitute a different result.
	// Compare the selected READ's exact JSON result span to the separately
	// selected retained raw payload, before any whole-result parsing or counts.
	var observedRead struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(x.observedFrames[1], &observedRead) != nil || !bytes.Equal(observedRead.Result, payload) || response["raw_result_digest"] != canonicalDigest(payload) || response["raw_result_byte_length"] != float64(len(payload)) {
		return fmt.Errorf("READ result/raw payload original-byte mismatch")
	}
	if err := successfulOneLocationReadEnvelope(x.observedFrames[1], uint64(owner["wire_id"].(float64))); err != nil {
		return err
	}
	var locations []struct {
		URI   string         `json:"uri"`
		Range map[string]any `json:"range"`
	}
	if err := json.Unmarshal(payload, &locations); err != nil || len(locations) != 1 {
		return fmt.Errorf("one-Location parse: %v", err)
	}
	occurrences := candidate["occurrences"].([]any)
	e := len(locations)
	p := len(occurrences)
	if eB != e || eT != eB || candidate["proposed_p"] != float64(p) || proposal["whole_result_p"] != float64(p) || proposal["known_e"] != float64(e) || proposal["observed_b"] != float64(1) || proposal["observed_e_b"] != float64(eB) || proposal["observed_e_t"] != float64(eT) || scanner["e"] != float64(e) || scanner["e_b"] != float64(eB) || scanner["e_t"] != float64(eT) {
		return fmt.Errorf("historical event/scanner/parsed counts mismatch")
	}
	parsedIDs := make([]string, 0, p)
	for ordinal, occValue := range occurrences {
		if ordinal >= e || !begun[ordinal] || !ended[ordinal] {
			return fmt.Errorf("historical occurrence without element terminal")
		}
		occ := occValue.(map[string]any)
		id := oneLocationID(candidate["context"].(map[string]any), locations[ordinal].URI, locations[ordinal].Range)
		if occ["occurrence_id"] != id || occ["ordinal"] != float64(ordinal) || occ["returned_uri"] != locations[ordinal].URI || !reflect.DeepEqual(occ["returned_range"], locations[ordinal].Range) {
			return fmt.Errorf("historical occurrence identity mismatch")
		}
		parsedIDs = append(parsedIDs, id)
	}
	terminal, ledger, accounting := records["production_terminal_ledger"], records["production_occurrence_ledger"], records["production_accounting_record"]
	if !refEquals(x.inventory["attempt_manifest_ref"], "attempt_evidence_manifest") {
		return fmt.Errorf("inventory attempt manifest mismatch")
	}
	for index, role := range []string{"production_terminal_ledger", "production_occurrence_ledger", "production_accounting_record"} {
		if !refEquals(x.inventory["ancillary_refs"].([]any)[index], role) {
			return fmt.Errorf("inventory %s substitution", role)
		}
		if !refEquals(records[role]["attempt_manifest_ref"], "attempt_evidence_manifest") || !reflect.DeepEqual(records[role]["historical_predecessor_refs"], []any{candidateRef}) {
			return fmt.Errorf("%s predecessor closure mismatch", role)
		}
	}
	for _, role := range []string{"production_terminal_ledger", "production_occurrence_ledger", "production_accounting_record"} {
		record := records[role]
		if !reflect.DeepEqual(record["policy_digests"], x.d["policy_digests"]) {
			return fmt.Errorf("%s policy mismatch", role)
		}
		if role == "production_occurrence_ledger" {
			if record["target_digest"] != x.e.ExpectedTargetDigest || record["result_digest"] != x.e.ExpectedResultDigest {
				return fmt.Errorf("occurrence ledger target/result mismatch")
			}
		} else {
			if !reflect.DeepEqual(record["target"], map[string]any{"status": "PRESENT", "digest": x.e.ExpectedTargetDigest}) || !reflect.DeepEqual(record["result"], map[string]any{"status": "PRESENT", "digest": x.e.ExpectedResultDigest}) {
				return fmt.Errorf("%s target/result mismatch", role)
			}
		}
	}
	if !refEquals(accounting["terminal_ledger_ref"], "production_terminal_ledger") || !refEquals(accounting["occurrence_ledger_ref"], "production_occurrence_ledger") {
		return fmt.Errorf("accounting terminal/occurrence ref mismatch")
	}
	initiation := records["attempt_initiation"]
	begin := records["attempt_query_begin"]
	journal := records["attempt_terminal_journal"]
	n := int(initiation["declared_n"].(float64))
	b, terminalCount := 0, 0
	if manifest["begin"].(map[string]any)["status"] == "PRESENT" && begin["evaluation_began"] == true && begin["query_ordinal"] == float64(0) {
		b++
	}
	if manifest["terminal"].(map[string]any)["status"] == "PRESENT" && journal["query_ordinal"] == begin["query_ordinal"] && refEquals(journal["begin_ref"], "attempt_query_begin") {
		terminalCount++
	}
	if n != 1 || b != 1 || terminalCount != 1 || terminalCount > b || b > n || eT > eB || eB > e || p > e || journal["method_outcome"] != proposal["proposed_outcome"] || journal["member_disposition"] != proposal["proposed_disposition"] || terminal["b"] != float64(b) || terminal["t"] != float64(terminalCount) || terminal["outcome"] != journal["method_outcome"] || terminal["disposition"] != journal["member_disposition"] {
		return fmt.Errorf("terminal and journal accounting mismatch")
	}
	parsed := ledger["parsed_occurrences"].([]any)
	if ledger["p"] != float64(p) || len(parsed) != p {
		return fmt.Errorf("occurrence ledger parsed count mismatch")
	}
	for index, entry := range parsed {
		if entry.(map[string]any)["identity"] != parsedIDs[index] || !reflect.DeepEqual(entry.(map[string]any)["evidence_refs"], []any{candidateRef}) {
			return fmt.Errorf("occurrence ledger parsed identity/evidence mismatch")
		}
	}
	admitted := ledger["admitted_occurrences"].([]any)
	a := len(admitted)
	seenAdmitted := map[string]bool{}
	parsedSet := map[string]bool{}
	for _, id := range parsedIDs {
		parsedSet[id] = true
	}
	for _, value := range admitted {
		id, ok := value.(string)
		if !ok || !parsedSet[id] || seenAdmitted[id] {
			return fmt.Errorf("occurrence ledger admitted subset mismatch")
		}
		seenAdmitted[id] = true
	}
	if a == 0 || a > p {
		return fmt.Errorf("final admission count mismatch")
	}
	for name, count := range map[string]int{"n": n, "b": b, "t": terminalCount, "e": e, "e_b": eB, "e_t": eT, "p": p, "a": a} {
		if accounting[name] != float64(count) {
			return fmt.Errorf("accounting %s mismatch", name)
		}
	}
	ctx := x.final["context"].(map[string]any)
	if fmt.Sprint(ctx["p"]) != fmt.Sprint(p) || fmt.Sprint(ctx["a"]) != fmt.Sprint(a) || ctx["method_outcome"] != journal["method_outcome"] || ctx["member_disposition"] != journal["member_disposition"] || accounting["outcome"] != journal["method_outcome"] || accounting["disposition"] != journal["member_disposition"] || ctx["accounting_digest"] != canonicalDigest(x.originals["production_accounting_record"]) {
		return fmt.Errorf("accounting digest/outcome/count mismatch")
	}
	return nil
}

// Rehash every claimant descendant of begin, including the accounting original
// and final identities, while leaving the host-frozen key and matched wire pair
// untouched. This is deliberately stronger than an early immutable-ref negative.
func substituteOneLocationBeginKey(t *testing.T, x *oneLocationReplay) {
	t.Helper()
	rewrite := func(role string, change func(map[string]any)) map[string]any {
		original, err := canonicalProposal(x.originals[role])
		if err != nil {
			t.Fatal(err)
		}
		change(original)
		body, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		x.originals[role] = body
		x.selected[role]["digest"], x.selected[role]["byte_length"] = canonicalDigest(body), len(body)
		return cloneProposal(x.selected[role])
	}
	begin := rewrite("attempt_query_begin", func(r map[string]any) { r["response_key"] = "unmatched-response" })
	terminal := rewrite("attempt_terminal_journal", func(r map[string]any) { r["begin_ref"] = begin })
	scanner := rewrite("attempt_scanner_knowledge", func(r map[string]any) { r["begin_ref"] = begin })
	manifest := rewrite("attempt_evidence_manifest", func(r map[string]any) {
		r["begin"], r["terminal"], r["scanner"] = map[string]any{"status": "PRESENT", "ref": begin}, map[string]any{"status": "PRESENT", "ref": terminal}, map[string]any{"status": "PRESENT", "ref": scanner}
	})
	terminalLedger := rewrite("production_terminal_ledger", func(r map[string]any) { r["attempt_manifest_ref"] = manifest })
	occurrenceLedger := rewrite("production_occurrence_ledger", func(r map[string]any) { r["attempt_manifest_ref"] = manifest })
	accounting := rewrite("production_accounting_record", func(r map[string]any) {
		r["attempt_manifest_ref"], r["terminal_ledger_ref"], r["occurrence_ledger_ref"] = manifest, terminalLedger, occurrenceLedger
	})
	x.inventory["attempt_manifest_ref"] = manifest
	x.inventory["ancillary_refs"] = []any{terminalLedger, occurrenceLedger, accounting}
	x.final["context"].(map[string]any)["accounting_digest"] = accounting["digest"]
	x.e.ExpectedAccountingDigest = accounting["digest"].(string)
	repinOneLocationInventory(x)
}

func TestADR0011OneLocationClaimantRehashedPostWriteKey(t *testing.T) {
	x := newOneLocationReplay(t)
	if err := replayOneLocation(t, x); err != nil {
		t.Fatalf("positive control: %v", err)
	}
	rewrite := func(role string, change func(map[string]any)) map[string]any {
		r, err := canonicalProposal(x.originals[role])
		if err != nil {
			t.Fatal(err)
		}
		change(r)
		body, _ := json.Marshal(r)
		x.originals[role] = body
		x.selected[role]["digest"], x.selected[role]["byte_length"] = canonicalDigest(body), len(body)
		return cloneProposal(x.selected[role])
	}
	fakeKey := "lsp-trace.request-key.v1:g=1;id=2"
	observation := rewrite("attempt_request_key_observation", func(r map[string]any) { r["request_key"], r["wire_id"] = fakeKey, 2 })
	begin := rewrite("attempt_query_begin", func(r map[string]any) {
		r["request_key"], r["response_key"], r["key_observation_ref"] = fakeKey, fakeKey, observation
	})
	terminal := rewrite("attempt_terminal_journal", func(r map[string]any) { r["request_key"], r["begin_ref"] = fakeKey, begin })
	scanner := rewrite("attempt_scanner_knowledge", func(r map[string]any) { r["request_key"], r["begin_ref"] = fakeKey, begin })
	manifest := rewrite("attempt_evidence_manifest", func(r map[string]any) {
		r["request_key"] = fakeKey
		r["key_observation"], r["begin"], r["terminal"], r["scanner"] = map[string]any{"status": "PRESENT", "ref": observation}, map[string]any{"status": "PRESENT", "ref": begin}, map[string]any{"status": "PRESENT", "ref": terminal}, map[string]any{"status": "PRESENT", "ref": scanner}
	})
	terminalLedger := rewrite("production_terminal_ledger", func(r map[string]any) { r["request_key"], r["attempt_manifest_ref"] = fakeKey, manifest })
	occurrenceLedger := rewrite("production_occurrence_ledger", func(r map[string]any) { r["request_key"], r["attempt_manifest_ref"] = fakeKey, manifest })
	accounting := rewrite("production_accounting_record", func(r map[string]any) {
		r["request_key"], r["attempt_manifest_ref"], r["terminal_ledger_ref"], r["occurrence_ledger_ref"] = fakeKey, manifest, terminalLedger, occurrenceLedger
	})
	x.inventory["attempt_manifest_ref"] = manifest
	x.inventory["ancillary_refs"] = []any{terminalLedger, occurrenceLedger, accounting}
	x.final["context"].(map[string]any)["request_key"] = fakeKey
	x.final["context"].(map[string]any)["accounting_digest"] = accounting["digest"]
	// The claimant can make its entire final/ledger chain self-consistent; the
	// independent owner expectation remains the actual completed WRITE's key.
	x.e.ExpectedRequestKey = fakeKey
	x.e.ExpectedAccountingDigest = accounting["digest"].(string)
	repinOneLocationInventory(&x)
	for _, role := range []string{"attempt_request_key_observation", "attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge", "attempt_evidence_manifest", "production_terminal_ledger", "production_occurrence_ledger", "production_accounting_record"} {
		r, _ := canonicalProposal(x.originals[role])
		definition := strings.TrimPrefix(x.selected[role]["schema_version"].(string), admissionID+"#/$defs/")
		if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, definition).Validate(r); err != nil {
			t.Fatalf("forged %s fails earlier schema: %v", role, err)
		}
	}
	if err := replayAdmission(x.d, x.inventory, x.final, x.e); err != nil {
		t.Fatalf("forged claimant fails earlier 21-ref/final closure: %v", err)
	}
	if err := replayOneLocation(t, x); err == nil || !strings.Contains(err.Error(), "observed WRITE/frame differs") {
		t.Fatalf("owner expected key did not reject rehashed claimant: %v", err)
	}
}

func TestADR0011OneLocationMatchedResponseSubstitution(t *testing.T) {
	x := newOneLocationReplay(t)
	if err := replayOneLocation(t, x); err != nil {
		t.Fatalf("positive control: %v", err)
	}
	substituteOneLocationBeginKey(t, &x)
	for _, role := range []string{"attempt_query_begin", "attempt_terminal_journal", "attempt_scanner_knowledge", "attempt_evidence_manifest", "production_terminal_ledger", "production_occurrence_ledger", "production_accounting_record"} {
		record, err := canonicalProposal(x.originals[role])
		if err != nil {
			t.Fatal(err)
		}
		definition := strings.TrimPrefix(x.selected[role]["schema_version"].(string), admissionID+"#/$defs/")
		if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, definition).Validate(record); err != nil {
			t.Fatalf("schema-valid %s: %v", role, err)
		}
	}
	for role, record := range map[string]map[string]any{"productionDependencyInventory": x.inventory, "productionAdmissionFinal": x.final} {
		if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, role).Validate(record); err != nil {
			t.Fatalf("schema-valid %s: %v", role, err)
		}
	}
	// The earlier 21-ref/original-byte/final-hash replay must pass. Only the
	// independently observed write/read key disagrees with begin.response_key.
	if err := replayAdmission(x.d, x.inventory, x.final, x.e); err != nil {
		t.Fatalf("earlier historical/final guard: %v", err)
	}
	if err := replayOneLocation(t, x); err == nil || !strings.Contains(err.Error(), "cross-record matched-response key mismatch") {
		t.Fatalf("ASSERT_MATCHED_RESPONSE_KEY: %v", err)
	}
}

func TestADR0011OneLocationWriteEnvelopeVersion(t *testing.T) {
	write, _, _ := oneLocationWire()
	altered := bytes.Replace(write, []byte(`"jsonrpc":"2.0"`), []byte(`"jsonrpc":"1.0"`), 1)
	if bytes.Equal(altered, write) {
		t.Fatal("WRITE version not changed")
	}
	x := newOneLocationReplayWire(t, oneLocationPayload, false, altered)
	if err := replayAdmission(x.d, x.inventory, x.final, x.e); err != nil {
		t.Fatalf("earlier historical/final closure: %v", err)
	}
	ownerRef := selectedAdmissionRef(x.e.Refs, "owner_read_references")
	owner, _ := canonicalProposal(x.e.Stored[ownerRef["selector"].(string)])
	responseRef := selectedAdmissionRef(x.e.Refs, "response_read")
	response, _ := canonicalProposal(x.e.Stored[responseRef["selector"].(string)])
	if err := validateAdmissionOriginal("owner_read_references", owner); err != nil {
		t.Fatalf("owner schema: %v", err)
	}
	if err := validateAdmissionOriginal("response_read", response); err != nil {
		t.Fatalf("response schema: %v", err)
	}
	if err := successfulOneLocationReadEnvelope(x.observedFrames[1], 1); err != nil {
		t.Fatalf("READ guard: %v", err)
	}
	if err := replayOneLocation(t, x); err == nil || !strings.Contains(err.Error(), "WRITE request envelope not valid") {
		t.Fatalf("ASSERT_WRITE_REQUEST_ENVELOPE: %v", err)
	}
}

func TestADR0011OneLocationWriteNestedParams(t *testing.T) {
	write, _, _ := oneLocationWire()
	altered := bytes.Replace(write, []byte(`"includeDeclaration":false}`), []byte(`"includeDeclaration":false,"unexpected":true}`), 1)
	if bytes.Equal(write, altered) {
		t.Fatal("nested WRITE params not changed")
	}
	x := newOneLocationReplayWire(t, oneLocationPayload, false, altered)
	if err := replayAdmission(x.d, x.inventory, x.final, x.e); err != nil {
		t.Fatalf("earlier historical/final closure: %v", err)
	}
	for _, role := range []string{"owner_read_references", "response_read"} {
		ref := selectedAdmissionRef(x.e.Refs, role)
		record, err := canonicalProposal(x.e.Stored[ref["selector"].(string)])
		if err != nil {
			t.Fatal(err)
		}
		if err := validateAdmissionOriginal(role, record); err != nil {
			t.Fatalf("%s historical shape: %v", role, err)
		}
	}
	if err := successfulOneLocationReadEnvelope(x.observedFrames[1], 1); err != nil {
		t.Fatalf("unchanged READ guard: %v", err)
	}
	if err := replayOneLocation(t, x); err == nil || !strings.Contains(err.Error(), "WRITE nested params not valid") {
		t.Fatalf("ASSERT_WRITE_NESTED_PARAMS: %v", err)
	}
}

func TestADR0011OneLocationWriteNestedParamsStrictMembers(t *testing.T) {
	write, _, params := oneLocationWire()
	if _, _, _, err := exactOneLocationParams(params); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ old, replacement string }{
		{`"includeDeclaration":false`, `"includeDeclaration":false,"unexpected":true`},
		{`"uri":"file:///tmp/a.go"`, `"uri":"file:///tmp/a.go","extra":true`},
		{`"character":0`, `"character":0,"extra":true`},
		{`"position":`, `"extra":true,"position":`},
		{`"line":0,`, `"line":0,"line":0,`},
		{`"includeDeclaration":false`, `"includeDeclaration":false,"includeDeclaration":false`},
		{`"uri":"file:///tmp/a.go"`, `"uri":1`},
		{`"line":0`, `"line":-1`},
		{`"character":0`, `"character":0.0`},
		{`"includeDeclaration":false`, `"includeDeclaration":0`},
		{`"includeDeclaration":false`, `"other":false`},
		{`"position":{"line":0,"character":0}`, `"position":null`},
	} {
		t.Run(tc.replacement, func(t *testing.T) {
			altered := bytes.Replace(write, []byte(tc.old), []byte(tc.replacement), 1)
			if bytes.Equal(write, altered) {
				t.Fatal("fixture mutation missed")
			}
			if err := validOneLocationWriteEnvelope(altered); err != nil {
				if strings.Contains(tc.replacement, `"line":0,"line":0`) || strings.Contains(tc.replacement, `"includeDeclaration":false,"includeDeclaration":false`) {
					return // existing whole-frame duplicate-name guard
				}
				t.Fatalf("top-level envelope: %v", err)
			}
			var request struct {
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(altered, &request); err != nil {
				t.Fatal(err)
			}
			if err := uniqueJSON(json.NewDecoder(bytes.NewReader(altered))); err == nil {
				if _, _, _, err := exactOneLocationParams(request.Params); err == nil {
					t.Fatal("accepted invalid nested params")
				}
			}
		})
	}
	for _, body := range [][]byte{append(append([]byte(nil), params...), []byte(` true`)...), []byte(`null`)} {
		if _, _, _, err := exactOneLocationParams(body); err == nil {
			t.Fatalf("accepted invalid params: %s", body)
		}
	}
}

func TestADR0011OneLocationWriteEnvelopeStrictMembers(t *testing.T) {
	write, _, _ := oneLocationWire()
	if err := validOneLocationWriteEnvelope(write); err != nil {
		t.Fatal(err)
	}
	for _, extra := range []string{`,"unknown":true`, `,"jsonrpc":"2.0"`, `,"method":"textDocument/references"`, `,"params":{}`} {
		altered := append(append([]byte(nil), write[:len(write)-1]...), []byte(extra+`}`)...)
		if err := validOneLocationWriteEnvelope(altered); err == nil {
			t.Fatalf("accepted unknown/duplicate member: %s", extra)
		}
	}
	for _, altered := range [][]byte{
		bytes.Replace(write, []byte(`"jsonrpc":"2.0",`), nil, 1),
		bytes.Replace(write, []byte(`"params":`), []byte(`"missing":`), 1),
		bytes.Replace(write, []byte(`"id":1`), []byte(`"id":"1"`), 1),
	} {
		if err := validOneLocationWriteEnvelope(altered); err == nil {
			t.Fatalf("accepted missing/string member: %s", altered)
		}
	}
}

func TestADR0011OneLocationReadEnvelopeStrictMembers(t *testing.T) {
	_, read, _ := oneLocationWire()
	if err := successfulOneLocationReadEnvelope(read, 1); err != nil {
		t.Fatal(err)
	}
	for _, extra := range []string{`,"error":null`, `,"unknown":true`, `,"result":[]`, `,"id":1`} {
		altered := append(append([]byte(nil), read[:len(read)-1]...), []byte(extra+`}`)...)
		if err := successfulOneLocationReadEnvelope(altered, 1); err == nil {
			t.Fatalf("accepted unknown/duplicate member: %s", extra)
		}
	}
	if err := successfulOneLocationReadEnvelope([]byte(`{"jsonrpc":"2.0","id":"1","result":[]}`), 1); err == nil {
		t.Fatal("accepted string ID")
	}
}

func TestADR0011OneLocationReadErrorEnvelope(t *testing.T) {
	x := newOneLocationReplayWire(t, oneLocationPayload, true)
	for _, role := range []string{"owner_read_references", "response_read", "raw_result", "proposal", "candidate"} {
		ref := selectedAdmissionRef(x.e.Refs, role)
		original, err := canonicalProposal(x.e.Stored[ref["selector"].(string)])
		if err != nil {
			t.Fatal(err)
		}
		if err := validateAdmissionOriginal(role, original); err != nil {
			t.Fatalf("schema-valid %s: %v", role, err)
		}
	}
	if err := replayAdmission(x.d, x.inventory, x.final, x.e); err != nil {
		t.Fatalf("earlier historical/final closure: %v", err)
	}
	ownerRef := selectedAdmissionRef(x.e.Refs, "owner_read_references")
	owner, _ := canonicalProposal(x.e.Stored[ownerRef["selector"].(string)])
	responseRef := selectedAdmissionRef(x.e.Refs, "response_read")
	response, _ := canonicalProposal(x.e.Stored[responseRef["selector"].(string)])
	begin, _ := canonicalProposal(x.originals["attempt_query_begin"])
	if err := matchedOneLocationResponse(x, owner, response, begin); err != nil {
		t.Fatalf("earlier matched-key guard: %v", err)
	}
	var read struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(x.observedFrames[1], &read); err != nil || !bytes.Equal(read.Result, x.selectedRawPayload) {
		t.Fatalf("earlier READ/raw pairing: %v", err)
	}
	if err := replayOneLocation(t, x); err == nil || !strings.Contains(err.Error(), "READ response envelope not successful") {
		t.Fatalf("ASSERT_READ_SUCCESS_ENVELOPE: %v", err)
	}
}

func TestADR0011OneLocationReadResultRawPayloadPairing(t *testing.T) {
	x := newOneLocationReplayRead(t, []byte("[]"))
	if bytes.Equal(x.observedFrames[1], x.selectedRawPayload) {
		t.Fatal("READ frame is not raw result")
	}
	for _, role := range []string{"owner_read_references", "response_read", "raw_result", "proposal", "candidate"} {
		ref := selectedAdmissionRef(x.e.Refs, role)
		original, err := canonicalProposal(x.e.Stored[ref["selector"].(string)])
		if err != nil {
			t.Fatal(err)
		}
		if err := validateAdmissionOriginal(role, original); err != nil {
			t.Fatalf("rehashed historical %s: %v", role, err)
		}
	}
	if err := replayAdmission(x.d, x.inventory, x.final, x.e); err != nil {
		t.Fatalf("earlier historical/final closure: %v", err)
	}
	ownerRef := selectedAdmissionRef(x.e.Refs, "owner_read_references")
	owner, _ := canonicalProposal(x.e.Stored[ownerRef["selector"].(string)])
	responseRef := selectedAdmissionRef(x.e.Refs, "response_read")
	response, _ := canonicalProposal(x.e.Stored[responseRef["selector"].(string)])
	begin, _ := canonicalProposal(x.originals["attempt_query_begin"])
	if err := matchedOneLocationResponse(x, owner, response, begin); err != nil {
		t.Fatalf("earlier matched-key guard: %v", err)
	}
	if err := replayOneLocation(t, x); err == nil || !strings.Contains(err.Error(), "READ result/raw payload original-byte mismatch") {
		t.Fatalf("ASSERT_READ_RAW_BYTE_PAIRING: %v", err)
	}
}

func TestADR0011OneLocationDerivedElementAccounting(t *testing.T) {
	x := newOneLocationReplay(t)
	if err := replayOneLocation(t, x); err != nil {
		t.Fatalf("positive control: %v", err)
	}
	accounting, err := canonicalProposal(x.originals["production_accounting_record"])
	if err != nil {
		t.Fatal(err)
	}
	accounting["e_b"] = 0 // schema-valid; selected event still has one element begin
	body, _ := json.Marshal(accounting)
	x.originals["production_accounting_record"] = body
	x.selected["production_accounting_record"]["digest"], x.selected["production_accounting_record"]["byte_length"] = canonicalDigest(body), len(body)
	x.inventory["ancillary_refs"].([]any)[2] = cloneProposal(x.selected["production_accounting_record"])
	x.final["context"].(map[string]any)["accounting_digest"] = canonicalDigest(body)
	x.e.ExpectedAccountingDigest = canonicalDigest(body)
	repinOneLocationInventory(&x)
	if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionAccountingRecord").Validate(accounting); err != nil {
		t.Fatalf("schema-valid accounting: %v", err)
	}
	if err := replayAdmission(x.d, x.inventory, x.final, x.e); err != nil {
		t.Fatalf("earlier replay: %v", err)
	}
	if err := replayOneLocation(t, x); err == nil || !strings.Contains(err.Error(), "accounting e_b mismatch") {
		t.Fatalf("ASSERT_DERIVED_ELEMENT_BEGIN: %v", err)
	}
}

func TestADR0011OneLocationReplayNegatives(t *testing.T) {
	for _, tc := range []struct {
		name, guard string
		mutate      func(*oneLocationReplay)
	}{
		{"terminal ledger ref substitution", "inventory production_terminal_ledger substitution", func(x *oneLocationReplay) {
			ref := x.inventory["ancillary_refs"].([]any)[0].(map[string]any)
			ref["digest"] = digestB
			repinOneLocationInventory(x)
		}},
		{"mutated ledger original", "original production_occurrence_ledger", func(x *oneLocationReplay) {
			record, _ := canonicalProposal(x.originals["production_occurrence_ledger"])
			record["p"] = 0 // schema-valid but contradicts retained occurrence
			x.originals["production_occurrence_ledger"], _ = json.Marshal(record)
		}},
		{"altered accounting digest", "final context accounting_digest differs", func(x *oneLocationReplay) {
			x.final["context"].(map[string]any)["accounting_digest"] = digestB
			repinOneLocationFinal(x)
		}},
		{"mismatched request key", "final context request_key differs", func(x *oneLocationReplay) {
			x.final["context"].(map[string]any)["request_key"] = "other-key"
			repinOneLocationFinal(x)
		}},
		{"mismatched invocation", "final context invocation_id differs", func(x *oneLocationReplay) {
			x.final["context"].(map[string]any)["invocation_id"] = digestB
			repinOneLocationFinal(x)
		}},
		{"absent scanner", "original attempt_evidence_manifest", func(x *oneLocationReplay) {
			record, _ := canonicalProposal(x.originals["attempt_evidence_manifest"])
			record["scanner"] = map[string]any{"status": "ABSENT_NOT_OBSERVED"}
			x.originals["attempt_evidence_manifest"], _ = json.Marshal(record)
		}},
		{"UNKNOWN scanner", "original attempt_scanner_knowledge", func(x *oneLocationReplay) {
			record, _ := canonicalProposal(x.originals["attempt_scanner_knowledge"])
			record["e"], record["e_b"], record["e_t"], record["top_level_state"] = "UNKNOWN", 0, 0, "UNKNOWN"
			x.originals["attempt_scanner_knowledge"], _ = json.Marshal(record)
		}},
		{"unverified final", "unverified final readback", func(x *oneLocationReplay) { x.verified = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newOneLocationReplay(t)
			if err := replayOneLocation(t, x); err != nil {
				t.Fatalf("positive control: %v", err)
			}
			tc.mutate(&x)
			for _, role := range []string{"production_occurrence_ledger", "attempt_evidence_manifest", "attempt_scanner_knowledge"} {
				original, err := canonicalProposal(x.originals[role])
				if err != nil {
					t.Fatal(err)
				}
				definition := strings.TrimPrefix(x.selected[role]["schema_version"].(string), admissionID+"#/$defs/")
				if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, definition).Validate(original); err != nil {
					t.Fatalf("mutated %s schema: %v", role, err)
				}
			}
			if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionAdmissionFinal").Validate(x.final); err != nil {
				t.Fatalf("rehash final shape: %v", err)
			}
			if err := proposalRole(t, "adr0011-production-admission-v1.proposed.schema.json", admissionID, "productionDependencyInventory").Validate(x.inventory); err != nil {
				t.Fatalf("inventory shape: %v", err)
			}
			if err := replayOneLocation(t, x); err == nil || !strings.Contains(err.Error(), tc.guard) {
				t.Fatalf("expected %q, got %v", tc.guard, err)
			}
		})
	}
}

func repinOneLocationInventory(x *oneLocationReplay) {
	x.inventory["inventory_id"] = roleDigest("ADR0011_PRODUCTION_DEPENDENCY_INVENTORY_V1", x.inventory, "inventory_id")
	ctx := x.final["context"].(map[string]any)
	ctx["inventory_id"] = x.inventory["inventory_id"]
	body, _ := json.Marshal(x.inventory)
	ctx["inventory_digest"], ctx["inventory_byte_length"] = canonicalDigest(body), len(body)
	repinOneLocationFinal(x)
}

func TestADR0011OneLocationHistoricalBaseline(t *testing.T) {
	d, _, _, e := admissionModel(true)
	candidateRef := selectedAdmissionRef(e.Refs, "candidate")
	candidate, err := canonicalProposal(e.Stored[candidateRef["selector"].(string)])
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAdmissionOriginal("candidate", candidate); err != nil {
		t.Fatal(err)
	}
	occ := candidate["occurrences"].([]any)
	if candidate["proposed_p"] != float64(1) || len(occ) != 1 {
		t.Fatal("ASSERT_ONE_LOCATION_CANDIDATE")
	}
	if d["occurrence_id"] != e.Plan["occurrence_id"] {
		t.Fatal("ASSERT_PREINVOKE_DECLARATION")
	}
	x := newOneLocationReplay(t)
	if err := replayOneLocation(t, x); err != nil {
		t.Fatalf("ASSERT_ONE_LOCATION_REPLAY: %v", err)
	}
}

func selectedAdmissionRef(refs []any, role string) map[string]any {
	for _, item := range refs {
		r := item.(map[string]any)
		if r["role"] == role {
			return r
		}
	}
	panic("missing selected role " + role)
}

// Lifecycle oracle inputs model the lock-held independently observed epoch,
// intent and bytes, rather than trusting the claimant terminal fields.
type lifecycleExpectation struct {
	Epoch        int
	Intent       map[string]any
	Dependents   []any
	Children     []map[string]any
	RootVerified bool
	BytesPresent map[string]bool
	Attempt      map[string]any
	IntentRef    map[string]any
	ChildBytes   map[string][]byte
	TargetBytes  map[string][]byte
	AttemptBytes map[string][]byte
	NoUnlink     bool
}

func replayLifecycle(root, terminal, reconciliation map[string]any, e lifecycleExpectation) error {
	fail := func() error { return fmt.Errorf("lifecycle replay rejected") }
	intent := e.Intent
	for _, x := range []map[string]any{root, terminal, reconciliation} {
		if x["epoch"] != e.Epoch || x["decision_id"] != intent["decision_id"] || !reflect.DeepEqual(x["root_identity"], intent["root_identity"]) || x["previous_epoch_digest"] != intent["previous_epoch_digest"] || !reflect.DeepEqual(x["intent_ref"], e.IntentRef) {
			return fail()
		}
	}
	if !e.RootVerified || len(e.Children) != len(e.Dependents) || root["dependent_count"] != len(e.Dependents) {
		return fail()
	}
	entries := root["children"].([]any)
	if len(entries) != len(e.Dependents) {
		return fail()
	}
	for n, dep := range e.Dependents {
		entry := entries[n].(map[string]any)
		if !reflect.DeepEqual(entry["dependent_ref"], dep) || !reflect.DeepEqual(e.Children[n]["dependent_ref"], dep) || !reflect.DeepEqual(e.Children[n]["intent_ref"], e.IntentRef) || e.Children[n]["epoch"] != e.Epoch || e.Children[n]["decision_id"] != intent["decision_id"] || !reflect.DeepEqual(e.Children[n]["root_identity"], intent["root_identity"]) || e.Children[n]["previous_epoch_digest"] != intent["previous_epoch_digest"] {
			return fail()
		}
	}
	if len(e.ChildBytes) != len(entries) {
		return fail()
	}
	for n, dep := range e.Dependents {
		entry := entries[n].(map[string]any)
		childRef, ok := entry["child_ref"].(map[string]any)
		if !ok {
			return fail()
		}
		selector, ok := childRef["selector"].(string)
		if !ok {
			return fail()
		}
		body, ok := e.ChildBytes[selector]
		if !ok {
			return fail()
		}
		decoded, err := canonicalProposal(body)
		if err != nil || !reflect.DeepEqual(decoded, cloneProposal(e.Children[n])) {
			return fail()
		}
		actualRef, _ := proposalOriginalRef("child", decoded)
		actualRef["selector"] = selector
		if !reflect.DeepEqual(childRef, actualRef) || !reflect.DeepEqual(decoded["dependent_ref"], cloneProposal(dep.(map[string]any))) || !reflect.DeepEqual(decoded["intent_ref"], cloneProposal(e.IntentRef)) || decoded["epoch"] != float64(e.Epoch) {
			return fail()
		}
	}
	{
		attempts, ok := terminal["attempts"].([]any)
		if !ok || len(attempts) != len(e.Dependents) || len(e.AttemptBytes) != len(attempts) {
			return fail()
		}
		for n, dep := range e.Dependents {
			ref, ok := attempts[n].(map[string]any)
			if !ok {
				return fail()
			}
			selector, ok := ref["selector"].(string)
			if !ok {
				return fail()
			}
			body, ok := e.AttemptBytes[selector]
			if !ok {
				return fail()
			}
			original, err := canonicalProposal(body)
			if err != nil {
				return fail()
			}
			actualRef, _ := proposalOriginalRef("attempt", original)
			actualRef["selector"] = selector
			if !reflect.DeepEqual(ref, actualRef) || !reflect.DeepEqual(original["target_ref"], cloneProposal(dep.(map[string]any))) || !reflect.DeepEqual(original["intent_ref"], cloneProposal(e.IntentRef)) || original["epoch"] != float64(e.Epoch) || original["decision_id"] != intent["decision_id"] || !reflect.DeepEqual(original["root_identity"], cloneProposal(intent["root_identity"].(map[string]any))) || original["previous_epoch_digest"] != intent["previous_epoch_digest"] || original["ordinal"] != float64(n) {
				return fail()
			}
			if terminal["outcome"] == "REMOVED_VERIFIED" && (original["pre_state"] != "MATCHED" || original["syscall_outcome"] != "UNLINKED" || original["directory_sync"] != "SUCCEEDED" || original["close_outcome"] != "SUCCEEDED" || original["post_state"] != "ABSENT") {
				return fail()
			}
			target := dep.(map[string]any)
			targetBody, ok := e.TargetBytes[target["selector"].(string)]
			if !ok {
				return fail()
			}
			canonicalTarget, err := canonicalProposal(targetBody)
			if err != nil || canonicalTarget["epoch"] != float64(e.Epoch) {
				return fail()
			}
			h := sha256.Sum256(targetBody)
			if target["digest"] != "sha256:"+hex.EncodeToString(h[:]) || target["byte_length"] != len(targetBody) {
				return fail()
			}
		}
		if terminal["outcome"] == "REMOVED_VERIFIED" && (e.Attempt["pre_state"] != "MATCHED" || e.Attempt["syscall_outcome"] != "UNLINKED" || e.Attempt["directory_sync"] != "SUCCEEDED" || e.Attempt["close_outcome"] != "SUCCEEDED" || e.Attempt["post_state"] != "ABSENT" || e.NoUnlink) {
			return fail()
		}
		if len(e.BytesPresent) != len(e.Dependents) {
			return fail()
		}
		for _, dep := range e.Dependents {
			present, known := e.BytesPresent[dep.(map[string]any)["selector"].(string)]
			if !known || (terminal["outcome"] == "REMOVED_VERIFIED" && present) {
				return fail()
			}
		}
	}
	observations, ok := reconciliation["observations"].([]any)
	if !ok || len(observations) != len(e.Dependents) {
		return fail()
	}
	for n, dep := range e.Dependents {
		observation, ok := observations[n].(map[string]any)
		if !ok || !reflect.DeepEqual(observation["expected_ref"], dep) {
			return fail()
		}
		present := e.BytesPresent[dep.(map[string]any)["selector"].(string)]
		status := observation["status"]
		if present && status != "MATCHED" {
			return fail()
		}
		if !present && ((terminal["outcome"] == "REMOVED_VERIFIED" && status != "ABSENT_WITH_TERMINAL") || (terminal["outcome"] != "REMOVED_VERIFIED" && status != "UNEXPLAINED_MISSING")) {
			return fail()
		}
	}
	if reconciliation["outcome"] == "ABORT_VERIFIED_NO_UNLINK" {
		if !e.NoUnlink || terminal["outcome"] == "REMOVED_VERIFIED" {
			return fail()
		}
		for _, body := range e.AttemptBytes {
			original, err := canonicalProposal(body)
			if err != nil || original["syscall_outcome"] == "UNLINKED" {
				return fail()
			}
		}
		for _, dep := range e.Dependents {
			if !e.BytesPresent[dep.(map[string]any)["selector"].(string)] {
				return fail()
			}
		}
	}
	if reconciliation["outcome"] == "TERMINAL_REPLAY_VERIFIED" && terminal["outcome"] != "REMOVED_VERIFIED" {
		return fail()
	}
	return nil
}
func proposalOriginalRef(role string, body map[string]any) (map[string]any, []byte) {
	b, _ := json.Marshal(body)
	h := sha256.Sum256(b)
	ref := lifecycleProposalRef(role)
	ref["digest"], ref["byte_length"] = "sha256:"+hex.EncodeToString(h[:]), len(b)
	return ref, b
}
func TestADR0011LifecycleProposalReplay(t *testing.T) {
	newCase := func() (map[string]any, map[string]any, map[string]any, lifecycleExpectation) {
		m := lifecycleFixtures()
		dep, depBytes := proposalOriginalRef("dependent", map[string]any{"role": "dependent", "epoch": 1})
		intent := m["fenceIntent"]
		intent["selected_dependencies"], intent["dependent_set"] = []any{dep}, []any{dep}
		intentRef, _ := proposalOriginalRef("intent", intent)
		for _, name := range []string{"tombstoneChild", "tombstoneRoot", "unlinkAttempt", "cleanupTerminal", "reconciliation"} {
			m[name]["intent_ref"] = intentRef
		}
		child := m["tombstoneChild"]
		child["dependent_ref"] = dep
		childRef, childBytes := proposalOriginalRef("child", child)
		m["tombstoneRoot"]["children"] = []any{map[string]any{"dependent_ref": dep, "child_ref": childRef}}
		attempt := m["unlinkAttempt"]
		attempt["target_ref"] = dep
		attemptRef, attemptBytes := proposalOriginalRef("attempt", attempt)
		m["cleanupTerminal"]["attempts"] = []any{attemptRef}
		m["reconciliation"]["observations"] = []any{map[string]any{"expected_ref": dep, "status": "ABSENT_WITH_TERMINAL"}}
		m["reconciliation"]["outcome"] = "TERMINAL_REPLAY_VERIFIED"
		return m["tombstoneRoot"], m["cleanupTerminal"], m["reconciliation"], lifecycleExpectation{Epoch: 1, Intent: intent, IntentRef: intentRef, Dependents: []any{dep}, Children: []map[string]any{child}, ChildBytes: map[string][]byte{"ref-child.json": childBytes}, TargetBytes: map[string][]byte{"ref-dependent.json": depBytes}, AttemptBytes: map[string][]byte{"ref-attempt.json": attemptBytes}, RootVerified: true, BytesPresent: map[string]bool{"ref-dependent.json": false}, Attempt: attempt}
	}
	// Build two independent originals, child records, and unlink attempts.
	twoCase := func() (map[string]any, map[string]any, map[string]any, lifecycleExpectation, []map[string]any) {
		m := lifecycleFixtures()
		var deps []any
		targets := map[string][]byte{}
		for n := 1; n <= 2; n++ {
			dep, body := proposalOriginalRef("dependent", map[string]any{"role": "dependent", "epoch": 1, "member": n})
			selector := fmt.Sprintf("ref-dependent-%d.json", n)
			dep["selector"], targets[selector] = selector, body
			deps = append(deps, dep)
		}
		intent := m["fenceIntent"]
		intent["selected_dependencies"], intent["dependent_set"] = deps, deps
		intentRef, _ := proposalOriginalRef("intent", intent)
		for _, name := range []string{"tombstoneRoot", "cleanupTerminal", "reconciliation"} {
			m[name]["intent_ref"] = intentRef
		}
		var entries, attemptRefs []any
		var children, attempts []map[string]any
		childBytes, attemptBytes, presence := map[string][]byte{}, map[string][]byte{}, map[string]bool{}
		for n, dep := range deps {
			child := lifecycleBase("ADR0011_TOMBSTONE_CHILD_V1")
			child["intent_ref"], child["dependent_ref"] = intentRef, dep
			childRef, body := proposalOriginalRef("child", child)
			selector := fmt.Sprintf("ref-child-%d.json", n+1)
			childRef["selector"], childBytes[selector] = selector, body
			children = append(children, child)
			entries = append(entries, map[string]any{"dependent_ref": dep, "child_ref": childRef})
			attempt := lifecycleBase("ADR0011_UNLINK_ATTEMPT_V1")
			attempt["intent_ref"], attempt["target_ref"], attempt["ordinal"] = intentRef, dep, n
			attempt["pre_state"], attempt["syscall_outcome"], attempt["post_state"] = "MATCHED", "UNLINKED", "ABSENT"
			attempt["directory_sync"], attempt["close_outcome"] = "SUCCEEDED", "SUCCEEDED"
			attemptRef, body := proposalOriginalRef("attempt", attempt)
			selector = fmt.Sprintf("ref-attempt-%d.json", n+1)
			attemptRef["selector"], attemptBytes[selector] = selector, body
			attempts = append(attempts, attempt)
			attemptRefs = append(attemptRefs, attemptRef)
			presence[dep.(map[string]any)["selector"].(string)] = false
		}
		root, terminal, reconciliation := m["tombstoneRoot"], m["cleanupTerminal"], m["reconciliation"]
		root["dependent_count"], root["children"] = 2, entries
		terminal["attempts"] = attemptRefs
		reconciliation["observations"] = []any{map[string]any{"expected_ref": deps[0], "status": "ABSENT_WITH_TERMINAL"}, map[string]any{"expected_ref": deps[1], "status": "ABSENT_WITH_TERMINAL"}}
		reconciliation["outcome"] = "TERMINAL_REPLAY_VERIFIED"
		e := lifecycleExpectation{Epoch: 1, Intent: intent, IntentRef: intentRef, Dependents: deps, Children: children, ChildBytes: childBytes, TargetBytes: targets, AttemptBytes: attemptBytes, RootVerified: true, BytesPresent: presence, Attempt: attempts[0]}
		return root, terminal, reconciliation, e, attempts
	}
	t.Run("two dependent bound attempts", func(t *testing.T) {
		checkSchema := func(r, c, q map[string]any, e lifecycleExpectation, attempts []map[string]any) {
			t.Helper()
			for role, x := range map[string]map[string]any{"fenceIntent": e.Intent, "tombstoneRoot": r, "cleanupTerminal": c, "reconciliation": q, "tombstoneChild": e.Children[0], "unlinkAttempt": attempts[0]} {
				if err := proposalRole(t, "adr0011-lifecycle-v1.proposed.schema.json", lifecycleID, role).Validate(x); err != nil {
					t.Fatalf("schema-invalid %s: %v", role, err)
				}
			}
			for _, x := range e.Children[1:] {
				if err := proposalRole(t, "adr0011-lifecycle-v1.proposed.schema.json", lifecycleID, "tombstoneChild").Validate(x); err != nil {
					t.Fatal(err)
				}
			}
			for _, x := range attempts[1:] {
				if err := proposalRole(t, "adr0011-lifecycle-v1.proposed.schema.json", lifecycleID, "unlinkAttempt").Validate(x); err != nil {
					t.Fatal(err)
				}
			}
		}
		r, c, q, e, attempts := twoCase()
		checkSchema(r, c, q, e, attempts)
		if err := replayLifecycle(r, c, q, e); err != nil {
			t.Fatalf("valid two-dependent replay rejected: %v", err)
		}
		for _, tc := range []struct {
			name   string
			mutate func(map[string]any, *lifecycleExpectation, []map[string]any)
		}{
			{"partial terminal second attempt omitted", func(c map[string]any, e *lifecycleExpectation, a []map[string]any) {
				c["outcome"] = "PARTIAL_UNCERTAIN"
				c["attempts"] = c["attempts"].([]any)[:1]
			}},
			{"second attempt omitted", func(c map[string]any, e *lifecycleExpectation, a []map[string]any) {
				c["attempts"] = c["attempts"].([]any)[:1]
				delete(e.AttemptBytes, "ref-attempt-2.json")
			}},
			{"second attempt wrong ordinal", func(c map[string]any, e *lifecycleExpectation, a []map[string]any) {
				a[1]["ordinal"] = 0
				ref, body := proposalOriginalRef("attempt", a[1])
				ref["selector"] = "ref-attempt-2.json"
				c["attempts"].([]any)[1], e.AttemptBytes["ref-attempt-2.json"] = ref, body
			}},
			{"second attempt wrong root identity", func(c map[string]any, e *lifecycleExpectation, a []map[string]any) {
				a[1]["root_identity"] = map[string]any{"device": 2, "inode": 1, "root_uri": "file:///tmp"}
				ref, body := proposalOriginalRef("attempt", a[1])
				ref["selector"] = "ref-attempt-2.json"
				c["attempts"].([]any)[1], e.AttemptBytes["ref-attempt-2.json"] = ref, body
			}},
			{"second attempt wrong predecessor epoch", func(c map[string]any, e *lifecycleExpectation, a []map[string]any) {
				a[1]["previous_epoch_digest"] = digestB
				ref, body := proposalOriginalRef("attempt", a[1])
				ref["selector"] = "ref-attempt-2.json"
				c["attempts"].([]any)[1], e.AttemptBytes["ref-attempt-2.json"] = ref, body
			}},
			{"second attempt swapped target", func(c map[string]any, e *lifecycleExpectation, a []map[string]any) {
				a[1]["target_ref"] = e.Dependents[0]
				ref, body := proposalOriginalRef("attempt", a[1])
				ref["selector"] = "ref-attempt-2.json"
				c["attempts"].([]any)[1] = ref
				e.AttemptBytes["ref-attempt-2.json"] = body
			}},
			{"second attempt failed close", func(c map[string]any, e *lifecycleExpectation, a []map[string]any) {
				a[1]["close_outcome"] = "FAILED"
				ref, body := proposalOriginalRef("attempt", a[1])
				ref["selector"] = "ref-attempt-2.json"
				c["attempts"].([]any)[1] = ref
				e.AttemptBytes["ref-attempt-2.json"] = body
			}},
			{"second attempt failed sync", func(c map[string]any, e *lifecycleExpectation, a []map[string]any) {
				a[1]["directory_sync"] = "FAILED"
				ref, body := proposalOriginalRef("attempt", a[1])
				ref["selector"] = "ref-attempt-2.json"
				c["attempts"].([]any)[1] = ref
				e.AttemptBytes["ref-attempt-2.json"] = body
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				r, c, q, e, attempts := twoCase()
				tc.mutate(c, &e, attempts)
				checkSchema(r, c, q, e, attempts)
				if replayLifecycle(r, c, q, e) == nil {
					t.Fatal("second dependent attempt violation accepted")
				}
				t.Log("second attempt guard rejected: " + tc.name)
			})
		}
	})
	r, c, q, e := newCase()
	if err := replayLifecycle(r, c, q, e); err != nil {
		t.Fatal(err)
	}
	t.Run("verified no-unlink abort positive", func(t *testing.T) {
		r, c, q, e := newCase()
		c["outcome"] = "PARTIAL_UNCERTAIN"
		q["outcome"] = "ABORT_VERIFIED_NO_UNLINK"
		q["observations"].([]any)[0].(map[string]any)["status"] = "MATCHED"
		e.NoUnlink, e.BytesPresent["ref-dependent.json"] = true, true
		e.Attempt["syscall_outcome"], e.Attempt["post_state"] = "NOT_ATTEMPTED", "PRESENT"
		e.Attempt["directory_sync"], e.Attempt["close_outcome"] = "NOT_ATTEMPTED", "NOT_ATTEMPTED"
		ref, body := proposalOriginalRef("attempt", e.Attempt)
		c["attempts"], e.AttemptBytes["ref-attempt.json"] = []any{ref}, body
		for role, obj := range map[string]map[string]any{"unlinkAttempt": e.Attempt, "cleanupTerminal": c, "reconciliation": q} {
			if err := proposalRole(t, "adr0011-lifecycle-v1.proposed.schema.json", lifecycleID, role).Validate(obj); err != nil {
				t.Fatal(err)
			}
		}
		if err := replayLifecycle(r, c, q, e); err != nil {
			t.Fatal(err)
		}
	})
	for name, mutate := range map[string]func(map[string]any, map[string]any, *lifecycleExpectation){
		"partial terminal missing attempts": func(c, q map[string]any, e *lifecycleExpectation) {
			c["outcome"] = "PARTIAL_UNCERTAIN"
			c["attempts"] = []any{}
		},
		"abort contradicts unlink original": func(c, q map[string]any, e *lifecycleExpectation) {
			c["outcome"] = "PARTIAL_UNCERTAIN"
			q["outcome"] = "ABORT_VERIFIED_NO_UNLINK"
			e.NoUnlink = true
			e.BytesPresent["ref-dependent.json"] = true
		},
		"reconciliation wrong ref": func(c, q map[string]any, e *lifecycleExpectation) {
			q["observations"].([]any)[0].(map[string]any)["expected_ref"] = lifecycleProposalRef("dependent")
		},
		"reconciliation wrong status": func(c, q map[string]any, e *lifecycleExpectation) {
			q["observations"].([]any)[0].(map[string]any)["status"] = "MATCHED"
		},
		"reconciliation replay with partial terminal": func(c, q map[string]any, e *lifecycleExpectation) { c["outcome"] = "PARTIAL_UNCERTAIN" },
	} {
		t.Run(name, func(t *testing.T) {
			r, c, q, e := newCase()
			mutate(c, q, &e)
			if replayLifecycle(r, c, q, e) == nil {
				t.Fatal("accepted lifecycle contradiction")
			}
		})
	}
	t.Run("canonical child and bound attempt guards", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			mutate func(map[string]any, map[string]any, map[string]any, *lifecycleExpectation)
		}{
			{"swapped child digest with valid attempt", func(r, c, q map[string]any, e *lifecycleExpectation) {
				r["children"].([]any)[0].(map[string]any)["child_ref"].(map[string]any)["digest"] = digestB
			}},
			{"missing bound attempt", func(r, c, q map[string]any, e *lifecycleExpectation) { c["attempts"] = []any{} }},
			{"unexplained presence", func(r, c, q map[string]any, e *lifecycleExpectation) { e.BytesPresent["other.json"] = true }},
			{"failed close", func(r, c, q map[string]any, e *lifecycleExpectation) {
				e.Attempt["close_outcome"] = "FAILED"
				ref, body := proposalOriginalRef("attempt", e.Attempt)
				e.AttemptBytes["ref-attempt.json"] = body
				c["attempts"] = []any{ref}
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				r, c, q, e := newCase()
				tc.mutate(r, c, q, &e)
				if replayLifecycle(r, c, q, e) == nil {
					t.Fatal("lifecycle guard accepted violating state")
				}
				t.Log("lifecycle guard rejected: " + tc.name)
			})
		}
	})
	cases := map[string]func(map[string]any, map[string]any, map[string]any, *lifecycleExpectation){"partial children without verified root": func(r, c, q map[string]any, e *lifecycleExpectation) { e.RootVerified = false; e.Children = nil }, "count mismatch": func(r, c, q map[string]any, e *lifecycleExpectation) { r["dependent_count"] = 2 }, "child root mismatch": func(r, c, q map[string]any, e *lifecycleExpectation) { e.Children[0]["epoch"] = 2 }, "stale epoch": func(r, c, q map[string]any, e *lifecycleExpectation) { e.Epoch = 2 }, "already missing object": func(r, c, q map[string]any, e *lifecycleExpectation) {
		e.Attempt["pre_state"] = "MISSING"
		e.Attempt["syscall_outcome"] = "NOT_ATTEMPTED"
	}, "unlink success sync failure": func(r, c, q map[string]any, e *lifecycleExpectation) { e.Attempt["directory_sync"] = "FAILED" }, "reappearing bytes": func(r, c, q map[string]any, e *lifecycleExpectation) { e.BytesPresent["ref-dependent.json"] = true }, "no unlink abort missing bytes": func(r, c, q map[string]any, e *lifecycleExpectation) {
		c["outcome"] = "PARTIAL_UNCERTAIN"
		q["outcome"] = "ABORT_VERIFIED_NO_UNLINK"
		e.NoUnlink = true
	}}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r, c, q, e := newCase()
			mutate(r, c, q, &e)
			for role, x := range map[string]map[string]any{"tombstoneRoot": r, "cleanupTerminal": c, "reconciliation": q} {
				s := proposalRole(t, "adr0011-lifecycle-v1.proposed.schema.json", lifecycleID, role)
				if err := s.Validate(x); err != nil {
					t.Fatalf("semantic fixture must be schema-valid: %v", err)
				}
			}
			if replayLifecycle(r, c, q, e) == nil {
				t.Fatal("invalid lifecycle replay accepted")
			}
		})
	}
}
