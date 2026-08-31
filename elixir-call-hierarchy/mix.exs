defmodule ElixirCallHierarchy.MixProject do
  use Mix.Project

  def project do
    [
      app: :elixir_call_hierarchy,
      version: "0.1.0",
      elixir: "~> 1.16",
      start_permanent: Mix.env() == :prod,
      escript: [main_module: ElixirCallHierarchy.CLI, name: "elixir-call-hierarchy"],
      test_ignore_filters: ["fixtures/*"],
      deps: [{:jason, "~> 1.4"}]
    ]
  end

  def application, do: [extra_applications: [:logger, :mix]]
end
