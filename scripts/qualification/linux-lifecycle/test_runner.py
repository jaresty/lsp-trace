#!/usr/bin/env python3
"""Synthetic-only qualification checks. Never spawn Go, native fixtures or networks."""
import importlib.util
import difflib
import errno
import itertools
import json
import os
from pathlib import Path
import tempfile
import unittest
from contextlib import ExitStack
from types import SimpleNamespace
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("qualification_runner", Path(__file__).with_name("runner.py"))
r = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(r)


def transcript(variant, trace=None):
    root = r.selection(variant)
    negative = variant in ("old-snapshot", "early-wait", "early-fence")
    outcome = "fail" if negative else "pass"
    records = []

    def event(action, test="", output=None):
        value = {"Time": "2026-11-27T00:00:00Z", "Action": action, "Package": r.PACKAGE}
        if test:
            value["Test"] = test
        if output is not None:
            value["Output"] = output + "\n"
        if action in ("pass", "fail"):
            value["Elapsed"] = 0.01
        records.append(value)

    def log(test, text):
        event("output", test, text)

    def run(test):
        event("run", test)
        log(test, "=== RUN   " + test)

    def end(test):
        # JSON-mode source format; independently witnessed below, not inferred
        # from the classifier. Diagnostic indentation is a separate contract.
        log(test, "--- " + outcome.upper() + ": " + test + " (0.01s)")
        event(outcome, test)

    event("start")
    run(root)
    if root == r.SNAP:
        for child in r.CHILDREN:
            test = root + "/" + child
            run(test)
            if negative:
                log(test, "    process_linux_supervision_test.go:419: ASSERT_LINUX_SNAPSHOT_COMPLETION_COHERENT: " + r.SNAP_PAYLOAD)
            end(test)
    else:
        if trace is None:
            indices = {"early-wait": (0, 1, 2, 5, 6, 3, 4), "early-fence": (0, 1, 4, 2, 5, 6, 3)}.get(variant, tuple(range(7)))
            trace = [r.EVENTS[i] for i in indices]
        formatted = "[" + " ".join(trace) + "]"
        loc = 482 if variant == "candidate-original" else 534
        prefix = "    process_linux_supervision_test.go:"
        if negative:
            log(root, prefix + "521: CONTROL_ACTUAL_WAIT_ENTERED_WHILE_SIGNAL_HELD")
            log(root, prefix + "524: ASSERT_LINUX_FINAL_RETURN_REQUIRED_WHILE_HELD")
            pos = [trace.index(e) for e in r.EVENTS]
            for a, b in r.EDGES:
                if pos[a] >= pos[b]:
                    log(root, prefix + str(loc) + ": ASSERT_LINUX_FINAL_RETURN_FENCE_WAIT_ORDER: " + formatted)
        log(root, prefix + str(loc) + ": LINUX_ORDERED_EVENTS=" + formatted)
    end(root)
    log("", outcome.upper())
    log("", ("FAIL\t" if negative else "ok  \t") + r.PACKAGE + "\t0.010s")
    event(outcome)
    return records


def encoded(records):
    return b"".join(json.dumps(row, ensure_ascii=False).encode() + b"\n" for row in records)


def classify(variant, records=None, **kwargs):
    kwargs.setdefault("returncode", 1 if variant in ("old-snapshot", "early-wait", "early-fence") else 0)
    return r.classify_synthetic([encoded(records if records is not None else transcript(variant))], variant=variant, **kwargs)


