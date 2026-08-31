defmodule ElixirCallHierarchy.WorkerProcess do
  @moduledoc false

  @bundle_version 1
  @schema_version 2
  @indexer_version 2
  @diagnostic_limit 8_192
  @source_files ~w(call.ex tracer.ex index.ex)
  for file <- @source_files, do: @external_resource(Path.join(__DIR__, file))
  @sources Enum.map(@source_files, &File.read!(Path.join(__DIR__, &1)))

  def bundle_version, do: @bundle_version
  def schema_version, do: @schema_version
  def indexer_version, do: @indexer_version

  def bundle_digest do
    :crypto.hash(:sha256, bundle()) |> Base.encode16(case: :lower)
  end

  def run(root, entry, fingerprint, profile?) do
    bundle_path = Path.join(entry, "worker-v#{@bundle_version}.exs")
    artifact_path = Path.join(entry, "worker-artifact.json")
    build_path = Path.join(entry, "build")
    File.write!(bundle_path, bundle(), [:binary, :sync])
    File.rm(artifact_path)

    env = [
      {"MIX_ENV", System.get_env("MIX_ENV") || Atom.to_string(Mix.env())},
      {"MIX_TARGET", System.get_env("MIX_TARGET") || "host"},
      {"MIX_DEPS_PATH", Path.join(root, "deps")},
      {"MIX_BUILD_PATH", build_path}
    ]

    try do
      args = [bundle_path, root, build_path, artifact_path, fingerprint, to_string(profile?)]
      {output, status} = System.cmd(elixir_executable(), args, env: env, stderr_to_stdout: true)
      relay_diagnostics(output)

      with 0 <- status,
           {:ok, json} <- File.read(artifact_path),
           {:ok, index} <- ElixirCallHierarchy.Cache.decode_index(json, fingerprint) do
        {:ok, index}
      else
        _ -> raise "workspace index worker failed with exit status #{status}"
      end
    after
      File.rm(artifact_path)
    end
  end

  defp elixir_executable do
    System.find_executable("elixir") || raise "inherited elixir executable not found"
  end

  defp relay_diagnostics(""), do: :ok

  defp relay_diagnostics(output) do
    output =
      if byte_size(output) <= @diagnostic_limit,
        do: output,
        else: binary_part(output, byte_size(output) - @diagnostic_limit, @diagnostic_limit)

    IO.write(:stderr, output)
  end

  defp bundle do
    [worker_support(), "\n", Enum.intersperse(@sources, "\n"), "\n", worker_main()]
    |> IO.iodata_to_binary()
  end

  defp worker_support do
    ~S'''
    defmodule ElixirCallHierarchy.WorkerJSON do
      def encode(value), do: value |> json() |> IO.iodata_to_binary()
      defp json(nil), do: "null"
      defp json(true), do: "true"
      defp json(false), do: "false"
      defp json(value) when is_integer(value), do: Integer.to_string(value)
      defp json(value) when is_float(value), do: :erlang.float_to_binary(value, [:compact])
      defp json(value) when is_atom(value), do: json(Atom.to_string(value))
      defp json(value) when is_binary(value), do: [34, escape(value), 34]
      defp json(value) when is_list(value), do: [91, join(Enum.map(value, &json/1)), 93]
      defp json(value) when is_map(value) do
        fields = value |> Enum.map(fn {k, v} -> {to_string(k), v} end) |> Enum.sort_by(&elem(&1, 0))
        [123, join(Enum.map(fields, fn {k, v} -> [json(k), 58, json(v)] end)), 125]
      end
      defp join([]), do: []
      defp join([one]), do: one
      defp join([head | tail]), do: [head, 44, join(tail)]
      defp escape(value), do: for(<<c::utf8 <- value>>, do: escaped(c))
      defp escaped(34), do: "\\\""
      defp escaped(92), do: "\\\\"
      defp escaped(8), do: "\\b"
      defp escaped(9), do: "\\t"
      defp escaped(10), do: "\\n"
      defp escaped(12), do: "\\f"
      defp escaped(13), do: "\\r"
      defp escaped(c) when c < 32, do: ["\\u", c |> Integer.to_string(16) |> String.pad_leading(4, "0")]
      defp escaped(c), do: <<c::utf8>>
    end

    defmodule ElixirCallHierarchy.Profile do
      def measure(false, _phase, fun), do: fun.()
      def measure(true, phase, fun) do
        emit(true, phase, %{event: "start"})
        started = System.monotonic_time()
        try do
          fun.()
        after
          elapsed = System.monotonic_time() - started
          emit(true, phase, %{event: "complete", duration_ms: System.convert_time_unit(elapsed, :native, :microsecond) / 1_000})
        end
      end
      def emit(false, _phase, _fields), do: :ok
      def emit(true, phase, fields) do
        IO.puts(:stderr, "ECH_PROFILE " <> ElixirCallHierarchy.WorkerJSON.encode(Map.put(fields, :phase, phase)))
      end
    end
    '''
  end

  defp worker_main do
    """
    defmodule ElixirCallHierarchy.WorkerMain do
      @schema_version #{@schema_version}
      @indexer_version #{@indexer_version}
      @bundle_version #{@bundle_version}

      def run([root, build, artifact, fingerprint, profile]) do
        index = ElixirCallHierarchy.Index.build(root, build, profile: profile == "true")
        value = %{
          "schema_version" => @schema_version,
          "indexer_version" => @indexer_version,
          "bundle_version" => @bundle_version,
          "fingerprint" => fingerprint,
          "index" => %{
            "definitions" => Enum.map(index.definitions, &definition/1),
            "calls" => Enum.map(index.calls, &call/1),
            "unsupported" => index.unsupported
          }
        }
        temporary = artifact <> ".tmp"
        File.write!(temporary, ElixirCallHierarchy.WorkerJSON.encode(value), [:binary, :sync])
        File.rename!(temporary, artifact)
      end

      defp definition(value), do: %{
        "identity" => identity(value.identity), "file" => value.file,
        "kind" => to_string(value.kind), "range" => value.range
      }
      defp call(value), do: %{
        "caller" => identity(value.caller), "caller_definition_range" => value.caller_definition_range,
        "target" => identity(value.target), "call_range" => value.call_range,
        "kind" => to_string(value.kind), "toolchain" => value.toolchain
      }
      defp identity({module, name, arity}), do: [Atom.to_string(module), Atom.to_string(name), arity]
      defp identity([module, name, arity]), do: [module, name, arity]
    end

    {:ok, _} = Application.ensure_all_started(:mix)
    ElixirCallHierarchy.WorkerMain.run(System.argv())
    """
  end
end
