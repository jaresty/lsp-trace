# Disposable interface-validation harness evidence

Harness: `/tmp/adr0011-private-interface-v3-harness/` (outside Git; disposable; never implementation or copy source).

Scope is declaration and call-shape validation only. It compiles canonical private declarations and explicitly typechecks all 20 committed TSV symbols: six reserve/copy/alias triples plus transfer and release. Canonical calls cover both generic directions, exact 32 MiB/+1 and `uint64` overflow, broad nil/zero/invalid account/attempt/component/Manager/owner/source/destination/acquisition/callback shapes, an explicit generic reservation with `privateCallHierarchyDirectionalComponentInvalidV3`, an explicit nil transfer source, an explicit nil borrow acquisition, and an explicit error-returning callback whose returned refusal error is captured. They also cover transfer success and failure-source-preservation signatures, callback/publication refusal, complete Manager→owner→account lock-order and account-held inversion-refusal surfaces, lease-bearing C15 shapes, three concurrent transfer/release/borrow race-shaped goroutine calls with deterministic `sync.WaitGroup` completion, and separate sequential duplicate release.

The goroutine calls execute only zero-effect declaration stubs. This harness proves compilation, method sets, explicit call-site availability, and deterministic harness completion only. It does not prove behavior, race safety, source preservation at runtime, lock correctness, refusal precedence, accounting effects, no-escape, C15 correctness, provider support, completeness, Stage B classification, or production readiness.

## Exact command

```sh
rm -rf /tmp/adr0011-private-interface-v3-harness &&
mkdir -p /tmp/adr0011-private-interface-v3-harness/sessionruntime /tmp/adr0011-private-interface-v3-harness/internal/lspwire &&
cp sessionruntime_private.go.txt /tmp/adr0011-private-interface-v3-harness/sessionruntime/call_hierarchy_directional_raw_v1_private.go &&
cp lspwire_private.go.txt /tmp/adr0011-private-interface-v3-harness/internal/lspwire/read_frame.go &&
printf 'module example.invalid/adr0011-private-interface-v3-harness\n\ngo 1.26\n' > /tmp/adr0011-private-interface-v3-harness/go.mod &&
cd /tmp/adr0011-private-interface-v3-harness &&
gofmt -w sessionruntime/call_hierarchy_directional_raw_v1_private.go internal/lspwire/read_frame.go &&
go test ./... &&
go test -run '^$' ./... &&
go vet ./... &&
shasum -a 256 go.mod internal/lspwire/read_frame.go sessionruntime/call_hierarchy_directional_raw_v1_private.go &&
go version
```

## Exact result

```text
?    example.invalid/adr0011-private-interface-v3-harness/internal/lspwire    [no test files]
?    example.invalid/adr0011-private-interface-v3-harness/sessionruntime    [no test files]
?    example.invalid/adr0011-private-interface-v3-harness/internal/lspwire    [no test files]
?    example.invalid/adr0011-private-interface-v3-harness/sessionruntime    [no test files]
afa0e0dda3b02919cfada9ef888eacced4874d176c7f0562e995253131a44f62  go.mod
7b1c92b7910953e5041f531d9cf30fde85c989d00b6d9c50b7449920931a50bf  internal/lspwire/read_frame.go
a7990de58b909316a61f9da715190c89a9a6345875904adb64007935f96a98f0  sessionruntime/call_hierarchy_directional_raw_v1_private.go
go version go1.26.5 darwin/arm64
```

All commands exited zero. No repository production/test source was run or mutated.

AUTHORED_NOT_GO
