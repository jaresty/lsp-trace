# Disposable compile-harness evidence

Harness: `/tmp/adr0011-raw-custody-interface-harness/`; outside Git, disposable, never implementation or copy source.

It mirrors existing Manager, generic owner/lease, C15 lease, and Reader only enough to compile the frozen additions. Call sites cover pointer-lease transfer, Manager-owned view/handoff, terminal/refusal, explicit generic lease Release, owner requestTerminal (the STOP/RESTART/Shutdown/cancel convergence point), direct Manager terminal release, and duplicate release.

## Exact command

```sh
cd /tmp/adr0011-raw-custody-interface-harness && env -i PATH="$PATH" HOME="$HOME" GOTOOLCHAIN=local gofmt -w internal/lspwire/read_frame.go sessionruntime/call_hierarchy_directional_raw_v1_private.go && env -i PATH="$PATH" HOME="$HOME" GOTOOLCHAIN=local go test ./... && env -i PATH="$PATH" HOME="$HOME" GOTOOLCHAIN=local go vet ./... && shasum -a 256 go.mod internal/lspwire/read_frame.go sessionruntime/call_hierarchy_directional_raw_v1_private.go && go version && go env GOOS GOARCH GOROOT GOPATH GOMOD
```

## Exact output

```text
?   	example.invalid/adr0011-raw-custody-interface-harness/internal/lspwire	[no test files]
?   	example.invalid/adr0011-raw-custody-interface-harness/sessionruntime	[no test files]
5d479d10adc32b98ef5c7369cd17f9ffd8dc946a26c0bf2e8024f0d8b526fe1d  go.mod
86324c319ebb1c58d4d6477607056ea6e18e49c6ea3f351f3cbe0971777dfa30  internal/lspwire/read_frame.go
900651bd070198be788c32604d1d5a3f05373b9471da62893c9fa7ac504b1be8  sessionruntime/call_hierarchy_directional_raw_v1_private.go
go version go1.26.5 darwin/arm64
darwin
arm64
/opt/homebrew/Cellar/go/1.26.5/libexec
/Users/schwa/go
/tmp/adr0011-raw-custody-interface-harness/go.mod
```

`go test` and `go vet` exited zero. This proves signatures and exact call-site typing only—not runtime lock behavior, races, C15 acceptance, or implementation.
