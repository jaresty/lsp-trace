#!/usr/bin/env python3
"""Structurally validate the CI workflow using Ruby's standard Psych YAML parser."""
import argparse
import json
import re
import shlex
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REQUIRED_NEEDS = {"quality", "elixir-companion", "test-linux", "test-darwin", "unix-vet-build", "windows"}
PARENT_PYTHON_FILES = [
    "scripts/retain-qualification.py", "scripts/check-program-c-profiles.py", "scripts/test-program-c-profiles.py",
    "scripts/check-program-c-gate-ii.py", "scripts/test-program-c-gate-ii.py", "scripts/check-program-c-gate-ii-v4.py",
    "scripts/test-program-c-gate-ii-v4.py",
]
NEW_PYTHON_FILES = [
    "scripts/check-ci-shards.py", "scripts/generate-ci-shards.py", "scripts/map-ci-affected-packages.py",
    "scripts/summarize-go-test-json.py", "scripts/check-ci-topology.py", "scripts/test-ci-shards.py",
    "scripts/test-ci-affected-packages.py", "scripts/test-summarize-go-test-json.py", "scripts/test-ci-topology.py",
]


def load_workflow(path):
    ruby = "require 'yaml'; require 'json'; puts JSON.generate(YAML.safe_load(File.read(ARGV[0]), aliases: true))"
    result = subprocess.run(["ruby", "-e", ruby, str(path)], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode:
        raise ValueError("workflow YAML parse failed: " + result.stderr.strip())
    return json.loads(result.stdout)


def fail(errors, message):
    errors.append(message)


def step_by_name(job, name):
    return next((step for step in job.get("steps", []) if step.get("name") == name), None)


def strip_shell_comment(line):
    """Remove an unquoted shell comment; this is deliberately not a full shell parser."""
    quote = None
    escaped = False
    for index, char in enumerate(line):
        if escaped:
            escaped = False
            continue
        if char == "\\" and quote != "'":
            escaped = True
            continue
        if quote:
            if char == quote:
                quote = None
            continue
        if char in "'\"":
            quote = char
        elif char == "#" and (index == 0 or line[index - 1].isspace()):
            return line[:index]
    return line


def executable_shell_lines(script):
    """Return normalized non-comment lines for the simple CI shell form we enforce."""
    return [
        " ".join(line.split())
        for raw in script.splitlines()
        if (line := strip_shell_comment(raw).strip())
    ]


def validate_shard_script(script):
    lines = executable_shell_lines(script)
    go_test_lines = [line for line in lines if re.search(r"(?:^|(?:&&|\|\||;|\|)\s*)go\s+test(?:\s|$)", line)]
    if len(go_test_lines) != 1:
        return False, False

    pipeline = go_test_lines[0]
    segments = [segment.strip() for segment in re.split(r"(?<!\|)\|(?!\|)", pipeline)]
    exact_pipeline = (
        len(segments) == 2
        and re.match(r"^go\s+test(?:\s|$)", segments[0]) is not None
        and re.match(r"^tee\s+go-test\.json$", segments[1]) is not None
    )
    if not exact_pipeline:
        return False, False
    try:
        go_tokens = shlex.split(segments[0])
    except ValueError:
        return True, False
    flags_ok = "-json" in go_tokens and "-timeout=10m" in go_tokens

    required = [
        "test_status=${PIPESTATUS[0]}",
        "tee_status=${PIPESTATUS[1]}",
        "summary_status=$?",
        'if [ "$test_status" -ne 0 ]; then exit "$test_status"; fi',
        'if [ "$tee_status" -ne 0 ]; then exit "$tee_status"; fi',
        'exit "$summary_status"',
    ]
    try:
        positions = [lines.index(fragment) for fragment in required]
        pipeline_position = lines.index(pipeline)
    except ValueError:
        return True, False
    statuses_ok = pipeline_position < positions[0] and positions == sorted(positions) and len(set(positions)) == len(positions)
    return True, flags_ok and statuses_ok


def validate(data):
    errors = []
    jobs = data.get("jobs")
    if not isinstance(jobs, dict):
        return ["workflow jobs must be a mapping"]
    for platform, runner, count in (("linux", "ubuntu-latest", 5), ("darwin", "macos-latest", 4)):
        name = f"test-{platform}"
        job = jobs.get(name, {})
        matrix = job.get("strategy", {}).get("matrix", {}).get("shard", [])
        expected_ids = [f"{platform}-{i}" for i in range(1, count + 1)]
        if job.get("runs-on") != runner:
            fail(errors, f"{name}: wrong runner")
        if [entry.get("id") for entry in matrix] != expected_ids:
            fail(errors, f"{name}: shard matrix mismatch")
        marked = [entry for entry in matrix if entry.get("needs_gopls") is True]
        if platform == "linux" and marked:
            fail(errors, "test-linux: gopls must not be enabled")
        if platform == "darwin" and [entry.get("id") for entry in marked] != ["darwin-1"]:
            fail(errors, "test-darwin: only darwin-1 may own gopls")
        run_step = step_by_name(job, "Run shard once and summarize") or {}
        pipeline_ok, script_ok = validate_shard_script(run_step.get("run", ""))
        if not pipeline_ok:
            fail(errors, f"{name}: test invocation must be exactly one effective pipeline")
        elif not script_ok:
            lines = executable_shell_lines(run_step.get("run", ""))
            go_line = next(line for line in lines if re.match(r"^go\s+test(?:\s|$)", line))
            go_tokens = shlex.split(re.split(r"(?<!\|)\|(?!\|)", go_line)[0])
            if "-json" not in go_tokens or "-timeout=10m" not in go_tokens:
                fail(errors, f"{name}: test invocation must include -json and -timeout=10m")
            else:
                fail(errors, f"{name}: test and telemetry statuses must remain separate with test status primary")
        upload = step_by_name(job, "Upload shard timing") or {}
        if upload.get("if") != "always()":
            fail(errors, f"{name}: upload step must use if: always()")
        install = step_by_name(job, "Install pinned gopls") or {}
        if install.get("if") != "matrix.shard.needs_gopls" or install.get("run") != "go install golang.org/x/tools/gopls@v0.23.0":
            fail(errors, f"{name}: gopls step mismatch")
    quality = jobs.get("quality", {})
    syntax = step_by_name(quality, "Check Python syntax") or {}
    expected_syntax = "python3 -m py_compile " + " ".join(PARENT_PYTHON_FILES + NEW_PYTHON_FILES)
    if syntax.get("run") != expected_syntax:
        fail(errors, "quality: Python syntax command mismatch")
    aggregate = jobs.get("repository-qualified", {})
    if aggregate.get("if") != "always()":
        fail(errors, "repository-qualified: must use if: always()")
    if set(aggregate.get("needs", [])) != REQUIRED_NEEDS:
        fail(errors, "repository-qualified: needs mismatch")
    aggregate_run = "\n".join(str(step.get("run", "")) for step in aggregate.get("steps", []))
    if 'value["result"] != "success"' not in aggregate_run:
        fail(errors, "repository-qualified: must require every needs result == success")
    for name in ("unix-vet-build", "windows"):
        runs = [step.get("run") for step in jobs.get(name, {}).get("steps", [])]
        if "go vet ./..." not in runs or "go build ./..." not in runs:
            fail(errors, f"{name}: vet/build coverage missing")
    triggers = data.get("on", data.get("true", {}))
    if not isinstance(triggers, dict) or "pull_request" not in triggers:
        fail(errors, "workflow: pull_request qualification missing")
    return errors


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--workflow", default=str(ROOT / ".github/workflows/ci.yml"))
    args = parser.parse_args()
    try:
        errors = validate(load_workflow(args.workflow))
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        errors = [str(exc)]
    for error in errors:
        print("FAIL " + error)
    if not errors:
        print("PASS CI workflow topology")
    return bool(errors)


if __name__ == "__main__":
    sys.exit(main())
