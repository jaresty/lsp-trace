# ADR0011 call-hierarchy Layer-1 RED oracle corpus v1 — independent byte review

## Scope and boundary

Reviewed `/Users/schwa/dev/lsp-trace-adr0011-c17` at working-tree HEAD `1f59dfb4a2f20de99c1f0b0ffd44e09d8d0a9659`, with the modified RED file and 26 untracked corpus files present. This review did not modify or execute the corpus, RED, source, or design; did not use a corpus generator or runtime output; and authorizes freezing the corpus only. It does not authorize the RED loader, RED behavior, implementation, production admission, or runtime semantics.

## Independent method

A fresh disposable in-memory Python parser was derived only from the qualified registry `docs/qualification/adr0011-callhierarchy-layer1-red-oracle-corpus-v1.md`. It parsed the four normative tables, canonicalized JSON as UTF-8 with sorted keys, compact separators, and exactly one LF, validated every published length and SHA-256, executed the literal `C_RECORD` and `C_WORLD` mappings, and compared the resulting 25 roots byte-for-byte with the untracked corpus. No parser or expanded output was written to disk.

The parser independently reproduced all 63 Table-3 records and all 25 Table-4 roots. It found 860 atomic rows, four constructors, 63 complete records, and 88 Table-4 invocations: 63 record invocations plus 25 roots.

## Exact byte and manifest results

- World files: 25/25 exact byte matches.
- Canonical encoding: 25/25 compact sorted-key UTF-8 JSON with exactly one terminal LF; no CR or extra LF.
- Published world lengths: 25/25 exact.
- Published world SHA-256 values: 25/25 exact.
- Paths and order: exactly `manifest.json` followed by `worlds/world-01.json` through `worlds/world-25.json`; manifest world entries are root-ID ordered and contain the exact name, path, length, and SHA-256 values.
- Extra corpus files: zero.
- Undefined references: zero.
- Cycles: zero.
- Record expansion mismatches: zero.
- World expansion mismatches: zero.
- Missing or unexpected canonical fields: zero.
- Manifest canonical bytes: 4,649 bytes, SHA-256 `189d931b38ce6179a9400e6a1d3abf9ab7472940171d5e1c7b0c2076c9e291b0`.
- Aggregate rule: SHA-256 over root-ID-sorted UTF-8 lines `ROOT_ID<TAB>SHA256<LF>`.
- Aggregate SHA-256: `88b897ed1e59e7d53c167d6cbd30c943f59b396e7e7ea6520a28b50598a56ac0`, exactly matching the manifest.

The manifest is canonical and noncircular: its top-level keys are exactly `aggregate_sha256`, `schema`, `source_pins`, and `worlds`; it neither names nor hashes itself; world derivation does not consume the manifest; and no generator identity, runtime result, or runtime-produced authority is present.

## Source-pin verification

All manifest source pins match direct repository bytes:

- source HEAD: `4f2b5504c1e4f4e559f48db7ab8e0c5b8ca70fa9`;
- fixture file and exact `[2044,4304)` excerpt;
- HEAD RED file;
- reviewed working-tree RED content and Git blob;
- exact `[0,19437)` Layer-1 RED excerpt;
- exact binary RED diff, including 454 insertions and zero deletions.

Source-pin defects: zero.

## Semantic and accounting recheck

- **S1:** pass. World 3 has exact top-level/result JSON `null`; Worlds 4 and 5 have distinct nonempty incoming/outgoing results and distinct response bodies.
- **S2:** pass. Worlds 6–11 have six distinct accepted inputs and coherent protocol, malformed, cancellation, deadline, shutdown, and transport-unavailable outcomes with unavailable results.
- **S3:** pass. World 12 has unavailable request payload and null result with `successful-null`; World 13 has available zero-length request payload and empty result; World 14 has available nonempty payload; World 15 changes source while retaining identical pre/post retained value, bytes, and digest with `no_alias=true`.
- **S4:** pass. All 63 `C_RECORD` expansions and 25 `C_WORLD` expansions are literal, complete, and neutral; validation-only `kind` and world `name` are not emitted; no canned result, ambient lookup, omitted input, or hidden transform was found.
- **S5:** pass. Worlds 17–23 express refusal only through `refusal`; error remains available but not present with class `none`, null code/message/data, canonical serialized `null`, and retained external/transport observations.
- **World 24:** exactly 16 unique complete workers. Ledger sums are callbacks 0, diagnostics 0, external bytes 608, leases 0, reservations 0, transport bytes 960, and transport messages 16.
- **World 25:** exactly 22 unique complete executions in 11 enabled/disabled pairs. Every pair preserves equal external and transport records while observation availability differs as declared.
- **Opacity:** all 63 complete records expose selected, handoff, and predecessor commitments only as `available`, opaque `identity`, `length`, and `sha256`; no underlying bytes or semantic authority leak.

Closure and semantic defect counts are all zero.

## Disposition

`ADR0011_CALLHIERARCHY_LAYER1_ORACLE_CORPUS_BYTES_GO`

This GO authorizes freezing only the exact reviewed corpus bytes and canonical manifest. It does not authorize the RED loader, RED behavior, source/design changes, runtime execution, implementation, production use, or any authority beyond this bounded synthetic corpus.
