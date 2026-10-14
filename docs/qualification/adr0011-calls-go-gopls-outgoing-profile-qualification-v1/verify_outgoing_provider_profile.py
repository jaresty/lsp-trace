#!/usr/bin/env python3
import argparse
import collections
import hashlib
import json
import pathlib
import shutil
import sys
import tempfile

SEMANTIC_STATEMENT = "caller queried A -> callee B; to endpoint identity/display only; fromRanges sole occurrence source; null/empty/malformed from generic package"
OCCURRENCE_SOURCE_POLICY = "fromRanges only; to.range and to.selectionRange are endpoint identity/display only; D/R, source text, documentSymbol, incoming reverse evidence, provider heuristic, and raw endpoint display spans are not CALLS occurrence sources"
MAXIMA_QUALIFIER = "measured in the historical and fresh outgoing exact 5,900-target runs only; not a universal provider ceiling"
PROFILE = "CALLS_GO_GOPLS_OUTGOING_EXACT_V1"
GENERIC = "CALLS_LSP_OUTGOING_EXACT_V1"
PRODUCER = "CALLS_GO_GOPLS_OUTGOING_PROFILE_QUALIFICATION_AUTHORED_NOT_GO"
PRODUCTION_REFUSAL = "REFUSED_NOT_REQUESTED_NOT_GRANTED"
BASE_COMMIT = "17057a8c53af8e8fe66be9c37788bbf59d10c5cd"
EVIDENCE_HEAD = "03359dc6344ee71c828cc3a22af42af6683333ae"
GENERIC_CHECKSUMS_SHA256 = "8c0efb403147ff70ad4def97809c65ec026c4984fdd8c8b5f7e1f9d0e31de6db"
ADDENDUM_CHECKSUMS_SHA256 = "3ed97c1031986186e5da49e1488f46554314c2bf945f02bca73217146d6abeca"
FINAL_CHECKSUMS_SHA256 = "b01bcfd5018432eed725af79a41ce049ac3ba2941d7d2f3a40077367047bb93b"
FINAL_SEAL_SHA256 = "a511c0f2ff435a30ba963f7b70671880959cf234bf37c7dc7a195b7886ba795c"
EXPECTED_ARTIFACT_COUNT = 53640
EXPECTED_COUNTS = {"FAILED_LSP_ERROR": 67, "SUCCEEDED_EMPTY": 1455, "SUCCEEDED_NON_EMPTY": 4378}
EXPECTED_EXACT_COUNTS = {"attempted": 5900, "succeeded_non_empty": 4378, "succeeded_empty": 1455, "failed_lsp_error": 67, "refusals": 0}
EXPECTED_HISTORICAL_RAW_AGGREGATE = "9cdbd26f6cd60887d5414b10c0f7d1fd67d2a185c83cb9aad44aab08c742427b"
EXPECTED_FRESH_INVENTORY_DIGEST = "967acce92cfac58e051f2f110aa274b3a151c79c31ab48453712b18867be0825"
STATIC_CHECKSUM_EXCLUDES = {"CHECKSUMS.sha256", "VERIFICATION.log", "TAMPER_TESTS.log"}
FORBIDDEN_EFFECT_KEYS = {
    "delegation", "commit", "incoming_authority_modification", "incoming_profile_use", "incoming_ledger_dependency",
    "incoming_raw_dependency", "incoming_run_dependency", "composition_change", "bidirectional_composition",
    "adr0011_to_adr0007_composition", "stage8_change", "public_surface_change", "runtime_change", "api_change",
    "schema_change", "cli_change", "mcp_change", "custody_owner_change", "ceiling_change", "integration", "push",
    "publication", "release", "deployment", "production_admission",
}
MANIFEST_KEYS = {"accepted", "accepted_prerequisites_pinned", "authority", "binds_to_generic_profile_id", "completeness", "direction", "files", "forbidden_effects", "forbidden_effects_semantics", "fresh_final_pins", "frontier", "incoming_evidence_or_authority_used", "incoming_profile_use", "lineage", "maxima_qualifier", "occurrence_source_policy", "outgoing_provider_go", "package", "producer_disposition", "production_admission", "production_go", "profile_id", "provider_go", "schema", "semantic_statement", "source_pins"}
RESULT_KEYS = {"accepted", "authority", "binds_to_generic_profile_id", "blockers", "comparison_result", "completeness", "direction", "exact_counts", "forbidden_effects", "fresh_final_pins", "frontier", "incoming_evidence_or_authority_used", "incoming_profile_use", "maxima_qualifier", "occurrence_source_policy", "outgoing_provider_go", "producer_disposition", "production_admission", "production_go", "profile_id", "provider_go", "qualification_basis", "required_review", "schema", "semantic_statement", "source_pins", "summary"}
ROLE_MATRIX_KEYS = {"accepted", "authority", "binds_to_generic_profile_id", "bounds", "completeness", "custody_roles", "custody_roles_closed_from", "direction", "forbidden_effects", "frontier", "incoming_evidence_or_authority_used", "incoming_profile_use", "non_roles", "occurrence_source_policy", "outgoing_provider_go", "producer_disposition", "production_admission", "production_go", "profile_id", "provider_binding_roles", "provider_go", "schema", "semantic_statement", "source_pins"}
ROLE_KEYS = {"accepted", "authority", "completeness", "direction", "forbidden_effects", "frontier", "incoming_evidence_or_authority_used", "incoming_evidence_used", "incoming_profile_use", "mandatory", "occurrence_source_policy", "outgoing_provider_go", "production_admission", "production_go", "provider_go", "role", "semantic_statement", "source", "source_run_dir", "status"}
COMPARISON_KEYS = {"accepted", "authority", "binds_to_generic_profile_id", "completeness", "differences", "direction", "equality", "expected_exact_values", "fresh_run", "frontier", "historical_run", "incoming_evidence_or_authority_used", "incoming_profile_use", "maxima_witnesses", "occurrence_source_policy", "outgoing_provider_go", "producer_disposition", "production_admission", "production_go", "profile_id", "provider_go", "schema", "semantic_difference_indicators", "semantic_statement", "source_pins"}
RUN_KEYS = {"artifact_inventory_digest", "failure_identity_digest", "final_checksums_sha256", "final_counts", "final_seal_sha256", "final_summary_sha256", "ledger_sha256", "maxima", "path", "raw_aggregate_digest", "raw_outgoing_file_count", "sealed_manifest_sha256", "target_count", "target_order_digest", "terminal_event_digest_excluding_timestamps"}
MAXIMA_KEYS = {"actual_response_count", "max_fromRanges_on_one_member", "max_outgoing_request_bytes", "max_outgoing_response_bytes", "max_returned_to_members_per_response", "max_total_fromRanges_in_one_response", "response_shape_counts", "total_fromRanges_occurrences_measured"}
NON_ROLES_KEYS = {"ceiling_change", "composition_authority", "custody_owner_change", "incoming_authority", "incoming_profile_authority", "production_admission", "public_surface_change", "runtime_code_change"}
EXPECTED_KNOWN_NONSEMANTIC_DIFFERENCES = ["ledger timestamps", "ledger SHA", "raw notification counts", "raw sidecar absolute paths", "raw file inventories/checksums"]

