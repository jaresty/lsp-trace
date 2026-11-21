# Checked-reader cleanup cancellation repair

The context-aware checked reader now completes all owned descriptor cleanup before its final context check. Both parent-walk failure cleanup and transferred data-read cleanup use the same private completion helper. Cancellation withholds bytes; `errors.Join` retains prior read/policy failures, all reported close failures, and the context classification for `errors.Is`. Data closes first, parents close in reverse acquisition order, and the borrowed root is never owned or closed by this reader.

Tiny deterministic tests synchronously cancel from data or parent Close, including combined close/read failures, parent-only cleanup and uncancelled controls. No sleep or OS-syscall-preemption claim. The scope inspector's existing outer context/report check remains unchanged; no authoritative scope-report leak was demonstrated.

Default overall job expiry remains null (none); the separate 30-second cooperative inspection bound and ordinary census behavior are unchanged. This is a narrow repair, not acceptance or consolidation of the broader delivery. Parent independent review remains required.

Evidence, exact commands, RED/GREEN logs and owned-file hashes: `/tmp/checked-cleanup-repair-cbgQj2/REPORT.md`.
