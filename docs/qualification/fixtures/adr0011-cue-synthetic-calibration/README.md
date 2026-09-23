# Synthetic CUE calibration (OPEN draft)

These two ASCII-only CUE files were authored for this calibration exercise. They are not sampled from a real project. Both files share package `calibration`; there are no imports or external dependencies. Positions below are zero-based `(line, UTF-16 character)` and point at the `#` of a definition or reference. ASCII ensures each character, including a tab, occupies one UTF-16 code unit (columns count the tab as one code unit, not its displayed width). This is an OPEN expectation map, not held-out labels or a provider qualification result.

| File | Query position | Expected high-level target / contrast |
| --- | --- | --- |
| `base.cue` | `(2, 0)` | Definition `#Port`, integer range constraint. |
| `base.cue` | `(4, 0)` | Definition `#Endpoint`, struct with `host` and `port`. |
| `base.cue` | `(6, 7)` | Reference `#Port` in `#Endpoint.port`, targeting `base.cue` `(2, 0)`. |
| `service.cue` | `(2, 0)` | Definition `#Service`, struct with `endpoint` and `retries`. |
| `service.cue` | `(3, 11)` | Cross-file reference `#Endpoint`, targeting `base.cue` `(4, 0)`. |
| `service.cue` | `(7, 9)` | Reference `#Service`, targeting `service.cue` `(2, 0)`. |
| `service.cue` | `(0, 0)` | Contrast: package declaration, not a named definition or reference query. An empty definition/reference match is legitimate here. |

Expected targets are source-level hypotheses only. A provider may expose different symbol ranges, naming, or unsupported operations; this document makes no managed-provider attestation, completeness claim, or real-world qualification claim. Keep any provider observation separate from these OPEN expectations.
