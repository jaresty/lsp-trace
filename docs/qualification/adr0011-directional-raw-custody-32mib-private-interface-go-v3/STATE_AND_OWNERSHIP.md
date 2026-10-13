# State, ownership, accounting, and synchronization v3

## Ownership

| Object | Lifetime/source | Bytes | Leases/charge |
|---|---|---:|---:|
| Manager directional-raw subledger | Manager lifetime, existing accounting domain | none | reservation/binding metadata only |
| generic owner | existing owner lifetime | sole retained backing | complete active lease set |
| owned acquisition | attempt/account lifetime until transfer/release | inseparable bytes | inseparable active lease set |
| alias/view acquisition | no longer than owner lease | no independently owned bytes | zero charge; bounded owner binding |
| account | attempt through terminal release | none | live/reserved counters |
| terminal coordinator | existing Manager operation | clears sole owner | exact-once nested discharge |

There is no second recorder, account authority, backing owner, terminal owner, or capacity source.

## States

`unreserved -> reserved -> acquired -> transferred/published -> terminal -> released -> reusable-under-fresh-identity`.

Reservation precedes every avoidable owned effect. Alias is not ownership. Transfer success leaves one destination owner and invalid source; failure leaves one source owner and unchanged destination. Terminal is immutable; release identity is separate and nonzero.

## Charge table

| Component | owned charge | alias |
|---|---|---|
| PREPARE_FRAME | exact retained capacity lease(s) | zero-charge bounded owner lease range |
| PREPARE_RESULT | exact retained capacity lease(s) | zero-charge bounded owner lease range |
| SELECTED_HANDOFF | exact retained capacity lease(s) | zero-charge span of prepare-result owner |
| DIRECTIONAL_FRAME | exact retained capacity lease(s) | zero-charge bounded owner lease range |
| DIRECTIONAL_RESULT | exact retained capacity lease(s) | zero-charge bounded owner lease range |
| RETAINED_EXTERNAL_EVIDENCE | exact retained capacity lease(s) | zero-charge while owner remains live |

## Lock order and terminal races

Only `Manager.mu -> owner.mu -> account.mu` is legal; reverse acquisition is forbidden. If release wins, view/borrow/publish/transfer refuses with terminal-race classification and no bytes. If an operation holds the legal lock prefix first, its bounded in-memory commit completes before release. Every outcome then converges on the same exact-once release operation. No lease can remain active after bytes/aliases are invalidated, and no bytes/alias can remain valid after lease discharge.

## Generic profiles

Incoming and outgoing are independent generic LSP contracts. Completeness is only `target_set_id + profile + evidence` relative. No repository/workspace/runtime/language/provider-family completeness or Go/gopls behavior is asserted.

AUTHORED_NOT_GO
