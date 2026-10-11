# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Independent Semantic/Adversarial Review v6b

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_BLOCKED`**

Reviewed exact repository HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` and the corrected untracked design `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`. This was a fresh static semantic/adversarial review. It authorizes no runtime, testdata, fixture, loader, source, RED, generated output, implementation, production behavior, or commit.

## Independent four-table expansion

I freshly parsed all four normative tables. Counts were:

- Table 1: 860 atomic rows
- Table 2: 4 constructor rows
- Table 3: 63 complete records
- Table 4: 88 explicit invocations/roots

Every Table 1 and Table 3 canonical literal independently reproduced its declared UTF-8 length and SHA-256. Table 4 closes over exactly 25 world roots, 16 workers, and 22 executions, with no missing referenced complete record in those sets.

The ordered current Table 4 world-root aggregate, computed over root-name-sorted UTF-8 lines `world-NN<TAB>expanded-world-sha256<LF>`, is:

`88b897ed1e59e7d53c167d6cbd30c943f59b396e7e7ea6520a28b50598a56ac0`

## Blocking constructor/expansion contradiction

Table 4 cannot be reproduced from the Table 2 constructor contracts.

1. `C_RECORD` takes explicit `kind` and Table 2 says its output fields are exactly the supplied fields. Literal neutral assembly of `CORE_01` therefore includes `"kind":"world"` and is 4,550 bytes, while Table 4 declares 4,535 bytes and Table 3 omits `kind`. Dropping `kind` makes all 63 complete-record invocations match, but that is an unstated transformation contrary to the exact supplied-field mapping.
2. Under the declared `C_WORLD` mapping—copy the supplied complete record and add the supplied expanded provenance and root identity—every world is exactly 64 bytes larger than its published Table 4 root. For example, `world-01` expands to 4,695 bytes with SHA-256 `f04514dd4cf363bc960233b86686b4c6943dbca3c8ae265e8b5ea546429a8b38`, not the declared 4,631 bytes and `6e709f397b1df965caadc2cf6571a50de0e8c2f063ec8b36cae7b925a8a36b76`. The identical 64-byte discrepancy holds for all 25 roots.

Thus the registry is not mechanically self-defining: reproducing its published records and roots requires hidden constructor projections. That violates its no-omission/no-ambient-value rule and blocks oracle qualification even though the semantic atoms below are otherwise coherent.

## S1–S5 and required semantic contrasts

- **S1 passes:** World 3 has response/result `null` (`6e756c6c`, form `null`). Worlds 4 and 5 contain distinct fixed nonempty incoming/outgoing results with distinct bodies, frame lengths, and hashes.
- **S2 passes:** Worlds 6–11 use pairwise-distinct accepted inputs and distinct protocol, malformed, cancellation, deadline, shutdown, and transport-unavailable error tuples. Results are unavailable, terminals match the six failure classes, external failure labels match those classes, and detailed code/message/data/serialization remain coherent.
- **S3 passes:** World 12 now consistently represents unavailable request payload plus top-level/result `null` and `successful-null`; World 13 represents available zero-length request payload plus `[]` and `successful-empty`; World 14 is available nonempty. World 15 preserves retained pre/post bytes/value/digest while changing the source and asserting `no_alias=true`.
- **S4 remains blocked:** semantic atoms are explicit and no canned scenario outcome is visible, but the constructor contract silently discards or transforms supplied fields as described above. It is therefore not a complete neutral expansion rule.
- **S5 passes:** Worlds 17–23 express refusal only through `refusal`; error observation is present but has `present=false`, class `none`, null code/message/data, canonical serialized `null`, while exact external and transport observations remain intact.

## World 24

World 24 names exactly 16 distinct complete workers. Independently summing their seven ledger deltas yields callbacks 0, diagnostics 0, external bytes 608, leases 0, reservations 0, transport bytes 960, and transport messages 16. `comparison` itself is unavailable; these totals are reviewer-derived from the named workers, not a stored aggregate object.

## World 25

World 25 names exactly 22 executions in 11 enabled/disabled pairs. In every pair, frames, external records, and transport records are exactly equal. Enabled records expose result/error/terminal/accounting/commitment/lifecycle/ledger observations; disabled records preserve the same external execution while marking those internal observation-owned fields unavailable. The contrast is observational rather than an external-execution mutation.

## Opacity and authority boundary

Selected, handoff, and predecessor commitments remain opaque; no CALLS direction or runtime meaning is derived from them. The design explicitly remains manually authored, synthetic, and non-runtime-recorded. It grants no source-completeness, producer-authentication, update/bless, runtime-derivation, production, or implementation authority.

## Disposition

The prior World 12 null/empty defect is corrected, and the semantic worlds, refusal policy, closure counts, World 24 arithmetic, World 25 contrast, and opacity boundary pass. Qualification is nevertheless blocked because the normative constructor mappings do not expand to the normative Table 3/Table 4 bytes and hashes.

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO` is not issued. Define explicit input-only discriminator/validation fields versus emitted fields for `C_RECORD` and `C_WORLD`, regenerate all dependent canonical lengths/hashes under that exact mapping, and obtain another independent review.