class ClassifierTests(unittest.TestCase):
    def reject(self, variant, records=None, **kwargs):
        self.assertEqual(classify(variant, records, **kwargs)["verdict"], "UNQUALIFIED")

    def test_all_six_synthetic_transcripts(self):
        for name in r.SEQUENCE:
            with self.subTest(name=name):
                got = classify(name)
                self.assertEqual(got["origin"], "SYNTHETIC")
                self.assertEqual(got["verdict"], "PASS" if name in r.SEQUENCE[:3] else "INTENDED_FAIL")

    def test_every_byte_boundary(self):
        for name in r.SEQUENCE:
            data = encoded(transcript(name))
            got = r.classify_synthetic((bytes([byte]) for byte in data), variant=name, returncode=0 if name in r.SEQUENCE[:3] else 1)
            self.assertNotEqual(got["verdict"], "UNQUALIFIED", got)

    def test_split_unicode_decoder_then_unknown_output(self):
        rows = transcript("candidate-original")
        rows[3]["Output"] = "unknown λ\n"
        raw = encoded(rows)
        # UTF-8 is decoded intact, then unknown source grammar is rejected.
        records = list(r.json_events(bytes([b]) for b in raw))
        self.assertEqual(records[3]["Output"], "unknown λ\n")
        self.reject("candidate-original", records)

    def test_arbitrary_output_chunks_and_identical_chunks(self):
        rows = transcript("early-fence")
        split = []
        for row in rows:
            if row["Action"] == "output":
                # Many identical ordinary one-character Output records, e.g. spaces.
                split.extend(dict(row, Output=char) for char in row["Output"])
            else:
                split.append(row)
        self.assertEqual(classify("early-fence", split)["verdict"], "INTENDED_FAIL")

    def test_edge_assertions_have_exact_multiplicity(self):
        # This trace violates final-return < fence-publication, exactly once.
        rows = transcript("early-fence")
        matches = [i for i, row in enumerate(rows) if "FENCE_WAIT_ORDER:" in row.get("Output", "")]
        self.assertEqual(len(matches), 1)
        self.reject("early-fence", rows[:matches[0]] + rows[matches[0] + 1:])
        self.reject("early-fence", rows[:matches[0]] + [rows[matches[0]]] + rows[matches[0]:])

    def test_every_record_removal_is_rejected(self):
        for name in r.SEQUENCE:
            rows = transcript(name)
            for index in range(len(rows)):
                with self.subTest(variant=name, removed=index):
                    self.reject(name, rows[:index] + rows[index + 1:])

    def test_every_record_duplication_is_rejected(self):
        for name in r.SEQUENCE:
            rows = transcript(name)
            for index in range(len(rows)):
                with self.subTest(variant=name, duplicate=index):
                    self.reject(name, rows[:index] + [rows[index]] + rows[index:])

    def test_adjacent_reordering(self):
        for name in r.SEQUENCE:
            rows = transcript(name)
            for index in range(len(rows) - 1):
                if rows[index] == rows[index + 1]:
                    continue  # identical ordinary diagnostics cannot be distinguished
                bad = rows.copy()
                bad[index], bad[index + 1] = bad[index + 1], bad[index]
                with self.subTest(variant=name, swap=index):
                    self.reject(name, bad)

    def test_process_status_invalidates_complete_witness(self):
        for name in r.SEQUENCE:
            for kw in ({"timed_out": True}, {"overflow": True}, {"cleanup_ok": False}, {"returncode": -9}, {"returncode": 2}, {"stderr": b"compiler diagnostic\n"}):
                with self.subTest(variant=name, status=kw):
                    self.reject(name, **kw)
            self.reject(name, returncode=1 if name in r.SEQUENCE[:3] else 0)

    def test_late_failure_unknown_output(self):
        for text in ("panic: deadlock", "WARNING: DATA RACE", "fixture join timeout: cleanup", "CONTROL_RENDEZVOUS_UNAVAILABLE", "CONTROL_RENDEZVOUS_INVALID", "[build failed]", "ordinary unknown compiler warning", "ASSERT_OTHER", '"CONTROL_ACTUAL_WAIT_ENTERED_WHILE_SIGNAL_HELD"'):
            rows = transcript("early-wait")
            rows.insert(-4, {"Action": "output", "Package": r.PACKAGE, "Test": r.ORDER, "Output": text + "\n"})
            with self.subTest(text=text):
                self.reject("early-wait", rows)

    def test_wrong_stream_for_every_output(self):
        for name in r.SEQUENCE:
            rows = transcript(name)
            for i, row in enumerate(rows):
                if row["Action"] != "output":
                    continue
                bad = [dict(v) for v in rows]
                bad[i]["Test"] = "wrong/root"
                with self.subTest(variant=name, output=i):
                    self.reject(name, bad)

    def test_markers_with_fabricated_location_or_payload(self):
        for name in ("old-snapshot", "early-wait"):
            rows = transcript(name)
            index = next(i for i, v in enumerate(rows) if "process_linux_supervision_test.go:" in v.get("Output", ""))
            for transform in (lambda x: '"' + x.rstrip() + '"\n', lambda x: x.replace(".go:", ".go:9"), lambda x: x.replace("COMPLETE", "FAILED"), lambda x: "prefix " + x):
                bad = [dict(v) for v in rows]
                altered = transform(bad[index]["Output"])
                if altered == bad[index]["Output"]:
                    continue
                bad[index]["Output"] = altered
                self.reject(name, bad)

    def test_foreign_skip_build_actions_and_extra_root(self):
        for action in ("skip", "bench", "pause", "cont", "build-output", "build-fail"):
            rows = transcript("candidate-original")
            rows[1] = dict(rows[1], Action=action)
            self.reject("candidate-original", rows)
        rows = transcript("candidate-original")
        self.reject("candidate-original", rows + rows)
        rows[0]["Package"] = "foreign"
        self.reject("candidate-original", rows)

    def test_swapped_variant_logs(self):
        for left, right in itertools.permutations(r.SEQUENCE, 2):
            # snapshot pass vs fail and original/paired log locations are distinct.
            if {left, right} == {"early-wait", "early-fence"} or r.selection(left) != r.selection(right) or left in r.SEQUENCE[:3]:
                self.reject(left, transcript(right))

    def test_valid_scheduling_variations(self):
        # Wait-return is not ordered against final-return/fence by the contract.
        for indices in ((0, 1, 2, 5, 3, 6, 4), (0, 1, 2, 5, 3, 4, 6)):
            trace = [r.EVENTS[i] for i in indices]
            self.assertEqual(classify("early-wait", transcript("early-wait", trace))["verdict"], "INTENDED_FAIL")

    def test_wrong_trace_relations_and_replayed_trace(self):
        rows = transcript("early-wait")
        for i, row in enumerate(rows):
            if "LINUX_ORDERED_EVENTS=" in row.get("Output", ""):
                for mutation in (row["Output"].replace("wait-entry", "observer-return"), row["Output"].replace("final-signal-return fence-publication", "fence-publication final-signal-return"), row["Output"].replace("wait-entry wait-return", "wait-return wait-entry")):
                    bad = [dict(v) for v in rows]
                    bad[i]["Output"] = mutation
                    self.reject("early-wait", bad)

    def test_json_malformed_duplicate_partial_and_nonfinite(self):
        raw = encoded(transcript("candidate-original"))
        for data in (raw[:-1], raw[:-10], raw + b"oops\n", raw.replace(b'"Action": "start"', b'"Action":"start","Action":"start"', 1), raw.replace(b'0.01', b'NaN'), raw + b'\xff\n'):
            self.assertEqual(r.classify_synthetic([data], variant="candidate-original")["verdict"], "UNQUALIFIED")

    def test_marker_only(self):
        self.reject("early-wait", [{"Action": "output", "Package": r.PACKAGE, "Test": r.ORDER, "Output": "CONTROL_ACTUAL_WAIT_ENTERED_WHILE_SIGNAL_HELD\n"}])


