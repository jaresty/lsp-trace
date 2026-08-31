defmodule ElixirCallHierarchy.CLI do
  @moduledoc false

  def main(args) do
    case args do
      ["--stdio"] -> ElixirCallHierarchy.Server.run()
      _ -> IO.puts(:stderr, "usage: elixir-call-hierarchy --stdio")
    end
  end
end
