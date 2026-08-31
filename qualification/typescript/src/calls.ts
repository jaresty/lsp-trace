export function leaf(value: number): number {
  return value + 1;
}

export function left(value: number): number {
  return leaf(value);
}

export function right(value: number): number {
  return leaf(value);
}

export function recursive(value: number): number {
  return value <= 0 ? leaf(value) : recursive(value - 1);
}

// Static-only witness: the language server may report this edge even though the
// fixture's runtime entry point never takes this branch.
export function staticButNotExecuted(value: number): number {
  if (process.env.LSP_TRACE_NEVER_SET === "execute") {
    return leaf(value);
  }
  return value;
}

export function entry(value: number): number {
  return left(value) + right(value) + recursive(value);
}

console.log(entry(2));
