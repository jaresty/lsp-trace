defmodule ElixirCallHierarchy.Profile do
  @moduledoc false

  def measure(false, _phase, fun), do: fun.()

  def measure(true, phase, fun) do
    emit(true, phase, %{event: "start"})
    started = System.monotonic_time()

    try do
      fun.()
    after
      elapsed = System.monotonic_time() - started
      duration_ms = System.convert_time_unit(elapsed, :native, :microsecond) / 1_000
      emit(true, phase, %{event: "complete", duration_ms: duration_ms})
    end
  end

  def emit(false, _phase, _fields), do: :ok

  def emit(true, phase, fields) do
    payload = fields |> Map.put(:phase, phase) |> Jason.encode!()
    IO.puts(:stderr, "ECH_PROFILE " <> payload)
  end
end
