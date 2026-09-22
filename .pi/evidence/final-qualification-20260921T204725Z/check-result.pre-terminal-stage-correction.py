#!/usr/bin/env python3
"""Fail-closed checker for retained gateway, post-session, and execution-accounting evidence."""

import argparse
import json
import re
import sys
from pathlib import Path

SELECTOR = re.compile(r"^g-[0-9a-f]{64}\.selector\.json$")


def fail(message: str) -> None:
    raise ValueError(message)


def load(path: Path, label: str):
    value = json.loads(path.read_text())
    if not isinstance(value, dict):
        fail(f"{label} must be a JSON object")
    return value


def unwrap_session_list(value):
    if isinstance(value.get("structuredContent"), dict):
        value = value["structuredContent"]
    if isinstance(value.get("result"), dict):
        value = value["result"]
    return value


def require_post_workers_zero(value) -> None:
    result = unwrap_session_list(value)
    census = result.get("Census", result.get("census")) if isinstance(result, dict) else None
    if not isinstance(census, dict):
        fail("post-result session-list must retain Census")
    workers = census.get("Workers", census.get("workers"))
    if workers != 0:
        fail(f"post-result session-list Census.Workers must equal integer zero, got {workers!r}")


def require_execution_accounting(value) -> None:
    if value.get("schema_version") != "lsp-trace.qualification-execution-accounting.v1":
        fail("execution accounting has unexpected schema_version")
    if value.get("custody") not in {"HOST_RETAINED", "OPERATOR_RETAINED", "PROVIDER_VERIFIED"}:
        fail("execution accounting must state recognized custody")
    if value.get("workers") != 0:
        fail(f"execution accounting workers must equal integer zero, got {value.get('workers')!r}")
    if value.get("model_invocations") != 0:
        fail(f"execution accounting model_invocations must equal integer zero, got {value.get('model_invocations')!r}")
    if not isinstance(value.get("source"), str) or not value["source"]:
        fail("execution accounting must identify its retained source")


def delegated_envelope(outer: dict) -> dict:
    if outer.get("tool") != "lsp_trace_v1_execute" or outer.get("requested_tool") != "lsp_trace_v1_census":
        fail("outer response is not the delegated census gateway")
    if "delegated_outcome" not in outer or "delegated_is_error" not in outer:
        fail("outer gateway does not foreground delegated outcome")
    raw = outer.get("delegated_envelope")
    if isinstance(raw, str):
        raw = json.loads(raw)
    if not isinstance(raw, dict) or raw.get("tool") != "lsp_trace_v1_census":
        fail("delegated envelope is not a census envelope")
    if outer["delegated_outcome"] != raw.get("outcome") or outer["delegated_is_error"] != raw.get("isError"):
        fail("foregrounded delegated outcome disagrees with delegated envelope")
    return raw


def selector(value) -> bool:
    return isinstance(value, str) and SELECTOR.fullmatch(value) is not None


def paused_v2(delegated, result) -> bool:
    if not (
        delegated.get("envelope_schema_id") == "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-feature-catalog-result.v2.schema.json"
        and delegated.get("outcome") == "PAUSED"
        and delegated.get("operation_status") == "SUCCEEDED"
        and delegated.get("isError") is False
        and result.get("schema_version") == "lsp-trace.census-feature-catalog-result.v2"
    ):
        return False
    catalog = result.get("catalog")
    if not isinstance(catalog, dict):
        return False
    exact = (
        catalog.get("kind") == "ADR_0007_FEATURE_CATALOG"
        and catalog.get("status") == "PAUSED"
        and catalog.get("authority") == 0
        and catalog.get("accepted") is False
        and catalog.get("completeness") == "UNKNOWN"
        and isinstance(catalog.get("request_count"), int)
        and catalog.get("request_count") >= 1
        and catalog.get("preparation_count") == 0
        and all(selector(catalog.get(key)) for key in ("checkpoint_selector", "composite_selector", "catalog_selector"))
    )
    identity = result.get("census_identity")
    census = result.get("census")
    identity_ok = isinstance(identity, dict) and selector(identity.get("selector")) and isinstance(identity.get("digest"), str) and re.fullmatch(r"sha256:[0-9a-f]{64}", identity["digest"]) and isinstance(identity.get("byte_length"), int) and identity["byte_length"] > 0
    return bool(exact and ((census is not None) ^ identity_ok))


