# Reproduction receipt

Two producer regenerations completed with exit 0 and were byte-identical for all five generator outputs.

## Producer boundary

The generator was built and run outside the repository. The source repository was a clean local clone checked out at the pinned revision. The existing untracked architecture package in the authoring worktree was never read or modified by the generator.

## Exact commands

```sh
SRC=/private/tmp/adr0011-incoming-calls-v1-6yiY4m
CLEAN=/private/tmp/adr0011-target-corpus-clean
RUN=/private/tmp/adr0011-target-corpus-runs
BIN=/private/tmp/adr0011-target-corpus-generator

git clone --quiet --no-local "$SRC" "$CLEAN"
git -C "$CLEAN" checkout --quiet 4b910fc0c517cb7801fb51d17d295897230d11fa

env -i PATH="$PATH" HOME="$HOME" GOCACHE="${GOCACHE:-$HOME/Library/Caches/go-build}" \
  go build -trimpath -o "$BIN" generator.go
cp generator.go "$BIN.source"
mkdir -p "$RUN/run1" "$RUN/run2"
env -i PATH="$PATH" HOME="$HOME" "$BIN" "$CLEAN" "$RUN/run1"
env -i PATH="$PATH" HOME="$HOME" "$BIN" "$CLEAN" "$RUN/run2"
diff -ru "$RUN/run1" "$RUN/run2"
```

The retained generator requires its exact source adjacent to the executable at `<executable>.source`; this makes the generator hash part of each output metadata record. A producer must use a clean workspace; `git status --porcelain=v1 --untracked-files=all` must be empty.

## Byte-identical outputs

| file | SHA-256 |
|---|---|
| generator.go | `737ab249398664e04edb347a5dbc7bab1bfd4a40834bef76525078424e0d19f7` |
| metadata.json | `1cfd526bb259d6fc9550cd3e56c789458f7f37b1806c34347e30b1255754d8bd` |
| source-manifest.json | `ab8122e23480ab67b8bad9166f4b1c4fc0ab1f3e9758afd2aee72784fcfef178` |
| targets.json | `301fdff765cb31a47900c9fc3bf6af8a02d27c91c7e027bfb5e3f3ebabadc16e` |
| targets.tsv | `8aa361546ab5245ae0ac2f945e3d19b36a48b745bfe7187e5baef6d6d9a14bd4` |

The recorded paths are reproduction evidence, not portable defaults. Substitute fresh external temporary paths while preserving commands and environment isolation.
