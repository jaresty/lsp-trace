defmodule ElixirCallHierarchy.StdioServerTest do
  use ExUnit.Case, async: false

  @wrong Path.expand("fixtures/wrong_stdio_server.fixture", __DIR__)

  setup do
    workspace = Path.join(System.tmp_dir!(), "ech-#{System.unique_integer([:positive])}")
    File.mkdir_p!(Path.join(workspace, "lib"))

    File.write!(Path.join(workspace, "mix.exs"), """
    defmodule Fixture.MixProject do
      use Mix.Project
      def project, do: [app: :fixture, version: "0.1.0", elixir: "~> 1.16", deps: []]
    end
    """)

    File.write!(Path.join(workspace, "lib/calls.ex"), """
    defmodule Fixture.Calls do
      def leaf(v) when is_integer(v), do: v
      def leaf(v), do: v
      def same_module(v), do: leaf(v)
      def recursive(0), do: leaf(0)
      def recursive(v), do: leaf(v) + recursive(v - 1)
      def static_only(v), do: if(System.get_env("NEVER") == "yes", do: leaf(v), else: v)
    end
    """)

    File.write!(Path.join(workspace, "lib/cross.ex"), """
    defmodule Fixture.Cross do
      alias Fixture.Calls, as: Calls
      def aliased(v), do: Calls.leaf(v)
      def qualified(v), do: Fixture.Calls.leaf(v)
    end
    """)

    on_exit(fn -> File.rm_rf!(workspace) end)
    %{workspace: workspace}
  end

  test "initialize advertises call hierarchy", %{workspace: workspace} do
    with_server(@wrong, workspace, "initialize", fn port ->
      result = request(port, 1, "initialize", %{"rootUri" => uri(workspace)})
      assert result["capabilities"]["callHierarchyProvider"] == true
    end)
  end

  test "prepare resolves and coalesces a multi-clause target", %{workspace: workspace} do
    with_server(@wrong, workspace, "prepare", fn port ->
      initialize(port, workspace)

      result =
        request(
          port,
          2,
          "textDocument/prepareCallHierarchy",
          position(workspace, "lib/calls.ex", 1, 7)
        )

      assert [
               %{
                 "name" => "leaf/1",
                 "data" => %{
                   "module" => "Elixir.Fixture.Calls",
                   "name" => "leaf",
                   "arity" => 1
                 },
                 "range" => %{"end" => %{"line" => end_line}}
               }
             ] = result

      assert end_line >= 2
    end)
  end

  test "incoming calls include every static call shape", %{workspace: workspace} do
    with_server(@wrong, workspace, "incoming_names", fn port ->
      initialize(port, workspace)

      [target] =
        request(
          port,
          2,
          "textDocument/prepareCallHierarchy",
          position(workspace, "lib/calls.ex", 1, 7)
        )

      incoming = request(port, 3, "callHierarchy/incomingCalls", %{"item" => target})
      names = MapSet.new(incoming, & &1["from"]["name"])

      assert MapSet.subset?(
               MapSet.new([
                 "same_module/1",
                 "recursive/1",
                 "static_only/1",
                 "aliased/1",
                 "qualified/1"
               ]),
               names
             )
    end)
  end

  test "incoming calls include exact non-empty ranges", %{workspace: workspace} do
    with_server(@wrong, workspace, "incoming_ranges", fn port ->
      initialize(port, workspace)

      [target] =
        request(
          port,
          2,
          "textDocument/prepareCallHierarchy",
          position(workspace, "lib/calls.ex", 1, 7)
        )

      incoming = request(port, 3, "callHierarchy/incomingCalls", %{"item" => target})

      assert Enum.all?(incoming, fn call ->
               call["fromRanges"] != [] and Enum.all?(call["fromRanges"], &non_empty?/1)
             end)
    end)
  end

  defp initialize(port, workspace) do
    result = request(port, 1, "initialize", %{"rootUri" => uri(workspace)})
    assert result["capabilities"]["callHierarchyProvider"] == true
    notify(port, "initialized", %{})
  end

  defp with_server(script, _workspace, wrong_behavior, fun) do
    project = Path.expand("..", __DIR__)
    wrong? = System.get_env("ECH_WRONG_SERVER") == "1"

    args =
      if wrong?,
        do: ["run", script],
        else: ["run", "-e", "ElixirCallHierarchy.CLI.main([\"--stdio\"])"]

    env = if wrong?, do: [{~c"WRONG_BEHAVIOR", String.to_charlist(wrong_behavior)}], else: []

    port =
      Port.open({:spawn_executable, System.find_executable("mix")}, [
        :binary,
        :exit_status,
        args: args,
        cd: project,
        env: env
      ])

    try do
      fun.(port)
    after
      if Port.info(port), do: Port.close(port)
    end
  end

  defp request(port, id, method, params) do
    send_message(port, %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params})
    %{"id" => ^id, "result" => result} = receive_message(port)
    result
  end

  defp notify(port, method, params),
    do: send_message(port, %{"jsonrpc" => "2.0", "method" => method, "params" => params})

  defp send_message(port, message) do
    body = Jason.encode!(message)
    Port.command(port, "Content-Length: #{byte_size(body)}\r\n\r\n#{body}")
  end

  defp receive_message(port, buffer \\ "") do
    receive do
      {^port, {:data, data}} -> decode_message(port, buffer <> data)
      {^port, {:exit_status, status}} -> flunk("server exited #{status}")
    after
      10_000 -> flunk("server response timeout")
    end
  end

  defp decode_message(port, buffer) do
    case :binary.match(buffer, "\r\n\r\n") do
      {header_end, 4} ->
        <<header::binary-size(^header_end), _::binary-size(4), rest::binary>> = buffer
        [_, length] = Regex.run(~r/Content-Length: (\d+)/, header)
        size = String.to_integer(length)

        if byte_size(rest) >= size do
          <<body::binary-size(^size), _tail::binary>> = rest
          Jason.decode!(body)
        else
          receive_message(port, buffer)
        end

      :nomatch ->
        receive_message(port, buffer)
    end
  end

  defp position(workspace, relative, line, character),
    do: %{
      "textDocument" => %{"uri" => uri(Path.join(workspace, relative))},
      "position" => %{"line" => line, "character" => character}
    }

  defp uri(path), do: "file://" <> path

  defp non_empty?(%{"start" => start, "end" => finish}),
    do: {start["line"], start["character"]} < {finish["line"], finish["character"]}
end
