# Exact retained partial inspection qualification (private)

`NewExactRetainedPartialQualificationInput` and
`QualifyRetainedPartialNativePrivate` implement one offline, bounded inspection
qualification, not a workflow driver, replay verifier, journal, native adapter or
Program C execution path.

## Input and custody boundary

Supply canonical JSON `RetainedPartialQualificationPinsPrivate`, three borrowed
original input retention capabilities and the two borrowed output capabilities.
The roots must remain open and must not be used/closed concurrently. No source
checkout, server, endpoint or pathname from configuration is opened. Configuration
input-root strings are comparison labels mapped to the supplied capabilities.
All five observed directory identities must differ.

The eight exact ordered records are configuration, input-observation, claim,
discovery, canonical-request-seeds, bounded-partial-artifact,
inspection-filter-views and outcome. The outcome names the preceding seven; its
own external reference is explicit because embedding its receipt in itself would
be recursive. Both actual copies of every object are bounded-read, hashed and
compared. IDs alone are insufficient. Retained publication receipts are checked
as observations, not authenticated producer identity.

Configuration uses the same `ValidateScopedRehearsalConfigPurePrivate` contract
as the producer: fixed schema/budgets, canonical path syntax, selector validation,
sorted unique environment names and exact argument/environment bounds. The pure
helper neither opens paths nor resolves profiles/executables or constructs a
Manager/capability. The producer alone retains its outside-temporary-root admission
rule. Retained `Runtime.ServerCommand` must equal configuration `Server` exactly;
provider-version metadata is not promoted to authenticated identity.

The existing retained-scope reader reconstructs both copies of the original input
objects, scope, selection and partition plan and verifies the terminal linked
receipt prefix. Its exact observation must equal the retained input-observation.
The qualifier does not reconstruct a runtime guard or pending/spent claim. The
retained CLAIMED observation is cross-bound to configuration, input pins, output
identities and the fixed private V2 policy using existing claim/CAS validators.
No coordinator head is advanced, reset, consumed or recovered.

Existing planning APIs recompute the one 25-target batch and seed bytes from
validated discovery. Exact historical labels, pathnames, target order and positions
are additionally checked against the retained canonical seed digest. The actual
private source parser validates source content digests, frozen query membership,
receipts, requests, graph references and acquisition accounting. Its six source
policy members must equal the retained scope's six file hashes and roles.
`inspection.ProjectAllSeeds`, `ValidateAllSeedAccounting` and
`filter.ProjectPairwise` recompute the views; canonical bytes must match, not just
stored hashes. Incomplete graph scope, request/frontier counts and source receipt
classifications remain visible in the report.

## Immutable identity and bounds

The constructor rejects unknown, duplicate, differently cased, missing,
noncanonical and wrong-version fields. Pins have value-only storage. The sealed
result exposes only cloned `ReportBytes`; zero/decoded values cannot provide a
runtime client, native receipt or transition capability. Copies of an inspection
result remain inspection results. Repeated verification is allowed: there is no
acquisition or one-shot consumption.

Each object is capped at 4 MiB, configuration at 32 KiB, outcome at 256 KiB,
aggregate at 16 MiB, read reservations at 128 MiB, and byte-equivalent work at
2 GiB. Caller budgets can only restrict these ceilings. Before reading, work
reserves the existing original-input 512 MiB allowance plus 128 times all eight
output byte lengths; read requests/receipt/head allowances are charged before IO.
This is invocation-local accounting, not a global resource or CPU/heap guarantee.
A 30-second context bounds work cooperatively. The report is capped at 32 KiB.
Qualification identity hashes the exact pins, code/policy assertions, explicit
budgets and five observed root identities.

`caller_asserted_code_head` is checked against retained configuration but is not
Git/binary authentication. `verifier_source_sha256` is a compiled, test-checked
source-digest proposal for `retained_partial_qualification_private.go`, explicitly
NOT an authenticated running-binary assertion or a digest of the whole dependency
closure. Policy digest is a separate explicit pin. Refreshing either requires
review; a pin does not confer authority.

## Productive result and exact native disposition

A positive result is
`INSPECTION_QUALIFIED_NATIVE_BLOCKED_NATIVE_CUSTODY`, with authority 0,
accepted false, source completeness UNKNOWN, feature identity UNRESOLVED,
partial true and canonical-complete false. It qualifies the retained source and
structural representation for inspection only. Missing native obligations are:

1. `REQUIRED_NATIVE_GRAPH_V5`: the original retained private acquisition graph is
   not an ordinary producer-owned native V5 constituent.
2. `ORDINARY_OR_APPROVED_PRIVATE_SOURCE_CARRIER`: the validated withholding profile
   is not ordinary provenance V2; there is no approved active private native source
   carrier contract. In particular, ReplayAdmitted remains false.
3. `NATIVE_INPUT_MANIFEST_SCOPE_POLICY_BINDING`: retained inspection request/scope
   bindings do not mint the native manifest/scope-policy producer capability.
4. `VERIFIED_NATIVE_PUBLICATION_AND_PLAN_SOURCE`: retained content receipts are not
   verified native constituent publication or an authoritative Program C plan source.

These are related custody obligations, not four independent architecture projects.
The next positive evidence step after re-review is read-only qualification of the
actual retained milestone, not another coordinator.

Partial graph scope is an intentional fact, not itself a fabricated missing
obligation or an EMPTY/resource-blocked graph. No graph merge or purported native
V5 is assembled. Existing ordinary native validators are unchanged.

The next small source seam is an explicitly reviewed private source-carrier
contract at the typed native producer boundary, with original-versus-derived
lineage and every required origin preserved. It remains inactive; this unit does
not approve it. Native publication and whole-partition-complete plus availability
families are still needed for authoritative `ProgramCTransitionPlanSource`.
Inventory workflow/successor `CALLS_ONLY_FROZEN_CENSUS_V1` and strict Describe
source/occurrence bindings remain downstream of actual Program C completion.

## Private preallocation boundary

Only the private source-withholding token scan is tightened. It refuses unknown
policy/member keys and case aliases of critical source/evidence array keys before
typed decoding. Canonical `members` is limited to six. Opaque LSP `data` remains
opaque. Tests distinguish six/seven members, `Members` aliases including a bounded
20,000-element payload, and canonical unknown fields. The allocation comparison
measures the private scan's early alias refusal, not a constant-allocation claim
for the entire parser or a new global resource proof. Historical V2 parsing and
wire contracts are unchanged.

## Executable coverage

Default tests produce a fresh owned exact-25 V4-profile retained pair through the
existing fixture, then verify it independently with no runtime traffic during
qualification. They exercise all eight backup corruptions; repinned semantic
configuration, claim, discovery, request/seed, private source and view mutations;
outcome authority/budget/reference/classification drift; input canonicalization,
code/policy/source pins, finite budgets, cancellation, root substitution,
determinism, detached output and absent client capabilities. An independent
ordinary V5 test supplies a genuine existing native graph fixture with a private
source descriptor and confirms refusal.

No historical milestone root is read by default, no historical acquisition is
replayed, and this change does not claim the user's historical milestone was
qualified. No live run, real-root write, commit, push or Program C execution is
part of these tests.
