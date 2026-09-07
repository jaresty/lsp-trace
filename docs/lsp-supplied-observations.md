# LSP-supplied document observations: runtime seam only

This is a preparatory runtime API, **not the completed graph-facing provenance
increment**. No CLI/MCP evidence flag, graph envelope, graph binding census,
post-traversal receipt collection, or offline graph-provenance validator is added
by this seam. Legacy callers do not opt in automatically.

A trusted Go caller may set `sessionruntime.DocumentRequest.CaptureSupply` when
calling `Manager.PrepareDocument`. After an actual successful `didOpen` or
full-text `didChange` write, `DocumentResult.Supply` contains:

- `Classification: LSP_SUPPLIED`;
- exact session ID, generation, URI, document version and notification method;
- owned source bytes from the same read used to construct the notification;
- an owned copy of the exact JSON notification parameters supplied to the writer.

A successful write is supply evidence only. It does not establish that the server
consumed those bytes, that an eventual graph corresponds to them, or that the
recorded generation remains active. A later supply or restart does not rewrite
historical observations. Public Go values are forgeable; checking their internal
consistency would not authenticate their origin.

Unchanged/cached documents return nil `Supply`, even if capture is requested.
In particular, enabling capture after an earlier uncaptured notification does not
reconstruct that missing historical evidence from the current filesystem or a
cached digest. Failures and omitted capture also return nil `Supply`. Callers must
not interpret nil as proof that the server never received any document.

Evidence mode accepts an exact canonical file URI contained by the host-owned
session workspace. Reads use `os.Root` and the existing platform-qualified scoped
regular-file opener. Symlink escapes, nonregular files, missing files, invalid
UTF-8, and content over the 1 MiB per-call bound fail without a supply observation.
Invalid UTF-8 is rejected because JSON serialization would otherwise replace
bytes. Slow regular-file I/O is not promised to be cancellable. Source trees are
not frozen, and document reads may overlap before the runtime's write lock.
Notification serialization/writing remains generation-checked under that lock.

The runtime retains no new content buffers in session state. Returned content and
parameter buffers belong to the caller, which controls their retention and must
handle them as sensitive source material. This is a per-call acquisition bound,
not an aggregate bound on all concurrent callers' retained results.

Omitted mode keeps the existing filesystem and document synchronization behavior.
The new optional fields are omitted when JSON-encoding zero-valued requests and
results. No legacy graph IDs, graph schemas, or graph bytes are changed.

This is weaker provenance, not adoption of the PRD's normative source snapshot
policy, AC1 completion, or analyzed-source authentication. Independently approved
byte custody remains separate from server consumption. Dependency completeness
remains unknown; this seam performs no dependency census. A future graph wrapper
must retain post-traversal captures separately and explicitly mark their analyzed
version unverified, rather than upgrading them to LSP-supplied evidence.
