# Finite variable source withholding — private V2

This profile is not ordinary graph-provenance V2, schema admission, analyzed
source, full-definition evidence, replay admission, or an acquisition grant.
`EvidenceV2` descriptor reuse does not imply wire-schema compatibility.

Only `BindSourcePrivateV2` can create a bound source capability, using the exact
Manager-owned V2 guard under a current READY lease and exact workspace identity.
Copies of the capability or guard are invalid; pointer aliases preserve identity.
The guard's detached expectations are the only source policy input. The bound
query declaration derives from that same sorted member list and capture requires
exact V2 query equality before file IO. V1 declarations cannot substitute.

Policy membership is 1..8 nominations, 0..8 context-only members, at most 16 total.
Session identifiers are bounded to 1024 bytes, workspace/member URIs to 8192 bytes,
and policy metadata to 128 KiB aggregate. Hashes are lowercase `sha256:` plus 64
hex digits. Metadata count and length preflight precedes typed member allocation.
The existing source limits remain 1 MiB/file, 4 MiB capture bytes, 64 attempts,
and a new prospective private V2 4 MiB artifact ceiling (including the emitted
newline). This is not inherited from `MaxEnvelopeBytesV2`: the ordinary and V1
carrier limit remains 192 MiB. Guard admission's separate 8 MiB expectation-sum
bound does not enlarge source capture or artifact limits. The parser checks the
input length before typed allocation; this is a length gate, not a full allocation
meter. Exactly 4 MiB padded valid input is permitted; 4 MiB plus one is refused.

The shared no-follow reader refuses symlink components. Workspace-local URIs
outside policy receive metadata-only `POLICY_WITHHELD` receipts with a private V2
digest namespace and no file reads. Outside-workspace receipts remain outside
scope. Readable bytes must equal bound expectations and policy hashes; drift is
an error, not a source receipt. Budget and unreadability remain honest gaps.
V2 cancellation observed at entry, before encoding, or after shared capture returns
refuses output with the context error; shared intermediate CANCELLED records are
not published by V2. Root-close always runs and its errors are joined. The final
check is a defined late-cancellation stop point, not a universal cancellation or
power-failure atomicity guarantee. Historical V1 cancellation-gap behavior remains
unchanged. Root-close failure clears output bytes.

`ParseSourceWithholdingPrivateV2` is the sole private wire entry. Strict bounded
preflight and canonical typed re-encoding precede validation; decode reconstructs
restrict-only query metadata, never a guard, bound source, or execution capability.
Root and RequiredTargets must name NOMINATION members in this prospective V2
source profile, before capture IO and on parse. CONTEXT_ONLY nodes may enter and
be queried through relationship frontiers, but cannot be nominated as seeds.
This does not add authority to pure acquisition declarations or change V1 role
semantics. The V1 parser, ordinary V2 validator, and ordinary schema deliberately
reject this profile. Native graph digest behavior, Census, supply projection, ordered metadata,
and V1 withholding namespaces remain shared or unchanged.

Owned synthetic fixtures cover 1/0, 2/1, 5/2, and 8/8 policy sizes. The fake-pipe
Manager fixtures cover 1/0, 2/1, and 5/2 and exercise actual V2-attached RoundTrip
calls for nomination seeds only; context sources arrive through incoming edges.
Their readable source and AST-related metadata do not establish genuine body-query
AST qualification. These establish only guard/query/capture connection. Pre-acquisition scope
freezing, origin/order independence, qualification, instability, preparation, and
private post-ledger integration remain outside this seam.
