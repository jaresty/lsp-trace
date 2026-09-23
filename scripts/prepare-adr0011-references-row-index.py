#!/usr/bin/env python3
"""Index every normative REFERENCES_SYMBOL_V1 matrix row without asserting a pass.

The pinned matrix line numbers are part of these draft IDs. A changed matrix must
be reviewed and this index regenerated, not silently reinterpreted.
"""

import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MATRIX = ROOT / "docs/qualification/adr0011-occurrence-falsification-matrix.review.md"
CONTRACT = ROOT / "docs/qualification/adr0011-production-admission-gate.review.md"
OUTPUT = ROOT / "docs/qualification/adr0011-references-row-evidence-index.draft.json"
MATRIX_HASH = "c370fd6ae76bb3b74a15da12a7a9c41d9730c68662830294a50dfe89177546a2"
CONTRACT_HASH = "d71ec03c9244eb70fe9d93023410e0dc5cedc9932c992b7ff1ef45f2fc42f7f9"

# Each normative substitution field is independently indexed, not rolled up.
# Some non-substitution rows also demand independent alternatives or bounds.
COMPOSITES = {
    22: ["partial_write", "short_write"],
    23: ["short_read", "unmatched_read"],
    32: ["pre_evaluation_work", "pre_evaluation_bytes"],
    38: ["duplicate_plain", "duplicate_escaped_alias", "duplicate_case_alias"],
    39: ["duplicate_plain", "duplicate_escaped_alias"],
    40: ["receipt_duplicate", "receipt_unknown", "receipt_trailing", "ledger_duplicate", "ledger_unknown", "ledger_trailing"],
    41: ["target_absent", "selection_tied", "uri_mismatch", "point_mismatch", "source_mismatch", "revision_mismatch"],
    53: ["diagnostic_within_cap", "diagnostic_withheld"],
    55: ["live_wire_diagnostic", "live_wire_withheld", "live_messages_diagnostic", "live_messages_withheld", "aggregate_work_diagnostic", "aggregate_work_withheld"],
    56: ["symbol_empty", "no_containing_selection"],
    57: ["selection_tied", "selection_overlap"],
    58: ["malformed_later_child", "unknown_field", "duplicate_key"],
    59: ["source_mismatch", "version_mismatch", "revision_mismatch", "encoding_mismatch", "missing_payload"],
    100: ["retire_delete_failed", "revoke_delete_failed"],
}

