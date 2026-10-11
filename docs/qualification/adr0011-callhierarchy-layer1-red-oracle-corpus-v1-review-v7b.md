# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Final Semantic Qualification Review v7b

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO`**

Reviewed exact repository HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9` and the corrected untracked design `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`. This was a fresh, read-only semantic/adversarial qualification of the design. Pre-existing worktree changes were not modified. This review creates only this v7b document and authorizes no runtime edit, testdata, fixture, loader, source, RED, generated output, implementation, production behavior, or commit.

## Fresh parse and exact constructor expansion

A fresh disposable validator parsed the four normative markdown tables directly. It canonicalized JSON as UTF-8 with sorted keys, compact separators, and one LF; recomputed every stored length and SHA-256; executed the exact Table 2 constructor mappings; enforced each emitted versus validation-only partition; expanded Table 4 from the named Table 1 and Table 3 values; and checked reference closure.

Results:

```text
normative tables=4
Table 1 atomic rows=860
Table 2 constructors=4
Table 3 complete records=63
Table 4 invocations/roots=88
C_RECORD expansions=63
world roots=25
World 24 workers=16
World 25 executions=22
atomic length/hash defects=0
Table 3 length/hash defects=0
C_RECORD output defects=0
C_RECORD expanded length/hash defects=0
C_WORLD expanded length/hash defects=0
undefined references=0
```

The name-sorted world-root aggregate over `world-NN<TAB>expanded-world-sha256<LF>` is:

`88b897ed1e59e7d53c167d6cbd30c943f59b396e7e7ea6520a28b50598a56ac0`

The v6 constructor blockers are corrected without hidden drops or semantic smuggling:

- `C_RECORD.kind` is explicitly a validation-only discriminator with a total exact predicate. It is never emitted. Every other supplied input is emitted verbatim at its stated path.
- `C_WORLD.name` is explicitly validation-only and must equal both `root_identity` and `record.name`. The emitted root contains exactly `provenance`, expanded `record`, and `root_identity`.
- Literal provenance identifiers remain literal and are not dereferenced.
- All 63 record invocations and all 25 world invocations reproduce their published bytes and pins exactly.
- No scenario label, enabled flag, world name, record name, or kind chooses canned semantic output. Kind and root name select validation only.

## S1–S5 semantic qualification

A separate semantic validator executed 201 exact checks with zero failures.

- **S1 passes.** World 3 has exact top-level JSON-RPC `result:null`, result bytes `6e756c6c`, and form `null`. Worlds 4 and 5 carry distinct independently fixed nonempty incoming and outgoing results, response bodies, frame lengths, and hashes.
- **S2 passes.** Worlds 6–11 use six pairwise-distinct accepted-input payloads. Protocol, malformed, cancellation, deadline, shutdown, and transport-unavailable outcomes have coherent and pairwise appropriate result unavailability, terminal, external failure, code, message, data, canonical serialized error, frames, accounting, transport, lifecycle, and all seven ledger dimensions.
- **S3 passes.** World 12 distinguishes unavailable nil request payload and top-level/result null with `successful-null`; World 13 distinguishes available zero-length payload and empty-array result with `successful-empty`; World 14 carries an available nonempty payload. World 15 changes the source bytes/value/digest while preserving equal retained pre/post bytes/value/digest and `no_alias=true`.
- **S4 passes.** Constructor behavior is literal, neutral, and complete. Every constructor input belongs to exactly one stated emitted or validation-only partition; no unaccounted input, overlap, ambient lookup, hidden projection, derived semantic value, or canned scenario outcome remains.
- **S5 passes.** Worlds 17–23 express refusal only through the explicit refusal record. Their error observations remain available but not present, with class `none`, null code/message/data, and canonical serialized `null`; exact external and transport observations remain intact.

For every World 3–15 available request and response, body and frame lengths and SHA-256 values recompute, each frame ends in its declared body, byte accounting equals request plus response transport bytes, and every available ledger dimension satisfies `after-before=delta` and exact `known_zero`; unavailable dimensions carry only null measures and `known_zero=false`.

## World 24 aggregate

World 24 names exactly 16 distinct, closed complete-worker records. Independent summation of their seven ledger deltas yields:

```text
callbacks=0
diagnostics=0
external_bytes=608
leases=0
reservations=0
transport_bytes=960
transport_messages=16
```

These are reviewer-derived totals from the named workers, not a hidden stored aggregate or runtime claim.

## World 25 external equality and observation contrast

World 25 names exactly 22 distinct executions in 11 complete enabled/disabled pairs. Every execution is closed over a named Table 3 record. For every pair:

- request and response frames are exactly equal;
- external records are exactly equal;
- transport records are exactly equal;
- the declared `external_equal` and `transport_equal` predicates are true;
- the enabled execution exposes result, error, terminal, accounting, commitments, lifecycle, and round-trip-ledger observations;
- the disabled execution preserves the same external execution while marking observation-owned fields unavailable and lifecycle observation absent.

Thus the contrast is observation availability only. It does not mutate or select external execution.

## Opacity, authority, and closure

Selected, handoff, and predecessor commitments expose only availability, opaque identity, length, and SHA-256. They carry no CALLS direction, source meaning, feature semantics, or runtime authority. The corpus remains a manually authored expectation specification rather than runtime observation. This GO grants no source completeness, producer authentication, execution proof, runtime derivation, update/bless authority, implementation correctness, production behavior, or permission to create corpus bytes, loaders, fixtures, testdata, source edits, RED edits, generated output, or commits.

All 25 roots, 63 complete records, 16 World 24 workers, and 22 World 25 executions close under the four normative tables with no undefined reference, cycle, omitted constructor input, extra constructor input, partial constructor, missing emitted field, unexpected emitted field, or canonical mismatch.

## Disposition

The corrected design now satisfies exact constructor reproducibility, no-drop/no-smuggling custody, S1–S5, Worlds 3–15, refusal transparency, World 24 aggregate accounting, World 25 external-equality/observation-only contrast, opacity, authority ceilings, and complete reference closure.

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO`
