# Receipt verification commands and captured status

Status: `AUTHORED_NOT_GO`

Run from repository root.

## Captured pre-authorship status

```text
$ git status --short --branch
## campaign/calls-incoming-exact-v1-adr0011-incoming-calls-v1-6yiY4m

$ git rev-parse HEAD
da55dae61369c3ead52e74b708feaa1c02d94c13
```

The empty short-status body proves the requested worktree was clean before this receipt package was authored.

## Dependency and package-byte verification

```sh
test "$(git rev-parse HEAD)" = da55dae61369c3ead52e74b708feaa1c02d94c13
git merge-base --is-ancestor 0e2583b2a535701da152e756253b4b96df55272b da55dae61369c3ead52e74b708feaa1c02d94c13
git merge-base --is-ancestor 2faac2671987f6e6f7c680fe4f072064cfe48136 da55dae61369c3ead52e74b708feaa1c02d94c13
git merge-base --is-ancestor f03672416bbe1342c06e7cbb33bf51127909ce8a da55dae61369c3ead52e74b708feaa1c02d94c13

sha256sum -c docs/qualification/adr0011-directional-raw-custody-32mib-test-writeset-go-v1/SHA256SUMS
sha256sum -c docs/qualification/adr0011-generic-directional-calls-profile-amendment-go-v1/SHA256SUMS
sha256sum -c docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v1/SHA256SUMS
sha256sum -c docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v2/SHA256SUMS

printf '%s  %s\n' \
  52df9064796d71710ebc2c81c314e1f5c4b4dbd9aff531d483a475a71e4e2d13 \
  docs/qualification/adr0011-directional-raw-custody-32mib-test-writeset-go-v1/SHA256SUMS \
  df44a8befc36ff69b7e903be2b78b5cf9fbb508538b8cf04ed5bbb7fd35b2df7 \
  docs/qualification/adr0011-generic-directional-calls-profile-amendment-go-v1/SHA256SUMS \
  d36211da45adb55e83f0b112a62140d7bec5ef2076edee12c3facebbd692af7c \
  docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v1/SHA256SUMS \
  8f329e1012278a7e92220d0499047a7b486402fcc2db8d10f7b4b2dcb06feeda \
  docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v2/SHA256SUMS | sha256sum -c -

(cd docs/qualification/adr0011-directional-raw-custody-32mib-implementation-authority-ready-v1 && sha256sum -c SHA256SUMS)
```

On macOS, replace `sha256sum` with `shasum -a 256` and use its compatible check mode.

## Scope and status verification

```sh
git diff --name-only da55dae61369c3ead52e74b708feaa1c02d94c13 --
git status --short
grep -R "AUTHORED_NOT_GO" \
  docs/qualification/adr0011-directional-raw-custody-32mib-test-writeset-go-v1 \
  docs/qualification/adr0011-generic-directional-calls-profile-amendment-go-v1 \
  docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v1 \
  docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v2
```

Before independent review, `git diff --name-only da55dae... --` and `git status --short` must list only this new receipt directory. No production file, test file, prior qualification package, generated file, or dependency may be modified.

No build, test, vet, runtime, provider, browser, deployment, or production command is part of this docs-only receipt.

AUTHORED_NOT_GO
