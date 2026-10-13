# Disposable compile harness

Run from this package directory:

```sh
python3 verify_package.py --harness
```

The verifier first establishes exact committed producer manifests and blobs, exact declaration equality, independently derived bridge semantics, current source pins, 47+1 accounting, proposal-only vocabulary, and package checksums. It then creates a fresh temporary module, copies only this package's exact declaration snapshots, runs `gofmt -d` (which must emit no diff), `go test ./...`, `go test -run '^$' ./...`, and `go vet ./...`, prints declaration hashes and `go version`, and deletes the module.

This proves only that the exact declarations and canonical call sites compile together reproducibly. It does not execute repository tests or prove behavior, accounting, race safety, lock correctness, source preservation, provider support, semantic classification, or implementation readiness.

Required successful verdict: `INTERFACE_COHERENCE_ONLY`.

AUTHORED_NOT_GO
