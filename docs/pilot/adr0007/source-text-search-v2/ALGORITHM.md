# Algorithm

1. Validate request, policy, source binding, limits, cancellation, and deadline before reading source bytes for search semantics.
2. Admit every source file through the pinned imported source-admission v2 identity. Reject duplicate path/revision/digest tuples, path traversal, invalid UTF-8 paths, invalid UTF-8 source, and any digest/revision/admission mismatch.
3. Precharge checked uint64 accounting before semantic work. Work formula: `W=50+3J+5Q+7P+S+11T+13M+17R+19U+31B`, where J=admitted file count, Q=query bytes, P=sum UTF-8 path bytes, S=sum source bytes, T=scanned tuple count, M=match count, R=range count, U=UTF-16 units in matched ranges, and B=canonical output bytes. Limits are inclusive; `limit+1` fails before returning partial success.
4. Poll cancellation before admission, after admission, before each file scan, after each match append, before output measurement, and before terminal record. Deadline takes precedence over cancellation when both are observed at the same poll; invalid input precedes both when detected before the first poll.
5. Scan literal query bytes at every byte offset. Advance one byte after a hit to preserve overlaps. Sort by UTF-8 path bytes, then byte offset.
6. Report exact byte half-open and LSP half-open line/UTF-16-character positions. CRLF is one line break, LF is one line break, bare CR is one line break, and non-BMP code points count as two UTF-16 code units. No clamping.
7. Measure final canonical result bytes by fixed point: encode with accounting output bytes zero, set measured byte length, re-encode once, and require equality with the declared fixed-point length.
