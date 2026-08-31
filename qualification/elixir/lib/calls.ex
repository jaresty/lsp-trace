defmodule LspTraceQualification.Calls do
  def leaf(value), do: value + 1
  def left(value), do: leaf(value)
  def right(value), do: leaf(value)
  def recursive(value), do: leaf(value) + recursive_loop(value)
  defp recursive_loop(value) when value <= 0, do: 0
  defp recursive_loop(value), do: recursive_loop(value - 1)

  # Static-only witness: the normal entry path never takes this branch.
  def static_but_not_executed(value) do
    if System.get_env("LSP_TRACE_NEVER_SET") == "execute", do: leaf(value), else: value
  end

  def entry(value), do: left(value) + right(value) + recursive(value)
end
