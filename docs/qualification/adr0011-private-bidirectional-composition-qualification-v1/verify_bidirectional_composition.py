#!/usr/bin/env python3
"""Deterministic verifier/composer for private ADR0011 bidirectional composition qualification.

Correction v2: the top-level representation remains ordered {incoming,outgoing}, but
both values are directional envelopes. A success envelope preserves an immutable
reference to the accepted directional qualification. A failure envelope is a bounded
private, evidence-only fixture-native directional failure result. Invalid composition
inputs remain rejected and are not represented as failures.

The composer reads only metadata-bearing dependency files (MANIFEST/RESULT/CHECKSUMS)
and never reads, copies, decodes, remarshal, concatenates, or reconstructs raw provider
request/response bytes. Raw evidence remains referenced only by immutable path/hash pins.
"""
from __future__ import annotations

import argparse
import copy
import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[3]
PKG = Path(__file__).resolve().parent
EXPECTED_HEAD = "b196fa22beffc1a0fa18c4fc6bc192f07967721e"
CANONICAL_JSON_KW = dict(ensure_ascii=False, indent=None, separators=(",", ":"))

DEPENDENCIES = {
    "incoming_generic": {
        "direction": "incoming",
        "kind": "generic_semantics",
        "profile_id": "CALLS_LSP_INCOMING_EXACT_V1",
        "accepted_token": "CALLS_LSP_INCOMING_EXACT_V1_SEMANTICS_GO",
        "commit": "c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a",
        "package_path": "docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1",
        "checksums_sha256": "844e2c072474f6b3756ec221ea0b6df2888d775e9b866e1da0bbef030e35b351",
        "manifest_sha256": "8657876ab14ad6c4939218331d8a58e3c222da61b21ae55200644e5c43674f7d",
    },
    "incoming_profile": {
        "direction": "incoming",
        "kind": "provider_profile",
        "profile_id": "CALLS_GO_GOPLS_INCOMING_EXACT_V1",
        "generic_profile_id": "CALLS_LSP_INCOMING_EXACT_V1",
        "accepted_token": "CALLS_GO_GOPLS_INCOMING_EXACT_V1_GO",
        "commit": "61bfa264d30595016ddb562082dd89d8d3f175da",
        "package_path": "docs/qualification/adr0011-calls-go-gopls-incoming-profile-qualification-v1",
        "checksums_sha256": "e0b1a9ea0b24e51768fad3bd89194ac50023dab8e370e6cc9a96ff6d78135abf",
        "manifest_sha256": "675456686a9b073b4462685ad833456ae02942b659f6ecc70bc8504dab401579",
        "result_sha256": "e9c8cce4b77dc91a8ed435166fbf88ce9d6e12bbc28bd4ad0db4fbbb45cc4154",
    },
    "outgoing_generic": {
        "direction": "outgoing",
        "kind": "generic_semantics",
        "profile_id": "CALLS_LSP_OUTGOING_EXACT_V1",
        "accepted_token": "CALLS_LSP_OUTGOING_EXACT_V1_SEMANTICS_GO",
        "commit": "17057a8c53af8e8fe66be9c37788bbf59d10c5cd",
        "package_path": "docs/qualification/adr0011-calls-lsp-outgoing-exact-v1-generic-semantics-v1",
        "checksums_sha256": "8c0efb403147ff70ad4def97809c65ec026c4984fdd8c8b5f7e1f9d0e31de6db",
        "manifest_sha256": "1385fa90ab3477959460b042ef1daae3ad0a31d9a851e26bdd45c4c572192ead",
    },
    "outgoing_profile": {
        "direction": "outgoing",
        "kind": "provider_profile",
        "profile_id": "CALLS_GO_GOPLS_OUTGOING_EXACT_V1",
        "generic_profile_id": "CALLS_LSP_OUTGOING_EXACT_V1",
        "accepted_token": "CALLS_GO_GOPLS_OUTGOING_EXACT_V1_GO",
        "commit": "b196fa22beffc1a0fa18c4fc6bc192f07967721e",
        "package_path": "docs/qualification/adr0011-calls-go-gopls-outgoing-profile-qualification-v1",
        "checksums_sha256": "411e2195723fc5ac5bb6a7af27331b187ca016d0d75767b634454223e5978e80",
        "manifest_sha256": "7fd171adce13cda709193365fac10c34566a3472e093f347650f2cbf336fa543",
        "result_sha256": "e6a60c88f4bd00d2e220d160765c8ee903903523cb86cfb7d814e31a7c433c9f",
    },
}

