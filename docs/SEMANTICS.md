# Graph semantics

An edge means the configured language server reported a static incoming Call Hierarchy relation. It does not prove runtime execution, reachability on a deployed configuration, frequency, or whole-program completeness. Discovery proceeds from callee to callers, while each edge is serialized in execution orientation: caller to callee.

Completeness is bounded by server capability and responses plus visible depth, node, timeout, cancellation, and error frontiers. Unsupported Call Hierarchy never authorizes a generic-reference fallback. Cycles describe graph structure; they are not roots or proof of runtime recursion.

The qualification fixtures include `StaticButNotExecuted`, whose call is behind a branch normal fixture execution does not take. A server may still report that edge, demonstrating the distinction between static evidence and observed execution.
