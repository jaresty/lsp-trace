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
that entry after the boundary. The normal post-publication outcome is therefore an empty private staging
container orphan: unsafe pathname deletion is intentionally omitted. After a
concurrent move, the intended container can instead remain at an
adversary-chosen name. These safe orphans are preferred to risking deletion of
competitor content. The payload either publishes from the pinned source parent
or publication fails without replacing the destination.

On Windows, the exporter opens the fixed `payload` child with `NtCreateFile`
relative to the already-pinned container handle, refusing reparse traversal,
and publishes using that exact payload handle. Windows staging directories and
files inherit ACLs from the destination parent; unlike Unix modes, the requested
`0700`/`0600` permissions do not replace or tighten those inherited ACLs.

The security boundary is namespace integrity, not immutable source content.
Pinned descriptors and handles resist moving, renaming, or substituting the
container and payload namespace entries between acquisition and publication.
Unix privacy assumes normal ownership and enforcement of the requested `0700`
directory and `0600` file modes. Neither platform claims protection against a
same-credential process or administrator that is permitted to mutate the
contents of an already-opened staging file or directory.
