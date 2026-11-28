#!/usr/bin/env python3
"""Fixed, pure Linux qualification. Inactive until separately reviewed pins are sealed.

No log-file execution-proof API: classify_synthetic is deliberately not qualification.
Only capture() creates execution receipts; run_mode() consumes its in-memory result.
Synthetic tests do not execute Go or exercise the Linux process supervisor.
"""
from __future__ import annotations

import argparse
import codecs
import difflib
import errno
import hashlib
import json
import math
import os
from pathlib import Path
import platform
import re
import selectors
import signal
import stat
import subprocess
import sys
import time
import uuid

BASE = "39c57a81d067b8868710fc88eddafb6f13340e77"
# Deliberately unresolved action/source review. Control SHA belongs in a LATER
# workflow-only commit, not in this runner's own commit. Workflow SHA is recorded
# from GitHub and authenticated externally before dispatch, never self-attested.
ACTION_REVIEW = None
P = "internal/managedprocess/supervision_linux.go"
T = "internal/managedprocess/process_linux_supervision_test.go"
PACKAGE = "lsp-trace/internal/managedprocess"
ORDER = "TestOwnedLinuxFinalSignalReturnFence"
SNAP = "TestOwnedLinuxSnapshotCompletionCoherent"
CHILDREN = ("stdout-error", "stderr-error", "stderr-truncation", "stdout-success")
VARIANTS = ("candidate-original", "candidate-rendezvous", "old-snapshot", "early-wait", "early-fence")
SEQUENCE = ("candidate-original", "candidate-rendezvous", "snapshot-baseline", "old-snapshot", "early-wait", "early-fence")
MEMBERS = ("positive_test.go", "negative_test.go", "old-snapshot.go", "early-wait.go", "early-fence.go")
REPLACEMENTS = {
    "candidate-original": {}, "candidate-rendezvous": {T: "positive_test.go"},
    "old-snapshot": {P: "old-snapshot.go"},
    "early-wait": {P: "early-wait.go", T: "negative_test.go"},
    "early-fence": {P: "early-fence.go", T: "negative_test.go"},
}
OLD_REPO = "/Users/schwa/dev/lsp-trace-variable-v2-TSZxdY"
OLD_CONTROL = "/tmp/lsp-linux-controls-39c57a81"
DOC_HASHES = {
    "manifest.json": "8a543158089fdb0361fa9643d1130b31353c8e70cc3e5ec7bde715d90db06d39",
    "verification.json": "09cd958b563ea29773958d5e1922994ce9efd0999e52aa3435869e06a054023a",
    "REPORT.md": "2b92f9835a161fbf9f30ee90f85188935a3335e695f6de9808de06d0fe1a37b8",
    "INDEPENDENT-REVIEW-ADDENDUM.md": "b09a6145f22feb463effa2b2d0e4514ec0e4d336ed833d95d64c9eb4b705f52b",
}
EVENTS = ("observer-return", "wait-gate-entry", "final-signal-entry", "final-signal-return", "fence-publication", "wait-entry", "wait-return")
EDGES = ((2, 3), (3, 4), (4, 5), (5, 6), (0, 1), (1, 5))
SNAP_PAYLOAD = ("{Kind: ExitCode:0 Err:<nil> StdoutDelivery:{Err:<nil> Category:} "
                "StderrDelivery:{Err:<nil> Category:} Stderr:{Bytes:[] Truncated:false} "
                "Reap:{Kind:COMPLETE Err:<nil> Evidence:} Evidence:}")
CAPS = (8 * 1024 * 1024, 2 * 1024 * 1024)
# Keep uncertain children alive as Python objects until supervisor exit so Popen
# finalization cannot introduce an implicit consuming poll. Never retry or signal
# through these objects; job teardown owns final containment. Fixed invocation
# counts bound this list. Successfully reaped children are not retained.
_UNREAPED_CHILDREN = []


class Unqualified(ValueError):
    pass


def require(condition, reason):
    if not condition:
        raise Unqualified(reason)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def unique_object(items):
    obj = {}
    for key, value in items:
        require(key not in obj, "duplicate JSON key")
        obj[key] = value
    return obj


def decode_json(data):
    return json.loads(data, object_pairs_hook=unique_object,
                      parse_constant=lambda value: (_ for _ in ()).throw(Unqualified("nonfinite JSON")))


def pair_hash(pair):
    require(set(pair) == {"head", "production_sha256", "test_sha256", "selection"}, "pair keys")
    return digest(json.dumps(pair, sort_keys=True).encode())


def selection(variant):
    require(variant in SEQUENCE, "unknown variant")
    return SNAP if variant in ("snapshot-baseline", "old-snapshot") else ORDER


