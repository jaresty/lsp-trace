# ADR0011 directional raw-custody 32 MiB implementation-authority readiness receipt v2

Status: `AUTHORED_NOT_GO`

Exact review token requested: `ADR0011_DIRECTIONAL_RAW_CUSTODY_32MIB_V3_IMPLEMENTATION_AUTHORITY_READY`

Receipt authoring HEAD and current implementation base: `be771d5627f9a2588781ac67707d7eb3c7b8d314` (clean before authorship).

## Purpose and external-review reconciliation

This documentation-only successor binds durable external GO provenance to exact committed dependency bytes. Each producer package remains immutably `AUTHORED_NOT_GO`: a producer cannot approve itself. The external verdicts recorded in `REVIEWER_RESULTS.md` affect only (1) applicability of the reviewed bytes as dependencies and (2) applicability of their old target-HEAD pins along the verified successor chain ending at the current implementation base. They do not rewrite package bytes, convert producer status, broaden architecture, relax exclusions, raise authority ceilings, or erase hard stops.

## Bound dependency chain

- accepted raw design commit: `0e2583b2a535701da152e756253b4b96df55272b`;
- exact six-production/four-new-test write-set package commit: `2faac2671987f6e6f7c680fe4f072064cfe48136`;
- generic amendment GO dependency commit: `f03672416bbe1342c06e7cbb33bf51127909ce8a`;
- C15 lease-source V2 GO dependency commit: `a1cd77950ffa45ebed52310878f772f3c7344756`, package-manifest digest `6d764eb05029c5663c740ac0bfa452552eeb7f07e5a74f68b45e144ffe15d4a6`;
- two-stage C15 contract GO dependency commit: `acf563e7708095d8a32c809b49d97c7432d49b03`, v2 run-manifest digest `12fb6b1f1fecb7eda022b0ffe84750cca5bcefbb75e87fcaa48d24016228e8e8`;
- private interface V3 GO dependency and current base commit: `be771d5627f9a2588781ac67707d7eb3c7b8d314`, package-manifest digest `02153887105c30cc6eaa3b976ec49e7f32b97afcf35ff6b8aaf185529e029704`.

The package commits form a verified ancestor chain to `be771d56…`. No production or test mutation occurs in this receipt.

## Exact implementation write-set

Future implementation authority, if separately granted, is confined to exactly the six production paths and four new test paths frozen by the reviewed packages. There is no fallback, historical-test mutation, fixture/corpus/golden change, or write-set expansion.

## Frozen staged authority

A later explicit implementation GO may authorize only this sequence:

1. scaffold the reviewed V3 private declarations and exact test owners in the six-plus-four write-set;
2. establish causal RED through the real named missing seams, not fabricated injection, test-only behavior, or unrelated compilation failure;
3. implement GREEN only in the six production paths, preserving every ownership, capacity, directionality, compatibility, and default constraint;
4. after a committed implementation exists, run canonical Stage B actual-source enumeration from its exact source manifest;
5. independently adjudicate every actual row and its correction lineage until `UNRESOLVED=0` before any `IMPLEMENTATION_GO` may be considered.

Stage A planned obligations permit implementation planning and, only after a separate explicit implementation GO, implementation. They are not semantic C15 classifications and never establish `NECESSARY`, `REUSE`, `FALSE_POSITIVE`, or source-semantic completeness. Stage B cannot be prefilled, inferred from plans, or bypassed.

## Authority ceiling and hard stops

This package requests implementation-authority readiness only. It is not implementation GO, RED GO, test GO, Stage B GO, `IMPLEMENTATION_GO`, activation, provider/profile GO, production admission, deployment, mutation authority, or commit authority.

Stop without widening on any checksum/pin mismatch; inaccessible or ambiguous reviewer provenance; unlisted path; historical test/fixture/corpus/golden change; public/global/schema/config/profile/registry/build-tag/dependency/package-boundary/default change; duplicate recorder/backing/terminal/C15 account; duplicate charge or lease-free owned bytes; reconstructed raw bytes; non-atomic transfer; release orphan/double; lock inversion; C15/directional ownership merge; incoming/outgoing inheritance; provider semantics in the generic layer; non-causal RED; incomplete exact accounting; Stage A semantic classification; Stage B not sourced from an exact committed implementation; Stage B duplicate/omission; incomplete planned-to-actual lineage; or `UNRESOLVED>0`.

Read in order:

1. `REVIEWER_RESULTS.md`
2. `DEPENDENCIES.sha256`
3. `COMMANDS.md`
4. `REVIEW_REQUEST.md`
5. `SHA256SUMS`

AUTHORED_NOT_GO
