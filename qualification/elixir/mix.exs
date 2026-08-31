defmodule LspTraceQualification.MixProject do
  use Mix.Project

  def project do
    [app: :lsp_trace_qualification, version: "1.0.0", elixir: "~> 1.15", deps: []]
  end

  def application, do: [extra_applications: [:logger]]
end
