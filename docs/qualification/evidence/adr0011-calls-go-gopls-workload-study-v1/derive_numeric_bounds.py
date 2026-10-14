#!/usr/bin/env python3
"""Deterministic numeric-bound derivation/review for ADR0011 CALLS Go/gopls runs.

Evidence-only package-local tool. It reads one explicit directional run directory at a
 time, writes derived review artifacts, and never mutates runner/source/raw/ledger evidence.
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import pathlib
import sys
from collections import Counter, defaultdict
from typing import Any, Dict, Iterable, List, Optional, Tuple

PACKAGE_DIR = pathlib.Path(__file__).resolve().parent
REPO_ROOT = PACKAGE_DIR.parents[3]
CONTRACT_PATH = PACKAGE_DIR / "contract.json"
EXPECTED_DENOMINATOR = 5900
EXPECTED_BATCH_COUNT = 118
DERIVED_FILES = [
    "NUMERIC_BOUNDS.json",
    "FINAL_SUMMARY.json",
    "FAILURES.jsonl",
    "REFUSALS.jsonl",
    "PROVENANCE.json",
    "SEALED_MANIFEST.json",
]
CHECKSUM_FILE = "DERIVED_CHECKSUMS.sha256"
RUN_ARTIFACT_SCHEMA = "lsp-trace.adr0011.calls.go.gopls.numeric-bounds.v1"
WORKLOAD_SCHEMA = "lsp-trace.adr0011.calls.go.gopls.workload-review-package.v1"

FINAL_DISPOSITIONS = {
    "SUCCEEDED_EMPTY",
    "SUCCEEDED_NON_EMPTY",
    "REFUSED_SOURCE_HASH_MISMATCH",
    "REFUSED_PREPARE_EMPTY",
    "REFUSED_PREPARE_AMBIGUOUS",
    "REFUSED_SELECTED_ITEM_TARGET_MISMATCH",
    "REFUSED_GOPLS_UNAVAILABLE",
    "FAILED_LSP_ERROR",
    "FAILED_TIMEOUT",
    "FAILED_EXCEPTION",
}

FAILURE_DISPOSITIONS = {"FAILED_LSP_ERROR", "FAILED_TIMEOUT", "FAILED_EXCEPTION"}
REFUSAL_DISPOSITIONS = {
    "REFUSED_SOURCE_HASH_MISMATCH",
    "REFUSED_PREPARE_EMPTY",
    "REFUSED_PREPARE_AMBIGUOUS",
    "REFUSED_SELECTED_ITEM_TARGET_MISMATCH",
    "REFUSED_GOPLS_UNAVAILABLE",
}


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def read_json(path: pathlib.Path) -> Any:
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def write_json(path: pathlib.Path, data: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    tmp.replace(path)


def write_jsonl(path: pathlib.Path, rows: Iterable[Dict[str, Any]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    with tmp.open("w", encoding="utf-8") as f:
        for row in rows:
            f.write(json.dumps(row, sort_keys=True) + "\n")
    tmp.replace(path)


def package_relative(path: pathlib.Path) -> str:
    return str(path.resolve().relative_to(PACKAGE_DIR.resolve()))


def assert_inside_package(path: pathlib.Path) -> pathlib.Path:
    resolved = path.resolve()
    try:
        resolved.relative_to(PACKAGE_DIR.resolve())
    except ValueError as exc:
        raise SystemExit(f"path escapes evidence package: {resolved}") from exc
    return resolved


def contract() -> Dict[str, Any]:
    return read_json(CONTRACT_PATH)


def load_targets(contract_obj: Dict[str, Any]) -> Dict[int, Dict[str, Any]]:
    target_tsv = REPO_ROOT / contract_obj["corpus_identity"]["target_tsv"]["path"]
    expected = contract_obj["corpus_identity"]["target_tsv"]["sha256"]
    actual = sha256_file(target_tsv)
    if actual != expected:
        raise SystemExit(f"target TSV digest mismatch: {actual} != {expected}")
    targets: Dict[int, Dict[str, Any]] = {}
    with target_tsv.open("r", encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f, delimiter="\t")
        for ordinal, row in enumerate(reader):
            targets[ordinal] = {
                "ordinal": ordinal,
                "target_id": row["id"],
                "source_path": row.get("source_path", ""),
                "symbol": row.get("symbol", ""),
                "source_sha256": row.get("source_sha256", ""),
                "byte_start": int(row["byte_start"]),
                "byte_end": int(row["byte_end"]),
            }
    if len(targets) != EXPECTED_DENOMINATOR:
        raise SystemExit(f"target denominator mismatch: {len(targets)}")
    return targets


def ledger_path(run_dir: pathlib.Path, direction: str, contract_obj: Dict[str, Any]) -> pathlib.Path:
    return run_dir / contract_obj["directions"][direction]["ledger"]


def read_ledger(path: pathlib.Path) -> List[Dict[str, Any]]:
    if not path.exists():
        raise SystemExit(f"missing ledger: {path}")
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]


def canonical_digest(obj: Any) -> str:
    return sha256_bytes(json.dumps(obj, sort_keys=True, separators=(",", ":")).encode("utf-8"))


def aggregate_digest(records: List[Dict[str, Any]]) -> str:
    return canonical_digest(records)


def parse_lsp_payload(data: bytes) -> Tuple[Optional[Any], Optional[str]]:
    marker = b"\r\n\r\n"
    if marker not in data:
        return None, "missing_lsp_header_separator"
    header, body = data.split(marker, 1)
    length = None
    for line in header.split(b"\r\n"):
        if line.lower().startswith(b"content-length:"):
            try:
                length = int(line.split(b":", 1)[1].strip())
            except Exception:
                return None, "invalid_content_length"
    if length is None:
        return None, "missing_content_length"
    if len(body) < length:
        return None, "truncated_lsp_body"
    try:
        return json.loads(body[:length].decode("utf-8")), None
    except Exception as exc:  # deterministic string from parser class/message only
        return None, f"json_parse_error:{exc.__class__.__name__}:{exc}"


def result_list(payload: Any) -> Optional[List[Any]]:
    if isinstance(payload, dict) and "result" in payload and isinstance(payload["result"], list):
        return payload["result"]
    return None


def count_ranges_in_result(items: List[Any], *, prepare: bool) -> int:
    total = 0
    for item in items:
        if not isinstance(item, dict):
            continue
        if prepare:
            if isinstance(item.get("range"), dict):
                total += 1
            if isinstance(item.get("selectionRange"), dict):
                total += 1
        else:
            ranges = item.get("fromRanges")
            if isinstance(ranges, list):
                total += len(ranges)
    return total


def count_nodes_in_result(items: List[Any], *, direction: str, prepare: bool) -> int:
    nodes = set()
    for item in items:
        if not isinstance(item, dict):
            continue
        if prepare:
            node = item
        elif direction == "incoming":
            node = item.get("from")
        else:
            node = item.get("to")
        if isinstance(node, dict):
            nodes.add(canonical_digest(node))
    return len(nodes)


def final_events(events: List[Dict[str, Any]], direction: str) -> Dict[int, Dict[str, Any]]:
    finals: Dict[int, List[Dict[str, Any]]] = defaultdict(list)
    for event in events:
        if event.get("direction") != direction:
            raise SystemExit(f"ledger direction contamination: expected {direction}, got {event.get('direction')}")
        if event.get("disposition") in FINAL_DISPOSITIONS:
            finals[int(event["ordinal"])].append(event)
    bad = {ordinal: len(items) for ordinal, items in finals.items() if len(items) != 1}
    if bad:
        raise SystemExit(f"expected one final disposition per ordinal, bad={bad}")
    return {ordinal: items[0] for ordinal, items in finals.items()}


def load_raw_records(run_dir: pathlib.Path, direction: str, final_by_ordinal: Dict[int, Dict[str, Any]]) -> Tuple[List[Dict[str, Any]], Dict[int, List[Dict[str, Any]]], List[str]]:
    raw_dir = run_dir / "raw"
    if not raw_dir.exists():
        raise SystemExit(f"missing raw dir: {raw_dir}")
    contamination: List[str] = []
    other_direction = "outgoing" if direction == "incoming" else "incoming"
    if (raw_dir / other_direction).exists():
        contamination.append(f"raw/{other_direction} exists in {direction} run")
    records: List[Dict[str, Any]] = []
    by_ordinal: Dict[int, List[Dict[str, Any]]] = defaultdict(list)
    sidecars = sorted(raw_dir.glob("**/*.bin.json"), key=lambda p: package_relative(p))
    for sidecar_path in sidecars:
        sidecar = read_json(sidecar_path)
        sidecar_dir = pathlib.Path(sidecar.get("path", "")).parent.name
        bin_path = pathlib.Path(sidecar.get("path", ""))
        if not bin_path.is_absolute():
            bin_path = (PACKAGE_DIR / bin_path).resolve()
        if not bin_path.exists():
            raise SystemExit(f"raw binary missing for sidecar {sidecar_path}: {bin_path}")
        binary_sha = sha256_file(bin_path)
        if binary_sha != sidecar.get("sha256"):
            raise SystemExit(f"raw binary digest mismatch: {bin_path}")
        raw_direction = sidecar.get("direction")
        if raw_direction not in {direction, "generic", "control"}:
            contamination.append(f"{package_relative(sidecar_path)} direction={raw_direction}")
        if raw_direction == other_direction:
            contamination.append(f"{package_relative(sidecar_path)} uses other direction {other_direction}")
        data = bin_path.read_bytes()
        payload, parse_error = parse_lsp_payload(data)
        ordinal = int(sidecar.get("ordinal", -999999))
        disposition = final_by_ordinal.get(ordinal, {}).get("disposition", "NON_TARGET_OR_UNKNOWN")
        target_id = final_by_ordinal.get(ordinal, {}).get("target_id", "UNKNOWN")
        rec = {
            "sidecar_path": package_relative(sidecar_path),
            "raw_path": package_relative(bin_path),
            "sidecar_sha256": sha256_file(sidecar_path),
            "raw_sha256": binary_sha,
            "byte_length": int(sidecar.get("byte_length", len(data))),
            "direction": raw_direction,
            "ordinal": ordinal,
            "target_id": target_id,
            "disposition": disposition,
            "label": sidecar.get("label"),
            "method": sidecar.get("method"),
            "partial": bool(sidecar.get("partial")),
            "sidecar_error": sidecar.get("error"),
            "parse_error": parse_error,
            "has_json_payload": payload is not None,
        }
        items = result_list(payload)
        if items is not None:
            is_prepare = sidecar.get("label") == "textDocument_prepareCallHierarchy.response"
            is_result = sidecar.get("label") == f"callHierarchy_{direction}Calls.response"
            if is_prepare or is_result:
                rec.update({
                    "returned_members": len(items),
                    "returned_ranges": count_ranges_in_result(items, prepare=is_prepare),
                    "returned_occurrences_from_ranges": count_ranges_in_result(items, prepare=False) if is_result else "UNKNOWN",
                    "returned_nodes": count_nodes_in_result(items, direction=direction, prepare=is_prepare),
                })
        records.append(rec)
        if raw_direction == direction and ordinal >= 0:
            by_ordinal[ordinal].append(rec)
    if contamination:
        raise SystemExit("cross-direction contamination: " + "; ".join(sorted(set(contamination))[:20]))
    return records, by_ordinal, contamination


def target_ref(final_by_ordinal: Dict[int, Dict[str, Any]], ordinal: int) -> Dict[str, Any]:
    final = final_by_ordinal.get(ordinal, {})
    return {
        "ordinal": ordinal,
        "target_id": final.get("target_id", "UNKNOWN"),
        "disposition": final.get("disposition", "UNKNOWN"),
    }


def maxima_record(metric: str, candidates: Iterable[Tuple[int, Any, Dict[str, Any]]], final_by_ordinal: Dict[int, Dict[str, Any]]) -> Dict[str, Any]:
    vals: List[Tuple[int, Any, Dict[str, Any]]] = [(ordinal, value, evidence) for ordinal, value, evidence in candidates if isinstance(value, (int, float))]
    if not vals:
        return {"metric": metric, "status": "UNKNOWN", "reason": "metric not present in ledger/raw sidecars"}
    max_value = max(value for _, value, _ in vals)
    targets = []
    for ordinal, value, evidence in vals:
        if value == max_value:
            ref = target_ref(final_by_ordinal, ordinal)
            ref["value"] = value
            ref["evidence"] = evidence
            targets.append(ref)
    targets.sort(key=lambda x: (x["ordinal"], json.dumps(x.get("evidence", {}), sort_keys=True)))
    return {"metric": metric, "status": "PRESENT", "max": max_value, "targets": targets}


def derive_run(direction: str, run_dir: pathlib.Path, *, write_artifacts: bool = True) -> Dict[str, Any]:
    if direction not in {"incoming", "outgoing"}:
        raise SystemExit("direction must be incoming or outgoing; both is deliberately unsupported")
    run_dir = assert_inside_package(run_dir)
    contract_obj = contract()
    targets = load_targets(contract_obj)
    ledger = ledger_path(run_dir, direction, contract_obj)
    # Detect both-ledger CLI/output misuse without merging authority.
    other = "outgoing" if direction == "incoming" else "incoming"
    if ledger_path(run_dir, other, contract_obj).exists():
        raise SystemExit(f"cross-direction ledger present in explicit {direction} run: {other}")
    events = read_ledger(ledger)
    final_by_ord = final_events(events, direction)
    raw_records, raw_by_ord, _ = load_raw_records(run_dir, direction, final_by_ord)

    started_ordinals = {int(e["ordinal"]) for e in events if e.get("disposition") == "STARTED"}
    final_ordinals = set(final_by_ord)
    if final_ordinals != set(range(EXPECTED_DENOMINATOR)):
        missing = sorted(set(range(EXPECTED_DENOMINATOR)) - final_ordinals)
        extra = sorted(final_ordinals - set(range(EXPECTED_DENOMINATOR)))
        raise SystemExit(f"final ordinal coverage mismatch: missing={missing[:10]} extra={extra[:10]}")

    disposition_counts = Counter(e["disposition"] for e in final_by_ord.values())
    event_disposition_counts = Counter(e["disposition"] for e in events)
    batch_manifest_paths = sorted((run_dir / "batches").glob("batch-*.manifest.json"), key=lambda p: p.name)
    if len(batch_manifest_paths) != EXPECTED_BATCH_COUNT:
        raise SystemExit(f"batch manifest count mismatch: {len(batch_manifest_paths)}")
    batch_records = [{"path": package_relative(p), "sha256": sha256_file(p), "data": read_json(p)} for p in batch_manifest_paths]

    elapsed_candidates = []
    events_by_ord: Dict[int, List[Dict[str, Any]]] = defaultdict(list)
    for event in events:
        events_by_ord[int(event["ordinal"])].append(event)
    for ordinal, evs in events_by_ord.items():
        starts = [float(e["timestamp_unix"]) for e in evs if e.get("disposition") == "STARTED"]
        final = final_by_ord.get(ordinal)
        if starts and final and "timestamp_unix" in final:
            elapsed_candidates.append((ordinal, round(float(final["timestamp_unix"]) - min(starts), 6), {"basis": "final.timestamp_unix - first STARTED.timestamp_unix"}))

    def raw_metric(label: Optional[str], field: str, request_or_response: Optional[str] = None) -> List[Tuple[int, Any, Dict[str, Any]]]:
        out = []
        for rec in raw_records:
            if rec["direction"] != direction or rec["ordinal"] < 0:
                continue
            if label is not None and rec.get("label") != label:
                continue
            if request_or_response == "request" and not str(rec.get("label", "")).endswith(".request"):
                continue
            if request_or_response == "response" and ".response" not in str(rec.get("label", "")):
                continue
            value = rec.get(field)
            evidence = {"sidecar_path": rec["sidecar_path"], "raw_path": rec["raw_path"], "label": rec["label"], "raw_sha256": rec["raw_sha256"]}
            out.append((int(rec["ordinal"]), value, evidence))
        return out

    per_target_message_counts = [(ordinal, len(raw_by_ord.get(ordinal, [])), {"basis": "raw sidecar count for direction/ordinal"}) for ordinal in range(EXPECTED_DENOMINATOR)]
    ledger_result_counts = [(ordinal, e.get("result_count"), {"basis": "final ledger result_count"}) for ordinal, e in final_by_ord.items() if "result_count" in e]

    prepare_label = "textDocument_prepareCallHierarchy.response"
    result_label = f"callHierarchy_{direction}Calls.response"
    metrics = {
        "max_prepare_request_bytes": maxima_record("max_prepare_request_bytes", raw_metric("textDocument_prepareCallHierarchy.request", "byte_length"), final_by_ord),
        "max_prepare_response_bytes": maxima_record("max_prepare_response_bytes", raw_metric(prepare_label, "byte_length"), final_by_ord),
        "max_result_request_bytes": maxima_record("max_result_request_bytes", raw_metric(f"callHierarchy_{direction}Calls.request", "byte_length"), final_by_ord),
        "max_result_response_bytes": maxima_record("max_result_response_bytes", raw_metric(result_label, "byte_length"), final_by_ord),
        "max_request_bytes": maxima_record("max_request_bytes", raw_metric(None, "byte_length", "request"), final_by_ord),
        "max_response_bytes": maxima_record("max_response_bytes", raw_metric(None, "byte_length", "response"), final_by_ord),
        "max_message_count_per_target": maxima_record("max_message_count_per_target", per_target_message_counts, final_by_ord),
        "max_elapsed_seconds": maxima_record("max_elapsed_seconds", elapsed_candidates, final_by_ord),
        "max_ledger_result_count": maxima_record("max_ledger_result_count", ledger_result_counts, final_by_ord),
        "max_prepare_returned_members": maxima_record("max_prepare_returned_members", raw_metric(prepare_label, "returned_members"), final_by_ord),
        "max_prepare_returned_ranges": maxima_record("max_prepare_returned_ranges", raw_metric(prepare_label, "returned_ranges"), final_by_ord),
        "max_prepare_returned_nodes": maxima_record("max_prepare_returned_nodes", raw_metric(prepare_label, "returned_nodes"), final_by_ord),
        "max_result_returned_members": maxima_record("max_result_returned_members", raw_metric(result_label, "returned_members"), final_by_ord),
        "max_result_returned_ranges": maxima_record("max_result_returned_ranges", raw_metric(result_label, "returned_ranges"), final_by_ord),
        "max_result_returned_occurrences": maxima_record("max_result_returned_occurrences", raw_metric(result_label, "returned_occurrences_from_ranges"), final_by_ord),
        "max_result_returned_nodes": maxima_record("max_result_returned_nodes", raw_metric(result_label, "returned_nodes"), final_by_ord),
        "max_frontier": {"metric": "max_frontier", "status": "UNKNOWN", "reason": "no frontier field is present in runner ledger or LSP raw sidecars"},
    }

    partial_ordinals = sorted({int(r["ordinal"]) for r in raw_records if r.get("partial") and int(r.get("ordinal", -1)) >= 0})
    malformed_raw = [r for r in raw_records if r.get("parse_error") or "malformed" in str(r.get("label", ""))]
    timeout_raw = [r for r in raw_records if r.get("sidecar_error") == "timeout" or "timeout" in str(r.get("label", ""))]

    category_counts = {
        "denominator": EXPECTED_DENOMINATOR,
        "attempted": len(started_ordinals),
        "unattempted": EXPECTED_DENOMINATOR - len(started_ordinals),
        "one_final_disposition_ordinals": len(final_by_ord),
        "nonempty": disposition_counts.get("SUCCEEDED_NON_EMPTY", 0),
        "empty": disposition_counts.get("SUCCEEDED_EMPTY", 0),
        "fail": sum(disposition_counts.get(d, 0) for d in FAILURE_DISPOSITIONS),
        "unsupported": sum(1 for e in final_by_ord.values() if "UNSUPPORTED" in e.get("disposition", "") or "unsupported" in str(e.get("error", "")).lower()),
        "cancel": sum(1 for e in final_by_ord.values() if "CANCEL" in e.get("disposition", "") or "cancel" in str(e.get("error", "")).lower()),
        "timeout": disposition_counts.get("FAILED_TIMEOUT", 0),
        "refused": sum(disposition_counts.get(d, 0) for d in REFUSAL_DISPOSITIONS),
        "partial": len(partial_ordinals),
        "malformed": len({int(r["ordinal"]) for r in malformed_raw if int(r.get("ordinal", -1)) >= 0}),
    }

    failures = []
    refusals = []
    for ordinal in sorted(final_by_ord):
        final = final_by_ord[ordinal]
        row = {
            "schema": "lsp-trace.adr0011.calls.go.gopls.numeric-bound-terminal-event.v1",
            "direction": direction,
            "ordinal": ordinal,
            "target_id": final["target_id"],
            "disposition": final["disposition"],
            "final_event": final,
            "raw_sidecars": sorted(raw_by_ord.get(ordinal, []), key=lambda r: r["sidecar_path"]),
            "raw_sidecar_count": len(raw_by_ord.get(ordinal, [])),
        }
        if final["disposition"] in FAILURE_DISPOSITIONS:
            failures.append(row)
        if final["disposition"] in REFUSAL_DISPOSITIONS:
            refusals.append(row)

    raw_digest_records = [{k: r[k] for k in ("sidecar_path", "sidecar_sha256", "raw_path", "raw_sha256", "byte_length", "direction", "ordinal", "label", "partial")} for r in raw_records]
    raw_digest_records.sort(key=lambda r: (r["sidecar_path"], r["raw_path"]))

    env_path = run_dir / "provenance" / "environment.json"
    full_plan = run_dir / "full-plan.json"
    provenance = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.numeric-bound-provenance.v1",
        "direction": direction,
        "run_dir": package_relative(run_dir),
        "ledger": {"path": package_relative(ledger), "sha256": sha256_file(ledger), "event_count": len(events), "event_digest": aggregate_digest(events)},
        "raw": {"file_count_including_sidecars_and_bins": len(raw_digest_records) * 2, "sidecar_count": len(raw_digest_records), "aggregate_digest": aggregate_digest(raw_digest_records), "files": raw_digest_records},
        "batches": {"count": len(batch_records), "aggregate_digest": aggregate_digest([{k: v for k, v in b.items() if k != "data"} for b in batch_records]), "manifests": [{k: v for k, v in b.items() if k != "data"} for b in batch_records]},
        "environment": {"status": "PRESENT" if env_path.exists() else "UNKNOWN", "path": package_relative(env_path) if env_path.exists() else "UNKNOWN", "sha256": sha256_file(env_path) if env_path.exists() else "UNKNOWN", "data": read_json(env_path) if env_path.exists() else "UNKNOWN"},
        "full_plan": {"status": "PRESENT" if full_plan.exists() else "UNKNOWN", "path": package_relative(full_plan) if full_plan.exists() else "UNKNOWN", "sha256": sha256_file(full_plan) if full_plan.exists() else "UNKNOWN", "data": read_json(full_plan) if full_plan.exists() else "UNKNOWN"},
        "contract": {"path": package_relative(CONTRACT_PATH), "sha256": sha256_file(CONTRACT_PATH), "schema_sha256": sha256_file(PACKAGE_DIR / "schemas" / "contract.schema.json"), "corpus_identity": contract_obj["corpus_identity"], "authority": contract_obj["authority"]},
        "script": {"path": package_relative(pathlib.Path(__file__)), "sha256": sha256_file(pathlib.Path(__file__))},
    }

    bounds = {
        "schema": RUN_ARTIFACT_SCHEMA,
        "direction": direction,
        "run_dir": package_relative(run_dir),
        "authority": {"evidence_authority": 0, "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED", "profile_go": "NOT_REQUESTED", "completeness": "UNKNOWN"},
        "denominator": EXPECTED_DENOMINATOR,
        "batch_count": EXPECTED_BATCH_COUNT,
        "category_counts": category_counts,
        "final_disposition_counts": dict(sorted(disposition_counts.items())),
        "ledger_event_disposition_counts": dict(sorted(event_disposition_counts.items())),
        "raw_message_counts": dict(sorted(Counter(r["label"] for r in raw_records).items())),
        "raw_target_message_count": sum(1 for r in raw_records if r["direction"] == direction and r["ordinal"] >= 0),
        "raw_control_or_generic_message_count": sum(1 for r in raw_records if r["direction"] in {"control", "generic"} or r["ordinal"] < 0),
        "metrics": metrics,
        "unknown_metrics": {"frontier": "UNKNOWN: no frontier field in exact ledger/raw sidecars"},
    }

    summary = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.final-summary.v1",
        "direction": direction,
        "run_dir": package_relative(run_dir),
        "status": "NUMERIC_BOUNDS_AUTHORED_NOT_GO",
        "denominator_verified": category_counts["denominator"] == EXPECTED_DENOMINATOR,
        "one_final_disposition_per_ordinal_verified": category_counts["one_final_disposition_ordinals"] == EXPECTED_DENOMINATOR,
        "batch_count_verified": len(batch_records) == EXPECTED_BATCH_COUNT,
        "counts": category_counts,
        "final_disposition_counts": dict(sorted(disposition_counts.items())),
        "failure_count": len(failures),
        "refusal_count": len(refusals),
        "ledger_sha256": provenance["ledger"]["sha256"],
        "raw_aggregate_digest": provenance["raw"]["aggregate_digest"],
        "contract_sha256": provenance["contract"]["sha256"],
        "environment_sha256": provenance["environment"]["sha256"],
        "not_granted": ["production_admission", "profileGO", "manager_custody", "completeness"],
    }

    if not write_artifacts:
        return {
            "summary": summary,
            "bounds": bounds,
            "failures": failures,
            "refusals": refusals,
            "provenance": provenance,
        }

    write_json(run_dir / "NUMERIC_BOUNDS.json", bounds)
    write_json(run_dir / "FINAL_SUMMARY.json", summary)
    write_jsonl(run_dir / "FAILURES.jsonl", failures)
    write_jsonl(run_dir / "REFUSALS.jsonl", refusals)
    write_json(run_dir / "PROVENANCE.json", provenance)

    # Seal only after the other deterministic artifacts exist.
    manifest_records = []
    for rel in ["NUMERIC_BOUNDS.json", "FINAL_SUMMARY.json", "FAILURES.jsonl", "REFUSALS.jsonl", "PROVENANCE.json"]:
        p = run_dir / rel
        manifest_records.append({"path": package_relative(p), "sha256": sha256_file(p), "byte_length": p.stat().st_size})
    sealed = {
        "schema": "lsp-trace.adr0011.calls.go.gopls.sealed-manifest.v1",
        "direction": direction,
        "run_dir": package_relative(run_dir),
        "derived_artifacts": manifest_records,
        "ledger_sha256": provenance["ledger"]["sha256"],
        "raw_aggregate_digest": provenance["raw"]["aggregate_digest"],
        "failure_count": len(failures),
        "refusal_count": len(refusals),
        "denominator": EXPECTED_DENOMINATOR,
        "batch_count": EXPECTED_BATCH_COUNT,
    }
    write_json(run_dir / "SEALED_MANIFEST.json", sealed)
    checksum_lines = []
    for rel in DERIVED_FILES:
        p = run_dir / rel
        checksum_lines.append(f"{sha256_file(p)}  {package_relative(p)}")
    (run_dir / CHECKSUM_FILE).write_text("\n".join(checksum_lines) + "\n", encoding="utf-8")
    return summary


def verify_run_artifacts(direction: str, run_dir: pathlib.Path) -> Dict[str, Any]:
    run_dir = assert_inside_package(run_dir)
    errors: List[str] = []
    try:
        expected = derive_run(direction, run_dir, write_artifacts=False)
        expected_summary = expected["summary"]
    except SystemExit as exc:
        raise
    bounds = read_json(run_dir / "NUMERIC_BOUNDS.json")
    summary = read_json(run_dir / "FINAL_SUMMARY.json")
    provenance = read_json(run_dir / "PROVENANCE.json")
    sealed = read_json(run_dir / "SEALED_MANIFEST.json")
    if bounds.get("direction") != direction or summary.get("direction") != direction or provenance.get("direction") != direction or sealed.get("direction") != direction:
        errors.append("artifact direction mismatch")
    if bounds.get("category_counts", {}).get("denominator") != EXPECTED_DENOMINATOR:
        errors.append("missing denominator 5900")
    if bounds.get("category_counts", {}).get("one_final_disposition_ordinals") != EXPECTED_DENOMINATOR:
        errors.append("missing final ordinal")
    if bounds.get("batch_count") != EXPECTED_BATCH_COUNT:
        errors.append("missing batch118")
    if bounds != expected["bounds"]:
        errors.append("bounds are not deterministic re-derivation output")
    if summary != expected_summary:
        errors.append("summary is not deterministic re-derivation output")
    if provenance != expected["provenance"]:
        errors.append("provenance is not deterministic re-derivation output")
    failure_lines = [json.loads(line) for line in (run_dir / "FAILURES.jsonl").read_text(encoding="utf-8").splitlines() if line.strip()]
    refusal_lines = [json.loads(line) for line in (run_dir / "REFUSALS.jsonl").read_text(encoding="utf-8").splitlines() if line.strip()]
    if failure_lines != expected["failures"]:
        errors.append("failures are not deterministic re-derivation output")
    if refusal_lines != expected["refusals"]:
        errors.append("refusals are not deterministic re-derivation output")
    if len(failure_lines) != summary.get("failure_count"):
        errors.append("failure jsonl count mismatch")
    if len(refusal_lines) != summary.get("refusal_count"):
        errors.append("refusal jsonl count mismatch")
    if summary.get("failure_count") != bounds.get("category_counts", {}).get("fail"):
        errors.append("failure count does not match bounds fail count")
    for row in failure_lines:
        if row.get("disposition") not in FAILURE_DISPOSITIONS:
            errors.append(f"non-failure in FAILURES.jsonl ordinal={row.get('ordinal')}")
        if not row.get("raw_sidecars"):
            errors.append(f"failure missing raw sidecars ordinal={row.get('ordinal')}")
    for row in refusal_lines:
        if row.get("disposition") not in REFUSAL_DISPOSITIONS:
            errors.append(f"non-refusal in REFUSALS.jsonl ordinal={row.get('ordinal')}")
    checksum_path = run_dir / CHECKSUM_FILE
    if not checksum_path.exists():
        errors.append("missing derived checksum file")
    else:
        for line in checksum_path.read_text(encoding="utf-8").splitlines():
            digest, rel = line.split("  ", 1)
            p = PACKAGE_DIR / rel
            if not p.exists():
                errors.append(f"checksum path missing: {rel}")
            elif sha256_file(p) != digest:
                errors.append(f"checksum mismatch: {rel}")
    sealed_paths = {r["path"]: r["sha256"] for r in sealed.get("derived_artifacts", [])}
    for rel in ["NUMERIC_BOUNDS.json", "FINAL_SUMMARY.json", "FAILURES.jsonl", "REFUSALS.jsonl", "PROVENANCE.json"]:
        p = run_dir / rel
        pkg_rel = package_relative(p)
        if sealed_paths.get(pkg_rel) != sha256_file(p):
            errors.append(f"sealed manifest missing/mismatched {pkg_rel}")
    other = "outgoing" if direction == "incoming" else "incoming"
    if ledger_path(run_dir, other, contract()).exists() or (run_dir / "raw" / other).exists():
        errors.append("cross-direction contamination present")
    status = "NUMERIC_BOUNDS_VERIFIED" if not errors else "NUMERIC_BOUNDS_VERIFICATION_FAILED"
    if errors:
        raise SystemExit(json.dumps({"status": status, "direction": direction, "run_dir": package_relative(run_dir), "errors": errors}, indent=2, sort_keys=True))
    return {"status": status, "direction": direction, "run_dir": package_relative(run_dir), "failure_count": summary["failure_count"], "refusal_count": summary["refusal_count"], "denominator": EXPECTED_DENOMINATOR, "batch_count": EXPECTED_BATCH_COUNT}


def derive_workload(incoming_run_dir: pathlib.Path, outgoing_run_dir: pathlib.Path, *, write_artifacts: bool = True) -> Dict[str, Any]:
    incoming_run_dir = assert_inside_package(incoming_run_dir)
    outgoing_run_dir = assert_inside_package(outgoing_run_dir)
    incoming = verify_run_artifacts("incoming", incoming_run_dir)
    outgoing = verify_run_artifacts("outgoing", outgoing_run_dir)
    contract_obj = contract()
    readme = PACKAGE_DIR / "README.md"
    runner = PACKAGE_DIR / "adr0011_workload_runner.py"
    metadata = {
        "schema": WORKLOAD_SCHEMA,
        "status": "WORKLOAD_BOUNDS_AUTHORED_NOT_GO",
        "authority_policy": {
            "directions_are_separate_authority_streams": True,
            "direction_authority_merged": False,
            "evidence_authority": 0,
            "production_admission": "REFUSED_NOT_REQUESTED_NOT_GRANTED",
            "profile_go": "NOT_REQUESTED",
            "composition_admission": "NOT_REQUESTED_NOT_GRANTED",
        },
        "direction_refs": {
            "incoming": {
                "run_dir": package_relative(incoming_run_dir),
                "summary_path": package_relative(incoming_run_dir / "FINAL_SUMMARY.json"),
                "summary_sha256": sha256_file(incoming_run_dir / "FINAL_SUMMARY.json"),
                "bounds_path": package_relative(incoming_run_dir / "NUMERIC_BOUNDS.json"),
                "bounds_sha256": sha256_file(incoming_run_dir / "NUMERIC_BOUNDS.json"),
                "verification": incoming,
            },
            "outgoing": {
                "run_dir": package_relative(outgoing_run_dir),
                "summary_path": package_relative(outgoing_run_dir / "FINAL_SUMMARY.json"),
                "summary_sha256": sha256_file(outgoing_run_dir / "FINAL_SUMMARY.json"),
                "bounds_path": package_relative(outgoing_run_dir / "NUMERIC_BOUNDS.json"),
                "bounds_sha256": sha256_file(outgoing_run_dir / "NUMERIC_BOUNDS.json"),
                "verification": outgoing,
            },
        },
        "recursive_presumption_ledger": {
            "readme_path": package_relative(readme),
            "readme_sha256": sha256_file(readme),
            "presumptions_are_references_not_new_authority": True,
            "ledger_rows_ref": "README.md#Presumed / Revised-if / Depends-on ledger",
        },
        "evidence_contract_hashes": {
            "contract_json_sha256": sha256_file(CONTRACT_PATH),
            "contract_schema_sha256": sha256_file(PACKAGE_DIR / "schemas" / "contract.schema.json"),
            "runner_sha256": sha256_file(runner),
            "numeric_deriver_sha256": sha256_file(pathlib.Path(__file__)),
        },
        "corpus_identity": contract_obj["corpus_identity"],
        "environment_and_gopls_identity_refs": {
            "incoming_environment_path": package_relative(incoming_run_dir / "provenance" / "environment.json"),
            "incoming_environment_sha256": sha256_file(incoming_run_dir / "provenance" / "environment.json"),
            "outgoing_environment_path": package_relative(outgoing_run_dir / "provenance" / "environment.json"),
            "outgoing_environment_sha256": sha256_file(outgoing_run_dir / "provenance" / "environment.json"),
        },
        "forbidden_or_not_granted": ["profileGO", "production admission", "composition admission", "merged bidirectional authority"],
    }
    if not write_artifacts:
        return metadata
    out_path = PACKAGE_DIR / "WORKLOAD_REVIEW_METADATA.json"
    write_json(out_path, metadata)
    checksum_targets = [out_path, incoming_run_dir / "SEALED_MANIFEST.json", outgoing_run_dir / "SEALED_MANIFEST.json", incoming_run_dir / CHECKSUM_FILE, outgoing_run_dir / CHECKSUM_FILE]
    lines = [f"{sha256_file(p)}  {package_relative(p)}" for p in checksum_targets]
    (PACKAGE_DIR / "WORKLOAD_REVIEW_CHECKSUMS.sha256").write_text("\n".join(lines) + "\n", encoding="utf-8")
    return metadata


def verify_workload(incoming_run_dir: pathlib.Path, outgoing_run_dir: pathlib.Path) -> Dict[str, Any]:
    metadata = derive_workload(incoming_run_dir, outgoing_run_dir, write_artifacts=False)
    existing_metadata_path = PACKAGE_DIR / "WORKLOAD_REVIEW_METADATA.json"
    existing_metadata = read_json(existing_metadata_path) if existing_metadata_path.exists() else None
    errors: List[str] = []
    if existing_metadata != metadata:
        errors.append("workload metadata is not deterministic re-derivation output")
    if metadata.get("authority_policy", {}).get("direction_authority_merged") is not False:
        errors.append("direction authority is merged")
    for direction, ref in metadata.get("direction_refs", {}).items():
        summary = read_json(PACKAGE_DIR / ref["summary_path"])
        if summary.get("direction") != direction:
            errors.append(f"{direction} summary direction mismatch")
        if ref.get("summary_sha256") != sha256_file(PACKAGE_DIR / ref["summary_path"]):
            errors.append(f"{direction} summary checksum mismatch")
    checksum_path = PACKAGE_DIR / "WORKLOAD_REVIEW_CHECKSUMS.sha256"
    if not checksum_path.exists():
        errors.append("missing workload checksums")
    else:
        for line in checksum_path.read_text(encoding="utf-8").splitlines():
            digest, rel = line.split("  ", 1)
            p = PACKAGE_DIR / rel
            if not p.exists() or sha256_file(p) != digest:
                errors.append(f"workload checksum mismatch: {rel}")
    status = "WORKLOAD_BOUNDS_VERIFIED_NOT_GO" if not errors else "WORKLOAD_BOUNDS_VERIFICATION_FAILED"
    if errors:
        raise SystemExit(json.dumps({"status": status, "errors": errors}, indent=2, sort_keys=True))
    return {"status": status, "metadata_path": package_relative(existing_metadata_path), "metadata_sha256": sha256_file(existing_metadata_path), "incoming": metadata["direction_refs"]["incoming"]["verification"], "outgoing": metadata["direction_refs"]["outgoing"]["verification"]}


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="cmd", required=True)
    for name in ("derive-run", "verify-run"):
        p = sub.add_parser(name)
        p.add_argument("--direction", required=True, choices=["incoming", "outgoing"])
        p.add_argument("--run-dir", required=True)
    for name in ("derive-workload", "verify-workload"):
        p = sub.add_parser(name)
        p.add_argument("--incoming-run-dir", required=True)
        p.add_argument("--outgoing-run-dir", required=True)
    args = parser.parse_args(argv)
    try:
        if args.cmd == "derive-run":
            result = derive_run(args.direction, pathlib.Path(args.run_dir))
        elif args.cmd == "verify-run":
            result = verify_run_artifacts(args.direction, pathlib.Path(args.run_dir))
        elif args.cmd == "derive-workload":
            result = derive_workload(pathlib.Path(args.incoming_run_dir), pathlib.Path(args.outgoing_run_dir))
        elif args.cmd == "verify-workload":
            result = verify_workload(pathlib.Path(args.incoming_run_dir), pathlib.Path(args.outgoing_run_dir))
        else:
            raise AssertionError(args.cmd)
        print(json.dumps(result, indent=2, sort_keys=True))
        return 0
    except SystemExit as exc:
        if isinstance(exc.code, str):
            print(json.dumps({"status": "NUMERIC_BOUNDS_FAILED", "error": exc.code}, indent=2, sort_keys=True), file=sys.stderr)
            return 2
        raise


if __name__ == "__main__":
    raise SystemExit(main())
