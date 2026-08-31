defmodule LspTraceQualification.Calls do
  def leaf(value), do: value + 1
  def left(value), do: leaf(value)
  def right(value), do: leaf(value)
  def recursive(value) when value <= 0, do: leaf(value)
  def recursive(value), do: recursive(value - 1)

  # Static-only witness: the normal entry path never takes this branch.
  def static_but_not_executed(value) do
    if System.get_env("LSP_TRACE_NEVER_SET") == "execute", do: leaf(value), else: value
  end

  def entry(value), do: left(value) + right(value) + recursive(value)
end