class CustodyTests(unittest.TestCase):
    def test_pins_fail_closed_before_any_subprocess(self):
        with patch.dict(os.environ, {}, clear=True), patch.object(r.subprocess, "Popen", side_effect=AssertionError("must not launch")):
            with self.assertRaisesRegex(r.Unqualified, "UNRESOLVED"):
                r.activate()

    def test_commands_exact_six_modes(self):
        for mode in ("normal", "race"):
            for variant in r.SEQUENCE:
                argv = r.command(Path("/tool/go"), Path("/q"), mode, variant)
                self.assertIn("^" + r.selection(variant) + "$", argv)
                self.assertEqual(argv[-1], "./internal/managedprocess")
                self.assertEqual("-race" in argv, mode == "race")
                self.assertIn("-count=1", argv)
                self.assertNotIn("./...", argv)
        with self.assertRaises(r.Unqualified):
            r.command(Path("/go"), Path("/q"), "normal", "; touch /tmp/injected")

    def test_environment_drops_inherited_flags(self):
        with patch.dict(os.environ, {"GOFLAGS": "-run=.*", "LD_PRELOAD": "/evil", "GORACE": "evil"}):
            env = r.environment(Path("/q"), Path("/go/bin/go"), "normal")
            self.assertFalse({"GOFLAGS", "LD_PRELOAD", "GORACE"} & env.keys())
            self.assertEqual(env["GOPROXY"], "off")
            self.assertEqual(env["GOTOOLCHAIN"], "local")

    def test_pair_digest_encoding_and_mutation(self):
        pair = {"head": r.BASE, "production_sha256": "p", "test_sha256": "t", "selection": "^" + r.ORDER + "$"}
        self.assertNotEqual(r.pair_hash(pair), r.digest(json.dumps(pair, sort_keys=True, separators=(",", ":")).encode()))
        changed = dict(pair, test_sha256="p", production_sha256="t")
        self.assertNotEqual(r.pair_hash(pair), r.pair_hash(changed))
        with self.assertRaises(r.Unqualified):
            r.pair_hash(dict(pair, extra=True))

    def test_fixture_inventory_and_hashes(self):
        fixtures = Path(__file__).parent / "fixtures/39c57a81"
        for name, value in r.DOC_HASHES.items():
            self.assertEqual(r.digest((fixtures / name).read_bytes()), value)
        manifest = r.decode_json((fixtures / "manifest.json").read_bytes())
        self.assertEqual(len(manifest["variants"]), 5)
        names = set(r.DOC_HASHES)
        for row in manifest["variants"]:
            self.assertEqual(r.pair_hash(row["pair"]), row["pair_sha256"])
            for kind in ("patch", "overlay"):
                name = Path(row[kind]).name
                names.add(name)
                self.assertEqual(r.digest((fixtures / name).read_bytes()), row[kind + "_sha256"])
            for change in row["changed_external_paths"]:
                name = Path(change["external_path"]).name + ".fixture"
                names.add(name)
                self.assertEqual(r.digest((fixtures / name).read_bytes()), change["controlled_sha256"])
        self.assertEqual(len(names), 19)
        self.assertEqual(names, {p.name for p in fixtures.iterdir()})
        self.assertFalse(any(name.endswith(".go") for name in names))

    def test_remapping_and_post_capture_pair_recheck(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            candidate, fixtures, q = (root / n for n in ("candidate", "fixtures", "q"))
            for path in (candidate, fixtures, q):
                path.mkdir()
            for name in (r.P, r.T):
                path = candidate / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(name.encode())
            for member in r.MEMBERS:
                (fixtures / (member + ".fixture")).write_bytes(member.encode())
            bound = {}
            for variant in r.SEQUENCE:
                mapping = r.REPLACEMENTS.get(variant, {})
                pair = {"head": r.BASE, "selection": "^" + r.selection(variant) + "$"}
                for path, key in ((r.P, "production_sha256"), (r.T, "test_sha256")):
                    pair[key] = r.digest(mapping.get(path, path).encode())
                bound[variant] = {"pair": pair, "pair_sha256": r.pair_hash(pair)}
            r.remap(fixtures, candidate, q, bound)
            r.verify_effective(candidate, bound)
            overlay = q / "maps/early-wait.json"
            original = overlay.read_bytes()
            wrong = r.decode_json(original)
            wrong["Replace"][str(candidate / r.P)] = r.OLD_CONTROL + "/early-wait.go"
            overlay.write_text(json.dumps(wrong))
            with self.assertRaises(r.Unqualified):
                r.verify_effective(candidate, bound)
            # Even if an attacker updates the overlay digest, exact map is checked.
            bound["early-wait"]["overlay_sha256"] = r.digest(overlay.read_bytes())
            with self.assertRaises(r.Unqualified):
                r.verify_effective(candidate, bound)
            overlay.write_bytes(original)
            bound["early-wait"]["overlay_sha256"] = r.digest(original)
            r.verify_effective(candidate, bound)
            (q / "controls/early-wait.go").write_bytes(b"changed")
            with self.assertRaises(r.Unqualified):
                r.verify_effective(candidate, bound)

    def test_expired_capture_does_not_spawn_and_retains_failure(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(r.subprocess, "Popen", side_effect=AssertionError("must not spawn")):
            artifact = Path(tmp).resolve() / "capture"
            out, err, receipt = r.capture(["/never-executed"], {}, Path(tmp), -1, artifact, {"mode": "normal"})
            self.assertEqual(out, b"")
            self.assertEqual(err, b"")
            self.assertIsNone(receipt["status"]["returncode"])
            self.assertFalse(receipt["status"]["cleanup_ok"])
            self.assertEqual(receipt["origin"], "SUPERVISOR")
            self.assertEqual(len(receipt["invocation_id"]), 32)
            self.assertTrue((artifact / "receipt.json").is_file())

    def test_evidence_failure_covers_all_six_invocations(self):
        with tempfile.TemporaryDirectory() as tmp:
            evidence = Path(tmp).resolve()
            r.seal_evidence(evidence, "race", 1, "preparation failed")
            index = r.decode_json((evidence / "index.json").read_bytes())
            self.assertFalse(index["provisional_red_valid"])
            self.assertEqual(index["runner_exit"], 1)
            self.assertEqual(len(index["files"]), 6)
            for variant in r.SEQUENCE:
                row = r.decode_json((evidence / variant / "classification.json").read_bytes())
                self.assertFalse(row["executed"])
                self.assertEqual(row["verdict"], "UNQUALIFIED")
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(r.Unqualified, "missing invocation artifact"):
                r.seal_evidence(Path(tmp).resolve(), "normal", 0)

    def test_private_telemetry_and_symlink_rejection(self):
        # Resolve macOS /var alias before testing our own private directory.
        with tempfile.TemporaryDirectory() as tmp:
            q = Path(tmp).resolve()
            r.prepare_home(q)
            r.check_home(q)
            mode = q / "xdg-config/go/telemetry/mode"
            mode.write_bytes(b"on\n")
            with self.assertRaises(r.Unqualified):
                r.check_home(q)
            link = q / "link"
            link.symlink_to(mode)
            with self.assertRaises(r.Unqualified):
                r.safe_path(link)
            with self.assertRaises(r.Unqualified):
                r.file_bytes(q, "../elsewhere")


def source_snapshot_records(failed=False):
    """Independent JSON-mode source fixture, not the general transcript generator.

    Go 1.26.5 testing.go:832-846 bypasses parent indentation in JSON mode;
    :2496-2504 uses '--- %s: %s (%s)\\n'; test2json.go:306-332 preserves origLine.
    Diagnostic indentation is deliberately separate and is NOT normalized.
    """
    package = "lsp-trace/internal/managedprocess"
    root = "TestOwnedLinuxSnapshotCompletionCoherent"
    result = "fail" if failed else "pass"
    rows = [{"Action": "start", "Package": package}]

    def structural(action, test=""):
        rows.append({"Action": action, "Package": package, **({"Test": test} if test else {})})

    def output(text, test=""):
        rows.append({"Action": "output", "Package": package, "Output": text, **({"Test": test} if test else {})})

    structural("run", root)
    output("=== RUN   TestOwnedLinuxSnapshotCompletionCoherent\n", root)
    for child in ("stdout-error", "stderr-error", "stderr-truncation", "stdout-success"):
        name = root + "/" + child
        structural("run", name)
        output("=== RUN   " + name + "\n", name)
        if failed:
            output("    process_linux_supervision_test.go:419: ASSERT_LINUX_SNAPSHOT_COMPLETION_COHERENT: " + r.SNAP_PAYLOAD + "\n", name)
        output("--- " + result.upper() + ": " + name + " (0.01s)\n", name)
        structural(result, name)
    output("--- " + result.upper() + ": " + root + " (0.01s)\n", root)
    structural(result, root)
    output(result.upper() + "\n")
    output(("FAIL\t" if failed else "ok  \t") + package + "\t0.010s\n")
    structural(result)
    return rows


class SourceFormatRegressionTests(unittest.TestCase):
    def test_independent_json_mode_snapshot_format(self):
        for variant, failed, verdict in (("snapshot-baseline", False, "PASS"), ("old-snapshot", True, "INTENDED_FAIL")):
            with self.subTest(variant=variant):
                self.assertEqual(classify(variant, source_snapshot_records(failed))["verdict"], verdict)

    def test_terminal_and_diagnostic_whitespace_are_not_interchangeable(self):
        for variant, failed in (("snapshot-baseline", False), ("old-snapshot", True)):
            rows = source_snapshot_records(failed)
            for prefix in (" ", "    ", "\t"):
                bad = [dict(row) for row in rows]
                i = next(i for i, row in enumerate(bad) if "/stdout-error (" in row.get("Output", ""))
                bad[i]["Output"] = prefix + bad[i]["Output"]
                self.assertEqual(classify(variant, bad)["verdict"], "UNQUALIFIED")
        rows = source_snapshot_records(True)
        i = next(i for i, row in enumerate(rows) if "process_linux_supervision_test.go:" in row.get("Output", ""))
        rows[i]["Output"] = rows[i]["Output"].lstrip()
        self.assertEqual(classify("old-snapshot", rows)["verdict"], "UNQUALIFIED")


def synthetic_bundle(root):
    """Small accepted six-pair model; Git itself is never invoked by these tests."""
    candidate, fixtures = root / "candidate", root / "fixtures"
    candidate.mkdir()
    fixtures.mkdir()
    baselines = {name: ("baseline " + name + "\n").encode() for name in (
        r.P, r.T, "internal/managedprocess/process.go", "internal/managedprocess/private_provider_window.go",
        "internal/managedprocess/supervision_linux_unsupported.go", "go.mod", "go.sum")}
    for name, data in baselines.items():
        path = candidate / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
    for name in r.MEMBERS:
        (fixtures / (name + ".fixture")).write_bytes(("fixture " + name + "\n").encode())
    manifest = {"head": r.BASE, "sources": [{"path": name, "head_sha256": r.digest(data), "current_sha256": r.digest(data)} for name, data in baselines.items() if name.startswith("internal/")], "variants": []}
    for name in r.VARIANTS:
        replacements = r.REPLACEMENTS[name]
        pair = {"head": r.BASE, "selection": "^" + r.selection(name) + "$"}
        changes, delta = [], ""
        for path, member in replacements.items():
            data = (fixtures / (member + ".fixture")).read_bytes()
            changes.append({"repository_path": path, "external_path": r.OLD_CONTROL + "/" + member, "controlled_sha256": r.digest(data), "baseline_sha256": r.digest(baselines[path])})
            delta += "".join(difflib.unified_diff(baselines[path].decode().splitlines(True), data.decode().splitlines(True), fromfile="a/" + path, tofile="b/" + path))
        for path, key in ((r.P, "production_sha256"), (r.T, "test_sha256")):
            data = (fixtures / (replacements[path] + ".fixture")).read_bytes() if path in replacements else baselines[path]
            pair[key] = r.digest(data)
        overlay = json.dumps({"Replace": {r.OLD_REPO + "/" + p: r.OLD_CONTROL + "/" + m for p, m in replacements.items()}}).encode()
        (fixtures / (name + ".patch")).write_bytes(delta.encode())
        (fixtures / (name + ".overlay.json")).write_bytes(overlay)
        manifest["variants"].append({"variant": name, "pair": pair, "pair_sha256": r.pair_hash(pair), "changed_external_paths": changes,
            "patch": r.OLD_CONTROL + "/" + name + ".patch", "patch_sha256": r.digest(delta.encode()),
            "overlay": r.OLD_CONTROL + "/" + name + ".overlay.json", "overlay_sha256": r.digest(overlay)})
    docs = {"manifest.json": json.dumps(manifest).encode(), "verification.json": b"{}", "REPORT.md": b"synthetic", "INDEPENDENT-REVIEW-ADDENDUM.md": b"synthetic"}
    for name, data in docs.items():
        (fixtures / name).write_bytes(data)
    extras, calls = [], []

    def git(_candidate, *args):
        calls.append(args)
        if args == ("rev-parse", "HEAD"):
            return r.BASE.encode() + b"\n"
        if args[0] == "show":
            return baselines[args[1].split(":", 1)[1]]
        if args[0] == "diff":
            return b""  # Git diff never sees any untracked file.
        if args[0] == "ls-files" and "--others" in args:
            assert args[-1] == "internal/managedprocess", args
            if "--exclude-standard" in args:
                return b""  # Counterexample: all injected names are ignored.
            return b"".join(name.encode() + b"\0" for name in extras)
        raise AssertionError("unexpected Git operation: " + repr(args))

    return candidate, fixtures, {name: r.digest(data) for name, data in docs.items()}, extras, calls, git


class PackageInputRegressionTests(unittest.TestCase):
    def test_clean_six_pairs_then_all_ignored_package_inputs_rejected(self):
        for extra in ("ignored_linux.go", "ignored_test.go", "ignored.c", "ignored.s", "embedded.bin", "nested/asset.dat", "symlink_linux.go"):
            with self.subTest(extra=extra), tempfile.TemporaryDirectory() as tmp:
                candidate, fixtures, docs, extras, calls, git = synthetic_bundle(Path(tmp).resolve())
                with patch.object(r, "DOC_HASHES", docs), patch.object(r, "git", side_effect=git), patch.object(r.subprocess, "Popen", side_effect=AssertionError("no real subprocess")):
                    self.assertEqual(len(r.verify_bundle(fixtures, candidate)), 6)
                    name = "internal/managedprocess/" + extra
                    path = candidate / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    if extra.startswith("symlink"):
                        path.symlink_to(candidate / r.T)
                    else:
                        path.write_bytes(b"untracked compiler input; never executed")
                    extras.append(name)
                    with self.assertRaisesRegex(r.Unqualified, "untracked package"):
                        r.verify_bundle(fixtures, candidate)
                    inventories = [c for c in calls if c[0] == "ls-files"]
                    self.assertTrue(all("--exclude-standard" not in c and c[-1] == "internal/managedprocess" for c in inventories))


def mocked_capture(root, scenario="normal", purpose="test", *, deadline=10.0, exit_code=1, delay_at=None):
    """ALL kernel/process operations replaced; fake descriptors cannot reach OS."""
    events, now, signalled = [], [0.0], [False]
    eof = set()
    stream_reads = {900001: 0, 900002: 0}

    class Stream:
        def __init__(self, fd):
            self.fd = fd
        def fileno(self):
            return self.fd
        def close(self):
            events.append(("close", self.fd))
            if delay_at == ("stdout-close" if self.fd == 900001 else "stderr-close"):
                now[0] += 181.0

    class Process:
        pid = 900003
        returncode = None
        stdout, stderr = Stream(900001), Stream(900002)
        def wait(self, timeout=None):
            events.append(("wait", timeout, now[0]))
            self.returncode = 1
            return 1
        def poll(self):
            raise AssertionError("consuming poll forbidden")

    class Selector:
        def __init__(self):
            self.keys = {}
        def __enter__(self):
            return self
        def __exit__(self, *args):
            self.close()
        def close(self):
            events.append(("selector-close",))
            if delay_at == "selector-close":
                now[0] += 181.0
        def register(self, stream, _event, data):
            self.keys[stream.fd] = SimpleNamespace(fd=stream.fd, fileobj=stream, data=data)
        def unregister(self, stream):
            self.keys.pop(stream.fd)
        def get_map(self):
            return self.keys
        def select(self, timeout):
            now[0] += min(timeout, 0.2)
            if scenario == "timeout" and not signalled[0]:
                return []
            return [(key, None) for key in tuple(self.keys.values())]

    process = Process()

    def popen(*args, **kwargs):
        events.append(("spawn", kwargs))
        if scenario == "spawn-error":
            raise OSError("synthetic spawn failure")
        return process

    def read(fd, count):
        assert fd in stream_reads
        stream_reads[fd] += 1
        if scenario == "overflow" and fd == 900001 and stream_reads[fd] == 1:
            return b"x" * 9
        eof.add(fd)
        return b""

    def waitid(*args):
        events.append(("observe", signalled[0]))
        if scenario == "lost-custody" or (scenario == "lost-after-stop" and signalled[0]):
            raise ChildProcessError(errno.ECHILD, "synthetic lost custody")
        if scenario in ("timeout", "never-exits") and (not signalled[0] or scenario == "never-exits"):
            return None
        return SimpleNamespace(si_pid=process.pid, si_code=1, si_status=1)

    def killpg(pid, sig):
        events.append(("group-stop", pid, sig, now[0]))
        assert pid == process.pid
        if scenario == "cleanup-error":
            raise PermissionError(errno.EPERM, "synthetic group stop failure")
        signalled[0] = True

    def waitpid(pid, flags):
        events.append(("reap", pid, flags, now[0]))
        assert signalled[0], "must fence group before consuming wait"
        if scenario == "lost-at-reap":
            raise ChildProcessError(errno.ECHILD, "synthetic lost custody at reap")
        if delay_at == "wait-return":
            now[0] += 181.0
        return pid, exit_code << 8

    write = r.private_write

    def artifact_write(path, data):
        write(path, data)
        if delay_at == "raw-artifact" and path.name == "stderr.bin":
            now[0] += 181.0

    def monotonic():
        now[0] += 0.01
        return now[0]

    with ExitStack() as stack:
        for name, value in (("P_PID", 1), ("WEXITED", 4), ("WNOHANG", 1), ("WNOWAIT", 8)):
            stack.enter_context(patch.object(r.os, name, value, create=True))
        for obj, name, value in ((r.subprocess, "Popen", popen), (r.selectors, "DefaultSelector", Selector),
                (r.os, "set_blocking", lambda *_: None), (r.os, "read", read), (r.os, "waitid", waitid),
                (r.os, "killpg", killpg), (r.os, "waitpid", waitpid), (r.time, "monotonic", monotonic),
                (r.time, "sleep", lambda amount: now.__setitem__(0, now[0] + amount))):
            stack.enter_context(patch.object(obj, name, value, create=True))
        stack.enter_context(patch.object(r, "private_write", side_effect=artifact_write))
        stack.enter_context(patch.object(r, "CAPS", (8, 8)))
        retained = []
        stack.enter_context(patch.object(r, "_UNREAPED_CHILDREN", retained, create=True))
        out, err, receipt = r.capture(["/never-executed", "mod" if purpose == "preparation" else "test"], {}, root, deadline, root / "capture", {"purpose": purpose, "mode": "normal"})
        if scenario not in ("normal", "timeout", "overflow", "spawn-error"):
            assert retained == [process], "uncertain Popen must not be implicitly finalized/reaped"
        else:
            assert retained == []
    return receipt, events, eof


class SupervisorRegressionTests(unittest.TestCase):
    def test_normal_eof_leader_exit_stops_owned_group_before_reap(self):
        for purpose in ("test", "preparation"):
            with self.subTest(purpose=purpose), tempfile.TemporaryDirectory() as tmp:
                receipt, events, eof = mocked_capture(Path(tmp).resolve(), purpose=purpose)
                stop = [i for i, event in enumerate(events) if event[0] == "group-stop"]
                self.assertEqual(len(stop), 1, events)
                reap = [i for i, event in enumerate(events) if event[0] in ("reap", "wait")]
                self.assertEqual(len(reap), 1)
                self.assertLess(stop[0], reap[0])
                self.assertTrue(receipt["status"]["cleanup_ok"])
                self.assertEqual(receipt["status"]["returncode"], 1)
                self.assertEqual(eof, {900001, 900002})
                self.assertFalse(receipt["cleanup"]["descendant_absence_established"])
                self.assertTrue(receipt["cleanup"]["group_stop_sent"])
                self.assertTrue(receipt["cleanup"]["leader_reaped"])
                self.assertFalse(receipt["cleanup"]["leader_object_retained"])

    def test_timeout_overflow_cleanup_stays_inside_original_deadline(self):
        for scenario, flag in (("timeout", "timed_out"), ("overflow", "overflow")):
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as tmp:
                receipt, events, _ = mocked_capture(Path(tmp).resolve(), scenario)
                self.assertTrue(receipt["status"][flag])
                self.assertTrue(receipt["status"]["cleanup_ok"])
                self.assertEqual(sum(e[0] == "group-stop" for e in events), 1)
                self.assertLessEqual(receipt["ended_monotonic"], 10.0)
                self.assertFalse(any(e[0] == "wait" for e in events), "Popen.wait can swallow ECHILD")

    def test_spawn_failure_never_signals_or_reaps(self):
        with tempfile.TemporaryDirectory() as tmp:
            receipt, events, _ = mocked_capture(Path(tmp).resolve(), "spawn-error")
            self.assertFalse(receipt["status"]["cleanup_ok"])
            self.assertIsNone(receipt["status"]["returncode"])
            self.assertFalse(any(e[0] in ("group-stop", "wait", "reap") for e in events))

    def test_cleanup_error_cannot_claim_cleanup_or_retry(self):
        with tempfile.TemporaryDirectory() as tmp:
            receipt, events, _ = mocked_capture(Path(tmp).resolve(), "cleanup-error")
            self.assertFalse(receipt["status"]["cleanup_ok"])
            self.assertEqual(sum(e[0] == "group-stop" for e in events), 1)
            self.assertFalse(any(e[0] in ("wait", "reap") for e in events))

    def test_lost_custody_never_signals(self):
        with tempfile.TemporaryDirectory() as tmp:
            receipt, events, _ = mocked_capture(Path(tmp).resolve(), "lost-custody")
            self.assertFalse(receipt["status"]["cleanup_ok"])
            self.assertIsNone(receipt["status"]["returncode"])
            self.assertFalse(any(e[0] in ("group-stop", "wait", "reap") for e in events), events)
            self.assertTrue(receipt["cleanup"]["custody_lost"])
            self.assertTrue(receipt["cleanup"]["leader_object_retained"])

    def test_lost_custody_after_stop_or_at_reap_never_signals_again(self):
        for scenario in ("lost-after-stop", "lost-at-reap"):
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as tmp:
                receipt, events, _ = mocked_capture(Path(tmp).resolve(), scenario)
                self.assertFalse(receipt["status"]["cleanup_ok"])
                self.assertIsNone(receipt["status"]["returncode"])
                self.assertEqual(sum(e[0] == "group-stop" for e in events), 1)
                self.assertTrue(receipt["cleanup"]["custody_lost"])

    def test_nonexiting_leader_exhausts_fixed_cleanup_budget(self):
        with tempfile.TemporaryDirectory() as tmp:
            receipt, events, _ = mocked_capture(Path(tmp).resolve(), "never-exits")
            self.assertFalse(receipt["status"]["cleanup_ok"])
            self.assertIsNone(receipt["status"]["returncode"])
            self.assertTrue(receipt["cleanup"]["deadline_exhausted"])
            self.assertEqual(sum(e[0] == "group-stop" for e in events), 1)
            self.assertFalse(any(e[0] in ("wait", "reap") for e in events))
            self.assertLessEqual(receipt["ended_monotonic"], 10.1)


class DeadlineFinalizationRegressionTests(unittest.TestCase):
    def test_late_completion_cannot_admit_complete_expected_negative(self):
        for boundary in ("wait-return", "stdout-close", "stderr-close", "selector-close", "raw-artifact"):
            with self.subTest(boundary=boundary), tempfile.TemporaryDirectory() as tmp:
                receipt, events, _ = mocked_capture(Path(tmp).resolve(), deadline=480.0, delay_at=boundary)
                # Keep the exact 180s invocation ceiling within the 480s job budget.
                self.assertEqual(receipt["deadline_monotonic"], receipt["started_monotonic"] + 180)
                self.assertGreater(receipt["ended_monotonic"], receipt["deadline_monotonic"])
                self.assertLess(receipt["ended_monotonic"], 480.0)
                # Observed reap identity is retained despite late admission failure.
                self.assertEqual(receipt["status"]["returncode"], 1)
                self.assertTrue(receipt["cleanup"]["leader_reaped"])
                self.assertFalse(receipt["cleanup"]["leader_object_retained"])
                self.assertEqual(sum(e[0] == "group-stop" for e in events), 1)
                stop = next(i for i, e in enumerate(events) if e[0] == "group-stop")
                reap = next(i for i, e in enumerate(events) if e[0] == "reap")
                self.assertLess(stop, reap)
                self.assertFalse(any(e[0] == "group-stop" for e in events[reap + 1:]))
                verdict = r.classify_synthetic([encoded(transcript("early-wait"))], variant="early-wait", **receipt["status"])
                self.assertEqual(verdict["verdict"], "UNQUALIFIED", receipt)
                self.assertFalse(receipt["status"]["cleanup_ok"])
                self.assertTrue(receipt["status"]["timed_out"])
                self.assertTrue(receipt["cleanup"]["deadline_exhausted"])
                retained = r.decode_json((Path(tmp) / "capture/receipt.json").read_bytes())
                self.assertEqual(retained["status"], receipt["status"])
                self.assertEqual(retained["ended_monotonic"], receipt["ended_monotonic"])

    def test_ontime_positive_and_negative_status_still_admitted(self):
        for variant, exit_code, expected in (("candidate-original", 0, "PASS"), ("early-wait", 1, "INTENDED_FAIL")):
            with self.subTest(variant=variant), tempfile.TemporaryDirectory() as tmp:
                receipt, _, _ = mocked_capture(Path(tmp).resolve(), deadline=480.0, exit_code=exit_code)
                self.assertLess(receipt["ended_monotonic"], receipt["deadline_monotonic"])
                self.assertTrue(receipt["status"]["cleanup_ok"])
                self.assertFalse(receipt["status"]["timed_out"])
                self.assertFalse(receipt["cleanup"]["deadline_exhausted"])
                verdict = r.classify_synthetic([encoded(transcript(variant))], variant=variant, **receipt["status"])
                self.assertEqual(verdict["verdict"], expected)


if __name__ == "__main__":
    unittest.main()
