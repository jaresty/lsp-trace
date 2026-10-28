# Private partial-inspection rehearsal (test binary only)

Status: deterministic fake-wire qualification only. NOT live-qualified, NOT
accepted. Stage-1 approval is not live authorization. No server, operational
configuration, retained-root preparation, live sentinel, commit or push was run
for this change. Independent wiring/configuration review must precede a separate
parent-user authorization boundary. A future reviewed clean commit containing
this wiring is required; the starting HEAD is not the future executable pin.

Prospective budget successor: [SCOPED_PARTIAL_BUDGET_V2_PRIVATE.md](SCOPED_PARTIAL_BUDGET_V2_PRIVATE.md)
adds config v4 / partial policy v2 with an explicit fixed 4 MiB serialized ceiling.
The v3 template and 2 MiB policy below remain historical-version semantics, not
silently increased defaults; the successor does not authorize another live run.

## Closed purpose and configuration

Use the exact canonical v2 configuration layout documented in
[SCOPED_LIVE_REHEARSAL_PRIVATE.md](SCOPED_LIVE_REHEARSAL_PRIVATE.md), but with schema
`lsp-trace.scoped-rehearsal.private.v3`. This selects only
`one-partial-inspection-private-v1`: 256 enumerated symbols, four nomination files,
six frozen source supplies, depth 64, at most 63 targets, down/up 1/1. V1/v2 bytes,
policies and defaults are unchanged. The old full-constituent driver refuses v3/v4;
the partial driver accepts only v3/v4 and refuses v1/v2 and mismatched config/claim bindings.

The policy-bearing claim reuses the strict v2 claim layout. Its distinct policy
name changes the bound claim digest; the topology proof/store selector remains
one slot across policies. Existing or uncertain claims, copied/lost pending
ownership and reused roots cannot authorize another semantic attempt. No replay,
retry or pending recovery is provided.

This is a NONCONCURRENT invocation. All exact input pins/workspace checks,
output device/inode pins, no-follow topology preflight, finite budget checks and
root separation precede startup. The runtimeprofile selector and server absolute
path, args and sorted process environment are explicit; there are no ambient
profile/server defaults or environment inheritance. Startup validates exact READY
session/generation, command metadata and workspace lease. Six document supplies
must match independently before semantic discovery. Claim CAS precedes semantics.

### Non-executable template (do not instantiate without the later boundaries)

Canonical encoding uses Go encoding/json.Marshal in the existing struct order,
without whitespace or trailing newline. The following is a field checklist, not
a JSON configuration or permission to create roots:

```
schema = lsp-trace.scoped-rehearsal.private.v3
input_roots = [<exact existing primary>, <exact existing backup>, <exact existing coordinator>]
restart_locator = {head_key, head:{id,byte_length}, scope:{id,byte_length}, metadata:{id,byte_length}}
output_roots = [<fresh private primary>, <fresh private backup>, <fresh private coordinator>]
output_identities = [{device,inode}, {device,inode}, {device,inode}]
workspace = <exact retained workspace, no rebasing>
executable_head = <FUTURE independently reviewed clean 40-hex commit containing this wiring>
session = {SessionID:<exact resolved selector key>, Generation:1}
trust_domain = <explicit>; profile = <explicit>; environment_reference = <explicit>
server = <canonical absolute executable>; args = <explicit bounded list>
environment = <explicit nonempty sorted unique NAME=value list>
budget = {artifact_bytes:2097152,total_bytes:16777216,objects:8,staging_bytes:33554432,read_hash_bytes:268435456}
```

The config remains capped at 32 KiB. No actual configuration or roots accompany
this document. After independent review AND explicit parent-user live authority,
a future operator could use this finite single-test command (not run here):

```sh
LSP_TRACE_PARTIAL_INSPECTION_REHEARSAL=one-partial-inspection \
LSP_TRACE_PARTIAL_INSPECTION_REHEARSAL_CONFIG=/absolute/private/reviewed-config.json \
go test ./internal/censuslineagepilot \
  -run '^TestScopedPartialLiveInspectionRehearsalPrivate$' -count=1 -v -timeout=90s
```

The separate sentinel avoids older tests' incompatible sentinel contracts. Absent
skips; every other nonempty value rejects. Do not run an all-tests command with
this sentinel set. Linux compilation does not qualify the existing Darwin-only
supervision implementation; no alternate starter is introduced.

## Pipeline and finite objects

1. Exact retained reconstruction and root/policy checks.
2. Exact runtime lease, one absent-predecessor claim CAS, six frozen supplies.
3. Scoped discovery; incomplete/resource refusals are never EMPTY. No truncation
   to fit 63 targets. Zero successfully discovered targets is explicitly EMPTY.
4. Core.PlanDiscovery reuses Core's canonical target/seed/partition/identity
   helpers, but never acquires/admit constituents. Its equality guard compares
   its planned batch with Core.Run's actual acquisition argument.
5. Mint the existing Manager-bound guard/selection, seed authority and real
   reservation, then call AcquireScopedPartialInspectionPrivate once.