class VerifyError(Exception):
    pass

def fail(msg):
    raise VerifyError(msg)

def require(cond, msg):
    if not cond:
        fail(msg)

def sha(path):
    return hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest()

def sha_text(text):
    return hashlib.sha256(text.encode()).hexdigest()

def canon(obj):
    return json.dumps(obj, sort_keys=True, separators=(",", ":"))

def duplicate_rejecting_pairs(path_label):
    def hook(pairs):
        seen = set()
        out = {}
        for key, value in pairs:
            if key in seen:
                fail(f"{path_label} duplicate JSON key {key}")
            seen.add(key)
            if key != "semantic_statement" and "semantic_statement" in key:
                fail(f"{path_label} unrecognized semantic-bearing duplicate key {key}")
            if key != "occurrence_source_policy" and "occurrence_source_policy" in key:
                fail(f"{path_label} unrecognized occurrence-policy duplicate key {key}")
            out[key] = value
        return out
    return hook

def load(path):
    path = pathlib.Path(path)
    return json.loads(path.read_text(), object_pairs_hook=duplicate_rejecting_pairs(path.name))

def require_keys(obj, allowed, label):
    keys = set(obj.keys())
    require(keys == allowed, f"{label} keyset mismatch: extra={sorted(keys-allowed)} missing={sorted(allowed-keys)}")

def walk(obj, path="$"):
    if isinstance(obj, dict):
        for key, value in obj.items():
            yield path + "." + str(key), key, value
            yield from walk(value, path + "." + str(key))
    elif isinstance(obj, list):
        for index, value in enumerate(obj):
            yield from walk(value, path + f"[{index}]")

def common_object_checks(label, obj):
    require(obj.get("profile_id") == PROFILE, f"{label} profile mismatch")
    require(obj.get("binds_to_generic_profile_id") == GENERIC, f"{label} generic profile mismatch")
    require(obj.get("producer_disposition") == PRODUCER, f"{label} producer disposition mismatch")
    require(obj.get("semantic_statement") == SEMANTIC_STATEMENT, f"{label} semantic statement mismatch")
    require(obj.get("occurrence_source_policy") == OCCURRENCE_SOURCE_POLICY, f"{label} occurrence source policy mismatch")
    require(obj.get("direction") == "outgoing", f"{label} direction is not outgoing")
    require(type(obj.get("authority")) is int and obj.get("authority") == 0, f"{label} authority must be numeric 0")
    require(type(obj.get("accepted")) is bool and obj.get("accepted") is False, f"{label} accepted must be false bool")
    require(obj.get("production_admission") == PRODUCTION_REFUSAL, f"{label} production refusal mismatch")
    require(obj.get("completeness") == "UNKNOWN", f"{label} completeness must be UNKNOWN")
    require(obj.get("frontier") == "UNKNOWN", f"{label} frontier must be UNKNOWN")
    require(obj.get("provider_go") is False, f"{label} provider_go must be false")
    require(obj.get("outgoing_provider_go") is False, f"{label} outgoing_provider_go must be false")
    require(obj.get("production_go") is False, f"{label} production_go must be false")

def recursive_boundary_checks(label, obj):
    for path, key, value in walk(obj):
        if key == "authority":
            require(type(value) is int and value == 0, f"{label} {path} authority must be numeric 0")
        if key == "accepted":
            require(type(value) is bool and value is False, f"{label} {path} accepted must be false bool")
        if key in {"provider_go", "outgoing_provider_go", "production_go", "profile_go"}:
            require(value is False, f"{label} {path} must be false")
        if key == "production_admission" and ".forbidden_effects." not in path:
            require(value == PRODUCTION_REFUSAL or value is False, f"{label} {path} production refusal mismatch")
        if key == "direction":
            require(value == "outgoing", f"{label} {path} direction isolation failed")
        if key in {"completeness", "frontier"}:
            require(value == "UNKNOWN", f"{label} {path} must be UNKNOWN")
        if key == "incoming_evidence_or_authority_used":
            require(value is False, f"{label} {path} incoming evidence/authority must be false")
        if key == "incoming_profile_use":
            require(value == "FORBIDDEN" or value is False, f"{label} {path} incoming_profile_use must be forbidden")
        if key in {"incoming_ledger_dependency", "incoming_raw_dependency", "incoming_run_dependency", "composition_change", "bidirectional_composition", "adr0011_to_adr0007_composition", "stage8_change", "ceiling_change", "custody_owner_change", "runtime_change", "public_surface_change", "api_change", "schema_change", "cli_change", "mcp_change", "integration", "push", "publication", "release", "deployment"}:
            require(value is False, f"{label} {path} forbidden effect must be false")
        if key == "semantic_statement":
            require(value == SEMANTIC_STATEMENT, f"{label} {path} semantic statement mismatch")
        if key == "occurrence_source_policy":
            require(value == OCCURRENCE_SOURCE_POLICY, f"{label} {path} occurrence policy mismatch")
        lowered = str(value).lower() if isinstance(value, str) else ""
        if key in {"path", "source", "source_run_dir", "run_dir"} or key.endswith("_dependency"):
            require("incoming" not in lowered, f"{label} {path} must not depend on incoming artifact")

