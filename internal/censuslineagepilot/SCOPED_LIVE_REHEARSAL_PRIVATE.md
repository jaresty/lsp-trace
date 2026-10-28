# Private one-native-V5 rehearsal (test binary only)

Status: offline fake-qualified wiring, NOT live-qualified, NOT accepted. Authority
0, accepted false, completeness UNKNOWN, feature identity UNRESOLVED. Retained
revision assertions remain CALLER_ASSERTED_NOT_AUTHENTICATED and separate from the
executable checkout HEAD. No public dispatch, Program C, expiration policy, input
bundle mutation, arbitrary production-client constructor, or completion head.

## Independent review before any live invocation

The opt-in test was compiled and skipped, never executed against a live server or
real retained roots in this implementation session. Review these additions and
the exact configuration independently first. A later reviewed clean commit must
contain this wiring: the live gate rejects dirty or untracked checkout content.
No commit or push was performed here.

The runner uses existing Manager Start / BeginReadiness / WaitReadiness, then the
exact READY workspace/session/generation lease. It makes no automatic restart,
workspace derivation, substitute session, alternate seed, or retry. Initialization
is protocol traffic, not semantic acquisition. Six captured document supplies
(didOpen/didChange) are preparation traffic, not discovery. The synthetic default
fixture's actual initialize traffic predates the runner; its six preparations and
semantic requests are observed through the existing fake transport. This is not
a live traffic census or process-containment qualification.

Live process construction reuses NewLocalDarwinSupervisor. Its qualification is
LOCAL_DARWIN_SUPERVISION_ONLY, not anti-escape/owner-death containment. The test
compiles on Linux but this live starter may fail unavailable there; no alternate
starter is installed. Server executable, args and a nonempty explicit sorted
process environment come solely from config, never PATH lookup or inherited
os.Environ. The selected runtimeprofile identity is validated, not loaded through
public CLI orchestration. Startup validation first requires a successful READY
snapshot with the exact configured session/generation. It then calls
`Manager.Metadata(session,generation)` and checks ServerCommand against the
explicit path, followed by an exact-workspace READY lease check. Readiness
snapshot metadata intentionally has no ServerCommand; the Manager contract is
unchanged. The same private validator gates both live and default fake execution
before any execution claim or discovery.

## Configuration

`LSP_TRACE_SCOPE_REHEARSAL_CONFIG` must name an absolute canonical bounded JSON
file. No roots are defaulted or created. Primary, backup and coordinator must be
precreated, distinct, private, outside temporary directories, with explicit device
and inode identities. Source/config paths may be temporary; output paths may not.
Existing no-follow root admission and topology proof enforce input/output
separation, ancestry and ongoing identities. Root capabilities are owned by one
NONCONCURRENT invocation. Stores close before topology proof, which closes before
output roots; no asynchronous Close is supported.

Encode with Go `encoding/json.Marshal(ScopedRehearsalConfigPrivate)` in exact
struct field order, WITHOUT a trailing newline. The parser rejects duplicate or
unknown keys, aliases, whitespace/reordered noncanonical encodings, invalid
bounds, environment duplicates/unsorted names, and omitted process environment.
The type is test-export-only; it is not a production construction API. Fields in
canonical order:

1. `schema`: `lsp-trace.scoped-rehearsal.private.v1` (historical fixed 128-symbol
   discovery) or explicit prospective `lsp-trace.scoped-rehearsal.private.v2`
   (fixed 256-symbol discovery). All other config fields and canonical ordering
   are identical; there is no caller-settable discovery budget or policy field.
2. `input_roots`: exactly `[primary,backup,coordinator]` for the existing terminal
   retained bundle. No recursive scan or input rebuilding.
3. `restart_locator`: exact `head_key`, `head`, `scope`, `metadata` from the existing
   retained-reader locator. Each blob has `id` (raw `sha256:` ID) and `byte_length`.
4. `output_roots`: exactly `[primary,backup,coordinator]` for this attempt.
5. `output_identities`: three `{device:<uint64>,inode:<uint64>}` identities.
6. `workspace`: exact retained manifest root AND explicit Manager-registered
   workspace; no URI/path rebasing. All six current supplies must equal retained
   bytes independently. The retained revision assertion is not checkout identity.
7. `executable_head`: exact 40-lowercase-hex clean reviewed code HEAD containing
   this test. This is a local checkout assertion/check, not authenticated binary
   or server provenance.
8. `session`: `{SessionID:<string>,Generation:<uint64>}` (case matters). The exact
   fresh Manager identity is the validated runtimeprofile's
   `Resolve(selector).SessionKey().String()`, generation 1; derive the key without
   launching a server. A mismatch fails rather than discovering a replacement.
9. `trust_domain`, `profile`, `environment_reference`: explicit runtimeprofile
   selector identity. The workspace is the field above.
