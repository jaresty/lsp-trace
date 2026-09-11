#!/usr/bin/env python3
import argparse
import csv
import hashlib
import json
import re
import sys
from pathlib import Path

EXPECTED = {
    "calls-v1": ["CALLS"],
    "callback-flow-v1": ["BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK"],
    "state-flow-v1": ["UPDATES_STATE"],
    "ui-lifecycle-v1": ["RENDERS_FROM", "TRIGGERS_RELOAD"],
}
STATUSES = {"MISSING", "PASS", "FAIL", "BLOCKED", "UNSUPPORTED", "INCOMPLETE"}
DIGEST = re.compile(r"sha256:[0-9a-f]{64}\Z")
REVISION = re.compile(r"[0-9a-f]{40}\Z")
SCHEMA = "lsp-trace.program-c-profile-qualification-receipt.v1"
LEGACY_CALLS_RECEIPTS = {
    "sha256:9f09091e4cd5510fe80dc0edcb4111ef640c064d6ed1f0999421a35fdfe9bd2c",
    "sha256:36ff70993224c2c5a4534424896ace7462c3eed140d0b8903c255d6681853890",
}
STRICT_KEYS = {
    "schema_version", "result", "current", "repository_revision", "selection", "profile",
    "coordinate", "policy_matrix_identity", "tool_identity", "exact_command", "command_bindings",
    "run_identity", "input_sha256", "result_sha256", "relation_evidence", "claim_ceiling",
}
PLACEHOLDER = re.compile(r"^\$([A-Z][A-Z0-9_]*)$")


def canonical_digest(profile):
    value = {"name": profile["name"], "relations": profile["relations"]}
    payload = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return "sha256:" + hashlib.sha256(payload).hexdigest()


def fail(assertion, reason):
    print(f"{assertion} result=FAIL reason={reason}")
    raise SystemExit(1)


def regular_receipt(root, value, digest):
    path = Path(value)
    if path.is_absolute() or ".." in path.parts:
        return False
    full = root / path
    if not full.is_file() or full.is_symlink():
        return False
    actual = "sha256:" + hashlib.sha256(full.read_bytes()).hexdigest()
    return actual == digest


def exact_keys(value, keys):
    return isinstance(value, dict) and set(value) == set(keys)


def nonempty_strings(value):
    return isinstance(value, dict) and bool(value) and all(isinstance(k, str) and k and isinstance(v, str) and v for k, v in value.items())


def receipt_error(receipt, row, profile, profiles_path, matrix_path, root):
    profile_name, language, framework, provider, version, required, status, _, file_digest = row
    if receipt.get("schema_version") != SCHEMA:
        return "schema-version"
    if receipt.get("result") != "PASS" or receipt.get("current") is not True:
        return "result-or-current"
    revision = receipt.get("repository_revision")
    tool = receipt.get("tool_identity")
    if not REVISION.fullmatch(revision or "") or not exact_keys(tool, {"name", "revision", "version_output"}):
        return "repository-or-tool-identity"
    if tool["name"] != "lsp-trace" or tool["revision"] != revision or revision not in tool["version_output"]:
        return "repository-revision-mismatch"
    coordinate = receipt.get("coordinate")
    coordinate_keys = {"language", "framework", "provider", "provider_version", "observed_provider_identity", "observed_provider_version"}
    if not exact_keys(coordinate, coordinate_keys):
        return "coordinate-shape"
    if [coordinate[k] for k in ("language", "framework", "provider", "provider_version")] != [language, framework, provider, version]:
        return "coordinate-mismatch"
    if coordinate["observed_provider_identity"] != provider or coordinate["observed_provider_version"] != version:
        return "observed-provider-identity"
    projected = receipt.get("profile")
    relations = [item["kind"] for item in profile["relations"]]
    if not exact_keys(projected, {"name", "logical_digest", "relations"}):
        return "profile-shape"
    if projected != {"name": profile_name, "logical_digest": profile["logical_digest"], "relations": relations}:
        return "profile-mismatch"
    policy = receipt.get("policy_matrix_identity")
    policy_keys = {"profile_manifest", "profile_manifest_sha256", "qualification_matrix", "qualification_matrix_prepublication_sha256", "matrix_row"}
    if not exact_keys(policy, policy_keys):
        return "policy-binding-shape"
    try:
        expected_profiles = profiles_path.resolve().relative_to(root).as_posix()
        expected_matrix = matrix_path.resolve().relative_to(root).as_posix()
    except ValueError:
        return "policy-path-outside-root"
    manifest_digest = "sha256:" + hashlib.sha256(profiles_path.read_bytes()).hexdigest()
    if policy["profile_manifest"] != expected_profiles or policy["profile_manifest_sha256"] != manifest_digest:
        return "manifest-binding"
    if policy["qualification_matrix"] != expected_matrix or policy["matrix_row"] != row[:7] or not DIGEST.fullmatch(policy["qualification_matrix_prepublication_sha256"] or ""):
        return "matrix-binding"
    selection = receipt.get("selection")
    if not exact_keys(selection, {"status", "supersedes"}) or selection["status"] != "CURRENT" or (selection["supersedes"] is not None and not DIGEST.fullmatch(selection["supersedes"] or "")):
        return "stale-or-superseded"
    command = receipt.get("exact_command")
    bindings = receipt.get("command_bindings")
    if not isinstance(command, list) or len(command) < 2 or not all(isinstance(v, str) and v for v in command):
        return "exact-command"
    if not nonempty_strings(bindings):
        return "command-bindings"
    placeholders = {match.group(1) for value in command if (match := PLACEHOLDER.fullmatch(value))}
    if placeholders != set(bindings):
        return "resolved-bindings"
    run = receipt.get("run_identity")
    if not nonempty_strings(run) or len(run) < 2:
        return "run-generation-identity"
    inputs = receipt.get("input_sha256")
    if not isinstance(inputs, dict) or not inputs or not all(isinstance(k, str) and k and DIGEST.fullmatch(v or "") for k, v in inputs.items()):
        return "input-digests"
    if inputs.get("qualification_matrix_prepublication") != policy["qualification_matrix_prepublication_sha256"]:
        return "matrix-input-digest"
    if not DIGEST.fullmatch(receipt.get("result_sha256") or ""):
        return "result-digest"
    evidence = receipt.get("relation_evidence")
    if not isinstance(evidence, list) or [item.get("relation") for item in evidence if isinstance(item, dict)] != relations:
        return "relation-evidence-order"
    evidence_keys = {"relation", "status", "count", "custody", "replay"}
    for item in evidence:
        if not exact_keys(item, evidence_keys) or item["status"] != "PASS" or not isinstance(item["count"], int) or item["count"] < 1:
            return "blocked-or-empty-relation"
        expected_custody = "SERVER_REPORTED" if item["relation"] == "CALLS" else "PROVIDER_PROVED"
        if item["custody"] != expected_custody or item["replay"] != "EXACT_BYTES":
            return "relation-custody-or-replay"
    ceiling = receipt.get("claim_ceiling")
    expected_ceiling = {"scope": "EXACT_COORDINATE_AND_INPUTS_ONLY", "whole_source_complete": False,
                        "source_authenticated": False, "cross_coordinate_transfer": False}
    if ceiling != expected_ceiling:
        return "claim-ceiling"
    if set(receipt) != STRICT_KEYS:
        return "receipt-shape"
    return None


