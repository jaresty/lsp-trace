# ADR 0007 amendment: key-free local inference with optional GPT

- **Status:** Accepted scope and governance amendment; execution remains disabled
- **Decision basis:** @jaresty (GitHub handle) clarified that local-model support is intended to let the feature work without an API key, not primarily to avoid remote disclosure. GPT remains in the development/evaluation loop and may be an optional backend. @jaresty is the approval authority and authorized this documentation revision.
- **Amends:** [ADR 0007](0007-optional-local-semantic-feature-index.md)

> **Successor direction:** [Caller-provided inference first](0007-caller-provided-inference-amendment.md) supersedes the first-delivery priority below. This document preserves the earlier local-model and optional-GPT profile decisions; standalone key-free inference remains deferred, not abandoned. Read its execution gates in the scope of those backend profiles, not as prerequisites for ordinary host-assisted evidence interpretation.

## Scope and precedence

**Key-free local inference remains the delivery goal.** Once an approved local model/runtime is provisioned, the feature must be usable without a GPT account, API key, or successful remote request. GPT is not a mandatory runtime dependency. This amendment corrects the earlier uncommitted wording that incorrectly made the GPT-backed profile the delivery plan.

Two explicitly selected backend profiles share the evidence contract:

- **Local model:** inference runs on the operator's machine without an API key. The original ADR's local-model verification, staged provisioning, network-denied inference, process isolation, and applicable supply-chain controls remain requirements for this backend. Yzma remains a candidate, not a selected or qualified implementation. Key-free does not imply zero setup, bundled model weights, or no provisioning downloads.
- **Optional GPT:** remote inference may support development/evaluation or separately enabled feature execution. Source transmission, provider terms, credentials, costs, and remote provenance limits require explicit approval. A GPT development agent is not evidence that the shipped feature needs GPT or that a local backend is qualified.

This amendment supersedes the blanket rejection of hosted inference only for the optional GPT profile. It also scopes governance and evaluation to single-user TARGET Describe below. It does not retire the local-model plan or waive its backend-specific requirements. Other evidence, authority, lineage, accounting, and public-enablement restrictions remain in force.

“Local tool” describes orchestration, artifact management, and user-facing use on the operator's machine; inference location is always stated separately. Local-model requests never silently fall back to GPT. Missing local models or resources produce an explicit unavailable/failure outcome, not an API-key requirement or hidden upload. Enabling GPT is an explicit backend choice. Neither profile authorizes a hosted application, shared service, public CLI/MCP surface, or integration into core binaries.

The first deliverable is a bounded source-supported description of one explicitly admitted target, optionally with its deterministically selected evidenced consumer. Embed, index-build, Search, grouping, NEIGHBORHOOD, and CENSUS execution are outside this initial profile. Their existing requirements are deferred, not satisfied by TARGET results.

## Operator and approval

@jaresty is the accountable operator and final human approval authority for this single-user profile, including source disclosure, local artifact handling, provider selection, resource budgets, and enablement. Separate human security, legal, records, and evaluation approvers are not mandatory roles for this profile. This does not grant rights to disclose third-party source or override repository, employer, client, or provider obligations; unresolved permission blocks disclosure.

Agent reviewers provide technical assessment in separately recorded contexts. They are not human acceptance authorities, and using the same model family must be disclosed. Reviews must not receive hidden answer keys or be represented as independent organizational approval. Generated products retain `authority=0`, `accepted=false`, and explicit coverage/completeness limitations. Human approval is a separate record and does not rewrite those fields.

## Controlled optional remote inference

Before sending source, the operator approves the source scope and the provider/endpoint profile. Every request contains only explicitly selected, caller-admitted evidence and frozen instructions. No silent upload, whole-repository sweep, background acquisition, model-selected tools, external browsing, or hidden live source reacquisition is permitted.

Local orchestration may contact only the approved inference transport for these requests. That is an explicit exception to network denial, not unrestricted network access. Credentials remain outside model evidence and retained logs; record nonsecret credential references only. SDK updates, downloads, fallback providers, and retries are not implicit permissions.

Record provider, requested and returned model/version identifiers where exposed, endpoint/API version, transport/client identity, inference settings, request/response bytes and digests, and request identifiers where available. Remote weights, tokenizer internals, service binaries, and hardware may be unavailable: record that limitation rather than invent local-artifact hashes or claim exact backend reproducibility. A provider alias is not an immutable model digest. Cache identities bind the available provider profile and its version/currentness policy; unknown remote changes limit cache and replay claims.