def verify_forbidden_effects(obj, label):
    effects = obj.get("forbidden_effects")
    require(isinstance(effects, dict), f"{label} forbidden_effects missing")
    require(set(effects.keys()) == FORBIDDEN_EFFECT_KEYS, f"{label} forbidden_effects keyset mismatch")
    require(all(value is False for value in effects.values()), f"{label} all forbidden effects must be false")

def verify_source_pins(pins, label):
    require(set(pins.keys()) == {"generic_outgoing_semantics", "outgoing_custody_addendum", "fresh_outgoing_evidence"}, f"{label} source pin keyset mismatch")
    generic = pins["generic_outgoing_semantics"]
    require(generic["path"] == "docs/qualification/adr0011-calls-lsp-outgoing-exact-v1-generic-semantics-v1/CHECKSUMS.sha256", f"{label} generic path mismatch")
    require(generic["sha256"] == GENERIC_CHECKSUMS_SHA256 and generic["commit"] == BASE_COMMIT and generic["profile_id"] == GENERIC, f"{label} generic pin mismatch")
    add = pins["outgoing_custody_addendum"]
    require(add["path"] == "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/CHECKSUMS.sha256", f"{label} addendum path mismatch")
    require(add["sha256"] == ADDENDUM_CHECKSUMS_SHA256 and add["commit"] == EVIDENCE_HEAD, f"{label} addendum pin mismatch")
    fresh = pins["fresh_outgoing_evidence"]
    require(fresh["head"] == EVIDENCE_HEAD and fresh["run_id"] == "outgoing-custody-correction-v1", f"{label} fresh evidence head/run mismatch")
    require(fresh["final_checksums_sha256"] == FINAL_CHECKSUMS_SHA256 and fresh["final_seal_sha256"] == FINAL_SEAL_SHA256 and fresh["artifact_count"] == EXPECTED_ARTIFACT_COUNT, f"{label} fresh evidence pins mismatch")
    recursive_boundary_checks(label + ".source_pins", pins)

def expected_accepted_prerequisites():
    return [
        {"name": "generic_outgoing_semantics", "path": "docs/qualification/adr0011-calls-lsp-outgoing-exact-v1-generic-semantics-v1/CHECKSUMS.sha256", "sha256": GENERIC_CHECKSUMS_SHA256, "commit": BASE_COMMIT, "profile_id": GENERIC, "status": "PINNED_ACCEPTED_PREREQUISITE_AUTHORED_NOT_GO", "authority": 0, "accepted": False, "production_admission": PRODUCTION_REFUSAL},
        {"name": "outgoing_custody_addendum", "path": "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/CHECKSUMS.sha256", "sha256": ADDENDUM_CHECKSUMS_SHA256, "commit": EVIDENCE_HEAD, "status": "PINNED_ACCEPTED_PREREQUISITE_AUTHORED_NOT_GO", "authority": 0, "accepted": False, "production_admission": PRODUCTION_REFUSAL},
        {"name": "fresh_outgoing_evidence", "path": "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/runs/outgoing-custody-correction-v1/full-outgoing.final-run.seal.json", "sha256": FINAL_SEAL_SHA256, "head": EVIDENCE_HEAD, "run_id": "outgoing-custody-correction-v1", "final_checksums_sha256": FINAL_CHECKSUMS_SHA256, "artifact_count": EXPECTED_ARTIFACT_COUNT, "status": "PINNED_ACCEPTED_EVIDENCE_AUTHORED_NOT_GO", "authority": 0, "accepted": False, "production_admission": PRODUCTION_REFUSAL},
    ]

def expected_provider_binding_roles():
    return [
        {"role": "generic_outgoing_semantics_pinned", "source": "docs/qualification/adr0011-calls-lsp-outgoing-exact-v1-generic-semantics-v1/CHECKSUMS.sha256", "sha256": GENERIC_CHECKSUMS_SHA256, "status": "PINNED_TO_SOURCE_PINS_AND_CURRENT_FILE", "authority": 0, "accepted": False, "production_admission": PRODUCTION_REFUSAL},
        {"role": "fresh_outgoing_custody_final_seal_pinned", "source": "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/runs/outgoing-custody-correction-v1/full-outgoing.final-run.seal.json", "sha256": FINAL_SEAL_SHA256, "status": "PINNED_TO_SOURCE_PINS_AND_CURRENT_FILE", "authority": 0, "accepted": False, "production_admission": PRODUCTION_REFUSAL},
        {"role": "fresh_outgoing_final_checksums_pinned", "source": "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/runs/outgoing-custody-correction-v1/full-outgoing.final-run.checksums.sha256", "sha256": FINAL_CHECKSUMS_SHA256, "status": "PINNED_TO_SOURCE_PINS_AND_CURRENT_FILE", "authority": 0, "accepted": False, "production_admission": PRODUCTION_REFUSAL},
        {"role": "historical_outgoing_comparison_only", "source": "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-outgoing/FINAL_SUMMARY.json", "sha256": "e6c8798f57e020f4af3c8fc149acdc2d21a76039d427da241db62a5234858d46", "status": "COMPARISON_ONLY_NOT_CUSTODY_SOURCE", "authority": 0, "accepted": False, "production_admission": PRODUCTION_REFUSAL},
    ]


