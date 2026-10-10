# ADR 0011 → ADR 0007 private operational composition contract v2

- **Schema:** `lsp-trace.adr0011-adr0007-private-composition-contract.v2`
- **Status:** corrected documentation candidate; not accepted; not GO
- **Authority:** `0`
- **Completeness:** `UNKNOWN`
- **Feature identity:** `UNRESOLVED`
- **Positive-world scope:** `CAUSAL_TEST_ONLY`; `production_admissible=false`
- **Frozen v1 authority:** `docs/qualification/adr0011-adr0007-private-composition-contract.effe3f74.md`, SHA-256 `effe3f7482029a55ce9c13ab6e3fb95fb0c813032087e1820a4afac5cfddd9b9`
- **Unchanged causal specification:** `docs/qualification/adr0011-adr0007-private-composition-causal-tests.1105f75e.md`, SHA-256 `1105f75e5a46d6bb3e9382039fd7a321ec9d87a1cbd92d4822d36847472bc28d`
- **Literal appendix:** `docs/qualification/adr0011-adr0007-private-composition-v2-literal-appendix.json`, pre-freeze SHA-256 `fb26ed5ee4e95f3f0857495626cd13eb12ed66919599f4c9148ae65e35ec4c84`, 189256 bytes

## 1. Scope

This version supplies exactly one reconstructible minimal CALLS-only causal-test world and one counterworld. It does not amend v1, ADR0011, ADR0007, the causal specification, historical artifacts, implementation, runner/library dependencies, public or internal interfaces, CLI, MCP, catalog, capability discovery, schemas, release, deployment, publication, acceptance, or GO.

The positive world has one CALLS occurrence, one community, one CALLS representative, two independently custodied source roles, one member per private stage, one candidate, six completion checkpoints, and one private test-generation commit. Its Stage4 qualification receipt is test-only and cannot be consumed by production.

## 2. Single normative contract identity

There is exactly one normative v2 contract identity:

```text
sha256:6d9777305530d08fbbf2e3fba34983556007107726799511a26473eb2f31a69c
```

It is the domain-separated identity of appendix artifact `contract-definition`, not a prose-document hash and not a hash of its own identity. Its exact decoded LF JSON bytes name the v2 schema, version 2, base v1 and causal-spec SHA-256 values, test-only world, and unchanged ceiling.

Every identity-bearing positive and counterworld artifact carries this exact identity. The request, generation key, stage requests/results, checkpoints, final generation, mutation reseals, and refusal therefore have one acyclic contract commitment. No other contract identity is normative.

## 3. Canonical reconstruction

For every appendix artifact `a`:

```text
bytes(a) = RFC4648-standard-padded-base64-decode(a.canonical_bytes_base64)
len(bytes(a)) = a.byte_length
sha256(bytes(a)) = strip_prefix(a.sha256)
a.identity = "sha256:" + sha256(
  UTF8(a.domain) || 0x00 || UTF8(a.schema) || 0x00 || bytes(a)
)
```

`bytes(a)` is strict duplicate-free UTF-8 JSON with object keys sorted by Unicode code point, comma/colon separators, no insignificant whitespace, and exactly one trailing LF. No BOM, CR, extra LF, alternate base64, normalization, pretty-print, map-order, or integer rewrite is permitted.

Every carrier decodes to the exact closed `ExactCarrierV1` wrapper and resolves to exactly one artifact. Carrier SHA, carried-object SHA, and domain-separated object identity are distinct.

The positive request, generation key, and final generation are:

```text
request=sha256:b746ddc857f35d78fdcfcad449da529660fe13bf16851a4d17f0cefedb6a73b8
generation_key=sha256:a79dd568e631ffd59e6eadcfc3ef181b06f2f199bdc67ca8856daf5f417ed495
final_generation=sha256:5b405b58bedcb87d72b7d82e14bc6a33db6bb2dc89136cb33202c2ef455c8dac
```

The generation key is exactly:

```text
sha256(UTF8("generation") || 0x00 || UTF8(request.identity) ||
       0x00 || UTF8(contract.identity))
```

## 4. Closed request inventory

The request binds the appendix’s exact ordered 47-role inventory. It includes the contract definition; Stage3 input; Stage4 policy, plan, corpus, partitions, implementation/environment, metrics, report, qualification, evidence, trust, and epoch; Stage5 composition/readiness/grouping/successor/seed/algorithm; Stage6 representative/disclosures; four Stage7A method/projection receipts and source context; five Stage7B objects; two Stage7C objects; source/workspace binding; and thirteen workflow/stage/privacy/resource/checkpoint/arbitration/replay policies.

The appendix field `request_roles` is the normative full spelling and order. Omission, duplication, reordering, conflicting role reuse, or replacement changes identity or refuses.

## 5. Stage4 test-only qualification world

Repository evidence supplies no production-qualified Stage4 grouping receipt. The draft qualification plan explicitly says it supplies none, while the causal specification permits structurally valid test-only builders and forbids them from minting production qualification.

