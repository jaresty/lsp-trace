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


def hierarchy(workspace, cache, source, mix_env):
    env = {
        **os.environ,
        "XDG_CACHE_HOME": str(cache),
        "MIX_ENV": mix_env,
        "MIX_TARGET": "host",
    }
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
        process.stdin.write(message("textDocument/prepareCallHierarchy", 2, {"textDocument": {"uri": source.resolve().as_uri()}, "position": {"line": 3, "character": 7}}))
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


def invoke(workspace, cache, request, mix_env, timeout=TIMEOUT):
    env = os.environ.copy()
    env.update(
        {
            "XDG_CACHE_HOME": str(cache),
            "ECH_PROFILE": "1",
            "MIX_ENV": mix_env,
            "MIX_TARGET": "host",
        }
    )
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
    require(result.returncode == 0, f"actual escript exits successfully (stderr={result.stderr[-8000:].decode(errors='replace')!r})")
    return read_message(result.stdout), result.stderr.decode(errors="replace"), elapsed


def main():
    require(ESCRIPT.is_file(), "actual escript binary exists")
    with tempfile.TemporaryDirectory(prefix="ech-escript-integration-") as temporary:
        base = pathlib.Path(temporary)
        workspace = base / "workspace"
        cache = base / "cache"
        (workspace / "lib").mkdir(parents=True)
        (workspace / "deps").mkdir()
        (workspace / "config").mkdir()
        (workspace / "config" / "config.exs").write_text(
            'import Config\nconfig :escript_fixture, :compile_value, :configured\n'
        )

        def dependency(name, deps="[]", body="def value, do: :ok"):
            root = workspace / "deps" / name
            (root / "lib").mkdir(parents=True)
            module = "".join(part.capitalize() for part in name.split("_"))
            (root / "mix.exs").write_text(
                f"""defmodule {module}.MixProject do
  use Mix.Project
  def project, do: [app: :{name}, version: \"0.1.0\", deps: {deps}]
end
"""
            )
            (root / "lib" / f"{name}.ex").write_text(
                f"defmodule {module} do\n  {body}\nend\n"
            )

        dependency("active_transitive")
        dependency(
            "test_only",
            '[{:active_transitive, path: "../active_transitive"}]',
            "@value ActiveTransitive.value()\n  def value, do: @value",
        )
        dependency("prod_only")
        dependency("target_only")
        shutil.copy2(PROJECT / "mix.lock", workspace / "mix.lock")
        jason = workspace / "deps" / "jason"
        shutil.copytree(PROJECT / "deps" / "jason", jason)
        jason_module = jason / "lib" / "jason.ex"
        jason_module.write_text(
            jason_module.read_text().replace(
                "defmodule Jason do\n",
                "defmodule Jason do\n  def encode(:workspace_collision, :incompatible, :arity), do: {:ok, :workspace_version}\n",
                1,
            )
        )
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
            """defmodule Mix.Tasks.Compile.FixtureMarker do
  use Mix.Task.Compiler
  def run(_) do
    if Mix.env() == :test and not Code.ensure_loaded?(ActiveTransitive) do
      raise "active transitive dependency module is unavailable during project compilation"
    end

    File.write!(Path.join(Mix.Project.build_path(), "fixture-compiler-ran"), "ok")
    {:ok, []}
  end
end

defmodule EscriptFixture.MixProject do
  use Mix.Project
  def project do
    [
      app: :escript_fixture,
      version: \"0.1.0\",
      elixir: \"~> 1.16\",
      compilers: [:fixture_marker] ++ Mix.compilers(),
      deps: [
        {:jason, \"~> 1.4\"},
        {:test_only, path: \"deps/test_only\", only: :test},
        {:prod_only, path: \"deps/prod_only\", only: :prod},
        {:target_only, path: \"deps/target_only\", targets: [:special]}
      ]
    ]
  end
end
"""
        )
        source = workspace / "lib" / "calls.ex"
        source.write_text(
            """defmodule EscriptFixture.Calls do
  @workspace_version Jason.encode(:workspace_collision, :incompatible, :arity)
  @compile_value Application.compile_env!(:escript_fixture, :compile_value)
  def leaf, do: {Jason.ResourceProbe.value(), %Jason.DecodeError{}, @workspace_version, @compile_value}
  def caller, do: leaf()
end
"""
        )
        clean_probe = subprocess.run(
            [
                shutil.which("elixir"),
                "-pa",
                str(cache / "clean-probe" / "lib" / "jason" / "ebin"),
                "-e",
                "Code.require_file(\"lib/jason.ex\", System.argv() |> hd()); IO.inspect(Jason.encode(:workspace_collision, :incompatible, :arity))",
                str(jason),
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env={**os.environ, "MIX_ENV": "test", "MIX_TARGET": "host"},
            check=False,
        )
        require(
            clean_probe.returncode == 0 and b"{:ok, :workspace_version}" in clean_probe.stdout,
            "clean worker uses workspace collision module encode/3",
        )
        uri = workspace.resolve().as_uri()
        initialize = message("initialize", 1, {"rootUri": uri})
        response, cold_stderr, cold_elapsed = invoke(workspace, cache / "test", initialize, "test")
        require(response["result"]["capabilities"]["callHierarchyProvider"] is True, "actual escript isolates colliding workspace module")
        require(response["jsonrpc"] == "2.0" and response["id"] == 1, "parent JSON framing survives workspace collision")
        require("encode/3" not in cold_stderr, "companion Jason remains uncontaminated")
        require("Unknown dependency prod_only" not in cold_stderr, "test excludes restored prod-only dependency")
        require("Unknown dependency target_only" not in cold_stderr, "host target excludes restored special-target dependency")
        require("undefined function ActiveTransitive.value/0" not in cold_stderr, "active transitive compiles before test-only dependent")
        require(any(cache.rglob("fixture-compiler-ran")), "workspace custom compiler runs in Mix order")

        hierarchy(workspace, cache / "test", source, "test")
        require("\"phase\":\"deps_compile\"" in cold_stderr, "cold initialize profiles dependency compilation")
        require("redefining module Jason" not in cold_stderr, "dependency compilation does not contaminate the escript VM")
        require(not (workspace / "_build").exists(), "workspace has no _build directory")
        require(any(cache.rglob("index.json")), "cache artifact is outside workspace")

        prod_response, prod_stderr, _ = invoke(workspace, cache / "prod", initialize, "prod")
        require(prod_response["result"]["capabilities"]["callHierarchyProvider"] is True, "prod initialize advertises call hierarchy")
        require("Unknown dependency test_only" not in prod_stderr, "prod excludes restored test-only dependency")

        warm_response, warm_stderr, warm_elapsed = invoke(workspace, cache / "test", initialize, "test")
        require(warm_response["result"]["capabilities"]["callHierarchyProvider"] is True, "warm initialize succeeds")
        require("\"status\":\"hit\"" in warm_stderr, "warm initialize reports a cache hit")
        require("\"phase\":\"deps_compile\"" not in warm_stderr, "warm initialize does not recompile dependencies")
        require(warm_elapsed < min(5, max(1, cold_elapsed / 2)), "warm initialize completes quickly")

        print(f"PASS: cold={cold_elapsed:.3f}s warm={warm_elapsed:.3f}s")


if __name__ == "__main__":
    main()
