# PREDECESSORS

- source admission history: `2f21811f`
- Location v1 history: `945bf7c2`
- immutable blocked predecessor: `v1blocked@13638e0deda0e3e3021a5b0c877280dbcfc16b45`

Current `sourceadmissionv2` admits explicit NFC canonical paths, exact revision and UTF-8 bytes, verifies file and object SHA-256, includes both digests in the admission digest, rejects duplicates and returns defensive copies. It replaces the narrower current `sourceadmissionv1`, whose admission digest omitted ObjectDigest and whose path check did not enforce NFC.