This fixture therefore freezes a bounded `CAUSAL_TEST_ONLY` qualification packet containing all ADR0011-required fields:

- preregistered plan with replay, permutation, false-merge, fragmentation, and acceptance criteria;
- exact synthetic corpus manifest;
- fixture and held-out partitions;
- non-executable implementation and environment identities (`implementation=non-executable-fixture`, `os=none`);
- complete metric definitions/results and balanced `expected=2`, `completed=2`, `missing=0` accounting;
- report identity and explicit fixture-only uncertainty;
- fixed timestamp `2000-01-01T00:00:00Z`;
- every predecessor identity, byte length, and SHA-256;
- two exact `TEST_FIXTURE_MAINTAINER` acceptances;
- closed ADR0011 decision `QUALIFIED`;
- literal `production_admissible=false` and `scope=CAUSAL_TEST_ONLY`.

This receipt satisfies only causal case C01’s assumed positive predecessor. Production admission must reject its scope. It does not qualify a production policy, corpus, implementation, environment, provider, public surface, or historical artifact.

## 6. Stage3 → Stage7 chain

1. Stage3 admits one revision-bound `symbol:caller → symbol:target` CALLS occurrence.
2. Stage4 binds the Stage3 input, test-only policy, complete qualification packet, evidence, trust class, and epoch.
3. Stage5 composition is `AVAILABLE` and readiness is `READY` only in the test scope; grouping and seed-7 successor bind exact Stage3/4 identities.
4. Stage6 selects `symbol:target` only under CALLS representative rules and discloses `CAUSAL_TEST_ONLY` and `NOT_PRODUCTION_QUALIFIED`.
5. Stage7A binds two independent method receipts, two independent projection receipts, exact source bytes, five range classes per role, and source/workspace custody.
6. Stage7B snapshot, packet, checkpoint, replay request, and replay result bind the exact Stage7A identity and report byte-identical test reconstruction.
7. Stage7C CALLS coupling and instability bind Stage7A plus Stage7B packet/replay identities, exact successor bytes, and seed inventory.

## 7. Stage7A exact roles, ranges, receipts, and custody

### Query/reference role

Exact LF source:

```go
package fixture

func caller() {
	target()
}
```

The role independently retains:

```text
evidence_range  = [(3,1),(3,9))   // target()
item_range      = [(2,5),(2,11))  // caller
selection_range = [(3,3),(3,3))   // exact query point
 display_range  = [(2,0),(5,0))   // containing caller definition
 context_range  = [(0,0),(5,0))   // admitted file context
```

It binds method `callHierarchy/outgoingCalls`, provider, request, result ordinal, session/generation, and an independent projection receipt binding the exact document digest and `full-definition-v1` policy.

### Target role

Exact LF source:

```go
package fixture

func target() string {
	return "ok"
}
```

The role independently retains:

```text
evidence_range  = [(2,5),(2,11))
item_range      = [(2,0),(2,20))
selection_range = [(2,6),(2,6))
display_range   = [(2,0),(5,0))
context_range   = [(0,0),(5,0))
```

It binds a separate `textDocument/documentSymbol` receipt and projection receipt. Both roles separately retain logical URI, document version, revision, session, generation, position encoding, source digest, source length, and exact bytes. Range classes and roles are never collapsed or inferred from one another.

All characters before LF are ASCII, so UTF-8 byte columns and UTF-16 code-unit columns coincide. The source/workspace binding independently carries truthful source digests and lengths.

## 8. Stage7B and Stage7C

Stage7B binds every predecessor required for workspace-free replay. Its checkpoint genesis is literal `GENESIS`; replay limits are 16 objects and 8192 bytes; result is `BYTE_IDENTICAL` for the exact target packet.

Stage7C coupling is exact CALLS scope:

```text
Ca=1; Ce=1; I=Ce/(Ca+Ce)=1/2
```

Instability has one completed run, one comparison unit, zero failed runs, exact ratio `0/1`, threshold `1/10`, and `STABLE`. These values describe only the bounded fixture.

## 9. Reconstructible six-stage transformations

### Describe

Input is the exact source-bearing packet. The request freezes backend `captured-response`, backend identity, LF prompt bytes, prompt SHA-256, and policy. The result freezes LF response `Target returns the literal string "ok" to caller.`, response SHA-256, and a citation to the exact Stage7A target display custody. No model call or ambient source is required.

### Search

The corpus is exactly the Describe result. Deterministic tokenization is: ASCII lowercase; split maximal `[a-z0-9]+` runs; discard empty tokens; deduplicate; sort lexicographically. The corpus tokens are `caller,literal,ok,returns,string,target,the,to`. Applying the same rule to the exact LF query bytes `returns ok to caller\n` yields exactly `caller,ok,returns,to`. Score is reproduced as:

```text
intersection = {caller,ok,returns,to}
|query_tokens ∩ corpus_tokens| / |query_tokens| = 4/4
```

Order is descending exact rational score then member identity; the one result is `member:0`.

