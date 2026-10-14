#!/usr/bin/env python3
from __future__ import annotations
import argparse, collections, csv, copy, hashlib, json, pathlib, subprocess, sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
EXPECTED_HEAD = "b4a86914702fd595f66bd1e0085c7063fe517144"
PACKAGE_PREFIX = "docs/qualification/adr0011-calls-go-gopls-incoming-profile-qualification-v1/"
GENERIC_COMMIT = "c3d77247f5a2e06c0d6ac5b1b55a3df7dbea588a"
GENERIC_CHECKSUMS = "844e2c072474f6b3756ec221ea0b6df2888d775e9b866e1da0bbef030e35b351"
GENERIC_MANIFEST = "8657876ab14ad6c4939218331d8a58e3c222da61b21ae55200644e5c43674f7d"
CUSTODY_TOKEN = "INCOMING_CUSTODY_CORRECTION_EVIDENCE_GO"
FINAL_CHECKSUMS_SHA = "ba003061790bb4033bcabf85480e92ec97aba4522ab9877d99ccabbf24db60a8"
FINAL_SEAL_SHA = "a15ddcb57a0f908e13e7742dd0697dff717fd08e66e2aee28a478020f1c3ba34"
COUNTS = {"denominator": 5900, "attempted": 5900, "unattempted": 0, "nonempty": 5608, "empty": 225, "fail": 67, "refused": 0, "unsupported": 0, "timeout": 0, "cancel": 0, "malformed": 0, "partial": 0, "one_final_disposition_ordinals": 5900}
FINALS = {"FAILED_LSP_ERROR": 67, "SUCCEEDED_EMPTY": 225, "SUCCEEDED_NON_EMPTY": 5608}
MAXIMA = {"max_elapsed_seconds": 1.950021, "max_message_count_per_target": 157, "max_prepare_request_bytes": 309, "max_prepare_response_bytes": 10861, "max_prepare_returned_members": 1, "max_prepare_returned_nodes": 1, "max_prepare_returned_ranges": 2, "max_request_bytes": 144704, "max_response_bytes": 67906, "max_result_request_bytes": 572, "max_result_response_bytes": 67906, "max_result_returned_members": 114, "max_result_returned_nodes": 114, "max_result_returned_occurrences": 328, "max_result_returned_ranges": 328, "max_ledger_result_count": 114}
SEMANTIC_STATEMENTS = [
    {"id": "top_level_response_null_vs_empty_array_distinct", "statement": "top-level response null and [] remain distinct successful-empty source forms when admitted by terminal policy"},
    {"id": "missing_fromRanges_malformed", "statement": "missing member fromRanges is malformed for semantic admission and preserved only in raw custody"},
    {"id": "null_fromRanges_malformed", "statement": "null member fromRanges is malformed for semantic admission and preserved only in raw custody"},
    {"id": "present_empty_fromRanges_retains_member_zero_occurrence_range_rows", "statement": "present empty fromRanges: [] retains an otherwise-valid relationship/member and emits zero CALLS occurrence rows and zero occurrence range rows"},
    {"id": "nonempty_fromRanges_preserve_supplied_ranges_order_duplicates_ordinals_without_synthesis", "statement": "nonempty supplied fromRanges are the only CALLS occurrence ranges; supplied range order, target ordinal, member ordinal, and duplicate multiplicity are preserved without synthesized ranges"},
    {"id": "endpoint_spans_identity_display_never_occurrence_evidence", "statement": "returned from.range and from.selectionRange are endpoint identity/display spans, never occurrence evidence; CALLS occurrences are derived solely from supplied fromRanges"},
]
SEMANTIC_IDS = [s["id"] for s in SEMANTIC_STATEMENTS]
SEMANTIC_TEXTS = [s["statement"] for s in SEMANTIC_STATEMENTS]
OCCURRENCE_SOURCE_POLICY = "CALLS occurrence evidence is derived solely from supplied fromRanges; from.range and from.selectionRange are endpoint identity/display spans only and never occurrence evidence."
SOURCE = {
    "docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1/CHECKSUMS.sha256": GENERIC_CHECKSUMS,
    "docs/qualification/adr0011-calls-lsp-incoming-exact-v1-generic-semantics-v1/MANIFEST.json": GENERIC_MANIFEST,
    "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming/FINAL_SUMMARY.json": "7bab05495d489f4e12f2e25a9ab16ba9206ca7eb74f62f9d947f92498f55066f",
    "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming/NUMERIC_BOUNDS.json": "46a484d004dfcf8c019290aa8d8a724e1a1a1682982c5460c1976c68c47b6397",
    "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming/FAILURES.jsonl": "ad2785c3690f1ce104cd803058c9dace7fbb379ad65bd18d8ebc87ab7cef345f",
    "docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming/ledgers/incoming.attempts.jsonl": "5c181ac1304547ba8f9e915aa512f3c4e93ca1fd8594da381f7f61b165d7139f",
    "docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/runs/incoming-custody-correction-v2/full-incoming.final-run.checksums.sha256": FINAL_CHECKSUMS_SHA,
    "docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/runs/incoming-custody-correction-v2/full-incoming.final-run.seal.json": FINAL_SEAL_SHA,
}
TEXT_FILES = ["README.md", "MANIFEST.json", "RESULT.json", "REVIEW_REQUEST.md", "FAILURE_IDENTITIES.tsv"]


