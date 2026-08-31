defmodule ElixirCallHierarchy.Index do
  @moduledoc false

  defstruct definitions: [], calls: [], unsupported: []

  def build(root) do
    temporary =
      Path.join(System.tmp_dir!(), "elixir-call-hierarchy-#{System.unique_integer([:positive])}")

    try do
      build(root, Path.join(temporary, "build"))
    after
      File.rm_rf!(temporary)
    end
  end

  def build(root, build_path, opts \\ []) do
    root = Path.expand(root)
    File.mkdir_p!(build_path)
    profile? = Keyword.get(opts, :profile, false)

    definitions =
      ElixirCallHierarchy.Profile.measure(profile?, "definition_parse", fn ->
        root |> Path.join("lib/**/*.ex") |> Path.wildcard() |> Enum.flat_map(&definitions/1)
      end)

    calls = compile(root, definitions, build_path, profile?)
    %__MODULE__{definitions: definitions, calls: calls, unsupported: []}
  end

  def prepare(%__MODULE__{definitions: definitions}, file, line) do
    definitions
    |> Enum.filter(
      &(&1.file == Path.expand(file) and line >= &1.range.start.line and line <= &1.range.end.line)
    )
    |> Enum.min_by(&range_size/1, fn -> nil end)
  end

  def incoming(%__MODULE__{} = index, target) do
    index.calls
    |> Enum.filter(&(&1.target == target.identity))
    |> Enum.group_by(& &1.caller)
    |> Enum.map(fn {identity, calls} ->
      definition = Enum.find(index.definitions, &(&1.identity == identity))

      %{
        definition: definition,
        ranges: calls |> Enum.map(& &1.call_range) |> Enum.uniq() |> Enum.sort_by(&range_key/1)
      }
    end)
    |> Enum.reject(&is_nil(&1.definition))
    |> Enum.sort_by(fn %{definition: definition} ->
      {definition.file, definition.range.start.line, definition.identity}
    end)
  end

  defp definitions(file) do
    source = File.read!(file)
    {:ok, ast} = Code.string_to_quoted(source, columns: true, token_metadata: true)
    {_ast, {_module, found}} = Macro.traverse(ast, {nil, []}, &pre/2, &post/2)

    found
    |> Enum.map(&%{&1 | file: Path.expand(file)})
    |> Enum.reverse()
    |> Enum.group_by(& &1.identity)
    |> Enum.map(fn {identity, clauses} ->
      first = Enum.min_by(clauses, & &1.range.start.line)
      last = Enum.max_by(clauses, & &1.range.end.line)
      %{first | identity: identity, range: %{start: first.range.start, end: last.range.end}}
    end)
  end

  defp pre({:defmodule, _meta, [{:__aliases__, _, parts}, _]} = node, {_module, found}),
    do: {node, {Module.concat(parts), found}}

  defp pre({kind, meta, [head | _]} = node, {module, found})
       when kind in [:def, :defp, :defmacro, :defmacrop] do
    {name, args} = function_head(head)
    arity = length(args || [])
    line = meta[:line]
    end_line = get_in(meta, [:end, :line]) || line
    column = max((meta[:column] || 1) - 1, 0)
    identity = {module, name, arity}

    definition = %{
      identity: identity,
      file: nil,
      kind: kind,
      range: range(line, column, end_line, column + String.length(to_string(name)))
    }

    {node, {module, [definition | found]}}
  end

  defp pre(node, state), do: {node, state}
  defp post(node, state), do: {node, state}

  defp compile(root, definitions, build_path, profile?) do
    previous_build = System.get_env("MIX_BUILD_PATH")
    previous_deps = System.get_env("MIX_DEPS_PATH")
    previous_options = Code.compiler_options()
    previous_code_path = :code.get_path()
    deps_path = Path.join(root, "deps")
    Process.register(self(), ElixirCallHierarchy.Collector)
    System.put_env("MIX_BUILD_PATH", build_path)
    System.put_env("MIX_DEPS_PATH", deps_path)

    try do
      ElixirCallHierarchy.Profile.measure(profile?, "deps_compile", fn ->
        compile_dependencies(root, deps_path, build_path)
      end)

      ElixirCallHierarchy.Profile.measure(profile?, "deps_loadpaths", fn ->
        add_dependency_paths(build_path)
      end)

      ElixirCallHierarchy.Profile.measure(profile?, "project_compile", fn ->
        compile_project(root, previous_options)
      end)

      ElixirCallHierarchy.Profile.measure(profile?, "tracer_drain", fn ->
        drain([], definitions)
      end)
    after
      Code.compiler_options(previous_options)
      :code.set_path(previous_code_path)
      Process.unregister(ElixirCallHierarchy.Collector)
      restore_env("MIX_BUILD_PATH", previous_build)
      restore_env("MIX_DEPS_PATH", previous_deps)
    end
  end

  defp compile_project(root, previous_options) do
    Mix.Project.in_project(:elixir_call_hierarchy_workspace, root, fn _ ->
      Mix.Task.clear()
      config = Path.join(root, "config/config.exs")
      if File.regular?(config), do: Mix.Task.run("loadconfig", [config])
      Code.ensure_loaded!(ElixirCallHierarchy.Tracer)
      Code.compiler_options(tracers: [ElixirCallHierarchy.Tracer])

      try do
        case Mix.Task.run("compile", ["--force", "--no-deps-check"]) do
          result when elem(result, 0) in [:ok, :noop] -> :ok
          result -> raise "workspace compilation failed: #{inspect(result)}"
        end
      after
        Code.compiler_options(previous_options)
        Mix.Task.clear()
      end
    end)
  end

  defp compile_dependencies(root, deps_path, build_path) do
    env = [{"MIX_DEPS_PATH", deps_path}, {"MIX_BUILD_PATH", build_path}]
    dependencies = active_dependencies(root)
    compile_dependencies(root, dependencies, build_path, env, MapSet.new())
  end

  defp compile_dependencies(root, dependencies, build_path, env, repaired) do
    pending = Enum.reject(dependencies, &MapSet.member?(repaired, &1.name))

    result =
      if MapSet.size(repaired) > 0 and pending == [] do
        {"", 0}
      else
        args =
          if MapSet.size(repaired) == 0,
            do: ["deps.compile"],
            else: ["deps.compile" | Enum.map(pending, & &1.name)]

        System.cmd("mix", args, cd: root, env: env, stderr_to_stdout: true)
      end

    case result do
      {_output, 0} ->
        :ok

      {output, status} ->
        case resource_failure(output, dependencies, build_path, repaired) do
          {:ok, app, dependency} ->
            repair_dependency(app, dependency, build_path, env)
            compile_dependencies(root, dependencies, build_path, env, MapSet.put(repaired, app))

          :error ->
            diagnostic = bounded_diagnostic(output)
            IO.puts(:stderr, "mix deps.compile failed (exit #{status}):\n#{diagnostic}")
            raise "dependency compilation failed with exit status #{status}"
        end
    end
  end

  defp active_dependencies(root) do
    Mix.Project.in_project(:elixir_call_hierarchy_workspace, root, fn _ ->
      Mix.Dep.clear_cached()

      try do
        Mix.Dep.load_and_cache()
        |> Enum.map(fn dependency ->
          %{name: Atom.to_string(dependency.app), path: Path.expand(dependency.opts[:dest])}
        end)
      after
        Mix.Dep.clear_cached()
      end
    end)
  end

  defp resource_failure(output, dependencies, build_path, repaired) do
    app =
      ~r/^==> ([a-zA-Z0-9_]+)$/m
      |> Regex.scan(output, capture: :all_but_first)
      |> List.last()
      |> case do
        [name] -> name
        _ -> nil
      end

    dependency = app && Enum.find(dependencies, &(&1.name == app))
    expected = app && Path.join([build_path, "lib", app, "priv"])

    if dependency && !MapSet.member?(repaired, app) &&
         File.dir?(Path.join(dependency.path, "priv")) && String.contains?(output, expected) do
      {:ok, app, dependency.path}
    else
      :error
    end
  end

  defp repair_dependency(app, dependency, build_path, env) do
    run_dependency_compiler("compile.app", dependency, env)

    Enum.each(["priv", "include"], fn resource ->
      source = Path.join(dependency, resource)

      if File.dir?(source) do
        destination = Path.join([build_path, "lib", app, resource])
        File.rm_rf!(destination)
        File.cp_r!(source, destination)
      end
    end)

    Enum.each(
      ["compile.erlang", "compile.elixir", "compile.app"],
      &run_dependency_compiler(&1, dependency, env)
    )
  end

  defp run_dependency_compiler(task, dependency, env) do
    case System.cmd("mix", [task], cd: dependency, env: env, stderr_to_stdout: true) do
      {_output, 0} ->
        :ok

      {output, status} ->
        diagnostic = bounded_diagnostic(output)
        IO.puts(:stderr, "mix #{task} failed (exit #{status}):\n#{diagnostic}")
        raise "dependency resource repair failed with exit status #{status}"
    end
  end

  defp add_dependency_paths(build_path) do
    build_path
    |> Path.join("lib/*/ebin")
    |> Path.wildcard()
    |> Enum.sort()
    |> Enum.each(&Code.prepend_path/1)
  end

  defp bounded_diagnostic(output) do
    limit = 8_192

    if byte_size(output) <= limit,
      do: output,
      else: binary_part(output, byte_size(output) - limit, limit)
  end

  defp drain(calls, definitions) do
    receive do
      {:compiler_trace, call} ->
        caller = {call.caller_module, call.caller_name, call.caller_arity}

        definition =
          Enum.find(definitions, &(&1.identity == caller and &1.file == Path.expand(call.file))) ||
            Enum.find(definitions, &(&1.identity == caller))

        record = %{
          caller: caller,
          caller_definition_range: definition && definition.range,
          target: {call.target_module, call.target_name, call.target_arity},
          call_range: call.call_range,
          kind: call.kind,
          toolchain: call.toolchain
        }

        drain([record | calls], definitions)
    after
      0 -> Enum.reverse(calls)
    end
  end

  defp function_head({:when, _, [head | _]}), do: function_head(head)
  defp function_head({name, _, args}) when is_atom(name), do: {name, args}

  defp range(line, column, end_line, end_column),
    do: %{
      start: %{line: line - 1, character: column},
      end: %{line: end_line - 1, character: max(end_column, column + 1)}
    }

  defp range_size(definition), do: definition.range.end.line - definition.range.start.line

  defp range_key(range),
    do: {range.start.line, range.start.character, range.end.line, range.end.character}

  defp restore_env(key, nil), do: System.delete_env(key)
  defp restore_env(key, value), do: System.put_env(key, value)
end
