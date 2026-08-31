defmodule ElixirCallHierarchy.Call do
  @moduledoc "A compiler-observed source call."

  @enforce_keys [
    :kind,
    :caller_module,
    :caller_name,
    :caller_arity,
    :target_module,
    :target_name,
    :target_arity,
    :file,
    :line,
    :column,
    :toolchain
  ]
  defstruct @enforce_keys

  @type t :: %__MODULE__{
          kind: :remote_function | :remote_macro | :imported_function | :imported_macro,
          caller_module: module(),
          caller_name: atom(),
          caller_arity: non_neg_integer(),
          target_module: module(),
          target_name: atom(),
          target_arity: non_neg_integer(),
          file: String.t(),
          line: pos_integer() | nil,
          column: pos_integer() | nil,
          toolchain: %{elixir: String.t(), otp: String.t(), mix: String.t()}
        }
end