def rel(p: pathlib.Path | str) -> pathlib.Path:
    return ROOT / pathlib.Path(p)


def sha(p: pathlib.Path) -> str:
    h = hashlib.sha256()
    with p.open("rb") as f:
        for c in iter(lambda: f.read(1024 * 1024), b""):
            h.update(c)
    return h.hexdigest()


def j(p: pathlib.Path) -> dict:
    return json.loads(p.read_text())


def jl(p: pathlib.Path):
    with p.open() as f:
        for line in f:
            if line.strip():
                yield json.loads(line)


def req(label: str, ok: bool):
    print(("PASS " if ok else "FAIL ") + label)
    if not ok:
        raise AssertionError(label)


def exact_zero(v) -> bool:
    return type(v) is int and v == 0


def exact_false(v) -> bool:
    return type(v) is bool and v is False


def package_jsons():
    return j(HERE / "MANIFEST.json"), j(HERE / "RESULT.json")


def check_git():
    req("HEAD_PIN", subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip() == EXPECTED_HEAD)
    st = subprocess.check_output(["git", "-C", str(ROOT), "status", "--porcelain"], text=True).splitlines()
    req("WRITE_SCOPE_PROVIDER_PACKAGE_ONLY", all(x[3:].startswith(PACKAGE_PREFIX) for x in st if x))


def check_sources():
    for p, e in SOURCE.items():
        req("SOURCE_HASH " + p, sha(rel(p)) == e)


