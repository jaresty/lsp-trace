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
            for variant in SUBSTITUTIONS[number] if family == "SUBSTITUTION" else ["base"]:
                case_id = f"R11-{family}-{number:03}-{variant}"
                cases.append({
                    "case_id": case_id,
                    "matrix_line": number,
                    "variant": variant,
                    "fixture": cells[0],
                    "expected_as_written": {
                        "outcome_and_query": cells[1],
                        "counters": ("N=1 declared; no new verified B/T; E/E_B/E_T/P/A=U/0/0/0/0 for new admission"
                                     if family == "SUBSTITUTION" else (cells[1] if family == "LIFECYCLE" else cells[2])),
                        "receipt_publication": None if family == "SUBSTITUTION" else cells[2 if family == "LIFECYCLE" else 3],
                        "eligibility": None if family == "SUBSTITUTION" else cells[3 if family == "LIFECYCLE" else 4],
                    },
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
        "substitution_common_verdict": "No newly issued request outcome or terminal; E/E_B/E_T/P/A=U/0/0/0/0 for new admission; N=1 already declared; no newly verified receipt; substituted path inactive. Original unchanged receipt may remain verified.",
        "counts_by_family": {family: sum(c["case_id"].startswith("R11-" + family + "-") for c in cases)
                             for family, _ in ranges},
        "cases": cases,
    }
    OUTPUT.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"{OUTPUT.relative_to(ROOT)}: {len(cases)} indexed fixtures; all INCOMPLETE")


if __name__ == "__main__":
    main()
