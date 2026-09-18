# Network-denied pilot worker run

- **Status:** `DIAGNOSTIC_RUN_COMPLETE`
- **Pilot enablement:** `DISABLED`
- **Worker:** SHA-256 `623ad4341720192d57fe95372343ad7d87450ef8bfb5baa6ecc0c05288d16d83`
- **Model:** Qwen2.5-Coder-7B-Instruct-Q4_K_M; SHA-256 `1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6`
- **Native bundle:** `/tmp/adr0007-llama-lib`, llama.cpp `v0.4.0`
- **Sandbox:** macOS `sandbox-exec`, `(allow default)(deny network*)`
- **Worker timeout:** `90s`
- **Max tokens:** `384`
- **Context:** `16384`

## Results

All four recovered fixed-topology packets completed under the network-denied profile:

| Packet | Status | Verdict | Consumer | Tokens |
|---|---|---|---|---:|
| transient structural context | `COMPLETE` | `SUPPORTED` | `C2 OUTWARD_CONSUMER` | 225 |
| incoming calls | `COMPLETE` | `SUPPORTED` | `callContext` | 133 |
| slice | `COMPLETE` | `SUPPORTED` | `callContext` | 129 |
| lifecycle | `COMPLETE` | `SUPPORTED` | `callContext` | 136 |

Result files are retained beside this record with their SHA-256 digests:

- `01-transient-structural-context.result.json`: `83bdee10cf4439b617c10d2dd6d64591092f59e68d757f576ef449acfdd00897`
- `02-incoming-calls.result.json`: `e0b57a854036e02cd13c709d9f0d94baf115d94c0426deb16186f6ad16fee848`
- `03-slice.result.json`: `fee2124629099f556059d483a1ca834f2e0332be5056bfab9de756b7f6de94bd`
- `04-lifecycle.result.json`: `aca17e1ec6de291425681c9c42d0bc05b1ba84929287ea5ed8020c38a37f7cd8`

This is a successful diagnostic run under network denial. It is not yet a frozen NDJSON pilot execution: process/resource limits, admission records, full conformance, and enablement remain incomplete.