def check_semantics(m, r):
    req("MANIFEST_GENERIC_SEMANTICS_EXACT", m.get("accepted_generic_semantic_statements") == SEMANTIC_STATEMENTS and m.get("generic_semantics_preserved_verbatim") == SEMANTIC_TEXTS)
    req("RESULT_GENERIC_SEMANTICS_EXACT", r.get("accepted_generic_semantic_statements") == SEMANTIC_STATEMENTS and r.get("semantic_artifacts", {}).get("generic_semantics_preserved_verbatim") == SEMANTIC_TEXTS)
    roles = {x["role"]: x for x in m["role_manifest"]}
    semantic_role = roles.get("generic_semantic_statement_preservation", {})
    req("ROLE_SEMANTIC_ARTIFACTS_EXACT", semantic_role.get("status") == "PRESENT_EXACT" and semantic_role.get("statement_ids") == SEMANTIC_IDS and semantic_role.get("statement_count") == len(SEMANTIC_STATEMENTS))
    req("RESULT_OCCURRENCE_SOURCE_POLICY_EXACT", r.get("semantic_artifacts", {}).get("occurrence_source_policy") == OCCURRENCE_SOURCE_POLICY)
    req("MANIFEST_OCCURRENCE_SOURCE_POLICY_EXACT", m.get("occurrence_source_policy") == OCCURRENCE_SOURCE_POLICY and m.get("semantic_artifacts", {}).get("occurrence_source_policy") == OCCURRENCE_SOURCE_POLICY)
    req("ROLE_OCCURRENCE_SOURCE_POLICY_EXACT", semantic_role.get("occurrence_source_policy") == OCCURRENCE_SOURCE_POLICY)
    req("OCCURRENCE_EVIDENCE_SOLELY_FROMRANGES", "derived solely from supplied fromRanges" in OCCURRENCE_SOURCE_POLICY)
    req("ENDPOINT_SPANS_IDENTITY_DISPLAY_ONLY", "from.range and from.selectionRange are endpoint identity/display spans only" in OCCURRENCE_SOURCE_POLICY and "never occurrence evidence" in OCCURRENCE_SOURCE_POLICY)
    joined = "\n".join(SEMANTIC_TEXTS)
    req("SEMANTIC_NULL_VS_EMPTY_PIN", "top-level response null and [] remain distinct" in joined)
    req("SEMANTIC_MISSING_FROMRANGES_MALFORMED", "missing member fromRanges is malformed" in joined)
    req("SEMANTIC_NULL_FROMRANGES_MALFORMED", "null member fromRanges is malformed" in joined)
    req("SEMANTIC_EMPTY_FROMRANGES_ZERO_ROWS", "zero CALLS occurrence rows and zero occurrence range rows" in joined)
    req("SEMANTIC_NONEMPTY_FROMRANGES_PRESERVE", "supplied range order, target ordinal, member ordinal, and duplicate multiplicity are preserved without synthesized ranges" in joined)
    req("SEMANTIC_ENDPOINT_SPANS_NOT_OCCURRENCE", "never occurrence evidence" in joined and "derived solely from supplied fromRanges" in joined)


def declared_authority_objects(m, r):
    objs = [("MANIFEST.top", m), ("RESULT.top", r)]
    for source_name, source in (("MANIFEST", m), ("RESULT", r)):
        for name, value in source.get("declared_authority", {}).items():
            objs.append((source_name + ".declared_authority." + name, value))
    return objs


def check_authority(m, r):
    for name, obj in declared_authority_objects(m, r):
        req("AUTHORITY_NUMERIC_ZERO " + name, exact_zero(obj.get("authority")))
        if "accepted" in obj:
            req("ACCEPTED_FALSE " + name, exact_false(obj.get("accepted")))
        if "production_admission" in obj:
            req("PRODUCTION_REFUSED " + name, obj.get("production_admission") == "REFUSED_NOT_REQUESTED_NOT_GRANTED")
    req("AUTHORITY_NO_BOOL_OR_STRING_SENTINEL", all(not isinstance(obj.get("authority"), bool) and not isinstance(obj.get("authority"), str) for _, obj in declared_authority_objects(m, r)))


