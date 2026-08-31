defmodule ElixirCallHierarchy do
  @moduledoc "Generic compiler-tracer capture for Elixir call hierarchy tooling."

  @spec compile_string(String.t(), String.t()) :: [ElixirCallHierarchy.Call.t()]
  def compile_string(source, file \\ "fixture.ex") do
    Process.register(self(), ElixirCallHierarchy.Collector)
    previous = Code.compiler_options()
    Code.compiler_options(tracers: [ElixirCallHierarchy.Tracer])

    try do
      Code.compile_string(source, file)
      drain([])
    after
      Code.compiler_options(previous)
      Process.unregister(ElixirCallHierarchy.Collector)
    end
  end

  defp drain(calls) do
    receive do
      {:compiler_trace, call} -> drain([call | calls])
    after
      0 -> Enum.reverse(calls)
    end
  end
end