def ledger_events(path):
    return [json.loads(line) for line in pathlib.Path(path).read_text().splitlines() if line]

def terminal_events(events):
    return [event for event in events if event.get("disposition") != "STARTED"]

def final_counts(events):
    counts = collections.Counter(event.get("disposition") for event in terminal_events(events))
    return dict(sorted(counts.items()))

def target_order(events):
    return [event.get("target_id") for event in events if event.get("disposition") == "STARTED"]

def failure_identities(events):
    fields = ["ordinal", "target_id", "batch_index", "attempt", "direction", "disposition", "error", "authority", "production_admission", "schema"]
    return [{field: event.get(field) for field in fields} for event in terminal_events(events) if event.get("disposition") == "FAILED_LSP_ERROR"]

def terminal_canon(events):
    fields = ["ordinal", "target_id", "batch_index", "attempt", "direction", "disposition", "error", "authority", "production_admission", "schema", "result_count", "source_proof", "open_info"]
    return [{field: event.get(field) for field in fields if field in event} for event in terminal_events(events)]

def tsv_failure_identities(path):
    lines = pathlib.Path(path).read_text().splitlines()
    require(lines and lines[0].split("\t") == ["ordinal", "target_id", "batch_index", "attempt", "direction", "disposition", "error", "authority", "production_admission", "schema"], "failure identities header mismatch")
    out = []
    for line in lines[1:]:
        ordinal, target_id, batch_index, attempt, direction, disposition, error, authority, production_admission, schema = line.split("\t", 9)
        out.append({"ordinal": int(ordinal), "target_id": target_id, "batch_index": int(batch_index), "attempt": int(attempt), "direction": direction, "disposition": disposition, "error": error, "authority": int(authority), "production_admission": production_admission, "schema": schema})
    return out

def framed_obj(path):
    data = pathlib.Path(path).read_bytes()
    index = data.find(b"\r\n\r\n")
    require(index >= 0, f"missing LSP frame separator in {path}")
    return json.loads(data[index+4:])

def raw_actual_map(run_dir, label):
    out = {}
    notifications = 0
    for path in sorted((run_dir / "raw/outgoing").glob(f"*/{label}.*.bin")):
        ordinal = int(path.parent.name)
        obj = framed_obj(path)
        if label.endswith("response") and isinstance(obj, dict) and obj.get("method"):
            notifications += 1
            continue
        out[ordinal] = obj
    return out, notifications

def raw_file_inventory(run_dir):
    base = run_dir / "raw/outgoing"
    return {str(path.relative_to(base)): sha(path) for path in sorted(base.rglob("*")) if path.is_file()}

def response_maxima(run_dir):
    responses, _ = raw_actual_map(run_dir, "callHierarchy_outgoingCalls.response")
    counts = collections.Counter()
    max_members = (0, None, None)
    max_ranges_member = (0, None, None, None)
    max_total_ranges = (0, None, None)
    max_resp_bytes = (0, None, None)
    total_occurrences = 0
    for ordinal, obj in responses.items():
        backing = None
        for candidate in sorted((run_dir / "raw/outgoing" / f"{ordinal:05d}").glob("callHierarchy_outgoingCalls.response.*.bin")):
            if framed_obj(candidate) == obj:
                backing = candidate
                break
        require(backing is not None, f"missing backing response for ordinal {ordinal}")
        if backing.stat().st_size > max_resp_bytes[0]:
            max_resp_bytes = (backing.stat().st_size, ordinal, backing.name)
        if "error" in obj:
            counts["error"] += 1
            members = []
        else:
            result = obj.get("result")
            if result is None:
                counts["null"] += 1
                members = []
            elif isinstance(result, list):
                counts["array_empty" if not result else "array_nonempty"] += 1
                members = result
            else:
                counts["malformed"] += 1
                members = []
        if len(members) > max_members[0]:
            max_members = (len(members), ordinal, backing.name)
        total_ranges = 0
        for member_index, member in enumerate(members):
            ranges = member.get("fromRanges") if isinstance(member, dict) else None
            count = len(ranges) if isinstance(ranges, list) else -1
            if count >= 0:
                total_occurrences += count
                total_ranges += count
            if count > max_ranges_member[0]:
                max_ranges_member = (count, ordinal, backing.name, member_index)
        if total_ranges > max_total_ranges[0]:
            max_total_ranges = (total_ranges, ordinal, backing.name)
    max_req = (0, None, None)
    for path in sorted((run_dir / "raw/outgoing").glob("*/callHierarchy_outgoingCalls.request.*.bin")):
        ordinal = int(path.parent.name)
        if path.stat().st_size > max_req[0]:
            max_req = (path.stat().st_size, ordinal, path.name)
    return {
        "actual_response_count": len(responses),
        "response_shape_counts": dict(sorted(counts.items())),
        "max_returned_to_members_per_response": {"value": max_members[0], "ordinal": max_members[1], "file": max_members[2], "qualifier": MAXIMA_QUALIFIER},
        "max_fromRanges_on_one_member": {"value": max_ranges_member[0], "ordinal": max_ranges_member[1], "file": max_ranges_member[2], "member_ordinal": max_ranges_member[3], "qualifier": MAXIMA_QUALIFIER},
        "max_total_fromRanges_in_one_response": {"value": max_total_ranges[0], "ordinal": max_total_ranges[1], "file": max_total_ranges[2], "qualifier": MAXIMA_QUALIFIER},
        "max_outgoing_response_bytes": {"value": max_resp_bytes[0], "ordinal": max_resp_bytes[1], "file": max_resp_bytes[2], "qualifier": MAXIMA_QUALIFIER},
        "max_outgoing_request_bytes": {"value": max_req[0], "ordinal": max_req[1], "file": max_req[2], "qualifier": MAXIMA_QUALIFIER},
        "total_fromRanges_occurrences_measured": total_occurrences,
    }

