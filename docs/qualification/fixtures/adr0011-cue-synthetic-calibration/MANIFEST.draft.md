# Draft synthetic calibration manifest

Provenance: `base.cue` and `service.cue` were self-authored solely for this OPEN CUE calibration exercise; they are not copied from a third-party or production project. This manifest is a draft inventory, not held-out labels, real-world/provider qualification, managed-provider attestation, or a clean-checkout assertion. The surrounding main working tree may be dirty.

SHA-256 of the formatted source bytes:

| Source | SHA-256 |
| --- | --- |
| `base.cue` | `8b143d522ed67fcececa9fcdea861d1caa9f9dc8e60c49d9283f9b6eaa8cf8df` |
| `service.cue` | `7580f604b61f3b431b7da63ba4bcbf59580eb9563d9d7c84eef3a48226fd5d66` |

Reproduce source inventory from this directory:

```sh
find . -maxdepth 1 -type f -name '*.cue' -print | LC_ALL=C sort
shasum -a 256 base.cue service.cue
```

Offline verification performed with `cue version v0.17.1`:

```sh
cue fmt base.cue
cue fmt service.cue
cue fmt -d base.cue service.cue
cue vet base.cue service.cue
```

The first `cue fmt -d` on the originally space-indented files showed tab-alignment diffs. After the two formatting commands, `cue fmt -d` emitted no diff and `cue vet` emitted no output; both exited 0. A local UTF-16 substring assertion for the six positive README positions exited 0. No provider operation was run.
