# Corrected ADR0011 C15 delta qualification

## Disposition

`C15_DELTA_GO`

## Enumerator derivation

- Canonical packaged SHA-256: `b5e1fc97e14fddb02ee700963e1e36831446c81372c3c3ef8004270207358012`
- Derived SHA-256: `46128beee82ac8eb9deadfa96bdb6422a166e8462ec8be5247c1410673cacd9b`
- Derived bytes differ only by the authorized HEAD and patch-SHA literal substitutions; reverse substitution reconstructs canonical bytes exactly.
- Canonical enumeration test passed.

## Mechanical accounting

- Observed/generated: 4154
- Added: 309
- Removed: 2
- Changed: 41
- Unchanged: 3804
- Duplicate IDs: baseline=0 generated=0
- Affected universe: 352 unique IDs

## Semantic adjudication

- NECESSARY: 29
- REUSE: 4
- FALSE_POSITIVE: 319
- UNRESOLVED: 0
- FALSE_POSITIVE complement SHA-256: `0bdfdeb69508e0f3e21b4d5287f7d18fdba2f7b7b78d0d2db014558a79e223b9`

Every affected row appears in `ADJUDICATION_CORRECTED.tsv`, `.json`, and `.md`. The 33 NECESSARY/REUSE rows carry ownership, custody, copy, lifecycle, and source evidence in `ADJUDICATION_CORRECTED.json`; every FALSE_POSITIVE row carries its canonical excluded classification.

## Correction comparison

A disposable replay of the prior accepted patch proved all three production files byte-identical. The only source difference is the authorized test map-key alignment from `W04/W05/W23` to `world-04/world-05/world-23`. The corrected canonical enumeration artifacts are byte-identical to the prior accepted enumeration artifacts.

## Cleanup

The temporary enumerator was removed. The corpus retains exactly four tracked modified files, no untracked files, and patch SHA-256 `ca762452805215750a122ee250e92326387c343e92c36b844fcaf12ad2cefcba`.