def safe_path(path, directory=False):
    """Reject symlink components, traversal and nonregular leaves, including roots."""
    path = Path(path)
    require(path.is_absolute() and ".." not in path.parts, "absolute non-traversing path required")
    for parent in reversed((path, *path.parents)):
        require(not parent.is_symlink(), "symlink path")
    mode = path.stat().st_mode
    require(stat.S_ISDIR(mode) if directory else stat.S_ISREG(mode), "wrong path type")
    return path


def file_bytes(root, name):
    require(not Path(name).is_absolute() and ".." not in Path(name).parts, "unsafe member")
    return safe_path(Path(root) / name).read_bytes()


def private_dir(path):
    path.mkdir(mode=0o700, parents=True, exist_ok=False)
    return path


def private_write(path, data):
    with path.open("xb") as stream:
        os.chmod(path, 0o600)
        stream.write(data)


def write_json(path, value):
    private_write(path, (json.dumps(value, sort_keys=True, indent=2) + "\n").encode())


def git(candidate, *args):
    # Only fixed arguments/internal allow-listed member names reach this helper.
    return subprocess.run(["/usr/bin/git", "-C", str(candidate), *args], check=True,
                          capture_output=True, timeout=10, env={"PATH": "/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM": "1", "HOME": "/nonexistent"}).stdout


def verify_bundle(fixtures, candidate):
    fixtures = safe_path(fixtures, True)
    candidate = safe_path(candidate, True)
    for name, expected in DOC_HASHES.items():
        require(digest(file_bytes(fixtures, name)) == expected, "document identity: " + name)
    manifest = decode_json(file_bytes(fixtures, "manifest.json"))
    require(manifest["head"] == BASE, "wrong base")
    require(git(candidate, "rev-parse", "HEAD").decode().strip() == BASE, "candidate HEAD")
    require(tuple(v["variant"] for v in manifest["variants"]) == VARIANTS, "variant inventory")
    expected_names = set(DOC_HASHES) | {n + ".fixture" for n in MEMBERS}
    expected_names |= {v + suffix for v in VARIANTS for suffix in (".patch", ".overlay.json")}
    require({p.name for p in fixtures.iterdir()} == expected_names, "fixture inventory")
    sources = {}
    for row in manifest["sources"]:
        name = row["path"]
        data = file_bytes(candidate, name)
        require(data == git(candidate, "show", BASE + ":" + name), "changed source " + name)
        require(digest(data) == row["head_sha256"] == row["current_sha256"], "source hash " + name)
        sources[name] = data
    require(not git(candidate, "diff", BASE, "--", "internal/managedprocess", "go.mod", "go.sum"), "package or module tracked changes")
    # Do NOT use --exclude-standard: ignored Go/cgo/assembly/embed inputs can
    # enter compilation/init despite exact -run selection. Check this entire
    # package only, including untracked symlinks and assets, before any Go call.
    require(not git(candidate, "ls-files", "--others", "-z", "--", "internal/managedprocess"), "untracked package input (including ignored)")
    # Bind module files to immutable checkout; no network or Go invocation here.
    for name in ("go.mod", "go.sum"):
        require(file_bytes(candidate, name) == git(candidate, "show", BASE + ":" + name), "module identity")
    effective = {}
    for row in manifest["variants"]:
        name = row["variant"]
        replacements = REPLACEMENTS[name]
        require(row["pair"]["selection"] == "^" + selection(name) + "$", "selection")
        expected_map = {OLD_REPO + "/" + p: OLD_CONTROL + "/" + f for p, f in replacements.items()}
        for kind, suffix in (("patch", ".patch"), ("overlay", ".overlay.json")):
            require(row[kind] == OLD_CONTROL + "/" + name + suffix, "historical path")
            require(digest(file_bytes(fixtures, name + suffix)) == row[kind + "_sha256"], "historical digest")
        require(decode_json(file_bytes(fixtures, name + ".overlay.json")) == {"Replace": expected_map}, "historical overlay")
        actual = {P: sources[P], T: sources[T]}
        require(len(row["changed_external_paths"]) == len(replacements), "replacement count")
        patch = ""
        for change in row["changed_external_paths"]:
            path = change["repository_path"]
            require(path in replacements, "extra replacement")
            member = replacements[path]
            require(change["external_path"] == OLD_CONTROL + "/" + member, "external provenance path")
            data = file_bytes(fixtures, member + ".fixture")
            require(digest(data) == change["controlled_sha256"], "fixture bytes")
            require(digest(sources[path]) == change["baseline_sha256"], "replacement base")
            actual[path] = data
            patch += "".join(difflib.unified_diff(sources[path].decode().splitlines(True), data.decode().splitlines(True), fromfile="a/" + path, tofile="b/" + path))
        require(patch.encode() == file_bytes(fixtures, name + ".patch"), "patch reproduction")
        pair = {"head": BASE, "production_sha256": digest(actual[P]), "test_sha256": digest(actual[T]), "selection": "^" + selection(name) + "$"}
        require(pair == row["pair"] and pair_hash(pair) == row["pair_sha256"], "effective pair")
        effective[name] = {"pair": pair, "pair_sha256": pair_hash(pair)}
    pair = dict(effective["candidate-original"]["pair"], selection="^" + SNAP + "$")
    effective["snapshot-baseline"] = {"pair": pair, "pair_sha256": pair_hash(pair)}
    return effective


