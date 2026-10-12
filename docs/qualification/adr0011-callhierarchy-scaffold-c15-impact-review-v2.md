# ADR-0011 Call-Hierarchy Scaffold C15 Impact Review v2

Status: **INDEPENDENT REVIEW / WRITE-SET GO**

- Repository: `/Users/schwa/dev/lsp-trace-adr0011-c17`
- Reviewed HEAD: `2f5fd760f8d39cf0bca63258291bffd67f44a06c`
- Reviewed corrected assessment: `docs/qualification/adr0011-callhierarchy-scaffold-c15-impact-assessment.md`
- Corrected assessment SHA-256: `603ebe7f580cab60bb5e1649f5a80b536ad5a809aa758ab0607a825cd495e601`
- Preserved predecessor blocked review: `docs/qualification/adr0011-callhierarchy-scaffold-c15-impact-review.md`
- Predecessor blocked-review SHA-256: `4269959e3af03b255592c42d58e77c0127b97259cb23595e3857c730b087320c`
- Reviewed experiment: `docs/qualification/evidence/adr0011-callhierarchy-scaffold-c15-experiment-v1/`
- Review authority: write-set policy only; no implementation, source, test, catalog, semantic-adjudication, evidence-rewrite, production, or commit authority
- Disposition: `ADR0011_CALLHIERARCHY_SCAFFOLD_C15_WRITESET_GO`

## Decision

The corrected assessment resolves the predecessor review's B1–B3 blockers for the pinned experiment and states an acceptably narrow future-successor write-set. The write-set gate is issued only for that policy boundary. It does not authorize implementation, regeneration in the reviewed worktree, artifact creation, semantic ownership changes, Manager changes, or commit.

The original blocked review is preserved as immutable predecessor history. This v2 review does not rewrite or negate that review's correct finding about the former unsupported `3,805→3,888` / `+83` claim; the corrected assessment withdraws that claim and supplies a separately pinned, independently reproduced experiment.

## Independent verification

### Packet integrity and exact reproduction

From exact HEAD `2f5fd760f8d39cf0bca63258291bffd67f44a06c`, I independently:

1. verified every repository-relative entry in the packet's `SHA256SUMS` manifest;
2. verified `source.patch` SHA-256 `2934e297165b05cd986c96c20888f74a5f22e7cb3ba555f4c099883ece48c269`;
3. created a disposable checkout from the exact HEAD;
4. applied the exact packet-local `source.patch` after `git apply --check`;
5. ran the canonical regeneration command:

```sh
C15_REGENERATE=1 go test ./sessionruntime \
  -run '^TestPrivateB4C15RegenerateOfflineCatalog$' \
  -count=1
```

6. byte-compared all four regenerated outputs and sidecars to the packet's retained after-artifacts.

The canonical regeneration passed and reproduced the retained bytes exactly.

### Finite delta

Independent row and retained `delta.json` checks establish:

- candidates: `3,805→3,838`, exactly 33 added;
- classifications: `3,805→3,838`, exactly 33 added;
- all 3,805 predecessor candidate and classification rows retained byte-identically;
- all 33 added classifications are typed `excluded`;
- zero removed rows;
- zero changed or reclassified rows.

These facts apply only to the exact pinned experimental patch and generator at the reviewed HEAD. They establish mechanical AST-derived catalog impact, not runtime execution, volume, feature identity, ownership, or production behavior.

### Mapping and Manager boundary

The accepted operation mapping remains byte-identical at SHA-256 `34a8ffa1bbc26775c42a2329f5ca9652444e6832ba790a3cc8673f4b15c4ffd1` and 44 rows: 15 Owner, 7 Alias, and 22 Copy. No mapping change is authorized or implied.

The exact patch changes only `sessionruntime/b4_owned_method_pair_lease.go`. The retained experiment reports no `sessionruntime` Go diff after resetting the lifecycle patch and records Manager propagation as absent. Independent patch-path inspection found no Manager path. No Manager source, ownership, behavior, or propagation change is authorized or implied.

## Three-state distinction and immutable history

The corrected assessment correctly separates:

1. **Immutable v1/current HEAD:** the canonical non-writing verification reproduces the retained 3,805-row `c15-census-v1` artifacts. Their current hashes remain `13d45a61…` for candidates and `cde6aa720…` for classifications. They are historical evidence and must not be overwritten, refreshed, renamed, redirected, normalized, or deleted.
2. **Pinned patched experiment:** the exact disposable overlay alone reproduces 3,838 rows and the exact +33/all-excluded finite delta. `experiment-v1` is retained non-authoritative evidence and must not be mutated to match later implementation.
3. **Future final v2:** final bytes, identities, counts, hashes, and classifications remain unknown until accepted final lifecycle source spelling is fixed and independently regenerated under a successor-specific contract. The experiment's +33 is not a normative v2 expectation and may not be copied forward without exact reproduction.

The predecessor assessment/review, closure pins, v1 artifacts and sidecars, operation mapping, and experiment packet remain immutable history. Corrections require new successor artifacts rather than retroactive rewriting.

## Accepted narrow successor write-set policy

After separate acceptance of final lifecycle implementation and a separate authorization to generate successor evidence, the write-set may contain only:

1. the exact accepted lifecycle source files and lifecycle tests named by that later gate;
2. versioned `sessionruntime/testdata/c15-census-v2/candidates.tsv` and its SHA-256 sidecar;
3. versioned `sessionruntime/testdata/c15-census-v2/classifications.tsv` and its SHA-256 sidecar;
4. narrowly scoped successor-specific generator/test metadata required to bind exact inputs, deterministic generation, v1→v2 finite-delta accounting, sidecar/hash verification, non-reading of generated outputs, and the unchanged-44 mapping guard.

It must not perform broad regeneration or modify v1 artifacts, v1 sidecars, operation mapping, closure matrices or pins, predecessor qualification/evidence, Manager code, unrelated source/tests, catalogs, generated production bindings, or cleanup targets. Exact final source/test paths and exact v2 bytes require a later independent review; this gate does not pre-authorize them.

## Gate

```text
ADR0011_CALLHIERARCHY_SCAFFOLD_C15_WRITESET_GO
```
