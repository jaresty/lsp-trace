# Typed seed-file foundation qualification

## Owned contract

`seedformat` owns the transport-neutral `lsp-trace.seeds.v2` decoder,
semantic validator, committed Draft 2020-12 schema, and deterministic translation
to `acquisitionops.Manifest` for slice acquisition. It reads no source, starts no
provider, and infers no graph node or call.

The file contract requires `coordinate_convention: one-based`, one to 64 ordered
explicitly typed seeds, unique labels matching
`^[A-Za-z][A-Za-z0-9._-]{0,63}$`, canonical workspace-contained paths, and depths
within the acquisition bound `0..64`. Position coordinates are positive and
translate exactly to zero-based `uint32` acquisition locators. A slice target is
exactly a position or symbol and cannot recursively contain a slice.

Translation maps the first seed to acquisition root ID `root` and all remaining
seeds, in order, to `required_targets`. Label-derived IDs are deterministic and
collision-safe around the reserved root ID. Slice bounds override file defaults;
file defaults override acquisition defaults. Callers must supply every existing
global acquisition limit. Callers may set `TopmostSiblings`, which translates
exactly to `expansion.topmost_siblings` for later V5 wiring.

The JSON Schema is structural. `Decode` additionally enforces recursive duplicate
member rejection and workspace-relative path semantics because those depend on
raw member identity and caller-supplied workspace context.

## Compatibility boundary

The existing untyped seed file is parsed in CLI-owned code whose exact historical
contract is not imported into this package on this change. Compatibility decoding
and CLI selection are therefore intentionally deferred to later wiring rather
than approximated here. No CLI, MCP, acquisition handler, README, or skill file is modified. The only
non-package change is the repository-required exact-path ownership registration
in `internal/integratedconformance/harness_test.go`.

## Guards

`seedformat_test.go` covers strict version/type/member handling, duplicates, type
confusion, nested slices, labels, paths, one-based coordinate bounds, depth bounds,
seed count, schema/runtime structural and complete-document parity (including
trailing JSON rejection), deterministic isolated schema bytes, generated
depth/coordinate properties, ID collision chains, bound precedence,
global-limit requirements, topmost-sibling forcing, deterministic output bytes,
and translation completeness.

## Qualification commands

Run from repository root:

```sh
go test ./internal/seedformat
go test -p 1 ./...
go vet ./...
./scripts/release-check.sh
GOOS=linux GOARCH=amd64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...
git diff --check
git status --short
```

A commit is permitted only after every command is GREEN and the status contains
only the intended package files plus their exact ownership registration before
commit, then no entries after commit.