def check_manifest(m=None, r=None):
    m = m or j(HERE / "MANIFEST.json")
    r = r or j(HERE / "RESULT.json")
    req("PROFILE_ID", m["profile_id"] == "CALLS_GO_GOPLS_INCOMING_EXACT_V1" and r["profile_id"] == m["profile_id"])
    req("INCOMING_ONLY", m["canonical_identity_domain"].endswith("incoming.exact.v1") and m["accepted_custody_dependency"]["run_id"] == "incoming-custody-correction-v2")
    req("GENERIC_TOKEN_PIN", m["accepted_generic_dependency"]["token"] == "CALLS_LSP_INCOMING_EXACT_V1_SEMANTICS_GO")
    req("GENERIC_COMMIT_PIN", m["accepted_generic_dependency"]["commit"] == GENERIC_COMMIT)
    req("GENERIC_HASH_PINS", m["accepted_generic_dependency"]["checksums_sha256"] == GENERIC_CHECKSUMS and m["accepted_generic_dependency"]["manifest_sha256"] == GENERIC_MANIFEST)
    req("CUSTODY_TOKEN_PIN", m["accepted_custody_dependency"]["token"] == CUSTODY_TOKEN and r["bounded_qualification_candidate"]["custody_dependency_token"] == CUSTODY_TOKEN)
    req("CUSTODY_DIGEST_PINS", m["accepted_custody_dependency"]["final_checksums_sha256"] == FINAL_CHECKSUMS_SHA and m["accepted_custody_dependency"]["final_seal_sha256"] == FINAL_SEAL_SHA)
    req("AUTHORED_NOT_GO_NO_SELF_GO", m["producer_disposition"] == "AUTHORED_NOT_GO" and r["producer_disposition"] == "AUTHORED_NOT_GO" and m["producer_may_issue_go"] is False and r["producer_may_issue_go"] is False and m["accepted"] is False and r["accepted"] is False)
    req("REVIEW_TOKEN_SEPARATED", m["review_token_requested"] == "ADR0011_QUALIFIED_CALLS_INCOMING_PROFILE_GO" and m["review_token_requested_by_producer"] is False and r["review_token_requested_by_producer"] is False)
    check_authority(m, r)
    req("ROLE_CLOSURE_NO_MISSING", m["role_closure"]["mandatory_missing_count"] == 0 and m["role_closure"]["mandatory_refused_count"] == 0)
    roles = {x["role"]: x for x in m["role_manifest"]}
    for role in ("gopls_executable_byte_length", "complete_prelaunch_environment", "go_toolchain_binary_digest_inventory"):
        req("MANDATORY_CUSTODY_ROLE_CLOSED " + role, roles[role]["mandatory"] is True and roles[role]["status"].startswith("PRESENT"))
    req("GOPLS_BYTE_LENGTH", m["provider_runtime_evidence_v2"]["gopls"]["byte_length"] == 41038194 and r["closed_v2_custody_roles"]["gopls_executable_byte_length"] == 41038194)
    req("COMPLETE_ENV_CAPTURED", m["provider_runtime_evidence_v2"]["complete_prelaunch_environment"]["key_count"] == 64 and r["closed_v2_custody_roles"]["complete_prelaunch_environment"]["environment_key_count"] == 64)
    req("GO_TOOLCHAIN_COMPLETE", m["provider_runtime_evidence_v2"]["toolchain_summary"]["complete"] is True and r["closed_v2_custody_roles"]["go_toolchain_binary_digest_inventory"]["complete"] is True)
    check_semantics(m, r)
    req("COUNTS_PIN", m["workload_counts"] == COUNTS and m["final_disposition_counts"] == FINALS and r["bounded_qualification_candidate"]["counts"] == {"succeeded_nonempty": 5608, "succeeded_empty": 225, "failed_lsp_error": 67, "refused": 0, "one_final_disposition_ordinals": 5900})
    req("MAXIMA_MEASURED_NOT_UNIVERSAL", "measured-not-universal" in m["numeric_envelope_scope"] and r["bounded_qualification_candidate"]["numeric_maxima_scope"] == "measured-not-universal")
    req("FRONTIER_UNKNOWN", m["frontier"]["status"] == "UNKNOWN" and r["bounded_qualification_candidate"]["frontier"] == "UNKNOWN")
    req("HISTORICAL_CORRECTION_COMPARISON", m["historical_vs_correction"]["final_counts_equal"] is True and r["historical_vs_correction"]["final_counts_equal"] is True and m["historical_vs_correction"]["semantic_result_drift_blocker"] is None)
    req("LINEAGE_COMPLETE", [x["event"] for x in m["recursive_presumption_ledger"]] == ["first_provider_candidate_NO_GO", "generic_prerequisite_GO", "failed_pre_effect_v1", "second_provider_candidate_NO_GO", "incoming_custody_rerun_corrections", "accepted_v2", "verifier_tamper_coverage_NO_GO", "verifier_tamper_coverage_corrected", "occurrence_source_policy_escape_NO_GO", "occurrence_source_policy_escape_corrected"])
    req("NO_STALE_MISSING_ROLE_FLAGS", "absent_or_refused_roles" not in json.dumps(m) and "ABSENT_FAIL_CLOSED" not in json.dumps(m) and "not_evidenced_fail_closed" not in json.dumps(m) and "absent_or_refused_roles" not in json.dumps(r))
    req("NO_FORBIDDEN_EFFECTS", not any(m["forbidden_effects"].values()))


