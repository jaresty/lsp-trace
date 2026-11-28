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
    "scripts/run-ci-shard.py", "scripts/test-run-ci-shard.py",
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
    """One checked plan replaces one pipeline, not the exact-execution invariant.

    Exact shell form prevents extra/dead commands, status masking, unchecked runner
    variants and relative telemetry. Runner unit tests own argv/plan/exit semantics.
    """
    lines = executable_shell_lines(script)
    runners = [line for line in lines if 'scripts/run-ci-shard.py' in line]
    if len(runners) != 1 or any(re.search(r'\bgo\s+test\b', line) for line in lines):
        return False, False, False
    telemetry = 'telemetry_dir=$(mktemp -d "$RUNNER_TEMP/go-test-${{ matrix.shard.id }}-XXXXXX")'
    command = ('exec python3 scripts/run-ci-shard.py --platform "$PLATFORM" '
               '--shard "${{ matrix.shard.id }}" --revision "$GITHUB_SHA" --output "$telemetry_dir"')
    telemetry_ok = telemetry in lines and '--output "$telemetry_dir"' in runners[0] and lines.index(telemetry) < lines.index(runners[0])
    return True, lines == [telemetry, command], telemetry_ok


# Normative postchecks copied from the preserved 42698e5 workflow, not derived
# from the candidate under validation. Their implicit success() guard is retained.
POSTCHECKS = {
    'Check controlled inventory source identity': (
        "matrix.shard.id == 'linux-1' || matrix.shard.id == 'darwin-1'",
        "go test -tags lsp_trace_controlled_describe_fixture ./cmd/lsp-trace -run '^TestCensusFreshIdentityCaptureV3$' -count=1 -timeout=2m", {}),
    'Check Go-only normal-folder caller': (
        "matrix.shard.id == 'darwin-1'",
        "go test -tags lsp_trace_controlled_describe_fixture ./cmd/lsp-trace -run '^TestCensusFolder(GoHost|HostNeutralBuild|AdmissionRequest|CrossFileInventory)$' -count=1 -timeout=2m", {}),
    'Check durable inventory and legacy Describe across process exit': (
        "matrix.shard.id == 'darwin-1'",
        "go test -race ./internal/inventorysuccessor -run '^TestDurableDescribe(ProcessBoundary|CompetingProcesses|KilledClaim)$' -count=1 -timeout=2m\n"
        "go test -race ./internal/inventorysuccessor/sourcecontinuation -run '^TestDurableMutationDeadline$' -count=1 -timeout=1m\n"
        "go test -race ./cmd/lsp-trace -run '^TestCensusDurable(HeldOpenInput|InputComplete|InputDeadlineNotification)$' -count=1 -timeout=1m\n"
        "go test -race -tags lsp_trace_controlled_describe_fixture ./cmd/lsp-trace -run '^TestCensusDurableNormalFolderProcesses$' -count=1 -timeout=3m\n"
        "go test -race ./internal/publication -run '^TestTimedCompletion(KilledWriter|RejectsLateProof)$' -count=1 -timeout=1m\n"
        "go test -race -tags lsp_trace_controlled_describe_fixture ./cmd/lsp-trace -run '^TestCensusDefaultDurableInventoryProcesses$' -count=1 -timeout=3m",
        {'GOPROXY': 'off', 'GOSUMDB': 'off', 'GOTOOLCHAIN': 'local', 'GOTELEMETRY': 'off'}),
}


def condition_matches(owner, expected='success()'):
    """Only baseline conditions and harmless expression wrappers are supported.

    No condition/true has Actions' implicit success() guard. always()/cancelled()
    do not, so they are NOT interchangeable with true, even if currently truthy.
    YAML booleans are tested by identity, never Python truthiness of strings.
    """
    if 'if' not in owner:
        return expected == 'success()'
    value = owner['if']
    if type(value) is bool:
        return value is True and expected == 'success()'
    if not isinstance(value, str):
        return False
    value = value.strip()
    if value.startswith('${{') and value.endswith('}}'):
        value = value[3:-2].strip()
    return value == expected or (expected == 'success()' and value == 'true')


def unmasked(owner):
    return 'continue-on-error' not in owner or owner['continue-on-error'] is False


