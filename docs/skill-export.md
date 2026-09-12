# Skill export publication safety

`lsp-trace skills get` materializes an embedded skill transactionally. On Unix,
the exporter creates a private staging container relative to an already-opened
destination-parent descriptor, opens that container, writes the complete skill
as its fixed `payload` child, and publishes that child with a no-replace rename
from the pinned container descriptor to the pinned destination-parent
descriptor. Moving, renaming, or substituting the container's directory entry
therefore cannot substitute a different payload.

Before the publication hook is crossed, failures remove the private staging
container. After that boundary, the container's pathname is no longer trusted:
an adversary may have moved the intended container and installed competitor
content at the old name. Unix does not provide a portable operation that removes
an opened directory itself by descriptor, so the exporter does not pathname-delete
that entry after the boundary. An empty private staging container can therefore
remain after publication, or the intended container can remain at an
adversary-chosen name after a concurrent move. This safe orphan is preferred to
risking deletion of competitor content. The payload either publishes from the
pinned source parent or publication fails without replacing the destination.
Windows publishes using the exact opened payload handle and follows the same
post-boundary pathname-cleanup restriction.
