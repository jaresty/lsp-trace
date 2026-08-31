#!/usr/bin/env python3
import json
import os
import pathlib
import select
import shutil
import subprocess
import tempfile
import time

PROJECT = pathlib.Path(__file__).resolve().parents[1]
ESCRIPT = PROJECT / "elixir-call-hierarchy"
TIMEOUT = 30


def require(condition, message):
    if not condition:
        raise AssertionError(message)
    print("PASS: " + message)


def message(method, request_id, params):
    body = json.dumps(
        {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params},
        separators=(",", ":"),
    ).encode()
    return b"Content-Length: %d\r\n\r\n" % len(body) + body


def read_message(output):
    header, body = output.split(b"\r\n\r\n", 1)
    length = int(header.split(b"Content-Length: ", 1)[1].splitlines()[0])
    return json.loads(body[:length])


def read_framed(stream, deadline):
    header = b""
    while b"\r\n\r\n" not in header:
        remaining = deadline - time.monotonic()
        if remaining <= 0 or not select.select([stream], [], [], remaining)[0]:
            raise AssertionError(f"actual escript response exceeded {TIMEOUT}s")
        header += os.read(stream.fileno(), 1)
    length = int(header.split(b"Content-Length: ", 1)[1].split(b"\r\n", 1)[0])
    body = b""
    while len(body) < length:
        remaining = deadline - time.monotonic()
        if remaining <= 0 or not select.select([stream], [], [], remaining)[0]:
            raise AssertionError(f"actual escript response exceeded {TIMEOUT}s")
        body += os.read(stream.fileno(), length - len(body))
    return json.loads(body)


def hierarchy(workspace, cache, source):
    env = {**os.environ, "XDG_CACHE_HOME": str(cache)}
    process = subprocess.Popen(
        [str(ESCRIPT), "--stdio"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd=PROJECT,
        env=env,
    )
    deadline = time.monotonic() + TIMEOUT
    try:
        process.stdin.write(message("initialize", 1, {"rootUri": workspace.resolve().as_uri()}))
        process.stdin.flush()
        require(read_framed(process.stdout, deadline)["result"]["capabilities"]["callHierarchyProvider"] is True, "initialize advertises call hierarchy")
        process.stdin.write(message("textDocument/prepareCallHierarchy", 2, {"textDocument": {"uri": source.resolve().as_uri()}, "position": {"line": 1, "character": 7}}))
        process.stdin.flush()
        prepared = read_framed(process.stdout, deadline)["result"]
        require(len(prepared) == 1 and prepared[0]["name"] == "leaf/0", "dependency-bearing workspace prepares leaf/0")
        process.stdin.write(message("callHierarchy/incomingCalls", 3, {"item": prepared[0]}))
        process.stdin.flush()
        incoming = read_framed(process.stdout, deadline)["result"]
        require(any(call["from"]["name"] == "caller/0" for call in incoming), "dependency-bearing workspace returns exact caller/callee hierarchy")
    finally:
        process.terminate()
        try:
            process.wait(timeout=2)
        except subprocess.TimeoutExpired:
            process.kill()


def invoke(workspace, cache, request, timeout=TIMEOUT):
    env = os.environ.copy()
    env.update({"XDG_CACHE_HOME": str(cache), "ECH_PROFILE": "1"})
    started = time.monotonic()
    try:
        result = subprocess.run(
            [str(ESCRIPT), "--stdio", "--profile"],
            input=request,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            cwd=PROJECT,
            env=env,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired as error:
        raise AssertionError(f"actual escript initialize exceeded {timeout}s") from error
    elapsed = time.monotonic() - started
    require(result.returncode == 0, f"actual escript exits successfully (stderr={result.stderr[-2000:].decode(errors='replace')!r})")
    return read_message(result.stdout), result.stderr.decode(errors="replace"), elapsed


def main():
    require(ESCRIPT.is_file(), "actual escript binary exists")
    with tempfile.TemporaryDirectory(prefix="ech-escript-integration-") as temporary:
        base = pathlib.Path(temporary)
        workspace = base / "workspace"
        cache = base / "cache"
        (workspace / "lib").mkdir(parents=True)
        (workspace / "deps").mkdir()
        shutil.copy2(PROJECT / "mix.lock", workspace / "mix.lock")
        jason = workspace / "deps" / "jason"
        shutil.copytree(PROJECT / "deps" / "jason", jason)
        resource = jason / "priv" / "static" / "resource_probe.txt"
        resource.parent.mkdir(parents=True)
        resource.write_text("assembled by Mix\n")
        probe = jason / "lib" / "jason" / "resource_probe.ex"
        probe.parent.mkdir(parents=True, exist_ok=True)
        probe.write_text(
            """defmodule Jason.ResourceProbe do
  resource = Application.app_dir(:jason, "priv/static/resource_probe.txt")
  @value File.read!(resource)
  def value, do: @value
end
"""
        )
        (workspace / "mix.exs").write_text(
            """defmodule EscriptFixture.MixProject do
  use Mix.Project
  def project, do: [app: :escript_fixture, version: \"0.1.0\", elixir: \"~> 1.16\", deps: [{:jason, \"~> 1.4\"}]]
end
"""
        )
        source = workspace / "lib" / "calls.ex"
        source.write_text(
            """defmodule EscriptFixture.Calls do
  def leaf, do: {Jason.ResourceProbe.value(), %Jason.DecodeError{}}
  def caller, do: leaf()
end
"""
        )
        uri = workspace.resolve().as_uri()
        initialize = message("initialize", 1, {"rootUri": uri})
        response, cold_stderr, cold_elapsed = invoke(workspace, cache, initialize)
        require(response["result"]["capabilities"]["callHierarchyProvider"] is True, "initialize advertises call hierarchy")

        hierarchy(workspace, cache, source)
        require("\"phase\":\"deps_compile\"" in cold_stderr, "cold initialize profiles dependency compilation")
        require("redefining module Jason" not in cold_stderr, "dependency compilation does not contaminate the escript VM")
        require(not (workspace / "_build").exists(), "workspace has no _build directory")
        require(any(cache.rglob("index.json")), "cache artifact is outside workspace")

        warm_response, warm_stderr, warm_elapsed = invoke(workspace, cache, initialize)
        require(warm_response["result"]["capabilities"]["callHierarchyProvider"] is True, "warm initialize succeeds")
        require("\"status\":\"hit\"" in warm_stderr, "warm initialize reports a cache hit")
        require("\"phase\":\"deps_compile\"" not in warm_stderr, "warm initialize does not recompile dependencies")
        require(warm_elapsed < min(5, max(1, cold_elapsed / 2)), "warm initialize completes quickly")

        print(f"PASS: cold={cold_elapsed:.3f}s warm={warm_elapsed:.3f}s")


if __name__ == "__main__":
    main()
