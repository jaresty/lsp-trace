#!/usr/bin/env python3
import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("check_result", Path(__file__).with_name("check-adr0007-census-qualification-result.py"))
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)

SELECTOR = "g-" + "a" * 64 + ".selector.json"
BASE = {
    "envelope_schema_id": "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-feature-catalog-result.v2.schema.json",
    "outcome": "PAUSED", "operation_status": "SUCCEEDED", "isError": False,
}


def result(request_count=3, preparation_count=2, workers=0, models=0):
    catalog = {
        "kind": "ADR_0007_FEATURE_CATALOG", "status": "PAUSED", "authority": 0,
        "accepted": False, "completeness": "UNKNOWN", "request_count": request_count,
        "preparation_count": preparation_count, "resume_guidance": checker.__dict__.get("RESUME_GUIDANCE", "Resume with this selector and omit stop_after to continue exactly the remaining work once."),
        "checkpoint_selector": SELECTOR, "composite_selector": SELECTOR, "catalog_selector": SELECTOR,
    }
    return {**BASE, "result": {"schema_version": "lsp-trace.census-feature-catalog-result.v2", "catalog": catalog, "census": {}, "workers": workers, "model_invocations": models}}


class PreparationPolicyTest(unittest.TestCase):
    def test_valid_counts(self):
        for request_count, preparation_count in ((3, 2), (163, 156)):
            self.assertTrue(checker.paused_v2(result(request_count, preparation_count), result(request_count, preparation_count)["result"]))

    def test_invalid_counts(self):
        for preparation_count in (-1, 4, 1.5, None):
            self.assertFalse(checker.paused_v2(result(preparation_count=preparation_count), result(preparation_count=preparation_count)["result"]))

    def test_workers_and_models_are_independent(self):
        self.assertTrue(checker.paused_v2(result(workers=1, models=1), result(workers=1, models=1)["result"]))


if __name__ == "__main__":
    unittest.main()
