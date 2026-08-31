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

  def document_symbols(%__MODULE__{definitions: definitions}, file) do
    file = Path.expand(file)

    definitions
    |> Enum.filter(&(&1.file == file))
    |> Enum.sort_by(fn definition ->
      {definition.range.start.line, definition.range.start.character, definition.identity}
    end)
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

      ElixirCallHierarchy.Profile.measure(profile?, "project_compile", fn ->
        compile_project(root, previous_options, profile?)
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

  defp compile_project(root, previous_options, profile?) do
    Mix.Project.in_project(:elixir_call_hierarchy_workspace, root, fn _ ->
      Mix.Task.clear()
      config = Path.join(root, "config/config.exs")
      if File.regular?(config), do: Mix.Task.run("loadconfig", [config])

      ElixirCallHierarchy.Profile.measure(profile?, "deps_loadpaths", fn ->
        Mix.Task.run("deps.loadpaths", ["--no-deps-check"])
      end)

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
    env = dependency_compiler_env(deps_path, build_path)
    dependencies = active_dependencies(root)
    compile_dependencies(root, dependencies, build_path, env, MapSet.new())
    ensure_dependency_outputs(root, dependencies, build_path, env)
  end

  defp dependency_compiler_env(deps_path, build_path) do
    root_dependency_lib = Path.join(build_path, "lib")

    erl_libs =
      [root_dependency_lib, System.get_env("ERL_LIBS")]
      |> Enum.reject(&(&1 in [nil, ""]))
      |> Enum.uniq()
      |> Enum.join(if(match?({:win32, _}, :os.type()), do: ";", else: ":"))

    [
      {"MIX_DEPS_PATH", deps_path},
      {"MIX_BUILD_PATH", build_path},
      {"ERL_LIBS", erl_libs}
    ]
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

  defp ensure_dependency_outputs(root, dependencies, build_path, env) do
    dependencies
    |> Enum.filter(&source_bearing_dependency?/1)
    |> Enum.filter(&(missing_dependency_beams(&1, build_path) != []))
    |> Enum.each(fn dependency ->
      force_output = force_compile_dependency(root, dependency, build_path, env)

      repair_output =
        if missing_dependency_beams(dependency, build_path) != [] do
          repair_dependency(dependency.name, dependency.path, build_path, env)
        else
          ""
        end

      missing = missing_dependency_beams(dependency, build_path)

      if missing != [] do
        diagnostic = bounded_diagnostic(force_output <> repair_output)

        IO.puts(
          :stderr,
          "dependency #{dependency.name} is missing compiled modules #{Enum.join(missing, ", ")} after forced compilation:\n#{diagnostic}"
        )

        raise "dependency #{dependency.name} has source modules without BEAM output after forced compilation"
      end
    end)
  end

  defp source_bearing_dependency?(dependency) do
    Enum.any?(["lib/**/*.ex", "src/**/*.erl", "src/**/*.xrl", "src/**/*.yrl"], fn pattern ->
      dependency.path
      |> Path.join(pattern)
      |> Path.wildcard()
      |> Enum.any?()
    end)
  end

  defp missing_dependency_beams(dependency, build_path) do
    dependency.path
    |> Path.join("lib/**/*.ex")
    |> Path.wildcard()
    |> Enum.flat_map(&declared_modules/1)
    |> Enum.uniq()
    |> Enum.reject(fn module ->
      build_path
      |> Path.join("lib/#{dependency.name}/ebin/#{Atom.to_string(module)}.beam")
      |> File.regular?()
    end)
    |> Enum.map(&Atom.to_string/1)
    |> Enum.sort()
  end

  defp declared_modules(file) do
    with {:ok, source} <- File.read(file),
         {:ok, ast} <- Code.string_to_quoted(source) do
      collect_declared_modules(ast, [])
    else
      _ -> []
    end
  end

  defp collect_declared_modules({:__block__, _, forms}, stack) do
    Enum.flat_map(forms, &collect_declared_modules(&1, stack))
  end

  defp collect_declared_modules({:defmodule, _, [name, body]}, stack) do
    with {:ok, module} <- declared_module_name(name, stack),
         {:ok, nested} <- Keyword.fetch(body, :do) do
      [module | collect_declared_modules(nested, [module | stack])]
    else
      _ -> []
    end
  end

  defp collect_declared_modules(_node, _stack), do: []

  defp declared_module_name({:__aliases__, _, parts}, stack),
    do: {:ok, qualify_module(parts, stack)}

  defp declared_module_name(module, stack) when is_atom(module),
    do: {:ok, qualify_module([module], stack)}

  defp declared_module_name(_module, _stack), do: :error

  defp qualify_module([:"Elixir" | _] = parts, _stack), do: Module.concat(parts)

  defp qualify_module(parts, [parent | _]), do: Module.concat([parent | parts])

  defp qualify_module(parts, _stack), do: Module.concat(parts)

  defp force_compile_dependency(root, dependency, build_path, env) do
    case System.cmd(
           "mix",
           ["deps.compile", dependency.name, "--force"],
           cd: root,
           env: env,
           stderr_to_stdout: true
         ) do
      {output, 0} ->
        output

      {output, status} ->
        expected_priv = Path.join([build_path, "lib", dependency.name, "priv"])

        if File.dir?(Path.join(dependency.path, "priv")) and
             String.contains?(output, expected_priv) do
          output <> repair_dependency(dependency.name, dependency.path, build_path, env)
        else
          diagnostic = bounded_diagnostic(output)

          IO.puts(
            :stderr,
            "mix deps.compile #{dependency.name} --force failed (exit #{status}):\n#{diagnostic}"
          )

          raise "forced dependency compilation failed with exit status #{status}"
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

    run_complete_dependency_compiler(dependency, env)
  end

  defp run_complete_dependency_compiler(dependency, env) do
    args = ["do", "deps.loadpaths", "--no-deps-check", "+", "compile", "--force"]

    case System.cmd("mix", args, cd: dependency, env: env, stderr_to_stdout: true) do
      {output, 0} ->
        output

      {output, status} ->
        diagnostic = bounded_diagnostic(output)

        IO.puts(
          :stderr,
          "mix do deps.loadpaths + compile --force failed (exit #{status}):\n#{diagnostic}"
        )

        raise "dependency project compilation failed with exit status #{status}"
    end
  end

  defp run_dependency_compiler(task, dependency, env) do
    case System.cmd("mix", [task], cd: dependency, env: env, stderr_to_stdout: true) do
      {output, 0} ->
        output

      {output, status} ->
        diagnostic = bounded_diagnostic(output)
        IO.puts(:stderr, "mix #{task} failed (exit #{status}):\n#{diagnostic}")
        raise "dependency resource repair failed with exit status #{status}"
    end
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
