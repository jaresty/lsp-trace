# Receipt verification commands and captured status

Status: `AUTHORED_NOT_GO`

Run from repository root. These are review commands, not commands executed as implementation, test, build, runtime, or production activity.

## Captured pre-authorship state

```text
$ git status --short --branch
## campaign/calls-incoming-exact-v1-adr0011-incoming-calls-v1-6yiY4m

$ git rev-parse HEAD
be771d5627f9a2588781ac67707d7eb3c7b8d314
```

The empty short-status body establishes a clean worktree before this new receipt directory was authored.

## Commit and ancestry verification

```sh
test "$(git rev-parse HEAD)" = be771d5627f9a2588781ac67707d7eb3c7b8d314
for c in \
  0e2583b2a535701da152e756253b4b96df55272b \
  2faac2671987f6e6f7c680fe4f072064cfe48136 \
  f03672416bbe1342c06e7cbb33bf51127909ce8a \
  a1cd77950ffa45ebed52310878f772f3c7344756 \
  acf563e7708095d8a32c809b49d97c7432d49b03; do
  git merge-base --is-ancestor "$c" be771d5627f9a2588781ac67707d7eb3c7b8d314
done
```

## Exact committed dependency-byte verification

```sh
sha256sum -c docs/qualification/adr0011-incoming-outgoing-raw-custody-32mib-design-v1/SHA256SUMS
sha256sum -c docs/qualification/adr0011-directional-raw-custody-32mib-test-writeset-go-v1/SHA256SUMS
sha256sum -c docs/qualification/adr0011-generic-directional-calls-profile-amendment-go-v1/SHA256SUMS
sha256sum -c docs/qualification/adr0011-directional-raw-custody-c15-lease-source-v2-go-v1/SHA256SUMS
sha256sum -c docs/qualification/adr0011-directional-raw-custody-c15-two-stage-contract-v2/RUN_MANIFEST.sha256
sha256sum -c docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v3/SHA256SUMS

printf '%s  %s\n' \
  8ada636667939df18e10f8820ba10792e33d2c4f159ec32fcee2b99b458f71fd docs/qualification/adr0011-incoming-outgoing-raw-custody-32mib-design-v1/SHA256SUMS \
  52df9064796d71710ebc2c81c314e1f5c4b4dbd9aff531d483a475a71e4e2d13 docs/qualification/adr0011-directional-raw-custody-32mib-test-writeset-go-v1/SHA256SUMS \
  df44a8befc36ff69b7e903be2b78b5cf9fbb508538b8cf04ed5bbb7fd35b2df7 docs/qualification/adr0011-generic-directional-calls-profile-amendment-go-v1/SHA256SUMS \
  6d764eb05029c5663c740ac0bfa452552eeb7f07e5a74f68b45e144ffe15d4a6 docs/qualification/adr0011-directional-raw-custody-c15-lease-source-v2-go-v1/SHA256SUMS \
  12fb6b1f1fecb7eda022b0ffe84750cca5bcefbb75e87fcaa48d24016228e8e8 docs/qualification/adr0011-directional-raw-custody-c15-two-stage-contract-v2/RUN_MANIFEST.sha256 \
  0c9909507b6abd5fd16034c70f7b34f1ae04719822af669ba8c3f373e4a49a1d docs/qualification/adr0011-directional-raw-custody-c15-two-stage-contract-v2/STAGE_A_PLANNED_OBLIGATIONS.tsv \
  02153887105c30cc6eaa3b976ec49e7f32b97afcf35ff6b8aaf185529e029704 docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v3/SHA256SUMS | sha256sum -c -

(cd docs/qualification/adr0011-directional-raw-custody-32mib-implementation-authority-ready-v2 && sha256sum -c SHA256SUMS)
```

On macOS, replace `sha256sum` with `shasum -a 256` and its compatible check mode.

## Immutable reviewer-output verification

```sh
base=docs/qualification/adr0011-directional-raw-custody-32mib-implementation-authority-ready-v2
printf '%s  %s\n' \
  aa5a9066918eef445d6d46d73c118743fef3aba88a2e837dd67224c34661fc20 "$base/evidence/reviewers/c15-lease-source-v2-go.output" \
  fa2c94fa9f9b0c65564720de48c4f714e5da0c3a0d76fa8aff168f6525c25f0c "$base/evidence/reviewers/c15-two-stage-contract-go.output" \
  6fa84fe0fa45bc9cc9b2183d652a20d27629924c93d47984925bf066f1123718 "$base/evidence/reviewers/private-interface-v3-go.output" | sha256sum -c -
wc -c \
  "$base/evidence/reviewers/c15-lease-source-v2-go.output" \
  "$base/evidence/reviewers/c15-two-stage-contract-go.output" \
  "$base/evidence/reviewers/private-interface-v3-go.output"

python3 - <<'PY'
import json
from pathlib import Path
base = Path("docs/qualification/adr0011-directional-raw-custody-32mib-implementation-authority-ready-v2/evidence/reviewers")
expected = {
    "c15-lease-source-v2-go.output": (977346, "ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_LEASE_SOURCE_V2_GO"),
    "c15-two-stage-contract-go.output": (673755, "ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_TWO_STAGE_CONTRACT_GO"),
    "private-interface-v3-go.output": (736979, "ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_PRIVATE_INTERFACE_V3_GO"),
}
for name, (length, token) in expected.items():
    data = (base / name).read_bytes()
    assert len(data) == length
    records = [json.loads(line) for line in data.splitlines()]
    final = records[-1]
    assert final["type"] == "assistant"
    texts = [item["text"] for item in final["message"]["content"] if item.get("type") == "text"]
    assert texts == [token]
    assert all(word not in texts[0] for word in ("BLOCKED", "REVISE"))
PY
```

The parser proves each exact GO token is the final assistant record. Because it is the final record, there is no subsequent blocked or revise context.

## Scope verification

```sh
git diff --name-only be771d5627f9a2588781ac67707d7eb3c7b8d314 --
git status --short
grep -R "AUTHORED_NOT_GO" \
  docs/qualification/adr0011-directional-raw-custody-32mib-test-writeset-go-v1 \
  docs/qualification/adr0011-generic-directional-calls-profile-amendment-go-v1 \
  docs/qualification/adr0011-directional-raw-custody-c15-lease-source-v2-go-v1 \
  docs/qualification/adr0011-directional-raw-custody-c15-two-stage-contract-v2 \
  docs/qualification/adr0011-directional-raw-custody-32mib-private-interface-go-v3
```

Before review, the diff/status must contain only this new versioned receipt directory. No build, test, vet, generator, runtime, provider, browser, deployment, implementation, commit, or production command is part of receipt authorship.

AUTHORED_NOT_GO
