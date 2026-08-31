defmodule ElixirCallHierarchy.CacheTest do
  use ExUnit.Case, async: false

  import ExUnit.CaptureIO

  alias ElixirCallHierarchy.{Cache, Index, Profile}

  setup do
    base = Path.join(System.tmp_dir!(), "ech-cache-test-#{System.unique_integer([:positive])}")
    workspace = Path.join(base, "workspace")
    cache = Path.join(base, "cache")
    File.mkdir_p!(Path.join(workspace, "lib"))
    File.mkdir_p!(Path.join(workspace, "config"))
    File.write!(Path.join(workspace, "mix.exs"), fixture_mix())
    File.write!(Path.join(workspace, "mix.lock"), "%{}\n")
    File.write!(Path.join(workspace, "config/config.exs"), "import Config\n")
    File.write!(Path.join(workspace, "lib/calls.ex"), fixture_source("one"))
    on_exit(fn -> File.rm_rf!(base) end)
    %{workspace: workspace, cache: cache}
  end

  test "CLI accepts stdio cache-dir reindex and profile and rejects unknown options" do
    assert {:ok, %{stdio: true, cache_dir: "/tmp/cache", reindex: true, profile: false}} =
             ElixirCallHierarchy.CLI.parse(["--stdio", "--cache-dir", "/tmp/cache", "--reindex"])

    assert {:ok, %{profile: true}} = ElixirCallHierarchy.CLI.parse(["--stdio", "--profile"])

    assert {:error, message} = ElixirCallHierarchy.CLI.parse(["--wat"])
    assert message =~ "unknown option"
  end

  test "profile emits machine-readable phase start and completion on stderr" do
    output =
      capture_io(:stderr, fn ->
        assert Profile.measure(true, "test_phase", fn -> :ok end) == :ok
      end)

    events =
      output
      |> String.split("\n", trim: true)
      |> Enum.map(fn "ECH_PROFILE " <> json -> Jason.decode!(json) end)

    assert [
             %{"phase" => "test_phase", "event" => "start"},
             %{
               "phase" => "test_phase",
               "event" => "complete",
               "duration_ms" => duration
             }
           ] = events

    assert is_number(duration) and duration >= 0
  end

  test "default cache directory is external and nonempty", %{workspace: workspace} do
    path = Cache.default_dir()
    refute path == ""
    refute String.starts_with?(Path.expand(path), Path.expand(workspace) <> "/")
  end

  test "fingerprint invalidates for lib config mix.lock and dependency source changes", ctx do
    first = Cache.fingerprint(ctx.workspace)

    for {relative, content} <- [
          {"lib/calls.ex", fixture_source("two")},
          {"config/config.exs", "import Config\nconfig :fixture, :x, 1\n"},
          {"mix.lock", "%{changed: true}\n"},
          {"deps/local/lib/dep.ex", "defmodule LocalDep, do: nil\n"}
        ] do
      File.mkdir_p!(Path.dirname(Path.join(ctx.workspace, relative)))
      File.write!(Path.join(ctx.workspace, relative), content)
      changed = Cache.fingerprint(ctx.workspace)
      refute changed == first, "expected #{relative} to invalidate the fingerprint"
      File.rm_rf!(Path.join(ctx.workspace, relative))
      restore(ctx.workspace, relative)
    end
  end

  test "directory symlinks do not escape fingerprint root", ctx do
    outside = Path.join(Path.dirname(ctx.workspace), "outside")
    File.mkdir_p!(outside)
    File.write!(Path.join(outside, "secret.ex"), "one")
    File.ln_s!(outside, Path.join(ctx.workspace, "lib/escape"))
    first = Cache.fingerprint(ctx.workspace)
    File.write!(Path.join(outside, "secret.ex"), "two")
    assert Cache.fingerprint(ctx.workspace) == first
  end

  test "JSON index round trips and rejects malformed schema or mismatched fingerprint" do
    index = %Index{definitions: [], calls: [], unsupported: []}
    encoded = Cache.encode_index(index, "fingerprint-one")
    assert {:ok, ^index} = Cache.decode_index(encoded, "fingerprint-one")
    assert {:error, :invalid_schema} = Cache.decode_index(encoded, "fingerprint-two")
    assert {:error, :invalid_schema} = Cache.decode_index(~s({"schema_version":999,"index":{}}))
    assert {:error, :invalid_json} = Cache.decode_index("not json")
  end

  test "second identical subprocess initialization does not repeat a compile-time side effect",
       ctx do
    side_effect = Path.join(Path.dirname(ctx.workspace), "side-effect")
    assert {_, 0} = external_load(ctx, side_effect)
    assert File.read!(side_effect) == "compiled\n"
    assert {_, 0} = external_load(ctx, side_effect)
    assert File.read!(side_effect) == "compiled\n"
  end

  test "reindex recompiles unchanged inputs", ctx do
    marker = Path.join(Path.dirname(ctx.workspace), "reindex-side-effect")
    previous = System.get_env("ECH_COMPILE_SIDE_EFFECT")
    System.put_env("ECH_COMPILE_SIDE_EFFECT", marker)

    try do
      assert {:miss, %Index{}} = Cache.load(ctx.workspace, cache_dir: ctx.cache)
      assert File.read!(marker) == "compiled\n"
      assert {:miss, %Index{}} = Cache.load(ctx.workspace, cache_dir: ctx.cache, reindex: true)
      assert File.read!(marker) == "compiled\ncompiled\n"
    after
      if previous,
        do: System.put_env("ECH_COMPILE_SIDE_EFFECT", previous),
        else: System.delete_env("ECH_COMPILE_SIDE_EFFECT")
    end
  end

  test "corrupt index is a miss and is atomically replaced", ctx do
    assert {:miss, %Index{}} = Cache.load(ctx.workspace, cache_dir: ctx.cache)
    index_file = cache_index(ctx)
    File.write!(index_file, "corrupt")
    assert {:miss, %Index{}} = Cache.load(ctx.workspace, cache_dir: ctx.cache)

    assert {:ok, %Index{}} =
             index_file |> File.read!() |> Cache.decode_index(Cache.fingerprint(ctx.workspace))
  end

  test "concurrent cold subprocesses compile once", ctx do
    side_effect = Path.join(Path.dirname(ctx.workspace), "concurrent-side-effect")

    results =
      1..4
      |> Task.async_stream(fn _ -> external_load(ctx, side_effect) end,
        ordered: false,
        timeout: 60_000
      )
      |> Enum.to_list()

    assert Enum.all?(results, &match?({:ok, {_, 0}}, &1))
    assert File.read!(side_effect) == "compiled\n"
  end

  test "workspace receives no build or cache artifacts", ctx do
    before = tree(ctx.workspace)
    assert {_, %Index{}} = Cache.load(ctx.workspace, cache_dir: ctx.cache)
    assert tree(ctx.workspace) == before
    refute File.exists?(Path.join(ctx.workspace, "_build"))
  end

  defp external_load(ctx, side_effect) do
    project = Path.expand("..", __DIR__)

    expression =
      "ElixirCallHierarchy.Cache.load(Enum.at(System.argv(), 0), cache_dir: Enum.at(System.argv(), 1))"

    System.cmd("mix", ["run", "-e", expression, "--", ctx.workspace, ctx.cache],
      cd: project,
      env: [{"ECH_COMPILE_SIDE_EFFECT", side_effect}],
      stderr_to_stdout: true
    )
  end

  defp cache_index(ctx) do
    Path.join([ctx.cache, Cache.fingerprint(ctx.workspace), "index.json"])
  end

  defp tree(path), do: path |> Path.join("**/*") |> Path.wildcard() |> Enum.sort()

  defp restore(workspace, "lib/calls.ex"),
    do: File.write!(Path.join(workspace, "lib/calls.ex"), fixture_source("one"))

  defp restore(workspace, "config/config.exs"),
    do: File.write!(Path.join(workspace, "config/config.exs"), "import Config\n")

  defp restore(workspace, "mix.lock"), do: File.write!(Path.join(workspace, "mix.lock"), "%{}\n")
  defp restore(_workspace, _relative), do: :ok

  defp fixture_mix do
    """
    defmodule Fixture.MixProject do
      use Mix.Project
      def project, do: [app: :fixture, version: "0.1.0", elixir: "~> 1.16", deps: []]
    end
    """
  end

  defp fixture_source(tag) do
    """
    if marker = System.get_env("ECH_COMPILE_SIDE_EFFECT") do
      File.write!(marker, "compiled\n", [:append])
    end

    defmodule Fixture.Calls do
      @tag #{inspect(tag)}
      def tag, do: @tag
      def leaf(v), do: v
      def caller(v), do: leaf(v)
    end
    """
  end
end