def check_text():
    forbidden = [b"CALLS_GO_GOPLS_OUTGOING", b"CALLS_LSP_OUTGOING", b"callHierarchy/outgoingCalls"]
    for n in TEXT_FILES:
        data = (HERE / n).read_bytes()
        for tok in forbidden:
            req("NO_FORBIDDEN_MACHINE_TOKEN " + n + " " + tok.decode(), tok not in data)


def check_evidence():
    hist = rel("docs/qualification/evidence/adr0011-calls-go-gopls-workload-study-v1/runs/full-incoming")
    v2_base = rel("docs/qualification/adr0011-calls-go-gopls-incoming-custody-rerun-addendum-v1/runs/incoming-custody-correction-v2")
    v2 = v2_base / "full-incoming"
    s = j(hist / "FINAL_SUMMARY.json")
    b = j(hist / "NUMERIC_BOUNDS.json")
    seal = j(v2_base / "full-incoming.final-run.seal.json")
    summary = j(v2 / "custody/prelaunch-custody.summary.json")
    env = j(v2 / "provenance/environment.json")
    ledger_finals = [x for x in jl(v2 / "ledgers/incoming.attempts.jsonl") if x.get("disposition") != "STARTED"]
    req("SUMMARY_COUNTS", s["counts"] == COUNTS)
    req("SUMMARY_FINALS", s["final_disposition_counts"] == FINALS)
    req("V2_SEAL_HASH", sha(v2_base / "full-incoming.final-run.seal.json") == FINAL_SEAL_SHA and seal["checksums_sha256"] == FINAL_CHECKSUMS_SHA)
    req("V2_CHECKSUMS_HASH", sha(v2_base / "full-incoming.final-run.checksums.sha256") == FINAL_CHECKSUMS_SHA)
    req("V2_ARTIFACT_COUNT", seal["artifact_count"] == 53380 and sum(1 for _ in (v2_base / "full-incoming.final-run.checksums.sha256").open("rb")) == 53380)
    req("V2_COUNTS_EQUAL", seal["final_counts"]["final_disposition_counts"] == FINALS and seal["final_counts"]["one_final_disposition_ordinals"] == 5900)
    req("V2_LEDGER_FINALS", len(ledger_finals) == 5900 and dict(collections.Counter(r["disposition"] for r in ledger_finals)) == FINALS)
    by = collections.defaultdict(list)
    for row in ledger_finals:
        by[row["ordinal"]].append(row)
    req("V2_LEDGER_ONE_FINAL_PER_ORDINAL", len(by) == 5900 and all(len(v) == 1 for v in by.values()))
    req("V2_GOPLS_BYTE_LENGTH", env["gopls_executable"]["byte_length"] == 41038194)
    req("V2_COMPLETE_ENV", env["prelaunch_custody_seal"]["complete_env_key_count"] == 64 and env["prelaunch_custody_seal"]["sealed_before_gopls_launch"] is True)
    req("V2_GO_TOOLCHAIN", summary["toolchain_summary"]["complete"] is True and summary["toolchain_summary"]["required_record_count"] == 14 and summary["executable_summaries"]["go"]["byte_length"] == 14481762)
    req("V2_NO_PRODUCTION", seal["production_admission"] == "REFUSED_NOT_REQUESTED_NOT_GRANTED" and summary["production_admission"] == "REFUSED_NOT_REQUESTED_NOT_GRANTED")
    for k, v in MAXIMA.items():
        req("MAXIMUM " + k, b["metrics"][k]["status"] == "PRESENT" and b["metrics"][k]["max"] == v and bool(b["metrics"][k]["targets"]))
    req("FRONTIER_UNKNOWN_EVIDENCE", b["metrics"]["max_frontier"]["status"] == "UNKNOWN")
    with (HERE / "FAILURE_IDENTITIES.tsv").open(newline="") as f:
        rows = list(csv.DictReader(f, delimiter="\t"))
    failures = list(jl(hist / "FAILURES.jsonl"))
    exp = {(str(r["ordinal"]), r["final_event"]["target_id"], r["final_event"]["error"]) for r in failures}
    got = {(r["ordinal"], r["target_id"], r["error"]) for r in rows}
    req("FAILURE_INDEX_IDENTITIES", got == exp and len(got) == 67)


