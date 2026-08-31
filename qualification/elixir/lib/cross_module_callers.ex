defmodule LspTraceQualification.CrossModuleCallers do
  alias LspTraceQualification.Calls, as: Calls

  def aliased_cross_file(value), do: Calls.leaf(value)

  def direct_qualified(value), do: LspTraceQualification.Calls.leaf(value)
end
