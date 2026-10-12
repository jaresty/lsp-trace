# ADR-0011 Call Hierarchy Layer-1 RED Oracle Corpus v1 — Independent Semantic/Adversarial Review v5b

**Verdict: `ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_BLOCKED`**

Reviewed exact repository HEAD `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`, the corrected untracked design `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`, and the prior reviews. The pre-existing RED modification remains unchanged. This review creates only this v5b document and authorizes no corpus bytes, fixture update, loader, source or RED edit, testdata, runtime behavior, generated output, production behavior, or commit.

## Independent evidence boundary

This was a fresh static semantic/adversarial rereview. I inspected the four normative tables and the expanded records, independently recomputed the ordered root-hash aggregate from the 25 Table 4 world rows, and did not use implementation/runtime output to choose or repair expected values. Prior review conclusions were treated as questions to recheck, not as authority.

## Mechanical result and aggregate hash

The corrected design retains 25 world roots, 16 World 24 workers, and 22 World 25 executions arranged as 11 enabled/disabled pairs. The Table 4 root rows are complete from `world-01` through `world-25`.

Aggregate SHA-256: `d70d3107a7eac21ef57b3488ebec91c09798ca6005582ef0e03830abcbfbfba2`, computed over root-name-sorted UTF-8 lines:

```text
world-NN<TAB>expanded-world-sha256<LF>
```

Boundary pins were `world-01 = 6e709f397b1df965caadc2cf6571a50de0e8c2f063ec8b36cae7b925a8a36b76` and `world-25 = d954ef0e54a68fe95b3ca2a05fa5111507162b3246617a5077f69892f67e1d86`.

## S1–S5 rereview

### S1 — corrected for Worlds 3–5

World 3 now records actual JSON `null`: result bytes `6e756c6c`, form `null`, and an exact response body whose top-level result is `null`. Worlds 4 and 5 now record distinct nonempty incoming/outgoing response bodies and nonempty result bytes. Their frame lengths, body lengths, and hashes differ as required. The former collapsed `[]` result defect is corrected for these three worlds.

### S2 — corrected for Worlds 6–11

Worlds 6–11 now use pairwise-distinct fixed accepted-input payloads and coherent protocol, malformed, cancellation, deadline, shutdown, and transport-unavailable records. Each has an unavailable result, a present error with distinct class/code/message/data/serialization, a matching failed terminal, exact request/response frames, matching external failure, transport/accounting values, complete lifecycle, and all seven ledger dimensions with terminal-after matching the world’s terminal. The former shared-success collapse is not present.

### S3 — payload and mutation inputs corrected, but one output contradiction remains

The input and mutation evidence is explicit:

- World 12 payload is unavailable: null bytes, length, and digest.
- World 13 payload is available with zero bytes and the empty SHA-256.
- World 14 payload is available with 13 bytes (`fixed-payload`) and its nonempty digest.
- World 15 records source pre/post values, bytes, and digests; source post changes to `changed-source!`, retained pre/post remain exactly `immutable-source`, and `no_alias=true`.

However, World 12 is internally contradictory. Its response frame/body contains top-level `"result":null`, and its terminal plus ledger terminal-after say `successful-null`, but `CORE_12_RESULT` is:

```json
{"available":true,"bytes_hex":"5b5d","form":"empty"}
```

`5b5d` is `[]`, not `null`. Thus the result record is not derived coherently from the fixed response bytes, and the world simultaneously asserts empty and null output semantics. This is a blocking oracle defect.

### S4 — constructor smuggling corrected

`C_RECORD` now receives explicit identity, frames, result, error, terminal, external, accounting, transport, commitments, lifecycle, ledger, refusal, and applicable control/comparison/concurrency records. Its mapping is neutral field-for-field assembly and expressly does not branch on enabled state, world/record name, kind, refusal reason, or scenario label. `C_WORLD` only copies the supplied record and adds supplied provenance/root identity. No canned success/error/refusal semantics are hidden in either constructor.

### S5 — refusal coherence corrected

Worlds 17–23 represent refusal only in the refusal record. Their error form is coherent and externally transparent: available error observation, `present=false`, class `none`, null code/message/data, and canonical serialized `null`. Exact external request/response and transport observations remain present; refusal does not rewrite the external surface. Selected, handoff, and predecessor values remain opaque availability/identity/length/SHA-256 commitments.

## World 24

World 24 names exactly 16 complete worker records. The declared aggregate remains exact over those workers: callbacks 0, diagnostics 0, external bytes 608, leases 0, reservations 0, transport bytes 960, and transport messages 16. No worker name selects constructor behavior.

## World 25

World 25 names exactly 22 executions in 11 enabled/disabled pairs. Within every pair, request/response frames, external observations, and transport observations are equal. The enabled member exposes the explicitly supplied observation-owned result/error/terminal/accounting/commitment/lifecycle/ledger state. The disabled member preserves the same external and transport facts while marking only those internal observation-owned fields unavailable. The difference is observation-only; disabling does not change the represented external execution.

## Opacity and authority

Selected, handoff, and predecessor commitments remain opaque; the corpus does not decode them or infer CALLS direction from them. Provenance remains synthetic and `runtime_recorded:false`. The design and this review provide no production authority, no source-completeness claim, no update/bless authority, no producer-authentication claim, and no authority to derive expected values from implementation or runtime output.

## Disposition

Mechanical closure, constructor neutrality, refusal transparency, Worlds 3–11, World 15, World 24, and World 25 pass this rereview. Semantic completeness does not pass because World 12’s explicit result atom is `[]` while its exact response and terminal semantics are `null`.

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_DESIGN_GO` is therefore not issued. Correct `CORE_12_RESULT` and every dependent expanded length/hash, then obtain another independent semantic and mechanical review.
