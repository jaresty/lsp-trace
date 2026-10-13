# Disposable interface-validation harness evidence

Harness: `/private/tmp/adr0011-v2-final-harness/` (outside Git; disposable; never implementation or copy source).

Scope is interface validation only. It compiles canonical private declarations, constructs nonzero canonical domain-separated digests for both generic profiles, and typechecks the complete claimed future WRITESET call shapes for both directions: exact 32 MiB/+1, nil/zero/invalid, every disposition/terminal/refusal/lifetime, attempted outcome and ordered frontier accounting, release races, and duplicate release. Stub bodies do not establish behavior, lock/race correctness, 32 MiB effects, C15 correctness, provider support, completeness, or production readiness.

## Exact command

```sh
cd /private/tmp/adr0011-v2-final-harness && env -i PATH="$PATH" HOME="$HOME" GOTOOLCHAIN=local gofmt -w sessionruntime/call_hierarchy_directional_raw_v1_private.go internal/lspwire/read_frame.go && env -i PATH="$PATH" HOME="$HOME" GOTOOLCHAIN=local go test ./... && env -i PATH="$PATH" HOME="$HOME" GOTOOLCHAIN=local go test -run '^$' ./... && env -i PATH="$PATH" HOME="$HOME" GOTOOLCHAIN=local go vet ./... && shasum -a 256 go.mod internal/lspwire/read_frame.go sessionruntime/call_hierarchy_directional_raw_v1_private.go && go version && go env GOOS GOARCH GOROOT GOPATH GOMOD
```

## Exact output

```text
?   	example.invalid/adr0011-v2-final-harness/internal/lspwire	[no test files]
?   	example.invalid/adr0011-v2-final-harness/sessionruntime	[no test files]
?   	example.invalid/adr0011-v2-final-harness/internal/lspwire	[no test files]
?   	example.invalid/adr0011-v2-final-harness/sessionruntime	[no test files]
9b903cb67949cbe35cd2010823bd5a0c0406ae2c5ecc80a3570a40222eb358b0  go.mod
bd05c4224d2662bbe88f3f259f76e1315daf82d315841180aedcfbf7f297b2bc  internal/lspwire/read_frame.go
e41154da7140a8f4c4742480b703b64807df37ed630a930d91aa1cf90b3351e9  sessionruntime/call_hierarchy_directional_raw_v1_private.go
go version go1.26.5 darwin/arm64
darwin
arm64
/opt/homebrew/Cellar/go/1.26.5/libexec
/Users/schwa/go
/private/tmp/adr0011-v2-final-harness/go.mod
```

`go test`, compile-only `go test -run '^$'`, and `go vet` exited zero. No repository production test was run or mutated.

AUTHORED_NOT_GO