EVIDENCE_REFS = {
    "incoming": {
        "failures_ref": {"path": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming/FAILURES.jsonl", "sha256": "ad2785c3690f1ce104cd803058c9dace7fbb379ad65bd18d8ebc87ab7cef345f"},
        "numeric_bounds_ref": {"path": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming/NUMERIC_BOUNDS.json", "sha256": "46a484d004dfcf8c019290aa8d8a724e1a1a1682982c5460c1976c68c47b6397", "scope": "target-set-relative observed maxima; not universal; no ceiling change"},
        "final_summary_ref": {"path": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming/FINAL_SUMMARY.json", "sha256": "7bab05495d489f4e12f2e25a9ab16ba9206ca7eb74f62f9d947f92498f55066f"},
        "ledger_ref": {"path": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming/ledgers/incoming.attempts.jsonl", "sha256": "5c181ac1304547ba8f9e915aa512f3c4e93ca1fd8594da381f7f61b165d7139f", "merge_admitted": False},
        "custody_refs": {"run_id": "incoming-custody-correction-v2", "run_dir_sha256": "ddc7a39f2322ed5d3e0730f376563ef69e8ffd31c982c1f0928d3ea7f0174633", "final_checksums_sha256": "ba003061790bb4033bcabf85480e92ec97aba4522ab9877d99ccabbf24db60a8", "final_seal_sha256": "a15ddcb57a0f908e13e7742dd0697dff717fd08e66e2aee28a478020f1c3ba34", "artifact_count": 53380, "opaque": True},
        "raw_provenance_ref": {"historical_raw_aggregate_digest": "dfedbc400dcdb7315143fd0479e791a67dda79331112bd922446e4f770c717bc", "historical_sidecar_count": 26708, "correction_artifact_inventory_digest": "2e59bdcc6ff3b1dab8d8303a18f544c1e8c7af6956802b2da14ec5c54c7939cc", "reference_only": True},
    },
    "outgoing": {
        "failures_ref": {"path": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-outgoing/FAILURES.jsonl", "sha256": "ac9e323cb2b58c02fd9c50ff860dea32894278572ecd46bd50c0e0bbc2560dbe"},
        "numeric_bounds_ref": {"path": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-outgoing/NUMERIC_BOUNDS.json", "sha256": "abe46e255bad7b0b2838f7152a4acf5d5bb8016c4cfeb1852ec3b5be3eceadb4", "scope": "target-set-relative observed maxima; not universal; no ceiling change"},
        "final_summary_ref": {"path": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-outgoing/FINAL_SUMMARY.json", "sha256": "e6c8798f57e020f4af3c8fc149acdc2d21a76039d427da241db62a5234858d46"},
        "ledger_ref": {"path": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-outgoing/ledgers/outgoing.attempts.jsonl", "sha256": "1442c679853e85866ec3070fe745c8f66d6eb26d60fb0c9a28ff9b0e0b66a897", "merge_admitted": False},
        "custody_refs": {"run_id": "outgoing-custody-correction-v1", "run_dir": "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/runs/outgoing-custody-correction-v1/full-outgoing", "final_checksums_sha256": "b01bcfd5018432eed725af79a41ce049ac3ba2941d7d2f3a40077367047bb93b", "final_seal_sha256": "a511c0f2ff435a30ba963f7b70671880959cf234bf37c7dc7a195b7886ba795c", "artifact_count": 53640, "opaque": True},
        "raw_provenance_ref": {"fresh_artifact_count": 53640, "final_checksums_sha256": "b01bcfd5018432eed725af79a41ce049ac3ba2941d7d2f3a40077367047bb93b", "final_seal_sha256": "a511c0f2ff435a30ba963f7b70671880959cf234bf37c7dc7a195b7886ba795c", "reference_only": True},
    },
}

COUNTS = {
    "incoming": {"attempted": 5900, "succeeded_nonempty": 5608, "succeeded_empty": 225, "failed_lsp_error": 67, "refused": 0, "one_final_disposition_ordinals": 5900},
    "outgoing": {"attempted": 5900, "succeeded_nonempty": 4378, "succeeded_empty": 1455, "failed_lsp_error": 67, "refused": 0, "one_final_disposition_ordinals": 5900},
}

ENVELOPE_KEYS = ["schema", "direction", "status", "authority", "accepted", "production_admission", "production_refusal", "target_set_relative_completeness", "frontier", "raw_provider_bytes_policy", "ceiling", "opaque_custody", "calls_only", "immutable_reference_only", "peer_failure_effect", "payload"]
SUCCESS_PAYLOAD_KEYS = ["schema", "payload_type", "direction", "profile_id", "generic_profile_id", "source_commit", "package_path", "package_checksums_sha256", "manifest_sha256", "result_sha256", "disposition", "go_evidence_reference", "accepted_profile_qualification_state", "counts", "failures_ref", "final_summary_ref", "ledger_ref", "numeric_bounds_ref", "raw_provenance_ref", "custody_refs", "forbidden_synthesis"]
FAILURE_PAYLOAD_KEYS = ["schema", "payload_type", "direction", "failure_disposition", "failure_code", "failure_stage", "message_digest", "evidence_ref", "private_evidence_only", "counts_claimed", "success_claims", "inverse_inference", "peer_evidence_used"]
BAD_KEY_ALIASES = {"sharedAuthority", "shared_authority", "global_authority", "combined_authority", "sharedCompleteness", "shared_completeness", "global_completeness", "combined_completeness", "combined_ceiling", "combinedCeiling", "merged_ledger", "merged_failure", "ledger_concat", "ledger_concatenation", "dedup", "dedupe", "union_graph", "graph_claim", "inverse_synthesis", "raw_bytes", "provider_bytes", "canonical_bytes_base64", "remarshaled_raw", "raw_copy", "default_on", "public_surface", "production_enabled", "stage8", "adr0007", "peer_evidence_copy", "peer_evidence", "promoted_success", "success_counts"}
MESSAGE_DIGEST_RE = re.compile(r"^sha256:[0-9a-f]{64}$")


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def canonical_bytes(obj: Any) -> bytes:
    return (json.dumps(obj, **CANONICAL_JSON_KW) + "\n").encode("utf-8")


def canonical_sha(obj: Any) -> str:
    return sha256_bytes(canonical_bytes(obj))


def sha_file(path: str) -> str:
    return sha256_bytes((ROOT / path).read_bytes())


def load_json(path: str) -> Any:
    with (ROOT / path).open("r", encoding="utf-8") as f:
        return json.load(f)


def git_output(args: list[str]) -> bytes:
    return subprocess.check_output(["git", *args], cwd=ROOT)


def git_text(args: list[str]) -> str:
    return git_output(args).decode("utf-8").strip()


def git_show_sha(commit: str, path: str) -> str:
    return sha256_bytes(git_output(["show", f"{commit}:{path}"]))


def common_envelope(direction: str, status: str, payload: dict[str, Any]) -> dict[str, Any]:
    return {
        "schema": "lsp-trace.adr0011.private-bidirectional-composition.direction-envelope.v2",
        "direction": direction,
        "status": status,
        "authority": 0,
        "accepted": False,
        "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED",
        "production_refusal": True,
        "target_set_relative_completeness": {"status": "UNKNOWN", "target_count": 5900 if status == "SUCCESS" else None, "repository_or_graph_completeness": "NOT_CLAIMED"},
        "frontier": "UNKNOWN",
        "raw_provider_bytes_policy": "NO_READ_NO_COPY_NO_REMARSHAL_REFERENCE_BY_PATH_HASH_ONLY",
        "ceiling": {"per_direction_raw_bytes": 33554432, "aggregate_live_bytes": 33554432, "combined_ceiling_admitted": False, "ceiling_change": False},
        "opaque_custody": True,
        "calls_only": True,
        "immutable_reference_only": True,
        "peer_failure_effect": "PEER_UNCHANGED_NOT_ERASED_NOT_RELABELED",
        "payload": payload,
    }


def success_payload(direction: str) -> dict[str, Any]:
    pdep = DEPENDENCIES[f"{direction}_profile"]
    gdep = DEPENDENCIES[f"{direction}_generic"]
    return {
        "schema": "lsp-trace.adr0011.private-bidirectional-composition.success-reference.v2",
        "payload_type": "accepted_directional_success_reference",
        "direction": direction,
        "profile_id": pdep["profile_id"],
        "generic_profile_id": pdep["generic_profile_id"],
        "source_commit": pdep["commit"],
        "package_path": pdep["package_path"],
        "package_checksums_sha256": pdep["checksums_sha256"],
        "manifest_sha256": pdep["manifest_sha256"],
        "result_sha256": pdep["result_sha256"],
        "disposition": "AUTHORED_NOT_GO" if direction == "incoming" else "CALLS_GO_GOPLS_OUTGOING_PROFILE_QUALIFICATION_AUTHORED_NOT_GO",
        "go_evidence_reference": {"dependency_token": pdep["accepted_token"], "generic_dependency_token": gdep["accepted_token"], "generic_commit": gdep["commit"], "generic_package_path": gdep["package_path"], "generic_package_checksums_sha256": gdep["checksums_sha256"], "generic_manifest_sha256": gdep["manifest_sha256"], "receipt": "20261014131937-8575"},
        "accepted_profile_qualification_state": {"dependency_go_accepted_by_user": True, "producer_self_go": False, "result_accepted": False, "result_authority": 0, "production_go": False},
        "counts": COUNTS[direction],
        "failures_ref": EVIDENCE_REFS[direction]["failures_ref"],
        "final_summary_ref": EVIDENCE_REFS[direction]["final_summary_ref"],
        "ledger_ref": EVIDENCE_REFS[direction]["ledger_ref"],
        "numeric_bounds_ref": EVIDENCE_REFS[direction]["numeric_bounds_ref"],
        "raw_provenance_ref": EVIDENCE_REFS[direction]["raw_provenance_ref"],
        "custody_refs": EVIDENCE_REFS[direction]["custody_refs"],
        "forbidden_synthesis": {"inverse_synthesis": False, "dedup_union": False, "graph_claim": False, "ledger_concatenation": False, "shared_authority": False, "shared_completeness": False},
    }


def success_envelope(direction: str) -> dict[str, Any]:
    return common_envelope(direction, "SUCCESS", success_payload(direction))


def failure_payload(direction: str, code: str | None = None, stage: str = "composition_fixture", message: str | None = None) -> dict[str, Any]:
    code = code or f"{direction.upper()}_FIXTURE_NATIVE_FAILURE"
    msg = message or f"private evidence-only {direction} fixture-native failure; no successful counts or inverse inference"
    return {
        "schema": "lsp-trace.adr0011.private-bidirectional-composition.fixture-native-failure.v1",
        "payload_type": "fixture_native_directional_failure",
        "direction": direction,
        "failure_disposition": "FAILURE",
        "failure_code": code,
        "failure_stage": stage,
        "message_digest": "sha256:" + sha256_bytes(msg.encode("utf-8")),
        "evidence_ref": {"kind": "fixture-native-private-evidence-only", "path": "docs/qualification/adr0011-private-bidirectional-composition-qualification-v1/fixtures.json", "selector": f"cases/{direction}_failure_valid", "raw_provider_bytes": "NOT_APPLICABLE"},
        "private_evidence_only": True,
        "counts_claimed": False,
        "success_claims": False,
        "inverse_inference": False,
        "peer_evidence_used": False,
    }


def failure_envelope(direction: str, code: str | None = None, stage: str = "composition_fixture", message: str | None = None) -> dict[str, Any]:
    return common_envelope(direction, "FAILURE", failure_payload(direction, code, stage, message))


def compose(incoming: dict[str, Any], outgoing: dict[str, Any]) -> dict[str, Any]:
    validate_envelope("incoming", incoming)
    validate_envelope("outgoing", outgoing)
    return {"incoming": incoming, "outgoing": outgoing}


def expected_artifact() -> dict[str, Any]:
    return compose(success_envelope("incoming"), success_envelope("outgoing"))


def verify_dependency_pins(log: list[str]) -> None:
    head = git_text(["rev-parse", "HEAD"])
    if head != EXPECTED_HEAD:
        raise AssertionError(f"HEAD_PIN expected {EXPECTED_HEAD} got {head}")
    log.append("PASS HEAD_PIN")
    for name, dep in DEPENDENCIES.items():
        checksums_path = f"{dep['package_path']}/CHECKSUMS.sha256"
        actual = sha_file(checksums_path)
        if actual != dep["checksums_sha256"]:
            raise AssertionError(f"{name} CHECKSUMS hash drift {actual}")
        historical = git_show_sha(dep["commit"], checksums_path)
        if historical != dep["checksums_sha256"]:
            raise AssertionError(f"{name} commit package hash drift {historical}")
        log.append(f"PASS DEPENDENCY_PACKAGE_HASH {name} {dep['commit']} {actual}")
        manifest_path = f"{dep['package_path']}/MANIFEST.json"
        if sha_file(manifest_path) != dep["manifest_sha256"]:
            raise AssertionError(f"{name} manifest hash drift")
        log.append(f"PASS DEPENDENCY_MANIFEST_HASH {name}")
        if dep["kind"] == "provider_profile":
            result_path = f"{dep['package_path']}/RESULT.json"
            if sha_file(result_path) != dep["result_sha256"]:
                raise AssertionError(f"{name} result hash drift")
            log.append(f"PASS DEPENDENCY_RESULT_HASH {name}")


def verify_dependency_fields(log: list[str]) -> None:
    ig = load_json(f"{DEPENDENCIES['incoming_generic']['package_path']}/MANIFEST.json")
    ipr = load_json(f"{DEPENDENCIES['incoming_profile']['package_path']}/RESULT.json")
    og = load_json(f"{DEPENDENCIES['outgoing_generic']['package_path']}/MANIFEST.json")
    opr = load_json(f"{DEPENDENCIES['outgoing_profile']['package_path']}/RESULT.json")
    checks = [
        (ig["profile"], "CALLS_LSP_INCOMING_EXACT_V1", "GENERIC_INCOMING_PROFILE"), (ig["authority"], 0, "GENERIC_INCOMING_AUTHORITY"), (ig["accepted"], False, "GENERIC_INCOMING_ACCEPTED_FALSE"),
        (og["profile"], "CALLS_LSP_OUTGOING_EXACT_V1", "GENERIC_OUTGOING_PROFILE"), (og["authority"], 0, "GENERIC_OUTGOING_AUTHORITY"), (og["accepted"], False, "GENERIC_OUTGOING_ACCEPTED_FALSE"),
        (ipr["profile_id"], "CALLS_GO_GOPLS_INCOMING_EXACT_V1", "INCOMING_RESULT_PROFILE_ID"), (ipr["authority"], 0, "INCOMING_AUTHORITY"), (ipr["accepted"], False, "INCOMING_ACCEPTED_FALSE"), (ipr["production_admission"], "REFUSED_NOT_REQUESTED_NOT_GRANTED", "INCOMING_PRODUCTION_REFUSED"), (ipr["bounded_qualification_candidate"]["frontier"], "UNKNOWN", "INCOMING_FRONTIER"),
        (opr["profile_id"], "CALLS_GO_GOPLS_OUTGOING_EXACT_V1", "OUTGOING_RESULT_PROFILE_ID"), (opr["authority"], 0, "OUTGOING_AUTHORITY"), (opr["accepted"], False, "OUTGOING_ACCEPTED_FALSE"), (opr["production_admission"], "REFUSED_NOT_REQUESTED_NOT_GRANTED", "OUTGOING_PRODUCTION_REFUSED"), (opr["frontier"], "UNKNOWN", "OUTGOING_FRONTIER"),
    ]
    for actual, expected, label in checks:
        if actual != expected:
            raise AssertionError(f"{label} expected {expected!r} got {actual!r}")
        log.append(f"PASS {label}")
    if ipr["bounded_qualification_candidate"]["counts"] != {"succeeded_nonempty": 5608, "succeeded_empty": 225, "failed_lsp_error": 67, "refused": 0, "one_final_disposition_ordinals": 5900}:
        raise AssertionError("INCOMING_COUNTS_DRIFT")
    if opr["exact_counts"] != {"attempted": 5900, "failed_lsp_error": 67, "refusals": 0, "succeeded_empty": 1455, "succeeded_non_empty": 4378}:
        raise AssertionError("OUTGOING_COUNTS_DRIFT")
    log.append("PASS DIRECTIONAL_COUNTS")


def assert_no_bad_keys(obj: Any, allowed_forbidden_path: tuple[str, ...] = ()) -> None:
    stack: list[tuple[tuple[str, ...], Any]] = [((), obj)]
    while stack:
        path, node = stack.pop()
        if isinstance(node, dict):
            for k, v in node.items():
                child = (*path, k)
                if k in BAD_KEY_ALIASES and not (len(child) >= 2 and child[-2] == "forbidden_synthesis"):
                    raise AssertionError(f"SEMANTIC_ALIAS_OR_FORBIDDEN_KEY {'.'.join(child)}")
                stack.append((child, v))
        elif isinstance(node, list):
            for item in node:
                stack.append((path, item))


def no_cross_direction_text(slot: str, obj: dict[str, Any]) -> None:
    text = json.dumps(obj, sort_keys=True)
    opposite = "outgoing" if slot == "incoming" else "incoming"
    bad = [opposite, opposite.upper(), f"CALLS_GO_GOPLS_{opposite.upper()}", f"CALLS_LSP_{opposite.upper()}"]
    if any(token in text for token in bad):
        raise AssertionError(f"CROSS_DIRECTION_REFERENCE_IN_{slot.upper()}")


def validate_envelope(slot: str, env: dict[str, Any]) -> None:
    if list(env.keys()) != ENVELOPE_KEYS:
        extra = [k for k in env if k not in ENVELOPE_KEYS]
        missing = [k for k in ENVELOPE_KEYS if k not in env]
        raise AssertionError(f"{slot.upper()}_UNKNOWN_OR_MISSING_KEYS extra={extra} missing={missing}")
    assert_no_bad_keys(env)
    if env["schema"] != "lsp-trace.adr0011.private-bidirectional-composition.direction-envelope.v2":
        raise AssertionError(f"{slot.upper()}_SCHEMA_DRIFT")
    if env["direction"] != slot:
        raise AssertionError("DIRECTION_SWAP_REFUSED")
    if env["authority"] != 0 or env["accepted"] is not False or env["production_admission"] != "REFUSED_NOT_REQUESTED_NOT_GRANTED" or env["production_refusal"] is not True:
        raise AssertionError(f"{slot.upper()}_AUTHORITY_COMPLETENESS_ESCALATION")
    if env["target_set_relative_completeness"]["status"] != "UNKNOWN" or env["frontier"] != "UNKNOWN":
        raise AssertionError(f"{slot.upper()}_COMPLETENESS_OR_FRONTIER_DRIFT")
    if env["raw_provider_bytes_policy"] != "NO_READ_NO_COPY_NO_REMARSHAL_REFERENCE_BY_PATH_HASH_ONLY":
        raise AssertionError(f"{slot.upper()}_RAW_POLICY_DRIFT")
    if env["ceiling"] != {"per_direction_raw_bytes": 33554432, "aggregate_live_bytes": 33554432, "combined_ceiling_admitted": False, "ceiling_change": False}:
        raise AssertionError(f"{slot.upper()}_CEILING_DRIFT")
    if env["opaque_custody"] is not True or env["calls_only"] is not True or env["immutable_reference_only"] is not True:
        raise AssertionError(f"{slot.upper()}_CUSTODY_OR_CALLS_ONLY_DRIFT")
    if env["peer_failure_effect"] != "PEER_UNCHANGED_NOT_ERASED_NOT_RELABELED":
        raise AssertionError(f"{slot.upper()}_PEER_EFFECT_DRIFT")
    if env["status"] == "SUCCESS":
        validate_success_payload(slot, env["payload"])
    elif env["status"] == "FAILURE":
        validate_failure_payload(slot, env["payload"])
    else:
        raise AssertionError(f"{slot.upper()}_STATUS_DRIFT")
    no_cross_direction_text(slot, env)


def validate_success_payload(slot: str, payload: dict[str, Any]) -> None:
    if list(payload.keys()) != SUCCESS_PAYLOAD_KEYS:
        extra = [k for k in payload if k not in SUCCESS_PAYLOAD_KEYS]
        missing = [k for k in SUCCESS_PAYLOAD_KEYS if k not in payload]
        raise AssertionError(f"{slot.upper()}_SUCCESS_UNKNOWN_OR_MISSING_KEYS extra={extra} missing={missing}")
    expected = success_payload(slot)
    if payload != expected:
        raise AssertionError(f"{slot.upper()}_SUCCESS_PAYLOAD_MISMATCH")


def validate_failure_payload(slot: str, payload: dict[str, Any]) -> None:
    if list(payload.keys()) != FAILURE_PAYLOAD_KEYS:
        extra = [k for k in payload if k not in FAILURE_PAYLOAD_KEYS]
        missing = [k for k in FAILURE_PAYLOAD_KEYS if k not in payload]
        raise AssertionError(f"{slot.upper()}_FAILURE_UNKNOWN_OR_MISSING_KEYS extra={extra} missing={missing}")
    if payload["schema"] != "lsp-trace.adr0011.private-bidirectional-composition.fixture-native-failure.v1" or payload["payload_type"] != "fixture_native_directional_failure":
        raise AssertionError(f"{slot.upper()}_FAILURE_SCHEMA_DRIFT")
    if payload["direction"] != slot or payload["failure_disposition"] != "FAILURE":
        raise AssertionError(f"{slot.upper()}_FAILURE_DIRECTION_OR_DISPOSITION_DRIFT")
    if not payload["failure_code"].startswith(slot.upper() + "_"):
        raise AssertionError(f"{slot.upper()}_CROSS_DIRECTION_FAILURE_CODE")
    if not payload["failure_stage"]:
        raise AssertionError(f"{slot.upper()}_FAILURE_STAGE_OR_MESSAGE_MISSING")
    if not isinstance(payload["message_digest"], str) or not MESSAGE_DIGEST_RE.fullmatch(payload["message_digest"]):
        raise AssertionError(f"{slot.upper()}_FAILURE_MESSAGE_DIGEST_FORMAT")
    ev = payload["evidence_ref"]
    if list(ev.keys()) != ["kind", "path", "selector", "raw_provider_bytes"] or ev["kind"] != "fixture-native-private-evidence-only" or ev["raw_provider_bytes"] != "NOT_APPLICABLE" or f"cases/{slot}_failure_valid" not in ev["selector"]:
        raise AssertionError(f"{slot.upper()}_FAILURE_EVIDENCE_MISSING_OR_CROSS_DIRECTION")
    for k in ("private_evidence_only",):
        if payload[k] is not True:
            raise AssertionError(f"{slot.upper()}_FAILURE_PRIVATE_EVIDENCE_DRIFT")
    for k in ("counts_claimed", "success_claims", "inverse_inference", "peer_evidence_used"):
        if payload[k] is not False:
            raise AssertionError(f"{slot.upper()}_FAILURE_FORBIDDEN_CLAIM")


def validate_artifact(artifact: dict[str, Any]) -> None:
    if list(artifact.keys()) != ["incoming", "outgoing"]:
        raise AssertionError("TOP_LEVEL_ORDER_OR_KEYS")
    validate_envelope("incoming", artifact["incoming"])
    validate_envelope("outgoing", artifact["outgoing"])


def composed_case(name: str, incoming: dict[str, Any], outgoing: dict[str, Any]) -> dict[str, Any]:
    artifact = compose(incoming, outgoing)
    validate_artifact(artifact)
    return {
        "name": name,
        "expect": "PASS",
        "composition_sha256": canonical_sha(artifact),
        "incoming_subtree_sha256": canonical_sha(artifact["incoming"]),
        "outgoing_subtree_sha256": canonical_sha(artifact["outgoing"]),
        "incoming_status": artifact["incoming"]["status"],
        "outgoing_status": artifact["outgoing"]["status"],
    }


def build_fixture_cases() -> list[dict[str, Any]]:
    si = success_envelope("incoming")
    so = success_envelope("outgoing")
    fi = failure_envelope("incoming")
    fo = failure_envelope("outgoing")
    return [
        composed_case("successful_pair", si, so),
        composed_case("incoming_failure_with_outgoing_success", fi, so),
        composed_case("outgoing_failure_with_incoming_success", si, fo),
        composed_case("both_failures", fi, fo),
    ]


def fixtures() -> dict[str, Any]:
    return {
        "schema": "lsp-trace.adr0011.private-bidirectional-composition.fixtures.v2",
        "envelope_subtree_preservation": "canonical subtree sha256 is computed before and after composition and must match",
        "valid_cases": build_fixture_cases(),
        "invalid_cases": [
            "direction_swap_refusal", "profile_pin_drift", "frontier_mutation", "shared_completeness_injection", "shared_authority_injection", "ledger_merge", "inverse_synthesis", "dedup_union_graph_claim", "raw_copy_or_remarshal", "ceiling_changes", "calls_only_drift", "public_default_on_exposure", "cross_direction_reference", "unknown_key_or_semantic_alias", "failure_relabel_direction", "failure_erase_peer", "failure_copy_peer_evidence", "failure_to_success_promotion", "cross_direction_failure_code_ref", "merged_failure", "missing_failure_evidence", "failure_authority_completeness_escalation", "failure_digest_uppercase", "failure_digest_short", "failure_digest_long", "failure_digest_nonhex", "failure_digest_missing_prefix", "failure_digest_extra_suffix", "failure_digest_whitespace", "failure_digest_nonstring",
        ],
    }


def oracles(success_artifact_sha: str) -> dict[str, Any]:
    return {
        "schema": "lsp-trace.adr0011.private-bidirectional-composition.oracles.v2",
        "success_artifact_sha256": success_artifact_sha,
        "valid_failure_case_hashes": {case["name"]: case["composition_sha256"] for case in build_fixture_cases() if "failure" in case["name"] or case["name"] == "both_failures"},
        "canonical_serialization": "UTF-8 JSON, incoming key before outgoing key, duplicate-free metadata/envelope object, one trailing LF",
        "composition_state": {"authority": 0, "accepted": False, "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED", "private": True, "default_on": False, "producer_disposition": "ADR0011_PRIVATE_BIDIRECTIONAL_COMPOSITION_AUTHORED_NOT_GO", "producer_may_issue_go": False, "completeness_pair": {"incoming": "UNKNOWN", "outgoing": "UNKNOWN"}, "frontier_pair": {"incoming": "UNKNOWN", "outgoing": "UNKNOWN"}},
        "failure_semantics": "valid directional FAILURE envelopes are retained as failures; invalid composition inputs are rejected, not represented",
        "raw_provider_bytes": "NEVER_READ_NEVER_COPIED_NEVER_REMARSHALED_NEVER_RECONSTRUCTED",
        "no_combined_ceiling_admission": True,
    }


def input_pins() -> dict[str, Any]:
    return {"schema": "lsp-trace.adr0011.private-bidirectional-composition.input-pins.v2", "dependencies": DEPENDENCIES, "evidence_refs": EVIDENCE_REFS, "counts": COUNTS, "raw_byte_policy": "metadata-only composition; raw provider evidence is referenced by path/hash only"}


def manifest(success_artifact_sha: str) -> dict[str, Any]:
    return {
        "schema": "lsp-trace.adr0011.private-bidirectional-composition.package-manifest.v2",
        "package": "docs/qualification/adr0011-private-bidirectional-composition-qualification-v1",
        "private": True,
        "default_on": False,
        "producer_disposition": "ADR0011_PRIVATE_BIDIRECTIONAL_COMPOSITION_AUTHORED_NOT_GO",
        "producer_may_issue_go": False,
        "authority": 0,
        "accepted": False,
        "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED",
        "composition_artifact_sha256": success_artifact_sha,
        "representation": "ordered top-level {incoming,outgoing}; each value is a success or fixture-native failure directional envelope",
        "stricter_contract_inspection": {"inspected": "docs/qualification/adr0011-adr0007-private-composition-contract.effe3f74.md", "applicable": False, "reason": "ADR0011-to-ADR0007 composition contract is explicitly out of scope for this authorization; no accepted generic/private bidirectional incoming/outgoing contract with stricter representation was found in small qualification files"},
        "dependencies": DEPENDENCIES,
        "files": [
            {"path": "README.md", "role": "overview and refusal boundary"}, {"path": "CONTRACT.md", "role": "private composition contract/spec"}, {"path": "SCHEMA.md", "role": "envelope schema and sum type"}, {"path": "INPUT_PINS.json", "role": "four accepted dependency pins and evidence references"}, {"path": "COMPOSITION_ARTIFACT.json", "role": "byte-stable canonical success composition artifact"}, {"path": "COMPOSITION_ARTIFACT.sha256", "role": "success artifact checksum"}, {"path": "fixtures.json", "role": "positive, valid-failure, and adversarial fixture inventory"}, {"path": "ORACLES.json", "role": "expected package oracles and failure hashes"}, {"path": "RECURSIVE_PRESUMPTION_LEDGER.md", "role": "presumption/revision/dependency ledger"}, {"path": "ROLE_CLOSURE.md", "role": "role and non-role closure"}, {"path": "REVIEW_REQUEST.md", "role": "independent review request"}, {"path": "verify_bidirectional_composition.py", "role": "independent verifier and deterministic composer"}, {"path": "VERIFICATION.log", "role": "verification output"}, {"path": "TAMPER_TESTS.log", "role": "tamper suite output"}, {"path": "MANIFEST.json", "role": "package manifest"}, {"path": "CHECKSUMS.sha256", "role": "checksum manifest excluding self"},
        ],
    }


def run_tamper_suite() -> list[str]:
    si = success_envelope("incoming"); so = success_envelope("outgoing"); fi = failure_envelope("incoming"); fo = failure_envelope("outgoing")
    valid = [("successful_pair", si, so), ("incoming_failure_with_outgoing_success", fi, so), ("outgoing_failure_with_incoming_success", si, fo), ("both_failures", fi, fo)]
    lines: list[str] = []
    baseline_in_success_sha = canonical_sha(si); baseline_out_success_sha = canonical_sha(so); baseline_in_failure_sha = canonical_sha(fi); baseline_out_failure_sha = canonical_sha(fo)
    for name, inc, out in valid:
        art = compose(inc, out)
        validate_artifact(art)
        if canonical_sha(art["incoming"]) != canonical_sha(inc) or canonical_sha(art["outgoing"]) != canonical_sha(out):
            raise AssertionError(f"SUBTREE_PRESERVATION {name}")
        lines.append(f"PASS {name} -> PASS composition={canonical_sha(art)} incoming_subtree={canonical_sha(art['incoming'])} outgoing_subtree={canonical_sha(art['outgoing'])}")
    if canonical_sha(compose(fi, so)["outgoing"]) != baseline_out_success_sha:
        raise AssertionError("INCOMING_FAILURE_CHANGED_OUTGOING")
    if canonical_sha(compose(si, fo)["incoming"]) != baseline_in_success_sha:
        raise AssertionError("OUTGOING_FAILURE_CHANGED_INCOMING")
    if canonical_sha(compose(fi, fo)["incoming"]) != baseline_in_failure_sha or canonical_sha(compose(fi, fo)["outgoing"]) != baseline_out_failure_sha:
        raise AssertionError("BOTH_FAILURE_SUBTREE_DRIFT")
    lines.append("PASS PEER_SUBTREE_PRESERVATION")

    cases: list[tuple[str, dict[str, Any], dict[str, Any], str]] = []
    m = copy.deepcopy(fi); m["direction"] = "outgoing"; cases.append(("failure_relabel_direction", m, so, "DIRECTION_SWAP_REFUSED"))
    cases.append(("failure_erase_peer", fi, {}, "OUTGOING_UNKNOWN_OR_MISSING_KEYS"))
    m = copy.deepcopy(fi); m["payload"]["peer_evidence_copy"] = EVIDENCE_REFS["outgoing"]["failures_ref"]; cases.append(("failure_copy_peer_evidence", m, so, "SEMANTIC_ALIAS_OR_FORBIDDEN_KEY"))
    m = copy.deepcopy(fi); m["status"] = "SUCCESS"; cases.append(("failure_to_success_promotion", m, so, "INCOMING_SUCCESS_UNKNOWN_OR_MISSING_KEYS"))
    m = copy.deepcopy(fi); m["payload"]["failure_code"] = "OUTGOING_BORROWED_FAILURE"; cases.append(("cross_direction_failure_code_ref", m, so, "INCOMING_CROSS_DIRECTION_FAILURE_CODE"))
    m = copy.deepcopy(fi); m["merged_failure"] = True; cases.append(("merged_failure", m, so, "INCOMING_UNKNOWN_OR_MISSING_KEYS"))
    m = copy.deepcopy(fi); del m["payload"]["evidence_ref"]; cases.append(("missing_failure_evidence", m, so, "INCOMING_FAILURE_UNKNOWN_OR_MISSING_KEYS"))
    m = copy.deepcopy(fi); m["authority"] = 1; cases.append(("failure_authority_completeness_escalation", m, so, "INCOMING_AUTHORITY_COMPLETENESS_ESCALATION"))
    digest_mutations = [
        ("failure_digest_uppercase", "sha256:" + "A" * 64),
        ("failure_digest_short", "sha256:" + "a" * 63),
        ("failure_digest_long", "sha256:" + "a" * 65),
        ("failure_digest_nonhex", "sha256:" + "g" * 64),
        ("failure_digest_missing_prefix", "a" * 64),
        ("failure_digest_extra_suffix", "sha256:" + "a" * 64 + "x"),
        ("failure_digest_whitespace", "sha256:" + "a" * 64 + " "),
        ("failure_digest_nonstring", 123),
    ]
    for name, bad_digest in digest_mutations:
        m = copy.deepcopy(fi)
        m["payload"]["message_digest"] = bad_digest
        cases.append((name, m, so, "INCOMING_FAILURE_MESSAGE_DIGEST_FORMAT"))
    m = {"outgoing": so, "incoming": si};
    try:
        validate_artifact(m)
        got = "PASS"
    except AssertionError as e:
        got = str(e).split()[0]
    if got != "TOP_LEVEL_ORDER_OR_KEYS":
        raise AssertionError(f"TAMPER direction_swap_refusal expected TOP_LEVEL_ORDER_OR_KEYS got {got}")
    lines.append("PASS direction_swap_refusal -> TOP_LEVEL_ORDER_OR_KEYS")
    more = [
        ("profile_pin_drift", copy.deepcopy(si), so, "INCOMING_SUCCESS_PAYLOAD_MISMATCH", lambda x: x["payload"].update({"package_checksums_sha256": "0" * 64})),
        ("frontier_mutation", copy.deepcopy(si), so, "INCOMING_COMPLETENESS_OR_FRONTIER_DRIFT", lambda x: x.update({"frontier": "KNOWN"})),
        ("shared_completeness_injection", copy.deepcopy(si), so, "INCOMING_UNKNOWN_OR_MISSING_KEYS", lambda x: x.update({"shared_completeness": "UNKNOWN"})),
        ("shared_authority_injection", si, copy.deepcopy(so), "OUTGOING_UNKNOWN_OR_MISSING_KEYS", lambda x: x.update({"shared_authority": 0})),
        ("ledger_merge", copy.deepcopy(si), so, "INCOMING_UNKNOWN_OR_MISSING_KEYS", lambda x: x.update({"merged_ledger": True})),
        ("inverse_synthesis", si, copy.deepcopy(so), "OUTGOING_UNKNOWN_OR_MISSING_KEYS", lambda x: x.update({"inverse_synthesis": True})),
        ("dedup_union_graph_claim", copy.deepcopy(si), so, "INCOMING_UNKNOWN_OR_MISSING_KEYS", lambda x: x.update({"dedup": True})),
        ("raw_copy_or_remarshal", si, copy.deepcopy(so), "OUTGOING_UNKNOWN_OR_MISSING_KEYS", lambda x: x.update({"raw_bytes": "e30="})),
        ("ceiling_changes", copy.deepcopy(si), so, "INCOMING_CEILING_DRIFT", lambda x: x["ceiling"].update({"per_direction_raw_bytes": 67108864})),
        ("calls_only_drift", si, copy.deepcopy(so), "OUTGOING_CUSTODY_OR_CALLS_ONLY_DRIFT", lambda x: x.update({"calls_only": False})),
        ("public_default_on_exposure", copy.deepcopy(si), so, "INCOMING_UNKNOWN_OR_MISSING_KEYS", lambda x: x.update({"default_on": True})),
        ("cross_direction_reference", copy.deepcopy(si), so, "INCOMING_SUCCESS_PAYLOAD_MISMATCH", lambda x: x["payload"].update({"raw_provenance_ref": {"bad": "outgoing"}})),
        ("unknown_key_or_semantic_alias", si, copy.deepcopy(so), "OUTGOING_UNKNOWN_OR_MISSING_KEYS", lambda x: x.update({"combinedAuthority": 0})),
    ]
    for name, inc, out, expected, mut in more:
        # Apply mutation to the copy-bearing side explicitly.
        if name in {"shared_authority_injection", "inverse_synthesis", "raw_copy_or_remarshal", "calls_only_drift", "unknown_key_or_semantic_alias"}:
            mut(out)
        else:
            mut(inc)
        cases.append((name, inc, out, expected))
    for name, inc, out, expected in cases:
        try:
            compose(inc, out)
            got = "PASS"
        except AssertionError as e:
            got = str(e).split()[0]
        if got != expected:
            raise AssertionError(f"TAMPER {name} expected {expected} got {got}")
        lines.append(f"PASS {name} -> {got}")
    return lines


def write_json(path: Path, obj: Any) -> str:
    data = canonical_bytes(obj)
    path.write_bytes(data)
    return sha256_bytes(data)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--write-artifact", action="store_true")
    ap.add_argument("--write-logs", action="store_true")
    args = ap.parse_args()
    log: list[str] = []
    verify_dependency_pins(log)
    verify_dependency_fields(log)
    artifact = expected_artifact()
    validate_artifact(artifact)
    a1 = canonical_bytes(artifact); a2 = canonical_bytes(expected_artifact())
    if a1 != a2 or sha256_bytes(a1) != sha256_bytes(a2):
        raise AssertionError("BYTE_STABLE_IDEMPOTENCE")
    artifact_sha = sha256_bytes(a1)
    log.append(f"PASS BYTE_STABLE_IDEMPOTENCE {artifact_sha}")
    log.append("PASS ORDERED_REPRESENTATION incoming,outgoing direction_envelopes")
    log.append("PASS AUTHORITY0_ACCEPTEDFALSE_PRODUCTION_REFUSED")
    log.append("PASS COMPLETENESS_PAIR_UNKNOWN_FRONTIER_PAIR_UNKNOWN")
    log.append("PASS RAW_PROVIDER_BYTES_NOT_READ_NOT_COPIED_NOT_REMARSHALED")
    log.append("PASS PER_DIRECTION_32MIB_NO_COMBINED_CEILING")
    tamper_lines = run_tamper_suite()
    log.append("PASS VALID_FAILURE_ENVELOPES_RETAINED_AS_DIRECTIONAL_RESULTS")
    log.append("ADR0011_PRIVATE_BIDIRECTIONAL_COMPOSITION_AUTHORED_NOT_GO")
    if args.write_artifact:
        write_json(PKG / "INPUT_PINS.json", input_pins())
        write_json(PKG / "COMPOSITION_ARTIFACT.json", artifact)
        (PKG / "COMPOSITION_ARTIFACT.sha256").write_text(f"{artifact_sha}  COMPOSITION_ARTIFACT.json\n", encoding="utf-8")
        write_json(PKG / "fixtures.json", fixtures())
        write_json(PKG / "ORACLES.json", oracles(artifact_sha))
        write_json(PKG / "MANIFEST.json", manifest(artifact_sha))
    if args.write_logs:
        (PKG / "VERIFICATION.log").write_text("\n".join(log) + "\n", encoding="utf-8")
        (PKG / "TAMPER_TESTS.log").write_text("\n".join(tamper_lines) + "\n", encoding="utf-8")
    print("\n".join(log))
    print("\n".join(tamper_lines))
    return 0


if __name__ == "__main__":
    sys.exit(main())
