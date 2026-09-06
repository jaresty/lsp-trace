# Contributing-input receipts

Owner: isolated contributing-input agent. Baseline 4969599. This local committed claim supersedes the initial shared namespace draft; parent copies it to the coordination namespace. No other claim owned.

## Scope and consumer contract
Own only new `internal/source/contributing_inputs.go`, `internal/source/contributing_inputs_test.go`, `scripts/check_contributing_inputs.py`, `artifacts/contributing-input-witnesses.json`, and this claim. Identity, receipt.go, discovery, manifest and composition files remain unchanged.

Goal: capture bytes actually read through bounded acquisition, classified SOURCE/CONFIGURATION/DECLARATION/GENERATED_MAPPING, bind retained contribution identifiers to immutable receipt digests; never infer use from census.
Dimensions: changed bytes, omitted contributions, unknown dependencies, lexical and symlink escapes, duplicates/order, class separation, read failure, immutable copies.
Enforcement: existing subject exercise; assertion RED; recorder over os.Root actual ReadFile; existing CanonicalizeReceipt; contribution bindings only to observed reads; deterministic evidence; isolated mutation checks; source/full tests.

### Additive API
- `NewInputRecorder(root string) (*InputRecorder,error)` opens bounded os.Root.
- `ReadInput(name string, class InputClass) ([]byte,string,error)` reads actual bytes and returns canonical receipt SHA-256. Classes: InputSource, InputConfiguration, InputDeclaration, InputGeneratedMapping. Mechanism `bounded-input/<CLASS>` binds class into existing canonical receipt bytes. Malformed path/class: rejection, no IO/receipt. Scoped read failure: unreadable receipt ID plus error, no bytes.
- `BindContribution(id string, receiptIDs ...string) error` accepts only already observed IDs; no externally declared receipt import; rejects duplicate contribution and duplicate/unobserved references.
- `Evidence(contributionIDs []string) (InputEvidence,error)` supplies detached Inputs, Contributions, IncompleteReasons. All attempted scoped reads retained (not all necessarily consumed by a retained output); expected retained outputs lacking bindings remain explicit empty-reference rows. Always includes `unobserved dependencies are unaccounted for`; never complete/authenticated.
- `Close() error` closes root. Recorder methods serialize through mutex.
- `InputReceipt` fields: ID, Class, Receipt, Content, CanonicalReceipt. ID hashes CanonicalReceipt; existing receipt hashes Content. `InputContribution`: ID, ReceiptIDs.

### Identity and production joins
No new snapshot identity construction. Identity owner BuildIdentity requires ManifestReceipt/ManifestDecision; parent must derive those from observed receipts and a separately explicit membership decision. Receipt bytes/class identity alone are not source/snapshot/collection identity or custody authority.
Composition must route actual generic acquisition file reads through ReadInput, bind actual retained output IDs at its consumer seam, and pass the retained output set to Evidence. This frame does NOT claim live provider consumption coverage, integrated production collection, or authenticated admission. No Ember-specific additions or provider-injected declarations. Arbitrary callbacks and external processes remain unobserved; all evidence remains incomplete for those dependencies.

## Observations
Existing generic subject: `go test ./internal/source` — 28 passed.
Initial present minimal recorder: `go test ./internal/source -run TestInputRecorder -v` — 0 passed, 8 failed, with assertion-specific accounting/class/failure/binding/order failures (not compilation errors).
Minimal implementation: same command — 8 passed.
`python3 scripts/check_contributing_inputs.py artifacts/contributing-input-witnesses.json` — all 14 assertion-specific perturbations rejected, source restored. Includes removal of unknown/omitted evidence, class binding, changed-byte sensitivity, lexical/symlink containment, failure capture, defensive copying, unknown/duplicate references, duplicate expected/retained contribution IDs, and canonical order. Script retains exact commands/assertion failure outputs and refuses compilation failures as witnesses.
`go test -race ./internal/source` — 36 passed. `git diff --check` passed.
Initial full suite while new files were uncommitted: 3359 passed, 2 failed (one parent/subtest), 5 skipped; only `ASSERT_PACKAGE_OWNERSHIP_ONLY` rejected the first uncommitted new path. It checks git status against historical exact paths. No guard was weakened. Full suite on clean committed state: `go test ./...` — 3361 passed in 41 packages.

## Derivation
Goal: Retain classified immutable receipts for bytes actually read by bounded acquisition and bind observed input receipts to retained contributions without treating discovery as consumption.
Dimensions: Source/configuration/declaration/generated-mapping distinctions; byte changes; scoped and failed reads; omitted and unknown contributions; duplicate/order invariance; defensive copies.
Enforcement: os.Root-scoped actual reads, canonical existing receipt construction with class-bound mechanism, SHA-256 receipt references, observed-reference-only contribution binding, deterministic detached evidence and explicit incomplete reasons; executable assertion RED/GREEN and isolated perturbations.
Limits: Opt-in generic acquisition seam, not yet wired into production providers/execution/publication. Caller contribution binding remains a reporting obligation, not independent proof of use. Unobserved dependencies always remain unaccounted for. Identity joins and separately provisioned custody authority belong to their owners/parent. Historical schemas/identities unchanged; no deployment or program acceptance.