def verify_maxima_object(maxima, label):
    require(set(maxima.keys()) == MAXIMA_KEYS, f"{label} maxima keyset mismatch")
    for key in ["max_returned_to_members_per_response", "max_fromRanges_on_one_member", "max_total_fromRanges_in_one_response", "max_outgoing_response_bytes", "max_outgoing_request_bytes"]:
        value = maxima[key]
        require(value.get("qualifier") == MAXIMA_QUALIFIER, f"{label}.{key} qualifier mismatch")
        require(type(value.get("value")) is int and value["value"] >= 0, f"{label}.{key} value must be nonnegative int")
        require(type(value.get("ordinal")) is int and value["ordinal"] >= 0, f"{label}.{key} ordinal witness missing")
        require(isinstance(value.get("file"), str) and value["file"], f"{label}.{key} file witness missing")
    require(maxima["response_shape_counts"] == {"array_empty": 1455, "array_nonempty": 4378}, f"{label} response shape counts mismatch")
    require(maxima["actual_response_count"] == 5833, f"{label} actual response count mismatch")
    require(maxima["total_fromRanges_occurrences_measured"] == 33619, f"{label} occurrence total mismatch")

def verify_run_block(run, expected, label):
    require(set(run.keys()) <= RUN_KEYS, f"{label} run keyset has extra keys {sorted(set(run)-RUN_KEYS)}")
    require(run["target_count"] == 5900, f"{label} target_count mismatch")
    require(run["final_counts"] == EXPECTED_COUNTS, f"{label} final_counts mismatch")
    require(run["target_order_digest"] == expected["target_order_digest"], f"{label} target order digest mismatch")
    require(run["terminal_event_digest_excluding_timestamps"] == expected["terminal_digest"], f"{label} terminal digest mismatch")
    require(run["failure_identity_digest"] == expected["failure_digest"], f"{label} failure digest mismatch")
    require(run["raw_outgoing_file_count"] == expected["raw_file_count"], f"{label} raw outgoing file count mismatch")
    verify_maxima_object(run["maxima"], label)