def failure_v1(delegated, result) -> bool:
    return (
        delegated.get("outcome") == "COMMITTED_DEGRADED"
        and delegated.get("operation_status") == "SUCCEEDED"
        and delegated.get("isError") is False
        and result.get("schema_version") == "lsp-trace.census-continuation-diagnostic.v1"
        and result.get("code") == "CONTINUATION_CAPTURE_FAILED"
        and result.get("phase") == "CONTINUATION"
        and result.get("stage") == "CAPTURE"
        and result.get("status") == "FAILED"
        and result.get("observed_status") == "FAILED_CAPTURE"
        and result.get("census_commit_preserved") is True
        and result.get("retry") is False
        and result.get("recovery") == "RESTART_FROM_PRESERVED_CENSUS_COMMIT"
    )


def failure_v2(delegated, result) -> bool:
    return (
        delegated.get("envelope_schema_id") == "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-continuation-diagnostic.v2.schema.json"
        and delegated.get("outcome") == "COMMITTED_DEGRADED"
        and delegated.get("operation_status") == "SUCCEEDED"
        and delegated.get("isError") is False
        and result.get("schema_version") == "lsp-trace.census-continuation-diagnostic.v2"
        and result.get("code") == "CONTINUATION_CAPTURE_FAILED"
        and result.get("phase") == "CONTINUATION"
        and result.get("stage") == "CAPTURE"
        and result.get("status") == "FAILED"
        and result.get("observed_stage") == "PROGRAM_C_COMPUTED"
        and result.get("observed_status") == "FAILED_CAPTURE"
        and result.get("last_successful_stage") == "PROGRAM_C_COMPUTED"
        and result.get("terminal_stage") == "CAPTURE"
        and result.get("terminal_status") == "FAILED_CAPTURE"
        and result.get("census_commit_preserved") is True
        and result.get("retry") is False
        and result.get("recovery") == "RESTART_FROM_PRESERVED_CENSUS_COMMIT"
        and result.get("recovery_selector_state") == "RECOVERY_SELECTOR_UNAVAILABLE"
        and isinstance(result.get("preserved_census_selector"), str)
        and bool(result.get("preserved_census_selector"))
        and result.get("authority") == 0
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("result", type=Path)
    parser.add_argument("--post-session-list", type=Path, required=True)
    parser.add_argument("--execution-accounting", type=Path, required=True)
    args = parser.parse_args()

    outer = load(args.result, "gateway result")
    post_sessions = load(args.post_session_list, "post-result session-list")
    accounting = load(args.execution_accounting, "execution accounting")
    require_post_workers_zero(post_sessions)
    require_execution_accounting(accounting)
    delegated = delegated_envelope(outer)
    result = delegated.get("result")
    if not isinstance(result, dict):
        fail("delegated result must be an object")

    classes = [paused_v2(delegated, result), failure_v1(delegated, result), failure_v2(delegated, result)]
    if sum(classes) != 1:
        fail("result must match exactly one strict accepted terminal class")
    verdict = ("ACCEPT_PAUSED_DESCRIBE_REQUESTS_V2", "ACCEPT_TYPED_IMMUTABLE_CONTINUATION_FAILURE_V1", "ACCEPT_TYPED_IMMUTABLE_CONTINUATION_FAILURE_V2")[classes.index(True)]
    print(f"DELEGATED_GATEWAY_OUTCOME {outer['delegated_outcome']}")
    print(f"DELEGATED_GATEWAY_IS_ERROR {str(outer['delegated_is_error']).lower()}")
    print("WORKERS_ZERO retained_post_result_session_list+execution_accounting")
    print("MODEL_INVOCATIONS_ZERO retained_execution_accounting")
    print(f"ACCOUNTING_CUSTODY {accounting['custody']}")
    print(verdict)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, json.JSONDecodeError) as error:
        print(f"RESULT_REJECTED: {error}", file=sys.stderr)
        raise SystemExit(1)
