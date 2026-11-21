# Private preparation-intent enumeration V1

This internal operation funds enumeration before a frozen discovery file plan
exists. It adapts the existing resource kernel and
`censusacquisition.EnumerateWorkspaceContext`; it is not a replacement census
controller. It is deliberately not connected to the normal census entry yet.

## Admission and sequence

1. The caller forms a `PrivatePreparationIntentV1`: schema, absolute lexically
   canonical source root, sorted unique lexical roots/filter/pruning lists,
   positive finite `source.Limits`, exact attempt hash and absolute deadline.
   No future source-content digest, Git classification or execution identity
   is required. Identity calculation is pure; deadline freshness is checked by
   execution admission. Canonical identity normalizes the deadline to UTC.
2. An independently caller-authorized job must already exist under the held
   private accounting root. Its scope equals
   `PrivatePreparationEnumerationScopeV1()`, its SourceAuthorization equals
   `PrivatePreparationIntentIdentityV1(intent)`, and its ResourceRules equal
   `PrivatePreparationEnumerationRulesV1(intent)`. This operation does not
   initialize or renew jobs, authorizations, decisions or grants.
3. `OpenPrivatePreparationEnumeratorV1` reopens the expected verified READY
   chain. `Enumerate` requires the exact intent and attempt and a fresh,
   uncharged job. Opening a handle, parsing a head or obtaining a quote is not
   an enumeration permit.
4. The one-use owner path reserves the complete logical quote using the
   existing kernel. A successful reservation **and independent verified
   PENDING-chain readback** must precede the call to the existing enumerator.
   Copied handles share the burned state; independently reopened handles are
   constrained by the same persisted kernel chain and CAS/replay rules.
5. Successful enumeration completes OBSERVED. An ordinary enumeration/limit
   refusal attempts FAILED completion without refund. Cancellation or
   accounting uncertainty can leave a spent PENDING or committed head. There
   is no retry, renewed deadline, grant reuse or data release on uncertainty.
6. Only verified completion readback releases `PrivatePreparationEnumerationResultV1`.
   `Files()` returns detached string-only SourceFile records. Facts retain
   caller-asserted/not-authenticated assurance, Authority=0, Accepted=false and
   UNKNOWN completeness. This is not a native result, source identity,
   execution permission, Git observation or whole-tree completeness claim.

## Limits and accounting

The quote covers bounded metadata representations, URI/JSON escaping, detached
copies, logical traversal/sorting work and bounded kernel verification and
publication/readback. Its fourteen currencies use the existing kernel's
logical accounting model, not measured OS CPU/RAM/IO or monetary accounting.
Provider request/message/request-byte/response-byte/wire currencies are zero.
No source content read/hash or provider/native/HOST call occurs.

Hard admission maxima are 64 MiB logical bytes, 4 Gi logical work units, a
15-second remaining window, verification 128 records / 4 MiB / 128 work,
1,000,000 discovered records, 2^26 walker work, 4096 path bytes, depth 64,
64 entries per lexical list and 16 KiB pre-encoding intent material. The quote
can refuse substantially smaller combinations when their conservative
metadata/work envelope exceeds the admitted limits. These are enumeration
limits; existing native 12-second-active + 3-second-cleanup behavior is not
changed.

The enumerator remains cooperative and metadata-only, not an OS sandbox or
an atomic filesystem snapshot. Its existing roots/pruned-directories govern
walking; includes/excludes govern its accepted-file counter and later file
selection, not a promise never to observe other names under authorized roots.
Existing unavailable/unsupported file metadata is preserved. The returned
in-memory plan data is not a durable file-plan publication or fresh-process
recovery mechanism. Only the resource-accounting chain is durably read back
in this slice.

## Deliberately remaining work

- Structured URI/digest export from already-paid discovery hashes; enumeration
  here supplies no source digest and cannot form the final content identity.
- Frozen-plan discovery admission and fresh identity construction.
- Close preparation before human confirmation; separately authorize and admit
  subsequent execution. Never reuse this window or metadata as native custody.
- Normal command/host integration, discovery revision/header limits, aggregate
  catalog handoff, fresh-process recovery and release/security qualification.

Default tests cover intent/attempt binding, all nonzero quote currencies,
reservation-before-enumeration, copied/reopened and concurrent handles,
failure conservation against a captured committed debit, CAS/readback
uncertainty, finite bounds, escaping and detached result metadata.