def verify(repo_root=None, root=None):
    pkg = pathlib.Path(root or pathlib.Path(__file__).resolve().parent).resolve()
    repo = pathlib.Path(repo_root).resolve() if repo_root else next(p for p in [pkg, *pkg.parents] if (p / "go.mod").exists())
    manifest = load(pkg / "MANIFEST.json")
    result = load(pkg / "RESULT.json")
    roles = load(pkg / "ROLE_MATRIX.json")
    comp = load(pkg / "RUN_COMPARISON.json")
    require_keys(manifest, MANIFEST_KEYS, "MANIFEST")
    require_keys(result, RESULT_KEYS, "RESULT")
    require_keys(roles, ROLE_MATRIX_KEYS, "ROLE_MATRIX")
    require_keys(comp, COMPARISON_KEYS, "RUN_COMPARISON")
    for label, obj in [("MANIFEST", manifest), ("RESULT", result), ("ROLE_MATRIX", roles), ("RUN_COMPARISON", comp)]:
        common_object_checks(label, obj)
        recursive_boundary_checks(label, obj)
        verify_source_pins(obj["source_pins"], label)
    for label, obj in [("MANIFEST", manifest), ("RESULT", result), ("ROLE_MATRIX", roles)]:
        verify_forbidden_effects(obj, label)
    require(manifest["accepted_prerequisites_pinned"] == expected_accepted_prerequisites(), "accepted prerequisites must exactly mirror canonical source pins")
    require(result["qualification_basis"]["incoming_evidence_or_authority_used"] is False, "result basis incoming flag mismatch")
    require(result["qualification_basis"]["incoming_profile_use"] == "FORBIDDEN", "result basis incoming profile use mismatch")
    require(result["exact_counts"] == EXPECTED_EXACT_COUNTS, "result exact counts mismatch")
    require(result["blockers"] == [], "result blockers must remain empty")
    require(set(result["comparison_result"].keys()) == {"failure_identities_equal", "historical_and_fresh_counts_equal", "known_nonsemantic_differences", "maxima_equal", "outgoing_request_objects_equal", "outgoing_response_objects_equal", "prepare_request_objects_equal", "semantic_outgoing_payload_difference_found", "target_order_equal", "terminal_event_identities_excluding_timestamps_equal"}, "result comparison_result keyset mismatch")
    require(result["comparison_result"]["known_nonsemantic_differences"] == EXPECTED_KNOWN_NONSEMANTIC_DIFFERENCES, "result known nonsemantic differences mismatch")
    require(result["comparison_result"]["semantic_outgoing_payload_difference_found"] is False, "result semantic difference flag mismatch")
    require(roles["custody_roles_closed_from"] == "fresh_outgoing_only", "custody roles source mismatch")
    require(set(roles["non_roles"].keys()) == NON_ROLES_KEYS and all(value is False for value in roles["non_roles"].values()), "ROLE_MATRIX non_roles must be exact canonical false set")
    require(roles["provider_binding_roles"] == expected_provider_binding_roles(), "provider_binding_roles must exactly mirror canonical pins/current files")
    require(set(roles["bounds"].keys()) == {"authority", "accepted", "completeness", "frontier", "production_admission", "profile_go", "provider_go", "outgoing_provider_go", "production_go", "maxima_qualifier"}, "bounds keyset mismatch")
    recursive_boundary_checks("ROLE_MATRIX.bounds", roles["bounds"])
    for index, role in enumerate(roles["custody_roles"]):
        require_keys(role, ROLE_KEYS, f"custody_roles[{index}]")
        recursive_boundary_checks(f"custody_roles[{index}]", role)
        verify_forbidden_effects(role, f"custody_roles[{index}]")
        require(role["source"] == "fresh_outgoing_only", f"custody_roles[{index}] source mismatch")
        require(role["incoming_evidence_used"] is False, f"custody_roles[{index}] incoming_evidence_used mismatch")
    seal_path = repo / "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/runs/outgoing-custody-correction-v1/full-outgoing.final-run.seal.json"
    sums_path = repo / "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/runs/outgoing-custody-correction-v1/full-outgoing.final-run.checksums.sha256"
    generic_path = repo / "docs/qualification/adr0011-calls-lsp-outgoing-exact-v1-generic-semantics-v1/CHECKSUMS.sha256"
    addendum_path = repo / "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/CHECKSUMS.sha256"
    historical_summary_path = repo / "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-outgoing/FINAL_SUMMARY.json"
    require(sha(generic_path) == GENERIC_CHECKSUMS_SHA256, "generic prerequisite checksum actual mismatch")
    require(sha(addendum_path) == ADDENDUM_CHECKSUMS_SHA256, "addendum prerequisite checksum actual mismatch")
    require(sha(historical_summary_path) == "e6c8798f57e020f4af3c8fc149acdc2d21a76039d427da241db62a5234858d46", "historical summary checksum actual mismatch")
    require(sha(seal_path) == FINAL_SEAL_SHA256, "fresh final seal actual sha mismatch")
    require(sha(sums_path) == FINAL_CHECKSUMS_SHA256, "fresh final checksums actual sha mismatch")
    seal = load(seal_path)
    require(seal["artifact_count"] == EXPECTED_ARTIFACT_COUNT, "seal artifact count mismatch")
    require(seal["artifact_inventory_digest"] == EXPECTED_FRESH_INVENTORY_DIGEST, "seal artifact inventory digest mismatch")
    require(seal["checksums_sha256"] == FINAL_CHECKSUMS_SHA256, "seal checksums pin mismatch")
    require(seal["profile_id"] == PROFILE and seal["binds_to_generic_profile_id"] == GENERIC, "seal profile binding mismatch")
    require(type(seal["authority"]) is int and seal["authority"] == 0, "seal authority must be numeric 0")
    require(seal["production_admission"] == PRODUCTION_REFUSAL, "seal production refusal mismatch")
    require(seal["final_counts"]["final_disposition_counts"] == EXPECTED_COUNTS, "seal final counts mismatch")
    require(seal["final_counts"]["one_final_disposition_ordinals"] == 5900 and seal["final_counts"]["ordinal_count"] == 5900, "seal ordinal counts mismatch")
    hist_dir = repo / "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-outgoing"
    fresh_dir = repo / "docs/qualification/adr0011-calls-go-gopls-outgoing-custody-rerun-addendum-v1/runs/outgoing-custody-correction-v1/full-outgoing"
    hist_events = ledger_events(hist_dir / "ledgers/outgoing.attempts.jsonl")
    fresh_events = ledger_events(fresh_dir / "ledgers/outgoing.attempts.jsonl")
    hist_failures = failure_identities(hist_events)
    fresh_failures = failure_identities(fresh_events)
    require(final_counts(hist_events) == final_counts(fresh_events) == EXPECTED_COUNTS, "ledger final counts mismatch")
    require(target_order(hist_events) == target_order(fresh_events), "target order mismatch")
    require(hist_failures == fresh_failures, "failure identities differ")
    require(tsv_failure_identities(pkg / "FAILURE_IDENTITIES.tsv") == fresh_failures, "failure identities TSV mismatch")
    hist_raw = raw_file_inventory(hist_dir)
    fresh_raw = raw_file_inventory(fresh_dir)
    expected = {
        "historical": {"target_order_digest": sha_text(canon(target_order(hist_events))), "terminal_digest": sha_text(canon(terminal_canon(hist_events))), "failure_digest": sha_text(canon(hist_failures)), "raw_file_count": len(hist_raw)},
        "fresh": {"target_order_digest": sha_text(canon(target_order(fresh_events))), "terminal_digest": sha_text(canon(terminal_canon(fresh_events))), "failure_digest": sha_text(canon(fresh_failures)), "raw_file_count": len(fresh_raw)},
    }
    verify_run_block(comp["historical_run"], expected["historical"], "historical_run")
    verify_run_block(comp["fresh_run"], expected["fresh"], "fresh_run")
    require(comp["historical_run"]["raw_aggregate_digest"] == EXPECTED_HISTORICAL_RAW_AGGREGATE, "historical raw aggregate mismatch")
    require(comp["fresh_run"]["artifact_inventory_digest"] == EXPECTED_FRESH_INVENTORY_DIGEST, "fresh artifact inventory mismatch")
    require(comp["fresh_run"]["final_checksums_sha256"] == FINAL_CHECKSUMS_SHA256 and comp["fresh_run"]["final_seal_sha256"] == FINAL_SEAL_SHA256, "fresh final pins mismatch")
    require(response_maxima(hist_dir) == comp["historical_run"]["maxima"], "historical maxima recompute mismatch")
    require(response_maxima(fresh_dir) == comp["fresh_run"]["maxima"], "fresh maxima recompute mismatch")
    prep_h, _ = raw_actual_map(hist_dir, "textDocument_prepareCallHierarchy.request")
    prep_f, _ = raw_actual_map(fresh_dir, "textDocument_prepareCallHierarchy.request")
    outreq_h, _ = raw_actual_map(hist_dir, "callHierarchy_outgoingCalls.request")
    outreq_f, _ = raw_actual_map(fresh_dir, "callHierarchy_outgoingCalls.request")
    outres_h, notif_h = raw_actual_map(hist_dir, "callHierarchy_outgoingCalls.response")
    outres_f, notif_f = raw_actual_map(fresh_dir, "callHierarchy_outgoingCalls.response")
    require(prep_h == prep_f and outreq_h == outreq_f and outres_h == outres_f, "semantic LSP request/response objects differ")
    equality = comp["equality"]
    for key in ["target_order_equal", "full_plan_file_sha256_equal", "terminal_event_identities_excluding_timestamps_equal", "failure_identities_equal", "prepare_request_objects_equal", "outgoing_request_objects_equal", "outgoing_response_objects_equal", "maxima_equal", "exact_counts_equal", "refusals_equal_zero"]:
        require(equality.get(key) is True, f"comparison equality {key} must be true")
    semdiff = comp["semantic_difference_indicators"]
    require(semdiff["semantic_outgoing_payload_difference_found"] is False, "semantic difference flag mismatch")
    require(semdiff["known_nonsemantic_differences"] == EXPECTED_KNOWN_NONSEMANTIC_DIFFERENCES, "known nonsemantic differences mismatch")
    require(result["comparison_result"]["known_nonsemantic_differences"] == semdiff["known_nonsemantic_differences"], "result/RUN_COMPARISON known differences mirror mismatch")
    differences = comp["differences"]
    require(differences["outgoing_response_notification_count_historical"] == notif_h and differences["outgoing_response_notification_count_fresh"] == notif_f, "notification counts mismatch")
    require(differences["raw_outgoing_same_relative_path_same_digest_count"] + differences["raw_outgoing_same_relative_path_different_digest_count"] == len(set(hist_raw) & set(fresh_raw)), "raw overlap accounting mismatch")
    exact = comp["expected_exact_values"]
    require(exact["target_count"] == 5900 and exact["final_counts"] == EXPECTED_COUNTS, "expected exact counts mismatch")
    require(exact["historical_raw_outgoing_file_count"] == len(hist_raw) and exact["fresh_raw_outgoing_file_count"] == len(fresh_raw), "expected raw file counts mismatch")
    require(exact["historical_target_order_digest"] == expected["historical"]["target_order_digest"] and exact["fresh_target_order_digest"] == expected["fresh"]["target_order_digest"], "expected target digest mismatch")
    require(exact["historical_terminal_event_digest_excluding_timestamps"] == expected["historical"]["terminal_digest"] and exact["fresh_terminal_event_digest_excluding_timestamps"] == expected["fresh"]["terminal_digest"], "expected terminal digest mismatch")
    require(exact["historical_failure_identity_digest"] == expected["historical"]["failure_digest"] and exact["fresh_failure_identity_digest"] == expected["fresh"]["failure_digest"], "expected failure digest mismatch")
    require(comp["maxima_witnesses"]["historical"] == comp["historical_run"]["maxima"] and comp["maxima_witnesses"]["fresh"] == comp["fresh_run"]["maxima"], "maxima witnesses mismatch")
    require(comp["maxima_witnesses"]["qualifier"] == MAXIMA_QUALIFIER, "maxima witness qualifier mismatch")
    checks = {}
    for line in (pkg / "CHECKSUMS.sha256").read_text().splitlines():
        digest, rel = line.split("  ", 1)
        checks[rel] = digest
    actual = {}
    for path in sorted(pkg.rglob("*")):
        if path.is_file():
            rel = str(path.relative_to(pkg))
            if rel not in STATIC_CHECKSUM_EXCLUDES:
                actual[rel] = sha(path)
    require(checks == actual, "package static checksum inventory mismatch")
    return True

