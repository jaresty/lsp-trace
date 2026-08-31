namespace LspTraceQualification;

public static class Calls
{
    public static int Leaf(int value) => value + 1;
    public static int Left(int value) => Leaf(value);
    public static int Right(int value) => Leaf(value);
    public static int Recursive(int value) => value <= 0 ? Leaf(value) : Recursive(value - 1);

    // Static-only witness: normal fixture execution never takes this branch.
    public static int StaticButNotExecuted(int value) =>
        Environment.GetEnvironmentVariable("LSP_TRACE_NEVER_SET") == "execute" ? Leaf(value) : value;

    public static int Entry(int value) => Left(value) + Right(value) + Recursive(value);
    public static void Main() => Console.WriteLine(Entry(2));
}
