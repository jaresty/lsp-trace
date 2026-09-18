# G2 admission and lineage draft

- **Status:** `PREREQUISITES_DRAFT`
- **Pilot:** `PILOT_DISABLED`
- **Scope:** documentation-only requirements for typed admission and immutable derivation lineage.

## Admission boundary

The caller performs `Select → Resolve → Assemble` before admission. The semantic worker receives only caller-admitted assembled records. It cannot select additional objects, resolve selectors, open a source store, read a workspace or live session, repair ranges, substitute bytes, or broaden relationships.

Each admission belongs to exactly one corpus and one finite item type. The initial corpus families are:

- revision-bound code and structural evidence;
- accepted decisions and requirements;
- working context.

The exact finite item-type vocabulary remains a G2 freeze decision.

## Required admission fields

A frozen admission record must contain:

- immutable admission ID;
- corpus and finite item type;
- acquisition mode: `TARGET`, `NEIGHBORHOOD`, or `CENSUS`;
- exact submitted-byte digest and byte length;
- schema/media and canonicalization identities;
- logical source selector and immutable source identity;
- source revision and/or content digest;
- custody and authority classification;
- source acceptance and currentness state;
- privacy and policy identities;
- declared scope, expansion rules, exclusions, failures, and denominator;
- exact assembled range, symbol, document, or passage identity;
- graph subject and admitted relationships, when applicable;
- admission disposition and complete provenance references.

Digest and length establish byte integrity only. They do not establish producer authentication, source truth, completeness, custody, ownership, semantic authority, or feature identity.

## Coverage ceilings

- `TARGET` means exact admitted-item coverage only.
- `NEIGHBORHOOD` means only the declared bounded expansion from explicit targets.
- `CENSUS` means only the declared closed scope.

Partial indexes, bounded zero results, exclusions, failures, and empty results never imply absence outside the admitted scope or completeness of a repository, source, or engineering domain.

## Immutable lineage

Every derived description, embedding, index member, search result, group, and context packet binds:

- all direct admission identities;
- ordered dependency identities;
- representation, prompt, model, runtime, policy, protocol, and schema identities;
- predecessor and `supersedes` references;
- correction-event ID, actor identity, and actor authority classification;
- versioned neutral `context_state_delta`;
- affected products and invalidation/rebuild dispositions.

Corrections, deletions, revocations, source changes, policy changes, and dependency changes append new records. They never mutate or erase predecessor evidence.

## Required G2 rejection cases

Admission must fail closed for mutable identity, missing source revision where required, missing custody or policy identity, unaccounted expansion, invalid range, digest mismatch, unsupported corpus/item type, unqualified relationship, or incomplete provenance. A failed admission must preserve its attempt record and reason without exposing it as admitted evidence.

All generated semantic products remain `authority=0` and `accepted=false` regardless of admission source authority.