def remap(fixtures, candidate, q, effective):
    controls = private_dir(q / "controls")
    maps = private_dir(q / "maps")
    for member in MEMBERS:
        private_write(controls / member, file_bytes(fixtures, member + ".fixture"))
    for name in SEQUENCE:
        mapping = REPLACEMENTS.get(name, {})
        overlay = {"Replace": {str(safe_path(candidate / path)): str(safe_path(controls / member)) for path, member in mapping.items()}}
        path = maps / (name + ".json")
        write_json(path, overlay)
        effective[name].update(overlay=str(path), overlay_sha256=digest(path.read_bytes()))
    return effective


def verify_effective(candidate, bound):
    for name, row in bound.items():
        raw = safe_path(row["overlay"]).read_bytes()
        require(digest(raw) == row["overlay_sha256"], "overlay changed")
        mapping = decode_json(raw)["Replace"]
        controls = Path(row["overlay"]).parent.parent / "controls"
        expected = {str(candidate / p): str(controls / f) for p, f in REPLACEMENTS.get(name, {}).items()}
        require(mapping == expected, "remapped overlay identity")
        pair = {"head": BASE, "selection": "^" + selection(name) + "$"}
        for path, key in ((P, "production_sha256"), (T, "test_sha256")):
            pair[key] = digest(safe_path(mapping.get(str(candidate / path), candidate / path)).read_bytes())
        require(pair == row["pair"] and pair_hash(pair) == row["pair_sha256"], "pair changed")


def json_events(chunks):
    """Incremental UTF-8 AND newline decoder; no last-line truncation tolerance."""
    decoder = codecs.getincrementaldecoder("utf-8")("strict")
    pending = ""
    size = 0
    for chunk in chunks:
        require(isinstance(chunk, bytes), "stdout is bytes")
        size += len(chunk)
        require(size <= CAPS[0], "stdout cap")
        pending += decoder.decode(chunk)
        while "\n" in pending:
            line, pending = pending.split("\n", 1)
            require(bool(line), "blank JSON line")
            yield decode_json(line)
    pending += decoder.decode(b"", final=True)
    require(not pending, "truncated JSON final line")


def tokens(chunks):
    """Output chunks become line tokens, without losing stream or action order."""
    pending, owner = "", None
    for event in json_events(chunks):
        require(isinstance(event, dict), "JSON event object")
        require(set(event) <= {"Time", "Action", "Package", "Test", "Elapsed", "Output"}, "unknown event fields")
        require(event.get("Package") == PACKAGE, "foreign package")
        action, test = event.get("Action"), event.get("Test", "")
        require(isinstance(test, str), "test identity")
        if "Elapsed" in event:
            require(type(event["Elapsed"]) in (int, float) and math.isfinite(event["Elapsed"]) and event["Elapsed"] >= 0, "elapsed")
        if action == "output":
            require(isinstance(event.get("Output"), str) and event["Output"], "empty/nonstring output")
            require(not pending or owner == test, "interleaved partial output")
            owner = test
            pending += event["Output"]
            while "\n" in pending:
                line, pending = pending.split("\n", 1)
                yield ("line", test, line)
        else:
            require(not pending, "action within partial output")
            require("Output" not in event, "output on structural event")
            require(action in ("start", "run", "pass", "fail"), "unrecognized action")
            yield (action, test, None)
    require(not pending, "partial output at EOF")


class Cursor:
    def __init__(self, chunks):
        self.items = iter(tokens(chunks))

    def take(self, action, test="", line=None):
        item = next(self.items, None)
        require(item is not None and item[:2] == (action, test), "unexpected stream/action order: " + repr(item))
        if line is not None:
            require(item[2] == line, "unrecognized output: " + repr(item[2]))
        return item[2]

    def terminal(self, test, outcome):
        # Go 1.26.5 JSON-mode reports bypass parent indentation (testing.go
        # 832-846, 2496-2504); test2json preserves the original terminal line.
        text = self.take("line", test)
        require(re.fullmatch(re.escape("--- " + outcome.upper() + ": " + test) + r" \([0-9]+\.[0-9]{2}s\)", text) is not None, "terminal output grammar")
        self.take(outcome, test)

    def run(self, test):
        self.take("run", test)
        self.take("line", test, "=== RUN   " + test)


def trace_value(text):
    require(text.startswith("[") and text.endswith("]"), "trace delimiters")
    values = text[1:-1].split(" ")
    require(len(values) == 7 and set(values) == set(EVENTS), "trace multiplicity")
    return values


