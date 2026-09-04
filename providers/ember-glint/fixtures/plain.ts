export function leaf(value: number): number {
  return value + 1;
}

export function caller(value: number): number {
  return leaf(value);
}