def validate_shard_envelope(job, platform, errors):
    """Bounded contract for the existing shared steps, including their containers."""
    name = 'test-' + platform
    allowed_job_keys = {'strategy', 'runs-on', 'env', 'steps', 'if', 'continue-on-error', 'timeout-minutes'}
    if not condition_matches(job) or not unmasked(job) or set(job) - allowed_job_keys:
        fail(errors, f'{name}: required job execution contract mismatch')
    if job.get('env') != {'PLATFORM': platform}:
        fail(errors, f'{name}: native platform/environment contract mismatch')
    strategy = job.get('strategy', {})
    if (set(strategy) != {'matrix', 'fail-fast'} or strategy.get('fail-fast') is not False
            or set(strategy.get('matrix', {})) != {'shard'}):
        fail(errors, f'{name}: full matrix execution contract mismatch')
    expected_steps = ['actions/checkout@v4', 'actions/setup-go@v5', 'Validate manifest and resolve packages',
                      'Install pinned gopls', 'Run shard once and summarize', *POSTCHECKS, 'Upload shard timing']
    steps = job.get('steps', [])
    # The first two roles are action identities, never human-facing display names.
    if [s.get('uses') if i < 2 else s.get('name', s.get('uses'))
            for i, s in enumerate(steps)] != expected_steps:
        fail(errors, f'{name}: required steps must occur exactly once in baseline order')
    for index, step in enumerate(steps):
        label = expected_steps[index] if index < 2 else step.get('name', step.get('uses', '<unnamed>'))
        allowed_keys = {'name', 'id', 'uses', 'with', 'run', 'if', 'env', 'shell', 'working-directory', 'continue-on-error'}
        environment = POSTCHECKS.get(label, (None, None, {}))[2]
        common_ok = (unmasked(step) and not (set(step) - allowed_keys)
                     and step.get('env', {}) == environment
                     and step.get('working-directory', '.') == '.'
                     and step.get('shell', 'bash') == 'bash')
        if label not in {*POSTCHECKS, 'Install pinned gopls', 'Upload shard timing'}:
            common_ok = common_ok and condition_matches(step)
        if label == 'Run shard once and summarize':
            common_ok = common_ok and step.get('shell') == 'bash' and 'uses' not in step
        if label == 'Upload shard timing':
            common_ok = (common_ok and step.get('uses') == 'actions/upload-artifact@v4'
                         and 'run' not in step and set(step.get('with', {})) == {'name', 'path'})
        if not common_ok:
            fail(errors, f'{name}: {label} execution contract mismatch')
        if label == 'actions/checkout@v4' and (step.get('uses') != 'actions/checkout@v4'
                                               or step.get('with', {}) or 'run' in step):
            fail(errors, f'{name}: checkout must use the default exact workflow revision')
        if label == 'actions/setup-go@v5' and (step.get('uses') != 'actions/setup-go@v5'
                                               or step.get('with') != {'go-version-file': 'go.mod'} or 'run' in step):
            fail(errors, f'{name}: setup-go contract mismatch')
        if label in POSTCHECKS:
            condition, command, _ = POSTCHECKS[label]
            if (not condition_matches(step, condition) or step.get('run', '').strip() != command
                    or 'uses' in step):
                fail(errors, f'{name}: {label} postcheck contract mismatch')


def validate(data):
    errors = []
    jobs = data.get("jobs")
    if not isinstance(jobs, dict):
        return ["workflow jobs must be a mapping"]
    if data.get('env') or data.get('defaults'):
        fail(errors, 'workflow: unsupported inherited environment/defaults')
    for platform, runner, count in (("linux", "ubuntu-latest", 5), ("darwin", "macos-latest", 4)):
        name = f"test-{platform}"
        job = jobs.get(name, {})
        validate_shard_envelope(job, platform, errors)
        matrix = job.get("strategy", {}).get("matrix", {}).get("shard", [])
        expected_ids = [f"{platform}-{i}" for i in range(1, count + 1)]
        if any(set(entry) != {'id', 'needs_gopls'} or type(entry.get('needs_gopls')) is not bool for entry in matrix):
            fail(errors, f'{name}: unsupported matrix entry schema')
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
        pipeline_ok, script_ok, telemetry_ok = validate_shard_script(run_step.get("run", ""))
        if not pipeline_ok:
            fail(errors, f"{name}: test invocation must be exactly one checked exhaustive plan")
        elif not telemetry_ok:
            fail(errors, f"{name}: test telemetry must use unique absolute paths under RUNNER_TEMP")
        elif not script_ok:
            fail(errors, f"{name}: checked runner arguments and unmasked exit status required")
        upload = step_by_name(job, "Upload shard timing") or {}
        if upload.get("if") != "always()":
            fail(errors, f"{name}: upload step must use if: always()")
        if upload.get("with", {}).get("path") != "${{ runner.temp }}/go-test-*/**\n":
            fail(errors, f"{name}: uploaded telemetry must use absolute runner.temp paths")
        if upload.get('with', {}).get('name') != 'go-test-${{ matrix.shard.id }}':
            fail(errors, f'{name}: existing shard artifact names must be preserved')
        if 'timeout-minutes' in job:
            fail(errors, f'{name}: job timeout policy must remain unchanged')
        install = step_by_name(job, "Install pinned gopls") or {}
        if install.get("if") != "matrix.shard.needs_gopls" or install.get("run") != "go install golang.org/x/tools/gopls@v0.23.0":
            fail(errors, f"{name}: gopls step mismatch")
    elixir = jobs.get("elixir-companion", {})
    if elixir.get("runs-on") != "ubuntu-22.04":
        fail(errors, "elixir-companion: pinned OTP 25.3.2.7 requires ubuntu-22.04")
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