10. `server`: canonical absolute executable path; `args`: at most 16 strings;
    `environment`: 1–32 `NAME=value` strings, strictly sorted by unique name.
    Supply every required environment value explicitly (including PATH/HOME/etc.
    if the chosen server needs them); no ambient merge.
11. `budget`: exactly
    `{"artifact_bytes":2097152,"total_bytes":16777216,"objects":8,"staging_bytes":33554432,"read_hash_bytes":268435456}`.

The config file is capped at 32 KiB. Each path/arg/env entry is bounded; restart
input copy/read work reuses the existing finite retention-plan/reader ceilings.
The output constants are an explicit PRIVATE PROPOSAL, not public defaults or a
new general storage framework: at most eight objects, 2 MiB each, 16 MiB total,
32 MiB two-copy staging, sixteen byte-equivalent read/hash passes (256 MiB).
A 256 KiB final observation reservation prevents recursive receipt growth. These
are serialized/copy/work bounds, NOT a bound on producer/JSON heap allocation.
RawV5 is size-checked AFTER producer return, before Core admission; an oversized
raw result is refused, not truncated or repaired, and cannot be retained under
this proposal. Discovery/request serialized overflow is refused before Core
acquisition/producer semantics respectively. The existing adapter remains 30 s,
5 s/request, 256 requests, 64 messages, 2 MiB result cap, 8 MiB captured response
budget. No budgets/retries are increased to make a run pass.

### Prospective discovery policy v2 (source-qualified, live pending)

The original v1 config, claim bytes and default adapter retain the fixed 128-symbol
policy. V2 is an explicit, separate prospective choice, not repair or replay of a
spent claim. Its claim schema is `lsp-trace.scoped-execution-claim.private.v2`, with
request policy `one-native-v5-private-v2` and explicit `max_symbols:256`. V1 retains
its original claim schema and request-policy bytes (no added field). Mixed config,
claim or adapter policies, unknown versions, case aliases and numeric overrides
are refused. The one-slot coordinator selector deliberately remains unchanged:
changing policy cannot bypass a coordinator's existing spent claim.

Both versions retain four nominated files, six source guards, depth 64, maximum
63 acquisition targets, and the wire/time/request/retention bounds above. The
symbol denominator includes nested and noncallable nodes, preserving enumeration
order and duplicates; 256 is not a callable or target limit. At 257 enumerated
nodes v2 still reports the omitted node and file as incomplete, never truncates to
success. More than 63 prepared targets remains a separate refusal before Core.
The native request/seed wire family is unchanged; retention receipts bind its
configuration and policy-bearing claim source. V2 terminal observations use
`lsp-trace.scoped-rehearsal-observation.private.v2` and explicitly report effective
name, symbol/file/depth/target bounds in `discovery_policy`; v1 omits that field.

Presumption: 256 is a finite prospective budget motivated by the read-only first
attempt's 219 enumerated symbols, not evidence that all later preparations or
native acquisition will succeed within unchanged bounds. Inspected historical
causes were 91 `SYMBOL_LIMIT` ordinals 128–218 and one `CHILD_INCOMPLETE` for file
ordinal 3, with no other cause code. Revision trigger: a fresh authorized event
reveals another cause or a required budget beyond these fixed bounds. Stop and
report it rather than expanding scope, manually seeding, retrying the spent claim,
or changing historical artifacts. Generated fake tests are not new live custody.
No later output roots/configuration or live attempt is authorized by these tests;
those remain pending independent review.

After independent review, the exact command is:

```sh
LSP_TRACE_SCOPE_REHEARSAL=one-native-v5 \
LSP_TRACE_SCOPE_REHEARSAL_CONFIG=/absolute/private/config.json \
go test ./internal/censuslineagepilot \
  -run '^TestScopedBatchLiveConstituentRehearsalPrivate$' -count=1 -v -timeout=90s
```

Absent sentinel skips. Every other nonempty sentinel rejects. Run only this
named test when opted in: the older `retain-inputs` rehearsal has a different
sentinel contract and is deliberately not invoked by this command.

## Prospective terminal observation v3 (private diagnostic wiring)

Future execution with the unchanged v2 configuration/resource policy writes
`lsp-trace.scoped-rehearsal-observation.private.v3`. The v2 observation description
above documents the earlier encoding; existing v2 artifacts are not reinterpreted
or rewritten. No historical outcome reader/parser is installed by this bridge.
V1 still emits its original fieldset and schema, with no diagnostics field.

The existing terminal outcome object gains `diagnostics` containing exactly
`available`, `events`, and `overflow`. Events are detached Manager-owned closed
observations (`p` phase, `m` method, `o` ordinal, `u` URI class, `r` reason), not
caller inputs. The existing Manager cap is 64 events and 8000 event bytes including
separators; array/envelope overhead keeps the complete diagnostics value below
8192 bytes. There is no raw URI, source content, credential, capability, seed, or
custody claim. Existing output, total-byte and eight-object budgets are unchanged;
no ninth object is allocated. Public failure text/error chains remain separate.