def checks():
    check_git()
    check_sources()
    m, r = package_jsons()
    check_manifest(m, r)
    check_text()
    check_evidence()
    print("INCOMING_PROVIDER_PROFILE_PACKAGE_VERIFY_OK_AUTHORED_NOT_GO")


def tamper(label, expected_assertion, mut):
    m, r = package_jsons()
    try:
        mut(m, r)
        check_manifest(m, r)
    except AssertionError as e:
        observed = str(e)
        if observed != expected_assertion:
            print(f"FAIL TAMPER {label} expected {expected_assertion} observed {observed}")
            raise
        print(f"PASS TAMPER {label} ASSERTION {observed}")
        return
    raise AssertionError("tamper not detected: " + label)


def remove_statement(container, sid):
    container["accepted_generic_semantic_statements"] = [x for x in container["accepted_generic_semantic_statements"] if x["id"] != sid]


def substitute_statement(container, sid):
    for x in container["accepted_generic_semantic_statements"]:
        if x["id"] == sid:
            x["statement"] = "TAMPERED " + x["statement"]
            return
    raise KeyError(sid)


def tampers():
    tamper("provider_self_go", "AUTHORED_NOT_GO_NO_SELF_GO", lambda m, r: (m.__setitem__("producer_may_issue_go", True), r.__setitem__("producer_may_issue_go", True)))
    tamper("review_token_by_producer", "REVIEW_TOKEN_SEPARATED", lambda m, r: m.__setitem__("review_token_requested_by_producer", True))
    tamper("authority_manifest_1", "AUTHORITY_NUMERIC_ZERO MANIFEST.top", lambda m, r: m.__setitem__("authority", 1))
    tamper("authority_result_1", "AUTHORITY_NUMERIC_ZERO RESULT.top", lambda m, r: r.__setitem__("authority", 1))
    tamper("authority_manifest_bool", "AUTHORITY_NUMERIC_ZERO MANIFEST.top", lambda m, r: m.__setitem__("authority", False))
    tamper("authority_result_string", "AUTHORITY_NUMERIC_ZERO RESULT.top", lambda m, r: r.__setitem__("authority", "0"))
    tamper("final_checksums_sha", "CUSTODY_DIGEST_PINS", lambda m, r: m["accepted_custody_dependency"].__setitem__("final_checksums_sha256", "0" * 64))
    tamper("final_seal_sha", "CUSTODY_DIGEST_PINS", lambda m, r: m["accepted_custody_dependency"].__setitem__("final_seal_sha256", "0" * 64))
    tamper("remove_null_vs_empty_pin", "MANIFEST_GENERIC_SEMANTICS_EXACT", lambda m, r: remove_statement(m, "top_level_response_null_vs_empty_array_distinct"))
    tamper("remove_missing_null_malformed_semantics", "MANIFEST_GENERIC_SEMANTICS_EXACT", lambda m, r: (remove_statement(m, "missing_fromRanges_malformed"), remove_statement(m, "null_fromRanges_malformed")))
    tamper("endpoint_spans_to_occurrence_evidence", "RESULT_GENERIC_SEMANTICS_EXACT", lambda m, r: substitute_statement(r, "endpoint_spans_identity_display_never_occurrence_evidence"))
    tamper("result_occurrence_source_policy_endpoint_spans", "RESULT_OCCURRENCE_SOURCE_POLICY_EXACT", lambda m, r: r["semantic_artifacts"].__setitem__("occurrence_source_policy", "CALLS occurrences are derived from endpoint spans"))
    tamper("result_occurrence_source_policy_deleted", "RESULT_OCCURRENCE_SOURCE_POLICY_EXACT", lambda m, r: r["semantic_artifacts"].pop("occurrence_source_policy"))
    tamper("manifest_occurrence_source_policy_endpoint_spans", "MANIFEST_OCCURRENCE_SOURCE_POLICY_EXACT", lambda m, r: m.__setitem__("occurrence_source_policy", "CALLS occurrences are derived from endpoint spans"))
    tamper("role_occurrence_source_policy_endpoint_spans", "ROLE_OCCURRENCE_SOURCE_POLICY_EXACT", lambda m, r: next(x for x in m["role_manifest"] if x["role"] == "generic_semantic_statement_preservation").__setitem__("occurrence_source_policy", "CALLS occurrences are derived from endpoint spans"))
    for s in SEMANTIC_STATEMENTS:
        sid = s["id"]
        tamper("manifest_delete_" + sid, "MANIFEST_GENERIC_SEMANTICS_EXACT", lambda m, r, sid=sid: remove_statement(m, sid))
        tamper("manifest_substitute_" + sid, "MANIFEST_GENERIC_SEMANTICS_EXACT", lambda m, r, sid=sid: substitute_statement(m, sid))
        tamper("result_delete_" + sid, "RESULT_GENERIC_SEMANTICS_EXACT", lambda m, r, sid=sid: remove_statement(r, sid))
        tamper("result_substitute_" + sid, "RESULT_GENERIC_SEMANTICS_EXACT", lambda m, r, sid=sid: substitute_statement(r, sid))
    tamper("role_semantic_statement_delete", "ROLE_SEMANTIC_ARTIFACTS_EXACT", lambda m, r: next(x for x in m["role_manifest"] if x["role"] == "generic_semantic_statement_preservation").__setitem__("statement_ids", SEMANTIC_IDS[:-1]))
    tamper("stale_missing_role_flag", "NO_STALE_MISSING_ROLE_FLAGS", lambda m, r: m.__setitem__("absent_or_refused_roles", ["gopls_executable_byte_length"]))
    tamper("remove_gopls_byte_length_role", "MANDATORY_CUSTODY_ROLE_CLOSED gopls_executable_byte_length", lambda m, r: next(x for x in m["role_manifest"] if x["role"] == "gopls_executable_byte_length").__setitem__("status", "ABSENT_FAIL_CLOSED"))
    tamper("complete_env_missing", "COMPLETE_ENV_CAPTURED", lambda m, r: m["provider_runtime_evidence_v2"]["complete_prelaunch_environment"].__setitem__("key_count", 0))
    tamper("go_toolchain_incomplete", "GO_TOOLCHAIN_COMPLETE", lambda m, r: m["provider_runtime_evidence_v2"]["toolchain_summary"].__setitem__("complete", False))
    tamper("counts_drift", "COUNTS_PIN", lambda m, r: m["final_disposition_counts"].__setitem__("SUCCEEDED_EMPTY", 224))
    tamper("maxima_universal", "MAXIMA_MEASURED_NOT_UNIVERSAL", lambda m, r: m.__setitem__("numeric_envelope_scope", "universal ceiling"))
    tamper("frontier_invented", "FRONTIER_UNKNOWN", lambda m, r: m["frontier"].__setitem__("status", "KNOWN"))
    tamper("outgoing_profile", "INCOMING_ONLY", lambda m, r: m.__setitem__("canonical_identity_domain", "lsp-trace.adr0011.calls.go.gopls.outgoing.exact.v1"))
    print("INCOMING_PROVIDER_PROFILE_TAMPER_TESTS_OK_AUTHORED_NOT_GO")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tamper-tests", action="store_true")
    args = ap.parse_args()
    if args.tamper_tests:
        tampers()
    else:
        checks()


if __name__ == "__main__":
    try:
        main()
    except AssertionError as e:
        print("VERIFY_FAILED", e, file=sys.stderr)
        sys.exit(1)
