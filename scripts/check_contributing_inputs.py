#!/usr/bin/env python3
"""Repeatable bounded receipt guard witnesses; no production perturbation switches.

Run from any directory. Each mutant is restored in finally. Do not run concurrently
with edits/builds in this worktree. A compilation failure never counts as a kill.
"""
import json
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
SOURCE = ROOT / "internal/source/contributing_inputs.go"

# label, exact source reduction, replacement, guard, assertion result marker
MUTATIONS = [
    ("unknown-dependencies", '[]string{"unobserved dependencies are unaccounted for"}', '[]string{}', 'Accounting', 'ASSERT_INPUT_ACCOUNTING'),
    ("omitted-contribution", 'e.IncompleteReasons = append(e.IncompleteReasons, "missing input binding for contribution: "+id)', '// omitted missing binding', 'Accounting', 'ASSERT_INPUT_ACCOUNTING'),
    ("classification-binding", '"bounded-input/" + string(class)', '"bounded-input/SOURCE"', 'ClassifiedBytes', 'ASSERT_INPUT_CLASS_BYTES'),
    ("changed-bytes", '}, acquisition, content)', '}, acquisition, []byte("fixed"))', 'ClassifiedBytes', 'ASSERT_INPUT_CHANGED_BYTES'),
    ("lexical-scope", 'path.Clean(name) != name', 'path.Clean(name) == "never"', 'Scope', 'ASSERT_INPUT_SCOPE'),
    ("unknown-class", 'return nil, "", errors.New("unknown input classification")', '// accept unknown class', 'Scope', 'ASSERT_INPUT_CLASS'),
    ("symlink-scope", 'r.root.ReadFile(name)', 'os.ReadFile(r.root.Name()+"/"+name)', 'Symlink', 'ASSERT_INPUT_SYMLINK'),
    ("read-failure", 'if readErr != nil {', 'if false {', 'Failure', 'ASSERT_INPUT_FAILURE'),
    ("immutable-evidence", 'return detached, nil', 'return e, nil', 'Immutable', 'ASSERT_INPUT_IMMUTABLE'),
    ("unknown-reference", 'if _, ok := r.inputs[ref]; !ok {', 'if _, ok := r.inputs[ref]; !ok && false {', 'Duplicates', 'ASSERT_INPUT_BINDING'),
    ("duplicate-reference", 'if seen[ref] {', 'if seen[ref] && false {', 'Duplicates', 'ASSERT_INPUT_BINDING'),
    ("duplicate-contribution", 'if _, ok := r.contributions[id]; ok {', 'if _, ok := r.contributions[id]; ok && false {', 'Duplicates', 'ASSERT_INPUT_DUPLICATE'),
    ("duplicate-expected", '(i > 0 && ordered[i-1] == id)', '(i > 0 && ordered[i-1] == id && false)', 'Duplicates', 'ASSERT_INPUT_DUPLICATE'),
    ("canonical-order", 'sort.Strings(ordered)', '// skip expected ordering', 'Order', 'ASSERT_INPUT_ORDER'),
]


def run(guard):
    command = ["go", "test", "./internal/source", "-run", "^TestInputRecorder" + guard + "$", "-count=1", "-json"]
    result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True)
    events = []
    for line in result.stdout.splitlines():
        try:
            events.append(json.loads(line))
        except json.JSONDecodeError:
            pass
    name = "TestInputRecorder" + guard
    actions = [e.get("Action") for e in events if e.get("Test") == name]
    output = "".join(e.get("Output", "") for e in events if e.get("Test") == name)
    return result.returncode, actions, output, command


def main():
    original = SOURCE.read_text()
    records = []
    try:
        for label, old, new, guard, marker in MUTATIONS:
            if original.count(old) != 1:
                raise RuntimeError("mutation must match once: " + label)
            SOURCE.write_text(original)
            code, actions, _, command = run(guard)
            if code != 0 or "pass" not in actions:
                raise RuntimeError("baseline guard did not pass: " + label)
            SOURCE.write_text(original.replace(old, new))
            code, actions, output, _ = run(guard)
            if code == 0 or "fail" not in actions or marker not in output:
                raise RuntimeError("no assertion-specific failure for " + label + ": " + output)
            records.append({"perturbation": label, "procedure": " ".join(command), "assertion": marker,
                            "satisfying": "PASS TestInputRecorder" + guard,
                            "violating": output.strip()})
            print("WITNESS " + label + ": PASS -> FAIL " + marker, flush=True)
    finally:
        SOURCE.write_text(original)
    if len(sys.argv) == 2:
        dest = pathlib.Path(sys.argv[1])
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(json.dumps(records, indent=2) + "\n")
        print("Witness records written: " + str(dest))
    print("All " + str(len(records)) + " assertion-specific perturbations rejected; source restored.")


if __name__ == "__main__":
    main()