6. Parse the closed private source-withholding carrier before classifying rows.
   READABLE and deliberate POLICY_WITHHELD are distinct from lexical zero-read
   source boundaries: OUTSIDE_SCOPE, VIRTUAL_URI and INVALID_URI are accepted
   only when motivated by RelativeURI for that validated row. They are not
   fabricated readable/withheld sources. Ordinary UNREADABLE, MISSING,
   NONREGULAR, BYTE_LIMIT_EXCEEDED, CANCELLED and BUDGET_EXCEEDED still refuse
   a successful artifact. Source alias refusal follows genuine reservation release.
   Its typed diagnostic contains only the first failing closed status, zero-based
   capture row ordinal and all-row status counts; never URI/source/parameters.
7. Existing inspection.ProjectAllSeeds + accounting validation and
   filter.ProjectPairwise compare the first two original canonical target labels
   (self-pair for a single target). Context-only nodes never nominate targets.
   CALLS partitions may legitimately be empty and establish no feature identity.
8. Retain exact carrier and views with visible coverage; snapshot bounded Manager
   diagnostics before lease exit; close store before proof; retain terminal
   outcome before later root close and runtime shutdown.

Successful record plan (exactly eight objects, no recursive outcome receipt):
configuration, input-observation, claim, discovery, canonical-request-seeds,
bounded-partial-artifact, inspection-filter-views, outcome. Failed attempts retain
available bounded pieces, including source-failure-observation metadata when
closed capture classification fails (not the raw carrier or successful partial
artifact). This replaces an otherwise absent artifact slot, never increases the
object budget. Primary-only/uncertain receipts remain visible and cannot be rolled back. No retry follows a partial publication.

Limits unchanged: 2 MiB/object, 16 MiB/8 objects total, 32 MiB two-copy staging,
256 MiB read/hash work and 256 KiB terminal reserve. Request/discovery/view
serialization caps are checked before write; artifact cap is checked after
producer return and is NOT a producer heap-bound claim. Adapter lifetime 30 s,
request 5 s, 256 requests, 64 messages, 2 MiB response and 8 MiB captured responses.
The private acquisition manifest is explicitly 25 s total / 5 s request, beneath
the fixed lifetime (also accommodates race instrumentation); no public default
changes. Diagnostics retain the existing 64-event/byte-bound overflow semantics,
including the first-refusal measurements; their collection never replaces the
original operation failure. Withholding is counted separately from ordinary
readable captures, not as evidence of an external query or a fabricated response.

## Terminal schema and evidence ceiling

`lsp-trace.scoped-partial-inspection-observation.private.v1` embeds the existing
report fields and adds `purpose:PARTIAL_INSPECTION`, `core_admission:NOT_ATTEMPTED`,
`source_completeness:UNKNOWN`, `canonical_complete` and `policy_withheld_count`.
The optional `source_disposition_counts` observation histogram counts each
validated capture status separately, including OUTSIDE_SCOPE versus
POLICY_WITHHELD versus READABLE. There is no closed observation wire parser;
this does not change the closed private source carrier or old source schemas.
`outcome` is PARTIAL_INSPECTION_RETAINED only when acquisition, views and paired
retention succeed; EMPTY requires successful zero discovery; failures remain
INDETERMINATE with failure/receipt details. core_admitted_constituents is always
0. Authority remains 0, accepted false, feature identity UNRESOLVED, revision
CALLER_ASSERTED_NOT_AUTHENTICATED. A boundary leaves canonical_complete false.
Store/proof close failures enter the immutable outcome; later root-close and
shutdown native errors are separate public test failures, not retroactive edits.

No full Census, native-V5 admission, source closure, public CLI, Program C or
replay claim is made. Inspection/filter serialization preserves their own global
boundary and requested/observed accounting; no two-target feature identity is
inferred.

## Deterministic qualification scope

The existing reviewed exact-25 table is reused, not copied. Fresh padded fixture
source and noncallable filler symbols preserve its sparse historical ordinals:
219 enumerated nodes; four documentSymbol requests, 25 discovery prepares and 75
acquisition requests = 104 semantic requests. Six real Manager supplies; 25
canonical targets across all four nomination files; external B is never queried,
context-only C is queried only as the existing incoming start, never nominated;
no incoming reseed of root A. Artifact has CALLS=3, frontier=2, readable captures=5,
POLICY_WITHHELD=1 (six total captures). All eight objects are read/hash-verified
from both fresh roots. Existing source alias, resource, claim, supply, topology,
retention and diagnostics contracts remain separate qualification surfaces.

The additional outside-workspace endpoint fixture uses a lexical URI only:
no external file is created. The actual closed producer, parser, existing views
and paired retention run with five readable captures, one OUTSIDE_SCOPE,
zero POLICY_WITHHELD, five capture attempts, and unchanged 104 semantic requests
and six supplies. Source completeness stays UNKNOWN; canonical graph incomplete.
The graph-provenance injected-reader test separately observes zero outside reads.

## Historical live failure boundary

The previously authorized single live attempt is spent and must not be rerun.
Its immutable generic failure (`partial inspection ordinary source capture failed`)
did not retain a closed capture status. Historical cause remains CAUSE_UNKNOWN;
this deterministic OUTSIDE_SCOPE counterexample does not identify that cause.
Nothing here modifies the historical output or authorizes retry, new roots,
configuration changes, server execution or a larger budget.

Fake/race/vet success is not future live qualification. Independent review and
parent commit remain pending; only the parent may decide to commit this work.
