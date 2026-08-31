defmodule ElixirCallHierarchy.CLI do
  @moduledoc false

  def parse(args) do
    case OptionParser.parse(args,
           strict: [stdio: :boolean, cache_dir: :string, reindex: :boolean]
         ) do
      {options, [], []} ->
        parsed = %{
          stdio: Keyword.get(options, :stdio, false),
          cache_dir: Keyword.get(options, :cache_dir),
          reindex: Keyword.get(options, :reindex, false)
        }

        if parsed.stdio, do: {:ok, parsed}, else: {:error, "--stdio is required"}

      {_options, remaining, invalid} ->
        detail = inspect(invalid ++ remaining)
        {:error, "unknown option or argument: #{detail}"}
    end
  end

  def main(args) do
    case parse(args) do
      {:ok, options} ->
        ElixirCallHierarchy.Server.run(%{index: nil, documents: %{}, options: options})

      {:error, message} ->
        IO.puts(
          :stderr,
          "#{message}\nusage: elixir-call-hierarchy --stdio [--cache-dir PATH] [--reindex]"
        )

        System.halt(2)
    end
  end
end