def order_diagnostics(cursor, variant):
    negative = variant in ("early-wait", "early-fence")
    prefix = "    process_linux_supervision_test.go:"
    location = "482" if variant == "candidate-original" else "534"
    if negative:
        cursor.take("line", ORDER, prefix + "521: CONTROL_ACTUAL_WAIT_ENTERED_WHILE_SIGNAL_HELD")
        cursor.take("line", ORDER, prefix + "524: ASSERT_LINUX_FINAL_RETURN_REQUIRED_WHILE_HELD")
    assertions = []
    # At most six original edges can fail. Identical assertion output is legitimate
    # for different broken edges, but its exact count must equal the trace defects.
    while True:
        line = cursor.take("line", ORDER)
        assertion_prefix = prefix + location + ": ASSERT_LINUX_FINAL_RETURN_FENCE_WAIT_ORDER: "
        trace_prefix = prefix + location + ": LINUX_ORDERED_EVENTS="
        if line.startswith(assertion_prefix) and negative:
            assertions.append(trace_value(line[len(assertion_prefix):]))
            require(len(assertions) <= len(EDGES), "replayed assertion")
            continue
        require(line.startswith(trace_prefix), "unrecognized source diagnostic")
        trace = trace_value(line[len(trace_prefix):])
        break
    pos = [trace.index(e) for e in EVENTS]
    violations = sum(pos[a] >= pos[b] for a, b in EDGES)
    require(len(assertions) == violations and all(a == trace for a in assertions), "assertion/trace disagreement")
    if not negative:
        require(violations == 0, "positive trace order")
    else:
        require(violations > 0, "missing intended edge failure")
        require(pos[0] < pos[1] < pos[2] < pos[5] < pos[6] and pos[2] < pos[3], "uncontested trace order")
        if variant == "early-wait":
            require(pos[5] < pos[3] < pos[4], "early-wait relation")
        else:
            require(pos[1] < pos[4] < pos[2] < pos[5] < pos[3], "early-fence relation")


def _classify(chunks, stderr, status, variant):
    require(variant in SEQUENCE, "variant")
    require(status == {"returncode": 1 if variant in ("old-snapshot", "early-wait", "early-fence") else 0,
                       "timed_out": False, "overflow": False, "cleanup_ok": True}, "process status")
    require(stderr == b"", "unrecognized nonempty stderr")
    root = selection(variant)
    outcome = "fail" if status["returncode"] else "pass"
    cursor = Cursor(chunks)
    cursor.take("start")
    cursor.run(root)
    if root == SNAP:
        for child in CHILDREN:
            test = root + "/" + child
            cursor.run(test)
            if variant == "old-snapshot":
                cursor.take("line", test, "    process_linux_supervision_test.go:419: ASSERT_LINUX_SNAPSHOT_COMPLETION_COHERENT: " + SNAP_PAYLOAD)
            cursor.terminal(test, outcome)
    else:
        order_diagnostics(cursor, variant)
    cursor.terminal(root, outcome)
    cursor.take("line", "", outcome.upper())
    summary = cursor.take("line")
    start = "ok  \t" if outcome == "pass" else "FAIL\t"
    require(re.fullmatch(re.escape(start + PACKAGE + "\t") + r"[0-9]+\.[0-9]{3}s", summary) is not None, "package summary grammar")
    cursor.take(outcome)
    require(next(cursor.items, None) is None, "output/event after package terminal")
    return "INTENDED_FAIL" if outcome == "fail" else "PASS"


def classify_synthetic(chunks, *, variant, returncode=0, stderr=b"", timed_out=False, overflow=False, cleanup_ok=True):
    """Explicitly synthetic: never accepted by run_mode as execution evidence."""
    try:
        verdict = _classify(chunks, stderr, dict(returncode=returncode, timed_out=timed_out, overflow=overflow, cleanup_ok=cleanup_ok), variant)
        return {"origin": "SYNTHETIC", "verdict": verdict}
    except (ValueError, TypeError, KeyError, UnicodeError) as exc:
        return {"origin": "SYNTHETIC", "verdict": "UNQUALIFIED", "reason": str(exc)}


def prepare_home(q):
    """Call before setup-go, which may itself invoke Go. No Go process here."""
    os.umask(0o077)
    for name in ("home", "xdg-config", "xdg-cache", "tmp", "gopath", "modcache", "buildcache"):
        private_dir(q / name)
    for mode in (q / "xdg-config/go/telemetry/mode", q / "home/.config/go/telemetry/mode"):
        mode.parent.mkdir(mode=0o700, parents=True)
        private_write(mode, b"off\n")
    check_home(q)


def check_home(q):
    for name in ("home", "xdg-config", "xdg-cache"):
        require(stat.S_IMODE(safe_path(q / name, True).stat().st_mode) == 0o700, "private HOME mode")
    for mode in (q / "xdg-config/go/telemetry/mode", q / "home/.config/go/telemetry/mode"):
        require(file_bytes(mode.parent, mode.name) == b"off\n" and stat.S_IMODE(mode.stat().st_mode) == 0o600, "telemetry mode")


