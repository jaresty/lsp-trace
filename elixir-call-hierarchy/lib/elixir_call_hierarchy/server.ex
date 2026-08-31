defmodule ElixirCallHierarchy.Server do
  @moduledoc false

  def run(state \\ %{index: nil, documents: %{}}) do
    case read_message() do
      :eof ->
        :ok

      {:ok, message} ->
        {state, response, stop?} = handle(message, state)
        if response, do: write_message(response)
        if stop?, do: :ok, else: run(state)
    end
  end

  defp handle(%{"id" => id, "method" => "initialize", "params" => params}, state) do
    root =
      uri_path(params["rootUri"] || get_in(params, ["workspaceFolders", Access.at(0), "uri"]))

    {:module, Jason.Encode} = Code.ensure_loaded(Jason.Encode)
    options = Map.get(state, :options, %{cache_dir: nil, reindex: false})
    cache_options = [reindex: options.reindex]

    cache_options =
      if options.cache_dir,
        do: Keyword.put(cache_options, :cache_dir, options.cache_dir),
        else: cache_options

    {_status, index} = ElixirCallHierarchy.Cache.load(root, cache_options)

    {Map.put(state, :index, index),
     response(id, %{
       "capabilities" => %{"callHierarchyProvider" => true, "textDocumentSync" => 1},
       "serverInfo" => %{"name" => "elixir-call-hierarchy", "version" => "0.1.0"}
     }), false}
  end

  defp handle(%{"method" => "initialized"}, state), do: {state, nil, false}

  defp handle(%{"method" => "textDocument/didOpen", "params" => params}, state) do
    document = params["textDocument"]
    {put_in(state, [:documents, document["uri"]], document["text"]), nil, false}
  end

  defp handle(
         %{"id" => id, "method" => "textDocument/prepareCallHierarchy", "params" => params},
         state
       ) do
    file = uri_path(get_in(params, ["textDocument", "uri"]))
    line = get_in(params, ["position", "line"])
    item = state.index |> ElixirCallHierarchy.Index.prepare(file, line) |> item()
    {state, response(id, if(item, do: [item], else: nil)), false}
  end

  defp handle(%{"id" => id, "method" => "callHierarchy/incomingCalls", "params" => params}, state) do
    data = get_in(params, ["item", "data"])
    target = %{identity: [data["module"], data["name"], data["arity"]]}

    incoming =
      state.index
      |> ElixirCallHierarchy.Index.incoming(target)
      |> Enum.map(fn call ->
        %{"from" => item(call.definition), "fromRanges" => Enum.map(call.ranges, &json_range/1)}
      end)

    {state, response(id, incoming), false}
  end

  defp handle(%{"id" => id, "method" => "shutdown"}, state), do: {state, response(id, nil), false}
  defp handle(%{"method" => "exit"}, state), do: {state, nil, true}

  defp handle(%{"id" => id}, state),
    do:
      {state,
       %{
         "jsonrpc" => "2.0",
         "id" => id,
         "error" => %{"code" => -32601, "message" => "Method not found"}
       }, false}

  defp handle(_notification, state), do: {state, nil, false}

  defp item(nil), do: nil

  defp item(definition) do
    [module, name, arity] = identity_strings(definition.identity)
    range = json_range(definition.range)

    %{
      "name" => "#{name}/#{arity}",
      "detail" => inspect(module),
      "kind" => kind(definition.kind),
      "uri" => path_uri(definition.file),
      "range" => range,
      "selectionRange" => range,
      "data" => %{
        "module" => module,
        "name" => name,
        "arity" => arity
      }
    }
  end

  defp read_message do
    case IO.binread(:stdio, :line) do
      :eof ->
        :eof

      line ->
        [_, length] = Regex.run(~r/^Content-Length: (\d+)\r?\n$/, line)
        read_headers()
        {:ok, IO.binread(:stdio, String.to_integer(length)) |> Jason.decode!()}
    end
  end

  defp read_headers do
    case IO.binread(:stdio, :line) do
      line when line in ["\r\n", "\n"] -> :ok
      _ -> read_headers()
    end
  end

  defp write_message(message) do
    body = Jason.encode!(message)
    IO.binwrite(:stdio, "Content-Length: #{byte_size(body)}\r\n\r\n#{body}")
  end

  defp response(id, result), do: %{"jsonrpc" => "2.0", "id" => id, "result" => result}
  defp json_range(range), do: %{"start" => stringify(range.start), "end" => stringify(range.end)}
  defp stringify(position), do: %{"line" => position.line, "character" => position.character}
  defp kind(kind) when kind in [:defmacro, :defmacrop, "defmacro", "defmacrop"], do: 12
  defp kind(_), do: 12

  defp identity_strings({module, name, arity}),
    do: [Atom.to_string(module), Atom.to_string(name), arity]

  defp identity_strings([module, name, arity]), do: [module, name, arity]
  defp uri_path(nil), do: nil
  defp uri_path("file://" <> path), do: URI.decode(path)
  defp path_uri(path), do: "file://" <> URI.encode(path)
end