def legacy_receipt_error(receipt, row, profile):
    profile_name, language, framework, provider, version, _, _, _, _ = row
    relations = [item["kind"] for item in profile["relations"]]
    if receipt.get("schema_version") != SCHEMA or receipt.get("result") != "PASS" or receipt.get("current") is not True:
        return "legacy-result"
    if receipt.get("repository_revision") != receipt.get("tool_identity", {}).get("revision"):
        return "legacy-revision"
    if receipt.get("profile") != {"name": profile_name, "logical_digest": profile["logical_digest"], "relations": relations}:
        return "legacy-profile"
    coordinate = receipt.get("coordinate", {})
    if [coordinate.get(k) for k in ("language", "framework", "provider", "provider_version")] != [language, framework, provider, version]:
        return "legacy-coordinate"
    if not receipt.get("exact_command") or not receipt.get("input_sha256") or not DIGEST.fullmatch(receipt.get("result_sha256", "")):
        return "legacy-command-or-digests"
    evidence = receipt.get("evidence", {})
    if evidence.get("v5_native_verification") != "PASS" or evidence.get("deterministic_replay") != "EXACT_BYTES":
        return "legacy-evidence"
    if not receipt.get("claim_ceiling") and not receipt.get("authority_ceiling"):
        return "legacy-claim-ceiling"
    return None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--profiles", required=True)
    parser.add_argument("--matrix", required=True)
    parser.add_argument("--root", required=True)
    parser.add_argument("--require-profile", action="append", default=[])
    args = parser.parse_args()
    root = Path(args.root).resolve()
    profiles_path = Path(args.profiles)
    matrix_path = Path(args.matrix)

    try:
        artifact = json.loads(profiles_path.read_text())
    except (OSError, json.JSONDecodeError) as exc:
        fail("ASSERT_PROGRAM_C_PROFILE_MANIFEST", f"unreadable:{exc}")

    if artifact.get("schema_version") != "lsp-trace.program-c-projection-profiles.v1" or artifact.get("status") != "DEFERRED":
        fail("ASSERT_PROGRAM_C_PROFILE_MANIFEST", "identity-or-status")
    profiles = artifact.get("profiles")
    if not isinstance(profiles, list) or [p.get("name") for p in profiles] != list(EXPECTED):
        fail("ASSERT_PROGRAM_C_PROFILE_INVENTORY", "profile-set-or-order")

    by_name = {}
    for profile in profiles:
        name = profile["name"]
        relations = profile.get("relations")
        if not isinstance(relations, list) or [r.get("kind") for r in relations] != EXPECTED[name]:
            fail("ASSERT_PROGRAM_C_PROFILE_RELATIONS", name)
        for relation in relations:
            if relation != {"kind": relation["kind"], "direction": "directed", "multiplicity": "preserve", "weight": 1}:
                fail("ASSERT_PROGRAM_C_PROFILE_SEMANTICS", name)
        if profile.get("logical_digest") != canonical_digest(profile):
            fail("ASSERT_PROGRAM_C_PROFILE_DIGESTS", name)
        by_name[name] = profile

    compositions = artifact.get("compositions")
    if not isinstance(compositions, list) or len(compositions) != 1:
        fail("ASSERT_PROGRAM_C_ALL_QUALIFIED", "composition-count")
    composition = compositions[0]
    if composition.get("name") != "all-qualified-v1" or composition.get("requires_all_members") is not True or composition.get("on_missing_member") != "INCOMPLETE":
        fail("ASSERT_PROGRAM_C_ALL_QUALIFIED", "composition-policy")
    members = composition.get("members")
    if not isinstance(members, list) or [m.get("profile") for m in members] != list(EXPECTED):
        fail("ASSERT_PROGRAM_C_ALL_QUALIFIED", "composition-members")
    for member in members:
        if member.get("digest") != by_name[member["profile"]]["logical_digest"]:
            fail("ASSERT_PROGRAM_C_ALL_QUALIFIED", "member-digest")

    mixed = artifact.get("mixed_language_policy", {})
    if not all(mixed.get(key) is True for key in (
        "require_passing_tuple_per_edge",
        "require_qualified_cross_language_edges",
        "preserve_relation_kind",
        "preserve_provider_identity",
        "preserve_language_framework_scope",
    )):
        fail("ASSERT_PROGRAM_C_MIXED_LANGUAGE_POLICY", "missing-fail-closed-rule")

    try:
        with matrix_path.open(newline="") as stream:
            rows = [row for row in csv.reader((line for line in stream if not line.startswith("#")), delimiter="\t") if row]
    except OSError as exc:
        fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", f"unreadable:{exc}")

    seen = set()
    legacy_compatibility = 0
    status_by_profile = {name: [] for name in EXPECTED}
    for row in rows:
        if len(row) != 9:
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "column-count")
        profile, language, framework, provider, version, required, status, receipt, digest = row
        key = (profile, language, framework, provider, version)
        if key in seen:
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "duplicate-tuple")
        seen.add(key)
        if profile not in EXPECTED or not all((language, framework, provider, version)):
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "unknown-or-empty-coordinate")
        if required not in {"yes", "no"} or status not in STATUSES:
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "invalid-required-or-status")
        if status == "MISSING":
            if receipt != "-" or digest != "-":
                fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "missing-row-has-receipt")
        else:
            if status != "PASS":
                fail("ASSERT_PROGRAM_C_PROFILE_RECEIPT", "non-pass-row-selects-receipt")
            if not DIGEST.fullmatch(digest) or not regular_receipt(root, receipt, digest):
                fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", "unverified-receipt")
            try:
                document = json.loads((root / receipt).read_text())
            except (OSError, json.JSONDecodeError) as exc:
                fail("ASSERT_PROGRAM_C_PROFILE_RECEIPT", f"unreadable:{exc}")
            if digest in LEGACY_CALLS_RECEIPTS:
                reason = legacy_receipt_error(document, row, by_name[profile])
                legacy_compatibility += 1
            else:
                reason = receipt_error(document, row, by_name[profile], profiles_path, matrix_path, root)
            if reason:
                fail("ASSERT_PROGRAM_C_PROFILE_RECEIPT", reason)
        status_by_profile[profile].append((required, status))

    for name, statuses in status_by_profile.items():
        if not statuses:
            fail("ASSERT_PROGRAM_C_PROFILE_MATRIX", f"missing-profile:{name}")

    print("ASSERT_PROGRAM_C_PROFILE_MANIFEST result=PASS")
    print("ASSERT_PROGRAM_C_PROFILE_INVENTORY result=PASS profiles=" + ",".join(EXPECTED))
    print("ASSERT_PROGRAM_C_PROFILE_RELATIONS result=PASS")
    print("ASSERT_PROGRAM_C_PROFILE_DIGESTS result=PASS")
    print("ASSERT_PROGRAM_C_ALL_QUALIFIED result=PASS")
    print("ASSERT_PROGRAM_C_MIXED_LANGUAGE_POLICY result=PASS")
    print(f"ASSERT_PROGRAM_C_PROFILE_MATRIX result=PASS tuples={len(rows)}")
    print(f"ASSERT_PROGRAM_C_PROFILE_RECEIPT result=PASS legacy_compatibility={legacy_compatibility}")

    def require(name):
        if name in EXPECTED:
            required_rows = [status for required, status in status_by_profile[name] if required == "yes"]
            return bool(required_rows) and all(status == "PASS" for status in required_rows)
        if name == "all-qualified-v1":
            return all(require(member) for member in EXPECTED)
        fail("ASSERT_PROGRAM_C_REQUESTED_PROFILE", f"unknown-profile:{name}")

    for name in args.require_profile:
        if not require(name):
            fail("ASSERT_PROGRAM_C_REQUESTED_PROFILE", f"not-qualified:{name}")
        print(f"ASSERT_PROGRAM_C_REQUESTED_PROFILE result=PASS profile={name}")


if __name__ == "__main__":
    main()
