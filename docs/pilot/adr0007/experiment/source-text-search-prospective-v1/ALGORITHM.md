# Algorithm

1. Validate schema_version, nonempty valid UTF-8 query, explicit positive work/output limits, and nonnegative caps.
2. Independently hash source bytes and require equality with the bound file_sha256 before scanning.
3. Scan every byte offset in deterministic path-byte then offset order; compare query bytes literally and case-sensitively; advance by one byte to preserve overlaps.
4. Convert byte half-open endpoints to UTF-16 half-open endpoints by decoding UTF-8 before each endpoint; non-BMP code points count as two UTF-16 units.
5. Precharge work as W=50+3J+5Q+7P+S+11T+13M+17R+19U+31B. Equality to max passes; max+1 fails.
6. On any failed check emit failure with empty matches and zero counters.
