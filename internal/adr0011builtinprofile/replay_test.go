package adr0011builtinprofile

import (
	"bytes"
	"errors"
	"testing"
)

func TestPrivateHeldV1ReplayComparator(t *testing.T) {
	// The selected originals are prepared independently of replay claimants.
	p := BuiltinV1()
	measurement := canonical(map[string]any{"role": "ADR0011_INSTALLED_EXECUTABLE_MEASUREMENT_V1", "version": 1, "profile_digest": digest(p.Profile), "executable_digest": digest([]byte("installed")), "executable_byte_length": 9, "adapter_scope": "LINKED_INTO_INSTALLED_EXECUTABLE_ONLY_V1", "measurement_rule": "PIN_OPENED_EXECUTABLE_FILE_COMPARE_PATH_AT_START_PREWRITE_POSTCAPTURE_V1"})
	targetImpl := targetImplementation(digest(measurement))
	profile := object(p.Profile)
	policies := map[string]any{}
	for _, role := range []string{"method", "admission", "privacy", "retention"} {
		policies[role] = field(nested(nested(profile, "policies"), role), "digest")
	}
	schemaDigest := field(nested(profile, "admission_schema"), "digest")
	d := digest([]byte("fixture"))
	ref := func(role, uri string) map[string]any {
		return map[string]any{"role": role, "schema_version": uri, "selector": role + ".json", "digest": d, "byte_length": 1}
	}
	base := "https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.proposed.schema.json#/$defs/"
	plan := canonical(map[string]any{"role": "ADR0011_LOCAL_REFERENCES_PLAN_V1", "version": 1, "profile_digest": digest(p.Profile), "session_id": "synthetic", "generation": 1, "workspace_uri": "file:///fixture", "query_uri": "file:///fixture/a.go", "line": 1, "character": 2, "encoding": "utf-16", "document_version": 1, "document_byte_length": 3, "document_digest": d, "source_ref": ref("prepared_source", base+"preparedSource"), "git_root_uri": "file:///fixture", "git_commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "git_before": ref("host_git_before", base+"hostGit"), "policy_digests": policies, "schema_digest": schemaDigest, "implementation_digest": digest(measurement), "invocation_nonce": "cccccccccccccccccccccccccccccccc", "params_digest": d, "target_schema_digest": digest(p.TargetSchema), "target_policy_digest": digest(p.TargetPolicy), "target_implementation_digest": targetImpl, "limits": map[string]any{"max_messages": 4, "max_wire_bytes": 4096, "timeout_ms": 1000, "max_work": 1000}})
	selection := canonical(map[string]any{"role": "ADR0011_LOCAL_REFERENCES_SELECTION_V1", "version": 1, "profile_digest": digest(p.Profile), "plan_id": domainDigest("ADR0011_LOCAL_REFERENCES_PLAN_V1", plan), "plan_ref": map[string]any{"role": "plan", "schema_version": "https://jaresty.github.io/lsp-trace/schemas/adr0011-builtin-local-references-v1.proposed.schema.json#/$defs/plan", "selector": "plan.json", "digest": digest(plan), "byte_length": len(plan)}, "declaration_ref": ref("production_query_declaration", "https://jaresty.github.io/lsp-trace/schemas/adr0011-production-admission-v1.proposed.schema.json#/$defs/productionQueryDeclaration"), "implementation_digest": digest(measurement), "policy_digests": policies, "schema_digest": schemaDigest, "root_identity_digest": d, "session_id": "synthetic", "generation": 1, "invocation_nonce": "cccccccccccccccccccccccccccccccc", "expected_custody_ref": ref("attempt_host_custody", "https://jaresty.github.io/lsp-trace/schemas/adr0011-production-admission-v1.proposed.schema.json#/$defs/attemptHostCustody"), "target_schema_digest": digest(p.TargetSchema), "target_policy_digest": digest(p.TargetPolicy), "target_implementation_digest": targetImpl})
	p.Measurement, p.Plan, p.Selection = measurement, plan, selection
	prewrite, err := FreezeV1(p)
	if err != nil {
		t.Fatal("ASSERT_PREWRITE_FREEZE_VALID", err)
	}
	p.Descendants = map[string][]byte{"target-receipt": []byte(`{"ref":"held"}`)}
	held, err := prewrite.HoldDescendants(p.Descendants)
	if err != nil {
		t.Fatal("ASSERT_HOST_SELECTED_DESCENDANTS", err)
	}
	if _, err := prewrite.HoldDescendants(map[string][]byte{"target-receipt": []byte(`{"bad":`)}); !errors.Is(err, ErrInvalidFreeze) {
		t.Fatal("ASSERT_INVALID_POSTWRITE_ORIGINAL", err)
	}
	if err = held.Replay(p); err != nil {
		t.Fatal("ASSERT_V1_REPLAY", err)
	}
	invalid := clone(p)
	badPlan := object(invalid.Plan)
	badPlan["version"] = "two"
	invalid.Plan = canonical(badPlan)
	rehashSelection(&invalid)
	invalidFrozen := clone(invalid)
	invalidFrozen.Descendants = nil
	if _, err := FreezeV1(invalidFrozen); !errors.Is(err, ErrInvalidFreeze) {
		t.Fatalf("ASSERT_FREEZE_SCHEMA_INVALID_REHASH: %v", err)
	}
	for name, change := range map[string]func(*Originals){
		"profile_version": func(x *Originals) {
			x.Profile = bytes.Replace(x.Profile, []byte(`"version":1`), []byte(`"version":2`), 1)
		},
		"profile_coherent": func(x *Originals) {
			x.Profile = bytes.Replace(x.Profile, []byte(`"max_acquisition_queries":16`), []byte(`"max_acquisition_queries":17`), 1)
			rehash(x)
		},
		"target_policy": func(x *Originals) {
			x.TargetPolicy = bytes.Replace(x.TargetPolicy, []byte(`"max_symbols":10000`), []byte(`"max_symbols":10001`), 1)
			rehash(x)
		},
		"target_schema": func(x *Originals) { x.TargetSchema = append(append([]byte(nil), x.TargetSchema...), ' '); rehash(x) },
		"measurement": func(x *Originals) {
			x.Measurement = bytes.Replace(x.Measurement, []byte(`"executable_byte_length":9`), []byte(`"executable_byte_length":8`), 1)
			rehash(x)
		},
		"plan": func(x *Originals) {
			x.Plan = bytes.Replace(x.Plan, []byte(`"invocation_nonce":"cccccccccccccccccccccccccccccccc"`), []byte(`"invocation_nonce":"dddddddddddddddddddddddddddddddd"`), 1)
			rehashSelection(x)
		},
		"selection": func(x *Originals) {
			x.Selection = bytes.Replace(x.Selection, []byte(`"invocation_nonce":"cccccccccccccccccccccccccccccccc"`), []byte(`"invocation_nonce":"dddddddddddddddddddddddddddddddd"`), 1)
		},
		"descendant": func(x *Originals) { x.Descendants["target-receipt"] = []byte(`{"ref":"other"}`) },
	} {
		t.Run(name, func(t *testing.T) {
			x := clone(p)
			change(&x)
			if name != "profile_version" && name != "profile_coherent" {
				if err := validateV1Roles(x); err != nil {
					t.Fatalf("ASSERT_COHERENT_SCHEMA_VALID_%s: %v", name, err)
				}
			}
			if err := held.Replay(x); !errors.Is(err, ErrHeldMismatch) {
				t.Fatalf("ASSERT_HELD_COMPARATOR_%s: %v", name, err)
			}
		})
	}
	// A fresh installation may change defaults; held replay has no argument or
	// dependency on current defaults and still checks the same pinned originals.
	if err := held.Replay(p); err != nil {
		t.Fatal("ASSERT_RESTART_V1_NO_CURRENT_DEFAULTS", err)
	}
	snapshot, expected := held.Snapshot(), held.SnapshotDigest()
	restarted, err := RestoreV1(snapshot, expected)
	if err != nil || restarted.Replay(p) != nil {
		t.Fatal("ASSERT_HELD_V1_RESTART_ORIGINAL_BYTES", err)
	}
	snapshot.Plan = append(snapshot.Plan, ' ')
	if _, err := RestoreV1(snapshot, expected); !errors.Is(err, ErrHeldMismatch) {
		t.Fatal("ASSERT_INDEPENDENT_SNAPSHOT_DIGEST", err)
	}
	// Copies returned by Snapshot may be edited without changing the holder.
	if err := held.Replay(p); err != nil {
		t.Fatal("ASSERT_SNAPSHOT_ISOLATION", err)
	}
}

func rehash(x *Originals) {
	m := object(x.Measurement)
	m["profile_digest"] = digest(x.Profile)
	x.Measurement = canonical(m)
	pl := object(x.Plan)
	pl["profile_digest"] = digest(x.Profile)
	pl["implementation_digest"] = digest(x.Measurement)
	pl["target_policy_digest"] = digest(x.TargetPolicy)
	pl["target_schema_digest"] = digest(x.TargetSchema)
	pl["target_implementation_digest"] = targetImplementationFor(digest(x.Measurement), digest(x.TargetPolicy), digest(x.TargetSchema))
	x.Plan = canonical(pl)
	rehashSelection(x)
}
func rehashSelection(x *Originals) {
	pl := object(x.Plan)
	s := object(x.Selection)
	for _, key := range []string{"profile_digest", "implementation_digest", "target_policy_digest", "target_schema_digest", "target_implementation_digest"} {
		s[key] = pl[key]
	}
	s["plan_id"] = domainDigest("ADR0011_LOCAL_REFERENCES_PLAN_V1", x.Plan)
	planRef := nested(s, "plan_ref")
	planRef["digest"], planRef["byte_length"] = digest(x.Plan), len(x.Plan)
	x.Selection = canonical(s)
}
