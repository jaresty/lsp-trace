# Private terminal discovery accounting (V2 opt-in)

This is private engineering accounting, not an application inventory qualification.
Authority remains 0; accepted false; feature identity UNRESOLVED; source
completeness UNKNOWN; absence inference PROHIBITED. There is no new model,
provider, ordinary Native, Describe, workflow, or public enablement.

## Owned producer

`DiscoverScopeOutcomePrivateV2` is additive. `DiscoverScopePrivateV2` retains its
old return/error behavior, including its early incomplete check on successful
zero-target Census closure. Neither API retries a frozen owner's discovery.

Only the original discovery attempt can mint `TerminalDiscoveryOutcomePrivateV2`:
its original frozen source, complete original ledgers, exact policy, concrete
Manager, retained guard, session identity and original active lease are bound.
There is no JSON/DTO constructor, retrospective report classifier or restored
owner. The outcome is not an acquired `EnumerationOriginPrivateV2`.

Two reasons are permitted:
- `TERMINAL_RESOURCE_GAP_TARGET_CAP`: complete successful original discovery with
  more targets than the unchanged effective cap (at most 63).
- `TERMINAL_NO_PREPARABLE_TARGETS`: successful original closure, no targets, and
  every enumerated symbol explicitly non-callable. Census keeps `Complete=false`
  for this case; complete ledger closure is checked separately. Its six UNKNOWN
  observation dimensions are preserved, never interpreted as source completeness.

Incomplete discovery, source omissions, provider failures, timeout, source/guard/
runtime drift and invalid ledgers are errors, not terminal resource gaps.
No acquisition planning is run for either terminal reason. Counts are never
truncated or inferred from Go AST declarations.

## One-shot terminal retention

`RetainTerminalDiscoveryPrivateV2` consumes an irreversible retention attempt
while the exact original lease is alive. It validates original source and root
identity, revalidates the original adapter, retains two create-only copies,
requires exact readback and physically distinct objects, rechecks topology and
source, and returns detached receipts. The separate fixed accounting publication
family is bounded to 1 MiB. Retention failure is not retried and mints no Native
seal. `InspectTerminalDiscoveryPrivateV2` performs closed canonical full-record
validation and source/ledger correspondence; it never restores the owner.
Detached inspection enforces selected Accounting.Files for every nominated file
and the per-file sum of retained symbol-name UTF-8 bytes against the exact bound
in EffectivePolicy.Discovery.MaxStringBytes. This is a necessary **lower-bound**
check only. Original discovery enforces Name+Detail byte sums and tree MaxDepth,
but this detached projection does not retain Detail or the original symbol tree.
Those discarded dimensions are **UNVERIFIED by detached inspection**; valid policy
bounds are not proof of full observation compliance. Original owner minting still
requires original once-only bounded discovery; no DTO can recreate that owner.
Final terminal custody uses its own fixed 1 MiB descriptor-relative nonblocking,
no-follow regular/private-file observer, checks named/opened identity and physical
separation, and propagates close/context errors. This closes the local FIFO-open
hang, not arbitrary filesystem latency, a namespace lease or an NFS guarantee.

## Application CLI profile and exit contract

`Application.Profile = PRIVATE_CENSUS_APPLICATION_TERMINAL_V2` explicitly opts in.
The existing V1 profile and nil Application keep their existing semantics. The
new profile is in config, full-source-plan and scope hashes; normal disclosure
approval pins that scope exactly. Terminal-only accounting requires no model
approval claim because it performs no semantic disclosure.

A successful terminal segment exits 0 only with status
`TERMINAL_ACCOUNTED_GAP_NOT_NATIVE_DELIVERY`, retained source-bound terminal
pairs and successful original Stop/Shutdown/Join cleanup. It is NOT Native
delivery and never `PROVISIONAL_COMPLETE`. The next invocation advances only
through the exact contiguous claim/result/outer-host-observation prefix. The
host observation must record actual Wait/exit 0, Joined true and Stopped false;
its trust remains SAME_USER_HOST_ATTESTED_NOT_AUTHENTICATED, not authentication.
Every attempted new-profile segment claims before runtime startup, even without
Automation, so timeout cannot erase its spent identity. A normal segment must
already carry exact semantic approval. The hidden ordinary CLI operation
`--private-inventory-scope-preview` requires this profile and nil Automation. It
exports `scope.json` and its digest in CLI metadata with status
`SCOPE_PREVIEW_METADATA_ONLY_NOT_GENERATION`, before claim/startup. Repeating it
is nonspending: no generation, provider, session, model or approval is minted.
It may install/read the full frozen source metadata plan but never stores its
scope digest inside that hashed plan (avoiding circular hashes). Only this
operation selector is omitted from the execution-controls hash; all source,
configuration, runtime, policy and other control pins remain exact. Execution
rechecks those pins and the contiguous verified prefix before claim/startup.
The host obtains canonical scope bytes from this CLI artifact, not a private Go
hash helper or reconstructed owner. Preview is not preparation or execution.
Normal discovery without that approval is a spent failure, not a retry path.
A failed attempted generation exits 1; a missing, failed, malformed or unverified
host/terminal record hard-stops progression.

Mixed-segment contract: retain complete producer pairs for computed origins,
label them `COMPUTED_NOT_NATIVE_DELIVERED_SEGMENT_GAP`, and account the entire
selected segment as a gap. Their computation is explicitly disposed, not
silently discarded. No Native pair, preparation, candidates or semantic
interpretation is manufactured for that segment. Per-origin accounting retains
the exact reason. Target partitioning remains deferred.

Accounting distinguishes COMPLETED_NATIVE and TERMINAL_ACCOUNTED_GAP. Terminal
files remain in GapFiles, never CompletedFiles; TerminalAccountedTargets are not
NativePreparedTargets. Future files remain unattempted, and excluded files stay
excluded. A final accounting-complete host-attested status does not mean Native
application delivery or feature acceptance. No cross-segment global CALLS graph,
feature identity or independence claim is made.

## Verification boundary

Owned protocol fixtures execute actual original 84-target discovery (within the
256-symbol ledger bound), an eight-noncallable segment, and a subsequent normal
segment. Separate actual CLI process Wait receipts, source-bound terminal pairs,
normal Native pairs, post-gap failure/spent receipts, mixed-segment disposition,
and closed-record refusal tests are retained privately. Fixtures author model
roles through the existing file transport; no external models run.

No actual application LSP or model inventory has been run by this increment.
Actual source/disclosure approval is still required. Existing acquisition,
preparation, file/origin/root, response and 240-second operation ceilings are
unchanged. The fixed process proof executes three delivery invocations
(cap, noncallables, approved normal), an additional NONDELIVERY scope-preview
CLI process before normal approval, and final read-only verification. Mixed-gap
advancement now runs the complete existing pure producer-packet inspector over
all retained batches, source/Native artifacts, roles, runtime and seeds, then
binds its policy and every batch to the original report before the terminal return.
Pair inspection requires the sole basename to equal the receipt's exact digest;
renamed objects cannot satisfy an absent receipt-selected object.
