# ADR-0011 C11 historical source packet

This packet preserves the predecessor logical source denominator under `historical/` using exact bytes from approved commit `1c6bf29f93f0bbdc6ee795d3a6d1f91c513a6d69`.

The manifest is authority-only and does not claim broader matrix closure, live runtime proof, managed-session identity, or production authority. The selector names this packet as current and the predecessor manifest as historical. Live or dirty working-tree bytes are not substituted for the frozen snapshot.