def environment(q, go, mode, network=False):
    require(mode in ("normal", "race"), "mode")
    env = {"HOME": str(q / "home"), "XDG_CONFIG_HOME": str(q / "xdg-config"), "XDG_CACHE_HOME": str(q / "xdg-cache"),
           "TMPDIR": str(q / "tmp"), "PATH": str(go.parent) + ":/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TZ": "UTC",
           "GOPATH": str(q / "gopath"), "GOMODCACHE": str(q / "modcache"), "GOCACHE": str(q / "buildcache"),
           "GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local", "GOTELEMETRY": "off", "GOPROXY": "off", "GOSUMDB": "off",
           "GOVCS": "*:off", "GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "1" if mode == "race" else "0"}
    if mode == "race":
        env.update(CC="/usr/bin/gcc", CXX="/usr/bin/g++")
    if network:
        env.update(GOPROXY="https://proxy.golang.org", GOSUMDB="sum.golang.org")
    return env


def command(go, q, mode, variant):
    require(variant in SEQUENCE and mode in ("normal", "race"), "fixed command selection")
    return [str(go), "test"] + (["-race"] if mode == "race" else []) + [
        "-json", "-count=1", "-timeout=10s", "-parallel=1", "-vet=off", "-mod=readonly", "-buildvcs=false",
        "-overlay", str(q / "maps" / (variant + ".json")), "-run", "^" + selection(variant) + "$", "./internal/managedprocess"]


def capture(argv, env, cwd, deadline, artifact, binding):
    """Exclusively own this Popen child; never poll/wait through another owner.

    Normal EOF/leader exit does NOT prove group cleanup. On every spawned path,
    request SIGKILL of our group while its direct leader is still unreaped, then
    consume only that leader's status. WNOWAIT/ECHILD loss of custody forbids any
    later signal. No Popen.wait (which can hide ECHILD), background reaper, retry,
    or fresh cleanup allowance is used. The two-second cleanup reserve is INSIDE
    min(original job deadline, start+180s), not added after timeout.

    cleanup_ok means: group-stop request succeeded, this direct leader was reaped,
    both synchronous readers reached EOF and were closed, and no capture/cleanup
    error occurred. It does NOT establish descendant absence/reaping. Escaped or
    uninterruptible descendants remain the hosted job's final containment concern.
    """
    private_dir(artifact)
    invocation = uuid.uuid4().hex
    begin = time.monotonic()
    stop = min(deadline, begin + 180)
    work_stop = stop - 2
    buffers = [bytearray(), bytearray()]
    status = dict(returncode=None, timed_out=False, overflow=False, cleanup_ok=False)
    cleanup = dict(group_stop_attempted=False, group_stop_sent=False,
                   leader_reaped=False, readers_eof=[False, False], readers_closed=[False, False],
                   custody_lost=False, deadline_exhausted=False, descendant_absence_established=False,
                   leader_object_retained=False)
    receipt = dict(binding, invocation_id=invocation, argv=argv, environment=env,
                   origin="SUPERVISOR", started_monotonic=begin, deadline_monotonic=stop,
                   work_deadline_monotonic=work_stop, cleanup=cleanup)
    proc, sel = None, None

    def observe_owned_leader():
        # The process object remains live and exclusively owned here. A valid
        # WNOHANG result of None still confirms a child, without consuming it.
        if proc.returncode is not None:
            cleanup["custody_lost"] = True
            raise Unqualified("leader already consumed outside capture")
        try:
            info = os.waitid(os.P_PID, proc.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT)
        except OSError as exc:
            if exc.errno == errno.ECHILD:
                cleanup["custody_lost"] = True
            raise
        require(info is None or info.si_pid == proc.pid, "unexpected waitid identity")
        return info

    def drain_ready(until):
        remaining = until - time.monotonic()
        if remaining <= 0:
            return
        if sel is None or not sel.get_map():
            time.sleep(min(remaining, 0.01))
            return
        for key, _ in sel.select(min(remaining, 0.05)):
            if time.monotonic() >= until:
                return
            try:
                data = os.read(key.fd, 65536)
            except BlockingIOError:
                continue
            i = key.data
            if not data:
                cleanup["readers_eof"][i] = True
                sel.unregister(key.fileobj)
                continue
            room = CAPS[i] - len(buffers[i])
            buffers[i].extend(data[:room])
            if len(data) > room:
                status["overflow"] = True
            # After overflow, retain the capped bytes but drain remaining input
            # only until the same absolute deadline; overflow always disqualifies.

    try:
        require(work_stop > begin, "insufficient original budget including cleanup reserve")
        proc = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        receipt["pid"] = proc.pid
        sel = selectors.DefaultSelector()
        for i, stream in enumerate((proc.stdout, proc.stderr)):
            os.set_blocking(stream.fileno(), False)
            sel.register(stream, selectors.EVENT_READ, i)
        while True:
            if time.monotonic() >= work_stop:
                status["timed_out"] = True
                break
            info = observe_owned_leader()
            if info is not None and all(cleanup["readers_eof"]):
                break
            drain_ready(work_stop)
            if status["overflow"]:
                break
    except (OSError, subprocess.SubprocessError, Unqualified) as exc:
        receipt["error"] = str(exc)
    finally:
        if proc is not None and not cleanup["custody_lost"]:
            try:
                if time.monotonic() >= stop:
                    cleanup["deadline_exhausted"] = True
                    raise Unqualified("original cleanup deadline exhausted before group stop")
                observe_owned_leader()  # fresh non-consuming custody check
                if time.monotonic() >= stop:
                    cleanup["deadline_exhausted"] = True
                    raise Unqualified("original cleanup deadline exhausted before group stop")
                cleanup["group_stop_attempted"] = True
                os.killpg(proc.pid, signal.SIGKILL)
                cleanup["group_stop_sent"] = True
                # Exactly one signal request, including normal EOF. No consuming
                # wait occurred above. A failed request never becomes cleanup_ok.
                while True:
                    if time.monotonic() >= stop:
                        cleanup["deadline_exhausted"] = True
                        break
                    if not cleanup["leader_reaped"]:
                        info = observe_owned_leader()
                        if info is not None and time.monotonic() < stop:
                            try:
                                pid, wait_status = os.waitpid(proc.pid, os.WNOHANG)
                            except OSError as exc:
                                if exc.errno == errno.ECHILD:
                                    cleanup["custody_lost"] = True
                                raise
                            require(pid in (0, proc.pid), "unexpected waitpid identity")
                            if pid == proc.pid:
                                status["returncode"] = os.waitstatus_to_exitcode(wait_status)
                                proc.returncode = status["returncode"]
                                cleanup["leader_reaped"] = True
                            # A consuming syscall may return after descheduling
                            # past stop. Keep its observed reap identity, but never
                            # admit late completion or signal/retry after the reap.
                            if time.monotonic() >= stop:
                                cleanup["deadline_exhausted"] = True
                                status["timed_out"] = True
                                break
                    if cleanup["leader_reaped"] and all(cleanup["readers_eof"]):
                        break
                    drain_ready(stop)
            except (OSError, subprocess.SubprocessError, Unqualified) as exc:
                cleanup["error"] = str(exc)
        if proc is not None:
            for i, stream in enumerate((proc.stdout, proc.stderr)):
                try:
                    stream.close()
                    cleanup["readers_closed"][i] = True
                except OSError as exc:
                    cleanup["error"] = str(exc)
        if sel is not None:
            try:
                sel.close()
            except OSError as exc:
                cleanup["error"] = str(exc)
        if proc is not None and not cleanup["leader_reaped"]:
            _UNREAPED_CHILDREN.append(proc)
            cleanup["leader_object_retained"] = True
        for name, data in zip(("stdout.jsonl", "stderr.bin"), buffers):
            private_write(artifact / name, bytes(data))
        # Include final reader/selector closure and raw-capture completion. Use
        # this SAME clock observation for admission and the receipt: no excluded
        # closing time, renewed allowance, or later unchecked end timestamp.
        ended = time.monotonic()
        if ended >= stop:
            cleanup["deadline_exhausted"] = True
            status["timed_out"] = True
        status["cleanup_ok"] = (cleanup["group_stop_sent"] and cleanup["leader_reaped"]
            and all(cleanup["readers_eof"]) and all(cleanup["readers_closed"])
            and not cleanup["custody_lost"] and not cleanup["deadline_exhausted"]
            and "error" not in receipt and "error" not in cleanup)
        receipt.update(status=status, ended_monotonic=ended,
                       stdout_sha256=digest(buffers[0]), stderr_sha256=digest(buffers[1]))
        write_json(artifact / "receipt.json", receipt)
    return bytes(buffers[0]), bytes(buffers[1]), receipt


def activate():
    control_sha = os.environ.get("CONTROL_SHA", "")
    workflow_sha = os.environ.get("GITHUB_WORKFLOW_SHA", "")
    require(re.fullmatch(r"[0-9a-f]{40}", control_sha) is not None, "UNRESOLVED_CONTROL_PIN: inactive")
    require(re.fullmatch(r"[0-9a-f]{40}", workflow_sha) is not None, "workflow identity unavailable")
    require(len({BASE, control_sha, workflow_sha}) == 3, "candidate/control/workflow identities must be separate")
    require(isinstance(ACTION_REVIEW, dict) and set(ACTION_REVIEW) == {"checkout", "setup-go", "upload-artifact", "telemetry_source_sha256"}, "UNRESOLVED_ACTION_REVIEW: inactive")
    for action in ("checkout", "setup-go", "upload-artifact"):
        require(isinstance(ACTION_REVIEW[action], str) and re.fullmatch(r"[0-9a-f]{40}", ACTION_REVIEW[action]), "unsealed action pin")
    require(isinstance(ACTION_REVIEW["telemetry_source_sha256"], str) and re.fullmatch(r"[0-9a-f]{64}", ACTION_REVIEW["telemetry_source_sha256"]), "unsealed telemetry source review")
    require(os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("GITHUB_EVENT_NAME") == "workflow_dispatch", "hosted dispatch only")
    require(os.environ.get("GITHUB_SHA") == workflow_sha, "dispatch head/workflow mismatch")
    require(platform.system() == "Linux" and platform.machine() == "x86_64", "native Linux amd64 only")
    return control_sha, workflow_sha


def require_readonly_inputs(candidate):
    # The future workflow prepares modes on its isolated checkout. The runner does
    # not edit source contents or apply patches, even when download needs go.sum.
    names = git(candidate, "ls-files", "-z", "--", "internal/managedprocess", "go.mod", "go.sum").decode().split("\0")
    require("go.mod" in names and "go.sum" in names, "module inventory")
    for name in filter(None, names):
        require(not safe_path(candidate / name).stat().st_mode & 0o222, "candidate input is writable: " + name)
    for path in (candidate, candidate / "internal", candidate / "internal/managedprocess"):
        require(not safe_path(path, True).stat().st_mode & 0o222, "candidate directory is writable")


def run_mode(candidate, fixtures, q, go, mode, deadline, evidence):
    """Future authorized entry; preparation counts against the original deadline."""
    control_sha, workflow_sha = activate()
    require(time.monotonic() < deadline, "deadline before preparation")
    safe_path(go)
    check_home(q)
    # Exact reviewed Go source identity is sealed with action review, not guessed
    # from environment variables. Failure is HOLD before our first Go command.
    telemetry_source = go.parent.parent / "src/cmd/vendor/golang.org/x/telemetry/internal/telemetry/dir.go"
    require(digest(safe_path(telemetry_source).read_bytes()) == ACTION_REVIEW["telemetry_source_sha256"], "pinned telemetry source identity")
    write_json(evidence / "platform.json", {
        "uname": list(platform.uname()), "mode": mode, "candidate_sha": BASE,
        "control_sha": control_sha, "workflow_sha": workflow_sha,
        "go_executable_sha256": digest(go.read_bytes()), "action_review": ACTION_REVIEW,
        "telemetry_pre": "off", "absolute_deadline_monotonic": deadline,
        "hosted": {key: os.environ.get(key) for key in ("GITHUB_REPOSITORY", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT", "GITHUB_JOB", "ImageOS", "ImageVersion")},
    })
    effective = verify_bundle(fixtures, candidate)
    require_readonly_inputs(candidate)
    bound = remap(fixtures, candidate, q, effective)
    env = environment(q, go, mode)
    write_json(evidence / "bound.json", bound)
    # Only this separate bootstrap may use the proxy. No retries/fallback/install.
    preps = [("version", [str(go), "version"], env),
             ("telemetry", [str(go), "telemetry"], env)]
    if mode == "race":
        for compiler in ("gcc", "g++"):
            compiler_path = Path("/usr/bin") / compiler
            require(compiler_path.is_file() and os.access(compiler_path, os.X_OK), "preexisting compiler required")
            preps.append((compiler, [str(compiler_path), "--version"], env))
    preps.extend([("download", [str(go), "mod", "download"], environment(q, go, mode, True)),
                  ("verify", [str(go), "mod", "verify"], env)])
    for name, argv, prep_env in preps:
        out, err, receipt = capture(argv, prep_env, candidate, deadline, evidence / name, {"mode": mode, "purpose": "preparation"})
        require(receipt["status"] == dict(returncode=0, timed_out=False, overflow=False, cleanup_ok=True), "preparation failure")
        if name == "version":
            require(out == b"go version go1.26.5 linux/amd64\n" and not err, "pinned toolchain")
        if name == "telemetry":
            require(out == b"off\n" and not err, "actual Go telemetry off")
        check_home(q)
    # Ephemeral cache only. Go's writable build cache remains separate.
    for root, dirs, files in os.walk(q / "modcache", followlinks=False):
        for name in files:
            path = Path(root) / name
            require(not path.is_symlink(), "module cache symlink")
            path.chmod(0o400)
        for name in dirs:
            require(not (Path(root) / name).is_symlink(), "module cache symlink")
        Path(root).chmod(0o500)
    verify_bundle(fixtures, candidate)
    results = []
    positives = True
    for variant in SEQUENCE:
        row = bound[variant]
        artifact = evidence / variant
        if not positives:
            private_dir(artifact)
            result = {"verdict": "UNQUALIFIED", "reason": "same-mode positive prerequisite failed", "executed": False}
        else:
            verify_effective(candidate, bound)
            out, err, receipt = capture(command(go, q, mode, variant), env, candidate, deadline, artifact,
                                       dict(row, variant=variant, mode=mode, candidate_sha=BASE, control_sha=control_sha, workflow_sha=workflow_sha))
            try:
                verdict = _classify([out], err, receipt["status"], variant)
                result = {"verdict": verdict, "invocation_id": receipt["invocation_id"], "executed": True}
            except (ValueError, TypeError, KeyError, UnicodeError) as exc:
                result = {"verdict": "UNQUALIFIED", "reason": str(exc), "invocation_id": receipt["invocation_id"], "executed": True}
        write_json(artifact / "classification.json", result)
        results.append(result)
        if variant in SEQUENCE[:3] and result["verdict"] != "PASS":
            positives = False
    verify_bundle(fixtures, candidate)
    verify_effective(candidate, bound)
    check_home(q)
    require(time.monotonic() < deadline, "overall deadline")
    qualified = [r["verdict"] for r in results] == ["PASS"] * 3 + ["INTENDED_FAIL"] * 3
    write_json(evidence / "result.json", {"qualified": qualified, "mode": mode, "results": results,
               "telemetry_post": "off", "external_job_and_upload_success_required": True})
    return 0 if qualified else 1


def seal_evidence(evidence, mode, code, reason=None):
    """Index only runner-owned, explicitly named evidence, including not-run rows.

    Aggregate failure invalidates every provisional RED. Successful upload and
    external job outcome are still required even when this index is complete.
    """
    paths = []
    for variant in SEQUENCE:
        artifact = evidence / variant
        if not artifact.exists():
            private_dir(artifact)
            write_json(artifact / "classification.json", {"verdict": "UNQUALIFIED", "executed": False, "reason": reason or "prerequisite failure"})
        classification = artifact / "classification.json"
        if not classification.exists():
            write_json(classification, {"verdict": "UNQUALIFIED", "reason": reason or "incomplete capture"})
    for name in ("version", "telemetry", "gcc", "g++", "download", "verify", *SEQUENCE):
        for leaf in ("stdout.jsonl", "stderr.bin", "receipt.json", "classification.json"):
            path = evidence / name / leaf
            if path.exists():
                paths.append(path)
    for name in ("bound.json", "platform.json", "result.json"):
        path = evidence / name
        if path.exists():
            paths.append(path)
    if code == 0:
        for variant in SEQUENCE:
            require(all((evidence / variant / name).is_file() for name in ("stdout.jsonl", "stderr.bin", "receipt.json", "classification.json")), "missing invocation artifact")
        require(all((evidence / name).is_file() for name in ("bound.json", "platform.json", "result.json")), "missing aggregate artifact")
    write_json(evidence / "index.json", {
        "mode": mode, "runner_exit": code, "reason": reason,
        "provisional_red_valid": code == 0, "external_job_and_upload_success_required": True,
        "files": {str(path.relative_to(evidence)): digest(safe_path(path).read_bytes()) for path in paths},
    })


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=("normal", "race"), required=True)
    args = parser.parse_args()
    # No caller-selected paths/logs/overlays/regex. The workflow supplies fixed roots.
    evidence = None
    code, reason = 1, None
    try:
        control_sha, _ = activate()
        workspace = safe_path(os.environ["GITHUB_WORKSPACE"], True)
        q = safe_path(Path(os.environ["RUNNER_TEMP"]) / "linux-lifecycle", True)
        evidence = safe_path(q / "evidence", True)
        control = workspace / "control"
        require(git(control, "rev-parse", "HEAD").decode().strip() == control_sha, "control checkout")
        require(Path(__file__).absolute() == control / "scripts/qualification/linux-lifecycle/runner.py", "runner checkout location")
        require(not git(control, "diff", control_sha, "--", "scripts/qualification/linux-lifecycle"), "modified control checkout")
        # start.json is written by earliest workflow bootstrap before any action/prep.
        start = decode_json(file_bytes(q, "start.json"))
        deadline = start["monotonic"] + 480  # leaves two minutes for cleanup/upload
        go = Path(os.environ["GOROOT"]) / "bin/go"
        code = run_mode(workspace / "candidate", control / "scripts/qualification/linux-lifecycle/fixtures/39c57a81", q, go, args.mode, deadline, evidence)
    except (ValueError, OSError, KeyError, subprocess.SubprocessError) as exc:
        reason = str(exc)
        print("UNQUALIFIED: " + reason, file=sys.stderr)
    if evidence is not None:
        try:
            seal_evidence(evidence, args.mode, code, reason)
        except (ValueError, OSError) as exc:
            print("UNQUALIFIED: evidence finalization: " + str(exc), file=sys.stderr)
            return 1
    return code


if __name__ == "__main__":
    raise SystemExit(main())