Before enablement, record the applicable provider data-use/retention terms and the operator's approval of source transmission. Local deletion cannot certify provider-side erasure. Receipts distinguish local deletion from external retention or unknown remote disposition. No unverified privacy, authentication, or secure-erasure claims are allowed.

## Preserved evidence and processing boundaries

The caller still completes Select → Resolve → Assemble and admits exact evidence before semantic processing. The model cannot invent CALLS, select topology, resolve source paths, change consumer selection, or promote descriptions to feature identity, correctness, ownership, runtime behavior, or acceptance.

Preserve typed admission, exact source/custody identities, immutable request/output and correction lineage, independent post-generation validation, explicit abstention and limitations, and closed terminal accounting. Model responses are untrusted data, never executable instructions.

Requests require finite input/output, time, cost, concurrency, and attempt budgets. If correction is enabled, its finite policy is frozen before execution; every original response, validation result, correction request, and response remains attributable. Report first-response success separately from corrected-output success. Fresh-context correction must be labeled as such. This amendment selects neither a retry count nor an implementation transport.

## Profile-specific G1–G8 prerequisites

Gate names remain, with common single-user TARGET Describe criteria and explicit backend-specific obligations below. Local-model qualification and optional GPT qualification are separate: neither backend's results automatically qualify the other. No gate is automatically completed by this amendment.

| Gate | Required single-user TARGET Describe decision/evidence |
|---|---|
| G1 Protocol | Complete executable input/output schemas; exact request construction; terminal/lifecycle, cancellation, validation, and correction rules; bounded conformance checks. No backend-specific types at the evidence boundary. |
| G2 Admission and lineage | Exact admitted target and optional selected-consumer evidence; immutable identities and custody; explicit historical/live distinction, revision/currentness, correction, invalidation, and coverage rules. |
| G3 Artifact governance | Inventory of artifacts actually created; local storage/access, retention/deletion and backup policy. Optional GPT additionally requires approved outbound payload scope and provider retention disclosure. No invented provider deletion guarantees. |
| G4 Model/runtime or provider/client | Local: verified model/runtime artifact identities, model license and distribution decisions, tokenizer/template/quantization and native dependency/ABI identities as applicable, staged provisioning, and update/revocation policy. GPT: approved provider/endpoint/model profile, credentials handling, data-use terms, and pinned local client/dependency identities where applicable; unavailable remote internals remain explicit. Local-model requirements cannot be marked satisfied by a GPT dossier. |
| G5 Runtime | Both: bounded orchestration, no model tools/traversal, cancellation, cleanup, and isolation of failures from structural operations. Local: separate network-denied inference process, automatic downloads disabled, and demonstrated operation without provider credentials or remote calls after provisioning. GPT: only approved inference egress. Backend selection is explicit; no automatic cross-backend fallback. |
| G6 Evaluation | Frozen TARGET Describe corpus and held-out policy, hidden expectations, source-support and limitation review, ambiguity/absence/failure cases, schema conformance, replay criteria, attempt accounting, numeric thresholds, resource limits, and stop rules. Semantic replay requires declared adjudication, not string labels or surface normalization. Retrieval/ranking/grouping metrics, index baselines, and NEIGHBORHOOD/CENSUS treatments do not gate this Describe-only profile; they remain prerequisites for claims about those operations. |
| G7 Ownership | @jaresty's recorded operator/approval identity and scoped decision; technical reviewer roles, model/context relationships, and conflicts disclosed. No fictional additional human approvers. |
| G8 Bundle and enablement | A new immutable backend-profile-specific manifest binding approved G1–G7 artifacts and decisions, followed by @jaresty's explicit enablement referencing that exact bundle. GPT enablement does not complete the key-free local delivery goal. Never relabel the historical draft bundle as approved. |

Schemas, policies, thresholds, budgets, and source-disclosure choices still need concrete values and approval. Routine preparation defects can be corrected before freezing within an authorized work budget; post-freeze changes require a successor identity, not retroactive repair.

## Existing evidence and remaining authorization

Historical local diagnostics, GPT descriptions, calibration runs, and transport probes remain bounded supporting evidence. This amendment neither reclassifies failed runs nor establishes general Describe qualification. No claim is made that local-model behavior has been qualified by remote GPT results.

This decision authorizes the revised scope and governance, not a new model run, pilot enablement, runtime implementation, source upload, shipment, public surface, commit, or push. The next decision packet should identify the smallest complete G1–G8 bundle for key-free local TARGET Describe, with any optional GPT development or runtime profile recorded separately. Reuse valid existing artifacts without repeating closed calibration merely to produce more evidence; GPT calibration cannot substitute for local-backend execution and evaluation.
