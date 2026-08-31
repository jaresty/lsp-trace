defmodule ElixirCallHierarchy.Cache do
  @moduledoc false

  alias ElixirCallHierarchy.Index

  @schema_version 1
  @indexer_version 1
  @lock_wait_ms 30_000
  @excluded_dirs MapSet.new([
                   ".git",
                   ".hg",
                   ".svn",
                   "_build",
                   ".elixir_ls",
                   ".lexical",
                   "node_modules"
                 ])

  def default_dir do
    case System.get_env("XDG_CACHE_HOME") do
      value when is_binary(value) and value != "" -> Path.join(value, "elixir-call-hierarchy")
      _ -> basedir_cache()
    end
  end

  def fingerprint(root) do
    root = Path.expand(root)

    identity = [
      "schema=#{@schema_version}",
      "indexer=#{@indexer_version}",
      "elixir=#{System.version()}",
      "otp=#{System.otp_release()}",
      "mix=#{mix_version()}",
      "os=#{inspect(:os.type())}",
      "arch=#{:erlang.system_info(:system_architecture)}",
      "mix_env=#{Mix.env()}",
      "workspace=#{root}"
    ]

    files =
      ["mix.exs", "mix.lock", "config", "lib", "deps"]
      |> Enum.flat_map(&fingerprint_entries(root, &1))
      |> Enum.sort()

    :crypto.hash(:sha256, Enum.join(identity ++ files, "\n")) |> Base.encode16(case: :lower)
  end

  def load(root, opts \\ []) do
    root = Path.expand(root)
    cache_dir = opts |> Keyword.get(:cache_dir, default_dir()) |> Path.expand()
    fingerprint = fingerprint(root)
    entry = Path.join(cache_dir, fingerprint)
    index_path = Path.join(entry, "index.json")
    reindex? = Keyword.get(opts, :reindex, false)

    case read_index(index_path, fingerprint) do
      {:ok, index} when not reindex? -> {:hit, index}
      _ -> locked_load(root, cache_dir, entry, index_path, fingerprint, opts)
    end
  end

  def encode_index(%Index{} = index, fingerprint) when is_binary(fingerprint) do
    Jason.encode!(%{
      "schema_version" => @schema_version,
      "indexer_version" => @indexer_version,
      "fingerprint" => fingerprint,
      "index" => %{
        "definitions" => Enum.map(index.definitions, &encode_definition/1),
        "calls" => Enum.map(index.calls, &encode_call/1),
        "unsupported" => index.unsupported
      }
    })
  end

  def decode_index(json, expected_fingerprint \\ nil) when is_binary(json) do
    with {:ok, value} <- Jason.decode(json),
         %{
           "schema_version" => @schema_version,
           "indexer_version" => @indexer_version,
           "fingerprint" => fingerprint,
           "index" => %{
             "definitions" => definitions,
             "calls" => calls,
             "unsupported" => unsupported
           }
         } <- value,
         true <- is_binary(fingerprint),
         true <- is_nil(expected_fingerprint) or fingerprint == expected_fingerprint,
         true <- is_list(definitions) and is_list(calls) and is_list(unsupported),
         {:ok, definitions} <- map_all(definitions, &decode_definition/1),
         {:ok, calls} <- map_all(calls, &decode_call/1) do
      {:ok, %Index{definitions: definitions, calls: calls, unsupported: unsupported}}
    else
      {:error, %Jason.DecodeError{}} -> {:error, :invalid_json}
      _ -> {:error, :invalid_schema}
    end
  end

  defp locked_load(root, cache_dir, entry, index_path, fingerprint, opts) do
    File.mkdir_p!(cache_dir)
    lock = entry <> ".lock"

    with_lock(lock, fn ->
      reindex? = Keyword.get(opts, :reindex, false)

      if reindex? do
        File.rm_rf!(entry)
      end

      case read_index(index_path, fingerprint) do
        {:ok, index} when not reindex? -> {:hit, index}
        _ -> rebuild(root, entry, index_path, fingerprint)
      end
    end)
  end

  defp rebuild(root, entry, index_path, fingerprint) do
    File.mkdir_p!(entry)
    build = Path.join(entry, "build")
    File.rm_rf!(build)
    File.mkdir_p!(build)
    index = Index.build(root, build) |> normalize_index()
    atomic_write(index_path, encode_index(index, fingerprint))
    {:miss, index}
  end

  defp read_index(path, fingerprint) do
    with {:ok, json} <- File.read(path),
         {:ok, index} <- decode_index(json, fingerprint),
         do: {:ok, index}
  end

  defp with_lock(lock, fun) do
    deadline = System.monotonic_time(:millisecond) + @lock_wait_ms
    acquire_lock(lock, deadline, fun)
  end

  defp acquire_lock(lock, deadline, fun) do
    case File.mkdir(lock) do
      :ok ->
        try do
          fun.()
        after
          File.rmdir(lock)
        end

      {:error, :eexist} ->
        if System.monotonic_time(:millisecond) >= deadline do
          raise "timed out waiting for cache lock #{lock}"
        else
          Process.sleep(50)
          acquire_lock(lock, deadline, fun)
        end

      {:error, reason} ->
        raise File.Error, reason: reason, action: "acquire cache lock", path: lock
    end
  end

  defp atomic_write(path, contents) do
    temporary = path <> ".tmp-#{System.unique_integer([:positive])}"
    File.write!(temporary, contents, [:binary, :sync])
    File.rename!(temporary, path)
  end

  defp fingerprint_entries(root, relative) do
    path = Path.join(root, relative)

    case File.lstat(path) do
      {:ok, %File.Stat{type: :regular}} -> [file_entry(root, path)]
      {:ok, %File.Stat{type: :directory}} -> walk(root, path)
      _ -> []
    end
  end

  defp walk(root, directory) do
    directory
    |> File.ls!()
    |> Enum.sort()
    |> Enum.flat_map(fn name ->
      path = Path.join(directory, name)

      case File.lstat(path) do
        {:ok, %File.Stat{type: :directory}} ->
          if MapSet.member?(@excluded_dirs, name), do: [], else: walk(root, path)

        {:ok, %File.Stat{type: :regular}} ->
          if junk?(name), do: [], else: [file_entry(root, path)]

        _ ->
          []
      end
    end)
  end

  defp junk?(name), do: name in [".DS_Store"] or String.ends_with?(name, [".beam", ".pyc", "~"])

  defp file_entry(root, path) do
    relative = Path.relative_to(path, root)
    digest = :crypto.hash(:sha256, File.read!(path)) |> Base.encode16(case: :lower)
    relative <> "\0" <> digest
  end

  defp normalize_index(%Index{} = index) do
    %Index{
      definitions: Enum.map(index.definitions, &normalize_definition/1),
      calls: Enum.map(index.calls, &normalize_call/1),
      unsupported: index.unsupported
    }
  end

  defp normalize_definition(definition) do
    %{
      definition
      | identity: encode_identity(definition.identity),
        kind: Atom.to_string(definition.kind)
    }
  end

  defp normalize_call(call) do
    %{
      call
      | caller: encode_identity(call.caller),
        target: encode_identity(call.target),
        kind: Atom.to_string(call.kind)
    }
  end

  defp encode_definition(definition) do
    %{
      "identity" => encode_identity(definition.identity),
      "file" => definition.file,
      "kind" => to_string(definition.kind),
      "range" => definition.range
    }
  end

  defp decode_definition(%{
         "identity" => identity,
         "file" => file,
         "kind" => kind,
         "range" => range
       })
       when is_list(identity) and length(identity) == 3 and is_binary(file) and is_binary(kind) do
    with {:ok, range} <- decode_range(range),
         do: {:ok, %{identity: identity, file: file, kind: kind, range: range}}
  end

  defp decode_definition(_), do: :error

  defp encode_call(call) do
    %{
      "caller" => encode_identity(call.caller),
      "caller_definition_range" => call.caller_definition_range,
      "target" => encode_identity(call.target),
      "call_range" => call.call_range,
      "kind" => to_string(call.kind),
      "toolchain" => call.toolchain
    }
  end

  defp decode_call(%{"caller" => caller, "target" => target, "call_range" => call_range} = call)
       when is_list(caller) and length(caller) == 3 and is_list(target) and length(target) == 3 do
    with {:ok, call_range} <- decode_range(call_range),
         {:ok, definition_range} <- decode_optional_range(call["caller_definition_range"]) do
      {:ok,
       %{
         caller: caller,
         caller_definition_range: definition_range,
         target: target,
         call_range: call_range,
         kind: call["kind"],
         toolchain: call["toolchain"]
       }}
    end
  end

  defp decode_call(_), do: :error

  defp decode_optional_range(nil), do: {:ok, nil}
  defp decode_optional_range(range), do: decode_range(range)

  defp decode_range(%{"start" => start, "end" => finish}) do
    with {:ok, start} <- decode_position(start),
         {:ok, finish} <- decode_position(finish),
         do: {:ok, %{start: start, end: finish}}
  end

  defp decode_range(_), do: :error

  defp decode_position(%{"line" => line, "character" => character})
       when is_integer(line) and line >= 0 and is_integer(character) and character >= 0,
       do: {:ok, %{line: line, character: character}}

  defp decode_position(_), do: :error

  defp encode_identity({module, name, arity}),
    do: [Atom.to_string(module), Atom.to_string(name), arity]

  defp encode_identity([module, name, arity]), do: [module, name, arity]

  defp map_all(values, mapper) do
    Enum.reduce_while(values, {:ok, []}, fn value, {:ok, result} ->
      case mapper.(value) do
        {:ok, mapped} -> {:cont, {:ok, [mapped | result]}}
        _ -> {:halt, :error}
      end
    end)
    |> case do
      {:ok, result} -> {:ok, Enum.reverse(result)}
      _ -> :error
    end
  end

  defp mix_version do
    case :application.get_key(:mix, :vsn) do
      {:ok, version} -> to_string(version)
      _ -> System.version()
    end
  end

  defp basedir_cache do
    try do
      :filename.basedir(:user_cache, "elixir-call-hierarchy") |> List.to_string()
    rescue
      _ -> Path.join(System.user_home!(), ".cache/elixir-call-hierarchy")
    end
  end
end
