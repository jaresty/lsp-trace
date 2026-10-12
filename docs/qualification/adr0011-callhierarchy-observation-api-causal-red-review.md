# ADR0011 Call-Hierarchy Observation API Causal RED Review

## Scope

Final independent gate adjudication of exact HEAD `ea049c2dad9be74e1b3e316fe042430be7f04c8e` (`ea049c2`) plus the single modified test file. No source, test, corpus, or other documentation was changed during adjudication.

## Reviewed write set

Exactly one pre-existing modified file was reviewed:

- `sessionruntime/b4_owned_method_pair_lease_test.go`
- Diff: `1 file changed, 1448 insertions(+)`, `1448/0` numstat
- Modified-file byte length: `106476`
- Modified-file SHA-256: `ea95dda977dc4a4153980c7b6333419c814af57679e898faaecdc20828a9c030`
- Binary diff byte length: `87378`
- Binary diff SHA-256: `f76c801188f4963f983837c094aed45225358f9138289c539f63fbb0bbf0c6fa`

This review document is the only file added by the adjudication record step.

## Corpus pins and source provenance

The accepted immutable corpus is pinned as follows:

- Schema: `adr0011.callhierarchy.layer1.red-oracle-corpus/v1`
- Source HEAD: `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`
- Aggregate byte length: `431974`
- Aggregate SHA-256: `7fddcd923a9299c52014334d5460cb1a5ce78604e4a732faaef11d02adc30e84`
- Manifest SHA-256 at reviewed HEAD: `c761601bf78195634163b98676e133bac2dff610c521be149b3445c9f620492a`
- Ordered corpus: `world-01.json` through `world-25.json`

The harness verifies the manifest schema, source HEAD, aggregate byte length/hash, ordered entries, per-world byte lengths/hashes, JSON decoding, and root identity before semantic execution. No corpus, external-execution, harness, decode, hash, or path mismatch occurred.

## Verification

### Layer 1 causal RED

Normal command:

```bash
go test ./sessionruntime -run '^(TestCallHierarchyLayer1AcceptedCorpusRED|TestCallHierarchyLayer1MutationPayloadAndPropagationRED)$' -count=3
```

Result: expected RED; `0 passed, 96 failed in 1 packages`. All three repetitions produced the intended API/propagation failures. There was no panic and no zero-cascade defect.

Race command:

```bash
go test -race ./sessionruntime -run '^(TestCallHierarchyLayer1AcceptedCorpusRED|TestCallHierarchyLayer1MutationPayloadAndPropagationRED)$' -count=2
```

Result: expected RED; `0 passed, 64 failed in 1 packages`. Both repetitions produced the same intended API/propagation failure classes. There was no race report and no panic.

### C15/private preservation

Normal command:

```bash
go test ./sessionruntime -run '^(TestPrivateB4OwnedMethodPair|TestPrivateOwnedMethodPair)' -count=3
```

Result: GREEN; `30 passed in 1 packages`.

Race command:

```bash
go test -race ./sessionruntime -run '^(TestPrivateB4OwnedMethodPair|TestPrivateOwnedMethodPair)' -count=2
```

Result: GREEN; `20 passed in 1 packages`.

C15/private ownership, refusal atomicity, lifecycle, exact-once release, production publication/consumption, equality/plus-one, callback drain/re-entry, terminal admission, and race preservation remain GREEN. The historical hard-coded `privateB4C15V2FinalCount == 3847` expectation is retained only in the explicitly superseded hand-authored block; the accepted corpus is the sole active Layer 1 oracle. Active historical accounting markers remain required through `knownZero`, `delta`, terminal before/after, owners/accounting transfers, and transport publication/retention fields.

## Assertion-family classification

Every observed failing assertion belongs to a precise missing API or propagation family:

1. `ASSERT_CH_API_GAP_*`
   - Missing result snapshot/API fields: notifications, responses, failure, request messages/bytes, external result/error/transport values.
   - Missing ledger and historical accounting fields: per-dimension `delta` and `knownZero`, accounting owners/transfers, transport publication/retention, and aggregate transport/external accounting.
   - Missing complete observation fields: identity, request/response frames and payload spans, commitments, accounting, lifecycle, refusal, result/error/terminal, transport, and round-trip ledger.
   - W24 aggregate and W25 comparison gaps remain precise implementation-facing API requirements.
2. `ASSERT_CH_CORPUS_PROPAGATION_MISSING_*`
   - `RoundTripResult.callHierarchyObservation` is absent only where the corpus expects an enabled, non-refused retained observation.
3. `ASSERT_CH_LAYER1_PROPAGATION_PAYLOAD_{nil,empty,nonempty}`
   - The manager does not yet publish the Layer 1 result snapshot for all payload tri-state cases.
4. `ASSERT_CH_LAYER1_PREPARE_OUTGOING_PROPAGATED_SEPARATELY`
   - Prepare and outgoing result attachments are not yet independently propagated.

Expected-absence records are rechecked before observation consumption: unexpected presence would fail `ASSERT_CH_CORPUS_OBSERVATION_EXPECTED_ABSENT_*`; that family did not fail. No pseudo-consumption was observed: W25 executes named enabled/disabled records independently, validates exact inline-to-named record identity, verifies actual request-attachment availability, and compares independently observed external, transport, and observation outcomes.

No failing assertion indicated a corpus defect, external execution defect, harness defect, decode failure, digest/hash failure, path mismatch, panic, race, or unrelated behavior regression.

## Coverage

The active harness covers and guards:

- `25` ordered top-level corpus worlds.
- W24: `16` independently executed concurrent workers plus aggregate ledger comparison.
- W25: `22` independently executed named records arranged into `11` isolated enabled/disabled pairs.

The `25/16/22/11` cardinality guards did not fail. W24 aggregate checks and W25 isolated-pair checks reached their intended API gaps without corpus/harness failure.

## Verdict

`ADR0011_CALLHIERARCHY_OBSERVATION_API_CAUSAL_RED_GO`

This verdict classifies the test as implementation-ready causal RED only. It authorizes no production behavior, API semantics, implementation, corpus change, test change, or broader write set. The implementation write set remains unspecified and unauthorized. This record authorizes and performs only creation of this qualification document; no commit is authorized or created.