### Group

The request consumes only the exact Search result. Rule is `all MATCHED members sharing candidate constituent set`; output group `group:0` contains only `member:0`, constituents are caller and target, unmatched is empty, and labels are excluded from identity authority.

### Location

The request binds Group plus Stage7A. The result emits `location:0` from the retained TARGET role only, preserving document, evidence range, digest, length, version, revision, session, generation, encoding, and projection receipt. It selects no new source and repairs no relation.

### Source Text Search

The request freezes admitted target bytes and exact UTF-8 pattern `return "ok"`, pattern SHA-256, source SHA-256, Location identity, and literal non-overlap policy. Replaying byte search yields exactly one match, ordinal 0, multiplicity 1, exact byte range and UTF-16 range `[(3,1),(3,12))`.

### Candidate Generation

The request binds Group, Location, and Source Text Search result identities. The candidate’s citation is not copied directly from Stage7A: it is derived from `location:0` plus `match:0`, and explicitly names both predecessor result identities. Its URI/digest comes from Location; byte and UTF-16 citation ranges come from the STS match. Candidate constituents come from Group.

Each result has exactly one successful member, `expected=evaluated=successful=terminal=1`, and `failed=omitted=missing=0`. Each of six completion checkpoints binds its exact stage request/result and previous checkpoint.

## 10. Positive result and effects

The final state is `CANDIDATE_COMMITTED` only inside the private causal-test generation model:

```text
stage_evaluations=6
checkpoints=6
candidates=1
candidate_commits=1
private_generation_commits=1
publication=0
```

It is not production publication or implementation evidence.

## 11. Exactly one semantic mutation and complete derived resealing

The sole changed admitted semantic property is:

```text
stage7a-source-context.roles[1].source_digest
```

It changes from the SHA-256 of the exact retained target bytes to 64 zero hex digits. Source bytes, range classes, roles, receipts, workspace binding, policies, grouping, representative, and all other semantic inputs remain unchanged.

Every derived commitment that encloses or depends on Stage7A is mechanically resealed: Stage7A identity; Stage7B snapshot, target packet, checkpoint, replay request, and replay result; Stage7C coupling and instability; composition request; generation key; and source-bearing packet. These derived identity updates are not additional semantic mutations.

The truthful source/workspace binding remains unchanged. Therefore v1 admission §4 step 4—source/workspace custody and source-context occurrence binding—is the exact earliest applicable guard. It detects claimed target digest versus retained target bytes/workspace digest before private-stage evaluation:

```text
mutation_request=sha256:deb9cc3afa23960c075c33ae19550c5847df6a639e7872280761c53dc3c58f3a
failure=SOURCE_CONTEXT_INCOMPLETE
refusal=sha256:a64d5b4a7841edf50e2dcc75cf7070498e3f045ecbd369c67597d855ec378b65
failure_precedence=4
```

No member starts. Refusal accounting is:

```text
Describe, Search, Group, Location, SourceTextSearch, CandidateGeneration:
  expected=1 evaluated=0 missing=1
stage_evaluations=0
checkpoints=0
candidates=0
candidate_commits=0
private_generation_commits=0
publication=0
```

There is no Describe request/result, completion checkpoint, candidate, or final generation in the counterworld.

## 12. Failure precedence

The v1 order is retained: resource preflight; carrier parsing; digest/length/identity recomputation; source/workspace custody; predecessor chain; qualification/readiness/trust; Stage7B replay custody; representative; coupling; instability; privacy/resource/contract identity; then private stages and checkpoint/candidate commit. Exactly one earliest failure is returned; later failures cannot replace it.

## 13. Normative and non-normative boundary

Historical omitted-selector `CALLS_ONLY` behavior and its canonical bytes remain unchanged and authoritative for their historical contracts. This v2 fixture neither modifies nor reconstructs those bytes, and it does not mint, recover, or imply historical production qualification.

Normatively for this document only, v2 is `CAUSAL_TEST_ONLY` and `production_admissible=false`. The following future-production statement is non-normative planning guidance and is not a carrier or world-semantic input: any production composition requires a new additive, versioned CALLS qualification/custody regime that independently binds method/provider custody, admitted occurrences, relation-family policy, grouping qualification, source/replay custody, instability, coupling, complete accounting, and the unchanged claim ceiling. No such regime is selected or authorized here.

## 14. Claim ceiling and freeze boundary

Every positive and counterworld artifact carries or is governed by:

```text
authority=0
accepted=false
completeness=UNKNOWN
feature_identity=UNRESOLVED
```

The appendix and content-addressed copies may be refreshed only after complete checks of canonical bytes, base64, lengths, SHAs, identities, contract closure, all role links, Stage3–7 predecessors, test-only qualification fields/accounting, five distinct range classes per role, six transformations, Location+STS citation derivation, six stage equations/checkpoints, the one semantic mutation, all mutation reseals, accurate precedence, and zero effects. Passing those checks freezes documentation only and does not claim GO.
