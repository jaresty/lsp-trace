#!/usr/bin/env python3
import contextlib
import importlib.util
import io
import json
import pathlib
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]


def load(name):
    path = ROOT / "scripts" / f"check-program-c-{name}.py"
    spec = importlib.util.spec_from_file_location(name.replace("-", "_"), path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def run(module):
    output = io.StringIO()
    with contextlib.redirect_stdout(output):
        status = module.main()
    return status, output.getvalue()


def main():
    i03 = load("i-03")
    i04 = load("i-04")

    for assertion, module in (
        ("ASSERT_I03_EXACT_CANDIDATES", i03),
        ("ASSERT_I04_EXACT_ENTRIES", i04),
    ):
        status, output = run(module)
        if status != 0 or f"{assertion} result=PASS" not in output:
            raise SystemExit(f"FAIL {assertion} frozen approved packet\n{output}")
        print(f"PASS {assertion} frozen approved packet")

    inventory_source = ROOT / "qualification/program-c/i-03-candidate-inventory.v1.json"
    inventory = json.loads(inventory_source.read_text(encoding="utf-8"))
    inventory["candidates"][0]["version"] = "v0.17.1"
    with tempfile.TemporaryDirectory() as directory:
        wrong = pathlib.Path(directory) / inventory_source.name
        wrong.write_text(json.dumps(inventory), encoding="utf-8")
        original = i03.PATH
        try:
            i03.PATH = wrong
            status, output = run(i03)
        finally:
            i03.PATH = original
    expected = "ASSERT_I03_EXACT_CANDIDATES result=FAIL"
    if status == 0 or expected not in output:
        raise SystemExit(f"FAIL ASSERT_I03_WRONG_VERSION_REJECTED\n{output}")
    print(f"PASS ASSERT_I03_WRONG_VERSION_REJECTED observed={expected}")

    source = ROOT / "qualification/program-c/i-04-license-inputs.v1.json"
    packet = json.loads(source.read_text(encoding="utf-8"))
    packet["entries"][0]["sources"][0] = "https://github.com/gonum/gonum/blob/v0.17.0/LICENSE.txt"
    with tempfile.TemporaryDirectory() as directory:
        wrong = pathlib.Path(directory) / source.name
        wrong.write_text(json.dumps(packet), encoding="utf-8")
        original = i04.P
        try:
            i04.P = wrong
            status, output = run(i04)
        finally:
            i04.P = original
    expected = "ASSERT_I04_EXACT_ENTRIES result=FAIL"
    if status == 0 or expected not in output:
        raise SystemExit(f"FAIL ASSERT_I04_WRONG_LICENSE_URL_REJECTED\n{output}")
    print(f"PASS ASSERT_I04_WRONG_LICENSE_URL_REJECTED observed={expected}")
    print("PROGRAM C DOCUMENTARY SOURCE TESTS PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
