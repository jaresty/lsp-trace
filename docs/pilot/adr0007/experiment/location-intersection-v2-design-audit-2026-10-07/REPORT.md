# ADR 0007 Location v2 design audit

Verdict: **LOCATION_DESIGN_BLOCKED**

Freeze: `sha256:ca1c860a1dd5a80e9c37afc57dde67d6cefc246f796b46bf252df02baa34de88` (`47,755` bytes)

The deterministic 24-case corpus, runtime projection, schemas, baseline mutation suite, tests, race tests, vet, and two-root regeneration passed. However, the freeze verifier did not enforce a bijection between every regular root file except `FREEZE.json` and exactly one manifest entry.

Independent mutations proved that deleting a freeze entry and adding an unlisted regular file were both falsely accepted. V2 therefore remains immutable and blocked. A prospective successor must enforce the complete root census and persist guards for omitted entries, unlisted files, duplicate entries, and malformed or substituted entries.

No `LOCATION_DESIGN_GO`, qualification execution, semantic acceptance, feature identity, completeness, production authority, public surface, release, or push is established. Ceilings remain `authority=0`, `accepted=false`, `completeness=UNKNOWN`, and `featureIdentity=UNRESOLVED`.
