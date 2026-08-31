defmodule ElixirCallHierarchy.Tracer do
  @moduledoc false

  @call_kinds [:remote_function, :remote_macro, :imported_function, :imported_macro]

  def trace({kind, metadata, target_module, target_name, target_arity}, env)
      when kind in @call_kinds do
    with {caller_name, caller_arity} <- env.function,
         pid when is_pid(pid) <- Process.whereis(ElixirCallHierarchy.Collector) do
      send(pid, {
        :compiler_trace,
        %ElixirCallHierarchy.Call{
          kind: kind,
          caller_module: env.module,
          caller_name: caller_name,
          caller_arity: caller_arity,
          target_module: target_module,
          target_name: target_name,
          target_arity: target_arity,
          file: to_string(env.file),
          line: metadata[:line],
          column: metadata[:column],
          toolchain: toolchain()
        }
      })
    else
      _ -> :ok
    end

    :ok
  end

  def trace(_event, _env), do: :ok

  defp toolchain do
    %{
      elixir: System.version(),
      otp: to_string(:erlang.system_info(:otp_release)),
      mix: Application.spec(:mix, :vsn) |> to_string()
    }
  end
end
