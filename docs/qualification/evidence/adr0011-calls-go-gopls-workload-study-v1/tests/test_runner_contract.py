import importlib.util
import json
import os
import pathlib
import shutil
import sys
import tempfile
import unittest
from types import SimpleNamespace

PACKAGE = pathlib.Path(__file__).resolve().parents[1]
REPO = PACKAGE.parents[3]
RUNNER_PATH = PACKAGE / "adr0011_workload_runner.py"

spec = importlib.util.spec_from_file_location("adr0011_workload_runner", RUNNER_PATH)
runner = importlib.util.module_from_spec(spec)
assert spec.loader is not None
sys.modules[spec.name] = runner
spec.loader.exec_module(runner)


def target0():
    return runner.load_targets_tsv(PACKAGE / "fixtures" / "targets.smoke.tsv", 1)[0]


class ContractTests(unittest.TestCase):
    def test_contract_check_passes(self):
        result = runner.verify_contract(PACKAGE)
        self.assertEqual(result["status"], "CONTRACT_CHECK_PASSED")
        contract = runner.read_json(PACKAGE / "contract.json")
        self.assertFalse(contract["authority"]["manager_custody_claimed"])
        self.assertEqual(contract["corpus_identity"]["qualification_corpus_commit"], "7f09e5357b117d20449fc16097631e32d4eead7f")
        self.assertNotIn("requested_commit_prefix", contract["corpus_identity"])

    def test_first_target_byte_offset_maps_to_identifier_position(self):
        source = REPO / "acquisitionops" / "executor.go"
        if not source.exists():
            self.skipTest("source workspace unavailable")
        data = source.read_bytes()
        t = target0()
        self.assertEqual(runner.sha256_bytes(data), t.source_sha256)
        self.assertEqual(runner.byte_offset_to_lsp_utf16(data, t.byte_start), {"line": 35, "character": 0})
        symbol_offset = runner.target_symbol_byte_offset(data, t)
        pos = runner.byte_offset_to_lsp_utf16(data, symbol_offset)
        self.assertEqual(symbol_offset, 1122)
        self.assertEqual(pos, {"line": 35, "character": 5})
        self.assertEqual(data[symbol_offset:symbol_offset + len(b"DecodeManifest")], b"DecodeManifest")

    def test_select_prepared_item_requires_uri_symbol_and_range(self):
        t = target0()
        uri = "file:///x/acquisitionops/executor.go"
        pos = {"line": 35, "character": 5}
        good = {"uri": uri, "name": "DecodeManifest", "selectionRange": {"start": {"line": 35, "character": 5}, "end": {"line": 35, "character": 19}}}
        item, status = runner.select_prepared_item([good], t, uri, pos)
        self.assertEqual(status, "OK")
        self.assertIs(item, good)

        wrong_range = {"uri": uri, "name": "DecodeManifest", "selectionRange": {"start": {"line": 34, "character": 0}, "end": {"line": 34, "character": 1}}}
        item, status = runner.select_prepared_item([wrong_range], t, uri, pos)
        self.assertIsNone(item)
        self.assertEqual(status, "REFUSED_SELECTED_ITEM_TARGET_MISMATCH")

        wrong_symbol = {"uri": uri, "name": "Other", "selectionRange": {"start": {"line": 35, "character": 5}, "end": {"line": 35, "character": 19}}}
        item, status = runner.select_prepared_item([wrong_symbol], t, uri, pos)
        self.assertIsNone(item)
        self.assertEqual(status, "REFUSED_SELECTED_ITEM_TARGET_MISMATCH")

    def test_verify_run_accepts_abandoned_then_retry(self):
        root = PACKAGE / "runs" / "unit-verify-abandoned-retry"
        shutil.rmtree(root, ignore_errors=True)
        try:
            ledger = root / "ledgers" / "incoming.attempts.jsonl"
            t = target0()
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "STARTED", 1))
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "ABANDONED", 1))
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "STARTED", 2))
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "SUCCEEDED_EMPTY", 2))
            result = runner.verify_run(root, directions=["incoming"], ordinals=[0])
            self.assertEqual(result["status"], "RUN_VERIFICATION_PASSED")
        finally:
            shutil.rmtree(root, ignore_errors=True)

    def test_verify_run_rejects_any_append_after_final(self):
        root = PACKAGE / "runs" / "unit-verify-append-after-final"
        shutil.rmtree(root, ignore_errors=True)
        try:
            ledger = root / "ledgers" / "incoming.attempts.jsonl"
            t = target0()
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "STARTED", 1))
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "SUCCEEDED_EMPTY", 1))
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "STARTED", 2))
            with self.assertRaisesRegex(runner.ContractError, "append after final disposition"):
                runner.verify_run(root, directions=["incoming"], ordinals=[0])
        finally:
            shutil.rmtree(root, ignore_errors=True)

    def test_verify_run_rejects_retry_without_abandoned(self):
        root = PACKAGE / "runs" / "unit-verify-retry-without-abandoned"
        shutil.rmtree(root, ignore_errors=True)
        try:
            ledger = root / "ledgers" / "incoming.attempts.jsonl"
            t = target0()
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "STARTED", 1))
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "STARTED", 2))
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "SUCCEEDED_EMPTY", 2))
            with self.assertRaises(runner.ContractError):
                runner.verify_run(root, directions=["incoming"], ordinals=[0])
        finally:
            shutil.rmtree(root, ignore_errors=True)

    def test_prepare_attempt_rejects_append_after_final(self):
        with tempfile.TemporaryDirectory() as td:
            root = pathlib.Path(td)
            ledger = root / "ledgers" / "incoming.attempts.jsonl"
            t = target0()
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "STARTED", 1))
            runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "SUCCEEDED_EMPTY", 1))
            with self.assertRaisesRegex(runner.ContractError, "cannot append after final"):
                runner.prepare_attempt(ledger, "incoming", t)

    def test_contract_tamper_guard_rejects_authority_claim(self):
        contract = runner.read_json(PACKAGE / "contract.json")
        contract["authority"]["manager_custody_claimed"] = True
        with tempfile.TemporaryDirectory() as td:
            root = pathlib.Path(td)
            (root / "schemas").mkdir(parents=True)
            runner.write_json(root / "contract.json", contract)
            (root / "schemas" / "contract.schema.json").write_text((PACKAGE / "schemas" / "contract.schema.json").read_text(encoding="utf-8"), encoding="utf-8")
            (root / "fixtures").mkdir()
            (root / "fixtures" / "targets.smoke.tsv").write_text((PACKAGE / "fixtures" / "targets.smoke.tsv").read_text(encoding="utf-8"), encoding="utf-8")
            with self.assertRaises(runner.ContractError):
                runner.verify_contract(root)

    def test_lsp_read_rejects_declared_body_over_max_without_overread(self):
        with tempfile.TemporaryDirectory() as td:
            r, w = os.pipe()
            os.write(w, b"Content-Length: 100\r\n\r\n")
            os.close(w)
            client = object.__new__(runner.LSPClient)
            client.proc = SimpleNamespace(stdout=os.fdopen(r, "rb"))
            client.raw_dir = pathlib.Path(td)
            client.max_raw_response_bytes = 32
            with self.assertRaisesRegex(runner.LSPError, "max_raw_response_bytes"):
                client._read_message("incoming", 0, "probe", 1)
            sidecars = list(pathlib.Path(td).rglob("*.json"))
            self.assertTrue(sidecars)
            self.assertTrue(any(json.loads(p.read_text()).get("partial") for p in sidecars))

    def test_lsp_read_timeout_preserves_partial_record(self):
        with tempfile.TemporaryDirectory() as td:
            r, w = os.pipe()
            self.addCleanup(os.close, w)
            client = object.__new__(runner.LSPClient)
            client.proc = SimpleNamespace(stdout=os.fdopen(r, "rb"))
            client.raw_dir = pathlib.Path(td)
            client.max_raw_response_bytes = 1024
            with self.assertRaises(runner.LSPTimeout):
                client._read_message("incoming", 0, "probe", 0.01)
            sidecars = list(pathlib.Path(td).rglob("*.json"))
            self.assertTrue(sidecars)
            self.assertEqual(json.loads(sidecars[0].read_text()).get("error"), "timeout")

    def test_full_batch_plan_exact_denominator_and_digests(self):
        contract = runner.read_json(PACKAGE / "contract.json")
        targets = runner.load_exact_full_targets(contract)
        batches = runner.batch_records(targets)
        self.assertEqual(len(targets), 5900)
        self.assertEqual(len(batches), 118)
        self.assertEqual(batches[0]["start_ordinal"], 0)
        self.assertEqual(batches[0]["end_ordinal_inclusive"], 49)
        self.assertEqual(batches[-1]["start_ordinal"], 5850)
        self.assertEqual(batches[-1]["end_ordinal_inclusive"], 5899)
        run_dir = PACKAGE / "runs" / "unit-full-plan"
        shutil.rmtree(run_dir, ignore_errors=True)
        try:
            result = runner.write_full_plan(run_dir, "incoming", targets, contract, dry_run=True)
            self.assertEqual(result["status"], "FULL_PLAN_VALIDATED")
            self.assertEqual(len(list((run_dir / "batches").glob("*.manifest.json"))), 118)
            self.assertEqual(len(list((run_dir / "checkpoints").glob("*.checkpoint.json"))), 118)
            first = runner.read_json(run_dir / "batches" / "batch-000.manifest.json")
            self.assertEqual(first["target_count"], 50)
            self.assertEqual(first["output_digest"], runner.sha256_bytes(b""))
        finally:
            shutil.rmtree(run_dir, ignore_errors=True)

    def test_full_mode_requires_direction(self):
        args = SimpleNamespace(direction="both", run_dir=str(PACKAGE / "runs" / "unit-invalid"), full_targets=None)
        with self.assertRaisesRegex(runner.ContractError, "direction"):
            runner.run_full_workload(args, dry_run=True)

    def test_semantic_invariant_tamper_examples_rejected(self):
        contract = runner.read_json(PACKAGE / "contract.json")
        tampered = json.loads(json.dumps(contract))
        tampered["corpus_identity"]["target_count"] = 1
        self.assertTrue(any("target_count" in e for e in runner.semantic_invariant_errors(tampered)))
        tampered = json.loads(json.dumps(contract))
        tampered["limits"]["batch_size"] = 999
        self.assertTrue(any("batch_size" in e for e in runner.semantic_invariant_errors(tampered)))
        tampered = json.loads(json.dumps(contract))
        tampered["limits"]["smoke_max_targets_hard"] = 5900
        self.assertTrue(any("smoke_max_targets_hard" in e for e in runner.semantic_invariant_errors(tampered)))
        tampered = json.loads(json.dumps(contract))
        tampered["limits"]["max_raw_response_bytes"] = 1
        self.assertTrue(any("max_raw_response_bytes" in e for e in runner.semantic_invariant_errors(tampered)))

    def _smoke_args(self, run_dir):
        return SimpleNamespace(
            run_dir=str(run_dir),
            workspace=str(REPO),
            gopls="definitely-missing-gopls",
            max_targets=1,
            targets=str(PACKAGE / "fixtures" / "targets.smoke.tsv"),
            direction="incoming",
        )

    def _full_args(self, run_dir, direction="incoming"):
        return SimpleNamespace(
            run_dir=str(run_dir),
            direction=direction,
            full_targets=None,
            workspace=str(REPO),
            gopls="definitely-missing-gopls",
        )

    def _write_minimal_verified_run(self, run_dir):
        ledger = pathlib.Path(run_dir) / "ledgers" / "incoming.attempts.jsonl"
        t = target0()
        runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "STARTED", 1))
        runner.append_jsonl(ledger, runner.ledger_event("incoming", t, "SUCCEEDED_EMPTY", 1))

    def test_mode_paths_reject_outside_package(self):
        with tempfile.TemporaryDirectory() as td:
            outside = pathlib.Path(td) / "outside-run"
            with self.assertRaises(runner.ContractError):
                runner.run_smoke(self._smoke_args(outside))
            with self.assertRaises(runner.ContractError):
                runner.run_full_workload(self._full_args(outside), dry_run=True)
            with self.assertRaises(runner.ContractError):
                runner.run_full_workload(self._full_args(outside), dry_run=False)
            with self.assertRaises(runner.ContractError):
                runner.verify_run(outside, directions=["incoming"], ordinals=[0])

    def test_mode_paths_reject_symlink_escape(self):
        with tempfile.TemporaryDirectory() as td:
            outside = pathlib.Path(td) / "outside-target"
            outside.mkdir()
            link = PACKAGE / "runs" / "unit-symlink-escape"
            link.parent.mkdir(exist_ok=True)
            if link.exists() or link.is_symlink():
                if link.is_dir() and not link.is_symlink():
                    shutil.rmtree(link)
                else:
                    link.unlink()
            link.symlink_to(outside, target_is_directory=True)
            try:
                with self.assertRaises(runner.ContractError):
                    runner.run_smoke(self._smoke_args(link))
                with self.assertRaises(runner.ContractError):
                    runner.run_full_workload(self._full_args(link), dry_run=True)
                with self.assertRaises(runner.ContractError):
                    runner.run_full_workload(self._full_args(link), dry_run=False)
                with self.assertRaises(runner.ContractError):
                    runner.verify_run(link, directions=["incoming"], ordinals=[0])
            finally:
                if link.exists() or link.is_symlink():
                    link.unlink()

    def test_mode_paths_accept_internal_dirs(self):
        smoke_dir = PACKAGE / "runs" / "unit-smoke-internal"
        dry_dir = PACKAGE / "runs" / "unit-dry-internal"
        verify_dir = PACKAGE / "runs" / "unit-verify-internal"
        full_dir = PACKAGE / "runs" / "unit-full-internal"
        for path in [smoke_dir, dry_dir, verify_dir, full_dir]:
            shutil.rmtree(path, ignore_errors=True)
        try:
            smoke = runner.run_smoke(self._smoke_args(smoke_dir))
            self.assertEqual(smoke["verification"]["status"], "RUN_VERIFICATION_PASSED")
            dry = runner.run_full_workload(self._full_args(dry_dir), dry_run=True)
            self.assertEqual(dry["status"], "FULL_PLAN_VALIDATED")
            self._write_minimal_verified_run(verify_dir)
            verified = runner.verify_run(verify_dir, directions=["incoming"], ordinals=[0])
            self.assertEqual(verified["status"], "RUN_VERIFICATION_PASSED")
            self.assertEqual(runner.confined_run_dir(full_dir), full_dir.resolve())
        finally:
            for path in [smoke_dir, dry_dir, verify_dir, full_dir]:
                shutil.rmtree(path, ignore_errors=True)


if __name__ == "__main__":
    unittest.main()
