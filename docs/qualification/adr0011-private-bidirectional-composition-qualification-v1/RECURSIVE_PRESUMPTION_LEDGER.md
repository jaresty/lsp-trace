# Recursive presumption ledger

Every reversible choice is recorded as Presumed, Revised, or Depends on.

| ID | Choice | State | Reason | Revision trigger |
|---|---|---|---|---|
| P01 | Use ordered top-level `{incoming,outgoing}` | Presumed | User authorized this representation unless a stricter accepted generic/private contract applies | Revise if an accepted incoming/outgoing private composition contract with stricter representation is produced |
| P02 | Do not import ADR0011->ADR0007 `ExactCarrierV1` | Revised | The only stricter contract found is gated by `ADR0011_ADR0007_COMPOSITION_CONTRACT_GO`, explicitly excluded here | Revise only under separate ADR0011->ADR0007 authorization |
| P03 | Top-level values are directional sum/envelopes | Revised | Reviewer NO_GO showed failure-retention fixtures must be valid failure results, not metadata mismatches | Revise if an accepted production failure schema supersedes fixture-native type |
| P04 | Define private fixture-native failure type | Revised | No accepted production failure schema is available inside this authorization; fixture-native failures are bounded and private evidence-only | Replace if a production/private accepted directional failure schema is authorized |
| P05 | Treat accepted dependency GO as dependency state, not producer self-GO | Presumed | Directional packages remain authority 0, accepted false, production refused | Revise if a reviewer requires a different dependency-state spelling without changing authority |
| P06 | Store success directional counts as metadata but failure envelopes claim no counts | Revised | Success references preserve accepted fields; failure results must not fabricate successful counts/claims | Revise if an accepted failure schema defines failure accounting fields |
| P07 | Preserve per-direction 32MiB ceiling and admit no combined ceiling | Depends on | Existing raw-custody design freezes `33554432` per attempt/live slot; this package changes nothing | Revise if accepted ceiling design changes upstream |
| P08 | Verifier exact-equality for success payloads and closed validation for failure payloads | Revised | Success pins must remain exact while failure envelopes need bounded valid variability | Revise if schema-specific diagnostics are required without broadening acceptance |
| P09 | Metadata canonicalization allowed | Presumed | Canonical metadata is necessary for byte-stable idempotence and subtree preservation; raw provider evidence is not read/remarshaled | Revise if accepted contract forbids canonical metadata serialization |
| P10 | One direction failure cannot erase/relabel/change the other | Revised | Now asserted as valid PASS fixtures with peer subtree hashes | Revise only if a later contract explicitly introduces coupled failure semantics |
| P11 | No graph/union/dedup claim | Presumed | Composition is evidence/failure envelope pairing, not relation graph construction | Revise only with separate graph-admission authority |
| P12 | Failure `message_digest` uses exact lowercase SHA-256 URI-like spelling | Revised | Reviewer NO_GO found `sha256:not-a-hex-digest` passed; verifier now requires string full-match `sha256:[0-9a-f]{64}` | Revise only if an accepted failure schema chooses a different digest syntax |
| P13 | Package remains private/default-off | Presumed | Assignment forbids public/default-on/integration/production changes | Revise only under new explicit authorization |
