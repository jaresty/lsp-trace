defmodule ElixirCallHierarchyTest do
  use ExUnit.Case, async: false

  @fixture_file "compiler_fixture.ex"
  @fixture """
  defmodule ECHFixture.Support do
    def remote_function(value), do: value
    defmacro remote_macro(value), do: value
    def imported_function(value), do: value
    defmacro imported_macro(value), do: value
  end

  defmodule ECHFixture.Caller do
    require ECHFixture.Support
    import ECHFixture.Support, only: [imported_function: 1, imported_macro: 1]

    @module_body ECHFixture.Support.remote_function(:module_body)
    def module_body, do: @module_body

    def run(value) do
      ECHFixture.Support.remote_function(value)
      ECHFixture.Support.remote_macro(value)
      imported_function(value)
      imported_macro(value)
    end
  end
  """

  test "remote_function events retain exact event kind" do
    assert call!(:remote_function).kind == :remote_function
  end

  test "remote_macro events retain exact event kind" do
    assert call!(:remote_macro).kind == :remote_macro
  end

  test "imported_function events retain exact event kind" do
    assert call!(:imported_function).kind == :imported_function
  end

  test "imported_macro events retain exact event kind" do
    assert call!(:imported_macro).kind == :imported_macro
  end

  test "caller and target identities come directly from compiler inputs" do
    call = call!(:remote_function)

    assert {call.caller_module, call.caller_name, call.caller_arity} ==
             {ECHFixture.Caller, :run, 1}

    assert {call.target_module, call.target_name, call.target_arity} ==
             {ECHFixture.Support, :remote_function, 1}
  end

  test "source metadata and toolchain identity are retained" do
    call = call!(:remote_function)

    assert call.file == @fixture_file
    assert is_integer(call.line) and call.line > 0
    assert is_integer(call.column) and call.column > 0

    assert call.toolchain == %{
             elixir: System.version(),
             otp: to_string(:erlang.system_info(:otp_release)),
             mix: Application.spec(:mix, :vsn) |> to_string()
           }
  end

  test "compiler events outside a function are not fabricated as source calls" do
    refute Enum.any?(capture(), &(&1.caller_name == nil))
  end

  test "supported fixture capture is deterministic" do
    assert capture() == capture()
  end

  test "supported calls across modules and imports are captured" do
    assert MapSet.new(capture(), & &1.kind) ==
             MapSet.new([:remote_function, :remote_macro, :imported_function, :imported_macro])
  end

  test "compiler call events expose resolved targets, not unresolved call events" do
    refute Enum.any?(capture(), &(&1.kind == :unresolved))
  end

  defp call!(name) do
    Enum.find(capture(), &(&1.target_name == name)) || flunk("missing #{name} fixture event")
  end

  defp capture do
    Enum.each([ECHFixture.Caller, ECHFixture.Support], fn module ->
      :code.purge(module)
      :code.delete(module)
    end)

    ElixirCallHierarchy.compile_string(@fixture, @fixture_file)
  end
end