SUBSTITUTIONS = {
    67: ["method", "query_occurrence_id"],
    68: ["query_uri"],
    69: ["query_line", "query_character"],
    70: ["encoding"],
    71: ["raw_params_formatting"],
    72: ["params_value", "include_declaration"],
    73: ["session_id", "generation"],
    74: ["request_key", "invocation_id"],
    75: ["completed_write", "selected_read"],
    76: ["capability", "provider", "adapter"],
    77: ["git_root", "git_before_commit", "git_after_commit", "git_before_clean", "git_after_clean"],
    78: ["revision_commit", "revision_custody"],
    79: ["prepared_uri", "prepared_version", "prepared_digest"],
    80: ["symbol_source_payload", "symbol_result_bytes"],
    81: ["symbol_selection_range", "target_symbol_id"],
    82: ["symbol_method_selector", "symbol_method_digest", "symbol_terminal_selector", "symbol_terminal_digest", "query_target_selector", "query_target_digest"],
    83: ["references_method", "references_result_payload", "references_result_presence"],
    84: ["returned_uri", "returned_range", "returned_ordinal"],
    85: ["references_method_selector", "references_method_digest"],
    86: ["terminal_selector", "terminal_digest", "N", "B", "T", "E", "E_B", "E_T", "P", "A"],
    87: ["occurrence_selector", "occurrence_digest", "kind", "custody_role", "source_role", "target_role", "occurrence_id"],
    88: ["privacy_policy", "method_policy", "admission_policy", "implementation_digest", "schema_digest"],
    89: ["family_version", "family_kind", "duplicate_key", "trailing_content"],
}


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> None:
    if digest(MATRIX) != MATRIX_HASH or digest(CONTRACT) != CONTRACT_HASH:
        raise SystemExit("normative input changed: independently review and update pinned index")
    if set(SUBSTITUTIONS) != set(range(67, 90)):
        raise SystemExit("substitution line not independently indexed")
    lines = MATRIX.read_text(encoding="utf-8").splitlines()
    ranges = [("TERMINAL", range(15, 42)), ("BOUND_TARGET", range(51, 60)),
              ("SUBSTITUTION", range(67, 90)), ("LIFECYCLE", range(97, 102))]
    cases = []
    for family, row_numbers in ranges:
        for number in row_numbers:
            row = lines[number - 1]
            if not (row.startswith("| ") and row.endswith(" |")):
                raise SystemExit(f"line {number}: no intact table row")
            cells = [cell.strip() for cell in row[1:-1].split("|")]
            expected_columns = 2 if family == "SUBSTITUTION" else (4 if family == "LIFECYCLE" else 5)
            if len(cells) != expected_columns:
                raise SystemExit(f"line {number}: unexpected column count")
            variants = SUBSTITUTIONS[number] if family == "SUBSTITUTION" else COMPOSITES.get(number, ["base"])
            for variant in variants:
                case_id = f"R11-{family}-{number:03}-{variant}"
                substituted = family == "SUBSTITUTION"
                lifecycle = family == "LIFECYCLE"
                receipt = cells[2 if lifecycle else 3] if not substituted else None
                if number in (53, 55):
                    receipt = ("D; VERIFIED_DIAGNOSTIC with independently replayable redaction"
                               if "diagnostic" in variant else "N; WITHHELD, no verified diagnostic")
                cases.append({
                    "case_id": case_id,
                    "source_kind": "NORMATIVE_MATRIX_ROW",
                    "matrix_line": number,
                    "variant": variant,
                    "fixture": cells[0],
                    "required_independent_observation": cells[1] if substituted else None,
                    "expected_as_written": {
                        "outcome_and_query": ("No new issued request outcome or terminal; pre-invocation REVISION_MISMATCH, POLICY_MISMATCH or TARGET_IDENTITY_UNRESOLVED only where applicable"
                                              if substituted else cells[1]),
                        "counters": ("N=1 declared; no new verified B/T; E/E_B/E_T/P/A=U/0/0/0/0 for new admission"
                                     if substituted else (cells[1] if lifecycle else cells[2])),
                        "receipt_publication": ("No newly verified receipt; substituted object NOT_ATTEMPTED or unchanged original VERIFIED_SUCCESS"
                                                if substituted else receipt),
                        "eligibility": "substituted path inactive" if substituted else cells[3 if lifecycle else 4],
                    },
                    "historical_stored_counters": ({"E": 2, "E_B": 2, "E_T": 2, "P": 2, "A": 2, "A_new": 0}
                                                   if lifecycle else None),
                    "original_unchanged_receipt_may_remain_verified": (True if substituted else None),
                    "lifecycle_original_eligibility": ("ACTIVE; removal denied" if number == 99 else
                                                       "TOMBSTONED_OR_QUARANTINED" if lifecycle else None),
                    "cap_reachability": ("BLOCKED_BY_CURRENT_1MIB_MANAGER_WIRE_CAP" if number == 53 else None),
                    "input_selectors_and_digests": None,
                    "owner_and_fixture_profile": None,
                    "observed_counters_and_dispositions": None,
                    "publication_and_readback": None,
                    "privacy_redaction": None,
                    "executable_guard": None,
                    "reviewer": None,
                    "disposition": "INCOMPLETE",
                })
    # The matrix's hard-bound prose additionally requires independently
    # selected limits not yet numerically frozen in the accepted contract.
    # These are explicit incomplete obligations, not invented test results.
    policy_bounds = ("elapsed_time", "total_retained_bytes", "total_retained_objects",
                     "allocation", "document_supply_bytes", "document_requests", "document_total_bytes")
    for bound in policy_bounds:
        for side in ("at", "over"):
            cases.append({
                "case_id": f"R11-POLICY_BOUND-045-{bound}-{side}",
                "source_kind": "SUPPLEMENTARY_POLICY_BOUND_PLACEHOLDER",
                "matrix_line": 45,
                "variant": f"{bound}-{side}",
                "fixture": f"independent selected {bound} {side} cap; policy value not yet frozen",
                "expected_as_written": {"outcome_and_query": None, "counters": None,
                                        "receipt_publication": None, "eligibility": None},
                "input_selectors_and_digests": None,
                "owner_and_fixture_profile": None,
                "observed_counters_and_dispositions": None,
                "publication_and_readback": None,
                "privacy_redaction": None,
                "executable_guard": None,
                "reviewer": None,
                "disposition": "INCOMPLETE",
            })
    # The normative tables contain over-limit fixtures but not independent
    # exactly-at-cap controls for all four accepted numeric bounds.
    for bound, line in (("queries_16", 51), ("params_65536", 52),
                        ("result_1048576", 53), ("canonical_1500000", 54)):
        cases.append({
            "case_id": f"R11-SELECTED_AT-{line:03}-{bound}",
            "source_kind": "SUPPLEMENTARY_AT_LIMIT_CONTROL",
            "matrix_line": line,
            "variant": bound + "-at",
            "fixture": f"independent exactly-at-limit {bound}; reject if shadowed by earlier cap",
            "expected_as_written": {"outcome_and_query": None,
                                    "counters": ("N=16 declared; other counters require a pinned full acquisition fixture"
                                                 if bound == "queries_16" else None),
                                    "receipt_publication": None, "eligibility": None},
            "cap_reachability": ("BLOCKED_BY_CURRENT_1MIB_MANAGER_WIRE_CAP" if bound == "result_1048576" else "UNVERIFIED"),
            "input_selectors_and_digests": None,
            "owner_and_fixture_profile": None,
            "observed_counters_and_dispositions": None,
            "publication_and_readback": None,
            "privacy_redaction": None,
            "executable_guard": None,
            "reviewer": None,
            "disposition": "INCOMPLETE",
        })
    ids = [case["case_id"] for case in cases]
    if len(ids) != len(set(ids)):
        raise SystemExit("duplicate row identity")
    manifest = {
        "version": "adr0011.references.row-evidence-index.draft.v1",
        "decision": "INCOMPLETE",
        "contract_sha256": "sha256:" + CONTRACT_HASH,
        "matrix_sha256": "sha256:" + MATRIX_HASH,
        "implementation_schema_policy_digests": None,
        "coverage_status": "DRAFT_INCOMPLETE_UNREVIEWED",
        "selected_result_bound_reachability": "BLOCKED_BY_CURRENT_1MIB_MANAGER_WIRE_CAP_FOR_1048576_BYTE_RESULT_AND_1048577_OVER_LIMIT",
        "substitution_common_verdict": "No newly issued request outcome or terminal; E/E_B/E_T/P/A=U/0/0/0/0 for new admission; N=1 already declared; no newly verified receipt; substituted path inactive. Original unchanged receipt may remain verified.",
        "counts_by_family": {family: sum(c["case_id"].startswith("R11-" + family + "-") for c in cases)
                             for family in [*(name for name, _ in ranges), "POLICY_BOUND", "SELECTED_AT"]},
        "cases": cases,
    }
    OUTPUT.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"{OUTPUT.relative_to(ROOT)}: {len(cases)} indexed fixtures; all INCOMPLETE")


if __name__ == "__main__":
    main()