def mutate_json(path, mutator):
    path = pathlib.Path(path)
    data = load(path)
    mutator(data)
    path.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n")

def append_duplicate_key(path, key, value):
    path = pathlib.Path(path)
    text = path.read_text()
    insert = f',\n  "{key}": {json.dumps(value)}\n'
    path.write_text(text[:-2] + insert + text[-2:])

def run_tamper_tests(pkg, repo):
    tests = []
    def add(name, mutator):
        tests.append((name, mutator))
    # Reviewer representative semantic/boundary mutations.
    add("manifest semantic statement mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d.__setitem__("semantic_statement", "mutated")))
    add("result semantic statement mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d.__setitem__("semantic_statement", "mutated")))
    add("role semantic statement mutation", lambda p: mutate_json(p/"ROLE_MATRIX.json", lambda d: d.__setitem__("semantic_statement", "mutated")))
    add("semantic statement duplicate alias", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d.__setitem__("semantic_statement_duplicate", SEMANTIC_STATEMENT)))
    add("occurrence source policy duplicate alias", lambda p: mutate_json(p/"RESULT.json", lambda d: d.__setitem__("occurrence_source_policy_duplicate", OCCURRENCE_SOURCE_POLICY)))
    add("json duplicate semantic key", lambda p: append_duplicate_key(p/"MANIFEST.json", "semantic_statement", SEMANTIC_STATEMENT))
    add("occurrence source policy mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d.__setitem__("occurrence_source_policy", "to.range is an occurrence")))
    add("authority string mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d.__setitem__("authority", "0")))
    add("nested authority nonzero mutation", lambda p: mutate_json(p/"ROLE_MATRIX.json", lambda d: d["custody_roles"][0].__setitem__("authority", 1)))
    add("accepted true mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d.__setitem__("accepted", True)))
    add("provider go true mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d.__setitem__("provider_go", True)))
    add("outgoing provider go true mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d.__setitem__("outgoing_provider_go", True)))
    add("production go true mutation", lambda p: mutate_json(p/"ROLE_MATRIX.json", lambda d: d.__setitem__("production_go", True)))
    add("production admission mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d.__setitem__("production_admission", "GRANTED")))
    add("completeness mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d.__setitem__("completeness", "COMPLETE")))
    add("frontier mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d.__setitem__("frontier", "CLOSED")))
    add("direction incoming mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d.__setitem__("direction", "incoming")))
    add("incoming evidence flag mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d.__setitem__("incoming_evidence_or_authority_used", True)))
    add("incoming profile use mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d.__setitem__("incoming_profile_use", True)))
    add("incoming authority non-role mutation", lambda p: mutate_json(p/"ROLE_MATRIX.json", lambda d: d["non_roles"].__setitem__("incoming_authority", True)))
    add("composition authority non-role mutation", lambda p: mutate_json(p/"ROLE_MATRIX.json", lambda d: d["non_roles"].__setitem__("composition_authority", True)))
    add("incoming ledger dependency mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["forbidden_effects"].__setitem__("incoming_ledger_dependency", True)))
    add("incoming raw dependency path mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("path", "docs/qualification/incoming/run")))
    add("composition effect mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["forbidden_effects"].__setitem__("composition_change", True)))
    add("bidirectional effect mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["forbidden_effects"].__setitem__("bidirectional_composition", True)))
    add("adr0011 adr0007 effect mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["forbidden_effects"].__setitem__("adr0011_to_adr0007_composition", True)))
    add("stage8 effect mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["forbidden_effects"].__setitem__("stage8_change", True)))
    add("ceiling effect mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["forbidden_effects"].__setitem__("ceiling_change", True)))
    add("custody owner effect mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["forbidden_effects"].__setitem__("custody_owner_change", True)))
    add("runtime effect mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["forbidden_effects"].__setitem__("runtime_change", True)))
    add("public api schema cli mcp effect mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: [d["forbidden_effects"].__setitem__(k, True) for k in ["public_surface_change", "api_change", "schema_change", "cli_change", "mcp_change"]]))
    add("integration push publication release deployment mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: [d["forbidden_effects"].__setitem__(k, True) for k in ["integration", "push", "publication", "release", "deployment"]]))
    # Pins and prerequisite mutations.
    add("generic prerequisite hash mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["source_pins"]["generic_outgoing_semantics"].__setitem__("sha256", "0"*64)))
    add("provider binding role hash mutation", lambda p: mutate_json(p/"ROLE_MATRIX.json", lambda d: d["provider_binding_roles"][0].__setitem__("sha256", "0"*64)))
    add("accepted prerequisite hash mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["accepted_prerequisites_pinned"][0].__setitem__("sha256", "0"*64)))
    add("generic prerequisite commit mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["source_pins"]["generic_outgoing_semantics"].__setitem__("commit", "bad")))
    add("fresh evidence head mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d["source_pins"]["fresh_outgoing_evidence"].__setitem__("head", "bad")))
    add("accepted evidence head mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["accepted_prerequisites_pinned"][2].__setitem__("head", "bad")))
    add("fresh run id mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d["source_pins"]["fresh_outgoing_evidence"].__setitem__("run_id", "wrong")))
    add("final seal pin mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("final_seal_sha256", "0"*64)))
    add("final checksum pin mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("final_checksums_sha256", "0"*64)))
    add("artifact count mutation", lambda p: mutate_json(p/"MANIFEST.json", lambda d: d["fresh_final_pins"].__setitem__("artifact_count", 1)))
    # Comparison/accounting mutations.
    add("target count mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("target_count", 5899)))
    add("raw file count mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("raw_outgoing_file_count", 1)))
    add("target order digest mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("target_order_digest", "0"*64)))
    add("terminal event digest mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("terminal_event_digest_excluding_timestamps", "0"*64)))
    add("failure digest mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("failure_identity_digest", "0"*64)))
    add("semantic difference indicator mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["semantic_difference_indicators"].__setitem__("semantic_outgoing_payload_difference_found", True)))
    add("result known differences mutation", lambda p: mutate_json(p/"RESULT.json", lambda d: d["comparison_result"].__setitem__("known_nonsemantic_differences", ["rewritten wording"])))
    add("final counts mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"]["final_counts"].__setitem__("FAILED_LSP_ERROR", 66)))
    add("raw aggregate mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["historical_run"].__setitem__("raw_aggregate_digest", "0"*64)))
    add("artifact inventory mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"].__setitem__("artifact_inventory_digest", "0"*64)))
    add("maxima witness value mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"]["maxima"]["max_outgoing_response_bytes"].__setitem__("value", 1)))
    add("maxima qualifier mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"]["maxima"]["max_outgoing_response_bytes"].__setitem__("qualifier", "universal ceiling")))
    add("maxima witness file mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["fresh_run"]["maxima"]["max_outgoing_response_bytes"].__setitem__("file", "")))
    add("maxima witnesses mirror mutation", lambda p: mutate_json(p/"RUN_COMPARISON.json", lambda d: d["maxima_witnesses"].__setitem__("qualifier", "universal ceiling")))
    add("custody role historical source mutation", lambda p: mutate_json(p/"ROLE_MATRIX.json", lambda d: d["custody_roles"][0].__setitem__("source", "historical_outgoing")))
    add("failure identity omission", lambda p: (p/"FAILURE_IDENTITIES.tsv").write_text("\n".join((p/"FAILURE_IDENTITIES.tsv").read_text().splitlines()[:-1]) + "\n"))
    passed = []
    for name, mutator in tests:
        with tempfile.TemporaryDirectory() as td:
            work = pathlib.Path(td) / "pkg"
            shutil.copytree(pkg, work)
            mutator(work)
            try:
                verify(repo_root=repo, root=work)
            except VerifyError as exc:
                passed.append((name, str(exc)))
            else:
                fail("tamper unexpectedly passed: " + name)
    return passed

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--tamper-tests", action="store_true")
    parser.add_argument("--root")
    parser.add_argument("--repo-root")
    args = parser.parse_args()
    pkg = pathlib.Path(args.root or pathlib.Path(__file__).resolve().parent).resolve()
    repo = pathlib.Path(args.repo_root).resolve() if args.repo_root else next(p for p in [pkg, *pkg.parents] if (p / "go.mod").exists())
    if args.tamper_tests:
        verify(repo_root=repo, root=pkg)
        passed = run_tamper_tests(pkg, repo)
        print("TAMPER_TESTS_PASSED", len(passed))
        for name, err in passed:
            print(f"- {name}: {err}")
    else:
        verify(repo_root=repo, root=pkg)
        print("VERIFICATION_PASSED")

if __name__ == "__main__":
    try:
        main()
    except VerifyError as exc:
        print("VERIFICATION_FAILED:", exc, file=sys.stderr)
        sys.exit(1)