A deferred snapshot inside the READY lease callback runs before callback return,
including Bind and producer failure paths, and before store/proof close. Identity
validation permits observing a frozen-state mismatch without running another
Check. Missing adapter/guard or rejected capability identity is explicitly
unavailable; a valid empty snapshot is available with zero events. Empty is not
exoneration and overflow explicitly means a discarded prefix. `CHECK`/`NONE`
observations do not imply a wire request; `ADMISSION` observations do not prove
request effects or server analysis. Successful admissions may also be events.

Local fake-wire fixtures qualify this transfer and retained primary/backup hashes,
not either spent attempt's 25-target DTO or the unknown actual live cause. No live
run is authorized: independent review, a clean commit and a fresh config/root are
still prerequisites. Neither historical attempt may be replayed.

## Claim, acquisition and retention

The topology-backed execution store occupies a separate coordinator namespace.
The owner attempts one absent-predecessor CAS before Discover (and before adapter
construction in this wiring). Only its exact pending pointer with still-valid
topology can discover/mint; existing heads, uncertain attempts, lost/copied pending
and invalidated proof never authorize semantic traffic or replay. The head stays
CLAIMED throughout and afterward. There is no finalize/reset/second CAS.

Actual static-only scoped discovery supplies Core's existing canonical seeds.
The wrapper refuses >63 targets before Core; Core plans one depth-1/1 batch with
max 63. The acquirer refuses a second Acquire, mints the exact manifest/input
binding through the retained adapter, and calls the actual closed scoped producer.
One Core-admitted constituent requires the exact returned RawV5; no graph-envelope
repair occurs. Successful empty closure is explicitly EMPTY without acquisition
(`Complete` alone is false for zero targets in the existing discoverer). Unknown
closure, omission, resource failure and malformed responses are not EMPTY.

The fixed record set is configuration, exact input head/scope/metadata/receipt
links, claim head/CAS observation, discovery accounting, canonical request/seeds,
exact producer RawV5 when returned within cap, Core manifest when admitted, and
one immutable outcome. Empty/failure paths retain the available subset. Every
record uses the existing independent primary/backup pair retention with close-
aware exact read/hash verification. No rollback/delete/readback promotion occurs.
A primary success survives backup/read/hash/close failure with its partial receipt.

The outcome links previously available record receipts and the claim, and is
retained after execution-store/proof close so those failures can be included.
Its own pair receipt appears only in the native report, outside the immutable
object. Output-root close and runtime shutdown uncertainty is reported separately
by the native test after outcome retention; the immutable record cannot promise
those later closes succeeded. Failure persistence itself may fail: the report is
then INDETERMINATE, never a guarantee of durable failure evidence. An immutable
observation is not an execution-completion head or acceptance decision.

## Offline qualification and limits

Default tests run without the sentinel or real roots. New tests cover sentinel
and canonical parser/bounds, workspace/root/budget refusal, lost/copied pending,
existing/uncertain attempted claim refusal, all six supply drifts before discovery,
EMPTY versus incomplete, 63/64 policy boundary, actual producer/Core one-call
admission, per-artifact overflow, primary-only/backup read/hash/close partial
receipts, input-tree snapshot equality, and store/proof/output close ordering.

Existing matrices remain the authority for full claim CAS/receipt ambiguity,
store binding, topology ancestor/replacement/lifetime faults, adapter wire budget
and supply equality, canonical seed/manifest binding, closed producer lifecycle,
native V5 admission, retention pair durability and legacy root guards. New wiring
tests reuse those seams rather than duplicating every matrix.

Startup regression: the actual fake Manager initializes with `/fixture/server`.
The test first confirms empty readiness ServerCommand and exact-generation
Metadata's configured command. With the old readiness-command comparison retained
in the extracted helper, `TestScopedLiveRehearsalStartupValidationPrivate/exact-server`
failed at `ASSERT_STARTUP_EXACT_METADATA_ACCEPTED` (not compilation). Removing only
that comparison, while retaining the exact-generation Metadata/server check, made
the same regression pass. Wrong server, stale/missing metadata, wrong workspace,
wrong snapshot identity/generation, non-READY and unavailable snapshots refuse;
execution-gate tests verify no claim, preparation, discovery or Acquire on invalid
startup. This is offline regression qualification, not live startup qualification.
Exact live configuration still requires independent inspection before invocation.

Not exercised: live sentinel; real retained roots; real server initialization,
real 30-second lifetime exhaustion, OS crash/power loss, disk exhaustion, physical
volume failure independence, hostile process containment, authenticated revisions,
production dispatch or Program C. Policy boundary tests do not claim a live
63-target workload. No exhaustive assertion-adequacy or independent acceptance
claim is made. Later Program C/DescribePending work remains separate.
